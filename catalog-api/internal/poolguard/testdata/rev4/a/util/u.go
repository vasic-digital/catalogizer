package util

import "context"

type I struct{}

func (I) Foo(ctx context.Context, id int) error { return nil }
