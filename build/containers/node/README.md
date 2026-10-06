# IMG-NODE

Web build image (Node 20, Debian bookworm) for catalog-web, the TypeScript modules and catalogizer-api-client. Node 20 matches catalog-web/Dockerfile.

| Item | Value |
|---|---|
| Lock id | `IMG-NODE` in `build/containers/images.lock.yaml` |
| Class | `compile` |
| Reference | `docker.io/library/node` |
| Digest (index) | `sha256:8f693eaa7e0a8e71560c9a82b55fd54c2ae920a2ba5d2cde28bac7d1c01c9ba5` |
| Platform digest | `sha256:cacf10e99285cbbc891452e31249c1b5ec3ba225f40028fae946b75aeaf1b66a` |
| Resolved | 2026-10-06T17:27:13Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/library/node:20-bookworm`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-NODE -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
