#!/usr/bin/env bash
# T040 helper: integrate_ff_only.sh - CPA stage S1 (docs/16 section 12.2): fetch objects for every owned repository and
# fast-forward ONLY the main repository; never a submodule at any depth; never a push, a rebase, a reset or a force.
#
# Usage   integrate_ff_only.sh --main-only --report-behind-submodules [--root <dir>] [--branch <b>] [--changeset-from <file>]
#           [--path-gates <tsv>] [--approved <manifest.json>] [--adoption-commit <sha>] [--owned-orgs a,b]
#           [--run-dir <dir>] [--json <out>] [--timeout <s>]
#   --root            main repository (default: the work tree of the current directory)
#   --changeset-from  declared change set, one path per line (relative to the main root)
#   --path-gates      rows `path<TAB>class`; class G-GATE paths are routed to the merge path unless their content matches
#                     the approved manifest (default <root>/scripts/repo/path_gates.tsv when it exists)
#   --approved        approved manifest, JSON {"<path>":"<sha256>"} (or {"paths":{...}}); absent = empty manifest
#   --adoption-commit CENTRAL C9 (c): a tip whose first-parent chain lacks it is routed to the merge path, never fast-forwarded
# Report  JSON (--json): status, exit, reason, remotes[{name,fetch,tip}], submodules[], pins[], moved{from,to,gitlinks[]},
#         paths[], commits[], gate_paths[]
# Exits   0 integrated or nothing to do; 11 blocked: no remote of the main repository could be reached, or a local-only commit cannot be cleared because
#         an unreachable remote has no known tip (reason remote_unreachable; an unreachable remote is never read as a remote that holds nothing: it
#         contributes the last fetched tip refs/remotes/<remote>/<branch> when this clone has one; WF3 review I-1); 12 needs the merge path / blocked
#         (reasons: diverged, remotes_diverged, ff_blocked_by_local_changes, merge_path_required, submodule_local_commits); 20 internal or refused
#         (reasons: force_refused, pin_not_on_remote, unrecorded_local_commit, usage, not_a_repository, wrong_branch, changeset_unreadable,
#         path_gates_unreadable: an explicitly named input file that is missing is a refusal, never an empty input)
# Round 3 (WF2 review): every remote name and URL (main repository and every submodule) passes lib_safe, ls-remote and fetch carry `--`, the fetches
#         never recurse into submodules, the live tip is the EXACT refs/heads/<branch> line, a declared path is a literal canonical path,
#         the R4 proof refuses a submodule with no remote (pin_not_on_remote), an uninitialised one or one with an unreachable remote
#         (pin_unverifiable), and a remote tip whose objects are not held locally refuses 20 `remote_tip_not_held` (the unrecorded-commit
#         guard never reads "cannot compute" as "nothing local-only").
# Decisions UNCONFIRMED until the T042 owner/review accepts them: a change-set gitlink held by NO remote of its submodule while
#         the submodule carries local commits is 12 submodule_local_commits (not 20); the pin of a declared submodule is its
#         work-tree HEAD (what the change set would commit); the CPA predicate here is trailer + the run's commits.tsv row
#         (the "GO committed in HEAD lists it" clause, the admission-table rule and the Foreign-Commit listing are not
#         implemented in this slice; the merge path itself is integrate_merge.sh, not this helper).
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
export GIT_ALLOW_PROTOCOL=file:ssh:git:https GIT_TERMINAL_PROMPT=0   # GIT_LITERAL_PATHSPECS only on the `g` calls, never exported into the hooks of the fast-forward (WF3 review m4)
ROOT=""; BRANCH=""; CSF=""; GATES=""; APPROVED=""; ADOPT=""; OWNED=""; RUNDIR=""; JSON=""; TMO=60; MAIN_ONLY=0; REPORT_SUB=0
W="$(mktemp -d "${TMPDIR:-/tmp}/iff.XXXXXX")" || exit 20; trap 'rm -rf "$W"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT
: > "$W/remotes"; : > "$W/subs"; : > "$W/pins"
status=""; reason=""; EXIT=0; MOVED='null'; PATHS='[]'; COMMITS='[]'; GATEP='[]'
js() { printf '%s\n' "$@" | jq -R . | jq -s .; }
finish() {  # finish <exit> <status> <reason>
  EXIT=$1; status=$2; reason=${3-}
  if [ -n "$JSON" ]; then
    jq -n --arg status "$status" --argjson exit "$EXIT" --arg reason "$reason" --slurpfile remotes "$W/remotes" \
      --slurpfile subs "$W/subs" --slurpfile pins "$W/pins" --argjson moved "$MOVED" --argjson paths "$PATHS" \
      --argjson commits "$COMMITS" --argjson gate_paths "$GATEP" \
      '{tool:"integrate_ff_only",status:$status,exit:$exit,reason:(if $reason=="" then null else $reason end),remotes:$remotes,
        submodules:$subs,pins:$pins,moved:$moved,paths:$paths,commits:$commits,gate_paths:$gate_paths}' > "$JSON" 2>/dev/null
  fi
  echo "integrate_ff_only: $status${reason:+ ($reason)} exit=$EXIT" ; exit "$EXIT"
}
prev=""; for a in "$@"; do [ "$prev" = --json ] && JSON="$a"; prev="$a"; done
for a in "$@"; do
  case "$a" in --force*|-f|+*|--rebase|--reset|--hard|--merge) finish 20 refused force_refused ;; esac
done
while [ $# -gt 0 ]; do
  case "$1" in --root|--branch|--changeset-from|--path-gates|--approved|--adoption-commit|--owned-orgs|--run-dir|--json|--timeout) [ $# -ge 2 ] || finish 20 refused usage ;; esac
  case "$1" in
    --main-only) MAIN_ONLY=1; shift ;; --report-behind-submodules) REPORT_SUB=1; shift ;;
    --root) ROOT="$2"; shift 2 ;; --branch) BRANCH="$2"; shift 2 ;; --changeset-from) CSF="$2"; shift 2 ;;
    --path-gates) GATES="$2"; shift 2 ;; --approved) APPROVED="$2"; shift 2 ;; --adoption-commit) ADOPT="$2"; shift 2 ;;
    --owned-orgs) OWNED="$2"; shift 2 ;; --run-dir) RUNDIR="$2"; shift 2 ;; --json) JSON="$2"; shift 2 ;;
    --timeout) TMO="$2"; shift 2 ;;
    *) finish 20 refused usage ;;
  esac
done
# every value that reaches git, jq, awk or timeout is validated first (lib_safe.sh; WF2 review I7)
[ -z "$ROOT" ] || safe_dir_arg "$ROOT" || finish 20 refused unsafe_root
[ -z "$BRANCH" ] || safe_branch "$BRANCH" || finish 20 refused unsafe_branch
for f in "$CSF" "$GATES" "$APPROVED" "$RUNDIR"; do [ -z "$f" ] || safe_dir_arg "$f" || finish 20 refused unsafe_argument; done
[ -z "$CSF" ] || [ -r "$CSF" ] || finish 20 refused changeset_unreadable
[ -z "$GATES" ] || [ -r "$GATES" ] || finish 20 refused path_gates_unreadable
[ -z "$ADOPT" ] || safe_sha "$ADOPT" || finish 20 refused unsafe_adoption_commit
[ -z "$OWNED" ] || [[ "$OWNED" =~ ^[A-Za-z0-9._-]+(,[A-Za-z0-9._-]+)*$ ]] || finish 20 refused unsafe_owned_orgs
[[ "$TMO" =~ ^[1-9][0-9]*$ ]] || finish 20 refused usage
[ -n "$ROOT" ] || ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || finish 20 refused not_a_repository
ROOT="$(cd "$ROOT" 2>/dev/null && git rev-parse --show-toplevel 2>/dev/null)" || finish 20 refused not_a_repository
g() { GIT_LITERAL_PATHSPECS=1 git -C "$ROOT" "$@"; }
cur="$(g symbolic-ref --short -q HEAD)" || finish 20 refused wrong_branch
[ -n "$BRANCH" ] || BRANCH="$cur"; [ "$cur" = "$BRANCH" ] || finish 20 refused wrong_branch
LOCAL="$(g rev-parse HEAD)"
tm() { timeout "$TMO" "$@"; }
# vet_remotes <repo dir>: every remote name and URL (fetch and push) of the repository passes lib_safe; sets VETREASON on a refusal
vet_remotes() {
  local d="$1" r u rl
  # the remote list is read with its status: a failing `git remote` inside a process substitution was "no remotes" and nothing was vetted (WF8)
  rl="$(git -C "$d" remote 2>/dev/null)" || { VETREASON=git_listing_failed; return 1; }
  while IFS= read -r r; do
    [ -n "$r" ] || continue
    safe_remote "$r" || { VETREASON=unsafe_remote_name; return 1; }
    while IFS= read -r u; do safe_url "$u" || { VETREASON=unsafe_remote_url; return 1; }; done \
      < <(git -C "$d" config --get-all "remote.$r.url" 2>/dev/null; git -C "$d" config --get-all "remote.$r.pushurl" 2>/dev/null)
  done <<< "$rl"
  return 0
}
heads_tips() { awk '$1 ~ /^[0-9a-f]+$/ && (length($1)==40 || length($1)==64) {print $1}'; }
VETREASON=""
vet_remotes "$ROOT" || finish 20 refused "$VETREASON"

# ---- fetch objects, per remote, main repository: no ref and no FETCH_HEAD is written (T032 form) ----------------------
TIPS=""; NREM=0; NFAIL=0; REMS="$(g remote 2>/dev/null)" || finish 20 refused git_listing_failed   # a failing `git remote` is never "no remotes" (WF7 M-3, the F3 class)
for r in $REMS; do
  NREM=$((NREM+1))
  if ! lr="$(tm git -C "$ROOT" ls-remote -- "$r" "refs/heads/$BRANCH" 2>&1)"; then
    NFAIL=$((NFAIL+1)); jq -n --arg n "$r" --arg e "$lr" '{name:$n,fetch:("fetch_failed:"+$n),error:$e,tip:null}' >> "$W/remotes"; continue; fi
  tip="$(printf '%s\n' "$lr" | lr_exact "$BRANCH")"
  if [ -z "$tip" ] || ! safe_sha "$tip"; then jq -n --arg n "$r" '{name:$n,fetch:"no_branch",tip:null}' >> "$W/remotes"; continue; fi
  if ! fe="$(tm git -C "$ROOT" fetch -q --no-tags --no-recurse-submodules --no-write-fetch-head --refmap= -- "$r" "refs/heads/$BRANCH" 2>&1)"; then
    NFAIL=$((NFAIL+1)); jq -n --arg n "$r" --arg e "$fe" '{name:$n,fetch:("fetch_failed:"+$n),error:$e,tip:null}' >> "$W/remotes"; continue; fi
  jq -n --arg n "$r" --arg t "$tip" '{name:$n,fetch:"ok",tip:$t}' >> "$W/remotes"
  case " $TIPS " in *" $tip "*) ;; *) TIPS="$TIPS $tip" ;; esac
done
# no remote of the main repository could be reached: nothing was fetched and nothing can be said about what any of them holds (WF3 review I-1)
[ "$NREM" -eq 0 ] || [ "$NFAIL" -lt "$NREM" ] || finish 11 blocked remote_unreachable

# ---- submodules at every depth: fetch objects, report behind (needs_update), never move ---------------------------------
SUBS="$(g submodule status --recursive 2>/dev/null | awk '{ if (substr($0,1,1)!="-") print $2 }')"
for p in $SUBS; do safe_relpath "$p" || finish 20 refused unsafe_path; vet_remotes "$ROOT/$p" || finish 20 refused "$VETREASON"; done
for p in $SUBS; do
  d="$ROOT/$p"; sh="$(git -C "$d" rev-parse HEAD 2>/dev/null)" || continue
  st=current; subtips=""; rows=""
  rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed   # a failing `git remote` is never "no remotes" (WF8)
  for r in $rl; do
    if lr="$(tm git -C "$d" ls-remote --heads -- "$r" 2>&1)" && tm git -C "$d" fetch -q --no-tags --no-recurse-submodules --no-write-fetch-head --refmap= -- "$r" $(printf '%s\n' "$lr" | heads_tips) >/dev/null 2>&1; then
      rows="$rows $r:ok"; for t in $(printf '%s\n' "$lr" | heads_tips); do subtips="$subtips $t"; done
    else rows="$rows $r:fetch_failed:$r"; fi
  done
  for t in $subtips; do
    if [ "$t" != "$sh" ] && git -C "$d" merge-base --is-ancestor "$sh" "$t" 2>/dev/null; then st=needs_update; fi
  done
  if [ "$st" = current ] && [ -n "$(git -C "$d" rev-list "$sh" --not $subtips 2>/dev/null | head -1)" ] && [ -n "$subtips" ]; then st=ahead; fi
  [ "$REPORT_SUB" = 1 ] && jq -n --arg p "$p" --arg h "$sh" --arg s "$st" --arg r "${rows# }" '{path:$p,head:$h,status:$s,remotes:($r|split(" "))}' >> "$W/subs"
done

# ---- declared change set --------------------------------------------------------------------------------------------------
CHANGED=""; [ -n "$CSF" ] && [ -f "$CSF" ] && CHANGED="$(grep -v '^[[:space:]]*$' "$CSF")"
# a declared path is a literal canonical path (a trailing `/` names a declared directory and is stripped; WF2 review B1)
while IFS= read -r c; do [ -n "$c" ] || continue; safe_declpath "${c%/}" || finish 20 refused unsafe_path; done <<< "$CHANGED"
inchanged() {  # inchanged <path> -> 0 when the path is a declared path or lies under a declared directory
  local x="$1" c; while IFS= read -r c; do [ -n "$c" ] || continue; c="${c%/}"; [ "$x" = "$c" ] && return 0; case "$x" in "$c"/*) return 0 ;; esac; done <<< "$CHANGED"; return 1; }
# R4 pin proof for every change-set gitlink: its commit must be held by every remote of that submodule, proven (WF2 review B2)
#   - the directory must be the root of its own work tree: an uninitialised submodule is an empty directory in the parent and `git -C` would
#     read the PARENT's HEAD as the pin (pin_unverifiable); at least one remote must exist (pin_not_on_remote otherwise)
#   - per remote: fetch `ok` or `failed` (ls-remote or the object fetch failed: the pin can be neither proven held nor proven lacking);
#     accepted only when no remote lacks it, none failed, and at least one holds it; a failed remote is pin_unverifiable
if [ -n "$CHANGED" ]; then
  while IFS= read -r c; do
    c="${c%/}"; [ -n "$c" ] || continue
    mode="$(g ls-files -s -- "$c" | awk 'NR==1{print $1}')"; [ "$mode" = 160000 ] || continue
    d="$ROOT/$c"
    { [ -d "$d" ] && [ "$(git -C "$d" rev-parse --show-toplevel 2>/dev/null)" = "$(cd "$d" && pwd -P)" ]; } || finish 20 refused pin_unverifiable
    pin="$(git -C "$d" rev-parse HEAD 2>/dev/null)" || finish 20 refused pin_unverifiable
    held=""; lack=""; failed=0; alltips=""; rrows=""; nrem=0
    rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed
    for r in $rl; do
      nrem=$((nrem+1)); has=false; fst=ok
      if ! lr="$(tm git -C "$d" ls-remote --heads -- "$r" 2>/dev/null)"; then fst=failed
      else
        rtips="$(printf '%s\n' "$lr" | heads_tips)"
        if [ -n "$rtips" ]; then
          if tm git -C "$d" fetch -q --no-tags --no-recurse-submodules --no-write-fetch-head --refmap= -- "$r" $rtips >/dev/null 2>&1; then
            for t in $rtips; do alltips="$alltips $t"; git -C "$d" merge-base --is-ancestor "$pin" "$t" 2>/dev/null && has=true; done
          else fst=failed; fi
        fi
      fi
      rrows="$rrows $r:$has:$fst"
      if [ "$fst" = failed ]; then failed=1; elif [ "$has" = true ]; then held="$held $r"; else lack="$lack $r"; fi
    done
    acc=false; [ -z "$lack" ] && [ "$failed" = 0 ] && [ -n "$held" ] && acc=true
    jq -n --arg p "$c" --arg pin "$pin" --argjson acc "$acc" --arg rr "${rrows# }" \
      '{path:$p,commit:$pin,accepted:$acc,remotes:($rr|split(" ")|map(select(length>0)|split(":")|{name:.[0],holds:(.[1]=="true"),fetch:(.[2]//"ok")}))}' >> "$W/pins"
    if [ -n "$lack" ]; then
      if [ -z "$held" ] && [ "$failed" = 0 ] && [ -n "$(git -C "$d" rev-list "$pin" --not $alltips -- 2>/dev/null | head -1)" ]; then finish 12 blocked submodule_local_commits; fi
      finish 20 refused pin_not_on_remote
    fi
    [ "$nrem" -gt 0 ] || finish 20 refused pin_not_on_remote
    [ "$acc" = true ] || finish 20 refused pin_unverifiable
  done <<< "$CHANGED"
fi

# ---- the CPA-commit predicate (plan owner's rule (d)): CPA-Run trailer plus the run's commits.tsv row ------------------------
is_cpa() {  # is_cpa <repo dir> <sha>: the one shared predicate (lib_safe.sh cpa_runid + cpa_row; WF3 review m3, X3)
  local id; id="$(cpa_runid "$1" "$2")" || return 1
  cpa_row "$ROOT/.audit/commit-push/$id/commits.tsv" "$2"
}
unrec() {  # unrec <repo dir> <branch> -> prints the non-CPA local-only commits; `!<tip>` instead when a remote tip is not held locally;
           # `?unreachable` after the commits when a remote could not be reached AND this clone knows no tip of it (cannot compute, WF3 review I-1)
  local dir="$1" br="$2" ts="" r t c unk=0 cannot=0 lr kt out="" rl
  # `%git_listing_failed` is the in-band marker for a failing `git remote` (this runs in a command substitution, where finish could not exit the script; WF8)
  rl="$(git -C "$dir" remote 2>/dev/null)" || { echo '%git_listing_failed'; return 0; }
  [ -n "$rl" ] || return 0
  for r in $rl; do
    if ! lr="$(tm git -C "$dir" ls-remote -- "$r" "refs/heads/$br" 2>/dev/null)"; then
      # an unreachable remote is never a remote that holds nothing: what it last held is the last fetched tip, and without one it is unknown
      kt="$(known_tip "$dir" "$r" "$br")"; if [ -n "$kt" ]; then ts="$ts $kt"; else cannot=1; fi; continue; fi
    t="$(printf '%s\n' "$lr" | lr_exact "$br")"; [ -n "$t" ] || continue
    if safe_sha "$t" && git -C "$dir" cat-file -e "$t^{commit}" 2>/dev/null; then ts="$ts $t"; else echo "!$t"; unk=1; fi
  done
  # a tip whose objects are missing makes `rev-list --not <tip>` fail and print nothing, which would read as "no local-only commit": fail closed
  [ "$unk" = 0 ] || return 0
  for c in $(git -C "$dir" rev-list "$br" --not $ts -- 2>/dev/null); do is_cpa "$dir" "$c" || out="$out$c"$'\n'; done
  [ -z "$out" ] || { printf '%s' "$out"; [ "$cannot" = 0 ] || echo '?unreachable'; }
}
bad="$(unrec "$ROOT" "$BRANCH")"
if [ -z "$bad" ] && [ -n "$OWNED" ]; then
  # an owned submodule is judged by the organisation of its remote URLs, parsed by the one shared owner parser (scp-form, https, ssh://;
  # case-insensitive), never by a private reading of the URL (WF2 review I6)
  ORGOF="$D/../audit/org_of.py"; OWNEDL="$(printf '%s' "$OWNED" | tr 'A-Z' 'a-z')"
  [ -r "$ORGOF" ] || finish 20 refused org_of_missing
  for p in $SUBS; do
    d="$ROOT/$p"; own=0
    rl="$(git -C "$d" remote 2>/dev/null)" || finish 20 refused git_listing_failed
    for r in $rl; do u="$(git -C "$d" remote get-url "$r" 2>/dev/null)"; org="$(python3 "$ORGOF" "$u" 2>/dev/null | head -1)"
      [ -n "$org" ] || continue
      case ",$OWNEDL," in *",$org,"*) own=1 ;; esac; done
    [ "$own" = 1 ] || continue
    # the branch the run pushes (refs/heads/<run branch>), whatever is checked out; the checked-out branch only when the run branch is absent (m2)
    sb="$BRANCH"; git -C "$d" rev-parse -q --verify "refs/heads/$BRANCH" >/dev/null 2>&1 || { sb="$(git -C "$d" symbolic-ref --short -q HEAD)" || continue; }
    bad="$(unrec "$d" "$sb")"; [ -n "$bad" ] && break
  done
fi
if [ -n "$bad" ]; then
  case "$bad" in '%'*) finish 20 refused git_listing_failed ;; esac
  case "$bad" in '!'*) COMMITS="$(js ${bad//!/})"; finish 20 refused remote_tip_not_held ;; esac
  CANNOT=0; case $'\n'"$bad" in *$'\n?unreachable'*) CANNOT=1; bad="$(printf '%s\n' "$bad" | grep -v '^?')" ;; esac
  COMMITS="$(js $bad)"
  [ "$CANNOT" = 0 ] || finish 11 blocked remote_unreachable
  finish 20 refused unrecorded_local_commit
fi

# ---- integrate the main repository: fast-forward only --------------------------------------------------------------------
rel() { # rel <tip> -> eq|behind|ahead|div  (relative to LOCAL)
  if [ "$1" = "$LOCAL" ]; then echo eq
  elif g merge-base --is-ancestor "$1" "$LOCAL" 2>/dev/null; then echo behind
  elif g merge-base --is-ancestor "$LOCAL" "$1" 2>/dev/null; then echo ahead
  else echo div; fi; }
CAND=""; ANYTIP=0
for t in $TIPS; do ANYTIP=1; case "$(rel "$t")" in ahead|div) CAND="$CAND $t" ;; esac; done
[ -z "$CAND" ] && { [ "$ANYTIP" = 1 ] && [ "$(for t in $TIPS; do rel "$t"; done | grep -c behind)" -gt 0 ] && finish 0 local_ahead; finish 0 up_to_date; }
NEWEST=""
for t in $CAND; do ok=1; for u in $CAND; do [ "$u" = "$t" ] || g merge-base --is-ancestor "$u" "$t" 2>/dev/null || ok=0; done; [ "$ok" = 1 ] && NEWEST="$t"; done
[ -n "$NEWEST" ] || finish 12 blocked remotes_diverged
[ "$(rel "$NEWEST")" = div ] && finish 12 blocked diverged
# 1. incoming commits touching a declared path block the fast-forward
IN="$(g diff --name-only "$LOCAL" "$NEWEST")"; hit=""
while IFS= read -r x; do [ -n "$x" ] && inchanged "$x" && hit="$hit $x"; done <<< "$IN"
if [ -n "$hit" ]; then PATHS="$(js $hit)"; finish 12 blocked ff_blocked_by_local_changes; fi
# 2. routing to the merge path: G-GATE content outside the approved manifest; adoption commit off the first-parent chain
RT=""; GP=""
[ -n "$GATES" ] || { [ -f "$ROOT/scripts/repo/path_gates.tsv" ] && GATES="$ROOT/scripts/repo/path_gates.tsv"; }
if [ -n "$GATES" ] && [ -f "$GATES" ]; then
  MAN='{}'; [ -n "$APPROVED" ] && [ -f "$APPROVED" ] && MAN="$(jq -c '(.paths // .)' "$APPROVED" 2>/dev/null || echo '{}')"
  for gp in $(awk -F'\t' '$2=="G-GATE"{print $1}' "$GATES"); do
    for c in $(g rev-list "$LOCAL..$NEWEST"); do
      ln="$(g diff-tree -r -m --no-commit-id --name-status "$c" -- "$gp" 2>/dev/null | head -1)"; [ -n "$ln" ] || continue
      want="$(printf '%s' "$MAN" | jq -r --arg p "$gp" '.[$p] // empty')"
      if [ "${ln%%	*}" = D ]; then [ -n "$want" ] && GP="$GP $gp"
      else got="$(g show "$c:$gp" 2>/dev/null | sha256sum | cut -d' ' -f1)"; [ "$got" = "$want" ] || GP="$GP $gp"; fi
    done
  done
  [ -n "$GP" ] && { GP="$(printf '%s\n' $GP | sort -u | tr '\n' ' ')"; GATEP="$(js $GP)"; RT=gate; }
fi
if [ -n "$ADOPT" ] && ! g rev-list --first-parent "$NEWEST" 2>/dev/null | grep -qx "$(g rev-parse "$ADOPT^{commit}" 2>/dev/null || echo none)"; then RT="${RT:+$RT,}adoption"; fi
[ -n "$RT" ] && finish 12 blocked merge_path_required
# 3. the fast-forward itself (main repository only; git refuses a merge that would overwrite an uncommitted change)
if ! mo="$(git -C "$ROOT" merge --ff-only -q "$NEWEST" 2>&1)"; then echo "$mo" >&2; finish 12 blocked ff_blocked_by_local_changes; fi
GL="[]"; while IFS=$'\t' read -r meta path; do
  [ -n "$meta" ] || continue; set -- $meta; om="${1#:}"; nm="$2"; oc="$3"; nc="$4"
  if [ "$om" = 160000 ] || [ "$nm" = 160000 ]; then GL="$(printf '%s' "$GL" | jq -c --arg p "$path" --arg o "$oc" --arg n "$nc" '. + [{path:$p,old:$o,new:$n}]')"; fi
done < <(g diff-tree -r --no-commit-id --raw "$LOCAL" "$NEWEST")
MOVED="$(jq -n --arg f "$LOCAL" --arg t "$NEWEST" --argjson gl "$GL" '{from:$f,to:$t,gitlinks:$gl}')"
finish 0 fast_forwarded
