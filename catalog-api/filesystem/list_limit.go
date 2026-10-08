package filesystem

import "context"

// list_limit.go - WF22 fix round 2 (R4/W1): a way for the scanner to tell a client how many entries one listing may return.
//
// The scanner bounds the entries of a scan and of one directory. Checking the bound only AFTER a listing came back means the whole
// directory was already fetched and held in memory. The limit travels in the context because the listing goes through decorators
// (read-only, host budget, retry) that pass the context on but know nothing about a limit.
//
// A client that honours the limit returns at most that many entries (it MAY stop reading the server answer early); a client that does
// not honour it is simply not bounded by it, and the scanner's own check after the listing still applies. The limit is a bound on what
// the client has to produce, not a filter: the scanner asks for (bound + 1) entries so that "more than the bound" stays detectable.

type listLimitKey struct{}

// WithListLimit returns a context that asks ListDirectory to return at most n entries. n < 1 returns ctx unchanged.
func WithListLimit(ctx context.Context, n int) context.Context {
	if n < 1 {
		return ctx
	}
	return context.WithValue(ctx, listLimitKey{}, n)
}

// ListLimit reports the entry limit carried by ctx.
func ListLimit(ctx context.Context) (int, bool) {
	n, ok := ctx.Value(listLimitKey{}).(int)
	return n, ok && n >= 1
}
