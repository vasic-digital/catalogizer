package main

import (
	"context"
	"database/sql"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	root_handlers "catalogizer/handlers"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// SEC-WS-1: the /ws route must authenticate like the rest of the API. These tests
// drive the real route helper with the production middleware wiring
// (newAuthMiddleware: session checker over a real migrated DB), a real HTTP
// server and a real gorilla client.
type wsRig struct {
	srv *httptest.Server
	h   *root_handlers.WebSocketHandler
	db  *database.DB
	mw  interface{}
	tok string
}

func newWSRig(t *testing.T) *wsRig {
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
	cfg := root_handlers.DefaultWebSocketConfig()
	cfg.AllowedOrigins = []string{"https://catalog.example.test"}
	h := root_handlers.NewWebSocketHandlerWithConfig(zap.NewNop(), cfg)
	r := gin.New()
	registerWebSocketRoute(r, mw, h)
	srv := httptest.NewServer(r)
	t.Cleanup(func() { h.Stop(); srv.Close() })
	return &wsRig{srv: srv, h: h, db: db, tok: mintTestToken(t, mw, "7", 2, "1")}
}

func (r *wsRig) url(q string) string { return "ws" + strings.TrimPrefix(r.srv.URL, "http") + "/ws" + q }

func dial(t *testing.T, url string, hdr http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	return websocket.DefaultDialer.Dial(url, hdr)
}

func TestWSRoute_UnauthenticatedRefused(t *testing.T) {
	r := newWSRig(t)
	conn, resp, err := dial(t, r.url(""), nil)
	if conn != nil {
		conn.Close()
	}
	require.Error(t, err, "an unauthenticated client must not complete the upgrade")
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, int64(0), r.h.GetStats().ActiveConnections)
}

func TestWSRoute_BadTokenRefused(t *testing.T) {
	r := newWSRig(t)
	_, resp, err := dial(t, r.url("?token=not-a-jwt"), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWSRoute_ValidTokenConnectsAndReceives(t *testing.T) {
	r := newWSRig(t)
	for name, tc := range map[string]struct {
		q   string
		hdr http.Header
	}{
		"query token (web + android clients)": {q: "?token=" + r.tok},
		"bearer header":                       {hdr: http.Header{"Authorization": {"Bearer " + r.tok}}},
	} {
		t.Run(name, func(t *testing.T) {
			conn, resp, err := dial(t, r.url(tc.q), tc.hdr)
			require.NoError(t, err)
			defer conn.Close()
			assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
			require.Eventually(t, func() bool { return r.h.GetStats().ActiveConnections >= 1 }, 2*time.Second, 10*time.Millisecond)
			r.h.BroadcastToClients(map[string]interface{}{"type": "notification", "marker": name})
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			_, msg, err := conn.ReadMessage()
			require.NoError(t, err)
			assert.Contains(t, string(msg), "notification")
		})
	}
}

func TestWSRoute_RevokedSessionRefused(t *testing.T) {
	r := newWSRig(t)
	_, err := r.db.Exec(`UPDATE user_sessions SET is_active = 0 WHERE id = 1`)
	require.NoError(t, err)
	_, resp, err := dial(t, r.url("?token="+r.tok), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWSRoute_SuspendedUserRefused(t *testing.T) {
	r := newWSRig(t)
	_, err := r.db.Exec(`UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = 7`)
	require.NoError(t, err)
	_, resp, err := dial(t, r.url("?token="+r.tok), nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestWSRoute_OriginPolicy(t *testing.T) {
	r := newWSRig(t)
	host := strings.TrimPrefix(r.srv.URL, "http://")
	cases := []struct {
		name, origin string
		ok           bool
	}{
		{"no Origin (native clients: Android OkHttp)", "", true},
		{"same origin as the Host header", "http://" + host, true},
		{"configured allow-list origin", "https://catalog.example.test", true},
		{"foreign browser origin", "https://evil.example.test", false},
		{"foreign origin with allow-listed substring", "https://catalog.example.test.evil.test", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hdr := http.Header{}
			if tc.origin != "" {
				hdr.Set("Origin", tc.origin)
			}
			conn, resp, err := dial(t, r.url("?token="+r.tok), hdr)
			if tc.ok {
				require.NoError(t, err)
				conn.Close()
				return
			}
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestMainGoMountsWebSocketThroughHelper(t *testing.T) {
	src, err := os.ReadFile("main.go")
	require.NoError(t, err)
	s := string(src)
	assert.Contains(t, s, "registerWebSocketRoute(router, jwtMiddleware, wsHandler)")
	assert.NotContains(t, s, `router.GET("/ws"`, "the raw unauthenticated /ws mount must not return")
}

// Behind a reverse proxy: the shipped nginx configs forward the client's Host with
// `proxy_set_header Host $http_host` (port kept). This drives a real
// httputil.ReverseProxy in front of the real route to show a browser Origin with a
// non-default port passes then, and (documenting the original defect) is refused
// when the proxy sends the port-less `$host`.
func TestWSRoute_BehindReverseProxyHostForwarding(t *testing.T) {
	r := newWSRig(t)
	target, err := url.Parse(r.srv.URL)
	require.NoError(t, err)
	for _, tc := range []struct {
		name      string
		stripPort bool
		ok        bool
	}{{"Host $http_host (port kept)", false, true}, {"Host $host (port dropped)", true, false}} {
		t.Run(tc.name, func(t *testing.T) {
			rp := httputil.NewSingleHostReverseProxy(target)
			inner := rp.Director
			rp.Director = func(q *http.Request) {
				h := q.Host
				inner(q)
				if tc.stripPort {
					if hn, _, err := net.SplitHostPort(h); err == nil {
						h = hn
					}
				}
				q.Host = h
			}
			px := httptest.NewServer(rp)
			defer px.Close()
			hdr := http.Header{"Origin": {px.URL}} // browser on http://127.0.0.1:<port>
			conn, resp, err := dial(t, "ws"+strings.TrimPrefix(px.URL, "http")+"/ws?token="+r.tok, hdr)
			if tc.ok {
				require.NoError(t, err)
				conn.Close()
				return
			}
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		})
	}
}

func TestShippedNginxConfigsForwardClientHost(t *testing.T) {
	for _, p := range []string{"../catalog-web/nginx.conf.template", "../config/nginx.conf"} {
		b, err := os.ReadFile(p)
		require.NoError(t, err, p)
		s := string(b)
		assert.Contains(t, s, "proxy_set_header Host $http_host;", p)
		assert.NotContains(t, s, "proxy_set_header Host $host;", p+": port-less Host breaks the /ws Origin check")
	}
}
