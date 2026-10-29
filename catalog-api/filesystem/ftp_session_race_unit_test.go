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
	"net/textproto"
	"os"
	pathpkg "path"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
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
	opsDone := make(chan struct{})
	go func() { opsLoop(t, c, stop, &wg, &passes); close(opsDone) }()
	const cycles = 3000
	deadline := time.Now().Add(90 * time.Second)
	// keep cycling until the operations have completed many full passes (150), so every operation is exercised against the writers.
	// The loop ends early (with a failure) if the operations goroutine exits (a panic) or the deadline passes (a hung operation).
	for i := 0; i < cycles || atomic.LoadInt32(&passes) < 150; i++ {
		select {
		case <-opsDone:
			t.Fatalf("operations goroutine exited early after %d passes (panic or error reported above)", atomic.LoadInt32(&passes))
		default:
		}
		if time.Now().After(deadline) {
			close(stop)
			select {
			case <-opsDone:
			case <-time.After(5 * time.Second):
			}
			t.Fatalf("deadline: only %d passes after 90s: an operation is hung", atomic.LoadInt32(&passes))
		}
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
	select {
	case <-opsDone:
	case <-time.After(30 * time.Second):
		t.Fatalf("operations goroutine did not stop within 30s of the stop signal: an operation is hung")
	}
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
	retrSize     int64  // bytes served by RETR; 0 = the 10-byte "hello-copy"
	retrEnd      string // reply after the RETR data connection is closed: "226" (default), "426", "451", or "drop" (control connection dies)
	storEnd      string // reply after STOR data: "226" (default) or "451"
	maxConns     int32  // > 0: control connections beyond this many are refused with 421
	storRefuse   string // e.g. "553": STOR is refused before any data flows (proftpd AllowOverwrite-off shape)
	noRenameOver bool   // RNTO onto an existing file is refused with 553
	retrStall    bool   // RETR sends 32 KiB and then stops (until the client closes the data connection)
	storStall    bool   // STOR accepts the data connection and never reads it (the client's writes block)
	onStor       func() // called when a STOR is accepted (before data is read)
	// observations
	accepted      int32
	live          int32 // control connections currently open (only counted when maxConns > 0)
	quits         int32
	rejected      int32
	retrs         int32 // RETR commands received
	storedN       int64 // bytes received by the last STOR
	storing       int64 // bytes received so far by any STOR (progress)
	mu            sync.Mutex
	stored        []byte // content of the last STOR when <= 1 MiB
	deleted       []string
	storedCh      chan struct{}
	files         map[string][]byte // the responder's tiny file store (<= 1 MiB content kept; bigger ones keep a marker)
	storNames     []string
	renames       [][2]string
	release       chan struct{} // closed at test cleanup: unblocks the stalled handlers
	noAbortReply  bool
	maxNameLen    int
	deleRefuse    bool
	deleCode      int           // > 0: a refused DELE answers with this code (e.g. 450) instead of 550
	deleAttempts  int32         // every DELE seen, refused or not
	retrEndDelay  time.Duration // pause before the final RETR reply (data already complete)
	deleRefuseMsg string        // text of the 550 a refused DELE answers with (default "permission denied")
	rntoStall     bool          // RNTO is accepted and never answered (until test cleanup)
	rntoDelay     time.Duration // RNTO is answered after this delay
	rntoDrop      bool          // RNTO makes the server drop the control connection without a reply
	listing       string        // LIST payload (default: a single f.bin line)
	refuseAfter   int32         // > 0: control connections beyond the Nth ever accepted are refused with 421 (no new session can be opened later)
	// storCreateEarly: a real server creates the file when the STOR is opened. startStrictFTP defaults it to true, so the responder records
	// the (partial) file at STOR start; a cleanup DELE that arrives later therefore always finds it (and a late end of the data stream
	// does not resurrect a deleted file). Set false to model a server that only shows the file after the transfer completed.
	storCreateEarly bool
}

func (s *strictFTP) seed(name, content string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files == nil {
		s.files = map[string][]byte{}
	}
	s.files[name] = []byte(content)
}

func (s *strictFTP) file(name string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.files[name]
	return string(b), ok
}

// tempNames lists the *.ftpcopy-*.part files the responder currently holds.
func (s *strictFTP) tempNames() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for n := range s.files {
		if strings.Contains(n, ".ftpcopy-") {
			out = append(out, n)
		}
	}
	return out
}

func startStrictFTP(t *testing.T, opts ...func(*strictFTP)) *strictFTP {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &strictFTP{ln: ln, storedCh: make(chan struct{}, 8), files: map[string][]byte{}, release: make(chan struct{}), storCreateEarly: true}
	for _, o := range opts {
		o(s) // configuration is applied before any goroutine of the responder starts
	}
	t.Cleanup(func() { close(s.release); ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			nAcc := atomic.AddInt32(&s.accepted, 1)
			if s.refuseAfter > 0 && nAcc > s.refuseAfter {
				fmt.Fprint(c, "421 no more sessions\r\n")
				c.Close()
				continue
			}
			over := func() bool { return atomic.AddInt32(&s.live, 1) > s.maxConns }
			if s.maxConns > 0 && over() { // a CONCURRENT-session cap, like a real server's
				// a real server needs a moment to notice a session the client just closed; wait for that before refusing
				for i := 0; i < 50 && atomic.LoadInt32(&s.live) > s.maxConns; i++ {
					time.Sleep(10 * time.Millisecond)
				}
				if atomic.LoadInt32(&s.live) <= s.maxConns {
					goto admit
				}
				atomic.AddInt32(&s.live, -1)
				fmt.Fprint(c, "421 too many sessions\r\n")
				c.Close()
				continue
			}
		admit:
			go func() {
				defer func() {
					if s.maxConns > 0 {
						atomic.AddInt32(&s.live, -1)
					}
				}()
				s.serve(c)
			}()
		}
	}()
	return s
}

func (s *strictFTP) port() int { return s.ln.Addr().(*net.TCPAddr).Port }

func strictClient(t *testing.T, s *strictFTP, mod ...func(*FTPConfig)) *FTPClient {
	t.Helper()
	cfg := &FTPConfig{Host: "127.0.0.1", Port: s.port(), Username: "u", Password: "p"}
	for _, m := range mod {
		m(cfg)
	}
	c := NewFTPClient(cfg)
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
	var rnfr string
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
			if s.listing != "" {
				fmt.Fprint(dc, s.listing)
			} else {
				fmt.Fprint(dc, "-rw-r--r-- 1 u g 10 Jan 01 2020 f.bin\r\n")
			}
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
				if s.retrStall {
					dc.Write(make([]byte, 32*1024)) // then nothing more: the client's Read blocks until it closes the connection
					io.Copy(io.Discard, dc)
					dc.Close()
					busy.Store(false)
					if s.noAbortReply {
						<-s.release
						return
					}
					say("426 aborted by client")
					return
				}
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
				if s.retrEndDelay > 0 {
					time.Sleep(s.retrEndDelay)
				}
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
			atomic.AddInt32(&s.deleAttempts, 1)
			if s.deleRefuse {
				if s.deleCode > 0 {
					say(fmt.Sprintf("%d file busy", s.deleCode))
				} else if s.deleRefuseMsg != "" {
					say("550 " + s.deleRefuseMsg)
				} else {
					say("550 permission denied")
				}
				continue
			}
			s.mu.Lock()
			s.deleted = append(s.deleted, arg)
			_, existed := s.files[arg]
			delete(s.files, arg)
			s.mu.Unlock()
			if existed {
				say("250 deleted")
			} else {
				say("550 no such file")
			}
		case "RNFR":
			s.mu.Lock()
			_, ok := s.files[arg]
			s.mu.Unlock()
			if !ok {
				say("550 no such file")
				continue
			}
			rnfr = arg
			say("350 ready for RNTO")
		case "RNTO":
			if s.rntoStall {
				<-s.release
				return
			}
			if s.rntoDrop {
				return
			}
			if s.rntoDelay > 0 {
				time.Sleep(s.rntoDelay)
			}
			s.mu.Lock()
			from, content := rnfr, s.files[rnfr]
			_, exists := s.files[arg]
			ok := true
			if s.noRenameOver && exists {
				ok = false
			} else {
				delete(s.files, from)
				s.files[arg] = content
				s.renames = append(s.renames, [2]string{from, arg})
			}
			s.mu.Unlock()
			if ok {
				say("250 renamed")
			} else {
				say("553 rename onto existing file not allowed")
			}
		case "RMD":
			say("250 removed")
		case "STOR":
			if busy.Load() {
				atomic.AddInt32(&s.rejected, 1)
				say("425 data connection open")
				continue
			}
			if s.maxNameLen > 0 && len(arg[strings.LastIndex(arg, "/")+1:]) > s.maxNameLen {
				say("553 file name too long")
				continue
			}
			s.mu.Lock()
			s.storNames = append(s.storNames, arg)
			s.mu.Unlock()
			if s.storRefuse != "" {
				dl.Close()
				say("%s STOR refused", s.storRefuse)
				continue
			}
			if s.storCreateEarly {
				s.mu.Lock()
				s.files[arg] = []byte("<partial>")
				s.mu.Unlock()
			}
			if s.onStor != nil {
				s.onStor()
			}
			say("150 ok")
			dc, err := dl.Accept()
			dl.Close()
			if err != nil {
				return
			}
			if s.storStall {
				<-s.release // never read: the client's writes fill the socket buffers and block
				dc.Close()
				continue
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
				s.mu.Lock()
				_, still := s.files[arg]
				if still || !s.storCreateEarly { // a file the client deleted meanwhile stays deleted (unlinked inode)
					if n <= 1<<20 {
						s.files[arg] = append([]byte(nil), b.Bytes()...)
					} else {
						s.files[arg] = []byte("<big>")
					}
				}
				s.mu.Unlock()
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

const oldDst = "OLD-DESTINATION-CONTENT"

// assertDstUntouched: the pre-existing destination is byte-identical, was never DELEd, and no temp file is left on the responder.
func assertDstUntouched(t *testing.T, s *strictFTP, dst string) {
	t.Helper()
	if got, ok := s.file(dst); !ok || got != oldDst {
		t.Fatalf("pre-existing destination %s = %q (exists=%v), want it byte-identical %q", dst, got, ok, oldDst)
	}
	for _, d := range s.deletedList() {
		if d == dst {
			t.Fatalf("the pre-existing destination %s was DELEd (data loss); DELE seen: %v", dst, s.deletedList())
		}
	}
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp files left on the server: %v", left)
	}
}

// onlyTempsDeleted: every DELE the copy issued targeted its own temp file.
func onlyTempsDeleted(t *testing.T, s *strictFTP, wantAtLeast int) {
	t.Helper()
	d := s.deletedList()
	if len(d) < wantAtLeast {
		t.Fatalf("DELE seen = %v, want at least %d (the copy's own temp removed)", d, wantAtLeast)
	}
	for _, n := range d {
		if !strings.HasPrefix(n, "/dir/.dst.bin.ftpcopy-") || !strings.HasSuffix(n, ".part") {
			t.Fatalf("DELE of %q: a copy may delete only its own temp file; DELE seen: %v", n, d)
		}
	}
}

// CopyFile must not issue MKD/STOR on the control connection that has the RETR data connection open, must not leak its second control
// connection, and must store under a temp name that is renamed over the destination only at the end.
func TestFTPCopyFile_NoControlCommandsWhileRetrDataConnectionOpen(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if n := atomic.LoadInt32(&s.rejected); n != 0 {
		t.Fatalf("server rejected %d commands issued while the RETR data connection was open", n)
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("destination = %q, want %q", got, "hello-copy")
	}
	s.mu.Lock()
	names, renames := append([]string(nil), s.storNames...), append([][2]string(nil), s.renames...)
	s.mu.Unlock()
	if len(names) != 1 || !strings.HasPrefix(names[0], "/dir/.dst.bin.ftpcopy-") {
		t.Fatalf("STOR targets = %v, want exactly one temp name beside the destination", names)
	}
	if len(renames) != 1 || renames[0][0] != names[0] || renames[0][1] != "/dir/dst.bin" {
		t.Fatalf("renames = %v, want temp -> /dir/dst.bin", renames)
	}
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp files left on the server: %v", left)
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

// F3 + R2-1: a failed RETR completion (426/451) fails the copy; the pre-existing destination is untouched and only the temp is removed.
// Run on the streaming path and on the single-session fallback (N10: the fallback must not ignore the close error).
func TestFTPCopyFile_RetrCompletionFailureIsReturned(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		for _, end := range []string{"426", "451"} {
			t.Run(mode+"/"+end, func(t *testing.T) {
				s := startStrictFTP(t, func(s *strictFTP) {
					s.retrEnd = end
					if mode == "fallback" {
						s.maxConns = 1
					}
				})
				s.seed("/dir/dst.bin", oldDst)
				c := strictClient(t, s)
				err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
				if err == nil || !strings.Contains(err.Error(), "not confirmed") {
					t.Fatalf("CopyFile = %v, want the %s reply to the RETR reported as an unconfirmed source transfer", err, end)
				}
				assertDstUntouched(t, s, "/dir/dst.bin")
				if mode == "stream" {
					onlyTempsDeleted(t, s, 1) // the temp was fully stored, then discarded
				}
				s.mu.Lock()
				nren := len(s.renames)
				s.mu.Unlock()
				if nren != 0 {
					t.Fatalf("an unconfirmed copy was renamed into place (%d renames)", nren)
				}
			})
		}
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

// F5 + R2-1: a failed STOR (disk full after the data) leaves the old destination byte-identical and removes only the temp, on both paths
// (N8: the fallback must also remove its temp).
func TestFTPCopyFile_FailedStorLeavesDestinationAndRemovesTemp(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) {
				s.storEnd = "451"
				if mode == "fallback" {
					s.maxConns = 1
				}
			})
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
				t.Fatalf("CopyFile succeeded although STOR was answered 451")
			}
			assertDstUntouched(t, s, "/dir/dst.bin")
			onlyTempsDeleted(t, s, 1)
		})
	}
}

// R2-1 (the reviewer's data-loss case): STOR is refused before any data flows. The existing destination must survive, on both paths.
func TestFTPCopyFile_RefusedStorKeepsExistingDestination(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) {
				s.storRefuse = "553"
				if mode == "fallback" {
					s.maxConns = 1
				}
			})
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
				t.Fatalf("CopyFile succeeded although STOR was refused")
			}
			assertDstUntouched(t, s, "/dir/dst.bin")
			// the server answered the refusal, so cleanup needs no extra session: client + (stream only) the copy's own second one
			want := int32(2) // the client's own session + the copy's second one (granted for "stream", refused with 421 for "fallback")
			if n := atomic.LoadInt32(&s.accepted); n != want {
				t.Fatalf("control sessions opened = %d, want %d (a refused STOR must not trigger a cleanup redial)", n, want)
			}
			onlyTempsDeleted(t, s, 1)
			if d := s.deletedList(); len(d) != 1 {
				t.Fatalf("DELE seen = %v, want exactly the one temp", d)
			}
		})
	}
}

// R2-1: the server refuses to rename onto an existing file. That is reported; the old destination is untouched and the temp is removed.
func TestFTPCopyFile_RenameOverExistingRefusedKeepsDestination(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) {
				s.noRenameOver = true
				if mode == "fallback" {
					s.maxConns = 1
				}
			})
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
			if err == nil || !strings.Contains(err.Error(), "left unchanged") {
				t.Fatalf("CopyFile = %v, want an error saying the existing destination was left unchanged", err)
			}
			assertDstUntouched(t, s, "/dir/dst.bin")
			onlyTempsDeleted(t, s, 1)
		})
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

// ... and one cancelled while the data is flowing: the copy stops, reports the cancellation, removes only its temp, leaves the old
// destination and leaves the client with a working session. On the fallback path the STOR reads from the spool file, where the
// ctxReader is the only thing that can stop it (no network read to interrupt).
func TestFTPCopyFile_CancelDuringTransferStopsAndKeepsDestination(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			size := int64(256 << 20)
			if mode == "fallback" {
				size = 128 << 20 // the fallback spools the whole file to disk first
			}
			s := startStrictFTP(t, func(s *strictFTP) {
				s.retrSize = size
				if mode == "fallback" {
					s.maxConns = 1
				}
			})
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s, func(cfg *FTPConfig) { cfg.SpoolDir = t.TempDir() })
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				// stream: cancel once STOR bytes flow. fallback: cancel once the STOR of the spooled file has started.
				for atomic.LoadInt64(&s.storing) == 0 {
					time.Sleep(time.Millisecond)
				}
				cancel()
			}()
			err := c.CopyFile(ctx, "/big.bin", "/dir/dst.bin")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("CopyFile cancelled mid-transfer = %v, want context.Canceled", err)
			}
			if n := atomic.LoadInt64(&s.storing); n <= 0 || n >= size {
				t.Fatalf("server received %d bytes, want a partial transfer (0 < n < %d)", n, size)
			}
			assertDstUntouched(t, s, "/dir/dst.bin")
			onlyTempsDeleted(t, s, 1)
			if err := c.TestConnection(context.Background()); err != nil {
				t.Fatalf("client left without a working session after the cancelled copy: %v", err)
			}
		})
	}
}

// R2-6: a Read that is blocked in the network (the server stops sending) is unblocked by cancelling the context.
func TestFTPCopyFile_CancelUnblocksStalledSourceRead(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) {
				s.retrStall = true
				if mode == "fallback" {
					s.maxConns = 1
				}
			})
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			time.AfterFunc(1200*time.Millisecond, cancel)
			start := time.Now()
			err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("CopyFile = %v, want context.Canceled", err)
			}
			if d := time.Since(start); d > 8*time.Second {
				t.Fatalf("a stalled source read was not interrupted by the cancellation: returned after %v", d)
			}
			assertDstUntouched(t, s, "/dir/dst.bin")
			if err := c.TestConnection(context.Background()); err != nil {
				t.Fatalf("client left without a working session: %v", err)
			}
		})
	}
}

// R2-6: a destination write blocked in the network (the server never reads) is unblocked by cancelling the context, and the orphaned temp
// is removed over a fresh session. (Streaming path only: the fallback's STOR runs on the untracked own session, a documented limit.)
func TestFTPCopyFile_CancelUnblocksStalledDestinationWrite(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.retrSize = 256 << 20; s.storStall = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(700*time.Millisecond, cancel) // long enough for the socket buffers to fill and the write to block
	start := time.Now()
	err := c.CopyFile(ctx, "/big.bin", "/dir/dst.bin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Fatalf("a stalled destination write was not interrupted by the cancellation: returned after %v", d)
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
	onlyTempsDeleted(t, s, 1) // the temp was removed over a fresh session
}

// A server that allows one session only: the copy falls back to a temp file, still works, and removes the temp file.
func TestFTPCopyFile_SingleSessionServerFallsBackToTempFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	s := startStrictFTP(t, func(s *strictFTP) { s.maxConns = 1 })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("destination = %q, want %q", got, "hello-copy")
	}
	if n := atomic.LoadInt32(&s.rejected); n != 0 {
		t.Fatalf("rejected %d commands", n)
	}
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp files left on the server: %v", left)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Fatalf("temp files left behind: %v", left)
	}
}

// R2-4: the spool directory is configurable and is the one used (default os.TempDir).
func TestFTPCopyFile_FallbackUsesConfiguredSpoolDir(t *testing.T) {
	other := t.TempDir()
	t.Setenv("TMPDIR", other) // must NOT be used
	spool := t.TempDir()
	var inSpool, inOther int32
	s := startStrictFTP(t, func(s *strictFTP) {
		s.maxConns = 1
		s.onStor = func() { // the STOR of the spooled file starts only after the whole source is in the spool file
			a, _ := os.ReadDir(spool)
			b, _ := os.ReadDir(other)
			atomic.StoreInt32(&inSpool, int32(len(a)))
			atomic.StoreInt32(&inOther, int32(len(b)))
		}
	})
	c := strictClient(t, s, func(cfg *FTPConfig) { cfg.SpoolDir = spool })
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}
	if atomic.LoadInt32(&inSpool) != 1 || atomic.LoadInt32(&inOther) != 0 {
		t.Fatalf("spool file placement: %d in the configured SpoolDir, %d in TMPDIR; want 1 and 0", inSpool, inOther)
	}
	if left, _ := os.ReadDir(spool); len(left) != 0 {
		t.Fatalf("spool file left behind: %v", left)
	}
}

// R2-4: a spool directory that cannot be used fails the copy up front (nothing retrieved, nothing stored).
func TestFTPCopyFile_FallbackUnusableSpoolDirFailsBeforeTransfer(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.maxConns = 1 })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s, func(cfg *FTPConfig) { cfg.SpoolDir = "/nonexistent-spool-dir-for-test" })
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err == nil {
		t.Fatalf("CopyFile succeeded with an unusable spool directory")
	}
	if r := atomic.LoadInt32(&s.retrs); r != 0 {
		t.Fatalf("%d RETR issued although the spool file could not be created", r)
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
}

// R2-4: when the source SIZE is known and the spool directory clearly cannot hold it, the fallback refuses up front.
func TestFTPCopyFile_FallbackRefusesWhenSpoolHasInsufficientFreeSpace(t *testing.T) {
	old := freeSpaceFn
	t.Cleanup(func() { freeSpaceFn = old })
	freeSpaceFn = func(string) (uint64, error) { return 5, nil } // the responder reports SIZE 10
	spool := t.TempDir()
	s := startStrictFTP(t, func(s *strictFTP) { s.maxConns = 1 })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s, func(cfg *FTPConfig) { cfg.SpoolDir = spool })
	err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
	if err == nil || !strings.Contains(err.Error(), "free") {
		t.Fatalf("CopyFile = %v, want a refusal naming the missing free space", err)
	}
	if r := atomic.LoadInt32(&s.retrs); r != 0 {
		t.Fatalf("%d RETR issued although the spool is too small", r)
	}
	if left, _ := os.ReadDir(spool); len(left) != 0 {
		t.Fatalf("spool file created: %v", left)
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
	// ... and an unknown amount of free space does not block the copy
	freeSpaceFn = func(string) (uint64, error) { return 0, errors.New("unknown") }
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/other.bin"); err != nil {
		t.Fatalf("CopyFile with unknown free space: %v", err)
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

// F7: concurrent copies onto one destination must each STOR under their OWN temp name (a constant or reused suffix would let them
// overwrite each other's partial data), and the temp name must stay within NAME_MAX for any base name.
func TestFTPCopyFile_ConcurrentCopiesUseDistinctTempNames(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/dst.bin", oldDst)
	const n = 6
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		c := strictClient(t, s)
		wg.Add(1)
		go func() { defer wg.Done(); errs <- c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin") }()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatalf("concurrent copy failed: %v", e)
		}
	}
	s.mu.Lock()
	names := append([]string(nil), s.storNames...)
	s.mu.Unlock()
	seen := map[string]bool{}
	for _, nm := range names {
		if seen[nm] {
			t.Fatalf("two concurrent copies stored under the same temp name %q (all: %v)", nm, names)
		}
		seen[nm] = true
	}
	if len(names) != n {
		t.Fatalf("STOR names = %d (%v), want %d", len(names), names, n)
	}
}

func TestFTPTempSibling_StaysWithinNameMaxAndRuneBoundary(t *testing.T) {
	for _, base := range []string{strings.Repeat("a", 250), strings.Repeat("ü", 200), strings.Repeat("日", 120), "x"} {
		tmp := tempSibling("/dir/" + base)
		name := tmp[strings.LastIndex(tmp, "/")+1:]
		if len(name) > 255 {
			t.Fatalf("temp name for a %d-byte base is %d bytes, over NAME_MAX", len(base), len(name))
		}
		if !utf8.ValidString(name) {
			t.Fatalf("temp name %q is cut in the middle of a rune", name)
		}
		if !strings.Contains(name, ".ftpcopy-") || !strings.HasSuffix(name, ".part") {
			t.Fatalf("temp name %q lost its marker", name)
		}
	}
}

func TestFTPTempNeverCreated_OnlyNotFoundCounts(t *testing.T) {
	for msg, want := range map[string]bool{"no such file": true, "File not found": true, "permission denied": false, "": false} {
		if got := tempNeverCreated(&textproto.Error{Code: 550, Msg: msg}); got != want {
			t.Errorf("550 %q: never-created=%v, want %v", msg, got, want)
		}
	}
	if tempNeverCreated(&textproto.Error{Code: 421, Msg: "no such file"}) || tempNeverCreated(errors.New("EOF")) {
		t.Error("a 421 or a non-protocol error must not count as 'never created'")
	}
}

func TestFreeSpace_ReportsPositiveForRealDir(t *testing.T) {
	free, err := freeSpace(t.TempDir())
	if err != nil {
		t.Skipf("free space unknown on this platform: %v", err)
	}
	if free == 0 {
		t.Fatal("freeSpace reported 0 bytes for a writable temp dir")
	}
}

// F1 (round 4): the source server never answers on the control connection after the data connection is torn down. Cancel must still
// return promptly with the cancellation error, leave the destination untouched, and leave the client with a WORKING (replaced) session.
func TestFTPCopyFile_CancelWithBlackholedSourceControlReplacesSession(t *testing.T) {
	for _, mode := range []string{"stream", "fallback"} {
		t.Run(mode, func(t *testing.T) { blackholedSourceControl(t, mode == "fallback") })
	}
}

func blackholedSourceControl(t *testing.T, fallback bool) {
	s := startStrictFTP(t, func(s *strictFTP) {
		s.retrStall = true
		s.noAbortReply = true
		if fallback {
			s.maxConns = 1
		}
	})
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(1200*time.Millisecond, cancel)
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- c.CopyFile(ctx, "/src.bin", "/dir/dst.bin") }()
	var err error
	select {
	case err = <-done:
	case <-time.After(8 * time.Second):
		t.Fatal("CopyFile still blocked long after cancel (resp.Close waits for a control reply that never comes)")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want it to wrap context.Canceled (after %v)", err, time.Since(start))
	}
	if got, ok := s.file("/dir/dst.bin"); !ok || got != oldDst {
		t.Fatalf("destination = %q (exists=%v), want it untouched", got, ok)
	}
	if !c.IsConnected() {
		t.Fatal("client is left disconnected after the cancelled copy; the stuck session must be replaced by a live one")
	}
	// replacement proof (not just "some session works"): the client's own control connection + the copy's 2nd connection (or the refused
	// attempt, on the fallback path) are 2 accepts; the REPLACEMENT session is the 3rd. Without it the stuck session would still be in use.
	if n := atomic.LoadInt32(&s.accepted); n < 3 {
		t.Fatalf("control connections accepted = %d, want >= 3: the stuck source session was not replaced by a new one", n)
	}
	fctx, fcancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer fcancel()
	if ok, ferr := c.FileExists(fctx, "/dir/dst.bin"); ferr != nil || !ok {
		t.Fatalf("follow-up FileExists on the replaced session = %v, %v; want true, nil", ok, ferr)
	}
}

// F3 (round 4): when the temp cannot be removed, the error must name it (never a silent orphan).
func TestFTPCopyFile_OrphanTempIsNamedInError(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true; s.deleRefuse = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
	if err == nil {
		t.Fatal("CopyFile succeeded although rename-over was refused")
	}
	left := s.tempNames()
	if len(left) != 1 {
		t.Fatalf("temps on server = %v, want exactly the one orphan", left)
	}
	// a permanent refusal (550 permission denied) is final: no extra cleanup session may be opened (client + the copy's 2nd connection = 2)
	if n := atomic.LoadInt32(&s.accepted); n != 2 {
		t.Fatalf("control connections accepted = %d, want 2 (a permanent DELE refusal must not trigger a retry on a fresh session)", n)
	}
	if !strings.Contains(err.Error(), left[0]) {
		t.Fatalf("error %q does not name the orphan temp %s", err, left[0])
	}
	if got, _ := s.file("/dir/dst.bin"); got != oldDst {
		t.Fatalf("destination = %q, want untouched", got)
	}
}

// ---- round 5 -------------------------------------------------------------------------------------------------------------------------

// F8: a cancelled copy whose STOR cleanup fails must still say that the temp may remain (the cancel mapping must not drop the note).
// M12: the cleanup session cannot be opened (the server refuses new sessions). M13b: the fresh-session DELE is refused.
func TestFTPCopyFile_CancelWithFailedCleanupNamesOrphan(t *testing.T) {
	cases := map[string]func(*strictFTP){
		"cleanup-session-dial-refused": func(s *strictFTP) { s.refuseAfter = 2 },
		"fresh-session-dele-refused":   func(s *strictFTP) { s.deleRefuse = true },
	}
	for name, mod := range cases {
		t.Run(name, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) { s.retrSize = 256 << 20; s.storStall = true; mod(s) })
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			time.AfterFunc(700*time.Millisecond, cancel)
			err := c.CopyFile(ctx, "/big.bin", "/dir/dst.bin")
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("CopyFile = %v, want context.Canceled", err)
			}
			left := s.tempNames()
			if len(left) != 1 {
				t.Fatalf("temps on server = %v, want exactly the one orphan (the cleanup was made to fail)", left)
			}
			if !strings.Contains(err.Error(), left[0]) || !strings.Contains(err.Error(), "may remain") {
				t.Fatalf("cancel error %q does not carry the orphan note for %s", err, left[0])
			}
			var oe *orphanError
			if !errors.As(err, &oe) || oe.tmp != left[0] {
				t.Fatalf("errors.As(*orphanError) failed or names another temp: %v", err)
			}
			if got, _ := s.file("/dir/dst.bin"); got != oldDst {
				t.Fatalf("destination = %q, want untouched", got)
			}
		})
	}
}

// F4: a cancel that arrives after the transfer is confirmed but before the watcher is disarmed abandons the copy (Canceled, destination
// untouched, temp removed, no rename). Kills `if false && !stopWatch()` / `if false {...}` (the disarm result unused).
func TestFTPCopyFile_CancelBeforeDisarmAbandonsCopy(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(setCopyHooksForTest(&copyHooks{beforeDisarm: func() { cancel(); time.Sleep(300 * time.Millisecond) }})) // the watcher fires (closes the connections) meanwhile
	err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "before it was moved into place") {
		t.Fatalf("CopyFile = %v, want a Canceled error saying the copy was abandoned before the move", err)
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
	onlyTempsDeleted(t, s, 1)
	s.mu.Lock()
	n := len(s.renames)
	s.mu.Unlock()
	if n != 0 {
		t.Fatalf("%d rename(s) happened for an abandoned copy", n)
	}
}

// F4: a cancel that arrives after the disarm (before the commit) is too late to matter: the copy completes and the destination is replaced.
// Kills a mutant that never disarms (the late cancel would close the destination connection and fail the rename).
func TestFTPCopyFile_CancelAfterDisarmStillCommits(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(setCopyHooksForTest(&copyHooks{beforeCommit: func() { cancel(); time.Sleep(300 * time.Millisecond) }}))
	if err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile = %v, want nil: a cancel after the disarm must not abort the rename", err)
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("destination = %q, want the new content", got)
	}
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp left: %v", left)
	}
}

// F4 wording: a refusal says the destination was left unchanged; a connection that dies during the rename says the outcome is unknown
// and must NOT claim "left unchanged".
func TestFTPCopyFile_RenameFailureWording(t *testing.T) {
	t.Run("server-refusal", func(t *testing.T) {
		s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true })
		s.seed("/dir/dst.bin", oldDst)
		c := strictClient(t, s)
		err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
		if err == nil || !strings.Contains(err.Error(), "left unchanged") || strings.Contains(err.Error(), "may or may not") {
			t.Fatalf("err = %v, want the 'left unchanged' wording", err)
		}
	})
	t.Run("connection-lost", func(t *testing.T) {
		s := startStrictFTP(t, func(s *strictFTP) { s.rntoDrop = true })
		s.seed("/dir/dst.bin", oldDst)
		c := strictClient(t, s)
		err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
		if err == nil || !strings.Contains(err.Error(), "may or may not have been applied") || strings.Contains(err.Error(), "left unchanged") {
			t.Fatalf("err = %v, want the 'may or may not have been applied' wording and no 'left unchanged' claim", err)
		}
	})
}

// F9/F-F: a cancel during a stalled RNTO (after the watcher was disarmed) is cut off renameGraceAfterCancel after the cancel: the
// destination connection is closed, the call returns, and the error says the rename may or may not have been applied.
func TestFTPCopyFile_CancelDuringStalledRenameIsCutOff(t *testing.T) {
	old := renameGraceAfterCancel
	renameGraceAfterCancel = 300 * time.Millisecond
	t.Cleanup(func() { renameGraceAfterCancel = old })
	s := startStrictFTP(t, func(s *strictFTP) { s.rntoStall = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(300*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() { done <- c.CopyFile(ctx, "/src.bin", "/dir/dst.bin") }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "may or may not have been applied") || strings.Contains(err.Error(), "left unchanged") {
			t.Fatalf("CopyFile = %v, want the 'may or may not have been applied' wording", err)
		}
	case <-time.After(8 * time.Second):
		t.Fatal("CopyFile still blocked 8s after the cancel: a stalled RNTO is not cut off")
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp left after the cut-off: %v", left)
	}
}

// Without a cancel a slow rename is never cut off (the grace window only starts at the cancel): the copy waits and succeeds.
func TestFTPCopyFile_SlowRenameWithoutCancelCompletes(t *testing.T) {
	old := renameGraceAfterCancel
	renameGraceAfterCancel = 50 * time.Millisecond
	t.Cleanup(func() { renameGraceAfterCancel = old })
	s := startStrictFTP(t, func(s *strictFTP) { s.rntoDelay = 600 * time.Millisecond })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	if err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile = %v, want nil", err)
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("destination = %q", got)
	}
}

// FTPClient.ListDirectory reports EVERY entry, CopyFile temp names included: a caller that lists, moves and then removes a directory (the
// move in internal/services/protocol_handlers.go) must see what is really there. The catalogue scanner hides the temps (see
// internal/services/generic_scanner.go and its test).
func TestFTPListDirectory_ReportsCopyTempFilesToo(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) {
		s.listing = "-rw-r--r-- 1 u g 10 Jan 01 2020 f.bin\r\n" +
			"-rw-r--r-- 1 u g 10 Jan 01 2020 .dst.bin.ftpcopy-0123456789abcdef.part\r\n" +
			"-rw-r--r-- 1 u g 10 Jan 01 2020 .x.part\r\n" +
			"-rw-r--r-- 1 u g 10 Jan 01 2020 .y.ftpcopy-XYZ.part\r\n" +
			"-rw-r--r-- 1 u g 10 Jan 01 2020 dst.bin.ftpcopy-0123456789abcdef.part\r\n"
	})
	c := strictClient(t, s)
	files, err := c.ListDirectory(context.Background(), "/dir")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	want := []string{"f.bin", ".dst.bin.ftpcopy-0123456789abcdef.part", ".x.part", ".y.ftpcopy-XYZ.part", "dst.bin.ftpcopy-0123456789abcdef.part"}
	if strings.Join(names, "|") != strings.Join(want, "|") {
		t.Fatalf("listed %v, want %v (nothing is hidden by the client)", names, want)
	}
}

func TestFTPIsCopyTemp_MatchesWhatTempSiblingProduces(t *testing.T) {
	for _, base := range []string{"a.bin", "x", ".hidden", strings.Repeat("é", 150), "name with spaces.mkv", strings.Repeat("\x80", 250), "line1\nline2.bin", strings.Repeat("a", 199) + "日" + strings.Repeat("b", 60)} {
		if n := pathpkg.Base(tempSibling("/d/" + base)); !IsCopyTempName(n) {
			t.Fatalf("tempSibling name %q is not recognised by IsCopyTempName", n)
		}
	}
	for _, n := range []string{".x.part", ".a.ftpcopy-0123456789abcde.part", ".a.ftpcopy-0123456789ABCDEF.part", "a.ftpcopy-0123456789abcdef.part", ".ftpcopy-0123456789abcdef.part"} {
		if IsCopyTempName(n) {
			t.Fatalf("%q must not be taken for a copy temp", n)
		}
	}
}
