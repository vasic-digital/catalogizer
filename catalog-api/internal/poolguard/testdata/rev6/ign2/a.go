package ign2

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

// R51: shadow.go carries //go:build ignore, so the real w.Load() is the pooled
// Base.Load; shadow.go declares a pure W.Load that no build sees.
func R51IgnoreTaggedFile(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
