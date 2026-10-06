#!/usr/bin/env bash
# T017 test (TDD): scripts/audit/derive_scope.sh classifies every submodule at every depth as own or third_party.
#
# Purpose   Build a fixture repository whose .gitmodules tree nests third-party code under an own-org module (and an
#           own-org-looking module under a third-party one), run derive_scope.sh on it and assert the exact TSV.
# Usage     RUNP IMG-TESTUTIL -- bash scripts/audit/tests/test_derive_scope.sh   (git builds the fixture)
#           Env: DS=<script under test> (default scripts/audit/derive_scope.sh)
# Review fixes (WF-REVIEW-wp02-wp03): DM2 alt-class inheritance under a pending-account parent, DM3 the relative-URL guard,
#           DM4 fail-closed on an unreadable .gitmodules, I8 an uninitialised submodule is reported not-descended.
# Contract  derive_scope.sh --root DIR --out FILE [--own-orgs FILE] [--pending-orgs FILE]
#           TSV columns (tab separated, sorted by path, no header): path, class (own|third_party), URL, alt_class, flag.
#           class counts accounts of the pending file (ODG-15) as third_party; alt_class counts them as own; flag is
#           ODG-15 on a row where the two differ, else "-". A nested module under a third_party parent is third_party
#           whatever its own URL says (docs/02 4.3, constitution 11.4.79(6)). Org match is case-insensitive.
#           Exit 3 on a URL that cannot be classified (fail closed).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
DS="${DS:-scripts/audit/derive_scope.sh}"; case "$DS" in /*) ;; *) DS="$(pwd)/$DS" ;; esac
OWN="$(pwd)/scripts/audit/own_orgs.txt"; PEND="$(pwd)/scripts/audit/own_orgs_pending.txt"
T="$(mktemp -d "${TMPDIR:-/tmp}/ds_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }

mk() {  # mk <dir> <name> <path> <url> : append one submodule block to <dir>/.gitmodules
  mkdir -p "$1"; git config -f "$1/.gitmodules" "submodule.$2.path" "$3"; git config -f "$1/.gitmodules" "submodule.$2.url" "$4"
}
F="$T/fx"; mkdir -p "$F"; git -C "$F" init -q . 2>/dev/null
mk "$F" own_a  submodules/own_a   https://github.com/VASIC-DIGITAL/own_a.git
mk "$F" third_b submodules/third_b git@github.com:Someone/third_b.git
mk "$F" pend   submodules/pend    https://github.com/milos85vasic/pend.git
mk "$F" hd1    submodules/hd1     ssh://git@github.com/helixdevelopment1/hd1
mk "$F/submodules/own_a" nested  vendor/nested   https://gitlab.com/evil/nested.git
mk "$F/submodules/own_a" inner   libs/inner      git@github.com:HelixDevelopment/inner.git
mk "$F/submodules/own_a/libs/inner" deep z        https://github.com/x/y.git
mk "$F/submodules/third_b" trick deep/trick       https://github.com/vasic-digital/trick.git
mk "$F/submodules/pend" pn vendor/x https://github.com/vasic-digital/x.git       # DM2: an own-org module under a pending-account parent
mk "$F/submodules/pend" py vendor/y https://github.com/other/y.git
mkdir -p "$F/submodules/pend/vendor/x" "$F/submodules/pend/vendor/y" "$F/submodules/hd1" "$F/submodules/own_a/vendor/nested" "$F/submodules/third_b/deep/trick" "$F/submodules/own_a/libs/inner/z"
# every checked-out submodule has a .git entry (a file, as git writes it); a directory without one is an UNINITIALISED submodule (I8)
for d in submodules/pend submodules/hd1 submodules/own_a submodules/third_b submodules/own_a/libs/inner submodules/own_a/vendor/nested submodules/own_a/libs/inner/z \
         submodules/third_b/deep/trick submodules/pend/vendor/x submodules/pend/vendor/y; do mkdir -p "$F/$d"; : > "$F/$d/.git"; done

want="$T/want.tsv"
printf '%s\t%s\t%s\t%s\t%s\n' \
  submodules/hd1 third_party ssh://git@github.com/helixdevelopment1/hd1 own ODG-15 \
  submodules/own_a own https://github.com/VASIC-DIGITAL/own_a.git own - \
  submodules/own_a/libs/inner own git@github.com:HelixDevelopment/inner.git own - \
  submodules/own_a/libs/inner/z third_party https://github.com/x/y.git third_party - \
  submodules/own_a/vendor/nested third_party https://gitlab.com/evil/nested.git third_party - \
  submodules/pend third_party https://github.com/milos85vasic/pend.git own ODG-15 \
  submodules/pend/vendor/x third_party https://github.com/vasic-digital/x.git own ODG-15 \
  submodules/pend/vendor/y third_party https://github.com/other/y.git third_party - \
  submodules/third_b third_party git@github.com:Someone/third_b.git third_party - \
  submodules/third_b/deep/trick third_party https://github.com/vasic-digital/trick.git third_party - > "$want"

[ -x "$DS" ] || echo "NOTE: $DS is absent or not executable (RED state)"
"$DS" --root "$F" --out "$T/o1.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >"$T/o1.log" 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "fixture run exits 0" || bad "fixture run: want rc=0, got rc=$rc"
if [ -f "$T/o1.tsv" ] && cmp -s "$T/o1.tsv" "$want"; then ok "TSV equals the expected 10 rows (every depth, nesting rules, pending flags)"
else bad "TSV differs from expected"; [ -f "$T/o1.tsv" ] && diff "$want" "$T/o1.tsv" | head -20; fi
"$DS" --root "$F" --out "$T/o2.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >/dev/null 2>&1
if [ -f "$T/o1.tsv" ] && [ -f "$T/o2.tsv" ] && cmp -s "$T/o1.tsv" "$T/o2.tsv"; then ok "second run byte-identical (deterministic)"; else bad "second run differs or absent"; fi
# fail closed: an unparsable URL (relative) makes the run exit 3 and write no output
mk "$F" rel submodules/rel ../rel.git; mkdir -p "$F/submodules/rel"
rm -f "$T/o3.tsv"; "$DS" --root "$F" --out "$T/o3.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 3 ] && [ ! -s "$T/o3.tsv" ]; then ok "unclassifiable URL exits 3 and writes nothing"; else bad "unclassifiable URL: want rc=3 and no output, got rc=$rc"; fi
# DM3: a relative URL that the regex alone would accept ("org" = vasic-digital) is still unclassifiable -> exit 3
F2="$T/fx2"; mkdir -p "$F2"; git -C "$F2" init -q . 2>/dev/null; mk "$F2" r submodules/r ../../vasic-digital/rel2.git; mkdir -p "$F2/submodules/r"
rm -f "$T/o4.tsv"; "$DS" --root "$F2" --out "$T/o4.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >/dev/null 2>&1; rc=$?
if [ "$rc" -eq 3 ] && [ ! -s "$T/o4.tsv" ]; then ok "relative URL ../../org/repo.git exits 3 and writes nothing (DM3)"; else bad "relative URL guard: want rc=3 and no output, got rc=$rc"; fi
# DM4: an unreadable .gitmodules (git config -f fails with 128) is fail-closed, never an empty module list
F3="$T/fx3"; mkdir -p "$F3"; git -C "$F3" init -q . 2>/dev/null; printf '[submodule "x"\n\tpath = broken\n' > "$F3/.gitmodules"
rm -f "$T/o5.tsv"; "$DS" --root "$F3" --out "$T/o5.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >/dev/null 2>"$T/o5.err"; rc=$?
if [ "$rc" -eq 3 ] && [ ! -s "$T/o5.tsv" ] && grep -q unreadable "$T/o5.err"; then ok "an unreadable .gitmodules exits 3 with a message and writes nothing (DM4)"; else bad "unreadable .gitmodules: want rc=3, no output, 'unreadable' on stderr; got rc=$rc"; fi
# I8: a declared submodule whose directory is empty (uninitialised) is listed AND reported not-descended on stderr
F4="$T/fx4"; mkdir -p "$F4/submodules/own_a"; git -C "$F4" init -q . 2>/dev/null; mk "$F4" own_a submodules/own_a https://github.com/vasic-digital/own_a.git
rm -f "$T/o6.tsv"; "$DS" --root "$F4" --out "$T/o6.tsv" --own-orgs "$OWN" --pending-orgs "$PEND" >/dev/null 2>"$T/o6.err"; rc=$?
if [ "$rc" -eq 0 ] && grep -qx 'not-descended submodules/own_a' "$T/o6.err" && [ "$(wc -l < "$T/o6.tsv")" -eq 1 ]; then ok "an uninitialised (empty directory) submodule is reported not-descended (I8)"; else bad "uninitialised submodule: want rc=0 and a not-descended line, got rc=$rc: $(cat "$T/o6.err")"; fi
# m-g: an unreadable --pending-orgs (or --own-orgs) file is fail-closed rc=3 with a message, never a traceback with rc=1
if [ "$(id -u)" != 0 ]; then
  printf 'someorg\n' > "$T/pend_unr.txt"; chmod 000 "$T/pend_unr.txt"
  rm -f "$T/o7.tsv"; "$DS" --root "$F4" --out "$T/o7.tsv" --own-orgs "$OWN" --pending-orgs "$T/pend_unr.txt" >/dev/null 2>"$T/o7.err"; rc=$?
  if [ "$rc" -eq 3 ] && [ ! -s "$T/o7.tsv" ] && ! grep -q Traceback "$T/o7.err" && [ -s "$T/o7.err" ]; then ok "an unreadable --pending-orgs file exits 3 with a message and no traceback (m-g)"
  else bad "unreadable pending file: want rc=3, no output, no traceback; got rc=$rc: $(head -c 200 "$T/o7.err")"; fi
  chmod 600 "$T/pend_unr.txt"
else ok "m-g unreadable pending file: skipped (running as root cannot make a file unreadable)"; fi
# negative control of the instrument: the expected file must not equal a TSV where a nested row is mis-classified own
sed 's#submodules/third_b/deep/trick\tthird_party#submodules/third_b/deep/trick\town#' "$want" > "$T/wrong.tsv"
if cmp -s "$T/wrong.tsv" "$want"; then bad "control needle: mis-classified copy equals expected (comparison is blind)"; else ok "control needle: a mis-classified copy is detected as different"; fi
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) script=$(sha256sum "$DS" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d" " -f3) python=$(python3 -V 2>&1 | cut -d" " -f2) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
