#!/bin/bash
# usage: mutant_run.sh <phase: before|after> ; run from repo root of the scratch clone
set -u
PHASE=$1; OUT=docs/qa/fu3-20261117/wprog3_mutants_$PHASE.txt; : > $OUT
H=catalog-api/handlers/media_handler.go
cp $H /tmp/.media_handler.orig 2>/dev/null || cp $H "$TMPDIR_FU3/media_handler.orig"
ORIG=$(mktemp); cp $H $ORIG
RUN='^TestWatchProgress|^TestRV2'
for m in R1-null-guard-dropped R2-unit-guard-dropped R3b-value-guard-le; do
  cp $ORIG $H; python3 mutate_wprog3.py $H $m >>$OUT
  # -skip excludes the text-shape test to show behaviour-only killing
  (cd catalog-api && GOMAXPROCS=2 nice -n 10 go test -p 1 -timeout 60m -count=1 ./handlers -run "$RUN" -skip 'GuardedUpdatePostgresRewrite|PG_' 2>&1 | grep -E '^(--- FAIL|ok|FAIL)' ) >>$OUT
  echo "== $m done" >>$OUT
done
cp $ORIG $H
