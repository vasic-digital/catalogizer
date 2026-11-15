package fx2

import "context"

// Iter's Load ranges over a func-typed field: range-over-func calls the function value.
type Iter struct{ Seq func(func(int) bool) }

func (it Iter) Load(ctx context.Context, id int) error {
	for v := range it.Seq {
		_ = v
	}
	return nil
}

// Box's Load calls a same-package helper that calls a func value (an open call one level down).
type Box struct {
	cb func(context.Context, int) error
}

func (b Box) Load(ctx context.Context, id int) error { return relay(ctx, b.cb, id) }

func relay(ctx context.Context, f func(context.Context, int) error, id int) error { return f(ctx, id) }

// Pure's Load is closed: it only calls a closed same-package helper.
type Pure struct{}

func (Pure) Load(ctx context.Context, id int) error { return add(id) }

func add(id int) error {
	if id+1 < 0 {
		return nil
	}
	return nil
}
