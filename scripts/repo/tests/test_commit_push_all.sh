#!/usr/bin/env bash
# T039 / T042a / T043 test (TDD): scripts/commit-push-all.sh (CPA) and its host entry point scripts/repo/host_entry/cpa-host.
# Exit-code matrix on THROWAWAY clones with LOCAL BARE remotes only (docs/16 section 12; tasks.md T039, CENTRAL C3): every case builds a fresh
# clone, two fresh bare remotes under $T/<case>/fx/ and its own test trust state ($CPA_HOST_STATE under $T). No real remote, no hosted URL,
# no network, no credential; the owner's real $CPA_HOST_STATE and $CPA_HOST_ENTRY are never read or written.
# A `git` shim on PATH logs every call; the test fails when a call carries --force, --force-with-lease, a +refspec, --no-verify, rebase or reset.
# Usage: bash scripts/repo/tests/test_commit_push_all.sh   Env: H=<script under test>, CPA_ONLY=<substring of a section name> to run one section
# Output: `ok`/`FAIL` lines and a summary; exit 0 only when no case failed AND the script under test exists.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; SRC="${CPA_SRC:-$D0}"   # SRC: where the scripts under test are read from (a mutated copy under the mutation runner)
H="${H:-$SRC/scripts/commit-push-all.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
REALGIT="$(command -v git)"; T="$(mktemp -d "${TMPDIR:-/tmp}/cpa_test.XXXXXX")"; [ -n "${CPA_KEEP:-}" ] || trap 'rm -rf "$T"' EXIT; [ -z "${CPA_KEEP:-}" ] || echo "NOTE: CPA_KEEP set, scratch tree kept: $T"
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
EVR=specs/001-full-project-audit-remediation/evidence; SCHEMA=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
export LONGOPS_ALLOW_TMPFS=1 CPA_OWNED_ORGS=fx

# ---- the git shim: logs argv; hooks for pauses, a one-shot command before the first push, and failures (test hooks only) -----------------
mkdir -p "$T/shim"; cat > "$T/shim/git" <<SHIM
#!/bin/bash
real="$REALGIT"
printf '%s\n' "\$*" >> "\${GITSHIM_LOG:-/dev/null}"
args="\$*"
pushed=0; [ -f "\${GITSHIM_LOG:-/nonexistent}" ] && grep -qE '(^| )push( |\$)' "\$GITSHIM_LOG" && pushed=1
if [ -n "\${GITSHIM_FAIL_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_FAIL_ON ]]; then echo "shim: refused \$args" >&2; exit 128; fi
if [ -n "\${GITSHIM_FAIL_AFTER_PUSH:-}" ] && [ "\$pushed" = 1 ] && [[ "\$args" =~ \$GITSHIM_FAIL_AFTER_PUSH ]]; then echo "shim: refused after push \$args" >&2; exit 128; fi
if [ -n "\${GITSHIM_HOOK_AFTER_COMMIT:-}" ] && [[ "\$args" =~ ls-remote ]] && grep -qE '(^| )commit( |\$)' "\$GITSHIM_LOG" 2>/dev/null && [ ! -e "\$GITSHIM_HOOK_AFTER_COMMIT.done" ]; then : > "\$GITSHIM_HOOK_AFTER_COMMIT.done"; "\$GITSHIM_HOOK_AFTER_COMMIT" >/dev/null 2>&1; fi
if [ -n "\${GITSHIM_PAUSE_AFTER_PUSH:-}" ] && [ "\$pushed" = 1 ] && [[ "\$args" =~ \$GITSHIM_PAUSE_AFTER_PUSH ]] && [ ! -e "\$GITSHIM_PAUSED" ]; then
  : > "\$GITSHIM_PAUSED"; n=0; while [ ! -e "\$GITSHIM_PAUSE_UNTIL" ] && [ \$n -lt 400 ]; do sleep 0.2; n=\$((n+1)); done
fi
if [ -n "\${GITSHIM_PAUSE_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_PAUSE_ON ]] && [ ! -e "\$GITSHIM_PAUSED" ]; then
  : > "\$GITSHIM_PAUSED"; n=0; while [ ! -e "\$GITSHIM_PAUSE_UNTIL" ] && [ \$n -lt 400 ]; do sleep 0.2; n=\$((n+1)); done
fi
exec "\$real" "\$@"
SHIM
chmod 755 "$T/shim/git"

# ---- the seed repository (what a CPA clone holds: the scripts, the tables, the schema, a submodule) -------------------------------------
mkdir -p "$T/fx"; git init -q --bare -b main "$T/fx/sub.git"
S="$T/seed"; mkrepo "$S"
mkdir -p "$S/scripts/repo/host_entry" "$S/scripts/audit" "$S/scripts/longops" "$S/scripts/anti-mess" "$S/$(dirname "$SCHEMA")" "$S/$EVR/reviews" "$S/src"
for f in "$SRC"/scripts/repo/*.sh "$SRC"/scripts/repo/*.tsv "$SRC"/scripts/repo/fixture_roots.txt; do cp "$f" "$S/scripts/repo/"; done
cp "$SRC"/scripts/audit/org_of.py "$S/scripts/audit/"; cp "$SRC"/scripts/longops/*.sh "$S/scripts/longops/"
cp "$SRC/scripts/commit-push-all.sh" "$S/scripts/" 2>/dev/null; cp "$SRC/scripts/repo/host_entry/cpa-host" "$S/scripts/repo/host_entry/" 2>/dev/null
printf 'fx\n' > "$S/scripts/audit/own_orgs.txt"
cp "$D0/$SCHEMA" "$S/$SCHEMA"
# a reduced check registry (the real one has container rows): three file checks and the changeset check, all on the host
{ printf '# check\tcommand\timage\tmode\tbaseline\tscope\tnote\n'
  printf 'shell_parse\tbuiltin:shell_parse\tIMG-TESTUTIL\tplain\t-\tfiles\tbash -n\n'
  printf 'merge_conflict\tbuiltin:merge_conflict\tIMG-TESTUTIL\tplain\t-\tfiles\tmarkers\n'
  printf 'no_ci\tscripts/repo/check_no_ci.sh --root {root}\tIMG-TESTUTIL\tplain\t-\tchangeset\tno pipeline\n'; } > "$S/scripts/repo/validate_checks.tsv"
printf '/.audit/\n' > "$S/.gitignore"
printf 'L1\nL2\nL3\nL4\nL5\n' > "$S/src/conf.txt"; printf 'M1\nM2\nM3\nM4\nM5\nM6\nM7\nM8\n' > "$S/src/z.txt"; echo base > "$S/README.txt"
cat > "$S/scripts/anti-mess/sweep.sh" <<'SW'
#!/usr/bin/env bash
# sweep shim of the test harness: logs its argv, exits SWEEP_RC (default 0)
printf '%s\n' "$*" >> "${SWEEP_LOG:-/dev/null}"; exit "${SWEEP_RC:-0}"
SW
chmod 755 "$S/scripts/anti-mess/sweep.sh"
commit_all "$S" seed
# the submodule: a bare repository under fx/ (org `fx` is owned), seeded with one commit
rm -rf "$T/subseed"; mkrepo "$T/subseed"; echo sub > "$T/subseed/s.txt"; commit_all "$T/subseed" "sub init"; git -C "$T/subseed" push -q "$T/fx/sub.git" main
git -C "$S" submodule add -q "$T/fx/sub.git" mods/sub 2>/dev/null
# a third-party submodule (organisation `ext` is not owned): a --repo run of it is refused, no run ever pushes it
mkdir -p "$T/ext"; git init -q --bare -b main "$T/ext/t.git"; rm -rf "$T/extseed"; mkrepo "$T/extseed"; echo t > "$T/extseed/t.txt"; commit_all "$T/extseed" "ext init"; git -C "$T/extseed" push -q "$T/ext/t.git" main
git -C "$S" submodule add -q "$T/ext/t.git" mods/ext 2>/dev/null; git -C "$S" commit -qm "add submodules" 2>/dev/null
git init -q --bare -b main "$T/seed.git" && git -C "$S" push -q "$T/seed.git" main

# ---- per-case construction ---------------------------------------------------------------------------------------------------------------
sha() { sha256sum "$1" | cut -d' ' -f1; }
mk() { # mk <case> [PRE=function applied to the clone before the trust state is built] [OMIT=<tracked path left out of the approved manifest>]
  local n="$1"; C="$T/$n"; rm -rf "$C"; mkdir -p "$C/fx" "$C/state/blobs" "$C/bin"; chmod 700 "$C/state"
  rm -rf "$T/fx/sub.git"; git clone -q --bare "$T/subseed" "$T/fx/sub.git" 2>/dev/null
  git clone -q --bare "$T/seed.git" "$C/fx/a.git"; git clone -q --bare "$T/seed.git" "$C/fx/b.git"
  git clone -q "$C/fx/a.git" "$C/w" 2>/dev/null; W="$C/w"
  git -C "$W" config user.email t@t; git -C "$W" config user.name t; git -C "$W" remote add mirror "$C/fx/b.git"; git -C "$W" fetch -q mirror
  git -C "$W" -c protocol.file.allow=always submodule update -q --init 2>/dev/null
  git -C "$W/mods/sub" checkout -q -B main 2>/dev/null; git -C "$W/mods/sub" config user.email t@t; git -C "$W/mods/sub" config user.name t
  if [ -n "${PRE:-}" ]; then "$PRE"; commit_all "$W" "case setup"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; fi
  trust_build "$C"; cp "$W/scripts/repo/host_entry/cpa-host" "$C/bin/cpa-host" 2>/dev/null; chmod 755 "$C/bin/cpa-host" 2>/dev/null
  : > "$C/git.log"; : > "$C/sweep.log"; PREV=""; SEEDHEAD="$(git -C "$W" rev-parse HEAD)"
}
moved() { [ "$(git -C "$W" rev-parse HEAD)" != "$SEEDHEAD" ] && ok "$1: HEAD moved (the commit was made)" || bad "$1: HEAD did not move"; }
trust_build() { # a test-state trust file built directly (the owner `approve` operation is T046a, not built here): manifest = tracked scripts/ files
  local C="$1" w="$1/w" key entries f s x ms
  key="$(git -C "$w" rev-list --first-parent --max-parents=0 HEAD | head -1)"; entries='[]'
  while IFS= read -r f; do
    case "$f" in scripts/*) ;; *) continue ;; esac
    case "$f" in scripts/repo/tests/*|scripts/longops/tests/*|scripts/anti-mess/tests/*) continue ;; esac
    [ -n "${OMIT:-}" ] && [ "$f" = "$OMIT" ] && continue
    s="$(sha "$w/$f")"; x=false; [ -x "$w/$f" ] && x=true
    [ -e "$C/state/blobs/$s" ] || { cp "$w/$f" "$C/state/blobs/$s"; chmod 0400 "$C/state/blobs/$s"; }
    entries="$(jq -c --arg p "$f" --arg s "$s" --argjson x "$x" '. + [{path:$p,sha256:$s,exec:$x}]' <<< "$entries")"
  done < <(git -C "$w" ls-files)
  entries="$(jq -cS 'sort_by(.path)' <<< "$entries")"; ms="$(printf '%s' "$entries" | sha256sum | cut -d' ' -f1)"
  jq -n --arg k "$key" --arg a "$(git -C "$w" rev-parse HEAD)" --argjson e "$entries" --arg ms "$ms" \
    '{schema:"cpa-host-trust/1",projects:{($k):{adoption_commit:$a,manifest:$e,history:[{op:"approve",verdict:"fixture",time:"2026-10-06T00:00:00Z",manifest:$e,manifest_sha256:$ms}]}}}' > "$C/state/trust.json"
  TRUSTKEY="$key"; TRUSTMS="$ms"
}
ENVV() { echo "CPA_HOST_STATE=$C/state" "CPA_HOST_ENTRY=$C/bin/cpa-host" "PATH=$T/shim:$PATH" "GITSHIM_LOG=$C/git.log" "SWEEP_LOG=$C/sweep.log" "CPA_OWNED_ORGS=fx"; }
run() { # run <cpa-host args...>   (cwd $W or $CWD); sets RC, RUN (run id), RD (run dir), REP (report path), OUT (stdout+stderr)
  : > "$C/git.log"; ( cd "${CWD:-$W}" && env $(ENVV) ${XENV:-} "$C/bin/cpa-host" "$@" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
  OUT="$(cat "$C/out.txt" "$C/err.txt")"; RUN="$(printf '%s\n' "$OUT" | sed -n 's/.*cpa: exit=[0-9]* run=\([^ ]*\).*/\1/p' | tail -1)"
  RD="$W/.audit/commit-push/$RUN"; REP="$RD/report.json"; shimcheck
}
shimcheck() { # no force, no force-with-lease, no +refspec, no --no-verify, no rebase, no reset in any git call of the run
  local l; l="$(grep -nE -- '--force|--force-with-lease|--no-verify|(^| )rebase( |$)|(^| )reset( |$)|(^| )\+[A-Za-z0-9_/.:-]+:|(^| )push .*( |:)\+' "$C/git.log" | head -3)"
  [ -z "$l" ] || bad "SHIM: a forbidden git call: $l"
}
pushes() { grep -cE '(^| )push( |$)' "$C/git.log"; }
tipof() { git -C "$C/fx/$1.git" rev-parse main 2>/dev/null || echo none; }
head_() { git -C "${1:-$W}" rev-parse HEAD; }
msg() { git -C "$W" log -1 --format=%B "${1:-HEAD}"; }
foreign() { # foreign <path> <content> [ab|a|b]: another clone commits a file change and pushes it to the named remotes
  local rem="${3:-ab}"; rm -rf "$C/f"; git clone -q "$C/fx/${rem:0:1}.git" "$C/f" 2>/dev/null; git -C "$C/f" config user.email f@f; git -C "$C/f" config user.name f
  mkdir -p "$C/f/$(dirname "$1")"; printf '%b' "$2" > "$C/f/$1"; git -C "$C/f" add -A; git -C "$C/f" commit -qm "foreign change $1"
  case "$rem" in *a*) git -C "$C/f" push -q "$C/fx/a.git" main ;; esac; case "$rem" in *b*) git -C "$C/f" push -q "$C/fx/b.git" main ;; esac
  FOREIGN="$(git -C "$C/f" rev-parse HEAD)"
}
paths() { printf '%b' "$1" > "$C/p.txt"; }
want_report() { [ -f "$REP" ] && ok "$1: report.json exists" || bad "$1: report.json missing at $REP"; }
GOJ() { printf '{"schema":"review-verdict/1","verdict":"%s","covers_runs":%s,"model":"opus","effort":"xhigh","blocking_findings":%s}\n' "$1" "$(printf '%s' "$2" | jq -c 'map({repository:".",commit:"0000000000000000000000000000000000000000",cpa_run:.})')" "$3"; }
sect() { [ -z "${CPA_ONLY:-}" ] && return 0; case "$1" in *"$CPA_ONLY"*) return 0 ;; esac; return 1; }
hshim() { # hshim <scripts/repo/x.sh> <shell body>: replaces a helper in the clone by a shim, pushes it (so no run refuses an unrecorded commit) and rebuilds the trust state
  printf '#!/bin/bash\n%s\n' "$2" > "$W/$1"; chmod 755 "$W/$1"; git -C "$W" add -A; git -C "$W" commit -qm "helper shim $1"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
}
tracked_clean() { [ -z "$(git -C "$W" status --porcelain --ignore-submodules=all)" ]; }

# ======== 1 clean run, deferral, ordinary failures ========================================================================================
if sect "1 exit codes"; then
mk clean
echo hello > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "add a"
eq "1.1 clean run: 0" "$RC" 0; want_report "1.1"
moved 1.1; eq "1.1 origin holds HEAD" "$(tipof a)" "$(head_)"; eq "1.1 mirror holds HEAD" "$(tipof b)" "$(head_)"
has "1.1 the commit carries the CPA-Run trailer of the run" "$(msg)" "CPA-Run: $RUN"; hasnot "1.1 no Deferred-Gates line" "$(msg)" "Deferred-Gates"
tracked_clean && ok "1.1 tracked tree clean after exit 0" || bad "1.1 tracked tree dirty: $(git -C "$W" status --porcelain)"
eq "1.1 report exit" "$(jq -r .exit "$REP" 2>/dev/null)" 0
[ -s "$C/sweep.log" ] && ok "1.1 the sweep shim was called" || bad "1.1 the sweep shim was never called"
h1="$(head_)"; run; eq "1.2 second run after a clean run: 0" "$RC" 0; eq "1.2 commits nothing" "$(head_)" "$h1"; tracked_clean && ok "1.2 clean" || bad "1.2 dirty"
RUN1="$RUN"; run; [ "$RUN" != "$RUN1" ] && ok "1.2 run ids differ" || bad "1.2 equal run ids"
case "$RUN" in [0-9]*T[0-9]*Z-[0-9]*-*) ok "1.2 run id holds UTC time, pid and a suffix" ;; *) bad "1.2 run id shape: $RUN" ;; esac

OMIT=scripts/anti-mess/sweep.sh mk nosweep
echo n > "$W/src/n.txt"; paths 'src/n.txt\n'; run --paths-from "$C/p.txt" "add n"
moved 1.3; eq "1.3 no sweep: 14" "$RC" 14; has "1.3 SWEEP_ABSENT in the commit's Deferred-Gates line" "$(msg)" "Deferred-Gates: SWEEP_ABSENT"; eq "1.3 still pushed to origin" "$(tipof a)" "$(head_)"

mk badcheck
printf 'if then\n' > "$W/src/bad.sh"; paths 'src/bad.sh\n'; h0="$(head_)"; run --paths-from "$C/p.txt" "bad"
eq "1.4 failing cheap check: 10" "$RC" 10; eq "1.4 nothing committed" "$(head_)" "$h0"; eq "1.4 no push call" "$(pushes)" 0
want_report "1.4"; eq "1.4 report names stage S3" "$(jq -r .failed_stage "$REP" 2>/dev/null)" S3
eq "1.4 lock released" "$(ls "$W/.audit/longops/claims" 2>/dev/null | grep -c commit_push)" 0
jq -e '.files|type=="object" and length>0' "$REP" >/dev/null 2>&1 && ok "1.4 report holds the sha256 of the run's files" || bad "1.4 files map missing"

mk reject
printf '#!/bin/sh\necho rejected >&2\nexit 1\n' > "$C/fx/a.git/hooks/pre-receive"; chmod 755 "$C/fx/a.git/hooks/pre-receive"
echo r > "$W/src/r.txt"; paths 'src/r.txt\n'; run --paths-from "$C/p.txt" "r"
eq "1.5 rejected push: 11" "$RC" 11; eq "1.5 the commit is local" "$(git -C "$W" log -1 --format=%s)" "r"; [ "$(tipof a)" != "$(head_)" ] && ok "1.5 origin lacks it" || bad "1.5 origin has it"
want_report "1.5"; eq "1.5 failed stage S6" "$(jq -r .failed_stage "$REP" 2>/dev/null)" S6

mk unreach
git -C "$W" remote set-url origin "$C/fx/nonexistent.git"; echo u > "$W/src/u.txt"; paths 'src/u.txt\n'; run --paths-from "$C/p.txt" "u"
eq "1.6 one remote unreachable: 11" "$RC" 11
mk allunreach
git -C "$W" remote set-url origin "$C/fx/n1.git"; git -C "$W" remote set-url mirror "$C/fx/n2.git"; run
eq "1.7 every remote unreachable: 20" "$RC" 20; has "1.7 reason no_remote_reachable" "$OUT" no_remote_reachable
run --local-only; eq "1.7 also under --local-only: 20" "$RC" 20; has "1.7 reason" "$OUT" no_remote_reachable
fi

# ======== 2 usage, preflight refusals, lock, report on every path =========================================================================
if sect "2 usage and S0"; then
mk usage
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; h0="$(head_)"
run --paths-from "$C/p.txt" "msg" --local-only; eq "2.1 argument after the message: 20" "$RC" 20; has "2.1 usage_error" "$OUT" usage_error; eq "2.1 no push call" "$(pushes)" 0; eq "2.1 nothing committed" "$(head_)" "$h0"
run --bogus; eq "2.2 unknown option: 20" "$RC" 20; has "2.2 usage_error" "$OUT" usage_error
run --paths-from "$C/p.txt"; eq "2.3 --paths-from without a message: 20" "$RC" 20
run --commit-before-integrate; eq "2.4 --commit-before-integrate without --paths-from: 20" "$RC" 20; has "2.4 usage_error" "$OUT" usage_error
run --commit-before-integrate --resolve-merge "$W/.audit/merge-resolution/x" --paths-from "$C/p.txt" "m"; eq "2.5 with --resolve-merge: 20" "$RC" 20
run --paths-from "$C/nonexistent.txt" "m"; eq "2.6 unreadable --paths-from: 20" "$RC" 20
printf '../escape\n' > "$C/p2.txt"; run --paths-from "$C/p2.txt" "m"; eq "2.7 unsafe declared path: 20" "$RC" 20; eq "2.7 nothing committed" "$(head_)" "$h0"
printf 'mods/sub/x.txt\n' > "$C/p3.txt"; echo x > "$W/mods/sub/x.txt"; run --paths-from "$C/p3.txt" "m"
eq "2.8 declared path below a gitlink: 20" "$RC" 20; has "2.8 path_in_submodule" "$OUT" path_in_submodule; eq "2.8 nothing committed in the main repository" "$(head_)" "$h0"
eq "2.8 nothing committed in the submodule" "$(git -C "$W/mods/sub" log --oneline | wc -l)" 1; rm -f "$W/mods/sub/x.txt"

mk mergeprog
echo a > "$W/src/a.txt"; head_ > "$W/.git/MERGE_HEAD"; run; eq "2.9 MERGE_HEAD present: 20 merge_in_progress" "$RC" 20; has "2.9 reason" "$OUT" merge_in_progress

mk hand
echo hand > "$W/src/h.txt"; git -C "$W" add src/h.txt; git -C "$W" commit -qm "hand commit"; run
eq "2.10 a plain git commit that no remote holds: 20" "$RC" 20; has "2.10 unrecorded_local_commit" "$OUT" unrecorded_local_commit; eq "2.10 no push call" "$(pushes)" 0
mk handtrailer
echo hand > "$W/src/h.txt"; git -C "$W" add src/h.txt; git -C "$W" commit -q -m "copy" -m "CPA-Run: 20261005T000000Z-1-aaaa"; run
eq "2.11 a copied CPA-Run trailer proves nothing: 20" "$RC" 20; has "2.11 unrecorded_local_commit" "$OUT" unrecorded_local_commit

mk regsh
mkdir -p "$W/scripts/longops"; printf '#!/bin/bash\necho "blocked\top1\tbuild:x\tsrc/a.txt\twrites a tracked path"; exit 1\n' > "$W/scripts/longops/check_no_build_writing_tracked.sh"; chmod 755 "$W/scripts/longops/check_no_build_writing_tracked.sh"
git -C "$W" add -A; git -C "$W" commit -qm "shim"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"; eq "2.12 a live writer in the registry: 20" "$RC" 20; has "2.12 evidence_writer_active" "$OUT" evidence_writer_active; eq "2.12 nothing pushed" "$(pushes)" 0

mk sweepfind
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; SWEEP_RC=1 run --paths-from "$C/p.txt" "a"
eq "2.13 a sweep finding (undeclared change): 20" "$RC" 20
mk sweepfind2; echo stray > "$W/src/stray.txt"; echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; export SWEEP_RC=1; run --paths-from "$C/p.txt" "a"; unset SWEEP_RC
eq "2.14 with the sweep present an undeclared file is refused at S0: 20" "$RC" 20; eq "2.14 the sweep was asked to exclude the declared change set" "$(grep -c -- '--paths-from' "$C/sweep.log")" 1

OMIT=scripts/anti-mess/sweep.sh mk strayns
echo stray > "$W/src/stray.txt"; echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"
eq "2.15 without the sweep an undeclared untracked file gives 13 at S7" "$RC" 13

mk interrupted
mkdir -p "$W/.audit/commit-push/20260101T000000Z-1-dead"; echo partial > "$W/.audit/commit-push/20260101T000000Z-1-dead/log.txt"; b="$(cd "$W/.audit/commit-push/20260101T000000Z-1-dead" && find . -type f | sort | xargs sha256sum | sha256sum)"
run; eq "2.16 clean run beside a run directory without a report: 0" "$RC" 0
has "2.16 the report lists the interrupted run" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "20260101T000000Z-1-dead"
eq "2.16 the interrupted run's directory is byte-unchanged" "$(cd "$W/.audit/commit-push/20260101T000000Z-1-dead" && find . -type f | sort | xargs sha256sum | sha256sum)" "$b"
fi

# ======== 3 the lock =========================================================================================================================
if sect "3 lock"; then
mk lock
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; export GITSHIM_LOG="$C/git.log" GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"
: > "$C/git.log"; ( cd "$W" && env $(ENVV) GITSHIM_PAUSE_AFTER_PUSH='ls-remote|fetch' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go" "$C/bin/cpa-host" --paths-from "$C/p.txt" "l" ) >"$C/hold.out" 2>"$C/hold.err" &
HP=$!; n=0; while [ ! -e "$C/paused" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done
[ -e "$C/paused" ] && ok "3.1 the first run is paused between S6 and S7" || bad "3.1 no pause reached"
HRUND="$(ls -1d "$W/.audit/commit-push/"*/ | head -1)"
hb="$(cd "$HRUND" && find . -type f | sort | xargs sha256sum | sha256sum)"; [ -n "$HRUND" ] && ok "3.1 the holder's run directory was found" || bad "3.1 no holder run directory"
run; eq "3.2 a second run while the lock is held: 20" "$RC" 20; has "3.2 reason lock_held" "$OUT" lock_held; has "3.2 names the holder's run id" "$OUT" "$(basename "$HRUND")"
eq "3.2 the holder's run directory is byte-unchanged" "$(cd "$HRUND" && find . -type f | sort | xargs sha256sum | sha256sum)" "$hb"
eq "3.2 the second run wrote its own run directory" "$(ls -1d "$W/.audit/commit-push/"*/ | wc -l)" 2
: > "$C/go"; wait $HP; hrc=$?; eq "3.3 the holder finishes with its own code: 0" "$hrc" 0
unset GITSHIM_LOG GITSHIM_PAUSED GITSHIM_PAUSE_UNTIL
run; eq "3.4 after the holder ended the lock is free: 0" "$RC" 0
fi

# ======== 4 deferrals and local-only =====================================================================================================
if sect "4 deferral"; then
mk skip
echo s > "$W/src/s.txt"; paths 'src/s.txt\n'; export SKIP_LONG="long gates not run"; run --paths-from "$C/p.txt" "s"; unset SKIP_LONG
moved 4.1; eq "4.1 SKIP_LONG: 14" "$RC" 14; has "4.1 Deferred-Gates SKIP_LONG" "$(msg)" "Deferred-Gates: SKIP_LONG"; eq "4.1 the commit is pushed" "$(tipof a)" "$(head_)"
mk localonly
echo lo > "$W/src/lo.txt"; paths 'src/lo.txt\n'; run --local-only --paths-from "$C/p.txt" "lo"
eq "4.2 --local-only: 14" "$RC" 14; has "4.2 LOCAL_ONLY in the Deferred-Gates line" "$(msg)" "LOCAL_ONLY"; eq "4.2 no push call" "$(pushes)" 0
[ "$(tipof a)" != "$(head_)" ] && ok "4.2 origin lacks the commit" || bad "4.2 origin has it"
eq "4.2 S7 ran without remote contact: report exit" "$(jq -r .exit "$REP" 2>/dev/null)" 14
# SKIP_LONG with a diverged repository: the merge commit carries SKIP_LONG
foreign src/f.txt 'f\n'
export SKIP_LONG="x"; run; unset SKIP_LONG
eq "4.3 SKIP_LONG, integrate-only run, diverged repository: 14" "$RC" 14; has "4.3 the merge commit's Deferred-Gates carries SKIP_LONG" "$(msg)" "SKIP_LONG"
eq "4.3 HEAD is a merge commit" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w)" 3
# a --local-only commit stays a CPA commit: the next run neither refuses nor names it, and pushes it
run; eq "4.6 the next run pushes the earlier --local-only commit: 0" "$RC" 0; eq "4.6 origin holds HEAD" "$(tipof a)" "$(head_)"
# S4 long gates: the verdict files named in the approved scripts/repo/long_gates.txt must exist and say PASS (11.4.135: absence blocks as a FAIL does)
mk longg
printf '%s\n' "$EVR/lg.json" > "$W/scripts/repo/long_gates.txt"; git -C "$W" add -A; git -C "$W" commit -qm "long gates"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; h0="$(head_)"; run --paths-from "$C/p.txt" "a"
eq "4.7 a missing long-gate verdict: 10" "$RC" 10; has "4.7 long_gate_verdict_missing" "$OUT" long_gate_verdict_missing; eq "4.7 nothing committed" "$(head_)" "$h0"
export SKIP_LONG="not now"; run --paths-from "$C/p.txt" "a"; unset SKIP_LONG; eq "4.7 the same window under SKIP_LONG: S4 skipped, 14" "$RC" 14
mk longg2
printf '%s\n' "$EVR/lg.json" > "$W/scripts/repo/long_gates.txt"; git -C "$W" add -A; git -C "$W" commit -qm "long gates"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
mkdir -p "$W/$EVR"; printf '{"verdict":"PASS","fingerprint":"x","utc":"2026-10-06T00:00:00Z"}\n' > "$W/$EVR/lg.json"; paths "$EVR/lg.json\n"; run --paths-from "$C/p.txt" "long gate verdict"
eq "4.8 a PASS long-gate verdict: 0" "$RC" 0
# a registry row that the approved copy lacks is never run: check_pending_release, a deferral of class 14, never 0
mk pendchk
printf 'extra\tscripts/repo/check_no_ci.sh --root /nonexistent\tIMG-TESTUTIL\tplain\t-\tchangeset\tnot in the approved copy\n' >> "$W/scripts/repo/validate_checks.tsv"
git -C "$W" add -A; git -C "$W" commit -qm "registry row"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"
eq "4.9 a registry row that the approved copy lacks is pending, never run: 14, not 10" "$RC" 14; eq "4.9 the report says check_pending_release" "$(jq -r .check_pending_release "$REP" 2>/dev/null)" true
mk skipfail
mkdir -p "$W/scripts/repo"; printf '#!/bin/bash\nexit 1\n' > "$W/scripts/repo/record_deferral.sh"; chmod 755 "$W/scripts/repo/record_deferral.sh"; git -C "$W" add -A; git -C "$W" commit -qm shim; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; export SKIP_LONG="x"; run --paths-from "$C/p.txt" "a"; unset SKIP_LONG
eq "4.4 a record_deferral.sh that fails to write its row: 20, never a 14 without its row" "$RC" 20; has "4.4 deferral_write_failed" "$OUT" deferral_write_failed
mk badhelper
printf '#!/bin/bash\nexit 2\n' > "$W/scripts/repo/commit_recursive.sh"; chmod 755 "$W/scripts/repo/commit_recursive.sh"; git -C "$W" add -A; git -C "$W" commit -qm shim; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"; eq "4.5 a stage helper exiting outside its set (commit_recursive 2): 20, never 10" "$RC" 20; has "4.5 commit_failed" "$OUT" commit_failed
fi

# ======== 5 S2 refusals (13) ===============================================================================================================
if sect "5 scope"; then
mk scope
echo SECRET=x > "$W/.env"; paths '.env\n'; run --paths-from "$C/p.txt" "env"; eq "5.1 a .env file: 13" "$RC" 13; eq "5.1 no push" "$(pushes)" 0; rm -f "$W/.env"
want_report "5.1"; eq "5.1 failed stage S2" "$(jq -r .failed_stage "$REP" 2>/dev/null)" S2
mk appendonly
mkdir -p "$W/$EVR"; printf 'l1\nl2\n' > "$W/$EVR/ledger.jsonl"; git -C "$W" add -A; git -C "$W" commit -qm ledger; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
printf 'l1\nCHANGED\n' > "$W/$EVR/ledger.jsonl"; paths "$EVR/ledger.jsonl\n"; run --paths-from "$C/p.txt" "edit ledger"; eq "5.2 an edited earlier ledger line: 13" "$RC" 13
git -C "$W" checkout -q -- "$EVR/ledger.jsonl"; printf 'l1\nl2\nl3\n' > "$W/$EVR/ledger.jsonl"; run --paths-from "$C/p.txt" "append ledger"; eq "5.3 a pure append passes: 0" "$RC" 0
echo x > "$W/$EVR/blobs_x"; mkdir -p "$W/$EVR/blobs"; echo notahash > "$W/$EVR/blobs/0000"; paths "$EVR/blobs/0000\n"; run --paths-from "$C/p.txt" "blob"; eq "5.4 a blob whose name is not the sha256 of its content: 13" "$RC" 13
mk dirtysub
echo dirty > "$W/mods/sub/untracked.txt"; run; eq "5.5 a dirty submodule: 13 at S2 (declared or not)" "$RC" 13
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"; eq "5.6 a dirty submodule with a declared change set: 13" "$RC" 13; eq "5.6 nothing pushed" "$(pushes)" 0
fi

# ======== 6 held commits ====================================================================================================================
if sect "6 held"; then
mk held
V="$EVR/reviews/WP-X.json"
echo u > "$W/src/u.txt"; echo h > "$W/src/h.txt"; paths "src/u.txt\nsrc/h.txt\t$V\n"; run --paths-from "$C/p.txt" "unheld and held"
eq "6.1 a held path: 14" "$RC" 14; has "6.1 review_pending" "$OUT" review_pending
hh="$(head_)"; eq "6.1 two commits: unheld first, held second" "$(git -C "$W" log --format=%s -2 | wc -l)" 2
has "6.1 the held commit awaits the verdict" "$(msg)" "Awaits-Review: $V"; has "6.1 the held commit carries LOCAL_ONLY" "$(msg)" "Deferred-Gates: LOCAL_ONLY"
hasnot "6.1 the unheld commit is not held" "$(msg HEAD~1)" "Awaits-Review"; hasnot "6.1 the unheld commit has no LOCAL_ONLY" "$(msg HEAD~1)" "LOCAL_ONLY"
eq "6.1 origin holds the unheld commit" "$(tipof a)" "$(git -C "$W" rev-parse HEAD~1)"
HRUN1="$RUN"
# a later run of another stream makes its own commit above the held one, pushes nothing past it
echo v > "$W/src/v.txt"; paths 'src/v.txt\n'; run --paths-from "$C/p.txt" "other stream"; eq "6.2 a later unheld commit above the held one: 14" "$RC" 14; eq "6.2 origin still at the unheld commit" "$(tipof a)" "$(git -C "$W" rev-parse HEAD~2)"
# verdicts that keep the hold
mkdir -p "$W/$EVR/reviews"; GOJ NO-GO "[\"$HRUN1\"]" 1 > "$W/$EVR/reviews/WP-X.json"; paths "$V\n"; run --paths-from "$C/p.txt" "nogo"
eq "6.3 a NO-GO verdict keeps the hold: 14" "$RC" 14; has "6.3 review_pending" "$OUT" review_pending
# verdict_already_go: a hold on a verdict that holds GO in HEAD is refused before any commit
GOJ GO '["x"]' 0 > "$W/$EVR/reviews/WP-Y.json"; paths "$EVR/reviews/WP-Y.json\n"; run --paths-from "$C/p.txt" "a GO file for another item"
echo w > "$W/src/w.txt"; paths "src/w.txt\t$EVR/reviews/WP-Y.json\n"; h0="$(head_)"; run --paths-from "$C/p.txt" "hold on a GO file"
eq "6.4 a hold on a verdict that holds GO in HEAD: 20" "$RC" 20; has "6.4 verdict_already_go" "$OUT" verdict_already_go; eq "6.4 nothing committed" "$(head_)" "$h0"; rm -f "$W/src/w.txt"
GOJ GO '["z"]' 0 > "$W/$EVR/reviews/WP-Y.json"; paths "$EVR/reviews/WP-Y.json\n"; run --paths-from "$C/p.txt" "edit a GO file"
eq "6.4 a change to a verdict file that holds GO in HEAD: 20" "$RC" 20; has "6.4 verdict_already_go" "$OUT" verdict_already_go; git -C "$W" checkout -q -- "$EVR/reviews/WP-Y.json"
echo w2 > "$W/src/w2.txt"; GOJ GO '["w"]' 0 > "$W/$EVR/reviews/WP-N.json"; paths "src/w2.txt\t$EVR/reviews/WP-N.json\n$EVR/reviews/WP-N.json\n"; run --paths-from "$C/p.txt" "hold with the GO in the same change set"
eq "6.4 a hold together with that verdict declared GO in the same change set: 20" "$RC" 20; has "6.4 verdict_already_go" "$OUT" verdict_already_go; rm -f "$W/src/w2.txt" "$W/$EVR/reviews/WP-N.json"
# release: a GO committed in HEAD naming the held run releases the whole range
GOJ GO "[\"$HRUN1\"]" 0 > "$W/$EVR/reviews/WP-X.json"; paths "$V\n"; run --paths-from "$C/p.txt" "GO for the held run"
eq "6.5 a GO committed in HEAD naming the held run releases the range: 0" "$RC" 0; eq "6.5 origin holds HEAD" "$(tipof a)" "$(head_)"; eq "6.5 mirror holds HEAD" "$(tipof b)" "$(head_)"

# --awaits-review gives the verdict to every declared path that has none; `--` ends the options
mk held4
V4="$EVR/reviews/WP-A.json"; echo a > "$W/src/aw.txt"; paths 'src/aw.txt\n'; run --awaits-review "$V4" --paths-from "$C/p.txt" -- "all held"
eq "6.8 --awaits-review holds every path without a verdict of its own: 14" "$RC" 14; has "6.8 the commit awaits that verdict" "$(msg)" "Awaits-Review: $V4"
# a GO whose covers_runs lacks the held run keeps the hold
mk held2
V2="$EVR/reviews/WP-Z.json"; echo h > "$W/src/h.txt"; paths "src/h.txt\t$V2\n"; run --paths-from "$C/p.txt" "held"; eq "6.6 setup: held commit: 14" "$RC" 14; tipbefore="$(tipof a)"
mkdir -p "$W/$EVR/reviews"; GOJ GO '["some-other-run"]' 0 > "$W/$EVR/reviews/WP-Z.json"; paths "$V2\n"; run --paths-from "$C/p.txt" "go for another run"
eq "6.6 a GO whose covers_runs lacks the held run keeps the hold: 14" "$RC" 14; has "6.6 review_pending" "$OUT" review_pending; eq "6.6 nothing past the held commit reached origin" "$(tipof a)" "$tipbefore"
# a NO-GO verdict file in HEAD whose schema is invalid keeps the hold too (an uncommitted GO releases nothing: the verdict is read from the committed HEAD)
mk held3
V3="$EVR/reviews/WP-W.json"; echo h > "$W/src/h.txt"; paths "src/h.txt\t$V3\n"; run --paths-from "$C/p.txt" "held"; HR3="$RUN"
mkdir -p "$W/$EVR/reviews"; GOJ GO "[\"$HR3\"]" 0 > "$W/$EVR/reviews/WP-W.json"; echo q > "$W/src/q.txt"; paths 'src/q.txt\n'
run --paths-from "$C/p.txt" "another stream, the GO file only in the working tree"
eq "6.7 a GO that exists only in the working tree, not committed: the hold stays (14) or the stray file is refused (13)" "$(case $RC in 13|14) echo ok;; *) echo $RC;; esac)" ok
eq "6.7 nothing past the held commit reached origin" "$(tipof a)" "$(git -C "$W" rev-parse HEAD~2 2>/dev/null)"
fi

# ======== 7 diverged repositories: the CPA merge ========================================================================================
if sect "7 merge"; then
mk merge1
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local work"; eq "7.1 setup: a --local-only commit: 14" "$RC" 14; L="$(head_)"
foreign src/f.txt 'f\n'; F="$FOREIGN"; run
eq "7.1 diverged repository, no held commit, no conflict: 0" "$RC" 0; eq "7.1 HEAD is a merge commit" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w)" 3
eq "7.1 the local tip is the merge's first parent" "$(git -C "$W" rev-parse HEAD^1)" "$L"; has "7.1 the merge carries the CPA-Run trailer" "$(msg)" "CPA-Run: $RUN"
has "7.1 one Foreign-Commit line for the foreign commit" "$(msg)" "Foreign-Commit: $F"; eq "7.1 origin holds the merge" "$(tipof a)" "$(head_)"; eq "7.1 mirror holds the merge" "$(tipof b)" "$(head_)"
bun="$(ls "$RD"/backup/*.bundle 2>/dev/null | head -1)"; [ -n "$bun" ] && git -C "$W" bundle verify "$bun" >/dev/null 2>&1 && ok "7.1 the backup bundle verifies" || bad "7.1 no verifying backup bundle under backup/ ($(ls "$RD/backup" 2>&1 | tr '\n' ' '))"
eq "7.1 the local commit keeps its sha below the merge" "$(git -C "$W" rev-parse HEAD~1)" "$L"
tracked_clean && ok "7.1 tracked tree clean" || bad "7.1 dirty"

# a held commit below the merge: the merge is held, nothing is pushed past the held commit
mk merge2
VH="$EVR/reviews/WP-H.json"; echo h > "$W/src/h.txt"; paths "src/h.txt\t$VH\n"; run --paths-from "$C/p.txt" "held work"; eq "7.2 setup: held commit: 14" "$RC" 14; HR="$RUN"; HC="$(head_)"
foreign src/f.txt 'f\n'; run; eq "7.2 a held commit below the merge: merge held, 14" "$RC" 14; MR="$RUN"
has "7.2 the merge awaits its merge-review file" "$(msg)" "Awaits-Review: $EVR/reviews/CPA-merge-$MR.json"; eq "7.2 nothing pushed to origin" "$(tipof a)" "$(git -C "$W" rev-parse HEAD^2)"; eq "7.2 no push call past the held commit" "$(pushes)" 0
# held commit's verdict GO committed, merge verdict still absent: no push call for any remote, 14, names the merge-review file
mkdir -p "$W/$EVR/reviews"; GOJ GO "[\"$HR\"]" 0 > "$W/$EVR/reviews/WP-H.json"; paths "$VH\n"; run --paths-from "$C/p.txt" "GO for the held commit"
eq "7.3 held verdict GO but merge verdict absent: 14" "$RC" 14; eq "7.3 no push call for any remote" "$(pushes)" 0; has "7.3 names the merge-review file" "$OUT" "CPA-merge-$MR.json"
# the merge verdict: the whole range is pushed
GOJ GO "[\"$MR\"]" 0 > "$W/$EVR/reviews/CPA-merge-$MR.json"; paths "$EVR/reviews/CPA-merge-$MR.json\n"; run --paths-from "$C/p.txt" "GO for the merge"
eq "7.4 both verdicts GO: whole range pushed, 0" "$RC" 0; eq "7.4 origin holds HEAD" "$(tipof a)" "$(head_)"; eq "7.4 mirror holds HEAD" "$(tipof b)" "$(head_)"

# a remote that moved after S1: no push call for it, 11; the next run integrates it
mk moved
cat > "$C/hook.sh" <<HK
#!/bin/bash
cd "$C" && rm -rf f2 && $REALGIT clone -q fx/b.git f2 && cd f2 && $REALGIT config user.email f@f && $REALGIT config user.name f && echo late > late.txt && $REALGIT add -A && $REALGIT commit -qm late && $REALGIT push -q "$C/fx/b.git" main
HK
chmod 755 "$C/hook.sh"; echo m > "$W/src/m.txt"; paths 'src/m.txt\n'
XENV="GITSHIM_HOOK_AFTER_COMMIT=$C/hook.sh" run --paths-from "$C/p.txt" "m"
eq "7.5 a remote moved after S1: 11" "$RC" 11; has "7.5 remote_moved_since_s1" "$OUT" remote_moved_since_s1
run; eq "7.5 the next run integrates it: 0" "$RC" 0; eq "7.5 mirror holds HEAD" "$(tipof b)" "$(head_)"; eq "7.5 origin holds HEAD" "$(tipof a)" "$(head_)"

# a conflict: 12, markers copied under conflicts/, HEAD and tracked files unchanged
mk conflict
sed -i 's/^L2$/LOCAL/' "$W/src/conf.txt"; paths 'src/conf.txt\n'; run --local-only --paths-from "$C/p.txt" "local edit"; L="$(head_)"
foreign src/conf.txt 'L1\nFOREIGN\nL3\nL4\nL5\n'
snapf() { ( cd "$W" && git ls-files -z | xargs -0 sha256sum 2>/dev/null | sha256sum ); }; snap="$(snapf)"; run
eq "7.6 a merge with a conflict: 12" "$RC" 12; has "7.6 merge_conflict" "$OUT" merge_conflict; eq "7.6 HEAD unchanged" "$(head_)" "$L"
eq "7.6 sha256 of every tracked file unchanged" "$(snapf)" "$snap"
grep -q '<<<<<<<' "$RD/conflicts/src/conf.txt" 2>/dev/null && ok "7.6 the conflicting file is copied with its markers" || bad "7.6 no conflicts/src/conf.txt with markers"
CRUN="$RUN"
# --resolve-merge: the wrong directory, then a record that names another model
run --resolve-merge "$C/elsewhere"; eq "7.7 a --resolve-merge directory at any other path: 20" "$RC" 20; has "7.7 resolution_dir_invalid" "$OUT" resolution_dir_invalid
mkdir -p "$W/.audit/merge-resolution/$CRUN"; printf '{"tip":"%s","paths":["src/conf.txt"],"model":"sonnet","effort":"high","files":{},"method":{}}\n' "$FOREIGN" > "$W/.audit/merge-resolution/$CRUN/resolution.json"
run --resolve-merge "$W/.audit/merge-resolution/$CRUN"; eq "7.8 a record naming another model or effort: 20" "$RC" 20; has "7.8 merge_resolver_not_pinned" "$OUT" merge_resolver_not_pinned
# two remote tips that conflict with each other
mk rdiv
h0="$(head_)"; foreign src/conf.txt 'L1\nTIP_A\nL3\nL4\nL5\n' a; foreign src/conf.txt 'L1\nTIP_B\nL3\nL4\nL5\n' b
run; eq "7.9 two conflicting remote tips: 12" "$RC" 12; has "7.9 remotes_diverged" "$OUT" remotes_diverged; eq "7.9 nothing moved" "$(head_)" "$h0"; eq "7.9 no push call" "$(pushes)" 0
# the same with a local commit: both tips are non-ancestors, the merge helper judges the pair
mk rdiv2
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; L2="$(head_)"
foreign src/conf.txt 'L1\nTIP_A\nL3\nL4\nL5\n' a; foreign src/conf.txt 'L1\nTIP_B\nL3\nL4\nL5\n' b
run; eq "7.9c a local commit and two conflicting remote tips: 12" "$RC" 12; has "7.9c remotes_diverged naming the pair" "$OUT" remotes_diverged; eq "7.9c nothing moved" "$(head_)" "$L2"; eq "7.9c no push call" "$(pushes)" 0
mk rdiv3
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"
foreign src/x.txt 'x\n' a; FA="$FOREIGN"; foreign src/y.txt 'y\n' b; FB="$FOREIGN"
run; eq "7.9d two remote tips that do not conflict, a local commit: merged in turn, 0" "$RC" 0
git -C "$W" merge-base --is-ancestor "$FA" HEAD && git -C "$W" merge-base --is-ancestor "$FB" HEAD && ok "7.9d both foreign commits are in HEAD" || bad "7.9d a foreign commit is missing from HEAD"
eq "7.9d origin holds HEAD" "$(tipof a)" "$(head_)"; eq "7.9d mirror holds HEAD" "$(tipof b)" "$(head_)"
# incoming commits that change a path with an uncommitted change
mk ffblock
foreign src/z.txt 'M1\nM2\nM3\nM4\nREMOTE\nM6\nM7\nM8\n'; echo local >> "$W/src/z.txt"; paths 'src/z.txt\n'; h0="$(head_)"; run --paths-from "$C/p.txt" "z"
eq "7.10 incoming commits change a declared, uncommitted path: 12" "$RC" 12; has "7.10 ff_blocked_by_local_changes" "$OUT" ff_blocked_by_local_changes; eq "7.10 nothing moved" "$(head_)" "$h0"
# the same window with --commit-before-integrate
sed -i 's/^M1$/MYLINE/' "$W/src/z.txt"; sed -i '$d' "$W/src/z.txt"; run --commit-before-integrate --paths-from "$C/p.txt" "z"
eq "7.11 --commit-before-integrate over other lines: clean merge pushed, 0" "$RC" 0; eq "7.11 origin holds HEAD" "$(tipof a)" "$(head_)"
eq "7.11 the content of both sides survived" "$(grep -c 'MYLINE\|REMOTE' "$W/src/z.txt")" 2
has "7.11 the merge names the foreign commit" "$(msg)" "Foreign-Commit: $FOREIGN"; hasnot "7.11 the run's S5 commit names no foreign commit" "$(msg HEAD^1)" "Foreign-Commit"
[ -f "$RD/backup/s1a/backup.json" ] && ok "7.11 the S1a backup lies under backup/s1a/" || bad "7.11 no backup/s1a/backup.json"
eq "7.11 S1a: no bundle, reason local_range_empty" "$(jq -r '(.bundle|tostring)+":"+.reason' "$RD/backup/s1a/backup.json" 2>/dev/null)" "null:local_range_empty"
ls "$RD"/backup/*.bundle* >/dev/null 2>&1 && ok "7.11 the S1b merge backup lies under backup/ beside the S1a one" || bad "7.11 no S1b backup under backup/"
# --commit-before-integrate with the same lines: 12, the commits stay local
mk cbiconf
foreign src/conf.txt 'L1\nFOREIGN\nL3\nL4\nL5\n'; sed -i 's/^L2$/MINE/' "$W/src/conf.txt"; paths 'src/conf.txt\n'; run --commit-before-integrate --paths-from "$C/p.txt" "mine"
eq "7.12 --commit-before-integrate over the same lines: 12" "$RC" 12; has "7.12 merge_conflict" "$OUT" merge_conflict; [ "$(git -C "$W" log -1 --format=%s)" = mine ] && ok "7.12 the run's commit is kept in the local branch" || bad "7.12 commit lost"
eq "7.12 pushed nowhere" "$(pushes)" 0; [ -f "$RD/conflicts/src/conf.txt" ] && ok "7.12 the conflicting file is copied" || bad "7.12 no conflicts copy"
# a backup that cannot be verified: 20 before S2, nothing committed
mk cbibk
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "l"; echo n > "$W/src/n.txt"; paths 'src/n.txt\n'; h0="$(head_)"
XENV="GITSHIM_FAIL_ON=bundle.verify" run --commit-before-integrate --paths-from "$C/p.txt" "n"
eq "7.13 a bundle that git bundle verify rejects: 20" "$RC" 20; has "7.13 backup_failed" "$OUT" backup_failed; eq "7.13 nothing committed" "$(head_)" "$h0"
# a live holder paused between its `git merge --no-commit` and its commit: a second run is refused with lock_held (no abort remediation); a dead one leaves merge_in_progress
mk livemerge
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; foreign src/f.txt 'f\n'
: > "$C/git.log"; export GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"
( cd "$W" && env $(ENVV) GITSHIM_PAUSE_ON='(^| )commit( |$)' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go" "$C/bin/cpa-host" ) >"$C/hold.out" 2>"$C/hold.err" &
HP=$!; n=0; while [ ! -e "$C/paused" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done
[ -e "$W/.git/MERGE_HEAD" ] && ok "7.16 the holder is paused with MERGE_HEAD in place" || bad "7.16 no MERGE_HEAD while paused"
run; eq "7.16 a second run while a live holder pauses between the merge and its commit: 20" "$RC" 20; has "7.16 lock_held" "$OUT" lock_held; hasnot "7.16 no git merge --abort remediation" "$OUT" "merge --abort"
: > "$C/go"; wait $HP; eq "7.16 the holder finishes unaffected with its merge: 0" "$?" 0; unset GITSHIM_PAUSED GITSHIM_PAUSE_UNTIL
eq "7.16 the holder's merge commit exists" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w)" 3
mk deadmerge
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; foreign src/f.txt 'f\n'
git -C "$W" fetch -q origin; git -C "$W" merge --no-ff --no-commit origin/main >/dev/null 2>&1
mkdir -p "$W/.audit/commit-push/20260101T000000Z-1-dead"; printf '{"repository":".","tip":"x","local_tip":"y","run_id":"20260101T000000Z-1-dead","pid":99999999,"pid_start":"1"}\n' > "$W/.audit/commit-push/20260101T000000Z-1-dead/merge.json"
run; eq "7.17 MERGE_HEAD left by a run whose process is gone: 20" "$RC" 20; has "7.17 merge_in_progress" "$OUT" merge_in_progress; has "7.17 names the interrupted run" "$OUT" "20260101T000000Z-1-dead"; has "7.17 the remediation" "$OUT" "merge --abort"
git -C "$W" merge --abort; run; eq "7.17 after the abort a new run integrates the tip: 0" "$RC" 0
# S1a unrecorded-commit guard and a verifying bundle for a non-empty local range
mk cbihand
echo hand > "$W/src/h.txt"; git -C "$W" add src/h.txt; git -C "$W" commit -qm "hand commit"; echo n > "$W/src/n.txt"; paths 'src/n.txt\n'; h0="$(head_)"; run --commit-before-integrate --paths-from "$C/p.txt" "n"
eq "7.18 --commit-before-integrate with a plain git commit that no remote holds: 20" "$RC" 20; has "7.18 unrecorded_local_commit" "$OUT" unrecorded_local_commit; eq "7.18 nothing committed" "$(head_)" "$h0"
mk cbibun
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; echo n > "$W/src/n.txt"; paths 'src/n.txt\n'
run --commit-before-integrate --paths-from "$C/p.txt" "n"; eq "7.19 --commit-before-integrate over a non-empty local range, no remote move: 0" "$RC" 0
bun="$RD/backup/s1a/local.bundle"; [ -f "$bun" ] && git -C "$W" bundle verify "$bun" >/dev/null 2>&1 && ok "7.19 the S1a bundle verifies" || bad "7.19 no verifying S1a bundle"
eq "7.19 backup.json names the bundle" "$(jq -r '.bundle|type' "$RD/backup/s1a/backup.json" 2>/dev/null)" string
# foreign commits: named once, by the first commit the run makes
mk foreign1
foreign src/f.txt 'f\n'; F="$FOREIGN"; echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "own change"
eq "7.14 a fast-forwarded foreign commit and an own change: 0" "$RC" 0; has "7.14 the run's first commit names the foreign commit" "$(msg)" "Foreign-Commit: $F"
echo b > "$W/src/b.txt"; paths 'src/b.txt\n'; run --paths-from "$C/p.txt" "second change"; hasnot "7.14 no commit is named twice" "$(msg)" "Foreign-Commit"
# a fast-forward that moves a gitlink: nothing settles it, so S7 gives 15 for the changed path
mk gitlink1
rm -rf "$C/subc"; git clone -q "$T/fx/sub.git" "$C/subc" 2>/dev/null; git -C "$C/subc" config user.email f@f; git -C "$C/subc" config user.name f; echo n > "$C/subc/new.txt"; git -C "$C/subc" add -A; git -C "$C/subc" commit -qm newsub; git -C "$C/subc" push -q origin main
rm -rf "$C/f"; git clone -q "$C/fx/a.git" "$C/f" 2>/dev/null; git -C "$C/f" config user.email f@f; git -C "$C/f" config user.name f
git -C "$C/f" update-index --add --cacheinfo "160000,$(git -C "$C/subc" rev-parse HEAD),mods/sub"; git -C "$C/f" commit -qm "bump the pin"; git -C "$C/f" push -q "$C/fx/a.git" main; git -C "$C/f" push -q "$C/fx/b.git" main
run; eq "7.15 a fast-forward that changed a gitlink with no settle helper: 15" "$RC" 15; has "7.15 the path is named" "$OUT" mods/sub; has "7.15 reason" "$(jq -r '.nested_unsettled|join(" ")' "$REP" 2>/dev/null)$OUT" nested
fi

# ======== 8 S7 mapping and pointer drift =====================================================================================================
if sect "8 verifier"; then
mk s7unreach
echo a > "$W/src/a.txt"; paths 'src/a.txt\n'
XENV="GITSHIM_FAIL_AFTER_PUSH=ls-remote" run --paths-from "$C/p.txt" "a"
eq "8.1 a remote unreachable at S7 (verifier 14): 11, never 14" "$RC" 11
mk s7noreport
printf '#!/bin/bash\nexit 0\n' > "$W/scripts/repo/verify_repos.sh"; chmod 755 "$W/scripts/repo/verify_repos.sh"; git -C "$W" add -A; git -C "$W" commit -qm shim; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
run; eq "8.2 a verifier exit 0 whose report is missing: 20 (fail closed)" "$RC" 20
mk drift
git -C "$W/mods/sub" commit -q --allow-empty -m "ahead"; git -C "$W/mods/sub" push -q origin main 2>/dev/null; run
eq "8.3 pointer drift with no pending row: 15" "$RC" 15; has "8.3 the drifted path is named" "$OUT$(cat "$RD/pins.json" 2>/dev/null)" "mods/sub"
eq "8.3 a drifted submodule is not moved" "$(git -C "$W" ls-tree HEAD mods/sub | awk '{print $3}')" "$(git -C "$W" ls-tree "origin/main" mods/sub | awk '{print $3}')"
printf 'mods/sub\t%s\tsomerun\n' "0000000000000000000000000000000000000000" > "$W/.audit/pending_pins.tsv"; run
eq "8.4 a pending row whose sha differs from the head: still 15" "$RC" 15
printf 'mods/sub\t%s\tsomerun\n' "$(git -C "$W/mods/sub" rev-parse HEAD)" > "$W/.audit/pending_pins.tsv"; run
eq "8.5 a pending row that matches path and head: reported pending, 0" "$RC" 0; has "8.5 reported as pending" "$(cat "$RD/pins.json" 2>/dev/null)" "mods/sub"
fi

# ======== 9 --repo mode ======================================================================================================================
if sect "9 repo mode"; then
mk repo
before="$(git -C "$W" rev-parse HEAD):$(git -C "$W" ls-tree HEAD mods/sub | awk '{print $3}')"
echo r > "$W/mods/sub/r.txt"; paths 'mods/sub/r.txt\n'; run --repo mods/sub --paths-from "$C/p.txt" "sub work"
eq "9.1 --repo commit in a submodule: 0" "$RC" 0; eq "9.1 the main HEAD and the recorded gitlink are unchanged" "$(git -C "$W" rev-parse HEAD):$(git -C "$W" ls-tree HEAD mods/sub | awk '{print $3}')" "$before"
eq "9.1 the submodule holds the commit" "$(git -C "$W/mods/sub" log -1 --format=%s)" "sub work"; has "9.1 the commit carries CPA-Run" "$(git -C "$W/mods/sub" log -1 --format=%B)" "CPA-Run: $RUN"
eq "9.1 the submodule's remote holds it" "$(git -C "$T/fx/sub.git" rev-parse main 2>/dev/null)" "$(git -C "$W/mods/sub" rev-parse HEAD)"
eq "9.1 a pending pin row is recorded" "$(awk -F'\t' '$1=="mods/sub"{print $2}' "$W/.audit/pending_pins.tsv" 2>/dev/null)" "$(git -C "$W/mods/sub" rev-parse HEAD)"
! grep -E ' -C [^ ]*(/w|\.) .*push|(^| )push .*origin' "$C/git.log" | grep -v 'mods/sub' >/dev/null 2>&1 && ok "9.1 no push call for the main repository" || bad "9.1 a push call for the main repository: $(grep push "$C/git.log" | head -2)"
run; eq "9.2 a main-repository run after a --repo run reports the pin as pending, never 15: 0" "$RC" 0
run --repo not/a/repo; eq "9.3 --repo of a path that is no repository: 20" "$RC" 20
run --repo mods/ext; eq "9.4 --repo of a third-party repository: 20" "$RC" 20; has "9.4 repo_not_owned" "$OUT" repo_not_owned
# a held --repo commit: 14 and local; the verdict of the main repository releases it
mk repoheld
VR="$EVR/reviews/WP-S.json"; echo hh > "$W/mods/sub/hh.txt"; paths "mods/sub/hh.txt\t$VR\n"; before="$(git -C "$T/fx/sub.git" rev-parse main)"; run --repo mods/sub --paths-from "$C/p.txt" "held in the submodule"
eq "9.5 a held --repo commit: 14" "$RC" 14; has "9.5 review_pending" "$OUT" review_pending; HRS="$RUN"; eq "9.5 the submodule's remote does not hold it" "$(git -C "$T/fx/sub.git" rev-parse main)" "$before"
mkdir -p "$W/$EVR/reviews"; GOJ GO '["some-other-run"]' 0 > "$W/$EVR/reviews/WP-S.json"; paths "$VR\n"; run --paths-from "$C/p.txt" "a GO that does not cover the held run"
eq "9.6 a GO without the held run in covers_runs: the --repo commit stays held, 14" "$RC" 14; eq "9.6 still not on the submodule's remote" "$(git -C "$T/fx/sub.git" rev-parse main)" "$before"
mk repoheld2
echo hh > "$W/mods/sub/hh.txt"; paths "mods/sub/hh.txt\t$VR\n"; run --repo mods/sub --paths-from "$C/p.txt" "held in the submodule"; HRS="$RUN"
mkdir -p "$W/$EVR/reviews"; GOJ GO "[\"$HRS\"]" 0 > "$W/$EVR/reviews/WP-S.json"; paths "$VR\n"; run --paths-from "$C/p.txt" "GO for the held --repo run"
eq "9.7 a main-repository run after the GO also pushes the submodule's released range: 0" "$RC" 0
eq "9.7 the submodule's remote holds the released commit" "$(git -C "$T/fx/sub.git" rev-parse main)" "$(git -C "$W/mods/sub" rev-parse HEAD)"
fi

# ======== 10 host entry point =================================================================================================================
if sect "10 host entry"; then
mk host
# started directly, not through cpa-host: the copied script's self-test refuses
( cd "$W" && env $(ENVV) "$W/scripts/commit-push-all.sh" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
eq "10.1 the tracked script started directly: 20" "$RC" 20; has "10.1 not_via_host_entry" "$(cat "$C/err.txt")" not_via_host_entry; ls "$W/.audit/commit-push" >/dev/null 2>&1 && bad "10.1 wrote a run directory" || ok "10.1 wrote nothing"
run; eq "10.2 through cpa-host: 0 (control)" "$RC" 0
# a subdirectory and a submodule resolve to the main root
CWD="$W/src" run; eq "10.3 started from a subdirectory: 0" "$RC" 0; ls "$W/src/.audit" >/dev/null 2>&1 && bad "10.3 .audit created in the subdirectory" || ok "10.3 only the main root's .audit is written"
CWD="$W/mods/sub" run; eq "10.4 started from inside a submodule: 0" "$RC" 0; ls "$W/mods/sub/.audit" >/dev/null 2>&1 && bad "10.4 .audit created in the submodule" || ok "10.4 only the main root's .audit is written"; unset CWD
# trust refusals
nr="$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)"; mv "$C/state/trust.json" "$C/state/trust.off"; run; eq "10.5 no trust file: 20 project_not_trusted" "$RC" 20; has "10.5 reason" "$OUT" project_not_trusted; eq "10.5 no run directory was created" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)" "$nr"
mv "$C/state/trust.off" "$C/state/trust.json"
jq '.projects|=with_entries(.value.adoption_commit="0000000000000000000000000000000000000000")' "$C/state/trust.json" > "$C/state/t2" && mv "$C/state/t2" "$C/state/trust.json"; run
eq "10.6 an adoption commit off the first-parent chain: 20" "$RC" 20; has "10.6 adoption_commit_not_first_parent" "$OUT" adoption_commit_not_first_parent
mk host2
rm -rf "$T/standalone"; mkrepo "$T/standalone"; mkdir -p "$T/standalone/scripts/repo/host_entry"; cp "$C/bin/cpa-host" "$T/standalone/scripts/repo/host_entry/"; commit_all "$T/standalone" x
CWD="$T/standalone" run; eq "10.7 a stand-alone repository with no trust entry: 20 project_not_trusted" "$RC" 20; has "10.7 reason" "$OUT" project_not_trusted; unset CWD
mk host3
run --run-id 'a/b'; eq "10.8 a malformed --run-id: 20" "$RC" 20; has "10.8 run_id_malformed" "$OUT" run_id_malformed
run --run-id fixed-id-1; eq "10.9 a fresh --run-id is used: 0" "$RC" 0; eq "10.9 the run directory carries it" "$(ls "$W/.audit/commit-push")" fixed-id-1
run --run-id fixed-id-1; eq "10.10 a run id in use: 20" "$RC" 20; has "10.10 run_id_in_use" "$OUT" run_id_in_use
run --run-id x1 --resume fixed-id-1; eq "10.11 --run-id with --resume: 20 usage_error" "$RC" 20; has "10.11 usage_error" "$OUT" usage_error
# an approved copy changed by one byte in the store
mk host4
b="$(jq -r '.projects|to_entries[0].value.manifest[]|select(.path=="scripts/repo/lib_safe.sh").sha256' "$C/state/trust.json")"; chmod 600 "$C/state/blobs/$b"; echo '#' >> "$C/state/blobs/$b"; chmod 400 "$C/state/blobs/$b"
run; eq "10.12 a store blob changed by one byte: 20" "$RC" 20; has "10.12 approved_copy_invalid" "$OUT" approved_copy_invalid; eq "10.12 no run directory is left" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)" 0
# a forged snapshot: the copied script's self-test refuses a manifest no approval recorded
mk host5
run; RD5="$RD"; snap="$RD5/released/trust-snapshot.json"; chmod u+w "$snap"; jq '.entries|=map(if .path=="scripts/repo/lib_safe.sh" then .sha256="'"$(printf x | sha256sum | cut -d' ' -f1)"'" else . end) | .manifest_sha256=("0"*64)' "$snap" > "$C/snap.new"
ents="$(jq -cS '.entries|sort_by(.path)' "$C/snap.new")"; jq --arg m "$(printf '%s' "$ents" | sha256sum | cut -d' ' -f1)" '.manifest_sha256=$m' "$C/snap.new" > "$snap"
( cd "$W" && env $(ENVV) CPA_ROOT="$W" CPA_RUN_ID="$(basename "$RD5")" "$RD5/released/scripts/commit-push-all.sh" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
eq "10.13 a snapshot whose manifest no approval recorded: 20" "$RC" 20; has "10.13 snapshot_not_trusted" "$(cat "$C/err.txt")" snapshot_not_trusted
# a revoked manifest
mk host6
jq --arg ms "$TRUSTMS" '.projects|=with_entries(.value.history+=[{op:"revoke",manifest_sha256:$ms,time:"2026-10-06T00:00:00Z"}])' "$C/state/trust.json" > "$C/state/t2" && mv "$C/state/t2" "$C/state/trust.json"
run; eq "10.14 a revoked manifest is refused by the self-test: 20" "$RC" 20; has "10.14 snapshot_not_trusted" "$OUT" snapshot_not_trusted
mk host7
run --show; eq "10.15 --show: 0" "$RC" 0; has "10.15 --show prints the project entry" "$OUT" "adoption_commit"
run --owner-trust approve x; eq "10.16 an owner operation is not an agent operation: 20" "$RC" 20; has "10.16 owner_op_unimplemented" "$OUT" owner_op_unimplemented
for m in "--resume x" "--exec-approved scripts/repo/lib_safe.sh" "--check-provenance v.json"; do run $m; eq "10.17 cpa-host $m (not built): 20" "$RC" 20; has "10.17 mode_unimplemented" "$OUT" mode_unimplemented; done
run --run-id; eq "10.18 --run-id without a value: 20 usage_error" "$RC" 20; has "10.18 usage_error" "$OUT" usage_error
CWD="$T" run; eq "10.19 started outside every repository: 20 no_project_root" "$RC" 20; has "10.19 reason" "$OUT" no_project_root; unset CWD
mk host8
cp "$C/state/trust.json" "$C/state/trust.keep"; printf '{ not json' > "$C/state/trust.json"; run; eq "10.20 an unparsable trust file: 20" "$RC" 20; has "10.20 trust_file_unparsable" "$OUT" trust_file_unparsable
: > "$C/state/trust.json"; run; eq "10.20 an EMPTY trust file (jq 1.6 reads empty input as success): 20" "$RC" 20; has "10.20 trust_file_absent" "$OUT" trust_file_absent
cp "$C/state/trust.keep" "$C/state/trust.json"; jq '.projects|=with_entries(.value.manifest|=map(select(.path!="scripts/commit-push-all.sh")))' "$C/state/trust.json" > "$C/state/t2" && mv "$C/state/t2" "$C/state/trust.json"
run; eq "10.21 a manifest without scripts/commit-push-all.sh: 20" "$RC" 20; has "10.21 core_not_in_manifest" "$OUT" core_not_in_manifest
cp "$C/state/trust.keep" "$C/state/trust.json"; b="$(jq -r '.projects|to_entries[0].value.manifest[]|select(.path=="scripts/repo/lib_safe.sh").sha256' "$C/state/trust.json")"; chmod 600 "$C/state/blobs/$b"; rm -f "$C/state/blobs/$b"
run; eq "10.22 a manifest entry whose store blob is gone: 20" "$RC" 20; has "10.22 store_blob_missing" "$OUT" store_blob_missing; eq "10.22 no run directory is left" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)" 0
fi

# ======== 13 helper exit mapping: every status outside a helper's documented set is 20 ============================================================
if sect "13 helpers"; then
ffshim() { # ffshim <exit> <reason>: an integrate_ff_only.sh that writes its --json and exits
  hshim scripts/repo/integrate_ff_only.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; printf "{\"reason\":\"%s\"}\n" "'"$2"'" > "$j"; exit '"$1"
}
mk hmap1; ffshim 11 remote_unreachable; run; eq "13.1 integrate_ff_only 11: 11" "$RC" 11; has "13.1 reason" "$OUT" remote_unreachable
mk hmap2; ffshim 12 ff_blocked_by_local_changes; run; eq "13.2 integrate_ff_only 12 with a reason that is not a merge-path one: 12" "$RC" 12; has "13.2 reason" "$OUT" ff_blocked_by_local_changes
mk hmap3; ffshim 20 pin_not_on_remote; run; eq "13.3 integrate_ff_only 20: 20" "$RC" 20; has "13.3 reason" "$OUT" pin_not_on_remote
mk hmap4; ffshim 7 x; run; eq "13.4 integrate_ff_only exit 7 (outside its set): 20, never read as success" "$RC" 20; has "13.4 internal_error" "$OUT" internal_error
mk hmap5; hshim scripts/repo/push_recursive.sh 'echo "REFUSED	.	unrecorded_local_commit	abc"; exit 20'; echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; run --paths-from "$C/p.txt" "a"
eq "13.5 push_recursive 20: 20" "$RC" 20; has "13.5 reason" "$OUT" unrecorded_local_commit
mk hmap6; hshim scripts/repo/push_recursive.sh 'exit 7'; run; eq "13.6 push_recursive exit 7 (outside its set): 20" "$RC" 20
mk hmap7; hshim scripts/repo/verify_repos.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; echo "{\"summary\":{\"pin_drift\":0,\"ahead\":0,\"dirty\":0},\"repos\":[]}" > "$j"; exit 20'; run
eq "13.7 a verifier exit 20 with a readable report: 20 (blind verifier)" "$RC" 20; has "13.7 verifier_blind" "$OUT" verifier_blind
mk hmap8; hshim scripts/repo/verify_repos.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; echo "{\"summary\":{\"pin_drift\":0,\"ahead\":1,\"dirty\":0},\"repos\":[]}" > "$j"; exit 11'; run
eq "13.8 a verifier 11 (commits not on a remote) with nothing held: 11" "$RC" 11; has "13.8 reason" "$OUT" commits_not_on_a_remote
fi

# ======== 12 no hooks (CENTRAL C4) ============================================================================================================
if sect "12 hooks"; then
plant() { # plant <git dir> <marker>: hooks that write a marker and succeed
  local hd="$1/hooks" h; mkdir -p "$hd"; for h in pre-commit commit-msg post-commit pre-push post-merge reference-transaction pre-merge-commit; do
    printf '#!/bin/sh\necho %s >> "%s"\nexit 0\n' "$h" "$2" > "$hd/$h"; chmod 755 "$hd/$h"; done
}
# control needle: the same hooks DO fire in an ordinary git commit and merge, so an absent marker below means CPA suppressed them
rm -rf "$T/hookneedle"; mkrepo "$T/hookneedle"; plant "$T/hookneedle/.git" "$T/needle.marker"; echo x > "$T/hookneedle/x"; git -C "$T/hookneedle" add x; git -C "$T/hookneedle" commit -qm x
[ -s "$T/needle.marker" ] && ok "12.0 control needle: a planted hook fires in an ordinary commit" || bad "12.0 the planted hook does not fire: the check below would be blind"
mk hooks
plant "$W/.git" "$C/hook.marker"; mkdir -p "$W/.git/modules/mods/sub"; plant "$W/.git/modules/mods/sub" "$C/hook.marker"
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; foreign src/f.txt 'f\n'; run
eq "12.1 a window that commits, merges and pushes under planted hooks: 0" "$RC" 0; eq "12.1 the merge was made" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w)" 3
[ ! -e "$C/hook.marker" ] && ok "12.1 no hook ran (pre-commit, commit-msg, post-commit, pre-push, post-merge, reference-transaction, pre-merge-commit)" || bad "12.1 hooks ran: $(sort -u "$C/hook.marker" | tr '\n' ' ')"
fi

# ======== 11 static properties ================================================================================================================
if sect "11 static"; then
for f in "$H" "$SRC/scripts/repo/host_entry/cpa-host"; do
  [ -f "$f" ] || { bad "11.1 $f is absent"; continue; }
  if grep -nE -- "--force|force-with-lease|--no-verify|[[:space:]]\+refs/|[[:space:]]\+HEAD|git[^|;]* (push|reset|rebase)[^|;]* -f( |\$)|(^|[^a-z_-])rebase |git[^|;]* reset " "$f" | grep -vE '^[0-9]+:[[:space:]]*#' >/dev/null; then bad "11.1 $(basename "$f") contains a force/lease/+refspec/no-verify/reset/rebase token: $(grep -nE -- '--force|force-with-lease|--no-verify|[[:space:]]\+refs/|rebase |git[^|;]* reset ' "$f" | grep -vE '^[0-9]+:[[:space:]]*#' | head -2)"; else ok "11.1 $(basename "$f") has no force/lease/+refspec/no-verify/reset/rebase token"; fi
  bash -n "$f" 2>/dev/null && ok "11.2 $(basename "$f") parses" || bad "11.2 $(basename "$f") does not parse"
done
fi
fin
