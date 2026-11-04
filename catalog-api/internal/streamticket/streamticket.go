// Package streamticket issues and verifies short-lived stream tickets.
//
// A <video>/<audio> element cannot send an Authorization header, and putting the session JWT in the URL leaks it into access
// logs, browser history and Referer headers. A ticket is the narrow replacement: HMAC-SHA256 signed, bound to ONE media id and
// ONE user (and the session that minted it, so logout revokes it), valid for a short time, with a purpose claim so it can never
// stand in for any other credential. It is accepted only by the stream route (middleware/auth.go), only on GET/HEAD.
package streamticket

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// TTL is how long a ticket is valid. Range/seek requests of one playback re-use the ticket within it; the client mints a new one
	// before expiry or on a 401.
	TTL = 60 * time.Second
	// Purpose is the claim value of a stream ticket. A verifier refuses any other purpose.
	Purpose = "stream"
	version = "v1"
	// keyContext domain-separates the ticket key from the JWT signing key derived from the same secret.
	keyContext = "catalogizer.stream-ticket.v1"
)

// ErrInvalid is returned for every verification failure; it deliberately carries no detail (no oracle for forgers).
var ErrInvalid = errors.New("invalid stream ticket")

// Claims are the signed contents of a ticket.
type Claims struct {
	Purpose   string `json:"p"`
	MediaID   int64  `json:"m"`
	UserID    string `json:"u"`
	Username  string `json:"n,omitempty"`
	SessionID string `json:"s,omitempty"`
	Expires   int64  `json:"e"` // unix seconds
}

// Signer mints and verifies tickets.
type Signer struct {
	key []byte
	now func() time.Time
}

// New derives a Signer from the server secret. An empty secret yields nil: callers must treat a nil Signer as "tickets disabled".
func New(secret string) *Signer {
	if secret == "" {
		return nil
	}
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(keyContext))
	return &Signer{key: m.Sum(nil), now: time.Now}
}

// WithClock returns a copy using the given clock (tests).
func (s *Signer) WithClock(now func() time.Time) *Signer { c := *s; c.now = now; return &c }

func (s *Signer) mac(payload string) []byte {
	m := hmac.New(sha256.New, s.key)
	m.Write([]byte(payload))
	return m.Sum(nil)
}

// Mint returns a ticket for (mediaID, user, session) valid for TTL, and its expiry.
func (s *Signer) Mint(mediaID int64, userID, username, sessionID string) (string, time.Time) {
	exp := s.now().Add(TTL)
	raw, _ := json.Marshal(Claims{Purpose: Purpose, MediaID: mediaID, UserID: userID, Username: username, SessionID: sessionID, Expires: exp.Unix()})
	p := base64.RawURLEncoding.EncodeToString(raw)
	return version + "." + p + "." + base64.RawURLEncoding.EncodeToString(s.mac(version+"."+p)), exp
}

// Verify checks the signature (constant time), the purpose, the expiry and that the ticket was minted for mediaID.
func (s *Signer) Verify(ticket string, mediaID int64) (Claims, error) {
	var c Claims
	parts := strings.Split(ticket, ".")
	if len(parts) != 3 || parts[0] != version {
		return c, ErrInvalid
	}
	got, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(got, s.mac(parts[0]+"."+parts[1])) {
		return c, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil || json.Unmarshal(raw, &c) != nil {
		return Claims{}, ErrInvalid
	}
	if c.Purpose != Purpose || c.MediaID != mediaID || c.UserID == "" || s.now().Unix() >= c.Expires {
		return Claims{}, ErrInvalid
	}
	return c, nil
}
