# identity: WP-12 PA-05 read-only SFTP client, evidence (round 1 + fix round fix-r2 + re-review WF24 + fix round fix-r3)
# head: fix-r2 is COMMITTED in the submodule `submodules/filesystem` at 83c0ac1f407f9e11044e97f4aa385ecfaf1cde21 (main repo pointer bump e9d6883d). fix-r3 (this round) is in the working tree of the submodule, UNCOMMITTED when this file was written (no commit, no push, no pointer bump by the fixer; the conductor commits). CORRECTION of a record: the commit message of 83c0ac1 / e9d6883d says "each client passed an independent one-pass review and one class-complete fix pass" - inaccurate when written (the round-1 review was NO-GO and the re-review had not run); the commit is not rewritten (11.4.113), this line is the correction.
# run_at: round 1 2026-10-07 (Sonnet worker); fix-r2 2026-10-07 (Sonnet single fixer); re-review WF24 2026-10-08 (Opus xhigh); fix-r3 2026-10-08 (Sonnet single fixer)
# independent review: round 1 DONE (WF19-REVIEW-sftp.md, Opus xhigh: NO-GO); round 2 DONE (WF24-REVIEW-sftp.md, Opus xhigh, of the COMMITTED fix-r2 tree: NO-GO, 4 MEDIUM source defects, 2 MEDIUM test-instrumentation, 1 MEDIUM process-doc, LOW items); round 3 (re-review of fix-r3): OWED, not performed
# verify evidence files:  `cd specs/001-full-project-audit-remediation/evidence/wp12/sftp && sha256sum -c SHA256SUMS`
# verify the sources the evidence is about (from the repository root):  `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/sftp/SOURCES.sha256`
#   (pkg/factory/factory.go and factory_test.go are shared with the FTP worker: their lines of SOURCES.sha256 are the state at the end of this round and will mismatch once that worker edits them again; pkg/sftp is the package under test)
# verify the package the GREEN runs tested (from the repository root):
#   `cd submodules/filesystem/pkg/sftp && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum`   must print the `# tested-pkg-sftp-sha256` of the fix-r3-green-run*.txt files (d86f6c6bb0a3...ec910) while the sources are unchanged (fix-r2: c31b5282...241e = the COMMITTED tree)
#   (the `# tested-tree-sha256` of the whole module differs between the three runs: pkg/ftp and pkg/factory are edited by another worker at the same time)

## Round 2 (fix-r2): the review's findings, one mechanism per class

The independent review of round 1 returned NO-GO. This round fixed every finding in ONE pass (constitution 11.4.276), after writing the convergence assessment `fix-r2-convergence.md` BEFORE the first edit
(classes A-H of the review, the one mechanism that closes each, the threat boundary). The mapping, finding -> mechanism -> the tests that pin it:

| Finding | Mechanism (class-wide) | Pinned by |
|---|---|---|
| SFTP-01 HIGH unbounded server calls | one guard around every blocking call (`with`/`run` for request-response, `file.call` for Read/WriteTo/Seek/Close): on context end the call gets `CancelGrace`, then the ssh connection is closed; `dial` keeps cancel hook and deadline until the sftp session is up and the root resolved; SSH keepalive; bounded file Close; `ListDirectory` via `ReadDirContext` | review probes P1 P2 P8 P11; `TestSeekEndHonoursContextOnHungServer`, `TestCloseIsBoundedOnAStalledServer`, `TestKeepaliveEndsACallOnAHalfOpenLink`, `TestKeepaliveCanBeDisabled`, `TestContextCancelEndsTheHandshake`, `TestDialTimeoutBoundsTheHandshake`, `TestCancelOfOneStreamDoesNotKillTheSharedConnection` (the guard does not take down healthy streams), `TestListDirectoryStopsAtTheContextOnASlowServer` |
| SFTP-02 TestConnection on a lost connection | `TestConnection` repairs the caller's connection in place (state up or lost) and uses a private connection otherwise | P3 |
| SFTP-03 Disconnect undone by a racing re-dial | the install of an implicit re-dial is conditional on the state still being "lost" under the lock (a `Disconnect` sets "down"; an explicit `Connect` that won meanwhile is reused) | P4 |
| SFTP-04 re-dial storm | single-flight `acquire` | P10, `TestConcurrentCallsAfterLossAreRaceFreeAndSucceed` |
| SFTP-05 STATUS EOF treated as connection loss | `serverStatus` wrapper for a reply; `isConnectionLoss` no longer includes `io.EOF`; `isConnectionLoss => IsTransient`; an `io.EOF` is checked with one keepalive round trip (`alive`) so that the write error of a dead channel (also `io.EOF` in `pkg/sftp`) is still a loss | P6, `TestClassification_LossImpliesTransient`, `TestStatusEOFIsNotAConnectionLoss`, `TestReadOnADeadConnectionIsNeverAnEOF`, `TestIsTransient_Table` (DNS not-found row) |
| SFTP-06 containment fails open | `checkWithin` fails closed (not-exist is returned as not-exist; any other `RealPath` error and an unknown root refuse) | P12, `TestContainmentFailsClosedButReportsNotExist`, `TestCheckWithinRefusesWhenTheResolvedRootIsUnknown` |
| SFTP-07 backslash rewrite | no separator translation in `confine` and `root` | P5, `TestConfine_Table` (the row that pinned the defect now pins the fix) |
| SFTP-08 panics on malformed replies | wire validator `frame.go` parses every reply frame completely before `pkg/sftp` reads it (covers the worker goroutines `pkg/sftp` starts itself, which `recover` cannot reach) + `recover` boundary | P7, `TestValidateFrame_Table`, `TestValidateFrame_NeverPanicsOnRandomInput`, `TestFrameReader_*`, `TestRaw_MalformedData*`, `TestRaw_MalformedReply*`, `TestProtectTurnsAPanicIntoAFaultAndClosesTheConnection`; interop with real servers: the OpenSSH run |
| SFTP-09 / SFTP-10 untestable claims | every reviewer mutant gets a killing test (table in `fix-r2-mutants.py`), the named weak tests are annotated and complemented (pipelining measured with a server-side gauge, positive factory test with an installed pin store) | see the mutation section |
| SFTP-11 docs refuted by measurement | `docs/testing/sftp-client.md` rev 2 and the package doc rewritten to the measured behaviour; the zeroing claim restated honestly | - |
| SFTP-12 keyboard-interactive | password also offered as keyboard-interactive; `isAuthFailure` also recognises the error a real OpenSSH produced for a wrong password (found by the integration run: `unexpected message type 51 (expected 60)`) | P9, `TestWrongPasswordIsAnAuthErrorEvenWhenKeyboardInteractiveIsAdvertised`, `TestKbdAnswer`, `TestIntegration_AuthFailureNotRetried` |
| SFTP-13 factory hygiene | secret-like keys rejected, empty `path` never wins, port validated | `TestDefaultFactory_SFTP_*` |
| SFTP-14 unbounded listing | `MaxDirEntries` budget counted by the wire validator | `TestRaw_ListingLimit`, `TestConn_ListingBudget` |
| SFTP-15 unpipelined ranged reads | `ranged.WriteTo` reads in windows | `TestRangedReadIsPipelinedToo` |
| SFTP-16 env name collisions | exact-case injective names, charset `[A-Za-z0-9_]` | `TestEnvCredentialResolver_NoCollisions` |
| SFTP-17 pin file integrity | mode/owner/symlink/directory checks, `flock`, directory fsync | `TestFilePinStore_*`, `TestTrustedOwner` |
| SFTP-18 protocol semantics / Discover ctx | documented limits; `discoverOne` honours the context | `TestDiscoverAllHonoursContextAfterConnect` |
| SFTP-19 evidence bookkeeping | this README's head line; the fixture prints and logs the sha256 of the tree it tested and refuses an undeclared `SFTP_FIXTURE_SRC` | `scripts/test-infra/sftp_fixture_gate_test.sh` (`fix-r2-gate-test.txt`) |

Found while fixing (not in the review), each with a test: (1) the pre-fix `Disconnect` closed the sftp session before the ssh connection; `pkg/sftp`'s `Close` waits for the dead connection, so a half-open link hung `Disconnect` while holding the client lock
(RED: `TestKeepaliveCanBeDisabled`); (2) `pkg/sftp` returns `io.EOF` for the write error of a closed channel, so a read or copy that ended on a dead connection looked like a clean end of file
(`TestReadOnADeadConnectionIsNeverAnEOF`); (3) a context that had already ended when `newFile` ran could call `Close` before the stop function was stored (nil function, data race; `TestNewFileWithAnAlreadyEndedContext`, mutant Q01);
(4) a real OpenSSH answers a wrong password followed by the keyboard-interactive start with a plain failure, which `x/crypto/ssh` reports as a protocol error (surfaced by `TestIntegration_AuthFailureNotRetried` in the first integration run of this round; the first run is kept in `fix-r2-history/`);
(5) the mutation run showed what an unbounded reply-frame length costs: with the length bound removed (mutant N07) the test binary was OOM-killed allocating about 4 GiB, so the bound is pinned by `TestFrameReader_LengthAndEOFHandling`; (6) it also showed two mechanisms of the first draft of the fix to be redundant (an intent counter next to the state check, a separate "down" case next to "state changed"): both were removed instead of being tested around.

### Results (all in the pinned `IMG-GO` image, rootless, through `scripts/containers/run_pinned.sh`)

| Run | Files | Result |
|---|---|---|
| pre-fix tree | `fix-r2-pre-sha.txt` | sha256 of the six sources equals the review's header |
| RED | `fix-r2-red.txt` | against the PRE-FIX client with only the new black-box tests and a behaviourless skeleton of the new symbols: part 1 `71 PASS, 32 distinct tests FAIL` (all 12 reviewer probes P1-P12 FAIL, as in the reviewer's own run; the 71 PASS are the unchanged round-1 suite and tests that pin a new mechanism or kill a mutant and hold on the old client too, e.g. the pipelining gauge); part 2, one process each: 3 tests CRASH the pre-fix test binary (`slice bounds out of range [:1048576] with capacity 32768` raised in a goroutine started by `pkg/sftp`, `index out of range [3] with length 0`), 2 keepalive/disconnect tests FAIL or hang (`Disconnect hung on a half-open link`, `a call on a half-open link stayed blocked > 10s`). White-box tests of new symbols cannot compile on the pre-fix tree (their RED is "the symbol does not exist"). |
| GREEN x3 | `fix-r2-green-run1.txt`, `-run2.txt`, `-run3.txt` | `scripts/test-infra/sftp_fixture.sh run --log ... -- -race -v ./pkg/sftp/ ./pkg/factory/` (OpenSSH 8.4p1 for the integration tests): 145 PASS, 0 FAIL, 0 `DATA RACE`, exit 0, three times; every log starts with `# tested-pkg-sftp-sha256: c31b5282...241e`, which equals the independent recomputation over `submodules/filesystem/pkg/sftp` of the checkout (the whole-module hash `4167ea05...` was identical in these three runs; in earlier runs of this round it differed from run to run because the FTP worker edited `pkg/ftp` meanwhile, which is why the package hash exists) |
| earlier fix-r2 runs kept for honesty | `fix-r2-history/` | `int1.txt`: the first integration run after the fix - 1 FAIL (`TestIntegration_AuthFailureNotRetried`: wrong password against real OpenSSH was a handshake error instead of `AuthError`; fixed in `isAuthFailure`, test `TestWrongPasswordIsAnAuthErrorEvenWhenKeyboardInteractiveIsAdvertised`); `unit1.txt`: the first unit run - 1 FAIL (`TestReview_P3`: the first fix classified a dead-channel `io.EOF` as a server reply; fixed with `alive()`) |
| mutation | `fix-r2-mutants-summary.txt`, `fix-r2-mutants-results.json`, `out/fix-r2-mut/` | 70 mutants (the reviewer's R01-R21, the author's M01-M11/M13/M14 re-targeted, and 36 for the new mechanisms) + negative controls Z00 (unmutated: suite passes) and Z01 (comment-only: survives). FINAL pass: 69 KILLED, 1 SURVIVED (Q01: a race-detector-only mutant of `newFile`; strengthened test kills it 3 of 3 times, `fix-r2-mutants-q01-final-reruns.json`), 0 INVALID. An earlier full pass on an earlier tree found 6 survivors and 6 invalid mutants (`fix-r2-mutants-pass1-results.json`); every one led to a test, a simplification of the code, or a corrected mutant (table in `fix-r2-mutants-summary.txt`). The reviewer's R15 ("equivalent") is killed too. M12 (integration-only) was not re-run: the code it mutates (`scfg.HostKeyAlgorithms = algs`) is unchanged and the OpenSSH run passes. The final pass ran on a tree that differs from the final tree by two test-only edits (Q01's test yields to the AfterFunc goroutine; `TestStuckRequestReturnsTheContextError` releases its stalled handler on failure) |
| fixture gate test | `fix-r2-gate-test.txt` | `scripts/test-infra/sftp_fixture_gate_test.sh`: 8 checks, 0 failed; with three mutants of the script (gate removed, whole-tree hash over a different file set, package hash over the wrong directory) the same test FAILS (2, 2 and 2 failed checks, `fix-r2-gate-test-mutants.txt`) |
| static | `fix-r2-static.txt` | `gofmt -l` empty for the package and the factory files, `go vet` clean (also `-tags integration`), `bash -n` of both scripts |

Process honesty (11.4.224(A)): as in round 1 the implementation of the fix was written before the new tests; the RED above is the new black-box suite against the unmodified client, not a record of tests that preceded the fix. The reviewer's probes, which did precede it, are the tests that carry the
original RED. The reviewer's file was adopted with the two deviations stated in the header of `review_probes_test.go` (private field access replaced by helpers; P6 creates the file `eofme` so that the STAT the probe is about is still reached now that the containment check fails closed).

### Findings not fixed or only partly fixed, and why (11.4.6)

* SFTP-18 TOCTOU between `RealPath` and `Open`/`Stat`: cannot be closed by this client; documented in the guide and the package doc.
* SFTP-18 absent size/permissions read as 0 and an epoch mtime reads as unknown: SFTP v3 gives the client no way to tell; documented.
* SFTP-01 consequence: cancelling a stuck call closes the shared connection; streams on it are not resumed. By design (the only way to end a blocked call), documented with the grace-period mitigation and a test that a healthy cancel does not disturb other streams.
* SFTP-17: the directory fsync after the rename is implemented but not observable by a test; ownership of a pin file by ANOTHER user cannot be created without privileges, so that branch is covered by a unit test with a fabricated `Stat_t` only.
* `ListDirectory` can still, in a microsecond window, accept a partial list from a connection that died at exactly the last reply (see the guide, honest limits).
* UNCONFIRMED (no NAS contacted): Synology keyboard-interactive requirement, `MaxSessions`, `@eaDir`, behaviour of its `REALPATH`.

## Round 3 (fix-r3): the findings of the WF24 re-review, one mechanism per class

The independent re-review (`WF24-REVIEW-sftp.md`, Opus xhigh, of the committed fix-r2 tree) returned NO-GO. This round fixed every finding in ONE pass (constitution 11.4.276), after writing `fix-r3-convergence.md` BEFORE the first edit
(classes C, L, B, E, F, T, D, the one mechanism that closes each, all members enumerated, the threat boundary). Every reviewer probe (N1 N1a N2 N3 N4 N5 N6 N7 N8 N10), discriminator (V01 V03 V04 V06 V07 V08 V09 V18 V27) was adopted as a permanent test,
byte-identical to the reviewer's file (`cmp` equal): `pkg/sftp/wf24_probes_test.go wf24_classify_test.go wf24_discriminators_test.go wf24_discriminators2_test.go`, `pkg/factory/wf24_authlatch_test.go wf24_factory_keys_test.go`.

| Finding | Mechanism (class-wide) | Pinned by |
|---|---|---|
| S01 MEDIUM a context error is a transport loss: a cancelled/expired `ListDirectory` on a healthy server tears down the shared connection | (a) `isConnectionLoss` returns false for any context error before the `net.Error` clause; (b) the drop decision is ONE pure function `callDisposition(ctxErr, err)` used by `run`: a call that returned the context error by itself leaves the connection alone (the guard drops one that is really stuck). The whole table of errors that can reach the call paths is asserted | `TestWF24_N1_*`, `TestWF24_N1a_*`, `TestClassification_TruthTable` (37 rows x invariants x drop decision), `TestClassification_LossImpliesTransient` |
| S02 MEDIUM the pool's rejected-login latch does not engage for the keyboard-interactive failure | typed contract: `*AuthError.Is(fabric.ErrAuth)` (precedent: `pkg/ftp` imports `pkg/fabric`); no text involved after the x/crypto error is recognised | `TestWF24_N5_*` (real pool, real factory, own ssh server, both shapes), `TestAuthContractWithFabric` (every exported sftp error through `fabric.Classify`), `TestWrongPasswordIsFabricAuthInBothShapes` |
| S03 MEDIUM `MaxDirEntries` is not per listing | per-HANDLE accounting by a `wireTracker` that parses the OUTGOING frames (READDIR id -> handle, CLOSE) and counts the incoming NAME frames against the handle; an over-budget page is replaced by a STATUS failure carrying a per-connection random marker, the connection stays up and only that listing fails with `ErrDirTooLarge` | `TestWF24_N2_*`, `TestListingOverBudgetFailsOnlyThatListing`, `TestOversizeListingDoesNotDisturbAStream`, `TestWireTracker_*`, `TestFrameReader_OverBudgetPageBecomesAStatusFailureNotAFault` |
| S04 MEDIUM the end-of-file check hard-codes 3 s | ONE liveness bound: `KeepAliveTimeout` for the periodic keepalive AND the end-of-file question (`Client.livenessTimeout()`); `probeTimeout` removed; the dead channel is checked first; documented | `TestWF24_N4_*`, `TestEndOfFileLivenessFollowsKeepAliveTimeout`, `TestLivenessAccessors` |
| S05 MEDIUM 11 surviving reviewer mutants | the reviewer's discriminators adopted verbatim; V02 and V15 made testable by accessors (`keepaliveInterval()`, `readWindow()`); V09 made deterministic by S09; the whole mutant set re-run on the final tree with negative controls | `TestWF24D_*`, `TestWF24D2_*`, `TestLivenessAccessors`, `TestReadWindow`; mutation section |
| S06 MEDIUM `sftp_fixture.sh run` exits 0 without running any test under `SFTP_FIXTURE_STOP_AFTER_HASH=1` | the hook is honoured in `selftest` only and exits 3; in `run` mode it is REFUSED (`stop_hook_only_in_selftest`) before anything is created; the complete list of the script's environment inputs is stated in its header | `sftp_fixture_gate_test.sh` (11 checks; `fix-r3-gate-test-red.txt` 4 FAIL on the pre-fix script, `fix-r3-gate-test.txt` 0 failed, `fix-r3-gate-test-mutants.txt` 5 mutants killed + negative control) |
| S07 LOW-MEDIUM `http.ServeContent` over `OpenSeekable` keeps 1 READ in flight | sequential read-ahead in `file.Read` (after two plain reads, doubling fills up to the read window, served from a buffer; Seek-aware; EOF/loss verified against the connection; never past the end of a bounded range) | `TestWF24_N7_*` (peak 8 READs in flight against 1), `TestReadAhead*`, `TestBoundedRangeNeverReadsBeyondItsEnd` |
| S08 LOW a server name `/` becomes an entry pointing at its own directory | `plausibleName` drops `/`, empty, NUL and names containing `/` | `TestWF24_N3_*`, `TestListingDropsImplausibleNames` |
| S09 LOW `file.Close` of a dead handle returns `failed to send packet: EOF` nondeterministically (6 of 40) | ONE helper `deadHandle` for `Close`; `file.call` maps the wrapped io.EOF of a closed channel to a connection loss for `Seek` etc. | `TestFile_CloseAndSeekOnADeadConnectionAreDeterministic` (40 of 40), `TestWF24D2_V09_*` |
| S10 LOW an explicit `Connect` in flight is installed after `Disconnect` returned | generation counter bumped by `Disconnect`; `Connect` installs only if unchanged, else `ErrDisconnected` | `TestDisconnectDuringConnectWins`, `TestWF24_N6_*` (observation kept) |
| S11 LOW factory secret refusal is spelling-exact | only the six keys the factory reads are accepted; secret-like keys (case-insensitive, separators removed) are refused with "use credential_ref"; unknown keys are refused | `TestDefaultFactory_SFTP_RefusesEverySecretSpelling`, `TestDefaultFactory_SFTP_RefusesUnknownKeysAndAcceptsTheReadOnes`, `TestWF24_N10_*` |
| S12 MEDIUM guide claims refuted | guide rev 3: six claims rewritten, "Corrections" table | `docs/testing/sftp-client.md` |
| S13 LOW stale headers, inaccurate commit message | headers state the committed state and the review status; the commit-message claim is corrected in this file and in the guide (the commit itself is not rewritten) | this file, the guide |
| S14 LOW undocumented limits | `MaxPacket` above 32768 is `ErrInvalidConfig` up front; the factory follows `sftp.DefaultPinStore` at connect time (`DefaultPinStoreRef`); the 4 MiB window cap is tested | `TestMaxPacketAboveWhatPkgSftpAcceptsIsRefusedUpFront`, `TestDefaultFactory_SFTP_FollowsAPinStoreInstalledAfterTheClientWasCreated`, `TestReadWindow` |

### Results of round 3 (all Go in the pinned `IMG-GO` image, rootless, through `scripts/containers/run_pinned.sh`)

| Run | Files | Result |
|---|---|---|
| pre-fix tree | `fix-r3-pre-sha.txt` | per-file sha256 of `pkg/sftp` and `pkg/factory` of the committed tree; the `pkg/sftp` directory hash `c31b5282...241e` equals the reviewed tree |
| RED part 1 | `fix-r3-red-part1.txt` | the reviewer's probes and discriminators adopted verbatim, run against the COMMITTED code (`-race`): 8 FAIL - N1 (a 4 MiB stream killed by a listing deadline), N1a, N2 (3 listings of 60 entries fail with a limit of 150 once another listing overlaps), N3, N4 (3.003 s, `connection lost`), N5 (latch missed, 4 login attempts), N7 (1 READ in flight), V09 (the flaky `Close`) -; 145 PASS, 0 `DATA RACE` |
| RED part 2 | `fix-r3-gate-test-red.txt` | the new gate test against the pre-fix fixture script: 4 of 11 checks FAIL (the run mode exits 0 without a test run). The white-box tests of fix-r3 (`fix_r3_test.go`, `frame_test.go`) need symbols that do not exist on the committed tree; their RED is "the symbol does not exist" (as in fix-r2) |
| GREEN x3 | `fix-r3-green-run1.txt`, `-run2.txt`, `-run3.txt` | on the FINAL tree: `scripts/test-infra/sftp_fixture.sh run --log ... -- -race -v ./pkg/sftp/ ./pkg/factory/` against a real OpenSSH 8.4p1 (`-tags integration`): 191 PASS, 0 FAIL, 0 `DATA RACE`, exit 0, three times; every log starts with `# tested-pkg-sftp-sha256: d86f6c6bb0a3...ec910` (and the same whole-tree hash `af0de74e...`), equal to the independent recomputation over `submodules/filesystem/pkg/sftp` of the checkout. `fix-r3-history/` keeps (a) `green-run*-tree-T1-pkg-d2ccd188.txt`: the first GREEN x3 on the tree T1 (188 PASS each) - the mutation run then found survivors (below), which changed two source lines and a test, so GREEN was repeated on the final tree T2; (b) `green-run{1,2}-ftp-worker-midflight.txt`: two earlier attempts where `pkg/sftp` was green (186 PASS) but `pkg/factory` had 2 FAIL in the FTP tests (`TestFTPFactory_*`) because the concurrent FTP worker was editing `pkg/ftp` at that moment (unrelated to this round: the same tests PASSED with the identical `pkg/sftp` and `factory.go` in the next attempt) |
| mutation | `fix-r3-mutants.py`, `fix-r3-mutants-summary.txt`, `fix-r3-mutants-pass1-results.json`, `fix-r3-mutants-final-rerun-results.json`, `out/fix-r3-mut/{pass1,final}/` | 115 mutants + negative controls Z00 (unmutated: suite green) and Z01 (comment-only: survives): the round-1 reviewer's R01-R21, the fix-r2 author's M*/N*/L*/Q*/P*/E*/F*/D*/K* re-targeted, the round-2 reviewer's V* (V01 V02 V03 V04 V06b V07 V08 V09 V10 V11 V12 V15 V16 V18 V27b V38), and X01-X29 (one per fix-r3 mechanism and per defect put back). PASS 1 on the tree T1: 110 KILLED, 4 SURVIVED (F01 X19 X24 X29), 1 INVALID (X04). The survivors led to: a stronger factory test (F01 X24: the "unknown setting" refusal also names `credential_ref`, so the old assertion could not tell a secret from an unknown key), two EQUIVALENT-code simplifications (X19: the `n != "/"` term of `plausibleName` was subsumed by the `/` containment test; X29: two terms of `IsTransient` were subsumed) with the mutants re-pointed at the remaining clauses, and a corrected mutant (X04: an unused import). The affected mutants and the controls were re-run on the final tree T2: Z00 survives, Z01 survives, the other 10 are KILLED. The reviewer's V13/V13b (fix-direction probes) are not re-run: their effect is the code now (X01-X03 put the defect back and are killed). The full pass 1 took 6 containers of 20 mutants; the first attempt (one container per mutant) was abandoned because the host admits a container only when memory is free and the tmpfs scratch of another worker shared the name `/dev/shm/fixr3` (it overwrote my log and cache; nothing of the evidence was lost, the run was repeated from scratch under a private directory and the cache moved to disk) |
| fixture gate test | `fix-r3-gate-test.txt`, `fix-r3-gate-test-mutants.txt` | 11 checks, 0 failed; five mutants of the script (run-mode refusal removed, hook exits 0 again, hook refused in selftest, refusal reason renamed, the fix-r2 `SFTP_FIXTURE_SRC` gate removed) are each caught (1, 3, 6, 1 and 2 failed checks); the unmutated mini-root copy passes all 11 |
| static | `fix-r3-static.txt` | on the final tree: `gofmt -l` empty, `go vet` clean (also `-tags integration`), `bash -n` of both scripts, the adopted reviewer files byte-identical |

Process honesty (11.4.224(A)): as in fix-r2 the fix was written before the NEW white-box tests; the RED above is the reviewer's own tests (which preceded the fix) against the committed code, and the new fixture gate test against the pre-fix script. A first mutation draft of the
reviewer mutants V13/V13b (fix-direction probes) is not re-run: they ARE the shape of the fix now in the code, and X01/X02/X03 put the defect back.

### Findings not fixed or only partly fixed, and why (11.4.6)

* S07 is fixed for sequential streams; random access that never reads two blocks in a row still pays one request per read (no read-ahead is started), by design. The read-ahead costs up to one read window of memory per sequentially read open file and server reads beyond the reader's position (documented).
* Observed while fixing S02, NOT part of it and NOT changed (`pkg/fabric` is another package, edited concurrently by another worker): reading `pkg/fabric/errors.go` `Classify` (state at the time of writing), it has no clause for `gosftp.ErrSSHFxConnectionLost` (what `pkg/sftp` returns when the connection dies mid-call), whose text carries no marker, so it would read as `ClassPermanent`; the sftp client retries such errors itself before they leave the package. UNCONFIRMED by a test; reported for the fabric owner.
* The factory now REFUSES every key it does not read (S11 fix direction). A caller that passes extra keys (`name`, `timeout` ...) to the sftp protocol must stop doing so; `catalog-api` is not wired to this factory yet (its own "sftp" protocol registration is separate), so nothing breaks today.
* UNCONFIRMED (no NAS contacted): whether Synology DSM advertises keyboard-interactive the way the fixture's OpenSSH does (S02 shape), the latency of its global-request answers under load (S04: now bounded by the owner's `KeepAliveTimeout`), the scanner's real listing overlap (S03).
* The independent re-review of fix-r3 is OWED.

## What was built (round 1 list, updated)

| Path (repo root) | Content |
|---|---|
| `submodules/filesystem/pkg/sftp/sftp.go`, `frame.go`, `hostkey.go`, `credential.go` | the client (`client.Client` + `client.SeekableClient`), the reply validator, host key pinning, credential_ref |
| `submodules/filesystem/pkg/sftp/*_test.go` | unit tests against a REAL in-process ssh server with the real pkg/sftp request server (read-only handlers) and a raw sftp peer; `review_probes_test.go` = the reviewer's probes P1-P12; `fix_r2_test.go`, `fix_r2_internal_test.go`, `frame_test.go`, `pinstore*_test.go`, `wire_helpers_test.go`, `helpers_test.go` = fix-r2; `fix_r3_test.go` and the adopted reviewer files `wf24_*_test.go` = fix-r3 (`frame_test.go` extended for the per-listing tracker) |
| `submodules/filesystem/pkg/sftp/integration_test.go` (`-tags integration`) | tests against a real OpenSSH 8.4p1 (`internal-sftp`, chroot, read-only volume) |
| `submodules/filesystem/pkg/factory/factory.go`, `factory_test.go`, `fix_r3_test.go`, `wf24_*_test.go` | factory protocol `sftp` (rejects inline secrets, `credential_ref` only, validated port, empty path never widens the root) |
| `submodules/filesystem/go.mod`, `go.sum` | `github.com/pkg/sftp v1.13.11`, `golang.org/x/crypto v0.54.0` (direct), `github.com/kr/fs v0.1.0`, `golang.org/x/sys v0.47.0` (indirect). NOTE: the FTP worker (PA-04) edited the same go.mod/go.sum concurrently (ftp v0.2.4, testify v1.12.1); the file holds both sets of changes. fix-r2 did not touch them. |
| `docker-compose.test-infra.sftp.yml`, `scripts/test-infra/sftp_fixture.sh`, `scripts/test-infra/sftp_fixture_gate_test.sh` | the fixture (new files) and the test of its own gates |
| `build/containers/images.lock.yaml` | one entry appended: `IMG-INFRA-SFTP` |
| `docs/testing/sftp-client.md`, `docs/scripts/sftp_fixture.md` | docs (revision 2) |

## Image pin (digest recorded with `scripts/containers/resolve_pin.sh`, read from podman, not typed)

`docker.io/atmoz/sftp:latest` -> `sha256:0960390462a4441dbb63698d7c185b76a41ffcee7b78ff4adf275f3e66f9c475` (index and platform digest identical, a single-platform image), 171983326 bytes,
resolved_at 2026-10-07T17:26:43Z on this host, `class: service`, `signature_status: UNKNOWN`. Pulled with rootless podman after `scripts/containers/disk_headroom.sh`. The tool wrote a clean
11-line append (verified by a diff against a copy before it was applied). `scripts/containers/check_pins.sh` reports 49 violations in the repository, all pre-existing in other files; none is in the files of this task.
Pinned Go image used for every test: `IMG-GO` (digest in the lock, `go1.25.14`).

## Round 1 results (historical; the round-1 tree no longer exists, the files stay as the record the review examined)

| Run | Files | Result |
|---|---|---|
| RED | `red.txt` (+ `out/red/red-raw.txt`, `out/red-summary.json`) | against a copy whose behaviour is stubbed (list in `mutate_sftp.py` `RED`): 32 distinct tests FAIL |
| GREEN x3 | `green-run1.txt`, `green-run2.txt`, `green-run3.txt` | `go test -race -v ./pkg/sftp/ ./pkg/factory/` with `-tags integration` against OpenSSH: 66 PASS, 0 FAIL, exit 0, three times |
| mutation | `mutations.txt`, `out/mut/`, `out/neg/` | 14 self-written mutants, 14 KILLED, 0 survived, 0 invalid; negative controls N01 (unmutated, passes) and N02 (comment-only edit, survives) behave as required; M12 survives the unit suite and is killed only by the OpenSSH run. (`mutate_sftp.py` still targets the round-1 text; its fixture call now also sets `SFTP_FIXTURE_ALLOW_SRC=1`; the same 14 intents are re-targeted as `M01`-`M14` in `fix-r2-mutants.py`.) |

The round-1 claims in this README that the review refuted were: "Context cancellation ends a read mid-stream" (only the between-reads case was tested), "reviewer-authored mutations are owed" (done by the review: 21 mutants, 16 genuine survivors), and the head line (`65e5f339` while the runs recorded `4c8b07b7`).
They are corrected in this revision; round-1 `green-run*.txt` and `red.txt` are not rewritten.

## What the tests prove (observed behaviour, not configuration)

* Listing with server attributes: the seeded mtime (2020-05-17 10:30:00 UTC) comes back from OpenSSH unchanged; an absent mtime is the zero time (unit, M06).
* Whole-file read through the pipelined `WriteTo` path (1 MiB, sha256 equal), ranged read (offset/length), tail read after `Seek(-100, End)`; pipelining measured by a server-side gauge of concurrent READs (>= 3 in flight), for `ReadFile` and for bounded `ReadRange`.
* Unknown host key: refused, the password was never offered (server counted 0 auth attempts). Pin without the owner's matching fingerprint: refused, nothing recorded. Changed key: refused, 0 auth attempts. Not retried (1 connection).
* Wrong password: `AuthError`, exactly one connection (unit, also with keyboard-interactive advertised) / no backoff elapsed with a 3 s base (OpenSSH). Wrong key: same. No secret in any error.
* Writes: every mutator returns `ErrReadOnly`; nothing created or deleted on the server; AND a raw sftp session of the same user is refused by the read-only volume (defence in depth).
* Path confinement: `..` refused, symlink to outside the root refused after server-side resolution, symlink that resolves back inside allowed, a path the server cannot resolve is refused (fail closed), a backslash in a name is an ordinary byte.
* Context: a call blocked on a server that never answers STAT, READDIR, READ, CLOSE or the sftp INIT returns the context error after the grace period (measured in the GREEN logs: P1 1.302 s for a 300 ms context + 1 s grace, P8 1.001 s after the cancel, P11 1.302 s, P2 301 ms); a half-open link ends calls through the keepalive; a healthy cancel does not disturb other streams on the connection.
* Re-dial: 8 concurrent calls after one connection loss open 1 new connection (P10, was 8); a `Disconnect` racing a re-dial wins (P4); a server STATUS EOF costs 0 re-dials and the open 4 MiB stream completes (P6, was 3 re-dials and a broken stream).
* Malformed replies: a 4-byte STATUS, a DATA frame whose length field lies (also on the pipelined path), over-long frames and unknown packet types end the connection with `ErrMalformedReply`; real OpenSSH and `pkg/sftp` replies are accepted (the integration run).
* Transient connect failures (dropped TCP connections) are retried to a bound (3 attempts for 2 drops; gives up at 1+2); a killed live connection is re-dialled.

## UNCONFIRMED / not done

* No throughput measurement in MB/s (pipelining is shown by concurrency). No chaos/fuzz lane beyond the random-input test of the frame validator (30,000 inputs, no panic) and the table tests for confinement; no `go test -fuzz`.
* Not contacted: any NAS. Synology SFTP behaviour (chroot, `@eaDir`, `MaxSessions`, password SFTP enabled, keyboard-interactive) is UNCONFIRMED.
* The pin store used by the application is not wired: `sftp.DefaultPinStore` is nil until the application installs one (a factory-built client refuses every host until then). Owner-confirmation UI/API is not part of this task.
* Race detector: run with `-race` in all three GREEN runs (CGO enabled in IMG-GO). No stress run.
* The independent re-review of fix-r2 (11.4.142 / 11.4.209) is OWED. This round's reviewer-adopted mutants and probes are the reviewer's, but the mutation harness `fix-r2-mutants.py` and the new tests were written by the fixer.
* Disk-headroom records of the round-1 work are in `../../disk/` (`sftp-pull.json`, `sftpmut-*.json`, `catalogizer-sftp-*.json`), outside this directory.

## catalog-api go.mod (NOT touched, as instructed): bumps it will need to consume the new client

`catalog-api/go.mod` already has `golang.org/x/crypto v0.54.0` and `golang.org/x/sys v0.47.0` (both match what `pkg/sftp v1.13.11` needs, in the file at the time of this run). It lacks
`github.com/pkg/sftp v1.13.11` and `github.com/kr/fs v0.1.0` (indirect); `go.sum` needs their entries (`go mod tidy` inside the Go container once the submodule pointer is bumped; the go.mod `replace digital.vasic.filesystem => ../submodules/filesystem` is already in place).

## Re-running

```bash
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
scripts/test-infra/sftp_fixture.sh run --log /dev/shm/sftp.txt -- -race -v ./pkg/sftp/ ./pkg/factory/        # unit + OpenSSH integration; the log's first line is the tested-tree hash
bash scripts/test-infra/sftp_fixture_gate_test.sh                                                                # the fixture script's own gates, no container
python3 -I specs/001-full-project-audit-remediation/evidence/wp12/sftp/fix-r2-mutants.py check                   # every mutation pattern matches the checkout exactly once
python3 -I specs/001-full-project-audit-remediation/evidence/wp12/sftp/fix-r2-mutants.py run /dev/shm/mut       # all mutants (~1 h)
```
