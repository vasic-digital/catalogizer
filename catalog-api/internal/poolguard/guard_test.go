package poolguard

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// allowlist: explicit, justified exceptions, keyed file:function. Anything not listed here that the
// scanner finds in non-test catalog-api code is a pool-deadlock class member and fails the build.
var allowlist = map[string]string{
	// Runs entirely on ONE *sql.Tx: a transaction pins a single connection, so the nested ExecContext
	// reuses it and never asks the pool for a second one (no pool-deadlock possible).
	"internal/services/playlist_service.go:PlaylistService.RefreshSmartPlaylist": "single-tx, one pinned connection",
}

// TestPoolClassGuard_NoNestedPooledQueryInsideRowsLoop fails when non-test code in catalog-api runs a
// pooled query/exec (directly or through a same-package helper, any depth up to 3) inside a
// `for rows.Next()` loop body. Fix such code by buffering the page and closing the cursor first.
func TestPoolClassGuard_NoNestedPooledQueryInsideRowsLoop(t *testing.T) {
	found, err := Scan("../..")
	require.NoError(t, err)
	var bad []string
	for _, f := range found {
		if _, ok := allowlist[f.Key()]; ok {
			continue
		}
		bad = append(bad, f.String())
	}
	sort.Strings(bad)
	require.Empty(t, bad, "pool-deadlock class members (query inside an open rows cursor):\n%s", strings.Join(bad, "\n"))
}

// Control needles: the instrument must see the positive fixtures and must not flag the negative one.
func TestPoolClassGuard_ControlNeedles(t *testing.T) {
	pos, err := Scan("testdata/positive")
	require.NoError(t, err)
	got := map[string]Finding{}
	for _, f := range pos {
		got[f.Func] = f
	}
	require.Contains(t, got, "Direct")
	require.False(t, got["Direct"].Indirect)
	require.Contains(t, got, "Svc.List")
	require.True(t, got["Svc.List"].Indirect, "pooled query two helpers deep must be found")
	require.Contains(t, got["Svc.List"].Via, "tags")
	require.Contains(t, got, "Svc.ExecPerRow")

	neg, err := Scan("testdata/negative")
	require.NoError(t, err)
	require.Empty(t, neg, "buffered shape and non-cursor loops must not be flagged: %v", neg)
}

// The allowlist must not rot: every entry must still correspond to a real finding.
func TestPoolClassGuard_AllowlistEntriesStillMatch(t *testing.T) {
	found, err := Scan("../..")
	require.NoError(t, err)
	seen := map[string]bool{}
	for _, f := range found {
		seen[f.Key()] = true
	}
	for k := range allowlist {
		require.True(t, seen[k], "stale allowlist entry %q (remove it)", k)
	}
}
