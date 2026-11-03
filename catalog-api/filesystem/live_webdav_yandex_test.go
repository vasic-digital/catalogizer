//go:build live

// LIVE test of the REAL catalog-api WebDAVClient against the REAL Yandex WebDAV service (constitution section 11.4.289: no stubs, mocks or
// fixture servers). Every operation goes through Catalogizer's own client code; the few raw requests (MOVE, Range GET, the bootstrap of the
// sandbox) use the same guarded transport.
//
// HARD RULE (operator, 2026-11-03): everything stays strictly inside the remote directory /Testing. A guard RoundTripper refuses (returns an
// error before any byte leaves the process) every request whose URL path, or Destination header, is not /Testing or below it, and every
// non-https or foreign-host request. The suite works in a unique per-run sub-directory /Testing/catalogizer-live-<epoch> and removes only that.
//
// Environment (names only; values are never logged or written to evidence):
//
//	LIVE_WEBDAV_URL        https://webdav.yandex.ru                    (absent => SKIP credentials_absent)
//	LIVE_WEBDAV_USER, LIVE_WEBDAV_PASSWORD
//	LIVE_WEBDAV_SANDBOX    must be exactly /Testing
//	LIVE_YANDEX_EVIDENCE   optional file path; a JSON evidence record (counts, timings, hashes, status codes) is written there
//
// Run: scripts/test-infra/webdav_yandex_live.sh   (N=3 identical runs and a signature comparison), or
// go test -tags live -count=1 -run TestLiveYandex -v ./filesystem/
package filesystem

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// wrongPasswordConstant is a deliberately wrong credential for the auth-failure leg (never the real one).
const wrongPasswordConstant = "catalogizer-deliberately-wrong-password-0000"

type evReq struct {
	Seq    int    `json:"seq"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Status int    `json:"status"`
	Ms     int64  `json:"ms"`
	BytesU int64  `json:"bytes_up"`
	Err    string `json:"err,omitempty"`
}

// guardTransport refuses anything outside the sandbox and records every request (never headers, never credentials).
type guardTransport struct {
	base       http.RoundTripper
	host       string
	mu         sync.Mutex
	reqs       []evReq
	violations int
}

func (g *guardTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !yandexAllowedURL(req.URL, g.host) {
		g.mu.Lock()
		g.violations++
		g.mu.Unlock()
		return nil, fmt.Errorf("GUARD: refused request outside %s (%s %s)", yandexSandboxRoot, req.Method, req.URL.EscapedPath())
	}
	if dst := req.Header.Get("Destination"); dst != "" {
		du, err := url.Parse(dst)
		if err != nil || !yandexAllowedURL(du, g.host) {
			g.mu.Lock()
			g.violations++
			g.mu.Unlock()
			return nil, fmt.Errorf("GUARD: refused Destination outside %s", yandexSandboxRoot)
		}
	}
	t0 := time.Now()
	resp, err := g.base.RoundTrip(req)
	e := evReq{Method: req.Method, Path: req.URL.Path, Ms: time.Since(t0).Milliseconds(), BytesU: max64(req.ContentLength, 0)}
	if err != nil {
		e.Err = rd(err)
	} else {
		e.Status = resp.StatusCode
	}
	g.mu.Lock()
	e.Seq = len(g.reqs) + 1
	g.reqs = append(g.reqs, e)
	g.mu.Unlock()
	return resp, err
}

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

type stepRes struct {
	Name   string         `json:"name"`
	Result string         `json:"result"`
	Ms     int64          `json:"ms"`
	Notes  map[string]any `json:"notes,omitempty"`
}

type yRun struct {
	t            *testing.T
	d            liveDAV
	gt           *guardTransport
	c            *WebDAVClient
	run          string // e.g. catalogizer-live-1730000000
	created      bool
	mu           sync.Mutex
	retries      int
	retryClasses map[string]int
	lostWrites   int
	idleCancels  int
	steps        []stepRes
	notes        map[string]any
	t0           time.Time
}

func loadYandex(t *testing.T) *yRun {
	t.Helper()
	d := liveDAV{os.Getenv("LIVE_WEBDAV_URL"), os.Getenv("LIVE_WEBDAV_USER"), os.Getenv("LIVE_WEBDAV_PASSWORD"), os.Getenv("LIVE_WEBDAV_SANDBOX")}
	if d.url == "" || d.user == "" || d.pass == "" || d.sandbox == "" {
		t.Skip("SKIP(credentials_absent): LIVE_WEBDAV_URL / _USER / _PASSWORD / _SANDBOX not set")
	}
	liveSecrets = append(liveSecrets, d.user, d.pass)
	u, err := url.Parse(d.url)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		t.Fatalf("LIVE_WEBDAV_URL must be a plain https origin")
	}
	liveSecrets = append(liveSecrets, u.Host)
	if strings.TrimRight(d.sandbox, "/") != yandexSandboxRoot {
		t.Fatalf("LIVE_WEBDAV_SANDBOX must be exactly %s", yandexSandboxRoot)
	}
	d.url = strings.TrimRight(d.url, "/")
	r := &yRun{t: t, d: d, notes: map[string]any{}, retryClasses: map[string]int{}, t0: time.Now()}
	r.gt = &guardTransport{base: http.DefaultTransport.(*http.Transport).Clone(), host: u.Host}
	r.run = fmt.Sprintf("catalogizer-live-%d", time.Now().Unix())
	return r
}

// newClient builds the REAL Catalogizer client rooted at /Testing (so Connect itself only ever touches /Testing).
func (r *yRun) newClient(pass string) *WebDAVClient {
	c, err := NewWebDAVClient(&WebDAVConfig{URL: r.d.url, Username: r.d.user, Password: pass, Path: yandexSandboxRoot})
	if err != nil {
		r.t.Fatalf("new client: %s", rd(err))
	}
	c.client.Transport = r.gt
	return c
}

// raw issues one guarded authenticated request relative to the sandbox root (rel starts with "/").
func (r *yRun) raw(ctx context.Context, method, rel string, hdr map[string]string, body io.Reader) (*http.Response, error) {
	full := r.d.url + (&url.URL{Path: yandexSandboxRoot + rel}).EscapedPath()
	req, err := http.NewRequestWithContext(ctx, method, full, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(r.d.user, r.d.pass)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	return (&http.Client{Transport: r.gt, Timeout: 60 * time.Second}).Do(req)
}

func (r *yRun) note(k string, v any) { r.mu.Lock(); r.notes[k] = v; r.mu.Unlock() }

// step runs a named sequential sub-test and records its result and notes.
func (r *yRun) step(name string, fn func(t *testing.T, note func(k string, v any))) {
	r.t.Helper()
	t0 := time.Now()
	sn := map[string]any{}
	var skipped bool
	ok := r.t.Run(name, func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		fn(t, func(k string, v any) { sn[k] = v })
	})
	res := "PASS"
	if skipped {
		res = "SKIP"
	} else if !ok {
		res = "FAIL"
	}
	r.mu.Lock()
	r.steps = append(r.steps, stepRes{Name: name, Result: res, Ms: time.Since(t0).Milliseconds(), Notes: sn})
	r.mu.Unlock()
}

func (r *yRun) rel(name string) string { return "/" + r.run + "/" + name }

func (r *yRun) writeEvidence() {
	path := os.Getenv("LIVE_YANDEX_EVIDENCE")
	if path == "" {
		return
	}
	r.gt.mu.Lock()
	reqs := append([]evReq(nil), r.gt.reqs...)
	viol := r.gt.violations
	r.gt.mu.Unlock()
	byM, byS := map[string]int{}, map[string]int{}
	var up int64
	var ms []int64
	for _, q := range reqs {
		byM[q.Method]++
		byS[fmt.Sprint(q.Status)]++
		up += q.BytesU
		ms = append(ms, q.Ms)
	}
	sort.Slice(ms, func(i, j int) bool { return ms[i] < ms[j] })
	pct := func(p int) int64 {
		if len(ms) == 0 {
			return 0
		}
		return ms[(len(ms)-1)*p/100]
	}
	// signature: the part that must be identical in every run (step results + content hashes), no timings
	sig := map[string]any{}
	for _, s := range r.steps {
		sig[s.Name] = s.Result
	}
	ev := map[string]any{
		"service": "webdav.yandex.ru", "sandbox": yandexSandboxRoot, "run_dir": r.run, "date_utc": time.Now().UTC().Format(time.RFC3339),
		"total_ms": time.Since(r.t0).Milliseconds(), "requests_total": len(reqs), "requests_by_method": byM, "requests_by_status": byS,
		"bytes_uploaded": up, "latency_ms_p50": pct(50), "latency_ms_p95": pct(95), "latency_ms_max": pct(100),
		"guard_violations": viol, "transient_retries": r.retries, "lost_acked_writes": r.lostWrites, "idle_cancel_errors": r.idleCancels, "transient_retry_classes": r.retryClasses, "steps": r.steps, "notes": r.notes, "signature": sig, "credentials_in_evidence": false,
	}
	b, _ := json.MarshalIndent(ev, "", "  ")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		r.t.Errorf("write evidence: %v", err)
	}
}

func (r *yRun) ctx(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// transient reports whether err is the kind of failure the real service produces under load (observed 2026-11-03: HTTP 500 on single PUTs, TCP resets,
// one mid-body idle cancel): the client has no retry of its own, so the live suite retries a bounded number of times and RECORDS every retry in the
// evidence. A failure that persists after the retries FAILs the step.
//
// "context canceled" is NOT transient: the client raises it when its idle guard fires (no byte moved for the idle timeout), and retrying it would mask
// a false idle-cancel in the code under test. It is counted as its own class (idleCancels) that FAILs the run.
var transientKeys = []string{"connection reset", "status 500", "status 502", "status 503", "status 504", "status 429", "unexpected EOF", "Client.Timeout", "i/o timeout", "context deadline exceeded"}

func transient(err error) bool {
	if err == nil {
		return false
	}
	m := err.Error()
	for _, k := range transientKeys {
		if strings.Contains(m, k) {
			return true
		}
	}
	return false
}

// retry runs fn up to 4 times while the error is transient and the suite context is alive; every retry is counted and classified in the evidence.
func (r *yRun) retry(what string, fn func() error) error {
	var err error
	for attempt := 1; attempt <= 4; attempt++ {
		if err = fn(); err == nil {
			return nil
		}
		if strings.Contains(err.Error(), "context canceled") {
			r.mu.Lock()
			r.idleCancels++
			r.mu.Unlock()
			r.t.Errorf("%s: context canceled (idle-guard cancel or unexpected cancel) is never retried and fails the run: %s", what, rd(err))
			return err
		}
		if !transient(err) {
			return err
		}
		r.mu.Lock()
		r.retries++
		cls := "other"
		for _, k := range transientKeys {
			if strings.Contains(err.Error(), k) {
				cls = k
				break
			}
		}
		r.retryClasses[cls]++
		r.mu.Unlock()
		time.Sleep(time.Duration(attempt) * 2 * time.Second)
	}
	return err
}

// writeVerified uploads and then reads the size back (HEAD). UNCONFIRMED hypothesis from one throwaway probe whose output is NOT committed: after a
// PUT cut by "connection reset by peer" the retried PUT returned success once but the file was absent from the directory for at least 55 s, i.e. an
// acknowledged write may be lost server-side. No committed run shows it (all committed runs have lost_acked_writes=0). The client cannot see that, so
// the test confirms the write and re-uploads a lost one; every lost acknowledged write is counted in the evidence (lost_acked_writes), the live script
// FAILs the verdict on any, and more than 3 per run FAIL the step.
func (r *yRun) writeVerified(ctx context.Context, name string, data []byte) error {
	for attempt := 1; attempt <= 3; attempt++ {
		if err := r.retry("write", func() error { return r.c.WriteFile(ctx, r.rel(name), bytes.NewReader(data)) }); err != nil {
			return err
		}
		var fi *FileInfo
		err := r.retry("verify_head", func() error { var e error; fi, e = r.c.GetFileInfo(ctx, r.rel(name)); return e })
		if err == nil && fi != nil && fi.Size == int64(len(data)) {
			return nil
		}
		r.mu.Lock()
		r.lostWrites++
		lw := r.lostWrites
		r.mu.Unlock()
		if lw > 3 {
			return fmt.Errorf("too many lost acknowledged writes (%d)", lw)
		}
	}
	return fmt.Errorf("write not visible after 3 verified attempts")
}

// Idempotent-operation helpers: the live service resets connections now and then (measured: PUT, DELETE, PROPFIND). A reset reply says nothing about
// whether the server applied the operation, so a retry can meet the effect of the first attempt; that outcome ("already exists" 405 for MKCOL, "gone"
// 404 for DELETE) is accepted ONLY after an earlier attempt of the same call failed transiently.
func (r *yRun) retryIdem(what string, fn func() error, benign string) error {
	prevFailed := false
	return r.retry(what, func() error {
		err := fn()
		if err != nil && prevFailed && strings.Contains(err.Error(), benign) {
			prevFailed = false
			return nil
		}
		prevFailed = err != nil
		return err
	})
}

func (r *yRun) mkdir(ctx context.Context, p string) error {
	return r.retryIdem("mkdir", func() error { return r.c.CreateDirectory(ctx, p) }, "status 405 for directory")
}
func (r *yRun) delFile(ctx context.Context, p string) error {
	return r.retryIdem("delete", func() error { return r.c.DeleteFile(ctx, p) }, "status 404")
}
func (r *yRun) delDir(ctx context.Context, p string) error {
	return r.retryIdem("delete_dir", func() error { return r.c.DeleteDirectory(ctx, p) }, "status 404")
}
func (r *yRun) copy(ctx context.Context, a, b string) error {
	return r.retry("copy", func() error { return r.c.CopyFile(ctx, a, b) })
}
func (r *yRun) list(ctx context.Context, p string) (ents []*FileInfo, err error) {
	err = r.retry("list", func() error { var e error; ents, e = r.c.ListDirectory(ctx, p); return e })
	return
}
func (r *yRun) info(ctx context.Context, p string) (fi *FileInfo, err error) {
	err = r.retry("info", func() error { var e error; fi, e = r.c.GetFileInfo(ctx, p); return e })
	return
}
func (r *yRun) exists(ctx context.Context, p string) (ok bool, err error) {
	err = r.retry("exists", func() error { var e error; ok, e = r.c.FileExists(ctx, p); return e })
	return
}

func (r *yRun) readAll(ctx context.Context, rel string) []byte {
	r.t.Helper()
	var b []byte
	err := r.retry("read", func() error {
		rc, err := r.c.ReadFile(ctx, rel)
		if err != nil {
			return err
		}
		defer rc.Close()
		b, err = io.ReadAll(rc)
		return err
	})
	if err != nil {
		r.t.Fatalf("read %s: %s", rel, rd(err))
	}
	return b
}

func (r *yRun) mustWrite(ctx context.Context, rel string, data []byte) {
	r.t.Helper()
	if err := r.retry("write", func() error { return r.c.WriteFile(ctx, rel, bytes.NewReader(data)) }); err != nil {
		r.t.Fatalf("write %s: %s", rel, rd(err))
	}
}

// bootstrapSandbox makes sure /Testing exists (creating only that one directory when missing).
func (r *yRun) bootstrapSandbox() {
	ctx, cancel := r.ctx(30 * time.Second)
	defer cancel()
	_ = ctx
	var resp *http.Response
	var err error
	for attempt := 1; attempt <= 3; attempt++ { // a slow first answer is retried before it is called unavailable
		q, _ := http.NewRequestWithContext(ctx, "PROPFIND", r.d.url+yandexSandboxRoot, nil)
		q.SetBasicAuth(r.d.user, r.d.pass)
		q.Header.Set("Depth", "0")
		resp, err = (&http.Client{Transport: r.gt, Timeout: 30 * time.Second}).Do(q)
		if err == nil {
			break
		}
		r.note(fmt.Sprintf("bootstrap_attempt_%d_error", attempt), rd(err))
		ctx2, c2 := r.ctx(30 * time.Second)
		ctx = ctx2
		defer c2()
	}
	if err != nil {
		r.t.Skipf("SKIP(service_unavailable): WebDAV endpoint unreachable after 3 attempts (%s)", rd(err))
	}
	resp.Body.Close()
	switch resp.StatusCode {
	case 207:
		r.note("sandbox_preexisting", true)
	case 404:
		q, _ := http.NewRequestWithContext(ctx, "MKCOL", r.d.url+yandexSandboxRoot, nil)
		q.SetBasicAuth(r.d.user, r.d.pass)
		m, err := (&http.Client{Transport: r.gt, Timeout: 30 * time.Second}).Do(q)
		if err != nil {
			r.t.Fatalf("mkcol %s: %s", yandexSandboxRoot, rd(err))
		}
		m.Body.Close()
		if m.StatusCode != 201 {
			r.t.Fatalf("create %s: status %d", yandexSandboxRoot, m.StatusCode)
		}
		r.note("sandbox_preexisting", false)
	case 401, 403:
		r.t.Fatalf("credential rejected by the real service on the sandbox probe: status %d", resp.StatusCode)
	default:
		r.t.Fatalf("sandbox probe: unexpected status %d", resp.StatusCode)
	}
}

func TestLiveYandex_Suite(t *testing.T) {
	r := loadYandex(t)
	r.bootstrapSandbox()
	r.c = r.newClient(r.d.pass)
	defer r.writeEvidence()

	ctx, cancel := r.ctx(25 * time.Minute)
	defer cancel()

	r.step("01_connect_auth_success", func(t *testing.T, note func(string, any)) {
		if err := r.retry("connect", func() error { return r.c.Connect(ctx) }); err != nil {
			t.Fatalf("connect: %s", rd(err))
		}
		if !r.c.IsConnected() {
			t.Fatalf("not connected after Connect")
		}
		if err := r.retry("test_connection", func() error { return r.c.TestConnection(ctx) }); err != nil {
			t.Fatalf("TestConnection: %s", rd(err))
		}
		note("protocol", r.c.GetProtocol())
	})
	if r.c == nil || !r.c.IsConnected() {
		return
	}
	defer r.c.Disconnect(ctx)

	// cleanup: delete ONLY the run directory this process created
	defer func() {
		if !r.created {
			return
		}
		cc, cf := r.ctx(60 * time.Second)
		defer cf()
		if err := r.delDir(cc, "/"+r.run); err != nil {
			t.Errorf("cleanup of own run dir failed: %s", rd(err))
			return
		}
		// Yandex deletes collections asynchronously at times: poll until gone
		for i := 0; i < 20; i++ {
			if ok, err := r.c.FileExists(cc, "/"+r.run+"/"); err == nil && !ok {
				if resp, err := r.raw(cc, "PROPFIND", "/"+r.run, map[string]string{"Depth": "0"}, nil); err == nil {
					resp.Body.Close()
					if resp.StatusCode == 404 {
						r.note("cleanup", "run dir removed, PROPFIND 404")
						return
					}
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		t.Errorf("run dir still present after cleanup")
	}()

	r.step("02_auth_failure_wrong_password", func(t *testing.T, note func(string, any)) {
		bad := r.newClient(wrongPasswordConstant)
		// The service sometimes stalls a PROPFIND past the 30 s metadata cap (observed in 2 of 3 runs): that is no verdict on the credential, so the
		// attempt is repeated (and counted) until the service gives a definitive answer.
		err := r.retry("bad_connect", func() error { return bad.Connect(ctx) })
		if err == nil {
			t.Fatalf("connect with a wrong password must fail")
		}
		if !strings.Contains(err.Error(), "401") {
			t.Fatalf("expected a clean 401 error, got: %s", rd(err))
		}
		if bad.IsConnected() {
			t.Fatalf("client connected despite rejected credential")
		}
		if _, err := bad.ReadFile(ctx, "/x"); err == nil || !strings.Contains(err.Error(), "not connected") {
			t.Fatalf("operation on unauthenticated client must say not connected, got %v", err)
		}
		if strings.Contains(err.Error(), wrongPasswordConstant) || strings.Contains(rd(err), r.d.pass) {
			t.Fatalf("password leaked in error")
		}
		note("status", 401)
	})

	r.step("03_mkdir", func(t *testing.T, note func(string, any)) {
		if err := r.mkdir(ctx, "/"+r.run); err != nil {
			t.Fatalf("mkdir run dir: %s", rd(err))
		}
		r.created = true
		if err := r.mkdir(ctx, r.rel("sub")); err != nil {
			t.Fatalf("mkdir sub: %s", rd(err))
		}
		err := r.c.CreateDirectory(ctx, r.rel("sub"))
		if err == nil {
			t.Fatalf("mkdir on an existing collection must fail")
		}
		note("mkdir_existing_error", rd(err))
		if err := r.c.CreateDirectory(ctx, r.rel("no/such/parent")); err == nil {
			t.Fatalf("mkdir with missing parent must fail")
		} else {
			note("mkdir_missing_parent_error", rd(err))
		}
	})
	if !r.created {
		return
	}

	type fx struct {
		name string
		n    int
	}
	files := []fx{{"small.bin", 4096}, {"one_mb.bin", 1 << 20}, {"two_mb.bin", 2 << 20}}
	want := map[string]string{}
	for _, f := range files {
		want[f.name] = shaHex(detBytes(f.name, f.n))
	}

	r.step("04_upload_known_sha256", func(t *testing.T, note func(string, any)) {
		for _, f := range files {
			t0 := time.Now()
			r.mustWrite(ctx, r.rel(f.name), detBytes(f.name, f.n))
			note("upload_ms_"+f.name, time.Since(t0).Milliseconds())
			note("sha256_"+f.name, want[f.name])
		}
	})

	r.step("05_list_sizes_mtimes", func(t *testing.T, note func(string, any)) {
		ents, err := r.list(ctx, "/"+r.run)
		if err != nil {
			t.Fatalf("list: %s", rd(err))
		}
		got := map[string]*FileInfo{}
		for _, e := range ents {
			got[e.Name] = e
		}
		for _, f := range files {
			e := got[f.name]
			if e == nil {
				t.Fatalf("%s missing from listing (%d entries)", f.name, len(ents))
			}
			if e.Size != int64(f.n) || e.IsDir {
				t.Fatalf("%s: size %d want %d isDir=%v", f.name, e.Size, f.n, e.IsDir)
			}
			if e.ModTime.IsZero() {
				t.Fatalf("%s: zero mtime", f.name)
			}
			skew := time.Since(e.ModTime)
			if skew < -15*time.Minute || skew > 15*time.Minute {
				t.Fatalf("%s: mtime %v implausible (skew %v)", f.name, e.ModTime, skew)
			}
			note("mtime_skew_s_"+f.name, int(skew.Seconds()))
		}
		if s := got["sub"]; s == nil || !s.IsDir {
			t.Fatalf("sub directory missing or not a directory in listing")
		}
		if len(ents) != 4 {
			t.Fatalf("expected 4 entries (3 files + sub), got %d", len(ents))
		}
	})

	r.step("06_download_sha256_and_stat", func(t *testing.T, note func(string, any)) {
		for _, f := range files {
			b := r.readAll(ctx, r.rel(f.name))
			if len(b) != f.n || shaHex(b) != want[f.name] {
				t.Fatalf("%s: download differs (len %d, sha %s)", f.name, len(b), shaHex(b))
			}
			fi, err := r.info(ctx, r.rel(f.name))
			if err != nil || fi.Size != int64(f.n) || fi.ModTime.IsZero() {
				t.Fatalf("%s: stat wrong (%v, %+v)", f.name, err, fi)
			}
			if ok, err := r.exists(ctx, r.rel(f.name)); err != nil || !ok {
				t.Fatalf("exists: %v %v", ok, err)
			}
		}
	})

	r.step("07_range_partial_read", func(t *testing.T, note func(string, any)) {
		// WebDAVClient has no ranged read API (SeekableClient is not implemented): the support of the SERVICE is probed with a raw guarded GET.
		resp, err := r.raw(ctx, "GET", r.rel("one_mb.bin"), map[string]string{"Range": "bytes=1000-1999"}, nil)
		if err != nil {
			t.Fatalf("range get: %s", rd(err))
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		full := detBytes("one_mb.bin", 1<<20)
		note("range_status", resp.StatusCode)
		note("accept_ranges", resp.Header.Get("Accept-Ranges"))
		switch resp.StatusCode {
		case 206:
			if shaHex(b) != shaHex(full[1000:2000]) {
				t.Fatalf("206 body differs from the expected slice")
			}
			note("range_supported", true)
		case 200:
			if shaHex(b) != want["one_mb.bin"] {
				t.Fatalf("200 body differs from the full file")
			}
			note("range_supported", false)
			t.Logf("service ignored Range (full body returned); WebDAVClient has no ranged read either")
		default:
			t.Fatalf("range get: unexpected status %d", resp.StatusCode)
		}
	})

	r.step("08_copy_rename_move", func(t *testing.T, note func(string, any)) {
		if err := r.copy(ctx, r.rel("small.bin"), r.rel("copy.bin")); err != nil {
			t.Fatalf("copy: %s", rd(err))
		}
		if shaHex(r.readAll(ctx, r.rel("copy.bin"))) != want["small.bin"] {
			t.Fatalf("copy content differs")
		}
		// copy over an existing destination (Overwrite defaults to T)
		if err := r.copy(ctx, r.rel("one_mb.bin"), r.rel("copy.bin")); err != nil {
			note("copy_over_existing_error", rd(err))
		} else if shaHex(r.readAll(ctx, r.rel("copy.bin"))) != want["one_mb.bin"] {
			t.Fatalf("copy-over-existing did not replace the content")
		} else {
			note("copy_over_existing", "replaced")
		}
		// MOVE (rename): the client has no MOVE method, so the service is probed with a raw guarded MOVE
		dst := r.d.url + (&url.URL{Path: yandexSandboxRoot + r.rel("renamed.bin")}).EscapedPath()
		resp, err := r.raw(ctx, "MOVE", r.rel("copy.bin"), map[string]string{"Destination": dst}, nil)
		if err != nil {
			t.Fatalf("move: %s", rd(err))
		}
		resp.Body.Close()
		note("move_status", resp.StatusCode)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			t.Fatalf("move status %d", resp.StatusCode)
		}
		if ok, _ := r.c.FileExists(ctx, r.rel("copy.bin")); ok {
			t.Fatalf("source still present after MOVE")
		}
		if shaHex(r.readAll(ctx, r.rel("renamed.bin"))) != want["one_mb.bin"] {
			t.Fatalf("moved content differs")
		}
		// copy of a missing source is a clean error
		if err := r.c.CopyFile(ctx, r.rel("missing.bin"), r.rel("x.bin")); err == nil {
			t.Fatalf("copy of a missing source must fail")
		} else {
			note("copy_missing_error", rd(err))
		}
	})

	r.step("09_overwrite", func(t *testing.T, note func(string, any)) {
		p := r.rel("overwrite.bin")
		r.mustWrite(ctx, p, detBytes("ow1", 3000))
		r.mustWrite(ctx, p, detBytes("ow2", 7000))
		b := r.readAll(ctx, p)
		if shaHex(b) != shaHex(detBytes("ow2", 7000)) {
			t.Fatalf("overwrite did not replace the content")
		}
		if fi, err := r.info(ctx, p); err != nil || fi.Size != 7000 {
			t.Fatalf("size after overwrite: %v %+v", err, fi)
		}
	})

	r.step("10_nonexistent_errors", func(t *testing.T, note func(string, any)) {
		m := r.rel("does-not-exist.bin")
		if _, err := r.c.ReadFile(ctx, m); err == nil || !strings.Contains(err.Error(), "status 404") {
			t.Fatalf("ReadFile missing: want status 404, got %v", err)
		}
		if _, err := r.c.GetFileInfo(ctx, m); err == nil || !strings.Contains(err.Error(), "status 404") {
			t.Fatalf("GetFileInfo missing: got %v", err)
		}
		if ok, err := r.c.FileExists(ctx, m); err != nil || ok {
			t.Fatalf("FileExists missing: %v %v", ok, err)
		}
		if _, err := r.c.ListDirectory(ctx, r.rel("no-such-dir")); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("ListDirectory missing: got %v", err)
		}
		if err := r.c.DeleteFile(ctx, m); err == nil || !strings.Contains(err.Error(), "404") {
			t.Fatalf("DeleteFile missing: got %v", err)
		}
		if err := r.c.DeleteDirectory(ctx, r.rel("no-such-dir")); err == nil {
			t.Fatalf("DeleteDirectory missing must fail")
		}
		if err := r.c.WriteFile(ctx, r.rel("no-parent/x.bin"), bytes.NewReader([]byte("x"))); err == nil {
			t.Fatalf("write into a missing parent must fail")
		} else {
			note("write_missing_parent_error", rd(err))
		}
	})

	r.step("11_unicode_and_special_names", func(t *testing.T, note func(string, any)) {
		if err := r.mkdir(ctx, r.rel("каталог 日本")); err != nil {
			t.Fatalf("mkdir unicode dir: %s", rd(err))
		}
		names := []string{"тест файл ёЖ.txt", "日本語 ファイル.txt", "ünï-cödé.txt", "emoji-\U0001F600.txt", "sp ace & plus+eq=.txt", "hash#pct%qm?.txt", "каталог 日本/вложенный файл.txt"}
		for _, n := range names {
			data := detBytes("u:"+n, 1500)
			if err := r.retry("write", func() error { return r.c.WriteFile(ctx, r.rel(n), bytes.NewReader(data)) }); err != nil {
				note("write_error_"+n, rd(err))
				t.Errorf("write %q: %s", n, rd(err))
				continue
			}
			if got := r.readAll(ctx, r.rel(n)); shaHex(got) != shaHex(data) {
				t.Errorf("%q: content differs after round trip", n)
			}
		}
		ents, err := r.list(ctx, "/"+r.run)
		if err != nil {
			t.Fatalf("list: %s", rd(err))
		}
		have := map[string]bool{}
		for _, e := range ents {
			have[e.Name] = true
		}
		for _, n := range names[:6] {
			if !have[n] {
				t.Errorf("%q not listed under its own name (listing names differ)", n)
			}
		}
		sub, err := r.list(ctx, r.rel("каталог 日本"))
		if err != nil || len(sub) != 1 || sub[0].Name != "вложенный файл.txt" {
			t.Errorf("unicode sub-directory listing wrong: %v %d", err, len(sub))
		}
	})

	r.step("12_large_listing_200", func(t *testing.T, note func(string, any)) {
		if err := r.mkdir(ctx, r.rel("many")); err != nil {
			t.Fatalf("mkdir many: %s", rd(err))
		}
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		errs := make(chan error, 200)
		for i := 0; i < 200; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				n := fmt.Sprintf("many/f%03d.txt", i)
				if err := r.writeVerified(ctx, n, detBytes(n, 16)); err != nil {
					errs <- fmt.Errorf("%s: %s", n, rd(err))
				}
			}(i)
		}
		wg.Wait()
		close(errs)
		for e := range errs {
			t.Errorf("upload: %v", e)
		}
		t0 := time.Now()
		ents, err := r.list(ctx, r.rel("many"))
		if err != nil {
			t.Fatalf("list many: %s", rd(err))
		}
		note("list_ms", time.Since(t0).Milliseconds())
		if len(ents) != 200 {
			t.Fatalf("listing has %d entries, want 200", len(ents))
		}
		seen := map[string]bool{}
		for _, e := range ents {
			if e.Size != 16 || e.IsDir {
				t.Errorf("%s: size %d dir=%v", e.Name, e.Size, e.IsDir)
			}
			seen[e.Name] = true
		}
		for i := 0; i < 200; i++ {
			if !seen[fmt.Sprintf("f%03d.txt", i)] {
				t.Errorf("f%03d.txt missing", i)
			}
		}
		note("entries", len(ents))
	})

	r.step("13_concurrent_uploads_4", func(t *testing.T, note func(string, any)) {
		if err := r.mkdir(ctx, r.rel("conc")); err != nil {
			t.Fatalf("mkdir conc: %s", rd(err))
		}
		const n = 8
		var wg sync.WaitGroup
		sem := make(chan struct{}, 4)
		var mu sync.Mutex
		var failed []string
		for i := 0; i < n; i++ {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				name := fmt.Sprintf("conc/c%d.bin", i)
				if err := r.retry("write", func() error { return r.c.WriteFile(ctx, r.rel(name), bytes.NewReader(detBytes(name, 256<<10))) }); err != nil {
					mu.Lock()
					failed = append(failed, name+": "+rd(err))
					mu.Unlock()
				}
			}(i)
		}
		wg.Wait()
		if len(failed) > 0 {
			t.Fatalf("concurrent uploads failed: %v", failed)
		}
		for i := 0; i < n; i++ {
			name := fmt.Sprintf("conc/c%d.bin", i)
			if shaHex(r.readAll(ctx, r.rel(name))) != shaHex(detBytes(name, 256<<10)) {
				t.Errorf("%s differs after concurrent upload", name)
			}
		}
		note("files", n)
		note("max_parallel", 4)
	})

	r.step("14_unsized_reader_upload", func(t *testing.T, note func(string, any)) {
		// A reader whose length the HTTP stack cannot know (os.File, pipe, io.LimitReader) is sent chunked. Probe what the real service does.
		data := detBytes("unsized", 100000)
		err := r.c.WriteFile(ctx, r.rel("unsized.bin"), io.LimitReader(bytes.NewReader(data), int64(len(data))))
		if err != nil {
			note("unsized_upload_error", rd(err))
			t.Errorf("DEFECT-CANDIDATE: upload from an unsized reader failed against the real service: %s", rd(err))
			return
		}
		if shaHex(r.readAll(ctx, r.rel("unsized.bin"))) != shaHex(data) {
			t.Fatalf("unsized upload stored different content")
		}
		note("unsized_upload", "ok (chunked accepted)")
	})

	r.step("15_timeout_and_cancel", func(t *testing.T, note func(string, any)) {
		big := detBytes("two_mb.bin", 2<<20)
		// (a) deadline far below what a 2 MB upload needs
		tctx, tcancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
		t0 := time.Now()
		err := r.c.WriteFile(tctx, r.rel("timeout.bin"), bytes.NewReader(big))
		tcancel()
		el := time.Since(t0)
		if err == nil {
			t.Fatalf("upload under a 30 ms deadline cannot succeed")
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error must wrap context.DeadlineExceeded, got %s", rd(err))
		}
		if el > 10*time.Second {
			t.Fatalf("deadline not honoured promptly: %v", el)
		}
		note("deadline_return_ms", el.Milliseconds())
		// the interrupted upload must not leave a truncated file visible
		time.Sleep(1 * time.Second)
		if fi, err := r.c.GetFileInfo(ctx, r.rel("timeout.bin")); err == nil {
			if fi.Size != int64(len(big)) {
				t.Errorf("DEFECT-CANDIDATE: truncated file of %d bytes left visible after an interrupted upload", fi.Size)
			}
			note("interrupted_upload_left_file_size", fi.Size)
		} else {
			note("interrupted_upload_left_file", false)
		}
		// (b) already-cancelled context
		cctx, ccancel := context.WithCancel(context.Background())
		ccancel()
		if _, err := r.c.ReadFile(cctx, r.rel("two_mb.bin")); !errors.Is(err, context.Canceled) {
			t.Fatalf("read with a cancelled context must wrap context.Canceled, got %v", err)
		}
		// (c) cancel in the middle of a download body
		mctx, mcancel := context.WithCancel(context.Background())
		rc, err := r.c.ReadFile(mctx, r.rel("two_mb.bin"))
		if err != nil {
			mcancel()
			t.Fatalf("open: %s", rd(err))
		}
		buf := make([]byte, 64<<10)
		if _, err := io.ReadFull(rc, buf); err != nil {
			mcancel()
			t.Fatalf("first chunk: %v", err)
		}
		mcancel()
		_, err = io.Copy(io.Discard, rc)
		rc.Close()
		if err == nil {
			t.Errorf("DEFECT-CANDIDATE: body read after cancel completed without error")
		} else {
			note("mid_body_cancel_error", rd(err))
		}
		// (d) the client is still usable afterwards
		if _, err := r.info(ctx, r.rel("small.bin")); err != nil {
			t.Fatalf("client unusable after cancel: %s", rd(err))
		}
	})

	r.step("16_delete", func(t *testing.T, note func(string, any)) {
		for _, n := range []string{"small.bin", "overwrite.bin", "renamed.bin"} {
			if err := r.delFile(ctx, r.rel(n)); err != nil {
				t.Fatalf("delete %s: %s", n, rd(err))
			}
			if ok, err := r.exists(ctx, r.rel(n)); err != nil || ok {
				t.Fatalf("%s still present after delete (%v %v)", n, ok, err)
			}
		}
		if err := r.delDir(ctx, r.rel("conc")); err != nil {
			t.Fatalf("delete conc dir (recursive): %s", rd(err))
		}
	})

	r.step("17_guard_refuses_outside_sandbox", func(t *testing.T, note func(string, any)) {
		// the guard must refuse BEFORE any byte leaves the process; the client's own ".." confinement is exercised too
		for _, p := range []string{"/../etc", "/../../", "/.."} {
			_, err := r.c.ListDirectory(ctx, p)
			// ConfinePath maps these inside the sandbox; either a clean error or a listing of the sandbox is fine, but never a guard violation
			_ = err
		}
		if _, err := r.raw(ctx, "PROPFIND", "/../Other", map[string]string{"Depth": "0"}, nil); err == nil || !strings.Contains(err.Error(), "GUARD") {
			t.Fatalf("raw request escaping the sandbox must be refused by the guard, got %v", err)
		}
		dst := r.d.url + "/Other/x"
		if _, err := r.raw(ctx, "MOVE", r.rel("conc-none"), map[string]string{"Destination": dst}, nil); err == nil || !strings.Contains(err.Error(), "GUARD") {
			t.Fatalf("MOVE with a Destination outside the sandbox must be refused by the guard, got %v", err)
		}
		r.gt.mu.Lock()
		tv := r.gt.violations
		r.gt.mu.Unlock()
		if tv != 2 {
			t.Fatalf("guard violations after step 17 = %d, want exactly 2 (the two deliberate probes); any other value means an escaping request was attempted", tv)
		}
	})

	r.gt.mu.Lock()
	v := r.gt.violations
	r.gt.mu.Unlock()
	r.note("guard_violations_after_suite_expected_2_plus_client_confine", v)
}
