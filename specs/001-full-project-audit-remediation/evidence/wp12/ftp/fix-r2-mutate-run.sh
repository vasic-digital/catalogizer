#!/bin/sh
# identity: WP-12 PA-04 fix round 2: runs INSIDE the pinned IMG-GO container (see fix-r2-mutate.py gen). For every mutated copy under /out/mut/<id>/filesystem
# it runs the suite of the mutant's target package (no -race: a kill is a failed assertion, not a race report; GREEN runs use -race) and records the exit
# code and the log next to the copy. usage (inside the container): sh /out/mut/run.sh [id ...]   (default: every id)
cd /out/mut || exit 2
ids="$*"
[ -n "$ids" ] || ids="$(ls -d */ | tr -d /)"
for id in $ids; do
  [ -d "/out/mut/$id/filesystem" ] || continue
  target="$(cat /out/mut/$id/target)"
  ( cd "/out/mut/$id/filesystem" && env GOTOOLCHAIN=local GOFLAGS=-mod=mod GOMAXPROCS=2 CGO_ENABLED=1 go test -count=1 -timeout 170s "./$target/" ) >"/out/mut/$id.log" 2>&1
  echo $? >"/out/mut/$id.rc"
done
