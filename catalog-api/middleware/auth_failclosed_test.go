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
	assert.JSONEq(t, `{"success":false,"error":"Could not verify admin role"}`, w.Body.String())
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

func runAdminGate(checker AdminChecker) (*httptest.ResponseRecorder, bool) {
	mw := setupJWTMiddleware()
	mw.SetAdminChecker(checker)
	reached := false
	router := gin.New()
	router.GET("/admin/x", func(c *gin.Context) {
		c.Set("role_id", RoleAdminID)
		c.Set("user_id", 5)
	}, mw.RequireAdmin(), func(c *gin.Context) { reached = true; c.String(http.StatusOK, "handler-ran") })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/x", nil))
	return w, reached
}

// SSEFU2R-m1: both checker-error shapes, (true, err) and (false, err), are denied 503 with the pinned body.
func TestSSEFU2R_M1_RequireAdmin_BothErrorShapesPinnedBody(t *testing.T) {
	for name, admin := range map[string]bool{"true_with_err": true, "false_with_err": false} {
		admin := admin
		t.Run(name, func(t *testing.T) {
			w, reached := runAdminGate(func(context.Context, int) (bool, error) { return admin, errors.New("db down") })
			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
			assert.JSONEq(t, `{"success":false,"error":"Could not verify admin role"}`, w.Body.String())
			assert.False(t, reached)
		})
	}
}

// SSEFU2R-m2: positive control in the same file: an admin passes through to the handler; a non-admin gets the pinned 403 body.
func TestSSEFU2R_M2_RequireAdmin_PositiveControl(t *testing.T) {
	w, reached := runAdminGate(func(context.Context, int) (bool, error) { return true, nil })
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "handler-ran", w.Body.String())
	assert.True(t, reached, "an admin must reach the handler")

	w, reached = runAdminGate(func(context.Context, int) (bool, error) { return false, nil })
	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.JSONEq(t, `{"success":false,"error":"Admin role required"}`, w.Body.String())
	assert.False(t, reached)
}
