#!/usr/bin/env bash
# run_mutations.sh - T062/T063 mutation runner: for each mutant DDL of mutate_ddl.py run the tests; a mutant is
# KILLED when a test exits non-zero for a reason that is not a bare schema-object COUNT (WF2 review B-1/I-7).
# A control run of every test on the unmutated DDL must PASS first (a test that fails on the original kills
# every mutant for the wrong reason).
# Usage: run_mutations.sh --test <test.sh> [--test <test2.sh> ...] --out <tsv> [--jobs N] [--ddl <file>]
#          [--kinds trigger,check,clause] [--equivalent <tsv>] [--count-checks <ERE>]
#   --kinds       restrict the mutant kinds (default: all)
#   --equivalent  reviewed-equivalent list, columns: kind<TAB>name<TAB>detail-prefix<TAB>reason<TAB>reviewed-by.
#                 A surviving mutant that matches a row is EQUIV, not a survivor. A row matching no mutant, or
#                 matching a mutant that a test KILLS, is a stale entry and an error (exit 4).
#   --count-checks an ERE matched against the FAIL lines of a test: a mutant whose only failing lines match it
#                 is COUNT_ONLY (a tautology kill) and counts as NOT killed by that test.
# Output columns: file, kind, name, killed(yes|NO|EQUIV|COUNT_ONLY), killing test, first non-count failing check
# or the equivalence reason. <out>.summary is written by this script (never by hand):
#   mutants=N killed=K equivalent_reviewed=E survived_unreviewed=U  (K+E+U = N; COUNT_ONLY rows are in U)
# Exit 0 only when U = 0.
set -u
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
TESTS=(); OUT=""; JOBS=4; DDL="$ROOT/scripts/register/register_ext.sql"; KINDS=""; EQUIV=""; COUNTRE=""
while [ $# -gt 0 ]; do case "$1" in
  --test|--out|--jobs|--ddl|--kinds|--equivalent|--count-checks)
     [ $# -ge 2 ] || { echo "run_mutations: $1 needs a value" >&2; exit 2; }
     case "$1" in --test) TESTS+=("$2");; --out) OUT=$2;; --jobs) JOBS=$2;; --ddl) DDL=$2;; --kinds) KINDS=$2;; --equivalent) EQUIV=$2;; --count-checks) COUNTRE=$2;; esac; shift 2;;
  *) echo "run_mutations: unknown argument $1" >&2; exit 2;; esac; done
[ ${#TESTS[@]} -gt 0 ] && [ -n "$OUT" ] || { echo "usage: run_mutations.sh --test T [--test T2] --out O [--kinds k,k] [--equivalent F] [--count-checks RE]" >&2; exit 2; }
for t in "${TESTS[@]}"; do [ -r "$t" ] || { echo "run_mutations: test not readable: $t" >&2; exit 2; }; done
[ -z "$EQUIV" ] || [ -r "$EQUIV" ] || { echo "run_mutations: equivalence list not readable: $EQUIV" >&2; exit 2; }
W=$(mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX"); trap 'rm -rf "$W"' EXIT
python3 "$ROOT/scripts/register/mutate_ddl.py" "$DDL" "$W/m" >"$W/gen.out" || { cat "$W/gen.out" >&2; exit 2; }
mkdir -p "$W/s"
ti=0; for t in "${TESTS[@]}"; do ti=$((ti+1))
  if ! REG_SCRATCH="$W/s" REG_EXT_SQL="$DDL" bash "$t" >"$W/control.$ti.txt" 2>&1; then
    echo "control run of $t on the unmutated DDL FAILED: mutation results would be meaningless" >&2; tail -5 "$W/control.$ti.txt" >&2; exit 3; fi
done
export W DDL COUNTRE; export TESTS_JOINED=$(printf '%s\n' "${TESTS[@]}")
one() { f=$1; b=$(basename "$f"); mkdir -p "$W/s/$b"; killed=NO; killer=""; first=""; ti=0
  while IFS= read -r t; do ti=$((ti+1))
    REG_SCRATCH="$W/s/$b" REG_EXT_SQL="$f" bash "$t" >"$W/s/$b.$ti.out" 2>&1; rc=$?
    [ $rc -eq 0 ] && continue
    if [ -n "$COUNTRE" ]; then nonc=$(grep '^FAIL' "$W/s/$b.$ti.out" | grep -Ecv -- "$COUNTRE"); else nonc=$(grep -c '^FAIL' "$W/s/$b.$ti.out"); fi
    if [ "$nonc" -gt 0 ] || ! grep -q '^FAIL' "$W/s/$b.$ti.out"; then   # a non-zero exit with no FAIL line (crash) is a kill too
      killed=yes; killer=$(basename "$t")
      if [ -n "$COUNTRE" ]; then first=$(grep '^FAIL' "$W/s/$b.$ti.out" | grep -Ev -- "$COUNTRE" | head -1 | cut -c1-110); else first=$(grep -m1 '^FAIL' "$W/s/$b.$ti.out" | cut -c1-110); fi
      break
    else killed=COUNT_ONLY; killer=$(basename "$t"); first=$(grep -m1 '^FAIL' "$W/s/$b.$ti.out" | cut -c1-110); fi
  done <<<"$TESTS_JOINED"
  printf '%s\t%s\t%s\t%s\n' "$b" "$killed" "$killer" "$first" > "$W/s/$b.res"; }
export -f one
if [ -n "$KINDS" ]; then awk -F'\t' -v k=",$KINDS," 'index(k,","$2",")>0' "$W/m/manifest.tsv" > "$W/sel.tsv"; else cp "$W/m/manifest.tsv" "$W/sel.tsv"; fi
cut -f1 "$W/sel.tsv" | while IFS= read -r f; do printf '%s\0' "$W/m/$f"; done | xargs -0 -P "$JOBS" -n1 bash -c 'one "$1"' _
{ echo "# identity: task=T062/T063 utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null)"
  echo "# ddl=$DDL ddl_sha256=$(sha256sum "$DDL" | cut -d' ' -f1) runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1) mutator_sha256=$(sha256sum "$ROOT/scripts/register/mutate_ddl.py" | cut -d' ' -f1)"
  for t in "${TESTS[@]}"; do echo "# test=$t test_sha256=$(sha256sum "$t" | cut -d' ' -f1) control=PASS"; done
  echo "# kinds=${KINDS:-all} count_checks=${COUNTRE:-none} equivalent_list=${EQUIV:-none}${EQUIV:+ equivalent_sha256=$(sha256sum "$EQUIV" | cut -d' ' -f1)}"
  echo "# sqlite3=$(sqlite3 --version 2>&1 | cut -d' ' -f1,2); columns: mutant<TAB>kind<TAB>name<TAB>killed<TAB>killing test<TAB>first non-count failing check | equivalence reason"
  while IFS=$'\t' read -r f kind name detail; do r=$(cat "$W/s/$f.res"); st=$(printf '%s' "$r" | cut -f2); killer=$(printf '%s' "$r" | cut -f3); first=$(printf '%s' "$r" | cut -f4)
    if [ "$st" != yes ] && [ -n "$EQUIV" ]; then
      eq=$(awk -F'\t' -v k="$kind" -v n="$name" -v d="$detail" '!/^#/ && $1==k && $2==n && ($3=="" || index(d,$3)==1) {print $4; exit}' "$EQUIV")
      [ -n "$eq" ] && { st=EQUIV; first=$eq; }
    fi
    printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$f" "$kind" "$name" "$st" "$killer" "$first"; done < "$W/sel.tsv"; } > "$OUT"
# stale equivalence entries: a row that matches no mutant of the selected kinds, or whose mutant a test kills
stale=0
if [ -n "$EQUIV" ]; then
  # tab-separated rows are split with cut, not `read`: IFS whitespace collapses the empty detail-prefix field
  while IFS= read -r line; do
    case "$line" in ''|'#'*) continue;; esac
    kind=$(printf '%s' "$line" | cut -f1); name=$(printf '%s' "$line" | cut -f2); prefix=$(printf '%s' "$line" | cut -f3)
    [ -z "$KINDS" ] || case ",$KINDS," in *",$kind,"*) ;; *) continue;; esac   # a row of a kind this run did not select is not stale
    if ! awk -F'\t' -v k="$kind" -v n="$name" -v p="$prefix" '$2==k && $3==n && (p=="" || index($4,p)==1) {f=1} END{exit !f}' "$W/m/manifest.tsv"; then echo "STALE equivalence entry (no such mutant): $kind $name" >&2; stale=1; continue; fi
    # the mutants this row names (kind, name, detail prefix), looked up by file in the result table
    awk -F'\t' -v k="$kind" -v n="$name" -v p="$prefix" 'NR==FNR{ if ($2==k && $3==n && (p=="" || index($4,p)==1)) m[$1]=1; next } !/^#/ && ($1 in m) && $4=="yes" {f=1} END{exit !f}' "$W/m/manifest.tsv" "$OUT" && { echo "STALE equivalence entry (a test kills it): $kind $name" >&2; stale=1; }
  done < "$EQUIV"
fi
total=$(grep -vc '^#' "$OUT"); kill=$(grep -v '^#' "$OUT" | awk -F'\t' '$4=="yes"' | wc -l); eqn=$(grep -v '^#' "$OUT" | awk -F'\t' '$4=="EQUIV"' | wc -l)
surv=$(grep -v '^#' "$OUT" | awk -F'\t' '$4=="NO" || $4=="COUNT_ONLY"' | wc -l); conly=$(grep -v '^#' "$OUT" | awk -F'\t' '$4=="COUNT_ONLY"' | wc -l)
[ $((kill+eqn+surv)) -eq "$total" ] || { echo "run_mutations: internal inconsistency killed+equivalent+survived != total" >&2; exit 5; }
echo "mutants=$total killed=$kill equivalent_reviewed=$eqn survived_unreviewed=$surv count_only_in_survived=$conly" | tee "$OUT.summary"
[ "$stale" -eq 0 ] || exit 4
[ "$surv" -eq 0 ]
