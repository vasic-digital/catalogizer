package fixture

import (
	"context"
	"database/sql"
)

type Store struct{ db *sql.DB }

// HasPerm is the DB-backed method: a pooled query.
func (s *Store) HasPerm(ctx context.Context, id int) bool {
	var ok bool
	_ = s.db.QueryRowContext(ctx, "SELECT 1 FROM p WHERE id = ?", id).Scan(&ok)
	return ok
}

// Real member through a typed local: st is a *Store, so st.HasPerm IS the pooled method.
func (s *Store) TypedLocalNested(ctx context.Context) error {
	st := &Store{db: s.db}
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM p")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		st.HasPerm(ctx, id)
	}
	return nil
}
