# WP-09 evidence index and tracked deviations (round 4)

| Field | Value |
|---|---|
| Revision | 3 (round 7; round 4 text = revision 1, round 5 to 6 additions = revision 2) |
| Last modified | 2026-10-06T16:30:00Z |
| Date (UTC) | 2026-10-06 |
| Scope | T003 (RUNP test), T005d, T006, T007 (`run_pinned.sh`), T008 (`smoke_probe.sh`, `smoke_images.sh`) |
| Source of the round-4 changes | independent review `WF3-REVIEW-wp09-images-runner` (findings I1 to I6) and `wp04/verify-repos-container-rootcause.md` (cause 3) |
| Author statement | produced by the author, not independently reviewed; the round-4 change set is unreviewed (T011a) |

## Round-4 evidence files

| File | Content |
|---|---|
| `runp-r4-RED.txt` | the new T003 test against the round-3 launcher and probe: 64 failures |
| `runp-r4-GREEN-run{1,2,3}.txt`, `runp-r4-mutations-run{1,2,3}.txt` | the new test against the round-4 launcher, three runs (253 passed, 0 failed each), and the paired mutation records (19 mutants caught, 0 survived, each run) |
| `smoke-r4-run{1,2,3}.txt` | live image smoke through `run_pinned.sh` (container user = host uid) |
| `suites-r4-*.txt` | `test_planned_paths_tracked.sh` and `test_record_deferral.sh` run inside IMG-TESTUTIL and IMG-KCOV through `run_pinned.sh` |
| `smoke-r4-mutation-*.txt` | the smoke probe's old behaviour (exit status ignored) driven through the real smoke run |

Every file carries a header with the sha256 of the launcher, the probe and the test it ran against.

## I6: T006 acceptance NOT met (tracked deviation, not closed)

T006 says IMG-TESTUTIL is built rootless on the remote build host through the T005b dispatcher, brought back by `podman save`
and loaded only after its digest verifies. That did not happen. Recorded precisely, without a faked round trip:

1. The images (IMG-TESTUTIL-BASE, IMG-TESTUTIL, IMG-KCOV; IMG-GO and IMG-SHELLCHECK were pulled, T005d) were built with a plain
   local `podman build` on `anton`, the host this repository is worked on (`hostname` = `anton`), on 2026-10-05 20:11 to
   20:14 UTC. There was no dispatcher (T005b) and no T006a probe at that time. Nothing was saved, transferred or re-verified.
   The lock entries say `resolved_on: anton`; the lock `purpose` text says "built locally on the build host anton, deviation: no
   dispatcher yet".
2. Whether `anton` is the owner's designated remote build host and whether it passes the T006a probe is UNCONFIRMED: the owner
   intake (`decisions/owner-request-list.md`, the §11.4.173 deviation acknowledgement) is still blank. Until the owner
   acknowledges it (HC-0), this item is `Operator-blocked`.
3. Owed, each a separate item and none done here:
   - O1: rebuild IMG-TESTUTIL-BASE, IMG-TESTUTIL and IMG-KCOV through the dispatcher once T005b and T006a exist, bring them back,
     verify digests, then update `build/containers/images.lock.yaml` and re-run `scripts/containers/smoke_images.sh`.
     OR the owner acknowledges the local build as an accepted deviation (HC-0), recorded in the decisions record.
   - O2: re-derive `min_free_bytes` in `scripts/containers/disk_headroom.conf` with the T001 formula from the measured image sizes
     (the file is still marked PROVISIONAL; it is owned by another agent and was not touched in round 4). The lock `size_bytes`
     (the default `--need`) differs from podman's current `.Size` (reviewer m6: 259,193,207 against 291,820,919 for IMG-TESTUTIL,
     283,106,672 against 301,687,664 for IMG-KCOV), so the headroom need is understated by about 11 percent and 6 percent.
   - O3: the `podman build` command lines are not in `build-*.txt` (the logs start at the build output). The Containerfile hashes
     (`6949901c8b1d9112`, `0f210a61b3723d65`) and the content-hash tags identify what was built; the commands themselves are
     UNKNOWN, not reconstructed.

## Image layer was left alone

Round 4 did not rebuild any image and did not change the lock digests: the container user fix is in the launcher
(`--user "$(id -u):$(id -g)"`, with `--userns=keep-id` kept), which makes the images' `USER root` irrelevant at run time.
The `column` command, gawk and the jsonschema pin (root-cause items RC5, RC6b, defence) were not applied: the owner-chosen fix
layer is the script, not the image.

## Not done in round 4 (stated, not hidden)

- Reviewer minors m2 (resolve_pin exit status and a test), m3 (glibc and extra tools in the map), m4 (`--ulimit`, tmpfs size,
  timeout, jobs = 1), m5 (SHA256SUMS and some evidence names), m6, m7 (companion docs), m8 (ShellCheck notes) remain open.
- kcov producing real coverage under the RUNP profile (cap-drop ALL, no-new-privileges, mapped uid) is UNCONFIRMED: only presence
  and version were probed.

## Round 5 fix (WF5 review of wp09-images-runner, GO-with-fixes)

Fixed, each test-first (RED `r5-red.txt`: 18 FAIL in `test_run_pinned.sh` + 10 FAIL in `test_host_checklist.sh` on the unfixed
scripts; GREEN x3 `r5-green-1..3.txt`: test_run_pinned 277 passed / 0 failed and test_host_checklist 153 passed / 0 failed each run,
22 of 22 launcher mutations caught and 22 of 22 checklist mutations CAUGHT, 0 survived; wp10 copies `wp10/r5-green-*.txt`, real-host
record refreshed `wp10/host-checklist.json`, `wp10/r5-host-checklist-run.txt`):

- N1 `host_checklist.sh` C13 fails closed: a lock entry whose `digest` is absent, empty or not `sha256:<lowercase hex>` is status
  `lock_malformed` (counted as bad); a non-empty malformed `platform_digest` likewise; an absent `platform_digest` is allowed
  (documented schema) and then only `digest` can match. An empty value can no longer become a case pattern. Fixtures: neither key,
  empty digest, no prefix, empty hex, non-hex, malformed platform_digest, no platform_digest with a mismatching image, plus the
  false-refusal guard (no platform_digest, digest matches: pass). New mutations `c13dg`, `c13pd`.
- N2 `run_pinned.sh`: `RUNP_PIDS` must be 1..min(8192, `ulimit -u` / 2) (`pids_override_out_of_bounds`, exit 1); the 2048 default is
  clamped to that ceiling; `RUNP_ULIMIT_U` is the test hook. Fixtures: 8192 accepted, 8193 / 4194303 / 999999999999999999 refused,
  ulimit 4000 (2000 ok, 2001 refused), ulimit 1000 (default 500), unlimited. New mutation `pids-ceiling`; `pids-guard` re-pointed.
- N3/m-a `run_pinned.sh` refuses a RESOLVED `--out` containing ':' or whitespace (`out_dir_malformed`, fixture through a symlink; the whitespace half is SUPERSEDED by the WF6 fix F1 below: only ':' and a newline are refused) and a
  checkout path (`$PWD` or its resolution) containing ':' (`source_path_malformed`). New mutations `out-resolved`, `pwd-colon`.
  Note: with a ':' in the checkout the default `--out` also trips `out_dir_malformed`; the `pwd-colon` mutant is therefore caught by the
  reason check only (1 failing check), not by a composed-argv difference.

Owed, not done in round 5 (each needs the owner or a later task):

- m-b (owner decision): `--out` equal to a sanctioned root (`.audit/out`, `.audit/scratch`, the whole `evidence` tree) is accepted; decide
  whether `--out` must be strictly below a root (e.g. a per-op subdirectory).
- m-c (docs-evidence): `host_checklist.sh` lines 5 and 26 cite "owner decision 2026-10-06 ... this host anton, no remote host exists" and mark
  C01/C17/C18/C19 `na`; `decisions/owner-request-list.md` section 1.4 says the choice was relayed 2026-10-05 and is not confirmed in HC-0
  (keeping `thinker.local` / `amber.local` is still open). Reword to cite the relay and drop the `anton` literal, or mark those items
  `unconfirmed` until HC-0. Not changed here (would invalidate the identity of this round's runs without an owner answer).
- X3 mutation adequacy: `smoke_images.sh` has no automated test, so the map needles c1-c5, the missing per-image file case and a pair with
  a missing tool cannot be killed by any test. Add a stub-launcher test for the map (reviewer mutant X3, dropping the uid needle, SURVIVES).
- The round-5 mutation for the `pids` ceiling uses the `RUNP_ULIMIT_U` test hook; a real `ulimit -u` lowering was not exercised (CLOSED by the WF6 fix: a real `ulimit -u 10000` leg is in `test_run_pinned.sh`).

UNCONFIRMED: the owed items above remain open; effort of the WF5 review as dispatched (xhigh) is the reviewer's statement, this fix round's
own effort is not reported by the dispatch path.

## Round 6 fix (WF6 reviews `wp09-checklist-runner` and `provenance-eventcore`; evidence files `wf6fix-*`; notes in `round6-notes.md`)

- F1 `run_pinned.sh`: a space or a TAB in the checkout or `--out` path is accepted (real podman binds both, `wf6fix-podman-path-probe.txt`); `:` stays refused
  (podman rc 125), a newline is refused by launcher policy (one argv element per line); the reasons say which. F2: `RUNP_MEMINFO` and `RUNP_ULIMIT_U` need
  `RUNP_TEST_MODE=1` (`test_hook_outside_test_mode`); an unparsable, zero or unreadable ulimit is `pids_budget_unavailable`, never 8192; the pids floor of 1 has
  a marker (`pids-floor`) and fixtures. F3/F6 `host_checklist.sh` C13: the lock is read with PyYAML, every field validated, digest exactly 64 hex.
- Event core: the checked fsyncs of `effects/` and `effects.log` run on every attempt (EC-1); a FIFO at `consumed/<seq>`, `events.jsonl`, `effects.log`,
  `terminal/callback.state` no longer blocks under a lock (EC-2); fixtures for a DUP redo of a `running` callback (EC-3) and an oversize `progress.json` (EC-4);
  provenance checker: the jq HOME and the python smoke-test isolation have fixtures (PR-1, PR-2).
- RED, GREEN x3 and mutation results: see the `wf6fix-*` files and `round6-notes.md` (Results). Independent review of this round is owed.

## Round 7 (WF7 review `disk-docs-governance`; evidence files `r7-*`; the `n1-*` and `n3-*` files are WF5 N1 to N3 closure evidence of round 6)

Index of the round-6 files that were reachable only through `SHA256SUMS`:

| File | Content and honest limits |
|---|---|
| `n1-red-early.txt`, `n1-red-load.txt` | RED of the single-early-`gcmd`-read form (WF5 N1). The header `test_disk_headroom.sh sha256` names the FIXED test; the run itself used a scratch driver (its sha256 is on the `# driver:` line), so that line, not the test sha, identifies what ran |
| `n1-green-early-{1,2,3}.txt` | GREEN x3 of the fixed wait loop without load |
| `n1-green-load-3.txt` | GREEN under load, ONE run only (GREEN x3 under load is NOT evidenced) |
| `n1-disk-suite-full.txt` | full `test_disk_headroom.sh`: only the summary tail was kept (`370 passed, 0 failed`, `rc=0`). The 46 mutation verdicts of that run are NOT in the file; the WF7 reviewer re-ran the suite and saw 46/46 observed failing, which is the reviewer's statement, not this file's |
| `n3-probe-suite-green.txt` | `test_probe_host.sh`: tail of the run only (`122 passed, 0 failed`); its header says `identity: n1-probe-suite` (a mislabel, the content is the N3 probe run). The 24 mutation verdicts are NOT in the file (the WF7 reviewer saw 24/24 in its own run) |

Transcripts are not edited; the omissions above are stated here instead.

Round 7 changes (author statement, independent review owed, constitution 11.4.142):

- WF7 F7-3 (the N1 fix had no in-repo guard): `test_disk_headroom.sh` now has `wait_gate_image` (the wait loop, factored out), a `bash` shim first on strace's PATH that delays the first exec by 2.5 s so the foreign-image window is real on every host (a check asserts the delay happened), `gate_image_suite` (a unit test with no strace need) and the paired mutation `gate-image-single-read` (the pre-fix single read must fail it). RED: `r7-red-singleread-widened.txt` (the single-read form under the widened window: 4 failures, both tools); GREEN: `r7-green-preexec-driver.txt` (the preexec suite alone: 81 passed, 0 failed) and `r7-disk-full-{1,2,3}.txt` (full suite x3, each `375 passed, 0 failed`, `rc=0`, 47 of 47 mutations CAUGHT and none SURVIVED, mutation record appended to each transcript, run with nice 10 on the shared host). The pass lines were filtered out of the transcripts (`grep -v '^PASS'`); the summary line, the rc and the mutation record are kept.
- WF7 F7-8 (empty `disk_headroom.XXXXXX` left in the caller's TMPDIR): the test now exports a TMPDIR below its own scratch directory, removed by its trap. Which suite leaves the directory is UNCONFIRMED (not bisected); after the three full runs no new `disk_headroom.*` directory existed in `/tmp` (the one present is dated before the fix).
- Docs: the `docs/scripts/` guides were re-aligned to their scripts (host_checklist rev 2: 31 mutations, PyYAML C13, exact-64-hex rule, new statuses; event_core rev 7: `effects_log_not_regular`, `read_regular` reads, fsync on every attempt; check_review_provenance rev 7; index_health, scope_to_lumen_json, verify_repos counts re-measured; stale "draft, untracked" statuses corrected) and `docs/scripts/README.md` was created and linked from the root `README.md` (11.4.212).

Not changed, tracked:

- WF7 F7-6 (owner decision): T001 `test_disk_headroom.sh` is a host-only leg (inside IMG-TESTUTIL the `/dev` device check differs); recording it as host-only in the T008 table is owed to the task owner. No skip logic was added.
- WF7 F7-7 (cross-area, wp02): `mutate_index_health.sh` at HEAD reports `m29-noncanonical-no-control-char-check` SURVIVED; UNCONFIRMED whether equivalent to a round-6 check. Noted in `docs/scripts/index_health.md`.
- WF7 F7-5: 19 of 33 scripts have no guide; listed in `docs/scripts/README.md`.
- `$EV/wp09/SHA256SUMS` now also lists the `r7-*` files.

### Owed docs/04 sync patch: current state (note only, owner-decision text NOT applied)

`evidence/wp06/owed-docs04-sync.diff` was cut against `docs/04-findings-register-design.md` sha256 `e25ebd31...`; the file is now `bd128a40...` (the owner edited it). A `patch --dry-run` of the diff against a copy of the current file (run 2026-10-06, scratch copy) applies 10 of 11 hunks and rejects hunk 8 (at line 962); the rejected hunk text is the exact replacement for that region. The docs/04 text still says `ext_schema_version=4`, so the v5 sync (DDL v5: 43 triggers) remains OWED to the owner; this round did not edit docs/04.
