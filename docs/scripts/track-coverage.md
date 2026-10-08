# track-coverage.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:57:44Z |
| Status | new in the working tree (T198), not yet committed; independent review owed (constitution 11.4.142, T208); its row in `docs/scripts/README.md` is owed; review round 1 (WF11, 2026-10-06 UTC): fixes I10, I6 applied; independent re-review owed (constitution 11.4.142) |
| Source | `scripts/coverage/track-coverage.sh`, `scripts/coverage/gocov_merge.py`; test `scripts/coverage/tests/test_track_coverage.sh`. Replaces the role of the old `scripts/track-coverage.sh`, which is left in place and unmodified |

## Purpose
The Go coverage collector for the split lanes (docs/05 7.4 A1). The old script swallowed a failing package with `|| true` and `2>/dev/null`. The rewrite has two phases because a result comes only from the callback record of its build, never from a foreground wait.

## Usage
`track-coverage.sh submit --app A --src DIR --lanes "unit integration" [--packages FILE] --state DIR [--group G] [--image IMG-GO]` submits one build group (one member per lane and package, the compile half `go test -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test`), seals it and returns. `track-coverage.sh collect --state DIR --out DIR [--item ID]` runs when the group callback fires: per member it reads `dispatch.sh status`, checks the binary exists, has `verify_artifact.sh` accept it, then runs it in the run half (TIC) with `-test.coverprofile` from the package directory; profiles are merged (`gocov_merge.py`, counts summed per block) and the result goes through the recorder (`evrec run`).

## Exit statuses
`collect`: 0 ok; 1 failed, with each package named and the reason on stderr: `callback_record_missing`, `build_failed`, `infra_failed (<detail>)`, `cancelled`/blocked kinds, `binary_missing`, `verify_refused`, `package_panicked` (also with exit 0), `package_failed`, `profile_empty`. Healthy packages are still run and merged; `result.json` says `failed` and no baseline is recorded. 2 usage; 1 refusals (`test_hook_outside_test_mode`, `no_submitted_group`, ...).

## Stated limits (11.4.6)
UNCONFIRMED against the real dispatcher: T121a (the compile-half lane rows) and T121b (`verify_artifact.sh`) are not built, so the `status` fields `artifact_dir` and `artifact_name`, the call form `verify_artifact.sh --build-id ID --file PATH`, the group callback that triggers `collect`, and the run-half command are this script's stated interface, exercised only against shims that replay recorded records. The registry row that fires `collect` (`scripts/build/callbacks.tsv`) is owed to the dispatcher owner. Statement coverage from Go's atomic profile is a proxy (11.4.224 C).

## Review round 1 (WF11, 2026-10-06 UTC)
- **I10**: `go list {{.Dir}}` is an absolute path. The collector now normalises `/src/<dir>` to the checkout-relative `<dir>` (compile target `./<pkg>`, run half `cd /src/<dir>`), and refuses an absolute directory outside `/src` (`package_dir_outside_source`). UNCONFIRMED against a real `go list` output in this session (no Go run on the host); proven against the documented absolute form by the new test legs.
- **I6**: the fence file of the application is gated (`check_exclusions.sh --root <app>`) and then APPLIED to the merged profile (`gocov_merge.py summary --exclusions`): the blocks of every file the fence names are dropped from the figure and counted in `excluded` (also in the baseline record). Before, the fence was only recorded by path, so a figure claimed a scope it did not have.
- Test hook added: `TC_FENCE_DIR` (the directory holding `<app>.yaml` fences; honoured only with `TC_TEST_MODE=1`, refused otherwise like the other `TC_*` hooks) so the test can show that a fence the gate refuses fails the collection.

## Review round 5 (WP-23 fix round, 2026-10-07 UTC): the real producers
- **Runs inside the module**: the compile argv is `sh -c 'cd /src/<app> && go test [-tags integration] -c -cover -covermode=atomic -coverpkg=./... -o /out/<lane>/<pkg>.test ./<rel>'`; the repository root has no `go.mod`, so the old argv failed on the real tree. The collector moves to the repository root itself, so it works from any directory, and `--state/--out/--src/--packages` may be relative (made absolute against the caller's directory).
- **Lanes**: membership is read from the real gating of the tests (a `tests/integration` directory, or `//go:build integration` in a test file, parsed as a build expression: `!integration` does not count), never from a flag. A tagged package is compiled with `-tags integration`; the build tags no lane compiles are listed in `gaps.json` (`uncompiled_build_tags`, `packages_without_tests`) and travel into `result.json` and the baseline.
- **State**: `state.json` (`tc-state/1`) carries the nonce, the snapshot, the commit, `expected_members` and `sealed`; INT/TERM during submit leave an unsealed state, and `collect` refuses `group_not_sealed`, `state_incomplete`, `snapshot_skew` (the source changed since submit), `out_not_empty`.
- **Collect**: reads the REAL `dispatch.sh status` (terminal kind/exit_class/reason; the builds root layout), verifies the brought-back tree against the digest of the completed event and the copy against the tree manifest, runs each binary through TIC, merges the profiles, applies the fence only after the T200 gate passed on the snapshot taken into the run directory, and refuses a member the loop dropped (`member_unaccounted`).
- **Recorder**: `--item` is mandatory and must be a register item id; the result is recorded as `BASELINE 1 go_binary result.json --evidence-class runtime --oracle invariant --oracle-independent --test-source gocov_merge.py` through the real `evrec`. `gocov_merge.py baseline` refuses a result whose nonce is not this run's, a status other than ok, and a record naming neither commit nor snapshot; it records the sha256 of the fence that was applied.
- **Hooks** (only with `TC_TEST_MODE=1`): `TC_DISPATCH`, `TC_TIC`, `TC_RECORDER`, `TC_FENCE_DIR`, `TC_GOMOD`, `TC_VERIFY`.
- **Not confirmed**: the remote path (T121a/T121b) is not built; the real-lane leg runs `go list` through TIC and SKIPs by name when the anti-mess sweep reports drift on the host (it ran in the final GREEN runs).
