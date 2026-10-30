#!/usr/bin/env bash
# shellcheck disable=SC2034  # variables are read by the eval in chk()
# Fixture test for scripts/check_submodule_ff.sh (gitlink fast-forward gate).
# Every case builds REAL throwaway git repositories with REAL submodules in a mktemp
# directory (no mocks, constitution 11.4.289) and runs the real gate against them.
# Exit 0 = every case passed; 1 = at least one case failed.
set -uo pipefail
HERE="$(cd "$(dirname "$0")/.." && pwd)"
GATE="$HERE/check_submodule_ff.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/subff_test.XXXXXX")"
trap 'rm -rf "${T:?}"' EXIT
export GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_0=protocol.file.allow GIT_CONFIG_VALUE_0=always \
  GIT_CONFIG_KEY_1=core.hooksPath GIT_CONFIG_VALUE_1=/dev/null GIT_CONFIG_KEY_2=commit.gpgsign GIT_CONFIG_VALUE_2=false
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@t GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@t
fail=0
chk() { if eval "$2"; then echo "PASS $1"; else echo "FAIL $1"; fail=1; fi; }

# commit helper: one empty commit in repo $1 with message $2, prints its sha
c() { git -C "$1" commit -q --allow-empty -m "$2" && git -C "$1" rev-parse HEAD; }

# Build: submodule repo $T/<name>_src with linear history s1 <- s2 <- s3 and a side branch x1 off s1.
mksub() {
  local d="$T/$1_src"
  git init -q -b main "$d"
  S1=$(c "$d" s1); S2=$(c "$d" s2); S3=$(c "$d" s3)
  git -C "$d" checkout -q -b side "$S1"; X1=$(c "$d" x1); git -C "$d" checkout -q main
}

# Build a parent repo $T/$1 with submodule at path "sub" pinned to $2, .gitmodules url $3.
mkparent() {
  local p="$T/$1"
  git init -q -b main "$p"
  c "$p" root >/dev/null
  git -C "$p" submodule add -q "$T/${4:-sub}_src" sub
  git -C "$p/sub" checkout -q "$2"
  git -C "$p" config -f .gitmodules submodule.sub.url "$3"
  git -C "$p" add .gitmodules sub
  git -C "$p" commit -q -m "pin sub"
  git -C "$p" rev-parse HEAD
}
# move the sub pin in parent $1 to $2, commit, print the commit sha
move() { git -C "$T/$1/sub" checkout -q "$2"; git -C "$T/$1" add sub; git -C "$T/$1" commit -q -m "move sub"; git -C "$T/$1" rev-parse HEAD; }

OWN_URL='git@github.com:vasic-digital/sub.git'
TP_URL='https://github.com/somevendor/sub.git'
mksub sub

run() { out="$(cd "$T/$1" && nice -n 15 bash "$GATE" "$2" "$3" 2>&1)"; rc=$?; }

# 1. own-org forward move: PASS (exit 0), FF reported
B=$(mkparent fwd "$S1" "$OWN_URL"); E=$(move fwd "$S3")
run fwd "$B" "$E"
chk "forward own-org exits 0" '[ "$rc" -eq 0 ]'
chk "forward own-org reported FF" 'printf "%s\n" "$out" | grep -q "^FF own sub $S1 $S3"'
chk "forward own-org verdict PASS" 'printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'

# 2. own-org backward move: FAIL (exit 1)
B=$(mkparent back "$S3" "$OWN_URL"); E=$(move back "$S1")
run back "$B" "$E"
chk "backward own-org exits 1" '[ "$rc" -eq 1 ]'
chk "backward own-org reported NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'
chk "backward own-org verdict FAIL" 'printf "%s\n" "$out" | grep -q "^VERDICT: FAIL"'

# 3. own-org sideways move (diverged: neither is an ancestor): FAIL
B=$(mkparent side "$S2" "$OWN_URL"); E=$(move side "$X1")
run side "$B" "$E"
chk "sideways own-org exits 1" '[ "$rc" -eq 1 ]'
chk "sideways own-org reported NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S2 $X1"'

# own-org anchor: a .gitmodules entry with an own-org URL, so the own-org list matches something
# in the range (C1: a list matching nothing while pins move is BLIND, see fixture 38)
anchor() {
  git -C "$T/$1" config -f .gitmodules submodule.anchor.path anchor
  git -C "$T/$1" config -f .gitmodules submodule.anchor.url "$OWN_URL"
  git -C "$T/$1" add .gitmodules; git -C "$T/$1" commit -q -m anchor; git -C "$T/$1" rev-parse HEAD
}

# 4. third-party backward move: reported separately, does not fail
mkparent tp "$S3" "$TP_URL" >/dev/null; B=$(anchor tp); E=$(move tp "$S1")
run tp "$B" "$E"
chk "third-party backward exits 0" '[ "$rc" -eq 0 ]'
chk "third-party backward reported as third-party" 'printf "%s\n" "$out" | grep -q "^NONFF third-party sub $S3 $S1"'
chk "third-party not counted as own NONFF" '! printf "%s\n" "$out" | grep -q "^NONFF own "'
chk "summary counts third-party separately" 'printf "%s\n" "$out" | grep -q "own_nonff=0 own_blind=0 third_party_ff=0 third_party_nonff=2"'

# 5. per-commit check: back then forward again (net forward) still FAILs
B=$(mkparent zig "$S2" "$OWN_URL"); move zig "$S1" >/dev/null; E=$(move zig "$S3")
run zig "$B" "$E"
chk "zig-zag range exits 1" '[ "$rc" -eq 1 ]'
chk "zig-zag backward step reported" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S2 $S1"'
chk "zig-zag forward step reported" 'printf "%s\n" "$out" | grep -q "^FF own sub $S1 $S3"'

# 6. absent submodule objects: BLIND (exit 2), never PASS
B=$(mkparent blind "$S1" "$OWN_URL"); E=$(move blind "$S3")
git clone -q --no-recurse-submodules "$T/blind" "$T/blind_clone"
run blind_clone "$B" "$E"
chk "absent objects exits 2" '[ "$rc" -eq 2 ]'
chk "absent objects reported BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $S1 $S3 .*reason=pin-not-in-local-object-store"'
chk "absent objects verdict BLIND" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND"'
chk "absent objects never PASS" '! printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'

# 7. old commit missing from the submodule store (new present): BLIND
git init -q -b main "$T/orph_src"; c "$T/orph_src" o1 >/dev/null; O2=$(c "$T/orph_src" o2)
B=$(mkparent orph "$O2" "$OWN_URL" orph)
# point the gitlink at a commit that exists nowhere (fabricated id), then forward to O2
FAKE=0123456789abcdef0123456789abcdef01234567
git -C "$T/orph" update-index --cacheinfo "160000,$FAKE,sub"; git -C "$T/orph" commit -q -m fake
E=$(move orph "$O2")
F=$(git -C "$T/orph" rev-parse HEAD~1)
run orph "$F" "$E"
chk "missing old commit exits 2" '[ "$rc" -eq 2 ]'
chk "missing old commit reported BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $FAKE $O2"'

# 8. range without gitlink moves: PASS with zero checked
E=$(c "$T/fwd" noop)
P=$(git -C "$T/fwd" rev-parse HEAD~1)
run fwd "$P" "$E"
chk "no gitlink moves exits 0" '[ "$rc" -eq 0 ]'
chk "no gitlink moves verdict PASS checked=0" 'printf "%s\n" "$out" | grep -q "^VERDICT: PASS.*checked=0"'

# 9. added and removed submodules are reported, not failed
git init -q -b main "$T/addrm"; A0=$(c "$T/addrm" root)
git -C "$T/addrm" submodule add -q "$T/sub_src" sub; git -C "$T/addrm" commit -q -m add
git -C "$T/addrm" rm -q sub; git -C "$T/addrm" commit -q -m rm; A2=$(git -C "$T/addrm" rev-parse HEAD)
run addrm "$A0" "$A2"
chk "add/remove exits 0" '[ "$rc" -eq 0 ]'
chk "add reported" 'printf "%s\n" "$out" | grep -q "^ADDED .* sub "'
chk "remove reported" 'printf "%s\n" "$out" | grep -q "^REMOVED .* sub "'

# 10. control needle: the gate proves on every run that it sees a forward and a backward move
run fwd HEAD~1 HEAD
chk "control needle forward seen" 'printf "%s\n" "$out" | grep -q "^NEEDLE forward=FF"'
chk "control needle backward seen" 'printf "%s\n" "$out" | grep -q "^NEEDLE forward=FF backward=NONFF class=own$"'
sout="$(nice -n 15 bash "$GATE" --selftest 2>&1)"; src=$?
chk "selftest exits 0" '[ "$src" -eq 0 ]'
chk "selftest reports needle ok" 'printf "%s\n" "$sout" | grep -q "^NEEDLE ok"'

# 10b. blind instrument: a git whose ancestry query answers "not an ancestor" for everything
# (gate-logic test of the needle, constitution 11.4.289(C)(2): the wrapper replaces only the
# merge-base answer to simulate a broken instrument; the real gate must refuse, never PASS)
mkdir -p "$T/blindgit"; REALGIT="$(command -v git)"
printf '#!/bin/sh\nfor a in "$@"; do [ "$a" = merge-base ] && exit 1; done\nexec "%s" "$@"\n' "$REALGIT" > "$T/blindgit/git"
chmod +x "$T/blindgit/git"
out="$(cd "$T/fwd" && PATH="$T/blindgit:$PATH" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "blind instrument exits 2" '[ "$rc" -eq 2 ]'
chk "blind instrument reports needle failure" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND reason=control-needle-failed"'
chk "blind instrument never PASS" '! printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'

# 11. accepts OLD..NEW range syntax
B=$(git -C "$T/back" rev-parse HEAD~1); E=$(git -C "$T/back" rev-parse HEAD)
out="$(cd "$T/back" && bash "$GATE" "$B..$E" 2>&1)"; rc=$?
chk "range syntax backward exits 1" '[ "$rc" -eq 1 ]'

# 13. path and name with a space and a non-ASCII byte: parsed verbatim (diff-tree -z) and the
# .gitmodules name lookup works (F7/F8: proven with a third-party URL, which is only honoured if
# the name -> url lookup succeeds; a failed lookup falls back to own and would FAIL)
odd() {  # $1 = repo dir, $2 = url
  git init -q -b main "$T/$1"; c "$T/$1" root >/dev/null
  git -C "$T/$1" submodule add -q "$T/sub_src" "my süb"
  git -C "$T/$1" config -f .gitmodules "submodule.my süb.url" "$2"
  git -C "$T/$1/my süb" checkout -q "$S3"; git -C "$T/$1" add .gitmodules "my süb"; git -C "$T/$1" commit -q -m pin
  anchor "$1" >/dev/null
  git -C "$T/$1/my süb" checkout -q "$S1"; git -C "$T/$1" add "my süb"; git -C "$T/$1" commit -q -m back
}
odd odd "$OWN_URL"
run odd HEAD~1 HEAD
chk "odd path own backward exits 1" '[ "$rc" -eq 1 ]'
chk "odd path reported verbatim" 'printf "%s\n" "$out" | grep -qF "NONFF own my süb $S3 $S1"'
odd odd_tp "$TP_URL"
run odd_tp HEAD~1 HEAD
chk "odd name third-party URL found (exit 0)" '[ "$rc" -eq 0 ]'
chk "odd name classed third-party" 'printf "%s\n" "$out" | grep -qF "NONFF third-party my süb $S3 $S1"'

# 14. F1: a merge where the local side is the FIRST parent must not drop the upstream advance.
# base S1; upstream S1->S3; local S1->X1 (X1 descends from S1); merge upstream keeping X1.
mkparent mrg "$S1" "$OWN_URL" >/dev/null; MB=$(git -C "$T/mrg" rev-parse HEAD)
git -C "$T/mrg" checkout -q -b upstream; MU=$(move mrg "$S3")
git -C "$T/mrg" checkout -q main; git -C "$T/mrg" submodule update -q
move mrg "$X1" >/dev/null
git -C "$T/mrg" merge -q -s ours upstream -m merge; MM=$(git -C "$T/mrg" rev-parse HEAD)
run mrg "$MB" "$MM"
chk "F1 merge keeping local side exits 1" '[ "$rc" -eq 1 ]'
chk "F1 merge upstream advance reported NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $X1"'
run mrg "$MU" "$MM"
chk "F1 upstream..HEAD exits 1" '[ "$rc" -eq 1 ]'
chk "F1 upstream..HEAD net endpoint NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $X1 commit=RANGE"'

# 15. F2: a git error must never read as PASS
cp -a "$T/back" "$T/back_corrupt"
TREE=$(git -C "$T/back_corrupt" rev-parse "HEAD^{tree}")
rm -f "${T:?}/back_corrupt/.git/objects/${TREE:0:2}/${TREE:2}"
run back_corrupt HEAD~1 HEAD
chk "F2 missing tree object exits 2" '[ "$rc" -eq 2 ]'
chk "F2 missing tree object never PASS" '! printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'
chk "F2 missing tree object reason git-error" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=git-error"'
cp -a "$T/zig" "$T/zig_corrupt"
ZO=$(git -C "$T/zig_corrupt" rev-parse HEAD~2); ZN=$(git -C "$T/zig_corrupt" rev-parse HEAD); ZM=$(git -C "$T/zig_corrupt" rev-parse HEAD~1)
rm -f "${T:?}/zig_corrupt/.git/objects/${ZM:0:2}/${ZM:2}"
run zig_corrupt "$ZO" "$ZN"
chk "F2 missing commit in range exits 2" '[ "$rc" -eq 2 ]'
chk "F2 missing commit in range never PASS" '! printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'

# 16. F3: renaming the submodule path AND moving the pin backward in one commit FAILs
mkparent ren "$S3" "$OWN_URL" >/dev/null
git -C "$T/ren" mv sub sub2; git -C "$T/ren/sub2" checkout -q "$S1"; git -C "$T/ren" add sub2 .gitmodules
git -C "$T/ren" commit -q -m "rename and move back"
run ren HEAD~1 HEAD
chk "F3 rename+backward exits 1" '[ "$rc" -eq 1 ]'
chk "F3 rename+backward reported as a move" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub2 $S3 $S1"'

# 17. F4: URL switched between own and third-party with a backward move counts as own (either side)
mkparent sw1 "$S3" "$OWN_URL" >/dev/null
git -C "$T/sw1" config -f .gitmodules submodule.sub.url "$TP_URL"; git -C "$T/sw1/sub" checkout -q "$S1"
git -C "$T/sw1" add .gitmodules sub; git -C "$T/sw1" commit -q -m switch
run sw1 HEAD~1 HEAD
chk "F4 own->third-party backward exits 1" '[ "$rc" -eq 1 ]'
mkparent sw2 "$S3" "$TP_URL" >/dev/null
git -C "$T/sw2" config -f .gitmodules submodule.sub.url "$OWN_URL"; git -C "$T/sw2/sub" checkout -q "$S1"
git -C "$T/sw2" add .gitmodules sub; git -C "$T/sw2" commit -q -m switch
run sw2 HEAD~1 HEAD
chk "F4 third-party->own backward exits 1" '[ "$rc" -eq 1 ]'

# 18. F5: URLs without a trustworthy organisation are own (fail closed)
for u in 'https://github.com/sub.git' '/srv/git/vendor/sub.git' './sub' 'file:///srv/vendor/sub.git'; do
  d="f5_$(printf '%s' "$u" | tr -c 'a-z0-9' '_')"
  mkparent "$d" "$S3" "$u" >/dev/null; move "$d" "$S1" >/dev/null
  run "$d" HEAD~1 HEAD
  chk "F5 url <$u> backward is own and exits 1" '[ "$rc" -eq 1 ]'
done

# 19. F6: a shallow submodule store cannot decide ancestry: BLIND, never NONFF or FF
mkparent shal "$S1" "$OWN_URL" >/dev/null; move shal "$S3" >/dev/null
git -C "$T/shal/sub" fetch -q --depth 1 "file://$T/sub_src" main
run shal HEAD~1 HEAD
chk "F6 shallow store exits 2" '[ "$rc" -eq 2 ]'
chk "F6 shallow store reason" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $S1 $S3 .*reason=shallow-store"'

# 20. missing NEW pin alone (old present): BLIND
mkparent nnew "$S1" "$OWN_URL" >/dev/null
git -C "$T/nnew" update-index --cacheinfo "160000,$FAKE,sub"; git -C "$T/nnew" commit -q -m fake
run nnew HEAD~1 HEAD
chk "missing new pin exits 2" '[ "$rc" -eq 2 ]'
chk "missing new pin reported BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $S1 $FAKE .*reason=pin-not-in-local-object-store"'

# 21. merge-base error (both pins present, an intermediate commit missing): BLIND own, exit 2
mkparent mbe "$S1" "$OWN_URL" >/dev/null; move mbe "$S3" >/dev/null
MD=$(git -C "$T/mbe" rev-parse --path-format=absolute --git-common-dir)/modules/sub
rm -f "${MD:?}/objects/${S2:0:2}/${S2:2}"
run mbe HEAD~1 HEAD
chk "merge-base error exits 2" '[ "$rc" -eq 2 ]'
chk "merge-base error reported BLIND own" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $S1 $S3 .*reason=merge-base-rc-"'

# 22. needle must require exactly two lines: a git whose rev-list repeats its last commit
# (gate-logic test, 11.4.289(C)(2)) makes the needle print three lines -> BLIND
mkdir -p "$T/dupgit"
printf '#!/bin/sh\nfor a in "$@"; do if [ "$a" = rev-list ]; then o=$("%s" "$@") || exit $?; printf "%%s\\n" "$o"; printf "%%s\\n" "$o" | tail -n 1; exit 0; fi; done\nexec "%s" "$@"\n' "$REALGIT" "$REALGIT" > "$T/dupgit/git"
chmod +x "$T/dupgit/git"
out="$(cd "$T/fwd" && PATH="$T/dupgit:$PATH" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "needle with extra line exits 2" '[ "$rc" -eq 2 ]'
chk "needle with extra line reports control-needle-failed" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND reason=control-needle-failed"'

# 23. N1: inherited GIT_DIR / GIT_WORK_TREE / GIT_INDEX_FILE must not redirect the needle or
# the scan; the scanned repository (refs, objects, index) is left exactly as it was
state() { (cd "$T/$1" && env -u GIT_DIR -u GIT_WORK_TREE -u GIT_INDEX_FILE sh -c \
  'git for-each-ref --format="%(refname) %(objectname)"; git rev-list --all --count; sha256sum .git/index 2>/dev/null || sha256sum "$(git rev-parse --git-dir)/index"'); }
before="$(state back)"
out="$(cd "$T/back" && GIT_DIR="$T/back/.git" GIT_WORK_TREE="$T/back" GIT_INDEX_FILE="$T/back/.git/index" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
after="$(state back)"
chk "N1 exported GIT_DIR backward exits 1" '[ "$rc" -eq 1 ]'
chk "N1 exported GIT_DIR needle ok" 'printf "%s\n" "$out" | grep -q "^NEEDLE ok"'
chk "N1 exported GIT_DIR leaves repo unchanged" '[ "$before" = "$after" ]'
git -C "$T/back" worktree add -q "$T/back_wt" -b wt
before="$(state back)"; wtc0=$(git -C "$T/back_wt" rev-list --count wt)
out="$(git -C "$T/back_wt" -c alias.ff="!bash $GATE" ff HEAD~1 HEAD 2>&1)"; rc=$?
after="$(state back)"; wtc1=$(git -C "$T/back_wt" rev-list --count wt)
chk "N1 worktree alias backward exits 1" '[ "$rc" -eq 1 ]'
chk "N1 worktree alias needle ok" 'printf "%s\n" "$out" | grep -q "^NEEDLE ok"'
chk "N1 worktree alias wrote no commits" '[ "$wtc0" = "$wtc1" ] && [ "$before" = "$after" ]'

# 24. N2: rename that also changes the .gitmodules name (git rm sub; git submodule add URL lib)
# at an older pin: paired by normalised URL, reported as one own move, FAIL
rn() {  # $1 dir, $2 url written for lib
  mkparent "$1" "$S3" "$OWN_URL" >/dev/null
  git -C "$T/$1" rm -q sub
  git -C "$T/$1" submodule add -q "$T/sub_src" lib
  git -C "$T/$1" config -f .gitmodules submodule.lib.url "$2"
  git -C "$T/$1/lib" checkout -q "$S1"; git -C "$T/$1" add .gitmodules lib; git -C "$T/$1" commit -q -m "rename+rename"
}
rn rn2 "$OWN_URL"
run rn2 HEAD~1 HEAD
chk "N2 renamed name+path backward exits 1" '[ "$rc" -eq 1 ]'
chk "N2 renamed name+path reported as one move" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>lib $S3 $S1"'
rn rn3 'git@github.com:vasic-digital/sub/'
run rn3 HEAD~1 HEAD
chk "N2 URL normalised (.git and trailing slash) exits 1" '[ "$rc" -eq 1 ]'

# 25. N3: replace refs and grafts must not fake ancestry
mkparent rpl "$S3" "$OWN_URL" >/dev/null; move rpl "$S1" >/dev/null
git -C "$T/rpl/sub" replace --graft "$S1" "$S3"
run rpl HEAD~1 HEAD
chk "N3 replace --graft ignored, backward exits 1" '[ "$rc" -eq 1 ]'
chk "N3 replace --graft ignored, NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'
mkparent grf "$S3" "$OWN_URL" >/dev/null; move grf "$S1" >/dev/null
GD=$(git -C "$T/grf/sub" rev-parse --path-format=absolute --git-common-dir)
mkdir -p "$GD/info"; printf '%s %s\n' "$S1" "$S3" > "$GD/info/grafts"
run grf HEAD~1 HEAD
chk "N3 grafts in store exits 2" '[ "$rc" -eq 2 ]'
chk "N3 grafts in store BLIND reason" 'printf "%s\n" "$out" | grep -q "^BLIND own sub $S3 $S1 .*reason=grafts"'
cp -a "$T/fwd" "$T/pgrf"; mkdir -p "$T/pgrf/.git/info"
printf '%s\n' "$(git -C "$T/pgrf" rev-parse HEAD)" > "$T/pgrf/.git/info/grafts"
run pgrf HEAD~1 HEAD
chk "N3 grafts in parent exits 2" '[ "$rc" -eq 2 ]'
chk "N3 grafts in parent reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=parent-grafts"'

# 26. N4: URL switched to third-party for one commit cannot hide a backward move
SB=$(mkparent sw3 "$S3" "$OWN_URL")
git -C "$T/sw3" config -f .gitmodules submodule.sub.url "$TP_URL"; git -C "$T/sw3" add .gitmodules; git -C "$T/sw3" commit -q -m tp
move sw3 "$S1" >/dev/null
git -C "$T/sw3" config -f .gitmodules submodule.sub.url "$OWN_URL"; git -C "$T/sw3/sub" checkout -q "$S3"
git -C "$T/sw3" add .gitmodules sub; git -C "$T/sw3" commit -q -m back-to-own
run sw3 "$SB" HEAD
chk "N4 brief third-party URL backward exits 1" '[ "$rc" -eq 1 ]'
chk "N4 brief third-party URL move classed own" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'

# 26b. N4: own URL only at NEW (third-party at OLD and at both sides of the backward commit)
SB=$(mkparent sw4 "$S3" "$TP_URL")
move sw4 "$S1" >/dev/null
git -C "$T/sw4" config -f .gitmodules submodule.sub.url "$OWN_URL"; git -C "$T/sw4/sub" checkout -q "$S3"
git -C "$T/sw4" add .gitmodules sub; git -C "$T/sw4" commit -q -m own-at-new
run sw4 "$SB" HEAD
chk "N4 own URL only at NEW, backward exits 1" '[ "$rc" -eq 1 ]'

# 26c. N1: an exported GIT_DIR that the work tree does not resolve to is refused (exit 64)
out="$(cd "$T/fwd" && GIT_DIR="$T/back/.git" GIT_WORK_TREE="$T/fwd" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "N1 mismatched GIT_DIR/GIT_WORK_TREE exits 64" '[ "$rc" -eq 64 ]'

# 27. OM1: own-org list match is case-insensitive (HelixDevelopment)
mkparent hx "$S3" 'git@github.com:HelixDevelopment/sub.git' >/dev/null; move hx "$S1" >/dev/null
run hx HEAD~1 HEAD
chk "OM1 HelixDevelopment URL is own, exits 1" '[ "$rc" -eq 1 ]'

# 28. OM2: a rename paired by name whose URL goes own -> third-party stays own
mkparent rnt "$S3" "$OWN_URL" >/dev/null
git -C "$T/rnt" mv sub sub2; git -C "$T/rnt" config -f .gitmodules submodule.sub.url "$TP_URL"
git -C "$T/rnt/sub2" checkout -q "$S1"; git -C "$T/rnt" add sub2 .gitmodules; git -C "$T/rnt" commit -q -m rnt
run rnt HEAD~1 HEAD
chk "OM2 rename own->third-party backward exits 1" '[ "$rc" -eq 1 ]'

# 29. OM3: gitlinks with no .gitmodules entry (empty name and URL) are not paired
git init -q -b main "$T/nogm"; c "$T/nogm" root >/dev/null
git -C "$T/nogm" update-index --add --cacheinfo "160000,$S3,x"; git -C "$T/nogm" commit -q -m x
git -C "$T/nogm" rm -q --cached x; git -C "$T/nogm" update-index --add --cacheinfo "160000,$S1,y"; git -C "$T/nogm" commit -q -m y
run nogm HEAD~1 HEAD
# an unnamed own-class remove plus an unnamed own-class add may be a rename: BLIND, never PASS (S1)
chk "OM3 unnamed remove+add exits 2" '[ "$rc" -eq 2 ]'
chk "OM3 unnamed remove reported BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own x=>? $S3 - commit=.* reason=unpaired-rename"'
chk "OM3 unnamed add reported" 'printf "%s\n" "$out" | grep -q "^ADDED own y $S1"'

# 30. OM4: path lookup is exact (sub2 listed first with a third-party URL, sub own)
git init -q -b main "$T/pfx"; c "$T/pfx" root >/dev/null
git -C "$T/pfx" submodule add -q "$T/sub_src" sub2; git -C "$T/pfx" submodule add -q "$T/sub_src" sub
git -C "$T/pfx" config -f .gitmodules submodule.sub2.url "$TP_URL"; git -C "$T/pfx" config -f .gitmodules submodule.sub.url "$OWN_URL"
git -C "$T/pfx/sub" checkout -q "$S3"; git -C "$T/pfx" add .gitmodules sub sub2; git -C "$T/pfx" commit -q -m pins
move pfx "$S1" >/dev/null
run pfx HEAD~1 HEAD
chk "OM4 exact path lookup, own backward exits 1" '[ "$rc" -eq 1 ]'
chk "OM4 exact path lookup classed own" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'

# 31. OM6: an own NONFF and an own BLIND in one range: FAIL (exit 1) wins over BLIND
FAKE2=1123456789abcdef0123456789abcdef01234567
mkparent om6 "$S3" "$OWN_URL" >/dev/null
git -C "$T/om6" update-index --add --cacheinfo "160000,$FAKE,ghost"; git -C "$T/om6" commit -q -m ghost
git -C "$T/om6/sub" checkout -q "$S1"; git -C "$T/om6" add sub
git -C "$T/om6" update-index --cacheinfo "160000,$FAKE2,ghost"; git -C "$T/om6" commit -q -m both
run om6 HEAD~1 HEAD
chk "OM6 NONFF+BLIND exits 1" '[ "$rc" -eq 1 ]'
chk "OM6 NONFF+BLIND verdict FAIL" 'printf "%s\n" "$out" | grep -q "^VERDICT: FAIL .*own_nonff=[1-9].*own_blind=[1-9]"'

# 32. OM7: git@host:grp/sub/repo (more than two path segments) is own
mkparent ms "$S3" 'git@github.com:somevendor/group/sub.git' >/dev/null; move ms "$S1" >/dev/null
run ms HEAD~1 HEAD
chk "OM7 multi-segment scp URL is own, exits 1" '[ "$rc" -eq 1 ]'

# 33. N6: own-org list with inline comments
printf 'vasic-digital   # main org\n  # comment line\nHelixDevelopment\n' > "$T/orgs_inline.txt"
out="$(cd "$T/back" && SUBFF_OWN_ORGS="$T/orgs_inline.txt" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "N6 inline comment stripped from own-org list (exit 1)" '[ "$rc" -eq 1 ]'

# 34. N6: an empty range says nothing was checked
P=$(git -C "$T/fwd" rev-parse HEAD)
run fwd "$P" "$P"
chk "N6 empty range exits 0" '[ "$rc" -eq 0 ]'
chk "N6 empty range verdict notes nothing checked" 'printf "%s\n" "$out" | grep -q "^VERDICT: PASS checked=0 .*note=nothing-checked"'

# 35. T1: a submodule below a directory (libs/sub) is seen (diff-tree must recurse)
git init -q -b main "$T/nest"; c "$T/nest" root >/dev/null
git -C "$T/nest" submodule add -q "$T/sub_src" libs/sub
git -C "$T/nest" config -f .gitmodules submodule.libs/sub.url "$OWN_URL"
git -C "$T/nest/libs/sub" checkout -q "$S3"; git -C "$T/nest" add .gitmodules libs/sub; git -C "$T/nest" commit -q -m pin
git -C "$T/nest/libs/sub" checkout -q "$S1"; git -C "$T/nest" add libs/sub; git -C "$T/nest" commit -q -m back
run nest HEAD~1 HEAD
chk "T1 nested path backward exits 1" '[ "$rc" -eq 1 ]'
chk "T1 nested path reported" 'printf "%s\n" "$out" | grep -q "^NONFF own libs/sub $S3 $S1"'

# 36. S1: rename (name+path) whose URL changes form only (scp/ssh/https/git/case/port/user) is paired
i=0
for u in 'ssh://git@github.com/vasic-digital/sub.git' 'https://github.com/vasic-digital/sub' \
         'git://github.com/vasic-digital/sub.git' 'git@GitHub.com:Vasic-Digital/Sub.git' \
         'ssh://someone@github.com:22/vasic-digital/sub/' 'https://GITHUB.COM/vasic-digital/sub.GIT'; do
  i=$((i + 1)); rn "rnf$i" "$u"; run "rnf$i" HEAD~1 HEAD
  chk "S1 URL form $i ($u) paired, exits 1" '[ "$rc" -eq 1 ]'
  chk "S1 URL form $i reported as one move" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>lib $S3 $S1"'
done

# 37. S1: own remove + own add in one commit that cannot be paired is BLIND, never PASS
rn rnx 'git@github.com:vasic-digital/other.git'
run rnx HEAD~1 HEAD
chk "S1 unpaired own remove+add exits 2" '[ "$rc" -eq 2 ]'
chk "S1 unpaired own remove+add reason" 'printf "%s\n" "$out" | grep -q "^BLIND own sub=>? $S3 - commit=.* reason=unpaired-rename"'

# 38. C1: a stale own-org list that matches no submodule URL while pins move is BLIND
printf 'someotherorg\n' > "$T/orgs_stale.txt"
mkparent cst "$S1" "$OWN_URL" >/dev/null; move cst "$S3" >/dev/null
out="$(cd "$T/cst" && SUBFF_OWN_ORGS="$T/orgs_stale.txt" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "C1 positive control: default list on the same range exits 0" '(cd "$T/cst" && bash "$GATE" HEAD~1 HEAD >/dev/null 2>&1)'
chk "C1 stale own-org list exits 2" '[ "$rc" -eq 2 ]'
chk "C1 stale own-org list reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=own-orgs-match-nothing"'
out="$(cd "$T/fwd" && bash "$GATE" --selftest 2>&1)"; rc=$?
chk "C1 selftest prints own-org class" 'printf "%s\n" "$out" | grep -q "^NEEDLE forward=FF backward=NONFF class=own"'

# 39. T2: annotated tags as OLD and NEW
git -C "$T/back" tag -a -m o t_old HEAD~1; git -C "$T/back" tag -a -m n t_new HEAD
run back t_old t_new
chk "T2 annotated tags exit 1" '[ "$rc" -eq 1 ]'
chk "T2 annotated tags reported" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1 commit=RANGE"'

# 40. T2/R5-3: two removed gitlinks sharing one URL, one added: which old pin went where cannot be
# told, so both removals are BLIND ambiguous-pairing (never a first-match pairing)
git init -q -b main "$T/two"; c "$T/two" root >/dev/null
git -C "$T/two" submodule add -q "$T/sub_src" a; git -C "$T/two" submodule add -q "$T/sub_src" b
git -C "$T/two" config -f .gitmodules submodule.a.url "$OWN_URL"; git -C "$T/two" config -f .gitmodules submodule.b.url "$OWN_URL"
git -C "$T/two/a" checkout -q "$S1"; git -C "$T/two/b" checkout -q "$S3"; git -C "$T/two" add .gitmodules a b; git -C "$T/two" commit -q -m two
git -C "$T/two" rm -q a b; git -C "$T/two" submodule add -q "$T/sub_src" cc
git -C "$T/two" config -f .gitmodules submodule.cc.url "$OWN_URL"; git -C "$T/two/cc" checkout -q "$S2"
git -C "$T/two" add .gitmodules cc; git -C "$T/two" commit -q -m merge-into-cc
run two HEAD~1 HEAD
chk "R5-3 shared URL, two removals one addition exits 2" '[ "$rc" -eq 2 ]'
chk "R5-3 shared URL first removal ambiguous" 'printf "%s\n" "$out" | grep -q "^BLIND own a=>? $S1 - commit=.* reason=ambiguous-pairing"'
chk "R5-3 shared URL second removal ambiguous" 'printf "%s\n" "$out" | grep -q "^BLIND own b=>? $S3 - commit=.* reason=ambiguous-pairing"'
chk "R5-3 shared URL never paired by first match" '! printf "%s\n" "$out" | grep -q "a=>cc"'

# 41. T2: the first store candidate (.git/modules/<name>) lacks only the OLD pin; the checkout has both
git init -q -b main "$T/cand"; c "$T/cand" root >/dev/null
git clone -q "$T/sub_src" "$T/cand/sub"; git -C "$T/cand/sub" checkout -q "$X1"
printf '[submodule "sub"]\n\tpath = sub\n\turl = %s\n' "$OWN_URL" > "$T/cand/.gitmodules"
git -C "$T/cand" add .gitmodules sub 2>/dev/null; git -C "$T/cand" commit -q -m pin
# --no-local: a local-path clone copies every object, so --single-branch alone would still hold X1
mkdir -p "$T/cand/.git/modules"; git clone -q --bare --no-local --single-branch -b main "$T/sub_src" "$T/cand/.git/modules/sub"
chk "T2 precondition: first store really lacks the OLD pin" '! git --git-dir="$T/cand/.git/modules/sub" cat-file -e "$X1^{commit}" 2>/dev/null'
chk "T2 precondition: first store holds the NEW pin" 'git --git-dir="$T/cand/.git/modules/sub" cat-file -e "$S3^{commit}" 2>/dev/null'
git -C "$T/cand/sub" checkout -q "$S3"; git -C "$T/cand" add sub; git -C "$T/cand" commit -q -m sideways
run cand HEAD~1 HEAD
chk "T2 first store lacks OLD, checkout used: exits 1" '[ "$rc" -eq 1 ]'
chk "T2 first store lacks OLD, sideways NONFF" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $X1 $S3"'

# 42. T2: own-org list matched exactly, not as a substring (org "vasic" is third-party)
git init -q -b main "$T/sstr"; c "$T/sstr" root >/dev/null
git -C "$T/sstr" submodule add -q "$T/sub_src" sub; git -C "$T/sstr" submodule add -q "$T/sub_src" own
git -C "$T/sstr" config -f .gitmodules submodule.sub.url 'git@github.com:vasic/sub.git'
git -C "$T/sstr" config -f .gitmodules submodule.own.url "$OWN_URL"
git -C "$T/sstr/sub" checkout -q "$S3"; git -C "$T/sstr" add .gitmodules sub own; git -C "$T/sstr" commit -q -m pins
move sstr "$S1" >/dev/null
run sstr HEAD~1 HEAD
chk "T2 org substring is third-party (exit 0)" '[ "$rc" -eq 0 ]'
chk "T2 org substring reported third-party" 'printf "%s\n" "$out" | grep -q "^NONFF third-party sub $S3 $S1"'

# 43. T2: a path owned by an own-org entry anywhere in the range stays own even when the
# moving commit's .gitmodules names it with a third-party URL
mkparent pkey "$S3" "$TP_URL" >/dev/null; PB=$(git -C "$T/pkey" rev-parse HEAD)
printf '[submodule "other"]\n\tpath = sub\n\turl = %s\n' "$OWN_URL" > "$T/pkey/.gitmodules"; git -C "$T/pkey" add .gitmodules; git -C "$T/pkey" commit -q -m own-name
printf '[submodule "sub"]\n\tpath = sub\n\turl = %s\n' "$TP_URL" > "$T/pkey/.gitmodules"; git -C "$T/pkey" add .gitmodules; git -C "$T/pkey" commit -q -m tp-name
move pkey "$S1" >/dev/null
run pkey "$PB" HEAD
chk "T2 path key keeps own class (exit 1)" '[ "$rc" -eq 1 ]'

# 44. P1: A...B is refused with a clear message
out="$(cd "$T/back" && bash "$GATE" HEAD~1...HEAD 2>&1)"; rc=$?
chk "P1 A...B exits 64" '[ "$rc" -eq 64 ]'
chk "P1 A...B message" 'printf "%s\n" "$out" | grep -q "symmetric"'

# 45. P1: a shallow parent repository cannot prove the range: BLIND
git clone -q --depth 1 "file://$T/back" "$T/pshal"
out="$(cd "$T/pshal" && bash "$GATE" HEAD HEAD 2>&1)"; rc=$?
chk "P1 shallow parent exits 2" '[ "$rc" -eq 2 ]'
chk "P1 shallow parent reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND reason=parent-shallow"'

# 46. S1/F5: scp-like URL without a user and without a dot in the host is not a remote; it stays
# own (fail-safe), so a backward move FAILs (kills a mutant that drops the scp host guard)
mkparent scph "$S3" 'localbox:vendor/sub.git' >/dev/null; anchor scph >/dev/null; move scph "$S1" >/dev/null
run scph HEAD~1 HEAD
chk "F5 localbox:path is not a remote, own backward exits 1" '[ "$rc" -eq 1 ]'
chk "F5 localbox:path reported own" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'

# 47. C1: the needle uses the first own-org entry; an entry that cannot form an org URL blinds it
printf 'bad/org\nvasic-digital\n' > "$T/orgs_bad.txt"
out="$(cd "$T/fwd" && SUBFF_OWN_ORGS="$T/orgs_bad.txt" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "C1 needle with unusable first org exits 2" '[ "$rc" -eq 2 ]'
chk "C1 needle with unusable first org reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND reason=control-needle-failed"'

# 48. T2: OLD / NEW must peel to a commit; an annotated tag on a tree is a usage error (exit 64)
git -C "$T/fwd" -c user.name=t -c user.email=t@t tag -a treetag -m treetag 'HEAD^{tree}'
out="$(cd "$T/fwd" && bash "$GATE" treetag HEAD 2>&1)"; rc=$?
chk "T2 tag on a tree as OLD exits 64" '[ "$rc" -eq 64 ]'
out="$(cd "$T/fwd" && bash "$GATE" HEAD treetag 2>&1)"; rc=$?
chk "T2 tag on a tree as NEW exits 64" '[ "$rc" -eq 64 ]'

# 49. R5-1: submodule "ignore" settings must not hide a gitlink move (diff-tree --ignore-submodules=none)
mkparent ign "$S3" "$OWN_URL" >/dev/null; move ign "$S1" >/dev/null
git -C "$T/ign" config submodule.sub.ignore all
run ign HEAD~1 HEAD
chk "R5-1 repo-local submodule.sub.ignore=all, backward exits 1" '[ "$rc" -eq 1 ]'
chk "R5-1 repo-local ignore=all, NONFF reported" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1 commit=RANGE"'
git -C "$T/ign" config --unset submodule.sub.ignore; git -C "$T/ign" config diff.ignoreSubmodules all
run ign HEAD~1 HEAD
chk "R5-1 repo-local diff.ignoreSubmodules=all, backward exits 1" '[ "$rc" -eq 1 ]'
git -C "$T/ign" config --unset diff.ignoreSubmodules
mkparent ign2 "$S3" "$OWN_URL" >/dev/null
git -C "$T/ign2" config -f .gitmodules submodule.sub.ignore all; git -C "$T/ign2" add .gitmodules; git -C "$T/ign2" commit -q -m ignore-all
move ign2 "$S1" >/dev/null
run ign2 HEAD~1 HEAD
chk "R5-1 worktree .gitmodules ignore = all, backward exits 1" '[ "$rc" -eq 1 ]'
chk "R5-1 .gitmodules ignore=all, NONFF reported" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1 commit=RANGE"'

# 50. R5-1: the RANGE diff is cross-checked against ls-tree of OLD and NEW; a blinded diff is BLIND
# (gate-logic test, 11.4.289(C)(2): the wrapper only drops --ignore-submodules=none from git's
# argument list, so the repo-local ignore=all really hides the move from diff-tree)
mkdir -p "$T/nonegit"
printf '#!/bin/sh\nfor a in "$@"; do shift; [ "$a" = --ignore-submodules=none ] || set -- "$@" "$a"; done\nexec "%s" "$@"\n' "$REALGIT" > "$T/nonegit/git"
chmod +x "$T/nonegit/git"
git -C "$T/ign" config submodule.sub.ignore all
out="$(cd "$T/ign" && PATH="$T/nonegit:$PATH" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
git -C "$T/ign" config --unset submodule.sub.ignore
chk "R5-1 blinded diff exits 2" '[ "$rc" -eq 2 ]'
chk "R5-1 blinded diff reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=gitlink-diff-mismatch"'
chk "R5-1 blinded diff never PASS" '! printf "%s\n" "$out" | grep -q "^VERDICT: PASS"'
out="$(cd "$T/back" && PATH="$T/nonegit:$PATH" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "R5-1 positive control: wrapper without ignore config still FAILs a backward move" '[ "$rc" -eq 1 ]'
mkdir -p "$T/nolsgit"
printf '#!/bin/sh\nfor a in "$@"; do [ "$a" = ls-tree ] && exit 128; done\nexec "%s" "$@"\n' "$REALGIT" > "$T/nolsgit/git"
chmod +x "$T/nolsgit/git"
out="$(cd "$T/fwd" && PATH="$T/nolsgit:$PATH" bash "$GATE" HEAD~1 HEAD 2>&1)"; rc=$?
chk "R5-1 failing ls-tree cross-check exits 2" '[ "$rc" -eq 2 ]'
chk "R5-1 failing ls-tree cross-check reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=git-error"'

# 51. R5-2: the submodule store now lives under another name (renamed since): any other store
# under <common>/modules holding both pins is used; shallow and grafted stores are skipped
mkparent fb "$S3" "$OWN_URL" >/dev/null; FB=$(move fb "$S2")
mv "$T/fb/.git/modules/sub" "$T/fb/.git/modules/zz_full"
rm -rf "${T:?}/fb/sub"
git clone -q --bare --depth 2 "file://$T/sub_src" "$T/fb/.git/modules/a_shallow"
git clone -q --bare --no-local "$T/sub_src" "$T/fb/.git/modules/b_grafted"
printf '%s\n' "$S2" > "$T/fb/.git/modules/b_grafted/info/grafts"
chk "R5-2 precondition: moved store has a dangling core.worktree" '! git --git-dir="$T/fb/.git/modules/zz_full" rev-parse --is-shallow-repository >/dev/null 2>&1'
chk "R5-2 precondition: shallow store holds both pins" 'git --git-dir="$T/fb/.git/modules/a_shallow" cat-file -e "$S2^{commit}" && git --git-dir="$T/fb/.git/modules/a_shallow" cat-file -e "$S3^{commit}"'
chk "R5-2 precondition: shallow store is shallow" '[ "$(git --git-dir="$T/fb/.git/modules/a_shallow" rev-parse --is-shallow-repository)" = true ]'
run fb HEAD~1 HEAD
chk "R5-2 renamed store found by fallback, backward exits 1" '[ "$rc" -eq 1 ]'
chk "R5-2 renamed store, NONFF reported" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S2 commit=RANGE"'

# 52. R5-3: two same-URL submodules swapped under new names cannot be paired: BLIND, never first match
git init -q -b main "$T/swap"; c "$T/swap" root >/dev/null
git -C "$T/swap" submodule add -q "$T/sub_src" a; git -C "$T/swap" submodule add -q --name bname "$T/sub_src" b
git -C "$T/swap" config -f .gitmodules submodule.a.url "$OWN_URL"; git -C "$T/swap" config -f .gitmodules submodule.bname.url "$OWN_URL"
git -C "$T/swap/a" checkout -q "$S3"; git -C "$T/swap/b" checkout -q "$S1"; git -C "$T/swap" add -A; git -C "$T/swap" commit -q -m two
git -C "$T/swap" mv a d; git -C "$T/swap" mv b c
git -C "$T/swap" config -f .gitmodules --rename-section submodule.a submodule.dn; git -C "$T/swap" config -f .gitmodules submodule.dn.path d
git -C "$T/swap" config -f .gitmodules --rename-section submodule.bname submodule.cn; git -C "$T/swap" config -f .gitmodules submodule.cn.path c
git -C "$T/swap/d" checkout -q "$S1"; git -C "$T/swap/c" checkout -q "$S3"; git -C "$T/swap" add -A; git -C "$T/swap" commit -q -m swap
run swap HEAD~1 HEAD
chk "R5-3 same-URL swap exits 2" '[ "$rc" -eq 2 ]'
chk "R5-3 same-URL swap reason" 'printf "%s\n" "$out" | grep -q "^BLIND own a=>? $S3 - commit=RANGE reason=ambiguous-pairing"'

# 53. R5-3: a URL-paired addition is consumed; an unrelated own removal in the same commit stays REMOVED
git init -q -b main "$T/upr"; c "$T/upr" root >/dev/null
git -C "$T/upr" submodule add -q "$T/sub_src" a; git -C "$T/upr" submodule add -q "$T/sub_src" b
git -C "$T/upr" config -f .gitmodules submodule.a.url "$OWN_URL"; git -C "$T/upr" config -f .gitmodules submodule.b.url 'git@github.com:vasic-digital/other.git'
git -C "$T/upr/a" checkout -q "$S1"; git -C "$T/upr/b" checkout -q "$S3"; git -C "$T/upr" add -A; git -C "$T/upr" commit -q -m two
git -C "$T/upr" rm -q a b; git -C "$T/upr" submodule add -q "$T/sub_src" cc
git -C "$T/upr" config -f .gitmodules submodule.cc.url "$OWN_URL"; git -C "$T/upr/cc" checkout -q "$S2"
git -C "$T/upr" add .gitmodules cc; git -C "$T/upr" commit -q -m "a renamed to cc, b dropped"
run upr HEAD~1 HEAD
chk "R5-3 single URL pair consumed, exits 0" '[ "$rc" -eq 0 ]'
chk "R5-3 single URL pair FF" 'printf "%s\n" "$out" | grep -q "^FF own a=>cc $S1 $S2 commit=RANGE"'
chk "R5-3 unrelated removal REMOVED" 'printf "%s\n" "$out" | grep -q "^REMOVED own b $S3 commit=RANGE"'

# 54. R5-4: relative own URLs (../sub.git) are own and cannot be hidden by the own-org list, so a
# range moving only them is decided normally (no false own-orgs-match-nothing)
mkparent rel "$S1" '../sub.git' >/dev/null; move rel "$S3" >/dev/null
run rel HEAD~1 HEAD
chk "R5-4 relative URL forward exits 0" '[ "$rc" -eq 0 ]'
chk "R5-4 relative URL forward reported own FF" 'printf "%s\n" "$out" | grep -q "^FF own sub $S1 $S3 commit=RANGE"'
move rel "$S1" >/dev/null; run rel HEAD~1 HEAD
chk "R5-4 relative URL backward exits 1" '[ "$rc" -eq 1 ]'

# 55. R5-7: a scheme URL's ?query and #fragment are not part of the path (rename still pairs)
rn rnq 'https://github.com/vasic-digital/sub.git?ref=x#frag'; run rnq HEAD~1 HEAD
chk "R5-7 query+fragment URL paired, exits 1" '[ "$rc" -eq 1 ]'
chk "R5-7 query+fragment URL one move" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>lib $S3 $S1"'

# 56. R5-7: "~user" and "%XX" paths yield no organisation, so they are own (fail closed)
i=0
for u in 'https://github.com/somevendor%2Dx/sub.git' 'ssh://git@github.com/~someone/sub.git'; do
  i=$((i + 1)); mkparent "tl$i" "$S3" "$u" >/dev/null; anchor "tl$i" >/dev/null; move "tl$i" "$S1" >/dev/null
  run "tl$i" HEAD~1 HEAD
  chk "R5-7 URL $u is own, backward exits 1" '[ "$rc" -eq 1 ]'
  chk "R5-7 URL $u reported own" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1"'
done

# 57. R5-8: one checked move (only the RANGE line: OLD is a side commit, NEW never touches the pin)
# with a stale own-org list classing it third-party is BLIND
B=$(mkparent one "$S1" "$OWN_URL")
git -C "$T/one" checkout -q -b side; OLD1=$(move one "$S2"); git -C "$T/one" checkout -q main
git -C "$T/one/sub" checkout -q "$S1"; NEW1=$(c "$T/one" noop)
out="$(cd "$T/one" && SUBFF_OWN_ORGS="$T/orgs_stale.txt" bash "$GATE" "$OLD1" "$NEW1" 2>&1)"; rc=$?
chk "R5-8 single checked move, stale list exits 2" '[ "$rc" -eq 2 ]'
chk "R5-8 single checked move is checked=1" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND checked=1 .*reason=own-orgs-match-nothing"'
run one "$OLD1" "$NEW1"
chk "R5-8 positive control: default list FAILs the backward RANGE move" '[ "$rc" -eq 1 ]'

# 58. R5-8: a path known third-party only in a middle tree (no .gitmodules entry for it at OLD,
# at NEW, or on either side of the moving commit) is classed by its path key: third-party, so the
# backward move does not FAIL
git init -q -b main "$T/pk3"; c "$T/pk3" root >/dev/null
git clone -q "$T/sub_src" "$T/pk3/sub"; git -C "$T/pk3/sub" checkout -q "$S3"
printf '[submodule "anchor"]\n\tpath = anchor\n\turl = %s\n' "$OWN_URL" > "$T/pk3/.gitmodules"
git -C "$T/pk3" add .gitmodules sub 2>/dev/null; git -C "$T/pk3" commit -q -m "pin without entry"; PK=$(git -C "$T/pk3" rev-parse HEAD)
printf '[submodule "anchor"]\n\tpath = anchor\n\turl = %s\n[submodule "vend"]\n\tpath = sub\n\turl = %s\n' "$OWN_URL" "$TP_URL" > "$T/pk3/.gitmodules"
git -C "$T/pk3" add .gitmodules; git -C "$T/pk3" commit -q -m "tp entry"
printf '[submodule "anchor"]\n\tpath = anchor\n\turl = %s\n' "$OWN_URL" > "$T/pk3/.gitmodules"
git -C "$T/pk3" add .gitmodules; git -C "$T/pk3" commit -q -m "entry dropped"
move pk3 "$S1" >/dev/null
chk "R5-8 precondition: gitlink recorded at OLD" '[ "$(git -C "$T/pk3" ls-tree "$PK" sub | cut -c1-6)" = 160000 ]'
run pk3 "$PK" HEAD
chk "R5-8 third-party path key, backward exits 0" '[ "$rc" -eq 0 ]'
chk "R5-8 third-party path key reported" 'printf "%s\n" "$out" | grep -q "^NONFF third-party sub $S3 $S1 commit=RANGE"'

# 59. F2 (round 5): an intermediate commit's tree is missing while both endpoints are intact, so the
# ls-tree cross-check of the endpoints cannot catch it: only the per-commit diff error can
cp -a "$T/zig" "$T/zigt"
ZTO=$(git -C "$T/zigt" rev-parse HEAD~2); ZTN=$(git -C "$T/zigt" rev-parse HEAD)
ZTT=$(git -C "$T/zigt" rev-parse "HEAD~1^{tree}")
rm -f "${T:?}/zigt/.git/objects/${ZTT:0:2}/${ZTT:2}"
chk "F2b precondition: endpoint trees readable" 'git -C "$T/zigt" cat-file -e "$ZTO^{tree}" && git -C "$T/zigt" cat-file -e "$ZTN^{tree}"'
chk "F2b precondition: middle tree missing" '! git -C "$T/zigt" cat-file -e "$ZTT" 2>/dev/null'
run zigt "$ZTO" "$ZTN"
chk "F2b missing middle tree exits 2" '[ "$rc" -eq 2 ]'
chk "F2b missing middle tree reason git-error" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=git-error"'

# 60. OM3 (round 5): a gitlink with no .gitmodules entry (no name, no URL) removed and another one
# added in the same commit must not be paired through two empty URL keys: BLIND unpaired-rename
git init -q -b main "$T/eu"; c "$T/eu" root >/dev/null
git -C "$T/eu" update-index --add --cacheinfo 160000,"$S3",a
git -C "$T/eu" commit -q -m "pin a"; EU0=$(git -C "$T/eu" rev-parse HEAD)
git -C "$T/eu" update-index --force-remove a
git -C "$T/eu" update-index --add --cacheinfo 160000,"$S1",b
git -C "$T/eu" commit -q -m "a gone, b added"; EU1=$(git -C "$T/eu" rev-parse HEAD)
chk "OM3b precondition: no .gitmodules" '! git -C "$T/eu" cat-file -e "$EU1:.gitmodules" 2>/dev/null'
run eu "$EU0" "$EU1"
chk "OM3b unnamed remove+add exits 2" '[ "$rc" -eq 2 ]'
chk "OM3b unnamed remove+add unpaired" 'printf "%s\n" "$out" | grep -q "^BLIND own a=>? $S3 - commit=$EU1 reason=unpaired-rename"'
chk "OM3b never paired a=>b" '! printf "%s\n" "$out" | grep -q "a=>b"'

# 61. F1 (round 6): remove and re-add in SEPARATE commits must not hide a backward move. The
# re-added pin is compared with the last pin the same submodule (name, else URL key) had.
# rmsec REPO NAME: drop gitlink NAME at path NAME and its .gitmodules section, commit
rmsec() { git -C "$T/$1" rm -q --cached "$2"; git -C "$T/$1" config -f .gitmodules --remove-section "submodule.$2"
  git -C "$T/$1" add .gitmodules; git -C "$T/$1" commit -q -m "remove $2"; git -C "$T/$1" rev-parse HEAD; }
# addsec REPO NAME PIN URL: record gitlink NAME at path NAME pinned to PIN with a .gitmodules section, commit
addsec() { git -C "$T/$1" update-index --add --cacheinfo 160000,"$3","$2"
  git -C "$T/$1" config -f .gitmodules "submodule.$2.path" "$2"; git -C "$T/$1" config -f .gitmodules "submodule.$2.url" "$4"
  git -C "$T/$1" add .gitmodules; git -C "$T/$1" commit -q -m "add $2"; git -C "$T/$1" rev-parse HEAD; }
A1O=$(mkparent ra1 "$S1" "$OWN_URL"); move ra1 "$S3" >/dev/null; rmsec ra1 sub >/dev/null; addsec ra1 sub "$S1" "$OWN_URL" >/dev/null
run ra1 "$A1O" HEAD
chk "A1 FF, remove, re-add older pin exits 1" '[ "$rc" -eq 1 ]'
chk "A1 re-add compared with the last pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S1 commit="'
# control: the same net change in one commit is NONFF too
A1C=$(mkparent ra1c "$S1" "$OWN_URL"); move ra1c "$S3" >/dev/null; move ra1c "$S1" >/dev/null
run ra1c "$A1C" HEAD
chk "A1 control: one-commit backward move exits 1" '[ "$rc" -eq 1 ]'
# A2: OLD lacks the submodule; add at S3, remove, add at S1
git init -q -b main "$T/ra2"; A2O=$(c "$T/ra2" root)
git -C "$T/ra2" submodule add -q "$T/sub_src" sub; git -C "$T/ra2/sub" checkout -q "$S3"
git -C "$T/ra2" config -f .gitmodules submodule.sub.url "$OWN_URL"; git -C "$T/ra2" add .gitmodules sub; git -C "$T/ra2" commit -q -m "add sub"
rmsec ra2 sub >/dev/null; addsec ra2 sub "$S1" "$OWN_URL" >/dev/null
run ra2 "$A2O" HEAD
chk "A2 add, remove, re-add older pin exits 1" '[ "$rc" -eq 1 ]'
chk "A2 re-add compared with the removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S1 commit="'
# A9: rename across commits (sub removed, lib added with the same URL), net S1 -> S1 through S3
A9O=$(mkparent ra9 "$S1" "$OWN_URL"); move ra9 "$S3" >/dev/null; rmsec ra9 sub >/dev/null; addsec ra9 lib "$S1" "$OWN_URL" >/dev/null
run ra9 "$A9O" HEAD
chk "A9 cross-commit rename to an older pin exits 1" '[ "$rc" -eq 1 ]'
chk "A9 rename paired by URL key" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>lib $S3 $S1 commit="'
# chained renames: sub -> lib (forward to S3) -> tool (S2, not a descendant of S3)
A9C=$(mkparent ra9c "$S1" "$OWN_URL"); rmsec ra9c sub >/dev/null; addsec ra9c lib "$S3" "$OWN_URL" >/dev/null
rmsec ra9c lib >/dev/null; addsec ra9c tool "$S2" "$OWN_URL" >/dev/null
run ra9c "$A9C" HEAD
chk "A9 chained rename exits 1" '[ "$rc" -eq 1 ]'
chk "A9 chained first hop forward" 'printf "%s\n" "$out" | grep -q "^FF own sub=>lib $S1 $S3 commit="'
chk "A9 chained second hop backward" 'printf "%s\n" "$out" | grep -q "^NONFF own lib=>tool $S3 $S2 commit="'
# negative control: remove and re-add at a later pin is a forward move, PASS
A1F=$(mkparent ra1f "$S1" "$OWN_URL"); rmsec ra1f sub >/dev/null; addsec ra1f sub "$S3" "$OWN_URL" >/dev/null
run ra1f "$A1F" HEAD
chk "A1 negative control: re-add at a later pin exits 0" '[ "$rc" -eq 0 ]'
chk "A1 negative control: reported FF" 'printf "%s\n" "$out" | grep -q "^FF own sub=>sub $S1 $S3 commit="'
# two same-URL submodules removed at different pins, one re-added under a new name: which old pin? BLIND
AMO=$(mkparent ram "$S3" "$OWN_URL"); addsec ram twin "$S2" "$OWN_URL" >/dev/null
AMO=$(git -C "$T/ram" rev-parse HEAD); rmsec ram sub >/dev/null; rmsec ram twin >/dev/null; addsec ram lib "$S1" "$OWN_URL" >/dev/null
run ram "$AMO" HEAD
chk "A9 ambiguous re-add exits 2" '[ "$rc" -eq 2 ]'
chk "A9 ambiguous re-add reason" 'printf "%s\n" "$out" | grep -q "^BLIND own ?=>lib - $S1 commit=.* reason=readd-ambiguous"'

# 62. F2 (round 6): an unparsable .gitmodules is BLIND, never silently ignored. OLD names sub with
# an own URL after a malformed first line; NEW switches it to a fork URL and moves it backward.
B1P=$(mkparent rb1 "$S3" "$OWN_URL")
{ printf 'this is not config\n'; git -C "$T/rb1" show HEAD:.gitmodules; } > "$T/rb1/.gitmodules"
git -C "$T/rb1" add .gitmodules; git -C "$T/rb1" commit -q -m "malformed"; B1O=$(git -C "$T/rb1" rev-parse HEAD)
chk "B1 precondition: OLD .gitmodules does not parse" '! git -C "$T/rb1" config --blob "$B1O:.gitmodules" --list >/dev/null 2>&1'
printf '[submodule "sub"]\n\tpath = sub\n\turl = %s\n' "$TP_URL" > "$T/rb1/.gitmodules"
git -C "$T/rb1" add .gitmodules; git -C "$T/rb1" update-index --cacheinfo 160000,"$S1",sub; git -C "$T/rb1" commit -q -m "fork url, backward"
run rb1 "$B1O" HEAD
chk "B1 unparsable .gitmodules exits 2" '[ "$rc" -eq 2 ]'
chk "B1 unparsable .gitmodules reason" 'printf "%s\n" "$out" | grep -q "^VERDICT: BLIND .*reason=gitmodules-unparsable"'
# control: the same history with a well-formed OLD .gitmodules FAILs (the own URL is seen)
B1C=$(mkparent rb1c "$S3" "$OWN_URL")
printf '[submodule "sub"]\n\tpath = sub\n\turl = %s\n' "$TP_URL" > "$T/rb1c/.gitmodules"
git -C "$T/rb1c" add .gitmodules; git -C "$T/rb1c" update-index --cacheinfo 160000,"$S1",sub; git -C "$T/rb1c" commit -q -m "fork url, backward"
run rb1c "$B1C" HEAD
chk "B1 control: parsable OLD exits 1" '[ "$rc" -eq 1 ]'

# 63. Round 7: removals are remembered per line of descent, name matches win over URL-key matches.
# F6-1: branch A pins sub at S3 then removes it; branch B (not a descendant of A's removal) adds sub
# at S1 fresh; the merge keeps A's tree (sub absent). No commit moves sub backward: PASS.
P1O=$(mkparent rp1 "$S1" "$OWN_URL"); rmsec rp1 sub >/dev/null; P1B=$(git -C "$T/rp1" rev-parse HEAD)
git -C "$T/rp1" checkout -q -b A; addsec rp1 sub "$S3" "$OWN_URL" >/dev/null; rmsec rp1 sub >/dev/null
git -C "$T/rp1" checkout -q -b B "$P1B"; addsec rp1 sub "$S1" "$OWN_URL" >/dev/null
git -C "$T/rp1" checkout -q A; git -C "$T/rp1" merge -q --no-ff -s ours -m merge B >/dev/null
run rp1 "$P1O" HEAD
chk "F6-1 parallel branch re-add, removal kept, exits 0" '[ "$rc" -eq 0 ]'
chk "F6-1 parallel branch add never compared with the other line" '! printf "%s\n" "$out" | grep -q "^NONFF"'
# F6-1b: same, but the merge brings sub@S1 onto the line that removed sub@S3: FAIL on the merge
P1BO=$(mkparent rp1b "$S1" "$OWN_URL"); rmsec rp1b sub >/dev/null; P1BB=$(git -C "$T/rp1b" rev-parse HEAD)
git -C "$T/rp1b" checkout -q -b A; addsec rp1b sub "$S3" "$OWN_URL" >/dev/null; rmsec rp1b sub >/dev/null
git -C "$T/rp1b" checkout -q -b B "$P1BB"; addsec rp1b sub "$S1" "$OWN_URL" >/dev/null
git -C "$T/rp1b" checkout -q A; git -C "$T/rp1b" merge -q --no-ff -m merge B >/dev/null
P1BM=$(git -C "$T/rp1b" rev-parse HEAD)
run rp1b "$P1BO" HEAD
chk "F6-1b merge re-introduces an older pin exits 1" '[ "$rc" -eq 1 ]'
chk "F6-1b merge compared with the removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S1 commit=$P1BM"'
# F6-2: x (URL u) added at S3 and removed; y (name y, same URL) added at S3 takes the URL-key match;
# x re-added at S1. The name match must still find x's removed pin: FAIL
P2O=$(mkparent rp2 "$S1" "$OWN_URL"); addsec rp2 x "$S3" "$OWN_URL" >/dev/null; rmsec rp2 x >/dev/null
addsec rp2 y "$S3" "$OWN_URL" >/dev/null; addsec rp2 x "$S1" "$OWN_URL" >/dev/null
run rp2 "$P2O" HEAD
chk "F6-2 same-URL sibling does not consume x's removal, exits 1" '[ "$rc" -eq 1 ]'
chk "F6-2 x re-add compared with x's removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own x=>x $S3 $S1 commit="'
chk "F6-2 y compared with x by URL key" 'printf "%s\n" "$out" | grep -q "^FF own x=>y $S3 $S3 commit="'
# A3: name lib reused for an unrelated repository (different URL): the same name is compared (round 8,
# a differing URL key proves nothing), and the store holds no pair of pins: BLIND, never ADDED/PASS
SS1=$S1; SS2=$S2; SS3=$S3; SX1=$X1; mksub other; Q1=$S1; S1=$SS1; S2=$SS2; S3=$SS3; X1=$SX1
P3O=$(mkparent rp3 "$S1" "$OWN_URL"); addsec rp3 lib "$S3" "$OWN_URL" >/dev/null; rmsec rp3 lib >/dev/null
addsec rp3 lib "$Q1" 'git@github.com:vasic-digital/other.git' >/dev/null
run rp3 "$P3O" HEAD
chk "A3 name reused for another repository exits 2" '[ "$rc" -eq 2 ]'
chk "A3 reused name compared, BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own lib=>lib $S3 $Q1 commit=.* reason=pin-not-in-local-object-store"'
# A7: two parallel lines remove sub at S2 and at S3; re-added after the merge: compared with both
mk7() { mkparent "$1" "$S1" "$OWN_URL" >/dev/null; local b; b=$(git -C "$T/$1" rev-parse HEAD)
  git -C "$T/$1" checkout -q -b A; move "$1" "$S2" >/dev/null; rmsec "$1" sub >/dev/null
  git -C "$T/$1" checkout -q -b B "$b"; move "$1" "$S3" >/dev/null; rmsec "$1" sub >/dev/null
  git -C "$T/$1" checkout -q A; git -C "$T/$1" merge -q --no-ff -m merge B >/dev/null; echo "$b"; }
P7O=$(mk7 rp7); addsec rp7 sub "$S3" "$OWN_URL" >/dev/null
run rp7 "$P7O" HEAD
chk "A7 re-add reaching both removed pins exits 0" '[ "$rc" -eq 0 ]'
chk "A7 compared with the S2 line" 'printf "%s\n" "$out" | grep -q "^FF own sub=>sub $S2 $S3 commit="'
chk "A7 compared with the S3 line" 'printf "%s\n" "$out" | grep -q "^FF own sub=>sub $S3 $S3 commit="'
P7BO=$(mk7 rp7b); addsec rp7b sub "$S2" "$OWN_URL" >/dev/null
run rp7b "$P7BO" HEAD
chk "A7b re-add older than one line's pin exits 1" '[ "$rc" -eq 1 ]'
chk "A7b backward against the S3 line" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S2 commit="'
# R2: a same-name match wins over a URL-key match (x at S3 and y at S1 share the URL; x back at S3)
mkparent rp8 "$S1" "$OWN_URL" >/dev/null; addsec rp8 x "$S3" "$OWN_URL" >/dev/null; addsec rp8 y "$S1" "$OWN_URL" >/dev/null
P8O=$(git -C "$T/rp8" rev-parse HEAD); rmsec rp8 x >/dev/null; rmsec rp8 y >/dev/null; addsec rp8 x "$S3" "$OWN_URL" >/dev/null
run rp8 "$P8O" HEAD
chk "R2 name match before URL key exits 0" '[ "$rc" -eq 0 ]'
chk "R2 x compared with x" 'printf "%s\n" "$out" | grep -q "^FF own x=>x $S3 $S3 commit="'
# R3: a and b (same URL, both at S3) removed; c takes both by URL key; d added later at S1 is a new
# same-URL submodule (c continues the a/b line), so it is ADDED, not compared with b
# (OLD is before a and b exist, so the OLD -> NEW diff only adds c and d and stays unambiguous)
mkparent rp9 "$S1" "$OWN_URL" >/dev/null; P9O=$(git -C "$T/rp9" rev-parse HEAD)
addsec rp9 a "$S3" "$OWN_URL" >/dev/null; addsec rp9 b "$S3" "$OWN_URL" >/dev/null
rmsec rp9 a >/dev/null; rmsec rp9 b >/dev/null
addsec rp9 c "$S3" "$OWN_URL" >/dev/null; addsec rp9 d "$S1" "$OWN_URL" >/dev/null
run rp9 "$P9O" HEAD
chk "R3 every key candidate consumed, exits 0" '[ "$rc" -eq 0 ]'
chk "R3 d reported ADDED by its own commit" 'printf "%s\n" "$out" | grep -q "^ADDED own d $S1 commit=[0-9a-f]"'
chk "R3 d never compared with b" '! printf "%s\n" "$out" | grep -q "=>d "'
# R4: committer dates make the default rev-list order put branch B's re-add before the commit that
# removed sub; parents-first (--topo-order) order is required to see the backward move
P10O=$(mkparent rp10 "$S3" "$OWN_URL")
GIT_COMMITTER_DATE='2030-01-01T00:00:00Z' rmsec rp10 sub >/dev/null; P10B=$(git -C "$T/rp10" rev-parse HEAD)
git -C "$T/rp10" checkout -q -b A; GIT_COMMITTER_DATE='2030-01-02T00:00:00Z' c "$T/rp10" a1 >/dev/null
git -C "$T/rp10" checkout -q -b B "$P10B"; P10R=$(GIT_COMMITTER_DATE='2010-01-01T00:00:00Z' addsec rp10 sub "$S1" "$OWN_URL")
git -C "$T/rp10" checkout -q A; GIT_COMMITTER_DATE='2030-01-03T00:00:00Z' git -C "$T/rp10" merge -q --no-ff -s ours -m merge B >/dev/null
chk "R4 precondition: default order lists the re-add before the removal" \
  '[ "$(git -C "$T/rp10" rev-list --reverse "$P10O..HEAD" | grep -x -e "$P10R" -e "$P10B" | head -1)" = "$P10R" ]'
run rp10 "$P10O" HEAD
chk "R4 re-add on a line after the removal exits 1" '[ "$rc" -eq 1 ]'
chk "R4 re-add compared with the removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S1 commit=$P10R"'
# R1: the OLD..NEW (RANGE) diff never uses remembered removals. a (URL u) added at S3, z (same URL)
# added at S1 while a still exists, a removed, one more commit: z is a second copy, never a re-add
mkparent rp11 "$S1" "$OWN_URL" >/dev/null; P11O=$(git -C "$T/rp11" rev-parse HEAD)
addsec rp11 a "$S3" "$OWN_URL" >/dev/null; addsec rp11 z "$S1" "$OWN_URL" >/dev/null; rmsec rp11 a >/dev/null
c "$T/rp11" tail >/dev/null
run rp11 "$P11O" HEAD
chk "R1 RANGE add not paired with an in-range removal, exits 0" '[ "$rc" -eq 0 ]'
chk "R1 z reported ADDED for RANGE" 'printf "%s\n" "$out" | grep -q "^ADDED own z $S1 commit=RANGE"'
# a name match consumes the removal: sub removed at S3 and re-added at S3 by name, then z (same URL)
# added at S1 is a second copy, never compared with the already re-added removal
mkparent rp12 "$S3" "$OWN_URL" >/dev/null; P12O=$(git -C "$T/rp12" rev-parse HEAD)
rmsec rp12 sub >/dev/null; addsec rp12 sub "$S3" "$OWN_URL" >/dev/null; addsec rp12 z "$S1" "$OWN_URL" >/dev/null
run rp12 "$P12O" HEAD
chk "name-consumed removal not reused by a URL key, exits 0" '[ "$rc" -eq 0 ]'
chk "z reported ADDED by its own commit" 'printf "%s\n" "$out" | grep -q "^ADDED own z $S1 commit=[0-9a-f]"'
# memory is per commit: branch A moves sub to S3, removes it and adds one more commit (the removal is
# still live there); branch B, processed right after A's last commit, adds z (same URL) at S1 and
# must not see A's removal; merged with -s ours
P13O=$(mkparent rp13 "$S1" "$OWN_URL")
git -C "$T/rp13" checkout -q -b A; move rp13 "$S3" >/dev/null; rmsec rp13 sub >/dev/null; P13A=$(c "$T/rp13" a3)
git -C "$T/rp13" checkout -q -b B "$P13O"; P13B=$(addsec rp13 z "$S1" "$OWN_URL")
git -C "$T/rp13" checkout -q A; git -C "$T/rp13" merge -q --no-ff -s ours -m merge B >/dev/null
P13L="$(git -C "$T/rp13" rev-list --topo-order --reverse "$P13O..HEAD" | tr '\n' ' ')"
chk "precondition: B's addition is listed right after A's last commit" 'case "$P13L" in *"$P13A $P13B "*) true ;; *) false ;; esac'
run rp13 "$P13O" HEAD
chk "removal on a parallel line not seen by the next listed commit, exits 0" '[ "$rc" -eq 0 ]'
chk "z reported ADDED, not compared" 'printf "%s\n" "$out" | grep -q "^ADDED own z $S1 commit=$P13B" && ! printf "%s\n" "$out" | grep -q "=>z "'

# 64. Round 8. F7-1: a known URL key that differs does not prove another repository. sub moved to S3,
# removed, re-added under the same name at S1 with a URL form the key does not collapse: compared, FAIL
for n1 in "n1:../sub.git" "n1b:git@gitlab.com:vasic-digital/sub.git" "n1c:$OWN_URL"; do
  r="${n1%%:*}"; u="${n1#*:}"
  N1O=$(mkparent "$r" "$S1" "$OWN_URL"); move "$r" "$S3" >/dev/null; rmsec "$r" sub >/dev/null; addsec "$r" sub "$S1" "$u" >/dev/null
  run "$r" "$N1O" HEAD
  chk "$r same name, URL $u, re-added older exits 1" '[ "$rc" -eq 1 ]'
  chk "$r re-add compared with the removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S1 commit="'
done
# F7-1: the name lib reused for an unrelated repository: its pins cannot be compared, BLIND (never PASS)
SS1=$S1; SS2=$S2; SS3=$S3; SX1=$X1; mksub unrel; Q2=$S1; S1=$SS1; S2=$SS2; S3=$SS3; X1=$SX1
N4O=$(mkparent n4 "$S1" "$OWN_URL"); addsec n4 lib "$S3" "$OWN_URL" >/dev/null; rmsec n4 lib >/dev/null
addsec n4 lib "$Q2" 'git@github.com:vasic-digital/unrel.git' >/dev/null
run n4 "$N4O" HEAD
chk "F7-1 name reused for an unrelated repository exits 2" '[ "$rc" -eq 2 ]'
chk "F7-1 reused name compared, store cannot hold both pins" 'printf "%s\n" "$out" | grep -q "^BLIND own lib=>lib $S3 $Q2 commit=.* reason=pin-not-in-local-object-store"'
# F7-2 (N2): a gitlink with no .gitmodules entry added at S3, removed, re-added at S1 at the same path
N2O=$(mkparent n2 "$S1" "$OWN_URL")
git -C "$T/n2" update-index --add --cacheinfo 160000,"$S3",xp; git -C "$T/n2" commit -q -m "xp S3"
git -C "$T/n2" rm -q --cached xp; git -C "$T/n2" commit -q -m "rm xp"
git -C "$T/n2" update-index --add --cacheinfo 160000,"$S1",xp; git -C "$T/n2" commit -q -m "xp S1"
run n2 "$N2O" HEAD
chk "F7-2 N2 nameless gitlink re-added older at the same path exits 1" '[ "$rc" -eq 1 ]'
chk "F7-2 N2 matched by path" 'printf "%s\n" "$out" | grep -q "^NONFF own xp=>xp $S3 $S1 commit="'
# F7-2 (N3): the .gitmodules section of x goes one commit before its gitlink; x re-added at S1 later
N3O=$(mkparent n3 "$S1" "$OWN_URL"); addsec n3 x "$S3" "$OWN_URL" >/dev/null
git -C "$T/n3" config -f .gitmodules --remove-section submodule.x; git -C "$T/n3" add .gitmodules; git -C "$T/n3" commit -q -m "drop x section"
git -C "$T/n3" rm -q --cached x; git -C "$T/n3" commit -q -m "rm x"
addsec n3 x "$S1" "$OWN_URL" >/dev/null
run n3 "$N3O" HEAD
chk "F7-2 N3 section removed first, re-added older exits 1" '[ "$rc" -eq 1 ]'
chk "F7-2 N3 matched by path" 'printf "%s\n" "$out" | grep -q "^NONFF own x=>x $S3 $S1 commit="'
# F7-2: a nameless removal that no later addition can be matched with (other path, a name): BLIND
N5O=$(mkparent n5 "$S1" "$OWN_URL")
git -C "$T/n5" update-index --add --cacheinfo 160000,"$S3",xp; git -C "$T/n5" commit -q -m "xp S3"
git -C "$T/n5" rm -q --cached xp; git -C "$T/n5" commit -q -m "rm xp"
addsec n5 lib "$S1" "$OWN_URL" >/dev/null
run n5 "$N5O" HEAD
chk "F7-2 unidentifiable removal then own addition exits 2" '[ "$rc" -eq 2 ]'
chk "F7-2 unidentifiable reason" 'printf "%s\n" "$out" | grep -q "^BLIND own ?=>lib - $S1 commit=.* reason=readd-unidentified"'
# an empty URL key never matches: nameless xp removed at S3, nameless yp added at S3 elsewhere: BLIND
N5BO=$(mkparent n5b "$S1" "$OWN_URL")
git -C "$T/n5b" update-index --add --cacheinfo 160000,"$S3",xp; git -C "$T/n5b" commit -q -m "xp S3"
git -C "$T/n5b" rm -q --cached xp; git -C "$T/n5b" commit -q -m "rm xp"
git -C "$T/n5b" update-index --add --cacheinfo 160000,"$S3",yp; git -C "$T/n5b" commit -q -m "yp S3"
run n5b "$N5BO" HEAD
chk "F7-2 two nameless paths never matched by an empty key, exits 2" '[ "$rc" -eq 2 ]'
chk "F7-2 nameless yp unidentified" 'printf "%s\n" "$out" | grep -q "^BLIND own ?=>yp - $S3 commit=.* reason=readd-unidentified"'
# a third-party addition after an unidentifiable own removal is a third-party submodule: ADDED
N7O=$(mkparent n7 "$S1" "$OWN_URL")
git -C "$T/n7" update-index --add --cacheinfo 160000,"$S3",xp; git -C "$T/n7" commit -q -m "xp S3"
git -C "$T/n7" rm -q --cached xp; git -C "$T/n7" commit -q -m "rm xp"
addsec n7 vend "$S1" "$TP_URL" >/dev/null
run n7 "$N7O" HEAD
chk "third-party addition never takes an unidentifiable removal, exits 0" '[ "$rc" -eq 0 ]'
chk "third-party addition reported ADDED" 'printf "%s\n" "$out" | grep -q "^ADDED third-party vend $S1 commit=[0-9a-f]"'
# path match only when a name is missing: the path of a named removal reused by another named,
# unrelated submodule is a new submodule (ADDED), not compared
N6O=$(mkparent n6 "$S1" "$OWN_URL"); addsec n6 p "$S3" "$OWN_URL" >/dev/null; rmsec n6 p >/dev/null
git -C "$T/n6" update-index --add --cacheinfo 160000,"$Q2",p
git -C "$T/n6" config -f .gitmodules submodule.q.path p; git -C "$T/n6" config -f .gitmodules submodule.q.url 'git@github.com:vasic-digital/unrel.git'
git -C "$T/n6" add .gitmodules; git -C "$T/n6" commit -q -m "q at path p"
run n6 "$N6O" HEAD
chk "path of a named removal reused by another name exits 0" '[ "$rc" -eq 0 ]'
chk "path reuse reported ADDED" 'printf "%s\n" "$out" | grep -q "^ADDED own p $Q2 commit=[0-9a-f]"'

# 65. F7-3: removal-memory set operations. V1: a parent with g live for both kinds of match wins over
# a parent with g live for a name only. Base removes x (S2); line B adds y (same URL, S3), which marks
# g name-only there; merge -s ours on A (B second parent) drops y again (entry h at S3); z (same URL)
# added at S1 matches g (S2) and h (S3): pins differ, BLIND readd-ambiguous
mkparent v1 "$S1" "$OWN_URL" >/dev/null; addsec v1 x "$S2" "$OWN_URL" >/dev/null; V1O=$(git -C "$T/v1" rev-parse HEAD)
rmsec v1 x >/dev/null; V1R=$(git -C "$T/v1" rev-parse HEAD)
git -C "$T/v1" checkout -q -b A; c "$T/v1" a1 >/dev/null
git -C "$T/v1" checkout -q -b B "$V1R"; addsec v1 y "$S3" "$OWN_URL" >/dev/null
git -C "$T/v1" checkout -q A; git -C "$T/v1" merge -q --no-ff -s ours -m merge B >/dev/null
addsec v1 z "$S1" "$OWN_URL" >/dev/null
run v1 "$V1O" HEAD
chk "V1 both-kind entry survives a name-only copy, z ambiguous" 'printf "%s\n" "$out" | grep -q "^BLIND own ?=>z - $S1 commit=.* reason=readd-ambiguous"'
chk "V1 z not compared with y alone" '! printf "%s\n" "$out" | grep -q "y=>z"'
# V6: every same-name candidate is consumed (rp7 lines remove sub at S2 and S3, re-added at S3);
# w (same URL) added afterwards finds nothing left: ADDED, exit 0
P6O=$(mk7 v6); addsec v6 sub "$S3" "$OWN_URL" >/dev/null; addsec v6 w "$S1" "$OWN_URL" >/dev/null
run v6 "$P6O" HEAD
chk "V6 all same-name entries consumed, exits 0" '[ "$rc" -eq 0 ]'
chk "V6 w reported ADDED" 'printf "%s\n" "$out" | grep -q "^ADDED own w $S1 commit=[0-9a-f]"'
# V7: a consumed entry never reaches a child set. Base removes x (S3); line A re-adds x (consumed);
# line B adds y (same URL, x name-only); merge on B (-s ours, A second parent); x re-added at S1 must
# still match B's name-only entry: NONFF
mkparent v7 "$S1" "$OWN_URL" >/dev/null; addsec v7 x "$S3" "$OWN_URL" >/dev/null; V7O=$(git -C "$T/v7" rev-parse HEAD)
rmsec v7 x >/dev/null; V7R=$(git -C "$T/v7" rev-parse HEAD)
git -C "$T/v7" checkout -q -b A; addsec v7 x "$S3" "$OWN_URL" >/dev/null
git -C "$T/v7" checkout -q -b B "$V7R"; addsec v7 y "$S3" "$OWN_URL" >/dev/null
git -C "$T/v7" merge -q --no-ff -s ours -m merge A >/dev/null
addsec v7 x "$S1" "$OWN_URL" >/dev/null
run v7 "$V7O" HEAD
chk "V7 consumed entry not propagated, re-add older exits 1" '[ "$rc" -eq 1 ]'
chk "V7 x compared with the name-only entry" 'printf "%s\n" "$out" | grep -q "^NONFF own x=>x $S3 $S1 commit="'
# V3: two lines remove sub at the SAME pin; the re-add is compared once per distinct pin (output only:
# a duplicate comparison has the same verdict, it only repeats the line and the count)
mkparent v3 "$S1" "$OWN_URL" >/dev/null; V3O=$(git -C "$T/v3" rev-parse HEAD)
git -C "$T/v3" checkout -q -b A; move v3 "$S3" >/dev/null; rmsec v3 sub >/dev/null
git -C "$T/v3" checkout -q -b B "$V3O"; move v3 "$S3" >/dev/null; c "$T/v3" b2 >/dev/null; rmsec v3 sub >/dev/null
git -C "$T/v3" checkout -q A; git -C "$T/v3" merge -q --no-ff -m merge B >/dev/null
addsec v3 sub "$S3" "$OWN_URL" >/dev/null
run v3 "$V3O" HEAD
chk "V3 same pin from two lines exits 0" '[ "$rc" -eq 0 ]'
chk "V3 one comparison per distinct pin" '[ "$(printf "%s\n" "$out" | grep -c "^FF own sub=>sub $S3 $S3 commit=")" -eq 1 ]'

# 66. Round 9 (F8-1): the same path re-added under ANOTHER name after its removal. When one store holds
# both pins the history is shared: compare. When no store resolves either pin nothing can be told:
# compare (BLIND). Only when a store resolves one pin but none holds both is it a new submodule (n6).
# addas REPO NAME PATH PIN [URL]: gitlink at PATH pinned to PIN under section NAME, commit
addas() { git -C "$T/$1" update-index --add --cacheinfo 160000,"$4","$3"
  git -C "$T/$1" config -f .gitmodules "submodule.$2.path" "$3"
  [ -n "${5:-}" ] && git -C "$T/$1" config -f .gitmodules "submodule.$2.url" "$5"
  git -C "$T/$1" add .gitmodules; git -C "$T/$1" commit -q -m "add $2 at $3"; git -C "$T/$1" rev-parse HEAD; }
for f81 in "f8r1:sub2:git@gitlab.com:vasic-digital/sub.git" "f8r2:zz:../sub.git" "f8r3:zz:"; do
  r="${f81%%:*}"; rest="${f81#*:}"; nm="${rest%%:*}"; u="${rest#*:}"
  F8O=$(mkparent "$r" "$S1" "$OWN_URL"); move "$r" "$S3" >/dev/null; rmsec "$r" sub >/dev/null
  addas "$r" "$nm" sub "$S2" "$u" >/dev/null
  run "$r" "$F8O" HEAD
  chk "F8-1 $r same path, other name ($nm, URL '$u'), re-added older exits 1" '[ "$rc" -eq 1 ]'
  chk "F8-1 $r compared with the removed pin" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S2 commit="'
done
# R7: a merge -s ours keeps the removal of line main over the move to S3 on line a; the path is then
# re-added under another name at S2: the merge's own diff against a is the removal at S3, so FAIL
F87O=$(mkparent f8r7 "$S1" "$OWN_URL")
git -C "$T/f8r7" checkout -q -b a; move f8r7 "$S3" >/dev/null
git -C "$T/f8r7" checkout -q main; rmsec f8r7 sub >/dev/null
git -C "$T/f8r7" merge -q --no-ff -s ours -m merge a >/dev/null
addas f8r7 q sub "$S2" 'git@gitlab.com:vasic-digital/sub.git' >/dev/null
run f8r7 "$F87O" HEAD
chk "F8-1 R7 merge-dropped removal, re-added older under another name exits 1" '[ "$rc" -eq 1 ]'
chk "F8-1 R7 compared with the pin removed on the merged line" 'printf "%s\n" "$out" | grep -q "^NONFF own sub=>sub $S3 $S2 commit="'
# R12: added inside the range at S3 under name a, removed, re-added under name b at S1; no local store
# holds either pin: never a silent PASS, BLIND
git init -q -b main "$T/f8r12"; F812O=$(c "$T/f8r12" root)
addas f8r12 a sub "$S3" "$OWN_URL" >/dev/null
git -C "$T/f8r12" rm -q --cached sub; git -C "$T/f8r12" config -f .gitmodules --remove-section submodule.a
git -C "$T/f8r12" add .gitmodules; git -C "$T/f8r12" commit -q -m "remove a"
addas f8r12 b sub "$S1" 'git@gitlab.com:vasic-digital/sub.git' >/dev/null
chk "F8-1 R12 precondition: sub absent from the gitlink tree at HEAD~1" '[ -z "$(git -C "$T/f8r12" ls-tree HEAD~1 sub)" ]'
run f8r12 "$F812O" HEAD
chk "F8-1 R12 no store resolves either pin, exits 2" '[ "$rc" -eq 2 ]'
chk "F8-1 R12 compared, BLIND" 'printf "%s\n" "$out" | grep -q "^BLIND own sub=>sub $S3 $S1 commit=.* reason=pin-not-in-local-object-store"'
# R11 control: the same rename and backward move in ONE commit is a gitlink modification: FAIL
F811O=$(mkparent f8r11 "$S3" "$OWN_URL")
git -C "$T/f8r11" config -f .gitmodules --remove-section submodule.sub
addas f8r11 b sub "$S1" '../x.git' >/dev/null
run f8r11 "$F811O" HEAD
chk "F8-1 R11 control, rename and move in one commit exits 1" '[ "$rc" -eq 1 ]'
chk "F8-1 R11 control compared in place" 'printf "%s\n" "$out" | grep -q "^NONFF own sub $S3 $S1 commit="'
# the mirror case: the removed pin is in no store, the new one is: another repository, ADDED
F8BO=$(mkparent f8n6b "$S1" "$OWN_URL"); addsec f8n6b p "$Q2" 'git@github.com:vasic-digital/unrel.git' >/dev/null
rmsec f8n6b p >/dev/null; addas f8n6b q p "$S2" 'git@gitlab.com:vasic-digital/sub.git' >/dev/null
run f8n6b "$F8BO" HEAD
chk "F8-1 path reused, only the new pin resolvable, exits 0" '[ "$rc" -eq 0 ]'
chk "F8-1 path reused, only the new pin resolvable, ADDED" 'printf "%s\n" "$out" | grep -q "^ADDED own p $S2 commit=[0-9a-f]"'
# forward under another name at the same path: compared, FF, PASS (the new rule is not a blanket refusal)
F8FO=$(mkparent f8fw "$S1" "$OWN_URL"); rmsec f8fw sub >/dev/null
addas f8fw zz sub "$S3" 'git@gitlab.com:vasic-digital/sub.git' >/dev/null
run f8fw "$F8FO" HEAD
chk "F8-1 same path, other name, re-added later exits 0" '[ "$rc" -eq 0 ]'
chk "F8-1 forward re-add compared, FF" 'printf "%s\n" "$out" | grep -q "^FF own sub=>sub $S1 $S3 commit="'

# 12. usage errors
bash "$GATE" >/dev/null 2>&1; rc=$?
chk "no arguments exits 64" '[ "$rc" -eq 64 ]'
(cd "$T/fwd" && bash "$GATE" nosuchref HEAD >/dev/null 2>&1); rc=$?
chk "unknown ref exits 64" '[ "$rc" -eq 64 ]'

exit $fail
