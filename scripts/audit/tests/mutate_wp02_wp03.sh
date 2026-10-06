#!/usr/bin/env bash
# Mutation runner for the WP-02 / WP-03 review fixes (WF-REVIEW-wp02-wp03, WF2-REVIEW round 3): kills the reviewer mutants RM1-RM11, RV1-RV6, SV1-SV9, DM1-DM4, SM1-SM4, round-3 NB/NI/MC/Ma
# and the author's own mutants of the fixes (B1-B3, I1, I5, I9, m1-m4, MX1-MX5: injection and toplevel guards).
#
# Purpose   Copy each script under test into a scratch tree, apply ONE textual mutation (the replaced text must exist exactly
#           once, else the mutant is reported BROKEN, never silently equal), run the matching test with the mutant and require a
#           FAIL (exit != 0). A mutant that passes its suite SURVIVED = a gap in the tests.
# Usage     [ONLY=<id regex>] [DRYRUN=1: check only that every verify pattern still applies] bash scripts/audit/tests/mutate_wp02_wp03.sh [verify|derive|scope|all]   (default all; one line per mutant)
# Exit      0 when every mutant is CAUGHT; 1 when any SURVIVED or is BROKEN.
# Side effects  temp directories under $TMPDIR, removed on exit. Needs git, jq, python3. Takes several minutes (verify suite x N).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
WHICH="${1:-all}"
M="$(mktemp -d "${TMPDIR:-/tmp}/mut.XXXXXX")"; trap 'rm -rf "${M:?}"' EXIT
CAUGHT=0; SURV=0; BROKEN=0
skip() { [ -n "${ONLY:-}" ] && [[ ! "$1" =~ $ONLY ]]; }   # ONLY=<regex of mutant ids> runs a subset (parallel chunks)
mutate() {  # mutate <src> <dst> <old> <new>  (python: old must occur exactly once)
  python3 - "$1" "$2" "$3" "$4" <<'PY'
import sys
src, dst, old, new = sys.argv[1:5]
s = open(src).read()
if s.count(old) != 1:
    print("BROKEN-MUTANT: pattern occurs %d times: %r" % (s.count(old), old[:70])); sys.exit(7)
open(dst, "w").write(s.replace(old, new))
PY
}
verdict() {  # verdict <id> <rc-of-mutation> <rc-of-suite> <desc>
  if [ "$2" -ne 0 ]; then BROKEN=$((BROKEN+1)); echo "BROKEN   $1 $4"
  elif [ "$3" -ne 0 ]; then CAUGHT=$((CAUGHT+1)); echo "CAUGHT   $1 $4"
  else SURV=$((SURV+1)); echo "SURVIVED $1 $4"; fi
}
vtree() {  # a scratch tree with the verifier layout: $M/vN/repo/verify_repos.sh, $M/vN/audit/{org_of.py,own_orgs*.txt}
  mkdir -p "$M/$1/repo" "$M/$1/audit"; cp scripts/audit/org_of.py scripts/audit/own_orgs*.txt "$M/$1/audit/"; cp scripts/repo/exceptions.tsv "$M/$1/repo/"
}
vmut() {  # vmut <id> <old> <new> <desc>
  skip "$1" && return 0
  vtree "$1"; mutate scripts/repo/verify_repos.sh "$M/$1/repo/verify_repos.sh" "$2" "$3"; local r=$?; chmod +x "$M/$1/repo/verify_repos.sh"
  local s=0
  if [ -n "${DRYRUN:-}" ]; then s=1   # DRYRUN=1: only prove every pattern still applies exactly once (BROKEN otherwise); the suite is not run
  else [ "$r" -eq 0 ] && { VR="$M/$1/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos.sh >"$M/$1.out" 2>&1; s=$?; }; fi
  verdict "$1" "$r" "$s" "$4"
}
if [ "$WHICH" = all ] || [ "$WHICH" = verify ]; then
vmut RM1  'if [ -n "$sup" ] && [ -z "$branch" ]; then   # detached owned' 'if false; then   # detached owned' "detached R4 block disabled"
vmut RM2  '[ "$n_dirty" -gt 0 ] && exit 13' '[ "$n_dirty" -gt 0 ] && exit 13
[ "$n_unp" -gt 0 ] && exit 14' "14 checked before 12, 11, 15"
vmut RM2b '[ "$n_div" -gt 0 ] && exit 12
[ "$n_ahead" -gt 0 ] && exit 11' '[ "$n_ahead" -gt 0 ] && exit 11
[ "$n_div" -gt 0 ] && exit 12' "11 checked before 12"
vmut RM2c '[ "$n_pin" -gt 0 ] && exit 15
[ "$n_beh" -gt 0 ] && exit 12' '[ "$n_beh" -gt 0 ] && exit 12
[ "$n_pin" -gt 0 ] && exit 15' "strict behind (12) checked before 15"
vmut RM2d '[ "$n_ahead" -gt 0 ] && exit 11
[ "$n_pin" -gt 0 ] && exit 15' '[ "$n_pin" -gt 0 ] && exit 15
[ "$n_ahead" -gt 0 ] && exit 11' "15 checked before 11"
vmut RM3  '$2=="dirty" && $3==f' '$3==f' "exception ignores kind"
vmut RM4  ' && length($6)>0 {print' ' {print' "exception accepts an empty reason"
vmut RM5  'dirty=false; [ $((tracked + untracked + stash)) -gt 0 ] && dirty=true' 'dirty=false; [ $((tracked + untracked)) -gt 0 ] && dirty=true' "stash not counted as dirt"
vmut RM6  '--refmap= --quiet' '--quiet' "--refmap= dropped (tracking refs move)"
vmut RM7  'needle() {' 'needle() { return 0' "control needle always passes"
vmut RM8  '.=="UNKNOWN-DIFFERENT" or .=="NO-REMOTE-BRANCH") ] + (if' '.=="UNKNOWN-DIFFERENT") ] + (if' "NO-REMOTE-BRANCH not unproven (class)"
vmut RM9  '($r.pin == "+" or $r.pin == "U" or $r.pin == "-")' '($r.pin == "+" or $r.pin == "U")' "uninitialised not counted as pin drift"
vmut RM10 '--argjson excepted "$excepted" --argjson remotes' '--argjson excepted true --argjson remotes' "excepted:true on every row"
vmut RM11 '[ -n "$pinned" ] && cmp="$pinned"' ':' "pinned gitlink ignored"
vmut MB1a '|| { fail "git status failed: $(head -c 160 "${outdir:?}/${idx:?}.status.err" | tr '"'"'\n'"'"' '"'"' '"'"')"; return 1; }' '|| true' "B1 git status exit not checked"
vmut MB1b 'head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)" || { fail "git rev-parse HEAD failed"; return 1; }
  [ -n "$head" ] || { fail "empty HEAD"; return 1; }' 'head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)"' "B1 rev-parse HEAD exit and emptiness not checked"
vmut MB1c 'stash="$($GIT -C "$abs" stash list 2>/dev/null)" || { fail "git stash list failed"; return 1; }' 'stash="$($GIT -C "$abs" stash list 2>/dev/null)"' "B1 stash list exit not checked"
vmut MB1e '[ "$o1" = 0 ] && [ "$o2" = 1 ] && [ "$r3" != 0 ]' '[ "$o1" = 0 ] && [ "$o2" = 1 ]' "needle does not probe the error path"
vmut MB2a '|| die20 "cannot enumerate the submodules:' '|| : # cannot enumerate the submodules:' "B2 enumeration exit ignored"
vmut MB2b '"$n" "$lp" "$lpin"; done < "$LIST"' '1 "$lp" "$lpin"; done < "$LIST"' "B2 worker output name collides (every record numbered 1)"
vmut MB3a '[ "$r_wt" = "$wth" ] && [ "$r_bl" = "$bth" ]' '[ "$r_bl" = "$bth" ]' "B3 working-tree hash not compared"
vmut MB3b '[ "$r_wt" = "$wth" ] && [ "$r_bl" = "$bth" ]' '[ "$r_wt" = "$wth" ]' "B3 blob hash not compared"
vmut MB3c '[ "$dirty" = true ] && [ "$stash" = 0 ] && [ "$untracked" = 0 ] && [ -f "$EXC_FILE" ]' '[ "$dirty" = true ] && [ "$untracked" = 0 ] && [ -f "$EXC_FILE" ]' "B3 stash allowed in an excepted repository"
vmut MB3d '[ "$dirty" = true ] && [ "$stash" = 0 ] && [ "$untracked" = 0 ] && [ -f "$EXC_FILE" ]' '[ "$dirty" = true ] && [ "$stash" = 0 ] && [ -f "$EXC_FILE" ]' "B3 untracked file allowed in an excepted repository"
vmut MB3e 'if [ "$xy" = " M" ] && [ -f' 'if [ -f' "B3 staged change accepted as excepted"
vmut MI1a "tr 'A-Z' 'a-z' | tr -d ' ')" "tr -d ' ')" "I1 owned list not lower-cased"
vmut MI5  'if [ "$cmp" != "$head" ]; then   # detached: HEAD' 'if false; then   # detached: HEAD' "I5 detached HEAD past the pin not compared"
vmut MI5b '[ "$(rank "$cls_h")" -gt "$(rank "$cls")" ] && cls="$cls_h"' ':' "I5 worse class not taken"
vmut MI9  'export GIT_OPTIONAL_LOCKS=0' ':' "I9 index may be refreshed (rewritten)"
vmut Mm1  'if [ "$pin" = "-" ]; then   # uninitialised' 'if false; then   # uninitialised' "m1 uninitialised row inherits the parent"
vmut Mm2  '&& noremote_unproven=true   # no remote at all' '&& noremote_unproven=false   # no remote at all' "m2 no remote at all read as clean"
vmut Mm4a '--self-test) SELFTEST=1; shift ;;' '--self-test) shift ;;' "m4 --self-test not honoured after other args"
vmut Mm4b 'posint() { case "$2" in '"''"'|*[!0-9]*|0|0[0-9]*) die20 "$1 needs a positive integer, got '"'"'$2'"'"'" ;; esac; }' 'posint() { :; }' "m4 --timeout/--jobs not validated"
vmut Mdet 'case "$a" in 0) echo LOCAL-BEHIND; return 0 ;; 1) echo DIVERGED; return 0 ;; *) return 1 ;; esac' 'case "$a" in 0) echo LOCAL-BEHIND; return 0 ;; 1) echo DIVERGED; return 0 ;; *) echo DIVERGED; return 0 ;; esac' "ancestry error read as DIVERGED"
vmut MX1 'valid_branch() { [ -n "$1" ]' 'valid_branch() { return 0; } ; valid_branch_old() { [ -n "$1" ]' "injection: branch validation removed"
vmut MX3  '&& ! valid_remote "$r"; then why=' '&& false; then why=' "injection: remote-name validation removed"
vmut MX3b 'for r in "${rlist[@]}"; do' 'for r in $(cat "${outdir:?}/${idx:?}.remotes"); do' "remote names word-split again (a name with whitespace becomes two)"
vmut MX4  'if [ -n "$rbranch" ] && ! valid_branch "$rbranch"; then why=' 'if [ -n "$rbranch" ] && false; then why=' "injection: remote-announced default branch validation removed"
vmut MX5  '|| { fail "git resolves this path to another repository ($top)"; return 1; }' '|| :' "--show-toplevel mismatch guard removed"
skip MX2 || {
# MX2: two edits (validation off + the old fetch form): applied by chaining a mutate on the intermediate file
vtree MX2; mutate scripts/repo/verify_repos.sh "$M/MX2/step1.sh" 'valid_branch() { [ -n "$1" ]' 'valid_branch() { return 0; } ; valid_branch_old() { [ -n "$1" ]'; r1=$?
mutate "$M/MX2/step1.sh" "$M/MX2/repo/verify_repos.sh" '--refmap= --quiet -- "$r" "refs/heads/$rbranch"' '--refmap= --quiet "$r" "$rbranch"'; r2=$?; chmod +x "$M/MX2/repo/verify_repos.sh"
s=0; [ -n "${DRYRUN:-}" ] && s=1; [ "$r1$r2" = 00 ] && [ -z "${DRYRUN:-}" ] && { VR="$M/MX2/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos.sh >"$M/MX2.out" 2>&1; s=$?; }
verdict MX2 "$((r1+r2))" "$s" "injection: validation removed AND the old fetch form (no --, raw branch): the original exploit"
}
# ---- round 3 (WF2-REVIEW): the reviewer mutants RV1-RV6 and the mutants of the round-3 fixes -----------------------------------
vmut RV1  'REMOTE-BEHIND) echo 4;; UNKNOWN-DIFFERENT) echo 3;; LOCAL-BEHIND) echo 2;;' 'REMOTE-BEHIND) echo 2;; UNKNOWN-DIFFERENT) echo 3;; LOCAL-BEHIND) echo 4;;' "RV1 rank: LOCAL-BEHIND above REMOTE-BEHIND"
vmut RV1b 'UNKNOWN-DIFFERENT) echo 3;; LOCAL-BEHIND) echo 2;;' 'UNKNOWN-DIFFERENT) echo 2;; LOCAL-BEHIND) echo 3;;' "RV1b rank: LOCAL-BEHIND above UNKNOWN-DIFFERENT"
vmut RV1c 'DIVERGED) echo 5;; REMOTE-BEHIND) echo 4;;' 'DIVERGED) echo 4;; REMOTE-BEHIND) echo 5;;' "RV1c rank: REMOTE-BEHIND above DIVERGED"
vmut RV1d 'LOCAL-BEHIND) echo 2;; *) echo 1;;' 'LOCAL-BEHIND) echo 1;; *) echo 2;;' "RV1d rank: SAME above LOCAL-BEHIND"
vmut RV2  '$0!~/^#/ && NF>=6 && $1==p && $2=="dirty"' '$0!~/^#/ && NF>=6 && $2=="dirty"' "RV2 exception ignores the repository path column"
vmut RV3  '[ "$o1" = 0 ] && [ "$o2" = 1 ] && [ "$r3" != 0 ]' '[ "$o1" = 0 ] && [ "$r3" != 0 ]' "RV3 needle does not require seeing the untracked file"
vmut RV4  ' && [ ! -L "$abs/$f" ]' '' "RV4 exception symlink guard removed"
vmut RV5  '[ "$o1" = 0 ] && [ "$o2" = 1 ] && [ "$r3" != 0 ]' '[ "$o2" = 1 ] && [ "$r3" != 0 ]' "RV5 needle does not require clean-reads-clean"
vmut RV6  'STATUS_ARGS=("${GITQ[@]}" status --porcelain -z --ignore-submodules=all --untracked-files=all)' 'STATUS_ARGS=("${GITQ[@]}" status --porcelain -z --ignore-submodules=all)' "N-I1 explicit --untracked-files=all dropped"
vmut RV6b 'STATUS_ARGS=("${GITQ[@]}" status --porcelain -z --ignore-submodules=all --untracked-files=all)' 'STATUS_ARGS=("${GITQ[@]}" status --porcelain -z --ignore-submodules=all --untracked-files=no)' "N-I1 --untracked-files=no (the reviewer -uno)"
vmut NB1a 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do unset "$_v"; done' 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do :; done' "N-B1 inherited git environment not scrubbed"
vmut NB1b 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do unset "$_v"; done' 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do [ "$_v" = GIT_INDEX_FILE ] || unset "$_v"; done' "N-B1 GIT_INDEX_FILE kept"
vmut NB1c 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do unset "$_v"; done' 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do [ "$_v" = GIT_DIR ] || unset "$_v"; done' "N-B1 GIT_DIR kept"
vmut NB1d 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do unset "$_v"; done' 'GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '"'"'^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'"'"'); do [ "$_v" = GIT_OBJECT_DIRECTORY ] || unset "$_v"; done' "N-B1 GIT_OBJECT_DIRECTORY kept"
skip NB1e || {
# NB1e: the needle commits again (the pre-fix behaviour) and does not disable hooks: the global-hook and hook-recursion fixtures must see it
vtree NB1e; mutate scripts/repo/verify_repos.sh "$M/NB1e/step1.sh" 'G2="$GIT -c core.hooksPath=/dev/null"' 'G2="$GIT"'; r1=$?
mutate "$M/NB1e/step1.sh" "$M/NB1e/repo/verify_repos.sh" "  printf 'garbage' > \"\$N/.git/index\"" "  \$G2 -C \"\$N\" -c user.email=n@n -c user.name=n add needle.txt >/dev/null 2>&1; \$G2 -C \"\$N\" -c user.email=n@n -c user.name=n commit -qm n >/dev/null 2>&1; printf 'garbage' > \"\$N/.git/index\""; r2=$?
chmod +x "$M/NB1e/repo/verify_repos.sh"; s=0; [ -n "${DRYRUN:-}" ] && s=1; [ "$r1$r2" = 00 ] && [ -z "${DRYRUN:-}" ] && { VR="$M/NB1e/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos.sh >"$M/NB1e.out" 2>&1; s=$?; }
verdict NB1e "$((r1+r2))" "$s" "N-B1 the needle commits again with hooks enabled"
}
vmut NB2a 'if [ "$m1" = 160000 ] || [ "$m2" = 160000 ]; then tracked=' 'if false; then tracked=' "N-B2 staged gitlink changes not detected"
vmut NB2b 'if [ "$m1" = 160000 ] || [ "$m2" = 160000 ]; then tracked=' 'if [ "$m1" = 160000 ]; then tracked=' "N-B2 a staged ADDED gitlink not detected"
vmut NB2c 'if [ "$m1" = 160000 ] || [ "$m2" = 160000 ]; then tracked=' 'if [ "$m2" = 160000 ]; then tracked=' "N-B2 a staged REMOVED gitlink not detected"
vmut NB2d '|| { fail "git diff-index --cached failed: $(head -c 160 "${outdir:?}/${idx:?}.cached.err" | tr '"'"'\n'"'"' '"'"' '"'"')"; return 1; }' '|| true' "N-B2 git diff-index exit not checked"
vmut NB2e 'cands+=("G  $pth"); fi' ':; fi' "N-B2 staged gitlink counted but listed nowhere (could be excepted)"
vmut NI1b 'case "$suf" in no|NO|No|false) echo' 'case "$suf" in zzz) echo' "N-I1 hidden-untracked configuration not noted"
vmut NI2a 'case "$tag" in S|[a-z]) ;; *) continue ;; esac' 'case "$tag" in S) ;; *) continue ;; esac' "N-I2 assume-unchanged entries ignored"
vmut NI2b 'else wh=""; [ "$tag" = S ] && continue; fi' 'else wh=""; continue; fi' "N-I2 a deleted assume-unchanged file ignored"
vmut NI2c 'else wh=""; [ "$tag" = S ] && continue; fi' 'else wh=""; fi' "N-I2 an absent skip-worktree file counted as dirt"
vmut NI2d 'if [ "$wh" != "$ish" ]; then tracked=' 'if false; then tracked=' "N-I2 flagged files never compared"
vmut NI2e '|| { fail "git ls-files -v failed: $(head -c 160 "${outdir:?}/${idx:?}.lsv.err" | tr '"'"'\n'"'"' '"'"' '"'"')"; return 1; }' '|| true' "N-I2 git ls-files -v exit not checked"
vmut MC1  '[ "$unclass" = true ] && [ "$NO_REMOTE" = 0 ] && noremote_unproven=true' ':' "m-c unclassifiable remote URL read as third-party clean"
vmut Ma1  '|| { fail "git ls-tree HEAD -- $relsub failed in the superproject"; return 1; }' '|| true' "m-a pinned-gitlink ls-tree exit not checked"
# ---- round 4 (WF3 review: I-1..I-4, m-1, m-2, m-4 and the container portability fixes): guard-removing mutants of each fix -------
vmut R4a '-c fetch.recurseSubmodules=false -c maintenance.auto=false -c gc.auto=0 -C "$abs" fetch --no-recurse-submodules --no-auto-maintenance' '-C "$abs" fetch' "I-1 --fetch recurses into submodules and runs auto maintenance again"
vmut R4b 'NF==2 && $2==want && length($1)==40' 'NF==2 && $2 ~ (want "$") && length($1)==40' "I-2 ls-remote tail match: refs/archive/refs/heads/<b> read as the branch tip"
vmut R4c 'GITQ=(-c core.fsmonitor=false -c core.untrackedCache=false)' 'GITQ=()' "I-3 core.fsmonitor / untracked cache not disabled for status, diff-index, ls-files"
vmut R4d 'elif [ -n "$sup" ]; then   # ATTACHED owned submodule' 'elif false; then   # ATTACHED owned submodule' "I-4 an attached owned submodule's pin is not checked for reachability"
vmut R4e 'case "$cls_p" in REMOTE-BEHIND|DIVERGED|UNKNOWN-DIFFERENT)' 'case "$cls_p" in REMOTE-BEHIND|DIVERGED|UNKNOWN-DIFFERENT|LOCAL-BEHIND|SAME)' "I-4 a pin merely behind the pushed tip is reported as a class (false alarm)"
vmut R4f 'GIT_LITERAL_PATHSPECS=1 $GIT' '$GIT' "m-4 flagged-file blob lookup treats the name as a glob pathspec"
vmut R4g '*) fail "git symbolic-ref failed (rc=$rc)"; return 1 ;; esac' '*) branch="" ;; esac' "m-1 a failing git symbolic-ref is read as detached"
vmut R4h 'if [ -L "$abs/$lf" ]; then wh=' 'if [ -L "$abs/$lf" ]; then continue; wh=' "m-2 a flagged symlink is never compared"
vmut R4i '"$WORK/table.tsv" "$WORK/table.tsv"' '"$WORK/table.tsv" "$WORK/table.tsv" | column -t' "RC5 the external column command is used again"
vmut R4j 'length($4)==64 && $4!~/[^0-9a-f]/ && length($5)==64' '$4~/^[0-9a-f]{64}$/ && length($5)==64' "RC2 a {64} interval expression (not understood by mawk) is used again"
R4K_OLD="$(cat <<'X'
{ n=0; while IFS=$'\t' read -r lp lpin; do n=$((n+1)); [ "$lpin" = " " ] && lpin="="; printf '%06d\0%s\0%s\0' "$n" "$lp" "$lpin"; done < "$LIST"; } | \
X
)"
R4K_NEW="$(cat <<'X'
awk -F'\t' '{printf "%06d\0%s\0%s\0", NR, $1, ($2==" "?"=":$2)}' "$LIST" | \
X
)"
vmut R4k "$R4K_OLD" "$R4K_NEW" "RC1 NUL inside an awk printf format (mawk truncates every record) is used again"
fi
# ---- org_of.py (shared parser) -------------------------------------------------------------------------------------
omut() {  # omut <id> <old> <new> <desc>: mutate org_of.py, run the verify matrix
  skip "$1" && return 0
  vtree "$1"; mutate scripts/audit/org_of.py "$M/$1/audit/org_of.py" "$2" "$3"; local r=$?
  local s=0
  if [ -n "${DRYRUN:-}" ]; then s=1
  else [ "$r" -eq 0 ] && { mkdir -p "$M/$1/repo"; cp scripts/repo/verify_repos.sh "$M/$1/repo/"; chmod +x "$M/$1/repo/verify_repos.sh"
    VR="$M/$1/repo/verify_repos.sh" bash scripts/repo/tests/test_verify_repos.sh >"$M/$1.out" 2>&1; s=$?; }; fi
  verdict "$1" "$r" "$s" "$4"
}
if [ "$WHICH" = all ] || [ "$WHICH" = verify ]; then
omut MI1b 'return m.group(1).lower()' 'return m.group(1)' "I1 organisation not lower-cased in the shared parser"
omut MI1c '(?:\.git)?/?$' '(?:\.git)?$' "I1 trailing slash not tolerated in the shared parser"
fi
# ---- derive_scope.sh ------------------------------------------------------------------------------------------------
dmut() {  # dmut <id> <old> <new> <desc>
  skip "$1" && return 0
  mkdir -p "$M/$1"; cp scripts/audit/org_of.py "$M/$1/"; mutate scripts/audit/derive_scope.sh "$M/$1/derive_scope.sh" "$2" "$3"; local r=$?; chmod +x "$M/$1/derive_scope.sh"
  local s=0; [ "$r" -eq 0 ] && { DS="$M/$1/derive_scope.sh" bash scripts/audit/tests/test_derive_scope.sh >"$M/$1.out" 2>&1; s=$?; }
  verdict "$1" "$r" "$s" "$4"
}
if [ "$WHICH" = all ] || [ "$WHICH" = derive ]; then
dmut DM2 'walk(sub, full, cls == "third_party", alt == "third_party")' 'walk(sub, full, cls == "third_party", cls == "third_party")' "alt-class inheritance uses class"
dmut DM4 'if r.returncode not in (0, 1):' 'if False:' "git config error ignored"
dmut DM5 'if os.path.exists(os.path.join(sub, ".git")):' 'if os.path.isdir(sub):' "I8 uninitialised submodule descended as if checked out"
dmut DM6 'if o is None: bad.append((full, url)); continue' 'if o is None: continue' "unclassifiable URL silently skipped"
fi
if [ "$WHICH" = all ] || [ "$WHICH" = derive ]; then
dmut DM7 'except OSError as e:
        print("orgs file unreadable' 'except ZeroDivisionError as e:
        print("orgs file unreadable' "m-g unreadable orgs file is a traceback with rc 1"
fi
dormut() {  # dormut <id> <old> <new> <desc>: mutate org_of.py, run the derive test
  skip "$1" && return 0
  mkdir -p "$M/$1"; mutate scripts/audit/org_of.py "$M/$1/org_of.py" "$2" "$3"; local r=$?; cp scripts/audit/derive_scope.sh "$M/$1/"; chmod +x "$M/$1/derive_scope.sh"
  local s=0; [ "$r" -eq 0 ] && { DS="$M/$1/derive_scope.sh" bash scripts/audit/tests/test_derive_scope.sh >"$M/$1.out" 2>&1; s=$?; }
  verdict "$1" "$r" "$s" "$4"
}
if [ "$WHICH" = all ] || [ "$WHICH" = derive ]; then
dormut DM1 'return m.group(1).lower()' 'return m.group(1)' "org match case-sensitive"
dormut DM3 'or (url or "").startswith(("./", "../"))' 'or False' "relative-URL guard removed"
fi
# ---- scope_to_lumen_json.py -----------------------------------------------------------------------------------------
smut() {  # smut <id> <old> <new> <desc>
  skip "$1" && return 0
  mkdir -p "$M/$1"; mutate scripts/audit/scope_to_lumen_json.py "$M/$1/stj.py" "$2" "$3"; local r=$?
  local s=0; [ "$r" -eq 0 ] && { STJ="$M/$1/stj.py" bash scripts/audit/tests/test_scope_to_lumen_json.sh >"$M/$1.out" 2>&1; s=$?; }
  verdict "$1" "$r" "$s" "$4"
}
if [ "$WHICH" = all ] || [ "$WHICH" = scope ]; then
smut SM1 'for key in ("allow", "deny", "classes", "dropped_negations", "root_files"):' 'for key in ("allow", "classes", "dropped_negations", "root_files"):' "--check ignores deny membership"
smut SM2 'for key in ("allow", "deny", "classes", "dropped_negations", "root_files"):' 'for key in ("deny", "classes", "dropped_negations", "root_files"):' "--check ignores allow roots"
smut SM3 '    nested = {"/" + lit(r[0]) + "/" for r in rows if r[1] == "third_party"
              and any(r[0].startswith(a + "/") for a in allow)}' '    nested = set()' "nested third-party not denied (re-pointed in round 4: the old len(r) >= 2 form no longer exists)"
smut SM8 'if seen.setdefault(r[0], r[1]) != r[1]:' 'if False:' "m-5a a path classed both own and third_party accepted"
smut SM9 '{"/" + lit(r[0]) + "/" for r in rows' '{"/" + r[0] + "/" for r in rows' "m-5b nested third_party path written as a glob (metacharacters not escaped)"
smut SM10 'if key in have and any(not isinstance(x, str) for x in have[key]):' 'if False:' "m-8 --check non-string list element (TypeError traceback)"
smut SM11 'if set(have[key]) == set(want[key]):   # same members' 'if False:   # same members' "m-8 --check duplicate entry passes (set-wise comparison)"
smut SM12 'hit = sorted(t for t in third if a == t or a.startswith(t + "/"))' 'hit = sorted(t for t in third if a == t or a.startswith(t))' "m-3 third_party root refusal without the path boundary (over-refusal of a sibling root)"
smut SM4 '            if p.startswith("!"):
                dropped.append(p)
                continue' '            if p.startswith("!"):
                pass' "negation kept"
smut SM5 'for key in ("allow", "deny", "classes", "dropped_negations", "root_files"):' 'for key in ("allow", "deny", "classes", "dropped_negations"):' "--check ignores root_files"
smut SM6 '        if have[key] == want[key]:
            continue' '        continue' "--check never compares (always rc 0)"
smut SV1 'any(r[0].startswith(a + "/") for a in allow)}' 'any(r[0].startswith(a) for a in allow)}' "nested-third-party prefix test without the path boundary"
smut SV2 'r[1] not in ("own", "third_party")' 'False' "unknown TSV classes accepted (allow = every non-third_party row)"
smut SV3 'hit = sorted(t for t in third if a == t or a.startswith(t + "/"))' 'hit = sorted(t for t in third if a == t)' "an allow root INSIDE a third_party row accepted"
smut SV4 'hit = sorted(t for t in third if a == t or a.startswith(t + "/"))' 'hit = sorted(t for t in third if a.startswith(t + "/"))' "an allow root EQUAL to a third_party row accepted"
smut SV5 'roots = {str(a).strip().strip("/")' 'roots = {str(a).strip()' "trailing slash hides a third_party root"
smut SV6 'not r[0].strip() or ' '' "a TSV row with an empty path accepted"
smut SV7 'if len(r) < 2 or not r[0].strip()' 'if not r[0].strip()' "a TSV row without a tab (IndexError traceback)"
smut SV8 '        except OSError as e:
            fail(3, "cannot write --out: %s" % e)' '        except ZeroDivisionError as e:
            fail(3, "cannot write --out: %s" % e)' "unwritable --out is a traceback"
smut SV9 '    if "classes" in have and (not isinstance(have["classes"], dict)
                              or any(not isinstance(v, list) for v in have["classes"].values())):' '    if False:' "malformed --check classes accepted (traceback / wrong rc)"
smut SM7 '    if not isinstance(scope, dict):
        fail(3,' '    if False:
        fail(3,' "non-mapping scope accepted (traceback, rc 1)"
fi
echo "MUTATION SUMMARY caught=$CAUGHT survived=$SURV broken=$BROKEN"
[ "$SURV" -eq 0 ] && [ "$BROKEN" -eq 0 ]
