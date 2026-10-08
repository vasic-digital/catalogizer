#!/usr/bin/env bash
# fix-r5-inventory.sh <tree-root> [<needle-tree-root>] - class inventories of the longops / anti-mess round 5 (constitution 11.4.276 C: name the class, enumerate every member with a control-needled instrument).
# A count is a lead; the listed lines are the finding. Each instrument is run on the NEEDLE tree (the pre-fix tree 350372a8 extracted by `git archive`) and must find >= 1 member there, or the instrument is blind
# (11.4.201 (7)(b)) and the zero on the fixed tree proves nothing. Read-only; no network; no podman.
# Classes:  K = an option arm that takes a value (`--x) v=$2; shift 2`) with no guard against the flag being the last token (an endless loop or a silent empty value)
#           A = a jq default (`//`) applied to a REQUIRED op/holder field outside the one record shape (an absent field read as a value)
#           J = a per-record jq/python spawn loop over ops/*.json (a scan that costs O(records) processes)
#           H = a script that sources the library without the C locale being forced by the library
# Output: for each class and tree: COUNT and the member lines.
set -u
T=${1:?tree root}; N=${2:-}
FILES="scripts/longops/*.sh scripts/anti-mess/sweep.sh"
inv() {  # inv <root> <class>
  local root=$1 cls=$2 f
  cd "$root" || return 2
  for f in $FILES; do
    [ -f "$f" ] || continue
    case "$cls" in
      K) grep -nE '^\s*(--[a-z-]+)\)|;; *--[a-z-]+\)|--[a-z-]+\) [^;]*shift 2' "$f" | grep 'shift 2' | grep -v 'lo_need' | grep -v '\$# -ge 2' | sed "s|^|$f:|" ;;
      A) grep -nE '\.(builds|run_id|state|pid|start_time|purpose_key|op_id|boot_id)[[:space:]]*//' "$f" | grep -vE '^[0-9]+:[[:space:]]*#' | sed "s|^|$f:|" ;;
      AR) grep -nE '\.(run_id|state)[[:space:]]*// ""' "$f" | grep -vE '^[0-9]+:[[:space:]]*#' | sed "s|^|$f:|" ;;
      J) grep -nE 'for [a-z_]+ in .*ops/\*\.json|ops/\*\.json.*\| *while|for [a-z_]+ in "?\$LD/ops' "$f" | grep -vE '^[0-9]+:[[:space:]]*#' | sed "s|^|$f:|" ;;
      H) grep -qE '^export LC_ALL=C|^\s*export LC_ALL=C' "$f" || { grep -qE 'lib\.sh' "$f" && echo "$f: no LC_ALL=C in the script itself"; } ;;
    esac
  done
}
rc=0
for cls in K A J; do
  if [ -n "$N" ]; then
    n=$(inv "$N" "$cls" | wc -l)
    echo "NEEDLE tree class $cls: $n member(s)$([ "$n" -ge 1 ] || echo ' -- BLIND: the zero below proves nothing')"
    [ "$n" -ge 1 ] || rc=1
  fi
  out=$(inv "$T" "$cls"); c=$(printf '%s' "$out" | grep -c . || true)
  if [ "$cls" = A ]; then   # REVIEWED dispositions (each line read): `.run_id // ""` is compared with a NON-EMPTY expected run id (an absent field is unequal: the holder is left untouched, fail-safe) and the `.state // ""` of
    # reap.sh is read inside the grace wait AFTER step A validated the record by the shape (an unreadable record keeps waiting; step B re-derives it with the shape and refuses). They are listed, not counted.
    rev=$(inv "$T" AR); [ -z "$rev" ] || { echo "FIXED  tree class A: $(printf '%s\n' "$rev" | grep -c .) REVIEWED non-member(s) (default compared with a non-empty value / read after the shape check):"; printf '%s\n' "$rev" | sed 's/^/    reviewed: /'; }
    out=$(grep -vFx -f <(printf '%s\n' "$rev") <<<"$out" || true); c=$(printf '%s' "$out" | grep -c . || true)
  fi
  echo "FIXED  tree class $cls: $c member(s)"; [ -z "$out" ] || printf '%s\n' "$out" | sed 's/^/    /'
  [ "$c" -eq 0 ] || rc=1
done
# H: the library forces the locale for every sourcing script
cd "$T" && if grep -q '^export LC_ALL=C' scripts/longops/lib.sh; then echo "FIXED  tree class H: 0 member(s) (lib.sh exports LC_ALL=C)"; else echo "FIXED  tree class H: 1 member (lib.sh does not export LC_ALL=C)"; rc=1; fi
[ -z "$N" ] || { cd "$N" && if grep -q '^export LC_ALL=C' scripts/longops/lib.sh; then echo "NEEDLE tree class H: already forced -- BLIND"; rc=1; else echo "NEEDLE tree class H: 1 member (the pre-fix library does not force the locale)"; fi; }
exit $rc
