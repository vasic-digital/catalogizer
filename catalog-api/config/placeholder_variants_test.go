package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAuthEnv(t *testing.T, jwt, user, pass string) {
	t.Helper()
	t.Setenv("JWT_SECRET", jwt)
	t.Setenv("ADMIN_USERNAME", user)
	t.Setenv("ADMIN_PASSWORD", pass)
}

const goodJWT = "f3a91c07d2b84e5a9c6d1e0b7a38f245c9d0e1f2a3b4c5d6"
const goodPass = "a-real-looking-pass-9f3k"

// Every literal default this repository used to ship, plus whitespace/case/
// separator variants, must be rejected.
func TestPlaceholderJWT_AllRemovedDefaultsAndVariants(t *testing.T) {
	for _, jwt := range []string{
		"distributed-catalogizer-secret-key-minimum-32-chars",
		"catalogizer-development-secret-key-min-32-chars",
		"your-super-secret-jwt-key-change-this",
		"your super secret jwt key change me in production",
		"change me change me change me change me change me",
		"Change.Me-Change.Me-Change.Me-Change.Me-Change.Me",
		"  distributed-catalogizer-secret-key-minimum-32-chars\n",
		"CATALOGIZER-DEVELOPMENT-SECRET-KEY-MIN-32-CHARS",
	} {
		setAuthEnv(t, jwt, "admin", goodPass)
		err := validateConfig(getDefaultConfig())
		if assert.Error(t, err, "%q", jwt) {
			assert.Contains(t, err.Error(), "placeholder", "%q", jwt)
		}
	}
}

func TestPlaceholderAdminPassword_AllRemovedDefaultsAndVariants(t *testing.T) {
	for _, pw := range []string{
		"admin123", "catalogizer_dev", "catalogizer_password", "admin_password",
		" admin", "admin ", "admin\n", "\tAdmin\r\n", "ADMIN123", "Admin-123",
		"minioadmin", "minioadmin123", "sonar_password", "password", "change me", "12345678",
	} {
		setAuthEnv(t, goodJWT, "admin", pw)
		err := validateConfig(getDefaultConfig())
		if assert.Error(t, err, "%q", pw) {
			assert.Contains(t, err.Error(), "placeholder", "%q", pw)
		}
	}
}

// LoadConfig on a MISSING config.json (every fresh container) must validate too.
func TestLoadConfig_MissingFile_RejectsPlaceholders(t *testing.T) {
	setAuthEnv(t, "your-super-secret-jwt-key-change-me-in-production", "admin", goodPass)
	p := filepath.Join(t.TempDir(), "config.json")
	_, err := LoadConfig(p)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "placeholder")
	}
	setAuthEnv(t, goodJWT, "admin", "admin123")
	_, err = LoadConfig(filepath.Join(t.TempDir(), "config.json"))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "placeholder")
	}
}

func TestLoadConfig_MissingFile_AcceptsRandomSecrets(t *testing.T) {
	setAuthEnv(t, goodJWT, "admin", goodPass)
	p := filepath.Join(t.TempDir(), "config.json")
	cfg, err := LoadConfig(p)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	_, serr := os.Stat(p)
	assert.NoError(t, serr)
}
