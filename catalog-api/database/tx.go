package database

import (
	"context"
	"database/sql"
)

// Tx is a transaction whose statements receive the same dialect rewrite as the DB wrappers (placeholders `?` -> `$n`,
// INSERT OR IGNORE / OR REPLACE, boolean literals on PostgreSQL). DB.Begin and DB.BeginTx return it, so a statement
// run on a transaction can no longer bypass the rewrite (CAT-PG-TX-SWEEP-1: a raw *sql.Tx sent `?` to PostgreSQL).
//
// The embedded *sql.Tx supplies Commit, Rollback, Stmt, StmtContext and the rest. Every statement-running method of
// *sql.Tx is shadowed below; SQLTx returns the underlying *sql.Tx for the rare caller that needs it and accepts that its
// statements are NOT rewritten.
type Tx struct {
	*sql.Tx
	db *DB
}

// Begin starts a transaction whose statements are dialect-rewritten.
func (db *DB) Begin() (*Tx, error) {
	return db.BeginTx(context.Background(), nil)
}

// BeginTx starts a transaction whose statements are dialect-rewritten.
func (db *DB) BeginTx(ctx context.Context, opts *sql.TxOptions) (*Tx, error) {
	tx, err := db.DB.BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &Tx{Tx: tx, db: db}, nil
}

// SQLTx returns the underlying *sql.Tx; statements run on it bypass the dialect rewrite.
func (tx *Tx) SQLTx() *sql.Tx { return tx.Tx }

func (tx *Tx) Exec(query string, args ...interface{}) (sql.Result, error) {
	return tx.Tx.ExecContext(context.Background(), tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return tx.Tx.ExecContext(ctx, tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) Query(query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Tx.QueryContext(context.Background(), tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	return tx.Tx.QueryContext(ctx, tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) QueryRow(query string, args ...interface{}) *sql.Row {
	return tx.Tx.QueryRowContext(context.Background(), tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return tx.Tx.QueryRowContext(ctx, tx.db.rewriteQuery(query), args...)
}

func (tx *Tx) Prepare(query string) (*sql.Stmt, error) {
	return tx.Tx.PrepareContext(context.Background(), tx.db.rewriteQuery(query))
}

func (tx *Tx) PrepareContext(ctx context.Context, query string) (*sql.Stmt, error) {
	return tx.Tx.PrepareContext(ctx, tx.db.rewriteQuery(query))
}
