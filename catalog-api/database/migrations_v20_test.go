// Package database — migration-v20 regression tests.
//
// FIX-CONCURRENCY-2026-06-29: migration v20
// (external_metadata_unique_index) adds a UNIQUE index on
// external_metadata(media_item_id, provider) so concurrent enrichment can no
// longer create duplicate provider rows for one media item. The migration is
// self-healing: it deletes pre-existing duplicates (keeping MAX(id) per group)
// before creating the index, and is idempotent across repeated runs.
//
// These property tests assert all three obligations:
//   1. the unique index exists after the full migration chain;
//   2. it rejects a duplicate (media_item_id, provider) at the DB layer;
//   3. it self-heals an environment that already has duplicate rows, and
//      re-running it is a no-op.
package database

import (
	"context"
	"strings"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMigrationV20_UniqueIndexExists asserts the unique index is present after
// the full migration chain and is genuinely UNIQUE.
func TestMigrationV20_UniqueIndexExists(t *testing.T) {
	db, _ := newMigratedDB(t)
	ctx := context.Background()

	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_external_metadata_item_provider'").Scan(&count))
	assert.Equal(t, 1, count, "unique index idx_external_metadata_item_provider must exist after migrations")

	var indexSQL string
	require.NoError(t, db.QueryRowContext(ctx,
		"SELECT sql FROM sqlite_master WHERE type='index' AND name='idx_external_metadata_item_provider'").Scan(&indexSQL))
	assert.Contains(t, strings.ToUpper(indexSQL), "UNIQUE",
		"the index must be declared UNIQUE")
	assert.Contains(t, strings.ToLower(indexSQL), "media_item_id")
	assert.Contains(t, strings.ToLower(indexSQL), "provider")
}

// TestMigrationV20_RejectsDuplicatePair proves the index rejects a duplicate
// (media_item_id, provider) at the DB layer after migrations.
func TestMigrationV20_RejectsDuplicatePair(t *testing.T) {
	_, raw := newMigratedDB(t)

	_, err := raw.Exec(`INSERT INTO external_metadata (media_item_id, provider, external_id, data) VALUES (1, 'tmdb', 'a', '{}')`)
	require.NoError(t, err)

	_, err = raw.Exec(`INSERT INTO external_metadata (media_item_id, provider, external_id, data) VALUES (1, 'tmdb', 'b', '{}')`)
	require.Error(t, err, "a duplicate (media_item_id, provider) must be rejected by the unique index")
	assert.Contains(t, strings.ToLower(err.Error()), "unique constraint failed")
}

// TestMigrationV20_DedupSelfHealsPreExistingDuplicates simulates a pre-fix
// environment (index dropped, duplicate rows inserted) and re-runs v20,
// asserting it deletes the older duplicates (keeping MAX(id)) and restores the
// index so further duplicates are rejected.
func TestMigrationV20_DedupSelfHealsPreExistingDuplicates(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	// Recreate the production defect: drop the unique index, then insert three
	// rows that all share (media_item_id, provider) = (7, 'tmdb').
	_, err := raw.Exec(`DROP INDEX IF EXISTS idx_external_metadata_item_provider`)
	require.NoError(t, err)
	for _, ext := range []string{"a", "b", "c"} {
		_, err := raw.Exec(
			`INSERT INTO external_metadata (media_item_id, provider, external_id, data) VALUES (7, 'tmdb', ?, '{}')`, ext)
		require.NoError(t, err)
	}

	var before int
	require.NoError(t, raw.QueryRow(
		`SELECT COUNT(*) FROM external_metadata WHERE media_item_id=7 AND provider='tmdb'`).Scan(&before))
	require.Equal(t, 3, before, "precondition: three duplicate rows exist before the migration re-runs")

	// Force v20 to re-execute.
	_, err = raw.Exec(`DELETE FROM migrations WHERE version = 20`)
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(ctx))

	var after int
	require.NoError(t, raw.QueryRow(
		`SELECT COUNT(*) FROM external_metadata WHERE media_item_id=7 AND provider='tmdb'`).Scan(&after))
	assert.Equal(t, 1, after, "v20 must dedup pre-existing duplicate (media_item_id, provider) rows to one")

	var survivor string
	require.NoError(t, raw.QueryRow(
		`SELECT external_id FROM external_metadata WHERE media_item_id=7 AND provider='tmdb'`).Scan(&survivor))
	assert.Equal(t, "c", survivor, "dedup keeps the highest-id survivor (most recently enriched row)")

	// Index restored — a further duplicate is now rejected.
	_, err = raw.Exec(`INSERT INTO external_metadata (media_item_id, provider, external_id, data) VALUES (7, 'tmdb', 'd', '{}')`)
	require.Error(t, err, "after self-heal the restored unique index must reject new duplicates")
	assert.Contains(t, strings.ToLower(err.Error()), "unique constraint failed")
}

// TestMigrationV20_IsIdempotent_ManyRuns re-runs v20 ten times on a clean DB
// and asserts the index exists exactly once after every run (no
// "index already exists" / dedup-deletes-good-rows misfire).
func TestMigrationV20_IsIdempotent_ManyRuns(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	// Seed one legitimate row that must survive every dedup pass.
	_, err := raw.Exec(`INSERT INTO external_metadata (media_item_id, provider, external_id, data) VALUES (3, 'imdb', 'tt1', '{}')`)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		_, err := raw.Exec("DELETE FROM migrations WHERE version = 20")
		require.NoErrorf(t, err, "iteration %d: delete migrations row", i)
		require.NoErrorf(t, db.RunMigrations(ctx), "iteration %d: RunMigrations", i)

		var idx int
		require.NoError(t, db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_external_metadata_item_provider'").Scan(&idx))
		assert.Equalf(t, 1, idx, "iteration %d: index must exist exactly once", i)

		var rows int
		require.NoError(t, raw.QueryRow(
			`SELECT COUNT(*) FROM external_metadata WHERE media_item_id=3 AND provider='imdb'`).Scan(&rows))
		assert.Equalf(t, 1, rows, "iteration %d: the legitimate row must survive every dedup pass", i)
	}
}
