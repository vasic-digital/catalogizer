package grp

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

// M2: the cgo file sorts LAST (z.go), after this plain file; the cgo rule must apply to every file, not only the first.
func M2CgoFileSortsLast(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
