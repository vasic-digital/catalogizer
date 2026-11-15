package c

import (
	"context"
	"database/sql"

	"catalogizer/internal/poolguard/testdata/rev4/alias/p"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Foo(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE x")
	return err
}

// R02: p.T here is nm/p.T (an interface, may hold *Repo); the outer module has a struct alias/p.T.
func R02NestedModuleIface(ctx context.Context, db *sql.DB, x p.T) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	defer rows.Close()
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
