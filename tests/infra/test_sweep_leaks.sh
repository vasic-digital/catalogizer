#!/usr/bin/env bash
# test_sweep_leaks.sh - WF12 F2 (RED first). Oracle for scripts/test-infra/sweep_leaks.sh: a leaked pod or output directory of this stack's per-run project namespace is removed ONLY after its
# ownership is proven, and everything that fails a proof survives. Real rootless podman (empty pods and one created-not-started container), real directories; no stack is started.
# Oracle strategy (11.4.245): SPECIFIED. The fixtures are the proofs of the script header, each with a survivor and a victim:
#   victim   an empty, unlabelled pod created exactly as podman-compose creates it (`--name=pod_<project> --infra=false --share=`) of a project that is not live
#   survivors a pod of another shape (different create command), a labelled pod, a pod that holds a container, a pod of a LIVE project (per-run state directory present), a pod whose name
#            does not match the namespace, a directory that is a symlink, `-logs`, a directory of a live project, a directory whose name does not match
# WF17 round 5 (TI-G7, TI-A3, TI-C3, TI-B5): one SURVIVOR fixture per proof of live() (a pod live only by its claim, only by a labelled container; a `-client` directory whose whole name is itself a live project), the
# THIRD kind (per-run state directories: victim = terminal operation or provably dead owner, survivors = `.keep-state` marker, live claim, labelled container, no op record, live owner) and the FOURTH kind
# (the NAS leg's run directories: victim = leg process gone or a recycled pid, survivor = a live leg). The pod-state proof (SW4) cannot be constructed independently: a pod that lost its last container returns to
# `Created` (measured), so `state_is_not_Created` is covered by `holds_a_container` and the test says so instead of claiming a survivor.
# Paired mutations: the sweep copy with ONE proof removed or one pattern widened; the same fixtures must FAIL; identity mutant (SW5) must SURVIVE.
# Usage: test_sweep_leaks.sh   (SWEEP_NO_MUTATIONS=1: tests only)   Env: SWEEP_SUT (repo-relative script path)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT="${SWEEP_SUT:-scripts/test-infra/sweep_leaks.sh}"
export TI_ROOT="$TI_REPO"
if [ ! -f "$TI_REPO/$SUT" ]; then bad "script absent: $SUT"; ti_summary; exit 1; fi
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));e=[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0];print(e['reference']+'@'+e['digest'])")
OUTD="$TI_REPO/.audit/out"; STDIR="$TI_REPO/.audit/test-infra"; STD_="$STDIR"   # STDIR is the state DIRECTORY; STD (below) is the NAME of the dead-owner fixture project
R="zzsw$RANDOM"
# fixtures (rebuilt from scratch before every battery: a mutant that removed a survivor must not hide the next mutant) ----------------------------------------------------------
mkpod() { podman pod create "--name=$1" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("$1"); }
mkd() { mkdir -p "$1"; echo x >"$1/f"; TI_FOREIGN_DIRS+=("$1"); }
V="pod_catalogizer-test-${R}v"; S1="pod_catalogizer-test-${R}shape"; S2="pod_catalogizer-test-${R}label"; S3="pod_catalogizer-test-${R}full"; S4="pod_catalogizer-test-${R}live"; S5="pod_${R}other"
DV1="$OUTD/catalogizer-test-${R}d-client"; DV2="$OUTD/catalogizer-test-${R}d-seed"; DS1="$OUTD/catalogizer-test-${R}d-logs"; DS2="$OUTD/catalogizer-test-${R}live-client"; DS3="$OUTD/${R}-client"
TGT="$TI_SCRATCH/symtarget"; DS4="$OUTD/catalogizer-test-${R}sym-client"
mkfix() {
  podman rm -f "catalogizer-test-${R}full-c" >/dev/null 2>&1; podman pod rm -f "$V" "$S1" "$S2" "$S3" "$S4" "$S5" >/dev/null 2>&1
  rm -rf -- "$DV1" "$DV2" "$DS1" "$DS2" "$DS3" "$DS4" "$TGT" "$STDIR/catalogizer-test-${R}live"
  mkpod "$V"                                                                                       # the victim
  podman pod create --name "$S1" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("$S1")  # same name shape, other create command (`--name x`)
  podman pod create "--name=$S2" --infra=false --share= --label keep=me >/dev/null 2>&1; TI_FOREIGN_PODS+=("$S2")
  mkpod "$S3"; podman create --pull=never --pod "$S3" --name "catalogizer-test-${R}full-c" --entrypoint sleep "$LOCKIMG" 60 >/dev/null 2>&1
  mkpod "$S4"; mkdir -p "$STDIR/catalogizer-test-${R}live"; TI_FOREIGN_DIRS+=("$STDIR/catalogizer-test-${R}live")
  mkpod "$S5"
  mkd "$DV1"; mkd "$DV2"; mkd "$DS1"; mkd "$DS2"; mkd "$DS3"
  mkdir -p "$TGT"; echo x >"$TGT/f"; ln -s "$TGT" "$DS4"; TI_FOREIGN_DIRS+=("$DS4")
  # WF17 survivors per proof of live(): live only by its claim | only by a labelled container | a -client directory whose whole name is itself a live project
  podman rm -f "$SWX-c" >/dev/null 2>&1; podman pod rm -f "pod_$SWC" "pod_$SWX" >/dev/null 2>&1; rm -rf -- "$OUTD/${SWD}-client" "$STD_/$SWD-client" "$STD_/$SWD"
  mkpod "pod_$SWC"; mkpod "pod_$SWX"; SWXC=$(podman create --pull=never --name "$SWX-c" --label "catalogizer.test_project=$SWX" --entrypoint sleep "$LOCKIMG" 60 2>/dev/null); TI_FOREIGN+=("$SWXC")
  mkd "$OUTD/${SWD}-client"; mkdir -p "$STD_/$SWD-client"; TI_FOREIGN_DIRS+=("$STD_/$SWD-client")   # the state directory of the project named like the whole directory: that project is live, the project `$SWD` is not
  # the THIRD kind: per-run state directories
  for sd in "$STV" "$STK" "$STC" "$STN" "$STL" "$STD" "$STX"; do rm -rf -- "$STD_/$sd"; mkdir -p "$STD_/$sd/data"; echo x >"$STD_/$sd/data/f"; TI_FOREIGN_DIRS+=("$STD_/$sd"); done
  echo "zz-stv-$R" >"$STD_/$STV/op_id"; echo "zz-stk-$R" >"$STD_/$STK/op_id"; : >"$STD_/$STK/.keep-state"; echo "zz-stc-$R" >"$STD_/$STC/op_id"; echo "zz-stl-$R" >"$STD_/$STL/op_id"; echo "zz-std-$R" >"$STD_/$STD/op_id"; echo "zz-stx-$R" >"$STD_/$STX/op_id"
  podman rm -f "$STX-c" >/dev/null 2>&1; STXC=$(podman create --pull=never --name "$STX-c" --label "catalogizer.test_project=$STX" --entrypoint sleep "$LOCKIMG" 60 2>/dev/null); TI_FOREIGN+=("$STXC")
  # the FOURTH kind: the NAS leg's run directories (tmpfs and the old in-checkout place)
  RTD="${XDG_RUNTIME_DIR:-}"; GONE_PID=$(bash -c 'echo $$'); NASV="$RTD/catalogizer-nas-ro.$GONE_PID"; NASR="$RTD/catalogizer-nas-ro.$$"; NASL_PROC_PID=""
  bash -c 'exec -a nas_readonly_leg sleep 120' & NASL_PROC_PID=$!; disown; NASK="$TI_REPO/.audit/out/nas-ro-$NASL_PROC_PID"
  for d in "$NASV" "$NASR" "$NASK"; do mkdir -p "$d"; echo x >"$d/auth-decoy"; TI_FOREIGN_DIRS+=("$d"); done
}
REG="$(ti_regdir)"; REGS=()
regop() { # regop <op id> <purpose> <claim yes|no> <owner pid>: registers a fixture operation in the real registry (released at exit)
  local c=(); [ "$3" = yes ] || c=(--no-claim); bash "$TI_REPO/scripts/longops/register.sh" --purpose "$2" --owner sweep-test --op-id "$1" --pid "$4" "${c[@]}" >/dev/null 2>&1; REGS+=("$1"); }
trap 'for o in "${REGS[@]:-}"; do [ -n "$o" ] && bash "$TI_REPO/scripts/longops/release.sh" --op-id "$o" --state complete --verdict test_done >/dev/null 2>&1; done; ti_cleanup' EXIT
sleep 600 & ALIVE=$!; disown; sleep 600 & DEADPID=$!; disown
SWC="catalogizer-test-${R}claim"; SWX="catalogizer-test-${R}cont"; SWD="catalogizer-test-${R}xx"
STV="catalogizer-test-${R}stv"; STK="catalogizer-test-${R}stk"; STC="catalogizer-test-${R}stc"; STN="catalogizer-test-${R}stn"; STL="catalogizer-test-${R}stl"; STD="catalogizer-test-${R}std"; STX="catalogizer-test-${R}stx"
regop "zz-swc-$R" "$SWC" yes "$ALIVE"            # a live CLAIM, no state directory, no container
regop "zz-stc-$R" "$STC" yes "$ALIVE"
regop "zz-stv-$R" "$STV" no "$ALIVE"; bash "$TI_REPO/scripts/longops/release.sh" --op-id "zz-stv-$R" --state complete --verdict test >/dev/null 2>&1   # terminal
regop "zz-stl-$R" "$STL" no "$ALIVE"             # non-terminal, live owner
regop "zz-std-$R" "$STD" no "$DEADPID"; kill "$DEADPID" 2>/dev/null; wait "$DEADPID" 2>/dev/null   # non-terminal, owner then dies: PROVEN dead
mkfix
state() { local v=""; for p in "$V" "$S1" "$S2" "$S3" "$S4" "$S5"; do podman pod exists "$p" 2>/dev/null && v="$v${p##*_}:present " || v="$v${p##*_}:gone "; done
  for d in "$DV1" "$DV2" "$DS1" "$DS2" "$DS3"; do [ -e "$d" ] && v="$v$(basename "$d"):present " || v="$v$(basename "$d"):gone "; done
  [ -L "$DS4" ] && [ -d "$TGT" ] && v="${v}symlink:present target:present" || v="${v}symlink:GONE-OR-TARGET-GONE"; echo "$v"; }
# kr: the REMOVED line of a victim; the sweeps of OTHER streams share this checkout's directories and pods, and a concurrent sweep that removed the fixture first (its proofs are the same) leaves the fixture gone and no KEPT line
# for it: that is accepted with a note, a victim that is STILL THERE or KEPT is a failure as before
kr() { local rx=$1 nm; shift; printf '%s\n' "$o" | grep -q "$rx" && return 0; nm=$(printf '%s' "$rx" | sed 's/^\^REMOVED [a-z]* //; s/ proof=.*//')
  if "$@" && ! printf '%s\n' "$o" | grep -q "^KEPT [a-z]* $nm"; then echo "NOTE: $nm was removed by a concurrent sweep of another stream before this one" >&2; return 0; fi
  echo "FAIL expected line missing: $rx"; n=$((n+1)); }
battery() { # battery <sut-rel> [--dry-run]: the sweep runs; prints one FAIL line per violated expectation; exit status = count
  local sut=$1 n=0 o st; shift
  o=$(bash "$TI_REPO/$sut" "$@" 2>&1); RC=$?; st="$(state)"
  if [ "${1:-}" = --dry-run ]; then
    case "$st" in *"${V##*_}:present"*) ;; *) echo "FAIL the dry run removed the victim pod"; n=$((n+1));; esac
    printf '%s\n' "$o" | grep -q "^WOULD_REMOVE pod $V " || { echo "FAIL the dry run did not announce the victim pod"; n=$((n+1)); }
    return "$n"
  fi
  [ "$RC" = 0 ] || { echo "FAIL the sweep exited $RC"; n=$((n+1)); }
  case "$st" in *"${V##*_}:gone"*) ;; *) echo "FAIL the victim pod survived"; n=$((n+1));; esac
  case "$st" in *"catalogizer-test-${R}d-client:gone"*"catalogizer-test-${R}d-seed:gone"*) ;; *) echo "FAIL a victim directory survived ($st)"; n=$((n+1));; esac
  case "$st" in *"${S1##*_}:present"*"${S2##*_}:present"*"${S3##*_}:present"*"${S4##*_}:present"*"${S5##*_}:present"*) ;; *) echo "FAIL a survivor pod was removed ($st)"; n=$((n+1));; esac
  case "$st" in *"catalogizer-test-${R}d-logs:present"*"catalogizer-test-${R}live-client:present"*"${R}-client:present"*"symlink:present target:present") ;; *) echo "FAIL a survivor directory was removed ($st)"; n=$((n+1));; esac
  kr "^REMOVED pod $V proof=" bash -c "! podman pod exists $V"
  printf '%s\n' "$o" | grep -q "^KEPT pod $S3 failed=holds_a_container" || { echo "FAIL the pod that holds a container is not KEPT for that reason"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -q "^KEPT pod $S4 failed=project_is_live(state_directory_exists)" || { echo "FAIL the pod of the live project is not KEPT for that reason"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -q "^KEPT dir catalogizer-test-${R}sym-client failed=not_a_real_directory" || { echo "FAIL the symlink is not KEPT for that reason"; n=$((n+1)); }
  # WF17 survivors per proof (SW1, SW2, SW3) and the state / nasdir kinds
  k() { printf '%s\n' "$o" | grep -q "$1" || { echo "FAIL expected line missing: $1"; n=$((n+1)); }; }
  k "^KEPT pod pod_$SWC failed=project_is_live(lease_claim_exists)"; k "^KEPT pod pod_$SWX failed=project_is_live(labelled_container_exists)"
  k "^KEPT dir ${SWD}-client failed=name_is_itself_a_live_project(state_directory_exists)"
  kr "^REMOVED state $STV proof=" test ! -e "$STD_/$STV"; kr "^REMOVED state $STD proof=.*op_owner_proven_dead" test ! -e "$STD_/$STD"
  k "^KEPT state $STK failed=kept_by_request"; k "^KEPT state $STC failed=lease_claim_exists"; k "^KEPT state $STN failed=no_op_record"
  k "^KEPT state $STL failed=op_not_provably_dead"; k "^KEPT state $STX failed=labelled_container_exists"
  [ ! -e "$STD_/$STV" ] && [ ! -e "$STD_/$STD" ] || { echo "FAIL a victim state directory survived"; n=$((n+1)); }
  [ -e "$STD_/$STK" ] && [ -e "$STD_/$STC" ] && [ -e "$STD_/$STN" ] && [ -e "$STD_/$STL" ] && [ -e "$STD_/$STX" ] || { echo "FAIL a survivor state directory was removed"; n=$((n+1)); }
  kr "^REMOVED nasdir catalogizer-nas-ro.$GONE_PID proof=" test ! -e "$NASV"; kr "^REMOVED nasdir catalogizer-nas-ro.$$ proof=.*leg_process_gone" test ! -e "$NASR"; k "^KEPT nasdir nas-ro-$NASL_PROC_PID failed=process_alive"
  [ ! -e "$NASV" ] && [ ! -e "$NASR" ] || { echo "FAIL a victim nas directory survived (a gone pid / a recycled pid)"; n=$((n+1)); }
  [ -e "$NASK" ] || { echo "FAIL the directory of a LIVE leg process was removed"; n=$((n+1)); }
  return "$n"
}
# the dry run first: nothing is removed
res=$(battery "$SUT" --dry-run); n=$?
if [ "$n" -eq 0 ]; then ok "--dry-run announces the victim and removes nothing"; else bad "--dry-run: $(printf '%s' "$res" | tr '\n' ';' | cut -c1-300)"; fi
res=$(battery "$SUT"); n=$?
if [ "$n" -eq 0 ]; then ok "the victims are removed with their proofs, every survivor survives for its named reason"; else bad "$n violation(s): $(printf '%s' "$res" | tr '\n' ';' | cut -c1-500)"; fi
# the control needle: the sweep sees the pods (a sweep that sees nothing proves nothing): a second run finds no candidate left among the victims
o=$(bash "$TI_REPO/$SUT" 2>&1); printf '%s\n' "$o" | grep -q "pod $V " && bad "a second sweep still sees the victim" || ok "a second sweep finds the victim gone (idempotent)"
printf '%s\n' "$o" | grep -q "^KEPT pod $S3 " && ok "control needle: the sweep still sees the surviving full pod on the second run" || bad "control needle: the sweep no longer sees the surviving pods"

if [ "${SWEEP_NO_MUTATIONS:-0}" != 1 ] && [ "${SWEEP_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${SWEEP_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mut() { # mut <name> <old> <new> [<old2> <new2>]: rebuild the fixtures, run the battery against a mutated copy
    local name=$1 md="$TI_REPO/.audit/scratch/sweep-mut-$$-$1" r n; local dst="$md/sweep_leaks.sh"; shift; mkdir -p "$md"; cp "$TI_REPO/scripts/test-infra/lib.sh" "$md/"
    mkfix   # BEFORE the rewrite: the confinement below bakes the fixture paths (NASV, NASR, NASK carry the pids of THIS mkfix) into the mutant
    # every mutant is confined to THIS test's fixtures (names carry $R): a mutant that dropped a proof must never touch a pod or directory of another stream on this shared host
    python3 -I - "$TI_REPO/$SUT" "$dst" "$@" "for pod in \$(podman pod ls --format '{{.Name}}' 2>/dev/null); do" "for pod in \$(podman pod ls --format '{{.Name}}' 2>/dev/null | grep \"$R\"); do" 'for d in "$TI_ROOT"/.audit/out/catalogizer-test-*; do' "for d in \"\$TI_ROOT\"/.audit/out/catalogizer-test-$R*; do" 'for sd in "$TI_STATE_DIR"/catalogizer-test-*; do' "for sd in \"\$TI_STATE_DIR\"/catalogizer-test-$R*; do" 'for d in "${XDG_RUNTIME_DIR:-/nonexistent}"/catalogizer-nas-ro.* "$TI_ROOT"/.audit/out/nas-ro-*; do' "for d in \"$NASV\" \"$NASR\" \"$NASK\"; do" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read(); a = sys.argv[3:]
for i in range(0, len(a), 2):
    if s.count(a[i]) != 1: print("anchor count %d for %r" % (s.count(a[i]), a[i])); sys.exit(1)
    s = s.replace(a[i], a[i + 1])
open(sys.argv[2], "w").write(s)
PY
    r=$(battery ".audit/scratch/sweep-mut-$$-$name/sweep_leaks.sh"); n=$?
    if [ "${SWEEP_IDENTITY:-0}" = 1 ]; then   # an identity mutant must keep the whole battery green
      if [ "$n" -eq 0 ]; then ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$MUTLOG"; else bad "identity mutant $name FAILED the battery ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name FAILED-BUT-IDENTITY" >>"$MUTLOG"; fi
    elif [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${md:?}"
  }
  mut no_empty_proof '  { [ "$(podman pod inspect "$pod" --format '"'"'{{len .Containers}}'"'"' 2>/dev/null)" = 0 ] && [ -z "$(podman ps -a -q --filter "pod=$pod" 2>/dev/null)" ]; } || { echo "KEPT pod $pod failed=holds_a_container"; pk=$((pk+1)); continue; }' '  true'
  mut no_live_proof '  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT pod $pod failed=project_is_live($why)"; pk=$((pk+1)); continue; }' '  true'
  mut no_create_command_proof '  [ "$got" = "$want" ] || { echo "KEPT pod $pod failed=create_command_is_not_podman_compose'"'"'s"; pk=$((pk+1)); continue; }' '  true'
  mut pod_pattern_wide '  [[ "$pod" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})$ ]] || continue' "  [[ \"\$pod\" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30}|${R}other)\$ ]] || continue"
  mut dir_follows_symlink '  { [ -d "$d" ] && [ ! -L "$d" ]; } || { echo "KEPT dir $n failed=not_a_real_directory"; dk=$((dk+1)); continue; }' '  true' 'elif podman unshare rm -rf -- "${d:?}" 2>/dev/null || rm -rf -- "$d" 2>/dev/null; then' 'elif rm -rf -- "$d/" "$d" 2>/dev/null; then'
  mut dir_live_ignored '  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT dir $n failed=project_is_live($why)"; dk=$((dk+1)); continue; }' '  true'
  mut dir_logs_removed '[[ "$n" =~ ^(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})-(client|seed)$ ]] || continue' '[[ "$n" =~ ^(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})-(client|seed|logs)$ ]] || continue'
  # SW4 (state_is_not_Created) is NOT independently constructible: a pod that started a container and lost it is `Created` again (measured here), so the proof is covered by holds_a_container; recorded, not claimed
  podman pod create --name "pod_zzsw4-$R" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("pod_zzsw4-$R")
  sw4c=$(podman create --pull=never --pod "pod_zzsw4-$R" --name "zzsw4-$R-c" --entrypoint sleep "$LOCKIMG" 30 2>/dev/null); podman pod start "pod_zzsw4-$R" >/dev/null 2>&1; podman rm -f -t 0 "$sw4c" >/dev/null 2>&1
  check "SW4 note: an emptied infra-less pod is back in state Created (so 'state is not Created' has no independent victim; holds_a_container covers it)" "$(podman pod inspect "pod_zzsw4-$R" --format '{{.State}} {{len .Containers}}' 2>/dev/null)" "Created 0"
  # SW1..SW3 of the WF17 proof lens: one proof of live() removed each
  mut sw1_claim_proof_removed '  [ ! -d "$LD/claims/$1" ] || { echo "lease_claim_exists"; return; }' '  :'
  mut sw2_labelled_container_proof_removed '  [ -z "$ids" ] || { echo "labelled_container_exists"; return; }' '  :'
  mut sw3_name_is_itself_a_project_proof_removed '  if ti_valid_project "$n"; then why="$(live "$n")"; [ -z "$why" ] || { echo "KEPT dir $n failed=name_is_itself_a_live_project($why)"; dk=$((dk+1)); continue; }; fi' '  :'
  # the state and nasdir kinds: each proof load-bearing
  mut state_keep_marker_ignored '  [ ! -e "$sd/.keep-state" ] || { echo "KEPT state $n failed=kept_by_request"; sk=$((sk+1)); continue; }' '  :'
  mut state_claim_ignored '  [ ! -d "$LD/claims/$n" ] || { echo "KEPT state $n failed=lease_claim_exists"; sk=$((sk+1)); continue; }' '  :'
  mut state_container_ignored '  [ -z "$ids" ] || { echo "KEPT state $n failed=labelled_container_exists"; sk=$((sk+1)); continue; }' '  :'
  mut state_op_record_ignored '|| { echo "KEPT state $n failed=no_op_record"; sk=$((sk+1)); continue; }' '|| { :; }'
  mut state_op_liveness_ignored 'else echo "KEPT state $n failed=op_not_provably_dead(${ost:-unknown})"; sk=$((sk+1)); continue; fi;;' 'else proof=op_ignored; fi;;'
  mut nas_live_process_ignored '  if [ -n "$nst" ] && [ "$nst" != Z ] && tr '"'"'\0'"'"' '"'"' '"'"' <"/proc/$npid/cmdline" 2>/dev/null | grep -q nas_readonly_leg; then echo "KEPT nasdir $n failed=process_alive(pid=$npid)"; nk=$((nk+1)); continue; fi' '  :'
  mut nas_recycled_pid_kept '2>/dev/null | grep -q nas_readonly_leg; then echo "KEPT nasdir' '2>/dev/null | grep -q .; then echo "KEPT nasdir'
  SWEEP_IDENTITY=1 mut sw5_identity 'ti_need podman jq realpath' 'ti_need podman jq realpath; :'
  [ -z "${SWEEP_EV:-}" ] || cp "$MUTLOG" "$SWEEP_EV/sweep-mutations.txt"
fi
ti_summary
