#!/usr/bin/env bash
# Shared settings/helpers for the on-demand SonarQube QA service (sourced by up.sh / scan.sh / down.sh).
# Pure shell + curl + jq + podman: NO model/LLM calls. Secrets live ONLY in $SQ_DIR (mode 0600, outside the repo).
STACK="${SQ_STACK:-catsonar}"
POD="$STACK"
SQ_DIR="${SQ_DIR:-$HOME/.cache/claude-tmp/${STACK}}"
SONAR_BIND="${SONAR_BIND:-192.168.1.115}"      # LAN address the UI is published on (never 0.0.0.0)
SONAR_PORT="${SONAR_PORT:-19000}"
SONAR_IMAGE="${SONAR_IMAGE:-docker.io/library/sonarqube:community}"
PG_IMAGE="${PG_IMAGE:-docker.io/library/postgres:15-alpine}"
SCANNER_IMAGE="${SCANNER_IMAGE:-docker.io/sonarsource/sonar-scanner-cli:12.2}"
# Resource caps (podman --memory / --memory-swap / --cpus). swap == memory => no swap use.
SONAR_MEM="${SONAR_MEM:-4g}"; SONAR_CPUS="${SONAR_CPUS:-2}"
PG_MEM="${PG_MEM:-512m}";     PG_CPUS="${PG_CPUS:-1}"
SCANNER_MEM="${SCANNER_MEM:-6g}"; SCANNER_XMX="${SCANNER_XMX:-5g}"; SCANNER_CPUS="${SCANNER_CPUS:-2}"
LOAD_MAX="${LOAD_MAX:-20}"                      # do not start heavy work while 1-min load is above this
API_URL="http://${SONAR_BIND}:${SONAR_PORT}"

die() { echo "ERROR: $*" >&2; exit 1; }

wait_for_load() { # wait up to ${1:-900}s for 1-min loadavg <= LOAD_MAX
  local t="${1:-900}" s=0 l
  while :; do
    l="$(cut -d' ' -f1 /proc/loadavg)"
    awk -v l="$l" -v m="$LOAD_MAX" 'BEGIN{exit !(l<=m)}' && return 0
    [ "$s" -ge "$t" ] && die "load $l stayed above $LOAD_MAX for ${t}s"
    echo "load $l > $LOAD_MAX, waiting..."; sleep 20; s=$((s+20))
  done
}

# Secrets file: SONAR_ADMIN_PASSWORD, SONAR_DB_PASSWORD (+ SONAR_TOKEN once minted). Created 0600, never printed.
ensure_secrets() {
  mkdir -p "$SQ_DIR"; chmod 700 "$SQ_DIR"
  if [ ! -f "$SQ_DIR/sonar.env" ]; then
    ( umask 077
      { echo "SONAR_ADMIN_PASSWORD=Aa1-$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)"
        echo "SONAR_DB_PASSWORD=$(head -c 24 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | head -c 24)"
      } > "$SQ_DIR/sonar.env" )
  fi
  chmod 600 "$SQ_DIR/sonar.env"
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
