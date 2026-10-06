# T050 fix round 3: what changed, per review finding

Date: 2026-10-05. Review: `WF2-REVIEW-wp05-evrec` (xhigh, NO-GO: 0 blocking, 13 important, 14 minor, 20 surviving reviewer mutants). Scope touched:
`tools/evidence/**` and this directory only. Records of the round: `T050r3-red.txt` (the round-3 tests against the round-2 tools, which are preserved byte for byte,
sha256-pinned, in `T050r3-baseline/`), `T050r3-green.txt` (x3), `T050r3-mutations.txt`, `T048a-green-r3.txt`; each starts with an identity block and ends with `# DONE`.
Host bash 5.3.9 / python 3.14.4 / jsonschema 4.19.2; not run under RUNP IMG-KCOV (absent). This round is UNREVIEWED.

## Findings

| # | What changed (code in `evcore.py`) | Test that pins it |
|---|---|---|
| I1 | `_holder_stale`: a live holder whose `/proc/<pid>/cmdline` is unreadable is NOT proven stale and is never reaped. Test seam `EVREC_PROC_ROOT` (default `/proc`). | `test_evrec_r3.sh` I1 (unreadable-cmdline holder, 3 writers x 6 appends = 18 entries and no duplicate seq, pid reuse reaped, five unreadable-identity forms never reaped); unit cases in `test_evcore_golden.py` |
| I2 | The grant is re-checked at three points: `pre_run` (may reap, once), `post_command` (no reaper) and `in_lock` (no reaper), before any blob is written; blobs are now written inside the lock after the check, ledger validation and record validation. The refusal names `where=`. Test hook `EVREC_FAULT=stall_before_lock` (touches `<ledger>.stalled`, waits for `<ledger>.go`, at most 30 s) makes the window deterministic. | r3 I2 |
| I3 | `verify` accepts only `--ledger FILE` and `--blobs DIR`, each once; anything else is `usage_error` (64). First stdout line names `ledger=` and `blobs=`. | r3 I3 |
| I4 | `verify` is deep: ev/1 schema per entry using the tool's own schema (`EV_SCHEMA` is gone from both tools), canonical-byte form (`non_canonical_line`, also catches a duplicate key), blobs must exist (`blob_missing`, exit 3) and hash to their name (`blob_mismatch`, exit 1), anchor present and not compared is exit 3 (`anchor_not_compared`), a non-integer bound is a usage error, no `assert`-based checks (works under `PYTHONOPTIMIZE`). | r3 I4 |
| I5 | Golden tests: a ledger hashed independently with `jq -S -c \| sha256sum` (non-ASCII cwd, `exit_status` 3) verifies; three wrong constructions (no prev prefix, ASCII-escaped canonical form, hash skipping `exit_status`/`verdict`) are refused as `bad hash at line 1`; two pinned hash literals, per-field coverage, canon bytes, in `test_evcore_golden.py`. | `test_evrec_r3.sh` I5, `test_evrec_golden.sh` |
| I6 | (a) `evrec run` and `verify` list blobs above the `evidence large_file` bound as `owed_relocation blob=<digest> bytes=<n> bound=<b>` (bound from `scripts/repo/check_classes.tsv`, override `EV_BLOB_BOUND`; the relocation itself stays a held, reviewed step). (b) `rerecord` refuses (70, `out_is_shared_store`) an `--out` that is the shared ledger directory or the evidence directory, or whose output files are an input, the shared ledger or the anchor; it takes the output ledger's lock. | r3 I6 (shared ledger byte-unchanged after the refused and after a legitimate run) |
| I7 | Redaction: all spans (named values with every occurrence, plus default patterns) are found on the original bytes and merged, so overlapping or prefix secrets leave no fragment. Default patterns: Bearer, `password/secret/token/api_key` key-value, AWS key id, GitHub token, URL userinfo password, PEM private key. `argv`, both streams, `cwd` and `target_ref` are redacted; `redacted: true` set on any hit. | r3 I7, golden redaction cases |
| I8 | An unresolved target gets the all-zero sentinel (no real digest equals it, nothing is derived from the ref); only `PROBE` may carry it, every other polarity is refused 77 `target_unreadable`; `verify` refuses a non-PROBE entry that carries it (`unresolved_fingerprint`). | r3 I8 |
| I9 | docs/06 rule 11: RED, GREEN and MUTATION with an interpreter `argv[0]` and no declared test source are refused 69 `interpreter_without_test_sources` before the command runs. The interpreter set is a closed list in `evcore.INTERPRETERS` (UNCONFIRMED: complete); a script run directly is its own test bytes. | r3 I9 |
| I10 | `--oracle` requires the caller to also pass `--oracle-independent`; otherwise `oracle_independence_undeclared` (64) before the run. The recorder no longer asserts independence. | r3 I10 |
| I11 | `cmd_run` order: parse (unknown flags, missing values) -> encodability of argv/cwd/item/ref -> `--mutation-json` -> oracle flags -> grant (`pre_run`) -> test sources -> rule 11 -> target fingerprint -> record built and validated against ev/1 for every possible outcome (pass, fail, error) -> THEN the command runs. An outcome-dependent refusal (a GREEN that fails) still happens after the run, but stores no blob and no entry and says `command ran, exit N; nothing stored`. | r3 I11 |
| I12 | `hermetic.sh` (sourced by every test file) unsets every variable the tools read and points `EVREC_REPO_ROOT`, `HOME` and `CPA_HOST_ENTRY` at scratch stand-ins; `test_evrec_hermetic.sh` runs the other suites from a scratch checkout holding a LIVE foreign grant with a hostile environment and asserts 0 calls to the host-entry stand-in. | `test_evrec_hermetic.sh`; mutants H-I12a..e |
| I13 | Not decided. See `T050r3-DR-E1-open-deviation.md`. | - |

## Changes to existing tests (stated, with the reason; no check removed or weakened)

- `test_evrec.sh`, `test_evrec_more.sh`: source `hermetic.sh` (I12); `test_evrec_more.sh` passes `--oracle-independent` wherever it passes `--oracle` (I10: the interface now makes the caller declare it; the checks themselves are unchanged); each ends with a "no call reached the host entry" check. `test_evrec.sh` is 55 checks (was 54), `test_evrec_more.sh` 24 (was 23).
- `run_mutations.py`: the edits of mutants whose target code moved were re-pointed at the new code with the same intent (V2, V4, V5, R3, R10, R11, R18, R19, R21, C1, C2, C4, C7, C9, C10). R3 now removes all three validation points (pre-run, post-run, in-lock), because the first two now also catch what R3 used to be the only defence against; C1 now removes all three grant checks for the same reason.

## Counts (round 3 final tree)

Tests: `test_evrec.sh` 55 checks, `test_evrec_more.sh` 24, `test_evrec_r3.sh` 140, `test_evrec_golden.sh` 44 (python unit and golden cases), `test_evrec_hermetic.sh` 6; GREEN x3 in `T050r3-green.txt`. RED in `T050r3-red.txt`: the same files against the round-2 tools (r3 70 of 140 checks FAIL, golden 6 of 44, more 2 of 24, hermetic 3 of 6).
Mutants: 134 in `run_mutations.py` (51 of the author's, adapted to the new code; the reviewer's 21 as `RV-X*`; one or more `N-*` per round-3 check; `H-*` mutants of the test helper `hermetic.sh`), result in `T050r3-mutations.txt`.

## Findings made by this round's own runs (stated)

- The first capture run was started from a directory that contains a file named `x`; `PROBE` target refs such as `x` are resolved against the cwd, so a fingerprint test went red (hermeticity defect of the tests, review class I12). `hermetic_init` now moves into an empty scratch cwd, `test_evrec_hermetic.sh` runs the suites from a cwd that holds a file `x` (mutant H-I12f), and the aborted capture was discarded and redone.
- The first full mutation run left 14 mutants not CAUGHT-by-named-cause. Eleven were caught by checks whose FAIL text differs from the fragment I had listed (fragments corrected, no test changed for them); five were real gaps and got new checks: the `redacted: true` flag for a stream-only hit (R10, N-I7i), a held grant plus a refusing host entry must stop the command before it runs (C8), a hash-valid but non-canonical line (N-I4f), `EV_BLOB_BOUND` validated before the run (N-m10), a default-environment run lists nothing as owed (H-I12d). H-I12c (hostile `EVREC_LOCK_TIMEOUT=0`) survived because the value only matters under contention; the hostile value is now `not-a-number`, which fails every append if it leaks.
- Host-entry stubbing is not a separate mutant: without the repo-root override (H-I12a) a leaked grant fails the suites and reaches the stand-in; with it, the reaper path is never reached outside the commit-turn fixtures, which set their own `CPA_HOST_ENTRY`.

## Declared deviations and owed items (not hidden)

- Minor 1: a torn final line gives verify exit 1 (as T049's `torn final line exits 1` demands), while docs/06 s16 says an unparsable last line is UNVERIFIED (3). Left at 1; DEVIATION from docs/06 s16, owner question whether the T049 test or s16 wins.
- Minor 6: `reconcile --runner-count N` still compares N with the whole ledger; not usable on a shared ledger. OPEN.
- Minor 7: `ledger.jsonl.lock.guard` and stale `.lock` / `.tmp.<pid>` files are not gitignored under `specs/.../evidence/`. Needs a `.gitignore` entry, outside this change's scope. OWED.
- Minor 8: target fingerprinting and test fingerprinting now stream; the stdout/stderr capture still holds the whole stream in memory (redaction needs the bytes). OPEN.
- Minor 9: the bound is still parsed from `check_classes.tsv` directly, not through `scripts/repo/check_class.sh`. OPEN.
- Minor 12: `T048a-green.txt` is stale; `T048a-green-r3.txt` is a fresh x3 capture with an identity header against the revision 7 contract. The RED of the rev 7 withdrawn-prefix checks cannot be re-created after the fact; stated, not faked.
- Minor 13: `test_evrec_more.sh` was written after the implementation; its efficacy rests on the mutation run, as before.
- Minor 14: the 11.4.18 companion `docs/scripts/evrec.md` / `verify.md` is outside this change's scope. OWED; this file and the module docstring of `evcore.py` carry the interface meanwhile.
- The I1 checks pass vacuously against the round-2 tools in `T050r3-red.txt` (the round-2 code ignores `EVREC_PROC_ROOT` and reads the real, readable `/proc`); their efficacy is proved by mutant `N-I1-unreadable-cmdline-reaped` and `RV-X9`, not by the RED run. The I5 golden-good checks also pass on round 2 (round 2 builds the construction correctly); they pin it, and mutants RV-X1..X3 prove they bite.
- `RV-X18` (rerecord skips the schema check of a re-chained record): the sides are deep-verified first, so the check is reachable only through a stand-in `schema_errors`; it is pinned by a unit test that patches the validator (`test_evcore_golden.py` X18). The defensive check is kept (11.4.124).
- `_DupKey` (duplicate-key rejection while parsing) is shadowed by the canonical-form check for the same input; it only improves the message and is not separately mutation-killable (equivalent mutant, not claimed as a kill).
- UNCONFIRMED: behaviour on a non-Linux host (no `/proc`); on such a host every live holder is now "unreadable" and is waited for until `EVREC_LOCK_TIMEOUT`, never reaped.
