#!/usr/bin/env bash
# mutate_safety.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of the containment library mutation_safety.sh itself (WF14 R2-T4 RM8; 11.4.263, 11.4.201): each mutation breaks ONE load-bearing line in a COPY of
# the library; test_mutation_safety.sh run against that copy (env MS_LIB) must FAIL. A mutant that leaves the test green survives and fails this runner. The test uses a recorder in place of the kill binary and
# signal 0 only, and the run itself sits inside the PRISTINE containment (ms_run of the real library), so a weakened library cannot signal anything.
# Usage  mutate_safety.sh [--only MS01,MS02]     Output: one line per mutation, then `MUTATION RESULT caught=N survived=M total=T`
. "$(dirname "$0")/lib.sh"
. "$(dirname "$0")/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim-outer" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; [ "${1:-}" = --only ] && ONLY=,${2:-},
caught=0; surv=0; tot=0
M() {  # M <id> <description> <old> <new>
  local id=$1 desc=$2 old=$3 new=$4
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); local d=$FX/mut-$id; rm -rf "$d"; mkdir -p "$d"
  python3 -I - "$(dirname "$0")/mutation_safety.sh" "$d/mutation_safety.sh" "$old" "$new" <<'PY' || { echo "INVALID $id $desc: pattern not found"; surv=$((surv+1)); return; }
import sys
src,dst,old,new=sys.argv[1:5]; s=open(src).read()
if old not in s: sys.exit(1)
open(dst,'w').write(s.replace(old,new,1))
PY
  MS_LIB=$d/mutation_safety.sh ms_run bash "$(dirname "$0")/test_mutation_safety.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
M MS01 "RM8 (round-2 reviewer): any line containing a # is a comment, so a kill line with a trailing comment is admitted" "case \"\${line#\"\${line%%[![:space:]]*}\"}\" in '#'*) continue ;; esac" "case \"\$line\" in *'#'*) continue ;; esac"
M MS02 "a glob-built absolute path (/usr/bin/k?ll) is not recognised" ' || [[ "$line" =~ $MS_GLOB_RE ]]' ''
M MS03 "the left boundary excludes a dot again: os.killpg( and .kill( are invisible" '(^|[^A-Za-z0-9_-])(kill|' '(^|[^A-Za-z0-9_.-])(kill|'
M MS04 "the shim functions are not read-only: a mutant can redefine the guard" 'readonly -f kill pkill killall podman ms_blocked ms_kill_guard 2>/dev/null' ':'
M MS05 "the PATH stub of podman is missing: podman inside the containment is the host binary" '>"$MS_BIN/podman"; chmod +x "$MS_BIN/podman"' '>/dev/null'
M MS06 "a podman stop written with the PODMAN variable is not a side-effect token" '|PODMAN"?[[:space:]]+(stop|' '|PODMANX"?[[:space:]]+(stop|'
M MS07 "psutil is not a signal token" '|psutil|os\.kill' '|os\.kill'
M MS08 "an ms_* function name is not a containment-tampering token" '|MS_[A-Z_]+|ms_[a-z_]+|' '|MS_[A-Z_]+|'
echo "MUTATION RESULT caught=$caught survived=$surv total=$tot"
[ "$surv" -eq 0 ]
