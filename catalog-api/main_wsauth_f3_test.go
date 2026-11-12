package main

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	root_middleware "catalogizer/middleware"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SEC-WS-1 F3 (PROVISIONAL, hermetic-real: real migrated DB, real middleware and
// route helper, real HTTP server, real gorilla clients): after the upgrade the
// socket is re-validated (session / suspension / lock / token expiry / admin role)
// and events are delivered only to recipients entitled to them.
type f3Rig struct {
	srv *httptest.Server
	h   *root_handlers.WebSocketHandler
	db  *database.DB
}

const f3Recheck = 150 * time.Millisecond

func newF3Rig(t *testing.T) *f3Rig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	for _, q := range []string{
		`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (7, 'dem', 'd@x.test', 'h', 's', 2, 1)`,
		`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (8, 'adm', 'a@x.test', 'h', 's', 1, 1)`,
		`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (1, 7, 't1', 'r1', 1, datetime('now','+1 hour'))`,
		`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (2, 8, 't2', 'r2', 1, datetime('now','+1 hour'))`,
	} {
		_, err = db.Exec(q)
		require.NoError(t, err)
	}
	mw := newAuthMiddleware("wire-test-secret", db, repository.NewUserRepository(db))
	cfg := root_handlers.DefaultWebSocketConfig()
	cfg.RecheckInterval = f3Recheck
	h := root_handlers.NewWebSocketHandlerWithConfig(zap.NewNop(), cfg)
	r := gin.New()
	registerWebSocketRoute(r, mw, h)
	srv := httptest.NewServer(r)
	t.Cleanup(func() { h.Stop(); srv.Close() })
	return &f3Rig{srv: srv, h: h, db: db}
}

func f3Token(t *testing.T, sub string, role int, sid string, ttl time.Duration) string {
	t.Helper()
	claims := &root_middleware.Claims{Username: "u", RoleID: role, SessionID: sid,
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)), Issuer: "catalog-api"}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wire-test-secret"))
	require.NoError(t, err)
	return s
}

func (r *f3Rig) dial(t *testing.T, tok string) *websocket.Conn {
	t.Helper()
	u := "ws" + strings.TrimPrefix(r.srv.URL, "http") + "/ws"
	c, _, err := websocket.DefaultDialer.Dial(u, http.Header{"Authorization": {"Bearer " + tok}})
	require.NoError(t, err)
	t.Cleanup(func() { c.Close() })
	// make sure the server registered the client before the test broadcasts
	require.NoError(t, c.WriteJSON(map[string]interface{}{"type": "ping"}))
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, m, err := c.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(m), "pong")
	return c
}

// expectClosed waits for the server to close the socket within d.
func expectClosed(t *testing.T, c *websocket.Conn, d time.Duration) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(d))
	for {
		if _, _, err := c.ReadMessage(); err != nil {
			ne, isNet := err.(interface{ Timeout() bool })
			require.False(t, isNet && ne.Timeout(), "socket was NOT closed within %s: %v", d, err)
			return
		}
	}
}

func expectNoMessage(t *testing.T, c *websocket.Conn, d time.Duration) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(d))
	_, m, err := c.ReadMessage()
	if err == nil {
		t.Fatalf("unexpected message delivered: %s", m)
	}
}

func expectMessage(t *testing.T, c *websocket.Conn, contains string) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(3 * time.Second))
	_, m, err := c.ReadMessage()
	require.NoError(t, err)
	require.Contains(t, string(m), contains)
}

func TestF3_SessionRevokedAfterConnectClosesSocket(t *testing.T) {
	r := newF3Rig(t)
	c := r.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	_, err := r.db.Exec(`UPDATE user_sessions SET is_active = 0 WHERE id = 1`)
	require.NoError(t, err)
	expectClosed(t, c, 4*f3Recheck+time.Second)
}

func TestF3_UserSuspendedAfterConnectClosesSocket(t *testing.T) {
	r := newF3Rig(t)
	c := r.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	_, err := r.db.Exec(`UPDATE users SET is_active = 0 WHERE id = 7`)
	require.NoError(t, err)
	expectClosed(t, c, 4*f3Recheck+time.Second)
}

func TestF3_UserLockedAfterConnectClosesSocket(t *testing.T) {
	r := newF3Rig(t)
	c := r.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	_, err := r.db.Exec(`UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = 7`)
	require.NoError(t, err)
	expectClosed(t, c, 4*f3Recheck+time.Second)
}

func TestF3_TokenExpiryClosesSocket(t *testing.T) {
	r := newF3Rig(t)
	// JWT NumericDate has 1 s resolution: use 3 s so the handshake still succeeds
	c := r.dial(t, f3Token(t, "7", 2, "1", 3*time.Second))
	expectClosed(t, c, 4*f3Recheck+4*time.Second)
}

func TestF3_RevokedSessionRefusedOnNextInboundMessage(t *testing.T) {
	r := newF3Rig(t)
	// recheck interval huge: only the per-message check can close it
	cfg := root_handlers.DefaultWebSocketConfig()
	cfg.RecheckInterval = time.Hour
	mw := newAuthMiddleware("wire-test-secret", r.db, repository.NewUserRepository(r.db))
	h := root_handlers.NewWebSocketHandlerWithConfig(zap.NewNop(), cfg)
	gr := gin.New()
	registerWebSocketRoute(gr, mw, h)
	srv := httptest.NewServer(gr)
	t.Cleanup(func() { h.Stop(); srv.Close() })
	r2 := &f3Rig{srv: srv, h: h, db: r.db}
	c := r2.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	_, err := r.db.Exec(`UPDATE user_sessions SET is_active = 0 WHERE id = 1`)
	require.NoError(t, err)
	require.NoError(t, c.WriteJSON(map[string]interface{}{"type": "ping"}))
	expectClosed(t, c, 3*time.Second)
}

func TestF3_AdminOnlyEventsReachOnlyAdmins(t *testing.T) {
	r := newF3Rig(t)
	user := r.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	admin := r.dial(t, f3Token(t, "8", 1, "2", time.Hour))
	for _, typ := range []string{"scan_started", "notification", "file_created", "file_modified", "file_deleted", "some_future_type"} {
		r.h.BroadcastToClients(map[string]interface{}{"type": typ, "payload": map[string]interface{}{"path": "/srv/secret"}})
		expectMessage(t, admin, typ)
		expectNoMessage(t, user, 150*time.Millisecond)
	}
}

func TestF3_LibraryEventsReachEveryAuthenticatedUser(t *testing.T) {
	r := newF3Rig(t)
	user := r.dial(t, f3Token(t, "7", 2, "1", time.Hour))
	admin := r.dial(t, f3Token(t, "8", 1, "2", time.Hour))
	for _, typ := range []string{"media_update", "asset_update"} {
		r.h.BroadcastToClients(map[string]interface{}{"type": typ, "entity_id": 5})
		expectMessage(t, user, typ)
		expectMessage(t, admin, typ)
	}
}

func TestF3_ForgedAdminRoleClaimDoesNotGrantAdminEvents(t *testing.T) {
	r := newF3Rig(t)
	// user 7 is role 2 in the DB; the token claims role 1
	c := r.dial(t, f3Token(t, "7", 1, "1", time.Hour))
	r.h.BroadcastToClients(map[string]interface{}{"type": "scan_started"})
	expectNoMessage(t, c, 250*time.Millisecond)
}

func TestF3_AdminDemotedAfterConnectStopsReceivingAdminEvents(t *testing.T) {
	r := newF3Rig(t)
	c := r.dial(t, f3Token(t, "8", 1, "2", time.Hour))
	r.h.BroadcastToClients(map[string]interface{}{"type": "scan_started"})
	expectMessage(t, c, "scan_started")
	_, err := r.db.Exec(`UPDATE users SET role_id = 2 WHERE id = 8`)
	require.NoError(t, err)
	time.Sleep(4 * f3Recheck)
	r.h.BroadcastToClients(map[string]interface{}{"type": "scan_started"})
	expectNoMessage(t, c, 250*time.Millisecond)
}
