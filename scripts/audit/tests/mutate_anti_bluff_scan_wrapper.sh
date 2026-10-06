#!/usr/bin/env bash
# Mutation test for scripts/anti-bluff-scan.sh: each mutant breaks one safeguard and test_anti_bluff_scan_wrapper.sh (run with W pointing
# at the mutant in a mirror tree) MUST then FAIL. Exit 0 = every mutant caught and the unmutated wrapper passes.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
SRC=scripts/anti-bluff-scan.sh; TEST=scripts/audit/tests/test_anti_bluff_scan_wrapper.sh
T="$(mktemp -d "${TMPDIR:-/tmp}/abs_mut.XXXXXX")"; trap 'rm -rf "$T"' EXIT
python3 - "$SRC" "$T" <<'PY' || exit 2
import sys
src, t = sys.argv[1:3]
text = open(src).read()
MUTS = [
 ("m01-no-declpath-check", 'safe_declpath "$line" || die unsafe_path', 'true || die unsafe_path'),
 ("m02-symlink-followed", 'if [ -L "$acc" ]; then die symlink_component', 'if false; then die symlink_component'),
 ("m03-empty-list-clean", '[ "$n" -gt 0 ] || die empty_list', 'true || die empty_list'),
 ("m04-always-exit-0", 'if [ "$rc" -ne 0 ] || [ -s "$RES" ]; then exit 1; fi', 'if false; then exit 1; fi'),
 ("m05-list-optionlike-allowed", 'case "$LIST" in -*) die option_like_list', 'case "$LIST" in -ZZZ*) die option_like_list', 'safe_dir_arg "$LIST" || die unsafe_list_path', 'true || die unsafe_list_path'),
 ("m06-scan-whole-tree-in-list-mode", 'SCAN_ROOT="$T/tree"; mkdir -p "$SCAN_ROOT"', 'SCAN_ROOT="$ROOT"; mkdir -p "$T/tree"'),
 ("m07-inner-rc-only", 'if [ "$rc" -ne 0 ] || [ -s "$RES" ]; then exit 1; fi', 'if [ "$rc" -ne 0 ]; then exit 1; fi'),
 ("m08-copy-skips-regular-file-check", '[ -f "$acc" ] || die not_a_regular_file', 'true || die not_a_regular_file'),
]
for name, *pairs in MUTS:   # a mutant is one or more (old, new) pairs applied together (defence-in-depth layers)
    out = text
    for old, new in zip(pairs[0::2], pairs[1::2]):
        if out.count(old) != 1:
            print("HARNESS ERROR: %s: old text occurs %d times" % (name, out.count(old))); sys.exit(3)
        out = out.replace(old, new)
    open("%s/%s.txt" % (t, name), "w").write(out)
PY
mirror() { # mirror <wrapper-copy> <tag> -> mirror tree with the real inner scanner and lib_safe.sh; echoes the wrapper path
  local d="$T/mir_$2"; mkdir -p "$d/scripts/audit" "$d/scripts/repo"
  cp "$1" "$d/scripts/anti-bluff-scan.sh"; cp scripts/audit/anti-bluff-scan.sh "$d/scripts/audit/"; cp scripts/repo/lib_safe.sh "$d/scripts/repo/"
  chmod +x "$d/scripts/anti-bluff-scan.sh"; echo "$d/scripts/anti-bluff-scan.sh"
}
BW="$(mirror "$SRC" base)"; W="$BW" bash "$TEST" >"$T/base.out" 2>&1; brc=$?
echo "baseline (unmutated, mirror tree) rc=$brc $(tail -1 "$T/base.out")"; [ "$brc" -eq 0 ] || { echo "baseline FAILS: results meaningless"; exit 2; }
SURV=0; N=0
for f in "$T"/m*.txt; do
  n="$(basename "$f" .txt)"; N=$((N+1)); mw="$(mirror "$f" "$n")"
  W="$mw" bash "$TEST" >"$T/$n.out" 2>&1; rc=$?
  if [ "$rc" -ne 0 ]; then echo "CAUGHT   $n ($(grep -c '^FAIL' "$T/$n.out") failing check(s); first: $(grep '^FAIL' "$T/$n.out" | head -1 | cut -c1-80))"
  else echo "SURVIVED $n"; SURV=$((SURV+1)); fi
done
echo "MUTATIONS total=$N survived=$SURV"; [ "$SURV" -eq 0 ]
