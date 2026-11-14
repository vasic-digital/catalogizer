package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const scanRawError = "smb://nas-internal.lan/private-share: login failed for svc_backup (NT_STATUS_LOGON_FAILURE)"

// STATS-SCANS-ERR-1: GET /api/v1/stats/scans leaked the raw scan error_message (host names,
// share names, account names) to every authenticated user. Non-admins get a fixed error class
// text; administrators still see the raw text. Real binary, real router, real SQLCipher DB.
func TestRoutePerms_RealBinary_StatsScansErrorMessage(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, dir, stop := startPermServerDir(t, t.TempDir(), t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "scanerrplain")

	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "catalog.db"),
		EncryptionKeyFile: filepath.Join(dir, "keys", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('scanerr-root','local','/x',1)`)
	require.NoError(t, err)
	var root int64
	require.NoError(t, db.QueryRowContext(context.Background(), `SELECT id FROM storage_roots WHERE name='scanerr-root'`).Scan(&root))
	_, err = db.Exec(`INSERT INTO scan_history (storage_root_id, scan_type, status, start_time, error_count, error_message)
		VALUES (?, 'full', 'failed', '2026-11-01 10:00:00', 1, ?)`, root, scanRawError)
	require.NoError(t, err)
	db.Close()

	type resp struct {
		Data struct {
			Scans []struct {
				Status       string  `json:"status"`
				ErrorMessage *string `json:"error_message"`
			} `json:"scans"`
		} `json:"data"`
	}
	get := func(tok string) (resp, string) {
		code, raw := c.do("GET", "/api/v1/stats/scans", tok, nil)
		require.Equal(t, 200, code, "%s", raw)
		var r resp
		require.NoError(t, json.Unmarshal(raw, &r))
		return r, string(raw)
	}

	// Positive control: admin sees the seeded row with the raw text.
	ra, rawA := get(adm)
	require.Len(t, ra.Data.Scans, 1, "control: seeded row must be listed: %s", rawA)
	require.NotNil(t, ra.Data.Scans[0].ErrorMessage)
	assert.Equal(t, scanRawError, *ra.Data.Scans[0].ErrorMessage, "admin keeps the raw text")

	// The defect: non-admin must not see it, but still sees the row and a fixed class text.
	rp, rawP := get(plain)
	require.Len(t, rp.Data.Scans, 1, "non-admin still sees the row: %s", rawP)
	assert.Equal(t, "failed", rp.Data.Scans[0].Status)
	require.NotNil(t, rp.Data.Scans[0].ErrorMessage, "non-admin gets a fixed class text, not nothing: %s", rawP)
	assert.Equal(t, "scan error (details restricted to administrators)", *rp.Data.Scans[0].ErrorMessage)
	for _, leak := range []string{"nas-internal", "private-share", "svc_backup", "NT_STATUS"} {
		assert.NotContains(t, rawP, leak, "raw scan error text leaked to a non-admin")
	}
	code, _ := c.do("GET", "/api/v1/stats/scans", "", nil)
	assert.Equal(t, 401, code, "anonymous")
}

// SYNC-ADMIN-1: every GET under /api/v1/sync is admin-only (sync endpoint config, sessions,
// statistics and the alias routes). /browse/roots and /storage/roots are unchanged.
func TestRoutePerms_RealBinary_SyncGetsAdminOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	c, adm, stop := startPermServer(t, t.TempDir())
	defer stop()
	plain := selfRegister(t, c, "syncplain")

	paths := []string{
		"/api/v1/sync/endpoints", "/api/v1/sync/endpoints/1",
		"/api/v1/sync/sessions", "/api/v1/sync/sessions/1",
		"/api/v1/sync/statistics", "/api/v1/sync/history",
		"/api/v1/sync/status", "/api/v1/sync/providers",
		"/api/v1/sync/devices", "/api/v1/sync/conflicts",
	}
	for _, p := range paths {
		code, _ := c.do("GET", p, "", nil)
		assert.Equal(t, 401, code, "anonymous GET "+p)
		code, raw := c.do("GET", p, adm, nil)
		assert.NotContains(t, []int{401, 403, 503}, code, "admin must reach GET %s: %.200s", p, raw)
		code, raw = c.do("GET", p, plain, nil)
		assert.Equal(t, 403, code, "non-admin must be refused on GET %s: %.200s", p, raw)
	}
	// Unchanged: read-only root listings stay open to authenticated users.
	for _, p := range []string{"/api/v1/browse/roots", "/api/v1/storage/roots"} {
		code, raw := c.do("GET", p, plain, nil)
		assert.NotContains(t, []int{401, 403}, code, "must stay open to authenticated users: GET %s: %.200s", p, raw)
	}
}
