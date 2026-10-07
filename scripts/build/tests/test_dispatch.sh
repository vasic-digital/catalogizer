#!/usr/bin/env bash
# test_dispatch.sh - T005a, dispatcher slice (host control-plane; P0-P1 host exception, as T003): tests scripts/build/dispatch.sh, remote/emit.sh and
# lib/evwait.py against a scripted fake emitter (the real emitter's functions with its daemon replaced) and an `ssh` shim.
# Cases (T005a letters): (a) (c) (d) (f) (h) (j-ish) (k) (l) (m) (n0-n2 via core) (q2/q3) (r0) plus the submit refusals and the secret scan (t).
# The consume core cases stay in test_dispatch_events.sh (unchanged). OWED cases: see OWED below.
# Usage: test_dispatch.sh            all cases;   ONLY="s6 s9" test_dispatch.sh   a subset (the paired mutations use it).
# DISPATCH_SCRIPT / EMIT_SCRIPT / EVWAIT_SCRIPT override the files under test (mutation runs).
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
root=$(cd "$here/../../.." && pwd)
DSP=${DISPATCH_SCRIPT:-$here/../dispatch.sh}
EMIT=${EMIT_SCRIPT:-$here/../remote/emit.sh}
EVW=${EVWAIT_SCRIPT:-$here/../lib/evwait.py}
CORE="$here/../event_core.sh"
[ -f "$DSP" ] || { echo "FAIL script absent: $DSP"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }
for t in jq python3 flock openssl tar sha256sum; do command -v "$t" >/dev/null 2>&1 || { echo "FAIL dependency missing: $t"; echo "RESULT pass=0 fail=1 skip=0"; exit 1; }; done
pass=0; fail=0; skipn=0
say() { printf '%s\n' "$1"; }
ok()  { pass=$((pass+1)); say "ok   $1"; }
bad() { fail=$((fail+1)); say "FAIL $1${2:+ -- $2}"; }
chk() { local what=$1; shift; if "$@"; then ok "$what"; else bad "$what"; fi; }
want() { [ -z "${ONLY:-}" ] && return 0; case " $ONLY " in *" $1 "*) return 0;; esac; return 1; }
OWED="(i) driver crash between completed and consumption with a real hub, (i2) emitter re-signing after restart, (j) kill before submit, (n0-n2/n/o/p) beyond the core's cases, (q2) keyring refill, (r) second hub refused, (u/u2) git snapshot and closure, (v) groups, (w) transfer budget, (x) hub identity: event_hub.sh and run_callback.sh are OWED"
REAL_EMIT="$root/scripts/build/remote/emit.sh"
tmproot=$(mktemp -d "${TMPDIR:-/tmp}/tdsp.XXXXXX"); trap 'cleanup' EXIT
cleanup() { kill_ours; if [ -n "${KEEP:-}" ]; then echo "kept $tmproot"; else rm -rf "$tmproot"; fi; }
kill_ours() { # stop pumps and fake daemons of this run by exact pid, proven ours by start time and cmdline (never a pattern kill)
  local f p s; for f in "$tmproot"/*/builds/b-*/pump.pid "$tmproot"/*/builds/b-*/remote/daemon.pid "$tmproot"/*/fakehome/.cache/catalogizer/builds/*/daemon.pid; do
    [ -s "$f" ] || continue; read -r p s < "$f"; [ "${p:-0}" -gt 1 ] 2>/dev/null || continue
    [ "$(awk '{print $22}' /proc/$p/stat 2>/dev/null)" = "$s" ] && kill -TERM "$p" 2>/dev/null
  done; }

# ---- sandbox: builds root, state dir, source tree, fake emitter dir, callbacks table, ssh shim ----
mk() { # name : sets T, env; prepares everything
  T="$tmproot/$1"; mkdir -p "$T/state" "$T/src" "$T/fake/remote" "$T/fake/lib" "$T/fakehome" "$T/log"
  printf 'payload-%s\n' "$1" > "$T/src/file.txt"
  ln -sf "$EVW" "$T/fake/lib/evwait.py"
  printf '#!/usr/bin/env bash\n. %q\n%s\nmain "$@"\n' "$EMIT" "$(fake_daemon)" > "$T/fake/remote/emit.sh"; chmod +x "$T/fake/remote/emit.sh"
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\n' > "$T/callbacks.tsv"
  export DISPATCH_BUILDS_ROOT="$T/builds" DISPATCH_STATE_DIR="$T/state" DISPATCH_CALLBACKS_TSV="$T/callbacks.tsv" DISPATCH_TRANSPORT=local DISPATCH_ALLOW_LOCAL=1 \
         DISPATCH_EMIT_DIR="$T/fake" DISPATCH_DISK_OUT="$T/disk" DISPATCH_HOSTS_FILE="$T/hosts.env" DISPATCH_RECONNECT_DELAY=0 DISPATCH_RECONNECTS=1 DISPATCH_JOBS=4 \
         FAKE_LOG="$T/log" FAKE_MODE=ok DISPATCH_EVWAIT="$EVW" EVWAIT="$EVW"
  unset DISPATCH_SSH
}
fake_daemon() { cat <<'EOF'
[ -z "${FAKE_SKEW:-}" ] || isoz() { echo 2001-01-01T00:00:00Z; }
hbev() { emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" heartbeat "$(jq -nc --argjson p "$1" --argjson s "$2" --argjson e "$3" '{progress_offset:$p,stage:$s,elapsed_monotonic_ms:$e}')"; }
done_ev() { local m; m=$(manifest_sha "$DIR/out/artifacts"); [ -z "${1:-}" ] || m=$1
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" completed "$(jq -nc --arg m "$m" --arg l "$(printf '%064d' 0)" '{exit_class:"succeeded",artifact_manifest_sha256:$m,image_digest:("sha256:"+$m),remote_log_sha256:$l}')"; }
daemon() {
  read -r KEY; mkdir -p "$DIR/out/artifacts"; printf 'started\n' >> "$FAKE_LOG/daemon.$BID"
  printf '%s %s\n' "$$" "$(pid_start $$)" > "$DIR/daemon.pid"
  trap 'printf "cancelled\n" >> "$FAKE_LOG/cancel.$BID"; kill $(jobs -p) 2>/dev/null; exit 0' TERM
  printf 'artifact\n' > "$DIR/out/artifacts/a.txt"
  printf '%s start\n' "$BID" >> "$FAKE_LOG/order.log"
  emit_event "$DIR" "$KEY" "$BID" "$RUN" "$VARIANT" "$HOST" accepted
  case "${FAKE_MODE:-ok}" in
    ok)       hbev 100 1 500; hbev 200 2 1000; done_ev;;
    silent)   sleep 600 & wait $!;;
    flat)     local e=0; while :; do e=$((e+1000)); hbev 5 1 $e; sleep 1 & wait $!; done;;
    slow)     local e=0 p=0; while :; do e=$((e+1000)); p=$((p+10)); hbev $p $p $e; sleep 1 & wait $!; done;;
    late)     hbev 100 1 500; sleep 600 & wait $!;;
    hold)     local t0=$(mono_ms) p=100; hbev 100 1 500   # advancing heartbeats until released (a silent hold would be HUNG, correctly)
              while [ ! -e "$FAKE_LOG/release.$BID" ]; do p=$((p+1)); hbev $p $p $(( $(mono_ms) - t0 + 500 )); sleep 0.5 & wait $!; done
              printf '%s end\n' "$BID" >> "$FAKE_LOG/order.log"; done_ev;;
    mismatch) hbev 100 1 500; done_ev "$(printf '%064d' 7)";;
    forgedc)  hbev 100 1 500
              # a forged completed event (wrong key, an artifact digest that would not match) takes journal line 3; the genuine events follow
              local fl; fl=$(jq -nc --arg b "$BID" --arg r "$RUN" --arg v "$VARIANT" --arg h "$HOST" --arg m "$(printf '%064d' 9)" --arg l "$(printf '%064d' 0)" '{schema:"build-event/1",run_id:$r,build_id:$b,variant:$v,seq:3,kind:"completed",host:$h,sent_at:"2026-10-06T00:00:00Z",exit_class:"succeeded",artifact_manifest_sha256:$m,image_digest:("sha256:"+$m),remote_log_sha256:$l}' | sign_event "$(printf '%064d' 1)")
              printf '%s\n' "$fl" >> "$DIR/journal.jsonl"; printf '3\n' > "$DIR/seq"; hbev 200 2 1000; done_ev;;
    badsig)   hbev 100 1 500
              # a forged heartbeat (correct shape, signed with a WRONG key) takes journal line 3, then the genuine events follow
              local line; line=$(jq -nc --arg b "$BID" --arg r "$RUN" --arg v "$VARIANT" --arg h "$HOST" '{schema:"build-event/1",run_id:$r,build_id:$b,variant:$v,seq:3,kind:"heartbeat",host:$h,sent_at:"2026-10-06T00:00:00Z",progress_offset:150,stage:1,elapsed_monotonic_ms:700}' | sign_event "$(printf '%064d' 1)")
              printf '%s\n' "$line" >> "$DIR/journal.jsonl"; printf '3\n' > "$DIR/seq"; hbev 200 2 1000; done_ev;;
  esac
  exit 0
}
EOF
}
sdig() { bash "$DSP" snapshot "$T/src"; }
pk() { local it=${1:-1}; printf 'build:app:lane:%s:%s:primary:%s' "$(sdig)" "$(bash "$DSP" argv-digest fake-build x)" "$it"; }
sub() { # iteration extra-args... : submit, print the build id
  local it=$1; shift
  bash "$DSP" submit --purpose "$(pk "$it")" --callback record-only --image IMG-GO --src "$T/src" --heartbeat 1 --no-progress-budget "${BUDGET:-3}" --wallclock-cap "${WALL:-60}" "$@" -- fake-build x 2>>"$T/log/sub.err"; }
bd() { echo "$T/builds/$1"; }
tkind() { jq -r .kind "$(bd "$1")/terminal/state.json" 2>/dev/null; }
treason() { jq -r .digest "$(bd "$1")/terminal/state.json" 2>/dev/null; }
effects() { ls "$(bd "$1")/effects" 2>/dev/null | wc -l; }
waitb() { bash "$DSP" wait "$1" "${2:-40}" >/dev/null 2>&1; }
pump_gone() { local i; for i in $(seq 1 50); do [ "$(bash "$DSP" status "$1" | jq -r .pump_alive)" = false ] && return 0; sleep 0.1; done; return 1; }   # test-side bounded wait
jlines() { wc -l < "$(bd "$1")/events.jsonl" 2>/dev/null || echo 0; }

# ================================================================ s1-s5: submit refusals (no build directory may be left behind)
if want s1; then mk s1
  out=$(bash "$DSP" submit --purpose "build:app:lane:abc:def:primary" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s1 malformed purpose key refused (20 purpose_malformed)" test "$rc" = 20 -a "${out#*reason=}" != "$out"
  case $out in *purpose_malformed*) ok "s1 reason is purpose_malformed";; *) bad "s1 reason is purpose_malformed" "$out";; esac
  out=$(bash "$DSP" submit --purpose "build:app:lane:$(sdig | cut -c1-63):$(bash "$DSP" argv-digest fake-build x):primary" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s1 a 63-hex digest is refused as malformed" test "$rc" = 20 -a "${out#*purpose_malformed}" != "$out"
  chk "s1 no build directory left" test ! -d "$T/builds" -o -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')"
fi
if want s2; then mk s2
  printf 'other-id\t-\tcb-x\tbuild-callback\t60\n' > "$T/callbacks.tsv"
  out=$(bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s2 a callback id outside the registry is refused (20)" test "$rc" = 20
  case $out in *callback_not_registered*) ok "s2 reason callback_not_registered";; *) bad "s2 reason callback_not_registered" "$out";; esac
  chk "s2 nothing submitted, no build directory left" test -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')"
  printf 'record-only\t-\tcb-record-only\tbuild-callback\t60\n' > "$T/callbacks.tsv"
  id=$(sub 1); chk "s2 an id in the table is accepted" test -n "$id"; waitb "$id"
fi
if want s3; then mk s3
  out=$(bash "$DSP" submit --purpose "build:app:lane:$(printf '%064d' 1):$(bash "$DSP" argv-digest fake-build x):primary" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s3 a purpose key whose snapshot digest does not match the tree is refused" test "$rc" = 20
  case $out in *purpose_digest_mismatch*) ok "s3 reason purpose_digest_mismatch (snapshot)";; *) bad "s3 snapshot reason" "$out";; esac
  out=$(bash "$DSP" submit --purpose "build:app:lane:$(sdig):$(printf '%064d' 1):primary" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s3 a purpose key whose argv digest does not match the argv is refused" test "$rc" = 20
  # an uncommitted edit of the tree changes the snapshot digest, so the purpose key
  a=$(sdig); printf 'edit\n' >> "$T/src/file.txt"; b=$(sdig); chk "s3 an edit of the tree changes the snapshot digest" test "$a" != "$b"
fi
if want s4; then mk s4
  out=$(DISPATCH_ALLOW_LOCAL= bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s4 the local transport is refused without the declared exception (20)" test "$rc" = 20
  case $out in *local_transport_forbidden*) ok "s4 reason local_transport_forbidden";; *) bad "s4 reason" "$out";; esac
fi
if want s5; then mk s5
  out=$(DISPATCH_TRANSPORT=ssh bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s5 hosts.env absent: submit refused (20)" test "$rc" = 20
  case $out in *no_qualified_host*) ok "s5 reason no_qualified_host (absent)";; *) bad "s5 absent reason" "$out";; esac
  : > "$T/hosts.env"
  out=$(DISPATCH_TRANSPORT=ssh bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s5 hosts.env naming no host: submit refused (20)" test "$rc" = 20
  chk "s5 no build directory left" test -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')"
fi

# ================================================================ s6-s8: (a) completed once, build once, resume of a terminal build
if want s6; then mk s6
  id=$(sub 1); chk "s6 submit returned a build id at once" test -n "$id"
  waitb "$id"
  chk "s6 (a) terminal kind completed" test "$(tkind "$id")" = completed
  chk "s6 (a) callback done, keyed effect applied once" test "$(cat "$(bd "$id")/terminal/callback.state")" = done -a "$(effects "$id")" = 1
  chk "s6 events consumed in order 1..4 (accepted, 2 heartbeats, completed)" test "$(ls "$(bd "$id")/consumed" | sort -n | tr '\n' ' ')" = "1 2 3 4 "
  chk "s6 artifact brought back and its digest equals the event's" test "$(bash -c ". '$DSP'; tree_manifest '$(bd "$id")/artifacts'" 2>/dev/null)" = "$(jq -r 'select(.event == null and .kind=="completed") | .artifact_manifest_sha256' "$(bd "$id")/events.jsonl")"
  st=$(bash "$DSP" status "$id"); chk "s6 status reports terminal completed, callback done" test "$(jq -r '.terminal.kind + "/" + .callback_state' <<<"$st")" = completed/done
fi
if want s7; then mk s7
  id=$(sub 1); waitb "$id"; n1=$(jlines "$id"); r1=$(jq -r .run_id "$(bd "$id")/submit.json")
  id2=$(sub 1 2>/dev/null)
  chk "s7 the same purpose gives the same build id (build once)" test "$id" = "$id2"
  chk "s7 no second run: the run id, the journal and the daemon start count are unchanged" test "$(jq -r .run_id "$(bd "$id")/submit.json")" = "$r1" -a "$(jlines "$id")" = "$n1" -a "$(wc -l < "$T/log/daemon.$id")" = 1
  id3=$(sub 2); waitb "$id3"; chk "s7 a fresh iteration is a fresh build" test "$id3" != "$id" -a "$(tkind "$id3")" = completed
  # a failed purpose is never silently rebuilt
  mk s7b; id=$(FAKE_MODE=silent BUDGET=2 sub 1); waitb "$id"; out=$(sub 1 2>&1); rc=$?
  chk "s7 a terminal-but-not-completed purpose is refused until a fresh iteration (20)" test "$rc" = 20
fi
if want s8; then mk s8
  id=$(sub 1); waitb "$id"; pump_gone "$id"; n=$(jlines "$id")
  out=$(bash "$DSP" resume "$id"); chk "s8 resume of a terminal build drains and runs no second effect" test "$out" = drained -a "$(effects "$id")" = 1 -a "$(tkind "$id")" = completed
  chk "s8 resume added no second consume of the completed event (journal unchanged, DUP dropped)" test "$(jlines "$id")" = "$n"
fi

# ================================================================ s9-s12: (c) liveness, (d) late event
if want s9; then mk s9
  id=$(FAKE_MODE=silent sub 1); waitb "$id" 30
  chk "s9 (c) heartbeats stop: blocked-unavailable build_liveness_lost" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = build_liveness_lost
  chk "s9 (c) never a pass; callback once" test "$(effects "$id")" = 1 -a "$(cat "$(bd "$id")/terminal/callback.state")" = done
  chk "s9 (c) the remote build was cancelled and reaped" test "$(wc -l < "$T/log/cancel.$id" 2>/dev/null || echo 0)" = 1
fi
if want s10; then mk s10
  id=$(FAKE_MODE=flat BUDGET=2 sub 1); waitb "$id" 40
  chk "s10 (c) heartbeats arrive but progress is flat: build_progress_flat" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = build_progress_flat
  chk "s10 (c) heartbeats were consumed before the verdict (the stream was alive)" test "$(ls "$(bd "$id")/consumed" | wc -l)" -ge 3
  chk "s10 (c) remote cancelled, callback once" test "$(wc -l < "$T/log/cancel.$id" 2>/dev/null || echo 0)" = 1 -a "$(effects "$id")" = 1
fi
if want s11; then mk s11
  id=$(FAKE_SKEW=1 FAKE_MODE=slow WALL=3 BUDGET=30 sub 1); waitb "$id" 40
  chk "s11 (c) a build that advances past the wall-clock cap: build_wallclock_exceeded, with every sent_at skewed to 2001 (the cap is the emitter's monotonic time)" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = build_wallclock_exceeded
  chk "s11 (c) the skewed fixture really carried skewed sent_at values" test "$(grep -c '"sent_at":"2001-01-01T00:00:00Z"' "$(bd "$id")/events.jsonl")" -ge 2
  mk s11b; id=$(FAKE_SKEW=1 FAKE_MODE=ok sub 1); waitb "$id"
  chk "s11 (c) control: an ordinary build with skewed sent_at still completes (sent_at changes nothing)" test "$(tkind "$id")" = completed
fi
late_completed() { bash -s -- "$REAL_EMIT" "$CORE" "$T/state" "$1" "$(bd "$1")" <<'XEOF'
. "$1"; core=$2; state=$3; id=$4; d=$5
DIR=$d/remote; run=$(jq -r .run_id "$d/submit.json"); KEY=$(bash "$core" derive-key "$state" "$id" "$run")
m=$(manifest_sha "$DIR/out/artifacts")
emit_event "$DIR" "$KEY" "$id" "$run" primary local-proof completed "$(jq -nc --arg m "$m" --arg l "$(printf '%064d' 0)" '{exit_class:"succeeded",artifact_manifest_sha256:$m,image_digest:("sha256:"+$m),remote_log_sha256:$l}')"
XEOF
}
if want s12; then mk s12
  id=$(FAKE_MODE=late BUDGET=2 sub 1); waitb "$id" 30
  chk "s12 (d) the build is HUNG first" test "$(tkind "$id")" = blocked-unavailable
  late_completed "$id"
  bash "$DSP" resume "$id" >/dev/null
  chk "s12 (d) the late completed event is recorded late_ignored and never flips the verdict" test "$(grep -c late_ignored "$(bd "$id")/events.jsonl")" = 1 -a "$(tkind "$id")" = blocked-unavailable -a "$(effects "$id")" = 1
fi

# ================================================================ s13: (m) bring-back mismatch;  s14: (f) forged event refused
if want s13; then mk s13
  id=$(FAKE_MODE=mismatch sub 1); waitb "$id" 30
  chk "s13 (m) a bring-back whose digest differs from its completed event ends infra_failed, never completed" test "$(tkind "$id")" = infra_failed -a "$(treason "$id")" = artifact_mismatch
  chk "s13 (m) the mismatching artifact was never kept; callback once" test ! -d "$(bd "$id")/artifacts" -a "$(effects "$id")" = 1
  chk "s13 (m) status reads infra_failed with the detail" test "$(bash "$DSP" status "$id" | jq -r '.terminal.kind + "/" + .terminal.reason')" = infra_failed/artifact_mismatch
fi
if want s14; then mk s14
  id=$(FAKE_MODE=badsig sub 1); waitb "$id" 30
  chk "s14 (f) the run still completes after a forged heartbeat" test "$(tkind "$id")" = completed
  chk "s14 (f) the wrongly signed event was refused (event_unauthenticated) and never consumed" test "$(grep -c 'rc=20.*event_unauthenticated' "$(bd "$id")/pump.log")" -ge 1 -a "$(grep -c '"progress_offset":150' "$(bd "$id")/events.jsonl")" = 0
  chk "s14 (f) golden-good: every correctly signed event was consumed (seq 1, 2, 4, 5; the forged seq 3 slot is a refused line)" test "$(ls "$(bd "$id")/consumed" | sort -n | tr '\n' ' ')" = "1 2 4 5 "
fi

if want s14b; then mk s14b
  id=$(FAKE_MODE=forgedc sub 1); waitb "$id" 30
  chk "s14b (f) a forged completed event (wrong key) neither ends nor poisons the build: the genuine completion is consumed, kind completed" test "$(tkind "$id")" = completed -a "$(jq -r .exit_class "$(bd "$id")/terminal/state.json")" = succeeded -a "$(effects "$id")" = 1
  chk "s14b (f) the forged event was refused and no bring-back was attempted for it (artifacts match the genuine digest)" test -d "$(bd "$id")/artifacts" -a "$(grep -c 'rc=20.*event_unauthenticated' "$(bd "$id")/pump.log")" -ge 1
fi

# ================================================================ s15: (h) host selection over the ssh shim
mkshim() { # unreachable-list not-rootless-list : an ssh shim that runs the remote command locally under a fake HOME
  cat > "$T/ssh" <<EOF
#!/usr/bin/env bash
host=\$1; shift
printf '%s\n' "\$host" >> "$T/log/ssh.hosts"
case " $1 " in *" \$host "*) exit 255;; esac
cmd="\$*"
case "\$cmd" in
  "true") exit 0;;
  podman\ info*) case " $2 " in *" \$host "*) echo false;; *) echo true;; esac; exit 0;;
esac
cd "$T/fakehome" && HOME="$T/fakehome" exec bash -c "\$cmd"
EOF
  chmod +x "$T/ssh"; export DISPATCH_SSH="$T/ssh"; }
if want s15; then mk s15
  printf 'BUILD_HOST_1=u@down\nBUILD_HOST_2=u@nonrootless\nBUILD_HOST_3=u@good\n' > "$T/hosts.env"
  mkshim "u@down" "u@nonrootless"
  export DISPATCH_TRANSPORT=ssh DISPATCH_REMOTE_RUNP=/bin/true
  id=$(sub 1); waitb "$id" 40
  chk "s15 (h) failover at submit time to the next qualified host, recorded" test "$(jq -r .host "$(bd "$id")/submit.json")" = u@good
  chk "s15 (h) the unreachable and the non-rootless host are recorded with their reasons" test "$(jq -r '[.host_attempts[] | .result + ":" + (.detail // "")] | join(",")' "$(bd "$id")/submit.json")" = "host_unreachable:,not_qualified:runtime_not_rootless,qualified:"
  chk "s15 (h) the build ran on the qualified host through ssh and completed once" test "$(tkind "$id")" = completed -a "$(effects "$id")" = 1
  chk "s15 (h) the emitter was shipped (its sha256 recorded) and the tree went through the content-addressed cache" test -n "$(jq -r .emit_sha256 "$(bd "$id")/submit.json")" -a -d "$T/fakehome/.cache/catalogizer/trees/$(sdig)"
  # all hosts unusable: blocked-unavailable no_qualified_host, callback once, no local build
  mk s15b; printf 'BUILD_HOST_1=u@down\nBUILD_HOST_2=u@nonrootless\n' > "$T/hosts.env"; mkshim "u@down" "u@nonrootless"; export DISPATCH_TRANSPORT=ssh
  id=$(sub 1); waitb "$id" 20
  chk "s15 (h) none left: blocked-unavailable no_qualified_host, callback once, never a pass" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = no_qualified_host -a "$(effects "$id")" = 1
  chk "s15 (h) no local build ran (no remote directory, no daemon)" test ! -e "$T/log/daemon.$id"
fi

if want s22; then mk s22
  export DISPATCH_JOBS=0; id=$(FAKE_MODE=silent sub 1)
  out=$(bash "$DSP" wait "$id" 1); rc=$?
  chk "s22 wait on a build that never reaches a callback times out with exit 3 and says so" test "$rc" = 3 -a "$out" = timeout
  chk "s22 wait on an unknown build is refused (20)" test "$(bash "$DSP" wait b-nope 1 >/dev/null 2>&1; echo $?)" = 20
fi
if want s23; then mk s23
  printf 'min_free_bytes=900000000000000000\n' > "$T/headroom.conf"
  out=$(DISK_HEADROOM_CONF="$T/headroom.conf" sub 1 2>&1); rc=$?
  out2=$(DISK_HEADROOM_CONF="$T/headroom.conf" bash "$DSP" submit --purpose "$(pk 1)" --callback record-only --image IMG-GO --src "$T/src" -- fake-build x 2>&1); rc=$?
  chk "s23 the disk gate runs first: a submit with no headroom is refused (20) before anything is created" test "$rc" = 20
  case $out2 in *disk_below_headroom*) ok "s23 reason disk_below_headroom (passed through)";; *) bad "s23 reason disk_below_headroom" "$out2";; esac
  chk "s23 no build directory, no pump left" test -z "$(ls "$T/builds" 2>/dev/null | grep '^b-')"
fi

# ================================================================ s16: (k) no polling
if want s16; then mk s16
  if ! command -v strace >/dev/null 2>&1; then skipn=$((skipn+1)); say "SKIP s16 (k) strace absent"; else
    export DISPATCH_JOBS=0
    id=$(FAKE_MODE=silent BUDGET=8 sub 1); rm -f "$(bd "$id")/queued"
    st="$T/strace.out"
    FAKE_MODE=silent DISPATCH_RECONNECTS=0 strace -f -qq -ttt -e trace=execve,vfork,clone,clone3 -o "$st" bash "$DSP" _pump "$id" >/dev/null 2>&1 &
    spid=$!
    # wait (event-driven is not needed in a test; bounded wait on the file) until the accepted event has been consumed
    for _ in $(seq 1 100); do [ -e "$(bd "$id")/consumed/1" ] && break; sleep 0.1; done
    sleep 0.6; t0=$(date +%s.%N); sleep 3.5; t1=$(date +%s.%N)
    n=$(awk -v a="$t0" -v b="$t1" '$2 >= a && $2 <= b' "$st" | grep -c -E 'execve|vfork|clone')
    chk "s16 (k) idle event stream: the pump and its blocking wait start no process in 3.5 s" test "$n" = 0
    # control needle: the same instrument sees a poll loop
    strace -f -qq -ttt -e trace=execve,vfork,clone,clone3 -o "$T/needle.out" timeout 2 bash -c 'while :; do sleep 0.3; done' >/dev/null 2>&1
    t3=$(head -1 "$T/needle.out" | awk '{print $2}')
    chk "s16 (k) control needle: the same strace and the same window logic see the process starts of a sleep loop" test "$(awk -v a="$t3" '$2 >= a + 0.5' "$T/needle.out" | grep -c -E 'execve|vfork|clone')" -ge 3
    # the dispatcher's wait and the callback wait: a blocking inotify wait, no process while idle
    strace -f -qq -ttt -e trace=execve,vfork,clone,clone3 -o "$T/wait.out" timeout 4 python3 -I "$EVW" wait "$(bd "$id")" 3 >/dev/null 2>&1
    t2=$(head -1 "$T/wait.out" | awk '{print $2}')
    chk "s16 (k) the wait primitive starts no process during its idle window (after its own start-up, 2.5 s of idle)" test "$(awk -v a="$t2" '$2 >= a + 1.0' "$T/wait.out" | grep -c -E 'execve|vfork|clone')" = 0
    # end the traced pump: the build record is cancelled, which makes the pump leave
    bash "$DSP" cancel "$id" >/dev/null 2>&1; kill "$spid" 2>/dev/null; wait "$spid" 2>/dev/null
  fi
fi

# ================================================================ s17: (l) concurrency
if want s17; then mk s17
  export DISPATCH_JOBS=1
  a=$(FAKE_MODE=hold sub 1); b=$(FAKE_MODE=hold sub 2)
  sleep 1.5
  chk "s17 (l) the second submit is queued, never started, while one build runs" test -e "$(bd "$b")/queued" -a ! -e "$T/log/daemon.$b" -a -s "$T/log/daemon.$a"
  chk "s17 (l) status of the queued build reads queued" test "$(bash "$DSP" status "$b" | jq -r .state)" = queued
  : > "$T/log/release.$a"; waitb "$a" 30
  for _ in $(seq 1 50); do [ -s "$T/log/daemon.$b" ] && break; sleep 0.2; done
  chk "s17 (l) when the first ends, the queued build is started" test -s "$T/log/daemon.$b" -a ! -e "$(bd "$b")/queued"
  : > "$T/log/release.$b"; waitb "$b" 30
  chk "s17 (l) both builds completed, effects once each, never more than 1 running at a time" test "$(tkind "$a")" = completed -a "$(tkind "$b")" = completed -a "$(effects "$a")" = 1 -a "$(effects "$b")" = 1 -a "$(awk '/ start/{n++} / end/{n--} n>1{bad=1} END{print bad+0}' "$T/log/order.log")" = 0
fi

# ================================================================ s18-s20: cancel, resume of an open build, secret loss
if want s18; then mk s18
  id=$(FAKE_MODE=silent BUDGET=30 sub 1); sleep 1.2
  bash "$DSP" cancel "$id" >/dev/null 2>&1; waitb "$id" 20
  chk "s18 cancel ends the build cancelled, callback once" test "$(tkind "$id")" = cancelled -a "$(effects "$id")" = 1
  chk "s18 the remote cancel was sent" test "$(wc -l < "$T/log/cancel.$id" 2>/dev/null || echo 0)" = 1
  sleep 1
  chk "s18 the pump is gone" test "$(bash "$DSP" status "$id" | jq -r .pump_alive)" = false
fi
if want s19; then mk s19
  id=$(FAKE_MODE=hold sub 1); sleep 1.5
  read -r p s < "$(bd "$id")/pump.pid"; [ "$p" -gt 1 ] && [ "$(awk '{print $22}' /proc/$p/stat)" = "$s" ] && kill -KILL "$p"
  sleep 0.3
  chk "s19 (r0) the pump died while the remote build runs" test "$(bash "$DSP" status "$id" | jq -r .pump_alive)" = false
  bash "$DSP" resume "$id" >/dev/null; : > "$T/log/release.$id"; waitb "$id" 30
  chk "s19 (i/r0) the restarted pump resumed without resubmission: completed once, daemon started once" test "$(tkind "$id")" = completed -a "$(effects "$id")" = 1 -a "$(wc -l < "$T/log/daemon.$id")" = 1
  chk "s19 (q3) the resumed pump did not mark the build HUNG for its own downtime" test "$(tkind "$id")" != blocked-unavailable
fi
if want s20; then mk s20
  id=$(FAKE_MODE=hold sub 1); sleep 1.2
  read -r p s < "$(bd "$id")/pump.pid"; kill -KILL "$p" 2>/dev/null; sleep 0.2
  rm -f "$T/state/build_hmac.key"
  bash "$DSP" resume "$id" >/dev/null; waitb "$id" 20
  chk "s20 the secret file lost: blocked-unavailable driver_secret_lost, callback once" test "$(tkind "$id")" = blocked-unavailable -a "$(treason "$id")" = driver_secret_lost -a "$(effects "$id")" = 1
  chk "s20 never resubmitted (daemon started once)" test "$(wc -l < "$T/log/daemon.$id")" = 1
  : > "$T/log/release.$id"
fi

# ================================================================ s21: (t) the secret and the derived key never appear in what the run wrote
if want s21; then mk s21
  id=$(FAKE_MODE=badsig sub 1); waitb "$id" 30
  sec=$(cat "$T/state/build_hmac.key"); run=$(jq -r .run_id "$(bd "$id")/submit.json"); key=$(bash "$CORE" derive-key "$T/state" "$id" "$run")
  scan() { local needle=$1 f; for f in $(find "$T" -type f ! -path "$T/state/*" 2>/dev/null); do grep -qF -- "$needle" "$f" 2>/dev/null && echo "$f"; done; }
  b64s=$(printf '%s' "$sec" | base64 -w0); b64k=$(printf '%s' "$key" | base64 -w0)
  hits="$(scan "$sec"; scan "$key"; scan "$b64s"; scan "$b64k")"
  chk "s21 (t) the driver secret and the per-build key (hex and base64) are in no file of the run outside the state directory" test -z "$hits"
  printf '%s\n' "$key" > "$T/log/planted-needle"
  chk "s21 (t) control needle: the scan sees a planted copy of the key" test -n "$(scan "$key")"
fi

say "OWED: $OWED"
say "RESULT pass=$pass fail=$fail skip=$skipn"
[ "$fail" = 0 ]
