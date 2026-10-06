#!/usr/bin/env bash
# T041 test (TDD): scripts/repo/push_recursive.sh - per repository and remote the longest releasable prefix, never a force.
# Throwaway repositories and LOCAL BARE remotes only; no real remote, no network, no credential. A push helper test that could
# reach a real remote would be a defect, so every remote below is a directory under $T.
# Usage: bash scripts/repo/tests/test_push_recursive.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/push_recursive.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/pr_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
SCHEMA=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
M="$T/m"; A="$T/audit"; RUND="$A/20261005T000000Z-1-aaaa"
GOJ() { printf '{"schema":"review-verdict/1","verdict":"%s","covers_runs":%s,"model":"opus","effort":"xhigh","blocking_findings":%s}\n' "$1" "$(printf '%s' "$2" | jq -c 'map({repository:".",commit:"0000000000000000000000000000000000000000",cpa_run:.})')" "$3"; }
fresh() {
  rm -rf "$T/m" "$T/b" "$A" "$T/c" "$T/sbare.git"; mkdir -p "$T/b" "$RUND" "$A/20261005T000000Z-2-bbbb"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$M"; mkdir -p "$M/specs/ev/reviews" "$M/$(dirname "$SCHEMA")"; cp "$D0/$SCHEMA" "$M/$SCHEMA"
  GOJ GO '["20261005T000000Z-1-aaaa"]' 0 > "$M/specs/ev/reviews/V_go.json"; GOJ GO '["20261005T000000Z-9-cccc"]' 0 > "$M/specs/ev/reviews/V_other.json"
  GOJ NO-GO '["20261005T000000Z-1-aaaa"]' 1 > "$M/specs/ev/reviews/V_nogo.json"; GOJ GO '["20261005T000000Z-1-aaaa"]' 2 > "$M/specs/ev/reviews/V_blk.json"
  GOJ GO '["20261005T000000Z-1-aaaa"]' 0 | jq -c 'del(.model,.effort)' > "$M/specs/ev/reviews/V_bad.json"
  GOJ GO '["20261005T000000Z-2-bbbb"]' 0 > "$M/specs/ev/reviews/CPA-merge-20261005T000000Z-2-bbbb.json"
  echo base > "$M/base.txt"; commit_all "$M" init
  for r in r1 r2; do git -C "$M" remote add $r "$T/b/$r.git"; git -C "$M" push -q $r main; done
}
cpa() { # cpa <repo dir> <repo key> <file> [awaits-review path] [run id]
  local d="$1" k="$2" f="$3" aw="${4-}" id="${5:-20261005T000000Z-1-aaaa}"; echo "$f $RANDOM" > "$d/$f"; git -C "$d" add -- "$f"
  git -C "$d" commit -q -m "change $f" -m "CPA-Run: $id${aw:+
Awaits-Review: $aw}"; printf '%s\t%s\t%s\n' "$k" "$(git -C "$d" rev-parse HEAD)" "$id" >> "$A/$id/commits.tsv"; }
run() { "$H" --main-root "$M" --branch main --run-dir "$RUND" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
tip() { git -C "$T/b/$1.git" rev-parse main 2>/dev/null || echo none; }
head_() { git -C "${1:-$M}" rev-parse HEAD; }
fresh
# 1 golden: two CPA commits are pushed to every remote, as <sha>:refs/heads/<branch>
cpa "$M" . a.txt; cpa "$M" . b.txt; want="$(head_)"
run; eq "two unheld CPA commits: 0" "$RC" 0; eq "r1 holds the tip" "$(tip r1)" "$want"; eq "r2 holds the tip" "$(tip r2)" "$want"
has "report names each push" "$(cat "$T/out")" "PUSHED	.	r1	$want"
run; eq "nothing outgoing: 0" "$RC" 0; has "already holds the tip, no push call" "$(cat "$T/out")" "already_holds_tip"
# 2 a commit that is not a CPA commit withholds every push of the repository (20 unrecorded_local_commit)
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; hand="$(head_)"
run; eq "trailer-less commit: 20" "$RC" 20; has "names the commit" "$(cat "$T/out")$(cat "$T/err")" "$hand"; has "reason" "$(cat "$T/out")$(cat "$T/err")" "unrecorded_local_commit"
eq "no remote received anything" "$(tip r1)$(tip r2)" "$(git -C "$M" rev-parse HEAD~2)$(git -C "$M" rev-parse HEAD~2)"
fresh; echo copied > "$M/c.txt"; git -C "$M" add c.txt; git -C "$M" commit -q -m "copied trailer" -m "CPA-Run: 20261005T000000Z-1-aaaa"
run; eq "copied CPA-Run trailer, no commits.tsv row: 20" "$RC" 20; eq "r1 unchanged" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~1)"
fresh; cpa "$M" . a.txt; echo x > "$M/o.txt"; git -C "$M" add o.txt; git -C "$M" commit -q -m "other run" -m "CPA-Run: 20261005T000000Z-2-bbbb"; printf '.\t%s\t20261005T000000Z-2-bbbb\n' "$(head_)" > "$A/20261005T000000Z-2-bbbb/commits.tsv"
run; eq "a CPA commit of another run, recorded in that run's commits.tsv: 0" "$RC" 0
fresh; echo x > "$M/o.txt"; git -C "$M" add o.txt; git -C "$M" commit -q -m "wrong row" -m "CPA-Run: 20261005T000000Z-1-aaaa"; printf '.\t%s\t20261005T000000Z-1-aaaa\n' "0000000000000000000000000000000000000000" > "$A/20261005T000000Z-1-aaaa/commits.tsv"
run; eq "commits.tsv row for another sha: 20" "$RC" 20
# 3 held commits: released only by a valid GO committed in the main HEAD that covers the commit's run
rel() { # rel <label> <verdict path> <want rc> -- one unheld commit, one held commit
  fresh; cpa "$M" . a.txt; cpa "$M" . h.txt "$2"; run; eq "$1: exit" "$RC" "$3"
}
rel "GO committed in HEAD, run listed" specs/ev/reviews/V_go.json 0; eq "held commit pushed" "$(tip r1)" "$(head_)"
rel "GO covering another run only" specs/ev/reviews/V_other.json 14; eq "held commit stays held (r1 at the unheld prefix)" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~1)"; has "HELD line" "$(cat "$T/out")" "HELD"
rel "NO-GO verdict" specs/ev/reviews/V_nogo.json 14
rel "GO with blocking findings" specs/ev/reviews/V_blk.json 14
rel "verdict failing the schema (no model/effort)" specs/ev/reviews/V_bad.json 14
rel "verdict file absent" specs/ev/reviews/V_missing.json 14
fresh; GOJ GO '["20261005T000000Z-1-aaaa"]' 0 > "$M/specs/ev/reviews/V_wt.json"; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_wt.json; run
eq "GO only uncommitted in the working tree: stays held (14)" "$RC" 14
fresh; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_go.json; cpa "$M" . z.txt; run; eq "unheld, held-and-released, unheld: all pushed" "$(tip r1)" "$(head_)"
fresh; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_nogo.json; cpa "$M" . z.txt; run
eq "unheld, held, unheld: only the prefix below the first unreleased commit" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~2)"
fresh; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_nogo.json; cpa "$M" . h2.txt specs/ev/reviews/V_nogo.json; cpa "$M" . z.txt; run
eq "two unreleased held commits: the prefix stops below the FIRST" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~3)"
fresh; cpa "$M" . h.txt "../../outside/V.json"; run; eq "unsafe verdict path in an Awaits-Review line stays held, never read: 14" "$RC" 14
# 4 an unreleased CPA merge commit in the range: nothing below it is pushed to any remote
mergefix() { # mergefix <verdict path>
  fresh; cpa "$M" . a.txt; git -C "$M" checkout -q -b side HEAD~1; echo s > "$M/s.txt"; git -C "$M" add s.txt; git -C "$M" commit -q -m side -m "CPA-Run: 20261005T000000Z-1-aaaa"
  printf '.\t%s\t20261005T000000Z-1-aaaa\n' "$(head_)" >> "$A/20261005T000000Z-1-aaaa/commits.tsv"; git -C "$M" checkout -q main
  git -C "$M" merge -q --no-ff -m "Merge side (CPA)" -m "CPA-Run: 20261005T000000Z-1-aaaa
Awaits-Review: $1" side; printf '.\t%s\t20261005T000000Z-1-aaaa\n' "$(head_)" >> "$A/20261005T000000Z-1-aaaa/commits.tsv"
}
mergefix specs/ev/reviews/CPA-merge-20261005T000000Z-2-bbbb.json; run; eq "merge whose verdict does not cover its run: 14" "$RC" 14
eq "no remote received the commit below the merge (r1)" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~2 2>/dev/null)"; eq "no push to r2 either" "$(tip r2)" "$(tip r1)"
mergefix specs/ev/reviews/V_go.json; run; eq "merge released by a GO that covers its run: 0" "$RC" 0; eq "merge tip pushed" "$(tip r1)" "$(head_)"
# 5 a remote whose live tip moved since S1 gets no push call; the others continue
fresh; cpa "$M" . a.txt; git clone -q "$T/b/r2.git" "$T/c"; ( cd "$T/c" && git config user.email t@t && git config user.name t && echo other > o.txt && git add o.txt && git commit -qm other && git push -q origin main ); moved="$(tip r2)"
run; eq "one remote moved: 11" "$RC" 11; has "r2 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"; eq "r2 unchanged" "$(tip r2)" "$moved"; eq "r1 received the tip" "$(tip r1)" "$(head_)"
# 6 a remote without the branch is pushed; an unreachable remote and a rejecting remote are recorded, the others continue
fresh; git init -q --bare -b main "$T/b/r3.git"; git -C "$M" remote add r3 "$T/b/r3.git"; cpa "$M" . a.txt; run; eq "remote without the branch: 0" "$RC" 0; eq "r3 received the tip" "$(tip r3)" "$(head_)"
fresh; git -C "$M" remote add a4 "$T/b/nonexistent.git"; cpa "$M" . a.txt; run; eq "unreachable remote: 11" "$RC" 11; has "recorded per remote" "$(cat "$T/out")" "a4"; eq "r1 still pushed" "$(tip r1)" "$(head_)"
fresh; git init -q --bare -b main "$T/b/a5.git"; printf '#!/bin/sh\nexit 1\n' > "$T/b/a5.git/hooks/pre-receive"; chmod +x "$T/b/a5.git/hooks/pre-receive"; git -C "$M" remote add a5 "$T/b/a5.git"
cpa "$M" . a.txt; run; eq "push rejected by a remote: 11" "$RC" 11; has "PUSH_FAILED recorded" "$(cat "$T/out")" "PUSH_FAILED	.	a5"; eq "others continued (r1)" "$(tip r1)" "$(head_)"
# 7 never a force, never an unvalidated value
fresh; cpa "$M" . a.txt
for f in --force -f --force-with-lease --force-with-lease=main +main --mirror --delete; do run "$f"; eq "option $f refused: 20" "$RC" 20; has "option $f named force_refused" "$(cat "$T/err")" force_refused; done
eq "nothing pushed by any refusal" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~1)"
"$H" --main-root "$M" --branch '-x' --run-dir "$RUND" >/dev/null 2>&1; eq "branch starting with a dash: 20" "$?" 20
"$H" --main-root "$M" --branch 'a..b' --run-dir "$RUND" >/dev/null 2>&1; eq "branch that is no ref name: 20" "$?" 20
"$H" --main-root "-x" --branch main --run-dir "$RUND" >/dev/null 2>&1; eq "main root starting with a dash: 20" "$?" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --main-root -x --branch main --run-dir "$RUND" ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20
"$H" --main-root "$M" --branch main --run-dir "$T/norun" >/dev/null 2>&1; eq "run directory absent: 20" "$?" 20
"$H" --main-root "$M" --branch main >/dev/null 2>&1; eq "no --run-dir: 20" "$?" 20
run --bogus; eq "unknown option: 20" "$RC" 20
git -C "$M" config 'remote.-oevil.url' "$T/b/r1.git"; run; eq "remote named like an option: 20" "$RC" 20; has "unsafe_remote_name" "$(cat "$T/out")$(cat "$T/err")" "unsafe_remote_name"; git -C "$M" config --remove-section 'remote.-oevil'
git -C "$M" config remote.r9.url '-oevil'; run; eq "remote URL starting with a dash: 20" "$RC" 20; has "unsafe_remote_url" "$(cat "$T/out")$(cat "$T/err")" "unsafe_remote_url"; git -C "$M" config --remove-section remote.r9
eq "still nothing pushed" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~1)"
# 6b a remote tip that is held locally but is not an ancestor of the target gives no push call (ancestry is really checked)
fresh; git -C "$M" checkout -q -b other; echo o > "$M/o.txt"; git -C "$M" add o.txt; git -C "$M" commit -qm other; git -C "$M" push -q r2 other:main; git -C "$M" checkout -q main
cpa "$M" . a.txt; run; eq "known remote tip that diverged from the target: 11" "$RC" 11; has "r2 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"; eq "r1 pushed" "$(tip r1)" "$(head_)"
# 6c an ext:: transport URL is refused before any git call and runs nothing
fresh; cpa "$M" . a.txt; git -C "$M" config remote.rx.url "ext::sh -c 'touch $T/pwned'"; run; eq "remote URL with the ext:: transport: 20" "$RC" 20; has "unsafe_remote_url" "$(cat "$T/out")$(cat "$T/err")" unsafe_remote_url; eq "nothing was executed" "$(test -e "$T/pwned"; echo $?)" 1
# 7b every git call the helper makes: pushes carry only <sha>:refs/heads/<branch> after `--`, never a plus, never a force-like flag
fresh; cpa "$M" . a.txt; mkdir -p "$T/shim"; REALGIT="$(command -v git)"
printf '#!/bin/sh\ncase "$*" in *push*|*ls-remote*) printf "%%s\\n" "$*" >> "$GITLOG" ;; esac\nexec "%s" "$@"\n' "$REALGIT" > "$T/shim/git"; chmod +x "$T/shim/git"; : > "$T/gitlog"
PATH="$T/shim:$PATH" GITLOG="$T/gitlog" run; eq "run through the logging shim: 0" "$RC" 0
eq "every push is 'push -q -- <remote> <sha>:refs/heads/main'" "$(grep ' push ' "$T/gitlog" | grep -vcE ' push -q -- [A-Za-z0-9._-]+ [0-9a-f]{40}:refs/heads/main$')" 0
eq "at least one push was logged" "$(grep -c ' push ' "$T/gitlog" | awk '{print ($1>0)?"yes":"no"}')" yes
eq "every ls-remote follows --" "$(grep ' ls-remote ' "$T/gitlog" | grep -vcE ' ls-remote -- ')" 0
eq "no plus refspec and no force flag anywhere" "$(grep -cE ' \+|--force| -f ' "$T/gitlog")" 0
# 8 submodules at every depth, deepest first; a trailer-less commit made in a submodule gets no push call
fresh; git init -q --bare -b main "$T/sbare.git"; git clone -q "$T/sbare.git" "$T/sinit" 2>/dev/null; ( cd "$T/sinit" && git config user.email t@t && git config user.name t && echo s > s && git add s && git commit -qm s && git push -q origin HEAD:main ); rm -rf "$T/sinit"
( cd "$M" && git submodule -q add -b main "$T/sbare.git" mods/sub && git commit -qm "add sub" && git push -q r1 main && git push -q r2 main ); S="$M/mods/sub"
git -C "$S" config user.email t@t; git -C "$S" config user.name t; git -C "$S" checkout -q -B main origin/main
cpa "$S" mods/sub s1.txt; cpa "$M" . m1.txt; run --recursive; eq "submodule and main, recursive: 0" "$RC" 0
eq "submodule remote received its tip" "$(git -C "$T/sbare.git" rev-parse main)" "$(head_ "$S")"; eq "main r1 received its tip" "$(tip r1)" "$(head_)"
eq "deepest first: the submodule is reported before the main repository" "$(grep -n 'PUSHED' "$T/out" | head -2 | awk -F'\t' '{print $2}' | tr '\n' ' ')" "mods/sub . "
cpa "$S" mods/sub s2.txt; echo hand > "$S/h.txt"; git -C "$S" add h.txt; git -C "$S" commit -qm "hand commit in submodule"; sbefore="$(git -C "$T/sbare.git" rev-parse main)"
run --recursive; eq "trailer-less commit in a submodule: 20" "$RC" 20; eq "the submodule remote got no push call" "$(git -C "$T/sbare.git" rev-parse main)" "$sbefore"
git -C "$S" reset -q --hard HEAD~2; cpa "$S" mods/sub s3.txt; git -C "$S" commit -q --amend -m "copied" -m "CPA-Run: 20261005T000000Z-1-aaaa"
run --recursive; eq "copied trailer in a submodule: 20" "$RC" 20
# 9 a verdict that exists only in the pushed repository's own tree does not release a held submodule commit
git -C "$S" reset -q --hard HEAD~1; mkdir -p "$S/specs/ev/reviews"; GOJ GO '["20261005T000000Z-1-aaaa"]' 0 > "$S/specs/ev/reviews/V_sub.json"
cpa "$S" mods/sub s4.txt specs/ev/reviews/V_sub.json; run --repo mods/sub; eq "verdict only in the submodule tree: held (14)" "$RC" 14
eq "submodule remote still at its old tip" "$(git -C "$T/sbare.git" rev-parse main)" "$(git -C "$S" rev-parse HEAD~1)"
# 10 (WF5 F1) every reachable remote moved after S1: their tips are not held locally, so nothing is known to be excluded from the outgoing set;
#    that is remote_moved_since_s1 (11), NEVER a false unrecorded_local_commit (20) naming already-published commits
moveall() { local r; for r in "$@"; do rm -rf "$T/c_$r"; git clone -q "$T/b/$r.git" "$T/c_$r"; ( cd "$T/c_$r" && git config user.email t@t && git config user.name t && echo "other $r" > o_$r.txt && git add . && git commit -qm "other $r" && git push -q origin main ); done; }
fresh; cpa "$M" . a.txt; moveall r1 r2; m1="$(tip r1)"; m2="$(tip r2)"; run
eq "every remote moved after S1, nothing unrecorded: 11 (F1)" "$RC" 11; hasnot "no false unrecorded_local_commit" "$(cat "$T/out")" unrecorded_local_commit
has "r1 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r1	remote_moved_since_s1"; has "r2 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"
eq "r1 untouched" "$(tip r1)" "$m1"; eq "r2 untouched" "$(tip r2)" "$m2"
fresh; cpa "$M" . a.txt; moveall r1; run; eq "single moved remote and one that did not: 11, no false refusal" "$RC" 11; hasnot "no unrecorded_local_commit (one moved)" "$(cat "$T/out")" unrecorded_local_commit
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; hand="$(head_)"; moveall r1 r2; m1h="$(tip r1)"; run
eq "every remote moved AND a real hand commit: 11 (round 6 W6-1: conservative, was 20 in round 5; the hand commit is re-judged by S1 once the moved tips are fetched)" "$RC" 11
has "names the moved remote r1" "$(cat "$T/out")" "NOPUSH	.	r1	remote_moved_since_s1"; has "names the moved remote r2" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"
eq "no commit is named unrecorded while a remote moved" "$(grep -c unrecorded_local_commit "$T/out")" 0; eq "nothing pushed to r1" "$(tip r1)" "$m1h"
fresh; cpa "$M" . a.txt; moveall r1 r2; git -C "$M" update-ref -d refs/remotes/r1/main; git -C "$M" update-ref -d refs/remotes/r2/main; run
eq "every remote moved and no tracking ref at all: 11, never 20" "$RC" 11; hasnot "no unrecorded_local_commit (no tracking ref)" "$(cat "$T/out")" unrecorded_local_commit
has "moved remote still named" "$(cat "$T/out")" "remote_moved_since_s1"
# a git shim that FAILS (rc 128) when the argument list matches a glob, else runs the real git: a failing listing must never read as clean
mkshim() { mkdir -p "$T/fshim"; local rg; rg="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in $FAILPAT) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg" > "$T/fshim/git"; chmod +x "$T/fshim/git"; }
# 11 (WF5 F3) a failing gitlink listing under --recursive is no "no submodules": 20, nothing pushed
fresh; cpa "$M" . a.txt; mkshim; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='*ls-files -s -z*' run --recursive; eq "gitlink listing failing under --recursive: 20 (F3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed
eq "nothing pushed on the refusal" "$(tip r1)" "$r1b"

# ---- round 6 (WF6 W6-1 .. W6-4, W6-8, W6-9, W6-12) ----
# 12 (W6-2) a failing `git rev-list` is no "nothing outgoing": 20 git_listing_failed, nothing pushed (the old mapfile over a process substitution lost the status)
fresh; cpa "$M" . a.txt; mkshim; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='*rev-list --topo-order*' run; eq "rev-list failing, CPA commit only: 20 (W6-2)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/out")$(cat "$T/err")" git_listing_failed
eq "nothing pushed to r1 when rev-list fails" "$(tip r1)" "$r1b"; hasnot "no PUSHED line" "$(cat "$T/out")" PUSHED
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='*rev-list --topo-order*' run; eq "rev-list failing with an unrecorded hand commit: 20, never published (W6-2)" "$RC" 20
eq "the hand commit was not pushed to r1" "$(tip r1)" "$r1b"; eq "nor to r2" "$(tip r2)" "$r1b"; hasnot "no PUSHED line (hand commit)" "$(cat "$T/out")" PUSHED
# 12b (W6-12) a failing `git remote` listing is no "no remotes": 20, nothing pushed
fresh; cpa "$M" . a.txt; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='* remote' run; eq "git remote failing: 20 (W6-12)" "$RC" 20; has "named git_listing_failed (remote)" "$(cat "$T/out")$(cat "$T/err")" git_listing_failed; eq "nothing pushed when the remote list fails" "$(tip r1)" "$r1b"
# 13 (W6-1) S1 fast-forwards to a foreign commit F WITHOUT writing a tracking ref, a CPA commit is made on F, then every remote moves again:
#    the stale tracking ref (base) must never make F look unrecorded: 11 remote_moved_since_s1, not 20 naming the published F
fresh; rm -rf "$T/cf"; git clone -q "$T/b/r1.git" "$T/cf"; ( cd "$T/cf" && git config user.email t@t && git config user.name t && echo foreign > f.txt && git add f.txt && git commit -qm foreign && git push -q origin main && git push -q "$T/b/r2.git" main ); F="$(git -C "$T/cf" rev-parse HEAD)"
git -C "$M" fetch -q --no-tags --refmap= r1 refs/heads/main; git -C "$M" merge -q --ff-only "$F"; eq "S1-form fast-forward moved HEAD to F" "$(head_)" "$F"
eq "tracking ref r1/main is still the stale base (S1 writes none)" "$(git -C "$M" rev-parse refs/remotes/r1/main)" "$(git -C "$M" rev-parse "$F^")"
cpa "$M" . a.txt; moveall r1 r2; m1="$(tip r1)"; m2="$(tip r2)"; run
eq "S1-form ff, CPA commit, every remote moved again: 11 (W6-1)" "$RC" 11; hasnot "no false unrecorded_local_commit naming the published F" "$(cat "$T/out")" unrecorded_local_commit
has "r1 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r1	remote_moved_since_s1"; eq "r1 untouched" "$(tip r1)" "$m1"; eq "r2 untouched" "$(tip r2)" "$m2"
# 13b conservative rule: unrecorded commits AND a moved remote are never told apart from here (S1 re-run decides): 11, nothing pushed; with NO remote moved the refusal stays 20
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; moveall r1; run
eq "hand commit and one moved remote: 11 (conservative, W6-1)" "$RC" 11; has "moved remote named" "$(cat "$T/out")" "NOPUSH	.	r1	remote_moved_since_s1"; hasnot "no unrecorded_local_commit while a remote moved" "$(cat "$T/out")" unrecorded_local_commit
eq "r2 not pushed either" "$(tip r2)" "$(git -C "$M" rev-parse refs/remotes/r2/main)"
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; hand="$(head_)"; run
eq "golden-false: hand commit, NO remote moved: still 20" "$RC" 20; has "names the hand commit" "$(cat "$T/out")" "REFUSED	.	unrecorded_local_commit	$hand"
# 13c moved remote with a tracking ref plus an unreachable remote with no known tip, hand commit: both are named (W6-9)
fresh; cpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"; moveall r1; mv "$T/b/r2.git" "$T/b/r2.gone"; git -C "$M" update-ref -d refs/remotes/r2/main; run
eq "moved r1 (tracking ref) and unreachable r2 (no known tip), hand commit: 11" "$RC" 11; has "r1 named moved" "$(cat "$T/out")" "NOPUSH	.	r1	remote_moved_since_s1"; has "r2 named unreachable" "$(cat "$T/out")" "NOPUSH	.	r2	remote_unreachable"
mv "$T/b/r2.gone" "$T/b/r2.git"
# 14 (W6-3) a submodule at the path '?x' is no listing failure: the marker shares no stream with the paths
fresh; git init -q --bare -b main "$T/qx.git"; git init -q -b main "$M/?x"; echo q > "$M/?x/q"; git -C "$M/?x" add q; git -C "$M/?x" commit -qm q
git -C "$M/?x" remote add origin "$T/qx.git"; git -C "$M/?x" push -q origin main
git -C "$M" update-index --add --cacheinfo "160000,$(git -C "$M/?x" rev-parse HEAD),?x"; git -C "$M" commit -qm "gitlink ?x"; git -C "$M" push -q r1 main; git -C "$M" push -q r2 main
cpa "$M" . a.txt; run --recursive; eq "submodule at the path ?x, nothing outgoing there: 0 (W6-3)" "$RC" 0; eq "main received the CPA commit" "$(tip r1)" "$(head_)"
has "?x reported as a repository" "$(cat "$T/out")" "?x"; hasnot "no git_listing_failed for ?x" "$(cat "$T/err")$(cat "$T/out")" git_listing_failed
# 15 (W6-4) an unsafe gitlink path is a REFUSAL (20), effective whatever the process layout: nothing pushed
fresh; git -C "$M" update-index --add --cacheinfo "160000,$(git -C "$M" rev-parse HEAD),-evil"; git -C "$M" commit -qm "gitlink -evil"; git -C "$M" push -q r1 main; git -C "$M" push -q r2 main
cpa "$M" . a.txt; r1b="$(tip r1)"; run --recursive; eq "gitlink path -evil under --recursive: 20 (W6-4)" "$RC" 20; has "named unsafe_repo" "$(cat "$T/err")" unsafe_repo; eq "nothing pushed on the unsafe gitlink" "$(tip r1)" "$r1b"
# 16 (W6-8) a gitlink listing that fails only INSIDE a submodule refuses 20 (nested), main not pushed
fresh; git init -q --bare -b main "$T/sbare.git"; git clone -q "$T/sbare.git" "$T/sinit" 2>/dev/null; ( cd "$T/sinit" && git config user.email t@t && git config user.name t && echo s > s && git add s && git commit -qm s && git push -q origin HEAD:main ); rm -rf "$T/sinit"
( cd "$M" && git submodule -q add -b main "$T/sbare.git" mods/sub && git commit -qm "add sub" && git push -q r1 main && git push -q r2 main ); S="$M/mods/sub"; git -C "$S" config user.email t@t; git -C "$S" config user.name t
cpa "$M" . m1.txt; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='*mods/sub ls-files -s -z*' run --recursive; eq "listing failing only inside mods/sub: 20 (W6-8)" "$RC" 20; has "named git_listing_failed with the submodule path" "$(cat "$T/err")" "git_listing_failed: mods/sub"
eq "nothing pushed when a nested listing fails" "$(tip r1)" "$r1b"
# 16b (W6-12 / mutation F8) a failing parents listing of a held commit is a refusal (20), not "no merge commit": nothing is pushed
fresh; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_nogo.json; r1b="$(tip r1)"
PATH="$T/fshim:$PATH" FAILPAT='*rev-list --parents*' run; eq "parents listing failing for a held commit: 20 (W6-12)" "$RC" 20; has "named git_listing_failed (parents)" "$(cat "$T/out")" "git_listing_failed"
eq "nothing pushed when a parents listing fails" "$(tip r1)" "$r1b"
fresh; cpa "$M" . a.txt; cpa "$M" . h.txt specs/ev/reviews/V_nogo.json; run; eq "control, real git: the held commit stays held (14), the prefix is pushed" "$RC" 14; eq "prefix below the held commit pushed" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~1)"
fin
