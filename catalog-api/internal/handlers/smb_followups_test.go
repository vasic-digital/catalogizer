package handlers

import (
	"sync"
	"testing"

	"catalogizer/internal/services"

	"github.com/stretchr/testify/require"
)

var smbHandlerGlobalsMu sync.Mutex

// withSMBConfiguredPorts replaces the configured-port registry under a lock and restores it (SMB-FU-14). Tests using it must not call
// t.Parallel().
func withSMBConfiguredPorts(t *testing.T, ports []int) {
	t.Helper()
	smbHandlerGlobalsMu.Lock()
	old := services.SMBConfiguredPorts()
	t.Cleanup(func() { services.SetSMBConfiguredPorts(old); smbHandlerGlobalsMu.Unlock() })
	services.SetSMBConfiguredPorts(ports)
}

func TestValidateSMBTarget_ConfiguredPortAndPathRules(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredPorts(t, []int{1445})
	const host = "192.0.2.10"
	require.Equal(t, "", validateSMBTarget(host, 1445, "Data", "docs"))
	require.Equal(t, "Unsupported port", validateSMBTarget(host, 22, "Data", "docs"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "a:b"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "dir/CON.txt"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "../x"))
	require.Equal(t, "", validateSMBTarget(host, 445, "Data", "config.txt"))
}
