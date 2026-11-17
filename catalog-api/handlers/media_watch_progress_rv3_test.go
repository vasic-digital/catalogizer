package handlers

// WPROG3 follow-up F1: reviewer-authored interleaving tests adopted as real named tests (CAT WPROG3-R3-F1).
// File-backed WAL SQLite, real connection pool, real handler, real PlaybackSessionRepository.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"catalogizer/database"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	rv3Sel = "SELECT position_unit, duration_total FROM media_progress WHERE user_id = ?"
	rv3Upd = "UPDATE media_progress SET duration_total = ?, last_position = ?"
	rv3Ins = "VALUES (?, ?, 'seconds', ?, ?, ?)"
)

var (
	rv3Mu     sync.Mutex
	rv3Hooks  = map[string][]func(){} // key: needle; each matching statement pops the next hook (nil = no-op)
	rv3Counts = map[string]int{}
)

func rv3Fire(q string) {
	for _, k := range []string{rv3Sel, rv3Upd, rv3Ins} {
		if strings.Contains(q, k) {
			rv3Mu.Lock()
			rv3Counts[k]++
			var h func()
			if l := rv3Hooks[k]; len(l) > 0 {
				h, rv3Hooks[k] = l[0], l[1:]
			}
			rv3Mu.Unlock()
			if h != nil {
				h()
			}
		}
	}
}

type rv3Conn struct{ *sqlite3.SQLiteConn }

func (c rv3Conn) Exec(q string, a []driver.Value) (driver.Result, error) {
	rv3Fire(q)
	return c.SQLiteConn.Exec(q, a)
}
func (c rv3Conn) Query(q string, a []driver.Value) (driver.Rows, error) {
	rv3Fire(q)
	return c.SQLiteConn.Query(q, a)
}

type rv3Driver struct{ sqlite3.SQLiteDriver }

func (d *rv3Driver) Open(dsn string) (driver.Conn, error) {
	c, err := d.SQLiteDriver.Open(dsn)
	if err != nil {
		return nil, err
	}
	return rv3Conn{c.(*sqlite3.SQLiteConn)}, nil
}

func init() { sql.Register("sqlite3_rv3", &rv3Driver{}) }

func rv3Reset() {
	rv3Mu.Lock()
	rv3Hooks = map[string][]func(){}
	rv3Counts = map[string]int{}
	rv3Mu.Unlock()
}

func rv3Arm(k string, hs ...func()) {
	rv3Mu.Lock()
	rv3Hooks[k] = append(rv3Hooks[k], hs...)
	rv3Mu.Unlock()
}

func rv3Count(k string) int { rv3Mu.Lock(); defer rv3Mu.Unlock(); return rv3Counts[k] }

func rv3DB(t *testing.T) *database.DB {
	t.Helper()
	rv3Reset()
	t.Cleanup(rv3Reset)
	sqlDB, err := sql.Open("sqlite3_rv3", "file:"+filepath.Join(t.TempDir(), "rv3.db")+"?_busy_timeout=10000&_journal_mode=WAL")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(4)
	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx))
	for u := 1; u <= 3; u++ {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected')`)
	require.NoError(t, err)
	return db
}

func rv3Set(t *testing.T, db *database.DB, uid int, unit string, dur interface{}, pos int64) {
	t.Helper()
	ctx := context.Background()
	_, err := db.ExecContext(ctx, `DELETE FROM media_progress WHERE user_id = ? AND media_item_id = 1`, uid)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at) VALUES (?,1,?,?,?,CURRENT_TIMESTAMP)`, uid, unit, dur, pos)
	require.NoError(t, err)
}

func rv3Get(t *testing.T, db *database.DB, uid int) (unit string, pos int64, dur sql.NullInt64) {
	t.Helper()
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT position_unit, last_position, duration_total FROM media_progress WHERE user_id = ? AND media_item_id = 1`, uid).Scan(&unit, &pos, &dur))
	return
}

// N1: NULL-duration seconds row; a session End learns 9000 between the PUT's SELECT and UPDATE (exercises the IS NULL
// guard branch under a real interleaving). The PUT must re-plan against 9000, not write its body duration 50.
func TestWatchProgress_RV3_N1_NullDurationLearnedByEndBetweenSelectAndUpdate(t *testing.T) {
	db := rv3DB(t)
	rv3Set(t, db, 1, "seconds", nil, 0)
	rv3Arm(rv3Upd, func() { rv2Session(t, db, 1, "seconds", 7000, i64(9000)) })
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"position":40,"duration":50}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	unit, pos, dur := rv3Get(t, db, 1)
	assert.Equal(t, "seconds", unit)
	require.True(t, dur.Valid)
	assert.Equal(t, int64(9000), dur.Int64, "session-learned duration must survive")
	assert.Equal(t, int64(40), pos)
	assert.Equal(t, 2, rv3Count(rv3Upd))
}

// N2: unit changes with the SAME duration (only the unit guard can catch it). End switches the row to pages, dur 7200.
// Body position 100 of 200 -> fraction 0.5 -> 3600 pages. A missing unit guard would write 100 (seconds semantics).
func TestWatchProgress_RV3_N2_UnitChangeSameDuration(t *testing.T) {
	db := rv3DB(t)
	rv3Set(t, db, 1, "seconds", 7200, 10)
	rv3Arm(rv3Upd, func() {
		_, err := db.ExecContext(context.Background(), `UPDATE media_progress SET position_unit = 'pages' WHERE user_id = 1 AND media_item_id = 1`)
		require.NoError(t, err)
	})
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"position":100,"duration":200}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	unit, pos, dur := rv3Get(t, db, 1)
	assert.Equal(t, "pages", unit)
	assert.Equal(t, int64(7200), dur.Int64)
	assert.Equal(t, int64(3600), pos, "re-planned as pages")
}

// N5: two PUTs on a NULL-duration seconds row with different body durations, B runs entirely inside A's window.
// The first CAS winner (B, 200) fixes the duration; A must re-plan and keep 200 (no shrink to 100).
func TestWatchProgress_RV3_N5_TwoPutsDurationShrink(t *testing.T) {
	db := rv3DB(t)
	rv3Set(t, db, 1, "seconds", nil, 0)
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	var codeB int
	rv3Arm(rv3Upd, func() { codeB, _ = wprogPut(r, "1", `{"position":150,"duration":200}`) })
	code, resp := wprogPut(r, "1", `{"position":180,"duration":100}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	require.Equal(t, http.StatusOK, codeB)
	_, pos, dur := rv3Get(t, db, 1)
	assert.Equal(t, int64(200), dur.Int64, "no duration shrink")
	assert.Equal(t, int64(180), pos, "A's position against the kept duration 200")
}

// N10: same unit, End SHRINKS the duration (7200 -> 3000) inside the window; the value guard must be equality, not <=.
func TestWatchProgress_RV3_N10_EndShrinksDurationBetweenSelectAndUpdate(t *testing.T) {
	db := rv3DB(t)
	rv3Set(t, db, 1, "seconds", 7200, 10)
	rv3Arm(rv3Upd, func() { rv2Session(t, db, 1, "seconds", 100, i64(3000)) })
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	_, pos, dur := rv3Get(t, db, 1)
	assert.Equal(t, int64(3000), dur.Int64, "the shorter session-learned duration survives")
	assert.Equal(t, int64(1500), pos)
}
