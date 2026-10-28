package database

// CAT-001: encryption-at-rest for the main SQLite database via SQLCipher.
//
// Key handling (owner decision, binding):
//   - 32 random bytes (crypto/rand), hex-encoded, generated on first run;
//   - stored in a file of mode 0600 OUTSIDE the repository, at a
//     config-injected path (config.DatabaseConfig.EncryptionKeyFile, else
//     $DATABASE_ENCRYPTION_KEY_FILE, else <user config dir>/catalogizer/db.key);
//   - never logged, never part of an error message, never in the DSN of a
//     log line;
//   - fail closed: unreadable / wrongly-permissioned / malformed / wrong key
//     => NewConnection returns an error, never falls back to plaintext.
//     The ONLY opt-out is an in-memory database (":memory:").
//
// The key is applied by the driver on EVERY pooled connection through the
// DSN parameter _pragma_key (go-sqlcipher runs "PRAGMA key" inside Open()).

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"catalogizer/config"
)

const (
	// EncryptionKeyFileEnv names the environment variable that injects the key
	// file path when config.DatabaseConfig.EncryptionKeyFile is empty.
	EncryptionKeyFileEnv = "DATABASE_ENCRYPTION_KEY_FILE"

	sqlitePlainHeader = "SQLite format 3\x00"
	keyBytes          = 32
)

type dbFileState int

const (
	dbMissing dbFileState = iota // no file, or zero bytes
	dbPlaintext
	dbEncrypted // anything else: assumed SQLCipher (verified on open)
)

// isMemoryPath reports whether path denotes an in-memory SQLite database,
// the single explicit opt-out from encryption.
func isMemoryPath(path string) bool {
	p := strings.TrimSpace(path)
	if p == ":memory:" || p == "" {
		return p == ":memory:"
	}
	return strings.HasPrefix(p, "file::memory:") || strings.Contains(p, "mode=memory")
}

// ResolveKeyFilePath returns the key file location: config, then env, then the
// per-user config directory. It never returns a path inside the working tree
// unless the operator injected one.
func ResolveKeyFilePath(cfg *config.DatabaseConfig) (string, error) {
	if cfg != nil && strings.TrimSpace(cfg.EncryptionKeyFile) != "" {
		return filepath.Clean(cfg.EncryptionKeyFile), nil
	}
	if v := strings.TrimSpace(os.Getenv(EncryptionKeyFileEnv)); v != "" {
		return filepath.Clean(v), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine a default database key location (set %s or database.encryption_key_file): %w", EncryptionKeyFileEnv, err)
	}
	return filepath.Join(dir, "catalogizer", "db.key"), nil
}

func inspectDBFile(path string) (dbFileState, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return dbMissing, nil
		}
		return 0, fmt.Errorf("cannot inspect database file: %w", err)
	}
	defer f.Close()
	buf := make([]byte, len(sqlitePlainHeader))
	n, err := io.ReadFull(f, buf)
	if n == 0 && (err == io.EOF || err == nil) {
		return dbMissing, nil
	}
	if n == len(sqlitePlainHeader) && string(buf) == sqlitePlainHeader {
		return dbPlaintext, nil
	}
	return dbEncrypted, nil
}

func validKeyHex(s string) bool {
	if len(s) != keyBytes*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// loadKey reads and validates the key file. Fails closed. The file is opened
// once (O_NOFOLLOW on POSIX) and every check is made on that descriptor, so
// the file that was checked is the file that is read.
func loadKey(path string) (string, error) {
	f, err := openKeyFileNoFollow(path)
	if err != nil {
		if errors.Is(err, errKeySymlink) {
			return "", fmt.Errorf("database key file %q must be a regular file (not a symlink)", path)
		}
		return "", fmt.Errorf("database key file %q: %w", path, err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("database key file %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("database key file %q must be a regular file (not a symlink)", path)
	}
	if keyPermissionsEnforced() {
		if info.Mode().Perm()&0o077 != 0 {
			return "", fmt.Errorf("database key file %q has insecure permission %04o: group/world access is not allowed (chmod 600)", path, info.Mode().Perm())
		}
		if err := checkKeyFileOwner(info); err != nil {
			return "", err
		}
		if err := checkKeyDir(filepath.Dir(path)); err != nil {
			return "", err
		}
	}
	raw, err := io.ReadAll(io.LimitReader(f, 4096))
	if err != nil {
		return "", fmt.Errorf("database key file %q is not readable: %w", path, err)
	}
	k := strings.TrimSpace(string(raw))
	if !validKeyHex(k) {
		return "", fmt.Errorf("database key file %q is malformed: expected %d hex characters", path, keyBytes*2)
	}
	return strings.ToLower(k), nil
}

func newKey() (string, error) {
	b := make([]byte, keyBytes)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("cannot read system randomness: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// writeKeyFileExclusive atomically publishes key at path with mode 0600 and
// fails with os.ErrExist when path already exists (so two racing processes can
// never both win). The temp file is fsynced before it is linked into place.
func writeKeyFileExclusive(path, key string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("cannot create database key directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".dbkey-*")
	if err != nil {
		return fmt.Errorf("cannot create database key file: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpName) }
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if _, err := tmp.WriteString(key + "\n"); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Link(tmpName, path); err != nil {
		cleanup()
		if errors.Is(err, os.ErrExist) {
			return os.ErrExist
		}
		return fmt.Errorf("cannot publish database key file: %w", err)
	}
	cleanup()
	syncDir(dir)
	return nil
}

// replaceKeyFile atomically overwrites path with key (mode 0600).
func replaceKeyFile(path, key string) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dbkey-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tmp.Chmod(0o600); err == nil {
		_, err = tmp.WriteString(key + "\n")
		if err == nil {
			err = tmp.Sync()
		}
		if cerr := tmp.Close(); err == nil {
			err = cerr
		}
		if err == nil {
			err = os.Rename(tmpName, path)
		}
		if err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		syncDir(dir)
		return nil
	}
	tmp.Close()
	_ = os.Remove(tmpName)
	return errors.New("cannot set key file permissions")
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		d.Close()
	}
}

// loadOrCreateKey returns the existing key, or (only when create is true)
// generates and persists one.
func loadOrCreateKey(path string, create bool) (string, error) {
	k, err := loadKey(path)
	if err == nil {
		return k, nil
	}
	if !create || !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	nk, gerr := newKey()
	if gerr != nil {
		return "", gerr
	}
	switch werr := writeKeyFileExclusive(path, nk); {
	case werr == nil:
		return nk, nil
	case errors.Is(werr, os.ErrExist): // lost the race: use the winner's key
		return loadKey(path)
	default:
		return "", werr
	}
}

// keyedDSN builds the driver DSN for a file database with the raw key applied
// on every connection the pool opens.
func keyedDSN(path, keyHex string, params string) string {
	return path + "?" + params + "&_pragma_key=" + url.QueryEscape("x'"+keyHex+"'")
}

// verifyKeyed proves the connection really is a SQLCipher database readable
// with the supplied key. It fails closed if the cipher is absent (a plain
// SQLite driver would silently ignore the key) or the key is wrong.
func verifyKeyed(db *sql.DB) error {
	var ver string
	if err := db.QueryRow(`PRAGMA cipher_version`).Scan(&ver); err != nil || strings.TrimSpace(ver) == "" {
		return errors.New("SQLCipher is not active in this build; refusing to open a file database unencrypted")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&n); err != nil {
		return errors.New("database cannot be decrypted with the configured key (wrong key or corrupted file)")
	}
	return nil
}

// prepareSQLiteKey decides what to do with a file database before the pool is
// opened: create a key for a brand-new DB, migrate a plaintext DB, or load the
// existing key for an encrypted DB. Returns the key as hex.
func prepareSQLiteKey(cfg *config.DatabaseConfig) (string, error) {
	keyPath, err := ResolveKeyFilePath(cfg)
	if err != nil {
		return "", err
	}
	state, err := inspectDBFile(cfg.Path)
	if err != nil {
		return "", err
	}
	switch state {
	case dbMissing:
		return loadOrCreateKey(keyPath, true)
	case dbPlaintext:
		key, err := loadOrCreateKey(keyPath, true)
		if err != nil {
			return "", err
		}
		if err := migratePlaintextToEncrypted(cfg.Path, key); err != nil {
			return "", fmt.Errorf("plaintext to encrypted migration failed (original data left intact; its journal mode may have been switched from WAL to DELETE, the server restores WAL when it opens the database): %w", err)
		}
		return key, nil
	default:
		k, err := loadOrCreateKey(keyPath, false)
		if err != nil {
			return "", fmt.Errorf("%w%s", err, rekeyHint(keyPath))
		}
		return k, nil
	}
}

// rekeyHint explains a leftover <keyfile>.new, which means a previous rekey
// was interrupted: the database may already be encrypted with that key.
func rekeyHint(keyPath string) string {
	if _, err := os.Lstat(keyPath + ".new"); err == nil {
		return fmt.Sprintf(" (a previous rekey did not finish: %s exists and may hold the key the database is now encrypted with; see docs/DATABASE_ENCRYPTION.md)", keyPath+".new")
	}
	return ""
}

func sidecars(path string) []string { return []string{path + "-wal", path + "-shm"} }

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// tableRowCounts returns row counts of every ordinary table.
func tableRowCounts(db *sql.DB) (map[string]int64, error) {
	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, n)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	sort.Strings(names)
	out := make(map[string]int64, len(names))
	for _, n := range names {
		var c int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM ` + quoteIdent(n)).Scan(&c); err != nil {
			return nil, err
		}
		out[n] = c
	}
	return out, nil
}

func equalCounts(a, b map[string]int64) error {
	if len(a) != len(b) {
		return fmt.Errorf("table count differs: %d vs %d", len(a), len(b))
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return fmt.Errorf("row count differs for table %q: %d vs %d", k, v, w)
		}
	}
	return nil
}

func timestamp() string { return time.Now().UTC().Format("20060102T150405Z") }

// uniquePath returns base+suffix, adding a counter if it already exists.
func uniquePath(base string) string {
	p := base
	for i := 1; ; i++ {
		if _, err := os.Lstat(p); os.IsNotExist(err) {
			return p
		}
		p = fmt.Sprintf("%s.%d", base, i)
	}
}

// lockBusyMS is the busy timeout (milliseconds) used when trying to take the
// exclusive lock for rekey/restore. Overridable in tests.
var lockBusyMS = 5000

// Test seams (nil in production): called after the export / after the rekey so
// tests can corrupt the result and prove the verification refuses it.
var (
	afterExportHook func(tmpPath, keyHex string)
	afterRekeyHook  func(dbPath, newKeyHex string)
	beforeSwapHook  func()
)

// exportEncryptedCopy writes a full SQLCipher copy of the open database db
// into the NEW file dest (created 0600), encrypted with keyHex, preserving
// user_version. Uses sqlcipher_export: a clean, consistent snapshot that needs
// no -wal/-shm sidecars.
func exportEncryptedCopy(db *sql.DB, dest, keyHex string, userVersion int64) error {
	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	attach := fmt.Sprintf(`ATTACH DATABASE '%s' AS encrypted KEY "x'%s'"`, strings.ReplaceAll(dest, "'", "''"), keyHex)
	if _, err := db.Exec(attach); err != nil {
		return fmt.Errorf("attach encrypted target: %w", scrubDSN(err))
	}
	if _, err := db.Exec(`SELECT sqlcipher_export('encrypted')`); err != nil {
		_, _ = db.Exec(`DETACH DATABASE encrypted`)
		return fmt.Errorf("sqlcipher_export: %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA encrypted.user_version = %d`, userVersion)); err != nil {
		_, _ = db.Exec(`DETACH DATABASE encrypted`)
		return fmt.Errorf("copy user_version: %w", err)
	}
	if _, err := db.Exec(`DETACH DATABASE encrypted`); err != nil {
		return err
	}
	return nil
}

// verifyEncryptedCopy opens path with keyHex and proves it is a readable
// SQLCipher database with exactly the expected per-table row counts and
// user_version. It leaves no sidecars behind.
func verifyEncryptedCopy(path, keyHex string, wantCounts map[string]int64, wantUV int64) error {
	dst, err := sql.Open("sqlite3", keyedDSN(path, keyHex, "_busy_timeout=30000"))
	if err != nil {
		return scrubDSN(err)
	}
	dst.SetMaxOpenConns(1)
	defer dst.Close()
	if err := verifyKeyed(dst); err != nil {
		return fmt.Errorf("verify encrypted copy: %w", err)
	}
	got, err := tableRowCounts(dst)
	if err != nil {
		return fmt.Errorf("count encrypted rows: %w", err)
	}
	var uv int64
	if err := dst.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		return err
	}
	if _, err := dst.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return err
	}
	if err := equalCounts(wantCounts, got); err != nil {
		return fmt.Errorf("row-count verification failed: %w", err)
	}
	if uv != wantUV {
		return fmt.Errorf("user_version mismatch: %d vs %d", wantUV, uv)
	}
	return nil
}

// requireDeleteMode reports an error unless the journal mode reported by
// "PRAGMA journal_mode=DELETE" is DELETE. On the bundled SQLite 3.20.1 the
// pragma itself fails with "database is locked" while another connection has a
// WAL database open, so this branch is defence in depth for engines that
// return the unchanged mode (e.g. "wal") instead of an error.
func requireDeleteMode(mode string) error {
	if !strings.EqualFold(mode, "delete") {
		return fmt.Errorf("journal mode stayed %q", mode)
	}
	return nil
}

// acquireExclusive makes the (single) connection of db the only user of the
// database for as long as it stays open: it switches to exclusive locking,
// leaves WAL mode (which fails while any other connection - e.g. a running
// server's pool - has the database open) and takes the lock. Returns an
// "in use" error when another process holds the database.
func acquireExclusive(db *sql.DB) error {
	inUse := func(err error) error {
		return fmt.Errorf("database is in use by another process (stop the server first): %v", err)
	}
	if _, err := db.Exec(`PRAGMA locking_mode=EXCLUSIVE`); err != nil {
		return inUse(err)
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode=DELETE`).Scan(&mode); err != nil {
		return inUse(err)
	}
	if err := requireDeleteMode(mode); err != nil {
		return inUse(err)
	}
	if _, err := db.Exec(`BEGIN EXCLUSIVE`); err != nil {
		return inUse(err)
	}
	if _, err := db.Exec(`COMMIT`); err != nil { // exclusive locking_mode keeps the lock
		return inUse(err)
	}
	return nil
}

// hostOS is the platform the file-handling decisions are made for; a variable
// so tests can exercise the Windows branch.
var hostOS = runtime.GOOS

// closeBeforeRename reports whether the exclusive lock (an open handle) must
// be released before a file is renamed over the database. The bundled SQLite
// opens files on Windows without FILE_SHARE_DELETE, so renaming over (or
// hard-linking) a file with an open handle fails there; elsewhere the lock is
// held through the rename. On Windows a small unlocked window remains.
func closeBeforeRename(goos string) bool { return goos == "windows" }

// classifyProbeError turns a failed read of a probed database into the probe
// result: busy/locked = in use (fail closed); NOTADB = a file this key cannot
// open, so not a server using it; anything else = cannot determine (refuse).
func classifyProbeError(err error) (func(), error) {
	switch {
	case isBusyErr(err):
		return nil, fmt.Errorf("database is in use by another process (stop the server first): %v", err)
	case isNotADBErr(err):
		return func() {}, nil
	default:
		return nil, fmt.Errorf("cannot determine whether the database is in use: %v", scrubDSN(err))
	}
}

// isBusyErr reports whether err is SQLite's "database is locked/busy".
func isBusyErr(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "locked") || strings.Contains(m, "busy")
}

// isNotADBErr reports SQLite's NOTADB error: what a wrong key (or a plaintext
// file) produces on the first page read.
func isNotADBErr(err error) bool {
	m := strings.ToLower(err.Error())
	return strings.Contains(m, "not a database") || strings.Contains(m, "encrypted")
}

// probeInUse takes the exclusive lock on the database at path and returns a
// release function; the caller keeps the lock for as long as it needs the
// guarantee that nobody else opens the file (call release when done). It
// FAILS CLOSED: an error is returned when another connection holds or uses
// the database, when the lock is busy, or when the state cannot be
// determined. Only a genuine decrypt failure (NOTADB: wrong key or plaintext
// file, on a database nobody holds) means "cannot be a server using this
// key" and returns a no-op release with a nil error.
// Limitation: a server that currently holds NO connection (idle pool) cannot
// be detected - see docs/DATABASE_ENCRYPTION.md.
func probeInUse(path, keyHex string) (func(), error) {
	db, err := sql.Open("sqlite3", keyedDSN(path, keyHex, fmt.Sprintf("_busy_timeout=%d", lockBusyMS)))
	if err != nil {
		return nil, scrubDSN(err)
	}
	db.SetMaxOpenConns(1)
	var ver string
	if err := db.QueryRow(`PRAGMA cipher_version`).Scan(&ver); err != nil {
		db.Close()
		if isBusyErr(err) {
			return nil, fmt.Errorf("database is in use by another process (stop the server first): %v", scrubDSN(err))
		}
		// The underlying error (a directory at the path, a missing parent,
		// a permission problem) is reported as it is, never as a missing cipher.
		return nil, fmt.Errorf("cannot open the database to probe it: %v", scrubDSN(err))
	}
	if strings.TrimSpace(ver) == "" {
		db.Close()
		return nil, errors.New("SQLCipher is not active in this build; cannot probe the database")
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&n); err != nil {
		db.Close()
		return classifyProbeError(err)
	}
	if err := acquireExclusive(db); err != nil {
		db.Close()
		return nil, err
	}
	return func() { _ = db.Close() }, nil
}

// linkOrExplain hard-links src to dst (so both names exist at all times).
func linkOrExplain(src, dst string) error {
	if err := os.Link(src, dst); err != nil {
		if errors.Is(err, os.ErrExist) {
			return err
		}
		return fmt.Errorf("cannot create a hard link %s -> %s (the filesystem may not support hard links; move the database to a filesystem that does, or copy it aside manually and retry): %w", src, dst, err)
	}
	return nil
}

// migratePlaintextToEncrypted converts path (plaintext SQLite) into an
// encrypted database using sqlcipher_export. Safe sequence:
//  1. checkpoint+export into path.encrypting-<ts> (original data unchanged);
//  2. open the new file with the key and verify identical per-table row counts
//     and user_version;
//  3. hard-link the plaintext original to path.plaintext-backup-<ts> (0600) -
//     kept, never deleted; the database name never disappears;
//  4. atomically rename the encrypted file over path.
//
// On any failure before step 3 the original data is intact (its journal mode may have been switched from WAL to DELETE); the temp file is
// removed.
func migratePlaintextToEncrypted(path, keyHex string) (err error) {
	ts := timestamp()
	tmpPath := uniquePath(path + ".encrypting-" + ts)
	defer func() {
		if err != nil {
			_ = os.Remove(tmpPath)
			for _, s := range sidecars(tmpPath) {
				_ = os.Remove(s)
			}
		}
	}()

	src, err := sql.Open("sqlite3", fmt.Sprintf("%s?_busy_timeout=%d", path, lockBusyMS))
	if err != nil {
		return err
	}
	src.SetMaxOpenConns(1)
	// The source stays exclusively locked (and out of WAL mode, which also
	// folds the WAL into the main file) from before the row counts until the
	// encrypted file has been renamed over it: a commit by anyone else is
	// refused instead of being silently missing from the encrypted copy. If
	// another process has the database open, the migration refuses.
	defer src.Close()
	if err = acquireExclusive(src); err != nil {
		return fmt.Errorf("plaintext database cannot be locked for migration: %w", err)
	}
	srcCounts, err := tableRowCounts(src)
	if err != nil {
		return fmt.Errorf("count source rows: %w", err)
	}
	var userVersion int64
	if err = src.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		return err
	}
	if err = exportEncryptedCopy(src, tmpPath, keyHex, userVersion); err != nil {
		return err
	}
	if afterExportHook != nil {
		afterExportHook(tmpPath, keyHex)
	}
	if err = verifyEncryptedCopy(tmpPath, keyHex, srcCounts, userVersion); err != nil {
		return err
	}
	for _, s := range sidecars(tmpPath) { // none expected after checkpoint
		_ = os.Remove(s)
	}
	if err = os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}
	// A leftover plaintext -wal/-shm would be attached to the new file.
	for _, s := range sidecars(path) {
		if st, serr := os.Stat(s); serr == nil && st.Size() > 0 {
			return fmt.Errorf("plaintext database still has unmerged %s; stop other users of the database and retry", filepath.Base(s))
		}
		_ = os.Remove(s)
	}

	if closeBeforeRename(hostOS) {
		_ = src.Close() // release the lock: Windows cannot link/rename over an open file
	}
	if beforeSwapHook != nil {
		beforeSwapHook()
	}
	// Swap: keep the plaintext original as a (0600) backup via a hard link
	// (the database name never disappears), then atomically replace it.
	bak := uniquePath(path + ".plaintext-backup-" + ts)
	if err = linkOrExplain(path, bak); err != nil {
		return fmt.Errorf("keep plaintext backup: %w", err)
	}
	_ = os.Chmod(bak, 0o600)
	if err = os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(bak) // path still names the original; nothing was lost
		return fmt.Errorf("install encrypted database: %w", err)
	}
	syncDir(filepath.Dir(path))
	return nil
}

// copyFile copies src to dst (0600), fsyncing dst. dst must not exist.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		_ = os.Remove(dst)
		return err
	}
	return out.Close()
}

// copyDBSet copies the main database file plus its -wal/-shm sidecars (when
// present and non-empty) to dest, dest-wal, dest-shm. Backups made by copying
// only the main file silently lose every commit still living in the WAL.
// It returns the files THIS call created, so a caller can clean up exactly
// those and never a pre-existing file.
func copyDBSet(srcPath, dest string) (created []string, err error) {
	if err := copyFile(srcPath, dest); err != nil {
		return created, err
	}
	created = append(created, dest)
	for _, s := range []string{"-wal", "-shm"} {
		st, serr := os.Stat(srcPath + s)
		if serr != nil || st.Size() == 0 {
			continue
		}
		if err := copyFile(srcPath+s, dest+s); err != nil {
			return created, err
		}
		created = append(created, dest+s)
	}
	return created, nil
}

func removeAll(paths []string) {
	for _, p := range paths {
		_ = os.Remove(p)
	}
}

// BackupTo writes a consistent, still-encrypted copy of the database to dest
// (mode 0600) including the WAL/-shm sidecars when a reader prevented the WAL
// from being fully checkpointed. dest must not exist: an existing file is
// never overwritten or deleted. The key is NOT part of the backup: back the
// key file up separately (docs/DATABASE_ENCRYPTION.md).
func (db *DB) BackupTo(dest string) error {
	if db.DatabaseType() != "sqlite" {
		return errors.New("backup is supported only for SQLite databases")
	}
	src := db.DBPath()
	if src == "" || isMemoryPath(src) {
		return errors.New("no file-backed database to back up")
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("backup destination %q already exists; refusing to overwrite", dest)
	}
	// Best-effort, non-blocking: fold as much of the WAL as possible into the
	// main file. A pinned reader may leave frames in the WAL; the sidecars are
	// copied as well, so nothing committed is lost.
	if _, err := db.DB.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		return fmt.Errorf("checkpoint before backup: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	// Block writers for the duration of the copy so main+WAL are consistent.
	conn, err := db.DB.Conn(context.Background())
	if err != nil {
		return err
	}
	defer conn.Close()
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("cannot lock database for backup: %w", err)
	}
	created, copyErr := copyDBSet(src, dest)
	if _, err := conn.ExecContext(context.Background(), `ROLLBACK`); err != nil && copyErr == nil {
		copyErr = err
	}
	if copyErr != nil {
		removeAll(created) // only what this call created
		return copyErr
	}
	return nil
}

// RestoreBackup installs a backup made by BackupTo at cfg.Path, using the
// ACTIVE key. The existing database is never touched until the backup is
// proven good:
//  1. the active key file is loaded (fail closed);
//  2. the backup is copied to a staging file next to the target and opened
//     with the active key (readable, integrity ok) - on failure the original
//     database is untouched;
//  3. a running server is detected by an exclusive-lock probe of the target;
//  4. the existing database is hard-linked to target.pre-restore-<ts> (never
//     deleted, §9.2) and the staging file is atomically renamed over it.
func RestoreBackup(cfg *config.DatabaseConfig, backup string) (err error) {
	if cfg == nil || cfg.Path == "" || isMemoryPath(cfg.Path) {
		return errors.New("restore requires a file-backed SQLite database")
	}
	target := cfg.Path
	keyPath, err := ResolveKeyFilePath(cfg)
	if err != nil {
		return err
	}
	key, err := loadKey(keyPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(backup); err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}
	if st, err := inspectDBFile(backup); err != nil {
		return err
	} else if st == dbPlaintext {
		return errors.New("backup is a plaintext SQLite file; refusing to restore it as the encrypted database (install it as the database file and start the server once to migrate it)")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	ts := timestamp()
	staging := uniquePath(target + ".restoring-" + ts)
	created, err := copyDBSet(backup, staging)
	if err != nil {
		removeAll(created)
		return fmt.Errorf("cannot stage backup: %w", err)
	}
	defer func() {
		if err != nil {
			removeAll(created)
			removeAll(sidecars(staging))
		}
	}()
	if err = verifyStaged(staging, key); err != nil {
		return fmt.Errorf("backup cannot be used with the active key; the existing database was not touched: %w", err)
	}
	for _, s := range sidecars(staging) {
		if st, serr := os.Stat(s); serr == nil && st.Size() > 0 {
			return fmt.Errorf("staged backup still has unmerged %s", filepath.Base(s))
		}
		_ = os.Remove(s)
	}

	if _, serr := os.Lstat(target); serr == nil {
		if rel, perr := probeInUse(target, key); perr != nil {
			return perr
		} else if closeBeforeRename(hostOS) {
			rel() // Windows cannot rename over an open file; the window stays open there
		} else {
			// Hold the exclusive lock until the restored file is renamed in:
			// nobody can open the old file between the probe and the swap.
			defer rel()
		}
		aside := uniquePath(target + ".pre-restore-" + ts)
		if err = linkOrExplain(target, aside); err != nil {
			return fmt.Errorf("keep existing database: %w", err)
		}
		// Stale sidecars must not be attached to the restored file.
		for _, s := range []string{"-wal", "-shm"} {
			if _, e := os.Lstat(target + s); e == nil {
				if rerr := os.Rename(target+s, aside+s); rerr != nil {
					_ = os.Remove(aside)
					return fmt.Errorf("move stale %s aside: %w", s, rerr)
				}
			}
		}
	} else {
		for _, s := range []string{"-wal", "-shm"} {
			if _, e := os.Lstat(target + s); e == nil {
				_ = os.Rename(target+s, uniquePath(target+".pre-restore-"+ts+s))
			}
		}
	}
	if beforeSwapHook != nil {
		beforeSwapHook()
	}
	if err = os.Rename(staging, target); err != nil {
		return fmt.Errorf("install restored database (the previous database is intact): %w", err)
	}
	created = nil
	syncDir(filepath.Dir(target))
	return nil
}

// verifyStaged proves the staged backup opens with key and is intact.
func verifyStaged(path, keyHex string) error {
	db, err := sql.Open("sqlite3", keyedDSN(path, keyHex, "_busy_timeout=30000"))
	if err != nil {
		return scrubDSN(err)
	}
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err := verifyKeyed(db); err != nil {
		return err
	}
	var res string
	if err := db.QueryRow(`PRAGMA integrity_check`).Scan(&res); err != nil {
		return fmt.Errorf("integrity check: %w", err)
	}
	if res != "ok" {
		return fmt.Errorf("integrity check failed: %s", res)
	}
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

// RekeyResult describes a completed key rotation (no key material).
type RekeyResult struct {
	BackupPath       string // encrypted pre-rekey copy of the database
	OldKeyBackupPath string // the OLD key file copy, needed to open BackupPath
	KeyFile          string // the (new) active key file
}

// verifyRekey proves, after PRAGMA rekey: the new key opens the database with
// exactly the row counts it had before, and the old key no longer opens it.
func verifyRekey(dbPath, oldKeyHex, newKeyHex string, before map[string]int64) error {
	chk, err := sql.Open("sqlite3", keyedDSN(dbPath, newKeyHex, "_busy_timeout=5000"))
	if err != nil {
		return scrubDSN(err)
	}
	chk.SetMaxOpenConns(1)
	if err := verifyKeyed(chk); err != nil {
		chk.Close()
		return fmt.Errorf("verification with new key failed: %w", err)
	}
	after, err := tableRowCounts(chk)
	chk.Close()
	if err != nil {
		return err
	}
	if err := equalCounts(before, after); err != nil {
		return fmt.Errorf("row-count verification after rekey failed: %w", err)
	}
	old, err := sql.Open("sqlite3", keyedDSN(dbPath, oldKeyHex, "_busy_timeout=5000"))
	if err != nil {
		return scrubDSN(err)
	}
	old.SetMaxOpenConns(1)
	oerr := verifyKeyed(old)
	old.Close()
	if oerr == nil {
		return errors.New("old key still opens the database after rekey")
	}
	return nil
}

// Rekey rotates the SQLCipher key of the configured database. The server MUST
// be stopped. A running server is detected only while it holds a connection:
// the database is switched to exclusive locking (which fails while any other
// connection has it open) and that lock is held for the whole operation, so
// nothing can write between the backup and the rekey and a live pool cannot be
// broken. A server whose pool is idle (connections are closed after 3 minutes)
// holds no lock and is NOT detected. Sequence: open
// with the old key -> take the exclusive lock -> §9.2 backup (sqlcipher_export
// under the old key, verified) and old-key copy -> stage the new key ->
// PRAGMA rekey -> verify (new key opens with identical row counts, old key
// fails) -> atomically install the new key file. The new key is staged in
// <keyfile>.new first so a crash can never lose it.
func Rekey(cfg *config.DatabaseConfig) (*RekeyResult, error) {
	if cfg == nil || cfg.Path == "" || isMemoryPath(cfg.Path) {
		return nil, errors.New("rekey requires a file-backed SQLite database")
	}
	state, err := inspectDBFile(cfg.Path)
	if err != nil {
		return nil, err
	}
	if state == dbMissing {
		return nil, fmt.Errorf("database %q does not exist; nothing to rekey", cfg.Path)
	}
	if state == dbPlaintext {
		return nil, errors.New("database is still plaintext; start the server once to migrate it before rekeying")
	}
	keyPath, err := ResolveKeyFilePath(cfg)
	if err != nil {
		return nil, err
	}
	oldKey, err := loadKey(keyPath)
	if err != nil {
		return nil, err
	}
	stage := keyPath + ".new"
	if _, err := os.Lstat(stage); err == nil {
		return nil, fmt.Errorf("a previous rekey did not finish: %q exists. Inspect it (it may hold the key the database is currently encrypted with) and remove it before retrying", stage)
	}

	db, err := sql.Open("sqlite3", keyedDSN(cfg.Path, oldKey, fmt.Sprintf("_busy_timeout=%d", lockBusyMS)))
	if err != nil {
		return nil, scrubDSN(err)
	}
	db.SetMaxOpenConns(1)
	closeDB := func() { _ = db.Close() }
	if err := verifyKeyed(db); err != nil {
		closeDB()
		return nil, err
	}
	if err := acquireExclusive(db); err != nil {
		closeDB()
		return nil, err
	}
	before, err := tableRowCounts(db)
	if err != nil {
		closeDB()
		return nil, err
	}
	var uv int64
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&uv); err != nil {
		closeDB()
		return nil, err
	}

	ts := timestamp()
	res := &RekeyResult{
		BackupPath:       uniquePath(cfg.Path + ".pre-rekey-" + ts),
		OldKeyBackupPath: uniquePath(keyPath + ".pre-rekey-" + ts),
		KeyFile:          keyPath,
	}
	if err := exportEncryptedCopy(db, res.BackupPath, oldKey, uv); err != nil {
		closeDB()
		_ = os.Remove(res.BackupPath)
		return nil, fmt.Errorf("pre-rekey backup failed (database unchanged): %w", err)
	}
	if err := verifyEncryptedCopy(res.BackupPath, oldKey, before, uv); err != nil {
		closeDB()
		_ = os.Remove(res.BackupPath)
		removeAll(sidecars(res.BackupPath))
		return nil, fmt.Errorf("pre-rekey backup did not verify (database unchanged): %w", err)
	}
	if err := copyFile(keyPath, res.OldKeyBackupPath); err != nil {
		closeDB()
		return nil, fmt.Errorf("old key backup failed (database unchanged, backup at %s): %w", res.BackupPath, err)
	}

	newKeyHex, err := newKey()
	if err != nil {
		closeDB()
		return nil, err
	}
	if err := writeKeyFileExclusive(stage, newKeyHex); err != nil {
		closeDB()
		return nil, fmt.Errorf("cannot stage new key (database unchanged): %w", err)
	}
	if _, err := db.Exec(fmt.Sprintf(`PRAGMA rekey = "x'%s'"`, newKeyHex)); err != nil {
		closeDB()
		_ = os.Remove(stage)
		return nil, fmt.Errorf("PRAGMA rekey failed (database unchanged, backup at %s): %w", res.BackupPath, scrubDSN(err))
	}
	closeDB() // releases the exclusive lock
	if afterRekeyHook != nil {
		afterRekeyHook(cfg.Path, newKeyHex)
	}
	if err := verifyRekey(cfg.Path, oldKey, newKeyHex, before); err != nil {
		return nil, fmt.Errorf("%w (new key staged at %s, backup at %s, old key backup at %s)", err, stage, res.BackupPath, res.OldKeyBackupPath)
	}
	if err := os.Rename(stage, keyPath); err != nil {
		return nil, fmt.Errorf("install new key (the new key is at %s): %w", stage, err)
	}
	syncDir(filepath.Dir(keyPath))
	return res, nil
}

// scrubDSN guarantees an error never carries key material: driver errors may
// quote the DSN, which contains the key.
func scrubDSN(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if i := strings.Index(msg, "_pragma_key="); i >= 0 {
		return errors.New(msg[:i] + "_pragma_key=<redacted>")
	}
	return err
}
