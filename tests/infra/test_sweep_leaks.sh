#!/usr/bin/env bash
# test_sweep_leaks.sh - WF12 F2 (RED first). Oracle for scripts/test-infra/sweep_leaks.sh: a leaked pod or output directory of this stack's per-run project namespace is removed ONLY after its
# ownership is proven, and everything that fails a proof survives. Real rootless podman (empty pods and one created-not-started container), real directories; no stack is started.
# Oracle strategy (11.4.245): SPECIFIED. The fixtures are the proofs of the script header, each with a survivor and a victim:
#   victim   an empty, unlabelled pod created exactly as podman-compose creates it (`--name=pod_<project> --infra=false --share=`) of a project that is not live
#   survivors a pod of another shape (different create command), a labelled pod, a pod that holds a container, a pod of a LIVE project (per-run state directory present), a pod whose name
#            does not match the namespace, a directory that is a symlink, `-logs`, a directory of a live project, a directory whose name does not match
# Paired mutations: the sweep copy with ONE proof removed or one pattern widened; the same fixtures must FAIL.
# Usage: test_sweep_leaks.sh   (SWEEP_NO_MUTATIONS=1: tests only)   Env: SWEEP_SUT (repo-relative script path)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SUT="${SWEEP_SUT:-scripts/test-infra/sweep_leaks.sh}"
export TI_ROOT="$TI_REPO"
if [ ! -f "$TI_REPO/$SUT" ]; then bad "script absent: $SUT"; ti_summary; exit 1; fi
LOCKIMG=$(python3 -I -c "import yaml;d=yaml.safe_load(open('$TI_REPO/build/containers/images.lock.yaml'));e=[i for i in d['images'] if i['id']=='IMG-INFRA-REDIS'][0];print(e['reference']+'@'+e['digest'])")
OUTD="$TI_REPO/.audit/out"; STD="$TI_REPO/.audit/test-infra"
R="zzsw$RANDOM"
# fixtures (rebuilt from scratch before every battery: a mutant that removed a survivor must not hide the next mutant) ----------------------------------------------------------
mkpod() { podman pod create "--name=$1" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("$1"); }
mkd() { mkdir -p "$1"; echo x >"$1/f"; TI_FOREIGN_DIRS+=("$1"); }
V="pod_catalogizer-test-${R}v"; S1="pod_catalogizer-test-${R}shape"; S2="pod_catalogizer-test-${R}label"; S3="pod_catalogizer-test-${R}full"; S4="pod_catalogizer-test-${R}live"; S5="pod_${R}other"
DV1="$OUTD/catalogizer-test-${R}d-client"; DV2="$OUTD/catalogizer-test-${R}d-seed"; DS1="$OUTD/catalogizer-test-${R}d-logs"; DS2="$OUTD/catalogizer-test-${R}live-client"; DS3="$OUTD/${R}-client"
TGT="$TI_SCRATCH/symtarget"; DS4="$OUTD/catalogizer-test-${R}sym-client"
mkfix() {
  podman rm -f "catalogizer-test-${R}full-c" >/dev/null 2>&1; podman pod rm -f "$V" "$S1" "$S2" "$S3" "$S4" "$S5" >/dev/null 2>&1
  rm -rf -- "$DV1" "$DV2" "$DS1" "$DS2" "$DS3" "$DS4" "$TGT" "$STD/catalogizer-test-${R}live"
  mkpod "$V"                                                                                       # the victim
  podman pod create --name "$S1" --infra=false --share= >/dev/null 2>&1; TI_FOREIGN_PODS+=("$S1")  # same name shape, other create command (`--name x`)
  podman pod create "--name=$S2" --infra=false --share= --label keep=me >/dev/null 2>&1; TI_FOREIGN_PODS+=("$S2")
  mkpod "$S3"; podman create --pull=never --pod "$S3" --name "catalogizer-test-${R}full-c" --entrypoint sleep "$LOCKIMG" 60 >/dev/null 2>&1
  mkpod "$S4"; mkdir -p "$STD/catalogizer-test-${R}live"; TI_FOREIGN_DIRS+=("$STD/catalogizer-test-${R}live")
  mkpod "$S5"
  mkd "$DV1"; mkd "$DV2"; mkd "$DS1"; mkd "$DS2"; mkd "$DS3"
  mkdir -p "$TGT"; echo x >"$TGT/f"; ln -s "$TGT" "$DS4"; TI_FOREIGN_DIRS+=("$DS4")
}
mkfix
state() { local v=""; for p in "$V" "$S1" "$S2" "$S3" "$S4" "$S5"; do podman pod exists "$p" 2>/dev/null && v="$v${p##*_}:present " || v="$v${p##*_}:gone "; done
  for d in "$DV1" "$DV2" "$DS1" "$DS2" "$DS3"; do [ -e "$d" ] && v="$v$(basename "$d"):present " || v="$v$(basename "$d"):gone "; done
  [ -L "$DS4" ] && [ -d "$TGT" ] && v="${v}symlink:present target:present" || v="${v}symlink:GONE-OR-TARGET-GONE"; echo "$v"; }
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
  printf '%s\n' "$o" | grep -q "^REMOVED pod $V proof=" || { echo "FAIL the removal of the victim pod is not announced with its proofs"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -q "^KEPT pod $S3 failed=holds_a_container" || { echo "FAIL the pod that holds a container is not KEPT for that reason"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -q "^KEPT pod $S4 failed=project_is_live(state_directory_exists)" || { echo "FAIL the pod of the live project is not KEPT for that reason"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -q "^KEPT dir catalogizer-test-${R}sym-client failed=not_a_real_directory" || { echo "FAIL the symlink is not KEPT for that reason"; n=$((n+1)); }
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
    # every mutant is confined to THIS test's fixtures (names carry $R): a mutant that dropped a proof must never touch a pod or directory of another stream on this shared host
    python3 -I - "$TI_REPO/$SUT" "$dst" "$@" "for pod in \$(podman pod ls --format '{{.Name}}' 2>/dev/null); do" "for pod in \$(podman pod ls --format '{{.Name}}' 2>/dev/null | grep \"$R\"); do" 'for d in "$TI_ROOT"/.audit/out/catalogizer-test-*; do' "for d in \"\$TI_ROOT\"/.audit/out/catalogizer-test-$R*; do" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
s = open(sys.argv[1]).read(); a = sys.argv[3:]
for i in range(0, len(a), 2):
    if s.count(a[i]) != 1: print("anchor count %d for %r" % (s.count(a[i]), a[i])); sys.exit(1)
    s = s.replace(a[i], a[i + 1])
open(sys.argv[2], "w").write(s)
PY
    mkfix
    r=$(battery ".audit/scratch/sweep-mut-$$-$name/sweep_leaks.sh"); n=$?
    if [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${md:?}"
  }
  mut no_empty_proof '  { [ "$(podman pod inspect "$pod" --format '"'"'{{len .Containers}}'"'"' 2>/dev/null)" = 0 ] && [ -z "$(podman ps -a -q --filter "pod=$pod" 2>/dev/null)" ]; } || { echo "KEPT pod $pod failed=holds_a_container"; pk=$((pk+1)); continue; }' '  true'
  mut no_live_proof '  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT pod $pod failed=project_is_live($why)"; pk=$((pk+1)); continue; }' '  true'
  mut no_create_command_proof '  [ "$got" = "$want" ] || { echo "KEPT pod $pod failed=create_command_is_not_podman_compose'"'"'s"; pk=$((pk+1)); continue; }' '  true'
  mut pod_pattern_wide '  [[ "$pod" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})$ ]] || continue' "  [[ \"\$pod\" =~ ^pod_(catalogizer-test-[a-z0-9][a-z0-9-]{0,30}|${R}other)\$ ]] || continue"
  mut dir_follows_symlink '  { [ -d "$d" ] && [ ! -L "$d" ]; } || { echo "KEPT dir $n failed=not_a_real_directory"; dk=$((dk+1)); continue; }' '  true' 'elif podman unshare rm -rf -- "${d:?}" 2>/dev/null || rm -rf -- "$d" 2>/dev/null; then' 'elif rm -rf -- "$d/" "$d" 2>/dev/null; then'
  mut dir_live_ignored '  why="$(live "$P")"; [ -z "$why" ] || { echo "KEPT dir $n failed=project_is_live($why)"; dk=$((dk+1)); continue; }' '  true'
  mut dir_logs_removed '[[ "$n" =~ ^(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})-(client|seed)$ ]] || continue' '[[ "$n" =~ ^(catalogizer-test-[a-z0-9][a-z0-9-]{0,30})-(client|seed|logs)$ ]] || continue'
  [ -z "${SWEEP_EV:-}" ] || cp "$MUTLOG" "$SWEEP_EV/sweep-mutations.txt"
fi
ti_summary
