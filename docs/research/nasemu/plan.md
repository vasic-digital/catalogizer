# BG-NASEMU-2026-11-13 — Design, risks, work items (rev 2)
Based on report.md rev 2 (primary-source pass 2026-11-13). Read it first for evidence.

## 1. Recommendation on D5 (package layout) — decided here
**Add a NEW package `pkg/nasemu` in Containers, layered ON TOP of existing packages; do not stuff NAS profiles into `pkg/emulator`.**
Reasons (evidence): `pkg/emulator` is Android/ADB-shaped (`Containerized` Boot/Install/RunInstrumentation/gradle setters); NAS services are protocol servers with a different lifecycle (seed, protocol health, golden manifest). The reusable pieces are `pkg/runtime` typed run options (memory/cpus/pids/device/userns/publish/digest/labels, host-escalation refusals), `pkg/hostbudget` (Admit + Evidence JSON + KVMUsable), `pkg/health` (http/tcp/custom), `pkg/network`, `pkg/vm` (qemu/network/teardown). `nasemu` composes them; VM-tier profiles delegate to `pkg/vm` / a thin container-with-/dev/kvm run (vdsm, dockur images are containers that need `/dev/kvm`, `/dev/net/tun`, `NET_ADMIN`).
Prerequisite: `pkg/hostbudget` and the typed run options exist only on branch `fix/containers-g1-g2-g11` (not upstream). **NE-1 must land those on Containers `main` (reviewed, ff-only per §11.4.288) before `nasemu` builds on them**; also extend hostbudget with a **disk-free axis** (host `/` is 97%).
Conflict to resolve: existing Catalogizer test-servers use `--network host`; the typed runtime options refuse host-escalating forms. `nasemu` uses `Publish` (hostIP:port) and a port registry; connectors already take configurable ports (verify in NE-7).

## 2. Design
- `Profile` (DATA, config-injected, no Catalogizer literals §11.4.28): `Name, Tier(lite|vm), ImageRef@sha256, Protocols[], Ports, BindAddr, Env/secret refs (0600 file paths, never values), SeedSpec ref, Volumes, Limits(mem/cpu/pids), Need (for hostbudget), HealthProbe(protocol-level), LegalAttestationRef (required for vm tier), Labels`.
- **Lite tier (evidence-based starting set):** `smb` (real smbd or samba-in-kubernetes `quay.io/samba.org/samba-server`, published high port), `ftp`/`sftp`/`webdav`/`http`/`nfs` (rclone serve; NFSv3), `ftps` (existing python server), `s3` (Garage; SeaweedFS Apache-2.0 as fallback; versioning-dependent tests SKIP because Garage lacks versioning), optional "real-server fidelity" profiles (ProFTPD/vsftpd, SFTPGo). Seed = deterministic generator -> golden manifest JSON (path, size, sha256, mtime; unicode names, long path, empty dir, symlink, large file); the existing `corpus` and `verify.sh` are the migration source.
- **VM tier:** `vdsm` (vdsm/virtual-dsm, pinned digest + pinned local `.pat` with recorded sha256), `windows` (dockur/windows, operator licence), `macos` (dockur/macos, opt-in only). Each: LegalAttestation DATA required, `hostbudget.Admit` with `NeedsKVM`, KVM open-for-write probe, disk-free check, named volume on an operator-chosen disk, single-instance lock, bounded first-boot wait.
- Lifecycle: Plan -> Admit -> Start(rootless, labelled `nasemu.run=<id>`) -> WaitHealthy (real protocol probe) -> Seed verify -> Evidence JSON -> Stop -> label-scoped reap (own resources only, §11.4.174).
- Evidence JSON fields (§11.4.289(D)): profile, image digest, container id, endpoint actually reached, answering identity (executable/image digest), hostbudget Evidence, seed manifest sha256, probe transcript hashes, timings, SKIP reason from closed set (`hardware_not_present`, `credentials_absent`, `topology_unsupported`, `legal_attestation_absent`, `host_budget_refused`).
- Consumer binding: Catalogizer supplies a DATA file mapping connector test -> profile + env var names for host/port/creds; tests SKIP-with-reason when the emulator is absent; no stubs.
- Docs: user guide, manual, FAQ, diagrams (architecture, lifecycle state machine, sequence of a test run), README link (§11.4.212/.257/.258).

## 3. Gap / risk register (risk -> mitigation -> verifying test)
1. Host OOM, load ~20, ~16 GiB available -> hostbudget.Admit (60% ceiling, PSI, load/CPU) + container memory caps -> test: tight Policy yields `does_not_fit`, no container created.
2. Disk 97% / 57 GiB free (Windows 32 GB, macOS 64 GB default, vdsm 16 GB) -> add disk axis to hostbudget; VM disks only on an operator-chosen volume -> test: threshold probe live + fixture.
3. KVM: works now via ACL (`user:milosvasic:rw-`, verified) but ACL can vanish -> `KVMUsable` open-for-write each run, SKIP `hardware_not_present` otherwise -> live probe test.
4. LAN exposure -> default bind 127.0.0.1; LAN bind explicit DATA; generated 0600 creds; samba-container default `sambauser/samba` forbidden -> test: evidence shows bind address; default-cred login must FAIL.
5. Credentials leakage -> secrets by file ref only; redaction -> test: control-needle scan of evidence/logs.
6. Legal: DSM (Synology HW only, per vdsm FAQ), macOS (Apple HW only, per dockur readme), Windows (licence) -> VM tier off by default, attestation DATA required -> test: no attestation => SKIP `legal_attestation_absent`.
7. Provenance/reproducibility: vdsm downloads DSM (default 7.0.1) at runtime -> supply pinned `.pat`, record sha256; images by digest; Windows updates non-deterministic -> test: evidence includes digest + .pat hash; 3x runs (§11.4.50).
8. Rootless limits: ports <1024 (`ip_unprivileged_port_start=1024`), tun/NET_ADMIN/vhost-net for VM images, kernel nfsd/Ganesha impossible -> high ports + NFS via rclone; vdsm rootless = spike NE-0b with captured result; may end `topology_unsupported`.
9. Flakiness/slow first boot -> bounded waits, protocol-level health, deterministic seed -> N>=3 identical.
10. State/cleanup -> label-scoped reap, ephemeral lite volumes, VM volumes explicit -> test: no labelled leftovers after Stop.
11. Port clashes with catqa1/2/3 and existing `cattsrv-*` -> port registry DATA + pre-bind probe + `--network host` retired -> test.
12. Licences: SFTPGo/Garage/pgsty-MinIO AGPL, theme proprietary -> unmodified test infra only; dependency register entry (§11.4.270); MinIO upstream archived -> excluded.
13. Fidelity gap: rclone/Samba are not DSM -> docs state it; Synology-specific behaviour verified only against vdsm; no open-source Synology API mock exists, so any API shim is our own code (decision D6).
14. Unreviewed Containers branch (not upstream) -> NE-1 lands it first.
15. Existing test-servers regress during migration -> keep `docs/qa/test-servers` working until nasemu parity proven (verify.sh as acceptance).
16. Cost/time of ISO and .pat downloads -> VM tier opt-in.
17. Remaining UNCONFIRMED research items (rootless vdsm, API PDF details, /dev/sda1 mount state) -> NE-0b.

## 4. Test plan
Per tier LIVE, deterministic (N>=3 identical): start the real container, run the real Catalogizer connector AND an independent client (smbclient, sftp, curl, showmount/mount, aws-cli-class), compare listings and hashes to the golden manifest, negative controls (wrong creds rejected, absent path error), schema-check the evidence JSON, disconnection mutation (stop container => test must FAIL). Mocks only in unit tests of config/admission logic (§11.4.289). VM tier: boot to protocol-ready, same manifest checks, plus Synology API login/list (`SYNO.API.Auth`, FileStation) against vdsm.

## 5. Work items (S<=0.5d, M~1-2d, L>2d)
- **NE-0b (S, now):** residual verification spike: open Synology API PDFs; try rootless run of vdsm only after operator OK (D1); confirm `sda1` mount/free; record. Evidence: dated notes.
- **NE-1 (M, now):** review and land `fix/containers-g1-g2-g11` (G1/G2/G11) on Containers main; add disk-free axis to hostbudget with tests. Acceptance: tests green, ff-only on all upstreams, Catalogizer pin bumped.
- **NE-2 (M, NE-1):** `pkg/nasemu` core (Profile, load, lifecycle, evidence, label reap). Acceptance: unit + live start/stop of a trivial profile.
- **NE-3 (M, NE-2):** seed generator + golden-manifest verifier; migrate existing corpus approach. Acceptance: byte-identical on 3 runs.
- **NE-4 (M, NE-3):** lite profiles smb/ftp/ftps/sftp/webdav/http/nfs generalised from `docs/qa/test-servers`; Publish instead of host net. Acceptance: verify.sh parity + manifest match + neg controls.
- **NE-5 (M, NE-3):** S3 profile (Garage; decide Garage vs SeaweedFS — D7) with SigV4 client check; SKIP versioning tests.
- **NE-6 (S, NE-4):** NFS decision record: rclone serve nfs (evidenced) vs unfs3; document kernel/Ganesha exclusion.
- **NE-7 (M, NE-4):** Catalogizer binding DATA, port registry (no clash with catqa1-3/cattsrv), connector tests on custom ports.
- **NE-8 (L, NE-2, D1,D2,D4'):** VM tier base: hostbudget gate, KVM probe, attestation gate, volume on chosen disk, vdsm profile.
- **NE-9 (M, NE-8):** Synology API checks + Synology-specific connector behaviour against real vdsm.
- **NE-10 (L, NE-8, D2):** Windows profile, then macOS (only if D2 allows).
- **NE-11 (M, ongoing):** docs (guide, manual, FAQ, diagrams, README links, translations as mandated).
- **NE-12 (S each):** gates + paired mutations (no stub in live evidence, evidence fields, attestation required, dependency register).
Order: NE-0b, NE-1 -> NE-2, NE-3 -> NE-4, NE-5, NE-6 -> NE-7 -> NE-11/12 -> NE-8 -> NE-9 -> NE-10.
**Can start now (no operator decision):** NE-1, NE-2, NE-3, NE-4, NE-6 (lite tier all). **Needs operator:** NE-8/9/10 (D1,D2,D4'), NE-5 choice (D7).

## 6. Remaining operator decisions
- D1 Approve VM tier at all given host (about 16 GiB available RAM, 57 GiB free on `/`, load ~20) and choose the VM disk volume (candidate `sda1` 7.3T, mount state UNCONFIRMED).
- D2 Legal stance: DSM via vdsm (Synology EULA: Synology hardware only), Windows (licence source), macOS (Apple hardware only) — permit private testing or exclude each. XPEnology/Arc/redpill recommended excluded.
- D3 LAN exposure policy (bind addresses, ports, credentials), including retiring `--network host`.
- D4' KVM: no action needed now (ACL access verified); confirm that relying on a per-user ACL is acceptable or add user to `kvm`.
- D5 ANSWERED above (new `pkg/nasemu` over existing packages; land hostbudget branch first) — operator may veto.
- D6 Synology fidelity: real vdsm only, or also own lightweight FileStation/Auth shim (no open-source mock exists).
- D7 S3 emulator: Garage (AGPL, no versioning, already chosen per memory) vs SeaweedFS (Apache-2.0); AGPL test-infra acceptable?
- D8 Approve landing the Containers branch `fix/containers-g1-g2-g11` upstream (prerequisite NE-1).
