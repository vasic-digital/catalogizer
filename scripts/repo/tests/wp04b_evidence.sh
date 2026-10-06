#!/usr/bin/env bash
# WP-04 second slice: evidence recorder. Usage: wp04b_evidence.sh red|green|mut <out-file>
#   red      runs every second-slice test once (use while the helpers are absent) and prints the verbatim transcript with an identity header
#   redfinal runs every FINAL test once with H pointing at a path that does not exist (the helpers exist on disk but are not given to the test),
#            which is the RED state of the final test versions
#   green runs each test three times (fresh throwaway trees each run)
# The identity header of every section carries the sha256 of the test and of the helper (or ABSENT), git HEAD, bash/git versions.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
mode="$1"; shift
PAIRS="integrate_merge:test_integrate_merge check_no_ci:test_check_no_ci check_revision_headers:test_check_revision_headers scope_check:test_scope_check validate_cheap:test_validate_cheap commit_recursive:test_commit_recursive push_recursive:test_push_recursive"
sha() { [ -f "$1" ] && sha256sum "$1" | cut -d' ' -f1 || echo ABSENT; }
echo "# WP-04 second slice evidence ($mode) - host run, bash ${BASH_VERSION}, $(git --version), head $(git rev-parse --short HEAD) (working tree; nothing committed by this task)"
echo "# generated $(date -u +%Y-%m-%dT%H:%M:%SZ)"
n=1; [ "$mode" = green ] && n=3
for p in $PAIRS; do
  h="scripts/repo/${p%%:*}.sh"; t="scripts/repo/tests/${p##*:}.sh"
  for i in $(seq 1 $n); do
    if [ "$mode" = redfinal ]; then
      echo; echo "== ${p##*:} run $i  [H forced to /nonexistent/${h##*/} (the helper $h exists, sha256 $(sha "$h"), but is not given to the test); test $t sha256 $(sha "$t"); lib sha256 $(sha scripts/repo/lib_safe.sh)]"
      H="/nonexistent/${h##*/}" bash "$t" 2>&1; echo "exit=$?"
    else
      echo; echo "== ${p##*:} run $i  [helper $h sha256 $(sha "$h"); test $t sha256 $(sha "$t"); lib sha256 $(sha scripts/repo/lib_safe.sh)]"
      bash "$t" 2>&1; echo "exit=$?"
    fi
  done
done
