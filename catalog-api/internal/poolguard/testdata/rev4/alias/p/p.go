package p

import "context"

type T struct{}

func (T) Foo(ctx context.Context, id int) error { return nil }
