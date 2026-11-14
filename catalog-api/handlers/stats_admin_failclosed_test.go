package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SSE-F1: the admin decision in StatsHandler.GetScanHistory must fail closed. The real handler,
// the real StatsRepository and the production-migrated SQLite schema are used; only the identity
// claims (user_id / role_id, normally set by the JWT middleware) are injected.
const sseSecretErr = "smb://nas.internal/share failed for user bob"

type sseClaims struct {
	setUser bool
	userID  interface{}
	setRole bool
	roleID  interface{}
}

func sseScanHistory(t *testing.T, checker func(context.Context, int) (bool, error), cl sseClaims) string {
	t.Helper()
	h := newAPIHarness(t, false)
	_, err := h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, path, enabled) VALUES (1, 'r', 'local', '/x', 1)`)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO scan_history (storage_root_id, scan_type, status, start_time, error_message) VALUES (1, 'full', 'failed', CURRENT_TIMESTAMP, ?)`, sseSecretErr)
	require.NoError(t, err)

	sh := NewStatsHandler(nil, repository.NewStatsRepository(h.db))
	if checker != nil {
		sh.WithAdminChecker(checker)
	}
	r := gin.New()
	r.GET("/scans", func(c *gin.Context) {
		if cl.setUser {
			c.Set("user_id", cl.userID)
		}
		if cl.setRole {
			c.Set("role_id", cl.roleID)
		}
	}, sh.GetScanHistory)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/scans", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var out struct {
		Data struct {
			Scans []struct {
				ErrorMessage *string `json:"error_message"`
			} `json:"scans"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	require.Len(t, out.Data.Scans, 1)
	require.NotNil(t, out.Data.Scans[0].ErrorMessage)
	return *out.Data.Scans[0].ErrorMessage
}

func TestSSEF1_StatsAdmin_CheckerErrorFailsClosed(t *testing.T) {
	// (a) the checker returns admin=true AND an error: the response must still be redacted.
	got := sseScanHistory(t, func(context.Context, int) (bool, error) { return true, errors.New("db down") },
		sseClaims{setUser: true, userID: 5, setRole: true, roleID: 1})
	assert.Equal(t, scanErrorClassText, got)
	assert.NotContains(t, got, "nas.internal")
}

func TestSSEF1_StatsAdmin_CheckerAdminSeesRawText(t *testing.T) { // positive control: the redaction is not unconditional
	got := sseScanHistory(t, func(context.Context, int) (bool, error) { return true, nil },
		sseClaims{setUser: true, userID: 5})
	assert.Equal(t, sseSecretErr, got)
}

func TestSSEF1_StatsAdmin_CheckerSaysNonAdminRedacts(t *testing.T) {
	// the live checker wins over a role_id=1 claim in the token.
	got := sseScanHistory(t, func(context.Context, int) (bool, error) { return false, nil },
		sseClaims{setUser: true, userID: 5, setRole: true, roleID: 1})
	assert.Equal(t, scanErrorClassText, got)
}

func TestSSEF1_StatsAdmin_MissingOrInvalidUserIDRedacts(t *testing.T) {
	always := func(context.Context, int) (bool, error) { return true, nil }
	for name, cl := range map[string]sseClaims{
		"missing":    {},
		"zero":       {setUser: true, userID: 0},
		"negative":   {setUser: true, userID: -3},
		"string":     {setUser: true, userID: "5"},
		"int64 zero": {setUser: true, userID: int64(0)},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, scanErrorClassText, sseScanHistory(t, always, cl))
		})
	}
	// int64 positive is accepted (JWT middleware may set either width).
	assert.Equal(t, sseSecretErr, sseScanHistory(t, always, sseClaims{setUser: true, userID: int64(7)}))
}

// No checker wired: the documented fallback trusts role_id only when it is the int 1 (the seeded
// admin role). Everything else (other roles, other types, absent) fails closed.
func TestSSEF1_StatsAdmin_NoCheckerRoleIDFallback(t *testing.T) {
	assert.Equal(t, sseSecretErr, sseScanHistory(t, nil, sseClaims{setRole: true, roleID: 1}))
	for name, cl := range map[string]sseClaims{
		"role 2":        {setRole: true, roleID: 2},
		"role 0":        {setRole: true, roleID: 0},
		"role absent":   {},
		"role string 1": {setRole: true, roleID: "1"},
		"role int64 1":  {setRole: true, roleID: int64(1)},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, scanErrorClassText, sseScanHistory(t, nil, cl))
		})
	}
}
