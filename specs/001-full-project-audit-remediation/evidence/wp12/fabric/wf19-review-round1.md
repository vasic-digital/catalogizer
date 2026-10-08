# WF19 independent review: `pkg/decorators` (PA-02) and `pkg/fabric` (PA-07)

| Field | Value |
|---|---|
| Reviewer | independent reviewer, Opus at xhigh (11.4.142 / 11.4.209); not the author |
| Reviewed at | 2026-10-07T19:28Z |
| Target | `submodules/filesystem/pkg/decorators/*`, `pkg/fabric/*`, uncommitted in the own-org submodule |
| Target identity | `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/fabric/SOURCES.sha256` gives 20/20 OK before and after the review, so the reviewed bytes are the bytes the author's evidence describes. The author evidence `SHA256SUMS` gives 14/14 OK. |
| Repo edits | none. All probes and mutants ran in a copy at `/dev/shm/wf19-fabric-review/tree`. The copy's non-probe files equal the author tree (`tree-pristine.sha256` diff was empty). |
| Runner | `scripts/containers/run_pinned.sh --out /dev/shm/wf19-fabric-review IMG-GO` (golang 1.25.14 linux/amd64, digest-pinned), `-race -count=1`, GOMAXPROCS=2, one container at a time. No NAS or network target was contacted. Only dummy credential strings were used, and only their lengths were printed. |
| Evidence (durable copy) | `scratchpad/WF19-fabric-evidence/` (SHA256SUMS inside): `baseline-src.txt`, `cover-func.txt`, `probes.txt`, `probes2.txt`, `probes3.txt`, `mutants-run.txt`, `rv_mutants.json`, `rv_mut.py`, `fabric_zz_review_test.go`, `decorators_zz_review_test.go`, `run{1,2,3}.sh` |
| Verdict | **NO-GO**: 15 source-defects (1 HIGH, 6 MEDIUM, 8 LOW), 2 test-instrumentation, 7 process-doc |

## 0. What was confirmed

- **Baseline.** I re-ran the author's tests on the real tree. `go vet` is clean. `pkg/decorators` passes with 100.0% statement coverage and `pkg/fabric` passes with 97.1% (`baseline-src.txt`, `cover-func.txt`). This matches the author's green-run claims. The uncovered statements are in `Pool.get` (86.5%), `register` (75.0%) and `ReturnClient` (95.8%).
- **Read-only guarantee at the Go interface.** It holds. The five mutators refuse and never reach the inner client, no interface is embedded, and the class-exhaustive reflection guard is control-needled. The author's D1–D10 mutants cover this well. The defects below are about what the read-only handle still hands out (S6, S7), not about the mutator methods.
- **No goroutines are spawned anywhere in either package**, so there is no goroutine-leak class. `systemClock.Sleep` stops its timer.
- **`Confined` lexical containment held** against every traversal form I tried: `..`, backslash, segment-boundary siblings, `//`, relative paths and NUL. The two gaps are control characters (S11) and server-side symlinks (P6).
- **Auth is never retried by `Retrying` itself.** The FTP glue marks login failures with `MarkAuth` (`pkg/ftp/ftp.go:305-314`), and that check runs first.

## 1. Method

1. **Read every source file and test.** I also read the consumers: `pkg/ftp/scan.go` (`NewScanClient` / `NewWorkerPool` already compose `ReadOnly`, `Limited`, `Retrying` and `Pool`), and catalog-api's `comic_pages_handler.go` and `stream_handler.go`, which type-assert stream capabilities.
2. **Wrote 19 adversarial probe tests** (`zz_review_test.go`, in the scratch copy only). Each asserts the correct behaviour, so a FAIL is the captured evidence. Timing-sensitive probes ran 3× (`probes2.txt`, `probes3.txt`) and were deterministic.
3. **Ran my own mutation harness** (`rv_mut.py`, not the author's `mutate.py`).
   - It copies `go.mod`, `go.sum` and `pkg/`, deletes my probe files so that **only the author's tests judge**, and applies one exact-match edit per mutant.
   - The baseline must PASS first. A positive control (PC1) must be CAUGHT, so the harness provably can see a catch. A negative control (NC1, no-op) must SURVIVE.
   - Result: baseline PASS, PC1 CAUGHT, NC1 SURVIVED, **20 of 20 reviewer-authored mutants SURVIVED** (`mutants-run.txt`).
4. **Checked the FTP code semantics against the RFC text** (rfc-editor.org): RFC 959 4yz/5yz definitions and codes 332, 421, 425, 426, 450, 451, 452, 530, 532 and 550; RFC 2228 codes 431 and 533–536 (430 does not appear in RFC 2228). I also checked the `golang.org/x/time/rate` `WaitN` and `Reservation.CancelAt` semantics.

## 2. Findings

Ordered by severity. `finding_layer` follows 11.4.235(D).

### S1: HIGH, source-defect. `HostBudget` never returns cancelled start slots, so drift turns into livelock

**Where:** `budget.go` `Acquire`. The reservation `b.nextSlot = start.Add(b.interval)` is kept when `clock.Sleep` returns a context error. The comment says this is deliberate: "keeps the rate bound strict".

**Mechanism.** Every waiter whose context ends pushes `nextSlot` one interval further, but no start ever happens in that slot. Reservations arrive at `MaxConcurrent / T` per second, where T is the waiter patience. Once `(MaxConcurrent / T) × interval > 1`, `nextSlot` runs ahead of real time faster than time passes. Every later waiter then times out too, which pushes it further still: throughput collapses to zero and stays there. For `NASBudget()` (4 concurrent, 10/s) this happens for any caller patience under 0.4 s.

Even with patient callers, a burst of cancellations leaves a backlog that all later callers must wait out. Because the budget is shared per host across protocols, one burst stalls SMB, SFTP and FTP to that host together. Cancellations are routine in an HTTP server: a client navigates away, a thumbnail scrolls out of view.

**Evidence** (`probes.txt`, `probes2.txt`, deterministic 3/3):
- **RV01** (budget 2 concurrent, 20/s; 8 callers with 30 ms patience for 0.8 s): 1 real start, 213–216 cancellations, `Stats().Acquired` = 107–159 reservations, `TotalRateWait` = 3m58s to 9m23s. A **fresh caller with a 1 s budget is then starved** (`context deadline exceeded` after 1.0006 s), although the host saw one start.
- **RV01b** (callers willing to wait 80 ms, which exceeds the 50 ms spacing, so a correct limiter serves them about 20/s): starts per 400 ms window were **`[2 0 0 0]`** in all 3 runs. That is a livelock: 2 starts in 1.6 s against a nominal 32.

**Why the suite cannot see it:** see T2.

**Fix direction:**
- Do not reserve a slot whose wait exceeds the context's remaining deadline; fail fast. This is the `x/time/rate` `WaitN` contract: "returns an error if ... the expected wait time exceeds the Context's Deadline".
- On cancellation, give the slot back for reuse. `Reservation.CancelAt` "reverses the effects of this Reservation on the rate limit as much as possible"; here that could be a free-slot min-heap that later acquirers consume before extending `nextSlot`.
- The rate bound stays strict, because a reused slot is still at least one interval from its neighbours.
- Adopting `x/time/rate` would need an 11.4.270 dependency verdict.
- Add a regression test that uses the real clock, or a fake clock whose `Sleep` blocks until either the virtual deadline or context cancellation (see T2).

### S2: MEDIUM, source-defect. One cancelled borrow destroys every healthy idle connection of the root

**Where:** `pool.go` `get`.
- `healthy()` derives the probe context from the caller's context, so a caller deadline is read as "connection unhealthy". The candidate is then `discard`ed (disconnected), and the loop moves on to the next candidate, whose probe fails immediately because the context is already done.
- All idle connections go. The pool then creates a new client and calls `Connect(ctx)`, which is one more login, and that fails too.
- With `GetClientContext` and an already-cancelled context, `select` picks the ready token case about half the time, so even a pre-cancelled caller can wipe the pool.

**Evidence: RV03** (`probes.txt`). Three healthy idle connections; one `GetClientContext` with a 5 ms deadline against clients whose probe takes 20 ms and honours the context gives `err=fabric: connect a: context deadline exceeded idle=0 disconnects=3 connects=4`.

**Impact:** connection churn and login amplification against the NAS, driven by ordinary client cancellations.

**Fix direction:**
- When `ctx.Err() != nil` after a failed probe, push the candidate back to idle and return `ctx.Err()`.
- Check `ctx.Err()` before the token `select` and before creating a client.
- Never count caller cancellation as a health verdict.

### S3: MEDIUM, source-defect. The pool keeps logging in after an authentication failure

**Where:** `pool.go` `get`. A failed `Connect` leaves nothing idle, so every borrow creates a client and logs in again.

**Evidence: RV05.** 50 borrows after a bad password gave **50 login attempts**. Every error correctly classified as `ClassAuth`.

**Impact:**
- The fabric's own rationale ("repeating a failed login can lock the account out", in both `errors.go` and the docs) holds per call but not across calls.
- A scan that borrows per file or per directory after a password change hammers the NAS with failed logins. Synology-class auto-block works by counting failed logins per IP.
- `NewWorkerPool` in `pkg/ftp` is built exactly this way.

**Fix direction:**
- Negative-cache `ClassAuth` per key: the pool fails fast with the cached error until `Evict` or a new config generation.
- Optionally use a time-based circuit breaker for transient failures.
- Add a test that counts `Connect` calls.

### S4: MEDIUM, source-defect. Pooled `Limited` clients throttle their own health probe; `GetClient` ("without waiting") blocks indefinitely and discards a healthy connection

**Where:** `limited.go` exempts only `Disconnect`, so the pool's `TestConnection` probe and its new-connection `Connect` take host-budget slots. `Pool.GetClient` passes `context.Background()`, so `Connect` waits without a bound. `pkg/ftp` `NewWorkerPool` with `ScanOptions.Budget` builds exactly this composition.

**Evidence: RV15** (`probes3.txt`, 3/3). Budget of 1 concurrent; one open read stream (any protocol) on the host holds it. `GetClient` was **still blocked after 1 s**, and **the healthy idle connection was disconnected** (`disconnects=1`). It returned only after the stream closed, having created a second connection.

**Fix direction:**
- Exempt health probes from `Limited`, or treat a budget wait as "busy" rather than "unhealthy".
- Bound `Connect` in `GetClient` (for example with `HealthTimeout`, or a connect timeout option).
- Document that pooled clients should not be wrapped by a budget the pool's own probes depend on.

### S5: MEDIUM, source-defect (latent until wiring). Every fabric layer strips `io.ReaderAt` / `io.ReadSeeker` / `io.WriterTo` from returned streams

**Where:** `layer.go`. `readStream` embeds only `io.ReadCloser`, and `seekStream` embeds only `client.ReadSeekCloser`.

**Evidence: RV07.** For the raw local client, `OpenSeekable` implements `io.ReaderAt` and `ReadFile` implements `io.ReadSeeker`; behind `Limited` neither does.

**Impact on real consumers once catalog-api wires the fabric:**
- `catalog-api/handlers/comic_pages_handler.go:214-217`, "Strategy 1 — random-access, no full download", requires `rs.(io.ReaderAt)`. It would fall back to buffering the **whole comic file per request**.
- `catalog-api/internal/handlers/stream_handler.go:168` loses the second-chance Range path.

Both are silent performance and amplification regressions against the NAS (whole-file reads instead of ranged reads). The docs claim only that `SeekableClient` is preserved, which is true, and that is exactly why this goes unnoticed.

**Fix direction:**
- Pick the wrapper type at construction from the inner stream's dynamic capabilities (ReaderAt, WriterTo, Seeker), forwarding with the same metering hooks (count `n` from `ReadAt` too).
- Add a capability-preservation test over the real local client.

### S6: MEDIUM, source-defect. The read-only handle hands out the live inner config, including the plaintext password

**Where:** `readonly.go` `GetConfig` forwards `r.in.GetConfig()`, and so does every fabric layer. `pkg/smb` and `pkg/webdav` return the live `*Config` (`smb.go:302`, `webdav.go:512`); `pkg/local` returns `c.config`.

**Evidence: RV21.**
- `ReadOnly(smb).GetConfig()` exposes the password (length 25, dummy value); `smb.NewSMBClient(got)` would be a read-write client.
- Setting `got.Share = "admin$"` re-targets the **inner** client's share through the read-only handle.
- `ReadOnly(webdav)` exposes the password too.
- For local, mutating `BasePath` re-roots the client (a read escape).

Contrast: `pkg/ftp`, `pkg/sftp` and `pkg/nfs3` already return redacted copies (`PublicConfig`, `ReadOnly: true`).

**Impact:**
- The decorator's guarantee ("a catalog scan can never modify a storage host") is defeated by one forwarded method, which also exposes credentials (CONST-042 / 11.4.10).
- The documented limit ("a second, non-`client.Client` write path") does not cover it, because `GetConfig` *is* a `client.Client` method.

**Fix direction:**
- `ReadOnly.GetConfig` (and the fabric layers) should return a redacted, defensive copy, or nil with a typed accessor.
- Fix the SMB, WebDAV and local clients to return value copies without secrets.
- Add a test that a mutation of the returned value cannot reach the inner client and that no secret field is present.

### S7: LOW, source-defect. `ReadOnly` returns the inner client's raw stream handle, which can still mutate metadata

**Where:** `readonly.go` `ReadFile` / `OpenSeekable` return the inner stream unwrapped.

**Evidence: RV22.**
- `ReadOnly(local).ReadFile` returns an `*os.File`, and `(*os.File).Chmod(0o600)` **succeeded and changed the file mode on disk** (`err=<nil> mode now -rw-------`). fchmod works on a read-only descriptor for the owner; so do `futimens`, xattrs, and anything else reachable through `Fd()`.
- Legacy `pkg/nfs` also returns `*os.File` from a **read-write** mount (`syscall.Mount(..., 0, ...)`, `nfs.go:63`). By code reading the same calls become NFS SETATTR on the NAS. This is **UNCONFIRMED at runtime**, because the NFS mount needs root and was not attempted.
- For SMB, the go-smb2 `GENERIC_READ` open mask should make the server refuse; not probed (no server).

LOW because a scan would not call `Chmod` accidentally, and because in the recommended chain the outer fabric layers wrap the stream (which hides the handle, and is also the cause of S5).

**Fix direction:** wrap returned streams in a minimal type that exposes only Read, Seek, ReadAt and Close, chosen by capability as in S5. Alternatively, state the limit explicitly in the docs.

### S8: LOW, source-defect. The pool's "comparable" guard asserts a proxy, which leads to a panic and then a deadlocked pool

**Where:** `pool.go`.
- `register` and `ReturnClient` check `reflect.TypeOf(c).Comparable()`. That is true for a struct holding an interface field even when the dynamic value inside is unhashable. This is a 11.4.201 proxy, not the real condition; `reflect.Value.Comparable` (Go 1.20+) checks the dynamic value.
- In `ReturnClient` the map lookup panics **after `p.mu.Lock()` without `defer`**.

**Evidence: RV04 / RV04b.**
- `ReturnClient(foreign)` panics with "hash of unhashable type". After `recover`, `p.Stats()` **deadlocks** (2 s watchdog).
- `GetClient` from a factory returning such a value panics. `register` uses `defer`, so it does not deadlock.
- A foreign client should produce `ErrNotFromPool`.

**Fix direction:**
- Use `reflect.ValueOf(c).Comparable()`, or key `inUse` by a pool-owned handle rather than the client value.
- `defer p.mu.Unlock()` in `ReturnClient`'s critical section.

### S9: LOW, source-defect. The error classifier misreads server-controlled and path-controlled text, mis-orders auth precedence, and mis-maps some RFC codes

The class is error classification in `errors.go` `Classify`. Members found:

1. **Substring markers over the whole error string**, which embeds file names and server reply text (RV08):
   - a reset on a file named `Access Denied (2019).mkv` gives `auth` (want `transient`);
   - not-found on `not logged in.txt` gives `auth` (want `permanent`);
   - FTP `550 Access denied`, a file-level permanent reply per RFC 959 ("File unavailable (e.g., file not found, no access)"), gives `auth`.

   Retry behaviour is the same (none), but consumers branch on `ClassAuth`; `pkg/ftp` tests assert it. A per-file permission error or a hostile file name would read as "bad credentials".
2. **Precedence contradicts "auth first"** (RV16): `421 "Login authentication failed, too many attempts"` gives `transient`, because the `textproto` 4yz switch runs before the auth text markers. Any glue that does not `MarkAuth` would retry a lockout reply.
3. **Code table versus RFC.**
   - 332 is a 3yz *positive intermediate* reply ("Need account for login").
   - 532 is "Need account for storing files".
   - 534 and 535 are RFC 2228 security-policy replies ("Request denied for policy reasons", "Failed security check"), not credential failures.
   - 430 is not defined in RFC 2228.
   - RFC 959 defines all 4yz as transient ("the error condition is temporary and the action may be requested again"), yet 431 and 452 fall to permanent (RV08: `452` gives `permanent`).

**Fix direction:**
- Match markers only on errors the glue marks, or on structured types (`textproto` codes, `ssh` errors); never on path-bearing text.
- Decide auth before transient for all sources.
- Align the code table with RFC 959 and RFC 2228, and document any deliberate exception.

### S10: LOW, source-defect. `HostKey` is lexical; URLs collapse to the scheme

**Where:** `op.go` `HostKey` strips everything after a single `:`.

**Evidence:**
- **RV09:** `HostKey("smb://nas-a.example") == HostKey("smb://nas-b.example") == "smb"`. Unrelated hosts share one budget, or the second gets `ErrBudgetConflict`. WebDAV configs carry a URL, so this misuse is plausible.
- **RV10 (observation):** `::1` versus `0:0:0:0:0:0:0:1`, and `192.168.1.5` versus `::ffff:192.168.1.5`, get different keys (no RFC 5952 canonicalisation).

**Fix direction:**
- Reject input containing `://` or `/`.
- Canonicalise IP literals with `net.ParseIP` / `netip` (unmap 4-in-6).
- Document that names are not resolved (`nas` versus `nas.local` versus its IP do not share).

### S11: LOW, source-defect (defence in depth). `Confined` refuses NUL but forwards CR, LF, ESC and other C0 controls

**Evidence: RV06.** The inner client saw `"/data/a\r\nDELE /data/b"`, `"/data/a\nx"` and `"/data/a\x1b[2J"`.

The FTP glue refuses CR, LF and NUL itself (`pkg/ftp/ftp.go` `confine`), so there is no current exploit. But the fabric is advertised as inherited by every future protocol, and command injection through a read path would bypass `ReadOnly`. The control-character class is handled for one member (NUL) only.

**Fix direction:** refuse all bytes below 0x20 and 0x7F in `normalise`; add them to `FuzzConfined`'s invariant.

### S12: LOW, source-defect. A valid tiny `MaxReqPerSec` fails open

**Where:** `budget.go`. `time.Duration(float64(time.Second) / rate)` overflows for any rate below about 1.08e-10 per second. The conversion result is implementation-defined, and here the result fell below 1, so it was clamped to 1 ns: no spacing at all.

**Evidence: RV02.** Rate 1e-11 (one start every ~3000 years) allowed an immediate second start, with `TotalRateWait=0s`.

**Fix direction:** reject a config whose interval is not representable, or saturate the interval at its maximum value.

### S13: LOW, source-defect (claim). A typed-nil inner is accepted at construction

**Evidence: RV20, RV11.** `ReadOnly((*local.Client)(nil))` and `Limited((*local.Client)(nil), b)` do not panic. The docs say "A nil inner panics at construction, not on first use".

**Fix direction:** add a `reflect.ValueOf(inner).Kind() == Ptr && IsNil()` check, or soften the claim.

### S14: LOW, source-defect. `RetryPolicy` allows a zero-delay, unbounded retry storm; `delay` is O(attempt)

**Evidence: RV12.** `MaxAttempts: 1000` with `BaseDelay: 0` is accepted, and produced **1000 attempts in 10.5 ms** against a host refusing connections.

`delay()` loops `attempt-1` times when the delay cannot saturate (BaseDelay 0, no MaxDelay), which is O(n²) across a run.

**Fix direction:**
- Require `BaseDelay > 0` when `MaxAttempts > 1`, and cap `MaxAttempts`.
- Compute the backoff in closed form (shift with a saturation check).

### S15: LOW, design robustness. An unclosed stream holds its host slot forever, across all protocols

**Evidence: RV14 (measurement).** Two unclosed `ReadFile` streams (one SMB-tagged, one SFTP-tagged) on a shared `MaxConcurrent: 2` budget block every further operation to the host (`InFlight=2`, deadline exceeded).

This is within the documented contract, but the blast radius is the whole host, and there is no lease, idle timeout or leak detection.

**Fix direction:** add an optional per-stream lease or idle timeout that releases the slot (and logs), plus a held-slots-by-age statistic.

### T1: MEDIUM, test-instrumentation. 20 of 20 reviewer-authored mutants survive the author's suite; "38/38 CAUGHT" measures an author-chosen set

The harness has been shown able to see a catch (PC1 CAUGHT by `TestClassify_Table` and `TestRetrying_NeverRetriesAuthFailure`), and NC1 SURVIVED. Survivors, with the gap each exposes:

| ID | Mutant | Missing assertion |
|---|---|---|
| R01, R02 | `ReadOnly.GetFileInfo` / `FileExists` forward path `"/"` | The recording fake ignores paths, and no real-filesystem test calls these through `ReadOnly`. |
| R03 | every fabric layer forwards `CopyFile(src, src)` | Only the first path is asserted anywhere (`TestConfined_InsidePaths…`, `FuzzConfined`). |
| R04 | `OpenSeekable` leaks a stream returned with an error | `leakyOpen` covers `ReadFile` only. |
| R05 | `layer.IsConnected` returns true | The pool's health and return checks are blind through decorators. |
| R06 | default `HealthTimeout` set to 0 | Fakes ignore the context (T2). |
| R07 | new-connection `Connect` uses `context.Background` | Fakes ignore the context (T2). |
| R20 | `discard` has no timeout | Fakes ignore the context (T2). |
| R08 | idle order MRU changed to LRU | No test checks idle order. |
| R09 | `MaxLifetime` boundary `>=` changed to `>` | Tests use 30 s / 61 s, never the exact boundary. |
| R10 | a disconnected client is re-pooled on return | Contradicts the `ReturnClient` doc; untested. |
| R11 | `GetClientContext` waiters not woken by `CloseAll` | `TestPool_CloseAll` discards the waiter's result (`_ = err`). |
| R12 | ECONNABORTED, ETIMEDOUT, ECONNREFUSED, EHOSTUNREACH, ENETUNREACH, ENETDOWN no longer transient | The classifier table is only spot-checked. |
| R13 | FTP 425, 426, 450, 451 no longer transient | Same. |
| R14 | FTP 534, 332, 532 no longer auth (534 is documented) | Same. |
| R15 | 9 of 12 auth text markers removed | Same. |
| R16 | DNS timeout no longer transient | Same. |
| R17 | `io.EOF` no longer transient | Same. |
| R18 | `Sample.Protocol` never set | Unasserted. |
| R19 | the `ctx.Err()` check in the retry loop removed | **Near-equivalent**: both shipped clocks return `ctx.Err()` from `Sleep`, so only the error wrapping differs. Listed for completeness, not counted as a gap. |

**Fix direction:** add one test per surviving behaviour, and re-run `rv_mut.py` until no non-equivalent mutant survives.

### T2: MEDIUM, test-instrumentation. The instruments are structurally blind to cancellation and timeout behaviour

- `fakeClock.Sleep` never blocks: it advances virtual time instantly, so a reservation can never be cancelled mid-wait. That is why S1 is invisible. `TestBudget_RateWaitInterruptedByContextReleasesSlot` asserts only `InFlight`, never whether the reserved start slot was reusable.
- Every pool fake (`fk`, `poolClient`) ignores `ctx`. Therefore S2, R06, R07 and R20 cannot fail under the suite.

**Fix direction:**
- Use a fake clock whose `Sleep` parks until advanced or cancelled.
- Use context-honouring fakes (as `ctxClient` in RV03 does).
- Add real-clock drift and livelock regression tests (RV01b shape).

### P1: LOW, process-doc. `pkg_fabric.md` states the classification order as "auth first", which is false

The FTP 4yz switch precedes the auth text markers (S9 item 2, RV16).

### P2: LOW, process-doc. Pool and budget claims overstate behaviour

- **"IdleTimeout retires a connection that sat idle this long" / "MaxLifetime and IdleTimeout retirement".** Retirement is lazy, on borrow, and only for the most-recently-used entry. RV18: an idle connection sat for about 10× `IdleTimeout` and was never retired while a single worker reused the top entry. There is no reaper, so connections stay open on the NAS and count against per-IP limits.
- **"GetClient borrows … without waiting".** It can block indefinitely (S4).
- **"HostBudget … bounds the load one host sees from this process".** It bounds operations, not connections. The pool cap is per storage root, not per host, so N roots on one NAS hold N×MaxPerKey connections.

### P3: LOW, process-doc. The `decorators` doc overreaches

- "a catalog scan can never modify a storage host" omits `GetConfig` (S6) and the raw stream handle (S7).
- "A nil inner panics at construction" is not true for a typed nil (S13).

### P4: LOW, process-doc. `BudgetStats` semantics are undocumented

`Acquired` counts reservations, including cancelled ones: RV01 shows 101–159 "Acquired" against 1–2 real starts. `InFlight` includes callers sleeping for a start slot.

### P5: LOW, process-doc. `retrying.go` doc comment references a nonexistent `FullJitter`

Confirmed by grep: there is no definition anywhere in `pkg/` or the docs.

### P6: LOW, process-doc. Two composition limits are missing from the docs

- `Confined` is lexical. A server-side symlink or junction inside the root that points outside it is followed by the server. This was not probed (no server); it follows from code reading, since `normalise` never consults the server.
- Nested retries. `pkg/sftp` `Connect` already retries internally (`sftp.go:320`, `retry` at line 374). `fabric.Retrying` (where `Connect` is retryable) multiplies the attempts.

### P7: LOW, process-doc. The evidence README presents "38 of 38 CAUGHT" without saying the mutant set is author-selected

The independent mutation result is 20 of 20 non-control mutants surviving (T1). The README line "independent review OWED" can now point to this file. Other README claims checked out:
- The test count (60 Test/Fuzz functions in fabric, consistent with "57+").
- The coverage figures.
- The RED disclosure: tests ran against absent code, as disclosed.

## 3. Verdict

**NO-GO.**

- **S1** is a HIGH availability defect in the component whose purpose is to protect the NAS: a burst of cancelled callers livelocks every protocol to the host.
- **S2–S6** are MEDIUM source-defects. Each is reproduced by a captured RED probe, and each touches a claim the docs make: lockout safety, "without waiting", read-only, Range support once wired.
- **T1 and T2** show that the current suite cannot detect these classes.

All source-defects are in uncommitted, not-yet-wired code, so the cost of fixing them now is low.

Suggested order:
1. S1 (with T2's blocking fake clock).
2. S2, S3 and S4 together, since they are one context and auth discipline in `Pool.get`.
3. S6 and S5.
4. The LOW items and the doc corrections.
5. Re-run `rv_mut.py` together with the RV probes as regression guards.
