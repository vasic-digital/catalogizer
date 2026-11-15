package handlers

// PGLAX1-F4: PUT /roles/:id that would strip system.admin from the role of the last active administrator is refused by
// the repository with ErrLastAdmin; the handler must answer 409 (like UpdateUser / the admin handler), not 500.
// Real gin router, real migrated SQLite, real UserRepository / AuthService (no mocks).

import (
	"encoding/json"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGLAX1F4_RoleUpdateStrippingLastAdminIs409(t *testing.T) {
	e := newRTEnv(t)
	w := e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"Ops","description":"d","permissions":["system.admin","media.view"]}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var role models.Role
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &role))

	// rtadmin becomes the only administrator, through the custom role.
	_, err := e.db.Exec(`UPDATE users SET role_id = 2 WHERE username <> 'rtadmin'`)
	require.NoError(t, err)
	_, err = e.db.Exec(`UPDATE users SET role_id = ? WHERE username = 'rtadmin'`, role.ID)
	require.NoError(t, err)

	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(role.ID), `{"name":"Ops","permissions":["media.view"]}`)
	assert.Equal(t, 409, w.Code, "refused strip of the last administrator is a conflict, not a server error: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "last administrator")

	got, gerr := e.repo.GetRole(role.ID)
	require.NoError(t, gerr)
	assert.Contains(t, []string(got.Permissions), "system.admin", "the refused write changed nothing")

	// negative control: a strip that leaves another active administrator is applied (200).
	// PGLAXFU-N2: deterministic second administrator, created unconditionally (no branching on seeded state).
	hash, salt, herr := e.authSvc.HashPasswordForUser("Throwaway-pw1!")
	require.NoError(t, herr)
	_, cerr := e.repo.Create(&models.User{Username: "second", Email: "second@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, cerr)
	var admins int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = 1`).Scan(&admins))
	require.Equal(t, 1, admins, "exactly the created second administrator holds the system role")
	w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(role.ID), `{"name":"Ops","permissions":["media.view"]}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
}
