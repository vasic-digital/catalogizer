package handlers

// ADMD1RRR-I1: PUT /api/v1/admin/users/:id onto an email that already belongs to another user must answer 409 on
// SQLite AND PostgreSQL (both the UpdateGuarded branch and the plain Update branch mapped it to 500). Real gin
// router, real migrations, real UserRepository; the live PostgreSQL leg needs CAT_TEST_PG_DSN (honest SKIP otherwise).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dupAdminUpdateChecks(t *testing.T, e *rtEnv) {
	mk := func(name string) int {
		w := e.do(e.adminTok, "POST", "/api/v1/users",
			fmt.Sprintf(`{"username":%q,"email":%q,"password":"Pw!Throwaway#9x","role_id":2}`, name, name+"@example.test"))
		require.Equal(t, 201, w.Code, w.Body.String())
		var u struct {
			ID int `json:"id"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &u))
		return u.ID
	}
	mk("admdupalice")
	bob := mk("admdupbob")
	url := "/api/v1/admin/users/" + rtItoa(bob)

	// guarded branch (the real repository implements UpdateGuarded)
	w := e.do(e.adminTok, "PUT", url, `{"email":"admdupalice@example.test"}`)
	assert.Equal(t, 409, w.Code, "guarded branch, email onto an existing one: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Username or email already exists")

	// plain Update branch: hide the optional UpdateGuarded method; real repository and database stay underneath
	plain := gin.New()
	plain.PUT("/api/v1/admin/users/:id", NewAdminHandler(&AuthServiceAdapter{Inner: e.authSvc},
		struct{ AdminUserRepoInterface }{e.repo}, e.db, "test").UpdateUser)
	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader(`{"email":"admdupalice@example.test"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec := httptest.NewRecorder()
	plain.ServeHTTP(rec, req)
	assert.Equal(t, 409, rec.Code, "plain Update branch, email onto an existing one: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Username or email already exists")

	// negative control: a fresh email is accepted
	w = e.do(e.adminTok, "PUT", url, `{"email":"admdupbob2@example.test"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
}

func adminNonUniqueChecks(t *testing.T, e *rtEnv, nulRejected bool) {
	w := e.do(e.adminTok, "POST", "/api/v1/users", `{"username":"admnul","email":"admnul@example.test","password":"Pw!Throwaway#9x","role_id":2}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var u struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &u))
	url := "/api/v1/admin/users/" + rtItoa(u.ID)
	body := `{"email":"nul\u0000x@example.test"}`
	plain := gin.New()
	plain.PUT("/api/v1/admin/users/:id", NewAdminHandler(&AuthServiceAdapter{Inner: e.authSvc},
		struct{ AdminUserRepoInterface }{e.repo}, e.db, "test").UpdateUser)
	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec := httptest.NewRecorder()
	plain.ServeHTTP(rec, req)
	for name, got := range map[string]*httptest.ResponseRecorder{"guarded": e.do(e.adminTok, "PUT", url, body), "plain": rec} {
		assert.NotEqual(t, 409, got.Code, "%s: non-unique failure classified as a duplicate: %s", name, got.Body.String())
		assert.NotContains(t, got.Body.String(), "already exists", name)
		if nulRejected {
			assert.Equal(t, 500, got.Code, "%s: PostgreSQL rejects the NUL byte as a plain write failure: %s", name, got.Body.String())
		}
	}
}

func TestADMD1RRRi1_AdminNonUniqueFailureIsNot409_SQLite(t *testing.T) {
	adminNonUniqueChecks(t, newRTEnv(t), false)
}

func TestADMD1RRRi1_AdminNonUniqueFailureIsNot409_PostgresLive(t *testing.T) {
	adminNonUniqueChecks(t, newRTEnvPG(t), true)
}

func TestADMD1RRRi1_AdminDuplicateEmailUpdateIs409_SQLite(t *testing.T) {
	dupAdminUpdateChecks(t, newRTEnv(t))
}

func TestADMD1RRRi1_AdminDuplicateEmailUpdateIs409_PostgresLive(t *testing.T) {
	dupAdminUpdateChecks(t, newRTEnvPG(t))
}
