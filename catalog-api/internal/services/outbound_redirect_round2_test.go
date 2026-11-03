package services

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

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
	content, _, err := s.downloadContent(context.Background(), aURL+"/ok")
	require.NoError(t, err)
	assert.Equal(t, "public-bytes", content)
	content, _, err = s.downloadContent(context.Background(), aURL+"/redir")
	assert.Zero(t, atomic.LoadInt32(bHits), "the redirect target must never be contacted")
	assert.NotContains(t, content, "INTERNAL-SECRET")
	assert.Error(t, err)
}
