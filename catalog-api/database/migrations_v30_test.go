package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-HAUTHZ-F2: migration v30 creates user_favorites through the real RunMigrations chain and leaves the legacy
// media_items.is_favorite data untouched (it is not attributed to any user).
func TestMigrationV30_UserFavorites(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	var applied int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 30`).Scan(&applied))
	assert.Equal(t, 1, applied)

	_, err := raw.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'a', 'a@x.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO media_items (media_type_id, title, status, is_favorite) VALUES (1, 'T', 'detected', 1)`)
	require.NoError(t, err)

	var legacyRows int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_favorites`).Scan(&legacyRows))
	assert.Zero(t, legacyRows, "legacy is_favorite flags are not attributed to any user")

	_, err = raw.Exec(`INSERT INTO user_favorites (user_id, media_id) VALUES (1, 1)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO user_favorites (user_id, media_id) VALUES (1, 1)`)
	assert.Error(t, err, "(user_id, media_id) is the primary key")

	// idempotent: whole chain again keeps the row; legacy column untouched
	require.NoError(t, db.createUserFavoritesTable(ctx))
	require.NoError(t, db.RunMigrations(ctx))
	var rows, legacy int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_favorites`).Scan(&rows))
	assert.Equal(t, 1, rows)
	require.NoError(t, raw.QueryRow(`SELECT is_favorite FROM media_items WHERE id = 1`).Scan(&legacy))
	assert.Equal(t, 1, legacy)

	// cascade: deleting the user removes its favorites (foreign keys are on in the real connection)
	_, err = raw.Exec(`PRAGMA foreign_keys = ON`)
	require.NoError(t, err)
	_, err = raw.Exec(`DELETE FROM users WHERE id = 1`)
	require.NoError(t, err)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_favorites`).Scan(&rows))
	assert.Zero(t, rows)
}

func TestMigrationV30_UpgradeFromV29(t *testing.T) {
	db, raw := newMigratedDB(t)
	_, err := raw.Exec(`DROP TABLE user_favorites`)
	require.NoError(t, err)
	_, err = raw.Exec(`DELETE FROM migrations WHERE version = 30`)
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(context.Background()))
	var n int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_favorites'`).Scan(&n))
	assert.Equal(t, 1, n)
}
