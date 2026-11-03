package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPasswordWithSalt("Pw-Throwaway1!", "s1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$2"))
	ok, up := CheckPassword("Pw-Throwaway1!", "s1", h)
	assert.True(t, ok)
	assert.False(t, up)
	ok, _ = CheckPassword("Pw-Throwaway1!", "s2", h)
	assert.False(t, ok)
	ok, _ = CheckPassword("nope", "s1", h)
	assert.False(t, ok)
}

func TestCheckPassword_LegacySHA256(t *testing.T) {
	sum := sha256.Sum256([]byte("Pw-Throwaway1!" + "s1"))
	legacy := hex.EncodeToString(sum[:])
	ok, up := CheckPassword("Pw-Throwaway1!", "s1", legacy)
	assert.True(t, ok)
	assert.True(t, up)
	ok, up = CheckPassword("wrong", "s1", legacy)
	assert.False(t, ok)
	assert.False(t, up)
	ok, _ = CheckPassword("Pw-Throwaway1!", "other-salt", legacy)
	assert.False(t, ok)
	// Hash-as-password replay: presenting the stored hex as the password fails.
	ok, _ = CheckPassword(legacy, "s1", legacy)
	assert.False(t, ok)
}

func TestCheckPassword_RejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "x", strings.Repeat("0", 64), strings.Repeat("g", 64), strings.Repeat("A", 64), "$2a$10$short"} {
		ok, up := CheckPassword("Pw-Throwaway1!", "s1", bad)
		assert.False(t, ok, bad)
		assert.False(t, up, bad)
	}
}
