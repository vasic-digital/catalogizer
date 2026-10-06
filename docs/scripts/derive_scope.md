# derive_scope.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06T16:00:00Z (round 7 docs sync); earlier: 2026-10-05 (WF2-REVIEW round 3: m-g) |
| Status | tracked since commit 26755ca5; round 7 documentation sync (status and counts re-measured by the WF7 review; independent review of this revision owed, constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: work product of T017 (WP-02) and its review fixes (DM1-DM6) |
| Source | `scripts/audit/derive_scope.sh` (parser `scripts/audit/org_of.py`, inputs `scripts/audit/own_orgs.txt`, `scripts/audit/own_orgs_pending.txt`) |

## Purpose

Classifies every git submodule at every depth as `own` or `third_party`. The index scope (`scope_to_lumen_json.py`), the repository verifier
and the coverage corpus all consume this one split. It walks `.gitmodules` recursively: the root file, then the `.gitmodules` of every
checked-out submodule. It is read-only: it never writes inside the repository and never touches the network.

## Usage

```bash
scripts/audit/derive_scope.sh [--root DIR] [--out FILE] [--own-orgs FILE] [--pending-orgs FILE]
```

| Option | Default |
|---|---|
| `--root DIR` | git toplevel of `$PWD` |
| `--out FILE` | `specs/001-full-project-audit-remediation/audit/submodules.tsv` under the root |
| `--own-orgs FILE` | `scripts/audit/own_orgs.txt` beside the script (required, one organisation per line, `#` comments) |
| `--pending-orgs FILE` | `scripts/audit/own_orgs_pending.txt` (optional; accounts BLOCKED-ON ODG-15) |

Output: TSV, tab separated, sorted by path, no header, five columns: `path`, `class` (`own|third_party`), `URL`, `alt_class`, `flag`. `class` counts the
pending accounts as third_party (the conservative reading of `own_orgs.txt`); `alt_class` counts them as own; `flag` is `ODG-15` on a row where
the two differ, else `-` (docs/21 IC-30). A module nested under a third_party module is third_party whatever its own URL says (11.4.79(6)).
Organisation match is case-insensitive, the organisation comes from `org_of.py`.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | written |
| 2 | usage (unknown or valueless option, root not a directory, not in a git repository and no `--root`) |
| 3 | fail closed: a URL that cannot be classified (including a relative URL), a module block with no path, an unreadable `.gitmodules` (a `git config -f` error other than "no match"), an unreadable own-orgs file; NOTHING is written to `--out` |

## Side effects

Writes `--out` (and creates its parent directory) only on success, through a temp file removed on exit. Reads `.gitmodules` files with
`git config -f`. Needs bash, git, python3.

## Honest limits

- A submodule that is declared but not checked out (no `.git` entry: missing directory, or the empty directory an uninitialised submodule
  leaves) is LISTED, but its own nested modules cannot be seen. This is reported on stderr as `not-descended <path>` and the exit stays 0;
  nothing of it can enter an index until it is initialised and the script is re-run. A caller that needs a complete tree must treat any
  `not-descended` line as an incomplete run (UNCONFIRMED whether the nested modules of such a module are own or third-party).
- Classification is by URL organisation only; a mirror under another account, a renamed organisation, or a URL that points to a fork is
  classified by what the URL says, not by who controls the repository.
- Accounts of `own_orgs_pending.txt` are an open owner decision (ODG-15): `class` and `alt_class` both ship, no default is chosen here.
- Covered by `scripts/audit/tests/test_derive_scope.sh`; mutants DM1-DM6 of `scripts/audit/tests/mutate_wp02_wp03.sh` (alt-class inheritance,
  `git config` error handling, uninitialised descent, unclassifiable URL, relative-URL guard, case-insensitive match).

## Round 3 (WF2-REVIEW)

- m-g: an unreadable `--own-orgs` or `--pending-orgs` file is exit 3 with a message and nothing written (before, a PermissionError traceback with exit 1). Test: `test_derive_scope.sh` (skipped honestly when run as root); mutant DM7.
- m-m (open): the `not-descended` warning reaches stderr only; the TSV carries no marker, so `scope_to_lumen_json.py` cannot know a TSV was produced while a module was uninitialised. The TSV contract has five fixed columns, so a marker is a contract change: owed to the owner.
