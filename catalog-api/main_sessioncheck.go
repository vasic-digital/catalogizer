package main

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"catalogizer/database"
	root_middleware "catalogizer/middleware"
)

// sessionQuerier is the slice of *database.DB the session check needs.
type sessionQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) rowScanner
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

type dbSessionQuerier struct{ db *database.DB }

func (q dbSessionQuerier) QueryRowContext(ctx context.Context, query string, args ...interface{}) rowScanner {
	return q.db.QueryRowContext(ctx, query, args...)
}

// sessionCheckTimeout bounds the per-request lookup (one primary-key read).
const sessionCheckTimeout = 3 * time.Second

// newSessionChecker makes server-side session revocation (admin-reset-password,
// logout, deactivation) effective for stateless access JWTs on every /api/v1
// route (CAT-021 round 4 F1). It fails closed: any lookup error, a missing
// row, an inactive or expired session, or a session owned by a different user
// than the token subject rejects the request.
func newSessionChecker(db *database.DB) root_middleware.SessionChecker {
	return sessionCheckerFor(dbSessionQuerier{db})
}

func sessionCheckerFor(q sessionQuerier) root_middleware.SessionChecker {
	return func(ctx context.Context, sessionID, userID string) error {
		sid, err := strconv.Atoi(sessionID)
		if err != nil {
			return errors.New("invalid session id")
		}
		ctx, cancel := context.WithTimeout(ctx, sessionCheckTimeout)
		defer cancel()
		var (
			active  bool
			expires time.Time
			owner   int
		)
		if err := q.QueryRowContext(ctx,
			"SELECT is_active, expires_at, user_id FROM user_sessions WHERE id = ?", sid,
		).Scan(&active, &expires, &owner); err != nil {
			return fmt.Errorf("session lookup: %w", err)
		}
		if !active || expires.Before(time.Now()) {
			return errors.New("session inactive or expired")
		}
		if strconv.Itoa(owner) != userID {
			return errors.New("session owner mismatch")
		}
		return nil
	}
}
