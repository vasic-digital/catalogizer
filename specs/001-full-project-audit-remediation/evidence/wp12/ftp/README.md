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
