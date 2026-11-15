package handlers

// PGLA-X1: UpdateUserSettings read the whole user row and saved it back through the unguarded Update, so a role or
// activation change committed between the read and the write was silently reverted (it could also undo the promotion
// that made another administrator removable). Real stack (real SQLite schema, real repository, real AuthService); the only
// seam is a hook that runs a concurrent write right after the handler's GetByID, which makes the race deterministic.
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/require"
)

type pglaRaceRepo struct {
	*repository.UserRepository
	afterGet func(id int)
}

func (p *pglaRaceRepo) GetByID(id int) (*models.User, error) {
	u, err := p.UserRepository.GetByID(id)
	if err == nil && p.afterGet != nil {
		hook := p.afterGet
		p.afterGet = nil
		hook(id)
	}
	return u, err
}

func TestPGLAX1_UpdateUserSettings_DoesNotRevertConcurrentRoleChange(t *testing.T) {
	e := newRTEnv(t)
	id, err := e.repo.Create(&models.User{Username: "target", Email: "t@example.test", PasswordHash: "h", Salt: "s", RoleID: 1, IsActive: true})
	require.NoError(t, err)

	race := &pglaRaceRepo{UserRepository: e.repo}
	race.afterGet = func(uid int) { // concurrent demotion + deactivation committed after the handler's read
		_, werr := e.db.Exec(`UPDATE users SET role_id = 2, is_active = 0 WHERE id = ?`, uid)
		require.NoError(t, werr)
	}
	uh := NewUserHandler(race, &AuthServiceAdapter{Inner: e.authSvc})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+rtItoa(id)+"/settings", strings.NewReader(`{"settings":{"theme":"dark"}}`))
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec := httptest.NewRecorder()
	uh.UpdateUserSettings(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var rid int
	var act bool
	var settings string
	require.NoError(t, e.db.QueryRow(`SELECT role_id, is_active, COALESCE(settings,'') FROM users WHERE id = ?`, id).Scan(&rid, &act, &settings))
	require.Equal(t, 2, rid, "the concurrent role change must survive the settings save")
	require.False(t, act, "the concurrent deactivation must survive the settings save")
	require.JSONEq(t, `{"theme":"dark"}`, settings, "the settings themselves are saved")
}

// negative control: with no concurrent change the endpoint behaves exactly as before.
func TestPGLAX1_UpdateUserSettings_PlainSave(t *testing.T) {
	e := newRTEnv(t)
	id, err := e.repo.Create(&models.User{Username: "plain", Email: "p@example.test", PasswordHash: "h", Salt: "s", RoleID: 2, IsActive: true})
	require.NoError(t, err)
	uh := NewUserHandler(e.repo, &AuthServiceAdapter{Inner: e.authSvc})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/users/"+rtItoa(id)+"/settings", strings.NewReader(`{"settings":{"a":1}}`))
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec := httptest.NewRecorder()
	uh.UpdateUserSettings(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	u, err := e.repo.GetByID(id)
	require.NoError(t, err)
	require.Equal(t, 2, u.RoleID)
	require.True(t, u.IsActive)
	require.JSONEq(t, `{"a":1}`, u.Settings)

	// unknown user keeps 404
	req = httptest.NewRequest(http.MethodPut, "/api/v1/users/99999/settings", strings.NewReader(`{"settings":{}}`))
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec = httptest.NewRecorder()
	uh.UpdateUserSettings(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
