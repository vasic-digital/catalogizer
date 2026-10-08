package services

// WF22 review probe R17: the generic scanner through processScanJob with a REAL (SQLite) database - the layer no author test exercises
// (every job/realfs test passes db=nil, where insertFileRecord only counts). Scratch copy only.

import (
	"context"
	"database/sql"
	"sort"
	"testing"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fileRow struct {
	id     int64
	path   string
	isDir  bool
	parent *int64
}

func dumpFiles(t *testing.T, db *database.DB) []fileRow {
	rows, err := db.Query(`SELECT id, path, is_directory, parent_id FROM files ORDER BY path`)
	require.NoError(t, err)
	defer rows.Close()
	var out []fileRow
	for rows.Next() {
		var r fileRow
		require.NoError(t, rows.Scan(&r.id, &r.path, &r.isDir, &r.parent))
		out = append(out, r)
	}
	return out
}


func reviewDB(t *testing.T) *database.DB {
	sqlDB, err := sql.Open("sqlite3", "file:r17?mode=memory&cache=shared")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	for _, q := range []string{
		`CREATE TABLE storage_roots (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, protocol TEXT NOT NULL, host TEXT, port INTEGER, path TEXT, username TEXT, password TEXT, domain TEXT, mount_point TEXT, options TEXT, url TEXT, enabled BOOLEAN DEFAULT 1, max_depth INTEGER DEFAULT 10, enable_duplicate_detection BOOLEAN DEFAULT 1, enable_metadata_extraction BOOLEAN DEFAULT 1, include_patterns TEXT, exclude_patterns TEXT, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP, last_scan_at DATETIME)`,
		`CREATE TABLE files (id INTEGER PRIMARY KEY AUTOINCREMENT, storage_root_id INTEGER NOT NULL, path TEXT NOT NULL, name TEXT NOT NULL, extension TEXT, mime_type TEXT, file_type TEXT, size INTEGER NOT NULL, is_directory BOOLEAN DEFAULT 0, created_at DATETIME DEFAULT CURRENT_TIMESTAMP, modified_at DATETIME NOT NULL, accessed_at DATETIME, deleted BOOLEAN DEFAULT 0, deleted_at DATETIME, last_scan_at DATETIME DEFAULT CURRENT_TIMESTAMP, last_verified_at DATETIME, md5 TEXT, sha256 TEXT, sha1 TEXT, blake3 TEXT, quick_hash TEXT, is_duplicate BOOLEAN DEFAULT 0, duplicate_group_id INTEGER, parent_id INTEGER, FOREIGN KEY (storage_root_id) REFERENCES storage_roots(id), FOREIGN KEY (parent_id) REFERENCES files(id))`,
		`CREATE UNIQUE INDEX idx_files_root_path ON files(storage_root_id, path)`,
	} {
		_, err := sqlDB.Exec(q)
		require.NoError(t, err)
	}
	return database.WrapDB(sqlDB, database.DialectSQLite)
}

func TestReviewProbe_R17_GenericScannerWithRealDB(t *testing.T) {
	db := reviewDB(t)
	defer db.Close()
	run := func() *ScanStatus {
		us := NewUniversalScanner(db, zap.NewNop(), nil, &memFactory{client: seededTree()})
		job := ScanJob{ID: "r17", Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(),
			StorageRoot: &models.StorageRoot{Name: "r17-root", Protocol: "ftp"}}
		us.processScanJob(job, 0)
		st, ok := us.GetActiveScanStatus("r17")
		require.True(t, ok)
		s := st.GetSnapshot()
		us.Stop()
		return &s
	}
	st := run()
	rows := dumpFiles(t, db)
	byPath := map[string]fileRow{}
	var paths []string
	for _, r := range rows {
		byPath[r.path] = r
		paths = append(paths, r.path)
		par := "NULL"
		if r.parent != nil {
			par = byPath2(rows, *r.parent)
		}
		t.Logf("R17 row id=%d path=%q dir=%v parent=%s", r.id, r.path, r.isDir, par)
	}
	sort.Strings(paths)
	t.Logf("R17 first scan: status=%s found=%d errors=%d rows=%d", st.Status, st.FilesFound, st.ErrorCount, len(rows))
	require.Equal(t, "completed", st.Status, "control: the job completes against the real DB")
	require.Contains(t, byPath, "/music/b.flac", "control: rows were written with the scanner's paths")

	firstIDs := map[string]int64{}
	for _, r := range rows {
		firstIDs[r.path] = r.id
	}
	// (a) exactly one directory row per directory of the tree
	dirRows := 0
	for _, r := range rows {
		if r.isDir {
			dirRows++
		}
	}
	assert.Equal(t, 2, dirRows, "the tree has 2 directories (/music, /music/album); got %d directory rows: %v", dirRows, paths)
	// (b) a file's parent is the directory row the scanner recorded
	if f, ok := byPath["/music/b.flac"]; ok && f.parent != nil {
		assert.Equal(t, byPath["/music"].id, *f.parent, "the parent of /music/b.flac must be the /music row")
	}

	// (c) a rescan keeps file ids stable (media_files / media_items reference files.id)
	_ = run()
	rows2 := dumpFiles(t, db)
	changed := 0
	for _, r := range rows2 {
		if id, ok := firstIDs[r.path]; ok && id != r.id {
			changed++
		}
	}
	t.Logf("R17 rescan: %d of %d rows got a new id; rows after rescan=%d", changed, len(firstIDs), len(rows2))
	assert.Equal(t, 0, changed, "a rescan of an unchanged tree must not renumber files (foreign keys point at files.id)")
}

func byPath2(rows []fileRow, id int64) string {
	for _, r := range rows {
		if r.id == id {
			return r.path
		}
	}
	return "?"
}
