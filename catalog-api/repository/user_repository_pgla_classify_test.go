package repository

// PGLA-M2: the n==0 classification in LockAccount ("the guard held the write back" vs "already
// suspended / not an admin / no such user") has negative cases. The real LockAccount runs against
// sqlmock-backed SQL (unit class); the discriminating input is the re-read row.
import (
	"database/sql/driver"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// nearNow matches a time.Time bound argument within `within` of the moment of the call (FUTA-F2): the
// "not locked at this moment" bound of the other-active-admin clause must be the real current time.
// The bound is taken a few microseconds before the match runs (sqlmock, no I/O), so 1 s is ample slack
// even on a loaded host while still failing a clause shifted by 2 s or more (FUTB-F1).
type nearNow struct {
	within time.Duration
}

func (n nearNow) Match(v driver.Value) bool {
	tv, ok := v.(time.Time)
	if !ok {
		return false
	}
	d := time.Since(tv)
	if d < 0 {
		d = -d
	}
	return d <= n.within
}

func TestLockAccount_ZeroRowsClassification(t *testing.T) {
	now := time.Now()
	until := now.Add(time.Hour)
	cases := []struct {
		name     string
		reread   func() *sqlmock.Rows
		wantLast bool
	}{
		{"unlocked administrator is held back by the guard (positive control)",
			func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"role_id", "is_locked", "locked_until"}).AddRow(1, false, nil)
			}, true},
		{"timed-locked administrator is still held back (positive control)",
			func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"role_id", "is_locked", "locked_until"}).AddRow(1, true, now.Add(time.Minute))
			}, true},
		{"already suspended administrator is NOT ErrLastAdmin",
			func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"role_id", "is_locked", "locked_until"}).AddRow(1, true, nil)
			}, false},
		{"non-administrator is NOT ErrLastAdmin",
			func() *sqlmock.Rows {
				return sqlmock.NewRows([]string{"role_id", "is_locked", "locked_until"}).AddRow(2, false, nil)
			}, false},
		{"missing user is NOT ErrLastAdmin",
			func() *sqlmock.Rows { return sqlmock.NewRows([]string{"role_id", "is_locked", "locked_until"}) }, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo, mock := newMockUserRepo(t)
			mock.ExpectBegin()
			mock.ExpectExec("UPDATE roles SET updated_at = updated_at").WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("FROM roles").WillReturnRows(sqlmock.NewRows([]string{"id", "permissions"}).
				AddRow(1, `["system.admin"]`))
			// pin the guarded UPDATE (TIERPGLA-F2): the NOT IN admin-role clause, the last-admin sub-select and every bound argument
			// (lock time, target id, true, admin role id, then the other-active-admin clause args) so removing or reshaping the guard fails.
			mock.ExpectExec(`UPDATE users SET is_locked = 1, locked_until = \? WHERE id = \? AND NOT \(COALESCE\(is_locked, '0'\) = \? AND locked_until IS NULL\) AND \(role_id NOT IN \(\?\) OR \(SELECT COUNT\(\*\) FROM users o WHERE o\.id <> \? AND o\.is_active = \? .* AND o\.role_id IN \(\?\)\) >= 1\)`).
				WithArgs(until, 7, true, 1, 7, true, false, nearNow{within: time.Second}, 1).WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectQuery("SELECT role_id").WithArgs(7).WillReturnRows(c.reread())
			mock.ExpectCommit()
			if !c.wantLast {
				mock.ExpectExec("UPDATE user_sessions SET is_active = 0").WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 0))
			}
			err := repo.LockAccount(7, until)
			if got := errors.Is(err, ErrLastAdmin); got != c.wantLast {
				t.Fatalf("ErrLastAdmin=%v want %v (err=%v)", got, c.wantLast, err)
			}
			if !c.wantLast && err != nil {
				t.Fatalf("unexpected error %v", err)
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
