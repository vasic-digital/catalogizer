package netguard

import (
	"context"
	"errors"
	"fmt"
	xproxy "golang.org/x/net/proxy"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

// AZ2-1: ONE constructor for every server-side HTTP client that talks to a third-party provider API (TMDB, fanart.tv,
// IGDB, Cover Art Archive, lyrics, translation, recognition, recommendation, LLM and webhook endpoints). A plain
// http.Client checked the URL string once (GuardProviderURL) but then followed redirects anywhere and resolved the
// name again at dial time, so a provider answer (or a rebinding DNS answer) could aim the server at loopback, the LAN
// or cloud metadata. A client built here
//   - dials only addresses that pass the class check, and dials the checked IP itself (pinned: no second lookup),
//   - re-validates every redirect hop (http or https only, no credentials in the URL, host not in a blocked class,
//     at most MaxRedirects hops),
//   - has a total timeout,
//   - caps the response body (ErrResponseTooLarge, never a silent truncation),
//   - ignores environment proxies (a proxy resolves the destination itself).
//
// Strict clients ignore CATALOGIZER_OUTBOUND_ALLOW_CIDRS (public hosts only); non-strict clients honour it, for
// endpoints an operator may legitimately keep on the LAN (a local LLM, a webhook relay).
//
// Operator proxy (SOCKS5 / HTTP): the operator chose it, the proxy resolves the destination, so the dial-time pin
// cannot apply to the destination; the redirect hops are still class-checked and the cap and timeout still apply.
// That residual is stated in docs/FOLLOW_UPS_NEXT_PHASE.md (AZ2-1).

// DefaultProviderTimeout and DefaultProviderMaxBytes apply when the config leaves them zero.
const (
	DefaultProviderTimeout   = 30 * time.Second
	DefaultProviderMaxBytes  = 64 << 20
	DefaultProviderRedirects = 3
)

// ErrResponseTooLarge is returned by a body read that crosses the cap.
var ErrResponseTooLarge = errors.New("provider response larger than the configured cap")

// ProviderProxy is the operator's outbound proxy. Zero value: none.
type ProviderProxy struct {
	// HTTP is an http(s) proxy URL for http.Transport.Proxy.
	HTTP *url.URL
	// Dial replaces the dialer (a SOCKS5 proxy). When set, the destination is resolved by the proxy.
	Dial func(ctx context.Context, network, addr string) (net.Conn, error)
}

// ProviderClientConfig configures NewProviderClient.
type ProviderClientConfig struct {
	Timeout      time.Duration // 0: DefaultProviderTimeout
	MaxRedirects int           // 0: DefaultProviderRedirects; negative: follow none
	MaxBytes     int64         // 0: DefaultProviderMaxBytes; negative: no cap (large downloads that stream to disk)
	Strict       bool          // ignore the operator allow-list
	Proxy        ProviderProxy
}

// testAllowLoopback lets tests reach httptest servers (always on loopback) through provider clients. Production
// never sets it; only the test relax helpers of internal/services call SetTestAllowLoopback.
var testAllowLoopback atomic.Bool

// SetTestAllowLoopback relaxes ONLY loopback for provider clients (every other class stays blocked) and returns
// the restore function. Tests only.
func SetTestAllowLoopback(allow bool) func() {
	prev := testAllowLoopback.Swap(allow)
	return func() { testAllowLoopback.Store(prev) }
}

func providerBlocked(strict bool) func(net.IP) bool {
	return func(ip net.IP) bool {
		if testAllowLoopback.Load() && ip.IsLoopback() {
			return false
		}
		return isBlocked(ip, !strict)
	}
}

func providerDial(strict bool) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		ips, err := resolveWith(ctx, host, providerBlocked(strict))
		if err != nil {
			return nil, err
		}
		d := &net.Dialer{Timeout: 10 * time.Second}
		var last error
		for _, ip := range ips {
			c, derr := dialTCP(ctx, d, network, ip.String(), port)
			if derr == nil {
				return c, nil
			}
			last = derr
		}
		return nil, last
	}
}

// providerCheckURL is the per-hop URL check: absolute http(s) URL, no userinfo, host not in a blocked class.
func providerCheckURL(ctx context.Context, u *url.URL, strict bool) error {
	if u == nil || u.Host == "" {
		return fmt.Errorf("%w: not an absolute URL", ErrBlocked)
	}
	if !HTTPOrHTTPS(u) {
		return fmt.Errorf("%w: scheme %q not permitted", ErrBlocked, u.Scheme)
	}
	if u.User != nil {
		return fmt.Errorf("%w: credentials in the URL are not permitted", ErrBlocked)
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && providerBlocked(strict)(ip) {
		return fmt.Errorf("%w: %s is a loopback, private, link-local or reserved address", ErrBlocked, u.Hostname())
	}
	return nil
}

type cappedBody struct {
	rc   io.ReadCloser
	left int64
	over bool
}

func (b *cappedBody) Read(p []byte) (int, error) {
	if b.over {
		return 0, ErrResponseTooLarge
	}
	if b.left == 0 {
		// at the cap: one probe byte tells a body of exactly the cap from a longer one
		var one [1]byte
		n, err := b.rc.Read(one[:])
		if n > 0 {
			b.over = true
			return 0, ErrResponseTooLarge
		}
		return 0, err
	}
	if int64(len(p)) > b.left {
		p = p[:b.left]
	}
	n, err := b.rc.Read(p)
	b.left -= int64(n)
	return n, err
}
func (b *cappedBody) Close() error { return b.rc.Close() }

type cappingTransport struct {
	rt  http.RoundTripper
	max int64
}

func (t *cappingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.rt.RoundTrip(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}
	if resp.ContentLength > t.max {
		resp.Body.Close()
		return nil, fmt.Errorf("%w: Content-Length %d", ErrResponseTooLarge, resp.ContentLength)
	}
	resp.Body = &cappedBody{rc: resp.Body, left: t.max}
	return resp, nil
}

// NewProviderClient builds the shared provider HTTP client (see the package comment above).
func NewProviderClient(cfg ProviderClientConfig) *http.Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = DefaultProviderTimeout
	}
	hops := cfg.MaxRedirects
	if hops == 0 {
		hops = DefaultProviderRedirects
	}
	tr := &http.Transport{
		DialContext:           providerDial(cfg.Strict),
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: timeout,
		Proxy:                 nil,
	}
	if cfg.Proxy.Dial != nil {
		tr.DialContext = cfg.Proxy.Dial
	} else if cfg.Proxy.HTTP != nil {
		tr.Proxy = http.ProxyURL(cfg.Proxy.HTTP)
		tr.DialContext = (&net.Dialer{Timeout: 10 * time.Second}).DialContext // the operator's proxy itself
	}
	var rt http.RoundTripper = tr
	if cfg.MaxBytes >= 0 {
		max := cfg.MaxBytes
		if max == 0 {
			max = DefaultProviderMaxBytes
		}
		rt = &cappingTransport{rt: tr, max: max}
	}
	strict := cfg.Strict
	return &http.Client{
		Timeout:   timeout,
		Transport: rt,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if hops < 0 || len(via) > hops {
				return http.ErrUseLastResponse
			}
			if err := providerCheckURL(req.Context(), req.URL, strict); err != nil {
				return err
			}
			return nil
		},
	}
}

// socks5DialTimeout bounds the SOCKS5 connect and handshake: net/http detaches a dial from the request context, so a
// proxy that accepts TCP and never answers would otherwise hold the connection forever.
const socks5DialTimeout = 30 * time.Second

// ProxyFromConfig turns the operator's proxy settings into a ProviderProxy: a socks5 URL wins, then the HTTP proxy
// URL; an unusable setting yields the zero value (direct, still guarded). Identical precedence to the former
// per-client builders.
func ProxyFromConfig(enabled bool, socksURL, user, pass, httpURL string) ProviderProxy {
	if !enabled {
		return ProviderProxy{}
	}
	if socksURL != "" {
		if p, err := url.Parse(socksURL); err == nil && p.Scheme == "socks5" {
			var auth *xproxy.Auth
			if user != "" || pass != "" {
				auth = &xproxy.Auth{User: user, Password: pass}
			}
			if d, err := xproxy.SOCKS5("tcp", p.Host, auth, xproxy.Direct); err == nil {
				return ProviderProxy{Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
					if cd, ok := d.(xproxy.ContextDialer); ok {
						dctx, cancel := context.WithTimeout(ctx, socks5DialTimeout)
						defer cancel()
						return cd.DialContext(dctx, network, addr)
					}
					return d.Dial(network, addr)
				}}
			}
		}
	}
	if httpURL != "" {
		if p, err := url.Parse(httpURL); err == nil {
			return ProviderProxy{HTTP: p}
		}
	}
	return ProviderProxy{}
}
