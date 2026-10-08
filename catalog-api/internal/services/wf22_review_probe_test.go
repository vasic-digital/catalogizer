package services

// WF22 independent-review probes (reviewer-authored), adopted as permanent tests.
// ADAPTATION NOTE (WF22 fix round 2): this file is the reviewer's probe adopted as a permanent test. The verbatim original is kept in
// specs/001-full-project-audit-remediation/evidence/wp12/scanner/fix-r2-reviewer-probes-verbatim/ with its RED log. Differences from the
// original are listed here and nowhere else:
//   - R3: the default stays "an empty root fails" (the owner decision of ODG-20); the probe asserts that failure now names allow_empty and that a
//     root with allow_empty=true completes, for the empty share and for the Synology-metadata-only share.
//   - R4: the original counted the whole directory (len of the server-side slice) in the list hook, a measure no per-directory stream can meet
//     (listing /a at all costs its 500 entries on the server side). The fake now honours the list limit the scanner passes
//     (filesystem.WithListLimit) and the hook counts what was RETURNED; a second probe asserts /b is never listed.
//   - R6: a production scanner must NOT claim incremental support (it has no token store); with a token store the incremental scan skips.
//   - R13: ScanPath now returns as soon as the context is done, even though the client call is still blocked (that is the fix). The abandoned call
//     finishes on its own; the probe's fake counts such calls and the test waits for them before it ends, so the package's goroutine-leak check
//     (goleak) sees a clean exit.
//   - R14: the hostile names with a control character or more than NAME_MAX bytes are now rejected (counted as errors); 5 of 8 are recorded.
// Polarity: every TestReviewProbe_* asserts the CORRECT / documented behaviour, so a FAIL here = the defect is present (RED on the reviewed code).
// Each probe carries a positive control (the instrument can see the thing) so a PASS is not a blind instrument.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/models"

	"digital.vasic.filesystem/pkg/fabric"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// R1: CoverArtService.copyRemoteFileToTemp still builds an SMB-shaped settings map for EVERY protocol; the new strict factory rejects it
// for ftp/nfs/webdav before any connection is attempted (regression: before PA-01 ftp got a working client from host/port/user/pass).
func TestReviewProbe_R1_CoverArtRemoteCopyRejectedByStrictFactory(t *testing.T) {
	s := NewCoverArtService(nil, zap.NewNop())
	s.SetClientFactory(filesystem.NewDefaultClientFactory())
	mk := func(proto string) thumbnailTask {
		return thumbnailTask{mediaItemID: 1, videoPath: "/v.mkv", protocol: proto,
			rootPath: sql.NullString{String: "/media", Valid: true}, host: sql.NullString{String: "127.0.0.1", Valid: true},
			port: sql.NullInt64{Int64: 1, Valid: true}, username: sql.NullString{String: "u", Valid: true}, password: sql.NullString{String: "p", Valid: true}}
	}
	// control: smb gets past the factory and fails at Connect (port 1 is closed) - the instrument distinguishes the two failure layers
	_, err := s.copyRemoteFileToTemp(context.Background(), mk("smb"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect filesystem client", "control: smb reaches Connect")
	for _, proto := range []string{"ftp", "nfs", "webdav"} {
		_, err := s.copyRemoteFileToTemp(context.Background(), mk(proto))
		require.Error(t, err)
		t.Logf("R1 %s -> %v", proto, err)
		assert.NotContains(t, err.Error(), "invalid "+proto+" settings", "%s thumbnail copy must not be refused by the settings contract", proto)
	}
}

func deepTree(levels int) (*memFS, string) {
	m := newMemFS()
	m.addDir("/")
	dir := "/"
	for i := 0; i < levels; i++ {
		m.addSubdir(dir, fmt.Sprintf("d%02d", i))
		dir = strings.TrimRight(dir, "/") + fmt.Sprintf("/d%02d", i)
	}
	m.addFile(dir, "deep.mkv", 1, t0)
	return m, dir + "/deep.mkv"
}

// R2: the documented guarantee "Hitting a bound FAILS the scan, it never truncates silently" (docs/scripts/generic_scanner.md, Bounded row
// names depth first). The depth bound silently truncates: the job default (scan_handler sets 10) loses everything deeper and the scan completes.
func TestReviewProbe_R2_DepthBoundIsSilentTruncation(t *testing.T) {
	m, deep := deepTree(12)
	// control: with enough depth the instrument sees the deep file
	rec := newRecorder()
	_, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 30, "full"))
	require.NoError(t, err)
	require.Contains(t, rec.sorted(), deep, "control: the deep file exists and is found with depth 30")

	m2, _ := deepTree(12)
	rec2 := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec2, nil), m2, jobOf("/", 10, "full")) // 10 = the API default (scan_handler.go)
	t.Logf("R2 depth 10: err=%v recorded=%d errorCount=%d", err, len(rec2.sorted()), st.GetSnapshot().ErrorCount)
	assert.NotContains(t, rec2.sorted(), deep)
	assert.True(t, err != nil || st.GetSnapshot().ErrorCount > 0, "a truncated tree must not end clean (doc: never truncates silently)")
}

// R3: a legitimately empty share (or a fresh Synology share holding only @eaDir / #recycle) ends FAILED through the real job path, with no way
// to configure AllowEmptyRoot from a StorageRoot or the API. Polarity: assert the share's job is not failed.
func TestReviewProbe_R3_LegitEmptyShareJobFails(t *testing.T) {
	// control: a populated share completes (the instrument reads status correctly)
	st := runJob(t, "ftp", &memFactory{client: seededTree()}, context.Background())
	require.Equal(t, "completed", st.Status)

	m := newMemFS()
	m.addDir("/")
	m.addSubdir("/", "@eaDir")
	m.addSubdir("/", "#recycle")
	// default: an empty answer is also what an unreachable source looks like, so it fails - and says how to declare the share empty
	st = runJob(t, "ftp", &memFactory{client: m}, context.Background())
	t.Logf("R3 synology-fresh share, default: status=%s reason=%q", st.Status, st.Reason)
	assert.Equal(t, "failed", st.Status)
	assert.Contains(t, st.Reason, "allow_empty")
	// per-root allow_empty: the same share completes
	st = runJobRoot(t, &memFactory{client: m}, &models.StorageRoot{Name: "root-empty", Protocol: "ftp", AllowEmpty: true})
	t.Logf("R3 synology-fresh share, allow_empty: status=%s reason=%q", st.Status, st.Reason)
	assert.Equal(t, "completed", st.Status, "a share holding only Synology metadata directories is a legitimately empty share when the root says so")
	// and a genuinely empty share
	e := newMemFS()
	e.addDir("/")
	assert.Equal(t, "completed", runJobRoot(t, &memFactory{client: e}, &models.StorageRoot{Name: "root-empty2", Protocol: "webdav", AllowEmpty: true}).Status)
}

// runJobRoot runs one job for a given storage root through processScanJob and returns the final status.
func runJobRoot(t *testing.T, factory filesystem.ClientFactory, root *models.StorageRoot) *ScanStatus {
	t.Helper()
	us := NewUniversalScanner(nil, zap.NewNop(), nil, factory)
	job := ScanJob{ID: "job-" + root.Name, Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(), StorageRoot: root}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus(job.ID)
	require.True(t, ok)
	snap := st.GetSnapshot()
	us.Stop()
	return &snap
}

// R4: bounds are checked AFTER a whole BFS level has been listed and held in memory, so MaxEntries does not bound server load or memory.
func TestReviewProbe_R4_EntryBoundCheckedAfterWholeLevelListed(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for _, d := range []string{"a", "b"} {
		m.addSubdir("/", d)
		for i := 0; i < 500; i++ {
			m.addFile("/"+d, fmt.Sprintf("f%03d", i), 1, t0)
		}
	}
	var fetched int64
	m.returnHook = func(dir string, returned int) { atomic.AddInt64(&fetched, int64(returned)) }
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 3 }), m, jobOf("/", 10, "full"))
	require.Error(t, err, "control: the bound fires")
	assert.Equal(t, FailLimit, scanKind(t, err))
	t.Logf("R4 MaxEntries=3: entries returned by the server before the bound fired = %d, listings = %v", atomic.LoadInt64(&fetched), m.listedPaths())
	assert.LessOrEqual(t, atomic.LoadInt64(&fetched), int64(3+2), "with MaxEntries 3 the scan should stop listing once the bound is reached")
}

// R4b: the bound is checked after every single directory, not after a whole BFS level: once /a blew the budget /b is never listed.
func TestReviewProbe_R4b_SecondDirectoryOfTheLevelIsNeverListed(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for _, d := range []string{"a", "b"} {
		m.addSubdir("/", d)
		for i := 0; i < 5; i++ {
			m.addFile("/"+d, fmt.Sprintf("f%03d", i), 1, t0)
		}
	}
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) { o.MaxEntries = 4 }), m, jobOf("/", 10, "full"))
	require.Error(t, err)
	assert.Equal(t, FailLimit, scanKind(t, err))
	assert.NotContains(t, m.listedPaths(), "/b", "the budget was exhausted in /a: /b must not even be listed")
}

// R5: ChangeToken.String is not injective: an ETag containing ";i=" collides with a different (etag, inode) pair -> a false "unchanged".
func TestReviewProbe_R5_ChangeTokenNotInjective(t *testing.T) {
	fi := &filesystem.FileInfo{Name: "a", Size: 1, ModTime: t0}
	a := NewChangeToken(fi, &extrasClient{etag: "x;i=5", inode: 0})
	b := NewChangeToken(fi, &extrasClient{etag: "x", inode: 5})
	require.NotEqual(t, a, b, "control: the two tokens are different values")
	t.Logf("R5 a=%q b=%q", a.String(), b.String())
	assert.NotEqual(t, a.String(), b.String(), "two different tokens must not persist as the same string")
}

// R6: production scanners advertise SupportsIncrementalScan()==true but are built without a token store (no WithTokenStore anywhere), so an
// "incremental" job re-records everything: the advertised capability is not delivered.
func TestReviewProbe_R6_ProductionIncrementalIsFull(t *testing.T) {
	us := NewUniversalScanner(nil, zap.NewNop(), nil, &memFactory{client: seededTree()})
	defer us.Stop()
	us.protocolScannersMu.RLock()
	ps := us.protocolScanners["ftp"]
	us.protocolScannersMu.RUnlock()
	assert.False(t, ps.SupportsIncrementalScan(), "a production scanner has no token store, so it must not claim incremental support")
	g := ps.(*GenericScanner)
	require.True(t, g.o.Incremental, "control: the scanner is built for incremental scans")
	rec := newRecorder()
	g2 := g.WithRecorder(rec.record).WithTokenStore(NewMemoryTokenStore())
	g2.o.Budget = fastBudget
	g2.o.Budgets = fabric.NewBudgets()
	require.True(t, g2.SupportsIncrementalScan(), "with a token store it does")
	for i := 0; i < 2; i++ {
		rec.paths = nil
		require.NoError(t, g2.ScanPath(context.Background(), seededTree(), jobOf("/", 10, "incremental"), &ScanStatus{}))
	}
	t.Logf("R6 second incremental scan re-recorded %d of 5 unchanged entries", len(rec.sorted()))
	assert.Less(t, len(rec.sorted()), 5, "an incremental scan of an unchanged tree by a scanner that claims incremental support should not re-record all")
}

// R7: a job whose context hits a DEADLINE (a timeout, i.e. the scan did not finish) is reported "cancelled", not "failed".
func TestReviewProbe_R7_TimeoutReportedAsCancelled(t *testing.T) {
	m := seededTree()
	m.listDelay = 200 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	st := runJob(t, "ftp", &memFactory{client: m}, ctx)
	t.Logf("R7 deadline: status=%s reason=%q", st.Status, st.Reason)
	require.NotEqual(t, "completed", st.Status, "control")
	assert.Equal(t, "failed", st.Status, "a timed-out scan is a failure (deadline), not an operator cancellation")
}

// R8: a cancelled job publishes EventScanFailed (event type) while its status says cancelled.
// (checked by reading publishScanEvent; no probe needed)

// R9: the WebDAV client puts the URL (including any userinfo) into its errors; ScanStatus.Reason now carries err.Error() to the API.
func TestReviewProbe_R9_ReasonLeaksURLUserinfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	secret := "S3cr3t-Pw-Probe"
	u := strings.Replace(srv.URL, "http://", "http://alice:"+secret+"@", 1)
	us := NewUniversalScanner(nil, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
	job := ScanJob{ID: "r9", Path: "/", MaxDepth: 10, ScanType: "full", Context: context.Background(),
		StorageRoot: &models.StorageRoot{Name: "r9", Protocol: "webdav", URL: &u}}
	us.processScanJob(job, 0)
	st, ok := us.GetActiveScanStatus("r9")
	require.True(t, ok)
	snap := st.GetSnapshot()
	us.Stop()
	require.Equal(t, "failed", snap.Status, "control: the job reached the listing and failed there")
	t.Logf("R9 reason contains secret: %v", strings.Contains(snap.Reason, secret))
	assert.NotContains(t, snap.Reason, secret, "the API field reason must never carry a credential (11.4.10)")
}

// R10: WebDAV entry name is taken from DAV:displayname (presentation) instead of the href (identity). A server whose displayname differs
// from the path segment makes the scanner record a non-existent path and list it; the 404 is "benign", so the subtree vanishes and the
// scan completes.
func TestReviewProbe_R10_WebDAVDisplayNameUsedAsPathSegment(t *testing.T) {
	ms := func(self string, entries ...string) string {
		return `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>` + self + `</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` + strings.Join(entries, "") + `</D:multistatus>`
	}
	dir := func(href, disp string) string {
		return `<D:response><D:href>` + href + `</D:href><D:propstat><D:prop><D:displayname>` + disp + `</D:displayname><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`
	}
	file := func(href, disp string) string {
		return `<D:response><D:href>` + href + `</D:href><D:propstat><D:prop><D:displayname>` + disp + `</D:displayname><D:getcontentlength>3</D:getcontentlength><D:resourcetype/></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			w.WriteHeader(405)
			return
		}
		switch r.URL.Path {
		case "/", "":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(ms("/", dir("/Docs/", "My Documents"), file("/top.txt", "top.txt"))))
		case "/Docs/":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(ms("/Docs/", file("/Docs/inner.txt", "inner.txt"))))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "webdav", Settings: map[string]interface{}{"url": srv.URL}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	rec := newRecorder()
	st := &ScanStatus{}
	err = newTestScanner(rec, nil).ScanPath(context.Background(), c, jobOf("/", 10, "full"), st)
	t.Logf("R10 err=%v recorded=%v errorCount=%d", err, rec.sorted(), st.GetSnapshot().ErrorCount)
	require.Contains(t, rec.sorted(), "/top.txt", "control: the client and scanner see the server")
	assert.Contains(t, rec.sorted(), "/Docs/inner.txt", "the file under the collection must be catalogued")
}

// R11: the WebDAV client strips ".." anywhere in a path (resolveURL), so a directory named like "Wait.." / "...And Justice" is listed at a
// different URL; the 404 is "benign" and the subtree silently vanishes.
func TestReviewProbe_R11_WebDAVDotDotInNameMangled(t *testing.T) {
	ms := func(self string, entries ...string) string {
		return `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>` + self + `</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>` + strings.Join(entries, "") + `</D:multistatus>`
	}
	ent := func(href string, coll bool) string {
		rt := `<D:resourcetype/>`
		if coll {
			rt = `<D:resourcetype><D:collection/></D:resourcetype>`
		}
		return `<D:response><D:href>` + href + `</D:href><D:propstat><D:prop>` + rt + `<D:getcontentlength>1</D:getcontentlength></D:prop></D:propstat></D:response>`
	}
	var seen []string
	var smu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		smu.Lock()
		seen = append(seen, r.URL.Path)
		smu.Unlock()
		switch r.URL.Path {
		case "/":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(ms("/", ent("/Wait..%20Live/", true), ent("/ok/", true))))
		case "/Wait.. Live/":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(ms("/Wait..%20Live/", ent("/Wait..%20Live/track.flac", false))))
		case "/ok/":
			w.WriteHeader(207)
			_, _ = w.Write([]byte(ms("/ok/", ent("/ok/song.flac", false))))
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "webdav", Settings: map[string]interface{}{"url": srv.URL}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	rec := newRecorder()
	err = newTestScanner(rec, nil).ScanPath(context.Background(), c, jobOf("/", 10, "full"), &ScanStatus{})
	smu.Lock()
	t.Logf("R11 err=%v recorded=%v server saw=%v", err, rec.sorted(), seen)
	smu.Unlock()
	require.Contains(t, rec.sorted(), "/ok/song.flac", "control")
	assert.Contains(t, rec.sorted(), "/Wait.. Live/track.flac", "a directory whose name contains '..' must be scanned")
}

// R12: a sub-directory 404 is "benign": if the remote volume disappears mid-scan (every remaining listing 404s) the job still ends completed.
func TestReviewProbe_R12_VolumeVanishingMidScanEndsCompleted(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for i := 0; i < 20; i++ {
		d := fmt.Sprintf("d%02d", i)
		m.addSubdir("/", d)
		m.addFile("/"+d, "f.mkv", 1, t0)
		if i > 0 {
			m.listErr["/"+d] = errors.New("WebDAV server returned status 404 for directory http://nas/" + d + "/")
		}
	}
	st := runJob(t, "webdav", &memFactory{client: m}, context.Background())
	t.Logf("R12 19/20 subtrees 404: status=%s errorCount=%d found=%d reason=%q", st.Status, st.ErrorCount, st.FilesFound, st.Reason)
	require.GreaterOrEqual(t, st.ErrorCount, int64(19), "control: the skipped listings are counted")
	assert.NotEqual(t, "completed", st.Status, "95% of the tree unreadable is an outage, not a smaller catalog")
}

// R13: a ListDirectory that ignores ctx blocks cancellation until it returns (no goroutine leak, but cancel is not prompt).
type stuckFS struct {
	*memFS
	block time.Duration
	calls int32 // listings still blocked inside the client (atomic: the abandoned call runs on a goroutine the test does not synchronise with)
	seen  int32 // listings that ever started
}

// drain waits until every listing that started has returned (bounded), so the package's goroutine-leak check sees a clean exit.
func (s *stuckFS) drain() {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) && (atomic.LoadInt32(&s.seen) == 0 || atomic.LoadInt32(&s.calls) > 0) {
		time.Sleep(10 * time.Millisecond)
	}
}

func (s *stuckFS) ListDirectory(ctx context.Context, p string) ([]*filesystem.FileInfo, error) {
	if p == "/music" {
		atomic.AddInt32(&s.seen, 1)
		atomic.AddInt32(&s.calls, 1)
		defer atomic.AddInt32(&s.calls, -1)
		time.Sleep(s.block) // ignores ctx, like a client without context support
	}
	return s.memFS.ListDirectory(context.Background(), p)
}

func TestReviewProbe_R13_CancelLatencyBoundByClient(t *testing.T) {
	before := runtime.NumGoroutine()
	s := &stuckFS{memFS: seededTree(), block: 1500 * time.Millisecond}
	t.Cleanup(s.drain)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(100 * time.Millisecond); cancel() }()
	start := time.Now()
	err := newTestScanner(newRecorder(), nil).ScanPath(ctx, s, jobOf("/", 10, "full"), &ScanStatus{})
	el := time.Since(start)
	time.Sleep(50 * time.Millisecond)
	t.Logf("R13 cancel at 100ms, ScanPath returned after %v err=%v goroutines before=%d after=%d", el, err, before, runtime.NumGoroutine())
	require.Error(t, err)
	assert.Less(t, el, 600*time.Millisecond, "cancellation should be honoured promptly")
}

// R14: hostile names that validEntryName accepts and the catalog stores verbatim: control characters, a very long name, unicode NFC vs NFD
// duplicates. Logged only (informational); asserts nothing beyond the control.
func TestReviewProbe_R14_HostileNamesAccepted(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	long := strings.Repeat("A", 70000)
	for _, n := range []string{"line\nbreak.mkv", "cr\rname.mkv", "back\\slash.mkv", long, "café.mkv", "café.mkv", " lead.mkv", "trail .mkv"} {
		m.addFile("/", n, 1, t0)
	}
	rec := newRecorder()
	st, err := scanOnce(t, newTestScanner(rec, nil), m, jobOf("/", 10, "full"))
	require.NoError(t, err)
	t.Logf("R14 recorded %d of 8 hostile/edge names, errorCount=%d (NFC and NFD 'cafe' both recorded as distinct paths)", len(rec.sorted()), st.GetSnapshot().ErrorCount)
	assert.Equal(t, 5, len(rec.sorted()), "control characters and a name over NAME_MAX are rejected; NFC/NFD spellings, leading and trailing blanks and a backslash are names")
	assert.Equal(t, int64(3), st.GetSnapshot().ErrorCount, "each rejected name is counted")
}

// R15: partial storage failure: 4 of 5 records fail, the scan returns nil (completed) - the catalog misses 80% and says completed.
func TestReviewProbe_R15_PartialStorageFailureCompletes(t *testing.T) {
	rec := newRecorder()
	n := 0
	rec.fail = func(p string) error {
		n++
		if n > 1 {
			return errors.New("db locked")
		}
		return nil
	}
	st, err := scanOnce(t, newTestScanner(rec, nil), seededTree(), jobOf("/", 10, "full"))
	t.Logf("R15 4/5 records failed: err=%v errorCount=%d", err, st.GetSnapshot().ErrorCount)
	require.Equal(t, int64(4), st.GetSnapshot().ErrorCount, "control")
	assert.Error(t, err, "losing 80% of the records should not end the scan clean")
}
