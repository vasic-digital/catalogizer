# disk_headroom.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T12:00:00Z |
| Status | tracked script (T001), guide written by T009; independent review owed (constitution 11.4.142) |
| Source | `scripts/containers/disk_headroom.sh`; threshold data `scripts/containers/disk_headroom.conf`; test `scripts/containers/tests/test_disk_headroom.sh` |
| Evidence | `specs/001-full-project-audit-remediation/evidence/disk/` (one record per call); `evidence/wp09/` |

## Purpose

Free-space headroom gate (constitution 12.9) run before every image pull or build and by `run_pinned.sh` ([guide](run_pinned.md)). It checks the
rootless podman graphroot (from `podman info`) and the repository filesystem; each must keep at least `min_free_bytes` free after the stated need.

## Usage

```bash
scripts/containers/disk_headroom.sh --need BYTES [--op-id ID]
```

| Option | Meaning |
|---|---|
| `--need BYTES` | size the caller will add; base-10 integer, no leading zeros, at most INT64_MAX; given once |
| `--op-id ID` | record name, `[A-Za-z0-9._-]`, no leading dot, at most 128 characters |

`scripts/containers/disk_headroom.conf` holds `min_free_bytes` as an ABSOLUTE byte count (a percentage is a configuration error; the file records the
derivation; the value is PROVISIONAL until T115 re-derives it with a measured scratch peak).

### Environment

| Variable | Meaning |
|---|---|
| `DISK_HEADROOM_OUT_DIR` | write the record here (CPA sets it); when set and non-empty the commit-turn freeze is not consulted |
| `EV` | evidence root; default record path `$EV/disk/<op_id>.json` |
| `DISK_HEADROOM_CMD_TIMEOUT` (default 30) | seconds bound for `podman info` and `df`; positive integer (0 would mean no timeout and is refused) |
| `DISK_HEADROOM_REAPER_TIMEOUT` (default 60) | seconds bound for the reaper call of the commit-turn freeze |
| `DISK_HEADROOM_CONF`, `DISK_HEADROOM_REPO_ROOT`, `CPA_HOST_ENTRY` | test overrides |

## Exit codes

| Code | Meaning |
|---|---|
| 0 | both filesystems keep the headroom after the need; record written |
| 1 | refused: `disk_below_headroom`, `disk_free_unreadable`, `commit_turn_held` |
| 2 | usage or configuration error: `usage`, `op_id_too_long`, `config_percentage_refused`, `config_missing_min_free_bytes`, `config_not_integer`, `config_duplicate_key`, `config_cmd_timeout`, `config_reaper_timeout`, `dependency_missing` |
| 128+n | stopped by SIGTERM/SIGINT/SIGHUP; nothing written, temp output removed |

## Side effects

Writes one JSON record with before/after values per call. While `.audit/commit_turn.json` names another run (and no out dir is set) it calls the
reaper once and refuses with `commit_turn_held` if the grant stays held, unreadable or non-JSON; an unreachable grant counts as held.
Only processes this script started are signalled, each by an exact pid greater than 1 verified through `/proc` (11.4.263, 11.4.196(D)).

## Troubleshooting

* `disk_below_headroom`: free space on the named filesystem (image prune is an owner action for images of other projects).
* `disk_free_unreadable`: `podman info` or `df` timed out or printed nothing; the gate refuses rather than guess.
