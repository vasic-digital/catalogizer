package stress

import (
	"database/sql"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/tests"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// STRESS TEST: Cache operations (concurrent set/get, data consistency)
// =============================================================================

func TestCacheStress_ConcurrentSetGet(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	db := tests.SetupTestDB(t)
	defer db.Close()

	t.Run("100ConcurrentCacheOperations", func(t *testing.T) {
		concurrency := 100
		opsPerWorker := 20
		var writeSuccess, readSuccess, writeError, readError int64

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < opsPerWorker; j++ {
					key := fmt.Sprintf("cache_key_%d_%d",
						workerID, j)
					value := fmt.Sprintf(
						`{"worker":%d,"op":%d}`,
						workerID, j)

					// Write
					_, err := db.Exec(
						`INSERT OR REPLACE INTO cache_entries
						 (cache_key, value, expires_at)
						 VALUES (?, ?, datetime('now', '+1 hour'))`,
						key, value,
					)
					if err != nil {
						atomic.AddInt64(&writeError, 1)
						continue
					}
					atomic.AddInt64(&writeSuccess, 1)

					// Read back
					var stored string
					err = db.QueryRow(
						`SELECT value FROM cache_entries
						 WHERE cache_key = ?`, key,
					).Scan(&stored)
					if err != nil {
						atomic.AddInt64(&readError, 1)
						continue
					}
					if stored == value {
						atomic.AddInt64(&readSuccess, 1)
					} else {
						atomic.AddInt64(&readError, 1)
					}
				}
			}(i)
		}

		wg.Wait()

		ws := atomic.LoadInt64(&writeSuccess)
		rs := atomic.LoadInt64(&readSuccess)
		we := atomic.LoadInt64(&writeError)
		re := atomic.LoadInt64(&readError)
		total := int64(concurrency * opsPerWorker)

		t.Logf("Cache stress results:")
		t.Logf("  Write success: %d, Write errors: %d", ws, we)
		t.Logf("  Read success:  %d, Read errors:  %d", rs, re)
		t.Logf("  Total ops:     %d", total)

		writeRate := float64(ws) / float64(total) * 100
		assert.Greater(t, writeRate, 95.0,
			"Write success rate should be >95%%")

		readRate := float64(rs) / float64(ws) * 100
		assert.Greater(t, readRate, 95.0,
			"Read-back consistency should be >95%%")
	})
}

func TestCacheStress_DataConsistency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	db := tests.SetupTestDB(t)
	defer db.Close()

	t.Run("NoLostWrites", func(t *testing.T) {
		// Each worker writes unique keys; verify none are missing
		workers := 20
		keysPerWorker := 50
		expectedKeys := workers * keysPerWorker

		var wg sync.WaitGroup
		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for k := 0; k < keysPerWorker; k++ {
					key := fmt.Sprintf("consist_%d_%d",
						workerID, k)
					value := fmt.Sprintf("val_%d_%d",
						workerID, k)
					db.Exec(
						`INSERT OR REPLACE INTO cache_entries
						 (cache_key, value, expires_at)
						 VALUES (?, ?, datetime('now', '+1 hour'))`,
						key, value,
					)
				}
			}(w)
		}

		wg.Wait()

		// Count all inserted keys
		var count int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM cache_entries
			 WHERE cache_key LIKE 'consist_%'`,
		).Scan(&count)
		require.NoError(t, err)

		assert.Equal(t, expectedKeys, count,
			"No cache writes should be lost; "+
				"expected %d, got %d", expectedKeys, count)
	})

	t.Run("OverwriteConsistency", func(t *testing.T) {
		// Multiple workers overwrite the same key; final value
		// should be valid (not corrupted)
		sharedKey := "shared_overwrite_key"
		writers := 50
		writesPerWriter := 20

		var wg sync.WaitGroup
		for w := 0; w < writers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				for j := 0; j < writesPerWriter; j++ {
					value := fmt.Sprintf(
						"worker_%d_write_%d", workerID, j)
					db.Exec(
						`INSERT OR REPLACE INTO cache_entries
						 (cache_key, value, expires_at)
						 VALUES (?, ?, datetime('now', '+1 hour'))`,
						sharedKey, value,
					)
				}
			}(w)
		}

		wg.Wait()

		// Read the final value - it should be one of the written
		// values (not corrupted)
		var finalValue string
		err := db.QueryRow(
			`SELECT value FROM cache_entries
			 WHERE cache_key = ?`, sharedKey,
		).Scan(&finalValue)
		require.NoError(t, err)
		assert.Contains(t, finalValue, "worker_",
			"Final value should be a valid written value")
		assert.Contains(t, finalValue, "_write_",
			"Final value should have expected format")
	})
}

func TestCacheStress_ExpirationCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	db := tests.SetupTestDB(t)
	defer db.Close()

	t.Run("CleanupExpiredEntries", func(t *testing.T) {
		// Insert entries with past expiration
		expiredCount := 100
		validCount := 50

		for i := 0; i < expiredCount; i++ {
			_, err := db.Exec(
				`INSERT INTO cache_entries
				 (cache_key, value, expires_at)
				 VALUES (?, 'expired_val',
				  datetime('now', '-1 hour'))`,
				fmt.Sprintf("expired_%d", i),
			)
			require.NoError(t, err)
		}

		for i := 0; i < validCount; i++ {
			_, err := db.Exec(
				`INSERT INTO cache_entries
				 (cache_key, value, expires_at)
				 VALUES (?, 'valid_val',
				  datetime('now', '+1 hour'))`,
				fmt.Sprintf("valid_%d", i),
			)
			require.NoError(t, err)
		}

		// Run cleanup (simulates CacheService cleanup goroutine)
		result, err := db.Exec(
			`DELETE FROM cache_entries
			 WHERE expires_at < datetime('now')`,
		)
		require.NoError(t, err)

		deleted, _ := result.RowsAffected()
		assert.Equal(t, int64(expiredCount), deleted,
			"All expired entries should be cleaned up")

		// Verify valid entries remain
		var remaining int
		err = db.QueryRow(
			`SELECT COUNT(*) FROM cache_entries
			 WHERE cache_key LIKE 'valid_%'`,
		).Scan(&remaining)
		require.NoError(t, err)
		assert.Equal(t, validCount, remaining,
			"Valid entries should not be deleted")
	})

	t.Run("ConcurrentCleanupNoDeadlock", func(t *testing.T) {
		// Use a file-based SQLite with WAL mode so multiple connections can
		// operate concurrently without starving on MaxOpenConns(1).
		tmpDir := t.TempDir()
		dbPath := tmpDir + "/deadlock_test.db"
		fileDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&cache=shared")
		require.NoError(t, err)
		t.Cleanup(func() { fileDB.Close() })

		_, _ = fileDB.Exec("PRAGMA journal_mode=WAL")
		_, _ = fileDB.Exec("PRAGMA foreign_keys = ON")
		_, err = fileDB.Exec(`CREATE TABLE IF NOT EXISTS cache_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cache_key TEXT NOT NULL UNIQUE,
			value TEXT NOT NULL,
			expires_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`)
		require.NoError(t, err)

		fileDB.SetMaxOpenConns(10)
		fileDB.SetMaxIdleConns(5)

		// Insert test data
		for i := 0; i < 200; i++ {
			fileDB.Exec(
				`INSERT OR REPLACE INTO cache_entries
				 (cache_key, value, expires_at)
				 VALUES (?, 'test_val',
				  datetime('now', '-1 minute'))`,
				fmt.Sprintf("deadlock_test_%d", i),
			)
		}

		// Run cleanup concurrently with reads and writes
		done := make(chan struct{})
		var wg sync.WaitGroup

		// Cleanup goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					fileDB.Exec(
						`DELETE FROM cache_entries
						 WHERE expires_at < datetime('now')
						 AND cache_key LIKE 'deadlock_test_%'`)
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()

		// Writer goroutines
		for w := 0; w < 5; w++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						fileDB.Exec(
							`INSERT OR REPLACE INTO cache_entries
							 (cache_key, value, expires_at)
							 VALUES (?, 'new_val',
							  datetime('now', '+1 hour'))`,
							fmt.Sprintf("concurrent_w%d_%d",
								id, time.Now().UnixNano()),
						)
						time.Sleep(5 * time.Millisecond)
					}
				}
			}(w)
		}

		// Reader goroutines
		for r := 0; r < 5; r++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-done:
						return
					default:
						var count int
						fileDB.QueryRow(
							`SELECT COUNT(*) FROM cache_entries`).Scan(&count)
						time.Sleep(5 * time.Millisecond)
					}
				}
			}()
		}

		// Let it run for 1 second - if no deadlock, test passes
		time.Sleep(1 * time.Second)
		close(done)

		// Wait with a hard deadline to prevent hanging
		wgDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(wgDone)
		}()
		select {
		case <-wgDone:
		case <-time.After(10 * time.Second):
			t.Fatal("ConcurrentCleanupNoDeadlock: goroutines did not finish within deadline")
		}

		// If we got here without hanging, no deadlock occurred
		t.Log("Concurrent cleanup completed without deadlock")
	})
}

func TestCacheStress_HighThroughput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	// Use file-based SQLite with WAL mode for concurrent write throughput testing.
	// In-memory SQLite with MaxOpenConns(1) serializes all operations through a single
	// connection, which causes goroutines to block indefinitely waiting for the connection.
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/throughput_test.db"
	db, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	_, _ = db.Exec("PRAGMA journal_mode=WAL")
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS cache_entries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		cache_key TEXT NOT NULL UNIQUE,
		value TEXT NOT NULL,
		expires_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	t.Run("CacheWriteThroughput", func(t *testing.T) {
		workers := 10
		duration := 2 * time.Second
		var ops, failed int64

		// Host-adaptive floor (CAT-PREEXIST-10): SQLite commit rate is bound by
		// the host's fsync latency, which varies by an order of magnitude under
		// IO pressure (measured 23-110 ops/sec on a loaded host). Calibrate a
		// serial single-writer baseline on the same DB, then require the
		// concurrent run to retain a fraction of it, capped at 100 ops/sec.
		//
		// Scope: this covers driver/host SQLite write throughput only, NOT the
		// product cache code. Limit: with a 20% floor the oracle only catches
		// roughly 5x or worse degradations; a 2x slowdown passes. Any write
		// error fails the test (assert.Zero on failed writes).
		var baseOps int
		baseStart := time.Now()
		for time.Since(baseStart) < 500*time.Millisecond {
			_, berr := db.Exec(`INSERT OR REPLACE INTO cache_entries
				(cache_key, value, expires_at)
				VALUES (?, 'v', datetime('now', '+1 hour'))`,
				fmt.Sprintf("baseline_%d", baseOps))
			require.NoError(t, berr)
			baseOps++
		}
		baseRate := float64(baseOps) / time.Since(baseStart).Seconds()
		require.Greater(t, baseRate, 0.0)
		floor := math.Min(100.0, baseRate*0.2)
		if floor < 5.0 {
			t.Logf("WARNING: computed floor %.2f ops/sec is under 5; this run is near-tautological", floor)
		}

		done := make(chan struct{})
		var wg sync.WaitGroup

		for w := 0; w < workers; w++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				counter := 0
				for {
					select {
					case <-done:
						return
					default:
						key := fmt.Sprintf("throughput_%d_%d",
							id, counter)
						_, execErr := db.Exec(
							`INSERT OR REPLACE INTO cache_entries
							 (cache_key, value, expires_at)
							 VALUES (?, 'v',
							  datetime('now', '+1 hour'))`,
							key,
						)
						if execErr != nil {
							atomic.AddInt64(&failed, 1)
						} else {
							atomic.AddInt64(&ops, 1)
						}
						counter++
					}
				}
			}(w)
		}

		time.Sleep(duration)
		close(done)

		// Wait with a hard deadline to prevent hanging
		wgDone := make(chan struct{})
		go func() {
			wg.Wait()
			close(wgDone)
		}()
		select {
		case <-wgDone:
		case <-time.After(10 * time.Second):
			t.Fatal("CacheWriteThroughput: goroutines did not finish within deadline")
		}

		totalOps := atomic.LoadInt64(&ops)
		opsPerSec := float64(totalOps) / duration.Seconds()

		t.Logf("Cache write throughput: %d ops in %v (%.0f ops/sec)",
			totalOps, duration, opsPerSec)

		t.Logf("baseline serial %.0f ops/sec, floor %.1f ops/sec, failed writes %d",
			baseRate, floor, atomic.LoadInt64(&failed))

		assert.Zero(t, atomic.LoadInt64(&failed), "no write may fail")
		assert.Greater(t, opsPerSec, floor,
			"concurrent writes must retain >=20%% of the serial baseline (cap 100 ops/sec)")
	})
}
