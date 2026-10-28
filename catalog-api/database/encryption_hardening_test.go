package database

// CAT-001 review round 1: regression tests for the findings of the
// independent review (R1-R3 probes and surviving mutants RM1-RM6). Real
// go-sqlcipher driver and real files, no mocks.

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func plainDB(t *testing.T, path string, rows int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	defer raw.Close()
	_, err = raw.Exec(`CREATE TABLE enc_t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	for i := 0; i < rows; i++ {
		_, err = raw.Exec(`INSERT INTO enc_t (v) VALUES ('p')`)
		require.NoError(t, err)
	}
}

// R1: a second backup to an existing destination must fail WITHOUT deleting
// the first backup (the old code removed dest on any failure).
func TestHardening_R1_BackupCollisionKeepsFirstBackup(t *testing.T) {
	cfg, _, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	seed(t, db, "r1")
	dest := filepath.Join(t.TempDir(), "b.db")
	require.NoError(t, db.BackupTo(dest))
	before, err := os.ReadFile(dest)
	require.NoError(t, err)

	err = db.BackupTo(dest)
	require.Error(t, err)
	after, rerr := os.ReadFile(dest)
	require.NoError(t, rerr, "first backup must still exist")
	assert.Equal(t, before, after, "first backup must be untouched")
}

// R1 (cleanup half): a failed copy removes only files this call created.
func TestHardening_BackupFailureRemovesOnlyOwnFiles(t *testing.T) {
	cfg, _, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	seed(t, db, "own")
	dir := t.TempDir()
	dest := filepath.Join(dir, "b.db")
	// A pre-existing sidecar name must survive a failing backup.
	require.NoError(t, os.WriteFile(dest+"-wal", []byte("pre-existing"), 0o600))
	// Make a WAL-resident row so a -wal copy is attempted and collides.
	rdb := rawKeyedReader(t, cfg)
	defer rdb()
	_, err = db.Exec(`INSERT INTO enc_t (v) VALUES ('wal-row')`)
	require.NoError(t, err)
	err = db.BackupTo(dest)
	require.Error(t, err, "the pre-existing -wal name must make the sidecar copy fail")
	_, serr := os.Stat(dest)
	assert.True(t, os.IsNotExist(serr), "the main backup file this call created is removed")
	b, rerr := os.ReadFile(dest + "-wal")
	require.NoError(t, rerr, "pre-existing file must survive")
	assert.Equal(t, "pre-existing", string(b))
}

// rawKeyedReader pins a read transaction so the WAL cannot be fully
// checkpointed; returns a release func.
func rawKeyedReader(t *testing.T, cfg *config.DatabaseConfig) func() {
	t.Helper()
	key, err := loadKey(cfg.EncryptionKeyFile)
	require.NoError(t, err)
	r, err := sql.Open("sqlite3", keyedDSN(cfg.Path, key, "_busy_timeout=5000"))
	require.NoError(t, err)
	r.SetMaxOpenConns(1)
	tx, err := r.Begin()
	require.NoError(t, err)
	var n int
	require.NoError(t, tx.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n))
	return func() { _ = tx.Rollback(); _ = r.Close() }
}

// R2: Rekey must detect a live pool, refuse, and leave the live pool and the
// key untouched (probe: the server's next write failed "file is not a
// database" after a rekey under it).
func TestHardening_R2_RekeyRefusesLiveServer(t *testing.T) {
	cfg, _, keyPath := encEnv(t)
	live, err := NewConnection(cfg)
	require.NoError(t, err)
	defer live.Close()
	seed(t, live, "r2")
	oldKey, err := os.ReadFile(keyPath)
	require.NoError(t, err)

	old := lockBusyMS
	lockBusyMS = 200
	defer func() { lockBusyMS = old }()
	res, err := Rekey(cfg)
	require.Error(t, err)
	assert.Nil(t, res)
	assert.Contains(t, err.Error(), "in use")

	newKey, _ := os.ReadFile(keyPath)
	assert.Equal(t, oldKey, newKey, "key file must be unchanged")
	_, werr := live.Exec(`INSERT INTO enc_t (v) VALUES ('after-refused-rekey')`)
	assert.NoError(t, werr, "the live pool must still work")
	assert.Equal(t, 21, countRows(t, live))
	_, serr := os.Stat(keyPath + ".new")
	assert.True(t, os.IsNotExist(serr), "no staged key may be left behind")
}

// R3: restoring a backup the active key cannot open fails BEFORE the live
// database is touched.
func TestHardening_R3_RestoreMismatchedKeyLeavesOriginal(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "r3")
	backup := filepath.Join(t.TempDir(), "b.db")
	require.NoError(t, db.BackupTo(backup))
	require.NoError(t, db.Close())

	// rotate the key: the backup is now encrypted with a key that is gone
	_, err = Rekey(cfg)
	require.NoError(t, err)
	keyAfter, _ := os.ReadFile(keyPath)
	dbBefore, _ := os.ReadFile(dbPath)

	err = RestoreBackup(cfg, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "existing database was not touched")
	dbAfter, _ := os.ReadFile(dbPath)
	assert.Equal(t, dbBefore, dbAfter, "original database bytes unchanged")
	keyNow, _ := os.ReadFile(keyPath)
	assert.Equal(t, keyAfter, keyNow)
	pre, _ := filepath.Glob(dbPath + ".pre-restore-*")
	assert.Empty(t, pre)
	stg, _ := filepath.Glob(dbPath + ".restoring-*")
	assert.Empty(t, stg, "staging copies must be cleaned up")

	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db2.Close()
	assert.Equal(t, 20, countRows(t, db2))
}

// R3: restoring over a database a running server holds open is refused.
func TestHardening_R3_RestoreRefusesLiveServer(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	live, err := NewConnection(cfg)
	require.NoError(t, err)
	defer live.Close()
	seed(t, live, "live")
	backup := filepath.Join(t.TempDir(), "b.db")
	require.NoError(t, live.BackupTo(backup))
	before, _ := os.ReadFile(dbPath)

	old := lockBusyMS
	lockBusyMS = 200
	defer func() { lockBusyMS = old }()
	err = RestoreBackup(cfg, backup)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "in use")
	pre, _ := filepath.Glob(dbPath + ".pre-restore-*")
	assert.Empty(t, pre, "database must not be moved aside while in use")
	after, _ := os.ReadFile(dbPath)
	assert.Equal(t, before, after)
	_, werr := live.Exec(`INSERT INTO enc_t (v) VALUES ('still-alive')`)
	assert.NoError(t, werr)
}

// R3: a plaintext backup is refused (it would otherwise be installed as the
// "encrypted" database and fail at the next start).
func TestHardening_RestorePlaintextBackupRefused(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "x")
	require.NoError(t, db.Close())
	plain := filepath.Join(t.TempDir(), "plain.db")
	plainDB(t, plain, 3)
	before, _ := os.ReadFile(dbPath)
	err = RestoreBackup(cfg, plain)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "plaintext", "the refusal must say why")
	after, _ := os.ReadFile(dbPath)
	assert.Equal(t, before, after)
}

// RM1: the migration must verify the row counts of the encrypted copy. The
// hook deletes rows from the temp file after export; the migration has to
// refuse and leave the plaintext original in place, with no leftovers.
func TestHardening_RM1_MigrationRowCountCheck(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	plainDB(t, dbPath, 5)
	afterExportHook = func(tmp, keyHex string) {
		d, err := sql.Open("sqlite3", keyedDSN(tmp, keyHex, "_busy_timeout=5000"))
		require.NoError(t, err)
		defer d.Close()
		d.SetMaxOpenConns(1)
		_, err = d.Exec(`DELETE FROM enc_t WHERE id > 2`)
		require.NoError(t, err)
		_, _ = d.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	}
	defer func() { afterExportHook = nil }()

	_, err := NewConnection(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row-count")
	assert.Equal(t, plainHeader, string(headerOf(t, dbPath)), "original must stay plaintext and intact")
	left, _ := filepath.Glob(dbPath + ".encrypting-*")
	assert.Empty(t, left, "temp file must be removed on failure")
	bak, _ := filepath.Glob(dbPath + ".plaintext-backup-*")
	assert.Empty(t, bak, "no backup is made before the copy is verified")
}

// Migration success path: encrypted in place, plaintext original preserved
// as a 0600 hard-link backup (no window without a database file).
func TestHardening_MigrationKeepsPlaintextBackup(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	plainDB(t, dbPath, 7)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	assert.Equal(t, 7, countRows(t, db))
	assert.NotEqual(t, plainHeader, string(headerOf(t, dbPath)))
	bak, _ := filepath.Glob(dbPath + ".plaintext-backup-*")
	require.Len(t, bak, 1)
	assert.Equal(t, plainHeader, string(headerOf(t, bak[0])))
	st, err := os.Stat(bak[0])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
}

// RM2: Rekey must prove the OLD key no longer opens the database.
func TestHardening_RM2_VerifyRekeyOldKeyStillOpens(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "rm2")
	require.NoError(t, db.Close())
	key, err := loadKey(cfg.EncryptionKeyFile)
	require.NoError(t, err)
	counts := map[string]int64{"enc_t": 20}
	// "rekey" to the same key: new key opens, row counts equal, old key opens too.
	err = verifyRekey(dbPath, key, key, counts)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "old key still opens")
}

// RM6: Rekey must verify row counts after the rekey.
func TestHardening_RM6_VerifyRekeyRowCounts(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "rm6")
	require.NoError(t, db.Close())
	key, err := loadKey(cfg.EncryptionKeyFile)
	require.NoError(t, err)
	other, err := newKey()
	require.NoError(t, err)
	err = verifyRekey(dbPath, other, key, map[string]int64{"enc_t": 999})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "row-count")
}

// Rekey end to end after the changes: backup is encrypted with the OLD key,
// the new key opens the database, the old one does not.
func TestHardening_RekeyEndToEnd(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "e2e")
	require.NoError(t, db.Close())
	oldKey, _ := os.ReadFile(keyPath)
	res, err := Rekey(cfg)
	require.NoError(t, err)
	newKey, _ := os.ReadFile(keyPath)
	assert.NotEqual(t, oldKey, newKey)
	require.FileExists(t, res.BackupPath)
	require.FileExists(t, res.OldKeyBackupPath)
	_, serr := os.Stat(keyPath + ".new")
	assert.True(t, os.IsNotExist(serr))

	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	assert.Equal(t, 20, countRows(t, db2))
	require.NoError(t, db2.Close())

	// the pre-rekey backup opens with the old key only
	k := strings.TrimSpace(string(oldKey))
	chk, err := sql.Open("sqlite3", keyedDSN(res.BackupPath, k, ""))
	require.NoError(t, err)
	defer chk.Close()
	require.NoError(t, verifyKeyed(chk))
	_ = dbPath
}

// A failing verification after the rekey must NOT install the new key file
// (the staged key stays for recovery).
func TestHardening_RekeyVerifyFailureKeepsStagedKey(t *testing.T) {
	cfg, _, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "vf")
	require.NoError(t, db.Close())
	oldKey, _ := os.ReadFile(keyPath)
	// break the database after the rekey: re-encrypt with a third key
	afterRekeyHook = func(path, newKeyHex string) {
		d, err := sql.Open("sqlite3", keyedDSN(path, newKeyHex, "_busy_timeout=5000"))
		require.NoError(t, err)
		defer d.Close()
		d.SetMaxOpenConns(1)
		third, err := newKey()
		require.NoError(t, err)
		_, err = d.Exec(`PRAGMA rekey = "x'` + third + `'"`)
		require.NoError(t, err)
	}
	defer func() { afterRekeyHook = nil }()
	_, err = Rekey(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), keyPath+".new")
	cur, _ := os.ReadFile(keyPath)
	assert.Equal(t, oldKey, cur, "key file must not be replaced when verification fails")
	require.FileExists(t, keyPath+".new")
}

// After an interrupted rekey, the startup error points at <keyfile>.new.
func TestHardening_StartupErrorMentionsStagedKey(t *testing.T) {
	cfg, _, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "hint")
	require.NoError(t, db.Close())
	other, err := newKey()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(keyPath, []byte(other+"\n"), 0o600))
	require.NoError(t, os.WriteFile(keyPath+".new", []byte("staged"), 0o600))
	_, err = NewConnection(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), ".new")
	assert.NotContains(t, err.Error(), other, "key material must never appear in errors")
}

// RM3: scrubDSN must cut the key out of driver errors.
func TestHardening_RM3_ScrubDSN(t *testing.T) {
	secret := strings.Repeat("ab", 32)
	e := errors.New("open file:/x.db?_busy_timeout=1&_pragma_key=x%27" + secret + "%27: boom")
	got := scrubDSN(e)
	require.Error(t, got)
	assert.NotContains(t, got.Error(), secret)
	assert.Contains(t, got.Error(), "<redacted>")
	assert.NoError(t, scrubDSN(nil))
	plain := errors.New("no key here")
	assert.Equal(t, plain, scrubDSN(plain))
}

// Key file handling: a symlinked key file and a group/world-writable key
// directory are refused (POSIX; skipped where the platform cannot express it).
func TestHardening_KeyFileSymlinkAndDirRefused(t *testing.T) {
	if !keyPermissionsEnforced() {
		t.Skip("POSIX mode/owner checks are not enforced on this platform (documented)")
	}
	cfg, _, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "k")
	require.NoError(t, db.Close())

	// symlink
	real := keyPath + ".real"
	require.NoError(t, os.Rename(keyPath, real))
	require.NoError(t, os.Symlink(real, keyPath))
	_, err = NewConnection(cfg)
	require.Error(t, err, "symlinked key file must be refused")
	require.NoError(t, os.Remove(keyPath))
	require.NoError(t, os.Rename(real, keyPath))

	// writable directory
	dir := filepath.Dir(keyPath)
	require.NoError(t, os.Chmod(dir, 0o777))
	defer os.Chmod(dir, 0o700)
	_, err = NewConnection(cfg)
	require.Error(t, err, "group/world-writable key directory must be refused")
	assert.Contains(t, err.Error(), "writable")
	require.NoError(t, os.Chmod(dir, 0o700))
	db3, err := NewConnection(cfg)
	require.NoError(t, err)
	require.NoError(t, db3.Close())
}

// Key file with loose permissions is refused (POSIX).
func TestHardening_KeyFileLoosePermsRefused(t *testing.T) {
	if !keyPermissionsEnforced() {
		t.Skip("POSIX mode checks are not enforced on this platform (documented)")
	}
	cfg, _, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	require.NoError(t, os.Chmod(keyPath, 0o644))
	_, err = NewConnection(cfg)
	require.Error(t, err)
}
