package filesystem

import (
	"context"
	"net"
	"testing"
)

// Unit (hermetic): Disconnect must forget the transport handle so a later call cannot reuse a closed connection.
func TestSmbClient_Disconnect_ForgetsHandles(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	c := NewSmbClient(&SmbConfig{Host: "unused", Port: 445, Share: "x"})
	c.conn = a
	_ = c.Disconnect(context.Background())
	if c.conn != nil || c.session != nil || c.share != nil {
		t.Fatalf("Disconnect left handles set: conn=%v session=%v share=%v", c.conn != nil, c.session != nil, c.share != nil)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected true after Disconnect")
	}
}
