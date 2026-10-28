package database

// CAT-001 review round 2: regression tests for R4 (migration lost a late
// write), R6 (probe failed open on a busy database) and the surviving
// reviewer mutants X1, X2, X4, X5, X6. Real go-sqlcipher driver, real files.

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func keyOfCfg(t *testing.T, keyPath string) string {
	t.Helper()
	k, err := loadKey(keyPath)
	require.NoError(t, err)
	return k
}

func plainWAL(t *testing.T, path string, rows int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer raw.Close()
	_, err = raw.Exec(`PRAGMA journal_mode=WAL`)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE enc_t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	for i := 0; i < rows; i++ {
		_, err = raw.Exec(`INSERT INTO enc_t (v) VALUES ('p')`)
		require.NoError(t, err)
	}
}

func rawCount(t *testing.T, path string) int {
	t.Helper()
	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer raw.Close()
	var n int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n))
	return n
}

// R4: the source stays locked from before the row counts until the rename, so
// a writer that tries to commit after the export is refused instead of being
// silently dropped from the encrypted copy.
func TestHardening_R4_MigrationHoldsSourceLock(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	plainWAL(t, dbPath, 5)

	var writerErr error
	afterExportHook = func(string, string) {
		w, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=100")
		if err != nil {
			writerErr = err
			return
		}
		defer w.Close()
		_, writerErr = w.Exec(`INSERT INTO enc_t (v) VALUES ('late-write')`)
	}
	defer func() { afterExportHook = nil }()

	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	baks, _ := filepath.Glob(dbPath + ".plaintext-backup-*")
	require.Len(t, baks, 1)
	// Invariant: no committed write may exist only in the plaintext backup.
	assert.Equal(t, rawCount(t, baks[0]), countRows(t, db),
		"a write committed to the plaintext source was lost by the migration")
	require.Error(t, writerErr, "the late writer must have been refused while migration holds the lock")
}

// R4b: another process with the plaintext database open (a running old
// server) makes the migration refuse instead of racing it.
func TestHardening_R4_MigrationRefusesWhenSourceInUse(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	plainWAL(t, dbPath, 5)
	other, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	other.SetMaxOpenConns(1)
	defer other.Close()
	var n int
	require.NoError(t, other.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n)) // keeps the connection open

	old := lockBusyMS
	lockBusyMS = 200
	defer func() { lockBusyMS = old }()
	_, err = NewConnection(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "in use")
	// the plaintext database is untouched and no temp files are left
	assert.Equal(t, "SQLite format 3\x00", string(headerOf(t, dbPath)))
	left, _ := filepath.Glob(dbPath + ".encrypting-*")
	assert.Empty(t, left)
	baks, _ := filepath.Glob(dbPath + ".plaintext-backup-*")
	assert.Empty(t, baks)
}

// X6: unmerged plaintext -wal content at swap time is refused.
func TestHardening_X6_MigrationRefusesUnmergedSidecar(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	plainDB(t, dbPath, 5)
	afterExportHook = func(string, string) {
		_ = os.WriteFile(dbPath+"-wal", []byte("unmerged frames"), 0o600)
	}
	defer func() { afterExportHook = nil }()
	_, err := NewConnection(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unmerged")
	assert.Equal(t, "SQLite format 3\x00", string(headerOf(t, dbPath)))
}

// R6: a database held under an exclusive lock by another process (BUSY) is
// IN USE; the probe must fail closed. Only a genuine decrypt failure on a
// quiescent file means "not in use".
func TestHardening_R6_ProbeFailsClosedWhenBusy(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "r6")
	require.NoError(t, db.Close())
	key := keyOfCfg(t, keyPath)

	holder, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=200"))
	require.NoError(t, err)
	holder.SetMaxOpenConns(1)
	defer holder.Close()
	require.NoError(t, verifyKeyed(holder))
	require.NoError(t, acquireExclusive(holder))

	old := lockBusyMS
	lockBusyMS = 200
	defer func() { lockBusyMS = old }()
	rel, err := probeInUse(dbPath, key)
	if rel != nil {
		rel()
	}
	require.Error(t, err, "busy database must be reported in use")
	assert.Contains(t, err.Error(), "in use")
}

func TestHardening_R6_ProbeWrongKeyQuiescentIsNotInUse(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "r6b")
	require.NoError(t, db.Close())
	other, err := newKey()
	require.NoError(t, err)
	rel, err := probeInUse(dbPath, other)
	require.NoError(t, err, "undecryptable quiescent file cannot be a server using this key")
	if rel != nil {
		rel()
	}
}

// X1: after acquireExclusive nobody else can read or write the database.
func TestHardening_X1_ExclusiveLockBlocksOthers(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "x1")
	require.NoError(t, db.Close())
	key := keyOfCfg(t, keyPath)

	holder, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=200"))
	require.NoError(t, err)
	holder.SetMaxOpenConns(1)
	defer holder.Close()
	require.NoError(t, verifyKeyed(holder))
	require.NoError(t, acquireExclusive(holder))

	other, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=100"))
	require.NoError(t, err)
	other.SetMaxOpenConns(1)
	defer other.Close()
	_, err = other.Exec(`INSERT INTO enc_t (v) VALUES ('intruder')`)
	assert.Error(t, err, "a write must be refused while the exclusive lock is held")
	var n int
	assert.Error(t, other.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n),
		"a read must be refused while the exclusive lock is held")
}

// X2: another connection keeps the database in WAL mode; the journal-mode
// switch does not take effect and acquireExclusive must refuse.
func TestHardening_X2_WALHolderRefusesExclusive(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "x2")
	defer db.Close()
	key := keyOfCfg(t, keyPath)

	second, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=200"))
	require.NoError(t, err)
	second.SetMaxOpenConns(1)
	defer second.Close()
	require.NoError(t, verifyKeyed(second))
	err = acquireExclusive(second)
	require.Error(t, err, "live WAL connection present: exclusive access must be refused")
	assert.Contains(t, err.Error(), "in use")
}

// X4: a backup that decrypts (cipher_version + sqlite_master readable) but is
// internally inconsistent must be refused by the integrity check.
func TestHardening_X4_RestoreRefusesCorruptButDecryptableBackup(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "x4")
	_, err = db.Exec(`CREATE INDEX idx_enc_v ON enc_t(v)`)
	require.NoError(t, err)
	conn, err := db.DB.Conn(t.Context())
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `PRAGMA writable_schema=ON`)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `DELETE FROM sqlite_master WHERE type='index' AND name='idx_enc_v'`)
	require.NoError(t, err)
	_, err = conn.ExecContext(t.Context(), `PRAGMA writable_schema=OFF`)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
	bad := filepath.Join(t.TempDir(), "bad.db")
	require.NoError(t, db.BackupTo(bad))
	require.NoError(t, db.Close())

	// precondition: the backup really decrypts yet fails the integrity check
	key := keyOfCfg(t, keyPath)
	chk, err := sql.Open("sqlite3", keyedDSN(bad, key, "_busy_timeout=1000"))
	require.NoError(t, err)
	chk.SetMaxOpenConns(1)
	require.NoError(t, verifyKeyed(chk))
	var res string
	require.NoError(t, chk.QueryRow(`PRAGMA integrity_check`).Scan(&res))
	require.NotEqual(t, "ok", res)
	chk.Close()

	err = RestoreBackup(cfg, bad)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "integrity")
	m, _ := filepath.Glob(dbPath + ".pre-restore-*")
	assert.Empty(t, m, "existing database must not be touched")
}

// X5: BackupTo must take the write lock before copying: a concurrent write
// transaction makes it fail instead of copying a moving target.
func TestHardening_X5_BackupRequiresWriteLock(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "x5")
	defer db.Close()
	db.DB.SetMaxOpenConns(1)
	_, err = db.Exec(`PRAGMA busy_timeout=100`)
	require.NoError(t, err)
	key := keyOfCfg(t, keyPath)

	w, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=100"))
	require.NoError(t, err)
	w.SetMaxOpenConns(1)
	defer w.Close()
	_, err = w.Exec(`BEGIN IMMEDIATE`)
	require.NoError(t, err)

	dest := filepath.Join(t.TempDir(), "b.db")
	err = db.BackupTo(dest)
	require.Error(t, err, "backup must not proceed while another writer holds the lock")
	assert.Contains(t, err.Error(), "lock")
	_, serr := os.Lstat(dest)
	assert.True(t, os.IsNotExist(serr))
}

// F3: the probe lock is held until the restored file is renamed in, so no
// process can open the old file between the probe and the swap.
func TestHardening_F3_RestoreHoldsLockUntilSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("rename over an open file is not possible on Windows (documented)")
	}
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "f3")
	b := filepath.Join(t.TempDir(), "b.db")
	require.NoError(t, db.BackupTo(b))
	require.NoError(t, db.Close())
	key := keyOfCfg(t, keyPath)

	var intruderErr error
	beforeSwapHook = func() {
		w, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=100"))
		if err != nil {
			intruderErr = err
			return
		}
		defer w.Close()
		w.SetMaxOpenConns(1)
		_, intruderErr = w.Exec(`INSERT INTO enc_t (v) VALUES ('intruder')`)
	}
	defer func() { beforeSwapHook = nil }()
	require.NoError(t, RestoreBackup(cfg, b))
	require.Error(t, intruderErr, "a writer must be refused between the probe and the swap")
	assert.True(t, strings.Contains(fmt.Sprint(intruderErr), "lock") || strings.Contains(fmt.Sprint(intruderErr), "busy"), "%v", intruderErr)
}

// X2: the journal-mode result check, exercised directly (the bundled SQLite
// 3.20.1 errors out of the pragma before this branch can be reached).
func TestHardening_X2_RequireDeleteMode(t *testing.T) {
	assert.NoError(t, requireDeleteMode("delete"))
	assert.NoError(t, requireDeleteMode("DELETE"))
	assert.Error(t, requireDeleteMode("wal"))
	assert.Error(t, requireDeleteMode(""))
}

// Round 3 / Windows: the platform decision and the lock release before the
// final rename (the bundled SQLite opens files without FILE_SHARE_DELETE).
func TestHardening_R7_CloseBeforeRenameDecision(t *testing.T) {
	assert.True(t, closeBeforeRename("windows"))
	assert.False(t, closeBeforeRename("linux"))
	assert.False(t, closeBeforeRename("darwin"))
}

// lockedAtSwap reports whether a reader is refused at the moment the swap
// hook fires (i.e. whether the exclusive lock is still held).
func lockedAtSwap(t *testing.T, path, key string) *bool {
	var res bool
	beforeSwapHook = func() {
		var r *sql.DB
		var err error
		if key == "" {
			r, err = sql.Open("sqlite3", path+"?_busy_timeout=100")
		} else {
			r, err = sql.Open("sqlite3", keyedDSN(path, key, "_busy_timeout=100"))
		}
		if err != nil {
			return
		}
		defer r.Close()
		r.SetMaxOpenConns(1)
		var n int
		res = r.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&n) != nil
	}
	return &res
}

func TestHardening_R7_MigrationLockReleasedBeforeRenameOnWindowsOnly(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		goos := goos
		t.Run(goos, func(t *testing.T) {
			cfg, dbPath, _ := encEnv(t)
			plainDB(t, dbPath, 5)
			old := hostOS
			hostOS = goos
			defer func() { hostOS = old; beforeSwapHook = nil }()
			locked := lockedAtSwap(t, dbPath, "")
			db, err := NewConnection(cfg)
			require.NoError(t, err)
			require.NoError(t, db.Close())
			assert.Equal(t, goos == "linux", *locked, "lock held at swap time must be %v on %s", goos == "linux", goos)
		})
	}
}

func TestHardening_R7_RestoreLockReleasedBeforeRenameOnWindowsOnly(t *testing.T) {
	for _, goos := range []string{"linux", "windows"} {
		goos := goos
		t.Run(goos, func(t *testing.T) {
			cfg, dbPath, keyPath := encEnv(t)
			db, err := NewConnection(cfg)
			require.NoError(t, err)
			seed(t, db, "r7")
			b := filepath.Join(t.TempDir(), "b.db")
			require.NoError(t, db.BackupTo(b))
			require.NoError(t, db.Close())
			old := hostOS
			hostOS = goos
			defer func() { hostOS = old; beforeSwapHook = nil }()
			locked := lockedAtSwap(t, dbPath, keyOfCfg(t, keyPath))
			require.NoError(t, RestoreBackup(cfg, b))
			assert.Equal(t, goos == "linux", *locked)
		})
	}
}

// Y2: an error that is neither busy nor NOTADB must make the probe refuse.
func TestHardening_Y2_ProbeUnknownErrorFailsClosed(t *testing.T) {
	rel, err := classifyProbeError(errors.New("disk I/O error"))
	require.Error(t, err)
	assert.Nil(t, rel)
	assert.Contains(t, err.Error(), "cannot determine")
	rel, err = classifyProbeError(errors.New("file is not a database"))
	require.NoError(t, err)
	assert.NotNil(t, rel)
	_, err = classifyProbeError(errors.New("database is locked"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "in use")
}
