package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Review round 1 of fix-p2-route-perms (F1, F3 gate, F5, F6, F7, F9): routes that
// mutate server configuration, reach outbound networks or expose other accounts'
// / system-wide data must not be open to every self-registered user.

type permCase struct {
	method, path string
	perm         string // "" = admin only; otherwise the permission that also admits
}

var round1Cases = []permCase{
	// F1: wizard writes reach config.json and probe hosts: system.configure or admin.
	{"POST", "/api/v1/configuration/wizard/complete", "system.configure"},
	{"POST", "/api/v1/configuration/wizard/step/x/save", "system.configure"},
	{"POST", "/api/v1/configuration/wizard/step/x/validate", "system.configure"},
	// F3 (gate half): sync endpoint mutation reaches outbound networks: admin only (same gate as storage roots, scans, smb, copy).
	{"POST", "/api/v1/sync/endpoints", ""},
	{"PUT", "/api/v1/sync/endpoints/1", ""},
	{"DELETE", "/api/v1/sync/endpoints/1", ""},
	{"POST", "/api/v1/sync/endpoints/1/sync", ""},
	{"POST", "/api/v1/sync/schedules", ""},
	// F6, F7: system-wide data and maintenance: admin only.
	{"GET", "/api/v1/analytics/system", ""},
	{"GET", "/api/v1/reports/usage", ""},
	{"GET", "/api/v1/reports/performance", ""},
	{"POST", "/api/v1/sync/cleanup", ""},
}

func TestRoutePerms_RealBinary_Round1Gates(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	status := func(r permCase, tok string) int {
		code, _ := c.do(r.method, r.path, tok, map[string]interface{}{})
		return code
	}
	plain := selfRegister(t, c, "r1plain")
	holder := func(name, perm string) string {
		role := c.id("POST", "/api/v1/roles", adm, map[string]interface{}{"name": "r1-" + name, "permissions": []string{perm}}, 201)
		c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": "r1u" + name, "email": "r1u" + name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": role}, 201)
		return c.login("r1u"+name, "Contract-Value-Strong-77")
	}
	tokens := map[string]string{
		"system.configure": holder("cfg", "system.configure"),
		"storage.manage":   holder("stor", "storage.manage"),
	}
	// F9: near-miss permission strings (no segment boundary) must not admit.
	near := holder("near", "storage.managex")
	nearCfg := holder("nearcfg", "system.configurex")

	for _, r := range round1Cases {
		name := r.method + " " + r.path
		code, _ := c.do(r.method, r.path, "", map[string]interface{}{})
		assert.Equal(t, 401, code, "anonymous "+name)
		assert.Equal(t, 403, status(r, plain), "self-registered user must be refused on "+name)
		assert.NotContains(t, []int{401, 403, 503}, status(r, adm), "admin must reach "+name)
		if r.perm != "" {
			assert.NotContains(t, []int{401, 403, 503}, status(r, tokens[r.perm]), r.perm+" holder must reach "+name)
		} else {
			assert.Equal(t, 403, status(r, tokens["storage.manage"]), "storage.manage must not open admin-only "+name)
		}
		assert.Equal(t, 403, status(r, near), "storage.managex must not admit on "+name)
		assert.Equal(t, 403, status(r, nearCfg), "system.configurex must not admit on "+name)
	}
}

// F5: another account's analytics / recommendations are reachable only by that
// account or an administrator.
func TestRoutePerms_RealBinary_SelfOrAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	mk := func(name string) (int, string) {
		id := c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": 2}, 201)
		return id, c.login(name, "Contract-Value-Strong-77")
	}
	aID, aTok := mk("sa1")
	bID, _ := mk("sa2")
	for _, tpl := range []string{"/api/v1/analytics/user/%d", "/api/v1/recommendations/personalized/%d"} {
		other := fmt.Sprintf(tpl, bID)
		own := fmt.Sprintf(tpl, aID)
		code, _ := c.do("GET", other, "", nil)
		assert.Equal(t, 401, code, "anonymous "+other)
		code, _ = c.do("GET", other, aTok, nil)
		assert.Equal(t, 403, code, "user A must not read user B via "+other)
		code, _ = c.do("GET", own, aTok, nil)
		assert.NotContains(t, []int{401, 403, 503}, code, "user A must reach own "+own)
		code, _ = c.do("GET", other, adm, nil)
		assert.NotContains(t, []int{401, 403, 503}, code, "admin must reach "+other)
	}
	require.NotEqual(t, aID, bID)
}
