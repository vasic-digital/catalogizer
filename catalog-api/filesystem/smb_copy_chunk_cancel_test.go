package filesystem

import (
	"context"
	"errors"
	"io"
	"testing"
)

type cancelReader struct {
	cancel func()
	reads  int
}

func (r *cancelReader) Read(p []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		r.cancel()
	}
	return len(p), nil
}

func TestCopyChunkedStopsAfterCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &cancelReader{cancel: cancel}
	done := make(chan error, 1)
	go func() { done <- copyChunked(ctx, io.Discard, io.LimitReader(r, 64<<20)) }()
	err := <-done
	if !errors.Is(err, context.Canceled) || r.reads > 1 {
		t.Fatalf("err=%v reads=%d, want Canceled after 1 read", err, r.reads)
	}
}
