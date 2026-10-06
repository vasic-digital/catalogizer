#!/usr/bin/env bash
# resolve_pin.sh - T005d / T006 helper. Resolves the digest of an image already in local rootless storage and writes its entry
# into the lock file (docs/16 section 7.1). Digests are read from podman, never typed.
# Usage: resolve_pin.sh --id IMG-X --reference docker.io/lib/name --tag-intent TAG --purpose "text" [--entrypoint-override /bin/x]
#                       [--signature-status UNKNOWN] [--lock FILE] [--local-ref <reference:tag>]
# The image must already be pulled (`podman pull <reference>:<tag>`, preceded by scripts/containers/disk_headroom.sh --need ...).
# digest          = the digest of the image INDEX (manifest list) the tag pointed at: the RepoDigests entry of this reference whose
#                   manifest media type is an index, found by `podman manifest inspect <reference>@<entry>`; when only a single-platform
#                   image exists, that one. platform_digest = `podman image inspect .Digest` (the linux/amd64 manifest actually stored).
# An unresolvable digest leaves digest: "" (UNPINNED) and exits 1.
set -u
LOCK="build/containers/images.lock.yaml"; ID=""; REF=""; TAG=""; PURPOSE=""; EPO=""; SIG="UNKNOWN"; LOCALREF=""
while [ $# -gt 0 ]; do case "$1" in
  --id) ID="$2"; shift 2;; --reference) REF="$2"; shift 2;; --tag-intent) TAG="$2"; shift 2;; --purpose) PURPOSE="$2"; shift 2;;
  --entrypoint-override) EPO="$2"; shift 2;; --signature-status) SIG="$2"; shift 2;; --lock) LOCK="$2"; shift 2;; --local-ref) LOCALREF="$2"; shift 2;;
  *) echo "resolve_pin: unknown argument $1" >&2; exit 2;; esac; done
[ -n "$ID" ] && [ -n "$REF" ] && [ -n "$TAG" ] || { echo "resolve_pin: --id --reference --tag-intent are required" >&2; exit 2; }
PULLED="${LOCALREF:-$REF:$TAG}"
SIZE="$(podman image inspect --format '{{.Size}}' "$PULLED" 2>/dev/null)"
PLAT="$(podman image inspect --format '{{.Digest}}' "$PULLED" 2>/dev/null)"
CANDS="$(podman image inspect --format '{{range .RepoDigests}}{{println .}}{{end}}' "$PULLED" 2>/dev/null | grep "^$REF@sha256:" | sed 's/.*@//')"
DIGEST=""
for c in $CANDS; do
  mt="$(podman manifest inspect "$REF@$c" 2>/dev/null | python3 -c 'import sys,json
try: print(json.load(sys.stdin).get("mediaType",""))
except Exception: print("")')"
  case "$mt" in *index*|*manifest.list*) DIGEST="$c";; esac
done
[ -n "$DIGEST" ] || { [ "$(printf '%s\n' $CANDS | wc -l)" = 1 ] && DIGEST="$CANDS"; }
python3 - "$LOCK" "$ID" "$REF" "$TAG" "$DIGEST" "$PLAT" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(hostname)" "$PURPOSE" "$EPO" "$SIG" "${SIZE:-}" <<'PY'
import sys, os, yaml
lock, id_, ref, tag, dig, plat, at, host, purpose, epo, sig, size = sys.argv[1:13]
d = {"schema": 1, "images": []}
if os.path.exists(lock):
    d = yaml.safe_load(open(lock)) or d
d["images"] = [i for i in d["images"] if i.get("id") != id_]
e = {"id": id_, "reference": ref, "tag_intent": tag, "digest": dig, "platform_digest": plat if dig else "",
     "resolved_at": at if dig else "", "resolved_on": host if dig else "", "purpose": purpose,
     "signature_status": sig}
if epo: e["entrypoint_override"] = epo
if size.isdigit(): e["size_bytes"] = int(size)
d["images"].append(e)
os.makedirs(os.path.dirname(lock) or ".", exist_ok=True)
hdr = "# build/containers/images.lock.yaml - schema docs/16 section 7.1. Digests are filled by scripts/containers/resolve_pin.sh from `podman image inspect`/`podman manifest inspect`, never by hand.\n# digest = image index (manifest list) digest of the tag at resolved_at; platform_digest = the linux/amd64 manifest stored locally.\n"
open(lock, "w").write(hdr + yaml.safe_dump(d, sort_keys=False, default_flow_style=False))
PY
[ -n "$DIGEST" ] || { echo "resolve_pin: UNPINNED $ID (digest unresolved)" >&2; exit 1; }
echo "resolve_pin: $ID $REF@$DIGEST platform=$PLAT size=$SIZE"
