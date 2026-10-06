# Round 7 fix notes: WF7 review `runner-checklist-eventcore-r6` (round-6 fixes of `wp09-checklist-runner` and `provenance-eventcore`)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06T16:00:00Z |
| Last modified | 2026-10-06T17:00:00Z |
| Status | draft: independent review of this change set is owed (producer is not the verifier; constitution 11.4.142) |
| Input | `scratchpad/WF7-REVIEW-runner-checklist-eventcore-r6.md` (verdict GO-with-fixes: 0 blocking, 1 important, 6 minor real-defect / owner-decision, 3 minor mutation-adequacy, info) and its probes |
| Scope | `scripts/containers/{run_pinned,host_checklist}.sh` + tests, `scripts/build/event_core.sh` + `tests/{test_dispatch_events,mutate_dispatch_events}.sh`, `scripts/review/tests/*` (the harness), `docs/scripts/{event_core,check_review_provenance,host_checklist}.md`, evidence under `wp09` and `wp10` only |
| Not touched | `tasks.md`, any git index or history, any credential, `check_review_provenance.sh` itself (no defect found in it), `resolve_pin.sh`, `smoke_*.sh` |

Evidence files carry the prefix `wf7fix-` and a sha256 identity header.
Identities (sha256 prefix): `run_pinned.sh 98e82568`, `test_run_pinned.sh b984e5e3`, `host_checklist.sh 9ffbfaf1`, `test_host_checklist.sh ddc13341`, `event_core.sh 6d8abc16`,
`test_dispatch_events.sh 8c4583bf`, `mutate_dispatch_events.sh fedf7fdc`, `test_check_review_provenance.sh ed5f4e02`, `mutations_check_review_provenance.sh 478b408e`; `check_review_provenance.sh` unchanged (`9f4fdbf8`). `SHA256SUMS` of `wp09` and `wp10` were refreshed.

## Findings and what was done

| id | class | change | proof |
|---|---|---|---|
| F-A (important) | real-defect | `host_checklist.sh` C13: the `platform_digest` no longer matches by itself. The lock `digest` (the one `run_pinned.sh` runs) must be present (`@<digest> ` RepoDigest or ` <digest> ` stored `.Digest`); when the lock lists a `platform_digest` it is an ADDITIONAL requirement. | RED: a well-formed wrong digest + matching platform_digest read `pass` / `present_pinned` (2 fixtures, 4 failing checks). Real host, real podman, one hex character of the IMG-GO digest flipped: now `fail` / `digest_mismatch` and `--strict` rc 1; the pre-fix script said `pass` (`wp10/wf7fix-hc-real-host.txt`). The tracked lock still passes 5/5 `present_pinned` (13 pass, `--strict` rc 0). |
| F-B | real-defect | `run_pinned.sh`: a trailing newline survives the path resolution. `rp <var> <realpath args>` appends a sentinel after the newline realpath prints and sets the variable with `printf -v` (a `$(...)` capture of `rp` would strip it again: found while testing). Used for the checkout path, the state directory and `--out`. | RED: a symlinked checkout whose resolved path ends in a newline composed (rc 0, `-v <link>:/src:ro`), and so did `--out <symlink to a dir ending in a newline>` (bound the newline-stripped, different directory). |
| F-C | real-defect | `run_pinned.sh` lock reader runs `python3 -I -` like the checklist reader. | RED: a `yaml.py` in the checkout root and one on `PYTHONPATH` each made print mode compose `docker.io/evil/forged@sha256:ffff...`. |
| F-D | real-defect | `event_core.sh`: new `excl_write`: an entry already at the temp name is removed (`rm` never opens it), then the file is created `O_CREAT|O_EXCL|O_NOFOLLOW|O_NONBLOCK`. Used for the callback effect temp `tmp/e.<pid>` (under `.cblock`) and `atomic_write`'s `tmp/w.<pid>.<RANDOM>` (under `.lock`). | RED: a FIFO at either name blocked the writer (rc 124, lock held); a symlink at `tmp/e.<pid>` made the callback overwrite the file it pointed to with a timestamp and end `done`; a symlink at the `atomic_write` temp overwrote the victim with the payload. |
| F-F | real-defect (instrument) | `mutations_check_review_provenance.sh`: exit 3 when no mutant ran (a stray `MUT_ONLY` naming nothing), exit 4 for `MUT_ANCHORS_ONLY` (anchors only, nothing killed); exit 0 only when at least one mutant ran and all that ran were killed. | RED: `MUT_ONLY=no-such-id` exit 0 with 0 mutants; `MUT_ANCHORS_ONLY=1` exit 0 with 0 kills. The suite also runs a copy of the harness with the two `MUTGUARD` lines removed and requires exit 0 from it (the check can see the defect). |
| I-2 (info) | real-defect | `--out` that cannot be resolved (symlink loop) is `out_dir_unresolvable`, not a misleading `secret_state_in_mount --out  overlaps`. | RED: the old reason text; GREEN fixture asserts the new one; mutant `x-out-unresolvable` killed. |

## The six surviving reviewer mutants, and the mutants added

| reviewer id | row in the repository | killed by |
|---|---|---|
| RM1 (`case "$PWD$PWD_REAL"` -> `case "$PWD"`) | `x-pwd-real-newline` (test_run_pinned.sh) | new fixture: a symlinked checkout (newline-free logical path) whose RESOLVED path holds a newline; plus the trailing-newline fixture |
| RM2 (raw `--out` newline check dropped) | `x-out-raw-newline` | new fixture: a raw `--out` ending in a newline must be refused by the RAW check (the refusal text must not speak of the "resolved out directory"). Note: after the F-B fix the resolved check also refuses a trailing newline, so the two checks give the same exit and reason; only the message distinguishes them, which is why the fixture asserts it |
| RM3 (`${RUNP_MEMINFO+x}` -> `:+x`) | `x-hook-empty` | new fixtures: set-but-empty `RUNP_MEMINFO` and set-but-empty `RUNP_ULIMIT_U` without the test mode, each alone, reason `test_hook_outside_test_mode` |
| HM1 (`python3 -I -` -> `python3 -`) | `c13iso` (test_host_checklist.sh) | new fixture: a forged `yaml.py` on `PYTHONPATH` that would hide the absent IMG-B |
| HM4 (`\Z` dropped from the digest regex) | `c13digz` | new fixture: `digest: "sha256:<64 hex>\nextra"` must be `lock_malformed` |
| EM1 (`O_NOFOLLOW` dropped in `regular_or_absent`) | `m_r7_regular_or_absent_follow` (mutate_dispatch_events.sh) | new fixtures: `effects.log` as a symlink to an existing file and as a dangling symlink |

Other new rows: `x-lock-isolation`, `x-rp-trailing`, `x-out-unresolvable` (run_pinned), `c13pdreq`, `c13pdalone` (host_checklist: the exact-digest rule), `m_r7_fd1_callback_temp_gt`,
`m_r7_fd2_atomic_temp_gt`, `m_r7_excl_stale_kept` (event_core). The existing mutant `c13pd` stopped being distinguishable by verdict (the new platform_digest requirement also
fails a malformed one), so its case now asserts the STATUS `lock_malformed`: a strengthening, not a weakening.

## Results (every number is in an evidence file)

| leg | RED on the unfixed script (final tests) | GREEN x3 (final scripts and tests) | mutations run (new + affected only) |
|---|---|---|---|
| `run_pinned.sh` (`wf7fix-runp-*`) | 370 passed, 11 failed | 381 passed, 0 failed, three runs | 11 of 11 caught (6 new `x-*` + `test-hooks`, `pwd-colon`, `pwd-newline`, `out-resolved`, `secret`) |
| `host_checklist.sh` (`wp10/wf7fix-hc-*`) | 255 passed, 8 failed (4 on the defect, 4 because the new mutation rows have no marker in the unfixed script) | 263 passed, 0 failed, three runs | 35 of 35 in-suite mutations caught in each run (31 earlier + 4 new) |
| `event_core.sh` (`wf7fix-dispatch-*`) | 306 passed, 4 failed | 310 passed, 0 failed, 0 skipped, three runs | 8 of 8 killed (4 new `m_r7_*` incl. reviewer mutant EM1 + `m_r6_ec2_effects_log_regular_off`, `R4_atomic_mv_notT`, `m_r5_n2_follow_symlink`, `m_r6_ec1_log_fsync_only_on_append`) |
| harness and `check_review_provenance.sh` (`wf7fix-provenance-*`) | 150 passed, 3 failed (the harness fixtures) | 153 passed, 0 failed, DOCCHECKS 16/0, three runs | no script change; the harness is covered by its own fixtures |

RED files are copies of runs made on a `git archive` of `HEAD` (`26755ca5`) with the new tests overlaid (header says so). The fixtures that kill RM1, RM2, RM3, HM1, HM4 and EM1 pass on
the unfixed scripts by design (they are survivors of the OLD tests, not defects of the scripts); the RED failures are only the real defects F-A, F-B, F-C, F-D, F-F, I-2.
`wp10/host-checklist.json` was regenerated by the real script: 13 pass, 0 fail, 4 na, 2 unconfirmed, 5 lock images `present_pinned`, `identity.script_sha256` equals the final script.

## Owner decisions: recorded, NOT decided here

- **F-E** `host_checklist.sh` honours every `CHK_*` override with no test-mode gate and the JSON does not say an override was active (real `ulimit -u 3900`: C09 `fail`; with an exported
  `CHK_ULIMIT_U=100000`: `pass`). Options: the same `*_TEST_MODE` gate as `run_pinned.sh`, or record the active overrides in the JSON. Not changed.
- **F-G** the two lock consumers disagree on a one-component `reference` (`alpine`): the checklist says `lock_malformed`, `run_pinned.sh` accepts it and composes `alpine@sha256:...`.
  Either refuse short names in `run_pinned.sh` or relax the checklist. Not changed.
- **RM1 policy note** (reviewer): the `PWD_REAL` newline half is a POLICY refusal (the logical `$PWD` in the argv has no newline). Kept and now tested; drop it only by an owner decision.
- **I-1 (UNCONFIRMED)** a retried fsync that returns 0 after an earlier EIO on the same page is not proof the page reached the disk (errseq semantics). The robust form re-writes the
  line/effect before the fsync. Not measured (needs a real writeback error, root, dm-flakey); not changed.
- **I-3, I-4** (info, fail-closed): the lock's UNPINNED state `digest: ""` and an unquoted all-digit `tag_intent` are reported `lock_malformed`; wording only, UNCONFIRMED whether the
  lock writer always quotes the tag. Not changed.

## Owed, not done here, and UNCONFIRMED

- **Independent review of this round** (Opus at xhigh via the Workflow path): owed. This note and the evidence are the producer's. The effort of this producer run is not observable by it.
- **EC-2 residuals still open:** the lock-file openers `exec 9>>$d/.lock` / `exec 8>>$d/.cblock` on a FIFO planted AT those names (no lock held yet); the time-of-check window of the
  `awk`/`printf` by path on `effects.log` (the `O_EXCL` temp closes the two temp paths of F-D, not this one); `awk` on a very large `effects.log` is unbounded. A FIFO at the temp name
  is now REMOVED and replaced, not refused: a reviewer may prefer a refusal (decision not taken).
- **Mutation sweeps not re-run in full:** the 127 older `mutate_dispatch_events.sh` rows and the 65 older `mutations_check_review_provenance.sh` rows were not re-run (still anchor:
  `MUT_ANCHORS_ONLY` run inside the suite, 71 rows, exit 4 as designed); the older `run_pinned` markers other than the 5 affected ones (`test-hooks`, `pwd-colon`, `pwd-newline`, `out-resolved`, `secret`) were not re-run (the review ran all 26 and all were caught); whether they still KILL is UNCONFIRMED.
  All 35 `host_checklist` mutations ran (they are part of its suite).
- **Docs:** `docs/scripts/{host_checklist,event_core,check_review_provenance}.md` got a minimal addition each (C13 exact-digest rule and 35 mutations; the F-D / EM1 round-7 paragraph;
  the harness exit codes). Other lines of those guides were edited concurrently by another worker and were not touched. `run_pinned.sh` still has NO companion guide (11.4.18) and
  `docs/scripts/` is outside this task's allowed paths for it: owed. `evidence/wp09/README.md` still titles itself "(round 4)" (F-H, other worker's file, not edited).
- **A real `run_pinned.sh` run mode with real podman** was not exercised (disk headroom refuses on this host, as in the review); print mode and the podman shim are the two oracles.
- **Host load** was 11 to 12 during the runs; the numbers are verdict counts, not timings.
