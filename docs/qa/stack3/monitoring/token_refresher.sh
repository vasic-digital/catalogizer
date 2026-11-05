#!/bin/sh
# Sidecar loop (runs INSIDE the curl container): logs in as the stack admin, writes the session token (24h TTL) atomically to
# /tokens/metrics.tok every 6h so Prometheus' credentials_file always holds a valid token. Credentials arrive as env NAMES
# (ADMIN_USERNAME/ADMIN_PASSWORD) and are piped to curl on stdin (never in argv). Nothing secret is printed.
# Preferred alternative (needs a stack-3 API restart, so NOT done here): set METRICS_SCRAPE_TOKEN (>=32 chars) on the API and use it.
H="${MON_HOST:-host.containers.internal}"; P="${API_PORT:-38082}"
while :; do
  T=$(printf '{"username":"%s","password":"%s"}' "$ADMIN_USERNAME" "$ADMIN_PASSWORD" |
      curl -fsS -m 15 -X POST "http://$H:$P/api/v1/auth/login" -H 'Content-Type: application/json' --data @- |
      sed -n 's/.*"session_token":"\([^"]*\)".*/\1/p')
  if [ -n "$T" ]; then
    umask 022; printf '%s' "$T" > /tokens/.t && mv /tokens/.t /tokens/metrics.tok
    echo "$(date -u +%FT%TZ) metrics token refreshed"; sleep 21600
  else
    echo "$(date -u +%FT%TZ) login failed - retry in 30s" >&2; sleep 30
  fi
done
