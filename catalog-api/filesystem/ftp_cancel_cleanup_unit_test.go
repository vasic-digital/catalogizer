package filesystem

import (
	"context"
	"errors"
	"net/textproto"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// Cancel during a stalled STOR while the cleanup DELE is refused: the temp stays on the server and the cancel error must name it.
func TestFTPCopyFile_CancelDuringStorOrphanReported(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.storStall = true; s.deleRefuse = true; s.storCreateEarly = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(800*time.Millisecond, cancel)
	err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	left := s.tempNames()
	if len(left) != 1 {
		t.Fatalf("scenario broken: want exactly 1 temp left on the server (the STOR creates it at open and DELE is refused), got %v", left)
	}
	if !strings.Contains(err.Error(), left[0]) {
		t.Errorf("orphan %v exists on the server but the cancel error does not name it: %v", left, err)
	}
	var oe *orphanError
	if !errors.As(err, &oe) {
		t.Errorf("the orphan note must be reachable with errors.As: %v", err)
	}
	if got, _ := s.file("/dir/dst.bin"); got != oldDst {
		t.Errorf("destination changed to %q", got)
	}
}

// tempNeverCreated on real-world and adversarial 550 wordings. Only a text naming a missing file or directory, without any refusal wording,
// means "nothing to clean up"; everything else must be reported as a possible orphan.
func TestTempNeverCreatedWordings(t *testing.T) {
	for msg, want := range map[string]bool{
		"Delete operation failed.":                           false, // vsftpd (any cause)
		"foo.part: No such file or directory":                true,  // proftpd
		"Can't delete: No such file or directory":            true,  // pure-ftpd
		"no such file":                                       true,
		"No such directory":                                  true,
		"foo.part: File not found":                           true,
		"foo.part does not exist":                            true,
		"The system cannot find the file specified.":         false, // IIS: other wording => conservative
		"Datei nicht gefunden":                               false, // localized => conservative
		"File not found or permission denied":                false, // mixed: cannot tell => possible orphan
		"File unavailable (e.g., file not found, no access)": false, // RFC 959 generic 550: may be a refusal
		"No such privilege":                                  false, // contains "no such", but it is a permission refusal
		"No such user":                                       false,
		"Could not delete: does not exist or read-only":      false, // read-only vetoes
		"Permission denied":                                  false,
		"Access is denied, file not found":                   false,
		"":                                                   false,
	} {
		if got := tempNeverCreated(&textproto.Error{Code: 550, Msg: msg}); got != want {
			t.Errorf("tempNeverCreated(550 %q) = %v, want %v", msg, got, want)
		}
	}
	// only 550 counts; other codes and non-protocol errors never do
	for _, code := range []int{450, 421, 500, 551, 553} {
		if tempNeverCreated(&textproto.Error{Code: code, Msg: "No such file or directory"}) {
			t.Errorf("code %d must not be read as never-created", code)
		}
	}
	if tempNeverCreated(errors.New("No such file or directory")) || tempNeverCreated(nil) {
		t.Error("a non-protocol error / nil must not be read as never-created")
	}
}

// End to end: a refused rename plus a DELE refusal whose text only LOOKS like "never created" must still leave the orphan note.
func TestFTPCopyFile_AmbiguousDeleRefusalStillReportsOrphan(t *testing.T) {
	for _, msg := range []string{"No such privilege", "File not found or permission denied"} {
		t.Run(msg, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true; s.deleRefuse = true; s.deleRefuseMsg = msg })
			s.seed("/dir/dst.bin", oldDst)
			c := strictClient(t, s)
			err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
			left := s.tempNames()
			if err == nil || len(left) != 1 {
				t.Fatalf("scenario broken: err=%v temps=%v", err, left)
			}
			var oe *orphanError
			if !errors.As(err, &oe) || !strings.Contains(err.Error(), left[0]) {
				t.Fatalf("orphan %v left on the server for DELE answer %q, but the error does not carry the orphan note: %v", left, msg, err)
			}
		})
	}
}

// tempSibling with invalid UTF-8 / continuation-only bases: the name stays within NAME_MAX, is recognised as a copy temp, and stays unique.
func TestTempSiblingInvalidUTF8(t *testing.T) {
	cases := map[string]string{
		"allcont":  strings.Repeat("\x80", 250),
		"allff":    strings.Repeat("\xff", 250),
		"mixed":    strings.Repeat("a", 199) + "日" + strings.Repeat("\x80", 60),
		"exact200": strings.Repeat("a", 200),
		"201":      strings.Repeat("a", 201),
	}
	for n, b := range cases {
		tmp := tempSibling("/d/" + b)
		name := tmp[strings.LastIndex(tmp, "/")+1:]
		if len(name) > 255 {
			t.Errorf("%s: name is %d bytes, over NAME_MAX", n, len(name))
		}
		if !IsCopyTempName(name) {
			t.Errorf("%s: %q is not recognised as a copy temp", n, name)
		}
		// the cut steps back at most utf8.UTFMax-1 bytes: a base over the limit keeps at least maxTempBase-3 bytes of itself, so the
		// step-back is bounded (kills a mutant that walks back up to 100 bytes and silently shortens the name)
		if len(b) > maxTempBase {
			kept := len(name) - len(".") - len(".ftpcopy-0123456789abcdef.part")
			if kept < maxTempBase-3 || kept > maxTempBase {
				t.Errorf("%s: kept %d base bytes, want %d..%d", n, kept, maxTempBase-3, maxTempBase)
			}
		}
		if utf8.ValidString(b) && !utf8.ValidString(name) {
			t.Errorf("%s: a valid base was cut into invalid UTF-8", n)
		}
	}
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		seen[tempSibling("/d/"+cases["allcont"])] = true
	}
	if len(seen) != 1000 {
		t.Errorf("temp names collide: %d distinct of 1000", len(seen))
	}
}

// Cancel at assorted delays while 8 copies run concurrently: every copy returns an error in bounded time (no hang), no destination changes.
func TestFTPCopyFile_ConcurrentCancelStorm(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.retrStall = true; s.noAbortReply = true })
	for i := 0; i < 8; i++ {
		s.seed("/dir/d"+string(rune('a'+i)), oldDst)
	}
	var wg sync.WaitGroup
	var bad int32
	for i := 0; i < 8; i++ {
		wg.Add(1)
		c := strictClient(t, s)
		go func(i int) {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			time.AfterFunc(time.Duration(100+i*150)*time.Millisecond, cancel)
			done := make(chan error, 1)
			go func() { done <- c.CopyFile(ctx, "/src.bin", "/dir/d"+string(rune('a'+i))) }()
			select {
			case err := <-done:
				if err == nil {
					atomic.AddInt32(&bad, 1)
				}
			case <-time.After(20 * time.Second):
				atomic.AddInt32(&bad, 1)
				t.Errorf("copy %d hung", i)
			}
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		if got, _ := s.file("/dir/d" + string(rune('a'+i))); got != oldDst {
			t.Errorf("destination %d changed: %q", i, got)
		}
	}
	if bad != 0 {
		t.Errorf("%d of 8 copies did not return an error in time", bad)
	}
}

// N9: the single-session fallback must stop copying when the context is cancelled while data is still FLOWING (not only when the read is
// blocked). A large, fast source is cancelled mid-transfer; CopyFile must return Canceled promptly and leave no spool file or temp.
func TestFTPCopyFile_FallbackCancelMidFlowingTransfer(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	s := startStrictFTP(t, func(s *strictFTP) { s.maxConns = 1; s.retrSize = 512 << 20 })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(150*time.Millisecond, cancel)
	start := time.Now()
	err := c.CopyFile(ctx, "/big.bin", "/dir/dst.bin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile = %v, want context.Canceled", err)
	}
	if d := time.Since(start); d > 8*time.Second {
		t.Fatalf("flowing fallback transfer ignored the cancellation: %v", d)
	}
	assertDstUntouched(t, s, "/dir/dst.bin")
	if ents, _ := os.ReadDir(tmp); len(ents) != 0 {
		t.Fatalf("spool file left behind: %v", ents)
	}
}
