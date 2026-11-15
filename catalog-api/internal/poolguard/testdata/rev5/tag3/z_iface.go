//go:build !linux

package tag3

import "context"

// ... and an interface elsewhere (it may hold a same-package pooled implementation).
type T interface {
	Foo(ctx context.Context, id int) error
}
