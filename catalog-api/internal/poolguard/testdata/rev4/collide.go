package rev4

import (
	"context"
	"database/sql"

	u "catalogizer/internal/poolguard/testdata/rev4/conc"
)

// R11: in THIS file u = conc (struct); in main.go u = ifc (interface). Helper in this file.
func pureHelper(ctx context.Context, x u.T) error { return x.Foo(ctx, 1) }

// R11b: loop in this file on u.T (concrete here) - sanctioned-class removal expected.
func R11bCollideConcrete(ctx context.Context, db *sql.DB, x u.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
