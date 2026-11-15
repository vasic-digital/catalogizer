package fixture

import (
	"context"
	"database/sql"
	"strings"
)

type Repo struct{ db *sql.DB }

// HasPerm on Repo is DB-backed (a pooled query) ...
func (r *Repo) HasPerm(ctx context.Context, id int) bool {
	var ok bool
	_ = r.db.QueryRowContext(ctx, "SELECT 1 FROM p WHERE id = ?", id).Scan(&ok)
	return ok
}

// Perms is a pure value type with a same-named method (the models.Permissions shape).
type Perms []string

func (p Perms) HasPerm(x string) bool { return len(p) > 0 && p[0] == x }

// ... but inside the loop the receiver is a pure Perms (unqualified type) and a strings.Builder
// (qualified external type); neither may resolve to Repo.HasPerm.
func (r *Repo) PureSameName(ctx context.Context) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM p")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		var perms Perms
		var sb strings.Builder
		if perms.HasPerm("x") {
			ids = append(ids, id)
		}
		_ = sb.String()
	}
	return ids, rows.Err()
}
