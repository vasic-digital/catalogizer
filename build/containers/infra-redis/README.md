# IMG-INFRA-REDIS

Test-infrastructure Redis 7 (alpine). Service class.

| Item | Value |
|---|---|
| Lock id | `IMG-INFRA-REDIS` in `build/containers/images.lock.yaml` |
| Class | `service` |
| Reference | `docker.io/library/redis` |
| Digest (index) | `sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499` |
| Platform digest | `sha256:ca0acbb137c1dc3339c8b147a58fd6f42775d4599327b50e7b116c23de501af2` |
| Resolved | 2026-10-06T17:28:16Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/library/redis:7-alpine`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-INFRA-REDIS -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
