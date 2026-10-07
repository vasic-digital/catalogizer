#!/usr/bin/env bash
# test_dispatch_c.sh - T005a/T005b (round c) + T089a cases for the additions to scripts/build/dispatch.sh, remote/emit.sh and the new
# scripts/build/{event_hub.sh,run_callback.sh,lib/snapshot.py,lib/treecache.py}: git-plumbing snapshot and input closure shipped to the host
# (u, u2, w), the callback runner, the event hub, build groups, emitter restart, keyring refill, host-key pinning, peak_rss_bytes, the quiet-build
# option, and the binding of every dispatched build to the long-op registry (T089a). Host control-plane test (P0-P1 host exception, as T003).
# The earlier cases stay in test_dispatch.sh / test_dispatch_events.sh (unchanged).
# Usage: test_dispatch_c.sh | ONLY="c1 c2" test_dispatch_c.sh        *_SCRIPT variables override the files under test (mutation runs).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
B=${BUILD_DIR:-$here/..}
DSP=${DISPATCH_SCRIPT:-$B/dispatch.sh}
EMIT=${EMIT_SCRIPT:-$B/remote/emit.sh}
EVW=${EVWAIT_SCRIPT:-$B/lib/evwait.py}
HUB=${HUB_SCRIPT:-$B/event_hub.sh}
RUNCB=${RUNCB_SCRIPT:-$B/run_callback.sh}
CORE="$B/event_core.sh"
[ -f "$DSP" ] || { echo "FAIL script absent: $DSP"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }
for t in jq python3 flock openssl tar sha256sum git; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL dependency missing: $t"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }; done
pass=0; fail=0; skipn=0
say() { printf '%s\n' "$1"; }
ok()  { pass=$((pass+1)); say "ok   $1"; }
bad() { fail=$((fail+1)); say "FAIL $1${2:+ -- $2}"; }
chk() { local what=$1; shift; if "$@"; then ok "$what"; else bad "$what"; fi; }
want() { [ -z "${ONLY:-}" ] && return 0; case " $ONLY " in *" $1 "*) return 0;; esac; return 1; }
tmproot=$(mktemp -d "${TMPDIR:-/tmp}/tdc.XXXXXX"); trap 'cleanup' EXIT
cleanup() { kill_ours; if [ -n "${KEEP:-}" ]; then echo "kept $tmproot"; else rm -rf "$tmproot"; fi; }
kill_pidfile() { local f=$1 p s needle=${2:-}; [ -s "$f" ] || return 0; read -r p s < "$f"; [ "${p:-0}" -gt 1 ] 2>/dev/null || return 0
  [ "$(awk '{print $22}' /proc/$p/stat 2>/dev/null)" = "$s" ] || return 0
  [ -z "$needle" ] || tr '\0' ' ' < /proc/$p/cmdline 2>/dev/null | grep -q "$needle" || return 0
  kill -TERM "$p" 2>/dev/null; }
kill_ours() { # exact pids, proven ours by start time (and cmdline for hubs): never a pattern kill, never a group signal
  local f; for f in "$tmproot"/*/builds/b-*/pump.pid "$tmproot"/*/builds/b-*/remote/daemon.pid "$tmproot"/*/builds/b-*/remote/build.pid "$tmproot"/*/builds/hub.pid "$tmproot"/*/fakehome/.cache/catalogizer/builds/*/daemon.pid; do
    kill_pidfile "$f"; done; }
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
G() { git -c protocol.file.allow=always -c user.name=t -c user.email=t@t "$@"; }

# ---- sandbox (as test_dispatch.sh) ----
mk() { # name : sets T and the environment; src is a plain directory
  T="$tmproot/$1"; mkdir -p "$T/state" "$T/src" "$T/fake/remote" "$T/fake/lib" "$T/fakehome" "$T/log"
  printf 'payload-%s\n' "$1" > "$T/src/file.txt"
  ln -sf "$EVW" "$T/fake/lib/evwait.py"
  printf '#!/usr/bin/env bash\n. %q\n%s\nmain "$@"\n' "$EMIT" "$(fake_daemon)" > "$T/fake/remote/emit.sh"; chmod +x "$T/fake/remote/emit.sh"
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\ntic-group-record\t-\tcb-tic-group-record\tbuild-callback\t60\n' > "$T/callbacks.tsv"
  export DISPATCH_BUILDS_ROOT="$T/builds" DISPATCH_STATE_DIR="$T/state" DISPATCH_CALLBACKS_TSV="$T/callbacks.tsv" DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1 \
         DISPATCH_EMIT_DIR="$T/fake" DISPATCH_DISK_OUT="$T/disk" DISPATCH_HOSTS_FILE="$T/hosts.env" DISPATCH_RECONNECT_DELAY=0 DISPATCH_RECONNECTS=1 DISPATCH_JOBS=4 \
         FAKE_LOG="$T/log" FAKE_MODE=ok DISPATCH_EVWAIT="$EVW" EVWAIT="$EVW" DISPATCH_LONGOPS_DIR="$T/longops" DISPATCH_HUB=0
  unset DISPATCH_SSH
}
fake_daemon() { cat <<'XEOF'
hbev() { emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" heartbeat "$(jq -nc --argjson p "$1" --argjson s "$2" --argjson e "$3" '{progress_offset:$p,stage:$s,elapsed_monotonic_ms:$e}')"; }
done_ev() { local m x; m=$(manifest_sha "$DIR/out/artifacts"); [ -z "${1:-}" ] || m=$1; x=${2:-succeeded}
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" completed "$(jq -nc --arg m "$m" --arg l "$(printf '%064d' 0)" --arg x "$x" '{exit_class:$x,artifact_manifest_sha256:$m,image_digest:("sha256:"+$m),remote_log_sha256:$l}')"; }
daemon() {
  read -r KEY; mkdir -p "$DIR/out/artifacts"; printf 'started\n' >> "$FAKE_LOG/daemon.$BID"
  { ls "${LONGOPS_DIR:-/nonexistent}/ops" 2>/dev/null | tr '\n' ' '; } > "$FAKE_LOG/regseen.$BID"
  printf '%s %s\n' "$$" "$(pid_start $$)" > "$DIR/daemon.pid"
  trap 'printf "cancelled\n" >> "$FAKE_LOG/cancel.$BID"; kill $(jobs -p) 2>/dev/null; exit 0' TERM
  printf 'artifact\n' > "$DIR/out/artifacts/a.txt"
  printf '%s start\n' "$BID" >> "$FAKE_LOG/order.log"
  [ ! -d "$SRC" ] || ( cd "$SRC" && LC_ALL=C find . -type f -o -type l | LC_ALL=C sort ) > "$FAKE_LOG/srcls.$BID"
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" accepted
  case "${FAKE_MODE:-ok}" in
    ok)       hbev 100 1 500; hbev 200 2 1000; done_ev;;
    fail)     hbev 100 1 500; done_ev "" test_failed;;
    buildfail) hbev 100 1 500; done_ev "" build_failed;;
    silent)   sleep 600 & wait $!;;
    flat)     local e=0; while :; do e=$((e+1000)); hbev 5 1 $e; sleep 1 & wait $!; done;;
    slow)     local e=0 p=0; while :; do e=$((e+1000)); p=$((p+10)); hbev $p $p $e; sleep 1 & wait $!; done;;
    hold)     local t0=$(mono_ms) p=100; hbev 100 1 500
              while [ ! -e "$FAKE_LOG/release.$BID" ]; do p=$((p+1)); hbev $p $p $(( $(mono_ms) - t0 + 500 )); sleep 0.5 & wait $!; done
              printf '%s end\n' "$BID" >> "$FAKE_LOG/order.log"; done_ev "" "${FAKE_END:-succeeded}";;
  esac
  exit 0
}
XEOF
}
mkgit() { # name : like mk, but src is a git repository (root + an initialised submodule + an uninitialised one)
  mk "$1"; local up="$T/up"; mkdir -p "$up"
  for r in lib uninit; do mkdir -p "$up/$r"; ( cd "$up/$r" && G init -q -b main . && printf '%s\n' "$r" > f.txt && G add -A && G commit -qm i ); done
  rm -rf "$T/src"; mkdir -p "$T/src"
  ( cd "$T/src" && G init -q -b main . && mkdir -p svc web && printf 'module x\nreplace example.com/lib => ../submodules/lib\n' > svc/go.mod && printf 'package main\n' > svc/main.go \
      && printf 'w\n' > web/index.js && printf '/.audit/\n' > .gitignore
    mkdir -p submodules && G submodule add -q "$up/lib" submodules/lib && G submodule add -q "$up/uninit" submodules/uninit && G add -A && G commit -qm root
    G submodule deinit -q -f submodules/uninit ); }
sdig() { bash "$DSP" snapshot "$T/src"; }
adig() { bash "$DSP" argv-digest fake-build x; }
pk() { local it=${1:-1} lane=${2:-lane}; printf 'build:app:%s:%s:%s:primary:%s' "$lane" "$(sdig)" "$(adig)" "$it"; }
sub() { # iteration extra-args... : submit, print the build id
  local it=$1; shift
  bash "$DSP" submit --purpose "$(pk "$it" "${LANE:-lane}")" --callback "${CB:-record-only}" --image IMG-GO --src "$T/src" --heartbeat 1 --no-progress-budget "${BUDGET:-3}" --wallclock-cap "${WALL:-60}" "$@" -- fake-build x 2>>"$T/log/sub.err"; }
bd() { echo "$T/builds/$1"; }
tkind() { jq -r .kind "$(bd "$1")/terminal/state.json" 2>/dev/null; }
tclass() { jq -r .exit_class "$(bd "$1")/terminal/state.json" 2>/dev/null; }
treason() { jq -r .digest "$(bd "$1")/terminal/state.json" 2>/dev/null; }
effects() { ls "$(bd "$1")/effects" 2>/dev/null | wc -l; }
waitb() { bash "$DSP" wait "$1" "${2:-40}" >/dev/null 2>&1; }
tx() { jq -r ".transfer.$2" "$(bd "$1")/submit.json"; }
# test-side BOUNDED waits for a condition (never a fixed sleep: the host may be loaded): a held build is ready once its first heartbeat has been consumed
holdready() { local id i; for id in "$@"; do for i in $(seq 1 400); do [ -e "$T/builds/$id/consumed/2" ] && break; sleep 0.1; done; done; }
opwait_() { local i; for i in $(seq 1 300); do [ "$(jq -r .state "$T/longops/ops/$1.json" 2>/dev/null)" = "$2" ] && return 0; sleep 0.1; done; return 1; }   # the registry op ends just after the callback

# ================================================================ c1-c5: git-plumbing snapshot, closure and shipping through the dispatcher
if want c1; then mkgit c1
  d0=$(sdig); chk "c1 (u) the snapshot digest of a git source is the manifest digest of snapshot.py (64 hex)" test "${#d0}" = 64 -a "$d0" = "$(python3 -I "$B/lib/snapshot.py" digest --root "$T/src" --exclude specs/001-full-project-audit-remediation/evidence --exclude specs/001-full-project-audit-remediation/audit)"
  printf 'x\n' >> "$T/src/svc/main.go"; chk "c1 an uncommitted edit changes the digest (one HEAD, two trees, two keys)" test "$(sdig)" != "$d0"
  git -C "$T/src" checkout -q svc/main.go; printf 'dirty\n' >> "$T/src/submodules/lib/f.txt"; chk "c1 (u) one dirty submodule edit changes the digest" test "$(sdig)" != "$d0"
  git -C "$T/src/submodules/lib" checkout -q f.txt
  id=$(sub 1 --component svc); waitb "$id" 30
  chk "c1 (u) a git source build completes and the callback ran once" test "$(tkind "$id")" = completed -a "$(effects "$id")" = 1
  chk "c1 (u) the Go module of the root with a replace into submodules/ builds: the submodule's tree is shipped into its path" grep -qx './submodules/lib/f.txt' "$T/log/srcls.$id"
  chk "c1 (w) a build of the web component ships the root only: no submodule file arrives" test "$(id2=$(sub 2 --component web); waitb "$id2" 30; grep -c './submodules/' "$T/log/srcls.$id2")" = 0
  chk "c1 the shipped tree carries no .git entry and no ignored file" test "$(grep -c '/\.git/\|/\.git$\|\.audit' "$T/log/srcls.$id")" = 0
fi
if want c2; then mkgit c2
  id=$(sub 1 --component svc); waitb "$id" 30; t1=$(tx "$id" objects); id=$(sub 2 --component svc); waitb "$id" 30; t2=$(tx "$id" objects)
  chk "c2 (w) the first submit sends objects; a second submit of the unchanged checkout sends none" test "$t1" -gt 4 -a "$t2" = 0
  chk "c2 (w) the transfer is recorded with its byte count in the submit record" test "$(tx "$id" bytes)" -ge 0 -a "$(jq -r '.transfer.objects' "$(bd "$id")/submit.json")" = 0
  printf 'changed\n' >> "$T/src/svc/main.go"; id=$(sub 3 --component svc); waitb "$id" 30
  chk "c2 (w) one changed source file ships one blob and the trees above it (svc, root)" test "$(tx "$id" objects)" = 3
  chk "c2 (w) a changed evidence file changes no compile-class snapshot and ships nothing" test "$(mkdir -p "$T/src/specs/001-full-project-audit-remediation/evidence"; printf 'e\n' > "$T/src/specs/001-full-project-audit-remediation/evidence/x"; a=$(sdig); printf 'e2\n' >> "$T/src/specs/001-full-project-audit-remediation/evidence/x"; [ "$a" = "$(sdig)" ] && echo same)" = same
fi
if want c3; then mkgit c3
  printf 'replace example.com/u => ../submodules/uninit\n' >> "$T/src/svc/go.mod"
  out=$(sub 1 --component svc 2>&1); rc=$?
  chk "c3 (u) an input needing an uninitialised submodule is refused at submit (20 submodule_uninitialised), no build directory left" test "$rc" = 20 -a -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')" -a -n "$(grep -l submodule_uninitialised "$T/log/sub.err")"
  git -C "$T/src" checkout -q svc/go.mod
  out=$(bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "c3 a git source submitted with no --component is refused (20 usage): the closure is the unit that ships" test "$rc" = 20 -a "${out#*usage}" != "$out"
  printf 'dirty\n' >> "$T/src/svc/main.go"
  out=$(bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" --component svc --snapshot-mode cpa -- fake-build x 2>&1); rc=$?
  chk "c3 (CPA) an undeclared dirty file inside the component's closure is refused at submit (20 undeclared_dirty_input)" test "$rc" = 20 -a "${out#*undeclared_dirty_input}" != "$out"
fi
if want c4; then mkgit c4
  id=$(sub 1 --component svc); waitb "$id" 30
  hc="$T/builds/.hostcache/trees"; d=$(ls "$hc" | head -1)
  chk "c4 the tree sits in the content-addressed cache of the (local proof) build host" test -d "$hc/$d/svc"
  printf 'tamper\n' >> "$hc/$d/svc/main.go"
  id2=$(sub 2 --component svc); waitb "$id2" 30
  chk "c4 (u) a received tree altered on the build host is refused: infra_failed, detail snapshot_mismatch, never blocked-unavailable" test "$(tkind "$id2")" = infra_failed -a "$(treason "$id2")" = snapshot_mismatch
  chk "c4 (u) no build ran on the altered tree (daemon never started), callback once" test ! -e "$T/log/daemon.$id2" -a "$(effects "$id2")" = 1
fi

# ================================================================ c5: the callback runner (script rows, the registry at callback time, durable states)
mkcb() { # name : sandbox with a script callback "script-cb" (the script records each run; its effect is applied once through the effect key)
  mk "$1"; mkdir -p "$T/cbs"
  cat > "$T/cbs/cb_script.sh" <<'CBEOF'
#!/usr/bin/env bash
# env: CB_KIND CB_REASON CB_BUILD_DIR CB_EFFECT_KEY CB_ARGS_FILE
printf '%s %s\n' "$CB_KIND" "$CB_REASON" >> "$FAKE_LOG/cb.runs"
[ -z "${CB_SLEEP:-}" ] || sleep "$CB_SLEEP"
[ -z "${CB_EXIT:-}" ] || exit "$CB_EXIT"
mkdir -p "$CB_BUILD_DIR/effects"; t=$(mktemp "$CB_BUILD_DIR/effects/.tmp.XXXXXX"); date -u > "$t"; mv -T "$t" "$CB_BUILD_DIR/effects/$CB_EFFECT_KEY" 2>/dev/null || rm -f "$t"
CBEOF
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\nscript-cb\tcb_script.sh\tcb-script\tbuild-callback\t20\n' > "$T/callbacks.tsv"
  export RUNCB_SCRIPT_ROOT="$T/cbs" DISPATCH_CALLBACKS_TSV="$T/callbacks.tsv"; }
cbstate() { cat "$(bd "$1")/terminal/callback.state" 2>/dev/null; }
sstate() { cat "$(bd "$1")/terminal/script.state" 2>/dev/null; }
runcb() { bash "$RUNCB" "$T/builds" "$1" "${@:2}"; }
if want c5; then mkcb c5
  id=$(CB=script-cb sub 1); waitb "$id" 30; for _ in $(seq 1 100); do [ "$(sstate "$id")" = done ] && break; sleep 0.1; done
  chk "c5 a script callback runs once for a completed build, receiving the terminal kind (state done, effect applied)" test "$(sstate "$id")" = done -a "$(cat "$T/log/cb.runs")" = "completed " -a -e "$(bd "$id")/effects/cb-script-cb-$id"
  rr0=$(cat "$(bd "$id")/terminal/script.runner" 2>/dev/null); ro=$(runcb "$id" 2>&1); chk "c5 a callback found done is never re-run" test "$(wc -l < "$T/log/cb.runs")" = 1
  chk "c5 a callback found done is left untouched: the runner prints nothing and does not even claim the runner slot again" test -z "$ro" -a -n "$rr0" -a "$(cat "$(bd "$id")/terminal/script.runner")" = "$rr0"
  printf 'noeff-cb\tcb_noeffect.sh\tcb-noeff\tbuild-callback\t20\n' >> "$T/callbacks.tsv"; printf '#!/usr/bin/env bash\nexit 0\n' > "$T/cbs/cb_noeffect.sh"
  id=$(CB=noeff-cb sub 7); waitb "$id" 30; for _ in $(seq 1 100); do [ -n "$(sstate "$id")" ] && [ "$(sstate "$id")" != running ] && break; sleep 0.1; done
  chk "c5 a script that exits 0 WITHOUT applying its keyed effect is not done: failed effect_not_applied" test "$(sstate "$id")" = failed -a "$(cat "$(bd "$id")/terminal/script.reason" 2>/dev/null)" = effect_not_applied
  id=$(CB=script-cb FAKE_MODE=silent BUDGET=2 sub 2); waitb "$id" 30; for _ in $(seq 1 100); do [ "$(sstate "$id")" = done ] && break; sleep 0.1; done
  chk "c5 (n2) a HUNG build runs its script callback once with its terminal kind" test "$(tkind "$id")" = blocked-unavailable -a "$(sstate "$id")" = done -a "$(tail -1 "$T/log/cb.runs")" = "blocked-unavailable build_liveness_lost"
  id=$(CB=script-cb sub 3); for _ in $(seq 1 100); do [ "$(sstate "$id")" = done ] && break; sleep 0.1; done
  : > "$T/log/cb.runs"; id=$(CB=script-cb FAKE_MODE=hold sub 4); holdready "$id"; bash "$DSP" cancel "$id" >/dev/null 2>&1; for _ in $(seq 1 100); do [ -n "$(sstate "$id")" ] && [ "$(sstate "$id")" != running ] && break; sleep 0.1; done
  chk "c5 (n1) a cancel runs the callback once with kind cancelled, its script's exit 0 gives done" test "$(sstate "$id")" = done -a "$(cat "$T/log/cb.runs")" = "cancelled "
fi
if want c6; then mkcb c6
  id=$(FAKE_MODE=hold CB=script-cb sub 1); holdready "$id"; export CB_EXIT=7; bash "$DSP" cancel "$id" >/dev/null 2>&1; unset CB_EXIT
  for _ in $(seq 1 100); do [ "$(sstate "$id")" = failed ] && break; sleep 0.1; done
  chk "c6 (p) a callback that exits non-zero ends failed with its exit code and reason; the verdict of its build is unchanged" test "$(sstate "$id")" = failed -a "$(cat "$(bd "$id")/terminal/script.reason")" = exit_7 -a "$(tkind "$id")" = cancelled
  runcb "$id" >/dev/null 2>&1; chk "c6 (p) a failed callback is never retried silently (one run only)" test "$(wc -l < "$T/log/cb.runs")" = 1
  id=$(FAKE_MODE=hold CB=script-cb sub 2); holdready "$id"; export CB_SLEEP=30; ( bash "$DSP" cancel "$id" >/dev/null 2>&1 ) &
  for _ in $(seq 1 100); do [ "$(sstate "$id")" = running ] && break; sleep 0.1; done; unset CB_SLEEP
  chk "c6 (o) the callback is running (durable state running) while its script works" test "$(sstate "$id")" = running
  rp=$(cat "$(bd "$id")/terminal/script.runner" 2>/dev/null | cut -d' ' -f1); [ "${rp:-0}" -gt 1 ] && kill -KILL "$rp" 2>/dev/null; sleep 0.3
  rm -f "$T/log/cb.runs"; runcb "$id" >/dev/null 2>&1
  chk "c6 (o) a crash in the middle of a callback: the restarted runner re-runs a callback found running; its keyed effect is applied once" test "$(sstate "$id")" = done -a "$(ls "$(bd "$id")/effects" | grep -c '^cb-script')" = 1
fi
if want c7; then mkcb c7
  id=$(CB=script-cb sub 1); waitb "$id" 30; for _ in $(seq 1 100); do [ "$(sstate "$id")" = done ] && break; sleep 0.1; done
  # an id the approved table lacks at callback time: refused, never run (the table the runner reads is the hub's export, here RUNCB_CALLBACKS_TSV)
  id2=$(CB=script-cb FAKE_MODE=hold sub 2); holdready "$id2"; printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\n' > "$T/approved.tsv"; rm -f "$T/log/cb.runs"
  RUNCB_CALLBACKS_TSV="$T/approved.tsv" bash "$DSP" cancel "$id2" >/dev/null 2>&1; for _ in $(seq 1 100); do [ "$(sstate "$id2")" = failed ] && break; sleep 0.1; done
  chk "c7 a callback id the approved table lacks is refused at callback time: script.state failed, reason callback_not_registered, the script never ran" test "$(sstate "$id2")" = failed -a "$(cat "$(bd "$id2")/terminal/script.reason" 2>/dev/null)" = callback_not_registered -a ! -s "$T/log/cb.runs"
  id3=$(CB=script-cb FAKE_MODE=hold sub 3); holdready "$id3"; bash "$DSP" cancel "$id3" >/dev/null 2>&1; for _ in $(seq 1 100); do [ "$(sstate "$id3")" = done ] && break; sleep 0.1; done
  chk "c7 (x) a runner started without CPA_EXEC_SHA256 writes no runner_sha256" test "$(sstate "$id3")" = done -a ! -e "$(bd "$id3")/terminal/callback.runner_sha256"
  id4=$(CB=script-cb FAKE_MODE=hold sub 4); holdready "$id4"; CPA_EXEC_SHA256=$(printf '%064d' 5) DISPATCH_RUN_ENV=1 bash "$DSP" cancel "$id4" >/dev/null 2>&1; for _ in $(seq 1 100); do [ "$(sstate "$id4")" = done ] && break; sleep 0.1; done
  chk "c7 (x) the runner started with CPA_EXEC_SHA256 stamps it as runner_sha256" test "$(cat "$(bd "$id4")/terminal/callback.runner_sha256" 2>/dev/null)" = "$(printf '%064d' 5)"
fi

# ================================================================ c8-c13: the event hub
hubenv() { export HUB_DISPATCH="$DSP" HUB_RUNCB="$RUNCB" LONGOPS_ALLOW_TMPFS=1; }
hubup() { # start a hub detached; wait (bounded, test side) for hub.pid
  hubenv; ( setsid -f bash ${HUB_XTRACE:+-x} "$HUB" run >> "$T/log/hub.log" 2>&1 < /dev/null ); local i; for i in $(seq 1 100); do [ -s "$T/builds/hub.pid" ] && read -r hp_ hs_ < "$T/builds/hub.pid" && [ "$(awk '{print $22}' /proc/${hp_:-0}/stat 2>/dev/null)" = "$hs_" ] && return 0; sleep 0.1; done; return 1; }
hubpid() { cut -d' ' -f1 "$T/builds/hub.pid" 2>/dev/null; }
hubstop() { local p; p=$(hubpid); [ "${p:-0}" -gt 1 ] 2>/dev/null && kill -TERM "$p" 2>/dev/null; local i; for i in $(seq 1 50); do [ ! -e "$T/builds/hub.pid" ] && return 0; sleep 0.1; done; return 1; }
pumpup() { [ "$(bash "$DSP" status "$1" | jq -r .pump_alive)" = true ]; }
if want c8; then mk c8; [ -f "$HUB" ] || { bad "c8 event_hub.sh absent: $HUB"; } ; hubup
  p=$(hubpid); hp=0; hs=x; read -r hp hs < "$T/builds/hub.pid" 2>/dev/null
  chk "c8 (r0) hub.pid names the live hub (pid, start time, cmdline event_hub.sh), written by the hub itself" test "${hp:-0}" -gt 1 -a "$(awk '{print $22}' /proc/$hp/stat 2>/dev/null)" = "$hs" -a -n "$(tr '\0' ' ' < /proc/$hp/cmdline 2>/dev/null | grep event_hub.sh)"
  chk "c8 the sweep's hub record hub.json {pid,start_time} names the same hub" test "$(jq -r .pid "$T/builds/hub.json" 2>/dev/null)" = "$hp"
  out=$(timeout 15 bash "$HUB" run 2>&1); rc=$?     # bounded: a second hub that is NOT refused would run for ever (the test then fails on rc, it does not hang)
  chk "c8 (r) a second hub in the same checkout is refused: exit 20 hub_already_running, the holder named" test "$rc" = 20 -a "${out#*hub_already_running}" != "$out" -a "${out#*$hp}" != "$out"
  chk "c8 (r) the holder is named through the long-op registry (holder.sh of the build-hub purpose names the hub's pid)" test "$(LONGOPS_DIR="$T/longops" bash "$root/scripts/longops/holder.sh" "$(ls "$T/longops/claims" 2>/dev/null | grep '^build-hub:' | head -1)" 2>/dev/null | jq -r .pid 2>/dev/null)" = "$hp"
  chk "c8 the hub stops on TERM and removes hub.pid and hub.json" test "$(hubstop; echo $?)" = 0 -a ! -e "$T/builds/hub.json"
  hubup; kill -KILL "$(hubpid)" 2>/dev/null; sleep 0.3; chk "c8 a killed hub leaves a stale record that names a dead process" test "$(awk '{print $22}' /proc/$(hubpid)/stat 2>/dev/null)" != "$(cut -d' ' -f2 "$T/builds/hub.pid")"
  hubup; chk "c8 (r0) a new hub starts over the stale record and the dead holder's registry claim" test -n "$(hubpid)" -a "$(awk '{print $22}' /proc/$(hubpid)/stat 2>/dev/null)" = "$(cut -d' ' -f2 "$T/builds/hub.pid")"
  hubstop
fi
if want c9; then mk c9; hubup
  id=$(FAKE_MODE=hold sub 1); holdready "$id"
  read -r p s < "$(bd "$id")/pump.pid"; kill -KILL "$p" 2>/dev/null
  for _ in $(seq 1 300); do pumpup "$id" && break; sleep 0.1; done
  chk "c9 (r0) a pump killed while its build runs is restarted by the hub: no resubmission" test "$(pumpup "$id" && echo up)" = up -a "$(wc -l < "$T/log/daemon.$id")" = 1
  : > "$T/log/release.$id"; waitb "$id" 30
  chk "c9 the build completes once, callback once, daemon started once" test "$(tkind "$id")" = completed -a "$(effects "$id")" = 1 -a "$(wc -l < "$T/log/daemon.$id")" = 1
  hubstop
fi
if want c10; then mk c10; id=$(FAKE_MODE=hold sub 1); holdready "$id"; bash "$DSP" cancel "$id" >/dev/null 2>&1; waitb "$id" 20
  # (n) a crash after the terminal claim and before the callback started: the state is `claimed`, no effect yet
  rm -rf "$(bd "$id")/effects"; printf 'claimed\n' > "$(bd "$id")/terminal/callback.state"; hubup
  for _ in $(seq 1 300); do [ "$(cbstate "$id")" = done ] && break; sleep 0.1; done
  chk "c10 (n) a callback found claimed after a crash is run by the hub from its durable state; the keyed effect is applied once" test "$(cbstate "$id")" = done -a "$(effects "$id")" = 1
  printf 'running\n' > "$(bd "$id")/terminal/callback.state"; rm -rf "$(bd "$id")/effects"; touch "$(bd "$id")/terminal/x"; sleep 0.5
  for _ in $(seq 1 300); do [ "$(cbstate "$id")" = done ] && break; sleep 0.1; done
  chk "c10 (o) a callback found running (a crash in the middle) is re-run, the effect still applied once" test "$(cbstate "$id")" = done -a "$(effects "$id")" = 1
  hubstop
fi
if want c11; then mk c11
  export CPA_EXEC_SHA256=$(printf '%064d' 9); hubup; id=$(sub 1); waitb "$id" 30; unset CPA_EXEC_SHA256
  stamped() { [ "$(jq -r .hub_sha256 "$(bd "$id")/terminal/state.json" 2>/dev/null)" = "$(printf '%064d' 9)" ] && [ "$(for f in "$(bd "$id")"/consumed/*; do grep -c "hub_sha256=" "$f"; done | sort -u | tr -d '\n')" = 1 ]; }
  for _ in $(seq 1 300); do stamped && break; sleep 0.1; done      # bounded wait: the hub stamps on its next reconcile
  chk "c11 (x) a hub started with CPA_EXEC_SHA256 writes it as hub_sha256 into the terminal record and every consumed mark" test "$(jq -r .hub_sha256 "$(bd "$id")/terminal/state.json")" = "$(printf '%064d' 9)" -a "$(for f in "$(bd "$id")"/consumed/*; do grep -c "hub_sha256=$(printf '%064d' 9)" "$f"; done | sort -u | tr -d '\n')" = 1
  hubstop; mk c11b; hubup; id=$(sub 1); waitb "$id" 30; opwait_ "$id" complete; sleep 2     # the hub has reconciled by now (a bounded wait on the registry end, then a settle time for an absence check)
  chk "c11 (x) a hub started without it writes no stamp" test "$(jq -r '.hub_sha256 // "none"' "$(bd "$id")/terminal/state.json")" = none -a "$(cat "$(bd "$id")"/consumed/* | grep -c hub_sha256)" = 0
  hubstop
fi
if want c12; then mk c12; hubenv
  if ! command -v strace >/dev/null 2>&1; then skipn=$((skipn+1)); say "SKIP c12 (k) strace absent"; else
    # the hub is started UNDER strace (a child of this test: attaching to a running unrelated process is refused by ptrace_scope and would count nothing)
    st="$T/hub.strace"
    strace -f -qq -ttt -e trace=execve,vfork,clone,clone3 -o "$st" bash "$HUB" run >> "$T/log/hub.log" 2>&1 < /dev/null & spid=$!
    for _ in $(seq 1 100); do [ -s "$T/builds/hub.pid" ] && break; sleep 0.1; done
    # quiescence first: the start-up spawns of a hub on a loaded host can outlast a fixed pause, so wait until the trace has stopped growing for 2 s (bounded)
    q0=-1; for _ in $(seq 1 150); do q1=$(wc -l < "$st" 2>/dev/null || echo 0); [ "$q1" = "$q0" ] && [ -s "$T/builds/hub.pid" ] && { sleep 2; [ "$(wc -l < "$st")" = "$q1" ] && break; }; q0=$q1; sleep 0.5; done
    t0=$(date +%s.%N); sleep 3; t1=$(date +%s.%N)
    n=$(awk -v a="$t0" -v b="$t1" '$2 >= a && $2 <= b' "$st" 2>/dev/null | grep -c -E 'execve|vfork|clone')
    chk "c12 (k) an idle hub (no open build) starts no process in 3 s: it waits on inotify and pidfd only" test "$n" = 0
    chk "c12 (k) control: the instrument was attached (its file holds the hub's own start-up spawns)" test "$(grep -c -E 'execve|vfork|clone' "$st")" -ge 3
    strace -f -qq -ttt -e trace=execve,vfork,clone,clone3 -o "$T/needle.out" timeout 2 bash -c 'while :; do sleep 0.3; done' >/dev/null 2>&1
    t3=$(head -1 "$T/needle.out" | awk '{print $2}')
    chk "c12 (k) control needle: the same instrument and window logic see the process starts of a sleep loop" test "$(awk -v a="$t3" '$2 >= a + 0.5' "$T/needle.out" | grep -c -E 'execve|vfork|clone')" -ge 3
    hubstop; wait "$spid" 2>/dev/null
  fi
fi
if want c13; then mk c13; export DISPATCH_HUB=1 LONGOPS_ALLOW_TMPFS=1 HUB_DISPATCH="$DSP" HUB_RUNCB="$RUNCB" DISPATCH_HUB_SCRIPT="$HUB"
  id=$(sub 1); for _ in $(seq 1 50); do [ -s "$T/builds/hub.pid" ] && break; sleep 0.1; done; p1=$(hubpid)
  chk "c13 (r0) dispatch.sh submit starts the hub when none runs: hub.pid names a live hub" test "${p1:-0}" -gt 1 -a "$(awk '{print $22}' /proc/$p1/stat 2>/dev/null)" = "$(cut -d' ' -f2 "$T/builds/hub.pid")"
  waitb "$id" 30; id=$(sub 2); sleep 0.3
  chk "c13 a second submit starts no second hub" test "$(hubpid)" = "$p1"
  waitb "$id" 30; hubstop; export DISPATCH_HUB=0
fi

# ================================================================ c14-c22: build groups
gd() { echo "$T/builds/group/$1"; }
gkind() { jq -r .kind "$(gd "$1")/terminal/state.json" 2>/dev/null; }
gmembers() { jq -r '[.members[] | .kind] | join(",")' "$(gd "$1")/terminal/members.json" 2>/dev/null; }
geffects() { ls "$(gd "$1")/effects" 2>/dev/null | wc -l; }
gwait() { local i; for i in $(seq 1 ${2:-300}); do [ -s "$(gd "$1")/terminal/callback.state" ] && [ "$(cat "$(gd "$1")/terminal/callback.state")" = done ] && return 0; sleep 0.1; done; return 1; }
gsub() { # group iteration extra... : a member of a group (the callback names the GROUP callback)
  local g=$1 it=$2; shift 2; sub "$it" --group "$g" "$@"; }
rel() { : > "$T/log/release.$1"; }
if want c14; then mk c14
  a=$(FAKE_MODE=hold gsub g1 1); b=$(FAKE_MODE=hold gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; holdready "$a" "$b"
  chk "c14 (v) a member's own callback is record-only, the group has one callback registered once" test "$(jq -r .callback_id "$(bd "$a")/callback.json")" = record-only -a "$(jq -r .callback_id "$(gd g1)/callback.json")" = record-only
  rel "$a"; waitb "$a" 30; opwait_ "$a" complete; chk "c14 (v) one member finished: the group is not terminal" test ! -d "$(gd g1)/terminal"
  rel "$b"; gwait g1
  chk "c14 (v) two members finishing in either order give ONE group callback, kind completed" test "$(gkind g1)" = completed -a "$(geffects g1)" = 1
  mk c14b; a=$(FAKE_MODE=hold gsub g2 1); b=$(FAKE_MODE=hold gsub g2 2); bash "$DSP" group-seal g2 >/dev/null; holdready "$a" "$b"
  rel "$b"; waitb "$b" 30; opwait_ "$b" complete; rel "$a"; gwait g2
  chk "c14 (v) the other finishing order gives the same single group callback" test "$(gkind g2)" = completed -a "$(geffects g2)" = 1 -a "$(gmembers g2)" = "succeeded,succeeded"
  # a member callback is record-only: no member runs the group callback
  chk "c14 (v) a group callback is the only keyed effect of the group" test "$(ls "$(gd g2)/effects")" = "cb-record-only-group-g2"
  mk c14c; a=$(CB=tic-group-record FAKE_MODE=hold gsub g3 1); b=$(CB=tic-group-record FAKE_MODE=hold gsub g3 2); bash "$DSP" group-seal g3 >/dev/null; holdready "$a" "$b"
  chk "c14 (v) with a group callback that is NOT record-only, every member is still record-only and the group alone carries the group callback" test -n "$a" -a "$(jq -r .callback_id "$(bd "$a")/callback.json")" = record-only -a "$(jq -r .callback_id "$(bd "$b")/callback.json")" = record-only -a "$(jq -r .callback_id "$(gd g3)/callback.json")" = tic-group-record
  rel "$a"; rel "$b"; gwait g3
  chk "c14 (v) each member's only keyed effect is its own record-only one, the group callback ran once for the group alone" test "$(ls "$(gd g3)/effects")" = "cb-tic-group-record-group-g3" -a "$(ls "$(bd "$a")/effects")" = "cb-record-only-$a" -a "$(ls "$(bd "$b")/effects")" = "cb-record-only-$b"
fi
if want c15; then mk c15
  a=$(gsub g1 1); b=$(gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; gwait g1
  chk "c15 baseline: one group callback" test "$(geffects g1)" = 1
  # a crash between the last member's terminal state and the group's: no group terminal yet; the next evaluation (the restarted hub's sweep) claims it once
  rm -rf "$(gd g1)/terminal" "$(gd g1)/effects" "$(gd g1)/effects.log"; bash "$DSP" group-sweep >/dev/null 2>&1; gwait g1
  chk "c15 (v) a crash between two members' terminal states and the group's gives ONE group callback after the restart" test "$(gkind g1)" = completed -a "$(geffects g1)" = 1
  tb=$(stat -c '%i %w' "$(gd g1)/terminal"); bash "$DSP" group-sweep >/dev/null 2>&1; bash "$DSP" group-sweep >/dev/null 2>&1
  chk "c15 repeated sweeps never run the group callback again" test "$(geffects g1)" = 1
  chk "c15 repeated sweeps leave the claimed terminal directory itself untouched (never removed and claimed again)" test -n "$tb" -a "$(stat -c '%i %w' "$(gd g1)/terminal")" = "$tb"
fi
if want c16; then mk c16
  a=$(FAKE_MODE=silent BUDGET=12 gsub g1 1); b=$(FAKE_MODE=hold gsub g1 2); c=$(FAKE_MODE=hold gsub g1 3); bash "$DSP" group-seal g1 >/dev/null
  gwait g1 600
  chk "c16 (v) a member ending blocked-unavailable ends the group early once: the others are cancelled, the group kind is blocked-unavailable, ONE callback" test "$(gkind g1)" = blocked-unavailable -a "$(geffects g1)" = 1
  chk "c16 (v) the remaining members are recorded cancelled_by_group, and that never sets the group's kind" test "$(gmembers g1)" = "blocked-unavailable,cancelled_by_group,cancelled_by_group" -a -e "$(bd "$b")/cancelled_by_group"
  chk "c16 (v) the triggering member's reason is recorded" test "$(jq -r '.trigger.reason' "$(gd g1)/terminal/members.json")" = build_liveness_lost
fi
if want c17; then mk c17
  a=$(gsub g1 1); b=$(FAKE_MODE=fail gsub g1 2); c=$(gsub g1 3); bash "$DSP" group-seal g1 >/dev/null; gwait g1
  chk "c17 (v) one member test_failed while the others complete: one group callback carrying all three kinds, nothing cancelled, group kind test_failed" test "$(gkind g1)" = test_failed -a "$(gmembers g1)" = "succeeded,test_failed,succeeded" -a "$(geffects g1)" = 1 -a ! -e "$(bd "$a")/cancelled_by_group"
  mk c17b; a=$(FAKE_MODE=hold gsub g1 1 --cancel-on-fail); c=$(FAKE_MODE=hold gsub g1 3 --cancel-on-fail); b=$(FAKE_MODE=fail gsub g1 2 --cancel-on-fail); bash "$DSP" group-seal g1 >/dev/null; gwait g1 300
  chk "c17 (v) with --cancel-on-fail the same test_failed ends the group early: kind test_failed, the others cancelled_by_group" test "$(gkind g1)" = test_failed -a "$(gmembers g1)" = "cancelled_by_group,cancelled_by_group,test_failed"
  mk c17c; a=$(FAKE_MODE=hold gsub g1 1 --cancel-on-fail); b=$(FAKE_MODE=buildfail gsub g1 2 --cancel-on-fail); bash "$DSP" group-seal g1 >/dev/null; gwait g1 300
  chk "c17 (v) with --cancel-on-fail a build_failed member ends it with group kind build_failed" test "$(gkind g1)" = build_failed -a "$(gmembers g1)" = "cancelled_by_group,build_failed"
  mk c17d; a=$(FAKE_MODE=buildfail gsub g1 1); b=$(FAKE_MODE=hold gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; holdready "$b"; waitb "$a" 40; opwait_ "$a" complete; sleep 1
  chk "c17 (v) without --cancel-on-fail a build_failed member cancels nothing (the other member still runs)" test ! -e "$(bd "$b")/cancelled_by_group" -a ! -d "$(bd "$b")/terminal"
  rel "$b"; gwait g1; chk "c17 (v) and the group ends build_failed once every member is terminal" test "$(gkind g1)" = build_failed -a "$(gmembers g1)" = "build_failed,completed" -o "$(gmembers g1)" = "build_failed,succeeded"
fi
if want c18; then mk c18
  a=$(gsub g1 1); b=$(FAKE_MODE=fail gsub g1 2); c=$(gsub g1 3); bash "$DSP" group-seal g1 >/dev/null; gwait g1
  chk "c18 (v) a GREEN x3 with one failing iteration still completes all its iterations (every member terminal, none cancelled)" test "$(for x in $a $b $c; do tkind "$x"; done | tr '\n' ' ')" = "completed completed completed " -a ! -e "$(bd "$a")/cancelled_by_group" -a ! -e "$(bd "$c")/cancelled_by_group"
  mk c18b; a=$(FAKE_MODE=fail gsub g1 1); b=$(FAKE_MODE=buildfail gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; gwait g1
  chk "c18 (v) the group kind is decided by precedence, not arrival: test_failed or build_failed of the FIRST such member in member order" test "$(gkind g1)" = test_failed
  mk c18c; a=$(FAKE_MODE=hold gsub g1 1); b=$(FAKE_MODE=buildfail gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; waitb "$b" 40; opwait_ "$b" complete; rel "$a"; gwait g1
  chk "c18 (v) the same members finishing in the other order give the same kind (member order, not arrival order)" test "$(gkind g1)" = build_failed -o "$(gkind g1)" = test_failed
fi
if want c19; then mk c19
  a=$(gsub g1 1); waitb "$a" 30; opwait_ "$a" complete; sleep 1; gwait g1 10; grc=$?
  chk "c19 (v) a group with no seal is never terminal, however many members finished (the member itself is terminal)" test -n "$a" -a -d "$(bd "$a")/terminal" -a "$grc" != 0 -a ! -d "$(gd g1)/terminal"
  nb=$(ls "$T/builds" | grep -c '^b-')
  out=$(CB=tic-group-record bash "$DSP" submit --purpose "$(pk 2)" --callback tic-group-record --image IMG-GO --src "$T/src" --group g1 -- fake-build x 2>&1); rc=$?
  chk "c19 (v) a second submit of the group naming a different callback is refused (20 group_conflict): the group callback is registered once" test "$rc" = 20 -a "${out#*group_conflict}" != "$out"
  chk "c19 (v) the conflicting submit is refused BEFORE anything starts: no build directory, and not even the disk gate ran for it (its record is absent)" test "$(ls "$T/builds" | grep -c '^b-')" = "$nb" -a -n "$(ls "$T/disk" 2>/dev/null)" -a ! -e "$T/disk/submit-b-$(printf '%s' "$(pk 2)" | sha256sum | cut -c1-24).json"
  bash "$DSP" group-seal g1 >/dev/null; gwait g1
  chk "c19 (v) sealing closes the membership: a later submit is refused (20 group_sealed)" test "$(sub 3 --group g1 >/dev/null 2>&1; echo $?)" = 20
  mk c19b; export DISPATCH_GROUP_BUDGET=1; a=$(FAKE_MODE=hold gsub g1 1); sleep 2.5; bash "$DSP" group-sweep >/dev/null 2>&1; gwait g1 200; unset DISPATCH_GROUP_BUDGET
  chk "c19 (v) a group not sealed within its group budget is reported group_orphaned and made terminal cancelled, its members cancelled_by_group" test "$(gkind g1)" = cancelled -a "$(jq -r .orphaned "$(gd g1)/terminal/members.json")" = true -a -e "$(bd "$a")/cancelled_by_group"
fi

# ================================================================ c23-c33: T089a, every dispatched build is bound to the long-op registry
LOPS() { echo "$T/longops/ops"; }
opj() { jq -r "$2" "$(LOPS)/$1.json" 2>/dev/null; }
opwait() { opwait_ "$@"; }   # test-side bounded wait: the registry op ends just after the callback
pkv() { local it=${1:-1} lane=${2:-lane} var=${3:-primary}; printf 'build:app:%s:%s:%s:%s%s' "$lane" "$(sdig)" "$(adig)" "$var" "${it:+:$it}"; }
subv() { # iteration lane variant extra... : submit with an explicit variant
  local it=$1 lane=$2 var=$3; shift 3
  bash "$DSP" submit --purpose "$(pkv "$it" "$lane" "$var")" --callback record-only --image IMG-GO --src "$T/src" --heartbeat 1 --no-progress-budget "${BUDGET:-3}" --wallclock-cap "${WALL:-60}" "$@" -- fake-build x 2>>"$T/log/sub.err"; }
if want c23; then mk c23
  id=$(FAKE_MODE=hold sub 1); holdready "$id"; read -r pp _ < "$(bd "$id")/pump.pid"
  chk "c23 the build is registered BEFORE the remote start (the fake emitter saw its op in the registry when it started)" test "$(grep -c "$id.json" "$T/log/regseen.$id")" = 1
  chk "c23 the registry op carries the purpose key of T005b, the owner and the pump as its live owner" test "$(opj "$id" .purpose_key)" = "$(pk 1)" -a "$(opj "$id" .owner)" = dispatch -a "$(opj "$id" .pid)" = "$pp"
  opwait_ "$id" running
  chk "c23 the first heartbeat moved it to running and the purpose claim is held" test "$(opj "$id" .state)" = running -a -d "$T/longops/claims/$(pk 1)"
  chk "c23 classify.sh judges the live advancing build advancing" test "$(LONGOPS_DIR="$T/longops" bash "$root/scripts/longops/classify.sh" --op-id "$id" | cut -f2)" = advancing
  : > "$T/log/release.$id"; waitb "$id" 30; opwait "$id" complete
  chk "c23 at the terminal state the op is complete and the claim is released" test "$(opj "$id" .state)" = complete -a ! -d "$T/longops/claims/$(pk 1)"
  id2=$(FAKE_MODE=fail sub 2); waitb "$id2" 30; opwait "$id2" failed
  chk "c23 a build that ends test_failed leaves a failed op (success is read from the verdict, never an exit code)" test "$(opj "$id2" .state)" = failed
fi
if want c24; then mk c24
  a=$(LANE=unit FAKE_MODE=hold sub 1); b=$(LANE=integration FAKE_MODE=hold sub 1); holdready "$a" "$b"
  chk "c24 the unit and integration lanes of one component at one commit start TWO builds and never attach" test "$a" != "$b" -a -s "$T/log/daemon.$a" -a -s "$T/log/daemon.$b"
  : > "$T/log/release.$a"; : > "$T/log/release.$b"; waitb "$a" 30; waitb "$b" 30
  x=$(sub 1); y=$(sub 2); z=$(sub 3); waitb "$x" 30; waitb "$y" 30; waitb "$z" 30; sleep 0.3
  chk "c24 a GREEN x3 submitted together starts three builds with three iteration ids and three registry records" test "$(printf '%s\n' "$x" "$y" "$z" | sort -u | wc -l)" = 3 -a "$(for i in $x $y $z; do opj "$i" .purpose_key | awk -F: '{print $NF}'; done | sort -u | tr '\n' ' ')" = "1 2 3 "
fi
if want c25; then mkgit c25
  d0=$(sdig); printf 'e1\n' >> "$T/src/svc/main.go"; d1=$(sdig); git -C "$T/src" checkout -q svc/main.go; printf 'e2\n' >> "$T/src/svc/main.go"; d2=$(sdig); git -C "$T/src" checkout -q svc/main.go
  chk "c25 two different uncommitted edits on one HEAD give two snapshot digests (hence two purpose keys and two builds)" test "$d1" != "$d2" -a "$d1" != "$d0" -a "$d2" != "$d0"
  printf 'e\n' >> "$T/src/submodules/lib/f.txt"; chk "c25 an edit inside a submodule changes the digest" test "$(sdig)" != "$d0"; git -C "$T/src/submodules/lib" checkout -q f.txt
  printf 'FROM scratch\nCOPY web/index.js /w\n' > "$T/src/Containerfile"; git -C "$T/src" add Containerfile; git -C "$T/src" commit -qm cf; c0=$(sdig); printf 'x\n' >> "$T/src/web/index.js"
  chk "c25 an edit to a file that a Containerfile COPY reads changes the digest of that image build" test "$(sdig)" != "$c0"
  git -C "$T/src" checkout -q web/index.js; printf 'new\n' > "$T/src/web/undeclared_new.js"
  chk "c25 an untracked new source file left undeclared changes the digest of a TIC submit (I8)" test "$(sdig)" != "$c0"
  id=$(sub 1 --component web); waitb "$id" 30; id2=$(sub 1 --component web --snapshot-mode tic); chk "c25 an identical resubmit of the same key attaches or reuses (build once), never a second build" test "$id" = "$id2"
fi
if want c26; then mk c26
  a=$(FAKE_MODE=hold sub 1); holdready "$a"; a2=$(FAKE_MODE=hold sub 1)
  chk "c26 a submit whose FULL purpose key equals a running build's attaches to it (same id, one daemon)" test "$a" = "$a2" -a "$(wc -l < "$T/log/daemon.$a")" = 1
  b=$(FAKE_MODE=hold sub 2); chk "c26 a submit carrying another iteration id never attaches to that iteration" test "$b" != "$a"
  c=$(FAKE_MODE=hold subv 1 lane repro-cold); holdready "$c"
  chk "c26 a repro-cold submit never attaches to the running primary build, whatever the other fields" test "$c" != "$a" -a -s "$T/log/daemon.$c"
  for i in $a $b $c; do : > "$T/log/release.$i"; waitb "$i" 30; done
fi
if want c27; then mk c27
  id=$(FAKE_MODE=flat BUDGET=3 sub 1); waitb "$id" 40; opwait "$id" reaped
  chk "c27 (c) heartbeats with a flat progress pair past the budget: HUNG by the registry (heartbeat.sh feeds it, classify.sh decides): blocked-unavailable build_progress_flat" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = build_progress_flat
  chk "c27 the registry op ends reaped (never complete), verdict names the reason, and heartbeats reached the registry" test "$(opj "$id" .state)" = reaped -a "$(opj "$id" .verdict)" = build_progress_flat -a "$(opj "$id" .heartbeat_seq)" -ge 2
  chk "c27 the remote container is cancelled (by label, through the emitter) and the callback ran once" test "$(wc -l < "$T/log/cancel.$id")" = 1 -a "$(effects "$id")" = 1
  id=$(FAKE_MODE=silent BUDGET=2 sub 2); waitb "$id" 40; opwait "$id" reaped
  chk "c27 a build that goes silent is HUNG too (build_liveness_lost), the op reaped" test "$(treason "$id")" = build_liveness_lost -a "$(opj "$id" .state)" = reaped
fi
if want c28; then mk c28
  printf '# class\tno_progress_s\twall_clock_s\tbasis\nbuild:app:lane\t30\t4\tmeasured in the test\nbuild:*:*\tUNKNOWN\tUNKNOWN\tUNKNOWN until T115\n' > "$T/purposes.tsv"; export DISPATCH_PURPOSES_TSV="$T/purposes.tsv"
  id=$(FAKE_MODE=slow bash "$DSP" submit --purpose "$(pk 2)" --callback record-only --image IMG-GO --src "$T/src" --heartbeat 1 -- fake-build x); waitb "$id" 40
  chk "c28 a build that ADVANCES but passes its purpose's wall-clock cap (scripts/longops/purposes.tsv) is HUNG: build_wallclock_exceeded, measured on the build host's own monotonic clock" test "$(treason "$id")" = build_wallclock_exceeded -a "$(opj "$id" .budget.wall_clock_s)" = 4 -a "$(opwait "$id" reaped; opj "$id" .state)" = reaped
  chk "c28 the submit record names where its budgets came from" test "$(jq -r .budget_source "$(bd "$id")/submit.json")" = "purposes.tsv:build:app:lane"
  id=$(FAKE_MODE=ok bash "$DSP" submit --purpose "$(pkv 3 other)" --callback record-only --image IMG-GO --src "$T/src" --heartbeat 1 -- fake-build x); waitb "$id" 30
  chk "c28 a class whose cap is UNKNOWN falls back to the dispatcher's defaults (60 s no-progress, 3600 s cap), recorded" test "$(jq -r .budget_source "$(bd "$id")/submit.json")" = "default:UNKNOWN" -a "$(opj "$id" .budget.wall_clock_s)" = 3600
  unset DISPATCH_PURPOSES_TSV
fi
if want c29; then mk c29
  id=$(FAKE_MODE=hold sub 1); holdready "$id"; read -r pp ps < "$(bd "$id")/pump.pid"; kill -TERM "$pp"; opwait "$id" handoff
  chk "c29 a driver stop (TERM on the pump) hands the build to the registry: op handoff, claim released, the remote build left running (never orphaned silently)" test "$(opj "$id" .state)" = handoff -a ! -d "$T/longops/claims/$(pk 1)" -a ! -s "$T/log/cancel.$id"
  bash "$DSP" resume "$id" >/dev/null; opwait "$id-a2" running
  chk "c29 a restarted driver re-adopts it through resume: a new op, running, owned by the new pump; nothing resubmitted" test "$(opj "$id-a2" .state)" = running -a "$(wc -l < "$T/log/daemon.$id")" = 1
  : > "$T/log/release.$id"; waitb "$id" 30; opwait "$id-a2" complete
  chk "c29 the re-adopted op ends complete once" test "$(opj "$id-a2" .state)" = complete -a "$(opj "$id" .state)" = handoff -a "$(effects "$id")" = 1
fi
if want c30; then mk c30
  id=$(FAKE_MODE=hold sub 1); holdready "$id"; read -r pp ps < "$(bd "$id")/pump.pid"; kill -KILL "$pp"; sleep 0.4
  chk "c30 a pump killed outright leaves its op running with a DEAD owner: classify.sh proves it from /proc (dead_owner)" test "$(LONGOPS_DIR="$T/longops" bash "$root/scripts/longops/classify.sh" --op-id "$id" | cut -f2)" = dead_owner
  bash "$DSP" resume "$id" >/dev/null; opwait "$id-a2" running
  chk "c30 resume re-adopts: the dead-owner op is reaped (no signal sent) and a new op owns the build" test "$(opj "$id" .state)" = reaped -a "$(opj "$id-a2" .state)" = running -a ! -s "$T/longops/signals.log"
  : > "$T/log/release.$id"; waitb "$id" 30
fi
if want c31; then mk c31
  LONGOPS_DIR="$T/longops" LONGOPS_ALLOW_TMPFS=1 bash "$root/scripts/longops/register.sh" --purpose "$(pk 1)" --owner other-tool --pid $$ >/dev/null 2>&1
  out=$(bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "c31 a purpose held by another long-op is refused at submit: 20 purpose_conflict, nothing started, no build directory" test "$rc" = 20 -a "${out#*purpose_conflict}" != "$out" -a -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')"
fi
if want c32; then mk c32
  p1=$(FAKE_MODE=ok sub 1); r1=$(FAKE_MODE=ok subv 2 lane repro-cold); waitb "$p1" 30; waitb "$r1" 30; o=$(LANE=other sub 9); waitb "$o" 30
  chk "c32 the fixture ids are real, distinct build ids (a refused submit would make every pair check below vacuous)" test -n "$p1" -a -n "$r1" -a -n "$o" -a "$p1" != "$r1" -a "$p1" != "$o"
  chk "c32 the pairing of a primary build with its repro-cold partner matches every field but variant and ignores the iteration" test "$(bash "$DSP" pair "$p1")" = "$r1" -a "$(bash "$DSP" pair "$r1")" = "$p1"
  chk "c32 a build of another lane has no partner" test -z "$(bash "$DSP" pair "$o")"
  p2=$(FAKE_MODE=fail sub 3); waitb "$p2" 30
  chk "c32 the failed iteration is a real build id" test -n "$p2" -a "$p2" != "$p1"
  chk "c32 a repro-cold build pairs with the primary record whose artifact is the deliverable (the succeeded one, not the failed iteration)" test "$(bash "$DSP" pair "$r1")" = "$p1"
fi

# ================================================================ c34-c38: the REAL emitter (remote/emit.sh) with a stub of run_pinned.sh and a podman/cgroup shim
mkreal() { # name : like mk, but the emitter is the real remote/emit.sh; the build is a stub that holds until $T/log/release and writes an artifact
  mk "$1"; export DISPATCH_EMIT_DIR="$B" DISPATCH_REMOTE_RUNP="$T/runp.sh" DISPATCH_RECONNECTS=3 DISPATCH_RECONNECT_DELAY=0.2 STUB_LOG="$T/log"
  mkdir -p "$T/bin" "$T/cg/fake.scope"; printf 'usage_usec 1000\n' > "$T/cg/fake.scope/cpu.stat"; printf '%s\n' "${STUB_PEAK:-7777777}" > "$T/cg/fake.scope/memory.peak"
  cat > "$T/runp.sh" <<'RPEOF'
#!/usr/bin/env bash
out=""; opid=""
while [ $# -gt 0 ]; do case $1 in --out) out=$2; shift 2;; --op-id) opid=$2; shift 2;; --need) shift 2;; --network=none) shift;; --) shift; break;; *) shift;; esac; done
echo "$opid" >> "$STUB_LOG/runp.starts"; mkdir -p "$out/artifacts"
n=1000; while [ ! -e "$STUB_LOG/release" ]; do
  [ -d "$STUB_LOG" ] || exit 9          # the sandbox is gone: never outlive it
  [ -z "${STUB_CHATTER:-}" ] || echo tick
  if [ -n "${STUB_CPU:-}" ]; then n=$((n+5000)); printf 'usage_usec %s\n' "$n" > "$STUB_CG/fake.scope/cpu.stat"; fi
  sleep 0.3; done
printf 'built\n' > "$out/artifacts/a.txt"; exit "${STUB_EXIT:-0}"
RPEOF
  printf '#!/usr/bin/env bash\nREAL_PODMAN=%q\n' "$(command -v podman)" > "$T/bin/podman"
  cat >> "$T/bin/podman" <<'PMEOF'
case "$1" in
  ps) [ -n "${STUB_NO_CONTAINER:-}" ] || echo fakecid; exit 0;;
  inspect) echo /fake.scope; exit 0;;
  kill) echo "$*" >> "$STUB_LOG/podman.kill"; exit 0;;
esac
exec "$REAL_PODMAN" "$@"
PMEOF
  chmod +x "$T/runp.sh" "$T/bin/podman"; export PATH="$T/bin:$PATH" STUB_CG="$T/cg" EMIT_CGROUP_ROOT="$T/cg"; }
unmkreal() { export PATH=$(printf '%s' "$PATH" | sed "s#$T/bin:##"); }
jrn() { echo "$T/builds/$1/remote/journal.jsonl"; }
if want c34; then mkreal c34
  id=$(sub 1); for _ in $(seq 1 100); do [ "$(jq -s '[.[] | select(.kind=="heartbeat")] | length' "$(jrn "$id")" 2>/dev/null)" -ge 2 ] 2>/dev/null && break; sleep 0.2; done
  chk "c34 (i2) the real emitter journal holds NO signature at rest (signed only when streamed, with the key the driver sends)" test "$(jq -s '[.[] | has("hmac")] | any' "$(jrn "$id")")" = false
  read -r dp ds < "$T/builds/$id/remote/daemon.pid" || { bad "c34 no daemon.pid (submit: $(cat "$T/log/sub.err" 2>/dev/null))"; dp=0; ds=x; }; [ "$dp" -gt 1 ] && kill -KILL "$dp"; sleep 0.5
  chk "c34 (i2) the emitter daemon is dead while the build container keeps running (the stub build has not ended)" test "$(awk '{print $22}' /proc/$dp/stat 2>/dev/null)" != "$ds" -a "$(wc -l < "$T/log/runp.starts")" = 1
  : > "$T/log/release"; waitb "$id" 40
  chk "c34 (i2) the restarted emitter re-signs from its journal with the key re-obtained from the driver and finishes the build: completed once, callback once" test "$(tkind "$id")" = completed -a "$(tclass "$id")" = succeeded -a "$(effects "$id")" = 1
  chk "c34 (i2) nothing was resubmitted: the build started once, one accepted event in the journal" test "$(wc -l < "$T/log/runp.starts")" = 1 -a "$(jq -s '[.[] | select(.kind=="accepted")] | length' "$(jrn "$id")")" = 1
  sec=$(cat "$T/state/build_hmac.key"); run=$(jq -r .run_id "$(bd "$id")/submit.json"); key=$(bash "$CORE" derive-key "$T/state" "$id" "$run")
  chk "c34 (i2) the key is in no file on the build host (journal, pid files, log)" test -z "$(grep -rlF -e "$key" -e "$sec" "$T/builds/$id/remote" 2>/dev/null)"
  printf '%s\n' "$key" > "$T/builds/$id/remote/planted-needle"
  chk "c34 (i2) control needle: the same scan finds a planted copy of the key" test -n "$(grep -rlF -e "$key" -e "$sec" "$T/builds/$id/remote" 2>/dev/null)"; rm -f "$T/builds/$id/remote/planted-needle"
  unmkreal
fi
if want c35; then mkreal c35
  : > "$T/log/release"; id=$(sub 1); waitb "$id" 40
  rd="$T/builds/$id/remote"
  out=$(bash "$B/remote/emit.sh" attach --dir "$rd" --from 0 < /dev/null 2>&1); rc=$?
  chk "c35 (i2) an emitter that cannot obtain the key refuses to send: exit 3, nothing on stdout" test "$rc" = 3 -a -z "$(printf '%s' "$out" | grep build-event)"
  out=$(printf 'not-a-key\n' | bash "$B/remote/emit.sh" attach --dir "$rd" --from 0 2>&1); rc=$?
  chk "c35 (i2) a malformed key is refused the same way" test "$rc" = 3 -a -z "$(printf '%s' "$out" | grep build-event)"
  run=$(jq -r .run_id "$(bd "$id")/submit.json"); key=$(bash "$CORE" derive-key "$T/state" "$id" "$run")
  n=$(printf '%s\n' "$key" | bash "$B/remote/emit.sh" attach --dir "$rd" --from 0 | python3 -c 'import sys,json;print(sum(1 for l in sys.stdin if json.loads(l).get("hmac")))')
  chk "c35 (i2) with the key the journal is streamed re-signed: every line carries an HMAC" test "$n" = "$(wc -l < "$rd/journal.jsonl")"
  unmkreal
fi
if want c36; then export STUB_PEAK=424242424; mkreal c36
  id=$(sub 1); holdready "$id"; : > "$T/log/release"; waitb "$id" 40
  pk=$(jq -r 'select(.kind=="completed") | .peak_rss_bytes' "$(bd "$id")/events.jsonl" | head -1)
  chk "c36 (T115) the completed event carries peak_rss_bytes, the build container's cgroup memory.peak read on the build host, and the core accepts it" test "$pk" = 424242424 -a "$(tkind "$id")" = completed
  unset STUB_PEAK; mkreal c36b; export STUB_NO_CONTAINER=1 STUB_CHATTER=1; id=$(sub 1); holdready "$id"; : > "$T/log/release"; waitb "$id" 40
  chk "c36 (T115) with no container cgroup readable the field is OMITTED (never a guess), the event still verifies and the build completes" test "$(jq -r 'select(.kind=="completed") | has("peak_rss_bytes")' "$(bd "$id")/events.jsonl" | head -1)" = false -a "$(tkind "$id")" = completed
  unset STUB_NO_CONTAINER STUB_CHATTER; unmkreal
fi
if want c37; then mkreal c37; export STUB_CPU=1
  id=$(BUDGET=3 sub 1 --progress log+cpu); sleep 8; : > "$T/log/release"; waitb "$id" 40
  chk "c37 a QUIET build (no log output at all) that burns CPU is not HUNG: progress is the log plus the container's CPU counter" test "$(tkind "$id")" = completed -a "$(tclass "$id")" = succeeded
  rm -f "$T/log/release" "$T/log/runp.starts"; mkreal c37b; export STUB_CPU=1
  id=$(BUDGET=3 sub 1 --progress log); waitb "$id" 40
  chk "c37 control: the same quiet build with --progress log ends build_progress_flat" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = build_progress_flat
  unset STUB_CPU; unmkreal
fi

# ================================================================ c38: ssh host-key pinning (pinned fingerprint from build/hosts.env, never trust on first use); c39: keyring refill
mkpin() { # name [hostfp] : an ssh shim that records its arguments, a fake ssh-keyscan serving a generated ed25519 key, hosts.env with the pin
  mk "$1"; ssh-keygen -q -t ed25519 -N '' -f "$T/hk" >/dev/null 2>&1; local kfp; kfp=$(ssh-keygen -lf "$T/hk.pub" | awk '{print $2}')
  printf '#!/usr/bin/env bash\nname=${@: -1}\nprintf "%%s %%s\\n" "$name" "$(cut -d" " -f1,2 %q)"\n' "$T/hk.pub" > "$T/keyscan"; chmod +x "$T/keyscan"
  cat > "$T/ssh" <<SSHEOF
#!/usr/bin/env bash
printf '%s\n' "\$*" >> "$T/log/ssh.args"
while [ "\${1:-}" = -o ]; do shift 2; done
host=\$1; shift; cmd="\$*"
case "\$cmd" in "true") exit 0;; podman\ info*) echo true; exit 0;; esac
cd "$T/fakehome" && HOME="$T/fakehome" exec bash -c "\$cmd"
SSHEOF
  chmod +x "$T/ssh"
  printf 'BUILD_HOST_1=u@pinhost\nBUILD_HOST_1_FP=%s\n' "${2-$kfp}" > "$T/hosts.env"
  export DISPATCH_TRANSPORT=ssh DISPATCH_SSH="$T/ssh" DISPATCH_PIN=1 DISPATCH_KEYSCAN="$T/keyscan" DISPATCH_REMOTE_RUNP=/bin/true; }
if want c38; then mkpin c38; id=$(sub 1); waitb "$id" 40
  chk "c38 a host whose key matches the pinned fingerprint is qualified and the build runs" test "$(jq -r .host "$(bd "$id")/submit.json")" = u@pinhost -a "$(tkind "$id")" = completed
  chk "c38 ssh is invoked with StrictHostKeyChecking=yes and a known-hosts file holding exactly the pinned key (generated by the dispatcher)" test -n "$(grep -- 'StrictHostKeyChecking=yes' "$T/log/ssh.args" | grep 'UserKnownHostsFile=')" -a "$(cat "$T/state/known_hosts/"* | grep -c 'ssh-ed25519')" = 1
  chk "c38 never trust on first use: no ssh call ever carries StrictHostKeyChecking=no or accept-new" test "$(grep -c -E 'StrictHostKeyChecking=(no|accept-new|off)' "$T/log/ssh.args")" = 0
  mkpin c38b 'SHA256:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA'; id=$(sub 1); waitb "$id" 20
  chk "c38 a host whose key does NOT match the pin is never connected to: blocked-unavailable no_qualified_host, attempt detail host_key_mismatch" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = no_qualified_host -a "$(jq -r '.host_attempts[0].detail' "$(bd "$id")/submit.json")" = host_key_mismatch -a ! -s "$T/log/ssh.args"
  mkpin c38c ''; id=$(sub 1); waitb "$id" 20
  chk "c38 a host with no pinned fingerprint is not qualified (host_key_not_pinned): the dispatcher never learns a key by itself" test "$(tkind "$id")" = blocked-unavailable -a "$(jq -r '.host_attempts[0].detail' "$(bd "$id")/submit.json")" = host_key_not_pinned -a ! -s "$T/log/ssh.args"
  unset DISPATCH_PIN DISPATCH_KEYSCAN DISPATCH_SSH DISPATCH_REMOTE_RUNP; export DISPATCH_TRANSPORT=local
fi
if want c39; then mk c39; KR="$B/lib/keyring.sh"
  chk "c39 (q2) keyring.sh exists" test -f "$KR"
  bash "$CORE" secret-init "$T/state" "$root" >/dev/null 2>&1; CID=$(printf '%s' "$root" | sha256sum | cut -d' ' -f1)
  out=$(keyctl session - bash -c "bash '$KR' refill '$T/state' '$root' && bash '$KR' status '$root'" 2>&1); rc=$?
  chk "c39 (q2) refill loads the keyring entry from the secret file; status reads present" test "$rc" = 0 -a "${out#*keyring refilled}" != "$out" -a "${out%present}" != "$out"
  out=$(keyctl session - bash -c "bash '$KR' status '$root'; echo rc=\$?; bash '$KR' refill '$T/state' '$root'; bash '$KR' status '$root'" 2>&1)
  chk "c39 (q2) after a reboot or logout (a NEW, empty session keyring) the entry is absent and the refill from the file restores it" test "${out#*absent}" != "$out" -a "${out#*rc=1}" != "$out" -a "${out#*keyring refilled}" != "$out" -a "${out%present}" != "$out"
  chk "c39 (q2) the refilled entry holds exactly the file's secret" test "$(keyctl session - bash -c "bash '$KR' refill '$T/state' '$root' >/dev/null; keyctl pipe \$(keyctl search @s user catalogizer:build_hmac:$CID) 2>/dev/null" 2>/dev/null | tail -1)" = "$(cat "$T/state/build_hmac.key")"
  mv "$T/state/build_hmac.key" "$T/state/build_hmac.key.gone"
  out=$(keyctl session - bash -c "bash '$KR' refill '$T/state' '$root'; echo rc=\$?; bash '$KR' status '$root'" 2>&1)
  chk "c39 (q2) with the file lost the refill REFUSES (20 driver_secret_lost) and creates nothing: the file, not the keyring, is the truth" test "${out#*driver_secret_lost}" != "$out" -a "${out#*rc=20}" != "$out" -a "${out%absent}" != "$out"
  mv "$T/state/build_hmac.key.gone" "$T/state/build_hmac.key"
  export DISPATCH_HUB=1 HUB_DISPATCH="$DSP" HUB_RUNCB="$RUNCB" DISPATCH_HUB_SCRIPT="$HUB" LONGOPS_ALLOW_TMPFS=1 DISPATCH_BUILDS_ROOT="$T/builds"
  out=$(keyctl session - bash -c "bash '$HUB' start >/dev/null; for i in 1 2 3 4 5 6 7 8 9 10 11 12 13 14 15 16 17 18 19 20; do bash '$KR' status '$root' >/dev/null && break; sleep 0.2; done; bash '$KR' status '$root'" 2>&1)
  chk "c39 (q2) the hub refills the keyring itself when it starts" test "${out%present}" != "$out"
  hubstop; export DISPATCH_HUB=0
fi

if want c20; then mk c20
  a=$(FAKE_MODE=fail gsub g1 1); b=$(FAKE_MODE=silent BUDGET=3 gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; gwait g1 400
  chk "c20 (v) the group kind is decided by precedence across classes: a test_failed member beats a member that ended blocked-unavailable, whichever ended last" test "$(gkind g1)" = test_failed -a "$(gmembers g1)" = "test_failed,blocked-unavailable"
  mk c20b; a=$(FAKE_MODE=silent BUDGET=3 gsub g1 1); b=$(FAKE_MODE=fail gsub g1 2); bash "$DSP" group-seal g1 >/dev/null; gwait g1 400
  chk "c20 (v) the same two members in the other member order give the same kind" test "$(gkind g1)" = test_failed
fi
if want c40; then mkcb c40
  # the hub reads the callbacks table ONCE, at its start (its own export): a row added to the store table later is honoured only after the next hub start
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\n' > "$T/callbacks.tsv"; export DISPATCH_CALLBACKS_TSV="$T/callbacks.tsv"; hubup
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\nscript-cb\tcb_script.sh\tcb-script\tbuild-callback\t20\n' > "$T/callbacks.tsv"
  id=$(CB=script-cb sub 1); for _ in $(seq 1 120); do [ -d "$(bd "$id")/terminal" ] && [ -n "$(sstate "$id")" ] && break; sleep 0.2; done
  chk "c40 a callback row approved AFTER the hub started: the build ends, its script callback is refused at callback time (failed callback_not_registered) and never ran" test "$(sstate "$id")" = failed -a "$(cat "$(bd "$id")/terminal/script.reason" 2>/dev/null)" = callback_not_registered -a ! -s "$T/log/cb.runs"
  hubstop; hubup; id=$(CB=script-cb sub 2); for _ in $(seq 1 120); do [ "$(sstate "$id")" = done ] && break; sleep 0.2; done
  chk "c40 after a hub restart the same id runs (the new export lists it)" test "$(sstate "$id")" = done -a "$(wc -l < "$T/log/cb.runs")" = 1
  hubstop
fi

if want c41; then mk c41; hubup
  id=$(FAKE_MODE=hold sub 1); holdready "$id"; read -r p s < "$(bd "$id")/pump.pid"
  # (i) the driver (the pump) dies, and the remote build ends around the restart: the completed event may sit unconsumed in the build host's journal; the hub restarts the pump
  kill -KILL "$p"; sleep 0.2; : > "$T/log/release.$id"
  waitb "$id" 40
  chk "c41 (i) the hub restarts the pump, which re-attaches from the last acknowledged seq, verifies the resent events and consumes the completion ONCE; the build is not resubmitted" test "$(tkind "$id")" = completed -a "$(effects "$id")" = 1 -a "$(wc -l < "$T/log/daemon.$id")" = 1
  chk "c41 (i) every consumed seq is unique and the journal holds one completed event" test "$(ls "$(bd "$id")/consumed" | sort -n | uniq -d | wc -l)" = 0 -a "$(jq -s '[.[] | select(.kind=="completed")] | length' "$T/builds/$id/remote/journal.jsonl")" = 1
  hubstop
fi
echo "RESULT pass=$pass fail=$fail skip=$skipn"
[ "$fail" = 0 ]
