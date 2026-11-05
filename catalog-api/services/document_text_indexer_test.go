package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

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
	if _, lerr := lookPath("prlimit"); lerr != nil {
		t.Log("SKIP(§11.4.69 hardware_not_present/tool): prlimit absent, PDF leg not exercised")
	} else {
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

// --- review round 1 additions (F1, F3, F5, F6) ---

type fakeExtractor struct {
	pages  int
	gotEnd int
	err    error
	calls  int
	cur    *int32
	peak   *int32
}

func (f *fakeExtractor) NumPages(context.Context, string) (int, error) { return f.pages, f.err }
func (f *fakeExtractor) ExtractPages(_ context.Context, _ string, _, end int) ([]string, error) {
	f.gotEnd = end
	f.calls++
	return []string{"PAGETEXT"}, nil
}

func TestDocumentTextIndexer_NonRegularFileDoesNotBlock(t *testing.T) { // kills RM1
	e := newIndexerEnv(t, "local")
	require.NoError(t, syscall.Mkfifo(filepath.Join(e.root, "x.txt"), 0o644))
	fid := e.addFile(t, "x.txt", "txt", nil, 1)
	done := make(chan struct{})
	go func() {
		_, _, _ = NewDocumentTextIndexer(e.db).IndexStorageRoot(context.Background(), e.id)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("indexer blocked on a FIFO")
	}
	_, s, _ := e.text(t, fid)
	assert.Equal(t, "failed", s)
}

func TestDocumentTextIndexer_PlainTextReadCap(t *testing.T) { // kills RM5
	e := newIndexerEnv(t, "local")
	big := make([]byte, docIndexMaxPlainBytes+4096)
	for i := range big {
		big[i] = 'a'
	}
	copy(big[docIndexMaxPlainBytes+100:], "BEYONDCAP")
	fid := e.addFile(t, "big.txt", "txt", big, int64(len(big)))
	_, _, err := NewDocumentTextIndexer(e.db).IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	c, _, _ := e.text(t, fid)
	assert.NotContains(t, c, "BEYONDCAP")
	assert.LessOrEqual(t, len(c), docIndexMaxStoredBytes) // also kills RM2: 4MiB read > 2MiB stored cap
	assert.Equal(t, docIndexMaxStoredBytes, len(c))
}

func TestDocumentTextIndexer_MtimeOnlyChangeReindexes(t *testing.T) { // kills RM3
	e := newIndexerEnv(t, "local")
	fid := e.addFile(t, "a.txt", "txt", []byte("AAAA OLDWORD"), 12)
	x := NewDocumentTextIndexer(e.db)
	_, _, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.txt"), []byte("AAAA NEWWORD"), 0o644)) // same size
	_, err = e.db.Exec(`UPDATE files SET modified_at = '2026-02-02 00:00:00' WHERE id = ?`, fid)
	require.NoError(t, err)
	_, _, err = x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	c, _, _ := e.text(t, fid)
	assert.Contains(t, c, "NEWWORD")
}

func TestDocumentTextIndexer_PDFPageCap(t *testing.T) { // kills RM4 (extractor stub: unit-class)
	e := newIndexerEnv(t, "local")
	e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	fx := &fakeExtractor{pages: 5000}
	x := NewDocumentTextIndexer(e.db)
	x.extractor = fx
	_, _, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	assert.Equal(t, docIndexMaxPDFPages, fx.gotEnd)
}

func TestDocumentTextIndexer_EnvironmentFailureNotPersisted(t *testing.T) { // F5
	e := newIndexerEnv(t, "local")
	fid := e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	x := NewDocumentTextIndexer(e.db)
	x.extractor = &fakeExtractor{err: errPrlimitMissing}
	_, f, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	assert.Equal(t, 1, f)
	_, _, ok := e.text(t, fid)
	assert.False(t, ok, "a missing tool must not be stored as a permanent failure")
}

// F6: a parent directory swapped for a symlink out of the root must not be read.
func TestDocumentTextIndexer_ParentSymlinkSwapEscape(t *testing.T) {
	e := newIndexerEnv(t, "local")
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "s.txt"), []byte("SECRET-VIA-PARENT"), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(e.root, "sub")))
	fid := e.addFile(t, "sub/s.txt", "txt", nil, 17)
	_, _, err := NewDocumentTextIndexer(e.db).IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	c, s, _ := e.text(t, fid)
	assert.Equal(t, "failed", s)
	assert.NotContains(t, c, "SECRET")
}

// F1: concurrent calls for one root coalesce: the extractor is never entered by two passes at once.
func TestDocumentTextIndexer_SingleFlightPerRoot(t *testing.T) {
	e := newIndexerEnv(t, "local")
	e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	var cur, peak int32
	x := NewDocumentTextIndexer(e.db)
	x.extractor = &gateExtractor{cur: &cur, peak: &peak}
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, _ = x.IndexStorageRoot(context.Background(), e.id) }()
	}
	wg.Wait()
	assert.EqualValues(t, 1, atomic.LoadInt32(&peak), "indexer passes must not overlap per root")
}

type gateExtractor struct{ cur, peak *int32 }

func (g *gateExtractor) NumPages(context.Context, string) (int, error) {
	n := atomic.AddInt32(g.cur, 1)
	for {
		p := atomic.LoadInt32(g.peak)
		if n <= p || atomic.CompareAndSwapInt32(g.peak, p, n) {
			break
		}
	}
	time.Sleep(50 * time.Millisecond)
	atomic.AddInt32(g.cur, -1)
	return 0, nil
}
func (g *gateExtractor) ExtractPages(context.Context, string, int, int) ([]string, error) {
	return nil, nil
}

// F1: PDF extraction takes a slot of the shared reader budget.
func TestDocumentTextIndexer_UsesSharedPDFSlot(t *testing.T) {
	e := newIndexerEnv(t, "local")
	e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	var taken []func()
	for i := 0; i < PDFReaderSlots; i++ {
		r, ok := AcquirePDFReaderSlot()
		require.True(t, ok)
		taken = append(taken, r)
	}
	x := NewDocumentTextIndexer(e.db)
	x.extractor = &fakeExtractor{pages: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	_, _, _ = x.IndexStorageRoot(ctx, e.id)
	assert.Equal(t, 0, x.extractor.(*fakeExtractor).calls, "no extraction while all reader slots are taken")
	for _, r := range taken {
		r()
	}
}

type countingReader struct {
	n, limit int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	if c.n >= c.limit {
		return 0, io.EOF
	}
	c.n += int64(len(p))
	return len(p), nil
}

func TestReadPlainCapped_BoundsBytesRead(t *testing.T) { // kills RM5 deterministically (R2-6)
	cr := &countingReader{limit: 8 << 20}
	_, err := readPlainCapped(cr)
	require.NoError(t, err)
	assert.LessOrEqual(t, cr.n, int64(docIndexMaxPlainBytes)+64*1024)
}

// R2-1: with two indexing roots hammering the PDF slots, an interactive non-blocking reader still gets in.
func TestDocumentTextIndexer_DoesNotStarveInteractiveReaders(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for ctx.Err() == nil {
				rel, err := acquireIndexerPDFSlot(ctx)
				if err != nil {
					return
				}
				time.Sleep(20 * time.Millisecond)
				rel()
			}
		}()
	}
	ok := 0
	for i := 0; i < 300; i++ {
		if rel, got := AcquirePDFReaderSlot(); got {
			ok++
			rel()
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	wg.Wait()
	assert.Greater(t, ok, 250, "interactive readers must keep getting a slot")
}

type pathRecorder struct{ path string }

func (p *pathRecorder) NumPages(_ context.Context, path string) (int, error) {
	p.path = path
	return 1, nil
}
func (p *pathRecorder) ExtractPages(_ context.Context, path string, _, _ int) ([]string, error) {
	return []string{"T"}, nil
}

// R2-3: the child must be handed the verified descriptor, not the resolved path.
func TestDocumentTextIndexer_ChildGetsVerifiedFdPath(t *testing.T) {
	e := newIndexerEnv(t, "local")
	e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	rec := &pathRecorder{}
	x := NewDocumentTextIndexer(e.db)
	x.extractor = rec
	_, _, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	assert.Regexp(t, fmt.Sprintf(`^/proc/%d/fd/\d+$`, os.Getpid()), rec.path)
}

// the descriptor keeps the original inode after the path is replaced.
func TestOpenInRoot_FdSurvivesPathSwap(t *testing.T) {
	e := newIndexerEnv(t, "local")
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.txt"), []byte("ORIGINAL"), 0o644))
	f, err := openInRoot(e.root, "a.txt")
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, os.Remove(filepath.Join(e.root, "a.txt")))
	require.NoError(t, os.WriteFile(filepath.Join(e.root, "a.txt"), []byte("JUNK"), 0o644))
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), f.Fd()))
	require.NoError(t, err)
	assert.Equal(t, "ORIGINAL", string(b))
}

// R2-5: a per-file timeout is recorded failed (not retried forever); a missing tool is not recorded.
func TestDocumentTextIndexer_PerFileTimeoutRecordedFailed(t *testing.T) {
	e := newIndexerEnv(t, "local")
	fid := e.addFile(t, "p.pdf", "pdf", []byte("%PDF-1.4 stub"), 13)
	x := NewDocumentTextIndexer(e.db)
	x.extractor = &fakeExtractor{err: context.DeadlineExceeded}
	_, _, err := x.IndexStorageRoot(context.Background(), e.id)
	require.NoError(t, err)
	_, s, ok := e.text(t, fid)
	assert.True(t, ok)
	assert.Equal(t, "failed", s)
}

// R2-2: a call that lands while a pass is finishing is never dropped (kills `if true || again` and the pending loop removal).
type blockExtractor struct {
	enter chan struct{}
	gate  chan struct{}
	calls int32
}

func (b *blockExtractor) NumPages(context.Context, string) (int, error) {
	if atomic.AddInt32(&b.calls, 1) == 1 {
		b.enter <- struct{}{}
		<-b.gate
	}
	return 0, nil
}
func (b *blockExtractor) ExtractPages(context.Context, string, int, int) ([]string, error) {
	return nil, nil
}

func TestDocumentTextIndexer_CoalescedRequestRunsAgain(t *testing.T) {
	e := newIndexerEnv(t, "local")
	e.addFile(t, "p1.pdf", "pdf", []byte("%PDF-1.4 a"), 10)
	bx := &blockExtractor{enter: make(chan struct{}), gate: make(chan struct{})}
	x := NewDocumentTextIndexer(e.db)
	x.extractor = bx
	done := make(chan struct{})
	go func() { _, _, _ = x.IndexStorageRoot(context.Background(), e.id); close(done) }()
	<-bx.enter // first pass is inside the extractor
	e.addFile(t, "p2.pdf", "pdf", []byte("%PDF-1.4 b"), 10)
	_, _, _ = x.IndexStorageRoot(context.Background(), e.id) // coalesced: must cause a second pass
	close(bx.gate)
	<-done
	var n int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM file_text_index`).Scan(&n))
	assert.Equal(t, 2, n, "the file added during the first pass must be indexed by the follow-up pass")
}
