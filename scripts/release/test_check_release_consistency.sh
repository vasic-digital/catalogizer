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
# chk <dir> [args]: the checker only trusts committed state (a clone sees HEAD), so fixtures are
# auto-committed first (tracked-file edits and staged files); untracked files stay untracked.
# RAW=1 skips the auto-commit for tests that exercise uncommitted state on purpose.
chk() { local dir="$1"; shift; [ -n "${RAW:-}" ] || ( cd "$dir" && git commit -q -a -m auto >/dev/null 2>&1 || true ); "$CHECKER" --repo "$dir" "$@"; }
# expect <label> <rc> <must-contain-or-empty> <dir> [checker args...]
expect() {
  local label="$1" rc="$2" needle="$3" dir="$4"; shift 4
  local out got
  out="$(chk "$dir" "$@" 2>&1)"; got=$?
  if [ "$got" != "$rc" ]; then bad "$label: rc=$got want $rc"; return; fi
  if [ -n "$needle" ] && ! printf '%s\n' "$out" | grep -qF -- "$needle"; then
    bad "$label: output lacks '$needle'"; return; fi
  if ! printf '%s\n' "$out" | grep -q '^VERDICT: '; then bad "$label: no VERDICT line"; return; fi
  ok
}
cm() { ( cd "$1" && git add -A && git commit -q -m case ) ; }
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
d=$(fresh); mut "$d" docs/CHANGELOG.md 's@^- nothing, `docs/unreleased/never_checked.txt`@### VERIFIED\n- `docs/qa/zzz/unreleased_missing.txt`@' && expect "an [Unreleased] heading is not a section: --section Unreleased (even with a VERIFIED part) -> BLIND, not a path check" 2 "VERDICT: BLIND" "$d" --check changelog --section Unreleased
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
if chk "$d" --check changelog 2>&1 | grep -q 'etc/hostname (checksum mismatch'; then bad "F4 escaping manifest must not be executed"; else ok; fi
# F2: every dropped slash-token is counted
d=$(fresh); expect "F2 url and spaced token counted as skipped" 0 "skipped=2" "$d" --check changelog
d=$(fresh); mut "$d" docs/CHANGELOG.md 's#^- Tool#- Glob `docs/qa/*/a.txt` and `~/x/y`; Tool#' && expect "F2 glob and ~ tokens counted" 0 "skipped=4" "$d" --check changelog
# F3: all VERIFIED subsections are read
d=$(fresh); mut "$d" docs/CHANGELOG.md 's@^### REOPENED@\n### VERIFIED (more)\n- `docs/nope_second.txt`\n\n### REOPENED@' && expect "F3 second VERIFIED subsection checked" 1 "docs/nope_second.txt" "$d" --check changelog
# F5
d=$(fresh); mut "$d" docs/api/openapi.yaml 's#^  version: 2.4.0#  version: 2.4.0\n  version: 2.4.0#' && expect "F5 duplicate openapi info.version FAIL" 1 "exactly one info.version" "$d" --check version
expect "F5 empty --version is a usage error" 2 "VERDICT: BLIND" "$GOLD" --version ""
d=$(fresh); : > "$d/catalogizer-android/app/build.gradle"; cm "$d"; expect "F5 both build.gradle and .kts -> FAIL" 1 "ambiguous" "$d" --check version
d=$(fresh); echo q > "$d/\"q\".txt"; (cd "$d" && git add -A); expect "F5 needle survives a quoted tracked name" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); echo b > "$d/.0first.txt"; (cd "$d" && git add -A && git commit -q -m first) ; rm "$d/.0first.txt"; RAW=1 expect "F5 needle survives first tracked file deleted from tree" 0 "VERDICT: PASS" "$d" --check changelog
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
# round 5: the checker reads a snapshot of HEAD, so the sibling shares the SNAPSHOT's path prefix (RELCHK_SNAP_DIR fixes it)
d=$(fresh); sn="$ROOT/snapf3"; mkdir -p "$sn-evil"; echo e > "$sn-evil/f.txt"; ln -s ../../../snapf3-evil "$d/docs/qa/sibling"; (cd "$d" && git add -A)
mut "$d" docs/CHANGELOG.md 's#^- Tool `scripts/tool.sh`#- Link `docs/qa/sibling/`; Tool `scripts/tool.sh`#' && RELCHK_SNAP_DIR="$sn" expect "F3 symlink to sibling dir sharing the snapshot-path prefix -> MISS" 1 "resolves outside the repository" "$d" --check changelog
rm -rf "${sn:?}-evil"
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
d=$(fresh); sn="$ROOT/snapf2"; mkdir -p "$sn-evil2"; echo e > "$sn-evil2/f.txt"; ln -s ../../../../snapf2-evil2 "$d/docs/qa/run1/evil"
echo "$(sha256sum < "$sn-evil2/f.txt" | cut -d' ' -f1)  evil/f.txt" >> "$d/docs/qa/run1/SHA256SUMS"; (cd "$d" && git add -A)
RELCHK_SNAP_DIR="$sn" expect "F2 manifest entry via symlink to sibling dir sharing the snapshot-path prefix FAIL" 1 "resolves outside" "$d" --check changelog
rm -rf "${sn:?}-evil2"
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
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; cm "$d"; expect "RC-3b unlisted file -> FAIL" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; cm "$d"; expect "RC-3b summary counts unlisted" 1 "unlisted=1" "$d" --check changelog
d=$(fresh); sed -i '/ b.txt$/d' "$d/docs/qa/run1/SHA256SUMS"; echo tampered > "$d/docs/qa/run1/b.txt"; expect "RC-3b entry deleted + file tampered -> FAIL" 1 "UNLISTED  docs/qa/run1/b.txt" "$d" --check changelog
d=$(fresh); echo ignored > "$d/docs/qa/run1/pic.png"; expect "RC-3b gitignored file need not be listed" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); ln -s c.txt "$d/docs/qa/run2/lnk.txt"; expect "RC-3b unlisted symlink is not a regular file" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; cm "$d"; expect "RC-3b file in subdir without nested manifest -> FAIL" 1 "UNLISTED  docs/qa/run2/sub/n.txt" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2" && sha256sum sub/n.txt >> SHA256SUMS ); cm "$d"; expect "RC-3b subdir file listed with relative path accepted" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; expect "RC-3b nested manifest covers its subtree" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; echo m > "$d/docs/qa/run2/sub/m.txt"; cm "$d"; expect "RC-3b nested manifest completeness checked -> FAIL" 1 "UNLISTED  docs/qa/run2/sub/m.txt" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; echo t > "$d/docs/qa/run2/sub/n.txt"; expect "RC-3b nested manifest checksum checked -> FAIL" 1 "docs/qa/run2/sub/n.txt (checksum mismatch)" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS ); (cd "$d" && git add -A) ; expect "RC-3b manifest files themselves are not required" 0 "unlisted=0" "$d" --check changelog
# waivers: declared, printed, counted, never silent
wv() { mkdir -p "$1/docs/qa"; printf '%s\n' "$2" > "$1/docs/qa/EVIDENCE_WAIVERS.txt"; (cd "$1" && git add -A && git commit -q -m waivers); }
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|scratch note, not evidence'
expect "waiver: unlisted file waived" 0 "WAIVED  docs/qa/run2/extra.txt (not listed in manifest): scratch note, not evidence" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|scratch note'; expect "waiver: summary counts waived=1" 0 "waived=1" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; addrow "$d" docs/qa/run3/d.txt; wv "$d" 'no-manifest|docs/qa/run3/d.txt|CAT-T-1|2099-12-31|pre-manifest evidence'
expect "waiver: no-manifest waived" 0 "WAIVED  docs/qa/run3/d.txt (no manifest): pre-manifest evidence" "$d" --check changelog
d=$(fresh); echo "$z  gone.txt" >> "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|gone.txt|CAT-T-1|2099-12-31|E-1 manifest entry whose file was never committed'
expect "waiver: missing manifest entry waived and printed" 0 "WAIVED  docs/qa/run2/gone.txt (missing/unreadable): E-1" "$d" --check changelog
# I-2: a bad-entry waiver never excuses a checksum MISMATCH (that is tampering or corruption, not a gap)
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1 laundering attempt'
expect "I-2 bad-entry waiver cannot launder a checksum mismatch" 1 "SHAFAIL  docs/qa/run2/c.txt (checksum mismatch)" "$d" --check changelog
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1 laundering attempt'; out=$(chk "$d" --check changelog 2>&1); case "$out" in *WAIVED*|*PASS-WITH-WAIVERS*) bad "I-2 mismatch must never print WAIVED/PASS-WITH-WAIVERS" ;; *) ok ;; esac
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
d=$(fresh); echo "$z  ./gone.txt" >> "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|gone.txt|CAT-T-1|2099-12-31|E-1 entry written with ./ prefix'; expect "waiver: ./-prefixed manifest entry matches bare waiver name" 0 "waived=1" "$d" --check changelog
# completeness-helper integrity: a stub that forges only the RC-3b helper's output
cmpstub() { local sd; sd="$(mktemp -d "$ROOT/cstub.XXXXXX")"
  printf '#!/bin/sh\nin=$(cat)\nout=$(printf "%%s" "$in" | %s -I "$@")\ncase "$in" in\n*UNLISTED*) printf "%%s\\n" "$out" | { %s; } ;;\n*) printf "%%s\\n" "$out" ;;\nesac\nexit 0\n' "$REALPY" "$1" > "$sd/python3"; chmod +x "$sd/python3"; echo "$sd"; }
sd=$(cmpstub 'cat'); d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; cm "$d"; PATH="$sd:$PATH" expect "RC-3b pass-through stub still FAILs on unlisted" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
sd=$(cmpstub 'grep -v "^END"'); d=$(fresh); PATH="$sd:$PATH" expect "RC-3b helper without END -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
sd=$(cmpstub 'grep -v "^UNLISTED"'); d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; cm "$d"; PATH="$sd:$PATH" expect "RC-3b helper with UNLISTED row dropped -> BLIND" 2 "completeness helper output incomplete" "$d" --check changelog
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
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); ln -s qa/run3 "$d/docs/ev"; addrow "$d" docs/ev/d.txt; expect "CAT-REL-CHECK-2: committed directory-symlink alias into manifested evidence is resolved hop by hop -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); addrow "$d" docs/ev/d.txt; ( cd "$d" && git add -A && git commit -q -m x ); ln -s qa/run3 "$d/docs/ev"
RAW=1 expect "directory-symlink alias that is not committed -> MISS (a clone lacks it), never a silent pass" 1 "MISS  docs/ev/d.txt" "$d" --check changelog
# R6: the evidence prefix directory itself
d=$(fresh); addrow "$d" docs/qa/; expect "R6 VERIFIED docs/qa/ (prefix itself) is evidence -> NOMANIFEST" 1 "NOMANIFEST" "$d" --check changelog
# control characters fail closed
d=$(fresh); mkrun3 "$d"; addrow "$d" "docs/qa/run3/x${T}y.txt"; expect "VERIFIED token with TAB is BLIND (M-2), never silently dropped" 2 "control character" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/su${T}b"; echo n > "$d/docs/qa/run2/su${T}b/n.txt"; ( cd "$d/docs/qa/run2/su${T}b" && sha256sum n.txt > SHA256SUMS ); ( cd "$d" && git add -A ); expect "nested dir name with TAB -> BLIND (never skipped)" 2 "control character" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/su
b"; echo n > "$d/docs/qa/run2/su
b/n.txt"; ( cd "$d" && git add -A ); expect "nested dir name with LF: its file is reported UNLISTED with LF escaped" 1 'UNLISTED  docs/qa/run2/su\x0ab/n.txt' "$d" --check changelog
# R5: control chars in UNLISTED names are escaped, never raw
d=$(fresh); echo e > "$d/docs/qa/run2/a${T}b.txt"; cm "$d"; expect "R5 TAB in unlisted name is escaped" 1 'UNLISTED  docs/qa/run2/a\x09b.txt' "$d" --check changelog
# glob characters in directory names are literal
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir name run[x] is literal (sibling runx not mixed in)" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); echo extra > "$d/docs/qa/run[x]/extra.txt"; addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir name: unlisted file inside is found" 1 'UNLISTED  docs/qa/run[x]/extra.txt' "$d" --check changelog
d=$(fresh); mkrun3 "$d"; addrow "$d" "docs/qa/run3/x"$'\001'"y.txt"; expect "VERIFIED path with a control character -> BLIND" 2 "control character" "$d" --check changelog
# check_manifest_dir never skips a directory it cannot verify (forged NESTED rows via the helper stub)
sd=$(cmpstub 'sed "s/^END\t\([0-9]*\)\t\([0-9]*\)\t0/NESTED\tdocs\/qa\/ghost\nEND\t\1\t\2\t1/"'); d=$(fresh); PATH="$sd:$PATH" expect "nested dir that does not exist -> BLIND" 2 "is missing or has no tracked manifest" "$d" --check changelog
sd=$(cmpstub 'sed "s/^END\t\([0-9]*\)\t\([0-9]*\)\t0/NESTED\tdocs\/qa\/run3\nEND\t\1\t\2\t1/"'); d=$(fresh); mkrun3 "$d"; ( cd "$d" && git add -A ); PATH="$sd:$PATH" expect "nested dir without a tracked manifest -> BLIND" 2 "is missing or has no tracked manifest" "$d" --check changelog
# waiver file must equal HEAD
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r'; echo '# edited' >> "$d/docs/qa/EVIDENCE_WAIVERS.txt"; RAW=1 expect "waiver file edited but uncommitted -> BLIND" 2 "differs from HEAD" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r'; ( cd "$d" && git rm -q --cached docs/qa/EVIDENCE_WAIVERS.txt ); RAW=1 expect "waiver file removed from index -> BLIND" 2 "differs from HEAD" "$d" --check changelog
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
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2026-11-12|r'; out=$(RELCHK_TODAY=2026-11-12 chk "$d" --check changelog 2>&1); rc=$?; if [ "$rc" = 0 ]; then ok; else bad "waiver valid on its expiry day rc=$rc"; fi
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2026-11-12|r'; out=$(RELCHK_TODAY=2026-11-13 chk "$d" --check changelog 2>&1); rc=$?; if [ "$rc" = 1 ]; then ok; else bad "waiver expired the day after rc=$rc"; fi
# PASS-WITH-WAIVERS is distinct
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|extra.txt|CAT-T-1|2099-12-31|r'; expect "relying on a waiver -> VERDICT: PASS-WITH-WAIVERS (exit 0)" 0 "VERDICT: PASS-WITH-WAIVERS" "$d" --check changelog
d=$(fresh); out=$(chk "$d" --check changelog 2>&1); case "$out" in *PASS-WITH-WAIVERS*) bad "plain PASS must not say PASS-WITH-WAIVERS" ;; *"VERDICT: PASS "*) ok ;; *) bad "plain PASS missing" ;; esac
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2099-12-31|r'; out=$(chk "$d" --check changelog 2>&1); case "$out" in *PASS-WITH-WAIVERS*) bad "unused waiver must not turn PASS into PASS-WITH-WAIVERS" ;; *UNUSED-WAIVER*) ok ;; *) bad "unused waiver not listed" ;; esac
# padded waiver fields are trimmed (kind/path/item/expiry/reason)
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" ' unlisted | docs/qa/run2 | extra.txt | CAT-T-1 | 2099-12-31 |  padded reason  '; expect "padded waiver fields are trimmed" 0 "WAIVED  docs/qa/run2/extra.txt (not listed in manifest): padded reason [item" "$d" --check changelog
# glob metacharacters in a directory name never pull in siblings (git pathspec must be literal)
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A && git rm -q --cached -- ':(literal)docs/qa/run[x]/SHA256SUMS' ); expect "glob dir run[x]: untracked own manifest not satisfied by sibling runx manifest" 1 "NOMANIFEST  docs/qa/run[x]" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; echo x > "$d/docs/qa/runx/extra.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); expect "glob dir run[x]: sibling runx files are not unlisted files of run[x]" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run[x]" "$d/docs/qa/runx"; echo a > "$d/docs/qa/run[x]/a.txt"; echo b > "$d/docs/qa/runx/b.txt"; ( cd "$d/docs/qa/run[x]" && sha256sum a.txt > SHA256SUMS ); ( cd "$d/docs/qa/runx" && sha256sum b.txt > SHA256SUMS ); addrow "$d" 'docs/qa/run[x]/a.txt'; ( cd "$d" && git add -A ); out=$(chk "$d" --check changelog 2>&1); case "$out" in *runx*) bad "glob dir run[x]: sibling runx leaked into the listing: $(printf '%s' "$out" | grep runx | head -1)" ;; *) ok ;; esac
# waivers are path-scoped to the resolved path
d=$(fresh); mkrun3 "$d"; addrow "$d" docs/./qa/run3/d.txt; wv "$d" 'no-manifest|docs/qa/run3/d.txt|CAT-T-1|2099-12-31|r'; expect "waiver key is the resolved path (spelling does not matter)" 0 "VERDICT: PASS-WITH-WAIVERS" "$d" --check changelog

echo "== review round 2: HEAD-consistency (I-1), nested/untracked manifests and deep paths (M-1), TAB token (M-2)"
# I-1a: manifest added to the index but never committed -> a clone has none
d=$(fresh); mkdir -p "$d/docs/qa/run3"; echo e3 > "$d/docs/qa/run3/d.txt"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); addrow "$d" docs/qa/run3/d.txt
RAW=1 expect "I-1a staged-but-uncommitted manifest -> BLIND" 2 "not committed" "$d" --check changelog
# I-1b: HEAD has tampered c.txt + old manifest; the working-tree manifest was re-hashed afterwards
d=$(fresh); echo tampered > "$d/docs/qa/run2/c.txt"; ( cd "$d" && git commit -q -a -m tamper ); ( cd "$d/docs/qa/run2" && sha256sum c.txt > SHA256SUMS )
RAW=1 expect "I-1b manifest re-hashed only in the working tree (HEAD tampered) -> BLIND" 2 "not committed" "$d" --check changelog
# I-1b': covered file edited only in the working tree
d=$(fresh); echo tampered > "$d/docs/qa/run2/c.txt"
RAW=1 expect "I-1b' covered file edited only in the working tree -> BLIND" 2 "not committed" "$d" --check changelog
# I-1 negative control: the same committed state passes
d=$(fresh); echo e4 > "$d/docs/qa/run2/e4.txt"; ( cd "$d/docs/qa/run2" && sha256sum c.txt e4.txt > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m ok )
RAW=1 expect "I-1 committed state is clean -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# M-1 / RV2-1: an untracked nested manifest does not exist for a clone, so it must not cover its subtree
d=$(fresh); mkdir -p "$d/docs/qa/run2/sub"; echo n > "$d/docs/qa/run2/sub/n.txt"; ( cd "$d" && git add docs/qa/run2/sub/n.txt && git commit -q -m sub ); ( cd "$d/docs/qa/run2/sub" && sha256sum n.txt > SHA256SUMS )
RAW=1 expect "M-1 untracked nested manifest does not cover its subtree (n.txt UNLISTED)" 1 "UNLISTED  docs/qa/run2/sub/n.txt" "$d" --check changelog
# M-1 / RV2-4: the nearest manifest may be many levels above the cited path
d=$(fresh); mkdir -p "$d/docs/qa/run2/a/b/c"; echo f > "$d/docs/qa/run2/a/b/c/f.txt"; ( cd "$d/docs/qa/run2" && sha256sum a/b/c/f.txt >> SHA256SUMS ); addrow "$d" docs/qa/run2/a/b/c/f.txt
expect "M-1 cited path 4 levels below its manifest finds the manifest" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/qa/run2/a/b/c"; echo f > "$d/docs/qa/run2/a/b/c/f.txt"; ( cd "$d/docs/qa/run2" && sha256sum a/b/c/f.txt >> SHA256SUMS ); addrow "$d" docs/qa/run2/a/b/c/f.txt
expect "M-1 deep path: no NOMANIFEST" 0 "no_manifest=0" "$d" --check changelog
# M-2: a TAB inside a VERIFIED backtick token is a control character -> BLIND, never a skipped token
d=$(fresh); mut "$d" docs/CHANGELOG.md "s#^- Tool \`scripts/tool.sh\`#- Tab \`docs/qa/run1/a.txt\ttail\`; Tool \`scripts/tool.sh\`#" && ( cd "$d" && git add -A )
expect "M-2 TAB in a VERIFIED token -> BLIND" 2 "control character" "$d" --check changelog

echo "== review round 3: HEAD-consistency for changelog, version files and cited paths (R3-1), test gaps (R3-2), refused waiver (R3-4)"
mkother() { mkdir -p "$1/docs/other"; echo x > "$1/docs/other/z.txt"; ( cd "$1" && git add -A && git commit -q -m other ); }
# R3-1a: a cited path outside any manifest tree that is only staged (a clone has no such file)
d=$(fresh); mkdir -p "$d/docs/other"; echo x > "$d/docs/other/z.txt"; addrow "$d" docs/other/z.txt; ( cd "$d" && git commit -q -m cl docs/CHANGELOG.md )
RAW=1 expect "R3-1a cited path only staged -> MISS (a clone lacks it)" 1 "MISS  docs/other/z.txt" "$d" --check changelog
# R3-1b: cited path modified in the working tree only
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; ( cd "$d" && git commit -q -a -m cl ); echo y >> "$d/docs/other/z.txt"
RAW=1 expect "R3-1b cited path edited only in the working tree -> BLIND" 2 "docs/other/z.txt" "$d" --check changelog
# R3-1c: the changelog itself has an uncommitted edit
d=$(fresh); addrow "$d" docs/qa/run1/a.txt
RAW=1 expect "R3-1c uncommitted changelog edit -> BLIND" 2 "docs/CHANGELOG.md" "$d" --check changelog
# R3-1d: a version file edited only in the working tree (valid JSON, same version)
d=$(fresh); echo >> "$d/catalog-web/package.json"
RAW=1 expect "R3-1d version file edited only in the working tree -> BLIND" 2 "catalog-web/package.json" "$d" --check version
# R3-1 negative control: the same state committed passes
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; ( cd "$d" && git commit -q -a -m cl )
RAW=1 expect "R3-1 committed cited path outside manifest trees -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# R3-2 (R-x): a tracked file deleted from the working tree, uncommitted -> MISS (not in working tree)
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; ( cd "$d" && git commit -q -a -m cl ); rm "$d/docs/other/z.txt"
RAW=1 expect "R3-2 tracked file deleted from the tree uncommitted -> MISS" 1 "MISS  docs/other/z.txt (not in working tree)" "$d" --check changelog
# R3-3 (survivor 124): a covered file whose edit is staged only (worktree == index) differs from HEAD
d=$(fresh); echo tampered > "$d/docs/qa/run2/c.txt"; ( cd "$d" && git add -A )
RAW=1 expect "R3-3 covered file edit staged only -> BLIND (not committed)" 2 "not committed" "$d" --check changelog
# R3-3 (survivor 137): the changelog is untracked (absent from HEAD) -> BLIND, not silently read
d=$(fresh); ( cd "$d" && git rm -q --cached docs/CHANGELOG.md && git commit -q -m untrack )
RAW=1 expect "R3-3 changelog not in HEAD -> BLIND" 2 "docs/CHANGELOG.md not found" "$d" --check changelog
# R3-3 (survivor 139): git status failing inside head_check must be BLIND, never silently clean
gs=$(mktemp -d "$ROOT/gstub.XXXXXX"); realgit="$(command -v git)"
printf '#!/bin/sh\ncase " $* " in *" status "*" docs/CHANGELOG.md"*|*" status "*"literal)docs/CHANGELOG.md"*) exit 1;; esac\nexec %s "$@"\n' "$realgit" > "$gs/git"; chmod +x "$gs/git"
d=$(fresh); PATH="$gs:$PATH" RAW=1 expect "R3-3 git status failure in head_check -> BLIND" 2 "git status failed" "$d" --check changelog
# R3-3 (survivor 102): a C1 control character (U+0080) passes the helper regex but not the shell [[:cntrl:]] check
d=$(fresh); mkrun3 "$d"; addrow "$d" "docs/qa/run3/x"$'\xc2\x80'"y.txt"
LC_ALL=C.utf8 expect "R3-3 C1 control char in VERIFIED path -> BLIND" 2 "control character" "$d" --check changelog
echo "== review round 4: cited spelling vs resolved path (I4-1), directory cite (mutant A), every version file kind (mutant C)"
# I4-1 / P1x: symlink committed to a missing target, then retargeted in the working tree only to manifested evidence
d=$(fresh); mkdir -p "$d/docs/other"; ( cd "$d/docs/other" && ln -s ../nowhere ln ); addrow "$d" docs/other/ln; ( cd "$d" && git commit -q -a -m cl ); ( cd "$d/docs/other" && rm ln && ln -s ../qa/run2/c.txt ln )
RAW=1 expect "I4-1 P1x symlink retargeted only in the working tree -> BLIND" 2 "docs/other/ln" "$d" --check changelog
# I4-1 / P2x: a brand-new symlink that is only staged (a clone has no such path)
d=$(fresh); mkdir -p "$d/docs/other"; ( cd "$d/docs/other" && ln -s ../qa/run2/c.txt ln ); addrow "$d" docs/other/ln; ( cd "$d" && git commit -q docs/CHANGELOG.md -m cl )
RAW=1 expect "I4-1 P2x symlink only staged -> MISS (a clone lacks it)" 1 "MISS  docs/other/ln" "$d" --check changelog
# I4-1 negative control: the same symlink committed passes
d=$(fresh); mkdir -p "$d/docs/other"; ( cd "$d/docs/other" && ln -s ../qa/run2/c.txt ln ); addrow "$d" docs/other/ln; ( cd "$d" && git commit -q -a -m cl )
RAW=1 expect "I4-1 committed symlink to manifested evidence -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# mutant A: a cited non-manifest DIRECTORY holding a tracked, uncommitted edit
d=$(fresh); mkother "$d"; addrow "$d" docs/other; ( cd "$d" && git commit -q -a -m cl ); echo y >> "$d/docs/other/z.txt"
RAW=1 expect "R4 cited directory with an uncommitted tracked edit -> BLIND" 2 "docs/other" "$d" --check changelog
# mutant C: an uncommitted working-tree edit of each non-JSON version file kind is BLIND
for vf in docs/api/openapi.yaml catalogizer-desktop/src-tauri/Cargo.toml catalogizer-android/app/build.gradle.kts; do
  d=$(fresh); echo >> "$d/$vf"
  RAW=1 expect "R4 uncommitted edit of $vf -> BLIND" 2 "$vf" "$d" --check version
done

# R3-2 (R-m): a waiver file that is only staged must go BLIND
d=$(fresh); mkdir -p "$d/docs/qa"; printf '%s\n' 'unlisted|docs/qa/run2|x.txt|CAT-T-1|2099-12-31|r' > "$d/docs/qa/EVIDENCE_WAIVERS.txt"; ( cd "$d" && git add docs/qa/EVIDENCE_WAIVERS.txt )
RAW=1 expect "R3-2 staged-only waiver file -> BLIND" 2 "not git-tracked" "$d" --check changelog
# R3-4: a waiver naming a mismatched entry is refused (BADWAIVER), not left as an unused waiver
d=$(fresh); echo "$z  c.txt" > "$d/docs/qa/run2/SHA256SUMS"; wv "$d" 'bad-entry|docs/qa/run2|c.txt|CAT-T-1|2099-12-31|E-1 laundering attempt'
expect "R3-4 mismatch waiver is reported as BADWAIVER" 1 "BADWAIVER" "$d" --check changelog
expect "R3-4 mismatch waiver counted in bad_waivers" 1 "bad_waivers=1" "$d" --check changelog
expect "R3-4 mismatch waiver is not an unused waiver" 1 "unused_waivers=0" "$d" --check changelog

echo "== CAT-REL-CHECK-2: class matrix of working-tree vs clone divergence (docs/release_checker_path_inventory.md)"
# cm <dir>: commit everything staged/unstaged (the committed baseline of a case)
cm() { ( cd "$1" && git add -A && git commit -q -m case ) ; }
# lk <dir> <link-path-rel> <target>: create a symlink (parents made)
lk() { mkdir -p "$(dirname "$1/$2")"; ( cd "$(dirname "$1/$2")" && rm -f "$(basename "$2")" && ln -s "$3" "$(basename "$2")" ); }
# Z1 / M-chain: a 2-hop chain, committed: PASS (negative control)
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../qa/run2/c.txt; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z1 committed 2-hop chain -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# R5-1a: intermediate hop retargeted in the working tree only
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../nowhere; addrow "$d" docs/other/ln; cm "$d"; lk "$d" docs/other/ln2 ../qa/run2/c.txt
RAW=1 expect "Z2 R5-1a middle hop retargeted only in the working tree -> BLIND" 2 "docs/other/ln2" "$d" --check changelog
# R5-1b: intermediate hop only staged (a clone has no such path)
d=$(fresh); lk "$d" docs/other/ln ln2; addrow "$d" docs/other/ln; ( cd "$d" && git add docs/other/ln docs/CHANGELOG.md && git commit -q -m cl ); lk "$d" docs/other/ln2 ../qa/run2/c.txt; ( cd "$d" && git add docs/other/ln2 )
RAW=1 expect "Z3 R5-1b middle hop only staged -> BLIND" 2 "docs/other/ln2" "$d" --check changelog
# middle hop untracked (not even staged)
d=$(fresh); lk "$d" docs/other/ln ln2; addrow "$d" docs/other/ln; ( cd "$d" && git add docs/other/ln docs/CHANGELOG.md && git commit -q -m cl ); lk "$d" docs/other/ln2 ../qa/run2/c.txt
RAW=1 expect "Z4 middle hop untracked -> BLIND" 2 "docs/other/ln2" "$d" --check changelog
# 3 hops, the LAST symlink hop retargeted in the working tree
d=$(fresh); lk "$d" docs/other/a1 a2; lk "$d" docs/other/a2 a3; lk "$d" docs/other/a3 ../qa/run2/c.txt; addrow "$d" docs/other/a1; cm "$d"; lk "$d" docs/other/a3 ../qa/run1/a.txt
RAW=1 expect "Z5 3-hop chain, last hop retargeted -> BLIND" 2 "docs/other/a3" "$d" --check changelog
# middle hop with skip-worktree and a retarget: git status cannot see it
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../nowhere; addrow "$d" docs/other/ln; cm "$d"; lk "$d" docs/other/ln2 ../qa/run2/c.txt; ( cd "$d" && git update-index --skip-worktree docs/other/ln2 )
RAW=1 expect "Z6 m-1 middle hop skip-worktree + retarget -> BLIND" 2 "docs/other/ln2" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../nowhere; addrow "$d" docs/other/ln; cm "$d"; lk "$d" docs/other/ln2 ../qa/run2/c.txt; ( cd "$d" && git update-index --assume-unchanged docs/other/ln2 )
RAW=1 expect "Z7 m-1 middle hop assume-unchanged + retarget -> BLIND" 2 "docs/other/ln2" "$d" --check changelog
# final file: modified + assume-unchanged / skip-worktree (status silent)
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; echo y >> "$d/docs/other/z.txt"; ( cd "$d" && git update-index --assume-unchanged docs/other/z.txt )
RAW=1 expect "Z8 m-1 cited file assume-unchanged + modified -> BLIND" 2 "docs/other/z.txt" "$d" --check changelog
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; echo y >> "$d/docs/other/z.txt"; ( cd "$d" && git update-index --skip-worktree docs/other/z.txt )
RAW=1 expect "Z9 m-1 cited file skip-worktree + modified -> BLIND" 2 "docs/other/z.txt" "$d" --check changelog
# cited directory: a file inside is assume-unchanged + modified
d=$(fresh); mkother "$d"; addrow "$d" docs/other; cm "$d"; echo y >> "$d/docs/other/z.txt"; ( cd "$d" && git update-index --assume-unchanged docs/other/z.txt )
RAW=1 expect "Z10 m-1 cited dir holds an assume-unchanged modified file -> BLIND" 2 "docs/other" "$d" --check changelog
# manifest tree: HEAD holds a covered file that does NOT match its manifest (a clone FAILs); the working tree has the
# matching content and the file is flagged assume-unchanged, so git status is silent and the local run would PASS
d=$(fresh); echo tampered > "$d/docs/qa/run2/c.txt"; ( cd "$d" && git commit -q -a -m tamper ); echo evidence-c > "$d/docs/qa/run2/c.txt"; ( cd "$d" && git update-index --assume-unchanged docs/qa/run2/c.txt )
RAW=1 expect "Z11 m-1 manifest-covered file hides a HEAD mismatch behind assume-unchanged -> BLIND" 2 "assume-unchanged" "$d" --check changelog
# Z11b: the hidden file is covered by the manifest but NOT cited, so only the manifest-tree flag check can see it
d=$(fresh); echo e5 > "$d/docs/qa/run2/e5.txt"; ( cd "$d/docs/qa/run2" && sha256sum c.txt e5.txt > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m e5 ); echo tampered > "$d/docs/qa/run2/e5.txt"; ( cd "$d" && git commit -q -a -m tamper ); echo e5 > "$d/docs/qa/run2/e5.txt"; ( cd "$d" && git update-index --assume-unchanged docs/qa/run2/e5.txt )
RAW=1 expect "Z11b m-1 uncited manifest-covered file hides a HEAD mismatch behind assume-unchanged -> BLIND" 2 "manifest tree" "$d" --check changelog
# Z49: git status trusts the stat cache; a same-size, same-mtime edit is invisible to it (core.checkStat=minimal), only the content hash sees it
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; ( cd "$d" && git config core.checkStat minimal && git config core.trustctime false && git update-index --really-refresh >/dev/null ); touch -r "$d/docs/other/z.txt" "$d/ref.stamp"; printf 'Y\n' > "$d/docs/other/z.txt"; touch -r "$d/ref.stamp" "$d/docs/other/z.txt"
RAW=1 expect "Z49 same-size same-mtime edit invisible to git status -> BLIND (content hash)" 2 "docs/other/z.txt raw bytes differ" "$d" --check changelog
# directory symlink in the middle of the cited path
d=$(fresh); lk "$d" docs/other/dl ../qa/run2; addrow "$d" docs/other/dl/c.txt; cm "$d"
RAW=1 expect "Z12 committed directory symlink mid-path -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/dl ../qa/run2; addrow "$d" docs/other/dl/c.txt; mkdir -p "$d/docs/other2"; echo c > "$d/docs/other2/c.txt"; cm "$d"; lk "$d" docs/other/dl ../other2
RAW=1 expect "Z13 directory symlink retargeted only in the working tree -> BLIND" 2 "docs/other/dl" "$d" --check changelog
d=$(fresh); addrow "$d" docs/other/dl/c.txt; ( cd "$d" && git commit -q docs/CHANGELOG.md -m cl ); lk "$d" docs/other/dl ../qa/run2; ( cd "$d" && git add docs/other/dl )
RAW=1 expect "Z14 directory symlink only staged -> MISS (a clone lacks it)" 1 "MISS  docs/other/dl" "$d" --check changelog
# a committed real directory replaced by a symlink to a copy in the working tree
d=$(fresh); mkdir -p "$d/docs/other/dd" "$d/docs/other/cp"; echo q > "$d/docs/other/dd/f.txt"; echo q > "$d/docs/other/cp/f.txt"; addrow "$d" docs/other/dd/f.txt; cm "$d"; rm -r "$d/docs/other/dd"; lk "$d" docs/other/dd cp
RAW=1 expect "Z15 real directory replaced by a symlink -> BLIND" 2 "docs/other/dd is a directory in HEAD but not in the working tree" "$d" --check changelog
# absolute hop, leaving-the-repo hop, dangling hop, loop
d=$(fresh); lk "$d" docs/other/ln "$d/docs/qa/run2/c.txt"; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z16 absolute symlink hop (even into the repo) -> MISS, never PASS" 1 "MISS  docs/other/ln" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/ln ../../../outside_target; : > "$ROOT/outside_target"; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z17 hop leaving the repo is never PASS" 2 "leaves the repository" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../nowhere; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z18 dangling chain -> MISS" 1 "MISS  docs/other/ln" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ln; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z19 symlink loop -> MISS, never PASS" 1 "MISS  docs/other/ln" "$d" --check changelog
# hop into a submodule, and a clean / dirty gitlink cited directly (m-3)
smk() { local sub="$ROOT/sub.$RANDOM"; mkdir -p "$sub" && ( cd "$sub" && git init -q . && git config user.email a@b.c && git config user.name x && git config commit.gpgsign false && echo hi > f && git add f && git commit -q -m i ) && ( cd "$1" && git -c protocol.file.allow=always submodule add -q "$sub" docs/subm >/dev/null 2>&1 ); }
d=$(fresh); smk "$d"; addrow "$d" docs/subm; cm "$d"
RAW=1 expect "Z20 m-3 clean submodule gitlink cited directly -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); smk "$d"; addrow "$d" docs/subm; cm "$d"; ( cd "$d/docs/subm" && echo more >> f && git commit -q -a -m m )
RAW=1 expect "Z21 submodule checked out at another commit -> BLIND" 2 "docs/subm" "$d" --check changelog
d=$(fresh); smk "$d"; lk "$d" docs/other/ln ../subm/f; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z22 hop into a submodule -> BLIND" 2 "continues inside the submodule docs/subm" "$d" --check changelog
d=$(fresh); smk "$d"; addrow "$d" docs/subm/f; cm "$d"
RAW=1 expect "Z23 path inside a submodule is never PASS (git does not track below a gitlink)" 1 "docs/subm/f" "$d" --check changelog
# case-insensitive collision in a tree level of the path
d=$(fresh); mkdir -p "$d/docs/other"; echo 1 > "$d/docs/other/Foo.txt"; echo 2 > "$d/docs/other/foo.txt"; addrow "$d" docs/other/foo.txt; cm "$d"
RAW=1 expect "Z24 case-insensitive collision of the cited name -> BLIND" 2 "collides" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/OTHER/x" "$d/docs/other/x"; echo 1 > "$d/docs/OTHER/x/f.txt"; echo 2 > "$d/docs/other/x/f.txt"; addrow "$d" docs/other/x/f.txt; cm "$d"
RAW=1 expect "Z25 case-insensitive collision of a parent directory -> BLIND" 2 "collides" "$d" --check changelog
# gitignored hop / path
d=$(fresh); lk "$d" docs/other/ln.png ../qa/run2/c.txt; addrow "$d" docs/other/ln.png; cm "$d"
RAW=1 expect "Z26 gitignored untracked symlink is never PASS" 1 "MISS  docs/other/ln.png" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; echo 1 > "$d/docs/other/f.png"; ( cd "$d" && git add -f docs/other/f.png ); addrow "$d" docs/other/f.png; cm "$d"
RAW=1 expect "Z27 force-added file under an ignore rule, committed -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# paths with spaces and unicode (m-2): extractor + the whole chain
d=$(fresh); mkdir -p "$d/docs/other dir"; echo s > "$d/docs/other dir/a b.txt"; lk "$d" "docs/other dir/l n" "a b.txt"; addrow "$d" "docs/other dir/a b.txt\`, \`docs/other dir/l n"; cm "$d"
RAW=1 expect "Z28 m-2 tracked path and symlink with spaces, committed -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other dir"; echo s > "$d/docs/other dir/a b.txt"; addrow "$d" "docs/other dir/a b.txt"; cm "$d"; echo t >> "$d/docs/other dir/a b.txt"
RAW=1 expect "Z29 m-2 path with a space edited only in the working tree -> BLIND" 2 "a b.txt" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other dir"; echo s > "$d/docs/other dir/a b.txt"; lk "$d" "docs/other dir/l n" "a b.txt"; addrow "$d" "docs/other dir/l n"; cm "$d"; lk "$d" "docs/other dir/l n" ../qa/run2/c.txt
RAW=1 expect "Z30 m-2 symlink with a space retargeted only in the working tree -> BLIND" 2 "l n" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other dir"; echo s > "$d/docs/other dir/a b.txt"; addrow "$d" "docs/other dir/missing b.txt"; cm "$d"
expect "Z31 m-2 untracked name with a space stays a skipped non-path token (honest: indistinguishable from a command)" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/ünï"; echo u > "$d/docs/ünï/ü.txt"; addrow "$d" "docs/ünï/ü.txt"; cm "$d"
RAW=1 expect "Z32 unicode path committed -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/ünï"; echo u > "$d/docs/ünï/ü.txt"; addrow "$d" "docs/ünï/ü.txt"; cm "$d"; echo v >> "$d/docs/ünï/ü.txt"
RAW=1 expect "Z33 unicode path edited only in the working tree -> BLIND" 2 "ü.txt" "$d" --check changelog
# CRLF: worktree converted to CRLF without a clean filter differs from HEAD; with autocrlf it does not
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; printf 'x\r\n' > "$d/docs/other/z.txt"
RAW=1 expect "Z34 CRLF working copy of an LF blob (no autocrlf) -> BLIND" 2 "docs/other/z.txt" "$d" --check changelog
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; ( cd "$d" && git config core.autocrlf true ) && printf 'x\r\n' > "$d/docs/other/z.txt"
RAW=1 expect "Z35 CRLF working copy even with core.autocrlf=true -> BLIND (git status reports it modified; fail-safe)" 2 "docs/other/z.txt" "$d" --check changelog
# committed .. and mid-path ./ spellings (negative controls) and an uncommitted edit seen through them
d=$(fresh); addrow "$d" docs/qa/../qa/run2/c.txt; cm "$d"
RAW=1 expect "Z36 committed '..' spelling -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkother "$d"; addrow "$d" docs/./other/z.txt; cm "$d"
RAW=1 expect "Z37 committed mid-path './' spelling -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkother "$d"; addrow "$d" docs/./other/../other/z.txt; cm "$d"; echo y >> "$d/docs/other/z.txt"
RAW=1 expect "Z38 uncommitted edit seen through a '.'/'..' spelling -> BLIND" 2 "docs/other/z.txt" "$d" --check changelog
d=$(fresh); lk "$d" docs/other/ln ../qa/../qa/run2/./c.txt; addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z39 committed link target with '..' and '.' -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# directory symlink + '..' after it follows kernel semantics (through the target, not lexically)
d=$(fresh); mkdir -p "$d/docs/other/real/inner"; echo k > "$d/docs/other/real/k.txt"; echo i > "$d/docs/other/real/inner/i.txt"; lk "$d" docs/other/dl real/inner; addrow "$d" docs/other/dl/../k.txt; cm "$d"
RAW=1 expect "Z40 '..' after a directory symlink resolves through the link target -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# RV5-4: HEAD membership with several specs (version files), where git status cannot see it
d=$(fresh); ( cd "$d" && git rm -q --cached catalog-web/package.json && git commit -q -m untrack )
RAW=1 expect "Z41 RV5-4 one of several version files not in HEAD -> FAIL (a clone lacks it)" 1 "catalog-web/package.json version: cannot read" "$d" --check version
# a symlink hop of HEAD replaced by a regular file with the same content (type differs)
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../qa/run2/c.txt; addrow "$d" docs/other/ln; cm "$d"; rm "$d/docs/other/ln2"; cp "$d/docs/qa/run2/c.txt" "$d/docs/other/ln2"
RAW=1 expect "Z45 symlink hop replaced by a regular file -> BLIND" 2 "docs/other/ln2 is a symlink in HEAD but not in the working tree" "$d" --check changelog
# a file of HEAD replaced by a symlink to identical content (final entry type differs)
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"; cp "$d/docs/other/z.txt" "$d/docs/other/zz.txt"; rm "$d/docs/other/z.txt"; ln -s zz.txt "$d/docs/other/z.txt"
RAW=1 expect "Z46 file replaced by a symlink to the same content -> BLIND" 2 "docs/other/z.txt is a file in HEAD but not in the working tree" "$d" --check changelog
# glob characters in a tracked name are literal
d=$(fresh); mkdir -p "$d/docs/other"; echo 1 > "$d/docs/other/f[1].txt"; echo 2 > "$d/docs/other/f1.txt"; addrow "$d" "docs/other/f[1].txt"; cm "$d"
RAW=1 expect "Z47 glob characters in a tracked name are literal -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; echo 1 > "$d/docs/other/f[1].txt"; echo 2 > "$d/docs/other/f1.txt"; addrow "$d" "docs/other/f[1].txt"; cm "$d"; echo more >> "$d/docs/other/f[1].txt"
RAW=1 expect "Z48 edit of a glob-named file -> BLIND" 2 "f[1].txt" "$d" --check changelog
# --version X without a matching changelog section must not fall back to the topmost section
d=$(fresh); expect "Z42 --version with no matching changelog section -> BLIND" 2 "section not found: 2.9.9" "$d" --check changelog --version 2.9.9
d=$(fresh); expect "Z43 --version selects its own section (negative control)" 0 "section=2.4.0" "$d" --check changelog --version 2.4.0
d=$(fresh); expect "Z44 explicit --section wins over --version" 0 "section=2.4.0" "$d" --check changelog --version 2.9.9 --section 2.4.0

echo "== CAT-REL-CHECK-2 round 2 (F1 root-resolving cited path, F2 raw hop bytes + hop entry state, F3 local filters, F6)"
# F1: a cited path that resolves to the repository root was exempt from the HEAD walk
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); lk "$d" docs/r qa/run3; addrow "$d" docs/r; cm "$d"; lk "$d" docs/r ..
RAW=1 expect "Z50 F1 committed docs/r -> qa/run3 retargeted to .. (root) in the working tree -> BLIND" 2 "docs/r link target differs from HEAD" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); lk "$d" docs/qa/al run3; addrow "$d" docs/qa/al; cm "$d"; lk "$d" docs/qa/al ../..
RAW=1 expect "Z51 F1 committed docs/qa/al -> run3 retargeted to ../.. (root) -> BLIND" 2 "docs/qa/al link target differs from HEAD" "$d" --check changelog
d=$(fresh); mkrun3 "$d"; ( cd "$d/docs/qa/run3" && sha256sum d.txt > SHA256SUMS ); lk "$d" docs/r qa/run3; addrow "$d" docs/r; cm "$d"
RAW=1 expect "Z52 F1 negative control: the committed alias is untouched -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# F2: hop target bytes are compared raw (newlines survive), and the forged target is a distinct string
d=$(fresh); mkother "$d"; echo x > "$d/docs/other/x"; lk "$d" docs/other/ln x; addrow "$d" docs/other/ln; cm "$d"
rm "$d/docs/other/ln"; ( cd "$d/docs/other" && ln -s $'x\n' ln && echo FORGED > $'x\n' )
RAW=1 expect "Z53 F2 target x -> 'x<newline>' with an untracked forged file -> BLIND" 2 "docs/other/ln link target differs from HEAD" "$d" --check changelog
d=$(fresh); mkother "$d"; echo x > "$d/docs/other/x"; ( cd "$d/docs/other" && ln -s $'x\nfoo' ln && echo y > $'x\nfoo' ); addrow "$d" docs/other/ln; cm "$d"
RAW=1 expect "Z54 F2 committed target with a newline -> BLIND (unsupported)" 2 "contains a control character or newline" "$d" --check changelog
d=$(fresh); mkother "$d"; echo x > "$d/docs/other/x"; ( cd "$d/docs/other" && ln -s $'x\n' ln ); addrow "$d" docs/other/ln; cm "$d"; ( cd "$d/docs/other" && rm ln && ln -s x ln )
RAW=1 expect "Z55 F2 committed target 'x<newline>' vs working tree x -> BLIND" 2 "docs/other/ln link target differs from HEAD" "$d" --check changelog
# F2: a hop is a tracked entry: a skip-worktree flag on an otherwise identical hop hides it from git status
d=$(fresh); lk "$d" docs/other/ln ln2; lk "$d" docs/other/ln2 ../qa/run2/c.txt; addrow "$d" docs/other/ln; cm "$d"; ( cd "$d" && git update-index --skip-worktree docs/other/ln2 )
RAW=1 expect "Z56 F2 identical hop flagged skip-worktree -> BLIND" 2 "docs/other/ln2 holds a skip-worktree" "$d" --check changelog
# F3: local clean filter / info attributes: git status clean, raw bytes differ from the blob a clone gets
d=$(fresh); mkdir -p "$d/docs/other"; ( cd "$d" && git config filter.crx.clean "tr -d '\\r'" && printf '*.txt filter=crx\n' >> .git/info/attributes ); printf 'a\r\n' > "$d/docs/other/f.txt"; addrow "$d" docs/other/f.txt; cm "$d"
RAW=1 expect "Z57 F3 local clean filter hides CR bytes (status clean) -> BLIND" 2 "raw bytes differ from the HEAD blob" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; ( cd "$d" && git config core.autocrlf true ); printf 'a\r\n' > "$d/docs/other/f.txt"; addrow "$d" docs/other/f.txt; cm "$d"
RAW=1 expect "Z58 F3 core.autocrlf=true after git add (status clean) -> BLIND" 2 "raw bytes differ from the HEAD blob" "$d" --check changelog
d=$(fresh); mkdir -p "$d/docs/other"; printf 'a\r\n' > "$d/docs/other/f.txt"; addrow "$d" docs/other/f.txt; cm "$d"
RAW=1 expect "Z59 F3 negative control: committed CRLF bytes, no filter -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# F6: inherited GIT_* environment must not redirect the checker
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"
out="$(GIT_INDEX_FILE="$ROOT/bogus-index" GIT_DIR="$ROOT/bogus-gitdir" "$CHECKER" --repo "$d" --check changelog 2>&1)"; got=$?
if [ "$got" = 0 ] && printf '%s\n' "$out" | grep -q '^VERDICT: PASS'; then ok; else bad "Z60 F6 inherited GIT_DIR/GIT_INDEX_FILE changed the verdict (rc=$got)"; fi
# F6: no --version/--section: the section comes from versions.json, never the topmost heading
d=$(fresh); mut "$d" docs/CHANGELOG.md '0,/^## \[2\.4\.0\]/s//## [9.9.9] - next\n\n- nothing\n\n## [2.4.0]/'
expect "Z61 F6 topmost section 9.9.9 is not used without --version (derived 2.4.0)" 0 "section=2.4.0" "$d" --check changelog
d=$(fresh); ( cd "$d" && git rm -q versions.json && git commit -q -m novj )
RAW=1 expect "Z62 F6 no --version and no versions.json -> BLIND, not the topmost section" 2 "refusing to use the topmost changelog section" "$d" --check changelog

# Reviewer own-mutant follow-ups OWN1-OWN6 (each has a paired mutant CC2r2 OWNn)
# gitstub <fail-word>: a git wrapper that fails when any argument equals <fail-word>, else runs the real git
REALGIT="$(command -v git)"
gitstub() { local sd; sd="$(mktemp -d "$ROOT/gstub.XXXXXX")"; printf '#!/bin/sh\nfor a in "$@"; do [ "$a" = "%s" ] && exit 1; done\nexec %s "$@"\n' "$1" "$REALGIT" > "$sd/git"; chmod +x "$sd/git"; echo "$sd"; }
gitstub2() { local sd; sd="$(mktemp -d "$ROOT/gstub.XXXXXX")"; printf '#!/bin/sh\n%s\nexec %s "$@"\n' "$1" "$REALGIT" > "$sd/git"; chmod +x "$sd/git"; echo "$sd"; }
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"
sd=$(gitstub2 'case " $* " in *" --name-only HEAD -- "*) exit 1;; esac'); PATH="$sd:$PATH" RAW=1 expect "Z63 OWN1 git failing in the case-collision listing -> BLIND" 2 "git ls-tree failed" "$d" --check changelog
d=$(fresh); mkother "$d"; addrow "$d" docs/other/z.txt; cm "$d"
sd=$(gitstub hash-object); PATH="$sd:$PATH" RAW=1 expect "Z64 OWN6 git hash-object failing -> BLIND" 2 "git hash-object failed for docs/other/z.txt" "$d" --check changelog
d=$(fresh); mkother "$d"; echo q > "$d/docs/other/a"; cm "$d"; rm "$d/docs/other/a"; mkdir "$d/docs/other/a"; echo b > "$d/docs/other/a/b"; addrow "$d" docs/other/a/b; ( cd "$d" && git commit -q docs/CHANGELOG.md -m cl )
RAW=1 expect "Z65 OWN2 HEAD file replaced by a directory that the cited path goes through -> MISS (HEAD snapshot has a file there)" 1 "MISS  docs/other/a/b" "$d" --check changelog
d=$(fresh); mkother "$d"; e="$(printf '' | git -C "$d" hash-object -w --stdin)"; ( cd "$d" && git update-index --add --cacheinfo "120000,$e,docs/other/e" && git commit -q -m emptylink ); echo 1 > "$d/docs/other/x"; ln -s x "$d/docs/other/e"; addrow "$d" docs/other/e
RAW=1 expect "Z66 OWN3 empty symlink in HEAD -> BLIND (the snapshot cannot hold it)" 2 "cannot write the HEAD snapshot" "$d" --check changelog
# OWN4 (unsupported HEAD mode) is an equivalent mutant: git ls-tree canonicalises every regular-file mode to 100644/100755 (measured with a literal tree holding 100664), so the `*)` branch is defensive only.
d=$(fresh); smk "$d"; addrow "$d" docs/subm; cm "$d"; rm -rf "${d:?}/docs/subm"; echo f > "$d/docs/subm"
RAW=1 expect "Z68 OWN5 submodule replaced by a regular file -> BLIND" 2 "docs/subm is a submodule in HEAD but not a directory" "$d" --check changelog

echo "== CAT-REL-CHECK-2 round 3 (N1 manifest tree raw compare, N4 eol attribute)"
# N1a: HEAD holds a tampered, uncited manifest-covered file; a local clean filter makes the restored good bytes look clean
d=$(fresh); echo e5 > "$d/docs/qa/run2/e5.txt"; ( cd "$d/docs/qa/run2" && sha256sum c.txt e5.txt > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m e5 )
( cd "$d" && git config filter.x.clean 'sed s/evidence-b/tampered-b/' && git config filter.x.clean "sed s/e5/tampered/" && printf 'docs/qa/run2/e5.txt filter=x\n' > .git/info/attributes ); touch "$d/docs/qa/run2/e5.txt"; ( cd "$d" && git add docs/qa/run2/e5.txt && git commit -q -m tamper ); echo e5 > "$d/docs/qa/run2/e5.txt"
RAW=1 expect "Z69 N1a tampered HEAD blob hidden by a local clean filter (status clean) -> BLIND" 2 "docs/qa/run2/e5.txt raw bytes differ from the HEAD blob" "$d" --check changelog
# N1b: no filter; same-size in-place overwrite with the mtime restored, core.trustctime=false and checkStat=minimal
d=$(fresh); echo e5 > "$d/docs/qa/run2/e5.txt"; ( cd "$d/docs/qa/run2" && sha256sum c.txt e5.txt > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m e5 )
( cd "$d" && git config core.checkStat minimal && git config core.trustctime false ); echo XX > "$d/docs/qa/run2/e5.txt"; sleep 2; ( cd "$d" && git commit -q -a -m tamper ); touch -r "$d/docs/qa/run2/e5.txt" "$d/ref.stamp"; ( cd "$d" && git update-index --really-refresh >/dev/null )
printf 'e5\n' | dd of="$d/docs/qa/run2/e5.txt" conv=notrunc status=none; touch -r "$d/ref.stamp" "$d/docs/qa/run2/e5.txt"
RAW=1 expect "Z70 N1b same-size overwrite with restored mtime (status clean) -> BLIND" 2 "docs/qa/run2/e5.txt raw bytes differ from the HEAD blob" "$d" --check changelog
# negative control: untouched manifest tree
d=$(fresh); echo e5 > "$d/docs/qa/run2/e5.txt"; ( cd "$d/docs/qa/run2" && sha256sum c.txt e5.txt > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m e5 )
RAW=1 expect "Z72 N1 negative control: committed manifest tree, untouched -> PASS" 0 "VERDICT: PASS" "$d" --check changelog
# N1: a manifest entry reached through a symlink to a file outside the tree is walked against HEAD
d=$(fresh); mkother "$d"; echo y >> "$d/docs/other/z.txt"; ( cd "$d/docs/qa/run2" && ln -s ../../other/z.txt zl.txt && sha256sum zl.txt >> SHA256SUMS ); ( cd "$d" && git add docs/qa/run2 && git commit -q -m zl )
RAW=1 expect "Z73 N1 manifest entry via symlink to a file modified only in the working tree -> SHAFAIL like a clone" 1 "SHAFAIL  docs/qa/run2/zl.txt" "$d" --check changelog
# N4: a committed eol attribute makes the cited file BLIND (accepted limit); the message names the resolution
d=$(fresh); mkdir -p "$d/docs/other"; printf '*.txt text eol=crlf\n' > "$d/.gitattributes"; printf 'a\r\n' > "$d/docs/other/f.txt"; addrow "$d" docs/other/f.txt; cm "$d"; printf 'a\r\n' > "$d/docs/other/f.txt"
RAW=1 expect "Z71 N4 committed eol=crlf attribute -> BLIND with the resolution wording" 2 "to resolve, commit the bytes exactly as they are in the working tree or remove the eol/filter attribute" "$d" --check changelog
# N2: --help states the derived default
out="$("$CHECKER" --help 2>&1)"; if printf '%s\n' "$out" | grep -q 'versions.json global' && ! printf '%s\n' "$out" | grep -q 'default: topmost numbered section'; then ok; else bad "Z74 N2 --help still advertises the topmost-section default"; fi

echo "== CAT-REL-CHECK-2 round 4 (R3-1 gitlinks, R3-2 stat-cache symlink, R3-2 stub branches)"
# smk2 <repo> <path>: adds a submodule with f='good' at <path> (committed in the parent by the caller)
smk2() { local sub="$ROOT/sub.$RANDOM"; mkdir -p "$sub" && ( cd "$sub" && git init -q . && git config user.email a@b.c && git config user.name x && git config commit.gpgsign false && printf good > f && git add f && git commit -q -m i ) && ( cd "$1" && git -c protocol.file.allow=always submodule add -q "$sub" "$2" >/dev/null 2>&1 ); }
# R3-1 clean: manifest lists a file below a submodule; status is clean but a plain clone has no such file
d=$(fresh); smk2 "$d" docs/qa/run2/sub; ( cd "$d/docs/qa/run2" && sha256sum c.txt sub/f > SHA256SUMS ); cm "$d"
RAW=1 expect "Z75 R3-1 manifest tree holding a submodule (clean, manifest lists sub/f) -> BLIND" 2 "contains the submodule" "$d" --check changelog
# R3-1 .gitmodules ignore=all hides a modified submodule file the manifest lists
d=$(fresh); smk2 "$d" docs/qa/run2/sub; ( cd "$d" && git config -f .gitmodules submodule.docs/qa/run2/sub.ignore all ); printf evil > "$d/docs/qa/run2/sub/f"; ( cd "$d/docs/qa/run2" && sha256sum c.txt sub/f > SHA256SUMS ); cm "$d"
RAW=1 expect "Z76 R3-1 .gitmodules ignore=all + modified submodule file -> BLIND (dirty, not committed)" 2 "has changes that are not committed" "$d" --check changelog
# R3-1 local-config variant
d=$(fresh); smk2 "$d" docs/qa/run2/sub; ( cd "$d" && git config diff.ignoreSubmodules all ); printf evil > "$d/docs/qa/run2/sub/f"; ( cd "$d/docs/qa/run2" && sha256sum c.txt sub/f > SHA256SUMS ); cm "$d"
RAW=1 expect "Z77 R3-1 local diff.ignoreSubmodules=all + modified submodule file -> BLIND (dirty, not committed)" 2 "has changes that are not committed" "$d" --check changelog
# R3-1 cited gitlink itself, dirty submodule hidden by ignore=all
d=$(fresh); smk2 "$d" docs/subm; ( cd "$d" && git config -f .gitmodules submodule.docs/subm.ignore all ); addrow "$d" docs/subm; cm "$d"; printf evil > "$d/docs/subm/f"
RAW=1 expect "Z78 R3-1 cited gitlink, ignore=all, modified submodule file -> BLIND" 2 "docs/subm differs from HEAD" "$d" --check changelog
d=$(fresh); smk2 "$d" docs/subm; ( cd "$d" && git config diff.ignoreSubmodules all ); addrow "$d" docs/subm; cm "$d"; printf evil > "$d/docs/subm/f"
RAW=1 expect "Z79 R3-1 cited gitlink, local diff.ignoreSubmodules=all, modified submodule file -> BLIND" 2 "docs/subm differs from HEAD" "$d" --check changelog
# R3-1 cited file below a gitlink (direct cite and through the cited-path walk) is never PASS
d=$(fresh); smk2 "$d" docs/subm; ( cd "$d" && git config -f .gitmodules submodule.docs/subm.ignore all ); lk "$d" docs/other/ln2 ../subm/f; addrow "$d" docs/other/ln2; cm "$d"
RAW=1 expect "Z80 R3-1 cited symlink into a submodule with ignore=all -> BLIND" 2 "continues inside the submodule docs/subm" "$d" --check changelog
# R3-1 a manifest entry reached through a symlink into a submodule
d=$(fresh); smk2 "$d" docs/subm; ( cd "$d/docs/qa/run2" && ln -s ../../subm/f sl.txt && sha256sum sl.txt >> SHA256SUMS ); cm "$d"
RAW=1 expect "Z81 R3-1 manifest entry via symlink into a submodule -> SHAFAIL like a clone" 1 "SHAFAIL  docs/qa/run2/sl.txt" "$d" --check changelog

# R3-2: a tracked symlink in the manifest tree retargeted with the same length, stat cache fooled
d=$(fresh); ( cd "$d/docs/qa/run2" && ln -s aa lnk ); ( cd "$d" && git add -A && git commit -q -m lnk && git config core.checkStat minimal && git config core.trustctime false )
( cd "$d/docs/qa/run2" && ln -sfn bb lnk ); sleep 2; ( cd "$d" && git commit -q -a -m retarget && git update-index --really-refresh >/dev/null ); m="$(stat -c %.9Y "$d/docs/qa/run2/lnk")"
( cd "$d/docs/qa/run2" && ln -sfn aa lnk && touch -h -d "@$m" lnk ); st="$(git -C "$d" status --porcelain --untracked-files=no)"
if [ -n "$st" ]; then bad "Z82 fixture: git status is not clean ($st)"; else RAW=1 expect "Z82 R3-2 same-length symlink retarget hidden from git status -> BLIND" 2 "symlink docs/qa/run2/lnk differs from HEAD" "$d" --check changelog; fi
# R3-2: newline in a tracked file name under the manifest tree (hashed on its own), edit hidden from git status
d=$(fresh); printf 'e5\n' > "$d/docs/qa/run2/$(printf 'nl\nx.txt')"; ( cd "$d" && git add -A && git commit -q -m nl && git config core.checkStat minimal && git config core.trustctime false )
printf 'XX\n' > "$d/docs/qa/run2/$(printf 'nl\nx.txt')"; sleep 2; ( cd "$d" && git commit -q -a -m tamper && git update-index --really-refresh >/dev/null ); touch -r "$d/docs/qa/run2/$(printf 'nl\nx.txt')" "$d/ref.stamp"
printf 'e5\n' | dd of="$d/docs/qa/run2/$(printf 'nl\nx.txt')" conv=notrunc status=none; touch -r "$d/ref.stamp" "$d/docs/qa/run2/$(printf 'nl\nx.txt')"
st="$(git -C "$d" status --porcelain --untracked-files=no)"
if [ -n "$st" ]; then bad "Z83 fixture: git status is not clean ($st)"; else RAW=1 expect "Z83 R3-2 newline-named file with a hidden edit -> BLIND" 2 "nl\\nx.txt' raw bytes differ from the HEAD blob" "$d" --check changelog; fi
# R3-2: stdin-paths hash count mismatch and the ls-tree probe (git wrappers)
gitstub2() { local sd; sd="$(mktemp -d "$ROOT/gstub.XXXXXX")"; printf '#!/bin/sh\n%s\nexec %s "$@"\n' "$1" "$REALGIT" > "$sd/git"; chmod +x "$sd/git"; echo "$sd"; }
d=$(fresh); sd=$(gitstub2 'for a in "$@"; do if [ "$a" = "--stdin-paths" ]; then cat >/dev/null; echo 0000000000000000000000000000000000000000; exit 0; fi; done')
PATH="$sd:$PATH" RAW=1 expect "Z84 R3-2 hash-object returns fewer hashes than files -> BLIND" 2 "hashes for" "$d" --check changelog
d=$(fresh); sd=$(gitstub2 'case "$*" in *" ls-tree HEAD -- docs/qa/run2/") exit 1 ;; esac')
PATH="$sd:$PATH" RAW=1 expect "Z85 R3-2 failing ls-tree probe of the manifest tree -> BLIND" 2 "cannot prove the manifest tree matches HEAD" "$d" --check changelog

echo "== CAT-REL-CHECK-2 round 5 (HEAD is the single source of every read; C1-C8 each compared with a plain clone)"
# cc <label> <local-rc> <needle> <dir> [checker args]: the local verdict must be <local-rc> AND must never be a
# PASS when a plain `git clone` of the same repository (HEAD only) is not one.
cc() {
  local label="$1" rc="$2" needle="$3" dir="$4" crc lrc cl; shift 4
  RAW=1 expect "$label" "$rc" "$needle" "$dir" "$@"
  cl="$dir.clone"; git clone -q "$dir" "$cl" >/dev/null 2>&1 || { bad "$label: clone failed"; return; }
  "$CHECKER" --repo "$cl" "$@" >/dev/null 2>&1; crc=$?
  "$CHECKER" --repo "$dir" "$@" >/dev/null 2>&1; lrc=$?
  if [ "$crc" -ne 0 ] && [ "$lrc" -eq 0 ]; then bad "$label: local PASS but a plain clone gives rc=$crc"; else ok; fi
}
# C1: HEAD's manifest names a wrong checksum; the working tree and the index no longer have the manifest at all
d=$(fresh); ( cd "$d/docs/qa/run2" && printf '%064d  c.txt\n' 0 > SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m badsum && git rm -q docs/qa/run2/SHA256SUMS )
cc "Z86 C1 manifest deleted from index and worktree, HEAD's checksum is wrong -> never PASS (BLIND: worktree differs from HEAD)" 2 "has changes that are not committed" "$d" --check changelog
# C2: a manifest entry names a file that exists only as an untracked file in the working tree
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; ( cd "$d/docs/qa/run2" && sha256sum extra.txt >> SHA256SUMS ); ( cd "$d" && git add docs/qa/run2/SHA256SUMS && git commit -q -m entry )
cc "Z87 C2 manifest entry names an untracked working-tree file -> FAIL (a clone lacks it)" 1 "docs/qa/run2/extra.txt (missing/unreadable, not gitignored)" "$d" --check changelog
# C3: the working-tree waiver file differs from HEAD's, hidden from `git diff HEAD`
w3() { d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2099-12-31|r'; sed -i 's/nothing.txt/extra.txt/' "$d/docs/qa/EVIDENCE_WAIVERS.txt"; }
w3; ( cd "$d" && git update-index --assume-unchanged docs/qa/EVIDENCE_WAIVERS.txt )
cc "Z88 C3a waiver edited in the worktree behind assume-unchanged -> HEAD's waivers rule" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
w3; ( cd "$d" && git update-index --skip-worktree docs/qa/EVIDENCE_WAIVERS.txt )
cc "Z89 C3b waiver edited in the worktree behind skip-worktree -> HEAD's waivers rule" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
w3; ( cd "$d" && git config filter.wv.clean 'sed s/extra.txt/nothing.txt/' && printf 'docs/qa/EVIDENCE_WAIVERS.txt filter=wv\n' > .git/info/attributes )
cc "Z90 C3c waiver edited in the worktree behind a local clean filter -> HEAD's waivers rule" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
# C4: .git/info/exclude or core.excludesFile must not turn a missing entry into UNVERIFIABLE
d=$(fresh); echo 'gone.txt' >> "$d/.git/info/exclude"; ( cd "$d/docs/qa/run2" && printf '%s  gone.txt\n' "$(printf g | sha256sum | cut -d' ' -f1)" >> SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m gone )
cc "Z91 C4 .git/info/exclude names the missing entry -> still SHAFAIL (only the committed .gitignore counts)" 1 "docs/qa/run2/gone.txt (missing/unreadable, not gitignored)" "$d" --check changelog
d=$(fresh); echo 'gone.txt' > "$d/.git/myexcludes"; ( cd "$d" && git config core.excludesFile "$d/.git/myexcludes" ); ( cd "$d/docs/qa/run2" && printf '%s  gone.txt\n' "$(printf g | sha256sum | cut -d' ' -f1)" >> SHA256SUMS ); ( cd "$d" && git add -A && git commit -q -m gone )
cc "Z92 C4 core.excludesFile names the missing entry -> still SHAFAIL" 1 "docs/qa/run2/gone.txt (missing/unreadable, not gitignored)" "$d" --check changelog
# C5: both gradle files are in HEAD; the working tree lost one of them
d=$(fresh); cp "$d/catalogizer-android/app/build.gradle.kts" "$d/catalogizer-android/app/build.gradle"; ( cd "$d" && git add -A && git commit -q -m both ); rm "$d/catalogizer-android/app/build.gradle"
cc "Z93 C5 build.gradle and build.gradle.kts both in HEAD, one deleted locally -> FAIL as in a clone" 1 "both build.gradle.kts and build.gradle exist" "$d" --check version
# C6: the tracked-path sets come from HEAD: an index-only top-level directory must not make a token a path
d=$(fresh); mkdir -p "$d/zzz"; echo q > "$d/zzz/new.txt"; ( cd "$d" && git add zzz/new.txt ); addrow "$d" zzz/new.txt; ( cd "$d" && git add -A && git reset -q zzz/new.txt && git commit -q -m c6 ); ( cd "$d" && git add zzz/new.txt )
cc "Z94 C6 index-only top-level directory does not turn a token into a path -> same verdict as a clone" 0 "skipped" "$d" --check changelog
o1="$("$CHECKER" --repo "$d" --check changelog 2>&1 | grep '^VERDICT' | sed 's/reason:.*//')"; o2="$("$CHECKER" --repo "$d.clone" --check changelog 2>&1 | grep '^VERDICT' | sed 's/reason:.*//')"
if [ "$o1" = "$o2" ] && [ -n "$o1" ]; then ok; else bad "Z94b C6 local verdict differs from the clone's ($o1 vs $o2)"; fi
# C7: --check changelog derives the section from versions.json; a local edit must neither be used nor go unnoticed
d=$(fresh); sed -i 's/"minor": 4/"minor": 3/' "$d/versions.json"
cc "Z95 C7 versions.json edited locally, --check changelog without --version -> HEAD's section, BLIND for the edit" 2 "section 2.4.0:" "$d" --check changelog
out="$(RAW=1 "$CHECKER" --repo "$d" --check changelog 2>&1)"; if printf '%s\n' "$out" | grep -qF 'versions.json'; then ok; else bad "Z95b C7 the edited versions.json is not reported"; fi
# C8: a replace ref must not change what is read; the local repository "heals" HEAD, a plain clone does not get the ref
d=$(fresh); A="$(git -C "$d" rev-parse HEAD)"; echo tampered > "$d/docs/qa/run2/c.txt"; ( cd "$d" && git commit -q -a -m tamper ); B="$(git -C "$d" rev-parse HEAD)"
( cd "$d" && git replace "$B" "$A" && git checkout -q "$A" -- docs/qa/run2/c.txt )
cc "Z96 C8 replace ref hides a tampered HEAD from the checker -> never PASS (replace refs ignored, worktree differs from the real HEAD)" 2 "VERDICT: BLIND" "$d" --check changelog
# OWN6: a failing `git hash-object` in the per-path chain walk is BLIND, never a pass
d=$(fresh); sd=$(gitstub2 'case "$*" in *"hash-object --no-filters -- "*) exit 1 ;; esac')
PATH="$sd:$PATH" RAW=1 expect "Z97 OWN6 failing hash-object in the chain walk -> BLIND" 2 "git hash-object failed for" "$d" --check changelog
echo "== selftest (11.4.273 control needles built into the tool)"
out="$("$CHECKER" --selftest 2>&1)"; got=$?
if [ "$got" = 0 ] && printf '%s\n' "$out" | grep -q '^VERDICT: PASS selftest'; then ok; else bad "selftest rc=$got"; fi

echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
