package repository

// PGLAXFU-F1: the PostgreSQL path of UpdateRole fails closed (section 11.4.252) when the current role row cannot be
// parsed: the transaction is rolled back and no UPDATE is issued. Two forms:
//   - sqlmock (unit class, always runs): corrupt permissions JSON returned after the advisory lock.
//   - live PostgreSQL (gated on CATALOGIZER_PG_TEST_{HOST,PORT,USER,PASSWORD}; honest SKIP otherwise).

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPGLAXFUF1_UpdateRole_PGPathFailsClosedOnUnparsableRole(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()
	repo := NewUserRepository(database.WrapDB(sqlDB, database.DialectPostgres))

	mock.ExpectBegin()
	mock.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("FROM roles").WillReturnRows(sqlmock.NewRows([]string{"id", "permissions"}).AddRow(5, `["system.admin"]`))
	mock.ExpectQuery("FROM roles WHERE id").WithArgs(5).WillReturnRows(
		sqlmock.NewRows([]string{"permissions", "is_system"}).AddRow("not json", false))
	// no ExpectExec("UPDATE roles"): any write attempt is an unexpected call and fails the test.
	mock.ExpectRollback()

	err = repo.UpdateRole(&models.Role{ID: 5, Name: "n", Permissions: models.Permissions{"media.view"}})
	require.Error(t, err, "an unreadable current role must refuse the strip on the PostgreSQL path")
	require.Contains(t, err.Error(), "failed to parse current role permissions")
	require.NoError(t, mock.ExpectationsWereMet(), "lock, reads, then rollback; no UPDATE was issued")

	// control: a scan error on the same row (not ErrNoRows) is refused too.
	sqlDB2, mock2, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB2.Close()
	repo2 := NewUserRepository(database.WrapDB(sqlDB2, database.DialectPostgres))
	mock2.ExpectBegin()
	mock2.ExpectExec("pg_advisory_xact_lock").WillReturnResult(sqlmock.NewResult(0, 0))
	mock2.ExpectQuery("FROM roles").WillReturnRows(sqlmock.NewRows([]string{"id", "permissions"}).AddRow(5, `["system.admin"]`))
	mock2.ExpectQuery("FROM roles WHERE id").WithArgs(5).WillReturnError(fmt.Errorf("connection reset"))
	mock2.ExpectRollback()
	err = repo2.UpdateRole(&models.Role{ID: 5, Name: "n", Permissions: models.Permissions{"media.view"}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to read role before update")
	require.NoError(t, mock2.ExpectationsWereMet())
}

func TestPGLAXFUF1_Live_UpdateRoleCorruptPermissionsFailsClosed(t *testing.T) {
	host := os.Getenv("CATALOGIZER_PG_TEST_HOST")
	port, _ := strconv.Atoi(os.Getenv("CATALOGIZER_PG_TEST_PORT"))
	user := os.Getenv("CATALOGIZER_PG_TEST_USER")
	pass := os.Getenv("CATALOGIZER_PG_TEST_PASSWORD")
	if host == "" || port == 0 {
		t.Skip("CATALOGIZER_PG_TEST_HOST/PORT not set: live PostgreSQL UpdateRole fail-closed test skipped (honest SKIP, not a pass)")
	}
	ctx := context.Background()
	adm, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", user, pass, host, port))
	require.NoError(t, err)
	defer adm.Close()
	name := fmt.Sprintf("tmp_fuf1_%d", time.Now().UnixNano())
	_, err = adm.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	defer func() { _, _ = adm.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name) }()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: user, Password: pass, SSLMode: "disable"})
	require.NoError(t, err)
	defer db.Close()
	require.NoError(t, db.RunMigrations(ctx))
	repo := NewUserRepository(db)
	_, err = db.Exec(`UPDATE users SET role_id = 2`)
	require.NoError(t, err)

	rid, err := repo.CreateRole(&models.Role{Name: "fuf1ops", Permissions: models.Permissions{"system.admin"}})
	require.NoError(t, err)
	// permissions is a TEXT column, so corrupt JSON can be stored. On the live path the corrupt row is
	// refused by the administrator-role-set read (adminRoleIDsTx) that precedes currentlyAdmin; the sqlmock test
	// above isolates currentlyAdmin itself. Either way the write must be refused and change nothing.
	_, err = db.Exec(`UPDATE roles SET permissions = ? WHERE id = ?`, "not json", rid)
	require.NoError(t, err)
	uerr := repo.UpdateRole(&models.Role{ID: rid, Name: "fuf1ops", Permissions: models.Permissions{"media.view"}})
	require.Error(t, uerr, "fail closed on the live PostgreSQL path")
	var got string
	require.NoError(t, db.QueryRow(`SELECT permissions FROM roles WHERE id = ?`, rid).Scan(&got))
	require.Equal(t, "not json", got, "the refused write changed nothing")
}
