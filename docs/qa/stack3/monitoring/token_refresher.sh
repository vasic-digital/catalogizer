#!/bin/sh
# Sidecar loop (runs INSIDE the curl container): logs in as the stack admin, writes the session token (24h TTL) atomically to
# $TOK_DIR/metrics.tok every 6h so Prometheus' credentials_file always holds a valid token, then LOGS OUT the previous session.
# Secrets never appear in argv (/proc/<pid>/cmdline is world-readable): the login body AND the logout Authorization header are piped to curl
# on stdin (`--data @-`, `-H @-`); admin credentials come from a podman secret file (CRED_FILE, KEY=VALUE lines), not env.
# The superseded token is persisted (0600, via umask 077) in $TOK_DIR/previous.tok so a container restart can still revoke it; a failing logout is retried
# 3 times (10 s apart) and only then dropped (the session then lives until its 24 h expiry - logged). Failed logins back off 30 s doubling to 900 s.
# Token file: owner $TOK_CHOWN (65534 = the Prometheus uid), mode 0400. Nothing secret is printed.
# Test hooks (defaults are production values): TOK_DIR TOK_CHOWN LOGOUT_DELAY CYCLE_SLEEP MAX_CYCLES (0 = forever) BACKOFF_CAP.
H="${MON_HOST:-127.0.0.1}"; P="${API_PORT:-38082}"; B="http://$H:$P/api/v1/auth"
CF="${CRED_FILE:-/run/secrets/qa_admin}"; D="${TOK_DIR:-/tokens}"; CH="${TOK_CHOWN-65534:65534}"
LD="${LOGOUT_DELAY:-20}"; CS="${CYCLE_SLEEP:-21600}"; MAXC="${MAX_CYCLES:-0}"; CAP="${BACKOFF_CAP:-900}"
get() { sed -n "s/^$1=//p" "$CF" | head -n1; }
esc() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }   # JSON string escaping for \ and "
logout() {   # $1 = token; bearer goes in via stdin header file, never argv
  n=0
  while [ "$n" -lt 3 ]; do
    c=$(printf 'Authorization: Bearer %s\n' "$1" | curl -s -m 10 -o /dev/null -w '%{http_code}' -X POST "$B/logout" -H @-)
    case "$c" in 200|401) echo "$(date -u +%FT%TZ) previous session logout http=$c"; return 0;; esac   # 401 = already invalid
    n=$((n+1)); echo "$(date -u +%FT%TZ) previous session logout http=$c (attempt $n/3)" >&2; [ "$n" -ge 3 ] || sleep 10
  done
  echo "$(date -u +%FT%TZ) previous session NOT revoked - left to expire (<=24h)" >&2; return 1
}
umask 077; mkdir -p "$D"
OLD=""; [ -s "$D/previous.tok" ] && OLD=$(cat "$D/previous.tok")   # survive a container restart
BACK=30; CYC=0
while :; do
  U=$(esc "$(get ADMIN_USERNAME)"); W=$(esc "$(get ADMIN_PASSWORD)")
  T=$(printf '{"username":"%s","password":"%s"}' "$U" "$W" |
      curl -fsS -m 15 -X POST "$B/login" -H 'Content-Type: application/json' --data @- |
      sed -n 's/.*"session_token":"\([^"]*\)".*/\1/p')
  U=""; W=""
  if [ -n "$T" ]; then
    printf '%s' "$T" > "$D/.t"; [ -z "$CH" ] || chown "$CH" "$D/.t"; chmod 0400 "$D/.t" && mv -f "$D/.t" "$D/metrics.tok"
    echo "$(date -u +%FT%TZ) metrics token refreshed"
    if [ -n "$OLD" ]; then   # revoke the superseded admin session (not before the new file is in place)
      sleep "$LD"; logout "$OLD"; rm -f "$D/previous.tok"
    fi
    OLD="$T"; T=""; ( umask 077; printf '%s' "$OLD" > "$D/previous.tok" ); BACK=30
    CYC=$((CYC+1)); [ "$MAXC" -eq 0 ] || [ "$CYC" -lt "$MAXC" ] || exit 0
    sleep "$CS"
  else
    echo "$(date -u +%FT%TZ) login failed - retry in ${BACK}s" >&2; sleep "$BACK"; BACK=$((BACK*2)); [ "$BACK" -le "$CAP" ] || BACK="$CAP"
  fi
done
