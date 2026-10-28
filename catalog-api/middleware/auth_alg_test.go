package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func forgedClaims() *Claims {
	return &Claims{
		Username: "mallory",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			Issuer:    "catalog-api",
		},
	}
}

func signWith(t *testing.T, method jwt.SigningMethod, key interface{}) string {
	s, err := jwt.NewWithClaims(method, forgedClaims()).SignedString(key)
	require.NoError(t, err)
	return s
}

func algTokens(t *testing.T) map[string]string {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return map[string]string{
		"alg=none":                   signWith(t, jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType),
		"RS256 (asymmetric)":         signWith(t, jwt.SigningMethodRS256, rsaKey),
		"HS384 signed with real key": signWith(t, jwt.SigningMethodHS384, []byte(testSecret)),
		"HS512 signed with real key": signWith(t, jwt.SigningMethodHS512, []byte(testSecret)),
	}
}

func TestValidateToken_RejectsUnexpectedAlgorithms(t *testing.T) {
	mw := setupJWTMiddleware()
	for name, tok := range algTokens(t) {
		_, err := mw.ValidateToken(tok)
		assert.Error(t, err, "%s must be rejected by ValidateToken", name)
	}
}

func TestRequireAuth_RejectsUnexpectedAlgorithms(t *testing.T) {
	mw := setupJWTMiddleware()
	r := gin.New()
	r.GET("/p", mw.RequireAuth(), func(c *gin.Context) { c.String(200, "ok") })
	for name, tok := range algTokens(t) {
		req := httptest.NewRequest("GET", "/p", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, "%s must yield 401", name)
	}
}

func TestValidateToken_StillAcceptsPinnedHS256(t *testing.T) {
	mw := setupJWTMiddleware()
	tok, err := mw.GenerateToken("alice", "42", 1)
	require.NoError(t, err)
	_, err = mw.ValidateToken(tok)
	assert.NoError(t, err)
}
