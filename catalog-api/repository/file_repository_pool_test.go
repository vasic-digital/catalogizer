package repository

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

// RED/GREEN regression for the LIVE scaling finding (docs/TESTING_GUIDE.md "Scaling and chaos (live)"):
// SearchFiles ran the per-row metadata query while its own result cursor was still open, so every
// in-flight search held one pooled connection and asked for a second. With more concurrent searches
// than pool connections (25 in production; 32 live clients) every search waited for a connection
// that only another waiting search could free: a pool deadlock that ends only when the client gives up.
// Real SQLite file, production migrations, real repository; no mock.
func TestSearchFiles_ConcurrentSearchesExceedingPoolDoNotDeadlock(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "pool.db")+"?_busy_timeout=5000&_journal_mode=WAL")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(3) // the production default is 25; the mechanism is identical for any N < concurrency
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))

	_, err = sqlDB.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/x',1)`)
	require.NoError(t, err)
	for i := 0; i < 60; i++ {
		_, err = sqlDB.Exec(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at)
			VALUES (1, ?, ?, 'mp4', 'video/mp4', 'video', 10, 0, '2026-01-01 00:00:00')`, fmt.Sprintf("/x/f%03d.mp4", i), fmt.Sprintf("f%03d.mp4", i))
		require.NoError(t, err)
	}

	repo := NewFileRepository(db)
	const searches = 12 // > MaxOpenConns
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	errs := make([]error, searches)
	for i := 0; i < searches; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res, e := repo.SearchFiles(ctx, models.SearchFilter{Extension: "mp4"}, models.PaginationOptions{Page: 1, Limit: 50}, models.SortOptions{})
			if e == nil && len(res.Files) != 50 {
				e = fmt.Errorf("got %d files, want 50", len(res.Files))
			}
			errs[i] = e
		}(i)
	}
	wg.Wait()
	for i, e := range errs {
		require.NoError(t, e, "search %d (pool deadlock if this is a context deadline)", i)
	}
}
