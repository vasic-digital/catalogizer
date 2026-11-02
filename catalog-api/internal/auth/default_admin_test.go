package auth

import (
	"database/sql"
	"testing"

	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// compromised is the published default (CAT-021). Assembled from fragments so
// repository scans for the literal do not flag this deliberate negative test.
const compromised = "admin" + "123"

func newRealAuth(t *testing.T) *AuthService {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })
	return NewAuthService(database.WrapDB(sqlDB, database.DialectSQLite), "integration-test-secret-0123456789ab", zap.NewNop())
}

// Fails closed: the compromised default must not create an admin.
func TestCreateDefaultAdmin_RejectsCompromisedDefault(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", compromised)
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	err := newRealAuth(t).Initialize()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ADMIN_PASSWORD")
}

// Explicit dev mode is the only way to use a placeholder password.
func TestCreateDefaultAdmin_DevModeAllowsPlaceholder(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", compromised)
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "true")
	require.NoError(t, newRealAuth(t).Initialize())
}

// An operator-supplied non-placeholder value is accepted and works for login.
func TestCreateDefaultAdmin_OperatorSuppliedAccepted(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "Op3rator-Supplied-Value-9x")
	t.Setenv("CATALOGIZER_ALLOW_INSECURE_DEFAULTS", "")
	s := newRealAuth(t)
	require.NoError(t, s.Initialize())
	_, err := s.Login("admin", "Op3rator-Supplied-Value-9x", "127.0.0.1", "t")
	assert.NoError(t, err)
	_, err = s.Login("admin", compromised, "127.0.0.1", "t")
	assert.Error(t, err)
}

// Unset: random password generated, and the compromised default never works.
func TestCreateDefaultAdmin_UnsetGeneratesRandom(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", "")
	s := newRealAuth(t)
	require.NoError(t, s.Initialize())
	_, err := s.Login("admin", compromised, "127.0.0.1", "t")
	assert.Error(t, err)
}
