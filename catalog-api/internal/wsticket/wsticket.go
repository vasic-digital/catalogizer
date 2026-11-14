// Package wsticket issues and redeems short-lived, single-use WebSocket tickets (SEC-WS-1 F2).
//
// A browser WebSocket cannot send an Authorization header, so the web client used to put the session JWT in the /ws query
// string, where it leaks into proxy and server logs and browser history. A ticket is the narrow replacement: 32 random bytes,
// valid for TTL, bound to ONE user (and the session that minted it, so logout revokes it), redeemable exactly ONCE, and only
// the SHA-256 hash is kept server side, so a memory or log disclosure of the store yields nothing replayable.
package wsticket

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"sync"
	"time"
)

const (
	// TTL is how long a ticket can be redeemed after it was issued.
	TTL = 30 * time.Second
	// maxLive bounds memory: a flood of mint calls cannot grow the store without limit.
	maxLive = 10000
)

// ErrInvalid is returned for every redemption failure (unknown, used, expired, malformed); it carries no detail.
var ErrInvalid = errors.New("invalid websocket ticket")

// ErrBusy is returned by Issue when too many unredeemed tickets are live.
var ErrBusy = errors.New("too many pending websocket tickets")

// Claims are what a redeemed ticket authenticates.
type Claims struct {
	UserID    string
	Username  string
	SessionID string
	RoleID    int
}

type entry struct {
	hash    [sha256.Size]byte
	claims  Claims
	expires time.Time
}

// Store holds unredeemed tickets by hash.
type Store struct {
	mu   sync.Mutex
	live map[[sha256.Size]byte]entry
	now  func() time.Time
	ttl  time.Duration
}

// NewStore returns an empty store using the production TTL and clock.
func NewStore() *Store {
	return &Store{live: map[[sha256.Size]byte]entry{}, now: time.Now, ttl: TTL}
}

// WithClock replaces the clock (tests).
func (s *Store) WithClock(now func() time.Time) *Store { s.now = now; return s }

func hashOf(ticket string) [sha256.Size]byte { return sha256.Sum256([]byte(ticket)) }

// Issue mints a ticket for the given identity and returns it with its expiry. Only its hash is stored.
func (s *Store) Issue(c Claims) (string, time.Time, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", time.Time{}, err
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw[:])
	now := s.now()
	exp := now.Add(s.ttl)
	s.mu.Lock()
	defer s.mu.Unlock()
	for h, e := range s.live { // opportunistic purge of expired entries
		if !now.Before(e.expires) {
			delete(s.live, h)
		}
	}
	if len(s.live) >= maxLive {
		return "", time.Time{}, ErrBusy
	}
	h := hashOf(ticket)
	s.live[h] = entry{hash: h, claims: c, expires: exp}
	return ticket, exp, nil
}

// Redeem consumes the ticket: it succeeds at most once, and only before expiry. The stored hash is compared in constant time.
func (s *Store) Redeem(ticket string) (Claims, error) {
	if ticket == "" || len(ticket) > 128 {
		return Claims{}, ErrInvalid
	}
	h := hashOf(ticket)
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.live[h]
	if !ok {
		return Claims{}, ErrInvalid
	}
	delete(s.live, h) // single use: consumed even when the checks below refuse it
	if subtle.ConstantTimeCompare(e.hash[:], h[:]) != 1 || !s.now().Before(e.expires) {
		return Claims{}, ErrInvalid
	}
	return e.claims, nil
}

// ContainsPlaintextForTest reports whether any stored key or value equals the plaintext ticket bytes (it must never).
func (s *Store) ContainsPlaintextForTest(ticket string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, e := range s.live {
		if string(k[:]) == ticket || string(e.hash[:]) == ticket {
			return true
		}
	}
	return false
}
