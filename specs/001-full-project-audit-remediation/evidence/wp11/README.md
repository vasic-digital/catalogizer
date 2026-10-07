# wp11 evidence (T104, T105, T106, T148, T149)

Identity headers (`# identity`, `# head`, `# sha256 <file>`, `# run_at`) are on every captured run. HEAD moved while this work was in progress (7f5e9f4b, later b3728d76); all files under test are uncommitted.

| File | Task | Content |
|---|---|---|
| `t104-red.txt`, `t104-green-run1..3.txt`, `check-pins-mutation.txt` | T104, T105 | RED (script absent), GREEN x3 (89 checks) through `RUNP IMG-KCOV`, 23 mutants all CAUGHT |
| `pins-baseline.txt`, `pins-baseline-recursive.txt` | T105 | unpinned-reference rows: main repository (91), and every submodule checkout (3 more files: includes third-party, own-org filter not applied) |
| `t106-red.txt`, `t106-green-run1..3.txt`, `containerfiles-mutation.txt` | T106 | RED (tree incomplete), GREEN x3 (28 checks), 10 mutants all CAUGHT |
| `t106-build-*.txt`, `t106-smoke-*.txt` | T106 | local builds on anton (sigverify, gotools, infra-client, mut, docs) and one smoke each |
| `gotools.json`, `rust-coverage-tools.json`, `mc-client.json`, `nfs-client.json` | T106 | 11.4.270 existence verdicts |
| `goimports-baseline.txt`, `out-goimports/goimports-violations.txt` | T106 | goimports -l over 5243 .go files: 324 violations, control needle seen |
| `t148-red.txt`, `t148-green-run1..3.txt`, `t148-proposed.patch` | T148 | RED on the tracked files; patch applied to COPIES (`.audit/scratch/t148`), GREEN x3 |
| `t149-proposed.patch` | T149 | remaining six compose image lines by lock digest (applied to the t148 copy: 0 violations) |
| `../wp15/signatures.json` | T149 | IMG-SIGVERIFY digest first, then per-scanner cosign verdicts |
| `disk/` | all | disk_headroom records of every pull and build |

## Round 2 (T107-T114, T116, T148/T149 applied) - 2026-10-06

Builds ran rootless on this host (anton, owner decision) with `podman build` directly (disk gate `disk_headroom.sh` before each, `--pull=never`, images pulled by digest only); `scripts/build/dispatch.sh` runs argv inside an `IMG-*` image and cannot run `podman build`, so it was not used (not a RED/GREEN input). Root-context builds used a scratch ignorefile (the repository has no root `.dockerignore`, owed). Local-only aliases created: `docker.io/library/golang:1.25` (alias of `1.25-bookworm`), `docker.io/library/debian:trixie-slim` (pulled by digest a29215f6...).

| Files | Task | Content |
|---|---|---|
| `t107-red.txt`, `t107-v05-workaround-check.txt` | T107 | D-03 RED (COPY of `WebSocket-Client-TS/package*.json` fails), no out-of-tree workaround |
| `t108-d01-red.txt`, `t108-d04-red.txt` | T108 | D-01 RED (`COPY submodules/assets/`), D-04 RED (`COPY docker/go1.26.1...tar.gz`) |
| `t109-green-run1..3.txt`, `t109-smoke.txt` | T109 | D-04 fix (golang digest stage), 3 builds rc 0, identical image id (cache-warm) |
| `t110-red.txt`, `t110-green-check-run1..3.txt`, `t110-green-manual.txt`, `t110-green-build-run1..3.txt`, `t110-smoke.txt` | T110 | check_pins RED (3 violations) then 0; manual rows for the 3 items check_pins has no rule for; 3 builds rc 0 |
| `t111-iter1-run*-runtime-base-not-local.txt`, `t111-green-run1..3.txt` | T111 | D-01 fix; iteration 1 failed only because `debian:trixie-slim` was not in local storage (pulled by digest), 3 builds rc 0 after |
| `t112-iter1-fail-modules-built-before-source.txt`, `t112-green-run1..3.txt` | T112 | D-03 fix; iteration 1 passed all COPY lines and failed at `npm run build` (module builds ran before their sources were copied: second defect, fixed in the same file), 3 builds rc 0 |
| `t113-d02-red.txt`, `t113-green-run1..3.txt` | T113 | D-02 RED then fix, 3 builds rc 0 |
| `t114-red.txt`, `t114-green-run1..3.txt`, `t114-minio-blocked.txt` | T114 | 13 violations then 3 (MinIO, BLOCKED: image unavailable) |
| `t148-green`/`t149-green-run1..3.txt`, `t149-consistency.txt` | T148/T149 | patches applied to the tracked files; 0 violations in the two files; compose digests all map to lock entries |
| `mutations.txt`, `t116-mutate.sh` | T116 | D-01..D-04 revert mutants all fail at the COPY step; pins mutants caught by check_pins |

## Round 3 fixes (WF10 round 1 fix files indexed, WF13 round 3) - 2026-10-07

`wf10fix-p1-*` (WF10 p1 fix, 178ee136 and 96779242; these files carry no identity header, so a RED's target cannot be read from them: WF13 D2; the RED recorded there has 182 checks while the suite then had 205, so it predates 23 checks; `wf13fix-p1-check-pins-red.txt` re-records a RED for the full current suite): `check-pins-red/green-run1..3/container-run/mutation`, `containerfiles-red/green-run1..3/container-run/mutation`, `f5-golang-alias`, `f6-android-c7`, `pins-after.tsv`; their sums are `SHA256SUMS.wf10fix-p1`.

`wf13fix-p1-*` (round 3, constitution 11.4.276 structural round; every file carries `# identity`, `# head`, `# sha256` of the files under test and `# run_at`; sums in `SHA256SUMS.wf13fix-p1`):

| File | Content |
|---|---|
| `wf13fix-p1-check-pins-red.txt` | the round-3 `test_check_pins.sh` against the COMMITTED scanner (sha256 f94f45c2...): 281 passed, 71 failed |
| `wf13fix-p1-check-pins-green-run1..3.txt`, `-full-run.txt`, `-mutation.txt`, `-container-run.txt` | GREEN x3 (352 checks, host), the full run (352 fixtures + 113 mutants all CAUGHT + 1 = 466 passed, 0 failed), the mutation record, one run through `run_pinned IMG-KCOV` (351: no podman inside, the live-drift check is an honest SKIP) |
| `wf13fix-p1-containerfiles-red.txt`, `-green-run1..3.txt`, `-full-run.txt`, `-mutation.txt`, `-container-run.txt` | RED (the HEAD checker + the new fixture block: 62 passed, 15 failed), GREEN x3 (77), the full run (36 mutants all CAUGHT, 113 passed), the record, one container run |
| `wf13fix-p1-pins-after.tsv`, `wf13fix-p1-pins-diff.txt` | the live rows of the round-3 scanner (52) and their difference to the committed scanner (control needle: the committed scanner returns 51 rows outside the three edited files): one added row, `scripts/build_in_container.sh:25` |
| `wf13fix-p1-engine-options-regen.txt` | the committed option snapshot equals a regeneration from the live podman 5.7.0 help (243 rows) |

`pins-baseline.txt` (91 rows) and `pins-baseline-recursive.txt` are SUPERSEDED for the main repository by `wf13fix-p1-pins-after.tsv`: the first contained the false positives removed since, and its header cites `t105-notes.md`, a file that was never written (WF13 D3). They are left byte-identical because `SHA256SUMS` lists them.

## Round 4 fixes, pins / Containerfiles (WF16, constitution 11.4.276: the round after the structural round; review WF15 round 3) - 2026-10-07

`fix-r4-*` files named below belong to this change (other `fix-r4-*` files in this directory, `fix-r4-race-*`, `fix-r4-tic-*`, `fix-r4-red-tic.txt`, belong to the test-in-container fix); every one carries `# identity`, `# head`, `# sha256` of the files under test and `# run_at`; sums in `SHA256SUMS.fix-r4-pins`.

| File | Content |
|---|---|
| `fix-r4-convergence-assessment.txt` | the 11.4.276(E) convergence assessment written FIRST: root causes, the classes S, A, B, F, G, H, C4, C7 with their enumerated members, the decided residuals |
| `fix-r4-check-pins-red.txt` | the round-4 `test_check_pins.sh` against the COMMITTED round-3 scanner (sha256 100f3409...): 395 passed, 43 failed |
| `fix-r4-check-pins-green-run1..3.txt`, `-full-run.txt`, `-mutation.txt`, `-container-run.txt` | GREEN x3 (438 checks, host), the full run (438 fixtures + 172 mutants all CAUGHT + 1 = 611 passed, 0 failed), the mutation record (172 rows), one run through `run_pinned IMG-KCOV` (437: no podman inside, the live-drift check is an honest SKIP) |
| `fix-r4-containerfiles-red.txt` | the round-3 Containerfile checker (HEAD heredoc, its own wrapper list and count pairing) with the round-4 fixtures: 86 passed, 17 failed |
| `fix-r4-containerfiles-green-run1..3.txt`, `-full-run.txt`, `-mutation.txt`, `-container-run.txt` | GREEN x3 (103 checks), the full run (62 mutants all CAUGHT, 165 passed, 0 failed), the record, one container run (103) |
| `fix-r4-shell-ground-truth.sh` / `.txt` | real shells and tools: every wrapper form runs its command, a compound command is ONE pipeline stage, which shells/interpreters read their PROGRAM from stdin; control needle per block; result ALL-AS-MODELLED |
| `fix-r4-podman-ground-truth.sh` / `.txt` | the live podman 5.7.0 names the image for every boolean/value/cluster/`--`/array form (`--pull=never`, an image that does not exist: nothing pulled, nothing created); `podman create` rejects `-d` (the option tables merge run and create) |
| `fix-r4-mutation-negative-control.txt` | the mutation harness tells survival from capture: a comment-only edit SURVIVES, a no-op and a missing-anchor mutant are NOT-APPLIED, two real mutants are CAUGHT |
| `fix-r4-pins-after.tsv`, `fix-r4-pins-diff.txt` | the live rows of the round-4 scanner (52) and the difference to the committed scanner (control needle: it returns the same 52 rows through the same instrument): 0 added, 0 removed |
| `fix-r4-engine-options-regen.txt` | the committed option snapshot (unchanged) equals a regeneration from the live podman 5.7.0 help (243 rows) |

Host measurements are the HEAD 350372a8 work tree with this change uncommitted; the full suites took about 105 minutes (check_pins) and 40 minutes (Containerfiles) under a loaded host.
