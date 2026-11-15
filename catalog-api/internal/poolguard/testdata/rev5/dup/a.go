package dup

import "context"

// a.go and b.go declare T twice (no build tags): the package does not compile, so its types are not
// trusted. The first declaration is a concrete T with a pure Foo ...
type T struct{}

func (T) Foo(ctx context.Context, id int) error { return nil }
