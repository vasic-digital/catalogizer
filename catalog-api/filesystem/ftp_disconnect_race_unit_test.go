package filesystem

// UNIT-CLASS TEST DOUBLE regression test (F1, independent review round 2; uses the fakeFTP responder): Disconnect() issued while a reader's Close() is
// still redialling. reconnectIf releases c.mu before Connect(), so Connect() later stores the new connection.

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"
)

// slowProxy forwards to the fake FTP server but delays every connection after the first by 300ms.
func TestFTPReadCloser_DisconnectDuringRedialDoesNotResurrect(t *testing.T) {
	s := startFakeFTP(t, "drop")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var n int32
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			k := atomic.AddInt32(&n, 1)
			go func(c net.Conn, k int32) {
				if k > 1 {
					time.Sleep(300 * time.Millisecond)
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
	// The fake answers EPSV with its own port on 127.0.0.1, so the data connection goes direct; only control is proxied.
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	rc := partialRead(t, c)
	done := make(chan error, 1)
	go func() { done <- rc.Close() }()
	time.Sleep(100 * time.Millisecond) // Close is now inside Connect(), waiting for the delayed greeting
	_ = c.Disconnect(context.Background())
	cerr := <-done
	t.Logf("close err=%v proxied_conns=%d", cerr, atomic.LoadInt32(&n))
	if c.IsConnected() {
		t.Fatalf("IsConnected()==true after Disconnect(): the in-flight redial resurrected the client")
	}
}
