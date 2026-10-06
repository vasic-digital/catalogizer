# build/containers

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Task | T116 (WP-11) |

Catalogue of the container image definitions and of the single lock file `images.lock.yaml`, with the pinning and bump procedure. Status: this file was written before any review; every statement below was read from the repository files named in it on 2026-10-06.

## Catalogue

Each sub-directory holds one `Containerfile` (all `FROM` lines digest-pinned), a `README.md` and a `digests.lock`:

| Directory | Lock id | Purpose |
|---|---|---|
| `go`, `node`, `playwright`, `rust`, `android`, `docs`, `kcov`, `mut`, `testutil`, `gotools`, `sigverify` | `IMG-GO`, `IMG-NODE`, `IMG-PW`, `IMG-RUST`, `IMG-ANDROID`, `IMG-DOCS`, `IMG-KCOV`, `IMG-MUT`, `IMG-TESTUTIL`, `IMG-GOTOOLS`, `IMG-SIGVERIFY` | language and tool images used by `scripts/containers/run_pinned.sh` |
| `infra-postgres`, `infra-redis`, `infra-ftp`, `infra-smb`, `infra-webdav`, `infra-client` | `IMG-INFRA-*` | the test infrastructure services and their client image |

Third-party images that only compose files or scripts reference (no Containerfile here) are lock entries too: `IMG-SCAN-*` (scanners), `IMG-UBUNTU-BASE` (24.04) and `IMG-UBUNTU-JAMMY` (22.04, base of `docker/Dockerfile.builder`), `IMG-INFRA-PGADMIN` and `IMG-INFRA-REDISCOMMANDER` (`docker-compose.dev.yml`).

Not in the lock, on purpose and honestly: MinIO. `quay.io/minio/minio:latest` (`docker-compose.dev.yml`, `deploy/infra-compose.yml`, `deploy/infra-compose-test.yml`) answers `unauthorized` to a manifest read and the Docker Hub tag-list request for `minio/minio` returned no `tags` field (evidence `specs/001-full-project-audit-remediation/evidence/wp11/t114-minio-blocked.txt`); the three rows stay in the `check_pins` baseline until the owner names a source for the image.

## Lock file

`images.lock.yaml` (schema 1, docs/16 section 7.1): one entry per image with `reference`, `tag_intent`, `digest` (the INDEX digest the tag pointed at), `platform_digest`, `resolved_at`, `resolved_on`, `purpose`, `signature_status` (`VERIFIED` only after `cosign verify` in `IMG-SIGVERIFY`, else `UNKNOWN`), `size_bytes` and `class`. Digests are written by `scripts/containers/resolve_pin.sh` from `podman image inspect` and `podman manifest inspect`, never typed.

## Pinning an image

1. Read the digest of the wanted tag from the registry without pulling (`podman manifest inspect`, or the registry `Docker-Content-Digest` header of a HEAD on the manifest with the index media types).
2. `scripts/containers/disk_headroom.sh --need <bytes>`, then `podman pull <reference>@<digest>` (by digest only).
3. `scripts/containers/resolve_pin.sh --id IMG-X --reference <ref> --tag-intent <tag> --purpose "<why>" --local-ref <ref>@<digest>`, then add the `class:` line.
4. Reference the image as `<ref>:<tag>@sha256:<digest>` in compose files and `FROM` lines.
5. `scripts/containers/run_pinned.sh IMG-KCOV -- bash scripts/containers/check_pins.sh <files>` must report 0 violations for the files touched.

## Bumping an image

Change the tag intent, repeat the pinning steps, replace the digest in every consumer (`grep -rn <old digest>`), rebuild the images that are `FROM` it, re-run `check_pins.sh` over the whole tree, and record the old and new digest in the commit message.

## Build contexts (D-01..D-03)

`catalog-api/Dockerfile` and `catalog-web/Dockerfile` copy `submodules/...` and `catalog-*/...` paths: both must be built with the repository root as context (`docker-compose.yml`, `docker-compose.test.yml` do). The repository has no root `.dockerignore`; a root-context build without one sends every file of the tree (about 4.4 GB without `.git`) and a module checkout that holds `node_modules` would overwrite the image's. Owed: add one.
