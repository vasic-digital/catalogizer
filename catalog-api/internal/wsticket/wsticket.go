// Package wsticket issues and redeems short-lived, single-use WebSocket tickets (SEC-WS-1 F2).
//
// A browser WebSocket cannot send an Authorization header, so the web client used to put the session JWT in the /ws query
// string, where it leaks into proxy and server logs and browser history. A ticket is the narrow replacement: 32 random bytes,
// valid for TTL, bound to ONE user (and the session that minted it, so logout revokes it), redeemable exactly ONCE, and only
// the SHA-256 hash is kept server side, so a memory or log disclosure of the store yields nothing replayable.
package wsticket

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
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
	// MaxPerUser bounds the live tickets ONE user can hold, so a single (even legitimate) account cannot exhaust maxLive and
	// lock every other user out of /ws. A client mints one ticket per (re)connect and redeems it within seconds.
	MaxPerUser = 8
	// MintsPerWindow is the per-user rate limit on POST /api/v1/ws/ticket.
	MintsPerWindow = 20
	MintWindow     = time.Minute
)

// ErrInvalid is returned for every redemption failure (unknown, used, expired, malformed); it carries no detail.
var ErrInvalid = errors.New("invalid websocket ticket")

// ErrBusy is returned by Issue when too many unredeemed tickets are live.
var ErrBusy = errors.New("too many pending websocket tickets")

// ErrUserBusy is returned by Issue when this user already holds MaxPerUser unredeemed tickets.
var ErrUserBusy = errors.New("too many pending websocket tickets for this user")

// Claims are what a redeemed ticket authenticates.
type Claims struct {
	UserID    string
	Username  string
	SessionID string
	RoleID    int
	// TokenExpiry is the expiry of the session token that minted the ticket (zero when unknown); /ws closes the socket then,
	// exactly like a Bearer socket.
	TokenExpiry time.Time
}

type entry struct {
	claims  Claims
	expires time.Time
}

// queued records issue order. The TTL is constant, so expiry order equals issue order and expired entries are always at the
// front of the queue: the purge pops from the front and stops at the first live one (amortised O(1) per Issue).
type queued struct {
	hash    [sha256.Size]byte
	expires time.Time
}

// Store holds unredeemed tickets by hash.
type Store struct {
	mu      sync.Mutex
	live    map[[sha256.Size]byte]entry
	order   []queued // order[head:] is the live queue; order[:head] are already-purged slots awaiting compaction
	head    int
	perUser map[string]int
	now     func() time.Time
	ttl     time.Duration
	scanned int // queue entries examined by purge (test introspection)
	moved   int // queue elements copied by compaction (test introspection)
}

// NewStore returns an empty store using the production TTL and clock.
func NewStore() *Store {
	return &Store{live: map[[sha256.Size]byte]entry{}, perUser: map[string]int{}, now: time.Now, ttl: TTL}
}

// WithClock replaces the clock (tests).
func (s *Store) WithClock(now func() time.Time) *Store { s.now = now; return s }

func hashOf(ticket string) [sha256.Size]byte { return sha256.Sum256([]byte(ticket)) }

// drop removes a live entry and releases its per-user slot. Caller holds s.mu.
func (s *Store) drop(h [sha256.Size]byte) {
	e, ok := s.live[h]
	if !ok {
		return
	}
	delete(s.live, h)
	if n := s.perUser[e.claims.UserID]; n <= 1 {
		delete(s.perUser, e.claims.UserID)
	} else {
		s.perUser[e.claims.UserID] = n - 1
	}
}

// compactMin is the smallest purged prefix worth reclaiming; below it the dead slots are left in place.
const compactMin = 256

// purge drops expired tickets from the front of the issue queue by advancing a head index (no copying). The dead prefix is
// reclaimed only when it is at least compactMin slots AND at least as long as the live remainder, so each element is moved
// O(1) times amortised. Caller holds s.mu.
func (s *Store) purge(now time.Time) {
	for s.head < len(s.order) {
		s.scanned++
		q := s.order[s.head]
		if now.Before(q.expires) {
			break
		}
		if e, ok := s.live[q.hash]; ok && !now.Before(e.expires) {
			s.drop(q.hash)
		}
		s.order[s.head] = queued{} // do not retain the hash in the dead prefix
		s.head++
	}
	if s.head >= compactMin && s.head >= len(s.order)-s.head {
		n := len(s.order) - s.head
		s.moved += n
		copy(s.order, s.order[s.head:])
		s.order = s.order[:n]
		s.head = 0
	}
}

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
	s.purge(now)
	if s.perUser[c.UserID] >= MaxPerUser {
		return "", time.Time{}, ErrUserBusy
	}
	if len(s.live) >= maxLive {
		return "", time.Time{}, ErrBusy
	}
	h := hashOf(ticket)
	s.live[h] = entry{claims: c, expires: exp}
	s.order = append(s.order, queued{hash: h, expires: exp})
	s.perUser[c.UserID]++
	return ticket, exp, nil
}

// Redeem consumes the ticket: it succeeds at most once, and only before expiry. Lookup is by SHA-256 of the presented value
// (a map hit on a 256-bit hash leaks nothing useful about the stored secret through timing).
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
	s.drop(h) // single use: consumed even when the expiry check below refuses it
	if !s.now().Before(e.expires) {
		return Claims{}, ErrInvalid
	}
	return e.claims, nil
}

// LenForTest returns the number of live tickets.
func (s *Store) LenForTest() int { s.mu.Lock(); defer s.mu.Unlock(); return len(s.live) }

// ScannedForTest returns how many queue entries purge has examined in total.
func (s *Store) ScannedForTest() int { s.mu.Lock(); defer s.mu.Unlock(); return s.scanned }

// MovedForTest returns how many queue elements compaction has copied in total.
func (s *Store) MovedForTest() int { s.mu.Lock(); defer s.mu.Unlock(); return s.moved }

// ContainsPlaintextForTest reports whether the plaintext ticket (its text or its decoded 32 raw bytes) appears as a stored
// key, or inside any stored claim string. A hash-only store never does.
func (s *Store) ContainsPlaintextForTest(ticket string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := base64.RawURLEncoding.DecodeString(ticket)
	for k, e := range s.live {
		if len(raw) > 0 && bytes.Equal(k[:], raw) {
			return true
		}
		if bytes.Contains(k[:], []byte(ticket)) {
			return true
		}
		if e.claims.UserID == ticket || e.claims.Username == ticket || e.claims.SessionID == ticket {
			return true
		}
	}
	return false
}

// Limiter is a small per-key sliding-window rate limiter for the mint route.
type Limiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	hits   map[string][]time.Time
	sweep  time.Time
	now    func() time.Time
}

// NewLimiter allows max calls per key per window.
func NewLimiter(max int, window time.Duration) *Limiter {
	return &Limiter{max: max, window: window, hits: map[string][]time.Time{}, now: time.Now}
}

// WithClock replaces the clock (tests).
func (l *Limiter) WithClock(now func() time.Time) *Limiter { l.now = now; return l }

// Allow records a call for key; when over the limit it returns false and how long until a slot frees.
func (l *Limiter) Allow(key string) (bool, time.Duration) {
	now := l.now()
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.sweep) >= l.window { // at most one idle-key sweep per window
		for k, ts := range l.hits {
			if len(ts) == 0 || !ts[len(ts)-1].After(now.Add(-l.window)) {
				delete(l.hits, k)
			}
		}
		l.sweep = now
	}
	cutoff := now.Add(-l.window)
	ts := l.hits[key]
	i := 0
	for i < len(ts) && !ts[i].After(cutoff) {
		i++
	}
	ts = ts[i:]
	if len(ts) >= l.max {
		l.hits[key] = ts
		return false, ts[0].Add(l.window).Sub(now)
	}
	l.hits[key] = append(ts, now)
	return true, 0
}
