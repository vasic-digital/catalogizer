package netguard

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/netguard/netguardtest"

	"github.com/stretchr/testify/require"
)

// AZ2-1: the shared provider client, driven over real loopback listeners.

func TestAZ21_ProviderClient_RefusesRedirectToUnspecifiedAddress(t *testing.T) {
	restore := SetTestAllowLoopback(true)
	defer restore()
	top := netguardtest.New(t)
	for _, strict := range []bool{true, false} {
		c := NewProviderClient(ProviderClientConfig{Strict: strict, Timeout: 5 * time.Second})
		r, err := c.Get(top.FrontURL + "/ok") // control needle: loopback front reachable under the test relax
		require.NoError(t, err)
		r.Body.Close()
		_, err = c.Get(top.FrontURL + "/to-unspecified")
		require.ErrorIs(t, err, ErrBlocked, "strict=%v", strict)
	}
	require.Equal(t, 0, top.SinkHits(), "the sink must never be reached")
	// control needle for the counter: a direct request does reach it
	r, err := NewProviderClient(ProviderClientConfig{}).Get(top.SinkURL + "/secret")
	require.NoError(t, err)
	r.Body.Close()
	require.Equal(t, 1, top.SinkHits())
}

func TestAZ21_ProviderClient_RefusesSchemeAndLoopAndHops(t *testing.T) {
	defer SetTestAllowLoopback(true)()
	top := netguardtest.New(t)
	c := NewProviderClient(ProviderClientConfig{Timeout: 5 * time.Second})
	_, err := c.Get(top.FrontURL + "/to-gopher")
	require.ErrorIs(t, err, ErrBlocked)
	// a redirect loop stops at MaxRedirects with the last response (302), not an endless chase
	resp, err := c.Get(top.FrontURL + "/loop")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	// MaxRedirects < 0 follows none
	c = NewProviderClient(ProviderClientConfig{MaxRedirects: -1, Timeout: 5 * time.Second})
	resp, err = c.Get(top.FrontURL + "/to-unspecified")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	require.Equal(t, 0, top.SinkHits())
}

func TestAZ21_ProviderClient_DialIsClassCheckedWithoutTestRelax(t *testing.T) {
	// no relax: a loopback destination is refused at dial time, before any connection
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&hits, 1) }))
	defer srv.Close()
	t.Setenv(AllowCIDRsEnv, "")
	_, err := NewProviderClient(ProviderClientConfig{Timeout: 3 * time.Second}).Get(srv.URL)
	require.ErrorIs(t, err, ErrBlocked)
	require.Zero(t, atomic.LoadInt32(&hits))
	// the operator allow-list re-admits loopback for a non-strict client, never for a strict one
	t.Setenv(AllowCIDRsEnv, "127.0.0.0/8")
	r, err := NewProviderClient(ProviderClientConfig{Timeout: 3 * time.Second}).Get(srv.URL)
	require.NoError(t, err)
	r.Body.Close()
	require.EqualValues(t, 1, atomic.LoadInt32(&hits))
	_, err = NewProviderClient(ProviderClientConfig{Strict: true, Timeout: 3 * time.Second}).Get(srv.URL)
	require.ErrorIs(t, err, ErrBlocked)
	require.EqualValues(t, 1, atomic.LoadInt32(&hits))
}

func TestAZ21_ProviderClient_TestRelaxIsLoopbackOnly(t *testing.T) {
	defer SetTestAllowLoopback(true)()
	_, err := providerDial(true)(context.Background(), "tcp", "169.254.169.254:80")
	require.ErrorIs(t, err, ErrBlocked)
	_, err = providerDial(true)(context.Background(), "tcp", "10.0.0.5:80")
	require.ErrorIs(t, err, ErrBlocked)
	_, err = providerDial(false)(context.Background(), "tcp", "0.0.0.0:80")
	require.ErrorIs(t, err, ErrBlocked)
}

// DNS rebinding: the first lookup answers a public address, a later lookup loopback. Only the address that passed
// the check is dialled (pinned), one lookup per dial, and the rebound answer is refused before any connection.
func TestAZ21_ProviderClient_DialPinsCheckedAddressAgainstRebinding(t *testing.T) {
	t.Setenv(AllowCIDRsEnv, "")
	oldL, oldD := lookupIPAddr, dialTCP
	defer func() { lookupIPAddr, dialTCP = oldL, oldD }()
	n := 0
	lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
		n++
		if n == 1 {
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}
		return []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}, nil
	}
	var dialed []string
	dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
		dialed = append(dialed, ip+":"+port)
		return nil, net.ErrClosed
	}
	dial := providerDial(true)
	_, _ = dial(context.Background(), "tcp", "rebind.example:443")
	require.Equal(t, []string{"93.184.216.34:443"}, dialed)
	require.Equal(t, 1, n)
	dialed = nil
	_, err := dial(context.Background(), "tcp", "rebind.example:443")
	require.ErrorIs(t, err, ErrBlocked)
	require.Empty(t, dialed)
}

func TestAZ21_ProviderClient_ResponseCap(t *testing.T) {
	defer SetTestAllowLoopback(true)()
	top := netguardtest.New(t)
	// /big streams ~200 MiB with no Content-Length: the default cap (64 MiB) ends the read with an error
	c := NewProviderClient(ProviderClientConfig{Timeout: 30 * time.Second})
	resp, err := c.Get(top.FrontURL + "/big")
	require.NoError(t, err)
	n, err := io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.ErrorIs(t, err, ErrResponseTooLarge)
	require.LessOrEqual(t, n, int64(DefaultProviderMaxBytes))
	// a small explicit cap
	c = NewProviderClient(ProviderClientConfig{Timeout: 30 * time.Second, MaxBytes: 1 << 20})
	resp, err = c.Get(top.FrontURL + "/big")
	require.NoError(t, err)
	n, err = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	require.ErrorIs(t, err, ErrResponseTooLarge)
	require.LessOrEqual(t, n, int64(1<<20))
	// a body under the cap is read in full
	resp, err = c.Get(top.FrontURL + "/ok")
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, "ok", string(b))
	// declared Content-Length over the cap is refused up front
	big := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "5000000")
		_, _ = w.Write([]byte(strings.Repeat("x", 10)))
	}))
	defer big.Close()
	_, err = c.Get(big.URL)
	require.Error(t, err)
}

// An operator HTTP proxy: the connection goes to the proxy, the redirect hops are still class-checked.
func TestAZ21_ProviderClient_OperatorHTTPProxyStillChecksRedirects(t *testing.T) {
	var via int32
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&via, 1)
		http.Redirect(w, r, "http://0.0.0.0:1/x", http.StatusFound)
	}))
	defer proxySrv.Close()
	pu, _ := url.Parse(proxySrv.URL)
	c := NewProviderClient(ProviderClientConfig{Timeout: 3 * time.Second, Proxy: ProviderProxy{HTTP: pu}})
	_, err := c.Get("http://provider.example/ok")
	require.ErrorIs(t, err, ErrBlocked)
	require.EqualValues(t, 1, atomic.LoadInt32(&via), "the request really went through the proxy")
}

func TestAZ21_ProxyFromConfig(t *testing.T) {
	require.Equal(t, ProviderProxy{}, ProxyFromConfig(false, "socks5://x:1", "", "", "http://y:2"))
	require.NotNil(t, ProxyFromConfig(true, "", "", "", "http://y:2").HTTP)
	require.NotNil(t, ProxyFromConfig(true, "socks5://127.0.0.1:1", "u", "p", "").Dial)
	require.Equal(t, ProviderProxy{}, ProxyFromConfig(true, "http://notsocks", "", "", ""))
	// a socks5 proxy that never answers is bounded by the handshake timeout, not held forever
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c
		}
	}()
	p := ProxyFromConfig(true, "socks5://"+ln.Addr().String(), "", "", "")
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, err = p.Dial(ctx, "tcp", "example.com:80")
	require.Error(t, err)
	require.True(t, errors.Is(err, context.DeadlineExceeded) || strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "timeout"), err.Error())
}
