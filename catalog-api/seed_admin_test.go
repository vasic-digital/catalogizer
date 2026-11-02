package main

import (
	"database/sql"
	"errors"
	"testing"

	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTestDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	_, err = sqlDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, email TEXT, password_hash TEXT, salt TEXT, role_id INTEGER, first_name TEXT, last_name TEXT, display_name TEXT, is_active INTEGER)`)
	require.NoError(t, err)
	return database.WrapDB(sqlDB, database.DialectSQLite)
}

const seedCompromised = "admin" + "123"

// F10: never seed an empty or placeholder password.
func TestSeedDefaultAdmin_RefusesEmptyAndPlaceholder(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	for _, p := range []string{"", seedCompromised, "<ADMIN_PASSWORD>"} {
		db := seedTestDB(t)
		err := seedDefaultAdmin(db, "admin", p)
		require.Error(t, err, "password %q", p)
		var n int
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM users").Scan(&n))
		assert.Equal(t, 0, n, "no row may be seeded for %q", p)
	}
}

func TestSeedDefaultAdmin_SeedsOperatorValue(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	db := seedTestDB(t)
	require.NoError(t, seedDefaultAdmin(db, "admin", "Op3rator-Supplied-Value-9x"))
	require.NoError(t, seedDefaultAdmin(db, "admin", "Op3rator-Supplied-Value-9x"))
}

// F2: an existing admin whose stored hash matches a denylisted password.
func TestSeedDefaultAdmin_ExistingCompromisedAdminRefused(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "true")
	db := seedTestDB(t)
	require.NoError(t, seedDefaultAdmin(db, "admin", seedCompromised))
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	err := seedDefaultAdmin(db, "admin", "Op3rator-Supplied-Value-9x")
	require.Error(t, err)
	assert.True(t, errors.Is(err, errCompromisedAdmin))
}
