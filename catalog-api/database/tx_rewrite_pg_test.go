package database

// CAT-PG-TX-SWEEP-1: statements run on a transaction obtained from DB.Begin/BeginTx must receive the same dialect rewrite as
// the DB wrappers. Against a REAL PostgreSQL 15 (rootless podman via internal/testpg; SKIP, never faked, when unavailable)
// a raw `?` placeholder is a syntax error, so every statement method of the transaction is exercised with `?`.

import (
	"context"
	"database/sql"
	"testing"

	"catalogizer/internal/testpg"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTxPG(t *testing.T) *DB {
	t.Helper()
	dsn, _ := testpg.Start(t)
	raw, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	raw.SetMaxOpenConns(4)
	t.Cleanup(func() { _ = raw.Close() })
	db := WrapDB(raw, DialectPostgres)
	_, err = db.Exec(`CREATE TABLE txsweep (id SERIAL PRIMARY KEY, name TEXT UNIQUE, is_active BOOLEAN NOT NULL DEFAULT TRUE, n INTEGER NOT NULL DEFAULT 0)`)
	require.NoError(t, err)
	return db
}

func TestTxStatementsAreDialectRewrittenOnPostgres(t *testing.T) {
	db := newTxPG(t)
	ctx := context.Background()

	for _, begin := range []string{"Begin", "BeginTx"} {
		begin := begin
		t.Run(begin, func(t *testing.T) {
			var (
				tx interface {
					Commit() error
					Rollback() error
				}
				err error
			)
			// The same statement set runs against whatever type the wrapper's Begin/BeginTx returns.
			type stmts interface {
				Exec(string, ...interface{}) (sql.Result, error)
				ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
				Query(string, ...interface{}) (*sql.Rows, error)
				QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
				QueryRow(string, ...interface{}) *sql.Row
				QueryRowContext(context.Context, string, ...interface{}) *sql.Row
				Prepare(string) (*sql.Stmt, error)
				PrepareContext(context.Context, string) (*sql.Stmt, error)
			}
			var s stmts
			if begin == "Begin" {
				x, e := db.Begin()
				tx, s, err = x, x, e
			} else {
				x, e := db.BeginTx(ctx, nil)
				tx, s, err = x, x, e
			}
			require.NoError(t, err)
			defer tx.Rollback()

			_, err = s.Exec(`INSERT INTO txsweep (name, n) VALUES (?, ?)`, begin+"-exec", 1)
			require.NoError(t, err, "Exec with ? on a PostgreSQL tx")
			_, err = s.ExecContext(ctx, `INSERT INTO txsweep (name, n) VALUES (?, ?)`, begin+"-execctx", 2)
			require.NoError(t, err, "ExecContext with ? on a PostgreSQL tx")

			var n int
			require.NoError(t, s.QueryRow(`SELECT n FROM txsweep WHERE name = ?`, begin+"-exec").Scan(&n), "QueryRow with ?")
			assert.Equal(t, 1, n)
			require.NoError(t, s.QueryRowContext(ctx, `SELECT n FROM txsweep WHERE name = ?`, begin+"-execctx").Scan(&n), "QueryRowContext with ?")
			assert.Equal(t, 2, n)

			rows, err := s.Query(`SELECT name FROM txsweep WHERE n = ?`, 1)
			require.NoError(t, err, "Query with ?")
			require.True(t, rows.Next())
			require.NoError(t, rows.Close())
			rows, err = s.QueryContext(ctx, `SELECT name FROM txsweep WHERE n = ?`, 2)
			require.NoError(t, err, "QueryContext with ?")
			require.True(t, rows.Next())
			require.NoError(t, rows.Close())

			st, err := s.Prepare(`UPDATE txsweep SET n = ? WHERE name = ?`)
			require.NoError(t, err, "Prepare with ?")
			_, err = st.Exec(10, begin+"-exec")
			require.NoError(t, err)
			require.NoError(t, st.Close())
			st, err = s.PrepareContext(ctx, `UPDATE txsweep SET n = ? WHERE name = ?`)
			require.NoError(t, err, "PrepareContext with ?")
			_, err = st.Exec(20, begin+"-execctx")
			require.NoError(t, err)
			require.NoError(t, st.Close())

			// sqlite-flavoured constructs the wrapper rewrites for PostgreSQL: INSERT OR IGNORE and integer boolean literals.
			_, err = s.Exec(`INSERT OR IGNORE INTO txsweep (name, n) VALUES (?, ?)`, begin+"-exec", 99)
			require.NoError(t, err, "INSERT OR IGNORE on a PostgreSQL tx")
			_, err = s.Exec(`UPDATE txsweep SET n = n + 1 WHERE is_active = 1 AND name = ?`, begin+"-exec")
			require.NoError(t, err, "integer boolean literal on a PostgreSQL tx")
			require.NoError(t, s.QueryRow(`SELECT n FROM txsweep WHERE name = ?`, begin+"-exec").Scan(&n))
			assert.Equal(t, 11, n, "INSERT OR IGNORE must not overwrite, the boolean-literal update must match")

			require.NoError(t, tx.Commit())
			require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM txsweep WHERE name LIKE ?`, begin+"-%").Scan(&n))
			assert.Equal(t, 2, n)
		})
	}
}
