package pathguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Environment variables that configure the confinement roots. Each is a colon-separated
// list of directories; when set (non-empty) it REPLACES the built-in default of that class.
const (
	// EnvReadRoots: where the server may READ request-named paths (sync sources, upload
	// sources of a sync, conversion inputs, cover-art lookup). Default: MEDIA_ROOT_PATH (if
	// set) plus ./data.
	EnvReadRoots = "CATALOGIZER_READ_ROOTS"
	// EnvSyncRoots: where a sync may WRITE (download targets, destination_directory,
	// bidirectional sides). Everything under these roots is also readable. Default: ./data.
	EnvSyncRoots = "CATALOGIZER_SYNC_ROOTS"
	// EnvConversionRoots: where a conversion may WRITE its output. Everything under these
	// roots is also readable as a conversion input. Default: ./data.
	EnvConversionRoots = "CATALOGIZER_CONVERSION_ROOTS"
	// EnvMediaRoot is the existing media-library variable; it seeds the default read roots.
	EnvMediaRoot = "MEDIA_ROOT_PATH"
)

// DataDir is the built-in default (relative to the working directory).
var DataDir = filepath.Join(".", "data")

// ReadRoots returns the effective read roots (re-read from the environment on every call).
func ReadRoots() []string {
	defaults := []string{DataDir}
	if m := getenv(EnvMediaRoot); m != "" {
		defaults = append([]string{m}, defaults...)
	}
	return RootsFromEnv(EnvReadRoots, defaults...)
}

// SyncWriteRoots returns the effective sync write roots.
func SyncWriteRoots() []string { return RootsFromEnv(EnvSyncRoots, DataDir) }

// ConversionWriteRoots returns the effective conversion write roots.
func ConversionWriteRoots() []string { return RootsFromEnv(EnvConversionRoots, DataDir) }

// Readable returns the roots a path may be READ from: the read roots plus the given write roots.
func Readable(write []string) []string {
	out := append([]string{}, ReadRoots()...)
	return append(out, write...)
}

// Describe renders the effective roots for a single boot log line.
func Describe() string {
	return fmt.Sprintf("read=%s sync_write=%s conversion_write=%s protected=%s",
		strings.Join(ReadRoots(), ":"), strings.Join(SyncWriteRoots(), ":"), strings.Join(ConversionWriteRoots(), ":"),
		strings.Join(ProtectedPaths(), ":"))
}

// ErrProtected is returned by ConfineData when a path is, contains, or lies under a protected
// application file (the database, its SQLite companions, its key file).
var ErrProtected = errors.New("path overlaps a protected application file")

// DefaultDatabaseFile is the default SQLite database location (main.go), relative to the
// working directory. It lives inside the default write root DataDir, so it is protected even
// if the server never calls ProtectDatabase.
var DefaultDatabaseFile = filepath.Join(DataDir, "catalogizer.db")

// sqliteCompanions are the files SQLite keeps next to a database (WAL, shared memory, rollback
// journal); overwriting or deleting them corrupts or rolls back the database.
var sqliteCompanions = []string{"", "-wal", "-shm", "-journal"}

var protected struct {
	mu    sync.RWMutex
	paths []string
}

func init() { protected.paths = defaultProtected() }

func defaultProtected() []string { return databaseFiles(DefaultDatabaseFile) }

// databaseFiles returns db and its SQLite companions. When db is a symlink the companions next
// to its target are added too: which name SQLite uses for them is not assumed (review round 4).
func databaseFiles(db string) []string {
	var out []string
	for _, suf := range sqliteCompanions {
		out = append(out, db+suf)
	}
	if abs, err := filepath.Abs(db); err == nil {
		if real, err := filepath.EvalSymlinks(abs); err == nil && real != abs {
			for _, suf := range sqliteCompanions {
				out = append(out, real+suf)
			}
		}
	}
	return out
}

// Protect adds files that request-named paths may never name, contain or lie under (see
// ConfineData). Relative paths are made absolute against the working directory at call time;
// empty entries are ignored.
func Protect(paths ...string) {
	protected.mu.Lock()
	defer protected.mu.Unlock()
	for _, p := range paths {
		if p == "" {
			continue
		}
		protected.paths = append(protected.paths, p)
	}
}

// ProtectDatabase protects a SQLite database file and its companion files.
func ProtectDatabase(db string) {
	if db == "" {
		return
	}
	Protect(databaseFiles(db)...)
}

// ProtectedPaths returns the absolute form of every protected path (for logs and tests).
func ProtectedPaths() []string {
	protected.mu.RLock()
	defer protected.mu.RUnlock()
	var out []string
	for _, p := range protected.paths {
		if abs, err := filepath.Abs(p); err == nil {
			out = append(out, abs)
		}
	}
	return out
}

// SetProtected replaces the protected list and returns the previous one, so a caller that
// changes it temporarily (a test in another package) can put it back with SetProtected(prev).
func SetProtected(paths []string) (previous []string) {
	protected.mu.Lock()
	defer protected.mu.Unlock()
	previous = protected.paths
	protected.paths = append([]string(nil), paths...)
	return previous
}

// setProtectedForTest replaces the protected list and returns a function restoring it.
func setProtectedForTest(paths []string) func() {
	old := SetProtected(paths)
	return func() { SetProtected(old) }
}

// ConfineData is Confine for paths a request names as data to read or write (sync local
// paths, sync_settings directories, conversion source and target). In addition to Confine it
// refuses, with ErrProtected, a path whose resolved form equals a protected file, lies below
// one, or is a directory containing one: a sync of ./data would otherwise upload, overwrite or
// mirror-delete ./data/catalogizer.db, and a conversion could write its output over it
// (review round 3, N2). Both sides are compared after symlink resolution, so an alias of the
// database is refused too.
func ConfineData(p string, roots []string) (string, error) { return confineData(p, roots, false) }

// ConfineDataFile is ConfineData for a path used as a FILE (see ConfineFile): it must also
// resolve strictly below the root it matched, so that its Dir stays inside that root.
func ConfineDataFile(p string, roots []string) (string, error) { return confineData(p, roots, true) }

func confineData(p string, roots []string, file bool) (string, error) {
	resolved, err := confine(p, roots, file)
	if err != nil {
		return "", err
	}
	// A hard link is the same file under another name, invisible to any path comparison:
	// compare file identity with every protected file that exists.
	target, terr := os.Stat(resolved)
	for _, q := range ProtectedPaths() {
		rq, rerr := resolve(q)
		if rerr != nil {
			rq = q // resolution of the protected path failed: compare its lexical form
		}
		if within(rq, resolved) || within(resolved, rq) {
			return "", fmt.Errorf("pathguard: %q: %w", p, ErrProtected)
		}
		if terr == nil {
			if qi, err := os.Stat(rq); err == nil && os.SameFile(target, qi) {
				return "", fmt.Errorf("pathguard: %q: %w", p, ErrProtected)
			}
		}
	}
	return resolved, nil
}
