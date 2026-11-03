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

// Real-binary contract for docs/api/openapi.yaml (docs C2/C5 landing):
//  1. the 13 wrap()'d path-parameter routes that the spec once marked
//     x-currently-broken reach their handler with the path parameter (the
//     gin -> mux.Vars fix in handlers/gin_adapter.go), and answer the status
//     codes the spec now documents;
//  2. the 403 / self-action / self-update behaviour of the 9 /users/{id}
//     and /roles/{id} operations documented in the spec, including that
//     PUT /users/{id} persists role_id (and revokes the target's sessions).
// Runs the real server binary on SQLite; skipped under -short.

type apiClient struct {
	t    *testing.T
	base string
}

func (c *apiClient) do(method, path, token string, body interface{}) (int, []byte) {
	var rd io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(c.t, err)
		rd = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, rd)
	require.NoError(c.t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := http.DefaultClient.Do(req)
	require.NoError(c.t, err)
	defer r.Body.Close()
	out, _ := io.ReadAll(r.Body)
	return r.StatusCode, out
}

func (c *apiClient) id(method, path, token string, body interface{}, want int) int {
	code, raw := c.do(method, path, token, body)
	require.Equal(c.t, want, code, "%s %s: %s", method, path, raw)
	var m map[string]interface{}
	require.NoError(c.t, json.Unmarshal(raw, &m), "%s", raw)
	v, ok := m["id"].(float64)
	require.True(c.t, ok, "no id in %s", raw)
	return int(v)
}

func (c *apiClient) login(user, pw string) string {
	code, raw := c.do("POST", "/api/v1/auth/login", "", map[string]string{"username": user, "password": pw})
	require.Equal(c.t, 200, code, "login %s: %s", user, raw)
	var m map[string]interface{}
	require.NoError(c.t, json.Unmarshal(raw, &m))
	tok, _ := m["session_token"].(string)
	require.NotEmpty(c.t, tok)
	return tok
}

func (c *apiClient) user(token string, id int) map[string]interface{} {
	code, raw := c.do("GET", fmt.Sprintf("/api/v1/users/%d", id), token, nil)
	require.Equal(c.t, 200, code, "%s", raw)
	var m map[string]interface{}
	require.NoError(c.t, json.Unmarshal(raw, &m))
	return m
}

func TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	adm := c.login("admin", adminPw)

	// ---- 1. formerly x-currently-broken routes --------------------------
	eid := c.id("POST", "/api/v1/errors/report", adm, map[string]string{"level": "error", "message": "contract", "component": "contract"}, 200)
	cid := c.id("POST", "/api/v1/errors/crash", adm, map[string]string{"signal": "SIGSEGV", "message": "contract"}, 200)
	lid := c.id("POST", "/api/v1/logs/collect", adm, map[string]interface{}{"name": "contract", "components": []string{"api"}}, 200)
	code, raw := c.do("POST", "/api/v1/logs/share", adm, map[string]interface{}{"collection_id": lid, "share_type": "public", "permissions": []string{"read"}})
	require.Equal(t, 200, code, "%s", raw)
	var share map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &share))
	stok, _ := share["share_token"].(string)
	require.NotEmpty(t, stok)
	sid := int(share["id"].(float64))

	routes := []struct {
		method, path string
		body         interface{}
		want         int
	}{
		{"GET", "/api/v1/configuration/wizard/step/welcome", nil, 200},
		{"GET", "/api/v1/configuration/wizard/step/no-such-step", nil, 404},
		{"POST", "/api/v1/configuration/wizard/step/welcome/validate", map[string]string{}, 200},
		{"POST", "/api/v1/configuration/wizard/step/welcome/save", map[string]string{}, 200},
		{"GET", fmt.Sprintf("/api/v1/errors/reports/%d", eid), nil, 200},
		{"GET", "/api/v1/errors/reports/999999", nil, 404},
		{"GET", "/api/v1/errors/reports/abc", nil, 400},
		{"PUT", fmt.Sprintf("/api/v1/errors/reports/%d/status", eid), map[string]string{"status": "resolved"}, 204},
		{"GET", fmt.Sprintf("/api/v1/errors/crashes/%d", cid), nil, 200},
		{"GET", "/api/v1/errors/crashes/999999", nil, 404},
		{"PUT", fmt.Sprintf("/api/v1/errors/crashes/%d/status", cid), map[string]string{"status": "resolved"}, 204},
		{"GET", fmt.Sprintf("/api/v1/logs/collections/%d", lid), nil, 200},
		{"GET", "/api/v1/logs/collections/999999", nil, 404},
		{"GET", fmt.Sprintf("/api/v1/logs/collections/%d/entries", lid), nil, 200},
		{"POST", fmt.Sprintf("/api/v1/logs/collections/%d/export", lid), map[string]string{"format": "json"}, 200},
		{"GET", fmt.Sprintf("/api/v1/logs/collections/%d/analyze", lid), nil, 200},
		{"GET", "/api/v1/logs/share/" + stok, nil, 200},
		{"GET", "/api/v1/logs/share/no-such-token", nil, 404},
		{"DELETE", fmt.Sprintf("/api/v1/logs/share/%d", sid), nil, 204},
		// Documented as-is: the revoke handler maps "not found" to 500.
		{"DELETE", "/api/v1/logs/share/999999", nil, 500},
	}
	for _, rt := range routes {
		code, raw := c.do(rt.method, rt.path, adm, rt.body)
		assert.Equal(t, rt.want, code, "%s %s: %s", rt.method, rt.path, raw)
	}

	// ---- 2. /users/{id} and /roles/{id} permissions ---------------------
	// Users created through POST /api/v1/users log in with the password given
	// at creation (one shared hasher). The extra reset below keeps coverage of
	// POST /users/{id}/reset-password.
	mrole := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "contract-manager", "permissions": []string{"user.*"}}, 201)
	mk := func(name string, role int) (int, string) {
		id := c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": role}, 201)
		code, raw := c.do("POST", fmt.Sprintf("/api/v1/users/%d/reset-password", id), adm, map[string]string{"new_password": "Reset-Value-Strong-55"})
		require.Equal(t, 200, code, "%s", raw)
		return id, c.login(name, "Reset-Value-Strong-55")
	}
	plainID, plain := mk("contractplain", 2)
	mgrID, mgr := mk("contractmgr", mrole)
	victimID, _ := mk("contractvictim", 2)

	type tc struct {
		method, path string
		body         interface{}
	}
	userOps := func(id int) []tc {
		return []tc{
			{"GET", fmt.Sprintf("/api/v1/users/%d", id), nil},
			{"PUT", fmt.Sprintf("/api/v1/users/%d", id), map[string]string{"display_name": "x"}},
			{"POST", fmt.Sprintf("/api/v1/users/%d/reset-password", id), map[string]string{"new_password": "Another-Strong-Pass-91"}},
			{"POST", fmt.Sprintf("/api/v1/users/%d/lock", id), map[string]string{"lock_until": "2099-01-01T00:00:00Z"}},
			{"POST", fmt.Sprintf("/api/v1/users/%d/unlock", id), map[string]string{}},
			{"DELETE", fmt.Sprintf("/api/v1/users/%d", id), nil},
		}
	}
	roleOps := []tc{
		{"GET", "/api/v1/roles/2", nil},
		{"PUT", "/api/v1/roles/2", map[string]string{"name": "User"}},
		{"DELETE", fmt.Sprintf("/api/v1/roles/%d", mrole), nil},
	}
	// A user without user.* permissions gets 403 on another user and on every role op.
	for _, op := range append(userOps(victimID), roleOps...) {
		code, raw := c.do(op.method, op.path, plain, op.body)
		assert.Equal(t, 403, code, "plain %s %s: %s", op.method, op.path, raw)
	}
	// user.* without system.admin: may read an admin, may not act on one; roles stay 403.
	for i, op := range append(userOps(1), roleOps...) {
		want := 403
		if i == 0 {
			want = 200
		}
		code, raw := c.do(op.method, op.path, mgr, op.body)
		assert.Equal(t, want, code, "manager %s %s: %s", op.method, op.path, raw)
	}
	// Self lock / self delete: 403 without the permission, 400 with it (and for admin).
	code, _ = c.do("POST", fmt.Sprintf("/api/v1/users/%d/lock", plainID), plain, map[string]string{"lock_until": "2099-01-01T00:00:00Z"})
	assert.Equal(t, 403, code)
	code, _ = c.do("DELETE", fmt.Sprintf("/api/v1/users/%d", plainID), plain, nil)
	assert.Equal(t, 403, code)
	for _, who := range []struct {
		tok string
		id  int
	}{{mgr, mgrID}, {adm, 1}} {
		code, raw := c.do("POST", fmt.Sprintf("/api/v1/users/%d/lock", who.id), who.tok, map[string]string{"lock_until": "2099-01-01T00:00:00Z"})
		assert.Equal(t, 400, code, "%s", raw)
		assert.Contains(t, string(raw), "Cannot lock your own account")
		code, raw = c.do("DELETE", fmt.Sprintf("/api/v1/users/%d", who.id), who.tok, nil)
		assert.Equal(t, 400, code, "%s", raw)
		assert.Contains(t, string(raw), "Cannot delete your own account")
	}
	// Self update: profile fields apply, role_id and is_active are ignored (200).
	for _, who := range []struct {
		tok  string
		id   int
		role float64
	}{{plain, plainID, 2}, {adm, 1, 1}} {
		code, raw := c.do("PUT", fmt.Sprintf("/api/v1/users/%d", who.id), who.tok, map[string]interface{}{"display_name": "self-edit", "role_id": mrole, "is_active": false})
		require.Equal(t, 200, code, "%s", raw)
		u := c.user(adm, who.id)
		assert.Equal(t, "self-edit", u["display_name"])
		assert.Equal(t, who.role, u["role_id"], "self update must not change role_id")
		assert.Equal(t, true, u["is_active"], "self update must not change is_active")
	}
	// A non-admin manager may not assign the admin role to another user.
	code, _ = c.do("PUT", fmt.Sprintf("/api/v1/users/%d", victimID), mgr, map[string]int{"role_id": 1})
	assert.Equal(t, 403, code)

	// Self GET needs no permission; self reset-password and self unlock need
	// user.manage (403 without it, 200 with it; there is no self-action refusal).
	code, raw = c.do("GET", fmt.Sprintf("/api/v1/users/%d", plainID), plain, nil)
	assert.Equal(t, 200, code, "self GET: %s", raw)
	code, _ = c.do("POST", fmt.Sprintf("/api/v1/users/%d/reset-password", plainID), plain, map[string]string{"new_password": "Another-Strong-Pass-91"})
	assert.Equal(t, 403, code)
	code, _ = c.do("POST", fmt.Sprintf("/api/v1/users/%d/unlock", plainID), plain, map[string]string{})
	assert.Equal(t, 403, code)
	code, raw = c.do("POST", fmt.Sprintf("/api/v1/users/%d/unlock", mgrID), mgr, map[string]string{})
	assert.Equal(t, 200, code, "manager self unlock: %s", raw)
	code, raw = c.do("POST", fmt.Sprintf("/api/v1/users/%d/reset-password", mgrID), mgr, map[string]string{"new_password": "Another-Strong-Pass-91"})
	assert.Equal(t, 200, code, "manager self reset-password: %s", raw)

	// An admin PUT with role_id on another user persists the role (and
	// is_active), echoed and read back identically.
	code, raw = c.do("PUT", fmt.Sprintf("/api/v1/users/%d", victimID), adm, map[string]interface{}{"role_id": mrole, "is_active": false})
	require.Equal(t, 200, code, "%s", raw)
	var echoed map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &echoed))
	assert.Equal(t, float64(mrole), echoed["role_id"], "the PUT response echoes the requested role_id")
	v := c.user(adm, victimID)
	assert.Equal(t, float64(mrole), v["role_id"], "role_id is persisted")
	assert.Equal(t, false, v["is_active"], "is_active is persisted")
	code, _ = c.do("GET", "/api/v1/users/1", "", nil)
	assert.Equal(t, 401, code)
}
