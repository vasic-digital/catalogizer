# Windows and macOS test targets as containerized QEMU VMs: research

| Field | Value |
|---|---|
| Revision | 1 (read-only web research; nothing pulled, booted or installed) |
| Retrieval date | 2026-11-13 (all web facts below) |
| Context | docs/research/device_emulation_plan_20261112.md (host: Ryzen 2700X, 30 GiB RAM, load ~26/16 threads, 58 GiB free disk, /dev/kvm usable rootless via ACL, no host qemu) |
| Labels | FACT = read on the cited page today; UNCONFIRMED = third-party / memory / not verified; not legal advice |

## 1. Sources and findings

### dockur/windows (https://github.com/dockur/windows, fetched 2026-11-13)
- FACT: repo MIT-licensed; "Windows inside a Docker container"; Docker or Podman on Linux with KVM.
- FACT: needs >= 2 GB RAM, 32 GB free disk (defaults: 4 GB RAM, 2 cores, 64 GB disk via RAM_SIZE/CPU_CORES/DISK_SIZE).
- FACT: example passes `/dev/kvm`, `/dev/net/tun`, cap NET_ADMIN, a storage volume; ports 8006 (web viewer), 3389 (RDP). Optional DHCP mode needs /dev/vhost-net. Troubleshooting suggests `privileged` only to rule out device problems.
- FACT: versions: Win 11/10 (Pro, LTSC, Enterprise), 8.1 Ent, server 2003-2025, legacy, Tiny11/Tiny10, ReactOS. ARM64 is a separate project (dockur/windows-arm).
- FACT: Windows is downloaded automatically at first start (the page does not name the source), or a custom ISO via URL in VERSION or mounted `/custom.iso`.
- FACT: automation: `/oem` folder copied to C:\OEM, `install.bat` runs in the final install step; `COMMAND` runs one command. Default user `Docker` / password `admin` (USERNAME/PASSWORD env). Page does NOT mention SSH or WinRM (UNCONFIRMED whether OpenSSH can be enabled via install.bat; likely, untested).
- FACT: page says the user must hold a valid Windows licence; keys in code are generic Microsoft trial/install keys, not activation.
- FACT: no rootless-specific guidance on the page. UNCONFIRMED: rootless podman needs `--userns=keep-id` style handling and tun/NET_ADMIN under rootless (the Containers submodule already handles kvm ACL at pkg/emulator/containerized.go:93-102 per the plan doc).

### dockur/macos (https://github.com/dockur/macos, fetched 2026-11-13)
- FACT: Docker/Podman on Linux with KVM; AVX2 CPU (Zen 1000+ ok, so 2700X ok); >= 4 GB RAM, 32 GB disk (defaults 4 GB, 2 cores, 64 GB). On AMD: do not assign > 8 GB RAM during install.
- FACT: macOS 11 to 15 (default 15); 26 (Tahoe) selectable, "very slow, not recommended".
- FACT: web viewer 8006; 5900 mapped. No SSH mentioned. Licence: page links license.md but content not shown, licence name UNCONFIRMED (repo says it contains only open-source code and does not distribute macOS).
- FACT (legal): FAQ states installing macOS needs accepting Apple's EULA, which does not permit installation on non-official hardware, so run it only on Apple hardware.

### OSX-KVM (https://github.com/kholia/OSX-KVM, fetched 2026-11-13)
- FACT: needs QEMU >= 8.2.2, AVX2 for Ventura+, `fetch-macOS-v2.py` downloads BaseSystem (Apple servers) then dmg2img, 256 GB qcow2; `boot-macOS-headless.sh` exists. SSH/VNC "out of the box" is an open contributor item. Licence not stated on the page (UNCONFIRMED). Page places the EULA decision on the user and cites OSK-string court document.

### Quickemu (https://github.com/quickemu-project/quickemu, fetched 2026-11-13)
- FACT: MIT; `quickget` downloads upstream OS images; Windows 10/11/Server and macOS (Mojave..Sequoia); automatic SSH port forwarding, QEMU guest agent via QMP; no elevated permissions. A host tool (needs host qemu), not a container; adds little over dockur for our Containers-submodule model.

### Microsoft evaluation / dev VMs
- FACT (search result, Microsoft Evaluation Center pages): Windows 11 Enterprise evaluation is a 90-day full-featured evaluation; lab kits state VMs expire 90 days after provisioning and the lab must not be connected to production. https://www.microsoft.com/evalcenter/download-windows-11-enterprise , https://www.microsoft.com/evalcenter/evaluate-lab-kit
- FACT (search result): developer VMs at https://developer.microsoft.com/downloads/virtual-machines are Enterprise Evaluation, no key activation, x86 only, formats Hyper-V/VirtualBox/VMware/Parallels (no QEMU/qcow2; convertible, UNCONFIRMED); 8 GB RAM / 70 GB disk recommended; refreshed periodically, old builds expire (an Oct 2024 expiry reported). Whether WinDev images still exist in 2026: UNCONFIRMED (no 2025/26 source found).
- UNCONFIRMED (third party only): `slmgr /rearm` extension to 270 days; do not rely on or encode it.
- UNCONFIRMED: exact EULA text of the evaluation (I did not read the licence PDF); the evaluation is intended for evaluation/testing, not production. Operator must read the terms at download.

### Apple macOS licence
- FACT (secondary sources, Apple SLA text quoted on blogs/Apple dev forums; primary Tahoe SLA not read): Section 2B allows up to two additional macOS instances in virtual environments on each Mac you own or control that already runs macOS, for software development/testing, macOS Server or personal non-commercial use; Apple-branded hardware only. Apple dev forum: Virtualization.framework caps at 2 VMs; more needs written permission. https://developer.apple.com/forums/thread/826102 , https://threedots.ovh/blog/2020/12/macos-eula-licensing-restrictions-affecting-virtualisation/
- Conclusion (reading, UNCONFIRMED legally): macOS in QEMU on this AMD Linux host is outside the licence. Both dockur/macos and OSX-KVM say so or defer to the user.

## 2. Feasibility on this host

| Target | Technical | Legal | Disk/RAM | Verdict |
|---|---|---|---|---|
| Windows 10/11 guest (dockur/windows, QEMU/KVM in rootless podman) | Plausible: /dev/kvm ACL works rootless (plan doc); needs /dev/net/tun (present), NET_ADMIN (rootless netns ok for pasta/user-net, UNCONFIRMED). Untested here. | Only with an operator-supplied/accepted licence: Microsoft eval (90 days, re-provision) or owned licence. dockur auto-download source unnamed; prefer operator-supplied ISO, sha256 recorded | 64 GB disk default (host has 58 GiB free, 97% used): BLOCKED until prune. 4 GB RAM ok but CPU load 26/16 makes boots slow | FEASIBLE after disk is freed and licence choice made; heavy |
| Windows guest running Tauri installer | NSIS `/S` and MSI `/qn` silent flags (third-party skill pages, UNCONFIRMED vs Tauri docs); WebView2 runtime must be present (Win11 ships it; Win10 may need offline installer ~127 MB per v1 docs) | as above | | feasible |
| Windows UI driving | RDP 3389 / noVNC 8006 + screenshots/OCR satisfies §11.4.193 see-before-type; tauri-driver WebDriver on Windows needs msedgedriver (memory, UNCONFIRMED) | | | feasible, slow |
| macOS guest on this host | dockur/macos or OSX-KVM technically boot on Zen (AVX2 ok); Tahoe very slow; no SSH out of box; OSX-KVM needs host qemu | NOT permitted by Apple SLA on non-Apple hardware (reading); dockur itself says Apple hardware only | 64 GB+ disk, 4-8 GB RAM | MUST remain honest SKIP |
| Tauri macOS build/run | tauri-driver has no macOS WKWebView driver (memory, UNCONFIRMED) | as above | | SKIP in all cases on this host |

## 3. Determinism and pinning
- Image: pin container image by digest (`@sha256:`), never `:latest` (dockur tags move). UNCONFIRMED: dockur downloads the Windows ISO at first boot, so the digest of the container does not pin the guest OS: pin by supplying `/custom.iso` with recorded sha256 (Microsoft eval ISOs are re-issued and expire).
- Boot a copy-on-write overlay per run (qemu `-snapshot` or qcow2 backing file) so the golden disk is immutable and run N>=3 starts identical (§11.4.50). Golden install is done once and recorded (sha256 of disk, ISO sha256, Windows build `ver`, WebView2 version).
- Evidence fields (§11.4.289(D)): ISO sha256, container digest, guest build id, Tauri artifact sha256, endpoint reached (API IP over virtual bridge), screenshots with OCR.
- 90-day eval expiry makes a golden image a decaying asset: record expiry date; a run on an expired image is SKIP, not PASS.
- §11.4.298(B): an emulator is never "this run" for hardware products; Windows/macOS desktop are software targets, so a VM can be LIVE-class for "Tauri on Windows", but must not be labelled real-hardware verification.

## 4. Risks
1. Disk: 58 GiB free cannot hold a 64 GB Windows disk plus ISO (~5-6 GB) (blocker until operator prunes).
2. CPU: host load 26/16; Windows first install under TCG-free KVM still takes tens of minutes; memory PSI and §12.6 ceiling must be checked before each boot.
3. Licensing: eval expiry/EULA; Apple SLA hard no.
4. Rootless: NET_ADMIN/tun/vhost-net behaviour untested; `privileged` is excluded by operator constraint (rootless, no privileged).
5. dockur auto-download is opaque (source unnamed on the page); supply ISO instead.
6. Default creds `Docker/admin` must be overridden via a 0600 env file (§11.4.10), not committed.
7. Windows tauri-driver/msedgedriver availability unverified.
8. Containers submodule has no Windows/macOS VM runtime (pkg/vm/macos is Tart, Apple-Silicon host only; Wine is build-only) so new upstream code (typed device/memory/cpus run options, a dockur-style VM profile) is needed per §11.4.74.

## 5. Recommendation
- Windows: implement a `windows` VM profile in the Containers submodule (pinned dockur/windows digest, operator-supplied ISO with sha256, immutable golden disk + per-run overlay, tun/kvm devices, memory/cpus/pids limits, 0600 credential env file, /oem install.bat that enables OpenSSH + installs the Tauri NSIS package silently), gated on free disk >= ~100 GB, memory PSI check, and a recorded licence decision. Start with one Win11 Enterprise eval guest. Drive UI via RDP/noVNC screenshots + OCR. Do this after the Android/Linux legs (cheaper).
- macOS: do NOT build. Record `topology_unsupported` (reason: Apple SLA permits macOS virtualization only on Apple-branded hardware; host is AMD x86) for every macOS leg; revisit only if the operator has a Mac (then Tart path in pkg/vm/macos).
- No Wine substitute may be reported as Windows.

## 6. Operator decisions needed
1. Free disk (prune) to allow >= ~100 GB for a Windows guest? Which sibling data may be removed?
2. Windows licence basis: Microsoft 90-day evaluation (re-provision each cycle) vs an owned licence/ISO; who downloads the ISO and accepts the terms.
3. Confirm macOS stays SKIP (`topology_unsupported`), or supply Apple hardware.
4. Accept dockur/windows as an external dependency (MIT, pinned by digest) vs building our own QEMU image in the Containers submodule (§11.4.74/§11.4.270 existence-verdict register entry needed).
5. Accept slow serial runs (Windows boot minutes, CPU contention), or defer Windows until the host is quieter.
6. Confirm rootless-only (no privileged, no root NET_ADMIN beyond userns) is binding for these VMs.
