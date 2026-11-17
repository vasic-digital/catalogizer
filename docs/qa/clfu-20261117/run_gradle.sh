#!/bin/bash
# usage: run_tv.sh <clone> <module> <logfile> <gradle args...>
C=$1; M=$2; L=$3; shift 3
podman run --rm --userns=keep-id --memory=8g --cpus=4 -v "$C":/work -v ~/Android/Sdk:/opt/sdk -v /home/milosvasic/.cache/claude-tmp/clprog-gh:/gh -e GRADLE_USER_HOME=/gh -e ANDROID_HOME=/opt/sdk -w /work/$M docker.io/library/eclipse-temurin:17-jdk nice -n 15 ./gradlew --no-daemon --max-workers=1 "$@" > "$L" 2>&1
echo rc=$? >> "$L"
