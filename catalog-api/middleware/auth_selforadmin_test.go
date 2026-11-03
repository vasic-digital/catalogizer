package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// F-B: RequireSelfOrAdmin compares the path id to the caller's id as an exact integer string.
// The token's subject is user 1 (role 2, not an administrator by the live check).
func TestRequireSelfOrAdmin_ExactIDMatch(t *testing.T) {
	mw := setupJWTMiddleware()
	isAdmin := false
	mw.SetAdminChecker(func(ctx context.Context, userID int) (bool, error) { return isAdmin, nil })
	router := gin.New()
	router.GET("/u/:id", mw.RequireAuth(), mw.RequireSelfOrAdmin("id"), func(c *gin.Context) { c.Status(http.StatusOK) })
	tok := signTokenWithRole(t, "u1", 2)
	call := func(id string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/u/"+id, nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		router.ServeHTTP(w, req)
		return w.Code
	}
	assert.Equal(t, http.StatusOK, call("1"), "own id")
	for _, other := range []string{"11", "2", "01", "001", "+1", "1.0", "0x1", "1%20", "-1"} {
		assert.Equal(t, http.StatusForbidden, call(other), "id %q is not user 1 and must not pass as self", other)
	}
	isAdmin = true
	for _, other := range []string{"11", "2", "01"} {
		assert.Equal(t, http.StatusOK, call(other), "an administrator may read %q", other)
	}
}
