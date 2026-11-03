package main

import (
	"fmt"
	"net"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-binary proof of the production admin gate (CAT-051 F4, review R2 B1'/D1/P1):
// the real router assembly of main.go, real JWT middleware, real migrated
// SQLCipher DB, real login tokens. Kills mutations of the main.go wiring that
// the source-grep test cannot see (SetAdminChecker(nil) after the helper;
// adminGroup.Use(RequireAuth) instead of RequireAdmin) and covers the two
// /admin routes that used to sit outside the group.
func TestAdminGate_RealBinary(t *testing.T) {
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

	mk := func(name string, role int) string {
		c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": role}, 201)
		return c.login(name, "Contract-Value-Strong-77")
	}
	// /admin/health, /config, /logs and /errors have NO handler-level admin
	// check: only the group's RequireAdmin protects them, so they are the
	// canaries for the main.go wiring. system-info also checks in the handler.
	const gated = "/api/v1/admin/health"
	status := func(method, path, tok string) int {
		code, _ := c.do(method, path, tok, map[string]interface{}{})
		return code
	}

	// (b) a plain role-2 user is refused (kills adminGroup.Use(RequireAuth)).
	plain := mk("gateplain", 2)
	for _, p := range []string{gated, "/api/v1/admin/config", "/api/v1/admin/logs", "/api/v1/admin/errors"} {
		assert.Equal(t, 403, status("GET", p, plain), p)
		assert.Equal(t, 200, status("GET", p, adm), p)
	}

	// The handlers of /admin/config etc. also check their own permission, so a
	// holder of ONLY system.configure passes the handler but must still be
	// stopped by the group's RequireAdmin (this alone distinguishes the group gate).
	cfgRole := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "gate-config-only", "permissions": []string{"system.configure"}}, 201)
	cfgOnly := mk("gatecfg", cfgRole)
	assert.Equal(t, 403, status("GET", "/api/v1/admin/config", cfgOnly), "system.configure alone must not pass the admin group gate")

	// (a) a role that is NOT id 1 but holds system.admin passes (kills SetAdminChecker(nil)).
	superRole := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "gate-super", "permissions": []string{"system.admin"}}, 201)
	require.NotEqual(t, 1, superRole)
	super := mk("gatesuper", superRole)
	assert.Equal(t, 200, status("GET", gated, super), "role %d holding system.admin must pass the production admin gate", superRole)

	// (c) the role loses system.admin without any session revocation: the same
	// token (whose claim names the role) must stop working at once.
	code, raw := c.do("PUT", fmt.Sprintf("/api/v1/roles/%d", superRole), adm, map[string]interface{}{"name": "gate-super", "permissions": []string{"media.view"}})
	require.Equal(t, 200, code, "%s", raw)
	assert.Equal(t, 403, status("GET", gated, super), "stale claim after the role lost system.admin")

	// D1/P1: the two admin routes that were registered outside the admin group.
	for _, p := range []string{"/api/v1/admin/image-quality/revalidate", "/api/v1/admin/covers/generate-thumbnails"} {
		assert.Equal(t, 403, status("POST", p, plain), "non-admin must be refused on "+p)
		assert.Equal(t, 403, status("POST", p, super), "ex-admin role must be refused on "+p)
		code := status("POST", p, adm)
		assert.NotContains(t, []int{401, 403, 503}, code, "admin must reach "+p)
	}
}
