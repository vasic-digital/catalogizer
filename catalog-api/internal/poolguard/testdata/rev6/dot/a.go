package dot

import (
	"context"
	"database/sql"
)

type Base struct{ db *sql.DB }

func (b *Base) Load() error {
	_, err := b.db.ExecContext(context.Background(), "UPDATE")
	return err
}

type W struct{ *Base }

// R50: the go tool ignores files whose name starts with ".", so the real w.Load() is the pooled
// Base.Load; .shadow.go declares a pure W.Load that no build sees.
func R50DotFileIgnored(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
