package win

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
