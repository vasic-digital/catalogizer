# Android QA boot, 2026-11-05 (Roadmap-Item CAT-AND-1)

## Build (rootless podman, JDK 17, Constitution 11.4.173 container build; remote offload not done here)
1. `podman build -t catalogizer-android-builder:jdk17 -f docker/Dockerfile.android4 docker` (log: image-build.log)
2. `docs/qa/android-boot-20261105/build_apk.sh catalogizer-android <gradle-home-dir>` then the same for `catalogizer-androidtv`. Run them one after the other, never in parallel (shared gradle home lock; see git history of this dir).
   Outputs: `<module>/app/build/outputs/apk/debug/app-debug.apk`, sha256 in `apk-sha256.txt`.
Root causes fixed:
- catalogizer-android compiled with Java 21 targets (`invalid source release: 21` on JDK 17, log `build-android-RED-jdk21-target.log`); now 17, consistent with androidtv and gradle.properties.
- catalogizer-androidtv needs a `google-services.json` (gitignored, log `build-androidtv-RED-google-services-missing.log`). For local QA a PLACEHOLDER file was created at `catalogizer-androidtv/app/google-services.json` (project `catalogizer-local-qa-placeholder`, key `AIzaSyPLACEHOLDER-...`, 39 chars because Firebase validates the key format and crashes the app at start otherwise). It is NOT committed; Firebase/Crashlytics reports do not work with it. Real builds need the real file.
- Debug signing key is generated per container run (home is not persistent), so a rebuilt APK cannot update an installed one: `adb uninstall` first. Mount a persistent `~/.android` into the container to avoid this (not done).
## Emulators (host, KVM, headless)
SDK images installed into /home/milosvasic/Android/Sdk: `system-images;android-34;default;x86_64`, `system-images;android-34;android-tv;x86` (log sdk-install-images.log).
AVDs (created with the host SDK's avdmanager inside the container, in a scratch AVD home): phone `qa_phone` (pixel_6), TV `qa_tv` (tv_1080p). See avd-create.log.
Boot: `ANDROID_SDK_ROOT=$HOME/Android/Sdk ANDROID_AVD_HOME=<avd dir> emulator -avd qa_phone -port 5584 -no-window -no-audio -no-boot-anim -gpu swiftshader_indirect -memory 2560 -cores 3 -no-snapshot -accel on` (TV: `-avd qa_tv -port 5586`). Wrap in `systemd-run --user --scope -p MemoryMax=5G -p MemorySwapMax=0`.
Evidence: boot-phone.txt, boot-tv.txt, phone-launch.png (login screen), phone-urlentered.png / phone-connected.png (server URL http://192.168.1.115:38082 entered via the in-app Server URL field + Connect; app shows "Connected: ..."), tv-launch.png (TV login screen), logcat-*.txt.
The QA server URL is a runtime setting in the login screen's Server URL field (debug BuildConfig API_BASE_URL is empty by design); nothing is hardcoded.
Sign-in with qa_admin was NOT performed (password would appear in adb process arguments); UNCONFIRMED.
## Human QA display over LAN
adb and the emulator console bind to 127.0.0.1 only (default); do not expose them on 0.0.0.0 (unauthenticated device control).
Option A (recommended): from the QA workstation, `ssh -L 5037:127.0.0.1:5037 <host>` is awkward; simpler: `ssh -L 5585:127.0.0.1:5585 <host>` then `adb connect 127.0.0.1:5585` and `scrcpy -s 127.0.0.1:5585` (emulator adb port = console port + 1; 5585 phone, 5587 TV). Needs the host user's SSH access.
Option B: run the emulator with a window under VNC/noVNC on the host, bound to localhost and reached through an SSH tunnel. Not set up here.
Both are documented, not verified from a second machine (UNCONFIRMED).
