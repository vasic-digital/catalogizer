# up.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T131), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/up.sh`; tests `tests/infra/test_up_down.sh`, `tests/infra/test_concurrency.sh` |

## Purpose

Starts ONE test-infrastructure project `catalogizer-test-<build_id>`: per-project single-owner lease (11.4.119, registered long operation 11.4.232), per-run credentials and ports
(`gen_env.sh`), the deterministic corpus (`seed_corpus.sh` through `TIC tooling unit`), `podman-compose up -d`, then a bounded wait until every started service answers its protocol-level
probe (`probe.sh`). Rootless; images are digest-pinned and never pulled.

## Usage

```bash
scripts/test-infra/up.sh --build-id <id> [--services postgres,redis,ftp,smb,webdav,nfs] [--seed STR] [--timeout S] [--ev-dir DIR]
```

`nfs` adds `docker-compose.test-infra.nfs.yml` and builds the user-space NFS image (`nfs_build.sh`). `minio` is refused (BLOCKED, no obtainable image). `--ev-dir` receives the non-secret
`<project>.ports.env` and `<project>.corpus.sha256`.

## Lease

The purpose key is the project name. A live holder gives exit 3 `lease_held` (a second owner of the SAME project is refused; a DIFFERENT project is admitted at the same time); a dead holder gives
exit 4 `lease_stale` (never taken over silently: `scripts/longops/reap.sh --purpose <project>` decides). The holder is a keeper process that lives until `down.sh`; the registered operation is
`<project>-up-<UTC time>-<pid>` (`TI_OP_ID` of the env file); every service carries the labels `project=catalogizer`, `op_id`, `catalogizer.op_id`, `catalogizer.test_project=<project>`.

## Exits

0 up and every probe passing; 3 lease held; 4 stale lease; 1 failure (the partial project is torn down by label); 2 usage. TIC refusals that are transient on this shared host (`anti_mess_drift`,
`limit_exceeds_envelope`) are retried up to `TI_TIC_RETRIES` times (default 90, 5 s apart).
