package handlers

// Authorization-gap regression tests (CAT-049 R3-M1, CAT-050 R3-M2, CAT-051 F4,
// CAT-052). Everything here runs on the REAL stack built by newRTEnv:
// production migrations on SQLite, the real UserRepository / AuthService, the
// real JWT middleware and the real handlers behind a real gin router. No
// repository or auth fake is used as evidence (constitution 11.4.289).

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const azPass = "Az!Throwaway#5"

// azUser creates a user with roleID and returns its id and a real login token.
func (e *rtEnv) azUser(t *testing.T, name string, roleID int) (int, string) {
	t.Helper()
	hash, salt, err := e.authSvc.HashPasswordForUser(azPass)
	require.NoError(t, err)
	id, err := e.repo.Create(&models.User{Username: name, Email: name + "@example.test",
		PasswordHash: hash, Salt: salt, RoleID: roleID, IsActive: true})
	require.NoError(t, err)
	w := e.login(name, azPass)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	return id, res.SessionToken
}

func (e *rtEnv) azRole(t *testing.T, name string, perms ...string) int {
	t.Helper()
	id, err := e.repo.CreateRole(&models.Role{Name: name, Description: &name, Permissions: models.Permissions(perms)})
	require.NoError(t, err)
	return id
}

func (e *rtEnv) ping(tok string) int {
	return e.do(tok, "GET", "/api/v1/admin/ping", "").Code
}

// F4 / CAT-051: the admin gate follows the role's permissions, not role_id==1.
func TestAZ_F4_AdminGateUsesRolePermissionNotMagicRoleID(t *testing.T) {
	e := newRTEnv(t)
	superRole := e.azRole(t, "Superuser", models.PermissionSystemAdmin, models.PermissionUserManage)
	_, tok := e.azUser(t, "super1", superRole)
	assert.Equal(t, http.StatusOK, e.ping(tok), "a role carrying system.admin must pass the admin gate even when its id is not 1")
	assert.Equal(t, http.StatusOK, e.ping(e.adminTok), "seeded admin (role 1, \"*\") still passes")
}

func TestAZ_F4_DemotionTakesEffectWithoutRelogin(t *testing.T) {
	e := newRTEnv(t)
	id, tok := e.azUser(t, "adm2", 1)
	require.Equal(t, http.StatusOK, e.ping(tok))
	// Demote through the data layer, bypassing the handler (which would revoke
	// sessions): any other writer, a second instance, or a restored backup.
	_, err := e.db.Exec("UPDATE users SET role_id = 2 WHERE id = ?", id)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, e.ping(tok), "stale role_id claim must not keep admin access after demotion")
}

func TestAZ_F4_RolePermissionRemovalTakesEffect(t *testing.T) {
	e := newRTEnv(t)
	superRole := e.azRole(t, "Superuser", models.PermissionSystemAdmin)
	_, tok := e.azUser(t, "super2", superRole)
	require.Equal(t, http.StatusOK, e.ping(tok))
	_, err := e.db.Exec(`UPDATE roles SET permissions = '["media.view"]' WHERE id = ?`, superRole)
	require.NoError(t, err)
	assert.Equal(t, http.StatusForbidden, e.ping(tok))
}

func TestAZ_F4_DeactivatedAdminLosesAccess(t *testing.T) {
	e := newRTEnv(t)
	id, tok := e.azUser(t, "adm3", 1)
	require.Equal(t, http.StatusOK, e.ping(tok))
	_, err := e.db.Exec("UPDATE users SET is_active = 0 WHERE id = ?", id)
	require.NoError(t, err)
	assert.NotEqual(t, http.StatusOK, e.ping(tok))
}

func (e *rtEnv) putUser(tok string, id int, body string) *httptest.ResponseRecorder {
	return e.do(tok, "PUT", "/api/v1/users/"+rtItoa(id), body)
}

// R3-M2 / CAT-050: no assigning a role stronger than your own.
func TestAZ_R3M2_NonAdminCannotAssignStrongerNonAdminRole(t *testing.T) {
	e := newRTEnv(t)
	mgr := e.azRole(t, "Manager", models.PermissionUserManage, models.PermissionUserUpdate, models.PermissionUserCreate,
		models.PermissionMediaView, models.PermissionMediaDownload)
	strong := e.azRole(t, "Powerful", models.PermissionUserManage, models.PermissionUserDelete, models.PermissionSystemConfig)
	weak := e.azRole(t, "Viewer", models.PermissionMediaView)
	_, mtok := e.azUser(t, "mgr", mgr)
	vid, _ := e.azUser(t, "victim", 2)

	w := e.putUser(mtok, vid, `{"role_id":`+rtItoa(strong)+`}`)
	assert.Equal(t, http.StatusForbidden, w.Code, "stronger non-admin role must be refused: "+w.Body.String())
	u, _ := e.repo.GetByID(vid)
	assert.Equal(t, 2, u.RoleID, "role must be unchanged")

	w = e.putUser(mtok, vid, `{"role_id":`+rtItoa(weak)+`}`)
	assert.Equal(t, http.StatusOK, w.Code, "a role within the caller's own permissions is allowed: "+w.Body.String())
	u, _ = e.repo.GetByID(vid)
	assert.Equal(t, weak, u.RoleID)

	// creating a user with a stronger role is the same hole
	body := `{"username":"mint","email":"mint@example.test","password":"` + rtUserPass + `","role_id":` + rtItoa(strong) + `}`
	w = e.do(mtok, "POST", "/api/v1/users", body)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	// ...and an equal-or-weaker role is fine
	body = `{"username":"mint2","email":"mint2@example.test","password":"` + rtUserPass + `","role_id":2}`
	w = e.do(mtok, "POST", "/api/v1/users", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// admin may still assign anything
	w = e.putUser(e.adminTok, vid, `{"role_id":`+rtItoa(strong)+`}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

// hookRepo wraps the REAL repository and runs hook once, the first time the
// handler resolves the target's role: that is exactly the point between the
// authorization decision and the write.
type hookRepo struct {
	*repository.UserRepository
	once sync.Once
	hook func()
}

func (h *hookRepo) GetRole(id int) (*models.Role, error) {
	r, err := h.UserRepository.GetRole(id)
	h.once.Do(func() {
		if h.hook != nil {
			h.hook()
		}
	})
	return r, err
}

// R3-M1 / CAT-049 (deterministic interleaving): between the guard's read of the
// target and the save, an admin promotes the target. The non-admin's stale save
// must not demote/overwrite the admin.
func TestAZ_R3M1_StaleSaveCannotOverwriteConcurrentPromotion(t *testing.T) {
	e := newRTEnv(t)
	mgr := e.azRole(t, "Manager", models.PermissionUserManage, models.PermissionUserUpdate)
	_, mtok := e.azUser(t, "mgr", mgr)
	vid, _ := e.azUser(t, "victim", 2)

	hr := &hookRepo{UserRepository: e.repo}
	hr.hook = func() {
		_, err := e.db.Exec("UPDATE users SET role_id = 1 WHERE id = ?", vid)
		require.NoError(t, err)
	}
	e.r.PUT("/api/v1/azrace/users/:id", WrapHTTPHandler(NewUserHandler(hr, &AuthServiceAdapter{Inner: e.authSvc}).UpdateUser))

	w := e.do(mtok, "PUT", "/api/v1/azrace/users/"+rtItoa(vid), `{"display_name":"renamed"}`)
	u, err := e.repo.GetByID(vid)
	require.NoError(t, err)
	assert.Equal(t, 1, u.RoleID, "the concurrent promotion must survive the stale save (response %d %s)", w.Code, w.Body.String())
	assert.NotEqual(t, http.StatusOK, w.Code, "a save based on a stale authorization decision must be rejected")
}

// R3-M1 under real concurrency: run with -race. An admin promotes the victim
// once while managers keep saving; afterwards the victim must still be admin.
func TestAZ_R3M1_ConcurrentPromotionAndManagerSaves(t *testing.T) {
	for iter := 0; iter < 25; iter++ {
		e := newRTEnv(t)
		mgr := e.azRole(t, "Manager", models.PermissionUserManage, models.PermissionUserUpdate)
		_, mtok := e.azUser(t, "mgr", mgr)
		vid, _ := e.azUser(t, "victim", 2)

		var wg sync.WaitGroup
		stop := make(chan struct{})
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; ; i++ {
					select {
					case <-stop:
						return
					default:
					}
					e.putUser(mtok, vid, `{"display_name":"n`+rtItoa(g*1000+i)+`"}`)
				}
			}(g)
		}
		w := e.putUser(e.adminTok, vid, `{"role_id":1}`)
		require.Equal(t, 200, w.Code, w.Body.String())
		close(stop)
		wg.Wait()
		u, err := e.repo.GetByID(vid)
		require.NoError(t, err)
		require.Equal(t, 1, u.RoleID, "iteration %d: a manager save overwrote the admin promotion", iter)
	}
}

// CAT-052: the fake store used by the handler tests must behave like the real
// repository for an unknown role id.
func TestAZ_CAT052_FakeGetRoleMatchesRealRepositoryForUnknownRole(t *testing.T) {
	e := newRTEnv(t)
	_, realErr := e.repo.GetRole(987654)
	require.Error(t, realErr)
	_, fakeErr := newMemStore().GetRole(987654)
	assert.Error(t, fakeErr, "memStore.GetRole must fail for an unknown role like the real repository")
}

// B2: a strong custom role without system.admin is refused at the admin gate.
func TestAZ_F4_StrongNonAdminRoleRefusedAtAdminGate(t *testing.T) {
	e := newRTEnv(t)
	strong := e.azRole(t, "Strong", models.PermissionSystemConfig, models.PermissionUserManage, models.PermissionUserDelete)
	_, tok := e.azUser(t, "strong1", strong)
	assert.Equal(t, http.StatusForbidden, e.ping(tok))
	prefixish := e.azRole(t, "Sysish", "system.administrator", "system.admin.extra")
	_, tok = e.azUser(t, "strong2", prefixish)
	assert.Equal(t, http.StatusForbidden, e.ping(tok), "near-miss permission names must not grant admin")
}

// N2: a prefix wildcard without a segment boundary ("sys*") must not grant system.admin.
func TestAZ_N2_WildcardWithoutSegmentBoundaryDoesNotGrantAdmin(t *testing.T) {
	e := newRTEnv(t)
	for i, perm := range []string{"sys*", "s*", "system*"} {
		r := e.azRole(t, "Wild"+rtItoa(i), perm)
		_, tok := e.azUser(t, "wild"+rtItoa(i), r)
		assert.Equal(t, http.StatusForbidden, e.ping(tok), perm)
	}
	ok := e.azRole(t, "WildOK", "system.*")
	_, tok := e.azUser(t, "wildok", ok)
	assert.Equal(t, http.StatusOK, e.ping(tok), "system.* is a real segment wildcard")
}

// B3: the REAL IsAdmin error paths fail closed with 503 through the router.
func TestAZ_F4_IsAdminLookupErrorFailsClosed(t *testing.T) {
	t.Run("corrupt permissions JSON", func(t *testing.T) {
		e := newRTEnv(t)
		role := e.azRole(t, "Soon-corrupt", models.PermissionSystemAdmin)
		_, tok := e.azUser(t, "corrupt1", role)
		require.Equal(t, http.StatusOK, e.ping(tok))
		_, err := e.db.Exec(`UPDATE roles SET permissions = '{not json' WHERE id = ?`, role)
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, e.ping(tok))
	})
	t.Run("roles table unavailable", func(t *testing.T) {
		e := newRTEnv(t)
		_, tok := e.azUser(t, "gone1", 1)
		require.Equal(t, http.StatusOK, e.ping(tok))
		_, err := e.db.Exec(`ALTER TABLE roles RENAME TO roles_gone`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusServiceUnavailable, e.ping(tok))
	})
}
