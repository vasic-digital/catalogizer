package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/services"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Config: LoadEndpointConfig + DefaultConfigPath
// ---------------------------------------------------------------------------

func TestLoadEndpointConfig_FullParsing(t *testing.T) {
	tests := []struct {
		name        string
		json        string
		wantErr     bool
		endpointCnt int
	}{
		{
			name:        "single endpoint with all fields",
			endpointCnt: 1,
			json: `{
				"endpoints": [{
					"id": "nas-1",
					"name": "Primary NAS",
					"host": "192.168.1.100",
					"port": 445,
					"share": "media",
					"username": "admin",
					"password": "secret",
					"domain": "WORKGROUP",
					"readonly": true,
					"directories": [
						{"path": "Movies", "content_type": "movie"},
						{"path": "Music",  "content_type": "music"}
					]
				}]
			}`,
		},
		{
			name:        "multiple endpoints",
			endpointCnt: 2,
			json: `{
				"endpoints": [
					{"id": "a", "name": "A", "host": "h1", "port": 445, "share": "s1", "directories": []},
					{"id": "b", "name": "B", "host": "h2", "port": 445, "share": "s2", "directories": []}
				]
			}`,
		},
		{
			name:        "empty endpoints array",
			endpointCnt: 0,
			json:        `{"endpoints": []}`,
		},
		{
			name:    "invalid json",
			json:    `{not valid}`,
			wantErr: true,
		},
		{
			name:        "empty json object",
			endpointCnt: 0,
			json:        `{}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "endpoints.json")
			require.NoError(t, os.WriteFile(path, []byte(tt.json), 0644))

			cfg, err := LoadEndpointConfig(path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, cfg.Endpoints, tt.endpointCnt)
		})
	}
}

func TestLoadEndpointConfig_AllFieldsParsed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "endpoints.json")
	content := `{
		"endpoints": [{
			"id": "test-id",
			"name": "Test Name",
			"host": "10.0.0.1",
			"port": 2445,
			"share": "shared",
			"username": "user1",
			"password": "pass1",
			"domain": "DOM",
			"readonly": false,
			"directories": [
				{"path": "/video", "content_type": "movie"},
				{"path": "/audio", "content_type": "music"},
				{"path": "/shows", "content_type": "tv_show"},
				{"path": "/soft",  "content_type": "software"},
				{"path": "/cmx",   "content_type": "comic"}
			]
		}]
	}`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))

	cfg, err := LoadEndpointConfig(path)
	require.NoError(t, err)
	require.Len(t, cfg.Endpoints, 1)

	ep := cfg.Endpoints[0]
	assert.Equal(t, "test-id", ep.ID)
	assert.Equal(t, "Test Name", ep.Name)
	assert.Equal(t, "10.0.0.1", ep.Host)
	assert.Equal(t, 2445, ep.Port)
	assert.Equal(t, "shared", ep.Share)
	assert.Equal(t, "user1", ep.Username)
	assert.Equal(t, "pass1", ep.Password)
	assert.Equal(t, "DOM", ep.Domain)
	assert.False(t, ep.ReadOnly)
	assert.Len(t, ep.Directories, 5)
	assert.Equal(t, "movie", ep.Directories[0].ContentType)
}

func TestLoadEndpointConfig_MissingFile_ErrorMessage(t *testing.T) {
	_, err := LoadEndpointConfig("/does/not/exist/endpoints.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read endpoint config")
}

func TestLoadEndpointConfig_InvalidJSON_ErrorMsg(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	require.NoError(t, os.WriteFile(path, []byte(`{{{`), 0644))

	_, err := LoadEndpointConfig(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse endpoint config")
}

func TestDefaultConfigPath_NonEmpty(t *testing.T) {
	path := DefaultConfigPath()
	assert.NotEmpty(t, path)
	assert.True(t, strings.HasSuffix(path, "endpoints.json"),
		"expected path to end with endpoints.json, got %s", path)
}

func TestDefaultConfigPath_EnvOverride(t *testing.T) {
	t.Setenv("CHALLENGE_CONFIG_PATH", "/custom/path/endpoints.json")
	path := DefaultConfigPath()
	assert.Equal(t, "/custom/path/endpoints.json", path)
}

// ---------------------------------------------------------------------------
// Config structs: JSON serialization round-trip
// ---------------------------------------------------------------------------

func TestEndpointConfig_JSONRoundTrip(t *testing.T) {
	original := EndpointConfig{
		Endpoints: []Endpoint{
			{
				ID:       "id1",
				Name:     "name1",
				Host:     "host1",
				Port:     445,
				Share:    "share1",
				Username: "user",
				Password: "pass",
				Domain:   "dom",
				ReadOnly: true,
				Directories: []Directory{
					{Path: "/a", ContentType: "movie"},
					{Path: "/b", ContentType: "music"},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored EndpointConfig
	require.NoError(t, json.Unmarshal(data, &restored))

	assert.Equal(t, original.Endpoints[0].ID, restored.Endpoints[0].ID)
	assert.Equal(t, original.Endpoints[0].Host, restored.Endpoints[0].Host)
	assert.Equal(t, original.Endpoints[0].Port, restored.Endpoints[0].Port)
	assert.Len(t, restored.Endpoints[0].Directories, 2)
}

func TestDirectory_JSONTags(t *testing.T) {
	d := Directory{Path: "music", ContentType: "music"}
	data, err := json.Marshal(d)
	require.NoError(t, err)

	assert.Contains(t, string(data), `"path"`)
	assert.Contains(t, string(data), `"content_type"`)
}

// ---------------------------------------------------------------------------
// BrowsingConfig: LoadBrowsingConfig
// ---------------------------------------------------------------------------

func TestLoadBrowsingConfig_Defaults(t *testing.T) {
	for _, key := range []string{
		"BROWSING_API_URL", "ADMIN_USERNAME", "ADMIN_PASSWORD",
		"BROWSING_WEB_URL", "CATALOG_WEB_DIR",
	} {
		t.Setenv(key, "")
	}

	cfg := LoadBrowsingConfig()
	assert.Equal(t, "http://localhost:28080", cfg.BaseURL)
	assert.Equal(t, "admin", cfg.Username)
	assert.Empty(t, cfg.Password, "no default admin password (CAT-021)")
	assert.Equal(t, "http://localhost:3000", cfg.WebAppURL)
	assert.Equal(t, "../catalog-web", cfg.WebAppDir)
}

func TestLoadBrowsingConfig_EnvOverrides(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "http://api:9000")
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", "s3cret")
	t.Setenv("BROWSING_WEB_URL", "http://web:4000")
	t.Setenv("CATALOG_WEB_DIR", "/opt/web")

	cfg := LoadBrowsingConfig()
	assert.Equal(t, "http://api:9000", cfg.BaseURL)
	assert.Equal(t, "root", cfg.Username)
	assert.Equal(t, "s3cret", cfg.Password)
	assert.Equal(t, "http://web:4000", cfg.WebAppURL)
	assert.Equal(t, "/opt/web", cfg.WebAppDir)
}

func TestLoadBrowsingConfig_PartialEnv(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "")
	t.Setenv("ADMIN_USERNAME", "custom_user")
	t.Setenv("ADMIN_PASSWORD", "")
	t.Setenv("BROWSING_WEB_URL", "")
	t.Setenv("CATALOG_WEB_DIR", "")

	cfg := LoadBrowsingConfig()
	assert.Equal(t, "http://localhost:28080", cfg.BaseURL)
	assert.Equal(t, "custom_user", cfg.Username)
	assert.Empty(t, cfg.Password, "no default admin password (CAT-021)")
}

// ---------------------------------------------------------------------------
// IsInvalidTitle
// ---------------------------------------------------------------------------

func TestIsInvalidTitle_TableDriven(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"", true},
		{"unknown", true},
		{"Unknown", true},
		{"UNKNOWN", true},
		{"  unknown  ", true},
		{"untitled", true},
		{"Untitled", true},
		{"placeholder", true},
		{"PLACEHOLDER", true},
		{"n/a", true},
		{"N/A", true},
		{"tbd", true},
		{"TBD", true},
		{" TBD ", true},
		// Valid titles
		{"The Matrix", false},
		{"Breaking Bad", false},
		{"Unknown Pleasures", false},
		{"Untitled Goose Game", false},
		{"placeholder_extended", false},
		{"My TBD Project", false},
		{"a", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			assert.Equal(t, tt.want, IsInvalidTitle(tt.title),
				"IsInvalidTitle(%q)", tt.title)
		})
	}
}

// ---------------------------------------------------------------------------
// invalidTitlePatterns / requiredMediaTypes / viteErrorIndicators / criticalModules
// ---------------------------------------------------------------------------

func TestInvalidTitlePatterns_NotEmpty(t *testing.T) {
	assert.True(t, len(invalidTitlePatterns) > 0, "invalidTitlePatterns should not be empty")
	for _, p := range invalidTitlePatterns {
		// all patterns except "" should be lowercase
		if p != "" {
			assert.Equal(t, strings.ToLower(p), p,
				"pattern %q should be lowercase", p)
		}
	}
}

func TestRequiredMediaTypes_Complete(t *testing.T) {
	assert.GreaterOrEqual(t, len(requiredMediaTypes), 5)

	expected := []string{"music", "tv_show", "movie", "software", "comic"}
	typeSet := make(map[string]bool)
	for _, mt := range requiredMediaTypes {
		typeSet[mt] = true
	}
	for _, e := range expected {
		assert.True(t, typeSet[e], "requiredMediaTypes should contain %q", e)
	}
}

func TestViteErrorIndicators_NonEmpty(t *testing.T) {
	assert.True(t, len(viteErrorIndicators) >= 5,
		"expected at least 5 vite error indicators, got %d", len(viteErrorIndicators))
	for _, indicator := range viteErrorIndicators {
		assert.NotEmpty(t, indicator, "vite error indicator should not be empty")
	}
}

func TestCriticalModules_ContainsEntryPoint(t *testing.T) {
	assert.True(t, len(criticalModules) >= 3,
		"expected at least 3 critical modules, got %d", len(criticalModules))

	hasMain := false
	hasApp := false
	hasWS := false
	for _, m := range criticalModules {
		assert.True(t, strings.HasPrefix(m, "/src/"),
			"critical module %q should start with /src/", m)
		if m == "/src/main.tsx" {
			hasMain = true
		}
		if m == "/src/App.tsx" {
			hasApp = true
		}
		if m == "/src/lib/websocket.ts" {
			hasWS = true
		}
	}
	assert.True(t, hasMain, "must include /src/main.tsx")
	assert.True(t, hasApp, "must include /src/App.tsx")
	assert.True(t, hasWS, "must include /src/lib/websocket.ts")
}

func TestAllFirstCatalogDeps(t *testing.T) {
	assert.Len(t, allFirstCatalogDeps, 7)
	depSet := make(map[challenge.ID]bool)
	for _, d := range allFirstCatalogDeps {
		depSet[d] = true
	}
	for _, expected := range []string{
		"first-catalog-smb-connect",
		"first-catalog-dir-discovery",
		"first-catalog-music-scan",
		"first-catalog-series-scan",
		"first-catalog-movies-scan",
		"first-catalog-software-scan",
		"first-catalog-comics-scan",
	} {
		assert.True(t, depSet[challenge.ID(expected)],
			"allFirstCatalogDeps should contain %q", expected)
	}
}

// ---------------------------------------------------------------------------
// smb_helper.go: extensionSet
// ---------------------------------------------------------------------------

func TestExtensionSet_Empty(t *testing.T) {
	m := extensionSet(nil)
	assert.NotNil(t, m)
	assert.Len(t, m, 0)
}

func TestExtensionSet_Populated(t *testing.T) {
	m := extensionSet([]string{".mp3", ".flac", ".wav"})
	assert.Len(t, m, 3)
	assert.True(t, m[".mp3"])
	assert.True(t, m[".flac"])
	assert.True(t, m[".wav"])
	assert.False(t, m[".ogg"])
}

func TestExtensionSet_Duplicates(t *testing.T) {
	m := extensionSet([]string{".mp3", ".mp3", ".mp3"})
	assert.Len(t, m, 1)
	assert.True(t, m[".mp3"])
}

// ---------------------------------------------------------------------------
// smb_helper.go: walkResult struct
// ---------------------------------------------------------------------------

func TestWalkResult_ZeroValue(t *testing.T) {
	r := &walkResult{
		ExtensionsFound: make(map[string]int),
	}
	assert.Equal(t, 0, r.FileCount)
	assert.Equal(t, 0, r.DirCount)
	assert.Equal(t, int64(0), r.TotalSize)
	assert.Len(t, r.ExtensionsFound, 0)
}

// ---------------------------------------------------------------------------
// smb_helper.go: audioExtensions
// ---------------------------------------------------------------------------

func TestAudioExtensions_NotEmpty(t *testing.T) {
	assert.True(t, len(audioExtensions) >= 5,
		"expected at least 5 audio extensions, got %d", len(audioExtensions))
	for _, ext := range audioExtensions {
		assert.True(t, strings.HasPrefix(ext, "."),
			"extension %q should start with '.'", ext)
	}
}

// ---------------------------------------------------------------------------
// Challenge constructors: all 35 original challenges
// ---------------------------------------------------------------------------

func TestAllChallengeConstructors_IDAndMetadata(t *testing.T) {
	ep := &Endpoint{
		ID:       "test-nas",
		Host:     "nas.test",
		Port:     445,
		Share:    "share",
		Username: "user",
		Password: "pass",
		Directories: []Directory{
			{Path: "Music", ContentType: "music"},
			{Path: "Series", ContentType: "tv_show"},
			{Path: "Movies", ContentType: "movie"},
			{Path: "Software", ContentType: "software"},
			{Path: "Comics", ContentType: "comic"},
		},
	}

	tests := []struct {
		name         string
		create       func() challenge.Challenge
		expectedID   string
		expectedName string
		category     string
		depCount     int
	}{
		// CH-001 to CH-007: First Catalog
		{
			name:         "CH-001 SMB Connectivity",
			create:       func() challenge.Challenge { return NewSMBConnectivityChallenge(ep) },
			expectedID:   "first-catalog-smb-connect",
			expectedName: "SMB Connectivity",
			category:     "integration",
			depCount:     0,
		},
		{
			name:         "CH-002 Directory Discovery",
			create:       func() challenge.Challenge { return NewDirectoryDiscoveryChallenge(ep) },
			expectedID:   "first-catalog-dir-discovery",
			expectedName: "Directory Discovery",
			category:     "integration",
			depCount:     1,
		},
		{
			name:         "CH-003 Music Scan",
			create:       func() challenge.Challenge { return NewMusicScanChallenge(ep, ep.Directories[0]) },
			expectedID:   "first-catalog-music-scan",
			expectedName: "Music Content Scan",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-004 Series Scan",
			create:       func() challenge.Challenge { return NewSeriesScanChallenge(ep, ep.Directories[1]) },
			expectedID:   "first-catalog-series-scan",
			expectedName: "TV Series Content Scan",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-005 Movies Scan",
			create:       func() challenge.Challenge { return NewMoviesScanChallenge(ep, ep.Directories[2]) },
			expectedID:   "first-catalog-movies-scan",
			expectedName: "Movies Content Scan",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-006 Software Scan",
			create:       func() challenge.Challenge { return NewSoftwareScanChallenge(ep, ep.Directories[3]) },
			expectedID:   "first-catalog-software-scan",
			expectedName: "Software Content Scan",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-007 Comics Scan",
			create:       func() challenge.Challenge { return NewComicsScanChallenge(ep, ep.Directories[4]) },
			expectedID:   "first-catalog-comics-scan",
			expectedName: "Comics Content Scan",
			category:     "e2e",
			depCount:     1,
		},
		// CH-008: Populate
		{
			name:         "CH-008 Populate",
			create:       func() challenge.Challenge { return NewFirstCatalogPopulateChallenge() },
			expectedID:   "first-catalog-populate",
			expectedName: "Populate Catalog Database",
			category:     "e2e",
			depCount:     7,
		},
		// CH-009 to CH-011: Browsing
		{
			name:         "CH-009 API Health",
			create:       func() challenge.Challenge { return NewBrowsingAPIHealthChallenge() },
			expectedID:   "browsing-api-health",
			expectedName: "API Health & Auth",
			category:     "e2e",
			depCount:     0,
		},
		{
			name:         "CH-010 API Catalog",
			create:       func() challenge.Challenge { return NewBrowsingAPICatalogChallenge() },
			expectedID:   "browsing-api-catalog",
			expectedName: "API Catalog Browsing",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-011 Web App",
			create:       func() challenge.Challenge { return NewBrowsingWebAppChallenge() },
			expectedID:   "browsing-web-app",
			expectedName: "Web App Browsing",
			category:     "e2e",
			depCount:     1,
		},
		// CH-012 to CH-013: Asset
		{
			name:         "CH-012 Asset Serving",
			create:       func() challenge.Challenge { return NewAssetServingChallenge() },
			expectedID:   "asset-serving",
			expectedName: "Asset Serving",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-013 Asset Lazy Loading",
			create:       func() challenge.Challenge { return NewAssetLazyLoadingChallenge() },
			expectedID:   "asset-lazy-loading",
			expectedName: "Asset Lazy Loading",
			category:     "e2e",
			depCount:     1,
		},
		// CH-014 to CH-015: Database
		{
			name:         "CH-014 Database Connectivity",
			create:       func() challenge.Challenge { return NewDatabaseConnectivityChallenge() },
			expectedID:   "database-connectivity",
			expectedName: "Database Connectivity",
			category:     "e2e",
			depCount:     0,
		},
		{
			name:         "CH-015 Database Schema",
			create:       func() challenge.Challenge { return NewDatabaseSchemaValidationChallenge() },
			expectedID:   "database-schema-validation",
			expectedName: "Database Schema Validation",
			category:     "e2e",
			depCount:     1,
		},
		// CH-016 to CH-020: Entity
		{
			name:         "CH-016 Entity Aggregation",
			create:       func() challenge.Challenge { return NewEntityAggregationChallenge() },
			expectedID:   "entity-aggregation",
			expectedName: "Entity Aggregation",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-017 Entity Browsing",
			create:       func() challenge.Challenge { return NewEntityBrowsingChallenge() },
			expectedID:   "entity-browsing",
			expectedName: "Entity Browsing",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-018 Entity Metadata",
			create:       func() challenge.Challenge { return NewEntityMetadataChallenge() },
			expectedID:   "entity-metadata",
			expectedName: "Entity Metadata Enrichment",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-019 Entity Duplicates",
			create:       func() challenge.Challenge { return NewEntityDuplicatesChallenge() },
			expectedID:   "entity-duplicates",
			expectedName: "Entity Duplicate Detection",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-020 Entity Hierarchy",
			create:       func() challenge.Challenge { return NewEntityHierarchyChallenge() },
			expectedID:   "entity-hierarchy",
			expectedName: "Entity Hierarchical Navigation",
			category:     "e2e",
			depCount:     1,
		},
		// CH-021 to CH-025: Module integration
		{
			name:         "CH-021 Collections API",
			create:       func() challenge.Challenge { return NewCollectionsAPIChallenge() },
			expectedID:   "collections-api",
			expectedName: "Collections API",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-022 Entity User Metadata",
			create:       func() challenge.Challenge { return NewEntityUserMetadataChallenge() },
			expectedID:   "entity-user-metadata",
			expectedName: "Entity User Metadata",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-023 Entity Search",
			create:       func() challenge.Challenge { return NewEntitySearchChallenge() },
			expectedID:   "entity-search",
			expectedName: "Entity Search",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-024 Storage Roots API",
			create:       func() challenge.Challenge { return NewStorageRootsAPIChallenge() },
			expectedID:   "storage-roots-api",
			expectedName: "Storage Roots API",
			category:     "e2e",
			depCount:     1,
		},
		{
			name:         "CH-025 Auth Token Refresh",
			create:       func() challenge.Challenge { return NewAuthTokenRefreshChallenge() },
			expectedID:   "auth-token-refresh",
			expectedName: "Auth Token Refresh",
			category:     "e2e",
			depCount:     1,
		},
		// CH-026 to CH-035: Extended validation
		{
			name:         "CH-026 Stress Test",
			create:       func() challenge.Challenge { return NewStressTestChallenge() },
			expectedID:   "stress-test",
			expectedName: "API Stress Test",
			category:     "stress",
			depCount:     1,
		},
		{
			name:         "CH-027 Rate Limiting",
			create:       func() challenge.Challenge { return NewRateLimitingChallenge() },
			expectedID:   "rate-limiting",
			expectedName: "Rate Limiting",
			category:     "security",
			depCount:     1,
		},
		{
			name:         "CH-028 Favorites Workflow",
			create:       func() challenge.Challenge { return NewFavoritesWorkflowChallenge() },
			expectedID:   "favorites-workflow",
			expectedName: "Favorites Workflow",
			category:     "workflow",
			depCount:     1,
		},
		{
			name:         "CH-029 Collection Management",
			create:       func() challenge.Challenge { return NewCollectionManagementChallenge() },
			expectedID:   "collection-management",
			expectedName: "Collection Management",
			category:     "workflow",
			depCount:     2,
		},
		{
			name:         "CH-030 Media Playback",
			create:       func() challenge.Challenge { return NewMediaPlaybackChallenge() },
			expectedID:   "media-playback",
			expectedName: "Media Playback",
			category:     "playback",
			depCount:     1,
		},
		{
			name:         "CH-031 Search Filter",
			create:       func() challenge.Challenge { return NewSearchFilterChallenge() },
			expectedID:   "search-filter",
			expectedName: "Search & Filter",
			category:     "search",
			depCount:     1,
		},
		{
			name:         "CH-032 Cover Art",
			create:       func() challenge.Challenge { return NewCoverArtChallenge() },
			expectedID:   "cover-art",
			expectedName: "Cover Art",
			category:     "media",
			depCount:     1,
		},
		{
			name:         "CH-033 WebSocket Events",
			create:       func() challenge.Challenge { return NewWebSocketEventsChallenge() },
			expectedID:   "websocket-events",
			expectedName: "WebSocket Events",
			category:     "realtime",
			depCount:     1,
		},
		{
			name:         "CH-034 Security",
			create:       func() challenge.Challenge { return NewSecurityChallenge() },
			expectedID:   "security",
			expectedName: "Security",
			category:     "security",
			depCount:     1,
		},
		{
			name:         "CH-035 Config Wizard",
			create:       func() challenge.Challenge { return NewConfigWizardChallenge() },
			expectedID:   "config-wizard",
			expectedName: "Configuration Wizard",
			category:     "configuration",
			depCount:     1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()

			require.NotNil(t, ch)
			assert.Equal(t, challenge.ID(tt.expectedID), ch.ID(),
				"unexpected ID")
			assert.Equal(t, tt.expectedName, ch.Name(),
				"unexpected Name")
			assert.Equal(t, tt.category, ch.Category(),
				"unexpected Category")
			assert.Len(t, ch.Dependencies(), tt.depCount,
				"unexpected dependency count")
			assert.NotEmpty(t, ch.Description(),
				"description should not be empty")
		})
	}
}

// ---------------------------------------------------------------------------
// Challenge ID uniqueness across all 35 original challenges
// ---------------------------------------------------------------------------

func TestAllOriginalChallengeIDs_Unique(t *testing.T) {
	ep := &Endpoint{
		Host: "test", Port: 445, Share: "s",
		Directories: []Directory{
			{Path: "a", ContentType: "music"},
			{Path: "b", ContentType: "tv_show"},
			{Path: "c", ContentType: "movie"},
			{Path: "d", ContentType: "software"},
			{Path: "e", ContentType: "comic"},
		},
	}

	challenges := []challenge.Challenge{
		NewSMBConnectivityChallenge(ep),
		NewDirectoryDiscoveryChallenge(ep),
		NewMusicScanChallenge(ep, ep.Directories[0]),
		NewSeriesScanChallenge(ep, ep.Directories[1]),
		NewMoviesScanChallenge(ep, ep.Directories[2]),
		NewSoftwareScanChallenge(ep, ep.Directories[3]),
		NewComicsScanChallenge(ep, ep.Directories[4]),
		NewFirstCatalogPopulateChallenge(),
		NewBrowsingAPIHealthChallenge(),
		NewBrowsingAPICatalogChallenge(),
		NewBrowsingWebAppChallenge(),
		NewAssetServingChallenge(),
		NewAssetLazyLoadingChallenge(),
		NewDatabaseConnectivityChallenge(),
		NewDatabaseSchemaValidationChallenge(),
		NewEntityAggregationChallenge(),
		NewEntityBrowsingChallenge(),
		NewEntityMetadataChallenge(),
		NewEntityDuplicatesChallenge(),
		NewEntityHierarchyChallenge(),
		NewCollectionsAPIChallenge(),
		NewEntityUserMetadataChallenge(),
		NewEntitySearchChallenge(),
		NewStorageRootsAPIChallenge(),
		NewAuthTokenRefreshChallenge(),
		NewStressTestChallenge(),
		NewRateLimitingChallenge(),
		NewFavoritesWorkflowChallenge(),
		NewCollectionManagementChallenge(),
		NewMediaPlaybackChallenge(),
		NewSearchFilterChallenge(),
		NewCoverArtChallenge(),
		NewWebSocketEventsChallenge(),
		NewSecurityChallenge(),
		NewConfigWizardChallenge(),
	}

	seen := make(map[challenge.ID]bool)
	for _, ch := range challenges {
		id := ch.ID()
		assert.False(t, seen[id], "duplicate challenge ID: %s", id)
		seen[id] = true
	}
	assert.Equal(t, 35, len(seen), "expected 35 unique original challenge IDs")
}

// ---------------------------------------------------------------------------
// Challenge dependency chain correctness
// ---------------------------------------------------------------------------

func TestDependencyChain_BrowsingSequence(t *testing.T) {
	health := NewBrowsingAPIHealthChallenge()
	catalog := NewBrowsingAPICatalogChallenge()
	webapp := NewBrowsingWebAppChallenge()

	assert.Empty(t, health.Dependencies())
	assert.Contains(t, catalog.Dependencies(), challenge.ID("browsing-api-health"))
	assert.Contains(t, webapp.Dependencies(), challenge.ID("browsing-api-catalog"))
}

func TestDependencyChain_AssetSequence(t *testing.T) {
	serving := NewAssetServingChallenge()
	lazy := NewAssetLazyLoadingChallenge()

	assert.Contains(t, serving.Dependencies(), challenge.ID("browsing-api-health"))
	assert.Contains(t, lazy.Dependencies(), challenge.ID("asset-serving"))
}

func TestDependencyChain_DatabaseSequence(t *testing.T) {
	conn := NewDatabaseConnectivityChallenge()
	schema := NewDatabaseSchemaValidationChallenge()

	assert.Empty(t, conn.Dependencies(), "database connectivity should have no deps")
	assert.Contains(t, schema.Dependencies(), challenge.ID("database-connectivity"))
}

func TestDependencyChain_EntitySequence(t *testing.T) {
	agg := NewEntityAggregationChallenge()
	browse := NewEntityBrowsingChallenge()
	meta := NewEntityMetadataChallenge()
	dups := NewEntityDuplicatesChallenge()
	hier := NewEntityHierarchyChallenge()

	assert.Contains(t, agg.Dependencies(), challenge.ID("first-catalog-populate"))
	assert.Contains(t, browse.Dependencies(), challenge.ID("entity-aggregation"))
	assert.Contains(t, meta.Dependencies(), challenge.ID("entity-aggregation"))
	assert.Contains(t, dups.Dependencies(), challenge.ID("entity-aggregation"))
	assert.Contains(t, hier.Dependencies(), challenge.ID("entity-aggregation"))
}

func TestDependencyChain_SearchFilter(t *testing.T) {
	search := NewEntitySearchChallenge()
	filter := NewSearchFilterChallenge()

	assert.Contains(t, search.Dependencies(), challenge.ID("entity-aggregation"))
	assert.Contains(t, filter.Dependencies(), challenge.ID("entity-search"))
}

func TestDependencyChain_CollectionManagement(t *testing.T) {
	cm := NewCollectionManagementChallenge()
	deps := cm.Dependencies()
	assert.Len(t, deps, 2)
	assert.Contains(t, deps, challenge.ID("collections-api"))
	assert.Contains(t, deps, challenge.ID("entity-aggregation"))
}

func TestDependencyChain_Populate(t *testing.T) {
	ch := NewFirstCatalogPopulateChallenge()
	deps := ch.Dependencies()
	assert.Len(t, deps, 7, "populate should depend on all 7 first-catalog challenges")

	for _, d := range allFirstCatalogDeps {
		assert.Contains(t, deps, d)
	}
}

// ---------------------------------------------------------------------------
// Challenges with config: verify config is loaded
// ---------------------------------------------------------------------------

func TestChallengesWithConfig_ConfigNotNil(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD", testAdminPassword)
	tests := []struct {
		name   string
		create func() interface{ config() *BrowsingConfig }
	}{
		{"populate", func() interface{ config() *BrowsingConfig } {
			ch := NewFirstCatalogPopulateChallenge()
			return configGetter{ch.config}
		}},
		{"health", func() interface{ config() *BrowsingConfig } {
			ch := NewBrowsingAPIHealthChallenge()
			return configGetter{ch.config}
		}},
		{"catalog", func() interface{ config() *BrowsingConfig } {
			ch := NewBrowsingAPICatalogChallenge()
			return configGetter{ch.config}
		}},
		{"webapp", func() interface{ config() *BrowsingConfig } {
			ch := NewBrowsingWebAppChallenge()
			return configGetter{ch.config}
		}},
		{"asset_serving", func() interface{ config() *BrowsingConfig } {
			ch := NewAssetServingChallenge()
			return configGetter{ch.config}
		}},
		{"asset_lazy", func() interface{ config() *BrowsingConfig } {
			ch := NewAssetLazyLoadingChallenge()
			return configGetter{ch.config}
		}},
		{"db_conn", func() interface{ config() *BrowsingConfig } {
			ch := NewDatabaseConnectivityChallenge()
			return configGetter{ch.config}
		}},
		{"db_schema", func() interface{ config() *BrowsingConfig } {
			ch := NewDatabaseSchemaValidationChallenge()
			return configGetter{ch.config}
		}},
		{"entity_agg", func() interface{ config() *BrowsingConfig } {
			ch := NewEntityAggregationChallenge()
			return configGetter{ch.config}
		}},
		{"security", func() interface{ config() *BrowsingConfig } {
			ch := NewSecurityChallenge()
			return configGetter{ch.config}
		}},
		{"stress", func() interface{ config() *BrowsingConfig } {
			ch := NewStressTestChallenge()
			return configGetter{ch.config}
		}},
		{"rate_limit", func() interface{ config() *BrowsingConfig } {
			ch := NewRateLimitingChallenge()
			return configGetter{ch.config}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()
			assert.NotNil(t, ch.config())
			assert.NotEmpty(t, ch.config().BaseURL)
			assert.NotEmpty(t, ch.config().Username)
			assert.NotEmpty(t, ch.config().Password)
		})
	}
}

type configGetter struct {
	cfg *BrowsingConfig
}

func (g configGetter) config() *BrowsingConfig { return g.cfg }

// ---------------------------------------------------------------------------
// RegisterAll: end-to-end registration test
// ---------------------------------------------------------------------------

func TestRegisterAll_NoConfig_NoError(t *testing.T) {
	svc := services.NewChallengeService(t.TempDir())
	err := RegisterAll(svc)
	require.NoError(t, err)
	// Without config, only non-config challenges are registered
	challenges := svc.ListChallenges()
	assert.NotNil(t, challenges)
}

func TestRegisterAll_WithConfig_RegistersAll(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "challenges", "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	content := `{
		"endpoints": [{
			"id": "test",
			"name": "Test NAS",
			"host": "test.local",
			"port": 445,
			"share": "media",
			"username": "u",
			"password": "p",
			"readonly": true,
			"directories": [
				{"path": "Music",    "content_type": "music"},
				{"path": "Series",   "content_type": "tv_show"},
				{"path": "Movies",   "content_type": "movie"},
				{"path": "Software", "content_type": "software"},
				{"path": "Comics",   "content_type": "comic"}
			]
		}]
	}`

	configPath := filepath.Join(configDir, "endpoints.json")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))

	t.Setenv("CHALLENGE_CONFIG_PATH", configPath)

	svc := services.NewChallengeService(filepath.Join(dir, "results"))
	err := RegisterAll(svc)
	require.NoError(t, err)

	challenges := svc.ListChallenges()
	// 7 first-catalog + 1 populate + 3 browsing + 2 asset + 2 database
	// + 5 entity + 5 module integration + 10 extended + 174 userflow = 209
	assert.GreaterOrEqual(t, len(challenges), 35,
		"expected at least 35 original challenges registered")

	// Verify key challenge IDs present
	ids := make(map[string]bool)
	for _, c := range challenges {
		ids[c.ID] = true
	}
	for _, expected := range []string{
		"first-catalog-smb-connect",
		"first-catalog-populate",
		"browsing-api-health",
		"browsing-api-catalog",
		"browsing-web-app",
		"asset-serving",
		"asset-lazy-loading",
		"database-connectivity",
		"database-schema-validation",
		"entity-aggregation",
		"entity-browsing",
		"entity-metadata",
		"entity-duplicates",
		"entity-hierarchy",
		"collections-api",
		"entity-user-metadata",
		"entity-search",
		"storage-roots-api",
		"auth-token-refresh",
		"stress-test",
		"rate-limiting",
		"favorites-workflow",
		"collection-management",
		"media-playback",
		"search-filter",
		"cover-art",
		"websocket-events",
		"security",
		"config-wizard",
	} {
		assert.True(t, ids[expected], "challenge %q should be registered", expected)
	}
}

// ---------------------------------------------------------------------------
// Userflow registration functions
// ---------------------------------------------------------------------------

func TestRegisterUserFlowAPIChallenges_Count(t *testing.T) {
	challenges := registerUserFlowAPIChallenges()
	assert.Equal(t, 49, len(challenges),
		"expected 49 API userflow challenges, got %d", len(challenges))

	// Verify all have unique IDs
	seen := make(map[challenge.ID]bool)
	for _, ch := range challenges {
		assert.False(t, seen[ch.ID()], "duplicate UF API ID: %s", ch.ID())
		seen[ch.ID()] = true
		assert.NotEmpty(t, ch.Name())
		assert.NotEmpty(t, ch.ID())
	}
}

func TestRegisterUserFlowWebChallenges_Count(t *testing.T) {
	challenges := registerUserFlowWebChallenges()
	assert.Equal(t, 59, len(challenges),
		"expected 59 web userflow challenges, got %d", len(challenges))

	seen := make(map[challenge.ID]bool)
	for _, ch := range challenges {
		assert.False(t, seen[ch.ID()], "duplicate UF web ID: %s", ch.ID())
		seen[ch.ID()] = true
	}
}

func TestRegisterUserFlowDesktopChallenges_Count(t *testing.T) {
	challenges := registerUserFlowDesktopChallenges()
	assert.Equal(t, 28, len(challenges),
		"expected 28 desktop userflow challenges, got %d", len(challenges))

	seen := make(map[challenge.ID]bool)
	for _, ch := range challenges {
		assert.False(t, seen[ch.ID()], "duplicate UF desktop ID: %s", ch.ID())
		seen[ch.ID()] = true
	}
}

func TestRegisterUserFlowMobileChallenges_Count(t *testing.T) {
	challenges := registerUserFlowMobileChallenges()
	assert.Equal(t, 38, len(challenges),
		"expected 38 mobile userflow challenges, got %d", len(challenges))

	seen := make(map[challenge.ID]bool)
	for _, ch := range challenges {
		assert.False(t, seen[ch.ID()], "duplicate UF mobile ID: %s", ch.ID())
		seen[ch.ID()] = true
	}
}

func TestAllUserFlowChallenges_TotalCount(t *testing.T) {
	api := registerUserFlowAPIChallenges()
	web := registerUserFlowWebChallenges()
	desktop := registerUserFlowDesktopChallenges()
	mobile := registerUserFlowMobileChallenges()

	total := len(api) + len(web) + len(desktop) + len(mobile)
	assert.Equal(t, 174, total,
		"expected 174 total userflow challenges, got %d", total)
}

func TestAllUserFlowChallenges_GlobalIDUniqueness(t *testing.T) {
	var all []challenge.Challenge
	all = append(all, registerUserFlowAPIChallenges()...)
	all = append(all, registerUserFlowWebChallenges()...)
	all = append(all, registerUserFlowDesktopChallenges()...)
	all = append(all, registerUserFlowMobileChallenges()...)

	seen := make(map[challenge.ID]bool)
	for _, ch := range all {
		id := ch.ID()
		assert.False(t, seen[id], "duplicate userflow ID across platforms: %s", id)
		seen[id] = true
	}
	assert.Equal(t, 174, len(seen))
}

func TestRegisterUserFlowAPIChallenges_ViaService(t *testing.T) {
	svc := services.NewChallengeService(t.TempDir())
	RegisterUserFlowAPIChallenges(svc)
	challenges := svc.ListChallenges()
	assert.Equal(t, 49, len(challenges))
}

func TestRegisterUserFlowWebChallenges_ViaService(t *testing.T) {
	svc := services.NewChallengeService(t.TempDir())
	RegisterUserFlowWebChallenges(svc)
	challenges := svc.ListChallenges()
	assert.Equal(t, 59, len(challenges))
}

func TestRegisterUserFlowDesktopChallenges_ViaService(t *testing.T) {
	svc := services.NewChallengeService(t.TempDir())
	RegisterUserFlowDesktopChallenges(svc)
	challenges := svc.ListChallenges()
	assert.Equal(t, 28, len(challenges))
}

func TestRegisterUserFlowMobileChallenges_ViaService(t *testing.T) {
	svc := services.NewChallengeService(t.TempDir())
	RegisterUserFlowMobileChallenges(svc)
	challenges := svc.ListChallenges()
	assert.Equal(t, 38, len(challenges))
}

// ---------------------------------------------------------------------------
// Userflow helper functions
// ---------------------------------------------------------------------------

func TestUserFlowAPIAdapter_Defaults(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "")
	adapter := userFlowAPIAdapter()
	assert.NotNil(t, adapter)
}

func TestUserFlowAPIAdapter_EnvOverride(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "http://custom:9090")
	adapter := userFlowAPIAdapter()
	assert.NotNil(t, adapter)
}

func TestUserFlowCredentials_Defaults(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "")
	t.Setenv("ADMIN_PASSWORD", "")
	creds := userFlowCredentials()
	assert.Equal(t, "admin", creds.Username)
	assert.Empty(t, creds.Password, "no default admin password (CAT-021)")
}

func TestUserFlowCredentials_EnvOverride(t *testing.T) {
	t.Setenv("ADMIN_USERNAME", "root")
	t.Setenv("ADMIN_PASSWORD", "secure123")
	creds := userFlowCredentials()
	assert.Equal(t, "root", creds.Username)
	assert.Equal(t, "secure123", creds.Password)
}

func TestDefaultBrowserConfig(t *testing.T) {
	cfg := defaultBrowserConfig()
	assert.Equal(t, "chromium", cfg.BrowserType)
	assert.True(t, cfg.Headless)
	assert.Equal(t, [2]int{1920, 1080}, cfg.WindowSize)
}

func TestWebAppURL_Constant(t *testing.T) {
	assert.Equal(t, "http://localhost:3000", webAppURL)
}

func TestHealthDep(t *testing.T) {
	assert.Len(t, healthDep, 1)
	assert.Equal(t, challenge.ID("UF-API-HEALTH"), healthDep[0])
}

func TestAuthDep(t *testing.T) {
	assert.Len(t, authDep, 2)
	assert.Equal(t, challenge.ID("UF-API-HEALTH"), authDep[0])
	assert.Equal(t, challenge.ID("UF-WEB-AUTH-LOGIN"), authDep[1])
}

func TestDesktopProjectRoot_Default(t *testing.T) {
	t.Setenv("DESKTOP_PROJECT_ROOT", "")
	root := desktopProjectRoot()
	assert.Equal(t, "../catalogizer-desktop", root)
}

func TestDesktopProjectRoot_EnvOverride(t *testing.T) {
	t.Setenv("DESKTOP_PROJECT_ROOT", "/custom/path")
	root := desktopProjectRoot()
	assert.Equal(t, "/custom/path", root)
}

func TestDesktopBinaryPath_Default(t *testing.T) {
	t.Setenv("DESKTOP_BINARY_PATH", "")
	p := desktopBinaryPath()
	assert.Contains(t, p, "catalogizer-desktop")
}

func TestAndroidProjectRoot_Default(t *testing.T) {
	t.Setenv("ANDROID_PROJECT_ROOT", "")
	root := androidProjectRoot()
	assert.Equal(t, "../catalogizer-android", root)
}

func TestAndroidAPKPath_Default(t *testing.T) {
	t.Setenv("ANDROID_APK_PATH", "")
	p := androidAPKPath()
	assert.Contains(t, p, "app-debug.apk")
}

func TestAndroidTVProjectRoot_Default(t *testing.T) {
	t.Setenv("ANDROID_TV_PROJECT_ROOT", "")
	root := androidTVProjectRoot()
	assert.Equal(t, "../catalogizer-androidtv", root)
}

// ---------------------------------------------------------------------------
// browsing_helper.go: isEndpointReachable / isWebAppReachable
// (only testable with unreachable addresses to avoid network deps)
// ---------------------------------------------------------------------------

func TestIsEndpointReachable_Unreachable(t *testing.T) {
	// Use a non-routable address to test the failure path quickly
	result := isEndpointReachable("192.0.2.1", 445)
	assert.False(t, result)
}

func TestIsWebAppReachable_Unreachable(t *testing.T) {
	result := isWebAppReachable("http://192.0.2.1:1")
	assert.False(t, result)
}

// ---------------------------------------------------------------------------
// env.GetOrDefault (used widely across the package)
// ---------------------------------------------------------------------------

func TestEnvGetOrDefault_Set(t *testing.T) {
	t.Setenv("TEST_COVERAGE_VAR", "myval")
	assert.Equal(t, "myval", env.GetOrDefault("TEST_COVERAGE_VAR", "default"))
}

func TestEnvGetOrDefault_Empty(t *testing.T) {
	t.Setenv("TEST_COVERAGE_VAR", "")
	assert.Equal(t, "default", env.GetOrDefault("TEST_COVERAGE_VAR", "default"))
}

func TestEnvGetOrDefault_Unset(t *testing.T) {
	assert.Equal(t, "fallback", env.GetOrDefault("DEFINITELY_NOT_SET_XYZ123", "fallback"))
}

// ---------------------------------------------------------------------------
// Endpoint struct field access
// ---------------------------------------------------------------------------

func TestEndpoint_AllFields(t *testing.T) {
	ep := Endpoint{
		ID:       "id",
		Name:     "name",
		Host:     "host",
		Port:     445,
		Share:    "share",
		Username: "user",
		Password: "pass",
		Domain:   "dom",
		ReadOnly: true,
		Directories: []Directory{
			{Path: "/a", ContentType: "movie"},
		},
	}

	assert.Equal(t, "id", ep.ID)
	assert.Equal(t, "name", ep.Name)
	assert.Equal(t, "host", ep.Host)
	assert.Equal(t, 445, ep.Port)
	assert.Equal(t, "share", ep.Share)
	assert.Equal(t, "user", ep.Username)
	assert.Equal(t, "pass", ep.Password)
	assert.Equal(t, "dom", ep.Domain)
	assert.True(t, ep.ReadOnly)
	assert.Len(t, ep.Directories, 1)
}

func TestEndpoint_EmptyDirectories(t *testing.T) {
	ep := Endpoint{Host: "h", Port: 445}
	assert.Empty(t, ep.Directories)
}

func TestDirectory_Fields(t *testing.T) {
	d := Directory{Path: "/media/movies", ContentType: "movie"}
	assert.Equal(t, "/media/movies", d.Path)
	assert.Equal(t, "movie", d.ContentType)
}

// ---------------------------------------------------------------------------
// BrowsingConfig struct field access
// ---------------------------------------------------------------------------

func TestBrowsingConfig_Fields(t *testing.T) {
	cfg := &BrowsingConfig{
		BaseURL:   "http://api:28080",
		Username:  "admin",
		Password:  "pass",
		WebAppURL: "http://web:3000",
		WebAppDir: "/app",
	}

	assert.Equal(t, "http://api:28080", cfg.BaseURL)
	assert.Equal(t, "admin", cfg.Username)
	assert.Equal(t, "pass", cfg.Password)
	assert.Equal(t, "http://web:3000", cfg.WebAppURL)
	assert.Equal(t, "/app", cfg.WebAppDir)
}

// ---------------------------------------------------------------------------
// Scan challenge constructors with different directory configurations
// ---------------------------------------------------------------------------

func TestScanChallenges_DifferentDirectories(t *testing.T) {
	ep := &Endpoint{Host: "h", Port: 445, Share: "s"}

	tests := []struct {
		name       string
		create     func() challenge.Challenge
		expectedID string
	}{
		{
			name: "music with cyrillic path",
			create: func() challenge.Challenge {
				return NewMusicScanChallenge(ep, Directory{Path: "Музыка", ContentType: "music"})
			},
			expectedID: "first-catalog-music-scan",
		},
		{
			name: "movies with nested path",
			create: func() challenge.Challenge {
				return NewMoviesScanChallenge(ep, Directory{Path: "media/video/movies", ContentType: "movie"})
			},
			expectedID: "first-catalog-movies-scan",
		},
		{
			name: "series with empty path",
			create: func() challenge.Challenge {
				return NewSeriesScanChallenge(ep, Directory{Path: "", ContentType: "tv_show"})
			},
			expectedID: "first-catalog-series-scan",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()
			assert.Equal(t, challenge.ID(tt.expectedID), ch.ID())
			assert.NotEmpty(t, ch.Description())
		})
	}
}

// ---------------------------------------------------------------------------
// Content type switch coverage (used in RegisterAll)
// ---------------------------------------------------------------------------

func TestRegisterAll_ContentTypeSwitchCoverage(t *testing.T) {
	// Create a config file with all 5 content types + an unknown one
	dir := t.TempDir()
	configDir := filepath.Join(dir, "challenges", "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	content := `{
		"endpoints": [{
			"id": "test",
			"host": "test.local",
			"port": 445,
			"share": "s",
			"directories": [
				{"path": "a", "content_type": "music"},
				{"path": "b", "content_type": "tv_show"},
				{"path": "c", "content_type": "movie"},
				{"path": "d", "content_type": "software"},
				{"path": "e", "content_type": "comic"},
				{"path": "f", "content_type": "unknown_type"}
			]
		}]
	}`

	configPath := filepath.Join(configDir, "endpoints.json")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	t.Setenv("CHALLENGE_CONFIG_PATH", configPath)

	svc := services.NewChallengeService(filepath.Join(dir, "results"))
	err := RegisterAll(svc)
	require.NoError(t, err)

	// Should register: 1 smb + 1 dir_discovery + 5 scans (unknown type skipped)
	// + 28 post-config challenges + 174 userflow = 209
	challenges := svc.ListChallenges()
	assert.GreaterOrEqual(t, len(challenges), 35)

	// Verify the unknown type was skipped (no challenge with "unknown" in ID)
	ids := make(map[string]bool)
	for _, c := range challenges {
		ids[c.ID] = true
	}
	assert.False(t, ids["first-catalog-unknown_type-scan"],
		"unknown content type should not create a challenge")
}

// ---------------------------------------------------------------------------
// RegisterAll with multiple endpoints
// ---------------------------------------------------------------------------

func TestRegisterAll_MultipleEndpoints(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "challenges", "config")
	require.NoError(t, os.MkdirAll(configDir, 0755))

	content := `{
		"endpoints": [
			{
				"id": "nas1",
				"host": "nas1.local",
				"port": 445,
				"share": "s1",
				"directories": [
					{"path": "Movies", "content_type": "movie"}
				]
			},
			{
				"id": "nas2",
				"host": "nas2.local",
				"port": 445,
				"share": "s2",
				"directories": [
					{"path": "Music", "content_type": "music"}
				]
			}
		]
	}`

	configPath := filepath.Join(configDir, "endpoints.json")
	require.NoError(t, os.WriteFile(configPath, []byte(content), 0644))
	t.Setenv("CHALLENGE_CONFIG_PATH", configPath)

	svc := services.NewChallengeService(filepath.Join(dir, "results"))
	err := RegisterAll(svc)
	require.NoError(t, err)

	challenges := svc.ListChallenges()
	// 2 endpoints * (1 smb + 1 dir + 1 scan) = 6, but some IDs collide since
	// they are not endpoint-specific. The total should be at least 35.
	assert.GreaterOrEqual(t, len(challenges), 35)
}

// ---------------------------------------------------------------------------
// WaitTimeout constant
// ---------------------------------------------------------------------------

func TestWaitTimeout_Value(t *testing.T) {
	assert.Greater(t, waitTimeout.Seconds(), float64(0))
}

// ---------------------------------------------------------------------------
// Wizard project root helper
// ---------------------------------------------------------------------------

func TestWizardProjectRoot_Default(t *testing.T) {
	t.Setenv("WIZARD_PROJECT_ROOT", "")
	root := wizardProjectRoot()
	assert.Contains(t, root, "installer-wizard")
}

func TestWizardBinaryPath_Default(t *testing.T) {
	t.Setenv("WIZARD_BINARY_PATH", "")
	p := wizardBinaryPath()
	assert.Contains(t, p, "installer-wizard")
}

func TestAndroidTVAPKPath_Default(t *testing.T) {
	t.Setenv("ANDROIDTV_APK_PATH", "")
	p := androidTVAPKPath()
	assert.Contains(t, p, "app-debug.apk")
}

func TestAndroidMobileConfig(t *testing.T) {
	t.Setenv("ANDROID_DEVICE_SERIAL", "")
	cfg := androidMobileConfig()
	assert.Equal(t, "com.vasic.catalogizer", cfg.PackageName)
	assert.Equal(t, ".MainActivity", cfg.ActivityName)
}

func TestAndroidTVMobileConfig(t *testing.T) {
	t.Setenv("ANDROIDTV_DEVICE_SERIAL", "")
	cfg := androidTVMobileConfig()
	assert.Equal(t, "com.catalogizer.androidtv", cfg.PackageName)
	assert.Equal(t, ".ui.MainActivity", cfg.ActivityName)
}

func TestDesktopCargoAdapter(t *testing.T) {
	t.Setenv("DESKTOP_PROJECT_ROOT", "")
	adapter := desktopCargoAdapter()
	assert.NotNil(t, adapter)
}

func TestWizardCargoAdapter(t *testing.T) {
	t.Setenv("WIZARD_PROJECT_ROOT", "")
	adapter := wizardCargoAdapter()
	assert.NotNil(t, adapter)
}

func TestDesktopTauriAdapter(t *testing.T) {
	t.Setenv("DESKTOP_BINARY_PATH", "")
	adapter := desktopTauriAdapter()
	assert.NotNil(t, adapter)
}

func TestWizardTauriAdapter(t *testing.T) {
	t.Setenv("WIZARD_BINARY_PATH", "")
	adapter := wizardTauriAdapter()
	assert.NotNil(t, adapter)
}

// ---------------------------------------------------------------------------
// Execute method early-return paths (no server needed)
// These tests exercise the Execute code paths that trigger when
// infrastructure (NAS/API/Web) is not reachable. The challenges
// return early with StatusPassed (skipped) or StatusFailed.
// ---------------------------------------------------------------------------

func TestExecute_SMBChallenges_UnreachableEndpoint(t *testing.T) {
	// Use a non-routable IP so isEndpointReachable returns false quickly.
	ep := &Endpoint{
		ID:       "test",
		Host:     "192.0.2.1", // TEST-NET, guaranteed unreachable
		Port:     445,
		Share:    "test",
		Username: "u",
		Password: "p",
		Directories: []Directory{
			{Path: "Music", ContentType: "music"},
			{Path: "Series", ContentType: "tv_show"},
			{Path: "Movies", ContentType: "movie"},
			{Path: "Software", ContentType: "software"},
			{Path: "Comics", ContentType: "comic"},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name   string
		create func() challenge.Challenge
	}{
		{"SMB Connectivity", func() challenge.Challenge { return NewSMBConnectivityChallenge(ep) }},
		{"Directory Discovery", func() challenge.Challenge { return NewDirectoryDiscoveryChallenge(ep) }},
		{"Music Scan", func() challenge.Challenge { return NewMusicScanChallenge(ep, ep.Directories[0]) }},
		{"Series Scan", func() challenge.Challenge { return NewSeriesScanChallenge(ep, ep.Directories[1]) }},
		{"Movies Scan", func() challenge.Challenge { return NewMoviesScanChallenge(ep, ep.Directories[2]) }},
		{"Software Scan", func() challenge.Challenge { return NewSoftwareScanChallenge(ep, ep.Directories[3]) }},
		{"Comics Scan", func() challenge.Challenge { return NewComicsScanChallenge(ep, ep.Directories[4]) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()

			// Challenge interface has Execute, but we need the concrete type
			type executor interface {
				Execute(context.Context) (*challenge.Result, error)
			}
			exec, ok := ch.(executor)
			require.True(t, ok, "challenge should implement Execute")

			result, err := exec.Execute(ctx)
			require.NoError(t, err, "Execute should not return error")
			require.NotNil(t, result, "result should not be nil")

			// When NAS is unreachable, challenges return StatusSkipped
			assert.Equal(t, challenge.StatusSkipped, result.Status,
				"unreachable NAS should result in skipped status")
			assert.True(t, len(result.Assertions) > 0,
				"should have at least one assertion")
		})
	}
}

func TestExecute_APIChallenges_NoServer(t *testing.T) {
	// Point challenges at an unreachable API to test the login-failure path.
	// Use a port that nothing is listening on.
	t.Setenv("BROWSING_API_URL", "http://127.0.0.1:19999")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", testAdminPassword)
	t.Setenv("BROWSING_WEB_URL", "http://127.0.0.1:19998")

	// Use a short context timeout so LoginWithRetry fails fast
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	tests := []struct {
		name   string
		create func() challenge.Challenge
	}{
		{"Populate", func() challenge.Challenge { return NewFirstCatalogPopulateChallenge() }},
		{"API Health", func() challenge.Challenge { return NewBrowsingAPIHealthChallenge() }},
		{"API Catalog", func() challenge.Challenge { return NewBrowsingAPICatalogChallenge() }},
		{"Database Connectivity", func() challenge.Challenge { return NewDatabaseConnectivityChallenge() }},
		{"Database Schema", func() challenge.Challenge { return NewDatabaseSchemaValidationChallenge() }},
		{"Entity Aggregation", func() challenge.Challenge { return NewEntityAggregationChallenge() }},
		{"Entity Browsing", func() challenge.Challenge { return NewEntityBrowsingChallenge() }},
		{"Entity Metadata", func() challenge.Challenge { return NewEntityMetadataChallenge() }},
		{"Entity Duplicates", func() challenge.Challenge { return NewEntityDuplicatesChallenge() }},
		{"Entity Hierarchy", func() challenge.Challenge { return NewEntityHierarchyChallenge() }},
		{"Collections API", func() challenge.Challenge { return NewCollectionsAPIChallenge() }},
		{"Entity User Metadata", func() challenge.Challenge { return NewEntityUserMetadataChallenge() }},
		{"Entity Search", func() challenge.Challenge { return NewEntitySearchChallenge() }},
		{"Storage Roots API", func() challenge.Challenge { return NewStorageRootsAPIChallenge() }},
		{"Auth Token Refresh", func() challenge.Challenge { return NewAuthTokenRefreshChallenge() }},
		{"Stress Test", func() challenge.Challenge { return NewStressTestChallenge() }},
		{"Rate Limiting", func() challenge.Challenge { return NewRateLimitingChallenge() }},
		{"Favorites Workflow", func() challenge.Challenge { return NewFavoritesWorkflowChallenge() }},
		{"Collection Management", func() challenge.Challenge { return NewCollectionManagementChallenge() }},
		{"Media Playback", func() challenge.Challenge { return NewMediaPlaybackChallenge() }},
		{"Search Filter", func() challenge.Challenge { return NewSearchFilterChallenge() }},
		{"Cover Art", func() challenge.Challenge { return NewCoverArtChallenge() }},
		{"Security", func() challenge.Challenge { return NewSecurityChallenge() }},
		{"Config Wizard", func() challenge.Challenge { return NewConfigWizardChallenge() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()

			type executor interface {
				Execute(context.Context) (*challenge.Result, error)
			}
			exec, ok := ch.(executor)
			require.True(t, ok, "challenge should implement Execute")

			result, err := exec.Execute(ctx)
			require.NoError(t, err, "Execute should not return an error (failures are in Result)")
			require.NotNil(t, result)

			// Should fail because no server is running
			assert.Equal(t, challenge.StatusFailed, result.Status,
				"should fail when API is unreachable")
			assert.True(t, len(result.Assertions) > 0,
				"should have at least one assertion about the failure")
		})
	}
}

func TestExecute_WebAppChallenge_NoServer(t *testing.T) {
	t.Setenv("BROWSING_WEB_URL", "http://127.0.0.1:19998")
	t.Setenv("BROWSING_API_URL", "http://127.0.0.1:19999")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := NewBrowsingWebAppChallenge()

	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Web app unreachable -> StatusSkipped
	assert.Equal(t, challenge.StatusSkipped, result.Status,
		"unreachable web app should result in skipped")
}

func TestExecute_WebSocketEvents_NoServer(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "http://127.0.0.1:19999")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch := NewWebSocketEventsChallenge()

	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Should fail when API is not reachable
	assert.Equal(t, challenge.StatusFailed, result.Status)
	assert.True(t, len(result.Assertions) > 0)
}

// ---------------------------------------------------------------------------
// Mock server tests: Execute methods with a fake API that returns
// enough data for challenges to exercise their full validation logic.
// ---------------------------------------------------------------------------

func startMockAPIServer(t *testing.T) string {
	t.Helper()

	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "healthy",
		})
	})

	// Login endpoint
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["username"] == "nonexistent_user" || body["username"] == "invalid_user" {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "invalid credentials"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_token": "mock-jwt-token-for-testing",
			"token":         "mock-jwt-token-for-testing",
			"refresh_token": "mock-refresh-token",
			"user": map[string]interface{}{
				"id":       1,
				"username": "admin",
				"role":     "admin",
			},
		})
	})

	// Auth me endpoint
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || strings.Contains(auth, "invalid") {
			w.WriteHeader(401)
			json.NewEncoder(w).Encode(map[string]interface{}{"error": "unauthorized"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"username": "admin",
			"user": map[string]interface{}{
				"id":       1,
				"username": "admin",
				"role":     "admin",
			},
		})
	})

	// Auth status
	mux.HandleFunc("/api/v1/auth/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"authenticated": true,
			"user": map[string]interface{}{
				"id": 1, "username": "admin",
			},
		})
	})

	// Auth refresh
	mux.HandleFunc("/api/v1/auth/refresh", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_token": "new-mock-jwt-token",
			"token":         "new-mock-jwt-token",
		})
	})

	// Stats overall
	mux.HandleFunc("/api/v1/stats/overall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_files": 1000, "total_size": 5000000, "total_items": 500,
		})
	})

	// Storage roots
	mux.HandleFunc("/api/v1/storage/roots", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "name": "db-test-mock"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{"id": 1, "name": "db-test-mock", "protocol": "local", "path": "/tmp"},
			},
		})
	})

	mux.HandleFunc("/api/v1/storage-roots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"roots": []interface{}{
				map[string]interface{}{"id": 1, "name": "test-root", "status": "connected"},
			},
		})
	})

	// Entity stats
	mux.HandleFunc("/api/v1/entities/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"total_entities": float64(100)})
	})

	// Entity types
	mux.HandleFunc("/api/v1/entities/types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"types": []interface{}{
				map[string]interface{}{"type": "movie", "count": 50},
				map[string]interface{}{"type": "music", "count": 30},
			},
		})
	})

	// Entity list (handles query params too)
	mux.HandleFunc("/api/v1/entities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": float64(1), "title": "Test Movie", "media_type": "movie", "file_count": 1},
				map[string]interface{}{"id": float64(2), "title": "Test Song", "media_type": "music", "file_count": 1},
			},
			"total": 2, "limit": 20, "offset": 0,
		})
	})

	// Entity detail
	mux.HandleFunc("/api/v1/entities/1", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": float64(1), "title": "Test Movie", "media_type": "movie",
			"file_count": 1, "metadata": []interface{}{},
		})
	})

	// Entity browse by type
	mux.HandleFunc("/api/v1/entities/browse/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"id": float64(1), "title": "Test", "media_type": "movie"},
			},
			"total": 1,
		})
	})

	// Entity duplicates
	mux.HandleFunc("/api/v1/entities/duplicates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"groups": []interface{}{}})
	})

	// Entity children
	mux.HandleFunc("/api/v1/entities/1/children", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}})
	})

	// Entity metadata
	mux.HandleFunc("/api/v1/entities/1/metadata", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(202)
			json.NewEncoder(w).Encode(map[string]interface{}{"status": "refreshing"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"metadata": []interface{}{map[string]interface{}{"key": "genre", "value": "action"}},
		})
	})

	// Entity user metadata
	mux.HandleFunc("/api/v1/entities/1/user-metadata", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"is_favorite": true, "rating": 5})
	})

	// Entity duplicates by ID
	mux.HandleFunc("/api/v1/entities/1/duplicates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"duplicates": []interface{}{}})
	})

	// Collections
	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": float64(1), "name": "Test Collection"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"collections": []interface{}{
				map[string]interface{}{"id": float64(1), "name": "Test Collection"},
			},
		})
	})

	// Collection by ID
	mux.HandleFunc("/api/v1/collections/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(204)
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": float64(1), "name": "Updated Collection"})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": float64(1), "name": "Test Collection"})
	})

	// Challenges endpoint
	mux.HandleFunc("/api/v1/challenges", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"challenges": []interface{}{}})
	})

	// Media stats
	mux.HandleFunc("/api/v1/media/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"total_items": 500, "total_files": 1000, "total_size": 5000000,
		})
	})

	// Assets
	mux.HandleFunc("/api/v1/assets/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "url": "/assets/test.jpg", "status": "ready"})
	})

	// Favorites
	mux.HandleFunc("/api/v1/favorites", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "entity_id": 1})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"favorites": []interface{}{map[string]interface{}{"id": 1, "entity_id": 1}},
		})
	})

	// Favorites by ID
	mux.HandleFunc("/api/v1/favorites/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1, "entity_id": 1, "is_favorite": true})
	})

	// Wizard
	mux.HandleFunc("/api/v1/wizard/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "complete", "steps": []interface{}{}})
	})

	// Configuration
	mux.HandleFunc("/api/v1/configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"config": map[string]interface{}{}})
	})

	// Database schema
	mux.HandleFunc("/api/v1/database/schema", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"tables": []interface{}{
				map[string]interface{}{"name": "files", "row_count": 1000},
			},
			"migration_version": 9,
		})
	})

	// Catch-all
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"status": "ok"})
	})

	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start mock server: %v", err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { server.Close() })

	return "http://" + listener.Addr().String()
}

func TestExecute_APIChallenges_WithMockServer(t *testing.T) {
	baseURL := startMockAPIServer(t)

	t.Setenv("BROWSING_API_URL", baseURL)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", testAdminPassword)
	t.Setenv("BROWSING_WEB_URL", "http://127.0.0.1:19998")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tests := []struct {
		name   string
		create func() challenge.Challenge
	}{
		{"API Health", func() challenge.Challenge { return NewBrowsingAPIHealthChallenge() }},
		{"Database Connectivity", func() challenge.Challenge { return NewDatabaseConnectivityChallenge() }},
		{"Database Schema", func() challenge.Challenge { return NewDatabaseSchemaValidationChallenge() }},
		{"Entity Aggregation", func() challenge.Challenge { return NewEntityAggregationChallenge() }},
		{"Entity Browsing", func() challenge.Challenge { return NewEntityBrowsingChallenge() }},
		{"Entity Metadata", func() challenge.Challenge { return NewEntityMetadataChallenge() }},
		{"Entity Duplicates", func() challenge.Challenge { return NewEntityDuplicatesChallenge() }},
		{"Entity Hierarchy", func() challenge.Challenge { return NewEntityHierarchyChallenge() }},
		{"Entity Search", func() challenge.Challenge { return NewEntitySearchChallenge() }},
		{"Entity User Metadata", func() challenge.Challenge { return NewEntityUserMetadataChallenge() }},
		{"Collections API", func() challenge.Challenge { return NewCollectionsAPIChallenge() }},
		{"Storage Roots API", func() challenge.Challenge { return NewStorageRootsAPIChallenge() }},
		{"Auth Token Refresh", func() challenge.Challenge { return NewAuthTokenRefreshChallenge() }},
		{"Security", func() challenge.Challenge { return NewSecurityChallenge() }},
		{"Stress Test", func() challenge.Challenge { return NewStressTestChallenge() }},
		{"Favorites Workflow", func() challenge.Challenge { return NewFavoritesWorkflowChallenge() }},
		{"Collection Management", func() challenge.Challenge { return NewCollectionManagementChallenge() }},
		{"Media Playback", func() challenge.Challenge { return NewMediaPlaybackChallenge() }},
		{"Search Filter", func() challenge.Challenge { return NewSearchFilterChallenge() }},
		{"Cover Art", func() challenge.Challenge { return NewCoverArtChallenge() }},
		{"Config Wizard", func() challenge.Challenge { return NewConfigWizardChallenge() }},
		{"API Catalog", func() challenge.Challenge { return NewBrowsingAPICatalogChallenge() }},
		{"WebSocket Events", func() challenge.Challenge { return NewWebSocketEventsChallenge() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ch := tt.create()

			type executor interface {
				Execute(context.Context) (*challenge.Result, error)
			}
			exec, ok := ch.(executor)
			require.True(t, ok)

			result, err := exec.Execute(ctx)
			require.NoError(t, err, "Execute should not return error")
			require.NotNil(t, result)
			assert.True(t, len(result.Assertions) > 0,
				"should have at least one assertion")
		})
	}
}

func TestExecute_Populate_WithMockServer(t *testing.T) {
	baseURL := startMockAPIServer(t)

	t.Setenv("BROWSING_API_URL", baseURL)
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", testAdminPassword)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ch := NewFirstCatalogPopulateChallenge()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, len(result.Assertions) > 0)
}

func TestExecute_WebApp_WithMockWebServer(t *testing.T) {
	webMux := http.NewServeMux()
	webMux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<!DOCTYPE html><html><head><title>Catalogizer</title></head><body><div id="root"></div></body></html>`)
	})

	webServer := &http.Server{Handler: webMux}
	webListener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	go func() { _ = webServer.Serve(webListener) }()
	defer webServer.Close()

	webURL := "http://" + webListener.Addr().String()
	apiURL := startMockAPIServer(t)

	t.Setenv("BROWSING_WEB_URL", webURL)
	t.Setenv("BROWSING_API_URL", apiURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ch := NewBrowsingWebAppChallenge()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, len(result.Assertions) > 0)
}
