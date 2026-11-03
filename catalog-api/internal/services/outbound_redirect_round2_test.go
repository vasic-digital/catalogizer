package services

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"

	"digital.vasic.assets/pkg/asset"
	"digital.vasic.assets/pkg/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Authz round 2: URL-guard-then-plain-client lets a redirect (or a name that
// re-resolves) reach hosts the guard never saw. The fetches below go through
// internal/netguard, whose dialer checks and pins every connection.
//
// Topology: "public" A on 127.0.0.1 (the operator allowed 127.0.0.1/32, as a stand-in
// for a legitimate remote host) answers a redirect to B on 127.0.0.2 (not allowed).
func redirectTopology(t *testing.T) (aURL string, bHits *int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	var hits int32
	b := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("INTERNAL-SECRET"))
	}))
	b.Listener = ln
	b.Start()
	t.Cleanup(b.Close)
	a := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Header().Set("Content-Type", "image/jpeg")
			w.Write([]byte("public-bytes"))
			return
		}
		http.Redirect(w, r, b.URL+"/internal", http.StatusFound)
	}))
	t.Cleanup(a.Close)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	return a.URL, &hits
}

func TestExternalMetadataResolver_RedirectToBlockedHostIsNotFollowed(t *testing.T) {
	aURL, bHits := redirectTopology(t)
	r := NewExternalMetadataResolver(nil, 2)
	r.client = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS) // non-strict seam: the redirect guard is under test
	req := func(path string) *resolver.ResolveRequest {
		return &resolver.ResolveRequest{AssetID: asset.NewID(), SourceHint: aURL + path, EntityType: "file", EntityID: "1"}
	}
	// positive control: the allowed host is fetched
	res, err := r.Resolve(context.Background(), req("/ok"))
	require.NoError(t, err)
	body, _ := io.ReadAll(res.Content)
	res.Content.Close()
	assert.Equal(t, "public-bytes", string(body))
	// redirect to the non-allowed host
	res, err = r.Resolve(context.Background(), req("/redir"))
	if res != nil {
		res.Content.Close()
	}
	assert.Zero(t, atomic.LoadInt32(bHits), "the redirect target must never be contacted")
	assert.Error(t, err, "a refused redirect is an error, not a successful cover")
}

func TestSubtitleService_DownloadContent_RedirectToBlockedHostIsNotFollowed(t *testing.T) {
	aURL, bHits := redirectTopology(t)
	s := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), &MockCacheService{})
	s.httpClient = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS) // non-strict seam
	content, _, err := s.downloadContent(context.Background(), aURL+"/ok")
	require.NoError(t, err)
	assert.Equal(t, "public-bytes", content)
	content, _, err = s.downloadContent(context.Background(), aURL+"/redir")
	assert.Zero(t, atomic.LoadInt32(bHits), "the redirect target must never be contacted")
	assert.NotContains(t, content, "INTERNAL-SECRET")
	assert.Error(t, err)
}

// F3: the clients used for provider/third-party-data URLs ignore the operator allow-list.
// Reviewer scenario: the operator allowed 127.0.0.0/8 (for a local metadata mirror);
// a URL taken from provider data, or a redirect to 127.0.0.2, must still be refused.
func strictTarget(t *testing.T) (url string, hits *int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	var n int32
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&n, 1)
		w.Write([]byte("INTERNAL-SECRET"))
	}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	return srv.URL, &n
}

func TestExternalMetadataResolver_IgnoresOperatorAllowList(t *testing.T) {
	u, hits := strictTarget(t)
	r := NewExternalMetadataResolver(nil, 2)
	res, err := r.Resolve(context.Background(), &resolver.ResolveRequest{AssetID: asset.NewID(), SourceHint: u + "/x", EntityType: "file", EntityID: "1"})
	if res != nil {
		res.Content.Close()
	}
	assert.Error(t, err)
	assert.Zero(t, atomic.LoadInt32(hits), "the internal host must never be contacted even when the env allows 127.0.0.0/8")
}

func TestSubtitleService_IgnoresOperatorAllowList(t *testing.T) {
	u, hits := strictTarget(t)
	s := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), &MockCacheService{})
	content, _, err := s.downloadContent(context.Background(), u+"/x")
	assert.Error(t, err)
	assert.NotContains(t, content, "INTERNAL-SECRET")
	assert.Zero(t, atomic.LoadInt32(hits), "the internal host must never be contacted even when the env allows 127.0.0.0/8")
}

// F6: downloadContent must treat non-2xx as an error and bound the body.
func subtitleSeamService(t *testing.T, h http.HandlerFunc) (*SubtitleService, string) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	s := NewSubtitleService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop(), &MockCacheService{})
	s.httpClient = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS)
	return s, srv.URL
}

func TestSubtitleService_DownloadContent_NonSuccessStatusIsError(t *testing.T) {
	for _, code := range []int{404, 500, 403, 301} {
		s, u := subtitleSeamService(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			w.Write([]byte("<html>error page, not a subtitle</html>"))
		})
		content, _, err := s.downloadContent(context.Background(), u+"/x")
		assert.Error(t, err, "HTTP %d must be an error", code)
		assert.Empty(t, content, "an error page must never be returned as subtitle content (HTTP %d)", code)
	}
}

func TestSubtitleService_DownloadContent_BoundedBody(t *testing.T) {
	// positive control: a body exactly at the cap is accepted
	s, u := subtitleSeamService(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.Write([]byte(strings.Repeat("a", maxSubtitleDownloadBytes)))
			return
		}
		w.Write([]byte(strings.Repeat("a", maxSubtitleDownloadBytes+1)))
	})
	content, _, err := s.downloadContent(context.Background(), u+"/ok")
	require.NoError(t, err)
	assert.Len(t, content, maxSubtitleDownloadBytes)
	content, _, err = s.downloadContent(context.Background(), u+"/big")
	assert.Error(t, err, "one byte over the cap is an error, not a silent truncation")
	assert.Empty(t, content)
}
