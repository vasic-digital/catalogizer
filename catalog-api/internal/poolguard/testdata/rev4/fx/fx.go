package fx

import "context"

// Hooks is a plain concrete struct whose members are func-typed FIELDS (dependency injection).
type Hooks struct {
	Save     func(ctx context.Context, id int) error
	LoadTags func(ctx context.Context, id int) []string
}

// Fn is a named func type with a method that calls the func value.
type Fn func(ctx context.Context, id int) error

func (f Fn) Do(ctx context.Context, id int) error { return f(ctx, id) }

// L is a slice type with a method.
type L []int

func (l L) Load(ctx context.Context, id int) error { return nil }
