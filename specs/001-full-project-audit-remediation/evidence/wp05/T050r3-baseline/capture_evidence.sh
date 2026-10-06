#!/usr/bin/env bash
# T050 evidence capture: RED (tool absent, scratch tree), GREEN x3, paired mutations, each file headed with an
# identity block (sha256 of the scripts and tests, git HEAD, host, UTC time). Output: $EV/wp05/T050-*.txt.
# Host bash/python3 are used because RUNP IMG-KCOV (scripts/containers/run_pinned.sh) does not exist yet.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
EVD=${EVD:-$root/specs/001-full-project-audit-remediation/evidence/wp05}; mkdir -p "$EVD"
T=$root/tools/evidence
hdr() { # hdr TITLE
  echo "# $1"; echo "# generated_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname) kernel=$(uname -r) python=$(python3 --version 2>&1) bash=${BASH_VERSION}"
  echo "# git_head=$(git -C "$root" rev-parse HEAD 2>/dev/null) (tree under test is uncommitted; the sha256 lines identify it)"
  for f in "$T/evrec" "$T/verify" "$T/evcore.py" "$T/tests/test_evrec.sh" "$T/tests/test_evrec_more.sh" "$T/tests/run_mutations.py" \
           "$root/specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json"; do
    [ -f "$f" ] && echo "# sha256 $(sha256sum "$f" | cut -d' ' -f1)  ${f#$root/}" || echo "# sha256 ABSENT  ${f#$root/}"; done
}
red() { # RED: both tests copied into a scratch tree that holds NO tool (the state before T050)
  S=$(mktemp -d); mkdir -p "$S/tools/evidence/tests"; cp "$T/tests/test_evrec.sh" "$T/tests/test_evrec_more.sh" "$S/tools/evidence/tests/"
  { hdr "T050 RED: tests run against an absent tools/evidence/evrec and verify (scratch tree $S, removed after)"
    for t in test_evrec.sh test_evrec_more.sh; do echo "## $t"; bash "$S/tools/evidence/tests/$t" 2>&1 | sed "s#$S#<scratch>#g"; echo "rc=${PIPESTATUS[0]}"; done; } >"$EVD/T050-red.txt"
  rm -rf "$S"
}
green() {
  { hdr "T050 GREEN x3: tests run against the implemented tools"
    for i in 1 2 3; do for t in test_evrec.sh test_evrec_more.sh; do echo "## run $i $t"; bash "$T/tests/$t" 2>&1; echo "rc=$?"; done; done; } >"$EVD/T050-green.txt"
}
mut() { { hdr "T050 paired mutations (run_mutations.py): every mutant must be CAUGHT by a check that names its cause"
          python3 "$T/tests/run_mutations.py" -j "${MUT_JOBS:-3}"; echo "rc=$?"; } >"$EVD/T050-mutations.txt" 2>&1; }
case "${1:-all}" in red) red;; green) green;; mut) mut;; all) red; green; mut;; esac
