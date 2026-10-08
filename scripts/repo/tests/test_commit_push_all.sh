#!/usr/bin/env bash
# T039 / T042a / T043 test (TDD): scripts/commit-push-all.sh (CPA) and its host entry point scripts/repo/host_entry/cpa-host.
# Exit-code matrix on THROWAWAY clones with LOCAL BARE remotes only (docs/16 section 12; tasks.md T039, CENTRAL C3): every case builds a fresh
# clone, two fresh bare remotes under $T/<case>/fx/ and its own test trust state ($CPA_HOST_STATE under $T). No real remote, no hosted URL,
# no network, no credential; the owner's real $CPA_HOST_STATE and $CPA_HOST_ENTRY are never read or written.
# A `git` shim on PATH logs every call; the test fails when a call carries --force, --force-with-lease, a +refspec, --no-verify, rebase or reset.
# The run's environment is an ALLOWLIST (WF17-cpa): the harness controls (GITSHIM_*, SWEEP_*) therefore travel in FILES, not in variables: a per-run shim directory holds a copy of the shim and its
# ctl.env, and the sweep shim reads $ANTIMESS_ROOT/.audit/sweep.ctl. A variable of those families given to `run` (as a prefix, an export or in XENV) is written to the file and ALSO offered to
# the run's environment, where the allowlist must drop it (the proof that the controls do not need the environment).
# Usage: bash scripts/repo/tests/test_commit_push_all.sh   Env: H=<script under test>, CPA_ONLY=<substring of a section name> to run one section
# Output: `ok`/`FAIL` lines and a summary; exit 0 only when no case failed AND the script under test exists.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; SRC="${CPA_SRC:-$D0}"   # SRC: where the scripts under test are read from (a mutated copy under the mutation runner)
H="${H:-$SRC/scripts/commit-push-all.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
REALGIT="$(command -v git)"; T="$(mktemp -d "${TMPDIR:-/tmp}/cpa_test.XXXXXX")"; [ -n "${CPA_KEEP:-}" ] || trap 'chmod -R u+w "$T" 2>/dev/null; rm -rf "$T"' EXIT; [ -z "${CPA_KEEP:-}" ] || echo "NOTE: CPA_KEEP set, scratch tree kept: $T"
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
EVR=specs/001-full-project-audit-remediation/evidence; SCHEMA=specs/001-full-project-audit-remediation/contracts/review-verdict.schema.json
export LONGOPS_ALLOW_TMPFS=1

# ---- the git shim: logs argv; hooks for pauses, a one-shot command before the first push, and failures (test hooks only) -----------------
mkdir -p "$T/shim"; cat > "$T/shim/git" <<SHIM
#!/bin/bash
real="$REALGIT"
[ -f "\$(dirname "\$0")/ctl.env" ] && . "\$(dirname "\$0")/ctl.env"
printf '%s\n' "\$*" >> "\${GITSHIM_LOG:-/dev/null}"
args="\$*"
pushed=0; [ -f "\${GITSHIM_LOG:-/nonexistent}" ] && grep -qE '(^| )push( |\$)' "\$GITSHIM_LOG" && pushed=1
if [ -n "\${GITSHIM_FAIL_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_FAIL_ON ]]; then echo "shim: refused \$args" >&2; exit 128; fi
if [ -n "\${GITSHIM_FAIL_AFTER_PUSH:-}" ] && [ "\$pushed" = 1 ] && [[ "\$args" =~ \$GITSHIM_FAIL_AFTER_PUSH ]]; then echo "shim: refused after push \$args" >&2; exit 128; fi
if [ -n "\${GITSHIM_HOOK_AFTER_COMMIT:-}" ] && [[ "\$args" =~ ls-remote ]] && grep -qE '(^| )commit( |\$)' "\$GITSHIM_LOG" 2>/dev/null && [ ! -e "\$GITSHIM_HOOK_AFTER_COMMIT.done" ]; then : > "\$GITSHIM_HOOK_AFTER_COMMIT.done"; "\$GITSHIM_HOOK_AFTER_COMMIT" >/dev/null 2>&1; fi
if [ -n "\${GITSHIM_SLEEP_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_SLEEP_ON ]]; then sleep "\${GITSHIM_SLEEP_S:-2}"; fi
if [ -n "\${GITSHIM_HOOK_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_HOOK_ON ]] && [ ! -e "\$GITSHIM_HOOK.ran" ]; then : > "\$GITSHIM_HOOK.ran"; "\$GITSHIM_HOOK" >/dev/null 2>&1; fi
if [ -n "\${GITSHIM_SIGNAL_ON:-}" ] && [[ "\$args" =~ \$GITSHIM_SIGNAL_ON ]] && [ ! -e "\$GITSHIM_SIGNALED" ]; then
  q=\$\$; t=""; while [ -n "\$q" ] && [ "\$q" -gt 1 ]; do   # the OUTERMOST ancestor that matches: a command substitution's subshell has the same command line
    if tr '\0' ' ' < "/proc/\$q/cmdline" 2>/dev/null | grep -q -- "\$GITSHIM_SIGNAL_TARGET"; then t="\$q"; fi
    q="\$(sed -e 's/^.*) //' "/proc/\$q/stat" 2>/dev/null | awk '{print \$2}')"
  done
  if [ -n "\$t" ] && [ "\$t" -gt 1 ]; then : > "\$GITSHIM_SIGNALED"; kill -"\$GITSHIM_SIGNAL" "\$t"; fi
fi
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
for f in "$SRC"/scripts/repo/*.sh "$SRC"/scripts/repo/*.tsv "$SRC"/scripts/repo/*.py "$SRC"/scripts/repo/fixture_roots.txt; do cp "$f" "$S/scripts/repo/"; done
cp "$SRC"/scripts/audit/org_of.py "$S/scripts/audit/"; cp "$SRC"/scripts/longops/*.sh "$S/scripts/longops/"
cp "$SRC/scripts/commit-push-all.sh" "$S/scripts/" 2>/dev/null; cp "$SRC/scripts/repo/host_entry/cpa-host" "$S/scripts/repo/host_entry/" 2>/dev/null
printf 'fx\n' > "$S/scripts/audit/own_orgs.txt"
cp "$D0/$SCHEMA" "$S/$SCHEMA"
# a reduced owed-gates list (the fixture models "every gate built", like the reduced check registry below); the REAL list is exercised by section 14
printf '# gate\twhen\tnote\n' > "$S/scripts/repo/owed_gates.tsv"
printf '# the fixture names no long gate (an explicit empty list: S4 runs nothing and owes nothing)\n' > "$S/scripts/repo/long_gates.txt"
# a reduced check registry (the real one has container rows): three file checks and the changeset check, all on the host
{ printf '# check\tcommand\timage\tmode\tbaseline\tscope\tnote\n'
  printf 'shell_parse\tbuiltin:shell_parse\tIMG-TESTUTIL\tplain\t-\tfiles\tbash -n\n'
  printf 'merge_conflict\tbuiltin:merge_conflict\tIMG-TESTUTIL\tplain\t-\tfiles\tmarkers\n'
  printf 'no_ci\tscripts/repo/check_no_ci.sh --root {root}\tIMG-TESTUTIL\tplain\t-\tchangeset\tno pipeline\n'; } > "$S/scripts/repo/validate_checks.tsv"
printf '/.audit/\n' > "$S/.gitignore"
printf 'L1\nL2\nL3\nL4\nL5\n' > "$S/src/conf.txt"; printf 'M1\nM2\nM3\nM4\nM5\nM6\nM7\nM8\n' > "$S/src/z.txt"; echo base > "$S/README.txt"
cat > "$S/scripts/anti-mess/sweep.sh" <<'SW'
#!/bin/bash -p
# sweep shim of the test harness: logs its argv, exits SWEEP_RC (default 0); both come from $ANTIMESS_ROOT/.audit/sweep.ctl (the run's environment is an allowlist)
[ -f "${ANTIMESS_ROOT:-/nonexistent}/.audit/sweep.ctl" ] && . "$ANTIMESS_ROOT/.audit/sweep.ctl"
printf '%s\n' "$*" >> "${SWEEP_LOG:-/dev/null}"; exit "${SWEEP_RC:-0}"
SW
chmod 755 "$S/scripts/anti-mess/sweep.sh"
commit_all "$S" seed
# the submodule: a bare repository under fx/ (org `fx` is owned), seeded with one commit
rm -rf "$T/subseed"; mkrepo "$T/subseed"; echo sub > "$T/subseed/s.txt"; commit_all "$T/subseed" "sub init"; git -C "$T/subseed" push -q "$T/fx/sub.git" main
git -C "$S" submodule add -q "$T/fx/sub.git" mods/sub 2>/dev/null
# a third-party submodule (organisation `ext` is not owned): a --repo run of it is refused, and no run pushes it as long as ANY url of ANY of its remotes names a third party (18.15)
mkdir -p "$T/ext"; git init -q --bare -b main "$T/ext/t.git"; rm -rf "$T/extseed"; mkrepo "$T/extseed"; echo t > "$T/extseed/t.txt"; commit_all "$T/extseed" "ext init"; git -C "$T/extseed" push -q "$T/ext/t.git" main
git -C "$S" submodule add -q "$T/ext/t.git" mods/ext 2>/dev/null; git -C "$S" commit -qm "add submodules" 2>/dev/null
git init -q --bare -b main "$T/seed.git" && git -C "$S" push -q "$T/seed.git" main

# ---- per-case construction ---------------------------------------------------------------------------------------------------------------
sha() { sha256sum "$1" | cut -d' ' -f1; }
mk() { # mk <case> [PRE=function applied to the clone before the trust state is built] [OMIT=<tracked path left out of the approved manifest>]
  local n="$1"; C="$T/$n"; chmod -R u+w "$C" 2>/dev/null; rm -rf "$C"; mkdir -p "$C/fx" "$C/state/blobs" "$C/bin"; chmod 700 "$C/state"
  rm -rf "$T/fx/sub.git"; git clone -q --bare "$T/subseed" "$T/fx/sub.git" 2>/dev/null
  git clone -q --bare "$T/seed.git" "$C/fx/a.git"; git clone -q --bare "$T/seed.git" "$C/fx/b.git"
  git clone -q "$C/fx/a.git" "$C/w" 2>/dev/null; W="$C/w"
  git -C "$W" config user.email t@t; git -C "$W" config user.name t; git -C "$W" remote add mirror "$C/fx/b.git"; git -C "$W" fetch -q mirror
  git -C "$W" -c protocol.file.allow=always submodule update -q --init 2>/dev/null
  git -C "$W/mods/sub" checkout -q -B main 2>/dev/null; git -C "$W/mods/sub" config user.email t@t; git -C "$W/mods/sub" config user.name t
  if [ -n "${PRE:-}" ]; then "$PRE"; commit_all "$W" "case setup"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; fi
  trust_build "$C"; cp "$W/scripts/repo/host_entry/cpa-host" "$C/bin/cpa-host" 2>/dev/null; chmod 755 "$C/bin/cpa-host" 2>/dev/null
  : > "$C/git.log"; : > "$C/sweep.log"; PREV=""; SEEDHEAD="$(git -C "$W" rev-parse HEAD)"
  # the run's git settings come from the owner's HOME only (the run drops every caller GIT_CONFIG_* and XDG_CONFIG_HOME, WF14 round 3): the fixture HOME holds what the
  # harness used to export as GIT_CONFIG_COUNT (file-protocol submodules, a container user that does not own the bind-mounted tree)
  shimdir "$C/shimr"
  mkdir -p "$C/home"; printf '[protocol "file"]\n\tallow = always\n[safe]\n\tdirectory = *\n' > "$C/home/.gitconfig"
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
shimdir() { # shimdir <dir> [NAME=VALUE ...]: a directory with a copy of the git shim and its control file ctl.env (read by the shim from beside itself)
  local d="$1" kv; shift; mkdir -p "$d"; cp "$T/shim/git" "$d/git"; chmod 755 "$d/git"
  { printf 'GITSHIM_LOG=%q\n' "$C/git.log"; for kv in "$@"; do printf '%s=%q\n' "${kv%%=*}" "${kv#*=}"; done; } > "$d/ctl.env"
}
# a harness that runs in the background (or under nohup) hands every child SIGINT, SIGQUIT and SIGHUP IGNORED, and an ignored signal cannot be trapped: the run is therefore started through a
# python that resets EVERY catchable disposition to the default before it execs (a false FAIL of section 16 under nohup, TEST-5, and a blind INT case, are both closed this way)
PYBIN="$(command -v python3)"
SIGDFL=("$PYBIN" -I -c 'import os, signal, sys
for n in range(1, 65):
    if n in (signal.SIGKILL, signal.SIGSTOP): continue
    try: signal.signal(n, signal.SIG_DFL)
    except (OSError, ValueError): pass
os.execvp(sys.argv[1], sys.argv[1:])')
envr() { "${SIGDFL[@]}" env "$@"; }
ENVV() { echo "HOME=$C/home" "CPA_HOST_STATE=$C/state" "CPA_HOST_ENTRY=$C/bin/cpa-host" "PATH=${SHIMD:-$C/shimr}:$PATH"; }
ctl_collect() { # ctl_collect: the harness controls offered to this run (ambient exports, a prefix, XENV) as NAME=VALUE lines on stdout
  local n kv; for n in $(compgen -e); do case "$n" in GITSHIM_*|SWEEP_*) printf '%s=%s\n' "$n" "${!n}" ;; esac; done
  for kv in ${XENV:-}; do case "$kv" in GITSHIM_*|SWEEP_*) printf '%s\n' "$kv" ;; esac; done
}
ctl_apply() { # ctl_apply: write the shim control file of the default run shim dir and the sweep control file of the clone from ctl_collect
  local kvs=() kv; while IFS= read -r kv; do [ -z "$kv" ] || kvs+=("$kv"); done < <(ctl_collect)
  shimdir "$C/shimr" "${kvs[@]+"${kvs[@]}"}"
  mkdir -p "$W/.audit"; { printf 'SWEEP_LOG=%q\n' "$C/sweep.log"; for kv in "${kvs[@]+"${kvs[@]}"}"; do case "$kv" in SWEEP_*) printf '%s=%q\n' "${kv%%=*}" "${kv#*=}" ;; esac; done; } > "$W/.audit/sweep.ctl"
}
run() { runenv -- "$@"; }
runenv() { # runenv [NAME=VALUE | -u NAME ...] -- <cpa-host args...>: run with extra environment words (each one argument, so a value may hold spaces); the rest as for `run`
  local -a ev=() uf=(); while [ $# -gt 0 ] && [ "$1" != -- ]; do if [ "$1" = -u ]; then uf+=(-u "$2"); shift 2; else ev+=("$1"); shift; fi; done; [ $# -eq 0 ] || shift
  local -a base=(); local w n u i; for w in $(ENVV) ${XENV:-}; do n="${w%%=*}"; u=0; for ((i=1; i<${#uf[@]}; i+=2)); do [ "${uf[$i]}" = "$n" ] && u=1; done; [ "$u" = 1 ] || base+=("$w"); done
  : > "$C/git.log"; ctl_apply; ( cd "${CWD:-$W}" && envr "${uf[@]+"${uf[@]}"}" "${base[@]}" "${ev[@]+"${ev[@]}"}" "$C/bin/cpa-host" "$@" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
  OUT="$(cat "$C/out.txt" "$C/err.txt")"; RUN="$(printf '%s\n' "$OUT" | sed -n 's/.*cpa: exit=[0-9]* run=\([^ ]*\).*/\1/p' | tail -1)"
  RD="$W/.audit/commit-push/$RUN"; REP="$RD/report.json"; shimcheck
}
forbidden_calls() { # forbidden_calls <git shim log>: the log lines (argv of one git call each) that spell a force, lease, +refspec, deleting or mirroring push, a history rewrite or a hook bypass
  awk '
    function flag(why) { print why ": " $0; hit = 1 }
    { hit = 0; n = split($0, t, " "); pi = 0
      for (i = 1; i <= n; i++) if (t[i] == "push") { pi = i; break }
      for (i = 1; i <= n; i++) {
        if (t[i] ~ /^--force/ || t[i] == "--no-verify" || t[i] == "--amend") { flag("flag"); break }
        if (t[i] == "rebase" || t[i] == "reset" || t[i] == "update-ref") { flag("verb"); break }
      }
      if (hit) next
      if (pi) { opts = 1
        for (i = pi + 1; i <= n; i++) {
          if (t[i] == "--") { opts = 0; continue }
          if (opts && (t[i] ~ /^-[a-zA-Z]*f[a-zA-Z]*$/ || t[i] ~ /^--(mirror|delete|prune)/ || t[i] == "-d")) { flag("push option"); break }
          if (t[i] ~ /^\+/ || t[i] ~ /^:./ || t[i] ~ /:\+/) { flag("push refspec"); break }
        } }
      if (hit) next
      for (i = 1; i <= n; i++) {
        if (t[i] == "checkout") for (j = i + 1; j <= n; j++) if (t[j] ~ /^-[a-zA-Z]*B[a-zA-Z]*$/) { flag("checkout -B"); break }
        if (t[i] == "branch") for (j = i + 1; j <= n; j++) if (t[j] ~ /^-[a-zA-Z]*[fF][a-zA-Z]*$/ || t[j] == "--force") { flag("branch -f"); break }
        if (t[i] == "fetch") for (j = i + 1; j <= n; j++) if (t[j] ~ /^-[a-zA-Z]*f[a-zA-Z]*$/ || t[j] ~ /^\+/) { flag("fetch force"); break }
        if (hit) break
      } }' "$1"
}
shimcheck() { # no force, no lease, no +refspec, no --no-verify, no rebase, no reset, no mirror/delete/:ref push, no update-ref/--amend/branch -f/checkout -B in any git call of the run
  local l; l="$(forbidden_calls "$C/git.log" | head -3)"
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
eq "1.2 a FINISHED run is never listed as interrupted (the run before it ended with its report)" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "[]"
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
# the dead run (no run.pid, older than this run) is CLOSED by this one: its own files are byte-unchanged, interrupted.json and a report.json (closed_by) are ADDED, nothing is deleted
eq "2.16 the interrupted run's own file is byte-unchanged" "$(sha256sum < "$W/.audit/commit-push/20260101T000000Z-1-dead/log.txt")" "$(printf 'partial\n' | sha256sum)"
eq "2.16 the dead run was closed: interrupted.json names this run" "$(jq -r .closed_by "$W/.audit/commit-push/20260101T000000Z-1-dead/interrupted.json" 2>/dev/null)" "$RUN"
eq "2.16 and its report says why" "$(jq -r .reason "$W/.audit/commit-push/20260101T000000Z-1-dead/report.json" 2>/dev/null)" no_report_process_gone
fi

# ======== 3 the lock =========================================================================================================================
if sect "3 lock"; then
mk lock
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; export GITSHIM_LOG="$C/git.log" GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"
: > "$C/git.log"; shimdir "$C/shimh" GITSHIM_PAUSE_AFTER_PUSH='ls-remote|fetch' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" --paths-from "$C/p.txt" "l" ) >"$C/hold.out" 2>"$C/hold.err" &
HP=$!; n=0; while [ ! -e "$C/paused" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done
[ -e "$C/paused" ] && ok "3.1 the first run is paused between S6 and S7" || bad "3.1 no pause reached"
HRUND="$(ls -1d "$W/.audit/commit-push/"*/ | head -1)"
hb="$(cd "$HRUND" && find . -type f | sort | xargs sha256sum | sha256sum)"; [ -n "$HRUND" ] && ok "3.1 the holder's run directory was found" || bad "3.1 no holder run directory"
run; eq "3.2 a second run while the lock is held: 20" "$RC" 20; has "3.2 reason lock_held" "$OUT" lock_held; has "3.2 names the holder's run id" "$OUT" "$(basename "$HRUND")"
eq "3.2 the holder's run directory is byte-unchanged" "$(cd "$HRUND" && find . -type f | sort | xargs sha256sum | sha256sum)" "$hb"
eq "3.2 the second run wrote its own run directory" "$(ls -1d "$W/.audit/commit-push/"*/ | wc -l)" 2
hasnot "3.2 the report does not list the LIVE holder as an interrupted run (F8)" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "$(basename "$HRUND")"
has "3.2 it lists it as a live run" "$(jq -c .live_runs "$REP" 2>/dev/null)" "$(basename "$HRUND")"
: > "$C/go"; wait $HP; hrc=$?; eq "3.3 the holder finishes with its own code: 0" "$hrc" 0
unset GITSHIM_LOG GITSHIM_PAUSED GITSHIM_PAUSE_UNTIL
run; eq "3.4 after the holder ended the lock is free: 0" "$RC" 0
fi

# ======== 4 deferrals and local-only =====================================================================================================
if sect "4 deferral"; then
mk skip
echo s > "$W/src/s.txt"; paths 'src/s.txt\n'; export SKIP_LONG="long gates not run"; run --paths-from "$C/p.txt" "s"; unset SKIP_LONG
moved 4.1; eq "4.1 SKIP_LONG: 14" "$RC" 14; has "4.1 Deferred-Gates SKIP_LONG" "$(msg)" "Deferred-Gates: SKIP_LONG"; eq "4.1 the commit is pushed" "$(tipof a)" "$(head_)"
hasnot "4.1 the detail names no pending check that does not exist (F6)" "$(jq -r .detail "$REP" 2>/dev/null)" check_pending_release; eq "4.1 check_pending_release is false" "$(jq -r .check_pending_release "$REP" 2>/dev/null)" false
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
has "4.9 the commit's Deferred-Gates line carries CHECK_PENDING_RELEASE" "$(msg)" CHECK_PENDING_RELEASE
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
GOJ GO '["20261001T000000Z-1-aaaa"]' 0 > "$W/$EVR/reviews/WP-Y.json"; paths "$EVR/reviews/WP-Y.json\n"; run --paths-from "$C/p.txt" "a GO file for another item"
echo w > "$W/src/w.txt"; paths "src/w.txt\t$EVR/reviews/WP-Y.json\n"; h0="$(head_)"; run --paths-from "$C/p.txt" "hold on a GO file"
eq "6.4 a hold on a verdict that holds GO in HEAD: 20" "$RC" 20; has "6.4 verdict_already_go" "$OUT" verdict_already_go; eq "6.4 nothing committed" "$(head_)" "$h0"; rm -f "$W/src/w.txt"
GOJ GO '["20261001T000000Z-3-cccc"]' 0 > "$W/$EVR/reviews/WP-Y.json"; paths "$EVR/reviews/WP-Y.json\n"; run --paths-from "$C/p.txt" "edit a GO file"
eq "6.4 a change to a verdict file that holds GO in HEAD: 20" "$RC" 20; has "6.4 verdict_already_go" "$OUT" verdict_already_go; git -C "$W" checkout -q -- "$EVR/reviews/WP-Y.json"
echo w2 > "$W/src/w2.txt"; GOJ GO '["20261001T000000Z-4-dddd"]' 0 > "$W/$EVR/reviews/WP-N.json"; paths "src/w2.txt\t$EVR/reviews/WP-N.json\n$EVR/reviews/WP-N.json\n"; run --paths-from "$C/p.txt" "hold with the GO in the same change set"
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
mkdir -p "$W/$EVR/reviews"; GOJ GO '["20261001T000000Z-2-bbbb"]' 0 > "$W/$EVR/reviews/WP-Z.json"; paths "$V2\n"; run --paths-from "$C/p.txt" "go for another run"
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
shimdir "$C/shimh" GITSHIM_PAUSE_ON='(^| )commit( |$)' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" ) >"$C/hold.out" 2>"$C/hold.err" &
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
mkdir -p "$W/$EVR/reviews"; GOJ GO '["20261001T000000Z-2-bbbb"]' 0 > "$W/$EVR/reviews/WP-S.json"; paths "$VR\n"; run --paths-from "$C/p.txt" "a GO that does not cover the held run"
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
( cd "$W" && envr $(ENVV) "$W/scripts/commit-push-all.sh" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
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
( cd "$W" && envr $(ENVV) CPA_ROOT="$W" CPA_RUN_ID="$(basename "$RD5")" "$RD5/released/scripts/commit-push-all.sh" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
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
eq "13.5 the REPORT states the reason the helper's REFUSED line stated (it was null: the grep list did not hold it)" "$(jq -r .reason "$REP" 2>/dev/null)" unrecorded_local_commit
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

# ======== 14 the exit-code contract (WF11 review of the launcher slice): a deferred or unbuilt part is never a success ============================
# Defect classes (11.4.276 (C)), each closed for EVERY member: C3 a gate that did not run reads as exit 0 (a `deferred` registry row, an unbuilt secret fold,
# private-key fold, path gate, G-PIN, remote check, container S3, an absent owed-gate list); C2 a helper status outside its documented set reads as success;
# C4 report fields that state what did not happen; C5 a caller-environment variable that widens what a run may do; C6 a stale lock reported as a live one;
# C7 three run-id grammars; C8 a host-entry exit outside its documented set.
if sect "14 contract"; then
realowed() { cp "$SRC/scripts/repo/owed_gates.tsv" "$W/scripts/repo/owed_gates.tsv"; }
badowed() { printf 'X_GATE\tsometimes\tnot a condition\n' >> "$W/scripts/repo/owed_gates.tsv"; }
defrows() { printf 'go_vet\t-\tIMG-GO\tdeferred\t-\tfiles\tremote measurement\nanti_bluff\t-\tIMG-TESTUTIL\tdeferred\t-\tfiles\tnot built\n' >> "$W/scripts/repo/validate_checks.tsv"; }
# control needle of the real list: it names the unbuilt folds the reviewer found
grep -q '^SECRET_FOLD	' "$SRC/scripts/repo/owed_gates.tsv" 2>/dev/null && grep -q '^PRIVATE_KEY_FOLD	' "$SRC/scripts/repo/owed_gates.tsv" 2>/dev/null && ok "14.0 the real owed-gate list names SECRET_FOLD and PRIVATE_KEY_FOLD" || bad "14.0 the real owed-gate list is absent or lacks the secret/private-key folds"

PRE=realowed mk owed1
printf -- '-----BEGIN OPENSSH PRIVATE KEY-----\nNOTAREALKEYNOTAREALKEYNOTAREALKEY\n-----END OPENSSH PRIVATE KEY-----\n' > "$W/src/id_fake"; paths 'src/id_fake\n'; run --paths-from "$C/p.txt" "key-shaped file"
eq "14.1 unbuilt secret and private-key folds and a key-shaped file declared: 14, never 0" "$RC" 14; moved 14.1
has "14.1 the report names GATES_NOT_BUILT" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" GATES_NOT_BUILT
has "14.1 the deferral row names the private-key fold" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" PRIVATE_KEY_FOLD; has "14.1 and the secret fold" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" SECRET_FOLD
has "14.1 the commit carries the flag in its Deferred-Gates line" "$(msg)" "Deferred-Gates: GATES_NOT_BUILT"; eq "14.1 the report reason" "$(jq -r .reason "$REP" 2>/dev/null)" deferral_recorded
eq "14.1 the loud deferral still pushed (the mechanism is never blocked, 11.4.234 (D))" "$(tipof a)" "$(head_)"
PRE=realowed mk owed2; run
eq "14.2 the real list, nothing declared: the always-owed gates still give 14" "$RC" 14; has "14.2 REMOTE_CHECKS owed" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" REMOTE_CHECKS; hasnot "14.2 the folds of a declared change set are not owed when nothing is declared" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" SECRET_FOLD
OMIT=scripts/repo/owed_gates.tsv mk owed3; echo o > "$W/src/o.txt"; paths 'src/o.txt\n'; run --paths-from "$C/p.txt" "o"
eq "14.3 an approved copy without the owed-gate list: 14 (the unbuilt set cannot be read), never 0" "$RC" 14; has "14.3 the reason says the list is absent" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" "owed_gates.tsv absent"
PRE=badowed mk owed4; run; eq "14.4 an owed-gate row with a condition outside {always, declared_paths}: 20" "$RC" 20; has "14.4 owed_gates_invalid" "$OUT" owed_gates_invalid
PRE=defrows mk defrow; echo d > "$W/src/d.txt"; paths 'src/d.txt\n'; run --paths-from "$C/p.txt" "d"
eq "14.5 deferred registry rows (go_vet, anti_bluff) in the approved registry: 14, never 0" "$RC" 14; moved 14.5
has "14.5 the report names CHECKS_DEFERRED" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECKS_DEFERRED; has "14.5 the row names go_vet" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" go_vet; has "14.5 and anti_bluff" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" anti_bluff
has "14.5 the commit carries Deferred-Gates: CHECKS_DEFERRED" "$(msg)" "Deferred-Gates: CHECKS_DEFERRED"

OMIT=scripts/repo/long_gates.txt mk nolong; echo n > "$W/src/n.txt"; paths 'src/n.txt\n'; run --paths-from "$C/p.txt" "n"
eq "14.33 an approved copy without a long-gate list (no long gate was run): 14, never 0" "$RC" 14; has "14.33 GATES_NOT_BUILT names the long gates" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" "long_gates.txt is absent"

# ---- C6 a stale lock is named as one, with its remediation (F2) ----
mk stale; echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; export GITSHIM_LOG="$C/git.log" GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"
: > "$C/git.log"; shimdir "$C/shimh" GITSHIM_PAUSE_AFTER_PUSH='ls-remote|fetch' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" --paths-from "$C/p.txt" "l" ) >"$C/hold.out" 2>"$C/hold.err" &
HP=$!; n=0; while [ ! -e "$C/paused" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done
[ -e "$C/paused" ] && ok "14.6 the first run is paused between S6 and S7" || bad "14.6 no pause reached"
hpid="$(jq -r .pid "$W/.audit/longops/claims/commit_push/holder.json" 2>/dev/null)"
if [[ "$hpid" =~ ^[0-9]+$ ]] && [ "$hpid" -gt 1 ] && tr '\0' ' ' < "/proc/$hpid/cmdline" 2>/dev/null | grep -q 'commit-push-all.sh'; then kill -KILL "$hpid"; ok "14.6 the holder (a process of our released commit-push-all.sh) was killed"; else bad "14.6 the holder pid '$hpid' is not our released script"; fi
: > "$C/go"; wait $HP 2>/dev/null; unset GITSHIM_LOG GITSHIM_PAUSED GITSHIM_PAUSE_UNTIL
dead6="$(basename "$(ls -1d "$W/.audit/commit-push/"*/ | head -1)")"
run; eq "14.6 a dead holder's claim is reaped by S0 itself (reap.sh --purpose commit_push proves the holder dead) and the run proceeds: 0" "$RC" 0
has "14.6 reap.out records the released stale claim" "$(cat "$RD/reap.out" 2>/dev/null)" "released stale claim commit_push"
eq "14.6 the reaped holder's own run is closed by this one (its process is gone, no report)" "$(jq -r .closed_by "$W/.audit/commit-push/$dead6/report.json" 2>/dev/null)" "$RUN"

# ---- C2 every helper status outside its documented set is 20 (F5) ----
mshim() { hshim scripts/repo/integrate_ff_only.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; echo "{\"reason\":\"remotes_diverged\"}" > "$j"; exit 12'; hshim scripts/repo/integrate_merge.sh "echo '$2'; exit $1"; }
mk mg13; mshim 13 ''; run; eq "14.7 integrate_merge 13: 13 secret_fold_refused" "$RC" 13; has "14.7 reason" "$OUT" secret_fold_refused
mk mg7; mshim 7 ''; run; eq "14.8 integrate_merge exit 7 (outside its set): 20, never read as success" "$RC" 20; has "14.8 internal_error" "$OUT" internal_error
mk mg10; mshim 10 ''; run; eq "14.9 integrate_merge 10: 10" "$RC" 10; has "14.9 reason" "$OUT" resolved_file_holds_marker
mk mg11; mshim 11 ''; run; eq "14.10 integrate_merge 11: 11" "$RC" 11; has "14.10 reason" "$OUT" remote_unreachable
mk mg20; mshim 20 'resolution_invalid'; run; eq "14.11 integrate_merge 20: 20" "$RC" 20; has "14.11 reason" "$OUT" resolution_invalid
vcs() { hshim scripts/repo/validate_cheap.sh "$2"; echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; }
mk vc20; vcs 20 'exit 20'; run --paths-from "$C/p.txt" "a"; eq "14.12 validate_cheap 20: 20 check_error" "$RC" 20; has "14.12 check_error" "$OUT" check_error; eq "14.12 nothing committed or pushed" "$(pushes)" 0
mk vc7; vcs 7 'exit 7'; run --paths-from "$C/p.txt" "a"; eq "14.13 validate_cheap exit 7 (outside its set): 20, never read as success" "$RC" 20; has "14.13 internal_error" "$OUT" internal_error; eq "14.13 no push call" "$(pushes)" 0
mk vc10; vcs 10 'echo "fail shim_check src/a.txt"; exit 10'; run --paths-from "$C/p.txt" "a"; eq "14.14 validate_cheap 10: 10" "$RC" 10; has "14.14 check_failed" "$OUT" check_failed
mk vc14; vcs 14 'echo "check_pending_release extra"; exit 14'; run --paths-from "$C/p.txt" "a"; eq "14.15 validate_cheap 14: 14, never 0" "$RC" 14; eq "14.15 the report says check_pending_release" "$(jq -r .check_pending_release "$REP" 2>/dev/null)" true
mk vr7; hshim scripts/repo/verify_repos.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; echo "{\"summary\":{\"pin_drift\":0,\"ahead\":0,\"dirty\":0},\"repos\":[]}" > "$j"; exit 7'; run
eq "14.16 a verifier exit 7 with a readable report (outside its set): 20, never clean" "$RC" 20; has "14.16 internal_error" "$OUT" internal_error
mk aq7; hshim scripts/longops/acquire.sh 'echo boom >&2; exit 7'; run; eq "14.17 acquire.sh exit 7 (outside its set): 20, the run does not proceed unlocked" "$RC" 20; has "14.17 lock_error" "$OUT" lock_error; eq "14.17 no push call" "$(pushes)" 0
mk aq3; hshim scripts/longops/acquire.sh 'echo "purpose_conflict: commit_push held (live): {}" >&2; exit 3'; run; eq "14.18 acquire.sh 3: 20 lock_held" "$RC" 20; eq "14.18 reason" "$(jq -r .reason "$REP" 2>/dev/null)" lock_held; has "14.18 the message carries acquire.sh's own text" "$OUT" "purpose_conflict"
mk aq4; hshim scripts/longops/acquire.sh 'echo "stale_claim: commit_push holder is dead (resolved from /proc); reap with scripts/longops/reap.sh --purpose commit_push" >&2; exit 4'; run; eq "14.19 acquire.sh 4 with a stale_claim text: lock_stale_claim" "$(jq -r .reason "$REP" 2>/dev/null)" lock_stale_claim; has "14.19 the reap hint" "$OUT" "reap.sh --purpose commit_push"

# ---- C2 a signal mid-run: the catch-all of S8 is 20, never the signal's status read as success (F5) ----
mk term; echo t > "$W/src/t.txt"; paths 'src/t.txt\n'; export GITSHIM_LOG="$C/git.log" GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"
: > "$C/git.log"; shimdir "$C/shimh" GITSHIM_PAUSE_AFTER_PUSH='ls-remote|fetch' GITSHIM_PAUSED="$C/paused" GITSHIM_PAUSE_UNTIL="$C/go"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" --run-id term-1 --paths-from "$C/p.txt" "t" ) >"$C/hold.out" 2>"$C/hold.err" &
HP=$!; n=0; while [ ! -e "$C/paused" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done
tpid="$(jq -r .pid "$W/.audit/longops/claims/commit_push/holder.json" 2>/dev/null)"
if [[ "$tpid" =~ ^[0-9]+$ ]] && [ "$tpid" -gt 1 ] && tr '\0' ' ' < "/proc/$tpid/cmdline" 2>/dev/null | grep -q 'commit-push-all.sh'; then kill -TERM "$tpid"; ok "14.20 SIGTERM sent to our released commit-push-all.sh"; else bad "14.20 the run pid '$tpid' is not our released script"; fi
: > "$C/go"; wait $HP; hrc=$?; unset GITSHIM_LOG GITSHIM_PAUSED GITSHIM_PAUSE_UNTIL
eq "14.20 a SIGTERM mid-run: 20, not 143 and never 0" "$hrc" 20
TR="$W/.audit/commit-push/term-1/report.json"; eq "14.20 the report exit" "$(jq -r .exit "$TR" 2>/dev/null)" 20; eq "14.20 reason internal_error" "$(jq -r .reason "$TR" 2>/dev/null)" internal_error; has "14.20 the detail names the signal status" "$(jq -r .detail "$TR" 2>/dev/null)" "exit 143"

# ---- C8/self-test: a released copy changed by one byte beside the genuine snapshot is refused (F5, the own-sha256 check) ----
mk tamper; run; RDT="$RD"; chmod u+w "$RDT/released/scripts/commit-push-all.sh"; echo '# tampered' >> "$RDT/released/scripts/commit-push-all.sh"
( cd "$W" && envr $(ENVV) CPA_ROOT="$W" CPA_RUN_ID="$(basename "$RDT")" "$RDT/released/scripts/commit-push-all.sh" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
eq "14.21 a released copy changed by one byte: 20" "$RC" 20; has "14.21 own sha256 differs from the snapshot entry" "$(cat "$C/err.txt")" "own sha256 differs from the snapshot entry"

# ---- C5 no caller-environment variable widens the owned set (F1) ----
mk own; git -C "$W/mods/ext" checkout -q -B main 2>/dev/null; git -C "$W/mods/ext" config user.email t@t; git -C "$W/mods/ext" config user.name t
echo x > "$W/mods/ext/x.txt"; printf 'mods/ext/x.txt\n' > "$C/p.txt"; e0="$(git -C "$T/ext/t.git" rev-parse main)"
run --repo mods/ext --paths-from "$C/p.txt" "third party"; eq "14.22 control: a --repo run of a third-party submodule: 20" "$RC" 20; has "14.22 repo_not_owned" "$OUT" repo_not_owned
XENV="CPA_OWNED_ORGS=fx,ext" run --repo mods/ext --paths-from "$C/p.txt" "third party"; unset XENV
eq "14.23 CPA_OWNED_ORGS in the caller's environment widens nothing: 20" "$RC" 20; has "14.23 repo_not_owned" "$OUT" repo_not_owned
eq "14.23 the third-party remote did not move" "$(git -C "$T/ext/t.git" rev-parse main)" "$e0"

# ---- C4 report fields state only what happened (F6, F7, F8) ----
mk rep; run --bogus; eq "14.24 a usage_error run never took the lock: lock.held false" "$(jq -r .lock.held "$REP" 2>/dev/null)" false
run; eq "14.25 a clean run held the lock" "$(jq -r .lock.held "$REP" 2>/dev/null)" true; eq "14.25 and released it" "$(jq -r .lock.released "$REP" 2>/dev/null)" true
mk pend; printf 'extra\tscripts/repo/check_no_ci.sh --root /nonexistent\tIMG-TESTUTIL\tplain\t-\tchangeset\tnot in the approved copy\n' >> "$W/scripts/repo/validate_checks.tsv"; git -C "$W" add -A; git -C "$W" commit -qm "registry row"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main
echo p > "$W/src/p.txt"; paths 'src/p.txt\n'; run --paths-from "$C/p.txt" "p"
eq "14.26 a pending check: 14" "$RC" 14; has "14.26 CHECK_PENDING_RELEASE in the commit's Deferred-Gates line" "$(msg)" CHECK_PENDING_RELEASE; has "14.26 the report says so" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECK_PENDING_RELEASE

# ---- C8 the host entry's exit set is closed (F9), C7 one run-id grammar (F10), C5 no inherited git state (F11), file arguments (F12) ----
mk noexec; jq '.projects|=with_entries(.value.manifest|=map(if .path=="scripts/commit-push-all.sh" then .exec=false else . end))' "$C/state/trust.json" > "$C/state/t2" && mv "$C/state/t2" "$C/state/trust.json"
run; eq "14.27 a core the manifest marks non-executable: 20, never the shell's 126" "$RC" 20; has "14.27 approved_copy_invalid" "$OUT" approved_copy_invalid; eq "14.27 no run directory is left" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)" 0
mk rid
run --run-id _lead; eq "14.28 a run id with a leading underscore: 20 at the host entry" "$RC" 20; has "14.28 run_id_malformed" "$OUT" run_id_malformed
long70="$(printf 'a%.0s' $(seq 1 70))"; run --run-id "$long70"; eq "14.29 a 70-character run id: 20 at the host entry, before any stage" "$RC" 20; has "14.29 run_id_malformed" "$OUT" run_id_malformed
eq "14.29 no run directory exists" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l)" 0
run --run-id Ok_id-1; eq "14.30 a run id every grammar accepts: 0" "$RC" 0
mk hookenv; mkdir -p "$C/chooks"; for h in pre-commit commit-msg post-commit pre-push post-merge reference-transaction pre-merge-commit; do printf '#!/bin/sh\necho %s >> "%s"\nexit 0\n' "$h" "$C/hook.marker" > "$C/chooks/$h"; chmod 755 "$C/chooks/$h"; done
rm -rf "$T/henv"; mkrepo "$T/henv"; echo x > "$T/henv/x"; git -C "$T/henv" add x; ( cd "$T/henv" && GIT_CONFIG_PARAMETERS="'core.hookspath=$C/chooks'" git commit -qm x ) >/dev/null 2>&1
[ -s "$C/hook.marker" ] && ok "14.31 control needle: the inherited GIT_CONFIG_PARAMETERS hook path fires in an ordinary commit" || bad "14.31 the inherited setting does not fire a hook here: the check below would be blind"
rm -f "$C/hook.marker"; echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; XENV="GIT_CONFIG_PARAMETERS='core.hookspath=$C/chooks'" run --paths-from "$C/p.txt" "l"; unset XENV
eq "14.31 a run under an inherited GIT_CONFIG_PARAMETERS hook path: 0" "$RC" 0; [ ! -e "$C/hook.marker" ] && ok "14.31 no inherited hook ran" || bad "14.31 inherited hooks ran: $(sort -u "$C/hook.marker" | tr '\n' ' ')"
mk relp; mkdir -p "$W/.audit/cwd"; echo a > "$W/src/a.txt"; printf 'src/a.txt\n' > "$W/.audit/list.txt"; printf 'src/NOPE\n' > "$C/list.txt"
CWD="$W/.audit/cwd" run --paths-from ../list.txt "rel"; unset CWD
eq "14.32 a relative --paths-from is read from the directory cpa-host was started in (the decoy beside the root is not): 0" "$RC" 0; moved 14.32; has "14.32 the commit holds a.txt" "$(git -C "$W" show --stat --format= HEAD)" "src/a.txt"
fi

# ======== 15 the caller's environment (WF14 round 3: the CLASS, reviewer N1/N3 = round-1 F1/F11/C5) =======================================
# A run is steered by what the OWNER approved, never by the caller's environment. Each hostile variable below is first proven to steer git (or run a
# program) OUTSIDE a run (a control needle, 11.4.201(7)(b)), then the same variable is given to a run.
if sect "15 env"; then
fmark() { if [ -s "$C/$1" ]; then wc -l < "$C/$1" | tr -d ' '; else echo 0; fi; }
evilclone() { # a bare repository that is in no approved list, and a work clone with one extra commit (the push-redirection needle)
  git clone -q --bare "$T/seed.git" "$C/evil.git"; e0="$(git -C "$C/evil.git" rev-parse main)"
  rm -rf "$C/pc"; git clone -q "$C/fx/a.git" "$C/pc" 2>/dev/null; echo pc > "$C/pc/pc.txt"; git -C "$C/pc" add pc.txt; git -C "$C/pc" commit -qm pc
}
fnotreached() { # fnotreached <label>: the evil remote holds exactly what it held before the run
  local e1; e1="$(git -C "$C/evil.git" rev-parse main)"
  if [ "$e1" = "$e0" ]; then ok "$1: the non-owned remote did not receive the run's commit"; else bad "$1: the run pushed to a remote that only the caller's environment named (evil main $e0 -> $e1)"; fi
}
# -- PA: GIT_CONFIG_COUNT/KEY_n/VALUE_n (an env-config pushurl) ----
mk pa; evilclone
GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_2=remote.origin.pushurl GIT_CONFIG_VALUE_2="$C/evil.git" git -C "$C/pc" push -q origin HEAD:refs/heads/pcbranch 2>/dev/null
git -C "$C/evil.git" rev-parse -q --verify refs/heads/pcbranch >/dev/null && ok "15.1 control needle: an env-config pushurl redirects a plain push on this git" || bad "15.1 control: the env pushurl did not redirect (probe blind)"
git -C "$C/fx/a.git" rev-parse -q --verify refs/heads/pcbranch >/dev/null && bad "15.1 control: origin got pcbranch too" || ok "15.1 control: origin did not get pcbranch"
echo x > "$W/src/x.txt"; paths 'src/x.txt\n'
XENV="GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_2=remote.origin.pushurl GIT_CONFIG_VALUE_2=$C/evil.git" run --paths-from "$C/p.txt" "x"; unset XENV
eq "15.1 a run under a caller's env-config pushurl: 0" "$RC" 0; fnotreached 15.1; eq "15.1 origin holds the run's commit" "$(tipof a)" "$(head_)"; eq "15.1 mirror holds it" "$(tipof b)" "$(head_)"
# -- PB: an env-config core.fsmonitor runs a caller program inside the run's git calls ----
mk pb; printf '#!/bin/sh\necho ran >> "%s"\nexit 1\n' "$C/fsm.marker" > "$C/fsm.sh"; chmod 755 "$C/fsm.sh"
rm -rf "$C/pbn"; mkrepo "$C/pbn"; echo y > "$C/pbn/y"; git -C "$C/pbn" add y; git -C "$C/pbn" commit -qm y
GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_2=core.fsmonitor GIT_CONFIG_VALUE_2="$C/fsm.sh" git -C "$C/pbn" status --porcelain >/dev/null 2>&1
[ -s "$C/fsm.marker" ] && ok "15.2 control needle: an env-config core.fsmonitor runs a program in git status" || bad "15.2 control: no program ran (probe blind)"
rm -f "$C/fsm.marker"; echo l > "$W/src/l.txt"; paths 'src/l.txt\n'
XENV="GIT_CONFIG_COUNT=3 GIT_CONFIG_KEY_2=core.fsmonitor GIT_CONFIG_VALUE_2=$C/fsm.sh" run --paths-from "$C/p.txt" "l"; unset XENV
eq "15.2 a run under a caller's env-config core.fsmonitor: 0" "$RC" 0; eq "15.2 no caller program ran inside the run's git calls" "$(fmark fsm.marker)" 0
# -- PG: GIT_CONFIG_GLOBAL / GIT_CONFIG_SYSTEM naming a caller's file ----
mk pg; evilclone; printf '[remote "origin"]\n\tpushurl = %s\n' "$C/evil.git" > "$C/g.cfg"
GIT_CONFIG_GLOBAL="$C/g.cfg" git -C "$C/pc" push -q origin HEAD:refs/heads/pcbranch 2>/dev/null
git -C "$C/evil.git" rev-parse -q --verify refs/heads/pcbranch >/dev/null && ok "15.3 control needle: a GIT_CONFIG_GLOBAL file with a pushurl redirects a plain push" || bad "15.3 control: GIT_CONFIG_GLOBAL did not redirect (probe blind)"
GIT_CONFIG_SYSTEM="$C/g.cfg" git -C "$C/pc" push -q origin HEAD:refs/heads/pcsys 2>/dev/null
git -C "$C/evil.git" rev-parse -q --verify refs/heads/pcsys >/dev/null && ok "15.3 control needle: a GIT_CONFIG_SYSTEM file redirects a plain push" || bad "15.3 control: GIT_CONFIG_SYSTEM did not redirect"
echo g > "$W/src/g.txt"; paths 'src/g.txt\n'
XENV="GIT_CONFIG_GLOBAL=$C/g.cfg GIT_CONFIG_SYSTEM=$C/g.cfg" run --paths-from "$C/p.txt" "g"; unset XENV
eq "15.3 a run under caller GIT_CONFIG_GLOBAL and GIT_CONFIG_SYSTEM files: 0" "$RC" 0; fnotreached 15.3; eq "15.3 origin holds the run's commit" "$(tipof a)" "$(head_)"
# -- PX: XDG_CONFIG_HOME (git reads $XDG_CONFIG_HOME/git/config) ----
mk px; evilclone; mkdir -p "$C/xdg/git" "$C/xhome"; printf '[remote "origin"]\n\tpushurl = %s\n' "$C/evil.git" > "$C/xdg/git/config"
env -u GIT_CONFIG_GLOBAL -u GIT_CONFIG_SYSTEM -u GIT_CONFIG_COUNT HOME="$C/xhome" XDG_CONFIG_HOME="$C/xdg" git -C "$C/pc" push -q origin HEAD:refs/heads/pcbranch 2>/dev/null
git -C "$C/evil.git" rev-parse -q --verify refs/heads/pcbranch >/dev/null && ok "15.4 control needle: an XDG_CONFIG_HOME config with a pushurl redirects a plain push" || bad "15.4 control: XDG_CONFIG_HOME did not redirect (probe blind)"
echo xd > "$W/src/xd.txt"; paths 'src/xd.txt\n'
XENV="XDG_CONFIG_HOME=$C/xdg" run --paths-from "$C/p.txt" "xd"; unset XENV
eq "15.4 a run under a caller XDG_CONFIG_HOME: 0" "$RC" 0; fnotreached 15.4
# -- PBE: BASH_ENV (every non-interactive bash sources it) ----
mk pbe; printf 'echo ran >> "%s"\n' "$C/be.marker" > "$C/be.sh"; BASH_ENV="$C/be.sh" bash -c true
eq "15.5 control needle: BASH_ENV runs a caller file in a plain non-interactive bash" "$(fmark be.marker)" 1
rm -f "$C/be.marker"; echo b > "$W/src/b.txt"; paths 'src/b.txt\n'
XENV="BASH_ENV=$C/be.sh" run --paths-from "$C/p.txt" "b"; unset XENV
eq "15.5 a run under a caller BASH_ENV: 0" "$RC" 0
eq "15.5 the caller file ran NOWHERE: #!/bin/bash -p ignores BASH_ENV in the entry process, the run and every helper (INSTALL.md 40 is true now)" "$(fmark be.marker)" 0
# -- PV: a caller-chosen program location read by a helper (VERIFY_GIT is the verifier's git binary) ----
mk pv; printf '#!/bin/sh\necho ran >> "%s"\nexec git "$@"\n' "$C/vg.marker" > "$C/vg.sh"; chmod 755 "$C/vg.sh"
VERIFY_GIT="$C/vg.sh" "$SRC/scripts/repo/verify_repos.sh" --root "$W" --no-remote --quiet --json "$C/vg.json" >/dev/null 2>&1
[ -s "$C/vg.marker" ] && ok "15.6 control needle: VERIFY_GIT runs a caller program in the verifier" || bad "15.6 control: VERIFY_GIT did not run the program (probe blind)"
rm -f "$C/vg.marker"; XENV="VERIFY_GIT=$C/vg.sh" run; unset XENV
eq "15.6 a run under a caller VERIFY_GIT: 0" "$RC" 0; eq "15.6 the caller program ran nowhere in the run" "$(fmark vg.marker)" 0
# -- PL: the longops / anti-mess families (a forged clock, a caller's directories) ----
mk pl; mkdir -p "$C/forged"; XENV="LONGOPS_DIR=$C/forged LONGOPS_NOW=1 ANTIMESS_ROOT=$C/forged ANTIMESS_CATALOGUE=$C/forged/none LONGOPS_PODMAN=$C/forged/podman" run; unset XENV
eq "15.7 a run under caller LONGOPS_*/ANTIMESS_* variables: 0" "$RC" 0; eq "15.7 nothing was written to the caller's directory" "$(ls "$C/forged" | wc -l | tr -d ' ')" 0
# -- the commit identity is one of the two kept names ----
mk pid; echo i > "$W/src/i.txt"; paths 'src/i.txt\n'
XENV="GIT_AUTHOR_NAME=Owner GIT_AUTHOR_EMAIL=owner@example.test GIT_COMMITTER_NAME=Owner GIT_COMMITTER_EMAIL=owner@example.test" run --paths-from "$C/p.txt" "ident"; unset XENV
eq "15.8 a caller commit identity is kept: 0" "$RC" 0; eq "15.8 the commit carries it" "$(git -C "$W" log -1 --format=%an)" Owner
fi

# ======== 16 every catchable terminating signal (WF14 round 3: the CLASS, reviewer N2 = round-1 F5/RM1) =======================================
# The oracle is signal(7): the signals whose default action terminates the process. A run that is hit by one reports exit 20 (internal_error, the status in the
# detail), never the status of the last completed command, and its process status stays inside the documented set.
if sect "16 sig"; then
# control needle: bash alone runs the EXIT trap with the status of the last completed command and re-raises the signal (the defect this section guards)
cn="$("${SIGDFL[@]}" bash -c 'trap "echo trapped-rc=\$?" EXIT; kill -HUP $$' 2>&1; echo "status=$?")"
has "16.0 control needle: an untrapped SIGHUP runs the EXIT trap with status 0 (the old report said exit 0)" "$cn" "trapped-rc=0"; has "16.0 and the process status is 129" "$cn" "status=129"
mk sg
hshim scripts/repo/validate_cheap.sh 'read -r s < "$CPA_ROOT/.audit/sig.txt"; p=$PPID
if [[ "$p" =~ ^[0-9]+$ ]] && [ "$p" -gt 1 ] && tr "\0" " " < "/proc/$p/cmdline" 2>/dev/null | grep -q "released/scripts/commit-push-all.sh"; then kill -"$s" "$p"; exit 0; fi
echo "shim: the parent is not our released commit-push-all.sh" >&2; exit 7'
mkdir -p "$W/.audit"
sigrun() { # sigrun <name or number> <expected process status>: one run hit by the signal at S3; 0 = a signal that does not terminate
  local s="$1" want="$2" st; printf '%s\n' "$s" > "$W/.audit/sig.txt"; run; st="$RC"
  if [ "$want" = 0 ]; then eq "16 SIG$s (does not terminate): the run completes 0" "$st" 0; return; fi
  eq "16 SIG$s: the process status is 20, never $want and never 0" "$st" 20; eq "16 SIG$s: the report exit" "$(jq -r .exit "$REP" 2>/dev/null)" 20
  eq "16 SIG$s: the report reason" "$(jq -r .reason "$REP" 2>/dev/null)" internal_error; eq "16 SIG$s: the report detail names the status" "$(jq -r .detail "$REP" 2>/dev/null)" "exit $want"
  eq "16 SIG$s: the failed stage is S3" "$(jq -r .failed_stage "$REP" 2>/dev/null)" S3; eq "16 SIG$s: the lock was released" "$(jq -r .lock.released "$REP" 2>/dev/null)" true
  eq "16 SIG$s: no push call" "$(pushes)" 0
}
sigrun WINCH 0
sigrun INT 130      # the run starts through envr, which resets every disposition: SIGINT is testable in a background harness too (it was a bare ok there)
for pair in HUP:129 QUIT:131 ILL:132 TRAP:133 ABRT:134 BUS:135 FPE:136 USR1:138 SEGV:139 USR2:140 PIPE:141 ALRM:142 TERM:143 STKFLT:144 XCPU:152 XFSZ:153 VTALRM:154 PROF:155 IO:157 PWR:158 SYS:159; do
  sigrun "${pair%%:*}" "${pair##*:}"
done
for rt in 34:162 40:168 50:178 64:192; do sigrun "${rt%%:*}" "${rt##*:}"; done      # the real-time range (a mutant that traps only one number of it is killed)
printf "WINCH\n" > "$W/.audit/sig.txt"; run; eq "16.2 after the signalled runs a plain run is 0 (no lock left behind)" "$RC" 0
# a second signal while the report is being written (S8) does not cut it short: release.sh runs inside the exit handler; every catchable terminating signal is tried
for s3 in TERM HUP INT QUIT; do
  mk sg3; hshim scripts/longops/release.sh 'p=$PPID; if [[ "$p" =~ ^[0-9]+$ ]] && [ "$p" -gt 1 ] && tr "\0" " " < "/proc/$p/cmdline" 2>/dev/null | grep -q "released/scripts/commit-push-all.sh"; then kill -'"$s3"' "$p"; fi; exit 0'
  run; eq "16.3 a SIG$s3 that arrives while the report is written: the run still ends 0 (the report is complete, the signal ignored)" "$RC" 0
  eq "16.3 SIG$s3: the report exists and says exit 0" "$(jq -r .exit "$REP" 2>/dev/null)" 0; eq "16.3 SIG$s3: the lock was released" "$(jq -r .lock.released "$REP" 2>/dev/null)" true
done
# a signal before the run is opened (the self-test phase of the copied script): 20 with a stated cause, never the signal's status; HUP, QUIT, TERM and USR1 are tried
for pair in HUP:129 QUIT:131 TERM:143 USR1:138; do
  sg="${pair%%:*}"; sn="${pair##*:}"; mk sg4; : > "$C/git.log"; rm -f "$C/signaled"
  shimdir "$C/shimh" GITSHIM_SIGNAL_ON='rev-list --first-parent --max-parents=0' GITSHIM_SIGNAL=$sg GITSHIM_SIGNAL_TARGET='released/scripts/commit-push-all.sh' GITSHIM_SIGNALED="$C/signaled"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
  [ -e "$C/signaled" ] && ok "16.4 control: the SIG$sg reached our released commit-push-all.sh" || bad "16.4 control: no signal was sent (the probe is blind)"
  eq "16.4 a SIG$sg during the self-test: 20, never $sn" "$RC" 20; has "16.4 SIG$sg: the refusal states the status" "$(cat "$C/err.txt")" "internal_error exit $sn"
  d4="$(ls -1d "$W"/.audit/commit-push/*/ 2>/dev/null | head -1)"; eq "16.4 SIG$sg: the run directory holds a report (reason internal_error)" "$(jq -r .reason "${d4}report.json" 2>/dev/null)" internal_error
done
# a signal in the host entry before it execs: 20 as one JSON line, never the signal's status
for pair in HUP:129 QUIT:131 TERM:143 USR1:138; do
  sg="${pair%%:*}"; sn="${pair##*:}"; mk sg5; rm -f "$C/signaled"
  shimdir "$C/shimh" GITSHIM_SIGNAL_ON='rev-parse --show-toplevel' GITSHIM_SIGNAL=$sg GITSHIM_SIGNAL_TARGET='bin/cpa-host' GITSHIM_SIGNALED="$C/signaled"; ( cd "$W" && SHIMD="$C/shimh" && envr $(ENVV) "$C/bin/cpa-host" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?
  [ -e "$C/signaled" ] && ok "16.5 control: the SIG$sg reached the host entry" || bad "16.5 control: no signal was sent (the probe is blind)"
  eq "16.5 a SIG$sg in the host entry: 20, never $sn" "$RC" 20; has "16.5 one refusal line with the status" "$(cat "$C/err.txt")" "host_exit_$sn"
done
fi

# ======== 17 round-2 review probes adopted verbatim (WF14 N4, N5, N7, N9, N10, N3) =======================================================
if sect "17 r2"; then
realowed() { cp "$SRC/scripts/repo/owed_gates.tsv" "$W/scripts/repo/owed_gates.tsv"; }
# -- N10: the new code of round 1 had no killing assertion (RN1, RN2, RN4, RN5) ----
mk pk1; hshim scripts/longops/release.sh 'echo release-refused >&2; exit 1'; run
eq "17.1 a release.sh that fails: lock.released is false, the report never claims a release that did not happen" "$(jq -r .lock.released "$REP" 2>/dev/null)" false
eq "17.1 and the run is 20 lock_release_failed, never a 0 that left the claim behind" "$RC:$(jq -r .reason "$REP" 2>/dev/null)" "20:lock_release_failed"
run; eq "17.1 the next run reaps the leaked claim itself (S0, reap.sh proves the holder dead): the run proceeds, no lock_stale_claim" "$(jq -r '.reason // "none"' "$REP" 2>/dev/null)" lock_release_failed
mk pk2; l65="$(printf 'a%.0s' $(seq 1 65))"; run --run-id "$l65"
eq "17.2 a 65-character run id: 20 at the host entry" "$RC" 20; has "17.2 run_id_malformed" "$OUT" run_id_malformed; eq "17.2 no run directory" "$(ls "$W/.audit/commit-push" 2>/dev/null | wc -l | tr -d ' ')" 0
l64="$(printf 'a%.0s' $(seq 1 64))"; run --run-id "$l64"; eq "17.2 a 64-character run id (the boundary) is accepted: 0" "$RC" 0
mk pk4; mkdir -p "$W/.audit/commit-push/20260101T000000Z-2-reused" "$W/.audit/commit-push/20260101T000000Z-3-live"
printf '%s 1\n' "$$" > "$W/.audit/commit-push/20260101T000000Z-2-reused/run.pid"
printf '%s %s\n' "$$" "$(sed -e 's/^.*) //' "/proc/$$/stat" | awk '{print $20}')" > "$W/.audit/commit-push/20260101T000000Z-3-live/run.pid"; run
has "17.4 control: a run.pid with the live process id AND its real start time is listed live" "$(jq -c .live_runs "$REP" 2>/dev/null)" "20260101T000000Z-3-live"
has "17.4 a run.pid whose process id is alive but whose start time differs (pid reuse) is listed interrupted" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "20260101T000000Z-2-reused"
hasnot "17.4 and not live" "$(jq -c .live_runs "$REP" 2>/dev/null)" "20260101T000000Z-2-reused"
mk pk5; run; hasnot "17.5 a run never lists itself as live" "$(jq -c .live_runs "$REP" 2>/dev/null)" "$RUN"; hasnot "17.5 nor as interrupted" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "$RUN"
# -- N9: the last-row and malformed-row branches of the owed-gate reader ----
nonl() { printf '# gate\twhen\tnote\nREMOTE_CHECKS\talways\tno final newline' > "$W/scripts/repo/owed_gates.tsv"; }
PRE=nonl mk pk3; run
eq "17.3 an owed row without a final newline is still owed: 14, never 0" "$RC" 14; has "17.3 the row is named" "$(cat "$RD/deferrals.tsv" 2>/dev/null)" REMOTE_CHECKS
crlf() { printf '# gate\twhen\tnote\r\nREMOTE_CHECKS\talways\r\n' > "$W/scripts/repo/owed_gates.tsv"; }
PRE=crlf mk pk3b; run; eq "17.3 a CRLF owed row is not a clean pass: 20 owed_gates_invalid (its condition is not in the set)" "$RC" 20; has "17.3 owed_gates_invalid" "$OUT" owed_gates_invalid
onlyc() { printf '# gate\twhen\tnote\n# nothing is owed: every gate is built\n\n' > "$W/scripts/repo/owed_gates.tsv"; }
PRE=onlyc mk pk3c; run; eq "17.3 a list holding comments only is the explicit 'every gate built' decision: 0" "$RC" 0
badname() { printf '# gate\twhen\tnote\nlower_case\talways\tx\n' > "$W/scripts/repo/owed_gates.tsv"; }
PRE=badname mk pk3d; run; eq "17.3 a gate name outside ^[A-Z][A-Z0-9_]*\$: 20" "$RC" 20; has "17.3 owed_gates_invalid" "$OUT" owed_gates_invalid
# -- N5: a flag recorded before S1 is in the S1 merge commit; a flag found at S3/S4 is stated as such ----
PRE=realowed mk pc
echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"
foreign src/f.txt 'f\n'; run
eq "17.6 control: the run made a merge commit" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w | tr -d ' ')" 3; eq "17.6 control: origin holds it" "$(tipof a)" "$(head_)"
has "17.6 control: the merge message carries the CPA-Run trailer of this run" "$(msg)" "CPA-Run: $RUN"
has "17.6 the pushed S1 merge commit carries the owed-gate flag (recorded at S0, before S1)" "$(msg)" "GATES_NOT_BUILT"
# -- N4: a declared path that no S3 check judged is not a clean exit ----
mk pd; ln -s ../README.txt "$W/src/link"; paths 'src/link\n'; run --paths-from "$C/p.txt" "symlink"
has "17.7 control: validate_cheap reported the symlink as not judged" "$(cat "$RD/validate.out" 2>/dev/null)" symlink_not_judged
eq "17.7 a declared symlink no check judged: 14, never 0" "$RC" 14; has "17.7 the report names CHECKS_NOT_JUDGED" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECKS_NOT_JUDGED
has "17.7 the report names the path" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" "src/link"
mk pd2; printf 'if then\n\0\n' > "$W/src/b.sh"; paths 'src/b.sh\n'; bash -n "$W/src/b.sh" 2>/dev/null && bad "17.8 control: b.sh parses" || ok "17.8 control: src/b.sh fails bash -n"
run --paths-from "$C/p.txt" "binary sh"; has "17.8 control: validate_cheap left shell_parse out" "$(cat "$RD/validate.out" 2>/dev/null)" "left_out	shell_parse"
eq "17.8 a shell file left out of shell_parse: 14, never 0" "$RC" 14; has "17.8 CHECKS_NOT_JUDGED" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECKS_NOT_JUDGED; has "17.8 the report names the path and the check" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" "shell_parse"
mk pd3; printf 'trailing_whitespace\tbuiltin:trailing_whitespace\tIMG-TESTUTIL\tplain\t-\tfiles\tws\n' >> "$W/scripts/repo/validate_checks.tsv"; git -C "$W" add -A; git -C "$W" commit -qm "ws row"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; printf '\x89PNG\r\n\x1a\n\0\0\0' > "$W/src/i.png"; paths 'src/i.png\n'; run --paths-from "$C/p.txt" "image"
has "17.9 control: the text checks left the image out" "$(cat "$RD/validate.out" 2>/dev/null)" left_out; eq "17.9 an image no text check applies to is not a deferral (nothing is owed): 0" "$RC" 0
hasnot "17.9 and not flagged" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECKS_NOT_JUDGED; has "17.9 yet the report still names the path no check judged" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" "src/i.png"
# -- N7: the host entry's exit set is closed when HOME and CPA_HOST_STATE are unset ----
mk pe; ( cd "$W" && envr -u HOME -u CPA_HOST_STATE PATH="$C/shimr:$PATH" "$C/bin/cpa-host" ) >"$C/o" 2>"$C/e"; prc=$?
eq "17.10 HOME and CPA_HOST_STATE unset: 20, never a shell error's 1" "$prc" 20; has "17.10 the refusal names the cause" "$(cat "$C/e")" state_unresolved
ctl_apply; ( cd "$W" && envr -u HOME PATH="$C/shimr:$PATH" CPA_HOST_STATE="$C/state" "$C/bin/cpa-host" ) >"$C/o" 2>"$C/e"; prc=$?
eq "17.10 HOME unset with CPA_HOST_STATE set: 0 (the state directory is all the run needs)" "$prc" 0
# -- N3: CPA with the REAL scripts/anti-mess/sweep.sh in the approved copy ----
realsweep() { rm -f "$W/scripts/anti-mess/sweep.sh"; cp "$SRC/scripts/anti-mess/sweep.sh" "$SRC/scripts/anti-mess/catalogue.yaml" "$W/scripts/anti-mess/"; chmod 755 "$W/scripts/anti-mess/sweep.sh"; }
PRE=realsweep mk ps; echo s > "$W/src/s.txt"; paths 'src/s.txt\n'; run --paths-from "$C/p.txt" "s"
has "17.11 control: the real sweep ran at S0 (its table is in sweep-s0.out)" "$(cat "$RD/sweep-s0.out" 2>/dev/null)" "AM-G2"
eq "17.11 a run with the real sweep in the approved manifest: 0, never a sweep_error" "$RC" 0; hasnot "17.11 no sweep_error" "$OUT" sweep_error
eq "17.11 no SWEEP_ABSENT: the sweep ran" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" "[]"; eq "17.11 the run's commit is on origin" "$(tipof a)" "$(head_)"
echo s2 > "$W/src/s2.txt"; paths 'src/s2.txt\n'; XENV="ANTIMESS_ROOT=$C ANTIMESS_OWNED_ORGS=ext" run --paths-from "$C/p.txt" "s2"; unset XENV
eq "17.12 the caller's ANTIMESS_ROOT and ANTIMESS_OWNED_ORGS do not aim the gate elsewhere: 0" "$RC" 0
has "17.12 the sweep judged the run's own repository" "$(cat "$RD/sweep-s0.out" 2>/dev/null)" "AM-G2"
fi

# ======== 18 round 4 of the 11.4.276 budget (WF17-cpa consolidation, fix-r5): every probe of the fix list, adopted ===============================
# Each case drives the REAL artefact through cpa-host (nothing re-implements the guarded logic). A hostile input is first proven to DO something outside a run (a control needle,
# 11.4.201(7)(b)), then given to a run; the oracle is the OUTCOME: the exit status, the tips of origin and mirror, the third-party remote, and a marker count of 0.
cnt() { if [ -s "$1" ]; then wc -l < "$1" | tr -d ' '; else echo 0; fi; }
needle() { local n; n="$(cnt "$2")"; [ "$n" -ge "$3" ] && ok "$1 (control needle: the hostile input acts outside a run, x$n)" || bad "$1 (control needle: the hostile input did nothing outside a run, x$n: the probe below is blind)"; }
chg() { echo a > "$W/src/a.txt"; paths 'src/a.txt\n'; }
realsweep18() { rm -f "$W/scripts/anti-mess/sweep.sh"; cp "$SRC/scripts/anti-mess/sweep.sh" "$SRC/scripts/anti-mess/catalogue.yaml" "$W/scripts/anti-mess/"; chmod 755 "$W/scripts/anti-mess/sweep.sh"; }
unshim() { # unshim <path>: the genuine file of the scripts under test back into the clone (committed, pushed, the trust state rebuilt)
  cp "$SRC/$1" "$W/$1"; chmod --reference="$SRC/$1" "$W/$1"; git -C "$W" add -A; git -C "$W" commit -qm "genuine $1"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; }
okrun() { eq "$1: 0" "$RC" 0; moved "$1"; eq "$1: origin holds HEAD" "$(tipof a)" "$(head_)"; }
sigwait() { local n=0; while [ ! -e "$1" ] && [ $n -lt 150 ]; do sleep 0.2; n=$((n+1)); done; [ -e "$1" ]; }
if sect "18 env"; then

# ---- ENV: the caller's process environment (ENV-1 .. ENV-12) --------------------------------------------------------------------------------------
mk e1; chg; M="$C/fn.marker"; : > "$M"
fnw=("BASH_FUNC_git%%=() { echo x >> $M; command git \"\$@\"; }" "BASH_FUNC_unset%%=() { echo x >> $M; }" "BASH_FUNC_exit%%=() { echo x >> $M; builtin exit \"\$@\"; }" "BASH_FUNC_[%%=() { echo x >> $M; builtin [ \"\$@\"; }")
env "${fnw[@]}" bash -c 'git --version >/dev/null; unset ZZ; [ 1 = 1 ]; exit 0'; needle "18.1 exported functions replace git, unset, exit and [" "$M" 4
: > "$M"; runenv "${fnw[@]}" -- --paths-from "$C/p.txt" "a"; okrun "18.1 a run under exported functions named git, unset, exit and ["; eq "18.1 no imported function ran in the entry process, the run or a helper" "$(cnt "$M")" 0
mk e2; chg; M="$C/xt.marker"; : > "$M"
[ "$(env SHELLOPTS=noexec bash -c 'echo ran' 2>&1 | wc -c | tr -d ' ')" = 0 ] && ok "18.2 control needle: SHELLOPTS=noexec makes a bash run nothing" || bad "18.2 control needle: SHELLOPTS=noexec did nothing here"
runenv SHELLOPTS=noexec -- --paths-from "$C/p.txt" "a"; okrun "18.2 a run under SHELLOPTS=noexec (it used to be exit 0 with nothing run)"
mk e2b; chg; M="$C/xt.marker"; : > "$M"; ps4="+\$(echo x >> $M)"
env SHELLOPTS=xtrace PS4="$ps4" bash -c 'true' 2>/dev/null; needle "18.2b SHELLOPTS=xtrace with a PS4 command substitution" "$M" 1
: > "$M"; runenv SHELLOPTS=xtrace "PS4=$ps4" -- --paths-from "$C/p.txt" "a"; okrun "18.2b a run under xtrace + PS4"; eq "18.2b the PS4 command ran nowhere" "$(cnt "$M")" 0
mk e2c; chg; echo base > "$C/ncl"; ( set -C; echo x > "$C/ncl" ) 2>/dev/null && bad "18.2c control needle: noclobber did not refuse" || ok "18.2c control needle: SHELLOPTS=noclobber refuses an overwrite"
runenv SHELLOPTS=noclobber -- --paths-from "$C/p.txt" "a"; okrun "18.2c a run under SHELLOPTS=noclobber (it used to give lock_error)"
mk e2d; chg; runenv SHELLOPTS=noglob -- --paths-from "$C/p.txt" "a"; okrun "18.2d a run under SHELLOPTS=noglob (it used to list interrupted=[\"*\"])"; eq "18.2d interrupted_runs is empty" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "[]"
# BASHOPTS=nocasematch: a branch named MAIN passes `case main|master` only when the option reaches the script
mk e3; git -C "$W" checkout -q -b MAIN 2>/dev/null; chg
[ "$(env BASHOPTS=nocasematch bash -c 'case MAIN in main) echo y ;; esac')" = y ] && ok "18.3 control needle: BASHOPTS=nocasematch makes case match MAIN against main" || bad "18.3 control needle: nocasematch did not act"
runenv BASHOPTS=nocasematch -- --paths-from "$C/p.txt" "a"; eq "18.3 the branch MAIN is refused under BASHOPTS=nocasematch: 20" "$RC" 20; has "18.3 wrong_branch" "$OUT" wrong_branch
# TMOUT: a read that times out lost the S1a listing (the 9.2 backup omitted the dirty file)
mk e5; sed -i 's/^L2$/DIRTY/' "$W/src/conf.txt"; echo a > "$W/src/a.txt"; paths 'src/a.txt\nsrc/conf.txt\n'
[ "$( (sleep 2; echo a) | env TMOUT=1 bash -c 'read x; echo "got=$x"')" = "got=" ] && ok "18.5 control needle: TMOUT=1 makes a read time out" || bad "18.5 control needle: TMOUT did not time the read out"
XENV="GITSHIM_SLEEP_ON=status.--porcelain" runenv TMOUT=1 -- --commit-before-integrate --paths-from "$C/p.txt" "a"; XENV=""
okrun "18.5 --commit-before-integrate under TMOUT=1 with a slow git status"; has "18.5 the S1a backup lists the dirty file (the listing was read in full)" "$(cat "$RD/backup/s1a/worktree.sha256" 2>/dev/null)" "src/conf.txt"
# PYTHONPATH: a sitecustomize runs in every python3 without -I (the run released a commit held on a NO-GO verdict to origin)
mk e6; chg; M="$C/py.marker"; mkdir -p "$C/pp"; printf 'open(%s,"a").write("x\\n")\n' "'$M'" > "$C/pp/sitecustomize.py"; : > "$M"
PYTHONPATH="$C/pp" python3 -c pass; needle "18.6 PYTHONPATH sitecustomize" "$M" 1; : > "$M"
runenv "PYTHONPATH=$C/pp" -- --paths-from "$C/p.txt" "a"; okrun "18.6 a run under PYTHONPATH"; eq "18.6 no python of the run imported the caller's sitecustomize" "$(cnt "$M")" 0
mk e6b; M="$C/py.marker"; mkdir -p "$C/pp"; printf 'open(%s,"a").write("x\\n")\n' "'$M'" > "$C/pp/sitecustomize.py"; : > "$M"; VB="$EVR/reviews/WP-P.json"
mkdir -p "$W/$EVR/reviews"; GOJ NO-GO '["x"]' 1 > "$W/$EVR/reviews/WP-P.json"; paths "$VB\n"; run --paths-from "$C/p.txt" "a NO-GO verdict"; eq "18.6b setup: the NO-GO verdict is committed" "$RC" 0
echo h > "$W/src/h.txt"; paths "src/h.txt\t$VB\n"; : > "$M"; runenv "PYTHONPATH=$C/pp" -- --paths-from "$C/p.txt" "held on the NO-GO verdict"
eq "18.6b a commit held on a NO-GO verdict stays held under PYTHONPATH: 14" "$RC" 14; has "18.6b review_pending" "$OUT" review_pending; eq "18.6b release_state's python ran no caller module" "$(cnt "$M")" 0
# LD_PRELOAD: a caller library ran in every process of the run
mk e7; chg; M="$C/ld.marker"; : > "$M"
printf '#include <stdio.h>\n__attribute__((constructor)) static void f(void){char b[64]="";FILE*c=fopen("/proc/self/comm","r");if(c){if(!fgets(b,64,c))b[0]=0;fclose(c);}FILE*h=fopen("%s","a");if(h){fputs(b,h);fputs("\\n",h);fclose(h);}}\n' "$M" > "$C/ld.c"
if cc -shared -fPIC -o "$C/ld.so" "$C/ld.c" 2>/dev/null; then
  env LD_PRELOAD="$C/ld.so" /bin/true; needle "18.7 LD_PRELOAD library" "$M" 1; : > "$M"
  runenv "LD_PRELOAD=$C/ld.so" -- --paths-from "$C/p.txt" "a"; okrun "18.7 a run under LD_PRELOAD"; eq "18.7 the caller library was loaded only by the launching entry (cpa-host, a bash script) and env, which start before the entry scrubs the environment, and by no process after it (no git, jq, python, sha256sum, helper)" "$(sort -u "$M" | grep -v '^$' | tr "\n" " ")" "cpa-host env "
else echo "SKIP 18.7 no C compiler: LD_PRELOAD cannot be probed (the allowlist drops it by the same mechanism as PYTHONPATH, 18.6)"; fi
# a relative CPA_HOST_STATE / HOME / TMPDIR, and empty or relative PATH elements
mk e8; chg; mkdir -p "$W/.audit/st"; cp "$C/state/trust.json" "$W/.audit/st/trust.json"; cp -a "$C/state/blobs" "$W/.audit/st/"
runenv CPA_HOST_STATE=.audit/st -- --paths-from "$C/p.txt" "a"; eq "18.8 a relative CPA_HOST_STATE is refused: 20" "$RC" 20; has "18.8 env_value_unsafe" "$OUT" env_value_unsafe; eq "18.8 nothing was committed" "$(head_)" "$SEEDHEAD"
runenv HOME=rel -- --paths-from "$C/p.txt" "a"; eq "18.8b a relative HOME is refused: 20" "$RC" 20; has "18.8b env_value_unsafe" "$OUT" env_value_unsafe
mk e9; chg; M="$C/bash.marker"; printf '#!/bin/sh\necho x >> %s\nexec /bin/bash "$@"\n' "$M" > "$W/bash"; chmod 755 "$W/bash"; printf '#!/bin/sh\necho x >> %s\nexec /usr/bin/od "$@"\n' "$M" > "$W/od"; chmod 755 "$W/od"
( cd "$W" && PATH=":$PATH" bash -c 'bash -c true' ) 2>/dev/null; needle "18.9 an empty PATH element runs a work-tree file named bash" "$M" 1; : > "$M"
runenv "PATH=:$C/shimr:$PATH" -- --paths-from "$C/p.txt" "a"; eq "18.9 an empty PATH element is refused: 20" "$RC" 20; has "18.9 env_value_unsafe" "$OUT" env_value_unsafe; eq "18.9 the work-tree bash and od ran nowhere" "$(cnt "$M")" 0
runenv "PATH=$C/shimr:$PATH:" -- --paths-from "$C/p.txt" "a"; eq "18.9d a TRAILING empty PATH element is refused too (word splitting drops it, only the textual check sees it): 20" "$RC" 20; has "18.9d env_value_unsafe" "$OUT" env_value_unsafe
runenv "PATH=$C/shimr:$PATH:rel/bin" -- --paths-from "$C/p.txt" "a"; eq "18.9b a relative PATH element is refused: 20" "$RC" 20
mkdir -p "$W/tools"; printf '#!/bin/sh\necho x >> %s\nexec /usr/bin/git "$@"\n' "$M" > "$W/tools/git"; chmod 755 "$W/tools/git"
runenv "PATH=$W/tools:$C/shimr:$PATH" -- --paths-from "$C/p.txt" "a"; eq "18.9c a PATH element inside the repository is refused before git runs: 20" "$RC" 20; eq "18.9c the work-tree git ran nowhere" "$(cnt "$M")" 0
# the class oracle (replaces the text inventory of 11.5): the environ of the released core AND of a helper holds NAMES from the allowlist only
mk e10; cp "$W/scripts/repo/scope_check.sh" "$W/scripts/repo/scope_check_orig.sh"
hshim scripts/repo/scope_check.sh 'cp /proc/$PPID/environ "$CPA_ROOT/.audit/env-core.txt"; cp /proc/$$/environ "$CPA_ROOT/.audit/env-helper.txt"; exec "$(dirname "$0")/scope_check_orig.sh" "$@"'
chg; ALLOW=" _ HOME PATH TMPDIR CPA_HOST_STATE SKIP_LONG SSH_AUTH_SOCK GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL LONGOPS_ALLOW_TMPFS LC_ALL PWD SHLVL CPA_ROOT CPA_RUN_ID CPA_ADOPTION_COMMIT CPA_APPROVED_DIR CPA_RUN LONGOPS_REPO DISK_HEADROOM_OUT_DIR GIT_TERMINAL_PROMPT GIT_CONFIG_COUNT GIT_CONFIG_KEY_0 GIT_CONFIG_VALUE_0 GIT_CONFIG_KEY_1 GIT_CONFIG_VALUE_1 GIT_CONFIG_KEY_2 GIT_CONFIG_VALUE_2 GIT_CONFIG_KEY_3 GIT_CONFIG_VALUE_3 GIT_CONFIG_KEY_4 GIT_CONFIG_VALUE_4 GIT_CONFIG_KEY_5 GIT_CONFIG_VALUE_5 GIT_CONFIG_KEY_6 GIT_CONFIG_VALUE_6 "
[ "$(env FOO=1 bash -c 'tr "\0" "\n" < /proc/$$/environ' | grep -c '^FOO=')" = 1 ] && ok "18.10 control needle: the environ dump sees a name that is there" || bad "18.10 control needle: the environ dump is blind"
runenv FOO=1 BAR=2 PYTHONPATH=/x LD_LIBRARY_PATH=/x LD_DEBUG=x PYTHONHOME=/x BASH_XTRACEFD=9 GLOBIGNORE=x GIT_DIR=/x GIT_INDEX_FILE=/x GIT_EXEC_PATH=/x GIT_SSH_COMMAND=x ENV=/x CDPATH=/x TMOUT=5 LANG=de_DE LC_MESSAGES=de http_proxy=x EMAIL=x SSH_ASKPASS=x DISPLAY=x XDG_CONFIG_HOME=/x -- --paths-from "$C/p.txt" "a"
okrun "18.10 a run under a caller environment that holds a name of every family"
un=""; for f in env-core env-helper; do [ -s "$W/.audit/$f.txt" ] || { un="$un MISSING:$f"; continue; }; for n in $(tr '\0' '\n' < "$W/.audit/$f.txt" | sed 's/=.*//'); do case "$ALLOW" in *" $n "*) ;; *) un="$un $f:$n" ;; esac; done; done
eq "18.10 every NAME in the environ of the released core and of a helper is allowlisted (unaccounted:${un:- none})" "${un:- none}" " none"
eq "18.10 LC_ALL is C in the helper" "$(tr '\0' '\n' < "$W/.audit/env-helper.txt" | grep '^LC_ALL=')" "LC_ALL=C"
# the commit identity and the other kept names are not dropped; GIT_INDEX_FILE/GIT_DIR/GIT_WORK_TREE of the caller do not steer the commit
mk e11; chg; GIT_INDEX_FILE="$C/evil.index" git -C "$W" read-tree HEAD; eb="$(echo evil | git -C "$W" hash-object -w --stdin)"; GIT_INDEX_FILE="$C/evil.index" git -C "$W" update-index --add --cacheinfo "100644,$eb,evil.txt"
runenv "GIT_INDEX_FILE=$C/evil.index" "GIT_DIR=$W/.git" "GIT_WORK_TREE=$W" -- --paths-from "$C/p.txt" "a"; okrun "18.11 a run under GIT_INDEX_FILE, GIT_DIR and GIT_WORK_TREE"; eq "18.11 the commit holds exactly the declared file" "$(git -C "$W" show --name-only --format= HEAD)" "src/a.txt"
mk e12; chg; runenv HOME= -u CPA_HOST_STATE -- --paths-from "$C/p.txt" "a"; eq "18.12 HOME empty and CPA_HOST_STATE unset: 20" "$RC" 20; has "18.12 state_unresolved" "$OUT" state_unresolved

fi
if sect "18 repo"; then
# ---- REPO: repository and work-tree state (REPO-1 .. REPO-4) --------------------------------------------------------------------------------------
mk r1; M="$C/json.marker"; printf 'open(%s,"a").write("x\\n")\nraise SystemExit(0)\n' "'$M'" > "$W/json.py"; echo json.py >> "$W/.git/info/exclude"; : > "$M"
( cd "$W" && python3 -c 'import json' ) 2>/dev/null; needle "18.13 a work-tree json.py replaces the stdlib module for a python started in the root" "$M" 1; : > "$M"
printf 'if then\n' > "$W/src/bad.sh"; paths 'src/bad.sh\n'; h0="$(head_)"; run --paths-from "$C/p.txt" "bad"
eq "18.13 the failing src/bad.sh is still refused under a work-tree json.py: 10" "$RC" 10; eq "18.13 nothing committed" "$(head_)" "$h0"; eq "18.13 the work-tree module ran nowhere" "$(cnt "$M")" 0
mk r2; M="$C/fsm.marker"; printf '#!/bin/sh\necho x >> %s\nexit 1\n' "$M" > "$C/fsm.sh"; chmod 755 "$C/fsm.sh"; git -C "$W" config core.fsmonitor "$C/fsm.sh"; : > "$M"; git -C "$W" status >/dev/null 2>&1
needle "18.14 a core.fsmonitor program in .git/config" "$M" 1; : > "$M"; chg; run --paths-from "$C/p.txt" "a"
eq "18.14 a program named by the repository's own config is refused: 20" "$RC" 20; has "18.14 repo_config_not_approved" "$OUT" repo_config_not_approved; eq "18.14 it ran nowhere in the run" "$(cnt "$M")" 0; eq "18.14 nothing committed" "$(head_)" "$SEEDHEAD"
for kv in "credential.helper=/bin/true" "core.sshCommand=/bin/true" "core.askPass=/bin/true" "include.path=/dev/null" "commit.gpgSign=true" "url.$C/fx/.pushInsteadOf=$C/evil/" "filter.x.clean=cat" "core.attributesFile=/dev/null"; do
  mk r2b; git -C "$W" config "${kv%%=*}" "${kv#*=}"; chg; run --paths-from "$C/p.txt" "a"; eq "18.14b the config key ${kv%%=*} is refused: 20" "$RC" 20; has "18.14b repo_config_not_approved" "$OUT" repo_config_not_approved
done
mk r2c; chg; printf '*.txt filter=lf\n' > "$W/.git/info/attributes"; run --paths-from "$C/p.txt" "a"; eq "18.14c a non-empty .git/info/attributes is refused: 20" "$RC" 20
mk r2d; printf '*.txt working-tree-encoding=UTF-16LE\n' > "$W/.gitattributes"; git -C "$W" add .gitattributes; git -C "$W" commit -qm attr; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; chg; run --paths-from "$C/p.txt" "a"
eq "18.14d a .gitattributes that names working-tree-encoding is refused (the judged bytes would not be the committed bytes): 20" "$RC" 20
# ownership from EVERY url
mk r3; git init -q --bare -b main "$T/fx/decoy.git"; git -C "$W/mods/ext" remote add decoy "$T/fx/decoy.git"
git -C "$W/mods/ext" checkout -q -B main 2>/dev/null; git -C "$W/mods/ext" config user.email t@t; git -C "$W/mods/ext" config user.name t; echo x > "$W/mods/ext/x.txt"; printf 'mods/ext/x.txt\n' > "$C/p.txt"; e0="$(git -C "$T/ext/t.git" rev-parse main)"

run --repo mods/ext --paths-from "$C/p.txt" "third party with an owned decoy remote"; eq "18.15 a third-party submodule with ONE extra owned remote is still not owned: 20" "$RC" 20; has "18.15 repo_not_owned" "$OUT" repo_not_owned
eq "18.15 the third-party remote did not move" "$(git -C "$T/ext/t.git" rev-parse main)" "$e0"; rm -rf "$T/fx/decoy.git"
mk r3b; mkdir -p "$T/ev"; git init -q --bare -b main "$T/ev/evil.git"; git -C "$W" config remote.origin.pushurl "$T/ev/evil.git"; chg; run --paths-from "$C/p.txt" "a"
eq "18.15b a push url that names a third party is refused before anything is committed: 20" "$RC" 20; has "18.15b repo_not_owned" "$OUT" repo_not_owned; eq "18.15b the unapproved repository holds nothing" "$(git -C "$T/ev/evil.git" rev-parse -q --verify main 2>/dev/null || echo none)" none; eq "18.15b nothing committed" "$(head_)" "$SEEDHEAD"
mk r3c; git -C "$W" config --add remote.origin.pushurl "$C/fx/a.git"; git -C "$W" config --add remote.origin.pushurl "$C/fx/b.git"; chg; run --paths-from "$C/p.txt" "a"; okrun "18.15c several owned push urls (the project's multi-upstream push) are allowed"
# the APPROVED registry decides the run set
mk r4; sed -i '/^shell_parse\t/d' "$W/scripts/repo/validate_checks.tsv"; printf 'if then\n' > "$W/src/bad.sh"; paths 'src/bad.sh\n'; h0="$(head_)"; run --paths-from "$C/p.txt" "bad"
eq "18.16 a registry row deleted in the work tree does not switch its check off: 10" "$RC" 10; eq "18.16 nothing committed" "$(head_)" "$h0"; has "18.16 the removed row is reported pending, with the reason" "$(cat "$RD/validate.out" 2>/dev/null)" "check_pending_release	shell_parse	removed_in_worktree"

fi
if sect "18 toctou"; then
# ---- TOCTOU: judged bytes are committed bytes (TOCTOU-1 .. TOCTOU-6) ----------------------------------------------------------------------------
tocase() { # tocase <case> <hook body, @W@ = the work tree>: the hook rewrites the declared file after S3 judged it, when S5 runs `git add -A -- :(literal)...`
  mk "$1"; printf 'echo ok\n' > "$W/src/ok.sh"; paths 'src/ok.sh\n'; printf '#!/bin/bash\n%s\n' "${2//@W@/$W}" > "$C/hook.sh"; chmod 755 "$C/hook.sh"; h0="$(head_)"
  XENV="GITSHIM_HOOK_ON=add.-A.--.:.literal. GITSHIM_HOOK=$C/hook.sh" run --paths-from "$C/p.txt" "ok"; XENV=""
}
tocase t1 "printf 'if then\\n' > '@W@/src/ok.sh'"
[ "$RC" != 0 ] && ok "18.17 a valid file turned into 'if then' between S3 and S5 does not exit 0" || bad "18.17 exit $RC: the unjudged bytes were committed and pushed"
has "18.17 the reason is changed_since_validation" "$OUT" changed_since_validation; eq "18.17 nothing was pushed" "$(pushes)" 0; hasnot "18.17 origin does not hold the bad bytes" "$(git -C "$C/fx/a.git" show main:src/ok.sh 2>&1)" "if then"
tocase t2 "printf '<<<<<<< x\\nb\\n=======\\nc\\n>>>>>>> y\\n' > '@W@/src/ok.sh'"
[ "$RC" != 0 ] && ok "18.18 conflict markers written after S3 do not exit 0" || bad "18.18 exit $RC"; has "18.18 changed_since_validation" "$OUT" changed_since_validation; hasnot "18.18 origin holds no markers" "$(git -C "$C/fx/a.git" show main:src/ok.sh 2>&1)" "<<<<<<<"
tocase t3 "rm -f '@W@/src/ok.sh' && ln -s ../README.txt '@W@/src/ok.sh'"
[ "$RC" != 0 ] && ok "18.19 a file swapped for a symlink after S3 does not exit 0" || bad "18.19 exit $RC"; has "18.19 changed_since_validation" "$OUT" changed_since_validation
tocase t4 "$REALGIT -C '@W@' checkout -q -b side"
eq "18.20 a branch switched between S0 and S5: 20" "$RC" 20; has "18.20 branch_moved, not remote_unproven" "$OUT" branch_moved; eq "18.20 the branch side holds no commit of the run" "$(git -C "$W" rev-parse side)" "$(git -C "$W" rev-parse main)"
# the released copy is read-only and re-hashed at each invocation
mk t5; chg; printf '#!/bin/bash\nchmod u+w "$CPA_ROOT/.audit/commit-push/$CPA_RUN_ID/released/scripts/repo/validate_cheap.sh"; printf "exit 0\\n" > "$CPA_ROOT/.audit/commit-push/$CPA_RUN_ID/released/scripts/repo/validate_cheap.sh"\n' > "$C/hook.sh"; chmod 755 "$C/hook.sh"
XENV="GITSHIM_HOOK_ON=check-ignore GITSHIM_HOOK=$C/hook.sh" run --paths-from "$C/p.txt" "a"; XENV=""; eq "18.21 a helper of the released copy replaced after the start: 20" "$RC" 20; has "18.21 approved_copy_changed" "$OUT" approved_copy_changed; has "18.21 the FAIL line reached the operator, not only the helper's output file" "$OUT" "cpa: FAIL S3 20 approved_copy_changed"; eq "18.21 nothing pushed" "$(pushes)" 0
mk t6; run; rd6="$RD/released"; eq "18.22 a released file is 0444/0555 and a released directory 0555" "$(stat -c %a "$rd6/scripts/repo/lib_safe.sh" "$rd6/scripts/commit-push-all.sh" "$rd6/scripts/repo" | tr '\n' ' ')" "444 555 555 "

fi
if sect "18 sig"; then
# ---- SIG: signal windows (SIG-1 .. SIG-6) ------------------------------------------------------------------------------------------------------
# SIG-1/2: a TERM 0.1 s after the exec (the old start-up spent 70-270 ms with no handler installed) is a 20 with a report, for the copied script and for the host entry
mk s1; for i in 1 2 3; do
  ( cd "$W" && envr $(ENVV) "$C/bin/cpa-host" ) >"$C/out.txt" 2>"$C/err.txt" & HP=$!
  c=""; n=0; while [ -z "$c" ] && [ $n -lt 4000 ]; do c="$(pgrep -f "$W/.audit/commit-push/[^ ]*/released/scripts/commit-push-all[.]sh" | head -1)"; [ -n "$c" ] || sleep 0.005; n=$((n+1)); done
  if [ -n "$c" ] && [ "$c" -gt 1 ]; then kill -TERM "$c" 2>/dev/null; fi; wait $HP; hrc=$?
  eq "18.23 TERM the instant the copied script is running, iteration $i: 20" "$hrc" 20; eq "18.23 every run directory holds a report" "$(for d in "$W"/.audit/commit-push/*/; do [ -f "$d/report.json" ] || echo missing; done | wc -l | tr -d ' ')" 0
done
mk s2
for dl in 0.05 0.1; do
  ( cd "$W" && envr $(ENVV) "$C/bin/cpa-host" ) >"$C/out.txt" 2>"$C/err.txt" & HP=$!; sleep "$dl"; hp="$(pgrep -f "$C/bin/cp[a]-host" | head -1)"; [ -z "$hp" ] || kill -TERM "$hp" 2>/dev/null; wait $HP; hrc=$?
  eq "18.24 TERM $dl s after the start of the host entry: 20" "$hrc" 20
  eq "18.24 and no run directory is left without a report ($dl s)" "$(for d in "$W"/.audit/commit-push/*/; do [ -d "$d" ] || continue; [ -f "$d/report.json" ] || echo missing; done | wc -l | tr -d ' ')" 0
done
# SIG-3/4: signals that arrive WHILE the report is written do not cut it: a first signal at S3, then a salvo from release.sh (inside finish), then a GROUP signal
mk s3; hshim scripts/repo/validate_cheap.sh 'kill -TERM $PPID; sleep 1; exit 0'
hshim scripts/longops/release.sh 'p=$PPID; for s in HUP INT QUIT TERM USR1 USR2 ALRM; do kill -$s "$p" 2>/dev/null; done; exec "$(dirname "$0")/release_real.sh" "$@"'
cp "$SRC/scripts/longops/release.sh" "$W/scripts/longops/release_real.sh"; git -C "$W" add -A; git -C "$W" commit -qm real; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; chg
run --paths-from "$C/p.txt" "a"; eq "18.25 a TERM at S3 and then HUP INT QUIT TERM USR1 USR2 ALRM during finish: 20, the salvo did not cut the exit path" "$RC" 20; eq "18.25 the report is complete" "$(jq -r .exit "$REP" 2>/dev/null)" 20; eq "18.25 the lock was released" "$(jq -r .lock.released "$REP" 2>/dev/null)" true
mk s4; hshim scripts/longops/release.sh 'p=$PPID; g="$(awk "{print \$5}" /proc/$p/stat)"; if [[ "$g" =~ ^[0-9]+$ ]] && [ "$g" -gt 1 ] && [ "$g" = "$p" ]; then kill -TERM -- "-$g"; sleep 0.3; fi; exec "$(dirname "$0")/release_real.sh" "$@"'
cp "$SRC/scripts/longops/release.sh" "$W/scripts/longops/release_real.sh"; git -C "$W" add -A; git -C "$W" commit -qm real; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; chg
: > "$C/git.log"; ctl_apply; ( cd "$W" && envr $(ENVV) setsid -w "$C/bin/cpa-host" --paths-from "$C/p.txt" "a" ) >"$C/out.txt" 2>"$C/err.txt"; RC=$?; OUT="$(cat "$C/out.txt" "$C/err.txt")"; RUN="$(printf '%s\n' "$OUT" | sed -n 's/.*cpa: exit=[0-9]* run=\([^ ]*\).*/\1/p' | tail -1)"; REP="$W/.audit/commit-push/$RUN/report.json"
eq "18.26 a TERM sent to the whole process GROUP while the report is written: the run still ends 0 with the lock released" "$RC:$(jq -r .lock.released "$REP" 2>/dev/null)" "0:true"
# SIG-5: latency: TERM while a helper sleeps 25 s ends the run at once (it used to wait the 25 s)
mk s5; hshim scripts/repo/validate_cheap.sh 'echo $PPID > "$CPA_ROOT/.audit/s5.ppid"; trap "echo TERM > \"$CPA_ROOT/.audit/s5.term\"; exit 143" TERM; sleep 25 & wait'; chg; rm -f "$W/.audit/s5.ppid"
( cd "$W" && envr $(ENVV) "$C/bin/cpa-host" --paths-from "$C/p.txt" "a" ) >"$C/out.txt" 2>"$C/err.txt" & HP=$!; sigwait "$W/.audit/s5.ppid"; t0=$SECONDS; kill -TERM "$(cat "$W/.audit/s5.ppid")"; wait $HP; hrc=$?; dt=$((SECONDS-t0))
eq "18.27 TERM while a helper sleeps: 20" "$hrc" 20; [ "$dt" -lt 12 ] && ok "18.27 the run ended within ${dt}s of the signal (the helper was not waited out)" || bad "18.27 the run ended ${dt}s after the signal"; [ -e "$W/.audit/s5.term" ] && ok "18.27 the helper received the forwarded TERM" || bad "18.27 the helper never received TERM"
# SIG-6: a documented residual: signal 33 cannot be caught; the NEXT run closes the run
mk s6; hshim scripts/repo/validate_cheap.sh 'kill -33 $PPID; sleep 1; exit 0'; chg; run --paths-from "$C/p.txt" "a"; eq "18.28 signal 33 (reserved by glibc, untrappable by bash): status 161, no report" "$RC" 161
unshim scripts/repo/validate_cheap.sh; run; eq "18.28 and the next run closes it and proceeds: 0" "$RC" 0

fi
if sect "18 res"; then
# ---- RES: a run ends and leaves state that blocks the next run ----------------------------------------------------------------------------------
PRE=realsweep18 mk x1
chg; hshim scripts/repo/validate_cheap.sh 'echo $PPID > "$CPA_ROOT/.audit/x1.ppid"; echo $$ > "$CPA_ROOT/.audit/x1.shim"; sleep 60'
( cd "$W" && envr $(ENVV) "$C/bin/cpa-host" --paths-from "$C/p.txt" "a" ) >"$C/out.txt" 2>"$C/err.txt" & HP=$!; sigwait "$W/.audit/x1.ppid"
kp="$(cat "$W/.audit/x1.ppid")"; sp="$(cat "$W/.audit/x1.shim")"; kill -KILL "$kp"; kill -KILL "$sp" 2>/dev/null; wait $HP 2>/dev/null; dead1="$(basename "$(ls -1d "$W/.audit/commit-push/"*/ | head -1)")"
[ -f "$W/.audit/commit-push/$dead1/report.json" ] && bad "18.29 setup: a SIGKILLed run has a report" || ok "18.29 setup: the SIGKILLed run left a directory without a report"
unshim scripts/repo/validate_cheap.sh; run
eq "18.29 the next run, under the REAL sweep, closes the dead run and proceeds: 0 (it used to be 20 sweep_finding at S0)" "$RC" 0; eq "18.29 the dead run is closed by this one" "$(jq -r .closed_by "$W/.audit/commit-push/$dead1/interrupted.json" 2>/dev/null)" "$RUN"
has "18.29 and listed in interrupted_runs" "$(jq -c .interrupted_runs "$REP" 2>/dev/null)" "$dead1"; hasnot "18.29 no claim is left behind" "$(ls "$W/.audit/longops/claims" 2>/dev/null)" commit_push
# a refusal AFTER the run directory exists writes its report: the copied script's self-test, a missing tool
mk x2; mkdir -p "$C/nopy"; for f in /usr/bin/*; do case "${f##*/}" in python3*|python) continue ;; esac; ln -s "$f" "$C/nopy/${f##*/}" 2>/dev/null; done
runenv "PATH=$C/nopy" -- ; eq "18.30 a missing python3 (tool_missing at the self-test): 20" "$RC" 20; d2="$(ls -1d "$W"/.audit/commit-push/*/ | head -1)"; eq "18.30 the run directory holds a report (reason tool_missing)" "$(jq -r .reason "$d2/report.json" 2>/dev/null)" tool_missing
runenv -- ; eq "18.30 and the next run proceeds: 0" "$RC" 0
# TERM during acquire.sh: the claim is released by the compare-and-swap in finish, never leaked
mk x3; cp "$W/scripts/longops/acquire.sh" "$W/scripts/longops/acquire_real.sh"
hshim scripts/longops/acquire.sh 'd="$(dirname "$0")"; "$d/acquire_real.sh" "$@"; r=$?; kill -TERM $PPID; sleep 0.5; exit $r'
run; eq "18.31 a TERM that arrives while acquire.sh returns: 20" "$RC" 20; [ -e "$W/.audit/longops/claims/commit_push" ] && bad "18.31 the claim was leaked" || ok "18.31 the claim was released by finish"
unshim scripts/longops/acquire.sh; run; eq "18.31 the next run is not blocked: 0" "$RC" 0
# a commit made but not recorded (TERM during git commit)
mk x4; chg; XENV="GITSHIM_SIGNAL_ON=commit.-q.-F GITSHIM_SIGNAL=TERM GITSHIM_SIGNAL_TARGET=commit_recursive.sh GITSHIM_SIGNALED=$C/sig4" run --paths-from "$C/p.txt" "a"; XENV=""
[ -e "$C/sig4" ] && ok "18.32 control: the TERM reached commit_recursive.sh at git commit" || bad "18.32 control: no signal was sent"; eq "18.32 the run ended 20" "$RC" 20
eq "18.32 the commit made is recorded in commits.tsv (the TERM was held until the row was written)" "$(wc -l < "$RD/commits.tsv" 2>/dev/null | tr -d ' ')" 1
run; eq "18.32 the next run pushes it: 0, never unrecorded_local_commit" "$RC" 0; eq "18.32 origin holds HEAD" "$(tipof a)" "$(head_)"
# report before release; a report that cannot be written is 20
mk x5; hshim scripts/longops/release.sh 'if [ -s "$CPA_RUN/report.json" ]; then echo yes > "$CPA_ROOT/.audit/rel-saw-report"; else echo no > "$CPA_ROOT/.audit/rel-saw-report"; fi; exec "$(dirname "$0")/release_real.sh" "$@"'
cp "$SRC/scripts/longops/release.sh" "$W/scripts/longops/release_real.sh"; git -C "$W" add -A; git -C "$W" commit -qm real; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; run
eq "18.33 report.json exists when the lock is released (a successor never starts before it)" "$(cat "$W/.audit/rel-saw-report" 2>/dev/null)" yes
mk x6; hshim scripts/longops/release.sh 'rm -f "$CPA_RUN/report.json"; mkdir "$CPA_RUN/report.json"; exit 0'; run
eq "18.34 a report.json that cannot be written: 20 report_write_failed, never a 0 without a report" "$RC" 20; has "18.34 the message" "$OUT" report_write_failed

fi
if sect "18 report"; then
# ---- MODEL / REPORT --------------------------------------------------------------------------------------------------------------------------------
mk p1; V1="$EVR/reviews/WP-A.json"; V2="$EVR/reviews/WP-B.json"; echo a > "$W/src/a.txt"; echo b > "$W/src/b.txt"; paths "src/a.txt\t$V1\nsrc/b.txt\t$V2\n"; run --paths-from "$C/p.txt" "two verdicts"
eq "18.35 two held verdicts: 14" "$RC" 14; eq "18.35 deferrals.tsv holds one LOCAL_ONLY row per verdict" "$(awk -F'\t' '$2=="LOCAL_ONLY"{print $4}' "$RD/deferrals.tsv" | sort | tr '\n' ' ')" "$V1 $V2 "
mk p2; hshim scripts/repo/validate_cheap.sh 'printf "x\n" > "$CPA_RUN/odd  name.txt"; printf "y\n" > "$CPA_RUN/back\\slash.txt"; exec "$(dirname "$0")/validate_cheap_orig.sh" "$@"'
cp "$SRC/scripts/repo/validate_cheap.sh" "$W/scripts/repo/validate_cheap_orig.sh"; git -C "$W" add -A; git -C "$W" commit -qm orig; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; chg; run --paths-from "$C/p.txt" "a"
eq "18.36 the files map keeps a double space and a backslash in a name exactly" "$(jq -c '.files|keys|map(select(test("odd  name|back")))' "$REP" 2>/dev/null)" '["back\\slash.txt","odd  name.txt"]'
eq "18.36 and its hashes are plain sha256" "$(jq -r '.files["back\\slash.txt"]' "$REP" 2>/dev/null)" "$(printf 'y\n' | sha256sum | cut -d' ' -f1)"
mk p3; printf 'if then\n\0\n' > "$W/src/b.sh"; paths 'src/b.sh\n'; run --paths-from "$C/p.txt" "binary sh"
eq "18.37 unjudged_paths holds ONE row per (path, check) and a path is deferred when any row is" "$(jq -c '[.unjudged_paths[]|select(.check=="shell_parse")]' "$REP" 2>/dev/null)" '[{"path":"src/b.sh","check":"shell_parse","deferred":true}]'
mk p4; rm -f "$W/src/conf.txt"; paths 'src/conf.txt\n'; run --paths-from "$C/p.txt" "delete"; eq "18.38 a declared deletion is committed: 0" "$RC" 0
eq "18.38 and reported as a path no check judged, owing nothing" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" '[{"path":"src/conf.txt","check":"deletion","deferred":false}]'
mk p5; printf 'src/\xff.txt\n' > "$C/p.txt"; run --paths-from "$C/p.txt" "bad name"; eq "18.39 a non-UTF-8 declared path: 20 unsafe_path, no traceback" "$RC" 20; has "18.39 the reason" "$OUT" "not UTF-8"; hasnot "18.39 no python traceback" "$OUT" Traceback
mk p6; git -C "$W" remote remove origin; git -C "$W" remote remove mirror; chg; run --paths-from "$C/p.txt" "a"; eq "18.40 a repository with no remote: 20 at S0, never an S6 refusal naming the seed commits" "$RC" 20; has "18.40 no_remote_reachable" "$OUT" no_remote_reachable; eq "18.40 nothing committed" "$(head_)" "$SEEDHEAD"
mk p7; hshim scripts/repo/push_recursive.sh 'echo "REASON	force_refused"; echo "REFUSED	.	force_refused	x"; exit 20'; chg; run --paths-from "$C/p.txt" "a"
eq "18.41 the report states the reason the helper stated" "$(jq -r .reason "$REP" 2>/dev/null)" force_refused
mk p8; hshim scripts/repo/integrate_merge.sh 'echo "REASON	commit_failed"; exit 20'; hshim scripts/repo/integrate_ff_only.sh 'j=""; while [ $# -gt 0 ]; do [ "$1" = --json ] && j="$2"; shift; done; printf "{\"reason\":\"remotes_diverged\"}\n" > "$j"; exit 12'; run
eq "18.42 an integrate_merge refusal outside the old word list keeps its stated reason" "$(jq -r .reason "$REP" 2>/dev/null)" commit_failed

fi
if sect "18 misc"; then
# ---- SKIP / LIST / NAME / INPUT / MSG / VERDICT / BOUNDS / STALE / ENC ------------------------------------------------------------------------
mk k1; printf '<<<<<<< a\nx\n=======\ny\n>>>>>>> b\n' > "$W/src/$(printf '\xc2\xa0')"; printf 'src/\xc2\xa0\n' > "$C/p.txt"; run --paths-from "$C/p.txt" "nbsp"; eq "18.43 a file named U+00A0 with conflict markers is judged: 10" "$RC" 10
mk k1b; printf '<<<<<<< a\nx\n' > "$W/src/ "; printf 'src/ \n' > "$C/p.txt"; run --paths-from "$C/p.txt" "space"; [ "$RC" != 0 ] && ok "18.43b a file named ' ' with markers does not exit 0 ($RC)" || bad "18.43b exit 0"; hasnot "18.43b origin holds no markers" "$(git -C "$C/fx/a.git" ls-tree -r main --name-only | grep -c 'src/ $')" "1"
mk k2; printf 'package m\n<<<<<<< a\nx\n=======\ny\n>>>>>>> b\n\0\n' > "$W/src/m.go"; paths 'src/m.go\n'; run --paths-from "$C/p.txt" "nul"; eq "18.44 a .go file with markers AND a NUL byte: 10 (one NUL no longer turns the scan off)" "$RC" 10
mk k3; printf '#!/bin/bash\nif then\n' > "$W/src/tool"; chmod 755 "$W/src/tool"; paths 'src/tool\n'; run --paths-from "$C/p.txt" "nosuffix"; eq "18.45 a shell script with no suffix is parsed by its shebang: 10" "$RC" 10
mk k4; printf 'if then\n' > "$W/src/x.bash"; paths 'src/x.bash\n'; run --paths-from "$C/p.txt" "bash"; eq "18.46 a failing .bash file: 10" "$RC" 10
mk k4b; printf 'if then\n' > "$W/src/X.SH"; paths 'src/X.SH\n'; run --paths-from "$C/p.txt" "upper"; eq "18.46b a failing .SH file (suffixes compare case-insensitively): 10" "$RC" 10
mk k4c; printf 'check_json\tbuiltin:check_json\tIMG-TESTUTIL\tplain\t-\tfiles\tjson\n' >> "$W/scripts/repo/validate_checks.tsv"; git -C "$W" add -A; git -C "$W" commit -qm "json row"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; printf '{"a":1}\n\0' > "$W/src/d.json"; paths 'src/d.json\n'; run --paths-from "$C/p.txt" "binary json"; eq "18.46c a binary-looking .json owes CHECKS_NOT_JUDGED: 14" "$RC" 14; has "18.46c named" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" check_json
mk k4d; printf 'check_json\tbuiltin:check_json\tIMG-TESTUTIL\tplain\t-\tfiles\tjson\n' >> "$W/scripts/repo/validate_checks.tsv"; printf 'src/d2.json binary\n' > "$W/.gitattributes"; git -C "$W" add -A; git -C "$W" commit -qm "json row and attribute"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; printf '{"a":1}\n\0' > "$W/src/d2.json"; paths 'src/d2.json\n'; run --paths-from "$C/p.txt" "attribute binary json"; eq "18.46d a .json that the repository attributes mark binary owes CHECKS_NOT_JUDGED: 14" "$RC" 14; has "18.46d named" "$(jq -c .unjudged_paths "$REP" 2>/dev/null)" check_json
mk k5; mkdir -p "$W/nested" && git init -q "$W/nested"; ln -s nowhere "$W/src/dangling"; chg
run --commit-before-integrate --paths-from "$C/p.txt" "a"; hasnot "18.47 --commit-before-integrate with an untracked nested repository and a dangling symlink is no backup_failed (it used to be)" "$OUT" backup_failed
has "18.47 the nested repository and the link are recorded in the backup" "$(cat "$RD/backup/s1a/nested.tsv" 2>/dev/null)" "nested_repo	nested"; has "18.47b and the dangling link" "$(cat "$RD/backup/s1a/nested.tsv" 2>/dev/null)" "symlink	src/dangling	nowhere"
mk k5b; git -C "$W" mv src/z.txt src/z2.txt; chg; run --commit-before-integrate --paths-from "$C/p.txt" "a"
has "18.47c a staged rename: the new path is backed up" "$(cat "$RD/backup/s1a/worktree.sha256" 2>/dev/null)" "src/z2.txt"; hasnot "18.47c and no garbled old path" "$(cat "$RD/backup/s1a/worktree.sha256" 2>/dev/null)" "_name"
mk k5c; echo old > "$W/aaaREADME.txt"; git -C "$W" add -A; git -C "$W" commit -qm "odd name"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; git -C "$W" mv aaaREADME.txt b.txt; chg; run --commit-before-integrate --paths-from "$C/p.txt" "a"
has "18.47d a staged rename whose old path: the new path is backed up" "$(cat "$RD/backup/s1a/worktree.sha256" 2>/dev/null)" "b.txt"; hasnot "18.47d a staged rename whose old path minus three characters names an UNCHANGED file: that file is no part of the backup" "$(cat "$RD/backup/s1a/worktree.sha256" 2>/dev/null)" "README.txt"
mk k6; chg; XENV="GITSHIM_FAIL_ON=ls-files.-s.-z$" run --paths-from "$C/p.txt" "a"; XENV=""; eq "18.48 a gitlink listing that fails is 20 git_listing_failed, never 'no submodules'" "$RC" 20; has "18.48 the reason" "$OUT" git_listing_failed
mk k6b; chg; XENV="GITSHIM_FAIL_ON=rev-list.refs/heads/main" run --commit-before-integrate --paths-from "$C/p.txt" "a"; XENV=""; eq "18.48b a failing outgoing-commit listing at S1a is 20, never an empty range" "$RC" 20; has "18.48b git_listing_failed" "$OUT" git_listing_failed
mk n1; git -C "$W" mv mods/sub "mods/my sub" 2>/dev/null; git -C "$W" commit -qm rename; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; echo r > "$W/mods/my sub/r.txt"; printf 'mods/my sub/r.txt\n' > "$C/p.txt"
run --repo "mods/my sub" --paths-from "$C/p.txt" "sub with a space"; eq "18.49 --repo of a submodule whose path holds a space: 0 (it used to be repo_not_found)" "$RC" 0
mk n2; git -C "$W" tag main 2>/dev/null; chg; run --paths-from "$C/p.txt" "a"; okrun "18.50 a tag named like the branch does not break it (it used to be wrong_branch)"
mk i1; paths 'src/a.txt\n'; run --awaits-review "$EVR/reviews/WP-Z.json"; eq "18.51 --awaits-review without --paths-from: 20" "$RC" 20; run "message only"; eq "18.51b a message without --paths-from: 20" "$RC" 20
mk i2; chg; run --repo mods/sub --repo mods/ext --paths-from "$C/p.txt" "a"; eq "18.52 the same option twice: 20 usage_error" "$RC" 20; has "18.52 given twice" "$OUT" "given twice"
run --run-id a1 --run-id a2 --paths-from "$C/p.txt" "a"; eq "18.52b cpa-host --run-id twice: 20" "$RC" 20
run --paths-from "$C/p.txt" -- "--run-id"; okrun "18.53 a message equal to --run-id after -- is a message"
mk i3; printf 'src/a.txt\t\n' > "$C/p.txt"; echo a > "$W/src/a.txt"; run --paths-from "$C/p.txt" "a"; eq "18.54 a paths line with a TAB and an empty verdict: 20" "$RC" 20
mk i4; chg; mkdir -p "$W/.audit/merge-resolution/x"; run --resolve-merge "$W/.audit/merge-resolution/x" --paths-from "$C/p.txt" "a"; eq "18.55 --resolve-merge when no merge ran: 20" "$RC" 20
mk m1; chg; run --paths-from "$C/p.txt" $'a\n\nAwaits-Review: specs/x.json'; eq "18.56 a message line that reads like a trailer: 20" "$RC" 20; has "18.56 usage_error" "$OUT" message_impersonates_trailer
run --paths-from "$C/p.txt" $'b\n\nDeferred-Gates: x'; eq "18.56b a Deferred-Gates line: 20" "$RC" 20
mk m3; foreign src/f1.txt 'f\n'; F1="$FOREIGN"; foreign src/f2.txt 'g\n'; git -C "$C/f" commit -q --allow-empty -m "$(printf 'body\n\nForeign-Commit: %s\n\nmore' "$F1")"; git -C "$C/f" push -q "$C/fx/a.git" main; git -C "$C/f" push -q "$C/fx/b.git" main
chg; run --paths-from "$C/p.txt" "own change"; okrun "18.57 an own change after foreign commits"; has "18.57 the first commit names the foreign commit although a body line of another commit spells it" "$(msg)" "Foreign-Commit: $F1"
mk v2; mkdir -p "$W/$EVR/reviews"; VX="$EVR/reviews/WP-S.json"; printf '{"schema":"review-verdict/1","verdict":"GO","covers_runs":[],"model":"opus","effort":"xhigh","blocking_findings":3}\n' > "$W/$EVR/reviews/WP-S.json"; paths "$VX\n"; run --paths-from "$C/p.txt" "a schema-invalid GO"
echo h > "$W/src/h.txt"; paths "src/h.txt\t$VX\n"; run --paths-from "$C/p.txt" "hold on it"; eq "18.58 a hold on a verdict that says GO but is schema-invalid is a legal hold: 14, not verdict_already_go" "$RC" 14
mk v5; mkdir -p "$C/nojs"; cat > "$C/nojs/python3" <<'NOJS'
#!/bin/sh
# a python3 without the jsonschema module for verdict_go.py only (every other python of the run is the genuine one)
case "$2" in
  */verdict_go.py) shift; exec /usr/bin/python3 -I -c 'import sys, runpy; sys.modules["jsonschema"] = None; s = sys.argv[1]; sys.argv = sys.argv[1:]; runpy.run_path(s, run_name="__main__")' "$@" ;;
  *) exec /usr/bin/python3 "$@" ;;
esac
NOJS
chmod 755 "$C/nojs/python3"
mkdir -p "$W/$EVR/reviews"; VF="$EVR/reviews/WP-F.json"; echo h > "$W/src/h.txt"; paths "src/h.txt\t$VF\n"; run --paths-from "$C/p.txt" "held"; HRF="$RUN"
printf '{"schema":"review-verdict/1","verdict":"GO","covers_runs":[{"repository":".","commit":"0000000000000000000000000000000000000000","cpa_run":"%s"}],"model":"opus","effort":"xhigh","blocking_findings":false}\n' "$HRF" > "$W/$EVR/reviews/WP-F.json"; paths "$VF\n"
runenv "PATH=$C/nojs:$C/shimr:$PATH" -- --paths-from "$C/p.txt" "a verdict with blocking_findings false"; hasnot "18.59 with no jsonschema the hold is NEVER released on a reduced check (origin lacks the held commit)" "$(git -C "$C/fx/a.git" log --format=%s main | head -3 | tr '\n' ' ')" "held"
mk b3; ( cd "$W" && envr $(ENVV) timeout 10 "$C/bin/cpa-host" --paths-from /dev/zero "m" ) >"$C/out.txt" 2>"$C/err.txt"; eq "18.60 --paths-from /dev/zero is refused at once: 20" "$?" 20
mk st1; mkdir -p "$W/.audit"; A1="$(git -C "$W/mods/sub" rev-parse HEAD)"; git -C "$W/mods/sub" commit -q --allow-empty -m two; B1="$(git -C "$W/mods/sub" rev-parse HEAD)"; git -C "$W/mods/sub" push -q origin main; git -C "$W" add mods/sub; git -C "$W" commit -qm pin; git -C "$W" push -q origin main; git -C "$W" push -q mirror main
git -C "$W/mods/sub" checkout -q "$A1"; printf 'mods/sub\t%s\tsomerun\n' "$A1" > "$W/.audit/pending_pins.tsv"; trust_build "$C"; run
eq "18.61 a pending pin row BEHIND the pin the parent records is stale: 15 pointer_drift, never 0" "$RC" 15; eq "18.61 the stale row was dropped" "$(awk -F'\t' '$1=="mods/sub"' "$W/.audit/pending_pins.tsv" | wc -l | tr -d ' ')" 0
mk en1; chg; SKIP_LONG=$'a\rb' run --paths-from "$C/p.txt" "a"; eq "18.62 a control character in SKIP_LONG is refused: 20" "$RC" 20
mk f1; mkfifo "$W/src/fifo"; paths 'src/fifo\n'; run --paths-from "$C/p.txt" "a fifo"
has "18.67 a declared FIFO is reported not_judged non_regular, never read as a deletion" "$(cat "$RD/validate.out" 2>/dev/null)" "not_judged	non_regular	src/fifo"; eq "18.67 and nothing is committed or pushed for it: 20" "$RC" 20
# a shell-level failure (status 127: a command not found, an unreadable file) after the run opened is a 20 with the status in the detail (a shim of lib_safe.sh makes a function the main shell calls directly at S0 end the shell with 127)
mk w14; printf '\nsafe_declpath() { exit 127; }\n' >> "$W/scripts/repo/lib_safe.sh"; git -C "$W" add -A; git -C "$W" commit -qm "lib shim"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"; chg
run --paths-from "$C/p.txt" "a"; eq "18.63 a shell-level 127 after the run opened: 20, never the shell's 127" "$RC" 20; eq "18.63 the report says internal_error with the status" "$(jq -r '.reason + ":" + .detail' "$REP" 2>/dev/null)" "internal_error:exit 127"
# a run directory whose state cannot be read is a 20, never "not live" (the predicate is read in a command substitution, where a set -u error ends only the substitution)
mk w14b; mkdir -p "$W/.audit/commit-push/20260101T000000Z-1-other"; echo '{}' > "$W/.audit/commit-push/20260101T000000Z-1-other/report.json"
printf '\ncpa_rundir_state() { : "${CPA_TEST_UNSET_VARIABLE}"; echo finished; }\n' >> "$W/scripts/repo/lib_safe.sh"; git -C "$W" add -A; git -C "$W" commit -qm "lib shim"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main; trust_build "$C"
run; eq "18.63b a run state that cannot be read: 20" "$RC" 20; eq "18.63b the report says internal_error" "$(jq -r .reason "$REP" 2>/dev/null)" internal_error; has "18.63b and names the run directory whose state was unreadable" "$(jq -r .detail "$REP" 2>/dev/null)" "could not be read"
# CHECKS_NOT_JUDGED is recorded although a registry row is pending (validate_cheap gives 14, not 0)
mk w15; printf 'extra\tscripts/repo/check_no_ci.sh --root /nonexistent\tIMG-TESTUTIL\tplain\t-\tchangeset\tnot in the approved copy\n' >> "$W/scripts/repo/validate_checks.tsv"; git -C "$W" add -A; git -C "$W" commit -qm "registry row"; git -C "$W" push -q origin main; git -C "$W" push -q mirror main
ln -s ../README.txt "$W/src/link"; paths 'src/link\n'; run --paths-from "$C/p.txt" "symlink and a pending row"
eq "18.64 a declared symlink and a pending registry row: 14" "$RC" 14; has "18.64 CHECKS_NOT_JUDGED is recorded although validate_cheap gave 14" "$(jq -c .deferred_gates "$REP" 2>/dev/null)" CHECKS_NOT_JUDGED
# the sweep judges the MAIN repository, also in a --repo run
mk w16; hshim scripts/anti-mess/sweep.sh 'echo "$ANTIMESS_ROOT" >> "'"$C"'/sweep.roots"; exit 0'; : > "$C/sweep.roots"; echo r > "$W/mods/sub/r.txt"; printf 'mods/sub/r.txt\n' > "$C/p.txt"; run --repo mods/sub --paths-from "$C/p.txt" "sub work"
eq "18.65 a --repo run: the sweep is rooted at the MAIN repository at S0 and at S7 (CM15: not at the submodule)" "$(sort -u "$C/sweep.roots" | tr '\n' ' ')" "$(cd "$W" && pwd -P) "
# the gate lists are read at S0: a merge commit made at S1 carries GATES_NOT_BUILT for an absent long-gate list
OMIT=scripts/repo/long_gates.txt mk w17; echo l > "$W/src/l.txt"; paths 'src/l.txt\n'; run --local-only --paths-from "$C/p.txt" "local"; foreign src/f.txt 'f\n'; run
eq "18.66 control: the run made a merge commit" "$(git -C "$W" rev-list --parents -n1 HEAD | wc -w | tr -d ' ')" 3; has "18.66 the S1 merge commit carries GATES_NOT_BUILT (an absent long-gate list, recorded at S0)" "$(msg)" GATES_NOT_BUILT
# REPORT-4: a held remainder explains verifier 11 only when NOTHING is unproven; a remote that cannot be read after the push is 11 remote_unproven, never review_pending
mk h68; V="$EVR/reviews/WP-X.json"; echo u > "$W/src/u.txt"; echo h > "$W/src/h.txt"; paths "src/u.txt\nsrc/h.txt\t$V\n"
XENV="GITSHIM_FAIL_AFTER_PUSH=ls-remote" run --paths-from "$C/p.txt" "unheld and held, remotes unreadable after the push"; XENV=""
eq "18.68 a held run whose remotes cannot be read after the push: 11, not 14 (REPORT-4)" "$RC" 11; has "18.68 remote_unproven" "$OUT" remote_unproven
fi

# ======== 11 static properties ================================================================================================================
fscan() { # fscan <file>: the non-comment source lines that spell a forbidden git form (the refusal guards of the helpers, which carry `force_refused`, are not spellings of the form)
  grep -nE -- '--force|force-with-lease|--no-verify|--mirror|--delete|update-ref|--amend|(^|[^A-Za-z_-])rebase( |"|$)|git[^|;]* reset( |")|git[^|;]*[[:space:]](push|fetch)([[:space:]][^|;]*)?[[:space:]]-[a-zA-Z]*f[a-zA-Z]*([[:space:]]|$)|git[^|;]*[[:space:]]checkout[[:space:]][^|;]*-[a-zA-Z]*B|git[^|;]*[[:space:]]branch[[:space:]][^|;]*-[a-zA-Z]*[fFD]([[:space:]]|$)|git[^|;]*[[:space:]](push|fetch)[[:space:]][^|;]*[[:space:]"'"'"'=]\+[A-Za-z$\{]|[[:space:]]\+refs/|[[:space:]]\+HEAD|git[^|;]*[[:space:]]push[[:space:]][^|;]*[[:space:]]:[A-Za-z$"]' "$1" | grep -vE '^[0-9]+:[[:space:]]*#' | grep -v 'force_refused'
}
if sect "11 static"; then
# 11.0 control needles: both oracles see every forbidden spelling and let the spellings the scripts really use pass (a blind oracle reads as a clean run)
n=0; m=0; fl="$T/oracle.log"
for line in '-C r push --force -- o x' '-C r push -f -- o x' '-C r push -qf -- o x' '-C r push -q --force-with-lease -- o x' '-C r push -q -- o +x:refs/heads/main' '-C r push -q -- o +refs/heads/main' '-C r push -q --mirror o' \
  '-C r push -q -- o :refs/heads/main' '-C r push -q --delete o main' '-C r push -q -- o x:+refs/heads/main' 'commit --amend -m x' 'branch -f main abc' 'checkout -q -B main' 'update-ref refs/heads/main abc' 'rebase origin/main' 'reset --hard' 'commit --no-verify -m x' 'fetch -f o main'; do
  m=$((m+1)); printf '%s\n' "$line" > "$fl"; [ -n "$(forbidden_calls "$fl")" ] && n=$((n+1)) || bad "11.0 shim oracle misses: $line"
done; eq "11.0 the shim oracle flags every forbidden spelling ($m)" "$n" "$m"
n=0; m=0
for line in '-C r push -q -- origin abc:refs/heads/main' '-C r merge --ff-only abc' 'merge --no-ff --no-commit abc' 'commit -q --only -F x -- p' '-C r fetch -q --no-tags --no-write-fetch-head --refmap= -- o main' 'ls-remote -- o refs/heads/main' '-C r checkout -q main' 'branch --show-current' 'bundle create b main'; do
  m=$((m+1)); printf '%s\n' "$line" > "$fl"; [ -z "$(forbidden_calls "$fl")" ] && n=$((n+1)) || bad "11.0 shim oracle flags a plain call: $line"
done; eq "11.0 the shim oracle passes the plain calls the scripts use ($m)" "$n" "$m"
n=0; m=0; sf="$T/oracle.sh"
for line in 'git -C "$d" push --force -- "$r" "$x"' 'git -C "$d" push -f -- "$r" "$x"' 'tm git -C "$dir" push -qf -- "$r" "$t:refs/heads/$BR"' 'git push -- "$r" "+$t:refs/heads/$BR"' 'git push "$r" +refs/heads/x' 'git push --mirror "$r"' 'git push "$r" :refs/heads/x' 'git push --delete "$r" x' 'git commit --amend' 'git branch -f x y' 'git checkout -B x' 'git update-ref x y' 'git rebase x' 'git reset --hard' 'git commit --no-verify' 'git push --force-with-lease'; do
  m=$((m+1)); printf '%s\n' "$line" > "$sf"; [ -n "$(fscan "$sf")" ] && n=$((n+1)) || bad "11.0 static scan misses: $line"
done; eq "11.0 the static scan flags every forbidden spelling, quoted +refspec included ($m)" "$n" "$m"
n=0; m=0
for line in 'tm git -C "$dir" push -q -- "$r" "$target:refs/heads/$BR"' 'git -C "$d" merge --ff-only "$t"' 'git config -f "$sup/.gitmodules" --get x' '# git push --force' 'jq -n "{fetch:(\"fetch_failed:\"+\$n)}"' 'case "$a" in --force|-f|+*) die force_refused "$a" ;; esac'; do
  m=$((m+1)); printf '%s\n' "$line" > "$sf"; [ -z "$(fscan "$sf")" ] && n=$((n+1)) || bad "11.0 static scan flags an innocent line: $line"
done; eq "11.0 the static scan passes the lines the scripts really hold ($m)" "$n" "$m"
for f in "$H" "$SRC"/scripts/repo/host_entry/cpa-host "$SRC"/scripts/repo/*.sh "$SRC"/scripts/longops/*.sh; do
  [ -f "$f" ] || { bad "11.1 $f is absent"; continue; }
  hit="$(fscan "$f" | head -2)"
  [ -z "$hit" ] && ok "11.1 $(basename "$f") has no force/lease/+refspec/mirror/delete/:ref/update-ref/amend/no-verify/reset/rebase/branch -f/checkout -B spelling" || bad "11.1 $(basename "$f") spells a forbidden git form: $hit"
  bash -n "$f" 2>/dev/null && ok "11.2 $(basename "$f") parses" || bad "11.2 $(basename "$f") does not parse"
done
# 11.3 the owned set has ONE source, the approved scripts/audit/own_orgs.txt (a caller-environment override is an undocumented escape hatch; WF11 F1)
grep -c 'CPA_RUN_ID' "$H" | grep -qv '^0$' && ok "11.3 control needle: the scan sees a token that is there" || bad "11.3 the scan is blind"
eq "11.3 the script reads no caller-environment owned-organisation override" "$(grep -c 'CPA_OWNED_ORGS' "$H")" 0
# 11.4 the environment scrub is ONE text in the host entry and in the copied script (a caller's variables are dropped before any git call, WF14 N1)
blk() { sed -n '/^# --- cpa env scrub begin ---$/,/^# --- cpa env scrub end ---$/p' "$1"; }
hb="$(blk "$SRC/scripts/repo/host_entry/cpa-host")"; cb="$(blk "$H")"
[ -n "$hb" ] && ok "11.4 control needle: the host entry holds a scrub block" || bad "11.4 the host entry holds no scrub block (the extraction is blind or the block is gone)"
eq "11.4 the host entry's and the copied script's scrub blocks are the same text" "$hb" "$cb"
sb() { sed -n '/^# --- cpa signals begin ---$/,/^# --- cpa signals end ---$/p' "$1"; }
[ -n "$(sb "$H")" ] && ok "11.4 control needle: the copied script holds a signals block" || bad "11.4 the copied script holds no signals block"
eq "11.4 the host entry's and the copied script's signal blocks are the same text" "$(sb "$SRC/scripts/repo/host_entry/cpa-host")" "$(sb "$H")"
# 11.5 the environment is an ALLOWLIST (WF17-cpa): the class is closed by the shape of the block, by the interpreter and by the environ oracle of matrix 18.10, not by a text inventory of ${NAME:-} reads
# (which was blind to every variable that bash, the loader, python and git read for themselves)
has "11.5 control needle: the block holds the allowlist" "$hb" "CPA_ENV_ALLOW="
hasnot "11.5 the allowlist names no family wildcard" "$(printf '%s' "$hb" | grep '^CPA_ENV_ALLOW=')" "*"
sh=""; for f in "$H" "$SRC"/scripts/repo/host_entry/cpa-host "$SRC"/scripts/repo/*.sh; do [ "$(head -1 "$f")" = '#!/bin/bash -p' ] || sh="$sh $(basename "$f")"; done
eq "11.5 every script of a run starts with #!/bin/bash -p (imported functions, SHELLOPTS, BASHOPTS, BASH_ENV and ENV are ignored by bash itself; an absolute interpreter cannot be replaced through PATH; unaccounted:${sh:- none})" "${sh:- none}" " none"
for f in "$H" "$SRC"/scripts/repo/host_entry/cpa-host; do
  first="$(sed -n '/^set -u$/,$p' "$f" | grep -v '^#' | grep -v '^$' | sed -n 2p)"
  case "$first" in 'CPA_SIGS=(); while read'*) ok "11.5 $(basename "$f"): the signal block is the first statement after set -u (no start-up interval without a handler)" ;; *) bad "11.5 $(basename "$f"): the first statement after set -u is '${first:0:60}'" ;; esac
done
py=""; for f in "$H" "$SRC"/scripts/repo/host_entry/cpa-host "$SRC"/scripts/repo/*.sh; do
  hit="$(grep -nE 'python3( |$)' "$f" | grep -vE '^[0-9]+:[[:space:]]*#|command -v python3|python3 -I|INTERPRETERS|bash, python3|python3 needs|python3, sha256sum|python3 required|word python3' | head -2)"; [ -z "$hit" ] || py="$py $(basename "$f"):${hit%%:*}"
done
eq "11.5 every python3 of a run is isolated (-I: no PYTHON* variable, no working-directory module; unaccounted:${py:- none})" "${py:- none}" " none"
# 11.6 the inventory of the row kinds validate_cheap.sh prints: each is decided by its exit status, informational by design, or read by name in the script (WF14 N4: a row that
# means "not judged" must never be a kind that only the helper's header knows)
vk="$(grep -oE "emit\('[a-z_]+'" "$SRC/scripts/repo/validate_cheap.sh" | sed "s/emit('//;s/'//" | sort -u | tr '\n' ' ')"
has "11.6 control needle: the inventory sees the row kinds validate_cheap prints" "$vk" left_out; has "11.6 and the one WF14 added" "$vk" not_judged
un=""; for k in $vk; do
  case "$k" in fail|class_table_unreviewed|legacy_row_not_dropped|table_admits_unheld_path|check_pending_release|size_alarm) continue ;; esac
  grep -qE "\\\$1==\"$k\"" "$H" || un="$un $k"
done
eq "11.6 every row kind of validate_cheap.sh is decided by its exit status, informational (size_alarm), or read by name by the script (unread:${un:- none})" "${un:- none}" " none"
fi
fin
