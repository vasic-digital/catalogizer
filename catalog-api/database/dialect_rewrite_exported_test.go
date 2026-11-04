package database

import "testing"

// Transactional repository code (Begin -> tx.Exec) must be able to obtain PostgreSQL-ready SQL; the raw *sql.Tx does no rewriting.
func TestDB_Rewrite_PostgresPlaceholders(t *testing.T) {
	pg := &DB{dialect: Dialect{Type: DialectPostgres}}
	if got, want := pg.Rewrite("UPDATE t SET a = NULL WHERE b = ? AND c = ?"), "UPDATE t SET a = NULL WHERE b = $1 AND c = $2"; got != want {
		t.Fatalf("postgres rewrite: got %q want %q", got, want)
	}
	lite := &DB{dialect: Dialect{Type: DialectSQLite}}
	if got := lite.Rewrite("SELECT ?"); got != "SELECT ?" {
		t.Fatalf("sqlite must be untouched, got %q", got)
	}
}
