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

## Hygiene guard parser coverage (O1-O4, after review round 2)

`scripts/tests/test_container_hygiene.sh` (compose reader: `scripts/tests/container_hygiene_compose.awk`) models
these shapes; each has a RED selftest fixture and a golden-FALSE control. This is not a full YAML or shell parser;
the gaps below are known and not closed.

- R7: compose volumes in short form (also quoted), long form (any key order, quoted keys, unknown first key,
  `target`/`dst`/`destination`) and single-line flow form; `type: volume|tmpfs` is not a bind.
- R9: HEAD probes of `/health` in flag clusters (`curl -fsI`, `-sSfI`), `-X HEAD`, `-XHEAD`, `--request HEAD`,
  `wget --method=HEAD`, `--spider`, `--head`, in Dockerfile HEALTHCHECK and compose `test:` (string, JSON list, block
  list, block scalar `>-`/`|`).
- R3: a swallowed build failure for `npm|yarn|pnpm|bun [opts] [run] build|compile` (opts incl. `--cwd`, `--prefix`,
  `--filter`, `workspace NAME`), `go build|install`, `make`, `gradle(w)`, `tsc`, `cargo build`, `mvn(w)`,
  `vite|ng|next|nuxt|webpack build`, optionally after `VAR=x` / `env` / `npx|bunx|pnpx` / `yarn|pnpm [exec]`, at the
  start of a RUN body (after `--flags`), after a separator, or opening a `sh -c "..."` string. Masks: `||`, a single
  pipe, any command after `;`, a background `&`. Exempt: `|| exit N` with N in 1..255 and `|| false` (the delimiter
  after them is kept, so `|| false; true` is flagged). A line with `set -e`/`errexit`/`sh -e` reads `;` as `&&`; a
  line with `pipefail` reads a single pipe as `&&` (its `||` still masks). A stage `SHELL` with pipefail or `-e` applies
  to later RUNs of that stage. `RUN` is case-insensitive. Interpreter heredocs (`RUN <<EOF`, `RUN bash -e <<EOF`) are
  read as their body joined with `;` (a `#!... -e` shebang counts as errexit); a heredoc feeding another command
  (`cat <<EOF > f`) is not read as commands.
- R5: `privileged` in block and single-line flow-mapping services, also as a quoted key or with a space before `:`.
- R10-COMPOSE-UNMODELED (fail closed, allowlistable by file path): merge keys; an alias, anchor, tag (`!!bool`),
  block scalar or flow collection as the value of `privileged`, `userns_mode`, `build`, `volumes` or `healthcheck`;
  a one-line `services: {...}`; multi-line flow service bodies; single-line flow bodies with
  `volumes`/`build`/`healthcheck`/anchors/aliases/tags; services not at 2 spaces or bodies not at 4; lines at an odd
  indentation; a volumes sequence at the key indentation; multi-line flow volume items; build/healthcheck keys at
  5 spaces; any service-level line that is not `key: value`.

Honest gaps (known, not closed):
- Compose: anchors/aliases outside the modelled keys, `extends:`, `include:`, env-substituted values
  (`privileged: ${X}`), multi-document files and `profiles` are not resolved; nested `- ` items deeper than a long-form
  volume are not checked; a `test:` outside a `healthcheck:` block is still read as a healthcheck test.
- R3: `set -e` is applied to the whole RUN line (a nested `sh -c` without `-e` on the same line is treated as
  errexit); a `;` inside a quoted string after a build is read as a mask (false positive possible); a legitimate
  `|| (cat log; exit 1)` is flagged; `if build; then ...; fi`, functions, `eval`, `$(...)`-wrapped builds, builds in
  shell scripts copied into the image, and `ONBUILD RUN` are not modelled; a heredoc whose command sits after the
  `<<EOF` marker on the same line is only read when the command before the marker is a shell.
- R9: only `/health` is checked, and only the HEAD forms above; a probe built from variables is not resolved.

## Findings recorded by the hygiene guard (2026-10-28)

* `docker-compose.dev.yml` api: bind-mounts `./catalog-api` rw into an image running as uid 1000; fixed with `userns_mode: keep-id` (podman-only). Guard rule R7 now covers any compose service.
* `docker-compose.test.yml` catalog-api: healthcheck used `wget --spider` (HEAD) against `/health`, which is GET-only; now a GET. Guard rule R9.
* `deployment/docker-compose.yml:109` `catalogizer/server` healthcheck uses `curl`. No Dockerfile in this repository builds `catalogizer/server` (the image is pulled by tag), so whether curl exists in it is UNCONFIRMED and the guard cannot check it. Tracked: verify against the published image or switch to a tool the image is known to carry.
* `deployment/amber-up.sh` runs plain `docker` by design (amber.local is a rootless-Docker host); allowlisted, conflicts with podman-only, operator decision tracked.
* `Build/lib/common.sh` `detect_runtime` no longer falls back to docker (the builder needs podman's `--userns=keep-id`); it fails loudly.
