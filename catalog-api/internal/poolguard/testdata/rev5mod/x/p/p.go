package p

import "context"

// In-tree nested module r5mod/x: T is a concrete struct with a pure Foo. The outer module does not
// build against this directory (its replace points elsewhere), so it must not be used for typing.
type T struct{}

func (T) Foo(ctx context.Context, id int) error { return nil }
