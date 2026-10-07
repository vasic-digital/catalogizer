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
# A resource that fails a proof is KEPT and the failed proof is printed. Usage: sweep_leaks.sh [--dry-run]
# Output: one line per candidate `REMOVED|WOULD_REMOVE|KEPT <kind> <name> <proofs or the failed proof>`, then `SWEEP pods_removed=<n> pods_kept=<n> dirs_removed=<n> dirs_kept=<n>`.
# Exit: 0; 1 a proven-leaked resource could not be removed; 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
DRY=0
while [ $# -gt 0 ]; do case "$1" in --dry-run) DRY=1; shift;; *) ti_die "unknown argument '$1'" 2;; esac; done
ti_need podman jq
LD="${LONGOPS_DIR:-$TI_ROOT/.audit/longops}"
pr=0; pk=0; dr=0; dk=0; rc=0
# live <project>: prints the reason when the project is live (claim, state directory, labelled container), nothing otherwise
live() {
  [ ! -d "$LD/claims/$1" ] || { echo "lease_claim_exists"; return; }
  [ ! -e "$TI_STATE_DIR/$1" ] || { echo "state_directory_exists"; return; }
  [ -z "$(podman ps -a -q --filter "label=catalogizer.test_project=$1" 2>/dev/null)" ] || { echo "labelled_container_exists"; return; }
}
for pod in $(podman pod ls --format '{{.Name}}' 2>/dev/null); do
  [[ "$pod" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})$ ]] || continue
  P="${BASH_REMATCH[1]}"
  want="[\"podman\",\"pod\",\"create\",\"--name=$pod\",\"--infra=false\",\"--share=\"]"
  got="$(podman pod inspect "$pod" --format '{{json .CreateCommand}}' 2>/dev/null)"
  [ "$got" = "$want" ] || { echo "KEPT pod $pod failed=create_command_is_not_podman_compose's"; pk=$((pk+1)); continue; }
  [ "$(podman pod inspect "$pod" --format '{{.State}}' 2>/dev/null)" = Created ] || { echo "KEPT pod $pod failed=state_is_not_Created"; pk=$((pk+1)); continue; }
  { [ "$(podman pod inspect "$pod" --format '{{len .Containers}}' 2>/dev/null)" = 0 ] && [ -z "$(podman ps -a -q --filter "pod=$pod" 2>/dev/null)" ]; } || { echo "KEPT pod $pod failed=holds_a_container"; pk=$((pk+1)); continue; }
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
  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT dir $n failed=project_is_live($why)"; dk=$((dk+1)); continue; }
  if ti_valid_project "$n"; then why="$(live "$n")"; [ -z "$why" ] || { echo "KEPT dir $n failed=name_is_itself_a_live_project($why)"; dk=$((dk+1)); continue; }; fi
  if [ "$DRY" = 1 ]; then echo "WOULD_REMOVE dir $n proof=real_directory,project_not_live"; dr=$((dr+1))
  elif podman unshare rm -rf -- "${d:?}" 2>/dev/null || rm -rf -- "$d" 2>/dev/null; then echo "REMOVED dir $n proof=real_directory,project_not_live"; dr=$((dr+1))
  else echo "KEPT dir $n failed=rm_refused"; rc=1; fi
done
echo "SWEEP pods_removed=$pr pods_kept=$pk dirs_removed=$dr dirs_kept=$dk"
exit "$rc"
