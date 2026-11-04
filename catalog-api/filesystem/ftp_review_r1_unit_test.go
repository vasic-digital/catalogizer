package filesystem

// Review round 1 probes (adopted from the independent reviewer). rvFake is a hand-written fake server: UNIT-CLASS (constitution 11.4.289 C); the pyftpdlib tests are LIVE-CLASS.

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fake server: conn #1 answers login then never answers SIZE; behaviour of later conns chosen by mode.
// mode "silent-banner": later conns accept and never send a banner. mode "ok": later conns answer SIZE 213 42.
func rvFake(t *testing.T, mode string) (int, *int32) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var n int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			k := atomic.AddInt32(&n, 1)
			go func(conn net.Conn, k int32) {
				defer conn.Close()
				if k > 1 && mode == "silent-banner" {
					time.Sleep(60 * time.Second)
					return
				}
				r := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 ready\r\n")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "USER"):
						fmt.Fprint(conn, "331 pw\r\n")
					case strings.HasPrefix(cmd, "PASS"):
						fmt.Fprint(conn, "230 ok\r\n")
					case strings.HasPrefix(cmd, "FEAT"):
						fmt.Fprint(conn, "211 end\r\n")
					case strings.HasPrefix(cmd, "OPTS"), strings.HasPrefix(cmd, "TYPE"):
						fmt.Fprint(conn, "200 ok\r\n")
					case strings.HasPrefix(cmd, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					case strings.HasPrefix(cmd, "SIZE") && k > 1:
						fmt.Fprint(conn, "213 42\r\n")
					case strings.HasPrefix(cmd, "MDTM") && k > 1:
						fmt.Fprint(conn, "213 20200101000000\r\n")
					default:
						if k > 1 {
							fmt.Fprint(conn, "502 not implemented\r\n")
						}
					}
				}
			}(conn, k)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, &n
}

// R1: after a cut-short op, the session is replaced and the next op works.
func TestRV_SessionReplacedAfterCut(t *testing.T) {
	port, _ := rvFake(t, "ok")
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := c.GetFileInfo(ctx, "/x"); err == nil {
		t.Fatal("expected error on silent op")
	}
	fi, err := c.GetFileInfo(context.Background(), "/x")
	if err != nil || fi.Size != 42 {
		t.Fatalf("next op after cut: fi=%v err=%v (session not replaced)", fi, err)
	}
}

// R2: cancel (no deadline) mid-op must unblock promptly.
func TestRV_CancelMidOpUnblocks(t *testing.T) {
	port, _ := rvFake(t, "ok")
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(300 * time.Millisecond); cancel() }()
	done := make(chan error, 1)
	go func() { _, err := c.GetFileInfo(ctx, "/x"); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected error")
		}
	case <-time.After(8 * time.Second):
		t.Fatal("HANG: cancel without deadline did not unblock")
	}
}

// R3: the cut-short op's reconnect must not hang when the server accepts but never greets.
func TestRV_ReconnectAfterCutBounded(t *testing.T) {
	port, _ := rvFake(t, "silent-banner")
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	start := time.Now()
	go func() { _, err := c.GetFileInfo(ctx, "/x"); done <- err }()
	select {
	case <-done:
		t.Logf("returned after %v", time.Since(start))
		if time.Since(start) > 15*time.Second {
			t.Fatalf("op returned only after %v", time.Since(start))
		}
	case <-time.After(25 * time.Second):
		t.Fatal("HANG: op with a 500ms ctx did not return in 25s (reconnect after cut blocks on banner with no deadline, holding the op turn)")
	}
	// second op with its own short ctx: must not be blocked forever by a held turn
	ctx2, cancel2 := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel2()
	_, err := c.GetFileInfo(ctx2, "/x")
	t.Logf("second op err=%v", err)
}

// R4: real pyftpdlib: ListDirectory + ReadFile through passive data connections after the Connect ctx ended.
func TestRV_LiveDataConnAfterConnectCtxEnds(t *testing.T) {
	c, sizes := realFTPFixture(t, 3) // fixture cancels its Connect ctx after Connect
	ents, err := c.ListDirectory(context.Background(), "/")
	if err != nil || len(ents) != len(sizes) {
		t.Fatalf("list: %d entries err=%v", len(ents), err)
	}
	rc, err := c.ReadFile(context.Background(), "/f01.bin")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || int64(len(b)) != sizes["f01.bin"] {
		t.Fatalf("read %d err=%v", len(b), err)
	}
}

type slowReader struct {
	r     io.Reader
	delay time.Duration
}

func (s *slowReader) Read(p []byte) (int, error) {
	time.Sleep(s.delay)
	if len(p) > 4096 {
		p = p[:4096]
	}
	return s.r.Read(p)
}

// R5: a long legit upload (> ftpOpTimeout) with a background ctx. With the cap shrunk to 1s, a 3s upload.
func TestRV_LongUploadExceedsOpCap(t *testing.T) {
	c, _ := realFTPFixture(t, 1)
	old := ftpOpTimeout
	ftpOpTimeout = 1 * time.Second
	defer func() { ftpOpTimeout = old }()
	data := bytes.Repeat([]byte("a"), 4096*15)
	err := c.WriteFile(context.Background(), "/up.bin", &slowReader{r: bytes.NewReader(data), delay: 200 * time.Millisecond})
	t.Logf("long upload err=%v", err)
	if err != nil {
		t.Errorf("a progressing upload longer than ftpOpTimeout failed: %v", err)
	}
}

// R6: long download (> cap) read slowly after ReadFile returned.
func TestRV_LongDownloadExceedsOpCap(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "big.bin"), bytes.Repeat([]byte("b"), 4096*15), 0o644); err != nil {
		t.Fatal(err)
	}
	port := startRealFTPServer(t, root)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "tester", Password: "t3st-only", Path: "/"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	old := ftpOpTimeout
	ftpOpTimeout = 1 * time.Second
	defer func() { ftpOpTimeout = old }()
	rc, err := c.ReadFile(context.Background(), "/big.bin")
	if err != nil {
		t.Fatal(err)
	}
	type res struct {
		n   int64
		err error
	}
	ch := make(chan res, 1)
	go func() {
		n, err := io.Copy(io.Discard, &slowReader{r: rc, delay: 200 * time.Millisecond})
		ch <- res{n, err}
	}()
	select {
	case r := <-ch:
		rc.Close()
		if r.err != nil || r.n != 4096*15 {
			t.Fatalf("download n=%d err=%v", r.n, r.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("HANG: slow download did not finish in 30s")
	}
}

func TestRV_SmbPathEdges(t *testing.T) {
	for _, p := range []string{"/", "//", "///a//b/", "/..", "/a/../b", `\x`, `\\srv\share\x`, "./x", "/./x"} {
		t.Logf("smbPath(%q) = %q", p, smbPath(p))
	}
}
