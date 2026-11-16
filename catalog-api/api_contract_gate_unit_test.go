package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// goldenAdminGatedSHA256 pins testdata/admin_gated_ops.golden, the frozen census of the 73 operations the pre-ADMD1F1-S1 description
// classifier gated (derived mechanically over the real spec before the markers were added) PLUS the 12 RequireAdmin routes the review
// found unmarked (4 sync mutators with a stale owner-wording 403, 8 routes without any 403) = 85. Changing the set of admin
// routes means changing the spec marker AND this golden file AND this hash in the same reviewed change.
const goldenAdminGatedSHA256 = "d30c5e8baad790e6934898100f3e8a3a6bc113d9a9abd0fb6c801e1192127726"

func loadRealSpec(t *testing.T) map[string]interface{} {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	return doc
}

func goldenSet(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "admin_gated_ops.golden"))
	require.NoError(t, err)
	sum := sha256.Sum256(raw)
	require.Equal(t, goldenAdminGatedSHA256, hex.EncodeToString(sum[:]), "golden census file was edited without updating its pinned hash")
	return strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
}

func opOf(t *testing.T, doc map[string]interface{}, method, path string) map[string]interface{} {
	t.Helper()
	item, ok := doc["paths"].(map[string]interface{})[path].(map[string]interface{})
	require.True(t, ok, "path %s", path)
	op, ok := item[method].(map[string]interface{})
	require.True(t, ok, "%s %s", method, path)
	return op
}

func r403(t *testing.T, op map[string]interface{}) map[string]interface{} {
	t.Helper()
	m, ok := op["responses"].(map[string]interface{})["403"].(map[string]interface{})
	require.True(t, ok)
	return m
}

// ADMD1F1-S1: the marked set of the real spec equals the frozen census exactly (diff = 0, both directions) and the spec has no drift.
func TestADMD1F1S1_RealSpecMarkerCensusEqualsGolden(t *testing.T) {
	doc := loadRealSpec(t)
	want := goldenSet(t)
	require.Len(t, want, 85)
	require.Equal(t, want, adminGatedSet(doc), "marked operations must equal the frozen census")
	require.Empty(t, adminGateSpecViolations(doc))
	for _, l := range []string{"get /api/v1/sync/endpoints/{id}", "get /api/v1/sync/sessions/{id}"} {
		require.Contains(t, want, l)
	}
}

func TestADMD1F1S1_MarkerSemantics(t *testing.T) {
	require.True(t, adminGatedOp(map[string]interface{}{adminGateMarker: true}))
	require.False(t, adminGatedOp(map[string]interface{}{adminGateMarker: false}))
	require.False(t, adminGatedOp(map[string]interface{}{adminGateMarker: "true"}), "string is not the boolean")
	require.False(t, adminGatedOp(map[string]interface{}{}))
	// a description is never consulted for the decision
	require.False(t, adminGatedOp(map[string]interface{}{"responses": map[string]interface{}{"403": map[string]interface{}{"description": "Admin role required"}}}))
}

// Mutants M1..M7 (each must turn a gate red, proving the checks are live; each mutates a fresh parse of the real spec).
func TestADMD1F1S1_MutantsAreKilled(t *testing.T) {
	const gatedM, gatedP = "get", "/api/v1/sync/endpoints/{id}"
	want := goldenSet(t)

	t.Run("M1 marker removed from one operation", func(t *testing.T) {
		doc := loadRealSpec(t)
		delete(opOf(t, doc, gatedM, gatedP), adminGateMarker)
		require.NotEqual(t, want, adminGatedSet(doc), "census gate")
		require.NotEmpty(t, adminGateSpecViolations(doc), "drift gate: its 403 still quotes the admin body")
	})
	t.Run("M2 marker set false", func(t *testing.T) {
		doc := loadRealSpec(t)
		opOf(t, doc, gatedM, gatedP)[adminGateMarker] = false
		require.NotEqual(t, want, adminGatedSet(doc))
		require.NotEmpty(t, adminGateSpecViolations(doc))
	})
	t.Run("M3 admin body text added to an unmarked operation", func(t *testing.T) {
		doc := loadRealSpec(t)
		op := opOf(t, doc, "get", "/api/v1/auth/status")
		require.False(t, adminGatedOp(op))
		op["responses"].(map[string]interface{})["403"] = map[string]interface{}{"description": `Forbidden; body {"success":false,"error":"Admin role required"}`}
		v := adminGateSpecViolations(doc)
		require.Len(t, v, 1)
		require.Contains(t, v[0], "no x-catalogizer-admin-gated")
	})
	t.Run("M4 marker on an operation without a 403", func(t *testing.T) {
		doc := loadRealSpec(t)
		op := opOf(t, doc, "get", "/api/v1/auth/status")
		delete(op["responses"].(map[string]interface{}), "403")
		op[adminGateMarker] = true
		v := adminGateSpecViolations(doc)
		require.Len(t, v, 1)
		require.Contains(t, v[0], "documents no 403")
	})
	t.Run("M5 census off by one (extra marker, with a 403)", func(t *testing.T) {
		doc := loadRealSpec(t)
		op := opOf(t, doc, "get", "/api/v1/auth/status")
		r := op["responses"].(map[string]interface{})
		if _, has := r["403"]; !has {
			r["403"] = map[string]interface{}{"description": "Forbidden"}
		}
		op[adminGateMarker] = true
		require.Len(t, adminGatedSet(doc), 86)
		require.NotEqual(t, want, adminGatedSet(doc))
	})
	t.Run("M6 non-boolean marker", func(t *testing.T) {
		doc := loadRealSpec(t)
		opOf(t, doc, gatedM, gatedP)[adminGateMarker] = "true"
		v := adminGateSpecViolations(doc)
		require.NotEmpty(t, v)
		require.Contains(t, strings.Join(v, "\n"), "must be a boolean")
	})
	t.Run("M7 a description reworded to hide the admin text still gates when marked, and a gated op loses nothing", func(t *testing.T) {
		doc := loadRealSpec(t)
		r403(t, opOf(t, doc, gatedM, gatedP))["description"] = "Forbidden"
		require.Equal(t, want, adminGatedSet(doc), "decision no longer depends on the wording")
		require.Empty(t, adminGateSpecViolations(doc))
	})
}

// golden-FALSE controls: legitimate non-admin 403 wording without the marker is not drift.
func TestADMD1F1S1_NonAdminDescriptionsAreNotDrift(t *testing.T) {
	doc := loadRealSpec(t)
	op := opOf(t, doc, "get", "/api/v1/auth/status")
	op["responses"].(map[string]interface{})["403"] = map[string]interface{}{"description": "Forbidden; the caller is not the owner of the session"}
	require.Empty(t, adminGateSpecViolations(doc))
}

// Round 2 (F4/F5): drift check for every method, $ref 403s, path-level and non-boolean markers, secondary wording signal.
func TestADMD1F1S1_DriftAllMethodsAndRefs(t *testing.T) {
	for _, mp := range [][2]string{{"post", "/api/v1/auth/login"}, {"put", "/api/v1/auth/profile"}, {"delete", "/api/v1/collections/{id}"}, {"get", "/api/v1/auth/status"}} {
		doc := loadRealSpec(t)
		item, ok := doc["paths"].(map[string]interface{})[mp[1]].(map[string]interface{})
		if !ok {
			continue
		}
		op, ok := item[mp[0]].(map[string]interface{})
		if !ok {
			continue
		}
		op["responses"].(map[string]interface{})["403"] = map[string]interface{}{"description": "Forbidden; caller is not an administrator (decided from the database)"}
		v := adminGateSpecViolations(doc)
		require.Len(t, v, 1, "%s %s", mp[0], mp[1])
	}
	t.Run("ref 403 resolved", func(t *testing.T) {
		doc := loadRealSpec(t)
		doc["components"].(map[string]interface{})["responses"] = map[string]interface{}{"AdminForbidden": map[string]interface{}{"description": "Administrator role required"}}
		op := opOf(t, doc, "post", "/api/v1/auth/login")
		op["responses"].(map[string]interface{})["403"] = map[string]interface{}{"$ref": "#/components/responses/AdminForbidden"}
		require.Len(t, adminGateSpecViolations(doc), 1)
	})
	t.Run("path-level marker", func(t *testing.T) {
		doc := loadRealSpec(t)
		doc["paths"].(map[string]interface{})["/api/v1/auth/status"].(map[string]interface{})[adminGateMarker] = true
		require.Len(t, adminGateSpecViolations(doc), 1)
		require.NotEmpty(t, adminGateLoaderViolations("/x", map[string]interface{}{adminGateMarker: true}))
	})
	t.Run("loader rejects non-boolean", func(t *testing.T) {
		require.NotEmpty(t, adminGateLoaderViolations("/x", map[string]interface{}{"get": map[string]interface{}{adminGateMarker: "yes"}}))
		require.Empty(t, adminGateLoaderViolations("/x", map[string]interface{}{"get": map[string]interface{}{adminGateMarker: true}}))
	})
	t.Run("wording variants all flagged when unmarked", func(t *testing.T) {
		for _, d := range []string{"Caller is not an administrator (decided from the database)", "Administrator role required", "Caller lacks `system.admin`", "lacks report.view", "Caller lacks user.manage"} {
			require.True(t, adminRoleText.MatchString(d), d)
		}
		require.False(t, adminRoleText.MatchString("Forbidden; the caller is not the owner of the session"))
	})
}
