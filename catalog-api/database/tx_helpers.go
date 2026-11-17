package database

import (
	"context"
	"database/sql"
)

// TxExecQuerier is what TxInsertReturningID needs from a transaction; both *Tx and a raw *sql.Tx satisfy it. The query is
// rewritten by TxInsertReturningID itself, so passing a raw *sql.Tx is safe here (a *Tx rewrites again, idempotently).
type TxExecQuerier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// TxInsertReturningID executes an INSERT inside a transaction and returns the
// new row's ID. It mirrors InsertReturningID but operates on a *sql.Tx.
// For PostgreSQL it appends "RETURNING id" and uses QueryRow; for SQLite it
// uses Exec + LastInsertId.
func (db *DB) TxInsertReturningID(ctx context.Context, tx TxExecQuerier, query string, args ...interface{}) (int64, error) {
	query = db.rewriteQuery(query)

	if db.dialect.IsPostgres() {
		query += " RETURNING id"
		var id int64
		err := tx.QueryRowContext(ctx, query, args...).Scan(&id)
		if err != nil {
			return 0, err
		}
		return id, nil
	}

	// SQLite path
	result, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}
