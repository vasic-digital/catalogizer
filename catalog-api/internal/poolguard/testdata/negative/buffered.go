package fixture

import (
	"context"
	"database/sql"
)

type Svc struct{ db *sql.DB }

func (s *Svc) tags(ctx context.Context, id int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT tag FROM t WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return nil, nil
}

// Fixed shape: buffer the page, close the cursor, then run the per-row queries.
func (s *Svc) List(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM p")
	if err != nil {
		return err
	}
	defer rows.Close()
	var ids []int
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		_, _ = s.tags(ctx, id)
		_, _ = s.db.Exec("UPDATE p SET seen = 1 WHERE id = ?", id)
	}
	return nil
}

// A loop that is not a cursor loop must not be flagged.
func (s *Svc) PlainLoop(ids []int) {
	for _, id := range ids {
		_, _ = s.db.Exec("UPDATE p SET seen = 1 WHERE id = ?", id)
	}
}
