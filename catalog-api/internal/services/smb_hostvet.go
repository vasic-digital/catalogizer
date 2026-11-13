package services

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"catalogizer/internal/netguard"
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
	net.ParseIP("169.254.169.254"), // cloud metadata (also link-local, refused above)
	net.ParseIP("168.63.129.16"),   // Azure wire server
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
	if allZero { // IPv4-compatible ::a.b.c.d
		return ip16[12:]
	}
	if bytes.Equal(ip16[:12], net.ParseIP("64:ff9b::").To16()[:12]) { // NAT64 64:ff9b::/96
		return ip16[12:]
	}
	if ip16[0] == 0x20 && ip16[1] == 0x02 { // 6to4 2002::/16: the IPv4 address is bytes 2..5
		return ip16[2:6]
	}
	// SIIT ::ffff:0:a.b.c.d (IPv4-translated, ::ffff:0:0/96)
	if bytes.Equal(ip16[:8], make([]byte, 8)) && ip16[8] == 0xff && ip16[9] == 0xff && ip16[10] == 0 && ip16[11] == 0 {
		return ip16[12:]
	}
	return nil
}

var (
	nat64LocalNe = &net.IPNet{IP: net.ParseIP("64:ff9b:1::"), Mask: net.CIDRMask(48, 128)}
)

func smbIPBlockedBase(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || ip.IsUnspecified() {
		return true
	}
	if netguard.IsMetadataSingleton(ip) { // the ONE shared metadata singleton list (AWS/GCE IPv6, Alibaba): cannot diverge from netguard
		return true
	}
	for _, b := range blockedExact {
		if ip.Equal(b) {
			return true
		}
	}
	if ip.Equal(net.IPv4bcast) || nat64LocalNe.Contains(ip) {
		return true // limited broadcast, local-use NAT64 64:ff9b:1::/48
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

// VetSMBHostIPs is the ONE host check for every discovery/probe route: syntax, then resolve (an IPv4 literal is its own answer), then
// refuse if the lookup fails or ANY resolved address is blocked (loopback, link-local, multicast, unspecified, 0/8, cloud metadata,
// embedded-IPv4 forms of those). RFC1918 stays allowed (LAN NAS targets). It returns the vetted addresses, in resolver order; callers
// MUST dial exactly these addresses and never re-resolve the name (SMB-FU-9: DNS rebinding between vet and dial).
func VetSMBHostIPs(host string) ([]net.IP, error) {
	if err := ValidateSMBHostSyntax(host); err != nil {
		return nil, err
	}
	ips := []net.IP{}
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		var err error
		ips, err = SMBHostResolver(host)
		if err != nil || len(ips) == 0 {
			return nil, fmt.Errorf("%w: host does not resolve", ErrInvalidSMBInput)
		}
	}
	for _, ip := range ips {
		if smbIPBlocked(ip) {
			return nil, fmt.Errorf("%w: host not allowed", ErrInvalidSMBInput)
		}
	}
	return ips, nil
}

// VetSMBHost reports whether host passes VetSMBHostIPs; it does not return the addresses, so it is for pre-checks only (handlers). A
// dial must use VetSMBHostIPs and its result.
func VetSMBHost(host string) error {
	_, err := VetSMBHostIPs(host)
	return err
}

// SMBAllowedPorts is the GLOBAL allowlist: {445} plus the ports in CATALOG_SMB_ALLOWED_PORTS (comma list, each 1-65535). Any invalid entry makes
// the whole value invalid and the result falls back to {445} (fail closed). Ports of configured storage roots are per host, see ValidateSMBPortForHost.
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
			warnInvalidSMBPorts(raw)
			return out
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

var (
	// smbWarnf reports a configuration problem; tests replace it.
	smbWarnf         = log.Printf
	smbWarnedRaw     string
	smbWarnedRawLock sync.Mutex
)

// warnInvalidSMBPorts logs once per distinct invalid CATALOG_SMB_ALLOWED_PORTS value that the allowlist fell back to {445}.
func warnInvalidSMBPorts(raw string) {
	smbWarnedRawLock.Lock()
	defer smbWarnedRawLock.Unlock()
	if smbWarnedRaw == raw {
		return
	}
	smbWarnedRaw = raw
	smbWarnf("WARNING: CATALOG_SMB_ALLOWED_PORTS=%q is invalid (comma list of ports 1-65535 expected); falling back to the default port 445 only", raw)
}
