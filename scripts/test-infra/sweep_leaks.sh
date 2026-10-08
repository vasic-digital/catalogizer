#!/usr/bin/env bash
# sweep_leaks.sh - WF12 F2. Removes the resources that earlier test-infrastructure runs leaked and that no lifecycle script will ever find again, each ONLY after its ownership by this
# stack's per-run project namespace `catalogizer-test-<build id>` is proven. It never selects by a bare name glob and never touches anything it cannot prove is its own.
#   pods    `pod_catalogizer-test-<id>` is removed when ALL hold: (1) its creation command is exactly what podman-compose runs for a project (`podman pod create --name=<pod>
#           --infra=false --share=`: nobody else creates a pod with that exact shape and that exact name); (2) it is in state Created and holds no container (a label would be part of
#           the creation command, so (1) already excludes a labelled pod);
#           (3) its project is not live: no lease claim, no per-run state directory, no container labelled catalogizer.test_project=<project>.
#   dirs    `<repo>/.audit/out/catalogizer-test-<id>-client` and `...-seed` (what run_client.sh and a failed seeding leave) are removed when ALL hold: (1) a real directory, never a
#           symlink; (2) no project it could belong to is live (the project named by the name without the suffix, and the project the whole name could itself be); `...-logs` (the
#           deliberate --keep-logs result) is never touched.
#   states  `<state dir>/catalogizer-test-<id>` (the per-run env file with credentials, data, logs) of a project that is not live is removed when ALL hold: (1) a real directory, no `.keep-state`
#           marker (down.sh --keep-state writes it: a deliberate result stays); (2) no lease claim, no live keeper process named by a lease.keep.* file, no container of ANY checkout labelled
#           catalogizer.test_project=<project>; (3) the operation named in <state>/op_id is terminal in THIS registry or its owner is PROVEN dead (reap.sh --op-id --dry-run); a directory with no op_id
#           record is kept (nothing proves it is dead). The leaks of a crash (SIGKILL) and of the documented direct use of gen_env.sh end here (WF17 TI-A3, TI-C6).
#   nasdirs `$XDG_RUNTIME_DIR/catalogizer-nas-ro.<pid>` and `<repo>/.audit/out/nas-ro-<pid>` (the run directory of nas_readonly_leg.sh, which holds the NAS auth file) are removed when ALL hold: (1) a real
#           directory; (2) /proc/<pid> proves the leg is gone: no such process, a zombie, or a process whose command line does not name nas_readonly_leg (a recycled pid). A SIGKILLed run ends here (WF17 TI-C3).
# A resource that fails a proof is KEPT and the failed proof is printed; a podman query that FAILS is `failed=podman_query_failed` (unknown is never "nothing there", WF17 TI-B5). Usage: sweep_leaks.sh [--dry-run]
# Output: one line per candidate `REMOVED|WOULD_REMOVE|KEPT <kind> <name> <proofs or the failed proof>`, then `SWEEP pods_removed=<n> pods_kept=<n> dirs_removed=<n> dirs_kept=<n> states_removed=<n> states_kept=<n> nasdirs_removed=<n> nasdirs_kept=<n>`.
# The registry path is the one scripts/longops/lib.sh derives (LONGOPS_DIR / LONGOPS_AUDIT / LONGOPS_REPO), never re-derived here.
# Exit: 0; 1 a proven-leaked resource could not be removed; 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
DRY=0
while [ $# -gt 0 ]; do case "$1" in --dry-run) DRY=1; shift;; *) ti_die "unknown argument '$1'" 2;; esac; done
ti_need podman jq realpath
ti_ld_init; LD="$TI_LD_CACHE"
pr=0; pk=0; dr=0; dk=0; sr=0; sk=0; nr=0; nk=0; rc=0
# live <project>: prints the reason when the project is live (claim, state directory, labelled container), nothing otherwise
live() {
  [ ! -d "$LD/claims/$1" ] || { echo "lease_claim_exists"; return; }
  [ ! -e "$TI_STATE_DIR/$1" ] || { echo "state_directory_exists"; return; }
  local ids; ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$1" 2>/dev/null)" || { echo "podman_query_failed"; return; }
  [ -z "$ids" ] || { echo "labelled_container_exists"; return; }
}
# a failing podman is UNKNOWN, never an empty list: nothing is removed and the failure is the printed reason
if ! podman pod ls --format '{{.Name}}' >/dev/null 2>&1; then echo "KEPT pods * failed=podman_query_failed"; pk=$((pk+1)); rc=1; PODS_UNKNOWN=1; else PODS_UNKNOWN=0; fi
for pod in $(podman pod ls --format '{{.Name}}' 2>/dev/null); do
  [ "$PODS_UNKNOWN" = 0 ] || break
  [[ "$pod" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})$ ]] || continue
  P="${BASH_REMATCH[1]}"
  want="[\"podman\",\"pod\",\"create\",\"--name=$pod\",\"--infra=false\",\"--share=\"]"
  got="$(podman pod inspect "$pod" --format '{{json .CreateCommand}}' 2>/dev/null)"
  [ "$got" = "$want" ] || { echo "KEPT pod $pod failed=create_command_is_not_podman_compose's"; pk=$((pk+1)); continue; }
  [ "$(podman pod inspect "$pod" --format '{{.State}}' 2>/dev/null)" = Created ] || { echo "KEPT pod $pod failed=state_is_not_Created"; pk=$((pk+1)); continue; }
  { [ "$(podman pod inspect "$pod" --format '{{len .Containers}}' 2>/dev/null)" = 0 ] && [ -z "$(podman ps -a -q --filter "pod=$pod" 2>/dev/null)" ]; } || { echo "KEPT pod $pod failed=holds_a_container"; pk=$((pk+1)); continue; }
  why="$(live "$P")"; [ "$why" != podman_query_failed ] || { echo "KEPT pod $pod failed=podman_query_failed"; pk=$((pk+1)); continue; }
  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT pod $pod failed=project_is_live($why)"; pk=$((pk+1)); continue; }
  if [ "$DRY" = 1 ]; then echo "WOULD_REMOVE pod $pod proof=create_command,created_empty,project_not_live"; pr=$((pr+1))
  elif podman pod rm "$pod" >/dev/null 2>&1; then echo "REMOVED pod $pod proof=create_command,created_empty,project_not_live"; pr=$((pr+1))
  else echo "KEPT pod $pod failed=podman_pod_rm_refused"; rc=1; fi
done
for d in "$TI_ROOT"/.audit/out/catalogizer-test-*; do
  [ -e "$d" ] || [ -L "$d" ] || continue
  n="$(basename "$d")"
  [[ "$n" =~ ^(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})-(client|seed)$ ]] || continue
  P="${BASH_REMATCH[1]}"
  { [ -d "$d" ] && [ ! -L "$d" ]; } || { echo "KEPT dir $n failed=not_a_real_directory"; dk=$((dk+1)); continue; }
  why="$(live "$P")"; [ "$why" != podman_query_failed ] || { echo "KEPT dir $n failed=podman_query_failed"; dk=$((dk+1)); continue; }
  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT dir $n failed=project_is_live($why)"; dk=$((dk+1)); continue; }
  if ti_valid_project "$n"; then why="$(live "$n")"; [ -z "$why" ] || { echo "KEPT dir $n failed=name_is_itself_a_live_project($why)"; dk=$((dk+1)); continue; }; fi
  if [ "$DRY" = 1 ]; then echo "WOULD_REMOVE dir $n proof=real_directory,project_not_live"; dr=$((dr+1))
  elif podman unshare rm -rf -- "${d:?}" 2>/dev/null || rm -rf -- "$d" 2>/dev/null; then echo "REMOVED dir $n proof=real_directory,project_not_live"; dr=$((dr+1))
  else echo "KEPT dir $n failed=rm_refused"; rc=1; fi
done
for sd in "$TI_STATE_DIR"/catalogizer-test-*; do
  { [ -d "$sd" ] && [ ! -L "$sd" ]; } || continue
  n="$(basename "$sd")"; ti_valid_project "$n" || continue
  [ ! -e "$sd/.keep-state" ] || { echo "KEPT state $n failed=kept_by_request"; sk=$((sk+1)); continue; }
  [ ! -d "$LD/claims/$n" ] || { echo "KEPT state $n failed=lease_claim_exists"; sk=$((sk+1)); continue; }
  if kf="$(ti_live_keeper_in "$sd")"; then echo "KEPT state $n failed=live_keeper($(basename "$kf"))"; sk=$((sk+1)); continue; fi
  ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$n" 2>/dev/null)" || { echo "KEPT state $n failed=podman_query_failed"; sk=$((sk+1)); continue; }
  [ -z "$ids" ] || { echo "KEPT state $n failed=labelled_container_exists"; sk=$((sk+1)); continue; }
  op="$(head -1 "$sd/op_id" 2>/dev/null)"
  { [ -n "$op" ] && [[ "$op" =~ ^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}$ ]]; } || { echo "KEPT state $n failed=no_op_record"; sk=$((sk+1)); continue; }
  ost="$(jq -r '.state // ""' "$LD/ops/$op.json" 2>/dev/null)"
  case "$ost" in
    complete|failed|reaped|blocked-escape) proof="op_terminal($ost)";;
    *) if DRYR="$(ti_lo reap --op-id "$op" --dry-run 2>&1)" && printf '%s' "$DRYR" | grep -q 'would reap dead'; then proof="op_owner_proven_dead"; else echo "KEPT state $n failed=op_not_provably_dead(${ost:-unknown})"; sk=$((sk+1)); continue; fi;;
  esac
  if [ "$DRY" = 1 ]; then echo "WOULD_REMOVE state $n proof=real_directory,not_kept,no_claim,no_keeper,no_container,$proof"; sr=$((sr+1))
  elif podman unshare rm -rf -- "${sd:?}" 2>/dev/null || rm -rf -- "$sd" 2>/dev/null; then echo "REMOVED state $n proof=real_directory,not_kept,no_claim,no_keeper,no_container,$proof"; sr=$((sr+1))
  else echo "KEPT state $n failed=rm_refused"; rc=1; fi
done
for d in "${XDG_RUNTIME_DIR:-/nonexistent}"/catalogizer-nas-ro.* "$TI_ROOT"/.audit/out/nas-ro-*; do
  [ -e "$d" ] || [ -L "$d" ] || continue
  n="$(basename "$d")"; [[ "$n" =~ ^(catalogizer-nas-ro\.|nas-ro-)([0-9]+)$ ]] || continue
  npid="${BASH_REMATCH[2]}"
  { [ -d "$d" ] && [ ! -L "$d" ]; } || { echo "KEPT nasdir $n failed=not_a_real_directory"; nk=$((nk+1)); continue; }
  nst=""; [ -r "/proc/$npid/stat" ] && nst="$(sed 's/^.*) //' "/proc/$npid/stat" 2>/dev/null | cut -c1)"
  if [ -n "$nst" ] && [ "$nst" != Z ] && tr '\0' ' ' <"/proc/$npid/cmdline" 2>/dev/null | grep -q nas_readonly_leg; then echo "KEPT nasdir $n failed=process_alive(pid=$npid)"; nk=$((nk+1)); continue; fi
  if [ "$DRY" = 1 ]; then echo "WOULD_REMOVE nasdir $n proof=real_directory,leg_process_gone"; nr=$((nr+1))
  elif rm -rf -- "${d:?}" 2>/dev/null; then echo "REMOVED nasdir $n proof=real_directory,leg_process_gone"; nr=$((nr+1))
  else echo "KEPT nasdir $n failed=rm_refused"; rc=1; fi
done
echo "SWEEP pods_removed=$pr pods_kept=$pk dirs_removed=$dr dirs_kept=$dk states_removed=$sr states_kept=$sk nasdirs_removed=$nr nasdirs_kept=$nk"
exit "$rc"
