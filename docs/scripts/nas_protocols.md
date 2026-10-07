# nas_protocols.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T17:25:20Z |
| Status | new in the working tree (not committed); the independent review is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/nas-protocols/` |
| Source | `scripts/test-infra/nas_protocols.sh`, `scripts/test-infra/client/nas_proto.py`, `scripts/test-infra/client/nas_proto_nfs.sh`; tests `tests/infra/test_nas_protocols.sh`, `tests/infra/nas_proto_unit.py` |

## Purpose

READ-ONLY survey of the owner's seven Synology hosts (`docs/infrastructure/synology-hosts.md`) over NFS, FTP, explicit FTPS and SFTP (SMB is covered by `nas_readonly_leg.md` and the earlier survey), to prove reachability and
read access per host and protocol and to measure listing latency and read throughput so that the best protocol per host can be chosen. Nothing is written, created, deleted, renamed or changed on a NAS.

## What is measured

- **NFS** (`nas_proto.py rpc`, then `nas_proto_nfs.sh` where a path is mountable): portmap registrations, NFS NULL calls for v2/v3/v4 on 2049 (a v4 PROG_MISMATCH records the supported range), the MOUNT EXPORT list,
  and MNT attempts on `/volume1`, `/volume2`, `/volumeN/<share>` plus one control path that cannot exist (when the control answers the same code, the code does not discriminate). Where a path mounts, a bounded
  breadth-first walk through the libnfs user-space client (v3, and v4 when offered) with listing latency and a 1 MiB read sample.
- **FTP / FTPS**: banner, FEAT, login result, `OPTS UTF8 ON` (a session option; without it these servers answer 0x7f for every non-ASCII name), MLSD availability, TLS version negotiated, TLS versions accepted (1.2, 1.3),
  cipher, certificate sha256 and subject/issuer sha256, self-signed flag, validity dates, bounded walk, one read sample per share.
- **SFTP**: host key fingerprints (ssh-keyscan, public), server software, kex and cipher, auth method, the `sftp` subsystem, SFTP v3 listing and one read sample per share; permission bits of the sampled entries (histograms) support
  the read-only inference without any write attempt (the account's own uid is not observable, so the permission bits are indicative, not proof).

Per share: at most 30 listing requests, depth 3, 2000 entries (checked before each request; a single large directory can exceed it), breadth-first, `@*`/`#*` system directories counted and never descended. The read sample is the first
1 MiB (`READ` requests / `head -c`) of ONE file (the smallest file of at least 1 MiB in the sample, else the largest smaller file): bytes are counted and discarded, never stored. `mib_per_s_total` includes the time to the first byte;
`mib_per_s_after_first_byte` excludes it. One 1 MiB sample per share is a single observation, not a benchmark.

## Rules

Read-only by construction (the test scans the clients for write-class verbs, with control needles); at most one request per second per host (a `Pacer`; hosts are measured in parallel, each host sequentially); entry names, file content
and host addresses are never recorded (aliases, share names, counts, sha256 of names, extension histograms only); credentials only from the gitignored env file via a 0600 file in a 0700 run directory on `$XDG_RUNTIME_DIR`
(read by the client, removed on every exit path; the SSH password reaches ssh through `SSH_ASKPASS` reading that file, never argv or the environment). Before the evidence is written it is scanned for the user name, the password
and every host address; the run fails and the files are removed if one occurs. Clients run in rootless containers through `scripts/containers/run_pinned.sh`: `IMG-GO` (python3 stdlib, ssh, openssl) and
`IMG-INFRA-CLIENT` (libnfs-utils); no kernel mount, no sudo.

## Usage

```bash
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
scripts/test-infra/nas_protocols.sh [--hosts 1,2,...] [--protocols nfs,ftp,ftps,sftp] [--ev-dir DIR] [--jobs N]
```

A full run takes about 10-15 minutes (run it in the background and poll). Without the env file: `SKIP nas_protocols reason=env_absent` (exit 0); with missing variables: `reason=credentials_absent variables: <NAMES>`.
A refused or unreachable protocol is a recorded result (exit 0), not a failure. Output: `nfs-<n>.json`, `ftp-<n>.json`, `ftps-<n>.json`, `sftp-<n>.json` (each with an `identity` header: head, run time, tool sha256),
`summary.json` (per host and protocol: works/status, latency p50/p95, median read MiB/s, best protocol by read throughput) and `SHA256SUMS` (verifies on a clean checkout: `cd <dir> && sha256sum -c SHA256SUMS`).

## Tests

`tests/infra/test_nas_protocols.sh` (A static scans with control needles, B the python unit oracle `tests/infra/nas_proto_unit.py` with a real TCP ONC-RPC fixture, C SKIP and usage behaviour, D evidence integrity and leak scan).
