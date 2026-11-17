package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-RATING-1: migration v31 creates user_media_ratings through the real RunMigrations chain; the 0..10 integer range is
// enforced by the schema as well as by the handler.
func TestMigrationV31_UserMediaRatings(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	var applied int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 31`).Scan(&applied))
	assert.Equal(t, 1, applied)

	_, err := raw.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'a', 'a@x.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'T', 'detected')`)
	require.NoError(t, err)

	_, err = raw.Exec(`INSERT INTO user_media_ratings (user_id, media_id, rating) VALUES (1, 1, 10)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO user_media_ratings (user_id, media_id, rating) VALUES (1, 1, 5)`)
	assert.Error(t, err, "(user_id, media_id) is the primary key")
	for _, bad := range []int{-1, 11} {
		_, err = raw.Exec(`INSERT INTO user_media_ratings (user_id, media_id, rating) VALUES (1, 2, ?)`, bad)
		assert.Error(t, err, "rating %d outside 0..10 rejected by CHECK", bad)
	}

	require.NoError(t, db.createUserMediaRatingsTable(ctx))
	require.NoError(t, db.RunMigrations(ctx))
	var rows int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings`).Scan(&rows))
	assert.Equal(t, 1, rows, "idempotent")
}

// Upgrade of a populated v30 database: users, media and a favorite survive v31; user_media_ratings is created empty.
func TestMigrationV31_UpgradeFromV30(t *testing.T) {
	db, raw := newMigratedDB(t)
	_, err := raw.Exec(`DROP TABLE user_media_ratings`)
	require.NoError(t, err)
	_, err = raw.Exec(`DELETE FROM migrations WHERE version = 31`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'a', 'a@x.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'T', 'detected')`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO user_favorites (user_id, media_id) VALUES (1, 1)`)
	require.NoError(t, err)

	require.NoError(t, db.RunMigrations(context.Background()))

	var n, fav, rat int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 31`).Scan(&n))
	assert.Equal(t, 1, n)
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_favorites`).Scan(&fav))
	assert.Equal(t, 1, fav, "v30 data survives")
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM user_media_ratings`).Scan(&rat))
	assert.Zero(t, rat)
	_, err = raw.Exec(`INSERT INTO user_media_ratings (user_id, media_id, rating) VALUES (1, 1, 4)`)
	assert.NoError(t, err)
}
