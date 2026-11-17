package handlers

// WPROG3 review: the compare-and-set PUT on a REAL PostgreSQL 15 (rootless podman via internal/testpg; SKIP if absent).
// A pq driver wrapper fires a hook before the guarded UPDATE so a session End commits inside the window.

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/testpg"

	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pgrvMu    sync.Mutex
	pgrvHooks []func()
	pgrvSeen  []string
)

type pgrvConn struct{ driver.Conn }

func (c pgrvConn) ExecContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Result, error) {
	if strings.Contains(q, "UPDATE media_progress SET duration_total = $1, last_position = $2") {
		pgrvMu.Lock()
		pgrvSeen = append(pgrvSeen, q)
		var h func()
		if len(pgrvHooks) > 0 {
			h, pgrvHooks = pgrvHooks[0], pgrvHooks[1:]
		}
		pgrvMu.Unlock()
		if h != nil {
			h()
		}
	}
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, q, a)
}
func (c pgrvConn) QueryContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.(driver.QueryerContext).QueryContext(ctx, q, a)
}
func (c pgrvConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	return c.Conn.(driver.ConnBeginTx).BeginTx(ctx, o)
}

type pgrvDriver struct{}

func (pgrvDriver) Open(dsn string) (driver.Conn, error) {
	c, err := pq.Driver{}.Open(dsn)
	if err != nil {
		return nil, err
	}
	return pgrvConn{c}, nil
}

func init() { sql.Register("postgres_rv3", pgrvDriver{}) }

func pgrvDB(t *testing.T) *database.DB {
	dsn, _ := testpg.Start(t)
	sqlDB, err := sql.Open("postgres_rv3", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(4)
	db := database.WrapDB(sqlDB, database.DialectPostgres)
	ctx := context.Background()
	require.NoError(t, db.RunMigrations(ctx))
	for u := 1; u <= 2; u++ {
		_, err := db.ExecContext(ctx, `INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, 'x', 'x', 2)`, u, fmt.Sprintf("u%d", u), fmt.Sprintf("u%d@example.test", u))
		require.NoError(t, err)
	}
	var mid int64
	require.NoError(t, db.QueryRowContext(ctx, `INSERT INTO media_items (media_type_id, title, status) VALUES (1, 'Item', 'detected') RETURNING id`).Scan(&mid))
	require.Equal(t, int64(1), mid)
	return db
}

func TestWatchProgress_PG_CompareAndSetOnRealPostgres(t *testing.T) {
	db := pgrvDB(t)
	ctx := context.Background()
	r := wprogRouter(NewAndroidTVMediaHandler(db), 1)
	get := func() (string, int64, sql.NullInt64) {
		var u string
		var p int64
		var d sql.NullInt64
		require.NoError(t, db.QueryRowContext(ctx, `SELECT position_unit, last_position, duration_total FROM media_progress WHERE user_id = 1 AND media_item_id = 1`).Scan(&u, &p, &d))
		return u, p, d
	}

	// 1. first insert
	code, resp := wprogPut(r, "1", `{"position":10,"duration":100}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	require.Equal(t, true, resp["persisted"])
	// 2. guarded UPDATE with a value guard
	code, resp = wprogPut(r, "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	_, p, d := get()
	assert.Equal(t, int64(50), p)
	assert.Equal(t, int64(100), d.Int64)
	// 3. guarded UPDATE with the IS NULL guard
	_, err := db.ExecContext(ctx, `UPDATE media_progress SET duration_total = NULL WHERE user_id = 1`)
	require.NoError(t, err)
	code, resp = wprogPut(r, "1", `{"position":5,"duration":50}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	_, p, d = get()
	assert.Equal(t, int64(5), p)
	assert.Equal(t, int64(50), d.Int64)
	// 4. NULL row; a real session End (transaction, pq) learns 9000 inside the PUT's window -> re-plan
	_, err = db.ExecContext(ctx, `UPDATE media_progress SET duration_total = NULL WHERE user_id = 1`)
	require.NoError(t, err)
	pgrvMu.Lock()
	pgrvHooks = []func(){func() {
		_, e := db.ExecContext(ctx, `UPDATE media_progress SET duration_total = 9000, last_position = 7000 WHERE user_id = 1`)
		require.NoError(t, e)
	}}
	pgrvMu.Unlock()
	code, resp = wprogPut(r, "1", `{"position":40,"duration":50}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	assert.Equal(t, true, resp["persisted"])
	u, p, d := get()
	assert.Equal(t, "seconds", u)
	assert.Equal(t, int64(9000), d.Int64)
	assert.Equal(t, int64(40), p)
	// 5. unit change inside the window (pages session End) -> re-plan as pages
	pgrvMu.Lock()
	pgrvHooks = []func(){func() {
		_, e := db.ExecContext(ctx, `UPDATE media_progress SET position_unit = 'pages', duration_total = 300 WHERE user_id = 1`)
		require.NoError(t, e)
	}}
	pgrvMu.Unlock()
	code, resp = wprogPut(r, "1", `{"progress":0.5}`)
	require.Equal(t, http.StatusOK, code, "%v", resp)
	u, p, d = get()
	assert.Equal(t, "pages", u)
	assert.Equal(t, int64(300), d.Int64)
	assert.Equal(t, int64(150), p)
	// 6. exhaustion -> 409
	n := 300
	bump := func() {
		n++
		_, err := db.ExecContext(ctx, `UPDATE media_progress SET duration_total = ? WHERE user_id = 1`, n)
		require.NoError(t, err)
	}
	pgrvMu.Lock()
	pgrvHooks = []func(){bump, bump, bump}
	pgrvMu.Unlock()
	code, resp = wprogPut(r, "1", `{"progress":0.9}`)
	assert.Equal(t, http.StatusConflict, code, "%v", resp)
	assert.Equal(t, "concurrent_update", resp["reason"])
	pgrvMu.Lock()
	for _, q := range pgrvSeen {
		t.Logf("PG executed: %s", strings.Join(strings.Fields(q), " "))
		assert.NotContains(t, q, "?")
	}
	pgrvMu.Unlock()
}
