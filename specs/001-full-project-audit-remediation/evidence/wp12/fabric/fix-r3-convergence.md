# fix-r3: convergence assessment (constitution 11.4.276(E)), written BEFORE the fix

| Field | Value |
|---|---|
| Subject | WP-12 PA-02 `pkg/decorators` (+ `guard`) and PA-07 `pkg/fabric`, committed in `submodules/filesystem` at `83c0ac1` (main repo `e9d6883d`) |
| Input | independent re-review `wf24-review-round2.md` (Opus xhigh, NO-GO: 11 source-defects N1..N11, 2 test-instrumentation T3/T4, 4 process-doc D1..D4), evidence dir `WF24-fabric-evidence/` (probes W01..W18, killers K11..K69, mutant sets) |
| Review round counter | this is the fix for review round 2 of this item (round 1 = WF19, fixed by round 2). Budget R_max is the project default (5). Not a structural round yet, see section 4 |
| Fixer | single Sonnet fixer, one pass, no commits (the conductor commits) |
| Written | 2026-10-08, before any source edit |
| Round-3 status | the code of rounds 1 and 2 is committed at `83c0ac1`; the round-3 changes described here are UNCOMMITTED at the time of writing |

## 1. Class-trigger analysis (11.4.276(E))

The re-review names ONE class shared by its two MEDIUM findings: "a fix checked against a simplified model of the protocol clients instead of the real ones" (11.4.276(B)): N1 (S3 verified only with `MarkAuth` fakes while the real go-smb2 and WebDAV errors are not auth) and N2 (S4 premised on "a probe is a keepalive" while SMB's real probe lists the share root). Two members in one round plus the round-1 history (S3 itself was the first member) means a SECOND member in the next review forces a structural round. So this pass applies the structural remedy already:

1. **Ground truth is rebuilt on the real artefacts, not on fakes.** Every new regression test for N1 builds its error from the library's own exported type (`*smb2.ResponseError` with the NTSTATUS code, the exact wrapping `pkg/smb` does) or drives the real `pkg/webdav` client against a loopback `httptest` server. For N2 the real `pkg/webdav` `TestConnection` (a PROPFIND) is driven through Pool + Limited and the server-side peak concurrency is measured; the SMB probe (`share.ReadDir(".")`, code-traced by the reviewer in go-smb2 v1.1.0) cannot be driven here (no server is started), so the budget bound is proven on the abstract property (a probe never runs outside MaxConcurrent / the rate) and the SMB `Stat(".")` change is reported as a needed change in `pkg/smb` (outside this scope).
2. **Every protocol the factory supports gets a row** in a table of "real login-failure shapes" (smb, webdav, ftp, ftps, sftp, nfs3, local) so no protocol is left classified by assumption.
3. **Class-complete enumeration with instruments** (section 2): marker lists are enumerated by an internal test over the real `authMarkers`, control characters over the whole Unicode `Cc` range plus invalid UTF-8, host forms over a generated grid.

## 2. Defect classes and their members (11.4.276(C))

| Class | Reported | Enumerated members | Test |
|---|---|---|---|
| C1 login-failure recognition (N1) | SMB, WebDAV | all NTSTATUS logon/account codes (MS-ERREF: 0xC000006A WRONG_PASSWORD, 0xC0000064 NO_SUCH_USER, 0xC000006D LOGON_FAILURE, 0xC000006E ACCOUNT_RESTRICTION, 0xC000006F INVALID_LOGON_HOURS, 0xC0000070 INVALID_WORKSTATION, 0xC0000071 PASSWORD_EXPIRED, 0xC0000072 ACCOUNT_DISABLED, 0xC0000193 ACCOUNT_EXPIRED, 0xC0000224 PASSWORD_MUST_CHANGE, 0xC0000234 ACCOUNT_LOCKED_OUT, 0xC000015B LOGON_TYPE_NOT_GRANTED) via the real type, wrapped like `pkg/smb` wraps; file-level NTSTATUS (ACCESS_DENIED, OBJECT_NAME_NOT_FOUND) must stay NOT auth; WebDAV 401/407 anywhere and 403 at login; structural seam `AuthFailure() bool`; per-protocol table | W01-W03, `fix_r3_*_test.go` |
| C2 probe load outside the budget (N2, D4) | `TestConnection` exempt | probe with a free slot is counted (concurrency + rate), probe with a busy budget / not-due start slot is SKIPPED (`ErrProbeSkipped`, no reservation, never "unhealthy"), skipped probe keeps the pooled connection, Disconnect stays exempt and is documented, real WebDAV through Pool+Limited | W11, K66-class, `fix_r3_*_test.go` |
| C3 rejected-login memory scope (N3) | single-flight, per account | concurrent borrows of one root, borrows of N roots on one account, different account not blocked, settings change, Evict, transient not cached, CloseAll, pilot outcome transient lets the next borrower retry, ctx cancel while waiting for the pilot | W04, `fix_r3_*_test.go` |
| C4 nil / typed-nil streams (N4, T4) | layer wraps nil | Limited, Retrying, Confined, Metered x ReadFile and OpenSeekable; nil and typed nil; slot released; `guard.Wrap(nil)`; `ReadOnly` typed nil | W05, K51 |
| C5 retry delay floor (N5) | 1 ns storm | base, max, jitter-returned-zero, closed-form overflow all floored by `MinRetryDelay` at the effective delay | W06, `fix_r3_*_test.go` |
| C6 path guard characters (N6) | C1 controls, invalid UTF-8 | every rune 0x00-0x9F, U+007F, raw bytes 0x80-0xFF alone, invalid sequences; `FuzzConfined` extended | W07, `fix_r3_*_test.go` |
| C7 4yz text rule (N7) | path echo | every `authMarker` placed in a path inside a 4yz reply (internal test over the real list) stays transient; credential-specific markers in the reply head stay auth | W08, `markers_internal_test.go` |
| C8 reservation give-back (N8) | reused tail | reused slot that is the tail winds back, cascade through free slots, randomised spacing invariant (W10) with its instrument control | W09, W10, W15, K69 |
| C9 RedactConfig shapes (N9) | nested | nested struct, pointer (deep copy), slice of maps/structs, array, interface, map of struct, depth bound, cycle, widened names (token based, no `bypass`/`passive` false positive) | W12, `guard_test.go` |
| C10 pool identity (N10) | duplicate client | duplicate from the factory refused before Connect and never disconnected, slot released | W13 |
| C11 host key forms (N11) | malformed accepted | bracket without close, text after `]`, two colons, empty/non-numeric/out-of-range port, bracketed non-IP | W14 |
| C12 stream lease semantics (D2) | absolute lease | idle-based lease: active read keeps the slot, an idle stream loses it, normal close releases and is not counted, WriteTo activity counted while copying | W16, W18 |
| C13 documentation (D1..D4, observations) | stale / overstated | status headers, MarkTransient precedence, lease, budget claims, error type for the fail-fast | docs diff |

## 3. What is adopted verbatim (11.4.276(D))

- All reviewer probes W01..W18, K11..K69 (three files `wf24*_test.go`). W17 and W18 are measurements/documentation probes: they are strengthened into assertions of the documented behaviour and marked `ADOPTED AS ASSERTION`.
- All reviewer mutant sets (`wf24_mutants.json`, `killer_mutants.json`, `w10ctl_mutants.json`, `r_plus_mutants.json`) and the reviewer's harness `wf24_mut.py`, copied unchanged next to the fix-r3 files; entries whose text moved are `ADAPTED` in `fix-r3-mutants.json`.
- Own mutants for every new member.

## 4. Prediction and stop rule

Expected: the first full run exposes existing tests encoding superseded premises (probe exemption, per-root negative cache, `OldestHeld`) and several tests with sub-10 ms retry delays; those are updated and listed. A source change that a mutant shows incomplete is fixed in the same pass.

If the next review finds a NEW defect of the "fix verified on a simplified model" class, round 4 MUST be a structural round (fresh fixer, re-derive ground truth from the protocol libraries' source, replace rather than layer).

Not claimed: the re-review (OWED, independent); behaviour against a live Samba/Synology/WebDAV server (none contacted); the SMB probe cost on a real share root (needs `pkg/smb` `Stat(".")`, outside scope).

## 5. Addendum (written AFTER the first full runs, still inside round 3; the sections above are unchanged)

The prediction in section 4 held (superseded-premise tests appeared) but real-artefact ground truth also corrected my own first design three times, which is the 11.4.276(B) point of building the tests on the real types:

1. **The real go-smb2 text of STATUS_ACCESS_DENIED is `{Access Denied} A process has requested access to an object ...`.** It contains the generic marker "access denied", so a refused SHARE (tree connect, `failed to mount SMB share`) classifies as ClassAuth. With a per-ACCOUNT rejected-login memory this would have blocked every other share of the account on one share's denial. Fix: two kinds of auth evidence (`isCredentialFailure` strict vs generic), strict ones remembered per account, generic ones per root (`TestR3_ShareDenialIsRememberedPerRootNotPerAccount`, `TestR3_EvictLiftsAPerRootDenial`).
2. **Settings that identify no account (pkg/ftp's scan factory holds the account itself and passes a bare `StorageConfig{ID}`)** would have merged all roots of a pool into one account; such a config is now its own account, keyed by its ID (`TestR3_ConfigsWithoutAccountSettingsAreSeparateAccounts`).
3. **A remembered rejection must not cut off working idle connections of other roots of the account** (the first design checked the memory before looking at idle connections); now only NEW logins are refused (`TestR3_AccountRefusalDoesNotCutOffIdleConnections`).

Process facts: the first mutation campaign was stopped after 25 mutants and restarted from scratch when (2) was found (`fix-r3-mut-aborted-partial.txt`, kept as history, not counted). The full campaign `fix-r3-mut-run1.txt` had 17 mutants INVALID with an empty compile diagnostic in one contiguous block (host memory pressure while the container ran with a ~567 MB ceiling is the probable cause, UNCONFIRMED); they compiled and ran in `-run2`. Two real survivors (X55, X56) and one more (X62) were found by the campaign and closed with new tests (`-run2`, `-run3`). The race-instrumented fuzz run was OOM-killed by the container ceiling and was repeated without `-race` (`fix-r3-fuzz-oom.txt`, `fix-r3-fuzz.txt`). One `go vet` compile check was run on the host by mistake before the rootless runner was used (no artefact kept; every result in this directory comes from the pinned container).
