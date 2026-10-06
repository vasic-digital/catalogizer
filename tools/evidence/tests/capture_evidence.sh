#!/usr/bin/env bash
# T050 evidence capture (round 3). Output goes ONLY under $EVD (default specs/001-full-project-audit-remediation/evidence/wp05),
# never a repo-root evidence/ directory. Each file starts with an identity block (sha256 of every tool, test and schema file,
# git HEAD, host, UTC time) and ends with a `# DONE` marker line, so a reader can tell a finished run from a cut-off one.
#   red    the round-3 tests against the ROUND-2 tool tree (evidence/wp05/T050${ROUND}-baseline, sha256-pinned there)
#   green  every test file x3 against the current tree
#   mut    run_mutations.py (author mutants + the reviewer's 21 ported mutants)
#   t048a  the ev/1 contract test x3 (stale record of review minor 12)
# Host bash/python3 are used because RUNP IMG-KCOV (scripts/containers/run_pinned.sh) does not exist yet.
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
F=specs/001-full-project-audit-remediation
ROUND=${ROUND:-r7}
EVD=${EVD:-$root/$F/evidence/wp05}; mkdir -p "$EVD"
T=$root/tools/evidence
TESTS="test_evrec.sh test_evrec_more.sh test_evrec_r3.sh test_evrec_r4.sh test_evrec_r5.sh test_evrec_r6.sh test_evrec_r7.sh test_evrec_golden.sh test_evrec_hermetic.sh"
hdr() { # hdr TITLE
  echo "# $1"; echo "# generated_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname) kernel=$(uname -r) python=$(python3 --version 2>&1) bash=${BASH_VERSION}"
  echo "# git_head=$(git -C "$root" rev-parse HEAD 2>/dev/null) (tree under test is uncommitted; the sha256 lines identify it)"
  for f in "$T/evrec" "$T/verify" "$T/evcore.py" "$T/tests/hermetic.sh" "$T/tests/test_evcore_golden.py" "$T/tests/run_mutations.py" $(for t in $TESTS; do echo "$T/tests/$t"; done) \
           "$T/tests/test_evidence_record_schema.sh" "$root/$F/contracts/evidence-record.schema.json"; do
    [ -f "$f" ] && echo "# sha256 $(sha256sum "$f" | cut -d' ' -f1)  ${f#$root/}" || echo "# sha256 ABSENT  ${f#$root/}"; done
}
runtests() { # runtests DIR LABEL   (DIR = a tree holding tools/evidence/tests)
  local d=$1 t
  for t in $TESTS; do echo "## $2 $t"; bash "$d/tools/evidence/tests/$t" 2>&1 | sed "s#$d#<tree>#g"; echo "rc=${PIPESTATUS[0]}"; done
}
red() { # RED: the round-3 tests against the round-2 tools (baseline copy), in a scratch tree
  local S; S=$(mktemp -d); mkdir -p "$S/tools/evidence/tests" "$S/$F/contracts" "$S/scripts/repo"
  cp "$EVD/T050${ROUND}-baseline/evrec" "$EVD/T050${ROUND}-baseline/verify" "$EVD/T050${ROUND}-baseline/evcore.py" "$S/tools/evidence/"; chmod +x "$S/tools/evidence/evrec" "$S/tools/evidence/verify"
  cp "$T/tests/"*.sh "$T/tests/"*.py "$S/tools/evidence/tests/"
  cp "$root/$F/contracts/evidence-record.schema.json" "$S/$F/contracts/"; cp "$root/scripts/repo/check_classes.tsv" "$S/scripts/repo/"
  { hdr "T050 round ${ROUND#r} RED: the current tests run against the PREVIOUS-round tools (evidence/wp05/T050${ROUND}-baseline: evcore.py sha256 $(sha256sum "$EVD/T050${ROUND}-baseline/evcore.py" | cut -d' ' -f1)), scratch tree removed after"
    runtests "$S" "RED"; echo "# DONE"; } >"$EVD/T050${ROUND}-red.txt" 2>&1
  rm -rf "$S"
}
green() {
  { hdr "T050 round ${ROUND#r} GREEN x3: every test file against the current tools"
    for i in 1 2 3; do runtests "$root" "run $i"; done; echo "# DONE"; } >"$EVD/T050${ROUND}-green.txt" 2>&1
}
mut() { { hdr "T050 round ${ROUND#r} paired mutations (run_mutations.py): every selected mutant must be CAUGHT by a check that names its cause (MUT_IDS limits the sweep to the new and affected mutants)"
          python3 "$T/tests/run_mutations.py" -j "${MUT_JOBS:-4}" ${MUT_IDS:-}; echo "rc=$?"; echo "# DONE"; } >"$EVD/T050${ROUND}-mutations.txt" 2>&1; }
t048a() { { hdr "T048a re-capture (review minor 12): test_evidence_record_schema.sh x3 against the current contract (revision 7)"
            for i in 1 2 3; do echo "## run $i"; bash "$T/tests/test_evidence_record_schema.sh" 2>&1; echo "rc=$?"; done; echo "# DONE"; } >"$EVD/T048a-green-r3.txt" 2>&1; }
container() { # every test file once inside the pinned test image (scripts/containers/run_pinned.sh IMG-TESTUTIL, repo at /src)
  { hdr "T050 round ${ROUND#r} CONTAINER run: every test file once inside IMG-TESTUTIL via scripts/containers/run_pinned.sh --network=none"
    local t; for t in $TESTS test_evidence_record_schema.sh; do echo "## container $t"
      ( cd "$root" && timeout 1800 scripts/containers/run_pinned.sh --network=none IMG-TESTUTIL -- bash "tools/evidence/tests/$t" 2>&1 ); echo "rc=$?"; done
    echo "# DONE"; } >"$EVD/T050${ROUND}-container.txt" 2>&1; }
case "${1:-all}" in red) red;; green) green;; mut) mut;; t048a) t048a;; container) container;; all) red; green; mut; t048a; container;; esac
