package dup

import "context"

// ... the second an interface that may hold a same-package pooled implementation.
type T interface {
	Foo(ctx context.Context, id int) error
}
