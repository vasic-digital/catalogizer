package services

// WF22 fix round 2 (11.4.276 single fixer): tests for the reviewer's findings and for every reviewer mutant that survived the author's tests.
// Each test names what it pins. The database tests use the REAL SQLite schema of the product (internal/tests), not db=nil: the layer the
// reviewer found nobody had exercised (T1).

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/eventbus"
	"catalogizer/models"

	"digital.vasic.filesystem/pkg/fabric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// realDB opens an in-memory SQLite and runs the PRODUCTION migrations on it (files, storage_roots incl. allow_empty), not a hand-written schema.
func realDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1) // every connection to :memory: is a separate database
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return db
}

type catalogRow struct {
	id         int64
	path       string
	name       string
	size       int64
	isDir      bool
	parentPath string // "" = no parent
	deleted    bool
}

func catalogRows(t *testing.T, db *database.DB) map[string]catalogRow {
	t.Helper()
	rows, err := db.Query(`SELECT f.id, f.path, f.name, f.size, f.is_directory, COALESCE(p.path, ''), f.deleted FROM files f LEFT JOIN files p ON p.id = f.parent_id ORDER BY f.path`)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]catalogRow{}
	for rows.Next() {
		var r catalogRow
		require.NoError(t, rows.Scan(&r.id, &r.path, &r.name, &r.size, &r.isDir, &r.parentPath, &r.deleted))
		out[r.path] = r
	}
	require.NoError(t, rows.Err())
	return out
}

func scanWithDB(t *testing.T, db *database.DB, protocol string, client filesystem.FileSystemClient, root *models.StorageRoot) *ScanStatus {
	t.Helper()
	us := NewUniversalScanner(db, zap.NewNop(), nil, &memFactory{client: client})
	job := ScanJob{ID: "job-" + root.Name, Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(), StorageRoot: root}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus(job.ID)
	require.True(t, ok)
	snap := st.GetSnapshot()
	us.Stop()
	return &snap
}

// ---- R17: one path convention in the catalog ---------------------------------------------------------------------------------------------

func TestWF22_CatalogPathConvention(t *testing.T) {
	for in, want := range map[string]string{
		"/": "", "": "", "/music": "music", "/music/b.flac": "music/b.flac", "music/b.flac": "music/b.flac",
		"//music//b.flac": "music/b.flac", "/a/./b": "a/b", "/a/../b": "b", "/..": "",
	} {
		assert.Equal(t, want, catalogPath(in), "catalogPath(%q)", in)
	}
}

// TestWF22_R17_EveryRemoteProtocolWritesTheSameTreeWithoutDuplicatesOrPhantomParents drives each production scanner through the real job path
// against a real database and checks the rows themselves.
func TestWF22_R17_EveryRemoteProtocolWritesTheSameTreeWithoutDuplicatesOrPhantomParents(t *testing.T) {
	want := map[string]catalogRow{
		"a.mkv":             {name: "a.mkv", size: 100},
		"music":             {name: "music", isDir: true},
		"music/b.flac":      {name: "b.flac", size: 200, parentPath: "music"},
		"music/album":       {name: "album", isDir: true, parentPath: "music"},
		"music/album/c.mp3": {name: "c.mp3", size: 300, parentPath: "music/album"},
	}
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		t.Run(proto, func(t *testing.T) {
			db := realDB(t)
			st := scanWithDB(t, db, proto, seededTree(), &models.StorageRoot{Name: "root-" + proto, Protocol: proto})
			require.Equal(t, "completed", st.Status, st.Reason)
			got := catalogRows(t, db)
			require.Len(t, got, len(want), "exactly one row per entry: %v", keysOf(got))
			for p, w := range want {
				g, ok := got[p]
				require.True(t, ok, "row %q missing; have %v", p, keysOf(got))
				assert.Equal(t, w.name, g.name, p)
				assert.Equal(t, w.size, g.size, p)
				assert.Equal(t, w.isDir, g.isDir, p)
				assert.Equal(t, w.parentPath, g.parentPath, "%s: its parent is the row the scanner recorded for the directory", p)
			}
			for p := range got {
				assert.False(t, strings.HasPrefix(p, "/"), "no leading slash in the catalog: %q", p)
			}
		})
	}
}

func keysOf(m map[string]catalogRow) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TestWF22_R17_SMBAndGenericScannerAgreeOnTheCatalogConvention: the same tree scanned by the SMB scanner and by the generic scanner gives the
// same rows (path, directory flag, parent).
func TestWF22_R17_SMBAndGenericScannerAgreeOnTheCatalogConvention(t *testing.T) {
	// the SMB scanner lists job.Path "" first; the memFS keys that directory as "."
	smbTree := newMemFS()
	smbTree.addDir(".")
	smbTree.addFile(".", "a.mkv", 100, t0)
	smbTree.addSubdir(".", "music")
	smbTree.addFile("music", "b.flac", 200, t0)
	smbTree.addSubdir("music", "album")
	smbTree.addFile("music/album", "c.mp3", 300, t0)

	dbS := realDB(t)
	sc := NewSMBScanner(dbS, zap.NewNop())
	stS := &ScanStatus{}
	require.NoError(t, sc.ScanPath(context.Background(), smbTree, ScanJob{ID: "s", Path: "", MaxDepth: 10, StorageRoot: &models.StorageRoot{Name: "smb-root", Protocol: "smb"}}, stS))

	dbG := realDB(t)
	stG := scanWithDB(t, dbG, "ftp", seededTree(), &models.StorageRoot{Name: "ftp-root", Protocol: "ftp"})
	require.Equal(t, "completed", stG.Status, stG.Reason)

	shape := func(rows map[string]catalogRow) []string {
		var out []string
		for p, r := range rows {
			out = append(out, fmt.Sprintf("%s|dir=%v|parent=%s", p, r.isDir, r.parentPath))
		}
		sort.Strings(out)
		return out
	}
	assert.Equal(t, shape(catalogRows(t, dbS)), shape(catalogRows(t, dbG)), "SMB (slashless paths) and the generic scanner write the same rows (WF22 R17a/B05r)")
}

// TestWF22_R17b_RescanKeepsIdsAndUpdatesTheRows: the upsert keeps files.id (media_files point at it), updates what changed, revives a deleted
// row and keeps created_at.
func TestWF22_R17b_RescanKeepsIdsAndUpdatesTheRows(t *testing.T) {
	db := realDB(t)
	root := &models.StorageRoot{Name: "root-rescan", Protocol: "ftp"}
	m := seededTree()
	require.Equal(t, "completed", scanWithDB(t, db, "ftp", m, root).Status)
	first := catalogRows(t, db)

	_, err := db.Exec(`UPDATE files SET created_at = '2001-02-03 04:05:06', deleted = 1, deleted_at = CURRENT_TIMESTAMP WHERE path = 'music/b.flac'`)
	require.NoError(t, err)
	m.setFile("/music", "b.flac", 999, t0.Add(time.Hour))
	m.addFile("/music", "new.flac", 5, t0)

	require.Equal(t, "completed", scanWithDB(t, db, "ftp", m, root).Status)
	second := catalogRows(t, db)
	require.Len(t, second, len(first)+1, "one new row, no duplicates")
	for p, r := range first {
		assert.Equal(t, r.id, second[p].id, "%s keeps its id across a rescan", p)
	}
	b := second["music/b.flac"]
	assert.Equal(t, int64(999), b.size, "the changed size is stored")
	assert.False(t, b.deleted, "a row that is seen again is no longer deleted")
	var created string
	require.NoError(t, db.QueryRow(`SELECT created_at FROM files WHERE path = 'music/b.flac'`).Scan(&created))
	assert.Contains(t, created, "2001-02-03", "created_at survives (INSERT OR REPLACE would have reset it)")
	assert.Equal(t, "music", second["music/new.flac"].parentPath)
}

func TestWF22_B06_B07_EveryProductionScannerIsWiredToTheDatabase(t *testing.T) {
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		db := realDB(t)
		us := NewUniversalScanner(db, zap.NewNop(), nil, &memFactory{client: seededTree()})
		us.protocolScannersMu.RLock()
		g := us.protocolScanners[proto].(*GenericScanner)
		us.protocolScannersMu.RUnlock()
		assert.Same(t, db, g.o.DB, "%s: the scanner built by NewUniversalScanner writes to the database", proto)
		us.Stop()
	}
}

// ---- names ------------------------------------------------------------------------------------------------------------------------------

func TestWF22_ValidEntryName(t *testing.T) {
	for _, n := range []string{"a.mkv", "Wait.. Live", "...And Justice", "café", " lead", "trail ", "back\\slash", strings.Repeat("A", 255), "日本語"} {
		assert.True(t, validEntryName(n), "%q", n)
	}
	for _, n := range []string{"", ".", "..", "a/b", "x\x00y", "line\nbreak", "cr\rname", "tab\tname", "bell\x07", "del\x7f", strings.Repeat("A", 256)} {
		assert.False(t, validEntryName(n), "%q", n)
	}
}

// ---- bounds -----------------------------------------------------------------------------------------------------------------------------

func chainTree(levels int, leaf bool) *memFS {
	m := newMemFS()
	m.addDir("/")
	dir := "/"
	for i := 0; i < levels; i++ {
		m.addSubdir(dir, fmt.Sprintf("d%02d", i))
		dir = strings.TrimRight(dir, "/") + fmt.Sprintf("/d%02d", i)
	}
	if leaf {
		m.addFile(dir, "leaf.mkv", 1, t0)
	}
	return m
}

func TestWF22_A01_DefaultDepthIsTen(t *testing.T) {
	// d00..d09 are listed (depth 1..10); the file sits in d09 at depth 10: found with the default. One level more and the bound hides it.
	for _, maxDepth := range []int{0, -3} {
		rec := newRecorder()
		_, err := scanOnce(t, newTestScanner(rec, nil), chainTree(10, true), jobOf("/", maxDepth, "full"))
		require.NoError(t, err, "max_depth %d means the default 10", maxDepth)
		assert.Contains(t, rec.sorted(), "/d00/d01/d02/d03/d04/d05/d06/d07/d08/d09/leaf.mkv")
	}
	_, err := scanOnce(t, newTestScanner(newRecorder(), nil), chainTree(11, true), jobOf("/", 0, "full"))
	require.Error(t, err, "11 levels do not fit the default depth 10")
	assert.Equal(t, FailLimit, scanKind(t, err))
}

func TestWF22_A03_TotalBoundIsExact(t *testing.T) {
	m := seededTree() // 5 entries
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 5 }), m, jobOf("/", 10, "full"))
	require.NoError(t, err, "exactly the bound is allowed")
	_, err = scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 4 }), seededTree(), jobOf("/", 10, "full"))
	require.Error(t, err, "one more than the bound fails")
	assert.Equal(t, FailLimit, scanKind(t, err))
}

func TestWF22_A04_PerDirectoryBoundIsExact(t *testing.T) {
	mk := func(n int) *memFS {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < n; i++ {
			m.addFile("/", fmt.Sprintf("f%02d", i), 1, t0)
		}
		return m
	}
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntriesPerDir = 4 }), mk(4), jobOf("/", 10, "full"))
	require.NoError(t, err, "a directory with exactly the bound is allowed")
	_, err = scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntriesPerDir = 4 }), mk(5), jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailLimit, scanKind(t, err))
}

// A listing under the budget limit that only looks full because of uncounted entries must not fail the scan: the directory is listed again.
func TestWF22_R4_UncountedEntriesDoNotFailATreeThatFitsTheBudget(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	m.addFile("/", ".", 0, t0) // an MLSD-style cdir/pdir pair and a skipped metadata directory inflate the raw listing
	m.addFile("/", "..", 0, t0)
	m.addSubdir("/", "@eaDir")
	m.addFile("/", "a", 1, t0)
	m.addFile("/", "b", 1, t0)
	rec := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec, func(o *GenericScannerOptions) { o.MaxEntries = 2 }), m, jobOf("/", 10, "full"))
	require.NoError(t, err, "2 real entries fit MaxEntries 2")
	assert.Equal(t, []string{"/a", "/b"}, rec.sorted())
	assert.Equal(t, int64(0), st.GetSnapshot().ErrorCount, "'.' and '..' are not errors")
}

// ---- cancellation -----------------------------------------------------------------------------------------------------------------------

func TestWF22_A13_CancelBetweenEntriesOfOneDirectory(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for i := 0; i < 5; i++ {
		m.addFile("/", fmt.Sprintf("f%d", i), 1, t0)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rec := newRecorder()
	n := 0
	inner := rec.record
	g := newTestScanner(rec, nil)
	g.o.Record = func(c context.Context, p string, fi *filesystem.FileInfo, j ScanJob, st *ScanStatus) error {
		n++
		if n == 2 {
			cancel()
		}
		return inner(c, p, fi, j, st)
	}
	err := g.ScanPath(ctx, m, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailCancelled, scanKind(t, err))
	assert.Equal(t, 2, len(rec.sorted()), "no entry is recorded after the cancel, although the directory listing was already complete")
}

func TestWF22_A14_CancelBeforeTheNextDirectoryIsListed(t *testing.T) {
	m := seededTree()
	ctx, cancel := context.WithCancel(context.Background())
	var musicListings int32
	m.listHook = func(dir string) {
		if dir == "/music" {
			atomic.AddInt32(&musicListings, 1)
		}
	}
	rec := newRecorder()
	g := newTestScanner(rec, nil)
	inner := rec.record
	g.o.Record = func(c context.Context, p string, fi *filesystem.FileInfo, j ScanJob, st *ScanStatus) error {
		err := inner(c, p, fi, j, st)
		if p == "/music" { // the LAST entry of the root listing: the cancel arrives after the root was processed, before /music is scheduled
			cancel()
		}
		return err
	}
	st := &ScanStatus{}
	err := g.ScanPath(ctx, m, jobOf("/", 10, "full"), st)
	require.Error(t, err)
	assert.Equal(t, FailCancelled, scanKind(t, err))
	time.Sleep(60 * time.Millisecond) // a listing started anyway would have reached the hook / the status by now
	assert.Equal(t, int32(0), atomic.LoadInt32(&musicListings), "nothing below the root is listed once the context is done")
	// the host-budget and retry decorators refuse a call on a done context, so the client never sees it either way; what only the scanner's own
	// check prevents is STARTING the listing, which shows in the progress it reports
	assert.Equal(t, "/", st.GetSnapshot().CurrentPath, "no listing of /music was started after the cancel")
}

func TestWF22_R7_DeadlineIsATimeoutFailureNotACancellation(t *testing.T) {
	m := seededTree()
	m.listDelay = 300 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := newTestScanner(newRecorder(), nil).ScanPath(ctx, m, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailTimeout, scanKind(t, err))
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Contains(t, err.Error(), "timeout")
}

// ---- failure classification -------------------------------------------------------------------------------------------------------------

type opaqueNotExist struct{}

func (opaqueNotExist) Error() string        { return "opaque failure code 7" }
func (opaqueNotExist) Is(target error) bool { return target == os.ErrNotExist }

type opaquePermission struct{}

func (opaquePermission) Error() string        { return "opaque failure code 8" }
func (opaquePermission) Is(target error) bool { return target == os.ErrPermission }

func TestWF22_A15_A16_BenignClassificationUsesTypesNotJustText(t *testing.T) {
	assert.True(t, isBenignListError(opaqueNotExist{}), "errors.Is(os.ErrNotExist) without any telltale text")
	assert.True(t, isBenignListError(opaquePermission{}), "errors.Is(os.ErrPermission) without any telltale text")
	assert.True(t, isBenignListError(&textproto.Error{Code: 550, Msg: "opaque"}))
	assert.True(t, isBenignListError(&textproto.Error{Code: 553, Msg: "opaque"}))
	assert.False(t, isBenignListError(&textproto.Error{Code: 421, Msg: "Service not available"}))
	assert.False(t, isBenignListError(&net.OpError{Op: "read", Err: syscall.ECONNRESET}))
	assert.False(t, isBenignListError(errors.New("opaque failure code 9")))
	assert.False(t, isBenignListError(nil))
}

func TestWF22_R12_MostOfTheTreeUnreadableIsAFailureNamingTheDirectories(t *testing.T) {
	mk := func(denied int) *memFS {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < 4; i++ {
			d := fmt.Sprintf("d%d", i)
			m.addSubdir("/", d)
			m.addFile("/"+d, "f", 1, t0)
			if i < denied {
				m.listErr["/"+d] = fmt.Errorf("list: %w", os.ErrPermission)
			}
		}
		return m
	}
	// 2 of 4 unreadable = exactly the threshold: allowed, noted
	st := &ScanStatus{}
	err := newTestScanner(newRecorder(), nil).ScanPath(context.Background(), mk(2), jobOf("/", 10, "full"), st)
	require.NoError(t, err)
	assert.Contains(t, st.GetSnapshot().Reason, "2 of 4 sub-directories could not be read: /d0, /d1")
	// 3 of 4 unreadable: the catalog would miss most of the tree
	err = newTestScanner(newRecorder(), nil).ScanPath(context.Background(), mk(3), jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailPartial, scanKind(t, err))
	assert.Contains(t, err.Error(), "3 of 4 sub-directories could not be read: /d0, /d1, /d2")
}

func TestWF22_R15_RecordFailureThreshold(t *testing.T) {
	run := func(failN int) error {
		rec := newRecorder()
		n := 0
		rec.fail = func(string) error {
			n++
			if n <= failN {
				return errors.New("db locked")
			}
			return nil
		}
		return newTestScanner(rec, nil).ScanPath(context.Background(), seededTree(), jobOf("/", 10, "full"), &ScanStatus{})
	}
	assert.NoError(t, run(0))
	assert.NoError(t, run(2), "2 of 5 failed: a smaller catalog, visible in error_count")
	err := run(3)
	require.Error(t, err, "3 of 5 is more than the 50% bound")
	assert.Equal(t, FailStorage, scanKind(t, err))
	assert.Contains(t, err.Error(), "3 of 5 records could not be stored")
}

// ---- Synology metadata ------------------------------------------------------------------------------------------------------------------

func TestWF22_A20_S1_MetadataDirectoriesAreSkippedAtEveryDepth(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for _, d := range []string{"@eaDir", "#recycle", "#snapshot"} {
		m.addSubdir("/", d)
		m.addFile("/"+d, "x", 1, t0)
	}
	m.addSubdir("/", "real")
	m.addFile("/real", "f", 1, t0)
	for _, d := range []string{"@eaDir", "#recycle", "#snapshot"} {
		m.addSubdir("/real", d) // Synology puts @eaDir into EVERY directory
		m.addFile("/real/"+d, "x", 1, t0)
	}
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, func(o *GenericScannerOptions) { o.SkipDirNames = nil }), m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/real", "/real/f"}, rec.sorted())
	for _, p := range m.listedPaths() {
		assert.NotContains(t, p, "@eaDir")
		assert.NotContains(t, p, "#recycle")
		assert.NotContains(t, p, "#snapshot")
	}
}

// ---- workers and the production guard ---------------------------------------------------------------------------------------------------

func TestWF22_A18_WorkersBoundTheListingsInFlight(t *testing.T) {
	mk := func() *memFS {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < 8; i++ {
			d := fmt.Sprintf("d%d", i)
			m.addSubdir("/", d)
			m.addFile("/"+d, "f", 1, t0)
		}
		m.listDelay = 20 * time.Millisecond
		return m
	}
	for _, workers := range []int{1, 3} {
		m := mk()
		_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.Workers = workers }), m, jobOf("/", 10, "full"))
		require.NoError(t, err)
		assert.Equal(t, int32(workers), atomic.LoadInt32(&m.maxInflight), "Workers=%d: exactly that many listings at the peak (an FTP control connection is not goroutine safe)", workers)
	}
}

func TestWF22_A22_A23_ProductionDefaultsOfTheHostGuard(t *testing.T) {
	g := NewGenericScanner(GenericScannerOptions{})
	assert.Equal(t, fabric.NASBudget(), g.o.Budget, "the NAS-safe per-host budget is the default")
	assert.Equal(t, fabric.BudgetConfig{MaxConcurrent: 4, MaxReqPerSec: 10}, g.o.Budget)
	assert.Equal(t, 0.5, g.o.MaxSkippedRatio)
	assert.Equal(t, 0.5, g.o.MaxRecordFailRatio)
	assert.Equal(t, []string{"@eaDir", "#recycle", "#snapshot"}, g.o.SkipDirNames)

	// retry: a read that fails twice with a connection reset succeeds on the third attempt WITHOUT any injected policy
	m := seededTree()
	var calls int32
	m.listHook = func(dir string) {
		if dir == "/music" {
			m.mu.Lock()
			if atomic.AddInt32(&calls, 1) <= 2 {
				m.listErr["/music"] = &net.OpError{Op: "read", Err: syscall.ECONNRESET}
			} else {
				delete(m.listErr, "/music")
			}
			m.mu.Unlock()
		}
	}
	rec := newRecorder()
	prod := NewGenericScanner(GenericScannerOptions{Protocol: "memfs", Logger: zap.NewNop(), Record: rec.record, Budgets: fabric.NewBudgets()})
	h := "memhost"
	job := ScanJob{ID: "j", Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(), StorageRoot: &models.StorageRoot{Name: "memroot", Host: &h}}
	require.NoError(t, prod.ScanPath(context.Background(), m, job, &ScanStatus{}))
	assert.Len(t, rec.sorted(), 5)
	assert.Equal(t, int32(3), atomic.LoadInt32(&calls), "three attempts: the default policy retries a transient failure twice")
}

func TestWF22_A27_TokensAreIsolatedPerStorageRoot(t *testing.T) {
	store := NewMemoryTokenStore()
	mk := func(name string) ScanJob {
		j := jobOf("/", 10, "incremental")
		j.StorageRoot = &models.StorageRoot{Name: name, Protocol: "memfs"}
		return j
	}
	recA := newRecorder()
	gA := newTestScanner(recA, func(o *GenericScannerOptions) { o.Tokens = store })
	require.NoError(t, gA.ScanPath(context.Background(), seededTree(), mk("A"), &ScanStatus{}))
	require.Len(t, recA.sorted(), 5)
	recB := newRecorder()
	gB := newTestScanner(recB, func(o *GenericScannerOptions) { o.Tokens = store })
	require.NoError(t, gB.ScanPath(context.Background(), seededTree(), mk("B"), &ScanStatus{}))
	assert.Len(t, recB.sorted(), 5, "root B has never been scanned: A's tokens do not make B's entries 'unchanged'")
	recA.paths = nil
	require.NoError(t, gA.ScanPath(context.Background(), seededTree(), mk("A"), &ScanStatus{}))
	assert.Empty(t, recA.sorted(), "and A's own second scan skips everything")
}

// ---- job classification (B03r), secrets (R9), panic (P0), events ------------------------------------------------------------------------

type fakeProtocolScanner struct {
	scan func(ctx context.Context, job ScanJob, st *ScanStatus) error
}

func (f *fakeProtocolScanner) ScanPath(ctx context.Context, _ filesystem.FileSystemClient, job ScanJob, st *ScanStatus) error {
	return f.scan(ctx, job, st)
}
func (f *fakeProtocolScanner) GetScanStrategy() ScanStrategy { return ScanStrategy{} }
func (f *fakeProtocolScanner) SupportsIncrementalScan() bool { return false }
func (f *fakeProtocolScanner) GetOptimalBatchSize() int      { return 1 }

func runFake(t *testing.T, ctx context.Context, root *models.StorageRoot, bus *eventbus.EventBus, f func(ctx context.Context, job ScanJob, st *ScanStatus) error) *ScanStatus {
	t.Helper()
	us := NewUniversalScanner(nil, zap.NewNop(), nil, &memFactory{client: seededTree()})
	if bus != nil {
		us.SetEventBus(bus)
	}
	us.RegisterProtocolScanner(root.Protocol, &fakeProtocolScanner{scan: f})
	job := ScanJob{ID: "fake-" + root.Name, Path: "/", MaxDepth: 10, ScanType: "full", Context: ctx, StorageRoot: root}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus(job.ID)
	require.True(t, ok)
	snap := st.GetSnapshot()
	us.Stop()
	return &snap
}

func TestWF22_B03_CancelledIsOnlyACancellationOfTheJobContext(t *testing.T) {
	// an operator cancellation, reported by the scanner as the context error: cancelled
	ctx, cancel := context.WithCancel(context.Background())
	st := runFake(t, ctx, &models.StorageRoot{Name: "c1", Protocol: "ftp"}, nil, func(c context.Context, _ ScanJob, _ *ScanStatus) error {
		cancel()
		return &ScanError{Kind: FailCancelled, Err: c.Err()}
	})
	assert.Equal(t, "cancelled", st.Status)
	// the context is done, but the scan failed for ANOTHER reason: failed, with that reason
	ctx2, cancel2 := context.WithCancel(context.Background())
	st = runFake(t, ctx2, &models.StorageRoot{Name: "c2", Protocol: "ftp"}, nil, func(context.Context, ScanJob, *ScanStatus) error {
		cancel2()
		return errors.New("disk full while recording")
	})
	assert.Equal(t, "failed", st.Status, "a failure that merely coincides with a cancelled context is still a failure")
	assert.Contains(t, st.Reason, "disk full")
	// a deadline: failed
	ctx3, cancel3 := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel3()
	st = runFake(t, ctx3, &models.StorageRoot{Name: "c3", Protocol: "ftp"}, nil, func(c context.Context, _ ScanJob, _ *ScanStatus) error {
		<-c.Done()
		return &ScanError{Kind: FailTimeout, Err: c.Err()}
	})
	assert.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "timeout")
}

func TestWF22_R9_ReasonAndEventNeverCarryACredential(t *testing.T) {
	const pw = "S3cr3t-Pw-Probe"
	bus := eventbus.New(eventbus.DefaultConfig())
	defer bus.Close()
	sub := bus.Subscribe(eventbus.EventScanFailed)
	defer sub.Cancel()
	urlStr := "http://alice:" + pw + "@nas.example/dav"
	root := &models.StorageRoot{Name: "r9", Protocol: "webdav", URL: &urlStr, Password: strPtr("Another-Pw-123")}
	st := runFake(t, context.Background(), root, bus, func(context.Context, ScanJob, *ScanStatus) error {
		return fmt.Errorf("failed to list WebDAV directory http://alice:%s@nas.example/dav/: dial tcp: refused; server said: bad login Another-Pw-123 / %s", pw, pw)
	})
	require.Equal(t, "failed", st.Status)
	assert.NotContains(t, st.Reason, pw)
	assert.NotContains(t, st.Reason, "Another-Pw-123")
	assert.Contains(t, st.Reason, "nas.example", "the rest of the message survives")
	select {
	case evt := <-sub.Channel:
		p := evt.Payload.(map[string]interface{})
		assert.NotContains(t, fmt.Sprint(p["error"]), pw, "the event bus 'error'")
		assert.NotContains(t, fmt.Sprint(p["reason"]), "Another-Pw-123", "the event bus 'reason'")
		assert.Equal(t, st.Reason, p["reason"])
	case <-time.After(2 * time.Second):
		t.Fatal("no scan.failed event")
	}
}

func TestWF22_ScrubSecrets(t *testing.T) {
	assert.Equal(t, "get ftp://***@h/x", scrubSecrets("get ftp://u:pw@h/x", nil))
	assert.Equal(t, "see https://***@h/x ok", scrubSecrets("see https://bob:pw@h/x ok", nil))
	assert.Equal(t, "see sftp://***@h", scrubSecrets("see sftp://bob@h", nil))
	assert.Equal(t, "token *** and %2A", scrubSecrets("token abc123 and %2A", []string{"abc123"}))
	assert.Equal(t, "ab stays", scrubSecrets("ab stays", secretsOf(&models.StorageRoot{Password: strPtr("ab")})), "a secret shorter than 3 is not replaced: it would destroy the message")
	assert.Equal(t, "no userinfo here: http://host/path", scrubSecrets("no userinfo here: http://host/path", nil))
	assert.Nil(t, secretsOf(nil))
}

func TestWF22_P0_PanicLeavesAVisibleFailedStatus(t *testing.T) {
	bus := eventbus.New(eventbus.DefaultConfig())
	defer bus.Close()
	sub := bus.Subscribe(eventbus.EventScanFailed)
	defer sub.Cancel()
	st := runFake(t, context.Background(), &models.StorageRoot{Name: "p0", Protocol: "ftp"}, bus, func(context.Context, ScanJob, *ScanStatus) error {
		panic("boom")
	})
	assert.Equal(t, "failed", st.Status, "the status clients poll is failed, not 'running' for the next 60 seconds")
	assert.Contains(t, st.Reason, "panic: boom")
	assert.GreaterOrEqual(t, st.ErrorCount, int64(1))
	select {
	case evt := <-sub.Channel:
		assert.Contains(t, fmt.Sprint(evt.Payload.(map[string]interface{})["reason"]), "panic: boom")
	case <-time.After(2 * time.Second):
		t.Fatal("no scan.failed event")
	}
}

func TestWF22_R7_CancelledPublishesItsOwnEventWithTheReason(t *testing.T) {
	bus := eventbus.New(eventbus.DefaultConfig())
	defer bus.Close()
	subC := bus.Subscribe(eventbus.EventScanCancelled)
	defer subC.Cancel()
	ctx, cancel := context.WithCancel(context.Background())
	st := runFake(t, ctx, &models.StorageRoot{Name: "r7", Protocol: "ftp"}, bus, func(c context.Context, _ ScanJob, _ *ScanStatus) error {
		cancel()
		return &ScanError{Kind: FailCancelled, Err: c.Err()}
	})
	require.Equal(t, "cancelled", st.Status)
	select {
	case evt := <-subC.Channel:
		p := evt.Payload.(map[string]interface{})
		assert.Equal(t, "cancelled", p["status"])
		assert.Contains(t, fmt.Sprint(p["reason"]), "cancelled")
	case <-time.After(2 * time.Second):
		t.Fatal("no scan.cancelled event")
	}
}

func TestWF22_ConnectFailureReasonIsScrubbedToo(t *testing.T) {
	m := seededTree()
	m.connect = errors.New("login failed for ftp://bob:hunter22@nas/ with hunter22")
	pw := "hunter22"
	us := NewUniversalScanner(nil, zap.NewNop(), nil, &memFactory{client: m})
	job := ScanJob{ID: "cf", Path: "/", MaxDepth: 10, Context: context.Background(), StorageRoot: &models.StorageRoot{Name: "cf", Protocol: "ftp", Password: &pw}}
	us.processScanJob(job, 0)
	s, _ := us.GetActiveScanStatus("cf")
	snap := s.GetSnapshot()
	us.Stop()
	assert.Equal(t, "failed", snap.Status)
	assert.NotContains(t, snap.Reason, "hunter22")
}

func TestWF22_G01_StatusJSONFieldReasonIsPartOfTheScanStatus(t *testing.T) {
	// the API JSON itself is asserted in package handlers; here: the snapshot carries it
	st := &ScanStatus{}
	st.fail(errors.New("because"))
	assert.Equal(t, "because", st.GetSnapshot().Reason)
	st2 := &ScanStatus{}
	st2.setNote("a note")
	st2.setNote("ignored: the first note wins")
	assert.Equal(t, "a note", st2.GetSnapshot().Reason)
	st3 := &ScanStatus{}
	st3.fail(errors.New("failure"))
	st3.setNote("a note never overwrites a failure")
	assert.Equal(t, "failure", st3.GetSnapshot().Reason)
}

// ---- R1: every CreateClient caller goes through the single mapping ----------------------------------------------------------------------

func TestWF22_R1_CoverArtThumbnailCopyBuildsEveryProtocolThroughTheSingleMapping(t *testing.T) {
	s := NewCoverArtService(nil, zap.NewNop())
	s.SetClientFactory(filesystem.NewDefaultClientFactory())
	base := func(proto string) thumbnailTask {
		return thumbnailTask{mediaItemID: 1, videoPath: "/v.mkv", protocol: proto,
			rootPath: sql.NullString{String: "/media", Valid: true}, host: sql.NullString{String: "127.0.0.1", Valid: true},
			port: sql.NullInt64{Int64: 1, Valid: true}, username: sql.NullString{String: "u", Valid: true}, password: sql.NullString{String: "p", Valid: true},
			url: sql.NullString{String: "http://127.0.0.1:1/dav", Valid: true}, mountPoint: sql.NullString{String: "/mnt/none", Valid: true}}
	}
	for _, proto := range []string{"smb", "ftp", "webdav", "nfs"} {
		_, err := s.copyRemoteFileToTemp(context.Background(), base(proto))
		require.Error(t, err, proto)
		assert.NotContains(t, err.Error(), "invalid "+proto+" settings", "%s: the settings contract accepts what the single mapping produces", proto)
		assert.NotContains(t, err.Error(), "failed to create filesystem client", "%s: the factory built a client", proto)
	}
	// a WebDAV task carries its url all the way: without it the client cannot be built into anything usable
	root := base("webdav").storageRoot()
	require.NotNil(t, root.URL)
	assert.Equal(t, "http://127.0.0.1:1/dav", filesystem.SettingsFromRoot(root, nil)["url"])
	nfs := base("nfs").storageRoot()
	assert.Equal(t, "/mnt/none", filesystem.SettingsFromRoot(nfs, nil)["mount_point"])
}

func TestWF22_StorageRootConnColumns(t *testing.T) {
	var r models.StorageRoot
	cols := strings.Split(models.StorageRootConnColumns, ", ")
	assert.Equal(t, len(cols), len(r.ConnScanTargets()), "one Scan destination per selected column")
	assert.Equal(t, "sr.protocol, sr.host, sr.port, sr.path, sr.username, sr.password, sr.domain, sr.url, sr.mount_point, sr.options", models.StorageRootConnColumnsFor("sr"))
	// every column the settings contract can consume is selected
	for _, want := range []string{"host", "port", "path", "username", "password", "domain", "url", "mount_point", "options"} {
		assert.Contains(t, cols, want)
	}
}

type panicFS struct{ *memFS }

func (p *panicFS) ListDirectory(ctx context.Context, d string) ([]*filesystem.FileInfo, error) {
	if d == "/music" {
		panic("nil connection after Disconnect")
	}
	return p.memFS.ListDirectory(ctx, d)
}

// A listing call can run into a client that was disconnected after an abandoned (cancelled) call: a panic in it must become an error of that
// listing, never a crash of the process.
func TestWF22_R13_PanicInAListingBecomesAnError(t *testing.T) {
	err := newTestScanner(newRecorder(), nil).ScanPath(context.Background(), &panicFS{memFS: seededTree()}, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailTransport, scanKind(t, err))
	assert.Contains(t, err.Error(), "nil connection after Disconnect")
}

// M18: a directory over the per-directory bound is reported as exactly that (the message names the bound), not as a generic fetch problem.
func TestWF22_M18_PerDirectoryOverflowNamesThePerDirectoryBound(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for i := 0; i < 5; i++ {
		m.addFile("/", fmt.Sprintf("f%d", i), 1, t0)
	}
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntriesPerDir = 4 }), m, jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailLimit, scanKind(t, err))
	assert.Contains(t, err.Error(), "per-directory bound 4")
}

// A client that ignores the list limit and returns the whole directory is still bounded by the same checks.
type unlimitedFS struct{ *memFS }

func (u *unlimitedFS) ListDirectory(ctx context.Context, p string) ([]*filesystem.FileInfo, error) {
	return u.memFS.ListDirectory(context.Background(), p) // drops the limit carried by ctx
}

func TestWF22_R4_AClientThatIgnoresTheListLimitIsStillBounded(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for i := 0; i < 50; i++ {
		m.addFile("/", fmt.Sprintf("f%02d", i), 1, t0)
	}
	rec := newRecorder()
	err := newTestScanner(rec, func(o *GenericScannerOptions) { o.MaxEntriesPerDir = 10 }).ScanPath(context.Background(), &unlimitedFS{m}, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailLimit, scanKind(t, err))
	assert.Contains(t, err.Error(), "50 entries exceed the per-directory bound 10")
	assert.Empty(t, rec.sorted(), "nothing of an over-sized directory is recorded")

	// and the total budget: 50 entries over MaxEntries 20 fail the same way
	err = newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 20 }).ScanPath(context.Background(), &unlimitedFS{m}, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailLimit, scanKind(t, err))
}

// T02: the FTP client itself drops the "." and ".." entries an MLSD server lists (cdir / pdir), not only the scanner.
func TestWF22_R16_FTPClientDropsDotEntries(t *testing.T) {
	m := "modify=20260102030405"
	tree := map[string][]string{
		"/": {"type=cdir;" + m + "; .", "type=pdir;" + m + "; ..", "type=dir;" + m + "; music", "type=file;size=3;" + m + "; a.mkv"},
	}
	host, port := startMLSDServer(t, tree)
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "ftp",
		Settings: map[string]interface{}{"host": host, "port": port, "username": "u", "password": "p"}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	defer c.Disconnect(context.Background())
	got, err := c.ListDirectory(context.Background(), "/")
	require.NoError(t, err)
	var names []string
	for _, f := range got {
		names = append(names, f.Name)
	}
	assert.ElementsMatch(t, []string{"music", "a.mkv"}, names, "no '.' or '..' child")
}
