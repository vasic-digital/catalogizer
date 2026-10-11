#!/usr/bin/env bash
# WP-02 review F4/F5 test (TDD): scripts/audit/scope_drift_check.sh reports scope drift as a machine report.
#
# Purpose   Assert that the report names the three checks, that the REAL committed inputs are drift-free, that each check turns the exit status
#           to 1 and its own rc non-zero when its subject drifts (stub tools and a perturbed copy of .lumenignore), that an absent tool is exit 3
#           (never a silent pass), and that --regen rewrites .lumenignore to the fresh render.
# Usage     bash scripts/audit/tests/test_scope_drift_check.sh      Env: SDC=<script under test> (the mutation harness points it at a mutant)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
SDC="${SDC:-scripts/audit/scope_drift_check.sh}"
T="$(mktemp -d "${TMPDIR:-/tmp}/sdc_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0; SKIPN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
skp() { SKIPN=$((SKIPN+1)); echo "SKIP $1 (reason recorded; not a pass)"; }
rcof() { jq -r --arg c "$1" '[.checks[]|select(.check==$c)|.rc][0] // "MISSING"' "$T/r.json" 2>/dev/null || echo MISSING; }
[ -x "$SDC" ] || echo "NOTE: $SDC absent or not executable (RED state)"
"$SDC" --out "$T/r.json" --fresh-out "$T/fresh" >"$T/o" 2>"$T/e"; RC=$?
[ "$RC" -eq 0 ] && ok "D1 the committed scope inputs are drift-free (rc=0)" || bad "D1 real inputs: rc=$RC $(head -c 300 "$T/e")"
for c in scope_render_check scope_to_lumen_json_check lumenignore_fresh; do
  [ "$(rcof $c)" = 0 ] && ok "D2 report names $c with rc 0" || bad "D2 $c: want rc 0, got $(rcof $c)"
done
[ -s "$T/fresh" ] && cmp -s "$T/fresh" .lumenignore && ok "D3 --fresh-out equals the committed .lumenignore" || bad "D3 fresh render differs from .lumenignore"
# D4 lumenignore drift: a perturbed copy (control: the same tool sees the unperturbed one as clean in D1)
{ cat .lumenignore; echo "/zz_added_by_test/"; } > "$T/li_bad"
"$SDC" --out "$T/r.json" --lumenignore "$T/li_bad" >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 1 ] && [ "$(rcof lumenignore_fresh)" = 1 ] && [ "$(rcof scope_render_check)" = 0 ]; } && ok "D4 stale .lumenignore -> rc=1, only lumenignore_fresh differs" || bad "D4 want rc=1 lumenignore_fresh=1, got rc=$RC $(rcof lumenignore_fresh)"
# D5 lumen_scope.json drift
jq '.allow += ["zz_extra_root"]' config/index/lumen_scope.json > "$T/lj_bad.json"
"$SDC" --out "$T/r.json" --lumen-json "$T/lj_bad.json" >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 1 ] && [ "$(rcof scope_to_lumen_json_check)" = 1 ]; } && ok "D5 drifted lumen_scope.json -> rc=1, scope_to_lumen_json_check=1" || bad "D5 want rc=1, got rc=$RC check=$(rcof scope_to_lumen_json_check)"
# D6 scope_render drift via a stub tool (the real renderer needs the repository's own gitignore state)
printf '#!/bin/sh\nexit 7\n' > "$T/stub_render"; chmod +x "$T/stub_render"
printf 'import sys\nsys.exit(7)\n' > "$T/stub_render.py"
SCOPE_RENDER="$T/stub_render.py" "$SDC" --out "$T/r.json" >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 1 ] && [ "$(rcof scope_render_check)" = 7 ]; } && ok "D6 a failing scope_render --check -> rc=1 with its rc recorded" || bad "D6 want rc=1 render=7, got rc=$RC render=$(rcof scope_render_check)"
# D7 absent generator is exit 3 with rc 127 recorded, never a pass
GEN_LUMENIGNORE="$T/no/such.py" "$SDC" --out "$T/r.json" >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 3 ] && [ "$(rcof lumenignore_fresh)" = 127 ]; } && ok "D7 absent gen_lumenignore.py -> rc=3, check rc 127 (not a pass)" || bad "D7 want rc=3 127, got rc=$RC $(rcof lumenignore_fresh)"
# D8 --regen rewrites the file to the fresh render
cp "$T/li_bad" "$T/li_regen"; "$SDC" --out "$T/r.json" --lumenignore "$T/li_regen" --regen >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 0 ] && cmp -s "$T/li_regen" .lumenignore; } && ok "D8 --regen restores the fresh render (rc=0)" || bad "D8 regen: rc=$RC"
# D10 a generator that writes nothing is blind, not "equal to an empty .lumenignore"
printf 'import sys\nopen(sys.argv[sys.argv.index("--out")+1], "w").close()\n' > "$T/stub_gl.py"; : > "$T/li_empty"
GEN_LUMENIGNORE="$T/stub_gl.py" "$SDC" --out "$T/r.json" --lumenignore "$T/li_empty" >/dev/null 2>&1; RC=$?
{ [ "$RC" -eq 1 ] && [ "$(rcof lumenignore_fresh)" = 1 ]; } && ok "D10 empty fresh render vs empty .lumenignore -> lumenignore_fresh=1 (blind, not equal)" || bad "D10 want rc=1 lumenignore_fresh=1, got rc=$RC $(rcof lumenignore_fresh)"
# D9 usage
"$SDC" >/dev/null 2>&1; [ $? -eq 2 ] && ok "D9 no --out -> usage rc=2" || bad "D9 usage"
# ---- round 9 (review WP-02-T028-r2 N3 and N2): the .lumenignore render depends on TRACKED inputs only, and the report is bound to what it judged
# D11 stray untracked directories on the host must not change the render (a fresh clone has none of them; a stray test directory flipped the gate)
S="$T/sr"; mkdir -p "$S" && git init -q "$S" && mkdir -p "$S/a" "$S/b" "$S/docs" && : > "$S/a/x.go" && : > "$S/b/y.go" && : > "$S/docs/r.md" && : > "$S/top.txt"
git -C "$S" add -A
printf '{"allow":["a"],"deny":[],"root_files":true}\n' > "$S/lumen_scope.json"; : > "$S/scope.yaml"
printf 'import sys\nsys.exit(0)\n' > "$T/ok.py"
GL="$PWD/constitution/scripts/lumen/gen_lumenignore.py"
sdcs() { SCOPE_RENDER="$T/ok.py" STJ="$T/ok.py" GEN_LUMENIGNORE="$GL" "$SDC" --repo "$S" --scope "$S/scope.yaml" --lumen-json "$S/lumen_scope.json" --lumenignore "$S/.lumenignore" "$@"; }
if [ -f "$GL" ]; then
  sdcs --out "$T/s1.json" --regen >/dev/null 2>&1; RC=$?; cp "$S/.lumenignore" "$T/li1"
  { [ "$RC" -eq 0 ] && grep -qx '/b/' "$T/li1" && grep -qx '/docs/' "$T/li1" && ! grep -qx '/a/' "$T/li1"; } && ok "D11a scratch repo: --regen renders the tracked tree (rc=0, /b/ and /docs/ denied, allowed /a/ not)" || bad "D11a scratch regen: rc=$RC $(tr '\n' ' ' < "$T/li1" | cut -c1-200)"
  mkdir -p "$S/prev3" "$S/zz_stray_test_dir" "$S/a/untracked_sub"; : > "$S/zz_untracked_file.txt"
  sdcs --out "$T/s2.json" --regen >/dev/null 2>&1; RC=$?
  { [ "$RC" -eq 0 ] && cmp -s "$S/.lumenignore" "$T/li1"; } && ok "D11b untracked directories prev3, zz_stray_test_dir and an untracked file do not change the render (host independent)" || bad "D11b stray dirs changed the render: rc=$RC $(diff "$T/li1" "$S/.lumenignore" | head -3 | tr '\n' ' ')"
  cp "$T/li1" "$S/.lumenignore"; sdcs --out "$T/s3.json" >/dev/null 2>&1; RC=$?
  { [ "$RC" -eq 0 ] && [ "$(jq -r '[.checks[]|select(.check=="lumenignore_fresh")|.rc][0]' "$T/s3.json" 2>/dev/null)" = 0 ]; } && ok "D11c the committed render stays fresh while stray directories exist (--check mode rc=0)" || bad "D11c check with stray dirs: rc=$RC"
  for hl in /.git/ /.claude/ /.remember/; do grep -qx "$hl" "$T/li1" && ok "D11d fixed host-local entry $hl is always denied (stable, tracked-independent)" || bad "D11d $hl missing from the render"; done
  # D11e a tracked directory added to the index changes the render (the instrument sees tracked structure)
  mkdir -p "$S/newtracked" && : > "$S/newtracked/f.go" && git -C "$S" add newtracked && sdcs --out "$T/s4.json" --regen >/dev/null 2>&1
  grep -qx '/newtracked/' "$S/.lumenignore" && ok "D11e control: a newly TRACKED top-level directory appears in the render" || bad "D11e tracked dir not rendered"
else skp "D11 gen_lumenignore.py absent ($GL)"; fi
# D12 the report is the scope-drift/1 object bound to the scope, the lumen json, the .lumenignore and the HEAD it judged
"$SDC" --out "$T/r12.json" >/dev/null 2>&1
{ [ "$(jq -r .schema "$T/r12.json" 2>/dev/null)" = "scope-drift/1" ] && [ "$(jq -r .scope_sha256 "$T/r12.json" 2>/dev/null)" = "$(sha256sum config/index/scope.yaml | cut -d' ' -f1)" ] \
  && [ "$(jq -r .lumenignore_sha256 "$T/r12.json" 2>/dev/null)" = "$(sha256sum .lumenignore | cut -d' ' -f1)" ] \
  && [ "$(jq -r .lumen_scope_sha256 "$T/r12.json" 2>/dev/null)" = "$(sha256sum config/index/lumen_scope.json | cut -d' ' -f1)" ] \
  && [ -n "$(jq -r '.head // empty' "$T/r12.json" 2>/dev/null)" ] && [ "$(jq -r '.checks|length' "$T/r12.json" 2>/dev/null)" = 3 ]; } \
  && ok "D12 report = scope-drift/1 with head, scope/lumen_scope/lumenignore sha256 equal to the files, and three checks" || bad "D12 report shape/binding: $(head -c 300 "$T/r12.json" 2>/dev/null)"
# D13 a fresh clone (no untracked directory of this host, no submodule checkout) renders the same .lumenignore as the committed one
C="$T/clone"
if [ -f "$GL" ] && git clone -q -n -s "$PWD" "$C" 2>/dev/null && git -C "$C" read-tree HEAD 2>/dev/null \
   && git -C "$C" submodule init -- constitution >/dev/null 2>&1 && git -C "$C" config submodule.constitution.url "$PWD/constitution" \
   && git -C "$C" -c protocol.file.allow=always submodule update --checkout -- constitution >/dev/null 2>&1; then
  SCOPE_RENDER="$T/ok.py" STJ="$T/ok.py" GEN_LUMENIGNORE="$GL" "$SDC" --repo "$C" --scope "$PWD/config/index/scope.yaml" --lumen-json "$PWD/config/index/lumen_scope.json" --lumenignore "$PWD/.lumenignore" --out "$T/r13.json" --fresh-out "$T/fresh13" >/dev/null 2>&1; RC=$?
  { [ "$RC" -eq 0 ] && cmp -s "$T/fresh13" .lumenignore; } && ok "D13 fresh-clone proof: the render from a clone's tracked tree equals the committed .lumenignore (rc=0)" || bad "D13 clone render differs: rc=$RC $(diff "$T/fresh13" .lumenignore 2>&1 | head -4 | tr '\n' ' ')"
else skp "D13 clone proof (git clone, the constitution submodule checkout in the clone, or gen_lumenignore.py unavailable)"; fi
echo "SUMMARY pass=$PASSN fail=$FAILN skip=${SKIPN:-0}"
[ "$FAILN" -eq 0 ] || exit 1
[ "$SKIPN" -eq 0 ] || exit 77
exit 0
