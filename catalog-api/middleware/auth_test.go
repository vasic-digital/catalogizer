package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

const testSecret = "test-middleware-secret-key"

func setupJWTMiddleware() *JWTMiddleware {
	return NewJWTMiddleware(testSecret)
}

// TestNewJWTMiddleware verifies middleware construction.

// TestGenerateToken_Success verifies token generation with valid parameters.
func TestGenerateToken_Success(t *testing.T) {
	mw := setupJWTMiddleware()

	token, err := mw.GenerateToken("testuser", "123", 24)
	require.NoError(t, err)
	assert.NotEmpty(t, token)
}

// TestGenerateAndValidateToken verifies round-trip token generation and validation.
func TestGenerateAndValidateToken(t *testing.T) {
	mw := setupJWTMiddleware()

	token, err := mw.GenerateToken("alice", "42", 24)
	require.NoError(t, err)

	claims, err := mw.ValidateToken(token)
	require.NoError(t, err)

	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "42", claims.Subject)
	assert.Equal(t, "catalog-api", claims.Issuer)
	assert.True(t, claims.ExpiresAt.After(time.Now()))
}

// TestValidateToken_InvalidToken verifies that garbage tokens are rejected.
func TestValidateToken_InvalidToken(t *testing.T) {
	mw := setupJWTMiddleware()

	claims, err := mw.ValidateToken("not.a.valid.token")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestValidateToken_EmptyToken verifies that an empty token is rejected.
func TestValidateToken_EmptyToken(t *testing.T) {
	mw := setupJWTMiddleware()

	claims, err := mw.ValidateToken("")
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestValidateToken_WrongSecret verifies that tokens signed with a different secret are rejected.
func TestValidateToken_WrongSecret(t *testing.T) {
	mw1 := NewJWTMiddleware("secret-one")
	mw2 := NewJWTMiddleware("secret-two")

	token, err := mw1.GenerateToken("user", "1", 24)
	require.NoError(t, err)

	claims, err := mw2.ValidateToken(token)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// TestValidateToken_ExpiredToken verifies that expired tokens are rejected.
func TestValidateToken_ExpiredToken(t *testing.T) {
	mw := setupJWTMiddleware()

	// Create a token that is already expired
	claims := &Claims{
		Username: "expireduser",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "catalog-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	result, err := mw.ValidateToken(tokenString)
	assert.Error(t, err)
	assert.Nil(t, result)
}

// TestRequireAuth_NoAuthorizationHeader verifies 401 when no Authorization header is set.
func TestRequireAuth_NoAuthorizationHeader(t *testing.T) {
	mw := setupJWTMiddleware()

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, false, body["success"])
	assert.Contains(t, body["error"], "Authorization header required")
}

// TestRequireAuth_InvalidHeaderFormat verifies 401 for malformed Authorization headers.
func TestRequireAuth_InvalidHeaderFormat(t *testing.T) {
	mw := setupJWTMiddleware()

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	tests := []struct {
		name   string
		header string
	}{
		{"no prefix", "some-token-string"},
		{"wrong prefix", "Token abc123"},
		{"basic auth", "Basic dXNlcjpwYXNz"},
		{"three parts", "Bearer abc 123"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			req.Header.Set("Authorization", tt.header)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// TestRequireAuth_InvalidToken verifies 401 when the token is invalid.
func TestRequireAuth_InvalidToken(t *testing.T) {
	mw := setupJWTMiddleware()

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_ExpiredToken_HTTP verifies 401 when the token is expired.
func TestRequireAuth_ExpiredToken_HTTP(t *testing.T) {
	mw := setupJWTMiddleware()

	// Create an already-expired token
	claims := &Claims{
		Username: "expireduser",
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
			Issuer:    "catalog-api",
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(testSecret))
	require.NoError(t, err)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestRequireAuth_ValidToken_SetsContext verifies that a valid token populates the gin context.
func TestRequireAuth_ValidToken_SetsContext(t *testing.T) {
	mw := setupJWTMiddleware()

	token, err := mw.GenerateToken("bob", "99", 24)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		username, exists := c.Get("username")
		assert.True(t, exists)
		assert.Equal(t, "bob", username)

		userID, exists := c.Get("user_id")
		assert.True(t, exists)
		assert.Equal(t, 99, userID) // Middleware converts string subject to int

		c.JSON(http.StatusOK, gin.H{"username": username, "user_id": userID})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	err = json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.Equal(t, "bob", body["username"])
	assert.Equal(t, float64(99), body["user_id"]) // JSON numbers decode as float64
}

// TestRequireAuth_ProtectedAndPublicRoutes verifies that protected and public routes coexist correctly.
func TestRequireAuth_ProtectedAndPublicRoutes(t *testing.T) {
	mw := setupJWTMiddleware()

	router := gin.New()
	router.GET("/public", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "public"})
	})
	router.GET("/protected", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "protected"})
	})

	// Public route should work without auth
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Protected route should fail without auth
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)

	// Protected route should work with valid auth
	token, err := mw.GenerateToken("user", "1", 24)
	require.NoError(t, err)

	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req3.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w3, req3)
	assert.Equal(t, http.StatusOK, w3.Code)
}

// TestRequireAuth_DifferentHTTPMethods verifies auth middleware works across HTTP methods.
func TestRequireAuth_DifferentHTTPMethods(t *testing.T) {
	mw := setupJWTMiddleware()

	token, err := mw.GenerateToken("user", "1", 24)
	require.NoError(t, err)

	router := gin.New()
	handler := func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"method": c.Request.Method})
	}

	router.GET("/resource", mw.RequireAuth(), handler)
	router.POST("/resource", mw.RequireAuth(), handler)
	router.PUT("/resource", mw.RequireAuth(), handler)
	router.DELETE("/resource", mw.RequireAuth(), handler)

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			// Without token: 401
			w := httptest.NewRecorder()
			req := httptest.NewRequest(method, "/resource", nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)

			// With token: 200
			w2 := httptest.NewRecorder()
			req2 := httptest.NewRequest(method, "/resource", nil)
			req2.Header.Set("Authorization", "Bearer "+token)
			router.ServeHTTP(w2, req2)
			assert.Equal(t, http.StatusOK, w2.Code)
		})
	}
}

// TestGenerateToken_DifferentExpirations verifies tokens with different expiration times.
func TestGenerateToken_DifferentExpirations(t *testing.T) {
	mw := setupJWTMiddleware()

	tests := []struct {
		name  string
		hours int
	}{
		{"1 hour", 1},
		{"24 hours", 24},
		{"168 hours (7 days)", 168},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token, err := mw.GenerateToken("user", "1", tt.hours)
			require.NoError(t, err)

			claims, err := mw.ValidateToken(token)
			require.NoError(t, err)

			expectedExpiry := time.Now().Add(time.Duration(tt.hours) * time.Hour)
			assert.InDelta(t, expectedExpiry.Unix(), claims.ExpiresAt.Unix(), 5)
		})
	}
}

// TestTokenTampering verifies that modifying any part of the token invalidates it.
func TestTokenTampering(t *testing.T) {
	mw := setupJWTMiddleware()

	token, err := mw.GenerateToken("user", "1", 24)
	require.NoError(t, err)

	// Tamper by flipping a character
	tampered := []byte(token)
	tampered[len(tampered)/2] ^= 0xFF
	tamperedStr := string(tampered)

	claims, err := mw.ValidateToken(tamperedStr)
	assert.Error(t, err)
	assert.Nil(t, claims)
}

// signTokenWithRole mints a JWT carrying the given role_id, signed
// with testSecret. Used by RequireAdmin tests below to simulate
// tokens minted by services/auth_service.go (which DOES carry
// role_id, unlike middleware.JWTMiddleware.GenerateToken which does
// not).
func signTokenWithRole(t *testing.T, username string, roleID int) string {
	t.Helper()
	claims := &Claims{
		Username: username,
		RoleID:   roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "catalog-api",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(testSecret))
	require.NoError(t, err)
	return signed
}

// TestRequireAdmin_AdminTokenAllowed asserts that a JWT carrying
// role_id == RoleAdminID passes through RequireAdmin.
//
// Article XI §11.2.5 anti-bluff anchor: comment out the
// RequireAdmin() line in main.go's adminGroup wiring and this test
// (paired with TestRequireAdmin_NonAdminForbidden) will fail —
// proving the gate is load-bearing.
func TestRequireAdmin_AdminTokenAllowed(t *testing.T) {
	mw := setupJWTMiddleware()
	token := signTokenWithRole(t, "admin", RoleAdminID)

	router := gin.New()
	router.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code, "admin role must reach the handler")
}

// TestRequireAdmin_NonAdminForbidden asserts that a JWT carrying
// any non-admin role (e.g. role_id == 2) is rejected with 403.
// Caught by FQA-API-010 in the 2026-04-29 real-binary bank
// verification.
func TestRequireAdmin_NonAdminForbidden(t *testing.T) {
	mw := setupJWTMiddleware()
	token := signTokenWithRole(t, "regular", 2)

	router := gin.New()
	router.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) {
		t.Fatal("handler must NOT execute for a non-admin caller")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code, "non-admin must be rejected with 403")
	assert.Contains(t, w.Body.String(), "Admin role required")
}

// TestRequireAdmin_NoTokenUnauthenticated asserts that the chain
// (RequireAuth → RequireAdmin) returns 401 when no token is
// present, NOT 403. The auth layer answers first.
func TestRequireAdmin_NoTokenUnauthenticated(t *testing.T) {
	mw := setupJWTMiddleware()

	router := gin.New()
	router.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) {
		t.Fatal("handler must NOT execute without auth")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"no token must be 401, not 403 — auth layer answers first")
}

// TestRequireAdmin_RoleZeroForbidden defends against the legacy
// JWTMiddleware.GenerateToken path that emits no role_id field
// (role_id defaults to 0). Tokens minted by GenerateToken MUST be
// rejected from /admin/* — they're for service-to-service or
// test-fixture use, not human admin sessions.
func TestRequireAdmin_RoleZeroForbidden(t *testing.T) {
	mw := setupJWTMiddleware()
	// GenerateToken does not set RoleID — claims.RoleID == 0.
	token, err := mw.GenerateToken("legacy-fixture", "42", 1)
	require.NoError(t, err)

	router := gin.New()
	router.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) {
		t.Fatal("handler must NOT execute for role_id == 0")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code,
		"role_id == 0 must be rejected — GenerateToken doesn't mint admin tokens")
}

// CAT-051 F4: with an AdminChecker installed the gate follows the checker (the
// database), not the role_id claim, and fails closed when it cannot decide.
func TestRequireAdmin_AdminCheckerOverridesClaim(t *testing.T) {
	run := func(checker AdminChecker, claimRole int) int {
		mw := setupJWTMiddleware()
		mw.SetAdminChecker(checker)
		token := signTokenWithRole(t, "u", claimRole)
		router := gin.New()
		router.GET("/admin/x", mw.RequireAuth(), mw.RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/admin/x", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(w, req)
		return w.Code
	}
	yes := func(context.Context, int) (bool, error) { return true, nil }
	no := func(context.Context, int) (bool, error) { return false, nil }
	boom := func(context.Context, int) (bool, error) { return false, errors.New("db down") }

	assert.Equal(t, http.StatusOK, run(yes, 2), "checker says admin: claim role_id 2 does not matter")
	assert.Equal(t, http.StatusForbidden, run(no, RoleAdminID), "checker says not admin: claim role_id 1 does not matter")
	assert.Equal(t, http.StatusServiceUnavailable, run(boom, RoleAdminID), "undeterminable must fail closed")
}

// N3: a non-integer user_id in the context (token subject not numeric) must
// fail closed when an AdminChecker is installed.
func TestRequireAdmin_NonIntUserIDFailsClosed(t *testing.T) {
	mw := setupJWTMiddleware()
	mw.SetAdminChecker(func(context.Context, int) (bool, error) { return true, nil })
	router := gin.New()
	router.GET("/admin/x", func(c *gin.Context) {
		c.Set("role_id", RoleAdminID)
		c.Set("user_id", "not-a-number")
	}, mw.RequireAdmin(), func(c *gin.Context) { c.Status(http.StatusOK) })
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/x", nil))
	assert.Equal(t, http.StatusForbidden, w.Code)
}
