//go:build !linux

package tag2

import "context"

type T interface {
	Foo(ctx context.Context, id int) error
}
