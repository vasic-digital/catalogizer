package services

import (
	"encoding/json"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pstr is a test-only helper that returns a *string pointing to s.
// Named pstr (not strPtr) to avoid conflict with media_player_service_test.go.
func pstr(s string) *string { return &s }

// identityOptionsJSON is a helper to build the options JSON for a storage root.
func identityOptionsJSON(index int) *string {
	b, _ := json.Marshal(identityOptions{IdentityIndex: index})
	s := string(b)
	return &s
}

func TestResolveSMBIdentity_DirectCredentials(t *testing.T) {
	// When Username is non-nil, ResolveSMBIdentity MUST return it directly,
	// ignoring options and env vars (§11.4.6 — deterministic priority order).
	root := &models.StorageRoot{
		Username: pstr("alice"),
		Password: pstr("secret"),
		Domain:   pstr("WORKGROUP"),
	}
	u, p, d := ResolveSMBIdentity(root)
	assert.Equal(t, "alice", u)
	assert.Equal(t, "secret", p)
	assert.Equal(t, "WORKGROUP", d)
}

func TestResolveSMBIdentity_DirectCredentialsOverridesIdentityIndex(t *testing.T) {
	// When BOTH direct credentials AND identity_index exist, direct credentials win.
	root := &models.StorageRoot{
		Username: pstr("bob"),
		Password: pstr("bobpass"),
		Options:  identityOptionsJSON(1),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "bob", u)
	assert.Equal(t, "bobpass", p)
}

func TestResolveSMBIdentity_NilUsernameWithIdentityIndex(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "2")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "api_token")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "skip_me")
	t.Setenv("CATALOGIZER_IDENTITY_1_TOKEN", "tok1")
	t.Setenv("CATALOGIZER_IDENTITY_2_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_2_USERNAME", "nas_admin")
	t.Setenv("CATALOGIZER_IDENTITY_2_PASSWORD", "nas_secret")
	t.Setenv("CATALOGIZER_IDENTITY_2_DOMAIN", "MYDOMAIN")

	root := &models.StorageRoot{
		Username: nil, // No direct credentials — must resolve from options
		Password: nil,
		Options:  identityOptionsJSON(2),
	}
	u, p, d := ResolveSMBIdentity(root)
	assert.Equal(t, "nas_admin", u)
	assert.Equal(t, "nas_secret", p)
	assert.Equal(t, "MYDOMAIN", d)
}

func TestResolveSMBIdentity_EmptyUsernameWithIdentityIndex(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "guest_user")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "guest_pass")

	root := &models.StorageRoot{
		Username: pstr(""), // Non-nil but empty — still treated as "no direct creds"
		Password: nil,
		Options:  identityOptionsJSON(1),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "guest_user", u)
	assert.Equal(t, "guest_pass", p)
}

func TestResolveSMBIdentity_NilUsernameNoIdentityIndex(t *testing.T) {
	// No direct credentials, no identity_index in options → empty strings
	root := &models.StorageRoot{
		Username: nil,
		Password: nil,
		Options:  nil,
	}
	u, p, d := ResolveSMBIdentity(root)
	assert.Equal(t, "", u)
	assert.Equal(t, "", p)
	assert.Equal(t, "", d)
}

func TestResolveSMBIdentity_NoMatchingIdentityIndex(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "adam")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "adam_pass")

	// Identity index 2 does not exist (only identity 1 is configured)
	root := &models.StorageRoot{
		Username: nil,
		Options:  identityOptionsJSON(2),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "", u)
	assert.Equal(t, "", p)
}

func TestResolveSMBIdentity_InvalidOptionsJSON(t *testing.T) {
	// Malformed options JSON should silently return empty strings
	root := &models.StorageRoot{
		Username: nil,
		Options:  pstr("{bad json}"),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "", u)
	assert.Equal(t, "", p)
}

func TestResolveSMBIdentity_IdentityIndexZero(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "adam")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "adam_pass")

	// identity_index 0 means guest/anonymous — should NOT resolve to a configured identity
	root := &models.StorageRoot{
		Username: nil,
		Options:  identityOptionsJSON(0),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "", u)
	assert.Equal(t, "", p)
}

func TestResolveSMBIdentity_SkipsNonCredentialIdentities(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "3")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "api_token")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "bot")
	t.Setenv("CATALOGIZER_IDENTITY_2_TYPE", "ssh_key")
	t.Setenv("CATALOGIZER_IDENTITY_2_USERNAME", "key_user")
	t.Setenv("CATALOGIZER_IDENTITY_3_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_3_USERNAME", "real_user")
	t.Setenv("CATALOGIZER_IDENTITY_3_PASSWORD", "real_pass")

	root := &models.StorageRoot{
		Username: nil,
		Options:  identityOptionsJSON(3),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "real_user", u)
	assert.Equal(t, "real_pass", p)
}

func TestResolveSMBIdentity_NoEnvVars(t *testing.T) {
	// When CATALOGIZER_IDENTITY_COUNT is unset, fallback gracefully.
	root := &models.StorageRoot{
		Username: nil,
		Options:  identityOptionsJSON(1),
	}
	u, p, _ := ResolveSMBIdentity(root)
	assert.Equal(t, "", u)
	assert.Equal(t, "", p)
}

// scanHandlerTestRoot creates a StorageRoot suitable for testing the scanner's
// storageRootToSettings with identity-based credentials.
func scanHandlerTestRoot(t *testing.T, username, password *string, identityIndex int) *models.StorageRoot {
	t.Helper()
	host := "nas-test"
	port := 445
	share := "media"
	return &models.StorageRoot{
		Name:     "test-scan-root",
		Protocol: "smb",
		Host:     &host,
		Port:     &port,
		Path:     &share,
		Username: username,
		Password: password,
		Domain:   nil,
		Options:  identityOptionsJSON(identityIndex),
	}
}

func TestStorageRootToSettings_IdentityResolution(t *testing.T) {
	// Simulate what the probe-and-ingest pipeline produces:
	// a storage root with NULL username/password and {"identity_index": 1} in options.
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "probe_user")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "probe_pass")

	scanner := &UniversalScanner{}
	root := scanHandlerTestRoot(t, nil, nil, 1)

	settings, serr := scanner.storageRootToSettings(root)
		require.NoError(t, serr)
	require.Equal(t, "probe_user", settings["username"], "scanner should resolve username from identity_index")
	require.Equal(t, "probe_pass", settings["password"], "scanner should resolve password from identity_index")
}

func TestStorageRootToSettings_DirectCredentialsWin(t *testing.T) {
	// Even when identity_index is set, direct credentials must take priority.
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "env_user")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "env_pass")

	scanner := &UniversalScanner{}
	root := scanHandlerTestRoot(t, pstr("direct_user"), pstr("direct_pass"), 1)

	settings, serr := scanner.storageRootToSettings(root)
		require.NoError(t, serr)
	assert.Equal(t, "direct_user", settings["username"], "direct credentials must override identity")
	assert.Equal(t, "direct_pass", settings["password"], "direct credentials must override identity")
}
