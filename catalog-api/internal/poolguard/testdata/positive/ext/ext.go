package ext

import "context"

// Checker is an interface declared in ANOTHER package; implementations live in the fixture package.
type Checker interface {
	Check(ctx context.Context, id int) bool
}
