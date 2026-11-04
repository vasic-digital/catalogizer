package handlers

// D9 review round 1 findings 2-4 (real router, real migrated SQLite). The
// probe repository wraps the REAL UserRepository and only observes/injects at
// well-defined seams (it never fakes a result).

import (
	"errors"
	"testing"

	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type probeRepo struct {
	*repository.UserRepository
	afterGetByID    func(id int)
	revokeCalls     int
	failRevokeCall  int // 1-based; 0 = never
	revokesAtUpdate int
}

func (p *probeRepo) GetByID(id int) (*models.User, error) {
	u, err := p.UserRepository.GetByID(id)
	if err == nil && p.afterGetByID != nil {
		p.afterGetByID(id)
	}
	return u, err
}

func (p *probeRepo) DeactivateAllUserSessions(id int) error {
	p.revokeCalls++
	if p.failRevokeCall != 0 && p.revokeCalls == p.failRevokeCall {
		return errors.New("injected revoke failure")
	}
	return p.UserRepository.DeactivateAllUserSessions(id)
}

func (p *probeRepo) UpdateGuarded(u *models.User, role int, active bool) (bool, error) {
	p.revokesAtUpdate = p.revokeCalls
	return p.UserRepository.UpdateGuarded(u, role, active)
}

func (e *d9Env) probeRoute(p *probeRepo) string {
	adm := NewAdminHandler(&AuthServiceAdapter{Inner: services.NewAuthServiceWithAudit(e.repo, "test-jwt-secret-throwaway", e.db)}, p, e.db, "test")
	e.r.PUT("/api/v1/probe/admin/users/:id", adm.UpdateUser)
	return "/api/v1/probe/admin/users/"
}

func TestD9R1_DemotingInactiveAdminIsAllowed(t *testing.T) {
	e := newD9Env(t)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"admin"}`).Code)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"status":"inactive"}`).Code)
	w := put(e, e.adminTok, e.userID, `{"role":"user"}`)
	assert.Equal(t, 200, w.Code, "an inactive admin is not an active admin: demoting it must not be refused: "+w.Body.String())
	assert.Equal(t, 2, e.storedRoleID(t, "dave"))
}

// Repository-level, atomic last-admin rule: inactive and locked admins do not count.
func TestD9R1_RepoLastAdminCountsOnlyActiveUnlocked(t *testing.T) {
	e := newD9Env(t)
	demote := func(name string) (bool, error) {
		u, err := e.repo.GetByUsername(name)
		require.NoError(t, err)
		oldRole, oldAct := u.RoleID, u.IsActive
		u.RoleID = 2
		return e.repo.UpdateGuarded(u, oldRole, oldAct)
	}
	setDave := func(role int, active, locked bool) {
		_, err := e.db.Exec(`UPDATE users SET role_id = ?, is_active = ?, is_locked = ? WHERE username = 'dave'`, role, active, locked)
		require.NoError(t, err)
	}
	setDave(1, false, false) // inactive second admin
	ok, err := demote("rtadmin")
	assert.False(t, ok)
	assert.ErrorIs(t, err, repository.ErrLastAdmin, "inactive admin must not count as another administrator")
	setDave(1, true, true) // locked second admin
	ok, err = demote("rtadmin")
	assert.False(t, ok)
	assert.ErrorIs(t, err, repository.ErrLastAdmin, "locked admin must not count")
	assert.Equal(t, 1, e.storedRoleID(t, "rtadmin"))
	setDave(1, true, false) // genuine second admin
	ok, err = demote("rtadmin")
	assert.True(t, ok)
	assert.NoError(t, err)
}

func TestD9R1_ConcurrentRoleChangeIs409(t *testing.T) {
	e := newD9Env(t)
	p := &probeRepo{UserRepository: e.repo}
	p.afterGetByID = func(id int) { // another writer changes the role between read and write
		p.afterGetByID = nil
		_, err := e.db.Exec(`UPDATE users SET role_id = 1 WHERE id = ?`, id)
		require.NoError(t, err)
	}
	base := e.probeRoute(p)
	w := e.do(e.adminTok, "PUT", base+rtItoa(e.userID), `{"email":"x@example.test","role":"user"}`)
	assert.Equal(t, 409, w.Code, w.Body.String())
	u, _ := e.repo.GetByUsername("dave")
	assert.Equal(t, 1, u.RoleID, "the concurrent change must not be overwritten")
	assert.NotEqual(t, "x@example.test", u.Email)
}

func TestD9R1_RevokeRunsBeforeUpdate(t *testing.T) {
	e := newD9Env(t)
	p := &probeRepo{UserRepository: e.repo}
	base := e.probeRoute(p)
	require.Equal(t, 200, e.do(e.adminTok, "PUT", base+rtItoa(e.userID), `{"role":"admin"}`).Code)
	assert.GreaterOrEqual(t, p.revokesAtUpdate, 1, "sessions must be revoked BEFORE the role is written (fail closed)")
	assert.Equal(t, 2, p.revokeCalls, "and once more after")
}

func (e *d9Env) auditDetails(t *testing.T, event string, userID int) []string {
	rows, err := e.db.Query(`SELECT COALESCE(details,'') FROM auth_audit_log WHERE event_type = ? AND user_id = ?`, event, userID)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var d string
		require.NoError(t, rows.Scan(&d))
		out = append(out, d)
	}
	return out
}

func TestD9R1_AuditRecordedAndSurvivesSecondRevokeFailureAndRetryRevokes(t *testing.T) {
	e := newD9Env(t)
	admin, _ := e.repo.GetByUsername("rtadmin")
	p := &probeRepo{UserRepository: e.repo, failRevokeCall: 2}
	base := e.probeRoute(p)
	w := e.do(e.adminTok, "PUT", base+rtItoa(e.userID), `{"role":"admin"}`)
	assert.Equal(t, 500, w.Code, w.Body.String())
	assert.Equal(t, 1, e.storedRoleID(t, "dave"), "the role change was persisted")
	d := e.auditDetails(t, "admin_user_role_changed", e.userID)
	require.Len(t, d, 1, "audit must be recorded right after the successful write, before the second revoke")
	assert.Equal(t, "actor="+rtItoa(admin.ID)+" role_id=2->1", d[0])
	assert.NotContains(t, d[0], "pass")

	before := p.revokeCalls
	w = e.do(e.adminTok, "PUT", base+rtItoa(e.userID), `{"role":"admin"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	assert.Equal(t, before+2, p.revokeCalls, "a retry naming the role must revoke again although no role diff remains")
	assert.Len(t, e.auditDetails(t, "admin_user_role_changed", e.userID), 1, "a no-diff retry does not duplicate the audit")
}

func TestD9R1_MixedCaseRoleNamesFromRealRepo(t *testing.T) {
	e := newD9Env(t)
	_, err := e.repo.CreateRole(&models.Role{Name: "Editor", Description: strPtrD9("d"), Permissions: models.Permissions{"media.view"}})
	require.NoError(t, err)
	require.Equal(t, 200, put(e, e.adminTok, e.userID, `{"role":"eDiTor"}`).Code)
	assert.Equal(t, "editor", e.roleOf(t, "dave"))
}
func strPtrD9(s string) *string { return &s }
