#!/usr/bin/env bash
# Shared settings/helpers for the on-demand SonarQube QA service (sourced by up.sh / scan.sh / down.sh).
# Pure shell + curl + jq + podman: NO model/LLM calls. Secrets live ONLY in $SQ_DIR (mode 0600, outside the repo).
STACK="${SQ_STACK:-catsonar}"
# Data-safety guard: this service may only ever touch resources named catsonar* (never catqa1/2/3 or any other stack).
[[ "$STACK" =~ ^catsonar[a-z0-9-]*$ ]] || { echo "ERROR: SQ_STACK '$STACK' refused (must match ^catsonar[a-z0-9-]*\$)" >&2; exit 1; }
POD="$STACK"
LABEL_KEY="io.catalogizer.service"; LABEL_VAL="sonarqube"   # ownership label on pod + volumes; nothing without it is ever removed
SQ_DIR="${SQ_DIR:-$HOME/.cache/claude-tmp/${STACK}}"
SONAR_BIND="${SONAR_BIND:-192.168.1.115}"      # LAN address the UI is published on (never 0.0.0.0)
SONAR_PORT="${SONAR_PORT:-19000}"
# All three images pinned by digest. SonarQube and Postgres digests are identical to docker-compose.security.yml;
# the scanner digest was read from the local image sonar-scanner-cli:12.2 (compose does not use a scanner).
SONAR_IMAGE="${SONAR_IMAGE:-docker.io/library/sonarqube:community@sha256:c0f1160bccfa435db4168c2d7df69af3c3ea7bfeb87395613014048fb33a68c1}"
PG_IMAGE="${PG_IMAGE:-docker.io/library/postgres:15-alpine@sha256:f7d23353e1b15400d22ebe31189f4d314b87a4c129cc400c8c2d8d4ca127bf81}"
SCANNER_IMAGE="${SCANNER_IMAGE:-docker.io/sonarsource/sonar-scanner-cli:12.2@sha256:a3f4215076706c95a17a68c19322ee916e40a3acd081a8c1a1e839e0194afa57}"
# Resource caps (podman --memory / --memory-swap / --cpus). swap == memory => no swap use.
SONAR_MEM="${SONAR_MEM:-4g}"; SONAR_CPUS="${SONAR_CPUS:-2}"
PG_MEM="${PG_MEM:-512m}";     PG_CPUS="${PG_CPUS:-1}"
SCANNER_MEM="${SCANNER_MEM:-6g}"; SCANNER_XMX="${SCANNER_XMX:-5g}"; SCANNER_CPUS="${SCANNER_CPUS:-2}"
# Do not start heavy work while the 1-min load is above this. Default derives from the host: 0.75 * nproc (16 cores -> 12).
LOAD_MAX="${LOAD_MAX:-$(awk -v n="$(nproc)" 'BEGIN{printf "%d", n*0.75}')}"
LOADAVG_FILE="${LOADAVG_FILE:-/proc/loadavg}"
DISK_MIN_FREE_GB="${DISK_MIN_FREE_GB:-10}"; DISK_MAX_USED_PCT="${DISK_MAX_USED_PCT:-98}"
API_URL="http://${SONAR_BIND}:${SONAR_PORT}"

die() { echo "ERROR: $*" >&2; exit 1; }

wait_for_load() { # wait up to ${1:-900}s for 1-min loadavg <= LOAD_MAX
  local t="${1:-900}" s=0 l
  while :; do
    l="$(cut -d' ' -f1 "$LOADAVG_FILE")"
    awk -v l="$l" -v m="$LOAD_MAX" 'BEGIN{exit !(l<=m)}' && return 0
    [ "$s" -ge "$t" ] && die "load $l stayed above $LOAD_MAX for ${t}s"
    echo "load $l > $LOAD_MAX, waiting..."; sleep 20; s=$((s+20))
  done
}

# Ownership guards (F1): only pods/volumes carrying our label may be removed or reused.
owned_pod() { [ "$(podman pod inspect "$POD" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" = "$LABEL_VAL" ]; }
owned_vol() { [ "$(podman volume inspect "$1" --format "{{index .Labels \"$LABEL_KEY\"}}" 2>/dev/null)" = "$LABEL_VAL" ]; }
require_owned_resources() { # die if any EXISTING pod/volume of this stack lacks the label (checked before anything is changed)
  if podman pod exists "$POD" 2>/dev/null && ! owned_pod; then die "pod $POD exists without label $LABEL_KEY=$LABEL_VAL - refusing to touch it"; fi
  local v; for v in pg data extensions logs; do
    if podman volume exists "${STACK}-$v" 2>/dev/null && ! owned_vol "${STACK}-$v"; then die "volume ${STACK}-$v exists without label $LABEL_KEY=$LABEL_VAL - refusing to touch it"; fi
  done
}
# Disk floor (F4): Elasticsearch needs room. Prints "<used%> <freeGB>" for path $1 (default: container volume store).
disk_stats() { df -Pk "${1:-$HOME}" | awk 'NR==2{gsub("%","",$5); printf "%d %d\n", $5, $4/1048576}'; }
check_disk_floor() {
  local st used free; st="$(disk_stats "${1:-$HOME}")"; used="${st% *}"; free="${st#* }"
  { [ -n "$used" ] && [ -n "$free" ]; } || die "cannot measure disk usage"
  [ "$free" -ge "$DISK_MIN_FREE_GB" ] || die "only ${free} GB free (< ${DISK_MIN_FREE_GB} GB) - refusing to start"
  [ "$used" -le "$DISK_MAX_USED_PCT" ] || die "disk ${used}% used (> ${DISK_MAX_USED_PCT}%) - refusing to start"
  DISK_USED_PCT="$used"; DISK_FREE_GB="$free"
}

# Images (digest-pinned): use local copies; otherwise pull. Docker Hub's anonymous pull limit is a known failure: fail with a clear
# message instead of a half-created pod. Credentials are never stored here: use `podman login docker.io` (your own session) or pre-pull.
ensure_images() {
  local img out
  for img in "$@"; do
    podman image exists "$img" 2>/dev/null && continue
    echo "image not present locally, pulling: ${img##*/}"
    if ! out="$(podman pull "$img" 2>&1)"; then
      if grep -qiE 'toomanyrequests|rate limit' <<<"$out"; then
        die "Docker Hub rate limit hit while pulling $img. Fix: run 'podman login docker.io' (authenticated pulls have a higher limit), or pre-pull/load the pinned image (podman pull / podman load) and re-run. Nothing was started."
      fi
      die "pull of $img failed: $(printf '%s' "$out" | tail -1)"
    fi
  done
}

# Secrets file: SONAR_ADMIN_PASSWORD, SONAR_DB_PASSWORD (+ SONAR_TOKEN once minted). Created 0600, never printed.
ensure_secrets() {
  mkdir -p "$SQ_DIR"
  [ -d "$SQ_DIR" ] && [ ! -L "$SQ_DIR" ] && [ -O "$SQ_DIR" ] || die "SQ_DIR $SQ_DIR must be a real directory owned by you"
  chmod 700 "$SQ_DIR"
  if [ ! -f "$SQ_DIR/sonar.env" ]; then
    ( umask 077
      { echo "SONAR_ADMIN_PASSWORD=Aa1-$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)"
        echo "SONAR_DB_PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)"
      } > "$SQ_DIR/sonar.env" )
  fi
  chmod 600 "$SQ_DIR/sonar.env"
  local k v; for k in SONAR_ADMIN_PASSWORD SONAR_DB_PASSWORD; do
    v="$(sed -n "s/^$k=//p" "$SQ_DIR/sonar.env")"; [ "${#v}" -ge 24 ] || die "$k in $SQ_DIR/sonar.env is shorter than 24 chars"
  done
}
load_secrets() { set -a; . "$SQ_DIR/sonar.env"; set +a; }

# curl netrc (0600) so credentials never appear in argv. $1 = login, $2 = password.
write_netrc() {
  ( umask 077; printf 'machine %s login %s password %s\n' "$SONAR_BIND" "$1" "$2" > "$SQ_DIR/netrc" )
}
sq() { curl -sS --max-time 60 --netrc-file "$SQ_DIR/netrc" "$@"; }

# Scrub known secret values from a file (in place). Secrets come from sonar.env by name; never echoed.
scrub_file() {
  local f="$1" v
  for k in SONAR_ADMIN_PASSWORD SONAR_DB_PASSWORD SONAR_TOKEN; do
    v="${!k:-}"; [ -n "$v" ] || continue
    SECRET="$v" python3 -I - "$f" <<'PY'
import os,sys
p=sys.argv[1]; s=os.environ["SECRET"]
d=open(p,"rb").read(); n=d.replace(s.encode(),b"[REDACTED]")
if n!=d: open(p,"wb").write(n)
PY
  done
}
# True if no secret value occurs in file $1.
no_secret_in() {
  local f="$1" v
  for k in SONAR_ADMIN_PASSWORD SONAR_DB_PASSWORD SONAR_TOKEN; do
    v="${!k:-}"; [ -n "$v" ] || continue
    SECRET="$v" python3 -I - "$f" <<'PY' || return 1
import os,sys
sys.exit(1 if os.environ["SECRET"].encode() in open(sys.argv[1],"rb").read() else 0)
PY
  done
}
