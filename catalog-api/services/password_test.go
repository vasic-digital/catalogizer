package services

import (
	"crypto/sha256"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashAndCheckPassword(t *testing.T) {
	h, err := HashPasswordWithSalt("Pw-Throwaway1!", "s1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(h, "$cz1$"))
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

func TestHashPasswordWithSalt_AllLengths(t *testing.T) {
	salt := "0123456789abcdef0123456789abcdef"
	for _, n := range []int{1, 8, 40, 41, 72, 73, 128, 1000} {
		pw := strings.Repeat("p", n)
		h, err := HashPasswordWithSalt(pw, salt)
		require.NoError(t, err, n)
		ok, up := CheckPassword(pw, salt, h)
		assert.True(t, ok, n)
		assert.False(t, up)
		ok, _ = CheckPassword(pw+"q", salt, h)
		assert.False(t, ok, n)
	}
}

// v1 rows (bcrypt(password+salt), written before the versioned scheme) stay valid.
func TestCheckPassword_V1BcryptRowsStillVerify(t *testing.T) {
	h, err := bcrypt.GenerateFromPassword([]byte("Pw-Throwaway1!"+"s1"), bcrypt.MinCost)
	require.NoError(t, err)
	ok, up := CheckPassword("Pw-Throwaway1!", "s1", string(h))
	assert.True(t, ok)
	assert.False(t, up)
	ok, _ = CheckPassword("nope", "s1", string(h))
	assert.False(t, ok)
	// over-72-byte input can never match a v1 row (bcrypt errors) - and must not panic
	ok, _ = CheckPassword(strings.Repeat("z", 80), "s1", string(h))
	assert.False(t, ok)
}
