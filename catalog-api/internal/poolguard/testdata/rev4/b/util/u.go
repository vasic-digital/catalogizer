package util

import "context"

type I interface {
	Foo(ctx context.Context, id int) error
}
