package raw

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

// M1: the cgo file below imports "C" as a raw string; with CGO_ENABLED=0 it is dropped, so w.Load() is the pooled Base.Load.
func M1RawStringCgoImport(ctx context.Context, db *sql.DB, w W) {
	rows, _ := db.QueryContext(ctx, "SELECT")
	for rows.Next() {
		_ = w.Load()
	}
}
