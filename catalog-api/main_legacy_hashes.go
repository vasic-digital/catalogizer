package main

import (
	"fmt"

	"catalogizer/database"
	"catalogizer/internal/metrics"
)

// legacyHashWhere selects rows still in the legacy hex SHA-256 shape written by
// the old POST /users: exactly 64 characters and no '$' prefix. bcrypt (v1) rows
// are 60 characters and v2 rows start with "$cz1$", so neither matches.
const legacyHashWhere = "LENGTH(password_hash) = 64 AND password_hash NOT LIKE '$%'"

// reportLegacyPasswordHashes counts the remaining legacy-hash rows, publishes the
// count as the catalogizer_auth_legacy_password_hashes gauge and, when it is
// non-zero, logs it once through warnf. It changes no behaviour: legacy rows keep
// verifying and keep being upgraded at login. It exists so the sunset of the
// legacy verify path can be decided on a measured number.
func reportLegacyPasswordHashes(db *database.DB, warnf func(format string, args ...any)) (int, error) {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM users WHERE " + legacyHashWhere).Scan(&n); err != nil {
		return 0, fmt.Errorf("count legacy password hashes: %w", err)
	}
	metrics.LegacyPasswordHashes.Set(float64(n))
	if n > 0 {
		warnf("%d user(s) still have a legacy SHA-256 password hash; each is upgraded to bcrypt at that user's next successful login (see docs/security/LEGACY_PASSWORD_HASH_SUNSET.md)", n)
	}
	return n, nil
}
