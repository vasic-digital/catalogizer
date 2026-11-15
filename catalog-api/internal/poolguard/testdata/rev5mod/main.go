package r5mod

import (
	"context"
	"database/sql"

	"r5mod/x/p"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Foo(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE x")
	return err
}

// R43: the only in-tree directory with import path r5mod/x/p belongs to ANOTHER module (nested go.mod);
// the scanner must not resolve the import to it. Stays flagged.
func R43OtherModuleSameImportPath(ctx context.Context, db *sql.DB, x p.T) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	defer rows.Close()
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
