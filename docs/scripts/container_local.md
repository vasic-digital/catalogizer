# scripts/container-local.sh

| Field | Value |
|---|---|
| Script | `scripts/container-local.sh` |
| Test | `scripts/tests/test_container_local.sh` |
| Revision | 1 (2026-10-28) |
| Status | active |

## Purpose

Builds and tests Catalogizer inside rootless podman on the LOCAL host; nothing compiles or installs on the bare host (CLAUDE.md section 11.4.173, section 11.4.161).

## Usage

```
scripts/container-local.sh [--dry-run] <api-build|api-test|web-build|web-test>
```

`--dry-run` prints the exact `podman run` command lines and runs nothing.

## Environment

| Variable | Default | Meaning |
|---|---|---|
| `GO_IMAGE` | `docker.io/library/golang:1.26` | Go image (go.mod needs >= 1.25.7) |
| `NODE_IMAGE` | `docker.io/library/node:22` | Node image |
| `CAT_CACHE` | `~/.cache/catalogizer-build` | Go module/build and npm caches |
| `CAT_OUT` | `<repo>/deploy/artifacts` | Output directory |
| `CAT_MEM` / `CAT_CPUS` | `8g` / `4` | Per-container limits (host-safety, section 12.6) |

## Outputs

`api-build` -> `$CAT_OUT/catalog-api`; `api-test` -> `$CAT_OUT/api-test.json`; `web-build` -> `$CAT_OUT/catalog-web-dist`; `web-test` -> `$CAT_OUT/web-test.log`.

## Safety properties

* Containers run with `--userns=keep-id` (non-root), never rootful, never docker or sudo.
* Dependency download steps get network; test execution runs with `--network none`.
* Submodules named by `catalog-api/go.mod` (`replace`) and `catalog-web/package.json` (`file:`) must be initialised first.

## Relation to the builder image

`Build/lib/orchestrator.sh` (used by `scripts/release-build.sh`) runs `docker/Dockerfile.builder`, whose user is uid 1000 (`builder`, HOME `/home/builder`). It passes `--userns=keep-id:uid=1000,gid=1000` and mounts caches at `/home/builder/{go,.npm,.gradle}`; `scripts/tests/test_container_hygiene.sh` (rule R7) fails if that is lost.

## Known limits

* `docker/Dockerfile.android4` (emulator image, uid 1000): `/dev/kvm` access for a non-root user under rootless podman likely needs `--group-add keep-groups`. UNCONFIRMED, not run here.
