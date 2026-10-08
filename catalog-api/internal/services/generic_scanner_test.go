package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/models"

	"digital.vasic.filesystem/pkg/decorators"
	"digital.vasic.filesystem/pkg/fabric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// PA-03 unit tests of the generic breadth-first scanner. The only fake is the in-memory client (memFS): unit tests only (11.4.27).

type recorder struct {
	mu    sync.Mutex
	paths []string
	dirs  map[string]bool
	fail  func(p string) error
}

func newRecorder() *recorder { return &recorder{dirs: map[string]bool{}} }

func (r *recorder) record(_ context.Context, p string, fi *filesystem.FileInfo, _ ScanJob, st *ScanStatus) error {
	if r.fail != nil {
		if err := r.fail(p); err != nil {
			return err
		}
	}
	r.mu.Lock()
	r.paths = append(r.paths, p)
	r.dirs[p] = fi.IsDir
	r.mu.Unlock()
	st.incrementCounters(1, 1, 0, 0, 0)
	return nil
}

func (r *recorder) sorted() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// fast budget so unit tests do not wait on the NAS-class 10 req/s default.
var fastBudget = fabric.BudgetConfig{MaxConcurrent: 8, MaxReqPerSec: 1_000_000}

func newTestScanner(rec *recorder, mut func(*GenericScannerOptions)) *GenericScanner {
	o := GenericScannerOptions{
		Protocol: "memfs", Logger: zap.NewNop(), Workers: 1,
		Budgets: fabric.NewBudgets(), Budget: fastBudget,
		Retry:  &fabric.RetryPolicy{MaxAttempts: 1},
		Record: rec.record, Incremental: true,
	}
	if mut != nil {
		mut(&o)
	}
	return NewGenericScanner(o)
}

func jobOf(path string, maxDepth int, scanType string) ScanJob {
	h := "memhost"
	return ScanJob{ID: "j", Path: path, MaxDepth: maxDepth, ScanType: scanType, Context: context.Background(),
		StorageRoot: &models.StorageRoot{Name: "memroot", Protocol: "memfs", Host: &h}}
}

func scanOnce(t *testing.T, g *GenericScanner, m *memFS, job ScanJob) (*ScanStatus, error) {
	t.Helper()
	st := &ScanStatus{JobID: job.ID, Protocol: "memfs"}
	err := g.ScanPath(context.Background(), m, job, st)
	return st, err
}

func scanKind(t *testing.T, err error) ScanFailureKind {
	t.Helper()
	var se *ScanError
	require.True(t, errors.As(err, &se), "want *ScanError, got %v", err)
	return se.Kind
}

func TestGenericScanner_BreadthFirstFindsEverythingInOrder(t *testing.T) {
	rec := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec, nil), seededTree(), jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/a.mkv", "/music", "/music/album", "/music/b.flac", "/music/album/c.mp3"}, rec.sorted(),
		"entries sorted by name within a directory, directories listed level by level (breadth first)")
	assert.True(t, rec.dirs["/music"])
	assert.False(t, rec.dirs["/a.mkv"])
	assert.Equal(t, int64(5), st.GetSnapshot().FilesFound)
}

func TestGenericScanner_EmptyJobPathMeansRoot(t *testing.T) {
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), seededTree(), jobOf("", 10, "full"))
	require.NoError(t, err)
	assert.Len(t, rec.sorted(), 5)
}

func TestGenericScanner_MaxDepthFailsWhenTheBoundHidesContent(t *testing.T) {
	m := seededTree()
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 1, "full"))
	require.Error(t, err, "/music/album holds c.mp3 below depth 1: the catalog would be incomplete, so the scan fails (WF22 R2: never a silent truncation)")
	assert.Equal(t, FailLimit, scanKind(t, err))
	assert.Contains(t, err.Error(), "depth bound 1")
	assert.Equal(t, []string{"/", "/music", "/music/album"}, m.listedPaths(), "depth 1 lists the root, its sub-directories and, once, the first directory below the bound to look for content")
}

func TestGenericScanner_MaxDepthWithEmptyDirectoriesBelowTheBoundCompletes(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	m.addFile("/", "a.mkv", 1, t0)
	m.addSubdir("/", "music")
	m.addFile("/music", "b.flac", 1, t0)
	m.addSubdir("/music", "empty-leaf")
	m.addSubdir("/music", "meta-only")
	m.addSubdir("/music/meta-only", "@eaDir")
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 1, "full"))
	require.NoError(t, err, "empty directories (and ones holding only skipped metadata directories) below the bound hide nothing")
	assert.Equal(t, []string{"/a.mkv", "/music", "/music/b.flac", "/music/empty-leaf", "/music/meta-only"}, rec.sorted())
}

func TestGenericScanner_HardDepthCapFailsInsteadOfTruncating(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	dir := "/"
	for i := 0; i < 8; i++ {
		m.addSubdir(dir, fmt.Sprintf("d%d", i))
		dir = strings.TrimRight(dir, "/") + fmt.Sprintf("/d%d", i)
	}
	m.addFile(dir, "deep.mkv", 1, t0)
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, func(o *GenericScannerOptions) { o.MaxDepthCap = 3 }), m, jobOf("/", 1000, "full"))
	require.Error(t, err, "job.MaxDepth 1000 is capped at 3 and the tree is deeper: failure, not truncation")
	assert.Equal(t, FailLimit, scanKind(t, err))
	assert.Equal(t, []string{"/", "/d0", "/d0/d1", "/d0/d1/d2", "/d0/d1/d2/d3"}, m.listedPaths(), "cap 3 lists depth 0..3 and probes the directory below")
}

func TestGenericScanner_HardDepthCapBeatsJobMaxDepthOnAShallowTree(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	dir := "/"
	for i := 0; i < 3; i++ {
		m.addSubdir(dir, fmt.Sprintf("d%d", i))
		dir = strings.TrimRight(dir, "/") + fmt.Sprintf("/d%d", i)
	}
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, func(o *GenericScannerOptions) { o.MaxDepthCap = 3 }), m, jobOf("/", 1000, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/", "/d0", "/d0/d1", "/d0/d1/d2"}, m.listedPaths())
}

func TestGenericScanner_UnreadableRootFailsWithKind(t *testing.T) {
	m := seededTree()
	m.listErr["/"] = &net.OpError{Op: "read", Err: syscall.ECONNRESET}
	rec := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailRootUnreadable, scanKind(t, err))
	assert.Empty(t, rec.sorted(), "nothing recorded for a root that could not be listed")
	assert.Equal(t, int64(1), st.GetSnapshot().ErrorCount)
}

func TestGenericScanner_NonexistentRootFails(t *testing.T) {
	_, err := scanOnce(t, newTestScanner(newRecorder(), nil), seededTree(), jobOf("/nope", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailRootUnreadable, scanKind(t, err))
	assert.True(t, errors.Is(err, os.ErrNotExist), "the cause stays reachable through Unwrap")
}

func TestGenericScanner_EmptyRootFailsUnlessAllowed(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	_, err := scanOnce(t, newTestScanner(newRecorder(), nil), m, jobOf("/", 10, "full"))
	require.Error(t, err, "listing zero entries must not read as a successful scan")
	assert.Equal(t, FailEmptyRoot, scanKind(t, err))

	_, err = scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.AllowEmptyRoot = true }), m, jobOf("/", 10, "full"))
	assert.NoError(t, err)
}

func TestGenericScanner_TransportErrorBelowRootFailsTheScan(t *testing.T) {
	for name, cause := range map[string]error{
		"reset":   &net.OpError{Op: "read", Err: syscall.ECONNRESET},
		"timeout": &net.DNSError{IsTimeout: true},
		"eof":     errors.New("unexpected EOF while reading the listing"),
		"auth":    errors.New("530 Login incorrect"),
	} {
		cause := cause
		t.Run(name, func(t *testing.T) {
			m := seededTree()
			m.listErr["/music"] = cause
			_, err := scanOnce(t, newTestScanner(newRecorder(), nil), m, jobOf("/", 10, "full"))
			require.Error(t, err, "a subtree that could not be listed because the transport failed is an outage, not a smaller catalog")
			assert.Equal(t, FailTransport, scanKind(t, err))
		})
	}
}

func TestGenericScanner_BenignDirectoryErrorsAreSkippedAndCounted(t *testing.T) {
	for name, cause := range map[string]error{
		"os-permission": fmt.Errorf("list: %w", os.ErrPermission),
		"ftp-550":       &textproto.Error{Code: 550, Msg: "Permission denied"},
		"smb-denied":    errors.New("STATUS_ACCESS_DENIED"),
		"http-404":      errors.New("PROPFIND /x: http 404"),
	} {
		cause := cause
		t.Run(name, func(t *testing.T) {
			m := seededTree()
			m.addSubdir("/", "docs") // 3 sub-directory listings are attempted (music, docs, and docs/inner), 1 is unreadable: a minority
			m.addFile("/docs", "d.txt", 1, t0)
			m.addSubdir("/docs", "inner")
			m.addFile("/docs/inner", "i.txt", 1, t0)
			m.listErr["/music"] = cause
			rec := newRecorder()
			st, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 10, "full"))
			require.NoError(t, err)
			assert.ElementsMatch(t, []string{"/a.mkv", "/docs", "/docs/d.txt", "/docs/inner", "/docs/inner/i.txt", "/music"}, rec.sorted(), "the readable part is catalogued, the unreadable subtree is not")
			snap := st.GetSnapshot()
			assert.Equal(t, int64(1), snap.ErrorCount, "the skipped directory is visible in the error count")
			assert.Contains(t, snap.Reason, "1 of 3 sub-directories could not be read: /music", "and in the status note, by name")
		})
	}
}

func TestGenericScanner_UnclassifiedErrorIsNotBenign(t *testing.T) {
	m := seededTree()
	m.listErr["/music"] = errors.New("xml: unexpected element in PROPFIND response")
	_, err := scanOnce(t, newTestScanner(newRecorder(), nil), m, jobOf("/", 10, "full"))
	require.Error(t, err, "the default for an error nobody recognised is to fail, not to drop the subtree")
	assert.Equal(t, FailTransport, scanKind(t, err))
}

func TestGenericScanner_TransientFailureIsRetriedThenSucceeds(t *testing.T) {
	m := seededTree()
	var calls int32
	m.listHook = func(dir string) {
		if dir == "/music" && atomic.AddInt32(&calls, 1) == 1 {
			m.mu.Lock()
			m.listErr["/music"] = &net.OpError{Op: "read", Err: syscall.ECONNRESET}
			m.mu.Unlock()
		} else if dir == "/music" {
			m.mu.Lock()
			delete(m.listErr, "/music")
			m.mu.Unlock()
		}
	}
	rec := newRecorder()
	g := newTestScanner(rec, func(o *GenericScannerOptions) {
		o.Retry = &fabric.RetryPolicy{MaxAttempts: 3, BaseDelay: time.Millisecond}
	})
	_, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Len(t, rec.sorted(), 5, "the complete tree after one transient reset")
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(2), "the failing listing was attempted again")
}

func TestGenericScanner_AuthFailureIsNeverRetried(t *testing.T) {
	m := seededTree()
	m.listErr["/"] = errors.New("530 Login incorrect")
	g := newTestScanner(newRecorder(), func(o *GenericScannerOptions) {
		o.Retry = &fabric.RetryPolicy{MaxAttempts: 5, BaseDelay: time.Millisecond}
	})
	_, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, 1, len(m.listedPaths()), "repeating a failed login can lock the account out")
}

func TestGenericScanner_BoundsFailInsteadOfTruncating(t *testing.T) {
	t.Run("total entries", func(t *testing.T) {
		_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 3 }), seededTree(), jobOf("/", 10, "full"))
		require.Error(t, err)
		assert.Equal(t, FailLimit, scanKind(t, err))
	})
	t.Run("entries per directory", func(t *testing.T) {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < 10; i++ {
			m.addFile("/", fmt.Sprintf("f%d", i), 1, t0)
		}
		_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntriesPerDir = 9 }), m, jobOf("/", 10, "full"))
		require.Error(t, err)
		assert.Equal(t, FailLimit, scanKind(t, err))
	})
	t.Run("exactly at the bound is fine", func(t *testing.T) {
		_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 5 }), seededTree(), jobOf("/", 10, "full"))
		assert.NoError(t, err)
	})
}

func TestGenericScanner_CancellationStopsTheScan(t *testing.T) {
	m := seededTree()
	ctx, cancel := context.WithCancel(context.Background())
	m.listHook = func(dir string) {
		if dir == "/music" {
			cancel()
		}
	}
	st := &ScanStatus{}
	err := newTestScanner(newRecorder(), nil).ScanPath(ctx, m, jobOf("/", 10, "full"), st)
	require.Error(t, err)
	assert.Equal(t, FailCancelled, scanKind(t, err))
	assert.True(t, errors.Is(err, context.Canceled))
}

func TestGenericScanner_AlreadyCancelledContextListsNothing(t *testing.T) {
	m := seededTree()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := newTestScanner(newRecorder(), nil).ScanPath(ctx, m, jobOf("/", 10, "full"), &ScanStatus{})
	require.Error(t, err)
	assert.Equal(t, FailCancelled, scanKind(t, err))
	assert.Empty(t, m.listedPaths())
}

func TestGenericScanner_ReadOnlyEvenIfAScannerBugTriedToWrite(t *testing.T) {
	m := seededTree()
	g := newTestScanner(newRecorder(), nil)
	c, err := g.guard(m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	ctx := context.Background()
	for name, call := range map[string]func() error{
		"WriteFile":       func() error { return c.WriteFile(ctx, "/x", strings.NewReader("x")) },
		"DeleteFile":      func() error { return c.DeleteFile(ctx, "/x") },
		"CopyFile":        func() error { return c.CopyFile(ctx, "/x", "/y") },
		"CreateDirectory": func() error { return c.CreateDirectory(ctx, "/d") },
		"DeleteDirectory": func() error { return c.DeleteDirectory(ctx, "/d") },
	} {
		err := call()
		require.Error(t, err, name)
		assert.True(t, errors.Is(err, decorators.ErrReadOnly), name)
	}
	assert.Equal(t, int32(0), atomic.LoadInt32(&m.mutations), "no mutating call reached the inner client")

	_, err = scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&m.mutations), "a whole scan performs no mutation")
}

func TestGenericScanner_HostBudgetCapsConcurrentListings(t *testing.T) {
	build := func() *memFS {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < 8; i++ {
			m.addSubdir("/", fmt.Sprintf("d%d", i))
			m.addFile(fmt.Sprintf("/d%d", i), "f", 1, t0)
		}
		m.listDelay = 25 * time.Millisecond
		return m
	}
	run := func(budget fabric.BudgetConfig) int32 {
		m := build()
		g := newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.Workers = 8; o.Budget = budget })
		_, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
		require.NoError(t, err)
		return atomic.LoadInt32(&m.maxInflight)
	}
	assert.Equal(t, int32(1), run(fabric.BudgetConfig{MaxConcurrent: 1, MaxReqPerSec: 1_000_000}), "budget of 1 serialises 8 workers")
	assert.Equal(t, int32(3), run(fabric.BudgetConfig{MaxConcurrent: 3, MaxReqPerSec: 1_000_000}))
	assert.Greater(t, run(fabric.BudgetConfig{MaxConcurrent: 8, MaxReqPerSec: 1_000_000}), int32(3), "control: without a tight budget the workers do overlap, so the cap above is the budget's doing")
}

func TestGenericScanner_HostBudgetIsSharedAcrossScansOfOneHost(t *testing.T) {
	budgets := fabric.NewBudgets()
	var inflight, peak int32
	hook := func(string) {
		c := atomic.AddInt32(&inflight, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if c <= p || atomic.CompareAndSwapInt32(&peak, p, c) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		atomic.AddInt32(&inflight, -1)
	}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m := seededTree()
			m.listHook = hook
			g := newTestScanner(newRecorder(), func(o *GenericScannerOptions) {
				o.Budgets = budgets
				o.Budget = fabric.BudgetConfig{MaxConcurrent: 2, MaxReqPerSec: 1_000_000}
			})
			_, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
	assert.LessOrEqual(t, atomic.LoadInt32(&peak), int32(2), "three concurrent scans of host memhost never exceed the host cap of 2")
	assert.Equal(t, []string{"memhost"}, budgets.Hosts())
}

func TestGenericScanner_ParallelListingIsDeterministic(t *testing.T) {
	build := func() *memFS {
		m := newMemFS()
		m.addDir("/")
		for i := 0; i < 6; i++ {
			d := fmt.Sprintf("d%d", i)
			m.addSubdir("/", d)
			for j := 0; j < 3; j++ {
				m.addSubdir("/"+d, fmt.Sprintf("s%d", j))
				m.addFile(fmt.Sprintf("/%s/s%d", d, j), "f.bin", 1, t0)
			}
		}
		return m
	}
	var first []string
	for i := 0; i < 3; i++ {
		rec := newRecorder()
		g := newTestScanner(rec, func(o *GenericScannerOptions) { o.Workers = 4 })
		_, err := scanOnce(t, g, build(), jobOf("/", 10, "full"))
		require.NoError(t, err)
		if first == nil {
			first = rec.sorted()
			require.Len(t, first, 6+18+18)
			continue
		}
		assert.Equal(t, first, rec.sorted(), "iteration %d recorded in a different order", i)
	}
}

func TestChangeToken_EqualityAndSensitivity(t *testing.T) {
	base := &filesystem.FileInfo{Name: "a", Size: 10, ModTime: t0}
	same := *base
	assert.Equal(t, NewChangeToken(base, nil), NewChangeToken(&same, nil))
	assert.Equal(t, "m=1767323045000000000;s=10", NewChangeToken(base, nil).String())

	touched := *base
	touched.ModTime = t0.Add(time.Nanosecond)
	assert.NotEqual(t, NewChangeToken(base, nil).String(), NewChangeToken(&touched, nil).String(), "mtime")
	grown := *base
	grown.Size = 11
	assert.NotEqual(t, NewChangeToken(base, nil).String(), NewChangeToken(&grown, nil).String(), "size")
	dir := *base
	dir.IsDir = true
	assert.NotEqual(t, NewChangeToken(base, nil).String(), NewChangeToken(&dir, nil).String(), "kind")
	zero := &filesystem.FileInfo{Name: "z", Size: 1}
	assert.Equal(t, "m=0;s=1", NewChangeToken(zero, nil).String(), "an unknown mtime is 0, never 'now'")
}

type extrasClient struct {
	*memFS
	etag  string
	inode uint64
}

func (e *extrasClient) ChangeTokenExtras(*filesystem.FileInfo) (string, uint64) {
	return e.etag, e.inode
}

func TestChangeToken_UsesClientExtras(t *testing.T) {
	fi := &filesystem.FileInfo{Name: "a", Size: 1, ModTime: t0}
	var src TokenSource = &extrasClient{etag: `"abc"`, inode: 77}
	tok := NewChangeToken(fi, src)
	assert.Equal(t, `"abc"`, tok.ETag)
	assert.Equal(t, uint64(77), tok.Inode)
	assert.NotEqual(t, NewChangeToken(fi, nil).String(), tok.String())
	assert.Contains(t, tok.String(), `e=`+strconv.Quote(`"abc"`), "the etag is quoted (the token is injective, WF22 R5)")
	assert.Contains(t, tok.String(), "i=77")
	other := NewChangeToken(fi, &extrasClient{etag: `"def"`, inode: 77})
	assert.NotEqual(t, tok.String(), other.String(), "a changed etag alone is a change")
}

func TestGenericScanner_IncrementalSkipsUnchangedAndFindsChanges(t *testing.T) {
	m := seededTree()
	store := NewMemoryTokenStore()
	rec := newRecorder()
	g := newTestScanner(rec, func(o *GenericScannerOptions) { o.Tokens = store })

	_, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	require.Len(t, rec.sorted(), 5)
	require.Equal(t, 5, store.Len(), "a full scan persists one token per entry")

	rec.paths = nil
	st, err := scanOnce(t, g, m, jobOf("/", 10, "incremental"))
	require.NoError(t, err)
	assert.Empty(t, rec.sorted(), "nothing changed: nothing re-recorded")
	assert.Equal(t, int64(5), st.GetSnapshot().FilesProcessed, "every entry was still examined")

	m.setFile("/music", "b.flac", 201, t0) // grew
	m.addFile("/music/album", "new.mp3", 5, t0)
	rec.paths = nil
	_, err = scanOnce(t, g, m, jobOf("/", 10, "incremental"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/music/b.flac", "/music/album/new.mp3"}, rec.sorted(), "exactly the grown file and the new file")

	rec.paths = nil
	_, err = scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Len(t, rec.sorted(), 6, "a full scan re-records everything")
}

func TestGenericScanner_IncrementalWithoutStoreIsFull(t *testing.T) {
	rec := newRecorder()
	g := newTestScanner(rec, nil)
	for i := 0; i < 2; i++ {
		rec.paths = nil
		_, err := scanOnce(t, g, seededTree(), jobOf("/", 10, "incremental"))
		require.NoError(t, err)
		assert.Len(t, rec.sorted(), 5)
	}
}

func TestGenericScanner_FailedRecordIsNotRememberedAsDone(t *testing.T) {
	m := seededTree()
	store := NewMemoryTokenStore()
	rec := newRecorder()
	rec.fail = func(p string) error {
		if p == "/music/b.flac" {
			return errors.New("db busy")
		}
		return nil
	}
	g := newTestScanner(rec, func(o *GenericScannerOptions) { o.Tokens = store })
	st, err := scanOnce(t, g, m, jobOf("/", 10, "full"))
	require.NoError(t, err, "a partial storage failure completes with errors counted")
	assert.Equal(t, int64(1), st.GetSnapshot().ErrorCount)

	rec.fail = nil
	rec.paths = nil
	_, err = scanOnce(t, g, m, jobOf("/", 10, "incremental"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/music/b.flac"}, rec.sorted(), "the entry whose record failed is retried by the next incremental scan")
}

func TestGenericScanner_AllRecordsFailingFailsTheScan(t *testing.T) {
	rec := newRecorder()
	rec.fail = func(string) error { return errors.New("db down") }
	_, err := scanOnce(t, newTestScanner(rec, nil), seededTree(), jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailStorage, scanKind(t, err))
}

func TestGenericScanner_HostileEntryNamesAreNotFollowed(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for _, n := range []string{"..", ".", "", "a/b", "x\x00y"} {
		m.addSubdir("/", n)
	}
	m.addFile("/", "ok.txt", 1, t0)
	rec := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/ok.txt"}, rec.sorted())
	assert.Equal(t, int64(3), st.GetSnapshot().ErrorCount, `"" , "a/b" and NUL are hostile names (counted); "." and ".." are the directory itself and its parent: dropped silently (WF22 R16)`)
	assert.Equal(t, []string{"/"}, m.listedPaths(), "no listing was issued for a hostile name")
}

func TestGenericScanner_SkipsSynologyMetadataDirectories(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	m.addSubdir("/", "@eaDir")
	m.addFile("/@eaDir", "thumb.jpg", 1, t0)
	m.addSubdir("/", "#recycle")
	m.addSubdir("/", "real")
	m.addFile("/real", "f", 1, t0)
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/real", "/real/f"}, rec.sorted())
	assert.NotContains(t, m.listedPaths(), "/@eaDir")

	rec2 := newRecorder()
	m2 := newMemFS()
	m2.addDir("/")
	m2.addSubdir("/", "@eaDir")
	_, err = scanOnce(t, newTestScanner(rec2, func(o *GenericScannerOptions) { o.SkipDirNames = []string{} }), m2, jobOf("/", 10, "full"))
	require.NoError(t, err)
	assert.Equal(t, []string{"/@eaDir"}, rec2.sorted(), "an explicit empty skip list scans everything")
}

func TestGenericScanner_ProgressIsReported(t *testing.T) {
	m := seededTree()
	var seen []string
	var mu sync.Mutex
	st := &ScanStatus{}
	m.listHook = func(string) {
		mu.Lock()
		seen = append(seen, st.GetSnapshot().CurrentPath)
		mu.Unlock()
	}
	require.NoError(t, newTestScanner(newRecorder(), nil).ScanPath(context.Background(), m, jobOf("/", 10, "full"), st))
	mu.Lock()
	defer mu.Unlock()
	assert.Contains(t, seen, "/music", "CurrentPath advances to the directory being listed")
	assert.Contains(t, seen, "/music/album")
	assert.Equal(t, int64(5), st.GetSnapshot().FilesFound)
}

func TestHostKey(t *testing.T) {
	s := func(v string) *string { return &v }
	p := 2121
	cases := []struct {
		name string
		root *models.StorageRoot
		want string
	}{
		{"nil root", nil, ""},
		{"host", &models.StorageRoot{Host: s("nas")}, "nas"},
		{"host port", &models.StorageRoot{Host: s("nas"), Port: &p}, "nas:2121"},
		{"url", &models.StorageRoot{URL: s("https://dav.example:8443/x")}, "dav.example:8443"},
		{"nothing", &models.StorageRoot{}, ""},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, hostKey(ScanJob{StorageRoot: c.root}), c.name)
	}
}

func TestIsBenignListError(t *testing.T) {
	benign := []error{os.ErrPermission, os.ErrNotExist, &textproto.Error{Code: 550}, &textproto.Error{Code: 553}, errors.New("Permission denied"), errors.New("STATUS_ACCESS_DENIED"), errors.New("no such file or directory")}
	for _, e := range benign {
		assert.True(t, isBenignListError(e), "%v", e)
	}
	fatal := []error{nil, context.Canceled, context.DeadlineExceeded, &textproto.Error{Code: 530}, &textproto.Error{Code: 421}, syscall.ECONNRESET, errors.New("boom"), errors.New("EOF"), errors.New("authentication failed")}
	for _, e := range fatal {
		assert.False(t, isBenignListError(e), "%v", e)
	}
}

func TestScanFailureKindsAreDistinct(t *testing.T) {
	seen := map[ScanFailureKind]bool{}
	for _, k := range []ScanFailureKind{FailRootUnreadable, FailTransport, FailEmptyRoot, FailLimit, FailStorage, FailCancelled, FailTimeout, FailPartial} {
		assert.NotEmpty(t, string(k))
		assert.False(t, seen[k])
		seen[k] = true
	}
	e := &ScanError{Kind: FailEmptyRoot, Path: "/r"}
	assert.Contains(t, e.Error(), "empty_root")
	assert.Nil(t, e.Unwrap())
}

func TestPluggableScannerStrategyFollowsWorkers(t *testing.T) {
	assert.False(t, NewFTPScanner(zap.NewNop()).GetScanStrategy().ParallelDirectories, "FTP has one control connection")
	assert.True(t, NewNFSScanner(zap.NewNop()).GetScanStrategy().ParallelDirectories)
	assert.Equal(t, 200, NewGenericProtocolScanner("sftp", nil, zap.NewNop()).GetOptimalBatchSize())
	assert.False(t, NewGenericProtocolScanner("sftp", nil, zap.NewNop()).SupportsIncrementalScan(), "no token store: no incremental claim (WF22 R6)")
	assert.True(t, NewGenericProtocolScanner("sftp", nil, zap.NewNop()).WithTokenStore(NewMemoryTokenStore()).SupportsIncrementalScan())
}
