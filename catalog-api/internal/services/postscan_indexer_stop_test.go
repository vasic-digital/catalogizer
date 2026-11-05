package services

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

// F2: Stop() must cancel a running post-scan indexer promptly instead of waiting for it.
func TestPostScanIndexer_StopCancelsRunningIndexer(t *testing.T) {
	s := NewUniversalScanner(nil, zap.NewNop(), nil, nil)
	started := make(chan struct{})
	s.SetPostScanIndexer(func(ctx context.Context, _ int64) error {
		close(started)
		<-ctx.Done() // a real indexer only returns when its context ends
		return ctx.Err()
	})
	s.startPostScanIndexer("job", 1, nil)
	<-started
	done := make(chan struct{})
	go func() { s.Stop(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stop() blocked on a running indexer")
	}
}
