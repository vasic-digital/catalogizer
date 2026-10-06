# T050 round 5 notes (evrec / verify), response to WF5-REVIEW-wp05-evrec (GO-with-fixes)

| Field | Value |
|---|---|
| Date | 2026-10-06 |
| Scope | `tools/evidence/**` and `evidence/wp05` only (no contract, tasks.md or governance edit, nothing staged or committed) |
| Baseline (round 4 tree, as reviewed) | `T050r5-baseline/` (evcore.py sha256 a931b295d1e90ba8) |
| Evidence | `T050r5-red.txt`, `T050r5-green.txt` (x3), `T050r5-mutations.txt`, `T050r5-container.txt`; each file starts with an identity header (sha256 of tools, tests, schema) and ends with `# DONE` |
| Tests | new `tools/evidence/tests/test_evrec_r5.sh` (57 checks); `run_mutations.py` gained 19 round-5 mutants and 8 round-4 mutant edits were re-seated onto the changed code (same mutation, new anchor text) |

## 1. Findings and state

| Finding | State | How |
|---|---|---|
| F1 redaction quadratic | FIXED, limit declared | `kv_secret`: the leading name part and the look-behind are dropped (they only chose where the match starts inside a run; the credential word is found at any offset and the value is group 1, so the matched set is the same). The part after the credential word (`kv_secret`, `json_secret`, `json_secret_sq`) and both name parts of `flag_pair` are bounded to 128 characters. `redact_argv` replaced one `fullmatch` with nested unbounded runs by two linear passes (`_is_cred_flag`). Timing check: 13 byte shapes of 100 000 bytes (dashes, `a-`, `_.`, letters, the credential words repeated, quote prefixes, `-token` repeated, `--`+letters) each finish in under 15 s; RED: 11 of them hit the 15 s timeout on the round-4 code. Functional checks pin that the same credentials are still redacted after a 100 000-dash or letter run. |
| F2 commit-turn guard fails open | FIXED | An empty or whitespace-only `run_id` is a malformed grant (`commit_turn_held`, 76); `EVREC_TURN_RUN_ID` set to empty counts as unset (`_is_holder`). The holder with the equal non-empty id still records. |
| F3 operand layer gaps | PARTLY CLOSED, limits stated precisely (section 3) | `--opt=VALUE` values, directory operands (sorted `relpath NUL sha256` digest, symlinked directories not followed, cap `EVREC_OPERAND_DIR_MAX`, default 20000 files; over the cap a RED/GREEN/MUTATION is refused `operand_directory_too_large` 69, a PROBE skips it) and interpreters reached through a symlink (`is_interpreter` also looks at the basename of `realpath(argv0)`) are closed. |
| F5 anchor beside `EV_LEDGER` | FIXED | `anchor_path()`: `EV_ANCHOR`, else `anchors.jsonl` beside the ledger when `EV_LEDGER` is set, else `$EV/anchors.jsonl`. The `--ledger` branch is unchanged. |
| F6 duplicated run-index row | FIXED | `reconcile --run` counts distinct `seq` values. A run-index row whose `seq` is not an integer is ignored (it crashed with `TypeError: unhashable type` for a list value; found while testing F6). |
| F7 `rerecord` onto an existing directory | FIXED | `out_is_directory` (70) when `<out>/<name>` or `<out>/ledger-seq-map.json` is an existing directory, checked before the lock and any write; `atomic_write` removes its `.tmp.<pid>` file on any failure; an `OSError` from the lock or the writes is `out_unwritable` (70), not a traceback. |
| F4 untokened `reconcile` false PASS on a shared ledger | NOT DECIDED (owner decision, docs/06 rule 3 contract) | Recorded here, section 2. |
| F8 mutant WF5-1 survives | OWED | Section 4. |
| F9 tree identity drift | NOTED | The round-4 evidence headers name a comment-only different `evcore.py` / `test_evrec_r4.sh`; the reviewer proved the logic equal. This round's evidence headers carry the identity of the round-5 tree, which supersedes it. |
| F10 `T050r4-red.txt` echoes the assembled fake credentials | OWED (not edited) | Section 4. |
| F11 hermetic list does not name `test_evrec_r4.sh` / `test_evrec_r5.sh` | OWED | Section 4. |

## 2. F4: owner decision, not taken

An `evrec reconcile --runner-count N` without a run token still counts the whole ledger when no tokened entry exists, so on a ledger shared with another writer it can give `reconciled N == N` for a runner that stored fewer entries than it ran. Round 4 kept this for existing runners; no runner calls `evrec reconcile` today (tasks.md has no caller). The choices are: (a) make the run token mandatory for `reconcile` and for `run` when a ledger is shared, (b) bind the token to the T051a per-run token, (c) keep the legacy behaviour and declare it. This is a docs/06 rule 3 contract rule and was NOT decided here. Track it with the owed rule-10 / `launder` question (round4-notes section 6, item 1).

## 3. Stated limits (replace the over-claim of round4-notes section 1, row N3)

The round-4 sentence "different test bytes cannot share a fingerprint behind an unknown launcher" is withdrawn. What holds: for a command outside every interpreter/launcher list, with no `--test-source`, each plain operand and each `--opt=VALUE` value that is a readable file, and each directory operand within the cap, is hashed into `test_fingerprint`, and argv0 is resolved through symlinks for the interpreter check. What does not hold, and is not claimed:

* test bytes the launcher only discovers (a config file, a rootdir scan, an environment variable, a file named inside an operand file such as a manifest or a wrapper script's own includes) or reads from stdin;
* a short option with an attached value (`-cFILE`) and a value that is not a path;
* inline code passed with `-c` through a renamed COPY of an interpreter binary (a copy is not recognised by name; its own bytes are fingerprinted as argv0, but the inline code is only in argv, which is recorded, not hashed into `test_fingerprint`);
* a directory above the cap (refused for RED/GREEN/MUTATION, skipped for PROBE) and special files (only regular files are hashed);
* a launcher outside the list that is also given the tests only on stdin.

Redaction limit added by F1: a credential word followed by more than 128 further name characters before the `=` or `:`, and a flag name with more than 128 characters before its credential word, are not recognised by the patterns (named variables through `EVREC_REDACT_VARS` remain the exact mechanism). The round-4 residual limits (round4-notes section 5) stand.

## 4. OWED (mutation-adequacy and docs, recorded, not done here)

1. F8: no check pins the legacy empty-`cmdline` compatibility rule (`or cmd == ""` in `_holder_stale`); reviewer mutant WF5-1 survives. Needs a check that writes a lock with `"cmdline":""` for a live pid and expects `lock_held` (75).
2. F10: `T050r4-red.txt` lines 270-283 echo the assembled fake credentials; extend the round-4 m11 declaration to this file or register it as a fixture exception before T040a / gitleaks runs over `evidence/wp05`. The file is a recorded artifact and was not rewritten.
3. F11: `test_evrec_hermetic.sh` lists the earlier suites but not `test_evrec_r4.sh` / `test_evrec_r5.sh`; the reviewer ran r4 under a hostile environment (pass). Not extended here.
4. Round-5 mutants not individually authored: each bound (`{0,128}`) separately for `json_secret_sq` vs `json_secret`, `EVREC_OPERAND_DIR_MAX` parsing refusal (`usage_error` on a non-integer), the symlink-to-directory non-follow rule of `_dir_digest`, the `out_unwritable` lock-acquire branch separately from the write branch, `--opt=VALUE` where the value is a directory.
5. Carried from round 4 (unchanged): round4-notes section 6 items 1-10 except those closed above.

## 5. UNCONFIRMED

* Behaviour of the 128-character bounds on a real credential name longer than 128 characters (none known; the limit is declared, not measured against a corpus).
* Whether any runner relies on a directory operand above 20000 files being silently accepted for a RED/GREEN (it is now refused; no caller exists in tasks.md).
* Timing on a host slower than this one: the 15 s limit per 100 000-byte case has about two orders of magnitude of margin here (the fixed cases finish in well under a second; see the check lines in `T050r5-green.txt`).

## 6. Results (identity header of every `T050r5-*.txt`: evcore.py 5afe608b62b8754b, test_evrec_r5.sh 6f359e57c2ada71b, run_mutations.py 38507697bac4c980; all equal the final files)

* RED (`T050r5-red.txt`, round-4 tools from `T050r5-baseline/`): `test_evrec_r5.sh` 51 checks, 35 FAIL (F1 timeouts, F2, F3, F5, F6, F7); the six older suites 0 FAIL (unchanged behaviour).
* GREEN x3 (`T050r5-green.txt`): 55 / 24 / 140 / 105 / 57 / 73 / 6 checks, 0 failures, rc 0, in all three runs.
* Mutations (`T050r5-mutations.txt`): 197 mutants, 197 caught by a check that names the cause, rc 0. A first sweep had 8 not caught: 2 mutant edits invalidated by a later code change, 1 test bug (the whitespace run_id case used an empty grant, now `{"run_id":" "}`) and 5 mutants whose expected check-name fragment named the success text instead of the failure text; all corrected and the full sweep re-run on the final tree.
* Container (`T050r5-container.txt`, IMG-TESTUTIL via run_pinned.sh, once): the same seven suites plus `test_evidence_record_schema.sh`, 0 failures.
* Side effects: none outside `tools/evidence/**` and `evidence/wp05`; `T048a-green-r3.txt` was not re-captured this round; no `__pycache__`, no repo-root `evidence/`.
