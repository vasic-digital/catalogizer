package rev4

import (
	"context"
	"database/sql"

	"catalogizer/internal/poolguard/testdata/rev4/defq"
	"catalogizer/internal/poolguard/testdata/rev4/tag2"
)

// R32: same type name in two build-tagged files (interface sorts first, struct last).
func R32TaggedIfaceFirst(ctx context.Context, db *sql.DB, x tag2.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

// R33: defined type over a qualified interface.
func R33DefinedOverQualifiedIface(ctx context.Context, db *sql.DB, x defq.D) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

type Tp struct{}

func (Tp) Foo(ctx context.Context, id int) error { return nil }

type GBox[Tp interface {
	Foo(context.Context, int) error
}] struct{}

// R34: receiver type parameter named like a package-level type with a pure Foo.
func (b *GBox[Tp]) R34RecvTypeParamShadow(ctx context.Context, db *sql.DB, x Tp) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
