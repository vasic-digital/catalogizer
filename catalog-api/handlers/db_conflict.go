package handlers

import (
	"errors"
	"strings"

	"github.com/lib/pq"
)

// isUniqueViolation reports whether err is a unique-constraint violation on either dialect: PostgreSQL SQLSTATE
// 23505 (also when wrapped), or the portable texts SQLite ("UNIQUE constraint failed: ...") and PostgreSQL
// ("duplicate key value violates unique constraint ...") produce. ADMD1RR-md: the one shared classifier for every
// handler that maps a unique violation to 409 (a SQLite-only text match answered 500 on PostgreSQL).
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		return true
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") ||
		strings.Contains(msg, "duplicate key value") ||
		strings.Contains(msg, "violates unique constraint")
}
