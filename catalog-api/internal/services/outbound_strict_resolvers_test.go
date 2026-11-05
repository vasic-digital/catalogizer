package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

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

// Review round 1 (item 1): download() itself. The IPv4-mapped IPv6 literal passes the
// fanart string pre-check (it only lists "://127." style prefixes) and the relaxed
// GuardProviderURL, so ONLY the dial-time check of the client can refuse it. A client
// that skipped the strict dialer (http.DefaultClient mutant) would fetch it.
func TestFanartDownload_MappedLoopbackLiteralIsRefusedByClient(t *testing.T) {
	topo := newSSRFTopo(t, func(internal string) string { return internal + "/internal" })
	AllowPrivateSSRFForTests(t)
	_, port, _ := net.SplitHostPort(topo.publicURL[len("http://"):])
	target := "http://[::ffff:127.0.0.1]:" + port
	require.NoError(t, allowPublicURLLocal(target), "premise: the string pre-check lets this through")
	// positive control: the same listener answers when reached by a plain client
	resp, err := http.Get(topo.publicURL + "/ok")
	require.NoError(t, err)
	resp.Body.Close()
	before := atomic.LoadInt32(topo.publicHits)
	for _, path := range []string{"/ok", "/redir"} {
		res, err := NewFanartTVResolver(1).download(context.Background(), target+path)
		assert.Empty(t, drain(res))
		require.Error(t, err, path)
		assert.True(t, errors.Is(err, netguard.ErrBlocked), "%s: %v", path, err)
	}
	assert.Equal(t, before, atomic.LoadInt32(topo.publicHits), "no connection to the mapped literal")
	assert.Zero(t, atomic.LoadInt32(topo.internalHit))
}

// Item 2: guardedRedirect (operator-proxy path) unit tests.
func TestGuardedRedirect(t *testing.T) {
	withStrictSSRFGuard(t) // the package TestMain relaxes the guard globally
	mk := func(raw string) *http.Request {
		u, _ := url.Parse(raw)
		return &http.Request{URL: u}
	}
	via := func(n int) []*http.Request { return make([]*http.Request, n) }
	assert.NoError(t, guardedRedirect(mk("http://93.184.216.34/next"), via(1)), "control: public hop is followed")
	assert.Error(t, guardedRedirect(mk("http://169.254.169.254/latest/meta-data/"), via(1)), "metadata hop")
	assert.Error(t, guardedRedirect(mk("http://127.0.0.2:8080/x"), via(1)), "loopback hop")
	assert.Error(t, guardedRedirect(mk("http://[::ffff:10.0.0.1]/x"), via(1)), "mapped private hop")
	assert.Error(t, guardedRedirect(mk("file:///etc/passwd"), via(1)), "non-http scheme hop")
	assert.Error(t, guardedRedirect(mk("ftp://93.184.216.34/x"), via(1)), "ftp scheme hop")
	assert.Equal(t, http.ErrUseLastResponse, guardedRedirect(mk("http://93.184.216.34/x"), via(4)), "more than 3 hops")
}

type fakeProxyCfg struct{ httpURL string }

func (c fakeProxyCfg) IsEnabled() bool     { return true }
func (c fakeProxyCfg) GetURL() string      { return "" }
func (c fakeProxyCfg) GetHTTPURL() string  { return c.httpURL }
func (c fakeProxyCfg) GetUsername() string { return "" }
func (c fakeProxyCfg) GetPassword() string { return "" }

// downloadImage with a real local forward proxy that answers a redirect to an internal
// address: the proxy path cannot dial-check, so every hop is re-checked.
func TestDownloadImage_ProxyRedirectToInternalIsRefused(t *testing.T) {
	withStrictSSRFGuard(t)
	ln, err := net.Listen("tcp", "127.0.0.2:0")
	if err != nil {
		t.Skipf("cannot bind 127.0.0.2: %v", err)
	}
	var ih, ph int32
	internal := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&ih, 1)
		w.Write([]byte("INTERNAL-SECRET"))
	}))
	internal.Listener = ln
	internal.Start()
	t.Cleanup(internal.Close)
	for name, loc := range map[string]string{"loopback": internal.URL + "/x", "metadata": "http://169.254.169.254/latest/meta-data/"} {
		t.Run(name, func(t *testing.T) {
			atomic.StoreInt32(&ph, 0)
			// forward proxy: a request in absolute form arrives here; answer with a redirect
			proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&ph, 1)
				http.Redirect(w, r, loc, http.StatusFound)
			}))
			defer proxySrv.Close()
			s := NewCoverArtService(nil, zap.NewNop())
			s.SetProxyConfig(fakeProxyCfg{httpURL: proxySrv.URL})
			body, err := s.downloadImage(context.Background(), "http://93.184.216.34/cover.jpg")
			require.Error(t, err)
			assert.Empty(t, body)
			assert.Equal(t, int32(1), atomic.LoadInt32(&ph), "control: the request really went through the proxy, once")
			assert.Zero(t, atomic.LoadInt32(&ih), "internal host must not be contacted")
		})
	}
}

// Item 7: the download is bounded.
func TestDownloadImage_BodyIsBounded(t *testing.T) {
	AllowPrivateSSRFForTests(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(make([]byte, 2048))
	}))
	defer srv.Close()
	s := NewCoverArtService(nil, zap.NewNop())
	s.httpClient = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS)
	defer func(old int64) { coverArtMaxBytes = old }(coverArtMaxBytes)
	coverArtMaxBytes = 4096
	b, err := s.downloadImage(context.Background(), srv.URL)
	require.NoError(t, err, "control: under the cap")
	assert.Len(t, b, 2048)
	coverArtMaxBytes = 1024
	b, err = s.downloadImage(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Nil(t, b)
	assert.Contains(t, err.Error(), "exceeds")
}

// Item 6: ffprobe/ffmpeg never open network protocols from a media file.
func TestCoverArtFFmpegCommands_ProtocolWhitelist(t *testing.T) {
	has := func(args []string) bool {
		for i := 0; i+1 < len(args); i++ {
			if args[i] == "-protocol_whitelist" && args[i+1] == "file,pipe" {
				return true
			}
		}
		return false
	}
	assert.True(t, has(ffprobeDurationCmd("/x.mp4").Args), "ffprobe")
	assert.True(t, has(ffmpegFrameCmd(context.Background(), 1, "/x.mp4", "/o.jpg").Args), "ffmpeg")
	// whitelist must precede -i (an input option)
	a := ffmpegFrameCmd(context.Background(), 1, "/x.mp4", "/o.jpg").Args
	wl, in := -1, -1
	for i, v := range a {
		if v == "-protocol_whitelist" {
			wl = i
		}
		if v == "-i" {
			in = i
		}
	}
	assert.True(t, wl >= 0 && wl < in, "protocol_whitelist must be an input option before -i: %v", a)
}

// Behavioural check against the installed ffmpeg: a crafted HLS/ffconcat playlist that
// points at a real local listener must cause no connection. (On the ffmpeg build used
// at the time of writing the default whitelist already blocks playlist-borne URLs, so this one
// is a regression guard; the behavioural REDs are in cover_art_ffmpeg_hardening_test.go: a direct
// http:// video path reaches a listener without -protocol_whitelist (1 hit each for ffprobe and
// ffmpeg measured), and a playlist naming a local file is followed without -format_whitelist.)
func TestCoverArtFFmpeg_PlaylistDoesNotReachNetwork(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not installed", tool)
		}
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	var hits int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&hits, 1)
			c.Close()
		}
	}()
	port := ln.Addr().(*net.TCPAddr).Port
	dir := t.TempDir()
	hls := filepath.Join(dir, "a.m3u8")
	cc := filepath.Join(dir, "b.ffconcat")
	require.NoError(t, os.WriteFile(hls, []byte(fmt.Sprintf("#EXTM3U\n#EXT-X-TARGETDURATION:1\n#EXTINF:1,\nhttp://127.0.0.1:%d/s.ts\n#EXT-X-ENDLIST\n", port)), 0o600))
	require.NoError(t, os.WriteFile(cc, []byte(fmt.Sprintf("ffconcat version 1.0\nfile 'http://127.0.0.1:%d/s.mp4'\n", port)), 0o600))
	s := NewCoverArtService(nil, zap.NewNop())
	s.cacheDir = dir
	for _, p := range []string{hls, cc} {
		_, _ = s.getVideoDuration(context.Background(), p)
		_, _ = s.generateVideoThumbnail(context.Background(), &VideoThumbnailRequest{MediaItemID: 1, VideoPath: p}, 1, 0)
	}
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, atomic.LoadInt32(&hits), "the playlist target must never be contacted")
}

// A server that never stops sending: the read must stop at the cap, not at the client timeout.
func TestDownloadImage_EndlessBodyStopsAtCap(t *testing.T) {
	AllowPrivateSSRFForTests(t)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chunk := make([]byte, 4096)
		for r.Context().Err() == nil {
			if _, err := w.Write(chunk); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	s := NewCoverArtService(nil, zap.NewNop())
	s.httpClient = netguard.Client(3*time.Second, 3, netguard.HTTPOrHTTPS)
	defer func(old int64) { coverArtMaxBytes = old }(coverArtMaxBytes)
	coverArtMaxBytes = 64 << 10
	start := time.Now()
	_, err := s.downloadImage(context.Background(), srv.URL)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds", "must fail on the cap, not on the timeout: %v", err)
	assert.Less(t, time.Since(start), 2*time.Second)
}
