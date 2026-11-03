package main

import (
	"fmt"

	"catalogizer/database"
	"catalogizer/internal/metrics"
	root_services "catalogizer/services"
)

// Candidate rows are those with a 64-character hash (bcrypt rows are 60
// characters, v2 rows start with "$cz1$"); the exact shape (64 LOWERCASE hex,
// which already excludes any '$'-prefixed value) is then checked in Go with the
// same function CheckPassword uses, so the gauge counts exactly the rows the
// verifier would accept as legacy. The length filter is portable SQL
// (SQLite and Postgres); hex matching is not.
const legacyCandidateSQL = "SELECT password_hash FROM users WHERE LENGTH(password_hash) = 64"

// reportLegacyPasswordHashes counts the remaining legacy-hash rows, publishes the
// count as the catalogizer_auth_legacy_password_hashes gauge and, when it is
// non-zero, logs it once through warnf. It changes no behaviour: legacy rows keep
// verifying and keep being upgraded at login. It exists so the sunset of the
// legacy verify path can be decided on a measured number.
func reportLegacyPasswordHashes(db *database.DB, warnf func(format string, args ...any)) (int, error) {
	rows, err := db.Query(legacyCandidateSQL)
	if err != nil {
		return 0, fmt.Errorf("count legacy password hashes: %w", err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var h string
		if err := rows.Scan(&h); err != nil {
			return 0, fmt.Errorf("count legacy password hashes: %w", err)
		}
		if root_services.IsLegacySHA256Hex(h) {
			n++
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("count legacy password hashes: %w", err)
	}
	metrics.LegacyPasswordHashes.Set(float64(n))
	if n > 0 {
		warnf("%d user(s) still have a legacy SHA-256 password hash; each is upgraded to bcrypt at that user's next successful login (see docs/security/LEGACY_PASSWORD_HASH_SUNSET.md)", n)
	}
	return n, nil
}
