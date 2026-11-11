#!/usr/bin/env bash
# Paired mutation runner: the unmutated tree must PASS (control) and every mutant
# (applied via `go test -overlay` on a scratch copy of one source file; the working tree is
# never modified) must FAIL. Exit 0 only if all expectations hold.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${1:-$HERE/results/mutation-$(date -u +%Y%m%dT%H%M%SZ)}"
mkdir -p "$OUT" || exit 2
bad=0
"$HERE/run.sh" --out "$OUT/baseline" >"$OUT/baseline.console.txt" 2>&1; rc=$?
echo "baseline (expect rc=0): rc=$rc" | tee -a "$OUT/summary.txt"; [ "$rc" -eq 0 ] || bad=1
for m in cap strict netguard m1 m2 m5 m7 m8 n1; do
  "$HERE/run.sh" --mutate "$m" --out "$OUT/mutant-$m" >"$OUT/mutant-$m.console.txt" 2>&1; rc=$?
  # KILLED only if the tests actually ran and an assertion failed: a compile failure (rc=2 BLIND) is not a kill
  if [ "$rc" -eq 1 ] && grep -q '^\s*--- FAIL: TestLiveCoverArt' "$OUT/mutant-$m/go_test.txt" && ! grep -q 'build failed' "$OUT/mutant-$m/go_test.txt"; then
    echo "mutant $m: KILLED (rc=1, assertion FAIL seen)" | tee -a "$OUT/summary.txt"
  else echo "mutant $m: SURVIVED or INVALID (rc=$rc)" | tee -a "$OUT/summary.txt"; bad=1; fi
done
( cd "$OUT" && find . -type f ! -name SHA256SUMS.txt -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS.txt )
{ [ "$bad" -eq 0 ] && echo "MUTATION-SUITE: PASS" || echo "MUTATION-SUITE: FAIL"; } | tee -a "$OUT/summary.txt"
exit "$bad"
