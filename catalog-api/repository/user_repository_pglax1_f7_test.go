package repository

// PGLAX1-F7 (SQLite): the user-write guards (UpdateGuarded, LockAccount, SetSuspended, Delete) read the
// administrator-role set outside any lock or transaction and then ran one guarded statement. A concurrent
// strip of administrator role A (allowed, because role B still has an active member) could commit between that
// read and the write, so the user write then removed B's last member while its guard still counted A's member as
// "another administrator": zero active administrators.
// Real SQLite file database with the production DSN shape (WAL, deferred BEGIN, busy_timeout), real
// UserRepository. The only addition is a driver wrapper that parks the user write at its first statement that
// follows the administrator-set read until the strip has had its chance to run, forcing the read-strip-write
// interleaving deterministically. A guarded implementation holds the database write lock across that point, so
// the strip cannot commit while the user write is parked (the park times out and the write completes first).
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/require"
)

var f7DriverSeq int64

type f7Park struct {
	match   func(q string) bool
	once    sync.Once
	reached chan struct{}
	proceed chan struct{}
	timeout time.Duration
}

func (p *f7Park) maybe(q string) {
	if !p.match(strings.Join(strings.Fields(q), " ")) {
		return
	}
	first := false
	p.once.Do(func() { first = true })
	if !first {
		return
	}
	close(p.reached)
	select {
	case <-p.proceed:
	case <-time.After(p.timeout):
	}
}

type f7Driver struct{ p *f7Park }

func (d f7Driver) Open(dsn string) (driver.Conn, error) {
	c, err := (&sqlite3.SQLiteDriver{}).Open(dsn)
	if err != nil {
		return nil, err
	}
	return &f7Conn{Conn: c, p: d.p}, nil
}

type f7Conn struct {
	driver.Conn
	p *f7Park
}

func (c *f7Conn) Exec(q string, args []driver.Value) (driver.Result, error) {
	c.p.maybe(q)
	return c.Conn.(driver.Execer).Exec(q, args) //nolint:staticcheck // the vendored driver only has the legacy interfaces
}

func (c *f7Conn) Query(q string, args []driver.Value) (driver.Rows, error) {
	c.p.maybe(q)
	return c.Conn.(driver.Queryer).Query(q, args) //nolint:staticcheck
}

// f7Run: roles A and B both grant system.admin, ua in A, ub in B. The user write targets ub (the last member
// of B) and is parked at its guarded statement; meanwhile role A is stripped. Returns the two errors and the
// number of active unlocked administrators afterwards.
func f7Run(t *testing.T, match func(string) bool, write func(repo *UserRepository, ub int) error) (writeErr, stripErr error, admins int) {
	path := filepath.Join(t.TempDir(), "f7.db")
	park := &f7Park{match: match, reached: make(chan struct{}), proceed: make(chan struct{}), timeout: 4 * time.Second}
	drv := fmt.Sprintf("sqlite3_f7_%d", atomic.AddInt64(&f7DriverSeq, 1))
	sql.Register(drv, f7Driver{p: park})
	sqlDB, err := sql.Open(drv, path+"?_busy_timeout=20000&_foreign_keys=1")
	require.NoError(t, err)
	_, err = sqlDB.Exec("PRAGMA journal_mode=WAL")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	_, err = db.Exec(`UPDATE users SET role_id = 2`)
	require.NoError(t, err)
	repo := NewUserRepository(db)
	a := pglaX1Role(t, repo, "roleA", "system.admin")
	b := pglaX1Role(t, repo, "roleB", "system.admin")
	pglaX1User(t, repo, "ua", a, true)
	ub := pglaX1User(t, repo, "ub", b, true)

	writeDone := make(chan error, 1)
	go func() { writeDone <- write(repo, ub) }()
	select {
	case <-park.reached:
	case err := <-writeDone:
		t.Fatalf("user write finished without reaching the parked statement: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("user write never reached the parked statement")
	}
	stripDone := make(chan error, 1)
	go func() {
		stripDone <- repo.UpdateRole(&models.Role{ID: a, Name: "stripped", Permissions: models.Permissions{"media.view"}})
	}()
	select {
	case stripErr = <-stripDone:
		close(park.proceed)
	case <-time.After(1 * time.Second):
		close(park.proceed) // a guarded user write holds the write lock: the strip is blocked behind it
		stripErr = <-stripDone
	}
	writeErr = <-writeDone
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.is_active = 1 AND (u.is_locked IS NULL OR u.is_locked = 0) AND r.permissions LIKE '%system.admin%'`).Scan(&admins))
	t.Logf("user write error: %v; strip error: %v; active unlocked administrators left: %d", writeErr, stripErr, admins)
	return
}

func f7Assert(t *testing.T, writeErr, stripErr error, admins int) {
	require.GreaterOrEqual(t, admins, 1, "last-administrator invariant violated: no active administrator left (write=%v strip=%v)", writeErr, stripErr)
	require.True(t, (writeErr == nil) != (stripErr == nil), "exactly one of the two writes must be refused: write=%v strip=%v", writeErr, stripErr)
	loser := writeErr
	if loser == nil {
		loser = stripErr
	}
	require.ErrorIs(t, loser, ErrLastAdmin)
}

const f7LockStmt = "UPDATE roles SET updated_at = updated_at WHERE id = 0"

// Two park points per path: "AtGuardedWrite" (the read-strip-write interleave of the finding: the user write is
// parked at its guarded statement) and "BeforeLock" (the guarded implementation parked at its lock statement, i.e.
// before the administrator set is read; the strip commits first and the user write must read its result).
func TestPGLAX1F7_UserWriteGuards_ConcurrentAdminRoleStrip_KeepsOneAdministrator(t *testing.T) {
	paths := []struct {
		name    string
		guarded func(q string) bool
		write   func(repo *UserRepository, ub int) error
	}{
		{"UpdateGuarded_deactivate", func(q string) bool { return strings.HasPrefix(q, "UPDATE users SET username") },
			func(repo *UserRepository, ub int) error {
				u, err := repo.GetByID(ub)
				if err != nil {
					return err
				}
				u.IsActive = false
				ok, err := repo.UpdateGuarded(u, u.RoleID, true)
				if err == nil && !ok {
					return fmt.Errorf("guard no longer held")
				}
				return err
			}},
		{"LockAccount", func(q string) bool { return strings.HasPrefix(q, "UPDATE users SET is_locked = 1") },
			func(repo *UserRepository, ub int) error { return repo.LockAccount(ub, time.Now().Add(time.Hour)) }},
		{"SetSuspended", func(q string) bool { return strings.HasPrefix(q, "UPDATE users SET is_locked") },
			func(repo *UserRepository, ub int) error { return repo.SetSuspended(ub, true) }},
		{"Delete", func(q string) bool { return strings.HasPrefix(q, "SELECT username FROM users") },
			func(repo *UserRepository, ub int) error { return repo.Delete(ub) }},
	}
	for _, p := range paths {
		p := p
		t.Run(p.name+"_AtGuardedWrite", func(t *testing.T) {
			w, s, n := f7Run(t, p.guarded, p.write)
			f7Assert(t, w, s, n)
		})
		t.Run(p.name+"_BeforeLock", func(t *testing.T) {
			w, s, n := f7Run(t, func(q string) bool { return q == f7LockStmt }, p.write)
			f7Assert(t, w, s, n)
		})
	}
}

// The guard itself must still refuse: with no other administrator, each path refuses with ErrLastAdmin.
func TestPGLAX1F7_UserWriteGuards_SoleAdministratorStillRefused(t *testing.T) {
	cases := map[string]func(repo *UserRepository, id int) error{
		"LockAccount":  func(repo *UserRepository, id int) error { return repo.LockAccount(id, time.Now().Add(time.Hour)) },
		"SetSuspended": func(repo *UserRepository, id int) error { return repo.SetSuspended(id, true) },
		"Delete":       func(repo *UserRepository, id int) error { return repo.Delete(id) },
		"UpdateGuarded": func(repo *UserRepository, id int) error {
			u, err := repo.GetByID(id)
			if err != nil {
				return err
			}
			u.IsActive = false
			_, err = repo.UpdateGuarded(u, u.RoleID, true)
			return err
		},
	}
	for name, fn := range cases {
		fn := fn
		t.Run(name, func(t *testing.T) {
			repo, _ := pglaX1Repo(t)
			a := pglaX1Role(t, repo, "roleA", "system.admin")
			only := pglaX1User(t, repo, "only", a, true)
			require.ErrorIs(t, fn(repo, only), ErrLastAdmin)
		})
	}
}
