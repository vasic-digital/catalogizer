package database

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Root cause under test (measured 2026-10-30): the bundled go-sqlcipher driver
// compiles SQLite WITHOUT -DHAVE_USLEEP, so SQLite's default busy handler
// (sqliteDefaultBusyCallback, sqlite3.c "#else" branch) sleeps a WHOLE SECOND
// per retry. A _busy_timeout of N ms therefore buys only N/1000 retries, and a
// writer that loses a lock waits >= 1 s even when the lock is released after a
// few milliseconds. With 10 concurrent writers and the default 5000 ms timeout
// only about 5 win; the rest fail with "database is locked".
//
// These tests use plain sql.Open("sqlite3", ...) on purpose: the fix must hold
// for every connection opened through the "sqlite3" driver in this process,
// not only for connections made by NewConnection.

func openPlainFileSQLite(t *testing.T, busyMS int) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "busy.db")
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=%d", path, busyMS))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)")
	require.NoError(t, err)
	return db
}

// TestSQLiteBusyHandler_ShortLockWaitIsSubSecond: a writer blocked by a lock
// held for ~150 ms must proceed shortly after release, not after a 1 s sleep.
// Oracle: wall-clock latency of the blocked INSERT (independent of the code
// under test). RED (1 s granularity): latency >= ~1000 ms.
func TestSQLiteBusyHandler_ShortLockWaitIsSubSecond(t *testing.T) {
	db := openPlainFileSQLite(t, 5000)
	ctx := context.Background()

	holder, err := db.Conn(ctx)
	require.NoError(t, err)
	defer holder.Close()
	_, err = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
	require.NoError(t, err)
	_, err = holder.ExecContext(ctx, "INSERT INTO t (v) VALUES ('holder')")
	require.NoError(t, err)

	const hold = 150 * time.Millisecond
	released := make(chan struct{})
	go func() {
		time.Sleep(hold)
		_, cerr := holder.ExecContext(ctx, "COMMIT")
		assert.NoError(t, cerr)
		close(released)
	}()

	start := time.Now()
	_, err = db.ExecContext(ctx, "INSERT INTO t (v) VALUES ('waiter')")
	waited := time.Since(start)
	<-released
	require.NoError(t, err, "the blocked writer must succeed once the lock is released")
	assert.GreaterOrEqual(t, waited, hold-20*time.Millisecond,
		"control: the writer really was blocked by the held lock (waited %v)", waited)
	assert.Less(t, waited, 700*time.Millisecond,
		"blocked writer waited %v: the busy handler is retrying at 1 s granularity", waited)
}

// TestSQLiteBusyHandler_ConcurrentWritersAllSucceed: N simultaneous single-row
// INSERTs on separate pooled connections must all succeed within the
// configured busy timeout. RED (1 s granularity, 5000 ms timeout): ~N-5 fail.
func TestSQLiteBusyHandler_ConcurrentWritersAllSucceed(t *testing.T) {
	db := openPlainFileSQLite(t, 5000)
	_, err := db.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)

	const writers = 20
	var wg sync.WaitGroup
	errs := make([]error, writers)
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			_, errs[i] = db.Exec("INSERT INTO t (v) VALUES (?)", fmt.Sprint(i))
		}(i)
	}
	close(start)
	wg.Wait()

	for i, e := range errs {
		assert.NoErrorf(t, e, "writer %d", i)
	}
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM t").Scan(&n))
	assert.Equal(t, writers, n)
}

// TestSQLiteBusyHandler_TimeoutStillHonoured: the fix must not turn a real,
// long-held lock into an unbounded wait. With a 300 ms timeout and a lock held
// for 2 s, the writer must fail with "database is locked" after roughly the
// timeout. This is the golden-FALSE side: a handler that retries forever or
// ignores the DSN timeout fails here.
func TestSQLiteBusyHandler_TimeoutStillHonoured(t *testing.T) {
	db := openPlainFileSQLite(t, 300)
	ctx := context.Background()

	holder, err := db.Conn(ctx)
	require.NoError(t, err)
	defer holder.Close()
	_, err = holder.ExecContext(ctx, "BEGIN IMMEDIATE")
	require.NoError(t, err)
	defer func() { _, _ = holder.ExecContext(ctx, "ROLLBACK") }()

	start := time.Now()
	_, err = db.ExecContext(ctx, "INSERT INTO t (v) VALUES ('waiter')")
	waited := time.Since(start)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
	assert.GreaterOrEqual(t, waited, 250*time.Millisecond, "gave up too early: %v", waited)
	assert.Less(t, waited, 1500*time.Millisecond, "did not honour the 300 ms timeout: %v", waited)
}

// TestSQLiteBusyHandler_HookInstalled fails loudly if the hook could not be
// attached (for example after a driver upgrade changed SQLiteConn's layout).
func TestSQLiteBusyHandler_HookInstalled(t *testing.T) {
	require.NoError(t, sqliteBusyHandlerErr)
}
