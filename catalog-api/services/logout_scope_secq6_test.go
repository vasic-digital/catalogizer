package services

import (
	"testing"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SECQ-6 / AZ2-4: POST /auth/logout is public (the token is its own credential), so it must revoke exactly the session of
// the presented token: not another session of the same user, not another user's session, and a forged token must revoke
// nothing. Real AuthService over a real SQLite schema (same helpers as the other services integration tests).
func TestAuthService_Logout_RevokesOnlyThePresentedSession_SECQ6(t *testing.T) {
	db := setupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	svc := NewAuthService(userRepo, "test-secret-key-12345")
	setupAuthUser(t, userRepo, "testuser", "Password1!")

	hash, salt, err := svc.HashPasswordForUser("Password1!")
	require.NoError(t, err)
	otherID, err := userRepo.Create(&models.User{Username: "other", Email: "other@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
	require.NoError(t, err)
	require.NotZero(t, otherID)

	a1, err := svc.Login(models.LoginRequest{Username: "testuser", Password: "Password1!"}, "127.0.0.1", "A1")
	require.NoError(t, err)
	a2, err := svc.Login(models.LoginRequest{Username: "testuser", Password: "Password1!"}, "127.0.0.2", "A2")
	require.NoError(t, err)
	b1, err := svc.Login(models.LoginRequest{Username: "other", Password: "Password1!"}, "127.0.0.3", "B1")
	require.NoError(t, err)
	require.NotEqual(t, a1.SessionToken, a2.SessionToken)

	// Positive control: all three sessions are live before the logout.
	for name, tok := range map[string]string{"a1": a1.SessionToken, "a2": a2.SessionToken, "b1": b1.SessionToken} {
		_, err := svc.GetCurrentUser(tok)
		require.NoError(t, err, "control: "+name+" must be live before logout")
	}

	// A forged token (signed with another secret) revokes nothing.
	forged := NewAuthService(userRepo, "attacker-secret")
	fa, err := forged.Login(models.LoginRequest{Username: "testuser", Password: "Password1!"}, "6.6.6.6", "evil")
	require.NoError(t, err)
	assert.Error(t, svc.Logout(fa.SessionToken), "a token signed with another secret must be rejected")
	for _, tok := range []string{a1.SessionToken, a2.SessionToken, b1.SessionToken} {
		_, err := svc.GetCurrentUser(tok)
		assert.NoError(t, err, "a rejected logout must not revoke any session")
	}

	// The real logout revokes a1 only.
	require.NoError(t, svc.Logout(a1.SessionToken))
	_, err = svc.GetCurrentUser(a1.SessionToken)
	assert.Error(t, err, "the presented session is revoked")
	_, err = svc.GetCurrentUser(a2.SessionToken)
	assert.NoError(t, err, "another session of the same user survives")
	_, err = svc.GetCurrentUser(b1.SessionToken)
	assert.NoError(t, err, "another user's session survives")
}
