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
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// AllowCIDRsEnv names the environment variable holding the operator's allowed
// private networks.
const AllowCIDRsEnv = "CATALOGIZER_OUTBOUND_ALLOW_CIDRS"

// ErrBlocked is wrapped by every refusal.
var ErrBlocked = errors.New("outbound destination not allowed")

var cgnat = mustCIDR("100.64.0.0/10")

// lookupIPAddr is the resolver; tests replace it to prove the dial is pinned to the checked address.
var lookupIPAddr = func(ctx context.Context, host string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, host)
}

// reservedNets are blocked and never re-admitted by the allow-list: benchmarking (198.18.0.0/15), IETF protocol
// assignments (192.0.0.0/24), deprecated IPv6 site-local (fec0::/10), the local-use NAT64 prefix (64:ff9b:1::/48) and
// the Azure WireServer address 168.63.129.16.
var reservedNets = []*net.IPNet{
	mustCIDR("198.18.0.0/15"), mustCIDR("192.0.0.0/24"), mustCIDR("fec0::/10"), mustCIDR("64:ff9b:1::/48"),
	mustCIDR("168.63.129.16/32"), // Azure WireServer / platform endpoint (SECQ-1 B5)
}

// embeddedV4 returns the IPv4 address an IPv6 address carries in an IPv4-compatible (::/96), NAT64 (64:ff9b::/96) or
// 6to4 (2002::/16) form, so it is judged as that IPv4 address (a loopback hidden in ::7f00:1 must not pass).
func embeddedV4(ip net.IP) net.IP {
	ip16 := ip.To16()
	if ip16 == nil || ip.To4() != nil {
		return nil
	}
	switch {
	case ip.IsUnspecified() || ip.Equal(net.IPv6loopback):
		return nil // :: and ::1 are not IPv4-compatible 0.0.0.0 / 0.0.0.1
	case mustCIDR("::/96").Contains(ip16):
		return net.IP{ip16[12], ip16[13], ip16[14], ip16[15]}
	case ip16[8] == 0xff && ip16[9] == 0xff && ip16[10] == 0 && ip16[11] == 0 && allZero(ip16[:8]):
		return net.IP{ip16[12], ip16[13], ip16[14], ip16[15]} // SIIT-translated ::ffff:0:a.b.c.d
	case mustCIDR("64:ff9b::/96").Contains(ip16):
		return net.IP{ip16[12], ip16[13], ip16[14], ip16[15]}
	case ip16[0] == 0x20 && ip16[1] == 0x02:
		return net.IP{ip16[2], ip16[3], ip16[4], ip16[5]}
	}
	return nil
}

func allZero(b []byte) bool {
	for _, x := range b {
		if x != 0 {
			return false
		}
	}
	return true
}

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// AllowBroadEnv, when set to 1, keeps allow-list entries shorter than /8 (IPv4) or /16 (IPv6), for example
// 0.0.0.0/0, which would re-admit every loopback address. Without it such entries are ignored and logged.
const AllowBroadEnv = "CATALOGIZER_OUTBOUND_ALLOW_BROAD"

var allowListWarned sync.Map

func warnAllowListOnce(entry, why string) {
	if _, loaded := allowListWarned.LoadOrStore(entry+"|"+why, struct{}{}); !loaded {
		log.Printf("netguard: %s entry %q ignored: %s", AllowCIDRsEnv, entry, why)
	}
}

// resetAllowListWarnings forgets which entries were already logged (tests).
func resetAllowListWarnings() {
	allowListWarned.Range(func(k, _ any) bool { allowListWarned.Delete(k); return true })
}

// allowedNets reads the operator allow-list. Invalid entries are ignored (fail closed: they allow nothing) and
// logged once; over-broad entries (prefix shorter than /8 for IPv4, /16 for IPv6) are ignored and logged unless
// AllowBroadEnv=1.
func allowedNets() []*net.IPNet {
	var out []*net.IPNet
	broadOK := strings.TrimSpace(os.Getenv(AllowBroadEnv)) == "1"
	for _, f := range strings.Split(os.Getenv(AllowCIDRsEnv), ",") {
		f = strings.TrimSpace(f)
		if f == "" {
			continue
		}
		_, n, err := net.ParseCIDR(f)
		if err != nil {
			warnAllowListOnce(f, "not a valid CIDR")
			continue
		}
		ones, bits := n.Mask.Size()
		if !broadOK && ((bits == 32 && ones < 8) || (bits == 128 && ones < 16)) {
			warnAllowListOnce(f, "prefix is too broad (shorter than /8 for IPv4 or /16 for IPv6); set "+AllowBroadEnv+"=1 to keep it")
			continue
		}
		out = append(out, n)
	}
	return out
}

// alwaysBlocked reports addresses no configuration may re-admit.
func alwaysBlocked(ip net.IP) bool {
	return ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.Equal(net.IPv4bcast)
}

// IsBlockedIP reports whether the server must not connect to ip.
func IsBlockedIP(ip net.IP) bool { return isBlocked(ip, true) }

// IsBlockedIPStrict is IsBlockedIP that ignores the operator allow-list: loopback,
// private, CGNAT, link-local, metadata and reserved addresses are always blocked.
func IsBlockedIPStrict(ip net.IP) bool { return isBlocked(ip, false) }

func isBlocked(ip net.IP, honorAllow bool) bool {
	if ip == nil {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4 // unwraps ::ffff:a.b.c.d
	}
	if alwaysBlocked(ip) {
		return true
	}
	for _, n := range reservedNets {
		if n.Contains(ip) {
			return true
		}
	}
	if v4 := embeddedV4(ip); v4 != nil {
		return isBlocked(v4, honorAllow)
	}
	if ip.IsLoopback() || ip.IsPrivate() || cgnat.Contains(ip) {
		if honorAllow {
			for _, n := range allowedNets() {
				if n.Contains(ip) {
					return false
				}
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
func resolve(ctx context.Context, host string, honorAllow bool) ([]net.IP, error) {
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrBlocked)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if isBlocked(ip, honorAllow) {
			return nil, fmt.Errorf("%w: %s is a loopback, private, link-local or reserved address", ErrBlocked, host)
		}
		return []net.IP{ip}, nil
	}
	if err := checkHostSpelling(host); err != nil {
		return nil, err
	}
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var ok []net.IP
	for _, a := range addrs {
		if isBlocked(a.IP, honorAllow) {
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
	if _, err := resolve(ctx, u.Hostname(), true); err != nil {
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
	return dial(ctx, network, addr, true)
}

// StrictDialContext is DialContext that ignores CATALOGIZER_OUTBOUND_ALLOW_CIDRS.
func StrictDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return dial(ctx, network, addr, false)
}

func dial(ctx context.Context, network, addr string, honorAllow bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := resolve(ctx, host, honorAllow)
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

// StrictClient is Client for fetches of URLs that come from third-party data (provider
// responses, subtitle/cover links): it ignores the operator allow-list, so loopback,
// private, link-local and metadata destinations are blocked on every hop whatever
// CATALOGIZER_OUTBOUND_ALLOW_CIDRS says. Environment proxies are ignored.
func StrictClient(timeout time.Duration, maxRedirects int, redirectOK func(*url.URL) bool) *http.Client {
	c := Client(timeout, maxRedirects, redirectOK)
	c.Transport = &http.Transport{DialContext: StrictDialContext, Proxy: nil, TLSHandshakeTimeout: 10 * time.Second}
	return c
}

// HTTPOrHTTPS is a redirectOK for Client that follows a redirect only to an http or
// https URL; the dialer still checks the destination of every hop.
func HTTPOrHTTPS(u *url.URL) bool {
	return strings.EqualFold(u.Scheme, "http") || strings.EqualFold(u.Scheme, "https")
}
