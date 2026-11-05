#!/bin/sh
# Sidecar loop (runs INSIDE the curl container): logs in as the stack admin, writes the session token (24h TTL) atomically to
# /tokens/metrics.tok every 6h so Prometheus' credentials_file always holds a valid token, then LOGS OUT the previous session.
# Credentials arrive as a podman secret file (CRED_FILE, KEY=VALUE lines) - never argv/env; the JSON body is built with proper escaping
# and piped to curl on stdin. Failures back off exponentially (30s doubling, cap 900s) so a wrong password cannot hammer the login lockout.
# Token file: owner 65534 (the Prometheus uid), mode 0400. Nothing secret is printed.
H="${MON_HOST:-127.0.0.1}"; P="${API_PORT:-38082}"; B="http://$H:$P/api/v1/auth"
CF="${CRED_FILE:-/run/secrets/qa_admin}"
get() { sed -n "s/^$1=//p" "$CF" | head -n1; }
esc() { printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'; }   # JSON string escaping for \ and "
OLD=""; BACK=30
while :; do
  U=$(esc "$(get ADMIN_USERNAME)"); W=$(esc "$(get ADMIN_PASSWORD)")
  T=$(printf '{"username":"%s","password":"%s"}' "$U" "$W" |
      curl -fsS -m 15 -X POST "$B/login" -H 'Content-Type: application/json' --data @- |
      sed -n 's/.*"session_token":"\([^"]*\)".*/\1/p')
  U=""; W=""
  if [ -n "$T" ]; then
    umask 077; printf '%s' "$T" > /tokens/.t && chown 65534:65534 /tokens/.t && chmod 0400 /tokens/.t && mv -f /tokens/.t /tokens/metrics.tok
    echo "$(date -u +%FT%TZ) metrics token refreshed"
    if [ -n "$OLD" ]; then   # revoke the superseded admin session (not before the new file is in place)
      sleep 20; c=$(curl -s -m 10 -o /dev/null -w '%{http_code}' -X POST "$B/logout" -H "Authorization: Bearer $OLD")
      echo "$(date -u +%FT%TZ) previous session logout http=$c"
    fi
    OLD="$T"; T=""; BACK=30; sleep 21600
  else
    echo "$(date -u +%FT%TZ) login failed - retry in ${BACK}s" >&2; sleep "$BACK"; BACK=$((BACK*2)); [ "$BACK" -le 900 ] || BACK=900
  fi
done
