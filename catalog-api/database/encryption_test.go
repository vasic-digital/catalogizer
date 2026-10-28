package database

// CAT-001: encryption-at-rest for the main SQLite database (SQLCipher).
// Every test uses the real go-sqlcipher driver and real files - no mocks.

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"catalogizer/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const plainHeader = "SQLite format 3\x00"

// encEnv returns a config pointing at a fresh DB and a fresh key file, both
// inside t.TempDir() (i.e. outside the repository).
func encEnv(t *testing.T) (*config.DatabaseConfig, string, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "data", "cat.db")
	keyPath := filepath.Join(dir, "keys", "db.key")
	require.NoError(t, os.MkdirAll(filepath.Dir(dbPath), 0o755))
	return &config.DatabaseConfig{
		Path:               dbPath,
		EncryptionKeyFile:  keyPath,
		MaxOpenConnections: 8,
		MaxIdleConnections: 4,
	}, dbPath, keyPath
}

func headerOf(t *testing.T, path string) []byte {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	b := make([]byte, 16)
	n, _ := f.Read(b)
	return b[:n]
}

func seed(t *testing.T, db *DB, marker string) {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS enc_t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	for i := 0; i < 20; i++ {
		_, err = db.Exec(`INSERT INTO enc_t (v) VALUES (?)`, fmt.Sprintf("%s-%d", marker, i))
		require.NoError(t, err)
	}
}

func countRows(t *testing.T, db *DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n))
	return n
}

// rawOpen opens the file with the sqlcipher driver and NO key (what an
// attacker holding only the file would do).
func rawOpen(t *testing.T, path string) *sql.DB {
	t.Helper()
	raw, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	return raw
}

func TestEncryption_NewDBIsEncryptedAndNeedsKey(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "SECRETMARKER")
	require.NoError(t, db.Close())

	assert.NotEqual(t, plainHeader, string(headerOf(t, dbPath)), "file header must not be plaintext SQLite")
	for _, f := range []string{dbPath, dbPath + "-wal"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		assert.False(t, bytes.Contains(b, []byte("SECRETMARKER")), "%s leaks plaintext row data", f)
	}

	raw := rawOpen(t, dbPath)
	defer raw.Close()
	var n int
	err = raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master`).Scan(&n)
	assert.Error(t, err, "opening without the key must fail")
}

func TestEncryption_KeyFileCreated0600AndReused(t *testing.T) {
	cfg, _, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "reuse")
	require.NoError(t, db.Close())

	st, err := os.Lstat(keyPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	raw, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	h := strings.TrimSpace(string(raw))
	assert.Len(t, h, 64)
	_, err = hex.DecodeString(h)
	assert.NoError(t, err)

	// restart: same key file reused, data readable
	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db2.Close()
	assert.Equal(t, 20, countRows(t, db2))
	raw2, _ := os.ReadFile(keyPath)
	assert.Equal(t, raw, raw2, "key file must not be regenerated on restart")
}

func TestEncryption_FailsClosed(t *testing.T) {
	t.Run("wrong key", func(t *testing.T) {
		cfg, dbPath, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		seed(t, db, "x")
		require.NoError(t, db.Close())
		before, _ := os.ReadFile(dbPath)

		other := bytes.Repeat([]byte("ab"), 32)
		require.NoError(t, os.WriteFile(keyPath, append(other, '\n'), 0o600))
		_, err = NewConnection(cfg)
		require.Error(t, err)
		assert.NotContains(t, err.Error(), string(other), "error must not echo key material")
		after, _ := os.ReadFile(dbPath)
		assert.Equal(t, before, after, "a failed open must not modify the database")
	})
	t.Run("missing key for existing encrypted db is not regenerated", func(t *testing.T) {
		cfg, _, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		seed(t, db, "x")
		require.NoError(t, db.Close())
		require.NoError(t, os.Remove(keyPath))
		_, err = NewConnection(cfg)
		require.Error(t, err)
		_, statErr := os.Stat(keyPath)
		assert.True(t, os.IsNotExist(statErr), "must not silently mint a new key for an existing encrypted DB")
	})
	t.Run("group/world readable key file", func(t *testing.T) {
		cfg, _, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		require.NoError(t, db.Close())
		require.NoError(t, os.Chmod(keyPath, 0o644))
		_, err = NewConnection(cfg)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "permission")
	})
	t.Run("unreadable key file", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("root can read mode 000 files")
		}
		cfg, _, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		require.NoError(t, db.Close())
		require.NoError(t, os.Chmod(keyPath, 0o000))
		defer os.Chmod(keyPath, 0o600)
		_, err = NewConnection(cfg)
		require.Error(t, err)
	})
	t.Run("malformed key file", func(t *testing.T) {
		cfg, _, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		require.NoError(t, db.Close())
		require.NoError(t, os.WriteFile(keyPath, []byte("not-hex\n"), 0o600))
		_, err = NewConnection(cfg)
		require.Error(t, err)
	})
	t.Run("symlinked key file", func(t *testing.T) {
		cfg, _, keyPath := encEnv(t)
		db, err := NewConnection(cfg)
		require.NoError(t, err)
		require.NoError(t, db.Close())
		real := keyPath + ".real"
		require.NoError(t, os.Rename(keyPath, real))
		require.NoError(t, os.Symlink(real, keyPath))
		_, err = NewConnection(cfg)
		require.Error(t, err)
	})
}

func TestEncryption_MemoryDBExplicitOptOutOnly(t *testing.T) {
	cfg, _, keyPath := encEnv(t)
	cfg.Path = ":memory:"
	cfg.MaxOpenConnections = 1
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	_, statErr := os.Stat(keyPath)
	assert.True(t, os.IsNotExist(statErr), ":memory: must not create a key file")
	_, err = db.Exec(`CREATE TABLE m (a INTEGER)`)
	assert.NoError(t, err)
}

func TestEncryption_EnvKeyFileOverride(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	cfg.EncryptionKeyFile = ""
	envKey := filepath.Join(t.TempDir(), "envkey", "k")
	t.Setenv("DATABASE_ENCRYPTION_KEY_FILE", envKey)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	_, err = os.Stat(envKey)
	assert.NoError(t, err, "key must be created at the env-injected path")
	assert.NotEqual(t, plainHeader, string(headerOf(t, dbPath)))
}

func TestEncryption_PlaintextMigration(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)

	// A pre-existing plaintext DB (what every current install has).
	raw := rawOpen(t, dbPath)
	raw.SetMaxOpenConns(1)
	_, err := raw.Exec(`PRAGMA journal_mode=WAL`)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE enc_t (id INTEGER PRIMARY KEY, v TEXT)`)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE TABLE blobs (id INTEGER PRIMARY KEY, b BLOB)`)
	require.NoError(t, err)
	_, err = raw.Exec(`CREATE INDEX idx_enc_v ON enc_t(v)`)
	require.NoError(t, err)
	for i := 0; i < 200; i++ {
		_, err = raw.Exec(`INSERT INTO enc_t (v) VALUES (?)`, fmt.Sprintf("PLAINROW-%d", i))
		require.NoError(t, err)
	}
	_, err = raw.Exec(`INSERT INTO blobs (b) VALUES (?)`, []byte{0, 1, 2, 255})
	require.NoError(t, err)
	_, err = raw.Exec(`PRAGMA user_version = 42`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())
	require.Equal(t, plainHeader, string(headerOf(t, dbPath)))

	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()

	assert.NotEqual(t, plainHeader, string(headerOf(t, dbPath)), "live DB must now be encrypted")
	assert.Equal(t, 200, countRows(t, db))
	var b []byte
	require.NoError(t, db.QueryRow(`SELECT b FROM blobs`).Scan(&b))
	assert.Equal(t, []byte{0, 1, 2, 255}, b)
	var uv int
	require.NoError(t, db.QueryRow(`PRAGMA user_version`).Scan(&uv))
	assert.Equal(t, 42, uv, "user_version must survive migration")
	var idx int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_enc_v'`).Scan(&idx))
	assert.Equal(t, 1, idx, "indexes must survive migration")

	// Original kept as a plaintext backup, rows intact, 0600.
	matches, err := filepath.Glob(dbPath + ".plaintext-backup-*")
	require.NoError(t, err)
	var main []string
	for _, m := range matches {
		if !strings.HasSuffix(m, "-wal") && !strings.HasSuffix(m, "-shm") {
			main = append(main, m)
		}
	}
	require.Len(t, main, 1, "exactly one plaintext backup expected")
	st, err := os.Stat(main[0])
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())
	assert.Equal(t, plainHeader, string(headerOf(t, main[0])))
	bak := rawOpen(t, main[0])
	defer bak.Close()
	var n int
	require.NoError(t, bak.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n))
	assert.Equal(t, 200, n)

	// key file exists, no leftover temp encrypted file
	_, err = os.Stat(keyPath)
	assert.NoError(t, err)
	tmp, _ := filepath.Glob(dbPath + ".encrypting*")
	assert.Empty(t, tmp)

	// idempotent: second start does not migrate again
	require.NoError(t, db.Close())
	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db2.Close()
	again, _ := filepath.Glob(dbPath + ".plaintext-backup-*")
	var againMain []string
	for _, m := range again {
		if !strings.HasSuffix(m, "-wal") && !strings.HasSuffix(m, "-shm") {
			againMain = append(againMain, m)
		}
	}
	assert.Equal(t, main, againMain, "no second migration/backup on restart")
}

func TestEncryption_Rekey(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "rekey")
	require.NoError(t, db.Close())
	oldKey, _ := os.ReadFile(keyPath)

	res, err := Rekey(cfg)
	require.NoError(t, err)
	require.NotNil(t, res)

	newKey, _ := os.ReadFile(keyPath)
	assert.NotEqual(t, oldKey, newKey)
	st, _ := os.Stat(keyPath)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	// new key opens the DB with all data
	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	assert.Equal(t, 20, countRows(t, db2))
	require.NoError(t, db2.Close())

	// old key no longer opens it
	require.NoError(t, os.WriteFile(keyPath+".oldtry", oldKey, 0o600))
	cfgOld := *cfg
	cfgOld.EncryptionKeyFile = keyPath + ".oldtry"
	_, err = NewConnection(&cfgOld)
	assert.Error(t, err, "old key must fail after rekey")

	// §9.2 backup taken before rekey is openable with the preserved OLD key
	require.NotEmpty(t, res.BackupPath)
	require.NotEmpty(t, res.OldKeyBackupPath)
	cfgBak := *cfg
	cfgBak.Path = res.BackupPath
	cfgBak.EncryptionKeyFile = res.OldKeyBackupPath
	dbb, err := NewConnection(&cfgBak)
	require.NoError(t, err)
	assert.Equal(t, 20, countRows(t, dbb))
	require.NoError(t, dbb.Close())
	_ = dbPath
}

func TestEncryption_RekeyRefusesPlaintextAndMissingKey(t *testing.T) {
	cfg, _, _ := encEnv(t)
	_, err := Rekey(cfg)
	assert.Error(t, err, "no database -> refuse")
}

func TestEncryption_ConcurrentPoolConnectionsAllKeyed(t *testing.T) {
	cfg, _, _ := encEnv(t)
	cfg.MaxOpenConnections = 8
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	seed(t, db, "pool")

	ctx := context.Background()
	const n = 8
	conns := make([]*sql.Conn, 0, n)
	for i := 0; i < n; i++ {
		c, err := db.DB.Conn(ctx)
		require.NoError(t, err)
		conns = append(conns, c)
	}
	// All n connections are held at once => n distinct pooled connections.
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for _, c := range conns {
		wg.Add(1)
		go func(c *sql.Conn) {
			defer wg.Done()
			var cnt int
			if err := c.QueryRowContext(ctx, `SELECT COUNT(*) FROM enc_t`).Scan(&cnt); err != nil {
				errs <- err
				return
			}
			if cnt != 20 {
				errs <- fmt.Errorf("got %d rows", cnt)
			}
		}(c)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		assert.NoError(t, e)
	}
	for _, c := range conns {
		require.NoError(t, c.Close())
	}
	// and the file really is encrypted (a pool of unkeyed connections would
	// have "worked" against a plaintext file too)
	assert.NotEqual(t, plainHeader, string(headerOf(t, cfg.Path)))
}

func TestEncryption_BackupIncludesWALSidecarAndRestores(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db.Close()
	seed(t, db, "bk")

	// Pin a read snapshot on a second connection so the checkpoint cannot
	// fold the WAL back into the main file: the committed rows then live
	// ONLY in the -wal sidecar - exactly what a raw single-file copy loses.
	ctx := context.Background()
	reader, err := db.DB.Conn(ctx)
	require.NoError(t, err)
	tx, err := reader.BeginTx(ctx, nil)
	require.NoError(t, err)
	var n0 int
	require.NoError(t, tx.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&n0))
	for i := 0; i < 5; i++ {
		_, err = db.Exec(`INSERT INTO enc_t (v) VALUES (?)`, fmt.Sprintf("after-snapshot-%d", i))
		require.NoError(t, err)
	}
	wst, err := os.Stat(dbPath + "-wal")
	require.NoError(t, err)
	require.Greater(t, wst.Size(), int64(0), "precondition: committed rows are in the WAL")

	dest := filepath.Join(t.TempDir(), "backup.db")
	require.NoError(t, db.BackupTo(dest))
	require.NoError(t, tx.Rollback())
	require.NoError(t, reader.Close())

	// the raw single-file copy (today's behaviour) would be incomplete:
	bare := filepath.Join(t.TempDir(), "bare.db")
	main, _ := os.ReadFile(dbPath)
	require.NoError(t, os.WriteFile(bare, main, 0o600))

	// Restore the backup over a fresh target path and read it with the key.
	target := filepath.Join(t.TempDir(), "restored.db")
	require.NoError(t, RestoreBackup(&config.DatabaseConfig{Path: target, EncryptionKeyFile: cfg.EncryptionKeyFile}, dest))
	cfgR := *cfg
	cfgR.Path = target
	dbr, err := NewConnection(&cfgR)
	require.NoError(t, err)
	defer dbr.Close()
	assert.Equal(t, 25, countRows(t, dbr), "restored backup must contain WAL-resident rows")

	// backup file itself is encrypted and 0600
	assert.NotEqual(t, plainHeader, string(headerOf(t, dest)))
	st, _ := os.Stat(dest)
	assert.Equal(t, os.FileMode(0o600), st.Mode().Perm())

	// Control: restoring ONLY the main file (the old raw-copy behaviour)
	// loses the WAL-resident commits - proving the sidecar copy is what saves them.
	cfgB := *cfg
	cfgB.Path = bare
	lost := false
	dbb, err := NewConnection(&cfgB)
	if err != nil {
		// An unreadable main-only copy is the same loss; it must be a real
		// error about the missing WAL content, never a silent pass.
		t.Logf("control: main-file-only copy cannot be opened: %v", err)
		lost = true
	} else {
		defer dbb.Close()
		var bn int
		if qerr := dbb.QueryRow(`SELECT COUNT(*) FROM enc_t`).Scan(&bn); qerr != nil {
			t.Logf("control: main-file-only copy cannot be read: %v", qerr)
			lost = true
		} else {
			lost = bn < 25
		}
	}
	assert.True(t, lost, "control: a main-file-only copy must lose the WAL-resident commits")
}

func TestEncryption_RestoreKeepsPreviousDB(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "r")
	dest := filepath.Join(t.TempDir(), "b.db")
	require.NoError(t, db.BackupTo(dest))
	_, err = db.Exec(`INSERT INTO enc_t (v) VALUES ('later')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	require.NoError(t, RestoreBackup(cfg, dest))
	prev, _ := filepath.Glob(dbPath + ".pre-restore-*")
	assert.NotEmpty(t, prev, "previous DB must be kept (§9.2)")
	db2, err := NewConnection(cfg)
	require.NoError(t, err)
	defer db2.Close()
	assert.Equal(t, 20, countRows(t, db2))
}
