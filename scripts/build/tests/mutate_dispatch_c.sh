#!/usr/bin/env bash
# mutate_dispatch_c.sh - paired mutations for the round-c additions (test_snapshot.sh, test_dispatch_c.sh): the git-plumbing snapshot, the input closure, the tree cache,
# the callback runner, the event hub, build groups, the emitter restart / signing / peak / progress, host-key pinning, the keyring, and the long-op registry binding (T089a).
# Each mutant is ONE textual change (the old text must occur exactly once) to a COPY of the file in a private tree; the shipped files are never touched. The cases that
# guard that change (ONLY=...) must FAIL on it: KILLED. A mutant the cases do not catch is SURVIVED (a defect of the tests).
# Usage: mutate_dispatch_c.sh list | check | all | <name>...        (`check` only verifies that every old text occurs exactly once; it runs no test)
# Row: name @@@ file (relative to scripts/build) @@@ suite (snap|c) @@@ cases @@@ old @@@ new      (<NL> stands for a newline)
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); build=$(cd "$here/.." && pwd); root=$(cd "$here/../../.." && pwd)
MUT=$(cat <<'M'
m_snap_lfs_filter@@@lib/snapshot.py@@@snap@@@p4@@@"-c", "filter.lfs.process=", "-c", "filter.lfs.clean=cat", @@@
m_snap_cpa_seed@@@lib/snapshot.py@@@snap@@@p8@@@if a.mode == "cpa" or not os.path.exists(real_index(abs_)):@@@if not os.path.exists(real_index(abs_)):
m_snap_untracked@@@lib/snapshot.py@@@snap@@@p1@@@["ls-files", "-m", "-d", "-o", "--exclude-standard", "-z"], index=idx)@@@["ls-files", "-m", "-d", "--exclude-standard", "-z"], index=idx)
m_snap_exclude@@@lib/snapshot.py@@@snap@@@p7 p5@@@excl = [e for e in (a.exclude or [])] if (a.klass == "compile" and rel == ".") else []@@@excl = []
m_snap_submodule_depth@@@lib/snapshot.py@@@snap@@@p2@@@        out.append({"path": rel, "tree": tree, "head": head, "gitlinks": gl})<NL>        for g in sorted(gl):<NL>            if initialised(abs_, g):<NL>                rec(os.path.join(abs_, g), g if rel == "." else rel + "/" + g)@@@        out.append({"path": rel, "tree": tree, "head": head, "gitlinks": gl})
m_snap_uninit_refusal@@@lib/snapshot.py@@@snap@@@p6@@@                    if not ini:<NL>                        raise Refused("submodule_uninitialised", "%s is inside the submodule %s, which is not initialised (referenced by@@@                    if False:<NL>                        raise Refused("submodule_uninitialised", "%s is inside the submodule %s, which is not initialised (referenced by
m_snap_descendants@@@lib/snapshot.py@@@snap@@@p6@@@need.update(g for g in repos if g != "." and under(g, rel) and rel != ".")@@@pass
m_snap_closure_incomplete@@@lib/snapshot.py@@@snap@@@p6@@@            if n not in names:<NL>                raise Refused("closure_incomplete", "the input closure@@@            if False:<NL>                raise Refused("closure_incomplete", "the input closure
m_snap_plan_root_only@@@lib/snapshot.py@@@snap@@@p9@@@    ids = set()<NL>    for _, abs_, r in shipped(a, m):@@@    ids = set()<NL>    for _, abs_, r in shipped(a, m)[:1]:
m_snap_npm_file@@@lib/snapshot.py@@@snap@@@p6@@@if isinstance(v, str) and re.match(r"^(file|link):", v):@@@if False:
m_snap_undeclared@@@lib/snapshot.py@@@snap@@@p8@@@            if full in declared or any(under(full, e) for e in excl):<NL>                continue@@@            continue
m_tc_object_verify@@@lib/treecache.py@@@snap@@@p9@@@    if oid(kind, data) != i:<NL>        raise Mismatch("object %s in the cache@@@    if False:<NL>        raise Mismatch("object %s in the cache
m_tc_recompute@@@lib/treecache.py@@@snap@@@p9@@@        if got != r["tree"]:@@@        if False:
m_tc_store_verify@@@lib/treecache.py@@@snap@@@p9@@@        if oid("blob" if kind == "b" else "tree", data) != i:@@@        if False:
m_tc_gitlink_manifest@@@lib/treecache.py@@@snap@@@p9@@@        put(g, ("160000", c))@@@        pass
m_ship_all_objects@@@dispatch.sh@@@c@@@c2@@@  hx "$h" "python3 -I $tc missing $cache" < "$ids" > "$miss" || { rm -f "$ids" "$miss"; return 1; }@@@  cp "$ids" "$miss"
m_ship_mismatch_kind@@@dispatch.sh@@@c@@@c4@@@  if [ $mismatch = 1 ]; then terminate "$d" infra_failed infra_failed snapshot_mismatch >&2; echo "$id"; return 0; fi@@@  if [ $mismatch = 1 ]; then terminate "$d" blocked-unavailable blocked host_unreachable >&2; echo "$id"; return 0; fi
m_ship_whole_checkout@@@dispatch.sh@@@c@@@c1@@@    crepos=$(jq -r '.repos | join(",")' "$closure")@@@    crepos=$(jq -r '[.repos[].path] | join(",")' "$msnap")
m_run_done_rerun@@@run_callback.sh@@@c@@@c5@@@  done) exit 0;;@@@  done) :;;
m_run_failed_retry@@@run_callback.sh@@@c@@@c6@@@  failed) [ $RETRY = 1 ] || exit 1;;@@@  failed) :;;
m_run_registry@@@run_callback.sh@@@c@@@c7@@@if [ -z "$row" ]; then@@@if false; then
m_run_stamp@@@run_callback.sh@@@c@@@c7@@@[ -z "${CPA_EXEC_SHA256:-}" ] || tw "$D/terminal/callback.runner_sha256" "$CPA_EXEC_SHA256"@@@tw "$D/terminal/callback.runner_sha256" "$(printf '%064d' 5)"
m_run_noeffect@@@run_callback.sh@@@c@@@c5@@@  if [ -e "$D/effects/$skey" ]; then setstate done; echo "callback done"; exit 0; fi<NL>  setstate failed effect_not_applied; echo "callback failed: effect_not_applied"; exit 1@@@  setstate done; echo "callback done"; exit 0
m_hub_single@@@event_hub.sh@@@c@@@c8@@@  if ! flock -w 3 9; then@@@  if false; then
m_hub_pidfile@@@event_hub.sh@@@c@@@c8@@@  tw "$BUILDS/hub.pid" "$me $st"; tw "$BUILDS/hub.json"@@@  tw "$BUILDS/hub.json"
m_hub_resume_pump@@@event_hub.sh@@@c@@@c9@@@      x bash "$DSP" resume "$id" >> "$d/pump.log" 2>&1; resumed=1@@@      resumed=1
m_hub_callback_resume@@@event_hub.sh@@@c@@@c10@@@      case $cbs in claimed|running) x env@@@      case $cbs in claimed_|running_) x env
m_hub_stamp@@@event_hub.sh@@@c@@@c11@@@    t="$f.tmp-$$"; jq --arg v "$v" '.hub_sha256 = $v'@@@    t="$f.tmp-$$"; jq --arg v "$(printf '%064d' 3)" '.hub_sha256 = $v'
m_hub_poll@@@event_hub.sh@@@c@@@c12@@@    if [ -n "$exp" ]; then read -r -t "$exp" -u "$efd" _ev; rc=$?; else read -r -u "$efd" _ev; rc=$?; fi@@@    read -r -t 0.2 -u "$efd" _ev; rc=$?; sleep 0.2; /bin/true
m_hub_live_table@@@event_hub.sh@@@c@@@c40@@@  x env RUNCB_CALLBACKS_TSV="$BUILDS/hub.callbacks.tsv" setsid -f bash "$RUNCB"@@@  x env setsid -f bash "$RUNCB"
m_hub_ensure@@@dispatch.sh@@@c@@@c13@@@ensure_hub() { [ "${DISPATCH_HUB:-$HUB_DEFAULT}" = 1 ] || return 0;@@@ensure_hub() { return 0;
m_grp_precedence@@@dispatch.sh@@@c@@@c20@@@    gk=""; for pass in "test_failed build_failed" infra_failed blocked-unavailable cancelled; do@@@    gk=""; for pass in cancelled blocked-unavailable infra_failed "test_failed build_failed"; do
m_grp_cof_ignored@@@dispatch.sh@@@c@@@c17@@@test_failed|build_failed) [ "$cof" = true ] && { trig=$m; trigk=$k; };;@@@test_failed|build_failed) :;;
m_grp_cof_always@@@dispatch.sh@@@c@@@c17 c18@@@test_failed|build_failed) [ "$cof" = true ] && { trig=$m; trigk=$k; };;@@@test_failed|build_failed) true && { trig=$m; trigk=$k; };;
m_grp_cbg_kind@@@dispatch.sh@@@c@@@c16@@@  if [ -e "$d/cancelled_by_group" ] && { [ "$k" = cancelled ] || [ "$ec" = cancelled ]; }; then echo cancelled_by_group@@@  if false; then echo cancelled_by_group
m_grp_unsealed@@@dispatch.sh@@@c@@@c19@@@    if [ "$sealed" = true ] || [ "$orphaned" = true ]; then :; else echo wait; exit 0; fi@@@    :
m_grp_all_terminal@@@dispatch.sh@@@c@@@c14@@@    [ $allt = 1 ] || { echo wait; exit 0; }@@@    :
m_grp_member_cb@@@dispatch.sh@@@c@@@c14@@@    gcb=$cb; cb=record-only@@@    gcb=$cb
m_grp_conflict@@@dispatch.sh@@@c@@@c19@@@    group_validate "$grp" "$cb" "$cof"@@@    :
m_grp_orphan@@@dispatch.sh@@@c@@@c19@@@ -gt "$(jq -r .budget_s <<<"$gj")" ]; }@@@ -gt 99999999 ]; }
m_grp_once@@@dispatch.sh@@@c@@@c15@@@    [ -d "$g/terminal" ] && { echo done; exit 0; }@@@    [ -d "$g/terminal" ] && { rm -rf "$g/terminal"; }
m_reg_before_start@@@dispatch.sh@@@c@@@c23@@@  if [ -z "$drain" ]; then   # registered BEFORE the remote start: the build is a long-op from here on@@@  if false; then
m_reg_release@@@dispatch.sh@@@c@@@c23@@@  reg_finish "$1"; script_phase "$1"; group_hook "$1"; }@@@  script_phase "$1"; group_hook "$1"; }
m_reg_lane_dropped@@@dispatch.sh@@@c@@@c24@@@  purpose="build:$pcomp:$plane:$psnap:$pargv:$pv${piter:+:$piter}"@@@  purpose="build:$pcomp:x:$psnap:$pargv:$pv${piter:+:$piter}"
m_reg_iteration_ignored@@@dispatch.sh@@@c@@@c24 c26@@@  purpose="build:$pcomp:$plane:$psnap:$pargv:$pv${piter:+:$piter}"@@@  purpose="build:$pcomp:$plane:$psnap:$pargv:$pv"
m_reg_variant_attached@@@dispatch.sh@@@c@@@c26@@@  purpose="build:$pcomp:$plane:$psnap:$pargv:$pv${piter:+:$piter}"@@@  purpose="build:$pcomp:$plane:$psnap:$pargv:primary${piter:+:$piter}"
m_reg_heartbeat_ignored@@@dispatch.sh@@@c@@@c27@@@        reg_beat "$po" "$st" "$el"                       # the heartbeat feeds@@@        :                       # the heartbeat feeds
m_reg_flat_as_advancing@@@dispatch.sh@@@c@@@c27@@@--progress-offset "$(( $1 + $2 ))" --elapsed-ms "$3"@@@--progress-offset "$(( $1 + $2 + SECONDS ))" --elapsed-ms "$3"
m_reg_wallclock@@@dispatch.sh@@@c@@@c28@@@--no-progress-s "$np" --wall-s "$wall" \<NL>      --container-label "catalogizer.op_id=dispatch-$id" >/dev/null 2>"$d/tmp/reg.err"; rc=$?<NL>  if [ $rc -eq 4 ]@@@--no-progress-s "$np" --wall-s 0 \<NL>      --container-label "catalogizer.op_id=dispatch-$id" >/dev/null 2>"$d/tmp/reg.err"; rc=$?<NL>  if [ $rc -eq 4 ]
m_reg_handoff@@@dispatch.sh@@@c@@@c29@@@  trap '[ -d "$d/terminal" ] || reg_handoff; exit 0' TERM@@@  trap 'exit 0' TERM
m_reg_dead_owner@@@dispatch.sh@@@c@@@c30@@@    [ "$cls" != dead_owner ] || bash "$LO/reap.sh" --op-id "$opid" >/dev/null 2>&1@@@    :
m_reg_conflict@@@dispatch.sh@@@c@@@c31@@@  if [ -n "$hj" ] && [ "$hj" != none ]; then hr=$(jq -r '.run_id // ""' <<<"$hj" 2>/dev/null)@@@  if false; then hr=$(jq -r '.run_id // ""' <<<"$hj" 2>/dev/null)
m_pair_variant@@@dispatch.sh@@@c@@@c32@@@mine="${BASH_REMATCH[1]}:${BASH_REMATCH[2]}:${BASH_REMATCH[3]}:${BASH_REMATCH[4]}"; want=primary; [ "${BASH_REMATCH[5]}" = primary ] && want=repro-cold@@@mine="${BASH_REMATCH[1]}:${BASH_REMATCH[2]}:${BASH_REMATCH[3]}:${BASH_REMATCH[4]}"; want=primary
m_emit_no_key_check@@@remote/emit.sh@@@c@@@c35@@@  valid_key "$key" || exit 3                                       # no key, no sending@@@  :
m_emit_signed_at_rest@@@remote/emit.sh@@@c@@@c34@@@  printf '%s\n' "$line" >> "$dir/journal.jsonl" &&@@@  line=$(printf '%s\n' "$line" | sign_event "$2" 2>/dev/null | tr -d '\n'); printf '%s\n' "$line" >> "$dir/journal.jsonl" &&
m_emit_adopt@@@remote/emit.sh@@@c@@@c34@@@              elif ! daemon_alive && ! grep -q '"kind":"completed"' "$DIR/journal.jsonl" 2>/dev/null && [ -s "$DIR/build.pid" ]; then@@@              elif false; then
m_emit_peak@@@remote/emit.sh@@@c@@@c36@@@  peak=$(cat "$DIR/peak_rss" 2>/dev/null || true); ex='{}'@@@  peak=; ex='{}'
m_emit_cpu@@@remote/emit.sh@@@c@@@c37@@@  if [ "$PROGRESS" = log+cpu ]; then cg_sample; po=$(( po + CPU_MS )); fi@@@  if [ "$PROGRESS" = log+cpu ]; then cg_sample; fi
m_pin_strict@@@dispatch.sh@@@c@@@c38@@@    "${SSHC[@]}" -o StrictHostKeyChecking=yes -o UserKnownHostsFile@@@    "${SSHC[@]}" -o UserKnownHostsFile
m_pin_anykey@@@dispatch.sh@@@c@@@c38@@@    if [ "$got" = "$fp" ]; then@@@    if [ -n "$got" ]; then
m_pin_unpinned@@@dispatch.sh@@@c@@@c38@@@  fp=$(host_fp "$h"); [ -n "$fp" ] || return 11@@@  fp=$(host_fp "$h")
m_keyring_file_truth@@@lib/keyring.sh@@@c@@@c39@@@    if ! bash "$EC" derive-key "$sd" keyring-probe keyring-probe >/dev/null 2>&1 || [ ! -s "$f" ]; then@@@    if false; then
M
)
list() { printf '%s\n' "$MUT" | awk -F'@@@' '{print $1 "\t" $2 "\t" $3 "\t" $4}'; }
prepare() { # tmpdir : private tree with the layout dispatch.sh derives its ROOT from
  local tmp=$1; mkdir -p "$tmp/scripts" && cp -r "$build" "$tmp/scripts/build" && rm -rf "$tmp/scripts/build/tests" "$tmp/scripts/build/lib/__pycache__"
  ln -s "$root/scripts/containers" "$tmp/scripts/containers"; ln -s "$root/scripts/longops" "$tmp/scripts/longops"; ln -s "$root/build" "$tmp/build"; }
apply() { # tmpdir line : applies the change to the copy; 1 when the old text does not occur exactly once
  python3 - "$1" "$2" <<'PY'
import sys
tmp, line = sys.argv[1], sys.argv[2]
name, f, suite, cases, old, new = [x.replace("<NL>", "\n") for x in line.split("@@@")]
path = tmp + "/scripts/build/" + f
s = open(path).read()
if s.count(old) != 1:
    print("old text occurs %d times in %s" % (s.count(old), f)); sys.exit(1)
open(path, "w").write(s.replace(old, new))
PY
}
run_one() { # name
  local line f suite cases tmp rc out
  line=$(printf '%s\n' "$MUT" | awk -F'@@@' -v n="$1" '$1 == n')
  [ -n "$line" ] || { echo "no such mutant $1"; return 2; }
  tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutdc.XXXXXX"); prepare "$tmp"
  apply "$tmp" "$line" || { rm -rf "$tmp"; echo "INVALID $1 (mutant text not found exactly once)"; SURV="$SURV $1"; return 2; }
  f=$(awk -F'@@@' '{print $2}' <<<"$line"); suite=$(awk -F'@@@' '{print $3}' <<<"$line"); cases=$(awk -F'@@@' '{print $4}' <<<"$line")
  case $suite in
    snap) out=$(ONLY="$cases" SNAPSHOT_PY="$tmp/scripts/build/lib/snapshot.py" TREECACHE_PY="$tmp/scripts/build/lib/treecache.py" timeout 900 bash "$here/test_snapshot.sh" 2>&1); rc=$?;;
    c)    out=$(ONLY="$cases" BUILD_DIR="$tmp/scripts/build" timeout 900 bash "$here/test_dispatch_c.sh" 2>&1); rc=$?;;
  esac
  printf '=== %s (%s, %s: %s)\n' "$1" "$f" "$suite" "$cases"
  printf '%s\n' "$out" | grep -E '^(FAIL|RESULT)' | head -4 | cut -c1-200
  if [ "$rc" != 0 ]; then echo "KILLED $1"; else echo "SURVIVED $1"; SURV="$SURV $1"; fi
  rm -rf "$tmp"
}
SURV=""
case ${1:-} in
  list) list;;
  check) bad=0; tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutdc.XXXXXX"); prepare "$tmp"
         while IFS= read -r n; do line=$(printf '%s\n' "$MUT" | awk -F'@@@' -v n="$n" '$1 == n'); t2=$(mktemp -d "${TMPDIR:-/tmp}/mutdc.XXXXXX"); prepare "$t2"
           if apply "$t2" "$line" >/dev/null 2>&1; then echo "ok      $n"; else echo "INVALID $n"; bad=1; fi; rm -rf "$t2"; done < <(list | cut -f1); rm -rf "$tmp"; exit $bad;;
  all) while IFS= read -r n; do run_one "$n"; done < <(list | cut -f1); echo "SURVIVORS:${SURV:- none}"; [ -z "$SURV" ];;
  "") echo "usage: $0 list | check | all | <name>..." >&2; exit 2;;
  *) for n in "$@"; do run_one "$n"; done; [ -z "$SURV" ];;
esac
