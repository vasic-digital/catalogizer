//go:build live

// WD-F1 probe (section 11.4.292 reproduction attempt): LIVE, real WebDAVClient.WriteFile against the real Yandex WebDAV service, inside
// /Testing only. For each upload records the client's answer, then polls existence/size (HEAD via GetFileInfo, PROPFIND via ListDirectory) at
// fixed offsets; an ACKed write (WriteFile returned nil) that is absent or has the wrong sha256 on read-back is a lost acknowledged write.
package filesystem

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"testing"
	"time"
)

type wdf1Rec struct {
	Name    string   `json:"name"`
	Size    int      `json:"size"`
	Sha     string   `json:"sha256"`
	PutErr  string   `json:"put_err,omitempty"`
	PutMs   int64    `json:"put_ms"`
	Retry   string   `json:"retry,omitempty"`
	Polls   []string `json:"polls"`
	Verdict string   `json:"verdict"`
}

func TestLiveYandexWDF1Probe(t *testing.T) {
	r := loadYandex(t)
	n, _ := strconv.Atoi(os.Getenv("WDF1_N"))
	if n == 0 {
		n = 12
	}
	sizes := []int{1 << 10, 64 << 10, 512 << 10, 2 << 20}
	if os.Getenv("WDF1_SMALL") == "1" {
		sizes = []int{512, 4 << 10, 16 << 10}
	}
	c := r.newClient(r.d.pass)
	ctx := context.Background()
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %s", rd(err))
	}
	dir := "/" + fmt.Sprintf("wdf1-%d", time.Now().Unix())
	if err := c.CreateDirectory(ctx, dir); err != nil {
		t.Fatalf("mkdir: %s", rd(err))
	}
	defer func() { _ = c.DeleteDirectory(context.Background(), dir) }()
	var recs []wdf1Rec
	lost, errAcked := 0, 0
	for i := 0; i < n; i++ {
		sz := sizes[i%len(sizes)]
		data := detBytes(fmt.Sprintf("wdf1-%d-%d", time.Now().UnixNano(), i), sz)
		h := sha256.Sum256(data)
		rec := wdf1Rec{Name: fmt.Sprintf("f%03d.bin", i), Size: sz, Sha: hex.EncodeToString(h[:])}
		p := dir + "/" + rec.Name
		t0 := time.Now()
		perr := c.WriteFile(ctx, p, bytes.NewReader(data))
		rec.PutMs = time.Since(t0).Milliseconds()
		if perr != nil {
			rec.PutErr = rd(perr)
			// retry once (the client has none), mimic the original probe
			rerr := c.WriteFile(ctx, p, bytes.NewReader(data))
			if rerr != nil {
				rec.Retry = "err:" + rd(rerr)
			} else {
				rec.Retry = "ok"
			}
		}
		acked := perr == nil || rec.Retry == "ok"
		present := false
		for _, off := range []time.Duration{0, 3 * time.Second, 10 * time.Second, 30 * time.Second, 60 * time.Second} {
			if off > 0 && present {
				break
			}
			time.Sleep(off - func() time.Duration {
				if off == 0 {
					return 0
				}
				return 0
			}())
			fi, err := c.GetFileInfo(ctx, p)
			ls, lerr := c.ListDirectory(ctx, dir)
			inList := false
			for _, e := range ls {
				if e.Name == rec.Name {
					inList = true
				}
			}
			s := fmt.Sprintf("+%s head_err=%v size=%d list=%v(%v)", off, err != nil, func() int64 {
				if fi != nil {
					return fi.Size
				}
				return -1
			}(), inList, lerr != nil)
			rec.Polls = append(rec.Polls, s)
			if err == nil && fi != nil && inList {
				present = true
			}
		}
		verdict := "ok"
		if present {
			rc, err := c.ReadFile(ctx, p)
			if err == nil {
				b, _ := io.ReadAll(rc)
				rc.Close()
				hh := sha256.Sum256(b)
				if hex.EncodeToString(hh[:]) != rec.Sha {
					verdict = "CORRUPT"
				}
			} else {
				verdict = "READ_ERR"
			}
		} else if acked {
			verdict = "LOST_ACKED_WRITE"
			lost++
		}
		if perr != nil && present {
			errAcked++
			if verdict == "ok" {
				verdict = "ok_after_put_error"
			}
		}
		rec.Verdict = verdict
		recs = append(recs, rec)
		t.Logf("%s size=%d put_err=%v retry=%s verdict=%s", rec.Name, sz, perr != nil, rec.Retry, verdict)
	}
	b, _ := json.MarshalIndent(map[string]any{"n": n, "lost_acked": lost, "put_err_but_present": errAcked, "recs": recs}, "", " ")
	if p := os.Getenv("LIVE_YANDEX_EVIDENCE"); p != "" {
		_ = os.WriteFile(p, b, 0o600)
	}
	if lost > 0 {
		t.Fatalf("WD-F1 REPRODUCED: lost acked writes = %d", lost)
	}
}
