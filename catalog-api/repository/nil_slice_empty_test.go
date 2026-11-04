package repository

import (
	"context"
	"encoding/json"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// Class defect: a list function that matched no rows returned a nil slice, which encoding/json writes as null where the API
// documents an array. Each case drives the real repository against a migrated in-memory database holding no rows for the query.
func TestEmptyListsAreNonNilAndSerialiseAsArray(t *testing.T) {
	db := newPlaybackTestDB(t)
	ctx := context.Background()

	stats := NewStatsRepository(db)
	groups, err := stats.GetTopDuplicateGroups(ctx, "count", 10, "")
	require.NoError(t, err)
	hist, _, err := stats.GetScanHistory(ctx, "", 10, 0)
	require.NoError(t, err)
	roots, err := NewFileRepository(db).GetStorageRoots(ctx)
	require.NoError(t, err)

	for name, v := range map[string]interface{}{
		"stats.GetTopDuplicateGroups": groups,
		"stats.GetScanHistory":        hist,
		"file.GetStorageRoots":        roots,
	} {
		b, err := json.Marshal(v)
		require.NoError(t, err, name)
		require.Equal(t, "[]", string(b), "%s must serialise as [] when there are no rows", name)
	}
}

func TestPermissionsNilSerialisesAsEmptyArray(t *testing.T) {
	var role models.Role
	b, err := json.Marshal(role)
	require.NoError(t, err)
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal(b, &m))
	require.Equal(t, []interface{}{}, m["permissions"], "a nil permission list must be [] not null")
}
