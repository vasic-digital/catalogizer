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
sd=$(filtstub 'grep -v "^END"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog helper without END -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END.*/END\t99\t0/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog END counts mismatch -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'awk "/^PATH/ && !done {done=1; next} {print}"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog PATH row dropped -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "SECTION\t1.0.0\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog second SECTION -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END.*/&\n&/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog duplicate END -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "BOGUS\tx\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog trailing non-END row -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'awk "/^SKIP/ && !done {done=1; next} {print}"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog SKIP row dropped -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'sed "s/^END/SECTION\t1.0.0\nEND/"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog second SECTION before END -> BLIND" 2 "incomplete" "$d" --check changelog
sd=$(filtstub 'cat; printf "OK\t--prev-tag\tforged\n"'); d=$(fresh); PATH="$sd:$PATH" expect "N1 forged OK on the --prev-tag row -> BLIND" 2 "unexpected rows" "$d" --check version
sd=$(mkstub 'printf "SECTION\t2.4.0\nPATH\tdocs/qa/run2/c.txt\n"; exit 0'); d=$(fresh); PATH="$sd:$PATH" expect "N1 changelog hand-forged output -> BLIND" 2 "incomplete" "$d" --check changelog
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
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/toroot`; Tool `scripts/tool.sh`#' && expect "F3 symlink resolving to the repo root itself accepted" 0 "VERDICT: PASS" "$d" --check changelog
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

echo "== selftest (11.4.273 control needles built into the tool)"
out="$("$CHECKER" --selftest 2>&1)"; got=$?
if [ "$got" = 0 ] && printf '%s\n' "$out" | grep -q '^VERDICT: PASS selftest'; then ok; else bad "selftest rc=$got"; fi

echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
