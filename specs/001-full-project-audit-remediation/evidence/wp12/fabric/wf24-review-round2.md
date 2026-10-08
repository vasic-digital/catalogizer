# WF24 independent re-review: `pkg/decorators` (+ `guard`) and `pkg/fabric` at commit 83c0ac1

| Field | Value |
|---|---|
| Reviewer | independent reviewer, Opus at xhigh (11.4.142 / 11.4.209). Not the author or the fixer. |
| Reviewed at | 2026-10-08, 02:40 to 03:40 (UTC+2 host time) |
| Target | `submodules/filesystem` commit `83c0ac1f407f9e11044e97f4aa385ecfaf1cde21` (main repo `e9d6883d`): `pkg/decorators/*`, `pkg/decorators/guard/*`, `pkg/fabric/*`, plus `docs/filesystem/pkg_fabric.md` and `pkg_decorators.md` |
| Prior review | `WF19-REVIEW-fabric.md`: NO-GO with 15 source-defects (1 HIGH, 6 MEDIUM, 8 LOW), 2 test-instrumentation, 7 process-doc. This is the re-review after one class-complete fix pass (review round 2 of this item, 11.4.276). |
| Target identity | `sha256sum -c .../evidence/wp12/fabric/SOURCES.sha256`: 30/30 OK. The committed tree equals the evidence the fixer describes. The scratch copy came from `git archive HEAD`; its non-probe files equal the repo files (`tree-check.txt`). `git status` of both packages is clean after the review. |
| Repo edits | None. All probes and mutants ran in `/dev/shm/wf24-fabric/tree`. |
| Runner | `scripts/containers/run_pinned.sh --out /dev/shm/wf24-fabric IMG-GO` (go1.25.14 linux/amd64), `-race -count=1`, GOMAXPROCS=2, one container at a time. No NAS was contacted. W03 talks only to a loopback `httptest` server inside the container. Only dummy credential strings were used, and only their lengths were printed. |
| Evidence | `scratchpad/WF24-fabric-evidence/` (`SHA256SUMS` inside). Logs: `baseline.log`, `probes1.log`, `probes2.log`, `probes_rep2.log`, `probes_rep3.log`, `mut1.log`, `killers.log`, `w10ctl.log`, `rmut.log`. Probe sources: `zz_wf24_test.go`, `zz_wf24b_test.go`, `zz_wf24c_test.go`. Harness and mutant sets: `wf24_mut.py`, `wf24_mutants.json`, `killer_mutants.json`, `w10ctl_mutants.json`, `r_plus_mutants.json`. |
| Verdict | **NO-GO.** 11 new source-defects (2 MEDIUM, both latent until SMB/WebDAV are wired; 9 LOW), 2 test-instrumentation (1 MEDIUM, 1 LOW), 4 process-doc (LOW). 21 of the 22 prior findings are closed. S3 is closed only for the protocols whose login failures the classifier recognises. |

## 0. Summary

**What the fix pass got right.**
- The HIGH finding (S1, budget livelock) is fixed.
- The class-complete approach worked in most places.
- All 20 of my round-1 mutants are now caught when the suite runs with `-race` (T1 closed).
- A randomised property test of the start-spacing invariant found 0 violations across 1816 granted starts and 1212 cancellations. Its instrument is control-needled.

**What stops a GO.** The two MEDIUM source-defects come from one class: a fix checked against a simplified model of the protocol clients instead of the real ones (11.4.276(B)).

- **N1:** S3's "no repeated logins" guard is verified only with `MarkAuth` fakes. The real SMB logon failure (go-smb2 `*ResponseError`, NTSTATUS description text) and the real WebDAV `401` are both classified `permanent`. The pool therefore logs in again on every borrow: 20 of 20 for each protocol.
- **N2:** S4 exempts `TestConnection` from the host budget on the premise that "a probe is a keepalive". For SMB the real probe is `share.ReadDir(".")`, a full listing of the share root. The pool runs it on every borrow, outside `MaxConcurrent`.

**Test gaps.** 10 of my 25 new mutants survive the committed suite (T3). Each one is killed by a probe that passes on the committed code, so none of them is equivalent. Two matter most:
- M08: with a stream lease, a normally closed stream never releases its slot.
- M66: the S1 fail-fast refuses callers it should serve.

**Scope note.** None of the new findings affects the FTP path. FTP is the only protocol wired through the fabric today (`pkg/ftp/scan.go`). Its glue marks logins with `MarkAuth`, and its probe is a `NOOP`. The NAS protocol survey (`evidence/wp12/nas-protocols`) covers only FTP, FTPS, SFTP and NFS. N1 and N2 become real when PA-08..10 wire SMB or WebDAV.

## 1. Method

1. **Baseline on the read-only `/src` mount** (`baseline.log`):
   - `go vet` is clean.
   - 146 top-level tests: 169 PASS lines, 0 FAIL, 0 SKIP.
   - Statement coverage: decorators 100.0%, guard 91.6%, fabric 97.1%. This matches the fixer's `fix-r2-green-run*`.
2. **Read every changed source file**, the adopted probes and the new tests.
   - I diffed the adopted `review_probes_test.go` files against my WF19 originals. Every change is one of the disclosed `ADOPTED AS ASSERTION` or `ADOPTED` strengthenings. Nothing was weakened.
   - I also read the real counterparts the fix depends on:
     - go-smb2 v1.1.0: `conn.go:597-672`, `errors.go:45`, `session.go:228-236`, `client.go:795-810`, `internal/erref/ntstatus.go`
     - `pkg/smb/smb.go:47-80` and `:120-126`
     - `pkg/webdav/webdav.go:48-92`
     - `pkg/ftp/proto.go:395-418` and `ftp.go:546-640`
     - `pkg/sftp/sftp.go:669-697`
     - `pkg/nfs3/client.go:600-613`
     - `pkg/factory/factory.go`
     - the catalog-api consumers
3. **Wrote 25 new probes.**
   - W01 to W18 are defect probes. Each asserts the correct behaviour, so a FAIL is the evidence.
   - K11 to K69 are killer probes. Each passes on the committed code and must fail on one surviving mutant.
   - W10 is a randomised property test of the start-spacing invariant: 40 seeds, `parkClock`, random arrivals, cancellations and time advances.
   - All probes ran 3 times with identical outcomes (`probes1`, `probes_rep2`, `probes_rep3`). W18 is a measurement and gave 34, 35 and 35.
4. **Ran my own mutation harness** (`wf24_mut.py`). It is a rewrite of my WF19 harness and is not the fixer's.
   - It copies the committed tree and deletes my probe files, so only the committed suite judges.
   - It applies one exact-match edit per mutant and runs `go test -race` over both packages.
   - Controls: the baseline must PASS, PC1 must be CAUGHT, NC1 must SURVIVE. All three held in every campaign.
5. **Ran three more campaigns:**
   - (a) the fixer's adapted R01..R20 under `-race` (`rmut.log`);
   - (b) the 10 survivors against the killer probes (`killers.log`);
   - (c) an instrument control for W10: mutant V02 reuses past freed slots and must be caught (`w10ctl.log`).

## 2. Prior findings: closure check (each probe re-run against 83c0ac1)

| Prior | Verdict | Evidence (this session) |
|---|---|---|
| **S1** HIGH budget livelock | **CLOSED** | RV01: a fresh caller is served in 50.2 ms after the burst. RV01b: starts per 400 ms window `[7 7 7 6]` against a nominal 8; it was `[2 0 0 0]`. W10 property test: 1816 starts granted, 1212 waiters cancelled, **0 spacing violations**; its control V02 (past slot reuse) is CAUGHT. Residuals: N8, and the T3 survivors M01, M66, M69. |
| S2 MEDIUM cancelled borrow wipes idle | **CLOSED** | RV03: `idle=3 disconnects=0 connects=3`. Residual: T3 survivor M11. |
| S3 MEDIUM repeated logins after auth failure | **PARTIALLY CLOSED** | RV05 (MarkAuth): 1 login for 50 borrows. **Not closed for SMB or WebDAV**: W02 gives 20 of 20, W03 gives 20 of 20 (N1). Not single-flight and keyed per root: N3. |
| S4 MEDIUM probe throttled; `GetClient` blocks | **CLOSED as reported** | RV15: returned in 70 µs with 0 disconnects. The exemption introduced N2. |
| S5 MEDIUM capabilities stripped | **CLOSED** | RV07: ReaderAt and ReadSeeker are preserved through `Limited`. My M73 (one capability subset dropped) is CAUGHT. |
| S6 MEDIUM GetConfig leaks the live config | **CLOSED** for the shipped configs | RV21 passes for smb, webdav and local. Residual N9 is latent: nested shapes. |
| S7 LOW raw stream handle | **CLOSED** | RV22: the stream is `struct{*guard.base; raMix; skMix; wtMix}` and has no `Chmod`. |
| S8 LOW comparable proxy / wedged lock | **CLOSED** | RV04 gives `ErrNotFromPool` with no panic. RV04b gives a refusal. Residual N10: duplicate client from the factory. |
| S9 LOW classifier | **CLOSED as reported** | RV08, RV16 pass. Residuals: N7 (4yz path text), D1 (MarkTransient doc), and the T3 survivors M38, M40. |
| S10 LOW HostKey | **CLOSED** | RV09 now rejects the URL; RV10 passes. Residual N11: malformed input accepted. |
| S11 LOW control characters | **CLOSED for C0 and DEL** | RV06 passes. Residual N6: C1 controls. |
| S12 LOW tiny rate | **CLOSED** | RV02 gives `ErrBudgetConfig`. |
| S13 LOW typed nil | **CLOSED** | RV11, RV20 pass. |
| S14 LOW retry storm | **CLOSED as reported** | RV12: BaseDelay=0 is rejected. Residual N5: 1 ns delay. |
| S15 LOW leaked stream | **CLOSED** | `Held` and `OldestHeld` are now visible; the lease is opt-in. Residuals: D2, and the T3 survivors M07f, M08. |
| T1 MEDIUM 20 of 20 reviewer mutants survived | **CLOSED** | `rmut.log`: R01..R20 are all CAUGHT with `-race`, PC1 CAUGHT, NC1 SURVIVED. I checked that the six `ADAPTED` entries keep the original meaning. |
| T2 MEDIUM instruments blind to cancellation | **CLOSED** | `parkClock` and the context-honouring fakes exist and are load-bearing: they catch R06, R07, R20, M66 and the cancellation tests. |
| P1..P7 docs | **CLOSED** | Auth precedence is documented correctly, lazy sweep over all roots, per-root pool cap, BudgetStats semantics, no `FullJitter` reference (grep: 0 hits), the Confined and nested-retry limits, and the README states that "38/38" was author-selected. New doc items: D1 to D4. |

## 3. New findings

`finding_layer` follows 11.4.235(D). Findings are ordered by severity.

### N1: MEDIUM, source-defect (latent until SMB/WebDAV are wired). The real SMB and WebDAV credential failures are not `ClassAuth`, so the pool's rejected-login guard never engages for them

**Where.**
- `errors.go:73-78`, the `authMarkers` list. It carries `"status_logon_failure"` and `"logon failure"`, but go-smb2 never prints either string.
- The S3 claim in `pkg_fabric.md`: "so a scan that borrows per file cannot trip a NAS auto-block".

**Ground truth: SMB (code-traced in go-smb2 v1.1.0).**
- The session-setup refusal goes through `accept()`, then `acceptError()`, which returns `&smb2.ResponseError{Code: status}` (`conn.go:597-672`).
- `Error()` prints `"response error: " + <MS-ERREF description>` (`errors.go:45`).
- `pkg/smb` wraps it as `fmt.Errorf("failed to create SMB session: %w", err)` (`smb.go:66`).

**Ground truth: WebDAV.** `Connect` returns `"WebDAV server returned status 401"` (`webdav.go:68`), with no marker in the text.

**Evidence (3 of 3 runs).**
- **W01:** all 8 NTSTATUS credential and account codes classify as `permanent`, using the library's own error type:
  - `STATUS_LOGON_FAILURE` 0xC000006D, whose text is "The attempted logon is invalid…"
  - `STATUS_ACCOUNT_LOCKED_OUT` 0xC0000234
  - `_DISABLED`, `PASSWORD_EXPIRED`, `NO_SUCH_USER`, `ACCOUNT_RESTRICTION`, `INVALID_LOGON_HOURS`, `PASSWORD_MUST_CHANGE`
  - Control needles: `MarkAuth` and the SSH text marker are both seen.
- **W02:** 20 pool borrows after a real `STATUS_LOGON_FAILURE` made **20 logins**.
- **W03:** the real `pkg/webdav` client against an HTTP `401` server (loopback `httptest`): **20 authenticated requests for 20 borrows**, `class=permanent`.

**Impact.**
- The account-lockout protection that S3 was fixed for does not exist for two of the six protocols the factory supports.
- It is worst for `ACCOUNT_LOCKED_OUT`, where the pool keeps trying a locked account.
- `Retrying` is unaffected, because permanent errors are not retried either.
- The RV05 regression test passes only because it uses `MarkAuth` (11.4.276(B): a simplified model).
- Live server: **UNCONFIRMED** at runtime against a Samba server; none was started. The error value is the library's own exported type, and the path was traced line by line.

**Fix direction.**
- In `pkg/smb` `Connect`, use `errors.As(err, *smb2.ResponseError)`. For the NTSTATUS logon and account class, return `fabric.MarkAuth(...)`; consider 0xC0000193 (account expired) as well.
- In `pkg/webdav` `Connect`, map `401`, and `403` at Connect, to `MarkAuth`.
- Add regression tests that use the real library error type and the real WebDAV client against `httptest`, not `MarkAuth` fakes.
- Until that lands, qualify the pool doc's guarantee by protocol.
- Optionally, add a structural seam (for example `interface{ AuthFailure() bool }`) so the classifier does not depend on English text.

### N2: MEDIUM, source-defect (design premise; latent until SMB is wired with a budget). The probe exemption lets the pool's per-borrow health probe bypass `MaxConcurrent` and the rate limit, and for SMB that probe is a full share-root listing

**Where.**
- `limited.go`: `TestConnection` is exempt "because a probe is a keepalive, not a unit of catalog load".
- `budget.go:16`: "MaxConcurrent is the maximum number of operations … in flight at once".

**Ground truth.**
- `pkg/smb` `TestConnection` is `c.share.ReadDir(".")` (`smb.go:124`). go-smb2 implements it as `Open` plus `Readdir(-1)` plus sort (`client.go:795-810`): a full enumeration of the share root, many QUERY_DIRECTORY round trips on a large root.
- WebDAV's probe is a PROPFIND (`webdav.go:91`). NFS3's is NULL plus GETATTR.
- `Pool.get` probes every idle connection it hands out (`pool.go:257`).

**Evidence.**
- **W11:** `MaxConcurrent=1` held by an open stream, rate 10/s. 8 `TestConnection` calls ran with **peak concurrency 8** (3 of 3 runs).
- With Pool plus Limited plus SMB, every borrow adds one unbudgeted root listing. Concurrency is bounded only by the number of borrowers (MaxPerKey per root, times the number of roots), and the rate by the borrow rate. The budget therefore no longer "bounds the load one host sees".

**Note.** My WF19 review offered "exempt health probes from Limited" as one fix option. Checking the real `TestConnection` implementations shows that option is unsafe for SMB. I am correcting my own suggestion, not blaming the fixer.

**Fix direction.**
- Keep probes inside the budget but never blocking: add a `TryAcquire`. A busy budget means "skip the probe and trust a connection used within a short validation window", or "treat it as healthy". It must never mean "unhealthy".
- Or skip the probe for connections returned less than N seconds ago (in the spirit of HikariCP's alive-bypass window).
- Make SMB `TestConnection` a `Stat(".")`.
- State the exemption, or its replacement, in the `BudgetConfig` and `HostBudget` docs (D4).

### N3: LOW, source-defect. The rejected-login memory is neither single-flight nor shared across roots that use one account

**Evidence: W04.**
- 4 concurrent `GetClientContext` calls against a slow login that fails produce **4 logins**.
- 3 roots (movies, music, photos) on one account produce 3 logins.

The cache is set only after the first `Connect` returns, and it is keyed by `StorageConfig.ID`.

**Impact.** Each settings generation costs up to `MaxPerKey × roots` failed logins. This is bounded, but at the NAS auto-block threshold scale.

**Fix direction.** Single-flight the new-connection login per key: later borrowers wait for the in-flight outcome. Optionally key the negative cache by (host, username, credential fingerprint).

### N4: LOW, source-defect. A fabric layer turns an inner `(nil, nil)` stream into a non-nil wrapper that panics on `Close`, and `Limited` then leaks the host slot

**Evidence: W05.**
- `Limited(inner).ReadFile` returns `*guard.base` wrapping nil.
- `Close` panics with a nil-pointer dereference before `OnClose` runs.
- Afterwards `InFlight=1, Held=1`: the slot is gone for good.

`ReadOnly` handles the same input correctly (K51), so the class is handled in one layer only. Where: `layer.go:94-116` and `:199-222`, and `guard.Wrap` does not check for nil.

**Fix direction.** In `layer.ReadFile` and `OpenSeekable`, treat a nil stream with a nil error as a protocol error: release, then return an error. Alternatively pass nil through as `ReadOnly` does, and make `guard.Wrap(nil)` return nil.

### N5: LOW, source-defect. `RetryPolicy` validation is a proxy: a 1 ns delay passes and is the same back-to-back storm

**Evidence: W06.** `{MaxAttempts:100, BaseDelay:1ns, MaxDelay:1ns}` is accepted and made **100 attempts in about 1.5 ms** against `ECONNREFUSED` (3 of 3 runs). `validate` (`retrying.go:43-55`) asserts `BaseDelay > 0`, not the real condition, which is "no retry storm" (11.4.201).

**Fix direction.** Set a floor on the minimum effective delay after the `MaxDelay` cap (for example 10 ms), or a minimum total backoff per attempt budget.

### N6: LOW, source-defect (defence in depth) and claim. `Confined` forwards C1 controls and raw invalid bytes

**Evidence: W07.** These paths all reached the inner client:
- U+0085 (NEL) and U+009B (CSI), both Unicode `Cc`;
- raw bytes 0x85 and 0x9B (invalid UTF-8).

`confined.go:15` documents "any control character"; `pkg_fabric.md:22` correctly says "the rest of C0 and DEL".

**Fix direction.**
- Refuse `unicode.IsControl(r)`.
- Refuse `utf8.RuneError` produced by invalid encoding, or require `utf8.ValidString`.
- Extend `FuzzConfined` accordingly.

### N7: LOW, source-defect. A 4yz FTP reply that echoes a file name containing an auth marker is classified as bad credentials

**Evidence: W08.**
- `450 "/Movies/Access Denied (2019).mkv: Resource temporarily unavailable"` gives `auth`.
- `451 "/home/m/not logged in.txt: …"` gives `auth`.

Both should be `transient`. The S9 path-text class comes back through the 4yz text check (`errors.go:154-158`). Some FTP servers, ProFTPD-style, echo the path in 4yz and 5yz replies. That echo is UNCONFIRMED for the Synology server.

**Fix direction.**
- Apply the 4yz "login failed" text rule only to replies to USER, PASS and ACCT. `pkg/ftp` already marks those with `MarkAuth` (`proto.go:526,539`).
- Otherwise a 4yz reply is transient.

### N8: LOW, source-defect. `giveBack` does not wind the schedule back when the returned slot was reused and has become the tail

**Evidence: W09.** The sequence:
- A starts at T+0.
- B reserves T+100, then C reserves T+200, and B cancels.
- D reuses T+100, then C (the tail) cancels and the front is wound back to T+200.
- D cancels. Because `reused=true`, its slot T+100 is inserted into `free`, and `nextSlot` stays at T+200.
- At T+150, caller E **waits 50 ms** for slot T+200. Nobody uses that slot, and a start at T+150 is already 1.5 intervals after the only real start.

`budget.go:184` gates the wind-back on `!reused`. The loss is bounded, at most one interval per such slot, and the spacing invariant holds (W10).

**Fix direction.** Wind back whenever `start + interval == nextSlot`, whatever `reused` is.

### N9: LOW, source-defect (latent) and claim. `RedactConfig` does not handle nested shapes, so "no secret, no pointer to the live config" holds only for flat configs

**Evidence: W12.** On a synthetic config:
- a nested struct field's `Password` is kept;
- an exported pointer field (`TLS *…`) is shared, and setting `InsecureSkipVerify` through the copy reached the live config;
- a secret inside a slice of maps is kept: `copySlice` copies the slice but does not redact the maps inside it, although the doc says "slices of such maps … copied too";
- fields named `Pass` and `PrivKey` are not treated as secrets.

There is no live leak today, because every shipped config is flat (smb, webdav, local, nfs3, plus the ftp and sftp `PublicConfig`).

**Fix direction.**
- Prefer an allow-list: protocol clients return a `PublicConfig`, and `RedactConfig` returns nil for unknown shapes.
- Or deep-copy recursively with a depth bound.
- Widen the name list (pass, pwd, key, cookie, authorization).
- State the limits in `guard/config.go` and `pkg_decorators.md`.

### N10: LOW, source-defect (latent). The borrowed set is keyed by the client value, so a factory that returns the same client twice leaks a pool slot permanently

**Evidence: W13.** With a memoising factory and `MaxPerKey=2`:
- two borrows give the same client and `InUse=1`;
- the second `ReturnClient` returns `ErrNotFromPool`;
- after that only one concurrent borrow is possible (`ErrPoolExhausted`).

This is the "key by a pool-owned handle" half of the S8 fix direction, which was not adopted. Where: `register`, `pool.go:331-342`.

**Fix direction.** In `register`, refuse a client already present in `inUse`: return an error and release the token. Or key by a pool-owned handle.

### N11: LOW, source-defect. `HostKey` accepts malformed input as a key

**Evidence: W14.**
- `"[::1"` gives the key `"[::1"`.
- `"nas:445:1"` gives `"nas:445:1"`.
- `"[nas]x:22"` gives `"nas"`, silently dropping the trailing garbage.

**Fix direction.** Split with `net.SplitHostPort` semantics, and reject anything after `]` other than `:port`.

### T3: MEDIUM, test-instrumentation. 10 of 25 new reviewer-authored mutants survive the committed suite, and all 10 are non-equivalent

Each survivor is killed by a probe that **passes on 83c0ac1**. The baseline `BASELINE_FAILING=` is empty in `killers.log`, and every killer CAUGHT its mutant.

| Mutant | Change | Killed by |
|---|---|---|
| **M08** | with a lease, a normal `Close` never releases the slot: every stream leaks, and hosts deadlock | W16 |
| **M66** | over-eager fail-fast refuses a caller whose deadline is after its slot. This is the 11.4.201(1) false-refusal direction of the S1 fix, and it is untested. | K66 |
| M07f | the lease timer is not stopped on `Close`, so `LeaseExpired` counts streams closed in time | W16 |
| M01 | the wind-back does not cascade through freed slots below the cancelled tail | W15, K69 |
| M69 | the free list is not kept in ascending order (a stale front; throughput loss) | K69 |
| M11 | `requeue` into a pool closed during the probe keeps the connection open for good | K11 |
| M13 | the zero fingerprint is accepted as a cache key, so a settings change is not detected | K13 |
| M38 | `errors.Join` members are not walked; a joined path wrapper reads as auth | K38 |
| M40 | FTP `500` is classified transient | K40 |
| M51 | `ReadOnly` wraps a nil stream, which panics on use. Statement coverage of 100% hides this branch. | K51 |

The suite caught the other 15 new mutants: M03, M19, M28, M29, M30, M35, M37, M39, M43, M44, M45, M46, M54, M70, M73.

**Fix direction.** Adopt W15, W16 and K11, K13, K38, K40, K51, K66, K69 verbatim as regression tests (11.4.276(D)), then re-run `wf24_mutants.json`.

### T4: LOW, test-instrumentation

- The adopted RV14 assertion `st.OldestHeld < 0` is vacuous (it can never be true). Only `Held != 2` carries weight; assert `OldestHeld > 0`.
- The `rc == nil` branch in `ReadOnly.ReadFile` and `OpenSeekable` is an expression inside a statement that 100% statement coverage counts as covered, yet it is never exercised (M51). This is the coverage-is-a-proxy point of 11.4.224(C).

### D1: LOW, process-doc. `MarkTransient`'s contract is false for marked structured replies and auth-marked leaves

**Where:** `errors.go:37` says "MarkTransient wraps err so that Classify reports ClassTransient".

**Evidence:** W17 gives `Classify(MarkTransient(&textproto.Error{550}))`, which returns `permanent`. A leaf with an auth marker inside `MarkTransient` returns `auth`, which is the intended precedence.

**Fix direction:** Document the precedence on `MarkTransient` itself.

### D2: LOW, process-doc. The stream-lease docs say the host sees "slightly" or "briefly" more than `MaxConcurrent`

**Where:** `budget.go:83` and `pkg_fabric.md:49`.

**Evidence:** W18 measured 34, 35 and 35 other operations running alongside an actively read stream during 400 ms, with `MaxConcurrent=1` and a 40 ms lease. An absolute lease cannot tell a leaked stream from a long active one, such as a 2-hour movie. The overrun lasts as long as the stream does.

**Fix direction:** Use an idle-based lease (reset on every read), or document that the lease suspends the cap for the rest of a long stream's life.

### D3: LOW, process-doc. Status and revision headers are stale after the commit

**Where:** `pkg_fabric.md:7-8`, `pkg_decorators.md:7-8` and the evidence `README.md` header still say "NOT committed there", "re-review is OWED" and "Last modified 2026-10-07T20:30". Commit 83c0ac1 committed the code, and this document is the re-review (11.4.44 / 11.4.106).

### D4: LOW, process-doc. The budget's headline claims ignore the exempted probes

**Where:** `BudgetConfig.MaxConcurrent` ("maximum number of operations … in flight") and `pkg_fabric.md` ("bounds the load one host sees") are no longer true once `TestConnection` and `Disconnect` bypass the budget.

**Fix direction:** Resolve this together with N2.

## 4. Observations (not findings)

- **Fail-fast error type.** The S1 fail-fast returns an error that wraps `context.DeadlineExceeded` while the caller's context is still alive. RV01's caller loop recorded 46,707 fail-fast returns in 0.8 s across 8 goroutines. `Retrying` does not spin, because `Classify` gives `permanent`. A naive consumer loop that retries without backoff would spin. Consider a distinct `ErrBudgetBusy` (fabric) error that still satisfies `errors.Is(..., context.DeadlineExceeded)`.
- **Lease timer clock.** `WithStreamLease` uses `time.AfterFunc`, the real clock, not the injected `Clock`. Tests of the lease therefore depend on wall time.
- **Unmarshalable settings.** When the settings cannot be marshalled (zero fingerprint), the rejected-login memory is off by design: every borrow logs in again. catalog-api builds string, int and bool settings, so this does not happen today.

## 5. Mutation results

| Campaign | Judge | Result |
|---|---|---|
| `mut1.log`: 25 new reviewer mutants plus PC1 and NC1 | committed suite, `-race` | baseline PASS; PC1 CAUGHT; NC1 SURVIVED; 15 CAUGHT, 10 SURVIVED. M07 did not compile; it was replaced by M07f in `rmut.log`, where it SURVIVED. |
| `rmut.log`: fixer-adapted R01..R20, plus PC1, NC1 and M07f | committed suite, `-race` | **R01..R20: 20 of 20 CAUGHT** (T1 closed); PC1 CAUGHT; NC1 SURVIVED; M07f SURVIVED |
| `killers.log`: the 10 survivors plus NC1 | killer probes only (`-run TestW15\|TestW16\|TestK`) | 10 of 10 CAUGHT, NC1 SURVIVED: no survivor is equivalent |
| `w10ctl.log`: V02 (past freed slots reused) plus NC1 | W10 only | V02 CAUGHT by W10, NC1 SURVIVED: the property instrument can see a spacing violation |

## 6. Verdict and what a GO needs

**NO-GO.**

The 11.4.276 class shared by N1 and N2 is "fix verified on a simplified model of the protocol client" (11.4.276(B)). It now has two MEDIUM members in round 2. If round 3 shows another member, 11.4.276(E) calls for a structural round. The next fix should build its ground-truth probe on the real libraries:
- the go-smb2 error type;
- the real WebDAV client against `httptest`;
- each protocol's real `TestConnection`.

**Shortest path to GO:**
1. **N1:** `MarkAuth` in the SMB and WebDAV glue, with tests that use the real error types (W01 to W03 adopted).
2. **N2 and D4:** either budgeted but non-blocking probes, or a probe-skip window; plus SMB `TestConnection` as a `Stat`.
3. **T3:** adopt the 9 killer probes (W15, W16, K11 to K69).
4. **N3 to N11, T4, D1 to D3:** fix them, or track each as a 11.4.197 item. These are LOW and none of them is release-gating alone.

**Alternative for N1 and N2.** If SMB and WebDAV are explicitly out of scope until PA-08..10, N1 and N2 can be closed by narrowing the claims: the pool's lockout guarantee and the budget bound stated per protocol, plus a tracked item each. The FTP path, which is the only wired consumer, is unaffected.

**Residual list if the conductor decides to proceed anyway:** N1 to N11, T3, T4, D1 to D4, as above.

**Not claimed / UNCONFIRMED.**
- A live Samba server returning `STATUS_LOGON_FAILURE`. The error value was built from go-smb2's exported type and traced, not observed on the wire.
- Synology FTP echoing paths in 4yz replies.
- The load of SMB `ReadDir(".")` on the real NAS share roots.
- No NAS was contacted.
