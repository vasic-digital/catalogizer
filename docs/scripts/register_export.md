# export.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T17:30:00Z |
| Status | tracked from the WP-06 slice T067; independent review of this revision owed (constitution 11.4.142) |
| Source | `scripts/register/export.sh` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative).

## Purpose

Export and reconciliation of the register (docs/04 sections 11.2 and 11.3) and the drift check.

## Usage

```bash
scripts/register/export.sh [--db docs/<name>.db] [--out-dir docs/<dir>]            # defaults docs/workable_items.db, docs/register
scripts/register/export.sh --check [--db ...] [--out-dir ...]                      # read-only drift check
scripts/register/export.sh --out-mode <absolute dir> [--db-file <name>] [--check]  # out mode: <dir>/<name>, outputs <dir>/export (replay.sh)
```

## Behaviour (one `locked.sh` call inside IMG-TESTUTIL)

`PRAGMA wal_checkpoint(TRUNCATE)`; the engine's `export --db <db> --out-dir <dir>` (`--no-formats` while the image has no pandoc: in P0 the HTML, PDF and DOCX siblings come from the remote `docs render` lane of IMG-DOCS from P2 on, T121a, T176); `reconcile.sh` (the CSV files and `Reconciliation.md`); `export-manifest.sha256`; the engine `diff` on `Issues.md` and `Fixed.md`, which must print "in sync" (else exit 5, no row); then one `reg_export_runs` row (fingerprint, engine version = the engine's `.source.sha256`, container image digest, `OK`) and one `reg_export_files` row per Markdown output (`written`) and per HTML, PDF and DOCX sibling (`written` with its sha256, else `skipped_tool_absent` with the sentinel sha256 of 64 zeros, since the schema needs 64 characters; no sibling file is ever faked).

`--check` refuses (20 `register_not_checkpointed`) while `<db>-wal` is non-empty (a check never checkpoints), works on a copy of the database file inside the image, and prints `STALE` (exit 1, reasons named) when the fingerprint differs from the last run, a recorded `written` file or a manifest line no longer matches the file on disk, or the engine diff is not "in sync"; `OK` (exit 0) otherwise. It writes no row.

## Deviations from docs/04 (owed to the docs/04 owner, UNCONFIRMED until accepted)

- **db_fingerprint**: docs/04 section 11.3 says "sha256 of the checkpointed .db". That value changes with the run's own bookkeeping rows, so every later check would read STALE. The recorded fingerprint is the sha256 of the canonical `.dump` without the `reg_export_runs` / `reg_export_files` rows and their `sqlite_sequence` rows.
- **CSV outputs** have no `reg_export_files` row: the schema's `format` enum is `md`, `html`, `pdf`, `docx`. Their sha256 are held by `export-manifest.sha256`.
- The size projection at 2,010 items (`$EV/wp06/db-size.json`) needs the T168 importer; this slice measures a 4-item fixture only (UNCONFIRMED projection).
- The export header (`**Revision:**` / `**Last modified:**`, `export_header` information item of T067) of the engine's own `Issues.md` is not recorded here.

## Exit codes

0 ok; 1 STALE; 2 usage; 5 engine diff not in sync; 20 refused (`path_invalid`, `register_not_checkpointed`).

## Tests and evidence

`scripts/register/tests/test_export.sh`: identical sha256 on two runs of an unchanged DB, golden reconciliation rows, recorded rows, STALE on a twin, a CSV and a database change, golden-true OK after a re-export, a golden-bad engine stub whose diff is not in sync, out mode. Mutations `mutate_register_ops.sh export`. Evidence `$EV/wp06/export-*`.
