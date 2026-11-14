package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 1 F3 / F4: server-side fetches must not be steerable at the server's own
// network. Each test starts a REAL listening "internal service" and asserts it is never
// contacted, so a refusal that merely happens for another reason cannot pass.

func internalService(t *testing.T) (*httptest.Server, *int32) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("INTERNAL-SECRET"))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestSSRF_RealBinary_ImageProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, _, stop := startPermServer(t, t.TempDir())
	defer stop()
	srv, hits := internalService(t)
	host := srv.Listener.Addr().String()
	bad := []string{
		// the reviewer's case: allow-listed name only in the query
		"http://" + host + "/internal?x=image.tmdb.org",
		"https://" + host + "/internal?x=image.tmdb.org",
		"http://" + host + "/image.tmdb.org",
		"https://image.tmdb.org.evil.example/x.jpg",
		"https://evil-image.tmdb.org/x.jpg",
		"https://image.tmdb.org@" + host + "/x.jpg",
		"http://image.tmdb.org/t/p/w500/x.jpg", // plain http
		"https://image.tmdb.org:8443/x.jpg",    // other port
		"file:///etc/passwd?x=image.tmdb.org",
	}
	for _, u := range bad {
		code, raw := c.do("GET", "/api/v1/image-proxy?url="+url.QueryEscape(u), "", nil)
		assert.Equal(t, 403, code, "%s must be refused: %s", u, raw)
		assert.NotContains(t, string(raw), "INTERNAL-SECRET")
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(hits), "the internal service must never be contacted")
	// an allow-listed https URL is still served (CDN answer or the offline placeholder)
	code, _ := c.do("GET", "/api/v1/image-proxy?url="+url.QueryEscape("https://image.tmdb.org/t/p/w500/none.jpg"), "", nil)
	assert.Equal(t, 200, code, "allow-listed host stays usable")
}

func TestSSRF_RealBinary_SyncEndpointURL(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	syncRoot := filepath.Join(dir, "syncroot")
	require.NoError(t, os.MkdirAll(filepath.Join(syncRoot, "local"), 0o755))
	c, adm, _, stop := startPermServerDir(t, dir, dir, "CATALOGIZER_SYNC_ROOTS="+syncRoot)
	defer stop()
	srv, hits := internalService(t)
	for i, u := range []string{
		srv.URL + "/dav",
		"http://127.0.0.1:1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/",
		"http://10.0.0.5/",
		"ftp://example.invalid/",
	} {
		for _, typ := range []string{"webdav", "cloud_storage"} {
			code, raw := c.do("POST", "/api/v1/sync/endpoints", adm, map[string]interface{}{
				"name": "ssrf", "type": typ, "url": u, "sync_direction": "upload",
				"local_path": filepath.Join(syncRoot, "local"), "remote_path": "/"})
			assert.Equal(t, 400, code, "case %d %s %s must be refused as an invalid URL: %s", i, typ, u, raw)
			// Since CAT-PXF23-R5B (197eec593) a 400 carries a fixed per-status class text, never the validator's own wording, so this
			// test can no longer tell WHICH validation refused (an earlier validation refusing every request would also pass; the attribution
			// to the URL check is carried by services/sync_service_netguard_test.go, which calls validateSyncEndpoint directly). The
			// zero-contact assertion below proves only that nothing was dialled, not which check refused. A positive control (a vetted URL
			// getting past validation) is a tracked follow-up: it needs a vetted public endpoint, which a hermetic test does not have.
			assert.Contains(t, string(raw), "invalid input", "a refusal must be the fixed 400 class text (no validator wording leaked): %s", raw)
			assert.NotContains(t, string(raw), "invalid endpoint URL", "the validator's own wording must not reach the client: %s", raw)
		}
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(hits), "the internal service must never be contacted")
}
