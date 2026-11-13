package filesystem

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strings"
	"time"
	"unicode"

	"catalogizer/internal/netguard"
	"catalogizer/models"
)

// SEC-DIAL-1: every storage connector that dials or mounts an operator-supplied host does so through ONE policy, netguard's operator class
// (loopback, link-local, cloud metadata, reserved and own addresses are refused; RFC 1918, ULA, CGNAT and public addresses are allowed, as for
// WebDAV and S3 roots). The check happens at root creation (VetStorageRoot, a clear early refusal) and again at EVERY dial, on the address
// that is connected to (the dial is pinned to the vetted IP, never the name), so a name that resolves differently later cannot get through.

// ErrStorageHostRefused is wrapped by every refusal of a storage host by the operator-class policy. Its text carries no address or DNS detail.
var ErrStorageHostRefused = errors.New("storage host is not allowed")

// storageDialContext is the dialer of every network storage connector (smb, ftp). Tests replace it to observe what is dialled.
var storageDialContext = netguard.OperatorDialContext

// storageOperatorResolve is the resolver of the NFS mount path. Tests replace it.
var storageOperatorResolve = netguard.OperatorResolve

// ValidateStorageHostSyntax accepts only an IP literal (no zone, no brackets) or a plain DNS name (letters, digits, '-', '_', '.', not starting
// with '-'). Colons other than in an IP literal, userinfo, slashes, whitespace, commas and control characters are refused, so no value can
// smuggle another address, port, option or flag into a dial string, a mount source or a command line.
func ValidateStorageHostSyntax(host string) error {
	if host == "" || len(host) > 253 || host[0] == '-' {
		return fmt.Errorf("invalid host")
	}
	if net.ParseIP(host) != nil {
		return nil
	}
	for _, r := range host {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !ok {
			return fmt.Errorf("invalid host")
		}
	}
	return nil
}

// ValidateNFSExport refuses an export path with a control character (newline, NUL, tab, DEL ...) or a leading '-'. A space or a ',' is a legal
// part of a share name ("/volume1/My Videos"): the export only ever forms the SOURCE of mount(2) (one string, never option-parsed; the server
// address always comes first) or one argument of an exec without a shell on darwin, so it cannot inject an option or another argument.
func ValidateNFSExport(export string) error {
	if export == "" || strings.HasPrefix(export, "-") {
		return fmt.Errorf("invalid nfs export path")
	}
	for _, r := range export {
		if unicode.IsControl(r) {
			return fmt.Errorf("invalid nfs export path")
		}
	}
	return nil
}

// resolveNFSServer resolves the NFS server under the operator-class policy and returns the first vetted address, the address the kernel is
// handed instead of the name. A policy refusal is ErrStorageHostRefused; a name that does not resolve is reported as such (not as a refusal).
// Shared by the linux and darwin mount paths.
func resolveNFSServer(ctx context.Context, host string) (net.IP, error) {
	ips, err := storageOperatorResolve(ctx, host)
	if errors.Is(err, netguard.ErrBlocked) {
		return nil, fmt.Errorf("%w", ErrStorageHostRefused)
	}
	if err != nil || len(ips) == 0 {
		return nil, fmt.Errorf("nfs host does not resolve")
	}
	return ips[0], nil
}

// addrCandidate matches a run of characters an IP literal is made of; scrubResolvedAddrs tests each run.
var addrCandidate = regexp.MustCompile(`[0-9A-Fa-f:.]{2,}`)

// scrubResolvedAddrs replaces every IP literal in msg other than the configured host with "<address>", so an error shown to a non-admin caller
// (/storage-roots/:id/status) does not disclose the address a name resolved to. The configured host itself is operator input and stays.
func scrubResolvedAddrs(msg, host string) string {
	host = strings.Trim(host, "[]")
	return addrCandidate.ReplaceAllStringFunc(msg, func(raw string) string {
		// sentence punctuation around the literal ("...:445: connection refused", "10.0.0.1.") is not part of it
		c := strings.TrimRight(raw, ":.")
		tail := raw[len(c):]
		ip, port := c, ""
		if net.ParseIP(ip) == nil {
			if i := strings.LastIndex(c, ":"); i > 0 && strings.Contains(c[:i], ".") {
				ip, port = c[:i], c[i:]
			}
		}
		parsed := net.ParseIP(ip)
		if parsed == nil || ip == host || (net.ParseIP(host) != nil && parsed.Equal(net.ParseIP(host))) {
			return raw
		}
		return "<address>" + port + tail
	})
}

// addrScrubbedError carries a message with the resolved addresses removed and keeps the cause inspectable (errors.Is / As).
type addrScrubbedError struct {
	msg string
	err error
}

func (e *addrScrubbedError) Error() string { return e.msg }
func (e *addrScrubbedError) Unwrap() error { return e.err }

// scrubDialErr returns err with the resolved addresses removed from its text; nil stays nil.
func scrubDialErr(err error, host string) error {
	if err == nil {
		return nil
	}
	m := err.Error()
	if s := scrubResolvedAddrs(m, host); s != m {
		return &addrScrubbedError{msg: s, err: err}
	}
	return err
}

// storageDial is the one call every storage connector uses to dial through storageDialContext; the returned error never names a resolved address.
func storageDial(ctx context.Context, network, addr string) (net.Conn, error) {
	return scrubDialFunc(storageDialContext)(ctx, network, addr)
}

// NFSOption is one parsed "key" or "key=value" mount option.
type NFSOption struct {
	Key, Value string
	HasValue   bool
}

// ParseNFSOptions parses a mount option string. The grammar is closed: comma-separated elements, each "key" or "key=value", key
// [A-Za-z0-9_], value [A-Za-z0-9_.:-]; no empty element, no whitespace, no newline, no leading '-'. Anything else could add an option the
// operator did not write, so it is refused. An empty string yields no options.
func ParseNFSOptions(opts string) ([]NFSOption, error) {
	if opts == "" {
		return nil, nil
	}
	var out []NFSOption
	for _, el := range strings.Split(opts, ",") {
		key, val, has := strings.Cut(el, "=")
		if key == "" || (has && val == "") {
			return nil, fmt.Errorf("invalid nfs options")
		}
		for _, r := range key {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
				return nil, fmt.Errorf("invalid nfs options")
			}
		}
		for _, r := range val {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' || r == '.' || r == ':' || r == '-') {
				return nil, fmt.Errorf("invalid nfs options")
			}
		}
		out = append(out, NFSOption{Key: key, Value: val, HasValue: has})
	}
	return out, nil
}

// nfsAddrOptions are the mount options that name a server address; each value is an address the kernel connects to and is vetted like the host.
var nfsAddrOptions = map[string]bool{"addr": true, "mountaddr": true}

// validateStorageSyntax is the pure (no network) host / export / option check of the built-in network protocols. ValidateSettings runs it, so
// ValidateRoot and the client factory both refuse a malformed value.
func validateStorageSyntax(protocol string, settings map[string]interface{}) error {
	switch protocol {
	case "smb", "ftp":
		if h, ok := settings[KeyHost].(string); ok {
			return ValidateStorageHostSyntax(h)
		}
	case "nfs":
		if h, ok := settings[KeyHost].(string); ok {
			if err := ValidateStorageHostSyntax(h); err != nil {
				return err
			}
		}
		if p, ok := settings[KeyPath].(string); ok {
			if err := ValidateNFSExport(p); err != nil {
				return err
			}
		}
		if o, ok := settings[KeyOptions].(string); ok {
			if _, err := ParseNFSOptions(o); err != nil {
				return err
			}
		}
	}
	return nil
}

// VetStorageRoot is the create-time network check of a smb / ftp / nfs root: the host (and, for nfs, every address named by a mount option)
// must pass the operator-class policy. Other protocols are not this function's concern (webdav and s3 are checked by their URL). The error
// wraps ErrStorageHostRefused and never names an address.
func VetStorageRoot(ctx context.Context, root *models.StorageRoot) error {
	switch root.Protocol {
	case "smb", "ftp", "nfs":
	default:
		return nil
	}
	host := deref(root.Host)
	if err := ValidateStorageHostSyntax(host); err != nil {
		return fmt.Errorf("%w", ErrStorageHostRefused)
	}
	// Only a POLICY refusal is refused here. A name that does not resolve yet (NAS powered off, DNS not up) is accepted, exactly like a WebDAV
	// URL: every dial re-resolves and re-checks, so an unresolvable name can never reach a network address it was not vetted for.
	if err := netguard.CheckOperatorHost(ctx, host); err != nil && errors.Is(err, netguard.ErrBlocked) {
		return fmt.Errorf("%w", ErrStorageHostRefused)
	}
	if root.Protocol == "nfs" {
		opts, err := ParseNFSOptions(deref(root.Options))
		if err != nil {
			return fmt.Errorf("%w", ErrStorageHostRefused)
		}
		for _, o := range opts {
			if nfsAddrOptions[o.Key] {
				if net.ParseIP(o.Value) == nil || errors.Is(netguard.CheckOperatorHost(ctx, o.Value), netguard.ErrBlocked) {
					return fmt.Errorf("%w", ErrStorageHostRefused)
				}
			}
		}
	}
	return nil
}

// dialStorage connects to host:port through the operator-class dialer, bounded by timeout. The address connected to is the vetted one.
func dialStorage(ctx context.Context, host, port string, timeout time.Duration) (net.Conn, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return storageDial(ctx, "tcp", net.JoinHostPort(host, port))
}

// scrubDialFunc wraps a dial function so the error it returns never names a resolved address (the configured host stays).
func scrubDialFunc(inner func(ctx context.Context, network, addr string) (net.Conn, error)) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		c, err := inner(ctx, network, addr)
		if err != nil {
			h, _, serr := net.SplitHostPort(addr)
			if serr != nil {
				h = addr
			}
			return nil, scrubDialErr(err, h)
		}
		return c, nil
	}
}

// operatorStorageTransport is netguard.OperatorTransport with a dial whose errors do not name the resolved address (WebDAV, S3).
func operatorStorageTransport() *http.Transport {
	tr := netguard.OperatorTransport()
	tr.DialContext = scrubDialFunc(tr.DialContext)
	return tr
}
