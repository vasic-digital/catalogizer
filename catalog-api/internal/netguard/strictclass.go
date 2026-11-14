package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

// StrictClassTransport is the transport of a client that must satisfy BOTH policies at once (CAT-SECQ1-C review round 2): the strict
// outbound policy of DialContext (private, loopback and link-local ranges refused unless CATALOGIZER_OUTBOUND_ALLOW_CIDRS lists them;
// metadata singletons and reserved ranges always refused; environment proxies ignored) AND the address class the endpoint is approved
// for (a public approval refuses every private or loopback address, including an allow-listed one, and the transport tightens itself
// to public after its first connection reaches a public address, see classPin). The address that is dialled must pass both checks, so the
// result is never looser than Transport(). The class check sits in the dial, so libraries that own their redirect policy (gowebdav) are
// covered on every hop.
func StrictClassTransport(approved Class) *http.Transport {
	if approved != ClassPublic {
		approved = ClassPrivate
	}
	pin := newClassPin(approved)
	return &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return strictClassDial(ctx, network, addr, pin)
		},
		Proxy:               nil,
		TLSHandshakeTimeout: 10 * time.Second,
	}
}

func strictClassDial(ctx context.Context, network, addr string, pin *classPin) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	end, gerr := pin.begin(ctx)
	if gerr != nil {
		return nil, gerr
	}
	defer end()
	ceiling := pin.get()
	ips, err := resolve(ctx, host, true)
	if err != nil {
		return nil, err
	}
	var okIPs []net.IP
	for _, ip := range ips {
		if !permitted(ip, ceiling, false) {
			return nil, fmt.Errorf("%w: %s is outside the approved class (%s)", ErrBlocked, ip, ceiling)
		}
		okIPs = append(okIPs, ip)
	}
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	d.Control = func(_, address string, _ syscall.RawConn) error {
		h, _, serr := net.SplitHostPort(address)
		if serr != nil {
			return serr
		}
		ip := net.ParseIP(h)
		if ip == nil || isBlocked(ip, true) || !permitted(ip, pin.get(), false) {
			return fmt.Errorf("%w: %s is outside the approved class (%s)", ErrBlocked, address, pin.get())
		}
		return nil
	}
	var last error
	for i, ip := range okIPs {
		if i >= maxVettedDials {
			break
		}
		c, derr := d.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if derr == nil {
			pin.note(ClassOf(ip))
			return c, nil
		}
		last = derr
	}
	return nil, last
}
