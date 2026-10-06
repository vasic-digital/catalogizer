# IMG-INFRA-POSTGRES

Test-infrastructure PostgreSQL 15 (alpine). Service class: started only by the stack scripts, never given a build or test command.

| Item | Value |
|---|---|
| Lock id | `IMG-INFRA-POSTGRES` in `build/containers/images.lock.yaml` |
| Class | `service` |
| Reference | `docker.io/library/postgres` |
| Digest (index) | `sha256:f7d23353e1b15400d22ebe31189f4d314b87a4c129cc400c8c2d8d4ca127bf81` |
| Platform digest | `sha256:25d430274d8a31184f9435cc5b2f56aff254952065bbbcac0c51acedb5a1d1e7` |
| Resolved | 2026-10-06T17:28:09Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/library/postgres:15-alpine`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-INFRA-POSTGRES -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
