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

// Upgrade of a populated v29 database: existing users, media (one with the legacy global flag) and unrelated rows survive
// v30; the legacy flag is neither migrated nor altered; user_favorites is created empty and usable.
func TestMigrationV30_UpgradeFromV29(t *testing.T) {
	db, raw := newMigratedDB(t)
	// Reconstruct a v29 state: v30 table and record gone, populated data present.
	_, err := raw.Exec(`DROP TABLE user_media_ratings`) // v31 (CAT-RATING-1) is newer than v29 too
	require.NoError(t, err)
	_, err = raw.Exec(`DROP TABLE user_favorites`)
	require.NoError(t, err)
	_, err = raw.Exec(`DELETE FROM migrations WHERE version IN (30, 31)`)
	require.NoError(t, err)
	var maxV int
	require.NoError(t, raw.QueryRow(`SELECT MAX(version) FROM migrations`).Scan(&maxV))
	require.Equal(t, 29, maxV, "the reconstructed database must be at v29")

	for i := 1; i <= 2; i++ {
		_, err = raw.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, i, "user"+string(rune('0'+i)), "u"+string(rune('0'+i))+"@x.test")
		require.NoError(t, err)
	}
	_, err = raw.Exec(`INSERT INTO media_items (media_type_id, title, status, is_favorite) VALUES (1, 'Flagged', 'detected', 1)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO media_items (media_type_id, title, status, is_favorite) VALUES (1, 'Plain', 'detected', 0)`)
	require.NoError(t, err)

	require.NoError(t, db.RunMigrations(context.Background()))

	var tbl, v30, users, media, legacy int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='user_favorites'`).Scan(&tbl))
	assert.Equal(t, 1, tbl)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 30`).Scan(&v30))
	assert.Equal(t, 1, v30)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&users))
	assert.Equal(t, 2, users)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM media_items`).Scan(&media))
	assert.Equal(t, 2, media)
	require.NoError(t, raw.QueryRow(`SELECT is_favorite FROM media_items WHERE title = 'Flagged'`).Scan(&legacy))
	assert.Equal(t, 1, legacy, "legacy flag untouched")
	var fav int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_favorites`).Scan(&fav))
	assert.Zero(t, fav, "legacy flags are not attributed to any user")
	_, err = raw.Exec(`INSERT INTO user_favorites (user_id, media_id) VALUES (2, 1)`)
	assert.NoError(t, err, "the new table accepts rows referencing the pre-existing users and media")
}
