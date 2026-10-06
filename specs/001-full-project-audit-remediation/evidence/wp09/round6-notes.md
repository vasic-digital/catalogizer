# Round 6 fix notes: WF6 reviews `wp09-checklist-runner` and `provenance-eventcore`

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06T13:40:00Z |
| Last modified | 2026-10-06T14:20:00Z |
| Status | draft: independent review of this change set is owed (producer is not the verifier) |
| Scope | `scripts/containers/{run_pinned,host_checklist}.sh` + tests; `scripts/review/**`; `scripts/build/**` + tests; evidence under `wp09` and `wp10` only |
| Not touched | `docs/scripts/*.md` (see Owed), `tasks.md`, any git index or history, any credential |

Evidence files of this round carry the prefix `wf6fix-`. Every one has a sha256 identity header; `SHA256SUMS` of `wp09` and `wp10` were refreshed.
Identities (sha256 prefix): `run_pinned.sh e3ea0a37`, `host_checklist.sh 588e3ffc`, `test_run_pinned.sh 0d18d9dc`, `test_host_checklist.sh def09bdf`,
`event_core.sh 8850750f`, `test_dispatch_events.sh ab8301dc`, `mutate_dispatch_events.sh 0a170c4b`, `check_review_provenance.sh 9f4fdbf8`,
`test_check_review_provenance.sh 75696612`, `mutations_check_review_provenance.sh 73a817fa`. `bev_crypto.py` is unchanged (`0b805f8b`).

## Decisions (what rule was chosen, and why)

- **F1 whitespace in a bind path.** Probed with REAL rootless podman (`wf6fix-podman-path-probe.txt`, 8 bind-source names): a space, a TAB, `,`, `=`,
  `"` and `+` are bound correctly (rc 0, file written); `:` fails (rc 125, the volume separator). A newline is also bound by podman (rc 0), so it is NOT
  refused because podman fails: it is refused by launcher POLICY, since print mode and the shim log are one argv element per line and an element holding
  a newline would make that two-oracle comparison ambiguous. Result: space and TAB are accepted (no false refusal); `:` -> `source_path_malformed` /
  `out_dir_malformed`; newline -> same reasons with a text that says "refused by policy"; the default-out case is named (the checkout path is what is
  refused, with `source_path_malformed`). The reason no longer says "whitespace". The README line that said "':' or whitespace" is superseded (see README).
- **F2 test hooks.** `RUNP_MEMINFO` and `RUNP_ULIMIT_U` are honoured ONLY with `RUNP_TEST_MODE=1`; set without it (even set-but-empty) the run is refused
  `test_hook_outside_test_mode`. I did not choose "a hook may only lower": the exact-number fixtures need a hook that REPLACES the reading, and a
  lower-only rule would make them depend on the live MemAvailable of the host. An unparsable, zero, signed, padded or empty `RUNP_ULIMIT_U` is
  `pids_budget_unavailable` (the most restrictive outcome, a refusal) and an unreadable REAL `ulimit -u` is the same refusal: never the 8192 ceiling. The
  pids floor of 1 (a ceiling of 0 would be `--pids-limit 0` = unlimited pids in podman, reviewer F4) is now marked `# MUT:pids-floor` and has fixtures
  (ulimit 1, 2, 3). A hook value with an arithmetic injection (`a[$(touch X)]`) executes nothing (fixture; the mutant that drops the refusal DOES execute it).
  Honest boundary: `RUNP_TEST_MODE=1` is itself an environment variable, so this stops an ACCIDENTAL or stray hook (an exported variable), it does not stop a
  caller that deliberately sets both. UNCONFIRMED whether any other launcher of this repo should be able to set the mode: none does today (grep).
- **F3 / F6 lock reader of host_checklist C13.** The awk field splitter is gone. The lock is read with PyYAML (the reader `run_pinned.sh` uses); python
  validates the SHAPE of every entry (a mapping, `id` of the form `IMG-...`, `reference` a repository path, `tag_intent` an OCI tag as a STRING, string
  values, no duplicate id), the shell validates the digests (`sha256:` + exactly 64 lowercase hex, F6). An entry whose first key is not `id` is now
  seen and checked (probe H1); a `|` in `tag_intent` can no longer shift a value into the digest field (probe H6); a YAML-quoted digest is read like
  `run_pinned.sh` reads it (probe H3 was a spurious `lock_malformed`); an unquoted `1.20` tag (a YAML float) is `lock_malformed`, not a silent `1.2`.
  `python3` + PyYAML are now a dependency of item C13 only: without them C13 is `error`/`fail` (`lock_unreadable` / `lock_reader_failed`), never a pass.
  Match of the local image is exact: `@<digest>` followed by a space, or ` <digest> ` as the stored `.Digest`.
- **F5 symlinked checkout.** The `$PWD_REAL` half of the colon check is KEPT as a conservative policy (the reviewer saw real podman accept a symlink to a
  `:` directory as a bind source; I did not re-run that probe: UNCONFIRMED by me). It now has a fixture with an explicit `--out` outside the tree that
  asserts the reason, so the half is tested.
- **EC-1.** The checked fsync of `effects/` now runs on EVERY attempt (moved out of the create branch); the checked fsync of `effects.log` runs on every
  attempt that finds the audit line already present (new `else`). Both retry cases of the reviewer (probes P2, P3) now stay `failed`.
- **EC-2.** Every file read or written under a lock is judged by fstat of an `O_NONBLOCK|O_NOFOLLOW` open: `consumed/<seq>` (now `read_regular`, FIFO ->
  `state_corrupt`), `events.jsonl` (the journal opens `O_NONBLOCK` and refuses a non-regular file, with or without a reader: `journal_failed`, and nothing is
  written into the FIFO), `effects.log` (new `regular_or_absent`, FIFO -> the callback fails `effects_log_not_regular`), `terminal/callback.state`
  (`cbstate` uses `read_regular`: a FIFO is "no state", so a DUP is acknowledged and `resume-callback` replaces the damaged file by a real one through the
  atomic write and completes, one effect). `terminal/state.json`, `callback.json` were already guarded by `-s`/`-f` (a FIFO has size 0); fixtures added.
  The rejected case is a refusal or a failed callback, never a block.

## Mutation-adequacy findings (reviewer survivors)

RVE6 (EC-3), RVE1 (EC-4), RVP5 (PR-1), RVP3 (PR-2), RMX1 (F4), RMX2 (F5), HMX1 and HMX2 (F6) are each now killed by a fixture. Proof that they were
survivors of the PREVIOUS tests, reproduced here for the two provenance ones: the pre-round tests pass 145/0 against both mutants
(`wf6fix-provenance-pre-tests-vs-mutants.txt`).

## Results (this session; each number is in an evidence file)

| leg | RED on the unfixed script (final tests) | GREEN x3 (final scripts and tests) | mutations of the new code |
|---|---|---|---|
| `run_pinned.sh` | 299 passed, 55 failed | 355 passed, 0 failed, three runs | 8 of 8 caught in each run (`test-hooks`, `pwd-newline`, `pwd-colon`, `ulimit-closed`, `pids-floor`, `out-resolved`, `pids-ceiling`, `pids-guard`) |
| `host_checklist.sh` | 215 passed, 30 failed | 245 passed, 0 failed, three runs | 31 of 31 caught (22 earlier + 9 new: `c13shape`, `c13hexlen`, `c13repo`, `c13dig`, `c13digdrop`, `c13seen`, `c13dup`, `c13idfirst`, `c13readerfail`) |
| `event_core.sh` | 295 passed, 8 failed, 0 skipped (`wf6fix-dispatch-red.txt`, against the pre-fix `event_core.sh`) | 303 passed, 0 failed, 0 skipped, three runs | 13 of 13 killed (11 new `m_r6_*` rows + the two rows I had to re-anchor, `R3_journal_follows_symlink_both` and `m_r5_n2_follow_symlink`) |
| `check_review_provenance.sh` | not a RED: the findings are mutation-adequacy | 150 passed, 0 failed, three runs | 6 of 6 killed (`r6pr1`, `r6pr2` new; `r5f1a`, `r5f1b`, `r5f2a`, `r5f2b` re-run) |

One reviewer-style survivor appeared on my own mutant set and was fixed before the final runs: `m_r6_ec2_journal_no_regular_check` first SURVIVED (a FIFO
with a live reader is also refused by the failing `fsync`, so the refusal alone did not show the check); the fixture now also reads the FIFO and asserts
that not one byte was written into it, and the mutant is KILLED.

## Owed, not done here, and UNCONFIRMED

- **Independent review of this round** (T011a style, Opus at xhigh via the Workflow path): owed. This note and the evidence are the producer's.
- **Docs not updated (outside the files I was allowed to touch):** `docs/scripts/host_checklist.md` (F7: no `lock_malformed` for C13, "19 paired mutations" is now 31,
  python3+PyYAML dependency, the C13 detail text), `docs/scripts/event_core.md` (EC-5: header date, the round-5 and round-6 evidence pointers, the effects/ and
  effects.log fsync-on-every-attempt step, the FIFO rules, the journal-failure orphan note and the "resume-callback owns the callback" refusal text from WF5),
  `docs/scripts/check_review_provenance.md` (PR-3: Status "round 5", `Last modified`, "What is still owed (1)", the exit-2 "unusable" line) and a mention of
  `command` next to BASH_ENV in the O4 residual text (PR-4; the script comment was updated, the guide was not). `docs/scripts/run_pinned` has no guide in this
  tree that I looked at: not checked.
- **Residuals of EC-2 that I did not close:** (a) the lock-file openers `exec 9>>$d/.lock` and `exec 8>>$d/.cblock` would still block on a FIFO planted AT
  those names (no lock is held yet at that point, so nothing else waits on it); (b) between `regular_or_absent`/`read_regular` and the next use of a path
  there is a time-of-check window for an attacker with write access to the build directory (the journal and `read_regular` close it by judging the opened
  descriptor; the `effects.log` awk/printf by path do not); (c) a FIFO at `terminal/callback.state` is treated as "no callback state" and repaired by the next
  callback run instead of refused `state_corrupt` (chosen: an idempotent re-run with one keyed effect is safe, and `terminal_ok` already refuses the terminal
  itself when the file is empty); (d) `awk` on a very large `effects.log` is not bounded (the file is fstat-checked as regular, not sized).
- **Mutation sweeps not re-run in full:** the `mutate_dispatch_events.sh` rows (only the 13 new or re-anchored ones ran; 127 older rows did not), the 22 `run_pinned`
  markers (the 8 of the new code ran), the full `mutations_check_review_provenance.sh` (6 of 71 rows ran). What WAS checked for all of them is the anchor: a script
  applying every row of `mutate_dispatch_events.sh` to the final `event_core.sh`/`bev_crypto.py` found ONE row broken by my change
  (`m_r5_n2_follow_symlink`: its old text `os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)` now occurs 3 times) and it was re-anchored and re-run (KILLED); all 71 rows of
  `mutations_check_review_provenance.sh` still anchor exactly once (`MUT_ANCHORS_ONLY=1`, `wf6fix-provenance-mutations.txt`). Whether the 127 older dispatch rows are
  still KILLED (not merely anchored) was not re-run: UNCONFIRMED. Harness switches added: `RUNP_ONLY_MUTATIONS` (test_run_pinned.sh), `MUT_ONLY` and `MUT_ANCHORS_ONLY`
  (mutations_check_review_provenance.sh; the second one was added after the six-row run, which used the file with `MUT_ONLY` only).
- **A real `run_pinned.sh` run in a checkout whose path contains a space** was exercised in print mode and through the podman SHIM only; the real podman bind of
  such a path was probed directly (`wf6fix-podman-path-probe.txt`) but not through the launcher itself.
- **Host load:** all legs ran in parallel on a shared host; the numbers are verdict counts, not timings. RLIMIT_NPROC accounting of rootless keep-id
  containers against the host user was not measured (the author's rationale stays UNCONFIRMED, as in WF6).
- **Still open from earlier rounds, not touched:** m-b (owner), m-c (docs), X3 (no test for `smoke_images.sh`), and the F8 nit that the r5 evidence headers carry
  the assembly time instead of the real run times (the `wf6fix-` files carry real times; the r5 files were left as they were).
- `wp10/host-checklist.json` was regenerated by the real script (13 pass, 0 fail, 4 na, 2 unconfirmed; 5 lock images all `present_pinned`, parser saw 5; its
  `identity.script_sha256` equals the final `host_checklist.sh`). The `r5-*` wp10 files describe the previous script and are historical.
