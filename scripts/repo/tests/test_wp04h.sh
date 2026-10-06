#!/usr/bin/env bash
# WF8 round 8 test (TDD): the cases of the independent round-6 review (WF7-REVIEW-wp04-verifier-r6) that the round-7 tests did not cover:
#   I-3  push_recursive.sh never publishes a parent whose changed gitlink points at a submodule commit that a remote of that submodule does not hold
#        (a fresh `clone --recurse-submodules` would fail `not our ref`; docs/16 S6 rule (b)), exit class 11 for the parent, nothing pushed for it;
#   M-3  every remaining `git remote` site of integrate_ff_only.sh (vet_remotes, the submodule fetch loop, the R4 pin loop, unrec, the owned test):
#        a failing `git remote` is exit 20 git_listing_failed, never "no remotes" (a counter shim fails the k-th call, k = 1..N);
#   INFO-1  integrate_merge.sh with EVERY remote unreachable is exit 11 remote_unreachable, never nothing_to_merge exit 0;
#   M-6  verify_repos.sh removes a stale --json FILE when the run exits 20 (a report that contradicts the exit status must not survive).
# Throwaway repositories and LOCAL BARE REMOTES only; the shims are PATH shims in the temp tree.
# Usage: bash scripts/repo/tests/test_wp04h.sh      Env: HS=<scripts/repo directory under test> (the RED run points it at the pre-fix copy)
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; HS="${HS:-scripts/repo}"; case "$HS" in /*) ;; *) HS="$D0/$HS" ;; esac
PR="$HS/push_recursive.sh"; FF="$HS/integrate_ff_only.sh"; IM="$HS/integrate_merge.sh"; VR="$HS/verify_repos.sh"; H="$PR"
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/wp04h_test.XXXXXX")"; trap 'rm -rf "${T:?}"' EXIT
for f in "$PR" "$FF" "$IM" "$VR"; do [ -x "$f" ] || echo "NOTE: $f is absent or not executable (RED state: every case must FAIL)"; done
echo "IDENTITY test=$(sha256sum "${BASH_SOURCE[0]}" | cut -c1-64) push_recursive=$(sha256sum "$PR" 2>/dev/null | cut -c1-64) integrate_ff_only=$(sha256sum "$FF" 2>/dev/null | cut -c1-64) integrate_merge=$(sha256sum "$IM" 2>/dev/null | cut -c1-64) verify_repos=$(sha256sum "$VR" 2>/dev/null | cut -c1-64) head=$(git rev-parse HEAD) host=$(hostname) git=$(git --version | cut -d' ' -f3) utc=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"

# ======================================================================================================================================
# I-3 push_recursive: a parent whose gitlink points at a submodule commit that no remote of the submodule holds is never pushed
# ======================================================================================================================================
RID=20261005T000000Z-1-aaaa; A="$T/audit"; mkdir -p "$A/$RID" "$T/b"
pn=0
# pfx: main repository m (remote r1) with ONE submodule `sub` whose remote is s.git; both published at the first commit
pfx() {
  pn=$((pn+1)); PX="$T/p$pn"; mkdir -p "$PX/b"; : > "$A/$RID/commits.tsv"
  git init -q --bare -b main "$PX/b/r1.git"; git init -q --bare -b main "$PX/b/s.git"
  git clone -q "$PX/b/s.git" "$PX/seed" 2>/dev/null; ( cd "$PX/seed" && git checkout -q -b main 2>/dev/null; echo s > s; git add s; $G commit -qm s; git push -q origin main )
  mkrepo "$PX/m"; ( cd "$PX/m" && echo base > base && git add base && $G commit -qm init && git remote add r1 "$PX/b/r1.git" && git push -q r1 main \
    && $G submodule add -q "$PX/b/s.git" sub 2>/dev/null && $G commit -qm addsub && git push -q r1 main )
  ( cd "$PX/m/sub" && git checkout -q main 2>/dev/null )
}
subc() { # subc <cpa|hand|held|pushed> : a commit in the submodule, then the main CPA commit that moves the gitlink to it
  local kind="$1" d="$PX/m/sub"
  echo "$kind $RANDOM" > "$d/x.txt"; git -C "$d" add x.txt
  case "$kind" in
    cpa|pushed) git -C "$d" commit -q -m "sub change" -m "CPA-Run: $RID"; printf 'sub\t%s\t%s\n' "$(git -C "$d" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv" ;;
    held) git -C "$d" commit -q -m "sub change" -m "CPA-Run: $RID" -m "Awaits-Review: reviews/none.json"; printf 'sub\t%s\t%s\n' "$(git -C "$d" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv" ;;
    hand) git -C "$d" commit -q -m "hand commit, no CPA trailer" ;;
  esac
  [ "$kind" != pushed ] || git -C "$d" push -q origin main
  SUBC="$(git -C "$d" rev-parse HEAD)"
  git -C "$PX/m" add sub; git -C "$PX/m" commit -q -m "bump sub" -m "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$PX/m" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"
}
mainr1() { git -C "$PX/b/r1.git" rev-parse main; }
prrun() { "$PR" --main-root "$PX/m" --branch main --run-dir "$A/$RID" "$@" >"$PX/out" 2>"$PX/err"; RC=$?; }
fresh_clone_ok() { rm -rf "$PX/fresh"; git -c protocol.file.allow=always clone -q --recurse-submodules "$PX/b/r1.git" "$PX/fresh" >"$PX/clone.out" 2>&1; echo $?; }

# a) an unrecorded hand commit in the submodule: the submodule is REFUSED, the main CPA commit that bumps the gitlink must NOT be pushed (the probe p7b)
pfx; M0="$(mainr1)"; subc hand; prrun --recursive
eq "I-3a unrecorded submodule commit: exit 20 (the submodule refusal stays the class)" "$RC" 20
has "I-3a the submodule is refused unrecorded_local_commit" "$(cat "$PX/out")" "REFUSED	sub	unrecorded_local_commit"
has "I-3a the parent is named withheld: submodule_commit_not_held" "$(cat "$PX/out")" "NOPUSH	.	r1	submodule_commit_not_held"
eq "I-3a main on r1 is UNCHANGED (nothing pushed for the parent)" "$(mainr1)" "$M0"
eq "I-3a the submodule remote does not hold the pinned commit (the cause)" "$(git -C "$PX/b/s.git" cat-file -e "$SUBC^{commit}" 2>/dev/null && echo yes || echo no)" no
# b) a HELD submodule commit (its review verdict is absent): the parent is withheld with exit 11 (precedence 11 over the hold's 14)
pfx; M0="$(mainr1)"; subc held; prrun --recursive
eq "I-3b held submodule commit: exit 11 (parent withheld)" "$RC" 11
has "I-3b the submodule commit is HELD" "$(cat "$PX/out")" "HELD	sub"
has "I-3b the parent is withheld submodule_commit_not_held" "$(cat "$PX/out")" "NOPUSH	.	r1	submodule_commit_not_held"
eq "I-3b main on r1 is UNCHANGED" "$(mainr1)" "$M0"
# c) golden-false: the submodule commit is a recorded CPA commit that this very run pushes first (deepest first): the parent is pushed
pfx; subc cpa; prrun --recursive
eq "I-3c golden-false: every submodule commit pushed in the run: exit 0" "$RC" 0
has "I-3c the submodule is pushed" "$(cat "$PX/out")" "PUSHED	sub	origin	$SUBC"
has "I-3c the parent is pushed" "$(cat "$PX/out")" "PUSHED	.	r1"
eq "I-3c a fresh clone --recurse-submodules of the published main works" "$(fresh_clone_ok)" 0
# d) golden-false: the gitlink moves to a commit the submodule remote ALREADY holds (pushed earlier): nothing for the submodule, the parent is pushed
pfx; subc pushed; prrun --recursive
eq "I-3d golden-false: the pinned commit already on the submodule remote: exit 0" "$RC" 0
has "I-3d the parent is pushed" "$(cat "$PX/out")" "PUSHED	.	r1"
eq "I-3d a fresh clone --recurse-submodules works" "$(fresh_clone_ok)" 0
# e) the submodule's remote cannot be read (moved away): its commit cannot be proven held, the parent is withheld, exit 11
pfx; subc cpa; M0="$(mainr1)"; mv "$PX/b/s.git" "$PX/b/s.gone"; prrun --recursive
eq "I-3e submodule remote unreachable: exit 11" "$RC" 11
has "I-3e the parent is withheld submodule_commit_not_held" "$(cat "$PX/out")" "NOPUSH	.	r1	submodule_commit_not_held"
eq "I-3e main on r1 is UNCHANGED" "$(mainr1)" "$M0"
mv "$PX/b/s.gone" "$PX/b/s.git"
# f) without --recursive only the parent is pushed, and its gitlink still must be held by the submodule's remote
pfx; subc cpa; M0="$(mainr1)"; prrun
eq "I-3f no --recursive, gitlink to a commit the submodule remote lacks: exit 11" "$RC" 11
eq "I-3f main on r1 is UNCHANGED" "$(mainr1)" "$M0"
git -C "$PX/m/sub" push -q origin main; prrun
eq "I-3f golden-false: once the submodule remote holds the commit the parent is pushed: exit 0" "$RC" 0
has "I-3f the parent is pushed" "$(cat "$PX/out")" "PUSHED	.	r1"
# g) a pin held by a remote TAG of the submodule only (not by a branch tip) is held: a fresh clone can fetch it
pfx; subc hand; ( cd "$PX/m/sub" && git tag keep && git push -q origin keep ); prrun --recursive
has "I-3g a pin held by a remote tag is held: the parent is not withheld for it" "$(cat "$PX/out")" "PUSHED	.	r1"

# h) golden-false: a gitlink that the remote's tip ALREADY carries is not this push's publication, whatever the submodule remote holds: an unrelated parent change is pushed
pfx; subc hand; git -C "$PX/m" push -q r1 main; PRE="$(mainr1)"
echo unrelated > "$PX/m/other.txt"; git -C "$PX/m" add other.txt; git -C "$PX/m" commit -q -m "unrelated change" -m "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$PX/m" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"
prrun
eq "I-3h golden-false: only a gitlink the remote tip already carries is unheld: the parent change is pushed, exit 0" "$RC" 0
has "I-3h the parent is pushed" "$(cat "$PX/out")" "PUSHED	.	r1"
# i) a pin that is an ANCESTOR of a remote branch tip is held (the remote moved on after the pin was pushed): equality alone is too strict
pfx; subc pushed; ( cd "$PX/seed" && git pull -q origin main 2>/dev/null; echo more > more && git add more && $G commit -qm D && git push -q origin main ); git -C "$PX/m/sub" fetch -q origin
prrun --recursive
eq "I-3i the submodule's own remote is ahead of its branch (the submodule row is remote_moved_since_s1): exit 11, and the parent is still pushed (the pin IS held)" "$RC" 11
has "I-3i the submodule row is remote_moved_since_s1, not submodule_commit_not_held" "$(cat "$PX/out")" "NOPUSH	sub	origin	remote_moved_since_s1"
has "I-3i the parent is pushed" "$(cat "$PX/out")" "PUSHED	.	r1"
# j) a submodule without any remote: nothing can be proven held, the parent is withheld (exit 11)
pfx; subc cpa; git -C "$PX/m/sub" remote remove origin; M0="$(mainr1)"; prrun --recursive
eq "I-3j a submodule with no remote: exit 20 (its whole history is outgoing and unrecorded: the submodule refusal class)" "$RC" 20
has "I-3j the reason names it" "$(cat "$PX/out")" "submodule has no remote"
eq "I-3j main on r1 is UNCHANGED" "$(mainr1)" "$M0"
# k) a gitlink moved in a parent whose submodule is NOT initialised cannot be proven held: withheld (exit 11), never published on a guess
pfx; M0="$(mainr1)"; rm -rf "$PX/m2"; git -c protocol.file.allow=always clone -q "$PX/b/r1.git" "$PX/m2" 2>/dev/null; git -C "$PX/m2" remote add r2 "$PX/b/r1.git" 2>/dev/null
git -C "$PX/m2" update-index --cacheinfo "160000,$(printf '%040d' 7),sub"; git -C "$PX/m2" config user.email t@t; git -C "$PX/m2" config user.name t
git -C "$PX/m2" commit -q -m "bump uninitialised sub" -m "CPA-Run: $RID"; printf '.\t%s\t%s\n' "$(git -C "$PX/m2" rev-parse HEAD)" "$RID" >> "$A/$RID/commits.tsv"
"$PR" --main-root "$PX/m2" --branch main --run-dir "$A/$RID" >"$PX/out" 2>"$PX/err"; RC=$?
eq "I-3k a changed gitlink whose submodule is not initialised: exit 11 (golden-false control below: unchanged one is fine)" "$RC" 11
has "I-3k the reason names it" "$(cat "$PX/out")" "submodule not initialised"
eq "I-3k main on r1 is UNCHANGED" "$(mainr1)" "$M0"

# ======================================================================================================================================
# M-3 integrate_ff_only: every `git remote` call site (counter shim: the k-th `git ... remote` fails)
# ======================================================================================================================================
fn=0
ffx() {
  fn=$((fn+1)); FP="$T/f$fn"; mkdir -p "$FP"
  git init -q --bare -b main "$FP/sm.git"; git init -q --bare -b main "$FP/o.git"
  git clone -q "$FP/sm.git" "$FP/smw" 2>/dev/null; ( cd "$FP/smw" && git checkout -q -b main 2>/dev/null; echo s1 > s; git add s; $G commit -qm s1; git push -q origin main )
  git clone -q "$FP/o.git" "$FP/w" 2>/dev/null
  ( cd "$FP/w" && git checkout -q -b main 2>/dev/null; printf '/.audit/\n' > .gitignore; echo a > f; $G submodule add -q "$FP/sm.git" sm 2>/dev/null; git add -A; $G commit -qm c1; git push -q origin main )
  FW="$FP/w"; FCS="$FP/cs.txt"; echo sm > "$FCS"; FOUT="$FP/out.json"
}
cnt_shim() { # cnt_shim <k|0>: a git shim counting `git ... remote` calls in $FP/n, failing the k-th (0: never)
  mkdir -p "$FP/shim"; : > "$FP/n"
  printf '#!/bin/sh\ncase "$*" in *" remote") n=$(cat "%s/n"); n=$((n+1)); echo $n > "%s/n"; if [ "$n" = "%s" ]; then echo "fatal: shim" >&2; exit 128; fi ;; esac\nexec "%s" "$@"\n' "$FP" "$FP" "$1" "$(command -v git)" > "$FP/shim/git"; chmod +x "$FP/shim/git"
}
ffrun() { PATH="$FP/shim:$PATH" "$FF" --root "$FW" --main-only --report-behind-submodules --changeset-from "$FCS" --owned-orgs "f$fn" --json "$FOUT" >"$FP/stdout" 2>"$FP/stderr"; RC=$?; }
ffx; cnt_shim 0; ffrun; eq "M-3 control: the fixture (owned submodule in the change set) runs clean with the counting shim: exit 0" "$RC" 0
NCALL="$(cat "$FP/n")"; case "$NCALL" in ''|*[!0-9]*) NCALL=0 ;; esac
[ "$NCALL" -ge 6 ] && ok "M-3 control: the run makes $NCALL \`git remote\` calls (every site is reached)" || bad "M-3 control: only $NCALL \`git remote\` calls, the sites are not all reached"
k=1; while [ "$k" -le "$NCALL" ]; do
  ffx; cnt_shim "$k"; ffrun
  eq "M-3 the failing git remote call #$k of $NCALL: exit 20" "$RC" 20
  eq "M-3 call #$k: reason git_listing_failed" "$(jq -r .reason "$FOUT" 2>/dev/null || echo '?')" git_listing_failed
  k=$((k+1))
done

# ======================================================================================================================================
# INFO-1 integrate_merge: every remote unreachable must not read clean
# ======================================================================================================================================
IMD="$T/im"; mkdir -p "$IMD/b" "$IMD/audit/$RID"; mkrepo "$IMD/m"; echo base > "$IMD/m/base"; commit_all "$IMD/m" init
for r in r1 r2; do git init -q --bare -b main "$IMD/b/$r.git"; git -C "$IMD/m" remote add $r "$IMD/b/$r.git"; git -C "$IMD/m" push -q $r main; done
imrun() { "$IM" --root "$IMD/m" --branch main --run-dir "$IMD/audit/$RID" >"$IMD/out" 2>"$IMD/err"; RC=$?; }
imrun; eq "INFO-1 golden-false: all remotes reachable and in step: nothing_to_merge, exit 0" "$RC" 0; has "INFO-1 control reports nothing_to_merge" "$(cat "$IMD/out")" nothing_to_merge
mv "$IMD/b/r1.git" "$IMD/b/r1.gone"; imrun
eq "INFO-1 golden-false: one remote reachable, one not, nothing diverged: exit 0 (a reachable remote answered)" "$RC" 0
mv "$IMD/b/r2.git" "$IMD/b/r2.gone"; imrun
eq "INFO-1 EVERY remote unreachable: exit 11, not a clean 0" "$RC" 11
has "INFO-1 reason remote_unreachable" "$(cat "$IMD/out") $(cat "$IMD/err")" remote_unreachable
hasnot "INFO-1 it is not reported as nothing_to_merge" "$(cat "$IMD/out")" nothing_to_merge
mv "$IMD/b/r1.gone" "$IMD/b/r1.git"; mv "$IMD/b/r2.gone" "$IMD/b/r2.git"
# every remote LISTS (ls-remote answers) but every object fetch fails (shim): the same, no remote could be read
mkdir -p "$IMD/shim"; printf '#!/bin/sh\ncase "$*" in *" fetch "*) echo "fatal: shim" >&2; exit 128 ;; esac\nexec "%s" "$@"\n' "$(command -v git)" > "$IMD/shim/git"; chmod +x "$IMD/shim/git"
rm -rf "$IMD/c1"; git clone -q "$IMD/b/r1.git" "$IMD/c1"; git -C "$IMD/c1" remote add r2 "$IMD/b/r2.git"
( cd "$IMD/c1" && echo r > r.txt && git add r.txt && git -c user.email=t@t -c user.name=t commit -qm remote-only && git push -q origin main && git push -q r2 main )
PATH="$IMD/shim:$PATH" imrun
eq "INFO-1 every remote lists a tip but every object fetch fails: exit 11, not a clean 0" "$RC" 11
imrun; eq "INFO-1 golden-false: the same remotes with a working fetch merge or fast-forward the remote tip (not 11)" "$([ "$RC" != 11 ] && echo notEleven || echo eleven)" notEleven
git -C "$IMD/m" remote remove r1; git -C "$IMD/m" remote remove r2; imrun
eq "INFO-1 golden-false: a repository with NO remote at all: nothing_to_merge, exit 0" "$RC" 0

# ======================================================================================================================================
# M-6 verify_repos: a stale --json FILE of an earlier run must not survive an exit-20 count failure
# ======================================================================================================================================
REALJQ="$(command -v jq)"; mkdir -p "$T/shim_jq"
cat > "$T/shim_jq/jq" <<SHIM
#!/usr/bin/env bash
for a in "\$@"; do case "\$a" in "\$JQFAIL") echo "jq: simulated failure" >&2; exit 5 ;; esac; done
exec "$REALJQ" "\$@"
SHIM
chmod +x "$T/shim_jq/jq"
F_DIRTY='[.repos[]|select(.problems|index("dirty"))]|length'
mkdir -p "$T/fx"; git init -q --bare -b main "$T/fx/r_v.git"; git clone -q "$T/fx/r_v.git" "$T/w_v" 2>/dev/null
( cd "$T/w_v" && git checkout -q -b main 2>/dev/null; echo a > f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null )
vrun() { "$VR" --root "$T/w_v" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 20 --json "$1" >"$T/v.out" 2>"$T/v.err"; RC=$?; }
printf '{"STALE":"report of an earlier run"}\n' > "$T/m6.json"
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" vrun "$T/m6.json"
eq "M-6 a count cannot be read: exit 20" "$RC" 20
[ -e "$T/m6.json" ] && bad "M-6 the stale --json FILE of the earlier run survived the exit-20 failure ($(head -c 60 "$T/m6.json"))" || ok "M-6 no --json FILE exists after the exit-20 failure (no report contradicts the exit status)"
printf '{"STALE":"old"}\n' > "$T/m6b.json"; vrun "$T/m6b.json"
eq "M-6 golden-false: a good run exits 0" "$RC" 0
eq "M-6 golden-false: a good run overwrites the old file with a real report" "$(jq -r 'has("repos")' "$T/m6b.json" 2>/dev/null || echo '?')" true
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" "$VR" --root "$T/w_v" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --json /dev/null >"$T/v.out" 2>"$T/v.err"; rc=$?
eq "M-6 a non-regular --json target (/dev/null) with an exit-20 failure: exit 20" "$rc" 20
[ -c /dev/null ] && ok "M-6 /dev/null is still a character device (the cleanup never removes a non-regular file)" || bad "M-6 /dev/null was removed"
ln -s "$T/no_such_report.json" "$T/m6s.link"; PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" vrun "$T/m6s.link"
[ -L "$T/m6s.link" ] && bad "M-6 a stale DANGLING --json symlink survived" || ok "M-6 a stale dangling --json symlink is removed"
printf '{"STALE":"x"}\n' > "$T/m6c.json"; ln -s "$T/m6c.json" "$T/m6c.link"
PATH="$T/shim_jq:$PATH" JQFAIL="$F_DIRTY" vrun "$T/m6c.link"
[ -e "$T/m6c.link" ] || [ -L "$T/m6c.link" ] && bad "M-6 a stale --json SYMLINK survived" || ok "M-6 a stale --json symlink is removed (the link, not its target)"
eq "M-6 the symlink target itself is left alone" "$([ -f "$T/m6c.json" ] && echo kept || echo gone)" kept
# a bad ARGUMENT before the run starts must not delete a file the operator named (no run, no stale report is possible)
printf '{"KEEP":"x"}\n' > "$T/m6d.json"; "$VR" --root "$T/w_v" --json "$T/m6d.json" --jobs zero >"$T/v.out" 2>"$T/v.err"; rc=$?
eq "M-6 a bad argument: exit 20" "$rc" 20; [ -f "$T/m6d.json" ] && ok "M-6 a usage error does not delete the named file" || bad "M-6 a usage error deleted the named file"
printf '{"KEEP":"y"}\n' > "$T/m6e.json"; "$VR" --root "$T/w_v" --json "$T/m6e.json" --bogus-option >"$T/v.out" 2>"$T/v.err"; rc=$?
eq "M-6 an unknown option: exit 20" "$rc" 20; [ -f "$T/m6e.json" ] && ok "M-6 an unknown option does not delete the named file" || bad "M-6 an unknown option deleted the named file"
echo "SUMMARY pass=$PASSN fail=$FAILN"
fin
