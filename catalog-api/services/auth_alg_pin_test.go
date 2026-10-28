package services

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServicesValidateToken_PinsHS256(t *testing.T) {
	const secret = "unit-test-secret-not-a-credential"
	svc := &AuthService{jwtSecret: []byte(secret)}
	claims := &JWTClaims{
		UserID: 1, Username: "mallory", RoleID: 1,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	sign := func(m jwt.SigningMethod) string {
		s, err := jwt.NewWithClaims(m, claims).SignedString([]byte(secret))
		require.NoError(t, err)
		return s
	}
	got, err := svc.validateToken(sign(jwt.SigningMethodHS256))
	require.NoError(t, err, "control: HS256 must validate")
	assert.Equal(t, "mallory", got.Username)
	for _, m := range []jwt.SigningMethod{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		_, err := svc.validateToken(sign(m))
		assert.Error(t, err, "%s token must be rejected", m.Alg())
	}
}
