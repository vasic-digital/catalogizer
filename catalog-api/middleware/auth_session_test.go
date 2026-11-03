package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

func tokenWithSession(t *testing.T, m *JWTMiddleware, sid string) string {
	t.Helper()
	claims := &Claims{Username: "u", RoleID: 1, SessionID: sid, RegisteredClaims: jwt.RegisteredClaims{
		Subject: "7", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secretKey)
	assert.NoError(t, err)
	return s
}

func doAuth(m *JWTMiddleware, tok string) int {
	r := gin.New()
	r.GET("/p", m.RequireAuth(), func(c *gin.Context) { c.Status(200) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/p", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	return w.Code
}

// CAT-021 round 4 F1: a revoked session must reject an otherwise valid access JWT.
func TestRequireAuth_SessionChecker(t *testing.T) {
	m := setupJWTMiddleware()
	var gotSID, gotUID string
	m.SetSessionChecker(func(_ context.Context, sid, uid string) error {
		gotSID, gotUID = sid, uid
		if sid == "dead" {
			return errors.New("revoked")
		}
		return nil
	})
	assert.Equal(t, 200, doAuth(m, tokenWithSession(t, m, "5")))
	assert.Equal(t, "5", gotSID)
	assert.Equal(t, "7", gotUID)
	assert.Equal(t, 401, doAuth(m, tokenWithSession(t, m, "dead")), "revoked session must be rejected")
	assert.Equal(t, 401, doAuth(m, tokenWithSession(t, m, "")), "no session_id claim must fail closed")
}

// Without a checker (unit-test construction) behaviour is unchanged.
func TestRequireAuth_NoCheckerUnchanged(t *testing.T) {
	m := setupJWTMiddleware()
	assert.Equal(t, 200, doAuth(m, tokenWithSession(t, m, "")))
}
