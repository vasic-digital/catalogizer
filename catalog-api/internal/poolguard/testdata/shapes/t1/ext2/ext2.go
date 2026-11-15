package ext2

import "context"

type Checker interface {
	Check(ctx context.Context, id int) bool
}

// alias to an interface: TypeSpec.Type is an Ident, not an InterfaceType.
type CheckerAlias = Checker

// defined type whose underlying type is an interface.
type CheckerDef Checker

// struct embedding an interface: methods promoted from the dynamic value.
type Wrapper struct{ Checker }

// generic wrapper forwarding to a type-parameter value.
type Box[T Checker] struct{ V T }

func (b *Box[T]) Check(ctx context.Context, id int) bool { return b.V.Check(ctx, id) }

// Handler collides with a tree struct below and with net/http.Handler.
type Handler struct{}
