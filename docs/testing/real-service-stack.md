# Real-service test stack (rootless podman)

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:43:12Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); independent review of that round owed (constitution 11.4.142 / 11.4.209); previously committed in 6d5ebb64 and revised after the WF12 review; linked from docs/README.md (11.4.212); script guides are indexed in [docs/scripts/test_infra_index.md](../scripts/test_infra_index.md) |
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
scripts/test-infra/roundtrip.sh --build-id <id> --protocol ftp [--record <absolute evidence dir> --iteration <n>]
scripts/test-infra/down.sh --build-id <id> --op-id <the op_id up.sh printed>
scripts/test-infra/sweep_leaks.sh [--dry-run]     # removes leaks of earlier runs, each only after its ownership is proven
scripts/setup-test-env.sh [--build-id <id>]       # the same start through one command; scripts/container-build.sh generates the per-run env for docker-compose.build.yml
```

One compose project per run, `catalogizer-test-<id>`. Credentials are generated per run (`gen_env.sh`) into a mode 0600 file under the gitignored `.audit/test-infra/<project>/`; host ports are random
loopback ports; every compose file reads `${TI_*}` variables only (no literal credential, no literal host port). The corpus is deterministic (`seed_corpus.sh`, two runs byte-identical, recorded seed).
The lease is per compose project (`scripts/longops`): a second owner of the same project is refused (`lease_held`, exit 3), a different project is admitted at once, a dead holder is never taken over silently (`lease_stale`, exit 4). The lease is a LIVE keeper process of the owning start (the tests check the holder identity and its liveness, never that a claim directory exists).
`down.sh` is owner-checked: a live holder is torn down only by the caller that names its operation (`--op-id`), any other caller is REFUSED (`not_lease_owner`, exit 5); a holder proven dead is reaped by any caller (its operation becomes `reaped`, no signal is sent).
`down.sh` removes only resources labelled with the project, plus what carries no label and would otherwise leak: the empty podman pod `pod_<project>` and the `<project>-client` / `<project>-seed` output directories of exactly that project. Containers start WITHOUT a pod (`podman-compose --in-pod false`), so a start no longer creates one.
A published port taken between the random draw and the bind is retried (3 attempts, fresh ports and credentials, announced); the cached corpus is re-verified (digest recomputed from the files) on every start.
**Clients never see the repository.** run_pinned binds its working directory at `/src`; the client runs from a scratch view that holds only the client scripts the command names (`/src/scripts/test-infra/...`), so neither the real `.env` (NAS credentials) nor any project's per-run credential file is readable from a client (`tests/infra/test_client_isolation.sh`). Residual (UNCONFIRMED fix, owed): the TIC lanes (`scripts/test-in-container.sh`, used for the seeder and the NFS state checks) still mount the whole repository read-only; that seam is owned by `scripts/containers/` and needs its own change. The isolation tests judge EXPOSURE, not script substrings: the view is built from canonical paths (a symlink, a `.`/`..`/empty path component, an env-file word, or a path outside `scripts/test-infra/` and `.audit/scratch/` is `client_path_not_in_view`; a `.env*`/`*.env` file inside an allowed directory is not copied), and the container's real mount set and `/src` contents are asserted (`tests/infra/test_client_isolation.sh`).
Operational contract (WF17 round 5): every start/teardown is serialised by one per-user project lock; the registered operation has a no-progress budget (`TI_OP_BUDGET_S`, default 3600 s) and a keeper that heartbeats only while a labelled container runs; resources are owned only when they carry `project`, `catalogizer.test_project`, `catalogizer.test_root` (this checkout) and an operation of THIS registry (anything else is `foreign_owner`, exit 3 for `up.sh` / exit 5 for `down.sh`, and is never touched); INT, TERM and HUP tear a start down; the compose files set `x-podman: {in_pod: false}`; test hooks (`TI_COMPOSE_FILE`, `TI_CORPUS_CACHE_DIR`, `TI_TEST_SLEEP_*`, ...) exist only under `TI_TEST_MODE=1`; the build stack (`container-build.sh`) follows the same contract and its Redis requires a per-run password.
Two stacks at once do not collide (`tests/infra/test_concurrency.sh`, `evidence/wp12/concurrency.json`).

## NFS (T134, docs/16 DR-16-2, finding D-10)

The unprivileged attempt: a user-space NFSv3 server (unfs3) in a rootless container, no kernel NFS server, no kernel mount, no added capability, and the libnfs user-space client. State: see
`specs/001-full-project-audit-remediation/evidence/wp10/nfs-attempt.json` (one terminal state, chosen by `scripts/test-infra/nfs_terminal_state.sh`, which reads the client verdict
`wp11/nfs-client.json` first) and `wp10/nfs-attempt-servers.md` (the servers tried and why each was kept or dropped).

What a `pass` proves: an NFSv3 round trip between a user-space server and the libnfs user-space client, rootless. What it does NOT prove: the application's own NFS path. `submodules/filesystem/pkg/nfs`
and `catalog-api/filesystem/nfs_client.go` mount through the KERNEL (`syscall.Mount`, `evidence/wp12/nfs-codepath.md`), which needs a privilege a rootless container does not have; that path stays
UNCONFIRMED on this host. The real-host leg (T134a) is NOT closed by that pass: the `pass` covers the protocol, not the kernel mount path, so `nfs_fallback_state.sh` records the leg `blocked` (`odg08_unanswered`, owed to ODG-08 and T159) with the unconfirmed path named (`evidence/wp10/nfs-fallback.json`); it could be `not_needed` only for an attempt record that proves the kernel path. `blocked_external.sh` derives its `nfs_owner_host` leg from that record, so the two records cannot disagree. The owner's seven Synology hosts have NFS closed from this host
(`docs/infrastructure/synology-hosts.md`). Owner decision owed (11.4.66): re-scope T134a to the application path or register the kernel-path confirmation as its own tracked item.

## Real NAS leg (read-only, separate)

`scripts/test-infra/nas_readonly_leg.sh` lists one DATA share per host (depth 1) and reads at most one small file, at most 2 requests per second, from IMG-INFRA-CLIENT; it never writes. Credentials come only
from the gitignored `.env` (`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_<n>`); the leg SKIPs (not fails) when the file is absent. It is not part of the deterministic suite.

The run directory with the auth file is `$XDG_RUNTIME_DIR/catalogizer-nas-ro.<pid>` (per-user tmpfs, mode 0700), never the checkout: a SIGKILL leaves it on the tmpfs only and `sweep_leaks.sh` removes it once the pid is proven gone; INT/TERM/HUP remove it. The secrecy test checks every execve of the leg's process tree (`strace -f -e execve`), not a 1 Hz sample.

## Legs that need an external credential or device (T135)

`scripts/test-infra/blocked_external.sh` records, per leg, `available` or `blocked-unavailable` with the reason and the variable NAMES (never values), BLOCKED-ON ODG-01 for credentials: `nas_smb_readonly`
(`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_3`, `SYNOLOGY_IP_4`), `minio` (`image_unavailable`), `nfs_owner_host` (ODG-08 unanswered). A blocked leg is never a pass. Record:
`evidence/wp12/blocked-external.json`.

## Known limits (UNCONFIRMED unless stated)

- FTP passive mode is proven on the compose network (the clients run on it). Passive mode through a published host port under pasta is UNCONFIRMED: a published passive range cannot use random host ports.
- The compose file adds one capability, `AUDIT_WRITE`, to the FTP service: without it the FTP service never becomes ready under this host's podman defaults (`tests/infra/test_ftp_capability.sh`, `evidence/wp12/wf12/ftp-capability.txt`; the exit text "252" quoted earlier was not reproduced: pure-ftpd logs to syslog). It is a capability of the container's own user namespace; nothing is granted on the host.
- The consumers of the reworked compose files were migrated (WF12 F1): `scripts/container-build.sh` (generates the per-run env, `--validate-only`), `scripts/setup-test-env.sh` (starts through `up.sh`; the stub compose generator with fixed ports and `testuser`/`testpass` is gone), `catalog-api/tests/infra_helper.go` (reads the per-run env file named by `CATALOGIZER_TEST_INFRA_ENV`; it has no caller in the tree and is kept, not removed), and the live documentation. `tests/infra/test_consumers.sh` scans the tracked files for the retired-contract patterns it lists (fixed ports 1445, 2121, 8081, 2049 as `TI_PORT_*` values or host literals, the credential words `testuser` / `testpass`, a heredoc that rewrites `scripts/setup-test-env.sh`, `getEnvOrDefault` defaults of the old ports), each pattern with a control needle, and every invocation of `down.sh`; `catalog-api/tests/integration/protocol_connectivity_test.go` and `docs/phases/PHASE_0_FOUNDATION.md` were migrated by that enumeration (the Go test now reads the per-run env file named by `CATALOGIZER_TEST_INFRA_ENV`; whether it compiles is UNCONFIRMED here: it needs the TIC Go lane).
- Per-run credentials reach the compose tool as `-e` arguments of `podman create` and persist in the container's create command (`podman inspect`): **readable by every local user** while `podman create` runs and as long as the container exists: `/proc` is mounted without `hidepid` and `/proc/<pid>/cmdline` is mode 0444, and the generated SMB password was sampled in the argv of `podman create` (WF17 TI-F3; the earlier text "readable by the same user only" was wrong). The values are random per run and the ports are loopback-bound, but the mode 0600 file is NOT the only holder. This limit is still ACCEPTED, not closed: the owner decision (keep it accepted, or mitigate by services reading a mounted 0600 secret file or an env-file pass-through) is OPEN (constitution 11.4.66) and F9 is not marked closed until it is answered.
- TIC (`scripts/test-in-container.sh`) is refused (`anti_mess_drift`) while any `run_pinned` container of any stream is running, so the test-infra scripts retry it up to 400 times, 5 s apart (up to about 33 minutes when the host stays busy; the retry count is printed, never silent). Root cause of the stack-induced part (WF17 class A, measured with the real sweep): the sweep reads `catalogizer.op_id` first and flags `hung_op`, `orphan_container`, `container_without_op_label`, `registry_row_dead_owner` and `duplicate_owner`; a running stack used to trip it because its keeper heartbeated nothing (the operation went `hung` after the default budget), its client containers carried an operation id other than the stack's, and the build stack was not registered at all. All three are fixed and `tests/infra/test_sweep_contract.sh` runs the real sweep at every lifecycle point of a start, probe, client run and teardown. The retry remains for `run_pinned` containers of OTHER streams.
- Tests must run one at a time on a shared host (see above).
