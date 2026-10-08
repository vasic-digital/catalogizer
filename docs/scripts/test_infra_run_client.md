# run_client.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/run_client.sh`; tests `tests/infra/test_probes.sh` |

## Purpose

Runs ONE protocol-client command in the interpreter-class image IMG-INFRA-CLIENT, attached to a project's compose network, through `scripts/containers/run_pinned.sh` (rootless, digest-pinned,
resource-limited, disk gate first). The composed argv is read with `RUNP_PRINT_ARGV=1` and gets, before the image, `--network <project>_test-network`, `--env-file <per-run env>` (credentials never in argv),
the labels `op_id` (the stack's registered operation) and `catalogizer.test_project`, and the corpus manifest mounted read-only at `/manifest.sha256`.

## Usage

```bash
scripts/test-infra/run_client.sh --build-id <id> [--out DIR] [--image IMG-ID] -- <command word>...
```

Refusals (`test-infra: REFUSED reason=<code>`, exit 1): `client_image_not_interpreter_class` (a service-class image), `not_the_infra_client` (any image other than IMG-INFRA-CLIENT), `image_id_unknown`,
`project_not_up`, `network_absent`, `run_pinned_failed`. The client scripts live in `scripts/test-infra/client/` and read the repository at `/src` (read-only).

## WF12 review fixes (revision 2)

- F3: the container's `/src` is a scratch VIEW, not the repository. run_pinned binds its working directory at `/src`, and the repository holds the real `.env` and every project's credential file. The view contains only the directories the command names under `/src/` (they must lie under `scripts/test-infra/` or `.audit/scratch/`, else `REFUSED reason=client_path_not_in_view`; a missing script is `client_script_missing`). The view is removed on exit (the script no longer `exec`s). The project's own credentials arrive only through `--env-file`. Test: `tests/infra/test_client_isolation.sh`.

## WF17 fix round 5 (revision 3)

- Correction (TI-I5): the client does NOT read the repository at `/src`; `/src` is a scratch view holding only the directories the command names (see WF12 F3 above). The view is built from canonical paths (a symlink, an empty, `.` or `..` component, an env-file word or a path outside `scripts/test-infra/` and `.audit/scratch/` is `client_path_not_in_view`), a newline in a command word is refused, and `tests/infra/test_client_isolation.sh` now judges EXPOSURE (what the container can actually reach, including the mount set and the view's contents) instead of substrings of the script.
- The container's `catalogizer.op_id` label is REPLACED (not added) by the stack's operation so the anti-mess sweep matches it to the stack; the `catalogizer.test_root` label is added; the environment passes through `ti_runpinned` (scrubbed); fd 9 (the project lock) is closed in the child; INT/TERM/HUP run the view cleanup (`ti_exit_on_signals`).
