#!/usr/bin/env bash
# WF7 round 7 test (TDD): the cases of the independent round-6 review that touch scope_check.sh (I-3: a broken `.git` entry that git walks past; M-2: a
# HEAD ref that does not resolve in a repository with history) and push_recursive.sh (I-4: a broken submodule skipped, main pushed).
# Throwaway repositories and LOCAL BARE REMOTES only. Usage: bash scripts/repo/tests/test_wp04g.sh   Env: SC=<scope_check path> PR=<push_recursive path>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; SC="${SC:-scripts/repo/scope_check.sh}"; PR="${PR:-scripts/repo/push_recursive.sh}"
case "$SC" in /*) ;; *) SC="$D0/$SC" ;; esac; case "$PR" in /*) ;; *) PR="$D0/$PR" ;; esac
H="$SC"; . "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04g_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
[ -x "$SC" ] && [ -x "$PR" ] || echo "NOTE: a helper is absent or not executable (RED state: every case must FAIL)"
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) scope_check=$(sha256sum "$SC" 2>/dev/null | cut -c1-64) push_recursive=$(sha256sum "$PR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
# ---- scope_check I-3: a LEGACY submodule (its .git is a directory) whose .git is broken, with a dirty file inside
git init -q -b main "$T/subsrc"; echo s > "$T/subsrc/s"; commit_all "$T/subsrc" s
mkrepo "$T/r"; echo a > "$T/r/a"; commit_all "$T/r" a
git clone -q "$T/subsrc" "$T/r/legacy" 2>/dev/null; ( cd "$T/r" && git submodule add -q "$T/subsrc" legacy 2>/dev/null; git commit -qm addlegacy )
printf 'a\n' > "$T/cs.lst"
scrun() { ( cd "$T/r" && "$SC" --root "$T/r" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; RC=$?; }
echo dirty > "$T/r/legacy/d.txt"
scrun; eq "I-3 control: a dirty legacy submodule: 13" "$RC" 13; has "I-3 control names the dirty file" "$(cat "$T/out")" dirty_submodule:d.txt
cp "$T/r/legacy/.git/HEAD" "$T/HEAD.bak"; : > "$T/r/legacy/.git/HEAD"
scrun; eq "I-3 a .git DIRECTORY with a truncated (empty) HEAD: 20, not a silent skip" "$RC" 20; has "I-3 named submodule_git_unreadable" "$(cat "$T/err")" submodule_git_unreadable
cp "$T/HEAD.bak" "$T/r/legacy/.git/HEAD"; scrun; eq "I-3 golden-false: the restored legacy submodule is judged again (13)" "$RC" 13
mv "$T/r/legacy/.git" "$T/gitdir.moved"; ln -s "$T/gitdir.gone" "$T/r/legacy/.git"
scrun; eq "I-3 a dangling .git SYMLINK: 20" "$RC" 20; has "I-3 dangling symlink named submodule_git_unreadable" "$(cat "$T/err")" submodule_git_unreadable
rm "$T/r/legacy/.git"; mv "$T/gitdir.moved" "$T/r/legacy/.git"; scrun; eq "I-3 golden-false: restored: 13" "$RC" 13
mv "$T/r/legacy/.git" "$T/gd2"; printf 'gitdir: %s\n' "$T/nowhere" > "$T/r/legacy/.git"; scrun; eq "I-3 a .git FILE to a missing dir (the round-6 case, still 20)" "$RC" 20
rm "$T/r/legacy/.git"; mv "$T/gd2" "$T/r/legacy/.git"
# an uninitialised submodule (no .git entry at all) is no failure
rm -rf "$T/r/legacy"; mkdir "$T/r/legacy"; scrun; eq "I-3 golden-false: an uninitialised submodule (empty directory) is no failure: 0" "$RC" 0
# ---- scope_check M-2: the HEAD ref does not resolve in a repository that HAS history; a rewritten append-only store must not read as a new one
mkrepo "$T/q"; mkdir -p "$T/q/ev"; printf 'l1\nl2\n' > "$T/q/ev/ledger.jsonl"; commit_all "$T/q" init; printf 'ev/ledger.jsonl\n' > "$T/cs2.lst"
qrun() { ( cd "$T/q" && "$SC" --root "$T/q" --ev ev --exceptions /dev/null --paths-from "$T/cs2.lst" ) >"$T/out" 2>"$T/err"; RC=$?; }
printf 'l1\nREWRITTEN\n' > "$T/q/ev/ledger.jsonl"
qrun; eq "M-2 control: a rewritten ledger: 13 append_only" "$RC" 13; has "M-2 control names append_only" "$(cat "$T/out")" append_only
sha="$(git -C "$T/q" rev-parse HEAD)"; : > "$T/q/.git/refs/heads/main"
qrun; eq "M-2 a zero-length branch ref: 20, the rewritten store is not a new one" "$RC" 20; has "M-2 zero-length ref named git_tree_unreadable" "$(cat "$T/err")" git_tree_unreadable
printf 'garbage\n' > "$T/q/.git/refs/heads/main"; qrun; eq "M-2 a garbage branch ref: 20" "$RC" 20
printf '%s\n' "$sha" > "$T/q/.git/refs/heads/main"; qrun; eq "M-2 golden-false: the ref restored: 13 again" "$RC" 13
git -C "$T/q" pack-refs --all; cp "$T/q/.git/packed-refs" "$T/pr.bak"; : > "$T/q/.git/packed-refs"
qrun; eq "M-2 an emptied packed-refs: 20" "$RC" 20; cp "$T/pr.bak" "$T/q/.git/packed-refs"; qrun; eq "M-2 golden-false: packed-refs restored: 13" "$RC" 13
# golden-false: a repository with no commit yet holds no store: nothing is refused for it
mkrepo "$T/z"; mkdir -p "$T/z/ev"; printf 'l1\n' > "$T/z/ev/ledger.jsonl"; printf 'ev/ledger.jsonl\n' > "$T/cs3.lst"
( cd "$T/z" && "$SC" --root "$T/z" --ev ev --exceptions /dev/null --paths-from "$T/cs3.lst" ) >"$T/out" 2>"$T/err"; RC=$?; eq "M-2 golden-false: a repository with no commit yet: 0" "$RC" 0
# ---- push_recursive I-4: a submodule whose .git is broken is skipped by `discover`; main (whose CPA commit moves the gitlink to an unpublished commit) was pushed
export GIT_CONFIG_COUNT=2
RID=20261005T000000Z-1-aaaa; A="$T/audit"; mkdir -p "$A/$RID" "$T/b"
git init -q --bare -b main "$T/b/r1.git"; git init -q --bare -b main "$T/b/s.git"
git clone -q "$T/b/s.git" "$T/seed" 2>/dev/null; ( cd "$T/seed" && git checkout -q -b main 2>/dev/null; echo s > s; git add s; git commit -qm s; git push -q origin main )
mkrepo "$T/m"; ( cd "$T/m" && echo base > base; git add base; git commit -qm init; git remote add r1 "$T/b/r1.git"; git push -q r1 main
  git submodule add -q "$T/b/s.git" sub 2>/dev/null; git commit -qm addsub; git push -q r1 main )
cpa() { local d="$1" k="$2" f="$3"; echo "$f $RANDOM" > "$d/$f"; git -C "$d" add -- "$f"; git -C "$d" commit -q -m "change $f" -m "CPA-Run: $RID"
  printf '%s\t%s\t%s\n' "$k" "$(git -C "$d" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"; }
( cd "$T/m/sub" && git checkout -q main 2>/dev/null ); cpa "$T/m/sub" sub x.txt
git -C "$T/m" add sub; git -C "$T/m" commit -q -m "bump sub" -m "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$T/m" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"
mainr1() { git -C "$T/b/r1.git" rev-parse main; }; M0="$(mainr1)"
prrun() { "$PR" --main-root "$T/m" --branch main --run-dir "$A/$RID" --recursive "$@" >"$T/out" 2>"$T/err"; RC=$?; }
mv "$T/m/.git/modules/sub" "$T/gd.moved"
prrun; eq "I-4 a submodule whose .git points at a missing git dir: 20" "$RC" 20; has "I-4 named submodule_git_unreadable" "$(cat "$T/err")" submodule_git_unreadable
eq "I-4 the main repository was NOT pushed" "$(mainr1)" "$M0"
mv "$T/gd.moved" "$T/m/.git/modules/sub"
# a legacy .git directory with a truncated HEAD, and a dangling symlink (own remote; every commit is a recorded CPA commit so that only the submodule decides)
git init -q --bare -b main "$T/b/r2.git"; mkrepo "$T/m2"; git -C "$T/m2" remote add r1 "$T/b/r2.git"
rcpa() { git -C "$T/m2" commit -q -m "$1" -m "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$T/m2" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"; }
echo base > "$T/m2/base"; git -C "$T/m2" add base; rcpa init
git clone -q "$T/b/s.git" "$T/m2/sub" 2>/dev/null; git -C "$T/m2" add sub 2>/dev/null; rcpa addsub
eq "I-4 fixture: sub is a gitlink of m2" "$(git -C "$T/m2" ls-files -s sub | cut -c1-6)" 160000
mv "$T/m2/sub/.git/HEAD" "$T/sub.HEAD"; : > "$T/m2/sub/.git/HEAD"
"$PR" --main-root "$T/m2" --branch main --run-dir "$A/$RID" --recursive >"$T/out" 2>"$T/err"; RC=$?; eq "I-4 a legacy .git directory with a truncated HEAD: 20" "$RC" 20; has "I-4 legacy named submodule_git_unreadable" "$(cat "$T/err")" submodule_git_unreadable
eq "I-4 legacy: the main repository was not pushed" "$(git -C "$T/b/r2.git" rev-parse -q --verify refs/heads/main >/dev/null 2>&1 && echo pushed || echo absent)" absent
mv "$T/m2/sub/.git" "$T/sub.gitdir"; ln -s "$T/nowhere" "$T/m2/sub/.git"; "$PR" --main-root "$T/m2" --branch main --run-dir "$A/$RID" --recursive >"$T/out" 2>"$T/err"; RC=$?; eq "I-4 a dangling .git symlink: 20" "$RC" 20
# golden-false: an UNINITIALISED submodule (empty directory) is skipped and the main repository is pushed
rm -f "$T/m2/sub/.git"; mv "$T/sub.gitdir" "$T/m2/sub/.git"; mv "$T/sub.HEAD" "$T/m2/sub/.git/HEAD"
"$PR" --main-root "$T/m2" --branch main --run-dir "$A/$RID" --recursive >"$T/out" 2>"$T/err"; RC=$?; eq "I-4 golden-false: the restored healthy submodule is pushed with main: 0" "$RC" 0
rm -rf "$T/m2/sub"; mkdir "$T/m2/sub"; echo more > "$T/m2/more"; git -C "$T/m2" add more; rcpa more
"$PR" --main-root "$T/m2" --branch main --run-dir "$A/$RID" --recursive >"$T/out" 2>"$T/err"; RC=$?
eq "I-4 golden-false: an uninitialised submodule (no .git entry) is skipped, main is pushed: 0" "$RC" 0; has "I-4 golden-false: main PUSHED" "$(cat "$T/out")" PUSHED
fin
