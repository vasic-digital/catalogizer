# anti-bluff-scan.sh (wrapper) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T16:00:00Z (round 7 docs sync); earlier: 2026-10-06 |
| Status | tracked since commit 26755ca5; round 7 documentation sync (status and counts re-measured by the WF7 review; independent review of this revision owed, constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: work product of the owner decision of 2026-10-05 (WP-04) |
| Source | `scripts/anti-bluff-scan.sh` over `scripts/audit/anti-bluff-scan.sh`; validators from `scripts/repo/lib_safe.sh`; tests `scripts/audit/tests/test_anti_bluff_scan_wrapper.sh` (45 checks), `mutate_anti_bluff_scan_wrapper.sh` (8 mutants) |

## Purpose

`scripts/anti-bluff-scan.sh` is the path the project `CLAUDE.md` and the constitution name. It calls the static scanner `scripts/audit/anti-bluff-scan.sh`
unchanged and adds `--files-from`, a read-only mode that scans only the listed files.

## Usage

```bash
scripts/anti-bluff-scan.sh [--root DIR] [--files-from FILE] [--] [DIR]
```

Exit 0 = zero findings, 1 = any finding (also when the scanner exits 0 but printed a finding), 2 = refused or inner scanner failure (nothing scanned,
never read as clean). Findings are the scanner's TSV (`file`, `line`, `kind`, `excerpt`) on stdout; refusals are one `anti-bluff-scan: REFUSED reason=<code>`
line on stderr. The root defaults to the git toplevel; without `--files-from` the whole tree is scanned.

## `--files-from FILE`

One path per line, relative to the root. The whole run is refused (rc 2, nothing scanned) when any entry fails: `safe_declpath` (no leading `-` or `/`,
no `..`, no `.` component or `./` prefix, no trailing or doubled `/`, no `* ? [`, no leading `:`, no control character or backslash), a missing or
non-regular file (a FIFO or directory), or a symlink on any component of the path. The list file itself must exist, be readable, not be option-like, and
list at least one path (an empty list scans nothing and must not read as clean). A final line without a newline is still read; duplicates are harmless.
The scan runs on a private temp copy holding exactly the listed files, so a violation in an unlisted file is never seen, the real tree is never
written and the temp directory is removed on exit. Variable state is carried by redirected files, not by pipes.

## Honest limits

- The inner scanner works per file name and path pattern. The relative paths are preserved in the copy, so the path-based rules (for example the integration-directory rule) still apply; rules that need sibling files in the tree (none found in the scanner) would not.
- UNCONFIRMED: whole-tree mode on the full repository was not run here (long); only fixture trees were scanned.
- The scanner's own exclusion list still applies (for example a listed file under `vendor/` is copied but not reported).
