#!/usr/bin/env bash
# T018 test (TDD): scripts/audit/scope_to_lumen_json.py derives the gen_lumenignore.py input JSON from scope.yaml.
#
# Purpose   Assert (1) a fixture scope gives a golden JSON byte-for-byte (sorted, no timestamp), (2) an empty allow-list is
#           refused, (3) a class present in scope.yaml but missing from the JSON FAILs the --check mode, (4) the JSON is
#           accepted by submodules/constitution/scripts/lumen/gen_lumenignore.py and yields a deterministic .lumenignore.
# Usage     RUNP IMG-TESTUTIL -- bash scripts/audit/tests/test_scope_to_lumen_json.sh   (python3 + PyYAML + jq)
#           Env: STJ=<script under test> (default scripts/audit/scope_to_lumen_json.py)
# Review fixes (WF-REVIEW I7, m6): --check is an exact re-derive-and-compare (allow, deny, root_files, classes, dropped_negations);
#           a malformed scope.yaml is exit 3, not a traceback.
# Contract  scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --out FILE     (derive)
#           scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --check FILE   (exit 1 on a missing class)
#           allow = own rows of the submodules TSV (derive_scope.sh output) + scope.yaml key `lumen_allow_roots`;
#           deny  = baseline_excludes + project_excludes + pathological_excludes (non-anchored patterns rewritten to
#                   `**/<pattern>`, negations dropped and listed under `dropped_negations`, because gen_lumenignore.py
#                   would mangle them) + every third_party row nested under an allowed root;
#           classes = {class name: [deny patterns]} so --check can prove no class was lost. Exit 3 on an empty allow.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
STJ="${STJ:-scripts/audit/scope_to_lumen_json.py}"
GL=submodules/constitution/scripts/lumen/gen_lumenignore.py
T="$(mktemp -d "${TMPDIR:-/tmp}/stj_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
cat > "$T/scope.yaml" <<'Y'
schema_version: 1
baseline_excludes:
  build_outputs: ["/out/", "/dist/"]
  caches: ["**/node_modules/", "**/.cache/"]
  secrets: [".env", "*secret*", "!*secret*/", "**/secrets/"]
  qa_corpora: ["**/qa-results/"]
project_excludes: ["/Website/public/"]
pathological_excludes: []
lumen_allow_roots: [catalog-api, catalog-web]
Y
printf '%s\n' \
 "submodules/own_a	own	https://github.com/vasic-digital/own_a.git	own	-" \
 "submodules/own_a/vendor/nested	third_party	https://gitlab.com/evil/nested.git	third_party	-" \
 "submodules/third_b	third_party	git@github.com:Someone/third_b.git	third_party	-" > "$T/subs.tsv"
cat > "$T/golden.json" <<'J'
{
  "allow": [
    "catalog-api",
    "catalog-web",
    "submodules/own_a"
  ],
  "classes": {
    "build_outputs": [
      "/dist/",
      "/out/"
    ],
    "caches": [
      "**/.cache/",
      "**/node_modules/"
    ],
    "project_excludes": [
      "/Website/public/"
    ],
    "qa_corpora": [
      "**/qa-results/"
    ],
    "secrets": [
      "**/*secret*",
      "**/.env",
      "**/secrets/"
    ],
    "third_party_nested": [
      "/submodules/own_a/vendor/nested/"
    ]
  },
  "deny": [
    "**/*secret*",
    "**/.cache/",
    "**/.env",
    "**/node_modules/",
    "**/qa-results/",
    "**/secrets/",
    "/Website/public/",
    "/dist/",
    "/out/",
    "/submodules/own_a/vendor/nested/"
  ],
  "dropped_negations": [
    "!*secret*/"
  ],
  "root_files": true
}
J
[ -f "$STJ" ] || echo "NOTE: $STJ is absent (RED state)"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/o1.json" >"$T/o1.log" 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "derive exits 0" || bad "derive: want rc=0, got rc=$rc"
if [ -f "$T/o1.json" ] && cmp -s "$T/o1.json" "$T/golden.json"; then ok "output equals the golden JSON byte-for-byte"
else bad "output differs from golden"; [ -f "$T/o1.json" ] && diff "$T/golden.json" "$T/o1.json" | head -20; fi
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/o2.json" >/dev/null 2>&1
{ [ -f "$T/o1.json" ] && cmp -s "$T/o1.json" "$T/o2.json"; } && ok "second run byte-identical" || bad "second run differs or absent"
# empty allow-list refused (no own rows, no lumen_allow_roots)
sed '/^lumen_allow_roots/d' "$T/scope.yaml" > "$T/scope_noallow.yaml"; : > "$T/subs_empty.tsv"
rm -f "$T/o3.json"; python3 "$STJ" --scope "$T/scope_noallow.yaml" --submodules-tsv "$T/subs_empty.tsv" --out "$T/o3.json" >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 3 ] && [ ! -s "$T/o3.json" ]; } && ok "empty allow-list refused (rc=3, nothing written)" || bad "empty allow: want rc=3 and no output, got rc=$rc"
# --check: golden passes, a JSON missing a class FAILs (negative control of the check)
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/golden.json" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "--check passes on the golden JSON" || bad "--check golden: want rc=0, got rc=$rc"
jq 'del(.classes.qa_corpora) | .deny |= map(select(. != "**/qa-results/"))' "$T/golden.json" > "$T/missing.json"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/missing.json" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 1 ] && ok "--check FAILs when a scope.yaml class is missing from the JSON (rc=1)" || bad "--check missing class: want rc=1, got rc=$rc"
# --check is an EXACT comparison, in both directions (I7): every deviation of the JSON from the re-derived one is rc=1
chk() {  # chk <label> <jq filter on golden>
  jq "$2" "$T/golden.json" > "$T/c.json"
  python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/c.json" >/dev/null 2>"$T/c.err"; rc=$?
  [ "$rc" -eq 1 ] && ok "--check rc=1: $1" || bad "--check $1: want rc=1, got rc=$rc"
}
chk "a third-party root added to allow (would index third-party code)" '.allow += ["submodules/third_b"]'
chk "root_files flipped to false" '.root_files = false'
chk "root_files removed" 'del(.root_files)'
chk "an allow root missing (SM2)" '.allow -= ["catalog-api"]'
chk "a deny pattern missing from deny while still in its class (SM1)" '.deny -= ["**/node_modules/"]'
chk "an extra deny pattern" '.deny += ["/extra/"]'
chk "an extra class" '.classes.bogus = ["/bogus/"] | .deny += ["/bogus/"]'
chk "an extra pattern inside a class" '.classes.caches += ["/more/"] | .deny += ["/more/"]'
chk "the nested third-party deny missing (class and deny)" 'del(.classes.third_party_nested) | .deny -= ["/submodules/own_a/vendor/nested/"]'
chk "dropped_negations changed" '.dropped_negations = []'
# a malformed scope.yaml is fail-closed rc=3 (m6), never a traceback rc=1 that collides with "--check found a gap"
printf -- '- just\n- a list\n' > "$T/scope_list.yaml"
python3 "$STJ" --scope "$T/scope_list.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/o9.json" >/dev/null 2>"$T/o9.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/o9.err"; } && ok "a non-mapping scope.yaml is rc=3, no traceback" || bad "non-mapping scope: want rc=3 without a traceback, got rc=$rc"
printf 'baseline_excludes: [a, b]\nlumen_allow_roots: [x]\n' > "$T/scope_bad2.yaml"
python3 "$STJ" --scope "$T/scope_bad2.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/golden.json" >/dev/null 2>"$T/o10.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/o10.err"; } && ok "a non-mapping baseline_excludes is rc=3 (also under --check)" || bad "non-mapping baseline_excludes: want rc=3, got rc=$rc"
# the JSON is a valid gen_lumenignore.py input and the output is deterministic
mkdir -p "$T/repo/catalog-api" "$T/repo/catalog-web" "$T/repo/submodules/own_a" "$T/repo/submodules/third_b" "$T/repo/docs"
python3 "$GL" --repo "$T/repo" --scope "$T/golden.json" --out "$T/l1" >/dev/null 2>&1; rc1=$?
python3 "$GL" --repo "$T/repo" --scope "$T/golden.json" --out "$T/l2" >/dev/null 2>&1
{ [ "$rc1" -eq 0 ] && cmp -s "$T/l1" "$T/l2" && grep -qx '/docs/' "$T/l1" && grep -qx '/submodules/third_b/' "$T/l1"; } \
  && ok "gen_lumenignore.py accepts the JSON and denies the non-allowed roots deterministically" || bad "gen_lumenignore.py leg: rc=$rc1"
# ---- N-I5: third-party code can not enter the semantic index through scope.yaml; unknown or malformed TSV rows fail closed ------
refuse() {  # refuse <label> <scope> <tsv> [--check file]: exit 3, no traceback, nothing written (derive) / a named reason on stderr
  local lab="$1" sc="$2" tv="$3"; shift 3; rm -f "$T/r.json"
  if [ "${1:-}" = "--check" ]; then python3 "$STJ" --scope "$sc" --submodules-tsv "$tv" --check "$2" >"$T/r.out" 2>"$T/r.err"; rc=$?
  else python3 "$STJ" --scope "$sc" --submodules-tsv "$tv" --out "$T/r.json" >"$T/r.out" 2>"$T/r.err"; rc=$?; fi
  if [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/r.err" && [ ! -s "$T/r.json" ] && [ -s "$T/r.err" ]; then ok "refused (rc=3, reason on stderr, no output): $lab"
  else bad "$lab: want rc=3 with a message and no output, got rc=$rc: $(head -c 160 "$T/r.err")"; fi
}
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api, submodules/third_b]#' "$T/scope.yaml" > "$T/scope_leak1.yaml"
refuse "lumen_allow_roots names a third_party row" "$T/scope_leak1.yaml" "$T/subs.tsv"
refuse "the same under --check (the leak is not re-derived as legal)" "$T/scope_leak1.yaml" "$T/subs.tsv" --check "$T/golden.json"
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api, submodules/third_b/inner]#' "$T/scope.yaml" > "$T/scope_leak2.yaml"
refuse "lumen_allow_roots lies INSIDE a third_party row" "$T/scope_leak2.yaml" "$T/subs.tsv"
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api, submodules/third_b/]#' "$T/scope.yaml" > "$T/scope_leak3.yaml"
refuse "a trailing slash does not hide a third_party root" "$T/scope_leak3.yaml" "$T/subs.tsv"
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api, submodules/own_a/vendor/nested]#' "$T/scope.yaml" > "$T/scope_leak4.yaml"
refuse "lumen_allow_roots names a third_party row nested under an own root" "$T/scope_leak4.yaml" "$T/subs.tsv"
printf '%s\n' "submodules/own_a	own	https://github.com/vasic-digital/own_a.git	own	-" "submodules/x	third-party	https://x/y/z.git	third_party	-" > "$T/subs_unk.tsv"
refuse "a TSV row with an unknown class (third-party)" "$T/scope.yaml" "$T/subs_unk.tsv"
printf '%s\n' "submodules/own_a	own	u	own	-" "garbage-without-a-tab" > "$T/subs_nt.tsv"
refuse "a TSV row without a tab" "$T/scope.yaml" "$T/subs_nt.tsv"
printf '%s\n' "submodules/own_a	own	u	own	-" "submodules/e		u	third_party	-" > "$T/subs_ec.tsv"
refuse "a TSV row with an empty class" "$T/scope.yaml" "$T/subs_ec.tsv"
printf '%s\n' "submodules/own_a	own	u	own	-" "	third_party	u	third_party	-" > "$T/subs_ep.tsv"
refuse "a TSV row with an empty path" "$T/scope.yaml" "$T/subs_ep.tsv"
printf '%s\n' "submodules/own_a	own	u	own	-" "" "   " "submodules/third_b	third_party	u	third_party	-" > "$T/subs_blank.tsv"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_blank.tsv" --out "$T/rb.json" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "control: blank lines in the TSV are tolerated (rc=0)" || bad "blank TSV lines: want rc=0, got $rc"
# the nested-third-party prefix test needs the path-boundary: own_ab is not nested under own_a
printf '%s\n' "submodules/own_a	own	u	own	-" "submodules/own_ab/x	third_party	u	third_party	-" > "$T/subs_pre.tsv"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_pre.tsv" --out "$T/pre.json" >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && [ "$(jq '[.deny[]|select(test("own_ab"))]|length' "$T/pre.json")" = 0 ]; } && ok "own_ab/x (a sibling, not nested under own_a) is not denied as nested" || bad "path-boundary: own_ab/x wrongly treated as nested under own_a (rc=$rc)"
# a trailing slash on an ALLOWED root is normalised away (the allow-list holds bare directory names, exactly as the golden does)
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api/, " catalog-web/ "]#' "$T/scope.yaml" > "$T/scope_slash.yaml"
python3 "$STJ" --scope "$T/scope_slash.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/slash.json" >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && cmp -s "$T/slash.json" "$T/golden.json"; } && ok "allowed roots with a trailing slash or padding derive the identical golden JSON" || bad "allow root normalisation: rc=$rc, output differs from golden"
# m-g: a malformed --check file or an unwritable --out is rc 3, not a traceback with rc 1
jq '.classes = ["a"]' "$T/golden.json" > "$T/c_list.json"
refuse "--check file whose classes is a list" "$T/scope.yaml" "$T/subs.tsv" --check "$T/c_list.json"
rm -f "$T/r.json"; python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/no_such_dir/o.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/r.err"; } && ok "--out to an unwritable path is rc=3 without a traceback" || bad "--out unwritable: want rc=3 without a traceback, got rc=$rc"
# round 4 (WF3 review m-3, m-5, m-8)
# m-3: the third_party root test has a PATH BOUNDARY: submodules/third_bx is a sibling of third_b, not inside it (no over-refusal)
sed 's#^lumen_allow_roots:.*#lumen_allow_roots: [catalog-api, submodules/third_bx]#' "$T/scope.yaml" > "$T/scope_sib.yaml"
python3 "$STJ" --scope "$T/scope_sib.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/sib.json" >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && jq -e '.allow|index("submodules/third_bx")!=null' "$T/sib.json" >/dev/null 2>&1; } && ok "m-3 a sibling root (third_bx next to third_b) is accepted, not refused (rc=0)" || bad "m-3 sibling root: want rc=0 with the root allowed, got rc=$rc"
# m-5a: a path classed both own and third_party is refused (it would be allowed and not denied)
printf '%s\n' "submodules/dup	own	u	own	-" "submodules/dup	third_party	u	third_party	-" > "$T/subs_dup.tsv"
refuse "m-5a a path classed both own and third_party" "$T/scope.yaml" "$T/subs_dup.tsv"
# m-5b: a nested third_party path with glob metacharacters is denied LITERALLY (x[1] must not become a pattern matching x1)
printf '%s\n' "submodules/own_a	own	u	own	-" 'submodules/own_a/vendor/x[1]	third_party	u	third_party	-' > "$T/subs_glob.tsv"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_glob.tsv" --out "$T/glob.json" >/dev/null 2>&1; rc=$?
{ [ "$rc" -eq 0 ] && jq -e '.deny|index("/submodules/own_a/vendor/x\\[1\\]/")!=null' "$T/glob.json" >/dev/null 2>&1 && ! jq -e '.deny|index("/submodules/own_a/vendor/x[1]/")!=null' "$T/glob.json" >/dev/null 2>&1; } && ok "m-5b a nested third_party path with [ ] is denied literally (backslash-escaped)" || bad "m-5b glob metacharacters in a nested third_party path: rc=$rc deny=$(jq -c '.deny|map(select(test("vendor")))' "$T/glob.json" 2>/dev/null)"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_glob.tsv" --check "$T/glob.json" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "m-5b control: --check of the escaped derivation is exact (rc=0)" || bad "m-5b control: --check rc=$rc"
# m-8: --check with a non-string list element is rc 3 without a traceback; a duplicated entry is a difference (rc 1), the comparison is EXACT
jq '.allow += [["x"]]' "$T/golden.json" > "$T/c_nonstr.json"
refuse "m-8 --check file whose allow holds a non-string element" "$T/scope.yaml" "$T/subs.tsv" --check "$T/c_nonstr.json"
jq '.allow += ["catalog-api"]' "$T/golden.json" > "$T/c_dup.json"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/c_dup.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 1 ] && grep -q duplicate "$T/r.err"; } && ok "m-8 a duplicated allow entry in the --check file is a difference (rc=1, named duplicate)" || bad "m-8 duplicate entry: want rc=1 naming a duplicate, got rc=$rc: $(head -c 160 "$T/r.err")"
# round 5 (WF5 review D1): a non-string (or unhashable) element INSIDE a classes list is rc 3 without a traceback (was TypeError, rc 1)
jq '.classes.caches += [["x"]]' "$T/golden.json" > "$T/c_clsnonstr.json"
refuse "round5 D1 --check file whose classes list holds a list element" "$T/scope.yaml" "$T/subs.tsv" --check "$T/c_clsnonstr.json"
jq '.classes.caches += [7]' "$T/golden.json" > "$T/c_clsint.json"
refuse "round5 D1 --check file whose classes list holds a number" "$T/scope.yaml" "$T/subs.tsv" --check "$T/c_clsint.json"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/c_clsnonstr.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/r.err"; } && ok "round5 D1 no traceback and rc=3 for a non-string classes element" || bad "round5 D1: want rc=3 without a traceback, got rc=$rc: $(head -c 120 "$T/r.err")"
# round 6 (WF6 N6-6): a non-string class name and non-UTF-8 input are rc 3 without a traceback; root_files compares EXACTLY (1 is not true)
printf 'schema_version: 1\nbaseline_excludes:\n  1: ["/x/"]\nlumen_allow_roots: [catalog-api]\n' > "$T/scope_intkey.yaml"
refuse "N6-6 an integer class name in baseline_excludes" "$T/scope_intkey.yaml" "$T/subs.tsv"
printf 'schema_version: 1\nbaseline_excludes:\n  null: ["/x/"]\nlumen_allow_roots: [catalog-api]\n' > "$T/scope_nullkey.yaml"
refuse "N6-6 a null class name in baseline_excludes" "$T/scope_nullkey.yaml" "$T/subs.tsv"
python3 "$STJ" --scope "$T/scope_intkey.yaml" --submodules-tsv "$T/subs.tsv" --out "$T/r.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/r.err"; } && ok "N6-6 integer class name: rc=3 and no traceback (was TypeError, rc 1)" || bad "N6-6 integer class name: want rc=3 without a traceback, got rc=$rc: $(head -c 120 "$T/r.err")"
printf 'submodules/own_a\377\town\tu\town\t-\n' > "$T/subs_nonutf8.tsv"
refuse "N6-6 a submodules TSV that is not UTF-8" "$T/scope.yaml" "$T/subs_nonutf8.tsv"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_nonutf8.tsv" --out "$T/r.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 3 ] && ! grep -q Traceback "$T/r.err"; } && ok "N6-6 non-UTF-8 TSV: rc=3 and no traceback (was UnicodeDecodeError, rc 1)" || bad "N6-6 non-UTF-8 TSV: want rc=3 without a traceback, got rc=$rc: $(head -c 120 "$T/r.err")"
printf 'schema_version: 1\nlumen_allow_roots: [\377]\n' > "$T/scope_nonutf8.yaml"
refuse "N6-6 a scope.yaml that is not UTF-8" "$T/scope_nonutf8.yaml" "$T/subs.tsv"
printf '\377\376{}' > "$T/check_nonutf8.json"
refuse "N6-6 a --check file that is not UTF-8" "$T/scope.yaml" "$T/subs.tsv" --check "$T/check_nonutf8.json"
jq '.root_files = 1' "$T/golden.json" > "$T/c_rf1.json"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/c_rf1.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 1 ] && grep -q root_files "$T/r.err"; } && ok "N6-6 root_files: 1 is NOT true: the exact check finds the difference (rc=1, root_files named)" || bad "N6-6 root_files 1: want rc=1 naming root_files, got rc=$rc: $(head -c 120 "$T/r.err")"
python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs.tsv" --check "$T/golden.json" >/dev/null 2>&1; rc=$?
[ "$rc" -eq 0 ] && ok "N6-6 control: root_files true (the golden) still passes the exact check (rc=0)" || bad "N6-6 control: golden --check rc=$rc"
# a valid UTF-8 non-ASCII path is read as UTF-8 whatever the locale (explicit encoding): the TSV is read under a hostile ASCII locale
printf 'submodules/caf\303\251\town\tu\town\t-\n' > "$T/subs_utf8.tsv"
LC_ALL=C PYTHONCOERCECLOCALE=0 PYTHONUTF8=0 python3 "$STJ" --scope "$T/scope.yaml" --submodules-tsv "$T/subs_utf8.tsv" --out "$T/utf8.json" >/dev/null 2>"$T/r.err"; rc=$?
{ [ "$rc" -eq 0 ] && jq -e '.allow|index("submodules/café")!=null' "$T/utf8.json" >/dev/null 2>&1; } && ok "N6-6 a UTF-8 TSV with a non-ASCII path is read as UTF-8 under an ASCII locale (rc=0)" || bad "N6-6 UTF-8 TSV under LC_ALL=C: want rc=0 with the path allowed, got rc=$rc: $(head -c 120 "$T/r.err")"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) script=$(sha256sum "$STJ" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) python=$(python3 -V 2>&1 | cut -d" " -f2) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
