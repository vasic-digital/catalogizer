#!/usr/bin/env bash
# nfs_build.sh - T134. Build the user-space NFS server image of the unprivileged NFS attempt: `localhost/catalogizer-infra-nfs:<hash>` where <hash> is the first 16 hex
# characters of the sha256 over the three input files (Containerfile, exports, entrypoint.sh), so the tag names its content. Rootless, on this host.
# The disk-headroom gate (scripts/containers/disk_headroom.sh) runs first. Prints `image=<ref>`, `image_id=<id>`, `digest=<manifest digest>`, `image_ref=localhost/catalogizer-infra-nfs@<digest>` (the form the NFS compose file takes in $TI_NFS_IMAGE).
# Usage:  nfs_build.sh [--need <bytes>]   (default 600000000)
# Exit:   0 built or already present; 1 failed / refused by the disk gate; 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
NEED=600000000
while [ $# -gt 0 ]; do case "$1" in --need) ti_optval "$1" $# "${2:-}"; NEED=$2; shift 2;; *) ti_die "unknown argument '$1'" 2;; esac; done
ti_uint "$NEED" 15 --need
ti_need podman sha256sum
CTX="$HERE/nfs"
HASH="$(cat "$CTX/Containerfile" "$CTX/exports" "$CTX/entrypoint.sh" | sha256sum | cut -c1-16)"
REF="localhost/catalogizer-infra-nfs:$HASH"
if podman image exists "$REF" 2>/dev/null; then echo "present=$REF"; else
  bash "$TI_ROOT/scripts/containers/disk_headroom.sh" --need "$NEED" --op-id "build-infra-nfs-$HASH" >&2 || ti_die "disk headroom gate refused the build" 1
  podman build --pull=never --label project=catalogizer -t "$REF" -f "$CTX/Containerfile" "$CTX" >&2 || ti_die "podman build failed" 1
fi
ID="$(podman image inspect --format '{{.Id}}' "$REF")"
DG="$(podman image inspect --format '{{.Digest}}' "$REF")"
echo "image=$REF"; echo "image_id=$ID"; echo "digest=$DG"; echo "image_ref=localhost/catalogizer-infra-nfs@$DG"
