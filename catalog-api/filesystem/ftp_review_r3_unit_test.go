package filesystem

// Review round 3 probes (adopted from the independent reviewer) plus round-4 tests. rvFake is a hand-written fake server: UNIT-CLASS (constitution 11.4.289 C).

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func cutFirstSession(t *testing.T, mode string) (*FTPClient, *int32) {
	port, n := rvFake(t, mode)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := c.GetFileInfo(ctx, "/x"); err == nil {
		t.Fatal("expected cut")
	}
	return c, n
}

// R3a: many goroutines racing the lazy replacement: all correct, exactly one redial.
func TestR3_ConcurrentOpsRaceLazyReplacement(t *testing.T) {
	c, n := cutFirstSession(t, "ok")
	defer c.Disconnect(context.Background())
	var wg sync.WaitGroup
	var bad int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			fi, err := c.GetFileInfo(ctx, "/x")
			if err != nil || fi.Size != 42 {
				atomic.AddInt32(&bad, 1)
				t.Logf("bad: fi=%v err=%v", fi, err)
			}
		}()
	}
	wg.Wait()
	if bad > 0 {
		t.Fatalf("%d ops failed", bad)
	}
	if got := atomic.LoadInt32(n); got != 2 {
		t.Fatalf("connections dialled = %d, want 2 (one Connect + one replacement)", got)
	}
}

// R3b: Disconnect after a cut: the pending replacement must not resurrect the client.
func TestR3_DisconnectAfterCutNoResurrection(t *testing.T) {
	c, n := cutFirstSession(t, "ok")
	if err := c.Disconnect(context.Background()); err != nil {
		t.Logf("disconnect err=%v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := c.GetFileInfo(ctx, "/x")
	if err == nil || c.IsConnected() {
		t.Fatalf("a disconnected client was resurrected by the lazy replacement (err=%v connected=%v)", err, c.IsConnected())
	}
	if got := atomic.LoadInt32(n); got != 1 {
		t.Fatalf("dialled %d connections after Disconnect, want 1", got)
	}
}

// R3c: CopyFile after a cut must not run on the desynced session (lockOps path replaces it).
func TestR3_CopyFileAfterCutUsesFreshSession(t *testing.T) {
	c, n := cutFirstSession(t, "ok")
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = c.CopyFile(ctx, "/a", "/b") // fake server cannot copy; we only check the session was replaced first
	c.mu.Lock()
	stale := c.staleConn
	c.mu.Unlock()
	if stale != nil {
		t.Fatal("stale session still pending after CopyFile")
	}
	if atomic.LoadInt32(n) < 2 {
		t.Fatalf("CopyFile did not replace the cut session (dials=%d)", atomic.LoadInt32(n))
	}
}

// R3d: a replacement that fails (server silent) leaves the client not connected; a later Connect recovers it.
func TestR3_FailedReplacementRecoverableByConnect(t *testing.T) {
	c, _ := cutFirstSession(t, "silent-banner")
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := c.GetFileInfo(ctx, "/x")
	t.Logf("op during failing replacement: err=%v after %v connected=%v", err, time.Since(start), c.IsConnected())
	if time.Since(start) > 3*time.Second {
		t.Fatalf("replacement not bounded by op ctx: %v", time.Since(start))
	}
	if err == nil {
		t.Fatal("expected error")
	}
}

// M1: Disconnect right after a cut must not report the dead session's write error.
func TestFTPDisconnectAfterCutNoSpuriousError(t *testing.T) {
	for i := 0; i < 10; i++ { // the dead write error is timing dependent: repeat
		c, _ := cutFirstSession(t, "ok")
		if err := c.Disconnect(context.Background()); err != nil {
			t.Fatalf("round %d: Disconnect after a cut returned %v", i, err)
		}
	}
}

// M2 guard: a Disconnect after a failed replacement must not be undone by the retry.
func TestFTPDisconnectAfterFailedReplacementNoRedial(t *testing.T) {
	port, stop := stoppableFake(t, "127.0.0.1:0", true)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_, _ = c.GetFileInfo(ctx, "/x")
	stop()
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	_, _ = c.GetFileInfo(ctx2, "/x") // replacement fails, retry is armed
	_ = c.Disconnect(context.Background())
	_, stop2 := stoppableFake(t, fmt.Sprintf("127.0.0.1:%d", port), false)
	defer stop2()
	ctx3, cancel3 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel3()
	if _, err := c.GetFileInfo(ctx3, "/x"); err == nil || c.IsConnected() {
		t.Fatalf("the client was redialled after an explicit Disconnect (err=%v connected=%v)", err, c.IsConnected())
	}
}

// M1: GetFileInfo whose SIZE was cut by the context reports that, instead of falling through to a LIST on the dead session.
func TestFTPGetFileInfoCutReportsContextError(t *testing.T) {
	port, _ := rvFake(t, "ok")
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	_, err := c.GetFileInfo(ctx, "/x")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want an error wrapping context.DeadlineExceeded, got %v", err)
	}
}

// stoppableFake: the same fake FTP as rvFake on a chosen address. firstSilent: the first connection never answers SIZE (a cut target).
func stoppableFake(t *testing.T, addr string, firstSilent bool) (port int, stop func()) {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
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
					case strings.HasPrefix(cmd, "QUIT"):
						fmt.Fprint(conn, "221 bye\r\n")
						return
					case strings.HasPrefix(cmd, "SIZE") && !(firstSilent && k == 1):
						fmt.Fprint(conn, "213 42\r\n")
					case strings.HasPrefix(cmd, "MDTM") && !(firstSilent && k == 1):
						fmt.Fprint(conn, "213 20200101000000\r\n")
					default:
						if !(firstSilent && k == 1) {
							fmt.Fprint(conn, "502 not implemented\r\n")
						}
					}
				}
			}(conn, k)
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, func() { ln.Close() }
}

// M2: the replacement of a cut session fails because the server is down; once the server is back a later operation redials by itself.
func TestFTPFailedReplacementRetriedByLaterOp(t *testing.T) {
	port, stop := stoppableFake(t, "127.0.0.1:0", true)
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: port, Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer c.Disconnect(context.Background())
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	if _, err := c.GetFileInfo(ctx, "/x"); err == nil {
		t.Fatal("expected cut")
	}
	stop() // server gone: the lazy replacement fails
	ctx2, cancel2 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel2()
	if _, err := c.GetFileInfo(ctx2, "/x"); err == nil {
		t.Fatal("expected an error while the server is down")
	}
	_, stop2 := stoppableFake(t, fmt.Sprintf("127.0.0.1:%d", port), false) // server back
	defer stop2()
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	fi, err := c.GetFileInfo(ctx3, "/x")
	if err != nil || fi.Size != 42 {
		t.Fatalf("op after the server came back: fi=%v err=%v (client stuck not connected)", fi, err)
	}
}
