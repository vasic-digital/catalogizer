#!/usr/bin/env bash
# Paired mutation runner (constitution 1.1 / 11.4.224): proves each check of
# check_release_consistency.sh is load-bearing. For every mutant it copies the
# checker into a temp dir, applies ONE exact textual mutation (asserting the
# mutation really changed the file), runs test_check_release_consistency.sh with
# CHECKER=<mutant>, and requires that suite to FAIL (mutant killed).
# Exit 0 only if every mutant is killed. A surviving mutant = a check nobody tests.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/mut.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/mut.*) ;; *) echo "odd ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT

# shellcheck disable=SC2016  # mutation texts contain literal $ on purpose
# name|||old|||new   (old must occur exactly once)
MUTANTS=(
'openapi version check removed|||check("docs/api/openapi.yaml info.version", openapi)|||pass'
'openapi nested indent accepted|||m = re.match(r"^  version:|||m = re.match(r"^\s*version:'
'versions.json derive drops minor|||{g['"'"'minor'"'"']}.{g['"'"'patch'"'"']}"|||3.{g['"'"'patch'"'"']}"'
'lock root-entry check reads top version|||["packages"][""]["version"]|||["version"]'
'lock top-version check skipped|||lambda c=comp: json.loads(read(f"{c}/package-lock.json"))["version"])|||lambda c=comp: expected)'
'package.json check skipped|||lambda c=comp: json.loads(read(f"{c}/package.json"))["version"])|||lambda c=comp: expected)'
'tauri check skipped|||tauri(f"{c}/src-tauri/tauri.conf.json")|||expected'
'cargo check skipped|||cargo(f"{c}/src-tauri/Cargo.toml")|||expected'
'gradle versionName check skipped|||lambda p=path: gradle_field(read(p), "versionName"))|||lambda p=path: expected)'
'versionCode allows equal|||elif cur > int(prev_s):|||elif cur >= int(prev_s):'
'versionCode non-integer accepted|||if not re.fullmatch(r"[0-9]+", code):|||if False:'
'unknown prev-tag accepted|||if r.returncode != 0:
        tag_ok = False|||if False:
        tag_ok = False'
'missing-file read errors ignored|||emit("FAIL", label, f"cannot read: {type(exc).__name__}: {exc}")|||emit("OK", label, "ignored")'
'existence check disabled|||if [ ! -e "$REPO/$q" ]; then|||if false; then'
'tracked check disabled|||elif ! tracked "$q"; then|||elif false; then'
'zero paths not blind|||[ "${#paths[@]}" -eq 0 ]|||[ "${#paths[@]}" -lt 0 ]'
'section-not-found not blind|||ERR) note_blind "$a"; return ;;|||ERR) : ;;'
'section heading accepts Unreleased|||^## \[(\d[^\]]*)\]|||^## \[([^\]]*)\]'
'VERIFIED subsection bleeds into next|||    if re.match(r"^#{1,3} ", ln):
        inv = False
        continue|||    if re.match(r"^#{1,3} ", ln):
        continue'
'sha mismatch not counted|||S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"|||sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"'
'sha missing counted unverifiable|||S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (missing/unreadable, not gitignored)"|||S_UNV=$((S_UNV+1)); sawfail=1'
'sha ignored file counted as fail|||if git -C "$REPO" check-ignore -q -- "$d/$name"; then|||if false; then'
'FAIL verdict exits 0|||echo "VERDICT: FAIL $SUMMARY"; exit 1|||echo "VERDICT: FAIL $SUMMARY"; exit 0'
'blind verdict exits 0|||echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 2|||echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 0'
'unknown option ignored|||*) usage >&2; blind "unknown option $1" ;;|||*) shift ;;'
'B1 helper exit status ignored|||if [ "$vrc" -ne 0 ]; then|||if false; then'
'B1 missing PLAN not blind|||if ! [[ "$plan" =~ ^[0-9]+$ ]]; then|||if false; then'
'B1 short row count accepted|||if [ "$rows" -lt "$plan" ]; then|||if false; then'
'F1 malformed manifest not failed|||if grep -q '"'"'improperly formatted'"'"' <<< "$o" || { [ "$rc" -ne 0 ] && [ "$sawfail" = 0 ] && [ "$sawunv" = 0 ]; }; then|||if false; then'
'F4 manifest escape detection off|||if [[ "$ent" == /*|||if false && [[ "$ent" == /*'
'F4 escaping manifest still executed|||[ "$esc" = 0 ] || continue|||:'
'F3 only first VERIFIED read|||        inv = True
        nsub += 1|||        inv = nsub == 0
        nsub += 1'
'F5 duplicate info.version accepted|||if len(found) != 1:|||if len(found) < 1:'
'F5 empty --version accepted|||{ [ $# -ge 2 ] && [ -n "$2" ]; }|||{ [ $# -ge 2 ]; }'
'F5 both gradle files ignored|||    if both:|||    if False:'
'F2 dropped tokens not counted|||            print(f"SKIP\t{tok.strip()}")
            continue|||            continue'
'F5 needle takes first tracked name|||| grep -m1 -E '"'"'^[A-Za-z0-9._/-]+$'"'"')|||| head -1)'
'F5 needle needs tree presence|||! tracked "$pos"|||! tracked "$pos" || [ ! -e "$REPO/$pos" ]'
'help has no VERDICT|||usage; echo "VERDICT: HELP"; exit 0|||usage; exit 0'
)

killed=0; survived=0; n=0
for m in "${MUTANTS[@]}"; do
  name="${m%%|||*}"; rest="${m#*|||}"; old="${rest%%|||*}"; new="${rest#*|||}"
  n=$((n+1)); d="$ROOT/m$n"; mkdir -p "$d"
  cp "$HERE/check_release_consistency.sh" "$HERE/release_fixture.sh" "$HERE/test_check_release_consistency.sh" "$d/"
  if ! python3 -I - "$d/check_release_consistency.sh" "$old" "$new" <<'PY'
import sys
p, old, new = sys.argv[1:4]
s = open(p, encoding="utf-8").read()
if s.count(old) != 1:
    sys.stderr.write(f"mutation target occurs {s.count(old)} times\n"); sys.exit(3)
open(p, "w", encoding="utf-8").write(s.replace(old, new))
PY
  then echo "ERROR  [$n] $name: mutation did not apply"; survived=$((survived+1)); continue; fi
  if CHECKER="$d/check_release_consistency.sh" "$d/test_check_release_consistency.sh" >"$d/out.txt" 2>&1; then
    echo "SURVIVED [$n] $name"; survived=$((survived+1))
  else
    echo "killed   [$n] $name ($(tail -1 "$d/out.txt"))"; killed=$((killed+1))
  fi
done
echo "== mutants: $n, killed: $killed, survived/errored: $survived"
[ "$survived" -eq 0 ]
