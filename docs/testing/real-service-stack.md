# Real-service test stack (rootless podman)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T135), not yet committed; independent review owed (constitution 11.4.142, T136); linking this page from the main README is owed (11.4.212) |
| Source | `docker-compose.test-infra.yml`, `docker-compose.test-infra.nfs.yml`, `docker-compose.build.yml` (postgres, redis), `scripts/test-infra/`, `tests/infra/`; evidence `specs/001-full-project-audit-remediation/evidence/wp12/` and `wp10/` |

Constitution 11.4.27 allows mocks only in unit tests; every other test type runs against real services. This page describes the stack that provides them on a rootless host, what each
protocol proves, and what is NOT provided. Task references are the WP-13 tasks T127 to T135 of `specs/001-full-project-audit-remediation/tasks.md`.

## What runs

| Protocol | Server (digest-pinned, lock entry) | Client (inside IMG-INFRA-CLIENT) | State |
|---|---|---|---|
| PostgreSQL 15 | `IMG-INFRA-POSTGRES` | `psql` | real round trip x3 |
| Redis 7 (password) | `IMG-INFRA-REDIS` | `redis-cli` | real round trip x3 |
| FTP (pure-ftpd), passive mode | `IMG-INFRA-FTP` | `lftp` | real round trip x3 (passive transfer on the compose network) |
| SMB (Samba, SMB3, authenticated) | `IMG-INFRA-SMB` | `smbclient` | real round trip x3 |
| WebDAV (Apache, basic auth) | `IMG-INFRA-WEBDAV` | `curl` | real round trip x3 |
| NFS (user-space NFSv3: unfs3 + rpcbind) | local build `localhost/catalogizer-infra-nfs` (`scripts/test-infra/nfs/`) | libnfs `nfs-ls`, `nfs-cp`, `nfs-cat` | see "NFS" below |
| MinIO / S3 | none | `mc` (in the client image) | **BLOCKED**: no registry serves a MinIO image (`evidence/wp12/minio-blocked.txt`) |

Every client runs in the interpreter-class image IMG-INFRA-CLIENT through `scripts/containers/run_pinned.sh` attached to the project's compose network: never a client on the host, never an `exec` into a
service container, never a probe that only checks an open port (the tests include a listener that accepts TCP and says nothing, and a wrong-credential negative control per protocol).

## Lifecycle

```bash
scripts/test-infra/up.sh   --build-id <id> [--services postgres,redis,ftp,smb,webdav,nfs]
scripts/test-infra/probe.sh --build-id <id>
scripts/test-infra/roundtrip.sh --build-id <id> --protocol ftp [--record <dir> --iteration <n>]
scripts/test-infra/down.sh --build-id <id>
```

One compose project per run, `catalogizer-test-<id>`. Credentials are generated per run (`gen_env.sh`) into a mode 0600 file under the gitignored `.audit/test-infra/<project>/`; host ports are random
loopback ports; every compose file reads `${TI_*}` variables only (no literal credential, no literal host port). The corpus is deterministic (`seed_corpus.sh`, two runs byte-identical, recorded seed).
The lease is per compose project (`scripts/longops`): a second owner of the same project is refused, a different project is admitted at once; `down.sh` removes only resources labelled with the project.
Two stacks at once do not collide (`tests/infra/test_concurrency.sh`, `evidence/wp12/concurrency.json`).

## NFS (T134, docs/16 DR-16-2, finding D-10)

The unprivileged attempt: a user-space NFSv3 server (unfs3) in a rootless container, no kernel NFS server, no kernel mount, no added capability, and the libnfs user-space client. State: see
`specs/001-full-project-audit-remediation/evidence/wp10/nfs-attempt.json` (one terminal state, chosen by `scripts/test-infra/nfs_terminal_state.sh`, which reads the client verdict
`wp11/nfs-client.json` first) and `wp10/nfs-attempt-servers.md` (the servers tried and why each was kept or dropped).

What a `pass` proves: an NFSv3 round trip between a user-space server and the libnfs user-space client, rootless. What it does NOT prove: the application's own NFS path. `submodules/filesystem/pkg/nfs`
and `catalog-api/filesystem/nfs_client.go` mount through the KERNEL (`syscall.Mount`, `evidence/wp12/nfs-codepath.md`), which needs a privilege a rootless container does not have; that path stays
UNCONFIRMED on this host. The real-host leg (T134a) is `not_needed` only when the T134 state is `pass` and the NFS fallback record says so; the owner's seven Synology hosts have NFS closed from this host
(`docs/infrastructure/synology-hosts.md`).

## Real NAS leg (read-only, separate)

`scripts/test-infra/nas_readonly_leg.sh` lists one DATA share per host (depth 1) and reads at most one small file, at most 2 requests per second, from IMG-INFRA-CLIENT; it never writes. Credentials come only
from the gitignored `.env` (`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_<n>`); the leg SKIPs (not fails) when the file is absent. It is not part of the deterministic suite.

## Legs that need an external credential or device (T135)

`scripts/test-infra/blocked_external.sh` records, per leg, `available` or `blocked-unavailable` with the reason and the variable NAMES (never values), BLOCKED-ON ODG-01 for credentials: `nas_smb_readonly`
(`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_3`, `SYNOLOGY_IP_4`), `minio` (`image_unavailable`), `nfs_owner_host` (ODG-08 unanswered). A blocked leg is never a pass. Record:
`evidence/wp12/blocked-external.json`.

## Known limits (UNCONFIRMED unless stated)

- FTP passive mode is proven on the compose network (the clients run on it). Passive mode through a published host port under pasta is UNCONFIRMED: a published passive range cannot use random host ports.
- The compose file adds one capability, `AUDIT_WRITE`, to the FTP service: pure-ftpd exits 252 ("Unable to switch capabilities") without it under this host's podman defaults (measured by bisection).
- `catalog-api/tests/infra_helper.go` still hard-codes the old fixed ports and credentials, and `scripts/container-build.sh` validates `docker-compose.build.yml` without the per-run env file: both are consumers
  of the reworked files and need their own change (owed, not part of T129).
- TIC (`scripts/test-in-container.sh`) is refused (`anti_mess_drift`) while any `run_pinned` container of any stream is running, so the test-infra scripts retry it up to 90 times, 5 s apart. UNCONFIRMED root cause:
  `scripts/anti-mess/sweep.sh` reads the container label `op_id`, `run_pinned.sh` sets `catalogizer.op_id`.
- Tests must run one at a time on a shared host (see above).
