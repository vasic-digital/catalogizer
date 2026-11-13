package repository

import (
	"context"
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
//   - It is a THROTTLE, never a lock: no block is permanent and every block is at most Policy.Cap (default 15 minutes) long
//     from the moment it is set. It is NOT a bound on how long an attacker can keep an account throttled: an attacker who
//     keeps guessing renews the block each time it expires (one further charged attempt per Cap), so the owner can be held
//     out for as long as the attack lasts, from every address that is not known-good for that account. Known-good addresses
//     (below) are the mitigation; an owner whose address changes (IPv6 privacy addresses, carrier-grade NAT, a new device) is
//     "new" and is held like anyone else. Matching known-good addresses per IPv6 /64 is a recorded follow-up, not done here.
//     The administrative suspension (users.is_locked) is a different mechanism and is not touched here: using it would revoke
//     the victim's sessions on the attacker's say-so.
//   - Two scopes. "account": every client together (stops distributed guessing of one account). "ip:<addr>": one client
//     address against one account (stops one address from using the whole account budget).
//   - Charge-first. An attempt is charged ATOMICALLY (compare-and-swap on a version column, no read-modify-write window)
//     BEFORE the password is checked, so N parallel attempts cannot all slip through before the first one is counted: exactly
//     Free evaluations happen before the block. A success gives back its own account charge (restoring the row as it was,
//     by compare-and-swap, never an earlier charge) and clears that client address's own bucket; it never resets the account
//     scope, because an account block that ended on the owner's success would tell a real account from a nonexistent name.
//     Giving the charge back is what keeps an owner who logs in from many new addresses from being locked out with no attacker.
//   - Known-good client. A client address that completed a login for this account within KnownGoodTTL (at most MaxKnownGood
//     addresses per account are remembered) is not stopped by the ACCOUNT block, so an attacker cannot lock the owner out of
//     the places the owner really logs in from. It is still subject to its own per-address block, so spoofing a known address
//     buys the attacker only that one address's budget, never an unbounded one.
//   - Unknown names are throttled exactly like known ones (subject "n:<hash of the EXACT submitted string>"), so the answer
//     does not reveal whether a name exists. The subject MUST be derived with the same equality the user lookup uses
//     (UserRepository.GetByUsernameOrEmail matches username or email byte for byte), otherwise a normalisation difference
//     (case folding, trimming) is an existence oracle: a nonexistent "Victim" would share a budget with "victim" while an
//     existing "victim" would not. Rows are bounded only by time: at most two per distinct submitted string, purged after
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
}

// DefaultLoginThrottlePolicy: 5 free attempts per address and 20 per account, then 30s, 60s, ... up to 15 minutes.
func DefaultLoginThrottlePolicy() LoginThrottlePolicy {
	return LoginThrottlePolicy{
		BucketFree: 5, AccountFree: 20,
		Base: 30 * time.Second, Cap: 15 * time.Minute,
		BucketDecay: time.Hour, AccountDecay: 24 * time.Hour,
		KnownGoodTTL: 30 * 24 * time.Hour, MaxKnownGood: 5,
		StaleAfter: 24 * time.Hour,
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
	// AccountRefund is the undo token for the account-scope charge THIS attempt made (zero when it made none). Pass it to
	// Success so a correct password gives its own charge back; see Success.
	AccountRefund AccountRefund
}

// AccountRefund records the account row exactly as it was before this attempt's charge, and the version the charge left.
type AccountRefund struct {
	charged        bool
	version        int64
	failures       int
	blockedUntilMs int64
	lastFailureMs  int64
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

type rowState struct {
	failures       int
	blockedUntilMs int64
	lastFailureMs  int64
	lastSuccessMs  int64
	version        int64
	exists         bool
}

func (t *LoginThrottle) read(ctx context.Context, subject, scope string) (rowState, error) {
	var s rowState
	err := t.db.QueryRowContext(ctx,
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

// reserve charges one attempt on (subject, scope) atomically. until is the end of the block when refused. undo describes the
// row as it was before the charge (used to give a correct password's own account charge back, see Success).
func (t *LoginThrottle) reserve(ctx context.Context, subject, scope string, free int, decay time.Duration, now time.Time, pol LoginThrottlePolicy) (allowed bool, until time.Time, undo AccountRefund, err error) {
	nowMs := now.UnixMilli()
	for i := 0; i < casRetries; i++ {
		if _, err = t.db.ExecContext(ctx,
			`INSERT OR IGNORE INTO login_throttle (subject, scope) VALUES (?, ?)`, subject, scope); err != nil {
			return false, time.Time{}, AccountRefund{}, err
		}
		s, rerr := t.read(ctx, subject, scope)
		if rerr != nil {
			return false, time.Time{}, AccountRefund{}, rerr
		}
		if s.blockedUntilMs > nowMs {
			return false, time.UnixMilli(s.blockedUntilMs), AccountRefund{}, nil
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
		res, uerr := t.db.ExecContext(ctx,
			`UPDATE login_throttle SET failures = ?, blocked_until_ms = ?, last_failure_ms = ?, version = version + 1 WHERE subject = ? AND scope = ? AND version = ?`,
			f, nb, nowMs, subject, scope, s.version)
		if uerr != nil {
			return false, time.Time{}, AccountRefund{}, uerr
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return true, time.Time{}, AccountRefund{charged: true, version: s.version + 1,
				failures: s.failures, blockedUntilMs: s.blockedUntilMs, lastFailureMs: s.lastFailureMs}, nil
		}
		runtime.Gosched()
	}
	return false, time.Time{}, AccountRefund{}, ErrLoginThrottleBusy
}

// Begin decides whether this attempt may be evaluated and charges it. It must be called BEFORE the password is checked.
func (t *LoginThrottle) Begin(ctx context.Context, subject, ip string) (Decision, error) {
	now, pol := t.clock()
	nowMs := now.UnixMilli()
	t.maybePurge(ctx, now, pol)
	bucket := ipScope(ip)

	// 1. This address's own block (read-only, so a refused attempt is not charged anywhere).
	b, err := t.read(ctx, subject, bucket)
	if err != nil {
		return Decision{}, err
	}
	if b.blockedUntilMs > nowMs {
		return refuse(time.UnixMilli(b.blockedUntilMs), now), nil
	}
	known := b.lastSuccessMs > 0 && nowMs-b.lastSuccessMs <= pol.KnownGoodTTL.Milliseconds()

	// 2. The account block, unless this is a known-good address.
	a, err := t.read(ctx, subject, scopeAccount)
	if err != nil {
		return Decision{}, err
	}
	if t.testAfterAccountRead != nil {
		t.testAfterAccountRead()
	}
	if a.blockedUntilMs > nowMs && !known {
		return refuse(time.UnixMilli(a.blockedUntilMs), now), nil
	}
	// A known-good address is never charged to the account scope (and so a legitimate owner's login has no effect on it, which
	// would otherwise tell a real account from a nonexistent name); it is still limited by its own address bucket below.
	var refund AccountRefund
	if !known {
		ok, until, undo, err := t.reserve(ctx, subject, scopeAccount, pol.AccountFree, pol.AccountDecay, now, pol)
		if errors.Is(err, ErrLoginThrottleBusy) {
			return refuse(now.Add(pol.Base), now), nil
		}
		if err != nil {
			return Decision{}, err
		}
		if !ok {
			return refuse(until, now), nil
		}
		refund = undo
	}

	// 3. This address's charge.
	ok, until, _, err := t.reserve(ctx, subject, bucket, pol.BucketFree, pol.BucketDecay, now, pol)
	if errors.Is(err, ErrLoginThrottleBusy) {
		return refuse(now.Add(pol.Base), now), nil
	}
	if err != nil {
		return Decision{}, err
	}
	if !ok {
		return refuse(until, now), nil
	}
	return Decision{Allowed: true, AccountRefund: refund}, nil
}

func refuse(until, now time.Time) Decision {
	d := until.Sub(now)
	if d < time.Second {
		d = time.Second
	}
	return Decision{Allowed: false, RetryAfter: d}
}

// Success clears this address's charges (never the account scope) of a completed login and (rememberClient) records the address as known-good.
func (t *LoginThrottle) Success(ctx context.Context, subject, ip string, rememberClient bool, refund AccountRefund) error {
	now, pol := t.clock()
	nowMs := now.UnixMilli()
	// Give back THIS attempt's own account charge (R4-1). Without it every owner login from a new address would leave +1 on the
	// account count and refresh its decay clock, so a quiet account with no attacker would reach the block after ~20 new-address
	// logins. The row is restored to exactly what it was before the charge, by compare-and-swap on the version the charge left:
	// if any other attempt touched the row since, nothing is restored (a prior or concurrent attacker charge is never erased; the
	// worst case is that one charge stays, which is the previous, safe behaviour). A nonexistent name never reaches Success, and
	// the restored row is byte-for-byte what a request that never happened would leave, so S1 stays closed: the owner's login
	// does not move the account state an attacker can observe.
	if refund.charged {
		if _, err := t.db.ExecContext(ctx,
			`UPDATE login_throttle SET failures = ?, blocked_until_ms = ?, last_failure_ms = ?, version = version + 1 WHERE subject = ? AND scope = ? AND version = ?`,
			refund.failures, refund.blockedUntilMs, refund.lastFailureMs, subject, scopeAccount, refund.version); err != nil {
			return err
		}
	}
	// The account scope is deliberately NOT touched: a nonexistent name can never log in, so an account block that ended early on
	// the owner's success would reveal that the account exists. Only the owner's own address bucket is cleared.
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

// Clear removes every throttle row of an account (administrator unlock, password change or reset).
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
	// Bounded work inside a login request: at most purgeBatch rows per run (the rest go on the next run, a minute later).
	_, _ = t.db.ExecContext(ctx,
		`DELETE FROM login_throttle WHERE (subject, scope) IN (
			SELECT subject, scope FROM login_throttle WHERE blocked_until_ms < ? AND last_failure_ms < ? AND (last_success_ms = 0 OR last_success_ms < ?) LIMIT ?)`,
		nowMs, nowMs-pol.StaleAfter.Milliseconds(), nowMs-pol.KnownGoodTTL.Milliseconds(), purgeBatch)
}
