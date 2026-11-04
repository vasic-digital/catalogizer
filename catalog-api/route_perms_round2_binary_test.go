package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Authz round 2 (branch fix-authz-round2). Operator decisions:
//   - /metrics and /debug/pprof/* require an administrator;
//   - endpoints that make the server call outbound services (metadata enrichment,
//     cover-art discovery, asset resolution, subtitle search/download/translate) are
//     admin-only and their fetches are SSRF-guarded (internal/netguard);
//   - GET /scans* stays open to any authenticated user.
// Real binary, real router, real SQLCipher DB, real register + login.

var round2ObservabilityCases = []permCase{
	{"GET", "/metrics", ""},
	{"GET", "/debug/pprof/", ""},
	{"GET", "/debug/pprof/cmdline", ""},
	{"GET", "/debug/pprof/heap", ""},
	{"GET", "/debug/pprof/goroutine", ""},
	{"GET", "/debug/pprof/allocs", ""},
	{"GET", "/debug/pprof/symbol", ""},
	{"POST", "/debug/pprof/symbol", ""},
}

// Outbound-capable triggers: admin only (perm "" = admin).
var round2OutboundCases = []permCase{
	{"POST", "/api/v1/entities/enrich", ""},
	{"POST", "/api/v1/entities/1/metadata/refresh", ""},
	{"POST", "/api/v1/assets/request", ""},
	{"GET", "/api/v1/subtitles/search?media_id=1&language=en", ""},
	{"POST", "/api/v1/subtitles/download", ""},
	{"POST", "/api/v1/subtitles/translate", ""},
}

func TestRoutePerms_RealBinary_Round2_ObservabilityAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, _, stop := startPermServerDir(t, t.TempDir(), t.TempDir(), "HELIX_PPROF_ENABLED=true")
	defer stop()
	plain := selfRegister(t, c, "r2obs")
	for _, r := range round2ObservabilityCases {
		name := r.method + " " + r.path
		code, _ := c.do(r.method, r.path, "", map[string]interface{}{})
		assert.Equal(t, 401, code, "anonymous "+name)
		code, _ = c.do(r.method, r.path, "not-a-token", map[string]interface{}{})
		assert.Equal(t, 401, code, "garbage token "+name)
		code, _ = c.do(r.method, r.path, plain, map[string]interface{}{})
		assert.Equal(t, 403, code, "self-registered user "+name)
		code, _ = c.do(r.method, r.path, adm, map[string]interface{}{})
		assert.Equal(t, 200, code, "admin must reach "+name)
	}
	// Positive control that the admin response is the real payload, not an empty 200.
	code, body := c.do("GET", "/metrics", adm, nil)
	require.Equal(t, 200, code)
	assert.Contains(t, string(body), "go_goroutines", "admin /metrics must return the Prometheus exposition")
	// Health stays public (load balancers, QA probes).
	code, _ = c.do("GET", "/health", "", nil)
	assert.Equal(t, 200, code)
}

func TestRoutePerms_RealBinary_Round2_OutboundTriggersAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "r2out")
	holder := func(name, perm string) string {
		role := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "r2-" + name, "permissions": []string{perm}}, 201)
		c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": "r2u" + name, "email": "r2u" + name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": role}, 201)
		return c.login("r2u"+name, "Contract-Value-Strong-77")
	}
	// holders of unrelated management permissions must not open outbound triggers either
	stor := holder("stor", "storage.manage")
	cfg := holder("cfg", "system.configure")
	for _, r := range round2OutboundCases {
		name := r.method + " " + r.path
		code, _ := c.do(r.method, r.path, "", map[string]interface{}{})
		assert.Equal(t, 401, code, "anonymous "+name)
		for who, tok := range map[string]string{"plain": plain, "storage.manage": stor, "system.configure": cfg} {
			code, _ = c.do(r.method, r.path, tok, map[string]interface{}{})
			assert.Equal(t, 403, code, who+" must be refused on "+name)
		}
		code, _ = c.do(r.method, r.path, adm, map[string]interface{}{})
		assert.NotContains(t, []int{401, 403, 503}, code, "admin must reach "+name)
	}
}

// Decision: GET /scans* is read-only and stays open to any authenticated user.
func TestRoutePerms_RealBinary_Round2_ScansReadStayAuthOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "r2scan")
	for _, p := range []string{"/api/v1/scans", "/api/v1/scans/does-not-exist"} {
		code, _ := c.do("GET", p, "", nil)
		assert.Equal(t, 401, code, "anonymous GET "+p)
		code, _ = c.do("GET", p, plain, nil)
		assert.NotContains(t, []int{401, 403}, code, "plain user must keep GET "+p)
		code, _ = c.do("GET", p, adm, nil)
		assert.NotContains(t, []int{401, 403}, code, "admin GET "+p)
	}
	// ...while queueing stays admin-only.
	code, _ := c.do("POST", "/api/v1/scans", plain, map[string]interface{}{})
	assert.Equal(t, 403, code)
}

// NOTE (F5): the former binary test for POST /assets/request SSRF was removed: resolution
// is asynchronous and the endpoint exposes no observable that proves the resolver ran, so
// "no hit after a sleep" also passed on the unfixed code. The SSRF behaviour is covered
// deterministically at the service layer (internal/services/outbound_redirect_round2_test.go:
// TestExternalMetadataResolver_IgnoresOperatorAllowList and ..._RedirectToBlockedHostIsNotFollowed
// (the latter has a positive control that the resolver fetched; internal/netguard/strict_test.go has one for the strict dialer), and the route stays admin-gated
// in TestRoutePerms_RealBinary_Round2_OutboundTriggersAdminOnly.

// Any authenticated user may upload a subtitle; the body must be bounded (the handler
// read the whole upload into memory).
func TestRoutePerms_RealBinary_Round2_SubtitleUploadIsBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, _, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "r2up")
	post := func(size int) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField("media_item_id", "1")
		mw.WriteField("language", "English")
		mw.WriteField("language_code", "en")
		fw, _ := mw.CreateFormFile("file", "big.srt")
		fw.Write(bytes.Repeat([]byte("1\n00:00:01,000 --> 00:00:02,000\nx\n\n"), size/38+1))
		mw.Close()
		req, _ := http.NewRequest("POST", c.base+"/api/v1/subtitles/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+plain)
		r, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer r.Body.Close()
		return r.StatusCode
	}
	assert.NotEqual(t, 413, post(1024), "a small subtitle is not refused for size")
	assert.Equal(t, 413, post(6<<20), "a 6 MiB subtitle must be refused with 413")
}

// F4: the two caps are enforced independently. A 2.5 MiB file is under the 3 MiB request
// cap but over the 2 MiB file cap (file.Size check); a 3.5 MiB non-file field is over the
// request cap with a tiny file (http.MaxBytesReader / *http.MaxBytesError path).
func TestRoutePerms_RealBinary_Round2_SubtitleUploadCapsAreIndependent(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, _, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "r2cap")
	post := func(fileBytes, fieldBytes int) int {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		mw.WriteField("media_item_id", "1")
		mw.WriteField("language", "English")
		mw.WriteField("language_code", "en")
		if fieldBytes > 0 {
			mw.WriteField("padding", strings.Repeat("p", fieldBytes))
		}
		fw, _ := mw.CreateFormFile("file", "s.srt")
		fw.Write(bytes.Repeat([]byte("1\n00:00:01,000 --> 00:00:02,000\nx\n\n"), fileBytes/38+1))
		mw.Close()
		req, _ := http.NewRequest("POST", c.base+"/api/v1/subtitles/upload", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+plain)
		r, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer r.Body.Close()
		return r.StatusCode
	}
	assert.NotEqual(t, 413, post(1<<20, 0), "control: a 1 MiB subtitle is not refused for size")
	assert.Equal(t, 413, post(2<<20+512<<10, 0), "2.5 MiB file (between the caps) must be refused: file cap")
	assert.Equal(t, 413, post(1024, 3<<20+512<<10), "3.5 MiB non-file body must be refused: request cap")
}

// F1: Prometheus cannot log in, so /metrics also accepts one dedicated scrape
// credential (METRICS_SCRAPE_TOKEN, at least 32 characters). It opens GET /metrics only.
func TestRoutePerms_RealBinary_Round2_MetricsScrapeToken(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	const tok = "scrape-token-0123456789abcdefghijKLMN"
	c, adm, _, stop := startPermServerDir(t, t.TempDir(), t.TempDir(), "METRICS_SCRAPE_TOKEN="+tok)
	defer stop()
	plain := selfRegister(t, c, "r2scrape")
	code, body := c.do("GET", "/metrics", tok, nil)
	require.Equal(t, 200, code, "scrape token must read /metrics")
	assert.Contains(t, string(body), "go_goroutines")
	for _, r := range []permCase{{"GET", "/api/v1/catalog", ""}, {"GET", "/api/v1/auth/me", ""}, {"GET", "/api/v1/stats/overall", ""}, {"POST", "/api/v1/smb/probe", ""}, {"GET", "/health/deep", ""}} {
		if r.path == "/health/deep" {
			continue // public route: the token is irrelevant there
		}
		code, _ = c.do(r.method, r.path, tok, map[string]interface{}{})
		assert.Equal(t, 401, code, "scrape token must not authenticate "+r.method+" "+r.path)
	}
	code, _ = c.do("GET", "/metrics", tok+"x", nil)
	assert.Equal(t, 401, code, "wrong token")
	code, _ = c.do("GET", "/metrics", tok[:len(tok)-1], nil)
	assert.Equal(t, 401, code, "truncated token")
	code, _ = c.do("GET", "/metrics", plain, nil)
	assert.Equal(t, 403, code, "plain user")
	code, _ = c.do("GET", "/metrics", adm, nil)
	assert.Equal(t, 200, code, "admin JWT keeps working")
	code, _ = c.do("POST", "/metrics", tok, map[string]interface{}{})
	assert.NotEqual(t, 200, code, "token is for GET only")
}

// A configured token shorter than 32 characters is refused at start-up use: it is ignored (admin JWT only).
func TestRoutePerms_RealBinary_Round2_ShortScrapeTokenIgnored(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	const short = "short-token"
	c, _, _, stop := startPermServerDir(t, t.TempDir(), t.TempDir(), "METRICS_SCRAPE_TOKEN="+short)
	defer stop()
	code, _ := c.do("GET", "/metrics", short, nil)
	assert.Equal(t, 401, code, "a token under 32 characters must not be accepted")
}

// AZ2-R2: the scrape credential opens GET /metrics only; it must not open any pprof
// route (heap/goroutine/allocs/cmdline expose memory contents and the command line).
// pprof is enabled here so a 401 is the gate answering, not a missing route.
func TestRoutePerms_RealBinary_Round2_ScrapeTokenDoesNotOpenPprof(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	const tok = "scrape-token-0123456789abcdefghijKLMN"
	c, adm, _, stop := startPermServerDir(t, t.TempDir(), t.TempDir(), "HELIX_PPROF_ENABLED=true", "METRICS_SCRAPE_TOKEN="+tok)
	defer stop()
	// controls: the token opens /metrics, the admin JWT opens pprof (the routes exist)
	code, _ := c.do("GET", "/metrics", tok, nil)
	require.Equal(t, 200, code, "control: scrape token reads /metrics")
	code, _ = c.do("GET", "/debug/pprof/heap", adm, nil)
	require.Equal(t, 200, code, "control: admin reaches /debug/pprof/heap (route is enabled)")
	for _, r := range []permCase{
		{"GET", "/debug/pprof/", ""},
		{"GET", "/debug/pprof/heap", ""},
		{"GET", "/debug/pprof/goroutine", ""},
		{"GET", "/debug/pprof/allocs", ""},
		{"GET", "/debug/pprof/cmdline", ""},
		{"GET", "/debug/pprof/symbol", ""},
		{"POST", "/debug/pprof/symbol", ""},
	} {
		code, _ = c.do(r.method, r.path, tok, map[string]interface{}{})
		assert.Equal(t, 401, code, "scrape token must not authenticate "+r.method+" "+r.path)
	}
}
