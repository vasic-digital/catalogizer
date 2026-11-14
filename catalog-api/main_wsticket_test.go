package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	root_middleware "catalogizer/middleware"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SEC-WS-1 F2: /ws must not take the session JWT in the query string; a short-lived single-use ticket replaces it.
// Real router (production wiring through registerWebSocketRoute), real migrated SQLite, real HTTP server, real gorilla client.
// A raw access-log middleware in front stands in for any non-redacting proxy / server log between client and API.

type rawLog struct {
	mu    sync.Mutex
	lines []string
}

func (l *rawLog) add(s string) { l.mu.Lock(); l.lines = append(l.lines, s); l.mu.Unlock() }
func (l *rawLog) all() string  { l.mu.Lock(); defer l.mu.Unlock(); return strings.Join(l.lines, "\n") }

type ticketRig struct {
	srv *httptest.Server
	h   *root_handlers.WebSocketHandler
	db  *database.DB
	mw  *root_middleware.JWTMiddleware
	tok string
	log *rawLog
}

func newTicketRig(t *testing.T) *ticketRig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	_, err = db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (7, 'dem', 'd@x.test', 'h', 's', 2, 1)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (1, 7, 't', 'r', 1, datetime('now','+1 hour'))`)
	require.NoError(t, err)
	mw := newAuthMiddleware("wire-test-secret", db, repository.NewUserRepository(db))
	h := root_handlers.NewWebSocketHandlerWithConfig(zap.NewNop(), root_handlers.DefaultWebSocketConfig())
	lg := &rawLog{}
	r := gin.New()
	r.Use(func(c *gin.Context) { lg.add(c.Request.Method + " " + c.Request.RequestURI); c.Next() })
	registerWebSocketRoute(r, mw, h)
	srv := httptest.NewServer(r)
	t.Cleanup(func() { h.Stop(); srv.Close() })
	return &ticketRig{srv: srv, h: h, db: db, mw: mw, tok: mintTestToken(t, mw, "7", 2, "1"), log: lg}
}

func (r *ticketRig) ws(q string) string {
	return "ws" + strings.TrimPrefix(r.srv.URL, "http") + "/ws" + q
}

func (r *ticketRig) mint(t *testing.T, hdr http.Header, query string) (int, string, int) {
	t.Helper()
	req, err := http.NewRequest("POST", r.srv.URL+"/api/v1/ws/ticket"+query, nil)
	require.NoError(t, err)
	for k, v := range hdr {
		req.Header[k] = v
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var m struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expires_in"`
	}
	_ = json.Unmarshal(b, &m)
	return resp.StatusCode, m.Ticket, m.ExpiresIn
}

func (r *ticketRig) bearer() http.Header { return http.Header{"Authorization": {"Bearer " + r.tok}} }

func dialStatus(t *testing.T, url string, hdr http.Header) (*websocket.Conn, int) {
	t.Helper()
	c, resp, err := websocket.DefaultDialer.Dial(url, hdr)
	if err != nil {
		require.NotNil(t, resp, "dial error without HTTP response: %v", err)
		return nil, resp.StatusCode
	}
	return c, resp.StatusCode
}

func TestWSTicket_QueryJWTRefusedAndNeverReachesLog(t *testing.T) {
	r := newTicketRig(t)
	for _, q := range []string{"?token=" + r.tok, "?access_token=" + r.tok} {
		c, code := dialStatus(t, r.ws(q), nil)
		if c != nil {
			c.Close()
		}
		assert.Equal(t, http.StatusUnauthorized, code, "a session JWT in the query must not authenticate /ws (%s)", q[:8])
	}
}

// The conforming flow (header mint, ?ticket= upgrade) leaves no JWT in the raw request log of any front proxy; the only
// credential-shaped value in the URL is a ticket that is already spent.
func TestWSTicket_ConformingFlowLeavesNoJWTInRawLog(t *testing.T) {
	r := newTicketRig(t)
	_, tk, _ := r.mint(t, r.bearer(), "")
	c, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	require.Equal(t, http.StatusSwitchingProtocols, code)
	c.Close()
	raw := r.log.all()
	assert.Contains(t, raw, "?ticket="+tk, "control: the raw log does capture request URLs")
	assert.NotContains(t, raw, r.tok, "no JWT in any logged URL")
	c2, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	if c2 != nil {
		c2.Close()
	}
	assert.Equal(t, http.StatusUnauthorized, code, "the logged ticket is worthless: spent")
}

func TestWSTicket_MintRequiresHeaderAuth(t *testing.T) {
	r := newTicketRig(t)
	code, tk, _ := r.mint(t, nil, "")
	assert.Equal(t, 401, code)
	assert.Empty(t, tk)
	code, tk, _ = r.mint(t, nil, "?token="+r.tok)
	assert.Equal(t, 401, code, "a query JWT must not mint a ticket")
	assert.Empty(t, tk)
}

func TestWSTicket_FlowSingleUseAndReplay(t *testing.T) {
	r := newTicketRig(t)
	code, tk, ttl := r.mint(t, r.bearer(), "")
	require.Equal(t, 200, code)
	require.NotEmpty(t, tk)
	assert.Equal(t, 30, ttl)
	assert.NotContains(t, tk, r.tok)

	conn, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	require.Equal(t, http.StatusSwitchingProtocols, code)
	defer conn.Close()
	require.Eventually(t, func() bool { return r.h.GetStats().ActiveConnections >= 1 }, 2*time.Second, 10*time.Millisecond)
	r.h.BroadcastToClients(map[string]interface{}{"type": "media_update", "marker": "ticket-flow"})
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg), "ticket-flow")

	// replay of the used ticket
	c2, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	if c2 != nil {
		c2.Close()
	}
	assert.Equal(t, http.StatusUnauthorized, code, "a used ticket must be refused")
}

func TestWSTicket_ExpiredRefused(t *testing.T) {
	r := newTicketRig(t)
	now := time.Now()
	clock := now
	r.mw.WSTickets().WithClock(func() time.Time { return clock })
	_, tk, _ := r.mint(t, r.bearer(), "")
	require.NotEmpty(t, tk)
	clock = now.Add(31 * time.Second)
	c, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	if c != nil {
		c.Close()
	}
	assert.Equal(t, http.StatusUnauthorized, code, "an expired ticket must be refused")
	// control: a fresh ticket from the same clock still works
	_, tk2, _ := r.mint(t, r.bearer(), "")
	c, code = dialStatus(t, r.ws("?ticket="+tk2), nil)
	require.Equal(t, http.StatusSwitchingProtocols, code)
	c.Close()
}

func TestWSTicket_GarbageAndRevokedSession(t *testing.T) {
	r := newTicketRig(t)
	for _, tk := range []string{"x", strings.Repeat("A", 43), strings.Repeat("A", 500)} {
		c, code := dialStatus(t, r.ws("?ticket="+tk), nil)
		if c != nil {
			c.Close()
		}
		assert.Equal(t, http.StatusUnauthorized, code, tk[:1])
	}
	_, tk, _ := r.mint(t, r.bearer(), "")
	_, err := r.db.Exec(`UPDATE user_sessions SET is_active = 0 WHERE id = 1`)
	require.NoError(t, err)
	c, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	if c != nil {
		c.Close()
	}
	assert.Equal(t, http.StatusUnauthorized, code, "a ticket must not outlive its session")
}

func TestWSTicket_BearerHeaderStillWorksAndTicketIgnoredWithHeader(t *testing.T) {
	r := newTicketRig(t)
	c, code := dialStatus(t, r.ws(""), r.bearer())
	require.Equal(t, http.StatusSwitchingProtocols, code)
	c.Close()
}

func TestWSTicket_StoreHoldsOnlyHash(t *testing.T) {
	r := newTicketRig(t)
	_, tk, _ := r.mint(t, r.bearer(), "")
	require.NotEmpty(t, tk)
	assert.False(t, r.mw.WSTickets().ContainsPlaintextForTest(tk), "the store must keep only the hash")
}
