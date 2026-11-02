package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recUserRepo struct {
	UserServiceInterface
	got map[string]string
}

func (s *recUserRepo) GetByID(id int) (*models.User, error) {
	s.got["GetByID"] = strconv.Itoa(id)
	return &models.User{ID: id}, nil
}
func (s *recUserRepo) Update(u *models.User) error { s.got["Update"] = strconv.Itoa(u.ID); return nil }
func (s *recUserRepo) Delete(id int) error         { s.got["Delete"] = strconv.Itoa(id); return nil }
func (s *recUserRepo) GetRole(int) (*models.Role, error) {
	return &models.Role{}, nil
}

type recUserAuth struct {
	UserAuthServiceInterface
	got map[string]string
}

func (s *recUserAuth) CheckPermission(int, string) (bool, error) { return true, nil }
func (s *recUserAuth) GetCurrentUser(string) (*models.User, error) {
	return &models.User{ID: 1}, nil
}
func (s *recUserAuth) ValidatePassword(string) error { return nil }
func (s *recUserAuth) ResetPassword(id int, _ string) error {
	s.got["ResetPassword"] = strconv.Itoa(id)
	return nil
}
func (s *recUserAuth) LockAccount(id int, _ time.Time) error {
	s.got["LockAccount"] = strconv.Itoa(id)
	return nil
}
func (s *recUserAuth) UnlockAccount(id int) error {
	s.got["UnlockAccount"] = strconv.Itoa(id)
	return nil
}

type recRoleRepo struct {
	RoleUserServiceInterface
	got map[string]string
}

func (s *recRoleRepo) GetRole(id int) (*models.Role, error) {
	s.got["GetRole"] = strconv.Itoa(id)
	return &models.Role{ID: id}, nil
}
func (s *recRoleRepo) UpdateRole(r *models.Role) error {
	s.got["UpdateRole"] = strconv.Itoa(r.ID)
	return nil
}
func (s *recRoleRepo) DeleteRole(id int) error { s.got["DeleteRole"] = strconv.Itoa(id); return nil }

type recRoleAuth struct{}

func (recRoleAuth) CheckPermission(int, string) (bool, error) { return true, nil }
func (recRoleAuth) GetCurrentUser(string) (*models.User, error) {
	return &models.User{ID: 1}, nil
}

// Real UserHandler/RoleHandler behind the real adapter on the real /api/v1
// prefixed routes (same shapes as main.go). The handlers must derive the id
// from the routed path, not from a hard-coded unversioned "/api/users/" prefix.
func TestUserRoleHandlers_ResolveIDsOnVersionedRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	got := map[string]string{}
	uh := NewUserHandler(&recUserRepo{got: got}, &recUserAuth{got: got})
	rh := NewRoleHandler(&recRoleRepo{got: got}, recRoleAuth{})

	type rt struct {
		method, pattern, url, svc, body string
		h                               func(http.ResponseWriter, *http.Request)
	}
	routes := []rt{
		{"GET", "/api/v1/users/:id", "/api/v1/users/5", "GetByID", "", uh.GetUser},
		{"PUT", "/api/v1/users/:id", "/api/v1/users/5", "GetByID", `{}`, uh.UpdateUser},
		{"DELETE", "/api/v1/users/:id", "/api/v1/users/5", "Delete", "", uh.DeleteUser},
		{"POST", "/api/v1/users/:id/reset-password", "/api/v1/users/5/reset-password", "ResetPassword", `{"new_password":"x"}`, uh.ResetPassword},
		{"POST", "/api/v1/users/:id/lock", "/api/v1/users/5/lock", "LockAccount", `{"lock_until":"2030-01-01T00:00:00Z"}`, uh.LockAccount},
		{"POST", "/api/v1/users/:id/unlock", "/api/v1/users/5/unlock", "UnlockAccount", "", uh.UnlockAccount},
		{"GET", "/api/v1/roles/:id", "/api/v1/roles/5", "GetRole", "", rh.GetRole},
		{"PUT", "/api/v1/roles/:id", "/api/v1/roles/5", "UpdateRole", `{"name":"n"}`, rh.UpdateRole},
		{"DELETE", "/api/v1/roles/:id", "/api/v1/roles/5", "DeleteRole", "", rh.DeleteRole},
	}
	r := gin.New()
	for _, x := range routes {
		r.Handle(x.method, x.pattern, WrapHTTPHandler(x.h))
	}
	for _, x := range routes {
		t.Run(x.method+" "+x.pattern, func(t *testing.T) {
			delete(got, x.svc)
			req := httptest.NewRequest(x.method, x.url, strings.NewReader(x.body))
			req.Header.Set("Authorization", "Bearer t")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusBadRequest, w.Code, w.Body.String())
			assert.Equal(t, "5", got[x.svc])
		})
	}
}

// Sweep gate: no handler may derive ids from r.URL.Path by trimming a
// hard-coded "/api/" prefix or by splitting / path.Base-ing the raw path
// (breaks under /api/v1 and any group move). routedID (path_params.go) is the
// single allowed reader of r.URL.Path.
var rawPathParseRe = regexp.MustCompile(`\.URL\.(Path|RawPath|EscapedPath\(\))`)

func TestNoHardcodedAPIPrefixPathParsing(t *testing.T) {
	// control needle: the regex must match synthetic positives and not a negative
	for _, pos := range []string{
		`strings.TrimPrefix(r.URL.Path, "/api/users/")`,
		`strings.Split(req.URL.Path, "/")`,
		`path.Base(httpReq.URL.Path)`,
		`strings.Split(strings.Trim(r.URL.Path, "/"), "/")`,
		`x := r.URL.RawPath`,
		`y := q.URL.EscapedPath()`,
	} {
		require.True(t, rawPathParseRe.MatchString(pos), "regex must match %q", pos)
	}
	require.False(t, rawPathParseRe.MatchString(`mux.Vars(r)["id"]`))
	files := handlerFiles(t)
	require.NotEmpty(t, files)
	scanned := 0
	sawAllowed := false
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		if filepath.Base(f) == "path_params.go" {
			// routedID is the one sanctioned reader; the scan must SEE it (positive needle)
			require.True(t, rawPathParseRe.Match(b), "control needle: scan must match path_params.go")
			sawAllowed = true
			continue
		}
		scanned++
		assert.False(t, rawPathParseRe.Match(b), "%s parses ids from the raw URL path; use the routed path param (routedID)", f)
	}
	assert.True(t, sawAllowed, "path_params.go must be among the scanned files")
	assert.Greater(t, scanned, 10)
}
