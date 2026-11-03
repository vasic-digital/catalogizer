//go:build live

package services

// LIVE scan of a REAL remote WebDAV tree (Yandex) through Catalogizer's real WebDAVClient and the real WebDAV GenericScanner (section 11.4.289:
// real service, no fixture server). Only the RECORD seam is replaced by a collector (the documented evidence/dry-run seam), so what is asserted is
// exactly what the scanner would have catalogued: paths, sizes, directory flags, and (by downloading each catalogued file through the client)
// the sha256 of the content. Everything stays under /Testing (guard transport); the test creates and removes only its own run directory.
//
// Environment: LIVE_WEBDAV_URL (https origin), LIVE_WEBDAV_USER, LIVE_WEBDAV_PASSWORD, LIVE_WEBDAV_SANDBOX (exactly /Testing),
// optional LIVE_YANDEX_SCAN_EVIDENCE (JSON evidence file). Without them: SKIP(credentials_absent).

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

	"catalogizer/filesystem"
	"catalogizer/models"

	"digital.vasic.security/pkg/ssrf"
	"go.uber.org/zap"
)

type scanGuard struct {
	base http.RoundTripper
	host string
	mu   sync.Mutex
	n    int
	viol int
}

func okSandboxPath(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\\x00") {
		return false
	}
	for _, s := range strings.Split(p, "/") {
		if s == ".." || s == "." {
			return false
		}
	}
	c := strings.TrimRight(p, "/")
	return c == "/Testing" || strings.HasPrefix(c, "/Testing/")
}

func (g *scanGuard) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Scheme != "https" || r.URL.Host != g.host || !okSandboxPath(r.URL.Path) {
		g.mu.Lock()
		g.viol++
		g.mu.Unlock()
		return nil, fmt.Errorf("GUARD: refused request outside /Testing")
	}
	g.mu.Lock()
	g.n++
	g.mu.Unlock()
	return g.base.RoundTrip(r)
}

func sha(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func det(label string, n int) []byte {
	out := make([]byte, 0, n+32)
	for i := 0; len(out) < n; i++ {
		h := sha256.Sum256([]byte(fmt.Sprintf("%s#%d", label, i)))
		out = append(out, h[:]...)
	}
	return out[:n]
}

func TestLiveYandexScan_RemoteTreeIsCatalogued(t *testing.T) {
	base, user, pass, sb := os.Getenv("LIVE_WEBDAV_URL"), os.Getenv("LIVE_WEBDAV_USER"), os.Getenv("LIVE_WEBDAV_PASSWORD"), os.Getenv("LIVE_WEBDAV_SANDBOX")
	if base == "" || user == "" || pass == "" || sb == "" {
		t.Skip("SKIP(credentials_absent): LIVE_WEBDAV_* not set")
	}
	u, err := url.Parse(base)
	if err != nil || u.Scheme != "https" {
		t.Fatalf("LIVE_WEBDAV_URL must be an https origin")
	}
	if strings.TrimRight(sb, "/") != "/Testing" {
		t.Fatalf("LIVE_WEBDAV_SANDBOX must be exactly /Testing")
	}
	redact := func(s string) string {
		for _, x := range []string{pass, user, u.Host} {
			if x != "" {
				s = strings.ReplaceAll(s, x, "<redacted>")
			}
		}
		return s
	}
	gt := &scanGuard{base: http.DefaultTransport.(*http.Transport).Clone(), host: u.Host}
	c, err := filesystem.NewWebDAVClient(&filesystem.WebDAVConfig{URL: strings.TrimRight(base, "/"), Username: user, Password: pass, Path: "/Testing"})
	if err != nil {
		t.Fatalf("client: %s", redact(err.Error()))
	}
	c.SetHTTPTransport(gt)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	if err := c.Connect(ctx); err != nil {
		t.Skipf("SKIP(service_unavailable): %s", redact(err.Error()))
	}
	defer c.Disconnect(ctx)

	// Setup calls retry a "connection reset by peer" (a service behaviour seen live, FOLLOW_UPS W9); every retry is counted in the evidence. An
	// "already exists" (405) after a failed earlier attempt means the first attempt was applied. Scan and verification steps are NOT retried.
	setupRetries := 0
	setup := func(fn func() error) error {
		var err error
		for a := 1; a <= 4; a++ {
			e := fn()
			if e != nil && err != nil && strings.Contains(e.Error(), "status 405") {
				return nil
			}
			err = e
			if err == nil || !strings.Contains(err.Error(), "connection reset") {
				return err
			}
			setupRetries++
			time.Sleep(time.Duration(a) * 2 * time.Second)
		}
		return err
	}
	run := fmt.Sprintf("catalogizer-live-scan-%d", time.Now().Unix())
	if err := setup(func() error { return c.CreateDirectory(ctx, "/"+run) }); err != nil {
		t.Fatalf("mkdir: %s", redact(err.Error()))
	}
	defer func() { // remove ONLY the run directory this test created
		cc, cf := context.WithTimeout(context.Background(), 60*time.Second)
		defer cf()
		if err := c.DeleteDirectory(cc, "/"+run); err != nil {
			t.Errorf("cleanup: %s", redact(err.Error()))
		}
	}()

	type f struct {
		rel  string
		size int
	}
	files := []f{{"a.bin", 3000}, {"sub/b.bin", 1500}, {"sub/deep/c.txt", 700}, {"каталог/файл ёЖ.txt", 900}, {"sub/日本語.bin", 1200}}
	for _, d := range []string{"sub", "sub/deep", "каталог"} {
		if err := setup(func() error { return c.CreateDirectory(ctx, "/"+run+"/"+d) }); err != nil {
			t.Fatalf("mkdir %s: %s", d, redact(err.Error()))
		}
	}
	want := map[string]string{}
	for _, x := range files {
		data := det("scan:"+x.rel, x.size)
		want[x.rel] = sha(data)
		if err := setup(func() error { return c.WriteFile(ctx, "/"+run+"/"+x.rel, bytes.NewReader(data)) }); err != nil {
			t.Fatalf("upload %s: %s", x.rel, redact(err.Error()))
		}
	}

	type rec struct {
		p     string
		size  int64
		isDir bool
	}
	var mu sync.Mutex
	var got []rec
	sc := NewWebDAVScanner(zap.NewNop()).WithRecorder(func(_ context.Context, p string, fi *filesystem.FileInfo, _ ScanJob, _ *ScanStatus) error {
		mu.Lock()
		got = append(got, rec{p: p, size: fi.Size, isDir: fi.IsDir})
		mu.Unlock()
		return nil
	})
	st := &ScanStatus{JobID: "live-scan", Protocol: "webdav"}
	job := ScanJob{ID: "live-scan", Path: "/" + run, MaxDepth: 10, ScanType: "full", Context: ctx, StorageRoot: &models.StorageRoot{Name: "yandex-live", Protocol: "webdav"}}
	t0 := time.Now()
	if err := sc.ScanPath(ctx, c, job, st); err != nil {
		t.Fatalf("scan: %s", redact(err.Error()))
	}
	scanMs := time.Since(t0).Milliseconds()

	byPath := map[string]rec{}
	for _, r := range got {
		byPath[strings.TrimPrefix(r.p, "/"+run+"/")] = r
	}
	var names []string
	for k := range byPath {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, x := range files {
		r, ok := byPath[x.rel]
		if !ok {
			t.Fatalf("catalogued set lacks %q; have %v", x.rel, names)
		}
		if r.isDir || r.size != int64(x.size) {
			t.Errorf("%s: catalogued size=%d dir=%v want %d", x.rel, r.size, r.isDir, x.size)
		}
		rc, err := c.ReadFile(ctx, "/"+run+"/"+x.rel)
		if err != nil {
			t.Fatalf("download %s: %s", x.rel, redact(err.Error()))
		}
		b, _ := io.ReadAll(rc)
		rc.Close()
		if sha(b) != want[x.rel] {
			t.Errorf("%s: content sha256 differs from the uploaded content", x.rel)
		}
	}
	for _, d := range []string{"sub", "sub/deep", "каталог"} {
		if r, ok := byPath[d]; !ok || !r.isDir {
			t.Errorf("directory %q not catalogued as a directory (%+v)", d, r)
		}
	}
	if len(got) != len(files)+3 {
		t.Errorf("catalogued %d entries, want %d (files+dirs only, nothing from outside the run dir): %v", len(got), len(files)+3, names)
	}
	snap := st.GetSnapshot()
	if snap.ErrorCount != 0 {
		t.Errorf("scan reported %d errors", snap.ErrorCount)
	}

	// SSRF policy, evaluated by the canonical guard itself (digital.vasic.security/pkg/ssrf, real DNS, no resolver stub): a legitimate public https
	// WebDAV host must pass, loopback must still be refused. (This package's TestMain may relax services.GuardProviderURL for httptest loopback
	// servers, so the canonical guard is called directly; the services adapter is checked for the public host only.) On main the WebDAV storage-root
	// path itself has no outbound guard; that is OPEN W5 in docs/FOLLOW_UPS_NEXT_PHASE.md (section "Live Yandex WebDAV suite").
	publicAllowed, loopbackRefused, adapterAllowed := true, true, true
	if err := ssrf.Validate(strings.TrimRight(base, "/"), ssrf.Config{}); err != nil {
		publicAllowed = false
		t.Errorf("SSRF guard blocked a legitimate public https WebDAV host: %v", err)
	}
	if err := ssrf.Validate("https://127.0.0.1/Testing", ssrf.Config{}); err == nil {
		loopbackRefused = false
		t.Errorf("SSRF guard must still refuse loopback")
	}
	if err := GuardProviderURL(strings.TrimRight(base, "/"), SSRFGuardConfig{}); err != nil {
		adapterAllowed = false
		t.Errorf("services.GuardProviderURL blocked the public host: %v", err)
	}
	// The sandbox guard transport must never have refused a request of the scan itself (the scan only touches /Testing/<run dir>).
	if gt.viol != 0 {
		t.Errorf("sandbox guard violations during the scan = %d, want 0", gt.viol)
	}

	if path := os.Getenv("LIVE_YANDEX_SCAN_EVIDENCE"); path != "" {
		sums := map[string]string{}
		for k, v := range want {
			sums[k] = v
		}
		ev := map[string]any{"service": "webdav.yandex.ru", "run_dir": run, "date_utc": time.Now().UTC().Format(time.RFC3339), "scan_ms": scanMs,
			"entries_catalogued": len(got), "names": names, "sha256": sums, "http_requests": gt.n, "guard_violations": gt.viol,
			"scan_errors": snap.ErrorCount, "setup_retries": setupRetries, "ssrf_public_https_allowed": publicAllowed, "ssrf_loopback_refused": loopbackRefused, "ssrf_adapter_public_allowed": adapterAllowed,
			"seam": "remote tree is live; the database record sink is an in-memory collector (insertFileRecord DB path and UniversalScanner job path not exercised, FOLLOW_UPS W8)", "credentials_in_evidence": false}
		b, _ := json.MarshalIndent(ev, "", "  ")
		if err := os.WriteFile(path, b, 0o600); err != nil {
			t.Errorf("evidence: %v", err)
		}
	}
}
