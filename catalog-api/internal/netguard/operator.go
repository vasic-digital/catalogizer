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
	"sync"
	"time"
)

// Operator-class policy (SECQ-1/W5).
//
// Some outbound destinations are chosen by the OPERATOR, not by third-party data: a WebDAV storage root is a
// NAS on the LAN (Synology on 192.168.x.x) or a public service (webdav.yandex.ru). The strict policy of
// DialContext (private ranges refused unless CATALOGIZER_OUTBOUND_ALLOW_CIDRS lists them) would break the first
// case, and no check at all (the state before this change) lets a storage root be aimed at the server itself,
// at the cloud metadata service or at any link-local service.
//
// The operator policy therefore refuses, always, after name resolution and on the address that is dialled:
// loopback, link-local (169.254.0.0/16, fe80::/10, which includes the metadata address 169.254.169.254),
// unspecified, multicast, broadcast, the reserved ranges of reservedNets, 0.0.0.0/8 and 240.0.0.0/4, the
// IPv6 metadata address fd00:ec2::254 and 100.100.100.200, and every IPv4-mapped, IPv4-compatible, NAT64 and
// 6to4 form of those. RFC 1918, IPv6 ULA (fc00::/7), CGNAT (Tailscale-style) and public addresses are allowed.
//
// Two operator controls:
//   - DenyPrivateEnv=1 (strict opt-in): RFC 1918, ULA and CGNAT are refused as well.
//   - AllowCIDRsEnv (the same allow-list the other clients use): re-admits loopback, private and CGNAT networks
//     it names, in both modes; it can never re-admit the always-refused addresses above.
const DenyPrivateEnv = "CATALOGIZER_OPERATOR_DENY_PRIVATE"

var operatorMetadataNets = []*net.IPNet{mustCIDR("fd00:ec2::254/128"), mustCIDR("100.100.100.200/32"), mustCIDR("2001::/32") /* Teredo */}

func denyPrivate() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(DenyPrivateEnv))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

// IsBlockedOperatorIP reports whether the operator-class policy refuses ip.
func IsBlockedOperatorIP(ip net.IP) bool { return isBlockedOperator(ip, denyPrivate()) }

func isBlockedOperator(ip net.IP, deny bool) bool {
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
	for _, n := range operatorMetadataNets {
		if n.Contains(ip) {
			return true
		}
	}
	if v4 := embeddedV4(ip); v4 != nil {
		return isBlockedOperator(v4, deny)
	}
	if v4 := ip.To4(); v4 != nil && (v4[0] == 0 || v4[0] >= 240) {
		return true
	}
	if ip.IsLoopback() {
		return !allowListed(ip)
	}
	if ip.IsPrivate() || cgnat.Contains(ip) {
		return deny && !allowListed(ip)
	}
	return false
}

func allowListed(ip net.IP) bool {
	for _, n := range allowedNets() {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// looksNumeric reports a host that is written like an address but is not a canonical IP literal (inet_aton forms:
// 0177.0.0.1, 2130706433, 127.1, 0x7f.1, a zone suffix). The system resolver may read such a name as an address, a
// proxy may read it differently from this code, so they are refused outright. A real DNS name never has an all-digit
// or 0x-hex last label.
func looksNumeric(host string) bool {
	if strings.Contains(host, "%") {
		return true
	}
	labels := strings.Split(strings.TrimSuffix(host, "."), ".")
	last := labels[len(labels)-1]
	if last == "" {
		return false
	}
	if strings.HasPrefix(strings.ToLower(last), "0x") {
		return true
	}
	for _, r := range last {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

var errUnverified = errors.New("destination could not be verified")

func resolveOperator(ctx context.Context, host string) ([]net.IP, error) {
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrBlocked)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if IsBlockedOperatorIP(ip) {
			return nil, fmt.Errorf("%w: %s is a loopback, link-local, metadata or reserved address", ErrBlocked, host)
		}
		return []net.IP{ip}, nil
	}
	if looksNumeric(host) {
		return nil, fmt.Errorf("%w: %q is not a canonical IP address or DNS name", ErrBlocked, host)
	}
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var ok []net.IP
	for _, a := range addrs {
		if IsBlockedOperatorIP(a.IP) {
			return nil, fmt.Errorf("%w: %s resolves to a loopback, link-local, metadata or reserved address", ErrBlocked, host)
		}
		ok = append(ok, a.IP)
	}
	if len(ok) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return ok, nil
}

// proxySet remembers the addresses of the proxies this transport was told to use, so the connection to a proxy
// (operator configuration, often 127.0.0.1) is not judged by the destination policy.
type proxySet struct{ m sync.Map }

func proxyAddr(u *url.URL) string {
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "https":
			port = "443"
		case "socks5", "socks5h":
			port = "1080"
		default:
			port = "80"
		}
	}
	return net.JoinHostPort(u.Hostname(), port)
}

// proxy selects the environment proxy for req and, when one is used, FAILS CLOSED unless the destination host is a
// canonical IP literal or a DNS name that resolves locally to permitted addresses only. A proxy resolves the destination
// itself, so a name that does not resolve here, or a numeric spelling the proxy reads differently, must never be
// forwarded. Residual risk (documented): the proxy resolves the name again, so a rebinding answer between this check and
// the proxy's own lookup is not covered; set DenyPrivateEnv and/or run without a proxy where that matters.
func (ps *proxySet) proxy(req *http.Request) (*url.URL, error) {
	p, err := proxyFunc(req)
	if err != nil || p == nil {
		return p, err
	}
	if _, err := resolveOperator(req.Context(), req.URL.Hostname()); err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", errUnverified, err)
	}
	ps.m.Store(proxyAddr(p), struct{}{})
	return p, nil
}

func (ps *proxySet) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	if _, ok := ps.m.Load(addr); ok {
		return (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	}
	return OperatorDialContext(ctx, network, addr)
}

// OperatorDialContext resolves addr, refuses the whole answer when any address is refused by the operator
// policy, and dials the checked IP itself (pinned), never the name.
func OperatorDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	ips, err := resolveOperator(ctx, host)
	if err != nil {
		return nil, err
	}
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
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

// OperatorTransport returns the transport for operator-chosen destinations: dial-time check on the resolved
// address of every connection; an environment proxy is honoured, fail-closed (see proxySet.proxy), and the proxy's own
// address is operator configuration and bypasses the destination policy. A direct connection to an address that equals
// a proxy used earlier by the same transport is therefore not checked (narrow residual).
func OperatorTransport() *http.Transport {
	ps := &proxySet{}
	return &http.Transport{
		Proxy:                 ps.proxy,
		DialContext:           ps.dial,
		TLSHandshakeTimeout:   15 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}
}

// OperatorCheckRedirect is an http.Client CheckRedirect: at most 10 hops, http(s) only, and the next hop's host
// is checked before the request is made (the dial check still runs on the connection itself).
func OperatorCheckRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if !HTTPOrHTTPS(req.URL) {
		return fmt.Errorf("%w: redirect to scheme %q not permitted", ErrBlocked, req.URL.Scheme)
	}
	if _, err := resolveOperator(req.Context(), req.URL.Hostname()); err != nil {
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) {
			return err
		}
	}
	return nil
}

// CheckOperatorURL validates raw for an operator-chosen destination: absolute URL, allowed scheme, no userinfo,
// host not refused by the operator policy (a name that does not resolve yet is accepted here and refused at
// dial time if it ever resolves to a refused address).
func CheckOperatorURL(ctx context.Context, raw string, schemes ...string) (*url.URL, error) {
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
	if _, err := resolveOperator(ctx, u.Hostname()); err != nil {
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) {
			return nil, err
		}
	}
	return u, nil
}

// proxyFunc selects the environment proxy; a variable so tests can drive a real proxy listener
// (http.ProxyFromEnvironment caches the process environment on first use).
var proxyFunc = http.ProxyFromEnvironment
