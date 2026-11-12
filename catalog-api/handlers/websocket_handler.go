package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// WebSocketConfig holds configuration for the WebSocket handler
type WebSocketConfig struct {
	MaxConnections    int
	ReadBufferSize    int
	WriteBufferSize   int
	PingInterval      time.Duration
	PongWait          time.Duration
	WriteWait         time.Duration
	MaxMessageSize    int64
	EnableCompression bool
	// AllowedOrigins lists the browser Origins (scheme://host[:port]) accepted besides
	// the request's own host. Empty falls back to CORS_ALLOWED_ORIGINS / the dev defaults.
	AllowedOrigins []string
}

// DefaultWebSocketConfig returns default configuration
func DefaultWebSocketConfig() WebSocketConfig {
	return WebSocketConfig{
		MaxConnections:    1000,
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
		PingInterval:      30 * time.Second,
		PongWait:          60 * time.Second,
		WriteWait:         10 * time.Second,
		MaxMessageSize:    512 * 1024, // 512KB
		EnableCompression: false,
	}
}

// wsConn wraps a websocket.Conn with thread-safe operations and lifecycle management.
// gorilla/websocket connections do not support concurrent writers, so all
// WriteMessage calls must be serialized per connection.
type wsConn struct {
	conn      *websocket.Conn
	wmu       sync.Mutex
	id        string
	connected time.Time
	lastPing  time.Time
	send      chan []byte
	done      chan struct{}
	logger    *zap.Logger
}

// writeMessage safely writes a message to the WebSocket connection with timeout.
func (wc *wsConn) writeMessage(messageType int, data []byte) error {
	wc.wmu.Lock()
	defer wc.wmu.Unlock()

	// Set write deadline
	wc.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer wc.conn.SetWriteDeadline(time.Time{})

	return wc.conn.WriteMessage(messageType, data)
}

// close safely closes the WebSocket connection.
func (wc *wsConn) close() {
	wc.wmu.Lock()
	defer wc.wmu.Unlock()

	select {
	case <-wc.done:
		// Already closed
		return
	default:
		close(wc.done)
	}

	// Set an immediate read deadline to unblock any ReadMessage call,
	// then close the underlying connection
	wc.conn.SetReadDeadline(time.Now())
	wc.conn.Close()
}

// WebSocketHandler manages WebSocket connections for real-time updates.
type WebSocketHandler struct {
	clients  map[*wsConn]bool
	mu       sync.RWMutex
	upgrader *websocket.Upgrader
	config   WebSocketConfig
	logger   *zap.Logger

	// Connection management
	connCount  int64
	maxReached int64

	// Cleanup
	ticker   *time.Ticker
	stopChan chan struct{}
	stopOnce sync.Once
	wg       sync.WaitGroup
}

// NewWebSocketHandler creates a new WebSocket handler with lifecycle management.
func NewWebSocketHandler(logger *zap.Logger) *WebSocketHandler {
	return NewWebSocketHandlerWithConfig(logger, DefaultWebSocketConfig())
}

// NewWebSocketHandlerWithConfig creates a new WebSocket handler with custom configuration.
func NewWebSocketHandlerWithConfig(logger *zap.Logger, config WebSocketConfig) *WebSocketHandler {
	if logger == nil {
		logger = zap.NewNop()
	}

	h := &WebSocketHandler{
		clients:  make(map[*wsConn]bool),
		config:   config,
		logger:   logger,
		stopChan: make(chan struct{}),
		upgrader: &websocket.Upgrader{
			ReadBufferSize:    config.ReadBufferSize,
			WriteBufferSize:   config.WriteBufferSize,
			CheckOrigin:       originChecker(config.AllowedOrigins),
			EnableCompression: config.EnableCompression,
		},
	}

	// Start cleanup goroutine
	h.ticker = time.NewTicker(config.PingInterval)
	h.wg.Add(1)
	go h.cleanupLoop()

	logger.Info("WebSocket handler created",
		zap.Int("max_connections", config.MaxConnections),
		zap.Duration("ping_interval", config.PingInterval))

	return h
}

// Stop gracefully shuts down the WebSocket handler.
func (h *WebSocketHandler) Stop() {
	h.stopOnce.Do(func() {
		h.logger.Info("Stopping WebSocket handler")

		// Signal stop
		close(h.stopChan)

		// Stop ticker
		h.ticker.Stop()

		// Close all connections
		h.mu.Lock()
		clients := make([]*wsConn, 0, len(h.clients))
		for wc := range h.clients {
			clients = append(clients, wc)
		}
		h.mu.Unlock()

		for _, wc := range clients {
			wc.close()
		}

		// Wait for cleanup goroutine
		h.wg.Wait()

		h.logger.Info("WebSocket handler stopped")
	})
}

// cleanupLoop periodically cleans up stale connections.
func (h *WebSocketHandler) cleanupLoop() {
	defer h.wg.Done()

	for {
		select {
		case <-h.ticker.C:
			h.cleanupStaleConnections()
		case <-h.stopChan:
			return
		}
	}
}

// cleanupStaleConnections removes connections that haven't responded to pings.
func (h *WebSocketHandler) cleanupStaleConnections() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	stale := make([]*wsConn, 0)

	for wc := range h.clients {
		if now.Sub(wc.lastPing) > h.config.PongWait {
			stale = append(stale, wc)
		}
	}

	for _, wc := range stale {
		delete(h.clients, wc)
		wc.close()
		h.connCount--
	}

	if len(stale) > 0 {
		h.logger.Info("Cleaned up stale WebSocket connections",
			zap.Int("count", len(stale)),
			zap.Int64("active", h.connCount))
	}
}

// GetStats returns WebSocket connection statistics.
func (h *WebSocketHandler) GetStats() WebSocketStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return WebSocketStats{
		ActiveConnections: int64(len(h.clients)),
		MaxConnections:    int64(h.config.MaxConnections),
		MaxReached:        h.maxReached,
	}
}

// WebSocketStats holds connection statistics.
type WebSocketStats struct {
	ActiveConnections int64
	MaxConnections    int64
	MaxReached        int64
}

// HandleConnection upgrades an HTTP request to a WebSocket connection
// and manages the connection lifecycle with proper cleanup.
func (h *WebSocketHandler) HandleConnection(c *gin.Context) {
	// Check connection limit atomically before upgrade
	h.mu.Lock()
	if len(h.clients) >= h.config.MaxConnections {
		h.mu.Unlock()
		h.logger.Warn("WebSocket connection limit reached",
			zap.Int("limit", h.config.MaxConnections))
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":       "Server at capacity",
			"retry_after": 30,
		})
		return
	}
	// Hold a reservation by pre-incrementing connCount.
	// If upgrade fails below, we decrement it back.
	h.connCount++
	h.mu.Unlock()

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		// Warn, not Error: a refused Origin or a bad handshake is a client condition.
		h.logger.Warn("WebSocket upgrade refused", zap.Error(err))
		h.mu.Lock()
		h.connCount--
		h.mu.Unlock()
		return
	}

	// Configure connection
	conn.SetReadLimit(h.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(h.config.PongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(h.config.PongWait))
		return nil
	})

	wc := &wsConn{
		conn:      conn,
		id:        generateConnID(),
		connected: time.Now(),
		lastPing:  time.Now(),
		send:      make(chan []byte, 256),
		done:      make(chan struct{}),
		logger:    h.logger,
	}

	h.mu.Lock()
	h.clients[wc] = true
	if h.connCount > h.maxReached {
		h.maxReached = h.connCount
	}
	active := h.connCount
	h.mu.Unlock()

	h.logger.Info("WebSocket client connected",
		zap.String("id", wc.id),
		zap.Int64("active", active))

	// Start goroutines for reading and writing
	go h.writePump(wc)
	h.readPump(wc)
}

// readPump pumps messages from the WebSocket connection to the hub.
func (h *WebSocketHandler) readPump(wc *wsConn) {
	defer func() {
		h.removeClient(wc)
		wc.close()
	}()

	wc.conn.SetReadLimit(h.config.MaxMessageSize)
	wc.conn.SetReadDeadline(time.Now().Add(h.config.PongWait))
	wc.conn.SetPongHandler(func(string) error {
		wc.lastPing = time.Now()
		wc.conn.SetReadDeadline(time.Now().Add(h.config.PongWait))
		return nil
	})

	for {
		messageType, message, err := wc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				h.logger.Error("WebSocket error", zap.Error(err), zap.String("id", wc.id))
			}
			break
		}

		if messageType == websocket.TextMessage {
			h.handleMessage(wc, message)
		}
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (h *WebSocketHandler) writePump(wc *wsConn) {
	ticker := time.NewTicker(h.config.PingInterval)
	defer func() {
		ticker.Stop()
		wc.close()
	}()

	for {
		select {
		case message, ok := <-wc.send:
			wc.conn.SetWriteDeadline(time.Now().Add(h.config.WriteWait))
			if !ok {
				wc.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := wc.writeMessage(websocket.TextMessage, message); err != nil {
				h.logger.Error("WebSocket write error", zap.Error(err), zap.String("id", wc.id))
				return
			}

		case <-ticker.C:
			wc.conn.SetWriteDeadline(time.Now().Add(h.config.WriteWait))
			if err := wc.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}

		case <-wc.done:
			return

		case <-h.stopChan:
			return
		}
	}
}

// removeClient removes a client from the hub.
func (h *WebSocketHandler) removeClient(wc *wsConn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[wc]; ok {
		delete(h.clients, wc)
		h.connCount--

		h.logger.Info("WebSocket client disconnected",
			zap.String("id", wc.id),
			zap.Int64("active", h.connCount))
	}
}

// generateConnID generates a unique connection ID.
func generateConnID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of specified length.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

// BroadcastToClients sends a message to all connected WebSocket clients.
// Concurrency is bounded by each client's buffered send channel (capacity 256).
// If a client's buffer is full the message is dropped for that client rather
// than blocking the publisher, so no additional semaphore is needed here.
func (h *WebSocketHandler) BroadcastToClients(msg map[string]interface{}) {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal broadcast message", zap.Error(err))
		return
	}

	// Snapshot client list under lock, then release before writing
	h.mu.RLock()
	clients := make([]*wsConn, 0, len(h.clients))
	for wc := range h.clients {
		clients = append(clients, wc)
	}
	h.mu.RUnlock()

	for _, wc := range clients {
		select {
		case wc.send <- data:
		default:
			// Client send buffer full, skip
			h.logger.Warn("WebSocket client send buffer full, dropping message",
				zap.String("id", wc.id))
		}
	}
}

// BroadcastToClientsContext sends a message with context cancellation support.
func (h *WebSocketHandler) BroadcastToClientsContext(ctx context.Context, msg map[string]interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Snapshot client list
	h.mu.RLock()
	clients := make([]*wsConn, 0, len(h.clients))
	for wc := range h.clients {
		clients = append(clients, wc)
	}
	h.mu.RUnlock()

	// Send with context
	for _, wc := range clients {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case wc.send <- data:
		default:
			// Buffer full
		}
	}

	return nil
}

// handleMessage processes incoming WebSocket messages.
func (h *WebSocketHandler) handleMessage(wc *wsConn, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		h.logger.Error("Failed to unmarshal WebSocket message", zap.Error(err))
		return
	}

	msgType, _ := msg["type"].(string)
	switch msgType {
	case "subscribe":
		channel, _ := msg["channel"].(string)
		ack := map[string]interface{}{
			"type":      "subscribed",
			"channel":   channel,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if data, err := json.Marshal(ack); err == nil {
			select {
			case wc.send <- data:
			default:
			}
		}

	case "unsubscribe":
		channel, _ := msg["channel"].(string)
		ack := map[string]interface{}{
			"type":      "unsubscribed",
			"channel":   channel,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if data, err := json.Marshal(ack); err == nil {
			select {
			case wc.send <- data:
			default:
			}
		}

	case "ping":
		wc.lastPing = time.Now()
		pong := map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		if data, err := json.Marshal(pong); err == nil {
			select {
			case wc.send <- data:
			default:
			}
		}

	default:
		h.logger.Warn("Unknown WebSocket message type",
			zap.String("type", msgType),
			zap.String("id", wc.id))
	}
}

// defaultAllowedOrigins returns the browser origins accepted for the upgrade: the
// operator's CORS_ALLOWED_ORIGINS (the same list the REST CORS middleware uses),
// or the local dev servers when unset.
func defaultAllowedOrigins() []string {
	raw := os.Getenv("CORS_ALLOWED_ORIGINS")
	if raw == "" {
		raw = "http://localhost:5173,http://localhost:3000"
	}
	var out []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			out = append(out, o)
		}
	}
	return out
}

// originParts splits an absolute origin URL into lower-cased scheme, host and the
// effective port (the scheme default 80/443 when none is written). ok is false for
// anything that is not a plain http(s)/ws(s) origin with a host.
func originParts(raw string) (scheme, host, port string, explicit, ok bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Hostname() == "" {
		return "", "", "", false, false
	}
	scheme = strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "ws":
		scheme, port = "http", "80"
	case "https", "wss":
		scheme, port = "https", "443"
	default:
		return "", "", "", false, false
	}
	if p := u.Port(); p != "" {
		port, explicit = p, true
	}
	return scheme, strings.ToLower(u.Hostname()), port, explicit, true
}

// originChecker builds the Upgrader.CheckOrigin policy (SEC-WS-1): no Origin header
// (native clients such as Android do not send one; they are still authenticated by
// the route) is accepted; a browser Origin must match the request's own Host
// (hostname AND port) or an exact scheme+host+port entry of the allow-list.
// Everything else is refused, which stops cross-site WebSocket hijacking from a
// foreign page. A Host header without a port can only match an Origin on its
// scheme's default port; the reverse proxy must therefore forward the client's Host
// (nginx: proxy_set_header Host $http_host). X-Forwarded-Host is never read.
func originChecker(allowed []string) func(*http.Request) bool {
	if len(allowed) == 0 {
		allowed = defaultAllowedOrigins()
	}
	return func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		scheme, host, port, explicit, ok := originParts(origin)
		if !ok {
			return false
		}
		if hu, err := url.Parse("//" + r.Host); err == nil && hu.Hostname() != "" {
			if strings.EqualFold(hu.Hostname(), host) {
				if hp := hu.Port(); hp != "" {
					if hp == port {
						return true
					}
				} else if !explicit || port == map[string]string{"http": "80", "https": "443"}[scheme] {
					return true
				}
			}
		}
		for _, a := range allowed {
			as, ah, ap, _, aok := originParts(a)
			if aok && as == scheme && ah == host && ap == port {
				return true
			}
		}
		return false
	}
}
