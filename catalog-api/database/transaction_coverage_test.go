package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"catalogizer/config"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTxTestDB creates a small SQLite database with a single table that
// transaction tests can mutate without requiring migrations.
func setupTxTestDB(t *testing.T) *DB {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "tx_cov_*.db")
	require.NoError(t, err)
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })
	tmpFile.Close()

	cfg := &config.DatabaseConfig{
		Path:        tmpFile.Name(),
		EnableWAL:   true,
		BusyTimeout: 5000,
	}
	raw, err := NewConnection(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { raw.Close() })

	db := WrapDB(raw.DB, DialectSQLite)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS tx_cov (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		counter INTEGER NOT NULL DEFAULT 0
	)`)
	require.NoError(t, err)
	return db
}

func TestTxContext_Begin_Commit_RoundTrip(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	tx, err := tc.Begin(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, tx)
	require.NotEmpty(t, tx.txID)
	require.False(t, tx.IsTimeout())
	require.True(t, tx.Duration() >= 0)

	_, err = tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "a", 1)
	require.NoError(t, err)

	var cnt int
	row := tx.QueryRow("SELECT COUNT(*) FROM tx_cov")
	require.NoError(t, row.Scan(&cnt))
	require.Equal(t, 1, cnt)

	rows, err := tx.Query("SELECT name FROM tx_cov")
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
	}

	require.NoError(t, tx.Commit())

	// After commit, the row must be visible on the parent DB.
	err = db.QueryRow("SELECT COUNT(*) FROM tx_cov").Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 1, cnt)
}

func TestTxContext_Begin_Rollback_IsIdempotent(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	tx, err := tc.Begin(context.Background(), nil)
	require.NoError(t, err)

	_, err = tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "b", 2)
	require.NoError(t, err)

	require.NoError(t, tx.Rollback())
	// Second rollback is a no-op.
	require.NoError(t, tx.Rollback())

	// Commit after rollback must fail.
	err = tx.Commit()
	require.Error(t, err)
	require.Contains(t, err.Error(), "rolled back")

	// Row must not be visible.
	var cnt int
	err = db.QueryRow("SELECT COUNT(*) FROM tx_cov").Scan(&cnt)
	require.NoError(t, err)
	require.Equal(t, 0, cnt)
}

func TestTxContext_BeginWithRetry_SucceedsFirstTry(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	tx, err := tc.BeginWithRetry(context.Background(), nil)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
}

func TestTxContext_RunInTransaction_CommitPath(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	err := tc.RunInTransaction(context.Background(), func(tx *Transaction) error {
		_, err := tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "c", 3)
		return err
	})
	require.NoError(t, err)

	var cnt int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tx_cov").Scan(&cnt))
	require.Equal(t, 1, cnt)
}

func TestTxContext_RunInTransaction_RollbackPath(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	wantErr := errors.New("intentional failure")
	err := tc.RunInTransaction(context.Background(), func(tx *Transaction) error {
		if _, err := tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "d", 4); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	var cnt int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tx_cov").Scan(&cnt))
	require.Equal(t, 0, cnt, "fn error must trigger rollback")
}

func TestTxContext_RunInTransactionWithRetry_NonDeadlockErrorPassesThrough(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	wantErr := errors.New("non-retryable")
	err := tc.RunInTransactionWithRetry(context.Background(), func(tx *Transaction) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}

func TestTransaction_Duration_IncreasesOverTime(t *testing.T) {
	db := setupTxTestDB(t)
	tc := NewTxContext(db, DefaultTransactionConfig())

	tx, err := tc.Begin(context.Background(), nil)
	require.NoError(t, err)
	defer tx.Rollback()

	first := tx.Duration()
	time.Sleep(10 * time.Millisecond)
	second := tx.Duration()
	require.True(t, second > first, "duration should grow, got %v→%v", first, second)

	require.False(t, tx.IsLongRunning(1*time.Hour))
	require.True(t, tx.IsLongRunning(1*time.Nanosecond))
}

func TestTxDeadlockDetector_BoundedTxOrder(t *testing.T) {
	d := NewTxDeadlockDetector()
	d.maxTxOrder = 10
	for i := 0; i < 25; i++ {
		d.RecordStart(generateTxID())
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	require.LessOrEqual(t, len(d.txOrder), 10)
}

func TestSafeRollback_NilIsNoOp(t *testing.T) {
	// Must not panic on nil.
	SafeRollback(nil)
}

func TestSafeRollback_RealTx(t *testing.T) {
	db := setupTxTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	// Rollback directly is idempotent here too — SafeRollback swallows errors.
	SafeRollback(tx.SQLTx())
	// Second call on the same (already finished) tx — SafeRollback swallows
	// the "Tx.done" sentinel via its stderr log.
	SafeRollback(tx.SQLTx())
}

func TestSafeCommit_NilReturnsError(t *testing.T) {
	err := SafeCommit(nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "nil")
}

func TestSafeCommit_RealCommit(t *testing.T) {
	db := setupTxTestDB(t)
	tx, err := db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	_, err = tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "e", 5)
	require.NoError(t, err)
	require.NoError(t, SafeCommit(tx.SQLTx()))
}

func TestTxLockOrder_GetAndSort(t *testing.T) {
	lo := NewTxLockOrder([]string{"users", "sessions", "media_items"})
	assert.Equal(t, 0, lo.GetOrder("users"))
	assert.Equal(t, 1, lo.GetOrder("sessions"))
	assert.Equal(t, 2, lo.GetOrder("media_items"))

	// Adding a new table gives it the next index.
	idx := lo.GetOrder("new_table")
	assert.Equal(t, 3, idx)

	// Sort preserves order ascending by index.
	sorted := lo.SortTables([]string{"media_items", "users", "sessions"})
	assert.Equal(t, []string{"users", "sessions", "media_items"}, sorted)
}

func TestWithTransactionTimeout_Cancels(t *testing.T) {
	ctx, cancel := WithTransactionTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Poll with a generous deadline to tolerate scheduling jitter on loaded systems.
	require.Eventually(t, func() bool {
		return ctx.Err() != nil
	}, 500*time.Millisecond, 10*time.Millisecond, "expected context to be cancelled after timeout")
	require.Error(t, ctx.Err())
}

func TestWithQueryTimeout_Cancels(t *testing.T) {
	ctx, cancel := WithQueryTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Poll with a generous deadline to tolerate scheduling jitter on loaded systems.
	require.Eventually(t, func() bool {
		return ctx.Err() != nil
	}, 500*time.Millisecond, 10*time.Millisecond, "expected context to be cancelled after timeout")
	require.Error(t, ctx.Err())
}

func TestGenerateTxID_IsUnique(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 200; i++ {
		id := generateTxID()
		require.False(t, ids[id], "duplicate txID: %s", id)
		ids[id] = true
		require.True(t, strings.HasPrefix(id, "tx_"))
	}
}
