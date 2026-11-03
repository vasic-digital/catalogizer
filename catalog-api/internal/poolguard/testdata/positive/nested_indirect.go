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

func (s *Svc) wrapper(ctx context.Context, id int) { _, _ = s.tags(ctx, id) }

// Indirect member: the pooled query sits two helpers deep (the pre-fix SearchFiles shape).
func (s *Svc) List(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, "SELECT id FROM p")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		s.wrapper(ctx, id)
	}
	return nil
}

// Exec-per-row member.
func (s *Svc) ExecPerRow() error {
	rows, err := s.db.Query("SELECT id FROM p")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		_, _ = s.db.Exec("UPDATE p SET seen = 1 WHERE id = ?", id)
	}
	return nil
}
