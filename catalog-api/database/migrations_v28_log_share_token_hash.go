package database

import (
	"context"
	"fmt"
)

// invalidateLegacyLogShareTokens (migration v28, CAT-LOGSHARE-TOK-1) retires every log share whose token is still stored
// in its legacy plaintext form. Legacy tokens were `log_share_<unix>_<nanos>` (derived from the clock, so guessable) and
// are kept at rest in clear text; new tokens are 256-bit random values of which only the SHA-256 is stored (64 lower-case
// hex characters, never starting with `log_share_`). A legacy row is deactivated AND its plaintext is overwritten with an
// unreachable marker, so a database dump no longer carries a usable (or even guessable-pattern) token, and the token can
// never validate again (the service also rejects the legacy shape before the lookup). The creator re-shares the collection.
//
// The PostgreSQL ESCAPE '\' relies on standard_conforming_strings=on (the default since 9.1).
//
// Idempotent: a re-run matches no row (the marker does not start with `log_share_`). The same statement runs on SQLite and
// PostgreSQL: `||` concatenation, `LIKE ... ESCAPE`, an integer 0 for is_active (PostgreSQL keeps is_active BOOLEAN, so the
// literal differs per dialect).
func (db *DB) invalidateLegacyLogShareTokens(ctx context.Context) error {
	q := legacyLogShareTokenSQL(db.dialect.IsPostgres())
	if _, err := db.ExecContext(ctx, q); err != nil {
		return fmt.Errorf("invalidate legacy log share tokens: %w", err)
	}
	return nil
}

// legacyLogShareTokenSQL is the retirement statement for one dialect (PostgreSQL keeps is_active BOOLEAN).
func legacyLogShareTokenSQL(postgres bool) string {
	inactive := "0"
	if postgres {
		inactive = "FALSE"
	}
	return fmt.Sprintf(
		`UPDATE log_shares SET share_token = 'revoked-legacy-' || CAST(id AS TEXT), is_active = %s WHERE share_token LIKE 'log\_share\_%%' ESCAPE '\'`,
		inactive)
}
