# check_class.sh and the check class tables - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 (round 4) |
| Status | draft, untracked work product of T040b (WP-04); the scripts index `docs/scripts/README.md` and the root `README.md` link are NOT created here (they edit tracked files, deferred) |
| Source | `scripts/repo/check_class.sh`, `scripts/repo/check_classes.tsv`, `scripts/repo/check_exemptions.tsv`, `scripts/repo/fixture_roots.txt`; consumers `scripts/repo/validate_cheap.sh`, `scripts/repo/validate_checks.tsv` |

## Purpose

The CPA stages S2 (`scope_check.sh`) and S3 (`validate_cheap.sh`) must not give every check every declared file: a register database written
only by the engine has no trailing-whitespace rule, an evidence capture is byte-exact, a deliberate-violation fixture exists to violate a check.
The path-class table (plan owner's rule (V)) decides, per path, which checks apply and the byte bound of the large-file check. The loader
`check_class.sh` is the one reader of the tables; no other script restates a class.

## Usage

```bash
scripts/repo/check_class.sh [--exemptions F] [--classes F] [--fixture-roots F] [--checks-registry F] [--check class|<check>] <path>
```

`<path>` is relative to the main-repository root, one path per call (the loader has no `--` form, so the callers validate the path first:
no leading dash, no `..`, no control character; `lib_safe.sh` `safe_relpath`). Without `--check` it prints `class <c>` and one `<check> <apply|skip|bound>` line per check of the closed
set; `--check class` prints the class; `--check <check>` prints `apply`, `skip` or, for `large_file`, the byte bound.

## Resolution

1. A path under a root of `fixture_roots.txt` is class `fixtures`; the root's row names the checks it skips.
2. Otherwise a row of `check_exemptions.tsv` that names the exact path (no glob character) decides; two exact rows of different classes for one
   path are refused with 20 `class_ambiguous` (never decided by row order; the same class twice is fine).
3. Otherwise the glob rows (git-wildmatch: `*` stays inside one directory, `**/` matches zero or more directories); glob rows of two
   different classes that match one path are refused with 20 `class_ambiguous`, never decided by row order.
4. A path no row matches is class `source`, the strictest.

`--checks-registry F` additionally refuses with 20 `check_unknown` a registry row with scope `files` and a mode other than `deferred` that has no row for every class
(never a default). Rows with scope `changeset` are outside the class table.

A fixture-root row needs the four non-empty columns that `fixture_roots.sh` needs (root, checks, reason, adding task), otherwise 20 `table_invalid`; an option
without its value is 20 `usage`, never a traceback.

## Classes and what they apply

Read from `scripts/repo/check_classes.tsv` (yes = applies, no = skipped, number = large-file byte bound):

| check | source | generated | evidence | patches | governance-carrier | fixtures | evidence-ledger | legacy-collection |
|---|---|---|---|---|---|---|---|---|
| secret_fold, private_key | yes | yes | yes | yes | yes | yes | yes | yes |
| merge_conflict | yes | yes | no | yes | yes | yes | yes | yes |
| trailing_whitespace, end_of_file | yes | no | no | no | yes | yes | no | yes |
| check_yaml, check_json | yes | yes | yes | no | no | yes | no | yes |
| shell_parse, anti_bluff, check_pins, bank_validator, go_*, no_false_positive_log, eslint, prettier | yes | no | no | no | no | yes | no | yes |
| revision_header | yes | no | no | no | yes | yes | no | no |
| large_file (bytes) | 1,024,000 | 16,777,216 | 1,024,000 | 1,024,000 | 4,194,304 | 1,024,000 | 33,554,432 | 1,024,000 |

No class exempts a file from the S2 secret fold; only a fixture root's row can, for its own root (T040a). The evidence-blob exception of
the `evidence` class (a blob named by its sha256 and named by a ledger entry has no bound, OD-76) is NOT implemented in this slice.

## The held-table rules (implemented in `validate_cheap.sh`)

A change to `check_classes.tsv`, `check_exemptions.tsv` or `fixture_roots.txt` is an admission-table change. S3 judges a change set with the tables of the
HEAD it starts from; the declared table is used only under the review verdict named by its header line `# review: <verdict path>`:

| Situation | Result |
|---|---|
| the verdict holds `GO` with `blocking_findings` 0 in HEAD, or is declared with `GO` in the same change set | every path is judged with the declared tables |
| the verdict is declared but not `GO`, or a `--held-from` row names it | only the paths held on that verdict use the declared tables; every other path uses the HEAD tables |
| an unheld path fails a check under the HEAD tables that the declared tables would not apply or would pass | exit 20 `table_admits_unheld_path` (path, check); the remedy is to declare the path held on the table's verdict |
| an unheld path passes under the HEAD tables | never refused, whatever the held tables decide |
| no review form | HEAD tables for every path and exit 10 `class_table_unreviewed` for the table change |
| HEAD holds no copy of a table | the helper's own reviewed table, taken from a TRUSTED source (WF3 review I-2), never from the helper's own working tree when it runs in place: (1) `--trusted-tables <dir>` (an approved copy); else (2) when `validate_cheap.sh` lives in a git work tree, the code root's committed HEAD (`git show HEAD:scripts/repo/<table>`); a table that is untracked there, or whose working copy differs from it, is refused: exit 20 `class_table_unreviewed`; else (3) a released snapshot that is no repository: the tables next to the script. `--adopt-working-tables` is the explicit, owner-approved form for the adoption run, before the tables are committed. The working-tree copy of the repository under judgement is NEVER taken as the HEAD table, and an untracked, undeclared table decides nothing. A declared one is a table change under the review rule above (WF2 review I1) |
| a declared legacy root report (exact-path row of class `legacy-collection`) whose content differs from HEAD while the tables in force keep its row | exit 10 `legacy_row_not_dropped` naming the path; dropping the row (a reviewed table change) makes the file class `source`, so it must carry the section 11.4.44 header |

Rows of `docs/issues/**` are glob rows and never trigger `legacy_row_not_dropped`.

## Declared paths and registry commands (round 3, WF2 review B1 and I2)

- A declared path (a change set line, a commit path, a held row, a `--repo` operand) is a LITERAL path: `lib_safe.sh` `safe_declpath` refuses `.`, a `./` prefix,
  a `/./` or `/.` component, a trailing `/`, the glob characters `* ? [` and a leading `:` (exit 20 `unsafe_path`), on top of the `safe_relpath` rules. An
  existing directory that is no gitlink is refused as `declared_directory` (declare its files). `./ev/ledger.jsonl` can therefore no longer dodge `append_only`,
  `./ALL_ISSUES_FIXED.md` can no longer dodge `legacy_row_not_dropped`, and `commit_recursive.sh` can no longer commit files nobody declared.
- A `--held-from` row must name a declared path (`held_row_unmatched` otherwise): a row that matches nothing would leave its file unheld.
- `validate_checks.tsv` command forms: `<script> args...` (the script must be an executable file) or `bash|python3 <script> args...` (the script only has to be
  readable; the registry says how it runs, the file mode is not the gate). The shipped `detect_landmines` row uses the interpreter form because
  `scripts/detect-landmines.sh` is tracked with mode 100644 and no tracked file mode was changed.

## Evidence

Tests: `scripts/repo/tests/test_check_classes.sh` (loader), `test_validate_cheap.sh` (class application and the held-table rules); mutation drivers
`run_wp04_mutations.sh`, `run_wp04b_mutations.sh` and, for round 3, `test_wp04c.sh` with `run_wp04c_mutations.sh`; transcripts under `specs/001-full-project-audit-remediation/evidence/wp04/`.

## Not covered

The moved-legacy-file rule, the S2 side of the held-table rules (the secret fold and the private-key carriers are T040a, `detect-secrets` is absent on the host) and
the pinned-container run of the checks are open; see `evidence/wp04/README.md` and `wp04b-notes.md`.

## Declared symlinks and held rows (round 4, WF3 review B-1, m1, m8)

- `scope_check.sh` (S2) judges the object git will commit: a declared append-only store or `$EV/blobs/<name>` that is a symlink, lies below a symlinked
  directory, or whose HEAD entry is a symlink (a type change) is refused 13 `append_only` / `blob_name`.
- `validate_cheap.sh` (S3) classifies a declared symlink by its link type: in class `evidence` or `evidence-ledger` (or a regular file whose HEAD entry is a
  symlink there) it is `fail symlink <path>` (exit 10); in any other class it is reported as `symlink_not_judged <path>` (status unchanged), never skipped silently.
  `check_revision_headers.sh` reports `symlink_not_judged revision_header <path>` the same way.
- A declared directory is judged by the index entry whose path equals the declared path: a directory whose first entry is a gitlink is still `declared_directory`.
- A `--held-from` row that names no declared path is `held_row_unmatched`, two rows with different verdicts for one path `held_row_duplicate` (both exit 20).
- The `fail` line of a script row names the files the check output mentions, not the first five files the script was given.
