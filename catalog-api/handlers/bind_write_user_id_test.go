package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// The route layer already refuses anonymous callers; this pins the helper's own refusal
// so a route mounted without the auth middleware cannot attribute a write to user 0.
func TestBindWriteUserID_ContextWithoutCallerIsRefused(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name string
		set  func(c *gin.Context)
	}{
		{"no user_id in context", func(c *gin.Context) {}},
		{"user_id zero", func(c *gin.Context) { c.Set("user_id", 0) }},
		{"user_id negative", func(c *gin.Context) { c.Set("user_id", -3) }},
		{"user_id not a number", func(c *gin.Context) { c.Set("user_id", "abc") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			tc.set(c)
			body := 7
			assert.False(t, bindWriteUserID(c, &body))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, 7, body, "body id untouched on refusal")
		})
	}
}
