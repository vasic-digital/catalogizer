#!/usr/bin/env bash
# T041 test (TDD): scripts/repo/commit_recursive.sh - one repository, explicit paths, the CPA trailers, the commits.tsv row.
# Throwaway repositories only (no remote, no network). Usage: bash scripts/repo/tests/test_commit_recursive.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/commit_recursive.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/cr_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
ATTR='Co-Authored-By: Test Agent <test@example.invalid>'
fresh() { rm -rf "$T/w" "$T/run"; mkrepo "$T/w"; printf '/.audit/\n' > "$T/w/.gitignore"; echo base > "$T/w/base.txt"; commit_all "$T/w" init
  mkdir -p "$T/run"; RUND="$T/run"; }
fresh
run() { "$H" --repo "$T/w" --run-dir "$RUND" --run-id "${RID:-run1}" --message "${MSG:-subject line}" --attribution "$ATTR" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
body() { git -C "$T/w" log -1 --format=%B "${1:-HEAD}"; }
# 1 golden: explicit paths only, the trailers, the row
echo a > "$T/w/a.txt"; echo b > "$T/w/b.txt"; echo other > "$T/w/other.txt"; echo staged > "$T/w/staged.txt"; git -C "$T/w" add staged.txt
run -- a.txt b.txt; eq "commit of two explicit paths: 0" "$RC" 0
eq "exactly the declared paths are in the commit" "$(git -C "$T/w" show --name-only --format= HEAD | sort | tr '\n' ' ')" "a.txt b.txt "
eq "an undeclared staged file stays staged, uncommitted" "$(git -C "$T/w" status --porcelain -- staged.txt)" "A  staged.txt"
eq "an undeclared file stays untracked" "$(git -C "$T/w" status --porcelain -- other.txt)" "?? other.txt"
has "attribution trailer" "$(body)" "$ATTR"; has "CPA-Run trailer" "$(body)" "CPA-Run: run1"; has "subject kept" "$(body)" "subject line"
hasnot "no Deferred-Gates line without a deferral" "$(body)" "Deferred-Gates"
sha1="$(git -C "$T/w" rev-parse HEAD)"
eq "commits.tsv row (repository key, sha, run id)" "$(tail -n1 "$RUND/commits.tsv")" ".	$sha1	run1"
# 2 deferral flags from the run directory
"$D0/scripts/repo/record_deferral.sh" --run-dir "$RUND" --flag SKIP_LONG --reason t >/dev/null 2>&1 || true
echo c > "$T/w/c.txt"; run -- c.txt; eq "second commit: 0" "$RC" 0
has "Deferred-Gates line from the run directory" "$(body)" "Deferred-Gates: SKIP_LONG"
eq "two rows now" "$(wc -l < "$RUND/commits.tsv" | tr -d ' ')" 2
# 3 Foreign-Commit lines: first commit of the run in this repository only; only ancestors; none named twice
fresh; echo in > "$T/w/in.txt"; commit_all "$T/w" incoming; INC="$(git -C "$T/w" rev-parse HEAD)"
git -C "$T/w" checkout -q -b side HEAD~1; echo s > "$T/w/s.txt"; commit_all "$T/w" side; NOTANC="$(git -C "$T/w" rev-parse HEAD)"; git -C "$T/w" checkout -q main
printf '%s\n%s\n' "$INC" "$NOTANC" > "$T/foreign.lst"; echo n > "$T/w/n.txt"
run --foreign-from "$T/foreign.lst" -- n.txt; eq "first commit with foreign list: 0" "$RC" 0
has "ancestor named" "$(body)" "Foreign-Commit: $INC"; hasnot "non-ancestor not named" "$(body)" "$NOTANC"
echo m > "$T/w/m.txt"; run --foreign-from "$T/foreign.lst" -- m.txt; hasnot "second commit of the run names no foreign commit" "$(body)" "Foreign-Commit"
git -C "$T/w" checkout -q -b old "$INC"; echo o > "$T/w/o.txt"; commit_all "$T/w" "second incoming"; INC2="$(git -C "$T/w" rev-parse HEAD)"; git -C "$T/w" checkout -q main; git -C "$T/w" merge -q --no-ff -m "take old" old
printf '%s\n' "$INC2" > "$T/foreign2.lst"; echo p > "$T/w/p.txt"; run --foreign-from "$T/foreign2.lst" -- p.txt; hasnot "a later commit of the run does not name a new foreign commit either" "$(body)" "Foreign-Commit: $INC2"
fresh; echo in > "$T/w/in.txt"; commit_all "$T/w" incoming; INC="$(git -C "$T/w" rev-parse HEAD)"; printf '%s\n' "$INC" > "$T/foreign.lst"
echo n > "$T/w/n.txt"; RID=r1 run --foreign-from "$T/foreign.lst" -- n.txt; mkdir -p "$T/run2"; echo m > "$T/w/m.txt"
RUND="$T/run2" RID=r2 run --foreign-from "$T/foreign.lst" -- m.txt; hasnot "a commit already named by an earlier Foreign-Commit line is not named again" "$(body)" "Foreign-Commit"
printf 'zzzz\n' > "$T/foreign.lst"; echo x > "$T/w/x.txt"; run --foreign-from "$T/foreign.lst" -- x.txt; eq "a foreign entry that is no sha: 20" "$RC" 20
# 4 held and unheld paths: the unheld commit first, one held commit per verdict, held commits carry Awaits-Review and no foreign lines
fresh; echo u > "$T/w/u.txt"; echo h1 > "$T/w/h1.txt"; echo h2 > "$T/w/h2.txt"
printf 'h1.txt\tspecs/ev/reviews/V1.json\nh2.txt\tspecs/ev/reviews/V2.json\n' > "$T/held.tsv"
run --held-from "$T/held.tsv" -- h1.txt u.txt h2.txt; eq "held and unheld paths: 0" "$RC" 0
eq "three commits made" "$(git -C "$T/w" rev-list --count HEAD)" 4
eq "first commit holds the unheld path" "$(git -C "$T/w" show --name-only --format= HEAD~2)" "u.txt"
has "second commit Awaits-Review V1" "$(body HEAD~1)" "Awaits-Review: specs/ev/reviews/V1.json"; has "third commit Awaits-Review V2" "$(body HEAD)" "Awaits-Review: specs/ev/reviews/V2.json"
hasnot "unheld commit has no Awaits-Review" "$(body HEAD~2)" "Awaits-Review"
eq "three rows in commit order" "$(cut -f2 "$RUND/commits.tsv" | tr '\n' ' ')" "$(git -C "$T/w" rev-list --reverse HEAD~3..HEAD | tr '\n' ' ')"
# 5 submodules: never a commit inside one; a pin move of the submodule path itself is an ordinary path of this repository
fresh; mkrepo "$T/sub"; echo s > "$T/sub/f"; commit_all "$T/sub" s
( cd "$T/w" && git submodule -q add "$T/sub" mods/sub && git commit -qm sub ); subhead="$(git -C "$T/w/mods/sub" rev-parse HEAD)"
echo y > "$T/w/mods/sub/new.txt"; run -- mods/sub/new.txt; eq "path inside a submodule: 20" "$RC" 20; has "reason path_in_submodule" "$(cat "$T/err")$(cat "$T/out")" "path_in_submodule"
eq "the submodule HEAD is unchanged" "$(git -C "$T/w/mods/sub" rev-parse HEAD)" "$subhead"
rm -f "$T/w/mods/sub/new.txt"; ( cd "$T/w/mods/sub" && echo z > z && git add z && git commit -qm z ); run -- mods/sub; eq "the gitlink path itself (a pin move): 0" "$RC" 0
# 6 refusals (20), each leaving no row
fresh; echo a > "$T/w/a.txt"; n0=0
echo real > "$T/w/-n"; mkdir -p "$T/w/a"; echo real > "$T/w/b"; echo real > "$T/x"
for badp in '../x' '/abs/x' '-n' 'a/../b'; do run -- "$badp"; eq "unsafe path '$badp': 20" "$RC" 20; done
run -- nonexistent.txt; eq "path that is neither tracked nor present: 20" "$RC" 20
RID='a b' run -- a.txt; eq "run id with a blank: 20" "$RC" 20
RID='-x' run -- a.txt; eq "run id starting with a dash: 20" "$RC" 20
"$H" --repo "$T/w" --run-dir "$RUND" --run-id r --attribution "$ATTR" -- a.txt >/dev/null 2>&1; eq "no --message: 20" "$?" 20
"$H" --repo "$T/w" --run-dir "$RUND" --run-id r --message m --attribution $'x\nCPA-Run: forged' -- a.txt >/dev/null 2>&1; eq "newline in the attribution trailer: 20" "$?" 20
"$H" --repo "$T/w" --run-dir "$T/norun" --run-id r --message m -- a.txt >/dev/null 2>&1; eq "run directory absent: 20" "$?" 20; eq "no commit made when the run directory is absent" "$(git -C "$T/w" rev-list --count HEAD)" 1
"$H" --repo "$T/plain_nonrepo" --run-dir "$RUND" --run-id r --message m -- a.txt >/dev/null 2>&1; eq "not a repository: 20" "$?" 20
"$H" --repo "-x" --run-dir "$RUND" --run-id r --message m -- a.txt >/dev/null 2>&1; eq "repository path starting with a dash: 20" "$?" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --repo -x --run-dir "$RUND" --run-id r --message m -- a.txt ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20
run --bogus; eq "unknown option: 20" "$RC" 20
run; eq "no paths: 20" "$RC" 20
printf 'a.txt\t../../etc/x\n' > "$T/held.tsv"; run --held-from "$T/held.tsv" -- a.txt; eq "unsafe verdict path in the held table: 20" "$RC" 20
eq "no commit was made by an unsafe path" "$(git -C "$T/w" rev-list --count HEAD)" 1
eq "no row was written by any refusal" "$(cat "$RUND/commits.tsv" 2>/dev/null | wc -l | tr -d ' ')" 0
eq "no commit was made by any refusal" "$(git -C "$T/w" rev-list --count HEAD)" 1
# 7 a commit refused by a repository hook: 20 and no row
printf '#!/bin/sh\nexit 1\n' > "$T/w/.git/hooks/pre-commit"; chmod +x "$T/w/.git/hooks/pre-commit"
run -- a.txt; eq "commit refused by a hook: 20" "$RC" 20; eq "no row after a refused commit" "$(cat "$RUND/commits.tsv" 2>/dev/null | wc -l | tr -d ' ')" 0
rm -f "$T/w/.git/hooks/pre-commit"
# 8 SIGKILL after the first commit leaves that commit's row
fresh; echo u > "$T/w/u.txt"; echo h > "$T/w/h.txt"; printf 'h.txt\tspecs/ev/reviews/V1.json\n' > "$T/held.tsv"
printf '#!/bin/sh\nif [ "$(git rev-list --count HEAD)" -ge 2 ]; then kill -9 "$CPA_HELPER_PID"; exit 1; fi\nexit 0\n' > "$T/w/.git/hooks/pre-commit"; chmod +x "$T/w/.git/hooks/pre-commit"
{ run --held-from "$T/held.tsv" -- u.txt h.txt; } 2>/dev/null; first="$(git -C "$T/w" rev-parse HEAD)"
eq "the helper was killed (137) after its first commit" "$RC" 137
eq "the first commit exists" "$(git -C "$T/w" rev-list --count HEAD)" 2
eq "its row survives the kill" "$(tail -n1 "$RUND/commits.tsv" | cut -f2)" "$first"
# a git shim that FAILS (rc 128) when the argument list matches a glob, else runs the real git: a failing listing must never read as clean
mkshim() { mkdir -p "$T/fshim"; local rg; rg="$(command -v git)"; printf '#!/bin/sh\ncase "$*" in $FAILPAT) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$rg" > "$T/fshim/git"; chmod +x "$T/fshim/git"; }
# 8 (WF5 F3) a failing gitlink listing must not be read as "no submodules": 20, nothing committed
fresh; mkshim; echo z > "$T/w/z.txt"; hb="$(git -C "$T/w" rev-parse HEAD)"
PATH="$T/fshim:$PATH" FAILPAT='*ls-files -s -z*' run -- z.txt; eq "gitlink listing failing: 20 (F3)" "$RC" 20; has "named git_listing_failed" "$(cat "$T/err")" git_listing_failed
eq "no commit made" "$(git -C "$T/w" rev-parse HEAD)" "$hb"
fin
