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
