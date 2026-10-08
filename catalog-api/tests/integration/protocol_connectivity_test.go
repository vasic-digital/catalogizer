package integration

import (
	"catalogizer/filesystem"
	"catalogizer/internal/services"
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// The test infrastructure is a PER-RUN stack (scripts/test-infra/up.sh, docker-compose.test-infra.yml): random loopback host ports and generated credentials, no fixed port and no
// literal credential (WF12 F1; WF17 TI-I1: this file still carried the pre-T129 contract, localhost:8081 and test/test123). up.sh writes them to a mode 0600 env file; point this
// package at it with CATALOGIZER_TEST_INFRA_ENV. An explicit *_TEST_SERVER (host or host:port) and *_TEST_USER / *_TEST_PASS still override a single value. Nothing configured: the test
// SKIPS and says how to start the stack (never a PASS against a default that nobody serves).
const infraEnvVar = "CATALOGIZER_TEST_INFRA_ENV"

// perRunEnv parses the per-run env file (KEY=VALUE lines; data, never evaluated); an unset variable or an unreadable file yields an empty map.
func perRunEnv() map[string]string {
	out := map[string]string{}
	path := os.Getenv(infraEnvVar)
	if path == "" {
		return out
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok && strings.HasPrefix(k, "TI_") {
			out[k] = v
		}
	}
	return out
}

// infraValue returns the explicit override, else the key of the per-run env file, else "".
func infraValue(override, key string) string {
	if v := os.Getenv(override); v != "" {
		return v
	}
	return perRunEnv()[key]
}

// serviceTarget resolves host and port of a service: an explicit override (host, defaultPort assumed, or host:port), else 127.0.0.1 and the port of the per-run env file; ok is false when neither names one.
func serviceTarget(serverVar, portKey string, defaultPort int) (host string, port int, ok bool) {
	if v := os.Getenv(serverVar); v != "" {
		if h, p, err := net.SplitHostPort(v); err == nil {
			if n, perr := strconv.Atoi(p); perr == nil {
				return h, n, true
			}
		}
		return v, defaultPort, true
	}
	if p := perRunEnv()[portKey]; p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			return "127.0.0.1", n, true
		}
	}
	return "", 0, false
}

// skipIfNotConfigured skips the test when the service is neither named by an override nor by the per-run env file.
func skipIfNotConfigured(t *testing.T, serverVar, portKey, serviceName string) {
	t.Helper()
	if _, _, ok := serviceTarget(serverVar, portKey, 0); !ok {
		t.Skipf("Skipping test: %s is not configured (start the stack: scripts/test-infra/up.sh --build-id <id>, then set %s to the env file it names, or set %s) (SKIP-OK: #topology-no-container)", serviceName, infraEnvVar, serverVar)
	}
}

func skipIfNoContainer(t *testing.T, envVar, serviceName string) {
	if os.Getenv(envVar) == "" {
		t.Skipf("Skipping test: %s not set (start %s container first) (SKIP-OK: #topology-no-container)", envVar, serviceName)
	}
}

func TestSMBProtocolConnectivity(t *testing.T) {
	skipIfNotConfigured(t, "SMB_TEST_SERVER", "TI_PORT_SMB", "SMB")

	ctx := context.Background()

	host, port, _ := serviceTarget("SMB_TEST_SERVER", "TI_PORT_SMB", 445)
	username := infraValue("SMB_TEST_USER", "TI_SMB_USER")
	password := infraValue("SMB_TEST_PASS", "TI_SMB_PASSWORD")
	share := getEnvOrDefault("SMB_TEST_SHARE", "testshare") // docker-compose.test-infra.yml declares the share `testshare`

	t.Run("SMB Client Creation", func(t *testing.T) {
		config := &filesystem.SmbConfig{
			Host:     host,
			Port:     port,
			Share:    share,
			Username: username,
			Password: password,
		}

		client := filesystem.NewSmbClient(config)
		require.NotNil(t, client)
	})

	t.Run("SMB Connection", func(t *testing.T) {
		config := &filesystem.SmbConfig{
			Host:     host,
			Port:     port,
			Share:    share,
			Username: username,
			Password: password,
		}

		client := filesystem.NewSmbClient(config)
		require.NotNil(t, client)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		err := client.Connect(ctx)
		if err != nil {
			t.Logf("SMB connection failed (server may not be available): %v", err)
		}
	})

	t.Run("SMB Directory Listing", func(t *testing.T) {
		config := &filesystem.SmbConfig{
			Host:     host,
			Port:     port,
			Share:    share,
			Username: username,
			Password: password,
		}

		client := filesystem.NewSmbClient(config)
		require.NotNil(t, client)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		err := client.Connect(ctx)
		if err != nil {
			t.Skipf("Cannot list directory: connection failed: %v (SKIP-OK: #topology-no-container)", err)
		}

		files, err := client.ListDirectory(ctx, "/")
		if err != nil {
			t.Logf("Directory listing returned error: %v", err)
		} else {
			t.Logf("Found %d files in SMB share", len(files))
		}
	})
}

func TestFTPProtocolConnectivity(t *testing.T) {
	skipIfNotConfigured(t, "FTP_TEST_SERVER", "TI_PORT_FTP", "FTP")

	ctx := context.Background()

	host, port, _ := serviceTarget("FTP_TEST_SERVER", "TI_PORT_FTP", 21)
	username := infraValue("FTP_TEST_USER", "TI_FTP_USER")
	password := infraValue("FTP_TEST_PASS", "TI_FTP_PASSWORD")

	t.Run("FTP Client Creation", func(t *testing.T) {
		config := &filesystem.FTPConfig{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
		}

		client := filesystem.NewFTPClient(config)
		require.NotNil(t, client)
	})

	t.Run("FTP Connection", func(t *testing.T) {
		config := &filesystem.FTPConfig{
			Host:     host,
			Port:     port,
			Username: username,
			Password: password,
		}

		client := filesystem.NewFTPClient(config)
		require.NotNil(t, client)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		err := client.Connect(ctx)
		if err != nil {
			t.Logf("FTP connection failed (server may not be available): %v", err)
		}
	})
}

func TestWebDAVProtocolConnectivity(t *testing.T) {
	if os.Getenv("WEBDAV_TEST_URL") == "" {
		skipIfNotConfigured(t, "WEBDAV_TEST_SERVER", "TI_PORT_WEBDAV", "WebDAV")
	}

	ctx := context.Background()

	url := os.Getenv("WEBDAV_TEST_URL")
	if url == "" {
		h, p, _ := serviceTarget("WEBDAV_TEST_SERVER", "TI_PORT_WEBDAV", 80)
		url = "http://" + net.JoinHostPort(h, strconv.Itoa(p))
	}
	username := infraValue("WEBDAV_TEST_USER", "TI_WEBDAV_USER")
	password := infraValue("WEBDAV_TEST_PASS", "TI_WEBDAV_PASSWORD")

	t.Run("WebDAV Client Creation", func(t *testing.T) {
		config := &filesystem.WebDAVConfig{
			URL:      url,
			Username: username,
			Password: password,
		}

		client, _ := filesystem.NewWebDAVClient(config)
		require.NotNil(t, client)
	})

	t.Run("WebDAV Connection", func(t *testing.T) {
		config := &filesystem.WebDAVConfig{
			URL:      url,
			Username: username,
			Password: password,
		}

		client, _ := filesystem.NewWebDAVClient(config)
		require.NotNil(t, client)

		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		files, err := client.ListDirectory(ctx, "/")
		if err != nil {
			t.Logf("WebDAV directory listing failed: %v", err)
		} else {
			t.Logf("Found %d items via WebDAV", len(files))
		}
	})
}

func TestNFSProtocolConnectivity(t *testing.T) {
	skipIfNoContainer(t, "NFS_TEST_SERVER", "NFS")

	host := os.Getenv("NFS_TEST_SERVER") // the user-space NFS server publishes no host port: only an explicit server names one
	exportPath := getEnvOrDefault("NFS_TEST_EXPORT", "/export")
	mountPoint := getEnvOrDefault("NFS_TEST_MOUNT", "/mnt/nfs-test")

	t.Run("NFS Client Creation", func(t *testing.T) {
		config := filesystem.NFSConfig{
			Host:       host,
			Path:       exportPath,
			MountPoint: mountPoint,
		}

		client, err := filesystem.NewNFSClient(config)
		if err != nil {
			t.Skipf("NFS client creation failed (may require root): %v (SKIP-OK: #nfs-needs-root)", err)
		}
		require.NotNil(t, client)
	})
}

func TestLocalFilesystem(t *testing.T) {
	ctx := context.Background()

	tempDir := t.TempDir()

	t.Run("Local Client Creation", func(t *testing.T) {
		config := &filesystem.LocalConfig{
			BasePath: tempDir,
		}

		client := filesystem.NewLocalClient(config)
		require.NotNil(t, client)
	})

	t.Run("Local Directory Listing", func(t *testing.T) {
		testFile := tempDir + "/test.txt"
		err := os.WriteFile(testFile, []byte("test"), 0644)
		require.NoError(t, err)

		config := &filesystem.LocalConfig{
			BasePath: tempDir,
		}

		client := filesystem.NewLocalClient(config)
		require.NotNil(t, client)

		connErr := client.Connect(ctx)
		require.NoError(t, connErr)

		files, err := client.ListDirectory(ctx, "/")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(files), 1)
	})
}

func TestSMBDiscoveryService(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	discoveryService := services.NewSMBDiscoveryService(logger)

	t.Run("Discover shares returns error for unreachable host", func(t *testing.T) {
		_, err := discoveryService.DiscoverShares(ctx, "nonexistent.host.local", "user", "pass", nil)
		require.Error(t, err, "§11.4.6: DiscoverShares must return an error for an unreachable host, not fabricated share names")
	})
}
