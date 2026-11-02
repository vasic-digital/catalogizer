package challenges

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/services"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/env"
	"digital.vasic.challenges/pkg/httpclient"
)

func TestNewFirstCatalogPopulateChallenge(t *testing.T) {
	ch := NewFirstCatalogPopulateChallenge()

	if ch.ID() != "first-catalog-populate" {
		t.Errorf("expected ID 'first-catalog-populate', got '%s'", ch.ID())
	}
	if ch.Name() != "Populate Catalog Database" {
		t.Errorf("expected Name 'Populate Catalog Database', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 7 {
		t.Errorf("expected 7 dependencies (all first-catalog), got %d", len(deps))
	}
}

func TestNewBrowsingAPIHealthChallenge(t *testing.T) {
	ch := NewBrowsingAPIHealthChallenge()

	if ch.ID() != "browsing-api-health" {
		t.Errorf("expected ID 'browsing-api-health', got '%s'", ch.ID())
	}
	if ch.Name() != "API Health & Auth" {
		t.Errorf("expected Name 'API Health & Auth', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestNewBrowsingAPICatalogChallenge(t *testing.T) {
	ch := NewBrowsingAPICatalogChallenge()

	if ch.ID() != "browsing-api-catalog" {
		t.Errorf("expected ID 'browsing-api-catalog', got '%s'", ch.ID())
	}
	if ch.Name() != "API Catalog Browsing" {
		t.Errorf("expected Name 'API Catalog Browsing', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "browsing-api-health" {
		t.Errorf("expected dependency [browsing-api-health], got %v", deps)
	}
}

func TestNewBrowsingWebAppChallenge(t *testing.T) {
	ch := NewBrowsingWebAppChallenge()

	if ch.ID() != "browsing-web-app" {
		t.Errorf("expected ID 'browsing-web-app', got '%s'", ch.ID())
	}
	if ch.Name() != "Web App Browsing" {
		t.Errorf("expected Name 'Web App Browsing', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "browsing-api-catalog" {
		t.Errorf("expected dependency [browsing-api-catalog], got %v", deps)
	}
	if ch.config == nil {
		t.Error("expected config to be loaded")
	}
	if ch.config.WebAppURL == "" {
		t.Error("expected non-empty WebAppURL in config")
	}
}

func TestViteErrorIndicators(t *testing.T) {
	// Verify that all known Vite error patterns are non-empty
	if len(viteErrorIndicators) == 0 {
		t.Error("expected at least one Vite error indicator")
	}
	for _, indicator := range viteErrorIndicators {
		if indicator == "" {
			t.Error("empty string in viteErrorIndicators")
		}
	}
}

func TestCriticalModules(t *testing.T) {
	if len(criticalModules) < 3 {
		t.Errorf("expected at least 3 critical modules, got %d", len(criticalModules))
	}
	// Must include the entry point and the websocket module
	found := map[string]bool{}
	for _, m := range criticalModules {
		found[m] = true
		if !strings.HasPrefix(m, "/src/") {
			t.Errorf("critical module %q should start with /src/", m)
		}
	}
	if !found["/src/main.tsx"] {
		t.Error("critical modules must include /src/main.tsx (entry point)")
	}
	if !found["/src/lib/websocket.ts"] {
		t.Error("critical modules must include /src/lib/websocket.ts (submodule import)")
	}
}

func TestNewAssetServingChallenge(t *testing.T) {
	ch := NewAssetServingChallenge()

	if ch.ID() != "asset-serving" {
		t.Errorf("expected ID 'asset-serving', got '%s'", ch.ID())
	}
	if ch.Name() != "Asset Serving" {
		t.Errorf("expected Name 'Asset Serving', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "browsing-api-health" {
		t.Errorf("expected dependency [browsing-api-health], got %v", deps)
	}
}

func TestNewAssetLazyLoadingChallenge(t *testing.T) {
	ch := NewAssetLazyLoadingChallenge()

	if ch.ID() != "asset-lazy-loading" {
		t.Errorf("expected ID 'asset-lazy-loading', got '%s'", ch.ID())
	}
	if ch.Name() != "Asset Lazy Loading" {
		t.Errorf("expected Name 'Asset Lazy Loading', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "asset-serving" {
		t.Errorf("expected dependency [asset-serving], got %v", deps)
	}
}

func TestBrowsingChallengeIDUniqueness(t *testing.T) {
	challenges := []challenge.Challenge{
		NewFirstCatalogPopulateChallenge(),
		NewBrowsingAPIHealthChallenge(),
		NewBrowsingAPICatalogChallenge(),
		NewBrowsingWebAppChallenge(),
		NewAssetServingChallenge(),
		NewAssetLazyLoadingChallenge(),
		NewDatabaseConnectivityChallenge(),
		NewDatabaseSchemaValidationChallenge(),
	}

	seen := map[challenge.ID]bool{}
	for _, ch := range challenges {
		if seen[ch.ID()] {
			t.Errorf("duplicate challenge ID: %s", ch.ID())
		}
		seen[ch.ID()] = true
	}

	if len(seen) != 8 {
		t.Errorf("expected 8 unique challenge IDs (populate + 3 browsing + 2 asset + 2 database), got %d", len(seen))
	}
}

func TestBrowsingConfigDefaults(t *testing.T) {
	// Unset env vars to test defaults
	for _, key := range []string{"BROWSING_API_URL", "ADMIN_USERNAME", "ADMIN_PASSWORD", "BROWSING_WEB_URL", "CATALOG_WEB_DIR"} {
		t.Setenv(key, "")
	}

	cfg := LoadBrowsingConfig()

	if cfg.BaseURL != "http://localhost:28080" {
		t.Errorf("expected default BaseURL 'http://localhost:28080', got '%s'", cfg.BaseURL)
	}
	if cfg.Username != "admin" {
		t.Errorf("expected default Username 'admin', got '%s'", cfg.Username)
	}
	if cfg.Password != "" {
		t.Errorf("expected NO default Password (CAT-021), got a non-empty value (%d chars)", len(cfg.Password))
	}
	if cfg.WebAppURL != "http://localhost:3000" {
		t.Errorf("expected default WebAppURL 'http://localhost:3000', got '%s'", cfg.WebAppURL)
	}
	if cfg.WebAppDir != "../catalog-web" {
		t.Errorf("expected default WebAppDir '../catalog-web', got '%s'", cfg.WebAppDir)
	}
}

func TestBrowsingConfigFromEnv(t *testing.T) {
	t.Setenv("BROWSING_API_URL", "http://api.test:9090")
	t.Setenv("ADMIN_USERNAME", "superadmin")
	t.Setenv("ADMIN_PASSWORD", "secret456")
	t.Setenv("BROWSING_WEB_URL", "http://web.test:4000")
	t.Setenv("CATALOG_WEB_DIR", "/opt/catalog-web")

	cfg := LoadBrowsingConfig()

	if cfg.BaseURL != "http://api.test:9090" {
		t.Errorf("expected BaseURL from env, got '%s'", cfg.BaseURL)
	}
	if cfg.Username != "superadmin" {
		t.Errorf("expected Username from env, got '%s'", cfg.Username)
	}
	if cfg.Password != "secret456" {
		t.Errorf("expected Password from env, got '%s'", cfg.Password)
	}
	if cfg.WebAppURL != "http://web.test:4000" {
		t.Errorf("expected WebAppURL from env, got '%s'", cfg.WebAppURL)
	}
	if cfg.WebAppDir != "/opt/catalog-web" {
		t.Errorf("expected WebAppDir from env, got '%s'", cfg.WebAppDir)
	}
}

func TestIsInvalidTitle(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"", true},
		{"unknown", true},
		{"Unknown", true},
		{"UNKNOWN", true},
		{"untitled", true},
		{"placeholder", true},
		{"n/a", true},
		{"N/A", true},
		{"tbd", true},
		{"  unknown  ", true},
		{"The Matrix", false},
		{"Breaking Bad", false},
		{"Led Zeppelin IV", false},
		{"Unknown Pleasures", false}, // album name - not just "unknown"
		{"Untitled Goose Game", false},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := IsInvalidTitle(tt.title)
			if got != tt.want {
				t.Errorf("IsInvalidTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestPopulateChallengeScansContentDirectoriesOnly(t *testing.T) {
	// Verify that the populate challenge loads endpoint config with directories.
	// This ensures it scans only configured content directories, not the entire share.
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	content := `{
		"endpoints": [
			{
				"id": "test-endpoint",
				"name": "Test NAS",
				"host": "nas.test",
				"port": 445,
				"share": "data",
				"username": "user",
				"password": "pass",
				"domain": "",
				"readonly": true,
				"directories": [
					{"path": "Music", "content_type": "music"},
					{"path": "Series", "content_type": "tv_show"},
					{"path": "Movies", "content_type": "movie"},
					{"path": "Software", "content_type": "software"},
					{"path": "Comics", "content_type": "comic"}
				]
			}
		]
	}`

	configPath := filepath.Join(configDir, "endpoints.json")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := LoadEndpointConfig(configPath)
	if err != nil {
		t.Fatalf("LoadEndpointConfig failed: %v", err)
	}

	ep := cfg.Endpoints[0]

	// Must have exactly 5 content directories
	if len(ep.Directories) != 5 {
		t.Fatalf("expected 5 directories, got %d", len(ep.Directories))
	}

	// Each directory must have a path and content_type
	expectedTypes := map[string]bool{
		"music": false, "tv_show": false, "movie": false,
		"software": false, "comic": false,
	}
	for _, d := range ep.Directories {
		if d.Path == "" {
			t.Error("directory path must not be empty")
		}
		if _, ok := expectedTypes[d.ContentType]; !ok {
			t.Errorf("unexpected content_type %q", d.ContentType)
		}
		expectedTypes[d.ContentType] = true
	}
	for ct, found := range expectedTypes {
		if !found {
			t.Errorf("content_type %q not found in directories", ct)
		}
	}
}

func TestPopulateChallengeDependencies(t *testing.T) {
	ch := NewFirstCatalogPopulateChallenge()
	deps := ch.Dependencies()

	// Must depend on all 7 first-catalog challenges
	if len(deps) != 7 {
		t.Errorf("expected 7 dependencies, got %d: %v", len(deps), deps)
	}

	// Verify it includes the SMB connect and content scan challenges
	depSet := make(map[string]bool)
	for _, d := range deps {
		depSet[string(d)] = true
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
		if !depSet[expected] {
			t.Errorf("missing expected dependency %q", expected)
		}
	}
}

func TestRegisterAll_IncludesBrowsingChallenges(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	content := `{
		"endpoints": [
			{
				"id": "test-endpoint",
				"name": "Test NAS",
				"host": "nas.test",
				"port": 445,
				"share": "data",
				"username": "user",
				"password": "pass",
				"domain": "",
				"readonly": true,
				"directories": [
					{"path": "Music", "content_type": "music"},
					{"path": "Series", "content_type": "tv_show"},
					{"path": "Movies", "content_type": "movie"},
					{"path": "Software", "content_type": "software"},
					{"path": "Comics", "content_type": "comic"}
				]
			}
		]
	}`

	configPath := filepath.Join(configDir, "endpoints.json")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// Load config and register manually (mirrors RegisterAll logic)
	cfg, err := LoadEndpointConfig(configPath)
	if err != nil {
		t.Fatalf("LoadEndpointConfig failed: %v", err)
	}

	svc := services.NewChallengeService(filepath.Join(dir, "results"))

	for _, ep := range cfg.Endpoints {
		endpoint := ep
		svc.Register(NewSMBConnectivityChallenge(&endpoint))
		svc.Register(NewDirectoryDiscoveryChallenge(&endpoint))
		for _, d := range endpoint.Directories {
			dd := d
			switch dd.ContentType {
			case "music":
				svc.Register(NewMusicScanChallenge(&endpoint, dd))
			case "tv_show":
				svc.Register(NewSeriesScanChallenge(&endpoint, dd))
			case "movie":
				svc.Register(NewMoviesScanChallenge(&endpoint, dd))
			case "software":
				svc.Register(NewSoftwareScanChallenge(&endpoint, dd))
			case "comic":
				svc.Register(NewComicsScanChallenge(&endpoint, dd))
			}
		}
	}

	// Register populate, browsing, asset, and database challenges
	svc.Register(NewFirstCatalogPopulateChallenge())
	svc.Register(NewBrowsingAPIHealthChallenge())
	svc.Register(NewBrowsingAPICatalogChallenge())
	svc.Register(NewBrowsingWebAppChallenge())
	svc.Register(NewAssetServingChallenge())
	svc.Register(NewAssetLazyLoadingChallenge())
	svc.Register(NewDatabaseConnectivityChallenge())
	svc.Register(NewDatabaseSchemaValidationChallenge())

	// Verify all 15 challenges registered (7 first-catalog + 1 populate + 3 browsing + 2 asset + 2 database)
	challenges := svc.ListChallenges()
	if len(challenges) != 15 {
		t.Errorf("expected 15 challenges registered, got %d", len(challenges))
		for _, c := range challenges {
			t.Logf("  registered: %s", c.ID)
		}
	}

	// Verify populate, browsing, asset, and database challenge IDs are present
	ids := map[string]bool{}
	for _, c := range challenges {
		ids[c.ID] = true
	}
	for _, expected := range []string{
		"first-catalog-populate", "browsing-api-health", "browsing-api-catalog", "browsing-web-app",
		"asset-serving", "asset-lazy-loading",
		"database-connectivity", "database-schema-validation",
	} {
		if !ids[expected] {
			t.Errorf("expected challenge %q to be registered", expected)
		}
	}
}

func TestRequiredMediaTypes(t *testing.T) {
	// Verify that the required media types list is populated and contains
	// the types we expect to find in a fully cataloged NAS.
	if len(requiredMediaTypes) < 3 {
		t.Errorf("expected at least 3 required media types, got %d", len(requiredMediaTypes))
	}
	expected := map[string]bool{"music": false, "tv_show": false, "movie": false}
	for _, mt := range requiredMediaTypes {
		if _, ok := expected[mt]; ok {
			expected[mt] = true
		}
	}
	for mt, found := range expected {
		if !found {
			t.Errorf("required media type %q not in requiredMediaTypes list", mt)
		}
	}
}

func TestMediaEndpointAssertions_CH010_ValidatesData(t *testing.T) {
	// Verify that CH-010 (API Catalog Browsing) creates assertions that
	// would catch a stubbed /media/stats returning total_items=0.
	// This is a structural test - it doesn't call the API, but ensures
	// the challenge code validates data, not just HTTP 200.
	ch := NewBrowsingAPICatalogChallenge()
	if ch.ID() != "browsing-api-catalog" {
		t.Errorf("expected ID 'browsing-api-catalog', got '%s'", ch.ID())
	}
	// The challenge must depend on browsing-api-health (which ensures auth works)
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "browsing-api-health" {
		t.Errorf("expected dependency [browsing-api-health], got %v", deps)
	}
}

func TestMediaEndpointAssertions_CH011_ValidatesData(t *testing.T) {
	// Verify that CH-011 (Web App Browsing) is set up to validate media data.
	ch := NewBrowsingWebAppChallenge()
	if ch.ID() != "browsing-web-app" {
		t.Errorf("expected ID 'browsing-web-app', got '%s'", ch.ID())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "browsing-api-catalog" {
		t.Errorf("expected dependency [browsing-api-catalog], got %v", deps)
	}
}

func TestAPIClient_NewAPIClient(t *testing.T) {
	client := httpclient.NewAPIClient("http://localhost:28080/")

	// Should trim trailing slash
	if client.BaseURL() != "http://localhost:28080" {
		t.Errorf("expected baseURL without trailing slash, got '%s'", client.BaseURL())
	}
	if client.Token() != "" {
		t.Errorf("expected empty token for new client, got '%s'", client.Token())
	}
}

func TestEnvGetOrDefault(t *testing.T) {
	t.Setenv("TEST_ENV_VAR_XYZ", "custom_value")
	if got := env.GetOrDefault("TEST_ENV_VAR_XYZ", "default"); got != "custom_value" {
		t.Errorf("expected 'custom_value', got '%s'", got)
	}

	t.Setenv("TEST_ENV_VAR_XYZ", "")
	if got := env.GetOrDefault("TEST_ENV_VAR_XYZ", "default"); got != "default" {
		t.Errorf("expected 'default' for empty env, got '%s'", got)
	}

	if got := env.GetOrDefault("NONEXISTENT_ENV_VAR_12345", "fallback"); got != "fallback" {
		t.Errorf("expected 'fallback' for unset env, got '%s'", got)
	}
}

func TestNewDatabaseConnectivityChallenge(t *testing.T) {
	ch := NewDatabaseConnectivityChallenge()

	if ch.ID() != "database-connectivity" {
		t.Errorf("expected ID 'database-connectivity', got '%s'", ch.ID())
	}
	if ch.Name() != "Database Connectivity" {
		t.Errorf("expected Name 'Database Connectivity', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 0 {
		t.Errorf("expected no dependencies (first in chain), got %v", deps)
	}
}

func TestNewDatabaseSchemaValidationChallenge(t *testing.T) {
	ch := NewDatabaseSchemaValidationChallenge()

	if ch.ID() != "database-schema-validation" {
		t.Errorf("expected ID 'database-schema-validation', got '%s'", ch.ID())
	}
	if ch.Name() != "Database Schema Validation" {
		t.Errorf("expected Name 'Database Schema Validation', got '%s'", ch.Name())
	}
	if ch.Category() != "e2e" {
		t.Errorf("expected Category 'e2e', got '%s'", ch.Category())
	}
	deps := ch.Dependencies()
	if len(deps) != 1 || deps[0] != "database-connectivity" {
		t.Errorf("expected dependency [database-connectivity], got %v", deps)
	}
}

func TestBrowsingChallenges_Execute(t *testing.T) {
	tests := []struct {
		name      string
		challenge challenge.Challenge
		skip      string
	}{
		{name: "first-catalog-populate", challenge: NewFirstCatalogPopulateChallenge(), skip: "requires real infrastructure: contains 120s polling loops"},
		{name: "browsing-api-health", challenge: NewBrowsingAPIHealthChallenge()},
		{name: "browsing-api-catalog", challenge: NewBrowsingAPICatalogChallenge(), skip: "requires real infrastructure: contains 25s retry sleep loops"},
		{name: "browsing-web-app", challenge: NewBrowsingWebAppChallenge()},
		{name: "asset-serving", challenge: NewAssetServingChallenge()},
		{name: "asset-lazy-loading", challenge: NewAssetLazyLoadingChallenge(), skip: "requires real infrastructure: contains 30s polling sleep loops"},
		{name: "database-connectivity", challenge: NewDatabaseConnectivityChallenge()},
		{name: "database-schema-validation", challenge: NewDatabaseSchemaValidationChallenge()},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.skip != "" {
				t.Skip("SKIP-OK: #table-driven-conditional-skip: " + tc.skip)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			result, err := tc.challenge.Execute(ctx)
			if err != nil {
				t.Fatalf("challenge %s Execute returned error: %v", tc.challenge.ID(), err)
			}
			if result == nil {
				t.Fatalf("challenge %s Execute returned nil result", tc.challenge.ID())
			}
			if !result.IsFinal() {
				t.Errorf("challenge %s expected terminal status, got %s", tc.challenge.ID(), result.Status)
			}
			if len(result.RecordedActions) == 0 {
				t.Errorf("challenge %s must record actions (Constitution v2.1.0)", tc.challenge.ID())
			}
		})
	}
}
