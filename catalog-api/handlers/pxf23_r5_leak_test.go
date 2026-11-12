package handlers

// CAT-PXF23-R5: no client-visible response from the auth/user handlers may carry the text of an internal
// error (DB driver text, wrapped %w chains, binding internals). Real stack (newRTEnv: production migrations,
// real repository + AuthService + handlers). A store failure is injected with a SQLite trigger that raises a
// driver-style marker, so the failure travels the production error-wrapping path.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const r5Marker = "pq: connection refused host=db"

func (e *rtEnv) r5Break(t *testing.T, op, table string) {
	t.Helper()
	_, err := e.db.Exec("CREATE TRIGGER r5_break_" + op + "_" + table + " BEFORE " + op + " ON " + table +
		" BEGIN SELECT RAISE(ABORT, '" + r5Marker + "'); END;")
	require.NoError(t, err)
}

func r5NoLeak(t *testing.T, w *httptest.ResponseRecorder, extra ...string) {
	t.Helper()
	all := w.Body.String()
	for k, v := range w.Header() {
		all += "\n" + k + ": " + strings.Join(v, ",")
	}
	for _, bad := range append([]string{r5Marker, "connection refused", "failed to ", "no such table", "UNIQUE constraint", "Field validation", "Key: '"}, extra...) {
		assert.NotContains(t, all, bad, "internal text %q reached the client (status %d, body %q)", bad, w.Code, w.Body.String())
	}
}

func (e *rtEnv) r5Other(t *testing.T) {
	t.Helper()
	hash, salt, err := e.authSvc.HashPasswordForUser(azPass)
	require.NoError(t, err)
	_, err = e.repo.Create(&models.User{Username: "r5other", Email: "r5other@example.test", PasswordHash: hash, Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)
}

func r5Routes(e *rtEnv) {
	ah := NewAuthHandler(e.authSvc)
	e.r.POST("/r5/refresh", ah.RefreshTokenGin)
	e.r.POST("/r5/logout", ah.LogoutGin)
	e.r.POST("/r5/register", func(c *gin.Context) { ah.RegisterGin(c, e.repo) })
	e.r.POST("/r5/std/login", WrapHTTPHandler(ah.Login))
	e.r.POST("/r5/std/refresh", WrapHTTPHandler(ah.RefreshToken))
	e.r.POST("/r5/std/logout", WrapHTTPHandler(ah.Logout))
	e.r.POST("/r5/std/logout-all", WrapHTTPHandler(ah.LogoutAll))
	e.r.POST("/r5/std/change-password", WrapHTTPHandler(ah.ChangePassword))
	e.r.GET("/r5/std/sessions", WrapHTTPHandler(ah.GetActiveSessions))
	e.r.POST("/r5/std/deactivate", WrapHTTPHandler(ah.DeactivateSession))
	uh := NewUserHandler(e.repo, &AuthServiceAdapter{Inner: e.authSvc})
	e.r.POST("/r5/users/:id/lock", WrapHTTPHandler(uh.LockAccount))
	e.r.POST("/r5/users/:id/unlock", WrapHTTPHandler(uh.UnlockAccount))
}

func (e *rtEnv) r5LoginJSON(t *testing.T) (access, refresh string) {
	t.Helper()
	w := e.login("rtadmin", rtAdminPass)
	require.Equal(t, 200, w.Code, w.Body.String())
	return jsonField(t, w, "session_token"), jsonField(t, w, "refresh_token")
}

func jsonField(t *testing.T, w *httptest.ResponseRecorder, k string) string {
	t.Helper()
	i := strings.Index(w.Body.String(), `"`+k+`":"`)
	require.True(t, i >= 0, w.Body.String())
	rest := w.Body.String()[i+len(k)+4:]
	return rest[:strings.Index(rest, `"`)]
}

func TestPXF23R5_StoreFailuresNeverLeakInternalText(t *testing.T) {
	type tc struct {
		name  string
		setup func(e *rtEnv, tok, ref string)
		call  func(e *rtEnv, tok, ref string) *httptest.ResponseRecorder
		want  int
	}
	pw := `{"username":"rtadmin","password":"` + rtAdminPass + `"}`
	cases := []tc{
		{"gin login: session insert fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "INSERT", "user_sessions") },
			func(e *rtEnv, _, _ string) *httptest.ResponseRecorder {
				return e.do("", "POST", "/api/v1/auth/login", pw)
			}, 401},
		{"std login: session insert fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "INSERT", "user_sessions") },
			func(e *rtEnv, _, _ string) *httptest.ResponseRecorder { return e.do("", "POST", "/r5/std/login", pw) }, 401},
		{"gin refresh: session update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, _, ref string) *httptest.ResponseRecorder {
				return e.do("", "POST", "/r5/refresh", `{"refresh_token":"`+ref+`"}`)
			}, 401},
		{"std refresh: session update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, _, ref string) *httptest.ResponseRecorder {
				return e.do("", "POST", "/r5/std/refresh", `{"refresh_token":"`+ref+`"}`)
			}, 401},
		{"gin logout: session update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder { return e.do(tok, "POST", "/r5/logout", ``) }, 500},
		{"std logout: session update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/std/logout", ``)
			}, 500},
		{"std logout-all: session update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/std/logout-all", ``)
			}, 500},
		{"std change-password: user update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "users") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/std/change-password", `{"current_password":"`+rtAdminPass+`","new_password":"N3w!Passw0rd#x"}`)
			}, 500},
		{"std sessions list: unreadable row", func(e *rtEnv, _, _ string) {
			_, err := e.db.Exec("INSERT INTO user_sessions (user_id, session_token, is_active, expires_at) VALUES (1, 'bad-row', 1, 'zzz-not-a-time')")
			require.NoError(t, err)
		}, func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
			return e.do(tok, "GET", "/r5/std/sessions", ``)
		}, 500},
		{"std deactivate session: update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "user_sessions") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/std/deactivate?session_id=1", ``)
			}, 500},
		{"create user: insert fails (500, was details)", func(e *rtEnv, _, _ string) { e.r5Break(t, "INSERT", "users") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/api/v1/users", createUserBody("bob"))
			}, 500},
		{"create user: duplicate (409, was details)", func(e *rtEnv, _, _ string) {},
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/api/v1/users", createUserBody("rtadmin"))
			}, 409},
		{"reset password: update fails", func(e *rtEnv, _, _ string) { e.r5Break(t, "UPDATE", "users") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/api/v1/users/1/reset-password", `{"new_password":"N3w!Passw0rd#x"}`)
			}, 500},
		{"lock account: update fails", func(e *rtEnv, _, _ string) { e.r5Other(t); e.r5Break(t, "UPDATE", "users") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/users/2/lock", `{"lock_until":"2099-01-01T00:00:00Z"}`)
			}, 500},
		{"unlock account: update fails", func(e *rtEnv, _, _ string) { e.r5Other(t); e.r5Break(t, "UPDATE", "users") },
			func(e *rtEnv, tok, _ string) *httptest.ResponseRecorder {
				return e.do(tok, "POST", "/r5/users/2/unlock", ``)
			}, 500},
		{"register: binding error", func(e *rtEnv, _, _ string) {},
			func(e *rtEnv, _, _ string) *httptest.ResponseRecorder {
				return e.do("", "POST", "/r5/register", `{"username":"x","email":"not-an-email","password":"short"}`)
			}, 400},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := newRTEnv(t)
			r5Routes(e)
			tok, ref := e.r5LoginJSON(t)
			c.setup(e, tok, ref)
			w := c.call(e, tok, ref)
			t.Logf("%s -> %d %q", c.name, w.Code, w.Body.String())
			r5NoLeak(t, w, "parsing time", "Scan error", "unsupported Scan", "bad-row", "zzz-not-a-time")
			if c.want != 0 {
				assert.Equal(t, c.want, w.Code, "status must be unchanged by the fix: %s", w.Body.String())
			}
		})
	}
}

// Positive control: legitimate, intentionally distinguishable messages are preserved.
func TestPXF23R5_IntentionalMessagesPreserved(t *testing.T) {
	e := newRTEnv(t)
	r5Routes(e)
	w := e.login("rtadmin", "Wrong-Pass1!")
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid credentials")
	w = e.do("", "POST", "/r5/refresh", `{"refresh_token":"does-not-exist"}`)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "invalid refresh token")
	tok, _ := e.r5LoginJSON(t)
	w = e.do(tok, "POST", "/r5/std/change-password", `{"current_password":"`+rtAdminPass+`","new_password":"short"}`)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "password must be at least 8 characters long")
	w = e.do(tok, "POST", "/r5/std/change-password", `{"current_password":"Not-The-Pass1!","new_password":"N3w!Passw0rd#x"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "current password is incorrect")
	w = e.do(tok, "POST", "/api/v1/users", createUserBody("rtadmin"))
	assert.Equal(t, 409, w.Code)
	assert.Contains(t, w.Body.String(), "Username or email already exists")
}
