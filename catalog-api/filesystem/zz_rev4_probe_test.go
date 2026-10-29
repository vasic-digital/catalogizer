package filesystem

import (
	"context"
	"errors"
	"net/textproto"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

// R1: cancel during STOR (destination stalled) while DELE is refused: is the orphan reported?
func TestR4_CancelDuringStorOrphanReported(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.storStall = true; s.deleRefuse = true; s.storCreateEarly = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(800*time.Millisecond, cancel)
	err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
	t.Logf("err=%v temps=%v", err, s.tempNames())
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("want canceled, got %v", err)
	}
	// the stalled STOR never created the temp in the responder map (it only records on completion) -> this scenario can't leave a file;
	// the point is only whether the error text carries cleanup failure
	if len(s.tempNames()) > 0 && !strings.Contains(err.Error(), s.tempNames()[0]) {
		t.Errorf("FINDING F8: orphan %v exists on server but the cancel error does not name it: %v", s.tempNames(), err)
	}
}

// R2: tempNeverCreated on real-world wordings
func TestR4_TempNeverCreatedWordings(t *testing.T) {
	for msg, want := range map[string]bool{
		"Delete operation failed.":                      false, // vsftpd (any cause)
		"foo.part: No such file or directory":           true,  // proftpd
		"Can't delete: No such file or directory":       true,  // pure-ftpd
		"The system cannot find the file specified.":    false, // IIS (localized/other wording => conservative)
		"Datei nicht gefunden":                          false, // localized: conservative => reported as maybe-orphan
		"File not found or permission denied":           true,  // AMBIGUOUS: treated as never-created
		"No such privilege":                             true,  // matches "no such" but is a permission refusal
		"Could not delete: does not exist or read-only": true,
	} {
		got := tempNeverCreated(&textproto.Error{Code: 550, Msg: msg})
		t.Logf("%-55q never-created=%v", msg, got)
		_ = want
	}
}

// R3: rune boundary with invalid UTF-8 / continuation-only
func TestR4_TempSiblingInvalidUTF8(t *testing.T) {
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
		t.Logf("%s: base=%d name=%d validutf8=%v prefix=%q", n, len(b), len(name), utf8.ValidString(name), name[:min(12, len(name))])
		if len(name) > 255 {
			t.Errorf("%s over NAME_MAX", n)
		}
	}
	// uniqueness for cut-down names
	seen := map[string]bool{}
	for i := 0; i < 1000; i++ {
		seen[tempSibling("/d/"+cases["allcont"])] = true
	}
	if len(seen) != 1000 {
		t.Errorf("collisions: %d", len(seen))
	}
}

// R4: cancel at assorted delays while 8 copies run concurrently; no goroutine growth, race clean, destination intact
func TestR4_ConcurrentCancelStorm(t *testing.T) {
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
			t.Errorf("dst %d changed: %q", i, got)
		}
	}
	if bad != 0 {
		t.Errorf("bad=%d", bad)
	}
	t.Logf("temps left=%v deleted=%v", s.tempNames(), s.deletedList())
}

// R5: cancel while RNTO stalls (after disarm)
func TestR4_CancelDuringRenameStall(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.rntoStall = true })
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(700*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() { done <- c.CopyFile(ctx, "/src.bin", "/dir/dst.bin") }()
	select {
	case err := <-done:
		t.Logf("returned: %v", err)
	case <-time.After(6 * time.Second):
		// F9: documented limit (CopyFile doc comment), pinned by TestFTPCopyFile_CancelDuringStalledRenameIsNotHonoured; not a failure here
		t.Logf("F9 (documented limit): cancel during stalled RNTO is not honoured (CopyFile blocked >5s after cancel)")
	}
}
