// Package pooltest is the shared helper for the pool-deadlock class tests
// (see internal/poolguard): a real SQLite file with the production migrations
// behind a deliberately small connection pool, and a runner that launches more
// concurrent callers than the pool has connections and reports a stall as a pool
// deadlock instead of hanging the test run.
package pooltest

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"catalogizer/database"

	"github.com/stretchr/testify/require"
)

// PoolSize is the connection-pool size used by the class tests. The production
// default is 25; the mechanism is identical for any pool smaller than the number
// of concurrent callers.
const PoolSize = 3

// NewPooledDB opens a real SQLite database file (WAL, busy timeout) with at most
// PoolSize connections and applies the production migrations.
func NewPooledDB(t *testing.T) (*database.DB, *sql.DB) {
	t.Helper()
	db, sqlDB := NewEmptyPooledDB(t)
	require.NoError(t, db.RunMigrations(context.Background()))
	return db, sqlDB
}

// NewEmptyPooledDB is NewPooledDB without migrations, for code whose schema is not
// part of the production migrations (the caller creates it).
func NewEmptyPooledDB(t *testing.T) (*database.DB, *sql.DB) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pool.db")+"?_busy_timeout=5000&_journal_mode=WAL")
	require.NoError(t, err)
	// Closing the pool also releases callers still blocked waiting for a connection,
	// so a deadlock does not outlive the test.
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(PoolSize)
	return database.WrapDB(sqlDB, database.DialectSQLite), sqlDB
}

// RunConcurrent runs fn from n goroutines at once (n must exceed PoolSize for the
// test to mean anything) and fails the test when a caller returns an error or when
// the callers are still blocked after timeout. The deadlock signature is the
// message "pool deadlock: K of N callers still blocked".
func RunConcurrent(t *testing.T, n int, timeout time.Duration, fn func(i int) error) {
	t.Helper()
	require.Greater(t, n, PoolSize, "need more concurrent callers than pool connections")
	var wg sync.WaitGroup
	errs := make([]error, n)
	finished := make([]bool, n)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := fn(i)
			mu.Lock()
			errs[i], finished[i] = e, true
			mu.Unlock()
		}(i)
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(timeout):
		mu.Lock()
		blocked := 0
		for _, f := range finished {
			if !f {
				blocked++
			}
		}
		mu.Unlock()
		t.Fatalf("pool deadlock: %d of %d callers still blocked after %v (pool size %d)", blocked, n, timeout, PoolSize)
	}
	for i, e := range errs {
		require.NoError(t, e, fmt.Sprintf("caller %d", i))
	}
}
