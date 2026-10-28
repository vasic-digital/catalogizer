# scripts/build_in_container.sh

| Field | Value |
|---|---|
| Script | `scripts/build_in_container.sh` |
| Revision | 1 (2026-10-28) |
| Status | active |
| Related | `scripts/container-local.sh` ([container_local.md](container_local.md)), `docs/scripts/container_build.md`, CLAUDE.md section 11.4.173 |

## Purpose

Builds `catalog-api` (Go) inside a container (CLAUDE.md section 11.4.173: no bare-host builds).

## Modes

| `BUILD_HOST` | Behaviour | Output |
|---|---|---|
| unset (default) | `exec scripts/container-local.sh api-build` on THIS host, rootless podman. No copy, no md5 compare. | `deploy/artifacts/catalog-api` |
| set to a host name | rsync the source to that host, build in a podman container there, scp the binary back, compare md5. Exit 1 on md5 mismatch. | `deploy/artifacts/catalog-api-container` and a proof block (remote md5, local md5, `md5_match: YES/NO`, file type) |

Other inputs (remote mode): `BUILD_USER` (default `milosvasic`), `REMOTE_BUILD_DIR` (default `catalogizer-build`), `GO_BUILD_IMAGE` (default `docker.io/library/golang:1.25-bookworm`).

## thinker.local

`thinker.local` is no longer the default build host; it is only used if a caller names it in `BUILD_HOST`.

Operator statement, 2026-10-28, relayed by the conductor (not verbatim, not given in writing): "thinker.local is obsolete; build only on the local host."
This contradicts CLAUDE.md section 11.4.173, which requires builds to be distributed to a remote build host (for example `thinker.local`) and the artifacts brought back.
It is therefore FLAGGED: the operator is asked to confirm it in writing, and CLAUDE.md / the constitution are intentionally not changed by this series.

## Exit status

0 on success; non-zero on any failed step or md5 mismatch (remote mode).
