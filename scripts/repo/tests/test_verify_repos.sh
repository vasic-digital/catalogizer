#!/usr/bin/env bash
# T031 test (TDD): executing exit-code and mode matrix of scripts/repo/verify_repos.sh (the single recursive verifier).
#
# Purpose   Build fixtures in a temp dir with LOCAL BARE REMOTES and drive the verifier through every mode, asserting the
#           docs/21 IC-37 and data-model section 9 mode rules and the exit map 0/11/12/13/14/15/20 (and nothing else).
# Usage     RUNP IMG-KCOV -- kcov /out/cov bash scripts/repo/tests/test_verify_repos.sh   (git is present in IMG-KCOV)
#           Env: VR=<verifier path> (default scripts/repo/verify_repos.sh; the mutation test T034 points it at a copy).
# Modes     plain | --fetch | --strict | --no-remote, named on every case below. Fixture remotes live under $T/fx/ so the
#           URL organisation segment is `fx`, which every run lists with `--owned-orgs fx` (a repo on `ext/` is third-party).
# Decisions asserted here that docs/21 section 12.2 and data-model section 9 leave to this matrix (recorded in
#           $EV/wp03/exit-code-decisions.md by T032): (a) when several failing classes are present the code is the first of
#           13 (dirty) > 12 (diverged) > 11 (unpushed) > strict 15 > strict-behind (12) > 14, asserted below for EVERY
#           pair of the six classes (section 18); (b) a `behind` row under --strict exits 12: a LOCAL-BEHIND remote is a
#           fast-forward, NOT a divergence; 12 is used only because the exit set {0,11..15,20} has no fitting code, the JSON
#           `problems` list tells the two apart. UNREVIEWED-by-owner: recorded in exit-code-decisions.md.
#           Review fixes (WF-REVIEW-wp02-wp03, B1-B3, I1-I9, m1-m4): sections 19 to 30. Needs git, jq, python3 (the shared
#           organisation parser scripts/audit/org_of.py), sha256sum. shellcheck: run via IMG-SHELLCHECK (see $EV/wp03).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
VR="${VR:-scripts/repo/verify_repos.sh}"; case "$VR" in /*) ;; *) VR="$(pwd)/$VR" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/vr_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
PASSN=0; FAILN=0; ALLRC=""
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$VR" ] || echo "NOTE: $VR is absent or not executable (RED state: every case must FAIL)"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) verifier=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) jq=$(jq --version) python=$(python3 -V 2>&1 | cut -d' ' -f2) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
mkdir -p "$T/fx" "$T/ext"
mk() {  # mk <name> [org]: bare remote + clone on branch main with one commit, pushed
  local org="${2:-fx}"; mkdir -p "$T/$org"
  git init -q --bare -b main "$T/$org/r_$1.git"
  git clone -q "$T/$org/r_$1.git" "$T/w_$1" 2>/dev/null
  ( cd "$T/w_$1" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null )
}
other() {  # other <name> <file> [org]: a second clone pushes a new commit to the remote of <name>
  local org="${3:-fx}"; rm -rf "$T/o_$1"; git clone -q "$T/$org/r_$1.git" "$T/o_$1" 2>/dev/null
  ( cd "$T/o_$1" && echo y > "$2" && git add "$2" && $G commit -qm other && git push -q origin main 2>/dev/null )
}
run() {  # run <name> <repo path> [verifier args...] -> RC, JSON at $T/<name>.json   (env ORGS overrides the owned list, EXC the exceptions file)
  local name="$1" root="$2"; shift 2
  rm -f "$T/$name.json"
  "$VR" --root "$root" --owned-orgs "${ORGS:-fx}" --exceptions "${EXC:-/dev/null}" --quiet --jobs 2 --timeout 20 --json "$T/$name.json" "$@" >"$T/$name.out" 2>"$T/$name.err"; RC=$?; ALLRC="$ALLRC $RC"
}
jq_() { jq -r "$2" "$T/$1.json" 2>/dev/null || echo "?"; }
row() { jq -r --arg p "$2" ".repos[] | select(.path==\$p) | $3" "$T/$1.json" 2>/dev/null || echo "?"; }

# ---- 1 golden clean tree, plain and strict (0) -----------------------------------------------------------------------
mk clean
run clean_plain "$T/w_clean";            eq "golden clean tree, plain: exit" "$RC" 0
run clean_strict "$T/w_clean" --strict;  eq "golden clean tree, --strict: exit" "$RC" 0
eq "clean report records mode booleans (plain)" "$(jq_ clean_plain '[.fetch,.strict,.no_remote]|@csv')" "false,false,false"
eq "clean report records mode booleans (--strict)" "$(jq_ clean_strict '[.fetch,.strict,.no_remote]|@csv')" "false,true,false"
eq "tool constant kept" "$(jq_ clean_plain .tool)" verify_repo.sh
# ---- 2 dirty file (plain 13) ----------------------------------------------------------------------------------------
mk dirty; echo b >> "$T/w_dirty/f"
run dirty "$T/w_dirty";                  eq "dirty file, plain: exit 13" "$RC" 13
eq "dirty row lists problem dirty" "$(row dirty . '.problems|index("dirty")!=null')" true
# ---- 3 unpushed commit (plain 11) -----------------------------------------------------------------------------------
mk ahead; ( cd "$T/w_ahead" && echo x > g && git add g && $G commit -qm c2 )
run ahead "$T/w_ahead";                  eq "unpushed commit, plain: exit 11" "$RC" 11
eq "unpushed class" "$(row ahead . '.remotes[0].class')" REMOTE-BEHIND
# ---- 4 diverged (--fetch 12) ----------------------------------------------------------------------------------------
mk div; other div h; ( cd "$T/w_div" && echo x > g && git add g && $G commit -qm l2 )
run div "$T/w_div" --fetch;              eq "diverged branch, --fetch: exit 12" "$RC" 12
eq "diverged class" "$(row div . '.remotes[0].class')" DIVERGED
# ---- 5 unproven classes map to 14, one fixture each ----------------------------------------------------------------
mk unreach; ( cd "$T/w_unreach" && git remote set-url origin "$T/fx/does_not_exist.git" )
run unreach "$T/w_unreach";              eq "unreachable remote, plain: exit 14" "$RC" 14
eq "UNREACHABLE class" "$(row unreach . '.remotes[0].class')" UNREACHABLE
mk unkdiff; other unkdiff h
run unkdiff "$T/w_unkdiff";              eq "remote tip object absent locally, plain: exit 14" "$RC" 14
eq "UNKNOWN-DIFFERENT class (never guessed)" "$(row unkdiff . '.remotes[0].class')" UNKNOWN-DIFFERENT
mk norb; ( cd "$T/w_norb" && git checkout -q -b feature && echo z > z && git add z && $G commit -qm cf )
run norb "$T/w_norb";                    eq "branch missing on the remote, plain: exit 14" "$RC" 14
eq "NO-REMOTE-BRANCH class" "$(row norb . '.remotes[0].class')" NO-REMOTE-BRANCH
# ---- 6 failing class takes precedence over 14 -----------------------------------------------------------------------
mk unreach_dirty; ( cd "$T/w_unreach_dirty" && git remote set-url origin "$T/fx/does_not_exist.git" && echo b >> f )
run unreach_dirty "$T/w_unreach_dirty";  eq "unreachable remote plus dirty file, plain: exit 13 (failing class precedes 14)" "$RC" 13
# two failing classes: dirty plus diverged (decision a)
mk two; other two h; ( cd "$T/w_two" && echo x > g && git add g && $G commit -qm l2 && echo b >> f )
run two "$T/w_two" --fetch;              eq "dirty plus diverged, --fetch: exit 13 (decision a: dirty first)" "$RC" 13
# ---- 7 stash-only repository (R3: dirty, both counts 0) ------------------------------------------------------------
mk stash; ( cd "$T/w_stash" && echo s >> f && $G stash -q )
run stash "$T/w_stash";                  eq "unexplained stash, plain: exit 13" "$RC" 13
eq "stash row dirty true" "$(row stash . .dirty)" true
eq "stash row dirty_tracked 0" "$(row stash . .dirty_tracked)" 0
eq "stash row dirty_untracked 0" "$(row stash . .dirty_untracked)" 0
eq "stash row problems has dirty" "$(row stash . '.problems|index("dirty")!=null')" true
# ---- 8 CRLF-style exception (plain 0, listed and counted) -----------------------------------------------------------
mk exc; echo b >> "$T/w_exc/f"
wth() { sha256sum < "$1/$2" | cut -c1-64; }; bth() { git -C "$1" cat-file blob "HEAD:$2" | sha256sum | cut -c1-64; }
excrow() {  # excrow <repo-dir> <path-col> <file> [kind] [reason]: a 6-column exception row for the CURRENT hashes
  printf '%s\t%s\t%s\t%s\t%s\t%s\n' "$2" "${4:-dirty}" "$3" "$(wth "$1" "$3")" "$(bth "$1" "$3")" "${5-fixture CRLF quirk}"
}
excrow "$T/w_exc" . f > "$T/exc.tsv"
"$VR" --root "$T/w_exc" --owned-orgs fx --exceptions "$T/exc.tsv" --quiet --jobs 2 --timeout 20 --json "$T/exc.json" >/dev/null 2>&1; RC=$?
eq "excepted dirty row, plain: exit 0" "$RC" 0
eq "exception listed" "$(row exc . '[.excepted,.exception_reason]|@csv')" 'true,"fixture CRLF quirk"'
eq "exception counted in summary.dirty_excepted" "$(jq_ exc .summary.dirty_excepted)" 1
eq "excepted row still counted in summary.dirty" "$(jq_ exc .summary.dirty)" 1
# ---- 9 remote tip rewritten by a forced update (R6, --fetch 12) -----------------------------------------------------
mk forced; ( cd "$T/w_forced" && echo x > g && git add g && $G commit -qm c2 && git push -q origin main 2>/dev/null )
( cd "$T/fx/r_forced.git" && $G update-ref refs/heads/main "$($G commit-tree -m rewritten "$(git rev-parse 'refs/heads/main^{tree}')")" )
run forced "$T/w_forced" --fetch;        eq "remote tip rewritten by a forced update, --fetch: exit 12" "$RC" 12
# ---- 10 owned upstream moved ahead: plain 0 with LOCAL-BEHIND, --strict 12 (decision b) -----------------------------
mk behind; other behind h
run behind_plain "$T/w_behind" --fetch;  eq "upstream moved ahead, --fetch plain: exit 0" "$RC" 0
eq "LOCAL-BEHIND listed in remotes" "$(row behind_plain . '.remotes[0].class')" LOCAL-BEHIND
eq "no behind problem in plain mode" "$(row behind_plain . '.problems|index("behind")')" null
run behind_strict "$T/w_behind" --fetch --strict; eq "upstream moved ahead, --fetch --strict: exit 12 (decision b)" "$RC" 12
eq "behind problem under --strict" "$(row behind_strict . '.problems|index("behind")!=null')" true
c1="$(jq_ behind_plain '.summary.classes|tostring')"; c2="$(jq_ behind_strict '.summary.classes|tostring')"
if [ "$c1" != "?" ] && [ "$c1" = "$c2" ]; then ok "summary classes equal in both modes ($c1)"; else bad "summary classes differ or absent (plain '$c1', strict '$c2')"; fi
# ---- 11 --no-remote on the unpushed fixture (0, mode boolean true) --------------------------------------------------
run noremote "$T/w_ahead" --no-remote;   eq "--no-remote on the unpushed fixture: exit 0" "$RC" 0
eq "--no-remote report records no_remote true" "$(jq_ noremote '[.fetch,.strict,.no_remote]|@csv')" "false,false,true"
run noremote_dirty "$T/w_dirty" --no-remote; eq "--no-remote still decides dirty (13)" "$RC" 13
# ---- 12 submodules: drifted pointer and uninitialised (--strict 15; plain 0 with pin_drift > 0) ---------------------
mk par; mk sub ext
( cd "$T/w_par" && $G submodule add -q "$T/ext/r_sub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
run sub_clean "$T/w_par" --strict;       eq "parent with a clean third-party submodule, --strict: exit 0" "$RC" 0
( cd "$T/w_par/s" && echo n > n && git add n && $G commit -qm subnew )   # the submodule HEAD moves ahead of the gitlink recorded in the parent
run drift_strict "$T/w_par" --strict;    eq "drifted submodule pointer, --strict: exit 15" "$RC" 15
run drift_plain "$T/w_par";              eq "drifted submodule pointer, plain: exit 0" "$RC" 0
eq "plain run reports summary.pin_drift > 0" "$(jq_ drift_plain '.summary.pin_drift>0')" true
eq "pin_state is drifted" "$(row drift_plain s .pin_state)" drifted
( cd "$T/w_par" && git submodule deinit -q -f s 2>/dev/null; rm -rf s; mkdir s )
run uninit_strict "$T/w_par" --strict;   eq "uninitialised submodule, --strict: exit 15" "$RC" 15
run uninit_plain "$T/w_par";             eq "uninitialised submodule, plain: exit 0" "$RC" 0
eq "plain run reports pin_drift > 0 for uninitialised" "$(jq_ uninit_plain '.summary.pin_drift>0')" true
# ---- 13 pinned owned submodule commit that no remote holds (R4) -----------------------------------------------------
mk par2; mk osub
( cd "$T/w_par2" && $G submodule add -q "$T/fx/r_osub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_par2/os" && echo n > n && git add n && $G commit -qm local_pin ) ; ( cd "$T/w_par2" && git add os && $G commit -qm bumpos && git push -q origin main 2>/dev/null )
run r4_fetch "$T/w_par2" --fetch;        eq "R4 pin descends from the remote tip, --fetch: exit 11 (REMOTE-BEHIND)" "$RC" 11
eq "R4 class REMOTE-BEHIND on the submodule row" "$(row r4_fetch os '.remotes[0].class')" REMOTE-BEHIND
other osub k
run r4_nofetch "$T/w_par2";              eq "R4 remote tip object absent locally, no --fetch: exit 14 (UNKNOWN-DIFFERENT)" "$RC" 14
eq "R4 no-fetch class on the submodule row" "$(row r4_nofetch os '.remotes[0].class')" UNKNOWN-DIFFERENT
run r4_div "$T/w_par2" --fetch;          eq "R4 histories split, --fetch: exit 12 (DIVERGED)" "$RC" 12
# ---- 14 report-shape and option equivalence ------------------------------------------------------------------------
"$VR" --root "$T/w_clean" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json-out "$T/jo.json" >/dev/null 2>&1
"$VR" --root "$T/w_clean" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$T/jj.json" >/dev/null 2>&1
a="$(jq -S 'del(.generated_utc)' "$T/jo.json" 2>/dev/null | sha256sum)"; b="$(jq -S 'del(.generated_utc)' "$T/jj.json" 2>/dev/null | sha256sum)"
if [ -s "$T/jo.json" ] && [ "$a" = "$b" ]; then ok "--json and --json-out reports are identical once generated_utc is removed"; else bad "--json vs --json-out differ or are absent"; fi
# ---- 15 --fetch writes objects only (refs and FETCH_HEAD unchanged) ----------------------------------------------
mk fobj; other fobj h
refs_before="$(git -C "$T/w_fobj" for-each-ref | sha256sum)"; fh_before="$(ls "$T/w_fobj/.git/FETCH_HEAD" 2>/dev/null | wc -l)"
run fobj "$T/w_fobj" --fetch
refs_after="$(git -C "$T/w_fobj" for-each-ref | sha256sum)"; fh_after="$(ls "$T/w_fobj/.git/FETCH_HEAD" 2>/dev/null | wc -l)"
if [ -s "$T/fobj.json" ]; then
  eq "--fetch: for-each-ref (remote-tracking refs included) byte-identical before and after" "$refs_after" "$refs_before"
  eq "--fetch: no FETCH_HEAD file created" "$fh_after" "$fh_before"
else bad "--fetch object-only check needs a report and the verifier produced none"; fi
eq "--fetch decided the moved remote (LOCAL-BEHIND, not unproven)" "$(row fobj . '.remotes[0].class')" LOCAL-BEHIND
# ---- 16 blind verifier and usage error (20) ---------------------------------------------------------------------
cat > "$T/blind_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim that hides every porcelain status entry: a verifier that trusts it can never see a dirty file
for a in "$@"; do if [ "$a" = "--porcelain" ]; then exit 0; fi; done
exec git "$@"
SHIM
chmod +x "$T/blind_git.sh"
VERIFY_GIT="$T/blind_git.sh" run blind "$T/w_dirty"; eq "blind needle (status hidden): exit 20" "$RC" 20
"$VR" --no-such-option >/dev/null 2>&1; eq "deliberately broken invocation: exit 20" "$?" 20
# ======================================================================================================================
# Review fixes (WF-REVIEW-wp02-wp03). Every section below was written BEFORE the fix and run against the unmodified verifier
# (RED), see evidence/wp03/review-fix-red.txt.
# ======================================================================================================================
has() { jq -e "$2" "$T/$1.json" >/dev/null 2>&1 && echo true || echo false; }
# ---- 18 exit-code precedence: EVERY pair of the six failing classes (decision a, I3) ---------------------------------
# flags: d dirty (13) | v diverged remote (12) | a ahead remote (11) | p pin drift, strict (15) | b behind remote, strict (12) |
#        u unreachable remote (14). A clean SAME remote rs is always present, so the no-remote rule never fires here.
combo() {  # combo <name> <flags> -> the tree $T/w_c_<name>
  local n="c_$1" f="$2" w="$T/w_c_$1" b
  git init -q -b main "$w"; ( cd "$w" && echo a > f && git add f && $G commit -qm c1 )
  if [[ "$f" == *p* ]]; then ( cd "$w" && $G submodule add -q "$T/ext/r_sub.git" s 2>/dev/null && $G commit -qm addsub ); else ( cd "$w" && echo c2 > g && git add g && $G commit -qm c2 ); fi
  b="$T/fx/r_${n}_rs.git"; git init -q --bare -b main "$b"; ( cd "$w" && git remote add rs "$b" && git push -q rs main 2>/dev/null )
  if [[ "$f" == *a* ]]; then b="$T/fx/r_${n}_ra.git"; git init -q --bare -b main "$b"; ( cd "$w" && git remote add ra "$b" && git push -q ra main~1:refs/heads/main 2>/dev/null ); fi
  if [[ "$f" == *b* ]]; then b="$T/fx/r_${n}_rb.git"; git init -q --bare -b main "$b"; ( cd "$w" && git remote add rb "$b" && git push -q rb main 2>/dev/null )
    rm -rf "$T/o_${n}b"; git clone -q "$b" "$T/o_${n}b" 2>/dev/null; ( cd "$T/o_${n}b" && echo c3 > h && git add h && $G commit -qm c3 && git push -q origin main 2>/dev/null ); fi
  if [[ "$f" == *v* ]]; then b="$T/fx/r_${n}_rd.git"; git init -q --bare -b main "$b"; ( cd "$w" && git remote add rd "$b" && git push -q rd main~1:refs/heads/main 2>/dev/null )
    rm -rf "$T/o_${n}d"; git clone -q "$b" "$T/o_${n}d" 2>/dev/null; ( cd "$T/o_${n}d" && echo c4 > k && git add k && $G commit -qm c4 && git push -q origin main 2>/dev/null ); fi
  [[ "$f" == *u* ]] && ( cd "$w" && git remote add ru "$T/fx/nonexistent_$n.git" )
  [[ "$f" == *p* ]] && ( cd "$w/s" && echo n > n && git add n && $G commit -qm subnew )
  [[ "$f" == *d* ]] && echo b >> "$w/f"
  return 0
}
code_of() { case "$1" in d) echo 13;; v) echo 12;; a) echo 11;; p) echo 15;; b) echo 12;; u) echo 14;; esac; }
present() {  # present <report> <flag>: the condition really exists in the report
  case "$2" in
    d) has "$1" '[.repos[]|select(.problems|index("dirty"))]|length>0';;
    v) has "$1" '[.repos[]|select(.problems|index("diverged"))]|length>0';;
    a) has "$1" '[.repos[]|select(.problems|index("ahead"))]|length>0';;
    p) has "$1" '[.repos[]|select(.problems|index("pin"))]|length>0';;
    b) has "$1" '[.repos[]|select(.problems|index("behind"))]|length>0';;
    u) has "$1" '.summary.unproven>0';;
  esac
}
ORDER="d v a p b u"
combo clean ""; run c_clean "$T/w_c_clean" --fetch --strict; eq "precedence base: only the clean remote, --fetch --strict: exit 0" "$RC" 0
for x in $ORDER; do combo "$x" "$x"; run "c_$x" "$T/w_c_$x" --fetch --strict
  eq "single class $x: exit $(code_of $x)" "$RC" "$(code_of $x)"; eq "single class $x: condition present in the report" "$(present "c_$x" $x)" true; done
set -- $ORDER; i=0
for x in $ORDER; do i=$((i+1)); j=0
  for y in $ORDER; do j=$((j+1)); [ "$j" -le "$i" ] && continue
    combo "$x$y" "$x$y"; run "c_$x$y" "$T/w_c_$x$y" --fetch --strict
    eq "pair $x+$y: exit $(code_of $x) ($x outranks $y)" "$RC" "$(code_of $x)"
    eq "pair $x+$y: both conditions really present" "$(present "c_$x$y" $x)/$(present "c_$x$y" $y)" true/true
  done; done
# ---- 19 B1: every git error is an error, never clean ------------------------------------------------------------
mk cidx; echo b >> "$T/w_cidx/f"; printf 'garbage' > "$T/w_cidx/.git/index"
run cidx "$T/w_cidx";                       eq "B1 corrupt index in the main repository (git status fails): exit 20, never 0" "$RC" 20
grep -q 'verify_repos: cannot' "$T/cidx.err" && ok "B1 the refusal names the problem on stderr" || bad "B1 no 'cannot ...' message on stderr: $(head -c 200 "$T/cidx.err")"
[ ! -s "$T/cidx.json" ] && ok "B1 no report is written for an unverifiable run" || bad "B1 a report was written although a repository could not be verified"
mk cpar; mk csub ext
( cd "$T/w_cpar" && $G submodule add -q "$T/ext/r_csub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
printf 'garbage' > "$T/w_cpar/.git/modules/s/index"; echo b >> "$T/w_cpar/s/f" 2>/dev/null
run csub "$T/w_cpar";                       eq "B1 corrupt index in a submodule: exit 20, never 0" "$RC" 20
grep -q 'verify_repos: cannot' "$T/csub.err" && ok "B1 the refusal names the problem (submodule enumeration fails on the corrupt index)" || bad "B1 message missing: $(head -c 300 "$T/csub.err")"
# the WORKER's own status check: a git whose `status` fails ONLY for the submodule s (enumeration still works)
cat > "$T/statusfail_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `git -C <path ending /s> status ...` fails with rc 128 (a locked or unreadable submodule); everything else is real git
c=""; st=0; prev=""
for a in "$@"; do if [ "$prev" = "-C" ]; then c="$a"; fi; [ "$a" = status ] && st=1; prev="$a"; done
if [ "$st" = 1 ] && [ "${c##*/}" = s ]; then echo "fatal: simulated status failure" >&2; exit 128; fi
exec git "$@"
SHIM
chmod +x "$T/statusfail_git.sh"
mk spar; mk ssub ext; ( cd "$T/w_spar" && $G submodule add -q "$T/ext/r_ssub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
VERIFY_GIT="$T/statusfail_git.sh" run sfail "$T/w_spar"; eq "B1 git status fails for one submodule (worker path): exit 20, never 0" "$RC" 20
grep -q 'verify_repos: cannot verify s: git status failed' "$T/sfail.err" && ok "B1 the worker names the submodule and the failing git status" || bad "B1 worker message missing: $(head -c 300 "$T/sfail.err")"
cat > "$T/subcmd_fail_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: the subcommand named in $FAILCMD fails with rc 128 (and prints nothing); everything else is real git
for a in "$@"; do if [ "$a" = "$FAILCMD" ]; then echo "fatal: simulated $FAILCMD failure" >&2; exit 128; fi; done
exec git "$@"
SHIM
chmod +x "$T/subcmd_fail_git.sh"
FAILCMD=stash VERIFY_GIT="$T/subcmd_fail_git.sh" run stfail "$T/w_spar"; eq "B1 git stash list fails: exit 20, never a clean row" "$RC" 20
mk mbf; ( cd "$T/w_mbf" && echo x > g && git add g && $G commit -qm c2 )
FAILCMD=merge-base VERIFY_GIT="$T/subcmd_fail_git.sh" run mbfail "$T/w_mbf"; eq "B1 git merge-base fails (ancestry unknown): exit 20, not DIVERGED" "$RC" 20
cat > "$T/headfail_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `git rev-parse HEAD` (the plain form) fails with rc 128; --show-toplevel and the rest are real git
[ "${*: -1}" = HEAD ] && case " $* " in *" rev-parse "*) echo "fatal: simulated rev-parse HEAD failure" >&2; exit 128 ;; esac
exec git "$@"
SHIM
chmod +x "$T/headfail_git.sh"
VERIFY_GIT="$T/headfail_git.sh" run hdfail "$T/w_spar"; eq "B1 git rev-parse HEAD fails: exit 20, never a clean row" "$RC" 20
cat > "$T/mb2_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: the SECOND `git merge-base` call of a run fails with rc 128 (the first is real); counter in $MBCOUNT
case " $* " in *" merge-base "*) n=$(( $(cat "$MBCOUNT" 2>/dev/null || echo 0) + 1 )); echo "$n" > "$MBCOUNT"; [ "$n" = 2 ] && { echo "fatal: simulated merge-base failure" >&2; exit 128; } ;; esac
exec git "$@"
SHIM
chmod +x "$T/mb2_git.sh"
mk mb2; other mb2 h; rm -f "$T/mbcount"
MBCOUNT="$T/mbcount" VERIFY_GIT="$T/mb2_git.sh" run mb2fail "$T/w_mb2" --fetch; eq "B1 the second ancestry check (LOCAL-BEHIND test) fails: exit 20, not DIVERGED" "$RC" 20
run sok "$T/w_spar"; eq "B1 control: the same tree with real git is exit 0" "$RC" 0
cat > "$T/errhide_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim that makes `git status --porcelain` FAIL SILENTLY (always exit 0): a verifier that does not probe the error path with its
# control needle can never notice that a corrupt repository reads as clean
for a in "$@"; do if [ "$a" = "--porcelain" ]; then git "$@"; exit 0; fi; done
exec git "$@"
SHIM
chmod +x "$T/errhide_git.sh"
VERIFY_GIT="$T/errhide_git.sh" run errhide "$T/w_spar"; eq "B1 needle probes the error path (a status that hides its failure): exit 20" "$RC" 20
# ---- 20 B2: repositories can not vanish; worker output names are collision-free ----------------------------------
mk gpar; mk gsub ext
( cd "$T/w_gpar" && $G submodule add -q "$T/ext/r_gsub.git" a 2>/dev/null && $G submodule add -q "$T/ext/r_gsub.git" z 2>/dev/null && $G commit -qm two )
echo dirt >> "$T/w_gpar/z/f"; rm -rf "$T/w_gpar/.git/modules/a"
run gone "$T/w_gpar";                       eq "B2 enumeration fails (a gitdir is missing): exit 20, not a silent row loss" "$RC" 20
mk npar; mk nsub ext
( cd "$T/w_npar" && $G submodule add -q "$T/ext/r_nsub.git" a.b 2>/dev/null && $G submodule add -q "$T/ext/r_nsub.git" a_b 2>/dev/null && $G commit -qm two )
echo dirt >> "$T/w_npar/a.b/f"
run coll "$T/w_npar";                       eq "B2 submodules a.b and a_b (same sanitised name): the dirty a.b is reported, exit 13" "$RC" 13
eq "B2 all 3 repositories are reported (rows == listed)" "$(jq_ coll .summary.repos)" 3
eq "B2 a.b row is dirty and a_b row is clean" "$(row coll a.b .dirty)/$(row coll a_b .dirty)" true/false
# ---- 21 B3: exception rows carry the sha256 of the working-tree file and of the committed blob ---------------------
mk e1; echo b >> "$T/w_e1/f"; excrow "$T/w_e1" . f > "$T/e1.tsv"
EXC="$T/e1.tsv" run e1 "$T/w_e1";           eq "B3 exact hashes match: the dirty file is excepted (exit 0)" "$RC" 0
eq "B3 excepted true, reason carried" "$(row e1 . '[.excepted,.exception_reason]|@csv')" 'true,"fixture CRLF quirk"'
echo more > "$T/w_e1/extra.txt"
EXC="$T/e1.tsv" run e1x "$T/w_e1";          eq "B3 an extra UNTRACKED file in an excepted repository: exit 13" "$RC" 13
rm -f "$T/w_e1/extra.txt"; ( cd "$T/w_e1" && echo y >> g 2>/dev/null; git add g 2>/dev/null; $G commit -qm g && echo z >> g )
EXC="$T/e1.tsv" run e1t "$T/w_e1";          eq "B3 an extra TRACKED modification in an excepted repository: exit 13" "$RC" 13
mk e2; echo b >> "$T/w_e2/f"; excrow "$T/w_e2" . f > "$T/e2.tsv"; echo c >> "$T/w_e2/f"
EXC="$T/e2.tsv" run e2 "$T/w_e2";           eq "B3 stale working-tree hash (the file changed again): exit 13, unexcepted dirty" "$RC" 13
eq "B3 stale hash: the row is not excepted" "$(row e2 . .excepted)" false
mk e3; echo b >> "$T/w_e3/f"; excrow "$T/w_e3" . f > "$T/e3.tsv"; cp "$T/w_e3/f" "$T/e3.f"
( cd "$T/w_e3" && git checkout -q -- f && echo ZZ > f && git add f && $G commit -qm blobchange ); cp "$T/e3.f" "$T/w_e3/f"
EXC="$T/e3.tsv" run e3 "$T/w_e3";           eq "B3 stale committed-blob hash (HEAD changed the file): exit 13" "$RC" 13
mk e4; echo b >> "$T/w_e4/f"; excrow "$T/w_e4" . f > "$T/e4.tsv"; ( cd "$T/w_e4" && git stash -q 2>/dev/null; echo b >> f )
EXC="$T/e4.tsv" run e4 "$T/w_e4";           eq "B3 a stash is never excepted (excepted file plus a stash): exit 13" "$RC" 13
mk e5; echo b >> "$T/w_e5/f"; excrow "$T/w_e5" . f other > "$T/e5.tsv"
EXC="$T/e5.tsv" run e5 "$T/w_e5";           eq "I6 an exception row of another kind excepts nothing: exit 13" "$RC" 13
mk e6; echo b >> "$T/w_e6/f"; excrow "$T/w_e6" . f dirty "" > "$T/e6.tsv"
EXC="$T/e6.tsv" run e6 "$T/w_e6";           eq "I6 an exception row with an empty reason excepts nothing: exit 13" "$RC" 13
mk e7; excrow "$T/w_e7" . f > "$T/e7.tsv"
EXC="$T/e7.tsv" run e7 "$T/w_e7";           eq "I6 an exception on a clean repository: exit 0" "$RC" 0
eq "I6 a clean row is never marked excepted" "$(row e7 . '[.excepted,.exception_reason]|@csv')" 'false,'
mk e8; echo b >> "$T/w_e8/f"; printf '.\tdirty\tlegacy three column row\n' > "$T/e8.tsv"
EXC="$T/e8.tsv" run e8 "$T/w_e8";           eq "B3 a legacy 3-column row (no hashes) excepts nothing: exit 13" "$RC" 13
mk e9; echo b >> "$T/w_e9/f"; printf '.\tdirty\tf\t%s\t%s\tshort hash\n' abc def > "$T/e9.tsv"
EXC="$T/e9.tsv" run e9 "$T/w_e9";           eq "B3 malformed hashes except nothing: exit 13" "$RC" 13
mk e10; echo b >> "$T/w_e10/f"; ( cd "$T/w_e10" && echo q >> f; git add f ); excrow "$T/w_e10" . f > "$T/e10.tsv"
EXC="$T/e10.tsv" run e10 "$T/w_e10";        eq "B3 a STAGED change is not an excepted worktree change: exit 13" "$RC" 13
# ---- 22 I1: owned-organisation detection is case-insensitive and trailing-slash safe, shared with derive_scope --------
mk ucase FX; ( cd "$T/w_ucase" && echo x > g && git add g && $G commit -qm c2 )
run ucase "$T/w_ucase";                     eq "I1 remote under FX/ with --owned-orgs fx and an unpushed commit: exit 11" "$RC" 11
mk ucase2; ( cd "$T/w_ucase2" && echo x > g && git add g && $G commit -qm c2 )
ORGS=FX run ucase2 "$T/w_ucase2";           eq "I1 --owned-orgs FX against a remote under fx/: exit 11" "$RC" 11
mk tslash; ( cd "$T/w_tslash" && echo x > g && git add g && $G commit -qm c2 && git remote set-url origin "$T/fx/r_tslash.git/" )
run tslash "$T/w_tslash";                   eq "I1 remote URL with a trailing slash and an unpushed commit: exit 11" "$RC" 11
eq "I1 trailing-slash row is owned" "$(row tslash . .owned)" true
# ---- 23 I2 + I5: the detached owned submodule (R4) path really executes -------------------------------------------
# r4mk <name> <dev yes|no> <headdev yes|no>: remote with main at c1 and dev at c1+cd; parent whose owned submodule os is DETACHED.
# dev=yes: .gitmodules carries branch=dev; headdev=yes: the remote's default branch (HEAD) is dev. Pin = the checked-out tip.
r4mk() {
  local n="$1" w="$T/w_${1}os"
  mk "${n}os"
  ( cd "$w" && git checkout -q -b dev && echo d > d && git add d && $G commit -qm cd && git push -q origin dev 2>/dev/null && git checkout -q main )
  [ "$3" = yes ] && git -C "$T/fx/r_${n}os.git" symbolic-ref HEAD refs/heads/dev
  mk "${n}par"
  if [ "$2" = yes ]; then ( cd "$T/w_${n}par" && $G submodule add -q -b dev "$T/fx/r_${n}os.git" os 2>/dev/null )
  else ( cd "$T/w_${n}par" && $G submodule add -q "$T/fx/r_${n}os.git" os 2>/dev/null ); fi
  ( cd "$T/w_${n}par" && $G commit -qm addos && git push -q origin main 2>/dev/null && git -C os checkout -q --detach )
}
r4mk ra yes no
run r4a "$T/w_rapar" --fetch --strict;      eq "I2a .gitmodules branch=dev, pin = dev tip, main behind: exit 0 (SAME against dev)" "$RC" 0
eq "I2a class SAME on the detached submodule row" "$(row r4a os '.remotes[0].class')" SAME
eq "I2a the row is detached (empty branch)" "$(row r4a os .branch)" ""
r4mk rc no yes
run r4c "$T/w_rcpar" --fetch --strict;      eq "I2c no branch line, remote default branch dev, pin = its tip: exit 0" "$RC" 0
eq "I2c class SAME (the default branch was resolved from the remote HEAD)" "$(row r4c os '.remotes[0].class')" SAME
r4mk rg yes no
( cd "$T/w_rgpar/os" && echo e > e && git add e && $G commit -qm ce && git push -q origin HEAD:dev 2>/dev/null )
run r4g "$T/w_rgpar";                       eq "I2g HEAD reached the dev tip, the recorded pin is older: plain exit 0" "$RC" 0
eq "I2g the pinned gitlink is compared too: LOCAL-BEHIND (pin older than the tip)" "$(row r4g os '.remotes[0].class')" LOCAL-BEHIND
run r4gs "$T/w_rgpar" --strict;             eq "I2g same, --strict: exit 15 (pin drift outranks behind)" "$RC" 15
r4mk rd yes no
( cd "$T/w_rdpar/os" && echo n > n && git add n && $G commit -qm local_pin ) ; ( cd "$T/w_rdpar" && git add os && $G commit -qm bumpos && git push -q origin main 2>/dev/null )
run r4d "$T/w_rdpar";                       eq "I2d pinned commit no remote holds, tip object present, no --fetch: exit 11" "$RC" 11
run r4df "$T/w_rdpar" --fetch;              eq "I2d same with --fetch: exit 11" "$RC" 11
r4mk re no no
( cd "$T/w_repar/os" && echo n > n && git add n && $G commit -qm n && git push -q origin HEAD:main 2>/dev/null \
  && git init -q --bare -b main "$T/fx/r_eosx.git" && git remote add rx "$T/fx/r_eosx.git" && git push -q rx HEAD~1:refs/heads/main 2>/dev/null )
( cd "$T/w_repar" && git add os && $G commit -qm bumpos && git push -q origin main 2>/dev/null )
run r4e "$T/w_repar";                       eq "I2e two remotes, origin SAME and rx behind the pin: exit 11 (R4: every remote)" "$RC" 11
eq "I2e both classes reported" "$(row r4e os '[.remotes[].class]|sort|join(",")')" "REMOTE-BEHIND,SAME"
r4mk rf no no
( cd "$T/w_rfpar/os" && echo n > n && git add n && $G commit -qm unpushed_past_pin )
run r4f "$T/w_rfpar";                       eq "I5 detached HEAD past the pin with a local-only commit, plain: exit 11 (not 0)" "$RC" 11
eq "I5 pin_state is drifted and class REMOTE-BEHIND" "$(row r4f os '[.pin_state,.remotes[0].class]|@csv')" '"drifted","REMOTE-BEHIND"'
# ---- 23b argument injection: a branch / remote value from .gitmodules or a remote never reaches git as an option ------------------
printf '#!/bin/sh\ntouch "%s/PWNED"\n' "$T" > "$T/pwn.sh"; chmod +x "$T/pwn.sh"
r4mk rx no no
orphan="$(git -c user.email=t@t -c user.name=t -C "$T/fx/r_rxos.git" commit-tree -m x 'main^{tree}')"
git -C "$T/fx/r_rxos.git" update-ref "refs/heads/--upload-pack=$T/pwn.sh" "$orphan"
git -C "$T/fx/r_rxos.git" update-ref "refs/heads/-x" "$orphan"
inj() {  # inj <label> <branch value>: the parent's .gitmodules names this branch for the detached owned submodule os
  git -C "$T/w_rxpar" config -f .gitmodules submodule.os.branch "$2"; ( cd "$T/w_rxpar" && $G commit -qam "branch $1" && git push -q origin main 2>/dev/null )
  rm -f "$T/PWNED" "/tmp/x_vr_inj"; run "inj_$1" "$T/w_rxpar" --fetch
  eq "inj $1: exit 14 (unproven, never clean)" "$RC" 14
  eq "inj $1: the submodule row is NO-REMOTE-BRANCH" "$(row "inj_$1" os '.remotes[0].class')" NO-REMOTE-BRANCH
  [ ! -e "$T/PWNED" ] && [ ! -e /tmp/x_vr_inj ] && ok "inj $1: nothing was executed" || bad "inj $1: A COMMAND WAS EXECUTED through the branch value"
  grep -q 'refused' "$T/inj_$1.err" && ok "inj $1: the refusal is named on stderr" || bad "inj $1: no refusal message: $(head -c 200 "$T/inj_$1.err")"
}
inj upload "--upload-pack=$T/pwn.sh"
inj uploadsp "--upload-pack=touch /tmp/x_vr_inj"
inj dash "-x"
inj space "a b"
inj empty ""
# a valid unusual-but-legal name still works (no false refusal): the dev branch of r4mk
git -C "$T/w_rxpar" config -f .gitmodules submodule.os.branch dev; ( cd "$T/w_rxpar" && $G commit -qam okbranch && git push -q origin main 2>/dev/null )
run inj_ok "$T/w_rxpar" --fetch; eq "inj control: a legal branch name (dev) is still compared (exit 11 or 0, not 14)" "$([ "$RC" = 14 ] && echo 14 || echo fine)" fine
# ---- 24 I9: the verifier does not write .git/index ---------------------------------------------------------------
mk idx; sleep 1; touch "$T/w_idx/f"
ix_before="$(sha256sum "$T/w_idx/.git/index" | cut -c1-64) $(stat -c %y "$T/w_idx/.git/index")"
run idx "$T/w_idx"
ix_after="$(sha256sum "$T/w_idx/.git/index" | cut -c1-64) $(stat -c %y "$T/w_idx/.git/index")"
eq "I9 .git/index bytes and mtime unchanged by a run (GIT_OPTIONAL_LOCKS=0)" "$ix_after" "$ix_before"
eq "I9 the touched-but-unchanged tree still reads clean" "$RC" 0
# ---- 25 m1: an uninitialised submodule row must not inherit the parent ----------------------------------------------
mk upar; mk usub ext
( cd "$T/w_upar" && $G submodule add -q "$T/ext/r_usub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
( cd "$T/w_upar" && git submodule deinit -q -f s 2>/dev/null; rm -rf s; mkdir s; echo parentdirt >> f )
run uninit "$T/w_upar";                     eq "m1 uninitialised submodule beside a dirty parent: exit 13 (the parent)" "$RC" 13
eq "m1 the uninitialised row has no head and is not owned" "$(row uninit s '[.head,.owned]|@csv')" '"",false'
eq "m1 the uninitialised row is not dirty and has no remotes" "$(row uninit s '[.dirty,(.remotes|length)]|@csv')" "false,0"
eq "m1 summary.dirty counts the parent once" "$(jq_ uninit .summary.dirty)" 1
# ---- 26 m2: a repository with no remote at all is unproven, never clean -------------------------------------------
git init -q -b main "$T/w_norem"; ( cd "$T/w_norem" && echo a > f && git add f && $G commit -qm c1 )
run norem "$T/w_norem";                     eq "m2 repository with a commit and no remote, plain: exit 14" "$RC" 14
eq "m2 reported in unproven as NO-REMOTE-BRANCH (no pseudo remote, remotes stay empty)" "$(row norem . '[.unproven,(.remotes|length)]|tojson')" '[["NO-REMOTE-BRANCH"],0]'
run norem_nr "$T/w_norem" --no-remote;      eq "m2 --no-remote contacts nothing: exit 0" "$RC" 0
# ---- 27 m3: a submodule path with whitespace --------------------------------------------------------------------
mk wpar; mk wsub ext
( cd "$T/w_wpar" && $G submodule add -q "$T/ext/r_wsub.git" "sp ace" 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null ); echo dirt >> "$T/w_wpar/sp ace/f"
run ws "$T/w_wpar";                         eq "m3 dirty submodule under a path with a space: exit 13" "$RC" 13
eq "m3 the row path is intact" "$(row ws 'sp ace' .dirty)" true
# ---- 28 m4: argument edge cases --------------------------------------------------------------------------------------
"$VR" --root "$T/w_dirty" --self-test >"$T/st.out" 2>&1; rc=$?; ALLRC="$ALLRC $rc"
eq "m4 --self-test after --root runs the self-test only (exit 0, not a full run's 13)" "$rc" 0
grep -q '^SELF-TEST' "$T/st.out" && ok "m4 self-test output present" || bad "m4 no SELF-TEST line"
for bad_arg in "--timeout 0" "--timeout abc" "--timeout -3" "--jobs 0" "--jobs abc" "--jobs -1"; do
  "$VR" --root "$T/w_clean" --owned-orgs fx --exceptions /dev/null --quiet $bad_arg >/dev/null 2>&1; rc=$?; ALLRC="$ALLRC $rc"; eq "m4 $bad_arg: exit 20" "$rc" 20; done
# ---- 29 IC-30, the real tree: owned == derive_scope class, rows == submodule status + 1, the real exception row -----------
VR_OUT="$T/real.json"; "$VR" --root "$(pwd)" --no-remote --quiet --jobs 4 --json "$VR_OUT" >/dev/null 2>"$T/real.err"; rc=$?; ALLRC="$ALLRC $rc"
case "$rc" in 0|13) ok "real tree --no-remote run completes (exit $rc: clean or dirty only)";; *) bad "real tree --no-remote run: exit $rc";; esac
nsub="$(git submodule status --recursive | wc -l)"
eq "A-1 enforced: rows == git submodule status --recursive + 1" "$(jq .summary.repos "$VR_OUT")" "$((nsub+1))"
scripts/audit/derive_scope.sh --root "$(pwd)" --out "$T/ds.tsv" 2>/dev/null; true
dsdiff="$(join -t "$(printf '\t')" -a1 -a2 -e MISSING -o 0,1.2,2.2 -j1 <(cut -f1,2 "$T/ds.tsv" | sort) <(jq -r '.repos[]|select(.path!=".")|[.path,(if .owned then "own" else "third_party" end)]|@tsv' "$VR_OUT" | sort) | awk -F'\t' '$2!=$3' | wc -l)"
eq "m9 on every real submodule row verify_repos owned == derive_scope class (shared parser)" "$dsdiff" 0
awk -F'\t' '$0!~/^#/ && NF>0 {n++; if (NF!=6 || $2!="dirty" || length($4)!=64 || $4~/[^0-9a-f]/ || length($5)!=64 || $5~/[^0-9a-f]/ || length($6)==0) bad_row=1} END{exit (n>0 && !bad_row)?0:1}' scripts/repo/exceptions.tsv \
  && ok "the real exceptions.tsv rows are 6-column with sha256 hashes" || bad "the real exceptions.tsv has a legacy or malformed row"
"$VR" --root "$T/w_clean" --no-remote --exceptions /dev/null --quiet >/dev/null 2>"$T/pend.err"; rc=$?; ALLRC="$ALLRC $rc"
grep -q 'ODG-15' "$T/pend.err" && ok "IC-30 pending accounts are noted on stderr (default own-org list)" || bad "IC-30 no ODG-15 note on stderr"
grep -q 'IC-30' specs/001-full-project-audit-remediation/audit/owed-contract-items.md 2>/dev/null && ok "IC-30 'emit both classifications' is a tracked owed contract item (visible)" || bad "IC-30 owed contract item file missing"
# ---- 30 argument injection through REMOTE NAMES and the remote-announced default branch; the --show-toplevel guard ------------
# 30a: a remote whose NAME could be read as a git option or as several words. That remote's row is refused (class NO-REMOTE-BRANCH,
# named on stderr, nothing sent to git); the clean origin of the same repository is still compared (SAME). `git config` is used
# because `git remote add` itself would read a leading dash as an option.
rcls() { jq -r --arg n "$3" '.repos[]|select(.path==".")|.remotes[]|select(.remote==$n)|.class' "$T/$1.json" 2>/dev/null || echo "?"; }
rn() {  # rn <label> <remote name>: the extra remote carries the hostile name (its url is a real, reachable fixture remote)
  mk "rn_$1"; git -C "$T/w_rn_$1" config "remote.$2.url" "$T/fx/r_rn_$1.git"
  rm -f "$T/PWNED_RN" /tmp/x_vr_rn; run "rn_$1" "$T/w_rn_$1" --fetch
  eq "rn $1: exit 14 (the refused remote is unproven, never clean)" "$RC" 14
  eq "rn $1: the hostile remote's row is NO-REMOTE-BRANCH" "$(rcls "rn_$1" . "$2")" NO-REMOTE-BRANCH
  eq "rn $1: the clean origin is still compared (SAME)" "$(rcls "rn_$1" . origin)" SAME
  [ ! -e "$T/PWNED_RN" ] && [ ! -e /tmp/x_vr_rn ] && ok "rn $1: nothing was executed" || bad "rn $1: A COMMAND WAS EXECUTED through the remote name"
  grep -q 'refused the remote name' "$T/rn_$1.err" && ok "rn $1: the refusal is named on stderr" || bad "rn $1: no refusal message: $(head -c 200 "$T/rn_$1.err")"
}
printf '#!/bin/sh\ntouch "%s/PWNED_RN"\n' "$T" > "$T/pwn_rn.sh"; chmod +x "$T/pwn_rn.sh"
rn upload "--upload-pack=$T/pwn_rn.sh"
rn uploadsp "--upload-pack=touch /tmp/x_vr_rn"
rn dash "-x"
rn space "a b"
mk rn_ok; git -C "$T/w_rn_ok" config remote.second.url "$T/fx/r_rn_ok.git"
run rn_ok "$T/w_rn_ok" --fetch;             eq "rn control: a second, legally named remote is compared, not refused: exit 0" "$RC" 0
eq "rn control: both remotes SAME" "$(row rn_ok . '[.remotes[].class]|unique|join(",")')" SAME
# 30b: the default branch the REMOTE announces (symref of HEAD) is attacker-influenced too: a detached owned submodule with no
# .gitmodules branch reaches it. A value starting with '-' (or with whitespace) is refused, never passed to git.
symref() {  # symref <label> <branch name>: the submodule's remote announces HEAD -> refs/heads/<name>
  r4mk "sy$1" no no
  local o="$T/fx/r_sy${1}os.git" c; c="$(git -c user.email=t@t -c user.name=t -C "$o" commit-tree -m x 'main^{tree}')"
  git -C "$o" update-ref "refs/heads/$2" "$c"; git -C "$o" symbolic-ref HEAD "refs/heads/$2"
  rm -f "$T/PWNED_SY" /tmp/x_vr_sy; run "sy$1" "$T/w_sy${1}par" --fetch
  eq "symref $1: exit 14 (unproven, never clean)" "$RC" 14
  eq "symref $1: the submodule row is NO-REMOTE-BRANCH" "$(row "sy$1" os '.remotes[0].class')" NO-REMOTE-BRANCH
  [ ! -e "$T/PWNED_SY" ] && [ ! -e /tmp/x_vr_sy ] && ok "symref $1: nothing was executed" || bad "symref $1: A COMMAND WAS EXECUTED through the announced branch"
  grep -q 'refused the default branch name' "$T/sy$1.err" && ok "symref $1: the refusal is named on stderr" || bad "symref $1: no refusal message: $(head -c 200 "$T/sy$1.err")"
}
printf '#!/bin/sh\ntouch "%s/PWNED_SY"\n' "$T" > "$T/pwn_sy.sh"; chmod +x "$T/pwn_sy.sh"
symref upload "--upload-pack=$T/pwn_sy.sh"
symref dash "-x"
# (a remote HEAD naming a branch with whitespace cannot be built as a fixture: git refuses such a ref name locally; the worker's symref pattern
# also excludes whitespace, so such an announcement yields no branch at all. Not covered by a fixture: stated in docs/scripts/verify_repos.md limits.)
# 30c: --show-toplevel guard: the path a worker examines must BE the repository git resolves it to. A directory inside a repository
# is not a repository root: git would answer for the enclosing one. Never read as that repository's verdict.
mk tl; mkdir "$T/w_tl/sub"; echo x > "$T/w_tl/sub/x"; ( cd "$T/w_tl" && git add sub && $G commit -qm sub && git push -q origin main 2>/dev/null )
run tl_sub "$T/w_tl/sub";                   eq "tl a --root inside a repository (not its root): exit 20" "$RC" 20
grep -q 'resolves this path to another repository' "$T/tl_sub.err" && ok "tl the refusal names the other repository" || bad "tl no 'another repository' message: $(head -c 200 "$T/tl_sub.err")"
[ ! -s "$T/tl_sub.json" ] && ok "tl no report is written" || bad "tl a report was written for a path that is not a repository root"
cat > "$T/tl_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `git -C <path ending /s> rev-parse --show-toplevel` answers with the PARENT repository (git resolving a submodule path to
# another repository); everything else is real git
c=""; prev=""; tl=0
for a in "$@"; do [ "$prev" = "-C" ] && c="$a"; [ "$a" = "--show-toplevel" ] && tl=1; prev="$a"; done
if [ "$tl" = 1 ] && [ "${c##*/}" = s ]; then echo "$TL_PARENT"; exit 0; fi
exec git "$@"
SHIM
chmod +x "$T/tl_git.sh"
TL_PARENT="$T/w_spar" VERIFY_GIT="$T/tl_git.sh" run tl_shim "$T/w_spar"; eq "tl a submodule row that git resolves to the parent: exit 20, never a clean row" "$RC" 20
grep -q 'cannot verify s: git resolves this path to another repository' "$T/tl_shim.err" && ok "tl the worker names the submodule" || bad "tl worker message missing: $(head -c 300 "$T/tl_shim.err")"
ln -s "$T/w_tl" "$T/lnk_tl"; run tl_lnk "$T/lnk_tl"; eq "tl control: --root through a symlink to the repository root is the same repository: exit 0" "$RC" 0
# ======================================================================================================================
# Review fixes round 3 (WF2-REVIEW-wp02-wp03): N-B1, N-B2, N-I1..N-I4, minors m-a, m-b, m-c, m-e, m-h. Every section below was
# written BEFORE the fix and run against the unmodified verifier (RED), see evidence/wp03/review-fix3-red-verify-repos.txt.
# ======================================================================================================================
snap() { ( cd "$1/.git" && find . -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -c1-64 ); }
# ---- 31 N-B1: an inherited repository-selecting git environment is scrubbed; the needle never writes into another repository --------
git init -q -b main "$T/sentinel"; ( cd "$T/sentinel" && echo s > s && git add s && $G commit -qm s )
sb="$(snap "$T/sentinel")"; sh_="$(git -C "$T/sentinel" rev-parse HEAD)"
mk envroot
GIT_DIR="$T/sentinel/.git" GIT_WORK_TREE="$T/sentinel" GIT_INDEX_FILE="$T/sentinel/.git/index" run envall "$T/w_envroot"
eq "N-B1 GIT_DIR+GIT_WORK_TREE+GIT_INDEX_FILE pointing at another repository: the real root is verified, exit 0" "$RC" 0
eq "N-B1 the sentinel repository is byte-identical afterwards (no needle commit, no index write)" "$(snap "$T/sentinel")" "$sb"
eq "N-B1 the sentinel HEAD did not move" "$(git -C "$T/sentinel" rev-parse HEAD)" "$sh_"
eq "N-B1 the report is about the real root (path ., one repo)" "$(jq_ envall '[.summary.repos,.repos[0].path]|@csv')" '1,"."'
GIT_INDEX_FILE="$T/sentinel/.git/index" run envidx "$T/w_envroot"
eq "N-B1 GIT_INDEX_FILE alone: exit 0" "$RC" 0
eq "N-B1 GIT_INDEX_FILE alone: sentinel byte-identical" "$(snap "$T/sentinel")" "$sb"
GIT_DIR="$T/sentinel/.git" run envdir "$T/w_envroot"
eq "N-B1 GIT_DIR alone: exit 0" "$RC" 0
eq "N-B1 GIT_DIR alone: sentinel byte-identical" "$(snap "$T/sentinel")" "$sb"
GIT_OBJECT_DIRECTORY="$T/sentinel/.git/objects" GIT_COMMON_DIR="$T/sentinel/.git" run envobj "$T/w_envroot"
eq "N-B1 GIT_OBJECT_DIRECTORY+GIT_COMMON_DIR: exit 0" "$RC" 0
eq "N-B1 GIT_OBJECT_DIRECTORY+GIT_COMMON_DIR: sentinel byte-identical (no object written)" "$(snap "$T/sentinel")" "$sb"
GIT_DIR="$T/sentinel/.git" "$VR" --self-test >"$T/envst.out" 2>&1; rc=$?; ALLRC="$ALLRC $rc"
eq "N-B1 --self-test under GIT_DIR: exit 0" "$rc" 0
eq "N-B1 --self-test under GIT_DIR: sentinel byte-identical" "$(snap "$T/sentinel")" "$sb"
# a global core.hooksPath hook must never run (the verifier observes only)
mkdir -p "$T/ghooks"; for h in pre-commit post-commit commit-msg prepare-commit-msg post-checkout reference-transaction pre-merge-commit post-index-change; do
  printf '#!/bin/sh\ntouch "%s/GHOOK_RAN"\n' "$T" > "$T/ghooks/$h"; chmod +x "$T/ghooks/$h"; done
printf '[core]\n\thooksPath = %s\n' "$T/ghooks" > "$T/ghooks.cfg"; rm -f "$T/GHOOK_RAN"
GIT_CONFIG_GLOBAL="$T/ghooks.cfg" run ghook "$T/w_envroot"
eq "N-B1 a global hooksPath: exit 0" "$RC" 0
[ ! -e "$T/GHOOK_RAN" ] && ok "N-B1 no global hook ran during a verification" || bad "N-B1 a global git hook was executed by the verifier"
# hook-exported variables: a pre-commit hook that runs the verifier, in the main worktree and in a linked worktree
cat > "$T/hk_hook.sh" <<'HOOK'
#!/usr/bin/env bash
# pre-commit hook under test: runs the verifier (recursion guard: a runaway chain stops at 5 and is then visible in the counter)
n="$(cat "$HKCNT" 2>/dev/null || echo 0)"; n=$((n+1)); echo "$n" > "$HKCNT"; [ "$n" -gt 5 ] && exit 0
"$VRH" --root "$HKROOT" --owned-orgs fx --exceptions /dev/null --quiet --jobs 1 --timeout 20 --no-remote --json "$HKCNT.json.$n" >/dev/null 2>"$HKCNT.err.$n"
echo $? > "$HKCNT.rc.$n"
exit 0
HOOK
export VRH="$VR"
mk hk; cp "$T/hk_hook.sh" "$T/w_hk/.git/hooks/pre-commit"; chmod +x "$T/w_hk/.git/hooks/pre-commit"
( cd "$T/w_hk" && echo wt >> f )
export HKCNT="$T/hkA.cnt" HKROOT="$T/w_hk"; rm -f "$HKCNT"*
( cd "$T/w_hk" && $G commit -q -a -m user ) >"$T/hkA.out" 2>&1; rc=$?
eq "N-B1 main worktree: 'git commit -a' with a pre-commit hook that runs the verifier SUCCEEDS" "$rc" 0
eq "N-B1 main worktree: the hook ran exactly once (no recursion)" "$(cat "$T/hkA.cnt" 2>/dev/null || echo 0)" 1
eq "N-B1 main worktree: the user commit is the only new commit (2 in total)" "$(git -C "$T/w_hk" rev-list --count HEAD)" 2
eq "N-B1 main worktree: no needle.txt in HEAD" "$(git -C "$T/w_hk" ls-tree -r --name-only HEAD | grep -c needle)" 0
eq "N-B1 main worktree: the verifier inside the hook judged the real repository (dirty: 13)" "$(cat "$T/hkA.cnt.rc.1" 2>/dev/null || echo none)" 13
git -C "$T/w_hk" worktree add -q "$T/lw_hk" -b lwb 2>/dev/null
( cd "$T/lw_hk" && echo lw > lwfile && git add lwfile )
export HKCNT="$T/hkB.cnt" HKROOT="$T/lw_hk"; rm -f "$HKCNT"*
( cd "$T/lw_hk" && $G commit -q -m linked ) >"$T/hkB.out" 2>&1; rc=$?
eq "N-B1 linked worktree: the commit with the hook SUCCEEDS" "$rc" 0
eq "N-B1 linked worktree: the hook ran exactly once (no runaway recursion)" "$(cat "$T/hkB.cnt" 2>/dev/null || echo 0)" 1
eq "N-B1 linked worktree: the user commit is the only new commit on the branch" "$(git -C "$T/lw_hk" rev-list --count lwb ^main)" 1
eq "N-B1 linked worktree: no needle.txt in the branch tip" "$(git -C "$T/lw_hk" ls-tree -r --name-only lwb | grep -c needle)" 0
eq "N-B1 linked worktree: the verifier inside the hook judged the linked worktree (staged: 13)" "$(cat "$T/hkB.cnt.rc.1" 2>/dev/null || echo none)" 13
unset HKCNT HKROOT VRH
# ---- 32 N-B2: a staged, uncommitted submodule pointer change is dirt ---------------------------------------------------
mk sgpar; mk sgsub ext
( cd "$T/w_sgpar" && $G submodule add -q "$T/ext/r_sgsub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
( cd "$T/w_sgpar/s" && echo n > n && git add n && $G commit -qm subnew ) ; ( cd "$T/w_sgpar" && git add s )
run sg "$T/w_sgpar";                        eq "N-B2 staged gitlink change, plain: exit 13 (not 0)" "$RC" 13
run sg_s "$T/w_sgpar" --strict;             eq "N-B2 staged gitlink change, --strict: exit 13 (not 0)" "$RC" 13
eq "N-B2 the parent row is dirty with a tracked change" "$(row sg . '[.dirty,(.dirty_tracked>=1)]|@csv')" "true,true"
run sg_nr "$T/w_sgpar" --no-remote;         eq "N-B2 staged gitlink change, --no-remote: exit 13" "$RC" 13
( cd "$T/w_sgpar" && $G commit -qm bump )
run sg_c "$T/w_sgpar" --no-remote --strict; eq "N-B2 control: the same pointer COMMITTED is clean: exit 0" "$RC" 0
mk sgpar2; mk sgsub2 ext
( cd "$T/w_sgpar2" && $G submodule add -q "$T/ext/r_sgsub2.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null && echo b >> f )
( cd "$T/w_sgpar2/s" && echo n > n && git add n && $G commit -qm subnew ) ; ( cd "$T/w_sgpar2" && git add s )
excrow "$T/w_sgpar2" . f > "$T/sg2.tsv"
EXC="$T/sg2.tsv" run sg2 "$T/w_sgpar2";     eq "N-B2 a staged gitlink change is never excepted, even beside an excepted file: exit 13" "$RC" 13
eq "N-B2 the row is not excepted" "$(row sg2 . .excepted)" false
mk sga; mk sgasub ext   # the mapping is committed first (git refuses to enumerate an unmapped gitlink: exit 20), the gitlink itself is only staged
( cd "$T/w_sga" && git config -f .gitmodules submodule.extra.path extra && git config -f .gitmodules submodule.extra.url "$T/ext/r_sgasub.git" && git add .gitmodules && $G commit -qm map \
  && $G update-index --add --cacheinfo 160000,2222222222222222222222222222222222222222,extra )
run sga "$T/w_sga" --no-remote;             eq "N-B2 a staged ADDED gitlink (no .gitmodules edit): exit 13" "$RC" 13
mk sgd; mk sgdsub ext
( cd "$T/w_sgd" && $G submodule add -q "$T/ext/r_sgdsub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null && git update-index --force-remove s && rm -rf s )   # (git rm --cached would stage a .gitmodules edit, and a left-over s/ shows as untracked: both would show it)
run sgd "$T/w_sgd" --no-remote;             eq "N-B2 a staged REMOVED gitlink: exit 13" "$RC" 13
FAILCMD=diff-index VERIFY_GIT="$T/subcmd_fail_git.sh" run sgfail "$T/w_spar"; eq "N-B2 git diff-index --cached fails: exit 20, never a clean row" "$RC" 20
FAILCMD=ls-files VERIFY_GIT="$T/subcmd_fail_git.sh" run lsvfail "$T/w_spar";  eq "N-I2 git ls-files -v fails: exit 20, never a clean row" "$RC" 20
# ---- 33 N-I1: a repository-level status.showUntrackedFiles=no must not hide untracked work ----------------------------
mk uf; git -C "$T/w_uf" config status.showUntrackedFiles no; echo n > "$T/w_uf/new_work.txt"
run uf "$T/w_uf";                           eq "N-I1 untracked file with status.showUntrackedFiles=no at the root: exit 13" "$RC" 13
eq "N-I1 dirty_untracked counts it" "$(row uf . .dirty_untracked)" 1
grep -q 'showUntrackedFiles' "$T/uf.err" && ok "N-I1 the hidden-untracked configuration is noted on stderr" || bad "N-I1 no showUntrackedFiles note on stderr: $(head -c 200 "$T/uf.err")"
mk ufpar; mk ufsub ext
( cd "$T/w_ufpar" && $G submodule add -q "$T/ext/r_ufsub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
git -C "$T/w_ufpar/s" config status.showUntrackedFiles no; echo n > "$T/w_ufpar/s/new_work.txt"
run ufs "$T/w_ufpar";                       eq "N-I1 untracked file in a submodule with status.showUntrackedFiles=no: exit 13" "$RC" 13
eq "N-I1 the submodule row counts it" "$(row ufs s .dirty_untracked)" 1
mk ufd; mkdir "$T/w_ufd/dd"; echo 1 > "$T/w_ufd/dd/a"; echo 2 > "$T/w_ufd/dd/b"
run ufd "$T/w_ufd";                         eq "N-I1 a new untracked directory is dirt: exit 13" "$RC" 13
mk ufn; run ufn "$T/w_ufn";                 eq "N-I1 control: a clean repository without that configuration: exit 0" "$RC" 0
# ---- 34 N-I2: skip-worktree and assume-unchanged files that really differ are not clean ---------------------------------
mk sw; echo b >> "$T/w_sw/f"; git -C "$T/w_sw" update-index --skip-worktree f
run sw "$T/w_sw";                           eq "N-I2 a modified skip-worktree file: exit 13" "$RC" 13
eq "N-I2 it is counted as a tracked change" "$(row sw . '.dirty_tracked>=1')" true
mk au; echo b >> "$T/w_au/f"; git -C "$T/w_au" update-index --assume-unchanged f
run au "$T/w_au";                           eq "N-I2 a modified assume-unchanged file: exit 13" "$RC" 13
mk swc; git -C "$T/w_swc" update-index --skip-worktree f
run swc "$T/w_swc";                         eq "N-I2 control: an UNMODIFIED skip-worktree file: exit 0" "$RC" 0
mk sws; git -C "$T/w_sws" update-index --skip-worktree f; rm "$T/w_sws/f"
run sws "$T/w_sws";                         eq "N-I2 control: an absent skip-worktree file (sparse checkout) is not dirt: exit 0" "$RC" 0
mk aud; git -C "$T/w_aud" update-index --assume-unchanged f; rm "$T/w_aud/f"
run aud "$T/w_aud";                         eq "N-I2 a DELETED assume-unchanged file: exit 13" "$RC" 13
mk swx; echo b >> "$T/w_swx/f"; git -C "$T/w_swx" update-index --skip-worktree f; excrow "$T/w_swx" . f > "$T/swx.tsv"
EXC="$T/swx.tsv" run swx "$T/w_swx";        eq "N-I2 a flagged modified file is never excepted: exit 13" "$RC" 13
mk swsub ext; mk swpar; ( cd "$T/w_swpar" && $G submodule add -q "$T/ext/r_swsub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
echo b >> "$T/w_swpar/s/f"; git -C "$T/w_swpar/s" update-index --assume-unchanged f
run swsub "$T/w_swpar";                     eq "N-I2 a modified assume-unchanged file inside a submodule: exit 13" "$RC" 13
# ---- 35 N-I3: the detached-submodule class is the WORSE of {pin class, HEAD class}: every pin x HEAD pair -----------------
# pin kinds SAME/LB/RB/DV/UNK = the recorded gitlink equals the tip / is an older commit / is a local-only commit past the tip / is a
# local-only commit diverging from the tip / names an object the submodule does not hold; head kinds the same without UNK.
rk() { case "$1" in DIVERGED) echo 5;; REMOTE-BEHIND) echo 4;; UNKNOWN-DIFFERENT) echo 3;; LOCAL-BEHIND) echo 2;; *) echo 1;; esac; }
kcls() { case "$1" in SAME) echo SAME;; LB) echo LOCAL-BEHIND;; RB) echo REMOTE-BEHIND;; DV) echo DIVERGED;; UNK) echo UNKNOWN-DIFFERENT;; esac; }
pinhead() {  # pinhead <n> <pin kind> <head kind>: parent whose owned submodule os is detached at <head kind> with the gitlink at <pin kind>
  local n="ph$1" pk="$2" hk="$3" o="$T/w_ph$1par/os" sT sC sP sX pin hd
  mk "${n}os"; ( cd "$T/w_${n}os" && git checkout -q -b dev && echo d > d && git add d && $G commit -qm cd && git push -q origin dev 2>/dev/null && git checkout -q main )
  mk "${n}par"; ( cd "$T/w_${n}par" && $G submodule add -q -b dev "$T/fx/r_${n}os.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
  sT="$(git -C "$o" rev-parse HEAD)"; sC="$(git -C "$o" rev-parse "$sT~1")"
  git -C "$o" checkout -q --detach "$sT"; ( cd "$o" && echo p > p && git add p && $G commit -qm P ); sP="$(git -C "$o" rev-parse HEAD)"
  git -C "$o" checkout -q --detach "$sC"; ( cd "$o" && echo x > x && git add x && $G commit -qm X ); sX="$(git -C "$o" rev-parse HEAD)"
  case "$pk" in SAME) pin="$sT";; LB) pin="$sC";; RB) pin="$sP";; DV) pin="$sX";; UNK) pin=1111111111111111111111111111111111111111;; esac
  case "$hk" in SAME) hd="$sT";; LB) hd="$sC";; RB) hd="$sP";; DV) hd="$sX";; esac
  ( cd "$T/w_${n}par" && $G update-index --add --cacheinfo "160000,$pin,os" && $G commit -qm pin && git push -q origin main 2>/dev/null ); git -C "$o" checkout -q --detach "$hd"
}
for pk in SAME LB RB DV UNK; do for hk in SAME LB RB DV; do
  pinhead "${pk}_$hk" "$pk" "$hk"
  cp_="$(kcls "$pk")"; ch_="$(kcls "$hk")"; want="$cp_"; [ "$(rk "$ch_")" -gt "$(rk "$cp_")" ] && want="$ch_"
  case "$want" in DIVERGED) wrc=12;; REMOTE-BEHIND) wrc=11;; UNKNOWN-DIFFERENT) wrc=14;; *) wrc=0;; esac
  run "ph_${pk}_$hk" "$T/w_ph${pk}_${hk}par"
  eq "N-I3 pin $pk (=$cp_) x HEAD $hk (=$ch_): the submodule class is the worse one" "$(row "ph_${pk}_$hk" os '.remotes[0].class')" "$want"
  eq "N-I3 pin $pk x HEAD $hk: exit $wrc" "$RC" "$wrc"
done; done
# ---- 36 N-I4 / m-e: the needle's two remaining legs, each with its own non-blind shim ------------------------------------
cat > "$T/dropq_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `status --porcelain` keeps its exit status but DROPS every untracked (??) entry: blind to untracked work only
for a in "$@"; do if [ "$a" = --porcelain ]; then o="$(mktemp)"; git "$@" > "$o"; rc=$?; awk 'BEGIN{RS="\0";ORS="\0"} substr($0,1,2)!="??"' "$o"; rm -f "$o"; exit $rc; fi; done
exec git "$@"
SHIM
cat > "$T/alldirt_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `status --porcelain` is the real output PLUS one invented modified file, exit status kept: it still sees untracked files and
# still fails on a corrupt index, so only the clean-reads-clean leg of the needle can refuse it (a verifier that trusts it false-FAILs every repository)
for a in "$@"; do if [ "$a" = --porcelain ]; then git "$@"; rc=$?; printf ' M ghost\0'; exit $rc; fi; done
exec git "$@"
SHIM
chmod +x "$T/dropq_git.sh" "$T/alldirt_git.sh"
mk nd; echo untracked > "$T/w_nd/u.txt"
VERIFY_GIT="$T/dropq_git.sh" run ndq "$T/w_nd";      eq "N-I4 a git that hides ONLY untracked entries (exit status kept): the needle refuses, exit 20" "$RC" 20
grep -q 'control needle' "$T/ndq.err" && ok "N-I4 the refusal names the control needle" || bad "N-I4 no control-needle message: $(head -c 200 "$T/ndq.err")"
VERIFY_GIT="$T/alldirt_git.sh" run nda "$T/w_clean"; eq "m-e a git that reports dirt on a clean tree: the needle refuses, exit 20 (no false FAIL of every repository)" "$RC" 20
VERIFY_GIT="$T/dropq_git.sh" "$VR" --self-test >/dev/null 2>&1; rc=$?; ALLRC="$ALLRC $rc"; eq "N-I4 --self-test with the untracked-hiding git: exit 20" "$rc" 20
# ---- 37 minors: m-a pinned-gitlink exit, m-b exception scoping, m-c unclassifiable remote URL ----------------------------
r4mk rl yes no
FAILCMD=ls-tree VERIFY_GIT="$T/subcmd_fail_git.sh" run lsfail "$T/w_rlpar" --fetch; eq "m-a git ls-tree (the pinned gitlink) fails: exit 20, never a HEAD-only comparison" "$RC" 20
mk xpar; mk xsub ext
( cd "$T/w_xpar" && $G submodule add -q "$T/ext/r_xsub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null ); echo b >> "$T/w_xpar/s/f"
excrow "$T/w_xpar/s" . f > "$T/xrow.tsv"
EXC="$T/xrow.tsv" run xrow "$T/w_xpar";     eq "m-b an exception row for repository '.' does not except the same change in submodule s: exit 13" "$RC" 13
eq "m-b the submodule row is dirty and not excepted" "$(row xrow s '[.dirty,.excepted]|@csv')" "true,false"
excrow "$T/w_xpar/s" s f > "$T/xrow2.tsv"
EXC="$T/xrow2.tsv" run xrow2 "$T/w_xpar";   eq "m-b control: the row naming s excepts it: exit 0" "$RC" 0
mk lnk; ( cd "$T/w_lnk" && echo A > a.txt && echo B > b.txt && ln -s a.txt lnk && git add a.txt b.txt lnk && $G commit -qm links && git push -q origin main 2>/dev/null && ln -sfn b.txt lnk )
excrow "$T/w_lnk" . lnk > "$T/lnk.tsv"
EXC="$T/lnk.tsv" run lnk "$T/w_lnk";        eq "m-b a modified tracked SYMLINK is never excepted (hash of its target is not its blob): exit 13" "$RC" 13
mk ucl; ( cd "$T/w_ucl" && echo x > g && git add g && $G commit -qm c2 && git remote set-url origin 'somehost:repo.git' )
run ucl "$T/w_ucl";                         eq "m-c a remote URL the organisation parser cannot classify, with an unpushed commit: exit 14 (unproven, never 0)" "$RC" 14
eq "m-c reported in unproven" "$(row ucl . '.unproven|tojson')" '["NO-REMOTE-BRANCH"]'
grep -q 'cannot classify' "$T/ucl.err" && ok "m-c the unclassifiable URL is named on stderr" || bad "m-c no 'cannot classify' message: $(head -c 200 "$T/ucl.err")"
run ucl_nr "$T/w_ucl" --no-remote;          eq "m-c --no-remote contacts nothing: exit 0" "$RC" 0
# ---- 38 round 4: I-1 (--fetch touches objects only: no submodule recursion, no maintenance), I-2 (exact ref name), I-3 (core.fsmonitor),
#      I-4 (attached owned submodule pin reachability), m-1/m-2/m-4 mutation-adequacy legs, container portability (mawk, no `column`) ----
snaprefs() { git -C "$1" for-each-ref --format='%(refname) %(objectname)'; cat "$1/.git/packed-refs" 2>/dev/null; ls "$1"/.git/FETCH_HEAD "$1"/.git/modules/*/FETCH_HEAD 2>/dev/null; }
# I-1a: the superproject's remote bumped a submodule pin; a diverged superproject makes --fetch fetch: the submodule's refs stay untouched
git init -q --bare -b main "$T/fx/r_f1sub.git"; git init -q --bare -b main "$T/fx/r_f1par.git"
git clone -q "$T/fx/r_f1sub.git" "$T/w_f1sub" 2>/dev/null; ( cd "$T/w_f1sub" && echo a > a && git add a && $G commit -qm s1 && git push -q origin main 2>/dev/null )
git clone -q "$T/fx/r_f1par.git" "$T/w_f1par" 2>/dev/null
( cd "$T/w_f1par" && echo p > p && git add p && $G commit -qm p1 && $G submodule add -q "$T/fx/r_f1sub.git" s 2>/dev/null && $G commit -qm addsub && git push -q origin main 2>/dev/null )
$G clone -q --recurse-submodules "$T/fx/r_f1par.git" "$T/o_f1" 2>/dev/null
( cd "$T/o_f1/s" && git checkout -q main && echo b > b && git add b && $G commit -qm s2 && git push -q origin main 2>/dev/null && cd .. && git add s && $G commit -qm bump && git push -q origin main 2>/dev/null )
( cd "$T/w_f1par" && echo l > l && git add l && $G commit -qm local )
f1b="$(snaprefs "$T/w_f1par/s" | md5sum)"; f1p="$(snaprefs "$T/w_f1par" | md5sum)"
run f1 "$T/w_f1par" --fetch
eq "I-1 --fetch on a diverged superproject whose remote bumped a submodule pin: exit 12" "$RC" 12
eq "I-1 the submodule's refs and FETCH_HEAD are byte-identical after --fetch (no recursion into submodules)" "$(snaprefs "$T/w_f1par/s" | md5sum)" "$f1b"
eq "I-1 the superproject's own refs and FETCH_HEAD are unchanged too" "$(snaprefs "$T/w_f1par" | md5sum)" "$f1p"
# I-1b: --fetch never runs `git maintenance` / gc --auto (a repository hook would execute and a detached job could outlive the verifier)
mk f1g; other f1g h; ( cd "$T/w_f1g" && echo l > l && git add l && $G commit -qm local )
git -C "$T/w_f1g" config gc.auto 1; git -C "$T/w_f1g" config gc.autoPackLimit 1; git -C "$T/w_f1g" config gc.autoDetach false; git -C "$T/w_f1g" config maintenance.autoDetach false
printf '#!/bin/sh\ntouch "%s/PRE_AUTO_GC_RAN"\n' "$T" > "$T/w_f1g/.git/hooks/pre-auto-gc"; chmod +x "$T/w_f1g/.git/hooks/pre-auto-gc"
mkdir -p "$T/f1g_blobs"; for i in $(seq 1 1500); do echo "blob $i" > "$T/f1g_blobs/b$i"; done   # >256 loose objects so that gc.auto=1 is really tripped
( cd "$T/f1g_blobs" && printf "%s\n" "$PWD"/b* | git -C "$T/w_f1g" hash-object -w --stdin-paths >/dev/null )
rm -f "$T/PRE_AUTO_GC_RAN"; f1gp="$(ls "$T/w_f1g/.git/objects/pack" | wc -l)"
run f1g "$T/w_f1g" --fetch; sleep 1
eq "I-1 --fetch on a diverged repository with gc.auto tripped: exit 12" "$RC" 12
[ -e "$T/PRE_AUTO_GC_RAN" ] && bad "I-1 the repository's pre-auto-gc hook was executed by the verifier (--fetch ran maintenance)" || ok "I-1 no pre-auto-gc hook execution (no maintenance under --fetch)"
eq "I-1 no gc ran: the pack count is unchanged" "$(ls "$T/w_f1g/.git/objects/pack" | wc -l)" "$f1gp"
# I-2: a remote ref that merely ENDS in refs/heads/<branch> is not the branch tip
mk i2; ( cd "$T/w_i2" && echo b > b && git add b && $G commit -qm c2_unpushed && git push -q origin HEAD:refs/archive/refs/heads/main 2>/dev/null )
run i2 "$T/w_i2";                           eq "I-2 the unpushed commit exists only under refs/archive/refs/heads/main: exit 11 (not 0)" "$RC" 11
eq "I-2 remote_tip is the tip of refs/heads/main exactly" "$(row i2 . '.remotes[0].remote_tip')" "$(git -C "$T/fx/r_i2.git" rev-parse refs/heads/main)"
eq "I-2 class REMOTE-BEHIND" "$(row i2 . '.remotes[0].class')" REMOTE-BEHIND
mk i2c; ( cd "$T/w_i2c" && git push -q origin HEAD:refs/archive/refs/heads/main 2>/dev/null )
run i2c "$T/w_i2c";                         eq "I-2 control: a same-tip decoy ref under another namespace, branch itself SAME: exit 0" "$RC" 0
# I-2b: the default-branch path (detached owned submodule, no .gitmodules branch) selects the HEAD line exactly, never a tail match
r4mk i2s no no
orphan2="$(git -c user.email=t@t -c user.name=t -C "$T/fx/r_i2sos.git" commit-tree -m x 'main^{tree}' 2>/dev/null)"
git -C "$T/fx/r_i2sos.git" update-ref refs/archive/HEAD "$orphan2" 2>/dev/null
run i2s "$T/w_i2spar";                      eq "I-2b a decoy ref archive/HEAD does not change the default-branch comparison: exit 0" "$RC" 0
# I-3: a core.fsmonitor hook that claims "nothing changed" must not make a modified file read clean
mk fsm
cat > "$T/w_fsm/.git/fsm.sh" <<'FSH'
#!/bin/sh
printf 'tok1\0'
FSH
chmod +x "$T/w_fsm/.git/fsm.sh"; git -C "$T/w_fsm" config core.fsmonitor "$T/w_fsm/.git/fsm.sh"; git -C "$T/w_fsm" config core.fsmonitorHookVersion 2
git -C "$T/w_fsm" status --porcelain >/dev/null; git -C "$T/w_fsm" update-index --refresh >/dev/null 2>&1; git -C "$T/w_fsm" status >/dev/null
sleep 1.1; echo CHANGED >> "$T/w_fsm/f"
run fsm "$T/w_fsm"
eq "I-3 a modified file under a lying core.fsmonitor: exit 13" "$RC" 13
eq "I-3 the row is dirty with a tracked change" "$(row fsm . '[.dirty,(.dirty_tracked>=1)]|@csv')" "true,true"
eq "I-3 control needle: plain git status under the same hook IS blind (the fixture lies; a non-lying fixture would prove nothing)" "$(git -C "$T/w_fsm" status --porcelain | wc -l)" 0
eq "I-3 control: with the monitor disabled git sees the change" "$(git -C "$T/w_fsm" -c core.fsmonitor=false status --porcelain | wc -l)" 1
# I-4: the recorded pin of an ATTACHED owned submodule must be reachable from a remote tip (a fresh clone must be able to check it out)
mk pasub; mk papar
( cd "$T/w_papar" && $G submodule add -q "$T/fx/r_pasub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_papar/os" && git checkout -q main 2>/dev/null; echo n > n && git add n && $G commit -qm local_only_pin )
( cd "$T/w_papar" && git add os && $G commit -qm pin_local_only && git push -q origin main 2>/dev/null )
( cd "$T/w_papar/os" && git reset -q --hard origin/main )
eq "I-4 fixture: the submodule is ATTACHED on main" "$(git -C "$T/w_papar/os" symbolic-ref --short -q HEAD)" main
run pa "$T/w_papar";                        eq "I-4 attached owned submodule, pin = a local-only commit no remote holds, parent pushed: exit 11 (not 0)" "$RC" 11
eq "I-4 the submodule row is REMOTE-BEHIND (the pin is ahead of the remote tip)" "$(row pa os '.remotes[0].class')" REMOTE-BEHIND
rm -rf "$T/fresh_pa"; git clone -q "$T/fx/r_papar.git" "$T/fresh_pa" 2>/dev/null
( cd "$T/fresh_pa" && git -c protocol.file.allow=always submodule update --init >/dev/null 2>"$T/fresh_pa.err" ); rc=$?
grep -q 'not our ref' "$T/fresh_pa.err" && ok "I-4 oracle: a fresh clone of the parent really fails with 'not our ref' (the condition the verifier must report)" || bad "I-4 oracle: a fresh clone did not fail with 'not our ref': $(head -c 200 "$T/fresh_pa.err")"
run pas "$T/w_papar" --strict;              eq "I-4 same fixture under --strict: exit 11 (ahead precedes pin drift)" "$RC" 11
mk pbsub; mk pbpar
( cd "$T/w_pbpar" && $G submodule add -q "$T/fx/r_pbsub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_pbpar/os" && git checkout -q main 2>/dev/null; echo n > n && git add n && $G commit -qm pushed_pin && git push -q origin main 2>/dev/null )
( cd "$T/w_pbpar" && git add os && $G commit -qm bump_to_pushed && git push -q origin main 2>/dev/null )
run pb "$T/w_pbpar";                        eq "I-4 control: an attached owned submodule whose pin is a PUSHED commit: exit 0" "$RC" 0
mk pcsub; mk pcpar
( cd "$T/w_pcpar" && $G submodule add -q "$T/fx/r_pcsub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_pcpar/os" && git checkout -q main 2>/dev/null; echo n > n && git add n && $G commit -qm pushed2 && git push -q origin main 2>/dev/null )
run pc "$T/w_pcpar";                        eq "I-4 control: pin BEHIND the pushed tip (parent not bumped yet) is reachable, not a failure: exit 0" "$RC" 0
eq "I-4 control: the pin-behind row's class is SAME (a pin merely behind the pushed tip is not a class)" "$(row pc os '.remotes[0].class')" SAME
run pcs "$T/w_pcpar" --strict;              eq "I-4 control: the same under --strict is pin drift only (15), never an unreachable-pin class" "$RC" 15
# m-1: the symbolic-ref exit status is checked (a failing symbolic-ref is not "detached")
mk sy1; FAILCMD=symbolic-ref VERIFY_GIT="$T/subcmd_fail_git.sh" run sy1 "$T/w_sy1"; eq "m-1 git symbolic-ref fails (rc 128): exit 20, never read as detached" "$RC" 20
# m-2: a modified skip-worktree SYMLINK is dirt
mk sl; ( cd "$T/w_sl" && echo A > a.txt && echo B > b.txt && ln -s a.txt lnk && git add a.txt b.txt lnk && $G commit -qm links && git push -q origin main 2>/dev/null && git update-index --skip-worktree lnk && ln -sfn b.txt lnk )
run sl "$T/w_sl";                           eq "m-2 a retargeted skip-worktree symlink: exit 13" "$RC" 13
# m-4: the flagged-file blob lookup is literal (a name with glob metacharacters is not a pathspec pattern)
mk gl; ( cd "$T/w_gl" && echo one > x1 && echo two > 'x[1]' && git add x1 'x[1]' && $G commit -qm globs && git push -q origin main 2>/dev/null && git update-index --skip-worktree 'x[1]' && echo one > 'x[1]' )
run gl "$T/w_gl";                           eq "m-4 skip-worktree file 'x[1]' rewritten to the content of sibling x1 is dirt (literal lookup): exit 13" "$RC" 13
# portability (container root causes RC1, RC2, RC5): the verifier must not depend on gawk features or on `column`
if command -v mawk >/dev/null 2>&1; then
  mkdir -p "$T/shim_mawk"; ln -sf "$(command -v mawk)" "$T/shim_mawk/awk"
  mk mw; ( cd "$T/w_mw" && echo b >> f )
  excrow "$T/w_mw" . f > "$T/mw.tsv"
  PATH="$T/shim_mawk:$PATH" EXC="$T/mw.tsv" run mw "$T/w_mw"
  eq "portability (mawk as awk): a fully excepted dirty file: exit 0 ({64} interval and NUL printf must work under mawk)" "$RC" 0
  eq "portability (mawk as awk): the root row path is '.', not empty" "$(jq_ mw '.repos|map(.path)|join(",")')" "."
  r4mk mwr yes no
  PATH="$T/shim_mawk:$PATH" run mwr "$T/w_mwrpar" --fetch
  eq "portability (mawk as awk): a detached owned submodule is enumerated and compared: exit 0" "$RC" 0
  eq "portability (mawk as awk): rows are . and os" "$(jq_ mwr '.repos|map(.path)|join(",")')" ".,os"
else ok "portability (mawk): SKIP-with-reason: mawk is not installed on this host (the container run covers it)"; fi
mkdir -p "$T/shim_nocol"; printf '#!/bin/sh\ntouch "%s/COLUMN_RAN"\nexit 99\n' "$T" > "$T/shim_nocol/column"; chmod +x "$T/shim_nocol/column"
rm -f "$T/COLUMN_RAN"
PATH="$T/shim_nocol:$PATH" "$VR" --root "$T/w_clean" --owned-orgs fx --exceptions /dev/null --jobs 2 --timeout 20 >"$T/nocol.out" 2>"$T/nocol.err"; rc=$?; ALLRC="$ALLRC $rc"
eq "portability (no column): the table run on a clean tree: exit 0" "$rc" 0
[ -e "$T/COLUMN_RAN" ] && bad "portability: the verifier invoked the external column command" || ok "portability: the verifier does not depend on the external column command"
grep -q '^PATH' "$T/nocol.out" && grep -q '^\.' "$T/nocol.out" && ok "portability: the table still prints its header and the root row" || bad "portability: no table in the output: $(head -c 200 "$T/nocol.out")"
# m-6: --help prints the header documentation only (no source code), exit 0
"$VR" --help >"$T/help.out" 2>&1; rc=$?; ALLRC="$ALLRC $rc"
eq "m-6 --help: exit 0" "$rc" 0
grep -q '^Usage ' "$T/help.out" && ok "m-6 --help prints the Usage paragraph" || bad "m-6 --help has no Usage paragraph"
{ grep -q 'set -u' "$T/help.out" || grep -q 'export GIT_OPTIONAL_LOCKS' "$T/help.out"; } && bad "m-6 --help prints source code after the header" || ok "m-6 --help stops at the end of the header"
# a `git submodule status` line the parser cannot read is exit 20 (never a silently skipped submodule)
cat > "$T/ssgarbage_git.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: `git submodule ...` prints a line that is not a status line; everything else is real git
for a in "$@"; do if [ "$a" = submodule ]; then echo "!!! not a status line"; exit 0; fi; done
exec git "$@"
SHIM
chmod +x "$T/ssgarbage_git.sh"
VERIFY_GIT="$T/ssgarbage_git.sh" run ssg "$T/w_clean"; eq "an unparseable git submodule status line: exit 20" "$RC" 20
grep -q 'cannot parse a git submodule status line' "$T/ssg.err" && ok "the unparseable line is named on stderr" || bad "no 'cannot parse' message: $(head -c 200 "$T/ssg.err")"
# ---- 39 round 5: R1 (no repository hook under --fetch, in any mode), R2 (a failing jq count is exit 20, never fail-open),
#      R3 (a pin held by ANY remote branch/tag tip is reachable; a pin no remote ref holds is still reported), M5-1 ------------------
# R1: git runs the reference-transaction hook for a fetch's empty transaction; the verifier must run NO repository hook
mk q5r1; other q5r1 h; ( cd "$T/w_q5r1" && echo l > l && git add l && $G commit -qm local )
printf '#!/bin/sh\necho "$1" >> "%s/REFTX_RAN"\ncat >/dev/null\n' "$T" > "$T/w_q5r1/.git/hooks/reference-transaction"; chmod +x "$T/w_q5r1/.git/hooks/reference-transaction"
for hk in post-checkout post-merge post-rewrite pre-commit; do printf '#!/bin/sh\ntouch "%s/HOOK_%s"\n' "$T" "$hk" > "$T/w_q5r1/.git/hooks/$hk"; chmod +x "$T/w_q5r1/.git/hooks/$hk"; done
# control: the hook fixture is live (a plain git fetch with the verifier's own flags DOES run it), else the absence below proves nothing
mk q5r1c; other q5r1c h; printf '#!/bin/sh\necho "$1" >> "%s/REFTX_CONTROL"\ncat >/dev/null\n' "$T" > "$T/w_q5r1c/.git/hooks/reference-transaction"; chmod +x "$T/w_q5r1c/.git/hooks/reference-transaction"
rm -f "$T/REFTX_CONTROL"; git -C "$T/w_q5r1c" fetch --no-tags --no-write-fetch-head --refmap= --quiet origin refs/heads/main >/dev/null 2>&1
# git runs the reference-transaction hook for the EMPTY transaction of a ref-less fetch only from some version on (host git 2.53: yes; the
# container's git 2.39.5: no). Where it does not, the R1 condition cannot occur, the control says so, and the R1 hook-absence lines below are
# labelled VACUOUS there instead of passing silently as if they had proven something (11.4.201).
R1NOTE=""
if [ -s "$T/REFTX_CONTROL" ]; then ok "R1 control: a plain fetch in a repository with a reference-transaction hook runs the hook (the fixture is live)"
else R1NOTE=" [VACUOUS on this git $(git --version | cut -d' ' -f3): the control hook did not fire]"; ok "R1 control SKIP-with-reason: git $(git --version | cut -d' ' -f3) runs no reference-transaction hook for a ref-less fetch, so the R1 condition cannot occur here (the host run on git >= 2.53 is the live proof)"; fi
rm -f "$T/REFTX_RAN" "$T"/HOOK_*
run q5r1p "$T/w_q5r1";                          eq "R1 plain mode on a diverged repository with repository hooks installed: exit 14 (remote tip unknown locally)" "$RC" 14
[ -e "$T/REFTX_RAN" ] && bad "R1 a hook ran in plain mode" || ok "R1 no hook execution in plain mode$R1NOTE"
run q5r1 "$T/w_q5r1" --fetch;                   eq "R1 --fetch on the same repository: exit 12" "$RC" 12
[ -e "$T/REFTX_RAN" ] && bad "R1 the reference-transaction hook ran under --fetch: $(tr '\n' ' ' < "$T/REFTX_RAN")" || ok "R1 no reference-transaction hook execution under --fetch$R1NOTE"
ls "$T"/HOOK_* >/dev/null 2>&1 && bad "R1 another repository hook ran: $(ls "$T"/HOOK_* | tr '\n' ' ')" || ok "R1 no other repository hook ran"
# R2: an exit-map count that cannot be read is exit 20 (the header promises it), never a fall-through to a lower code
REALJQ="$(command -v jq)"; mkdir -p "$T/shim_jq"
cat > "$T/shim_jq/jq" <<SHIM
#!/usr/bin/env bash
# jq shim: ONLY the exit-map count filter named in \$JQFAIL fails (rc 5, or prints a non-number with rc 0 when \$JQFAIL_MODE=junk); every other call is real jq
for a in "\$@"; do
  case "\$a" in
    "\$JQFAIL") if [ "\${JQFAIL_MODE:-rc}" = junk ]; then echo null; exit 0; else echo "jq: simulated failure" >&2; exit 5; fi ;;
  esac
done
exec "$REALJQ" "\$@"
SHIM
chmod +x "$T/shim_jq/jq"
mk q5r2n; git -C "$T/w_q5r2n" remote remove origin
mk q5r2d; echo b >> "$T/w_q5r2d/f"
PATH="$T/shim_jq:$PATH" JQFAIL='.summary.unproven' run q5r2n_ctl "$T/w_q5r2n"
eq "R2 control: a repository with no remote (real jq path, shim present but not triggered by another filter) is exit 14" "$("$VR" --root "$T/w_q5r2n" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/q5r2n_c.json" >/dev/null 2>&1; echo $?)" 14
eq "R2 control: a dirty repository without the shim is exit 13" "$("$VR" --root "$T/w_q5r2d" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json "$T/q5r2d_c.json" >/dev/null 2>&1; echo $?)" 13
eq "R2 the unproven count jq fails (rc 5) on a no-remote repository: exit 20, not 0" "$RC" 20
grep -q 'verify_repos:' "$T/q5r2n_ctl.err" && ok "R2 the failure is named on stderr" || bad "R2 no message on stderr: $(head -c 200 "$T/q5r2n_ctl.err")"
PATH="$T/shim_jq:$PATH" JQFAIL='[.repos[]|select(.problems|index("dirty"))]|length' run q5r2d_fail "$T/w_q5r2d"
eq "R2 the dirty count jq fails (rc 5) on a dirty repository: exit 20, not 14 or 0" "$RC" 20
PATH="$T/shim_jq:$PATH" JQFAIL='[.repos[]|select(.problems|index("dirty"))]|length' JQFAIL_MODE=junk run q5r2d_junk "$T/w_q5r2d"
eq "R2 the dirty count jq answers a non-number with rc 0: exit 20 (an answer that is not a count is an error)" "$RC" 20
PATH="$T/shim_jq:$PATH" JQFAIL='[.repos[]|select(.problems|index("ahead"))]|length' run q5r2a_fail "$T/w_ahead"
eq "R2 the ahead count jq fails (rc 5) on an unpushed repository: exit 20, not 0" "$RC" 20
# R3: a pin that a remote holds on ANOTHER branch (or tag) is reachable: a fresh clone can check it out; it is pin drift at most, never DIVERGED
r3mk() {  # r3mk <name>: main has c1; branch release = c1 + R (pushed); main advances with M (pushed); the submodule is ATTACHED on main at M; the parent pins R (pushed)
  mk "$1""sub"; mk "$1""par"
  ( cd "$T/w_$1par" && $G submodule add -q "$T/fx/r_$1sub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main 2>/dev/null && git checkout -q -b release && echo r > r && git add r && $G commit -qm R && git push -q origin release 2>/dev/null \
      && git checkout -q main && echo m > m && git add m && $G commit -qm M && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q release && cd .. && git add os && $G commit -qm pinR && git push -q origin main 2>/dev/null )
  ( cd "$T/w_$1par/os" && git checkout -q main )
}
r3mk q5a
eq "R3 fixture: the submodule is ATTACHED on main" "$(git -C "$T/w_q5apar/os" symbolic-ref --short -q HEAD)" main
eq "R3 fixture: the pin is the tip of the remote branch release (not of main)" "$(git -C "$T/w_q5apar" ls-tree HEAD os | awk '{print $3}')" "$(git -C "$T/fx/r_q5asub.git" rev-parse refs/heads/release)"
rm -rf "$T/fresh_q5a"; git clone -q "$T/fx/r_q5apar.git" "$T/fresh_q5a" 2>/dev/null
( cd "$T/fresh_q5a" && git -c protocol.file.allow=always submodule update --init >/dev/null 2>"$T/fresh_q5a.err" ); rcra=$?
eq "R3 oracle: a fresh clone of the parent checks the submodule pin out (rc)" "$rcra" 0
run q5a "$T/w_q5apar";                        eq "R3 a pin held as the tip of another remote branch: exit 0 (was 12 DIVERGED)" "$RC" 0
eq "R3 the submodule row class is SAME (HEAD vs main tip)" "$(row q5a os '.remotes[0].class')" SAME
run q5as "$T/w_q5apar" --strict;              eq "R3 the same under --strict is pin drift only (15)" "$RC" 15
run q5af "$T/w_q5apar" --fetch;               eq "R3 the same under --fetch: exit 0" "$RC" 0
# the pin is an ANCESTOR of a remote branch tip (held through ancestry, not tip equality)
r3mk q5b
( cd "$T/w_q5bpar/os" && git checkout -q release && echo r2 > r2 && git add r2 && $G commit -qm R2 && git push -q origin release 2>/dev/null && git checkout -q main )
run q5b "$T/w_q5bpar";                        eq "R3 a pin that is an ancestor of another remote branch tip: exit 0" "$RC" 0
# a TAG tip holds the pin
r3mk q5c
( cd "$T/w_q5cpar/os" && git tag held "$(git -C "$T/w_q5cpar" ls-tree HEAD os | awk '{print $3}')" && git push -q origin held 2>/dev/null && git push -q origin :release 2>/dev/null )
eq "R3 fixture: no remote branch holds the pin any more (release deleted), only the tag" "$(git -C "$T/fx/r_q5csub.git" branch --contains "$(git -C "$T/w_q5cpar" ls-tree HEAD os | awk '{print $3}')" | wc -l)" 0
run q5c "$T/w_q5cpar";                        eq "R3 a pin held by a remote tag only: exit 0" "$RC" 0
# M5-1: a pin DIVERGED from the pushed tip and held by NO remote ref is still reported (the not our ref condition)
mk q5dsub; mk q5dpar
( cd "$T/w_q5dpar" && $G submodule add -q "$T/fx/r_q5dsub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
( cd "$T/w_q5dpar/os" && git checkout -q main 2>/dev/null && echo p > p && git add p && $G commit -qm local_pin )
( cd "$T/w_q5dpar" && git add os && $G commit -qm pin_local && git push -q origin main 2>/dev/null )
( cd "$T/w_q5dpar/os" && git reset -q --hard HEAD~1 && echo q > q && git add q && $G commit -qm pushed_other && git push -q origin main 2>/dev/null )
run q5d "$T/w_q5dpar";                        eq "M5-1 attached owned submodule, pin diverged from the pushed tip and held by no remote ref: exit 12" "$RC" 12
eq "M5-1 the submodule row is DIVERGED" "$(row q5d os '.remotes[0].class')" DIVERGED
rm -rf "$T/fresh_q5d"; git clone -q "$T/fx/r_q5dpar.git" "$T/fresh_q5d" 2>/dev/null
( cd "$T/fresh_q5d" && git -c protocol.file.allow=always submodule update --init >/dev/null 2>"$T/fresh_q5d.err" )
grep -q 'not our ref' "$T/fresh_q5d.err" && ok "M5-1 oracle: a fresh clone really fails with 'not our ref'" || bad "M5-1 oracle: no 'not our ref': $(head -c 160 "$T/fresh_q5d.err")"
# M5-1b: the pin object is absent from the submodule's own object store and no remote tip equals it: UNKNOWN-DIFFERENT (exit 14), never held
mk q5esub; mk q5epar
( cd "$T/w_q5epar" && $G submodule add -q "$T/fx/r_q5esub.git" os 2>/dev/null && $G commit -qm addos && git push -q origin main 2>/dev/null )
rm -rf "$T/o_q5e"; $G clone -q --recurse-submodules "$T/fx/r_q5epar.git" "$T/o_q5e" 2>/dev/null
( cd "$T/o_q5e/os" && git checkout -q main && echo u > u && git add u && $G commit -qm unpushed && cd .. && git add os && $G commit -qm pin_unpushed && git push -q origin main 2>/dev/null )
( cd "$T/w_q5epar" && git pull -q --no-recurse-submodules --ff-only origin main 2>/dev/null )
eq "M5-1b fixture: the pin object is unknown to the submodule" "$(git -C "$T/w_q5epar/os" cat-file -e "$(git -C "$T/w_q5epar" ls-tree HEAD os | awk '{print $3}')^{commit}" 2>/dev/null && echo present || echo absent)" absent
run q5e "$T/w_q5epar";                      eq "M5-1b a pin unknown locally and held by no remote tip: exit 14 (UNKNOWN-DIFFERENT)" "$RC" 14
eq "M5-1b the submodule row is UNKNOWN-DIFFERENT" "$(row q5e os '.remotes[0].class')" UNKNOWN-DIFFERENT
# ---- 17 exit map is exactly the documented set ---------------------------------------------------------------------
outside=""
# shellcheck disable=SC2194  # the case word is a deliberate constant list of the allowed codes, matched against each seen code
for c in $ALLRC; do case " 0 11 12 13 14 15 20 " in *" $c "*) ;; *) outside="$outside $c" ;; esac; done
eq "every exit code seen in this matrix is a member of {0,11,12,13,14,15,20}" "${outside:-none}" none
echo "SUMMARY pass=$PASSN fail=$FAILN"
[ "$FAILN" -eq 0 ]
