package revmod

import (
	"context"
	"database/sql"

	"revmod/x/p"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Foo(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE x")
	return err
}

// R03: with the replace directive, revmod/x/p is realx/p (interface); in-tree x/p is a struct.
func R03ReplaceIface(ctx context.Context, db *sql.DB, x p.T) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	defer rows.Close()
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
