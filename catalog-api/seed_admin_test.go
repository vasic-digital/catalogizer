package main

import (
	"bytes"
	root_config "catalogizer/config"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"os"
	"os/exec"
	"path/filepath"
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

// M1: a failed hash check must fail closed (not silently allow startup).
func TestRefuseCompromisedAdminHashes_FailsClosedOnQueryError(t *testing.T) {
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite) // no users table
	err = refuseCompromisedAdminHashes(db)
	require.Error(t, err)
	assert.True(t, errors.Is(err, errAdminHashCheck))
	require.NoError(t, sqlDB.Close())
	assert.True(t, errors.Is(refuseCompromisedAdminHashes(db), errAdminHashCheck))
}

// M4/M5: every denylist candidate is detected on an existing account.
func TestRefuseCompromisedAdminHashes_EveryCandidateDetected(t *testing.T) {
	for _, c := range root_config.PlaceholderPasswordCandidates() {
		db := seedTestDB(t)
		t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "true")
		require.NoError(t, seedDefaultAdmin(db, "admin", c), c)
		t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
		assert.True(t, errors.Is(refuseCompromisedAdminHashes(db), errCompromisedAdmin), "candidate %q", c)
	}
	assert.Contains(t, root_config.PlaceholderPasswordCandidates(), "changeme")
}

// M4: the startup handler escalates to Fatal for both refusal classes only.
func TestHandleSeedAdminError(t *testing.T) {
	var fatalCalls, warnCalls int
	fatal := func(string, error) { fatalCalls++ }
	warn := func(string, error) { warnCalls++ }
	handleSeedAdminError(nil, fatal, warn)
	handleSeedAdminError(errCompromisedAdmin, fatal, warn)
	handleSeedAdminError(errAdminHashCheck, fatal, warn)
	handleSeedAdminError(errors.New("other"), fatal, warn)
	assert.Equal(t, 2, fatalCalls)
	assert.Equal(t, 1, warnCalls)
}

// I1: offline recovery rotates a compromised admin without printing secrets.
func TestAdminReset_RotatesCompromisedAdmin(t *testing.T) {
	const newPw = "Rotated-Op3rator-Value-7q"
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "true")
	db := seedTestDB(t)
	require.NoError(t, seedDefaultAdmin(db, "admin", seedCompromised))
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	require.True(t, errors.Is(seedDefaultAdmin(db, "admin", newPw), errCompromisedAdmin))

	t.Setenv("ADMIN_NEW_PASSWORD", newPw)
	var out, errb bytes.Buffer
	require.Equal(t, 0, runAdminReset(db, []string{"admin-reset-password"}, &out, &errb))
	assert.Contains(t, out.String(), "admin")
	assert.NotContains(t, out.String()+errb.String(), newPw)
	assert.NoError(t, seedDefaultAdmin(db, "admin", newPw), "server may start after rotation")
	// old password no longer matches
	assert.NoError(t, refuseCompromisedAdminHashes(db))
}

func TestAdminReset_RejectsMissingOrPlaceholderNewPassword(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	for _, v := range []string{"", seedCompromised, "<new>"} {
		db := seedTestDB(t)
		t.Setenv("ADMIN_NEW_PASSWORD", v)
		var out, errb bytes.Buffer
		assert.Equal(t, 2, runAdminReset(db, []string{"admin-reset-password"}, &out, &errb), "%q", v)
	}
}

// Real binary path: build the server and run the subcommand offline.
func TestAdminReset_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Skipf("cannot build binary here: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "admin-reset-password")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "ADMIN_NEW_PASSWORD=", "DATABASE_TYPE=sqlite",
		"DATABASE_PATH="+filepath.Join(dir, "x.db"), "JWT_SECRET=Zq7-test-only-jwt-secret-0123456789abcdef", "ADMIN_USERNAME=admin", "ADMIN_PASSWORD=Op3rator-Supplied-Value-9x")
	out, err := cmd.CombinedOutput()
	require.Error(t, err, "empty ADMIN_NEW_PASSWORD must be refused by the real binary: %s", out)
	assert.Contains(t, string(out), "ADMIN_NEW_PASSWORD")
}

// rows.Err must fail closed (mid-iteration error).
func TestRefuseCompromisedAdminHashes_FailsClosedOnRowsErr(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()
	mock.ExpectQuery("SELECT username").WillReturnRows(
		sqlmock.NewRows([]string{"username", "password_hash", "salt"}).
			AddRow("admin", "x", "").RowError(0, errors.New("boom")))
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	assert.True(t, errors.Is(refuseCompromisedAdminHashes(db), errAdminHashCheck))
}

// Reset must rotate an admin seeded with ANY denylisted candidate.
func TestAdminReset_EveryCandidateRotated(t *testing.T) {
	const newPw = "Rotated-Op3rator-Value-7q"
	for _, c := range root_config.PlaceholderPasswordCandidates() {
		t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "true")
		db := seedTestDB(t)
		require.NoError(t, seedDefaultAdmin(db, "admin", c), c)
		t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
		t.Setenv("ADMIN_NEW_PASSWORD", newPw)
		var out, errb bytes.Buffer
		require.Equal(t, 0, runAdminReset(db, []string{"admin-reset-password"}, &out, &errb))
		assert.Contains(t, out.String(), "rotated", "candidate %q", c)
		assert.NoError(t, refuseCompromisedAdminHashes(db), "candidate %q", c)
	}
}
