# test-infra library and client scripts - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
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

## WF17 fix round 5 (revision 3)

- New in `lib.sh`: the input layer (`LC_ALL=C`; `ti_optval`, `ti_uint`/`ti_uint0`, `ti_abs`, `ti_safe_token`/`ti_safe_word`/`ti_safe_path`), the environment scrub for compose and for `run_pinned` (`ti_envscrub`, `ti_compose`, `ti_runpinned`), the registry-path derivation from `scripts/longops/lib.sh` (`ti_ld_init`), the per-user project lock (`ti_lock`/`ti_unlock`, fd 9 closed in every child by the `podman` wrapper), the ownership scan (`ti_scan`, `ti_rm_resources`: exits 0, 1, 3 unknown, 5 foreign), the keeper helpers (`ti_keeper_start`, `ti_holder_ok`, `ti_live_keeper_in`), the client view helpers (`ti_path_in_view`, `ti_view_dir`) and `ti_exit_on_signals` (INT/TERM/HUP become a normal exit so an entry point's EXIT trap runs).
- The comment about the pod now says what was measured: podman-compose 1.5.0 defaults `in_pod` to true; the compose files set `x-podman: {in_pod: false}` and the scripts pass `--in-pod false`.
- Test hooks (`TI_COMPOSE_FILE`, `TI_CORPUS_CACHE_DIR`, `TI_LOCK`, `TI_SCRIPT_DIR`, `TI_PROBE_CLIENT_DIR`, `TI_RT_CLIENT_DIR`, `TI_TEST_SLEEP_*`) are ignored unless `TI_TEST_MODE=1`.
- Build ids are 1..31 lowercase letters, digits and dashes (ASCII only; `LC_ALL=C` makes the class byte-wise).
