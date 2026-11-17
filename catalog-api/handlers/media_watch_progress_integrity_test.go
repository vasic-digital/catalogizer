package handlers

// WPROG round 2: media_progress row integrity across the two writers (PUT /media/:id/progress and the playback-session
// repository), the real desktop/api-client body shape, and per-user isolation. Host-fixture tests (real migrated SQLite,
// real handler and repository); not a live-device result.

import (
	"context"
	"database/sql"
	"net/http"
	"testing"

	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// F1 interleaving PUT -> session End: whatever PUT stored, the session End must not find a permille row to inherit.
func TestWatchProgress_PutThenSessionEnd_NoPermille(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	repo := repository.NewPlaybackSessionRepository(db)

	// fraction only, no duration, no row: nothing can be stored without inventing a unit
	code, resp := wprogPut(r, "1", `{"progress":0.42}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, resp["persisted"])
	assert.NotEmpty(t, resp["reason"])
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_progress`).Scan(&n))
	assert.Equal(t, 0, n, "no permille/guess row may be created")

	// with a known duration the row is seconds and the session End then keeps coherent seconds data
	code, resp = wprogPut(r, "1", `{"progress":0.42,"duration":1000}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, resp["persisted"])
	id, err := repo.Start(ctx, repository.PlaybackStart{UserID: 1, MediaItemID: 1, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, repository.PlaybackEnd{SessionID: id, EndPosition: 300, TotalAmount: 300}))
	p, err := repo.GetProgress(ctx, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "seconds", p.PositionUnit)
	assert.Equal(t, int64(300), p.LastPosition)
	require.NotNil(t, p.DurationTotal)
	assert.Equal(t, int64(1000), *p.DurationTotal)
}

// F1 interleaving session End -> PUT: a seconds row with unknown duration keeps its unit and resume point when a PUT
// arrives with only a fraction.
func TestWatchProgress_SessionEndThenPut_KeepsSecondsResumePoint(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	repo := repository.NewPlaybackSessionRepository(db)
	id, err := repo.Start(ctx, repository.PlaybackStart{UserID: 1, MediaItemID: 1, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, repository.PlaybackEnd{SessionID: id, EndPosition: 3000, TotalAmount: 3000}))

	for _, body := range []string{`{"progress":0.5}`, `{"progress":0.5,"position":3500}`} {
		code, resp := wprogPut(r, "1", body)
		require.Equal(t, http.StatusOK, code, body)
		assert.Equal(t, false, resp["persisted"], body)
		assert.NotEmpty(t, resp["reason"], body)
		p, err := repo.GetProgress(ctx, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "seconds", p.PositionUnit, body)
		assert.Equal(t, int64(3000), p.LastPosition, body)
		assert.Equal(t, int64(1), p.TotalReproductions, body)
	}

	// the desktop shape carries the duration: the NULL duration is learned, unit and the new position are seconds
	code, resp := wprogPut(r, "1", `{"media_id":1,"position":3500.4,"duration":7200.2,"timestamp":1700000000000}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, resp["persisted"])
	p, _ := repo.GetProgress(ctx, 1, 1)
	assert.Equal(t, "seconds", p.PositionUnit)
	assert.Equal(t, int64(3500), p.LastPosition)
	require.NotNil(t, p.DurationTotal)
	assert.Equal(t, int64(7200), *p.DurationTotal)
	assert.Equal(t, int64(1), p.TotalReproductions)
}

// F2 real client body shapes: desktop PlaybackProgress {media_id, position, duration, timestamp} with floats and no
// `progress`, and the api-client markAsWatched shape (position == duration).
func TestWatchProgress_RealClientShapes(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	repo := repository.NewPlaybackSessionRepository(db)

	code, resp := wprogPut(r, "1", `{"media_id":1,"position":12.5,"duration":7200.25,"timestamp":1700000000000}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	p, err := repo.GetProgress(ctx, 1, 1)
	require.NoError(t, err)
	assert.Equal(t, "seconds", p.PositionUnit)
	assert.Equal(t, int64(13), p.LastPosition) // 12.5 rounds half away from zero
	assert.Equal(t, int64(7200), *p.DurationTotal)

	code, resp = wprogPut(r, "1", `{"media_id":1,"position":100,"duration":100,"timestamp":1700000000000}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	p, _ = repo.GetProgress(ctx, 1, 1)
	assert.Equal(t, int64(100), p.LastPosition, "markAsWatched: position == duration")
	assert.Equal(t, int64(7200), *p.DurationTotal, "the stored duration is not replaced")

	// a float position next to a fraction binds too
	code, _ = wprogPut(r, "1", `{"progress":0.5,"position":12.5}`)
	assert.Equal(t, http.StatusOK, code)

	// position without duration and without progress is not enough
	code, _ = wprogPut(r, "1", `{"media_id":1,"position":12.5}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{"media_id":1,"duration":100}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{"position":10,"duration":-1}`)
	assert.Equal(t, http.StatusBadRequest, code)
	code, _ = wprogPut(r, "1", `{"position":"10","duration":100}`)
	assert.Equal(t, http.StatusBadRequest, code)
}

// F3: user B's value survives A's second write (kills an UPDATE that is not scoped to the user).
func TestWatchProgress_SecondWriteDoesNotTouchOtherUser(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	h := NewAndroidTVMediaHandler(db)
	a, b := wprogRouter(h, 1), wprogRouter(h, 2)
	repo := repository.NewPlaybackSessionRepository(db)
	code, _ := wprogPut(a, "1", `{"progress":0.25,"duration":1000}`)
	require.Equal(t, http.StatusOK, code)
	code, _ = wprogPut(b, "1", `{"progress":0.9,"duration":1000}`)
	require.Equal(t, http.StatusOK, code)
	code, _ = wprogPut(a, "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code)
	code, _ = wprogPut(a, "1", `{"progress":0.6}`)
	require.Equal(t, http.StatusOK, code)
	pa, _ := repo.GetProgress(ctx, 1, 1)
	pb, _ := repo.GetProgress(ctx, 2, 1)
	assert.Equal(t, int64(600), pa.LastPosition)
	assert.Equal(t, int64(900), pb.LastPosition, "B still holds 0.9")
	assert.Equal(t, int64(1000), *pb.DurationTotal)
}

// F4 (R3/R4): the first insert fails but the competing row appears (lost race) -> retried as update, persisted true and
// the data really is stored; an insert that keeps failing is a 500 and persisted:true is never claimed with no row.
func TestWatchProgress_FirstInsertFailureRetried(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	for _, s := range []string{
		`CREATE TABLE wprog_flag (x INTEGER)`,
		`CREATE TRIGGER wprog_race BEFORE INSERT ON media_progress WHEN (SELECT COUNT(*) FROM wprog_flag) = 0
		 BEGIN
		   INSERT INTO wprog_flag VALUES (1);
		   INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at)
		     VALUES (NEW.user_id, NEW.media_item_id, 'seconds', 7200, 100, CURRENT_TIMESTAMP);
		   SELECT RAISE(FAIL, 'simulated lost race');
		 END`,
	} {
		_, err := db.ExecContext(ctx, s)
		require.NoError(t, err)
	}
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	code, resp := wprogPut(r, "1", `{"progress":0.5,"duration":7200}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	p, err := repository.NewPlaybackSessionRepository(db).GetProgress(ctx, 1, 1)
	require.NoError(t, err, "a row must exist when persisted:true is returned")
	assert.Equal(t, int64(3600), p.LastPosition, "the retry applied the request as an update")
}

func TestWatchProgress_PersistentInsertFailureIs500(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `CREATE TRIGGER wprog_never BEFORE INSERT ON media_progress BEGIN SELECT RAISE(ABORT, 'always fails'); END`)
	require.NoError(t, err)
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	code, resp := wprogPut(r, "1", `{"progress":0.5,"duration":7200}`)
	assert.Equal(t, http.StatusInternalServerError, code, "%v", resp)
	assert.NotEqual(t, true, resp["persisted"])
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_progress`).Scan(&n))
	assert.Equal(t, 0, n)
}

// F4 (R4): a position with no known duration (no row, no body duration) is ignored, never stored as a number of an
// unknown unit.
func TestWatchProgress_PositionWithoutKnownDurationIgnored(t *testing.T) {
	db := wprogDB(t)
	ctx := context.Background()
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	code, resp := wprogPut(r, "1", `{"progress":0.5,"position":3012}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, resp["persisted"])
	var n int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM media_progress`).Scan(&n))
	assert.Equal(t, 0, n)
	_, err := repository.NewPlaybackSessionRepository(db).GetProgress(ctx, 1, 1)
	assert.ErrorIs(t, err, sql.ErrNoRows)
}
