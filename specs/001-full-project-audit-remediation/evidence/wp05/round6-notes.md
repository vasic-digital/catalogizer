# T050 round 6 notes (evrec / verify), response to WF6-REVIEW-recorder (GO-with-fixes)

| Field | Value |
|---|---|
| Date | 2026-10-06 |
| Scope | `tools/evidence/**` and `evidence/wp05` only (no contract, tasks.md or governance edit, nothing staged or committed) |
| Baseline (round 5 tree, as reviewed) | `T050r6-baseline/` (evcore.py sha256 5afe608b62b8754b) |
| Evidence | `T050r6-red.txt`, `T050r6-green.txt` (x3), `T050r6-mutations.txt`, `T050r6-container.txt`; each starts with an identity header (sha256 of tools, tests, schema) and ends with `# DONE` |
| Tests | new `tools/evidence/tests/test_evrec_r6.sh`, written and run RED against the round-5 tools first; `run_mutations.py` gained 25 round-6 mutants, 6 older edits were re-seated, and it now compiles every mutated Python source |

## 1. Findings and state

| Finding | State | How |
|---|---|---|
| R6-2 128-character bound is fail-open | FIXED, no bound | Every name-bearing pattern (`kv_secret`, `json_secret`, `json_secret_sq`, `flag_pair`) now starts only at the start of a run of name characters (look-behind), checks with one lazy look-ahead that the run holds a credential word, and takes the whole run atomically (possessive). The name before and after the credential word is unbounded and each run is scanned a constant number of times. Redacted in every shape at tails/heads of 128, 129 and 5000 characters: plain `=`, YAML `: `, env dump, flag head, flag tail, JSON double and single quotes (tail and head), an argv element, and a bare credential flag with a long name followed by its value element (`_is_cred_flag` was already unbounded). Negative controls are not redacted (a 5000-char name with no credential word, a value shorter than 6, prose, a value starting with `-`, a long dashed name with no credential word). `json_secret*` values are matched possessively (the quote is excluded from the value class, so this changes no match). |
| R6-1 `jwt` quadratic | FIXED | One three-part token per name run: look-behind for the run start, one look-ahead for `. segment{8,} .`, a lazy prefix to the first `eyJ` that begins the run or follows a `-`, then possessive segments. Redacted span = from that `eyJ` (group 1), so a JWT that follows `-` is still redacted, including after a 100 000-dash run. 400 KB of `-eyJ`: ms-scale (round 5: 51 s). Timing cases at 100/200/400 KB for `-eyJ` and dashes and 400 KB for 16 further shapes, 10 s limit each. |
| R6-3 F5 false-null moved | NOT DECIDED (owner decision, docs/06 rule) | Unchanged from round 5. Case: `EV_LEDGER` outside `$EV`, `EV_ANCHOR` unset, anchor only at `$EV/anchors.jsonl` gives `OK` rc 0 (round 4 gave 3 `anchor_not_compared`). Options: (a) with `EV_ANCHOR` unset return 3 when either candidate anchor (beside the ledger, or `$EV/anchors.jsonl`) is non-empty (conservative), (b) declare which anchor binds a relocated ledger. Both change a docs/06 contract rule and were not taken here. Track with F4 and the rule-10/`launder` question. |
| R6-4 traceback paths in operand hashing | FIXED | Directory entries are hashed by `os.fsencode(relpath)` (a non-UTF-8 name records, different non-UTF-8 names give different fingerprints; the same encode fix applied to `fingerprint_target`, which had the identical pattern for target directories). A directory that cannot be listed (`os.walk` `onerror`) and a file that cannot be read are the named refusal `operand_unreadable` (69) on RED/GREEN/MUTATION and are skipped on a PROBE, the same rule as the cap. A plain file operand that cannot be read follows the same rule. FIFOs and other non-regular entries are never opened. |
| R6-5 directory cost bounded only by file count | FIXED | Two passes: the first only lists and stats; entries (files of any kind and subdirectories) are counted against `EVREC_OPERAND_DIR_MAX` (default 20000) and the bytes of regular files against the new `EVREC_OPERAND_DIR_BYTES_MAX` (default 268435456), both before anything is hashed. Over either cap: `operand_directory_too_large` (69) on RED/GREEN/MUTATION, skipped on a PROBE. A 4 GiB file is refused in well under a second (round 5: 3.2 s, then recorded). Declared change: the count was "readable regular files", it is now "entries visited". |
| R6-6 / R6-11 mutation adequacy | FIXED | Checks that kill reviewer mutants WF6-3 (RED {a.sh pass, t.sh fail} and GREEN {t.sh pass, z.sh fail} must differ), WF6-5 (two-level symlink chain refused), WF6-6 (MUTATION refused over the cap), WF6-7 (N files accepted, N+1 refused, and the opposite off-by-one), WF6-4 (`_is_holder({"run_id":""})`), the bound mutants WF6-1/WF6-2 (as bounds reintroduced into the unbounded patterns) and a 4 GiB/byte-cap pair. `run_mutations.py` compiles every mutated `.py` source and reports a mutant that does not compile as INVALID (not caught). The broken author mutant `F7-existing-directory-not-refused` (its edit left `if False:: an existing directory ...`, a SyntaxError that "caught" by crashing every evrec call) is repaired to `if False:` plus the original comment; the round-5 claim "197 caught by a check that names the cause" was over-stated for that one mutant and is superseded by `T050r6-mutations.txt`. |
| R6-7 dead `_NAMEMAX` | FIXED | Removed (the patterns are built from `_NM` / `_FM`; a check pins that `_NAMEMAX` is not defined). |
| R6-8 `EVREC_OPERAND_DIR_MAX` validation | FIXED | `EVREC_OPERAND_DIR_MAX` and the new `EVREC_OPERAND_DIR_BYTES_MAX` are validated when evrec starts (any subcommand): `0`, negative and non-integers are `usage_error` (64), an empty value is the default. `hermetic.sh` unsets the new variable too. |
| R6-9 hardlinks | DECLARED | A hardlink of an interpreter behaves like the renamed copy (same inode, new name): not detected by name, bytes fingerprinted as argv0, inline `-c` code not hashed. Added to the `is_interpreter` docstring and to the stated limits below. |
| R6-10 | none needed | The F10 class did not recur in round 5; round 6 evidence uses assembled fake values that no check prints. |

## 2. Stated limits (additions and changes to round5-notes section 3)

* The redaction name limit of round 5 ("more than 128 further name characters") is WITHDRAWN: there is no length limit on the name around a credential word. Remaining redaction limits are those of round 4 (round4-notes section 5): the credential-word list, encoded or split secrets, short flags such as `-p`, values that are not recognisable.
* `flag_pair` now starts a flag only where no name character (`[A-Za-z0-9_-]`) precedes the dash; round 5 allowed a flag directly after `_`. A flag that directly follows an underscore (`a_-password value`) is therefore not recognised by that pattern (it is by `kv_secret` only when written `a_-password=value`). The change is what makes the pattern exactly linear on dashes.
* `jwt`: the redacted span starts at the first `eyJ` of the run (or after a `-`); text before that stays. A JWT whose first segment is preceded by an alphanumeric or `_` (no boundary) is not recognised, as in round 5.
* Hardlinks of an interpreter are not detected (R6-9), like renamed copies.
* Directory operands: entries are counted, not only files; FIFOs/sockets/devices are not opened and are not hashed; a symlink that points at a special-looking file (for example `/proc/self/mem`) that cannot be read is `operand_unreadable`.
* The fuzz of the reviewer covered only short units; absence of other super-linear patterns for longer units remains UNCONFIRMED beyond the 25 timed shapes and the 400 KB sizes tested here.

## 3. OWED / not decided

1. R6-3 (above): owner decision.
2. F4 untokened `reconcile`: owner decision (round5-notes section 2), unchanged.
3. F8, F10, F11 (round5-notes section 4): unchanged; `test_evrec_hermetic.sh` still lists only the earlier suites, `capture_evidence.sh` now runs `test_evrec_r6.sh` too.
4. Mutants not individually authored: `json_secret_sq` bound separately from `json_secret`, `-{1,2}+` flag-dash count, the second-pass hashing order in `_dir_digest`.

## 4. UNCONFIRMED

* Timing on a host slower than this one: the 10 s limit per case has two to three orders of magnitude of margin here (the cases finish in tens of milliseconds).
* Whether any real credential name exceeds the formerly bounded lengths (irrelevant now: no bound).
* The full mutation sweep is run once on the final tree; see `T050r6-mutations.txt` for the totals.

## 5. Results

See the identity header of every `T050r6-*.txt` (they name the final `evcore.py`, `test_evrec_r6.sh` and `run_mutations.py`). The counts are in the report that accompanied this change (RED/GREEN totals, mutation totals, container run).
