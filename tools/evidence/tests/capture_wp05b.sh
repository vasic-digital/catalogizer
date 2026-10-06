#!/usr/bin/env bash
# T051..T056 evidence capture (WP-05 evidence-tooling slice). Output goes ONLY under $EVD (default
# specs/001-full-project-audit-remediation/evidence/wp05). Each file starts with an identity block (sha256 of every tool, test and
# fixture file named in IDFILES, git HEAD, host, UTC time) and ends with a `# DONE` marker line.
#   capture_wp05b.sh red NAME TEST...        run the named tests as they are NOW (before the tool exists) into $EVD/NAME-red.txt style file given by OUT
#   capture_wp05b.sh green OUT TEST...       run every test x3 into OUT
#   capture_wp05b.sh run OUT -- cmd...       run one command (container legs) into OUT
# The caller names OUT explicitly (OUT is a file name under $EVD). Host bash/python3 unless a command prefix is given (RUNPFX).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
F=specs/001-full-project-audit-remediation; EVD=${EVD:-$root/$F/evidence/wp05}; mkdir -p "$EVD"
T=$root/tools/evidence
hdr() { # hdr TITLE
  echo "# $1"; echo "# generated_utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname) kernel=$(uname -r) python=$(python3 --version 2>&1) bash=${BASH_VERSION}"
  echo "# git_head=$(git -C "$root" rev-parse HEAD 2>/dev/null) (tree under test is uncommitted; the sha256 lines identify it)"
  local f; for f in $(cd "$root" && ls tools/evidence/evrec tools/evidence/verify tools/evidence/verdict tools/evidence/evcore.py tools/evidence/evparse.py tools/evidence/evverdict.py tools/evidence/evanchor.py \
        tools/evidence/wrap-*.sh tools/evidence/lib/*.sh tools/evidence/tests/*.sh tools/evidence/tests/*.py tools/evidence/tests/verdict_cases/* "$F/contracts/evidence-record.schema.json" 2>/dev/null); do
    echo "# sha256 $(sha256sum "$root/$f" | cut -d' ' -f1)  $f"; done
  echo "# sha256-tree fixtures $( (cd "$T/tests/fixtures" 2>/dev/null && find . -type f | LC_ALL=C sort | xargs sha256sum | sha256sum | cut -d' ' -f1) )  tools/evidence/tests/fixtures"
}
runtests() { # runtests LABEL TEST...
  local lab=$1 t; shift
  for t in "$@"; do echo "## $lab $t"; ( cd "$root" && ${RUNPFX:-} bash "tools/evidence/tests/$t" 2>&1 ); echo "rc=$?"; done
}
redtree() { # redtree OUT TEST... : the CURRENT tests against the BASELINE tools (git HEAD versions of evrec, verify, evcore.py; none of the new files), in a scratch tree
  local out=$1 base=${BASELINE_REV:-HEAD} S; shift; S=$(mktemp -d)
  mkdir -p "$S/tools/evidence/tests" "$S/$F/contracts" "$S/scripts/repo" "$S/scripts/testing/full_automation"
  for f in evrec verify evcore.py; do git -C "$root" show "$base:tools/evidence/$f" >"$S/tools/evidence/$f" 2>/dev/null; done; chmod +x "$S/tools/evidence/evrec" "$S/tools/evidence/verify"
  local x; for x in ${REDTREE_EXTRA:-}; do cp "$T/$x" "$S/tools/evidence/$x"; done      # instruments that are NOT under test (e.g. the census tool for T052) may be carried over
  cp -r "$T/tests/." "$S/tools/evidence/tests/"; cp "$root/$F/contracts/evidence-record.schema.json" "$S/$F/contracts/"; cp "$root/scripts/repo/check_classes.tsv" "$S/scripts/repo/"
  for f in $(git -C "$root" ls-tree --name-only "$base" scripts/testing/full_automation/); do git -C "$root" show "$base:$f" >"$S/$f"; chmod +x "$S/$f"; done
  { hdr "RED: the CURRENT tests against the BASELINE tools ($base: evcore.py sha256 $(sha256sum "$S/tools/evidence/evcore.py" | cut -d' ' -f1), no new tool file present${REDTREE_EXTRA:+ except the instrument(s) under test-independent use: $REDTREE_EXTRA}), scratch tree removed after; every FAIL line is the observed failure"
    local t; for t in "$@"; do echo "## RED $t"; ( cd "$S" && bash "tools/evidence/tests/$t" >"$S/.out" 2>&1 ); local rc=$?; sed "s#$S#<tree>#g" "$S/.out"; echo "rc=$rc"; done; echo "# DONE"; } >"$EVD/$out" 2>&1
  rm -rf "$S"
}
case "${1:-}" in
  redtree) out=$2; shift 2; redtree "$out" "$@" ;;
  red)   out=$2; shift 2; { hdr "RED: these tests run BEFORE the tool they test exists (or against the previous tree where stated in the file name); every FAIL line is the observed failure"; runtests RED "$@"; echo "# DONE"; } >"$EVD/$out" 2>&1 ;;
  green) out=$2; shift 2; { hdr "GREEN x3: the tests against the current tools"; for i in 1 2 3; do runtests "run $i" "$@"; done; echo "# DONE"; } >"$EVD/$out" 2>&1 ;;
  run)   out=$2; shift 3; { hdr "one command: $*"; ( cd "$root" && "$@" 2>&1 ); echo "rc=$?"; echo "# DONE"; } >"$EVD/$out" 2>&1 ;;
  *) echo "usage: capture_wp05b.sh red|green OUT TEST... | run OUT -- cmd..." >&2; exit 64 ;;
esac
