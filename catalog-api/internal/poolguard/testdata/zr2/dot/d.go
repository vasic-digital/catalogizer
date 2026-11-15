package dot

import (
	"context"
	"database/sql"
	. "fmt"
)

type Repo struct{ db *sql.DB }

func (r *Repo) String() string {
	_, _ = r.db.ExecContext(context.Background(), "UPDATE s")
	return ""
}

type Deep struct{}

func (Deep) String() string { return "pure" }

type Mid struct{ Deep }
type I interface{ Stringer }
type O struct {
	I
	Mid
}

var _ = Sprint

// N12 dot-imported unresolved interface embedded.
func N12DotImport(ctx context.Context, db *sql.DB, o O) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = o.String()
	}
}
