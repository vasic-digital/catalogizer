# Device-leg emulation plan (Catalogizer + Containers submodule)

| Field | Value |
|---|---|
| Revision | 1 (draft, research + plan only) |
| Created | 2026-11-12 |
| Proposed repo path | `docs/research/device_emulation_plan_20261112.md` (NOT written to the main checkout) |
| Status | PROPOSAL, needs independent review (§11.4.142 / §11.4.209) and operator sight |
| Operator decision cited | 2026-11-12 (relayed): no TV, phone or second machine connected; fan out emulators of all kinds for device legs via the Containers submodule and extend it upstream (§11.4.74, §11.4.76, §11.4.161) |
| Author | research subagent (Opus); no code, no image pull, no emulator started |

## Table of contents

1. Measured host facts
2. Inventory: what the Containers submodule already does
3. Catalogizer device legs: what emulation can and cannot prove
4. Gap list for the Containers submodule (API, tests, size, host-safety)
5. Ordered work plan (Sonnet authors, Opus review)
6. Findings outside scope (pin hygiene) and UNCONFIRMED list

All `file:line` references are into `constitution/submodules/containers` at commit `5ea870a`
(equal to `origin/main`: `git ls-remote origin main` = `5ea870a5837097077abcf04caab29550dca2fa5f`,
measured 2026-11-12). The flat copy `submodules/containers` is at `22b4970`, an ancestor 9 commits behind
(`git merge-base --is-ancestor` true, `git log 22b4970..5ea870a | wc -l` = 9). Upstream work starts from `5ea870a`.

## 1. Measured host facts (2026-11-12 ~23:48 CET, read-only probes)

| Fact | Value | Command |
|---|---|---|
| CPU | AMD Ryzen 7 2700X, 16 threads, `svm` present (16 hits) | `/proc/cpuinfo`, `nproc` |
| KVM | `/dev/kvm` `crw-rw----+ root kvm`, ACL `user:milosvasic:rw-`; readable and writable by this user; module `kvm_amd`, nested = 1 | `ls -l`, `getfacl`, `test -r/-w`, `/sys/module/kvm_amd/parameters/nested` |
| RAM | 30 GiB total, ~17 GiB available; memory PSI some avg10 = 1.08 | `free -g`, `/proc/pressure/memory` |
| Load | 26.4 / 26.7 / 25.4 on 16 threads | `uptime` |
| Disk | `/` 1.8T, 97% used, 58 GiB free | `df -h` |
| Process limit | `ulimit -u` = 123694 | |
| Rootless podman | 5.7.0, rootless = true, backend netavark, pasta present; 45 containers running; rootless bridge networks exist (`podman network ls`) | `podman info`, `podman ps -q \| wc -l` |
| `/dev/net/tun` | present | |
| Binder (redroid) | `/dev/binder*` and binderfs ABSENT; `binder` not in `/proc/filesystems` | `ls`, `grep` |
| Host Android SDK | `~/Android/Sdk` with emulator 37.1.11, `system-images;android-34;android-tv;x86` (8.2 GiB, TagId android-tv, rev 3) and `system-images;android-34;default;x86_64` (1.7 GiB); `~/.android/avd` empty | `ls`, `source.properties`, `du -sh` |
| Host tools | `adb` (SDK), `scrcpy` (`/usr/bin/scrcpy`), `smbd` (`/sbin/smbd`), `rclone` present; `qemu-system-x86_64`, `qemu-img`, `swtpm`, `Xvfb`, `xvfb-run`, `weston` ABSENT | `command -v` |
| Local images | 53 `localhost/*` images; ZERO matching `tauri\|android` (control: same pipeline lists `localhost/catqa3-*`); `dxflrs/garage:v1.0.1`, `atmoz/sftp`, `alpine:3.20`, `debian:bookworm-slim`, `debian:trixie-slim` present | `podman images` |

Consequences: KVM works rootless for this user (named-user ACL, which is exactly the case
`pkg/emulator/containerized.go:78-95` handles with `--userns=keep-id`). CPU is the scarce resource
(load 26 on 16 threads); RAM allows at most two Android AVDs at a time inside the §12.6 60% ceiling
(18 GiB) with the other sessions' footprint. Disk (58 GiB) rules out Windows images and new multi-GB
emulator images until the operator prunes. The Tauri QA image `catalogizer-tauri:qa` is no longer on the
host and must be rebuilt before any Tauri live run.

## 2. Inventory: what the Containers submodule already does

| Capability | Evidence | Notes |
|---|---|---|
| Android emulator orchestration, two runners (host-direct, containerized) | `pkg/emulator/android.go:151` (`AndroidEmulator`), `pkg/emulator/containerized.go:286` (`Containerized`), `:427` Boot, `:540` WaitForBoot, `:791` Install, `:815` RunInstrumentation, `:883` Teardown | `Emulator` interface `pkg/emulator/types.go:345` |
| Runner policy per OS | `pkg/emulator/accel.go:126` `AccelProfileForOS`, `:184` `ResolveRunner`, `:219` `GateEligibleForOS` (gate-eligible only when runner == OS profile runner) | On Linux the gate-eligible runner is `containerized`; the host-direct run Catalogizer did in `docs/qa/android-boot-20261105` is NOT gate-eligible by this rule |
| Form factor field incl. `tv` | `pkg/emulator/types.go:31-44` (`AVD.FormFactor` "phone", "tablet", "tv", ...) | Metadata only: no TV-specific code path (grep `"tv"\|leanback\|tv_1080p` in non-test emulator code: 0 hits apart from the comment) |
| System-image cache incl. `android-tv` URL parsing | `pkg/emulator/android.go:274` `ensureSystemImageViaCache`, `:344` `parseSystemImageURL` (example `sys-img/android-tv/x86-30_r4.zip`, `:337`) | |
| KVM passthrough + rootless KVM fix | `pkg/emulator/containerized.go:37` `kvmAvailable` (`os.Stat` only), `:93-102` `--userns=keep-id` + `--device /dev/kvm` | Stat does not prove the device is openable; `pkg/vm/qemu.go:163-164` opens `/dev/kvm` O_RDWR instead (the stronger check) |
| Argv injection guard | `pkg/emulator/containerized.go:111-126` (`--` before image) | |
| ADB hygiene, phantom TCP, boot diagnostics | `pkg/emulator/adb_hygiene.go:60` `ParseADBDevicesLine`, `:134` `ResetADBHygiene`, `:223` `CaptureBootDiagnostic` | |
| Zombie reaping by real cmdline | `pkg/emulator/cleanup.go:75` `Cleanup`, `:234` `KillByPort`, `pkg/emulator/cleanup_os.go:42-64` (`/proc` walker) | Matches §11.4.196(D) discipline |
| Network shaping (edge..ethernet) | `pkg/emulator/network.go:34`, `:93` `applyNetworkConditions`; `pkg/vm/network.go` (duplicate table) | Emulator-console shaping only, not a second network host |
| Screenshot | `pkg/emulator/screenshot.go:24` `CaptureScreenshot`; `pkg/vm/screenshot.go` | No screen recording: `screenrecord` 0 files, `scrcpy` 0 files (control `screencap` = 3 files) |
| Release cold-start canary | `pkg/emulator/canary.go:139` `RunCanary` (activity RESUMED + logcat FATAL-free), `cmd/emulator-canary` | Launch via `am start`; no leanback launcher intent |
| Matrix runner + attestation | `pkg/emulator/matrix.go:525` `RunMatrix`, `:622` `writeAttestation`, `cmd/emulator-matrix` | |
| Baked emulator image | `pkg/emulator/Containerfile` (ARGs `API_LEVEL`, `ABI`, `TARGET_TYPE`, lines 36-39); `images/android-test/Dockerfile` (google_apis x86_64 API 28-36) + `images/android-test/avds/config_api*.ini` (`tag.id=google_apis`) | No TV AVD config shipped; image base `ghcr.io/vasic-digital/android-emulator-base:latest` (mutable tag) not present locally |
| QEMU Linux VMs (alpine/debian/fedora, x86_64/aarch64/riscv64) | `pkg/vm/qemu.go:189` Boot (`-m 2048 -smp 2`, user-net with SSH hostfwd `:201`, `-snapshot` on cold boot `:221`), `pkg/vm/types.go:25` `VMTarget`, `pkg/vm/clients.go` (real SSH/QMP), `cmd/vm-matrix` | Launches `qemu-system-*` ON THE HOST (`pkg/vm/qemu.go:173`); the host has no QEMU, so this path cannot run here today |
| Image cache with sha256 manifest | `pkg/cache/manifest.go:28` `ImageEntry` (sha256 + size REQUIRED) | |
| Cuttlefish | `pkg/cuttlefish/types.go:132-176`: requires ROOTFUL + `--privileged` + `--network host` | Excluded by the operator constraints (rootless, no privileged) |
| macOS/iOS | `pkg/vm/macos/macos.go:1-29` (Tart, Apple-Silicon macOS host only), `pkg/vm/ios`, `pkg/applesim` | Not usable on this Linux host |
| Genymotion | `pkg/genymotion/genymotion.go` | Commercial/cloud; not considered |
| Windows | `pkg/crossbuild/windows_wine.go` + `windows_wine.Containerfile` (Wine + JDK + Gradle) | BUILD backend only; no Windows runtime/VM target (grep `windows` in `pkg/vm/*.go` non-test: 0 hits) |
| Generic run primitive | `pkg/runtime/run.go:77-88` `runOptions` (Name, Entrypoint, WorkDir, User, Network, Env, Volumes, ExtraArgs), With* options `:119-187` | No typed memory / cpus / pids / device / userns / publish / label / digest options; this is Catalogizer's recorded deviation (b) in `scripts/containers/run_pinned.sh:44-45` |
| Remote distribution, SSH tunnels, overlay | `pkg/network/overlay.go:14-173` (`TunnelOverlay` over SSH), `pkg/network/tunnel.go`, `pkg/volume/nfs.go`, `sshfs.go`, `rsync.go`, `pkg/remote` | Overlay is SSH-tunnel based (needs a real remote host); no rootless virtual-LAN-peer primitive |
| Protocol-server fixtures (SMB/FTP/WebDAV/NFS/S3) | none: `grep -rli 'samba\|smbd\|webdav\|vsftpd\|ftpd\|minio\|garage'` over Go/Dockerfile/yml found only `pkg/policy/policy.go` (an unrelated mention) | Catalogizer keeps its own `docs/qa/test-servers/up.sh` (`cattsrv-*`) |
| Desktop GUI runtime (Xvfb/VNC/WebKit) | none in `pkg`/`cmd`/`images` (only `xvfb` as a build dep in `pkg/crossbuild/windows_wine.Containerfile:41`) | Catalogizer keeps its own `scripts/tauri/run-qa.sh` (Xvfb + openbox + x11vnc + noVNC, `--memory=2G --cpus=2 --pids-limit=1024`) |
| Resource limits on emulator containers | none: `grep -rni 'memory\|cpus'` over `pkg/emulator`, `pkg/cuttlefish`, `pkg/vm` non-test = 0 hits (control `kvm` = 33 hits in `containerized.go`) | Host-safety gap (§12.6) |

## 3. Catalogizer device legs: what emulation can and cannot prove

Sources: `docs/CONTINUATION.md` Rev 20 (lines 16, 26-27, 37, 67, 87-91, 98-100) and
`docs/FOLLOW_UPS_NEXT_PHASE.md` (lines 1503-1520 Android/Tauri, 1425/1578/1614 second LAN host,
605/1245/1384 Synology, 1617-1699 Tauri wizard).

Rule applied. Under §11.4.289 an emulator running the REAL artifact (the real APK, the real Tauri binary)
against the REAL API is LIVE-class evidence for claims scoped to the emulated target; it is not a test
double. Two limits bind every row below:

* The record must carry the §11.4.289(D) fields: artifact sha256, AVD/system-image identity
  (`ro.build.fingerprint`, system-image `source.properties`), emulator version, container image digest,
  endpoint actually reached.
* §11.4.298(B), verbatim: "An emulator, a simulator, a host-only run, a replayed or copied record, or a run
  on hardware of another product is never this run." So no emulated run makes a product `RELEASABLE`
  under the release gate; Android, Android TV and any hardware-bound product stays `BLOCKED-NO-HARDWARE`
  for deploy/tag purposes until real hardware is connected. Emulation raises PROVISIONAL items to
  VERIFIED only for the emulated-target claim, and the item text must say which target.

| Leg (item ids) | Emulated target | Claim an emulated run CAN support | Claim it CANNOT support (stays SKIP `hardware_not_present` / open) | Verdict |
|---|---|---|---|---|
| Tauri desktop, Linux (CAT-TAURI-NATIVE-1, TAURI-FU-7) | real Linux binary in WebKitGTK under Xvfb in a rootless container | first-run URL dialog, confirm-on-URL-change, login, SSRF refusal, close-X, all driven see-before-type (§11.4.193) with OCR/screenshots, N>=3 | GPU compositing (WEBKIT compositing disabled), Wayland, HiDPI, real desktop environments, tray/notifications on real DEs | COVERABLE (needs image rebuild) |
| Tauri installer wizard (CAT-TAURI-INSTALLER-1, TAURI-R1-FU-7) | same | GREEN run of the Save Configuration sequence on the rebuilt wizard, same target as the RED | install onto a real user session, OS integration (menu entries, file associations) | COVERABLE |
| Tauri Windows build | QEMU Windows guest (not implemented) | only with an operator-supplied, licensed Windows image recorded as DATA; WebView2 runtime behaviour | anything without that image; Wine does NOT count (WebView2 under Wine UNCONFIRMED, and Wine is not Windows) | SKIP until operator provides image + disk; reason `topology_unsupported` |
| Tauri macOS build | none on this host | none | everything: macOS on non-Apple hardware is not permitted by Apple's licence (operator to confirm the reading); `pkg/vm/macos` needs an Apple-Silicon macOS host | SKIP (structurally impossible on this host) |
| Android phone (CAT-ANDROID-LOGIN-1, AND-FU-3/4/6) | AVD `pixel_6`, API 34 `default;x86_64` (image on host), KVM | install, launch, server-URL entry, real qa_admin login via secret-safe input, browse/search/play against the real API, AND-FU-6 "Connected" on unreachable server reproduction | ARM-only native code paths, real touch latency, radios/Wi-Fi roaming, battery, OEM skins, HW codecs, real performance figures, release gate | COVERABLE for the emulated-target claim |
| Android TV (AND-FU-2, TV player items) | AVD `tv_1080p`, API 34 `android-tv;x86` (image on host, 8.2 GiB) | leanback launch, D-pad navigation/focus, login, browse, playback with software decode | HDMI/CEC, HW decode (HEVC/HDR/Dolby), real remote (IR/BT), TV OEM firmware, 4K output timing | COVERABLE for the emulated-target claim |
| Display access from a second machine (AND-FU-4) | n/a | none: needs a second machine by definition | the SSH-tunnel + scrcpy path from another host | SKIP `hardware_not_present` |
| "Anton" SMB second machine (CONTINUATION 87-91: "Anton.local IS this host") and bare-IP SMB | rootless netavark bridge with a peer container (own netns, own IP) running a real Samba; optional containerized QEMU Linux guest for a second kernel | client works against an SMB server at a different IP over a virtual L2 link (not loopback, not the API's own netns), bare-IP addressing, port 445 vs non-445, cut-recovery by stopping the peer, N>=3 | a physical NIC, switch, router, MTU/Wi-Fi loss, a Windows SMB server, Synology's Samba build | COVERABLE as "virtual LAN peer"; physical second-host claim stays SKIP |
| NFS / FTP / FTPS / SFTP / WebDAV / S3 on a second host | same peer pattern: userspace NFS (rclone/ganesha), vsftpd or pure-ftpd, atmoz/sftp, a WebDAV server, Garage (image on host) | protocol client behaviour at a non-local IP | kernel-NFS server behaviour (needs root), client-side NFS kernel mount (needs root; the earlier probe used sudo) | COVERABLE for userspace servers; kernel-NFS SKIP |
| TRUSTED_PROXIES / rate-limit from a second LAN host (STK3-FU-1, QA-MON-FU-4, TP-FU-3) | peer container on a separate bridge sending through the nginx front door | API sees a distinct client IP arriving over a veth/bridge path, XFF handling with a non-primary source | "real remote NIC" (stays UNCONFIRMED, as the 2026-11-06 substitute run already states) | PARTIAL, label precisely |
| Synology SMB/FTP/NFS/WebDAV, idle-deadline, cut-recovery (CAT-SYN-LIVE-1, ST-NFS, LV3-2) | none | none: generic Samba/FTP fixtures prove nothing about the Synology firmware; DSM emulation (XPEnology-class) is not pursued (DSM licence is tied to Synology hardware; operator to confirm) | all Synology claims | SKIP `hardware_not_present` (drives OFF since 2026-11-05) |
| Real phone / real TV hardware | none | none | every hardware claim and the §11.4.298 release gate | SKIP `hardware_not_present` |

Already proven once (do not re-claim): both debug APKs built in a rootless JDK 17 container and the phone
and TV AVDs booted headless with KVM on the host (`docs/qa/android-boot-20261105/README.md`), via the
host-direct runner and project-local commands, not via the Containers submodule.

## 4. Gap list for the Containers submodule

Each gap: proposed API, tests written first (RED captured on the current code, §11.4.224 / §11.4.292),
paired mutants, size (non-test LOC + test LOC, estimate), host-safety notes. All new code is project-agnostic
(§11.4.28): Catalogizer names, ports and IPs are consumer DATA.

### G1. Typed resource and device options in the run primitive (pkg/runtime)

* API: `WithRunMemory(bytes)`, `WithRunMemorySwap(bytes)` (default = memory, i.e. no swap),
  `WithRunCPUs(float)`, `WithRunPidsLimit(n)`, `WithRunDevice(path)` (validated: must exist, must be in an
  allowlist the caller passes; `/dev/kvm` opened O_RDWR first), `WithRunUserns("keep-id")` (podman only;
  docker returns a typed error), `WithRunPublish(hostIP, hostPort, ctrPort, proto)` (hostIP REQUIRED,
  `0.0.0.0` refused unless `WithRunPublishAllInterfaces()` is explicit), `WithRunLabel(k,v)`,
  `WithRunImageDigest(ref)` (refuses a mutable tag when digest pinning is requested). Never `--privileged`,
  never `--network host`: both refused at the builder with a typed error.
* Tests first: arg-builder table tests (each option present/absent), refusal tests (privileged, network host,
  0.0.0.0 publish, docker + userns), a podman-real test gated on podman presence that runs `alpine:3.20`
  (already on host) with `--memory` and reads `/sys/fs/cgroup/memory.max` inside to prove the limit is real.
* Mutants (>=4): drop the memory flag; drop the `--` before image; allow `--privileged`; treat a tag as digest.
* Size: ~250 + ~400 LOC. Unblocks Catalogizer deviation (b) (`scripts/containers/run_pinned.sh:44`).

### G2. Host-safety admission + emulator envelope (pkg/emulator, shared helper in pkg/policy or new pkg/hostbudget)

* API: `hostbudget.Admit(req Need) (Verdict, Evidence)` with `Need{MemBytes, CPUs, Pids, NeedsKVM}`;
  verdict closed set `fits | does_not_fit | undetermined` (mirrors §11.4.291 vocabulary); inputs
  `/proc/meminfo` MemAvailable/MemTotal, `/proc/pressure/memory` and `/proc/pressure/cpu`, loadavg vs nproc,
  `ulimit -u`, `/dev/kvm` O_RDWR probe (read-only intent: open+close, no ioctl). Reserve and thresholds are
  consumer DATA; defaults = §12.6 60% ceiling, `MemAvailable - need >= max(4 GiB, 15% MemTotal)`.
  `ContainerizedConfig` gains `Memory`, `CPUs`, `PidsLimit` (host-adaptive defaults; emulator default
  memory = AVD RAM + 1.5 GiB overhead, CPUs = min(3, 0.25*nproc)); `Boot` refuses on `does_not_fit` or
  `undetermined` with a typed error and writes the evidence JSON. Replace `kvmAvailable` stat
  (`pkg/emulator/containerized.go:37`) with the O_RDWR probe from `pkg/vm/qemu.go:163`.
* Tests first: fake `/proc` fixtures (golden-good fits, golden-bad does_not_fit, unreadable PSI ->
  undetermined, carrier: high load but low memory need is still judged by both axes); KVM probe with a temp
  char-device stand-in is not possible, so test the error-mapping seam plus one real probe on hosts with
  `/dev/kvm`.
* Mutants: stat instead of open; ignore PSI; treat undetermined as fits; drop `--memory` from run args.
* Size: ~300 + ~500 LOC.

### G3. Host-SDK-mount containerized emulator (no multi-GB image pull)

* Problem: the gate-eligible Linux runner needs an emulator image; the base `android-emulator-base:latest`
  is absent and disk is at 97%. The host already has the SDK, emulator 37.1.11 and both API 34 system images.
* API: `ContainerizedConfig.SDKMount{HostPath, ReadOnly: true}` and `AVDHome` (scratch dir, rw): run a slim
  image (Debian bookworm-slim, already on host, plus the emulator's runtime libs: libpulse0, libnss3,
  libxcomposite1, libxcursor1, libxi6, libxtst6, libasound2, socat) with the host SDK bind-mounted read-only
  at `/opt/android-sdk`, AVDs created by `avdmanager` inside the container into the scratch AVD home.
  New `images/android-emulator-slim/Containerfile`; image built locally (one ~150 MB layer, no SDK inside).
  Record the system-image `source.properties` and `ro.build.fingerprint` in `BootResult`.
* Tests first: arg-builder tests (mount ro, AVD home rw, no write mount on the SDK), avdmanager command builder
  for `-d pixel_6 -k "system-images;android-34;default;x86_64"` and `-d tv_1080p -k
  "system-images;android-34;android-tv;x86"`; live boot test gated on KVM + host SDK + admission `fits`.
* Mutants: rw SDK mount; wrong `-k` tag for TV; skip fingerprint capture; omit `--userns=keep-id`.
* Size: ~350 + ~450 LOC + Containerfile ~40 lines.

### G4. Android TV first-class (pkg/emulator)

* API: `FormFactorTV` constant; `TVProfile{Device: "tv_1080p", Tag: "android-tv", ABI: "x86"}`; canary
  option `LaunchCategory: "android.intent.category.LEANBACK_LAUNCHER"`; D-pad helpers `DPad(ctx, keys...)`
  mapping to `input keyevent KEYCODE_DPAD_UP/DOWN/LEFT/RIGHT/CENTER`, `KEYCODE_BACK`, `KEYCODE_HOME`;
  `FocusedNode()` via `uiautomator dump` and a parser that returns the focused node (see-before-type,
  §11.4.193). `images/android-test/avds/config_api34_tv.ini`.
* Tests first: keyevent argv tables, uiautomator XML fixtures (golden focused node, no focused node ->
  error, carrier: `focusable=true` but not `focused`), canary intent builder for leanback.
* Mutants: phone launcher category for TV; DPAD_CENTER mapped to ENTER; parser accepting `focusable`.
* Size: ~300 + ~450 LOC.

### G5. Recording and frame evidence (pkg/emulator, pkg/desktopgui)

* API: `StartRecording(ctx, serial, maxDur) (Recorder, error)` using `adb shell screenrecord --time-limit`
  (180 s cap per segment, rotate) with pull + sha256; optional scrcpy backend when present on host
  (`scrcpy --no-window --record`); `CaptureFrame` returns PNG + sha256 + timestamp + build fingerprint.
* Tests first: argv builders, segment rotation, sha256 file writing, failure on 0-byte file (FALSE-NULL guard).
* Mutants: accept empty file; skip rotation; drop time limit; wrong serial flag.
* Size: ~250 + ~350 LOC.

### G6. Secret-safe text input (pkg/emulator)

* Problem: AND-FU-3, `adb shell input text <password>` exposes the secret in the HOST process table.
* API: `TypeSecret(ctx, serial, secretFile string)`: pushes the 0600 file to `/data/local/tmp/<random>`
  via `adb push` (path in argv, value not), runs `adb shell 'input text "$(cat F)"; rm -f F'` with the
  command string free of the value, then verifies with `uiautomator dump` that the field is non-empty and
  masked. Secret never logged; evidence redacts.
* Tests first: argv never contains the secret (property test over random secrets), file removed on success
  and on failure path, refusal when the file is not 0600.
* Honest limit: the value is briefly in the device's own shell process; acceptable for an emulator
  (UNCONFIRMED whether the operator accepts it).
* Mutants: secret in argv; no rm; mode check skipped; no post-type verification.
* Size: ~150 + ~250 LOC.

### G7. Desktop GUI runtime for Linux apps (new pkg/desktopgui)

* API: `desktopgui.Session{Image, AppPath(ro), Screen, BindIP(default 127.0.0.1), VNCPasswordFile(0600),
  Memory, CPUs, PidsLimit}` with `Start`, `Screenshot` (xwd -> PNG + sha256), `OCR` hook (consumer-supplied),
  `Type`/`Click` via xdotool (see-before-type: screenshot before and after), `WebDriver()` endpoint when
  `tauri-driver` + `WebKitWebDriver` are in the image, `Stop`. New `images/desktopgui-webkit/Containerfile`
  (Debian bookworm-slim + Xvfb + openbox + x11vnc + novnc + websockify + xdotool + libwebkit2gtk-4.1).
  Port of the logic in Catalogizer `scripts/tauri/run-qa.sh` (Catalogizer then calls this package; its
  script becomes a thin wrapper).
* Tests first: arg builder (ro app mount, loopback bind default, LAN bind needs explicit flag, no host
  network), password-file mode check, screenshot non-blank check (golden blank frame must FAIL).
* Mutants: bind 0.0.0.0 by default; accept blank screenshot; rw app mount; missing `--pids-limit`.
* Size: ~500 + ~600 LOC + Containerfile ~40 lines.

### G8. Virtual LAN peer + protocol-server fixtures (extend pkg/network, new pkg/fixtures)

* API: `network.CreateBridge(name, subnet)` (rootless netavark, internal by default),
  `fixtures.Peer{Network, IP, Services []Service}` with typed services `SMB{Share, Path, Users from 0600
  file}`, `NFSUser{Export, RO}` (userspace), `FTP{Explicit/Implicit TLS}`, `SFTP`, `WebDAV`, `S3Garage`;
  each with a protocol-level health check (SMB: `smbclient -L` or a Go SMB negotiate; NFS: MOUNT/NULL
  RPC; FTP: banner + login; WebDAV: PROPFIND 207; S3: signed ListBuckets) and a fault API `Cut()` /
  `Restore()` (disconnect the peer from the bridge) for cut-recovery tests. Corpus mounted read-only
  except a declared `rw/` subdir. Ports: inside the bridge the real ports (445, 2049, 21) are used; no host
  publish unless asked.
* Tests first: arg builders, health checks against fixtures that answer wrongly (golden-bad: FTP banner but
  login refused must not be "healthy"), cut/restore state machine, real-podman test on alpine.
* Mutants: health = TCP connect only; publish on 0.0.0.0; corpus rw; Cut() that does not detach.
* Size: ~800 + ~1000 LOC + 4 small Containerfiles (Samba, vsftpd, WebDAV, NFS-userspace; reuse
  `atmoz/sftp` and `dxflrs/garage:v1.0.1` already on host).
* Honest limit: proves "different IP over a virtual L2 link", never a physical second machine.

### G9. Containerized QEMU runner (pkg/vm)

* Problem: `pkg/vm/qemu.go:173` launches `qemu-system-*` on the host; the host has none.
* API: `vm.NewContainerizedQEMU(cfg{Image, Memory, CPUs, KVM})` implementing the existing `VM` interface
  (`pkg/vm/types.go:179`); `--device /dev/kvm`, `--userns=keep-id`, qcow2 base read-only + `-snapshot`,
  networking either user-net with SSH hostfwd (existing) or `-netdev tap` inside the container attached to a
  G8 bridge (container has `/dev/net/tun` via `--device`, no privilege). `-m`/`-smp` from config instead of
  the hardcoded `2048`/`2` (`pkg/vm/qemu.go:196-197`).
* Tests first: arg builder parity with host runner, refusal without admission, live boot of
  `alpine-3.20-x86_64` gated on KVM (image fetched only through `pkg/cache` with sha256; not now: disk).
* Mutants: hardcoded memory; missing `-snapshot`; rw base image; tap without tun device.
* Size: ~400 + ~500 LOC + Containerfile.

### G10. Windows guest target (pkg/vm, later)

* API: `VMTarget{Distro: "windows", ...}` + OVMF + swtpm (TPM 2.0) inside the G9 container; image ONLY from an
  operator-supplied path with licence DATA (`licence: evaluation|retail`, expiry); never auto-download; boot
  readiness via WinRM or OpenSSH-for-Windows.
* Blocked now: disk (Windows image 20-30 GiB vs 58 GiB free), licence decision (operator), host load.
* Size: ~600 + ~600 LOC. Record SKIP reason `topology_unsupported` until the operator supplies an image.

### G11. Reachable-endpoint resolver (pkg/emulator, pkg/desktopgui)

* Problem: inside a rootless pasta container the host LAN IP resolves to the container
  (`scripts/tauri/run-qa.sh:10-12`); inside the Android emulator the host is `10.0.2.2`. Every run must
  record which URL it actually used.
* API: `ReachableAPI(target, hostPort) (url, evidence)` returning `http://10.0.2.2:<p>` (emulator),
  `http://host.containers.internal:<p>` (container), or a G8 bridge IP, with a probe from inside the target
  (curl/`adb shell` `toybox nc`) recorded as §11.4.289(D) endpoint identity.
* Size: ~150 + ~250 LOC.

### G12. Device leases for emulated devices (reuse §11.4.119 / §11.4.176 pattern)

* API: lease keyed by stable AVD name / container name, all-or-nothing, TTL-reaped by `kill -0` + cmdline, so
  parallel agents never drive one emulator. Size ~200 + ~300 LOC. May reuse an existing lease engine in the
  constitution (UNCONFIRMED whether one is importable by reference; check before writing).

Host-safety limits for every gap (binding): rootless only; `--privileged`, `--network host`, Cuttlefish and
redroid (needs binder kernel module, absent) excluded; `/dev/kvm` touched only by open+close probe and by
`--device` passthrough; every container carries `--memory`, `--memory-swap` = memory, `--cpus`,
`--pids-limit`; whole runs wrapped in `systemd-run --user --scope -p MemoryMax=… -p MemorySwapMax=0`;
at most 2 AVDs concurrently on this host (2 x ~4.5 GiB); no run while load > ~12 (CONTINUATION
dispatch rule); no image pull until the operator approves disk use; no nested privileged containers; no
emulator bound to 0.0.0.0 (adb/console unauthenticated).

## 5. Ordered work plan

Each step: one Sonnet author in a scratch clone of `vasic-digital/Containers` at `5ea870a` (fast-forward
only, §11.4.288 vehicle rules), test-first, >=4 own mutants, then Opus `xhigh` review (§11.4.209),
budget 5 rounds (§11.4.276(G)). Steps on one line run in parallel (disjoint files).

| Order | Steps | Depends on | Can run under current load/disk? |
|---|---|---|---|
| 1 | G1 (pkg/runtime) ‖ G2 (hostbudget + emulator envelope) ‖ G11 (resolver) | none | YES: unit + alpine-only real tests |
| 2 | G3 (SDK-mount slim emulator) ‖ G4 (TV) ‖ G6 (secret input) | G1, G2 | code YES; live boot only when load < ~12 |
| 3 | G5 (recording) ‖ G7 (desktopgui) | G1, G2 | code YES; image build needs base already on host (bookworm-slim) plus apt layer (network, ~300 MB) |
| 4 | G8 (virtual LAN peer + fixtures) | G1 | code YES; Samba/vsftpd images need apt layers |
| 5 | G9 (containerized QEMU) ‖ G12 (leases) | G1, G2 | code YES; live VM needs a qcow2 via pkg/cache |
| 6 | Catalogizer adoption: bump `submodules/containers` + constitution pin to the new tag; `scripts/tauri/run-qa.sh`, `docs/qa/android-boot-*` commands and `docs/qa/test-servers/up.sh` become thin wrappers; remove deviation (b) | 1-5 landed upstream | YES |
| 7 | Live legs, N>=3 each: Android phone login (AND-FU-3, AND-FU-6 repro), Android TV D-pad + playback, Tauri desktop GREEN, wizard GREEN (TAURI-R1-FU-7), SMB/NFS/FTP/WebDAV/S3 at a peer IP, bare-IP SMB, TRUSTED_PROXIES from peer | 6 | only when load < ~12 and disk freed |
| 8 | G10 (Windows) | operator: image + licence + disk | NO (blocked) |

Size total (estimate, non-test + test): ~4.4k + ~5.6k LOC plus 7 small Containerfiles; steps 1-5 ~ 12-16
author dispatches including review rounds.

Operator decisions needed before step 2 live runs: (a) disk: approve pruning or a target for the ~300 MB-
1 GB of new image layers; (b) whether the in-device secret exposure of G6 is acceptable; (c) Windows
image + licence (G10) or permanent SKIP; (d) confirm the macOS and Synology-DSM licence readings above.

## 6. Findings outside scope and UNCONFIRMED list

Findings (not fixed here, for registration as follow-ups):

* F1. `helix-deps.yaml:93` records `ref: bfd0e98` for `containers`, but the gitlink is `22b4970`
  (`git ls-tree HEAD submodules/containers`); `bfd0e98` is an ancestor (2 commits). Manifest drift
  (§11.4.31 / §11.4.233(G)).
* F2. Two checkouts of the same repo at different commits: `submodules/containers` `22b4970` and
  `constitution/submodules/containers` `5ea870a`. Decide which one Catalogizer code builds against
  (§11.4.28(C) one source of truth).
* F3. `pkg/emulator/containerized.go:37` judges KVM by `os.Stat`; `pkg/vm/qemu.go:163` already uses the
  stronger O_RDWR open. Inconsistent guard (§11.4.201). Covered by G2.
* F4. No resource limits on emulator/QEMU containers (control-needled 0 hits). Covered by G1/G2.
* F5. Catalogizer's Android boot and Tauri QA runs use project-local commands, not the Containers
  submodule (§11.4.74 / §11.4.76 deviation). Covered by step 6.
* F6. `images/android-test/Dockerfile` builds FROM a mutable tag `android-emulator-base:latest`
  (§11.4.264 digest pinning).

UNCONFIRMED:

* Exact host RAM footprint of a running TV AVD under container + Xvfb-free headless mode on this host
  (earlier run used `MemoryMax=5G` per emulator; not re-measured).
* Whether the host emulator 37.1.11 binary runs inside Debian bookworm-slim with only the listed libs
  (G3 assumption; needs the first RED/GREEN).
* Whether rootless netavark allows `--device /dev/net/tun` + tap inside a container for QEMU (G9).
* WebView2 behaviour under Wine; legal reading of macOS and Synology DSM licences (operator).
* Whether a reusable lease engine exists in the constitution for G12.
* Whether upstream `vasic-digital/Containers` has unpushed work on other mirrors (only `origin` checked).
