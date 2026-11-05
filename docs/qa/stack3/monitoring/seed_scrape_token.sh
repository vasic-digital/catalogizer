#!/bin/sh
# QA-MON-FU-1: write a static METRICS_SCRAPE_TOKEN as Prometheus' credentials file. Runs INSIDE the curl container (like token_refresher.sh).
# The token arrives on STDIN only (never argv/env). Refuses a token shorter than the API's minimum (32 chars, catalog-api/main_metricsauth.go):
# the API would silently ignore it and every scrape would 401. Test hooks: TOK_DIR (default /tokens), TOK_CHOWN (default 65534:65534, '' = skip).
D="${TOK_DIR:-/tokens}"; CH="${TOK_CHOWN-65534:65534}"
umask 077; mkdir -p "$D" || exit 1
T=$(cat); if [ "${#T}" -lt 32 ]; then echo "seed_scrape_token: token shorter than 32 chars - refusing" >&2; exit 2; fi
printf '%s' "$T" > "$D/.t" || exit 1; T=""
# a failed chown/chmod must NOT publish the file (unreadable-by-prometheus or wrongly-owned token would only show up as 401 scrapes later)
if [ -n "$CH" ] && ! chown "$CH" "$D/.t"; then rm -f "$D/.t"; echo "seed_scrape_token: chown $CH failed - nothing published" >&2; exit 1; fi
chmod 0400 "$D/.t" || { rm -f "$D/.t"; exit 1; }
mv -f "$D/.t" "$D/metrics.tok" || exit 1
rm -f "$D/previous.tok"   # the former refresher's superseded admin-session token: no refresher is left to revoke it (up.sh revokes it server-side first)
echo "static scrape token seeded"
