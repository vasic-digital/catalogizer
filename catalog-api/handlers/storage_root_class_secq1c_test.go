package handlers

// CAT-SECQ1-C (W4): storage-root create / class-change handlers. Real SQLite (production migrations), real router, real WebDAV fixture server.

import (
	"context"
	"net/http"
	"testing"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const (
	secq1cPublicIP  = "93.184.216.34" // a public address (documentation-adjacent literal): no DNS needed
	secq1cStepPW    = "Correct-Horse-9"
	secq1cAdminID   = 7
	secq1cNoConfirm = "private_network_confirmation_required"
)

// secq1cHarness is the shared harness plus the class-change route and a user with a known password, authenticated as that user.
func secq1cHarness(t *testing.T) *apiHarness {
	t.Helper()
	h := newAPIHarness(t, false)
	h.noAutoConfirm = true
	salt := "00112233445566778899aabbccddeeff"
	hash, err := services.HashPasswordWithSalt(secq1cStepPW, salt)
	require.NoError(t, err)
	_, err = h.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, 'op', 'op@example.test', ?, ?, 1)`, secq1cAdminID, hash, salt)
	require.NoError(t, err)
	h.r.PUT("/storage/roots/:id/approved-class", func(c *gin.Context) { c.Set("user_id", secq1cAdminID) }, h.handle.SetStorageRootApprovedClass)
	// the create route also needs the actor (step-up and audit)
	return h
}

func rootClass(t *testing.T, db *database.DB, name string) (string, string) {
	t.Helper()
	var cls, src string
	require.NoError(t, db.QueryRow(`SELECT approved_class, approved_class_source FROM storage_roots WHERE name = ?`, name).Scan(&cls, &src))
	return cls, src
}

func rootCount(t *testing.T, db *database.DB) int {
	t.Helper()
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM storage_roots`).Scan(&n))
	return n
}

// A host that classifies public is stored with approved_class=public, source=auto, produced by AutoClassifyApprovedClass.
func TestSECQ1C_CreatePublicRootIsAutoClassifiedPublic(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	cls, src := rootClass(t, h.db, "pub")
	require.Equal(t, "public", cls)
	require.Equal(t, "auto", src)
}

// A private host needs the operator's confirmation; without it nothing is stored and the answer names the reason.
func TestSECQ1C_CreatePrivateRootNeedsConfirmation(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan", "protocol": "webdav", "url": "http://192.168.1.20:5005/", "path": "/"})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db), "an unconfirmed private root is not stored")

	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "lan", "protocol": "webdav", "url": "http://192.168.1.20:5005/", "path": "/", "confirm_private_network": true})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	cls, src := rootClass(t, h.db, "lan")
	require.Equal(t, "private", cls)
	require.Equal(t, "operator", src, "a confirmed private approval is an operator decision")
}

// The same for the host-addressed protocols (smb): the host is classified, a LAN host needs confirmation.
func TestSECQ1C_CreateSMBRootNeedsConfirmation(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	req := map[string]interface{}{"name": "nas", "protocol": "smb", "host": "192.168.1.30", "path": "media", "username": "u", "password": "p"}
	code, body := h.do("POST", "/storage/roots", req)
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	req["confirm_private_network"] = true
	code, body = h.do("POST", "/storage/roots", req)
	require.Equal(t, http.StatusCreated, code, "%v", body)
	cls, _ := rootClass(t, h.db, "nas")
	require.Equal(t, "private", cls)
}

// A local root has no network destination: no class question, no confirmation.
func TestSECQ1C_LocalRootNeedsNoConfirmation(t *testing.T) {
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "loc", "protocol": "local", "path": t.TempDir()})
	require.Equal(t, http.StatusCreated, code, "%v", body)
}

// Widening public -> private is a step-up action; narrowing is not.
func TestSECQ1C_WideningNeedsStepUp(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := int(body["id"].(float64))
	path := "/storage/roots/" + itoa(int64(id)) + "/approved-class"

	code, body = h.do("PUT", path, map[string]interface{}{"class": "private"})
	require.Equal(t, http.StatusForbidden, code, "no step-up secret: %v", body)
	require.Equal(t, "step_up_required", body["code"])
	code, body = h.do("PUT", path, map[string]interface{}{"class": "private", "step_up_password": "wrong-password"})
	require.Equal(t, http.StatusForbidden, code, "wrong secret: %v", body)
	cls, _ := rootClass(t, h.db, "pub")
	require.Equal(t, "public", cls, "a refused widening changes nothing")

	code, body = h.do("PUT", path, map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	require.Equal(t, http.StatusOK, code, "%v", body)
	cls, src := rootClass(t, h.db, "pub")
	require.Equal(t, "private", cls)
	require.Equal(t, "operator", src)

	// narrowing needs no secret
	code, body = h.do("PUT", path, map[string]interface{}{"class": "public"})
	require.Equal(t, http.StatusOK, code, "%v", body)
	cls, _ = rootClass(t, h.db, "pub")
	require.Equal(t, "public", cls)
	// bad input
	code, _ = h.do("PUT", path, map[string]interface{}{"class": "never"})
	require.Equal(t, http.StatusBadRequest, code)
	code, _ = h.do("PUT", "/storage/roots/99999/approved-class", map[string]interface{}{"class": "public"})
	require.Equal(t, http.StatusNotFound, code)
}

// The stored class is what the real client is built with: a root approved for public refuses an allow-listed loopback fixture (class
// private), the same root approved for private reaches it. Loaded through loadStorageRoot, built by clientForRoot, connected for real.
func TestSECQ1C_StoredClassReachesTheClient(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	srv := davServer(t, "alice", "pw", map[string][]string{"/": {"a.txt"}})
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "fx", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "pw", "path": "/", "confirm_private_network": true})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := int64(body["id"].(float64))

	connect := func() error {
		root, err := h.handle.loadStorageRoot(context.Background(), id)
		require.NoError(t, err)
		c, err := h.handle.clientForRoot(root)
		require.NoError(t, err)
		return c.Connect(context.Background())
	}
	require.NoError(t, connect(), "approved private: the allow-listed LAN-class fixture is reachable")
	require.NoError(t, h.db.SetApprovedClass(context.Background(), database.TableStorageRoots, id, netguard.ClassPublic, database.ApprovedSourceOperator))
	require.Error(t, connect(), "approved public: the same destination is refused")
}
