package handlers

// D9 review round 1, finding 1: the last-admin protection must be atomic
// across every route that can remove an administrator's admin-ness or
// activity (admin PUT demote/deactivate, PUT /users/:id is_active, DELETE).

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/require"
)

func (e *d9Env) activeAdmins(t *testing.T) int {
	var n int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role_id = 1 AND is_active = 1 AND is_locked = 0`).Scan(&n))
	return n
}

func (e *d9Env) tokenFor(t *testing.T, user, pass string) string {
	w := e.login(user, pass)
	require.Equal(t, 200, w.Code, w.Body.String())
	var res struct {
		SessionToken string `json:"session_token"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &res))
	return res.SessionToken
}

// resetTwoAdmins makes rtadmin and dave active administrators again (dave is
// recreated when a previous iteration deleted him) and returns fresh tokens.
func (e *d9Env) resetTwoAdmins(t *testing.T) (aTok, bTok string, aID, bID int) {
	for _, u := range []struct{ name, pass string }{{"rtadmin", rtAdminPass}, {"dave", rtUserPass}} {
		if _, gerr := e.repo.GetByUsername(u.name); gerr != nil {
			hash, salt, herr := e.authSvc.HashPasswordForUser(u.pass)
			require.NoError(t, herr)
			_, cerr := e.repo.Create(&models.User{Username: u.name, Email: u.name + "@example.test", PasswordHash: hash, Salt: salt, RoleID: 1, IsActive: true})
			require.NoError(t, cerr)
		}
	}
	_, err := e.db.Exec(`UPDATE users SET role_id = 1, is_active = 1, is_locked = 0, failed_login_attempts = 0 WHERE username IN ('rtadmin','dave')`)
	require.NoError(t, err)
	_, err = e.db.Exec(`UPDATE user_sessions SET is_active = 0`)
	require.NoError(t, err)
	a, _ := e.repo.GetByUsername("rtadmin")
	b, _ := e.repo.GetByUsername("dave")
	return e.tokenFor(t, "rtadmin", rtAdminPass), e.tokenFor(t, "dave", rtUserPass), a.ID, b.ID
}

func TestD9_ConcurrentLastAdminNeverZero(t *testing.T) {
	e := newD9Env(t)
	e.r.DELETE("/api/v1/users/:id", WrapHTTPHandler(NewUserHandler(e.repo, &AuthServiceAdapter{Inner: e.authSvc}).DeleteUser))
	type req struct{ tok, method, url, body string }
	variants := map[string]func(aTok, bTok string, aID, bID int) (req, req){
		"admin_put_demote_both": func(aTok, bTok string, aID, bID int) (req, req) {
			return req{aTok, "PUT", "/api/v1/admin/users/" + rtItoa(bID), `{"role":"user"}`},
				req{bTok, "PUT", "/api/v1/admin/users/" + rtItoa(aID), `{"role":"user"}`}
		},
		"admin_put_demote_vs_users_deactivate": func(aTok, bTok string, aID, bID int) (req, req) {
			return req{aTok, "PUT", "/api/v1/admin/users/" + rtItoa(bID), `{"role":"user"}`},
				req{bTok, "PUT", "/api/v1/users/" + rtItoa(aID), `{"is_active":false}`}
		},
		"admin_put_deactivate_both": func(aTok, bTok string, aID, bID int) (req, req) {
			return req{aTok, "PUT", "/api/v1/admin/users/" + rtItoa(bID), `{"status":"inactive"}`},
				req{bTok, "PUT", "/api/v1/admin/users/" + rtItoa(aID), `{"status":"inactive"}`}
		},
		"delete_vs_admin_put_demote": func(aTok, bTok string, aID, bID int) (req, req) {
			return req{aTok, "DELETE", "/api/v1/users/" + rtItoa(bID), ``},
				req{bTok, "PUT", "/api/v1/admin/users/" + rtItoa(aID), `{"role":"user"}`}
		},
		"delete_both": func(aTok, bTok string, aID, bID int) (req, req) {
			return req{aTok, "DELETE", "/api/v1/users/" + rtItoa(bID), ``},
				req{bTok, "DELETE", "/api/v1/users/" + rtItoa(aID), ``}
		},
	}
	for name, mk := range variants {
		name, mk := name, mk
		t.Run(name, func(t *testing.T) {
			zero, both200 := 0, 0
			for i := 0; i < 12; i++ {
				aTok, bTok, aID, bID := e.resetTwoAdmins(t)
				r1, r2 := mk(aTok, bTok, aID, bID)
				var wg sync.WaitGroup
				start := make(chan struct{})
				codes := make([]int, 2)
				for k, rq := range []req{r1, r2} {
					wg.Add(1)
					go func(k int, rq req) {
						defer wg.Done()
						<-start
						codes[k] = e.do(rq.tok, rq.method, rq.url, rq.body).Code
					}(k, rq)
				}
				close(start)
				wg.Wait()
				if e.activeAdmins(t) == 0 {
					zero++
				}
				if codes[0] < 300 && codes[1] < 300 {
					both200++
				}
			}
			require.Zero(t, zero, "%s: iterations ending with ZERO active administrators (both succeeded in %d)", strings.ToUpper(name), both200)
		})
	}
}
