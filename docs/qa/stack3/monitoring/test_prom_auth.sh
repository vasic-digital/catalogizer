#!/usr/bin/env bash
# QA-PROM-AUTH live test (11.4.289: real Prometheus image, real HTTP; no stubs). Starts a DISPOSABLE Prometheus (name catqa-promauth-test) bound to
# 127.0.0.1 on a free high port, configured exactly as up.sh configures it (lib.sh prom_web_config + the real prometheus.yml), and asserts:
#   anonymous -> 401 (targets, query, ready), wrong password -> 401, right password -> 200, and scraping still works (self-scrape job up).
# PROMAUTH_MUTATE=noauth starts it WITHOUT web.config.file: the test MUST then FAIL (paired mutation). Secrets are random, never printed.
# usage: test_prom_auth.sh   exit 0 = all PASS
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
CN=catqa-promauth-test; T="$(mktemp -d)"; : "${T:?}"
cleanup() { podman rm -f "$CN" >/dev/null 2>&1 || true; rm -rf -- "${T:?}"; }; trap cleanup EXIT
podman rm -f "$CN" >/dev/null 2>&1 || true
PORT=39791; while ss -ltnH "sport = :$PORT" | grep -q .; do PORT=$((PORT+1)); done
PW="$(head -c 18 /dev/urandom | od -An -vtx1 | tr -d ' \n')"; BAD="wrong-$(head -c 6 /dev/urandom | od -An -vtx1 | tr -d ' \n')"
mkdir -p "$T/cfg" "$T/tok"; chmod 755 "$T" "$T/cfg" "$T/tok"
sed -e 's|__MON_HOST__|127.0.0.1|g' -e 's|__API_PORT__|38082|g' -e "s|__PROM_USER__|$PROM_AUTH_USER|g" "$HERE/prometheus/prometheus.yml" > "$T/cfg/prometheus.yml"
printf 'x' > "$T/tok/metrics.tok"; printf '%s' "$PW" > "$T/cfg/prom_pw"
if [ "${PROMAUTH_MUTATE:-}" = noauth ]; then : > "$T/cfg/web.yml"; else printf '%s' "$PW" | prom_web_config "$PROM_AUTH_USER" > "$T/cfg/web.yml" || { echo "FAIL setup.web_config"; exit 1; }; fi
chmod 644 "$T/cfg/"* "$T/tok/"*
podman run -d --name "$CN" --memory 256m -p "127.0.0.1:$PORT:9090" -v "$T/cfg:/cfg:ro" -v "$T/tok:/tokens:ro" -v "$T/cfg/prom_pw:/run/secrets/prom_pw:ro" \
  "$PROM_IMAGE" --config.file=/cfg/prometheus.yml --web.config.file=/cfg/web.yml --storage.tsdb.path=/prometheus >/dev/null || { echo "FAIL setup.start"; exit 1; }
fail=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1 $2"; fail=1; }
U="http://127.0.0.1:$PORT"
code() { # code <path> [user pass]  - credentials via curl -K stdin, never argv
  if [ $# -ge 3 ]; then printf 'user = "%s:%s"\n' "$2" "$3" | curl -s -m 5 -o /dev/null -w '%{http_code}' -K - "$U$1"
  else curl -s -m 5 -o /dev/null -w '%{http_code}' "$U$1"; fi; }
for i in $(seq 1 40); do [ "$(code /-/ready "$PROM_AUTH_USER" "$PW")" = 200 ] && break; [ "$(code /-/ready)" = 200 ] && break; sleep 1; done
for p in /api/v1/targets '/api/v1/query?query=up' /-/ready; do
  c="$(code "$p")"; [ "$c" = 401 ] && ok "anon$p->401" || bad "anon$p->401" "got $c"
  c="$(code "$p" "$PROM_AUTH_USER" "$BAD")"; [ "$c" = 401 ] && ok "wrongpw$p->401" || bad "wrongpw$p->401" "got $c"
  c="$(code "$p" "$PROM_AUTH_USER" "$PW")"; [ "$c" = 200 ] && ok "rightpw$p->200" || bad "rightpw$p->200" "got $c"; done
c="$(code /api/v1/targets "wrong_user" "$PW")"; [ "$c" = 401 ] && ok "wronguser->401" || bad "wronguser->401" "got $c"
h=""; for i in $(seq 1 30); do
  h="$(printf 'user = "%s:%s"\n' "$PROM_AUTH_USER" "$PW" | curl -s -m 5 -K - "$U/api/v1/targets" | python3 -I -c 'import json,sys
d=json.load(sys.stdin)["data"]["activeTargets"]; print([t["health"] for t in d if t["labels"].get("job")=="prometheus"])' 2>/dev/null)"
  [ "$h" = "['up']" ] && break; sleep 1; done
[ "$h" = "['up']" ] && ok scrape.self_target_up_with_basic_auth || bad scrape.self_target_up_with_basic_auth "health=$h"
exit "$fail"
