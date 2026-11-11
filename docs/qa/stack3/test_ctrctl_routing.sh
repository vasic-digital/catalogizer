#!/usr/bin/env bash
# CAT-QA-STACK-LIFECYCLE-1: stack3 up.sh / sonarqube/up.sh must START containers through qa_stack_start (ctrctl over the Containers submodule), never `podman run -d`.
# (1) static guard: no raw `podman run -d` / `podman start` in the migrated scripts. (2) behaviour: a shim records calls; every container gets `podman create` then
# a start via the helper (shim ctrctl via QA_STACK_BACKEND=containers is NOT used: no go build here; the podman fallback is forced and the helper's name guard is exercised).
# usage: test_ctrctl_routing.sh [--root DIR]   (--root runs against a mutated copy of docs/qa/stack3; used by test_ctrctl_routing_mutations.sh)
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[ "${1:-}" = --root ] && HERE="${2:?}"
pass=0; fail=0
ok() { if [ "$1" = 0 ]; then pass=$((pass+1)); echo "PASS $2"; else fail=$((fail+1)); echo "FAIL $2"; fi; }
# FU-6: the raw-start detector joins `\`-continued lines, ignores comments, and flags ANY podman invocation that runs (`run`, `container run`) with a
# detach flag in ANY position (-d, -dt, --detach) or starts a container (`start`, `container start`). Self-tested against golden-bad/golden-good lines below.
raw_start() { # <file>; exit 0 = a raw detached run / start was found
  sed -e ':a' -e '/\\$/{N;s/\\\n//;ba}' "$1" | grep -E '^[^#]*\bpodman\b' | grep -E '\bpodman +(container +)?(start|run)\b|\bpodman\b.*\brun\b.*( -[A-Za-z]*d[A-Za-z]*( |$)| --detach( |=|$))|\bpodman\b.*( -[A-Za-z]*d[A-Za-z]*| --detach).*\brun\b' >/dev/null
}
selftest_dir="$(mktemp -d "${TMPDIR:-/tmp}/ctrdet.XXXXXX")"; : "${selftest_dir:?}"
while IFS='|' read -r exp line; do
  printf '%b\n' "$line" > "$selftest_dir/l"; raw_start "$selftest_dir/l"; r=$?
  [ "$exp" = BAD ] && [ "$r" = 0 ] && ok 0 "detector self-test flags: $line"
  [ "$exp" = BAD ] && [ "$r" != 0 ] && ok 1 "detector self-test flags: $line"
  [ "$exp" = GOOD ] && [ "$r" != 0 ] && ok 0 "detector self-test ignores: $line"
  [ "$exp" = GOOD ] && [ "$r" = 0 ] && ok 1 "detector self-test ignores: $line"
done <<'LINES'
BAD|podman run -d --pod p img
BAD|podman run --pod p --name x img -d
BAD|podman run --detach --name x img
BAD|podman run -dt --name x img
BAD|podman --log-level=debug run --pod p -d img
BAD|podman container run -d img
BAD|podman container start x
BAD|podman start x
BAD|podman run \\\n  --pod p -d img
GOOD|# podman run -d is forbidden here
GOOD|podman create --pod p --name x img
GOOD|podman pod create --name p -p 1:2
GOOD|podman logs --tail 20 x
GOOD|podman inspect --format '{{.State.Status}}' x
LINES
rm -rf -- "${selftest_dir:?}"
for f in up.sh sonarqube/up.sh; do
  raw_start "$HERE/$f"; [ $? -ne 0 ]; ok $? "static: $f has no raw podman run -d / start (any flag position, container subcommand)"
  grep -q 'ctr_up ' "$HERE/$f"; ok $? "static: $f starts containers via ctr_up"
done
for f in lib.sh sonarqube/lib.sh; do
  grep -q 'qa_stack_start "\$n"' "$HERE/$f"; ok $? "static: $f ctr_up delegates the start to qa_stack_start"
  raw_start "$HERE/$f"; [ $? -ne 0 ]; ok $? "static: $f ctr_up has no raw podman run/start"
done
# behaviour: source lib.sh with a podman shim, call ctr_up, assert create + (helper-routed) start, and that a start failure propagates.
ROOT="$(mktemp -d "${TMPDIR:-/tmp}/ctrroute.XXXXXX")"; : "${ROOT:?}"; trap 'rm -rf -- "${ROOT:?}"' EXIT
mkdir -p "$ROOT/bin"
cat > "$ROOT/bin/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "$SHIM_LOG"
case "$1" in
  create) exit "${SHIM_CREATE_RC:-0}";;
  start) exit "${SHIM_START_RC:-0}";;
  inspect) case "$*" in *Health*) echo healthy;; *) echo running;; esac;;
esac
exit 0
S
chmod +x "$ROOT/bin/podman"
for lib in lib.sh sonarqube/lib.sh; do
  case "$lib" in lib.sh) pfx=catqa3;; *) pfx=catsonar;; esac
  for mode in ok startfail createfail; do
    export SHIM_LOG="$ROOT/log.$pfx.$mode"; : > "$SHIM_LOG"; unset SHIM_START_RC SHIM_CREATE_RC; [ "$mode" = startfail ] && export SHIM_START_RC=1; [ "$mode" = createfail ] && export SHIM_CREATE_RC=1
    ( export PATH="$ROOT/bin:$PATH" QA_STACK_BACKEND=podman QA_DIR="$ROOT/qa" SQ_DIR="$ROOT/sq" HOME="$HOME"; . "$HERE/$lib" 2>/dev/null; ctr_up "$pfx-api" --memory 1g img >/dev/null 2>&1 ); rc=$?
    if [ "$mode" = ok ]; then
      [ "$rc" = 0 ] && grep -q "^podman create --pod $pfx --name $pfx-api --memory 1g img" "$SHIM_LOG" && grep -q "^podman start $pfx-api" "$SHIM_LOG" && ! grep -qE '^podman run' "$SHIM_LOG"; ok $? "behaviour[$lib]: create then helper start, no podman run"
    elif [ "$mode" = createfail ]; then   # FU-7: a failed `podman create` must propagate and must NOT reach any start
      [ "$rc" != 0 ] && grep -q "^podman create " "$SHIM_LOG" && ! grep -qE '^podman (start|inspect)' "$SHIM_LOG"; ok $? "behaviour[$lib]: create failure propagates (rc=$rc) and nothing is started"
    else
      [ "$rc" != 0 ] && grep -q "^podman logs --tail 20 $pfx-api" "$SHIM_LOG"; ok $? "behaviour[$lib]: start failure propagates (rc=$rc) and logs are shown"
    fi
  done
  export SHIM_LOG="$ROOT/log.$pfx.foreign"; : > "$SHIM_LOG"; unset SHIM_START_RC SHIM_CREATE_RC
  ( export PATH="$ROOT/bin:$PATH" QA_STACK_BACKEND=podman QA_DIR="$ROOT/qa" SQ_DIR="$ROOT/sq"; . "$HERE/$lib" 2>/dev/null; ctr_up "otherstack-api" img >/dev/null 2>&1 ); rc=$?
  [ "$rc" != 0 ] && ! grep -q '^podman start' "$SHIM_LOG" && ! grep -q '^podman create' "$SHIM_LOG"; ok $? "behaviour[$lib]: name outside the stack prefix is refused (nothing created, nothing started)"  # FU-9
done
# FU-8: run the real up.sh with a recording shim and diff every `podman create` argv against the golden list (flags like --restart / --health-cmd / -v / -e are preserved).
# Golden lives next to THIS script (not under --root), paths are normalised to @ROOT@. Regenerate on purpose: GOLDEN_UPDATE=1 test_ctrctl_routing.sh.
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
G="$ROOT/gold"; mkdir -p "$G/bin" "$G/qa/website_dist" "$G/media"; echo "<html></html>" > "$G/qa/website_dist/index.html"; echo deadbeefcafe0123 > "$G/qa/BUILT_COMMIT"
cat > "$G/bin/podman" <<'S'
#!/usr/bin/env bash
case "$1 $2" in "pod exists") exit 1;; "image exists") exit 0;; esac
[ "$1" = create ] && { printf 'podman'; printf ' [%s]' "$@"; echo; } >> "$GOLD_LOG"
case "$1" in inspect) case "$*" in *Health*) echo healthy;; *) echo running;; esac;; esac
exit 0
S
printf '#!/usr/bin/env bash\nprintf 200\n' > "$G/bin/curl"; printf '#!/usr/bin/env bash\nexit 0\n' > "$G/bin/sleep"
printf '#!/usr/bin/env bash\nexit 0\n' > "$G/bin/ss"; printf '#!/usr/bin/env bash\nexit 1\n' > "$G/bin/ip"; chmod +x "$G"/bin/*
export GOLD_LOG="$G/create.log"; : > "$GOLD_LOG"
( export PATH="$G/bin:$PATH" QA_STACK_BACKEND=podman QA_DIR="$G/qa" MEDIA_DIR="$G/media"; unset TRUSTED_PROXIES QA_LAN_IP HELIX_PPROF_ENABLED; bash "$HERE/up.sh" gold >"$G/out" 2>"$G/err" ); urc=$?
sed -e "s#$G#@ROOT@#g" "$GOLD_LOG" > "$G/create.norm"
[ "$urc" = 0 ]; ok $? "golden: up.sh ran to completion under the recording shim (rc=$urc)"
if [ "${GOLDEN_UPDATE:-0}" = 1 ]; then mkdir -p "$SELF/golden"; cp "$G/create.norm" "$SELF/golden/up_create_argv.txt"; fi
diff -u "$SELF/golden/up_create_argv.txt" "$G/create.norm" >"$G/diff" 2>&1; ok $? "golden: every container's podman create argv equals golden/up_create_argv.txt"; [ -s "$G/diff" ] && sed 's/^/    /' "$G/diff" | head -20
# FU-11: the backend (go build of ctrctl) is resolved ONCE per script run, not once per qa_stack_start. A counting `go` shim stands in for the build.
B="$ROOT/bk"; mkdir -p "$B/bin" "$B/repo/submodules/containers/pkg/runtime" "$B/repo/docs/qa/lib/ctrctl" "$B/cache"
: > "$B/repo/submodules/containers/pkg/runtime/podman.go"; : > "$B/repo/docs/qa/lib/ctrctl/main.go"
cat > "$B/bin/go" <<'S'
#!/usr/bin/env bash
echo build >> "$GO_COUNT"; out=""; while [ $# -gt 0 ]; do [ "$1" = -o ] && out="$2"; shift; done
printf '#!/usr/bin/env bash\nexit 0\n' > "$out"; chmod +x "$out"
S
chmod +x "$B/bin/go"
for lib in lib.sh sonarqube/lib.sh; do
  case "$lib" in lib.sh) pfx=catqa3;; *) pfx=catsonar;; esac
  export GO_COUNT="$B/count.$pfx" SHIM_LOG="$B/log.$pfx"; : > "$GO_COUNT"; : > "$SHIM_LOG"; unset SHIM_START_RC SHIM_CREATE_RC
  ( export PATH="$B/bin:$ROOT/bin:$PATH" QA_REPO_ROOT="$B/repo" QA_CTRCTL_CACHE="$B/cache" QA_STACK_BACKEND=auto QA_DIR="$ROOT/qa" SQ_DIR="$ROOT/sq"; . "$HERE/$lib" 2>/dev/null
    ctr_up "$pfx-a" img >/dev/null 2>&1 && ctr_up "$pfx-b" img >/dev/null 2>&1 && ctr_up "$pfx-c" img >/dev/null 2>&1 ); rc=$?
  n="$(wc -l < "$GO_COUNT")"; [ "$rc" = 0 ] && [ "$n" = 1 ]; ok $? "perf[$lib]: three ctr_up calls build ctrctl exactly once (rc=$rc builds=$n)"
done
echo "pass=$pass fail=$fail"; [ "$fail" = 0 ]
