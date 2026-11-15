package ifc

import "context"

type T interface {
	Foo(ctx context.Context, id int) error
}
