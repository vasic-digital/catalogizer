# track-coverage.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T22:00:00Z |
| Status | new in the working tree (T198), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/coverage/track-coverage.sh`, `scripts/coverage/gocov_merge.py`; test `scripts/coverage/tests/test_track_coverage.sh`. Replaces the role of the old `scripts/track-coverage.sh`, which is left in place and unmodified |

## Purpose
The Go coverage collector for the split lanes (docs/05 7.4 A1). The old script swallowed a failing package with `|| true` and `2>/dev/null`. The rewrite has two phases because a result comes only from the callback record of its build, never from a foreground wait.

## Usage
`track-coverage.sh submit --app A --src DIR --lanes "unit integration" [--packages FILE] --state DIR [--group G] [--image IMG-GO]` submits one build group (one member per lane and package, the compile half `go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test`), seals it and returns. `track-coverage.sh collect --state DIR --out DIR [--item ID]` runs when the group callback fires: per member it reads `dispatch.sh status`, checks the binary exists, has `verify_artifact.sh` accept it, then runs it in the run half (TIC) with `-test.coverprofile` from the package directory; profiles are merged (`gocov_merge.py`, counts summed per block) and the result goes through the recorder (`evrec run`).

## Exit statuses
`collect`: 0 ok; 1 failed, with each package named and the reason on stderr: `callback_record_missing`, `build_failed`, `infra_failed (<detail>)`, `cancelled`/blocked kinds, `binary_missing`, `verify_refused`, `package_panicked` (also with exit 0), `package_failed`, `profile_empty`. Healthy packages are still run and merged; `result.json` says `failed` and no baseline is recorded. 2 usage; 1 refusals (`test_hook_outside_test_mode`, `no_submitted_group`, ...).

## Stated limits (11.4.6)
UNCONFIRMED against the real dispatcher: T121a (the compile-half lane rows) and T121b (`verify_artifact.sh`) are not built, so the `status` fields `artifact_dir` and `artifact_name`, the call form `verify_artifact.sh --build-id ID --file PATH`, the group callback that triggers `collect`, and the run-half command are this script's stated interface, exercised only against shims that replay recorded records. The registry row that fires `collect` (`scripts/build/callbacks.tsv`) is owed to the dispatcher owner. Statement coverage from Go's atomic profile is a proxy (11.4.224 C).
