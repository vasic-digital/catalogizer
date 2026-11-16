package esc

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

// M3: the cgo file below imports "C" as an escaped interpreted literal (\x43); with CGO_ENABLED=0 it is dropped, so w.Load() is the pooled Base.Load.
func M3EscapedCgoImport(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
