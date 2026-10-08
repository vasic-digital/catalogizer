# ftps_fixture.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08 |
| Status | committed (main repo `e9d6883d`, submodule `83c0ac1`); fix round 3 (answer to the WF24 re-review) changes the integration test only (a duration assertion per seek step), in the working tree, uncommitted; its independent re-review is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/ftp/` |
| Source | `scripts/test-infra/ftps_fixture.sh`, `docker-compose.test-infra.ftps.yml` |

## Purpose

Runs the integration tests of `submodules/filesystem/pkg/ftp` (`-tags integration`) against a **real pure-ftpd** that accepts clear-text FTP and explicit FTPS (`--tls=1`) on one server, rootless, with a certificate generated for the run.
It is the FTP/FTPS counterpart of `sftp_fixture.sh`.

## Usage

```
scripts/test-infra/ftps_fixture.sh run [--log FILE] [--keep] -- <go test arguments>     # e.g. -race -v ./pkg/ftp/
scripts/test-infra/ftps_fixture.sh selftest                                              # server presents the generated certificate; no other test
```

Exit code: the `go test` exit code; 1 = `ftps-fixture: REFUSED reason=<code>` or a failed sink-side check; 2 = usage.

## What a run does

1. Scratch directory `.audit/scratch/catalogizer-ftps-<id>/` (0700): random user and password, an RSA certificate and key (`pure-ftpd.pem`, 0600), a seeded tree (text file with a fixed mtime, a 1 MiB random binary, a nested directory, a UTF-8 named file).
   The SHA-256 of the certificate is computed with `openssl` outside the container: that is the owner's confirmation for the pin workflow.
2. Registers a long operation (`scripts/longops/register.sh`).
3. `podman-compose` starts `docker-compose.test-infra.ftps.yml` (pure-ftpd pinned by digest, the same digest as `IMG-INFRA-FTP`; `in_pod: false`; labels `project`, `op_id`, `catalogizer.test_project`; only `AUDIT_WRITE` added) and waits until the server answers `220` on its own control port.
4. Runs `go test -count=1 -tags integration <args>` in `IMG-GO` through `scripts/containers/run_pinned.sh`, attached to the project network; secrets in a 0600 env file, never argv.
   The disk head-room records of the run go to `$SD/disk` (removed with the scratch), never into the tracked `specs/.../evidence/disk/` (unless `DISK_HEADROOM_OUT_DIR` is set by the caller).
5. **Sink-side check**: a digest of the served tree (name, type, size, mtime, sha256) before and after the run must be equal: the read-only scan path changed nothing on the server.
6. `compose down -v`, leak check by label, release of the operation, removal of the scratch (from inside the user namespace, because pure-ftpd chowns the served directory to a sub-uid).

## Notes

* The user and password are passed to the server as environment variables of the container (the same way `docker-compose.test-infra.yml` does); both are random per run and the env file is 0600.
* `FTPS_FIXTURE_SRC=<dir>` replaces the checked-in module in the container view (used by the mutation harness).
* What the real-server leg asserts (besides the behaviours of round 1): a wrong password is exactly ONE login attempt (a counting credential resolver, one fetch = one attempt); an early-closed download leaves the control channel in step (three rounds of `GetFileInfo`/`FileExists` after it are answered correctly); `OpenSeekable` reads the whole 1 MiB file with the right `sha256` and random seeks equal `ReadFileFrom` at the same offsets; a cancelled context ends a transfer promptly and the next operation recovers; `Disconnect` closes and a reconnect works.
* Round 3 (WF24 G5): run ALONE, `TestIntegration_Seekable_WholeFileAndSeeks_NoTruncation` took 65 s on the committed client in 4 of 4 isolated iterations and failed at its 60 s context: a download that was closed after a few bytes with a non-zero `REST` was not answered by this server within 30 s. The client now waits at most 3 s for that reply; the test asserts that no seek step takes 15 s or more, so the stall can no longer pass silently. Trace: `raw/fix-r3-g5-forensics.txt` in the evidence directory. Why the server does not answer is UNCONFIRMED.
* Measured against this server (pure-ftpd, image of `IMG-INFRA-FTP`): it advertises `UTF8` in `FEAT` and answers `OPTS UTF8 ON` with `504`; a download closed early is answered with ONE reply (`150 <statistics>` over TLS, `226` in clear text), so the control connection is kept. Not asserted on the real server: a per-worker control-connection count (the server does not expose one to the test) and a permission `550` (the launcher's digest of the served tree needs every file readable by the launcher user).
* Not covered: the NAS. Nothing here contacts any NAS.
