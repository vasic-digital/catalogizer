# test-infra library and client scripts - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T128-T135), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/lib.sh`, `scripts/test-infra/client/lib.sh`, `scripts/test-infra/client/marker.sh`; tests `tests/infra/lib.sh`, `tests/infra/capture.sh` |

## `scripts/test-infra/lib.sh` (sourced)

`TI_ROOT` (repository root; `TI_ROOT` env overrides it for a mutation copy), `TI_STATE_DIR`, `TI_COMPOSE_FILE`, `TI_LOCK`, `ti_valid_id`, `ti_project`, `ti_state`, `ti_network`, `ti_env_get`, `ti_lo` (the
`scripts/longops` CLI), `ti_refuse` (`test-infra: REFUSED reason=<code>`). Siblings are found next to the calling script, so a copy of the directory with one changed line runs against its own siblings.

## `scripts/test-infra/client/*.sh` (run inside IMG-INFRA-CLIENT)

`lib.sh` (locale `C.UTF-8`, `host_of`, `step`/`finish`, `manifest_sha`, `secret_file` for 0600 credential files in the container's private `/tmp`), `probe_<proto>.sh`, `roundtrip_<proto>.sh`, `marker.sh`
(put / has / hasnot a marker file and row, for the isolation check of the concurrency test), `nas_smb_ro.sh`. No client script is run on the host.

## Test helpers

`tests/infra/lib.sh` (counters, identity header, `ti_new_id`, cleanup trap that downs every started project by label, `ti_tic` = TIC with a bounded retry on the transient refusals `anti_mess_drift` and
`limit_exceeds_envelope`), `tests/infra/capture.sh` (runs one command and stores its output with an identity header: head, run_at, sha256 of the files under test, command, exit code).
