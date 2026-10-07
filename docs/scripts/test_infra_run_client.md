# run_client.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T05:00:00Z |
| Status | committed in 6d5ebb64; revised after the WF12 independent review (NO-GO); a fresh independent review of the revision is owed (constitution 11.4.142) |
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
