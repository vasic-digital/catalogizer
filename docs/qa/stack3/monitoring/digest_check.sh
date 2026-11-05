# sourced by up.sh: verify pulled-image digests against the recorded pins. FAILS CLOSED: an image with no recorded pin is refused
# unless ALLOW_UNPINNED_IMAGES=1 is set explicitly (then a loud warning is printed). usage: check_image_digests <pins-file> <image>...
check_image_digests() {
  local pins="$1" i exp got rc=0; shift
  for i in "$@"; do
    exp="$(awk -v i="$i" '$1==i{print $2}' "$pins" 2>/dev/null)"; got="$(podman image inspect "$i" --format '{{.Digest}}' 2>/dev/null)"
    if [ -z "$exp" ]; then
      if [ "${ALLOW_UNPINNED_IMAGES:-0}" = 1 ]; then echo "WARNING: image $i has NO recorded digest pin in $pins - ALLOW_UNPINNED_IMAGES=1, running unpinned" >&2
      else echo "refusing: image $i has no recorded digest pin in $pins (record it, or set ALLOW_UNPINNED_IMAGES=1 to override)" >&2; rc=3; fi
    elif [ "$exp" != "$got" ]; then echo "digest mismatch for $i: expected $exp got ${got:-<none>}" >&2; rc=3; fi
  done
  return "$rc"
}
