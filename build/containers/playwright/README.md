# IMG-PW

Playwright image for web E2E and visual regression. Tag v1.57.0-noble is aligned with catalog-web/package.json ("@playwright/test": "^1.57.0"); the compose files' v1.40.0-jammy is not used (V-06).

| Item | Value |
|---|---|
| Lock id | `IMG-PW` in `build/containers/images.lock.yaml` |
| Class | `compile` |
| Reference | `mcr.microsoft.com/playwright` |
| Digest (index) | `sha256:3bed4b1a12f2338642f3d8cba28e291deef3c66bd4a964bbeb3e57bbff511dbd` |
| Platform digest | `sha256:8fb7af3bb488c51364d6554876a8eddf377736608327dbdf4177b4901faf7bc9` |
| Resolved | 2026-10-06T17:27:55Z on anton |
| Signature status | `UNKNOWN` (not verified by a publisher signature; T149 verifies where the publisher signs) |

Pulled, not built: `podman pull mcr.microsoft.com/playwright:v1.57.0-noble`, then `scripts/containers/resolve_pin.sh` wrote the entry. Run only through `scripts/containers/run_pinned.sh IMG-PW -- <command>`.
Checks: `scripts/containers/tests/test_containerfiles.sh` (digest-pinned `FROM`, no pipe-to-shell, `digests.lock`, lock entry and class).
