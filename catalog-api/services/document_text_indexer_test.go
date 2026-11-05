package services

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS (§11.4.289): real migrated SQLite database, real files on disk, real production
// extractor for PDF (poppler or the isolated pure-Go child). D4 of docs/qa/live-conv-docs-subs-r1-20261104.md.

type indexerEnv struct {
	db   *database.DB
	root string
	id   int64
}

func newIndexerEnv(t *testing.T, protocol string) *indexerEnv {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	root := t.TempDir()
	res, err := db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r', ?, ?, 1)`, protocol, root)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return &indexerEnv{db: db, root: root, id: id}
}

func (e *indexerEnv) addFile(t *testing.T, rel, ext string, content []byte, size int64) int64 {
	t.Helper()
	if content != nil {
		require.NoError(t, os.WriteFile(filepath.Join(e.root, rel), content, 0o644))
	}
	res, err := e.db.Exec(`INSERT INTO files (storage_root_id, path, name, extension, size, is_directory, modified_at) VALUES (?,?,?,?,?,0,'2026-01-01 00:00:00')`,
		e.id, rel, filepath.Base(rel), ext, size)
	require.NoError(t, err)
	fid, _ := res.LastInsertId()
	return fid
}

func (e *indexerEnv) text(t *testing.T, fid int64) (string, string, bool) {
	t.Helper()
	var c, s string
	err := e.db.QueryRow(`SELECT content, status FROM file_text_index WHERE file_id = ?`, fid).Scan(&c, &s)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false
	}
	require.NoError(t, err)
	return c, s, true
}

func TestDocumentTextIndexer_TxtMdPdfAndSkips(t *testing.T) {
	e := newIndexerEnv(t, "local")
	pdf, err := os.ReadFile(filepath.Join("testdata", "pdf", "multi.pdf"))
	require.NoError(t, err)
	txt := e.addFile(t, "a.txt", "txt", []byte("hello UNIQ-TXT-1"), 16)
	md := e.addFile(t, "b.MD", "MD", []byte("# t\nUNIQ-MD-2"), 13)
	pd := e.addFile(t, "c.pdf", "pdf", pdf, int64(len(pdf)))
	bin := e.addFile(t, "d.bin", "bin", []byte("UNIQ-BIN-3"), 10)
	missing := e.addFile(t, "gone.txt", "txt", nil, 5) // row without a file on disk

	x := NewDocumentTextIndexer(e.db)
	indexed, failed, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)

	c, s, ok := e.text(t, txt)
	assert.True(t, ok)
	assert.Equal(t, "ok", s)
	assert.Contains(t, c, "UNIQ-TXT-1")
	c, _, _ = e.text(t, md)
	assert.Contains(t, c, "UNIQ-MD-2", "extension case is ignored")
	if _, lerr := lookPath("prlimit"); lerr == nil {
		c, s, _ = e.text(t, pd)
		assert.Equal(t, "ok", s)
		assert.Contains(t, c, "lazy dog")
		assert.Equal(t, 3, indexed)
	}
	_, _, ok = e.text(t, bin)
	assert.False(t, ok, "non-document files are never indexed")
	_, s, ok = e.text(t, missing)
	assert.True(t, ok)
	assert.Equal(t, "failed", s, "an unreadable file is recorded as failed, with no text")
	assert.GreaterOrEqual(t, failed, 1)

	// idempotent: nothing changed, nothing is re-extracted
	i2, f2, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	assert.Equal(t, 0, i2)
	assert.Equal(t, 0, f2)

	// a changed size triggers re-extraction
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.txt"), []byte("now UNIQ-NEW-9 longer"), 0o644))
	_, err = e.db.Exec(`UPDATE files SET size = 21 WHERE id = ?`, txt)
	require.NoError(t, err)
	_, _, err = x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	c, _, _ = e.text(t, txt)
	assert.Contains(t, c, "UNIQ-NEW-9")
	assert.NotContains(t, c, "UNIQ-TXT-1")
}

func TestDocumentTextIndexer_RefusesSymlinkEscapeAndRemoteRoots(t *testing.T) {
	e := newIndexerEnv(t, "local")
	outside := filepath.Join(t.TempDir(), "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("SECRET-OUTSIDE-ROOT"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(e.root, "link.txt")))
	fid := e.addFile(t, "link.txt", "txt", nil, 19)
	_, _, err := NewDocumentTextIndexer(e.db).IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	c, s, _ := e.text(t, fid)
	assert.Equal(t, "failed", s)
	assert.NotContains(t, c, "SECRET")

	r := newIndexerEnv(t, "smb")
	rf := r.addFile(t, "x.txt", "txt", []byte("REMOTE-TOKEN"), 12)
	i, f, err := NewDocumentTextIndexer(r.db).IndexStorageRoot(context.Background(), r.id)
	require.NoError(t, err)
	assert.Equal(t, 0, i+f, "only local roots are indexed")
	_, _, ok := r.text(t, rf)
	assert.False(t, ok)
}

func TestDocumentTextIndexer_DeletedFilesNotIndexed(t *testing.T) {
	e := newIndexerEnv(t, "local")
	fid := e.addFile(t, "a.txt", "txt", []byte("DEL-TOKEN"), 9)
	_, err := e.db.Exec(`UPDATE files SET deleted = 1 WHERE id = ?`, fid)
	require.NoError(t, err)
	_, _, err = NewDocumentTextIndexer(e.db).IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	_, _, ok := e.text(t, fid)
	assert.False(t, ok, "deleted files are not indexed")
}
