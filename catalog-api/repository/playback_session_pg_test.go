package repository

// CAT-PG-SESSION-END-1: the playback session repository against a REAL PostgreSQL 15 (rootless podman via internal/testpg;
// SKIP, never faked, when podman or the image is missing). Found by the WPROG3 review: End bound an integer into the BOOLEAN
// completed column ("pq: column completed is of type boolean but expression is of type integer").

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/testpg"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPlaybackPG(t *testing.T) (*database.DB, int64) {
	t.Helper()
	dsn, _ := testpg.Start(t)
	raw, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	raw.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = raw.Close() })
	db := database.WrapDB(raw, database.DialectPostgres)
	ctx := context.Background()
	require.NoError(t, db.RunMigrations(ctx))
	_, err = db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'u1', 'u1@example.test', 'x', 'x', 2)`)
	require.NoError(t, err)
	var mid int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected') RETURNING id`).Scan(&mid))
	return db, mid
}

func TestPlaybackSessionPostgres_StartProgressEndGetHistory(t *testing.T) {
	db, mid := newPlaybackPG(t)
	repo := NewPlaybackSessionRepository(db)
	ctx := context.Background()
	dur := int64(7200)

	id1, err := repo.Start(ctx, PlaybackStart{UserID: 1, MediaItemID: mid, PositionUnit: "seconds", StartedAt: time.Now().UTC().Add(-time.Hour)})
	require.NoError(t, err, "Start on PostgreSQL")
	require.NoError(t, repo.Progress(ctx, PlaybackProgress{SessionID: id1, EndPosition: 30, TotalAmount: 30}))
	s, err := repo.Get(ctx, id1)
	require.NoError(t, err, "Get on PostgreSQL (BOOLEAN completed scan)")
	assert.False(t, s.Completed)

	require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id1, EndPosition: 100, TotalAmount: 100, DurationTotal: &dur, Completed: true}), "End completed=true on PostgreSQL")
	s, err = repo.Get(ctx, id1)
	require.NoError(t, err)
	assert.True(t, s.Completed)
	assert.Equal(t, int64(100), s.EndPosition)
	require.NotNil(t, s.EndedAt)

	id2, err := repo.Start(ctx, PlaybackStart{UserID: 1, MediaItemID: mid, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id2, EndPosition: 10, TotalAmount: 10, Completed: false}), "End completed=false on PostgreSQL")
	s, err = repo.Get(ctx, id2)
	require.NoError(t, err)
	assert.False(t, s.Completed)

	hist, err := repo.ListHistory(ctx, 1, mid, 10)
	require.NoError(t, err, "ListHistory on PostgreSQL")
	require.Len(t, hist, 2)
	byID := map[int64]bool{}
	for _, h := range hist {
		byID[h.ID] = h.Completed
	}
	assert.True(t, byID[id1])
	assert.False(t, byID[id2])

	p, err := repo.GetProgress(ctx, 1, mid)
	require.NoError(t, err)
	assert.Equal(t, int64(2), p.TotalReproductions)
	assert.Equal(t, int64(110), p.AggregateAmount)
	require.NotNil(t, p.DurationTotal)
	assert.Equal(t, dur, *p.DurationTotal)

	err = repo.End(ctx, PlaybackEnd{SessionID: 999999, Completed: true})
	assert.ErrorIs(t, err, ErrPlaybackSessionNotFound)
}

// Concurrent Ends of the SAME session on READ COMMITTED must count exactly one reproduction (compare-and-set on ended_at IS NULL),
// and concurrent Ends of DIFFERENT sessions of one user/item must not lose increments (atomic media_progress update).
func TestPlaybackSessionPostgres_ConcurrentEnds(t *testing.T) {
	db, mid := newPlaybackPG(t)
	repo := NewPlaybackSessionRepository(db)
	ctx := context.Background()

	// seed the progress row so the first-insert race is out of scope
	seed, err := repo.Start(ctx, PlaybackStart{UserID: 1, MediaItemID: mid, PositionUnit: "seconds"})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: seed, EndPosition: 1, TotalAmount: 1}))

	same, err := repo.Start(ctx, PlaybackStart{UserID: 1, MediaItemID: mid, PositionUnit: "seconds"})
	require.NoError(t, err)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: same, EndPosition: 10, TotalAmount: 10, Completed: true}))
		}()
	}
	wg.Wait()
	p, err := repo.GetProgress(ctx, 1, mid)
	require.NoError(t, err)
	assert.Equal(t, int64(2), p.TotalReproductions, "six racing Ends of one session count once")
	assert.Equal(t, int64(11), p.AggregateAmount)

	var ids []int64
	for i := 0; i < 6; i++ {
		id, err := repo.Start(ctx, PlaybackStart{UserID: 1, MediaItemID: mid, PositionUnit: "seconds"})
		require.NoError(t, err)
		ids = append(ids, id)
	}
	for _, id := range ids {
		id := id
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.NoError(t, repo.End(ctx, PlaybackEnd{SessionID: id, EndPosition: 5, TotalAmount: 5}))
		}()
	}
	wg.Wait()
	p, err = repo.GetProgress(ctx, 1, mid)
	require.NoError(t, err)
	assert.Equal(t, int64(8), p.TotalReproductions, "no lost increment across different sessions")
	assert.Equal(t, int64(41), p.AggregateAmount)
}
