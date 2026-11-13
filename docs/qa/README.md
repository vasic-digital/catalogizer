# Qa index

Documents in this directory.

Back to the documentation index: [docs/README.md](../README.md). Descriptions are copied mechanically from each file (its first heading and first prose line) unless overridden in scripts/docs/index_description_overrides.tsv; non-Markdown artefacts are listed by name and type without being read. 1377 entries.

**Host LAN address (CAT-LANIP-DYN-1).** Live QA scripts no longer hardcode the host's LAN IP (it changes with DHCP/reboots). They source [lib/lan_ip.sh](lib/lan_ip.sh): the address is auto-detected (`ip -4 route get 1.1.1.1` source address, validated as IPv4 and as locally assigned) and can be overridden with `QA_LAN_IP`; per-script variables (`SONAR_BIND`, `S3_BIND=lan`, `WEB_URL`, `TS_HOST`) still win. If the address cannot be resolved the scripts fail loudly. Addresses written in dated evidence packs and in the index rows below are historical and are not rewritten.

| File | Description |
|---|---|
| [CATALOGIZER_QA.md](CATALOGIZER_QA.md) | Catalogizer AI QA System Integration Guide - The Catalogizer AI QA System is now fully integrated into your Catalogizer project, providing comprehensive zero-defect quality assurance for all components. This system... |
| [EVIDENCE_WAIVERS.txt](EVIDENCE_WAIVERS.txt) | (plain-text artefact; not read, listed by name only) |
| [QA_TESTING_GUIDE.md](QA_TESTING_GUIDE.md) | Catalogizer QA Testing Guide - ![Test Status](https://img.shields.io/badge/tests-automated-brightgreen) |
| [android-boot-20261105/README.md](android-boot-20261105/README.md) | Android QA boot, 2026-11-05 (Roadmap-Item CAT-AND-1) - Outputs: <module>/app/build/outputs/apk/debug/app-debug.apk, sha256 in apk-sha256.txt. |
| [android-boot-20261105/apk-sha256.txt](android-boot-20261105/apk-sha256.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/avd-create.log](android-boot-20261105/avd-create.log) | (file; not read, listed by name only) |
| [android-boot-20261105/avd-list.txt](android-boot-20261105/avd-list.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/boot-phone.txt](android-boot-20261105/boot-phone.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/boot-tv.txt](android-boot-20261105/boot-tv.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/build-android-RED-jdk21-target.log](android-boot-20261105/build-android-RED-jdk21-target.log) | (file; not read, listed by name only) |
| [android-boot-20261105/build-android.log](android-boot-20261105/build-android.log) | (file; not read, listed by name only) |
| [android-boot-20261105/build-androidtv-RED-google-services-missing.log](android-boot-20261105/build-androidtv-RED-google-services-missing.log) | (file; not read, listed by name only) |
| [android-boot-20261105/build-androidtv.log](android-boot-20261105/build-androidtv.log) | (file; not read, listed by name only) |
| [android-boot-20261105/build_apk.sh](android-boot-20261105/build_apk.sh) | (shell script; not read, listed by name only) |
| [android-boot-20261105/disk-consumed.txt](android-boot-20261105/disk-consumed.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/emulator-phone.log](android-boot-20261105/emulator-phone.log) | (file; not read, listed by name only) |
| [android-boot-20261105/emulator-tv.log](android-boot-20261105/emulator-tv.log) | (file; not read, listed by name only) |
| [android-boot-20261105/image-build.log](android-boot-20261105/image-build.log) | (file; not read, listed by name only) |
| [android-boot-20261105/logcat-phone-launch.txt](android-boot-20261105/logcat-phone-launch.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/logcat-tv-launch.txt](android-boot-20261105/logcat-tv-launch.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/phone-after-connect.png](android-boot-20261105/phone-after-connect.png) | (PNG image; not read, listed by name only) |
| [android-boot-20261105/phone-after-dialog.png](android-boot-20261105/phone-after-dialog.png) | (PNG image; not read, listed by name only) |
| [android-boot-20261105/phone-connected.png](android-boot-20261105/phone-connected.png) | (PNG image; not read, listed by name only) |
| [android-boot-20261105/phone-launch.png](android-boot-20261105/phone-launch.png) | (PNG image; not read, listed by name only) |
| [android-boot-20261105/phone-urlentered.png](android-boot-20261105/phone-urlentered.png) | (PNG image; not read, listed by name only) |
| [android-boot-20261105/sdk-install-images.log](android-boot-20261105/sdk-install-images.log) | (file; not read, listed by name only) |
| [android-boot-20261105/sdk-list-images.txt](android-boot-20261105/sdk-list-images.txt) | (plain-text artefact; not read, listed by name only) |
| [android-boot-20261105/tv-launch.png](android-boot-20261105/tv-launch.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/06_after_select.png](androidtv-players-20260629/06_after_select.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/11_playing_position.png](androidtv-players-20260629/11_playing_position.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/12_after_pause.png](androidtv-players-20260629/12_after_pause.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/28_comic_detail.png](androidtv-players-20260629/28_comic_detail.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/29_comic_reader.png](androidtv-players-20260629/29_comic_reader.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/32_comic_page_render.png](androidtv-players-20260629/32_comic_page_render.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/PROOF_covers_render_captain_america_ironman.png](androidtv-players-20260629/PROOF_covers_render_captain_america_ironman.png) | (PNG image; not read, listed by name only) |
| [androidtv-players-20260629/Status.html](androidtv-players-20260629/Status.html) | (HTML export; not read, listed by name only) |
| [androidtv-players-20260629/Status.md](androidtv-players-20260629/Status.md) | Android TV Player Validation — Findings - (AV liveness battery) + §11.4.117 (pixel-oracle) + §11.4.143 (real-user journey) + |
| [androidtv-players-20260629/Status.pdf](androidtv-players-20260629/Status.pdf) | (PDF export; not read, listed by name only) |
| [androidtv-players-20260629/decoder_metrics.txt](androidtv-players-20260629/decoder_metrics.txt) | (plain-text artefact; not read, listed by name only) |
| [androidtv-players-20260629/exoplayer_diag.txt](androidtv-players-20260629/exoplayer_diag.txt) | (plain-text artefact; not read, listed by name only) |
| [androidtv-players-20260629/player_crash_log.txt](androidtv-players-20260629/player_crash_log.txt) | (plain-text artefact; not read, listed by name only) |
| [anton-smb-20261106/README.md](anton-smb-20261106/README.md) | Anton SMB live target (CAT-SYN-LIVE-1 substitute) - Target: SMB server on host Anton.local (= this machine, 192.168.1.115, Samba 4.23.6) - real SMB, but self-hosted loopback-class, NOT an external server. |
| [anton-smb-20261106/download_run1.sha256](anton-smb-20261106/download_run1.sha256) | (file; not read, listed by name only) |
| [anton-smb-20261106/download_run2.sha256](anton-smb-20261106/download_run2.sha256) | (file; not read, listed by name only) |
| [anton-smb-20261106/download_run3.sha256](anton-smb-20261106/download_run3.sha256) | (file; not read, listed by name only) |
| [anton-smb-20261106/manifest.sha256](anton-smb-20261106/manifest.sha256) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/README.html](anton-smb-n3-20261112/README.html) | (HTML export; not read, listed by name only) |
| [anton-smb-n3-20261112/README.md](anton-smb-n3-20261112/README.md) | Anton Public/Testing SMB live runs N=3 (CAT-SMB-SUBDIR-1) - PROVISIONAL/VERIFIED see below - Command (per docs/qa/smb-subdir-20261110/run.sh, minus systemd-run scope): GOMAXPROCS=2 nice -n 10 go test -count=1 -tags live_smb -run... |
| [anton-smb-n3-20261112/README.pdf](anton-smb-n3-20261112/README.pdf) | (PDF export; not read, listed by name only) |
| [anton-smb-n3-20261112/SHA256SUMS](anton-smb-n3-20261112/SHA256SUMS) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run1.log](anton-smb-n3-20261112/run1.log) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run1.meta](anton-smb-n3-20261112/run1.meta) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run1.normhash](anton-smb-n3-20261112/run1.normhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run1.rawhash](anton-smb-n3-20261112/run1.rawhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run2.log](anton-smb-n3-20261112/run2.log) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run2.meta](anton-smb-n3-20261112/run2.meta) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run2.normhash](anton-smb-n3-20261112/run2.normhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run2.rawhash](anton-smb-n3-20261112/run2.rawhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run3.log](anton-smb-n3-20261112/run3.log) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run3.meta](anton-smb-n3-20261112/run3.meta) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run3.normhash](anton-smb-n3-20261112/run3.normhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/run3.rawhash](anton-smb-n3-20261112/run3.rawhash) | (file; not read, listed by name only) |
| [anton-smb-n3-20261112/submodule_pins.txt](anton-smb-n3-20261112/submodule_pins.txt) | (plain-text artefact; not read, listed by name only) |
| [api-contract-live-20261103/go-test.txt](api-contract-live-20261103/go-test.txt) | (plain-text artefact; not read, listed by name only) |
| [api-contract-live-20261103/nil-slice-red/go-test.txt](api-contract-live-20261103/nil-slice-red/go-test.txt) | (plain-text artefact; not read, listed by name only) |
| [api-contract-live-20261103/nil-slice-red/operations.json](api-contract-live-20261103/nil-slice-red/operations.json) | (JSON data artefact; not read, listed by name only) |
| [api-contract-live-20261103/nil-slice-red/summary.json](api-contract-live-20261103/nil-slice-red/summary.json) | (JSON data artefact; not read, listed by name only) |
| [api-contract-live-20261103/operations.json](api-contract-live-20261103/operations.json) | (JSON data artefact; not read, listed by name only) |
| [api-contract-live-20261103/summary.json](api-contract-live-20261103/summary.json) | (JSON data artefact; not read, listed by name only) |
| [containerized-build-20260630/Status.html](containerized-build-20260630/Status.html) | (HTML export; not read, listed by name only) |
| [containerized-build-20260630/Status.md](containerized-build-20260630/Status.md) | §11.4.173 Containerized + Distributed Build — PROOF (first increment) - (thinker.local), artifact brought back; never on the bare host. |
| [containerized-build-20260630/build_proof.txt](containerized-build-20260630/build_proof.txt) | (plain-text artefact; not read, listed by name only) |
| [conversion-defects-fix-20261104/MANIFEST.sha256](conversion-defects-fix-20261104/MANIFEST.sha256) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/README.md](conversion-defects-fix-20261104/README.md) | Conversion defects fix, review round 1 follow-up (2026-11-04) - Base: 9bba3746 (round 0); round-1 follow-up bd9d2ada; round-2 (N1) commit follows bd9d2ada. Manifest regenerated 2026-11-04T23:54:11+01:00. This change: see git log for... |
| [conversion-defects-fix-20261104/green-handlers-repository-race-full.log](conversion-defects-fix-20261104/green-handlers-repository-race-full.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/green-merged-conversion-subtitle-race.log](conversion-defects-fix-20261104/green-merged-conversion-subtitle-race.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/green-root-package.log](conversion-defects-fix-20261104/green-root-package.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/green-services-tests-full.log](conversion-defects-fix-20261104/green-services-tests-full.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/mutations.log](conversion-defects-fix-20261104/mutations.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/red-handlers.log](conversion-defects-fix-20261104/red-handlers.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/red-n1-services.log](conversion-defects-fix-20261104/red-n1-services.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/red-r3-services.log](conversion-defects-fix-20261104/red-r3-services.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/red-root-conversion-tools.log](conversion-defects-fix-20261104/red-root-conversion-tools.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/red-services.log](conversion-defects-fix-20261104/red-services.log) | (file; not read, listed by name only) |
| [conversion-defects-fix-20261104/vet.log](conversion-defects-fix-20261104/vet.log) | (file; not read, listed by name only) |
| [cover-images-rootcause-20260629/Status.html](cover-images-rootcause-20260629/Status.html) | (HTML export; not read, listed by name only) |
| [cover-images-rootcause-20260629/Status.md](cover-images-rootcause-20260629/Status.md) | Catalog Cover Images Not Loading — Root Cause Analysis - Every catalog item on Android TV (and all clients) shows a placeholder gradient + |
| [cover-images-rootcause-20260629/Status.pdf](cover-images-rootcause-20260629/Status.pdf) | (PDF export; not read, listed by name only) |
| [coverart/.gitignore](coverart/.gitignore) | (file; not read, listed by name only) |
| [coverart/README.md](coverart/README.md) | Cover-art fetch live checks (CAT-COVERART-LIVE-1) - Closes the tooling gap in docs/PROVISIONAL_VERIFICATION_PLAN.md section 2.3: no script drove the |
| [coverart/fixtures/bad_fail.txt](coverart/fixtures/bad_fail.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/fixtures/bad_missing.txt](coverart/fixtures/bad_missing.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/fixtures/good.txt](coverart/fixtures/good.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/mutation.sh](coverart/mutation.sh) | (shell script; not read, listed by name only) |
| [coverart/results/final/SHA256SUMS.txt](coverart/results/final/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline.console.txt](coverart/results/final/baseline.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline/ENV.txt](coverart/results/final/baseline/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline/SHA256SUMS.txt](coverart/results/final/baseline/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline/go_test.json.txt](coverart/results/final/baseline/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline/go_test.txt](coverart/results/final/baseline/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/baseline/verdict.txt](coverart/results/final/baseline/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap.console.txt](coverart/results/final/mutant-cap.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/ENV.txt](coverart/results/final/mutant-cap/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/SHA256SUMS.txt](coverart/results/final/mutant-cap/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/go_test.json.txt](coverart/results/final/mutant-cap/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/go_test.txt](coverart/results/final/mutant-cap/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/mutation.diff.txt](coverart/results/final/mutant-cap/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-cap/verdict.txt](coverart/results/final/mutant-cap/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1.console.txt](coverart/results/final/mutant-m1.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/ENV.txt](coverart/results/final/mutant-m1/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/SHA256SUMS.txt](coverart/results/final/mutant-m1/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/go_test.json.txt](coverart/results/final/mutant-m1/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/go_test.txt](coverart/results/final/mutant-m1/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/mutation.diff.txt](coverart/results/final/mutant-m1/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m1/verdict.txt](coverart/results/final/mutant-m1/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2.console.txt](coverart/results/final/mutant-m2.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/ENV.txt](coverart/results/final/mutant-m2/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/SHA256SUMS.txt](coverart/results/final/mutant-m2/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/go_test.json.txt](coverart/results/final/mutant-m2/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/go_test.txt](coverart/results/final/mutant-m2/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/mutation.diff.txt](coverart/results/final/mutant-m2/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m2/verdict.txt](coverart/results/final/mutant-m2/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5.console.txt](coverart/results/final/mutant-m5.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/ENV.txt](coverart/results/final/mutant-m5/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/SHA256SUMS.txt](coverart/results/final/mutant-m5/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/go_test.json.txt](coverart/results/final/mutant-m5/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/go_test.txt](coverart/results/final/mutant-m5/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/mutation.diff.txt](coverart/results/final/mutant-m5/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m5/verdict.txt](coverart/results/final/mutant-m5/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7.console.txt](coverart/results/final/mutant-m7.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/ENV.txt](coverart/results/final/mutant-m7/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/SHA256SUMS.txt](coverart/results/final/mutant-m7/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/go_test.json.txt](coverart/results/final/mutant-m7/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/go_test.txt](coverart/results/final/mutant-m7/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/mutation.diff.txt](coverart/results/final/mutant-m7/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m7/verdict.txt](coverart/results/final/mutant-m7/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8.console.txt](coverart/results/final/mutant-m8.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/ENV.txt](coverart/results/final/mutant-m8/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/SHA256SUMS.txt](coverart/results/final/mutant-m8/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/go_test.json.txt](coverart/results/final/mutant-m8/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/go_test.txt](coverart/results/final/mutant-m8/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/mutation.diff.txt](coverart/results/final/mutant-m8/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-m8/verdict.txt](coverart/results/final/mutant-m8/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1.console.txt](coverart/results/final/mutant-n1.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/ENV.txt](coverart/results/final/mutant-n1/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/SHA256SUMS.txt](coverart/results/final/mutant-n1/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/go_test.json.txt](coverart/results/final/mutant-n1/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/go_test.txt](coverart/results/final/mutant-n1/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/mutation.diff.txt](coverart/results/final/mutant-n1/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-n1/verdict.txt](coverart/results/final/mutant-n1/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard.console.txt](coverart/results/final/mutant-netguard.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/ENV.txt](coverart/results/final/mutant-netguard/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/SHA256SUMS.txt](coverart/results/final/mutant-netguard/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/go_test.json.txt](coverart/results/final/mutant-netguard/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/go_test.txt](coverart/results/final/mutant-netguard/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/mutation.diff.txt](coverart/results/final/mutant-netguard/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-netguard/verdict.txt](coverart/results/final/mutant-netguard/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict.console.txt](coverart/results/final/mutant-strict.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/ENV.txt](coverart/results/final/mutant-strict/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/SHA256SUMS.txt](coverart/results/final/mutant-strict/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/go_test.json.txt](coverart/results/final/mutant-strict/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/go_test.txt](coverart/results/final/mutant-strict/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/mutation.diff.txt](coverart/results/final/mutant-strict/mutation.diff.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/mutant-strict/verdict.txt](coverart/results/final/mutant-strict/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race.console.txt](coverart/results/final/race.console.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race/ENV.txt](coverart/results/final/race/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race/SHA256SUMS.txt](coverart/results/final/race/SHA256SUMS.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race/go_test.json.txt](coverart/results/final/race/go_test.json.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race/go_test.txt](coverart/results/final/race/go_test.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/race/verdict.txt](coverart/results/final/race/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/results/final/summary.txt](coverart/results/final/summary.txt) | (plain-text artefact; not read, listed by name only) |
| [coverart/run.sh](coverart/run.sh) | (shell script; not read, listed by name only) |
| [coverart/test_run.sh](coverart/test_run.sh) | (shell script; not read, listed by name only) |
| [crashlytics-wiring-20260629/Status.md](crashlytics-wiring-20260629/Status.md) | Android TV Crashlytics Wiring — QA Evidence - Queried Firebase Crashlytics for app 1:881377664729:android:751a0d0e2d873db47768c8 over the |
| [crashlytics-wiring-20260629/app_init_markers.txt](crashlytics-wiring-20260629/app_init_markers.txt) | (plain-text artefact; not read, listed by name only) |
| [crashlytics-wiring-20260629/crashlytics_enabled_init.txt](crashlytics-wiring-20260629/crashlytics_enabled_init.txt) | (plain-text artefact; not read, listed by name only) |
| [crashlytics-wiring-20260629/crashlytics_prefs.xml](crashlytics-wiring-20260629/crashlytics_prefs.xml) | (file; not read, listed by name only) |
| [crashlytics-wiring-20260629/firebase_full_init.txt](crashlytics-wiring-20260629/firebase_full_init.txt) | (plain-text artefact; not read, listed by name only) |
| [crashlytics-wiring-20260629/firebase_tags.txt](crashlytics-wiring-20260629/firebase_tags.txt) | (plain-text artefact; not read, listed by name only) |
| [crashlytics-wiring-20260629/launch_logcat.txt](crashlytics-wiring-20260629/launch_logcat.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/MANIFEST](d11-d7-fix-20261105/MANIFEST) | (file; not read, listed by name only) |
| [d11-d7-fix-20261105/README.md](d11-d7-fix-20261105/README.md) | D11a / D11b / D7 fix evidence (2026-11-05) - PROVISIONAL-DONE - Branch fix-d11-d7. Each defect has red_*.txt (pre-fix), green_*.txt (post-fix), mutations_*.txt (reviewer-style mutations, all killed). |
| [d11-d7-fix-20261105/baseline_openapi_contract_test.txt](d11-d7-fix-20261105/baseline_openapi_contract_test.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/green_d11a.txt](d11-d7-fix-20261105/green_d11a.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/green_d11b.txt](d11-d7-fix-20261105/green_d11b.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/green_d7.txt](d11-d7-fix-20261105/green_d7.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/mutations_d11a.txt](d11-d7-fix-20261105/mutations_d11a.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/mutations_d11b.txt](d11-d7-fix-20261105/mutations_d11b.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/mutations_d7.txt](d11-d7-fix-20261105/mutations_d7.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/mutations_round1.txt](d11-d7-fix-20261105/mutations_round1.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/red_d11a.txt](d11-d7-fix-20261105/red_d11a.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/red_d11a_ffmpeg_raw.txt](d11-d7-fix-20261105/red_d11a_ffmpeg_raw.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/red_d11b.txt](d11-d7-fix-20261105/red_d11b.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/red_d7.txt](d11-d7-fix-20261105/red_d7.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/red_round1_f1f2.txt](d11-d7-fix-20261105/red_round1_f1f2.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/root_package_tests.txt](d11-d7-fix-20261105/root_package_tests.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/round1_package_tests.txt](d11-d7-fix-20261105/round1_package_tests.txt) | (plain-text artefact; not read, listed by name only) |
| [d11-d7-fix-20261105/round1_root_package_tests.txt](d11-d7-fix-20261105/round1_root_package_tests.txt) | (plain-text artefact; not read, listed by name only) |
| [d3-pdf-pages-fix-20261104/MANIFEST.txt](d3-pdf-pages-fix-20261104/MANIFEST.txt) | (plain-text artefact; not read, listed by name only) |
| [d3-pdf-pages-fix-20261104/MUTATIONS.txt](d3-pdf-pages-fix-20261104/MUTATIONS.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/MANIFEST.sha256](d4-doc-search-fix-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [d4-doc-search-fix-20261105/README.txt](d4-doc-search-fix-20261105/README.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/green_after_fix.txt](d4-doc-search-fix-20261105/green_after_fix.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/green_round1.txt](d4-doc-search-fix-20261105/green_round1.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/green_round2.txt](d4-doc-search-fix-20261105/green_round2.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/green_round3.txt](d4-doc-search-fix-20261105/green_round3.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/mutations.txt](d4-doc-search-fix-20261105/mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/mutations_round1.txt](d4-doc-search-fix-20261105/mutations_round1.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/mutations_round2.txt](d4-doc-search-fix-20261105/mutations_round2.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/mutations_round3.txt](d4-doc-search-fix-20261105/mutations_round3.txt) | (plain-text artefact; not read, listed by name only) |
| [d4-doc-search-fix-20261105/red_before_fix.txt](d4-doc-search-fix-20261105/red_before_fix.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/MANIFEST.md](d9-admin-role-fix-20261104/MANIFEST.md) | D9 admin role fix - evidence manifest (round 1 update) - f8646d344c4c15a3f049a47767343f71b764a4322718425269f9697ad7310728 green_d9.txt |
| [d9-admin-role-fix-20261104/green_d9.txt](d9-admin-role-fix-20261104/green_d9.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/green_d9_race.txt](d9-admin-role-fix-20261104/green_d9_race.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/green_full.txt](d9-admin-role-fix-20261104/green_full.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/green_repository.txt](d9-admin-role-fix-20261104/green_repository.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/green_round1_race.txt](d9-admin-role-fix-20261104/green_round1_race.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M1_role_not_applied.txt](d9-admin-role-fix-20261104/mut_M1_role_not_applied.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M2_self_demotion_allowed.txt](d9-admin-role-fix-20261104/mut_M2_self_demotion_allowed.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M3_no_session_revoke_on_role_change.txt](d9-admin-role-fix-20261104/mut_M3_no_session_revoke_on_role_change.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M4_list_default_user.txt](d9-admin-role-fix-20261104/mut_M4_list_default_user.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M5_unknown_role_accepted.txt](d9-admin-role-fix-20261104/mut_M5_unknown_role_accepted.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_M6_self_and_last_admin_checks_removed.txt](d9-admin-role-fix-20261104/mut_M6_self_and_last_admin_checks_removed.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R10_delete_guard_disabled.txt](d9-admin-role-fix-20261104/mut_R10_delete_guard_disabled.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R3_update_instead_of_updateguarded.txt](d9-admin-role-fix-20261104/mut_R3_update_instead_of_updateguarded.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R4a_count_inactive_admins.txt](d9-admin-role-fix-20261104/mut_R4a_count_inactive_admins.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R4b_count_locked_admins.txt](d9-admin-role-fix-20261104/mut_R4b_count_locked_admins.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R5_audit_event_renamed.txt](d9-admin-role-fix-20261104/mut_R5_audit_event_renamed.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R6_first_revoke_ignores_role.txt](d9-admin-role-fix-20261104/mut_R6_first_revoke_ignores_role.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R7_no_tolower_in_role_names.txt](d9-admin-role-fix-20261104/mut_R7_no_tolower_in_role_names.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R8_revoke_keyed_on_diff_only.txt](d9-admin-role-fix-20261104/mut_R8_revoke_keyed_on_diff_only.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mut_R9_update_guard_disabled.txt](d9-admin-role-fix-20261104/mut_R9_update_guard_disabled.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mutate.py](d9-admin-role-fix-20261104/mutate.py) | (Python script; not read, listed by name only) |
| [d9-admin-role-fix-20261104/mutate_round1.py](d9-admin-role-fix-20261104/mutate_round1.py) | (Python script; not read, listed by name only) |
| [d9-admin-role-fix-20261104/probe_suspend.txt](d9-admin-role-fix-20261104/probe_suspend.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/red.txt](d9-admin-role-fix-20261104/red.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/red_round1_race.txt](d9-admin-role-fix-20261104/red_round1_race.txt) | (plain-text artefact; not read, listed by name only) |
| [d9-admin-role-fix-20261104/vet.txt](d9-admin-role-fix-20261104/vet.txt) | (plain-text artefact; not read, listed by name only) |
| [device_qa_androidtv_20260626.md](device_qa_androidtv_20260626.md) | On-Device QA - Catalogizer Android TV (MIBOX4) - 2026-06-26: runtime-signature results for DEFECT-G, DEFECT-F and the OpenDesign palette on a clean debug APK |
| [docs_sync_c4.md](docs_sync_c4.md) | Docs sync C4 - documentation vs reality audit - Before = the new script run over the pristine base docs with exemptions disabled. After-without-exemptions: total gaps: 104 in the captured run (103 plus the 1 new... |
| [e2e-triage-20261103/README.md](e2e-triage-20261103/README.md) | E2E triage 2026-11-03 (constitution 11.4.289) - Mechanical REAL / MOCKED / MIXED classification of the browser-driven web tests. |
| [e2e-triage-20261103/classification.json](e2e-triage-20261103/classification.json) | (JSON data artefact; not read, listed by name only) |
| [e2e-triage-20261103/classification.md](e2e-triage-20261103/classification.md) | Web test classification (11.4.289) - Files: 43 \| REAL 3 MOCKED 32 MIXED 5 other 3 |
| [err_error_class_enumeration_20261113.html](err_error_class_enumeration_20261113.html) | (HTML export; not read, listed by name only) |
| [err_error_class_enumeration_20261113.md](err_error_class_enumeration_20261113.md) | err.Error() to HTTP response enumeration (PX-F18, §11.4.276(C) class enumeration), rework 3 (2026-11-13) - The NO-GO review (docs/FOLLOW_UPS_NEXT_PHASE.md, "PX-F18 enumeration doc review: NO-GO") found one unlisted class and five defects of the instrument and the wording.... |
| [err_error_class_enumeration_20261113.pdf](err_error_class_enumeration_20261113.pdf) | (PDF export; not read, listed by name only) |
| [err_error_class_scan/README.html](err_error_class_scan/README.html) | (HTML export; not read, listed by name only) |
| [err_error_class_scan/README.md](err_error_class_scan/README.md) | err.Error() class scan sources (PX-F18) - Three Go AST programs used by docs/qa/err_error_class_enumeration_20261113.md. Stored as .txt so no Go tool compiles them. |
| [err_error_class_scan/README.pdf](err_error_class_scan/README.pdf) | (PDF export; not read, listed by name only) |
| [err_error_class_scan/all_error_calls.go.txt](err_error_class_scan/all_error_calls.go.txt) | (plain-text artefact; not read, listed by name only) |
| [err_error_class_scan/astscan_sinks.go.txt](err_error_class_scan/astscan_sinks.go.txt) | (plain-text artefact; not read, listed by name only) |
| [err_error_class_scan/flows_assign.go.txt](err_error_class_scan/flows_assign.go.txt) | (plain-text artefact; not read, listed by name only) |
| [favr3-fu2/green.txt](favr3-fu2/green.txt) | (plain-text artefact; not read, listed by name only) |
| [favr3-fu2/red.txt](favr3-fu2/red.txt) | (plain-text artefact; not read, listed by name only) |
| [favr3-fu3/green.txt](favr3-fu3/green.txt) | (plain-text artefact; not read, listed by name only) |
| [favr3-fu3/mutants.txt](favr3-fu3/mutants.txt) | (plain-text artefact; not read, listed by name only) |
| [favr3-fu3/red.txt](favr3-fu3/red.txt) | (plain-text artefact; not read, listed by name only) |
| [findings_20260626/discovered_findings.html](findings_20260626/discovered_findings.html) | (HTML export; not read, listed by name only) |
| [findings_20260626/discovered_findings.md](findings_20260626/discovered_findings.md) | Discovered Findings — 2026-06-26 (during §11.4.170 work) - The defect: a dark error color token (#EF4444) used as an error surface/background paired with a light foreground (#F8FAFC) → contrast 3.60:1, below WCAG AA 4.5:1 for... |
| [findings_20260626/discovered_findings.pdf](findings_20260626/discovered_findings.pdf) | (PDF export; not read, listed by name only) |
| [findings_20260626/ui_critical_resolution.html](findings_20260626/ui_critical_resolution.html) | (HTML export; not read, listed by name only) |
| [findings_20260626/ui_critical_resolution.md](findings_20260626/ui_critical_resolution.md) | CRITICAL UI Defects — Resolution Evidence (2026-06-26) - Autonomous ADB driving of the Android-TV login form submit is blocked: the Sign In control is a TV-Compose node that uiautomator reports clickable=false, input tap is... |
| [findings_20260626/ui_critical_resolution.pdf](findings_20260626/ui_critical_resolution.pdf) | (PDF export; not read, listed by name only) |
| [github_support_x_at_x.html](github_support_x_at_x.html) | (HTML export; not read, listed by name only) |
| [github_support_x_at_x.md](github_support_x_at_x.md) | Draft: GitHub Support request (attribution of placeholder email x@x) - Subject: Commits attributed to an unrelated account via placeholder email x@x |
| [github_support_x_at_x.pdf](github_support_x_at_x.pdf) | (PDF export; not read, listed by name only) |
| [helixqa-androidtv-20260629/POST_QA_ANALYSIS.html](helixqa-androidtv-20260629/POST_QA_ANALYSIS.html) | (HTML export; not read, listed by name only) |
| [helixqa-androidtv-20260629/POST_QA_ANALYSIS.md](helixqa-androidtv-20260629/POST_QA_ANALYSIS.md) | HelixQA Android TV — Post-QA Analysis (clean-slate run, §11.4.158) - Pre-HelixQA gate (run first, as mandated): catalog-api Go suite = 47 packages exit 0; |
| [helixqa-androidtv-20260629/PROOF_home_catalog_27750_covers.png](helixqa-androidtv-20260629/PROOF_home_catalog_27750_covers.png) | (PNG image; not read, listed by name only) |
| [helixqa-androidtv-20260629/login.png](helixqa-androidtv-20260629/login.png) | (PNG image; not read, listed by name only) |
| [helixqa-androidtv-20260629/qa-report.md](helixqa-androidtv-20260629/qa-report.md) | HelixQA Android TV Autonomous QA Report |
| [helixqa-androidtv-20260629/run2_episodes_list_100items.png](helixqa-androidtv-20260629/run2_episodes_list_100items.png) | (PNG image; not read, listed by name only) |
| [lib/ctrctl/go.mod](lib/ctrctl/go.mod) | (file; not read, listed by name only) |
| [lib/ctrctl/main.go](lib/ctrctl/main.go) | (file; not read, listed by name only) |
| [lib/lan_ip.sh](lib/lan_ip.sh) | (shell script; not read, listed by name only) |
| [lib/podman_env.sh](lib/podman_env.sh) | (shell script; not read, listed by name only) |
| [lib/test_lan_ip.sh](lib/test_lan_ip.sh) | (shell script; not read, listed by name only) |
| [lib/test_lan_ip_mutations.sh](lib/test_lan_ip_mutations.sh) | (shell script; not read, listed by name only) |
| [lib/test_podman_env.sh](lib/test_podman_env.sh) | (shell script; not read, listed by name only) |
| [lib/test_podman_env_mutations.sh](lib/test_podman_env_mutations.sh) | (shell script; not read, listed by name only) |
| [live-conv-docs-subs-r1-20261104.md](live-conv-docs-subs-r1-20261104.md) | Live QA r1: conversions, documents, subtitles (stack 3, build 4c79b7e3, report-only) - Bundle: /home/milosvasic/.cache/claude-tmp/live-conv-docs-subs-r1/ (conv_r{1,2,3}.jsonl, life.jsonl, xuser.jsonl, docs.jsonl, subs.jsonl, env_fields.txt, art/ = fetched... |
| [live-evidence.doubles.txt](live-evidence.doubles.txt) | (plain-text artefact; not read, listed by name only) |
| [live-evidence.interpreter-options.json](live-evidence.interpreter-options.json) | (JSON data artefact; not read, listed by name only) |
| [live-evidence.schema.json](live-evidence.schema.json) | (JSON data artefact; not read, listed by name only) |
| [live-map-20261103/01_before_podman.txt](live-map-20261103/01_before_podman.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/02_before_ports.txt](live-map-20261103/02_before_ports.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/03_rootlessport_cmdlines.txt](live-map-20261103/03_rootlessport_cmdlines.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/04_why_exited.txt](live-map-20261103/04_why_exited.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/05_restart_stack1.txt](live-map-20261103/05_restart_stack1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/06_real_state.txt](live-map-20261103/06_real_state.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/07_netns.txt](live-map-20261103/07_netns.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/08_rootless_netns.txt](live-map-20261103/08_rootless_netns.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/09_netns_identity.txt](live-map-20261103/09_netns_identity.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/10_teardown_stale.command.txt](live-map-20261103/10_teardown_stale.command.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/10_teardown_stale.txt](live-map-20261103/10_teardown_stale.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/11_restart_stack1.txt](live-map-20261103/11_restart_stack1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/12_stack1_fail_logs.txt](live-map-20261103/12_stack1_fail_logs.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/13_restart_stack1_v2.txt](live-map-20261103/13_restart_stack1_v2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/14_web_start.txt](live-map-20261103/14_web_start.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/15_restart_stack2.txt](live-map-20261103/15_restart_stack2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/20_compose_services.txt](live-map-20261103/20_compose_services.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/21_components_inventory.txt](live-map-20261103/21_components_inventory.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/30_after_matrix.txt](live-map-20261103/30_after_matrix.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/31_after_containers.txt](live-map-20261103/31_after_containers.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/32_after_listeners.txt](live-map-20261103/32_after_listeners.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/33_stack3_and_all_ports_20261104T1352.txt](live-map-20261103/33_stack3_and_all_ports_20261104T1352.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/40_smoke_qa_stack.log](live-map-20261103/40_smoke_qa_stack.log) | (file; not read, listed by name only) |
| [live-map-20261103/40_smoke_qa_stack2.log](live-map-20261103/40_smoke_qa_stack2.log) | (file; not read, listed by name only) |
| [live-map-20261103/50_other_components.txt](live-map-20261103/50_other_components.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/60_qa_stack1_compose_REDACTED.yml](live-map-20261103/60_qa_stack1_compose_REDACTED.yml) | (YAML data; not read, listed by name only) |
| [live-map-20261103/61_qa_stack2_compose_REDACTED.yml](live-map-20261103/61_qa_stack2_compose_REDACTED.yml) | (YAML data; not read, listed by name only) |
| [live-map-20261103/70_test_restart_stack_RED_before_fix.txt](live-map-20261103/70_test_restart_stack_RED_before_fix.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/71_test_restart_stack_GREEN_after_fix.txt](live-map-20261103/71_test_restart_stack_GREEN_after_fix.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/72_test_RED_web_website_no_probe.txt](live-map-20261103/72_test_RED_web_website_no_probe.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/73_test_GREEN_with_mutations.txt](live-map-20261103/73_test_GREEN_with_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/SHA256SUMS](live-map-20261103/80_session_scratchpad_evidence/SHA256SUMS) | (file; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack2_QA_ENDPOINTS.md](live-map-20261103/80_session_scratchpad_evidence/qa_stack2_QA_ENDPOINTS.md) | Catalogizer QA stack 2 (main @ f28f8165, clean clone; rootless podman, project catqa2) - Built commit: f28f816581b10f2a4398979fa437ed4b8b67be03 (clean checkout in /home/milosvasic/.cache/claude-tmp/qa2root.ZRr5Wu/src; submodules copied from live repo at the... |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack2_smoke_20261103T133400_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack2_smoke_20261103T133400_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack2_smoke_20261103T230653_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack2_smoke_20261103T230653_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack_QA_ENDPOINTS.md](live-map-20261103/80_session_scratchpad_evidence/qa_stack_QA_ENDPOINTS.md) | Catalogizer QA stack (main @ 96b7b5ec, scratch clone; rootless podman, project catqa) - Host: anton, LAN 192.168.1.115 (enp5s0); also ppp0 10.1.106.107. Firewall: ufw ENABLED=no (/etc/ufw/ufw.conf), firewalld/nftables not running -> nothing filters LAN... |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232152_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232152_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232207_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232207_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232339_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261102T232339_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261103T230634_journey.txt](live-map-20261103/80_session_scratchpad_evidence/qa_stack_smoke_20261103T230634_journey.txt) | (plain-text artefact; not read, listed by name only) |
| [live-map-20261103/MAP.md](live-map-20261103/MAP.md) | Live map 2026-11-03: what runs on which port, podman desync root cause, staged restart, honest gaps |
| [live-map-20261103/restart_stack.sh](live-map-20261103/restart_stack.sh) | (shell script; not read, listed by name only) |
| [live-map-20261103/test_restart_stack.sh](live-map-20261103/test_restart_stack.sh) | (shell script; not read, listed by name only) |
| [live-pprof-20261111/README.md](live-pprof-20261111/README.md) | Live pprof verification (2026-11-11) - CAT-QA-PPROF-1 - Target: stack 3 API catqa3-api, image localhost/catqa3-api:main-70d10721 (health reports build 70d10721dde373d70e6e4d171cced36c81f99ee9), on this host. |
| [live-pprof-20261111/api_before_ids.txt](live-pprof-20261111/api_before_ids.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/api_recreate.sh](live-pprof-20261111/api_recreate.sh) | (shell script; not read, listed by name only) |
| [live-pprof-20261111/env_names_before.txt](live-pprof-20261111/env_names_before.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/health_before.json](live-pprof-20261111/health_before.json) | (JSON data artefact; not read, listed by name only) |
| [live-pprof-20261111/pprof_live.py](live-pprof-20261111/pprof_live.py) | (Python script; not read, listed by name only) |
| [live-pprof-20261111/recreate_off_attempt.txt](live-pprof-20261111/recreate_off_attempt.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/red_off_1.txt](live-pprof-20261111/red_off_1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/res_off_1.json](live-pprof-20261111/res_off_1.json) | (JSON data artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_1.json](live-pprof-20261111/res_on_1.json) | (JSON data artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_1.txt](live-pprof-20261111/res_on_1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_2.json](live-pprof-20261111/res_on_2.json) | (JSON data artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_2.txt](live-pprof-20261111/res_on_2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_3.json](live-pprof-20261111/res_on_3.json) | (JSON data artefact; not read, listed by name only) |
| [live-pprof-20261111/res_on_3.txt](live-pprof-20261111/res_on_3.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-20261111/MANIFEST.sha256](live-rerun-20261111/MANIFEST.sha256) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run1.end](live-rerun-20261111/d5d6_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run1.err](live-rerun-20261111/d5d6_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run1.json](live-rerun-20261111/d5d6_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run1.rc](live-rerun-20261111/d5d6_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run1.start](live-rerun-20261111/d5d6_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run2.end](live-rerun-20261111/d5d6_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run2.err](live-rerun-20261111/d5d6_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run2.json](live-rerun-20261111/d5d6_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run2.rc](live-rerun-20261111/d5d6_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run2.start](live-rerun-20261111/d5d6_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run3.end](live-rerun-20261111/d5d6_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run3.err](live-rerun-20261111/d5d6_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run3.json](live-rerun-20261111/d5d6_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run3.rc](live-rerun-20261111/d5d6_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d5d6_run3.start](live-rerun-20261111/d5d6_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run1.end](live-rerun-20261111/d7_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run1.err](live-rerun-20261111/d7_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run1.json](live-rerun-20261111/d7_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d7_run1.rc](live-rerun-20261111/d7_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run1.start](live-rerun-20261111/d7_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run2.end](live-rerun-20261111/d7_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run2.err](live-rerun-20261111/d7_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run2.json](live-rerun-20261111/d7_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d7_run2.rc](live-rerun-20261111/d7_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run2.start](live-rerun-20261111/d7_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run3.end](live-rerun-20261111/d7_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run3.err](live-rerun-20261111/d7_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run3.json](live-rerun-20261111/d7_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d7_run3.rc](live-rerun-20261111/d7_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d7_run3.start](live-rerun-20261111/d7_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run1.end](live-rerun-20261111/d8_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run1.err](live-rerun-20261111/d8_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run1.json](live-rerun-20261111/d8_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d8_run1.rc](live-rerun-20261111/d8_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run1.start](live-rerun-20261111/d8_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run2.end](live-rerun-20261111/d8_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run2.err](live-rerun-20261111/d8_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run2.json](live-rerun-20261111/d8_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d8_run2.rc](live-rerun-20261111/d8_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run2.start](live-rerun-20261111/d8_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run3.end](live-rerun-20261111/d8_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run3.err](live-rerun-20261111/d8_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run3.json](live-rerun-20261111/d8_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/d8_run3.rc](live-rerun-20261111/d8_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/d8_run3.start](live-rerun-20261111/d8_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/fingerprint.txt](live-rerun-20261111/fingerprint.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run1.end](live-rerun-20261111/invalid_network_outage/d5d6_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run1.err](live-rerun-20261111/invalid_network_outage/d5d6_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run1.json](live-rerun-20261111/invalid_network_outage/d5d6_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run1.rc](live-rerun-20261111/invalid_network_outage/d5d6_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run1.start](live-rerun-20261111/invalid_network_outage/d5d6_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run2.end](live-rerun-20261111/invalid_network_outage/d5d6_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run2.err](live-rerun-20261111/invalid_network_outage/d5d6_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run2.json](live-rerun-20261111/invalid_network_outage/d5d6_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run2.rc](live-rerun-20261111/invalid_network_outage/d5d6_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run2.start](live-rerun-20261111/invalid_network_outage/d5d6_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run3.end](live-rerun-20261111/invalid_network_outage/d5d6_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run3.err](live-rerun-20261111/invalid_network_outage/d5d6_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run3.json](live-rerun-20261111/invalid_network_outage/d5d6_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run3.rc](live-rerun-20261111/invalid_network_outage/d5d6_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d5d6_run3.start](live-rerun-20261111/invalid_network_outage/d5d6_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run1.end](live-rerun-20261111/invalid_network_outage/d7_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run1.err](live-rerun-20261111/invalid_network_outage/d7_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run1.json](live-rerun-20261111/invalid_network_outage/d7_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run1.rc](live-rerun-20261111/invalid_network_outage/d7_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run1.start](live-rerun-20261111/invalid_network_outage/d7_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run2.end](live-rerun-20261111/invalid_network_outage/d7_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run2.err](live-rerun-20261111/invalid_network_outage/d7_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run2.json](live-rerun-20261111/invalid_network_outage/d7_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run2.rc](live-rerun-20261111/invalid_network_outage/d7_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run2.start](live-rerun-20261111/invalid_network_outage/d7_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run3.end](live-rerun-20261111/invalid_network_outage/d7_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run3.err](live-rerun-20261111/invalid_network_outage/d7_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run3.json](live-rerun-20261111/invalid_network_outage/d7_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run3.rc](live-rerun-20261111/invalid_network_outage/d7_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d7_run3.start](live-rerun-20261111/invalid_network_outage/d7_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run1.end](live-rerun-20261111/invalid_network_outage/d8_run1.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run1.err](live-rerun-20261111/invalid_network_outage/d8_run1.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run1.json](live-rerun-20261111/invalid_network_outage/d8_run1.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run1.rc](live-rerun-20261111/invalid_network_outage/d8_run1.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run1.start](live-rerun-20261111/invalid_network_outage/d8_run1.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run2.end](live-rerun-20261111/invalid_network_outage/d8_run2.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run2.err](live-rerun-20261111/invalid_network_outage/d8_run2.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run2.json](live-rerun-20261111/invalid_network_outage/d8_run2.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run2.rc](live-rerun-20261111/invalid_network_outage/d8_run2.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run2.start](live-rerun-20261111/invalid_network_outage/d8_run2.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run3.end](live-rerun-20261111/invalid_network_outage/d8_run3.end) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run3.err](live-rerun-20261111/invalid_network_outage/d8_run3.err) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run3.json](live-rerun-20261111/invalid_network_outage/d8_run3.json) | (JSON data artefact; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run3.rc](live-rerun-20261111/invalid_network_outage/d8_run3.rc) | (file; not read, listed by name only) |
| [live-rerun-20261111/invalid_network_outage/d8_run3.start](live-rerun-20261111/invalid_network_outage/d8_run3.start) | (file; not read, listed by name only) |
| [live-rerun-20261111/run.sh](live-rerun-20261111/run.sh) | (shell script; not read, listed by name only) |
| [live-rerun-ssrf-20261111/MANIFEST.sha256](live-rerun-ssrf-20261111/MANIFEST.sha256) | (file; not read, listed by name only) |
| [live-rerun-ssrf-20261111/README.md](live-rerun-ssrf-20261111/README.md) | Live rerun 2026-11-11: SSRF-F2..F5, verify.py, front-door client-IP (stack 3, 192.168.1.17, build 70d10721; contains security-queue commits 4e6bc170/0a0a747c/08629078) - No container restarted, no production code edited. N=3 each; per-run outputs are the raw files. Secrets only sourced in subshells; none printed/stored. |
| [live-rerun-ssrf-20261111/clientip_run1.txt](live-rerun-ssrf-20261111/clientip_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/clientip_run2.txt](live-rerun-ssrf-20261111/clientip_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/clientip_run3.txt](live-rerun-ssrf-20261111/clientip_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/f2f5_run1.txt](live-rerun-ssrf-20261111/f2f5_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/f2f5_run2.txt](live-rerun-ssrf-20261111/f2f5_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/f2f5_run3.txt](live-rerun-ssrf-20261111/f2f5_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/listener.py](live-rerun-ssrf-20261111/listener.py) | (Python script; not read, listed by name only) |
| [live-rerun-ssrf-20261111/run_f2f5.sh](live-rerun-ssrf-20261111/run_f2f5.sh) | (shell script; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_declared_run1.txt](live-rerun-ssrf-20261111/verify_declared_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_declared_run2.txt](live-rerun-ssrf-20261111/verify_declared_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_declared_run3.txt](live-rerun-ssrf-20261111/verify_declared_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_run1.txt](live-rerun-ssrf-20261111/verify_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_run2.txt](live-rerun-ssrf-20261111/verify_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [live-rerun-ssrf-20261111/verify_run3.txt](live-rerun-ssrf-20261111/verify_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [live-storage-r1-20261104.md](live-storage-r1-20261104.md) | Live storage/scan investigation r1 (clone of main 4c79b7e3; no live-repo change) - Evidence bundle: /home/milosvasic/.cache/claude-tmp/live-storage-r1/ (yandex/, yandex_scan/, smb/, ftp/, nfs/, old_runs/, tests/). Credentials: 0 hits (password strings... |
| [live-web-stack3-20261104/REPORT.md](live-web-stack3-20261104/REPORT.md) | Live web specs, stack 3, round 3 (2026-11-04) - Target (LAN): web http://192.168.1.115:33020, API http://192.168.1.115:38082, website :33021. Specs + stack from main 4c79b7e3611fae4c400555ddf7f8db7a677f9ef7 (fresh... |
| [monitoring-deploy-20261104/00_README.txt](monitoring-deploy-20261104/00_README.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/01_otel_validate.txt](monitoring-deploy-20261104/01_otel_validate.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/02_http_status.txt](monitoring-deploy-20261104/02_http_status.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/03_containers_ports.txt](monitoring-deploy-20261104/03_containers_ports.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/04_collector_log.txt](monitoring-deploy-20261104/04_collector_log.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/05_prometheus_api.txt](monitoring-deploy-20261104/05_prometheus_api.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/06_regression_test_run.txt](monitoring-deploy-20261104/06_regression_test_run.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/07_mutation_run.txt](monitoring-deploy-20261104/07_mutation_run.txt) | (plain-text artefact; not read, listed by name only) |
| [monitoring-deploy-20261104/08_teardown.txt](monitoring-deploy-20261104/08_teardown.txt) | (plain-text artefact; not read, listed by name only) |
| [new1-cover-thumbnails/README.md](new1-cover-thumbnails/README.md) | NEW-1 generate-thumbnails returned {"generated":0} - Reproduction (reproduced-live): red_run1.txt (build 015896fd89180a42cc6c1e794719bd04e39e9c40, stack 3, POST returned 200 generated 0 in 0.03 s, no log, cover_art 0... |
| [new1-cover-thumbnails/exp_pg.txt](new1-cover-thumbnails/exp_pg.txt) | (plain-text artefact; not read, listed by name only) |
| [new1-cover-thumbnails/green_main5c48.txt](new1-cover-thumbnails/green_main5c48.txt) | (plain-text artefact; not read, listed by name only) |
| [new1-cover-thumbnails/red_run1.txt](new1-cover-thumbnails/red_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [nil-slice-null-census-20261104.md](nil-slice-null-census-20261104.md) | Nil slice / map serialised as JSON null - class census (2026-11-04) - Defect class (§11.4.276(C)): a Go nil slice or map is written by encoding/json as null, where the API documents an array |
| [player-lifecycle-wl13-20261104/README.md](player-lifecycle-wl13-20261104/README.md) | WL13 player lifecycle (F1, F2, F3): real-browser evidence, 2026-11-04 - Spec: catalog-web/e2e/player-real/player_lifecycle.spec.ts (real MediaPlayer, real <video>, chromium and firefox, local ticket |
| [player-lifecycle-wl13-20261104/green_final_both_browsers.txt](player-lifecycle-wl13-20261104/green_final_both_browsers.txt) | (plain-text artefact; not read, listed by name only) |
| [player-lifecycle-wl13-20261104/red_f1_revert_chromium_firefox.txt](player-lifecycle-wl13-20261104/red_f1_revert_chromium_firefox.txt) | (plain-text artefact; not read, listed by name only) |
| [player-lifecycle-wl13-20261104/red_main_chromium.txt](player-lifecycle-wl13-20261104/red_main_chromium.txt) | (plain-text artefact; not read, listed by name only) |
| [player-lifecycle-wl13-20261104/red_main_firefox.txt](player-lifecycle-wl13-20261104/red_main_firefox.txt) | (plain-text artefact; not read, listed by name only) |
| [provision-when-down-20260630/Status.md](provision-when-down-20260630/Status.md) | §11.4.76 / Boot-provisioning — provision-when-DOWN PROVEN - the digital.vasic.containers submodule (remote compose-up over SSH), SAFELY proven against a |
| [responsive_d1_d2/green-chromium.json](responsive_d1_d2/green-chromium.json) | (JSON data artefact; not read, listed by name only) |
| [responsive_d1_d2/red-chromium.json](responsive_d1_d2/red-chromium.json) | (JSON data artefact; not read, listed by name only) |
| [retest-20261112/RETEST_main_563617b1.md](retest-20261112/RETEST_main_563617b1.md) | Retest main @ 563617b1 (clean git archive; compare v2.4.1 a90896a3) - Method: git archive main + tag v2.4.1 under mktemp root; submodule dirs are empty in git archive, so working trees |
| [retest-20261112/SHA256SUMS](retest-20261112/SHA256SUMS) | (file; not read, listed by name only) |
| [s3-live-20261105/README.md](s3-live-20261105/README.md) | CAT-S3-1 live evidence (2026-11-05) - Target: real Garage v1.0.1 (see target_identity.txt). Test: catalog-api/filesystem/s3_live_test.go (-tags s3live). |
| [s3-live-20261105/cleanup_check_after_runs.txt](s3-live-20261105/cleanup_check_after_runs.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/control_server_stopped.txt](s3-live-20261105/control_server_stopped.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/down_purge_mutations.txt](s3-live-20261105/down_purge_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/down_purge_test_green.txt](s3-live-20261105/down_purge_test_green.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/green_exits.txt](s3-live-20261105/green_exits.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/green_run1.txt](s3-live-20261105/green_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/green_run2.txt](s3-live-20261105/green_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/green_run3.txt](s3-live-20261105/green_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/manifest.sha256](s3-live-20261105/manifest.sha256) | (file; not read, listed by name only) |
| [s3-live-20261105/mutation_M1_copysource_unencoded.txt](s3-live-20261105/mutation_M1_copysource_unencoded.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/mutation_M2_batch1001.txt](s3-live-20261105/mutation_M2_batch1001.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/mutation_MB_deletedir_prefix_no_slash.txt](s3-live-20261105/mutation_MB_deletedir_prefix_no_slash.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/mutation_MC_plus_unencoded.txt](s3-live-20261105/mutation_MC_plus_unencoded.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/run.sh](s3-live-20261105/run.sh) | (shell script; not read, listed by name only) |
| [s3-live-20261105/skip_control.txt](s3-live-20261105/skip_control.txt) | (plain-text artefact; not read, listed by name only) |
| [s3-live-20261105/target_identity.txt](s3-live-20261105/target_identity.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/REPRODUCTION.html](sec-ws-1/REPRODUCTION.html) | (HTML export; not read, listed by name only) |
| [sec-ws-1/REPRODUCTION.md](sec-ws-1/REPRODUCTION.md) | SEC-WS-1 Reproduction record (§11.4.292(C)) - PROVISIONAL |
| [sec-ws-1/REPRODUCTION.pdf](sec-ws-1/REPRODUCTION.pdf) | (PDF export; not read, listed by name only) |
| [sec-ws-1/green.txt](sec-ws-1/green.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/green_round2.txt](sec-ws-1/green_round2.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/mutants.sh](sec-ws-1/mutants.sh) | (shell script; not read, listed by name only) |
| [sec-ws-1/mutants.txt](sec-ws-1/mutants.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/mutants_round2.sh](sec-ws-1/mutants_round2.sh) | (shell script; not read, listed by name only) |
| [sec-ws-1/mutants_round2.txt](sec-ws-1/mutants_round2.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/mutants_round3.sh](sec-ws-1/mutants_round3.sh) | (shell script; not read, listed by name only) |
| [sec-ws-1/mutants_round3.txt](sec-ws-1/mutants_round3.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/nginx_upstream_test.txt](sec-ws-1/nginx_upstream_test.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1/red.txt](sec-ws-1/red.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1r3/green.txt](sec-ws-1r3/green.txt) | (plain-text artefact; not read, listed by name only) |
| [sec-ws-1r3/red.txt](sec-ws-1r3/red.txt) | (plain-text artefact; not read, listed by name only) |
| [second-host-20261106/README.md](second-host-20261106/README.md) | Second-source rate-limit bucket check, stack 3, 2026-11-06 (scratch branch, not pushed) - Method: no physically distinct LAN host was available (Anton.local IS this host; LAN neighbours are Android phones/router, ssh closed). |
| [second-host-20261106/api_seen_client_ips.txt](second-host-20261106/api_seen_client_ips.txt) | (plain-text artefact; not read, listed by name only) |
| [second-host-20261106/run_all.txt](second-host-20261106/run_all.txt) | (plain-text artefact; not read, listed by name only) |
| [second-host-20261106/run_bucket.sh](second-host-20261106/run_bucket.sh) | (shell script; not read, listed by name only) |
| [secq2-repro-20261112/README.html](secq2-repro-20261112/README.html) | (HTML export; not read, listed by name only) |
| [secq2-repro-20261112/README.md](secq2-repro-20261112/README.md) | SECQ-2 / WD-F1 / WD-F1a live re-reproduction (2026-11-12) - PROVISIONAL - Reproduction: result=not-reproducible-with-evidence; surface=live-target (real Yandex WebDAV, chaos class: TCP fault proxy, the real service answers);... |
| [secq2-repro-20261112/README.pdf](secq2-repro-20261112/README.pdf) | (PDF export; not read, listed by name only) |
| [secq2-repro-20261112/SHA256SUMS](secq2-repro-20261112/SHA256SUMS) | (file; not read, listed by name only) |
| [secq2-repro-20261112/rep6.json](secq2-repro-20261112/rep6.json) | (JSON data artefact; not read, listed by name only) |
| [secq2-repro-20261112/rep6.log.txt](secq2-repro-20261112/rep6.log.txt) | (plain-text artefact; not read, listed by name only) |
| [secq2-repro-20261112/rep6.start](secq2-repro-20261112/rep6.start) | (file; not read, listed by name only) |
| [security-live-20261106/README.md](security-live-20261106/README.md) | Live security verification, stack 3, 2026-11-06 (scratch branch, not pushed) - Scripts: run_prom.sh, run_pprof.sh, run_ssrf.sh, run_xff.sh/xff_all.sh (+listener.py). Each result file is the raw output; N=3 runs per item. |
| [security-live-20261106/listener.py](security-live-20261106/listener.py) | (Python script; not read, listed by name only) |
| [security-live-20261106/pprof_cap_detail.txt](security-live-20261106/pprof_cap_detail.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/pprof_run1.txt](security-live-20261106/pprof_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/pprof_run2.txt](security-live-20261106/pprof_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/pprof_run3.txt](security-live-20261106/pprof_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/prom_run1.txt](security-live-20261106/prom_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/prom_run2.txt](security-live-20261106/prom_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/prom_run3.txt](security-live-20261106/prom_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/run_pprof.sh](security-live-20261106/run_pprof.sh) | (shell script; not read, listed by name only) |
| [security-live-20261106/run_prom.sh](security-live-20261106/run_prom.sh) | (shell script; not read, listed by name only) |
| [security-live-20261106/run_ssrf.sh](security-live-20261106/run_ssrf.sh) | (shell script; not read, listed by name only) |
| [security-live-20261106/run_xff.sh](security-live-20261106/run_xff.sh) | (shell script; not read, listed by name only) |
| [security-live-20261106/ssrf_run1.txt](security-live-20261106/ssrf_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/ssrf_run2.txt](security-live-20261106/ssrf_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/ssrf_run3.txt](security-live-20261106/ssrf_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/xff_all.sh](security-live-20261106/xff_all.sh) | (shell script; not read, listed by name only) |
| [security-live-20261106/xff_normalised.txt](security-live-20261106/xff_normalised.txt) | (plain-text artefact; not read, listed by name only) |
| [security-live-20261106/xff_raw.txt](security-live-20261106/xff_raw.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/MANIFEST.sha256](security-queue-fix-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [security-queue-fix-20261105/NOTES.txt](security-queue-fix-20261105/NOTES.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item1_green.txt](security-queue-fix-20261105/item1_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item1_mutations.txt](security-queue-fix-20261105/item1_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item1_red_precode.txt](security-queue-fix-20261105/item1_red_precode.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item2_green.txt](security-queue-fix-20261105/item2_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item2_mutations.txt](security-queue-fix-20261105/item2_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item2_red_precode.txt](security-queue-fix-20261105/item2_red_precode.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item3_green.txt](security-queue-fix-20261105/item3_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item3_mutations.txt](security-queue-fix-20261105/item3_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item3_red_precode.txt](security-queue-fix-20261105/item3_red_precode.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item4_green.txt](security-queue-fix-20261105/item4_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item4_mutations.txt](security-queue-fix-20261105/item4_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item4_red_precode.txt](security-queue-fix-20261105/item4_red_precode.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item5_green.txt](security-queue-fix-20261105/item5_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item5_mutations.txt](security-queue-fix-20261105/item5_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item6_green.txt](security-queue-fix-20261105/item6_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/item6_mutations.txt](security-queue-fix-20261105/item6_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/regression_packages.txt](security-queue-fix-20261105/regression_packages.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/regression_root_package.txt](security-queue-fix-20261105/regression_root_package.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_F3_red.txt](security-queue-fix-20261105/round1_F3_red.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_F5_green.txt](security-queue-fix-20261105/round1_F5_green.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_F5_red.txt](security-queue-fix-20261105/round1_F5_red.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_NOTES.txt](security-queue-fix-20261105/round1_NOTES.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_cover_mutations.txt](security-queue-fix-20261105/round1_cover_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_internal_services.txt](security-queue-fix-20261105/round1_internal_services.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round1_logredact_mutations.txt](security-queue-fix-20261105/round1_logredact_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round2_NOTES.txt](security-queue-fix-20261105/round2_NOTES.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round2_internal_services.txt](security-queue-fix-20261105/round2_internal_services.txt) | (plain-text artefact; not read, listed by name only) |
| [security-queue-fix-20261105/round2_mutations.txt](security-queue-fix-20261105/round2_mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [secws1-fu/mutants.txt](secws1-fu/mutants.txt) | (plain-text artefact; not read, listed by name only) |
| [smb-subdir-20261110/README.md](smb-subdir-20261110/README.md) | SMB sub-directory storage roots (CAT-SMB-SUBDIR-1) - A storage root of protocol smb may now name share/sub/dir in its path; scanning, listing, download and copy are confined to that subtree. |
| [smb-subdir-20261110/live-run-20261111T121104Z/ENV.txt](smb-subdir-20261110/live-run-20261111T121104Z/ENV.txt) | (plain-text artefact; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/SHA256SUMS](smb-subdir-20261110/live-run-20261111T121104Z/SHA256SUMS) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/env-parametrization.diff](smb-subdir-20261110/live-run-20261111T121104Z/env-parametrization.diff) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/negctrl.raw](smb-subdir-20261110/live-run-20261111T121104Z/negctrl.raw) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/run1.raw](smb-subdir-20261110/live-run-20261111T121104Z/run1.raw) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/run2.raw](smb-subdir-20261110/live-run-20261111T121104Z/run2.raw) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-20261111T121104Z/run3.raw](smb-subdir-20261110/live-run-20261111T121104Z/run3.raw) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-anton-20261111T190325Z/README.md](smb-subdir-20261110/live-run-anton-20261111T190325Z/README.md) | Anton Public/Testing SMB live run (CAT-SMB-SUBDIR-1) - Command: docs/qa/smb-subdir-20261110/run.sh <0600 env> (go test -tags live_smb -run 'TestLiveSMBSubdir\|TestLiveSMBServiceSubdir' ./filesystem ./internal/services) on... |
| [smb-subdir-20261110/live-run-anton-20261111T190325Z/go_test.log](smb-subdir-20261110/live-run-anton-20261111T190325Z/go_test.log) | (file; not read, listed by name only) |
| [smb-subdir-20261110/live-run-anton-20261111T190325Z/go_test.log.sha256](smb-subdir-20261110/live-run-anton-20261111T190325Z/go_test.log.sha256) | (file; not read, listed by name only) |
| [smb-subdir-20261110/run.sh](smb-subdir-20261110/run.sh) | (shell script; not read, listed by name only) |
| [sonarqube-20261105/EVIDENCE_NOTE.md](sonarqube-20261105/EVIDENCE_NOTE.md) | Evidence note (run1 / run2) - postgres:15-alpine) that are no longer present in the local podman store. They were NOT produced by the reviewed scripts of |
| [sonarqube-20261105/run1/SHA256SUMS](sonarqube-20261105/run1/SHA256SUMS) | (file; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.ce_task.json](sonarqube-20261105/run1/catalog-api.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.hotspots.json](sonarqube-20261105/run1/catalog-api.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.issue_facets.json](sonarqube-20261105/run1/catalog-api.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.issues.json](sonarqube-20261105/run1/catalog-api.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.measures.json](sonarqube-20261105/run1/catalog-api.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-api.quality_gate.json](sonarqube-20261105/run1/catalog-api.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.ce_task.json](sonarqube-20261105/run1/catalog-web.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.hotspots.json](sonarqube-20261105/run1/catalog-web.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.issue_facets.json](sonarqube-20261105/run1/catalog-web.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.issues.json](sonarqube-20261105/run1/catalog-web.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.measures.json](sonarqube-20261105/run1/catalog-web.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/catalog-web.quality_gate.json](sonarqube-20261105/run1/catalog-web.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/gate_status.txt](sonarqube-20261105/run1/gate_status.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/run.json](sonarqube-20261105/run1/run.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/scanner_image_digest.txt](sonarqube-20261105/run1/scanner_image_digest.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/scrub_control.txt](sonarqube-20261105/run1/scrub_control.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/server_status.json](sonarqube-20261105/run1/server_status.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run1/sonarqube_image_digest.txt](sonarqube-20261105/run1/sonarqube_image_digest.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/SHA256SUMS](sonarqube-20261105/run2/SHA256SUMS) | (file; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.ce_task.json](sonarqube-20261105/run2/catalog-api.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.hotspots.json](sonarqube-20261105/run2/catalog-api.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.issue_facets.json](sonarqube-20261105/run2/catalog-api.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.issues.json](sonarqube-20261105/run2/catalog-api.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.measures.json](sonarqube-20261105/run2/catalog-api.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-api.quality_gate.json](sonarqube-20261105/run2/catalog-api.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.ce_task.json](sonarqube-20261105/run2/catalog-web.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.hotspots.json](sonarqube-20261105/run2/catalog-web.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.issue_facets.json](sonarqube-20261105/run2/catalog-web.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.issues.json](sonarqube-20261105/run2/catalog-web.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.measures.json](sonarqube-20261105/run2/catalog-web.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/catalog-web.quality_gate.json](sonarqube-20261105/run2/catalog-web.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/gate_status.txt](sonarqube-20261105/run2/gate_status.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/run.json](sonarqube-20261105/run2/run.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/scanner_image_digest.txt](sonarqube-20261105/run2/scanner_image_digest.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/scrub_control.txt](sonarqube-20261105/run2/scrub_control.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/server_status.json](sonarqube-20261105/run2/server_status.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run2/sonarqube_image_digest.txt](sonarqube-20261105/run2/sonarqube_image_digest.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/SHA256SUMS](sonarqube-20261105/run3/SHA256SUMS) | (file; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.ce_task.json](sonarqube-20261105/run3/catalog-api.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.hotspots.json](sonarqube-20261105/run3/catalog-api.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.issue_facets.json](sonarqube-20261105/run3/catalog-api.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.issues.json](sonarqube-20261105/run3/catalog-api.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.measures.json](sonarqube-20261105/run3/catalog-api.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.quality_gate.json](sonarqube-20261105/run3/catalog-api.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-api.scanner.txt](sonarqube-20261105/run3/catalog-api.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.ce_task.json](sonarqube-20261105/run3/catalog-web.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.hotspots.json](sonarqube-20261105/run3/catalog-web.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.issue_facets.json](sonarqube-20261105/run3/catalog-web.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.issues.json](sonarqube-20261105/run3/catalog-web.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.measures.json](sonarqube-20261105/run3/catalog-web.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.quality_gate.json](sonarqube-20261105/run3/catalog-web.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/catalog-web.scanner.txt](sonarqube-20261105/run3/catalog-web.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/gate_status.txt](sonarqube-20261105/run3/gate_status.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/image_digests.txt](sonarqube-20261105/run3/image_digests.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/run.json](sonarqube-20261105/run3/run.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/scrub_control.txt](sonarqube-20261105/run3/scrub_control.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261105/run3/server_status.json](sonarqube-20261105/run3/server_status.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/README.md](sonarqube-20261111/run4/README.md) | SonarQube run4 - Catalogizer main tag v2.4.1 (commit 04c6d422b093a8beca5ae6ac0818b0a6ed41957a) - Purpose: measure whether the landed Sonar fixes (CAT-QA-SONAR-1/-3/-4/-6, PROVISIONAL) cleared findings. Baseline: ../../sonarqube-20261105/run3/ (main 0020ad3e). |
| [sonarqube-20261111/run4/SHA256SUMS](sonarqube-20261111/run4/SHA256SUMS) | (file; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.ce_task.json](sonarqube-20261111/run4/catalog-api.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.hotspots.json](sonarqube-20261111/run4/catalog-api.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.issue_facets.json](sonarqube-20261111/run4/catalog-api.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.issues.json](sonarqube-20261111/run4/catalog-api.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.issues_closed.json](sonarqube-20261111/run4/catalog-api.issues_closed.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.measures.json](sonarqube-20261111/run4/catalog-api.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.quality_gate.json](sonarqube-20261111/run4/catalog-api.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-api.scanner.txt](sonarqube-20261111/run4/catalog-api.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.ce_task.json](sonarqube-20261111/run4/catalog-web.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.hotspots.json](sonarqube-20261111/run4/catalog-web.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.issue_facets.json](sonarqube-20261111/run4/catalog-web.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.issues.json](sonarqube-20261111/run4/catalog-web.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.issues_closed.json](sonarqube-20261111/run4/catalog-web.issues_closed.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.measures.json](sonarqube-20261111/run4/catalog-web.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.quality_gate.json](sonarqube-20261111/run4/catalog-web.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/catalog-web.scanner.txt](sonarqube-20261111/run4/catalog-web.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/comparison_vs_run3.md](sonarqube-20261111/run4/comparison_vs_run3.md) | comparison_vs_run3.md |
| [sonarqube-20261111/run4/gate_status.txt](sonarqube-20261111/run4/gate_status.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/image_digests.txt](sonarqube-20261111/run4/image_digests.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/run.json](sonarqube-20261111/run4/run.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/scrub_control.txt](sonarqube-20261111/run4/scrub_control.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run4/server_status.json](sonarqube-20261111/run4/server_status.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/README.md](sonarqube-20261111/run5/README.md) | SonarQube run5 - Catalogizer main 563617b1fe04ec9310adb8b1795264caf955f53b (PROVISIONAL) - PROVISIONAL: static analysis only. A closed finding means Sonar no longer reports it, not that the product works (11.4.289). Same procedure and same SonarQube... |
| [sonarqube-20261111/run5/SHA256SUMS](sonarqube-20261111/run5/SHA256SUMS) | (file; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.ce_task.json](sonarqube-20261111/run5/catalog-api.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.hotspots.json](sonarqube-20261111/run5/catalog-api.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.issue_facets.json](sonarqube-20261111/run5/catalog-api.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.issues.json](sonarqube-20261111/run5/catalog-api.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.measures.json](sonarqube-20261111/run5/catalog-api.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.quality_gate.json](sonarqube-20261111/run5/catalog-api.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-api.scanner.txt](sonarqube-20261111/run5/catalog-api.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.ce_task.json](sonarqube-20261111/run5/catalog-web.ce_task.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.hotspots.json](sonarqube-20261111/run5/catalog-web.hotspots.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.issue_facets.json](sonarqube-20261111/run5/catalog-web.issue_facets.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.issues.json](sonarqube-20261111/run5/catalog-web.issues.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.measures.json](sonarqube-20261111/run5/catalog-web.measures.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.quality_gate.json](sonarqube-20261111/run5/catalog-web.quality_gate.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/catalog-web.scanner.txt](sonarqube-20261111/run5/catalog-web.scanner.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/gate_status.txt](sonarqube-20261111/run5/gate_status.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/image_digests.txt](sonarqube-20261111/run5/image_digests.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/run.json](sonarqube-20261111/run5/run.json) | (JSON data artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/scrub_control.txt](sonarqube-20261111/run5/scrub_control.txt) | (plain-text artefact; not read, listed by name only) |
| [sonarqube-20261111/run5/server_status.json](sonarqube-20261111/run5/server_status.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-live-20261105/README.md](stack3-live-20261105/README.md) | Stack 3 LIVE re-verification 2 (CAT-QA-STACK3-LIVE2) - /health build_number 5c480072b2d5cda2102b7091a8f7422d941f24e2, API http://127.0.0.1:38082. |
| [stack3-live-20261105/live2-evidence/MANIFEST.sha256](stack3-live-20261105/live2-evidence/MANIFEST.sha256) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_cleanup_after_run1.txt](stack3-live-20261105/live2-evidence/cap_cleanup_after_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_cleanup_after_run2.txt](stack3-live-20261105/live2-evidence/cap_cleanup_after_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_cleanup_after_run3.txt](stack3-live-20261105/live2-evidence/cap_cleanup_after_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_full_state_burst.txt](stack3-live-20261105/live2-evidence/cap_co_full_state_burst.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_post1.txt](stack3-live-20261105/live2-evidence/cap_co_post1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_run1.txt](stack3-live-20261105/live2-evidence/cap_co_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_run2.txt](stack3-live-20261105/live2-evidence/cap_co_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_run3.txt](stack3-live-20261105/live2-evidence/cap_co_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_summary_run2.txt](stack3-live-20261105/live2-evidence/cap_co_summary_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_co_summary_run3.txt](stack3-live-20261105/live2-evidence/cap_co_summary_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_driver_one.sh](stack3-live-20261105/live2-evidence/cap_driver_one.sh) | (shell script; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_full_state_burst.txt](stack3-live-20261105/live2-evidence/cap_pl_full_state_burst.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_post1.txt](stack3-live-20261105/live2-evidence/cap_pl_post1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_run1.txt](stack3-live-20261105/live2-evidence/cap_pl_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_run2.txt](stack3-live-20261105/live2-evidence/cap_pl_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_run3.txt](stack3-live-20261105/live2-evidence/cap_pl_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_summary_run2.txt](stack3-live-20261105/live2-evidence/cap_pl_summary_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pl_summary_run3.txt](stack3-live-20261105/live2-evidence/cap_pl_summary_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/cap_pre.txt](stack3-live-20261105/live2-evidence/cap_pre.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__admin.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof__admin.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__admin.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof__admin.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__noauth.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof__noauth.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__noauth.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof__noauth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__user.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof__user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof__user.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof__user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_admin.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_admin.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_admin.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_admin.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_noauth.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_noauth.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_noauth.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_noauth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_user.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_user.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_cmdline_user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_admin.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_admin.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_admin.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_admin.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_noauth.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_noauth.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_noauth.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_noauth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_user.body](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_user.txt](stack3-live-20261105/live2-evidence/pprof__debug_pprof_heap_user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/roles.body](stack3-live-20261105/live2-evidence/roles.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/roles.txt](stack3-live-20261105/live2-evidence/roles.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_3.body](stack3-live-20261105/live2-evidence/secq1_cleanup_del_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_3.txt](stack3-live-20261105/live2-evidence/secq1_cleanup_del_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_4.body](stack3-live-20261105/live2-evidence/secq1_cleanup_del_4.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_4.txt](stack3-live-20261105/live2-evidence/secq1_cleanup_del_4.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_5.body](stack3-live-20261105/live2-evidence/secq1_cleanup_del_5.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_del_5.txt](stack3-live-20261105/live2-evidence/secq1_cleanup_del_5.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_cleanup_sql.txt](stack3-live-20261105/live2-evidence/secq1_cleanup_sql.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_final_list.body](stack3-live-20261105/live2-evidence/secq1_final_list.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_final_list.txt](stack3-live-20261105/live2-evidence/secq1_final_list.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_final_list2.body](stack3-live-20261105/live2-evidence/secq1_final_list2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_final_list2.txt](stack3-live-20261105/live2-evidence/secq1_final_list2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_1.body](stack3-live-20261105/live2-evidence/secq1_lan_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_1.txt](stack3-live-20261105/live2-evidence/secq1_lan_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_2.body](stack3-live-20261105/live2-evidence/secq1_lan_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_2.txt](stack3-live-20261105/live2-evidence/secq1_lan_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_3.body](stack3-live-20261105/live2-evidence/secq1_lan_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_lan_3.txt](stack3-live-20261105/live2-evidence/secq1_lan_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_1.body](stack3-live-20261105/live2-evidence/secq1_linklocal_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_1.txt](stack3-live-20261105/live2-evidence/secq1_linklocal_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_2.body](stack3-live-20261105/live2-evidence/secq1_linklocal_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_2.txt](stack3-live-20261105/live2-evidence/secq1_linklocal_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_3.body](stack3-live-20261105/live2-evidence/secq1_linklocal_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_linklocal_3.txt](stack3-live-20261105/live2-evidence/secq1_linklocal_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_1.body](stack3-live-20261105/live2-evidence/secq1_localhost_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_1.txt](stack3-live-20261105/live2-evidence/secq1_localhost_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_2.body](stack3-live-20261105/live2-evidence/secq1_localhost_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_2.txt](stack3-live-20261105/live2-evidence/secq1_localhost_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_3.body](stack3-live-20261105/live2-evidence/secq1_localhost_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_localhost_3.txt](stack3-live-20261105/live2-evidence/secq1_localhost_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_1.body](stack3-live-20261105/live2-evidence/secq1_loopback_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_1.txt](stack3-live-20261105/live2-evidence/secq1_loopback_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_2.body](stack3-live-20261105/live2-evidence/secq1_loopback_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_2.txt](stack3-live-20261105/live2-evidence/secq1_loopback_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_3.body](stack3-live-20261105/live2-evidence/secq1_loopback_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_loopback_3.txt](stack3-live-20261105/live2-evidence/secq1_loopback_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_1.body](stack3-live-20261105/live2-evidence/secq1_mapped_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_1.txt](stack3-live-20261105/live2-evidence/secq1_mapped_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_2.body](stack3-live-20261105/live2-evidence/secq1_mapped_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_2.txt](stack3-live-20261105/live2-evidence/secq1_mapped_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_3.body](stack3-live-20261105/live2-evidence/secq1_mapped_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_mapped_3.txt](stack3-live-20261105/live2-evidence/secq1_mapped_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_1.body](stack3-live-20261105/live2-evidence/secq1_metadata6_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_1.txt](stack3-live-20261105/live2-evidence/secq1_metadata6_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_2.body](stack3-live-20261105/live2-evidence/secq1_metadata6_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_2.txt](stack3-live-20261105/live2-evidence/secq1_metadata6_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_3.body](stack3-live-20261105/live2-evidence/secq1_metadata6_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_metadata6_3.txt](stack3-live-20261105/live2-evidence/secq1_metadata6_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_post_list.body](stack3-live-20261105/live2-evidence/secq1_post_list.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_post_list.txt](stack3-live-20261105/live2-evidence/secq1_post_list.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_pre_list.body](stack3-live-20261105/live2-evidence/secq1_pre_list.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq1_pre_list.txt](stack3-live-20261105/live2-evidence/secq1_pre_list.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_1.body](stack3-live-20261105/live2-evidence/secq5_get_discover_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_1.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_2.body](stack3-live-20261105/live2-evidence/secq5_get_discover_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_2.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_3.body](stack3-live-20261105/live2-evidence/secq5_get_discover_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_3.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_noauth.body](stack3-live-20261105/live2-evidence/secq5_get_discover_noauth.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_noauth.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_noauth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_querycreds.body](stack3-live-20261105/live2-evidence/secq5_get_discover_querycreds.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_querycreds.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_querycreds.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_user.body](stack3-live-20261105/live2-evidence/secq5_get_discover_user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_discover_user.txt](stack3-live-20261105/live2-evidence/secq5_get_discover_user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_1.body](stack3-live-20261105/live2-evidence/secq5_get_test_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_1.txt](stack3-live-20261105/live2-evidence/secq5_get_test_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_2.body](stack3-live-20261105/live2-evidence/secq5_get_test_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_2.txt](stack3-live-20261105/live2-evidence/secq5_get_test_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_3.body](stack3-live-20261105/live2-evidence/secq5_get_test_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_get_test_3.txt](stack3-live-20261105/live2-evidence/secq5_get_test_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_1.body](stack3-live-20261105/live2-evidence/secq5_post_discover_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_1.txt](stack3-live-20261105/live2-evidence/secq5_post_discover_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_2.body](stack3-live-20261105/live2-evidence/secq5_post_discover_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_2.txt](stack3-live-20261105/live2-evidence/secq5_post_discover_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_3.body](stack3-live-20261105/live2-evidence/secq5_post_discover_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_3.txt](stack3-live-20261105/live2-evidence/secq5_post_discover_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_creds.body](stack3-live-20261105/live2-evidence/secq5_post_discover_creds.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_creds.txt](stack3-live-20261105/live2-evidence/secq5_post_discover_creds.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_user.body](stack3-live-20261105/live2-evidence/secq5_post_discover_user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_discover_user.txt](stack3-live-20261105/live2-evidence/secq5_post_discover_user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_1.body](stack3-live-20261105/live2-evidence/secq5_post_test_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_1.txt](stack3-live-20261105/live2-evidence/secq5_post_test_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_2.body](stack3-live-20261105/live2-evidence/secq5_post_test_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_2.txt](stack3-live-20261105/live2-evidence/secq5_post_test_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_3.body](stack3-live-20261105/live2-evidence/secq5_post_test_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_3.txt](stack3-live-20261105/live2-evidence/secq5_post_test_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_user.body](stack3-live-20261105/live2-evidence/secq5_post_test_user.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/secq5_post_test_user.txt](stack3-live-20261105/live2-evidence/secq5_post_test_user.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_admin.body](stack3-live-20261105/live2-evidence/sub_del_admin.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_admin.txt](stack3-live-20261105/live2-evidence/sub_del_admin.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_1.body](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_1.txt](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_2.body](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_2.txt](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_3.body](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_3.txt](stack3-live-20261105/live2-evidence/sub_del_existing_nonowner_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_noauth.body](stack3-live-20261105/live2-evidence/sub_del_noauth.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_noauth.txt](stack3-live-20261105/live2-evidence/sub_del_noauth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_1.body](stack3-live-20261105/live2-evidence/sub_del_user_1.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_1.txt](stack3-live-20261105/live2-evidence/sub_del_user_1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_2.body](stack3-live-20261105/live2-evidence/sub_del_user_2.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_2.txt](stack3-live-20261105/live2-evidence/sub_del_user_2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_3.body](stack3-live-20261105/live2-evidence/sub_del_user_3.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_del_user_3.txt](stack3-live-20261105/live2-evidence/sub_del_user_3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/sub_existing_still_present.txt](stack3-live-20261105/live2-evidence/sub_existing_still_present.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_after.body](stack3-live-20261105/live2-evidence/user_after.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_after.txt](stack3-live-20261105/live2-evidence/user_after.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_create.body](stack3-live-20261105/live2-evidence/user_create.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_create.txt](stack3-live-20261105/live2-evidence/user_create.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_delete.body](stack3-live-20261105/live2-evidence/user_delete.body) | (file; not read, listed by name only) |
| [stack3-live-20261105/live2-evidence/user_delete.txt](stack3-live-20261105/live2-evidence/user_delete.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/MANIFEST.sha256](stack3-rebuild-live-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [stack3-rebuild-live-20261105/README.md](stack3-rebuild-live-20261105/README.md) | Stack 3 rebuild + live re-verification (2026-11-05) - Base: main d9238b4743c85dbb9c987617bb4409041f41e840. Operator: Sonnet QA agent. No product code touched; evidence only. |
| [stack3-rebuild-live-20261105/build_end.txt](stack3-rebuild-live-20261105/build_end.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/build_output.txt](stack3-rebuild-live-20261105/build_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/build_start.txt](stack3-rebuild-live-20261105/build_start.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/down_output.txt](stack3-rebuild-live-20261105/down_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/h/burst_expr.txt](stack3-rebuild-live-20261105/h/burst_expr.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/h/burst_late35.srt](stack3-rebuild-live-20261105/h/burst_late35.srt) | (file; not read, listed by name only) |
| [stack3-rebuild-live-20261105/h/live3.py](stack3-rebuild-live-20261105/h/live3.py) | (Python script; not read, listed by name only) |
| [stack3-rebuild-live-20261105/h/results.jsonl](stack3-rebuild-live-20261105/h/results.jsonl) | (file; not read, listed by name only) |
| [stack3-rebuild-live-20261105/h/scan.py](stack3-rebuild-live-20261105/h/scan.py) | (Python script; not read, listed by name only) |
| [stack3-rebuild-live-20261105/health.json](stack3-rebuild-live-20261105/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/images_after.txt](stack3-rebuild-live-20261105/images_after.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/images_before.txt](stack3-rebuild-live-20261105/images_before.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/live_synology.txt](stack3-rebuild-live-20261105/live_synology.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/live_yandex.txt](stack3-rebuild-live-20261105/live_yandex.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/pg_migrations.txt](stack3-rebuild-live-20261105/pg_migrations.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/results_table.md](stack3-rebuild-live-20261105/results_table.md) | results_table.md |
| [stack3-rebuild-live-20261105/run_r1.txt](stack3-rebuild-live-20261105/run_r1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/run_r2.txt](stack3-rebuild-live-20261105/run_r2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/run_r3.txt](stack3-rebuild-live-20261105/run_r3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/up_output.txt](stack3-rebuild-live-20261105/up_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-rebuild-live-20261105/yandex_evidence.json](stack3-rebuild-live-20261105/yandex_evidence.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-ui-20261105/MANIFEST.screenshots.sha256](stack3-ui-20261105/MANIFEST.screenshots.sha256) | (file; not read, listed by name only) |
| [stack3-ui-20261105/MANIFEST.sha256](stack3-ui-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [stack3-ui-20261105/README.md](stack3-ui-20261105/README.md) | Stack 3 live browser verification (UI), 2026-11-05 - Roadmap-Item: CAT-QA-STACK3-UI. Image main-5c480072 (QA stack 3). Real stack, real browser (headless Chromium via playwright-core), no mocks (section 11.4.289). Web UI... |
| [stack3-ui-20261105/results-app.json](stack3-ui-20261105/results-app.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-ui-20261105/results-entity.json](stack3-ui-20261105/results-entity.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-ui-20261105/results-mini.json](stack3-ui-20261105/results-mini.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-ui-20261105/results-site.json](stack3-ui-20261105/results-site.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/MANIFEST.sha256](stack3-verify-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [stack3-verify-20261105/README.md](stack3-verify-20261105/README.md) | Stack 3 rebuild + live re-verification of the security queue and monitoring add-on (2026-11-05) - Base: main 3293e280. Authoring commits on branch qa/stack3-verify-20261105 of a scratch clone (head recorded in the hand-back, never pushed). |
| [stack3-verify-20261105/api_env_names.txt](stack3-verify-20261105/api_env_names.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/api_trusted_proxies_value.txt](stack3-verify-20261105/api_trusted_proxies_value.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/build_output.txt](stack3-verify-20261105/build_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/build_start.txt](stack3-verify-20261105/build_start.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/clientip_probe.txt](stack3-verify-20261105/clientip_probe.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/clientip_probe_after.txt](stack3-verify-20261105/clientip_probe_after.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/clientip_probe_final.txt](stack3-verify-20261105/clientip_probe_final.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/containers_final.txt](stack3-verify-20261105/containers_final.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/cover_run1.txt](stack3-verify-20261105/cover_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/cover_services_run1.txt](stack3-verify-20261105/cover_services_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/cover_services_run2.txt](stack3-verify-20261105/cover_services_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/cover_services_run3.txt](stack3-verify-20261105/cover_services_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/ffprobe_guarded.txt](stack3-verify-20261105/ffprobe_guarded.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/ffprobe_guarded_control.txt](stack3-verify-20261105/ffprobe_guarded_control.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/ffprobe_unguarded.txt](stack3-verify-20261105/ffprobe_unguarded.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/gorun.sh](stack3-verify-20261105/gorun.sh) | (shell script; not read, listed by name only) |
| [stack3-verify-20261105/gotestc_output.txt](stack3-verify-20261105/gotestc_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/health_before.json](stack3-verify-20261105/health_before.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/health_final.json](stack3-verify-20261105/health_final.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/image_ids.txt](stack3-verify-20261105/image_ids.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/lib.py](stack3-verify-20261105/lib.py) | (Python script; not read, listed by name only) |
| [stack3-verify-20261105/live3.py](stack3-verify-20261105/live3.py) | (Python script; not read, listed by name only) |
| [stack3-verify-20261105/live3_lib.py](stack3-verify-20261105/live3_lib.py) | (Python script; not read, listed by name only) |
| [stack3-verify-20261105/live3_run1.txt](stack3-verify-20261105/live3_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/live3_run2.txt](stack3-verify-20261105/live3_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/live3_run3.txt](stack3-verify-20261105/live3_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/metrics_auth.txt](stack3-verify-20261105/metrics_auth.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/mon_up_output.txt](stack3-verify-20261105/mon_up_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/mon_up_r2_output.txt](stack3-verify-20261105/mon_up_r2_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/monitoring_verify_run1.txt](stack3-verify-20261105/monitoring_verify_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/monitoring_verify_run2.txt](stack3-verify-20261105/monitoring_verify_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/monitoring_verify_run3.txt](stack3-verify-20261105/monitoring_verify_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/new1_code_select.txt](stack3-verify-20261105/new1_code_select.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/new1_query_columns.txt](stack3-verify-20261105/new1_query_columns.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/ports_catqa3.txt](stack3-verify-20261105/ports_catqa3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/ports_catqa3mon.txt](stack3-verify-20261105/ports_catqa3mon.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/prom_targets.txt](stack3-verify-20261105/prom_targets.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/prom_targets_final.txt](stack3-verify-20261105/prom_targets_final.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/realbinary_run1.txt](stack3-verify-20261105/realbinary_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/realbinary_run2.txt](stack3-verify-20261105/realbinary_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/realbinary_run3.txt](stack3-verify-20261105/realbinary_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/red_monitoring_test_scripts.txt](stack3-verify-20261105/red_monitoring_test_scripts.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/red_stack3_test_scripts.txt](stack3-verify-20261105/red_stack3_test_scripts.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_1.json](stack3-verify-20261105/res_A_1.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_2.json](stack3-verify-20261105/res_A_2.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_3.json](stack3-verify-20261105/res_A_3.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_4.json](stack3-verify-20261105/res_A_4.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_5.json](stack3-verify-20261105/res_A_5.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_A_6.json](stack3-verify-20261105/res_A_6.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_B_1.json](stack3-verify-20261105/res_B_1.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_B_2.json](stack3-verify-20261105/res_B_2.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/res_B_3.json](stack3-verify-20261105/res_B_3.json) | (JSON data artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A1.txt](stack3-verify-20261105/run_A1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A2.txt](stack3-verify-20261105/run_A2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A3.txt](stack3-verify-20261105/run_A3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A4.txt](stack3-verify-20261105/run_A4.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A5.txt](stack3-verify-20261105/run_A5.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_A6.txt](stack3-verify-20261105/run_A6.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_B1.txt](stack3-verify-20261105/run_B1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_B2.txt](stack3-verify-20261105/run_B2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/run_B3.txt](stack3-verify-20261105/run_B3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/sec.py](stack3-verify-20261105/sec.py) | (Python script; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_mw_run1.txt](stack3-verify-20261105/unit_logredact_mw_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_mw_run2.txt](stack3-verify-20261105/unit_logredact_mw_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_mw_run3.txt](stack3-verify-20261105/unit_logredact_mw_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_run1.txt](stack3-verify-20261105/unit_logredact_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_run2.txt](stack3-verify-20261105/unit_logredact_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_logredact_run3.txt](stack3-verify-20261105/unit_logredact_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_middleware_run1.txt](stack3-verify-20261105/unit_middleware_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_middleware_run2.txt](stack3-verify-20261105/unit_middleware_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_middleware_run3.txt](stack3-verify-20261105/unit_middleware_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_monitoring_run1.txt](stack3-verify-20261105/unit_monitoring_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_monitoring_run2.txt](stack3-verify-20261105/unit_monitoring_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/unit_monitoring_run3.txt](stack3-verify-20261105/unit_monitoring_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up2_output.txt](stack3-verify-20261105/up2_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up3_output.txt](stack3-verify-20261105/up3_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_final2_output.txt](stack3-verify-20261105/up_final2_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_final_output.txt](stack3-verify-20261105/up_final_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_output.txt](stack3-verify-20261105/up_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_pprof2_output.txt](stack3-verify-20261105/up_pprof2_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_pprof_output.txt](stack3-verify-20261105/up_pprof_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_r2_output.txt](stack3-verify-20261105/up_r2_output.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/up_start.txt](stack3-verify-20261105/up_start.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3-verify-20261105/url_status_final.txt](stack3-verify-20261105/url_status_final.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/README.md](stack3/README.md) | Catalogizer QA stack 3 - Rootless-podman QA stack built from a clean export of a main commit (default: the repo HEAD). Independent of stacks |
| [stack3/build.sh](stack3/build.sh) | (shell script; not read, listed by name only) |
| [stack3/down.sh](stack3/down.sh) | (shell script; not read, listed by name only) |
| [stack3/evidence-lifecycle-20261111/probe_containers_backend.txt](stack3/evidence-lifecycle-20261111/probe_containers_backend.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence-lifecycle-20261111/probe_podman_fallback_backend.txt](stack3/evidence-lifecycle-20261111/probe_podman_fallback_backend.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/health_body.json](stack3/evidence/health_body.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/evidence/health_deep.json](stack3/evidence/health_deep.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/evidence/images.txt](stack3/evidence/images.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/lan_status.txt](stack3/evidence/lan_status.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/login_bad.txt](stack3/evidence/login_bad.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/login_via_web.txt](stack3/evidence/login_via_web.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/media_and_tools.txt](stack3/evidence/media_and_tools.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/other_stacks_ports.txt](stack3/evidence/other_stacks_ports.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/evidence/up_after_down.txt](stack3/evidence/up_after_down.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/golden/up_create_argv.txt](stack3/golden/up_create_argv.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/.gitignore](stack3/helixqa/.gitignore) | (file; not read, listed by name only) |
| [stack3/helixqa/README.md](stack3/helixqa/README.md) | HelixQA against QA stack 3 - Roadmap-Item: HELIXQA-STACK3 |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/health.json](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.err](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.json](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.txt](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/strict_check.txt](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_401_to_200/verdict.txt](stack3/helixqa/evidence-pack-20261105/M1_401_to_200/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M1_bank.yaml](stack3/helixqa/evidence-pack-20261105/M1_bank.yaml) | (YAML data; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/health.json](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.err](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.json](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.txt](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/strict_check.txt](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/verdict.txt](stack3/helixqa/evidence-pack-20261105/M3_action_typo_GETT/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M3_bank.yaml](stack3/helixqa/evidence-pack-20261105/M3_bank.yaml) | (YAML data; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M4_missing_cred/verdict.txt](stack3/helixqa/evidence-pack-20261105/M4_missing_cred/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/health.json](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.err](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.json](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.txt](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/strict_check.txt](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/verdict.txt](stack3/helixqa/evidence-pack-20261105/M5_fake_binary_exit0/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/health.json](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.err](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.json](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.txt](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/strict_check.txt](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/verdict.txt](stack3/helixqa/evidence-pack-20261105/M6_fake_binary_exit2/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/health.json](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.err](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.json](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.txt](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/strict_check.txt](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/verdict.txt](stack3/helixqa/evidence-pack-20261105/M7_action_typo_htp/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M7_bank.yaml](stack3/helixqa/evidence-pack-20261105/M7_bank.yaml) | (YAML data; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M9_sigterm/health.json](stack3/helixqa/evidence-pack-20261105/M9_sigterm/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M9_sigterm/http_api.err](stack3/helixqa/evidence-pack-20261105/M9_sigterm/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/M9_sigterm/verdict.txt](stack3/helixqa/evidence-pack-20261105/M9_sigterm/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/SUMMARY.txt](stack3/helixqa/evidence-pack-20261105/SUMMARY.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/health.json](stack3/helixqa/evidence-pack-20261105/baseline/health.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/http_api.err](stack3/helixqa/evidence-pack-20261105/baseline/http_api.err) | (file; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/http_api.json](stack3/helixqa/evidence-pack-20261105/baseline/http_api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/http_api.txt](stack3/helixqa/evidence-pack-20261105/baseline/http_api.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/strict_check.txt](stack3/helixqa/evidence-pack-20261105/baseline/strict_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/evidence-pack-20261105/baseline/verdict.txt](stack3/helixqa/evidence-pack-20261105/baseline/verdict.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/helixqa/run.sh](stack3/helixqa/run.sh) | (shell script; not read, listed by name only) |
| [stack3/helixqa/smoke_bank.yaml](stack3/helixqa/smoke_bank.yaml) | (YAML data; not read, listed by name only) |
| [stack3/lib.sh](stack3/lib.sh) | (shell script; not read, listed by name only) |
| [stack3/lifecycle_probe.sh](stack3/lifecycle_probe.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/README.md](stack3/monitoring/README.md) | Stack 3 monitoring add-on (QA) - Prometheus + Grafana + pgAdmin + nginx reverse proxy for the running QA stack 3 (catqa3), as one rootless podman pod |
| [stack3/monitoring/digest_check.sh](stack3/monitoring/digest_check.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/down.sh](stack3/monitoring/down.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/evidence/MANIFEST.sha256](stack3/monitoring/evidence/MANIFEST.sha256) | (file; not read, listed by name only) |
| [stack3/monitoring/evidence/image_digests.txt](stack3/monitoring/evidence/image_digests.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/inspect_leak_scan.txt](stack3/monitoring/evidence/inspect_leak_scan.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/pod_reachability_before_after.txt](stack3/monitoring/evidence/pod_reachability_before_after.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/pod_state.txt](stack3/monitoring/evidence/pod_state.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/prometheus_targets.txt](stack3/monitoring/evidence/prometheus_targets.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/resource_usage.txt](stack3/monitoring/evidence/resource_usage.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/secret_leak_scan.txt](stack3/monitoring/evidence/secret_leak_scan.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/stack3_containers_unchanged_check.txt](stack3/monitoring/evidence/stack3_containers_unchanged_check.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/test_scripts_GREEN.txt](stack3/monitoring/evidence/test_scripts_GREEN.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/test_scripts_RED_M7_umask022_mutant.txt](stack3/monitoring/evidence/test_scripts_RED_M7_umask022_mutant.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/test_scripts_RED_N2_failopen_mutant.txt](stack3/monitoring/evidence/test_scripts_RED_N2_failopen_mutant.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/test_scripts_RED_prefix_refresher.txt](stack3/monitoring/evidence/test_scripts_RED_prefix_refresher.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/token_file_mode.txt](stack3/monitoring/evidence/token_file_mode.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/token_refresher_log.txt](stack3/monitoring/evidence/token_refresher_log.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/evidence/verify_round1.txt](stack3/monitoring/evidence/verify_round1.txt) | (plain-text artefact; not read, listed by name only) |
| [stack3/monitoring/grafana/dashboards/catalogizer-api.json](stack3/monitoring/grafana/dashboards/catalogizer-api.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/monitoring/grafana/provisioning/dashboards/dashboards.yml](stack3/monitoring/grafana/provisioning/dashboards/dashboards.yml) | (YAML data; not read, listed by name only) |
| [stack3/monitoring/grafana/provisioning/datasources/prometheus.yml](stack3/monitoring/grafana/provisioning/datasources/prometheus.yml) | (YAML data; not read, listed by name only) |
| [stack3/monitoring/lanbind.py](stack3/monitoring/lanbind.py) | (Python script; not read, listed by name only) |
| [stack3/monitoring/lib.sh](stack3/monitoring/lib.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/nginx/nginx.conf](stack3/monitoring/nginx/nginx.conf) | (file; not read, listed by name only) |
| [stack3/monitoring/pgadmin/servers.json](stack3/monitoring/pgadmin/servers.json) | (JSON data artefact; not read, listed by name only) |
| [stack3/monitoring/prometheus/prometheus.yml](stack3/monitoring/prometheus/prometheus.yml) | (YAML data; not read, listed by name only) |
| [stack3/monitoring/seed_scrape_token.sh](stack3/monitoring/seed_scrape_token.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_clientip_frontdoor.sh](stack3/monitoring/test_clientip_frontdoor.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_frontdoor_interface.sh](stack3/monitoring/test_frontdoor_interface.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_lanbind.py](stack3/monitoring/test_lanbind.py) | (Python script; not read, listed by name only) |
| [stack3/monitoring/test_prom_auth.sh](stack3/monitoring/test_prom_auth.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_prom_auth_grafana.sh](stack3/monitoring/test_prom_auth_grafana.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_prom_user_validation.sh](stack3/monitoring/test_prom_user_validation.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/test_scripts.sh](stack3/monitoring/test_scripts.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/token_refresher.sh](stack3/monitoring/token_refresher.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/up.sh](stack3/monitoring/up.sh) | (shell script; not read, listed by name only) |
| [stack3/monitoring/verify.py](stack3/monitoring/verify.py) | (Python script; not read, listed by name only) |
| [stack3/s3/README.md](stack3/s3/README.md) | Live S3 server for Catalogizer (CAT-S3-1) - Real Garage v1.0.1 (docker.io/dxflrs/garage) under rootless podman, --memory 1g --memory-swap 1g --cpus 1, own network catqa-s3live, |
| [stack3/s3/down.sh](stack3/s3/down.sh) | (shell script; not read, listed by name only) |
| [stack3/s3/test_down_purge.sh](stack3/s3/test_down_purge.sh) | (shell script; not read, listed by name only) |
| [stack3/s3/up.sh](stack3/s3/up.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/README.md](stack3/sonarqube/README.md) | SonarQube QA service (on demand) - Static-analysis QA for catalog-api (Go) and catalog-web (TypeScript). It is started for a scan window and stopped |
| [stack3/sonarqube/down.sh](stack3/sonarqube/down.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/lib.sh](stack3/sonarqube/lib.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/mutation_guards.sh](stack3/sonarqube/mutation_guards.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/scan.sh](stack3/sonarqube/scan.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/sonar-catalog-api.properties](stack3/sonarqube/sonar-catalog-api.properties) | (file; not read, listed by name only) |
| [stack3/sonarqube/sonar-catalog-web.properties](stack3/sonarqube/sonar-catalog-web.properties) | (file; not read, listed by name only) |
| [stack3/sonarqube/test_guards.sh](stack3/sonarqube/test_guards.sh) | (shell script; not read, listed by name only) |
| [stack3/sonarqube/up.sh](stack3/sonarqube/up.sh) | (shell script; not read, listed by name only) |
| [stack3/test_ctrctl_routing.sh](stack3/test_ctrctl_routing.sh) | (shell script; not read, listed by name only) |
| [stack3/test_ctrctl_routing_mutations.sh](stack3/test_ctrctl_routing_mutations.sh) | (shell script; not read, listed by name only) |
| [stack3/test_lan_defaults.sh](stack3/test_lan_defaults.sh) | (shell script; not read, listed by name only) |
| [stack3/test_lan_defaults_mutations.sh](stack3/test_lan_defaults_mutations.sh) | (shell script; not read, listed by name only) |
| [stack3/test_scripts.sh](stack3/test_scripts.sh) | (shell script; not read, listed by name only) |
| [stack3/up.sh](stack3/up.sh) | (shell script; not read, listed by name only) |
| [storage-clients-fix-20261104/MANIFEST.txt](storage-clients-fix-20261104/MANIFEST.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/README.md](storage-clients-fix-20261104/README.md) | Storage clients fix (ST-D1, ST-D2, ST-D3), PROVISIONAL-DONE - Roadmap-Item: CAT-ST. Base main 4d04560f. Files: catalog-api/filesystem/{ftp_client.go,ftp_ops.go,smb_client.go} + tests ftp_realserver_live_test.go,... |
| [storage-clients-fix-20261104/full_package_verbose_race.txt](storage-clients-fix-20261104/full_package_verbose_race.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/green_filesystem_package_race.txt](storage-clients-fix-20261104/green_filesystem_package_race.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/green_go_vet.txt](storage-clients-fix-20261104/green_go_vet.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/green_new_tests_race_x3.txt](storage-clients-fix-20261104/green_new_tests_race_x3.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/mutations_round2.txt](storage-clients-fix-20261104/mutations_round2.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/mutations_round3.txt](storage-clients-fix-20261104/mutations_round3.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/mutations_round4.txt](storage-clients-fix-20261104/mutations_round4.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/red_review_r1_probes.txt](storage-clients-fix-20261104/red_review_r1_probes.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/red_review_r2.txt](storage-clients-fix-20261104/red_review_r2.txt) | (plain-text artefact; not read, listed by name only) |
| [storage-clients-fix-20261104/red_review_r3.txt](storage-clients-fix-20261104/red_review_r3.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/MANIFEST.txt](subtitle-defects-fix-20261104/MANIFEST.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/green_full.txt](subtitle-defects-fix-20261104/green_full.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/green_vet.txt](subtitle-defects-fix-20261104/green_vet.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/mutations.txt](subtitle-defects-fix-20261104/mutations.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/pg_v24.txt](subtitle-defects-fix-20261104/pg_v24.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/red_handlers.txt](subtitle-defects-fix-20261104/red_handlers.txt) | (plain-text artefact; not read, listed by name only) |
| [subtitle-defects-fix-20261104/red_services.txt](subtitle-defects-fix-20261104/red_services.txt) | (plain-text artefact; not read, listed by name only) |
| [tauri-boot-20261105/MANIFEST.screenshots.sha256](tauri-boot-20261105/MANIFEST.screenshots.sha256) | (file; not read, listed by name only) |
| [tauri-boot-20261105/README.md](tauri-boot-20261105/README.md) | CAT-TAURI-1: Tauri desktop + installer-wizard boot for manual QA (2026-11-05) - catalogizer-desktop ELF x86-64 16379216 B, sha256 0e7cee0ec3d9f92edd1c1dd44c992d39d295182a790d722d55dc28ac1e261eca. |
| [tauri-boot-20261105/dockerfile_hash_checks.txt](tauri-boot-20261105/dockerfile_hash_checks.txt) | (plain-text artefact; not read, listed by name only) |
| [tauri-boot-20261105/drive_ui.cjs](tauri-boot-20261105/drive_ui.cjs) | (file; not read, listed by name only) |
| [tauri-boot-20261105/memory_limits.txt](tauri-boot-20261105/memory_limits.txt) | (plain-text artefact; not read, listed by name only) |
| [tauri-boot-20261105/result.json](tauri-boot-20261105/result.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/README.md](tauri-native-20261106/README.md) | CAT-TAURI-NATIVE-1: native Tauri QA evidence (2026-11-06) - Companion of [../tauri-boot-20261105/README.md](tauri-boot-20261105/README.md). This folder carries the TEXT evidence |
| [tauri-native-20261106/close_x/checks.json](tauri-native-20261106/close_x/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/close_x/run_log.json](tauri-native-20261106/close_x/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/dev_run_checks_C6_C8_C9_expects_wrong/checks.json](tauri-native-20261106/dev_run_checks_C6_C8_C9_expects_wrong/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/dev_run_checks_C6_C8_C9_expects_wrong/run_log.json](tauri-native-20261106/dev_run_checks_C6_C8_C9_expects_wrong/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/mutant_R5/run_log.json](tauri-native-20261106/mutant_R5/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run1/checks.json](tauri-native-20261106/run1/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run1/run_log.json](tauri-native-20261106/run1/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run2/checks.json](tauri-native-20261106/run2/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run2/run_log.json](tauri-native-20261106/run2/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run3/checks.json](tauri-native-20261106/run3/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/run3/run_log.json](tauri-native-20261106/run3/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/scripts/close_x.py](tauri-native-20261106/scripts/close_x.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/mutant_check.py](tauri-native-20261106/scripts/mutant_check.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/qa.py](tauri-native-20261106/scripts/qa.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/runall.sh](tauri-native-20261106/scripts/runall.sh) | (shell script; not read, listed by name only) |
| [tauri-native-20261106/scripts/runwz.sh](tauri-native-20261106/scripts/runwz.sh) | (shell script; not read, listed by name only) |
| [tauri-native-20261106/scripts/scenario.py](tauri-native-20261106/scripts/scenario.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/scenario_lib.py](tauri-native-20261106/scripts/scenario_lib.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/wz_lib.py](tauri-native-20261106/scripts/wz_lib.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/scripts/wz_scenario.py](tauri-native-20261106/scripts/wz_scenario.py) | (Python script; not read, listed by name only) |
| [tauri-native-20261106/secret_scan.txt](tauri-native-20261106/secret_scan.txt) | (plain-text artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run1/checks.json](tauri-native-20261106/wz_red_run1/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run1/run_log.json](tauri-native-20261106/wz_red_run1/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run2/checks.json](tauri-native-20261106/wz_red_run2/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run2/run_log.json](tauri-native-20261106/wz_red_run2/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run3/checks.json](tauri-native-20261106/wz_red_run3/checks.json) | (JSON data artefact; not read, listed by name only) |
| [tauri-native-20261106/wz_red_run3/run_log.json](tauri-native-20261106/wz_red_run3/run_log.json) | (JSON data artefact; not read, listed by name only) |
| [test-servers/Containerfile](test-servers/Containerfile) | (file; not read, listed by name only) |
| [test-servers/README.md](test-servers/README.md) | Catalogizer live test servers (host anton; LAN address auto-detected, override with TS_HOST or QA_LAN_IP) - All serve the SAME corpus /home/milosvasic/Testing-servers/corpus (manifest.sha256, 12 files incl. unicode/space names, 0-byte, 5 MB, wav/mp3/mp4/srt/jpg/png/pdf/txt).... |
| [test-servers/down.sh](test-servers/down.sh) | (shell script; not read, listed by name only) |
| [test-servers/ftps_server.py](test-servers/ftps_server.py) | (Python script; not read, listed by name only) |
| [test-servers/ports.env](test-servers/ports.env) | (file; not read, listed by name only) |
| [test-servers/smb.conf](test-servers/smb.conf) | (file; not read, listed by name only) |
| [test-servers/test_up_cert.sh](test-servers/test_up_cert.sh) | (shell script; not read, listed by name only) |
| [test-servers/test_up_cert_mutations.sh](test-servers/test_up_cert_mutations.sh) | (shell script; not read, listed by name only) |
| [test-servers/up.sh](test-servers/up.sh) | (shell script; not read, listed by name only) |
| [test-servers/verify.sh](test-servers/verify.sh) | (shell script; not read, listed by name only) |
| [ui-d2d3-20261105/README.md](ui-d2d3-20261105/README.md) | UI-D2 / UI-D3 live reproduction (stack 3, 2026-11-05) - Commands (qa_admin token, API http://127.0.0.1:38082/api/v1), observed HTTP codes: |
| [ui-d2d3-20261105/red_entitydetail.txt](ui-d2d3-20261105/red_entitydetail.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d2d3-20261105/red_identitiesapi.txt](ui-d2d3-20261105/red_identitiesapi.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d5d6-20261105/README.md](ui-d5d6-20261105/README.md) | CAT-UI-D5D6: /admin 360px overflow + unnamed controls (axe) - RED (live, QA stack 3 image main-5c480072, qa_admin, headless Chromium + axe-core): red-live-axe.json produced by axe_live.cjs |
| [ui-d5d6-20261105/axe_live.cjs](ui-d5d6-20261105/axe_live.cjs) | (file; not read, listed by name only) |
| [ui-d5d6-20261105/red-live-axe.json](ui-d5d6-20261105/red-live-axe.json) | (JSON data artefact; not read, listed by name only) |
| [ui-d7-class-20261105/GREEN_affected.log](ui-d7-class-20261105/GREEN_affected.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/GREEN_affected2.log](ui-d7-class-20261105/GREEN_affected2.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/GREEN_final.log](ui-d7-class-20261105/GREEN_final.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/README.md](ui-d7-class-20261105/README.md) | UI D7 class-wide dark-mode contrast (D7-R2-F1..F4) - 2026-11-05 - Status: PROVISIONAL (unit-class, jsdom + real Tailwind theme; round 2 after reviewer NO-GO). No live-stack axe re-run was done here. |
| [ui-d7-class-20261105/RED_first_walker_shared.log](ui-d7-class-20261105/RED_first_walker_shared.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/RED_measured_ratios.txt](ui-d7-class-20261105/RED_measured_ratios.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7-class-20261105/RED_populated.log](ui-d7-class-20261105/RED_populated.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/RED_prefix_source.log](ui-d7-class-20261105/RED_prefix_source.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/SHA256SUMS](ui-d7-class-20261105/SHA256SUMS) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M10_walker_skips_all.log](ui-d7-class-20261105/mut_M10_walker_skips_all.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M11_formatconv_revert.log](ui-d7-class-20261105/mut_M11_formatconv_revert.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M12_autom_wrong_theme.log](ui-d7-class-20261105/mut_M12_autom_wrong_theme.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M13_walker_threshold.log](ui-d7-class-20261105/mut_M13_walker_threshold.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M1_collmgr_revert.log](ui-d7-class-20261105/mut_M1_collmgr_revert.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M2_wrong_colour_extint.log](ui-d7-class-20261105/mut_M2_wrong_colour_extint.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M3_wrong_theme_advsearch.log](ui-d7-class-20261105/mut_M3_wrong_theme_advsearch.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M4_mediachart_revert.log](ui-d7-class-20261105/mut_M4_mediachart_revert.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M5_realtime_red.log](ui-d7-class-20261105/mut_M5_realtime_red.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M6_upload_dark_gray700.log](ui-d7-class-20261105/mut_M6_upload_dark_gray700.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M7_settings_revert.log](ui-d7-class-20261105/mut_M7_settings_revert.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M8_entity_orange.log](ui-d7-class-20261105/mut_M8_entity_orange.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_M9_helper_no_conflict_throw.log](ui-d7-class-20261105/mut_M9_helper_no_conflict_throw.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P1_upload_row_no_darkbg.log](ui-d7-class-20261105/mut_P1_upload_row_no_darkbg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P2_extint_expanded_no_darkbg.log](ui-d7-class-20261105/mut_P2_extint_expanded_no_darkbg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P3_preset_no_darkbg.log](ui-d7-class-20261105/mut_P3_preset_no_darkbg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P4_status_default_no_darkbg.log](ui-d7-class-20261105/mut_P4_status_default_no_darkbg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P5_dragover_no_darkbg.log](ui-d7-class-20261105/mut_P5_dragover_no_darkbg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_P6_status_green_500.log](ui-d7-class-20261105/mut_P6_status_green_500.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_R5_walker_skips_own_bg.log](ui-d7-class-20261105/mut_R5_walker_skips_own_bg.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_R6_textlg_large.log](ui-d7-class-20261105/mut_R6_textlg_large.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mut_R7_overlay_unanchored.log](ui-d7-class-20261105/mut_R7_overlay_unanchored.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/mutations_summary.txt](ui-d7-class-20261105/mutations_summary.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7-class-20261105/tsc.log](ui-d7-class-20261105/tsc.log) | (file; not read, listed by name only) |
| [ui-d7-class-20261105/tsc2.log](ui-d7-class-20261105/tsc2.log) | (file; not read, listed by name only) |
| [ui-d7-fu-20261105/README.md](ui-d7-fu-20261105/README.md) | UI-D7-FU (remaining D7 contrast), PROVISIONAL until live axe |
| [ui-d7-fu-20261105/shared_unit_green.txt](ui-d7-fu-20261105/shared_unit_green.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7-fu-20261105/shared_unit_red.txt](ui-d7-fu-20261105/shared_unit_red.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7-fu-20261105/unit_green.txt](ui-d7-fu-20261105/unit_green.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7-fu-20261105/unit_red.txt](ui-d7-fu-20261105/unit_red.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7d8-20261105/README.md](ui-d7d8-20261105/README.md) | UI-D7 / UI-D8 (CAT-UI-D7D8), 2026-11-05 - Live RED on QA stack 3 (image main-5c480072+) before the fix. Credentials never recorded (read from the 0600 env file in a subshell). |
| [ui-d7d8-20261105/d7_axe.cjs](ui-d7d8-20261105/d7_axe.cjs) | (file; not read, listed by name only) |
| [ui-d7d8-20261105/d7_live_red.json](ui-d7d8-20261105/d7_live_red.json) | (JSON data artefact; not read, listed by name only) |
| [ui-d7d8-20261105/d7_live_red.txt](ui-d7d8-20261105/d7_live_red.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7d8-20261105/d7_unit_red.txt](ui-d7d8-20261105/d7_unit_red.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-d7d8-20261105/d8_live_red.json](ui-d7d8-20261105/d8_live_red.json) | (JSON data artefact; not read, listed by name only) |
| [ui-d7d8-20261105/d8_repro.cjs](ui-d7d8-20261105/d8_repro.cjs) | (file; not read, listed by name only) |
| [ui-d7d8-20261105/d8_unit_red.txt](ui-d7d8-20261105/d8_unit_red.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/README.md](ui-live-rerun-20261105-b/README.md) | UI live rerun on QA stack 3, image main-17ec3c1f (2026-11-05, evening) - Target: web http://192.168.1.115:33080 (nginx front door), API :38082, qa_admin. Same scripts as ui-live-rerun-20261105/ (earlier run on main-1db4760f). Chromium... |
| [ui-live-rerun-20261105-b/d1_run1.err](ui-live-rerun-20261105-b/d1_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d1_run1.json](ui-live-rerun-20261105-b/d1_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d1_run2.err](ui-live-rerun-20261105-b/d1_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d1_run2.json](ui-live-rerun-20261105-b/d1_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d1_run3.err](ui-live-rerun-20261105-b/d1_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d1_run3.json](ui-live-rerun-20261105-b/d1_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_live.cjs](ui-live-rerun-20261105-b/d2d3_live.cjs) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run1.err](ui-live-rerun-20261105-b/d2d3_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run1.json](ui-live-rerun-20261105-b/d2d3_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run2.err](ui-live-rerun-20261105-b/d2d3_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run2.json](ui-live-rerun-20261105-b/d2d3_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run3.err](ui-live-rerun-20261105-b/d2d3_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d2d3_run3.json](ui-live-rerun-20261105-b/d2d3_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run1.err](ui-live-rerun-20261105-b/d5d6_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run1.json](ui-live-rerun-20261105-b/d5d6_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run2.err](ui-live-rerun-20261105-b/d5d6_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run2.json](ui-live-rerun-20261105-b/d5d6_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run3.err](ui-live-rerun-20261105-b/d5d6_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d5d6_run3.json](ui-live-rerun-20261105-b/d5d6_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run1.err](ui-live-rerun-20261105-b/d7_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run1.json](ui-live-rerun-20261105-b/d7_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run2.err](ui-live-rerun-20261105-b/d7_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run2.json](ui-live-rerun-20261105-b/d7_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run3.err](ui-live-rerun-20261105-b/d7_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d7_run3.json](ui-live-rerun-20261105-b/d7_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run1.err](ui-live-rerun-20261105-b/d8_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run1.json](ui-live-rerun-20261105-b/d8_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run2.err](ui-live-rerun-20261105-b/d8_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run2.json](ui-live-rerun-20261105-b/d8_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run3.err](ui-live-rerun-20261105-b/d8_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105-b/d8_run3.json](ui-live-rerun-20261105-b/d8_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105-b/pprof_admin_gate.txt](ui-live-rerun-20261105-b/pprof_admin_gate.txt) | (plain-text artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/MANIFEST.sha256](ui-live-rerun-20261105/MANIFEST.sha256) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/README.md](ui-live-rerun-20261105/README.md) | UI live re-run, stack 3 image main-1db4760f (2026-11-05) - Target: web http://192.168.1.115:33020 (D1 via :33080), API :38082, qa_admin. Headless Chromium shell 1228, Playwright 1.64.1, axe-core 4.10.2 |
| [ui-live-rerun-20261105/d1_run1.err](ui-live-rerun-20261105/d1_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d1_run1.json](ui-live-rerun-20261105/d1_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d1_run2.err](ui-live-rerun-20261105/d1_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d1_run2.json](ui-live-rerun-20261105/d1_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d1_run3.err](ui-live-rerun-20261105/d1_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d1_run3.json](ui-live-rerun-20261105/d1_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_live.cjs](ui-live-rerun-20261105/d2d3_live.cjs) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run1.err](ui-live-rerun-20261105/d2d3_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run1.json](ui-live-rerun-20261105/d2d3_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run2.err](ui-live-rerun-20261105/d2d3_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run2.json](ui-live-rerun-20261105/d2d3_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run3.err](ui-live-rerun-20261105/d2d3_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d2d3_run3.json](ui-live-rerun-20261105/d2d3_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run1.err](ui-live-rerun-20261105/d5d6_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run1.json](ui-live-rerun-20261105/d5d6_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run2.err](ui-live-rerun-20261105/d5d6_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run2.json](ui-live-rerun-20261105/d5d6_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run3.err](ui-live-rerun-20261105/d5d6_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d5d6_run3.json](ui-live-rerun-20261105/d5d6_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run1.err](ui-live-rerun-20261105/d7_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run1.json](ui-live-rerun-20261105/d7_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run2.err](ui-live-rerun-20261105/d7_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run2.json](ui-live-rerun-20261105/d7_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run3.err](ui-live-rerun-20261105/d7_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d7_run3.json](ui-live-rerun-20261105/d7_run3.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run1.err](ui-live-rerun-20261105/d8_run1.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run1.json](ui-live-rerun-20261105/d8_run1.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run2.err](ui-live-rerun-20261105/d8_run2.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run2.json](ui-live-rerun-20261105/d8_run2.json) | (JSON data artefact; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run3.err](ui-live-rerun-20261105/d8_run3.err) | (file; not read, listed by name only) |
| [ui-live-rerun-20261105/d8_run3.json](ui-live-rerun-20261105/d8_run3.json) | (JSON data artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/README.md](verifypy-lan-fix-20261111/README.md) | verify.py LAN/pgAdmin fix evidence (CAT-QA-VERIFYPY-1) |
| [verifypy-lan-fix-20261111/test_lanbind_green.txt](verifypy-lan-fix-20261111/test_lanbind_green.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/test_lanbind_green_v2.txt](verifypy-lan-fix-20261111/test_lanbind_green_v2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/test_lanbind_green_v3.txt](verifypy-lan-fix-20261111/test_lanbind_green_v3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/test_lanbind_red_v2.txt](verifypy-lan-fix-20261111/test_lanbind_red_v2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/test_lanbind_red_v3.txt](verifypy-lan-fix-20261111/test_lanbind_red_v3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_declared_run1.txt](verifypy-lan-fix-20261111/verify_fixed_declared_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_declared_run2.txt](verifypy-lan-fix-20261111/verify_fixed_declared_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_declared_run3.txt](verifypy-lan-fix-20261111/verify_fixed_declared_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_run1.txt](verifypy-lan-fix-20261111/verify_fixed_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_run2.txt](verifypy-lan-fix-20261111/verify_fixed_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_fixed_run3.txt](verifypy-lan-fix-20261111/verify_fixed_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_declared_run1.txt](verifypy-lan-fix-20261111/verify_v2_declared_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_declared_run2.txt](verifypy-lan-fix-20261111/verify_v2_declared_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_declared_run3.txt](verifypy-lan-fix-20261111/verify_v2_declared_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_run1.txt](verifypy-lan-fix-20261111/verify_v2_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_run2.txt](verifypy-lan-fix-20261111/verify_v2_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v2_run3.txt](verifypy-lan-fix-20261111/verify_v2_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_declared_run1.txt](verifypy-lan-fix-20261111/verify_v3_declared_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_declared_run2.txt](verifypy-lan-fix-20261111/verify_v3_declared_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_declared_run3.txt](verifypy-lan-fix-20261111/verify_v3_declared_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_run1.txt](verifypy-lan-fix-20261111/verify_v3_run1.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_run2.txt](verifypy-lan-fix-20261111/verify_v3_run2.txt) | (plain-text artefact; not read, listed by name only) |
| [verifypy-lan-fix-20261111/verify_v3_run3.txt](verifypy-lan-fix-20261111/verify_v3_run3.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/README.md](wdf1-repro-20261105/README.md) | WD-F1 / SECQ-2 reproduction attempts (2026-11-05) - Result (closed vocabulary): UNREPRODUCED (a lost acknowledged write was not observed; root cause UNCONFIRMED). Run outcomes: run1, run2, chaos2, chaos3, hardened_live... |
| [wdf1-repro-20261105/chaos1.json](wdf1-repro-20261105/chaos1.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/chaos1.log.txt](wdf1-repro-20261105/chaos1.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/chaos2.json](wdf1-repro-20261105/chaos2.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/chaos2.log.txt](wdf1-repro-20261105/chaos2.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/chaos3.json](wdf1-repro-20261105/chaos3.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/chaos3.log.txt](wdf1-repro-20261105/chaos3.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/hardened_live.json](wdf1-repro-20261105/hardened_live.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/hardened_live.log.txt](wdf1-repro-20261105/hardened_live.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/hardening_review_round_RED_before_fix.log.txt](wdf1-repro-20261105/hardening_review_round_RED_before_fix.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/hardening_unit_RED_before_fix.log.txt](wdf1-repro-20261105/hardening_unit_RED_before_fix.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/mutation_F1_unknown_length_killed_by.txt](wdf1-repro-20261105/mutation_F1_unknown_length_killed_by.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/mutation_F2_head_unsupported_killed_by.txt](wdf1-repro-20261105/mutation_F2_head_unsupported_killed_by.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/run1.json](wdf1-repro-20261105/run1.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/run1.log.txt](wdf1-repro-20261105/run1.log.txt) | (plain-text artefact; not read, listed by name only) |
| [wdf1-repro-20261105/run2.json](wdf1-repro-20261105/run2.json) | (JSON data artefact; not read, listed by name only) |
| [wdf1-repro-20261105/run2.log.txt](wdf1-repro-20261105/run2.log.txt) | (plain-text artefact; not read, listed by name only) |
| [web-live-20261103/stack3/FULL_ARTIFACTS.sha256](web-live-20261103/stack3/FULL_ARTIFACTS.sha256) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_catalog.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_catalog.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_login.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_login.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_management.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_management.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_pages.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_pages.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_playback.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_playback.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_registration.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_registration.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_session.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_session.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_websocket.json](web-live-20261103/stack3/chromium-390/evidence/catalog-web-live_chromium-390_websocket.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run1/artifacts/.last-run.json](web-live-20261103/stack3/chromium-390/run1/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run1/obs/obs.jsonl](web-live-20261103/stack3/chromium-390/run1/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run1/report.json](web-live-20261103/stack3/chromium-390/run1/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run2/artifacts/.last-run.json](web-live-20261103/stack3/chromium-390/run2/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run2/obs/obs.jsonl](web-live-20261103/stack3/chromium-390/run2/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run2/report.json](web-live-20261103/stack3/chromium-390/run2/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run3/artifacts/.last-run.json](web-live-20261103/stack3/chromium-390/run3/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run3/obs/obs.jsonl](web-live-20261103/stack3/chromium-390/run3/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-390/run3/report.json](web-live-20261103/stack3/chromium-390/run3/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_catalog.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_catalog.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_login.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_login.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_management.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_management.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_pages.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_pages.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_playback.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_playback.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_registration.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_registration.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_session.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_session.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_websocket.json](web-live-20261103/stack3/chromium-desktop/evidence/catalog-web-live_chromium-desktop_websocket.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run1/artifacts/.last-run.json](web-live-20261103/stack3/chromium-desktop/run1/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run1/obs/obs.jsonl](web-live-20261103/stack3/chromium-desktop/run1/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run1/report.json](web-live-20261103/stack3/chromium-desktop/run1/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run2/artifacts/.last-run.json](web-live-20261103/stack3/chromium-desktop/run2/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run2/obs/obs.jsonl](web-live-20261103/stack3/chromium-desktop/run2/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run2/report.json](web-live-20261103/stack3/chromium-desktop/run2/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run3/artifacts/.last-run.json](web-live-20261103/stack3/chromium-desktop/run3/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run3/obs/obs.jsonl](web-live-20261103/stack3/chromium-desktop/run3/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/chromium-desktop/run3/report.json](web-live-20261103/stack3/chromium-desktop/run3/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_catalog.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_catalog.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_login.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_login.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_management.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_management.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_pages.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_pages.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_playback.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_playback.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_registration.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_registration.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_session.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_session.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_websocket.json](web-live-20261103/stack3/firefox-390/evidence/catalog-web-live_firefox-390_websocket.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run1/artifacts/.last-run.json](web-live-20261103/stack3/firefox-390/run1/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run1/obs/obs.jsonl](web-live-20261103/stack3/firefox-390/run1/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run1/report.json](web-live-20261103/stack3/firefox-390/run1/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run2/artifacts/.last-run.json](web-live-20261103/stack3/firefox-390/run2/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run2/obs/obs.jsonl](web-live-20261103/stack3/firefox-390/run2/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run2/report.json](web-live-20261103/stack3/firefox-390/run2/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run3/artifacts/.last-run.json](web-live-20261103/stack3/firefox-390/run3/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run3/obs/obs.jsonl](web-live-20261103/stack3/firefox-390/run3/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-390/run3/report.json](web-live-20261103/stack3/firefox-390/run3/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_catalog.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_catalog.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_login.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_login.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_management.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_management.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_pages.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_pages.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_playback.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_playback.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_registration.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_registration.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_session.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_session.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_websocket.json](web-live-20261103/stack3/firefox-desktop/evidence/catalog-web-live_firefox-desktop_websocket.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run1/artifacts/.last-run.json](web-live-20261103/stack3/firefox-desktop/run1/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run1/obs/obs.jsonl](web-live-20261103/stack3/firefox-desktop/run1/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run1/report.json](web-live-20261103/stack3/firefox-desktop/run1/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run2/artifacts/.last-run.json](web-live-20261103/stack3/firefox-desktop/run2/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run2/obs/obs.jsonl](web-live-20261103/stack3/firefox-desktop/run2/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run2/report.json](web-live-20261103/stack3/firefox-desktop/run2/report.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run3/artifacts/.last-run.json](web-live-20261103/stack3/firefox-desktop/run3/artifacts/.last-run.json) | (JSON data artefact; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run3/obs/obs.jsonl](web-live-20261103/stack3/firefox-desktop/run3/obs/obs.jsonl) | (file; not read, listed by name only) |
| [web-live-20261103/stack3/firefox-desktop/run3/report.json](web-live-20261103/stack3/firefox-desktop/run3/report.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/RED_pre_fix_client_run1.json](webdav-yandex-live-20261103/RED_pre_fix_client_run1.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/run1.json](webdav-yandex-live-20261103/run1.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/run2.json](webdav-yandex-live-20261103/run2.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/run3.json](webdav-yandex-live-20261103/run3.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/run_N3_stricter_verdict_PASS_WITH_ANOMALIES.log](webdav-yandex-live-20261103/run_N3_stricter_verdict_PASS_WITH_ANOMALIES.log) | (file; not read, listed by name only) |
| [webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log](webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log) | (file; not read, listed by name only) |
| [webdav-yandex-live-20261103/run_stricter_verdict_attempt1_FAIL_tls_handshake_timeout.log](webdav-yandex-live-20261103/run_stricter_verdict_attempt1_FAIL_tls_handshake_timeout.log) | (file; not read, listed by name only) |
| [webdav-yandex-live-20261103/run_stricter_verdict_attempt2_FAIL_idle_cancel_and_EOF.log](webdav-yandex-live-20261103/run_stricter_verdict_attempt2_FAIL_idle_cancel_and_EOF.log) | (file; not read, listed by name only) |
| [webdav-yandex-live-20261103/scan_leg_run.json](webdav-yandex-live-20261103/scan_leg_run.json) | (JSON data artefact; not read, listed by name only) |
| [webdav-yandex-live-20261103/scan_leg_run.log](webdav-yandex-live-20261103/scan_leg_run.log) | (file; not read, listed by name only) |
