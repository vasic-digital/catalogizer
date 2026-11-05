//go:build live

// WD-F1 forced-reset reproduction (CHAOS-class, section 11.4.292 / 11.4.289). The REAL Yandex WebDAV service still answers every request; a
// small TCP fault-injection proxy (this file's helper, a chaos tool, NOT a stand-in for the service) sits between the real WebDAVClient and the
// service as a raw byte relay (TLS is end-to-end, the client verifies the real certificate) and resets the connection:
//
//	mode A: after about half of the request body has been relayed (server never receives the full body);
//	mode B: after the whole request body was relayed and the first response byte arrives from the server (the ack is lost).
//
// After the fault the client behaviour is mimicked (the client has no retry: the caller retries WriteFile once), then HEAD+PROPFIND are polled
// to +60 s. A control needle deletes an acknowledged remote file and MUST be flagged LOST_ACKED_WRITE by the same verdict function.
package filesystem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type faultProxy struct {
	ln     net.Listener
	target string
	mode   atomic.Int32 // 0 none, 1 A, 2 B (consumed by the next accepted connection)
	size   atomic.Int64
	fired  atomic.Int32
}

func newFaultProxy(t *testing.T, target string) *faultProxy {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := &faultProxy{ln: ln, target: target}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go p.handle(c, int(p.mode.Swap(0)), p.size.Load())
		}
	}()
	return p
}

func rst(c net.Conn) {
	if tc, ok := c.(*net.TCPConn); ok {
		_ = tc.SetLinger(0)
	}
	_ = c.Close()
}

func (p *faultProxy) handle(cli net.Conn, mode int, size int64) {
	srv, err := net.DialTimeout("tcp", p.target, 15*time.Second)
	if err != nil {
		rst(cli)
		return
	}
	var c2s atomic.Int64
	var once sync.Once
	fire := func() { once.Do(func() { p.fired.Add(1); rst(cli); rst(srv) }) }
	go func() { // client -> server
		buf := make([]byte, 32<<10)
		for {
			n, err := cli.Read(buf)
			if n > 0 {
				if _, werr := srv.Write(buf[:n]); werr != nil {
					return
				}
				if c2s.Add(int64(n)) >= size/2 && mode == 1 {
					fire()
					return
				}
			}
			if err != nil {
				_ = srv.Close()
				return
			}
		}
	}()
	buf := make([]byte, 32<<10) // server -> client
	for {
		n, err := srv.Read(buf)
		if n > 0 {
			if mode == 2 && c2s.Load() >= size {
				fire()
				return
			}
			if _, werr := cli.Write(buf[:n]); werr != nil {
				return
			}
		}
		if err != nil {
			_ = cli.Close()
			return
		}
	}
}

// wdf1Verdict is the verdict logic under test; the control needle proves it fires.
func wdf1Verdict(acked, present bool, gotSha, wantSha string) string {
	switch {
	case present && gotSha != wantSha:
		return "CORRUPT"
	case present:
		return "ok"
	case acked:
		return "LOST_ACKED_WRITE"
	default:
		return "absent_not_acked"
	}
}

func wdf1Poll(ctx context.Context, c *WebDAVClient, dir, name string) (bool, []string) {
	var polls []string
	last := time.Duration(0)
	for _, off := range []time.Duration{0, 3 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second} {
		time.Sleep(off - last)
		last = off
		fi, err := c.GetFileInfo(ctx, dir+"/"+name)
		ls, lerr := c.ListDirectory(ctx, dir)
		in := false
		for _, e := range ls {
			if e.Name == name {
				in = true
			}
		}
		sz := int64(-1)
		if fi != nil {
			sz = fi.Size
		}
		polls = append(polls, fmt.Sprintf("+%s head_err=%v size=%d list=%v list_err=%v", off, err != nil, sz, in, lerr != nil))
		if err == nil && in {
			return true, polls
		}
	}
	return false, polls
}

var wdf1LastRead string // detail of the last read-back (length, first differing offset, read error class)

func wdf1Sha(ctx context.Context, c *WebDAVClient, p string, want []byte) string {
	rc, err := c.ReadFile(ctx, p)
	if err != nil {
		wdf1LastRead = "open_err=" + rd(err)
		return "read_err"
	}
	defer rc.Close()
	b, rerr := io.ReadAll(rc)
	h := sha256.Sum256(b)
	first := -1
	for i := 0; i < len(b) && i < len(want); i++ {
		if b[i] != want[i] {
			first = i
			break
		}
	}
	wdf1LastRead = fmt.Sprintf("len=%d want_len=%d first_diff=%d body_err=%v", len(b), len(want), first, rerr != nil)
	if rerr != nil {
		wdf1LastRead += " body_err_text=" + rd(rerr)
	}
	return hex.EncodeToString(h[:])
}

func TestLiveYandexWDF1ForcedReset(t *testing.T) {
	r := loadYandex(t)
	proxy := newFaultProxy(t, func() string { u := r.gt.host; return u + ":443" }())
	defer proxy.ln.Close()
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.DisableKeepAlives = true // every request = new connection, so the armed fault hits exactly the next request
	tr.ForceAttemptHTTP2 = false
	tr.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", proxy.ln.Addr().String())
	}
	r.gt.base = tr
	c := r.newClient(r.d.pass)
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %s", rd(err))
	}
	dir := fmt.Sprintf("/wdf1c-%d", time.Now().Unix())
	if err := c.CreateDirectory(ctx, dir); err != nil {
		t.Fatalf("mkdir: %s", rd(err))
	}
	defer func() { _ = c.DeleteDirectory(context.Background(), dir) }()

	type rec struct {
		Case, Name              string
		Size                    int
		FaultFired              bool
		PutErr                  string
		PollsNoRetry            []string
		PresentNoRetry          bool
		RetryAcked              bool
		PollsAfterRetry         []string
		Verdict                 string
		ReadDetail              string
		ErrButFullFilePresent   bool
		PartialOrCorruptVisible bool
	}
	var recs []rec
	lost := 0
	n := 0
	modes, szs := []int32{1, 2}, []int{64 << 10, 512 << 10, 2 << 20}
	if k, _ := strconv.Atoi(os.Getenv("WDF1_REPEAT_A2M")); k > 0 {
		modes, szs = []int32{1}, nil
		for i := 0; i < k; i++ {
			szs = append(szs, 2<<20)
		}
	}
	for _, mode := range modes {
		for _, sz := range szs {
			n++
			name := fmt.Sprintf("c%02d.bin", n)
			data := detBytes(fmt.Sprintf("wdf1c-%d-%d", time.Now().UnixNano(), n), sz)
			h := sha256.Sum256(data)
			want := hex.EncodeToString(h[:])
			rc := rec{Case: map[int32]string{1: "A_reset_mid_body", 2: "B_reset_after_full_body_ack_lost"}[mode], Name: name, Size: sz}
			p := dir + "/" + name
			before := proxy.fired.Load()
			proxy.size.Store(int64(sz))
			proxy.mode.Store(mode)
			perr := c.WriteFile(ctx, p, bytes.NewReader(data))
			proxy.mode.Store(0)
			rc.FaultFired = proxy.fired.Load() > before
			if perr != nil {
				rc.PutErr = rd(perr)
			}
			// 1) no retry: what does the service hold after the fault?
			rc.PresentNoRetry, rc.PollsNoRetry = wdf1Poll(ctx, c, dir, name)
			sha := ""
			if rc.PresentNoRetry {
				sha = wdf1Sha(ctx, c, p, data)
				rc.ErrButFullFilePresent = perr != nil && sha == want
				rc.PartialOrCorruptVisible = sha != want
			}
			// 2) caller retries once (the client itself has no retry)
			acked := perr == nil
			if perr != nil {
				if rerr := c.WriteFile(ctx, p, bytes.NewReader(data)); rerr == nil {
					acked = true
					rc.RetryAcked = true
				}
				var present bool
				present, rc.PollsAfterRetry = wdf1Poll(ctx, c, dir, name)
				sha = ""
				if present {
					sha = wdf1Sha(ctx, c, p, data)
				}
				rc.Verdict = wdf1Verdict(acked, present, sha, want)
			} else {
				rc.Verdict = wdf1Verdict(acked, rc.PresentNoRetry, sha, want)
			}
			rc.ReadDetail = wdf1LastRead
			if rc.Verdict == "LOST_ACKED_WRITE" || rc.Verdict == "CORRUPT" {
				lost++
			}
			recs = append(recs, rc)
			t.Logf("%s size=%d fired=%v put_err=%v present_no_retry=%v retry_acked=%v verdict=%s", rc.Case, sz, rc.FaultFired, perr != nil, rc.PresentNoRetry, rc.RetryAcked, rc.Verdict)
		}
	}

	// CONTROL NEEDLE: ack a write, delete it remotely behind the client's back, same verdict logic must say LOST_ACKED_WRITE.
	cdata := detBytes("wdf1-control", 4096)
	ch := sha256.Sum256(cdata)
	cw := hex.EncodeToString(ch[:])
	cerr := c.WriteFile(ctx, dir+"/control.bin", bytes.NewReader(cdata))
	if cerr != nil {
		t.Fatalf("control write: %s", rd(cerr))
	}
	derr := c.DeleteFile(ctx, dir+"/control.bin")
	cpresent, cpolls := wdf1Poll(ctx, c, dir, "control.bin")
	ctrl := wdf1Verdict(cerr == nil, cpresent, "", cw)
	t.Logf("control needle: delete_err=%v verdict=%s", derr != nil, ctrl)

	// CONTROL NEEDLE 2: acknowledged write, then overwritten remotely with SAME-SIZE DIFFERENT bytes; the CORRUPT branch must fire.
	c2 := detBytes("wdf1-control2", 4096)
	c2h := sha256.Sum256(c2)
	if err := c.WriteFile(ctx, dir+"/control2.bin", bytes.NewReader(c2)); err != nil {
		t.Fatalf("control2 write: %s", rd(err))
	}
	resp, rerr := r.raw(ctx, "PUT", dir+"/control2.bin", nil, bytes.NewReader(detBytes("wdf1-control2-other", 4096)))
	if rerr != nil {
		t.Fatalf("control2 overwrite: %s", rd(rerr))
	}
	resp.Body.Close()
	c2present, _ := wdf1Poll(ctx, c, dir, "control2.bin")
	ctrl2 := wdf1Verdict(true, c2present, wdf1Sha(ctx, c, dir+"/control2.bin", c2), hex.EncodeToString(c2h[:]))
	t.Logf("control needle 2 (same-size overwrite): verdict=%s", ctrl2)

	fired := 0
	for _, x := range recs {
		if x.FaultFired {
			fired++
		}
	}
	b, _ := json.MarshalIndent(map[string]any{"cases": recs, "faults_fired": fired, "lost_or_corrupt": lost, "control_needle_verdict": ctrl, "control_needle2_same_size_overwrite_verdict": ctrl2, "control_polls": cpolls}, "", " ")
	if p := os.Getenv("LIVE_YANDEX_EVIDENCE"); p != "" {
		_ = os.WriteFile(p, b, 0o600)
	}
	if ctrl != "LOST_ACKED_WRITE" {
		t.Fatalf("control needle did not fire (got %s): verdict logic is blind", ctrl)
	}
	if ctrl2 != "CORRUPT" {
		t.Fatalf("control needle 2 did not fire (got %s): the CORRUPT branch is blind", ctrl2)
	}
	if fired == 0 {
		t.Fatalf("no fault fired: the forced-reset condition was not exercised")
	}
	if lost > 0 {
		t.Fatalf("WD-F1 REPRODUCED: lost_or_corrupt=%d", lost)
	}
}
