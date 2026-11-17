package handlers

// WPROG3: races between PUT /media/:id/progress and the playback-session writer (adopted from the WPROG2 Opus review
// probes RV2-A/RV2-B, driver wrapper kept), the exhaustion answer, the Postgres rewrite of the guarded UPDATE, and the
// reviewer mutants X1-X4 as killing tests. File-backed SQLite, real handler, real PlaybackSessionRepository (host fixture).

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
	"catalogizer/repository"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	rv2HookMu   sync.Mutex
	rv2Hook     func()
	rv2Sticky   bool     // keep the hook armed for every matching UPDATE (exhaustion test)
	rv2Captured []string // every guarded UPDATE the handler executed (for the Postgres rewrite test)
)

type rv2Conn struct{ *sqlite3.SQLiteConn }

func (c rv2Conn) Exec(q string, args []driver.Value) (driver.Result, error) {
	if strings.Contains(q, "UPDATE media_progress SET duration_total = ?, last_position = ?") {
		rv2HookMu.Lock()
		h := rv2Hook
		if !rv2Sticky {
			rv2Hook = nil
		}
		rv2Captured = append(rv2Captured, q)
		rv2HookMu.Unlock()
		if h != nil {
			h()
		}
	}
	return c.SQLiteConn.Exec(q, args)
}

type rv2Driver struct{ sqlite3.SQLiteDriver }

func (d *rv2Driver) Open(dsn string) (driver.Conn, error) {
	c, err := d.SQLiteDriver.Open(dsn)
	if err != nil {
		return nil, err
	}
	return rv2Conn{c.(*sqlite3.SQLiteConn)}, nil
}

func init() { sql.Register("sqlite3_rv2", &rv2Driver{}) }

func rv2FileDB(t *testing.T, drv string, maxConns int) *database.DB {
	t.Helper()
	sqlDB, err := sql.Open(drv, "file:"+filepath.Join(t.TempDir(), "rv2.db")+"?_busy_timeout=10000&_journal_mode=WAL")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(maxConns)
	ctx := context.Background()
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(ctx))
	for u := 1; u <= 8; u++ {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected')`)
	require.NoError(t, err)
	return db
}

func rv2Session(t *testing.T, db *database.DB, uid int64, unit string, end int64, dur *int64) {
	t.Helper()
	ctx := context.Background()
	repo := repository.NewPlaybackSessionRepository(db)
	id, err := repo.Start(ctx, repository.PlaybackStart{UserID: uid, MediaItemID: 1, PositionUnit: unit})
	require.NoError(t, err)
	require.NoError(t, repo.End(ctx, repository.PlaybackEnd{SessionID: id, EndPosition: end, TotalAmount: end, DurationTotal: dur}))
}

func i64(v int64) *int64 { return &v }

func rv2Reset() {
	rv2HookMu.Lock()
	rv2Hook, rv2Sticky, rv2Captured = nil, false, nil
	rv2HookMu.Unlock()
}

func rv2Row(t *testing.T, db *database.DB) (unit string, pos int64, dur sql.NullInt64) {
	t.Helper()
	require.NoError(t, db.QueryRowContext(context.Background(),
		`SELECT position_unit, last_position, duration_total FROM media_progress WHERE user_id = 1 AND media_item_id = 1`).Scan(&unit, &pos, &dur))
	return
}

func rv2Insert(t *testing.T, db *database.DB, unit string, dur interface{}, pos int64) {
	t.Helper()
	_, err := db.ExecContext(context.Background(),
		`INSERT INTO media_progress (user_id, media_item_id, position_unit, duration_total, last_position, updated_at) VALUES (1,1,?,?,?,CURRENT_TIMESTAMP)`,
		unit, dur, pos)
	require.NoError(t, err)
}

// F1/RV2-A: a session End committing between the PUT's SELECT and its UPDATE changes the unit. RED on cc33a34ed
// (row ends unit=pages pos=3600 dur=7200: seconds duration written into a pages row).
func TestRV2_RaceSessionEndChangesUnitBetweenSelectAndUpdate(t *testing.T) {
	defer rv2Reset()
	db := rv2FileDB(t, "sqlite3_rv2", 4)
	rv2Session(t, db, 1, "seconds", 100, i64(7200))
	rv2HookMu.Lock()
	rv2Hook = func() { rv2Session(t, db, 1, "pages", 150, i64(300)) }
	rv2HookMu.Unlock()
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	unit, pos, dur := rv2Row(t, db)
	assert.Equal(t, "pages", unit)
	require.True(t, dur.Valid)
	assert.Equal(t, int64(300), dur.Int64, "pages row keeps the pages duration")
	assert.Equal(t, int64(150), pos, "re-planned against the new row: 0.5 of 300 pages")
}

// F1/RV2-B: same unit, the session learns a longer duration between SELECT and UPDATE. RED on cc33a34ed (the PUT rolls
// the session-learned 9000 back to 7200).
func TestRV2_RaceSessionEndLearnsDurationBetweenSelectAndUpdate(t *testing.T) {
	defer rv2Reset()
	db := rv2FileDB(t, "sqlite3_rv2", 4)
	rv2Session(t, db, 1, "seconds", 100, i64(7200))
	rv2HookMu.Lock()
	rv2Hook = func() { rv2Session(t, db, 1, "seconds", 8000, i64(9000)) }
	rv2HookMu.Unlock()
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, resp["persisted"])
	_, pos, dur := rv2Row(t, db)
	require.True(t, dur.Valid)
	assert.Equal(t, int64(9000), dur.Int64, "PUT must not overwrite the duration the session just learned")
	assert.Equal(t, int64(4500), pos)
}

// The row changes on EVERY attempt: the bounded loop ends with 409 persisted:false reason concurrent_update, never
// persisted:true (zero RowsAffected is not a write), and the PUT's own position is not stored.
func TestWatchProgress_ConcurrentUpdateExhaustionIs409NotPersisted(t *testing.T) {
	defer rv2Reset()
	db := rv2FileDB(t, "sqlite3_rv2", 4)
	rv2Insert(t, db, "seconds", 1000, 10)
	n := int64(1000)
	rv2HookMu.Lock()
	rv2Sticky = true
	rv2Hook = func() {
		n++
		_, err := db.ExecContext(context.Background(), `UPDATE media_progress SET duration_total = ? WHERE user_id = 1 AND media_item_id = 1`, n)
		require.NoError(t, err)
	}
	rv2HookMu.Unlock()
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"progress":0.9}`)
	assert.Equal(t, http.StatusConflict, code, "%v", resp)
	assert.Equal(t, false, resp["persisted"])
	assert.Equal(t, "concurrent_update", resp["reason"])
	_, pos, _ := rv2Row(t, db)
	assert.Equal(t, int64(10), pos, "nothing of the PUT was stored")
	rv2HookMu.Lock()
	attempts := len(rv2Captured)
	rv2HookMu.Unlock()
	assert.Equal(t, 3, attempts, "bounded to 3 attempts")
}

// A guarded UPDATE with a NULL stored duration is spelled `IS NULL` (never `? IS NULL`); both spellings must survive the
// Postgres placeholder rewrite (no "?" left, sequential $n).
func TestWatchProgress_GuardedUpdatePostgresRewrite(t *testing.T) {
	defer rv2Reset()
	db := rv2FileDB(t, "sqlite3_rv2", 2)
	rv2Insert(t, db, "seconds", 1000, 10)
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	code, _ := wprogPut(r, "1", `{"progress":0.5}`)
	require.Equal(t, 200, code)
	_, err := db.ExecContext(context.Background(), `UPDATE media_progress SET duration_total = NULL WHERE user_id = 1 AND media_item_id = 1`)
	require.NoError(t, err)
	code, _ = wprogPut(r, "1", `{"position":5,"duration":50}`)
	require.Equal(t, 200, code)
	rv2HookMu.Lock()
	qs := append([]string(nil), rv2Captured...)
	rv2HookMu.Unlock()
	require.Len(t, qs, 2)
	pg := database.Dialect{Type: database.DialectPostgres}
	withVal := pg.RewritePlaceholders(qs[0])
	withNull := pg.RewritePlaceholders(qs[1])
	assert.NotContains(t, withVal, "?")
	assert.NotContains(t, withNull, "?")
	assert.Contains(t, withVal, "position_unit = $6 AND duration_total = $7")
	assert.Contains(t, withNull, "position_unit = $6 AND duration_total IS NULL")
	assert.NotContains(t, withNull, "$7")
}

// X1 (mutant: duration rounding -> truncation): fractional seconds round to nearest.
func TestWatchProgress_X1_RoundsFractionalSeconds(t *testing.T) {
	db := rv2FileDB(t, "sqlite3", 2)
	code, _ := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"position":10.6,"duration":7200.6}`)
	require.Equal(t, 200, code)
	_, pos, dur := rv2Row(t, db)
	assert.Equal(t, int64(7201), dur.Int64)
	assert.Equal(t, int64(11), pos)
}

// X2 (mutant: fraction clamp f>1 -> f>100): a position past the duration on a non-seconds row is clamped to the fraction 1.
func TestWatchProgress_X2_FractionClamped(t *testing.T) {
	db := rv2FileDB(t, "sqlite3", 2)
	rv2Insert(t, db, "pages", 300, 10)
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"position":500,"duration":100}`)
	require.Equal(t, 200, code, "%v", resp)
	unit, pos, dur := rv2Row(t, db)
	assert.Equal(t, "pages", unit)
	assert.Equal(t, int64(300), dur.Int64)
	assert.Equal(t, int64(300), pos, "fraction 5 clamps to 1 -> the last page, never above the duration")
}

// X3 (mutant: a NULL-duration non-seconds row learns the body duration): the row is left alone, persisted:false.
func TestWatchProgress_X3_NullDurationNonSecondsRowNotTouched(t *testing.T) {
	db := rv2FileDB(t, "sqlite3", 2)
	rv2Insert(t, db, "pages", nil, 4)
	code, resp := wprogPut(wprogRouter(NewAndroidTVMediaHandler(db), 1), "1", `{"position":5,"duration":100}`)
	require.Equal(t, 200, code)
	assert.Equal(t, false, resp["persisted"])
	assert.Equal(t, "unknown_duration_unit_pages", resp["reason"])
	unit, pos, dur := rv2Row(t, db)
	assert.Equal(t, "pages", unit)
	assert.False(t, dur.Valid, "duration stays unknown")
	assert.Equal(t, int64(4), pos)
}

// X4 (mutant: position >= 0 -> > 0): a position of 0 (start of the item) is valid and stored.
func TestWatchProgress_X4_ZeroPositionAccepted(t *testing.T) {
	db := rv2FileDB(t, "sqlite3", 2)
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	code, resp := wprogPut(r, "1", `{"position":0,"duration":10}`)
	require.Equal(t, 200, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	_, pos, _ := rv2Row(t, db)
	assert.Equal(t, int64(0), pos)
}
