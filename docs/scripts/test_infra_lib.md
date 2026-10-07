# test-infra library and client scripts - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | committed in 6d5ebb64; revised after the WF12 independent review (NO-GO); a fresh independent review of the revision is owed (constitution 11.4.142) |
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

## WF12 review fixes (revision 2)

- `scripts/test-infra/lib.sh` gained `ti_pod`, `ti_rm_resources`, `ti_rm_out_dirs`, `ti_valid_project` (teardown incl. pod and output directories) and `ti_view_dir` (the client view of F3; refuses `.`, `..`, absolute paths, `.env`, `.git`, `.audit/test-infra`, `.audit/out`).
- `scripts/test-infra/client/isolation_probe.sh` reports what a client container can see (existence and readability only, never content).
- `tests/infra/lib.sh` gained `ti_down` (tears down as the owner), `ti_lease_state` (holder identity AND liveness of a lease, never "the claim directory exists"), the fixtures arrays `TI_FOREIGN_PODS` / `TI_FOREIGN_DIRS`, and `TI_FAILFAST=1` (mutant runs end at the first violated check).
