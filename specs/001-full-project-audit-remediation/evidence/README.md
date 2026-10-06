# Evidence root (`$EV`)

`$EV` is `specs/001-full-project-audit-remediation/evidence` (tasks.md line 76). Paths below are `$EV`-relative.

## Canonical paths (decision, W8 of the 2026-10-05 review of T001/T004)

tasks.md T001 and T100 name `$EV/host-probe.json` and `$EV/disk/<op_id>.json`; the scripts default to exactly these paths
(`scripts/containers/probe_host.sh`: `$EV/host-probe.json`; `scripts/containers/disk_headroom.sh`: `$EV/disk/<op_id>.json`).
The earlier decision that `evidence/wp09/host-probe.json` and `evidence/wp09/disk/` were canonical is REVERSED. Mapping:

| Old location (round r2) | Canonical (`$EV`-relative) |
|---|---|
| `wp09/host-probe.json` | `host-probe.json` |
| `wp09/disk/<op_id>.json` | `disk/<op_id>.json` |

Everything else T001 places under `$EV/wp09/` (RED captures, mutation records, GREEN x3) stays in `wp09/`.
`wp03/constitution-repo-verify.json` (another task's record) still cites the old `wp09/host-probe.json` path; it was not edited here (UNCONFIRMED: owner of that record decides whether to re-point it).

## wp09 identity headers

Each r3 file under `wp09/` starts with `#` lines carrying the generation time, the git HEAD and the sha256 of: both T001 scripts,
`disk_headroom.conf`, both T001 tests, the T004 test, the working-tree `.gitignore` and `wp09/.gitignore.before`.
The tree under test is uncommitted, so HEAD alone does not identify it; the sha256 lines do. Files without such a header
(`wp09/superseded-r2/*`) are the earlier round's records, kept for provenance and NOT evidence for the current scripts.

## No repo-root `evidence/` (rule and guard)

All evidence of this feature lives under `$EV` (the folder above). A directory, file or symlink named `evidence` at the repository root is forbidden, and no script or tool may name or default to a repo-root `evidence/...` path (write `$EV/wp01/...` in prose, never a bare `evidence/wp01/...`). The guard is `scripts/governance/tests/test_evidence_root.sh` (revision 2: it scans every text file under `scripts/` and `tools/`, tests `-e`, proves itself with control needles and mutation scenarios, and carries a shrink-only residue list for files of other areas; see its header). Transcripts of that guard and of the other WP-01 tests are indexed in `wp01/README.md`.

## Round r3 of containers/.gitignore (2026-10-05, fix of WF2-REVIEW-containers-gitignore I1-I5, M1-M7)

Current T001/T004 evidence (all with `#` identity headers: generation time, HEAD, sha256 of both T001 scripts, `disk_headroom.conf`, both T001 tests, the T004 test, `.gitignore`, `wp09/.gitignore.before`; the RED files carry the OLD code under test marked `(OLD)` and the NEW tests marked `(NEW test)`):

| Area | RED | GREEN x3 | Mutation record |
|---|---|---|---|
| T001 disk gate | `wp09/disk-headroom-red.txt` | `wp09/disk-headroom-green-{1,2,3}.txt` | `wp09/disk-headroom-mutation.txt` (= run 3; runs 1-3 in `-mutation-run{1,2,3}.txt`) |
| T001 probe | `wp09/probe-red.txt` | `wp09/probe-green-{1,2,3}.txt` | `wp09/probe-mutation.txt` (= run 3; runs 1-3 in `-mutation-run{1,2,3}.txt`) |
| T004 `.gitignore` | `wp09/gitignore-red.txt` | `wp09/gitignore-green-{1,2,3}.txt` | rule-line removal sweep and anchor sweep are inside the GREEN transcripts; `wp09/gitignore-block-mutation.txt`, `wp09/gitignore-w7-bound-mutation.txt`, `wp09/gitignore-class-diff.txt` |

Also: `wp09/real-host-run.txt` (the current scripts against the real host, which wrote `host-probe.json` and `disk/t001-real-host-need-largest-image.json`, both sha256-sealed in that file), `wp09/r3-reviewer-mutants.txt` (each reviewer survivor RM1, RM2, RM10, RM11, Pc, Pf, Gc and the paired mutation that now kills it), and `wp09/SHA256SUMS` (scope: the T001/T004 files listed above, `wp09/.gitignore.before`, `wp09/superseded-r3/*`, `host-probe.json` and `disk/t001-real-host-need-largest-image.json`; other work packages' files in `wp09/` are not covered and may be appended by their owners; check with `cd wp09 && sha256sum -c SHA256SUMS`).

Superseded (moved to `wp09/superseded-r3/`, see its README.txt): every earlier `disk-headroom-*`, `gitignore-*` and `probe-*` file. They carry older hashes or none (the bounded-signal and signal disk records never had a header, review I3); they are NOT evidence for the current files. `disk/` holds records of other tasks too (`smoke-*.json`, `probe-try.json`); only `disk/t001-real-host-need-largest-image.json` belongs to T001.

What the round changed (finding, then where it is pinned):
- I1 fork-to-exec window: `own_child` accepts a child that is still wearing the gate's own command line (parent link proven, non-empty own image), and `stop_chain` KILLs such a pre-exec child at once (it has no descendants yet). Pinned by `preexec_suite` (strace holds only the `timeout` execve for 10 s; podman probe and reaper cases; gate must exit within 2500 ms, the child must be gone and the hang shim must never start; SKIP with reason when strace is absent) and by `own_suite` units. RED on the old script 4/4 assertions, GREEN x3. Residual stated in round r3 (corrected in round r4, WF3-REVIEW R7): a signal landing between the `own_child` check and the KILL, with the exec completing in that window, is not covered; if a KILL ever lands after `timeout` has forked its child, that child is NOT bounded by anything but its own `timeout` (the bound only covers the child while `timeout` lives). That residual is NOT the window the reviewer found in round 3: a child INSIDE `execve` reads an EMPTY cmdline, which round r3 treated as "not ours", so NOTHING was signalled and the gate printed a false "stopped" (R1, fixed in round r4, see the r4 section).
- I2: an empty, null, numeric, boolean or missing grant `run_id` is refused `commit_turn_held` (fixtures for unset, empty and equal-value `EVREC_TURN_RUN_ID`); `--op-id` must match `[A-Za-z0-9_-][A-Za-z0-9._-]*` (no `/`, no leading `.`, an explicit empty value is refused, not replaced by a generated name); fixtures for `a/b`, `/../../../x`, `../x`, `..`, `.`, `.hidden` prove nothing is written anywhere under the test tree.
- I3: the evidence above.
- I5: repo-wide `__pycache__/`, `*.py[cod]`, `.venv/`, `.pytest_cache/`, `.mypy_cache/`, `.ruff_cache/` replace the 21 per-tree cache lines; controls at the root, nested, outside and inside the negated trees; `wp09/gitignore-class-diff.txt` shows `scripts/audit/__pycache__/org_of.cpython-314.pyc` now ignored and no untracked bytecode or cache path left unignored. The tracked set matched by any rule is unchanged (1837).
- M1: a TERM/INT/HUP during the record write removes the temp file and exits 128+n (disk gate and probe). M2: `bounded_out` fixtures (valid number then failure, then hang, then a TERM-ignoring probe that only `timeout -k 2` ends). M3: the probe's cgroup root is overridable (`PROBE_CGROUP_ROOT`), 16-digit and 15-digit meminfo fixtures. M4: `host-probe.json` gains `control_positive_dev` (`/dev/null`) and `control_positive_sys` (`/sys/kernel`), same-class needles for the devtmpfs and sysfs absences. M5: tools are preflighted (`reason=dependency_missing <tool>`, exit 2). M6: every example negation is probed at the root and nested, and an anchor sweep rewrites each unanchored rule line with a leading slash (31 lines, 31 caught). 
- M7 (OPEN, not done): no companion document for `probe_host.sh` or for the T004 test is planned in tasks.md (`docs/scripts/disk_headroom.md` is T009's); the owner of the docs task decides.

### Owed text amendments and an owner decision (not done here: tasks.md and decisions/ are not edited by this round)

- RESOLVED, T004 (review I4; corrected in round r4, WF3-REVIEW R7): `tasks.md` T004 was amended at 22:00 local on 2026-10-05 (before the round-r3 text above was written) and now names the `# BEGIN helix-t004-repo-wide-secrets` / `# END helix-t004-repo-wide-secrets` marker pair, the repo-wide secrets block and the cache/hosts block. The earlier "OPEN" entry here was stale; nothing is owed on T004's text under review I4 (the six repo-wide cache lines still await the owner confirmation in the OWNER DECISION entry below).
- OPEN, T001: the text names the probe record fields and the gate's refusal reasons; it should add `control_positive_dev` and `control_positive_sys`, the reason `dependency_missing` and the `--op-id` charset, and the evidence names above.
- OPEN OWNER DECISION: `decisions/owner-decisions.yaml` now carries `OAU-2026-10-05-5` ("keep the repo-wide secrets block and the cache/hosts block and amend T004 text; owner-approved", relayed by the conductor, `verbatim: false`). That record approves the two blocks the review found unauthorised, but it is a relayed paraphrase, not an owner quotation, and it predates the six repo-wide cache lines added in this round (they extend the cache group it names). UNCONFIRMED: that the owner approves the repo-wide cache lines as such; the owner-request list (`decisions/owner-request-list.md`) should carry one line asking for that confirmation. The tasks.md amendment above is still owed under that decision.

## Round r4 of containers/.gitignore (2026-10-06, fix of WF3-REVIEW-containers-gitignore R1-R9; owner scope: every blocking and every real-defect finding)

Current T001 evidence (all with `#` identity headers; the RED files carry the OLD round-3 scripts marked `(OLD)` and the NEW tests marked `(NEW test)`):

| Area | RED | GREEN x3 | Mutation record |
|---|---|---|---|
| T001 disk gate | `wp09/r4-disk-headroom-red.txt` (fast mode `DH_ONLY_R4=1`: 95 passed, 11 failed) | `wp09/r4-disk-headroom-green-{1,2,3}.txt` (370 passed, 0 failed, rc 0, three times) | `wp09/r4-disk-headroom-mutation-run{2,3}.txt`; run 1's record file is header-only (see below); every run's verdicts are the `PASS: mutation ... observed failing` lines of its GREEN transcript: 46 of 46 observed failing in each run (39 of round 3 + 7 new) |
| T001 probe | `wp09/r4-probe-red.txt` (full suite: 106 passed, 16 failed, per the transcript summary line) | `wp09/r4-probe-green-{1,2,3}.txt` (122 passed, 0 failed, rc 0, three times) | `wp09/r4-probe-mutation-run3.txt`; runs 1 and 2 header-only (see below); 24 of 24 observed failing in each run (20 + 4 new) |

Also: `wp09/r4-real-host-run.txt` (the round-4 scripts against the real host; it rewrote `host-probe.json` and `disk/t001-real-host-need-largest-image.json`, both sha256-sealed in that file), `wp09/r4-reviewer-mutants.txt` (each reviewer survivor RM-A..RM-G and what kills it), `wp09/r4-r1-e2e-scan.{sh,txt}` (statistical scan of the exec-window orphan, see R1), and `wp09/SHA256SUMS` (refreshed: `cd wp09 && sha256sum -c SHA256SUMS`). The round-3 T001 files moved to `wp09/superseded-r4/` (README.txt there); the T004 `gitignore-*` files and `r3-reviewer-mutants.txt` stay (the `.gitignore` and its test did not change in round r4; their headers still fingerprint the round-3 hashes of the other files, which is correct for the moment they were generated).

Header-only mutation records: the scratch files that held the per-mutation detail of probe runs 1-2 and disk run 1 were removed by another session before they were copied (UNCONFIRMED which; the scratch directory name was shared). Each of those record files says so on its last line. The per-run verdicts are in the GREEN transcripts.

What the round changed (finding, then where it is pinned):
- R1 (IMPORTANT) a child INSIDE `execve` reads an EMPTY `/proc/<pid>/cmdline`; `own_child` took the empty read for "not ours", so `stop_chain` signalled nothing, the child was orphaned and the gate printed a false "stopped". Now an empty read is decided by the parent link (this shell) plus a start time not older than this shell's own (`SELF_START`, stat field 22; unreadable means not ours), and `stop_chain` KILLs such a child like a pre-exec one (`pre_exec_child` accepts empty). Pinned by `own_suite`: a shell function `tr` answers empty for ONE pid so the kernel window is deterministic (own child yes; foreign child no; start time older than this shell no; no `SELF_START` no; empty is pre-exec yes; a `timeout` child is not pre-exec), and by a LIVE own child that re-execs itself in a loop (600 reads: RED on the round-3 script 88 of 600 answered "not ours", GREEN 0 of 600, three times). Mutations `own-empty-dropped`, `own-empty-start-dropped`, `preexec-empty-dropped`. Honest limits: the end-to-end scan `r4-r1-e2e-scan.txt` found 0 orphans in 300 trials for BOTH the round-3 and the round-4 script on this host, so it did NOT reproduce the orphan and does not discriminate (UNCONFIRMED end to end; the discriminating evidence is the live exec-loop unit above). The window between the `own_child` check and the KILL (residual named above) remains.
- R2 (IMPORTANT) "no grant" is now only a PROVEN absence: the grant is absent AND its directory is absent or searchable. An untraversable `.audit`, a dangling grant symlink, `.audit` being a file or a dangling symlink are an unreadable grant (`commit_turn_held`). Fixtures (RED on the round-3 script, 4 of 4: the gate wrote the record during a held turn) plus controls (readable `.audit` without a grant, no `.audit` at all: both still write); mutation `grant-enoent-as-none`.
- R3 (IMPORTANT) one decision, `OUT_DIR_IN_USE`, now drives the freeze and the record path. Fixture: empty `DISK_HEADROOM_OUT_DIR` plus a held grant (EVREC_TURN_RUN_ID set and unset) refuses `commit_turn_held`, nothing written anywhere (tree byte-unchanged); golden-FALSE: a non-empty value still bypasses the freeze. Mutation `freeze-outdir-set-semantics` is the reviewer's RM-A. Honest note: these fixtures also pass on the round-3 script (the defect existed only as the mutant).
- R4 the bounded probes' output lives in one private `mktemp -d` directory (mode 0700), removed as soon as the probes finish and by the signal handler; nothing below a shared TMPDIR is predictable. Fixture: FIFOs planted at the old predictable names (RED on the round-3 script: rc 124, the gate hung; GREEN: pass with the real values), and a normal run leaves TMPDIR empty. Mutation `bounded-predictable-name`. RM-B (noclobber dropped) is now equivalent by construction and stays untested by design (see `r4-reviewer-mutants.txt`).
- R5 `probe_host.sh` installs its TERM/INT/HUP trap BEFORE `mktemp` (a TERM landing while `mktemp` runs left `.host-probe.XXXXXX` in this tracked directory, which `.gitignore` does not ignore). Fixture with a `mktemp` stand-in that sends TERM to the probe (RED: residue 1); mutation `trap-after-mktemp`.
- R6 the DEFAULT positive needles are pinned by class on the real host: statfs type plus the same device id as `/dev` (devtmpfs) and as `/sys` (sysfs). A statfs-type-only first version let the dev mutant survive (the scratch directory is itself a tmpfs); the device-id check kills it. Mutations `dev-needle-default-wrong-class`, `sys-needle-default-wrong-class`.
- R7 this README: the stale T004 "OPEN" entry and the wrong residual description are corrected above.
- R8 only the leading and trailing whitespace of `min_free_bytes` is trimmed (tabs and a CR included); whitespace inside the value is refused `config_not_integer` (`100 0`, `1 000`, a tab inside, `10 # comment`). Fixtures (RED on the round-3 script: `100 0`, `1 000` and the tab case were read as a number and the gate passed) plus golden-FALSE: a value padded with spaces, a tab and a CR is accepted. Mutation `conf-whitespace-merged`.
- R9 `user.slice` `memory.max` is the word `max` or a plain decimal number (no leading zeros); anything else (`abc`, `12 34`, `-1`, `max max`, ` max`, `0123`, `1e5`, `0x10`) is `error`, never `present`. Fixtures (RED 8 of 8), golden-FALSE `0` stays `present` number 0; mutation `slice-nonnumeric-as-present`. Round 4 also made the probe suite empty its fixture directories on every invocation: a mutant that left a record behind made the residue checks of LATER mutants fail for the wrong reason.

Recorded as owed (tracked here, not done in round r4):
- R10 / M7 (carried): §11.4.18 companion documents for `probe_host.sh` and for `test_planned_paths_tracked.sh` are neither planned in tasks.md nor present (`docs/scripts/disk_headroom.md` is T009's). Owner decision still open.
- RM-G: a direct depth-2 probe for `/tools/evidence/**/.cache` in the T004 test (INFO; the `.gitignore` and its test are untouched).
- Secrets block coverage (INFO): `keystore.properties`, `*.p8`, `.git-credentials`, `.pgpass`, `service-account*.json`, `*.gpg`, `*.ovpn` are not ignored anywhere; the block is the owner-approved list (OAU-2026-10-05-5), so extending it is an owner decision, not made here.
- INFO: a missing `jq` prints `disk_headroom: jq is required` (rc 2) instead of the `REFUSED reason=dependency_missing` form; an unbounded `jq` read of a grant that is a FIFO (not realistic); T001's text does not yet name `control_positive_dev`/`_sys`, `dependency_missing` or the `--op-id` charset (the code is a superset of the text).
- Test junk: each full disk-suite run left one empty `/tmp/disk_headroom.XXXXXX` (five dirs over five runs, all empty; the suite does not set TMPDIR outside the bounded-signal fixtures). UNCONFIRMED which fixture or mutant leaves it (a mutant without signal handling is the likely source, not verified). Setting TMPDIR to the suite's scratch directory for all runs would contain it; owed. The five directories were removed.
- RM-E equivalent (drop_bdir removed from the signal handler) not re-run: UNCONFIRMED that the suite catches it.
- The T004 test `test_planned_paths_tracked.sh` and `.gitignore` were not changed and not re-run in round r4 (no finding touched them); the round-3 results stand for their own hashes.

## Index refresh of the round-4 cleanup pass (2026-10-06): dispatch, review-provenance r5, runner, smoke and sha256 seal of `wp09/`

`wp09/SHA256SUMS` seals every regular file under `wp09/` (183 lines: the 79 earlier lines are unchanged, 104 lines were appended in this pass). `cd wp09 && sha256sum -c SHA256SUMS` is clean. Three older lines name files outside `wp09/` (`../host-probe.json`, `../disk/t001-real-host-need-largest-image.json`) or the dot file `.gitignore.before`; they are kept as written. Note: `wp09/` also holds files of other tasks than T001/T004 (T003, T005a, T005d, T006 to T008, T094a); the area is named by the task owner of each file, not by the folder.

| Area | Files under `wp09/` (all `$EV`-relative) | Reading guide |
|---|---|---|
| T005a dispatch (event core, `scripts/build/event_core.sh`) | `dispatch-red.txt`, `dispatch-mutations.txt`, `dispatch-green-x3.txt` (round 1); `dispatch-r3-red.txt`, `dispatch-r3-green-x3.txt`, `dispatch-r3-green-x3-final.txt`, `dispatch-r3-mutations.txt`, `dispatch-r3-mutations-rerun.txt` (round 3); `dispatch-r4-red.txt`, `dispatch-r4-green-x3.txt`, `dispatch-r4-mutations.txt`, `dispatch-r4-container.txt`, `dispatch-r4-sha256sums.txt`, `dispatch-r4-notes.md` (round 4, current) | Current is r4; earlier rounds are history. `dispatch-r4-notes.md` lists what was fixed and the evidence names. |
| T094a review provenance (`scripts/review/check_review_provenance.sh`) | `review-provenance-INDEX.txt` (the naming and round guide), `review-provenance-{red,green-x3,mutations}.txt` (round 1, superseded), `-{red,green-r2-x3,mutations}-r2.txt`, `-r3`, `-r4` (and `review-provenance-red-r4-vs-r2input.txt`), and the current round 5: `review-provenance-red-r5.txt`, `review-provenance-green-r5-x3.txt`, `review-provenance-mutations-r5.txt` | Current is r5; the INDEX file states the result of each round. |
| T003 / T007 runner (`run_pinned.sh`) | `runp-RED.txt`, `runp-GREEN-run{1,2,3}.txt`, `runp-GREEN.done`, `runp-mutations-run{1,2,3}.txt` (round 3); `runp-r4-RED.txt`, `runp-r4-GREEN-run{1,2,3}.txt`, `runp-r4-mutations-run{1,2,3}.txt` (round 4, current) | see `wp09/README.md` (round-4 evidence table and tracked deviations) |
| T006 / T008 images and smoke | `build-kcov.txt`, `build-testutil.txt`, `smoke-IMG-*.json`, `smoke-out/<image>/`, `smoke-mutation-kcov-tool.txt`, `smoke-r4-run{1,2,3}.txt`, `smoke-r4-mutation-probe-status.txt`; disk records `disk/t005d-pre-*.json`, `disk/t006-*.json` | see `wp09/README.md` |
| T001 / T004 (containers, `.gitignore`) | see the two round sections above | unchanged |

The identity of each file (what it ran against) is in its own `#` header, not in this index.
