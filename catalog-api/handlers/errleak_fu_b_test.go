package handlers

// ERRLEAK-FU-B: behavioural redaction of the branch of GET /storage-roots/:id/status where Connect succeeds and TestConnection fails.
// It reuses the SSE-OPQ-1 harness (real handler, real JWT middleware, production-migrated SQLite); only the protocol client is a unit-class
// double that fails TestConnection with an error carrying the root's own password and a share name.

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/filesystem"
)

const (
	fubPassword = "FuB-RootPassword-8812"
	fubShare    = "fub-secret-share-5530"
)

func TestERRLEAKFUB_TestConnectionFailure_RedactionByCaller(t *testing.T) {
	o := newOPQHarness(t, nil)
	dir, err := os.MkdirTemp("/tmp", "fub-probe-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	_, err = o.h.db.Exec(`INSERT INTO storage_roots (id, name, protocol, path, password, enabled) VALUES (4, 'fubroot', 'local', ?, ?, 1)`, dir, fubPassword)
	require.NoError(t, err)
	o.sh.clientFactory = opqProbeFactory{&opqProbeClient{testErr: errors.New("share " + fubShare + " refused the probe for password " + fubPassword)}}
	var _ filesystem.FileSystemClient = &opqProbeClient{}

	user, adm := cat001Token(t, 2, "2"), cat001Token(t, 1, "1")

	// control: the admin reaches the branch (needle visible) and gets the credentials scrubbed out.
	code, body := o.get(adm, "/storage-roots/4/status")
	require.Equal(t, http.StatusOK, code, body)
	require.Contains(t, body, fubShare, "control needle: admin reaches the TestConnection branch and sees the scrubbed text: %s", body)
	assert.NotContains(t, body, fubPassword, "admin text must be credential-scrubbed")
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	assert.Equal(t, "offline", out["status"])
	assert.Equal(t, false, out["connected"])

	// non-admin: fixed class text only; neither the share nor the password.
	code, body = o.get(user, "/storage-roots/4/status")
	require.Equal(t, http.StatusOK, code, body)
	assert.NotContains(t, body, fubShare)
	assert.NotContains(t, body, fubPassword)
	out = map[string]interface{}{}
	require.NoError(t, json.Unmarshal([]byte(body), &out))
	assert.Equal(t, scanErrorClassText, out["error"])
	assert.Equal(t, "offline", out["status"])
	assert.Equal(t, false, out["connected"])
}
