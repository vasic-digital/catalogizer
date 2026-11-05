#!/bin/bash
# Usage: build_apk.sh <catalogizer-android|catalogizer-androidtv> <gradle-home-dir>
set -eu
MOD="${1:?module}"; GH="${2:?gradle home}"
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
systemd-run --user --scope -p MemoryMax=5G -p MemorySwapMax=0 \
 podman run --rm --userns=keep-id --user 1000:1000 \
  -v "$ROOT:/project:rw" -v "$GH:/home/builder/.gradle:rw" -w "/project/$MOD" \
  -e GRADLE_OPTS="-Dorg.gradle.jvmargs=-Xmx2g -Dorg.gradle.workers.max=2" \
  localhost/catalogizer-android-builder:jdk17 \
  bash -c 'echo "sdk.dir=$ANDROID_HOME" > local.properties; ./gradlew --no-daemon assembleDebug --console=plain'
