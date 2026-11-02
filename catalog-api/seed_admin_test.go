package main

import (
	"bytes"
	root_config "catalogizer/config"
	"database/sql"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"testing"

	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func seedTestDB(t *testing.T) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	_, err = sqlDB.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, username TEXT, email TEXT, password_hash TEXT, salt TEXT, role_id INTEGER, first_name TEXT, last_name TEXT, display_name TEXT, is_active INTEGER)`)
	require.NoError(t, err)
	_, err = sqlDB.Exec(`CREATE TABLE user_sessions (id INTEGER PRIMARY KEY, user_id INTEGER, session_token TEXT, refresh_token TEXT, is_active INTEGER DEFAULT 1)`)
	require.NoError(t, err)
	return database.WrapDB(sqlDB, database.DialectSQLite)
}

// verifyStored mirrors services.AuthService.verifyPassword: bcrypt(password+salt).
func verifyStored(t *testing.T, db *database.DB, user, password string) bool {
	t.Helper()
	var hash, salt string
	require.NoError(t, db.QueryRow("SELECT password_hash, COALESCE(salt,'') FROM users WHERE username = ?", user).Scan(&hash, &salt))
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+salt)) == nil
}

func addAdmin(t *testing.T, db *database.DB, user, password string) int64 {
	t.Helper()
	salt := "salt-" + user
	h, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.MinCost)
	require.NoError(t, err)
	res, err := db.Exec(`INSERT INTO users (username,email,password_hash,salt,role_id,first_name,last_name,display_name,is_active) VALUES (?,?,?,?,1,'a','b','c',1)`, user, user+"@x", string(h), salt)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	return id
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

// I3: sessions/refresh tokens issued before the reset are revoked by it.
func TestAdminReset_RevokesExistingSessions(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	db := seedTestDB(t)
	id := addAdmin(t, db, "admin", seedCompromised)
	other := addAdmin(t, db, "other", "Other-Strong-Pw-4k")
	for _, u := range []int64{id, other} {
		// explicit session id deliberately != user id so a revoke keyed on the wrong column cannot pass by coincidence
		_, err := db.Exec("INSERT INTO user_sessions (id, user_id, session_token, refresh_token) VALUES (?, ?, 'st', 'rt')", 1000+u*7, u)
		require.NoError(t, err)
	}
	t.Setenv("ADMIN_NEW_PASSWORD", "Rotated-Op3rator-Value-7q")
	var out, errb bytes.Buffer
	require.Equal(t, 0, runAdminReset(db, []string{"admin-reset-password"}, &out, &errb))
	var active int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1", id).Scan(&active))
	assert.Equal(t, 0, active, "pre-reset sessions must be revoked")
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = 1", other).Scan(&active))
	assert.Equal(t, 1, active, "unrotated account's sessions untouched")
}

// T1(a): after reset the NEW password verifies against stored hash+salt, with and without a username.
func TestAdminReset_NewPasswordVerifies(t *testing.T) {
	const newPw = "Rotated-Op3rator-Value-7q"
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	for _, args := range [][]string{{"admin-reset-password"}, {"admin-reset-password", "admin"}} {
		db := seedTestDB(t)
		addAdmin(t, db, "admin", seedCompromised)
		t.Setenv("ADMIN_NEW_PASSWORD", newPw)
		var out, errb bytes.Buffer
		require.Equal(t, 0, runAdminReset(db, args, &out, &errb))
		assert.True(t, verifyStored(t, db, "admin", newPw), "args %v", args)
		assert.False(t, verifyStored(t, db, "admin", seedCompromised), "args %v", args)
		var salt string
		require.NoError(t, db.QueryRow("SELECT salt FROM users WHERE username='admin'").Scan(&salt))
		assert.NotEqual(t, "salt-admin", salt, "salt must be regenerated")
	}
}

// T1(c)/R1: only the named admin is rotated, even when others are compromised too.
func TestAdminReset_OnlyNamedAdminRotated(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	db := seedTestDB(t)
	addAdmin(t, db, "alice", seedCompromised)
	addAdmin(t, db, "bob", seedCompromised)
	t.Setenv("ADMIN_NEW_PASSWORD", "Rotated-Op3rator-Value-7q")
	var out, errb bytes.Buffer
	require.Equal(t, 0, runAdminReset(db, []string{"admin-reset-password", "bob"}, &out, &errb))
	assert.True(t, verifyStored(t, db, "bob", "Rotated-Op3rator-Value-7q"))
	assert.True(t, verifyStored(t, db, "alice", seedCompromised), "alice must be untouched")
}

// M7: named account missing or not an admin exits non-zero.
func TestAdminReset_UnknownOrNonAdminNamedExitsNonZero(t *testing.T) {
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	db := seedTestDB(t)
	addAdmin(t, db, "alice", seedCompromised)
	_, err := db.Exec(`INSERT INTO users (username,email,password_hash,salt,role_id,first_name,last_name,display_name,is_active) VALUES ('norm','n@x','h','s',2,'a','b','c',1)`)
	require.NoError(t, err)
	t.Setenv("ADMIN_NEW_PASSWORD", "Rotated-Op3rator-Value-7q")
	for _, u := range []string{"nobody", "norm"} {
		var out, errb bytes.Buffer
		assert.Equal(t, 1, runAdminReset(db, []string{"admin-reset-password", u}, &out, &errb), u)
	}
}

// R4: rows.Err inside runAdminReset must fail the command.
func TestAdminReset_RowsErrFails(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()
	mock.ExpectQuery("SELECT id, username").WillReturnRows(
		sqlmock.NewRows([]string{"id", "username", "password_hash", "salt"}).AddRow(1, "admin", "x", "").RowError(0, errors.New("boom")))
	t.Setenv("ADMIN_NEW_PASSWORD", "Rotated-Op3rator-Value-7q")
	var out, errb bytes.Buffer
	assert.Equal(t, 1, runAdminReset(database.WrapDB(sqlDB, database.DialectSQLite), []string{"admin-reset-password"}, &out, &errb))
}
