package repository

import (
	"context"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/media/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-SQLite visibility matrix for ListVisible (mutation M6: dropping the public branch must fail here).
func TestMediaCollectionRepository_ListVisible_RealSQLite(t *testing.T) {
	dir := t.TempDir()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "v.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))
	repo := NewMediaCollectionRepository(db)
	ctx := context.Background()
	id := func(v int64) *int64 { return &v }
	add := func(name string, owner *int64, public bool) {
		_, err := repo.Create(ctx, &models.MediaCollection{Name: name, CollectionType: "custom", OwnerUserID: owner, IsPublic: public})
		require.NoError(t, err)
	}
	add("legacy", nil, false)
	add("alice-private", id(1), false)
	add("alice-public", id(1), true)
	add("bob-private", id(2), false)
	names := func(viewer int64, admin bool) []string {
		cs, total, err := repo.ListVisible(ctx, viewer, admin, 100, 0)
		require.NoError(t, err)
		assert.Equal(t, len(cs), total)
		var out []string
		for _, c := range cs {
			out = append(out, c.Name)
		}
		return out
	}
	assert.ElementsMatch(t, []string{"legacy", "alice-private", "alice-public"}, names(1, false))
	assert.ElementsMatch(t, []string{"legacy", "alice-public", "bob-private"}, names(2, false), "bob sees alice's PUBLIC collection")
	assert.ElementsMatch(t, []string{"legacy", "alice-public"}, names(3, false), "a stranger sees legacy and public only")
	assert.Len(t, names(3, true), 4, "admin sees everything")
}
