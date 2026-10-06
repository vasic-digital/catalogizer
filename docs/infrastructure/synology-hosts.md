# Synology hosts (owner-supplied, read-only)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T18:10:00Z |
| Status | active (survey of 2026-10-06) |
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
- NFS: TCP 2049 and 111 refused, UDP 111 silent, libnfs `nfs-ls` fails (`nfs_service failed`) on all seven, from the host and from the pinned IMG-INFRA-CLIENT container. Which host exports NFS, the export list and the allowed client address are UNCONFIRMED.
- SMB signing requirement is UNCONFIRMED; SMB3 encryption is supported and not required (IPC listing only).

## Use in tests

- T132 Samba leg: usable read-only (DATA shares of the five newer hosts and `Data`/`DATA8`).
- T134a real NFS host: not usable until the owner enables NFS and permits anton; stays UNMET.

Evidence and full tables: `specs/001-full-project-audit-remediation/evidence/wp10/synology-survey/` (`host-table.md`, `hosts.json`).

Owed: link this page from `docs/scripts/README.md` or the main README (constitution 11.4.212).
