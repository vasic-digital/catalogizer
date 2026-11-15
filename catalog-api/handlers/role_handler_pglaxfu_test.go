package handlers

// PGLAXFU-F2: negative controls for PUT /roles/:id error mapping. Real gin router, real migrated SQLite, real
// UserRepository / AuthService (no mocks): a missing role is 404, an unreadable stored role (a generic repository
// error) is 500, and neither is mistaken for the last-administrator conflict (409).

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPGLAXFUF2_RoleUpdateErrorMapping(t *testing.T) {
	t.Run("role not found is 404", func(t *testing.T) {
		e := newRTEnv(t)
		w := e.do(e.adminTok, "PUT", "/api/v1/roles/99999", `{"name":"Ghost","permissions":["media.view"]}`)
		assert.Equal(t, 404, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), "last administrator")
	})
	t.Run("generic repository error is 500 not 409", func(t *testing.T) {
		e := newRTEnv(t)
		w := e.do(e.adminTok, "POST", "/api/v1/roles", `{"name":"Broken","description":"d","permissions":["media.view"]}`)
		require.Equal(t, 201, w.Code, w.Body.String())
		var id int
		require.NoError(t, e.db.QueryRow(`SELECT id FROM roles WHERE name = 'Broken'`).Scan(&id))
		// Corrupt the stored permissions so the guarded read fails with a non-ErrLastAdmin, non-not-found error.
		_, err := e.db.Exec(`UPDATE roles SET permissions = 'not json' WHERE id = ?`, id)
		require.NoError(t, err)
		w = e.do(e.adminTok, "PUT", "/api/v1/roles/"+rtItoa(id), `{"name":"Broken","permissions":["media.edit"]}`)
		assert.Equal(t, 500, w.Code, w.Body.String())
		assert.NotContains(t, w.Body.String(), "last administrator")
	})
}
