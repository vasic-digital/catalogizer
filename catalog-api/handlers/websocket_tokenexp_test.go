package handlers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// SEC-WS-1F3FU-8: a frame arriving after the JWT expired but inside the per-message throttle window must be refused
// (previously the throttle shortcut returned true before tokenExp was looked at, so frames were accepted for up to
// MessageRecheckMinInterval past expiry).
func TestSEC_WS_1F3FU8_ExpiredTokenRefusedInsideThrottleWindow(t *testing.T) {
	cfg := testWebSocketConfig()
	cfg.MessageRecheckMinInterval = 10 * time.Second // far larger than the token's remaining life
	h := NewWebSocketHandlerWithConfig(zap.NewNop(), cfg)

	// Short-lived token; the connection was authorised just now, so the throttle window is wide open.
	wc := &wsConn{tokenExp: time.Now().Add(time.Second)} // F3T-4: ~1 s margin before the first assertion
	wc.lastOK.Store(time.Now().UnixNano())

	assert.True(t, h.authorizeMessage(wc), "frame before expiry is accepted (throttled shortcut)")

	time.Sleep(1300 * time.Millisecond) // now past tokenExp, still inside the 10 s throttle window
	assert.False(t, h.authorizeMessage(wc), "frame after JWT expiry must be refused even inside the throttle window")

	// A token with no exp claim keeps the old behaviour (only the session check bounds it).
	noExp := &wsConn{}
	noExp.lastOK.Store(time.Now().UnixNano())
	assert.True(t, h.authorizeMessage(noExp))
}
