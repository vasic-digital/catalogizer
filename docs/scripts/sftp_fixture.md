# sftp_fixture.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T23:30:00Z |
| Status | New in the working tree (not committed). Round 1 independent review: NO-GO; fix round `fix-r2` (this revision: the tested-tree hash and the declared `SFTP_FIXTURE_SRC`); the re-review is OWED; evidence: `specs/001-full-project-audit-remediation/evidence/wp12/sftp/` |
| Source | `scripts/test-infra/sftp_fixture.sh`, `docker-compose.test-infra.sftp.yml` |
| Client under test | `submodules/filesystem/pkg/sftp` (`docs/testing/sftp-client.md`) |

## Purpose

Runs the integration tests of the SFTP client against a REAL OpenSSH server (image `docker.io/atmoz/sftp`, `internal-sftp` with `ChrootDirectory`) in a rootless
container, with a data volume mounted READ-ONLY and one test user, end to end and self-cleaning. It is separate from `up.sh` / `down.sh` (which own the
Postgres/Redis/FTP/SMB/WebDAV stack and are not modified by this fixture).

## Usage

```bash
scripts/test-infra/sftp_fixture.sh run [--log FILE] [--keep] -- <go test arguments>
scripts/test-infra/sftp_fixture.sh selftest
```

`run` appends its arguments to `go test -count=1 -tags integration`, executed in `/src/filesystem` of the pinned `IMG-GO` container
(through `scripts/containers/run_pinned.sh`, attached to the project network). `selftest` starts the server, runs only `TestIntegrationSelftest` (the server presents the host
key whose fingerprint was computed outside the container; the client pins it and connects) and stops.
Environment: `SFTP_FIXTURE_SRC=<dir>` replaces the checked-in module in the container view (used by the mutation harness; the checkout is never modified). It is
honoured ONLY together with `SFTP_FIXTURE_ALLOW_SRC=1`; without it the script refuses (`REFUSED reason=src_override_not_declared`), because a stray exported `SFTP_FIXTURE_SRC` would test other
bytes than the checkout while the headers of the run describe the checkout (review finding SFTP-19).

Every run prints `sftp-fixture: tested tree sha256=<hash> pkg/sftp sha256=<hash> source=<dir>` on stderr and, with `--log FILE`, writes `# tested-tree-sha256:`, `# tested-pkg-sftp-sha256:` and
`# tested-tree-source:` as the first three lines of the log. The hashes are of the tree the container actually tested (the view copy, `.git` excluded). They can be recomputed from a checkout with
`cd submodules/filesystem && find . -path ./.git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum` (whole module) and
`cd submodules/filesystem/pkg/sftp && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum` (the package under test). A GREEN log whose hash differs from that command's output tested other bytes.
The whole-module hash also covers packages that other work edits while this one is tested (`pkg/ftp`, `pkg/factory`), so it can differ between two runs of the same SFTP sources; the `pkg/sftp` hash is the stable one.

## What one run does

1. Creates `.audit/scratch/catalogizer-sftp-<id>/` (mode 0700): a random password, an ed25519 and an rsa host key, an ed25519 client key, `users.conf` (0600), the seeded data
   tree (a text file with the fixed mtime 2020-05-17 10:30:00 UTC, a 1 MiB random binary with its sha256, a nested directory, a symlink `escape -> ..` that leaves the client's
   root `/data` but stays inside the chroot, a symlink that stays inside) and a copy of the module. The container sees this scratch VIEW, not the checkout (the real `.env` files are not mounted).
2. Registers a long operation (`scripts/longops/register.sh`, owner pid = the script, no-progress budget 900 s) and labels the containers `project=catalogizer`, `op_id`, `catalogizer.op_id`,
   `catalogizer.test_project`, so the anti-mess sweep matches them to a registered op.
3. `podman-compose up -d` of `docker-compose.test-infra.sftp.yml` (image pinned by index digest, entry `IMG-INFRA-SFTP` of `build/containers/images.lock.yaml`; no pod; no published host port;
   user list, host keys and the client public key are mounted `:ro`; the data volume is `:ro`).
4. Waits for `Executing sshd` in the container log, then runs the tests in the Go image on `<project>_test-network` with the credentials in a mode 0600 env file (never argv). The host key
   fingerprint the tests pin comes from `ssh-keygen -lf` on the public key file: it is the "owner confirmation" of the pin workflow.
5. On ANY exit (success, failure, INT/TERM/HUP): `podman-compose down -v`, a check that nothing labelled for the project and no network of it remains (a leak makes the exit non-zero), release of
   the operation, removal of the scratch directory (only when it is exactly this run's directory under `.audit/scratch`).

## Test of the script's own gates

`scripts/test-infra/sftp_fixture_gate_test.sh` starts no container. It checks that `SFTP_FIXTURE_SRC` without `SFTP_FIXTURE_ALLOW_SRC=1` is refused (`src_override_not_declared`), that `ALLOW`
must be exactly `1`, that the printed tested-tree hashes (whole tree and `pkg/sftp`) equal an independent computation over the source directory and change when one byte changes, that the checked-in module's `pkg/sftp` hash
equals the documented command, and that no scratch directory is left behind. It uses the script's test hook `SFTP_FIXTURE_STOP_AFTER_HASH=1` (honoured in `selftest` mode ONLY: it ends the script right after the hash line, before any
container, with the distinct exit code 3; in `run` mode it is REFUSED with `stop_hook_only_in_selftest` before anything is created, so an inherited variable can never turn a test run into a silent exit 0 -
review WF24 S06; companion guide `docs/scripts/sftp_fixture_gate_test.md`). Exit 0 = all checks passed. The complete list of environment variables the script reads: `SFTP_FIXTURE_SRC`, `SFTP_FIXTURE_ALLOW_SRC`, `SFTP_FIXTURE_STOP_AFTER_HASH`.

## Exits and refusals

The exit status is the `go test` status. `sftp-fixture: REFUSED reason=<code>` (exit 1): `tool_missing`, `compose_missing`, `scratch_uncreatable`, `keygen_failed`, `fingerprint_unreadable`,
`view_copy_failed`, `src_override_not_declared`, `src_module_missing`, `tree_hash_failed`, `register_failed`, `compose_up_failed`, `sshd_not_ready`, `network_absent`, `run_pinned_failed`, `disk_headroom`, `stop_hook_only_in_selftest`. Exit 2: usage. Exit 3: `selftest` stopped by the test hook (never a result of a test run).

## Limits

* One run at a time per checkout is assumed (the operation purpose is unique per run, but the host's memory/thread budget is not arbitrated by this script beyond `run_pinned.sh`'s limits).
* The image `atmoz/sftp` is a community image (`signature_status: UNKNOWN` in the lock); it is used only as a test peer.
* OpenSSH inside the image: `OpenSSH_8.4p1 Debian-5+deb11u3` (`ssh -V` in the pinned image); an old release, adequate as a peer, not a statement about Synology.
