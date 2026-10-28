package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Known placeholder / published-default secrets must be rejected at boot even
// when they satisfy the length check (the .env.example JWT placeholder is 52
// characters long).
func TestValidateConfig_RejectsPlaceholderJWTSecret(t *testing.T) {
	for _, jwt := range []string{
		"your-super-secret-jwt-key-change-me-in-production",
		"your-super-secret-jwt-key-change-this",
		"CHANGE_ME_CHANGE_ME_CHANGE_ME_CHANGE_ME_CHANGE_ME",
		"changeme-changeme-changeme-changeme-changeme",
		"Your-Super-Secret-Key-0123456789-abcdefghijkl",
	} {
		t.Run(jwt, func(t *testing.T) {
			os.Setenv("JWT_SECRET", jwt)
			os.Setenv("ADMIN_USERNAME", "admin")
			os.Setenv("ADMIN_PASSWORD", "a-real-looking-pass-9f3k")
			defer func() {
				os.Unsetenv("JWT_SECRET")
				os.Unsetenv("ADMIN_USERNAME")
				os.Unsetenv("ADMIN_PASSWORD")
			}()
			err := validateConfig(getDefaultConfig())
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "placeholder")
			}
		})
	}
}

func TestValidateConfig_RejectsPlaceholderAdminPassword(t *testing.T) {
	for _, pw := range []string{"admin_password", "CHANGE_ME", "change-me", "changeme", "admin", "password"} {
		t.Run(pw, func(t *testing.T) {
			os.Setenv("JWT_SECRET", "k7Qz9xLw2VbN4mRt8YcHd1FgJp6SaUe3")
			os.Setenv("ADMIN_USERNAME", "admin")
			os.Setenv("ADMIN_PASSWORD", pw)
			defer func() {
				os.Unsetenv("JWT_SECRET")
				os.Unsetenv("ADMIN_USERNAME")
				os.Unsetenv("ADMIN_PASSWORD")
			}()
			err := validateConfig(getDefaultConfig())
			if assert.Error(t, err) {
				assert.Contains(t, err.Error(), "placeholder")
			}
		})
	}
}

// Control: a random-looking secret and password are accepted.
func TestValidateConfig_AcceptsRandomSecrets(t *testing.T) {
	os.Setenv("JWT_SECRET", "k7Qz9xLw2VbN4mRt8YcHd1FgJp6SaUe3")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "a-real-looking-pass-9f3k")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()
	assert.NoError(t, validateConfig(getDefaultConfig()))
}
