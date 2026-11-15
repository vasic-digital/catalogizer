//go:build linux

package tag3

import "context"

// T is a concrete struct with a pure Foo on linux ...
type T struct{}

func (T) Foo(ctx context.Context, id int) error { return nil }
