#!/bin/bash
# usage: mutant_run.sh <phase: before|after> ; run from the repo root of the scratch clone.
# CLFU (FU3-R-M2): the mutator is found next to this script, the original is kept in a mktemp file (no /tmp literal,
# no unset variable under set -u) and is restored on exit even if a run is interrupted.
set -u
PHASE=${1:?usage: mutant_run.sh <before|after>}
HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
OUT=docs/qa/fu3-20261117/wprog3_mutants_$PHASE.txt; : > "$OUT"
H=catalog-api/handlers/media_handler.go
ORIG=$(mktemp) || exit 2
cp "$H" "$ORIG"
trap 'cp "$ORIG" "$H"; rm -f "$ORIG"' EXIT
RUN='^TestWatchProgress|^TestRV2'
for m in R1-null-guard-dropped R2-unit-guard-dropped R3b-value-guard-le; do
  cp "$ORIG" "$H"; python3 "$HERE/mutate_wprog3.py" "$H" "$m" >>"$OUT"
  # -skip excludes the text-shape test to show behaviour-only killing
  (cd catalog-api && GOMAXPROCS=2 nice -n 10 go test -p 1 -timeout 60m -count=1 ./handlers -run "$RUN" -skip 'GuardedUpdatePostgresRewrite|PG_' 2>&1 | grep -E '^(--- FAIL|ok|FAIL)' ) >>"$OUT"
  echo "== $m done" >>"$OUT"
done
