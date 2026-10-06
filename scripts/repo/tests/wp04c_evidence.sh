#!/usr/bin/env bash
# WP-04 fix round 3: evidence recorder. Usage: wp04c_evidence.sh red|green <out-file>
#   red     runs test_wp04c.sh once and prints the verbatim transcript with an identity header
#   green   runs test_wp04c.sh three times (fresh throwaway trees each run), then every earlier WP-04 test once
# The identity header carries the sha256 of the test and of every helper, git HEAD, bash/git versions. Output goes ONLY to <out-file>
# (callers pass a path under specs/001-full-project-audit-remediation/evidence/wp04). A done marker `# DONE` is written last.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
mode="$1"; out="$2"
sha() { [ -f "$1" ] && sha256sum "$1" | cut -d' ' -f1 || echo ABSENT; }
{
  echo "# WP-04 fix round 3 evidence ($mode) - host run, bash ${BASH_VERSION}, $(git --version), head $(git rev-parse --short HEAD) (working tree; nothing committed by this task)"
  echo "# generated $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  for f in test_wp04c.sh lib_wp04b.sh; do echo "# sha256 scripts/repo/tests/$f $(sha scripts/repo/tests/$f)"; done
  for f in lib_safe check_no_ci check_revision_headers check_class fixture_roots scope_check validate_cheap commit_recursive push_recursive integrate_merge integrate_ff_only record_deferral; do echo "# sha256 scripts/repo/$f.sh $(sha scripts/repo/$f.sh)"; done
  for f in validate_checks.tsv check_classes.tsv check_exemptions.tsv fixture_roots.txt; do echo "# sha256 scripts/repo/$f $(sha scripts/repo/$f)"; done
  n=1; [ "$mode" = green ] && n=3
  for i in $(seq 1 $n); do echo; echo "== test_wp04c run $i"; bash scripts/repo/tests/test_wp04c.sh 2>&1; echo "exit=$?"; done
  if [ "$mode" = green ]; then
    for t in check_classes check_no_ci check_revision_headers commit_recursive fixture_roots integrate_ff_only integrate_merge push_recursive record_deferral scope_check validate_cheap; do
      echo; echo "== test_$t (regression, once)  [test sha256 $(sha scripts/repo/tests/test_$t.sh)]"; bash scripts/repo/tests/test_$t.sh 2>&1 | tail -3; echo "exit=${PIPESTATUS[0]}"
    done
  fi
  echo; echo "# DONE"
} > "$out" 2>&1
