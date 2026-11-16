#!/usr/bin/env bash
# nfs_emulator_live.sh - CAT-NFS-EMU-2. Evidence class EMULATOR (never the real NAS NFS).
# Runs the unfs3 image (scripts/test-infra/nfs_build.sh) as ONE uniquely named rootless container, seeds a deterministic tree, writes the manifest
# (path, size, sha256) from INSIDE the server container, runs the project's real nfs3 client tests against it, and removes the container by exact name.
# Usage: nfs_emulator_live.sh --image <image_ref> --out <dir>   (disk gate: scripts/containers/disk_headroom.sh is run by nfs_build.sh, not bypassed)
set -u
IMG=""; OUT=""
while [ $# -gt 0 ]; do case "$1" in --image) IMG=${2:?}; shift 2;; --out) OUT=${2:?}; shift 2;; *) echo "unknown arg $1" >&2; exit 2;; esac; done
[ -n "$IMG" ] && [ -n "$OUT" ] || { echo "usage: --image <ref> --out <dir>" >&2; exit 2; }
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; ROOT="$(cd "$HERE/../.." && pwd)"
mkdir -p "$OUT"; N="catnfsemu-unfs3-$$-$(date +%s)"
trap 'podman rm -f "$N" >/dev/null 2>&1' EXIT
podman run -d --name "$N" --label project=catalogizer --label catalogizer.test=nfs-emulator --tmpfs /export:rw,mode=0777 --memory 256m --cpus 0.25 \
  -p 127.0.0.1::2049 -p 127.0.0.1::111 "$IMG" >"$OUT/container_id.txt" || exit 1
for i in $(seq 1 40); do podman exec "$N" rpcinfo -p 127.0.0.1 2>/dev/null | grep -q mountd && break; sleep 0.5; done
podman exec "$N" rpcinfo -p 127.0.0.1 >"$OUT/rpcinfo.txt" 2>&1; grep -q mountd "$OUT/rpcinfo.txt" || { echo "server not ready" >&2; podman logs "$N" >"$OUT/server.log" 2>&1; exit 1; }
podman exec "$N" sh -c 'mkdir -p /export/movies/2024 /export/music && head -c 3000000 /dev/urandom > /export/movies/2024/a.bin && printf "hello nfs emulator\n" > /export/readme.txt && head -c 70000 /dev/urandom > /export/music/b.flac && : > /export/empty.dat'
podman exec "$N" sh -c 'cd /export && for p in $(find . -mindepth 1 | sort); do q=${p#.}; if [ -d "$p" ]; then printf "{\"path\":\"%s\",\"dir\":true,\"size\":0,\"sha256\":\"\"}\n" "$q"; else printf "{\"path\":\"%s\",\"dir\":false,\"size\":%s,\"sha256\":\"%s\"}\n" "$q" "$(stat -c %s "$p")" "$(sha256sum "$p" | cut -d" " -f1)"; fi; done' | jq -s . >"$OUT/manifest.json" || exit 1
NFSP="$(podman port "$N" 2049/tcp | sed 's/.*://')"; PMP="$(podman port "$N" 111/tcp | sed 's/.*://')"
podman inspect "$N" --format 'image={{.ImageName}} id={{.Image}} state={{.State.Status}}' >"$OUT/container_identity.txt"
cd "$ROOT/catalog-api" && NFS_EMU_ADDR="127.0.0.1:$NFSP" NFS_EMU_PORTMAP_PORT="$PMP" NFS_EMU_MANIFEST="$OUT/manifest.json" \
  GOMAXPROCS=2 nice -n 10 go test -p 1 -buildvcs=false -count=1 -v ./tests/nfsemu/ >"$OUT/go_test.txt" 2>&1; rc=$?
# NFS3-WIRE-1: the storage factory lifecycle (factory -> Connect -> list -> Disconnect) against the same server
NFS_EMU_ADDR="127.0.0.1:$NFSP" GOMAXPROCS=2 nice -n 10 go test -p 1 -buildvcs=false -count=1 -v -run TestNFS3FactoryLifecycleLive ./filesystem/ >>"$OUT/go_test.txt" 2>&1; rc2=$?
[ "$rc" -eq 0 ] && rc=$rc2
# NFS3W-FU-m2: every required test must have its own anchored top-level PASS line (a SKIP, a zero-test run or a subtest-only PASS is rc!=0)
. "$HERE/nfs_emulator_check.sh"
if [ "$rc" -eq 0 ]; then
  if ! missing="$(nfs_emulator_missing_pass "$OUT/go_test.txt")"; then
    echo "required tests without a PASS line (skipped or missing): $(echo "$missing" | tr '\n' ' ')" | tee -a "$OUT/go_test.txt" >&2; rc=1
  fi
fi
podman logs "$N" >"$OUT/server.log" 2>&1
echo "go_test_rc=$rc" | tee -a "$OUT/go_test.txt" | tail -1; exit $rc
