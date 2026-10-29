package filesystem

// UNIT-CLASS TEST DOUBLE (constitution §11.4.289 clause C): the responders here are hand-written FTP stand-ins. They prove the client's
// own locking (every operation takes one consistent (conn, base) snapshot under the lock) and its command ordering on the control
// connection. They are NOT evidence that FTP works against a real server (live_nas_test.go is that job).

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// opsLoop runs the operations that read c.client/c.base until stop is closed; a panic (nil *ServerConn dereference) is reported as an error.
func opsLoop(t *testing.T, c *FTPClient, stop <-chan struct{}, wg *sync.WaitGroup, passes *int32) {
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
		if rc, err := c.ReadFile(ctx, "/f.bin"); err == nil {
			_, _ = io.Copy(io.Discard, rc)
			_ = rc.Close()
		}
		_ = c.WriteFile(ctx, "/d/w.bin", strings.NewReader("data"))
		_ = c.CreateDirectory(ctx, "/d/sub")
		_ = c.DeleteDirectory(ctx, "/d/sub")
		_ = c.DeleteFile(ctx, "/d/w.bin")
		_ = c.CopyFile(ctx, "/f.bin", "/d/copy.bin")
		atomic.AddInt32(passes, 1)
	}
}

// Session-field cycling against concurrent operations: no data race on client/base/connected, no nil dereference.
//
// Scope note (honest): jlaffaye/ftp's *ServerConn is itself not goroutine-safe, so a REAL Disconnect (conn.Quit) overlapping an operation
// that is mid-command on the same connection is a library-level hazard this client cannot remove. The cycler below therefore performs
// exactly the field writes Disconnect/Connect/reconnectIf perform (under c.mu: client=nil, connected=false, gen++ and the re-install of
// client/base/connected) but never Quit()s the connection the operation is using, so the test isolates the client's own locking.
func TestFTPSession_FieldCyclingVsOperations_NoRaceNoPanic(t *testing.T) {
	s := startStrictFTP(t)
	c := strictClient(t, s)
	c.mu.Lock()
	conn, base := c.client, c.base
	c.mu.Unlock()
	if conn == nil {
		t.Fatalf("no connection after Connect")
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	var passes int32
	go opsLoop(t, c, stop, &wg, &passes)
	const cycles = 3000
	// keep cycling until the operations have completed many full passes (150), so every operation is exercised against the writers
	for i := 0; i < cycles || atomic.LoadInt32(&passes) < 150; i++ {
		c.mu.Lock() // what Disconnect does, minus the Quit
		c.client = nil
		c.connected = false
		c.gen++
		c.mu.Unlock()
		c.mu.Lock() // what Connect/reconnectIf do on success
		c.client, c.base, c.connected = conn, base, true
		c.mu.Unlock()
		runtime.Gosched() // let the operation goroutine run between cycles so the writes land inside its unlocked windows
	}
	close(stop)
	wg.Wait()
	if err := c.Disconnect(context.Background()); err != nil {
		t.Logf("final Disconnect: %v", err)
	}
	if n := atomic.LoadInt32(&s.accepted); n < 2 {
		t.Fatalf("control connections = %d, want >= 2 (the initial Connect plus CopyFile's second connection): the loop did not reach CopyFile", n)
	}
	if c.gen < cycles || atomic.LoadInt32(&passes) < 150 {
		t.Fatalf("gen = %d (want >= %d), passes = %d (want >= 150)", c.gen, cycles, atomic.LoadInt32(&passes))
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

// strictFTP is a unit-class FTP responder with a per-CONTROL-CONNECTION rule: while a RETR data connection is still open on a control
// connection, STOR/MKD on that same control connection are rejected with 425 (some real servers behave like this). A second control
// connection is unaffected.
type strictFTP struct {
	ln net.Listener
	// configuration (set before the first connection)
	retrSize int64  // bytes served by RETR; 0 = the 10-byte "hello-copy"
	retrEnd  string // reply after the RETR data connection is closed: "226" (default), "426", "451", or "drop" (control connection dies)
	storEnd  string // reply after STOR data: "226" (default) or "451"
	maxConns int32  // > 0: control connections beyond this many are refused with 421
	// observations
	accepted int32
	quits    int32
	rejected int32
	retrs    int32 // RETR commands received
	storedN  int64 // bytes received by the last STOR
	storing  int64 // bytes received so far by any STOR (progress)
	mu       sync.Mutex
	stored   []byte // content of the last STOR when <= 1 MiB
	deleted  []string
	storedCh chan struct{}
}

func startStrictFTP(t *testing.T, opts ...func(*strictFTP)) *strictFTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &strictFTP{ln: ln, storedCh: make(chan struct{}, 8)}
	for _, o := range opts {
		o(s) // configuration is applied before any goroutine of the responder starts
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			n := atomic.AddInt32(&s.accepted, 1)
			if s.maxConns > 0 && n > s.maxConns {
				fmt.Fprint(c, "421 too many sessions\r\n")
				c.Close()
				continue
			}
			go s.serve(c)
		}
	}()
	return s
}

func (s *strictFTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func strictClient(t *testing.T, s *strictFTP) *FTPClient {
	t.Helper()
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: s.port(), Username: "u", Password: "p"})
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return c
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
	var busy atomic.Bool
	say("220 strict ready")
	var dl net.Listener
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		fields := strings.Fields(strings.TrimSpace(line) + " x")
		cmd := strings.ToUpper(fields[0])
		arg := ""
		if len(fields) > 2 {
			arg = fields[1]
		}
		switch cmd {
		case "USER":
			say("331 need password")
		case "PASS":
			say("230 ok")
		case "TYPE":
			say("200 type")
		case "PWD":
			say(`257 "/" is cwd`)
		case "SIZE":
			say("213 10")
		case "EPSV":
			dl, _ = net.Listen("tcp", "127.0.0.1:0")
			say("229 Entering Extended Passive Mode (|||%d|)", dl.Addr().(*net.TCPAddr).Port)
		case "LIST":
			say("150 listing")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			fmt.Fprint(dc, "-rw-r--r-- 1 u g 10 Jan 01 2020 f.bin\r\n")
			dc.Close()
			say("226 listed")
		case "RETR":
			atomic.AddInt32(&s.retrs, 1)
			say("150 opening")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			busy.Store(true)
			go func() { // serve the data, then wait for the client to close its side
				if s.retrSize > 0 {
					buf := make([]byte, 32*1024)
					for left := s.retrSize; left > 0; {
						n := int64(len(buf))
						if left < n {
							n = left
						}
						if _, err := dc.Write(buf[:n]); err != nil {
							break
						}
						left -= n
					}
				} else {
					dc.Write([]byte("hello-copy"))
				}
				dc.(*net.TCPConn).CloseWrite()
				io.Copy(io.Discard, dc)
				dc.Close()
				busy.Store(false)
				switch s.retrEnd {
				case "426":
					say("426 transfer aborted")
				case "451":
					say("451 local error")
				case "drop":
					c.Close()
				default:
					say("226 transfer complete")
				}
			}()
		case "MKD":
			if busy.Load() {
				atomic.AddInt32(&s.rejected, 1)
				say("425 data connection open")
			} else {
				say("257 created")
			}
		case "DELE":
			s.mu.Lock()
			s.deleted = append(s.deleted, arg)
			s.mu.Unlock()
			say("250 deleted")
		case "RMD":
			say("250 removed")
		case "STOR":
			if busy.Load() {
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
			var n int64
			buf := make([]byte, 32*1024)
			for {
				m, e := dc.Read(buf)
				if m > 0 {
					n += int64(m)
					atomic.AddInt64(&s.storing, int64(m))
					if n <= 1<<20 {
						b.Write(buf[:m])
					}
				}
				if e != nil {
					break
				}
			}
			dc.Close()
			s.mu.Lock()
			atomic.StoreInt64(&s.storedN, n)
			s.stored = nil
			if n <= 1<<20 {
				s.stored = b.Bytes()
			}
			s.mu.Unlock()
			if s.storEnd == "451" {
				say("451 disk full")
			} else {
				say("226 stored")
			}
			select {
			case s.storedCh <- struct{}{}:
			default:
			}
		case "QUIT":
			atomic.AddInt32(&s.quits, 1)
			say("221 bye")
			return
		default:
			say("502 not implemented")
		}
	}
}

func (s *strictFTP) deletedList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.deleted...)
}

// CopyFile must not issue MKD/STOR on the control connection that has the RETR data connection open, and must not leak its second control
// connection.
func TestFTPCopyFile_NoControlCommandsWhileRetrDataConnectionOpen(t *testing.T) {
	s := startStrictFTP(t)
	c := strictClient(t, s)
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
	// the second control connection CopyFile used must have been ended with QUIT (no leaked session)
	waitFor(t, "CopyFile's second control connection to be QUIT", func() bool { return atomic.LoadInt32(&s.quits) >= 1 })
}

// F1: a large copy must stream; the heap must not grow with the file size (io.ReadAll of the source would).
func TestFTPCopyFile_LargeFileStreamsWithoutBufferingInMemory(t *testing.T) {
	const size = 64 << 20
	s := startStrictFTP(t, func(s *strictFTP) { s.retrSize = size })
	c := strictClient(t, s)
	runtime.GC()
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	base := ms.HeapAlloc
	var peak uint64
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			if m.HeapAlloc > atomic.LoadUint64(&peak) {
				atomic.StoreUint64(&peak, m.HeapAlloc)
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()
	err := c.CopyFile(context.Background(), "/big.bin", "/dir/big.copy")
	close(stop)
	<-done
	if err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if n := atomic.LoadInt64(&s.storedN); n != size {
		t.Fatalf("server stored %d bytes, want %d", n, size)
	}
	if grew := atomic.LoadUint64(&peak) - base; atomic.LoadUint64(&peak) > base && grew > 24<<20 {
		t.Fatalf("heap grew by %d MiB while copying %d MiB: the copy is buffered in memory", grew>>20, size>>20)
	}
}

// F3: a failed RETR completion (426/451) must make CopyFile fail, and the unverified destination must be removed (F5).
func TestFTPCopyFile_RetrCompletionFailureIsReturned(t *testing.T) {
	for _, end := range []string{"426", "451"} {
		t.Run(end, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) { s.retrEnd = end })
			c := strictClient(t, s)
			if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
				t.Fatalf("CopyFile swallowed a %s reply to the RETR", end)
			}
			if d := s.deletedList(); len(d) != 1 || d[0] != "/dir/dst.bin" {
				t.Fatalf("DELE seen = %v, want exactly [/dir/dst.bin] (the unverified destination removed)", d)
			}
		})
	}
}

// F4: when the RETR dies together with the control connection, CopyFile fails AND the client is left with a working session.
func TestFTPCopyFile_DeadControlConnectionIsReplaced(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.retrEnd = "drop" })
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
		t.Fatalf("CopyFile succeeded although the source connection died before the transfer was confirmed")
	}
	if !c.IsConnected() {
		t.Fatalf("client not connected after the failed copy")
	}
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("session left dead after the failed copy: %v", err)
	}
}

// F5: a failed STOR leaves no partial destination.
func TestFTPCopyFile_FailedStorRemovesPartialDestination(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.storEnd = "451" })
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
		t.Fatalf("CopyFile succeeded although STOR was answered 451")
	}
	if d := s.deletedList(); len(d) != 1 || d[0] != "/dir/dst.bin" {
		t.Fatalf("DELE seen = %v, want exactly [/dir/dst.bin]", d)
	}
}

// F5: the copy honours a context cancelled before it starts (no fallback, nothing stored) ...
func TestFTPCopyFile_PreCancelledContextReturnsCanceled(t *testing.T) {
	s := startStrictFTP(t)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin"); !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile with a cancelled context = %v, want context.Canceled", err)
	}
	if n, r := atomic.LoadInt64(&s.storing), atomic.LoadInt32(&s.retrs); n != 0 || r != 0 {
		t.Fatalf("server saw %d RETR and %d STOR bytes for a cancelled copy, want none", r, n)
	}
}

// ... and one cancelled while the data is flowing: the copy stops, reports the cancellation and removes the partial destination.
func TestFTPCopyFile_CancelDuringTransferStopsAndRemovesPartial(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.retrSize = 256 << 20 })
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		for atomic.LoadInt64(&s.storing) == 0 {
			time.Sleep(time.Millisecond)
		}
		cancel()
	}()
	err := c.CopyFile(ctx, "/big.bin", "/dir/big.copy")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile cancelled mid-transfer = %v, want context.Canceled", err)
	}
	if n := atomic.LoadInt64(&s.storing); n <= 0 || n >= 256<<20 {
		t.Fatalf("server received %d bytes, want a partial transfer (0 < n < 256 MiB)", n)
	}
	if d := s.deletedList(); len(d) != 1 || d[0] != "/dir/big.copy" {
		t.Fatalf("DELE seen = %v, want exactly [/dir/big.copy]", d)
	}
}

// A server that allows one session only: the copy falls back to a temp file, still works, and removes the temp file.
func TestFTPCopyFile_SingleSessionServerFallsBackToTempFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	s := startStrictFTP(t, func(s *strictFTP) { s.maxConns = 1 })
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	select {
	case <-s.storedCh:
	case <-time.After(5 * time.Second):
		t.Fatalf("server never completed a STOR")
	}
	s.mu.Lock()
	got := string(s.stored)
	s.mu.Unlock()
	if got != "hello-copy" {
		t.Fatalf("stored content = %q, want %q", got, "hello-copy")
	}
	if n := atomic.LoadInt32(&s.rejected); n != 0 {
		t.Fatalf("rejected %d commands", n)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

// F6: Connect on a client that already holds a connection ends the previous one with QUIT instead of leaking it.
func TestFTPConnect_ReplacesAndQuitsPreviousConnection(t *testing.T) {
	s := startStrictFTP(t)
	c := strictClient(t, s)
	if err := c.Connect(context.Background()); err != nil {
		t.Fatalf("second Connect: %v", err)
	}
	if n := atomic.LoadInt32(&s.accepted); n != 2 {
		t.Fatalf("control connections = %d, want 2", n)
	}
	waitFor(t, "the first connection to be QUIT", func() bool { return atomic.LoadInt32(&s.quits) == 1 })
	assertStays(t, "QUIT count", &s.quits, 1)
	if err := c.TestConnection(context.Background()); err != nil {
		t.Fatalf("new session unusable: %v", err)
	}
}

// ftpResolve resolves p against the client's current base (what an operation does after session()).
func ftpResolve(c *FTPClient, p string) string { return c.resolvePath(c.base, p) }

// Defensive: a corrupt state (connected flag set, no connection) must answer "not connected", never dereference nil.
func TestFTPSession_ConnectedFlagWithNilClientIsNotConnected(t *testing.T) {
	c := NewFTPClient(&FTPConfig{Host: "127.0.0.1", Port: 1})
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()
	if _, _, err := c.session(); err == nil {
		t.Fatalf("session() returned a nil connection without an error")
	}
	if err := c.TestConnection(context.Background()); err == nil {
		t.Fatalf("TestConnection with a nil connection returned nil error")
	}
}
