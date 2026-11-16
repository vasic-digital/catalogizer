# nfs_build.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:48Z |
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

`image_ref` is what `up.sh --services nfs` stores as `TI_NFS_IMAGE` for `docker-compose.test-infra.nfs.yml`. Lock entry `IMG-INFRA-NFS` (class service) is in `build/containers/images.lock.yaml` for `localhost/catalogizer-infra-nfs:57392374e84d8212`
(digest = platform_digest `sha256:64c75fe9d3af407547037324a8d8f1af9a6fee43eb3f70a814793eec128c4e61`, written by `scripts/containers/resolve_pin.sh`, PROVISIONAL), and the
`test_containerfiles.sh` checker map has the entry `infra-nfs` -> `IMG-INFRA-NFS` (NFSE2 follow-up). The Containerfile lives in `scripts/test-infra/nfs/`, not under
`build/containers/`, so the checker's directory rules (C1-C5) do not run for it; only the lock entry and the map are added. The lock digest is the local build's digest; the
tag is content-derived, so a rebuild with changed inputs yields a new tag and needs a new lock entry.

## WF17 fix round 5 (revision 2)

- `--need` is validated as a strict integer (`ti_uint`); a trailing option without a value is a usage error (exit 2) instead of an endless loop. Nothing else changed.
