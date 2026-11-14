package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/wsticket"
	root_handlers "catalogizer/handlers"
	internal_middleware "catalogizer/internal/middleware"
	root_middleware "catalogizer/middleware"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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
func (l *rawLog) Write(p []byte) (int, error) { l.add(string(p)); return len(p), nil }
func (l *rawLog) all() string  { l.mu.Lock(); defer l.mu.Unlock(); return strings.Join(l.lines, "\n") }

type ticketRig struct {
	srv *httptest.Server
	h   *root_handlers.WebSocketHandler
	db  *database.DB
	mw  *root_middleware.JWTMiddleware
	tok string
	log *rawLog
	app *rawLog // the production access log (gin.LoggerWithFormatter(AccessLogFormatter), as in main_router.go)
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
	appLog := &rawLog{}
	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{Formatter: internal_middleware.AccessLogFormatter, Output: appLog}), internal_middleware.AccessLogRouteParams())
	r.Use(func(c *gin.Context) { lg.add(c.Request.Method + " " + c.Request.RequestURI); c.Next() })
	registerWebSocketRoute(r, mw, h)
	srv := httptest.NewServer(r)
	t.Cleanup(func() { h.Stop(); srv.Close() })
	return &ticketRig{srv: srv, h: h, db: db, mw: mw, tok: mintTestToken(t, mw, "7", 2, "1"), log: lg, app: appLog}
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
	// N2: the production access logger (the one main_router.go installs) recorded both refused requests and redacted the JWT
	app := r.app.all()
	assert.Contains(t, app, "401", "control: the access log did record the refused requests")
	assert.Equal(t, 2, strings.Count(app, "/ws?"), "control: both query-JWT requests are in the log")
	assert.NotContains(t, app, r.tok, "the session JWT must never reach the application access log")
	// the raw front-proxy stand-in is NOT redacting: it is the control that the JWT really was on the wire
	assert.Contains(t, r.log.all(), r.tok, "control: a non-redacting log would have captured the JWT")
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
	// Both credentials are sent: the Bearer header decides, and the (valid, unredeemed) ticket in the query is ignored, not consumed.
	_, tk, _ := r.mint(t, r.bearer(), "")
	require.NotEmpty(t, tk)
	c, code := dialStatus(t, r.ws("?ticket="+tk), r.bearer())
	require.Equal(t, http.StatusSwitchingProtocols, code)
	c.Close()
	c2, code := dialStatus(t, r.ws("?ticket="+tk), nil)
	require.Equal(t, http.StatusSwitchingProtocols, code, "the ticket was ignored while the header was present, so it is still redeemable")
	c2.Close()
	// A bad header with a valid ticket is NOT rescued by the ticket.
	_, tk2, _ := r.mint(t, r.bearer(), "")
	c3, code := dialStatus(t, r.ws("?ticket="+tk2), http.Header{"Authorization": {"Bearer garbage"}})
	if c3 != nil {
		c3.Close()
	}
	assert.Equal(t, http.StatusUnauthorized, code)
}

// probeRig mounts the production mint route plus a probe behind RequireWSAuth that echoes the context RequireWSAuth sets.
func probeRig(t *testing.T) (*ticketRig, *gin.Engine) {
	r := newTicketRig(t)
	pr := gin.New()
	pr.POST("/mint", r.mw.RequireBearerHeader(), r.mw.MintWSTicket)
	pr.GET("/probe", r.mw.RequireWSAuth(), func(c *gin.Context) {
		exp, _ := c.Get("token_expires_at")
		role, _ := c.Get("role_id")
		e, _ := exp.(time.Time)
		c.JSON(http.StatusOK, gin.H{"role": role, "exp": e.Unix()})
	})
	return r, pr
}

func probeCall(pr *gin.Engine, method, url, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	pr.ServeHTTP(w, req)
	return w
}

func tokenWithExp(t *testing.T, role int, exp time.Time) string {
	claims := &root_middleware.Claims{Username: "u", RoleID: role, SessionID: "1",
		RegisteredClaims: jwt.RegisteredClaims{Subject: "7", ExpiresAt: jwt.NewNumericDate(exp), Issuer: "catalog-api"}}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wire-test-secret"))
	require.NoError(t, err)
	return signed
}

// F3 of the review: a ticket socket must close at the minting token's expiry like a Bearer socket does.
func TestWSTicket_TicketCarriesMintingTokenExpiry(t *testing.T) {
	_, pr := probeRig(t)
	exp := time.Now().Add(90 * time.Minute).Truncate(time.Second)
	w := probeCall(pr, "POST", "/mint", tokenWithExp(t, 2, exp))
	require.Equal(t, 200, w.Code, w.Body.String())
	var m struct{ Ticket string }
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
	w = probeCall(pr, "GET", "/probe?ticket="+m.Ticket, "")
	require.Equal(t, 200, w.Code, w.Body.String())
	var got struct {
		Role float64
		Exp  int64
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, exp.Unix(), got.Exp, "token_expires_at must be the minting token's expiry")
}

// M4 of the review: the redeemed ticket carries the minting user's role, not a default.
func TestWSTicket_TicketCarriesMintingRole(t *testing.T) {
	_, pr := probeRig(t)
	for _, role := range []int{1, 2, 3} {
		w := probeCall(pr, "POST", "/mint", tokenWithExp(t, role, time.Now().Add(time.Hour)))
		require.Equal(t, 200, w.Code)
		var m struct{ Ticket string }
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m))
		w = probeCall(pr, "GET", "/probe?ticket="+m.Ticket, "")
		require.Equal(t, 200, w.Code)
		var got struct{ Role float64 }
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
		assert.Equal(t, role, int(got.Role), "role %d must survive mint -> redeem", role)
	}
}

// One user flooding mint must not lock other users out, and the mint route answers 429 with Retry-After.
func TestWSTicket_MintFloodBoundedPerUser(t *testing.T) {
	r := newTicketRig(t)
	_, err := r.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (8, 'b', 'b@x.test', 'h', 's', 2, 1)`)
	require.NoError(t, err)
	_, err = r.db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (2, 8, 't2', 'r2', 1, datetime('now','+1 hour'))`)
	require.NoError(t, err)
	bTok := mintTestToken(t, r.mw, "8", 2, "2")
	var last *http.Response
	for i := 0; i < wsticket.MaxPerUser+1; i++ {
		req, _ := http.NewRequest("POST", r.srv.URL+"/api/v1/ws/ticket", nil)
		req.Header.Set("Authorization", "Bearer "+r.tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		last = resp
	}
	assert.Equal(t, http.StatusTooManyRequests, last.StatusCode, "user A over the live-ticket cap")
	assert.NotEmpty(t, last.Header.Get("Retry-After"))
	code, tk, _ := r.mint(t, http.Header{"Authorization": {"Bearer " + bTok}}, "")
	assert.Equal(t, http.StatusOK, code, "user B must still mint while A floods")
	assert.NotEmpty(t, tk)
}

func TestWSTicket_MintRateLimitedPerUser(t *testing.T) {
	r := newTicketRig(t)
	var got429 *http.Response
	for i := 0; i < wsticket.MintsPerWindow+2 && got429 == nil; i++ {
		req, _ := http.NewRequest("POST", r.srv.URL+"/api/v1/ws/ticket", nil)
		req.Header.Set("Authorization", "Bearer "+r.tok)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		var m struct{ Ticket string }
		_ = json.NewDecoder(resp.Body).Decode(&m)
		resp.Body.Close()
		if resp.StatusCode == http.StatusTooManyRequests {
			got429 = resp
		} else if m.Ticket != "" {
			_, _ = r.mw.WSTickets().Redeem(m.Ticket) // keep the live-ticket cap out of the way: this test is about the rate
		}
	}
	require.NotNil(t, got429, "mint must be rate limited")
	assert.NotEmpty(t, got429.Header.Get("Retry-After"))
}

func TestWSTicket_StoreHoldsOnlyHash(t *testing.T) {
	r := newTicketRig(t)
	_, tk, _ := r.mint(t, r.bearer(), "")
	require.NotEmpty(t, tk)
	assert.False(t, r.mw.WSTickets().ContainsPlaintextForTest(tk), "the store must keep only the hash")
}

// WSF2-R2-F3: the mint limiter is keyed per authenticated USER, not per client IP.
func (r *ticketRig) addUser(t *testing.T, id, sid int, name string) string {
	t.Helper()
	_, err := r.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (?, ?, ?, 'h', 's', 2, 1)`, id, name, name+"@x.test")
	require.NoError(t, err)
	_, err = r.db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, is_active, expires_at) VALUES (?, ?, ?, 'r', 1, datetime('now','+1 hour'))`, sid, id, name+"-t")
	require.NoError(t, err)
	return mintTestToken(t, r.mw, strconv.Itoa(id), 2, strconv.Itoa(sid))
}

// mintAs mints with the given token and X-Forwarded-For (the router trusts it in the test), redeeming the ticket so the
// live-ticket cap stays out of the way; it returns the HTTP status.
func (r *ticketRig) mintAs(t *testing.T, tok, xff string) int {
	t.Helper()
	req, _ := http.NewRequest("POST", r.srv.URL+"/api/v1/ws/ticket", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-Forwarded-For", xff)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	var m struct{ Ticket string }
	_ = json.NewDecoder(resp.Body).Decode(&m)
	if m.Ticket != "" {
		_, _ = r.mw.WSTickets().Redeem(m.Ticket)
	}
	return resp.StatusCode
}

func TestWSTicket_MintLimiterKeyedPerUserNotPerIP(t *testing.T) {
	r := newTicketRig(t)
	bTok := r.addUser(t, 8, 2, "bee")
	const ip = "203.0.113.9"
	// two users behind ONE client IP each get their own full allowance
	for i := 0; i < wsticket.MintsPerWindow; i++ {
		require.Equal(t, http.StatusOK, r.mintAs(t, r.tok, ip), "user A mint %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, r.mintAs(t, r.tok, ip), "user A over its own limit")
	for i := 0; i < wsticket.MintsPerWindow; i++ {
		require.Equal(t, http.StatusOK, r.mintAs(t, bTok, ip), "user B mint %d must not be charged to A's shared IP", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, r.mintAs(t, bTok, ip), "user B over its own limit")
}

func TestWSTicket_MintLimiterOneUserAcrossIPsSharesOneAllowance(t *testing.T) {
	r := newTicketRig(t)
	for i := 0; i < wsticket.MintsPerWindow; i++ {
		ip := "198.51.100.1"
		if i%2 == 1 {
			ip = "198.51.100.2"
		}
		require.Equal(t, http.StatusOK, r.mintAs(t, r.tok, ip), "mint %d", i+1)
	}
	assert.Equal(t, http.StatusTooManyRequests, r.mintAs(t, r.tok, "198.51.100.3"), "rotating the client IP must not reset the user's allowance")
	assert.Equal(t, http.StatusTooManyRequests, r.mintAs(t, r.tok, "198.51.100.1"))
}
