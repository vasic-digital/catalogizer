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
'tracked check disabled|||elif ! tracked "$q" && ! tracked "$evrel"; then|||elif false; then'
'zero paths not blind|||[ "${#paths[@]}" -eq 0 ]|||[ "${#paths[@]}" -lt 0 ]'
'section-not-found not blind|||ERR) note_blind "$a"; return ;;|||ERR) : ;;'
'section heading accepts Unreleased|||^## \[(\d[^\]]*)\]|||^## \[([^\]]*)\]'
'VERIFIED subsection bleeds into next|||    if not isf and re.match(r"^#{1,3} ", ln):
        inv = False
        continue|||    if not isf and re.match(r"^#{1,3} ", ln):
        continue'
'sha mismatch not counted|||S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"
            if [ -n|||sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"
            if [ -n'
'sha missing counted unverifiable|||S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (missing/unreadable, not gitignored)"|||S_UNV=$((S_UNV+1)); sawfail=1'
'sha ignored file counted as fail|||if git -C "$REPO" check-ignore -q -- "$d/$name"; then|||if false; then'
'FAIL verdict exits 0|||echo "VERDICT: FAIL $SUMMARY"; exit 1|||echo "VERDICT: FAIL $SUMMARY"; exit 0'
'blind verdict exits 0|||echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 2|||echo "VERDICT: BLIND $SUMMARY reason:$BLINDS"; exit 0'
'unknown option ignored|||*) usage >&2; blind "unknown option $1" ;;|||*) shift ;;'
'B1 helper exit status ignored|||if [ "$vrc" -ne 0 ]; then|||if false; then'
'B1 missing PLAN not blind|||if ! [[ "$plan" =~ ^[0-9]+$ ]]; then|||if false; then'
'B1 short row count accepted|||if [ "$rows" -lt "$plan" ]; then|||if false; then'
'F1 malformed manifest not failed|||if grep -q '"'"'improperly formatted'"'"' <<< "$o" || { [ "$rc" -ne 0 ] && [ "$sawfail" = 0 ] && [ "$sawunv" = 0 ] && [ "$sawwaived" = 0 ]; }; then|||if false; then'
'F4 manifest escape detection off|||if [[ "$ent" == /*|||if false && [[ "$ent" == /*'
'F4 escaping manifest still executed|||[ "$esc" = 0 ] || continue|||:'
'F3 only first VERIFIED read|||        inv = True
        nsub += 1|||        inv = nsub == 0
        nsub += 1'
'F5 duplicate info.version accepted|||if len(found) != 1:|||if len(found) < 1:'
'F5 empty --version accepted|||{ [ $# -ge 2 ] && [ -n "$2" ]; } || blind "--version needs|||{ [ $# -ge 2 ]; } || blind "--version needs'
'F5 both gradle files ignored|||    if both:|||    if False:'
'F2 dropped tokens not counted|||            print(f"SKIP\t{tok.strip()}")
            nskip += 1
            continue|||            continue'
'F5 needle takes first tracked name|||| grep -m1 -E '"'"'^[A-Za-z0-9._/-]+$'"'"')|||| head -1)'
'F5 needle needs tree presence|||! tracked "$pos"|||! tracked "$pos" || [ ! -e "$REPO/$pos" ]'
'N1 duplicate PLAN accepted|||if [ "$nplan" -gt 1 ]; then|||if false; then'
'N1 plan count not pinned to 19|||if [ "$plan" -ne "$expected_n" ]; then|||if false; then'
'N1 missing expected label accepted|||if [ -n "$miss" ]; then|||if false; then'
'N1 duplicate label accepted|||if [ -n "$dup" ]; then|||if false; then'
'N1 foreign label accepted|||if [ -n "$foreign" ]; then|||if false; then'
'N1 INFO allowed on non-versionCode field|||*" versionCode") ;;|||*) ;;'
'N1 extra FAIL-only rows not enforced|||"--prev-tag"|*"/app") [ "${kinds["$key"]}" = FAIL ] || foreign="$foreign [$key]" ;;|||"--prev-tag"|*"/app") ;;'
'N1 changelog section count unchecked|||[ "$nsec" -ne 1 ] || |||false || '
'N1 changelog END count unchecked|||[ "$nsec" -ne 1 ] || [ "$nend" -ne 1 ] || |||[ "$nsec" -ne 1 ] || false || '
'N1 changelog END not last|||[ "$lastkind" != END ] \
|||false \
'
'N1 changelog path count unchecked|||[ "$endp" != "${#paths[@]}" ] || |||false || '
'N1 changelog skip count unchecked|||[ "$ends" != "${#skips[@]}" ]; then|||false; then'
'N2 BSD-tag lines not parsed|||elif [[ "$line" =~ ^[A-Za-z0-9-]+\ \((.*)\)\ =\ [0-9a-fA-F]+$ ]]; then|||elif false; then'
'N3 outside-repo symlink accepted|||if [ -e "$REPO/$q" ] && { [ -z "$rp" ]|||if false && { [ -z "$rp" ]'
'N4 versionCode expression accepted|||\s*;?\s*(?:/\*(?:[^*]|\*(?!/))*\*/\s*)*(?://.*)?$")|||")'
'N5 duplicate heading accepted|||hm.group(1) == name) > 1:|||hm.group(1) == name) > 99:'
'N5 versionNameSuffix ambiguity|||(?![A-Za-z0-9_])|||'
'R3 F1 non-matching manifest lines tolerated|||elif [ -n "${line//[[:space:]]/}" ]; then|||elif false; then'
'R3 F2 manifest symlink resolution skipped|||rp="$(readlink -m -- "$REPO/$d/$ent" 2>/dev/null || true)"|||rp="$REPO_REAL"'
'R3 F2 manifest prefix check without slash|||[ "$rp" != "$REPO_REAL" ] && [[ "$rp" != "$REPO_REAL"/* ]]; }; then
            S_FAIL|||[ "$rp" != "$REPO_REAL" ] && [[ "$rp" != "$REPO_REAL"* ]]; }; then
            S_FAIL'
'R3 F3 changelog prefix check without slash|||[[ "$rp" != "$REPO_REAL"/* ]]; }; }; then|||[[ "$rp" != "$REPO_REAL"* ]]; }; }; then'
'R3 F3 repo-root symlink branch dropped|||{ [ -z "$rp" ] || { [ "$rp" != "$REPO_REAL" ] && [[ "$rp" != "$REPO_REAL"/* ]]; }; }; then|||{ [ -z "$rp" ] || [[ "$rp" != "$REPO_REAL"/* ]]; }; then'
'R3 F3 BSD hex lowercase only|||=\ [0-9a-fA-F]+$ ]]; then|||=\ [0-9a-f]+$ ]]; then'
'R3 F3 single-slash comment regex|||\s*;?\s*(?:/\*(?:[^*]|\*(?!/))*\*/\s*)*(?://.*)?$")|||\s*;?\s*(?:/\*(?:[^*]|\*(?!/))*\*/\s*)?(?:/.*)?$")'
'R3 F4 forged INFO under prev-tag accepted|||[ "${kinds["--prev-tag"]:-}" != FAIL ]|||false'
'R3 F5 fenced headings counted as headings|||m = None if fenced[i] else head.match(ln)|||m = head.match(ln)'
'R3 F5 fenced duplicate counted|||if not fenced[k] and (hm := head.match(ln))|||if (hm := head.match(ln))'
'R3 F5 fenced section end honoured|||if not fenced[j] and lines[j].startswith("## "):|||if lines[j].startswith("## "):'
'R3 F5 fenced VERIFIED subsection honoured|||isf = fenced[start + 1 + k]|||isf = False'
'R3 F5 semicolon after versionCode rejected|||\s*;?\s*(?:/\*|||\s*(?:/\*'
'R3 F5 block comment after versionCode rejected|||(?:/\*(?:[^*]|\*(?!/))*\*/\s*)*(?://.*)?$")|||(?://.*)?$")'
'R4 final manifest line without newline skipped|||while IFS= read -r line || [ -n "$line" ]; do
      ent=""|||while IFS= read -r line; do
      ent=""'
'R4 fence regex needs column 0|||^\s{0,3}(```|~~~)|||^(```|~~~)'
'R4 CRLF trailing CR not stripped|||line="${line%$'"'"'\r'"'"'}"   # CRLF manifests|||:   # CRLF manifests'
'R4 CR inside a line accepted|||if [[ "$line" == *$'"'"'\r'"'"'* ]]; then|||if false; then'
'R4 manifest readlink -f (missing parent false escape)|||readlink -m -- "$REPO/$d/$ent"|||readlink -f -- "$REPO/$d/$ent"'
'R4 comment regex spans closing */ (N4-tail)|||(?:/\*(?:[^*]|\*(?!/))*\*/\s*)*(?://.*)?$")|||(?:/\*.*\*/\s*)*(?://.*)?$")'
'RC-3 evidence without manifest not counted|||C_NOMAN=$((C_NOMAN+1)); echo "NOMANIFEST|||echo "NOMANIFEST'
'RC-3 every path treated as evidence|||[[ "$1/" == "$pre"* ]] && return 0|||return 0'
'RC-3 no path treated as evidence|||[[ "$1/" == "$pre"* ]] && return 0|||false && return 0'
'RC-3b unlisted file not counted|||U_FILES=$((U_FILES+1)); echo "UNLISTED|||echo "UNLISTED'
'RC-3b completeness comparison disabled|||if os.path.normpath(rel) not in listed_for(d):|||if False:'
'RC-3b untracked files invisible|||"--cached", "--others", "--exclude-standard"|||"--cached"'
'RC-3b gitignored files demanded|||"--cached", "--others", "--exclude-standard"|||"--cached", "--others"'
'RC-3b nested manifests not queued|||for f in "${nst[@]+"${nst[@]}"}"; do queue+=("$f"); done|||:'
'RC-3b helper trailer counts unchecked||||| [ "$nu" != "${#unl[@]}" ] || [ "$nn" != "${#nst[@]}" ]; then|||; then'
'RC-3b unlisted verdict ignored|||C_MISS + S_FAIL + C_NOMAN + U_FILES + W_BAD|||C_MISS + S_FAIL + C_NOMAN + W_BAD'
'RC-3 no-manifest verdict ignored|||C_MISS + S_FAIL + C_NOMAN + U_FILES + W_BAD|||C_MISS + S_FAIL + U_FILES + W_BAD'
'waiver: bad waiver verdict ignored|||C_MISS + S_FAIL + C_NOMAN + U_FILES + W_BAD|||C_MISS + S_FAIL + C_NOMAN + U_FILES'
'waiver: any key accepted|||[ -n "${WV[$1]+x}" ] || return 1|||:'
'waiver: untracked waiver file honoured|||if ! tracked "$rel"; then note_blind|||if false; then note_blind'
'waiver: duplicates accepted|||if [ -n "${WV[$key]+x}" ]; then W_BAD|||if false; then W_BAD'
'waiver: unknown kind accepted|||*) W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n unknown kind '"'"'$k'"'"'"; continue ;;|||*) continue ;;'
'waiver: missing explicit file not blind|||[ "$explicit" = 0 ] || { note_blind|||{ note_blind'
'waiver: ./ entry prefix not normalised|||name="${name#./}"|||:'
'waiver: bad-entry waiver leaves rc check strict|||[ "$sawunv" = 0 ] && [ "$sawwaived" = 0 ]; }; then
      S_FAIL|||[ "$sawunv" = 0 ]; }; then
      S_FAIL'
'RC2-1 only one block comment allowed|||\*/\s*)*(?://.*)?$")|||\*/\s*)?(?://.*)?$")'
'RC-3 MANIFEST*.sha256 not recognised|||"$REPO/$d"/MANIFEST*.sha256; do|||"$REPO/$d"/MANIFEST-NONE.sha256; do'
'RC-3 SHA256SUMS.txt not recognised|||"$REPO/$d"/SHA256SUMS "$REPO/$d"/SHA256SUMS.txt "$REPO/$d"/MANIFEST|||"$REPO/$d"/SHA256SUMS "$REPO/$d"/MANIFEST'
'RC-3 python MANIFEST*.sha256 not a manifest|||fnmatch.fnmatchcase(n, "MANIFEST*.sha256")|||False'
'RC-3 manifest must be tracked|||tracked "$d/$n" && printf|||printf'
'RC-3 NOMANIFEST ignores waivers file|||if ! waived "no-manifest|$evrel" "$p (no manifest)"; then|||if true; then'
'RC-3 path spelling not normalised|||ev="$(readlink -m -- "$REPO/$q" 2>/dev/null || true)"|||ev=""'
'RC-3 control char in VERIFIED path not blind|||if [[ "$p" == *[[:cntrl:]]* ]]; then|||if false; then'
'RC-3 nested dir name control char not blind (python)|||    if re.search(r"[\x00-\x1f\x7f]", n):|||    if False:'
'RC-3 nested dir name control char not blind (bash)|||BADNAME) note_blind "nested manifest directory name contains a control character: $a"; return ;;|||BADNAME) ;;'
'RC-3 UNLISTED name not escaped|||    return re.sub(r"[\x00-\x1f\x7f]", lambda m: "\\x%02x" % ord(m.group()), t)|||    return t'
'RC-3 manifest dir missing/no-manifest not blind|||if [ ! -d "$REPO/$d" ] || [ -z "$mfs" ]; then|||if false; then'
'RC-3 manifest dir without manifest not blind||||| [ -z "$mfs" ]; then|||; then'
'RC-3b manifest files demanded in the manifest|||if f in tmf or os.path.islink(full) or not os.path.isfile(full):|||if os.path.islink(full) or not os.path.isfile(full):'
'RC-3b symlinks demanded in the manifest|||if f in tmf or os.path.islink(full) or not os.path.isfile(full):|||if f in tmf or not os.path.isfile(full):'
'RC-3b tracked() glob-expands dir names|||git -C "$REPO" ls-files -- ":(literal)$1"|||git -C "$REPO" ls-files -- "$1"'
# Equivalent mutant (not run): dropping :(literal) in the Python ls-files helper. A wildcard git pathspec matches only the exact path,
# never the children of a wildcard-matched sibling directory, and glob characters (* ?) in cited paths are skipped by the tokenizer.
'waiver: reason optional|||if [ -z "$reason" ]; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing reason"; continue; fi|||if false; then W_BAD=$((W_BAD+1)); echo "BADWAIVER  $rel:$n missing reason"; continue; fi'
'waiver: tracked item optional|||if ! [[ "$item" =~ ^[A-Za-z][A-Za-z0-9]*(-[A-Za-z0-9]+)+$ ]]; then|||if false; then'
'waiver: impossible date accepted||||| [ "$(date -d "$exp" +%F 2>/dev/null)" != "$exp" ]; then|||; then'
'waiver: expiry not enforced|||if [[ "$exp" < "$today" ]]; then|||if false; then'
'waiver: valid on expiry day lost|||if [[ "$exp" < "$today" ]]; then|||if [[ "$exp" < "$today" || "$exp" == "$today" ]]; then'
'waiver: file may differ from HEAD|||if ! git -C "$REPO" diff --quiet HEAD -- ":(literal)$rel" 2>/dev/null; then|||if false; then'
'waiver: PASS-WITH-WAIVERS not distinct|||if [ "$W_APPLIED" -gt 0 ]; then|||if false; then'
'waiver: unused waiver not listed|||echo "UNUSED-WAIVER  $key: ${WV[$key]}"|||:'
'waiver: whitespace fields not trimmed|||for i in "${!F[@]}"; do F[i]="$(trim "${F[i]}")"; done|||:'
'I-2 mismatch waivable again|||S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"
            if [ -n "${WV["bad-entry|$d|$name"]+x}" ]; then|||if waived "bad-entry|$d|$name" "$d/$name (checksum mismatch)"; then sawwaived=1; sawfail=0; else S_FAIL=$((S_FAIL+1)); sawfail=1; echo "SHAFAIL  $d/$name (checksum mismatch)"; fi
            if false; then'
'I-1 dirty tree check removed|||  if [ -n "$dirty" ]; then|||  if false; then'
'I-1 git status failure not blind|||  if ! dirty="$(git -C "$REPO" status|||  if false && ! dirty="$(git -C "$REPO" status'
'I-1 status without literal pathspec|||status --porcelain=v1 -z --untracked-files=no -- ":(literal)$d"|||status --porcelain=v1 -z --untracked-files=no -- "nonexistent-$d"'
'I-1 index changes ignored (diff vs worktree only)|||  local dirty
  if ! dirty="$(git -C "$REPO" status --porcelain=v1 -z --untracked-files=no|||  local dirty
  if ! dirty="$(git -C "$REPO" diff --name-only -z --'
'M-2 control char token not blind|||CTRL) note_blind|||CTRL) : note_blind'
'M-2 helper does not flag control chars|||if re.search(r"[\x00-\x1f\x7f]", tok):|||if False:'
'RV2-1 python counts untracked manifests|||tmf = {f for f in tracked if is_mf|||tmf = {f for f in files if is_mf'
'RV2-2 is_evidence on raw spelling|||if [ "$found" = 0 ] && is_evidence "$evrel"; then|||if [ "$found" = 0 ] && is_evidence "$q"; then'
'RV2-3 today pinned in the past|||today="${RELCHK_TODAY:-$(date +%F)}"|||today="0000-00-00"'
'RV2-4 manifest walk-up depth 2|||[ "$guard" -lt 500 ]|||[ "$guard" -lt 2 ]'
'RV2-5 waiver key uses raw dir|||key="$k|${F[1]:-}|${F[2]:-}"|||key="$k|${F[1]:-}/|${F[2]:-}"'
'RV2-6 waiver HEAD check without literal|||if ! git -C "$REPO" diff --quiet HEAD -- ":(literal)$rel" 2>/dev/null; then|||if ! git -C "$REPO" diff --quiet HEAD -- "nonexistent-$rel" 2>/dev/null; then'
'RV2-7 prefix dir itself not evidence|||[[ "$1/" == "$pre"* ]] && return 0|||[[ "$1" == "$pre"* ]] && return 0'
'R3-1 changelog head check removed|||head_check "docs/CHANGELOG.md" "docs/CHANGELOG.md"|||:'
'R4 READ only recorded for json|||    READ.append(path)|||    if path.endswith('.json'): READ.append(path)'
'R3-1 version files head check removed|||head_check "version files" "${vread[@]+"${vread[@]}"}"|||:'
'R3-4 refused mismatch waiver not counted|||WVUSED["bad-entry|$d|$name"]=1; W_BAD=$((W_BAD+1))|||WVUSED["bad-entry|$d|$name"]=1'
'R3-4 refused mismatch waiver not marked used|||WVUSED["bad-entry|$d|$name"]=1; W_BAD=$((W_BAD+1))|||W_BAD=$((W_BAD+1))'
'R3-1 cited path head check removed|||{ [ "$cited_ok" = 0 ] || [ "$evrel" = "." ]; } || head_check "cited path $p" "$q"|||:'
'I4-1 head check only the resolved location|||head_check "cited path $p" "$q"|||head_check "cited path $p" "$evrel"'
'R4 chain_one existence uses -f (directories skipped)|||  [ -e "$REPO/$spec" ] || return 0|||  [ -f "$REPO/$spec" ] || return 0'
'R3-1 chain ignores HEAD membership|||    if [ -z "$E_MODE" ]; then chain_bad "$ent is not in HEAD"; return 0; fi|||    if false; then chain_bad "$ent is not in HEAD"; return 0; fi'
'R3-1 chain ignores status|||  [ -z "$out" ] || chain_bad "differs from HEAD: |||  [ -z "$out" ] || : "differs from HEAD: '
'R3-1 chain status failure not blind|||    note_blind "$(printf '"'"'%q'"'"' "$cur"): git status failed; cannot prove the files match HEAD"; return 1|||    return 0'
'CC2 hop target not compared with HEAD|||if [ "$tgt" != "$want" ]; then chain_bad|||if false; then chain_bad'
'CC2 symlink type not compared|||if [ ! -L "$REPO/$ent" ]; then chain_bad "$ent is a symlink in HEAD|||if false; then chain_bad "$ent is a symlink in HEAD'
'CC2 directory type not compared|||if [ -L "$REPO/$ent" ] || [ ! -d "$REPO/$ent" ]; then chain_bad "$ent is a directory in HEAD|||if false; then chain_bad "$ent is a directory in HEAD'
'CC2 file type not compared|||if [ -L "$REPO/$ent" ] || [ ! -f "$REPO/$ent" ]; then chain_bad "$ent is a file in HEAD|||if false; then chain_bad "$ent is a file in HEAD'
'CC2 absolute hop accepted|||/*) chain_bad "$ent is an absolute symlink ($tgt); a clone elsewhere cannot prove it"; return 0 ;;|||/*) ;;'
'CC2 hop into submodule accepted|||if [ -n "$rest" ]; then chain_bad "$spec continues inside the submodule|||if false; then chain_bad "$spec continues inside the submodule'
'CC2 case collision check disabled|||if [ "$r" -eq 0 ]; then chain_bad "$ent collides|||if false; then chain_bad "$ent collides'
'CC2 case collision counts every entry|||[ "$n" -gt 1 ]|||[ "$n" -gt 0 ]'
'CC2 content hash not compared|||[ "$out" = "$fsha" ] || chain_bad "$cur differs from HEAD (content)"|||:'
'CC2 final flag check dropped|||[ -z "$flagged" ] || chain_bad "$cur holds|||[ -z "$flagged" ] || : "$cur holds'
'CC2 flag pattern misses assume-unchanged|||grep -m1 '"'"'^[a-zS] '"'"' || true)"
  [ -z "$flagged" ] || chain_bad|||grep -m1 '"'"'^S '"'"' || true)"
  [ -z "$flagged" ] || chain_bad'
'CC2 manifest flag check dropped|||  if [ -n "$flagged" ]; then
    note_blind "manifest tree|||  if false; then
    note_blind "manifest tree'
'CC2 hop target appended after the rest of the path|||pend=("${parts[@]}" "${pend[@]}") ;;|||pend=("${pend[@]}" "${parts[@]}") ;;'
'CC2 dot component not skipped|||      ""|.) continue ;;
      ..)|||      "") continue ;;
      ..)'
'CC2 dotdot drops the whole path|||if [[ "$cur" == */* ]]; then cur="${cur%/*}"; else cur=""; fi|||cur=""'
'CC2 tracked check ignores the resolved location|||elif ! tracked "$q" && ! tracked "$evrel"; then|||elif ! tracked "$q"; then'
'CC2 --version does not select the section|||[ -n "$SECTION" ] || SECTION="$EXPECTED"|||:'
'CC2 whitespace paths all skipped (m-2)|||if re.search(r"\s", t) and t not in tfiles and t not in tdirs:|||if re.search(r"\s", t):'
'CC2 whitespace paths never skipped|||if re.search(r"\s", t) and t not in tfiles and t not in tdirs:|||if False:'
'CC2 gitlink HEAD membership via cat-file (m-3)|||    head_entry "$ent" || {|||    git -C "$REPO" cat-file -e "HEAD:$ent" 2>/dev/null || { chain_bad "$ent is not in HEAD"; return 0; }
    head_entry "$ent" || {'
# Equivalent mutants (not run): the 40-hop cap in chain_one (a loop is MISS before the walk starts, and a HEAD loop cannot match a
# working tree that is not a loop because every hop target is compared with HEAD's blob) and the '..'-above-the-root guard in
# chain_one (the outside-repo rule rejects such paths first), and the exact-name filter in head_entry (git ls-tree does not
# glob: measured, a pathspec like 'd/f[1].txt' or 'd/f*' returns only the literal entry or nothing).
'help has no VERDICT|||usage; echo "VERDICT: HELP"; exit 0|||usage; exit 0'
)

# MUT_JOBS=<n> runs n mutants at once (default 4; each mutant runs the whole suite, ~40 s).
JOBS="${MUT_JOBS:-4}"
case "$JOBS" in ''|*[!0-9]*|0) echo "bad MUT_JOBS '$JOBS'" >&2; exit 2 ;; esac

run_one() { # run_one <n> <name> <old> <new>; writes $ROOT/m<n>/result (one line)
  local n="$1" name="$2" old="$3" new="$4" d="$ROOT/m$1"
  mkdir -p "$d"
  cp "$HERE/check_release_consistency.sh" "$HERE/release_fixture.sh" "$HERE/test_check_release_consistency.sh" "$d/"
  if ! python3 -I - "$d/check_release_consistency.sh" "$old" "$new" <<'PY'
import sys
p, old, new = sys.argv[1:4]
s = open(p, encoding="utf-8").read()
if s.count(old) != 1:
    sys.stderr.write(f"mutation target occurs {s.count(old)} times\n"); sys.exit(3)
open(p, "w", encoding="utf-8").write(s.replace(old, new))
PY
  then echo "ERROR  [$n] $name: mutation did not apply" > "$d/result"; return; fi
  if ! bash -n "$d/check_release_consistency.sh" 2>/dev/null; then
    echo "ERROR  [$n] $name: mutant is a syntax error (would be killed trivially)" > "$d/result"; return; fi
  if [ -n "${MUT_APPLY_ONLY:-}" ]; then echo "applies  [$n] $name" > "$d/result"; return; fi   # quick applicability pass, not a verdict
  if CHECKER="$d/check_release_consistency.sh" "$d/test_check_release_consistency.sh" >"$d/out.txt" 2>&1; then
    echo "SURVIVED [$n] $name" > "$d/result"
  else
    echo "killed   [$n] $name ($(tail -1 "$d/out.txt"))" > "$d/result"
  fi
}

n=0
for m in "${MUTANTS[@]}"; do
  name="${m%%|||*}"; rest="${m#*|||}"; old="${rest%%|||*}"; new="${rest#*|||}"
  # MUT_FILTER=<ERE>: run only mutants whose name matches (quick iteration; the full run is the gate).
  if [ -n "${MUT_FILTER:-}" ] && ! [[ "$name" =~ $MUT_FILTER ]]; then continue; fi
  n=$((n+1))
  while [ "$(jobs -rp | wc -l)" -ge "$JOBS" ]; do wait -n; done
  run_one "$n" "$name" "$old" "$new" &
done
wait
killed=0; survived=0
for i in $(seq 1 "$n"); do
  line="$(cat "$ROOT/m$i/result" 2>/dev/null || echo "ERROR  [$i] no result")"
  echo "$line"
  case "$line" in killed*) killed=$((killed+1)) ;; *) survived=$((survived+1)) ;; esac
done
echo "== mutants: $n, killed: $killed, survived/errored: $survived"
[ "$survived" -eq 0 ]
