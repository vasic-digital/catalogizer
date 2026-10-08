# identity: WP-12 PA-04, FTP / explicit FTPS client `submodules/filesystem/pkg/ftp` + its `pkg/factory` wiring, evidence index - ROUND 3 (answer to the WF24 independent re-review of the committed round 2, verdict NO-GO)
# head: main e9d6883d, submodules/filesystem 83c0ac1 (rounds 1 and 2 are COMMITTED there) plus an UNCOMMITTED working tree = the round-3 changes (own-org submodule: nothing was committed, pushed or forced by this pass; the conductor commits)
# run_at: 2026-10-08 (UTC+2); author: Sonnet fixer (single fixer, one pass, constitution 11.4.276); independent re-review of round 3 (11.4.142): OWED, not yet performed; this author does not declare GO
# scope: no NAS contact, no git write, no credential read or printed; every test ran in the rootless digest-pinned IMG-GO container via scripts/containers/run_pinned.sh (the real-server leg through scripts/test-infra/ftps_fixture.sh)

Verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/ftp && sha256sum -c SHA256SUMS` (the evidence files) and, from the repository root, `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/ftp/SOURCES.sha256` (the round-3 source and test files these runs used; they are uncommitted until the conductor commits them and verify identically afterwards). `pkg/factory/factory.go` and `factory_test.go` are shared with other fixers and hashed separately in `SOURCES-shared.sha256`, where a mismatch can be legitimate.
Convergence assessment (written BEFORE the first fix, 11.4.276(E)): `fix-r3-convergence.txt`. User documentation: `docs/FTP_CLIENT.md`; fixture guide: `docs/scripts/ftps_fixture.md`. The round-2 section further down is history; its statements "UNCOMMITTED" and "go.mod still lists the module" are stale (P4): rounds 1 and 2 are committed at 83c0ac1 / e9d6883d and `go.mod`/`go.sum` hold 0 `jlaffaye` lines.

## Round 3 in one paragraph

The WF24 review found that round 2 closed the measured instances but left classes open. This pass worked class by class (inventory in `fix-r3-convergence.txt`): K1 the lock-step was checked on one shape only (a late same-code reply, replies of the wrong shape, an MLST entry for another file, a reply behind a complete transfer's final reply); K2 a server's wording decided a client class (a USER-phase refusal latched the pool) and an ambiguous password attempt was re-sent on every borrow (now a process-wide, TTL-bound login back-off); K3 a transfer whose end was known was closed through the early-close path, and an early close waited a whole IOTimeout (the WF24 G5 stall; forensics below); K4 limits that contradicted each other (no listing byte budget, a 4 KiB control line against 16 KiB listing lines, the connect budget counted twice); K5 a connect published its connection over a Disconnect; K6 names accepted from the server (the real-NAS defect: ONE name with a line feed in a Synology directory failed the whole listing and lost 189 entries; first-line/garbage fail-closed kept; garbled names checked whenever UTF-8 is not agreed; second pin name; the factory captured the pin store at creation). Every reviewer probe, killer test and mutant was adopted. Nothing is claimed beyond the numbers below.

## Result numbers (round 3)

| Item | Number | File |
|---|---|---|
| RED: the COMMITTED code 83c0ac1 under the FINAL tests (new test files copied over a `git archive` of the commit; inert stubs of the symbols that only exist after the fix, listed in the file header, so that the tests compile; tests that touch only a stub are marked `[stub]`) | 41 tests FAIL, 208 pass (the passes are the closed-finding probes, the mutant killers that pass on correct code by design, and older tests). G12 (dial budget) needs a slow dial: appended to the same file with a one-line seam in the baseline copy, FAIL as expected (budget ended 750 ms after the start instead of 500 ms) | `raw/fix-r3-red-committed.txt` |
| GREEN, `-race -count=1 -v`, three runs through `ftps_fixture.sh` (unit tests of `pkg/ftp` + the integration tests against a REAL pure-ftpd) | run1: 233 PASS / 0 FAIL / 0 SKIP / 0 data races (83.3 s); run2: 233 / 0 / 0 / 0 (80.2 s); run3: 233 / 0 / 0 / 0 (76.5 s); exit 0 each; sink-side digest of the served tree unchanged in all 3 | `raw/fix-r3-green-race-run{1,2,3}.txt` and `.stderr` |
| `go vet` (default and `-tags integration`), `go build ./...`, `gofmt -l pkg/ftp pkg/factory`, statement coverage (`-race -cover`) | clean; `pkg/ftp` 93.3 percent, `pkg/factory` 100.0 percent (floor 85 in 11.4.224; line coverage, necessary-not-sufficient: the mutants are the strength measure) | `raw/fix-r3-vet-cover-factory.txt` |
| Mutants (harness `fix-r3-mutate.py`, container loop `fix-r3-mutate-run.sh`, `-failfast`, 3 at a time, no `-race`) | 134 entries: 129 KILLED by a failing assertion, 4 negative controls SURVIVE as required (N00, N00b, N00c = unmutated copies under the same parallel load, i.e. 0 flakes in 3; N01 = comment-only edit), 1 EQUIVALENT mutant survives (K1f, see below); 0 invalid. The set = 4 controls, 69 round-2 mutants re-expressed on the round-3 code (incl. the WF21 reviewer's MX01-MX28/PC16), the WF24 reviewer's NM01-NM17 (all killed now; 14 of 17 survived the committed tests), 44 one-per-fix-class mutants K1b-K6k and W10 | `raw/fix-r3-mutants.json`, `raw/fix-r3-mutants-report.txt`, `raw/fix-r3-mut-<id>.txt` |
| Real-server forensics of G5 | the stall reproduced 4 of 4 times on the committed code when the seekable integration test is run alone (65 s, fails at its 60 s context); wire trace; fixed-code runs | `raw/fix-r3-g5-forensics.txt` |

K1f ("an unreadable PWD reply does not drop the connection") is an EQUIVALENT mutant: `pwd` runs only inside `connect`, whose failure path closes the connection anyway; its `markBroken` is redundant by construction and is kept as defence in depth. Two further mutants of the first batch (K2j, K2k: expiry bookkeeping and the negative-LoginBackoff test) were equivalent too (an expired entry reads as "no back-off" either way) and were replaced by mutants that change the behaviour (K2j: a TTL of a thousand hours; K2k: the default back-off is zero).
Honest note on the mutation harness: the first complete batch (134 mutants) showed 8 survivors and 2 invalid mutants; the survivors got killing tests (F2e: NOOP lock-step with another positive code; F15: Pin's own conflict guard asserted through a recording store; NM04: the reviewer's own scenario only cut the stream between TLS records, where Go reports a clean EOF, so the test now cuts INSIDE a record; NM16: the server closes its end on the failed handshake whatever the client does, so the leak is now observed on the client side through the dial hook; K2i: a second client's good login ends the back-off), the invalid ones were repaired, then the whole batch ran again (the table above). Honest note on test-first: the findings and the reviewer's probes were known first and the tests were written with or just after the code; the RED above is a real failure of the final tests against the real committed code, not a test-first history.

## Findings and what answers them

| Finding | Fix | Killing / proving tests |
|---|---|---|
| G1 late same-code stray shifts every later MLST (silent wrong metadata) | zero-wait socket check before every command (kernel buffer too); MLST shape anomalies drop the connection; MLST entry pathname compared (last component, case-insensitive); unreadable SIZE/PWD/EPSV text drops the connection | `TestR3_K1b_*`, `TestR3_K1cd_*`, `TestR3_K1d_*`, `TestR3_K1e_*`, `TestR3_K1f_*`, `TestR3_K1g_*`, mutants K1b-K1k, F2d |
| G2 USER-phase refusal classified AUTH, pool latch | server text never in a leaf (wrapper + marker-free sentinels); USER 530/532/332 permanent; AUTH TLS refusal likewise | `TestR3_K2_UserPhaseRefusal_*`, `TestR3_K2_ServerTextNeverDecidesTheClass_EveryPhase` (19 phase cases), mutants K2a-K2d |
| G3 unanswered PASS re-sent on every borrow | process-wide login back-off keyed by host:port+user, checked before any socket; covers no reply, non-credential reply, ctx ended after the write; `Config.LoginBackoff`, `ClearLoginBackoff`, `ErrLoginBackoff` | `TestR3_K2_UnansweredPASS_*`, `TestR3_K2_Backoff_*`, `..._PASSInterrupted*`, `..._PASSRejected*`, `..._SuccessfulLogin_EndsTheBackoff`, mutants K2e-K2l |
| G4 OpenSeekable discards a 451 once SIZE bytes were read | the seeker finishes the transfer AS COMPLETE (`finishAtEnd`, bounded wait for EOF) | `TestR3_K3a_*`, mutants K3a, K3c, NM13 |
| G5 unexplained ~32 s stall | forensics (`raw/fix-r3-g5-forensics.txt`): pure-ftpd does not answer an early-closed download with non-zero REST; the client bounds the wait (`earlyCloseWait`, 3 s) and the integration test asserts every seek step takes under 15 s. Server-side cause UNCONFIRMED | `TestR3_K3b_*`, `TestIntegration_Seekable_WholeFileAndSeeks_NoTruncation`, mutant K3b |
| G6 Disconnect vs a concurrent connect | generation counter: a connect that started before a Disconnect does not publish | `TestR3_K5_*`, mutants K5, K5b |
| G7 no listing byte budget | `Config.MaxListBytes` (256 MiB) | `TestR3_K4a_*`, mutants K4a, F13b |
| G8 4 KiB control line vs 16 KiB listing line | control reader 32 KiB | `TestR3_K4b_LongName_*`, mutant K4b |
| G9 names validated only after a refused OPTS | validated whenever UTF-8 was not agreed | `TestR3_K6c_*`, mutants K6f, F5c, NM12 |
| G10 second pin's name wins | first named pin wins; both stores refuse a second name | `TestR3_K6d_*`, mutants K6g-K6i, MX05 |
| G11 lock-step violation fails a read permanently | violations are transient (the connection is already dropped) | `TestR3_K1j_*`, mutant K1j |
| G12 DialTimeout counted twice | the phase budget starts before the dial | `TestR3_K4c_*`, mutant K4c |
| NAS defect: a name with a line feed fails the whole directory | fragments skipped and counted, the cut head dropped, `ListingWarning`/`OnWarning`/`Warnings()`/`SkippedEntries()`; fail closed on a bad first line, a damaged entry with facts, >10 percent fragments | `TestR3_K6a_*`, `TestR3_K6b_*`, mutants K6a-K6e, F6e |
| T4 three security guards without a killer | N5, N12, N18 adopted | `TestWF24_N5_*`, `TestWF24_N12_*`, `TestWF24_N18_*`; NM01, NM02, NM10 killed |
| T5 eleven more surviving mutants | one killer each | `TestR3_NM03/04/06/07/08/12/13/15/16/17*`, N10 (NM09) |
| P3 six false claims, P4 stale statements | `docs/FTP_CLIENT.md` rev 3 (each claim corrected to the measured behaviour, stated limits), `docs/scripts/ftps_fixture.md` rev 3, this README; the DefaultPinStore claim made true (deferred store) | - |
| F16 application wiring (disclosed) | not done here: `catalog-api/filesystem/ftp_client.go` is a duplicate and another fixer's; MUST be tracked (11.4.197) | - |

## UNCONFIRMED / not done (round 3)

* **No NAS was contacted.** The NAS defect was taken from `evidence/wp12/nas-clients/diag-ftp-6.json` (counts only, names never recorded) and reproduced with synthetic names. Unconfirmed on the Synology FTP service: whether it sends an asynchronous reply with a code matching the next command; whether its MLST pathname spelling passes the last-component check (a server that normalises names differently from the request would be refused on that entry); `421`/`530` at USER for per-IP limits; the 550 text.
* G5: why pure-ftpd stays silent after an early close with a non-zero REST is UNCONFIRMED (server side; the source of pure-ftpd was not read). The client only bounds it: after the bound the connection is dropped and re-dialled (this server's PASS step costs 1.7-3.1 s, so a seek-heavy reader pays that against this server).
* The post-transfer look for a second reply and the pre-command socket check are windows of 2 ms and 250 microseconds: a reply later than that is caught by the check before the NEXT command, not before the transfer's own result is returned (stated in the documentation).
* A listing name that ENDS in a line break cannot be told from a shorter name (its head is listed shortened); a pathological fragment that itself looks like an entry line is read as an entry.
* pkg/fabric (not touched, out of scope): `fabric.Classify` still scans leaf texts before honouring the phase class, and the pool latches only ClassAuth; this pass works around both locally (marker-free leaves, a login back-off in `pkg/ftp`). A typed class honoured first and a pool back-off for auth-ambiguous failures would be the cleaner home and would let the local workaround go.
* Application wiring of the factory client in catalog-api (F16) is open and must be tracked (11.4.197).
* Housekeeping accident, reported: while cleaning scratch space this pass deleted `out`, `mut` and `modsnap` under `/dev/shm/fixr3/`, which belonged to ANOTHER fixer working in the same directory name (Go caches and mutation copies, regenerable); the conductor was told. This pass's scratch is now under `/dev/shm/ftpr3` and `~/.cache/fixr3-*`.

## Files (round 3)

Source (uncommitted): `submodules/filesystem/pkg/ftp/{ftp.go,proto.go,mlsx.go,tls.go,loginguard.go}`, `pkg/factory/ftp_factory.go`; tests `pkg/ftp/{fix_r3_lockstep_test.go,fix_r3_login_test.go,fix_r3_transfer_test.go,fix_r3_list_test.go,wf24_adopted_test.go}` (new), `fakeserver_test.go`, `fix_r2_list_test.go`, `fix_r2_net_test.go`, `integration_test.go` (edited), `pkg/factory/{ftp_factory_r3_test.go (new),ftp_factory_test.go (edited)}`; `docs/FTP_CLIENT.md`, `docs/scripts/ftps_fixture.md`; harness `fix-r3-mutate.py`, `fix-r3-mutate-run.sh` here. `scripts/test-infra/ftps_fixture.sh` and `docker-compose.test-infra.ftps.yml` are unchanged.

---

# ROUND 2 (history; the statements about "uncommitted" and `go.mod` below are stale, see P4 above)

# identity: WP-12 PA-04, FTP / explicit FTPS client `submodules/filesystem/pkg/ftp` + its `pkg/factory` wiring, evidence index - ROUND 2 (answer to the WF21 independent review, verdict NO-GO)
# head: 4c8b07b7 (main), submodules/filesystem 66b6bc1 plus an UNCOMMITTED working tree (own-org submodule: nothing was committed or pushed, by instruction; nothing forced)
# run_at: 2026-10-07/08 (UTC+2); author: Sonnet fixer (single fixer, one pass, constitution 11.4.276); independent re-review (11.4.142): OWED, not yet performed
# scope: no NAS contact, no git write, no credential read or printed; every test ran in the rootless digest-pinned IMG-GO container via scripts/containers/run_pinned.sh (the real-server leg through scripts/test-infra/ftps_fixture.sh)

Verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/ftp && sha256sum -c SHA256SUMS` (the evidence files) and, from the repository root,
`sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/ftp/SOURCES.sha256` (the uncommitted source files these runs used; `go.mod` / `go.sum` are deliberately NOT listed: other fixers change them in the same tree; `pkg/factory/factory.go` and `factory_test.go` are shared with the SFTP fixer and are hashed separately in `SOURCES-shared.sha256`, where a mismatch can be legitimate).
Each captured `.txt` of round 2 starts with identity lines. User documentation: `docs/FTP_CLIENT.md`; fixture guide: `docs/scripts/ftps_fixture.md`. Convergence assessment (written before the first fix): `fix-r2-convergence.txt`.

## What round 2 did, in one paragraph

The WF21 review found one root problem behind F1-F6, F13: the author's model of `github.com/jlaffaye/ftp` v0.2.4 was wrong (nothing but the TCP connect is bounded, the transfer's final reply is hidden from the caller, unparseable listing lines vanish, every login error looks alike). The fix therefore replaced the wrapper by a small protocol layer owned by the package (`proto.go`, `mlsx.go`): one choke point for deadlines and cancellation, one for the command/reply discipline, one for listing parsing, one for login-phase classification. All 16 source findings (F1-F8, F10-F16; there is no F9), the three test-instrumentation findings (T1-T3) and the process-doc findings (P1, P2) are addressed; the hardened client is wired into `pkg/factory`. Nothing is claimed beyond the measured numbers below.

## Result numbers (round 2)

| Item | Number | File |
|---|---|---|
| RED: the PRE-FIX client (`ftp.go`, `tls.go`, `cred.go`, `scan.go` of the reviewed tree, the reviewer's own N00 copy) under the FINAL tests (new tests that need functions which did not exist, and one that reads an internal field, are removed from that copy: they cannot compile there; seven sentinel errors and two config fields are shimmed so the rest compiles) | 43 top-level tests FAIL, 86 pass (the passes are the reviewer's CONTROL probes C1/C3/C9, the mutant-killers that pass on correct code by design, and tests of behaviour that was already right) | `raw/fix-r2-red-prefix.txt`, `raw/fix-r2-red-prefix-fails.txt` |
| GREEN, `-race -count=1 -v`, three independent runs through `ftps_fixture.sh` (unit tests of `pkg/ftp` + the integration tests against a REAL pure-ftpd) | run1: 145 PASS / 0 FAIL / 0 data races; run2: 145 PASS / 0 FAIL / 0 data races; run3: 145 PASS / 0 FAIL / 0 data races (top-level tests per run: 145), exit 0 each; sink-side digest of the served tree unchanged in all 3 | `raw/fix-r2-green-race-run{1,2,3}.txt` and `.stderr` |
| `pkg/factory` incl. the 6 new FTP wiring tests (a real socket peer), `-race` | all PASS | `raw/fix-r2-vet-cover-factory.txt` |
| Statement coverage, unit tests only (`go test -cover`) | `pkg/ftp` 90.8 percent, `pkg/factory` 100.0 percent (floor 85 in 11.4.224; LINE coverage of statements, branch coverage NOT measured, and coverage is necessary-not-sufficient: the mutants below are the real strength measure) | `raw/fix-r2-vet-cover-factory.txt` |
| `go vet` (default and tag `integration`), `go build ./...`, `gofmt -l pkg/ftp pkg/factory` | clean (no output) | `raw/fix-r2-vet-cover-factory.txt` |
| Mutants (harness `fix-r2-mutate.py`, container loop `fix-r2-mutate-run.sh`) | 69 of 69 mutants KILLED by a failing assertion (18 reviewer mutants MX01-MX28 re-expressed on the rewritten code, the reviewer's positive control PC16, 41 author mutants one per fix class, 9 factory-wiring mutants); 0 survived, 0 invalid; the harness ran WITHOUT `-race` (a kill is a failed assertion) | `raw/fix-r2-mutants.json`, `raw/fix-r2-mutants-report.txt`, `raw/fix-r2-mut-<id>.txt` |
| Negative controls | N00 (unmutated copy, same harness) PASSES (reported SURVIVED); N01 (comment-only edit) SURVIVES: the harness reports survival, it does not call everything killed | `raw/fix-r2-mut-N00.txt`, `raw/fix-r2-mut-N01.txt` |
| Real-server wire capture (control channel plaintext of a real pure-ftpd, TLS and clear text) | basis of two design corrections (see "Surprises") | `raw/fix-r2-realserver-wire.txt` |

Honest note on test-first: the findings, the reviewer's probes and the expected behaviours were known before any line was written, but the order of work was implementation first, tests second (as in round 1). The REDs above are real failures of the final tests against the real pre-fix code; they are not a test-first history (11.4.224 asks for it; this pass did not achieve it).
Honest note on the harness: the first complete batch of the mutant run showed 7 survivors (F1e control-write stall, F2d stray reply in the same segment, F2f stream failure, F4b seekable final reply, F6g MLST entry count, F8b degraded root, F11b re-dial leak), 5 invalid mutants (unused variables: F1a, F1c, F1f, W4, W5) and two mutants (MX09, MX19) that were "killed by timeout only" because the fake server's cleanup waited for leaked connections (it now closes them, so a leak fails its own test). Each survivor got a killing test and the invalid mutants were repaired; a second complete batch still showed F2f and W5 alive (the test did not fail on them), both got a sharper test; a third batch killed all 69; after it the test of F2f only gained a guard around `Close` (a hang is now a failure instead of a stall), so a fourth complete batch (the table above, with its per-mutant logs) was run against the final tests.

## Findings and what answers them

| Finding | Fix | Killing / proving tests |
|---|---|---|
| F1 deadlines (D1-D4 + static members; pool ConnectTimeout / HealthTimeout) | `proto.deadlineFrom` is the only source of deadlines: each control reply as a whole and each write get one `IOTimeout`, each data read/write one `IOTimeout`, the connect phase one `DialTimeout`, `ctx` always; `context.AfterFunc(ctx, interrupt)` breaks a blocked read or write | `TestDeadline_*` (no banner, TLS handshake, MLSD stall, RETR stall, no final reply, every control command and every login phase muted, control write stall), `TestCancel_*`, `TestSlowDrippingReply_*`, `TestDiscoverCert_StalledBanner_*` |
| F2 desync / 421 (D5, D6) | exact reply-code matching, unexpected positive reply and 421 and stray bytes make the connection unusable; an early close reads the server's one reply, then proves lock-step with `NOOP` | `TestDesync_*`, `TestUnexpectedPositiveReply_*`, `TestReply421_*`, `TestEarlyClose_PureFTPdStyleSingle150Reply_*` |
| F3 cancel / Disconnect between reads (D9, D9b) | `dconn` abort flag checked before AND after arming the deadline; `Disconnect` interrupts | `TestCancelBetweenReads_*`, `TestDisconnectBetweenReads_*`, `TestC9_*` |
| F4 truncation (D8) | EOF before `SIZE` bytes is `io.ErrUnexpectedEOF` or the server's own final-reply error | `TestSeekable_*`, `TestReadFile_FullReadWith451_*` |
| F5 login phases (D11, D12, D23) | classification by phase (table in `docs/FTP_CLIENT.md`); PASS is never resent | `TestLogin_*` |
| F6 MLSD parsing (D7) | own RFC 3659 parser (`mlsx.go`), unparseable line = `ErrListingIncomplete` | `TestParseMLEntry_Table_F6`, `TestList_*`, `TestMLST_*` |
| F7 550 (D18) | `classify550` + parent-listing settlement | `TestPermission550_*`, `TestAmbiguous550_*`, `TestClassify550` |
| F8 degraded stat (D10) | explicit `LIST <dir>` always, root checked with `CWD` | `TestDegradedStat_*` |
| F10 secret through fmt (D13) | value-receiver `Credential.String`/`GoString`, pointer-held config + redacting `String` in `scanFactory`, own copy in the pool | `TestSecret_NeverReachableThroughFmt_D13` |
| F11 timed-out Disconnect leak (D19) | close-when-idle flag executed by `release()`; a re-dial closes the connection it replaces | `TestDisconnectTimeout_*`, `TestReDial_*`, `TestConnectAfterTimedOutDisconnect_*` |
| F12 IAC (D15) | `escapeIAC` doubles 0xFF; POSIX-only confinement documented | `TestIACByteInAPath_IsDoubled_F12` |
| F13 size caps (D16, D17) | `Config.MaxReplyBytes`, `Config.MaxListEntries`, 16 KiB line bound | `TestReplyAndListingSizeAreBounded_*` |
| F14 Pin port 0 (D21) | `DiscoverCert`/`Pin` default the port | `TestPin_PortZero_*` |
| F15 server names | `Pin` refuses a second, different server name for a host (`ErrPinNameConflict`) | `TestPin_SecondServerName_*` |
| F16 not wired | `pkg/factory/ftp_factory.go`: `credential_ref`, `tls_mode`, `trusted_lan`, `allow_degraded_list`, `disable_epsv`, `ftp.DefaultPinStore`, `ftp.NewScanClient`; no inline secret | `pkg/factory/ftp_factory_test.go`, mutants W1-W9 |
| T1 17+1 surviving mutants | one killing test per reviewer mutant | table above |
| T2 fake cannot model failures | fake: per-session hooks (stall, cut transfer, extra reply, 421, permission 550, odd MLSD), working directory, leak-visible connection count | `fakeserver_test.go` |
| T3 unasserted hashes, ignored errors, "one attempt", control-connection count | all fixed; "one attempt" = a counting resolver on the real server; the per-worker control-connection count is NOT assertable on pure-ftpd (stated, not claimed) | `integration_test.go` |
| P1, P2 false documentation | `docs/FTP_CLIENT.md` rewritten to the measured behaviour; package doc; this README | - |

## Surprises (ground truth beat the model; found by the real-server leg)

* pure-ftpd advertises `UTF8` in `FEAT` and answers `OPTS UTF8 ON` with `504 Unknown command`. A first design that failed the connect on that refusal broke every real-server test. Now: connect, and refuse a LISTING only when it really contains a garbled name (invalid UTF-8 or `0x7f`, `ErrUTF8Refused`).
* pure-ftpd answers a download closed early with ONE reply: `150 <statistics>` over TLS (`226` in clear text), after a multi-line `150-Accepted data connection / 150 N kbytes to download`. Expecting 226/426 dropped the connection (30 s wait, then a re-dial per seek). Now the first reply may be anything below 400 and the `NOOP` lock-step check is the guard. This also answers the review's UNCONFIRMED question for pure-ftpd (one reply, connection kept); it stays UNCONFIRMED for the Synology FTP service. Capture: `raw/fix-r2-realserver-wire.txt`.
* The in-process fake listed `/` as a child of itself (a nameless entry the old library dropped silently); the new parser refused the listing and exposed it.

## Corrections to the round-1 statements in this README (they were measured wrong or understated)

* "a server that sends no reply could make `Close` wait. Not observed with pure-ftpd": it also hung after a FULL read (D4) and `Disconnect` could not break it. Now bounded.
* "wrong password classed as auth with no retry" and "a pool of 3 workers (one control connection each)" on the real server: neither was asserted. Now the first is (exactly one password fetch); the second still is not (stated above).
* "26 / 26 KILLED" described only the author's own mutant set; the reviewer's 18 independent mutants mostly survived. Round 2 runs both.
* "Control-channel commands cannot be interrupted by ctx", "`IOTimeout` bounds each data read", "`DialTimeout` bounds the TCP connect and the TLS handshake": false for the old code (D1-D3, D9); true for the new code, with tests.
* "`FileInfo.Mode` is 0 or `ModeDir`": symlinks have `ModeSymlink`; "`OPTS UTF8 ON` is sent right after login": it is sent after `FEAT` and `TYPE`.
* The launcher wrote its disk head-room records into the tracked `evidence/disk/`; it no longer does.

## UNCONFIRMED / not done

* **No NAS was contacted.** Unconfirmed against the Synology FTP service: its MLSD fact format (case of `Type`, `Size`, cdir naming, spaces in values), whether it sends `421` at `USER` for per-IP limits, whether it sends two replies on an early close, the leaf certificate SANs (the pin needs a matching or DNS name; Synology leaves are CA-issued with about one year validity, so each yearly renewal needs a re-pin and `ErrPinNameConflict` applies if the name changes), and the exact text of its permission `550`. A later read-only task owns that.
* A permission `550` and a per-worker control-connection count are not asserted against the real pure-ftpd (the launcher's tree digest needs every file readable; the server does not expose a count).
* Degraded-mode `LIST` limits (some servers glob `*`/`?`/`[` in the argument and hide dotfiles without `-a`): static, UNCONFIRMED per server.
* A data transfer as a whole is bounded only by `ctx` (each read by `IOTimeout`); a slow-loris data channel can keep a listing alive until `MaxListEntries` or `ctx`.
* TLS session resumption is proven against the in-process server only.
* `go.mod` still lists `github.com/jlaffaye/ftp` although no package imports it any more; `go mod tidy` removes it (left alone: other fixers change `go.mod`/`go.sum` in the same tree). `catalog-api/filesystem/ftp_client.go` (the duplicate) is another fixer's.
* Implicit FTPS (port 990), IPv6, FTP over a proxy: not implemented / not tested. The independent re-review of this round (11.4.142) is owed; this author does not declare GO.

## Files

Source (all uncommitted): `submodules/filesystem/pkg/ftp/{ftp.go,proto.go,mlsx.go,tls.go,cred.go,scan.go}`, tests `{ftp_test.go,ftps_test.go,fakeserver_test.go,fix_r2_net_test.go,fix_r2_list_test.go,integration_test.go}`;
`submodules/filesystem/pkg/factory/{factory.go,ftp_factory.go,factory_test.go,ftp_factory_test.go}`; `scripts/test-infra/ftps_fixture.sh`, `docker-compose.test-infra.ftps.yml` (unchanged); `docs/FTP_CLIENT.md`, `docs/scripts/ftps_fixture.md`;
harness `fix-r2-mutate.py`, `fix-r2-mutate-run.sh` here. Round-1 artefacts (`mutate_ftp.py`, `raw/M*.txt`, `raw/green-race-run*.txt`, `raw/red-*`, ...) describe the PRE-FIX tree and are kept as history only.
