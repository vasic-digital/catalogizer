package services

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"

	"golang.org/x/crypto/bcrypt"
)

// Canonical password scheme for the users table (password_hash + salt):
//
//	password_hash = bcrypt(password + salt)
//
// This is the only scheme every verifier (AuthService.Login, ChangePassword,
// the change-password endpoint, the admin-reset CLI, the default-admin seed)
// accepts, so every code path that WRITES a hash MUST use HashPasswordWithSalt.
//
// Compatibility: before the shared hasher existed, POST /users stored
// hex(SHA-256(password + salt)) which no verifier accepted. Those rows are
// recognised by shape (64 lowercase hex chars; a bcrypt hash always starts with
// "$2"), verified with a constant-time compare, and reported with
// needsUpgrade=true so the caller rewrites them as bcrypt on a successful login.

// HashPasswordWithSalt returns the canonical bcrypt hash of password+salt.
func HashPasswordWithSalt(password, salt string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password+salt), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

// CheckPassword verifies password+salt against a stored hash. needsUpgrade is
// true when the hash matched only as a legacy hex SHA-256 value and should be
// rewritten with HashPasswordWithSalt.
func CheckPassword(password, salt, hash string) (ok bool, needsUpgrade bool) {
	if isLegacySHA256Hex(hash) {
		sum := sha256.Sum256([]byte(password + salt))
		want := hex.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(want), []byte(hash)) == 1 {
			return true, true
		}
		return false, false
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+salt)) == nil {
		return true, false
	}
	return false, false
}

// isLegacySHA256Hex reports whether s has the exact shape the old CreateUser
// wrote: 64 lowercase hex characters.
func isLegacySHA256Hex(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
