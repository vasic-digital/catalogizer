package filesystem

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// R7-1: spool fallback, cancel while the spooled STOR runs, cleanup DELE refused -> the orphan note must survive the cancel mapping
// (the CHANGELOG promises errors.As(*orphanError) "also on the cancel path").
func TestR7_SpoolCancelDuringStorKeepsOrphanNote(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := startStrictFTP(t, func(s *strictFTP) {
		s.maxConns = 1
		s.deleRefuse = true
		s.onStor = func() { cancel() }
	})
	s.seed("/dir/dst.bin", oldDst)
	c := strictClient(t, s, func(cfg *FTPConfig) { cfg.SpoolDir = t.TempDir() })
	err := c.CopyFile(ctx, "/src.bin", "/dir/dst.bin")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyFile = %v, want Canceled", err)
	}
	left := s.tempNames()
	t.Logf("err=%v temps=%v", err, left)
	if len(left) != 1 {
		t.Fatalf("temps = %v, want the one orphan", left)
	}
	var oe *orphanError
	if !errors.As(err, &oe) || !strings.Contains(err.Error(), left[0]) {
		t.Fatalf("spool-path cancel dropped the orphan note: %v (temp %s remains)", err, left[0])
	}
}

// R7-2: a refused STOR on a server that never sends the final RETR reply after an early data close: with a context that has no deadline
// CopyFile blocks forever (closeResp only gives up on ctx.Done). Pre-existing class (old code: defer resp.Close()).
