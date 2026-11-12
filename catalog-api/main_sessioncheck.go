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

// sessionCheckTimeout bounds the per-request lookup (one JOIN on primary keys).
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
			active      bool
			expires     time.Time
			owner       int
			userActive  bool
			userLocked  bool
			lockedUntil *time.Time
		)
		// One JOIN, one round-trip: the owner's account state is read together
		// with the session row, so a deactivation or suspension closes every
		// session race (e.g. a login that created its session after the
		// admin's revokes) by construction.
		if err := q.QueryRowContext(ctx,
			"SELECT s.is_active, s.expires_at, s.user_id, u.is_active, COALESCE(u.is_locked, '0'), u.locked_until "+
				"FROM user_sessions s JOIN users u ON u.id = s.user_id WHERE s.id = ?", sid,
		).Scan(&active, &expires, &owner, &userActive, &userLocked, &lockedUntil); err != nil {
			return fmt.Errorf("session lookup: %w", err)
		}
		if !active || expires.Before(time.Now()) {
			return errors.New("session inactive or expired")
		}
		if strconv.Itoa(owner) != userID {
			return errors.New("session owner mismatch")
		}
		if !userActive {
			return errors.New("account disabled")
		}
		// Effective lock, same rule as models.User.IsAccountLocked: permanent when
		// locked_until is NULL, otherwise only until it passes.
		if userLocked && (lockedUntil == nil || lockedUntil.After(time.Now())) {
			return errors.New("account locked")
		}
		return nil
	}
}
