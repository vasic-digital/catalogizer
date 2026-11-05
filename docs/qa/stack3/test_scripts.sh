#!/usr/bin/env bash
# Sandbox tests for build.sh / up.sh (stub podman/curl/systemd-run/sleep/ss on PATH; touches no real container, port or QA data).
# usage: test_scripts.sh      exit 0 = all pass.   Everything runs under one mktemp -d root; HOME is never reassigned.
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/stack3-test.XXXXXX")"; : "${ROOT:?}"
trap 'rm -rf -- "${ROOT:?}"' EXIT
pass=0; fail=0
ok() { if [ "$1" = 0 ]; then pass=$((pass+1)); echo "PASS $2"; else fail=$((fail+1)); echo "FAIL $2"; fi; }

BIN="$ROOT/bin"; mkdir -p "$BIN"
cat > "$BIN/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "$STUB_LOG"
case "$1 $2" in
  "image exists") exit 0;;
  "pod exists") [ -f "$STUB_DIR/pod_exists" ]; exit $?;;
  "pod inspect") echo infra123; exit 0;;
  "port infra123") cat "$STUB_DIR/pod_ports" 2>/dev/null; exit 0;;
esac
case "$1" in
  inspect) case "$*" in *Health*) echo healthy;; *) echo running;; esac;;
esac
exit 0
S
cat > "$BIN/curl" <<'S'
#!/usr/bin/env bash
echo "curl $*" >> "$STUB_LOG"; printf '%s' "${CURL_CODE:-200}"
S
cat > "$BIN/sleep" <<'S'
#!/usr/bin/env bash
exit 0
S
cat > "$BIN/ss" <<'S'
#!/usr/bin/env bash
for p in $BUSY_PORTS; do case "$*" in *":$p") echo "LISTEN 0 1 0.0.0.0:$p";; esac; done; exit 0
S
cat > "$BIN/systemd-run" <<'S'
#!/usr/bin/env bash
echo "systemd-run $*" >> "$STUB_LOG"
while [ $# -gt 0 ]; do case "$1" in --user|--scope) shift;; -p) shift 2;; *) break;; esac; done
exec "$@"
S
cat > "$BIN/ip" <<'S'
#!/usr/bin/env bash
# stub `ip -4 route get 1.1.1.1`: prints the route line with src $STUB_IP_SRC (nothing and rc 1 when unset)
[ -n "${STUB_IP_SRC:-}" ] || exit 1; echo "1.1.1.1 via 10.0.0.1 dev eth0 src ${STUB_IP_SRC} uid 1000"
S
chmod +x "$BIN"/*

setup() { # <name>  -> fresh sandbox dirs + env
  T="$ROOT/${1:?}"; mkdir -p "$T/qa" "$T/media" "$T/stub"
  export STUB_IP_SRC=192.168.1.115 STUB_LOG="$T/log" STUB_DIR="$T/stub" QA_DIR="$T/qa" MEDIA_DIR="$T/media" BUSY_PORTS="" CURL_CODE=200
  : > "$STUB_LOG"
  echo deadbeefcafe0123 > "$QA_DIR/BUILT_COMMIT"
}
run_up() { PATH="$BIN:$PATH" bash "$HERE/up.sh" "$@" >"$T/out" 2>"$T/err"; echo $? > "$T/rc"; }

# 1. no secret value in podman argv
setup t1; run_up
bad=0
for k in POSTGRES_PASSWORD ADMIN_PASSWORD JWT_SECRET; do
  v="$(grep "^$k=" "$QA_DIR/qa.env" | cut -d= -f2-)"
  if [ -n "$v" ] && grep -qF -- "$v" "$STUB_LOG"; then echo "  secret $k leaked in argv"; bad=1; fi
done
if grep -q 'DATABASE_PASSWORD=' "$STUB_LOG"; then echo "  DATABASE_PASSWORD=value in argv"; bad=1; fi
grep -q -- '-e DATABASE_PASSWORD ' "$STUB_LOG" || { echo "  DATABASE_PASSWORD not passed by name"; bad=1; }
ok $bad "no secret value in podman argv"
[ "$(cat "$T/rc")" = 0 ]; ok $? "healthy stack exits 0"

# 2. unhealthy probes -> non-zero
setup t2; export CURL_CODE=502; run_up
[ "$(cat "$T/rc")" != 0 ]; ok $? "all probes 502 -> non-zero exit"

# 3. port busy with a foreign owner while the pod exists -> refuse BEFORE pod rm
setup t3; : > "$T/stub/pod_exists"; echo "8080/tcp -> 0.0.0.0:99999" > "$T/stub/pod_ports"; export BUSY_PORTS="38082"
run_up
[ "$(cat "$T/rc")" = 5 ] && ! grep -q 'pod rm' "$STUB_LOG"; ok $? "foreign port owner refused before pod rm"
setup t3b; : > "$T/stub/pod_exists"; echo "8080/tcp -> 0.0.0.0:38082" > "$T/stub/pod_ports"; export BUSY_PORTS="38082"
run_up
[ "$(cat "$T/rc")" = 0 ] && grep -q 'pod rm' "$STUB_LOG"; ok $? "own-pod port allowed, pod replaced"

# 4. build.sh in a tiny git repo that contains copies of the scripts
R="$ROOT/repo"; mkdir -p "$R/docs/qa/stack3" "$R/catalog-api" "$R/catalog-web"
cp "$HERE/build.sh" "$HERE/lib.sh" "$R/docs/qa/stack3/"
printf 'FROM scratch\nRUN go build -a -ldflags="-w -s" -o catalogizer-api .\n' > "$R/catalog-api/Dockerfile"
printf 'FROM scratch\n' > "$R/catalog-web/Dockerfile"
( cd "$R" && git init -q && git add -A && git -c user.name=t -c user.email=t@t commit -qm init )
setup b1; rm -f "$QA_DIR/BUILT_COMMIT"; rmdir "$QA_DIR"   # QA_DIR must not exist yet
export TMPBASE="$ROOT/tmpbase" WEBSITE=0
PATH="$BIN:$PATH" bash "$R/docs/qa/stack3/build.sh" >"$T/out" 2>"$T/err"; rc=$?
[ "$rc" = 0 ] && [ -f "$QA_DIR/BUILT_COMMIT" ]; ok $? "WEBSITE=0 with fresh QA_DIR succeeds (rc=$rc)"
setup b2; mkdir -p "$QA_DIR/website_dist"; echo old > "$QA_DIR/website_dist/stale.html"; export TMPBASE="$ROOT/tmpbase" WEBSITE=1
PATH="$BIN:$PATH" bash "$R/docs/qa/stack3/build.sh" >"$T/out" 2>"$T/err"
[ ! -e "$QA_DIR/website_dist/stale.html" ]; ok $? "stale website_dist cleared before build"

# 5. QA-MON-FU-1 / QA-MON follow-up: dedicated scrape token + explicit TRUSTED_PROXIES (passed by NAME / explicit CIDR, never a value in argv)
setup t5; run_up
tok="$(grep '^METRICS_SCRAPE_TOKEN=' "$QA_DIR/qa.env" | cut -d= -f2-)"
[ "${#tok}" -ge 32 ]; ok $? "fresh qa.env carries METRICS_SCRAPE_TOKEN (>=32 chars)"
[ "$(stat -c %a "$QA_DIR/qa.env")" = 600 ]; ok $? "qa.env stays mode 0600"
bad=0; [ -n "$tok" ] && grep -qF -- "$tok" "$STUB_LOG" && { echo "  scrape token value leaked in argv"; bad=1; }
grep -q -- '-e METRICS_SCRAPE_TOKEN ' "$STUB_LOG" || { echo "  METRICS_SCRAPE_TOKEN not passed by name to the api"; bad=1; }
ok $bad "scrape token passed by name, never as a value"
grep -qE -- '-e TRUSTED_PROXIES=[0-9a-fA-F:./,]+ ' "$STUB_LOG" && ! grep -qE -- 'TRUSTED_PROXIES=(auto|0\.0\.0\.0/0) ' "$STUB_LOG"; ok $? "TRUSTED_PROXIES is an explicit CIDR list (not auto, not 0.0.0.0/0)"
# an EXISTING qa.env without the token (stack already running) gets the token appended, other secrets untouched
setup t5b; ( umask 077; printf 'ADMIN_USERNAME=qa_admin\nADMIN_PASSWORD=keepme\nJWT_SECRET=keepjwt\nPOSTGRES_USER=c\nPOSTGRES_PASSWORD=keeppg\nPOSTGRES_DB=c\nREGISTRATION_MODE=approval\n' > "$QA_DIR/qa.env" )
run_up
t2="$(grep '^METRICS_SCRAPE_TOKEN=' "$QA_DIR/qa.env" | cut -d= -f2-)"
[ -n "$t2" ] && [ "${#t2}" -ge 32 ] && grep -q '^ADMIN_PASSWORD=keepme$' "$QA_DIR/qa.env" && grep -q '^JWT_SECRET=keepjwt$' "$QA_DIR/qa.env" && [ "$(stat -c %a "$QA_DIR/qa.env")" = 600 ]; ok $? "legacy qa.env: token appended, existing secrets and mode kept"
run_up; t3="$(grep '^METRICS_SCRAPE_TOKEN=' "$QA_DIR/qa.env" | cut -d= -f2-)"
[ -n "$t2" ] && [ -n "$t3" ] && [ "$t3" = "$t2" ]; ok $? "re-run keeps the same token (idempotent)"

# 6. pprof is opt-in per run: HELIX_PPROF_ENABLED reaches the api only when the caller sets it (by name), never by default
setup t6; run_up; ! grep -q 'HELIX_PPROF_ENABLED' "$STUB_LOG"; ok $? "pprof env NOT passed by default"
setup t6b; HELIX_PPROF_ENABLED=true run_up; grep -q -- '-e HELIX_PPROF_ENABLED ' "$STUB_LOG" && ! grep -q 'HELIX_PPROF_ENABLED=' "$STUB_LOG"; ok $? "pprof env passed by name when the caller sets it"
# 7. F4: default TRUSTED_PROXIES never contains loopback (127.0.0.0/8, ::1); a host-local client could otherwise forge X-Forwarded-For through the proxy
tp() { ( STUB_IP_SRC="$1"; export STUB_IP_SRC; unset TRUSTED_PROXIES; export PATH="$BIN:$PATH"; . "$HERE/lib.sh" >/dev/null 2>&1; printf '%s' "${TRUSTED_PROXIES-<unset>}" ); }
v="$(tp 192.168.1.115)"; [ "$v" = 192.168.1.115/32 ]; ok $? "default TRUSTED_PROXIES = detected host address /32 (got '$v')"
printf '%s' "$v" | grep -qE '(^|,)(127\.|::1|0\.0\.0\.0|localhost)'; [ $? != 0 ]; ok $? "default TRUSTED_PROXIES excludes 127.0.0.0/8, ::1 and 0.0.0.0"
v="$(tp "")"; [ "$v" = "" ]; ok $? "no detectable host address -> TRUSTED_PROXIES empty (trust nothing), not loopback (got '$v')"
v="$( STUB_IP_SRC=; unset TRUSTED_PROXIES; export PATH="$BIN:$PATH"; TRUSTED_PROXIES=10.1.2.3/32; . "$HERE/lib.sh" >/dev/null 2>&1; printf '%s' "$TRUSTED_PROXIES")"; [ "$v" = 10.1.2.3/32 ]; ok $? "explicit TRUSTED_PROXIES override is kept"
# 8. R-1: up.sh with NO detectable host address passes an EMPTY TRUSTED_PROXIES (trust nothing) and never a loopback/wildcard fallback
setup t8; export STUB_IP_SRC=""; unset TRUSTED_PROXIES; run_up
grep -qE -- '-e TRUSTED_PROXIES= ' "$STUB_LOG"; ok $? "no host address -> up.sh passes -e TRUSTED_PROXIES= (empty)"
grep -oE -- 'TRUSTED_PROXIES=[^ ]*' "$STUB_LOG" | grep -qE '127\.|::1|0\.0\.0\.0'; [ $? != 0 ]; ok $? "no host address -> no 127./::1/0.0.0.0 in the TRUSTED_PROXIES argument"
echo "passed=$pass failed=$fail"; [ "$fail" = 0 ]
