#!/usr/bin/env bash
# WF-R7 test (TDD, constitution 11.4.224): the cases of the independent re-review WP-04-T046-r2 (NO-GO).
#   F-01 (BLOCKING) everything between the writer and the stored commit object can alter the message the writer composed (commit.cleanup, core.commentChar, core.commentString, hooks, an
#        encoding, an amend): a held commit then loses its Awaits-Review line and is PUSHED. Class matrix: every configuration or hook that alters the stored message, through commit_recursive.sh
#        AND the merge commit of integrate_merge.sh; every configuration that alters how a message is READ (log.showSignature, i18n.logOutputEncoding, refs/replace).
#   F-04 one parser: a block with 0x1e is malformed for the Foreign-Commit lookup exactly as for cpa_trailer.   F-05 near-miss reserved keys inside the block are malformed.
#   F-06 Foreign-Commit lines of a malformed earlier message do not count (reviewer mutant N3).
# Throwaway repositories and LOCAL BARE remotes only; no real remote, no network, no credential; the global git config is a file under the scratch tree.
# Usage: bash scripts/repo/tests/test_wp04k.sh      Env: HS=<scripts/repo directory under test> (the RED run points it at the pre-fix copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; HS="${HS:-scripts/repo}"; case "$HS" in /*) ;; *) HS="$D0/$HS" ;; esac
PR="$HS/push_recursive.sh"; IM="$HS/integrate_merge.sh"; CR="$HS/commit_recursive.sh"; H="$PR"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04k_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) lib_safe=$(sha256sum "$HS/lib_safe.sh" 2>/dev/null | cut -c1-64) commit_recursive=$(sha256sum "$CR" 2>/dev/null | cut -c1-64) integrate_merge=$(sha256sum "$IM" 2>/dev/null | cut -c1-64)"
# shellcheck source=/dev/null
. "$HS/lib_safe.sh" 2>/dev/null
ID=20261005T000000Z-1-aaaa; A="$T/audit"; RUND="$A/$ID"; V=specs/ev/reviews/v.json; M="$T/m"; EVR=ev
GC="$T/gitconfig"; : > "$GC"
gcfg() { printf '%b' "$1" > "$GC"; export GIT_CONFIG_GLOBAL="$GC"; }
gcfg_off() { export GIT_CONFIG_GLOBAL=/dev/null; }
fresh() {
  gcfg_off; rm -rf "$M" "$T/b" "$A" "$T/hooks" "$T/c1"; mkdir -p "$T/b" "$RUND" "$T/hooks"; : > "$RUND/commits.tsv"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$M"; printf '/.audit/\n' > "$M/.gitignore"; mkdir -p "$M/scripts/repo" "$M/$EVR"; echo base > "$M/base.txt"; echo f0 > "$M/f.txt"; echo t0 > "$M/scripts/repo/check_classes.tsv"; commit_all "$M" init
  INIT="$(git -C "$M" rev-parse HEAD)"
  for r in r1 r2; do git -C "$M" remote add $r "$T/b/$r.git"; git -C "$M" push -q $r main; done
}
tip() { git -C "$T/b/$1.git" rev-parse main 2>/dev/null || echo none; }
head_() { git -C "$M" rev-parse HEAD; }
rows() { wc -l < "$RUND/commits.tsv" | tr -d ' '; }
# cr <message> [extra commit_recursive args] -- <file>: commit one new file
cr() { local m="$1"; shift; "$CR" --repo "$M" --run-dir "$RUND" --run-id "$ID" --message "$m" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
newfile() { echo "$1 $RANDOM" > "$M/$1"; }
prun() { "$PR" --main-root "$M" --branch main --run-dir "$RUND" >"$T/pout" 2>"$T/perr"; PRC=$?; }
printf 'f1.txt\t%s\n' "$V" > "$T/held.tsv"
# ---- F-01 matrix 1: configuration that alters the STORED message. A held commit is made under this global config; it must come out with its trailers intact and stay HELD ----
held_cfg() { # held_cfg <label> <global config text> [expected exit of the commit step: 0]
  fresh; gcfg "$2"; newfile f1.txt; cr "Add f1" --held-from "$T/held.tsv" -- f1.txt
  eq "F-01 [$1] the commit step: $3" "$RC" "${3:-0}"
  if [ "${3:-0}" = 0 ]; then
    eq "F-01 [$1] the stored commit keeps its Awaits-Review line" "$(cpa_trailer "$M" HEAD Awaits-Review 2>/dev/null)" "$V"
    eq "F-01 [$1] the stored commit keeps its CPA-Run line" "$(cpa_trailer "$M" HEAD CPA-Run 2>/dev/null)" "$ID"
  fi
  prun
  eq "F-01 [$1] push_recursive does not publish the held commit: 14 (hold) or 20 (unrecorded)" "$(case $PRC in 14|20) echo ok;; *) echo "$PRC";; esac)" ok
  eq "F-01 [$1] the remote still holds only the base commit" "$(tip r1)" "$INIT"
}
held_cfg "commit.cleanup=strip + core.commentChar=A (the review's needle)" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = A\n' 0
held_cfg "core.commentChar=C (the CPA-Run line)" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = C\n' 0
held_cfg "core.commentChar=D (Deferred-Gates)" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = D\n' 0
held_cfg "core.commentString=Awaits-Review" '[commit]\n\tcleanup = strip\n[core]\n\tcommentString = Awaits-Review\n' 0
held_cfg "core.commentChar=auto" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = auto\n' 0
held_cfg "commit.cleanup=scissors" '[commit]\n\tcleanup = scissors\n' 0
held_cfg "commit.cleanup=whitespace" '[commit]\n\tcleanup = whitespace\n' 0
held_cfg "commit.cleanup=default" '[commit]\n\tcleanup = default\n' 0
printf '# template line\nAwaits-Review: template\n' > "$T/tmpl"
held_cfg "commit.template" "[commit]\\n\\ttemplate = $T/tmpl\\n" 0
held_cfg "trailer.* configuration" '[trailer]\n\tifexists = replace\n\twhere = start\n[trailer "awaits"]\n\tkey = Awaits-Review\n' 0
held_cfg "log.showSignature + i18n.logOutputEncoding=UTF-16 (the READ side)" '[log]\n\tshowSignature = true\n[i18n]\n\tlogOutputEncoding = UTF-16\n' 0
held_cfg "i18n.commitEncoding=ISO-8859-1 with an ASCII message" '[i18n]\n\tcommitEncoding = ISO-8859-1\n' 0
held_cfg "merge.log + merge.branchdesc" '[merge]\n\tlog = 50\n\tbranchdesc = true\n' 0
# a non-ASCII message under a non-UTF-8 commit encoding is stored with an encoding header the readers cannot honour: refused, no row
fresh; gcfg '[i18n]\n\tcommitEncoding = ISO-8859-1\n'; newfile f1.txt; cr $'Add f1 \303\251' --held-from "$T/held.tsv" -- f1.txt
eq "F-01 [commitEncoding + non-ASCII message] refused: 20" "$RC" 20; has "F-01 [commitEncoding + non-ASCII message] names commit_message_altered" "$(cat "$T/err")" commit_message_altered; eq "F-01 [commitEncoding + non-ASCII message] no row" "$(rows)" 0
prun; eq "F-01 [commitEncoding + non-ASCII message] the unrecorded commit is not pushed: 20" "$PRC" 20; eq "F-01 [commitEncoding + non-ASCII message] remote untouched" "$(tip r1)" "$INIT"
# deferred gates are kept too (commentChar=D used to drop the line)
fresh; "$HS/record_deferral.sh" --run-dir "$RUND" --flag SKIP_LONG --reason t >/dev/null 2>&1; gcfg '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = D\n'; newfile u.txt; cr "Add u" -- u.txt
eq "F-01 [commentChar=D] an unheld commit with a deferral: 0" "$RC" 0; eq "F-01 [commentChar=D] the Deferred-Gates line is stored" "$(cpa_trailer "$M" HEAD Deferred-Gates 2>/dev/null)" SKIP_LONG
# ---- F-01 matrix 2: hooks and amends that alter the stored message after the writer composed it: the read-back refuses, no row is written ----
hook_case() { # hook_case <label> <hook name> <hook body>
  fresh; printf '#!/bin/sh\n%s\n' "$3" > "$T/hooks/$2"; chmod +x "$T/hooks/$2"; git -C "$M" config core.hooksPath "$T/hooks"; newfile f1.txt; rm -f "$T/hook_marker"
  timeout -k 2 "${HOOK_CASE_TIMEOUT:-30}" "$CR" --repo "$M" --run-dir "$RUND" --run-id "$ID" --message "Add f1" --held-from "$T/held.tsv" -- f1.txt >"$T/out" 2>"$T/err"; RC=$?
  { [ "$RC" -eq 124 ] || [ "$RC" -eq 137 ]; } && echo "FAIL hook_case [$1]: the hook-driven commit did not finish in ${HOOK_CASE_TIMEOUT:-30}s (recursive hook?)" >&2
  eq "F-01 [$1] refused: 20" "$RC" 20; has "F-01 [$1] names commit_message_altered" "$(cat "$T/err")" commit_message_altered; eq "F-01 [$1] no row was written" "$(rows)" 0
  prun; eq "F-01 [$1] the altered commit is unrecorded, nothing is pushed: 20" "$PRC" 20; eq "F-01 [$1] remote untouched" "$(tip r1)" "$INIT"
}
hook_case "commit-msg hook removes the Awaits-Review line" commit-msg 'grep -v "^Awaits-Review:" "$1" > "$1.n"; mv "$1.n" "$1"'
hook_case "prepare-commit-msg hook appends a line" prepare-commit-msg 'echo "Signed-off-by: hook" >> "$1"'
# RECURSION GUARD (host hang 2026-10-08): `git commit --amend` fires post-commit again, so an unguarded amending hook recurses without bound. The hook amends ONCE: it exits at once if its marker file exists (path baked in, no env reliance).
hook_case "post-commit hook amends the message" post-commit "[ -e '$T/hook_marker' ] && exit 0; : > '$T/hook_marker'; git commit -q --amend --no-verify -m rewritten >/dev/null 2>&1 || true"
fresh; printf '#!/bin/sh\nexit 0\n' > "$T/hooks/commit-msg"; chmod +x "$T/hooks/commit-msg"; git -C "$M" config core.hooksPath "$T/hooks"; newfile f1.txt; cr "Add f1" --held-from "$T/held.tsv" -- f1.txt
eq "F-01 golden-false: a commit-msg hook that changes nothing: 0" "$RC" 0; eq "F-01 golden-false: one row" "$(rows)" 1
fresh; export GIT_AUTHOR_NAME="Some One" GIT_COMMITTER_NAME="Other One" GIT_AUTHOR_DATE="2020-01-01T00:00:00Z"; newfile f1.txt; cr "Add f1" --held-from "$T/held.tsv" -- f1.txt
eq "F-01 golden-false: author and committer variables do not alter the message: 0" "$RC" 0; eq "F-01 golden-false: the hold is stored" "$(cpa_trailer "$M" HEAD Awaits-Review)" "$V"; export GIT_AUTHOR_NAME=t GIT_COMMITTER_NAME=t; unset GIT_AUTHOR_DATE
# ---- F-01 matrix 3: how a message is READ: a refs/replace entry shows another message for the commit ----
fresh; newfile f1.txt; cr "Add f1" --held-from "$T/held.tsv" -- f1.txt; held="$(head_)"
printf 'plain replacement\n' > "$T/rmsg"; repl="$(git -C "$M" commit-tree "$(git -C "$M" rev-parse 'HEAD^{tree}')" -p "$(git -C "$M" rev-parse 'HEAD^')" -F "$T/rmsg")"; git -C "$M" replace "$held" "$repl"
eq "F-01 a refs/replace entry for the held commit does not change what the readers see: hold" "$(cpa_trailer "$M" "$held" Awaits-Review 2>/dev/null)" "$V"
eq "F-01 a refs/replace entry does not change the run id either" "$(cpa_runid "$M" "$held" 2>/dev/null)" "$ID"
# ---- F-01 matrix 4: the merge commit of integrate_merge.sh ----
imfresh() { # a local held commit (made by commit_recursive under no special config), then a remote change: the merge must be held and carry its trailers
  fresh; newfile f1.txt; cr "Add f1" --held-from "$T/held.tsv" -- f1.txt; git clone -q "$T/b/r1.git" "$T/c1"; git -C "$T/c1" config user.email t@t; git -C "$T/c1" config user.name t
  echo r > "$T/c1/r.txt"; git -C "$T/c1" add -A; git -C "$T/c1" commit -q -m "remote change"; git -C "$T/c1" push -q origin main
}
imrun() { "$IM" --root "$M" --branch main --run-dir "$RUND" --ev "$EVR" >"$T/iout" 2>"$T/ierr"; IRC=$?; }
merge_cfg() { # merge_cfg <label> <global config text>
  imfresh; gcfg "$2"; imrun
  eq "F-01 merge [$1]: 0" "$IRC" 0
  eq "F-01 merge [$1]: the merge commit keeps its CPA-Run line" "$(cpa_trailer "$M" HEAD CPA-Run 2>/dev/null)" "$ID"
  eq "F-01 merge [$1]: the merge commit keeps its Awaits-Review line" "$(cpa_trailer "$M" HEAD Awaits-Review 2>/dev/null)" "$EVR/reviews/CPA-merge-$ID.json"
}
merge_cfg "commit.cleanup=strip + core.commentChar=A" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = A\n'
merge_cfg "core.commentChar=C" '[commit]\n\tcleanup = strip\n[core]\n\tcommentChar = C\n'
merge_cfg "core.commentString=Merge" '[commit]\n\tcleanup = strip\n[core]\n\tcommentString = Merge\n'
merge_cfg "merge.log + merge.autoEdit=false" '[merge]\n\tlog = 50\n\tautoEdit = false\n'
imfresh; printf '#!/bin/sh\ngrep -v "^Awaits-Review:" "$1" > "$1.n"; mv "$1.n" "$1"\n' > "$T/hooks/commit-msg"; chmod +x "$T/hooks/commit-msg"; git -C "$M" config core.hooksPath "$T/hooks"; imrun
eq "F-01 merge [commit-msg hook removes the Awaits-Review line] refused: 20" "$IRC" 20; has "F-01 merge [hook] names commit_message_altered" "$(cat "$T/ierr")" commit_message_altered
eq "F-01 merge [hook] no merge row was written (one row: the local commit)" "$(rows)" 1
# ---- F-04 / F-05 / F-06 the parser ----
rawc() { local tree c; printf -- "$1" > "$T/rawmsg"; tree="$(git -C "$M" rev-parse 'HEAD^{tree}')"; c="$(git -C "$M" commit-tree "$tree" -p HEAD -F "$T/rawmsg")"; git -C "$M" update-ref HEAD "$c"; printf '%s' "$c"; }
fresh; F1=1111111111111111111111111111111111111111; F3=3333333333333333333333333333333333333333
nm() { # nm <label> <printf format of the message>: a reserved key in a near-miss spelling INSIDE the block makes the block malformed (status 3)
  local c; c="$(rawc "$2")"; cpa_trailer "$M" "$c" Awaits-Review >/dev/null 2>&1; eq "F-05 [$1] malformed (3)" "$?" 3
}
nm "lower-case key" "s\n\nCPA-Run: $ID\nawaits-review: v.json\n"
nm "space before the colon" "s\n\nCPA-Run: $ID\nAwaits-Review : v.json\n"
nm "no space after the colon" "s\n\nCPA-Run: $ID\nAwaits-Review:v.json\n"
nm "no-break space after the colon" "s\n\nCPA-Run: $ID\nAwaits-Review:\302\240v.json\n"
nm "mixed-case duplicate" "s\n\nCPA-Run: $ID\nAwaits-Review: a.json\nAWAITS-REVIEW: b.json\n"
nm "Foreign-Commit in another case" "s\n\nCPA-Run: $ID\nforeign-commit: $F1\n"
nm "CPA-Run in another case after the opener" "s\n\nCPA-Run: $ID\ncpa-run: $ID\n"
c="$(rawc "s\n\nCPA-Run: $ID\nCo-Authored-By: X <x@y>\nSigned-off-by: Y <y@y>\nAwaits-Review: v.json\n")"; eq "F-05 golden-false: other keys in the block are not near misses; the hold is read" "$(cpa_trailer "$M" "$c" Awaits-Review 2>/dev/null; echo "/$?")" "v.json/0"
# F-04: 0x1e inside a block
c="$(rawc "s\n\nCPA-Run: $ID\nAwaits-Review: v\036.json\nForeign-Commit: $F1\n")"
cpa_trailer "$M" "$c" Awaits-Review >/dev/null 2>&1; eq "F-04 a block holding 0x1e is malformed for cpa_trailer (3)" "$?" 3
cpa_foreign_listed "$M" HEAD "$F1"; eq "F-04 a block holding 0x1e is malformed for cpa_foreign_listed too: its Foreign-Commit line does not count (1)" "$?" 1
c="$(rawc "free \036 text\n\nCPA-Run: $ID\nForeign-Commit: $F3\n")"
cpa_trailer "$M" "$c" CPA-Run >/dev/null 2>&1; eq "F-04 golden-false: 0x1e in the free text is no problem for cpa_trailer (0)" "$?" 0
cpa_foreign_listed "$M" HEAD "$F3"; eq "F-04 golden-false: 0x1e in the free text: the Foreign-Commit line counts (0)" "$?" 0
# F-06: the Foreign-Commit lines of a malformed earlier message do not count
F6=6666666666666666666666666666666666666666
c="$(rawc "earlier\n\nCPA-Run: $ID\nForeign-Commit: $F6\nAwaits-Review: a\nAwaits-Review: b\n")"
cpa_foreign_listed "$M" HEAD "$F6"; eq "F-06 a Foreign-Commit line of a malformed earlier message does not count (1)" "$?" 1
c="$(rawc "earlier\n\nCPA-Run: $ID\nForeign-Commit: $F6\n")"; cpa_foreign_listed "$M" HEAD "$F6"; eq "F-06 golden-false: the same line in a well-formed message counts (0)" "$?" 0
# ======================================================================================================================================
fin
