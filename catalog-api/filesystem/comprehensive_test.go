package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Factory Helper Functions — getStringSetting / getIntSetting
// =============================================================================

func TestGetStringSetting(t *testing.T) {
	tests := []struct {
		name         string
		settings     map[string]interface{}
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "key exists with string value",
			settings:     map[string]interface{}{"host": "example.com"},
			key:          "host",
			defaultValue: "localhost",
			expected:     "example.com",
		},
		{
			name:         "key missing returns default",
			settings:     map[string]interface{}{},
			key:          "host",
			defaultValue: "localhost",
			expected:     "localhost",
		},
		{
			name:         "key exists with non-string value returns default",
			settings:     map[string]interface{}{"port": 445},
			key:          "port",
			defaultValue: "default",
			expected:     "default",
		},
		{
			name:         "nil settings map returns default",
			settings:     nil,
			key:          "host",
			defaultValue: "fallback",
			expected:     "fallback",
		},
		{
			name:         "empty string value",
			settings:     map[string]interface{}{"host": ""},
			key:          "host",
			defaultValue: "default",
			expected:     "",
		},
		{
			name:         "key exists with bool value returns default",
			settings:     map[string]interface{}{"enabled": true},
			key:          "enabled",
			defaultValue: "false",
			expected:     "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringSetting(tt.settings, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetIntSetting(t *testing.T) {
	tests := []struct {
		name         string
		settings     map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "key exists with int value",
			settings:     map[string]interface{}{"port": 8080},
			key:          "port",
			defaultValue: 21,
			expected:     8080,
		},
		{
			name:         "key exists with float64 value (JSON numbers)",
			settings:     map[string]interface{}{"port": float64(445)},
			key:          "port",
			defaultValue: 21,
			expected:     445,
		},
		{
			name:         "key missing returns default",
			settings:     map[string]interface{}{},
			key:          "port",
			defaultValue: 21,
			expected:     21,
		},
		{
			name:         "key exists with string value returns default",
			settings:     map[string]interface{}{"port": "not-a-number"},
			key:          "port",
			defaultValue: 21,
			expected:     21,
		},
		{
			name:         "nil settings map returns default",
			settings:     nil,
			key:          "port",
			defaultValue: 445,
			expected:     445,
		},
		{
			name:         "zero int value",
			settings:     map[string]interface{}{"port": 0},
			key:          "port",
			defaultValue: 21,
			expected:     0,
		},
		{
			name:         "zero float64 value",
			settings:     map[string]interface{}{"port": float64(0)},
			key:          "port",
			defaultValue: 21,
			expected:     0,
		},
		{
			name:         "negative int value",
			settings:     map[string]interface{}{"offset": -10},
			key:          "offset",
			defaultValue: 0,
			expected:     -10,
		},
		{
			name:         "key exists with bool value returns default",
			settings:     map[string]interface{}{"port": true},
			key:          "port",
			defaultValue: 21,
			expected:     21,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntSetting(tt.settings, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// Factory — CreateClient with detailed settings extraction
// =============================================================================

func TestDefaultClientFactory_CreateClient_SettingsExtraction(t *testing.T) {
	factory := NewDefaultClientFactory()

	t.Run("SMB settings extraction with defaults", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "smb",
			Settings: map[string]interface{}{
				"host":  "myserver",
				"share": "myshare",
				// port, username, password, domain omitted — use defaults
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)
		assert.Equal(t, "smb", client.GetProtocol())

		smbCfg := client.GetConfig().(*SmbConfig)
		assert.Equal(t, "myserver", smbCfg.Host)
		assert.Equal(t, 445, smbCfg.Port) // default
		assert.Equal(t, "myshare", smbCfg.Share)
		assert.Equal(t, "", smbCfg.Username)        // default
		assert.Equal(t, "", smbCfg.Password)        // default
		assert.Equal(t, "WORKGROUP", smbCfg.Domain) // default
	})

	t.Run("FTP settings extraction with defaults", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "ftp",
			Settings: map[string]interface{}{
				"host": "ftpserver",
				// port omitted — default 21
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)
		assert.Equal(t, "ftp", client.GetProtocol())

		ftpCfg := client.GetConfig().(*FTPConfig)
		assert.Equal(t, "ftpserver", ftpCfg.Host)
		assert.Equal(t, 21, ftpCfg.Port) // default
		assert.Equal(t, "", ftpCfg.Username)
		assert.Equal(t, "", ftpCfg.Password)
		assert.Equal(t, "", ftpCfg.Path)
	})

	t.Run("NFS settings extraction with defaults", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "nfs",
			Settings: map[string]interface{}{
				"host":        "nfsserver",
				"path":        "/export",
				"mount_point": "/tmp/catalog-test-nfs",
				// options omitted — default "vers=3"
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)
		assert.Equal(t, "nfs", client.GetProtocol())

		nfsCfg := client.GetConfig().(*NFSConfig)
		assert.Equal(t, "nfsserver", nfsCfg.Host)
		assert.Equal(t, "/export", nfsCfg.Path)
		assert.Equal(t, "/tmp/catalog-test-nfs", nfsCfg.MountPoint)
		assert.Equal(t, "vers=3", nfsCfg.Options) // default
	})

	t.Run("NFS with empty mount_point returns error", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "nfs",
			Settings: map[string]interface{}{
				"host": "nfsserver",
				"path": "/export",
				// mount_point omitted — empty string
			},
		}
		_, err := factory.CreateClient(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create NFS client")
	})

	t.Run("WebDAV settings extraction", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "webdav",
			Settings: map[string]interface{}{
				"url":      "https://webdav.test.com",
				"username": "user1",
				"password": "pass1",
				"path":     "/remote",
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)
		assert.Equal(t, "webdav", client.GetProtocol())

		wdCfg := client.GetConfig().(*WebDAVConfig)
		assert.Equal(t, "https://webdav.test.com", wdCfg.URL)
		assert.Equal(t, "user1", wdCfg.Username)
		assert.Equal(t, "pass1", wdCfg.Password)
		assert.Equal(t, "/remote", wdCfg.Path)
	})

	t.Run("Local settings extraction", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "local",
			Settings: map[string]interface{}{
				"base_path": "/media/library",
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)
		assert.Equal(t, "local", client.GetProtocol())

		localCfg := client.GetConfig().(*LocalConfig)
		assert.Equal(t, "/media/library", localCfg.BasePath)
	})

	t.Run("SMB with float64 port from JSON", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "smb",
			Settings: map[string]interface{}{
				"host":  "server",
				"port":  float64(1445),
				"share": "data",
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)

		smbCfg := client.GetConfig().(*SmbConfig)
		assert.Equal(t, 1445, smbCfg.Port)
	})

	t.Run("FTP with float64 port from JSON", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "ftp",
			Settings: map[string]interface{}{
				"host": "ftphost",
				"port": float64(2121),
			},
		}
		client, err := factory.CreateClient(config)
		require.NoError(t, err)

		ftpCfg := client.GetConfig().(*FTPConfig)
		assert.Equal(t, 2121, ftpCfg.Port)
	})

	t.Run("unsupported protocol with empty settings", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "sftp",
			Settings: map[string]interface{}{},
		}
		_, err := factory.CreateClient(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported protocol: sftp")
	})
}

// =============================================================================
// Factory — SupportedProtocols
// =============================================================================

func TestDefaultClientFactory_SupportedProtocols_Contents(t *testing.T) {
	factory := NewDefaultClientFactory()
	protocols := factory.SupportedProtocols()

	expectedProtocols := []string{"smb", "ftp", "nfs", "webdav", "local"}
	assert.Equal(t, expectedProtocols, protocols)

	// Verify each protocol can create a client
	for _, proto := range protocols {
		t.Run("create_"+proto, func(t *testing.T) {
			settings := map[string]interface{}{}
			switch proto {
			case "nfs":
				settings["mount_point"] = "/tmp/test-nfs-" + proto
			}
			config := &StorageConfig{
				Protocol: proto,
				Settings: settings,
			}
			client, err := factory.CreateClient(config)
			require.NoError(t, err)
			assert.Equal(t, proto, client.GetProtocol())
		})
	}
}

// =============================================================================
// Interface Compliance — all client types implement FileSystemClient
// =============================================================================

func TestInterfaceCompliance(t *testing.T) {
	t.Run("LocalClient implements FileSystemClient", func(t *testing.T) {
		var _ FileSystemClient = (*LocalClient)(nil)
	})

	t.Run("FTPClient implements FileSystemClient", func(t *testing.T) {
		var _ FileSystemClient = (*FTPClient)(nil)
	})

	t.Run("SmbClient implements FileSystemClient", func(t *testing.T) {
		var _ FileSystemClient = (*SmbClient)(nil)
	})

	t.Run("WebDAVClient implements FileSystemClient", func(t *testing.T) {
		var _ FileSystemClient = (*WebDAVClient)(nil)
	})

	t.Run("NFSClient implements FileSystemClient", func(t *testing.T) {
		var _ FileSystemClient = (*NFSClient)(nil)
	})
}

// =============================================================================
// LocalClient — CreateDirectory, DeleteDirectory, DeleteFile, CopyFile, GetConfig
// =============================================================================

func newConnectedLocalClient(t *testing.T) (*LocalClient, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "local_client_comprehensive_test")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	config := &LocalConfig{BasePath: tempDir}
	client := NewLocalClient(config)
	err = client.Connect(context.Background())
	require.NoError(t, err)

	return client, tempDir
}

func TestLocalClient_GetConfig(t *testing.T) {
	config := &LocalConfig{BasePath: "/media/library"}
	client := NewLocalClient(config)

	retrieved := client.GetConfig().(*LocalConfig)
	assert.Equal(t, "/media/library", retrieved.BasePath)
}

func TestLocalClient_CreateDirectory(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	t.Run("create simple directory", func(t *testing.T) {
		err := client.CreateDirectory(ctx, "newdir")
		require.NoError(t, err)

		info, err := os.Stat(filepath.Join(tempDir, "newdir"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("create nested directory", func(t *testing.T) {
		err := client.CreateDirectory(ctx, "a/b/c")
		require.NoError(t, err)

		info, err := os.Stat(filepath.Join(tempDir, "a/b/c"))
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("create directory that already exists", func(t *testing.T) {
		err := client.CreateDirectory(ctx, "newdir")
		assert.NoError(t, err) // MkdirAll doesn't error on existing
	})
}

func TestLocalClient_CreateDirectory_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.CreateDirectory(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_DeleteDirectory(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	t.Run("delete empty directory", func(t *testing.T) {
		dirPath := filepath.Join(tempDir, "todelete")
		require.NoError(t, os.Mkdir(dirPath, 0755))

		err := client.DeleteDirectory(ctx, "todelete")
		require.NoError(t, err)

		_, err = os.Stat(dirPath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("delete directory with contents", func(t *testing.T) {
		dirPath := filepath.Join(tempDir, "todelete2")
		require.NoError(t, os.MkdirAll(filepath.Join(dirPath, "subdir"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dirPath, "file.txt"), []byte("data"), 0644))

		err := client.DeleteDirectory(ctx, "todelete2")
		require.NoError(t, err)

		_, err = os.Stat(dirPath)
		assert.True(t, os.IsNotExist(err))
	})
}

func TestLocalClient_DeleteDirectory_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.DeleteDirectory(ctx, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_DeleteFile(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	t.Run("delete existing file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "deleteme.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		err := client.DeleteFile(ctx, "deleteme.txt")
		require.NoError(t, err)

		_, err = os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("delete nonexistent file returns error", func(t *testing.T) {
		err := client.DeleteFile(ctx, "nonexistent.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete local file")
	})
}

func TestLocalClient_DeleteFile_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.DeleteFile(ctx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_CopyFile(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	t.Run("copy file to same directory", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "source.txt")
		require.NoError(t, os.WriteFile(srcPath, []byte("copy me"), 0644))

		err := client.CopyFile(ctx, "source.txt", "destination.txt")
		require.NoError(t, err)

		dstContent, err := os.ReadFile(filepath.Join(tempDir, "destination.txt"))
		require.NoError(t, err)
		assert.Equal(t, "copy me", string(dstContent))

		// Verify source still exists
		srcContent, err := os.ReadFile(srcPath)
		require.NoError(t, err)
		assert.Equal(t, "copy me", string(srcContent))
	})

	t.Run("copy file to nested directory", func(t *testing.T) {
		srcPath := filepath.Join(tempDir, "src2.txt")
		require.NoError(t, os.WriteFile(srcPath, []byte("nested copy"), 0644))

		err := client.CopyFile(ctx, "src2.txt", "deep/nested/dst2.txt")
		require.NoError(t, err)

		dstContent, err := os.ReadFile(filepath.Join(tempDir, "deep/nested/dst2.txt"))
		require.NoError(t, err)
		assert.Equal(t, "nested copy", string(dstContent))
	})

	t.Run("copy nonexistent source returns error", func(t *testing.T) {
		err := client.CopyFile(ctx, "no_such_file.txt", "dst.txt")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to open source file")
	})
}

func TestLocalClient_CopyFile_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.CopyFile(ctx, "a.txt", "b.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// LocalClient — Connect error paths
// =============================================================================

func TestLocalClient_Connect_NonexistentPath(t *testing.T) {
	config := &LocalConfig{BasePath: "/nonexistent/path/that/does/not/exist"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to access base path")
}

func TestLocalClient_Connect_FileInsteadOfDir(t *testing.T) {
	tempFile, err := os.CreateTemp("", "local_client_file_test")
	require.NoError(t, err)
	defer os.Remove(tempFile.Name())
	tempFile.Close()

	config := &LocalConfig{BasePath: tempFile.Name()}
	client := NewLocalClient(config)
	ctx := context.Background()

	err = client.Connect(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is not a directory")
}

// =============================================================================
// LocalClient — TestConnection error when not connected
// =============================================================================

func TestLocalClient_TestConnection_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.TestConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// LocalClient — ReadFile / WriteFile when not connected
// =============================================================================

func TestLocalClient_ReadFile_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	_, err := client.ReadFile(ctx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_WriteFile_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	err := client.WriteFile(ctx, "test.txt", bytes.NewReader([]byte("data")))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_GetFileInfo_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	_, err := client.GetFileInfo(ctx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_ListDirectory_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	_, err := client.ListDirectory(ctx, ".")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

func TestLocalClient_FileExists_NotConnected(t *testing.T) {
	config := &LocalConfig{BasePath: "/tmp"}
	client := NewLocalClient(config)
	ctx := context.Background()

	_, err := client.FileExists(ctx, "test.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")
}

// =============================================================================
// LocalClient — resolvePath with directory traversal
// =============================================================================

func TestLocalClient_ResolvePath_DirectoryTraversal(t *testing.T) {
	config := &LocalConfig{BasePath: "/base/path"}
	client := NewLocalClient(config)

	tests := []struct {
		name  string
		input string
	}{
		{"simple traversal", "../etc/passwd"},
		{"double traversal", "../../etc/shadow"},
		{"nested traversal", "data/../../etc/hosts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved := client.resolvePath(tt.input)
			assert.NotContains(t, resolved, "..")
			assert.True(t, strings.HasPrefix(resolved, "/base/path"))
		})
	}
}

func TestLocalClient_ResolvePath_NormalPaths(t *testing.T) {
	config := &LocalConfig{BasePath: "/base"}
	client := NewLocalClient(config)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple file", "file.txt", "/base/file.txt"},
		{"nested file", "a/b/c.txt", "/base/a/b/c.txt"},
		{"current dir", ".", "/base"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := client.resolvePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// LocalClient — WriteFile to nested path creates directories
// =============================================================================

func TestLocalClient_WriteFile_CreatesNestedDirs(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	content := "nested content"
	err := client.WriteFile(ctx, "deep/nested/dir/file.txt", bytes.NewReader([]byte(content)))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "deep/nested/dir/file.txt"))
	require.NoError(t, err)
	assert.Equal(t, content, string(data))
}

// =============================================================================
// LocalClient — ReadFile nonexistent
// =============================================================================

func TestLocalClient_ReadFile_Nonexistent(t *testing.T) {
	client, _ := newConnectedLocalClient(t)
	ctx := context.Background()

	_, err := client.ReadFile(ctx, "no_such_file.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open local file")
}

// =============================================================================
// LocalClient — GetFileInfo for directory
// =============================================================================

func TestLocalClient_GetFileInfo_Directory(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	subDir := filepath.Join(tempDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0755))

	info, err := client.GetFileInfo(ctx, "subdir")
	require.NoError(t, err)
	assert.Equal(t, "subdir", info.Name)
	assert.True(t, info.IsDir)
	assert.Equal(t, "subdir", info.Path)
}

func TestLocalClient_GetFileInfo_Nonexistent(t *testing.T) {
	client, _ := newConnectedLocalClient(t)
	ctx := context.Background()

	_, err := client.GetFileInfo(ctx, "nonexistent.txt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stat local file")
}

// =============================================================================
// LocalClient — ListDirectory empty
// =============================================================================

func TestLocalClient_ListDirectory_Empty(t *testing.T) {
	client, _ := newConnectedLocalClient(t)
	ctx := context.Background()

	files, err := client.ListDirectory(ctx, ".")
	require.NoError(t, err)
	assert.Empty(t, files)
}

func TestLocalClient_ListDirectory_Nonexistent(t *testing.T) {
	client, _ := newConnectedLocalClient(t)
	ctx := context.Background()

	_, err := client.ListDirectory(ctx, "no_such_dir")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list local directory")
}

// =============================================================================
// LocalClient — Full read/write/copy/delete lifecycle
// =============================================================================

func TestLocalClient_FullLifecycle(t *testing.T) {
	client, _ := newConnectedLocalClient(t)
	ctx := context.Background()

	// Write a file
	err := client.WriteFile(ctx, "lifecycle.txt", bytes.NewReader([]byte("hello world")))
	require.NoError(t, err)

	// Verify file exists
	exists, err := client.FileExists(ctx, "lifecycle.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	// Get file info
	info, err := client.GetFileInfo(ctx, "lifecycle.txt")
	require.NoError(t, err)
	assert.Equal(t, int64(11), info.Size) // "hello world" = 11 bytes
	assert.False(t, info.IsDir)

	// Read file
	reader, err := client.ReadFile(ctx, "lifecycle.txt")
	require.NoError(t, err)
	data, err := io.ReadAll(reader)
	reader.Close()
	require.NoError(t, err)
	assert.Equal(t, "hello world", string(data))

	// Copy file
	err = client.CopyFile(ctx, "lifecycle.txt", "lifecycle_copy.txt")
	require.NoError(t, err)

	exists, err = client.FileExists(ctx, "lifecycle_copy.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete original
	err = client.DeleteFile(ctx, "lifecycle.txt")
	require.NoError(t, err)

	exists, err = client.FileExists(ctx, "lifecycle.txt")
	require.NoError(t, err)
	assert.False(t, exists)

	// Copy still exists
	exists, err = client.FileExists(ctx, "lifecycle_copy.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	// Create directory, list it
	err = client.CreateDirectory(ctx, "mydir")
	require.NoError(t, err)

	files, err := client.ListDirectory(ctx, ".")
	require.NoError(t, err)
	assert.Equal(t, 2, len(files)) // lifecycle_copy.txt + mydir

	// Delete directory
	err = client.DeleteDirectory(ctx, "mydir")
	require.NoError(t, err)

	exists, err = client.FileExists(ctx, "mydir")
	require.NoError(t, err)
	assert.False(t, exists)

	// Disconnect
	err = client.Disconnect(ctx)
	require.NoError(t, err)
	assert.False(t, client.IsConnected())
}

// =============================================================================
// LocalClient — Connect/Disconnect/TestConnection state transitions
// =============================================================================

func TestLocalClient_StateTransitions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "local_state_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	config := &LocalConfig{BasePath: tempDir}
	client := NewLocalClient(config)
	ctx := context.Background()

	// Initially not connected
	assert.False(t, client.IsConnected())

	// Connect
	require.NoError(t, client.Connect(ctx))
	assert.True(t, client.IsConnected())

	// TestConnection succeeds
	require.NoError(t, client.TestConnection(ctx))

	// Disconnect
	require.NoError(t, client.Disconnect(ctx))
	assert.False(t, client.IsConnected())

	// TestConnection fails after disconnect
	err = client.TestConnection(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not connected")

	// Reconnect
	require.NoError(t, client.Connect(ctx))
	assert.True(t, client.IsConnected())
}

// =============================================================================
// FTPClient — resolvePath
// =============================================================================

func TestFTPClient_ResolvePath(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		input    string
		expected string
	}{
		{
			name:     "with base path",
			basePath: "/base/dir",
			input:    "file.txt",
			expected: "/base/dir/file.txt",
		},
		{
			name:     "without base path",
			basePath: "",
			input:    "file.txt",
			expected: "file.txt",
		},
		{
			name:     "nested path with base",
			basePath: "/media",
			input:    "movies/action/film.mkv",
			expected: "/media/movies/action/film.mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewFTPClient(&FTPConfig{
				Host: "localhost",
				Port: 21,
				Path: tt.basePath,
			})
			result := client.resolvePath(client.base, tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// SmbClient — isNotExistError
// =============================================================================

func TestIsNotExistError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "file does not exist",
			err:      fmt.Errorf("file does not exist"),
			expected: true,
		},
		{
			name:     "no such file or directory",
			err:      fmt.Errorf("no such file or directory"),
			expected: true,
		},
		{
			name:     "other error",
			err:      fmt.Errorf("connection refused"),
			expected: false,
		},
		{
			name:     "permission denied",
			err:      fmt.Errorf("permission denied"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNotExistError(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// =============================================================================
// DirectoryTreeInfo — additional tests
// =============================================================================

func TestDirectoryTreeInfo_Empty(t *testing.T) {
	tree := DirectoryTreeInfo{
		Path:       "/empty",
		TotalFiles: 0,
		TotalDirs:  0,
		TotalSize:  0,
		MaxDepth:   0,
		Files:      nil,
		Subdirs:    nil,
	}

	assert.Equal(t, "/empty", tree.Path)
	assert.Equal(t, 0, tree.TotalFiles)
	assert.Equal(t, 0, tree.TotalDirs)
	assert.Equal(t, int64(0), tree.TotalSize)
	assert.Equal(t, 0, tree.MaxDepth)
	assert.Nil(t, tree.Files)
	assert.Nil(t, tree.Subdirs)
}

func TestDirectoryTreeInfo_DeepNesting(t *testing.T) {
	level3 := &DirectoryTreeInfo{
		Path:       "/root/l1/l2/l3",
		TotalFiles: 1,
		TotalDirs:  0,
		TotalSize:  100,
		MaxDepth:   0,
		Files:      []*FileInfo{{Name: "deep.txt", Size: 100, Path: "/root/l1/l2/l3/deep.txt"}},
	}

	level2 := &DirectoryTreeInfo{
		Path:       "/root/l1/l2",
		TotalFiles: 0,
		TotalDirs:  1,
		TotalSize:  100,
		MaxDepth:   1,
		Subdirs:    []*DirectoryTreeInfo{level3},
	}

	level1 := &DirectoryTreeInfo{
		Path:       "/root/l1",
		TotalFiles: 0,
		TotalDirs:  1,
		TotalSize:  100,
		MaxDepth:   2,
		Subdirs:    []*DirectoryTreeInfo{level2},
	}

	root := DirectoryTreeInfo{
		Path:       "/root",
		TotalFiles: 2,
		TotalDirs:  1,
		TotalSize:  300,
		MaxDepth:   3,
		Files:      []*FileInfo{{Name: "readme.md", Size: 200}},
		Subdirs:    []*DirectoryTreeInfo{level1},
	}

	assert.Equal(t, 3, root.MaxDepth)
	assert.Equal(t, 1, len(root.Subdirs))
	assert.Equal(t, 1, len(root.Subdirs[0].Subdirs))
	assert.Equal(t, 1, len(root.Subdirs[0].Subdirs[0].Subdirs))
	assert.Equal(t, 1, len(root.Subdirs[0].Subdirs[0].Subdirs[0].Files))
	assert.Equal(t, "deep.txt", root.Subdirs[0].Subdirs[0].Subdirs[0].Files[0].Name)
}

// =============================================================================
// WebDAVClient — resolveURL additional cases
// =============================================================================

func TestWebDAVClient_ResolveURL_NoBasePath(t *testing.T) {
	config := &WebDAVConfig{
		URL: "https://example.com",
	}

	client, _ := NewWebDAVClient(config)

	result := client.resolveURL("file.txt")
	assert.Contains(t, result, "https://example.com")
	assert.Contains(t, result, "file.txt")
}

func TestWebDAVClient_ResolveURL_WithPort(t *testing.T) {
	config := &WebDAVConfig{
		URL:  "https://example.com:8443",
		Path: "/dav",
	}

	client, _ := NewWebDAVClient(config)

	result := client.resolveURL("doc.pdf")
	assert.Contains(t, result, "example.com:8443")
	assert.Contains(t, result, "doc.pdf")
}

// =============================================================================
// DefaultClientFactory — nil settings handling
// =============================================================================

func TestDefaultClientFactory_CreateClient_NilSettings(t *testing.T) {
	factory := NewDefaultClientFactory()

	tests := []struct {
		name     string
		protocol string
	}{
		{"local with nil settings", "local"},
		{"smb with nil settings", "smb"},
		{"ftp with nil settings", "ftp"},
		{"webdav with nil settings", "webdav"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &StorageConfig{
				Protocol: tt.protocol,
				Settings: nil,
			}
			client, err := factory.CreateClient(config)
			// All except NFS should succeed with nil settings (defaults kick in)
			require.NoError(t, err)
			assert.Equal(t, tt.protocol, client.GetProtocol())
		})
	}

	t.Run("nfs with nil settings fails", func(t *testing.T) {
		config := &StorageConfig{
			Protocol: "nfs",
			Settings: nil,
		}
		_, err := factory.CreateClient(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "mount point is required")
	})
}

// =============================================================================
// LocalClient — CopyFile with large content
// =============================================================================

func TestLocalClient_CopyFile_LargeContent(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	// Create a file with 1MB of data
	largeContent := bytes.Repeat([]byte("A"), 1024*1024)
	srcPath := filepath.Join(tempDir, "large.bin")
	require.NoError(t, os.WriteFile(srcPath, largeContent, 0644))

	err := client.CopyFile(ctx, "large.bin", "large_copy.bin")
	require.NoError(t, err)

	dstContent, err := os.ReadFile(filepath.Join(tempDir, "large_copy.bin"))
	require.NoError(t, err)
	assert.Equal(t, len(largeContent), len(dstContent))
	assert.Equal(t, largeContent, dstContent)
}

// =============================================================================
// LocalClient — ListDirectory with mixed entries
// =============================================================================

func TestLocalClient_ListDirectory_MixedEntries(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	// Create various entries
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "b.mp4"), []byte("bb"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir1"), 0755))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir2"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "c.mkv"), []byte("ccc"), 0644))

	files, err := client.ListDirectory(ctx, ".")
	require.NoError(t, err)
	assert.Equal(t, 5, len(files))

	// Verify file sizes
	sizeMap := make(map[string]int64)
	dirMap := make(map[string]bool)
	for _, f := range files {
		sizeMap[f.Name] = f.Size
		dirMap[f.Name] = f.IsDir
	}

	assert.Equal(t, int64(1), sizeMap["a.txt"])
	assert.Equal(t, int64(2), sizeMap["b.mp4"])
	assert.Equal(t, int64(3), sizeMap["c.mkv"])
	assert.True(t, dirMap["subdir1"])
	assert.True(t, dirMap["subdir2"])
	assert.False(t, dirMap["a.txt"])
}

// =============================================================================
// LocalClient — FileExists edge cases
// =============================================================================

func TestLocalClient_FileExists_DirectoryExists(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "adir"), 0755))

	exists, err := client.FileExists(ctx, "adir")
	require.NoError(t, err)
	assert.True(t, exists) // FileExists returns true for directories too (os.Stat behavior)
}

// =============================================================================
// WebDAVClient — NewWebDAVClient with invalid URL
// =============================================================================

func TestWebDAVClient_NewWebDAVClient_InvalidURL(t *testing.T) {
	config := &WebDAVConfig{
		URL: "://not-a-valid-url",
	}

	client, err := NewWebDAVClient(config)
	// Should return an error for invalid URL
	assert.Error(t, err)
	assert.Nil(t, client)
}

// =============================================================================
// LocalClient — WriteFile overwrites existing
// =============================================================================

func TestLocalClient_WriteFile_Overwrite(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	// Write initial content
	err := client.WriteFile(ctx, "overwrite.txt", bytes.NewReader([]byte("original")))
	require.NoError(t, err)

	// Overwrite
	err = client.WriteFile(ctx, "overwrite.txt", bytes.NewReader([]byte("new content")))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "overwrite.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new content", string(data))
}

// =============================================================================
// NFSClient — isMounted exercised through IsConnected
// =============================================================================

func TestNFSClient_IsMounted_ViaIsConnected(t *testing.T) {
	config := NFSConfig{
		Host:       "localhost",
		Path:       "/export",
		MountPoint: "/tmp/nfs-test-mount",
	}

	client, err := NewNFSClient(config)
	require.NoError(t, err)

	// Manually set internal state and present a kernel mount table that names the mount point as NFS: IsConnected asks the table (WF22 N1)
	client.connected = true
	client.mounted = true
	withProcMounts(t, "nas:/export /tmp/nfs-test-mount nfs4 rw 0 0\n")
	assert.True(t, client.IsConnected())
	// ... and without the mount in the table the same state is NOT connected: the old stub answered true for any readable /proc/mounts
	withProcMounts(t, "proc /proc proc rw 0 0\n")
	assert.False(t, client.IsConnected())
}

// newFakeConnectedNFSClient creates an NFSClient with a real temp directory
// as its mount point, then sets internal state to simulate a connected mount.
// This lets us test the file operations (which just use os.* functions under
// the mount point) without actually mounting NFS.
func newFakeConnectedNFSClient(t *testing.T) (*NFSClient, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "nfs_fake_mount")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	config := NFSConfig{
		Host:       "localhost",
		Path:       "/export",
		MountPoint: tempDir,
	}
	client, err := NewNFSClient(config)
	require.NoError(t, err)

	// Simulate connected state: the kernel mount table (the seam IsConnected really reads since WF22 N1) names the directory as an NFS mount.
	// The old stub that answered "mounted" for any readable /proc/mounts is what let a Connect that never mounted pass for a connection.
	withProcMounts(t, "nas:/export "+tempDir+" nfs4 rw 0 0\n")
	client.connected = true
	client.mounted = true

	return client, tempDir
}

func TestNFSClient_WriteFile_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	err := client.WriteFile(ctx, "test.txt", bytes.NewReader([]byte("nfs content")))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "nfs content", string(data))
}

func TestNFSClient_ReadFile_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "read.txt"), []byte("hello"), 0644))

	reader, err := client.ReadFile(ctx, "read.txt")
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "hello", string(data))
}

func TestNFSClient_GetFileInfo_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "info.txt"), []byte("12345"), 0644))

	info, err := client.GetFileInfo(ctx, "info.txt")
	require.NoError(t, err)
	assert.Equal(t, "info.txt", info.Name)
	assert.Equal(t, int64(5), info.Size)
	assert.False(t, info.IsDir)
}

func TestNFSClient_ListDirectory_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a.txt"), []byte("a"), 0644))
	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "subdir"), 0755))

	files, err := client.ListDirectory(ctx, ".")
	require.NoError(t, err)
	assert.Equal(t, 2, len(files))
}

func TestNFSClient_FileExists_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "exists.txt"), []byte("x"), 0644))

	exists, err := client.FileExists(ctx, "exists.txt")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = client.FileExists(ctx, "missing.txt")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestNFSClient_CreateDirectory_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	err := client.CreateDirectory(ctx, "newdir")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(tempDir, "newdir"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNFSClient_DeleteDirectory_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.Mkdir(filepath.Join(tempDir, "delme"), 0755))

	err := client.DeleteDirectory(ctx, "delme")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "delme"))
	assert.True(t, os.IsNotExist(err))
}

func TestNFSClient_DeleteFile_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "del.txt"), []byte("x"), 0644))

	err := client.DeleteFile(ctx, "del.txt")
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(tempDir, "del.txt"))
	assert.True(t, os.IsNotExist(err))
}

func TestNFSClient_CopyFile_FakeMount(t *testing.T) {
	client, tempDir := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "src.txt"), []byte("copy me"), 0644))

	err := client.CopyFile(ctx, "src.txt", "dst.txt")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "dst.txt"))
	require.NoError(t, err)
	assert.Equal(t, "copy me", string(data))
}

func TestNFSClient_TestConnection_FakeMount(t *testing.T) {
	client, _ := newFakeConnectedNFSClient(t)
	ctx := context.Background()

	err := client.TestConnection(ctx)
	require.NoError(t, err)
}

// =============================================================================
// LocalClient — WriteFile error paths
// =============================================================================

func TestLocalClient_WriteFile_EmptyContent(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	err := client.WriteFile(ctx, "empty.txt", bytes.NewReader([]byte{}))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "empty.txt"))
	require.NoError(t, err)
	assert.Equal(t, 0, len(data))
}

func TestLocalClient_WriteFile_LargeContent(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	largeData := bytes.Repeat([]byte("X"), 1024*100) // 100KB
	err := client.WriteFile(ctx, "large.bin", bytes.NewReader(largeData))
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(tempDir, "large.bin"))
	require.NoError(t, err)
	assert.Equal(t, len(largeData), len(data))
}

// =============================================================================
// LocalClient — ReadFile and verify full content with io.ReadAll
// =============================================================================

func TestLocalClient_ReadFile_FullContent(t *testing.T) {
	client, tempDir := newConnectedLocalClient(t)
	ctx := context.Background()

	expected := "line1\nline2\nline3\n"
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "multiline.txt"), []byte(expected), 0644))

	reader, err := client.ReadFile(ctx, "multiline.txt")
	require.NoError(t, err)
	defer reader.Close()

	data, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, expected, string(data))
}
