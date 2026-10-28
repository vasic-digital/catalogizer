package services

import (
	"os"
	"strings"

	"catalogizer/internal/pathguard"
)

// env names, via the package constants so a rename cannot silently desync the tests
const (
	pathGuardEnvSync = pathguard.EnvSyncRoots
	pathGuardEnvRead = pathguard.EnvReadRoots
	pathGuardEnvConv = pathguard.EnvConversionRoots
)

// allowSyncDirs makes dirs the sync write roots for one test through the real configuration
// variable (CATALOGIZER_SYNC_ROOTS); write roots are readable too. t.TempDir() is under
// $TMPDIR, which is not one of the fixed fixture roots TestMain configures.
func allowSyncDirs(t interface{ Setenv(string, string) }, dirs ...string) {
	t.Setenv(pathGuardEnvSync, strings.Join(dirs, string(os.PathListSeparator)))
}
