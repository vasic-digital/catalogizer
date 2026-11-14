package main

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PXF18-FU6: GET /api/v1/challenges/results returns stored challenge execution
// results (assertion text, captured output). Running challenges is admin-only,
// so reading what they produced is operator/diagnostic data and must be
// admin-only too. Real binary, real router, real SQLCipher DB, real register + login.
func TestRoutePerms_RealBinary_ChallengeResultsAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "fu6plain")

	// Seed a real result: admin runs one challenge so stored result text exists.
	code, raw := c.do("GET", "/api/v1/challenges", adm, nil)
	require.Equal(t, 200, code, "%s", raw)
	var list struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &list))
	require.NotEmpty(t, list.Data, "no challenges registered; cannot seed a result")
	code, raw = c.do("POST", "/api/v1/challenges/"+list.Data[0].ID+"/run", adm, map[string]interface{}{})
	require.NotContains(t, []int{401, 403, 503}, code, "admin must reach run: %s", raw)

	// Positive control: admin sees a non-empty result set.
	code, raw = c.do("GET", "/api/v1/challenges/results", adm, nil)
	require.Equal(t, 200, code, "%s", raw)
	var res struct {
		Count int `json:"count"`
	}
	require.NoError(t, json.Unmarshal(raw, &res))
	require.Greater(t, res.Count, 0, "control: admin must see seeded results: %s", raw)

	// Anonymous and garbage token are 401 (control that auth chain is live).
	code, _ = c.do("GET", "/api/v1/challenges/results", "", nil)
	assert.Equal(t, 401, code, "anonymous")
	code, _ = c.do("GET", "/api/v1/challenges/results", "not-a-token", nil)
	assert.Equal(t, 401, code, "garbage token")

	// The defect: a self-registered non-admin must be refused.
	code, raw = c.do("GET", "/api/v1/challenges/results", plain, nil)
	assert.Equal(t, 403, code, "non-admin must not read challenge results; got body: %.300s", raw)
	code, raw = c.do("GET", "/api/v1/challenges/results?limit=0", plain, nil)
	assert.Equal(t, 403, code, "non-admin (limit=0) must not read challenge results; got body: %.300s", raw)
	// Same class: challenge definitions are diagnostic metadata; admin can, plain cannot.
	for _, p := range []string{"/api/v1/challenges", "/api/v1/challenges/" + list.Data[0].ID} {
		code, raw = c.do("GET", p, plain, nil)
		assert.Equal(t, 403, code, "non-admin GET "+p+": %.200s", raw)
		code, _ = c.do("GET", p, "", nil)
		assert.Equal(t, 401, code, "anonymous GET "+p)
		code, _ = c.do("GET", p, adm, nil)
		assert.Equal(t, 200, code, "admin GET "+p)
	}
}
