package repository

// PGEND1-R-m1 / m4 follow-ups.
//
// m1: End's media_progress UPDATE is keyed by (user_id, media_item_id). A single-user/single-item test cannot see an UPDATE that
// drops one key column (mutant R6 dropped media_item_id and survived), so these tests keep THREE other progress rows
// (same user other item, other user same item, other user other item) and assert End leaves every one of them untouched.
// m4: End on a media_progress row that the PUT watch-progress handler created (only user_id, media_item_id, position_unit,
// duration_total, last_position, updated_at set; the rest defaulted) must update that row, not collide on the primary key.

import (
	"context"
	"testing"
	"time"

	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type progressRow struct {
	Reps, Agg, LastPos, LastAmt int64
}

func readProgress(t *testing.T, repo *PlaybackSessionRepository, u, m int64) progressRow {
	t.Helper()
	p, err := repo.GetProgress(context.Background(), u, m)
	require.NoError(t, err, "progress (%d,%d)", u, m)
	return progressRow{p.TotalReproductions, p.AggregateAmount, p.LastPosition, p.LastSessionAmount}
}

// endLeavesOtherProgressRowsUntouched runs the two-user / two-item matrix on any dialect. u1,u2 and i1,i2 must be valid keys.
func endLeavesOtherProgressRowsUntouched(t *testing.T, repo *PlaybackSessionRepository, u1, u2, i1, i2 int64) {
	ctx := context.Background()
	// give all four (user,item) pairs a distinct, non-zero progress row via one finished session each
	amounts := map[[2]int64]int64{{u1, i1}: 100, {u1, i2}: 200, {u2, i1}: 300, {u2, i2}: 400}
	for k, amt := range amounts {
		id, err := repo.Start(ctx, PlaybackStart{UserID: k[0], MediaItemID: k[1], PositionUnit: "seconds"})
		require.NoError(t, err)
		require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id, EndPosition: amt, TotalAmount: amt}))
	}
	before := map[[2]int64]progressRow{}
	for k := range amounts {
		before[k] = readProgress(t, repo, k[0], k[1])
		assert.Equal(t, progressRow{1, amounts[k], amounts[k], amounts[k]}, before[k], "seed row %v", k)
	}

	// one more session for (u1,i1) only
	id, err := repo.Start(ctx, PlaybackStart{UserID: u1, MediaItemID: i1, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id, EndPosition: 7, TotalAmount: 7}))

	assert.Equal(t, progressRow{2, 107, 7, 7}, readProgress(t, repo, u1, i1), "the ended (user,item) row advances")
	for k := range amounts {
		if k == [2]int64{u1, i1} {
			continue
		}
		assert.Equal(t, before[k], readProgress(t, repo, k[0], k[1]), "row %v must be untouched by an End of (u1,i1): the UPDATE is keyed by BOTH user_id and media_item_id", k)
	}
}

func TestPlaybackEnd_OtherUsersAndItemsProgressUntouched_SQLite(t *testing.T) {
	repo := NewPlaybackSessionRepository(newPlaybackTestDB(t))
	endLeavesOtherProgressRowsUntouched(t, repo, 1, 2, 42, 43)
}

func TestPlaybackEnd_OtherUsersAndItemsProgressUntouched_Postgres(t *testing.T) {
	db, i1 := newPlaybackPG(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (2, 'u2', 'u2@example.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	var i2 int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item2', 'detected') RETURNING id`).Scan(&i2))
	endLeavesOtherProgressRowsUntouched(t, NewPlaybackSessionRepository(db), 1, 2, i1, i2)
}

// putCreatedProgress inserts the row exactly as handlers/media_handler.go UpdateWatchProgress does for a new (user,item).
func putCreatedProgress(t *testing.T, db *database.DB, u, m, dur, pos int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at)
		 VALUES (?, ?, 'seconds', ?, ?, ?)`, u, m, dur, pos, time.Now().UTC())
	require.NoError(t, err)
}

func endOnPutCreatedRow(t *testing.T, db *database.DB, u, m int64) {
	ctx := context.Background()
	repo := NewPlaybackSessionRepository(db)
	putCreatedProgress(t, db, u, m, 3600, 120)

	id, err := repo.Start(ctx, PlaybackStart{UserID: u, MediaItemID: m, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id, EndPosition: 500, TotalAmount: 500}), "End on a PUT-created row must update it, not collide on the primary key")

	p, err := repo.GetProgress(ctx, u, m)
	require.NoError(t, err)
	assert.Equal(t, int64(1), p.TotalReproductions, "the PUT row had 0 reproductions; End counts the session once")
	assert.Equal(t, int64(500), p.AggregateAmount)
	assert.Equal(t, int64(500), p.LastPosition)
	require.NotNil(t, p.DurationTotal)
	assert.Equal(t, int64(3600), *p.DurationTotal, "a session without a duration keeps the one the PUT learned")
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_progress WHERE user_id = ? AND media_item_id = ?`, u, m).Scan(&n))
	assert.Equal(t, 1, n, "no second row")
}

func TestPlaybackEnd_OnPutCreatedRow_SQLite(t *testing.T) {
	endOnPutCreatedRow(t, newPlaybackTestDB(t), 1, 42)
}

func TestPlaybackEnd_OnPutCreatedRow_Postgres(t *testing.T) {
	db, mid := newPlaybackPG(t)
	endOnPutCreatedRow(t, db, 1, mid)
}
