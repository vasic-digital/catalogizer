#!/usr/bin/env bash
# WP-03 round 11 test (TDD, RED first): scripts/repo/verify_repos.sh, the STRUCTURAL round (constitution 11.4.276) after the third NO-GO review
# (evidence/reviews/WP-03-T037-r3.json): the verifier's dependence on git and environment behaviour, class-complete over the ground-truth table
# evidence/wp03/fix-r6-ground-truth.md (probe: fix-r6-probe.sh). Row ids Pnn below are the rows of that table.
#   P07/P08 forged commit-graph, INTERIOR commit, generation v1 and v2 (the case that fools `git merge-base --is-ancestor`) + tip + control
#   P22/P23/P24 core.excludesFile set through include.path / includeIf / config.worktree = repository level; global (also via an include in the
#   GLOBAL config) stays clean; P28 caller GIT_CONFIG_COUNT/KEY/VALUE; P40-P48 a caller's BASH_ENV / ENV / exported function / SHELLOPTS must reach no child
#   of a DIRECT run (a '#!/bin/bash' git shim, not -p); P13/P14 shallow and P15 alternates (golden: they never turn a failing class into a pass); P30 stated boundary.
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_verify_repos_r10.sh   Env: VR=<verifier path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
FORGE="$(pwd)/scripts/repo/tests/forge_commit_graph.py"
T="$(mktemp -d "${TMPDIR:-/tmp}/vr10_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$VR" ] || echo "NOTE: $VR is absent or not executable (RED state: every case must FAIL)"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) verifier=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) jq=$(jq --version) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$T/fx" "$T/home"
mk() { git init -q --bare -b main "$T/fx/r_$1.git"; git clone -q "$T/fx/r_$1.git" "$T/w_$1" 2>/dev/null
  ( cd "$T/w_$1" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null ); }
run() { local name="$1" root="$2"; shift 2; rm -f "${T:?}/${name:?}.json"
  HOME="$T/home" "$VR" --root "$root" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$T/$name.json" "$@" >"$T/$name.out" 2>"$T/$name.err"; RC=$?; }
row() { jq -r --arg p "$2" ".repos[] | select(.path==\$p) | $3" "$T/$1.json" 2>/dev/null || echo "?"; }
# diverged fixture with an INTERIOR remote commit: local main = c1<-c3 ; remote main = c1<-c2a<-c2 (objects fetched into the local clone)
mkdiv2() { mk "$1"; ( cd "$T/w_$1" && echo l > l && git add l && $G commit -qm c3 )
  rm -rf "${T:?}/o_$1"; git clone -q "$T/fx/r_$1.git" "$T/o_$1" 2>/dev/null
  ( cd "$T/o_$1" && echo y > y && git add y && $G commit -qm c2a && echo z > z && git add z && $G commit -qm c2 && git push -q origin main 2>/dev/null )
  git -C "$T/w_$1" fetch -q origin 2>/dev/null; }
forged() { # forged <name> <genver> : write a commit-graph in w_<name> and forge the interior commit's parent to local c3
  local n="$1" gv="$2" c3 c2a
  c3="$(git -C "$T/w_$n" rev-parse main)"; c2a="$(git -C "$T/w_$n" rev-parse origin/main~1)"
  git -C "$T/w_$n" -c commitGraph.generationVersion="$gv" commit-graph write --reachable --no-changed-paths 2>/dev/null
  python3 -I "$FORGE" "$T/w_$n" "$c2a" "$c3"; }
# ---- P07 forged commit-graph, interior commit: the instrument must not be fooled; control: plain git IS fooled
for gv in 1 2; do
  n="cg$gv"; mkdiv2 "$n"; forged "$n" "$gv"
  c3="$(git -C "$T/w_$n" rev-parse main)"; c2="$(git -C "$T/w_$n" rev-parse origin/main)"
  git -C "$T/w_$n" merge-base --is-ancestor "$c3" "$c2" 2>/dev/null; eq "P07.$gv control: plain git is-ancestor is FOOLED by the forged graph (generation v$gv): rc 0 = the fixture is live" "$?" 0
  git -C "$T/w_$n" -c core.commitGraph=false merge-base --is-ancestor "$c3" "$c2" 2>/dev/null; eq "P07.$gv control: with core.commitGraph=false the truth is rc 1" "$?" 1
  run "$n" "$T/w_$n"; eq "P07.$gv forged interior commit-graph (v$gv): exit 12 DIVERGED, never 0" "$RC" 12
  eq "P07.$gv the class is DIVERGED" "$(row "$n" . '.remotes[0].class')" DIVERGED
done
# P09 tip variant (what the r2 reviewer built): not fooled even by plain git; stays 12
mkdiv2 cgt; git -C "$T/w_cgt" commit-graph write --reachable --no-changed-paths 2>/dev/null; python3 -I "$FORGE" "$T/w_cgt" "$(git -C "$T/w_cgt" rev-parse origin/main)" "$(git -C "$T/w_cgt" rev-parse main)"
run cgt "$T/w_cgt"; eq "P09 forged commit-graph, tip commit: exit 12" "$RC" 12
# repo config core.commitGraph=true must not re-enable it (command line wins, P10) - same fixture as P07.1 plus the config
git -C "$T/w_cg1" config core.commitGraph true; run cgc "$T/w_cg1"; eq "P10 repository core.commitGraph=true cannot re-enable the forged graph: exit 12" "$RC" 12
# ---- P22/P23/P24 core.excludesFile reached through include.path / includeIf / config.worktree is REPOSITORY level
mk i1; echo u > "$T/w_i1/u"; echo u > "$T/i1.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/i1.excl" > "$T/i1.inc"; git -C "$T/w_i1" config include.path "$T/i1.inc"
git -C "$T/w_i1" status --porcelain | grep -q . && bad "P22 control: plain git shows u" || ok "P22 control: plain git reports clean (the include hides u: the fixture is live)"
[ "$(git -C "$T/w_i1" config --local --get core.excludesFile)" = "" ] && ok "P22 control: --local --get does NOT see the included value (why the old code missed it)" || bad "P22 control: --local sees it"
run i1 "$T/w_i1"; eq "P22 include.path in .git/config: exit 13" "$RC" 13; eq "P22 u counted" "$(row i1 . '.dirty_untracked')" 1
grep -E 'hidden only by local ignore rules' "$T/i1.err" | grep -q "[[:space:]]u\([[:space:]]\|$\)" && ok "P22 the refusal names u" || bad "P22 the refusal does not name u: $(head -c 200 "$T/i1.err")"
mk i2; echo u > "$T/w_i2/u"; echo u > "$T/i2.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/i2.excl" > "$T/i2.inc"; printf '[includeIf "gitdir:%s/"]\n\tpath = %s\n' "$T/w_i2" "$T/i2.inc" >> "$T/w_i2/.git/config"
git -C "$T/w_i2" status --porcelain | grep -q . && bad "P23 control: plain git shows u" || ok "P23 control: plain git reports clean (includeIf matched)"
run i2 "$T/w_i2"; eq "P23 includeIf gitdir: exit 13" "$RC" 13
mk i3; echo u > "$T/w_i3/u"; echo u > "$T/i3.excl"; git -C "$T/w_i3" config extensions.worktreeConfig true; git -C "$T/w_i3" config --worktree core.excludesFile "$T/i3.excl"
git -C "$T/w_i3" status --porcelain | grep -q . && bad "P24 control: plain git shows u" || ok "P24 control: plain git reports clean (config.worktree hides u)"
run i3 "$T/w_i3"; eq "P24 config.worktree core.excludesFile: exit 13" "$RC" 13
# golden-FALSE (11.4.201(1)): an include inside the GLOBAL config is still global = clean; a repository value overriding a global one with a file that hides nothing = visible
mk i4; echo u > "$T/w_i4/u"; echo u > "$T/i4.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/i4.excl" > "$T/i4.inc"; printf '[include]\n\tpath = %s\n' "$T/i4.inc" > "$T/i4.glob"
GIT_CONFIG_GLOBAL="$T/i4.glob" git -C "$T/w_i4" status --porcelain | grep -q . && bad "P25 control: plain git shows u" || ok "P25 control: a global-config include hides u from plain git"
GIT_CONFIG_GLOBAL="$T/i4.glob" run i4 "$T/w_i4"; eq "P25 golden-false: the excludes file reached through the GLOBAL config (include) is global: exit 0" "$RC" 0
mk i5; echo u > "$T/w_i5/u"; echo u > "$T/i5.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/i5.excl" > "$T/i5.glob"; git -C "$T/w_i5" config core.excludesFile /nonexistent/none
GIT_CONFIG_GLOBAL="$T/i5.glob" git -C "$T/w_i5" status --porcelain | grep -q . && ok "P27 control: the repository value replaces the global one, git shows u" || bad "P27 control: git hides u"
GIT_CONFIG_GLOBAL="$T/i5.glob" run i5 "$T/w_i5"; eq "P27 a repository core.excludesFile naming a file that does not exist hides nothing: exit 13 (u visible to git too)" "$RC" 13
# P28 a caller's GIT_CONFIG_COUNT/KEY/VALUE (scope command) must not hide anything: the isolation drops it
mk i6; echo u > "$T/w_i6/u"; echo u > "$T/i6.excl"
GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile GIT_CONFIG_VALUE_0="$T/i6.excl" git -C "$T/w_i6" status --porcelain | grep -q . && bad "P28 control: plain git shows u" || ok "P28 control: the caller env hides u from plain git"
GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.excludesFile GIT_CONFIG_VALUE_0="$T/i6.excl" run i6 "$T/w_i6"; eq "P28 caller GIT_CONFIG_COUNT/KEY/VALUE excludes: exit 13" "$RC" 13
# ---- P40-P48 a DIRECT run (the recommended path): no child sees the caller's BASH_ENV / ENV / exported function / SHELLOPTS (a '#!/bin/bash' shim, NOT -p)
mkdiv2 be; printf 'echo ran >> "%s/BASHENV_RAN"\n' "$T" > "$T/benv.sh"
cat > "$T/np_shim" <<SHIM
#!/bin/bash
# records what a NON -p bash child of the verifier sees: BASH_ENV, ENV, an exported function git, SHELLOPTS
echo "BE=\${BASH_ENV-unset} EN=\${ENV-unset} FN=\$(declare -F git >/dev/null 2>&1 && echo fn || echo nofn) SO=\$(case ":\${SHELLOPTS-}:" in *:xtrace:*) echo xtrace ;; *) echo none ;; esac)" >> "$T/np.log"
exec git "\$@"
SHIM
chmod +x "$T/np_shim"
direct() { : > "$T/np.log"; rm -f "${T:?}/BASHENV_RAN"; ( git() { echo FUNC; }; export -f git; HOME="$T/home" BASH_ENV="$T/benv.sh" ENV="$T/benv.sh" VERIFY_GIT="$T/np_shim" "$@" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be.json" >/dev/null 2>&1 ); RC=$?; }
direct "$VR"; eq "P40 direct run with a hostile caller environment: exit 12" "$RC" 12
NCALL="$(wc -l < "$T/np.log")"; [ "$NCALL" -gt 0 ] && ok "P40 control: the non -p shim saw $NCALL git calls" || bad "P40 control: the shim saw no call"
eq "P40 no child saw BASH_ENV" "$(grep -vc 'BE=unset' "$T/np.log")" 0
eq "P44 no child saw ENV" "$(grep -vc 'EN=unset' "$T/np.log")" 0
eq "P47 no child saw an exported function git" "$(grep -vc 'FN=nofn' "$T/np.log")" 0
RAN="$( [ -e "$T/BASHENV_RAN" ] && wc -l < "$T/BASHENV_RAN" || echo 0 )"; eq "P40 the caller's BASH_ENV file never ran in a direct run" "$RAN" 0
direct bash "$VR"; eq "P45 interpreter invocation: exit 12" "$RC" 12
eq "P45 interpreter invocation: no child saw BASH_ENV" "$(grep -vc 'BE=unset' "$T/np.log")" 0
RAN="$( [ -e "$T/BASHENV_RAN" ] && wc -l < "$T/BASHENV_RAN" || echo 0 )"; [ "$RAN" -le 1 ] && ok "P45 the BASH_ENV file ran at most once (bash's own startup, which no script line can prevent; ran $RAN)" || bad "P45 BASH_ENV ran $RAN times"
# SHELLOPTS is exported readonly by bash; set it through env(1) for a direct run
: > "$T/np.log"; env HOME="$T/home" SHELLOPTS=xtrace VERIFY_GIT="$T/np_shim" "$VR" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be2.json" >/dev/null 2>&1; RC=$?
eq "P48 direct run with SHELLOPTS=xtrace: exit 12" "$RC" 12; eq "P48 no child saw SHELLOPTS" "$(grep -vc 'SO=none' "$T/np.log")" 0
# the exported function alone (nothing else dirty) must also be seen: the detector reads what `env` shows a child, not just variable names
: > "$T/np.log"; env -i PATH="$PATH" HOME="$T/home" VERIFY_GIT="$T/np_shim" bash -c 'git() { echo FUNC; }; export -f git; exec "$@"' _ "$VR" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be5.json" >/dev/null 2>&1; RC=$?
eq "P47 an exported function as the ONLY dirt: exit 12" "$RC" 12; eq "P47 no child saw the function" "$(grep -vc 'FN=nofn' "$T/np.log")" 0
# loop guard: a dirty environment that claims to be isolated already fails closed (exit 20), it never re-execs forever or silently proceeds
VERIFY_ISOLATED=1 BASH_ENV="$T/benv.sh" HOME="$T/home" "$VR" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be6.json" >/dev/null 2>"$T/be6.err"; RC=$?
eq "VERIFY_ISOLATED=1 with a dirty environment: exit 20" "$RC" 20; grep -q 'cannot build a clean environment' "$T/be6.err" && ok "the refusal says why" || bad "no reason on stderr: $(head -c 200 "$T/be6.err")"
# the opt-in pass-through is for names a test shim needs; it never re-admits the dangerous families
: > "$T/np.log"; env HOME="$T/home" BASH_ENV="$T/benv.sh" VERIFY_PASS_ENV="BASH_ENV,SHIMVAR" SHIMVAR=1 VERIFY_GIT="$T/np_shim" "$VR" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be3.json" >/dev/null 2>&1; RC=$?
eq "VERIFY_PASS_ENV naming BASH_ENV still exit 12" "$RC" 12; eq "VERIFY_PASS_ENV cannot re-admit BASH_ENV" "$(grep -vc 'BE=unset' "$T/np.log")" 0
# ... and never the GIT_* family: a hostile GIT_INDEX_FILE named in VERIFY_PASS_ENV is still dropped (it would make every git status fail or read another index)
env HOME="$T/home" GIT_INDEX_FILE=/nonexistent/idx VERIFY_PASS_ENV="GIT_INDEX_FILE,GIT_DIR" GIT_DIR="$T/w_be/.git" VERIFY_GIT=git "$VR" --root "$T/w_be" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/be4.json" >/dev/null 2>&1; RC=$?
eq "VERIFY_PASS_ENV naming GIT_INDEX_FILE and GIT_DIR cannot re-admit them: exit 12" "$RC" 12
# ---- P13/P14 shallow, P15 alternates: golden cases, they never turn a failing class into a pass
mk sh1; ( cd "$T/w_sh1" && echo b > b && git add b && $G commit -qm c2 && echo c > c && git add c && $G commit -qm c3 )   # local is 2 commits AHEAD of origin
run sh0 "$T/w_sh1"; eq "P13 control: unshallowed, two commits ahead: exit 11" "$RC" 11
git -C "$T/w_sh1" rev-parse HEAD > "$T/w_sh1/.git/shallow"
run sh1 "$T/w_sh1"; [ "$RC" -ge 11 ] && [ "$RC" -le 14 ] && ok "P13 a shallow file never gives a clean pass (exit $RC)" || bad "P13 shallow: exit $RC"
mkdiv2 al; git clone -q --no-hardlinks "$T/w_al" "$T/al_clone" 2>/dev/null
rm -rf "$T/al_clone/.git/refs/remotes"; mkdir -p "$T/al_clone/.git/objects/info"; echo "$T/w_al/.git/objects" > "$T/al_clone/.git/objects/info/alternates"
git -C "$T/al_clone" remote set-url origin "$T/fx/r_al.git"; git -C "$T/al_clone" checkout -q main 2>/dev/null; git -C "$T/al_clone" reset -q --hard "$(git -C "$T/w_al" rev-parse main)"
run al "$T/al_clone"; [ "$RC" = 12 ] && ok "P15 objects reached through objects/info/alternates: the diverged remote is exit 12" || bad "P15 alternates: exit $RC"
# ---- P30 stated boundary (NOT switched off, by design: core.fileMode=true would refuse every checkout on a filesystem without an exec bit)
mk fm; chmod +x "$T/w_fm/f"; git -C "$T/w_fm" config core.fileMode false
run fm "$T/w_fm"; eq "P30 BOUNDARY (stated, not a fix): core.fileMode=false hides a chmod; the verifier follows git: exit 0" "$RC" 0
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" = 0 ]
