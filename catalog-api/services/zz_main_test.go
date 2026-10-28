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
	os.Setenv("CATALOGIZER_SYNC_ROOTS", roots)
	os.Setenv("CATALOGIZER_CONVERSION_ROOTS", roots)
	os.Exit(m.Run())
}
