package filesystem

// UNIT-CLASS (constitution 11.4.289 C): hand-written fake servers that misbehave in ways a real server rarely can on demand (silent data
// connection, no greeting). They prove the client's own deadline/lock bookkeeping; the real-server behaviour is in ftp_realserver_live_test.go.

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// silentDataServer: a normal control session, but the passive data connection it announces accepts and never sends a byte, and LIST
// is answered "150" and then nothing.
func silentDataServer(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	dln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { dln.Close() })
	go func() {
		for {
			c, err := dln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	dport := dln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				r := bufio.NewReader(conn)
				fmt.Fprint(conn, "220 ready\r\n")
				for {
					line, err := r.ReadString('\n')
					if err != nil {
						return
					}
					cmd := strings.ToUpper(strings.TrimSpace(line))
					switch {
					case strings.HasPrefix(cmd, "USER"):
						fmt.Fprint(conn, "331 pw\r\n")
					case strings.HasPrefix(cmd, "PASS"):
						fmt.Fprint(conn, "230 ok\r\n")
					case strings.HasPrefix(cmd, "FEAT"):
						fmt.Fprint(conn, "211 end\r\n")
					case strings.HasPrefix(cmd, "OPTS"), strings.HasPrefix(cmd, "TYPE"):
						fmt.Fprint(conn, "200 ok\r\n")
					case strings.HasPrefix(cmd, "EPSV"):
						fmt.Fprintf(conn, "229 Entering Extended Passive Mode (|||%d|)\r\n", dport)
					case strings.HasPrefix(cmd, "LIST"):
						fmt.Fprint(conn, "150 here it comes\r\n")
					case strings.HasPrefix(cmd, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// F3: a silent DATA connection must be bounded by the context too (data connections get the deadline).
func TestFTPSilentDataConnectionHonoursContext(t *testing.T) {
	port := silentDataServer(t)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	res := make(chan error, 1)
	start := time.Now()
	go func() { _, err := c.ListDirectory(ctx, "/d"); res <- err }()
	select {
	case err := <-res:
		if err == nil {
			t.Fatal("expected an error from a silent data connection")
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Fatalf("returned after %v", d)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("HANG: silent data connection ignored the context")
	}
}

// F3: the operation turn is released when the session snapshot fails (not connected): a second call must not block.
func TestFTPTurnReleasedWhenNotConnected(t *testing.T) {
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: 1})
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_, err := c.GetFileInfo(ctx, "/x")
		cancel()
		if err == nil || err.Error() != "not connected" {
			t.Fatalf("call %d: want \"not connected\", got %v", i, err)
		}
	}
}

// ctxErrCounting reports Err()==nil for the first n calls and Canceled afterwards; Done never closes. It models "the context ends
// right after the operation succeeded, before release runs" deterministically (review F4).
type ctxErrCounting struct {
	context.Context
	n, calls int
	never    chan struct{}
}

func (c *ctxErrCounting) Err() error {
	c.calls++
	if c.calls > c.n {
		return context.Canceled
	}
	return nil
}
func (c *ctxErrCounting) Done() <-chan struct{} { return c.never }

// F4: a SUCCESSFUL operation keeps its session even if the context ends before release (a stream returned by ReadFile must not be broken).
func TestLiveRealFTP_SuccessKeepsSessionWhenContextEndsLate(t *testing.T) {
	c, sizes := realFTPFixture(t, 2)
	before, _, err := c.session()
	if err != nil {
		t.Fatal(err)
	}
	ctx := &ctxErrCounting{Context: context.Background(), n: 2, never: make(chan struct{})} // beginOp reads Err() twice
	rc, err := c.ReadFile(ctx, "/f01.bin")
	if err != nil {
		t.Fatal(err)
	}
	after, _, err := c.session()
	if err != nil || after != before {
		t.Fatalf("session was replaced after a successful ReadFile (err=%v)", err)
	}
	b := make([]byte, 4096)
	total := 0
	for {
		n, rerr := rc.Read(b)
		total += n
		if rerr != nil {
			break
		}
	}
	rc.Close()
	if int64(total) != sizes["f01.bin"] {
		t.Fatalf("stream read %d bytes, want %d", total, sizes["f01.bin"])
	}
}

// F1 (Connect half): a server that accepts TCP but never sends the greeting must not block Connect past its context.
func TestFTPConnectSilentBannerHonoursContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: ln.Addr().(*net.TCPAddr).Port, Username: "u", Password: "p"})
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	res := make(chan error, 1)
	start := time.Now()
	go func() { res <- c.Connect(ctx) }()
	select {
	case err := <-res:
		if err == nil || time.Since(start) > 5*time.Second {
			t.Fatalf("err=%v after %v", err, time.Since(start))
		}
	case <-time.After(20 * time.Second):
		t.Fatal("HANG: Connect ignored its context while waiting for the greeting")
	}
}

// F3: with NO context deadline, the idle bound alone must end an operation stuck on a silent data connection.
func TestFTPSilentDataConnectionEndsByIdleBound(t *testing.T) {
	old := ftpOpTimeout
	ftpOpTimeout = 600 * time.Millisecond
	defer func() { ftpOpTimeout = old }()
	port := silentDataServer(t)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	res := make(chan error, 1)
	go func() { _, err := c.ListDirectory(context.Background(), "/d"); res <- err }()
	select {
	case err := <-res:
		if err == nil {
			t.Fatal("expected an idle-timeout error")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("HANG: the idle bound did not end an operation stuck on a silent data connection")
	}
}

// F3: cancelling (no deadline) while stuck on a silent DATA connection must unblock it; repeated because connection-set order is random.
func TestFTPCancelUnblocksSilentDataConnection(t *testing.T) {
	for i := 0; i < 6; i++ {
		port := silentDataServer(t)
		c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
		if err := c.Connect(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		go func() { time.Sleep(300 * time.Millisecond); cancel() }()
		res := make(chan error, 1)
		go func() { _, err := c.ListDirectory(ctx, "/d"); res <- err }()
		select {
		case err := <-res:
			if err == nil {
				t.Fatal("expected an error")
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("round %d: HANG: cancel did not unblock a read on the silent data connection", i)
		}
		c.Disconnect(context.Background())
	}
}
