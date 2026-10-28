package filesystem

// UNIT-CLASS TEST DOUBLE regression tests (constitution §11.4.289 clause C) for the races between ftpReadCloser.Close's redial and
// Disconnect / Connect. They use the fakeFTP responder from ftp_readcloser_unit_test.go behind a small TCP proxy that can delay or
// refuse chosen control connections, so the redial can be held "in flight" deterministically. They prove the client's locking and
// generation logic only; they say nothing about a real FTP server (that is live_nas_test.go).
//
// The race is made deterministic with a GATE, not with sleeps: the proxy holds the redial's control connection on a channel, the test
// waits until the proxy has actually accepted it (the redial is provably in flight: reconnectIf has released the lock and is inside
// dial()), performs Disconnect/Connect, and only then releases the gate. Every test also asserts the exact number of proxied control
// connections, so a run that never reached the scenario cannot pass (§11.4.199).

import (
	"context"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// plan decides, for the k-th control connection (1-based), a gate the proxy waits on before connecting upstream (nil = no wait) and
// whether the connection is then refused (closed without a greeting).
type plan func(k int32) (gate chan struct{}, refuse bool)

// startProxy forwards control connections to the fake FTP server according to plan and counts accepted ones in *n.
func startProxy(t *testing.T, s *fakeFTP, n *int32, p plan) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			k := atomic.AddInt32(n, 1)
			go func(c net.Conn, k int32) {
				gate, refuse := p(k)
				if gate != nil {
					<-gate
				}
				if refuse {
					c.Close()
					return
				}
				u, err := net.Dial("tcp", s.ln.Addr().String())
				if err != nil {
					c.Close()
					return
				}
				go func() {
					buf := make([]byte, 4096)
					for {
						m, e := u.Read(buf)
						if m > 0 {
							c.Write(buf[:m])
						}
						if e != nil {
							c.Close()
							return
						}
					}
				}()
				buf := make([]byte, 4096)
				for {
					m, e := c.Read(buf)
					if m > 0 {
						u.Write(buf[:m])
					}
					if e != nil {
						u.Close()
						return
					}
				}
			}(c, k)
		}
	}()
	// The fake answers EPSV with its own port on 127.0.0.1, so data connections go direct; only the control channel is proxied.
	return ln.Addr().(*net.TCPAddr).Port
}

// waitAccepted blocks until the proxy has accepted at least want control connections (the redial is then in flight).
func waitAccepted(t *testing.T, n *int32, want int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for atomic.LoadInt32(n) < want {
		if time.Now().After(deadline) {
			t.Fatalf("proxy accepted %d control connections, want %d (the redial never started)", atomic.LoadInt32(n), want)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func waitQuits(t *testing.T, s *fakeFTP, want int32) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt32(&s.quits) >= want {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(150 * time.Millisecond) // a surplus QUIT would show up here
	if got := atomic.LoadInt32(&s.quits); got != want {
		t.Fatalf("QUIT received %d times, want exactly %d", got, want)
	}
}

// gatedSetup connects a client through a proxy whose control connection #2 (the redial) waits on gate, starts Close of a partially
// read transfer in a goroutine and returns once the redial is provably in flight.
func gatedSetup(t *testing.T, refuseAfterGate bool) (c *FTPClient, s *fakeFTP, n *int32, gate chan struct{}, done chan error) {
	t.Helper()
	s = startFakeFTP(t, "drop")
	n = new(int32)
	gate = make(chan struct{})
	port := startProxy(t, s, n, func(k int32) (chan struct{}, bool) {
		if k == 2 {
			return gate, refuseAfterGate
		}
		return nil, false
	})
	c = NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) }) // runs after the test body (and its waitQuits), before the listeners close
	rc := partialRead(t, c)
	done = make(chan error, 1)
	go func() { done <- rc.Close() }()
	waitAccepted(t, n, 2)
	return
}

// F1: Disconnect issued while the redial is in flight must win: the client stays disconnected and the connection the redial eventually
// opens is ended with QUIT (not leaked). Control connections: #1 original, #2 the redial (gated).
func TestFTPReadCloser_DisconnectDuringRedialDoesNotResurrect(t *testing.T) {
	c, s, n, gate, done := gatedSetup(t, false)
	_ = c.Disconnect(context.Background())
	close(gate)
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(n); got != 2 {
		t.Fatalf("proxied control connections = %d, want 2", got)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true after Disconnect(): the in-flight redial resurrected the client")
	}
	// The fake already dropped original session #1 (mode "drop"), so its QUIT never arrives; the single QUIT is the discarded redial connection.
	waitQuits(t, s, 1)
}

// Connect issued while the redial is in flight: Connect's connection is the live one, the redial's connection is discarded with QUIT.
// Control connections: #1 original, #2 the redial (gated), #3 the explicit Connect.
func TestFTPReadCloser_ConnectDuringRedialKeepsConnectsConnection(t *testing.T) {
	c, s, n, gate, done := gatedSetup(t, false)
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	c.mu.Lock()
	connectsConn := c.client
	c.mu.Unlock()
	close(gate)
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(n); got != 3 {
		t.Fatalf("proxied control connections = %d, want 3", got)
	}
	c.mu.Lock()
	after := c.client
	c.mu.Unlock()
	if after == nil || after != connectsConn {
		t.Fatalf("the redial replaced the connection made by Connect (or left no connection)")
	}
	if !c.IsConnected() || c.TestConnection(context.Background()) != nil {
		t.Fatalf("Connect's connection is not usable")
	}
	// Original session #1 was dropped by the fake (no QUIT arrives); the single QUIT is the discarded redial connection (#2).
	// Connect's connection (#3) must stay open, so it must not have been QUIT.
	waitQuits(t, s, 1)
}

// Disconnect wins and the in-flight redial then fails: the user disconnected on purpose, so Close must not report "reconnect failed".
func TestFTPReadCloser_DisconnectThenRedialFailureIsNotAnError(t *testing.T) {
	c, _, n, gate, done := gatedSetup(t, true)
	_ = c.Disconnect(context.Background())
	close(gate) // connection #2 is now refused
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close after an intentional Disconnect = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(n); got != 2 {
		t.Fatalf("proxied control connections = %d, want 2", got)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true after Disconnect()")
	}
}

// The opposite case: NO Disconnect and the redial fails: the failure must be reported (mutation guard for the "disconnected" branch).
func TestFTPReadCloser_RedialFailureWithoutDisconnectIsReported(t *testing.T) {
	c, _, n, gate, done := gatedSetup(t, true)
	close(gate) // connection #2 is refused, nobody disconnected
	cerr := <-done
	if cerr == nil || !strings.Contains(cerr.Error(), "reconnect failed") {
		t.Fatalf("Close = %v, want an error containing \"reconnect failed\"", cerr)
	}
	if got := atomic.LoadInt32(n); got != 2 {
		t.Fatalf("proxied control connections = %d, want 2", got)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true although the redial failed")
	}
}
