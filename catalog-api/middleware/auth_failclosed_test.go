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

// SSEFU2-F2: a checker that returns (true, err) must be DENIED with 503 and the pinned text: the error wins
// over the boolean (kills `err != nil && !admin`).
func TestSSEFU2_F2_RequireAdmin_CheckerTrueWithErrorIsDenied503(t *testing.T) {
	mw := setupJWTMiddleware()
	mw.SetAdminChecker(func(context.Context, int) (bool, error) { return true, errors.New("db down") })
	reached := false
	router := gin.New()
	router.GET("/admin/x", func(c *gin.Context) {
		c.Set("role_id", RoleAdminID)
		c.Set("user_id", 5)
	}, mw.RequireAdmin(), func(c *gin.Context) { reached = true; c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/x", nil))
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "Could not verify admin role")
	assert.False(t, reached, "the protected handler must not run")
}

// The checker receives the caller's own uid: true only for uid 1.
func TestSSEFU2_F2_RequireAdmin_CheckerReceivesCallersOwnUID(t *testing.T) {
	run := func(uid int) int {
		mw := setupJWTMiddleware()
		mw.SetAdminChecker(func(_ context.Context, id int) (bool, error) { return id == 1, nil })
		router := gin.New()
		router.GET("/admin/x", func(c *gin.Context) {
			c.Set("role_id", RoleAdminID)
			c.Set("user_id", uid)
		}, mw.RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/x", nil))
		return w.Code
	}
	assert.Equal(t, http.StatusForbidden, run(5))
	assert.Equal(t, http.StatusOK, run(1))
}
