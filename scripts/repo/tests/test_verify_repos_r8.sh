#!/usr/bin/env bash
# WF8 round 9 test (TDD, RED first): scripts/repo/verify_repos.sh, the cases of the independent review of WP-03 / T037
# (evidence/reviews/WP-03-T037.json): R1 replace refs / grafts / commit-graph writes (the object-and-ancestry resolution class), R2 local ignore
# rules hiding untracked work, R3 BASH_ENV through the worker dispatch, R4 a conflicted submodule pin 'U', R5 a stale untracked cache.
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_verify_repos_r8.sh   Env: VR=<verifier path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/vr8_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$VR" ] || echo "NOTE: $VR is absent or not executable (RED state: every case must FAIL)"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) verifier=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) jq=$(jq --version) python=$(python3 -V 2>&1 | cut -d' ' -f2) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$T/fx"
mk() { git init -q --bare -b main "$T/fx/r_$1.git"; git clone -q "$T/fx/r_$1.git" "$T/w_$1" 2>/dev/null
  ( cd "$T/w_$1" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null ); }
run() { local name="$1" root="$2"; shift 2; rm -f "${T:?}/${name:?}.json"
  "$VR" --root "$root" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$T/$name.json" "$@" >"$T/$name.out" 2>"$T/$name.err"; RC=$?; }
row() { jq -r --arg p "$2" ".repos[] | select(.path==\$p) | $3" "$T/$1.json" 2>/dev/null || echo "?"; }
# a DIVERGED repository whose remote tip object is present locally: local c3 and remote c2 are both children of c1
mkdiv() { mk "$1"; ( cd "$T/w_$1" && echo l > l && git add l && $G commit -qm c3 )
  rm -rf "${T:?}/o_$1"; git clone -q "$T/fx/r_$1.git" "$T/o_$1" 2>/dev/null; ( cd "$T/o_$1" && echo y > y && git add y && $G commit -qm c2 && git push -q origin main 2>/dev/null )
  git -C "$T/w_$1" fetch -q origin 2>/dev/null; }
# ---- R1 object / ancestry resolution class
mkdiv d0; run d0 "$T/w_d0"; eq "R1 control: a diverged repository, plain: exit 12" "$RC" 12
mkdiv rp; C2="$(git -C "$T/w_rp" rev-parse origin/main)"; C3="$(git -C "$T/w_rp" rev-parse HEAD)"
FAKE="$(git -c user.email=t@t -c user.name=t -C "$T/w_rp" commit-tree "$C2^{tree}" -p "$C3" -m fake)"; git -C "$T/w_rp" replace "$C2" "$FAKE"
git -C "$T/w_rp" merge-base --is-ancestor "$C3" "$C2" && ok "R1 control: plain git, with the replace ref, reads local an ancestor of the remote tip (the fixture lies)" || bad "R1 control: the replace ref has no effect on plain git"
run rp "$T/w_rp"; eq "R1 a replace ref that makes the remote tip look like a descendant: still DIVERGED, exit 12" "$RC" 12
run rps "$T/w_rp" --strict; eq "R1 the same under --strict: exit 12 and the diverged class (not behind)" "$RC" 12
eq "R1 the diverged problem is listed under --strict" "$(row rps . '.problems|index("diverged")|type')" number
mkdiv gr; C2="$(git -C "$T/w_gr" rev-parse origin/main)"; C3="$(git -C "$T/w_gr" rev-parse HEAD)"; echo "$C2 $C3" > "$T/w_gr/.git/info/grafts"
git -C "$T/w_gr" merge-base --is-ancestor "$C3" "$C2" 2>/dev/null && ok "R1 control: plain git, with the graft, reads local an ancestor of the remote tip" || ok "R1 control SKIP-with-reason: this git ignores info/grafts, the graft cases below are VACUOUS"
run gr "$T/w_gr"; eq "R1 an info/grafts line that rewrites the remote tip's parent: still DIVERGED, exit 12" "$RC" 12
mkdiv ge; C2="$(git -C "$T/w_ge" rev-parse origin/main)"; C3="$(git -C "$T/w_ge" rev-parse HEAD)"; echo "$C2 $C3" > "$T/gfile"
GIT_GRAFT_FILE="$T/gfile" run ge "$T/w_ge"; eq "R1 a caller-set GIT_GRAFT_FILE cannot change the verdict: exit 12" "$RC" 12
mkdiv gn; C2="$(git -C "$T/w_gn" rev-parse origin/main)"; C3="$(git -C "$T/w_gn" rev-parse HEAD)"
FAKE="$(git -c user.email=t@t -c user.name=t -C "$T/w_gn" commit-tree "$C2^{tree}" -p "$C3" -m fake)"; git -C "$T/w_gn" replace "$C2" "$FAKE"
GIT_NO_REPLACE_OBJECTS=0 run gn "$T/w_gn"; eq "R1 a caller-set GIT_NO_REPLACE_OBJECTS=0 cannot re-enable replace refs: exit 12" "$RC" 12
GIT_REPLACE_REF_BASE=refs/zzz/ run gn2 "$T/w_gn"; eq "R1 a caller-set GIT_REPLACE_REF_BASE changes nothing: exit 12" "$RC" 12
mkdiv rf; C2="$(git -C "$T/w_rf" rev-parse origin/main)"; C3="$(git -C "$T/w_rf" rev-parse HEAD)"
FAKE="$(git -c user.email=t@t -c user.name=t -C "$T/w_rf" commit-tree "$C2^{tree}" -p "$C3" -m fake)"; git -C "$T/w_rf" replace "$C2" "$FAKE"
run rf "$T/w_rf" --fetch; eq "R1 the replace ref under --fetch: exit 12" "$RC" 12
# the shallow member of the class: a shallow boundary cuts history but must never turn a divergence into a pass
mk sh; rm -rf "${T:?}/s_sh"; git clone -q --depth 1 "file://$T/fx/r_sh.git" "$T/s_sh" 2>/dev/null; ( cd "$T/s_sh" && echo l > l && git add l && $G commit -qm c3 )
rm -rf "${T:?}/o_sh"; git clone -q "$T/fx/r_sh.git" "$T/o_sh" 2>/dev/null; ( cd "$T/o_sh" && echo y > y && git add y && $G commit -qm c2 && git push -q origin main 2>/dev/null )
git -C "$T/s_sh" fetch -q --depth 1 origin 2>/dev/null; run sh "$T/s_sh"; eq "R1 a shallow clone that diverged from its remote: exit 12 (a shallow file never makes it clean)" "$RC" 12
run shf "$T/s_sh" --fetch; eq "R1 the same under --fetch: exit 12" "$RC" 12
# the write half of the class: a repository-configured fetch.writeCommitGraph must not make --fetch write a commit-graph (objects only)
mk cg; ( cd "$T/w_cg" && git config fetch.writeCommitGraph true && echo l > l && git add l && $G commit -qm c3 )
rm -rf "${T:?}/o_cg"; git clone -q "$T/fx/r_cg.git" "$T/o_cg" 2>/dev/null; ( cd "$T/o_cg" && echo y > y && git add y && $G commit -qm c2 && git push -q origin main 2>/dev/null )
rm -rf "${T:?}/c_cg"; cp -r "$T/w_cg" "$T/c_cg"; git -C "$T/c_cg" fetch -q origin main 2>/dev/null
if [ -e "$T/c_cg/.git/objects/info/commit-graph" ] || [ -d "$T/c_cg/.git/objects/info/commit-graphs" ]; then ok "R1 control: a plain fetch with fetch.writeCommitGraph=true writes a commit-graph (the fixture is live)"; CGLIVE=1
else ok "R1 control SKIP-with-reason: this git wrote no commit-graph on a plain fetch, the absence below is VACUOUS"; CGLIVE=0; fi
rm -rf "${T:?}/w_cg/.git/objects/info/commit-graph" "${T:?}/w_cg/.git/objects/info/commit-graphs"
run cg "$T/w_cg" --fetch; eq "R1 diverged repository under --fetch: exit 12" "$RC" 12
if [ "$CGLIVE" = 0 ]; then ok "R1 --fetch writes no commit-graph: SKIP-with-reason, the control was vacuous on this git (CGLIVE=0)"
elif [ -e "$T/w_cg/.git/objects/info/commit-graph" ] || [ -d "$T/w_cg/.git/objects/info/commit-graphs" ]; then bad "R1 --fetch wrote a commit-graph into the repository under examination"; else ok "R1 --fetch wrote no commit-graph"; fi
# ---- R2 local ignore rules must not hide untracked work
mk x1; echo '*' >> "$T/w_x1/.git/info/exclude"; echo u > "$T/w_x1/u"
git -C "$T/w_x1" status --porcelain | grep -q . && bad "R2 control: plain git status shows the file (the fixture does not hide)" || ok "R2 control: plain git status hides the file (the fixture is live)"
run x1 "$T/w_x1"; eq "R2 .git/info/exclude '*' and a new untracked file: exit 13" "$RC" 13
eq "R2 the row counts the hidden file as untracked" "$(row x1 . '.dirty_untracked')" 1
mk x2; echo 'u' > "$T/x2.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/x2.excl" > "$T/gl2.cfg"; echo u > "$T/w_x2/u"
GIT_CONFIG_GLOBAL="$T/gl2.cfg" run x2 "$T/w_x2"; eq "R2 (round 10, N1: reversible decision) a file hidden ONLY by the global core.excludesFile is clean to git, so exit 0" "$RC" 0
mk x3; echo 'u' > "$T/x3.excl"; git -C "$T/w_x3" config core.excludesFile "$T/x3.excl"; echo u > "$T/w_x3/u"; run x3 "$T/w_x3"; eq "R2 a repository core.excludesFile hiding the file: exit 13" "$RC" 13
mk x4; ( cd "$T/w_x4" && echo 'build/' > .gitignore && git add .gitignore && $G commit -qm ign && git push -q origin main 2>/dev/null; mkdir build; echo b > build/x )
run x4 "$T/w_x4"; eq "R2 golden-false: a COMMITTED .gitignore still ignores build/ : exit 0" "$RC" 0
eq "R2 golden-false: no untracked file counted" "$(row x4 . '.dirty_untracked')" 0
mk x5; echo 'nomatch_zz' >> "$T/w_x5/.git/info/exclude"; run x5 "$T/w_x5"; eq "R2 golden-false: an info/exclude rule that hides nothing: exit 0" "$RC" 0
mk x6; ( cd "$T/w_x6" && mkdir sub && echo '*.log' > sub/.gitignore && git add sub/.gitignore && $G commit -qm ign && git push -q origin main 2>/dev/null; echo l > sub/a.log )
run x6 "$T/w_x6"; eq "R2 golden-false: a committed nested .gitignore ignores sub/a.log : exit 0" "$RC" 0
mk x7; echo 'u' >> "$T/w_x7/.git/info/exclude"; echo u > "$T/w_x7/u"; echo v > "$T/w_x7/v"; run x7 "$T/w_x7"
eq "R2 one file hidden by a local rule plus one visible untracked file: both counted" "$(row x7 . '.dirty_untracked')" 2
# ---- R3 BASH_ENV through the worker dispatch
mk b3; printf 'echo ran >> "%s/BASHENV_RAN"\n' "$T" > "$T/benv.sh"
rm -f "${T:?}/BASHENV_RAN"; BASH_ENV="$T/benv.sh" bash -c true; [ -s "$T/BASHENV_RAN" ] && ok "R3 control: a plain bash -c sources BASH_ENV (the canary is live)" || bad "R3 control: the canary did not run under a plain bash -c"
rm -f "${T:?}/BASHENV_RAN"; BASH_ENV="$T/benv.sh" run b3 "$T/w_b3"; eq "R3 standalone run with a caller BASH_ENV: exit 0" "$RC" 0
[ -e "$T/BASHENV_RAN" ] && bad "R3 the caller's BASH_ENV file ran inside the verifier (worker dispatch)" || ok "R3 the caller's BASH_ENV file never ran"
# ---- R4 a conflicted submodule (pin 'U')
mk cs; mk cp
( cd "$T/w_cp" && $G submodule add -q "$T/fx/r_cs.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_cp" && git checkout -q -b side && cd os && git checkout -q -b s1 && echo 1 > s1 && git add s1 && $G commit -qm s1 && cd .. && git add os && $G commit -qm side \
  && git checkout -q main && cd os && git checkout -q main && git checkout -q -b s2 && echo 2 > s2 && git add s2 && $G commit -qm s2 && cd .. && git add os && $G commit -qm mainmove )
( cd "$T/w_cp" && $G merge side >/dev/null 2>&1 )
eq "R4 fixture: git submodule status reports the conflict as U" "$(git -C "$T/w_cp" submodule status | cut -c1)" U
run cp "$T/w_cp" --no-remote --strict; eq "R4 conflicted submodule, --strict: parent dirty dominates, exit 13" "$RC" 13
eq "R4 the submodule row's pin_state is conflict" "$(row cp os '.pin_state')" conflict
eq "R4 --strict lists pin among the submodule row's problems" "$(row cp os '.problems|index("pin")|type')" number
run cpp "$T/w_cp" --no-remote; eq "R4 plain: the submodule row has no pin problem (strict only)" "$(row cpp os '.problems|index("pin")')" null
eq "R4 plain: pin_state is still conflict" "$(row cpp os '.pin_state')" conflict
# ---- R5 a stale untracked cache must not hide an untracked file (a regression guard; the -c core.untrackedCache=false override is the layer under test)
mk uc; ( cd "$T/w_uc" && mkdir d && echo a > d/a && git add d && $G commit -qm d && git push -q origin main 2>/dev/null && git config core.untrackedCache true && git config core.trustctime false \
  && git config core.checkStat minimal && git update-index --untracked-cache && git status --porcelain -uall >/dev/null && sleep 1 && touch -d '2020-01-01 00:00:00' d . && git status --porcelain -uall >/dev/null \
  && git status --porcelain -uall >/dev/null && echo n > d/new && touch -d '2020-01-01 00:00:00' d . )
CACHED="$(git -C "$T/w_uc" status --porcelain -uall | wc -l)"
if [ "$CACHED" = 0 ]; then ok "R5 control: plain git with the primed stale untracked cache hides the new file (the fixture is live)"; else ok "R5 control SKIP-with-reason: git's own cache validation saw the new file (got $CACHED lines); the override cannot be shown to matter by any fixture found, the mutant is treated as EQUIVALENT in practice (UNCONFIRMED in general)"; fi
run uc "$T/w_uc"; eq "R5 the untracked file under a primed untracked cache is found: exit 13" "$RC" 13
eq "R5 counted as untracked" "$(row uc . '.dirty_untracked')" 1
echo "SUMMARY pass=$PASSN fail=$FAILN"
