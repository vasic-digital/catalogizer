# identity: WP-12 PA-02 (ReadOnly decorator) and PA-07 (fabric: Pool, HostBudget, Limited/Retrying/Confined/Metered)
# head: submodules/filesystem 83c0ac1f407f9e11044e97f4aa385ecfaf1cde21 (main repo e9d6883d) COMMITS the code of review rounds 1 and 2; the round-3 fix described in the last section is UNCOMMITTED in the working tree at the time of writing (the conductor commits it)
# run_at: round 1 2026-10-07T17:55:11Z; round 2 2026-10-07; round 3 2026-10-08 (files fix-r3-*)
# author: Sonnet worker; independent review round 1 (WF19, Opus xhigh, 2026-10-07): NO-GO, wf19-review-round1.md; round 2 fix (files fix-r2-*); independent re-review round 2 (WF24, Opus xhigh, 2026-10-08): NO-GO, wf24-review-round2.md; round 3 fix (files fix-r3-*): applied, independent RE-REVIEW OWED, not performed

| File | Content |
|---|---|
| `pa02-red.txt` | RED: decorators tests against a pass-through stub (every mutator reaches the inner client): many FAILs |
| `pa02-green-run1..3.txt` | decorators alone, GREEN x3 (-race, 100.0% statements), taken before the fabric package existed |
| `pa07-red.txt` | RED: fabric tests with the implementation files absent (build failed). DISCLOSURE: the implementation was drafted before the tests; this RED is tests-against-absent-code, not a failing-first record of a prior prototype (same disclosure as wp12/README.md) |
| `green-run1..3.txt` | go vet + go test -race -count=1 -cover -v of BOTH packages, x3: decorators 100.0%, fabric 97.1% of statements, all PASS (57+ tests in fabric) |
| `fuzz-run.txt` | coverage-guided fuzz 20 s each: FuzzReadOnly_Mutators (45318 execs) and FuzzConfined (22204 execs), no failure |
| `mutants.json`, `mutate.py` | 38 mutants (10 decorators D1-D10, 28 fabric F1-F28) + 1 negative control NC1 (no-op). The harness copies the tree inside the container, applies ONE exact-string edit (must match exactly once), runs both packages with -race; a mutant that does not compile is INVALID (not "caught") |
| `mutations-run1.txt` | FIRST mutation run, kept as history: it found 1 SURVIVOR (F13: the FTP 530 test message also matched a text marker, so the code path was untested) and 2 INVALID mutants (F4, F17 did not compile). Fixed afterwards (test messages without markers, mutants repaired, an extra harness rule) |
| `mutations-final.txt` | FINAL run: BASELINE PASS, 38 of 38 CAUGHT, negative control SURVIVED as required, bad=0 |
| `SHA256SUMS` | sha256 of the evidence files above (verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/fabric && sha256sum -c SHA256SUMS`) |
| `SOURCES.sha256` | sha256 of the source files under test, repo-root relative (verify from the repo root: `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/fabric/SOURCES.sha256`); the files are uncommitted, so this proves what was tested, not a commit |

Run method: `scripts/containers/run_pinned.sh --out .audit/out/fabric-go IMG-GO -- sh -c 'cd /src/submodules/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=3 CGO_ENABLED=1 go test -race -count=1 ./pkg/decorators/ ./pkg/fabric/'`
(the runner wrappers scripts/containers/run_go.sh refused every start with anti_mess_drift caused by another worker's test-infra container, so run_pinned.sh was called directly, as the task permits; run_pinned.sh is not preceded by the anti-mess sweep).
Tests ran one suite at a time. Host load during the runs was about 20 (other workers), which only affects wall time.


---

# Round 2: fix of the WF19 independent review (files `fix-r2-*`, `wf19-review-round1.md`)

IMPORTANT for readers of the table above: the round-1 files (`green-run*.txt`, `mutations-final.txt`, `mutants.json`, `mutate.py`, `pa0*-*.txt`, `fuzz-run.txt`) describe the code BEFORE the round-2 fix and are kept as history. **"38 of 38 CAUGHT" in `mutations-final.txt` measured an author-selected mutant set**; the independent reviewer's own set of 20 mutants (R01..R20) ALL SURVIVED that suite (review T1), so that number is not evidence of suite strength. Current numbers are the `fix-r2-*` files below. The README line "independent review OWED" of round 1 is superseded: the review happened (`wf19-review-round1.md`, verdict NO-GO, 15 source-defects 1 HIGH + 6 MEDIUM + 8 LOW, 2 test-instrumentation, 7 process-doc); round 2 fixed all of them; the independent RE-REVIEW of round 2 also happened (`wf24-review-round2.md`, NO-GO, see the Round 3 section at the end).

| File | Content |
|---|---|
| `wf19-review-round1.md` | the independent review being fixed (verbatim copy of the reviewer's report) |
| `fix-r2-convergence.md` | convergence assessment written BEFORE the fix (11.4.276(E)): ground truth, defect classes with all enumerated members, what is adopted verbatim |
| `fix-r2-red-A.txt` | RED on the PRE-fix source (the five main files' sha256 equal `SOURCES.sha256` of the review): the reviewer's probes RV01..RV22 verbatim: 18 FAIL, 3 PASS. The 3 PASS (RV10, RV12, RV14) are the reviewer's measurement/observation probes that cannot fail in their original form; they were made strict assertions when adopted (marked `ADOPTED AS ASSERTION` in `review_probes_test.go`) and their strict forms are proven load-bearing by mutants N36/N36b (HostKey), N43 (retry policy), N06/N07 (held slots and lease) instead of by a RED on the old code |
| `fix-r2-red-B.txt` | pre-fix source + the round-2 test files: a BUILD failure (the tests need the new package `pkg/decorators/guard` and new fabric API), disclosed as such; it is not a behavioural RED |
| `fix-r2-green-run1..3.txt` | go vet + `go test -race -count=1 -cover -v` of `pkg/decorators/...` and `pkg/fabric`, x3: all PASS, 0 FAIL, 0 SKIP, 146 top-level tests per run; coverage decorators 100.0%, decorators/guard 91.6%, fabric 97.1% (a proxy: necessary, never sufficient, see the mutation run) |
| `fix-r2-fuzz.txt` | coverage-guided fuzz 20 s each under -race: `FuzzConfined` (now also asserts that no control character reaches the inner client) and `FuzzReadOnly_Mutators`, no failure |
| `fix-r2-mutants.json`, `fix-r2-mut.py` | 71 entries: the reviewer's controls PC1 (must be CAUGHT) and NC1 (must SURVIVE), ALL 20 reviewer mutants R01..R20 (verbatim; five - R05, R07, R13, R14, R15 - and R16 re-expressed because the fix moved the mutated text, each marked `ADAPTED` with the reason) and 49 own mutants N01..N48 + N36b, one or more per member of every defect class. The harness is the reviewer's, extended: it runs the WHOLE suite (including the adopted reviewer probes) as the judge |
| `fix-r2-mut-run1.txt` | FIRST run, kept as history: 60 CAUGHT, 7 SURVIVED, 3 INVALID. It found: R16 (equivalent: the DNS-timeout branch was redundant with the `net.Error` branch; the code was simplified and the mutant moved to the remaining branch), N04 (first version was an equivalent mutant because the cascade loop undid it; a real test, `TestBudget_CancelledTailThenIdleTimeIsNotLost`, and a real mutant were added), N11/N18/N19 (three missing tests, added), N17 (a race-only mutant: needs `-race`), and 3 mutants that did not compile (R07, N36, N45; repaired) |
| `fix-r2-mut-final.txt` | FINAL run on the final tree: BASELINE PASS; PC1 CAUGHT; NC1 SURVIVED as required; 69 CAUGHT; the only other survivor is N17 (see next); 0 INVALID, 0 NOT_APPLIED; `HARNESS_CONTROLS_OK=True` |
| `fix-r2-mut-race.txt` | N17 (the unborrow lock removed) is a data race, invisible without `-race`; with `RACE=1`: N17 CAUGHT by `TestPool_CloseAll`, `TestPool_RaceManyBorrowers`; NC1 SURVIVED; controls OK |
| `fix-r2-compat.txt` | `go build ./...` and `go vet` of ftp/factory/sftp/nfs3 clean; `pkg/ftp` tests (128 top-level, the FTP worker's tree, one snapshot) pass with the round-2 fabric exactly as with the pre-fix fabric. NOTHING outside the two packages was edited |
| `SHA256SUMS`, `SOURCES.sha256` | refreshed for this directory and for the 30 source/test/doc files of round 2 (see Verify) |

Run method: `scripts/containers/run_pinned.sh --out <dir> IMG-GO -- sh -c 'cd /src/submodules/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 CGO_ENABLED=1 go test -race -count=1 ./pkg/decorators/... ./pkg/fabric/'`, one container at a time; the mutation harness ran inside the same container (`python3 -I /out/fix-r2-mut.py /out/fix-r2-mutants.json /src/submodules/filesystem`) without `-race` for speed (N17 separately with `RACE=1`). Host load during the runs was 25 to 30 (other workers).

Not claimed / UNCONFIRMED: an independent re-review of round 2; behaviour against a live SMB/SFTP/FTP/WebDAV server (none was contacted); the legacy `pkg/nfs` read-write mount (S7 SETATTR path, needs root); S6 for the raw `pkg/smb`, `pkg/webdav`, `pkg/local` clients - their own `GetConfig` still returns the live config (outside the two packages; the decorators now hand out only redacted copies, which is the exposure the review measured); an optional time-based circuit breaker for transient connect failures (review S3 "optionally"); the reviewer's `Synology-class auto-block` premise.

Verify (from the repository root): `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/fabric/SOURCES.sha256` and `cd specs/001-full-project-audit-remediation/evidence/wp12/fabric && sha256sum -c SHA256SUMS`.


---

# Round 3: fix of the WF24 independent re-review (files `fix-r3-*`, `wf24-*`)

State: the code of rounds 1 and 2 is committed in `submodules/filesystem` at `83c0ac1` (main repo `e9d6883d`). Round 3 is the fix of the re-review `wf24-review-round2.md` (Opus xhigh, 2026-10-08, NO-GO: 11 source-defects N1..N11 = 2 MEDIUM + 9 LOW, 2 test-instrumentation T3/T4, 4 process-doc D1..D4). **The round-3 changes are UNCOMMITTED at the time of writing; the conductor commits them. An independent re-review of round 3 is OWED and was not performed.** Scope of the edits: `submodules/filesystem/pkg/decorators/**`, `pkg/fabric/**`, `docs/filesystem/pkg_fabric.md`, `pkg_decorators.md`, this directory.

| File | Content |
|---|---|
| `wf24-review-round2.md` | the independent re-review being fixed (verbatim copy of the reviewer's report) |
| `wf24-reviewer-*` | the reviewer's probes (`zz_wf24*_test.go`, stored as `.go.txt` so that no Go tool picks them up), mutant sets (`wf24_mutants.json`, `killer_mutants.json`, `w10ctl_mutants.json`, `r_plus_mutants.json`) and harness (`wf24_mut.py`), verbatim |
| `fix-r3-convergence.md` | convergence assessment written BEFORE the fix (11.4.276(E)): class-trigger analysis (the "fix verified on a simplified model" class has 2 MEDIUM members, so a third member forces a structural round), 13 defect classes with all enumerated members, what is adopted verbatim; plus an addendum with the three corrections that real-type ground truth forced during the pass |
| `fix-r3-red-A.txt` | RED on the COMMITTED source (`SOURCES.sha256` of the review: the 14 non-test source files OK): the reviewer's probes W01..W18 and K11..K69 adopted verbatim (header comment added and gofmt applied, nothing else at this point; W17 and W18 were strengthened afterwards) as `wf24_review_probes_test.go`, `wf24b_...`, `wf24c_...`: **14 FAIL** (W01 to W09, W11 to W14, W17 = the defects N1, N3, N4, N5, N6, N7, N8, N2/D4, N9, N10, N11, D1) and the 11 that PASS by design (W10 property test, W15, W16, W18 and the seven killers K11/K13/K38/K40/K51/K66/K69, which pass on the committed code and fail on the surviving mutants) |
| `fix-r3-red-B.txt` | committed source + the round-3 test files: a BUILD failure (`undefined: credentialMarkers`, `smbCredentialStatuses`, ...: the new tests need the new API), disclosed as such; it is not a behavioural RED. The behavioural evidence for the own tests is the mutation campaign below: every one of them is the killer of at least one mutant |
| `fix-r3-green-run1..3.txt` | go vet + `go test -race -count=1 -cover -v` of `pkg/decorators/...` and `pkg/fabric`, x3 on the final tree: all PASS, **0 FAIL, 0 SKIP, 226 top-level tests per run** (round 2: 146); coverage decorators 100.0%, decorators/guard 96.0%, fabric 97.8% to 97.9% (a proxy: necessary, never sufficient, see the mutation run) |
| `fix-r3-fuzz.txt`, `fix-r3-fuzz-oom.txt` | `FuzzConfined` (seeds now include C1 controls, raw 0x85/0x9B, invalid UTF-8, a valid multi-byte path; it asserts valid UTF-8 and no `unicode.IsControl` rune reach the inner client) and `FuzzReadOnly_Mutators`, 20 s each, no failure. The first attempt with `-race` was OOM-killed by the container memory ceiling (`fix-r3-fuzz-oom.txt`, kept as history); the repeat is without `-race` |
| `fix-r3-mutants.json`, `fix-r3-mut.py` | 115 entries: controls PC1 (must be CAUGHT) and NC1 (must SURVIVE), the reviewer's mutants (M01..M73 of `wf24_mutants.json` incl. M07/M08/M13/M37/M44/M45/M46/M51/M70 re-expressed because the fix moved the mutated text, each marked `ADAPTED`; R01..R20 of the round-2 set, R13/R15 re-expressed), the reviewer's W10 instrument control V02, and 67 own mutants X01..X70 (X50 unused) (one or more per member of every defect class). The harness is the round-2 harness (itself derived from the reviewer's, not the author's); it runs the WHOLE suite including the adopted reviewer probes as the judge, with `RACE=1` for every mutant |
| `fix-r3-mut-run1.txt`, `-run2.txt`, `-run3.txt`, `fix-r3-mut-final.txt` | run1 = all 115 on the tree of the first full runs: 96 CAUGHT, 2 SURVIVED (NC1 as required, X55), 17 INVALID with an empty compile diagnostic (contiguous block, probable host memory pressure, UNCONFIRMED) and X22/X57 whose own mutants did not compile (repaired); run2 = the 19 mutants that were not CAUGHT: 16 CAUGHT, X56 and X62 SURVIVED; run3 = X55, X56, X62 after the new tests `TestR3_TransientFailuresDoNotSerialiseAnUnprovenAccount`, `TestR3_ProvenAccountLoginsRunConcurrently` (strengthened) and `TestR3_EvictLiftsAPerRootDenial`: all CAUGHT. `fix-r3-mut-final.txt` is the per-mutant verdict derived by script from the three runs: **114 CAUGHT, 1 SURVIVED (the NC1 negative control, as required), 0 INVALID, 0 NOT_APPLIED**; the 10 reviewer survivors M01, M07(f), M08, M11, M13, M38, M40, M51, M66, M69 are all CAUGHT. No source file changed between run1 and run3, only tests |
| `fix-r3-mut-aborted-partial.txt` | the first campaign, stopped after 25 mutants and restarted from scratch when a source change (account key fallback) was found necessary; history only, not counted |
| `fix-r3-compat.txt` | `go build ./pkg/...`, `go vet` of the consumers and `go test -race` of `pkg/ftp`, `pkg/smb`, `pkg/webdav`, `pkg/local` against the round-3 fabric: all `ok` (one snapshot of the shared tree; other workers edit `pkg/ftp`, `pkg/sftp`, `pkg/nfs3`, `pkg/factory` concurrently; nothing outside `pkg/fabric` and `pkg/decorators` was edited by this fixer) |
| `SHA256SUMS`, `SOURCES.sha256` | refreshed for this directory and for ALL `.go` files of `pkg/decorators/**` and `pkg/fabric/**` plus the two docs (see Verify) |

What changed, by finding (details in `docs/filesystem/pkg_fabric.md` and `pkg_decorators.md`):

| Finding | Fix |
|---|---|
| N1 login failures of SMB / WebDAV not auth | `errors_smb.go`: a go-smb2 `*ResponseError` with one of 12 logon/account NTSTATUS values is auth (typed, no English text); WebDAV 401 / 407 text markers, 403 at login via `ClassifyLogin`; structural `AuthFailure() bool` seam; per-protocol table test over the real types and the real `pkg/webdav` client against `httptest` |
| N2 / D4 probe bypasses the budget | `HostBudget.TryAcquire`; `Limited.TestConnection` is budgeted without waiting and returns `ErrProbeSkipped` when busy; `Pool` treats a skipped probe as no fault; real `pkg/webdav` through Pool + Limited: server-side peak within `MaxConcurrent`; `Disconnect` is the one documented exemption |
| N3 single-flight / per account | pilot login per account, rejected-login memory per account (strict credential evidence) or per root (generic refusals), `Evict` lifts it, settings with no account identity are their own account |
| N4 / T4 nil streams | `ErrNilStream` in every layer (slot released), `guard.Wrap(nil)` is nil, `ReadOnly` keeps nil nil; `OldestHeld <= 0` assertion |
| N5 retry floor | `MinRetryDelay` (10 ms) applied to the effective delay after cap and jitter |
| N6 path characters | `unicode.IsControl` (C0, DEL, C1) and `utf8.ValidString` in `Confined`, whole 0x00..0xFF grid test, fuzz seeds |
| N7 4yz path echo | the 4yz text rule uses credential-specific markers only and only when no path precedes them; enumerated over the real marker lists in `markers_internal_test.go` |
| N8 reused tail | `giveBack` winds back whenever the slot is the tail |
| N9 RedactConfig | deep copy at any depth, word-based names, live handles blanked, depth bound |
| N10 duplicate client | `ErrDuplicateClient`, refused before Connect and never disconnected |
| N11 HostKey | strict `host`, `host:port`, `[ipv6]:port` grammar |
| T3 | W15, W16, K11, K13, K38, K40, K51, K66, K69 adopted verbatim; all 10 survivor mutants CAUGHT |
| D1, D2, D3 | `MarkTransient` precedence documented and tested; the lease is idle-based (W18 strengthened to an assertion, constants adapted: lease 250 ms, 800 ms of reading); status headers state the commit |

Existing tests changed because they encoded a premise this round replaces (all listed, none weakened): `TestLimited_EveryOpTakesOneSlotExceptDisconnectAndTestConnection` (renamed, the probe now takes a slot), `TestPool_RejectedLoginIsNotRepeatedAcrossBorrows` (account b uses a different user), `naiveDelay` of `TestRetryPolicy_DelayClosedFormMatchesDefinition` (the specification gained the floor), `FuzzConfined` (C1 and UTF-8), the `OldestHeld` assertion of RV14, and the adopted W17 / W18 (marked `ADOPTED AS ASSERTION`).

Not fixed / not done in round 3, with the reason:
- `pkg/smb` and `pkg/webdav` themselves (return `fabric.MarkAuth` from `Connect`; `pkg/smb` `TestConnection` as `Stat(".")`): outside the allowed scope; the classifier now recognises their errors from the outside. Needed change, owed to the owners of those packages.
- `go.mod` / `go.sum`: no change needed (go-smb2 v1.1.0 is already required); `pkg/fabric` now imports it.
- The live behaviour against Samba, a Synology SMB/WebDAV/FTP server and the SMB share-root listing cost: UNCONFIRMED (no server was started or contacted, no NAS contacted).
- The lease timer uses the real clock (observation in the review): documented, not changed.

Run method: `bash scripts/containers/run_pinned.sh --out /dev/shm/fabric-r3 IMG-GO -- sh -c 'cd /src/submodules/filesystem && env GOTOOLCHAIN=local GOFLAGS=-mod=mod HOME=/out GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 CGO_ENABLED=1 go test -race -count=1 ...'` (one container at a time; the launcher refused repeatedly for host memory while other workers ran and was retried). The mutation harness ran inside the same container (`RACE=1 python3 -I /out/fix-r3-mut.py /out/fix-r3-mutants.json /src/submodules/filesystem [ids]`).

Not claimed / UNCONFIRMED: the independent re-review of round 3; behaviour against any live server; the SMB probe cost on a real share root; the cause of the 17 empty-diagnostic INVALID results of run1 (they compiled in run2).

Verify (from the repository root): `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/fabric/SOURCES.sha256` and `cd specs/001-full-project-audit-remediation/evidence/wp12/fabric && sha256sum -c SHA256SUMS`.
