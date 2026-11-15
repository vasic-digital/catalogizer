package repository

// PGLAX1-F6 (SQLite): UpdateRole read wasAdmin and the administrator-role set outside the guarded UPDATE with no
// transaction, so two concurrent strips of two different administrator roles each decided from the same stale
// "both roles still grant system.admin" set and, after both writes, no active administrator was left.
// Real SQLite file database, real UserRepository; the only addition is a driver wrapper that holds each
// "UPDATE roles SET name" at a barrier until two writers have arrived, which forces the read-read-write-write
// interleaving deterministically (every read of both calls has finished before either UPDATE runs).
import (
	"context"
	"database/sql"
	"database/sql/driver"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	sqlite3 "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/require"
)

type f6Barrier struct {
	mu      sync.Mutex
	arrived int
	want    int
	release chan struct{}
	timeout time.Duration
}

func (b *f6Barrier) wait() {
	b.mu.Lock()
	b.arrived++
	if b.arrived == b.want {
		close(b.release)
	}
	b.mu.Unlock()
	select {
	case <-b.release:
	case <-time.After(b.timeout):
		// A guarded implementation serialises the two calls, so the second writer can never arrive
		// while the first is inside its transaction: let the first one through after the timeout.
	}
}

type f6Driver struct{ b *f6Barrier }

func (d f6Driver) Open(dsn string) (driver.Conn, error) {
	c, err := (&sqlite3.SQLiteDriver{}).Open(dsn)
	if err != nil {
		return nil, err
	}
	return &f6Conn{Conn: c, b: d.b}, nil
}

type f6Conn struct {
	driver.Conn
	b *f6Barrier
}

func (c *f6Conn) Exec(q string, args []driver.Value) (driver.Result, error) {
	if strings.HasPrefix(strings.TrimSpace(q), "UPDATE roles SET name") {
		c.b.wait()
	}
	return c.Conn.(driver.Execer).Exec(q, args) //nolint:staticcheck // the vendored driver only has the legacy interfaces
}

func (c *f6Conn) Query(q string, args []driver.Value) (driver.Rows, error) {
	return c.Conn.(driver.Queryer).Query(q, args) //nolint:staticcheck
}

func TestPGLAX1F6_ConcurrentStripOfTwoAdminRoles_KeepsOneAdministrator(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f6.db")
	bar := &f6Barrier{want: 2, release: make(chan struct{}), timeout: 2 * time.Second}
	drvName := "sqlite3_f6_" + strings.ReplaceAll(t.Name(), "/", "_")
	sql.Register(drvName, f6Driver{b: bar})
	sqlDB, err := sql.Open(drvName, path+"?_busy_timeout=20000&_foreign_keys=1")
	require.NoError(t, err)
	_, err = sqlDB.Exec("PRAGMA journal_mode=WAL") // as database.NewConnection does; default deferred BEGIN, as in production
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
	pglaX1User(t, repo, "ub", b, true)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i, id := range []int{a, b} {
		wg.Add(1)
		go func(i, id int) {
			defer wg.Done()
			errs[i] = repo.UpdateRole(&models.Role{ID: id, Name: "stripped" + string(rune(65+i)), Permissions: models.Permissions{"media.view"}})
		}(i, id)
	}
	wg.Wait()

	admins := 0
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users u JOIN roles r ON r.id = u.role_id
		WHERE u.is_active = 1 AND r.permissions LIKE '%system.admin%'`).Scan(&admins))
	t.Logf("strip errors: A=%v B=%v; active administrators left: %d", errs[0], errs[1], admins)
	require.GreaterOrEqual(t, admins, 1, "last-administrator invariant violated: both administrator roles were stripped")
	require.True(t, (errs[0] == nil) != (errs[1] == nil), "exactly one strip must succeed and one be refused: %v / %v", errs[0], errs[1])
	loser := errs[0]
	if loser == nil {
		loser = errs[1]
	}
	// The refusal must be the clean last-administrator refusal, not a database-busy failure that merely avoids the violation.
	require.ErrorIs(t, loser, ErrLastAdmin)
}
