#!/bin/sh
# identity: WP-12 PA-04 fix round 3: runs INSIDE the pinned IMG-GO container (see fix-r3-mutate.py gen). For every mutated copy under /out/mut/<id>/filesystem
# it runs the suite of the mutant's target package (no -race: a kill is a failed assertion, not a race report; GREEN runs use -race; -failfast: the first failing
# test is the kill) and records the exit code and the log next to the copy, MUT_PAR copies at a time (the tests wait more than they compute).
# usage (inside the container): sh /out/mut/run.sh [id ...]   (default: every id)
cd /out/mut || exit 2
ids="$*"
[ -n "$ids" ] || ids="$(ls -d */ | tr -d /)"
printf '%s\n' $ids | xargs -P "${MUT_PAR:-3}" -I{} sh -c '
  id={}
  [ -d "/out/mut/$id/filesystem" ] || exit 0
  target="$(cat /out/mut/$id/target)"
  ( cd "/out/mut/$id/filesystem" && env GOTOOLCHAIN=local GOFLAGS=-mod=mod GOMAXPROCS=2 CGO_ENABLED=1 go test -count=1 -failfast -timeout 170s "./$target/" ) >"/out/mut/$id.log" 2>&1
  echo $? >"/out/mut/$id.rc"
'
