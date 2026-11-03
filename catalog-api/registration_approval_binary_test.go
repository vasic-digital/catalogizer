package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-binary proof of the operator decision "registration is OPEN WITH ADMIN
// APPROVAL": POST /api/v1/auth/register creates a PENDING (inactive) role-2
// user and issues no session; REGISTRATION_MODE=open is the explicit opt-in to
// the old instant-active behaviour; REGISTRATION_MODE=disabled refuses.
// adminPut performs a mutating admin call. The admin group is CSRF-guarded, so
// it first GETs /admin/users to obtain the signed cookie + header token, then
// sends both with the PUT (what the browser client does).
func adminPut(t *testing.T, c *apiClient, path, tok string, body interface{}) (int, string) {
	req, _ := http.NewRequest("GET", c.base+"/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	r.Body.Close()
	csrfHeader := r.Header.Get("X-CSRF-Token")
	var csrfCookie *http.Cookie
	for _, ck := range r.Cookies() {
		if ck.Name == "__Host-csrf" {
			csrfCookie = ck
		}
	}
	require.NotEmpty(t, csrfHeader, "csrf token not minted")
	require.NotNil(t, csrfCookie, "csrf cookie not minted")
	raw, _ := json.Marshal(body)
	req, _ = http.NewRequest("PUT", c.base+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-CSRF-Token", csrfHeader)
	req.AddCookie(csrfCookie)
	r, err = http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer r.Body.Close()
	out, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(out)
}

func TestRegistrationApproval_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	const adminPw = "Op3rator-Supplied-Value-9x"
	const userPw = "Reg1stered-Value-Strong-77"

	// run starts one server instance with its own data dir and port.
	run := func(t *testing.T, name string, extra ...string) *apiClient {
		d := t.TempDir()
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		port := ln.Addr().(*net.TCPAddr).Port
		ln.Close()
		b := &binEnv{t: t, bin: bin, dir: d, port: port}
		cmd, done, out := b.start(append([]string{"GIN_MODE=release", "ADMIN_PASSWORD=" + adminPw}, extra...)...)
		require.True(t, b.waitHealthy(done), "%s: server must come up:\n%s", name, out.String())
		t.Cleanup(func() { b.stop(cmd, done) })
		return &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	}
	reg := func(c *apiClient, name string) (int, map[string]interface{}) {
		code, raw := c.do("POST", "/api/v1/auth/register", "", map[string]interface{}{
			"username": name, "email": name + "@example.invalid", "password": userPw,
			"first_name": "Reg", "last_name": "Ister"})
		var m map[string]interface{}
		_ = json.Unmarshal(raw, &m)
		return code, m
	}
	login := func(c *apiClient, name string) (int, map[string]interface{}) {
		code, raw := c.do("POST", "/api/v1/auth/login", "", map[string]string{"username": name, "password": userPw})
		var m map[string]interface{}
		_ = json.Unmarshal(raw, &m)
		return code, m
	}

	t.Run("default_is_pending_until_admin_approves", func(t *testing.T) {
		c := run(t, "default")
		adm := c.login("admin", adminPw)

		code, body := reg(c, "pendinguser")
		require.Equal(t, 201, code, "%v", body)
		assert.Equal(t, false, body["is_active"], "registered user must be pending (inactive)")
		assert.EqualValues(t, 2, body["role_id"], "role stays minimal")
		assert.Empty(t, body["session_token"], "no session at registration")

		// pending user cannot log in: distinct 403, not a session
		code, body = login(c, "pendinguser")
		assert.Equal(t, 403, code, "%v", body)
		assert.Contains(t, fmt.Sprint(body["error"]), "pending approval")
		assert.Empty(t, body["session_token"])
		// wrong password does not reveal the pending state
		code, raw := c.do("POST", "/api/v1/auth/login", "", map[string]string{"username": "pendinguser", "password": "Wrong-Value-Strong-11"})
		assert.Equal(t, 401, code, "%s", raw)

		// admin lists pending users; plain users and anon cannot
		code, raw = c.do("GET", "/api/v1/admin/users?status=pending", adm, nil)
		require.Equal(t, 200, code, "%s", raw)
		var list []map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &list))
		require.Len(t, list, 1, "%s", raw)
		assert.Equal(t, "pendinguser", list[0]["username"])
		assert.Equal(t, "pending", list[0]["status"])
		uid := list[0]["id"].(string)
		code, _ = c.do("GET", "/api/v1/admin/users?status=pending", "", nil)
		assert.Equal(t, 401, code)

		// admin approves through the existing update path
		code, rs := adminPut(t, c, "/api/v1/admin/users/"+uid, adm, map[string]string{"status": "active"})
		require.Equal(t, 200, code, "%s", rs)
		code, body = login(c, "pendinguser")
		require.Equal(t, 200, code, "%v", body)
		tok, _ := body["session_token"].(string)
		require.NotEmpty(t, tok)
		// approved user is role 2 and not an admin
		code, _ = c.do("GET", "/api/v1/admin/health", tok, nil)
		assert.Equal(t, 403, code, "approved user must not be admin")
		code, raw = c.do("GET", "/api/v1/admin/users?status=pending", adm, nil)
		require.Equal(t, 200, code)
		assert.JSONEq(t, "[]", string(raw), "approved user leaves the pending list")

		// a plain approved user cannot list pending users
		code, _ = c.do("GET", "/api/v1/admin/users?status=pending", tok, nil)
		assert.Equal(t, 403, code)

		// rejection: a second pending user set inactive is not pending any more
		reg(c, "rejecteduser")
		_, raw = c.do("GET", "/api/v1/admin/users?status=pending", adm, nil)
		require.NoError(t, json.Unmarshal(raw, &list))
		require.Len(t, list, 1)
		rid := list[0]["id"].(string)
		code, _ = adminPut(t, c, "/api/v1/admin/users/"+rid, adm, map[string]string{"status": "inactive"})
		require.Equal(t, 200, code)
		_, raw = c.do("GET", "/api/v1/admin/users?status=pending", adm, nil)
		assert.JSONEq(t, "[]", string(raw))
		code, _ = login(c, "rejecteduser")
		assert.Equal(t, 401, code, "rejected account is disabled, not pending")
	})

	t.Run("open_mode_is_explicit_opt_in", func(t *testing.T) {
		c := run(t, "open", "REGISTRATION_MODE=open")
		code, body := reg(c, "openuser")
		require.Equal(t, 201, code, "%v", body)
		assert.Equal(t, true, body["is_active"])
		assert.EqualValues(t, 2, body["role_id"])
		code, body = login(c, "openuser")
		assert.Equal(t, 200, code, "%v", body)
	})

	t.Run("disabled_mode_refuses", func(t *testing.T) {
		c := run(t, "disabled", "REGISTRATION_MODE=disabled")
		code, body := reg(c, "nouser")
		assert.Equal(t, 403, code, "%v", body)
		code, _ = login(c, "nouser")
		assert.Equal(t, 401, code, "no user was created")
	})
}
