package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type sqlQuerier struct{ db *sql.DB }

func (q sqlQuerier) QueryRowContext(ctx context.Context, query string, args ...interface{}) rowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

// Real sqlite: only an active, unexpired session owned by the token subject passes.
func TestSessionChecker_RealSQLite(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE user_sessions (id INTEGER PRIMARY KEY, user_id INTEGER, is_active BOOLEAN, expires_at DATETIME)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, is_active BOOLEAN, is_locked BOOLEAN DEFAULT 0, locked_until DATETIME)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO users (id, is_active) VALUES (3, 1), (4, 1)`)
	require.NoError(t, err)
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	for _, r := range []struct {
		id, uid int
		act     bool
		exp     time.Time
	}{{11, 3, true, future}, {12, 3, false, future}, {13, 3, true, past}} {
		_, err = db.Exec(`INSERT INTO user_sessions VALUES (?,?,?,?)`, r.id, r.uid, r.act, r.exp)
		require.NoError(t, err)
	}
	chk := sessionCheckerFor(sqlQuerier{db})
	ctx := context.Background()
	assert.NoError(t, chk(ctx, "11", "3"))
	assert.Error(t, chk(ctx, "12", "3"), "revoked")
	assert.Error(t, chk(ctx, "13", "3"), "expired")
	assert.Error(t, chk(ctx, "11", "4"), "owner mismatch")
	assert.Error(t, chk(ctx, "99", "3"), "missing row")
	assert.Error(t, chk(ctx, "abc", "3"), "bad id")
	db.Close()
	assert.Error(t, chk(ctx, "11", "3"), "DB error must fail closed")
}

// The owner's account state is part of the check: a deactivated, suspended or
// still-locked user has no usable session even if the session row is active;
// an EXPIRED timed lock does not reject. Real sqlite, no doubles.
func TestSessionChecker_AccountState(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE user_sessions (id INTEGER PRIMARY KEY, user_id INTEGER, is_active BOOLEAN, expires_at DATETIME)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, is_active BOOLEAN, is_locked BOOLEAN DEFAULT 0, locked_until DATETIME)`)
	require.NoError(t, err)
	future, past := time.Now().Add(time.Hour), time.Now().Add(-time.Hour)
	rows := []struct {
		id     int
		active bool
		locked bool
		until  *time.Time
	}{{1, true, false, nil}, {2, false, false, nil}, {3, true, true, nil}, {4, true, true, &future}, {5, true, true, &past}}
	for _, r := range rows {
		_, err = db.Exec(`INSERT INTO users (id, is_active, is_locked, locked_until) VALUES (?,?,?,?)`, r.id, r.active, r.locked, r.until)
		require.NoError(t, err)
		_, err = db.Exec(`INSERT INTO user_sessions VALUES (?,?,1,?)`, r.id*10, r.id, future)
		require.NoError(t, err)
	}
	chk := sessionCheckerFor(sqlQuerier{db})
	ctx := context.Background()
	assert.NoError(t, chk(ctx, "10", "1"), "normal user")
	assert.Error(t, chk(ctx, "20", "2"), "deactivated user")
	assert.Error(t, chk(ctx, "30", "3"), "permanent suspension")
	assert.Error(t, chk(ctx, "40", "4"), "timed lock still running")
	assert.NoError(t, chk(ctx, "50", "5"), "expired timed lock does not reject")
}
