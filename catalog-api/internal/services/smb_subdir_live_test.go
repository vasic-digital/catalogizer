//go:build live_smb

package services

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strconv"
	"testing"

	"catalogizer/internal/config"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Live, ROUTED-path test (§11.4.289: real SMB server, no fixture): SMBService is what the routed download/copy endpoints use. The share
// "Public/Testing" must mount "Public" and serve the corpus from the sub-directory. Run through docs/qa/smb-subdir-20261110/run.sh.
func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func TestLiveSMBServiceSubdir(t *testing.T) {
	host := os.Getenv("SMB_LIVE_HOST")
	if host == "" {
		t.Skip("SMB_LIVE_HOST not set: live SMB target absent")
	}
	port := 445
	if p := os.Getenv("SMB_LIVE_PORT"); p != "" {
		port, _ = strconv.Atoi(p)
	}
	cfg := &config.Config{}
	cfg.SMB.Hosts = []config.SMBHost{{Name: "anton", Host: host, Port: port, Share: envOr("SMB_LIVE_SUB", "Public/Testing"), Username: os.Getenv("SMB_LIVE_USER"), Password: os.Getenv("SMB_LIVE_PASS"), Domain: "WORKGROUP"}}
	cfg.SMB.ChunkSize = 32 * 1024
	s := NewSMBService(cfg, zap.NewNop())

	rc, size, err := s.StreamFile("anton", envOr("SMB_LIVE_FILE", "docs/book.txt"))
	require.NoError(t, err)
	defer rc.Close()
	h := sha256.New()
	n, err := io.Copy(h, rc)
	require.NoError(t, err)
	require.Equal(t, size, n)
	require.Equal(t, envOr("SMB_LIVE_SHA", "12dcda329a783e6877286d72b6bde722ceebf54ecf0415750008f095157a3d6d"), hex.EncodeToString(h.Sum(nil)))

	files, err := s.ListFiles("anton", "../..")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, f := range files {
		names[f.Name()] = true
	}
	require.True(t, names[envOr("SMB_LIVE_ENTRY", "docs")], "traversal is clamped to the sub root: %v", names)
	require.False(t, names[envOr("SMB_LIVE_SUBNAME", "Testing")], "the share root must not be visible: %v", names)
}
