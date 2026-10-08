# snapshot_manifest.py and check_freeze_snapshot.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T05:00:00Z |
| Status | tracked, new (WP-20, tasks T162 to T165); revision 2 adds the F3/F4 fixes of the independent review WF23 (a malformed manifest and a FIFO are refusals); the independent re-review of revision 2 is owed (constitution 11.4.142); STAND-INS written by the WP-20 test worker; T161 owns both files and may replace them |
| Source | `scripts/register/snapshot_manifest.py`, `scripts/register/check_freeze_snapshot.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

The manifest rule and the re-hash check of the frozen snapshot (tasks T161, round-31 review I1). `snapshot_manifest.py <dir>` prints `{"schema":"snapshot-manifest/1","entries":[{"path","sha256"},...]}` for every regular file and symlink of a directory (a symlink hashed as the sha256 of its `readlink` string, never followed; directories are not entries; entries sorted by byte order; a path that is not UTF-8 exits 20 `freeze_path_unsafe`; a file that is neither regular nor a symlink, a FIFO, socket or device, exits 20 `freeze_special_file` instead of blocking forever on the read). `check_freeze_snapshot.sh <snapshot dir> <manifest>` re-hashes the directory by the same rule and exits 20 with `freeze_snapshot_moved <added|missing|changed> <path>` per difference, 20 `freeze_manifest_invalid` for an unreadable or malformed manifest (never a traceback), 2 usage, 0 when equal. `enumerate_sources.py` and `lead_scan.py` pass the check's reason through (`freeze_manifest_invalid`, `freeze_special_file`, `freeze_path_unsafe`); everything else is `freeze_snapshot_moved`.

## Status (honest)

These two files are the T161 deliverables. They exist now only so that the WP-20 consumers (`enumerate_sources.py`, the T162/T164 tests) can run; they follow the rule text of T161 but their output format (`snapshot-manifest/1`) is **UNCONFIRMED** against the final T161 manifest, and the T161 fixtures s1 to s5 and the paired mutations are not written by this change. Replace, do not extend, when T161 lands.

Known limits of the stand-ins (WF23 class F, recorded as requests to the T161 owner in `$EV/wp20/fix-r2-open-requests.md`): the freeze.json shape is invented (R3 adds `remotes`); the manifest's own sha256 is recorded by T161 and verified by no consumer here; the listing check of `lead_scan.py` compares path sets, not the listing's sha256 (T161's `check_freeze_listing.sh` replaces it, R2); the check and the read are two walks (a time-of-check/time-of-use window that is closed only by the read-only `/src` mount); the T161 fixtures s1-s5 and their mutations are not written. Tests: `scripts/register/tests/test_snapshot_checks.py` (mutants F3, F3b, F4, F4b in `mutate_wf23.py`).
