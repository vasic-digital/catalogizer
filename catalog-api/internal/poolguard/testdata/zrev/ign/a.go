package ign

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

// Z02: the go tool ignores files whose name starts with "_", so the real w.Load() is the pooled
// Base.Load; the scanner type-checks _shadow.go and sees a pure W.Load.
func Z02UnderscoreFileIgnored(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
