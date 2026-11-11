package realtime

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/internal/media/database"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Nested-write-while-cursor-open class (see internal/poolguard). The media database is an
// SQLCipher *sql.DB with NO connection cap, so it cannot pool-deadlock (the nested Exec simply
// opens a second connection); the hazard here is different: in the production journal mode
// (the default rollback journal; setupSMBTestMediaDB switches to WAL to hide exactly this) the
// nested UPDATE on the second connection cannot commit while the outer SELECT cursor holds its
// shared lock, so it waits out the busy timeout and then fails with "database is locked" -- an
// error checkMediaItemIntegrity discards, silently losing the 'missing' status update.
func TestSMBChangeWatcher_CheckMediaItemIntegrity_UpdatesWhileProductionJournalMode(t *testing.T) {
	mediaDB, err := database.NewMediaDatabase(database.DatabaseConfig{
		Path:     filepath.Join(t.TempDir(), "prod_mode_media.db"),
		Password: "test_password",
	}, zap.NewNop())
	require.NoError(t, err)
	defer mediaDB.Close()
	db := mediaDB.GetDB()

	var mode string
	require.NoError(t, db.QueryRow(`PRAGMA journal_mode`).Scan(&mode))
	require.NotEqual(t, "wal", mode, "the test must run in the production (non-WAL) journal mode")

	// Three sparse items under one directory: each must be marked 'missing'.
	for i := 1; i <= 3; i++ {
		_, err = db.Exec(`INSERT INTO media_items (id, media_type_id, title, status) VALUES (?, 1, ?, 'active')`, i, fmt.Sprintf("Sparse %d", i))
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO media_files (id, media_item_id, file_path, smb_root, filename, file_size) VALUES (?, ?, ?, 'test_storage', ?, 2048)`,
			i, i, fmt.Sprintf("/sparse/movie%d.mp4", i), fmt.Sprintf("movie%d.mp4", i))
		require.NoError(t, err)
	}

	w := NewSMBChangeWatcher(mediaDB, nil, zap.NewNop())
	start := time.Now()
	done := make(chan struct{})
	go func() { defer close(done); w.checkMediaItemIntegrity(context.Background(), "/sparse/movie1.mp4", "test_storage") }()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("checkMediaItemIntegrity still running after 60s (nested write blocked behind its own cursor)")
	}

	rows, err := db.Query(`SELECT id, status FROM media_items ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	for rows.Next() {
		var id int
		var status string
		require.NoError(t, rows.Scan(&id, &status))
		require.Equal(t, "missing", status, "item %d: the nested UPDATE was lost (elapsed %v)", id, time.Since(start))
	}
}

// ProcessPendingChanges processed each change_log row (which itself INSERTs into change_log and
// UPDATEs media tables) while its SELECT cursor over change_log was still open: same hazard, same
// production journal mode. Expected: every pending row processed, one audit row logged per change,
// and the sparse item marked 'missing'.
func TestSMBChangeWatcher_ProcessPendingChanges_WritesWhileProductionJournalMode(t *testing.T) {
	mediaDB, err := database.NewMediaDatabase(database.DatabaseConfig{
		Path:     filepath.Join(t.TempDir(), "prod_mode_media.db"),
		Password: "test_password",
	}, zap.NewNop())
	require.NoError(t, err)
	defer mediaDB.Close()
	db := mediaDB.GetDB()

	// The embedded schema does not create change_log (only the schema.sql shipped beside the DB does).
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS change_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT, entity_type TEXT NOT NULL, entity_id TEXT NOT NULL,
		change_type TEXT NOT NULL, new_data TEXT, detected_at TIMESTAMP NOT NULL, processed INTEGER DEFAULT 0)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO media_items (id, media_type_id, title, status) VALUES (1, 1, 'Sparse', 'active')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO media_files (id, media_item_id, file_path, smb_root, filename, file_size) VALUES (1, 1, '/sparse/movie.mp4', 'test_storage', 'movie.mp4', 2048)`)
	require.NoError(t, err)
	for i := 0; i < 3; i++ {
		path := fmt.Sprintf("/sparse/movie%d.mp4", i)
		if i == 0 {
			path = "/sparse/movie.mp4"
		}
		_, err = db.Exec(`INSERT INTO change_log (entity_type, entity_id, change_type, new_data, detected_at, processed) VALUES ('file', ?, 'deleted', ?, ?, 0)`,
			path, `{"smb_root":"test_storage","size":2048,"is_dir":false}`, time.Now().Add(time.Duration(i)*time.Second))
		require.NoError(t, err)
	}

	w := NewSMBChangeWatcher(mediaDB, nil, zap.NewNop())
	done := make(chan error, 1)
	go func() { done <- w.ProcessPendingChanges(context.Background()) }()
	select {
	case err = <-done:
		require.NoError(t, err)
	case <-time.After(90 * time.Second):
		t.Fatal("ProcessPendingChanges still running after 90s (nested writes blocked behind its own cursor)")
	}

	var pending, logged int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM change_log WHERE processed = 0`).Scan(&pending))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM change_log`).Scan(&logged))
	require.Equal(t, 3, logged-pending, "processed rows") // 3 seeded + 3 audit rows logged by processChange, the 3 audit rows stay unprocessed
	var status string
	require.NoError(t, db.QueryRow(`SELECT status FROM media_items WHERE id = 1`).Scan(&status))
	require.Equal(t, "missing", status)
}
