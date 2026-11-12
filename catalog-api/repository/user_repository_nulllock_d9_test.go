package repository

// The last-admin guard must treat a NULL is_locked as unlocked (M6, CAT-D9-SUSPEND).
// Real SQL text is exercised through the clause against real sqlite in the
// existing SetSuspended tests; here the clause text is pinned.
import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestOtherActiveAdminsClause_NullLockIsUnlocked(t *testing.T) {
	c, _ := otherActiveAdminsClause(1, []int{1})
	if !strings.Contains(c, "o.is_locked IS NULL") {
		t.Fatalf("clause ignores NULL is_locked: %s", c)
	}
}

// M2: the lock is written BEFORE sessions are revoked (so no session can be
// issued between the two), and a failed lock write revokes nothing. sqlmock
// enforces statement order (MatchExpectationsInOrder is its default).
func TestLockAccount_WritesLockBeforeRevokingSessions(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	until := time.Now().Add(time.Hour)
	mock.ExpectExec("UPDATE users SET is_locked = 1").WithArgs(until, 7, true).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE user_sessions SET is_active = 0").WithArgs(7).WillReturnResult(sqlmock.NewResult(0, 1))
	if err := repo.LockAccount(7, until); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLockAccount_FailedLockWriteRevokesNothing(t *testing.T) {
	repo, mock := newMockUserRepo(t)
	until := time.Now().Add(time.Hour)
	mock.ExpectExec("UPDATE users SET is_locked = 1").WithArgs(until, 7, true).WillReturnError(errors.New("boom"))
	if err := repo.LockAccount(7, until); err == nil {
		t.Fatal("expected error")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
