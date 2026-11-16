# NAS live run 2026-11-15 (PROTOCOL-LIVE-1) - PROVISIONAL

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-15 |
| Status | PROVISIONAL (one host, one operator-scheduled window; independent review owed) |
| Item | PROTOCOL-LIVE-1 (extends CAT-SYN-LIVE-1, LV3-2, ST-NFS) |
| Evidence class | REAL NAS hardware (the seven hosts of docs/infrastructure/protocol-live-hosts.md), not an emulator; no test double in any path |
| Hosts | aliases LiveHost1..7 only; no address, user or password is recorded (leak scan over this directory: 0 hits for the user, the password and all 7 addresses) |

Context: operator 2026-11-15 reported all NAS drives back online (they were off since 2026-11-05, honest SKIP hardware_not_present). Everything below ran READ-ONLY (listing, stat, bounded reads; the NFS leg only attempts MNT). Credentials were read by NAME from the gitignored repo .env (the two scratchpad env files named in the task did not exist), in subshells; nothing printed.

## 1. Reachability (read-only, reachability.txt, 2026-11-15T21:26:14Z)
All 7 hosts answer ping; SMB 445, NFS 2049, rpcbind 111, DSM 5000/5001 are open on all 7. FTP 21 and SFTP 22 are open on hosts 2-7 and CLOSED on LiveHost1 (as in the 2026-10-07 survey).

## 2. Results

| # | Test / leg | Command | Result | Evidence |
|---|---|---|---|---|
| 1 | SMB read-only leg (scripts/test-infra/client/nas_smb_ro.sh: share list + one depth-1 listing + one read of at most 64 KiB, writes=0), N=3 x 7 hosts | the script, run on the HOST (note A), /out redirected to a 0700 scratch dir | PASS live 21/21 (7 hosts x 3); names_sha256 and read_sha256 identical across the 3 runs on every host; LiveHost1 has no small file in DATA8 (read_size=none_small_file, listing PASS) | smb-readonly/run{1,2,3}_host{1..7}.tsv |
| 2 | catalog-api -tags live, real SMB clients: TestLiveSMB_ListStatReadWalk / NameRoundTrip / NegativeControls (6 shares, 4 controls) | cd catalog-api; GOMAXPROCS=2 nice -n 10 go test -p 1 -buildvcs=false -count=3 -tags live -run TestLive -timeout 45m -v ./filesystem | PASS live x3 (all subtests) | synlive_go_live_nas.log |
| 3 | same, real FTP clients: TestLiveFTP_ListStatReadWalk / NameRoundTrip / NegativeControls / PartialCloseAndDisconnectSemantics | same command | PASS live x3 | same log |
| 4 | TestLiveHelpers_Defaults, TestLiveWebDAV{Apply,Classify,Validate}* (local helpers, do not touch the NAS) | same | PASS x3 | same log |
| 5 | TestLiveNFS_NoExports | same | SKIP topology_unsupported x3 (the test's own text: exports empty); see item 9 for what the real NAS does now | same log |
| 6 | TestLiveRealFTP_* (4) | same | SKIP x3: they start a LOCAL pyftpdlib server (FTP_PYFTPDLIB_DIR unset); not NAS tests | same log |
| 7 | TestLiveWebDAV_*, TestLiveYandex* | same | SKIP credentials_absent (LIVE_WEBDAV_* not set); not NAS | same log |
| 8 | WP-12 NAS client harness (scripts/test-infra/nas_clients, tag nasleg: FTPS with pinned cert, FTP, SFTP with pinned host key, through the fabric chain Confined > Retrying > Limited > ReadOnly) | go test -tags nasleg -count=1 -run TestNASClients, once per protocol, host-side (note A), NCL_* env from .env | FTPS 6 works / 1 refused (LiveHost1: port closed); FTP 6 / 1 same; SFTP 6 / 1 same. Each go test PASS; the per-host verdict is in the JSON | nas-clients/{ftps,ftp,sftp}-{1..7}.json, nas-clients/synlive_go_*.log |
| 9 | same harness, NFS | NCL_PROTO=nfs | BLOCKED x7: every MNT refused with AccessError (5/5 or 7/7 attempts); the export rules do not admit this machine (server decision, not a client defect). NOT a PASS. portmap/nfsd are now reachable on all 7 and the MNT is refused, which differs from the older "no exports" wording | nas-clients/nfs-{1..7}.json |
| 10 | Paired mutation LIVE_MUTATE=password, SMB walk | -count=1 -run 'TestLive(SMB or FTP)_ListStatReadWalk' | SMB subtests FAIL as required (5 FAIL + 1 optional SKIP). The FTP half was cut by my 110 s timeout: UNCONFIRMED | synlive_go_live_mut_password.log |

Note A (honest deviation from "exactly as designed"): scripts/test-infra/nas_readonly_leg.sh and nas_clients_leg.sh run inside pinned containers (IMG-INFRA-CLIENT, IMG-GO). run_pinned.sh REFUSED with image_not_present_locally (pulling is a separate step, T005d, "never here"). I did not build or pull images (host load average about 39 on 16 CPUs). The same client script and Go harness were therefore run on the host with the host smbclient and host Go 1.26.8 (GOFLAGS=-mod=mod, module under submodules/filesystem, no code edited). The container path's credential isolation is NOT claimed. In item 1 the script is a copy of nas_smb_ro.sh with only the /out and /tmp paths rewritten.

## 3. Failures / reproducers
No test FAILed. The only non-PASS outcomes are the NFS BLOCKED (item 9, server-side export rules) and the closed FTP/SFTP ports on LiveHost1 (service not offered; matches the survey).

## 4. UNCONFIRMED / not run
- Emulator evidence (docs/research/nasemu) is separate and was not touched; nothing here is emulator evidence.
- The stack-3 suite (via a real catqa3-api stack) and storage idle-deadline / cut-recovery against the real NAS (LV3-2: exist only as loopback/unit tests) were NOT run.
- An NFS PASS needs the operator to add an NFS permission rule for this host (owner action).
- Certificate / host-key trust used the pins of the 2026-10-07 survey (a SIMULATED owner confirmation, as the harness states); no pin mismatch occurred.
- The pinned-container variants of items 1 and 8 were not run (note A).
- The FTP half of the password mutation (item 10).
