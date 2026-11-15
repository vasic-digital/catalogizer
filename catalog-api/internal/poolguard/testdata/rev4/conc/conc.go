package conc

import "context"

type T struct{ N int }

func (T) Foo(ctx context.Context, id int) error { return nil }
