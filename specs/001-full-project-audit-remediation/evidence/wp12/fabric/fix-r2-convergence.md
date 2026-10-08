# fix-r2: convergence assessment (constitution 11.4.276(E)), written BEFORE the fix

| Field | Value |
|---|---|
| Subject | WP-12 PA-02 `pkg/decorators` and PA-07 `pkg/fabric`, uncommitted in `submodules/filesystem` |
| Input | independent review `WF19-REVIEW-fabric.md` (Opus xhigh, NO-GO: 15 source-defects, 2 test-instrumentation, 7 process-doc), evidence dir `WF19-fabric-evidence/` |
| Review round counter | this is the fix for review round 1 of this item; budget R_max is the project default (5); nothing here is a structural round |
| Fixer | single Sonnet fixer, one pass, no commits |
| Written | 2026-10-07, before any source edit |

## 1. Ground truth (11.4.276(B)) - what the fix is built on

The fixes rely on external behaviour, so the reviewer's ground-truth probes are the model, not my simplification of it:

- `golang.org/x/time/rate` `WaitN` / `Reservation.CancelAt` semantics (reviewer, rfc and package text): a wait longer than the context deadline fails without reserving, and a cancelled reservation is given back "as much as possible". The budget keeps its own limiter (adopting the package would need an 11.4.270 dependency verdict, not available), implementing the same two rules, and the rule that a returned slot in the PAST is not reusable (the reservation after it would then be closer than one interval).
- RFC 959 reply classes (4yz transient, 5yz permanent, 530/332/532 credential/account) and RFC 2228 (533 to 536 are security-negotiation replies, 430 is not defined). Taken from the reviewer's RFC check, not re-derived from memory.
- `reflect.Value.Comparable` documentation: for an interface it checks the dynamic value; `reflect.Type.Comparable` does not.
- Go `time.Duration` range (about 292 years) for the rate bound.

The reviewer's probes ran against the real clock, real `local` client, real `smb`/`webdav` config types; they are adopted verbatim (section 3), so the model that found the defects is also the model that guards the fix.

## 2. Defect classes and their members (11.4.276(C))

Each finding named a class; the fix closes every member I could enumerate with an instrument (the test file column), not only the reported instance.

| Class | Reported | Enumerated members | Test |
|---|---|---|---|
| C1 cancelled/abandoned budget reservations (S1, S15) | cancelled waiter, drift | tail cancel (wind back), middle cancel (reuse), past freed slot (must not reuse), deadline before slot (fail fast, no reservation), cancel while waiting for the concurrency slot (no reservation), many-cancellation livelock (real clock), leaked stream (lease), held-slot visibility | `budget_fix_test.go`, RV01, RV01b, RV14 |
| C2 cancellation read as a health/connection verdict in the pool (S2) | probe ctx | pre-cancelled ctx (200 iterations vs the 50% select), cancel during probe, cancel during new connect, unhealthy under live ctx still retired | `pool_fix_test.go`, RV03 |
| C3 repeated logins (S3) | per-borrow login | `GetClient`, `GetClientContext`, other root independent, `Evict`, settings change, transient not cached, `CloseAll` | `pool_fix_test.go`, RV05 |
| C4 pool probes/connect vs budget and unbounded waits (S4) | health probe throttled | probe exempt from `Limited`, `Connect` still budgeted, `ConnectTimeout` bounds `GetClient`, slot freed after a timed-out connect, hanging `Disconnect` bounded | `pool_fix_test.go`, RV15, `decorators_test.go` |
| C5 streams losing or hiding capabilities (S5, S7) | ReaderAt/Seeker/WriterTo stripped; raw handle exposed | all 8 capability subsets, metering of Read/ReadAt/WriteTo, close hook once, no extra methods (`Chmod`, `Fd`, `Sync`, `Truncate`), ReadFile and OpenSeekable, every fabric layer plus `ReadOnly` | `guard/guard_test.go`, `fix_classes_test.go`, `decorators/fix_test.go`, RV07, RV22 |
| C6 `GetConfig` leaking or aliasing the live config (S6) | ReadOnly | every fabric layer and `ReadOnly`; smb, webdav, local; struct, pointer, map, nested map, slice | `guard_test.go`, `fix_classes_test.go`, `decorators/fix_test.go`, RV21 |
| C7 comparability / lock safety (S8) | `ReturnClient` panic holding the lock | foreign unhashable return, unhashable from the factory, nil, wrapper value | `pool_fix_test.go`, RV04, RV04b |
| C8 error classification (S9) | substring on path text, precedence, code table | path-bearing wrappers and `PathError`, every auth marker, every FTP code 332/4yz/5yz/53x/430, every errno/EOF/DNS transient signal, precedence for every source | `fix_classes_test.go`, RV08, RV16 |
| C9 input canonicalisation (S10, S11, S12, S13, S14) | HostKey URL, control chars, tiny rate, typed nil, retry policy | URL/path/userinfo/whitespace rejected, IPv6 forms, IPv4-mapped, port forms; all 33 control characters plus DEL in paths and root, CopyFile either path; tiny and subnormal rates; typed nil in all five decorators; MaxAttempts bounds, zero BaseDelay, closed-form delay vs specification | `fix_classes_test.go`, RV02, RV06, RV09, RV10, RV11, RV12 |
| C10 test-instrumentation blind to cancellation (T1, T2) | fakes ignore ctx; clock never blocks | context-honouring pool fakes, `parkClock` (blocking `Sleep`), a test per reviewer mutant R01..R20 | all `*_fix_test.go`, `fix-r2-mutants.json` |
| C11 docs overstating behaviour (P1..P7) | 7 claims | each claim re-checked against measured behaviour, wrong ones removed or rewritten | `docs/filesystem/pkg_fabric.md`, `pkg_decorators.md`, evidence README |

## 3. What is adopted verbatim (11.4.276(D))

- All reviewer probes RV01..RV22 (fabric: `review_probes_test.go`, decorators: `review_probes_test.go`). Four observations/measurements (RV10, RV12, RV14, RV18) and two `t.Skipf` exits (RV02, RV22) were turned into assertions, each marked `ADOPTED AS ASSERTION`; RV15 needed one correction (it drained its result channel twice, which hangs on the now-correct fast path), marked `ADOPTED`.
- All reviewer mutants PC1, NC1, R01..R20 in `fix-r2-mutants.json`. Where the fix moved the mutated text (R05, R07, R13, R14, R15) the mutation keeps its meaning and the entry says `ADAPTED`.
- Own mutants N01..N48 for the new code (one or more per class member).

## 4. Prediction and stop rule

I expect the first full run to expose test-side mistakes of mine, not new source classes. A source change that a mutant or test shows incomplete is fixed in the same pass; a finding I cannot fix is reported with its reason.
Not claimed: the re-review (OWED, must be independent), live-server behaviour (no server contacted), the NFS SETATTR path of legacy `pkg/nfs` (UNCONFIRMED, needs root).
