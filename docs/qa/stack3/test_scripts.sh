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
chmod +x "$BIN"/*

setup() { # <name>  -> fresh sandbox dirs + env
  T="$ROOT/${1:?}"; mkdir -p "$T/qa" "$T/media" "$T/stub"
  export STUB_LOG="$T/log" STUB_DIR="$T/stub" QA_DIR="$T/qa" MEDIA_DIR="$T/media" BUSY_PORTS="" CURL_CODE=200
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

echo "passed=$pass failed=$fail"; [ "$fail" = 0 ]
