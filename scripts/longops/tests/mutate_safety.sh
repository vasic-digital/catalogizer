#!/usr/bin/env bash
# mutate_safety.sh - paired mutations (G-GATE, 11.4.115 F / 1.1) of the containment library mutation_safety.sh itself (WF14 R2-T4 RM8; 11.4.263, 11.4.201): each mutation breaks ONE load-bearing line in a COPY of
# the library; test_mutation_safety.sh run against that copy (env MS_LIB) must FAIL. A mutant that leaves the test green survives and fails this runner. The test uses a recorder in place of the kill binary and
# signal 0 only, and the run itself sits inside the PRISTINE containment (ms_run of the real library), so a weakened library cannot signal anything.
# Round 5 (CONS LO-T3): C13/C14/C15 (reviewer mutants, verbatim) survived because the test asserted only rc 1; it now asserts the identity of the resolved command and the INNER log, so a stub the library did not
# create (the OUTER containment's stub answers further down PATH) and a podman shim that is not read-only are visible. Every run is bounded by `timeout ${MUT_TIMEOUT_S:-900}`; `TIMEOUT` is its own result.
# Usage  mutate_safety.sh [--only MS01,MS02] [--check]     Output: one line per mutation, then `MUTATION RESULT caught=N survived=M timeout=T total=T`
. "$(dirname "$0")/lib.sh"
. "$(dirname "$0")/mutation_safety.sh"
[ "$(id -u)" != 0 ] || { echo "refusing to run mutations as root"; exit 2; }
ms_prepare "$FX/shim-outer" || { echo "SAFETY: the mutant containment cannot be built; no mutant is run"; exit 2; }
ONLY=""; CHECK=${MUT_CHECK_ONLY:-0}
while [ $# -gt 0 ]; do case "$1" in --only) ONLY=,${2:-},; shift 2 ;; --check) CHECK=1; shift ;; *) echo "unknown argument $1"; exit 2 ;; esac; done
TMO=${MUT_TIMEOUT_S:-900}
caught=0; surv=0; tot=0; tmo=0
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
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return; }
  MS_LIB=$d/mutation_safety.sh MUT_FAILFAST=1 ms_run timeout "$TMO" bash "$(dirname "$0")/test_mutation_safety.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -eq 124 ]; then tmo=$((tmo+1)); echo "TIMEOUT  $id $desc (exceeded ${TMO}s: never counted as caught)"
  elif [ $rc -ne 0 ]; then caught=$((caught+1)); echo "CAUGHT   $id $desc :: $(grep -m2 '^FAIL' "$FX/mut-$id.out" | cut -c1-110 | tr '\n' '|')"
  else surv=$((surv+1)); echo "SURVIVED $id $desc"; fi
}
NC() {  # NC <id> <description> <text>: the NEGATIVE CONTROL: an UNMUTATED copy of the library (the text replaced by itself) run through the identical pipeline MUST PASS test_mutation_safety.sh
  local id=$1 desc=$2 text=$3
  [ -z "$ONLY" ] || case "$ONLY" in *",$id,"*) ;; *) return ;; esac
  tot=$((tot+1)); local d=$FX/mut-$id; rm -rf "$d"; mkdir -p "$d"
  python3 -I - "$(dirname "$0")/mutation_safety.sh" "$d/mutation_safety.sh" "$text" "$text" <<'PY' || { echo "INVALID $id $desc: pattern not found"; surv=$((surv+1)); return; }
import sys
src,dst,old,new=sys.argv[1:5]; s=open(src).read()
if old not in s: sys.exit(1)
open(dst,'w').write(s.replace(old,new,1))
PY
  [ "$CHECK" = 1 ] && { echo "APPLIES $id"; return; }
  MS_LIB=$d/mutation_safety.sh MUT_FAILFAST=1 ms_run timeout "$TMO" bash "$(dirname "$0")/test_mutation_safety.sh" >"$FX/mut-$id.out" 2>&1; local rc=$?
  if [ $rc -eq 0 ]; then echo "CONTROL-OK $id $desc (the suite PASSED on the unmutated copy)"; else surv=$((surv+1)); echo "CONTROL-BROKEN $id $desc (rc $rc): the pipeline fails an UNMUTATED library, every CAUGHT is void"; fi
}
NC CTRL0 "negative control: an unmutated copy of mutation_safety.sh placed like a mutant" 'enable -n kill 2>/dev/null'
M MS01 "RM8 (round-2 reviewer): any line containing a # is a comment, so a kill line with a trailing comment is admitted" "case \"\${line#\"\${line%%[![:space:]]*}\"}\" in '#'*) continue ;; esac" "case \"\$line\" in *'#'*) continue ;; esac"
M MS02 "a glob-built absolute path (/usr/bin/k?ll) is not recognised" ' || [[ "$line" =~ $MS_GLOB_RE ]]' ''
M MS03 "the left boundary excludes a dot again: os.killpg( and .kill( are invisible" '(^|[^A-Za-z0-9_-])(kill|' '(^|[^A-Za-z0-9_.-])(kill|'
M MS04 "the shim functions are not read-only: a mutant can redefine the guard" 'readonly -f kill pkill killall podman ms_blocked ms_kill_guard 2>/dev/null' ':'
M MS05 "the PATH stub of podman is missing: podman inside the containment is the host binary" '>"$MS_BIN/podman"; chmod +x "$MS_BIN/podman"' '>/dev/null'
M MS06 "a podman stop written with the PODMAN variable is not a side-effect token" '|PODMAN"?[[:space:]]+(stop|' '|PODMANX"?[[:space:]]+(stop|'
M MS07 "psutil is not a signal token" '|psutil|os\.kill' '|os\.kill'
M MS08 "an ms_* function name is not a containment-tampering token" '|MS_[A-Z_]+|ms_[a-z_]+|' '|MS_[A-Z_]+|'
M MS09 "a NEW line naming a bin directory (a computed path) is not refused" ' || [[ "$line" =~ $MS_PATH_RE ]]' ''
M MS10 "a NEW line with a word split by an empty quote pair is not refused" ' || [[ "$line" =~ $MS_SPLIT_RE ]]' ''
M C13 "the PATH stub for kill is not created (command kill / env kill reach the host /usr/bin/kill)" 'local n; for n in kill pkill killall skill; do' 'local n; for n in pkill killall skill; do'
M C14 "the podman shim function is not read-only (a mutant can redefine podman inside the containment)" 'readonly -f kill pkill killall podman ms_blocked ms_kill_guard 2>/dev/null' 'readonly -f kill pkill killall ms_blocked ms_kill_guard 2>/dev/null'
M C15 "the PATH stub for pkill is not created (command pkill reaches the host /usr/bin/pkill)" 'local n; for n in kill pkill killall skill; do' 'local n; for n in kill killall skill; do'
M MS11 "the stub of skill is not created" 'local n; for n in kill pkill killall skill; do' 'local n; for n in kill pkill killall; do'
M MS12 "the shim reads the process group with a line-oriented sed again (a comm holding a newline makes a live process look group-less and the kill is refused: found by the negative control)" $'pg=$({ IFS= read -r -d \'\' pst; } 2>/dev/null <"/proc/$t/stat"; pst=${pst##*) }; set -- $pst; echo "${3:-}")' $'pg=$(sed \'s/^.*) //\' "/proc/$t/stat" 2>/dev/null | cut -d\' \' -f3)'
echo "MUTATION RESULT caught=$caught survived=$surv timeout=$tmo total=$tot"
[ "$surv" -eq 0 ] && [ "$tmo" -eq 0 ]
