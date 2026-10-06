#!/usr/bin/env bash
# T040 test (TDD): scripts/repo/integrate_merge.sh - the merge path of CPA stage S1. Throwaway repositories and LOCAL BARE remotes
# only; no real remote, no network. Usage: bash scripts/repo/tests/test_integrate_merge.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/integrate_merge.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/im_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
M="$T/m"; A="$T/audit"; RID=20261005T000000Z-1-aaaa; RUND="$A/$RID"; EVR=ev
fresh() {
  rm -rf "$T/m" "$T/b" "$T/c1" "$T/c2" "$A"; mkdir -p "$T/b" "$RUND"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$M"; printf '/.audit/\n' > "$M/.gitignore"; mkdir -p "$M/scripts/repo" "$M/$EVR"
  echo base > "$M/base.txt"; echo f0 > "$M/f.txt"; echo x0 > "$M/x.txt"; printf 'gate0\n' > "$M/gate.txt"; echo t0 > "$M/scripts/repo/check_classes.tsv"; printf 'd0\n' > "$M/$EVR/deferrals.jsonl"
  commit_all "$M" init; INIT="$(git -C "$M" rev-parse HEAD)"
  for r in r1 r2; do git -C "$M" remote add $r "$T/b/$r.git"; git -C "$M" push -q $r main; done
  for c in c1 c2; do git clone -q "$T/b/r$([ $c = c1 ] && echo 1 || echo 2).git" "$T/$c"; git -C "$T/$c" config user.email t@t; git -C "$T/$c" config user.name t; done
}
rc_() { # rc_ <clone> <file> <content> [message]: commit in the clone and push to its origin
  printf '%b' "$3" > "$T/$1/$2"; git -C "$T/$1" add -A; git -C "$T/$1" commit -q -m "${4:-remote change $2}"; git -C "$T/$1" push -q origin main; }
loc() { printf '%b' "$2" > "$M/$1"; git -C "$M" add -A -- "$1"; git -C "$M" commit -q -m "${3:-local change $1}"; }
run() { "$H" --root "$M" --branch main --run-dir "$RUND" --ev "$EVR" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
tip() { git -C "$T/b/$1.git" rev-parse main; }
body() { git -C "$M" log -1 --format=%B "${1:-HEAD}"; }
head_() { git -C "$M" rev-parse HEAD; }
fresh
# 1 golden: a clean merge of a diverged remote tip; nothing is pushed; backup, merge.json, row, trailers
loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; RT="$(tip r1)"; LT="$(head_)"; echo uncommitted > "$M/u.txt"
run; eq "diverged tip merged: 0" "$RC" 0
eq "merge commit with two parents" "$(git -C "$M" rev-list --parents -n1 HEAD | wc -w | tr -d ' ')" 3
eq "message subject" "$(body | head -1)" "Merge r1/main $RT (CPA)"; has "CPA-Run trailer" "$(body)" "CPA-Run: $RID"; hasnot "unheld: no Awaits-Review" "$(body)" "Awaits-Review"
eq "both sides present, uncommitted file untouched" "$(ls "$M" | grep -cE '^(l|r|u)\.txt$')" 3
eq "r1 tip unchanged (no push)" "$(tip r1)" "$RT"; eq "r2 tip unchanged" "$(tip r2)" "$INIT"
eq "commits.tsv row" "$(tail -n1 "$RUND/commits.tsv")" ".	$(head_)	$RID"
eq "merge.json repository" "$(jq -r .repository "$RUND/merge.json")" "."; eq "merge.json tip" "$(jq -r .tip "$RUND/merge.json")" "$RT"
eq "merge.json local_tip" "$(jq -r .local_tip "$RUND/merge.json")" "$LT"; eq "merge.json run_id" "$(jq -r .run_id "$RUND/merge.json")" "$RID"
eq "merge.json pid is a number" "$(jq -r '.pid|type' "$RUND/merge.json")" number; eq "merge.json pid_start from /proc/<pid>/stat" "$(jq -r '.pid_start|length>0' "$RUND/merge.json")" true
eq "bundle verifies" "$(git -C "$M" bundle verify "$RUND/backup/main.bundle" >/dev/null 2>&1; echo $?)" 0
has "bundle holds the local commit" "$(git -C "$M" bundle list-heads "$RUND/backup/main.bundle")" "$LT"
eq "uncommitted file copied to the backup" "$(cat "$RUND/backup/worktree/u.txt")" uncommitted
has "backup sha256 line" "$(cat "$RUND/backup/worktree.sha256")" "$(printf 'uncommitted\n' | sha256sum | cut -d' ' -f1)"
run; eq "second run: nothing to merge: 0" "$RC" 0; has "reported" "$(cat "$T/out")" nothing_to_merge
# 2 Foreign-Commit lines: non-CPA incoming commits above the anchor, not the CPA ones
fresh; ANC="$(head_)"; mkdir -p "$A/20261005T000000Z-7-zzzz"; loc l.txt 'l\n'
rc_ c1 a.txt 'a\n' "foreign commit"; FA="$(git -C "$T/c1" rev-parse HEAD)"
rc_ c1 b.txt 'b\n' "cpa commit
CPA-Run: 20261005T000000Z-7-zzzz"; CB="$(git -C "$T/c1" rev-parse HEAD)"; printf '.\t%s\t20261005T000000Z-7-zzzz\n' "$CB" > "$A/20261005T000000Z-7-zzzz/commits.tsv"
run --anchor "$ANC"; eq "merge with anchor: 0" "$RC" 0; has "foreign commit named" "$(body)" "Foreign-Commit: $FA"; hasnot "CPA commit not named" "$(body)" "Foreign-Commit: $CB"
fresh; loc l.txt 'l\n'; rc_ c1 a.txt 'a\n'; run; hasnot "without an anchor no foreign commit is named" "$(body)" "Foreign-Commit"
# 3 held merges
fresh; loc l.txt 'l\n' "held local
Awaits-Review: ev/reviews/V.json"; rc_ c1 r.txt 'r\n'; run
has "local held commit missing from a remote: Awaits-Review" "$(body)" "Awaits-Review: $EVR/reviews/CPA-merge-$RID.json"; has "reported held" "$(cat "$T/out")" "local_held_commit"
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; printf 'gate.txt\tG-GATE\n' > "$T/gates.tsv"; rc_ c1 gate.txt 'X\n' "gate change"
run --path-gates "$T/gates.tsv"; has "G-GATE content outside the approved manifest: held" "$(body)" "Awaits-Review"; has "reason gate" "$(cat "$T/out")" "gate"
fresh; loc l.txt 'l\n'; rc_ c1 gate.txt 'X\n' "gate change"; printf 'gate.txt\tG-GATE\n' > "$T/gates.tsv"; printf '{"gate.txt":"%s"}\n' "$(printf 'X\n' | sha256sum | cut -d' ' -f1)" > "$T/approved.json"
run --path-gates "$T/gates.tsv" --approved "$T/approved.json"; hasnot "G-GATE content equal to the approved entry: not held" "$(body)" "Awaits-Review"
fresh; loc l.txt 'l\n'; rc_ c1 scripts/repo/check_classes.tsv 't1\n' "foreign table change"
run; has "admission table changed by a non-CPA commit: held" "$(body)" "Awaits-Review"
fresh; loc l.txt 'l\n'; mkdir -p "$A/20261005T000000Z-7-zzzz"; rc_ c1 scripts/repo/check_classes.tsv 't1\n' "cpa table change
CPA-Run: 20261005T000000Z-7-zzzz"; printf '.\t%s\t20261005T000000Z-7-zzzz\n' "$(git -C "$T/c1" rev-parse HEAD)" > "$A/20261005T000000Z-7-zzzz/commits.tsv"
run; hasnot "admission table changed by a recorded CPA commit: not held" "$(body)" "Awaits-Review"
# adoption: a descendant tip whose first-parent chain lacks the adoption commit is a target; without the option it is not
fresh; git -C "$T/c1" checkout -q -b side; echo s > "$T/c1/s.txt"; git -C "$T/c1" add -A; git -C "$T/c1" commit -q -m side; AD="$(git -C "$T/c1" rev-parse HEAD)"
git -C "$T/c1" checkout -q main; git -C "$T/c1" merge -q --no-ff -m "merge side" side; git -C "$T/c1" push -q origin main
run; eq "descendant tip, no adoption option: nothing to merge" "$RC" 0; has "reported" "$(cat "$T/out")" nothing_to_merge
run --adoption-commit "$AD"; eq "descendant tip lacking the adoption commit on its first-parent chain: merged" "$RC" 0; has "held for adoption" "$(body)" "Awaits-Review"
# 4 conflict: files copied with markers, merge aborted, nothing changed
fresh; loc f.txt 'local\n'; rc_ c1 f.txt 'remote\n'; echo dirtybase >> "$M/base.txt"; LT="$(head_)"; st0="$(git -C "$M" status --porcelain)"
run; eq "conflicting merge: 12" "$RC" 12; has "merge_conflict naming the path" "$(cat "$T/out")" "merge_conflict: f.txt"
has "conflict file copy holds markers" "$(cat "$RUND/conflicts/f.txt")" "<<<<<<<"; eq "HEAD unchanged" "$(head_)" "$LT"
eq "work tree status unchanged" "$(git -C "$M" status --porcelain)" "$st0"; eq "no MERGE_HEAD" "$(test -e "$M/.git/MERGE_HEAD"; echo $?)" 1
eq "no commits.tsv row" "$(cat "$RUND/commits.tsv" 2>/dev/null | wc -l | tr -d ' ')" 0
# 5 working tree blocks
fresh; loc l.txt 'l\n'; rc_ c1 base.txt 'incoming\n'; echo mine >> "$M/base.txt"; LT="$(head_)"
run; eq "nothing written to the run dir (checked before anything moves)" "$(ls "$RUND" | wc -l | tr -d ' ')" 0; eq "uncommitted change in a path the incoming commits change: 12" "$RC" 12; has "named" "$(cat "$T/out")" "ff_blocked_by_local_changes"; eq "HEAD unchanged" "$(head_)" "$LT"; eq "my change kept" "$(tail -1 "$M/base.txt")" mine
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; echo staged > "$M/s.txt"; git -C "$M" add s.txt; run; eq "index differs from HEAD: 12" "$RC" 12; eq "nothing written to the run dir (index case)" "$(ls "$RUND" | wc -l | tr -d ' ')" 0
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; echo mine > "$M/r.txt"; run; eq "untracked file at an incoming path: 12" "$RC" 12; eq "nothing written to the run dir (untracked case)" "$(ls "$RUND" | wc -l | tr -d ' ')" 0
# 6 several targets
fresh; loc l.txt 'l\n'; rc_ c1 x.txt 'one\n'; rc_ c2 x.txt 'two\n'; LT="$(head_)"
run; eq "two targets that conflict with each other: 12" "$RC" 12; has "remotes_diverged naming the pair" "$(cat "$T/out")" "remotes_diverged r1"; has "and r2" "$(cat "$T/out")" "r2"; eq "nothing moved" "$(head_)" "$LT"
fresh; loc l.txt 'l\n'; rc_ c1 a.txt 'a\n'; rc_ c2 b.txt 'b\n'
run; eq "two compatible diverged targets: 0" "$RC" 0; eq "two MERGED lines" "$(grep -c '^MERGED' "$T/out")" 2; eq "both files present" "$(ls "$M" | grep -cE '^(a|b)\.txt$')" 2
eq "two rows in commits.tsv" "$(wc -l < "$RUND/commits.tsv" | tr -d ' ')" 2
fresh; loc l.txt 'l\n'; rc_ c1 a.txt 'a\n'; git -C "$T/c2" pull -q origin main 2>/dev/null; git -C "$T/c2" pull -q "$T/b/r1.git" main; git -C "$T/c2" push -q origin main; rc_ c2 b.txt 'b\n'
run; eq "r2 descends from r1: one target" "$RC" 0; eq "one MERGED line" "$(grep -c '^MERGED' "$T/out")" 1
# 7 backup failure and an interrupted merge
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; : > "$RUND/backup"; LT="$(head_)"
run; eq "backup cannot be written: 20" "$RC" 20; eq "nothing merged" "$(head_)" "$LT"; has "backup_failed" "$(cat "$T/err")" backup_failed
rm -f "$RUND/backup"; fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; mkdir -p "$T/shim"; REALGIT="$(command -v git)"; LT="$(head_)"
printf '#!/bin/sh\ncase "$*" in *"bundle verify"*) exit 1 ;; esac\nexec "%s" "$@"\n' "$REALGIT" > "$T/shim/git"; chmod +x "$T/shim/git"
PATH="$T/shim:$PATH" run; eq "bundle that does not verify: 20" "$RC" 20; has "backup_failed" "$(cat "$T/err")" backup_failed; eq "nothing merged" "$(head_)" "$LT"
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; echo x > "$M/.git/MERGE_HEAD"; run; eq "MERGE_HEAD present: 20" "$RC" 20; has "merge_in_progress" "$(cat "$T/err")" merge_in_progress; rm -f "$M/.git/MERGE_HEAD"
# 8 refusals
fresh; for f in --rebase --reset --hard --force -f +x --merge; do run "$f"; eq "option $f: 20" "$RC" 20; has "option $f named force_refused" "$(cat "$T/err")" force_refused; done
"$H" --root "$M" --branch '-x' --run-dir "$RUND" >/dev/null 2>&1; eq "branch starting with a dash: 20" "$?" 20
"$H" --root "-x" --branch main --run-dir "$RUND" >/dev/null 2>&1; eq "root starting with a dash: 20" "$?" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --root -x --branch main --run-dir "$RUND" ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20
"$H" --root "$M" --branch main --run-dir "$T/norun" >/dev/null 2>&1; eq "run dir absent: 20" "$?" 20
run --adoption-commit zz; eq "adoption commit that is no sha: 20" "$RC" 20
run --bogus x; eq "unknown option: 20" "$RC" 20
git -C "$M" checkout -q -b other; run; eq "branch not checked out: 20" "$RC" 20; has "wrong_branch" "$(cat "$T/err")" wrong_branch; git -C "$M" checkout -q main
git -C "$M" config 'remote.-oevil.url' "$T/b/r1.git"; run; eq "remote named like an option: 20" "$RC" 20; git -C "$M" config --remove-section 'remote.-oevil'
# 9 --resolution refusals (no resolved merge can complete in this slice: the S2 secret fold is not built)
resfix() { # resfix: a conflict, then a resolution directory for it
  fresh; loc f.txt 'local\n'; rc_ c1 f.txt 'remote\n'; run; RTIP="$(tip r1)"; RD="$M/.audit/merge-resolution/20261005T000000Z-0-oldr"; mkdir -p "$RD"
  printf 'merged\n' > "$RD/f.txt"; mkres "$RTIP" f.txt opus xhigh; LT="$(head_)"
}
mkres() { # mkres <tip> <paths space separated> <model> <effort>
  local files='{}' p; for p in $2; do files="$(printf '%s' "$files" | jq -c --arg p "$p" --arg h "$(sha256sum "$RD/$p" | cut -d' ' -f1)" '.[$p]=$h')"; done
  jq -n --arg tip "$1" --arg m "$3" --arg e "$4" --argjson files "$files" --argjson paths "$(printf '%s\n' $2 | jq -R . | jq -sc .)" '{tip:$tip,paths:$paths,model:$m,effort:$e,files:$files}' > "$RD/resolution.json"
}
resfix; run --resolution "$RD"; eq "valid resolution: stops at the missing secret fold (20)" "$RC" 20; has "secret_fold_unavailable" "$(cat "$T/err")" secret_fold_unavailable
eq "nothing written, HEAD unchanged" "$(head_)" "$LT"; eq "no MERGE_HEAD left" "$(test -e "$M/.git/MERGE_HEAD"; echo $?)" 1; eq "work tree clean of the merge" "$(git -C "$M" status --porcelain | wc -l | tr -d ' ')" 0
resfix; rm -rf "$T/elsewhere"; cp -r "$RD" "$T/elsewhere"; run --resolution "$T/elsewhere"; eq "resolution dir outside .audit/merge-resolution: 20" "$RC" 20; has "resolution_dir_invalid" "$(cat "$T/err")" resolution_dir_invalid
resfix; mkdir -p "$M/.audit/merge-resolution/20261005T000000Z-0-empt"; run --resolution "$M/.audit/merge-resolution/20261005T000000Z-0-empt"; eq "no resolution.json: 20" "$RC" 20
resfix; rc_ c1 g.txt 'g\n'; run --resolution "$RD"; eq "tip no longer the newest live tip: 12" "$RC" 12; has "merge_target_moved" "$(cat "$T/out")" merge_target_moved
resfix; mkres "$RTIP" f.txt sonnet xhigh; run --resolution "$RD"; eq "model not Opus: 20" "$RC" 20; has "merge_resolver_not_pinned" "$(cat "$T/err")" merge_resolver_not_pinned
resfix; mkres "$RTIP" f.txt opus high; run --resolution "$RD"; eq "effort not xhigh: 20" "$RC" 20; has "merge_resolver_not_pinned (effort)" "$(cat "$T/err")" merge_resolver_not_pinned
resfix; printf 'x.txt\n' > /dev/null; echo y > "$RD/y.txt"; mkres "$RTIP" y.txt opus xhigh; run --resolution "$RD"; eq "conflict set differs from the record: 12" "$RC" 12; has "merge_conflict" "$(cat "$T/out")" merge_conflict
resfix; printf '<<<<<<< HEAD\nmerged\n=======\nx\n>>>>>>> r1\n' > "$RD/f.txt"; mkres "$RTIP" f.txt opus xhigh; run --resolution "$RD"; eq "conflict marker in a resolved file: 10" "$RC" 10
resfix; echo tampered > "$RD/f.txt"; run --resolution "$RD"; eq "resolved file differs from its recorded sha256: 20" "$RC" 20; has "resolution_invalid" "$(cat "$T/err")" resolution_invalid
resfix2() { # a conflict on a chained store
  fresh; printf 'd0\nlocal\n' > "$M/$EVR/deferrals.jsonl"; git -C "$M" add -A; git -C "$M" commit -q -m ld; rc_ c1 "$EVR/deferrals.jsonl" 'd0\nremote\n'; run; RTIP="$(tip r1)"
  RD="$M/.audit/merge-resolution/20261005T000000Z-0-oldr"; mkdir -p "$RD/$EVR"; }
resfix2; printf 'd0\nremote\nlocal\n' > "$RD/$EVR/deferrals.jsonl"; mkres "$RTIP" "$EVR/deferrals.jsonl" opus xhigh; run --resolution "$RD"
eq "store path without method re-recorded: 20" "$RC" 20; has "store_not_rerecorded" "$(cat "$T/err")" store_not_rerecorded
resfix2; printf 'd0\nlocal\nremote\n' > "$RD/$EVR/deferrals.jsonl"; mkres "$RTIP" "$EVR/deferrals.jsonl" opus xhigh
jq --arg p "$EVR/deferrals.jsonl" '.method[$p]="re-recorded"' "$RD/resolution.json" > "$RD/r.tmp" && mv "$RD/r.tmp" "$RD/resolution.json"; run --resolution "$RD"
eq "re-recorded store that does not extend the remote side byte for byte: 20" "$RC" 20; has "store_not_rerecorded" "$(cat "$T/err")" store_not_rerecorded
resfix2; printf 'd0\nremote\nlocal\n' > "$RD/$EVR/deferrals.jsonl"; mkres "$RTIP" "$EVR/deferrals.jsonl" opus xhigh
jq --arg p "$EVR/deferrals.jsonl" '.method[$p]="re-recorded"' "$RD/resolution.json" > "$RD/r.tmp" && mv "$RD/r.tmp" "$RD/resolution.json"; run --resolution "$RD"
has "re-recorded store extending the remote side passes the store check (then the fold gap)" "$(cat "$T/err")" secret_fold_unavailable

# (WF6 W6-12) the working-tree gate reads `git status` and `git diff --name-only` through a pipe: a failing git read as "nothing dirty" / "no overlap" and let the merge run
mkshim() { mkdir -p "$T/fshim"; local rg; rg="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in $FAILPAT) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg" > "$T/fshim/git"; chmod +x "$T/fshim/git"; }
mkshim
fresh; loc l.txt 'l\n'; rc_ c1 base.txt 'incoming\n'; echo mine >> "$M/base.txt"; LT="$(head_)"
PATH="$T/fshim:$PATH" FAILPAT='*diff --name-only*' run; eq "git diff --name-only failing (shim): 20, never a silent merge (W6-12)" "$RC" 20; has "named git_diff_failed" "$(cat "$T/err")" git_diff_failed; eq "HEAD unchanged when the overlap check cannot run" "$(head_)" "$LT"; eq "my change kept" "$(tail -1 "$M/base.txt")" mine
fresh; loc l.txt 'l\n'; rc_ c1 base.txt 'incoming\n'; echo mine >> "$M/base.txt"; LT="$(head_)"
PATH="$T/fshim:$PATH" FAILPAT='*status --porcelain=v1*' run; eq "git status failing (shim): 20 (W6-12)" "$RC" 20; has "named git_status_failed" "$(cat "$T/err")" git_status_failed; eq "HEAD unchanged when the dirty list cannot be read" "$(head_)" "$LT"
fresh; loc l.txt 'l\n'; rc_ c1 base.txt 'incoming\n'; echo mine >> "$M/base.txt"; run; eq "control, real git: the overlap is still 12" "$RC" 12
# (WF7 M-3) the remote list is read with its status: a failing `git remote` was "no remotes" (nothing_to_merge, exit 0) while a remote held a diverged tip
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; LT="$(head_)"
PATH="$T/fshim:$PATH" FAILPAT='* remote' run; eq "git remote failing (shim): 20, never nothing_to_merge (M-3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed; eq "HEAD unchanged when the remote list cannot be read" "$(head_)" "$LT"
fresh; loc l.txt 'l\n'; rc_ c1 r.txt 'r\n'; run; eq "control, real git: the diverged remote tip is merged (exit 0 with a MERGED row)" "$RC" 0; has "MERGED row" "$(cat "$T/out")" MERGED
fin
