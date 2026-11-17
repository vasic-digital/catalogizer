package database

// PGTX1-R-F2 / F3: the TxContext transaction (Transaction) must rewrite EVERY statement method exactly once on PostgreSQL. It used
// to embed the raw *sql.Tx and rewrite only Exec/Query/QueryRow, so ExecContext/QueryContext/QueryRowContext/Prepare/PrepareContext
// reached PostgreSQL with raw `?` markers. Real PostgreSQL 15 (internal/testpg; SKIP, never faked, when unavailable).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionEveryStatementMethodIsRewrittenOnPostgres(t *testing.T) {
	db := newTxPG(t)
	ctx := context.Background()
	tc := NewTxContext(db, DefaultTransactionConfig())
	tx, err := tc.Begin(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO txsweep (name, n) VALUES (?, ?)`, "a", 1)
	require.NoError(t, err, "Exec")
	_, err = tx.ExecContext(ctx, `INSERT INTO txsweep (name, n) VALUES (?, ?)`, "b", 2)
	require.NoError(t, err, "ExecContext with ? on a TxContext transaction")

	var n int
	require.NoError(t, tx.QueryRow(`SELECT n FROM txsweep WHERE name = ?`, "a").Scan(&n), "QueryRow")
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT n FROM txsweep WHERE name = ?`, "b").Scan(&n), "QueryRowContext with ?")
	assert.Equal(t, 2, n)

	rows, err := tx.Query(`SELECT name FROM txsweep WHERE n = ?`, 1)
	require.NoError(t, err, "Query")
	require.True(t, rows.Next())
	require.NoError(t, rows.Close())
	rows, err = tx.QueryContext(ctx, `SELECT name FROM txsweep WHERE n = ?`, 2)
	require.NoError(t, err, "QueryContext with ?")
	require.True(t, rows.Next())
	require.NoError(t, rows.Close())

	st, err := tx.Prepare(`UPDATE txsweep SET n = ? WHERE name = ?`)
	require.NoError(t, err, "Prepare with ?")
	_, err = st.Exec(10, "a")
	require.NoError(t, err)
	require.NoError(t, st.Close())
	st, err = tx.PrepareContext(ctx, `UPDATE txsweep SET n = ? WHERE name = ?`)
	require.NoError(t, err, "PrepareContext with ?")
	_, err = st.Exec(20, "b")
	require.NoError(t, err)
	require.NoError(t, st.Close())
	require.NoError(t, tx.Commit())
}

// PGTX1-R-F3: a statement carries at most ONE INSERT OR IGNORE (RewriteInsertOrIgnore rewrites the first occurrence and appends
// one ON CONFLICT DO NOTHING). Running the rewrite twice on a two-occurrence string is NOT idempotent, which is why no tx site
// may rewrite explicitly any more: the transaction does it exactly once. This pins both facts.
func TestRewriteInsertOrIgnoreIsSingleStatementOnly(t *testing.T) {
	db := &DB{dialect: Dialect{Type: DialectPostgres}}
	two := `INSERT OR IGNORE INTO a (x) VALUES (?); INSERT OR IGNORE INTO b (x) VALUES (?)`
	once := db.rewriteQuery(two)
	assert.Contains(t, once, "INSERT OR IGNORE INTO b", "documented limit: only the first occurrence is rewritten")
	assert.NotEqual(t, once, db.rewriteQuery(once), "documented limit: a second pass is not idempotent for two occurrences")
	one := `INSERT OR IGNORE INTO a (x) VALUES (?)`
	assert.Equal(t, db.rewriteQuery(one), db.rewriteQuery(db.rewriteQuery(one)), "single occurrence stays idempotent")
}

// PGTX1-R-F4: SafeRollback/SafeCommit take the small TxFinisher interface, so a *Tx and a *Transaction need no SQLTx() unwrap, and
// a typed nil pointer is treated like nil (as the former *sql.Tx signature did).
func TestSafeFinishersAcceptWrapperTransactions(t *testing.T) {
	db := setupTxTestDB(t)
	ctx := context.Background()

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "fu1-commit", 1)
	require.NoError(t, err)
	require.NoError(t, SafeCommit(tx), "*Tx directly, no SQLTx() unwrap")

	tx2, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx2.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "fu1-rollback", 1)
	require.NoError(t, err)
	SafeRollback(tx2)
	var n int
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tx_cov WHERE name = ?", "fu1-rollback").Scan(&n))
	assert.Equal(t, 0, n, "SafeRollback on *Tx must roll back")

	tc := NewTxContext(db, DefaultTransactionConfig())
	wt, err := tc.Begin(ctx, nil)
	require.NoError(t, err)
	_, err = wt.Exec("INSERT INTO tx_cov (name, counter) VALUES (?, ?)", "fu1-wrapped", 1)
	require.NoError(t, err)
	require.NoError(t, SafeCommit(wt), "*Transaction")
	require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM tx_cov WHERE name = ?", "fu1-wrapped").Scan(&n))
	assert.Equal(t, 1, n)

	var nilTx *Tx
	var nilTransaction *Transaction
	SafeRollback(nilTx)
	SafeRollback(nilTransaction)
	require.Error(t, SafeCommit(nilTx))
	require.Error(t, SafeCommit(nilTransaction))
}
