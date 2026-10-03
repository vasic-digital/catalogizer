#!/usr/bin/env bash
# verify_repo.sh - recursive, read-only repository verifier (POC for spec 001, FR-001..FR-003 style checks).
#
# For the main repository and EVERY submodule at EVERY depth it reports:
#   * working-tree dirtiness  (git status --porcelain: tracked changes vs untracked)
#   * pin state               (first column of `git submodule status`: ' ' ok, '+' drifted, '-' uninitialised, 'U' conflict)
#   * for OWNED repos on a branch: each remote's branch tip (git ls-remote) compared with local HEAD
#         SAME            remote tip == local HEAD
#         REMOTE-BEHIND   remote tip is an ancestor of local HEAD (local is AHEAD: unpushed commits)
#         LOCAL-BEHIND    local HEAD is an ancestor of the remote tip (remote has newer commits)
#         DIVERGED        neither is an ancestor of the other
#         UNREACHABLE     git ls-remote failed / timed out
#         NO-REMOTE-BRANCH remote answered but has no such branch
#         UNKNOWN-DIFFERENT remote tip differs and its commit object is not present locally;
#                          ancestry cannot be decided WITHOUT --fetch (never guessed)
#
# Read-only by default: no fetch, no checkout, no write to any repository. With --fetch it runs
# `git fetch --no-tags <remote> <branch>` (object store only, worktree and refs/heads untouched)
# so that the ancestry questions can be answered.  Needs no `git submodule foreach` (which aborts
# on the first non-zero child); every repo is processed independently.
#
# Exit codes: 0 all clear | 1 any repo dirty, ahead, diverged (or --strict: also behind/unreachable/unknown/pin drift)
#             2 usage error | 3 nothing failed but at least one remote comparison is unproven
#             (UNREACHABLE / UNKNOWN-DIFFERENT / NO-REMOTE-BRANCH) - "could not verify" is never reported as clean.
#
# Usage: verify_repo.sh [--root DIR] [--json-out FILE] [--fetch] [--no-remote] [--strict] [--jobs N]
#                       [--timeout SEC] [--owned-orgs a,b,c] [--exceptions FILE] [--quiet] [--self-test]
# Env:   VERIFY_GIT (git binary or shim, default: git)
set -u
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
GIT="${VERIFY_GIT:-git}"
ROOT="$(pwd)"; JSON_OUT=""; DO_FETCH=0; NO_REMOTE=0; STRICT=0; JOBS=6; TMO=25; QUIET=0
OWNED_ORGS="vasic-digital,HelixDevelopment,milos85vasic"
EXC_FILE="$(dirname "$SELF")/exceptions.tsv"

usage() { sed -n '2,32p' "$SELF" | sed 's/^# \{0,1\}//'; }

# ---------------------------------------------------------------- worker (one repo -> one JSON file)
worker() {
  local rel="$1" pin="$2" outdir="$3" abs head branch porcelain tracked untracked remotes r url owned org
  [ "$rel" = "." ] && abs="$ROOT" || abs="$ROOT/$rel"
  head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)"
  branch="$($GIT -C "$abs" symbolic-ref --short -q HEAD 2>/dev/null)"
  # submodule state is judged on its own row (dirty/pin), so it is not double counted in the parent
  porcelain="$($GIT -C "$abs" status --porcelain --ignore-submodules=all 2>/dev/null)"
  tracked="$(printf '%s\n' "$porcelain" | grep -v '^??' | grep -c . )"
  untracked="$(printf '%s\n' "$porcelain" | grep -c '^??')"
  owned=false
  local rj="[]"
  remotes="$($GIT -C "$abs" remote 2>/dev/null)"
  for r in $remotes; do
    url="$($GIT -C "$abs" remote get-url "$r" 2>/dev/null)"
    org="$(printf '%s' "$url" | sed -E 's#^.*[:/]([^/:]+)/[^/]+$#\1#')"
    case ",$OWNED_ORGS," in *",$org,"*) owned=true ;; esac
  done
  if [ "$owned" = true ] && [ -n "$branch" ] && [ "$NO_REMOTE" = 0 ] && [ -n "$head" ]; then
    for r in $remotes; do
      url="$($GIT -C "$abs" remote get-url "$r" 2>/dev/null)"
      local out rc tip cls
      out="$(GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="ssh -o BatchMode=yes -o ConnectTimeout=10" \
             timeout "$TMO" $GIT -C "$abs" ls-remote "$r" "refs/heads/$branch" 2>/dev/null)"; rc=$?
      # ssh banners / MOTD noise may precede the answer: accept ONLY "<40 hex><ws>" lines.
      tip="$(printf '%s\n' "$out" | grep -E '^[0-9a-f]{40}[[:space:]]' | head -1 | cut -c1-40)"
      if [ "$rc" -ne 0 ]; then cls="UNREACHABLE"
      elif [ -z "$tip" ]; then cls="NO-REMOTE-BRANCH"
      elif [ "$tip" = "$head" ]; then cls="SAME"
      else
        if [ "$DO_FETCH" = 1 ]; then
          GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="ssh -o BatchMode=yes -o ConnectTimeout=10" \
            timeout "$TMO" $GIT -C "$abs" fetch --no-tags --quiet "$r" "$branch" >/dev/null 2>&1
        fi
        if ! $GIT -C "$abs" cat-file -e "${tip}^{commit}" 2>/dev/null; then cls="UNKNOWN-DIFFERENT"
        elif $GIT -C "$abs" merge-base --is-ancestor "$tip" "$head" 2>/dev/null; then cls="REMOTE-BEHIND"
        elif $GIT -C "$abs" merge-base --is-ancestor "$head" "$tip" 2>/dev/null; then cls="LOCAL-BEHIND"
        else cls="DIVERGED"; fi
      fi
      rj="$(jq -c --arg r "$r" --arg u "$url" --arg t "$tip" --arg c "$cls" \
            '. + [{remote:$r,url:$u,remote_tip:$t,class:$c}]' <<<"$rj")"
    done
  fi
  jq -nc --arg path "$rel" --arg pin "$pin" --arg head "$head" --arg branch "$branch" \
     --argjson owned "$owned" --argjson tracked "$tracked" --argjson untracked "$untracked" \
     --argjson remotes "$rj" \
     '{path:$path,pin:$pin,head:$head,branch:$branch,owned:$owned,dirty_tracked:$tracked,
       dirty_untracked:$untracked,dirty:(($tracked+$untracked)>0),remotes:$remotes}' \
     > "$outdir/$(printf '%s' "$rel" | tr '/.' '__').json"
}

if [ "${1:-}" = "--worker" ]; then
  shift; ROOT="$1"; shift; DO_FETCH="$1"; shift; NO_REMOTE="$1"; shift; TMO="$1"; shift; OWNED_ORGS="$1"; shift
  worker "$@"; exit 0
fi

# ---------------------------------------------------------------- self-test
self_test() {
  local T fails=0 pass=0
  T="$(mktemp -d)"; trap 'rm -rf "$T"' RETURN
  ok()   { pass=$((pass+1)); echo "  PASS  $1"; }
  bad()  { fails=$((fails+1)); echo "  FAIL  $1"; }
  expect() { # expect "<name>" "<actual>" "<wanted>"
    if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2' wanted '$3')"; fi; }
  local G="git -c user.email=t@t -c user.name=t -c protocol.file.allow=always"
  mk() { # mk <name>: bare remote + clone with 1 commit, owned-looking url via insteadOf is not needed: use OWNED_ORGS=fx
    ( cd "$T" && git init -q --bare -b main "r_$1.git" && git clone -q "r_$1.git" "w_$1" 2>/dev/null &&
      cd "w_$1" && git checkout -q -b main 2>/dev/null; echo a>f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null ); }
  fx() { # run verifier on a fixture; $1 = workdir, extra args follow; owned org is the directory name of the remote parent => use 'fx' via url rewrite
    local w="$1"; shift
    ( cd "$w" && git remote set-url origin "$T/fx/r_$(basename "$w" | sed s/^w_//).git" ) 2>/dev/null
    "$SELF" --root "$w" --owned-orgs fx --exceptions /dev/null --quiet --jobs 2 --timeout 15 "$@"; }
  mkdir -p "$T/fx"
  # Fixtures live under $T/fx/<name>.git so that the url org segment is "fx" -> owned.
  mk0() { ( cd "$T/fx" && git init -q --bare -b main "r_$1.git" && git clone -q "r_$1.git" "$T/w_$1" 2>/dev/null
            cd "$T/w_$1" && git checkout -q -b main 2>/dev/null; echo a>f; git add f; $G commit -qm c1; git push -q origin main 2>/dev/null ); }
  for n in clean dirty ahead behind diverged unreach; do mk0 $n; done
  echo "fixtures: $(ls "$T"/w_* -d | wc -l) repos"

  # golden-good: clean + synchronised
  local o rc
  o="$(fx "$T/w_clean" --json-out "$T/clean.json")"; rc=$?
  expect "golden-good clean exit" "$rc" 0
  expect "golden-good class" "$(jq -r '.repos[0].remotes[0].class' "$T/clean.json")" SAME
  # human table renders (non-quiet path) and carries the class
  o="$("$SELF" --root "$T/w_clean" --owned-orgs fx --exceptions /dev/null --timeout 15 2>&1)"
  case "$o" in *"PATH"*"origin:SAME"*) ok "human table renders with remote class";; *) bad "human table missing: $o";; esac
  # golden-bad 1: dirty tracked file
  echo b >> "$T/w_dirty/f"
  fx "$T/w_dirty" --json-out "$T/dirty.json" >/dev/null; rc=$?
  expect "golden-bad dirty exit" "$rc" 1
  expect "golden-bad dirty flag" "$(jq -r '.repos[0].dirty_tracked' "$T/dirty.json")" 1
  # golden-bad 2: ahead (unpushed commit) => REMOTE-BEHIND
  ( cd "$T/w_ahead" && echo x>g && git add g && $G commit -qm c2 )
  fx "$T/w_ahead" --json-out "$T/ahead.json" >/dev/null; rc=$?
  expect "golden-bad ahead exit" "$rc" 1
  expect "golden-bad ahead class" "$(jq -r '.repos[0].remotes[0].class' "$T/ahead.json")" REMOTE-BEHIND
  # golden-bad 3: local behind. Another clone pushes c2; local object absent -> UNKNOWN-DIFFERENT w/o fetch, LOCAL-BEHIND with --fetch
  ( git clone -q "$T/fx/r_behind.git" "$T/other_behind" 2>/dev/null && cd "$T/other_behind" && echo y>h && git add h && $G commit -qm c2 && git push -q origin main 2>/dev/null )
  fx "$T/w_behind" --json-out "$T/b1.json" >/dev/null; rc=$?
  expect "behind w/o fetch is not guessed" "$(jq -r '.repos[0].remotes[0].class' "$T/b1.json")" UNKNOWN-DIFFERENT
  expect "behind w/o fetch exit (unproven)" "$rc" 3
  fx "$T/w_behind" --fetch --json-out "$T/b2.json" >/dev/null; rc=$?
  expect "behind with --fetch class" "$(jq -r '.repos[0].remotes[0].class' "$T/b2.json")" LOCAL-BEHIND
  expect "behind exit (not in fail set)" "$rc" 0
  fx "$T/w_behind" --fetch --strict --json-out "$T/b3.json" >/dev/null; rc=$?
  expect "behind with --strict exit" "$rc" 1
  # golden-bad 4: diverged
  ( git clone -q "$T/fx/r_diverged.git" "$T/other_div" 2>/dev/null && cd "$T/other_div" && echo y>h && git add h && $G commit -qm r2 && git push -q origin main 2>/dev/null )
  ( cd "$T/w_diverged" && echo x>g && git add g && $G commit -qm l2 )
  fx "$T/w_diverged" --fetch --json-out "$T/d.json" >/dev/null; rc=$?
  expect "golden-bad diverged class" "$(jq -r '.repos[0].remotes[0].class' "$T/d.json")" DIVERGED
  expect "golden-bad diverged exit" "$rc" 1
  # unreachable remote must never read as SAME
  ( cd "$T/w_unreach" && git remote set-url origin "$T/fx/does_not_exist.git" )
  "$SELF" --root "$T/w_unreach" --owned-orgs fx --exceptions /dev/null --quiet --timeout 10 --json-out "$T/u.json" >/dev/null; rc=$?
  expect "unreachable class" "$(jq -r '.repos[0].remotes[0].class' "$T/u.json")" UNREACHABLE
  expect "unreachable exit (unproven)" "$rc" 3
  # NEGATIVE CONTROL 1 (control needle, 11.4.201(7)b): ssh banner noise before the answer must still parse as SAME,
  # and an answer that is ONLY noise must be NO-REMOTE-BRANCH, never SAME.
  cat > "$T/shim_noise.sh" <<'SHIM'
#!/usr/bin/env bash
# git shim: prints banner noise before genuine ls-remote output
real=git; args=("$@")
for a in "${args[@]}"; do if [ "$a" = "ls-remote" ]; then
  echo "Welcome to fake-host MOTD 0123456789abcdef0123456789abcdef01234567 not-a-ref"
  echo "Warning: Permanently added 'x' to the list of known hosts."
fi; done
exec git "$@"
SHIM
  chmod +x "$T/shim_noise.sh"
  VERIFY_GIT="$T/shim_noise.sh" fx "$T/w_clean" --json-out "$T/n1.json" >/dev/null; rc=$?
  expect "control: banner noise ignored -> still SAME" "$(jq -r '.repos[0].remotes[0].class' "$T/n1.json")" SAME
  cat > "$T/shim_onlynoise.sh" <<'SHIM'
#!/usr/bin/env bash
for a in "$@"; do if [ "$a" = "ls-remote" ]; then echo "banner only"; echo "0123456789abcdef0123456789abcdef0123456 short-not-40"; exit 0; fi; done
exec git "$@"
SHIM
  chmod +x "$T/shim_onlynoise.sh"
  VERIFY_GIT="$T/shim_onlynoise.sh" fx "$T/w_clean" --json-out "$T/n2.json" >/dev/null; rc=$?
  expect "control: noise-only answer is not SAME" "$(jq -r '.repos[0].remotes[0].class' "$T/n2.json")" NO-REMOTE-BRANCH
  # NEGATIVE CONTROL 2: a clean repo must not abort the run when a sibling is processed (the foreach pitfall):
  # parent with two submodules, first clean, second dirty -> BOTH are listed and exit is 1.
  mkdir -p "$T/p" && ( cd "$T/p" && git init -q -b main . && echo p>p && git add p && $G commit -qm p )
  ( cd "$T/p" && $G -c protocol.file.allow=always submodule add -q "$T/fx/r_clean.git" s_a 2>/dev/null && $G -c protocol.file.allow=always submodule add -q "$T/fx/r_dirty.git" s_b 2>/dev/null && $G commit -qm subs )
  echo z >> "$T/p/s_b/f"
  "$SELF" --root "$T/p" --owned-orgs nomatch --exceptions /dev/null --quiet --no-remote --json-out "$T/p.json" >/dev/null; rc=$?
  expect "control: all repos listed (no foreach abort)" "$(jq -r '.repos|length' "$T/p.json")" 3
  expect "control: dirty sibling after clean one -> exit" "$rc" 1
  expect "control: dirty sub flagged" "$(jq -r '[.repos[]|select(.path=="s_b")|.dirty]|.[0]' "$T/p.json")" true
  expect "control: clean sub not flagged" "$(jq -r '[.repos[]|select(.path=="s_a")|.dirty]|.[0]' "$T/p.json")" false
  # exception list: same dirty sub excepted -> exit 0, flagged excepted
  printf 's_b\tdirty\tfixture CRLF quirk\n' > "$T/exc.tsv"
  "$SELF" --root "$T/p" --owned-orgs nomatch --exceptions "$T/exc.tsv" --quiet --no-remote --json-out "$T/p2.json" >/dev/null 2>&1; rc=$?
  expect "exception list honoured exit" "$rc" 0
  expect "exception recorded" "$(jq -r '[.repos[]|select(.path=="s_b")|.excepted]|.[0]' "$T/p2.json")" true
  echo "SELF-TEST: pass=$pass fail=$fails"
  [ "$fails" = 0 ]
}

# ---------------------------------------------------------------- arg parsing
while [ $# -gt 0 ]; do
  case "$1" in
    --root) ROOT="$(cd "$2" && pwd)"; shift 2 ;;
    --json-out) JSON_OUT="$2"; shift 2 ;;
    --fetch) DO_FETCH=1; shift ;;
    --no-remote) NO_REMOTE=1; shift ;;
    --strict) STRICT=1; shift ;;
    --jobs) JOBS="$2"; shift 2 ;;
    --timeout) TMO="$2"; shift 2 ;;
    --owned-orgs) OWNED_ORGS="$2"; shift 2 ;;
    --exceptions) EXC_FILE="$2"; shift 2 ;;
    --quiet) QUIET=1; shift ;;
    --self-test) self_test; exit $? ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage >&2; exit 2 ;;
  esac
done
command -v jq >/dev/null || { echo "jq required" >&2; exit 2; }
$GIT -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || { echo "not a git repo: $ROOT" >&2; exit 2; }

# ---------------------------------------------------------------- enumerate + run
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT
LIST="$WORK/list.tsv"
printf '.\t.\n' > "$LIST"
# first char of each `git submodule status` line is the pin marker (a leading space must NOT be stripped)
$GIT -C "$ROOT" submodule status --recursive 2>/dev/null | while IFS= read -r line; do
  pin="${line:0:1}"; rest="${line:1}"; set -- $rest; printf '%s\t%s\n' "${2:-}" "${pin:- }"
done >> "$LIST"
export ROOT DO_FETCH NO_REMOTE TMO OWNED_ORGS GIT
mkdir -p "$WORK/out"
# xargs -P: every repo handled independently; one failure never stops the others
awk -F'\t' '{printf "%s\0%s\0", $1, ($2==" "?"=":$2)}' "$LIST" | \
  xargs -0 -n 2 -P "$JOBS" bash -c '[ "$2" = "=" ] && p=" " || p="$2"; "'"$SELF"'" --worker "$ROOT" "$DO_FETCH" "$NO_REMOTE" "$TMO" "$OWNED_ORGS" "$1" "$p" "'"$WORK/out"'"' _
EXC="$WORK/exc.json"
if [ -f "$EXC_FILE" ]; then
  awk -F'\t' '$0!~/^#/ && NF>=2 {print}' "$EXC_FILE" | jq -R -s -c 'split("\n")|map(select(length>0)|split("\t")|{path:.[0],kind:.[1],reason:(.[2]//"")})' > "$EXC"
else echo '[]' > "$EXC"; fi

STRICT_J=$([ "$STRICT" = 1 ] && echo true || echo false)
jq -s --slurpfile exc "$EXC" --argjson strict "$STRICT_J" --arg root "$ROOT" \
   --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --argjson fetch "$([ "$DO_FETCH" = 1 ] && echo true || echo false)" '
  def exc(p;k): [$exc[0][]|select(.path==p and .kind==k)]|first;
  (map(.remotes|=map(.))|sort_by(.path)) as $repos |
  ($repos | map(
     . as $r |
     ($r.remotes|map(.class)) as $cl |
     ($r.pin == "+" or $r.pin == "U" or $r.pin == "-") as $pindrift |
     {path,pin,head,branch,owned,dirty,dirty_tracked,dirty_untracked,remotes,
      pin_state:(if .pin=="=" or .pin==" " or .pin=="." then "ok" elif .pin=="+" then "drifted" elif .pin=="-" then "uninitialised" elif .pin=="U" then "conflict" else "?" end),
      excepted:(((exc($r.path;"dirty")) != null) and $r.dirty),
      exception_reason:(exc($r.path;"dirty").reason // null),
      problems:([
        (if ($r.dirty and (exc($r.path;"dirty")==null)) then "dirty" else empty end),
        (if ($cl|index("REMOTE-BEHIND")) then "ahead" else empty end),
        (if ($cl|index("DIVERGED")) then "diverged" else empty end),
        (if $strict and ($cl|index("LOCAL-BEHIND")) then "behind" else empty end),
        (if $strict and $pindrift then "pin" else empty end)
      ]),
      unproven:([ $cl[] | select(.=="UNREACHABLE" or .=="UNKNOWN-DIFFERENT" or .=="NO-REMOTE-BRANCH") ]|unique)
     })) as $rows |
  { tool:"verify_repo.sh", root:$root, generated_utc:$ts, fetch:$fetch, strict:$strict,
    summary:{ repos:($rows|length), owned:($rows|map(select(.owned))|length),
              dirty:($rows|map(select(.dirty))|length),
              dirty_excepted:($rows|map(select(.excepted))|length),
              ahead:($rows|map(select(.problems|index("ahead")))|length),
              diverged:($rows|map(select(.problems|index("diverged")))|length),
              pin_drift:($rows|map(select(.pin_state!="ok"))|length),
              unproven:($rows|map(select(.unproven|length>0))|length),
              classes:([$rows[].remotes[].class]|group_by(.)|map({key:.[0],value:length})|from_entries),
              failing:($rows|map(select(.problems|length>0))|length) },
    repos:$rows }' "$WORK"/out/*.json > "$WORK/report.json"

[ -n "$JSON_OUT" ] && cp "$WORK/report.json" "$JSON_OUT"

if [ "$QUIET" = 0 ]; then
  jq -r '.repos[] | [ .path, (.branch // "(detached)"), (if .owned then "owned" else "third" end), .pin_state,
        (if .dirty then ("dirty t=" + (.dirty_tracked|tostring) + " u=" + (.dirty_untracked|tostring) + (if .excepted then " [EXC]" else "" end)) else "clean" end),
        ((.remotes|map(.remote + ":" + .class)|join(" "))|if .=="" then "-" else . end) ] | @tsv' \
    "$WORK/report.json" | { printf 'PATH\tBRANCH\tOWN\tPIN\tTREE\tREMOTES\n'; cat; } | column -t -s $'\t'
  echo; jq -c '.summary' "$WORK/report.json"
fi
if [ -z "$JSON_OUT" ] && [ "$QUIET" = 1 ]; then cat "$WORK/report.json"; fi

fail="$(jq '.summary.failing' "$WORK/report.json")"; unp="$(jq '.summary.unproven' "$WORK/report.json")"
[ "$fail" -gt 0 ] && exit 1
[ "$STRICT" = 1 ] && [ "$unp" -gt 0 ] && exit 1
[ "$unp" -gt 0 ] && exit 3
exit 0
