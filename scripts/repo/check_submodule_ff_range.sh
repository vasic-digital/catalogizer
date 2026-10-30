#!/bin/bash -p
# check_submodule_ff_range.sh - CPA stage S3 row `submodule_ff` (constitution 11.4.234, 11.4.113, 11.4.288(D)): runs the read-only gate scripts/check_submodule_ff.sh over the range
# about to be pushed, for every distinct remote main tip, and maps its verdicts onto the S3 exit-code contract. Documented in docs/scripts/check_submodule_ff_range.md.
#
# Usage   check_submodule_ff_range.sh --root <repository>
# Range   <tip>..HEAD for each distinct sha of refs/remotes/*/main and of the current branch's @{upstream}. Nothing is fetched here (S1 fetched; the gate never fetches).
#         The range is HEAD at S3, i.e. the local commits already made; commits S5 will make later are not in it (docs/scripts/check_submodule_ff_range.md, Limits).
# Exits   0 every range FF / nothing to check (PASS); 10 at least one own-org gitlink moved NON-fast-forward (NONFF): S3 `fail`, the push does not happen until the pin is repaired;
#         14 the gate could not decide (BLIND rc 2, usage/own-org-list rc 64, any other rc, timeout, gate or tip not found): a recorded deferral (validate_cheap.sh turns a changeset
#         exit 14 into `deferred`, CPA records CHECKS_DEFERRED, exit 14, flag in every commit), never silent and never a block of the commit/push mechanism (11.4.234(D));
#         20 usage error of this wrapper only.
# Never   writes, stages, fetches or pushes; the gate it calls reads only. Bounded: the whole check stops after BUDGET seconds (default 100, below validate_cheap's 120 s kill) and
#         then reports 14 (SUBFF_RANGE_BUDGET is clamped to at most 100), so a long first push cannot become a refusal 20.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
die() { echo "check_submodule_ff_range: $1: $2" >&2; exit 20; }
ROOT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || die usage "--root needs a value"; ROOT="$2"; shift 2 ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac
done
[ -n "$ROOT" ] || die usage "--root is required"
case "$ROOT" in -*) die unsafe_root "$(printf '%q' "$ROOT")" ;; esac
[ -d "$ROOT" ] && git -C "$ROOT" rev-parse --show-toplevel >/dev/null 2>&1 || die not_a_repository "$(printf '%q' "$ROOT")"
BUDGET="${SUBFF_RANGE_BUDGET:-100}"; case "$BUDGET" in ''|*[!0-9]*) die usage "SUBFF_RANGE_BUDGET must be whole seconds" ;; esac
[ "${#BUDGET}" -le 6 ] || BUDGET=100; BUDGET=$((10#$BUDGET)); [ "$BUDGET" -le 100 ] || BUDGET=100   # clamp: never above validate_cheap's 120 s kill (a kill is a refusal 20)
GATE="$D/../check_submodule_ff.sh"
defer() { echo "BLIND submodule_ff_range: $1 (deferred, not a pass; run scripts/check_submodule_ff.sh by hand)"; exit 14; }
[ -x "$GATE" ] || defer "gate scripts/check_submodule_ff.sh is not in the code root"
git -C "$ROOT" rev-parse --verify -q 'HEAD^{commit}' >/dev/null || defer "HEAD is not a commit"
tips="$( { git -C "$ROOT" for-each-ref --format='%(objectname)' 'refs/remotes/*/main'
           git -C "$ROOT" rev-parse -q --verify '@{upstream}^{commit}' 2>/dev/null; } | sort -u)"
[ -n "$tips" ] || defer "no remote main tip and no upstream to compare HEAD with"
START=$SECONDS; nonff=""; blind=""; n=0; decided=0
while IFS= read -r tip; do
  [ -n "$tip" ] || continue
  n=$((n+1)); left=$((BUDGET - (SECONDS - START)))
  [ "$left" -gt 0 ] || { blind="$blind budget-exhausted-before:${tip:0:12}"; continue; }
  out="$(cd "$ROOT" && timeout "$left" "$GATE" "$tip" HEAD 2>&1)"; rc=$?
  case "$rc" in
    0) decided=$((decided+1)) ;;
    1) lines="$(printf '%s\n' "$out" | grep -E '^NONFF own ' | sed "s/^/tip ${tip:0:12}: /")"
       # fail closed (11.4.201(4)): gate exit 1 with no parsable NONFF line is still a refusal, never a pass
       [ -n "$lines" ] || lines="tip ${tip:0:12}: gate exit 1 (VERDICT FAIL) but no 'NONFF own' line parsed; output: $(printf '%s\n' "$out" | grep -E 'VERDICT|ERROR|rror' | head -1 | tr '\t\n' '  ')"
       nonff="$nonff"$'\n'"$lines"; decided=$((decided+1)) ;;
    124) blind="$blind timeout:${tip:0:12}" ;;
    *) blind="$blind rc$rc:${tip:0:12}:$(printf '%s\n' "$out" | grep -E 'VERDICT|ERROR' | head -1 | tr '\t\n' '  ')" ;;
  esac
done <<< "$tips"
if [ -n "${nonff//[$'\n ']/}" ]; then
  echo "NONFF own-org gitlink moved backward or sideways in the push range; repair the pin (point it at a descendant of the old pin or merge the submodule main) before pushing, never force-push:"
  printf '%s\n' "$nonff" | sed '/^$/d'
  exit 10
fi
[ -z "$blind" ] || defer "gate could not decide:$blind"
[ "$decided" -eq "$n" ] || defer "only $decided of $n remote tips were decided"   # fail closed: exit 0 only when every tip was judged
exit 0
