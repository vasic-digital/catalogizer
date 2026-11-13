package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGIN-LOCK-1: migration v28 creates login_throttle through the real RunMigrations chain. The PostgreSQL branch uses the
// same portable DDL (TEXT/INTEGER/BIGINT, IF NOT EXISTS); it is asserted in expectedPostgresSchema but not executed here (no
// PostgreSQL instance was available to this run).
func TestMigrationV28_LoginThrottleTable(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	var applied int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM migrations WHERE version = 28`).Scan(&applied))
	assert.Equal(t, 1, applied)

	_, err := raw.Exec(`INSERT INTO login_throttle (subject, scope) VALUES ('u:1', 'account')`)
	require.NoError(t, err)
	var failures int
	var blocked, lastFail, lastOK, version int64
	require.NoError(t, raw.QueryRow(`SELECT failures, blocked_until_ms, last_failure_ms, last_success_ms, version FROM login_throttle WHERE subject='u:1' AND scope='account'`).
		Scan(&failures, &blocked, &lastFail, &lastOK, &version))
	assert.Zero(t, failures+int(blocked+lastFail+lastOK+version), "every counter defaults to 0")
	_, err = raw.Exec(`INSERT INTO login_throttle (subject, scope) VALUES ('u:1', 'account')`)
	assert.Error(t, err, "(subject, scope) is the primary key")
	_, err = raw.Exec(`INSERT INTO login_throttle (subject, scope) VALUES (NULL, 'account')`)
	assert.Error(t, err, "subject is NOT NULL")

	// idempotent: re-running the migration body and the whole chain changes nothing and keeps the rows
	require.NoError(t, db.createLoginThrottleTable(ctx))
	require.NoError(t, db.RunMigrations(ctx))
	var rows int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM login_throttle`).Scan(&rows))
	assert.Equal(t, 1, rows)
}

// A database that stopped at v27 (the previous release) gains the table when the chain runs again.
func TestMigrationV28_UpgradeFromV27(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()
	_, err := raw.Exec(`DROP TABLE login_throttle`)
	require.NoError(t, err)
	_, err = raw.Exec(`DELETE FROM migrations WHERE version = 28`)
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(ctx))
	var n int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='login_throttle'`).Scan(&n))
	assert.Equal(t, 1, n)
}
