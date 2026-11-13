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
	"syscall"
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
// IPv6 metadata addresses fd00:ec2::254 and fd20:ce::254 and 100.100.100.200, and every IPv4-mapped, IPv4-compatible, NAT64 and
// 6to4 form of those. RFC 1918, IPv6 ULA (fc00::/7), CGNAT (Tailscale-style) and public addresses are allowed.
//
// Two operator controls:
//   - DenyPrivateEnv=1 (strict opt-in): RFC 1918, ULA and CGNAT are refused as well.
//   - AllowCIDRsEnv (the same allow-list the other clients use): re-admits loopback, private and CGNAT networks
//     it names, in both modes; it can never re-admit the always-refused addresses above.
const DenyPrivateEnv = "CATALOGIZER_OPERATOR_DENY_PRIVATE"

// fd20:ce::254 is the Compute Engine metadata server on IPv6-only instances, documented by Google at
// https://docs.cloud.google.com/compute/docs/metadata/querying-metadata (single address; the surrounding /32 is not refused).
// The metadata singletons themselves live in metadataSingletons (netguard.go) and are refused by every client class; the
// operator class additionally refuses Teredo (2001::/32).
var operatorMetadataNets = []*net.IPNet{mustCIDR("2001::/32") /* Teredo */}

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
	return !permitted(ip, ClassPrivate, deny)
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

// checkHostSpelling refuses host names that are not plain ASCII (fullwidth digits, encircled letters, homoglyphs: a later
// component that normalises IDNA could read them as 127.0.0.1 or localhost; IDNs must be given as punycode, xn--) and
// numeric spellings (inet_aton forms, zones). It runs before any name lookup, on the strict and the operator path.
func checkHostSpelling(host string) error {
	for i := 0; i < len(host); i++ {
		if host[i] >= 0x80 {
			return fmt.Errorf("%w: %q is not an ASCII host name (use punycode)", ErrBlocked, host)
		}
	}
	if looksNumeric(host) {
		return fmt.Errorf("%w: %q is not a canonical IP address or DNS name", ErrBlocked, host)
	}
	return nil
}

var errUnverified = errors.New("destination could not be verified")

func resolveOperator(ctx context.Context, host string) ([]net.IP, error) {
	return resolveOperatorMode(ctx, host, false, ClassPrivate)
}

// resolveOperatorStrict refuses the name when ANY resolved address is refused. It is used where the name (not a pinned
// address) is handed to another party that resolves it again, i.e. an environment proxy.
func resolveOperatorStrict(ctx context.Context, host string, approved Class) ([]net.IP, error) {
	return resolveOperatorMode(ctx, host, true, approved)
}

// resolveOperatorMode resolves host and keeps the addresses that an endpoint approved for approved may reach.
func resolveOperatorMode(ctx context.Context, host string, strict bool, approved Class) ([]net.IP, error) {
	deny := denyPrivate()
	if host == "" {
		return nil, fmt.Errorf("%w: empty host", ErrBlocked)
	}
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		if !permitted(ip, approved, deny) {
			return nil, fmt.Errorf("%w: %s is a loopback, link-local, metadata, reserved or own address, or outside the approved class (%s)", ErrBlocked, host, approved)
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
	// FILTER policy (SECQ1-F4): refused addresses are dropped from the answer and only the remaining, checked
	// addresses are dialled (the dial is pinned to them), so a name that answers 192.168.x.x together with fe80::
	// still works. The whole name is refused only when no permitted address remains.
	var ok []net.IP
	dropped := 0
	for _, a := range addrs {
		if !permitted(a.IP, approved, deny) {
			if strict {
				return nil, fmt.Errorf("%w: %s resolves to a loopback, link-local, metadata, reserved or own address, or outside the approved class (%s)", ErrBlocked, host, approved)
			}
			dropped++
			continue
		}
		ok = append(ok, a.IP)
	}
	if len(ok) == 0 {
		if dropped > 0 {
			return nil, fmt.Errorf("%w: %s resolves only to loopback, link-local, metadata, reserved or own addresses, or to addresses outside the approved class (%s)", ErrBlocked, host, approved)
		}
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	return ok, nil
}

// proxySet remembers the addresses of the proxies this transport was told to use, so the connection to a proxy
// (operator configuration, often 127.0.0.1) is not judged by the destination policy. pin is the class ceiling of the
// transport (see classPin).
type proxySet struct {
	m   sync.Map
	pin *classPin
}

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
// canonical IP literal or a DNS name that resolves locally to permitted addresses only (STRICT: one refused address in
// the answer refuses the name, because the name, not a pinned address, is forwarded). A proxy resolves the destination
// itself, so a name that does not resolve here, or a numeric spelling the proxy reads differently, must never be
// forwarded. Residual risk (documented): the proxy resolves the name again, so a rebinding answer between this check and
// the proxy's own lookup is not covered; set DenyPrivateEnv and/or run without a proxy where that matters.
func (ps *proxySet) proxy(req *http.Request) (*url.URL, error) {
	p, err := proxyFunc(req)
	if err != nil || p == nil {
		return p, err
	}
	end, gerr := ps.pin.begin(req.Context())
	if gerr != nil {
		return nil, gerr
	}
	defer end()
	ips, err := resolveOperatorStrict(req.Context(), req.URL.Hostname(), ps.pin.get())
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %v", errUnverified, err)
	}
	ps.pin.note(ClassOf(ips[0]))
	ps.m.Store(proxyAddr(p), struct{}{})
	return p, nil
}

// SECQ1-F1 (considered, not changed): bypassing only for proxy addresses that are not operator-blocked would break the
// supported local proxy on 127.0.0.1 (itself blocked), and a dial cannot tell a proxy connection from a direct one.
// Actual residual: a direct dial (e.g. NO_PROXY) to exactly host:port of a proxy already used by the same transport
// skips the destination check.
func (ps *proxySet) dial(ctx context.Context, network, addr string) (net.Conn, error) {
	if _, ok := ps.m.Load(addr); ok {
		return (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, addr)
	}
	return operatorDial(ctx, network, addr, ps.pin)
}

// dialTCP connects to one checked address; tests replace it to route a pinned public / private address to a local
// listener without weakening the checks that run before it.
var dialTCP = func(ctx context.Context, d *net.Dialer, network, ip, port string) (net.Conn, error) {
	return d.DialContext(ctx, network, net.JoinHostPort(ip, port))
}

// OperatorDialContext resolves addr, drops the addresses refused by the operator policy (refusing the name when none remain), and dials the checked IP itself (pinned), never the name.
// Approved class: ClassPrivate (private and public addresses).
func OperatorDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	return OperatorDialContextClass(ctx, network, addr, ClassPrivate)
}

// OperatorDialContextClass is OperatorDialContext for an endpoint approved for the given class: addresses of a more
// internal class are dropped from the answer, and a dial-time Control hook re-checks the address the kernel connects to.
func OperatorDialContextClass(ctx context.Context, network, addr string, approved Class) (net.Conn, error) {
	return operatorDial(ctx, network, addr, newClassPin(approved))
}

// operatorControl is the dial-time hook: it re-checks the address the kernel is about to connect to against the current ceiling.
func operatorControl(pin *classPin, deny bool) func(network, address string, c syscall.RawConn) error {
	return func(_, address string, _ syscall.RawConn) error {
		h, _, serr := net.SplitHostPort(address)
		if serr != nil {
			return serr
		}
		ip := net.ParseIP(h)
		if ip == nil || !permitted(ip, pin.get(), deny) {
			return fmt.Errorf("%w: %s is outside the approved class (%s)", ErrBlocked, address, pin.get())
		}
		return nil
	}
}

func operatorDial(ctx context.Context, network, addr string, pin *classPin) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	end, gerr := pin.begin(ctx)
	if gerr != nil {
		return nil, gerr
	}
	defer end()
	approved := pin.get()
	ips, err := resolveOperatorMode(ctx, host, false, approved)
	if err != nil {
		return nil, err
	}
	deny := denyPrivate()
	d := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	d.Control = operatorControl(pin, deny)
	var last error
	for _, ip := range ips {
		c, derr := dialTCP(ctx, d, network, ip.String(), port)
		if derr == nil {
			pin.note(ClassOf(ip))
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
// The transport is approved for ClassPrivate and tightens itself to ClassPublic when its first connection reaches a
// public address (see classPin), so a root on the internet cannot be turned into a path to the LAN.
func OperatorTransport() *http.Transport { return OperatorTransportFor(ClassPrivate) }

// OperatorTransportFor is OperatorTransport for an endpoint approved for the given class.
func OperatorTransportFor(approved Class) *http.Transport {
	ps := &proxySet{pin: newClassPin(approved)}
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
// is checked before the request is made (the dial check still runs on the connection itself). A redirect never
// reaches a more internal class than the first request of the chain: a public origin cannot send the client into the LAN.
func OperatorCheckRedirect(req *http.Request, via []*http.Request) error {
	return OperatorCheckRedirectFor(ClassPrivate)(req, via)
}

// OperatorCheckRedirectFor is OperatorCheckRedirect for an endpoint approved for the given class. The ceiling for the hop
// is the less internal of approved and the class of the first request's host.
func OperatorCheckRedirectFor(approved Class) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}
		if !HTTPOrHTTPS(req.URL) {
			return fmt.Errorf("%w: redirect to scheme %q not permitted", ErrBlocked, req.URL.Scheme)
		}
		ceiling := approved
		if len(via) > 0 && via[0].URL != nil {
			if oc, ok := hostClass(req.Context(), via[0].URL.Hostname()); ok && oc == ClassPublic {
				ceiling = ClassPublic
			}
		}
		if _, err := resolveOperatorMode(req.Context(), req.URL.Hostname(), false, ceiling); err != nil {
			var dnsErr *net.DNSError
			if !errors.As(err, &dnsErr) {
				return err
			}
		}
		return nil
	}
}

// hostClass returns the class of the first permitted address of host (a literal or a name); ok is false when it cannot be determined.
func hostClass(ctx context.Context, host string) (Class, bool) {
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		return ClassOf(ip), true
	}
	if checkHostSpelling(host) != nil {
		return ClassNever, false
	}
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil || len(addrs) == 0 {
		return ClassNever, false
	}
	for _, a := range addrs {
		if c := ClassOf(a.IP); c != ClassNever {
			return c, true
		}
	}
	return ClassNever, false
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
