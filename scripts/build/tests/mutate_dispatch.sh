#!/usr/bin/env bash
# mutate_dispatch.sh - T005a paired mutations for test_dispatch.sh. Each mutant is ONE textual change (the old text must occur exactly
# once) to a COPY of dispatch.sh, remote/emit.sh or lib/evwait.py in a private tree; the shipped files are never touched. The test cases that
# guard that check (ONLY=...) must FAIL on it: KILLED. A mutant the cases do not catch is SURVIVED (a defect of the tests).
# Usage: mutate_dispatch.sh list | all | <name>...
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); build=$(cd "$here/.." && pwd); root=$(cd "$here/../../.." && pwd)
# name | file | cases | old | new   (fields separated by "@@@"; <NL> stands for a newline)
MUT=$(cat <<'M'
m_callback_registry@@@dispatch.sh@@@s2@@@|| refuse callback_not_registered "$cb"@@@|| true
m_purpose_regex@@@dispatch.sh@@@s1@@@([0-9a-f]{64}):([0-9a-f]{64}):(primary@@@([0-9a-f]{63,64}):([0-9a-f]{64}):(primary
m_snapshot_check@@@dispatch.sh@@@s3@@@[ "$(snapshot_digest "$src")" = "$psnap" ] || refuse purpose_digest_mismatch "source snapshot digest"@@@true
m_argv_check@@@dispatch.sh@@@s3@@@[ "$(argv_digest "${argv[@]}")" = "$pargv" ] || refuse purpose_digest_mismatch "argv digest"@@@true
m_local_guard@@@dispatch.sh@@@s4@@@[ "${DISPATCH_ALLOW_LOCAL:-}" = 1 ] || refuse local_transport_forbidden@@@true || refuse local_transport_forbidden
m_hosts_absent@@@dispatch.sh@@@s5@@@[ -r "$HOSTS_FILE" ] && [ -n "$(hosts_list)" ] || refuse no_qualified_host@@@true || refuse no_qualified_host
m_disk_gate@@@dispatch.sh@@@s23@@@    || refuse "$(printf '%s' "$dh_err" | grep -o 'reason=[A-Za-z0-9_]*' | head -1 | cut -d= -f2 | sed 's/^$/disk_headroom_failed/')" "disk gate"@@@    || true
m_build_once@@@dispatch.sh@@@s7@@@&& artifact_ok "$d"; then echo "$id"; echo "reused (build once)"@@@&& false; then echo "$id"; echo "reused (build once)"
m_liveness@@@dispatch.sh@@@s9@@@terminate "$d" blocked-unavailable blocked build_liveness_lost >> "$d/pump.log" 2>&1; kill "$empid" 2>/dev/null; pump_end; exit 0@@@pump_end; exit 0
m_progress_flat@@@dispatch.sh@@@s10@@@        if [ "$(reg_class)" = hung ]; then<NL>          if [ "$el" -gt@@@        if false; then<NL>          if [ "$el" -gt
m_wallclock@@@dispatch.sh@@@s11@@@if [ "$el" -gt $(( wall * 1000 )) ]; then why=build_wallclock_exceeded; else why=build_progress_flat; fi@@@why=build_progress_flat
m_bringback_verify@@@dispatch.sh@@@s13@@@if [ "$got" != "$want" ]; then@@@if false; then
m_event_auth_before_bringback@@@dispatch.sh@@@s14b@@@if [ "$evok" = ok ] && ! bring_back@@@if ! bring_back
m_queue@@@dispatch.sh@@@s17@@@if [ "$(count_running "$d")" -ge "$JOBS" ]; then@@@if false; then
m_cancel_remote@@@dispatch.sh@@@s18@@@  cancel_remote "$d"<NL>  bash "$EC" cancel@@@  bash "$EC" cancel
m_secret_lost@@@dispatch.sh@@@s20@@@  if ! [[ $key =~ ^[0-9a-f]{64}$ ]]; then@@@  if false; then
m_failover_reachability@@@dispatch.sh@@@s15@@@if ! rsh "$h" true >/dev/null 2>&1; then@@@if false; then
m_rootless_check@@@dispatch.sh@@@s15@@@if [ "$out" != true ]; then@@@if false; then
m_polling@@@dispatch.sh@@@s16@@@IFS= read -r -t "$((budget + 1))" -u "$emfd" line; rc=$?@@@IFS= read -r -t 0.3 -u "$emfd" line; rc=$?; if [ $rc -gt 128 ]; then /bin/true; continue; fi
m_emit_resign_key@@@emit.sh@@@s6@@@k = bytes.fromhex(sys.stdin.readline().strip())@@@k = bytes.fromhex(sys.stdin.readline().strip())[::-1]
m_emit_cancel_kill@@@emit.sh@@@s9 s18@@@then kill -TERM "$p"; echo "cancel sent"@@@then echo "cancel sent"
m_emit_run_idempotent@@@emit.sh@@@s19@@@if [ ! -s "$DIR/journal.jsonl" ] && [ ! -s "$DIR/daemon.pid" ]; then@@@if true; then
m_wait_timeout_code@@@evwait.py@@@s22@@@            print("timeout")<NL>            return 3@@@            print("timeout")<NL>            return 0
m_tail_stops_early@@@evwait.py@@@s14b@@@if no_pid and b'"kind":"completed"' in line:@@@if b'"kind":"completed"' in line:
M
)
list() { printf '%s\n' "$MUT" | awk -F'@@@' '{print $1 "\t" $2 "\t" $3}'; }
run_one() { # name
  local line f cases old new tmp rc out
  line=$(printf '%s\n' "$MUT" | awk -F'@@@' -v n="$1" '$1 == n')
  [ -n "$line" ] || { echo "no such mutant $1"; return 2; }
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutdsp.XXXXXX")
  # private tree with the layout dispatch.sh derives its ROOT from; the shipped scripts are linked or copied, never edited
  mkdir -p "$tmp/scripts" "$tmp/mut"
  cp -r "$build" "$tmp/scripts/build"; rm -rf "$tmp/scripts/build/tests"
  ln -s "$root/scripts/containers" "$tmp/scripts/containers"; ln -s "$root/scripts/longops" "$tmp/scripts/longops"; ln -s "$root/build" "$tmp/build"
  python3 - "$tmp" "$line" <<'PY' || { rm -rf "$tmp"; echo "mutant text not found exactly once"; return 2; }
import sys
tmp, line = sys.argv[1], sys.argv[2]
name, f, cases, old, new = [x.replace("<NL>", "\n") for x in line.split("@@@")]
path = {"dispatch.sh": "scripts/build/dispatch.sh", "emit.sh": "scripts/build/remote/emit.sh", "evwait.py": "scripts/build/lib/evwait.py"}[f]
s = open(tmp + "/" + path).read()
if s.count(old) != 1:
    print("old text occurs %d times in %s" % (s.count(old), f)); sys.exit(1)
open(tmp + "/" + path, "w").write(s.replace(old, new))
PY
  f=$(printf '%s' "$line" | awk -F'@@@' '{print $2}'); cases=$(printf '%s' "$line" | awk -F'@@@' '{print $3}')
  diff -u "$build/${f/emit.sh/remote/emit.sh}" "$tmp/scripts/build/${f/emit.sh/remote/emit.sh}" 2>/dev/null | head -0
  case $f in
    dispatch.sh) out=$(ONLY="$cases" DISPATCH_SCRIPT="$tmp/scripts/build/dispatch.sh" timeout 300 bash "$here/test_dispatch.sh" 2>&1); rc=$?;;
    emit.sh)     out=$(ONLY="$cases" EMIT_SCRIPT="$tmp/scripts/build/remote/emit.sh" timeout 300 bash "$here/test_dispatch.sh" 2>&1); rc=$?;;
    evwait.py)   out=$(ONLY="$cases" EVWAIT_SCRIPT="$tmp/scripts/build/lib/evwait.py" timeout 300 bash "$here/test_dispatch.sh" 2>&1); rc=$?;;
  esac
  printf '=== %s (%s, cases: %s)\n' "$1" "$f" "$cases"
  printf '%s\n' "$out" | grep -E '^(FAIL|RESULT)' | head -6
  if [ "$rc" != 0 ]; then echo "KILLED $1"; else echo "SURVIVED $1"; SURV="$SURV $1"; fi
  rm -rf "$tmp"
}
SURV=""
case ${1:-} in
  list) list;;
  all) while IFS= read -r n; do run_one "$n"; done < <(list | cut -f1); echo "SURVIVORS:${SURV:- none}"; [ -z "$SURV" ];;
  "") echo "usage: $0 list | all | <name>..." >&2; exit 2;;
  *) for n in "$@"; do run_one "$n"; done; [ -z "$SURV" ];;
esac
