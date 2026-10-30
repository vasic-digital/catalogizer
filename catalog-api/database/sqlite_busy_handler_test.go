//go:build cgo && !windows

package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
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

// warmConn returns a dedicated, already-opened pooled connection, so a timed
// statement on it measures lock waiting only, never connection setup.
func warmConn(t *testing.T, db *sql.DB) *sql.Conn {
	t.Helper()
	c, err := db.Conn(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	var n int
	require.NoError(t, c.QueryRowContext(context.Background(), "SELECT COUNT(*) FROM t").Scan(&n))
	return c
}

// holdWriteLock takes the write lock on its own warmed connection.
func holdWriteLock(t *testing.T, db *sql.DB) *sql.Conn {
	t.Helper()
	ctx := context.Background()
	holder := warmConn(t, db)
	_, err := holder.ExecContext(ctx, "BEGIN IMMEDIATE")
	require.NoError(t, err)
	_, err = holder.ExecContext(ctx, "INSERT INTO t (v) VALUES ('holder')")
	require.NoError(t, err)
	return holder
}

// execWithin runs one statement on conn and fails the test fast if it has not
// returned within limit plus a grace period, so a busy handler that retries
// forever fails here instead of hanging the package until the go test
// timeout. The C busy loop does not observe the context, hence the watchdog.
func execWithin(t *testing.T, conn *sql.Conn, limit time.Duration, query string) (time.Time, error) {
	t.Helper()
	type res struct {
		end time.Time
		err error
	}
	ctx, cancel := context.WithTimeout(context.Background(), limit)
	defer cancel()
	ch := make(chan res, 1)
	go func() {
		_, err := conn.ExecContext(ctx, query)
		ch <- res{time.Now(), err}
	}()
	select {
	case r := <-ch:
		return r.end, r.err
	case <-time.After(limit + 500*time.Millisecond):
		t.Fatalf("statement %q did not return within %v: the busy handler ignores its timeout", query, limit)
		return time.Time{}, nil
	}
}

// commitAfter commits holder after d and records when COMMIT returned.
func commitAfter(holder *sql.Conn, d time.Duration) (*atomic.Int64, <-chan error) {
	var done atomic.Int64
	released := make(chan error, 1)
	go func() {
		time.Sleep(d)
		_, err := holder.ExecContext(context.Background(), "COMMIT")
		done.Store(time.Now().UnixNano())
		released <- err
	}()
	return &done, released
}

// TestSQLiteBusyHandler_ShortLockWaitIsSubSecond: a writer blocked by a short
// held lock must proceed soon after the lock is released.
//
// Oracle: time from the holder's COMMIT returning to the waiter's statement
// returning. Both connections are warmed first, so neither connection setup
// nor the COMMIT's own fsync falls inside the measured window. With the stock
// 1 s handler the waiter sleeps a full second from its first BUSY, so this
// value is about 1000 ms - hold (~850 ms); with the replacement handler the
// back-off step at that point is at most 50 ms. The 500 ms threshold leaves
// hundreds of milliseconds of margin either way.
func TestSQLiteBusyHandler_ShortLockWaitIsSubSecond(t *testing.T) {
	db := openPlainFileSQLite(t, 5000)
	waiter := warmConn(t, db)
	holder := holdWriteLock(t, db)

	const hold = 150 * time.Millisecond
	commitDone, released := commitAfter(holder, hold)

	start := time.Now()
	end, err := execWithin(t, waiter, 4*time.Second, "INSERT INTO t (v) VALUES ('waiter')")
	require.NoError(t, <-released)
	require.NoError(t, err, "the blocked writer must succeed once the lock is released")

	waited := end.Sub(start)
	afterRelease := end.Sub(time.Unix(0, commitDone.Load()))
	t.Logf("total wait %v, after release %v", waited, afterRelease)
	assert.GreaterOrEqual(t, waited, hold-20*time.Millisecond,
		"control: the writer really was blocked by the held lock (waited %v)", waited)
	assert.Less(t, afterRelease, 500*time.Millisecond,
		"waiter finished %v after the lock was released (total wait %v): busy handler retries at 1 s granularity",
		afterRelease, waited)
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
// long-held lock into an unbounded wait. With a 300 ms timeout and the lock
// held throughout, the writer must fail with "database is locked" after
// roughly the timeout. A handler that retries forever trips execWithin's 3 s
// watchdog instead of the go test timeout.
func TestSQLiteBusyHandler_TimeoutStillHonoured(t *testing.T) {
	db := openPlainFileSQLite(t, 300)
	waiter := warmConn(t, db)
	holder := holdWriteLock(t, db)
	t.Cleanup(func() { _, _ = holder.ExecContext(context.Background(), "ROLLBACK") })

	start := time.Now()
	end, err := execWithin(t, waiter, 3*time.Second, "INSERT INTO t (v) VALUES ('waiter')")
	waited := end.Sub(start)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
	assert.GreaterOrEqual(t, waited, 250*time.Millisecond, "gave up too early: %v", waited)
	assert.Less(t, waited, 1500*time.Millisecond, "did not honour the 300 ms timeout: %v", waited)
}

// TestSQLiteBusyHandler_SmallTimeoutAccounting pins the accumulated-delay
// accounting of the replacement handler: with a 50 ms timeout and the lock
// held throughout, the delays 1+2+5+10+15+17 (the last one trimmed) add up to
// the timeout, so the writer gives up after ~50 ms. A handler that forgets the
// time already slept keeps retrying well past the timeout, which the 200 ms
// ceiling catches.
func TestSQLiteBusyHandler_SmallTimeoutAccounting(t *testing.T) {
	db := openPlainFileSQLite(t, 50)
	waiter := warmConn(t, db)
	holder := holdWriteLock(t, db)
	t.Cleanup(func() { _, _ = holder.ExecContext(context.Background(), "ROLLBACK") })

	start := time.Now()
	end, err := execWithin(t, waiter, 3*time.Second, "INSERT INTO t (v) VALUES ('waiter')")
	waited := end.Sub(start)
	t.Logf("waited %v", waited)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "locked")
	assert.GreaterOrEqual(t, waited, 45*time.Millisecond, "gave up before the 50 ms timeout: %v", waited)
	assert.LessOrEqual(t, waited, 200*time.Millisecond, "did not honour the 50 ms timeout: %v", waited)
}

// TestSQLiteBusyHandler_RuntimePragmaRestoresStockHandler pins the documented
// limits of the hook (see sqliteBusyHandlerErr): after the hook ran, PRAGMA
// busy_timeout reads 0, and a runtime "PRAGMA busy_timeout=N" on a connection
// brings back SQLite's stock 1 s handler on that connection. A change that
// re-installs the handler on PRAGMA must update this test with it.
func TestSQLiteBusyHandler_RuntimePragmaRestoresStockHandler(t *testing.T) {
	db := openPlainFileSQLite(t, 5000)
	ctx := context.Background()
	waiter := warmConn(t, db)

	var bt int
	require.NoError(t, waiter.QueryRowContext(ctx, "PRAGMA busy_timeout").Scan(&bt))
	assert.Equal(t, 0, bt, "sqlite3_busy_handler resets busyTimeout, so PRAGMA busy_timeout reads 0 after the hook")

	_, err := waiter.ExecContext(ctx, "PRAGMA busy_timeout=3000")
	require.NoError(t, err)

	holder := holdWriteLock(t, db)
	commitDone, released := commitAfter(holder, 150*time.Millisecond)
	end, err := execWithin(t, waiter, 4*time.Second, "INSERT INTO t (v) VALUES ('waiter')")
	require.NoError(t, <-released)
	require.NoError(t, err)
	afterRelease := end.Sub(time.Unix(0, commitDone.Load()))
	assert.Greater(t, afterRelease, 500*time.Millisecond,
		"documented limit: a runtime PRAGMA busy_timeout restores the stock 1 s handler (waited %v after release)",
		afterRelease)
}

// TestSQLiteBusyHandler_HookInstalled fails loudly if the hook could not be
// attached (for example after a driver upgrade changed SQLiteConn's layout).
func TestSQLiteBusyHandler_HookInstalled(t *testing.T) {
	require.NoError(t, sqliteBusyHandlerErr)
	db, err := sql.Open("sqlite3", "")
	require.NoError(t, err)
	drv, ok := db.Driver().(*sqlite3.SQLiteDriver)
	_ = db.Close()
	require.True(t, ok)
	require.NotNil(t, drv.ConnectHook, "the shared sqlite3 driver must carry the busy-handler ConnectHook")
}

// Fake layouts for TestSQLiteBusyHandler_LayoutCheck (golden-bad inputs).
type fakeConnNoDB struct{ handle *int }
type fakeConnDBNotFirst struct {
	pad uint64
	db  *int
}
type fakeConnDBNotPtr struct{ db uintptr }
type fakeConnDBWrongElem struct{ db *int }

// TestSQLiteBusyHandler_LayoutCheck: the layout check accepts the pinned
// driver type and rejects every other shape, including a pointer to the wrong
// element type.
func TestSQLiteBusyHandler_LayoutCheck(t *testing.T) {
	require.NoError(t, checkSQLiteConnLayout(reflect.TypeOf(sqlite3.SQLiteConn{})))
	for name, typ := range map[string]reflect.Type{
		"no db field":      reflect.TypeOf(fakeConnNoDB{}),
		"db not first":     reflect.TypeOf(fakeConnDBNotFirst{}),
		"db not a pointer": reflect.TypeOf(fakeConnDBNotPtr{}),
		"db wrong elem":    reflect.TypeOf(fakeConnDBWrongElem{}),
		"not a struct":     reflect.TypeOf(0),
		"nil type":         nil,
	} {
		assert.Errorf(t, checkSQLiteConnLayout(typ), "layout %q must be rejected", name)
	}
}

// TestSQLiteBusyHandler_ConnectHookChainsPrevious: a ConnectHook registered
// before ours must still run after ours, and its error must be returned.
func TestSQLiteBusyHandler_ConnectHookChainsPrevious(t *testing.T) {
	var order []string
	hook := newBusyHandlerConnectHook(
		func(*sqlite3.SQLiteConn) error { order = append(order, "install"); return nil },
		func(*sqlite3.SQLiteConn) error { order = append(order, "prev"); return nil },
	)
	require.NoError(t, hook(&sqlite3.SQLiteConn{}))
	assert.Equal(t, []string{"install", "prev"}, order)

	prevErr := errors.New("prev failed")
	hook = newBusyHandlerConnectHook(
		func(*sqlite3.SQLiteConn) error { return nil },
		func(*sqlite3.SQLiteConn) error { return prevErr },
	)
	assert.ErrorIs(t, hook(&sqlite3.SQLiteConn{}), prevErr, "the previous hook's error must be returned")
}

var registerFailingDriver sync.Once

// TestSQLiteBusyHandler_InstallErrorFailsOpen: if installing the handler
// fails, opening the connection must fail (no silent fallback to the 1 s
// handler) and the previous hook must not run. Checked on the hook function
// and through a real driver open.
func TestSQLiteBusyHandler_InstallErrorFailsOpen(t *testing.T) {
	installErr := errors.New("busy handler install failed")
	prevRan := false
	hook := newBusyHandlerConnectHook(
		func(*sqlite3.SQLiteConn) error { return installErr },
		func(*sqlite3.SQLiteConn) error { prevRan = true; return nil },
	)
	assert.ErrorIs(t, hook(&sqlite3.SQLiteConn{}), installErr)
	assert.False(t, prevRan, "the previous hook must not run after a failed install")

	registerFailingDriver.Do(func() {
		sql.Register("sqlite3_busyhandler_failing_test", &sqlite3.SQLiteDriver{
			ConnectHook: newBusyHandlerConnectHook(
				func(*sqlite3.SQLiteConn) error { return installErr }, nil),
		})
	})
	db, err := sql.Open("sqlite3_busyhandler_failing_test", filepath.Join(t.TempDir(), "f.db"))
	require.NoError(t, err)
	defer db.Close()
	err = db.Ping()
	require.Error(t, err, "a connection whose busy handler could not be installed must not be handed out")
	assert.Contains(t, err.Error(), "busy handler install failed")
}
