package fixture

import (
	"context"

	"catalogizer/internal/poolguard/testdata/negative/pure"
)

// HasQ is DB-backed on Repo ...
func (r *Repo) HasQ(ctx context.Context, id int) bool {
	var ok bool
	_ = r.db.QueryRowContext(ctx, "SELECT 1 FROM p WHERE id = ?", id).Scan(&ok)
	return ok
}

// ... but the receiver below is a qualified in-tree concrete type whose HasQ is pure: not a member.
func (r *Repo) QualifiedPure(ctx context.Context, qp pure.QPerms) ([]int, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT id FROM p")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int{}
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		if qp.HasQ("x") {
			ids = append(ids, id)
		}
	}
	return ids, rows.Err()
}
