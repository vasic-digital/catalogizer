package main

import (
	"os"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-001: the offline `db-rekey` / `db-restore` subcommands work end to end
// against a real SQLCipher database.
func TestRunDBMaintenance_RekeyAndRestore(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.DatabaseConfig{
		Type:              "sqlite",
		Path:              filepath.Join(dir, "m.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"),
	}
	db, err := database.NewConnection(cfg)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE t (v TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO t (v) VALUES ('a'), ('b')`)
	require.NoError(t, err)
	backup := filepath.Join(dir, "b.db")
	require.NoError(t, db.BackupTo(backup))
	require.NoError(t, db.Close())

	oldKey, _ := os.ReadFile(cfg.EncryptionKeyFile)
	assert.Equal(t, 0, runDBMaintenance(cfg, []string{"db-rekey"}))
	newKey, _ := os.ReadFile(cfg.EncryptionKeyFile)
	assert.NotEqual(t, oldKey, newKey)

	// restore of a pre-rekey backup needs the OLD key: with the new key it must
	// fail closed (exit 1) and leave the current database untouched.
	assert.Equal(t, 1, runDBMaintenance(cfg, []string{"db-restore", backup}))
	pre, _ := filepath.Glob(cfg.Path + ".pre-restore-*")
	assert.Empty(t, pre, "a refused restore must not move the database aside")
	db2, err := database.NewConnection(cfg)
	require.NoError(t, err, "original database must still open with the active key")
	var n int
	require.NoError(t, db2.QueryRow(`SELECT COUNT(*) FROM t`).Scan(&n))
	assert.Equal(t, 2, n)
	require.NoError(t, db2.Close())

	assert.Equal(t, 2, runDBMaintenance(cfg, []string{"db-restore"}))
	cfgPG := *cfg
	cfgPG.Type = "postgres"
	assert.Equal(t, 2, runDBMaintenance(&cfgPG, []string{"db-rekey"}))
}
