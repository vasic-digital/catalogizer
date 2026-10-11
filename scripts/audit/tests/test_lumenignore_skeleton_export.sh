#!/usr/bin/env bash
# WP-02 fix round 8 test (review R3-8, TDD): a non-git export (git archive + git init) must NOT render a different .lumenignore silently.
#
# Purpose   scripts/audit/lumenignore_skeleton.py builds the tree gen_lumenignore.py renders from. When the repository has a .gitmodules path that
#           the listing does not carry as a gitlink (the export lost it), or an allowed root of the scope is absent from the listing, the skeleton is
#           WRONG for that repository: it must fail closed (exit 3), and scripts/audit/scope_drift_check.sh must then record a non-zero
#           lumenignore_fresh (the drift gate fails) instead of a clean render that differs from the committed file by one line.
# Usage     bash scripts/audit/tests/test_lumenignore_skeleton_export.sh      Env: SK=<skeleton under test>, SDC=<drift checker under test>
# Exit      0 all ok; 1 a case failed; 77 a leg was skipped (never a pass)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
RR="$PWD"
SK="${SK:-$RR/scripts/audit/lumenignore_skeleton.py}"
SDC="${SDC:-$RR/scripts/audit/scope_drift_check.sh}"
GL="$RR/constitution/scripts/lumen/gen_lumenignore.py"
T="$(mktemp -d "${TMPDIR:-/tmp}/sk_exp.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0; SKIPN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
skp() { SKIPN=$((SKIPN+1)); echo "SKIP $1 (reason recorded; not a pass)"; }
G() { git -c user.email=t@t -c user.name=t -c commit.gpgsign=false -c protocol.file.allow=always "$@"; }
# a sub repository and a super repository that records it as a gitlink
mkdir -p "$T/sub" && G -C "$T/sub" init -q . && echo x > "$T/sub/f.txt" && G -C "$T/sub" add -A && G -C "$T/sub" commit -q -m s
mkdir -p "$T/super/a" "$T/super/b" && G -C "$T/super" init -q . && : > "$T/super/a/x.go" && : > "$T/super/b/y.go"
G -C "$T/super" submodule add -q "$T/sub" mods/sub >/dev/null 2>&1 || { skp "E0 cannot build the fixture submodule here"; echo "SUMMARY pass=$PASSN fail=$FAILN skip=$SKIPN"; exit 77; }
G -C "$T/super" add -A && G -C "$T/super" commit -q -m super
printf '{"allow":["a","mods/sub"],"deny":["b"],"root_files":true}\n' > "$T/lumen_scope.json"
python3 "$SK" --repo "$T/super" --scope "$T/lumen_scope.json" --out "$T/sk_ok" >/dev/null 2>"$T/e"; RC=$?
[ "$RC" -eq 0 ] && ok "E1 control: the real repository (gitlink present) -> skeleton rc=0" || bad "E1 control: rc=$RC $(head -c 200 "$T/e")"
# the export: git archive drops the submodule content; re-initialise as a plain repository
mkdir -p "$T/exp" && G -C "$T/super" archive HEAD | tar -x -C "$T/exp" && G -C "$T/exp" init -q . && G -C "$T/exp" add -A && G -C "$T/exp" commit -q -m exp
[ -f "$T/exp/.gitmodules" ] && ok "E2 the export keeps .gitmodules (instrument sees the lost gitlink)" || bad "E2 export lost .gitmodules, the fixture does not model the defect"
python3 "$SK" --repo "$T/exp" --scope "$T/lumen_scope.json" --out "$T/sk_exp" >/dev/null 2>"$T/e"; RC=$?
{ [ "$RC" -eq 3 ] && grep -q "gitmodules" "$T/e"; } && ok "E3 export without the gitlink -> skeleton fail closed rc=3 naming .gitmodules" || bad "E3 export: want rc=3 naming .gitmodules, got rc=$RC $(head -c 200 "$T/e")"
printf '{"allow":["a","zz_missing_root"],"deny":[],"root_files":true}\n' > "$T/lumen_scope2.json"
python3 "$SK" --repo "$T/super" --scope "$T/lumen_scope2.json" --out "$T/sk_miss" >/dev/null 2>"$T/e"; RC=$?
{ [ "$RC" -eq 3 ] && grep -q "zz_missing_root" "$T/e"; } && ok "E4 an allowed root absent from the listing -> rc=3 naming the root" || bad "E4 missing allowed root: rc=$RC $(head -c 200 "$T/e")"
# the drift checker: the same export must make lumenignore_fresh non-zero, never a clean render
if [ -f "$GL" ]; then
  : > "$T/scope.yaml"; printf 'import sys\nsys.exit(0)\n' > "$T/ok.py"
  sdc() { SCOPE_RENDER="$T/ok.py" STJ="$T/ok.py" GEN_LUMENIGNORE="$GL" LUMEN_SKELETON="$SK" "$SDC" --repo "$1" --scope "$T/scope.yaml" --lumen-json "$T/lumen_scope.json" --lumenignore "$1/.lumenignore" "${@:2}"; }
  sdc "$T/super" --out "$T/r_ok.json" --regen >/dev/null 2>&1; RC=$?
  [ "$RC" -eq 0 ] && ok "E5 control: drift checker on the real repository, regenerated -> rc=0" || bad "E5 control rc=$RC"
  cp "$T/super/.lumenignore" "$T/exp/.lumenignore"
  sdc "$T/exp" --out "$T/r_exp.json" >/dev/null 2>&1; RC=$?
  rcl="$(jq -r '[.checks[]|select(.check=="lumenignore_fresh")|.rc][0]' "$T/r_exp.json" 2>/dev/null)"
  { [ "$RC" -ne 0 ] && [ "$rcl" != "0" ] && [ "$rcl" != "" ]; } && ok "E6 drift checker on the export -> non-zero (lumenignore_fresh rc=$rcl), not a silent difference" || bad "E6 export drift: rc=$RC lumenignore_fresh=$rcl"
else skp "E5/E6 gen_lumenignore.py absent ($GL)"; fi
echo "SUMMARY pass=$PASSN fail=$FAILN skip=$SKIPN"
[ "$FAILN" -eq 0 ] || exit 1
[ "$SKIPN" -eq 0 ] || exit 77
exit 0
