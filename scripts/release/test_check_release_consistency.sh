#!/usr/bin/env bash
# Test-first suite for check_release_consistency.sh (constitution 11.4.224).
# Golden-good, golden-bad (one mutation per file type), negative controls.
# CHECKER=<path> runs the suite against another copy (used by the mutation runner).
# shellcheck disable=SC2016  # backticks inside sed programs are literal on purpose
# Exit 0 only if every case behaves as required.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="${CHECKER:-$HERE/check_release_consistency.sh}"
# shellcheck source=scripts/release/release_fixture.sh
. "$HERE/release_fixture.sh"

BASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/test.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/test.*) ;; *) echo "refusing odd ROOT $ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT

PASS=0; FAIL=0
GOLD="$ROOT/gold"
make_fixture "$GOLD" || { echo "fixture build failed" >&2; exit 2; }

fresh() { local t; t="$(mktemp -d "$ROOT/c.XXXXXX")" && cp -a "$GOLD/." "$t/" && echo "$t"; }
ok()   { PASS=$((PASS+1)); }
bad()  { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }
# expect <label> <rc> <must-contain-or-empty> <dir> [checker args...]
expect() {
  local label="$1" rc="$2" needle="$3" dir="$4"; shift 4
  local out got
  out="$("$CHECKER" --repo "$dir" "$@" 2>&1)"; got=$?
  if [ "$got" != "$rc" ]; then bad "$label: rc=$got want $rc"; return; fi
  if [ -n "$needle" ] && ! printf '%s\n' "$out" | grep -qF -- "$needle"; then
    bad "$label: output lacks '$needle'"; return; fi
  if ! printf '%s\n' "$out" | grep -q '^VERDICT: '; then bad "$label: no VERDICT line"; return; fi
  ok
}
# mutation helper: mut <dir> <file> <sed-expr>  (asserts the sed really changed the file)
mut() { local before; before="$(cksum < "$1/$2")"; sed -i "$3" "$1/$2"; [ "$before" != "$(cksum < "$1/$2")" ] || { bad "mutation no-op: $2 $3"; return 1; }; }

echo "== golden-good"
d=$(fresh); expect "good: all" 0 "VERDICT: PASS" "$d"
d=$(fresh); expect "good: version only" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); expect "good: changelog only" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); expect "good: explicit version" 0 "VERDICT: PASS" "$d" --version 2.4.0
d=$(fresh); expect "good: prev-tag versionCode 6 -> 7" 0 "VERDICT: PASS" "$d" --prev-tag fx-prev
d=$(fresh); expect "good: explicit section" 0 "VERDICT: PASS" "$d" --section 2.4.0
d=$(fresh); expect "good: sha UNVERIFIABLE counted, not FAIL" 0 "unverifiable=1" "$d" --check changelog

echo "== golden-bad: stale version per file type"
vf="versions.json:s/\"minor\": 4/\"minor\": 3/
docs/api/openapi.yaml:s/^  version: 2.4.0/  version: 2.3.0/
catalog-web/package.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"2.3.0\"/
catalogizer-desktop/package.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"2.3.0\"/
installer-wizard/package.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"2.3.0\"/
catalog-web/package-lock.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"1.0.0\"/
catalogizer-desktop/package-lock.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"1.0.0\"/
installer-wizard/package-lock.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"1.0.0\"/
catalog-web/package-lock.json:s/\"name\": \"catalog-web\", \"version\": \"2.4.0\"/\"name\": \"catalog-web\", \"version\": \"1.0.0\"/
catalogizer-desktop/package-lock.json:s/\"name\": \"catalogizer-desktop\", \"version\": \"2.4.0\"/\"name\": \"catalogizer-desktop\", \"version\": \"1.0.0\"/
installer-wizard/package-lock.json:s/\"name\": \"installer-wizard\", \"version\": \"2.4.0\"/\"name\": \"installer-wizard\", \"version\": \"1.0.0\"/
catalogizer-desktop/src-tauri/tauri.conf.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"2.3.0\"/
installer-wizard/src-tauri/tauri.conf.json:s/^  \"version\": \"2.4.0\"/  \"version\": \"2.3.0\"/
catalogizer-desktop/src-tauri/Cargo.toml:s/^version = \"2.4.0\"/version = \"2.3.0\"/
installer-wizard/src-tauri/Cargo.toml:s/^version = \"2.4.0\"/version = \"2.3.0\"/
catalogizer-android/app/build.gradle.kts:s/versionName = \"2.4.0\"/versionName = \"2.3.0\"/
catalogizer-androidtv/app/build.gradle.kts:s/versionName = \"2.4.0\"/versionName = \"2.3.0\"/"
while IFS= read -r line; do
  f="${line%%:*}"; e="${line#*:}"
  d=$(fresh)
  if mut "$d" "$f" "$e"; then expect "stale: $f [$e]" 1 "FAIL" "$d" --check version; fi
done <<< "$vf"

echo "== golden-bad: version expectations and structure"
d=$(fresh); expect "expected version differs from all files" 1 "FAIL" "$d" --check version --version 2.4.1
d=$(fresh); rm "$d/catalogizer-android/app/build.gradle.kts"; expect "missing version file" 1 "FAIL" "$d" --check version
d=$(fresh); printf 'not json' > "$d/catalog-web/package.json"; expect "unparseable package.json" 1 "FAIL" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts '/versionName/d' && expect "gradle without versionName" 1 "FAIL" "$d" --check version
d=$(fresh); mut "$d" catalog-web/package-lock.json 's/"": {[^}]*}/"x": {}/' && expect "lock without root package entry" 1 "FAIL" "$d" --check version

echo "== golden-bad: versionCode"
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7/versionCode = 6/' && expect "versionCode equal to prev tag" 1 "FAIL" "$d" --prev-tag fx-prev
d=$(fresh); mut "$d" catalogizer-androidtv/app/build.gradle.kts 's/versionCode = 7/versionCode = 5/' && expect "versionCode lower than prev tag" 1 "FAIL" "$d" --prev-tag fx-prev
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7/versionCode = "seven"/' && expect "versionCode not an integer" 1 "FAIL" "$d" --check version
d=$(fresh); expect "unknown prev tag" 1 "FAIL" "$d" --check version --prev-tag no-such-tag

echo "== negative controls: dependency versions and history must not matter"
d=$(fresh); mut "$d" catalog-web/package.json 's/"lodash": "1.0.0"/"lodash": "0.0.1"/' && expect "dependency version change ignored (package.json)" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-desktop/src-tauri/Cargo.toml 's/serde = { version = "1.0.0"/serde = { version = "0.0.1"/' && expect "dependency version change ignored (Cargo)" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/a:b:1.0.0/a:b:0.0.1/' && expect "dependency version change ignored (gradle)" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" docs/api/openapi.yaml 's/version: 7.7.7/version: 0.0.1/' && expect "nested openapi version ignored" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalog-web/package-lock.json 's/"node_modules\/react": {"version": "2.3.0"}/"node_modules\/react": {"version": "0.0.1"}/' && expect "lock dependency entry ignored" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" versions.json 's/"version": "9.9.9"/"version": "0.0.1"/' && expect "versions.json component sub-fields ignored" 0 "VERDICT: PASS" "$d" --check version

echo "== golden-bad: changelog evidence paths"
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#`scripts/tool.sh`#`scripts/not_a_tool.sh`#' && expect "fake path MISS" 1 "MISS" "$d" --check changelog
d=$(fresh); rm "$d/docs/qa/run2/c.txt"; expect "tracked path removed from tree MISS" 1 "MISS" "$d" --check changelog
d=$(fresh); (cd "$d" && git rm -q --cached docs/qa/run2/c.txt); expect "present but untracked path MISS" 1 "MISS" "$d" --check changelog
d=$(fresh); expect "older section path missing is checked when asked" 1 "MISS" "$d" --check changelog --section 2.3.0
d=$(fresh); expect "section not found -> BLIND" 2 "section not found" "$d" --check changelog --section 9.9.9
d=$(fresh); mut "$d" docs/CHANGELOG.md 's/^### VERIFIED (live evidence)/### CHECKED (live evidence)/' && expect "no VERIFIED subsection -> BLIND" 2 "no VERIFIED subsection" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md '/^- Run one:/d;/^- Tool/d' && expect "zero paths -> BLIND" 2 "zero repo paths" "$d" --check changelog
d=$(fresh); rm "$d/docs/CHANGELOG.md"; expect "changelog file missing -> BLIND" 2 "VERDICT: BLIND" "$d" --check changelog
d=$(fresh); echo tampered > "$d/docs/qa/run1/a.txt"; (cd "$d" && git add -A); expect "SHA256SUMS mismatch FAIL" 1 "SHAFAIL" "$d" --check changelog
d=$(fresh); rm "$d/docs/qa/run1/b.txt"; expect "SHA256SUMS missing non-ignored file FAIL" 1 "SHAFAIL" "$d" --check changelog

echo "== negative controls: changelog"
d=$(fresh); expect "PROVISIONAL / Unreleased / dev-heading / older-section fakes not checked" 0 "miss=0" "$d" --check changelog
d=$(fresh); expect "default is topmost version section, not Unreleased" 0 "section=2.4.0" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#`docs/qa/run2/c.txt:12`#`docs/qa/run2/c.txt:12-20`#' && expect "line suffix stripped" 0 "miss=0" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#`scripts/tool.sh`#`nonexistent-top/x.sh`#' && expect "unknown top-level dir is skipped not MISS" 0 "skipped=1" "$d" --check changelog

echo "== usage"
d=$(fresh); expect "unknown option -> 2" 2 "VERDICT: BLIND" "$d" --bogus
d=$(fresh); expect "bad --check value -> 2" 2 "VERDICT: BLIND" "$d" --check nothing
expect "nonexistent repo -> 2" 2 "VERDICT: BLIND" "$ROOT/does-not-exist"

echo "== selftest (11.4.273 control needles built into the tool)"
out="$("$CHECKER" --selftest 2>&1)"; got=$?
if [ "$got" = 0 ] && printf '%s\n' "$out" | grep -q '^VERDICT: PASS selftest'; then ok; else bad "selftest rc=$got"; fi

echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
