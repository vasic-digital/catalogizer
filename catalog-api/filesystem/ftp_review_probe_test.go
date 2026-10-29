package filesystem

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

// P1: long basename (NAME_MAX 255) must still be copyable.
func TestRev_P1_LongBasename(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.maxNameLen = 255 })
	c := strictClient(t, s)
	name := "/dir/" + strings.Repeat("a", 250)
	if err := c.CopyFile(context.Background(), "/src.bin", name); err != nil {
		t.Fatalf("P1 CopyFile to a 250-char name: %v", err)
	}
}

// P2: rename refused AND temp DELE refused: temp left, error silent about it.
func TestRev_P2_OrphanTempSilent(t *testing.T) {
	s := startStrictFTP(t, func(s *strictFTP) { s.noRenameOver = true; s.deleRefuse = true })
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/src.bin", "/dir/dst.bin")
	t.Logf("P2 err=%v temps=%v", err, s.tempNames())
	if err != nil && len(s.tempNames()) > 0 && !strings.Contains(err.Error(), ".ftpcopy-") {
		t.Fatalf("P2 orphan temp %v left on the server but the error does not name it", s.tempNames())
	}
}

// P3: concurrent copies, separate clients, same destination.
func TestRev_P3_ConcurrentSameDest(t *testing.T) {
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
			t.Fatalf("P3 concurrent copy failed: %v", e)
		}
	}
	if got, _ := s.file("/dir/dst.bin"); got != "hello-copy" {
		t.Fatalf("P3 dst=%q", got)
	}
	if l := s.tempNames(); len(l) != 0 {
		t.Fatalf("P3 temps left %v", l)
	}
}

// P4: odd path chars (dots, unicode, dash-leading); fixture splits on space so no spaces.
func TestRev_P4_OddNames(t *testing.T) {
	for _, d := range []string{"/dir/-rf", "/dir/ünï.bin", "/dir/.hidden", "/dir/a..b", "/x"} {
		s := startStrictFTP(t)
		c := strictClient(t, s)
		if err := c.CopyFile(context.Background(), "/src.bin", d); err != nil {
			t.Errorf("P4 %q: %v", d, err)
			continue
		}
		if got, _ := s.file(d); got != "hello-copy" {
			t.Errorf("P4 %q content %q temps=%v", d, got, s.tempNames())
		}
	}
}

// P5: cancel with a blackholed source control reply (no 426 after data close): must return promptly.
func TestRev_P5_CancelBlackholeControl(t *testing.T) {
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
		t.Logf("P5 returned %v", err)
	case <-time.After(10 * time.Second):
		t.Fatalf("P5 CopyFile still blocked 9s after cancel (resp.Close waits for a control reply that never comes)")
	}
}

// P6: source == destination
func TestRev_P6_SameSrcDst(t *testing.T) {
	s := startStrictFTP(t)
	s.seed("/dir/a.bin", oldDst)
	c := strictClient(t, s)
	err := c.CopyFile(context.Background(), "/dir/a.bin", "/dir/a.bin")
	t.Logf("P6 err=%v content=%q", err, func() string { g, _ := s.file("/dir/a.bin"); return g }())
}
