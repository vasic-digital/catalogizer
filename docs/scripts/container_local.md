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

## Hygiene guard parser coverage (O1-O4 closure)

`scripts/tests/test_container_hygiene.sh` models these shapes (each has a RED selftest fixture and a golden-FALSE control):

- R7: compose volumes in short form, long form (`- type: bind` / `source:` / `target:` / `read_only:`, any key order) and flow form (`- {type: bind, source: ./x, target: /app}`); `type: volume|tmpfs` is not a bind.
- R9: HEAD probes of `/health` in flag clusters (`curl -fsI`, `-sSfI`), `-X HEAD`, `-XHEAD`, `--request HEAD`, `--spider`, `--head`, in Dockerfile HEALTHCHECK and compose `test:` (string, JSON list, block list).
- R3: a swallowed build failure for `npm|yarn|pnpm|bun [run] build|compile`, `go build|install`, `make`, `gradle(w)`, `tsc`, `cargo build`, `mvn(w)`, `vite|ng|next|nuxt|webpack build` (`|| x`, single pipe, `; true`, `; exit 0`, `; :`). `|| exit N` (N>0) and `|| false` are not treated as masking. Limit: a legitimate `|| (cat log; exit 1)` is still flagged.
- R5: `privileged` in block and flow-mapping services.
- R10-COMPOSE-UNMODELED (fail closed, allowlistable by file path): merge keys (`<<:`), YAML aliases as the `privileged` value, a one-line `services: {...}`, flow service bodies that carry `volumes`/`build`/`healthcheck`/aliases, and service entries the parser cannot recognise.

Honest gaps: other YAML features (anchors elsewhere, `extends:`, `include:`, env-substituted `privileged: ${X}`) are not resolved; multi-document and `profiles`-gated services are scanned as written.

## Findings recorded by the hygiene guard (2026-10-28)

* `docker-compose.dev.yml` api: bind-mounts `./catalog-api` rw into an image running as uid 1000; fixed with `userns_mode: keep-id` (podman-only). Guard rule R7 now covers any compose service.
* `docker-compose.test.yml` catalog-api: healthcheck used `wget --spider` (HEAD) against `/health`, which is GET-only; now a GET. Guard rule R9.
* `deployment/docker-compose.yml:109` `catalogizer/server` healthcheck uses `curl`. No Dockerfile in this repository builds `catalogizer/server` (the image is pulled by tag), so whether curl exists in it is UNCONFIRMED and the guard cannot check it. Tracked: verify against the published image or switch to a tool the image is known to carry.
* `deployment/amber-up.sh` runs plain `docker` by design (amber.local is a rootless-Docker host); allowlisted, conflicts with podman-only, operator decision tracked.
* `Build/lib/common.sh` `detect_runtime` no longer falls back to docker (the builder needs podman's `--userns=keep-id`); it fails loudly.
