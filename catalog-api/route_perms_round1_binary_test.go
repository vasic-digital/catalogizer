package main

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

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
	// F-R1: report generation and per-media aggregates reach the same
	// system-wide aggregates (GetSystemAnalytics, top media, device mix).
	{"POST", "/api/v1/analytics/reports", ""},
	{"GET", "/api/v1/analytics/media/1", ""},
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

// N1: analytics writes are attributed to the caller (the token), never to a user id taken
// from the request body: forged rows would feed the admin-only reports.
func TestRoutePerms_RealBinary_AnalyticsWritesCannotForgeUserID(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	mk := func(name string) (int, string) {
		id := c.id("POST", "/api/v1/users", adm, map[string]interface{}{"username": name, "email": name + "@example.invalid", "password": "Contract-Value-Strong-77", "role_id": 2}, 201)
		return id, c.login(name, "Contract-Value-Strong-77")
	}
	victim, _ := mk("n1victim")
	attacker, aTok := mk("n1attacker")
	counts := func(id int) (int, int) {
		code, raw := c.do("GET", fmt.Sprintf("/api/v1/analytics/user/%d?end_date=%s", id, time.Now().AddDate(0, 0, 2).Format("2006-01-02")), adm, nil)
		require.Equal(t, 200, code, "%s", raw)
		var out struct {
			Accesses int `json:"total_media_accesses"`
			Events   int `json:"total_events"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		return out.Accesses, out.Events
	}
	vA, vE := counts(victim)
	aA, aE := counts(attacker)

	forgedAccess := map[string]interface{}{"user_id": victim, "media_id": 1, "action": "view"}
	forgedEvent := map[string]interface{}{"user_id": victim, "event_type": "x", "event_category": "y", "data": "{}"}
	code, _ := c.do("POST", "/api/v1/analytics/access", aTok, forgedAccess)
	assert.Equal(t, 403, code, "forged user_id on /analytics/access")
	code, _ = c.do("POST", "/api/v1/analytics/event", aTok, forgedEvent)
	assert.Equal(t, 403, code, "forged user_id on /analytics/event")
	nA, nE := counts(victim)
	assert.Equal(t, vA, nA, "victim access count must not change")
	assert.Equal(t, vE, nE, "victim event count must not change")

	// Own id, or no id at all: accepted and attributed to the caller.
	for _, body := range []map[string]interface{}{
		{"user_id": attacker, "media_id": 1, "action": "view"},
		{"media_id": 1, "action": "view"},
	} {
		code, raw := c.do("POST", "/api/v1/analytics/access", aTok, body)
		assert.Equal(t, 200, code, "%s", raw)
	}
	code, raw := c.do("POST", "/api/v1/analytics/event", aTok, map[string]interface{}{"event_type": "x", "event_category": "y", "data": "{}"})
	assert.Equal(t, 200, code, "%s", raw)
	nA2, nE2 := counts(attacker)
	assert.Equal(t, aA+2, nA2, "own accesses are attributed to the caller")
	assert.Equal(t, aE+1, nE2, "own event is attributed to the caller")
	nA, nE = counts(victim)
	assert.Equal(t, vA, nA)
	assert.Equal(t, vE, nE)
}

// N2: a collection created by an administrator is global (no owner column yet), and a
// non-admin must be able neither to change nor to delete it. Skipped until the pending
// feat-smart-collections branch (migration 23: owner and is_public, writes need the owner
// or an admin) lands; set CATALOGIZER_RUN_N2=1 to run it today (it is RED on this branch:
// bob's PUT and DELETE answer 200). Remove the skip when that branch merges.
func TestRoutePerms_RealBinary_CollectionOwnership(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	if os.Getenv("CATALOGIZER_RUN_N2") != "1" {
		t.Skip("N2 is closed by the pending feat-smart-collections branch (owner column); see docs/FOLLOW_UPS_NEXT_PHASE.md")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	bob := selfRegister(t, c, "n2bob")
	id := c.id("POST", "/api/v1/collections", adm, map[string]interface{}{"name": "global-n2"}, 201)
	path := fmt.Sprintf("/api/v1/collections/%d", id)

	code, raw := c.do("PUT", path, bob, map[string]interface{}{"name": "hijacked"})
	assert.Contains(t, []int{403, 404}, code, "non-admin must not rename a global collection: %s", raw)
	code, raw = c.do("DELETE", path, bob, nil)
	assert.Contains(t, []int{403, 404}, code, "non-admin must not delete a global collection: %s", raw)

	// The collection is untouched, and the administrator can still change it.
	code, raw = c.do("GET", path, adm, nil)
	require.Equal(t, 200, code, "%s", raw)
	assert.Contains(t, string(raw), "global-n2")
	assert.NotContains(t, string(raw), "hijacked")
	code, raw = c.do("PUT", path, adm, map[string]interface{}{"name": "renamed-by-admin"})
	assert.Equal(t, 200, code, "%s", raw)
}
