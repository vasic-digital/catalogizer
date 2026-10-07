# Synology hosts (owner-supplied, read-only)

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T17:45:00Z |
| Status | active (SMB survey of 2026-10-06; NFS/FTP/FTPS/SFTP survey of 2026-10-07) |
| Decision | ODG-08 (NFS and real hosts), ODG-01 (credentials) |

Seven local-network Synology hosts supplied by the owner for scanning, crawling, catalog population, testing and investigation. Use is READ-ONLY: no writes, deletes, renames, share-config or DSM changes; bounded request rates; NAS content is third-party data and is never copied into the repository or evidence beyond names and counts.

## Credentials

Stored only in the gitignored, mode 0600 `/home/milosvasic/Projects/catalogizer/.env`, by variable name: `SYNOLOGY_SMB_USER`, `SYNOLOGY_SMB_PASSWORD`, `SYNOLOGY_HOST_1` to `SYNOLOGY_HOST_7`, `SYNOLOGY_IP_1` to `SYNOLOGY_IP_7`. Never commit, print, log or pass them in argv; tools read them through a 0600 auth file (`smbclient -A`) outside the repository.

## Hosts and shares (measured 2026-10-06 from host anton)

| Alias | IP | Read-only account can list | Read-only account denied (NT_STATUS_ACCESS_DENIED) | Protocols |
|---|---|---|---|---|
| Synology | 192.168.1.25 | DATA8 | MASS1-5, MASS6-9, music | SMB 2.02-3.11 open; NFS closed from anton |
| Synology2 | 192.168.1.40 | Data | usbshare1, usbshare2-1, usbshare2-2 | SMB 2.02-3.11 open; NFS closed from anton |
| Synology3 | 192.168.1.127 | DATA12 | - | SMB 2.02-3.11 open; NFS closed from anton |
| Synology4 | 192.168.1.128 | DATA18 | - | SMB 2.02-3.11 open; NFS closed from anton |
| Synology5 | 192.168.1.129 | DATA20 | music, WORK20 | SMB 2.02-3.11 open; NFS closed from anton |
| Synology6 | 192.168.1.130 | DATA20-2, DATA20-3 | music | SMB 2.02-3.11 open; NFS closed from anton |
| Synology7 | 192.168.1.131 | DATA22, DATA22-2 | music | SMB 2.02-3.11 open; NFS closed from anton |

- SMB1 is refused everywhere; anonymous and guest listing are refused everywhere.
- DSM web ports 5000 and 5001 are open on all seven (reachability only).
- NFS (2026-10-06): TCP 2049 and 111 were refused on all seven; superseded by the 2026-10-07 survey below (NFS, FTP and SFTP were enabled by the owner in between).
- SMB signing requirement is UNCONFIRMED; SMB3 encryption is supported and not required (IPC listing only).

## Protocol matrix (measured 2026-10-07, read-only, from host anton through rootless containers)

Evidence: `specs/001-full-project-audit-remediation/evidence/wp12/nas-protocols/` (`summary.json`, per-host-per-protocol JSON, `SHA256SUMS`); tool `scripts/test-infra/nas_protocols.sh` (`docs/scripts/nas_protocols.md`).
Listing latency is the per-request p50 / p95 in ms over the bounded breadth-first sample; read is the median over the shares of the throughput of reading (and discarding) the first 1 MiB of one file, in MiB/s, total (from the request)
and after the first byte. Each read is a single observation per share, so the rankings are indicative: the first request of a share can include disk spin-up (hosts 6 and 7 show it as high p95 and a low total next to a high after-first-byte rate).

| Host | NFS | FTP | FTPS (explicit, port 21) | SFTP | Best read (total / after first byte) |
|---|---|---|---|---|---|
| Synology | v2/v3 offered, no exports listed, all MNT refused (ACCES) | port 21 closed | not reachable (same port) | port 22 closed (connection refused) | none yet |
| Synology2 | as above | works, 5.1 / 63.6 ms, 54.8 MiB/s (86.7 after first byte), 4 shares | works, 25.6 / 49.2 ms, 23.3 (73.5) | works, 8.7 / 86.3 ms, 10.7 (11.6) | FTP / FTP |
| Synology3 | as above | works, 6.1 / 145.7 ms, 77.6 (93.3) | works, 28.7 / 194.5 ms, 25.3 (58.2) | works, 5.7 / 51.2 ms, 13.1 (13.9) | FTP / FTP |
| Synology4 | as above | works, 4.8 / 16.5 ms, 85.8 (107.1) | works, 25.2 / 31.5 ms, 26.5 (65.5) | works, 6.8 / 34.4 ms, 3.4 (13.2) | FTP / FTP |
| Synology5 | as above | works, 2.8 / 47.0 ms, 81.6 (97.9), 3 shares | works, 15.8 / 70.6 ms, 26.1 (46.2) | works, 4.8 / 44.3 ms, 17.3 (18.1) | FTP / FTP |
| Synology6 | as above | works, 5.5 / 1066 ms, 1.8 (42.6), 3 shares | works, 19.6 / 1097 ms, 0.6 (47.1) | works, 4.4 / 38.0 ms, 2.8 (17.3) | SFTP / FTPS |
| Synology7 | as above | works, 7.1 / 25.4 ms, 0.6 (60.0), 3 shares | works, 17.7 / 78.3 ms, 0.3 (108.4) | works, 6.8 / 66.0 ms, 13.2 (16.0) | SFTP / FTPS |

Findings (all from the evidence files; names and addresses are not recorded):

- **NFS**: portmap (111) and NFS (2049, TCP and UDP) answer on all seven; versions 2 and 3 are offered, **NFSv4 is not** (NULL on v4 answers PROG_MISMATCH with range 2-3). MOUNT EXPORT returns an EMPTY list on all seven and MNT of `/volume1`, `/volume2`
  and `/volumeN/<share>` is refused with `MNT3ERR_ACCES` - the same code the control path that cannot exist gets, so the refusal is "this client address is in no NFS permission rule", not path-specific. No directory could be read over NFS, so
  NFS latency and throughput are UNMEASURED on the real hosts (the walker `nas_proto_nfs.sh` is verified only against the user-space NFS fixture). Fix on the NAS side (owner): add an NFS permission rule for the client address on the shared folders.
- **FTP**: servers answer `220 SynologyN FTP server ready`, FEAT lists MLSD, MLST, UTF8, REST, SIZE, MDTM, MFMT, AUTH, PBSZ, PROT, CCC, TVFS. The `reader` account logs in over plain FTP (no TLS enforced) on hosts 2-7. Non-ASCII names need
  `OPTS UTF8 ON` first; without it the server answers 0x7f for every such character and the directory cannot be addressed.
- **FTPS**: explicit TLS (AUTH TLS) on port 21; TLS 1.3 negotiated (TLS_AES_256_GCM_SHA384), TLS 1.2 also accepted; the certificates are not self-signed (issuer differs from subject; expiry 2027-02 to 2027-09, in the evidence).
- **SFTP**: OpenSSH_8.2, `sftp` subsystem v3, curve25519-sha256 + chacha20-poly1305, host keys ecdsa-sha2-nistp256, ssh-ed25519, ssh-rsa on hosts 2-7; password authentication works for `reader`. Over FTP and SFTP the account lists a
  WIDER set than over SMB: `usbshare1` and `usbshare2-2` (Synology2) and `WORK20` (Synology5) were denied over SMB on 2026-10-06 but list and read over FTP/SFTP, while `usbshare2-1` and the `music` shares list empty (no file readable in the sample); the share-level
  permissions of the NAS differ per protocol and are worth the owner's review. The sampled entries carry mode 755 / 555 owned by the NAS user, so the read-only property of `reader` is inferred from permissions only (no write was attempted) and is UNCONFIRMED beyond that.
- **Host 1 (Synology)**: SMB and NFS (portmap) answer; FTP (21) and SSH (22) refuse connections (re-checked at the end of the run), so FTP, FTPS and SFTP are not enabled there yet.
- Plain FTP is the fastest for hosts 2-5 and has the lowest listing latency; FTPS costs about 20 ms per listing for the TLS data channel. SFTP is steady but slower per byte. For hosts 6 and 7 the total rates are dominated by a slow first request.

## Use in tests

- T132 Samba leg: usable read-only (DATA shares of the five newer hosts and `Data`/`DATA8`).
- T134a real NFS host: NFS is enabled on all seven but no path is exported to or mountable by anton (empty export list, MNT refused); stays UNMET until the owner adds an NFS permission rule for anton on a shared folder.

Evidence and full tables: `specs/001-full-project-audit-remediation/evidence/wp10/synology-survey/` (`host-table.md`, `hosts.json`).

## Bounded content survey and catalog-scanner run (2026-10-07)

Read-only, 1 request per second, depth <= 3, <= 5000 entries and <= 80 listing requests per share, run in the pinned IMG-INFRA-CLIENT container (`scripts/test-infra/client/nas_survey.sh`). Only counts, a sha256 of the sorted top-level names and an extension histogram are recorded; names are never stored. Evidence: `specs/001-full-project-audit-remediation/evidence/wp12/nas-survey/` (`survey-1..7.json`, `SHA256SUMS`).

| Host | Share | Top-level entries | Sampled entries | Truncated by bound |
|---|---|---|---|---|
| Synology | DATA8 | 7 | 637 | yes |
| Synology2 | Data | 14 | 11169 | yes |
| Synology3 | DATA12 | 12 | 5188 | yes |
| Synology4 | DATA18 | 13 | 1000 | yes |
| Synology5 | DATA20 | 8 | 4694 | yes |
| Synology6 | DATA20-2 | 7 | 5025 | yes |
| Synology6 | DATA20-3 | 11 | 589 | yes |
| Synology7 | DATA22 | 12 | 1343 | yes |
| Synology7 | DATA22-2 | 2 | 473 | no (complete to depth 3) |

All other shares answered NT_STATUS_ACCESS_DENIED for the read-only account. Sampled counts are a bounded breadth-first sample, not share totals.

Catalog scanner run: the real `SMBScanner` + `insertFileRecord` ran in the pinned IMG-GO container (overlay test `scripts/test-infra/nas_scan/zz_nas_scan_test.go`, a throttled decorator that refuses every write-class method) against `DATA22-2` into an in-memory SQLite database with the full migration chain: 473 rows ingested (458 directories, 15 files), equal to the 473 entries observed on the wire, 13 listing requests, 0 writes (`scan-DATA22-2.json`). Limits: in-memory database only (no persistent catalog populated), one share, depth 3, no file content read, no metadata providers run.

NFS stays UNCONFIRMED (ports closed from anton).
