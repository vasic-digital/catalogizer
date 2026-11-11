package services

import (
	"net"
	"strings"
	"sync"

	"catalogizer/internal/config"

	"go.uber.org/zap"
)

// SMB-FU-12: a non-default port of a configured SMB storage root (config.SMB.Hosts[]) is dialable by the discovery routes ONLY for that
// root's host, never for other hosts. 445 and CATALOG_SMB_ALLOWED_PORTS stay global. The set is read once at start-up (see
// NewSMBDiscoveryServiceFromConfig); a root created at runtime needs a restart before discovery accepts its non-default port.

// SMBRootPort binds one configured port to the host of its storage root.
type SMBRootPort struct {
	Host string
	Port int
}

var (
	smbConfiguredRootsMu sync.RWMutex
	smbConfiguredRoots   []SMBRootPort
)

// SetSMBConfiguredRoots replaces the set. Entries with an empty or syntactically invalid host, or a port outside 1-65535 or equal to
// 445, are ignored; duplicates are dropped.
func SetSMBConfiguredRoots(roots []SMBRootPort) {
	var clean []SMBRootPort
	seen := map[SMBRootPort]bool{}
	for _, r := range roots {
		r.Host = strings.TrimSpace(r.Host)
		if r.Port < 1 || r.Port > 65535 || r.Port == smbPort || ValidateSMBHostSyntax(r.Host) != nil {
			continue
		}
		key := SMBRootPort{Host: strings.ToLower(r.Host), Port: r.Port}
		if seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, r)
	}
	smbConfiguredRootsMu.Lock()
	smbConfiguredRoots = clean
	smbConfiguredRootsMu.Unlock()
}

// SMBConfiguredRoots returns a copy of the set.
func SMBConfiguredRoots() []SMBRootPort {
	smbConfiguredRootsMu.RLock()
	defer smbConfiguredRootsMu.RUnlock()
	return append([]SMBRootPort(nil), smbConfiguredRoots...)
}

// ConfigureSMBPortsFromConfig registers the (host, port) pairs of cfg.SMB.Hosts (a nil cfg clears the set).
func ConfigureSMBPortsFromConfig(cfg *config.Config) {
	if cfg == nil {
		SetSMBConfiguredRoots(nil)
		return
	}
	roots := make([]SMBRootPort, 0, len(cfg.SMB.Hosts))
	for _, h := range cfg.SMB.Hosts {
		roots = append(roots, SMBRootPort{Host: h.Host, Port: h.Port})
	}
	SetSMBConfiguredRoots(roots)
}

// NewSMBDiscoveryServiceFromConfig is the production constructor: it registers the configured storage-root ports and then builds the
// service. main.go must use it (TestMainWiresSMBConfiguredPorts guards that).
func NewSMBDiscoveryServiceFromConfig(cfg *config.Config, logger *zap.Logger) *SMBDiscoveryService {
	ConfigureSMBPortsFromConfig(cfg)
	return NewSMBDiscoveryService(logger)
}

// smbHostIPSet resolves host the way vetting does (an IPv4 literal is its own answer); a failed lookup yields an empty set.
func smbHostIPSet(host string) []net.IP {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}
	ips, err := SMBHostResolver(host)
	if err != nil {
		return nil
	}
	return ips
}

// smbSameHost reports whether the request host is the configured root host: same name (case-insensitive) or at least one common
// resolved address.
func smbSameHost(reqHost, rootHost string) bool {
	if strings.EqualFold(reqHost, rootHost) {
		return true
	}
	req := smbHostIPSet(reqHost)
	if len(req) == 0 {
		return false
	}
	for _, a := range smbHostIPSet(rootHost) {
		for _, b := range req {
			if a.Equal(b) {
				return true
			}
		}
	}
	return false
}

// ValidateSMBPortForHost accepts what ValidateSMBPort accepts, plus a port configured for a storage root whose host is this host.
func ValidateSMBPortForHost(host string, port int) error {
	err := ValidateSMBPort(port)
	if err == nil {
		return nil
	}
	for _, r := range SMBConfiguredRoots() {
		if r.Port == port && smbSameHost(host, r.Host) {
			return nil
		}
	}
	return err
}
