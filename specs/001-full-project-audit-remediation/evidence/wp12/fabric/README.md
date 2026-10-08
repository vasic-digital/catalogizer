# identity: WP-12 PA-02 (ReadOnly decorator) and PA-07 (fabric: Pool, HostBudget, Limited/Retrying/Confined/Metered)
# head: 69ca319d (main), working tree dirty; all code is UNCOMMITTED in the own-org submodule submodules/filesystem (pkg/decorators, pkg/fabric)
# run_at: 2026-10-07T17:55:11Z
# author: Sonnet worker; independent review round 1 (WF19, Opus xhigh, 2026-10-07): NO-GO, see wf19-review-round1.md; round 2 fix (this directory, files fix-r2-*): applied, independent RE-REVIEW OWED, not performed

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

IMPORTANT for readers of the table above: the round-1 files (`green-run*.txt`, `mutations-final.txt`, `mutants.json`, `mutate.py`, `pa0*-*.txt`, `fuzz-run.txt`) describe the code BEFORE the round-2 fix and are kept as history. **"38 of 38 CAUGHT" in `mutations-final.txt` measured an author-selected mutant set**; the independent reviewer's own set of 20 mutants (R01..R20) ALL SURVIVED that suite (review T1), so that number is not evidence of suite strength. Current numbers are the `fix-r2-*` files below. The README line "independent review OWED" of round 1 is superseded: the review happened (`wf19-review-round1.md`, verdict NO-GO, 15 source-defects 1 HIGH + 6 MEDIUM + 8 LOW, 2 test-instrumentation, 7 process-doc); round 2 fixes all of them; an independent RE-REVIEW of round 2 is OWED.

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
