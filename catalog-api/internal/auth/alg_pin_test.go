package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// HS384/HS512 tokens signed with the REAL secret were accepted by the
// SigningMethodHMAC family check; production only ever issues HS256
// (jwtmod.DefaultConfig), so validation must pin HS256 exactly.
func TestValidateToken_PinsHS256(t *testing.T) {
	const secret = "unit-test-secret-not-a-credential"
	svc := NewAuthService(nil, secret, zap.NewNop())
	now := time.Now()
	claims := jwt.MapClaims{
		"user_id": 1, "username": "mallory", "role": "admin", "type": "access",
		"iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
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
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, claims).SignedString(jwt.UnsafeAllowNoneSignatureType)
	require.NoError(t, err)
	_, err = svc.validateToken(none)
	assert.Error(t, err, "alg=none must be rejected")
}
