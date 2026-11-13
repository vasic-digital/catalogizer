package services

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain widens the path-confinement roots (pathguard) to the fixture paths the legacy
// unit tests use. The TestPathConfine_* tests narrow them again with t.Setenv, so the
// confinement itself stays under test.
func TestMain(m *testing.M) {
	const roots = "/input:/output:/tmp:/nonexistent:/media:/test:/data:/source:/remote:/local:/old:/new"
	os.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8,::1/128") // SECQ-1/W5: loopback WebDAV fixtures
	os.Setenv("CATALOGIZER_SYNC_ROOTS", roots)
	os.Setenv("CATALOGIZER_CONVERSION_ROOTS", roots)
	// NewConfigurationWizardService(nil) creates its config dirs; keep them out of the package directory (OBF-FU).
	cfgDir, mkErr := os.MkdirTemp("", "obf-wizard-config-")
	if mkErr != nil {
		panic(mkErr)
	}
	os.Setenv("CONFIG_PATH", cfgDir)
	code := m.Run()
	cleanupProdBinary()
	if cfgDir != "" && filepath.IsAbs(cfgDir) {
		os.RemoveAll(cfgDir)
	}
	os.Exit(code)
}
