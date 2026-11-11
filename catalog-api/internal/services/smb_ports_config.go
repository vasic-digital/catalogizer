package services

import (
	"sync"

	"catalogizer/internal/config"
)

// SMB-FU-12: ports of the configured SMB storage roots (config.SMB.Hosts[].Port) are dialable by the discovery routes in addition to 445 and
// CATALOG_SMB_ALLOWED_PORTS. Only these exact ports are added; every other port stays refused.
var (
	smbConfiguredPortsMu sync.RWMutex
	smbConfiguredPorts   []int
)

// SetSMBConfiguredPorts replaces the configured-port set. Entries outside 1-65535 are ignored; 445 and duplicates are dropped.
func SetSMBConfiguredPorts(ports []int) {
	var clean []int
	seen := map[int]bool{smbPort: true}
	for _, p := range ports {
		if p < 1 || p > 65535 || seen[p] {
			continue
		}
		seen[p] = true
		clean = append(clean, p)
	}
	smbConfiguredPortsMu.Lock()
	smbConfiguredPorts = clean
	smbConfiguredPortsMu.Unlock()
}

// SMBConfiguredPorts returns a copy of the configured-port set.
func SMBConfiguredPorts() []int {
	smbConfiguredPortsMu.RLock()
	defer smbConfiguredPortsMu.RUnlock()
	return append([]int(nil), smbConfiguredPorts...)
}

// ConfigureSMBPortsFromConfig takes the ports of cfg.SMB.Hosts (a nil cfg clears the set; port 0 means the default and adds nothing).
func ConfigureSMBPortsFromConfig(cfg *config.Config) {
	if cfg == nil {
		SetSMBConfiguredPorts(nil)
		return
	}
	ports := make([]int, 0, len(cfg.SMB.Hosts))
	for _, h := range cfg.SMB.Hosts {
		ports = append(ports, h.Port)
	}
	SetSMBConfiguredPorts(ports)
}
