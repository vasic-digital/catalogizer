package handlers

// ADMD1RR-md / ADMD1RR-ma: updating a user's username/email onto an existing one must answer 409 on SQLite AND on
// PostgreSQL (the update path used a SQLite-only text match and answered 500 on PostgreSQL), while a failing write
// that is NOT a unique violation (a foreign-key violation on role_id) must NOT be classified 409. Real gin router,
// real migrations, real UserRepository; the live PostgreSQL leg needs CAT_TEST_PG_DSN (honest SKIP otherwise).

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func dupUserUpdateChecks(t *testing.T, e *rtEnv, nulRejected bool) {
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
	mk("dupalice")
	bob := mk("dupbob")
	url := "/api/v1/users/" + rtItoa(bob)

	w := e.do(e.adminTok, "PUT", url, `{"username":"dupalice"}`)
	assert.Equal(t, 409, w.Code, "username onto an existing one: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Username or email already exists")
	w = e.do(e.adminTok, "PUT", url, `{"email":"dupalice@example.test"}`)
	assert.Equal(t, 409, w.Code, "email onto an existing one: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "Username or email already exists")

	// Second code path: a repository WITHOUT the guarded updater takes the plain Update branch of the handler.
	// The real repository and database stay underneath; only the optional UpdateGuarded method is hidden.
	plain := gin.New()
	plain.PUT("/api/v1/users/:id", WrapHTTPHandler(NewUserHandler(struct{ UserServiceInterface }{e.repo}, &AuthServiceAdapter{Inner: e.authSvc}).UpdateUser))
	for _, body := range []string{`{"username":"dupalice"}`, `{"email":"dupalice@example.test"}`} {
		req := httptest.NewRequest(http.MethodPut, url, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+e.adminTok)
		rec := httptest.NewRecorder()
		plain.ServeHTTP(rec, req)
		assert.Equal(t, 409, rec.Code, "plain Update branch, %s: %s", body, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "Username or email already exists")
	}

	// negative control: fresh values are accepted
	w = e.do(e.adminTok, "PUT", url, `{"username":"dupbob2","email":"dupbob2@example.test"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())

	// ADMD1RR-ma: a write that fails for a NON-unique reason must not be reported as a duplicate. A NUL byte in a
	// text column is rejected by PostgreSQL (invalid byte sequence, a non-unique SQLSTATE) and accepted by SQLite.
	w = e.do(e.adminTok, "PUT", url, `{"first_name":"nul\u0000byte"}`)
	assert.NotEqual(t, 409, w.Code, "non-unique failure classified as a duplicate: %s", w.Body.String())
	assert.NotContains(t, w.Body.String(), "already exists")
	if nulRejected {
		assert.Equal(t, 500, w.Code, "PostgreSQL must reject the NUL byte as a plain write failure: %s", w.Body.String())
	}
}

func TestADMD1RRmd_DuplicateUserUpdateIs409_SQLite(t *testing.T) {
	dupUserUpdateChecks(t, newRTEnv(t), false)
}

func TestADMD1RRmd_DuplicateUserUpdateIs409_PostgresLive(t *testing.T) {
	dupUserUpdateChecks(t, newRTEnvPG(t), true)
}

// Unit-class table for the shared classifier (doubles legal here, §11.4.289(C)(1)).
func TestADMD1RRmd_IsUniqueViolationTable(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"pq 23505", &pq.Error{Code: "23505", Message: "boom"}, true},
		{"wrapped pq 23505", fmt.Errorf("update: %w", &pq.Error{Code: "23505"}), true},
		{"pq text only", errors.New(`pq: duplicate key value violates unique constraint "users_username_key"`), true},
		{"sqlite text", errors.New("UNIQUE constraint failed: users.username"), true},
		{"pq fk 23503", &pq.Error{Code: "23503", Message: "fk"}, false},
		{"generic", errors.New("connection refused"), false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, isUniqueViolation(c.err), c.name)
	}
}
