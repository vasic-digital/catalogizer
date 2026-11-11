//go:build live_smb

package filesystem

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Live test against a REAL SMB server (§11.4.289: no fixture server). Run through docs/qa/smb-subdir-20261110/run.sh, which reads the
// credentials from a 0600 env file by variable name and exports SMB_LIVE_* for this process only. Never run in CI.
func liveCfg(t *testing.T, share string) *SmbConfig {
	t.Helper()
	host := os.Getenv("SMB_LIVE_HOST")
	if host == "" {
		t.Skip("SMB_LIVE_HOST not set: live SMB target absent")
	}
	port := 445
	if p := os.Getenv("SMB_LIVE_PORT"); p != "" {
		port, _ = strconv.Atoi(p)
	}
	return &SmbConfig{Host: host, Port: port, Share: share, Username: os.Getenv("SMB_LIVE_USER"), Password: os.Getenv("SMB_LIVE_PASS"), Domain: "WORKGROUP"}
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func names(t *testing.T, c *SmbClient, p string) map[string]bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	l, err := c.ListDirectory(ctx, p)
	require.NoError(t, err, "list %q", p)
	m := map[string]bool{}
	for _, f := range l {
		m[f.Name] = true
	}
	return m
}

func TestLiveSMBSubdir(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	whole := NewSmbClient(liveCfg(t, envOr("SMB_LIVE_WHOLE", "Public")))
	require.NoError(t, whole.Connect(ctx))
	defer whole.Disconnect(ctx)
	top := names(t, whole, "")
	require.True(t, top[envOr("SMB_LIVE_SUBNAME", "Testing")], "plain share root must still list Testing: %v", top)

	sub := NewSmbClient(liveCfg(t, envOr("SMB_LIVE_SUB", "Public/Testing")))
	require.NoError(t, sub.Connect(ctx))
	defer sub.Disconnect(ctx)
	require.NoError(t, sub.TestConnection(ctx))
	in := names(t, sub, "")
	require.True(t, in[envOr("SMB_LIVE_ENTRY", "docs")], "sub root must list the corpus: %v", in)
	require.False(t, in[envOr("SMB_LIVE_SUBNAME", "Testing")], "sub root must not show the share root: %v", in)

	// traversal attempts stay inside the subtree: ".." clamps to the root, so the listing equals the root listing
	for _, p := range []string{"..", "../..", `..\..`, envOr("SMB_LIVE_ENTRY", "docs") + "/../../.."} {
		require.Equal(t, in, names(t, sub, p), "traversal %q must be clamped to the sub root", p)
	}
	// a file that exists only OUTSIDE the subtree is unreachable by any spelling
	probed := 0
	for n := range top {
		if n == envOr("SMB_LIVE_SUBNAME", "Testing") || in[n] { // a name that also exists inside the subtree would legitimately resolve there
			continue
		}
		probed++
		ok, err := sub.FileExists(ctx, "../"+n)
		require.NoError(t, err)
		require.False(t, ok, "%s outside the sub root must not be reachable", n)
	}
	t.Logf("outside-name probes: %d", probed)
	// download inside the subtree: byte-identical to the corpus manifest
	rc, err := sub.ReadFile(ctx, envOr("SMB_LIVE_FILE", "docs/book.txt"))
	require.NoError(t, err)
	defer rc.Close()
	h := sha256.New()
	_, err = io.Copy(h, rc)
	require.NoError(t, err)
	require.Equal(t, envOr("SMB_LIVE_SHA", "12dcda329a783e6877286d72b6bde722ceebf54ecf0415750008f095157a3d6d"), hex.EncodeToString(h.Sum(nil)))
}
