package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Canonical password scheme for the users table (password_hash + salt).
//
// NEW hashes (every writer goes through HashPasswordWithSalt) are versioned:
//
//	password_hash = "$cz1$" + bcrypt( base64( HMAC-SHA256(key=salt, msg=password) ) )
//
// The HMAC pre-hash makes the bcrypt input a fixed 44 bytes, so any password
// length accepted by ValidatePassword (up to 128 bytes) hashes correctly. The
// previous scheme bcrypt(password+salt) FAILED for passwords over 40 bytes
// (bcrypt rejects input over 72 bytes and the salt is 32 characters); those
// rows are still verified by the v1 path below, which simply cannot match a
// password that was never storable.
//
// Verification accepts three stored shapes, selected by the hash itself:
//   - "$cz1$..."            v2 (above)
//   - "$2a$/$2b$/$2y$..."   v1 bcrypt(password+salt), rows written before v2
//   - 64 lowercase hex      legacy SHA-256(password+salt), written by the old
//     POST /users; verified in constant time and reported with
//     needsUpgrade=true so the login rewrites it (compare-and-swap, see
//     AuthService.Login). Legacy rows have NO sunset yet (tracked follow-up).
//
// Only v2 is ever written. v1 rows stay valid until the user's next password
// change.

const v2Prefix = "$cz1$"

func v2Input(password, salt string) []byte {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(password))
	return []byte(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

// HashPasswordWithSalt returns the canonical (v2) hash of password under salt.
func HashPasswordWithSalt(password, salt string) (string, error) {
	h, err := bcrypt.GenerateFromPassword(v2Input(password, salt), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return v2Prefix + string(h), nil
}

// CheckPassword verifies password under salt against a stored hash. needsUpgrade
// is true when the hash matched only as a legacy hex SHA-256 value and should be
// rewritten with HashPasswordWithSalt.
func CheckPassword(password, salt, hash string) (ok bool, needsUpgrade bool) {
	switch {
	case strings.HasPrefix(hash, v2Prefix):
		return bcrypt.CompareHashAndPassword([]byte(strings.TrimPrefix(hash, v2Prefix)), v2Input(password, salt)) == nil, false
	case isLegacySHA256Hex(hash):
		sum := sha256.Sum256([]byte(password + salt))
		want := hex.EncodeToString(sum[:])
		if subtle.ConstantTimeCompare([]byte(want), []byte(hash)) == 1 {
			return true, true
		}
		return false, false
	default:
		// v1: bcrypt(password+salt). Errors (input over 72 bytes) mean "no match".
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password+salt)) == nil, false
	}
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
