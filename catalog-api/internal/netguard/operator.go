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

var operatorMetadataNets = []*net.IPNet{mustCIDR("fd00:ec2::254/128"), mustCIDR("100.100.100.200/32")}

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
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	var ok []net.IP
	for _, a := range addrs {
		if IsBlockedOperatorIP(a.IP) {
			return nil, fmt.Errorf("%w: %s resolves to %s (loopback, link-local, metadata or reserved)", ErrBlocked, host, a.IP)
		}
		ok = append(ok, a.IP)
	}
	if len(ok) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return ok, nil
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

// OperatorProxy is an http.Transport Proxy function: the environment proxy is honoured (a LAN may require it),
// but because a proxy resolves the destination itself the dial-time check cannot apply, so the destination host is
// resolved and checked here before the request is handed to the proxy.
func OperatorProxy(req *http.Request) (*url.URL, error) {
	p, err := http.ProxyFromEnvironment(req)
	if err != nil || p == nil {
		return p, err
	}
	if _, err := resolveOperator(req.Context(), req.URL.Hostname()); err != nil {
		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) {
			return nil, err
		}
	}
	return p, nil
}

// OperatorTransport returns the transport for operator-chosen destinations: dial-time check on the resolved
// address of every connection, environment proxy honoured through OperatorProxy.
func OperatorTransport() *http.Transport {
	return &http.Transport{
		Proxy:                 OperatorProxy,
		DialContext:           OperatorDialContext,
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
