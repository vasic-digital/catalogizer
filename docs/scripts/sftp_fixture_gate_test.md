# sftp_fixture_gate_test.sh (test of the SFTP fixture script's own gates) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T23:50:00Z |
| Status | New in the working tree (not committed); the independent review (constitution 11.4.142 / 11.4.209) of fix-r2 is owed; evidence: `specs/001-full-project-audit-remediation/evidence/wp12/sftp/fix-r2-gate-test.txt`, `fix-r2-gate-test-mutants.txt` |
| Source | `scripts/test-infra/sftp_fixture_gate_test.sh` |
| Subject | `scripts/test-infra/sftp_fixture.sh` (guide: `docs/scripts/sftp_fixture.md`) |

## Purpose

Tests, WITHOUT starting any container, the two properties `sftp_fixture.sh` gained in fix-r2 (review finding SFTP-19): an undeclared `SFTP_FIXTURE_SRC` is refused, and the sha256 of the tree the
container will test is printed (and logged) correctly.

## Usage

```bash
bash scripts/test-infra/sftp_fixture_gate_test.sh        # exit 0 = every check passed; each check prints `ok   <name>` or `FAIL <name> (<detail>)`
SFTP_FIXTURE_UNDER_TEST=/path/to/mutant.sh bash scripts/test-infra/sftp_fixture_gate_test.sh   # run the same checks against another copy of the script (mutation proof)
```

It uses the script's own test hook `SFTP_FIXTURE_STOP_AFTER_HASH=1` (exit 0 right after the hash line, before registering an operation or starting a container) and a temporary source directory
under `$TMPDIR`, removed on exit. `SFTP_FIXTURE_UNDER_TEST` must live in `scripts/test-infra/` (the script derives the repository root from its own location).

## Checks (8)

1. `SFTP_FIXTURE_SRC` without `SFTP_FIXTURE_ALLOW_SRC=1` is refused with `REFUSED reason=src_override_not_declared`.
2. With `ALLOW=1` that gate passes (the next refusal is `src_module_missing` for a missing directory): the refusal in 1 is not a blanket refusal.
3. `ALLOW` must be exactly `1` (`yes` does not declare).
4. The printed whole-tree hash equals an independent computation over the source directory.
5. The printed `pkg/sftp` hash equals an independent computation over `pkg/sftp` of the source directory.
6. One changed byte changes the printed hash (and it still equals the independent one).
7. For the checked-in module (no override) the printed `pkg/sftp` hash equals the documented recomputation command. (The whole-module hash is printed as `info:` only: other work edits other packages of the module concurrently.)
8. No scratch directory (`.audit/scratch/catalogizer-sftp-*`) is left behind.

## Exits

0 all checks passed; 1 at least one failed. Mutation proof (`fix-r2-gate-test-mutants.txt`): with the gate disabled, with the whole-tree hash computed over a different file set, and with the package hash computed over the wrong directory, the
test fails (2 failed checks each).

## Limits

* It cannot prove the hash is of the bytes the container ran; it proves the printed hash is the hash of the copy the script made, which is what the container mounts.
* The disabled-gate mutant proceeds into the real fixture (the gate is what stops it): that mutant run starts and tears down the sshd container, then fails for want of a `go.mod`.
* Needs the tools `sftp_fixture.sh` itself needs (`podman`, `podman-compose`, `ssh-keygen`, `openssl`, `python3`, `tar`) because the script checks them before the gate.
