# CAT-LOGIN-LOCK-2 round 4 evidence (PROVISIONAL)

Status: PROVISIONAL. Round-4 author changes for review round 3 (NO-GO), findings F14-F20/N1/RM2b. Not independently reviewed yet.

Classes: SQLite tests = UNIT-CLASS (in-memory SQLite, injected clock). PostgreSQL tests = LIVE-CLASS (real PostgreSQL 15 in rootless podman, local images only: postgres:15-alpine and postgres:15; both report datcollate=en_US.utf8 in this run, so the "byte-order" label of earlier rounds for the alpine image is not what was measured here).

## Files
- red_before.txt: SQLite run on the F14 test with the round-3 logic (tests added, fix not yet applied; 5 new tests).
- green_sqlite.txt: after the fix: 23 PASS, 0 FAIL (`-skip Postgres -run 'Throttle|Invariant|Begin'`).
- mutants_run.txt: reviewer mutants RM1-RM6 (+RM2b) against the fixed code.
- pg_both_images.txt: real PostgreSQL, both images. df_before_pg.txt / df_after_pg.txt: free disk 24G before, 28G after (threshold 12G).
- vet.txt: `go vet -buildvcs=false ./...` exit 0. `gofmt -l` on login_throttle*.go: empty (playback_session_repository_test.go is unformatted on the base branch and untouched).

## RED -> GREEN (SQLite, unit-class)
- F14 TestInvariant_R4_EveryEvaluatedAttemptIsCharged: RED "evaluated 24, charged 6" (window 0-5: evaluated 4, charged 1 each) -> GREEN "evaluated 24, charged 24".
- F14 TestInvariant_R4_ChargingBlockedAccountFollowsDoublingAndCap: RED (failures 21, expected 24) -> GREEN.
- F15 TestInvariant_R4_CapWindowHoldsWithinTTLAndExpiresAfter: in red_before.txt it failed because of a bug in the test itself (cumulative clock steps, fixed afterwards); it is therefore NOT RED-on-old-code evidence. Its value is mutant RM6/RM4 kill (below).
- F16 TestInvariant_R4_FailErrorRollsBackAndKeepsToken, F17 TestInvariant_R4_PurgeBoundaryOfInflightTokens: pin behaviour that was already correct (PASS before and after); their value is the mutant kills.
- F18: the test comments now say that EvaluatedBeforeBlockBound/Waves/Concurrent pass on the round-2 code too (SQLite serialises connections); only the PostgreSQL bound test and the lock-order pin cover the atomic hand-off.

## Fix (F14)
Fail always charges the account in the same transaction as the token deletion, also when the account is already blocked (reserve(..., chargeBlocked=true)): failure count +1, block end = max(current end, now + doubling delay of new count), so never shortened, never longer than Cap from now; AccountFree + MaxInflight - 1 = 23 bound and cap of 4 unchanged (tests above, PG below).

## Mutants (SQLite suite, `-skip` of the two slow ~80s/17s model tests AttackerTraceIsIndependent / OwnerWithoutAttackerIsNeverBlocked, which do not exercise these mutants; the full 23-test run above includes them)
| Mutant | Result | Killed by |
|---|---|---|
| RM1 drop post-token account re-check | KILLED | EvaluatedBeforeBlockConcurrent, AccountBlockedBetweenReadAndToken |
| RM2 lockSlots updates nothing | KILLED | 16 tests |
| RM2b lockSlots reports success on 0 rows | SURVIVED | NOT TESTABLE on SQLite (needs a concurrent purge between the two statements; one connection here). UNCONFIRMED on PostgreSQL (not mutation-run there) |
| RM3 Fail charges after token gone | KILLED | RV52, RV54 |
| RM4 purge ignores TTL | KILLED | R4 CapWindow, R4 PurgeBoundary |
| RM5 swallow charge error | KILLED | R4 FailErrorRollsBackAndKeepsToken (charge failure injected by a SQLite trigger on the account insert) |
| RM6 cap counts only now-1 | KILLED | R4 CapWindow |

## PostgreSQL (live-class), both local images, 0 errors
- InflightCapHolds PASS (both). 
- EvaluatedBeforeBlockBound PASS: worst evaluated before block over 20 rounds = 23 on postgres:15-alpine and 23 on postgres:15 (bound 23); Begin errors 0, Fail errors 0 (now asserted).
- ChargeEveryEvaluatedAndCapWindow PASS (new): after the block, 4 windows x evaluated 4 / charged 4 on both images; cap holds inside PendingTTL and expires after it.
- Honest range: round-2 author run measured 22 on one image, reviewer 23 on both, this run 23 on both; the observed maximum varies with timing, the bound is 23.

## UNCONFIRMED / not done
- The new PG F14 test was not run against the round-3 code (only the SQLite RED was captured).
- PG mutation runs of RM2b/others not done.
- F20/N1 doc fixes: header now says Fail is one atomic commit with no slots lock; API doc says cap = row lock, hand-off = atomic commit, measured 22-23; Clear comment mentions deadlock abort with Fail and the cross-instance clock-skew (> PendingTTL) limit.
- Nothing pushed.
