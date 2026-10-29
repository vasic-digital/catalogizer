package filesystem

// UNIT-CLASS TEST DOUBLE (constitution §11.4.289 clause C): the responders here are hand-written FTP stand-ins. They prove the client's
// own locking (every operation takes one consistent (conn, base) snapshot under the lock) and its command ordering on the control
// connection. They are NOT evidence that FTP works against a real server (live_nas_test.go is that job).

import (
	"bufio"
	"bytes"
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

// opsLoop runs the operations that read c.client/c.base until stop is closed; a panic (nil *ServerConn dereference) is reported as an error.
func opsLoop(t *testing.T, c *FTPClient, stop <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("operation panicked while a concurrent Disconnect ran: %v", r)
		}
	}()
	ctx := context.Background()
	for {
		select {
		case <-stop:
			return
		default:
		}
		_, _ = c.ListDirectory(ctx, "/d")
		_, _ = c.FileExists(ctx, "/f.bin")
		_, _ = c.GetFileInfo(ctx, "/f.bin")
		_ = c.TestConnection(ctx)
	}
}

// Session-field cycling against concurrent operations: no data race on client/base/connected, no nil dereference.
//
// Scope note (honest): jlaffaye/ftp's *ServerConn is itself not goroutine-safe, so a REAL Disconnect (conn.Quit) overlapping an operation
// that is mid-command on the same connection is a library-level hazard this client cannot remove. The cycler below therefore performs
// exactly the field writes Disconnect/Connect/reconnectIf perform (under c.mu: client=nil, connected=false, gen++ and the re-install of
// client/base/connected) but never Quit()s the connection the operation is using, so the test isolates the client's own locking.
func TestFTPSession_FieldCyclingVsOperations_NoRaceNoPanic(t *testing.T) {
	s := startFakeFTP(t, "426")
	c := fakeClient(t, s)
	c.mu.Lock()
	conn, base := c.client, c.base
	c.mu.Unlock()
	if conn == nil {
		t.Fatalf("no connection after Connect")
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go opsLoop(t, c, stop, &wg)
	const cycles = 3000
	for i := 0; i < cycles; i++ {
		c.mu.Lock() // what Disconnect does, minus the Quit
		c.client = nil
		c.connected = false
		c.gen++
		c.mu.Unlock()
		c.mu.Lock() // what Connect/reconnectIf do on success
		c.client, c.base, c.connected = conn, base, true
		c.mu.Unlock()
	}
	close(stop)
	wg.Wait()
	if err := c.Disconnect(context.Background()); err != nil {
		t.Logf("final Disconnect: %v", err)
	}
	if n := atomic.LoadInt32(&s.accepted); n != 1 {
		t.Fatalf("control connections = %d, want exactly 1 (only the initial Connect; the operations here never redial)", n)
	}
	if c.gen < cycles {
		t.Fatalf("gen = %d, want >= %d", c.gen, cycles)
	}
}

// A client that is not connected at all must answer every operation with an error, never panic.
func TestFTPSession_DisconnectedOperationsReturnNotConnected(t *testing.T) {
	s := startFakeFTP(t, "426")
	c := fakeClient(t, s)
	if err := c.Disconnect(context.Background()); err != nil {
		t.Logf("Disconnect: %v", err)
	}
	ctx := context.Background()
	if _, err := c.ListDirectory(ctx, "/d"); err == nil {
		t.Errorf("ListDirectory on a disconnected client returned nil error")
	}
	if _, err := c.FileExists(ctx, "/f"); err == nil {
		t.Errorf("FileExists on a disconnected client returned nil error")
	}
	if _, err := c.GetFileInfo(ctx, "/f"); err == nil {
		t.Errorf("GetFileInfo on a disconnected client returned nil error")
	}
	if err := c.TestConnection(ctx); err == nil {
		t.Errorf("TestConnection on a disconnected client returned nil error")
	}
	if _, err := c.ReadFile(ctx, "/f"); err == nil {
		t.Errorf("ReadFile on a disconnected client returned nil error")
	}
	if err := c.CopyFile(ctx, "/a", "/b"); err == nil {
		t.Errorf("CopyFile on a disconnected client returned nil error")
	}
}

// strictFTP rejects STOR/MKD with 425 while a RETR data connection is still open (some real servers behave like this).
type strictFTP struct {
	ln       net.Listener
	mu       sync.Mutex
	busy     bool
	rejected int32
	stored   []byte
	storedCh chan struct{}
}

func startStrictFTP(t *testing.T) *strictFTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &strictFTP{ln: ln, storedCh: make(chan struct{}, 1)}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *strictFTP) serve(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	var wmu sync.Mutex
	say := func(f string, a ...interface{}) {
		wmu.Lock()
		defer wmu.Unlock()
		fmt.Fprintf(c, f+"\r\n", a...)
	}
	say("220 strict ready")
	var dl net.Listener
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.ToUpper(strings.Fields(strings.TrimSpace(line) + " x")[0])
		s.mu.Lock()
		busy := s.busy
		s.mu.Unlock()
		switch cmd {
		case "USER":
			say("331 need password")
		case "PASS":
			say("230 ok")
		case "TYPE":
			say("200 type")
		case "PWD":
			say(`257 "/" is cwd`)
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
			s.mu.Lock()
			s.busy = true
			s.mu.Unlock()
			dc.Write([]byte("hello-copy"))
			dc.(*net.TCPConn).CloseWrite()
			go func() { // the transfer is "open" until the client closes its side
				io.Copy(io.Discard, dc)
				dc.Close()
				s.mu.Lock()
				s.busy = false
				s.mu.Unlock()
				say("226 transfer complete")
			}()
		case "MKD":
			if busy {
				atomic.AddInt32(&s.rejected, 1)
				say("425 data connection open")
			} else {
				say("257 created")
			}
		case "STOR":
			if busy {
				atomic.AddInt32(&s.rejected, 1)
				say("425 data connection open")
				continue
			}
			say("150 ok")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			var b bytes.Buffer
			io.Copy(&b, dc)
			dc.Close()
			s.mu.Lock()
			s.stored = b.Bytes()
			s.mu.Unlock()
			say("226 stored")
			select {
			case s.storedCh <- struct{}{}:
			default:
			}
		case "QUIT":
			say("221 bye")
			return
		default:
			say("502 not implemented")
		}
	}
}

// CopyFile must not issue MKD/STOR on the control connection while the RETR data connection is open.
func TestFTPCopyFile_NoControlCommandsWhileRetrDataConnectionOpen(t *testing.T) {
	s := startStrictFTP(t)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: s.ln.Addr().(*net.TCPAddr).Port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	select {
	case <-s.storedCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("server never completed a STOR")
	}
	if n := atomic.LoadInt32(&s.rejected); n != 0 {
		t.Fatalf("server rejected %d commands issued while the RETR data connection was open", n)
	}
	s.mu.Lock()
	got := string(s.stored)
	s.mu.Unlock()
	if got != "hello-copy" {
		t.Fatalf("stored content = %q, want %q", got, "hello-copy")
	}
}

// ftpResolve resolves p against the client's current base (what an operation does after session()).
func ftpResolve(c *FTPClient, p string) string { return c.resolvePath(c.base, p) }
