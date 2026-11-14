# CAT-LOGIN-LOCK-2 round 5 evidence (PROVISIONAL, final round R_max=5)

Answers review round 4 (NO-GO on RV4-1; F14 confirmed closed by the reviewer). SQLite = unit-class; PostgreSQL = live-class (real PG 15, local images only). Not independently reviewed yet.

## Changes
- RV4-1: reviewer's TestRV4_BlockedChargeExtendsBlockByDoubling adopted unchanged (login_throttle_rv4_test.go). My old test renamed TestInvariant_R4_ChargingBlockedAccountIsMonotoneAndCapped and its comment no longer claims doubling.
- RV4-2: TestRV4_BlockedChargeNeverShortensAfterDecay adopted unchanged. AccountDecay < Cap is documented as safe (max rule) in the Clear/clock comment; withDefaults not changed.
- RV4-3: "byte-order" wording corrected in API doc, login_throttle_pg_test.go header, testpg.go: both images measured datcollate=en_US.utf8; no C-collation image exercised.
- RV4-5: API doc and code comment state "never longer than Cap from now" holds under ONE clock only.
- Extra: TestInvariant_R4_DelayForLiteralValues (literal delayFor values) because the RV4 tests use delayFor as their own oracle; it kills M3 and M4.

## Results
- green_sqlite.txt: 26 PASS, 0 FAIL (`-skip Postgres -run 'Throttle|Invariant|Begin|RV4'`). vet.txt: `go vet -buildvcs=false ./...` exit 0. gofmt clean on touched files (playback_session_repository_test.go unformatted on base, untouched).
- PG (pg_both_images.txt, disk 23G before, 26G after; threshold 12G): 3 tests PASS on both images, 0 Begin/Fail errors; worst evaluated before block: 22 (alpine) and 23 (Debian), bound 23.

## Mutants (SQLite suite, slow model tests skipped for mutation runs)
| Mutant | Result | By |
|---|---|---|
| M1 drop max() | KILLED (red_M1.txt) | RV4 NeverShortensAfterDecay |
| M7 keep old end | KILLED (red_M7.txt) | RV4 ExtendsBlockByDoubling |
| M2 bucket charges blocked | KILLED | BucketRefusalReleasesToken |
| M6 extra increment | KILLED | R4 EveryEvaluatedCharged, R4 Monotone |
| M3 extra doubling | KILLED after new test (survived before) | R4 DelayForLiteralValues |
| M4 cap doubled | KILLED after new test (survived before) | R4 DelayForLiteralValues |
| RM1,RM2,RM3,RM4,RM5,RM6 | KILLED (mutants_RM.txt) | as round 4 |
| RM2b | SURVIVED | not testable on SQLite |
| M5 (Fail statements not in the same tx) | NOT RUN here | UNCONFIRMED |

## UNCONFIRMED
M5 same-transaction coupling and RM2b were not mutation-run on PostgreSQL. M3/M4 survival before the new test was observed in this round's runs (mut_M3/M4.txt were overwritten by the post-fix runs). Nothing pushed.
