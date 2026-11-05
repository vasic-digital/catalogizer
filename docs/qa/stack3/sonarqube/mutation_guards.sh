#!/usr/bin/env bash
# Paired mutations for test_guards.sh: each mutation breaks one guard in a sandbox COPY of the scripts; the test suite MUST fail.
# A surviving mutation (suite still green) = the corresponding test is decoration. Exit 0 only if every mutation is killed.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SB="$(mktemp -d /home/milosvasic/.cache/claude-tmp/sqmut.XXXXXX)"; : "${SB:?}"; trap 'rm -rf "${SB:?}"' EXIT
for n in 1 2 3; do "$HERE/test_guards.sh" "$HERE" >/dev/null 2>&1 || { echo "baseline suite is not green (repeat $n) - abort"; exit 2; }; done
SURV=0
mut() { # name file python-expression-old new
  local n="$1" f="$2" old="$3" new="$4" d="$SB/$1"; mkdir -p "$d"; cp "$HERE"/*.sh "$d"/
  OLD="$old" NEW="$new" python3 -I - "$d/$f" <<'PY' || { echo "MUTATION $n: pattern not found (broken mutation)"; SURV=1; return; }
import os,sys
p=sys.argv[1]; s=open(p).read(); o=os.environ["OLD"]
if o not in s: sys.exit(1)
open(p,"w").write(s.replace(o,os.environ["NEW"],1))
PY
  if "$d/test_guards.sh" "$d" >/dev/null 2>&1; then echo "SURVIVED  $n"; SURV=1; else echo "killed    $n"; fi
}
mut M1-stack-regex-removed      lib.sh  '[[ "$STACK" =~ ^catsonar[a-z0-9-]*$ ]] ||' 'true ||'
mut M2-down-skips-owner-check   down.sh 'require_owned_resources' ':'
mut M3-load-comparison-inverted lib.sh  "awk -v l=\"\$l\" -v m=\"\$LOAD_MAX\" 'BEGIN{exit !(l<=m)}'" "awk -v l=\"\$l\" -v m=\"\$LOAD_MAX\" 'BEGIN{exit !(l>m)}'"
mut M4-load-default-fixed-20    lib.sh  'LOAD_MAX="${LOAD_MAX:-$(awk -v n="$(nproc)" '"'"'BEGIN{printf "%d", n*0.75}'"'"')}"' 'LOAD_MAX="${LOAD_MAX:-20}"'
mut M5-disk-free-floor-removed  lib.sh  '[ "$free" -ge "$DISK_MIN_FREE_GB" ] ||' 'true ||'
mut M6-disk-pct-floor-removed   lib.sh  '[ "$used" -le "$DISK_MAX_USED_PCT" ] ||' 'true ||'
mut M7-no_secret_in-always-ok   lib.sh  'no_secret_in() {' 'no_secret_in() { return 0;'
mut M8-runid-guard-removed      scan.sh 'if [ -d "$OUT/$RUN_ID" ] && [ -n "$(ls -A "$OUT/$RUN_ID")" ]; then' 'if false; then'
mut M9-up-skips-owner-check     up.sh   'require_owned_resources ' ': '
mut M10-volume-owner-check-off  lib.sh  'if podman volume exists "${STACK}-$v" 2>/dev/null && ! owned_vol "${STACK}-$v"; then' 'if false; then'
mut M11-gate-status-append-only scan.sh ': > "$R/gate_status.txt"' ':'
mut M12-up-skips-disk-floor     up.sh   'check_disk_floor "$VOLROOT"' 'DISK_USED_PCT=10; DISK_FREE_GB=999'
mut M13-rate-limit-message-gone lib.sh  'toomanyrequests|rate limit' 'zzzneverzzz'
mut M14-up-skips-image-check    up.sh   'ensure_images "$SONAR_IMAGE" "$PG_IMAGE"' ':'
mut M15-owned-pod-any-label     lib.sh  'owned_pod() { [ "$(podman pod inspect "$POD" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" = "$LABEL_VAL" ]; }' 'owned_pod() { [ -n "$(podman pod inspect "$POD" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" ]; }'
mut M16-owned-vol-any-label     lib.sh  'owned_vol() { [ "$(podman volume inspect "$1" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" = "$LABEL_VAL" ]; }' 'owned_vol() { [ -n "$(podman volume inspect "$1" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" ]; }'
mut M17-free-floor-ge-to-gt     lib.sh  '[ "$free" -ge "$DISK_MIN_FREE_GB" ]' '[ "$free" -gt "$DISK_MIN_FREE_GB" ]'
mut M18-addr-check-removed      up.sh   '[[ "$ADDRS" == *"inet ${SONAR_BIND}/"* ]] ||' 'true ||'
mut M19-port-check-removed      up.sh   'if grep -q "[:.]${SONAR_PORT}\$" <<<"$LISTEN" &&' 'if false &&'
mut M20-ss-fail-open            up.sh   'LISTEN="$(ss -ltn | awk '"'"'{print $4}'"'"')" || die' 'LISTEN="$(ss -ltn | awk '"'"'{print $4}'"'"')" || true; : die'
echo "survivors: $SURV"; exit "$SURV"
