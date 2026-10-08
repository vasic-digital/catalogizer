package database

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// WF22 R3: migration v21 adds storage_roots.allow_empty (default false). The PostgreSQL branch (ADD COLUMN IF NOT EXISTS) cannot run in this
// environment (no PostgreSQL server); only the SQLite branch is executed here.

func TestMigrationV21_AllowEmptyColumnDefaultsToFalseForExistingAndNewRoots(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	has, err := db.columnExistsSQLite(ctx, "storage_roots", "allow_empty")
	require.NoError(t, err)
	assert.True(t, has, "storage_roots.allow_empty exists after the migration chain")

	_, err = raw.Exec(`INSERT INTO storage_roots (name, protocol) VALUES ('plain', 'ftp')`)
	require.NoError(t, err)
	var v bool
	require.NoError(t, raw.QueryRow(`SELECT allow_empty FROM storage_roots WHERE name = 'plain'`).Scan(&v))
	assert.False(t, v, "default: an empty root is a failure")

	_, err = raw.Exec(`INSERT INTO storage_roots (name, protocol, allow_empty) VALUES ('empty-ok', 'ftp', 1)`)
	require.NoError(t, err)
	require.NoError(t, raw.QueryRow(`SELECT allow_empty FROM storage_roots WHERE name = 'empty-ok'`).Scan(&v))
	assert.True(t, v)
}

func TestMigrationV21_UpgradesAV20TableKeepingItsRowsAndIsIdempotent(t *testing.T) {
	// A storage_roots table as it was at v20 (no allow_empty) holding a row. (The SQLite bundled with go-sqlcipher predates DROP COLUMN, so the
	// old shape is built directly instead of removing the column from a migrated database.)
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	db := WrapDB(raw, DialectSQLite)
	ctx := context.Background()
	_, err = raw.Exec(`CREATE TABLE storage_roots (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE, protocol TEXT NOT NULL, host TEXT)`)
	require.NoError(t, err)
	_, err = raw.Exec(`INSERT INTO storage_roots (name, protocol, host) VALUES ('old', 'smb', 'nas')`)
	require.NoError(t, err)
	has, err := db.columnExistsSQLite(ctx, "storage_roots", "allow_empty")
	require.NoError(t, err)
	require.False(t, has, "control: the v20 table has no allow_empty")

	require.NoError(t, db.addStorageRootsAllowEmptyColumn(ctx))
	has, err = db.columnExistsSQLite(ctx, "storage_roots", "allow_empty")
	require.NoError(t, err)
	assert.True(t, has)
	var host string
	var v bool
	require.NoError(t, raw.QueryRow(`SELECT host, allow_empty FROM storage_roots WHERE name = 'old'`).Scan(&host, &v))
	assert.Equal(t, "nas", host)
	assert.False(t, v, "a pre-existing root keeps the strict behaviour")

	require.NoError(t, db.addStorageRootsAllowEmptyColumn(ctx), "running the migration again is a no-op")
}

func TestMigrationV21_IsRegisteredAndRecorded(t *testing.T) {
	db, _ := newMigratedDB(t)
	var name string
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT name FROM migrations WHERE version = 21").Scan(&name))
	assert.Equal(t, "storage_roots_allow_empty", name)
}
