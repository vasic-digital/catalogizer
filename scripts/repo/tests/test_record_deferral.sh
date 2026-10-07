#!/usr/bin/env bash
# T041 test (TDD): executing matrix of scripts/repo/record_deferral.sh (throwaway repo, no remotes, no network).
# Usage   bash scripts/repo/tests/test_record_deferral.sh        Env: H=<helper path> (the mutation driver points it at a copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
H="${H:-scripts/repo/record_deferral.sh}"; case "$H" in /*) ;; *) H="$(pwd)/$H" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/rd_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
G="git -c user.email=t@t -c user.name=t"
git init -q -b main "$T/w"; printf '/.audit/\n' > "$T/w/.gitignore"; ( cd "$T/w" && git add .gitignore && $G commit -qm i )
R="$T/w/.audit/commit-push/run1"
run() { "$H" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
# 1 golden: each closed-set flag is recorded, and --list prints them in closed-set order
run --run-dir "$R" --flag LOCAL_ONLY --reason "no remote";           eq "LOCAL_ONLY recorded" "$RC" 0
run --run-dir "$R" --flag SKIP_LONG --reason "long gate deferred";   eq "SKIP_LONG recorded" "$RC" 0
run --run-dir "$R" --flag SWEEP_ABSENT --reason "no sweep before WP-08"; eq "SWEEP_ABSENT recorded" "$RC" 0
run --run-dir "$R" --list;  eq "--list: closed-set order" "$(cat "$T/out")" "SKIP_LONG,SWEEP_ABSENT,LOCAL_ONLY"
eq "row count (header + 3)" "$(wc -l < "$R/deferrals.tsv" | tr -d ' ')" 4
eq "row carries flag and reason" "$(awk -F'\t' '$2=="SKIP_LONG"{print $3}' "$R/deferrals.tsv")" "long gate deferred"
# 2 duplicate flag is listed once
run --run-dir "$R" --flag SKIP_LONG --reason "again"; run --run-dir "$R" --list
eq "--list: duplicate flag listed once" "$(cat "$T/out")" "SKIP_LONG,SWEEP_ABSENT,LOCAL_ONLY"
# 2b the flags of a gate that did not run (WF11 review F3) are in the closed set, after the first three, and listed in that order
R2="$T/w/.audit/commit-push/run2"
for f in GATES_NOT_BUILT CHECKS_DEFERRED CHECK_PENDING_RELEASE; do run --run-dir "$R2" --flag "$f" --reason "gate not run"; eq "$f recorded" "$RC" 0; done
run --run-dir "$R2" --list; eq "--list: the three gate-not-run flags in closed-set order" "$(cat "$T/out")" "CHECK_PENDING_RELEASE,CHECKS_DEFERRED,GATES_NOT_BUILT"
run --run-dir "$R2" --flag SKIP_LONG --reason x; run --run-dir "$R2" --list; eq "--list: the first three keep their place ahead of them" "$(cat "$T/out")" "SKIP_LONG,CHECK_PENDING_RELEASE,CHECKS_DEFERRED,GATES_NOT_BUILT"
# 2c CHECKS_NOT_JUDGED (WF14 N4) is in the closed set, appended last
R3="$T/w/.audit/commit-push/run3"
run --run-dir "$R3" --flag CHECKS_NOT_JUDGED --reason "declared paths no check judged: src/link"; eq "CHECKS_NOT_JUDGED recorded" "$RC" 0
run --run-dir "$R3" --flag GATES_NOT_BUILT --reason x; run --run-dir "$R3" --list; eq "--list: CHECKS_NOT_JUDGED comes last in the closed set" "$(cat "$T/out")" "GATES_NOT_BUILT,CHECKS_NOT_JUDGED"
# 3 a held commit carries the verdict it waits for
run --run-dir "$R" --flag SKIP_LONG --reason held --commit abc123 --awaits-review specs/x/reviews/WP-04.json
eq "held commit with verdict recorded" "$RC" 0
eq "row carries awaits_review and commit" "$(tail -n1 "$R/deferrals.tsv" | awk -F'\t' '{print $4"|"$5}')" "specs/x/reviews/WP-04.json|abc123"
# 4 golden-false: refusals (20, and no row written)
n0="$(wc -l < "$R/deferrals.tsv")"
run --run-dir "$R" --flag NOT_A_FLAG --reason x;       eq "flag outside the closed set: 20" "$RC" 20
run --run-dir "$R" --flag skip_long --reason x;        eq "flag in wrong case: 20" "$RC" 20
run --run-dir "$R" --flag SKIP_LONG;                   eq "missing reason: 20" "$RC" 20
run --run-dir "$R" --flag SKIP_LONG --reason "";       eq "empty reason: 20" "$RC" 20
run --run-dir "$R" --flag SKIP_LONG --reason $'a\tb';  eq "tab in reason: 20" "$RC" 20
run --run-dir "$R" --flag SKIP_LONG --reason x --commit abc; eq "held commit without verdict: 20" "$RC" 20
run --reason x --flag SKIP_LONG;                        eq "no --run-dir: 20" "$RC" 20
run --run-dir "$R" --bogus;                             eq "unknown argument: 20" "$RC" 20
eq "no row written by any refusal" "$(wc -l < "$R/deferrals.tsv")" "$n0"
# 5 never into the tracked tree
mkdir -p "$T/w/tracked_dir"; echo x > "$T/w/tracked_dir/f"; ( cd "$T/w" && git add tracked_dir && $G commit -qm t )
run --run-dir "$T/w/tracked_dir" --flag SKIP_LONG --reason x; eq "run dir holding tracked files: 20" "$RC" 20
run --run-dir "$T/w/not_ignored_dir" --flag SKIP_LONG --reason x; eq "run dir not ignored by git: 20" "$RC" 20
eq "no deferrals.tsv in the tracked tree" "$(ls "$T/w/tracked_dir" "$T/w/not_ignored_dir" 2>/dev/null | grep -c deferrals)" 0
# 5b a run dir that git ignores but that holds a tracked file (force-added) is still refused: the row could reach a commit
mkdir -p "$T/w/.audit/commit-push/forced"; echo x > "$T/w/.audit/commit-push/forced/keep"; ( cd "$T/w" && git add -f .audit/commit-push/forced/keep && $G commit -qm forced )
run --run-dir "$T/w/.audit/commit-push/forced" --flag SKIP_LONG --reason x; eq "ignored run dir holding a tracked file: 20" "$RC" 20
# 6 a run dir outside any git work tree is accepted (golden-true carrier)
run --run-dir "$T/plain/run" --flag LOCAL_ONLY --reason ok; eq "run dir outside a work tree" "$RC" 0
# 7 failed write is 20, never success without the row
mkdir -p "$T/ro"; ( cd "$T/ro" && : ); mkdir -p "$T/w/.audit/commit-push/ro"; chmod 555 "$T/w/.audit/commit-push/ro"
if [ "$(id -u)" != 0 ]; then run --run-dir "$T/w/.audit/commit-push/ro" --flag SKIP_LONG --reason x; eq "unwritable run dir: 20 and no row" "$RC" 20; else ok "unwritable case skipped (root)"; fi
chmod 755 "$T/w/.audit/commit-push/ro"
# 7b an existing deferrals.tsv that cannot be appended to: 20, never a success without its row
mkdir -p "$T/w/.audit/commit-push/ro2"; printf 'utc_time\tflag\treason\tawaits_review\tcommit\n' > "$T/w/.audit/commit-push/ro2/deferrals.tsv"; chmod 444 "$T/w/.audit/commit-push/ro2/deferrals.tsv"
if [ "$(id -u)" != 0 ]; then run --run-dir "$T/w/.audit/commit-push/ro2" --flag SKIP_LONG --reason x; eq "read-only deferrals.tsv: 20 and no row" "$RC" 20
  eq "no row appended to the read-only file" "$(wc -l < "$T/w/.audit/commit-push/ro2/deferrals.tsv" | tr -d ' ')" 1; fi
chmod 644 "$T/w/.audit/commit-push/ro2/deferrals.tsv"
echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "$H" ]
