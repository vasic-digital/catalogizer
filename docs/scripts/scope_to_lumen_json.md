# scope_to_lumen_json.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 5 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 round 6 (N6-6); earlier 2026-10-05 (WF2-REVIEW round 3: N-I5, m-g); round 4 2026-10-06: m-3, m-5, m-8) |
| Status | draft, untracked work product of T018 (WP-02) and its review fixes (SM1-SM12); the scripts index `docs/scripts/README.md` and the root `README.md` link are NOT created here (they edit tracked files, deferred) |
| Source | `scripts/audit/scope_to_lumen_json.py` (input `scope.yaml`, TSV from `scripts/audit/derive_scope.sh`) |

## Purpose

The structural index scope (`scope.yaml`, rendered by `scope_render.py`) and the semantic index scope (`.lumenignore`, rendered by
`gen_lumenignore.py`) must come from ONE class source (constitution 11.4.275(B)). This tool turns `scope.yaml` plus the submodule split into
the JSON form `gen_lumenignore.py` reads, and can prove that an existing JSON file lost no class.

## Usage

```bash
scripts/audit/scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --out FILE      # derive
scripts/audit/scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --check FILE    # re-derive and compare exactly
```

Inputs: `scope.yaml` (keys `baseline_excludes{class: [patterns]}`, `project_excludes`, `pathological_excludes`, optional `lumen_allow_roots`),
and the five-column TSV of `derive_scope.sh`.

Output JSON (sorted keys, indent 2, final newline, no timestamp): `allow`, `deny`, `root_files` (true), `classes` (`{class: [deny patterns]}`),
`dropped_negations`. `gen_lumenignore.py` reads only `allow`, `deny`, `root_files`.

- `allow` = the `own` rows of the TSV plus `lumen_allow_roots`; `deny` = every class pattern plus every third_party row nested under an allowed root.
- A pattern that does not start with `/` or `**` is written `**/<pattern>` (the generator would otherwise anchor it at the root).
- A negation (`!x`) is dropped and listed under `dropped_negations`: the generator cannot carry it, so a directory such as `secretmgr/` loses
  semantic coverage. This is recorded, not hidden.
- `--check` is an exact re-derive-and-compare of `allow`, `deny`, `root_files`, `classes` and `dropped_negations`: a missing entry AND an extra one
  (a third-party root in `allow`, an extra deny pattern, a flipped `root_files`) are both reported on stderr, one line each.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | derived and written, or `--check` found no difference |
| 1 | `--check` found a difference (the differences are on stderr) |
| 2 | usage (argparse error; `--out` and `--check` are mutually exclusive and one is required) |
| 3 | fail closed: empty allow-list, unreadable or malformed input (a non-mapping `scope.yaml`, `baseline_excludes` not a mapping of lists, a list key that is not a list, an unreadable or non-object `--check` file) |

## Side effects

Writes `--out` only on success. `--check` writes nothing. Needs python3 and PyYAML.

## Honest limits

- It proves the JSON follows from the scope and the TSV; it does not prove the scope is right, nor that `gen_lumenignore.py` renders the
  JSON faithfully (that generator is out of this script's reach).
- Dropped negations lose semantic coverage by design of the generator; the tool reports them, it cannot fix them.
- The third-party-nested rule only sees third_party rows nested under an ALLOWED root; a third-party module outside every allowed root is not
  in `deny` because it is not in `allow` either.
- Covered by `scripts/audit/tests/test_scope_to_lumen_json.sh`; mutants SM1-SM12 of `scripts/audit/tests/mutate_wp02_wp03.sh`.

## Round 3 (WF2-REVIEW): fail-closed additions

- N-I5 (11.4.79(6)): a `lumen_allow_roots` entry that equals, or lies inside, a `third_party` row of the TSV is refused with exit 3 and a named reason (in `--out` and in `--check`; before, the leak was also re-derived as legal by `--check`). A trailing slash does not hide it.
- A TSV row that has no tab, an empty path, or a class other than `own` / `third_party` is refused with exit 3 (before, such rows were silently ignored, so a mistyped class was neither allowed nor denied under its own root and the code was indexed). Blank lines are tolerated.
- m-g: a `--check` file whose `allow`/`deny`/`dropped_negations` is not a list or whose `classes` is not a map of lists, and an unwritable `--out`, are exit 3 without a traceback.
- Tests: `scripts/audit/tests/test_scope_to_lumen_json.sh`; mutants SV1 to SV9 in `mutate_wp02_wp03.sh`.

## Round 4 (WF3 review m-3, m-5, m-8)

- A path classed BOTH `own` and `third_party` in the TSV is refused (exit 3): it would have been allowed and not denied (m-5a, mutant SM8).
- A nested `third_party` path is written into `deny` LITERALLY: `\`, `*`, `?`, `[`, `]` are backslash-escaped (and a leading `#` or `!`), so `x[1]` denies `x[1]` and not `x1` (m-5b, mutant SM9). Patterns taken from `scope.yaml` stay globs by design.
- `--check` with a non-string list element is exit 3 with a message, no traceback (m-8, SM10); the same for a non-string element INSIDE a `classes` list (round 5, D1: it raised TypeError, rc 1); a duplicated entry (same members, longer list) is a difference, exit 1, named `duplicate` (the comparison is exact, SM11).
- The sibling-root control (`submodules/third_bx` next to the third_party row `submodules/third_b` is accepted, not refused) pins the path boundary of the 11.4.79(6) refusal (m-3, SM12; the source was already correct, the test was missing).
- The runner's SM3 pattern was re-pointed at the current source (m-9); SM3 and the whole `scope` part run without BROKEN.

## Round 6 (N6-6; revision 5)

- A class name in `baseline_excludes` that is not a string (a YAML key `1:` or `null:`) is exit 3 with a named reason (it raised TypeError in `sorted()`, rc 1, and an int-only name derived rc 0 and then crashed its own `--check`).
- Input that is not UTF-8 (the TSV, `scope.yaml`, the `--check` file) is exit 3 without a traceback: every file is opened with `encoding="utf-8"` explicitly (not the locale's) and `UnicodeDecodeError` is part of the unreadable-input refusal.
- `root_files` is compared with its type in `--check`: Python's `1 == True` let `"root_files": 1` pass an "exact" comparison; it is now a difference (exit 1, `root_files` named).
- Tests: `test_scope_to_lumen_json.sh` 51 checks (was 42).
