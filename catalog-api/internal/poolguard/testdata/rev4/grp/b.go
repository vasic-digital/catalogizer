package grp

import "context"

type (
	B struct{ N int }
	I interface {
		Foo(ctx context.Context, id int) error
	}
)
