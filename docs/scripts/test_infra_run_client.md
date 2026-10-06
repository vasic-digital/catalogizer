# run_client.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T128), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
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
