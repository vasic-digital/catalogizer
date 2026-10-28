package filesystem

// UNIT-CLASS TEST DOUBLE regression tests (constitution §11.4.289 clause C) for the races between ftpReadCloser.Close's redial and
// Disconnect / Connect. They use the fakeFTP responder from ftp_readcloser_unit_test.go behind a small TCP proxy that can delay or
// refuse chosen control connections, so the redial can be held "in flight" deterministically. They prove the client's locking and
// generation logic only; they say nothing about a real FTP server (that is live_nas_test.go).
//
// Every test asserts its PRECONDITION (the exact number of proxied control connections), so a run that never reached the race
// cannot pass (§11.4.199).

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// plan decides, for the k-th control connection (1-based), how long the proxy waits before connecting upstream and whether it refuses.
type plan func(k int32) (delay time.Duration, refuse bool)

// startProxy forwards control connections to the fake FTP server according to plan and counts them in *n.
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
				delay, refuse := p(k)
				time.Sleep(delay)
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

// F1: Disconnect issued while the reader's Close is still redialling must win: the client stays disconnected and the connection the
// redial eventually opens is ended with QUIT (not leaked). Control connections: #1 original, #2 the redial (held 300ms).
func TestFTPReadCloser_DisconnectDuringRedialDoesNotResurrect(t *testing.T) {
	s := startFakeFTP(t, "drop")
	var n int32
	port := startProxy(t, s, &n, func(k int32) (time.Duration, bool) {
		if k == 2 {
			return 300 * time.Millisecond, false
		}
		return 0, false
	})
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := partialRead(t, c)
	done := make(chan error, 1)
	go func() { done <- rc.Close() }()
	time.Sleep(100 * time.Millisecond) // Close is now inside dial(), waiting for the delayed greeting of connection #2
	_ = c.Disconnect(context.Background())
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Fatalf("proxied control connections = %d, want 2 (precondition: the redial must have been in flight)", got)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true after Disconnect(): the in-flight redial resurrected the client")
	}
	// The fake already dropped original session #1 (mode "drop"), so its QUIT never arrives; the single QUIT is the discarded redial connection.
	waitQuits(t, s, 1)
}

// Connect issued while a redial is in flight: Connect's connection is the live one, the redial's connection is discarded with QUIT.
// Control connections: #1 original, #2 the redial (held 400ms), #3 the explicit Connect.
func TestFTPReadCloser_ConnectDuringRedialKeepsConnectsConnection(t *testing.T) {
	s := startFakeFTP(t, "drop")
	var n int32
	port := startProxy(t, s, &n, func(k int32) (time.Duration, bool) {
		if k == 2 {
			return 400 * time.Millisecond, false
		}
		return 0, false
	})
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := partialRead(t, c)
	done := make(chan error, 1)
	go func() { done <- rc.Close() }()
	time.Sleep(100 * time.Millisecond) // the redial (#2) is waiting for its delayed greeting
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	c.mu.Lock()
	connectsConn := c.client
	c.mu.Unlock()
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(&n); got != 3 {
		t.Fatalf("proxied control connections = %d, want 3 (precondition: Connect ran during the redial)", got)
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
// Control connections: #1 original, #2 the redial (held 300ms, then refused).
func TestFTPReadCloser_DisconnectThenRedialFailureIsNotAnError(t *testing.T) {
	s := startFakeFTP(t, "drop")
	var n int32
	port := startProxy(t, s, &n, func(k int32) (time.Duration, bool) {
		if k == 2 {
			return 300 * time.Millisecond, true
		}
		return 0, false
	})
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := partialRead(t, c)
	done := make(chan error, 1)
	go func() { done <- rc.Close() }()
	time.Sleep(100 * time.Millisecond)
	_ = c.Disconnect(context.Background())
	if cerr := <-done; cerr != nil {
		t.Fatalf("Close after an intentional Disconnect = %v, want nil", cerr)
	}
	if got := atomic.LoadInt32(&n); got != 2 {
		t.Fatalf("proxied control connections = %d, want 2 (precondition: the redial must have been attempted)", got)
	}
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true after Disconnect()")
	}
}
