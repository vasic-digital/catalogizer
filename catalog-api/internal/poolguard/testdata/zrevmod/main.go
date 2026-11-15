package zm

import (
	"context"
	"database/sql"

	"x/p"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Load() error {
	_, err := r.db.ExecContext(context.Background(), "UPDATE")
	return err
}

// Z04 (M17 killer): x/p is not a package of module zm; the nested stale copy must not be trusted.
func Z04NestedStaleCopy(ctx context.Context, db *sql.DB, t p.T) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = t.Load()
	}
}
