package main

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// SECQ-3 wiring: the pprof group must be guarded, and the guard must sit behind authentication and the administrator
// check (so an anonymous caller never reaches it, and removing the guard or reordering it is caught). main.go cannot be
// imported, so the registration statement is read from source; the behaviour of the guard itself is tested in
// middleware/pprof_guard_test.go.
func TestPprofGroupIsGuardedBehindAuthAndAdmin(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	m := regexp.MustCompile(`pprofGroup\.Use\(([^\n]*)\)`).FindStringSubmatch(string(src))
	require.NotNil(t, m, "main.go must register middleware on pprofGroup with a single Use(...) statement")
	use := m[1]
	auth := strings.Index(use, "RequireAuth()")
	admin := strings.Index(use, "RequireAdmin()")
	guard := strings.Index(use, "PprofSamplingGuard(")
	require.GreaterOrEqual(t, auth, 0, "RequireAuth missing from the pprof group")
	require.GreaterOrEqual(t, admin, 0, "RequireAdmin missing from the pprof group")
	require.GreaterOrEqual(t, guard, 0, "PprofSamplingGuard missing from the pprof group")
	require.True(t, auth < admin && admin < guard, "order must be RequireAuth, RequireAdmin, PprofSamplingGuard: %s", use)
}
