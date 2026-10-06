# IMG-INFRA-SMB

Test-infrastructure SMB server (dperson/samba). Service class. The image was last updated 2021-03-31 (Docker Hub), maintenance UNKNOWN (docs/16 section 10.2).

| Item | Value |
|---|---|
| Lock id | `IMG-INFRA-SMB` in `build/containers/images.lock.yaml` |
| Class | `service` |
| Reference | `docker.io/dperson/samba` |
| Digest (index) | `sha256:66088b78a19810dd1457a8f39340e95e663c728083efa5fe7dc0d40b2478e869` |
| Platform digest | `sha256:e1d2a7366690749a7be06f72bdbf6a5a7d15726fc84e4e4f41e967214516edfd` |
| Resolved | 2026-10-06T17:28:53Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull docker.io/dperson/samba:latest`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-INFRA-SMB -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
