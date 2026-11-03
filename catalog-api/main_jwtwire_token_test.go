package main

import (
	"testing"
	"time"

	root_middleware "catalogizer/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// mintTestToken signs an access token like the auth service does.
func mintTestToken(t *testing.T, _ *root_middleware.JWTMiddleware, sub string, roleID int, sessionID string) string {
	t.Helper()
	claims := &root_middleware.Claims{Username: "u", RoleID: roleID, SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{Subject: sub, ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)), Issuer: "catalog-api"}}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("wire-test-secret"))
	require.NoError(t, err)
	return signed
}
