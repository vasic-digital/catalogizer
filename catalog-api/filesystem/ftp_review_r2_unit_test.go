package filesystem

// Review round 2 probes (adopted from the independent reviewer). r2Fake is a hand-written fake server: UNIT-CLASS (constitution 11.4.289 C).

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// conn #1 behaves (login ok, SIZE 213 42, everything else 502); conns #2+ accept and never greet.
func r2Fake(t *testing.T) int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	var n int32
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			k := atomic.AddInt32(&n, 1)
			go func(conn net.Conn, k int32) {
				defer conn.Close()
				if k > 1 {
					time.Sleep(40 * time.Second)
					return
				}
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
					case strings.HasPrefix(cmd, "TYPE"), strings.HasPrefix(cmd, "OPTS"):
						fmt.Fprint(conn, "200 ok\r\n")
					case strings.HasPrefix(cmd, "FEAT"):
						fmt.Fprint(conn, "211 end\r\n")
					case strings.HasPrefix(cmd, "SIZE"):
						fmt.Fprint(conn, "213 42\r\n")
					case strings.HasPrefix(cmd, "QUIT"):
						return
					default:
						fmt.Fprint(conn, "502 no\r\n")
					}
				}
			}(conn, k)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// P1: CopyFile opens a second session; if that server never greets, CopyFile must still honour its ctx and must not hold the turn.
func TestR2_CopyFileSecondDialHonoursCtx(t *testing.T) {
	port := r2Fake(t)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	start := time.Now()
	go func() { done <- c.CopyFile(ctx, "/a", "/b") }()
	select {
	case err := <-done:
		t.Logf("CopyFile returned after %v err=%v", time.Since(start), err)
		if time.Since(start) > 5*time.Second {
			t.Fatalf("CopyFile took %v with a 700ms ctx", time.Since(start))
		}
	case <-time.After(20 * time.Second):
		// is the turn held? a parallel stat with its own ctx
		ctx2, c2 := context.WithTimeout(context.Background(), time.Second)
		defer c2()
		_, err := c.GetFileInfo(ctx2, "/x")
		t.Fatalf("HANG: CopyFile with a 700ms ctx did not return in 20s; concurrent GetFileInfo err=%v", err)
	}
}

// P2: a cancel landing after the I/O but before release leaves grp.cancelled sticky; an unarmed dial (CopyFile's Retr) then gets an
// already-expired deadline.
func TestR2_StickyCancelledFlag(t *testing.T) {
	g := &ftpConnGroup{}
	g.arm(context.Background(), time.Minute)
	g.cancel()  // watcher fired late
	g.disarm()  // release after a successful op
	g.mu.Lock() // what an unarmed data dial (lockOps path) would get
	dl := g.deadlineLocked()
	g.mu.Unlock()
	if !dl.IsZero() {
		t.Fatalf("after disarm an unarmed dial still gets deadline %v (cancelled flag sticky)", dl)
	}
}

// N3: after an idle-bound timeout under a context WITHOUT deadline the session must be replaced and the next operation must work.
func TestFTPSessionReplacedAfterIdleTimeout(t *testing.T) {
	old := ftpOpTimeout
	ftpOpTimeout = 500 * time.Millisecond
	defer func() { ftpOpTimeout = old }()
	port, _ := rvFake(t, "ok") // conn #1 never answers SIZE; later connections do
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	if _, err := c.GetFileInfo(context.Background(), "/x"); err == nil {
		t.Fatal("expected the idle bound to end the silent operation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	fi, err := c.GetFileInfo(ctx, "/x")
	if err != nil || fi.Size != 42 {
		t.Fatalf("next op after an idle-timeout cut: fi=%v err=%v (session not replaced)", fi, err)
	}
}

// N5: a RETR cut short by its context must replace the session too (ReadFile reports its error to release).
func TestFTPReadFileCutReplacesSession(t *testing.T) {
	port, _ := rvFake(t, "ok") // conn #1 never answers EPSV/PASV
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if _, err := c.ReadFile(ctx, "/x"); err == nil {
		t.Fatal("expected an error from a RETR that never gets a reply")
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	fi, err := c.GetFileInfo(ctx2, "/x")
	if err != nil || fi.Size != 42 {
		t.Fatalf("next op after a cut RETR: fi=%v err=%v (session not replaced)", fi, err)
	}
}

// N4: replacing a cut session is lazy: the cut operation returns at once, the redial happens (bounded by the NEXT op's ctx) later.
func TestFTPCutOperationReturnsWithoutWaitingForRedial(t *testing.T) {
	port, _ := rvFake(t, "silent-banner")
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, _ = c.GetFileInfo(ctx, "/x")
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("cut operation returned after %v, expected about its 500ms context", d)
	}
}

// N2: after Connect's dial deadline policy ends it must not linger: an unarmed operation (CopyFile) long after Connect still works.
func TestLiveRealFTP_CopyFileLongAfterConnect(t *testing.T) {
	old := ftpDialTimeout
	ftpDialTimeout = 400 * time.Millisecond
	defer func() { ftpDialTimeout = old }()
	c, sizes := realFTPFixture(t, 2)
	time.Sleep(900 * time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := c.CopyFile(ctx, "/f01.bin", "/copy.bin"); err != nil {
		t.Fatalf("CopyFile after the dial bound elapsed: %v", err)
	}
	fi, err := c.GetFileInfo(ctx, "/copy.bin")
	if err != nil || fi.Size != sizes["f01.bin"] {
		t.Fatalf("copy: fi=%v err=%v", fi, err)
	}
}
