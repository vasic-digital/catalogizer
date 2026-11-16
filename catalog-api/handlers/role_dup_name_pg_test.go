package handlers

// ADMD1R-m1: a duplicate role name must answer 409 on PostgreSQL as on SQLite. Real gin router, real migrated
// database, real UserRepository / AuthService. The live PostgreSQL leg runs only when CAT_TEST_PG_DSN names a
// reachable PostgreSQL (honest SKIP otherwise); the DSN itself is never printed.

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lib/pq"
	"os"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newRTEnvPG(t *testing.T) *rtEnv {
	t.Helper()
	dsn := os.Getenv("CAT_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("CAT_TEST_PG_DSN not set: live PostgreSQL leg not run")
	}
	sqlDB, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, sqlDB.Ping())
	// A private schema per run on ONE connection keeps the run independent of earlier state.
	sqlDB.SetMaxOpenConns(1)
	schema := fmt.Sprintf("admd1r_%d", time.Now().UnixNano())
	_, err = sqlDB.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = sqlDB.Exec("DROP SCHEMA " + schema + " CASCADE") })
	_, err = sqlDB.Exec("SET search_path TO " + schema)
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectPostgres)
	require.NoError(t, db.RunMigrations(context.Background()))
	return newRTEnvOn(t, db)
}

func dupRoleChecks(t *testing.T, e *rtEnv) {
	w := e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"DupRole","description":"d","permissions":["media.view"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	w = e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"DupRole","description":"d","permissions":["media.view"]}`)
	assert.Equal(t, 409, w.Code, "duplicate role name on create: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Role name already exists")

	w = e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"Other","description":"d","permissions":["media.view"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	// the role created second is renamed onto the first one's name
	var id struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &id))
	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(id.ID), `{"name":"DupRole","permissions":["media.view"]}`)
	assert.Equal(t, 409, w.Code, "duplicate role name on update: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Role name already exists")
	// negative control: a fresh name is accepted
	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(id.ID), `{"name":"Renamed","permissions":["media.view"]}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
}

func TestADMD1RM1_DuplicateRoleNameIs409_SQLite(t *testing.T) { dupRoleChecks(t, newRTEnv(t)) }

func TestADMD1RM1_DuplicateRoleNameIs409_PostgresLive(t *testing.T) { dupRoleChecks(t, newRTEnvPG(t)) }

// Unit-class table for the classifier (test doubles are legal here, §11.4.289(C)(1)): a PostgreSQL unique violation
// by SQLSTATE (also wrapped), the dialect texts, and a negative control that must NOT classify as a name conflict.
func TestADMD1RM1_IsRoleNameConflictTable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"pq 23505", &pq.Error{Code: "23505", Message: "boom"}, true},
		{"wrapped pq 23505", fmt.Errorf("failed to create role: %w", &pq.Error{Code: "23505"}), true},
		{"pq text only", errors.New(`pq: duplicate key value violates unique constraint "roles_name_key"`), true},
		{"sqlite text", errors.New("UNIQUE constraint failed: roles.name"), true},
		{"pq other code", &pq.Error{Code: "23503", Message: "fk"}, false},
		{"generic", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isRoleNameConflict(c.err), c.name)
	}
}

// ADMD1R-n3: PUT /roles/{id} that omits description and permissions writes description NULL and permissions [].
func TestADMD1RN3_PutOmittedFieldsWriteNullDescriptionAndEmptyPermissions(t *testing.T) {
	e := newRTEnv(t)
	w := e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"Keep","description":"orig","permissions":["media.view"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var created struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(created.ID), `{"name":"Keep"}`)
	require.Equal(t, 200, w.Code, w.Body.String())
	got, err := e.repo.GetRole(created.ID)
	require.NoError(t, err)
	assert.Nil(t, got.Description, "omitted description is stored as NULL, not as an empty string")
	assert.NotNil(t, got.Permissions)
	assert.Empty(t, got.Permissions, "omitted permissions are stored as an empty list")
	assert.Contains(t, w.Body.String(), `"permissions":[]`)
}
