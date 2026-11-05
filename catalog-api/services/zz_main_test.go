package services

import (
	"os"
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
	code := m.Run()
	cleanupProdBinary()
	os.Exit(code)
}
