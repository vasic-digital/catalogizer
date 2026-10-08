# check_evidence_binding.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T16:40:48Z |
| Status | new in WF17 fix round 5, in the working tree (not committed); independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `tests/infra/check_evidence_binding.sh`; tests `tests/infra/test_evidence_binding.sh` |

## Purpose

Evidence must bind to the committed files (WF17 TI-H1). A capture's `# sha256 <path> <hash>` identity header is what the file under test hashed to when the capture ran; a reader on a clean checkout can only verify it against the committed content.

## Usage

```bash
tests/infra/check_evidence_binding.sh [--against head|worktree] [--manifests-only] [--root <repo>] <evidence dir>...
```

Checks, per directory (recursively, from git, never `find`): (H) every header hash of every tracked text file equals the sha256 of that path in the reference tree (`ABSENT` requires the path to be absent); (M) every `SHA256SUMS*` entry names a TRACKED file, reads, and has the listed hash, and every tracked file beside or below a manifest (and not below a deeper manifest) is listed in the union of that directory's manifests. `head` (default) judges `git show HEAD:<path>`, i.e. what a clean checkout holds; `worktree` judges the working tree plus untracked non-ignored files (for the stage before the commit). `--manifests-only` skips (H): for HISTORICAL evidence of an earlier round whose headers name the revision it ran against, while its manifests must still verify (same completeness and tracked-only rules). Exit 0 clean; 1 findings (one `FAIL` line each); 2 usage; 3 not a git repository.
