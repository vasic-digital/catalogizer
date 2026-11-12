package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func testWebSocketConfig() WebSocketConfig {
	cfg := DefaultWebSocketConfig()
	cfg.PongWait = 3 * time.Second
	cfg.PingInterval = 1 * time.Second
	cfg.WriteWait = 2 * time.Second
	return cfg
}

func TestWebSocketHandler_HandleConnection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewWebSocketHandlerWithConfig(zap.NewNop(), testWebSocketConfig())

	router := gin.New()
	router.GET("/ws", handler.HandleConnection)
	server := httptest.NewServer(router)

	wsURL := "ws" + server.URL[4:] + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)

	// Send ping message
	err = conn.WriteJSON(map[string]interface{}{
		"type": "ping",
	})
	require.NoError(t, err)

	// Expect pong response
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	messageType, message, err := conn.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, websocket.TextMessage, messageType)
	assert.Contains(t, string(message), "pong")

	// Verify connection stats
	stats := handler.GetStats()
	assert.Equal(t, int64(1), stats.ActiveConnections)

	// Cleanup: stop handler to close server-side connections and unblock readPump
	handler.Stop()
	conn.Close()
	server.Close()
}

func TestWebSocketHandler_BroadcastToClients(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewWebSocketHandlerWithConfig(zap.NewNop(), testWebSocketConfig())

	router := gin.New()
	router.GET("/ws", handler.HandleConnection)
	server := httptest.NewServer(router)

	wsURL := "ws" + server.URL[4:] + "/ws"

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Give server time to register both connections
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	handler.BroadcastToClients(map[string]interface{}{
		"type":    "test",
		"message": "hello",
	})

	// Both connections should receive the broadcast
	conn1.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg1, err := conn1.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg1), "hello")

	conn2.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg2, err := conn2.ReadMessage()
	require.NoError(t, err)
	assert.Contains(t, string(msg2), "hello")

	// Cleanup
	handler.Stop()
	conn1.Close()
	conn2.Close()
	server.Close()
}

func TestOriginChecker_EnvDefaultAndSameHost(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://a.example.test, https://b.example.test")
	chk := originChecker(nil)
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/ws", nil)
		r.Host = host
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		return r
	}
	assert.True(t, chk(mk("", "api.local")), "no Origin header")
	assert.True(t, chk(mk("https://b.example.test", "api.local")), "env allow-list, trimmed")
	assert.True(t, chk(mk("https://API.local", "api.local")), "same host, case-insensitive")
	assert.False(t, chk(mk("https://evil.test", "api.local")))
	assert.False(t, chk(mk("not a url", "api.local")))
	assert.False(t, chk(mk("https://a.example.test.evil.test", "api.local")))
}

func TestOriginChecker_HostPortAndSchemeMatrix(t *testing.T) {
	chk := originChecker([]string{"https://catalog.example.test", "http://lan.example.test:8080"})
	mk := func(origin, host string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/ws", nil)
		r.Host = host
		r.Header.Set("Origin", origin)
		return r
	}
	cases := []struct {
		name, origin, host string
		ok                 bool
	}{
		{"LAN ip:3000, Host keeps the port (nginx $http_host)", "http://192.168.1.5:3000", "192.168.1.5:3000", true},
		{"same host, different port", "http://192.168.1.5:3000", "192.168.1.5:3001", false},
		{"Origin non-default port vs Host without port (nginx $host)", "http://192.168.1.5:3000", "192.168.1.5", false},
		{"default port written vs absent", "http://site.test:80", "site.test", true},
		{"https default port vs Host :443", "https://site.test", "site.test:443", true},
		{"hostname-only loosening must not pass: other port", "http://site.test:9999", "site.test:80", false},
		{"IPv6 literal with port", "http://[::1]:3000", "[::1]:3000", true},
		{"IPv6 literal other port", "http://[::1]:3000", "[::1]:3001", false},
		{"allow-list https entry", "https://catalog.example.test", "api.internal:8080", true},
		{"allow-list scheme mismatch (http vs https entry)", "http://catalog.example.test", "api.internal:8080", false},
		{"allow-list entry with port", "http://lan.example.test:8080", "api.internal:8080", true},
		{"allow-list entry, wrong port", "http://lan.example.test:8081", "api.internal:8080", false},
		{"non-http scheme", "file://catalog.example.test", "catalog.example.test", false},
		{"null origin", "null", "site.test", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.ok, chk(mk(tc.origin, tc.host)))
		})
	}
}
