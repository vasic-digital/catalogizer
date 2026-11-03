#!/usr/bin/env bash
# gen_media_fixtures_guard_test.sh - tests for the --out deletion guard of gen_media_fixtures.sh (constitution 11.4.287).
# Everything runs inside a mktemp sandbox; a fake repo copy of the script is used so "ancestor of the repo" can be tested
# without touching the real tree; HOME is never reassigned (fake homes go through SAFE_RM_EXTRA_HOMES).
#   gen_media_fixtures_guard_test.sh           run the guard tests against the real script
#   gen_media_fixtures_guard_test.sh --mutate  also run paired mutations (guard parts disabled in a sandbox copy) and require each to go RED
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL="$HERE/gen_media_fixtures.sh"
REPO="$(cd "$HERE/../.." && pwd)"
SB="$(mktemp -d "${TMPDIR:-/tmp}/gmf-guard.XXXXXX")" || exit 2
[ -n "$SB" ] && [ -d "$SB" ] || exit 2
trap 'rm -rf -- "${SB:?}"' EXIT
pass=0; fail=0
ok()  { echo "PASS $1"; pass=$((pass+1)); }
bad() { echo "FAIL $1"; fail=$((fail+1)); }
# mkrepo <name> [script]: fake repo with a copy of the script and the real constitution linked in
mkrepo() { local r="$SB/$1"; mkdir -p "$r/scripts/test-infra"; cp "${2:-$REAL}" "$r/scripts/test-infra/gen_media_fixtures.sh"; ln -s "$REPO/constitution" "$r/constitution"; echo "$r"; }
run() { local r=$1; shift; bash "$r/scripts/test-infra/gen_media_fixtures.sh" "$@" 2>&1; }

SCHEMA=catalogizer.media-fixtures/1
# marker_dir <dir> [files...]: a dir that looks like a previous corpus (manifest.json with our schema listing the given relative files, which are created)
marker_dir() { local d=$1; shift; mkdir -p "$d"; local f; for f in "$@"; do mkdir -p "$d/$(dirname "$f")"; echo x > "$d/$f"; done
  python3 -I - "$d" "$SCHEMA" "$@" <<'PY2'
import sys, json
d, sch, files = sys.argv[1], sys.argv[2], sys.argv[3:]
json.dump({"schema": sch, "files": {f: {} for f in files}}, open(d + "/manifest.json", "w"))
PY2
}
suite() {   # suite <label> <script>
  local label=$1 script=$2 r o out rc
  # 1. foreign file: refused rc=2, file survives
  r=$(mkrepo "$label-r1" "$script"); o="$SB/$label-foreign/data"; mkdir -p "$o"; echo precious > "$o/user.txt"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 2 ] && [ -f "$o/user.txt" ]; then ok "$label foreign file refused, survives"; else bad "$label foreign file (rc=$rc, survives=$([ -f "$o/user.txt" ] && echo y || echo n))"; fi
  # 2. ancestor of repo: the sandbox dir holding the repo
  r=$(mkrepo "$label-anc/proj" "$script"); local anc="$SB/$label-anc"; echo precious > "$anc/user.txt"
  out=$(run "$r" --out "$anc"); rc=$?
  if [ "$rc" = 2 ] && [ -f "$anc/user.txt" ] && [ -d "$r/scripts" ]; then ok "$label ancestor of repo refused, tree intact"; else bad "$label ancestor of repo (rc=$rc)"; fi
  # 1b. marker dir (valid manifest) + a foreign file: refused, nothing deleted
  o="$SB/$label-mfor/c"; marker_dir "$o" movies/a.mp4; echo precious > "$o/user.txt"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 2 ] && [ -f "$o/user.txt" ] && [ -f "$o/movies/a.mp4" ]; then ok "$label marker dir with foreign file refused"; else bad "$label marker+foreign (rc=$rc)"; fi
  # 1c. manifest.json with the wrong schema is not a marker
  o="$SB/$label-wsch/c"; mkdir -p "$o"; echo '{"schema":"other/1","files":{}}' > "$o/manifest.json"; echo precious > "$o/user.txt"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 2 ] && [ -f "$o/user.txt" ]; then ok "$label foreign manifest schema refused"; else bad "$label wrong schema (rc=$rc)"; fi
  # 1d. manifest.json as a symlink is not a marker
  o="$SB/$label-lnk/c"; mkdir -p "$o"; marker_dir "$SB/$label-lnk-real" ; ln -s "$SB/$label-lnk-real/manifest.json" "$o/manifest.json"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 2 ] && [ -L "$o/manifest.json" ]; then ok "$label symlinked manifest refused"; else bad "$label symlink manifest (rc=$rc)"; fi
  # 1e. --check on a foreign dir: refused before anything is written
  o="$SB/$label-chk/c"; mkdir -p "$o"; echo precious > "$o/user.txt"
  out=$(run "$r" --out "$o" --check); rc=$?
  if [ "$rc" = 2 ] && [ -f "$o/user.txt" ] && [ ! -e "$o/run1" ]; then ok "$label --check foreign dir refused, nothing written"; else bad "$label --check foreign (rc=$rc)"; fi
  # 1f. newline in --out: refused up front, nothing created
  out=$(run "$r" --out "$SB/$label-nl/a"$'\n'"b"); rc=$?
  if [ "$rc" = 2 ] && [ ! -e "$SB/$label-nl" ]; then ok "$label control char in --out refused"; else bad "$label control char (rc=$rc)"; fi
  # 2b. ancestor of the repo whose manifest lists EVERYTHING under it: the policy alone would accept it, only the ancestor rule refuses
  r=$(mkrepo "$label-anc2/proj" "$script"); anc="$SB/$label-anc2"
  python3 -I - "$anc" "$SCHEMA" <<'PY2'
import sys, os, json
d, sch = sys.argv[1], sys.argv[2]; files = {}
for dp, dn, fn in os.walk(d, followlinks=False):
    for n in fn + dn: files[os.path.relpath(os.path.join(dp, n), d)] = {}
json.dump({"schema": sch, "files": files}, open(d + "/manifest.json", "w"))
PY2
  out=$(run "$r" --out "$anc"); rc=$?
  if [ "$rc" = 2 ] && [ -d "$r/scripts" ] && echo "$out" | grep -q out_is_or_contains_repo; then ok "$label manifest-listed ancestor of repo refused by the ancestor rule"; else bad "$label ancestor w/ manifest (rc=$rc) $out"; fi
  # 3. HOME-equal (fake home, empty so only the home rule can refuse)
  r=$(mkrepo "$label-r3" "$script"); local fh="$SB/$label-fakehome/u"; mkdir -p "$fh"
  out=$(SAFE_RM_EXTRA_HOMES="$fh" run "$r" --out "$fh"); rc=$?
  if [ "$rc" = 2 ] && [ -z "$(ls -A "$fh")" ]; then ok "$label home-equal refused, untouched"; else bad "$label home-equal (rc=$rc)"; fi
  # 3a. home with a space in its name (SAFE_RM_EXTRA_HOMES must not be word-split) and a glob entry that must not expand
  fh="$SB/$label-sp/my home"; mkdir -p "$fh" "$SB/$label-sp/other"
  out=$(SAFE_RM_EXTRA_HOMES="$SB/$label-sp/*:$fh" run "$r" --out "$fh"); rc=$?
  if [ "$rc" = 2 ] && [ -z "$(ls -A "$fh")" ]; then ok "$label home with space refused"; else bad "$label home with space (rc=$rc)"; fi
  out=$(SAFE_RM_EXTRA_HOMES="$SB/$label-sp/*" run "$r" --out "$SB/$label-sp/other/x"); rc=$?
  if [ "$rc" != 2 ] || ! echo "$out" | grep -q out_is_or_contains_home; then ok "$label glob entry not expanded"; else bad "$label glob expanded"; fi
  # 3b. ancestor of a home
  mkdir -p "$SB/$label-hanc/a/home"
  out=$(SAFE_RM_EXTRA_HOMES="$SB/$label-hanc/a/home" run "$r" --out "$SB/$label-hanc/a"); rc=$?
  if [ "$rc" = 2 ] && [ -d "$SB/$label-hanc/a/home" ]; then ok "$label ancestor of home refused"; else bad "$label ancestor of home (rc=$rc)"; fi
}

allowed_cases() {
  local r o e out rc; r=$(mkrepo ok); o="$SB/ok-out/corpus"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 0 ] && grep -q 'catalogizer.media-fixtures/1' "$o/manifest.json"; then ok "absent dir generated"; else bad "absent dir (rc=$rc) $out"; return; fi
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 0 ] && [ -f "$o/manifest.json" ]; then ok "marker dir regenerated"; else bad "marker dir (rc=$rc)"; fi
  echo stray > "$o/stray.txt"
  out=$(run "$r" --out "$o"); rc=$?
  if [ "$rc" = 2 ] && [ -f "$o/stray.txt" ] && echo "$out" | grep -q out_has_entries_not_in_corpus_manifest; then ok "real corpus dir plus stray file refused, stray survives"; else bad "stray in corpus dir (rc=$rc) $out"; fi
  rm -f -- "${o:?}/stray.txt"
  e="$SB/ok-empty/corpus"; mkdir -p "$e"
  out=$(run "$r" --out "$e"); rc=$?
  if [ "$rc" = 0 ] && [ -f "$e/manifest.json" ]; then ok "empty dir generated"; else bad "empty dir (rc=$rc)"; fi
  echo extra > "$o/extra.txt"; out=$(run "$r" --out "$o" --verify); rc=$?
  if [ "$rc" = 1 ] && echo "$out" | grep -q 'EXTRA extra.txt'; then ok "--verify flags extra file"; else bad "--verify extra (rc=$rc) $out"; fi
  rm -f -- "${o:?}/extra.txt"
  out=$(run "$r" --out "$o" --verify); if [ $? = 0 ]; then ok "--verify clean passes"; else bad "--verify clean"; fi
  out=$(GMF_MAX_BYTES=1000 run "$r" --out "$SB/ok-cap/c" --check); rc=$?
  if [ "$rc" = 1 ] && echo "$out" | grep -q corpus_over_cap; then ok "--check applies size cap"; else bad "--check cap (rc=$rc) $out"; fi
  if python3 -I - "$o/manifest.json" <<'PY'
import json,sys
e=json.load(open(sys.argv[1]))["files"]["negative/truncated-tail.mp4"]
assert e["class"]=="partial_playback" and e["expected"]["must_fail_playback_or_parse"] is False and e["expected"]["partial_playback"] is True, e
PY
  then ok "truncated-tail manifest says partial_playback"; else bad "truncated-tail manifest"; fi
}

echo "== real script =="; suite real "$REAL"
if [ "${1:-}" != "--refusals-only" ]; then allowed_cases; fi
if [ "${1:-}" = "--mutate" ]; then
  echo "== mutations (each must be caught = refusal suite FAILs) =="
  mutate() { # mutate <name> <sed-expr>
    local m="$SB/mut-$1.sh" b; sed -E "$2" "$REAL" > "$m"
    if cmp -s "$m" "$REAL"; then bad "mutation $1 changed nothing"; return; fi
    b=$fail; suite "mut-$1" "$m" >/dev/null
    if [ "$fail" -gt "$b" ]; then fail=$b; ok "mutation $1 caught"; else bad "mutation $1 SURVIVED"; fi
  }
  mutate policy 's/^( *)out_guard_policy .*$/\1:/'
  mutate home 's/^( *)out_guard_home .*$/\1:/'
  mutate ancestor 's/^( *)out_guard_ancestor .*$/\1:/'
  mutate checkpolicy 's/^  \[ "\$MODE" != check \] \|\| out_guard_policy .*$/  :/'
  mutate all 's/^( *)out_guard_(policy|home|ancestor) .*$/\1:/'
fi
echo "guard-test: pass=$pass fail=$fail"; [ "$fail" = 0 ]
