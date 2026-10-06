#!/usr/bin/env bash
# T040 test (TDD): executing exit-code and mode matrix of scripts/repo/integrate_ff_only.sh (docs/16 section 12.2 S1).
#
# Fixtures  throwaway repositories under a temp dir with LOCAL BARE REMOTES only (never the real remotes); the helper never
#           pushes, and every case that matters asserts the remote tips are unchanged. No force operation anywhere.
# Usage     bash scripts/repo/tests/test_integrate_ff_only.sh       Env: H=<helper path> (the mutation driver points it at a copy)
# Covered   fetch per remote (fetch_failed:<remote>), fast-forward of the main repository only, needs_update for a behind
#           submodule, R4 pin proof (pin_not_on_remote 20), submodule local commits (12), ff_blocked_by_local_changes (12),
#           diverged / remotes_diverged (12), G-GATE routing against the approved manifest, adoption-commit first-parent
#           rule (C9 (c)), unrecorded_local_commit (20; the CPA predicate with its commits.tsv row), force refusal (20),
#           gitlinks moved by a fast-forward listed with old and new commit.
# Not here  integrate_merge.sh and the Foreign-Commit listing (T040 parts outside this helper's slice; see the evidence note).
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
H="${H:-scripts/repo/integrate_ff_only.sh}"; case "$H" in /*) ;; *) H="$(pwd)/$H" ;; esac
T="$(mktemp -d "${TMPDIR:-/tmp}/iff_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
n=0
# fx: build a main repo `w<n>` with two bare remotes (origin, mirror) both at c1 and a submodule `sm` (remotes origin, mirror).
fx() {
  n=$((n+1)); P="$T/c$n"; mkdir -p "$P"
  git init -q --bare -b main "$P/sm.git"; git init -q --bare -b main "$P/sm2.git"
  git clone -q "$P/sm.git" "$P/smw" 2>/dev/null; ( cd "$P/smw" && git checkout -q -b main 2>/dev/null; echo s1 > s; git add s; $G commit -qm s1; git push -q origin main; git remote add mirror "$P/sm2.git"; git push -q mirror main )
  git init -q --bare -b main "$P/o.git"; git init -q --bare -b main "$P/m.git"
  git clone -q "$P/o.git" "$P/w" 2>/dev/null
  ( cd "$P/w" && git checkout -q -b main 2>/dev/null; printf '/.audit/\n' > .gitignore; echo a > f; echo g > g; mkdir -p gate; echo v1 > gate/x
    $G submodule add -q "$P/sm.git" sm 2>/dev/null; git -C sm remote add mirror "$P/sm2.git"; git -C sm fetch -q mirror
    git add -A; $G commit -qm c1; git push -q origin main; git remote add mirror "$P/m.git"; git push -q mirror main )
  W="$P/w"; CS="$P/changeset.txt"; : > "$CS"; OUT="$P/out.json"
}
# other_commit <file> <content> [remotes...]: a second clone commits and pushes to the named remote(s) (default both)
other() {
  local f="$1" c="$2"; shift 2; local rs="${*:-origin mirror}"; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null
  ( cd "$P/oc" && git remote add mirror "$P/m.git" && mkdir -p "$(dirname "$f")" && echo "$c" > "$f" && git add "$f" && $G commit -qm "other $f"
    for r in $rs; do git push -q "$r" HEAD:main; done )
}
run() { "$H" --root "$W" --main-only --report-behind-submodules --changeset-from "$CS" --json "$OUT" "$@" >"$P/stdout" 2>"$P/stderr"; RC=$?; }
j() { jq -r "$1" "$OUT" 2>/dev/null || echo "?"; }
head_() { git -C "$W" rev-parse HEAD; }
tips() { git ls-remote "$P/o.git" refs/heads/main; git ls-remote "$P/m.git" refs/heads/main; }

# 1 golden: nothing to integrate
fx; run;                                         eq "up to date: exit" "$RC" 0
eq "up to date: status" "$(j .status)" up_to_date
# 2 fast-forward of the main repository only; submodule untouched; remotes untouched
fx; other f2 two; h0="$(head_)"; sm0="$(git -C "$W/sm" rev-parse HEAD)"; t0="$(tips)"
run;                                             eq "fast-forward: exit" "$RC" 0
eq "fast-forward: status" "$(j .status)" fast_forwarded
eq "fast-forward: HEAD is the remote tip" "$(head_)" "$(git -C "$P/o.git" rev-parse main)"
eq "fast-forward: submodule HEAD unmoved" "$(git -C "$W/sm" rev-parse HEAD)" "$sm0"
eq "fast-forward: remote tips unchanged (the helper never pushes)" "$(tips)" "$t0"
eq "fast-forward: report names the old HEAD" "$(j .moved.from)" "$h0"
# 2b the helper never pushes: a remote that is behind (the mirror) stays behind after the main repository fast-forwarded
fx; other f2 two origin; mt0="$(git ls-remote "$P/m.git" refs/heads/main)"; run
eq "behind mirror: exit" "$RC" 0
eq "behind mirror is not pushed to" "$(git ls-remote "$P/m.git" refs/heads/main)" "$mt0"
# 3 incoming commit touching a declared path blocks the fast-forward
fx; other f changed-upstream; h0="$(head_)"; echo f > "$CS"
run;                                             eq "ff blocked by declared path: exit" "$RC" 12
eq "ff blocked: reason" "$(j .reason)" ff_blocked_by_local_changes
eq "ff blocked: names the path" "$(j '.paths|join(",")')" f
eq "ff blocked: nothing moved" "$(head_)" "$h0"
# 3b golden-false: incoming commit touching another path is not blocked
fx; other f2 x; echo f > "$CS"; run;             eq "incoming on an undeclared path: exit" "$RC" 0
# 4 an uncommitted local change in a path the incoming commits touch: git refuses, 12 and nothing moved
fx; other g upstream-g; echo dirty >> "$W/g"; h0="$(head_)"
run;                                             eq "dirty overlap: exit" "$RC" 12
eq "dirty overlap: reason" "$(j .reason)" ff_blocked_by_local_changes
eq "dirty overlap: nothing moved" "$(head_)" "$h0"
# 5 diverged: local commit (a recorded CPA commit) and a remote commit
fx; other f2 up; ( cd "$W" && echo l > l && git add l && $G commit -qm "local" -m "CPA-Run: RUNX" ); mkdir -p "$W/.audit/commit-push/RUNX"
printf '.\t%s\tRUNX\n' "$(git -C "$W" rev-parse HEAD)" > "$W/.audit/commit-push/RUNX/commits.tsv"; h0="$(head_)"
run;                                             eq "diverged: exit" "$RC" 12
eq "diverged: reason" "$(j .reason)" diverged
eq "diverged: nothing moved" "$(head_)" "$h0"
# 6 a remote that cannot be fetched is recorded per remote while the others continue
fx; ( cd "$W" && git remote add bad "$P/does-not-exist.git" ); other f2 up
run;                                             eq "fetch_failed remote: exit (the others continue)" "$RC" 0
eq "fetch_failed:<remote> recorded" "$(j '[.remotes[]|select(.fetch!="ok")|.fetch]|join(",")')" "fetch_failed:bad"
eq "fetch_failed: the fast-forward still happened" "$(head_)" "$(git -C "$P/o.git" rev-parse main)"
# 7 a behind submodule is reported needs_update, never moved
fx; ( cd "$P/smw" && echo s2 > s && git add s && $G commit -qm s2 && git push -q origin main ); sm0="$(git -C "$W/sm" rev-parse HEAD)"
run;                                             eq "behind submodule: exit" "$RC" 0
eq "behind submodule: needs_update" "$(j '.submodules[]|select(.path=="sm")|.status')" needs_update
eq "behind submodule: never moved" "$(git -C "$W/sm" rev-parse HEAD)" "$sm0"
# 8 R4: a change-set gitlink whose pin every remote of the submodule holds is accepted with its proof
fx; echo sm > "$CS"; run;                        eq "pin on every remote: exit" "$RC" 0
eq "pin proof recorded" "$(j '.pins[]|select(.path=="sm")|.accepted')" true
eq "pin proof names each remote" "$(j '.pins[]|select(.path=="sm")|.remotes|length')" 2
# 8b pin held by only one remote of its submodule: 20 pin_not_on_remote, nothing moved
fx; ( cd "$W/sm" && echo s3 > s && git add s && $G commit -qm s3 && git push -q origin main ); h0="$(head_)"; echo sm > "$CS"
run;                                             eq "pin missing on one remote: exit" "$RC" 20
eq "pin missing: reason" "$(j .reason)" pin_not_on_remote
eq "pin missing: nothing moved" "$(head_)" "$h0"
# 9 change-set submodule carrying local commits no remote holds: 12
fx; ( cd "$W/sm" && echo s4 > s && git add s && $G commit -qm s4 ); echo sm > "$CS"
run;                                             eq "submodule local commit in change set: exit" "$RC" 12
eq "submodule local commit: reason" "$(j .reason)" submodule_local_commits
# 9b golden-false: the same local commit with sm NOT in the change set is not refused
fx; ( cd "$W/sm" && echo s4 > s && git add s && $G commit -qm s4 ); : > "$CS"; run; eq "submodule local commit, not declared: exit" "$RC" 0
# 10 unrecorded_local_commit (the CPA predicate with its commits.tsv row)
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "hand commit" ); sha="$(head_)"; h0="$sha"
run;                                             eq "trailer-less local commit: exit" "$RC" 20
eq "trailer-less: reason" "$(j .reason)" unrecorded_local_commit
eq "trailer-less: names the commit" "$(j '.commits|join(",")')" "$sha"
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "forged" -m "CPA-Run: RUNZ" ); run
eq "copied trailer, run unknown: exit" "$RC" 20
mkdir -p "$W/.audit/commit-push/RUNZ"; printf '.\t%s\tRUNZ\n' "$(head_)" > "$W/.audit/commit-push/RUNZ/commits.tsv"; run
eq "trailer plus the run's commits.tsv row: exit" "$RC" 0
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "x" -m "CPA-Run: RUNQ" ); mkdir -p "$W/.audit/commit-push/RUNQ"; printf '.\tdeadbeef\tRUNQ\n' > "$W/.audit/commit-push/RUNQ/commits.tsv"; run
eq "trailer, run lists another sha: exit" "$RC" 20
fx; ( cd "$W" && echo l > l && git add l && $G commit -qm "pushed hand commit" && git push -q origin main && git push -q mirror main ); run
eq "hand commit already on every remote: exit (not local-only)" "$RC" 0
# 11 G-GATE routing against the approved manifest
gate_fx() { fx; printf 'gate/x\tG-GATE\n' > "$P/gates.tsv"; echo '{}' > "$P/approved.json"; }
gate_fx; other gate/x v2; h0="$(head_)"
run --path-gates "$P/gates.tsv" --approved "$P/approved.json"; eq "G-GATE path not in manifest: exit" "$RC" 12
eq "G-GATE: reason" "$(j .reason)" merge_path_required
eq "G-GATE: names the path" "$(j '.gate_paths|join(",")')" gate/x
eq "G-GATE: not fast-forwarded" "$(head_)" "$h0"
sha2="$(printf 'v2\n' | sha256sum | cut -d' ' -f1)"; printf '{"gate/x":"%s"}\n' "$sha2" > "$P/approved.json"
run --path-gates "$P/gates.tsv" --approved "$P/approved.json"; eq "G-GATE content equal to the manifest entry: exit" "$RC" 0
eq "G-GATE approved: fast-forwarded" "$(j .status)" fast_forwarded
gate_fx; printf '{"gate/x":"%s"}\n' "$(printf 'other\n' | sha256sum | cut -d' ' -f1)" > "$P/approved.json"; other gate/x v2
run --path-gates "$P/gates.tsv" --approved "$P/approved.json"; eq "G-GATE content differs from its manifest entry: exit" "$RC" 12
gate_fx; printf '{"gate/x":"%s"}\n' "$(printf 'v1\n' | sha256sum | cut -d' ' -f1)" > "$P/approved.json"
rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && git rm -q gate/x && $G commit -qm del && git push -q origin HEAD:main && git remote add mirror "$P/m.git" && git push -q mirror HEAD:main )
run --path-gates "$P/gates.tsv" --approved "$P/approved.json"; eq "G-GATE deletion of an approved path: exit" "$RC" 12
gate_fx; other other/y 1; run --path-gates "$P/gates.tsv" --approved "$P/approved.json"; eq "G-GATE golden-false: change on a non-gate path: exit" "$RC" 0
# 12 adoption commit: the tip's first-parent chain must hold it (CENTRAL C9 (c))
fx; base="$(head_)"; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null
( cd "$P/oc" && git remote add mirror "$P/m.git" && git checkout -q -b side && echo a > side && git add side && $G commit -qm A && A="$(git rev-parse HEAD)" && echo "$A" > "$P/adopt"
  git checkout -q main && echo b > b && git add b && $G commit -qm B && $G merge -q --no-ff side -m M && git push -q origin main && git push -q mirror main )
run --adoption-commit "$(cat "$P/adopt")"; eq "adoption commit off the tip's first-parent chain: exit" "$RC" 12
eq "adoption: reason" "$(j .reason)" merge_path_required
eq "adoption: not fast-forwarded" "$(head_)" "$base"
run --adoption-commit "$base";                  eq "adoption commit on the first-parent chain: exit" "$RC" 0
# 13 two remote tips that neither descends from the other
fx; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && git remote add mirror "$P/m.git" && echo 1 > r1 && git add r1 && $G commit -qm r1 && git push -q origin HEAD:main \
  && git reset -q --hard HEAD~1 && echo 2 > r2 && git add r2 && $G commit -qm r2 && git push -q mirror HEAD:main ); h0="$(head_)"
run;                                             eq "remote tips diverged from each other: exit" "$RC" 12
eq "remotes_diverged: reason" "$(j .reason)" remotes_diverged
eq "remotes_diverged: nothing moved" "$(head_)" "$h0"
# 14 a fast-forward that moves a submodule pointer lists the gitlink with its old and new commit
fx; rm -rf "$P/oc"; git clone -q "$P/o.git" "$P/oc" 2>/dev/null; ( cd "$P/oc" && git remote add mirror "$P/m.git" && $G submodule update -q --init sm 2>/dev/null
  git -C sm checkout -q -b main 2>/dev/null; echo s9 > sm/s; git -C sm add s; $G -C sm commit -qm s9; git -C sm push -q origin main; git -C sm push -q origin HEAD:main 2>/dev/null
  git add sm && $G commit -qm bump && git push -q origin main && git push -q mirror main ); smold="$(git -C "$W/sm" rev-parse HEAD)"
run;                                             eq "gitlink move by ff: exit" "$RC" 0
eq "gitlink listed by path" "$(j '.moved.gitlinks[0].path')" sm
eq "gitlink old commit" "$(j '.moved.gitlinks[0].old')" "$smold"
eq "gitlink new commit is the submodule's pushed tip" "$(j '.moved.gitlinks[0].new')" "$(git -C "$P/sm.git" rev-parse main)"
eq "submodule worktree still unmoved" "$(git -C "$W/sm" rev-parse HEAD)" "$smold"
# 15 force and rewrite options are refused, nothing moves, no push
for opt in --force --force-with-lease +main --rebase --reset; do fx; other f2 x; h0="$(head_)"; t0="$(tips)"; run "$opt"
  eq "option $opt refused: exit" "$RC" 20; eq "option $opt: reason" "$(j .reason)" force_refused
  eq "option $opt: nothing moved" "$(head_)|$(tips)" "$h0|$t0"; done
fx; run --no-such-option;                         eq "unknown option: exit" "$RC" 20
echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "$H" ]
