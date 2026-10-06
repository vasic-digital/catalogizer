# org_of.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06T16:00:00Z (round 7 docs sync); earlier: 2026-10-06 (WF3 round 4: stale unproven wording, m-7) |
| Status | tracked since commit 26755ca5; round 7 documentation sync (status and counts re-measured by the WF7 review; independent review of this revision owed, constitution 11.4.142); indexed by `docs/scripts/README.md`; origin: work product of WP-02 / WP-03 review fixes (WF-REVIEW-wp02-wp03, I1, m9) |
| Source | `scripts/audit/org_of.py`; consumers `scripts/audit/derive_scope.sh`, `scripts/repo/verify_repos.sh` |

## Purpose

The single owner-organisation parser. Both the scope classifier (`derive_scope.sh`) and the repository verifier (`verify_repos.sh`) must
agree on which organisation a remote URL belongs to, or a repository could be "own" in one report and "third-party" in the other. This file
is the one place that decides. A test asserts that on every real submodule row the two consumers agree.

Rule: the organisation is the second-last path segment of the URL, lower-cased, tolerating one trailing `.git` and one trailing slash. A URL
that cannot be classified yields no organisation, never a guess: no organisation segment, a relative path starting `./` or `../`, or no slash
at all.

## Usage

```bash
python3 scripts/audit/org_of.py URL [URL ...]      # one output line per URL: the lower-cased organisation, or an empty line
```

```python
import org_of; org_of.org_of(url)                  # -> str, or None when the URL cannot be classified
```

Worked answers (from a run on this host): `git@github.com:Org/r.git` -> `org`; `ssh://git@h:22/org/r/` -> `org`; `https://gitlab.com/a/b/c.git`
-> `b` (second-last segment, not the top group); `../x/r.git` -> empty; `r.git` -> empty; an empty argument -> empty line.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | every argument was processed (an unclassifiable URL is an empty line, not an error) |
| 2 | called with no URL (usage on stderr) |

## Side effects

None. Pure function: no file access, no network, no environment reads.

## Honest limits

- It parses text only. It does not check that the organisation exists, that the remote is reachable, or that the account is owned. Ownership
  is decided by the caller against `scripts/audit/own_orgs.txt` (and `own_orgs_pending.txt`, BLOCKED-ON ODG-15).
- The second-last segment is a convention, not a hosting-provider model: on a nested GitLab group path the answer is the innermost group, not
  the top-level group; a URL with a host and one path segment (`https://host/repo`) answers the HOST as the organisation; a local path
  (`/tmp/x/r.git`) answers its parent directory name. Callers that need provider-aware ownership must not rely on this file.
- The two empty-answer cases (`None` / empty line) are the fail-closed signal: `derive_scope.sh` turns one into exit 3, `verify_repos.sh`
  does not count that remote as owned but reports the repository UNPROVEN (`unproven: [NO-REMOTE-BRANCH]`, exit 14, named on stderr): a remote whose organisation cannot be read is never silently third-party (round 3 m-c; the earlier wording here, "treats the remote as not owned", was stale, WF3 m-7).
- Covered by `scripts/audit/tests/test_derive_scope.sh` (DM1, DM3) and `scripts/repo/tests/test_verify_repos.sh` (I1 cases, the m9 agreement
  check) and the mutants MI1b, MI1c, DM1, DM3 of `scripts/audit/tests/mutate_wp02_wp03.sh`.
