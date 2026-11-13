package database

import (
	"context"
	"fmt"
)

// createLoginThrottleTable (migration v28, CAT-LOGIN-LOCK-1) creates login_throttle, the persisted state of the login
// back-off (repository.LoginThrottle). One row per (subject, scope):
//
//	subject  "u:<user id>" for an existing account, "n:<sha256 of the lower-cased login name>" for a name that matches no
//	         account (so an unknown name is throttled exactly like a known one and the answer does not reveal which exist)
//	scope    "account" (all clients together) or "ip:<client address>" (one client address of that subject)
//	failures consecutive charged attempts; blocked_until_ms back-off end; last_failure_ms / last_success_ms; version (CAS)
//
// All times are unix milliseconds in BIGINT columns, so SQLite and PostgreSQL store and compare them identically. The state is a
// separate table, not users.is_locked / locked_until: those columns are the administrative suspension and revoke the user's
// sessions, which a guessing attacker must never be able to trigger. The migration is idempotent (IF NOT EXISTS).
func (db *DB) createLoginThrottleTable(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS login_throttle (
			subject TEXT NOT NULL,
			scope TEXT NOT NULL,
			failures INTEGER NOT NULL DEFAULT 0,
			blocked_until_ms BIGINT NOT NULL DEFAULT 0,
			last_failure_ms BIGINT NOT NULL DEFAULT 0,
			last_success_ms BIGINT NOT NULL DEFAULT 0,
			version BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (subject, scope)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_login_throttle_last_failure ON login_throttle (last_failure_ms)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("create login_throttle: %w", err)
		}
	}
	return nil
}
