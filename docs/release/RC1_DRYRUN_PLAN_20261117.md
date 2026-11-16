# rc.1 dry-run plan (non-gate emulator pre-checks)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-11-17 |
| Status | PROVISIONAL plan; nothing in it was run. Every result it yields is DRY-RUN, never gate evidence |
| Basis | scratch clone of `main` `5242d0617`; read-only inspection of the main checkout's `submodules/containers` (not initialised in the clone) |

## Rules

1. ALL of this is a dry run. By operator decision 2026-11-17 emulator evidence is never gate evidence: it cannot make any 11.4.298 gate record RELEASABLE, cannot satisfy G2, and per-product records stay `BLOCKED-NO-HARDWARE` / `BLOCKED-NO-BASELINE`. Label every output directory and report "DRY-RUN, NOT GATE EVIDENCE".
2. Operator NOTE 2026-11-17 (relayed): every emulator runs through the Containers submodule (`submodules/containers`, 11.4.76) in rootless Podman containers (11.4.161), QEMU or another option that submodule offers; never a bare-host emulator launch (no `emulator -avd`, no host `qemu-system-*`).
3. Host load is high (measured 2026-11-17: 16 CPUs, load average about 48, 30 GiB RAM, 18 GiB available). Run one job at a time, bounded (`--cpus`, `--memory`, `timeout`), `nice`/`ionice`; the submodule's `hostbudget.Admit` refuses a start that does not fit (README "Host budget"). Never `pkill -f`. Never print credentials. The vendor name must not appear in outputs.

## What the Containers submodule offers (read from `submodules/containers/README.md` and `CLAUDE.md`, not executed)

- `pkg/emulator` + `cmd/emulator-matrix`: Android AVD boot and test matrix; the Linux x86_64 runner is `containerized` with KVM (`/dev/kvm`); flags include `--runner containerized`, `--container-runtime podman`, `--container-image`, `--android-sdk-root`, `--apk`, `--avds`, `--cold-boot`, `--boot-timeout`, `--evidence-dir`, `--concurrent`.
- `images/android-test` (Dockerfile, entrypoint, AVD configs `config_api28..36.ini`); `pkg/cuttlefish` (needs a rootful privileged container, so NOT usable under 11.4.161 here); `pkg/vm` + `cmd/vm-matrix` (QEMU full-system VMs); `pkg/crossbuild` (container-bound builds); `pkg/hostbudget` (admission with `RequireKVM`); `pkg/runtime` run options refuse `--privileged` and host namespaces and allow typed `--device`.
- Host facts measured: `podman` present, `rootless: true`; `/dev/kvm` read-write for this user through an ACL (the user is not in the `kvm` group; the ACL grants `rw`). The Android SDK is at `/home/milosvasic/Android/Sdk` with only `system-images/android-34` (see `ls`); `adb` and `avdmanager` exist; no `emulator`/`xvfb-run`/`cargo`/`rustc` on PATH (`/snap/bin/chromium` and `npm`/`go` exist).
- UNCONFIRMED: that `/dev/kvm` passes into a rootless container on this host (`--device /dev/kvm` under rootless podman with an ACL-only grant; the container user must map to a UID that keeps the ACL); that the `images/android-test` image is already built (`podman images` listed only other images in its first rows); that API 34 is a usable TV image (the `android-34` directory holds `android-tv` and `default`, so a TV image appears present; its tag/ABI was not read); that `emulator-matrix` runs end to end against the current APKs; Cuttlefish and QEMU board-level fidelity for the project's real hardware (none: an emulator is never the target hardware).

## Per product

| Product | Dry-run coverage | Container route | Evidence it yields (dry-run only) | Cost (estimate, UNCONFIRMED) |
|---|---|---|---|---|
| api | Local binary live run of `catalog-api` against a local DB and the project's test-infra compose files (`docker-compose.test-infra*.yml`) | `podman` rootless via the submodule's `pkg/compose`/runtime; build in a container (11.4.173); run the binary in a `podman run --cpus 2 --memory 2g` container | HTTP live-suite verdicts, response ids, binary sha256 | Go build 3-6 min at 2 CPUs; run 2-5 min; about 2 GiB RAM |
| web | Playwright `catalog-web/e2e/live` (the same suite as `docs/qa/baseline-v2.4.2-20261114/`) against the local stack with headless Chromium | Playwright image (or Chromium) in a rootless podman container on the compose network, `--shm-size 1g --cpus 2 --memory 3g` | per-test PASS/FAIL set, comparable with the v2.4.2 baseline rows as a hint only | 5-15 min; about 3 GiB RAM |
| desktop (Tauri) | Build and launch smoke only | Build in a rootless container with the Rust/Tauri toolchain (`pkg/crossbuild`); launch under Xvfb INSIDE the container; no `cargo` on the host, so the toolchain image must be provided or built (UNCONFIRMED whether one exists) | build log, binary sha256, a launch screenshot | Rust build 15-40 min at 2-4 CPUs, 4-6 GiB RAM: HEAVY, run last and alone |
| wizard (installer) | Same as desktop | Same container route | same | same |
| Android | AVD boot + install + instrumented test class | `emulator-matrix --runner containerized --container-runtime podman --container-image <android-test image> --android-sdk-root /home/milosvasic/Android/Sdk --apk <debug apk> --avds <AVD:34:phone> --cold-boot --evidence-dir docs/qa/dryrun-android-<date>/ --concurrent 1` | boot attestation rows, test report, screenshots | image build (if missing) 10-20 min; boot 2-5 min with KVM; 4 GiB RAM, 2-4 CPUs; ABI x86_64 only |
| Android TV | Same, with the TV system image | Same command with a TV AVD (`:34:tv`) | same | same; image availability UNCONFIRMED |

Notes: the Android APK build itself also runs in a container (11.4.173; `docs/qa/containerized-build-20260630/`). Exact flag spelling comes from `cmd/emulator-matrix/main.go`; run it with `--help` first (cheap, read-only). Neither desktop nor wizard has a dry-run path today that does not need a Rust toolchain image: that gap is real and stays listed.

## Suggested order and bounds

1. `emulator-matrix --help`, `podman images`, `podman run --rm --device /dev/kvm <image> ls -l /dev/kvm` (cheap KVM-in-rootless probe; answers the main UNCONFIRMED).
2. api, then web (cheap). 3. Android (one AVD). 4. Android TV. 5. desktop and wizard builds only when the host load average is under about 16.
Each step: `timeout`, a resource cap, one at a time, evidence dir named `docs/qa/dryrun-<product>-<date>/` with a `SHA256SUMS` and a README line "DRY-RUN, NOT GATE EVIDENCE". Any admission refusal (`ErrHostBudget`) is recorded as `does_not_fit` and the step is skipped, not forced.

## Honest boundary

A dry run can find defects early and cut the owed live work; it proves nothing about real hardware, and a green dry run never changes a gate state, a changelog VERIFIED entry, or the rc.1 labelling. v2.5.0 still needs the per-product live run on connected hardware (11.4.298).
