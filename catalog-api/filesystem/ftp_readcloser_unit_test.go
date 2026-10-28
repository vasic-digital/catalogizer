package filesystem

// UNIT-CLASS TEST DOUBLE (constitution §11.4.289 clause C): this file contains a minimal hand-written FTP protocol responder. It proves
// how ftpReadCloser reacts to specific control-connection behaviours that cannot be provoked on demand against a real server. It is
// NOT evidence that FTP works against a real service; that is the job of live_nas_test.go (//go:build live).

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeFTP struct {
	ln       net.Listener
	mode     string // "drop": close the control connection after RETR is aborted; "426": answer 426 and keep the session; "eofdrop": send the whole file, close data, drop control
	accepted int32
	quits    int32 // QUIT commands received (sessions ended in an orderly way)
	wg       sync.WaitGroup
}

func startFakeFTP(t *testing.T, mode string) *fakeFTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &fakeFTP{ln: ln, mode: mode}
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			atomic.AddInt32(&s.accepted, 1)
			go s.serve(c)
		}
	}()
	t.Cleanup(func() { ln.Close() })
	return s
}

func (s *fakeFTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func (s *fakeFTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	say := func(f string, a ...interface{}) { fmt.Fprintf(c, f+"\r\n", a...) }
	say("220 fake ready")
	var dl net.Listener
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.Fields(strings.TrimSpace(line) + " x")[0])
		switch cmd {
		case "USER":
			say("331 need password")
		case "PASS":
			say("230 ok")
		case "TYPE":
			say("200 type")
		case "PWD":
			say(`257 "/" is cwd`)
		case "NOOP":
			say("200 noop")
		case "EPSV":
			dl, _ = net.Listen("tcp", "127.0.0.1:0")
			say("229 Entering Extended Passive Mode (|||%d|)", dl.Addr().(*net.TCPAddr).Port)
		case "RETR":
			say("150 opening")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			dc.Write(make([]byte, 4096))
			if s.mode == "eofdrop" { // whole file sent, data connection closed, then the control connection dies without a 226
				dc.Close()
				return
			}
			buf := make([]byte, 1)
			dc.SetReadDeadline(time.Now().Add(5 * time.Second))
			dc.Read(buf) // returns when the client closes the data connection
			dc.Close()
			if s.mode == "drop" {
				return
			}
			say("426 transfer aborted")
		case "QUIT":
			atomic.AddInt32(&s.quits, 1)
			say("221 bye")
			return
		default:
			say("502 not implemented")
		}
	}
}

func fakeClient(t *testing.T, s *fakeFTP) *FTPClient {
	t.Helper()
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: s.port(), Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return c
}

func partialRead(t *testing.T, c *FTPClient) interface{ Close() error } {
	t.Helper()
	rc, err := c.ReadFile(context.Background(), "/f.bin")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	buf := make([]byte, 100)
	if _, err := rc.Read(buf); err != nil {
		t.Fatalf("read: %v", err)
	}
	return rc
}

// Partial read, server drops the control connection: Close returns nil AND the session is re-established.
func TestFTPReadCloser_PartialCloseDropsControl_ReconnectsAndReturnsNil(t *testing.T) {
	s := startFakeFTP(t, "drop")
	c := fakeClient(t, s)
	rc := partialRead(t, c)
	if err := rc.Close(); err != nil {
		t.Fatalf("Close after deliberate partial read = %v, want nil", err)
	}
	if !c.IsConnected() {
		t.Fatalf("client not connected after reconnect")
	}
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("session unusable after reconnect: %v", err)
	}
	if n := atomic.LoadInt32(&s.accepted); n != 2 {
		t.Fatalf("control connections accepted = %d, want 2 (original + one reconnect)", n)
	}
}

// A real protocol failure at close (426 with the control connection still alive) must NOT be swallowed, partial read or not.
func TestFTPReadCloser_Real426CloseFailureIsReturned(t *testing.T) {
	s := startFakeFTP(t, "426")
	c := fakeClient(t, s)
	rc := partialRead(t, c)
	if err := rc.Close(); err == nil {
		t.Fatalf("Close swallowed a real 426 failure")
	}
}

// Disconnect followed by Close of an outstanding reader must not dial a new session (reviewer-reproduced defect).
func TestFTPReadCloser_CloseAfterDisconnectDoesNotRedial(t *testing.T) {
	s := startFakeFTP(t, "drop")
	c := fakeClient(t, s)
	rc := partialRead(t, c)
	_ = c.Disconnect(context.Background())
	_ = rc.Close()
	if c.IsConnected() {
		t.Fatalf("IsConnected true after Disconnect+Close: Close redialled")
	}
	time.Sleep(200 * time.Millisecond)
	if n := atomic.LoadInt32(&s.accepted); n != 1 {
		t.Fatalf("control connections accepted = %d, want 1 (no redial after Disconnect)", n)
	}
}

// F2: a transfer read to its END and then a control drop is a real failure: Close must return the error, not swallow it as an early abort.
func TestFTPReadCloser_ReadToEOFThenControlDropIsReturned(t *testing.T) {
	s := startFakeFTP(t, "eofdrop")
	c := fakeClient(t, s)
	rc, err := c.ReadFile(context.Background(), "/f.bin")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if _, err := io.Copy(io.Discard, rc); err != nil {
		t.Fatalf("read to EOF: %v", err)
	}
	if err := rc.Close(); err == nil {
		t.Fatalf("Close swallowed a control drop after the transfer was read to EOF")
	}
}

// F3: after a real 426 the old control session must be ended with QUIT (not leaked), exactly one replacement is dialled, and the client works.
func TestFTPReadCloser_426QuitsOldSessionAndClientUsable(t *testing.T) {
	s := startFakeFTP(t, "426")
	c := fakeClient(t, s)
	rc := partialRead(t, c)
	if err := rc.Close(); err == nil {
		t.Fatalf("expected the 426 error")
	}
	time.Sleep(100 * time.Millisecond) // the fake counts QUIT asynchronously
	if n := atomic.LoadInt32(&s.quits); n != 1 {
		t.Fatalf("QUIT received %d times, want 1 (old session must be Quit)", n)
	}
	if n := atomic.LoadInt32(&s.accepted); n != 2 {
		t.Fatalf("control connections = %d, want 2", n)
	}
	if !c.IsConnected() || c.TestConnection(context.Background()) != nil {
		t.Fatalf("client unusable after Close")
	}
}
