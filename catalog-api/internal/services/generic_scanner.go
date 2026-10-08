// generic_scanner.go - PA-03 (WP-12, tasks T361/T362, ODG-20): ONE breadth-first scanner over the filesystem client interface.
//
// It replaces the FTP, NFS and WebDAV scanner bodies that were empty (`return nil`), which let a scan of a populated root end
// "completed" with 0 files. It works for every protocol that implements filesystem.FileSystemClient, so SFTP, FTPS and NFSv3 (registered
// through filesystem.RegisterProtocol) are scanned by it with no further code.
//
// Guarantees:
//   - READ ONLY: the client is wrapped in decorators.ReadOnly; a mutating call returns ErrReadOnly and never reaches the server.
//   - HOST SAFETY: every operation takes a slot from the per-host fabric.HostBudget (concurrency + request rate, shared by every
//     scanner and protocol for the same host), and reads are retried only for positively transient failures (fabric.Retrying).
//   - BOUNDED: depth (job.MaxDepth, hard cap), total entries and entries per directory are bounded; hitting a bound FAILS the scan
//     with a reason, it never silently truncates. The depth bound is checked by listing the first directory BELOW it: content there
//     fails the scan. The entry bounds are handed to the client as a list limit (filesystem.WithListLimit) and checked after every
//     single directory, never after a whole level.
//   - HONEST: a scan that cannot list its root, that sees a transport/authentication error, that finds an empty root (unless the storage
//     root sets allow_empty), that could not read most of its sub-directories, or whose records mostly cannot be stored, returns a
//     *ScanError and the job ends `failed` with the reason - never `completed` with 0 files. A deadline is its own kind (timeout).
//   - INCREMENTAL: every entry gets a ChangeToken (mtime, size, optional etag/inode). With ScanType "incremental" and a token store,
//     unchanged entries are not re-recorded. A scanner without a token store does not claim to be incremental.
//   - CANCELLABLE with progress: the context is checked between entries and while a listing is in flight (a client that ignores the context
//     no longer holds the scan); status.CurrentPath and the counters advance as it goes.
//   - ONE PATH CONVENTION in the catalog: remote paths are slash paths relative to the root, with no leading slash ("music/b.flac"), the
//     same as the SMB and local scanners write. The scanner works internally with slash-rooted paths ("/music/b.flac") because that is what
//     the clients take; the conversion happens at the database seam (catalogPath).
//
// Revision: 2 (2026-10-08, WF22 fix round 2). Documented in docs/scripts/generic_scanner.md.
package services

import (
	"context"
	"errors"
	"fmt"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"

	"digital.vasic.filesystem/pkg/decorators"
	"digital.vasic.filesystem/pkg/fabric"

	"go.uber.org/zap"
)

// ScanFailureKind classifies why a scan failed. It is the machine-readable reason of the failed job.
type ScanFailureKind string

const (
	// FailRootUnreadable: the scan root could not be listed (unreachable host, bad path, bad credentials).
	FailRootUnreadable ScanFailureKind = "root_unreadable"
	// FailTransport: a transport or authentication error while listing below the root; the tree below it is unknown.
	FailTransport ScanFailureKind = "transport_error"
	// FailEmptyRoot: the root listed fine but holds nothing; an empty result is not a successful catalog of a populated source.
	FailEmptyRoot ScanFailureKind = "empty_root"
	// FailLimit: a bound (entries, entries per directory) was reached; the result would have been truncated.
	FailLimit ScanFailureKind = "limit_exceeded"
	// FailStorage: every record the scanner tried to store failed.
	FailStorage ScanFailureKind = "storage_failure"
	// FailCancelled: the context was cancelled.
	FailCancelled ScanFailureKind = "cancelled"
	// FailTimeout: the context deadline passed. The scan did not finish; it is a failure, not an operator cancellation.
	FailTimeout ScanFailureKind = "timeout"
	// FailPartial: more than the allowed share of the sub-directories could not be read; the catalog would miss most of the tree.
	FailPartial ScanFailureKind = "partial_scan"
)

// ctxKind classifies a finished context: a deadline is a timeout, anything else a cancellation.
func ctxKind(ctx context.Context) ScanFailureKind {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return FailTimeout
	}
	return FailCancelled
}

// ScanError is returned by GenericScanner.ScanPath when the scan did not complete. Use errors.As to read Kind.
type ScanError struct {
	Kind ScanFailureKind
	Path string
	Err  error
}

func (e *ScanError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("scan failed (%s) at %q", e.Kind, e.Path)
	}
	return fmt.Sprintf("scan failed (%s) at %q: %v", e.Kind, e.Path, e.Err)
}

// Unwrap returns the underlying error.
func (e *ScanError) Unwrap() error { return e.Err }

// ChangeToken identifies one version of an entry. Two tokens are equal exactly when the entry is considered unchanged.
type ChangeToken struct {
	MTimeNano int64
	Size      int64
	IsDir     bool
	ETag      string // WebDAV getetag, when the client can supply it
	Inode     uint64 // NFS fileid / SFTP inode, when the client can supply it
}

// String is the persisted form of the token. It is injective: the free-text field (the ETag) is quoted, so an ETag that contains ";i=5" cannot
// be mistaken for the inode field of another token (WF22 R5).
func (t ChangeToken) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "m=%d;s=%d", t.MTimeNano, t.Size)
	if t.IsDir {
		b.WriteString(";d=1")
	}
	if t.ETag != "" {
		fmt.Fprintf(&b, ";e=%q", t.ETag)
	}
	if t.Inode != 0 {
		fmt.Fprintf(&b, ";i=%d", t.Inode)
	}
	return b.String()
}

// TokenSource is an optional extension of a client: it supplies the protocol-specific parts of the token (etag, inode) that FileInfo cannot carry.
type TokenSource interface {
	ChangeTokenExtras(fi *filesystem.FileInfo) (etag string, inode uint64)
}

// NewChangeToken builds the token of fi; src may be nil.
func NewChangeToken(fi *filesystem.FileInfo, src TokenSource) ChangeToken {
	t := ChangeToken{Size: fi.Size, IsDir: fi.IsDir}
	if !fi.ModTime.IsZero() {
		t.MTimeNano = fi.ModTime.UnixNano()
	}
	if src != nil {
		t.ETag, t.Inode = src.ChangeTokenExtras(fi)
	}
	return t
}

// ChangeTokenStore persists tokens between scans of one storage root.
type ChangeTokenStore interface {
	Get(rootKey, p string) (token string, ok bool)
	Put(rootKey, p, token string)
}

// MemoryTokenStore is an in-process ChangeTokenStore. It does not survive a restart (a database-backed store is a separate work item).
type MemoryTokenStore struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemoryTokenStore creates an empty store.
func NewMemoryTokenStore() *MemoryTokenStore { return &MemoryTokenStore{m: map[string]string{}} }

// Get returns the stored token.
func (s *MemoryTokenStore) Get(rootKey, p string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[rootKey+"\x00"+p]
	return v, ok
}

// Put stores a token.
func (s *MemoryTokenStore) Put(rootKey, p, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[rootKey+"\x00"+p] = token
}

// Len returns the number of stored tokens.
func (s *MemoryTokenStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.m)
}

// RecordFunc stores one scanned entry. The default stores it in the database through insertFileRecord.
type RecordFunc func(ctx context.Context, p string, fi *filesystem.FileInfo, job ScanJob, status *ScanStatus) error

// Defaults of GenericScannerOptions.
const (
	defaultScanDepth        = 10
	defaultMaxDepthCap      = 64
	defaultMaxEntries       = 5_000_000
	defaultMaxEntriesPerDir = 1_000_000
	// defaultMaxSkippedRatio: a scan fails when more than this share of the sub-directories it tried to list could not be read. A few
	// unreadable directories are a smaller catalog (reported in the status note); most of the tree unreadable is an outage.
	defaultMaxSkippedRatio = 0.5
	// defaultMaxRecordFailRatio: a scan fails when more than this share of its records could not be stored.
	defaultMaxRecordFailRatio = 0.5
	// maxEntryNameBytes is NAME_MAX: no file system stores a longer name, so a longer one is a hostile or broken answer.
	maxEntryNameBytes = 255
)

// sharedBudgets is the process-wide per-host budget registry: the FTP, SFTP, NFS and WebDAV scanners of one host share one cap.
var sharedBudgets = fabric.NewBudgets()

// GenericScannerOptions configures a GenericScanner. The zero value of every field selects a documented default.
type GenericScannerOptions struct {
	Protocol string
	DB       *database.DB // nil: entries are only counted (unit tests, dry runs)
	Logger   *zap.Logger
	Strategy ScanStrategy
	// Workers is the number of directories listed concurrently. Default 1: most clients (FTP) hold ONE control connection.
	Workers int
	// MaxDepthCap is the hard depth ceiling regardless of job.MaxDepth (default 64).
	MaxDepthCap int
	// MaxEntries bounds the entries seen in one scan (default 5,000,000); MaxEntriesPerDir bounds one listing (default 1,000,000).
	MaxEntries       int
	MaxEntriesPerDir int
	// Budgets is the per-host budget registry (default: the process-wide one); Budget its configuration (default fabric.NASBudget()).
	Budgets *fabric.Budgets
	Budget  fabric.BudgetConfig
	// Retry is the read retry policy (default 3 attempts, 100 ms base, 2 s cap).
	Retry *fabric.RetryPolicy
	// AllowEmptyRoot lets a scan of an empty root complete. Default false: an empty root is a failure. A storage root can set the same
	// per root (models.StorageRoot.AllowEmpty, the allow_empty column); either one allows it.
	AllowEmptyRoot bool
	// SkipDirNames are directory names that are never entered or recorded (default: Synology "@eaDir", "#recycle" and "#snapshot").
	SkipDirNames []string
	// MaxSkippedRatio and MaxRecordFailRatio (defaults 0.5) are the shares of unreadable sub-directories and of unstorable records above
	// which the scan fails instead of completing with a smaller catalog.
	MaxSkippedRatio    float64
	MaxRecordFailRatio float64
	// Tokens persists change tokens for incremental scans (nil: every scan is full).
	Tokens ChangeTokenStore
	// Record stores one entry (default: insertFileRecord on DB).
	Record RecordFunc
	// BatchSize is reported through GetOptimalBatchSize.
	BatchSize int
	// Incremental is reported through SupportsIncrementalScan.
	Incremental bool
}

// GenericScanner is the protocol-agnostic breadth-first ProtocolScanner.
type GenericScanner struct {
	o GenericScannerOptions
}

var _ ProtocolScanner = (*GenericScanner)(nil)

// NewGenericScanner builds a scanner; see GenericScannerOptions for the defaults.
func NewGenericScanner(o GenericScannerOptions) *GenericScanner {
	if o.Logger == nil {
		o.Logger = zap.NewNop()
	}
	if o.Workers < 1 {
		o.Workers = 1
	}
	if o.MaxDepthCap < 1 {
		o.MaxDepthCap = defaultMaxDepthCap
	}
	if o.MaxEntries < 1 {
		o.MaxEntries = defaultMaxEntries
	}
	if o.MaxEntriesPerDir < 1 {
		o.MaxEntriesPerDir = defaultMaxEntriesPerDir
	}
	if o.Budgets == nil {
		o.Budgets = sharedBudgets
	}
	if o.Budget == (fabric.BudgetConfig{}) {
		o.Budget = fabric.NASBudget()
	}
	if o.SkipDirNames == nil {
		o.SkipDirNames = []string{"@eaDir", "#recycle", "#snapshot"}
	}
	if o.MaxSkippedRatio <= 0 {
		o.MaxSkippedRatio = defaultMaxSkippedRatio
	}
	if o.MaxRecordFailRatio <= 0 {
		o.MaxRecordFailRatio = defaultMaxRecordFailRatio
	}
	if o.BatchSize < 1 {
		o.BatchSize = 100
	}
	return &GenericScanner{o: o}
}

// WithDB returns a copy of the scanner that stores entries in db.
func (g *GenericScanner) WithDB(db *database.DB) *GenericScanner {
	o := g.o
	o.DB = db
	return &GenericScanner{o: o}
}

// WithRecorder returns a copy of the scanner that hands every entry to rec instead of the database (dry runs, evidence collection).
func (g *GenericScanner) WithRecorder(rec RecordFunc) *GenericScanner {
	o := g.o
	o.Record = rec
	return &GenericScanner{o: o}
}

// WithTokenStore returns a copy of the scanner that persists change tokens in store (enables incremental scans).
func (g *GenericScanner) WithTokenStore(store ChangeTokenStore) *GenericScanner {
	o := g.o
	o.Tokens = store
	return &GenericScanner{o: o}
}

// GetScanStrategy returns the strategy the scanner was built with; ParallelDirectories is true exactly when more than one directory is listed at a time.
func (g *GenericScanner) GetScanStrategy() ScanStrategy {
	s := g.o.Strategy
	s.ParallelDirectories = g.o.Workers > 1
	return s
}

// SupportsIncrementalScan is true only for a scanner that was built for it AND has a token store: without somewhere to keep the tokens an
// "incremental" scan re-records everything, so claiming the capability would be false (WF22 R6).
func (g *GenericScanner) SupportsIncrementalScan() bool { return g.o.Incremental && g.o.Tokens != nil }

// GetOptimalBatchSize returns the database batch size.
func (g *GenericScanner) GetOptimalBatchSize() int { return g.o.BatchSize }

func (g *GenericScanner) record(ctx context.Context, p string, fi *filesystem.FileInfo, job ScanJob, status *ScanStatus) error {
	if g.o.Record != nil {
		return g.o.Record(ctx, p, fi, job, status)
	}
	// The database seam: the catalog stores slash paths relative to the root, without a leading slash (see catalogPath).
	cp := catalogPath(p)
	e := *fi
	e.Path = cp
	return insertFileRecord(ctx, g.o.DB, cp, &e, job, status, g.o.Logger)
}

// catalogPath converts the scanner's slash-rooted path ("/music/b.flac") to the catalog's form ("music/b.flac"): the form the SMB and local
// scanners write and the form ensureDirectoryPathExists builds the parent chain in. Mixing the two forms stored every directory twice and hung
// files under phantom parents (WF22 R17).
func catalogPath(p string) string {
	return strings.TrimLeft(path.Clean("/"+p), "/")
}

// hostKey names the host a job talks to, for the budget registry.
func hostKey(job ScanJob) string {
	r := job.StorageRoot
	if r == nil {
		return ""
	}
	if r.Host != nil && *r.Host != "" {
		h := *r.Host
		if r.Port != nil && *r.Port > 0 {
			return fmt.Sprintf("%s:%d", h, *r.Port)
		}
		return h
	}
	if r.URL != nil {
		if u, err := url.Parse(*r.URL); err == nil && u.Host != "" {
			return u.Host
		}
	}
	return ""
}

// guard wraps the client: ReadOnly innermost (a mutation is refused before anything else sees it), then the host budget, then read retries.
func (g *GenericScanner) guard(c filesystem.FileSystemClient, job ScanJob) (filesystem.FileSystemClient, error) {
	out := decorators.ReadOnly(c)
	if key := hostKey(job); key != "" {
		b, err := g.o.Budgets.For(key, g.o.Budget)
		if err != nil {
			return nil, fmt.Errorf("host budget: %w", err)
		}
		out = fabric.Limited(out, b)
	}
	pol := fabric.RetryPolicy{MaxAttempts: 3, BaseDelay: 100 * time.Millisecond, MaxDelay: 2 * time.Second}
	if g.o.Retry != nil {
		pol = *g.o.Retry
	}
	return fabric.Retrying(out, pol)
}

type dirTask struct {
	p     string
	depth int
}

type listResult struct {
	t       dirTask
	entries []*filesystem.FileInfo
	err     error
}

// scanRun is the state of one ScanPath call.
type scanRun struct {
	g        *GenericScanner
	c        filesystem.FileSystemClient
	job      ScanJob
	status   *ScanStatus
	rootKey  string
	src      TokenSource
	maxDepth int

	total       int
	recordedOK  int
	recordFails int
	listed      int // sub-directory listings attempted (read or skipped), the denominator of the skipped ratio
	skippedDirs []string
	skip        map[string]bool
	incremental bool
	allowEmpty  bool
}

// ScanPath scans job.Path breadth first. See the file comment for the guarantees.
func (g *GenericScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {
	root := job.Path
	if root == "" {
		root = "/"
	}
	maxDepth := job.MaxDepth
	if maxDepth <= 0 {
		maxDepth = defaultScanDepth
	}
	if maxDepth > g.o.MaxDepthCap {
		maxDepth = g.o.MaxDepthCap
	}
	guarded, err := g.guard(client, job)
	if err != nil {
		status.incrementCounters(0, 0, 0, 0, 1)
		return &ScanError{Kind: FailRootUnreadable, Path: root, Err: err}
	}
	rootKey := ""
	if job.StorageRoot != nil {
		rootKey = job.StorageRoot.Name
	}
	rs := &scanRun{g: g, c: guarded, job: job, status: status, rootKey: rootKey, maxDepth: maxDepth, skip: map[string]bool{}}
	rs.incremental = job.ScanType == "incremental" && g.o.Tokens != nil
	rs.allowEmpty = g.o.AllowEmptyRoot || (job.StorageRoot != nil && job.StorageRoot.AllowEmpty)
	if src, ok := client.(TokenSource); ok {
		rs.src = src
	}
	for _, n := range g.o.SkipDirNames {
		rs.skip[n] = true
	}

	if err := ctx.Err(); err != nil {
		return &ScanError{Kind: ctxKind(ctx), Path: root, Err: err}
	}
	status.updateCurrentPath(root)
	entries, err := rs.list(ctx, root, rs.fetchLimit(), rs.g.o.MaxEntries-rs.total)
	if err != nil {
		status.incrementCounters(0, 0, 0, 0, 1)
		if ctx.Err() != nil {
			return &ScanError{Kind: ctxKind(ctx), Path: root, Err: ctx.Err()}
		}
		return &ScanError{Kind: FailRootUnreadable, Path: root, Err: err}
	}

	next, err := rs.processEntries(ctx, root, 0, entries)
	if err != nil {
		return err
	}
	for len(next) > 0 {
		var level []dirTask
		level, next = next, nil
		// Each wave lists at most Workers directories and is processed before the next one is listed, so the entry bounds stop the scan
		// within one wave of the bound instead of after a whole level was fetched and held in memory (WF22 R4).
		for i := 0; i < len(level); i += g.o.Workers {
			end := i + g.o.Workers
			if end > len(level) {
				end = len(level)
			}
			for _, r := range rs.listWave(ctx, level[i:end]) {
				children, err := rs.handleListing(ctx, r)
				if err != nil {
					return err
				}
				next = append(next, children...)
			}
		}
	}

	if rs.total == 0 && !rs.allowEmpty {
		return &ScanError{Kind: FailEmptyRoot, Path: root, Err: errors.New("the root lists no entries (set allow_empty on the storage root if an empty share is expected)")}
	}
	if n := rs.recordedOK + rs.recordFails; rs.recordFails > 0 && float64(rs.recordFails) > g.o.MaxRecordFailRatio*float64(n) {
		return &ScanError{Kind: FailStorage, Path: root, Err: fmt.Errorf("%d of %d records could not be stored", rs.recordFails, n)}
	}
	if k := len(rs.skippedDirs); k > 0 {
		names := rs.skippedDirs
		if len(names) > 5 {
			names = append(append([]string(nil), names[:5]...), fmt.Sprintf("... (%d more)", len(rs.skippedDirs)-5))
		}
		msg := fmt.Sprintf("%d of %d sub-directories could not be read: %s", k, rs.listed, strings.Join(names, ", "))
		if float64(k) > g.o.MaxSkippedRatio*float64(rs.listed) {
			return &ScanError{Kind: FailPartial, Path: root, Err: errors.New(msg)}
		}
		status.setNote("completed with a smaller catalog: " + msg)
	}
	return nil
}

// fetchLimit is the most entries ONE listing may return now: the smaller of the per-directory bound and what is left of the scan budget, plus one
// so that "more than the bound" stays detectable.
func (rs *scanRun) fetchLimit() int {
	lim := rs.g.o.MaxEntriesPerDir
	if r := rs.g.o.MaxEntries - rs.total; r < lim {
		lim = r
	}
	if lim < 1 {
		lim = 1
	}
	return lim + 1
}

// countable is the number of entries that count toward the scan budget: not "." / "..", a valid name, not a skipped directory.
func (rs *scanRun) countable(entries []*filesystem.FileInfo) int {
	n := 0
	for _, e := range entries {
		if e != nil && e.Name != "." && e.Name != ".." && validEntryName(e.Name) && !(e.IsDir && rs.skip[e.Name]) {
			n++
		}
	}
	return n
}

// list lists one directory through the guarded client with the fetch limit in the context. A listing is never accepted truncated:
//   - if the answer reached the budget-derived limit and the entries that COUNT exceed the remaining budget, processEntries fails the scan
//     (the total goes over MaxEntries);
//   - if the answer reached the budget-derived limit but the entries that COUNT still fit the remaining budget (the extra ones were ".", ".."
//     or skipped directories), the directory is listed once more under the per-directory limit only, so a tree that fits the budget is never
//     failed because of uncounted entries;
//   - if the limit in force is the per-directory bound, an answer that reached it holds more than the bound, which processEntries fails.
//
// It returns as soon as the context is done, even if the client ignores the context (the FTP library has no context support): the abandoned
// call finishes on its own and its answer is dropped.
func (rs *scanRun) list(ctx context.Context, p string, limit, remaining int) ([]*filesystem.FileInfo, error) {
	entries, err := rs.listOnce(ctx, p, limit)
	if err == nil && len(entries) >= limit && limit < rs.g.o.MaxEntriesPerDir+1 && rs.countable(entries) <= remaining {
		entries, err = rs.listOnce(ctx, p, rs.g.o.MaxEntriesPerDir+1)
	}
	return entries, err
}

func (rs *scanRun) listOnce(ctx context.Context, p string, limit int) ([]*filesystem.FileInfo, error) {
	type answer struct {
		entries []*filesystem.FileInfo
		err     error
	}
	ch := make(chan answer, 1)
	lctx := filesystem.WithListLimit(ctx, limit)
	go func() {
		// A call abandoned after a cancel can run into a client that was disconnected meanwhile (a nil connection in the FTP client): that must
		// end this goroutine with an error nobody reads, never bring the process down.
		defer func() {
			if r := recover(); r != nil {
				ch <- answer{nil, fmt.Errorf("listing %s failed: %v", p, r)}
			}
		}()
		e, err := rs.c.ListDirectory(lctx, p)
		ch <- answer{e, err}
	}()
	select {
	case a := <-ch:
		return a.entries, a.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// listWave lists the directories of one wave (up to Workers in flight) and returns the results in the order of tasks.
func (rs *scanRun) listWave(ctx context.Context, tasks []dirTask) []listResult {
	results := make([]listResult, len(tasks))
	limit, remaining := rs.fetchLimit(), rs.g.o.MaxEntries-rs.total
	var wg sync.WaitGroup
	for i, t := range tasks {
		i, t := i, t
		results[i].t = t
		if ctx.Err() != nil {
			results[i].err = ctx.Err()
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			rs.status.updateCurrentPath(t.p)
			results[i].entries, results[i].err = rs.list(ctx, t.p, limit, remaining)
		}()
	}
	wg.Wait()
	return results
}

// handleListing processes the result of one sub-directory listing and returns the sub-directories to list next.
func (rs *scanRun) handleListing(ctx context.Context, r listResult) ([]dirTask, error) {
	rs.listed++
	if r.err != nil {
		if ctx.Err() != nil {
			return nil, &ScanError{Kind: ctxKind(ctx), Path: r.t.p, Err: ctx.Err()}
		}
		rs.status.incrementCounters(0, 0, 0, 0, 1)
		if isBenignListError(r.err) {
			rs.g.o.Logger.Warn("directory skipped", zap.String("path", r.t.p), zap.Error(r.err))
			rs.skippedDirs = append(rs.skippedDirs, r.t.p)
			return nil, nil
		}
		return nil, &ScanError{Kind: FailTransport, Path: r.t.p, Err: r.err}
	}
	if r.t.depth > rs.maxDepth {
		// The directory sits one level below the depth bound and was listed only to find out whether the bound hides something. Content
		// there means the catalog would be incomplete: that is a failure, not a smaller result (WF22 R2).
		for _, e := range r.entries {
			if e != nil && validEntryName(e.Name) && !(e.IsDir && rs.skip[e.Name]) {
				return nil, &ScanError{Kind: FailLimit, Path: r.t.p, Err: fmt.Errorf("the tree is deeper than the depth bound %d (raise max_depth, the hard cap is %d)", rs.maxDepth, rs.g.o.MaxDepthCap)}
			}
		}
		return nil, nil
	}
	return rs.processEntries(ctx, r.t.p, r.t.depth, r.entries)
}

// validEntryName rejects names a hostile or buggy server could use to escape the directory or to poison logs and the catalog: empty, ".", "..",
// a path separator, a control character (NUL, newline, CR, ...), or more than NAME_MAX bytes. Names are otherwise stored exactly as the server
// reports them: two spellings of the same text (NFC and NFD) are two different names on the server and stay two catalog paths, because the
// catalog path is also the path the file is read back with.
func validEntryName(n string) bool {
	if n == "" || n == "." || n == ".." || len(n) > maxEntryNameBytes {
		return false
	}
	for _, r := range n {
		if r == '/' || r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// processEntries records the entries of one directory and returns the sub-directories to list next.
func (rs *scanRun) processEntries(ctx context.Context, dir string, depth int, entries []*filesystem.FileInfo) ([]dirTask, error) {
	// A client that honours the list limit returns at most perDir+1 entries, so "more than the bound" is detected here without the whole directory.
	if len(entries) > rs.g.o.MaxEntriesPerDir {
		return nil, &ScanError{Kind: FailLimit, Path: dir, Err: fmt.Errorf("%d entries exceed the per-directory bound %d", len(entries), rs.g.o.MaxEntriesPerDir)}
	}
	sorted := make([]*filesystem.FileInfo, 0, len(entries))
	for _, e := range entries {
		if e == nil {
			continue
		}
		if e.Name == "." || e.Name == ".." {
			continue // the directory itself / its parent (MLSD, some LIST servers): not children, not errors (WF22 R16)
		}
		if !validEntryName(e.Name) {
			rs.status.incrementCounters(0, 0, 0, 0, 1)
			rs.g.o.Logger.Warn("entry with an invalid name skipped", zap.String("dir", dir))
			continue
		}
		sorted = append(sorted, e)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Name < sorted[j].Name })

	var children []dirTask
	for _, e := range sorted {
		if err := ctx.Err(); err != nil {
			return nil, &ScanError{Kind: ctxKind(ctx), Path: dir, Err: err}
		}
		if e.IsDir && rs.skip[e.Name] {
			continue
		}
		rs.total++
		if rs.total > rs.g.o.MaxEntries {
			return nil, &ScanError{Kind: FailLimit, Path: dir, Err: fmt.Errorf("more than %d entries", rs.g.o.MaxEntries)}
		}
		full := path.Join(dir, e.Name)
		entry := *e
		entry.Path = full

		tok := NewChangeToken(&entry, rs.src).String()
		unchanged := false
		if rs.incremental {
			if old, ok := rs.g.o.Tokens.Get(rs.rootKey, full); ok && old == tok {
				unchanged = true
			}
		}
		if unchanged {
			rs.status.incrementCounters(1, 0, 0, 0, 0)
		} else if err := rs.g.record(ctx, full, &entry, rs.job, rs.status); err != nil {
			rs.recordFails++
			rs.status.incrementCounters(0, 0, 0, 0, 1)
			rs.g.o.Logger.Error("failed to record entry", zap.String("path", full), zap.Error(err))
		} else {
			rs.recordedOK++
			if rs.g.o.Tokens != nil {
				rs.g.o.Tokens.Put(rs.rootKey, full, tok)
			}
		}
		if e.IsDir {
			// depth+1 > maxDepth is kept as a task: it is listed once to find out whether the depth bound hides content (handleListing).
			children = append(children, dirTask{p: full, depth: depth + 1})
		}
	}
	return children, nil
}

// isBenignListError reports a failure to list ONE sub-directory that says nothing about the transport: the directory is not readable by this
// account or has vanished. Anything else (reset, timeout, EOF, authentication, an unrecognised error) is not benign and fails the scan: the
// default is to refuse, because silently skipping a subtree turns an outage into a smaller catalog.
func isBenignListError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, os.ErrPermission) || errors.Is(err, os.ErrNotExist) {
		return true
	}
	var te *textproto.Error
	if errors.As(err, &te) {
		return te.Code == 550 || te.Code == 553
	}
	low := strings.ToLower(err.Error())
	for _, m := range []string{"permission denied", "no such file", "does not exist", "not found", "status_access_denied", "access is denied", "http 403", "http 404", "status 403", "status 404"} {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// newProtocolScanner builds the generic scanner of a protocol with its declared strategy.
func newProtocolScanner(protocol string, logger *zap.Logger, strat ScanStrategy, workers, batch int) *GenericScanner {
	return NewGenericScanner(GenericScannerOptions{Protocol: protocol, Logger: logger, Strategy: strat, Workers: workers, BatchSize: batch, Incremental: true})
}

// FTPScanner, NFSScanner and WebDAVScanner are the generic scanner with their protocol's strategy. Before PA-03 each was an empty body.
type (
	// FTPScanner scans an FTP/FTPS root (one control connection, so one listing at a time).
	FTPScanner = GenericScanner
	// NFSScanner scans an NFS root (listings in parallel: the client talks to a local mount or a pipelined user-space client).
	NFSScanner = GenericScanner
	// WebDAVScanner scans a WebDAV root (PROPFIND depth 1 per directory).
	WebDAVScanner = GenericScanner
)

// NewFTPScanner creates the FTP scanner. Use WithDB to store entries in the database.
func NewFTPScanner(logger *zap.Logger) *FTPScanner {
	return newProtocolScanner("ftp", logger, ScanStrategy{BatchSize: 100, MetadataExtraction: false}, 1, 100)
}

// NewNFSScanner creates the NFS scanner. Use WithDB to store entries in the database.
func NewNFSScanner(logger *zap.Logger) *NFSScanner {
	return newProtocolScanner("nfs", logger, ScanStrategy{UseRecursiveListing: true, BatchSize: 800, MetadataExtraction: true}, 4, 800)
}

// NewWebDAVScanner creates the WebDAV scanner. Use WithDB to store entries in the database.
func NewWebDAVScanner(logger *zap.Logger) *WebDAVScanner {
	return newProtocolScanner("webdav", logger, ScanStrategy{BatchSize: 200, MetadataExtraction: true}, 1, 200)
}

// NewGenericProtocolScanner builds the scanner of a registered (pluggable) protocol such as sftp, ftps or nfs3.
func NewGenericProtocolScanner(protocol string, db *database.DB, logger *zap.Logger) *GenericScanner {
	return newProtocolScanner(protocol, logger, ScanStrategy{BatchSize: 200, MetadataExtraction: true}, 1, 200).WithDB(db)
}
