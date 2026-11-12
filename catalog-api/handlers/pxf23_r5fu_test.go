package handlers

// CAT-R5FU-FU: authClientMessage must return the matched sentinel's OWN text (or the policy error's Msg),
// never the text of the wrapper chain around it. authClientMessage is the single choke point used by the
// gin handlers (LoginGin, RefreshTokenGin) and the net/http handlers (Login, RefreshToken, ChangePassword).

import (
	"errors"
	"fmt"
	"testing"

	"catalogizer/services"

	"github.com/stretchr/testify/assert"
)

func TestR5FUFA_WrappedSentinelReturnsOwnTextOnly(t *testing.T) {
	const internal = "pq: connection refused host=db password_hash=$2a$10$abcdefghijklmnopqrstuv"
	sentinels := []error{
		services.ErrInvalidCredentials, services.ErrAccountLocked, services.ErrAccountDisabled,
		services.ErrAccountPending, services.ErrUserChangedDuringLogin, services.ErrInvalidRefreshToken,
		services.ErrSessionExpired, services.ErrCurrentPasswordIncorrect,
	}
	for _, s := range sentinels {
		wrapped := fmt.Errorf("lookup failed (%s): %w", internal, s)
		joined := errors.Join(errors.New(internal), s)
		for name, e := range map[string]error{"wrapped": wrapped, "joined": joined} {
			msg, known := authClientMessage(e, "fallback")
			assert.True(t, known, "%s %v", name, s)
			assert.Equal(t, s.Error(), msg, "%s %v: only the sentinel's own text may reach the client", name, s)
		}
	}
}

func TestR5FUFA_WrappedPasswordPolicyReturnsMsgOnly(t *testing.T) {
	const internal = "pq: connection refused password_hash=$2a$10$zzzzzzzzzzzzzzzzzzzzzz"
	pp := &services.PasswordPolicyError{Msg: "password must contain at least one digit"}
	msg, known := authClientMessage(fmt.Errorf("validate: %s: %w", internal, pp), "fallback")
	assert.True(t, known)
	assert.Equal(t, "password must contain at least one digit", msg)
}

func TestR5FUFA_UnknownErrorStillFallsBack(t *testing.T) {
	msg, known := authClientMessage(errors.New("pq: connection refused"), "fallback")
	assert.False(t, known)
	assert.Equal(t, "fallback", msg)
}

// R5FU-F-C: ErrSessionNotFound is not a client-visible auth error for these handlers.
func TestR5FUFC_SessionNotFoundFallsBack(t *testing.T) {
	msg, known := authClientMessage(services.ErrSessionNotFound, "fallback")
	assert.False(t, known)
	assert.Equal(t, "fallback", msg)
}
