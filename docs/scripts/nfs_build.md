# nfs_build.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T134), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
| Source | `scripts/test-infra/nfs_build.sh`, `scripts/test-infra/nfs/`; tests `tests/infra/test_roundtrip_nfs.sh` |

## Purpose

Builds the user-space NFS server image of the unprivileged NFS attempt (docs/16 DR-16-2): unfs3 (unfsd 0.11.0, built from the upstream repository at the pinned commit
`ec1660ba33c80d5c67131e163e68834c1a10e243`, verified in the Containerfile) plus rpcbind, Debian bookworm at a dated snapshot with exact package versions. Rootless, on this host, `podman build`
after the disk-headroom gate. The tag is `localhost/catalogizer-infra-nfs:<first 16 hex of sha256 over Containerfile, exports, entrypoint.sh>`.

## Usage

```bash
scripts/test-infra/nfs_build.sh [--need <bytes>]      # prints image=, image_id=, digest=, image_ref=localhost/catalogizer-infra-nfs@sha256:...
```

`image_ref` is what `up.sh --services nfs` stores as `TI_NFS_IMAGE` for `docker-compose.test-infra.nfs.yml`. Owed: a lock entry `IMG-INFRA-NFS` (class service) and the `test_containerfiles.sh` image map
entry, by a reviewed change of the WP-11 files (this task did not edit them).
