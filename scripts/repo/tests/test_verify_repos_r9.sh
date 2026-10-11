#!/usr/bin/env bash
# WF8 round 10 test (TDD, RED first): scripts/repo/verify_repos.sh, the cases of the independent re-review of WP-03 / T037
# (evidence/reviews/WP-03-T037-r2.json): N1 globally-ignored files are not dirt (and a refusal names its files), N2 core.commitGraph=false on every git
# command, N3 an interpreter invocation does not carry BASH_ENV on, N4 a staged rename's old-path field in the hidden-untracked pass.
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_verify_repos_r9.sh   Env: VR=<verifier path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/vr9_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$VR" ] || echo "NOTE: $VR is absent or not executable (RED state: every case must FAIL)"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) verifier=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) jq=$(jq --version) python=$(python3 -V 2>&1 | cut -d' ' -f2) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$T/fx" "$T/home"
mk() { git init -q --bare -b main "$T/fx/r_$1.git"; git clone -q "$T/fx/r_$1.git" "$T/w_$1" 2>/dev/null
  ( cd "$T/w_$1" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null ); }
# HOME is a private empty directory for every run: no host global config or XDG ignore file leaks into a fixture; a case sets its own via GIT_CONFIG_GLOBAL / XDG_CONFIG_HOME
run() { local name="$1" root="$2"; shift 2; rm -f "${T:?}/${name:?}.json"
  HOME="$T/home" "$VR" --root "$root" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$T/$name.json" "$@" >"$T/$name.out" 2>"$T/$name.err"; RC=$?; }
row() { jq -r --arg p "$2" ".repos[] | select(.path==\$p) | $3" "$T/$1.json" 2>/dev/null || echo "?"; }
mkdiv() { mk "$1"; ( cd "$T/w_$1" && echo l > l && git add l && $G commit -qm c3 )
  rm -rf "${T:?}/o_$1"; git clone -q "$T/fx/r_$1.git" "$T/o_$1" 2>/dev/null; ( cd "$T/o_$1" && echo y > y && git add y && $G commit -qm c2 && git push -q origin main 2>/dev/null )
  git -C "$T/w_$1" fetch -q origin 2>/dev/null; }
# ---- N1 a file ignored ONLY by the user's global excludes file is clean to git, so it is not dirt
mk g1; printf '*.swp\n.idea/\n' > "$T/g1.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/g1.excl" > "$T/g1.cfg"; echo s > "$T/w_g1/.f.swp"; mkdir "$T/w_g1/.idea"; echo i > "$T/w_g1/.idea/ws.xml"
GIT_CONFIG_GLOBAL="$T/g1.cfg" git -C "$T/w_g1" status --porcelain | grep -q . && bad "N1 control: plain git shows the globally ignored files" || ok "N1 control: plain git reports the repository clean (the fixture is live)"
GIT_CONFIG_GLOBAL="$T/g1.cfg" run g1 "$T/w_g1"; eq "N1 global core.excludesFile hiding *.swp and .idea/: exit 0" "$RC" 0
eq "N1 no untracked file counted" "$(row g1 . '.dirty_untracked')" 0
mk g2; mkdir -p "$T/xdg/git"; printf '*.swp\n' > "$T/xdg/git/ignore"; echo s > "$T/w_g2/.f.swp"
XDG_CONFIG_HOME="$T/xdg" git -C "$T/w_g2" status --porcelain | grep -q . && bad "N1 control: plain git shows the XDG-ignored file" || ok "N1 control: plain git with the XDG default ignore file reports clean"
XDG_CONFIG_HOME="$T/xdg" run g2 "$T/w_g2"; eq "N1 the XDG default ignore file (core.excludesFile unset): exit 0" "$RC" 0
# a file hidden by a LOCAL rule stays dirt, also when the global file hides it too; the refusal names the file
mk g3; printf 'u\n' > "$T/g3.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/g3.excl" > "$T/g3.cfg"; echo u >> "$T/w_g3/.git/info/exclude"; echo u > "$T/w_g3/u"
GIT_CONFIG_GLOBAL="$T/g3.cfg" run g3 "$T/w_g3"; eq "N1 hidden by info/exclude AND by the global file: exit 13 (a local rule still hides unpushed work)" "$RC" 13
eq "N1 counted as untracked" "$(row g3 . '.dirty_untracked')" 1
grep -q 'u' "$T/g3.err" && grep -E 'hidden only by local ignore rules' "$T/g3.err" | grep -q "[[:space:]]u\([[:space:]]\|$\)" && ok "N1 the refusal names the offending file on stderr" || bad "N1 the refusal does not name the file: $(cat "$T/g3.err")"
mk g4; printf 'u\n' > "$T/g4.excl"; git -C "$T/w_g4" config core.excludesFile "$T/g4.excl"; echo u > "$T/w_g4/u"; run g4 "$T/w_g4"; eq "N1 a repository-level core.excludesFile hiding the file: exit 13" "$RC" 13
mk g5; printf 'u\n' > "$T/g5.excl"; printf '[core]\n\texcludesFile = %s\n' "$T/g5.excl" > "$T/g5.cfg"; echo u > "$T/w_g5/u"; echo v > "$T/w_g5/v"
GIT_CONFIG_GLOBAL="$T/g5.cfg" run g5 "$T/w_g5"; eq "N1 globally ignored u plus a visible untracked v: exit 13" "$RC" 13
eq "N1 only the visible file is counted" "$(row g5 . '.dirty_untracked')" 1
# ---- N2 / N3 through a recording git shim (the observable is the argument vector and the environment of every git call)
mkdiv s1; printf '#!/bin/bash -p\necho "ENV=${BASH_ENV-unset} ARGS=$*" >> "%s/shim.log"\nexec git "$@"\n' "$T" > "$T/gitshim"; chmod +x "$T/gitshim"
: > "$T/shim.log"; VERIFY_GIT="$T/gitshim" run s1 "$T/w_s1"; eq "N2 control: the diverged fixture through the shim: exit 12" "$RC" 12
MB="$(grep -c 'merge-base --is-ancestor' "$T/shim.log")"; MBC="$(grep 'merge-base --is-ancestor' "$T/shim.log" | grep -c 'core.commitGraph=false')"
[ "$MB" -gt 0 ] && ok "N2 control: the shim saw $MB merge-base calls" || bad "N2 control: the shim saw no merge-base call"
eq "N2 every merge-base --is-ancestor call carries core.commitGraph=false" "$MBC" "$MB"
NCG="$(grep -vc 'core.commitGraph=false' "$T/shim.log")"; eq "N2 no git call at all lacks core.commitGraph=false" "$NCG" 0
printf 'echo ran >> "%s/BASHENV_RAN"\n' "$T" > "$T/benv.sh"
: > "$T/shim.log"; rm -f "${T:?}/BASHENV_RAN"; BASH_ENV="$T/benv.sh" HOME="$T/home" VERIFY_GIT="$T/gitshim" bash "$VR" --root "$T/w_s1" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/n3.json" >/dev/null 2>&1; RC=$?
eq "N3 interpreter invocation with a caller BASH_ENV: exit 12" "$RC" 12
NB="$(grep -vc 'ENV=unset' "$T/shim.log")"; eq "N3 no git call of the verifier sees BASH_ENV" "$NB" 0
RAN="$( [ -e "$T/BASHENV_RAN" ] && wc -l < "$T/BASHENV_RAN" || echo 0 )"
[ "$RAN" -le 1 ] && ok "N3 the caller's BASH_ENV file ran at most once (the interpreter's own startup, which no script line can prevent; ran $RAN)" || bad "N3 the caller's BASH_ENV file ran $RAN times"
# ---- N4 a staged rename carries the old path as one more field; an old path spelled '?? u2' must not count as an untracked entry in the hidden pass
mk rn; ( cd "$T/w_rn" && echo q > "?? u2" && git add -A && $G commit -qm q && git push -q origin main 2>/dev/null; git mv "?? u2" renamed; echo u2 >> .git/info/exclude; echo h > u2 )
run rn "$T/w_rn" --no-remote; eq "N4 staged rename from '?? u2' plus a hidden untracked u2: exit 13" "$RC" 13
eq "N4 the hidden file is counted once" "$(row rn . '.dirty_untracked')" 1
eq "N4 the rename is one tracked change" "$(row rn . '.dirty_tracked')" 1
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" = 0 ]
