#!/usr/bin/env bash
# WP-04 fix round 4 test (TDD): the findings of the xhigh review WF3-REVIEW-wp04-helpers: B-1 (symlink type change defeats the S2 append_only and
# blob_name rules; S3 must not skip a declared symlink), I-1 (an unreachable remote is never read as a remote that holds nothing), I-2 (the helper's
# class tables come from a trusted source, never from the helper's own working tree when run in place), I-3 (the reviewer's mutants X1, X2, X3, X6,
# X7, X8, X9 get killing cases) and the real-defect minors m1-m8. Throwaway repositories and LOCAL BARE remotes only; no real remote, no credential.
# Usage: bash scripts/repo/tests/test_wp04d.sh        Env: R4=<dir that holds the helpers> (the mutation driver points it at a copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; R4="${R4:-$D0/scripts/repo}"; case "$R4" in /*) ;; *) R4="$D0/$R4" ;; esac
H="$R4/scope_check.sh"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04d.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable"
export GIT_SSH_COMMAND=false
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
RC=0
# ======================================================================================================================
# B-1  S2: a declared symlink (or a type change) never passes the append_only / blob_name rules
# ======================================================================================================================
SC="$R4/scope_check.sh"; EV=ev
sc4fresh() { rm -rf "$T/s" "$T/outside"; mkrepo "$T/s"; mkdir -p "$T/s/$EV/blobs" "$T/outside"; printf '{"n":1}\n{"n":2}\n' > "$T/s/$EV/ledger.jsonl"
  BX="$(printf 'good\n' | sha256sum | cut -d' ' -f1)"; printf 'good\n' > "$T/s/$EV/blobs/$BX"; commit_all "$T/s" init; }
sc4run() { ( cd "$T/s" && "$SC" --root "$T/s" --ev "$EV" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; RC=$?; }
sc4fresh; printf '%s\n' "$EV/ledger.jsonl" > "$T/cs.lst"
printf '{"n":9}\n{"n":2}\n' > "$T/s/$EV/ledger.jsonl"; sc4run; eq "control: line 1 rewritten in place: 13" "$RC" 13
sc4fresh; printf '{"n":3}\n' >> "$T/s/$EV/ledger.jsonl"; sc4run; eq "golden-true: a pure append of the regular ledger: 0" "$RC" 0
printf '%s\n' "$EV/blobs/$BX" > "$T/cs.lst"; sc4run; eq "golden-true: an unchanged correctly named blob: 0" "$RC" 0
sc4fresh; printf '%s\n' "$EV/ledger.jsonl" > "$T/cs.lst"; printf '{"n":1}\n{"n":2}\n{"n":3}\n' > "$T/outside/ext.jsonl"; rm "$T/s/$EV/ledger.jsonl"; ln -s "$T/outside/ext.jsonl" "$T/s/$EV/ledger.jsonl"
sc4run; eq "the ledger replaced by a symlink to a file that extends HEAD byte for byte: 13 (B-1)" "$RC" 13; has "named append_only" "$(cat "$T/out")" append_only
# HEAD already holds the link (round two of the same attack): a regular file that starts with the link text is a type change, not an append
git -C "$T/s" add -A; git -C "$T/s" commit -qm "link committed"; rm "$T/s/$EV/ledger.jsonl"; printf '%s\nforged\n' "$T/outside/ext.jsonl" > "$T/s/$EV/ledger.jsonl"
sc4run; eq "HEAD holds the link, the work tree a regular file starting with the link text (type change): 13 (B-1)" "$RC" 13; has "named append_only (type change)" "$(cat "$T/out")" append_only
sc4fresh; printf 'evil\n' > "$T/outside/X"; NM="$(sha256sum "$T/outside/X" | cut -d' ' -f1)"; ln -s "$T/outside/X" "$T/s/$EV/blobs/$NM"; printf '%s\n' "$EV/blobs/$NM" > "$T/cs.lst"
sc4run; eq "a content-addressed blob added as a symlink whose target hashes to its name: 13 (B-1)" "$RC" 13; has "named blob_name" "$(cat "$T/out")" blob_name
sc4fresh; printf '{"n":1}\n{"n":2}\n{"n":3}\n' > "$T/s/$EV/ledger.jsonl"; mv "$T/s/$EV" "$T/s/ev_real"; ln -s ev_real "$T/s/$EV"; printf '%s\n' "$EV/ledger.jsonl" > "$T/cs.lst"
sc4run; eq "the evidence directory itself replaced by a symlink (a symlink path component): 13 (B-1)" "$RC" 13

# ======================================================================================================================
# B-1  S3 / revision header: a declared symlink is classified by its link type, never skipped silently
# ======================================================================================================================
VCH="$R4/validate_cheap.sh"; EVR=specs/001-full-project-audit-remediation/evidence
vc4fresh() { rm -rf "$T/vr" "$T/code"; mkrepo "$T/vr"; mkdir -p "$T/vr/src" "$T/vr/$EVR" "$T/vr/docs"; echo ok > "$T/vr/src/keep.txt"; echo '{"n":1}' > "$T/vr/$EVR/ledger.jsonl"; echo r > "$T/vr/$EVR/y.txt"; commit_all "$T/vr" init
  mkdir -p "$T/code/scripts/repo" "$T/code/scripts/hooks"; cp "$R4"/*.sh "$R4"/*.tsv "$R4"/fixture_roots.txt "$T/code/scripts/repo/"
  printf '#!/usr/bin/env bash\n[ -e ./LANDMINE ] && { echo landmine; exit 1; }\nexit 0\n' > "$T/code/scripts/detect-landmines.sh"; chmod 644 "$T/code/scripts/detect-landmines.sh"
  cp "$D0/scripts/hooks/no-false-positive-log.sh" "$T/code/scripts/hooks/"; }
vc4run() { ( cd "$T/vr" && "$VCH" --root "$T/vr" --code-root "$T/code" --registry "${REGF:-$T/code/scripts/repo/validate_checks.tsv}" --adopt-working-tables ${VHELD:+--held-from "$VHELD"} --files-from "$T/cs.lst" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
vc4fresh; printf '%s/y.txt\n' "$EVR" > "$T/cs.lst"; echo r2 >> "$T/vr/$EVR/y.txt"; vc4run; eq "golden-true: a regular evidence file: 0" "$RC" 0
vc4fresh; rm "$T/vr/$EVR/y.txt"; ln -s /etc/hostname "$T/vr/$EVR/y.txt"; printf '%s/y.txt\n' "$EVR" > "$T/cs.lst"; vc4run
eq "a declared symlink in an evidence class: 10 (B-1)" "$RC" 10; has "reported as a symlink failure" "$(cat "$T/out")" "fail	symlink	$EVR/y.txt"
vc4fresh; rm "$T/vr/$EVR/ledger.jsonl"; ln -s /etc/hostname "$T/vr/$EVR/ledger.jsonl"; printf '%s/ledger.jsonl\n' "$EVR" > "$T/cs.lst"; vc4run
eq "a declared symlink in the evidence-ledger class: 10 (B-1)" "$RC" 10; has "reported as a symlink failure" "$(cat "$T/out")" "fail	symlink	$EVR/ledger.jsonl"
vc4fresh; ln -s /etc/hostname "$T/vr/src/l.txt"; printf 'src/l.txt\n' > "$T/cs.lst"; vc4run
eq "a declared symlink in the source class: 0 (not an evidence store)" "$RC" 0; has "but never skipped silently: it is reported" "$(cat "$T/out")" "symlink_not_judged	src/l.txt"
vc4fresh; rm "$T/vr/$EVR/y.txt"; ln -s /etc/hostname "$T/vr/$EVR/y.txt"; commit_all "$T/vr" "link"; rm "$T/vr/$EVR/y.txt"; printf '/etc/hostname\nforged\n' > "$T/vr/$EVR/y.txt"; printf '%s/y.txt\n' "$EVR" > "$T/cs.lst"; vc4run
eq "HEAD holds a link in an evidence path, the work tree a regular file (type change): 10 (B-1)" "$RC" 10; has "reported as a symlink failure" "$(cat "$T/out")" "fail	symlink	$EVR/y.txt"
CRH="$R4/check_revision_headers.sh"
vc4fresh; ln -s /etc/hostname "$T/vr/docs/a.md"; ( cd "$T/vr" && "$CRH" --root "$T/vr" docs/a.md ) >"$T/out" 2>"$T/err"; RC=$?
eq "check_revision_headers: a declared .md symlink is not failed for a missing header: 0" "$RC" 0; has "but it is reported, not skipped silently" "$(cat "$T/out")" "symlink_not_judged	revision_header	docs/a.md"
printf 'no header\n' > "$T/vr/docs/b.md"; ( cd "$T/vr" && "$CRH" --root "$T/vr" docs/b.md ) >"$T/out" 2>"$T/err"; eq "golden-false: a regular headerless .md still fails: 10" "$?" 10

# ======================================================================================================================
# I-1  S1 and S6: an unreachable remote is never read as a remote that holds nothing
# ======================================================================================================================
FF="$R4/integrate_ff_only.sh"; n=0
fx() {
  n=$((n+1)); P="$T/f$n"; mkdir -p "$P"
  git init -q --bare -b main "$P/sm.git"; git init -q --bare -b main "$P/sm2.git"
  git clone -q "$P/sm.git" "$P/smw" 2>/dev/null; ( cd "$P/smw" && git checkout -q -b main 2>/dev/null; echo s1 > s; git add s; $G commit -qm s1; git push -q origin main; git remote add mirror "$P/sm2.git"; git push -q mirror main )
  git init -q --bare -b main "$P/o.git"; git init -q --bare -b main "$P/m.git"
  git clone -q "$P/o.git" "$P/w" 2>/dev/null
  ( cd "$P/w" && git checkout -q -b main 2>/dev/null; printf '/.audit/\n' > .gitignore; echo a > f; echo g > g; mkdir -p d e; echo d > d/f0; echo e > e/f0
    $G submodule add -q "$P/sm.git" sm 2>/dev/null; git -C sm remote add mirror "$P/sm2.git"; git -C sm fetch -q mirror
    git add -A; $G commit -qm c1; git push -q origin main; git remote add mirror "$P/m.git"; git push -q mirror main )
  W="$P/w"; CS="$P/changeset.txt"; : > "$CS"; OUT="$P/out.json"
}
other() { local f="$1" c="$2"; shift 2; local rs="${*:-origin mirror}"; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null
  ( cd "$P/oc" && git remote add mirror "$P/m.git" && mkdir -p "$(dirname "$f")" && echo "$c" > "$f" && git add "$f" && $G commit -qm "other $f"
    for r in $rs; do git push -q "$r" HEAD:main; done ); }
ffrun() { timeout 60 "$FF" --root "$W" --main-only --report-behind-submodules --changeset-from "$CS" --json "$OUT" "$@" >"$P/stdout" 2>"$P/stderr"; RC=$?; }
j() { jq -r "$1" "$OUT" 2>/dev/null || echo "?"; }
head_() { git -C "$W" rev-parse HEAD; }
fx; git -C "$W" remote set-url origin "$P/gone1.git"; git -C "$W" remote set-url mirror "$P/gone2.git"; h0="$(head_)"; ffrun
eq "S1 with every remote unreachable: 11, never a refusal naming published history (I-1)" "$RC" 11; eq "reason remote_unreachable" "$(j .reason)" remote_unreachable
eq "every remote is recorded fetch_failed" "$(j '[.remotes[]|.fetch]|sort|unique|join(",")')" "fetch_failed:mirror,fetch_failed:origin"; eq "nothing moved" "$(head_)" "$h0"
fx; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && echo z > z && git add z && $G commit -qm foreign && git push -q origin HEAD:main ); git -C "$W" fetch -q origin main; git -C "$W" merge -q --ff-only origin/main
git -C "$W" remote set-url origin "$P/gone1.git"; ffrun
eq "S1 partial outage: the unreachable remote's last known tip covers the foreign commit it holds; the reachable remote only lags: 0 (I-1)" "$RC" 0; hasnot "no unrecorded_local_commit reason" "$(j .reason)" unrecorded_local_commit
fx; git -C "$W" remote add ghost "$P/gone3.git"; ( cd "$W" && echo l > l && git add l && $G commit -qm "hand commit" ); ffrun
eq "S1: an unreachable remote that was never fetched cannot clear a local-only commit: 11 (cannot compute)" "$RC" 11; eq "reason remote_unreachable" "$(j .reason)" remote_unreachable
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "hand commit" ); git -C "$W" remote set-url mirror "$P/gone2.git"; ffrun
eq "golden-false: a hand commit and one reachable remote still refuses: 20" "$RC" 20; eq "reason unrecorded_local_commit" "$(j .reason)" unrecorded_local_commit
eq "the hand commit is named, no published commit" "$(j '.commits|length')" 1
fx; other f2 two origin; git -C "$W" remote set-url mirror "$P/gone2.git"; ffrun
eq "golden-false: a partial outage with nothing local-only still fast-forwards from the reachable remote: 0" "$RC" 0; eq "status fast_forwarded" "$(j .status)" fast_forwarded
# S6
PRH="$R4/push_recursive.sh"; M="$T/pm"; PA="$T/paudit"; PRUND="$PA/20261005T000000Z-1-aaaa"
prfresh() { rm -rf "$T/pm" "$T/pb" "$PA" "$T/pc" "$T/pc2" "$T/elsewhere"; mkdir -p "$T/pb" "$PRUND"
  for r in r1 r2; do git init -q --bare -b main "$T/pb/$r.git"; done
  mkrepo "$M"; echo base > "$M/base.txt"; commit_all "$M" init
  for r in r1 r2; do git -C "$M" remote add $r "$T/pb/$r.git"; git -C "$M" push -q $r main; done; }
pcpa() { local d="$1" k="$2" f="$3" id="${4:-20261005T000000Z-1-aaaa}"; echo "$f $RANDOM" > "$d/$f"; git -C "$d" add -- "$f"
  git -C "$d" commit -q -m "change $f" -m "CPA-Run: $id"; printf '%s\t%s\t%s\n' "$k" "$(git -C "$d" rev-parse HEAD)" "$id" >> "$PA/$id/commits.tsv"; }
prrun() { timeout 30 "$PRH" --main-root "$M" --branch main --run-dir "$PRUND" "$@" >"$T/out" 2>"$T/err"; PRC=$?; }
ptip() { git -C "$T/pb/$1.git" rev-parse main 2>/dev/null || echo none; }
phead() { git -C "${1:-$M}" rev-parse HEAD; }
prfresh; git clone -q "$T/pb/r2.git" "$T/pc2"; ( cd "$T/pc2" && git config user.email t@t && git config user.name t && echo foreign > fr.txt && git add fr.txt && git commit -qm foreign && git push -q origin main )
git -C "$M" fetch -q r2 main; git -C "$M" merge -q --ff-only r2/main; pcpa "$M" . a.txt; git -C "$M" remote set-url r2 "$T/pb/gone2.git"; lt="$(phead)"; prrun
eq "S6 partial outage: the unreachable remote's last known tip covers its foreign commit: 11, not REFUSED (I-1)" "$PRC" 11; hasnot "no refusal naming the foreign commit" "$(cat "$T/out")" REFUSED
has "r2 reported unreachable" "$(cat "$T/out")" "NOPUSH	.	r2	remote_unreachable"; has "r1 pushed" "$(cat "$T/out")" "PUSHED	.	r1	$lt"
prfresh; git -C "$M" remote add ghost "$T/pb/gone3.git"; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm hand; r1b="$(ptip r1)"; prrun
eq "S6: an unreachable never-fetched remote cannot clear a local-only commit: 11 (cannot compute)" "$PRC" 11; has "ghost named unreachable" "$(cat "$T/out")" "NOPUSH	.	ghost	remote_unreachable"; eq "nothing was pushed" "$(ptip r1)" "$r1b"
prfresh; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm hand; git -C "$M" remote set-url r2 "$T/pb/gone2.git"; prrun
eq "golden-false: a hand commit with a known-tip remote unreachable still refuses: 20" "$PRC" 20; has "unrecorded_local_commit" "$(cat "$T/out")" unrecorded_local_commit

# ======================================================================================================================
# I-2  validate_cheap: the helper's class tables come from a trusted source, never from its own working tree when run in place
# ======================================================================================================================
codeg() { rm -rf "$T/codeg"; mkrepo "$T/codeg"; mkdir -p "$T/codeg/scripts/repo"; cp "$R4"/*.sh "$R4"/*.tsv "$R4"/fixture_roots.txt "$T/codeg/scripts/repo/"; git -C "$T/codeg" add -A; git -C "$T/codeg" commit -qm tables; }
vc5run() { ( cd "$T/vr" && "$1" --root "$T/vr" --code-root "$T/code" --registry "$T/code/scripts/repo/validate_checks.tsv" --files-from "$T/cs.lst" "${@:2}" ) >"$T/out" 2>"$T/err"; RC=$?; }
vc4fresh; printf 'a \n' > "$T/vr/x.txt"; printf 'x.txt\n' > "$T/cs.lst"
codeg; vc5run "$T/codeg/scripts/repo/validate_cheap.sh"; eq "in place, tables committed and unmodified: judged with them: 10" "$RC" 10; has "trailing whitespace reported" "$(cat "$T/out")" trailing_whitespace
printf 'x.txt\tgenerated\tprobe row\tT\n' >> "$T/codeg/scripts/repo/check_exemptions.tsv"
vc5run "$T/codeg/scripts/repo/validate_cheap.sh"; eq "in place with an uncommitted edit of the helper's own table (I-2): 20" "$RC" 20; has "class_table_unreviewed named" "$(cat "$T/err")" class_table_unreviewed
mkdir -p "$T/approved"; ( cd "$T/codeg" && for t in check_classes.tsv check_exemptions.tsv fixture_roots.txt; do git show "HEAD:scripts/repo/$t" > "$T/approved/$t"; done )
vc5run "$T/codeg/scripts/repo/validate_cheap.sh" --trusted-tables "$T/approved"; eq "an approved table copy decides, the edited working tree decides nothing: 10" "$RC" 10; has "whitespace failure kept" "$(cat "$T/out")" trailing_whitespace
vc5run "$T/codeg/scripts/repo/validate_cheap.sh" --adopt-working-tables; eq "the explicit owner-approved adoption form uses the working-tree tables: 0" "$RC" 0
rm -rf "$T/codeg2"; mkrepo "$T/codeg2"; mkdir -p "$T/codeg2/scripts/repo"; cp "$R4"/*.sh "$T/codeg2/scripts/repo/"; cp "$R4"/*.tsv "$R4"/fixture_roots.txt "$T/codeg2/scripts/repo/"; git -C "$T/codeg2" add scripts/repo/*.sh; git -C "$T/codeg2" commit -qm code
vc5run "$T/codeg2/scripts/repo/validate_cheap.sh"; eq "in place with the tables untracked: 20" "$RC" 20; has "class_table_unreviewed named" "$(cat "$T/err")" class_table_unreviewed
rm -rf "$T/snap"; mkdir -p "$T/snap"; cp "$R4"/*.sh "$R4"/*.tsv "$R4"/fixture_roots.txt "$T/snap/"; vc5run "$T/snap/validate_cheap.sh"
eq "golden-true: a released snapshot that is no repository is trusted as it is: 10" "$RC" 10; has "whitespace reported" "$(cat "$T/out")" trailing_whitespace
# a HEAD that holds its own tables is still the only table source (no trusted-table lookup needed)
vc4fresh; mkdir -p "$T/vr/scripts/repo"; cp "$R4/check_classes.tsv" "$R4/check_exemptions.tsv" "$R4/fixture_roots.txt" "$T/vr/scripts/repo/"; commit_all "$T/vr" tables; printf 'a \n' > "$T/vr/x.txt"
codeg; printf 'x.txt\tgenerated\tprobe row\tT\n' >> "$T/codeg/scripts/repo/check_exemptions.tsv"; vc5run "$T/codeg/scripts/repo/validate_cheap.sh"
eq "golden-true: the root's HEAD holds the tables, the helper's own working copy is never read: 10" "$RC" 10

# ======================================================================================================================
# I-3  the reviewer's mutants X1 X2 X3 X6 X7 X8 X9: cases that kill them
# ======================================================================================================================
IM="$R4/integrate_merge.sh"; A="$T/audit"; RID=20261005T000000Z-1-aaaa; RUND="$A/$RID"
im4fresh() { rm -rf "$T/m" "$T/b" "$T/c1" "$T/elsewhere" "$A"; mkdir -p "$T/b" "$RUND"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$T/m"; printf '/.audit/\n' > "$T/m/.gitignore"; mkdir -p "$T/m/scripts/repo"; echo base > "$T/m/base.txt"; printf 'gate0\n' > "$T/m/gate.txt"
  for t in check_classes.tsv check_exemptions.tsv fixture_roots.txt; do echo t0 > "$T/m/scripts/repo/$t"; done; commit_all "$T/m" init
  for r in r1 r2; do git -C "$T/m" remote add $r "$T/b/$r.git"; git -C "$T/m" push -q $r main; done
  git clone -q "$T/b/r1.git" "$T/c1"; git -C "$T/c1" config user.email t@t; git -C "$T/c1" config user.name t
  ( cd "$T/m" && echo l > l.txt && git add l.txt && git commit -qm local ); }
im4run() { "$IM" --root "$T/m" --branch main --run-dir "$RUND" --ev ev "$@" >"$T/out" 2>"$T/err"; RC=$?; }
body() { git -C "$T/m" log -1 --format=%B HEAD; }
remote_commit() { # remote_commit <file> <content> [extra message line]
  printf '%b' "$2" > "$T/c1/$1"; git -C "$T/c1" add -A; if [ -n "${3:-}" ]; then git -C "$T/c1" commit -q -m "change $1" -m "$3"; else git -C "$T/c1" commit -q -m "change $1"; fi; git -C "$T/c1" push -q origin main; }
printf 'gate.txt\tG-GATE\n' > "$T/gates.tsv"; printf '{"gate.txt":"%s"}\n' "$(printf 'gate0\n' | sha256sum | cut -d' ' -f1)" > "$T/approved.json"
im4fresh; ( cd "$T/c1" && git rm -q gate.txt && git commit -qm "delete gate" && git push -q origin main ); im4run --path-gates "$T/gates.tsv" --approved "$T/approved.json"
has "X1: a merge that deletes an approved G-GATE path is held" "$(body)" "Awaits-Review"; has "reason gate" "$(cat "$T/out")" gate
im4fresh; ( cd "$T/c1" && git rm -q gate.txt && git commit -qm "delete gate" && git push -q origin main ); printf '{}\n' > "$T/empty.json"; im4run --path-gates "$T/gates.tsv" --approved "$T/empty.json"
hasnot "golden-false: deleting a G-GATE path that the manifest never approved is not held" "$(body)" "Awaits-Review"
for tb in check_exemptions.tsv fixture_roots.txt check_classes.tsv; do
  im4fresh; remote_commit "scripts/repo/$tb" 'changed\n'; im4run
  has "X8: a non-CPA change of the admission table $tb holds the merge" "$(body)" "Awaits-Review"; has "reason admission_table for $tb" "$(cat "$T/out")" admission_table
done
im4fresh; remote_commit scripts/repo/fixture_roots.txt 'changed\n' "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$T/c1" rev-parse HEAD)" "$RID" >> "$RUND/commits.tsv"; im4run
hasnot "golden-false: the same change recorded as a CPA commit is not held" "$(body)" "Awaits-Review"
im4fresh; remote_commit scripts/repo/check_exemptions.tsv 'changed\n' "CPA-Run: ../elsewhere"; mkdir -p "$T/elsewhere"; printf '.\t%s\t../elsewhere\n' "$(git -C "$T/c1" rev-parse HEAD)" > "$T/elsewhere/commits.tsv"; im4run
has "X3 (merge path): a run id that walks out of the audit directory proves nothing: held" "$(body)" "Awaits-Review"
# X2: S6, a hand commit whose CPA-Run points at a forged row outside the audit directory
prfresh; mkdir -p "$T/elsewhere"; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -q -m hand -m "CPA-Run: ../elsewhere"; printf '.\t%s\t../elsewhere\n' "$(phead)" > "$T/elsewhere/commits.tsv"; r1b="$(ptip r1)"; prrun
eq "X2: a hand commit whose run id walks out of the audit directory is refused: 20" "$PRC" 20; has "unrecorded_local_commit" "$(cat "$T/out")" unrecorded_local_commit; eq "nothing pushed" "$(ptip r1)" "$r1b"
# X3: S1 (ff), the same forgery against .audit/commit-push
fx; mkdir -p "$W/.audit/elsewhere"; ( cd "$W" && echo l > l && git add l && $G commit -q -m hand -m "CPA-Run: ../elsewhere" ); printf '.\t%s\t../elsewhere\n' "$(head_)" > "$W/.audit/elsewhere/commits.tsv"; ffrun
eq "X3: S1 refuses a hand commit whose run id walks out of .audit/commit-push: 20" "$RC" 20; eq "reason unrecorded_local_commit" "$(j .reason)" unrecorded_local_commit
# X6: a check script that refuses (20) is a refusal, never a failed check (10)
x6reg() { printf 'check\tcommand\timage\tmode\tbaseline\tscope\tnote\nanti_bluff\tbash scripts/chk.sh {files}\tIMG-TESTUTIL\tplain\t-\tfiles\tprobe\n' > "$T/x6.tsv"; }
vc4fresh; x6reg; printf '#!/usr/bin/env bash\nexit %s\n' 0 > "$T/code/scripts/chk.sh"; printf 'src/keep.txt\n' > "$T/cs.lst"; REGF="$T/x6.tsv" vc4run; eq "X6 golden: a check script that exits 0: 0" "$RC" 0
printf '#!/usr/bin/env bash\necho bad-input-refused >&2\nexit 20\n' > "$T/code/scripts/chk.sh"; REGF="$T/x6.tsv" vc4run
eq "X6: a check script that refuses with 20 is a refusal: 20" "$RC" 20; has "check_refused named" "$(cat "$T/err")" check_refused
printf '#!/usr/bin/env bash\necho finding\nexit 1\n' > "$T/code/scripts/chk.sh"; REGF="$T/x6.tsv" vc4run; eq "X6 golden-false: a check script that fails with 1: 10" "$RC" 10; has "fail anti_bluff" "$(cat "$T/out")" "fail	anti_bluff"
# m8: the fail line names the failing file, not the first five files given
printf '#!/usr/bin/env bash\nwhile IFS= read -r p; do case "$p" in src/b.txt) echo "bad $p"; exit 1 ;; esac; done < "$1"\nexit 0\n' > "$T/code/scripts/chk.sh"
echo a > "$T/vr/src/a.txt"; echo b > "$T/vr/src/b.txt"; printf 'src/a.txt\nsrc/b.txt\n' > "$T/cs.lst"; REGF="$T/x6.tsv" vc4run
eq "m8: the script row fails" "$RC" 10; has "m8: the path column names the failing file only" "$(cat "$T/out")" "fail	anti_bluff	src/b.txt	"
# X7: a file below .github/workflows/<dir>/ is no pipeline definition
NC="$R4/check_no_ci.sh"; rm -rf "$T/ci"; mkrepo "$T/ci"; mkdir -p "$T/ci/.github/workflows/sub"; echo x > "$T/ci/.github/workflows/sub/x.yml"; echo y > "$T/ci/base"; commit_all "$T/ci" init
"$NC" --root "$T/ci" >"$T/out" 2>"$T/err"; eq "X7: .github/workflows/sub/x.yml is no pipeline definition: 0" "$?" 0; eq "no output" "$(cat "$T/out")" ""
echo z > "$T/ci/.github/workflows/top.yml"; "$NC" --root "$T/ci" >"$T/out" 2>"$T/err"; eq "X7 golden-false: .github/workflows/top.yml is one: 10" "$?" 10; has "reported" "$(cat "$T/out")" "ci_pipeline	$T/ci	.github/workflows/top.yml"
# X9: a declared directory (trailing /) blocks an incoming change under it
fx; other d/f new; printf 'd/\n' > "$CS"; h0="$(head_)"; ffrun
eq "X9: an incoming change under the declared directory d/ blocks the fast-forward: 12" "$RC" 12; eq "reason ff_blocked_by_local_changes" "$(j .reason)" ff_blocked_by_local_changes; eq "nothing moved" "$(head_)" "$h0"
fx; other e/f new; printf 'd/\n' > "$CS"; ffrun; eq "X9 golden-false: an incoming change outside the declared directory: 0" "$RC" 0; eq "fast_forwarded" "$(j .status)" fast_forwarded

# ======================================================================================================================
# minors
# ======================================================================================================================
# m1  a declared directory whose first index entry is a gitlink is still a directory
rm -rf "$T/gl" "$T/glsub"; mkrepo "$T/glsub"; echo s > "$T/glsub/f"; commit_all "$T/glsub" s; mkrepo "$T/gl"; mkdir -p "$T/gl/mods"; echo ok > "$T/gl/mods/zz.txt"; commit_all "$T/gl" init
( cd "$T/gl" && git submodule -q add "$T/glsub" mods/a && git commit -qm sub ); printf 'zz \n' > "$T/gl/mods/zz.txt"; printf 'mods\n' > "$T/cs.lst"
( cd "$T/gl" && "$SC" --root "$T/gl" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; RC=$?; eq "m1: scope_check refuses a directory that starts with a gitlink: 20" "$RC" 20; has "named declared_directory" "$(cat "$T/err")" declared_directory
vc4fresh; rm -rf "$T/vr"; cp -r "$T/gl" "$T/vr"; vc4run; eq "m1: validate_cheap refuses a directory that starts with a gitlink: 20" "$RC" 20; has "named declared_directory" "$(cat "$T/err")" declared_directory
printf 'mods/a\n' > "$T/cs.lst"; ( cd "$T/gl" && "$SC" --root "$T/gl" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; eq "m1 golden-true: the gitlink itself is a declarable path: 0" "$?" 0
# m2  the S1 owned-submodule guard judges refs/heads/<run branch>, not the checked-out branch
fx; git -C "$W/sm" remote set-url origin 'git@example.invalid:Vasic-Digital/sm.git'; git -C "$W/sm" remote remove mirror
( cd "$W/sm" && echo own > own && git add own && $G commit -qm "hand commit in the submodule" && git checkout -q --detach ); : > "$CS"; ffrun --owned-orgs vasic-digital
eq "m2: a detached owned submodule whose run branch holds a hand commit: 20" "$RC" 20; eq "reason unrecorded_local_commit" "$(j .reason)" unrecorded_local_commit
# m3  one CPA-Run predicate for S1, S6 and the merge path: a non-trailer attribution line does not make S1 refuse the run's own commit
fx; RDIR="$W/.audit/commit-push/RUN1"; mkdir -p "$RDIR"; echo c2 >> "$W/f"
"$R4/commit_recursive.sh" --repo "$W" --run-dir "$RDIR" --run-id RUN1 --message "m" --attribution "Generated with Claude Code" -- f >"$P/cr.out" 2>"$P/cr.err"; eq "m3 setup: commit_recursive commits with a plain attribution line: 0" "$?" 0
ffrun; eq "m3: S1 accepts the run's own commit whatever the attribution line is: 0" "$RC" 0
# m4  the helpers never export GIT_LITERAL_PATHSPECS into repository hooks
mkhook() { mkdir -p "$1/.git/hooks"; printf '#!/bin/sh\necho "${GIT_LITERAL_PATHSPECS-unset}" > "%s"\nexit 0\n' "$2" > "$1/.git/hooks/$3"; chmod +x "$1/.git/hooks/$3"; }
rm -rf "$T/hk" "$T/hkrun"; mkrepo "$T/hk"; echo a > "$T/hk/a.txt"; commit_all "$T/hk" init; mkdir -p "$T/hkrun"; echo b > "$T/hk/b.sh"; mkhook "$T/hk" "$T/hook.env" pre-commit
"$R4/commit_recursive.sh" --repo "$T/hk" --run-dir "$T/hkrun" --run-id H1 --message m -- b.sh >/dev/null 2>&1; eq "m4: the pre-commit hook of commit_recursive sees no literal-pathspec variable" "$(cat "$T/hook.env" 2>/dev/null)" unset
printf '#!/bin/sh\nif git diff --cached --name-only -- "*.sh" | grep -q .; then echo hook-saw-sh >&2; exit 1; fi\nexit 0\n' > "$T/hk/.git/hooks/pre-commit"; echo c > "$T/hk/c.sh"
"$R4/commit_recursive.sh" --repo "$T/hk" --run-dir "$T/hkrun" --run-id H2 --message m -- c.sh >/dev/null 2>&1; eq "m4: a pre-commit hook that selects '*.sh' sees the staged file and refuses it: 20" "$?" 20
fx; mkhook "$W" "$P/hook.env" post-merge; other f2 two; ffrun; eq "m4: S1 fast-forwards" "$RC" 0; eq "m4: the post-merge hook of integrate_ff_only sees no literal-pathspec variable" "$(cat "$P/hook.env" 2>/dev/null)" unset
prfresh; mkhook "$M" "$T/hook.env" pre-push; rm -f "$T/hook.env"; pcpa "$M" . a.txt; prrun; eq "m4: S6 pushes" "$PRC" 0; eq "m4: the pre-push hook of push_recursive sees no literal-pathspec variable" "$(cat "$T/hook.env" 2>/dev/null)" unset
# m5  an explicit input file that is missing is a refusal, never an empty input
fx; ffrun --changeset-from "$P/nope.txt"; eq "m5: --changeset-from naming a missing file: 20" "$RC" 20; eq "reason changeset_unreadable" "$(j .reason)" changeset_unreadable
fx; ffrun --path-gates "$P/nope.tsv"; eq "m5: ff_only --path-gates naming a missing file: 20" "$RC" 20; eq "reason path_gates_unreadable" "$(j .reason)" path_gates_unreadable
im4fresh; im4run --path-gates "$T/nope.tsv"; eq "m5: integrate_merge --path-gates naming a missing file: 20" "$RC" 20; has "path_gates_unreadable named" "$(cat "$T/err")" path_gates_unreadable
# m6  push_recursive bounds ls-remote and push with --timeout
printf '#!/bin/sh\nexec sleep 8\n' > "$T/slowssh"; chmod +x "$T/slowssh"
prfresh; git -C "$M" remote set-url r2 "ssh://x@localhost/none.git"; git -C "$M" remote set-url r1 "ssh://x@localhost/none.git"; pcpa "$M" . a.txt; s0=$SECONDS
GIT_SSH_COMMAND="$T/slowssh" timeout 25 "$PRH" --main-root "$M" --branch main --run-dir "$PRUND" --timeout 2 >"$T/out" 2>"$T/err"; PRC=$?; el=$((SECONDS-s0))
eq "m6: a stalled ssh remote ends with 11 under --timeout 2 (never rc 124)" "$PRC" 11; [ "$el" -lt 20 ] && ok "m6: ended in ${el}s" || bad "m6: took ${el}s"
has "m6: unreachable reported" "$(cat "$T/out")" remote_unreachable
# m7  temporary files are removed on a kill too
mkdir -p "$T/tmpd"
rm -rf "$T/hk2"; mkrepo "$T/hk2"; echo a > "$T/hk2/a.txt"; commit_all "$T/hk2" init; mkdir -p "$T/hk2/.git/hooks"; printf '#!/bin/sh\nsleep 6\n' > "$T/hk2/.git/hooks/pre-commit"; chmod +x "$T/hk2/.git/hooks/pre-commit"; echo b > "$T/hk2/b.txt"
TMPDIR="$T/tmpd" timeout -s TERM 2 "$R4/commit_recursive.sh" --repo "$T/hk2" --run-dir "$T/hkrun" --run-id H3 --message m -- b.txt >/dev/null 2>&1; eq "m7 setup: commit_recursive was really terminated (124)" "$?" 124
eq "m7: commit_recursive killed during its commit leaves no cr_msg.* file" "$(ls "$T/tmpd" | grep -c '^cr_msg')" 0
rm -rf "$T/tmpd"; mkdir -p "$T/tmpd"; prfresh; git -C "$M" remote set-url r1 "ssh://x@localhost/none.git"; git -C "$M" remote set-url r2 "ssh://x@localhost/none.git"; pcpa "$M" . a.txt
TMPDIR="$T/tmpd" GIT_SSH_COMMAND="$T/slowssh" timeout -s TERM 2 "$PRH" --main-root "$M" --branch main --run-dir "$PRUND" >/dev/null 2>&1; eq "m7 setup: push_recursive was really terminated (124)" "$?" 124
eq "m7: push_recursive killed by SIGTERM leaves no push_recursive.* directory" "$(ls "$T/tmpd" | grep -c '^push_recursive')" 0
rm -rf "$T/tmpd"; mkdir -p "$T/tmpd"; vc4fresh; printf 'src/keep.txt\n' > "$T/cs.lst"; TMPDIR="$T/tmpd" vc4run
eq "m7: validate_cheap leaves no vc_tbl.* / vc_reg.* directory" "$(ls "$T/tmpd" | grep -c '^vc_')" 0
# round-2 minors: held rows
vc4fresh; printf 'src/keep.txt\tv/V.json\n' > "$T/held.tsv"; printf 'src/other.txt\n' > "$T/cs.lst"; echo o > "$T/vr/src/other.txt"; VHELD="$T/held.tsv" vc4run
eq "a held row that names no declared path is refused by validate_cheap: 20" "$RC" 20; has "held_row_unmatched" "$(cat "$T/err")" held_row_unmatched
printf 'src/keep.txt\tv/V.json\nsrc/keep.txt\tv/W.json\n' > "$T/held.tsv"; printf 'src/keep.txt\n' > "$T/cs.lst"; VHELD="$T/held.tsv" vc4run
eq "two held rows with different verdicts for one path: 20 (validate_cheap)" "$RC" 20; has "held_row_duplicate" "$(cat "$T/err")" held_row_duplicate
rm -rf "$T/cw" "$T/crun"; mkrepo "$T/cw"; echo a > "$T/cw/a.txt"; commit_all "$T/cw" init; echo b >> "$T/cw/a.txt"; mkdir -p "$T/crun"; printf 'a.txt\tv/V.json\na.txt\tv/W.json\n' > "$T/held2.tsv"
"$R4/commit_recursive.sh" --repo "$T/cw" --run-dir "$T/crun" --run-id H4 --message m --held-from "$T/held2.tsv" -- a.txt >"$T/out" 2>"$T/err"; RC=$?
eq "two held rows with different verdicts for one path: 20 (commit_recursive)" "$RC" 20; has "held_row_duplicate" "$(cat "$T/err")" held_row_duplicate
# m9  the two WP-04 evidence notes the real S3 reported without a revision header now carry one
"$R4/check_revision_headers.sh" --root "$D0" "$EVR/wp04/README.md" "$EVR/wp04/T038-findings.md" >"$T/out" 2>"$T/err"; eq "m9: README.md and T038-findings.md of the wp04 evidence folder carry the 11.4.44 header: 0" "$?" 0
fin
