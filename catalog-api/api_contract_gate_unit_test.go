package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func spec403(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "docs", "api", "openapi.yaml"))
	require.NoError(t, err)
	var doc map[string]interface{}
	require.NoError(t, yaml.Unmarshal(raw, &doc))
	op := doc["paths"].(map[string]interface{})[path].(map[string]interface{})["get"].(map[string]interface{})
	return op["responses"].(map[string]interface{})["403"].(map[string]interface{})["description"].(string)
}

// ADMD1RRRRRRRRR-m1: the merged 403 of the two sync GET {id} operations must classify as an admin gate (the non-admin leg then
// asserts 403), and the classifier must keep rejecting genuinely non-gate descriptions.
func TestADMD1RRRRRRRRR_SyncMerged403IsClassifiedAdminGated(t *testing.T) {
	for _, p := range []string{"/api/v1/sync/endpoints/{id}", "/api/v1/sync/sessions/{id}"} {
		require.True(t, admin403Gated(spec403(t, p)), "%s: %q must classify as an admin gate", p, spec403(t, p))
	}
	// negative controls: non-gate wording stays non-gate; a bare owner-only wording is not forced to gated
	require.False(t, admin403Gated("Forbidden; the caller is not the owner of the session"))
	require.False(t, admin403Gated("Forbidden: registration is disabled"))
	require.True(t, admin403Gated("Forbidden (admin role required)"))
	require.False(t, admin403Gated("Forbidden: pending approval"), "no admin token at all")
}
