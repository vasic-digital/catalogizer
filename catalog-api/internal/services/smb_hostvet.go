package services

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"catalogizer/smb"
)

// SMBHostResolver resolves a host name for VetSMBHost. Tests replace it; production uses the system resolver.
var SMBHostResolver = net.LookupIP

// ValidateSMBHostSyntax accepts only an IPv4 literal or a plain DNS name (letters, digits, '-', '_', '.'). Brackets, colons (IPv6
// literals and host:port), userinfo, slashes, whitespace and zone ids are refused, so no value can smuggle another address or port into
// a "host:port" dial string.
func ValidateSMBHostSyntax(host string) error {
	if host == "" || len(host) > 253 {
		return fmt.Errorf("%w: host", ErrInvalidSMBInput)
	}
	for _, r := range host {
		ok := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.'
		if !ok {
			return fmt.Errorf("%w: host", ErrInvalidSMBInput)
		}
	}
	return nil
}

var blockedExact = []net.IP{
	net.ParseIP("169.254.169.254"), // cloud metadata
	net.ParseIP("100.100.100.200"), // Alibaba metadata
	net.ParseIP("168.63.129.16"),   // Azure wire server
	net.ParseIP("fd00:ec2::254"),   // AWS IPv6 metadata
}

// embeddedV4 returns the IPv4 address carried by an IPv4-mapped (::ffff:a.b.c.d), IPv4-compatible (::a.b.c.d) or NAT64 (64:ff9b::/96)
// IPv6 address, or nil.
func embeddedV4(ip net.IP) net.IP {
	if v4 := ip.To4(); v4 != nil {
		return v4
	}
	ip16 := ip.To16()
	if ip16 == nil {
		return nil
	}
	allZero := true
	for _, b := range ip16[:12] {
		if b != 0 {
			allZero = false
		}
	}
	if allZero {
		return ip16[12:]
	}
	if bytes.Equal(ip16[:12], net.ParseIP("64:ff9b::").To16()[:12]) {
		return ip16[12:]
	}
	return nil
}

func smbIPBlockedBase(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	for _, b := range blockedExact {
		if ip.Equal(b) {
			return true
		}
	}
	return false
}

func smbIPBlocked(ip net.IP) bool {
	if ip == nil || smbIPBlockedBase(ip) {
		return true
	}
	if v4 := embeddedV4(ip); v4 != nil {
		return v4[0] == 0 || smbIPBlockedBase(v4) // 0.0.0.0/8 and the embedded address itself
	}
	return false
}

// VetSMBHost is the ONE host check for every discovery/probe route: syntax, then resolve (an IPv4 literal is its own answer), then
// refuse if the lookup fails or ANY resolved address is blocked (loopback, link-local, multicast, unspecified, 0/8, cloud metadata,
// embedded-IPv4 forms of those). RFC1918 stays allowed (LAN NAS targets).
func VetSMBHost(host string) error {
	if err := ValidateSMBHostSyntax(host); err != nil {
		return err
	}
	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		var err error
		ips, err = SMBHostResolver(host)
		if err != nil || len(ips) == 0 {
			return fmt.Errorf("%w: host does not resolve", ErrInvalidSMBInput)
		}
	}
	for _, ip := range ips {
		if smbIPBlocked(ip) {
			return fmt.Errorf("%w: host not allowed", ErrInvalidSMBInput)
		}
	}
	return nil
}

// SMBAllowedPorts is {445} plus the ports in CATALOG_SMB_ALLOWED_PORTS (comma list, each 1-65535). Any invalid entry makes the whole
// value invalid and the result falls back to the default {445} (fail closed).
func SMBAllowedPorts() []int {
	out := []int{smbPort}
	raw := strings.TrimSpace(os.Getenv("CATALOG_SMB_ALLOWED_PORTS"))
	if raw == "" {
		return out
	}
	var extra []int
	for _, f := range strings.Split(raw, ",") {
		p, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || p < 1 || p > 65535 {
			return []int{smbPort}
		}
		extra = append(extra, p)
	}
	return append(out, extra...)
}

// filterShareInfos turns SRVSVC share names into SMBShareInfo, dropping every share smb.SplitShare would refuse (IPC$, ADMIN$, drive shares).
func filterShareInfos(host string, names []string) []SMBShareInfo {
	var out []SMBShareInfo
	for _, n := range names {
		if _, _, err := smb.SplitShare(n); err != nil {
			continue
		}
		out = append(out, SMBShareInfo{
			Host:        host,
			ShareName:   n,
			Path:        fmt.Sprintf("\\\\%s\\%s", host, n),
			Writable:    false,
			Description: getShareDescription(n),
		})
	}
	return out
}
