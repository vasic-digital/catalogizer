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
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#`scripts/tool.sh`#`nonexistent-top/x.sh`#' && expect "unknown top-level dir is skipped not MISS" 0 "skipped=3" "$d" --check changelog

echo "== usage"
d=$(fresh); expect "unknown option -> 2" 2 "VERDICT: BLIND" "$d" --bogus
d=$(fresh); expect "bad --check value -> 2" 2 "VERDICT: BLIND" "$d" --check nothing
expect "nonexistent repo -> 2" 2 "VERDICT: BLIND" "$ROOT/does-not-exist"

echo "== round-1 review fixes (B1, F1-F5)"
mkstub() { local sd; sd="$(mktemp -d "$ROOT/stub.XXXXXX")"; printf '#!/bin/sh\n%s\n' "$1" > "$sd/python3"; chmod +x "$sd/python3"; echo "$sd"; }
# B1: a crashed / silent / short python helper must be BLIND, never PASS
sd=$(mkstub 'exit 1');  d=$(fresh); PATH="$sd:$PATH" expect "B1 python3 crashes silently -> BLIND" 2 "VERDICT: BLIND" "$d" --check version
sd=$(mkstub 'exit 0');  d=$(fresh); PATH="$sd:$PATH" expect "B1 python3 silent exit 0 -> BLIND" 2 "no PLAN line" "$d" --check version
sd=$(mkstub 'printf "PLAN\t19\n"; i=0; while [ $i -lt 19 ]; do printf "OK\tx%s\ty\n" $i; i=$((i+1)); done; exit 1')
d=$(fresh); PATH="$sd:$PATH" expect "B1 helper emits all rows then exits 1 -> BLIND" 2 "version helper exited 1" "$d" --check version
sd=$(mkstub 'printf "PLAN\t19\n"; printf "OK\ta\tb\n"; exit 0')
d=$(fresh); PATH="$sd:$PATH" expect "B1 helper emits fewer rows than planned -> BLIND" 2 "of 19 planned" "$d" --check version
sd=$(mkstub 'exit 1');  d=$(fresh); PATH="$sd:$PATH" expect "B1 python3 crash also BLIND for changelog" 2 "VERDICT: BLIND" "$d" --check changelog
# F1/F4: SHA256SUMS hygiene
d=$(fresh); echo 'this is not a checksum line' >> "$d/docs/qa/run1/SHA256SUMS"; expect "F1 malformed SHA256SUMS line FAIL" 1 "SHAFAIL" "$d" --check changelog
z=$(printf '0%.0s' $(seq 64))
d=$(fresh); echo "$z  ../../etc/passwd" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F4 SHA256SUMS ../ escape FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); echo "$z  /etc/hostname" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F4 SHA256SUMS absolute path FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); echo "$z  /etc/hostname" >> "$d/docs/qa/run1/SHA256SUMS"
if "$CHECKER" --repo "$d" --check changelog 2>&1 | grep -q 'etc/hostname (checksum mismatch'; then bad "F4 escaping manifest must not be executed"; else ok; fi
# F2: every dropped slash-token is counted
d=$(fresh); expect "F2 url and spaced token counted as skipped" 0 "skipped=2" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#^- Tool#- Glob `docs/qa/*/a.txt` and `~/x/y`; Tool#' && expect "F2 glob and ~ tokens counted" 0 "skipped=4" "$d" --check changelog
# F3: all VERIFIED subsections are read
d=$(fresh); mut "$d" docs/CHANGELOG.md 's@^### REOPENED@\n### VERIFIED (more)\n- `docs/nope_second.txt`\n\n### REOPENED@' && expect "F3 second VERIFIED subsection checked" 1 "docs/nope_second.txt" "$d" --check changelog
# F5
d=$(fresh); mut "$d" docs/api/openapi.yaml 's#^  version: 2.4.0#  version: 2.4.0\n  version: 2.4.0#' && expect "F5 duplicate openapi info.version FAIL" 1 "exactly one info.version" "$d" --check version
expect "F5 empty --version is a usage error" 2 "VERDICT: BLIND" "$GOLD" --version ""
d=$(fresh); : > "$d/catalogizer-android/app/build.gradle"; expect "F5 both build.gradle and .kts -> FAIL" 1 "ambiguous" "$d" --check version
d=$(fresh); echo q > "$d/\"q\".txt"; (cd "$d" && git add -A); expect "F5 needle survives a quoted tracked name" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); echo b > "$d/.0first.txt"; (cd "$d" && git add -A) ; rm "$d/.0first.txt"; expect "F5 needle survives first tracked file deleted from tree" 0 "VERDICT: PASS" "$d" --check changelog
expect "F5 -h prints VERDICT: HELP" 0 "VERDICT: HELP" "$GOLD" -h

echo "== round-2 review follow-ups (N1-N5)"
REALPY="$(command -v python3)"
# filtstub <shell filter applied to the real helper's stdout>: runs the REAL helper, then forges its output.
filtstub() { local sd; sd="$(mktemp -d "$ROOT/fstub.XXXXXX")"; printf '#!/bin/sh\n%s -I "$@" | { %s; }\nexit 0\n' "$REALPY" "$1" > "$sd/python3"; chmod +x "$sd/python3"; echo "$sd"; }
# N1: forged / incomplete version-helper output must be BLIND or FAIL, never PASS
sd=$(mkstub 'printf "PLAN\t0\n"; exit 0');  d=$(fresh); PATH="$sd:$PATH" expect "N1 PLAN 0 and no rows -> BLIND" 2 "VERDICT: BLIND" "$d" --check version
sd=$(mkstub 'printf "PLAN\t19\n"; i=0; while [ $i -lt 19 ]; do printf "INFO\tx%s\ty\n" $i; i=$((i+1)); done; exit 0')
d=$(fresh); PATH="$sd:$PATH" expect "N1 19 INFO rows with foreign labels -> BLIND" 2 "VERDICT: BLIND" "$d" --check version
sd=$(filtstub 'cat; printf "PLAN\t19\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 duplicate PLAN line -> BLIND" 2 "more than one PLAN" "$d" --check version
sd=$(filtstub 'sed "s/^PLAN.*/PLAN\t18/" | grep -v "openapi.yaml info.version"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 helper plan not 19 -> BLIND" 2 "expected 19" "$d" --check version
sd=$(filtstub 'grep -v "openapi.yaml info.version"; printf "OK\tdocs/api/openapi.yaml info.version\t2.4.0\n"; printf "OK\tdocs/api/openapi.yaml info.version\t2.4.0\n"'); d=$(fresh)
PATH="$sd:$PATH" expect "N1 duplicate label forged to keep row count -> BLIND" 2 "duplicate" "$d" --check version
sd=$(filtstub 'grep -v "openapi.yaml info.version" | sed "s/^OK\tversions.json global/OK\tversions.json global/"; printf "OK\tinvented label\t2.4.0\n"'); d=$(fresh)
PATH="$sd:$PATH" expect "N1 expected label replaced by invented one -> BLIND" 2 "missing" "$d" --check version
sd=$(filtstub 'grep -v "versionCode"; '); d=$(fresh); PATH="$sd:$PATH" expect "N1 versionCode rows dropped (plan intact) -> BLIND" 2 "VERDICT: BLIND" "$d" --check version --prev-tag fx-prev
sd=$(filtstub 'sed "s/^INFO\t/OK\t/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 INFO relabelled OK without prev-tag stays consistent" 0 "VERDICT: PASS" "$d" --check version
sd=$(filtstub 'sed "s/^OK\t\(catalog-web\/package.json version\)/INFO\t\1/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 INFO on a non-versionCode field -> BLIND" 2 "VERDICT: BLIND" "$d" --check version
# N1 changelog helper completeness
sd=$(filtstub 'grep -v "^END"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog helper without END -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END.*/END\t99\t0/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog END counts mismatch -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'awk "/^PATH/ && !done {done=1; next} {print}"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog PATH row dropped -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "SECTION\t1.0.0\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog second SECTION -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END.*/&\n&/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog duplicate END -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "BOGUS\tx\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog trailing non-END row -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'awk "/^SKIP/ && !done {done=1; next} {print}"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog SKIP row dropped -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END/SECTION\t1.0.0\nEND/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog second SECTION before END -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "OK\t--prev-tag\tforged\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 forged OK on the --prev-tag row -> BLIND" 2 "unexpected rows" "$d" --check version
sd=$(mkstub 'printf "SECTION\t2.4.0\nPATH\tdocs/qa/run2/c.txt\n"; exit 0'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog hand-forged output -> BLIND" 2 "changelog helper output incomplete" "$d" --check changelog
# N2: BSD-tag manifest lines
d=$(fresh); echo "SHA256 (../../etc/passwd) = $z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "N2 BSD-tag ../ escape FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); echo "SHA256 (/etc/hostname) = $z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "N2 BSD-tag absolute escape FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); h="$(sha256sum < "$d/docs/qa/run1/a.txt" | cut -d' ' -f1)"; echo "SHA256 (a.txt) = $h" >> "$d/docs/qa/run1/SHA256SUMS"; expect "N2 valid BSD-tag line accepted" 0 "VERDICT: PASS" "$d" --check changelog
# N3: tracked symlink pointing outside the repository
d=$(fresh); mkdir -p "$ROOT/outside1"; echo x > "$ROOT/outside1/f.txt"; ( cd "$ROOT/outside1" && sha256sum f.txt | sed 's/^./1/' > SHA256SUMS )
ln -s "$ROOT/outside1" "$d/docs/qa/linkout"; (cd "$d" && git add -A)
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/linkout/`; Tool `scripts/tool.sh`#' && expect "N3 symlinked dir outside repo -> MISS" 1 "resolves outside the repository" "$d" --check changelog
d=$(fresh); ln -s run2 "$d/docs/qa/linkin"; (cd "$d" && git add -A)
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/linkin/`; Tool `scripts/tool.sh`#' && expect "N3 symlinked dir inside repo accepted" 0 "VERDICT: PASS" "$d" --check changelog
# N4: versionCode must be a bare literal
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 - 2/' && expect "N4 versionCode expression -> FAIL" 1 "FAIL" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/\/ bumped/' && expect "N4 versionCode with trailing comment accepted" 0 "VERDICT: PASS" "$d" --check version
# N5: duplicate changelog heading; versionNameSuffix is not a second versionName
d=$(fresh); printf '\n## [2.4.0] - duplicate\n\n### VERIFIED\n- `docs/qa/run2/c.txt`\n' >> "$d/docs/CHANGELOG.md"; expect "N5 duplicate section heading -> BLIND" 2 "duplicate" "$d" --check changelog
d=$(fresh); printf '\n## [2.3.0] - duplicate\n\n### VERIFIED\n- `docs/qa/run2/c.txt`\n' >> "$d/docs/CHANGELOG.md"; expect "N5 duplicate of a non-selected section ignored" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/^\( *\)versionName = "2.4.0"/&\n\1versionNameSuffix = "-dev"/' && expect "N5 versionNameSuffix is not ambiguity" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/^\( *\)versionName = "2.4.0"/&\n\1versionNameSuffix/' && expect "N5 bare versionNameSuffix token is not versionName" 0 "VERDICT: PASS" "$d" --check version

echo "== round-3 review follow-ups (F1-F5)"
# F1: backslash-prefixed manifest lines must fail closed (uutils sha256sum honours them)
d=$(fresh); echo "\\SHA256 (../../etc/passwd) = $z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F1 backslash BSD ../ escape FAIL" 1 "unsupported manifest line" "$d" --check changelog
d=$(fresh); echo "\\$z  ../../etc/passwd" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F1 backslash GNU line FAIL" 1 "unsupported manifest line" "$d" --check changelog
d=$(fresh); echo "# a comment" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F1 any non-matching line FAIL (comment)" 1 "unsupported manifest line" "$d" --check changelog
d=$(fresh); printf '\n' >> "$d/docs/qa/run1/SHA256SUMS"; expect "F1 blank manifest line tolerated" 0 "VERDICT: PASS" "$d" --check changelog
# F2: in-dir symlink entry pointing outside the repository
d=$(fresh); echo outside-content > "$ROOT/outside2.txt"; ln -s "$ROOT/outside2.txt" "$d/docs/qa/run1/lnk.txt"
echo "$(sha256sum < "$ROOT/outside2.txt" | cut -d' ' -f1)  lnk.txt" >> "$d/docs/qa/run1/SHA256SUMS"; (cd "$d" && git add -A)
expect "F2 manifest entry symlink outside repo FAIL" 1 "resolves outside" "$d" --check changelog
d=$(fresh); ln -s a.txt "$d/docs/qa/run1/lnk2.txt"; echo "$(sha256sum < "$d/docs/qa/run1/a.txt" | cut -d' ' -f1)  lnk2.txt" >> "$d/docs/qa/run1/SHA256SUMS"; (cd "$d" && git add -A)
expect "F2 manifest entry symlink inside repo accepted" 0 "VERDICT: PASS" "$d" --check changelog
# F3: reviewer mutants (sibling-prefix, repo-root symlink, uppercase BSD hex, '/' vs '//' comment)
d=$(fresh); rr="$(cd "$d" && pwd -P)"; mkdir -p "$rr-evil"; echo e > "$rr-evil/f.txt"; ln -s "$rr-evil" "$d/docs/qa/sibling"; (cd "$d" && git add -A)
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/sibling/`; Tool `scripts/tool.sh`#' && expect "F3 symlink to sibling dir sharing repo-path prefix -> MISS" 1 "resolves outside the repository" "$d" --check changelog
rm -rf "${rr:?}-evil"
d=$(fresh); ln -s ../.. "$d/docs/qa/toroot"; (cd "$d" && git add -A)
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/toroot`; Tool `scripts/tool.sh`#' && expect "F3 symlink resolving to the repo root itself accepted" 0 "VERDICT: PASS" "$d" --check changelog --evidence-prefix docs/none/
d=$(fresh); H="$(sha256sum < "$d/docs/qa/run1/a.txt" | cut -d' ' -f1 | tr a-f A-F)"; echo "SHA256 (../../etc/passwd) = $H" >> "$d/docs/qa/run1/SHA256SUMS"; expect "F3 BSD uppercase-hex escape FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/ 2/' && expect "F3 versionCode '7 / 2' (single slash) FAIL" 1 "FAIL" "$d" --check version
# F4: forged INFO on a versionCode row under --prev-tag
sd=$(filtstub 'sed "s/^OK\t\([^\t]*versionCode\)\t.*/INFO\t\1\tforged/"'); d=$(fresh); PATH="$sd:$PATH" expect "F4 forged INFO on versionCode under --prev-tag -> BLIND" 2 "VERDICT: BLIND" "$d" --check version --prev-tag fx-prev
d=$(fresh); expect "F4 unknown tag still gives FAIL (INFO rows legitimate then)" 1 "FAIL" "$d" --check version --prev-tag no-such-tag
d=$(fresh); expect "F4 INFO rows fine without --prev-tag" 0 "VERDICT: PASS" "$d" --check version
# F5: fenced heading, gradle literal forms
d=$(fresh); printf '\n```\n## [2.4.0] - example in a code fence\n```\n' >> "$d/docs/CHANGELOG.md"; expect "F5 fenced duplicate heading ignored" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); printf '\n```md\n## [2.5.0]\n### VERIFIED\n- `docs/nope_fenced.txt`\n```\n' >> "$d/docs/CHANGELOG.md"; expect "F5 fenced heading is not a section" 2 "section not found" "$d" --check changelog --section 2.5.0
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* c *\//' && expect "F5 versionCode with block comment parses" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7;/' && expect "F5 versionCode with semicolon parses" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* c *\/ - 2/' && expect "F5 expression after block comment still FAIL" 1 "FAIL" "$d" --check version

# F2/F5 residual mutants: manifest sibling-prefix symlink; fenced "## " / "### " inside a VERIFIED subsection
d=$(fresh); rr="$(cd "$d" && pwd -P)"; mkdir -p "$rr-evil2"; echo e > "$rr-evil2/f.txt"; ln -s "$rr-evil2" "$d/docs/qa/run1/evil"
echo "$(sha256sum < "$rr-evil2/f.txt" | cut -d' ' -f1)  evil/f.txt" >> "$d/docs/qa/run1/SHA256SUMS"; (cd "$d" && git add -A)
expect "F2 manifest entry via symlink to sibling dir sharing repo-path prefix FAIL" 1 "resolves outside" "$d" --check changelog
rm -rf "${rr:?}-evil2"
d=$(fresh); mut "$d" docs/CHANGELOG.md 's@^### PROVISIONAL (not verified)@```\n## fenced not a section end\n```\n- `docs/nope_after_fence2.txt`\n\n### PROVISIONAL (not verified)@' && expect "F5 fenced '## ' does not end the section" 1 "docs/nope_after_fence2.txt" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md 's@^### PROVISIONAL (not verified)@```\n### Notes fenced not a subsection end\n```\n- `docs/nope_after_fence3.txt`\n\n### PROVISIONAL (not verified)@' && expect "F5 fenced '### ' does not end the VERIFIED subsection" 1 "docs/nope_after_fence3.txt" "$d" --check changelog

# F1 residual: an empty manifest passes the line parser but sha256sum rejects it (no checksum lines)
d=$(fresh); : > "$d/docs/qa/run1/SHA256SUMS"; expect "F1 empty manifest judged malformed by sha256sum -> FAIL" 1 "SHAFAIL" "$d" --check changelog

echo "== round-4 review fixes (CRLF, missing parent, comment span, no-newline, indented fence)"
# R4-1: CRLF manifest lines must not bypass the escape / symlink checks
d=$(fresh); echo outside-content > "$ROOT/outside3.txt"; ln -s "$ROOT/outside3.txt" "$d/docs/qa/run1/lnk.txt"
printf '%s  lnk.txt\r\n' "$(sha256sum < "$ROOT/outside3.txt" | cut -d' ' -f1)" >> "$d/docs/qa/run1/SHA256SUMS"; (cd "$d" && git add -A)
expect "R4-1 CRLF line with outside symlink entry FAIL" 1 "resolves outside" "$d" --check changelog
d=$(fresh); printf '%s  ../../etc/passwd\r\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-1 CRLF line with ../ escape FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); printf '%s  a\rb.txt\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-1 CR inside a line FAIL" 1 "unsupported manifest line" "$d" --check changelog
d=$(fresh); h="$(sha256sum < "$d/docs/qa/run1/a.txt" | cut -d' ' -f1)"; printf '%s  a.txt\r\n' "$h" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-1 valid CRLF line still accepted" 0 "VERDICT: PASS" "$d" --check changelog
# R4-2: gitignored entry under a missing subdirectory stays UNVERIFIABLE, not an escape
d=$(fresh); printf '%s  sub/big.png\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-2 gitignored entry in missing subdir is UNVERIFIABLE" 0 "unverifiable=2" "$d" --check changelog
d=$(fresh); ln -s "$ROOT" "$d/docs/qa/run1/escdir"; (cd "$d" && git add -A); printf '%s  escdir/missing/x.png\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-2 missing tail below an outside symlink still FAIL" 1 "resolves outside" "$d" --check changelog
# R4-3: a comment must not span past its own closing */
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* a *\/ + 1 \/* b *\//' && expect "R4-3 versionCode '7 /* a */ + 1 /* b */' FAIL" 1 "FAIL" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionName = "2.4.0"/versionName = "2.4.0" \/* a *\/ + "x" \/* b *\//' && expect "R4-3 versionName with two comments around + FAIL" 1 "FAIL" "$d" --check version
# R4-4: final manifest line without trailing newline is still checked; indented fences
d=$(fresh); printf '%s  ../../CHANGELOG.md' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "R4-4 no-trailing-newline ../ entry FAIL" 1 "escapes its directory" "$d" --check changelog
d=$(fresh); printf '\n  ```\n## [2.4.0] - example in an indented fence\n  ```\n' >> "$d/docs/CHANGELOG.md"; expect "R4-4 indented fence ignored" 0 "VERDICT: PASS" "$d" --check changelog


echo "== RC-3 / RC-3b / RC2 follow-ups (manifest required, manifest complete, waivers, comments, CR)"
hsh() { sha256sum < "$1" | cut -d' ' -f1; }
# addrow <dir> <path>: append a path to the VERIFIED bullet of the 2.4.0 section
addrow() { mut "$1" docs/CHANGELOG.md "s#^- Tool \`scripts/tool.sh\`#- Extra \`$2\`; Tool \`scripts/tool.sh\`#" && (cd "$1" && git add -A); }
# RC-3: evidence path with no manifest at/above it
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt && expect "RC-3 evidence dir without manifest -> FAIL" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt && expect "RC-3 summary counts no_manifest" 1 "no_manifest=1" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; echo e3 > "$d/docs/other/d.txt"; addrow "$d" docs/other/d.txt && expect "RC-3 non-evidence path needs no manifest" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; echo e3 > "$d/docs/other/d.txt"; addrow "$d" docs/other/d.txt && expect "RC-3 --evidence-prefix makes it evidence" 1 "NOMANIFEST" "$d" --check changelog --evidence-prefix docs/other/
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt && expect "RC-3 custom prefix replaces the default" 0 "VERDICT: PASS" "$d" --check changelog --evidence-prefix docs/other/
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS.txt ); addrow "$d" docs/qa/run3/d.txt && expect "RC-3 SHA256SUMS.txt accepted as manifest" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS.txt ); echo tamper > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt && expect "RC-3 tampered file under SHA256SUMS.txt -> FAIL" 1 "checksum mismatch" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS.txt ); printf '%s  ../../etc/passwd\n' "$z" >> "$d/docs/qa/run3/SHA256SUMS.txt"; addrow "$d" docs/qa/run3/d.txt && expect "RC-3 escape in SHA256SUMS.txt -> FAIL" 1 "escapes its directory" "$d" --check changelog
# RC-3b: manifest must list every regular file of its tree
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; expect "RC-3b unlisted file -> FAIL" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; expect "RC-3b summary counts unlisted" 1 "unlisted=1" "$d" --check changelog
d=$(fresh); sed -i '/ b.txt$/d' "$d/docs/qa/run1/SHA256SUMS"; echo tampered > "$d/docs/qa/run1/b.txt"; expect "RC-3b entry deleted + file tampered -> FAIL" 1 "UNLISTED  docs/qa/run1/b.txt" "$d" --check changelog
d=$(fresh); echo ignored > "$d/docs/qa/run1/pic.png"; expect "RC-3b gitignored file need not be listed" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); ln -s c.txt "$d/docs/qa/run2/lnk.txt"; expect "RC-3b unlisted symlink is not a regular file" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; expect "RC-3b file in subdir without nested manifest -> FAIL" 1 "UNLISTED  docs/qa/run2/sub/n.txt" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2" && sha256sum sub/n.txt >> SHA256SUMS ); expect "RC-3b subdir file listed with relative path accepted" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; expect "RC-3b nested manifest covers its subtree" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; echo m > "$d/docs/qa/run2/sub/m.txt"; expect "RC-3b nested manifest completeness checked -> FAIL" 1 "UNLISTED  docs/qa/run2/sub/m.txt" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; echo t > "$d/docs/qa/run2/sub/n.txt"; expect "RC-3b nested manifest checksum checked -> FAIL" 1 "docs/qa/run2/sub/n.txt (checksum mismatch)" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; expect "RC-3b manifest files themselves are not required" 0 "unlisted=0" "$d" --check changelog
# waivers: declared, printed, counted, never silent
wv() { mkdir -p "$1/docs/qa"; printf '%s\n' "$2" > "$1/docs/qa/EVIDENCE_WAIVERS.txt"; (cd "$1" && git add -A && git commit -q -m waivers); }
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|scratch note, not evidence'
expect "waiver: unlisted file waived" 0 "WAIVED  docs/qa/run2/extra.txt (not listed in manifest): scratch note, not evidence" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|scratch note'; expect "waiver: summary counts waived=1" 0 "waived=1" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt; wv "$d" 'no-manifest|docs/qa/run3/d.txt|CAT-T-1|2099-12-31|pre-manifest evidence'
expect "waiver: no-manifest waived" 0 "WAIVED  docs/qa/run3/d.txt (no manifest): pre-manifest evidence" "$d" --check changelog
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1 known-bad manifest entry (documented)'
expect "waiver: known-bad checksum entry waived and printed" 0 "WAIVED  docs/qa/run2/c.txt (checksum mismatch): E-1" "$d" --check changelog
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; expect "waiver: known-bad entry without waiver -> FAIL" 1 "checksum mismatch" "$d" --check changelog
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|other.txt|CAT-T-1|2099-12-31|wrong name'; expect "waiver: waiver for a different entry does not apply" 1 "checksum mismatch" "$d" --check changelog
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1'; echo "$z  d2.txt" >> "$d/docs/qa/run2/SHA256SUMS"; (cd "$d" && git add -A); expect "waiver: other failures in the same manifest still FAIL" 1 "d2.txt (missing/unreadable" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2099-12-31|not needed'; expect "waiver: unused waiver is counted, not fatal" 0 "unused_waivers=1" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2099-12-31|'; expect "waiver: missing reason -> FAIL" 1 "BADWAIVER" "$d" --check changelog
d=$(fresh); wv "$d" 'forgive|docs/qa/run2|nothing.txt|why'; expect "waiver: unknown kind -> FAIL" 1 "BADWAIVER" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|a
unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|b'; expect "waiver: duplicate -> FAIL" 1 "duplicate waiver" "$d" --check changelog
d=$(fresh); wv "$d" '# comment

unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|fine'; expect "waiver: comments and blanks ignored" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); printf 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r\n' > "$d/docs/qa/EVIDENCE_WAIVERS.txt"; expect "waiver: untracked waiver file -> BLIND" 2 "not git-tracked" "$d" --check changelog
expect "waiver: --waivers file missing -> BLIND" 2 "not found" "$GOLD" --check changelog --waivers nope.txt
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt; printf 'no-manifest|docs/qa/run3/d.txt|CAT-T-1|2099-12-31|r\n' > "$d/w.txt"; (cd "$d" && git add -A && git commit -q -m waivers)
expect "waiver: --waivers custom path" 0 "waived=1" "$d" --check changelog --waivers w.txt
d=$(fresh); echo "$z  ./c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1 entry written with ./ prefix'; expect "waiver: ./-prefixed manifest entry matches bare waiver name" 0 "waived=1" "$d" --check changelog
# completeness-helper integrity: a stub that forges only the RC-3b helper's output
cmpstub() { local sd; sd="$(mktemp -d "$ROOT/cstub.XXXXXX")"
  printf '#!/bin/sh\nin=$(cat)\nout=$(printf "%%s" "$in" | %s -I "$@")\ncase "$in" in\n*UNLISTED*) printf "%%s\\n" "$out" | { %s; } ;;\n*) printf "%%s\\n" "$out" ;;\nesac\nexit 0\n' "$REALPY" "$1" > "$sd/python3"; chmod +x "$sd/python3"; echo "$sd"; }
sd=$(cmpstub 'cat'); d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; PATH="$sd:$PATH" expect "RC-3b pass-through stub still FAILs on unlisted" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
sd=$(cmpstub 'grep -v "^END"'); d=$(fresh); PATH="$sd:$PATH" expect "RC-3b helper without END -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
sd=$(cmpstub 'grep -v "^UNLISTED"'); d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; PATH="$sd:$PATH" expect "RC-3b helper with UNLISTED row dropped -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
sd=$(cmpstub 'cat; printf "UNLISTED\tforged\n"'); d=$(fresh); PATH="$sd:$PATH" expect "RC-3b helper with forged row after END -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
sd=$(cmpstub 'sed "/^NESTED/d"'); d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; echo m > "$d/docs/qa/run2/sub/m.txt"
PATH="$sd:$PATH" expect "RC-3b helper with NESTED row dropped -> BLIND (nested manifest never skipped silently)" 2 "completeness helper output incomplete" "$d" --check changelog
sd=$(cmpstub 'sed "s/^END.*/END\tx\t0\t0/"'); d=$(fresh); PATH="$sd:$PATH" expect "RC-3b helper with non-numeric END -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
# RC2-1: consecutive block comments after versionCode
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* a *\/ \/* b *\//' && expect "RC2-1 two consecutive block comments parse" 0 "VERDICT: PASS" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* a *\/ \/* b *\/ - 2/' && expect "RC2-1 expression after two comments still FAIL" 1 "FAIL" "$d" --check version
d=$(fresh); mut "$d" catalogizer-android/app/build.gradle.kts 's/versionCode = 7$/versionCode = 7 \/* a *\/ \/* b *\/ \/\/ c/' && expect "RC2-1 two comments then line comment parse" 0 "VERDICT: PASS" "$d" --check version
# RC2-3: the only CR is mid-line
d=$(fresh); printf 'SHA256 (a\r) = %s\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "RC2-3 BSD line with mid-line CR -> FAIL" 1 "CR inside a line" "$d" --check changelog
d=$(fresh); printf '%s  a\r.txt\n' "$z" >> "$d/docs/qa/run1/SHA256SUMS"; expect "RC2-3 GNU line with mid-line CR (no trailing CR) -> FAIL" 1 "CR inside a line" "$d" --check changelog

echo "== review round 1: MANIFEST*.sha256, control chars, path spelling, tracked manifest, waiver schema"
T=$'\t'
mkrun3() { mkdir -p "$1/docs/qa/run3"; echo e3 > "$1/docs/qa/run3/d.txt"; }
# MANIFEST*.sha256 is a manifest
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.sha256 ); addrow "$d" docs/qa/run3/d.txt && expect "MANIFEST.sha256 accepted as manifest" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.screenshots.sha256 ); addrow "$d" docs/qa/run3/d.txt && expect "MANIFEST.<x>.sha256 accepted as manifest" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.sha256 ); echo t > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt && expect "MANIFEST.sha256 checksum enforced -> FAIL" 1 "checksum mismatch" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.sha256 ); echo x > "$d/docs/qa/run3/x.txt"; addrow "$d" docs/qa/run3/d.txt && expect "MANIFEST.sha256 completeness enforced -> FAIL" 1 "UNLISTED  docs/qa/run3/x.txt" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.sha256 ); echo x > "$d/docs/qa/run3/x.txt"; addrow "$d" docs/qa/run3/d.txt && expect "manifest file itself not required in MANIFEST.sha256" 1 "unlisted=1" "$d" --check changelog
# manifest must be git-tracked
d=$(fresh); mkrun3 "$d"; addrow "$d" docs/qa/run3/d.txt; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); expect "untracked manifest does not count -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > MANIFEST.sha256 ); addrow "$d" docs/qa/run3/d.txt; ( cd "$d" && git rm -q --cached docs/qa/run3/MANIFEST.sha256 ); expect "untracked MANIFEST.sha256 does not count -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
# path spelling cannot dodge the evidence prefix
d=$(fresh); mkrun3 "$d"; addrow "$d" docs/./qa/run3/d.txt; expect "path docs/./qa/run3 -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; addrow "$d" docs//qa/run3/d.txt; expect "path docs//qa/run3 -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ln -s qa/run3 "$d/docs/ev"; addrow "$d" docs/ev/d.txt; expect "symlink alias docs/ev -> qa/run3 -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; addrow "$d" docs/qa/run3/../run3/d.txt; expect "path with .. inside evidence -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); ln -s qa/run3 "$d/docs/ev"; addrow "$d" docs/ev/d.txt; expect "symlink alias is not a tracked path as written -> MISS (never a silent pass)" 1 "docs/ev/d.txt (exists but not git-tracked)" "$d" --check changelog
# R6: the evidence prefix directory itself
d=$(fresh); addrow "$d" docs/qa/; expect "R6 VERIFIED docs/qa/ (prefix itself) is evidence -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
# control characters fail closed
d=$(fresh); mkrun3 "$d"; addrow "$d" "docs/qa/run3/x${T}y.txt"; expect "VERIFIED token with TAB is reported as skipped, never silently dropped" 0 "skipped non-repo-looking: docs/qa/run3/x" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/su${T}b"; echo n > "$d/docs/qa/run2/su${T}b/n.txt"; ( cd "$d/docs/qa/run2/su${T}b" && sha256sum n.txt > SHA256SUMS ); ( cd "$d" && git add -A ); expect "nested dir name with TAB -> BLIND (never skipped)" 2 "control character" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/su
b"; echo n > "$d/docs/qa/run2/su
b/n.txt"; ( cd "$d" && git add -A ); expect "nested dir name with LF: its file is reported UNLISTED with LF escaped" 1 'UNLISTED  docs/qa/run2/su\x0ab/n.txt' "$d" --check changelog
# R5: control chars in UNLISTED names are escaped, never raw
d=$(fresh); echo e > "$d/docs/qa/run2/a${T}b.txt"; expect "R5 TAB in unlisted name is escaped" 1 'UNLISTED  docs/qa/run2/a\x09b.txt' "$d" --check changelog
# glob characters in directory names are literal
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir name run[x] is literal (sibling runx not mixed in)" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); echo extra > "$d/docs/qa/run[x]/extra.txt"; addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir name: unlisted file inside is found" 1 'UNLISTED  docs/qa/run[x]/extra.txt' "$d" --check changelog
d=$(fresh); mkrun3 "$d"; addrow "$d" "docs/qa/run3/x"$'\001'"y.txt"; expect "VERIFIED path with a control character -> BLIND" 2 "control character" "$d" --check changelog
# check_manifest_dir never skips a directory it cannot verify (forged NESTED rows via the helper stub)
sd=$(cmpstub 'sed "s/^END\t\([0-9]*\)\t\([0-9]*\)\t0/NESTED\tdocs\/qa\/ghost\nEND\t\1\t\2\t1/"'); d=$(fresh); PATH="$sd:$PATH" expect "nested dir that does not exist -> BLIND" 2 "is missing or has no tracked manifest" "$d" --check changelog
sd=$(cmpstub 'sed "s/^END\t\([0-9]*\)\t\([0-9]*\)\t0/NESTED\tdocs\/qa\/run3\nEND\t\1\t\2\t1/"'); d=$(fresh); mkrun3 "$d"; ( cd "$d" && git add -A ); PATH="$sd:$PATH" expect "nested dir without a tracked manifest -> BLIND" 2 "is missing or has no tracked manifest" "$d" --check changelog
# waiver file must equal HEAD
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r'; echo '# edited' >> "$d/docs/qa/EVIDENCE_WAIVERS.txt"; expect "waiver file edited but uncommitted -> BLIND" 2 "differs from HEAD" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r'; ( cd "$d" && git rm -q --cached docs/qa/EVIDENCE_WAIVERS.txt ); expect "waiver file removed from index -> BLIND" 2 "not git-tracked" "$d" --check changelog
# R1: whitespace-only reason
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|   '; expect "R1 whitespace-only reason (unlisted) -> BADWAIVER" 1 "missing reason" "$d" --check changelog
d=$(fresh); wv "$d" "bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|${T} ${T}"; expect "R1 whitespace-only reason (bad-entry) -> BADWAIVER" 1 "missing reason" "$d" --check changelog
d=$(fresh); wv "$d" 'no-manifest|docs/qa/run3|CAT-T-1|2099-12-31|  '; expect "R1 whitespace-only reason (no-manifest) -> BADWAIVER" 1 "missing reason" "$d" --check changelog
# section 11.4.271: item and expiry are mandatory
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt||2099-12-31|r'; expect "waiver without tracked item -> BADWAIVER" 1 "tracked item id" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|not an id|2099-12-31|r'; expect "waiver with malformed item -> BADWAIVER" 1 "tracked item id" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1||r'; expect "waiver without expiry -> BADWAIVER" 1 "expires" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-13-45|r'; expect "waiver with impossible date -> BADWAIVER" 1 "expires" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|tomorrow|r'; expect "waiver with non-date expiry -> BADWAIVER" 1 "expires" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2020-01-01|r'; expect "expired waiver -> BADWAIVER and gap FAILs" 1 "expired on 2020-01-01" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2026-11-12|r'; out=$(RELCHK_TODAY=2026-11-12 "$CHECKER" --repo "$d" --check changelog 2>&1); rc=$?; if [ "$rc" = 0 ]; then ok; else bad "waiver valid on its expiry day rc=$rc"; fi
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2026-11-12|r'; out=$(RELCHK_TODAY=2026-11-13 "$CHECKER" --repo "$d" --check changelog 2>&1); rc=$?; if [ "$rc" = 1 ]; then ok; else bad "waiver expired the day after rc=$rc"; fi
# PASS-WITH-WAIVERS is distinct
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|r'; expect "relying on a waiver -> VERDICT: PASS-WITH-WAIVERS (exit 0)" 0 "VERDICT: PASS-WITH-WAIVERS" "$d" --check changelog
d=$(fresh); out=$("$CHECKER" --repo "$d" --check changelog 2>&1); case "$out" in *PASS-WITH-WAIVERS*) bad "plain PASS must not say PASS-WITH-WAIVERS" ;; *"VERDICT: PASS "*) ok ;; *) bad "plain PASS missing" ;; esac
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2099-12-31|r'; out=$("$CHECKER" --repo "$d" --check changelog 2>&1); case "$out" in *PASS-WITH-WAIVERS*) bad "unused waiver must not turn PASS into PASS-WITH-WAIVERS" ;; *UNUSED-WAIVER*) ok ;; *) bad "unused waiver not listed" ;; esac
# padded waiver fields are trimmed (kind/path/item/expiry/reason)
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" ' unlisted | docs/qa/run2 | extra.txt | CAT-T-1 | 2099-12-31 |  padded reason  '; expect "padded waiver fields are trimmed" 0 "WAIVED  docs/qa/run2/extra.txt (not listed in manifest): padded reason [item" "$d" --check changelog
# glob metacharacters in a directory name never pull in siblings (git pathspec must be literal)
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A && git rm -q --cached -- ':(literal)docs/qa/run[x]/SHA256SUMS' ); expect "glob dir run[x]: untracked own manifest not satisfied by sibling runx manifest" 1 "NOMANIFEST  docs/qa/run[x]" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; echo x > "$d/docs/qa/runx/extra.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir run[x]: sibling runx files are not unlisted files of run[x]" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); out=$("$CHECKER" --repo "$d" --check changelog 2>&1); case "$out" in *runx*) bad "glob dir run[x]: sibling runx leaked into the listing: $(printf '%s' "$out" | grep runx | head -1)" ;; *) ok ;; esac
# waivers are path-scoped to the resolved path
d=$(fresh); mkrun3 "$d"; addrow "$d" docs/./qa/run3/d.txt; wv "$d" 'no-manifest|docs/qa/run3/d.txt|CAT-T-1|2099-12-31|r'; expect "waiver key is the resolved path (spelling does not matter)" 0 "VERDICT: PASS-WITH-WAIVERS" "$d" --check changelog

echo "== selftest (11.4.273 control needles built into the tool)"
out="$("$CHECKER" --selftest 2>&1)"; got=$?
if [ "$got" = 0 ] && printf '%s\n' "$out" | grep -q '^VERDICT: PASS selftest'; then ok; else bad "selftest rc=$got"; fi

echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
