package emb2

import "io"

// Inner has a pure Close two embedding levels below S.
type Inner struct{}

func (Inner) Close() error { return nil }

type Mid struct{ Inner }

// S embeds io.Closer at depth one: in a real build S.Close is the interface method (dynamic) and
// shadows Inner.Close. io cannot be resolved by the scanner, so the embedding is invalid there.
type S struct {
	io.Closer
	Mid
}
