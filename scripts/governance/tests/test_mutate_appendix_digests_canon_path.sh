#!/usr/bin/env bash
# test_mutate_appendix_digests_canon_path.sh: mutate_appendix_digests.py must read the canon from `constitution/Constitution.md`
# (the real submodule path; the old `submodules/constitution/` location was moved to the project root by 772dd4c3 (2026-10-11) and this script's reference was missed by that move) in a mirror tree built from the real files.
# NOTE: the control (unmutated appendix) may still FAIL on appendix-vs-canon gaps (measured at base 0bdcf378: 2 GAPs for 11.4.191, 12 NO-DIGEST); that is a
# different defect (appendix lags the pinned canon) and is not asserted here.
# Legs: (1) the mirror with canon at constitution/ -> control green, exit 0; (2) the mirror WITHOUT canon -> exit non-zero and the
# error names constitution/Constitution.md (a tool that passed here would have read some other file).
# Env: CANON_SRC (default <repo>/constitution/Constitution.md). Missing canon source = SKIP (exit 0 with a SKIP line, never a PASS).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
CANON_SRC="${CANON_SRC:-$REPO/constitution/Constitution.md}"
P=0; F=0
ok(){ P=$((P+1)); echo "PASS: $1"; }; bad(){ F=$((F+1)); echo "FAIL: $1"; }
[ -f "$CANON_SRC" ] || { echo "SKIP: canon source $CANON_SRC absent (submodule not initialised)"; echo "Summary: PASS=0 FAIL=0 SKIP=1"; exit 0; }
T="$(mktemp -d "${TMPDIR:-/tmp}/mutcanon.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
mkdir -p "$T/m/scripts/governance/tests" "$T/m/.specify/memory"
cp "$REPO"/scripts/governance/*.py "$REPO"/scripts/governance/*.tsv "$T/m/scripts/governance/" 2>/dev/null
cp "$HERE/mutate_appendix_digests.py" "$T/m/scripts/governance/tests/"
cp "$REPO/.specify/memory/constitution-appendix.md" "$T/m/.specify/memory/"
run(){ nice -n 15 python3 -I "$T/m/scripts/governance/tests/mutate_appendix_digests.py" >"$T/out" 2>&1; echo $?; }
# leg 2 first: no canon in the mirror -> must refuse, naming the real path
rc="$(run)"
[ "$rc" != 0 ] && grep -q 'cannot read .*/m/constitution/Constitution.md' "$T/out" && ok "no canon in the mirror: refused (rc $rc) naming constitution/Constitution.md" || bad "no canon: rc $rc, output: $(head -c 300 "$T/out")"
# leg 1: canon at constitution/
mkdir -p "$T/m/constitution" && cp "$CANON_SRC" "$T/m/constitution/Constitution.md"
rc="$(run)"
! grep -q 'cannot read' "$T/out" && grep -q 'canon_blocks=[1-9]' "$T/out" && ok "canon at constitution/Constitution.md: the tool read and parsed it (canon_blocks>0; control verdict rc $rc is the appendix-vs-canon state, not the path)" || bad "canon present but not parsed: rc $rc, output: $(head -c 400 "$T/out")"
echo "Summary: PASS=$P FAIL=$F SKIP=0"; [ "$F" = 0 ]
