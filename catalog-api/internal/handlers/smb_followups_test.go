package handlers

import (
	"sync"
	"testing"

	"catalogizer/internal/services"

	"github.com/stretchr/testify/require"
)

var smbHandlerGlobalsMu sync.Mutex

// withSMBConfiguredRoots replaces the configured-root registry under a lock and restores it (SMB-FU-14). Tests using it must not call
// t.Parallel().
func withSMBConfiguredRoots(t *testing.T, roots []services.SMBRootPort) {
	t.Helper()
	smbHandlerGlobalsMu.Lock()
	old := services.SMBConfiguredRoots()
	t.Cleanup(func() { services.SetSMBConfiguredRoots(old); smbHandlerGlobalsMu.Unlock() })
	services.SetSMBConfiguredRoots(roots)
}

func TestValidateSMBTarget_ConfiguredPortAndPathRules(t *testing.T) {
	t.Setenv("CATALOG_SMB_ALLOWED_PORTS", "")
	withSMBConfiguredRoots(t, []services.SMBRootPort{{Host: "192.0.2.10", Port: 1445}})
	const host = "192.0.2.10"
	require.Equal(t, "", validateSMBTarget(host, 1445, "Data", "docs"))
	require.Equal(t, "Unsupported port", validateSMBTarget("192.0.2.11", 1445, "Data", "docs"), "configured port is bound to its host")
	require.Equal(t, "Unsupported port", validateSMBTarget(host, 22, "Data", "docs"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "a:b"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "dir/CON.txt"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "dir/COM²"))
	require.Equal(t, "Invalid path", validateSMBTarget(host, 445, "Data", "../x"))
	require.Equal(t, "", validateSMBTarget(host, 445, "Data", "config.txt"))
}
