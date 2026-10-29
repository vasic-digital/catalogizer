package filesystem

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// long basename (NAME_MAX 255) must still be copyable.
func TestFTPCopyFile_LongBasename(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.maxNameLen = 255 })
	c := strictClient(t, s)
	name := "/dir/" + strings.Repeat("a", 250)
	if err := c.CopyFile(context.Background(), "/src.bin", name); err != nil {
		t.Fatalf("CopyFile to a 250-char name: %v", err)
	}
}

// rename refused AND temp DELE refused: temp left, error silent about it.
func TestFTPCopyFile_RefusedRenameAndDeleNamesOrphan(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true; s.deleRefuse = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
	left := s.tempNames()
	if err == nil || len(left) != 1 {
		t.Fatalf("scenario broken: err=%v temps=%v", err, left)
	}
	if !strings.Contains(err.Error(), left[0]) {
		t.Fatalf("orphan temp %v left on the server but the error does not name it: %v", left, err)
	}
}

// concurrent copies, separate clients, same destination.
func TestFTPCopyFile_ConcurrentSameDestination(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/dst.bin", oldDst)
	var wg sync.WaitGroup
	errs := make(chan error, 6)
	for i := 0; i < 6; i++ {
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
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("dst=%q", got)
	}
	if l := s.tempNames(); len(l) != 0 {
		t.Fatalf("temps left %v", l)
	}
}

// odd path chars (dots, unicode, dash-leading); fixture splits on space so no spaces.
func TestFTPCopyFile_OddNames(t *testing.T) {
	for _, d := range []string{"/dir/-rf", "/dir/ünï.bin", "/dir/.hidden", "/dir/a..b", "/x"} {
		s := startStrictFTP(t)
		c := strictClient(t, s)
		if err := c.CopyFile(context.Background(), "/src.bin", d); err != nil {
			t.Errorf("%q: %v", d, err)
			continue
		}
		if got, _ := s.file(d); got != "hello-copy" {
			t.Errorf("%q content %q temps=%v", d, got, s.tempNames())
		}
	}
}

// cancel with a blackholed source control reply (no 426 after data close): must return promptly.
func TestFTPCopyFile_CancelWithBlackholedControlReturnsPromptly(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.retrStall = true; s.noAbortReply = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	time.AfterFunc(800*time.Millisecond, cancel)
	done := make(chan error, 1)
	go func() { done <- c.CopyFile(ctx, "/src.bin", "/dir/dst.bin") }()
	select {
	case err := <-done:
		if err == nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("CopyFile = %v, want a Canceled error", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("CopyFile still blocked 9s after cancel (resp.Close waits for a control reply that never comes)")
	}
}

// source == destination
func TestFTPCopyFile_SameSourceAndDestination(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/a.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/dir/a.bin", "/dir/a.bin")
	if err != nil {
		t.Fatalf("CopyFile onto itself = %v", err)
	}
	// the responder's RETR always serves "hello-copy"; what matters is that the file still exists, holds a complete copy and no temp is left
	if got, ok := s.file("/dir/a.bin"); !ok || got != "hello-copy" {
		t.Fatalf("content after copying a file onto itself = %q (exists=%v)", got, ok)
	}
	if left := s.tempNames(); len(left) != 0 {
		t.Fatalf("temp left: %v", left)
	}
}
