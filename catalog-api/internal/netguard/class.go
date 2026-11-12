package netguard

import (
	"fmt"
	"net"
	"sync"
)

// Address classes (SECQ-1, CAT-SECQ1-A).
//
// Every destination address belongs to exactly one class, from most internal to most public:
//
//	ClassNever   loopback, link-local, metadata, unspecified, multicast, broadcast, the reserved ranges, and the
//	             server's own interface addresses. Never reachable by an operator-chosen endpoint (the allow-list
//	             can re-admit loopback and own addresses, which then count as ClassPrivate).
//	ClassPrivate RFC 1918, IPv6 ULA and CGNAT (the LAN, Tailscale-style networks).
//	ClassPublic  everything else.
//
// An endpoint is APPROVED for a class: ClassPrivate (default, the behaviour before this change) may connect to
// private and public addresses; ClassPublic may connect to public addresses only; ClassNever approves nothing.
// A connection (and every redirect hop, which is a new connection) to a more internal class than the approved one
// is refused. This closes the redirect / DNS-rebinding path from a public server into the LAN (B1).
type Class int

const (
	ClassNever Class = iota
	ClassPrivate
	ClassPublic
)

func (c Class) String() string {
	switch c {
	case ClassNever:
		return "never"
	case ClassPrivate:
		return "private"
	case ClassPublic:
		return "public"
	}
	return fmt.Sprintf("class(%d)", int(c))
}

// Permits reports whether an endpoint approved for c may connect to an address of class target.
func (c Class) Permits(target Class) bool {
	if c == ClassNever || target == ClassNever {
		return false
	}
	return target >= c
}

// ParseClass reads the stored form of a class ("never", "private", "public").
func ParseClass(s string) (Class, bool) {
	switch s {
	case "never":
		return ClassNever, true
	case "private":
		return ClassPrivate, true
	case "public":
		return ClassPublic, true
	}
	return ClassNever, false
}

// interfaceAddrs lists the server's own addresses; tests replace it.
var interfaceAddrs = net.InterfaceAddrs

// isOwnAddr reports whether ip is assigned to a local interface (the server itself, seen through its LAN address).
func isOwnAddr(ip net.IP) bool {
	addrs, err := interfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		var own net.IP
		switch v := a.(type) {
		case *net.IPNet:
			own = v.IP
		case *net.IPAddr:
			own = v.IP
		}
		if own != nil && own.Equal(ip) {
			return true
		}
	}
	return false
}

// ClassOf returns the class of ip as seen by the operator policy, including the operator allow-list
// (AllowCIDRsEnv re-admits loopback and the server's own addresses as ClassPrivate).
func ClassOf(ip net.IP) Class {
	if ip == nil {
		return ClassNever
	}
	if v4 := ip.To4(); v4 != nil {
		ip = v4 // unwraps ::ffff:a.b.c.d
	}
	if alwaysBlocked(ip) {
		return ClassNever
	}
	for _, n := range reservedNets {
		if n.Contains(ip) {
			return ClassNever
		}
	}
	for _, n := range operatorMetadataNets {
		if n.Contains(ip) {
			return ClassNever
		}
	}
	if v4 := embeddedV4(ip); v4 != nil {
		return ClassOf(v4)
	}
	if v4 := ip.To4(); v4 != nil && (v4[0] == 0 || v4[0] >= 240) {
		return ClassNever
	}
	if ip.IsLoopback() || isOwnAddr(ip) {
		if allowListed(ip) {
			return ClassPrivate
		}
		return ClassNever
	}
	if ip.IsPrivate() || cgnat.Contains(ip) {
		return ClassPrivate
	}
	return ClassPublic
}

// permitted reports whether the operator policy lets an endpoint approved for approved connect to ip.
func permitted(ip net.IP, approved Class, deny bool) bool {
	c := ClassOf(ip)
	if c == ClassNever {
		return false
	}
	if c == ClassPrivate && deny && !allowListed(ip) {
		return false
	}
	return approved.Permits(c)
}

// classPin carries the class ceiling of ONE transport (one storage root / one client). It starts at the approved
// class; the first successful connection tightens it to the class that connection reached: a transport whose root
// resolved to a public address can never afterwards reach the LAN, whether through a redirect, a second hop or a
// DNS answer that changes later (trust on first connection). Libraries that own their redirect policy (gowebdav)
// are covered because the check sits in the dial, not in the redirect function.
type classPin struct {
	mu      sync.Mutex
	ceiling Class
	seen    bool // the first connection has been recorded
}

func newClassPin(approved Class) *classPin { return &classPin{ceiling: approved} }

func (p *classPin) get() Class {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ceiling
}

// note records that a connection reached class c. Only the FIRST connection of the transport counts: it is the root
// endpoint the transport was built for; a later hop (a LAN root that redirects to a public host) must not tighten it.
func (p *classPin) note(c Class) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.seen {
		return
	}
	p.seen = true
	if c == ClassPublic && p.ceiling == ClassPrivate {
		p.ceiling = ClassPublic
	}
}
