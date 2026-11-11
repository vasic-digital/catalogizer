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
echo "baseline (expect rc=0): rc=$rc"; [ "$rc" -eq 0 ] || bad=1
for m in cap strict netguard; do
  "$HERE/run.sh" --mutate "$m" --out "$OUT/mutant-$m" >"$OUT/mutant-$m.console.txt" 2>&1; rc=$?
  echo "mutant $m (expect rc=1): rc=$rc"; [ "$rc" -eq 1 ] || bad=1
done
( cd "$OUT" && find . -type f ! -name SHA256SUMS.txt -print0 | sort -z | xargs -0 sha256sum > SHA256SUMS.txt )
[ "$bad" -eq 0 ] && echo "MUTATION-SUITE: PASS" || echo "MUTATION-SUITE: FAIL"
exit "$bad"
