#!/usr/bin/env bash
# WF7 round 7 test (TDD): scripts/repo/verify_repos.sh, the cases of the independent round-6 review of the WP-04 verifier
# (I-1 a required filter driver, I-2 tag objects and non-commit tag targets read as undecided, M-5 core.alternateRefsCommand).
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_verify_repos_r7.sh   Env: VR=<verifier path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/vr7_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
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
# ---- I-1 a REQUIRED filter driver: the verifier switches every driver off (empty command); git then DIES on a required one whenever it must hash the file
mk f1; ( cd "$T/w_f1" && echo '*.dat filter=x' > .gitattributes && git config filter.x.clean cat && git config filter.x.smudge cat && git config filter.x.required true \
  && echo hello > a.dat && git add -A && $G commit -qm dat && git push -q origin main 2>/dev/null )
touch "$T/w_f1/a.dat"; git -C "$T/w_f1" status --porcelain >/dev/null 2>&1; eq "I-1 control: plain git status with the required filter configured succeeds" "$?" 0
touch "$T/w_f1/a.dat"; run f1 "$T/w_f1"; eq "I-1 a required filter and a touched (content-identical) file: exit 0, not 20" "$RC" 0
eq "I-1 the repository is not reported dirty" "$(row f1 . '.dirty')" false
echo changed >> "$T/w_f1/a.dat"; run f1b "$T/w_f1"; eq "I-1 golden-false: a really modified file under the required filter is still dirty (exit 13)" "$RC" 13
git -C "$T/w_f1" checkout -q -- a.dat
# the same driver configured only in the GLOBAL config (the git-lfs install layout)
mk f2; ( cd "$T/w_f2" && echo '*.dat filter=x' > .gitattributes && echo hello > a.dat && git add -A && $G commit -qm dat && git push -q origin main 2>/dev/null )
printf '[filter "x"]\n\tclean = cat\n\tsmudge = cat\n\trequired = true\n' > "$T/gl.cfg"
touch "$T/w_f2/a.dat"; GIT_CONFIG_GLOBAL="$T/gl.cfg" git -C "$T/w_f2" status --porcelain >/dev/null 2>&1; eq "I-1 control: plain status with the global required filter succeeds" "$?" 0
touch "$T/w_f2/a.dat"; GIT_CONFIG_GLOBAL="$T/gl.cfg" run f2 "$T/w_f2"; eq "I-1 a required filter set only in the global config: exit 0" "$RC" 0
# a driver whose NAME contains a dot (filter.a.b.clean)
mk f3; ( cd "$T/w_f3" && echo '*.dat filter=a.b' > .gitattributes && git config filter.a.b.clean cat && git config filter.a.b.required true \
  && echo hello > a.dat && git add -A && $G commit -qm dat && git push -q origin main 2>/dev/null )
touch "$T/w_f3/a.dat"; run f3 "$T/w_f3"; eq "I-1 a required filter whose name contains a dot: exit 0" "$RC" 0
# ---- I-2 a tag OBJECT line whose peeled commit is advertised, and a tag on a non-commit object, are not "undecided"
fixture() { # fixture <name>: the parent pins a LOCAL-ONLY commit W of the attached submodule os (the `not our ref` case): definite answer REMOTE-BEHIND
  mk "$1sub"; mk "$1par"
  ( cd "$T/w_$1par" && $G submodule add -q "$T/fx/r_$1sub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main 2>/dev/null && git checkout -q -b wip && echo w > w && git add w && $G commit -qm W && cd .. && git add os && $G commit -qm pinW && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main ); }
fixture b; run b "$T/w_bpar"; eq "I-2 baseline: a pin no remote ref holds: exit 11 (REMOTE-BEHIND)" "$RC" 11
fixture t1; rm -rf "${T:?}/o_t1"; git clone -q "$T/fx/r_t1sub.git" "$T/o_t1" 2>/dev/null; ( cd "$T/o_t1" && $G tag -a -m rel v1 HEAD && git push -q origin v1 2>/dev/null )
eq "I-2 fixture: the annotated tag object is not held by the submodule" "$(git -C "$T/w_t1par/os" cat-file -e "$(git -C "$T/fx/r_t1sub.git" rev-parse refs/tags/v1)" 2>/dev/null && echo held || echo absent)" absent
run t1 "$T/w_t1par"; eq "I-2 an annotated tag whose peeled commit is held does not make the answer undecided: exit 11" "$RC" 11
run t1f "$T/w_t1par" --fetch; eq "I-2 the same under --fetch: exit 11" "$RC" 11
fixture t2; rm -rf "${T:?}/o_t2"; git clone -q "$T/fx/r_t2sub.git" "$T/o_t2" 2>/dev/null; ( cd "$T/o_t2" && git tag blobtag "$(git rev-parse HEAD:f)" && git push -q origin blobtag 2>/dev/null )
run t2 "$T/w_t2par"; eq "I-2 a tag on a held blob cannot hold a commit pin and is no undecided ref: exit 11" "$RC" 11
fixture t3; rm -rf "${T:?}/o_t3"; git clone -q "$T/fx/r_t3sub.git" "$T/o_t3" 2>/dev/null; ( cd "$T/o_t3" && git tag treetag "$(git rev-parse HEAD^{tree})" && git push -q origin treetag 2>/dev/null )
run t3 "$T/w_t3par"; eq "I-2 a tag on a held tree: exit 11" "$RC" 11
# golden-false: a colleague's NEW BRANCH whose tip object is not held locally stays undecided (14): that tip could descend from the pin
fixture n; rm -rf "${T:?}/col_n"; git clone -q "$T/fx/r_nsub.git" "$T/col_n" 2>/dev/null; ( cd "$T/col_n" && git checkout -q -b feature && echo n > n && git add n && $G commit -qm feat && git push -q origin feature 2>/dev/null )
run n "$T/w_npar"; eq "I-2 golden-false: a branch tip not held locally leaves the question undecided: exit 14" "$RC" 14
# golden-false: an annotated tag whose peeled commit is NOT held (a colleague's new commit) is undecided
fixture u; rm -rf "${T:?}/col_u"; git clone -q "$T/fx/r_usub.git" "$T/col_u" 2>/dev/null; ( cd "$T/col_u" && echo u > u && git add u && $G commit -qm newc && $G tag -a -m rel v9 HEAD && git push -q origin v9 2>/dev/null )
run u "$T/w_upar"; eq "I-2 golden-false: an annotated tag over a commit not held locally: exit 14" "$RC" 14
# golden-false: a tag that holds the pin is HELD (exit 0)
fixture h; ( cd "$T/w_hpar/os" && git checkout -q wip && $G tag -a -m rel vp HEAD && git push -q origin vp 2>/dev/null; git checkout -q main )
run h "$T/w_hpar"; eq "I-2 golden-false: a tag whose commit is the pin holds it: exit 0" "$RC" 0
# ---- M-5 core.alternateRefsCommand (a repository-configured program run by git fetch for a repository with an alternate object store)
mk a5; other() { rm -rf "${T:?}/o_$1"; git clone -q "$T/fx/r_$1.git" "$T/o_$1" 2>/dev/null; ( cd "$T/o_$1" && echo y > "$2" && git add "$2" && $G commit -qm other && git push -q origin main 2>/dev/null ); }
other a5 h; ( cd "$T/w_a5" && echo l > l && git add l && $G commit -qm local )
git init -q --bare "$T/alt.git"; echo "$T/alt.git/objects" > "$T/w_a5/.git/objects/info/alternates"
printf '#!/bin/sh\necho ran >> "%s/ALT_RAN"\nexec git --git-dir="$1" for-each-ref --format="%%(objectname)"\n' "$T" > "$T/alt.sh"; chmod +x "$T/alt.sh"
git -C "$T/w_a5" config core.alternateRefsCommand "$T/alt.sh"
rm -f "${T:?}/ALT_RAN"; git -C "$T/w_a5" fetch -q --no-tags --no-write-fetch-head --refmap= origin refs/heads/main >/dev/null 2>&1
if [ -s "$T/ALT_RAN" ]; then ok "M-5 control: a plain fetch runs core.alternateRefsCommand (the fixture is live)"; else ok "M-5 control SKIP-with-reason: this git does not run it here, the absence below is VACUOUS"; fi
rm -f "${T:?}/ALT_RAN"; run a5 "$T/w_a5" --fetch; eq "M-5 diverged repository under --fetch: exit 12" "$RC" 12
[ -e "$T/ALT_RAN" ] && bad "M-5 the repository's core.alternateRefsCommand program ran under the verifier's --fetch" || ok "M-5 core.alternateRefsCommand never ran"
# ---- N6-4 mutation adequacy (reviewer mutant RM3): a PROCESS filter (filter.<n>.process, the git-lfs form) is a repository-configured program too
mk pf; printf '#!/bin/sh\necho ran >> "%s/PROC_RAN"\nexit 1\n' "$T" > "$T/proc.sh"; chmod +x "$T/proc.sh"
( cd "$T/w_pf" && echo '*.dat filter=x' > .gitattributes && echo hello > a.dat && git add -A && $G commit -qm dat && git push -q origin main 2>/dev/null && git config filter.x.process "$T/proc.sh" )
rm -f "${T:?}/PROC_RAN"; touch "$T/w_pf/a.dat"; git -C "$T/w_pf" status --porcelain >/dev/null 2>&1
if [ -s "$T/PROC_RAN" ]; then ok "RM3 control: a plain git status on a touched file runs the process filter (the fixture is live)"; else ok "RM3 control SKIP-with-reason: no process filter run by plain status here, VACUOUS"; fi
rm -f "${T:?}/PROC_RAN"; touch "$T/w_pf/a.dat"; run pf "$T/w_pf"; eq "RM3 a process filter and a touched file: exit 0" "$RC" 0
[ -e "$T/PROC_RAN" ] && bad "RM3 the repository's process filter ran under the verifier" || ok "RM3 the process filter never ran"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
