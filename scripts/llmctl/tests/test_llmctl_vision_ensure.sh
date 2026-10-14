#!/usr/bin/env bash
# Behavioural test of scripts/llmctl/llmctl_vision_ensure.sh against a fake llmctl (real executable) and real
# loopback HTTP servers. Every verdict leg + the "never starts without --start / never starts a GATED profile" rule.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUT="$HERE/../llmctl_vision_ensure.sh"
T="$(mktemp -d)"; trap 'kill $(jobs -p) 2>/dev/null; rm -rf "$T"' EXIT
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); echo "ok   $1"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $1"; }
expect_rc() { # name want got
  if [ "$3" = "$2" ]; then ok "$1 (rc=$3)"; else bad "$1 (want rc=$2 got rc=$3)"; fi
}
mkdir -p "$T/bin"
for x in curl python3 bash sed env cat dirname; do p="$(command -v $x)" && ln -s "$p" "$T/bin/$x"; done
SAFEPATH="$T/bin"
closed_url() { python3 -I -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print("http://127.0.0.1:%d"%s.getsockname()[1]);s.close()'; }
fake() { # dir planjson -> creates $dir/llmctl logging argv1
  mkdir -p "$1"; printf '%s' "$2" > "$1/plan.json"
  cat > "$1/llmctl" <<'F'
#!/bin/sh
D="$(dirname "$0")"
echo "$1" >> "$D/calls.log"
case "$1" in
  plan) cat "$D/plan.json";;
  start) exit 0;;
  setup) echo "setup-ran" >> "$D/calls.log"; exit 0;;
  *) exit 9;;
esac
F
  chmod +x "$1/llmctl"
}
GATED='{"tier":"below-minimum","budgets":{"ram_mb":5000},"profiles":{"vision":{"fits":false,"tier_ok":false,"min_tier":"baseline","ram_mb":2048,"vram_mb":4030}}}'
NOFIT='{"tier":"baseline","budgets":{"ram_mb":1000},"profiles":{"vision":{"fits":false,"tier_ok":true,"min_tier":"baseline","ram_mb":2048,"vram_mb":4030}}}'
RUNN='{"tier":"baseline","budgets":{"ram_mb":9000},"profiles":{"vision":{"fits":true,"tier_ok":true,"min_tier":"baseline","ram_mb":2048,"vram_mb":4030}}}'
run() { env -i HOME="$T" PATH="$SAFEPATH" "$@"; }

U="$(closed_url)"

# 1 ABSENT
run LLMCTL_VISION_URL="$U" LLMCTL_VENDORED_DIR="$T/none" "$SUT" >"$T/o" 2>&1; expect_rc "ABSENT" 3 $?
grep -q "ABSENT" "$T/o" && ok "ABSENT message" || bad "ABSENT message"
# 2 env set but unusable must not fall through to PATH
fake "$T/p" "$RUNN"; ln -s "$T/p/llmctl" "$T/bin/llmctl"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/missing" "$SUT" >"$T/o" 2>&1; expect_rc "env-unusable" 3 $?
rm -f "$T/bin/llmctl"
# 3 GATED: never starts, even with --start
fake "$T/g" "$GATED"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/g/llmctl" "$SUT" --start >"$T/o" 2>&1; expect_rc "GATED+--start" 4 $?
[ "$(sort -u "$T/g/calls.log")" = "plan" ] && ok "GATED ran only plan" || bad "GATED ran: $(cat "$T/g/calls.log")"
# 4 NO_FIT
fake "$T/n" "$NOFIT"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/n/llmctl" "$SUT" --start >"$T/o" 2>&1; expect_rc "NO_FIT+--start" 5 $?
[ "$(sort -u "$T/n/calls.log")" = "plan" ] && ok "NO_FIT ran only plan" || bad "NO_FIT ran: $(cat "$T/n/calls.log")"
# 5 NOT_RUNNING without --start does not start
fake "$T/r" "$RUNN"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/r/llmctl" "$SUT" >"$T/o" 2>&1; expect_rc "NOT_RUNNING" 6 $?
[ "$(sort -u "$T/r/calls.log")" = "plan" ] && ok "no start without flag" || bad "started without flag: $(cat "$T/r/calls.log")"
# 6 --start on a runnable profile invokes start (fake start does not bring a server up -> UNHEALTHY)
: > "$T/r/calls.log"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/r/llmctl" "$SUT" --start >"$T/o" 2>&1; expect_rc "start-but-no-server" 7 $?
grep -qx start "$T/r/calls.log" && ok "--start invoked llmctl start" || bad "--start did not invoke start"
# 7 READY (real server) wins before llmctl is consulted
free_port() { python3 -I -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }
P1="$(free_port)"; P2="$(free_port)"
mkdir -p "$T/www"; echo '{"status":"ok"}' > "$T/www/health"
( cd "$T/www" && exec python3 -I -m http.server "$P1" --bind 127.0.0.1 ) >/dev/null 2>&1 &
sleep 0.8
fake "$T/r2" "$GATED"
run LLMCTL_VISION_URL="http://127.0.0.1:$P1" LLMCTL_BIN="$T/r2/llmctl" "$SUT" --json >"$T/o" 2>&1; expect_rc "READY" 0 $?
python3 -I -c 'import json,sys; d=json.load(open(sys.argv[1])); assert d["state"]=="READY" and d["exit"]==0' "$T/o" && ok "READY json" || bad "READY json: $(cat "$T/o")"
[ ! -e "$T/r2/calls.log" ] && ok "READY did not even call llmctl" || bad "READY called llmctl"
# 8 UNHEALTHY: server answers non-200 on /health (404 on a dir without the file)
mkdir -p "$T/www2"
( cd "$T/www2" && exec python3 -I -m http.server "$P2" --bind 127.0.0.1 ) >/dev/null 2>&1 &
sleep 0.8
run LLMCTL_VISION_URL="http://127.0.0.1:$P2" LLMCTL_BIN="$T/r/llmctl" "$SUT" >"$T/o" 2>&1; expect_rc "UNHEALTHY" 7 $?
# 9 unreadable plan
fake "$T/u" 'not json'
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/u/llmctl" "$SUT" >"$T/o" 2>&1; expect_rc "UNKNOWN" 8 $?
# 10 usage
run "$SUT" --bogus >"$T/o" 2>&1; expect_rc "usage" 2 $?
# 11 --setup runs `llmctl setup` (never a start)
: > "$T/r/calls.log"
run LLMCTL_VISION_URL="$U" LLMCTL_BIN="$T/r/llmctl" "$SUT" --setup >"$T/o" 2>&1; expect_rc "setup" 6 $?
grep -qx setup "$T/r/calls.log" && ! grep -qx start "$T/r/calls.log" && ok "setup ran, start did not" || bad "setup/start mix: $(cat "$T/r/calls.log")"

echo "passed=$PASS failed=$FAIL"
[ "$FAIL" = 0 ]
