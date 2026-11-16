package handlers

// CAT-CPN-1 (operator decision 2026-11-16): a storage root whose destination classifies as PRIVATE LAN needs confirm_private_network=true for
// EVERY networked protocol; without it the answer is 400 with a fixed class text and nothing is stored. Real router, real handler, real SQLite.

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"catalogizer/filesystem"
	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

type cpn1Case struct {
	name string
	req  map[string]interface{}
}

func cpn1Cases() []cpn1Case {
	return []cpn1Case{
		{"smb", map[string]interface{}{"protocol": "smb", "host": "192.168.77.30", "path": "media", "username": "u", "password": "p"}},
		{"ftp", map[string]interface{}{"protocol": "ftp", "host": "192.168.77.31", "path": "/media", "username": "u", "password": "p"}},
		{"nfs", map[string]interface{}{"protocol": "nfs", "host": "192.168.77.32", "path": "/export/media", "mount_point": "/mnt/cpn1nfs"}},
		{"nfs3", map[string]interface{}{"protocol": "nfs3", "host": "192.168.77.33", "path": "/export/media"}},
		{"webdav", map[string]interface{}{"protocol": "webdav", "url": "http://192.168.77.34:5005/dav", "path": "/"}},
		{"s3", map[string]interface{}{"protocol": "s3", "url": "http://192.168.77.35:9000", "path": "bucket", "username": "k", "password": "s"}},
	}
}

func TestCPN1_PrivateRootWithoutConfirmationIs400ForEveryProtocol(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	require.NoError(t, filesystem.RegisterNFS3Protocol())
	defer filesystem.UnregisterProtocol("nfs3")
	require.NoError(t, filesystem.RegisterS3Protocol())
	defer filesystem.UnregisterProtocol("s3")
	for _, tc := range cpn1Cases() {
		t.Run(tc.name, func(t *testing.T) {
			h := secq1cHarness(t) // noAutoConfirm: the request goes to the handler exactly as sent
			req := map[string]interface{}{"name": "lan-" + tc.name}
			for k, v := range tc.req {
				req[k] = v
			}
			code, body := h.do("POST", "/storage/roots", req)
			require.Equal(t, http.StatusBadRequest, code, "%v", body)
			require.Equal(t, secq1cNoConfirm, body["code"])
			for _, v := range body {
				s, _ := v.(string)
				require.False(t, strings.Contains(s, "192.168.77."), "the refusal must not echo the destination: %q", s)
			}
			require.Equal(t, 0, rootCount(t, h.db), "an unconfirmed private root is not stored")

			req["confirm_private_network"] = true
			code, body = h.do("POST", "/storage/roots", req)
			require.Equal(t, http.StatusCreated, code, "%v", body)
			cls, src := rootClass(t, h.db, "lan-"+tc.name)
			require.Equal(t, "private", cls)
			require.Equal(t, "operator", src)
		})
	}
}

// confirm_private_network=false is the same as absent; a public destination and loopback are unchanged by the decision.
func cpn1RegisterNFS3(t *testing.T) {
	t.Helper()
	require.NoError(t, filesystem.RegisterNFS3Protocol())
	t.Cleanup(func() { filesystem.UnregisterProtocol("nfs3") })
}

func TestCPN1_FalseIsAbsentPublicUnchangedLoopbackRefused(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	cpn1RegisterNFS3(t)
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "f", "protocol": "smb", "host": "192.168.77.40", "path": "m", "username": "u", "password": "p", "confirm_private_network": false})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])

	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "smb", "host": secq1cPublicIP, "path": "m", "username": "u", "password": "p"})
	require.Equal(t, http.StatusCreated, code, "public needs no confirmation: %v", body)

	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "lo", "protocol": "nfs3", "host": "127.0.0.1", "path": "/x", "confirm_private_network": true})
	require.Equal(t, http.StatusBadRequest, code, "loopback stays refused outright, confirmation or not: %v", body)
	require.NotEqual(t, secq1cNoConfirm, body["code"], "the loopback refusal is not a confirmation request")
	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "md", "protocol": "smb", "host": "169.254.169.254", "path": "x", "confirm_private_network": true})
	require.Equal(t, http.StatusBadRequest, code, "metadata stays refused outright: %v", body)
}

// A destination that was public at the early check and re-resolves private at approval time is refused with the same 400 (nothing stored).
func TestCPN1_RebindPublicToPrivateAtApprovalIs400(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	calls := 0
	old := classifyDestination
	classifyDestination = func(ctx context.Context, host, rawURL string) (netguard.Class, error) {
		calls++
		if calls == 1 {
			return netguard.ClassPublic, nil
		}
		return netguard.ClassPrivate, nil
	}
	t.Cleanup(func() { classifyDestination = old })
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "rb", "protocol": "smb", "host": "example.invalid", "path": "m", "username": "u", "password": "p", "confirm_private_network": true}) // confirmed: only the rebinding guard can refuse
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db))
}

// A name that cannot be resolved now is treated like a private one: confirmation required, 400, nothing stored.
func TestCPN1_UnresolvableNameNeedsConfirmation(t *testing.T) {
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "unres", "protocol": "smb", "host": "nas-not-resolvable.example.invalid", "path": "m", "username": "u", "password": "p"})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db))
}
