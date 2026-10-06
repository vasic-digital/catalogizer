#!/usr/bin/env bash
# WF6 round 6 test (TDD): scripts/repo/verify_repos.sh, the cases of the independent round-6 review (N6-1 .. N6-4, MR1, MR2). A separate file so
# that the round-6 cases run in about a minute; test_verify_repos.sh (the full matrix, ~11 minutes) stays the regression suite and is re-run too.
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_verify_repos_r6.sh   Env: VR=<verifier path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/vr6_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
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
other() {  # other <name> <file>: a second clone pushes a new commit to the remote of <name>
  rm -rf "${T:?}/o_$1"; git clone -q "$T/fx/r_$1.git" "$T/o_$1" 2>/dev/null
  ( cd "$T/o_$1" && echo y > "$2" && git add "$2" && $G commit -qm other && git push -q origin main 2>/dev/null ); }
run() { local name="$1" root="$2"; shift 2; rm -f "${T:?}/${name:?}.json"
  "$VR" --root "$root" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$T/$name.json" "$@" >"$T/$name.out" 2>"$T/$name.err"; RC=$?; }
row() { jq -r --arg p "$2" ".repos[] | select(.path==\$p) | $3" "$T/$1.json" 2>/dev/null || echo "?"; }
# r3mk <name>: sub remote + parent; the submodule os is ATTACHED on main at M; the parent pins R, the tip of the remote branch release
r3mk() { mk "$1""sub"; mk "$1""par"
  ( cd "$T/w_$1par" && $G submodule add -q "$T/fx/r_$1sub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main 2>/dev/null && git checkout -q -b release && echo r > r && git add r && $G commit -qm R && git push -q origin release 2>/dev/null \
      && git checkout -q main && echo m > m && git add m && $G commit -qm M && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q release && cd .. && git add os && $G commit -qm pinR && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main ); }
pinof() { git -C "$T/w_$1par" ls-tree HEAD os | awk '{print $3}'; }
# ---- N6-1 the pin is only advertised under refs/archive/refs/heads/<x> (a tail match of refs/heads/*): no branch or tag holds it
r3mk n1; P="$(pinof n1)"
git -C "$T/fx/r_n1sub.git" update-ref refs/archive/refs/heads/old "$P"; git -C "$T/fx/r_n1sub.git" update-ref -d refs/heads/release
eq "N6-1 fixture: the pin is held only by refs/archive/refs/heads/old" "$(git -C "$T/fx/r_n1sub.git" for-each-ref --contains "$P" --format='%(refname)' refs/heads refs/tags | wc -l | tr -d ' ')" 0
run n1 "$T/w_n1par"; eq "N6-1 a pin advertised only under refs/archive/refs/heads/old is held by NO branch or tag: exit 12" "$RC" 12; eq "N6-1 the submodule row is DIVERGED" "$(row n1 os '.remotes[0].class')" DIVERGED
r3mk n1c; P="$(pinof n1c)"; git -C "$T/fx/r_n1csub.git" update-ref refs/archive/old "$P"; git -C "$T/fx/r_n1csub.git" update-ref -d refs/heads/release
run n1c "$T/w_n1cpar"; eq "N6-1 control: the same pin under refs/archive/old (no tail match): exit 12" "$RC" 12
r3mk n1d; run n1d "$T/w_n1dpar"; eq "N6-1 golden-false: a pin held by the branch release is still held: exit 0" "$RC" 0
# ---- N6-2 a remote tip whose object this clone does not hold cannot be compared: UNKNOWN-DIFFERENT (14), not a definite DIVERGED
r3mk n2
rm -rf "${T:?}/col_n2"; git clone -q "$T/fx/r_n2sub.git" "$T/col_n2" 2>/dev/null; ( cd "$T/col_n2" && git checkout -q release && echo more > more.txt && git add more.txt && $G commit -qm colleague && git push -q origin release 2>/dev/null )
eq "N6-2 fixture: the new release tip is unknown to the submodule" "$(git -C "$T/w_n2par/os" cat-file -e "$(git -C "$T/fx/r_n2sub.git" rev-parse refs/heads/release)^{commit}" 2>/dev/null && echo present || echo absent)" absent
run n2 "$T/w_n2par"; eq "N6-2 pin below a remote branch tip that is not held locally: exit 14, not 12" "$RC" 14; eq "N6-2 the submodule row is UNKNOWN-DIFFERENT" "$(row n2 os '.remotes[0].class')" UNKNOWN-DIFFERENT
# ---- MR1 the pin equals a remote branch tip and its OWN object is absent locally: held through the tip equality
mk m1sub; mk m1par
( cd "$T/w_m1par" && $G submodule add -q "$T/fx/r_m1sub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
rm -rf "${T:?}/o_m1"; $G clone -q --recurse-submodules "$T/fx/r_m1par.git" "$T/o_m1" 2>/dev/null
( cd "$T/o_m1/os" && git checkout -q main && echo u > u && git add u && $G commit -qm pinned_elsewhere && git push -q origin HEAD:refs/heads/release 2>/dev/null && cd .. && git add os && $G commit -qm pin && git push -q origin main 2>/dev/null )
( cd "$T/w_m1par" && git pull -q --no-recurse-submodules --ff-only origin main 2>/dev/null )
eq "MR1 fixture: the pin object is absent from the submodule" "$(git -C "$T/w_m1par/os" cat-file -e "$(pinof m1)^{commit}" 2>/dev/null && echo present || echo absent)" absent
eq "MR1 fixture: the remote branch release equals the pin" "$(git -C "$T/fx/r_m1sub.git" rev-parse refs/heads/release)" "$(pinof m1)"
run m1 "$T/w_m1par"; eq "MR1 a pin equal to a remote branch tip is held even when its object is absent locally: exit 0" "$RC" 0
# ---- N6-3 + MR2: an unreadable exit-map count is exit 20 BEFORE any report leaves the process; every count has its own case
REALJQ="$(command -v jq)"; mkdir -p "$T/shim_jq"
cat > "$T/shim_jq/jq" <<SHIM
#!/usr/bin/env bash
for a in "\$@"; do case "\$a" in "\$JQFAIL") echo "jq: simulated failure" >&2; exit 5 ;; esac; done
exec "$REALJQ" "\$@"
SHIM
chmod +x "$T/shim_jq/jq"
mk c6; echo dirty >> "$T/w_c6/f"
F_DIRTY='[.repos[]|select(.problems|index("dirty"))]|length'
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" run c6j "$T/w_c6"; eq "N6-3 dirty count unreadable, --json FILE: exit 20" "$RC" 20
[ -e "$T/c6j.json" ] && bad "N6-3 a report file exists after an exit-20 count failure" || ok "N6-3 no --json file is written when a count cannot be read"
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" "$VR" --root "$T/w_c6" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 >"$T/c6q.out" 2>"$T/c6q.err"; rc=$?
eq "N6-3 dirty count unreadable, --quiet stdout mode: exit 20" "$rc" 20; eq "N6-3 nothing on stdout (no report copy)" "$(wc -c < "$T/c6q.out" | tr -d ' ')" 0
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" "$VR" --root "$T/w_c6" --owned-orgs fx --exceptions /dev/null --jobs 2 >"$T/c6t.out" 2>"$T/c6t.err"; rc=$?
eq "N6-3 dirty count unreadable, table mode: exit 20" "$rc" 20; eq "N6-3 no table and no summary on stdout" "$(wc -c < "$T/c6t.out" | tr -d ' ')" 0
mk c6b
for spec in 'diverged|[.repos[]|select(.problems|index("diverged"))]|length' 'pin|[.repos[]|select(.problems|index("pin"))]|length' 'behind|[.repos[]|select(.problems|index("behind"))]|length'; do
  nm="${spec%%|*}"; fl="${spec#*|}"
  PATH="$T/shim_jq:$PATH" JQFAIL="$fl" run "c6_$nm" "$T/w_c6b" --strict; eq "MR2 the $nm count unreadable: exit 20 (fail-closed, not 0)" "$RC" 20
  [ -e "$T/c6_$nm.json" ] && bad "MR2 report written for the $nm count failure" || ok "MR2 no report for the $nm count failure"
done
# ---- N6-4 repository-configured programs are not run by the verifier
mk h4
printf '#!/bin/sh\necho ran >> "%s/FSM_RAN"\n' "$T" > "$T/fsm.sh"; chmod +x "$T/fsm.sh"
git -C "$T/w_h4" config core.fsmonitor "$T/fsm.sh"; rm -f "${T:?}/FSM_RAN"; git -C "$T/w_h4" submodule status --recursive >/dev/null 2>&1; git -C "$T/w_h4" ls-files -s >/dev/null 2>&1
if [ -s "$T/FSM_RAN" ]; then ok "N6-4 control: a plain git submodule status / ls-files runs the configured fsmonitor program (the fixture is live)"; else ok "N6-4 control SKIP-with-reason: this git does not run core.fsmonitor for those commands, the absence below is VACUOUS here"; fi
rm -f "${T:?}/FSM_RAN"; run h4 "$T/w_h4"; eq "N6-4 fsmonitor repository: exit 0" "$RC" 0
[ -e "$T/FSM_RAN" ] && bad "N6-4 the repository's core.fsmonitor program ran $(wc -l < "$T/FSM_RAN" | tr -d ' ') time(s)" || ok "N6-4 core.fsmonitor program never ran"
# uploadpack: the program of remote.<name>.uploadpack runs on ls-remote of a local-path remote
mk u4; printf '#!/bin/sh\necho ran >> "%s/UP_RAN"\nexec git-upload-pack "$@"\n' "$T" > "$T/up.sh"; chmod +x "$T/up.sh"
git -C "$T/w_u4" config remote.origin.uploadpack "$T/up.sh"; rm -f "${T:?}/UP_RAN"; git -C "$T/w_u4" ls-remote origin >/dev/null 2>&1
if [ -s "$T/UP_RAN" ]; then ok "N6-4 control: a plain git ls-remote runs the configured remote.origin.uploadpack program (the fixture is live)"; else ok "N6-4 control SKIP-with-reason: uploadpack is not run for this transport here, VACUOUS"; fi
rm -f "${T:?}/UP_RAN"; run u4 "$T/w_u4"; eq "N6-4 uploadpack repository: exit 0" "$RC" 0
[ -e "$T/UP_RAN" ] && bad "N6-4 the remote.origin.uploadpack program ran" || ok "N6-4 remote.origin.uploadpack program never ran (plain mode)"
rm -f "${T:?}/UP_RAN"; run u4f "$T/w_u4" --fetch; [ -e "$T/UP_RAN" ] && bad "N6-4 the uploadpack program ran under --fetch" || ok "N6-4 remote.origin.uploadpack program never ran (--fetch)"
# the FETCH call (only made when a tip differs from the local commit): a diverged repository under --fetch, uploadpack configured
mk u5; other u5 h; ( cd "$T/w_u5" && echo l > l && git add l && $G commit -qm local )
printf '#!/bin/sh\necho ran >> "%s/UP5_RAN"\nexec git-upload-pack "$@"\n' "$T" > "$T/up5.sh"; chmod +x "$T/up5.sh"; git -C "$T/w_u5" config remote.origin.uploadpack "$T/up5.sh"
rm -f "${T:?}/UP5_RAN"; git -C "$T/w_u5" fetch --no-tags --no-write-fetch-head --refmap= --quiet origin refs/heads/main >/dev/null 2>&1
if [ -s "$T/UP5_RAN" ]; then ok "N6-4 control: a plain fetch runs the configured uploadpack program (the fixture is live)"; else ok "N6-4 control SKIP-with-reason: no uploadpack run by a plain fetch here, VACUOUS"; fi
rm -f "${T:?}/UP5_RAN"; run u5 "$T/w_u5" --fetch; eq "N6-4 diverged repository under --fetch: exit 12 (the fetch really decided it)" "$RC" 12
[ -e "$T/UP5_RAN" ] && bad "N6-4 the uploadpack program ran under the verifier's --fetch" || ok "N6-4 the verifier's own fetch never ran the repository uploadpack program"
# the default-branch ls-remote (--symref HEAD) is used when no branch is known: a DETACHED HEAD
mk u6; printf '#!/bin/sh\necho ran >> "%s/UP6_RAN"\nexec git-upload-pack "$@"\n' "$T" > "$T/up6.sh"; chmod +x "$T/up6.sh"
git -C "$T/w_u6" config remote.origin.uploadpack "$T/up6.sh"; git -C "$T/w_u6" checkout -q --detach
rm -f "${T:?}/UP6_RAN"; git -C "$T/w_u6" ls-remote --symref origin HEAD >/dev/null 2>&1; [ -s "$T/UP6_RAN" ] && ok "N6-4 control: a plain ls-remote --symref runs the configured uploadpack program (the fixture is live)" || ok "N6-4 control SKIP-with-reason: VACUOUS"
rm -f "${T:?}/UP6_RAN"; run u6 "$T/w_u6"; eq "N6-4 detached owned repository (default-branch lookup): exit 0" "$RC" 0
[ -e "$T/UP6_RAN" ] && bad "N6-4 the uploadpack program ran in the default-branch lookup" || ok "N6-4 the default-branch lookup never ran the repository uploadpack program"
# filter driver: a configured clean filter runs under `git status` for a touched file; the verifier overrides every configured driver with an empty command
mk f4; printf '#!/bin/sh\necho ran >> "%s/FILT_RAN"\ncat\n' "$T" > "$T/filt.sh"; chmod +x "$T/filt.sh"
( cd "$T/w_f4" && echo '*.dat filter=x' > .gitattributes && git config filter.x.clean "$T/filt.sh" && echo hello > a.dat && git add -A && $G commit -qm dat && git push -q origin main 2>/dev/null )
rm -f "${T:?}/FILT_RAN"; touch "$T/w_f4/a.dat"; git -C "$T/w_f4" status --porcelain >/dev/null 2>&1
if [ -s "$T/FILT_RAN" ]; then ok "N6-4 control: git status on a touched file runs the configured clean filter (the fixture is live)"; else ok "N6-4 control SKIP-with-reason: no filter run by plain status here, VACUOUS"; fi
rm -f "${T:?}/FILT_RAN"; touch "$T/w_f4/a.dat"; run f4 "$T/w_f4"; eq "N6-4 filter repository with a touched file: exit 0 (unfiltered content is identical)" "$RC" 0
[ -e "$T/FILT_RAN" ] && bad "N6-4 the clean filter ran under the verifier" || ok "N6-4 the clean filter never ran"
# the header no longer claims what is false
if sed -n '1,80p' "$VR" | grep -q 'so NO git command of the verifier runs a repository'; then bad "N6-4 the header still says NO git command runs a repository hook (only hooks were covered)"; else ok "N6-4 the false 'NO git command ... runs a repository hook' claim is gone from the header"; fi
sed -n '1,80p' "$VR" | grep -q 'fsmonitor' && ok "N6-4 the header names the other repository-configured programs (fsmonitor)" || bad "N6-4 the header does not name fsmonitor"
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
