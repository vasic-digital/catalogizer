package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"

	// go-sqlcipher registers itself as the "sqlite3" driver on
	// import. It's the driver the production connection code
	// already uses, so tests should too.
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/require"
)

// newPlaybackTestDB opens an in-memory SQLite database, wraps
// it with the project's dialect-aware wrapper, and creates the
// two playback tables directly via the migration DDL so the
// test does not need the full migration stack.
func newPlaybackTestDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	return db
}

func TestPlaybackSessionRepository_StartProgressEnd(t *testing.T) {
	db := newPlaybackTestDB(t)
	repo := NewPlaybackSessionRepository(db)

	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)

	// Start a session at 0s
	fileID := int64(7)
	sessID, err := repo.Start(ctx, PlaybackStart{
		UserID:        1,
		MediaItemID:   42,
		FileID:        &fileID,
		PositionUnit:  "seconds",
		StartPosition: 0,
		StartedAt:     now,
	})
	require.NoError(t, err)
	require.Greater(t, sessID, int64(0))

	// Bump progress to 30s
	require.NoError(t, repo.Progress(ctx, PlaybackProgress{
		SessionID:   sessID,
		EndPosition: 30,
		TotalAmount: 30,
	}))

	// Finalise the session at 120s
	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID:   sessID,
		EndPosition: 120,
		TotalAmount: 120,
		EndedAt:     now.Add(2 * time.Minute),
		Completed:   false,
	}))

	sess, err := repo.Get(ctx, sessID)
	require.NoError(t, err)
	require.Equal(t, int64(120), sess.EndPosition)
	require.Equal(t, int64(120), sess.TotalAmount)
	require.False(t, sess.Completed)
	require.NotNil(t, sess.EndedAt)
	require.Equal(t, "seconds", sess.PositionUnit)

	// media_progress reflects one reproduction
	prog, err := repo.GetProgress(ctx, 1, 42)
	require.NoError(t, err)
	require.Equal(t, int64(120), prog.LastPosition)
	require.Equal(t, int64(120), prog.LastSessionAmount)
	require.Equal(t, int64(1), prog.TotalReproductions)
	require.Equal(t, int64(120), prog.AggregateAmount)
	require.Equal(t, "seconds", prog.PositionUnit)

	// A second session accumulates total_reproductions and
	// aggregate_amount — demonstrates the ON CONFLICT DO UPDATE
	// path in PlaybackSessionRepository.End.
	secondID, err := repo.Start(ctx, PlaybackStart{
		UserID:      1,
		MediaItemID: 42,
		PositionUnit: "seconds",
		StartedAt:   now.Add(time.Hour),
	})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID:   secondID,
		EndPosition: 200,
		TotalAmount: 200,
		EndedAt:     now.Add(time.Hour + 3*time.Minute),
		Completed:   true,
	}))

	prog2, err := repo.GetProgress(ctx, 1, 42)
	require.NoError(t, err)
	require.Equal(t, int64(2), prog2.TotalReproductions)
	require.Equal(t, int64(320), prog2.AggregateAmount) // 120 + 200
	require.Equal(t, int64(200), prog2.LastPosition)

	// ListHistory returns both sessions, newest first.
	hist, err := repo.ListHistory(ctx, 1, 42, 10)
	require.NoError(t, err)
	require.Len(t, hist, 2)
	require.Equal(t, secondID, hist[0].ID)
	require.Equal(t, sessID, hist[1].ID)
}

func TestPlaybackSessionRepository_HandlesBookPages(t *testing.T) {
	db := newPlaybackTestDB(t)
	repo := NewPlaybackSessionRepository(db)
	ctx := context.Background()

	sessID, err := repo.Start(ctx, PlaybackStart{
		UserID:        1,
		MediaItemID:   8,
		PositionUnit:  "pages",
		StartPosition: 120,
	})
	require.NoError(t, err)

	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID:   sessID,
		EndPosition: 140,
		TotalAmount: 20,
		Completed:   false,
	}))

	prog, err := repo.GetProgress(ctx, 1, 8)
	require.NoError(t, err)
	require.Equal(t, "pages", prog.PositionUnit)
	require.Equal(t, int64(140), prog.LastPosition)
	require.Equal(t, int64(20), prog.LastSessionAmount)
}

func TestPlaybackSessionRepository_PersistsDurationTotal(t *testing.T) {
	db := newPlaybackTestDB(t)
	repo := NewPlaybackSessionRepository(db)
	ctx := context.Background()

	// First session ends without duration_total — field stays
	// null even though we accumulated position.
	sess1, err := repo.Start(ctx, PlaybackStart{
		UserID: 1, MediaItemID: 11, PositionUnit: "seconds",
	})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID: sess1, EndPosition: 600, TotalAmount: 600,
	}))
	p, err := repo.GetProgress(ctx, 1, 11)
	require.NoError(t, err)
	require.Nil(t, p.DurationTotal)

	// Second session passes the parsed total — the upsert
	// populates duration_total and keeps it on later writes.
	dur := int64(7200)
	sess2, err := repo.Start(ctx, PlaybackStart{
		UserID: 1, MediaItemID: 11, PositionUnit: "seconds",
	})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID:     sess2,
		EndPosition:   1200,
		TotalAmount:   600,
		DurationTotal: &dur,
	}))
	p, err = repo.GetProgress(ctx, 1, 11)
	require.NoError(t, err)
	require.NotNil(t, p.DurationTotal)
	require.Equal(t, int64(7200), *p.DurationTotal)

	// Third session without duration_total — previously learned
	// value is preserved so the card badge never flickers to
	// "unknown total" after one bad caller.
	sess3, err := repo.Start(ctx, PlaybackStart{
		UserID: 1, MediaItemID: 11, PositionUnit: "seconds",
	})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, PlaybackEnd{
		SessionID: sess3, EndPosition: 1800, TotalAmount: 600,
	}))
	p, err = repo.GetProgress(ctx, 1, 11)
	require.NoError(t, err)
	require.NotNil(t, p.DurationTotal)
	require.Equal(t, int64(7200), *p.DurationTotal)
	require.Equal(t, int64(3), p.TotalReproductions)
	require.Equal(t, int64(1800), p.AggregateAmount)
}

func TestPlaybackSessionRepository_ListHistoryEmpty(t *testing.T) {
	db := newPlaybackTestDB(t)
	repo := NewPlaybackSessionRepository(db)

	hist, err := repo.ListHistory(context.Background(), 99, 99, 10)
	require.NoError(t, err)
	require.Empty(t, hist)
	// the HTTP layer serialises this slice: nil would become JSON null where the spec documents an array
	require.NotNil(t, hist)
}

// Contract sweep finding: an unknown session id must be reported as
// ErrPlaybackSessionNotFound (HTTP 404), not a generic error (HTTP 500).
func TestPlaybackSessionRepository_UnknownSessionIsNotFound(t *testing.T) {
	repo := NewPlaybackSessionRepository(newPlaybackTestDB(t))
	ctx := context.Background()

	err := repo.End(ctx, PlaybackEnd{SessionID: 987654})
	require.ErrorIs(t, err, ErrPlaybackSessionNotFound)

	err = repo.Progress(ctx, PlaybackProgress{SessionID: 987654})
	require.ErrorIs(t, err, ErrPlaybackSessionNotFound)
}
