# nas_clients_leg.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T00:00:00Z |
| Status | new, working tree (not committed); independent review owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/nas-clients/` |
| Source | `scripts/test-infra/nas_clients_leg.sh`, harness `scripts/test-infra/nas_clients/` (own Go module, build tag `nasleg`) |

## Purpose

Validates the new Go protocol clients of `submodules/filesystem` against the owner's real Synology hosts (`docs/infrastructure/synology-hosts.md`), READ-ONLY, one protocol at a time:
`pkg/ftp` (FTP and explicit FTPS with certificate pinning), `pkg/sftp` (host-key pinning), `pkg/nfs3` (user-space NFSv3), with `pkg/fabric` (Retrying, Limited, Confined) and
`pkg/decorators` (ReadOnly). The earlier survey (`nas_protocols.sh`, Python/ssh/libnfs) measured the servers; this leg measures the **Go clients** through the production chain.
No package code is modified; the harness is a separate nested module that `replace`s `digital.vasic.filesystem` with the submodule.

## What one host x protocol run does

1. Builds the chain `Confined > Retrying > Limited > ReadOnly > tripwire > client` (one `HostBudget`: 1 concurrent, 1 start/second). The tripwire is a harness layer BELOW `ReadOnly`: any write-class call that
   reaches it is refused there (nothing is ever sent to a NAS) and counted; the result records the probe outcome of all five write-class methods through the live chain and `tripwire_hits_below_ReadOnly`.
2. Negative tests of the trust layer before any credential-bearing connect: unpinned certificate/host key (`UnknownCertError` / `UnknownHostKeyError`), changed certificate/host key (`CertMismatchError` /
   `HostKeyMismatchError`), a `Pin` with a wrong confirmation, clear-text FTP without `trusted_lan` (`ErrClearTextRefused`).
3. Pins the certificate / host keys recorded by the earlier survey (`evidence/wp12/nas-protocols/ftps-<n>.json`, `sftp-<n>.json`) through the clients' `Pin` APIs. **This is a SIMULATED owner confirmation**
   (the recorded survey fingerprint stands in for the out-of-band owner step) and the evidence states so; a mismatch is reported as a refusal, never bypassed.
4. Connects, lists the root (the shares), takes a bounded breadth-first sample per share (depth <= 3, <= 2000 entries, <= 80 listing requests), reads at most 1 MiB of ONE file per share (the largest file of at most
   1 MiB, deterministic; the bytes are hashed and discarded), records listing p50/p95, TTFB, throughput, and the minimum gap between client-level requests.
5. Cross-checks the top-level entry count (and the sampled totals when neither side hit its bound) against `evidence/wp12/nas-survey/survey-<n>.json`. A share that SMB **denies** but the protocol lists is recorded
   as a SECURITY finding; for such a share only the bounded listing sample is taken and no file is read.
6. NFS: `MOUNT EXPORT`, then one `MNT` attempt per candidate path (exports, `/volume1`, `/volume2`, `/volumeN/<share>` for the SMB-listed shares, one nonexistent control path) with `TryPrivilegedPort` on;
   the precise `AccessError` (layer, source port, privileged or not) and mount status are recorded. A refused mount is BLOCKED (the server's export rule), not a client failure.

## Credentials and privacy

Credentials only from the gitignored `.env` (`SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_IP_<n>`), parsed by `dotenv_get.py`, written to a 0600 env file in a 0700 directory under `/dev/shm`, handed to the
container with `--env-file` (names only on the command line) and deleted after every container run and on every exit path. The harness resolves the clients' `credential_ref` from that environment by name.
Every recorded string is scrubbed of the user name, password and addresses; the harness refuses to write a result that contains one, and the script scans all results and logs again (fixed-string pattern file,
plus a generic dotted-quad scan with a control needle) and publishes nothing on a hit. Entry names are never recorded: counts, the sha256 of the sorted top-level names, extension histograms.

## Usage

```bash
TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1 scripts/test-infra/nas_clients_leg.sh [--hosts 1,2,3,4,5,6,7] [--protocols ftps,ftp,sftp,nfs] [--ev-dir DIR] [--assemble-only] [--diag]
```

- `--assemble-only`: no container; scan and assemble the results already under `.audit/out/nas-clients/res` (a protocol run refused for host memory, `run_pinned: REFUSED reason=memory_budget_unavailable`, can be repeated alone). A run replaces only the results of the protocols and hosts it measures.
- `--diag`: runs `TestDiagFTPUnparsed` (`nas_diag_test.go`, Synology6 / DATA20-3) instead of the main test: re-walks the same deterministic sample over FTP up to the first `ErrListingIncomplete`, records the STRUCTURE of the unparsed line and, over SFTP, counts of peculiar entry names of that directory (no names). Result `diag-ftp-6.json`.

One container at a time (rootless, `run_pinned.sh IMG-GO`); inside a protocol run the hosts are goroutines, each host sequential at <= 1 request/second. Go module and build caches persist in
`.audit/out/nas-clients/`. Without the env file: `SKIP nas_clients reason=env_absent` (exit 0); with missing variables: `SKIP ... credentials_absent variables: <NAMES>`. Exit 1 on a leak, a failed container run or an
assembly error; a refused, blocked or unreachable protocol is a recorded result.

## Evidence

`specs/001-full-project-audit-remediation/evidence/wp12/nas-clients/`: `<proto>-<n>.json` (identity header, tool hashes, submodule head, image digest), `summary.json` (matrix, measurements, cross-protocol
consistency, defects, findings), `go.sum.generated`, `SHA256SUMS` (verifies from inside the directory).

## Honest limits

- The client-level request spacing is measured at the layer under `Limited`; the protocol clients issue several wire commands per call (an FTP listing is PASV/MLSD), which the harness cannot count.
- "No credential sent" on a refused pin is the clients' documented behaviour; the harness does not capture the wire, so it is not independently verified here.
- Clear-text FTP sends the password unencrypted on the LAN; it is exercised because the task scope names FTP, and only with `trusted_lan=true`.
- A bounded sample is not a census: totals are compared with SMB only when neither side hit its bound.
