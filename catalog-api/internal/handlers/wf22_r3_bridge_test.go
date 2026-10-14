package handlers

// WF22 round 3, K15: a scan.cancelled event reaches a WebSocket client (the subscription list of the bridge covers every routed event type).

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	root_handlers "catalogizer/handlers"
	"catalogizer/internal/eventbus"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWF22R3_K15_ScanCancelledReachesAWebSocketClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ws := root_handlers.NewWebSocketHandler(nil)
	defer ws.Stop()
	r := gin.New()
	r.GET("/ws", ws.HandleConnection)
	srv := httptest.NewServer(r)
	defer srv.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	require.NoError(t, err)
	defer conn.Close()

	bus := eventbus.New(eventbus.DefaultConfig())
	b := NewEventBusBridge(bus, ws, nil)
	b.Start()
	defer b.Stop()
	time.Sleep(100 * time.Millisecond) // let the client register

	for _, c := range []struct {
		typ  eventbus.EventType
		want string
	}{
		{eventbus.EventScanFailed, "Scan failed: r1"},
		{eventbus.EventScanCancelled, "Scan cancelled: r1: operator stopped it"},
	} {
		payload := map[string]interface{}{"storage_root": "r1", "reason": "operator stopped it", "error": ""}
		bus.Publish(eventbus.NewEvent(c.typ, "test", payload))
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		var msg map[string]interface{}
		require.NoError(t, conn.ReadJSON(&msg), "no WebSocket message for %s", c.typ)
		p, _ := msg["payload"].(map[string]interface{})
		assert.Contains(t, p["message"], c.want, string(c.typ))
	}
}
