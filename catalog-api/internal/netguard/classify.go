package netguard

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ClassifyHost derives the class an operator-chosen endpoint is approved for (CAT-SECQ1-B / W3) from its host, an IP literal
// or a DNS name. A host whose every address is public is approved for ClassPublic: its transport may then never reach a
// private address (a redirect or a rebound DNS answer into the LAN is refused at the dial). A host with at least one private
// address is approved for ClassPrivate (the pre-SECQ-1 behaviour: private and public are reachable).
//
// The result is a decision about the addresses seen NOW; the first-connection pin and the stored class still apply when the
// name later resolves differently. A host that resolves to, or is, a ClassNever address (loopback, link-local, metadata,
// the server's own address, unless allow-listed), an empty host, a non-ASCII or numeric-spelling host, or a name that does
// not resolve yields an error: such an endpoint cannot be approved by classification, the caller keeps the stored default
// (private) and/or asks the operator.
func ClassifyHost(ctx context.Context, host string) (Class, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return ClassNever, fmt.Errorf("%w: empty host", ErrBlocked)
	}
	var ips []net.IP
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil {
		ips = []net.IP{ip}
	} else {
		if err := checkHostSpelling(host); err != nil {
			return ClassNever, err
		}
		addrs, err := lookupIPAddr(ctx, host)
		if err != nil {
			return ClassNever, fmt.Errorf("resolve %q: %w", host, err)
		}
		for _, a := range addrs {
			ips = append(ips, a.IP)
		}
	}
	if len(ips) == 0 {
		return ClassNever, fmt.Errorf("resolve %q: no addresses", host)
	}
	approved := ClassPublic
	for _, ip := range ips {
		switch ClassOf(ip) {
		case ClassNever:
			return ClassNever, fmt.Errorf("%w: %s is or resolves to %s, which no endpoint may be approved for", ErrBlocked, host, ip)
		case ClassPrivate:
			approved = ClassPrivate
		}
	}
	return approved, nil
}

// ClassifyEndpoint classifies a storage root or sync endpoint from its host field, or, when the host is empty, from the
// host of its URL.
func ClassifyEndpoint(ctx context.Context, host, rawURL string) (Class, error) {
	if strings.TrimSpace(host) != "" {
		return ClassifyHost(ctx, host)
	}
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return ClassNever, fmt.Errorf("%w: no host to classify", ErrBlocked)
	}
	return ClassifyHost(ctx, u.Hostname())
}
