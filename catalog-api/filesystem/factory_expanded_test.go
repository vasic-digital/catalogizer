package filesystem

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// getStringSetting helper
// ---------------------------------------------------------------------------

func TestGetStringSetting_Expanded(t *testing.T) {
	tests := []struct {
		name         string
		settings     map[string]interface{}
		key          string
		defaultValue string
		expected     string
	}{
		{
			name:         "key exists with string value",
			settings:     map[string]interface{}{"host": "192.168.1.100"},
			key:          "host",
			defaultValue: "",
			expected:     "192.168.1.100",
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
			settings:     map[string]interface{}{"host": 12345},
			key:          "host",
			defaultValue: "fallback",
			expected:     "fallback",
		},
		{
			name:         "nil settings returns default",
			settings:     nil,
			key:          "host",
			defaultValue: "default-host",
			expected:     "default-host",
		},
		{
			name:         "empty string value returns empty",
			settings:     map[string]interface{}{"host": ""},
			key:          "host",
			defaultValue: "fallback",
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStringSetting(tt.settings, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// getIntSetting helper
// ---------------------------------------------------------------------------

func TestGetIntSetting_Expanded(t *testing.T) {
	tests := []struct {
		name         string
		settings     map[string]interface{}
		key          string
		defaultValue int
		expected     int
	}{
		{
			name:         "key exists with int value",
			settings:     map[string]interface{}{"port": 445},
			key:          "port",
			defaultValue: 0,
			expected:     445,
		},
		{
			name:         "key exists with float64 value (JSON decoding)",
			settings:     map[string]interface{}{"port": float64(8080)},
			key:          "port",
			defaultValue: 0,
			expected:     8080,
		},
		{
			name:         "key missing returns default",
			settings:     map[string]interface{}{},
			key:          "port",
			defaultValue: 21,
			expected:     21,
		},
		{
			name:         "key exists with non-numeric type returns default",
			settings:     map[string]interface{}{"port": "not-a-number"},
			key:          "port",
			defaultValue: 443,
			expected:     443,
		},
		{
			name:         "nil settings returns default",
			settings:     nil,
			key:          "port",
			defaultValue: 22,
			expected:     22,
		},
		{
			name:         "zero int value",
			settings:     map[string]interface{}{"port": 0},
			key:          "port",
			defaultValue: 8080,
			expected:     0,
		},
		{
			name:         "negative int value",
			settings:     map[string]interface{}{"timeout": -1},
			key:          "timeout",
			defaultValue: 30,
			expected:     -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getIntSetting(tt.settings, tt.key, tt.defaultValue)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// DefaultClientFactory — expanded protocol tests
// ---------------------------------------------------------------------------

func TestDefaultClientFactory_CreateClient_SMBDefaults(t *testing.T) {
	factory := NewDefaultClientFactory()

	// Minimal settings — factory should use defaults for missing keys
	config := &StorageConfig{
		Protocol: "smb",
		Settings: map[string]interface{}{},
	}

	client, err := factory.CreateClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "smb", client.GetProtocol())
}

func TestDefaultClientFactory_CreateClient_FTPDefaults(t *testing.T) {
	factory := NewDefaultClientFactory()

	config := &StorageConfig{
		Protocol: "ftp",
		Settings: map[string]interface{}{},
	}

	client, err := factory.CreateClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "ftp", client.GetProtocol())
}

func TestDefaultClientFactory_CreateClient_LocalDefaults(t *testing.T) {
	factory := NewDefaultClientFactory()

	config := &StorageConfig{
		Protocol: "local",
		Settings: map[string]interface{}{},
	}

	client, err := factory.CreateClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "local", client.GetProtocol())
}

func TestDefaultClientFactory_CreateClient_SMBFloatPort(t *testing.T) {
	factory := NewDefaultClientFactory()

	// JSON decoding produces float64 for numbers
	config := &StorageConfig{
		Protocol: "smb",
		Settings: map[string]interface{}{
			"host":   "nas.local",
			"port":   float64(445),
			"share":  "media",
			"domain": "WORKGROUP",
		},
	}

	client, err := factory.CreateClient(config)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "smb", client.GetProtocol())
}

func TestDefaultClientFactory_CreateClient_MultipleUnsupportedProtocols(t *testing.T) {
	factory := NewDefaultClientFactory()

	unsupported := []string{"s3", "sftp", "hdfs", "gcs", "azure"}
	for _, proto := range unsupported {
		t.Run(proto, func(t *testing.T) {
			config := &StorageConfig{
				Protocol: proto,
				Settings: map[string]interface{}{},
			}
			_, err := factory.CreateClient(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported protocol")
		})
	}
}

// ---------------------------------------------------------------------------
// DirectoryTreeInfo tests
// ---------------------------------------------------------------------------

func TestDirectoryTreeInfo_Fields(t *testing.T) {
	tree := DirectoryTreeInfo{
		Path:       "/media/movies",
		TotalFiles: 100,
		TotalDirs:  10,
		TotalSize:  1024 * 1024 * 500,
		MaxDepth:   3,
		Files:      []*FileInfo{},
		Subdirs:    []*DirectoryTreeInfo{},
	}

	assert.Equal(t, "/media/movies", tree.Path)
	assert.Equal(t, 100, tree.TotalFiles)
	assert.Equal(t, 10, tree.TotalDirs)
	assert.Equal(t, int64(1024*1024*500), tree.TotalSize)
	assert.Equal(t, 3, tree.MaxDepth)
}

func TestDirectoryTreeInfo_NestedStructure(t *testing.T) {
	subdir := &DirectoryTreeInfo{
		Path:       "/media/movies/action",
		TotalFiles: 20,
		TotalDirs:  0,
		TotalSize:  1024 * 1024 * 100,
		MaxDepth:   0,
	}

	tree := DirectoryTreeInfo{
		Path:       "/media/movies",
		TotalFiles: 50,
		TotalDirs:  2,
		TotalSize:  1024 * 1024 * 300,
		MaxDepth:   1,
		Subdirs:    []*DirectoryTreeInfo{subdir},
	}

	assert.Len(t, tree.Subdirs, 1)
	assert.Equal(t, "/media/movies/action", tree.Subdirs[0].Path)
	assert.Equal(t, 20, tree.Subdirs[0].TotalFiles)
}

// ---------------------------------------------------------------------------
// SupportedProtocols — ordering and completeness
// ---------------------------------------------------------------------------

func TestSupportedProtocols_Order(t *testing.T) {
	factory := NewDefaultClientFactory()
	protocols := factory.SupportedProtocols()

	expected := []string{"smb", "ftp", "nfs", "webdav", "local"}
	assert.Equal(t, expected, protocols,
		"protocols must be in the documented order")
}

func TestSupportedProtocols_AllCreateable(t *testing.T) {
	factory := NewDefaultClientFactory()

	for _, proto := range factory.SupportedProtocols() {
		t.Run(proto, func(t *testing.T) {
			// PA-01: each protocol gets exactly the keys it consumes; a kitchen-sink map is now (rightly) rejected.
			perProtocol := map[string]map[string]interface{}{
				"smb":    {"host": "localhost"},
				"ftp":    {"host": "localhost", "path": "/tmp"},
				"nfs":    {"host": "localhost", "path": "/tmp", "mount_point": "/tmp/test-nfs-mount", "options": "vers=3"},
				"webdav": {"url": "http://localhost/webdav", "path": "/tmp"},
				"local":  {"base_path": "/tmp"},
			}
			config := &StorageConfig{Protocol: proto, Settings: perProtocol[proto]}

			client, err := factory.CreateClient(config)
			require.NoError(t, err,
				"protocol %q must be createable with basic settings", proto)
			require.NotNil(t, client)
			assert.Equal(t, proto, client.GetProtocol())
		})
	}
}
