#!/usr/bin/env bash
# WP-04 fix round 3 test (TDD): the findings of the xhigh review WF2-REVIEW-wp04-helpers (B1, B2, I1-I9, minors, survivors W1-W4, W7, W8).
# Throwaway repositories and LOCAL BARE remotes only; no real remote, no network, no credential.
# Usage: bash scripts/repo/tests/test_wp04c.sh        Env: R3=<dir that holds the helpers> (the mutation driver points it at a copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; R3="${R3:-$D0/scripts/repo}"; case "$R3" in /*) ;; *) R3="$D0/$R3" ;; esac
H="$R3/commit_recursive.sh"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04c.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable"
export GIT_SSH_COMMAND=false
RC=0
# ======================================================================================================================
# B1  declared paths are literal paths: spelling variants are refused, git sees literal pathspecs
# ======================================================================================================================
CR="$R3/commit_recursive.sh"
crfresh() { rm -rf "$T/w" "$T/run"; mkrepo "$T/w"; printf '/.audit/\n' > "$T/w/.gitignore"; mkdir -p "$T/w/d" "$T/run"
  echo a > "$T/w/d/a.txt"; echo b > "$T/w/b.txt"; echo base > "$T/w/base.txt"; echo f > "$T/w/f.txt"; echo ab > "$T/w/ab.txt"; commit_all "$T/w" init
  echo a2 >> "$T/w/d/a.txt"; echo b2 >> "$T/w/b.txt"; echo tc >> "$T/w/base.txt"; echo un > "$T/w/undeclared.txt"; echo f2 >> "$T/w/f.txt"; }
crrun() { "$CR" --repo "$T/w" --run-dir "$T/run" --run-id run1 --message "m" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
crfresh; h0="$(git -C "$T/w" rev-parse HEAD)"
for sp in '*' ':/' '.' './' './b.txt' 'd/' 'd/./a.txt' ':(glob)*.txt' ':x' 'a?.txt' '[ab].txt' 'd/*'; do
  crrun -- "$sp"; eq "commit_recursive refuses the spelling '$sp': 20" "$RC" 20
  eq "no commit made by the spelling '$sp'" "$(git -C "$T/w" rev-parse HEAD)" "$h0"
  eq "no commits.tsv row for '$sp'" "$(test -e "$T/run/commits.tsv"; echo $?)" 1
done
crrun -- d; eq "a directory is no declared path: 20" "$RC" 20; has "named declared_directory" "$(cat "$T/err")" declared_directory
eq "no commit for the directory" "$(git -C "$T/w" rev-parse HEAD)" "$h0"
crrun -- b.txt 'e.txt' ; eq "a path neither tracked nor present is still refused: 20" "$RC" 20
crfresh; crrun -- d/a.txt; eq "golden-true: the literal path d/a.txt commits: 0" "$RC" 0
eq "only the declared file is in the commit" "$(git -C "$T/w" show --name-only --format= HEAD)" "d/a.txt"
eq "the undeclared tracked change stays unstaged" "$(git -C "$T/w" status --porcelain -- base.txt)" " M base.txt"
eq "the undeclared file stays untracked" "$(git -C "$T/w" status --porcelain -- undeclared.txt)" "?? undeclared.txt"
# a gitlink (a pin move) is the one directory that may be declared; a path inside a submodule never may
crfresh; mkrepo "$T/csub"; echo s > "$T/csub/f"; commit_all "$T/csub" s; ( cd "$T/w" && git add -A && git commit -qm wip && git submodule -q add "$T/csub" mods/sub && git commit -qm sub )
echo n > "$T/w/mods/sub/n"; git -C "$T/w/mods/sub" add n; git -C "$T/w/mods/sub" commit -qm n
crrun -- mods/sub; eq "a gitlink path is declared as the pin it is: 0" "$RC" 0; eq "the commit holds the gitlink only" "$(git -C "$T/w" show --name-only --format= HEAD)" "mods/sub"
crrun -- mods/sub/n; eq "a path inside a submodule: 20" "$RC" 20; has "named path_in_submodule" "$(cat "$T/err")" path_in_submodule
# held rows: a spelling variant never matches silently; a row naming no declared path is refused; a valid row holds
crfresh; printf './d/a.txt\tv/V.json\n' > "$T/held.tsv"; crrun --held-from "$T/held.tsv" -- d/a.txt
eq "held row spelled ./d/a.txt: 20" "$RC" 20; has "named unsafe_held_row" "$(cat "$T/err")" unsafe_held_row
eq "no commit under the bad held row" "$(git -C "$T/w" log --oneline | wc -l | tr -d ' ')" 1
crfresh; printf 'zzz.txt\tv/V.json\n' > "$T/held.tsv"; crrun --held-from "$T/held.tsv" -- d/a.txt
eq "held row that matches no declared path: 20" "$RC" 20; has "named held_row_unmatched" "$(cat "$T/err")" held_row_unmatched
eq "no commit under the unmatched held row" "$(git -C "$T/w" log --oneline | wc -l | tr -d ' ')" 1
crfresh; printf 'd/a.txt\tv/V.json\n' > "$T/held.tsv"; crrun --held-from "$T/held.tsv" -- d/a.txt b.txt
eq "golden-true held row: 0" "$RC" 0; has "the held commit names its verdict" "$(git -C "$T/w" log -1 --format=%B)" "Awaits-Review: v/V.json"
eq "the held file is in the held commit only" "$(git -C "$T/w" show --name-only --format= HEAD)" "d/a.txt"

# scope_check (S2): spellings, directories, uninitialised submodules
SC="$R3/scope_check.sh"; EV=ev
scfresh() { rm -rf "$T/s" "$T/sub"; mkrepo "$T/s"; mkdir -p "$T/s/$EV" "$T/s/src" "$T/s/mods"; printf 'l1\nl2\n' > "$T/s/$EV/ledger.jsonl"; echo a > "$T/s/src/a.go"
  mkrepo "$T/sub"; echo s > "$T/sub/f.txt"; commit_all "$T/sub" s; ( cd "$T/s" && git submodule -q add "$T/sub" mods/sub ); commit_all "$T/s" init; }
scrun() { ( cd "$T/s" && "$SC" --root "$T/s" --ev "$EV" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; RC=$?; }
scfresh; printf 'l1\nL2\n' > "$T/s/$EV/ledger.jsonl"
printf '%s\n' "$EV/ledger.jsonl" > "$T/cs.lst"; scrun; eq "control: the canonical spelling of a rewritten ledger: 13" "$RC" 13
for sp in "./$EV/ledger.jsonl" "$EV/./ledger.jsonl" "$EV/" "$EV" '*' ':/' '.' "$EV/led*.jsonl" ':(glob)ev/*'; do
  printf '%s\n' "$sp" > "$T/cs.lst"; scrun; eq "scope_check refuses the spelling '$sp': 20" "$RC" 20
done
printf 'src/a.go\n' > "$T/cs.lst"
for sp in ./ev ev/./x '*' . ':/' 'e?'; do ( cd "$T/s" && "$SC" --root "$T/s" --ev "$sp" --exceptions /dev/null --paths-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; eq "scope_check refuses --ev '$sp': 20" "$?" 20; done
printf 'src\n' > "$T/cs.lst"; scrun; eq "a directory is no declared path (src): 20" "$RC" 20; has "named declared_directory" "$(cat "$T/err")" declared_directory
printf 'mods/sub\n' > "$T/cs.lst"; scrun; eq "a gitlink path is declared as the pin it is: 0" "$RC" 0
scfresh; git -C "$T/s" submodule -q deinit -f mods/sub; echo untracked > "$T/s/u.txt"; printf 'src/a.go\n' > "$T/cs.lst"
scrun; eq "uninitialised submodule never walks to the parent (W3): 0" "$RC" 0; hasnot "no dirty_submodule for the empty directory" "$(cat "$T/out")" dirty_submodule

# validate_cheap (S3)
VCH="$R3/validate_cheap.sh"
vcfresh() { rm -rf "$T/vr" "$T/code"; mkrepo "$T/vr"; mkdir -p "$T/vr/src" "$T/vr/dir"; echo ok > "$T/vr/src/keep.txt"; echo d > "$T/vr/dir/x.txt"; commit_all "$T/vr" init
  mkdir -p "$T/code/scripts/repo" "$T/code/scripts/hooks"; cp "$R3"/*.sh "$R3"/*.tsv "$R3"/fixture_roots.txt "$T/code/scripts/repo/"
  printf '#!/usr/bin/env bash\n[ -e ./LANDMINE ] && { echo landmine; exit 1; }\nexit 0\n' > "$T/code/scripts/detect-landmines.sh"; chmod 644 "$T/code/scripts/detect-landmines.sh"
  cp "$D0/scripts/hooks/no-false-positive-log.sh" "$T/code/scripts/hooks/"; }
vcrun() { ( cd "$T/vr" && "$VCH" --root "$T/vr" --adopt-working-tables --code-root "$T/code" --registry "$T/code/scripts/repo/validate_checks.tsv" ${VHELD:+--held-from "$VHELD"} --files-from "$T/cs.lst" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
vcfresh
printf 'src/keep.txt\n' > "$T/cs.lst"; vcrun; eq "golden-true change set through the shipped registry (mode 644 landmine script): 0" "$RC" 0
for sp in './src/keep.txt' 'src/' '*' ':/' '.' 'src/./keep.txt' 'src/ke?p.txt' ':(glob)src/*'; do printf '%s\n' "$sp" > "$T/cs.lst"; vcrun; eq "validate_cheap refuses the spelling '$sp': 20" "$RC" 20; done
printf 'dir\n' > "$T/cs.lst"; vcrun; eq "a directory is no declared path: 20" "$RC" 20; has "named declared_directory" "$(cat "$T/err")" declared_directory
mkrepo "$T/vsub"; echo s > "$T/vsub/f"; commit_all "$T/vsub" s; ( cd "$T/vr" && git submodule -q add "$T/vsub" mods/sub && git commit -qm sub ); printf 'mods/sub\n' > "$T/cs.lst"; vcrun
eq "a gitlink path is declared as the pin it is (validate_cheap): 0" "$RC" 0
printf 'src/keep.txt\n' > "$T/cs.lst"; printf './src/keep.txt\tv/V.json\n' > "$T/held.tsv"; VHELD="$T/held.tsv" vcrun; eq "a held row with a ./ spelling: 20" "$RC" 20
# legacy row cannot be dodged by a spelling (the S3 bypass of the review)
vcfresh; printf 'ALL_ISSUES_FIXED.md\n' > "$T/cs.lst"; echo "# old" > "$T/vr/ALL_ISSUES_FIXED.md"; commit_all "$T/vr" legacy
# (the legacy row of the shipped table names this exact path; the committed copy is the HEAD content)
echo "# edited" >> "$T/vr/ALL_ISSUES_FIXED.md"; vcrun; eq "legacy report edited, declared canonical: 10" "$RC" 10; has "legacy_row_not_dropped" "$(cat "$T/out")" legacy_row_not_dropped
printf './ALL_ISSUES_FIXED.md\n' > "$T/cs.lst"; vcrun; eq "legacy report declared as ./ALL_ISSUES_FIXED.md is refused, not passed: 20" "$RC" 20

# ======================================================================================================================
# I1  a HEAD without the admission tables never makes the working-tree tables the HEAD tables
# ======================================================================================================================
vcfresh
mkdir -p "$T/vr/scripts/repo"; cp "$R3/check_classes.tsv" "$R3/check_exemptions.tsv" "$R3/fixture_roots.txt" "$T/vr/scripts/repo/"
printf 'x.txt\tgenerated\tprobe row\tT\n' >> "$T/vr/scripts/repo/check_exemptions.tsv"; printf 'a \n' > "$T/vr/x.txt"
printf 'x.txt\n' > "$T/cs.lst"; vcrun
eq "undeclared untracked table row never exempts the declared file (I1): 10" "$RC" 10; has "the whitespace failure is reported" "$(cat "$T/out")" trailing_whitespace
hasnot "no table change is claimed when the table is undeclared" "$(cat "$T/out")" class_table_unreviewed
printf 'x.txt\nscripts/repo/check_exemptions.tsv\n' > "$T/cs.lst"; vcrun
eq "table declared with its file, no HEAD table, no review verdict: 10" "$RC" 10; has "class_table_unreviewed (I1)" "$(cat "$T/out")" class_table_unreviewed
has "the file is still judged with the reviewed tables" "$(cat "$T/out")" trailing_whitespace
# W8: an undeclared working-tree edit of a table that HEAD holds is not applied either
vcfresh; mkdir -p "$T/vr/scripts/repo"; cp "$R3/check_classes.tsv" "$R3/check_exemptions.tsv" "$R3/fixture_roots.txt" "$T/vr/scripts/repo/"; commit_all "$T/vr" tables
printf 'x.txt\tgenerated\tprobe row\tT\n' >> "$T/vr/scripts/repo/check_exemptions.tsv"; printf 'a \n' > "$T/vr/x.txt"; printf 'x.txt\n' > "$T/cs.lst"; vcrun
eq "undeclared working-tree table edit is not evaluated (W8): 10" "$RC" 10; has "whitespace failure kept" "$(cat "$T/out")" trailing_whitespace

# ======================================================================================================================
# I2  the shipped registry runs against the shipped tree: every row's command resolves the way the helper resolves it
# ======================================================================================================================
reg_resolve="$(python3 - "$R3" "$D0" <<'PY'
import os, sys, re
R3, root = sys.argv[1], sys.argv[2]; bad = []
for ln in open(os.path.join(R3, 'validate_checks.tsv')):
    if ln.startswith('#') or not ln.strip() or ln.startswith('check\t'): continue
    c = ln.rstrip('\n').split('\t'); name, cmd, mode = c[0], c[1], c[3]
    if mode == 'deferred' or cmd.startswith('builtin:'): continue
    toks = cmd.split(' ')
    if toks[0] in ('bash', 'python3'): script = toks[1]; need_x = False
    else: script = toks[0]; need_x = True
    p = os.path.join(root, script)
    if not os.path.isfile(p): bad.append(f'{name}: {script} absent')
    elif need_x and not os.access(p, os.X_OK): bad.append(f'{name}: {script} is not executable and the row names no interpreter')
print('ok' if not bad else '; '.join(bad))
PY
)"
eq "every shipped registry row resolves against the real tree (I2)" "$reg_resolve" ok
vcfresh; printf 'src/keep.txt\n' > "$T/cs.lst"; : > "$T/vr/LANDMINE"; vcrun; eq "the landmine script really runs through its interpreter (a LANDMINE file fails it): 10" "$RC" 10; has "detect_landmines named" "$(cat "$T/out")" detect_landmines
vcfresh; cp "$T/code/scripts/repo/validate_checks.tsv" "$T/reg2.tsv"; sed -i 's#^detect_landmines\t[^\t]*#detect_landmines\tscripts/detect-landmines.sh#' "$T/reg2.tsv"
( cd "$T/vr" && "$VCH" --root "$T/vr" --adopt-working-tables --code-root "$T/code" --registry "$T/reg2.tsv" --files-from "$T/cs.lst" ) >"$T/out" 2>"$T/err"; RC=$?
eq "golden-false: a direct row naming a non-executable file is still refused: 20" "$RC" 20; has "check_command_absent" "$(cat "$T/err")" check_command_absent

# ======================================================================================================================
# B2  the R4 pin proof refuses a pin no remote holds
# ======================================================================================================================
FF="$R3/integrate_ff_only.sh"; G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
n=0
fx() {
  n=$((n+1)); P="$T/f$n"; mkdir -p "$P"
  git init -q --bare -b main "$P/sm.git"; git init -q --bare -b main "$P/sm2.git"
  git clone -q "$P/sm.git" "$P/smw" 2>/dev/null; ( cd "$P/smw" && git checkout -q -b main 2>/dev/null; echo s1 > s; git add s; $G commit -qm s1; git push -q origin main; git remote add mirror "$P/sm2.git"; git push -q mirror main )
  git init -q --bare -b main "$P/o.git"; git init -q --bare -b main "$P/m.git"
  git clone -q "$P/o.git" "$P/w" 2>/dev/null
  ( cd "$P/w" && git checkout -q -b main 2>/dev/null; printf '/.audit/\n' > .gitignore; echo a > f; echo g > g
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
fx; echo sm > "$CS"; ffrun; eq "control: pin on every remote: 0" "$RC" 0
fx; echo sm > "$CS"; git -C "$W/sm" remote remove origin; git -C "$W/sm" remote remove mirror; h0="$(head_)"; ffrun
eq "submodule with no remote at all (B2a): 20" "$RC" 20; eq "reason pin_not_on_remote" "$(j .reason)" pin_not_on_remote
eq "the proof is not accepted" "$(j '.pins[]|select(.path=="sm")|.accepted')" false; eq "nothing moved" "$(head_)" "$h0"
fx; other f2 two; echo sm > "$CS"; git -C "$W" submodule -q deinit -f sm; h0="$(head_)"; ffrun
eq "uninitialised submodule: the parent's HEAD is never the pin (B2b): 20" "$RC" 20; eq "reason pin_unverifiable" "$(j .reason)" pin_unverifiable
eq "no pin proof row accepted for the parent's HEAD" "$(j '[.pins[]|select(.accepted==true)]|length')" 0; eq "nothing moved" "$(head_)" "$h0"
fx; echo sm > "$CS"; git -C "$W/sm" remote set-url origin "$P/gone1.git"; git -C "$W/sm" remote set-url mirror "$P/gone2.git"; ffrun
eq "only unreachable remotes: 20" "$RC" 20; eq "reason pin_unverifiable (not an unproven lack)" "$(j .reason)" pin_unverifiable
eq "the proof records fetch failed per remote" "$(j '[.pins[]|select(.path=="sm")|.remotes[]|.fetch]|unique|join(",")')" failed
fx; echo sm > "$CS"; git -C "$W/sm" remote set-url mirror "$P/gone2.git"; ffrun
eq "one remote holds the pin, the other is unreachable: 20" "$RC" 20; eq "reason pin_unverifiable" "$(j .reason)" pin_unverifiable
fx; ( cd "$W/sm" && echo s3 > s && git add s && $G commit -qm s3 && git push -q origin main ); echo sm > "$CS"; ffrun
eq "golden-false for the pin-lack path: a reachable remote that lacks the pin keeps pin_not_on_remote: 20" "$RC" 20; eq "reason" "$(j .reason)" pin_not_on_remote

# ======================================================================================================================
# I5  unrecorded_local_commit fails closed when a remote tip is not held locally
# ======================================================================================================================
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "hand commit" )
rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && echo z > z && git add z && $G commit -qm foreign && git push -q origin HEAD:main ); ftip="$(git -C "$P/o.git" rev-parse main)"
rm -f "$P/o.git/objects/${ftip:0:2}/${ftip:2}"; eq "setup: the foreign tip object is gone from the remote" "$(test -e "$P/o.git/objects/${ftip:0:2}/${ftip:2}"; echo $?)" 1
h0="$(head_)"; ffrun
eq "a remote tip whose objects cannot be fetched: 20 (fail closed, I5)" "$RC" 20; eq "reason remote_tip_not_held" "$(j .reason)" remote_tip_not_held; eq "nothing moved" "$(head_)" "$h0"
fx; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && echo z > z && git add z && $G commit -qm foreign && git push -q origin HEAD:main ); ftip="$(git -C "$P/o.git" rev-parse main)"
rm -f "$P/o.git/objects/${ftip:0:2}/${ftip:2}"; ffrun
eq "a remote tip whose objects cannot be fetched refuses even with no local commit (fail closed): 20" "$RC" 20; eq "reason remote_tip_not_held" "$(j .reason)" remote_tip_not_held
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "hand commit" ); ffrun; eq "control: a healthy remote still refuses the hand commit: 20" "$RC" 20; eq "reason unrecorded_local_commit" "$(j .reason)" unrecorded_local_commit

# ======================================================================================================================
# I6  owned-submodule guard: scp-form URLs, case-insensitive org match (W1)
# ======================================================================================================================
fx; git -C "$W/sm" remote set-url origin 'git@example.invalid:Vasic-Digital/sm.git'; git -C "$W/sm" remote remove mirror
( cd "$W/sm" && echo own > own && git add own && $G commit -qm "hand commit in the submodule" )
: > "$CS"; ffrun --owned-orgs VASIC-Digital,HelixDevelopment
eq "owned submodule with a local commit and an scp-form URL is judged (I6/W1): 20" "$RC" 20; eq "reason unrecorded_local_commit" "$(j .reason)" unrecorded_local_commit
fx; git -C "$W/sm" remote set-url origin 'git@example.invalid:Someone/sm.git'; git -C "$W/sm" remote remove mirror
( cd "$W/sm" && echo own > own && git add own && $G commit -qm "hand commit in a third-party submodule" ); ffrun --owned-orgs vasic-digital
eq "golden-false: a third-party submodule is not judged: 0" "$RC" 0

# ======================================================================================================================
# I7  integrate_ff_only is under lib_safe; I7/minor 1: option injection, a missing option value never loops
# ======================================================================================================================
fx; printf '#!/bin/sh\ntouch "%s"\n' "$P/pwned" > "$P/up.sh"; chmod +x "$P/up.sh"
git -C "$W" remote add -- "--upload-pack=$P/up.sh" "$P/o.git"; ffrun
eq "remote named like an option: 20 (I7)" "$RC" 20; eq "reason unsafe_remote_name" "$(j .reason)" unsafe_remote_name; eq "the option was never executed" "$(test -e "$P/pwned"; echo $?)" 1
fx; git -C "$W" config remote.rx.url "ext::sh -c 'touch $P/pwned2'"; ffrun; eq "ext:: remote URL: 20" "$RC" 20; eq "reason unsafe_remote_url" "$(j .reason)" unsafe_remote_url; eq "nothing executed" "$(test -e "$P/pwned2"; echo $?)" 1
fx; git -C "$W/sm" remote add -- "--upload-pack=$P/up.sh" "$P/sm.git"; ffrun; eq "submodule remote named like an option: 20" "$RC" 20; eq "the option was never executed (submodule)" "$(test -e "$P/pwned"; echo $?)" 1
for sp in './f' '*' 'f/./x' ':/' . 'f?'; do fx; printf '%s\n' "$sp" > "$CS"; ffrun; eq "ff_only change set spelled '$sp': 20" "$RC" 20; eq "reason unsafe_path for '$sp'" "$(j .reason)" unsafe_path; done
fx; printf 'f\n' > "$CS"; ffrun; eq "golden-true change set line f: 0" "$RC" 0
fx; ffrun --adoption-commit nothex; eq "an adoption commit that is no object name: 20" "$RC" 20; eq "reason unsafe_adoption_commit" "$(j .reason)" unsafe_adoption_commit
fx; ffrun --timeout abc; eq "--timeout that is no integer: 20" "$RC" 20
fx; ffrun --timeout 0; eq "--timeout 0: 20" "$RC" 20
fx; timeout 20 "$FF" --main-only --root >/dev/null 2>&1; eq "--root without a value ends with 20, never loops (minor 1)" "$?" 20
fx; timeout 20 "$FF" --main-only --json >/dev/null 2>&1; eq "--json without a value: 20" "$?" 20
fx; timeout 20 "$FF" --root -x --main-only >/dev/null 2>&1; eq "--root starting with a dash: 20" "$?" 20
RDF="$R3/record_deferral.sh"
timeout 20 "$RDF" --list --run-dir >/dev/null 2>&1; eq "record_deferral --run-dir without a value ends with 20 (minor 1)" "$?" 20
timeout 20 "$RDF" --run-dir "$T/rr" --flag >/dev/null 2>&1; eq "record_deferral --flag without a value: 20" "$?" 20
timeout 20 "$RDF" --run-dir "$T/rr" --flag SKIP_LONG --reason >/dev/null 2>&1; eq "record_deferral --reason without a value: 20" "$?" 20

# ======================================================================================================================
# I3  exact refs/heads/<branch> match: a decoy ref ending in refs/heads/<branch> decides nothing
# ======================================================================================================================
fx; base="$(head_)"; other f2 two origin
( cd "$P/oc" && git push -q origin "$base:refs/decoy/refs/heads/main" )
ffrun; eq "ff_only: a decoy ref sorting first does not hide the real tip (I3): 0" "$RC" 0; eq "the main repository fast-forwarded to the real tip" "$(head_)" "$(git -C "$P/o.git" rev-parse refs/heads/main)"
IM="$R3/integrate_merge.sh"; A="$T/audit"; RID=20261005T000000Z-1-aaaa; RUND="$A/$RID"
imfresh() { rm -rf "$T/m" "$T/b" "$T/c1" "$T/c2" "$A"; mkdir -p "$T/b" "$RUND"
  for r in r1 r2; do git init -q --bare -b main "$T/b/$r.git"; done
  mkrepo "$T/m"; printf '/.audit/\n' > "$T/m/.gitignore"; echo base > "$T/m/base.txt"; commit_all "$T/m" init; INIT="$(git -C "$T/m" rev-parse HEAD)"
  for r in r1 r2; do git -C "$T/m" remote add $r "$T/b/$r.git"; git -C "$T/m" push -q $r main; done
  git clone -q "$T/b/r1.git" "$T/c1"; git -C "$T/c1" config user.email t@t; git -C "$T/c1" config user.name t; }
imrun() { "$IM" --root "$T/m" --branch main --run-dir "$RUND" --ev ev "$@" >"$T/out" 2>"$T/err"; RC=$?; }
imfresh; ( cd "$T/m" && echo l > l.txt && git add l.txt && git commit -qm local ); ( cd "$T/c1" && echo r > r.txt && git add r.txt && git commit -qm remote && git push -q origin main && git push -q origin "$INIT:refs/decoy/refs/heads/main" )
imrun; eq "integrate_merge: a decoy ref does not hide the diverged real tip (I3): 0" "$RC" 0; has "the real tip was merged" "$(cat "$T/out")" MERGED
# push_recursive decoy
PRH="$R3/push_recursive.sh"; SCHEMA=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
M="$T/pm"; PA="$T/paudit"; PRUND="$PA/20261005T000000Z-1-aaaa"
prfresh() {
  rm -rf "$T/pm" "$T/pb" "$PA" "$T/pc" "$T/pc2"; mkdir -p "$T/pb" "$PRUND" "$PA/20261005T000000Z-2-bbbb"
  for r in r1 r2; do git init -q --bare -b main "$T/pb/$r.git"; done
  mkrepo "$M"; mkdir -p "$M/specs/ev/reviews" "$M/$(dirname "$SCHEMA")"; cp "$D0/$SCHEMA" "$M/$SCHEMA"
  printf '{"schema":"review-verdict/1","verdict":"NO-GO","covers_runs":[{"repository":".","commit":"0000000000000000000000000000000000000000","cpa_run":"20261005T000000Z-1-aaaa"}],"model":"opus","effort":"xhigh","blocking_findings":1}\n' > "$M/specs/ev/reviews/V_nogo.json"
  echo base > "$M/base.txt"; commit_all "$M" init
  for r in r1 r2; do git -C "$M" remote add $r "$T/pb/$r.git"; git -C "$M" push -q $r main; done
}
pcpa() { local d="$1" k="$2" f="$3" aw="${4-}" id="${5:-20261005T000000Z-1-aaaa}"; echo "$f $RANDOM" > "$d/$f"; git -C "$d" add -- "$f"
  git -C "$d" commit -q -m "change $f" -m "CPA-Run: $id${aw:+
Awaits-Review: $aw}"; printf '%s\t%s\t%s\n' "$k" "$(git -C "$d" rev-parse HEAD)" "$id" >> "$PA/$id/commits.tsv"; }
prrun() { timeout 20 "$PRH" --main-root "$M" --branch main --run-dir "$PRUND" "$@" >"$T/out" 2>"$T/err"; PRC=$?; }
ptip() { git -C "$T/pb/$1.git" rev-parse main 2>/dev/null || echo none; }
phead() { git -C "${1:-$M}" rev-parse HEAD; }
prfresh; pcpa "$M" . a.txt; lt="$(phead)"; git -C "$M" push -q r1 "$lt:refs/decoy/refs/heads/main"
prrun; eq "push_recursive: a decoy ref equal to the target does not suppress the push (I3): 0" "$PRC" 0; eq "r1's real main holds the target" "$(ptip r1)" "$lt"; has "PUSHED to r1" "$(cat "$T/out")" "PUSHED	.	r1	$lt"

# ======================================================================================================================
# I4  push_recursive --recursive: an uninitialised submodule never walks up to the parent and never runs away
# ======================================================================================================================
prfresh; git init -q --bare -b main "$T/psb.git"; git clone -q "$T/psb.git" "$T/psi" 2>/dev/null; ( cd "$T/psi" && git config user.email t@t && git config user.name t && echo s > s && git add s && git commit -qm s && git push -q origin HEAD:main ); rm -rf "$T/psi"
( cd "$M" && git submodule -q add -b main "$T/psb.git" zsub && git commit -qm "add zsub" && git push -q r1 main && git push -q r2 main ); git -C "$M" submodule -q deinit -f zsub
pcpa "$M" . m1.txt; prrun --recursive
eq "uninitialised submodule: ends, never runs away (I4): 0" "$PRC" 0; hasnot "no garbage repository key" "$(cat "$T/out")" "zsub/./"
hasnot "the uninitialised submodule is not reported as a repository" "$(cat "$T/out")" "	zsub	"; eq "main pushed" "$(ptip r1)" "$(phead)"
prrun --repo zsub; eq "an explicit --repo naming an uninitialised submodule is refused, never judged as the parent: 20" "$PRC" 20; has "not_a_repository" "$(cat "$T/out")" not_a_repository
for sp in ./ ./zsub zsub/ 'z*' ':/'; do prrun --repo "$sp"; eq "push_recursive refuses the --repo spelling '$sp': 20" "$PRC" 20; done
prrun --repo .; eq "golden-true: --repo . names the main repository: 0" "$PRC" 0

# ======================================================================================================================
# I8  push_recursive: every remote unreachable is NOPUSH remote_unreachable (11), never a refusal naming an innocent commit
# ======================================================================================================================
prfresh; pcpa "$M" . a.txt; git -C "$M" remote set-url r1 "$T/pb/gone1.git"; git -C "$M" remote set-url r2 "$T/pb/gone2.git"
prrun; eq "all remotes unreachable: 11 (I8)" "$PRC" 11; has "r1 named remote_unreachable" "$(cat "$T/out")" "NOPUSH	.	r1	remote_unreachable"; has "r2 named remote_unreachable" "$(cat "$T/out")" "NOPUSH	.	r2	remote_unreachable"
hasnot "no refusal naming a commit" "$(cat "$T/out")" REFUSED
prfresh; r1b="$(ptip r1)"; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand"; git -C "$M" remote set-url r1 "$T/pb/gone1.git"; git -C "$M" remote set-url r2 "$T/pb/gone2.git"
prrun; eq "an unrecorded commit with every remote unreachable is still never pushed: 11, no push" "$PRC" 11; eq "r1 unchanged" "$(ptip r1)" "$r1b"
# one reachable remote keeps judging with its tips
prfresh; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand"; git -C "$M" remote set-url r2 "$T/pb/gone2.git"; prrun
eq "golden-false: one reachable remote still refuses the unrecorded commit: 20" "$PRC" 20; has "unrecorded_local_commit" "$(cat "$T/out")" unrecorded_local_commit

# ======================================================================================================================
# I9  a held CPA merge of a remote that was NOT moved since S1 is a hold (14), not remote_moved_since_s1 (11)
# ======================================================================================================================
prfresh; git clone -q "$T/pb/r2.git" "$T/pc2"; ( cd "$T/pc2" && git config user.email t@t && git config user.name t && echo foreign > fr.txt && git add fr.txt && git commit -qm foreign && git push -q origin main ); r2tip="$(ptip r2)"
git -C "$M" fetch -q r2 main
pcpa "$M" . h.txt specs/ev/reviews/V_nogo.json
git -C "$M" merge -q --no-ff -m "Merge r2/main (CPA)" -m "CPA-Run: 20261005T000000Z-1-aaaa" "$r2tip"; printf '.\t%s\t20261005T000000Z-1-aaaa\n' "$(phead)" >> "$PA/20261005T000000Z-1-aaaa/commits.tsv"
prrun; eq "held commit under a merge of r2's own tip: 14 (I9)" "$PRC" 14; hasnot "r2 is not reported as moved" "$(cat "$T/out")" remote_moved_since_s1
has "r2 reported as held below its tip" "$(cat "$T/out")" "NOPUSH	.	r2	held_below_remote_tip"; eq "r2 untouched" "$(ptip r2)" "$r2tip"; has "HELD line" "$(cat "$T/out")" HELD
# golden-false: a remote that really diverged since S1 is still remote_moved_since_s1
prfresh; pcpa "$M" . a.txt; git clone -q "$T/pb/r2.git" "$T/pc2"; ( cd "$T/pc2" && git config user.email t@t && git config user.name t && echo other > o.txt && git add o.txt && git commit -qm other && git push -q origin main )
prrun; eq "golden-false: a remote that moved on its own: 11" "$PRC" 11; has "remote_moved_since_s1 kept" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"

# a remote that REALLY diverged since S1 is still moved while a held commit exists (the hold never hides a moved remote)
prfresh; pcpa "$M" . a.txt; pcpa "$M" . h.txt specs/ev/reviews/V_nogo.json; git clone -q "$T/pb/r2.git" "$T/pc2"; ( cd "$T/pc2" && git config user.email t@t && git config user.name t && echo other > o.txt && git add o.txt && git commit -qm other && git push -q origin main )
git -C "$M" fetch -q r2 main   # the diverged tip is held locally but is no ancestor of the local branch
prrun; eq "a held commit and a remote that moved on its own: 11 (the moved remote wins over the hold)" "$PRC" 11; has "r2 named remote_moved_since_s1" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"; has "the hold is still reported" "$(cat "$T/out")" HELD

# ======================================================================================================================
# W4  a remote tip whose objects are not local never enters --not: the CPA predicate still runs
# ======================================================================================================================
prfresh; pcpa "$M" . a.txt; echo hand > "$M/hand.txt"; git -C "$M" add hand.txt; git -C "$M" commit -qm "hand commit"
git clone -q "$T/pb/r2.git" "$T/pc2"; ( cd "$T/pc2" && git config user.email t@t && git config user.name t && echo foreign > fr.txt && git add fr.txt && git commit -qm foreign && git push -q origin main )
r1b="$(ptip r1)"; prrun
# round 6 (WF6 W6-1, conservative rule, owner decision F2 still open): an unrecorded commit together with a remote that MOVED (its live tip is not held locally) is
# 11 remote_moved_since_s1, no longer 20 naming the commit: the S1 fast-forward leaves refs/remotes/<r>/<branch> stale, so S6 cannot tell a hand commit from a commit
# S1 itself integrated; S1 judges the commit again once the moved tip is fetched. What stays (W4's point): the commit is NEVER published.
eq "an unrecorded commit with a remote tip unknown locally: 11, moved remote named, never published (W4, round 6 W6-1)" "$PRC" 11; has "names the moved remote" "$(cat "$T/out")" "NOPUSH	.	r2	remote_moved_since_s1"; hasnot "does not name the commit as unrecorded" "$(cat "$T/out")" unrecorded_local_commit; eq "nothing was pushed to r1" "$(ptip r1)" "$r1b"

# ======================================================================================================================
# minor 3  integrate_merge adopt_ok fails closed when the adoption commit cannot be resolved
# ======================================================================================================================
imfresh; ( cd "$T/c1" && echo r > r.txt && git add r.txt && git commit -qm remote && git push -q origin main ); unknown=1111111111111111111111111111111111111111
imrun --adoption-commit "$unknown"; eq "an unresolvable adoption commit routes the descending tip to the merge path (minor 3): 0" "$RC" 0; has "MERGED" "$(cat "$T/out")" MERGED

# minor 5  the section 9.2 backup copies untracked files that live inside an untracked directory
imfresh; ( cd "$T/m" && echo l > l.txt && git add l.txt && git commit -qm local ); ( cd "$T/c1" && echo r > r.txt && git add r.txt && git commit -qm remote && git push -q origin main )
mkdir -p "$T/m/ud/deep"; echo precious > "$T/m/ud/deep/f.txt"; imrun
eq "diverged tip merged with an untracked directory present: 0" "$RC" 0; eq "the untracked file inside the untracked directory is in the backup (minor 5)" "$(cat "$RUND/backup/worktree/ud/deep/f.txt" 2>/dev/null)" precious

# ======================================================================================================================
# minor 2  the S1 fetches never recurse into submodules (no ref of a submodule moves)
# ======================================================================================================================
fx; sm0="$(git -C "$W/sm" for-each-ref --format='%(refname) %(objectname)' | sort | tr '\n' ' ')"
rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && git remote add mirror "$P/m.git" && $G submodule update -q --init sm 2>/dev/null
  git -C sm checkout -q -b main 2>/dev/null; echo s9 > sm/s; git -C sm add s; $G -C sm commit -qm s9; git -C sm push -q origin main
  git add sm && $G commit -qm bump && git push -q origin main && git push -q mirror main )
ffrun; eq "main fast-forwarded across a gitlink move: 0" "$RC" 0
eq "no ref of the submodule moved by the main fetch (minor 2)" "$(git -C "$W/sm" for-each-ref --format='%(refname) %(objectname)' | sort | tr '\n' ' ')" "$sm0"
imfresh; ( cd "$T/m" && git submodule -q add "$P/sm.git" mods 2>/dev/null; git commit -qm addmods ); git -C "$T/m" push -q r1 main; git -C "$T/m" push -q r2 main
ms0="$(git -C "$T/m/mods" for-each-ref --format='%(refname) %(objectname)' | sort | tr '\n' ' ')"
( cd "$T/c1" && git pull -q origin main && $G submodule update -q --init mods 2>/dev/null; git -C mods checkout -q -b main 2>/dev/null; echo s8 > mods/s; git -C mods add s; $G -C mods commit -qm s8; git -C mods push -q origin HEAD:main; git add mods; git commit -qm bump2; git push -q origin main )
( cd "$T/m" && echo l > l.txt && git add l.txt && git commit -qm local ); imrun
eq "integrate_merge: the merge path never moves a submodule ref by fetch (minor 2)" "$(git -C "$T/m/mods" for-each-ref --format='%(refname) %(objectname)' | sort | tr '\n' ' ')" "$ms0"

# ======================================================================================================================
# minor 7  check_class: duplicate exact rows, fixture-root columns, a missing option value
# ======================================================================================================================
CC="$R3/check_class.sh"
printf 'p/dup.txt\tgenerated\treason\tT\np/dup.txt\tevidence\treason\tT\n' > "$T/ex_dup.tsv"
"$CC" --exemptions "$T/ex_dup.tsv" --classes "$R3/check_classes.tsv" --fixture-roots /dev/null --check class p/dup.txt >"$T/out" 2>"$T/err"; eq "two exact rows of different classes: 20 class_ambiguous (W7)" "$?" 20; has "named class_ambiguous" "$(cat "$T/err")" class_ambiguous
printf 'p/dup.txt\tgenerated\treason\tT\np/dup.txt\tgenerated\treason2\tT\n' > "$T/ex_dup2.tsv"
"$CC" --exemptions "$T/ex_dup2.tsv" --classes "$R3/check_classes.tsv" --fixture-roots /dev/null --check class p/dup.txt >"$T/out" 2>"$T/err"; eq "golden-false: the same class twice: 0" "$?" 0; eq "class generated" "$(cat "$T/out")" generated
printf 'fx/\ttrailing_whitespace\n' > "$T/fr2.txt"
"$CC" --exemptions "$R3/check_exemptions.tsv" --classes "$R3/check_classes.tsv" --fixture-roots "$T/fr2.txt" --check class fx/a.txt >"$T/out" 2>"$T/err"; eq "a fixture-root row with only two columns: 20 (the reader refuses it too)" "$?" 20; has "table_invalid" "$(cat "$T/err")" table_invalid
for opt in --exemptions --classes --fixture-roots --checks-registry --check; do "$CC" "$opt" >"$T/out" 2>"$T/err"; eq "$opt without a value: 20, never a traceback" "$?" 20; hasnot "no Python traceback for $opt" "$(cat "$T/err")" Traceback; done

# ======================================================================================================================
# B1 helper layer: the shared validator
# ======================================================================================================================
( . "$R3/lib_safe.sh"
  for g in a.txt d/a.txt 'd/a b.txt' 'a:b' '.hidden' 'd/.hidden/x' 'a.b/c'; do safe_declpath "$g" || echo "REFUSED-GOOD $g"; done
  for b in . ./ ./a a/ a/./b a/. '*' 'a*' 'a?' 'a[' ':x' ':/' '' '../a' '/a' '-a' 'a//b' 'a\b'; do safe_declpath "$b" && echo "ACCEPTED-BAD $b"; done; true ) >"$T/lib.out" 2>&1
eq "safe_declpath: golden-true paths accepted, every spelling refused" "$(cat "$T/lib.out")" ""
( . "$R3/lib_safe.sh"; printf 'sha1\trefs/decoy/refs/heads/main\nsha2\trefs/heads/main\nsha3\trefs/heads/mainx\n' | lr_exact main ) >"$T/lib2.out" 2>&1
eq "lr_exact: the exact ref only" "$(cat "$T/lib2.out")" "sha2"
fin
