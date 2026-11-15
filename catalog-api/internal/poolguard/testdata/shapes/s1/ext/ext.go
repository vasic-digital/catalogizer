package ext

import "context"

// Checker is an interface declared in ANOTHER package; implementations may live in pgshapes.
type Checker interface {
	Check(ctx context.Context, id int) bool
}

type Pure struct{}

func (Pure) Check(ctx context.Context, id int) bool { return id > 0 }
