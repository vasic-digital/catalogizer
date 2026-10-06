# IMG-INFRA-FTP

Test-infrastructure FTP server (pure-ftpd). Service class. The image was last updated 2018-12-14 (Docker Hub), maintenance UNKNOWN (docs/16 section 10.2).

| Item | Value |
|---|---|
| Lock id | `IMG-INFRA-FTP` in `build/containers/images.lock.yaml` |
| Class | `service` |
| Reference | `docker.io/stilliard/pure-ftpd` |
| Digest (index) | `sha256:6435d108eb3fad29730f1b6748c1d49f3fa13e5d0fb49979ec3e0e96c5dbd94a` |
| Platform digest | `sha256:6435d108eb3fad29730f1b6748c1d49f3fa13e5d0fb49979ec3e0e96c5dbd94a` |
| Resolved | 2026-10-06T17:28:37Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/stilliard/pure-ftpd:latest`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-INFRA-FTP -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
