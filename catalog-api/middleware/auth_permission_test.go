package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func permRouter(mw *JWTMiddleware, perm string, hit *bool) *gin.Engine {
	router := gin.New()
	router.GET("/x", mw.RequireAuth(), mw.RequirePermission(perm), func(c *gin.Context) {
		*hit = true
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return router
}

func callPerm(router *gin.Engine, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	router.ServeHTTP(w, req)
	return w
}

func TestRequirePermission_LiveCheckDecides(t *testing.T) {
	mw := setupJWTMiddleware()
	var asked struct {
		user int
		perm string
	}
	allow := false
	mw.SetPermissionChecker(func(ctx context.Context, userID int, permission string) (bool, error) {
		asked.user, asked.perm = userID, permission
		return allow, nil
	})
	var hit bool
	router := permRouter(mw, "storage.manage", &hit)
	// the claim says admin, the database says no: refused (stale claims never grant)
	tok := signTokenWithRole(t, "x", RoleAdminID)
	w := callPerm(router, tok)
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, hit)
	assert.Equal(t, 1, asked.user)
	assert.Equal(t, "storage.manage", asked.perm)
	// the claim says plain user, the database says yes: allowed
	allow = true
	w = callPerm(router, signTokenWithRole(t, "x", 2))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, hit)
}

func TestRequirePermission_FailsClosedOnCheckError(t *testing.T) {
	mw := setupJWTMiddleware()
	mw.SetPermissionChecker(func(context.Context, int, string) (bool, error) { return true, errors.New("db down") })
	var hit bool
	w := callPerm(permRouter(mw, "storage.manage", &hit), signTokenWithRole(t, "x", RoleAdminID))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.False(t, hit)
}

func TestRequirePermission_AnonymousIs401(t *testing.T) {
	mw := setupJWTMiddleware()
	mw.SetPermissionChecker(func(context.Context, int, string) (bool, error) { return true, nil })
	var hit bool
	w := callPerm(permRouter(mw, "storage.manage", &hit), "")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.False(t, hit)
}

func TestRequirePermission_WithoutCheckerOnlyAdminRoleClaimPasses(t *testing.T) {
	mw := setupJWTMiddleware()
	var hit bool
	router := permRouter(mw, "storage.manage", &hit)
	assert.Equal(t, http.StatusForbidden, callPerm(router, signTokenWithRole(t, "u", 2)).Code, "deny by default")
	assert.False(t, hit)
	assert.Equal(t, http.StatusOK, callPerm(router, signTokenWithRole(t, "a", RoleAdminID)).Code)
	assert.True(t, hit)
}

func TestRequirePermission_NonIntegerSubjectIsRefused(t *testing.T) {
	mw := setupJWTMiddleware()
	mw.SetPermissionChecker(func(context.Context, int, string) (bool, error) { return true, nil })
	var hit bool
	router := gin.New()
	router.GET("/x", func(c *gin.Context) { c.Set("role_id", 1); c.Set("user_id", "abc") }, mw.RequirePermission("p"), func(c *gin.Context) { hit = true })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/x", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.False(t, hit)
}
