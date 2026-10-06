# T050 round 7 notes (evrec / verify), response to WF7-REVIEW-recorder-r6 (GO-with-fixes)

| Field | Value |
|---|---|
| Date | 2026-10-06 |
| Scope | `tools/evidence/**` and `evidence/wp05` only (nothing staged or committed; the other modified files in the working tree are not mine) |
| Baseline (round 6 tree, as reviewed) | `T050r7-baseline/` (evcore.py sha256 524e4ed4fbdf3038) |
| Evidence | `T050r7-red.txt`, `T050r7-green.txt` (x3), `T050r7-mutations.txt`, `T050r7-container.txt`, plus `T050r7-red-runner.txt` (the W7-6 runner check against the round-6 runner); each starts with an identity header (sha256 of tools, tests, schema) and ends with `# DONE` |
| Tests | new `tools/evidence/tests/test_evrec_r7.sh`, written and run RED against the round-6 tools first; `run_mutations.py` gained 19 round-7 mutants, 5 older mutants re-seated onto the new code, one re-authored, one corrected, one declared equivalent, and an import check |

## 1. Findings and state

| Finding | State | How |
|---|---|---|
| W7-1 `flag_pair` regression (`--password VALUE` after ANSI colour or a name character) | FIXED | New pattern `flag_pair_inrun`: a credential flag INSIDE a name run. One start per run (look-behind), one atomic look-ahead to the first `--` or `_-` of the run (any later marker has a suffix of the same text, so the first decides), one scan for the credential word, then the whole run possessively, blank, value. The SGR terminator `m` is a name character, so `ESC[1m--password V`, `ESC[1;31m...`, `abc--password V`, `a_-password V` (round 5 redacted it, round 6 declared it a limit) are redacted without any ANSI-specific code. Differential, 60 000 random contexts: 0 shapes that round 5 or round 6 redacted are stored now, 808 shapes round 6 stored are redacted now. Timing tests (21 cases, 100/200/400 KB, 10 s limit) on `x--`, `_-`, `a_-`+word, `--`, `x--pas`, `ESC[1m` and others; they finish in about 0.3 s including interpreter start. A paired mutant removes the atomic group and the timing tests kill it. |
| W7-2 byte cap trusts `st_size` | FIXED | `_operand_file_sha_n(p, budget)` counts the bytes actually read and raises `too_large` when the budget is passed; `_dir_digest` pass 2 passes the REMAINING budget, so the read-time count is cumulative over the directory. The `st_size` pass-1 check stays as a fast path. Tested with `/proc/version` (reports size 0): refused over a cap, accepted under it, two such files together refused while each fits. |
| W7-3 plain file operands have no byte cap | FIXED | A plain operand (and `--opt=FILE`) is hashed with the same budget (`EVREC_OPERAND_DIR_BYTES_MAX`); over it a RED/GREEN/MUTATION is the new refusal `operand_file_too_large` (69), a PROBE skips it. A 4 GiB sparse operand is refused in under a second. |
| W7-4 unreadable operand silently dropped | FIXED | `operand_files` no longer drops an operand that fails `os.access(R_OK)`, and `_dir_digest` pass 1 no longer drops an unreadable file: both reach the read and are refused `operand_unreadable` (69) on RED/GREEN/MUTATION (skipped on a PROBE). Covers a mode-000 plain file, a mode-000 file inside a directory, an execute-only directory. `fingerprint_target` also got the `onerror` listing fix: a target directory with an unlistable sub-directory is `target_unreadable` (77), not a fingerprint without it. Declared change in behaviour: any named operand that exists but cannot be read (for example `/root`) now refuses a RED/GREEN/MUTATION. |
| W7-8 `_env_cap` lenient | FIXED | Strictly `[1-9][0-9]*` (ASCII): blanks, `+`, `_`, non-ASCII digits, leading zeros, `0`, `0x..`, `1e3`, `5.0` are `usage_error` (64) for both variables. A very large integer is accepted. An empty value is the default. |
| W7-10 pass 2 opens a file it only checked as regular in pass 1 | FIXED | The file is resolved, opened `O_RDONLY | O_NONBLOCK | O_NOFOLLOW`, and `fstat` must say regular file, so an entry that became a FIFO (or a symlink to one) is refused `unreadable` and never blocks. Tested directly and with the swap simulated (pass 1 made to accept a FIFO). |
| W7-5 mutation adequacy (3 surviving reviewer mutants) | FIXED | New checks: spaced JSON value under a 128/129/5000-character key, empty cap value is the default, cumulative byte cap (three 600-byte files, 1000-byte cap). Paired mutants: `R6-bound-json-run-128` re-authored as the reviewer's WF7-1 form (credential word kept, bound back to 128), `W7-5-empty-cap-not-default`, `W7-5-st-size-total-per-file`. |
| W7-6 regex mutant crashed instead of being caught | FIXED | `run_mutations.py` imports every mutated `evcore.py` (`python -I`) and reports a failure as INVALID. The `N-I7g` bearer mutant is repaired (`(?!x)x` after the leading `(?i)`) and a new check pins a bare `Bearer` token outside an `Authorization` header (X-Api-Auth header, prose, tab). A self-test in `test_evrec_r7.sh` runs the runner on an unimportable and an importable mutant. `T050r7-red-runner.txt` shows the self-test RED against the round-6 runner. |
| W7-7 dispositions of the 3 round-6 non-caught mutants | FIXED | `F1-json-tail-quadratic` is declared EQUIVALENT in `run_mutations.py` (`EQUIVALENT`, printed with its reason, counted separately): `*` vs `*+` after a name run followed by a quote cannot change the match set; I re-checked with 80 000 random cases, 0 differences. `R6-bound-json-run-128` re-authored (above). `WF6-3-dir-digest-drops-relpath` expect fragment corrected to the FAIL text (`name-swapped directories collide`). round6-notes section 1 R6-6/R6-11 over-stated mutant kills; this file supersedes it, and the counts are in `T050r7-mutations.txt`. |
| W7-9 Python version | OWED / UNCONFIRMED | The patterns use possessive quantifiers and atomic groups (Python 3.11+). No minimum is declared in the tool docs and I did not add a startup check. Behaviour on Python 3.10 or older is UNCONFIRMED (no such interpreter here). Owner decision where to declare it (docs/06 or a version guard in evcore). |
| R6-3, F4 | owner decisions, unchanged | see round6-notes / round5-notes |
| F8, F10, F11 | OWED, unchanged | `test_evrec_hermetic.sh` still runs only the older suites (r4 to r7 are not in its list); the reviewer's scratch hostile-environment run of r4/r5/r6 passed. |

## 2. Stated limits (additions to round6-notes section 2)

* `flag_pair_inrun` needs the marker `--` or `_-` inside the run. A single dash after a letter or digit (`abc-password value`, `ESC[1m-password value`) is not recognised (round 5 did not either), because `reset-password link` prose must stay untouched. A flag name run that contains no credential word after the marker is left alone.
* Only plain file operands, directory operands and `--opt=FILE` values are capped. `--test-source` files and argv0 are named explicitly and are hashed without the cap (declared; unchanged).
* The `st_size` pre-check in pass 1 and for plain operands is a fast path only; the real bound is the read-time count. A mutant that removes only the pre-check is observationally equivalent except for time (a 256 MiB read before the refusal), so none was authored.
* An operand whose parent is not searchable (stat fails) is still not seen as an operand (`isfile`/`isdir` are false). UNCONFIRMED whether any real test runner depends on that.
* `O_NOFOLLOW` guards the final component only: an intermediate directory swapped between `realpath` and `open` is not guarded. A read of a regular file on a hung network file system can still block.
* The permission-bit cases (W7-4, and the st_size fast-path cases) cannot run as root and are skipped when `id -u` is 0. The container run was not root here (it ran those cases, no `skip` line), and the host run carries them too.

## 3. OWED / not decided

1. W7-9 Python minimum: owner decision (docs/06) or a version guard.
2. R6-3 (anchor of a relocated ledger) and F4 (untokened `reconcile`): owner decisions, unchanged.
3. F8, F10, F11: unchanged, `test_evrec_hermetic.sh` does not list the r4 to r7 suites.
4. Not individually authored mutants: removal of the `st_size` pre-check (observationally equivalent), `O_NOFOLLOW` on its own (the symlink case is covered through `realpath`).

## 4. UNCONFIRMED

* Behaviour on Python older than 3.11.
* Timing on a host slower than this one (the 10 s limit has two orders of magnitude of margin here).
* Whether the first-marker argument for `flag_pair_inrun` holds for run shapes outside the 60 000 random contexts and the 21 timing shapes (it is an argument plus those measurements, not a proof).
* The mutation run covered the new and the affected mutants only (the list is in `T050r7-mutations.txt`); the other 148 round 1 to 6 mutants were not re-run after these edits, only `--check`ed (all 241 edits apply, compile and import).

## 5. Results

Identity header (sha256 of tools, tests, schema) on every `T050r7-*.txt`; all lines re-checked with `sha256sum -c`.

| Capture | Result |
|---|---|
| `T050r7-red.txt` (current tests against the round-6 tools) | test_evrec_r7.sh: 196 checks, 121 FAIL (it aborts early on the W7-10 FIFO hang, which is itself a FAIL line); the other 8 suites 0 FAIL (they pass on the old tools, as expected) |
| `T050r7-green.txt` x3 | 0 FAIL lines; r7 202/0, r6 102/0, r5 57/0, r4 105/0, r3 140/0, more 24/0, base 55/0, golden 73/0, hermetic 6/0, each three times |
| `T050r7-container.txt` (IMG-TESTUTIL, once) | 0 FAIL lines; r7 202/0 and every other suite as above |
| `T050r7-mutations.txt` | 93 mutants run (the 19 new and every mutant whose edit touches the changed code): caught=92, equivalent=1 (`F1-json-tail-quadratic`, declared), not_caught=0, rc=0 |
| `T050r7-red-runner.txt` | the W7-6 runner self-test FAILs against the round-6 runner (INVALID expected, got CAUGHT) |

How the mutation sweep found test gaps before this capture: the first sweep left 6 mutants uncaught (two flag-name bounds that the new in-run pattern had made unobservable through `--` flags, the two st_size fast-path mutants, a mis-authored mutant whose edit was a no-op, and a mutant caught only by an unrelated check). Each got a check or a corrected mutant (single-dash long-name flags, mode-000 files refused as too large before any open, a robust W7-10 symlink check) and the whole capture was re-run.
