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

## Hygiene guard parser coverage (O1-O4, after review round 3)

`scripts/tests/test_container_hygiene.sh` (compose reader: `scripts/tests/container_hygiene_compose.awk`, errexit /
pipefail reader: `scripts/tests/container_hygiene_shopts.awk`) models these shapes; each has a RED selftest fixture
and a golden-FALSE control. Both readers are line readers, not YAML or shell parsers. "Fail closed" below means
only that the listed shapes are reported as R10; a shape the reader does not recognise as unusual can still be read
wrongly. The gaps further down are known and not closed.

- R7: compose volumes in short form (also quoted), long form (any key order, quoted keys, unknown first key,
  `target`/`dst`/`destination`) and single-line flow form; `type: volume|tmpfs` is not a bind; a long-form
  `type: bind` source without `./` (`source: svc`) is read as a host path; a target containing `:` does not
  hide `read_only: true`.
- R9: HEAD probes of `/health` in flag clusters (`curl -fsI`, `-sSfI`, `-4I`), `-X HEAD`, `-XHEAD`,
  `--request HEAD`, `wget --method=HEAD`, `--spider`, `--head`, in Dockerfile HEALTHCHECK and compose `test:`
  (string, JSON list, block list, block scalar `>-`/`|`).
- R3: a swallowed build failure for `npm|yarn|pnpm|bun [opts] [run|run-script] build|compile` (script name may be
  quoted; opts incl. `--cwd`, `--prefix`, `--filter`, `workspace NAME`), `go build|install`, `make`, `gradle(w)`,
  `tsc`, `cargo build`, `mvn(w)`, `vite|ng|next|nuxt|webpack build`, optionally after `VAR=x` / `env` /
  `npx|bunx|pnpx` / `yarn|pnpm [exec]`, at the start of a RUN body (after `--flags`), after a separator, or opening a
  `sh -c "..."` string; the build word may be followed directly by `"`, `)` or `>`. Exec form
  (`RUN ["sh","-c","..."]`) is decoded to words first. Masks: `||`, a single pipe, any command after `;`, a
  background `&`. Exempt: `|| exit N` with N in 1..255 and `|| false` (the delimiter after them is kept, so
  `|| false; true` is flagged). errexit (`set -e`, `set -o errexit`, `sh -e`, `bash -e`) and pipefail
  (`set -o pipefail`, `bash -o pipefail`) count only at a command position (start of the RUN body or after
  `; & | (`) and apply only to the text after them: `npm run build; set -e`, `echo errexit; yarn build; true` and
  `echo nopipefail; npm run build | tee x` are flagged. After errexit `;` reads as `&&`; after pipefail a single
  pipe reads as `&&` (its `||` still masks). A stage `SHELL` with pipefail or `-e` applies to later RUNs of that
  stage. `RUN` is case-insensitive. Interpreter heredocs (`RUN <<EOF`, `RUN bash -e <<EOF`) are read as their body
  joined with `;` (a `#!... -e` shebang counts as errexit); a heredoc feeding another command (`cat <<EOF > f`) is
  not read as commands.
- R5: `privileged` in block and single-line flow-mapping services, also as a quoted key or with a space before `:`.
- R10-COMPOSE-UNMODELED (allowlistable by file path), reported for: a top-level line that is not a plain `key:` or a
  document marker (`services :`, `"services":`, a JSON document); merge keys; an alias, anchor, tag (`!!bool`),
  block scalar or flow collection as the value of `privileged`, `userns_mode`, `build`, `volumes` or `healthcheck`;
  an env-substituted `privileged`/`userns_mode` (`${X}`); a one-line `services: {...}`; multi-line flow service
  bodies; single-line flow bodies with `volumes`/`build`/`healthcheck`/anchors/aliases/tags; services not at 2
  spaces or bodies not at 4; any line at an odd indentation inside a service (1, 3, 5, 7, 9, ...); a line nested
  deeper than 6 spaces under a 6-space key that already has a value (a build/healthcheck/volumes body at 8); a
  volumes sequence at the key indentation; volumes given as a mapping; a block-scalar volume item; multi-line flow
  volume items; any service-level line that is not `key: value`. The content lines of a block scalar (a `key: |`
  value or a `- |` sequence item, as in `command:` lists) are skipped as text.

Honest gaps (known, not closed):
- Compose: anchors/aliases outside the modelled keys, `extends:`, `include:`, `profiles`, env substitution in
  anything but `privileged`/`userns_mode` (`build: ${CTX}`, `source: ${DIR}`), and multi-document files are not
  resolved; a long-form volume with no `type:` key is not treated as a bind; nested `- ` items deeper than a
  long-form volume are not checked; a `test:` outside a `healthcheck:` block is still read as a healthcheck test;
  keys below 6 spaces under a key with no value (`deploy:`, `logging:`) are skipped without being read.
- R3: errexit/pipefail are positional but not scoped: `set -e` before a nested `sh -c "..."` without `-e` still
  makes the nested `;` read as `&&`, and `set +e` / `set +o pipefail` are not modelled (errexit stays on); a `;`
  inside a quoted string after a build is read as a mask (false positive possible); a legitimate
  `|| (cat log; exit 1)` is flagged; `if build; then ...; fi`, functions, `eval`, `$(...)`-wrapped builds, builds in
  shell scripts copied into the image, and `ONBUILD RUN` are not modelled; exec-form arguments containing escaped
  quotes are not decoded; a heredoc whose command sits after the `<<EOF` marker on the same line is only read when
  the command before the marker is a shell.
- R9: only `/health` is checked, and only the HEAD forms above; a probe built from variables is not resolved.
- Equivalent mutants: dropping the `destination` key (no rule reads the mount target) does not change any verdict.

## Findings recorded by the hygiene guard (2026-10-28)

* `docker-compose.dev.yml` api: bind-mounts `./catalog-api` rw into an image running as uid 1000; fixed with `userns_mode: keep-id` (podman-only). Guard rule R7 now covers any compose service.
* `docker-compose.test.yml` catalog-api: healthcheck used `wget --spider` (HEAD) against `/health`, which is GET-only; now a GET. Guard rule R9.
* `deployment/docker-compose.yml:109` `catalogizer/server` healthcheck uses `curl`. No Dockerfile in this repository builds `catalogizer/server` (the image is pulled by tag), so whether curl exists in it is UNCONFIRMED and the guard cannot check it. Tracked: verify against the published image or switch to a tool the image is known to carry.
* `deployment/amber-up.sh` runs plain `docker` by design (amber.local is a rootless-Docker host); allowlisted, conflicts with podman-only, operator decision tracked.
* `Build/lib/common.sh` `detect_runtime` no longer falls back to docker (the builder needs podman's `--userns=keep-id`); it fails loudly.
