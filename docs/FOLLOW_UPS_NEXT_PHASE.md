# Follow-ups for the next phase(s)

Small items left after independent review rounds, documented per the operator rule of
2026-10-28: mid-size or bigger findings get 5 more fix/review rounds; only small ones
land and are listed here. Layers follow constitution §11.4.235(D).
Status values: OPEN until a commit on `main` closes the item.

## Container hygiene guard (`scripts/tests/test_container_hygiene.sh`) — test-instrumentation
Found by the round-3 reviewer (landed series `a9e5248b`); the guard fails open on these shapes:
- O1 (R7): compose long-form bind volume (`type: bind`, `source:`, `target:`) bypasses the
  non-root/keep-id rule; only the short form is caught.
- O2 (R9): combined short flags such as `curl -fsI .../health` bypass the HEAD-healthcheck rule.
- O3 (R3): `RUN yarn build || true` is not caught; enumerate the build-command class
  (npm/yarn/pnpm build, go build, make, gradle, tsc), not just `npm run build`.
- O4 (R5): flow-style `privileged: true` (`p: {image: x, privileged: true}`) is not caught;
  handle flow mappings or fail closed on syntax the parser cannot model.
Each needs a RED selftest first, then the fix.

## Operator decisions pending (process-doc)
- `deployment/amber-up.sh` runs plain `docker` (allowlisted R6): keep or migrate to podman.
- `scripts/deploy-vision-hosts.sh` default host `thinker.local` (allowlisted R4): the operator
  said it is obsolete, but constitution §11.4.173 still names it as the build host; needs a
  recorded decision.
- Image `catalogizer/server` (`deployment/docker-compose.yml:109`, curl healthcheck): no
  Dockerfile in the repo builds it; whether curl exists in it is UNCONFIRMED.
- Android `/dev/kvm` as uid 1000 (F10) and the real-image F1 run: UNCONFIRMED.

## Secrets scanner (`scripts/` secret scan) — test-instrumentation (security series round 5)
- TI-1 (fix soon): value-prefix exclusions (`/ [ { <`) hide real literals such as
  `JWT_SECRET: /k7Qz...`, `ADMIN_PASSWORD=[hunter2]`, `export DB_PASSWORD=/s3cr3t`. Treat `/` as a
  path only in the short-volume form; limit `<...>` to exact placeholders; add the six literals
  as must-flag fixtures and `VOL=api_keys:/app/keys` as must-not-flag.
- TI-2: generator scan skips every path containing `/tests/`; use an explicit list of test roots.
- TI-3: the stack-boot test greps only the literal `admin123`; broaden or rely on the scanner.
- PD-1: `run-full-qa.sh` registers `testuser_qa` with `ADMIN_PASSWORD`; use a separate value.
- Out of scope but real: `admin123` still appears in `catalog-api/challenges/*`, `start_server.sh`,
  `run_challenges*.sh`, `check_scans.sh`, `catalog-web/e2e/fixtures/auth.ts`, the Android and
  Android TV `MainActivity.kt` (production app source: own item), and the helixqa banks.

## SQLCipher encryption at rest
- Review of `60ee1acb` (GO, three LOW test-instrumentation findings, mutants that survive):
  RV5 `scrubDSN` bypass in the new `probeInUse` open-error branch (the driver error never carries the DSN, so W1 cannot fail);
  RV6 dropped `db.Close()` in that branch (needs a goroutine-count check);
  RV4 NOTADB split in the `cipher_version` branch (probably an equivalent mutant).
- `classifyProbeError` busy branch (`encryption.go:522`) formats `err` without `scrubDSN`; no live leak, add it for consistency.
- `docker-compose.dev.yml` keeps a literal `MINIO_ROOT_USER: minioadmin` (a username, not a password).
- Windows behaviour is validated only through emulated `hostOS` and `go vet`; no run on real Windows.
- Descriptor-count tests are Linux-only (`/proc/self/fd`).

## Path confinement
- F4: TOCTOU between check and use remains open; plan `os.Root`-based sinks
  (see `docs/security/PATH_CONFINEMENT.md` once landed).
- Consolidate `internal/pathguard` into `internal/pathsafe` after the security series lands.

## Live NAS tests
- Two blind sleeps remain in `catalog-api/filesystem/ftp_readcloser_unit_test.go` (around lines 171 and 201).
