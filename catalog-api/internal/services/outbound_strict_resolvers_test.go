package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"catalogizer/internal/netguard"

	"digital.vasic.assets/pkg/resolver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SSRF round 3: the content fetchers of third-party-controlled URLs (fanart, igdb,
// llm image search, cover art download) must go through netguard.StrictClient: the
// string-level GuardProviderURL is relaxed here (AllowPrivateSSRFForTests, as the
// existing httptest-based suites do) so only the HTTP CLIENT is under test, and the
// operator allow-list is set to 127.0.0.0/8, which the strict path must ignore.
//
// Real local listeners, no mocked clients:
//   - "public" is 127.0.0.1 and redirects to 127.0.0.2 / 169.254.169.254;
//   - "internal" is 127.0.0.2 and counts hits.
type ssrfTopo struct {
	publicURL   string // answers 200 on /ok and 302 elsewhere
	internalURL string
	publicHits  *int32
	internalHit *int32
}

func newSSRFTopo(t *testing.T, redirectTo func(internalURL string) string) ssrfTopo {
	t.Helper()
	AllowPrivateSSRFForTests(t)
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	var ih, ph int32
	internal := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ih, 1)
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write([]byte("INTERNAL-SECRET"))
	}))
	internal.Listener = ln
	internal.Start()
	t.Cleanup(internal.Close)
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ph, 1)
		if r.URL.Path == "/ok" {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write([]byte("public-bytes"))
			return
		}
		http.Redirect(w, r, redirectTo(internal.URL), http.StatusFound)
	}))
	t.Cleanup(public.Close)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	return ssrfTopo{public.URL, internal.URL, &ph, &ih}
}

func drain(res *resolver.ResolveResult) string {
	if res == nil || res.Content == nil {
		return ""
	}
	defer res.Content.Close()
	b, _ := io.ReadAll(res.Content)
	return string(b)
}

type fetchFn func(t *testing.T, target string) (string, error)

func fetchers() map[string]fetchFn {
	return map[string]fetchFn{
		"fanart": func(t *testing.T, target string) (string, error) {
			// download() has its own non-relaxable string pre-check (allowPublicURLLocal) that
			// refuses loopback literals before any client is used, so the resolver's HTTP
			// client is exercised directly here.
			resp, err := NewFanartTVResolver(1).client.Get(target)
			if err != nil {
				return "", err
			}
			defer resp.Body.Close()
			b, _ := io.ReadAll(resp.Body)
			if resp.StatusCode >= 300 {
				return string(b), fmt.Errorf("redirect not followed, status %d", resp.StatusCode)
			}
			return string(b), nil
		},
		"igdb": func(t *testing.T, target string) (string, error) {
			res, err := NewIGDBResolver(1).download(context.Background(), target)
			return drain(res), err
		},
		"llm_image": func(t *testing.T, target string) (string, error) {
			orch := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"url":%q}`, target)
			}))
			t.Cleanup(orch.Close)
			t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENABLED", "true")
			t.Setenv("CATALOGIZER_LLM_IMAGE_SEARCH_ENDPOINT", orch.URL) // operator-configured, loopback is legitimate
			res, err := NewLLMImageSearchResolver(90).Resolve(context.Background(),
				&resolver.ResolveRequest{EntityType: "movie", EntityID: "1"})
			return drain(res), err
		},
		"cover_art": func(t *testing.T, target string) (string, error) {
			s := NewCoverArtService(nil, zap.NewNop())
			b, err := s.downloadImage(context.Background(), target)
			return string(b), err
		},
	}
}

func TestStrictResolvers_RedirectToLoopbackIsNotFollowed(t *testing.T) {
	for name, fetch := range fetchers() {
		t.Run(name, func(t *testing.T) {
			topo := newSSRFTopo(t, func(internal string) string { return internal + "/internal" })
			AllowPrivateSSRFForTests(t)
			body, err := fetch(t, topo.publicURL+"/redir")
			assert.Zero(t, atomic.LoadInt32(topo.internalHit), "redirect target 127.0.0.2 must never be contacted")
			assert.NotContains(t, body, "INTERNAL-SECRET")
			require.Error(t, err)
			assert.True(t, errors.Is(err, netguard.ErrBlocked), "refusal must come from the strict dialer: %v", err)
		})
	}
}

func TestStrictResolvers_AllowListDoesNotOpenLoopbackDirectly(t *testing.T) {
	for name, fetch := range fetchers() {
		t.Run(name, func(t *testing.T) {
			topo := newSSRFTopo(t, func(internal string) string { return internal })
			AllowPrivateSSRFForTests(t)
			// positive control for the topology: the public listener is up
			resp, err := http.Get(topo.publicURL + "/ok")
			require.NoError(t, err)
			resp.Body.Close()
			before := atomic.LoadInt32(topo.publicHits)
			body, err := fetch(t, topo.publicURL+"/ok")
			require.Error(t, err, "CATALOGIZER_OUTBOUND_ALLOW_CIDRS must not apply to third-party URLs")
			assert.True(t, errors.Is(err, netguard.ErrBlocked), "%v", err)
			assert.Empty(t, body)
			assert.Equal(t, before, atomic.LoadInt32(topo.publicHits), "the connection must not be made")
		})
	}
}

func TestStrictResolvers_RedirectToMetadataAddressIsRefusedAtDial(t *testing.T) {
	for name, fetch := range fetchers() {
		t.Run(name, func(t *testing.T) {
			AllowPrivateSSRFForTests(t)
			t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8,169.254.0.0/16")
			_, err := fetch(t, "http://169.254.169.254/latest/meta-data/")
			require.Error(t, err)
			assert.True(t, errors.Is(err, netguard.ErrBlocked), "%v", err)
		})
	}
}

// DNS rebinding stand-in: a NAME whose resolution at connect time is loopback. The
// string-level guard is relaxed here, so only the dial-time check can refuse it.
func TestStrictResolvers_NameResolvingToLoopbackIsRefusedAtDial(t *testing.T) {
	for name, fetch := range fetchers() {
		t.Run(name, func(t *testing.T) {
			topo := newSSRFTopo(t, func(internal string) string { return internal })
			AllowPrivateSSRFForTests(t)
			_, port, _ := net.SplitHostPort(topo.publicURL[len("http://"):])
			before := atomic.LoadInt32(topo.publicHits)
			_, err := fetch(t, "http://localhost:"+port+"/ok")
			require.Error(t, err)
			assert.True(t, errors.Is(err, netguard.ErrBlocked), "%v", err)
			assert.Equal(t, before, atomic.LoadInt32(topo.publicHits))
		})
	}
}
