# Synology survey (WP-10, ODG-08), CLAIM versus MEASURED

- generated_utc: 2026-10-06T18:08:20Z
- source: `hosts.json` sha256 `af22a9fa54ee6aefc8513fe11dc22dae4c5bf6e8a413dd2c101af68a15bda897`
- from_host: anton; tools: IMG-INFRA-CLIENT sha256:83085e496b6219f5bf477dd5e099a767b9b6889e33dfa50a6c59106a8d137f62 (smbclient 4.17.12, libnfs-utils 4.0.0, bash /dev/tcp)
- credentials: variable names only (SYNOLOGY_SMB_USER, SYNOLOGY_SMB_PASSWORD in the gitignored `.env`); no value recorded
- rules: read-only, depth-1 listings, at most 2 requests/s per host, names and counts only, no file content copied

## Host table

| Alias | IP | SMB claim | SMB measured | NFS claim | NFS measured | DSM 5000/5001 | Anonymous / guest | SMB dialects accepted |
|---|---|---|---|---|---|---|---|---|
| Synology | 192.168.1.25 | yes | 445=open 139=open; read-only account lists 4 of 4 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology2 | 192.168.1.40 | yes | 445=open 139=open; read-only account lists 4 of 4 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology3 | 192.168.1.127 | yes | 445=open 139=open; read-only account lists 1 of 1 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology4 | 192.168.1.128 | yes | 445=open 139=open; read-only account lists 1 of 1 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology5 | 192.168.1.129 | yes | 445=open 139=open; read-only account lists 3 of 3 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology6 | 192.168.1.130 | yes | 445=open 139=open; read-only account lists 3 of 3 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |
| Synology7 | 192.168.1.131 | yes | 445=open 139=open; read-only account lists 3 of 3 claimed shares | some hosts, which unknown | 2049=closed_refused 111=closed_refused udp111=udp111_no_reply: closed from anton | open / open | NT_STATUS_LOGON_FAILURE / NT_STATUS_ACCOUNT_DISABLED | 3_11, 3_02, 3_00, 2_10, 2_02 |

SMB1 (NT1) is refused on every host (no compatible protocol). The claimed share set equals the listed Disk share set on all 7 hosts (`shares_claim_equals_listed`).

## Share access by the read-only account (depth-1 `ls` only)

| Host | Share | Read-only account access | Top-level entries | Capacity |
|---|---|---|---|---|
| Synology | DATA8 | listable | 7 (7 dirs, 0 files) | 0.09 / 7.22 TiB free/total |
| Synology | MASS1-5 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology | MASS6-9 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology | music | NT_STATUS_ACCESS_DENIED | - | - |
| Synology2 | Data | listable | 14 (13 dirs, 1 files) | 0.04 / 7.22 TiB free/total |
| Synology2 | usbshare1 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology2 | usbshare2-1 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology2 | usbshare2-2 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology3 | DATA12 | listable | 12 (11 dirs, 1 files) | 1.46 / 10.82 TiB free/total |
| Synology4 | DATA18 | listable | 13 (12 dirs, 1 files) | 0.80 / 16.30 TiB free/total |
| Synology5 | DATA20 | listable | 8 (7 dirs, 1 files) | 1.17 / 17.45 TiB free/total |
| Synology5 | music | NT_STATUS_ACCESS_DENIED | - | - |
| Synology5 | WORK20 | NT_STATUS_ACCESS_DENIED | - | - |
| Synology6 | DATA20-2 | listable | 7 (6 dirs, 1 files) | 0.09 / 17.45 TiB free/total |
| Synology6 | DATA20-3 | listable | 11 (10 dirs, 1 files) | 0.18 / 17.45 TiB free/total |
| Synology6 | music | NT_STATUS_ACCESS_DENIED | - | - |
| Synology7 | DATA22 | listable | 12 (11 dirs, 1 files) | 6.97 / 17.45 TiB free/total |
| Synology7 | DATA22-2 | listable | 2 (1 dirs, 1 files) | 2.03 / 17.45 TiB free/total |
| Synology7 | music | NT_STATUS_ACCESS_DENIED | - | - |

Read-only account account cannot open (NT_STATUS_ACCESS_DENIED): .25 MASS1-5, MASS6-9, music; .40 usbshare1, usbshare2-1, usbshare2-2; .129 music, WORK20; .130 music; .131 music. These shares are listed to the account but denied on tree connect: the owner may have excluded them on purpose; not tried by any other means.

## First 50 top-level names per listable share (third-party data, names only)

- Synology `DATA8`: `Филмови/`, `Серије/`, `Инсталације/`, `Разно/`, `Виртуализација/`, `Стрипови/`, `Музика/`
- Synology2 `Data`: `Филмови/`, `Руски/`, `Графика/`, `Инсталације/`, `Документарци/`, `Часописи/`, `Концерти/`, `Фонтови/`, `Сигурносне копије/`, `Цртаћи/`, `Музика/`, `Тренинзи/`, `Серије/`, `.DS_Store`
- Synology3 `DATA12`: `Филмови/`, `Концерти/`, `Цртани филмови/`, `Привремено/`, `Инсталације/`, `Филмови - Марвел/`, `Тренинзи/`, `Серије/`, `Стрипови/`, `Музика/`, `Документарци/`, `.DS_Store`
- Synology4 `DATA18`: `Серије/`, `Инсталације/`, `Документарци/`, `Тренинзи/`, `Концерти/`, `ISO/`, `Књиге/`, `Часописи/`, `.DS_Store`, `Музика/`, `Стрипови/`, `Цртани филмови/`, `Филмови/`
- Synology5 `DATA20`: `.DS_Store`, `Mounted/`, `Документарци/`, `Инсталације/`, `Mounted2/`, `Серије/`, `Тренинзи/`, `Цртани филмови/`
- Synology6 `DATA20-2`: `Филмови/`, `Серије/`, `Инсталације/`, `.DS_Store`, `Концерти/`, `Цртани филмови/`, `Тренинзи/`
- Synology6 `DATA20-3`: `.DS_Store`, `Тренинзи/`, `Концерти/`, `Музика/`, `Филмови/`, `Цртани филмови/`, `Инсталације/`, `Серије/`, `Књиге/`, `Стрипови/`, `Документарци/`
- Synology7 `DATA22`: `Mounted/`, `Стрипови/`, `Тренинзи/`, `Филмови/`, `Документарци/`, `Концерти/`, `Музика/`, `Серије/`, `Mounted2/`, `Цртани филмови/`, `Књиге/`, `.DS_Store`
- Synology7 `DATA22-2`: `Инсталације/`, `.DS_Store`

## NFS (ODG-08)

Measured from anton directly (TCP connect, UDP portmapper DUMP) and from the pinned IMG-INFRA-CLIENT container (bash /dev/tcp and libnfs `nfs-ls nfs://<ip>/`, which uses the mount protocol through the portmapper): TCP 2049 refused, TCP 111 refused, UDP 111 no reply, `nfs_service failed` on all 7 hosts; positive control: TCP 445 opens from the same container on all 7 (container network works). Verdict per host: **closed from anton**. NFSv4 on 2049 and v3 through the portmapper are both unreachable; exports and the allowed client address are UNCONFIRMED (the server cannot be asked while it refuses). Not probed: other ports (out of the named scope).

Possible explanations, none confirmed: the DSM NFS service is disabled on all 7; or enabled but firewalled for anton only (a refused connection normally means no listener, a firewall DROP normally times out: the refusals favour no listener); or NFS lives on a host not in the list. The owner can confirm in DSM Control Panel > File Services > NFS and the share NFS permission list (anton address).

## SMB security probe

signing: UNCONFIRMED whether the server requires signing (smbclient connected with client signing=off and required on every host; the client option does not show the server policy). Encryption: not required by the server (client smb encrypt=off connected on IPC listing), SMB3 encryption supported (encrypt=required connected); per-share encryption UNCONFIRMED.

## ODG-08 consequences

- T132 Samba leg (SMB protocol tests against a real server): usable. All 7 hosts serve SMB2.02 to SMB3.11 on 445 and the read-only account lists and opens the DATA shares (see table). Candidate read-only fixtures: .127 DATA12, .128 DATA18, .130 DATA20-2/DATA20-3, .131 DATA22/DATA22-2 (listable, small top level). Tests must stay read-only and depth-limited; write-path tests need the user-space Samba container (IMG-INFRA-SMB), not these hosts.
- T134a (real NFS host proof): not usable now. No host answers NFS from anton; real NFS host proof stays UNMET until the owner enables NFS (and permits the anton address) on a named host. The user-space NFS server leg is unaffected.
- DSM web ports 5000 and 5001 are open on all 7 (reachability only; no content requested).

## Owed

- Owner: confirm which host exports NFS, enable it and permit anton.
- Owner or conductor: link `docs/infrastructure/synology-hosts.md` from `docs/scripts/README.md` or the main README (not edited here).
- Signing requirement: needs a packet-level or server-side check (UNCONFIRMED).
