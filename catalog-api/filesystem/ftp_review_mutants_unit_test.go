package filesystem

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func requireOrphan(t *testing.T, err error) *orphanError {
	t.Helper()
	var oe *orphanError
	if err == nil || !errors.As(err, &oe) || !strings.HasPrefix(oe.tmp, "/dir/.dst.bin.ftpcopy-") {
		t.Fatalf("error %v does not carry an *orphanError naming the temp", err)
	}
	return oe
}

// Every error branch that follows a failed cleanup must keep the orphan note: cancel after the store, unconfirmed source, abandon before
// the move. (Reviewer mutants MY3/MY4/MY5: each dropped the note on one branch.)
func TestFTPCopyFile_OrphanNoteSurvivesEveryErrorBranch(t *testing.T) {
	t.Run("cancel-after-store-before-source-confirmation", func(t *testing.T) {
		s := startStrictFTP(t, func(s *strictFTP) { s.deleRefuse = true; s.retrEndDelay = 3 * time.Second })
		s.seed("/dir/dst.bin", oldDst)
		c := strictClient(t, s)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		time.AfterFunc(600*time.Millisecond, cancel)
		err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want Canceled", err)
		}
		requireOrphan(t, err)
	})
	t.Run("unconfirmed-source", func(t *testing.T) {
		s := startStrictFTP(t, func(s *strictFTP) { s.deleRefuse = true; s.retrEnd = "451" })
		s.seed("/dir/dst.bin", oldDst)
		c := strictClient(t, s)
		err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
		if err == nil || !strings.Contains(err.Error(), "not confirmed") {
			t.Fatalf("err = %v, want 'not confirmed'", err)
		}
		requireOrphan(t, err)
		assertDstContent(t, s)
	})
	t.Run("abandon-before-move", func(t *testing.T) {
		s := startStrictFTP(t, func(s *strictFTP) { s.deleRefuse = true })
		s.seed("/dir/dst.bin", oldDst)
		c := strictClient(t, s)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		t.Cleanup(setCopyHooksForTest(&copyHooks{beforeDisarm: func() { cancel(); time.Sleep(300 * time.Millisecond) }}))
		err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want Canceled", err)
		}
		requireOrphan(t, err)
		assertDstContent(t, s)
	})
}

func assertDstContent(t *testing.T, s *strictFTP) {
	t.Helper()
	if got, _ := s.file("/dir/dst.bin"); got != oldDst {
		t.Fatalf("destination = %q, want untouched %q", got, oldDst)
	}
}

// The orphan is also written to the log (the operator's only trace if the caller drops the error). Kills MY10.
func TestFTPCopyFile_OrphanIsLogged(t *testing.T) {
	var buf bytes.Buffer
	old := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(old) })
	s := startStrictFTP(t, func(s *strictFTP) { s.deleRefuse = true; s.retrEnd = "451" })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
	oe := requireOrphan(t, err)
	if !strings.Contains(buf.String(), "may remain on the server") || !strings.Contains(buf.String(), oe.tmp) {
		t.Fatalf("log %q does not name the orphaned temp %s", buf.String(), oe.tmp)
	}
}

// A transient (4xx) DELE refusal is retried over a fresh session; a permanent (5xx) one is not. Kills MY7 (>=400) and the 900 variant.
func TestFTPCopyFile_DeleRefusalRetryPolicy(t *testing.T) {
	for _, c := range []struct {
		name     string
		code     int
		attempts int32
	}{{"transient-450-retries", 450, 2}, {"permanent-550-does-not", 550, 1}} {
		t.Run(c.name, func(t *testing.T) {
			s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true; s.deleRefuse = true; s.deleCode = c.code })
			s.seed("/dir/dst.bin", oldDst)
			cl := strictClient(t, s)
			err := cl.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
			requireOrphan(t, err)
			if got := atomic.LoadInt32(&s.deleAttempts); got != c.attempts {
				t.Fatalf("DELE attempts = %d, want %d", got, c.attempts)
			}
		})
	}
}

// Names that are NOT copy temps (kills MY1 end anchor, MY11 empty base, and a missing start anchor / wrong hex).
func TestIsCopyTempNameRejectsLookAlikes(t *testing.T) {
	const h = "0123456789abcdef"
	for _, n := range []string{
		"." + "a.ftpcopy-" + h + ".part.bak",
		"." + "a.ftpcopy-" + h + ".partial",
		"." + "a.ftpcopy-" + h + ".part\n",
		"..ftpcopy-" + h + ".part",
		"a.ftpcopy-" + h + ".part",
		".a.ftpcopy-" + h[:15] + ".part",
		".a.ftpcopy-" + h + "0.part",
		".a.ftpcopy-0123456789ABCDEF.part",
		".a.ftpcopy-" + h,
	} {
		if IsCopyTempName(n) {
			t.Errorf("%q must not be a copy temp", n)
		}
	}
	for _, n := range []string{".a.ftpcopy-" + h + ".part", ".a\nb.ftpcopy-" + h + ".part", "..x.ftpcopy-" + h + ".part"} {
		if !IsCopyTempName(n) {
			t.Errorf("%q must be a copy temp", n)
		}
	}
}

// A 4-byte rune straddling the cut needs three steps back; two would leave invalid UTF-8. Kills MY8.
func TestTempSiblingFourByteRuneAtCut(t *testing.T) {
	b := strings.Repeat("a", maxTempBase-3) + "😀" + strings.Repeat("b", 50)
	tmp := tempSibling("/d/" + b)
	name := tmp[strings.LastIndex(tmp, "/")+1:]
	if !utf8.ValidString(name) {
		t.Fatalf("temp name cut a 4-byte rune in half: %q", name)
	}
	if !IsCopyTempName(name) || len(name) > 255 {
		t.Fatalf("bad temp name %q (%d bytes)", name, len(name))
	}
}

// A cancel that arrives during a healthy (merely slow) rename must not cut it off: the copy commits. Kills a zero grace.
func TestFTPCopyFile_CancelDuringSlowHealthyRenameStillCommits(t *testing.T) {
	old := renameGraceAfterCancel
	renameGraceAfterCancel = 3 * time.Second
	t.Cleanup(func() { renameGraceAfterCancel = old })
	s := startStrictFTP(t, func(s *strictFTP) { s.rntoDelay = 500 * time.Millisecond })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	t.Cleanup(setCopyHooksForTest(&copyHooks{beforeCommit: func() { cancel() }}))
	if err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin"); err != nil {
		t.Fatalf("CopyFile = %v, want success (rename answered within the grace)", err)
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("destination = %q, want the copied content", got)
	}
}
