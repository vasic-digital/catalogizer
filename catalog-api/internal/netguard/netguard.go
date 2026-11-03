// Package netguard keeps server-side outbound requests (sync endpoints, the image
// proxy) from being aimed at the server's own network. A caller who controls a URL
// controls where the server connects, so loopback services, cloud metadata
// (169.254.169.254), the LAN and link-local addresses are refused AFTER name
// resolution, and the connection is dialled to the address that was checked
// (pinned), so a name that resolves differently a second time cannot slip through
// (DNS rebinding).
//
// Operators who really sync to a NAS on the LAN list the networks they accept in
// CATALOGIZER_OUTBOUND_ALLOW_CIDRS (comma separated, for example "192.168.1.0/24").
// That list can re-admit loopback, private and carrier-grade-NAT ranges only;
// link-local (metadata), unspecified, multicast and broadcast addresses are refused
// whatever it says.
package netguard

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// AllowCIDRsEnv names the environment variable holding the operator's allowed
// private networks.
const AllowCIDRsEnv = "CATALOGIZER_OUTBOUND_ALLOW_CIDRS"

// ErrBlocked is wrapped by every refusal.
var ErrBlocked = errors.New("outbound destination not allowed")

var cgnat = mustCIDR("100.64.0.0/10")

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// allowedNets reads the operator allow-list. Invalid entries are ignored (fail
// closed: they allow nothing).
func allowedNets() []*net.IPNet {
	var out []*net.IPNet
	for _, f := range strings.Split(os.Getenv(AllowCIDRsEnv), ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(f); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// alwaysBlocked reports addresses no configuration may re-admit.
func alwaysBlocked(ip net.IP) bool {
	return ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.Equal(net.IPv4bcast)
}

// IsBlockedIP reports whether the server must not connect to ip.
func IsBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4 // unwraps ::ffff:a.b.c.d
	}
	if alwaysBlocked(ip) {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || cgnat.Contains(ip) {
		for _, n := range allowedNets() {
			if n.Contains(ip) {
				return false
			}
		}
		return true
	}
	if v4 := ip.To4(); v4 != nil && (v4[0] == 0 || v4[0] >= 240) {
		return true // 0.0.0.0/8 and the reserved 240.0.0.0/4 block
	}
	return false
}

// resolve returns the checked addresses of host (an IP literal or a name).
func resolve(ctx context.Context, host string) ([]net.IP, error) {
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrBlocked)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if IsBlockedIP(ip) {
			return nil, fmt.Errorf("%w: %s is a loopback, private, link-local or reserved address", ErrBlocked, host)
		}
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var ok []net.IP
	for _, a := range addrs {
		if IsBlockedIP(a.IP) {
			return nil, fmt.Errorf("%w: %s resolves to %s (loopback, private, link-local or reserved)", ErrBlocked, host, a.IP)
		}
		ok = append(ok, a.IP)
	}
	if len(ok) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return ok, nil
}

// CheckURL validates raw: an absolute URL with one of schemes, a host, no
// userinfo, and a host that does not resolve to a forbidden address (a name that does
// not resolve at all is accepted here and refused by DialContext at connection time).
func CheckURL(ctx context.Context, raw string, schemes ...string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return nil, fmt.Errorf("%w: not an absolute URL", ErrBlocked)
	}
	okScheme := false
	for _, s := range schemes {
		if strings.EqualFold(u.Scheme, s) {
			okScheme = true
		}
	}
	if !okScheme {
		return nil, fmt.Errorf("%w: scheme %q not permitted", ErrBlocked, u.Scheme)
	}
	if u.User != nil {
		return nil, fmt.Errorf("%w: credentials in the URL are not permitted", ErrBlocked)
	}
	if _, err := resolve(ctx, u.Hostname()); err != nil {
		// A name that does not resolve (yet) is not an internal destination; it cannot be
		// connected to, and DialContext resolves and checks again at every connection, so
		// accepting it here cannot open a path to the local network. Everything else
		// (a blocked address, an empty host) is refused.
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) {
			return nil, err
		}
	}
	return u, nil
}

// DialContext resolves the address, refuses blocked results and dials the checked
// IP itself (pinned), never the name.
func DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := resolve(ctx, host)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	var last error
	for _, ip := range ips {
		c, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if derr == nil {
			return c, nil
		}
		last = derr
	}
	return nil, last
}

// Transport returns an http.Transport that only dials permitted addresses and
// ignores environment proxies (a proxy would resolve the target itself).
func Transport() *http.Transport {
	return &http.Transport{DialContext: DialContext, Proxy: nil, TLSHandshakeTimeout: 10 * time.Second}
}

// Client is an http.Client with Transport(). Redirects are followed at most
// maxRedirects times and only when redirectOK(next) allows the target (nil means
// no redirect is followed).
func Client(timeout time.Duration, maxRedirects int, redirectOK func(*url.URL) bool) *http.Client {
	return &http.Client{
		Timeout:   timeout,
		Transport: Transport(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if redirectOK == nil || len(via) > maxRedirects || !redirectOK(req.URL) {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
}
