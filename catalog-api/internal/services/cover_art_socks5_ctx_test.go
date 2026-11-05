package services

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type socksProxyCfg struct{ addr string }

func (c socksProxyCfg) IsEnabled() bool     { return true }
func (c socksProxyCfg) GetURL() string      { return "socks5://" + c.addr }
func (c socksProxyCfg) GetHTTPURL() string  { return "" }
func (c socksProxyCfg) GetUsername() string { return "" }
func (c socksProxyCfg) GetPassword() string { return "" }

// SSRF-F1: the SOCKS5 handshake is bounded by the fixed socks5DialTimeout. net/http detaches the
// dial from the request context, so the caller's context does not reach it, and a proxy that
// answers slowly but steadily is not detected (SECQ-7). A real listener accepts the TCP
// connection and never answers the handshake; the half-open connection must be closed once
// the (lowered) dial timeout has passed.
func TestDownloadImage_Socks5HandshakeIsBoundedByDialTimeout(t *testing.T) {
	AllowPrivateSSRFForTests(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()
	conns := make(chan net.Conn, 4)
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns <- c // never answers the handshake
		}
	}()
	oldT := socks5DialTimeout
	socks5DialTimeout = 400 * time.Millisecond
	t.Cleanup(func() { socks5DialTimeout = oldT })
	s := NewCoverArtService(nil, zap.NewNop())
	s.SetProxyConfig(socksProxyCfg{addr: ln.Addr().String()})
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_, err = s.downloadImage(ctx, "http://203.0.113.9/cover.jpg")
	require.Error(t, err)
	var c net.Conn
	select {
	case c = <-conns: // control: the request really went to the proxy listener
	case <-time.After(2 * time.Second):
		t.Fatal("control failed: the SOCKS5 listener was never contacted")
	}
	defer c.Close()
	// The http transport abandons a blocked dial when ctx ends, so the call's own return time
	// proves nothing. What distinguishes a context-aware dialer is that the dial itself is
	// aborted: the client side closes the half-open handshake connection promptly. A dialer
	// that ignores ctx keeps it open until the 30 s client timeout.
	require.NoError(t, c.SetReadDeadline(time.Now().Add(3*time.Second)))
	var rerr error
	for rerr == nil { // the client's SOCKS5 greeting arrives first; drain until the close or the deadline
		_, rerr = c.Read(make([]byte, 16))
	}
	var ne net.Error
	if errors.As(rerr, &ne) && ne.Timeout() {
		t.Fatalf("the SOCKS5 handshake is not bounded: connection still open 3 s after the dial timeout")
	}
	assert.Error(t, rerr, "expected the client to close the connection (EOF/reset)")
}

// The production default is bounded (tests lower the variable).
func TestSocks5DialTimeout_ProductionDefaultIsBounded(t *testing.T) {
	assert.Equal(t, 30*time.Second, socks5DialTimeout)
}
