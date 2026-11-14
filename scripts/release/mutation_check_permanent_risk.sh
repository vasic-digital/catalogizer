#!/usr/bin/env bash
# Paired mutations (constitution 1.1 / 11.4.224) for the permanent-risk class of check_release_consistency.sh.
# Each mutant is the checker with ONE exact textual change; test_permanent_risk.sh must FAIL against it (killed).
# Exit 0 only if every mutant is killed.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/pmut.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/pmut.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
JOBS="${MUT_JOBS:-4}"
# name|||old|||new   (old must occur exactly once)
MUTANTS=(
'decision id check removed|||if ! [[ "$pdec" =~ ^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$ ]]; then|||if false; then'
'decision date check removed|||[ "$(date -d "$pdate" +%F 2>/dev/null)" != "$pdate" ] || [[ "$pdate" > "$today" ]]; then|||false; then'
'future decision date accepted||||| [[ "$pdate" > "$today" ]]; then|||; then'
're-affirmation note not required|||if [ -z "$pre" ]; then W_BAD|||if false; then W_BAD'
'permanent risk not printed|||echo "PERMANENT-RISK  $pkey:|||: "PERMANENT-RISK  $pkey:'
'permanent risk not counted|||PERMSEEN["$pkey"]=1; PERM_N=$((PERM_N+1))|||PERMSEEN["$pkey"]=1'
'duplicate permanent risk accepted|||if [ -n "${PERMSEEN[$pkey]+x}" ]; then|||if false; then'
'tracked item not required for permanent risk|||if ! [[ "$pitem" =~ ^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$ ]]; then|||if false; then'
'blank reason accepted for permanent risk|||if [ -z "$reason" ]; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing reason"; continue; fi
      if ! [[ "$pitem"|||if false; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing reason"; continue; fi
      if ! [[ "$pitem"'
'permanent risk treated as evidence waiver (verdict word)|||PERMSEEN["$pkey"]=1; PERM_N=$((PERM_N+1))|||PERMSEEN["$pkey"]=1; PERM_N=$((PERM_N+1)); W_APPLIED=$((W_APPLIED+1))'
'ordinary expired waiver accepted|||if [[ "$exp" < "$today" ]]; then|||if false; then'
)
run_one() {
  local n="$1" name="$2" old="$3" new="$4" d="$ROOT/m$1"
  mkdir -p "$d"
  cp "$HERE/check_release_consistency.sh" "$HERE/release_fixture.sh" "$HERE/test_permanent_risk.sh" "$d/"
  if ! python3 -I - "$d/check_release_consistency.sh" "$old" "$new" <<'PY'
import sys
p, old, new = sys.argv[1:4]
s = open(p, encoding="utf-8").read()
if s.count(old) != 1:
    sys.stderr.write(f"mutation target occurs {s.count(old)} times\n"); sys.exit(3)
open(p, "w", encoding="utf-8").write(s.replace(old, new))
PY
  then echo "ERROR  [$n] $name: mutation did not apply" > "$d/result"; return; fi
  if ! bash -n "$d/check_release_consistency.sh" 2>/dev/null; then echo "ERROR  [$n] $name: syntax error" > "$d/result"; return; fi
  if CHECKER="$d/check_release_consistency.sh" "$d/test_permanent_risk.sh" >"$d/out.txt" 2>&1; then
    echo "SURVIVED [$n] $name" > "$d/result"
  elif tail -1 "$d/out.txt" | grep -Eq '^== result: [0-9]+ passed, [1-9][0-9]* failed$'; then
    echo "killed   [$n] $name ($(tail -1 "$d/out.txt"))" > "$d/result"
  else
    echo "INVALID  [$n] $name: $(tail -1 "$d/out.txt")" > "$d/result"
  fi
}
n=0
for m in "${MUTANTS[@]}"; do
  name="${m%%|||*}"; rest="${m#*|||}"; old="${rest%%|||*}"; new="${rest#*|||}"
  n=$((n+1))
  while [ "$(jobs -rp | wc -l)" -ge "$JOBS" ]; do wait -n; done
  run_one "$n" "$name" "$old" "$new" &
done
wait
survived=0
for i in $(seq 1 "$n"); do
  line="$(cat "$ROOT/m$i/result" 2>/dev/null || echo "ERROR  [$i] no result")"
  echo "$line"
  case "$line" in killed*) ;; *) survived=$((survived+1)) ;; esac
done
echo "== mutants: $n, killed: $((n-survived)), survived/errored: $survived"
[ "$survived" -eq 0 ]
