//go:build !linux

package tagged

import "context"

type T interface {
	Foo(ctx context.Context, id int) error
}
