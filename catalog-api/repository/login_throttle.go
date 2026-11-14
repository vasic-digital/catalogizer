package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"catalogizer/database"
)

// LoginThrottle is the persisted, bounded, progressive back-off for POST /auth/login (CAT-LOGIN-LOCK-1).
//
// Design (the decisions, so a reviewer can attack them):
//
//   - THE INVARIANT (CAT-LOGIN-LOCK-2, structural; every rule below is derived from it). The set of state an attacker can
//     observe about an account (the account block and its end, the number of guesses that get evaluated before the block, the
//     status and Retry-After of every probe) MUST be a function only of attacker-caused events (the failed attempts charged to
//     the account scope, and time), never of an owner SUCCESS event. Then an existing account and a nonexistent name give the
//     same trace to every attacker, whatever the owner does and whenever, and the owner's SUCCESSFUL logins leave no mark to
//     read. Limits of that claim, stated honestly (F3/F6 of the round-1 review): (a) an owner's own WRONG password from an
//     address that is not known-good is a completed failure and is charged to the account scope exactly like a guess, so it
//     is an event the attacker can observe the effect of (it moves the count toward the block); it is rare, but it is not
//     "attacker-caused". (b) The model test (TestInvariant_*) has an alphabet of attacker probes and owner SUCCESSES only: it
//     contains no owner failures and no attacker concurrency, so it proves nothing about those two. (c) An attacker behind
//     the same NAT as a known-good address gets a fresh per-address bucket after every owner login from that address (the
//     owner's Success clears that address's own charges) and can therefore see that the owner logged in; it is bounded by
//     the 5 free attempts per address and needs the shared address.
//     Consequences, each enforced in code and by TestInvariant_* (a model-based test over interleavings):
//     (1) The ACCOUNT scope counts only completed FAILED attempts (Fail), charged after the password was evaluated. A success
//     never writes it: not a charge, not a refund, not a reset, so it can neither end a block early (the CAT-LOGIN-LOCK-1
//     refund did, RV5-1) nor leave a charge behind (R4-1: an owner who logs in from many new addresses with nobody
//     attacking is never charged, so is never locked out).
//     (2) Parallel guessing is bounded by IN-FLIGHT TOKENS (rows with scope "pend:<nonce>", Policy.MaxInflight per subject)
//     instead of by charging before the check. A token is not a charge: it is released by Success or converted into a
//     charge by Fail, so settled state never depends on whether an attempt was the owner's. A token cannot tell owner
//     from attacker before the password is checked, so the cap is a constant that does not depend on the failure count:
//     an owner's token then changes what a SERIAL attacker sees only if the attacker already holds MaxInflight-1
//     concurrent tokens. The cap is enforced atomically: takeToken serialises per subject on a row lock (the "slots" row) and
//     counts tokens by substr(scope,1,5), so it holds on PostgreSQL under READ COMMITTED and under any collation (the first
//     version used a single INSERT..SELECT..WHERE COUNT < cap and a range comparison on scope, which did not: measured on
//     real PostgreSQL 15, evidence docs/qa/login-lock-2-round2-20261114/pg_both_collations.txt). The hand-off from token to
//     charge is atomic too: Fail deletes the token and charges the account in ONE transaction (one atomic commit; Fail takes no
//     slots lock, the commit alone makes the swap invisible half-done), so no instant exists at which an evaluated attempt is
//     counted by neither the cap nor the account counter. EVERY evaluated attempt is charged exactly once, also when it settles
//     on an account that a sibling attempt has just blocked (round-3 review F14: it used to be deleted uncharged, 24 evaluated
//     against 6 charged over 6 block windows, a 4x guess rate); a charge on a blocked account raises the failure count by one
//     and the block end to max(current end, now + the doubling delay of the new count), so the doubling and the Cap bound hold. Cost, stated
//     honestly: evaluated guesses before the first block are at most AccountFree + MaxInflight - 1 (exactly AccountFree for
//     a serial guesser), not exactly AccountFree. Measured on real PostgreSQL 15 with 16 goroutines of Begin/Fail loops: the
//     worst case over 20 rounds was 22 on one image in the author's round-2 run and 23 on both images in the reviewer's run
//     (the bound is 23; the observed maximum depends on timing and is not a constant). Members of the class "an attempt is uncounted by both the cap and the
//     account counter at some instant", each closed or scoped out: Fail (closed: one transaction, charged also on a blocked account); Success (by design: a
//     correct password is not a guess, it is never charged); Begin refusals after the token was taken (closed: the attempt
//     is not evaluated, so nothing is owed); Clear (by design: wipes tokens and counters together; a later Fail of an
//     attempt from before the Clear charges nothing, RV5-2); the TTL purge (it removes only tokens older than PendingTTL,
//     which no longer count); an attempt outliving PendingTTL, a crash, or a Fail transaction that errors (scoped out,
//     accepted: the token keeps counting as in flight until PendingTTL and then the attempt stays uncharged; PendingTTL is
//     2 minutes against a bcrypt check of about 100 ms, and a store error is logged and fails the request closed); a
//     panic or early return in the caller (closed in the caller: the deferred settle turns the token into a charge); a
//     Busy error (Begin: the attempt is refused, not evaluated; Fail: the transaction rolls back, as above).
//     (3) Residual, accepted and documented: an attacker who bursts exactly MaxInflight parallel probes can see one fewer
//     slot while an owner login is in flight (a contention channel that exists only during the bcrypt window and needs
//     a burst); an administrator's Clear (unlock, password change) ends a block of a real account and has no counterpart
//     for a name that does not exist; a known-good address is not stopped by the account block, which only a holder of
//     that address can see.
//   - It is a THROTTLE, never a lock: no block is permanent and every block is at most Policy.Cap (default 15 minutes) long
//     from the moment it is set. It is NOT a bound on how long an attacker can keep an account throttled: an attacker who
//     keeps guessing renews the block each time it expires (one further charged attempt per Cap), so the owner can be held
//     out for as long as the attack lasts, from every address that is not known-good for that account. Known-good addresses
//     (below) are the mitigation; an owner whose address changes (IPv6 privacy addresses, carrier-grade NAT, a new device) is
//     "new" and is held like anyone else. Matching known-good addresses per IPv6 /64 is a recorded follow-up, not done here.
//     The administrative suspension (users.is_locked) is a different mechanism and is not touched here: using it would revoke
//     the victim's sessions on the attacker's say-so.
//   - Two scopes. "account": every client together (stops distributed guessing of one account). "ip:<addr>": one client
//     address against one account (stops one address from using the whole account budget). Plus transient "pend:<nonce>" rows.
//   - Fail-charged, token-gated. An attempt first takes an in-flight token (insert bounded by MaxInflight, serialised per
//     subject by a row lock inside one transaction) and its per-address bucket (only that) is charged by compare-and-swap
//     BEFORE the password is checked; the ACCOUNT charge
//     is made by Fail, after the check, by compare-and-swap on a version column. Success releases the token and clears that
//     client address's own bucket; it never writes the account scope.
//   - Known-good client. A client address that completed a login for this account within KnownGoodTTL (at most MaxKnownGood
//     addresses per account are remembered) is not stopped by the ACCOUNT block, so an attacker cannot lock the owner out of
//     the places the owner really logs in from. It is still subject to its own per-address block, so spoofing a known address
//     buys the attacker only that one address's budget, never an unbounded one.
//   - Unknown names are throttled exactly like known ones (subject "n:<hash of the EXACT submitted string>"), so the answer
//     does not reveal whether a name exists. The subject MUST be derived with the same equality the user lookup uses
//     (UserRepository.GetByUsernameOrEmail matches username or email byte for byte), otherwise a normalisation difference
//     (case folding, trimming) is an existence oracle: a nonexistent "Victim" would share a budget with "victim" while an
//     existing "victim" would not. Rows are bounded only by time: at most 2 + A persistent rows per distinct submitted string, where A is the number of distinct client addresses that tried it (the account row, the "slots" row that serialises the in-flight cap, and one ip:<addr> row per address), plus at most MaxInflight transient tokens, purged after
//     StaleAfter (rows that carry a known-good marker live up to KnownGoodTTL), purged in batches. There is no row cap, and the
//     per-address request limiter does not bound the number of distinct strings under address rotation (IPv6 /64, botnet):
//     table growth under such an attack is a recorded follow-up.
//   - The back-off is charged per account, so a username and the e-mail address of the same account share one budget. A
//     client that submits both names therefore learns that they belong to the same account once the budget is spent. That
//     linkage is accepted and documented (it needs the attacker to already know both names).
//   - Times are unix milliseconds from an injectable clock; state lives in the login_throttle table (migration v29) so it
//     survives a restart and is shared by every instance on one database.
type LoginThrottle struct {
	// testAfterAccountRead, when set (tests only), runs between Begin's read of the account row and its reserve, so a test can
	// interleave another request deterministically.
	testAfterAccountRead func()
	db                   *database.DB
	mu                   sync.Mutex
	pol                  LoginThrottlePolicy
	now                  func() time.Time
	purge                time.Time
}

// LoginThrottlePolicy holds the numbers. Zero fields fall back to the defaults of DefaultLoginThrottlePolicy.
type LoginThrottlePolicy struct {
	// BucketFree charged attempts per (account, address) before the first block; AccountFree the same for all addresses together.
	BucketFree, AccountFree int
	// Base is the first block length; each further charged attempt doubles it up to Cap.
	Base, Cap time.Duration
	// BucketDecay / AccountDecay: a quiet period after which the failure count restarts from zero.
	BucketDecay, AccountDecay time.Duration
	KnownGoodTTL              time.Duration
	MaxKnownGood              int
	StaleAfter                time.Duration
	// MaxInflight is the most attempts per subject that may be between Begin and Success/Fail at once (a constant, see the
	// invariant). PendingTTL: a token older than this no longer counts and is purged (a crashed attempt is not charged).
	MaxInflight int
	PendingTTL  time.Duration
}

// DefaultLoginThrottlePolicy: 5 free attempts per address and 20 per account, then 30s, 60s, ... up to 15 minutes.
func DefaultLoginThrottlePolicy() LoginThrottlePolicy {
	return LoginThrottlePolicy{
		BucketFree: 5, AccountFree: 20,
		Base: 30 * time.Second, Cap: 15 * time.Minute,
		BucketDecay: time.Hour, AccountDecay: 24 * time.Hour,
		KnownGoodTTL: 30 * 24 * time.Hour, MaxKnownGood: 5,
		StaleAfter:  24 * time.Hour,
		MaxInflight: 4, PendingTTL: 2 * time.Minute,
	}
}

func (p LoginThrottlePolicy) withDefaults() LoginThrottlePolicy {
	d := DefaultLoginThrottlePolicy()
	if p.BucketFree <= 0 {
		p.BucketFree = d.BucketFree
	}
	if p.AccountFree <= 0 {
		p.AccountFree = d.AccountFree
	}
	if p.Base <= 0 {
		p.Base = d.Base
	}
	if p.Cap <= 0 {
		p.Cap = d.Cap
	}
	if p.Cap < p.Base {
		p.Cap = p.Base
	}
	if p.BucketDecay <= 0 {
		p.BucketDecay = d.BucketDecay
	}
	if p.AccountDecay <= 0 {
		p.AccountDecay = d.AccountDecay
	}
	if p.KnownGoodTTL <= 0 {
		p.KnownGoodTTL = d.KnownGoodTTL
	}
	if p.MaxKnownGood <= 0 {
		p.MaxKnownGood = d.MaxKnownGood
	}
	if p.StaleAfter <= 0 {
		p.StaleAfter = d.StaleAfter
	}
	if p.MaxInflight <= 0 {
		p.MaxInflight = d.MaxInflight
	}
	if p.PendingTTL <= 0 {
		p.PendingTTL = d.PendingTTL
	}
	// A row older than StaleAfter is purged, so an account failure count can never outlive it: the effective account window is
	// min(AccountDecay, StaleAfter). Make that the configured value instead of a silent coupling.
	if p.AccountDecay > p.StaleAfter {
		p.AccountDecay = p.StaleAfter
	}
	return p
}

// ErrLoginThrottleBusy is returned when the compare-and-swap could not settle (extreme contention); callers fail closed.
var ErrLoginThrottleBusy = errors.New("login throttle busy")

const (
	scopeAccount = "account"
	casRetries   = 400
	purgeBatch   = 2000
)

// NewLoginThrottle builds the throttle over db with the default policy and the wall clock.
func NewLoginThrottle(db *database.DB) *LoginThrottle {
	return &LoginThrottle{db: db, pol: DefaultLoginThrottlePolicy(), now: time.Now}
}

// SetClock injects the clock (tests). SetPolicy replaces the policy (zero fields keep the defaults).
func (t *LoginThrottle) SetClock(now func() time.Time) { t.mu.Lock(); t.now = now; t.mu.Unlock() }
func (t *LoginThrottle) SetPolicy(p LoginThrottlePolicy) {
	t.mu.Lock()
	t.pol = p.withDefaults()
	t.mu.Unlock()
}

func (t *LoginThrottle) clock() (time.Time, LoginThrottlePolicy) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.now(), t.pol
}

// SubjectForUser / SubjectForUnknownName build the subject key. A name that matches no account is hashed, so the table
// never stores a raw attacker-chosen string. The hash covers the EXACT submitted bytes: no case folding, no trimming, no
// Unicode normalisation, because the user lookup compares exactly and any difference here would be an existence oracle.
func SubjectForUser(userID int) string { return fmt.Sprintf("u:%d", userID) }
func SubjectForUnknownName(name string) string {
	sum := sha256.Sum256([]byte(name))
	return "n:" + hex.EncodeToString(sum[:16])
}

func ipScope(ip string) string {
	if len(ip) > 64 {
		ip = ip[:64]
	}
	return "ip:" + ip
}

// Decision is the outcome of Begin.
type Decision struct {
	Allowed bool
	// RetryAfter is how long the caller must wait when Allowed is false.
	RetryAfter time.Duration
	// Attempt is the in-flight token of an allowed attempt that is not from a known-good address (zero otherwise). Pass it to
	// exactly one of Success or Fail.
	Attempt AttemptToken
}

// AttemptToken names one in-flight attempt: the subject it was taken for, a random nonce (so a recreated row is never confused
// with an earlier one, RV5-2) and the row's scope. It carries no state to restore, so settling it can never write anything an
// attacker could read except by Fail's charge.
type AttemptToken struct {
	held    bool
	subject string
	scope   string
}

// ErrAttemptSubject is returned when a token is settled for a different subject than it was taken for (RV5-4).
var ErrAttemptSubject = errors.New("login throttle: attempt token belongs to another subject")

// pendPrefix marks an in-flight token row; its length is the 5 in the substr(scope, 1, 5) tests. slotsScope is the per-subject
// row whose lock serialises takeToken.
const (
	pendPrefix = "pend:"
	slotsScope = "slots"
)

func newAttemptToken(subject string) (AttemptToken, error) {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		return AttemptToken{}, err
	}
	return AttemptToken{held: true, subject: subject, scope: pendPrefix + hex.EncodeToString(b[:])}, nil
}

func delayFor(f, free int, base, cap time.Duration) time.Duration {
	exp := f - free
	if exp < 0 {
		return 0
	}
	if exp > 20 {
		exp = 20
	}
	d := base << uint(exp)
	if d > cap || d <= 0 {
		d = cap
	}
	return d
}

// querier is what read and reserve need; *database.DB satisfies it (with the dialect rewrite) and so does txQ, so the same code
// charges a scope inside a transaction (Fail) or on its own (Begin's address bucket).
type querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// txQ runs statements on a transaction with the dialect rewrite (statements on a *sql.Tx bypass the DB wrappers).
type txQ struct {
	tx *sql.Tx
	db *database.DB
}

func (q txQ) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	return q.tx.ExecContext(ctx, q.db.Rewrite(query), args...)
}
func (q txQ) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	return q.tx.QueryRowContext(ctx, q.db.Rewrite(query), args...)
}

type rowState struct {
	failures       int
	blockedUntilMs int64
	lastFailureMs  int64
	lastSuccessMs  int64
	version        int64
	exists         bool
}

func (t *LoginThrottle) read(ctx context.Context, q querier, subject, scope string) (rowState, error) {
	var s rowState
	err := q.QueryRowContext(ctx,
		`SELECT failures, blocked_until_ms, last_failure_ms, last_success_ms, version FROM login_throttle WHERE subject = ? AND scope = ?`,
		subject, scope).Scan(&s.failures, &s.blockedUntilMs, &s.lastFailureMs, &s.lastSuccessMs, &s.version)
	if errors.Is(err, sql.ErrNoRows) {
		return rowState{}, nil
	}
	if err != nil {
		return rowState{}, err
	}
	s.exists = true
	return s, nil
}

// reserve charges one attempt on (subject, scope) atomically. until is the end of the block when refused (the row is blocked
// and nothing is charged), unless chargeBlocked is set: Fail charges a row that is already blocked too (the attempt was evaluated,
// so it is owed one charge, F14), growing the failure count by one and the block end to max(current end, now + the doubling
// delay of the new count), so a block is never shortened and never longer than Cap from now.
func (t *LoginThrottle) reserve(ctx context.Context, q querier, subject, scope string, free int, decay time.Duration, now time.Time, pol LoginThrottlePolicy, chargeBlocked bool) (allowed bool, until time.Time, err error) {
	nowMs := now.UnixMilli()
	for i := 0; i < casRetries; i++ {
		if _, err = q.ExecContext(ctx,
			`INSERT OR IGNORE INTO login_throttle (subject, scope) VALUES (?, ?)`, subject, scope); err != nil {
			return false, time.Time{}, err
		}
		s, rerr := t.read(ctx, q, subject, scope)
		if rerr != nil {
			return false, time.Time{}, rerr
		}
		if s.blockedUntilMs > nowMs && !chargeBlocked {
			return false, time.UnixMilli(s.blockedUntilMs), nil
		}
		f := s.failures
		if f > 0 && nowMs-s.lastFailureMs > decay.Milliseconds() {
			f = 0
		}
		f++
		var nb int64
		if f >= free {
			nb = nowMs + delayFor(f, free, pol.Base, pol.Cap).Milliseconds()
		}
		if chargeBlocked && s.blockedUntilMs > nb {
			nb = s.blockedUntilMs // never shorten an existing block
		}
		res, uerr := q.ExecContext(ctx,
			`UPDATE login_throttle SET failures = ?, blocked_until_ms = ?, last_failure_ms = ?, version = version + 1 WHERE subject = ? AND scope = ? AND version = ?`,
			f, nb, nowMs, subject, scope, s.version)
		if uerr != nil {
			return false, time.Time{}, uerr
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return true, time.Time{}, nil
		}
		runtime.Gosched()
	}
	return false, time.Time{}, ErrLoginThrottleBusy
}

// lockSlots takes the per-subject row lock that serialises the in-flight cap against every other Begin of the subject
// (takeToken: count-then-insert). Writing the row is what takes the lock.
func (t *LoginThrottle) lockSlots(ctx context.Context, tx *sql.Tx, subject string, nowMs int64) error {
	for i := 0; i < 3; i++ {
		if _, err := tx.ExecContext(ctx, t.db.Rewrite(`INSERT OR IGNORE INTO login_throttle (subject, scope) VALUES (?, ?)`), subject, slotsScope); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, t.db.Rewrite(`UPDATE login_throttle SET last_failure_ms = ?, version = version + 1 WHERE subject = ? AND scope = ?`), nowMs, subject, slotsScope)
		if err != nil {
			return err
		}
		// Zero rows means a concurrent purge removed the row between the two statements: try again.
		if n, _ := res.RowsAffected(); n == 1 {
			return nil
		}
	}
	return ErrLoginThrottleBusy
}

// takeToken inserts an in-flight token unless the subject already has MaxInflight live ones. The bound is serialised per subject
// by a row lock: inside one transaction the per-subject "slots" row is written first (a write takes its row lock on SQLite and,
// on PostgreSQL, the row lock that makes a concurrent transaction wait until this one commits), and only then are the live
// tokens counted and the new one inserted. A plain "INSERT .. SELECT .. WHERE (SELECT COUNT(*) ..) < cap" is NOT enough on
// PostgreSQL: under READ COMMITTED two concurrent statements both count before either inserts (measured by the round-1 reviewer on
// postgres:15-alpine: 40 concurrent Begins allowed 6 to 37 against a cap of 4; that review's own run logs are not in this repository). The tokens are recognised by substr(scope, 1, 5) = 'pend:', never by a
// range comparison on scope: a range depends on the database collation (it is empty under glibc en_US.utf8, so the cap never
// applied there).
func (t *LoginThrottle) takeToken(ctx context.Context, tok AttemptToken, now time.Time, pol LoginThrottlePolicy) (bool, error) {
	nowMs := now.UnixMilli()
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	if err = t.lockSlots(ctx, tx, tok.subject, nowMs); err != nil {
		return false, err
	}
	var live int
	if err = tx.QueryRowContext(ctx, t.db.Rewrite(`SELECT COUNT(*) FROM login_throttle WHERE subject = ? AND substr(scope, 1, 5) = ? AND last_failure_ms > ?`),
		tok.subject, pendPrefix, nowMs-pol.PendingTTL.Milliseconds()).Scan(&live); err != nil {
		return false, err
	}
	if live >= pol.MaxInflight {
		return false, nil
	}
	if _, err = tx.ExecContext(ctx, t.db.Rewrite(`INSERT INTO login_throttle (subject, scope, last_failure_ms) VALUES (?, ?, ?)`), tok.subject, tok.scope, nowMs); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// dropToken deletes the token row; true when this call removed it (false: already settled, expired and purged, or cleared).
func (t *LoginThrottle) dropToken(ctx context.Context, tok AttemptToken) (bool, error) {
	res, err := t.db.ExecContext(ctx, `DELETE FROM login_throttle WHERE subject = ? AND scope = ?`, tok.subject, tok.scope)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// Begin decides whether this attempt may be evaluated. It takes an in-flight token (unless the address is known-good) and charges
// only the address bucket; the account scope is charged later, by Fail. It must be called BEFORE the password is checked.
func (t *LoginThrottle) Begin(ctx context.Context, subject, ip string) (Decision, error) {
	now, pol := t.clock()
	nowMs := now.UnixMilli()
	t.maybePurge(ctx, now, pol)
	bucket := ipScope(ip)

	// 1. This address's own block (read-only, so a refused attempt is not charged anywhere).
	b, err := t.read(ctx, t.db, subject, bucket)
	if err != nil {
		return Decision{}, err
	}
	if b.blockedUntilMs > nowMs {
		return refuse(time.UnixMilli(b.blockedUntilMs), now), nil
	}
	known := b.lastSuccessMs > 0 && nowMs-b.lastSuccessMs <= pol.KnownGoodTTL.Milliseconds()

	// 2. The account block, unless this is a known-good address.
	a, err := t.read(ctx, t.db, subject, scopeAccount)
	if err != nil {
		return Decision{}, err
	}
	if t.testAfterAccountRead != nil {
		t.testAfterAccountRead()
	}
	if a.blockedUntilMs > nowMs && !known {
		return refuse(time.UnixMilli(a.blockedUntilMs), now), nil
	}
	// A known-good address takes no token and is never charged to the account scope, by Begin or by Fail (its own bucket still
	// limits it): it is the one client the account block does not stop, and only a holder of that address can see that.
	var tok AttemptToken
	if !known {
		tok, err = newAttemptToken(subject)
		if err != nil {
			return Decision{}, err
		}
		got, err := t.takeToken(ctx, tok, now, pol)
		if errors.Is(err, ErrLoginThrottleBusy) {
			return refuse(now.Add(pol.Base), now), nil // same answer as a Busy address charge below
		}
		if err != nil {
			return Decision{}, err
		}
		if !got {
			return refuse(now.Add(pol.Base), now), nil
		}
		// The account may have been blocked by a Fail that landed between the read above and the token: look again.
		a2, err := t.read(ctx, t.db, subject, scopeAccount)
		if err != nil {
			_, _ = t.dropToken(ctx, tok)
			return Decision{}, err
		}
		if a2.blockedUntilMs > nowMs {
			_, _ = t.dropToken(ctx, tok)
			return refuse(time.UnixMilli(a2.blockedUntilMs), now), nil
		}
	}

	// 3. This address's charge (charge-first: it is per address, so an owner's success clearing it is not visible to an
	// attacker, who uses other addresses).
	ok, until, err := t.reserve(ctx, t.db, subject, bucket, pol.BucketFree, pol.BucketDecay, now, pol, false)
	if errors.Is(err, ErrLoginThrottleBusy) {
		_, _ = t.dropToken(ctx, tok)
		return refuse(now.Add(pol.Base), now), nil
	}
	if err != nil {
		_, _ = t.dropToken(ctx, tok)
		return Decision{}, err
	}
	if !ok {
		_, _ = t.dropToken(ctx, tok)
		return refuse(until, now), nil
	}
	return Decision{Allowed: true, Attempt: tok}, nil
}

func refuse(until, now time.Time) Decision {
	d := until.Sub(now)
	if d < time.Second {
		d = time.Second
	}
	return Decision{Allowed: false, RetryAfter: d}
}

// Fail settles an allowed attempt whose password was wrong (or that ended without a correct password): the token becomes ONE
// charge on the account scope, in ONE transaction: the token is deleted and the account is charged in the same commit, so no
// other attempt (whose Begin counts only committed tokens, under the slots lock) can ever see the token gone and the charge not
// yet made, i.e. at no instant is the attempt counted by neither the in-flight cap nor the account counter. Fail takes no slots
// lock itself: atomic commit is enough, and holding it would only add a lock-order hazard against Clear (round-2 review F8: with the two steps apart, a new attempt could take the freed slot in the gap, and the evaluated-
// before-block count was 24, or 35 with delayed charges, against the documented 23). A token that is no longer there (settled,
// removed by Clear, or purged after PendingTTL) charges nothing: after Clear that is required (RV5-2); after PendingTTL it is
// the documented residual (an attempt that outlives PendingTTL is not charged; PendingTTL is 2 minutes against a bcrypt check of
// about 100 ms, so only a stalled or crashed attempt gets there). An account that is already blocked IS charged (F14): the
// attempt was evaluated, so it is owed one charge; the failure count grows by one, the block end becomes max(current end, now +
// doubling delay of the new count), never longer than Cap from now. If the transaction fails (store error, ErrLoginThrottleBusy) nothing is
// changed: the token stays and counts as in flight until PendingTTL, then expires uncharged (store failure, logged by the caller).
func (t *LoginThrottle) Fail(ctx context.Context, subject string, tok AttemptToken) error {
	if !tok.held {
		return nil
	}
	if tok.subject != subject {
		return ErrAttemptSubject
	}
	now, pol := t.clock()
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := txQ{tx: tx, db: t.db}
	res, err := q.ExecContext(ctx, `DELETE FROM login_throttle WHERE subject = ? AND scope = ?`, tok.subject, tok.scope)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return tx.Commit()
	}
	if _, _, err = t.reserve(ctx, q, subject, scopeAccount, pol.AccountFree, pol.AccountDecay, now, pol, true); err != nil {
		return err
	}
	return tx.Commit()
}

// Success releases an allowed attempt's token and clears this address's own charges (never the account scope: see the
// invariant) of a completed login; with rememberClient it also records the address as known-good.
func (t *LoginThrottle) Success(ctx context.Context, subject, ip string, rememberClient bool, tok AttemptToken) error {
	now, pol := t.clock()
	nowMs := now.UnixMilli()
	if tok.held {
		if tok.subject != subject {
			return ErrAttemptSubject
		}
		if _, err := t.dropToken(ctx, tok); err != nil {
			return err
		}
	}
	bucket := ipScope(ip)
	if !rememberClient {
		_, err := t.db.ExecContext(ctx,
			`UPDATE login_throttle SET failures = 0, blocked_until_ms = 0, version = version + 1 WHERE subject = ? AND scope = ?`,
			subject, bucket)
		return err
	}
	if _, err := t.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO login_throttle (subject, scope) VALUES (?, ?)`, subject, bucket); err != nil {
		return err
	}
	if _, err := t.db.ExecContext(ctx,
		`UPDATE login_throttle SET failures = 0, blocked_until_ms = 0, last_success_ms = ?, version = version + 1 WHERE subject = ? AND scope = ?`,
		nowMs, subject, bucket); err != nil {
		return err
	}
	// Remember at most MaxKnownGood addresses: drop the older success markers (rows that only carry failures are kept).
	_, err := t.db.ExecContext(ctx,
		`DELETE FROM login_throttle WHERE subject = ? AND scope <> ? AND last_success_ms > 0 AND scope NOT IN (
			SELECT scope FROM (SELECT scope FROM login_throttle WHERE subject = ? AND scope <> ? AND last_success_ms > 0 ORDER BY last_success_ms DESC LIMIT ?) k)`,
		subject, scopeAccount, subject, scopeAccount, pol.MaxKnownGood)
	return err
}

// Clear removes every throttle row of an account (administrator unlock, password change or reset). It is a single DELETE and takes
// no slots lock: on PostgreSQL a token inserted by a Begin that is committing at the same moment can survive the DELETE (the
// DELETE sees only rows committed before its statement). Such an attempt is then settled normally and charges the account after
// the Clear; that is one counted attempt in the new state, not a loss of the cap. A Fail racing a Clear can fail with a
// store error on PostgreSQL (both write the same rows, a deadlock aborts one of them: an administrator's unlock or password
// change can therefore return an error and must be retried; the caller of Fail logs its error and the request is refused).
// Across instances sharing one database the cap and the purge compare tokens' timestamps with each instance's own clock: clocks
// that differ by more than PendingTTL make the cap count too many or too few tokens and the purge remove live ones.
func (t *LoginThrottle) Clear(ctx context.Context, userID int) error {
	_, err := t.db.ExecContext(ctx, `DELETE FROM login_throttle WHERE subject = ?`, SubjectForUser(userID))
	return err
}

// maybePurge deletes rows that can no longer matter, at most once a minute per process.
func (t *LoginThrottle) maybePurge(ctx context.Context, now time.Time, pol LoginThrottlePolicy) {
	t.mu.Lock()
	if now.Sub(t.purge) < time.Minute {
		t.mu.Unlock()
		return
	}
	t.purge = now
	t.mu.Unlock()
	nowMs := now.UnixMilli()
	_, _ = t.db.ExecContext(ctx, `DELETE FROM login_throttle WHERE substr(scope, 1, 5) = ? AND last_failure_ms < ?`, pendPrefix, nowMs-pol.PendingTTL.Milliseconds())
	// Bounded work inside a login request: at most purgeBatch rows per run (the rest go on the next run, a minute later).
	_, _ = t.db.ExecContext(ctx,
		`DELETE FROM login_throttle WHERE (subject, scope) IN (
			SELECT subject, scope FROM login_throttle WHERE blocked_until_ms < ? AND last_failure_ms < ? AND (last_success_ms = 0 OR last_success_ms < ?) LIMIT ?)`,
		nowMs, nowMs-pol.StaleAfter.Milliseconds(), nowMs-pol.KnownGoodTTL.Milliseconds(), purgeBatch)
}
