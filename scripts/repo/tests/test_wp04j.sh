#!/usr/bin/env bash
# WF-R6 test (TDD, constitution 11.4.224): the cases of the independent review WP-04-T046 (NO-GO) that no earlier test covered.
#   R-01  ONE trailer parser for every reader of a CPA commit message (lib_safe.sh cpa_trailer / cpa_runid / cpa_hold_state / cpa_foreign_listed / cpa_msg_check): a `---` line in
#         the message must no more hide an Awaits-Review hold from push_recursive than from the CPA-commit predicate. CLASS matrix: every message shape that desyncs two parsers
#         (`---`, blank-line variants, folded trailers, CRLF, duplicate trailers, a trailer block without the CPA-Run line, a reserved key outside the block, a divider inside the block).
#   R-02  a held merge commit AFTER unheld commits (cut > 0) still withholds everything of the range, and the condition that decides it is not hidden by a second assignment.
#   R-06  repo_config_gate reads the status of `git config --list`; integrate_merge --resolution writes the 9.2 backup BEFORE its trial merge.
#   R-07  a verdict whose findings carry the constitution 11.4.235(D) finding_layer set also validates against contracts/review-verdict.schema.json.
# Throwaway repositories and LOCAL BARE remotes only; no real remote, no network, no credential.
# Usage: bash scripts/repo/tests/test_wp04j.sh      Env: HS=<scripts/repo directory under test> (the RED run points it at the pre-fix copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; HS="${HS:-scripts/repo}"; case "$HS" in /*) ;; *) HS="$D0/$HS" ;; esac
PR="$HS/push_recursive.sh"; IM="$HS/integrate_merge.sh"; CR="$HS/commit_recursive.sh"; H="$PR"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04j_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
SCHEMA=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
SCHEMA_FILE="${SCHEMA_FILE:-$D0/$SCHEMA}"   # the mutation driver points it at a mutated copy
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) lib_safe=$(sha256sum "$HS/lib_safe.sh" 2>/dev/null | cut -c1-64) push_recursive=$(sha256sum "$PR" 2>/dev/null | cut -c1-64) integrate_merge=$(sha256sum "$IM" 2>/dev/null | cut -c1-64) commit_recursive=$(sha256sum "$CR" 2>/dev/null | cut -c1-64)"
ID=20261005T000000Z-1-aaaa; ID2=20261005T000000Z-2-bbbb
A="$T/audit"; mkdir -p "$A/$ID" "$A/$ID2"

# ======================================================================================================================================
# R-01 class matrix 1: the shared parser, every message shape. Raw commit objects (git commit-tree keeps the bytes; `git commit` would clean CRs and blank lines up)
# ======================================================================================================================================
P="$T/p"; mkrepo "$P"; echo base > "$P/f"; commit_all "$P" init
# shellcheck source=/dev/null
. "$HS/lib_safe.sh" 2>/dev/null
have() { type -t "$1" >/dev/null 2>&1 && echo yes || echo no; }
eq "R-01 lib_safe.sh has the shared parser cpa_trailer" "$(have cpa_trailer)" yes
eq "R-01 lib_safe.sh has the shared message check cpa_msg_check" "$(have cpa_msg_check)" yes
eq "R-01 lib_safe.sh has the shared Foreign-Commit lookup cpa_foreign_listed" "$(have cpa_foreign_listed)" yes
eq "R-01 lib_safe.sh has the shared hold reader cpa_hold_state" "$(have cpa_hold_state)" yes
raw() { # raw <repo> <printf format> : a commit object with exactly these message bytes on top of HEAD (same tree); prints its sha
  local d="$1" tree c; printf -- "$2" > "$T/rawmsg"; tree="$(git -C "$d" rev-parse 'HEAD^{tree}')"; c="$(git -C "$d" commit-tree "$tree" -p HEAD -F "$T/rawmsg")"; git -C "$d" update-ref HEAD "$c"; printf '%s' "$c"
}
F1=1111111111111111111111111111111111111111; F2=2222222222222222222222222222222222222222
# case | message | want runid ("-" = none) | want Awaits-Review "<status>:<value>"
shapes() { cat <<EOF
plain block|s\n\nCPA-Run: $ID\nAwaits-Review: v.json\n|$ID|0:v.json
divider line in the body|s\n---\nnotes\n\nCPA-Run: $ID\nAwaits-Review: v.json\n|$ID|0:v.json
divider with text after it|s\n--- a/x\n+++ b/x\n\nCPA-Run: $ID\nAwaits-Review: v.json\n|$ID|0:v.json
whitespace-only blank lines|s\n \t\nbody\n  \n\nCPA-Run: $ID\nAwaits-Review: v.json\n|$ID|0:v.json
trailing blank lines|s\n\nCPA-Run: $ID\nAwaits-Review: v.json\n\n\n|$ID|0:v.json
leading blank lines|\n\ns\n\nCPA-Run: $ID\n|$ID|0:
block only|CPA-Run: $ID\n|$ID|0:
attribution lines after the trailers|s\n\nCPA-Run: $ID\nCo-Authored-By: X <x@y>\nplain attribution text\n|$ID|0:
CRLF line endings|s\r\n\r\nCPA-Run: $ID\r\nAwaits-Review: v.json\r\n|-|3:
CR inside a trailer value|s\n\nCPA-Run: $ID\nAwaits-Review: v.json\r\n|-|3:
control character in the block|s\n\nCPA-Run: $ID\nAwaits-Review: v\001.json\n|-|3:
folded trailer|s\n\nCPA-Run: $ID\nAwaits-Review: v.json\n  continued\n|-|3:
tab-folded trailer|s\n\nCPA-Run: $ID\nAwaits-Review: v.json\n\tcontinued\n|-|3:
duplicate Awaits-Review|s\n\nCPA-Run: $ID\nAwaits-Review: a.json\nAwaits-Review: b.json\n|-|3:
duplicate CPA-Run|s\n\nCPA-Run: $ID\nCPA-Run: $ID2\n|-|3:
duplicate Deferred-Gates|s\n\nCPA-Run: $ID\nDeferred-Gates: A\nDeferred-Gates: B\n|-|3:
divider line inside the block|s\n\nCPA-Run: $ID\n---\nAwaits-Review: v.json\n|-|3:
CPA-Run in an earlier paragraph, hold alone in the last|CPA-Run: $ID\n\nAwaits-Review: v.json\n|-|1:
Awaits-Review before the block|s\nAwaits-Review: x.json\n\nCPA-Run: $ID\n|-|3:
indented reserved key before the block|s\n  Awaits-Review: x.json\n\nCPA-Run: $ID\n|-|3:
lower-case reserved key before the block|s\ncpa-run: $ID2\n\nCPA-Run: $ID\n|-|3:
lower-case key as the block opener|cpa-run: $ID\n|-|1:
no trailer at all|just a subject\n|-|1:
two Foreign-Commit lines|s\n\nCPA-Run: $ID\nForeign-Commit: $F1\nForeign-Commit: $F2\n|$ID|0:
EOF
}
while IFS='|' read -r name fmt wantid wantaw; do
  [ -n "$name" ] || continue
  c="$(raw "$P" "$fmt")"
  gotid="$(cpa_runid "$P" "$c" 2>/dev/null)" || gotid="-"; [ -n "$gotid" ] || gotid="-"
  eq "R-01 cpa_runid [$name]" "$gotid" "$wantid"
  v="$(cpa_trailer "$P" "$c" Awaits-Review 2>/dev/null)"; rc=$?
  eq "R-01 cpa_trailer Awaits-Review [$name]" "$rc:$v" "$wantaw"
done < <(shapes)
c="$(raw "$P" "s\n\nCPA-Run: $ID\nForeign-Commit: $F1\nForeign-Commit: $F2\n")"
cpa_foreign_listed "$P" HEAD "$F1"; eq "R-01 cpa_foreign_listed finds the first Foreign-Commit line" "$?" 0
cpa_foreign_listed "$P" HEAD "$F2"; eq "R-01 cpa_foreign_listed finds the second Foreign-Commit line" "$?" 0
cpa_foreign_listed "$P" HEAD "3333333333333333333333333333333333333333"; eq "R-01 cpa_foreign_listed: an unnamed sha is not listed (1)" "$?" 1
c="$(raw "$P" "s\n---\nnotes\n\nCPA-Run: $ID\nForeign-Commit: $F1\n")"
cpa_foreign_listed "$P" HEAD "$F1"; eq "R-01 cpa_foreign_listed reads a Foreign-Commit line behind a divider in the body" "$?" 0
c="$(raw "$P" "text only\n\nForeign-Commit: 4444444444444444444444444444444444444444\n")"
cpa_foreign_listed "$P" HEAD "4444444444444444444444444444444444444444"; eq "R-01 a Foreign-Commit line in a message with no CPA block is no listing (1)" "$?" 1
cpa_foreign_listed "$T/does-not-exist" HEAD "$F1"; eq "R-01 cpa_foreign_listed: a failing git is status 2, never 'not listed'" "$?" 2
mkdir -p "$T/not-a-repo"; cpa_foreign_listed "$T/not-a-repo" HEAD "$F1"; eq "R-01 cpa_foreign_listed in a directory that is no repository: 2" "$?" 2
# cpa_hold_state: a message with no CPA block that carries an Awaits-Review line counts as held (the merge path holds more, never less)
c="$(raw "$P" "hand commit\nAwaits-Review: ev/reviews/V.json\n")"
eq "R-01 cpa_hold_state: a hand commit with an Awaits-Review line is held" "$(cpa_hold_state "$P" "$c" 2>/dev/null; echo "/$?")" "?/0"
c="$(raw "$P" "hand commit\n")"; eq "R-01 cpa_hold_state: a hand commit without one is not held" "$(cpa_hold_state "$P" "$c" 2>/dev/null; echo "/$?")" "/0"
c="$(raw "$P" "s\n\nCPA-Run: $ID\nAwaits-Review: a\nAwaits-Review: b\n")"; cpa_hold_state "$P" "$c" >/dev/null 2>&1; eq "R-01 cpa_hold_state: a malformed block cannot be judged (3)" "$?" 3
c="$(raw "$P" "s\n---\nx\n\nCPA-Run: $ID\nAwaits-Review: ev/reviews/V.json\n")"
eq "R-01 cpa_hold_state: a hold behind a divider is read" "$(cpa_hold_state "$P" "$c" 2>/dev/null; echo "/$?")" "ev/reviews/V.json/0"

# ======================================================================================================================================
# R-01 class matrix 2: the message check that stands in front of the writers (S0 of the launcher and commit_recursive.sh)
# ======================================================================================================================================
chk() { # chk <label> <printf format> <want: ok|bad>
  local m; m="$(printf -- "$2")"; local r rc
  r="$(cpa_msg_check "$m" 2>/dev/null)"; rc=$?
  if [ "$3" = ok ]; then eq "R-01 cpa_msg_check accepts [$1]" "$rc" 0; else eq "R-01 cpa_msg_check refuses [$1]" "$rc" 1; [ -n "$r" ] && ok "R-01 [$1] names a reason ($r)" || bad "R-01 [$1] names no reason"; fi
}
chk "divider line" 'a\n---\nb' bad
chk "divider as the only line" '---' bad
chk "longer divider" 'a\n----\nb' bad
chk "divider with text" 'a\n--- a/x\nb' bad
chk "CR" 'a\r\nb' bad
chk "control character" 'a\001b' bad
chk "CPA-Run line" 'x\nCPA-Run: y' bad
chk "lower-case cpa-run line" 'x\ncpa-run: y' bad
chk "indented Awaits-Review" 'x\n  Awaits-Review: z' bad
chk "Deferred-Gates with a space before the colon" 'x\nDeferred-Gates : q' bad
chk "Foreign-Commit without a space" 'x\nForeign-Commit:y' bad
chk "three dashes inside a line" 'a --- b' ok
chk "blank-line runs" 'a\n\n\nb' ok
chk "whitespace-only lines" 'a\n \t\nb\n' ok
chk "two dashes and a space" 'a\n-- \nb' ok
chk "two dashes" 'a\n--\nb' ok
chk "a non-reserved Key: value line" 'Fix: thing\nSigned-off-by: me' ok
chk "a tab inside a line" 'a\tb' ok
chk "a reserved word inside a sentence" 'the CPA-Run: id is made by the script' ok

# ======================================================================================================================================
# R-01 end to end, push_recursive.sh: a held commit whose message has a shape that desyncs the parsers is still HELD (or refused), never pushed
# ======================================================================================================================================
M="$T/m"; RUND="$A/$ID"
GOJ() { printf '{"schema":"review-verdict/1","verdict":"%s","covers_runs":%s,"model":"opus","effort":"xhigh","blocking_findings":%s}\n' "$1" "$(printf '%s' "$2" | jq -c 'map({repository:".",commit:"0000000000000000000000000000000000000000",cpa_run:.})')" "$3"; }
fresh() {
  rm -rf "$T/m" "$T/b" "$T/c"; mkdir -p "$T/b" "$RUND"; : > "$RUND/commits.tsv"; : > "$A/$ID2/commits.tsv"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$M"; mkdir -p "$M/specs/ev/reviews" "$M/$(dirname "$SCHEMA")"; cp "$D0/$SCHEMA" "$M/$SCHEMA"
  GOJ NO-GO "[\"$ID\"]" 1 > "$M/specs/ev/reviews/V_nogo.json"
  echo base > "$M/base.txt"; commit_all "$M" init
  for r in r1 r2; do git -C "$M" remote add $r "$T/b/$r.git"; git -C "$M" push -q $r main; done
}
rowof() { printf '%s\t%s\t%s\n' "${3:-.}" "$(git -C "$1" rev-parse HEAD)" "$2" >> "$A/$2/commits.tsv"; }
plain() { echo "$1 $RANDOM" > "$M/$1"; git -C "$M" add -- "$1"; git -C "$M" commit -q -m "change $1" -m "CPA-Run: $ID"; rowof "$M" "$ID"; }
run() { "$H" --main-root "$M" --branch main --run-dir "$RUND" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
tip() { git -C "$T/b/$1.git" rev-parse main 2>/dev/null || echo none; }
head_() { git -C "${1:-$M}" rev-parse HEAD; }
pushshape() { # pushshape <label> <printf format> <want rc> <want reason fragment> <prefix|init>: an unheld CPA commit, then a commit with this raw message. A hold pushes the unheld prefix; a refusal pushes nothing
  fresh; plain a.txt; local before init want; before="$(head_)"; init="$(git -C "$M" rev-list --max-parents=0 HEAD)"; [ "$5" = prefix ] && want="$before" || want="$init"
  raw "$M" "$2" >/dev/null; rowof "$M" "$ID"; run
  eq "R-01 push [$1]: exit" "$RC" "$3"; eq "R-01 push [$1]: r1 holds only what may be public" "$(tip r1)" "$want"; eq "R-01 push [$1]: r2 holds only what may be public" "$(tip r2)" "$want"
  has "R-01 push [$1]: reported ($4)" "$(cat "$T/out")" "$4"
}
V=specs/ev/reviews/V_nogo.json
fresh; plain a.txt; before="$(head_)"; raw "$M" "held b\n\nCPA-Run: $ID\nAwaits-Review: $V\n" >/dev/null; rowof "$M" "$ID"; run
eq "control: a held commit, plain message: 14" "$RC" 14; eq "control: the remote holds the unheld prefix" "$(tip r1)" "$before"; has "control: HELD" "$(cat "$T/out")" HELD
pushshape "divider line in the body (the review's needle)" "Add b\n---\nnotes\n\nCPA-Run: $ID\nAwaits-Review: $V\n" 14 HELD prefix
pushshape "divider line with a diff header" "Add b\n--- a/x\n+++ b/x\n\nCPA-Run: $ID\nAwaits-Review: $V\n" 14 HELD prefix
pushshape "whitespace-only blank lines" "Add b\n \t\nmore\n  \n\nCPA-Run: $ID\nAwaits-Review: $V\n" 14 HELD prefix
pushshape "trailing blank lines" "Add b\n\nCPA-Run: $ID\nAwaits-Review: $V\n\n\n" 14 HELD prefix
pushshape "CRLF message" "Add b\r\n\r\nCPA-Run: $ID\r\nAwaits-Review: $V\r\n" 20 unrecorded_local_commit init
pushshape "folded Awaits-Review" "Add b\n\nCPA-Run: $ID\nAwaits-Review: $V\n  x\n" 20 unrecorded_local_commit init
pushshape "duplicate Awaits-Review" "Add b\n\nCPA-Run: $ID\nAwaits-Review: $V\nAwaits-Review: specs/ev/reviews/other.json\n" 20 unrecorded_local_commit init
pushshape "duplicate CPA-Run" "Add b\n\nCPA-Run: $ID\nCPA-Run: $ID2\nAwaits-Review: $V\n" 20 unrecorded_local_commit init
pushshape "hold in the last paragraph, CPA-Run in an earlier one" "CPA-Run: $ID\n\nAwaits-Review: $V\n" 20 unrecorded_local_commit init
pushshape "Awaits-Review before the block, CPA-Run alone in the last paragraph" "Add b\nAwaits-Review: $V\n\nCPA-Run: $ID\n" 20 unrecorded_local_commit init
pushshape "divider line inside the block" "Add b\n\nCPA-Run: $ID\n---\nAwaits-Review: $V\n" 20 unrecorded_local_commit init
# a commit that passed the CPA predicate cannot reach the hold reader with a block it cannot read: the hold reader refuses (malformed_trailers) when the two disagree. Simulated with a
# git shim that corrupts ONLY the second message read of the held commit (the predicate's read is the first one, the Awaits-Review read the second, the CPA-Run read the third).
fresh; plain a.txt; before="$(head_)"; raw "$M" "held b\n\nCPA-Run: $ID\nAwaits-Review: $V\n" >/dev/null; rowof "$M" "$ID"
mkdir -p "$T/shim"; REALGIT="$(command -v git)"; HSHA="$(head_)"; rm -f "$T/reads"
cat > "$T/shim/git" <<SHIM
#!/bin/sh
for a; do last="\$a"; done
case "\$*" in
  *"log -1 --format=%B"*)
    if [ "\$last" = "$HSHA" ]; then
      printf x >> "$T/reads"
      if [ "\$(wc -c < "$T/reads")" -eq 2 ]; then printf 'held b\n\nCPA-Run: $ID\nAwaits-Review: a\nAwaits-Review: b\n'; exit 0; fi
    fi ;;
esac
exec "$REALGIT" "\$@"
SHIM
chmod +x "$T/shim/git"; PATH="$T/shim:$PATH" run
eq "R-01 a block that the hold reader cannot read although the predicate read it: 20" "$RC" 20; has "R-01 named malformed_trailers" "$(cat "$T/out")" malformed_trailers
eq "R-01 nothing was pushed on malformed_trailers" "$(tip r1)" "$(git -C "$M" rev-parse HEAD~2)"

# ======================================================================================================================================
# R-02 a held merge commit AFTER unheld commits withholds EVERYTHING of the range
# ======================================================================================================================================
mergeafter() { # mergeafter <n unheld commits before the merge> <verdict path of the merge>
  fresh; local i; for ((i=1; i<=$1; i++)); do plain "u$i.txt"; done
  git -C "$M" checkout -q -b side HEAD~"$1"; echo s > "$M/s.txt"; git -C "$M" add s.txt; git -C "$M" commit -q -m side -m "CPA-Run: $ID"; rowof "$M" "$ID"; git -C "$M" checkout -q main
  git -C "$M" merge -q --no-ff -m "Merge side (CPA)" -m "CPA-Run: $ID
Awaits-Review: $2" side; rowof "$M" "$ID"
}
mergeafter 1 "$V"; base="$(git -C "$M" rev-parse HEAD~2)"; run
eq "R-02 held merge after ONE unheld commit: 14" "$RC" 14; eq "R-02 nothing of the range reached r1" "$(tip r1)" "$base"; eq "R-02 nothing of the range reached r2" "$(tip r2)" "$base"
mergeafter 3 "$V"; base="$(git -C "$M" rev-parse HEAD~4)"; run
eq "R-02 held merge after THREE unheld commits: 14" "$RC" 14; eq "R-02 nothing of the range reached r1 (3)" "$(tip r1)" "$base"; eq "R-02 nothing of the range reached r2 (3)" "$(tip r2)" "$base"
has "R-02 the merge is reported HELD" "$(cat "$T/out")" HELD
fresh; plain u1.txt; plain u2.txt; git -C "$M" checkout -q -b side HEAD~2; echo s > "$M/s.txt"; git -C "$M" add s.txt; git -C "$M" commit -q -m side -m "CPA-Run: $ID"; rowof "$M" "$ID"; git -C "$M" checkout -q main
git -C "$M" merge -q --no-ff -m "Merge side (CPA)" -m "CPA-Run: $ID" side; rowof "$M" "$ID"; plain after.txt; run
eq "R-02 golden-false: an UNHELD merge in the range is pushed with everything: 0" "$RC" 0; eq "R-02 golden-false: r1 holds the tip" "$(tip r1)" "$(head_)"
# the held commit that is NOT a merge but sits after an unheld commit pushes the prefix below it (the mergeblock rule must not over-withhold)
fresh; plain u1.txt; before="$(head_)"; raw "$M" "held b\n\nCPA-Run: $ID\nAwaits-Review: $V\n" >/dev/null; rowof "$M" "$ID"; run
eq "R-02 golden-false: a held NON-merge commit after an unheld one: the prefix is pushed" "$(tip r1)" "$before"

# ======================================================================================================================================
# R-01 integrate_merge.sh: the held scan and the Foreign-Commit dedupe use the same parser
# ======================================================================================================================================
IMR="$A/$ID"; EVR=ev
imfresh() {
  rm -rf "$T/m" "$T/b" "$T/c1" "$A/$ID"; mkdir -p "$T/b" "$A/$ID"; : > "$A/$ID/commits.tsv"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$M"; printf '/.audit/\n' > "$M/.gitignore"; mkdir -p "$M/scripts/repo" "$M/$EVR"
  echo base > "$M/base.txt"; echo f0 > "$M/f.txt"; echo t0 > "$M/scripts/repo/check_classes.tsv"; commit_all "$M" init
  for r in r1 r2; do git -C "$M" remote add $r "$T/b/$r.git"; git -C "$M" push -q $r main; done
  git clone -q "$T/b/r1.git" "$T/c1"; git -C "$T/c1" config user.email t@t; git -C "$T/c1" config user.name t
}
imrc() { printf '%b' "$2" > "$T/c1/$1"; git -C "$T/c1" add -A; git -C "$T/c1" commit -q -m "${3:-remote change $1}"; git -C "$T/c1" push -q origin main; }
imrun() { "$IM" --root "$M" --branch main --run-dir "$IMR" --ev "$EVR" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
body() { git -C "$M" log -1 --format=%B "${1:-HEAD}"; }
# held scan: a local commit whose Awaits-Review sits behind a divider line is still held
imfresh; raw "$M" "local\n---\nnotes\n\nCPA-Run: $ID\nAwaits-Review: ev/reviews/V.json\n" >/dev/null; rowof "$M" "$ID"; imrc r.txt 'r\n'; imrun
eq "R-01 merge with a local held commit behind a divider: 0" "$RC" 0; has "R-01 the merge awaits its review" "$(body)" "Awaits-Review: $EVR/reviews/CPA-merge-$ID.json"; has "R-01 reported local_held_commit" "$(cat "$T/out")" local_held_commit
# a local commit whose trailer block cannot be read is a refusal, with the merge aborted
imfresh; raw "$M" "local\n\nCPA-Run: $ID\nAwaits-Review: a\nAwaits-Review: b\n" >/dev/null; rowof "$M" "$ID"; imrc r.txt 'r\n'; LT="$(head_ "$M")"; imrun
eq "R-01 merge with a local commit whose block is malformed: 20" "$RC" 20; has "R-01 named malformed_trailers" "$(cat "$T/err")" malformed_trailers
eq "R-01 HEAD unchanged after the refusal" "$(head_ "$M")" "$LT"; eq "R-01 no MERGE_HEAD left" "$(test -e "$M/.git/MERGE_HEAD"; echo $?)" 1
# Foreign-Commit dedupe: a Foreign-Commit line behind a divider in an earlier CPA message still counts as listed
imfresh; ANC="$(head_ "$M")"; imrc a.txt 'a\n' "foreign commit"; FA="$(git -C "$T/c1" rev-parse HEAD)"
raw "$M" "earlier\n---\nnotes\n\nCPA-Run: $ID\nForeign-Commit: $FA\n" >/dev/null; rowof "$M" "$ID"
imrun --anchor "$ANC"
eq "R-01 merge with an anchor: 0" "$RC" 0; hasnot "R-01 a Foreign-Commit line behind a divider is found: not named again" "$(body)" "Foreign-Commit: $FA"
# golden-false: a foreign commit that no message names is named
imfresh; ANC="$(head_ "$M")"; echo l > "$M/l.txt"; git -C "$M" add l.txt; git -C "$M" commit -q -m "local" -m "CPA-Run: $ID"; rowof "$M" "$ID"; imrc a.txt 'a\n' "foreign commit"; FA="$(git -C "$T/c1" rev-parse HEAD)"
imrun --anchor "$ANC"; has "R-01 golden-false: an unnamed foreign commit is named" "$(body)" "Foreign-Commit: $FA"

# ======================================================================================================================================
# R-06 integrate_merge --resolution: the 9.2 backup exists BEFORE the trial merge
# ======================================================================================================================================
imfresh; echo local > "$M/f.txt"; git -C "$M" add -A; git -C "$M" commit -q -m "local f"; imrc f.txt 'remote\n'; imrun; RTIP="$(git -C "$T/b/r1.git" rev-parse main)"
RDIR="$M/.audit/merge-resolution/20261005T000000Z-0-oldr"; mkdir -p "$RDIR"; printf 'merged\n' > "$RDIR/f.txt"
jq -n --arg tip "$RTIP" --arg h "$(sha256sum "$RDIR/f.txt" | cut -d' ' -f1)" '{tip:$tip,paths:["f.txt"],model:"opus",effort:"xhigh",files:{"f.txt":$h}}' > "$RDIR/resolution.json"
mkdir -p "$T/shim2"; REALGIT="$(command -v git)"; : > "$T/trial.log"
cat > "$T/shim2/git" <<SHIM
#!/bin/sh
case "\$*" in *"merge --no-ff --no-commit"*) if [ -d "$IMR/backup" ]; then echo backup_first >> "$T/trial.log"; else echo no_backup_yet >> "$T/trial.log"; fi ;; esac
exec "$REALGIT" "\$@"
SHIM
chmod +x "$T/shim2/git"; rm -rf "$IMR/backup"
PATH="$T/shim2:$PATH" imrun --resolution "$RDIR"
eq "R-06 the resolution run reaches the trial merge and stops at the missing secret fold: 20" "$RC" 20; has "R-06 stopped at secret_fold_unavailable" "$(cat "$T/err")" secret_fold_unavailable
eq "R-06 the trial merge ran" "$(grep -c . "$T/trial.log")" 1; eq "R-06 the 9.2 backup existed before the trial merge" "$(head -1 "$T/trial.log")" backup_first
eq "R-06 the backup holds the local range" "$(test -f "$IMR/backup/main.bundle" -o -f "$IMR/backup/main.bundle.empty"; echo $?)" 0
eq "R-06 golden-false: HEAD unchanged, no MERGE_HEAD" "$(test -e "$M/.git/MERGE_HEAD"; echo $?)" 1

# ======================================================================================================================================
# R-06 repo_config_gate: the status of `git config --local --list` is read
# ======================================================================================================================================
G1="$T/g1"; mkrepo "$G1"; echo x > "$G1/x"; commit_all "$G1" init; printf 'core.*\tcore\nuser.*\tuser\nremote.*\tr\nbranch.*\tb\n' > "$T/allow.tsv"
repo_config_gate "$G1" "$T/allow.tsv" >/dev/null 2>&1; eq "R-06 control: a clean config passes the gate" "$?" 0
mkdir -p "$T/gshim"; printf '#!/bin/sh\ncase "$*" in *"config --local --list"*) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$(command -v git)" > "$T/gshim/git"; chmod +x "$T/gshim/git"
out="$(PATH="$T/gshim:$PATH" repo_config_gate "$G1" "$T/allow.tsv" 2>&1)"; rc=$?
eq "R-06 a failing config listing is a refusal, not 'no keys': 1" "$rc" 1; has "R-06 the refusal names the listing" "$out" "listing"
git -C "$G1" config evil.key 1; repo_config_gate "$G1" "$T/allow.tsv" >/dev/null 2>&1; eq "R-06 golden-false: an unapproved key still refuses" "$?" 1; git -C "$G1" config --unset evil.key

# ======================================================================================================================================
# R-01 commit_recursive.sh refuses a message the readers could not agree on, and a written message round-trips through the parser
# ======================================================================================================================================
cfresh() { rm -rf "$T/w" "$T/run"; mkrepo "$T/w"; printf '/.audit/\n' > "$T/w/.gitignore"; echo base > "$T/w/base.txt"; commit_all "$T/w" init; mkdir -p "$T/run"; }
crrun() { "$CR" --repo "$T/w" --run-dir "$T/run" --run-id "$ID" --message "$1" "${@:2}" >"$T/out" 2>"$T/err"; RC=$?; }
cfresh; echo a > "$T/w/a.txt"; crrun $'Add a\n---\nnotes' -- a.txt
eq "R-01 commit_recursive refuses a message with a divider line: 20" "$RC" 20; has "R-01 reason unsafe_message" "$(cat "$T/err")" unsafe_message; eq "R-01 no commit was made" "$(git -C "$T/w" rev-list --count HEAD)" 1
cfresh; echo a > "$T/w/a.txt"; crrun $'Add a\r\nmore' -- a.txt; eq "R-01 commit_recursive refuses a CR in the message: 20" "$RC" 20
cfresh; echo a > "$T/w/a.txt"; crrun $'Add a\nawaits-review: x' -- a.txt; eq "R-01 commit_recursive refuses a reserved key in any case: 20" "$RC" 20
cfresh; echo a > "$T/w/a.txt"; crrun 'Add a' --attribution '---' -- a.txt; eq "R-01 commit_recursive refuses a divider as the attribution: 20" "$RC" 20
cfresh; echo a > "$T/w/a.txt"; crrun 'Add a' --attribution 'Awaits-Review: x.json' -- a.txt; eq "R-01 commit_recursive refuses a reserved trailer as the attribution: 20" "$RC" 20
cfresh; echo a > "$T/w/a.txt"; printf 'a.txt\tspecs/ev/reviews/V.json\n' > "$T/held.tsv"
crrun $'Add a\n \t\nsecond paragraph\n\n\nthird' --held-from "$T/held.tsv" --attribution 'Co-Authored-By: T <t@t.invalid>' -- a.txt
eq "R-01 commit_recursive accepts blank-line variants: 0" "$RC" 0
eq "R-01 the written commit is read by the shared parser: run id" "$(cpa_runid "$T/w" HEAD)" "$ID"
eq "R-01 the written commit is read by the shared parser: hold" "$(cpa_trailer "$T/w" HEAD Awaits-Review)" "specs/ev/reviews/V.json"

# ======================================================================================================================================
# R-07 the verdict vocabulary: findings with the 11.4.235(D) finding_layer set validate against the schema; the evidence-class sense stays in evidence_layer
# ======================================================================================================================================
vschema() { python3 -I - "$SCHEMA_FILE" "$1" <<'PY'
import json, sys
from jsonschema import Draft202012Validator
s = json.load(open(sys.argv[1])); d = json.load(open(sys.argv[2]))
errs = sorted(Draft202012Validator(s).iter_errors(d), key=lambda e: list(e.path))
for e in errs[:5]: print("ERR", list(e.path), e.message[:120])
sys.exit(1 if errs else 0)
PY
}
mkv() { # mkv <finding_layer> [extra json for the finding]
  local ex="${2:-}"; [ -n "$ex" ] || ex='{}'
  jq -n --arg fl "$1" --argjson extra "$ex" '{schema:"review-verdict/1",gate:"G-GATE",verdict:"NO-GO",model:"opus",effort:"?",covers_runs:[],blocking_findings:1,
    findings:[({id:"R-01",blocking:true,finding_layer:$fl,summary:"x"} + $extra)]}' > "$T/v.json"; }
for fl in source-defect test-instrumentation process-doc; do mkv "$fl" '{"evidence_layer":"runtime"}'; vschema "$T/v.json" >"$T/vs.out" 2>&1; eq "R-07 finding_layer $fl + evidence_layer validates" "$?" 0; done
for fl in source artifact runtime; do mkv "$fl"; vschema "$T/v.json" >"$T/vs.out" 2>&1; eq "R-07 golden: the contract's own finding_layer $fl still validates" "$?" 0; done
mkv "bogus-layer"; vschema "$T/v.json" >"$T/vs.out" 2>&1; eq "R-07 golden-false: an unknown finding_layer is refused" "$?" 1
mkv "source-defect" '{"evidence_layer":"bogus"}'; vschema "$T/v.json" >"$T/vs.out" 2>&1; eq "R-07 golden-false: an unknown evidence_layer is refused" "$?" 1
REV=specs/001-full-project-audit-remediation/evidence/reviews/WP-04-T046.json
if [ -f "$D0/$REV" ]; then vschema "$D0/$REV" >"$T/vs.out" 2>&1; eq "R-07 the real verdict $REV validates against the schema" "$?" 0; else echo "NOTE: $REV absent in this checkout, the real-verdict case is skipped"; fi

# ======================================================================================================================================
fin
