package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// CAT-LOGSHARE-TOK-1: log-share tokens are bearer secrets. A token is `log_share_` + 32 bytes (256 bits) from crypto/rand,
// base64url without padding. Only the SHA-256 of the token is stored (the token is shown once, in the create response), the
// lookup key is that hash, the stored hash is compared in constant time, and an unknown / malformed / expired / revoked
// token all produce the same error (ErrLogShareNotFound), so the route is not an oracle.
const (
	logShareTokenPrefix    = "log_share_"
	logShareTokenRandBytes = 32
	logShareTokenLen       = len(logShareTokenPrefix) + 43 // base64url(32 bytes), no padding = 43 chars
)

// ErrLogShareNotFound is the single error for every way a presented share token can fail (malformed, unknown, expired,
// revoked): the caller, and the API client behind it, cannot tell them apart.
var ErrLogShareNotFound = errors.New("log share not found")

// logShareRand is the entropy source; tests replace it to prove token generation fails closed.
var logShareRand io.Reader = rand.Reader

// newLogShareToken returns a fresh 256-bit token, or an error when the entropy source cannot supply all 32 bytes.
func newLogShareToken(r io.Reader) (string, error) {
	buf := make([]byte, logShareTokenRandBytes)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", fmt.Errorf("generate log share token: %w", err)
	}
	return logShareTokenPrefix + base64.RawURLEncoding.EncodeToString(buf), nil
}

// hashLogShareToken is the at-rest form of a token (lower-case hex SHA-256).
func hashLogShareToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// validLogShareTokenFormat accepts exactly the shape newLogShareToken produces; anything else (a legacy
// `log_share_<unix>_<nanos>` value included) is rejected before the database is touched.
func validLogShareTokenFormat(token string) bool {
	if len(token) != logShareTokenLen || !strings.HasPrefix(token, logShareTokenPrefix) {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(token[len(logShareTokenPrefix):])
	return err == nil && len(raw) == logShareTokenRandBytes
}

// logShareHashEqual compares two stored/computed hashes without an early exit.
func logShareHashEqual(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
