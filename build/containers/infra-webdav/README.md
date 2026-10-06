# IMG-INFRA-WEBDAV

Test-infrastructure WebDAV server (bytemark/webdav). Service class. The image was last updated 2018-12-14 (Docker Hub), maintenance UNKNOWN (docs/16 section 10.2).

| Item | Value |
|---|---|
| Lock id | `IMG-INFRA-WEBDAV` in `build/containers/images.lock.yaml` |
| Class | `service` |
| Reference | `docker.io/bytemark/webdav` |
| Digest (index) | `sha256:bcabbc024c511b9c63ed3345f88573e31d84c952ee493c9acb3fe345f4f80f57` |
| Platform digest | `sha256:bcabbc024c511b9c63ed3345f88573e31d84c952ee493c9acb3fe345f4f80f57` |
| Resolved | 2026-10-06T17:29:00Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/bytemark/webdav:2.4`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-INFRA-WEBDAV -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
