package database

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strings"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/internal/netguard"
)

// CAT-SECQ1-B (W3): migration v27 adds approved_class / approved_class_source to storage_roots and sync_endpoints.
// The PostgreSQL branch (ADD COLUMN IF NOT EXISTS) is not executed here: no PostgreSQL instance was available to this
// test run (see expectedPostgresSchema in migrations_postgres_integration_test.go, which asserts the columns there).

func insertRoot(t *testing.T, raw *sql.DB, name, host string) int64 {
	t.Helper()
	res, err := raw.Exec(`INSERT INTO storage_roots (name, protocol, host) VALUES (?, 'webdav', ?)`, name, host)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func insertSyncEndpoint(t *testing.T, raw *sql.DB, name, url string) int64 {
	t.Helper()
	_, err := raw.Exec(`INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, role_id) VALUES (1, 'u', 'u@x', 'h', 's', 1)`)
	if err != nil {
		t.Logf("users seed: %v (continuing: FK enforcement may be off)", err)
	}
	res, err := raw.Exec(`INSERT INTO sync_endpoints (user_id, name, type, url) VALUES (1, ?, 'webdav', ?)`, name, url)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func TestMigrationV27_ColumnsDefaultsAndChecksOnBothTables(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	rootID := insertRoot(t, raw, "r", "nas.lan")
	epID := insertSyncEndpoint(t, raw, "e", "https://nas.lan/dav")
	for _, c := range []struct {
		table string
		id    int64
	}{{TableStorageRoots, rootID}, {TableSyncEndpoints, epID}} {
		for _, col := range []string{"approved_class", "approved_class_source"} {
			has, err := db.columnExistsSQLite(ctx, c.table, col)
			require.NoError(t, err)
			assert.True(t, has, "%s.%s exists after the migration chain", c.table, col)
		}
		cls, src, err := db.GetApprovedClass(ctx, c.table, c.id)
		require.NoError(t, err)
		assert.Equal(t, netguard.ClassPrivate, cls, "%s: default keeps the pre-SECQ-1 behaviour", c.table)
		assert.Equal(t, ApprovedSourceDefault, src)
		// the CHECK constraints refuse values the code can not interpret
		_, err = raw.Exec(`UPDATE `+c.table+` SET approved_class = 'never' WHERE id = ?`, c.id)
		assert.Error(t, err, "%s: 'never' must not be storable", c.table)
		_, err = raw.Exec(`UPDATE `+c.table+` SET approved_class = 'bogus' WHERE id = ?`, c.id)
		assert.Error(t, err)
		_, err = raw.Exec(`UPDATE `+c.table+` SET approved_class_source = 'bogus' WHERE id = ?`, c.id)
		assert.Error(t, err)
	}
}

// stripApprovedClassColumns rebuilds table from its own sqlite_master DDL without the two v27 columns (the SQLCipher build
// has no DROP COLUMN), replaying its indexes: the result is the table as migrations 1..26 left it.
func stripApprovedClassColumns(t *testing.T, raw *sql.DB, table string) {
	t.Helper()
	var ddl string
	require.NoError(t, raw.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&ddl))
	i := strings.Index(ddl, ", approved_class TEXT")
	require.True(t, i > 0, "v27 columns not found in the DDL of %s: %s", table, ddl)
	require.True(t, strings.Contains(ddl[i:], "approved_class_source"), ddl)
	v26 := ddl[:i] + ")"
	rows, err := raw.Query(`SELECT sql FROM sqlite_master WHERE type='index' AND tbl_name=? AND sql IS NOT NULL`, table)
	require.NoError(t, err)
	var idx []string
	for rows.Next() {
		var q string
		require.NoError(t, rows.Scan(&q))
		idx = append(idx, q)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	for _, q := range append([]string{`PRAGMA foreign_keys=OFF`, `DROP TABLE ` + table, v26}, idx...) {
		_, err := raw.Exec(q)
		require.NoError(t, err, q)
	}
}

// F3 (review round 1): the upgrade is exercised through the REAL chain. The whole chain runs, the two tables are rebuilt
// without the v27 columns (the v26 shape, indexes included), migration row 27 is deleted, rows are inserted through the
// real schema, and RunMigrations runs again.
func TestMigrationV27_UpgradesV26TablesKeepingRowsAndIsIdempotent(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	for _, tb := range approvedClassTables {
		stripApprovedClassColumns(t, raw, tb)
		for _, col := range []string{"approved_class", "approved_class_source"} {
			has, err := db.columnExistsSQLite(ctx, tb, col)
			require.NoError(t, err)
			require.False(t, has, "control: the v26-shape %s has no %s", tb, col)
		}
	}
	_, err := raw.Exec(`DELETE FROM migrations WHERE version = 27`)
	require.NoError(t, err)
	var gone int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 27`).Scan(&gone))
	require.Equal(t, 0, gone, "control: migration 27 is not recorded")

	rid := insertRoot(t, raw, "old", "nas")
	eid := insertSyncEndpoint(t, raw, "oldep", "https://x/")

	require.NoError(t, db.RunMigrations(ctx))
	for _, c := range []struct {
		tb string
		id int64
	}{{TableStorageRoots, rid}, {TableSyncEndpoints, eid}} {
		cls, src, err := db.GetApprovedClass(ctx, c.tb, c.id)
		require.NoError(t, err)
		assert.Equal(t, netguard.ClassPrivate, cls, "%s: a pre-existing row keeps the private approval", c.tb)
		assert.Equal(t, ApprovedSourceDefault, src)
	}
	var host string
	require.NoError(t, raw.QueryRow(`SELECT host FROM storage_roots WHERE id = ?`, rid).Scan(&host))
	assert.Equal(t, "nas", host, "existing data untouched")
	var n, maxv int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*), MAX(version) FROM migrations`).Scan(&n, &maxv))
	assert.Equal(t, 28, maxv)

	// a stored approval survives the next run, which is a no-op (no new migration row, same data)
	_, err = raw.Exec(`UPDATE storage_roots SET approved_class = 'public', approved_class_source = 'operator' WHERE id = ?`, rid)
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(ctx), "the second run is a no-op")
	require.NoError(t, db.addApprovedClassColumns(ctx), "the migration body itself is idempotent (columns already there)")
	var n2, maxv2 int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*), MAX(version) FROM migrations`).Scan(&n2, &maxv2))
	assert.Equal(t, n, n2)
	assert.Equal(t, maxv, maxv2)
	cls, src, err := db.GetApprovedClass(ctx, TableStorageRoots, rid)
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, cls)
	assert.Equal(t, ApprovedSourceOperator, src)
}

func TestMigrationV27_IsRegisteredAndRecorded(t *testing.T) {
	db, _ := newMigratedDB(t)
	var name string
	require.NoError(t, db.QueryRowContext(context.Background(), "SELECT name FROM migrations WHERE version = 27").Scan(&name))
	assert.Equal(t, "add_endpoint_approved_class", name)
	// the whole chain run twice is a no-op
	require.NoError(t, db.RunMigrations(context.Background()))
}

func TestApprovedClassAccessorsValidate(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	id := insertRoot(t, raw, "r", "nas.lan")

	require.NoError(t, db.SetApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPublic, ApprovedSourceOperator))
	cls, src, err := db.GetApprovedClass(ctx, TableStorageRoots, id)
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, cls)
	assert.Equal(t, ApprovedSourceOperator, src)

	for _, bad := range []netguard.Class{netguard.ClassNever, netguard.Class(9)} {
		err := db.SetApprovedClass(ctx, TableStorageRoots, id, bad, ApprovedSourceAuto)
		assert.True(t, errors.Is(err, ErrApprovedClassInvalid), "class %v must be refused: %v", bad, err)
	}
	assert.True(t, errors.Is(db.SetApprovedClass(ctx, TableStorageRoots, id, netguard.ClassPrivate, "bogus"), ErrApprovedClassInvalid))
	assert.True(t, errors.Is(db.SetApprovedClass(ctx, "users; DROP TABLE users", id, netguard.ClassPrivate, ApprovedSourceAuto), ErrApprovedClassInvalid))
	_, _, err = db.GetApprovedClass(ctx, "files", id)
	assert.True(t, errors.Is(err, ErrApprovedClassInvalid))
	assert.True(t, errors.Is(db.SetApprovedClass(ctx, TableStorageRoots, 9999, netguard.ClassPrivate, ApprovedSourceAuto), ErrEndpointNotFound))
	_, _, err = db.GetApprovedClass(ctx, TableSyncEndpoints, 9999)
	assert.True(t, errors.Is(err, ErrEndpointNotFound))
	// a refused write changed nothing
	cls, _, err = db.GetApprovedClass(ctx, TableStorageRoots, id)
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, cls)
}

func TestAutoClassifyApprovedClass(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	t.Setenv(netguard.AllowCIDRsEnv, "")
	t.Setenv(netguard.DenyPrivateEnv, "")

	lan := insertRoot(t, raw, "lan", "192.168.7.20")
	pub := insertRoot(t, raw, "pub", "93.184.216.34")
	loop := insertRoot(t, raw, "loop", "127.0.0.1")
	op := insertRoot(t, raw, "op", "93.184.216.34")
	ep := insertSyncEndpoint(t, raw, "ep", "https://93.184.216.34/dav")
	require.NoError(t, db.SetApprovedClass(ctx, TableStorageRoots, op, netguard.ClassPrivate, ApprovedSourceOperator))

	// the host's own interface list would classify a LAN address that is the machine itself as never; the test addresses
	// are not this machine's, which the control below asserts instead of assuming.
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipn, ok := a.(*net.IPNet); ok && (ipn.IP.String() == "192.168.7.20" || ipn.IP.String() == "93.184.216.34") {
			t.Skip("test address is an interface address of this machine")
		}
	}

	c, err := db.AutoClassifyApprovedClass(ctx, TableStorageRoots, lan, "192.168.7.20", "")
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPrivate, c)
	c, err = db.AutoClassifyApprovedClass(ctx, TableStorageRoots, pub, "93.184.216.34", "")
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, c)
	cls, src, err := db.GetApprovedClass(ctx, TableStorageRoots, pub)
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, cls)
	assert.Equal(t, ApprovedSourceAuto, src)

	// URL fallback on a sync endpoint
	c, err = db.AutoClassifyApprovedClass(ctx, TableSyncEndpoints, ep, "", "https://93.184.216.34/dav")
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPublic, c)

	// never-class host: error, nothing written, row keeps default
	_, err = db.AutoClassifyApprovedClass(ctx, TableStorageRoots, loop, "127.0.0.1", "")
	assert.True(t, errors.Is(err, netguard.ErrBlocked))
	cls, src, err = db.GetApprovedClass(ctx, TableStorageRoots, loop)
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPrivate, cls)
	assert.Equal(t, ApprovedSourceDefault, src, "a failed classification writes nothing")

	// an operator decision is never overwritten by automatic classification
	c, err = db.AutoClassifyApprovedClass(ctx, TableStorageRoots, op, "93.184.216.34", "")
	require.NoError(t, err)
	assert.Equal(t, netguard.ClassPrivate, c, "returns the operator's class")
	cls, src, _ = db.GetApprovedClass(ctx, TableStorageRoots, op)
	assert.Equal(t, netguard.ClassPrivate, cls)
	assert.Equal(t, ApprovedSourceOperator, src)

	_, err = db.AutoClassifyApprovedClass(ctx, TableStorageRoots, 9999, "192.168.7.20", "")
	assert.True(t, errors.Is(err, ErrEndpointNotFound))
}
