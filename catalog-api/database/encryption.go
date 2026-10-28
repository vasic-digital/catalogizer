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

// loadKey reads and validates the key file. Fails closed.
func loadKey(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("database key file %q: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return "", fmt.Errorf("database key file %q must be a regular file (not a symlink)", path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("database key file %q has insecure permission %04o: group/world access is not allowed (chmod 600)", path, info.Mode().Perm())
	}
	if err := checkKeyFileOwner(info); err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
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
			return "", fmt.Errorf("plaintext to encrypted migration failed (original left untouched): %w", err)
		}
		return key, nil
	default:
		return loadOrCreateKey(keyPath, false)
	}
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

// migratePlaintextToEncrypted converts path (plaintext SQLite) into an
// encrypted database using sqlcipher_export. Safe sequence:
//  1. checkpoint+export into path.encrypting-<ts> (original untouched);
//  2. open the new file with the key and verify identical per-table row counts
//     and user_version;
//  3. move the plaintext original (and its -wal/-shm) to
//     path.plaintext-backup-<ts>[...] (0600) - kept, never deleted;
//  4. atomically rename the encrypted file into place.
//
// On any failure before step 3 the original is untouched; the temp file is
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

	src, err := sql.Open("sqlite3", path+"?_busy_timeout=30000")
	if err != nil {
		return err
	}
	src.SetMaxOpenConns(1)
	closeSrc := func() { _ = src.Close() }

	if _, err = src.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		closeSrc()
		return fmt.Errorf("checkpoint: %w", err)
	}
	srcCounts, err := tableRowCounts(src)
	if err != nil {
		closeSrc()
		return fmt.Errorf("count source rows: %w", err)
	}
	var userVersion int64
	if err = src.QueryRow(`PRAGMA user_version`).Scan(&userVersion); err != nil {
		closeSrc()
		return err
	}
	attach := fmt.Sprintf(`ATTACH DATABASE '%s' AS encrypted KEY "x'%s'"`, strings.ReplaceAll(tmpPath, "'", "''"), keyHex)
	if _, err = src.Exec(attach); err != nil {
		closeSrc()
		return fmt.Errorf("attach encrypted target: %w", err)
	}
	if _, err = src.Exec(`SELECT sqlcipher_export('encrypted')`); err != nil {
		closeSrc()
		return fmt.Errorf("sqlcipher_export: %w", err)
	}
	if _, err = src.Exec(fmt.Sprintf(`PRAGMA encrypted.user_version = %d`, userVersion)); err != nil {
		closeSrc()
		return fmt.Errorf("copy user_version: %w", err)
	}
	if _, err = src.Exec(`DETACH DATABASE encrypted`); err != nil {
		closeSrc()
		return err
	}
	// Re-checkpoint the original so nothing is left in its WAL, then release it.
	if _, err = src.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		closeSrc()
		return err
	}
	if err = src.Close(); err != nil {
		return err
	}

	dst, err := sql.Open("sqlite3", keyedDSN(tmpPath, keyHex, "_busy_timeout=30000"))
	if err != nil {
		return err
	}
	dst.SetMaxOpenConns(1)
	if err = verifyKeyed(dst); err != nil {
		dst.Close()
		return fmt.Errorf("verify encrypted copy: %w", err)
	}
	dstCounts, err := tableRowCounts(dst)
	if err != nil {
		dst.Close()
		return fmt.Errorf("count encrypted rows: %w", err)
	}
	var dstUV int64
	if err = dst.QueryRow(`PRAGMA user_version`).Scan(&dstUV); err != nil {
		dst.Close()
		return err
	}
	if _, err = dst.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		dst.Close()
		return err
	}
	if err = dst.Close(); err != nil {
		return err
	}
	if err = equalCounts(srcCounts, dstCounts); err != nil {
		return fmt.Errorf("row-count verification failed: %w", err)
	}
	if dstUV != userVersion {
		return fmt.Errorf("user_version mismatch after export: %d vs %d", userVersion, dstUV)
	}
	for _, s := range sidecars(tmpPath) { // none expected after checkpoint
		_ = os.Remove(s)
	}
	if err = os.Chmod(tmpPath, 0o600); err != nil {
		return err
	}

	// Swap: keep the plaintext original as a (0600) backup, install the new file.
	bak := uniquePath(path + ".plaintext-backup-" + ts)
	if err = os.Rename(path, bak); err != nil {
		return fmt.Errorf("keep plaintext backup: %w", err)
	}
	_ = os.Chmod(bak, 0o600)
	for _, s := range []string{"-wal", "-shm"} {
		if _, serr := os.Lstat(path + s); serr == nil {
			_ = os.Rename(path+s, bak+s)
			_ = os.Chmod(bak+s, 0o600)
		}
	}
	if err = os.Rename(tmpPath, path); err != nil {
		// restore the original so the system is never left without a database
		_ = os.Rename(bak, path)
		return fmt.Errorf("install encrypted database: %w", err)
	}
	syncDir(filepath.Dir(path))
	return nil
}

// copyFile copies src to dst (0600), fsyncing dst.
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
func copyDBSet(srcPath, dest string) error {
	if err := copyFile(srcPath, dest); err != nil {
		return err
	}
	for _, s := range []string{"-wal", "-shm"} {
		st, err := os.Stat(srcPath + s)
		if err != nil || st.Size() == 0 {
			continue
		}
		if err := copyFile(srcPath+s, dest+s); err != nil {
			return err
		}
	}
	return nil
}

// BackupTo writes a consistent, still-encrypted copy of the database to dest
// (mode 0600) including the WAL/-shm sidecars when a reader prevented the WAL
// from being fully checkpointed. The key is NOT part of the backup: back the
// key file up separately (docs/guides/DATABASE_ENCRYPTION.md).
func (db *DB) BackupTo(dest string) error {
	if db.DatabaseType() != "sqlite" {
		return errors.New("backup is supported only for SQLite databases")
	}
	src := db.DBPath()
	if src == "" || isMemoryPath(src) {
		return errors.New("no file-backed database to back up")
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
	copyErr := copyDBSet(src, dest)
	if _, err := conn.ExecContext(context.Background(), `ROLLBACK`); err != nil && copyErr == nil {
		copyErr = err
	}
	if copyErr != nil {
		_ = os.Remove(dest)
		_ = os.Remove(dest + "-wal")
		_ = os.Remove(dest + "-shm")
		return copyErr
	}
	return nil
}

// RestoreBackup installs a backup made by BackupTo at target. Any existing
// database at target (and its sidecars) is first moved aside to
// target.pre-restore-<ts> (never deleted, §9.2). The restore is performed with
// the database closed; the key file must be the one the backup was made with.
func RestoreBackup(backup, target string) error {
	if _, err := os.Stat(backup); err != nil {
		return fmt.Errorf("backup not found: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}
	ts := timestamp()
	if _, err := os.Lstat(target); err == nil {
		aside := uniquePath(target + ".pre-restore-" + ts)
		if err := os.Rename(target, aside); err != nil {
			return fmt.Errorf("move existing database aside: %w", err)
		}
		for _, s := range []string{"-wal", "-shm"} {
			if _, e := os.Lstat(target + s); e == nil {
				_ = os.Rename(target+s, aside+s)
			}
		}
	} else {
		// stale sidecars with no main file must not be attached to the restored DB
		for _, s := range []string{"-wal", "-shm"} {
			if _, e := os.Lstat(target + s); e == nil {
				_ = os.Rename(target+s, uniquePath(target+".pre-restore-"+ts+s))
			}
		}
	}
	return copyDBSet(backup, target)
}

// RekeyResult describes a completed key rotation (no key material).
type RekeyResult struct {
	BackupPath       string // encrypted pre-rekey copy of the database (+ sidecars)
	OldKeyBackupPath string // the OLD key file copy, needed to open BackupPath
	KeyFile          string // the (new) active key file
}

// Rekey rotates the SQLCipher key of the configured database. The server MUST
// be stopped. Sequence: open with old key -> checkpoint -> hold write lock ->
// §9.2 backup (db + sidecars) and old-key copy -> PRAGMA rekey with a fresh
// key -> verify (new key opens with identical row counts, old key fails) ->
// atomically install the new key file. The new key is staged in
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

	db, err := sql.Open("sqlite3", keyedDSN(cfg.Path, oldKey, "_busy_timeout=5000"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	closeDB := func() { _ = db.Close() }
	if err := verifyKeyed(db); err != nil {
		closeDB()
		return nil, err
	}
	before, err := tableRowCounts(db)
	if err != nil {
		closeDB()
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		closeDB()
		return nil, fmt.Errorf("checkpoint: %w", err)
	}

	ts := timestamp()
	res := &RekeyResult{
		BackupPath:       uniquePath(cfg.Path + ".pre-rekey-" + ts),
		OldKeyBackupPath: uniquePath(keyPath + ".pre-rekey-" + ts),
		KeyFile:          keyPath,
	}
	// Hold the write lock so no other process modifies the DB while we copy.
	conn, err := db.Conn(context.Background())
	if err != nil {
		closeDB()
		return nil, err
	}
	if _, err := conn.ExecContext(context.Background(), `BEGIN IMMEDIATE`); err != nil {
		conn.Close()
		closeDB()
		return nil, fmt.Errorf("database is busy or in use; stop the server before rekeying: %w", err)
	}
	copyErr := copyDBSet(cfg.Path, res.BackupPath)
	if _, err := conn.ExecContext(context.Background(), `ROLLBACK`); err != nil && copyErr == nil {
		copyErr = err
	}
	conn.Close()
	if copyErr != nil {
		closeDB()
		return nil, fmt.Errorf("pre-rekey backup failed (database unchanged): %w", copyErr)
	}
	if err := copyFile(keyPath, res.OldKeyBackupPath); err != nil {
		closeDB()
		return nil, fmt.Errorf("old key backup failed (database unchanged): %w", err)
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
		return nil, fmt.Errorf("PRAGMA rekey failed (database unchanged, backup at %s): %w", res.BackupPath, err)
	}
	if _, err := db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		closeDB()
		return nil, fmt.Errorf("checkpoint after rekey (new key staged at %s): %w", stage, err)
	}
	closeDB()

	// Verify with the NEW key, and that the OLD key no longer works.
	chk, err := sql.Open("sqlite3", keyedDSN(cfg.Path, newKeyHex, "_busy_timeout=5000"))
	if err != nil {
		return nil, err
	}
	chk.SetMaxOpenConns(1)
	if err := verifyKeyed(chk); err != nil {
		chk.Close()
		return nil, fmt.Errorf("verification with new key failed (new key staged at %s, backup at %s): %w", stage, res.BackupPath, err)
	}
	after, err := tableRowCounts(chk)
	chk.Close()
	if err != nil {
		return nil, err
	}
	if err := equalCounts(before, after); err != nil {
		return nil, fmt.Errorf("row-count verification after rekey failed (backup at %s): %w", res.BackupPath, err)
	}
	old, err := sql.Open("sqlite3", keyedDSN(cfg.Path, oldKey, "_busy_timeout=5000"))
	if err == nil {
		old.SetMaxOpenConns(1)
		oerr := verifyKeyed(old)
		old.Close()
		if oerr == nil {
			return nil, errors.New("old key still opens the database after rekey; aborting before installing the new key")
		}
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
