#!/usr/bin/env bash
#
# check_submodule_ff.sh - gate: a commit range must move every own-org submodule
# gitlink FAST-FORWARD only (the new pin is a descendant of the old pin).
#
# Why: a gitlink moved backward or sideways silently drops submodule commits from
# every clone of the parent (constitution 11.4.113 no history loss, 11.4.288(D)
# recorded gitlink on main, 11.4.233(G) pointer drift). This gate checks the
# recorded gitlinks of a range; it never fetches, commits, pushes or edits.
#
# What it compares (every gitlink, mode 160000):
#   * every commit of OLD..NEW against EVERY one of its parents (so a merge that
#     keeps the local pin and drops the other parent's advance is seen), and
#   * the OLD tree against the NEW tree (net endpoints, reported commit=RANGE).
# A gitlink deleted at one path and added at another in the same diff is a rename
# and is checked as one move "old=>new" when both sides carry the same
# .gitmodules name, or (left over after that) the same canonical .gitmodules URL.
# URL pairing is used only when it is unambiguous: if two removals or two additions
# in one diff share a URL key, each such removal is reported
#   BLIND <class> <old>=>? <oldpin> - commit=<tag> reason=ambiguous-pairing
# (never paired by first match). Entries with no name or no URL never pair.
# Across commits (walked parents-first, --topo-order): a gitlink an ancestor commit
# removed and a descendant adds back is checked as a move from the removed pin to the
# added one, so remove + re-add at an older pin FAILs. Removals are remembered per line
# of descent (a parallel branch never sees them). Same name wins (skipped when both URL
# keys are known and differ; every distinct remembered pin is compared); else same URL
# key, where removals with different pins give
# BLIND <class> ?=><new> - <newpin> commit=<sha> reason=readd-ambiguous. Renames chain
# hop by hop. The OLD -> NEW (RANGE) diff never uses remembered removals; removals
# before OLD are not known.
# A .gitmodules that git config cannot parse is reason=gitmodules-unparsable (BLIND).
# Each move is classified:
#   FF       git merge-base --is-ancestor OLDPIN NEWPIN exits 0
#   NONFF    it exits 1 (backward or diverged move)
#   BLIND    a pin is not in any local object store of the submodule
#            (.git/modules/<name>, the checked-out <path>/.git, or, as a fallback
#            after a rename, any non-shallow, graft-free store under
#            <common-git-dir>/modules/** that holds BOTH pins), the store is
#            shallow or has info/grafts, or the ancestry query errors; never read
#            as PASS (11.4.201 / 11.4.273)
#   ADDED / REMOVED  gitlink created or deleted (reported, not failed)
# Replace refs are ignored (GIT_NO_REPLACE_OBJECTS=1). Any git error while
# listing the range or diffing trees, or info/grafts in the parent repository,
# makes the verdict BLIND.
# Every diff runs with --ignore-submodules=none, so submodule.<name>.ignore (repo
# config or .gitmodules) and diff.ignoreSubmodules cannot hide a gitlink move. As a
# cross-check the OLD and NEW trees are also listed with ls-tree (unaffected by any
# ignore setting): the number of gitlink paths that differ must equal the number of
# gitlink entries the RANGE diff saw, else BLIND reason=gitlink-diff-mismatch; an
# ls-tree failure is BLIND reason=git-error. So a nothing-checked PASS cannot come
# from a diff that was blinded.
#
# Organisation (per move, over the WHOLE range): every tree the scan touches (OLD,
# NEW, each commit of the range and each of its parents) has its .gitmodules read.
# A move is own-org if its .gitmodules name or path has an own URL in ANY of those
# trees; else third-party if it has a third-party URL in any of them; else (no
# .gitmodules entry at all) own. A URL is third-party only if it has a strict
# remote shape (git@host:org/repo, or scheme://[user@]host[:port]/org/repo with
# scheme ssh|git|http|https; exactly two path segments) and its organisation
# (scripts/audit/org_of.py) is not in the own-org list (case-insensitive; "#"
# starts a comment, also inline). Anything else (local or relative path, file://,
# one or three+ path segments, an IPv6 host such as [::1], a git+https:// or other
# unlisted scheme, a path containing "~" or "%", no URL) is own (fail closed).
# Relative URLs (../x.git) are NOT resolved against the parent's remote, because
# that remote may be a personal fork; they are always own. Third-party moves are
# reported and counted separately; they never change the exit status.
#
# Limit of the own-org list: the guard below (own-orgs-match-nothing) fires only
# when the list matches NO URL in the whole range. A list that is only partly stale
# (it matches one own submodule but has lost the org of another) classes the lost
# one third-party and that move passes; keep the list complete. A submodule whose
# URL was changed to a third-party fork in the same range is still own when its
# name or path has an own URL in any tree of the range.
#
# Counters count reported lines: one move can be reported for its commit, for each
# parent of a merge, and again for the RANGE endpoints, so checked= may exceed the
# number of distinct moves. A range with no gitlink move ends with note=nothing-checked.
#
# Control needle (11.4.273): before the real scan every run builds a throwaway
# repository in a mktemp directory with one known forward and one known backward
# move and runs the SAME scan on it; unless it prints exactly two lines, FF then
# NONFF, the gate exits 2 (the instrument is blind).
#
# Environment: the repository is resolved once from the caller's environment
# (honouring an exported GIT_DIR / GIT_WORK_TREE); then GIT_DIR, GIT_WORK_TREE,
# GIT_INDEX_FILE, GIT_OBJECT_DIRECTORY, GIT_ALTERNATE_OBJECT_DIRECTORIES,
# GIT_COMMON_DIR, GIT_NAMESPACE, GIT_GRAFT_FILE and GIT_PREFIX are unset and every
# git call names its repository explicitly (-C / --git-dir), so the needle and the
# scan can never write into the caller's repository. GIT_NO_REPLACE_OBJECTS=1,
# GIT_NO_LAZY_FETCH=1, GIT_TERMINAL_PROMPT=0 and GIT_OPTIONAL_LOCKS=0 are set.
#
# Usage:  scripts/check_submodule_ff.sh OLD NEW
#         scripts/check_submodule_ff.sh OLD..NEW
#         scripts/check_submodule_ff.sh --selftest     (control needle only)
#   Run inside the parent repository (any directory of its work tree); a work tree is
#   required, so bare repositories and server-side pre-receive hooks are unsupported.
#   OLD...NEW (symmetric range) is rejected with exit 64. Not wired into any hook or
#   gate runner; it is a manual / scripted check. Cost is roughly 0.25 s per commit.
#   Env: SUBFF_OWN_ORGS=<file> overrides the own-org list; its first entry names the
#   control needle's org URL (git@github.com:<org>/needle_sub.git).
# URLs are canonicalised (scheme / scp form, user@, port, case, .git, trailing /,
#   and for scheme:// URLs any ?query and #fragment) into one host/org/repo key used
#   both for rename pairing and own-org matching.
# Output lines: FF|NONFF|BLIND <own|third-party> <path> <oldpin> <newpin> commit=<sha|RANGE> [reason=...]
#               ADDED|REMOVED <own|third-party> <path> <pin> commit=<sha|RANGE>
#               BLIND own <oldpath>=>? <oldpin> - commit=<sha> reason=unpaired-rename
#               BLIND <class> <oldpath>=>? <oldpin> - commit=<sha|RANGE> reason=ambiguous-pairing
#               BLIND <class> ?=><newpath> - <newpin> commit=<sha> reason=readd-ambiguous
#               NEEDLE forward=<x> backward=<y> class=<own|...> ; NEEDLE ok|FAILED
#               VERDICT: PASS|FAIL|BLIND checked=N own_ff=N own_nonff=N own_blind=N third_party_ff=N third_party_nonff=N third_party_blind=N [reason=...] [note=nothing-checked]
# Exit:   0 PASS; 1 FAIL (an own-org NONFF; wins over BLIND); 2 BLIND (an own-org
#         move undecidable, a git error, parent grafts, a shallow parent
#         (reason=parent-shallow), an own remove+add in one commit that cannot be
#         paired (reason=unpaired-rename), a same-URL pairing that is ambiguous
#         (reason=ambiguous-pairing), an ambiguous cross-commit re-add
#         (reason=readd-ambiguous), an unparsable .gitmodules
#         (reason=gitmodules-unparsable), an ls-tree count that disagrees with the
#         RANGE diff (reason=gitlink-diff-mismatch), an own-org list that matches
#         no URL in the range while some move was classed third-party
#         (reason=own-orgs-match-nothing), a failed control needle, or python3 /
#         org_of.py missing); 64 usage error.
# Side effects: none on the scanned repository (refs, objects and index unchanged);
#         scratch files and the needle repo live in a mktemp directory removed on exit.
set -uo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ORG_OF="${SCRIPT_DIR}/audit/org_of.py"
OWN_ORGS_FILE="${SUBFF_OWN_ORGS:-${SCRIPT_DIR}/audit/own_orgs.txt}"

# Resolve the caller's repository BEFORE dropping the inherited git environment.
IN_TOP="$(git rev-parse --show-toplevel 2>/dev/null || true)"
IN_GITDIR="$(git rev-parse --absolute-git-dir 2>/dev/null || true)"
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY GIT_ALTERNATE_OBJECT_DIRECTORIES \
      GIT_COMMON_DIR GIT_NAMESPACE GIT_GRAFT_FILE GIT_PREFIX
export GIT_NO_REPLACE_OBJECTS=1 GIT_NO_LAZY_FETCH=1 GIT_TERMINAL_PROMPT=0 GIT_OPTIONAL_LOCKS=0

usage() { echo "usage: check_submodule_ff.sh OLD NEW | OLD..NEW | --selftest" >&2; exit 64; }

OWN_ORGS=""
load_orgs() {
  [ -r "$OWN_ORGS_FILE" ] || { echo "ERROR own-org list unreadable: $OWN_ORGS_FILE" >&2; exit 64; }
  OWN_ORGS="$(sed 's/#.*//; s/[[:space:]]//g' "$OWN_ORGS_FILE" | grep -v '^$' | tr '[:upper:]' '[:lower:]')"
  [ -n "$OWN_ORGS" ] || { echo "ERROR own-org list empty: $OWN_ORGS_FILE" >&2; exit 64; }
}

# canon_url URL -> sets CANON_KEY (pairing key, never empty for a non-empty URL) and CANON_REMOTE
# (host/org/repo when the URL is a remote of exactly two path segments, else empty).
# Remote forms scheme://[user@]host[:port]/path and scp [user@]host:path collapse to one lowercase
# host/path key without user, port, trailing "/" or ".git"; anything else keeps its literal text.
canon_url() {
  local u="$1" l host path
  CANON_KEY=""; CANON_REMOTE=""
  # a scheme URL's ?query and #fragment are not part of the repository path
  if [[ "$u" == *://* ]]; then u="${u%%#*}"; u="${u%%\?*}"; fi
  while [ "${u%/}" != "$u" ]; do u="${u%/}"; done
  l="$(printf '%s' "$u" | tr '[:upper:]' '[:lower:]')"
  [ "${l%.git}" != "$l" ] && { u="${u%????}"; l="${l%.git}"; }
  while [ "${l%/}" != "$l" ]; do l="${l%/}"; u="${u%/}"; done
  CANON_KEY="$u"
  if [[ "$l" =~ ^(ssh|git|http|https|git\+ssh|ssh\+git)://([^@/]+@)?([^/:@]+)(:[0-9]*)?/+(.+)$ ]]; then
    host="${BASH_REMATCH[3]}"; path="${BASH_REMATCH[5]}"
  elif [[ "$l" != *://* ]] && [[ "$l" =~ ^([a-z0-9._-]+@)?([^/:@]+):/*([^:].*)$ ]] &&
       { [ -n "${BASH_REMATCH[1]}" ] || [[ "${BASH_REMATCH[2]}" == *.* ]]; }; then
    host="${BASH_REMATCH[2]}"; path="${BASH_REMATCH[3]}"
  else
    return 0
  fi
  CANON_KEY="$host/$path"
  # "~user" (a home directory, not an organisation) and "%XX" escapes (org%2Dname is org-name, and
  # an escaped "/" would hide a segment) never yield an organisation: own (fail closed)
  if [[ "$path" =~ ^[^/]+/[^/]+$ ]] && [[ "/$path/" != */./* ]] && [[ "/$path/" != */../* ]] &&
     [[ "$path" != *[~%]* ]]; then
    CANON_REMOTE="$host/$path"
  fi
}

declare -A CLASS_CACHE=()
ORG_MATCH=0
# class_of URL -> sets CLS to own | third-party (fail closed: anything not provably third-party is own);
# sets ORG_MATCH=1 when the URL's organisation is in the own-org list
class_of() {
  local url="$1" org m=n
  if [ -n "${CLASS_CACHE[x$url]:-}" ]; then
    CLS="${CLASS_CACHE[x$url]%:*}"; [ "${CLASS_CACHE[x$url]#*:}" = y ] && ORG_MATCH=1; return
  fi
  CLS=own
  canon_url "$url"
  if [ -n "$CANON_REMOTE" ]; then
    org="$(python3 "$ORG_OF" "https://$CANON_REMOTE" 2>/dev/null | tr '[:upper:]' '[:lower:]')"
    if [ -n "$org" ]; then
      if printf '%s\n' "$OWN_ORGS" | grep -qxF "$org"; then m=y; ORG_MATCH=1; else CLS=third-party; fi
    fi
  fi
  CLASS_CACHE[x$url]="$CLS:$m"
}

declare -A GM_DONE=() GM_NAME=() GM_URL=() OWN_KEY=() TP_KEY=()
declare -a E_SHA=() E_PATH=() E_NAME=() E_KEY=()
declare -A GSET=() WL=()
NEWRM=""
# parse_gm COMMIT: read .gitmodules at COMMIT into GM_NAME/GM_URL["commit<TAB>path"]
# and record every name/path as own or third-party in OWN_KEY / TP_KEY
parse_gm() {
  local commit="$1" rec key val name gmf grc
  local -A path_of=() url_of=()
  [ -n "${GM_DONE[$commit]:-}" ] && return 0
  GM_DONE[$commit]=1
  git -C "$CUR_REPO" cat-file -e "$commit:.gitmodules" 2>/dev/null || return 0
  # a file git config cannot parse could hide the own URL: never ignore it (BLIND, reported at the
  # end). --get-regexp exits 1 both for "no matching key" and for a malformed file (git 2.53), so
  # the parse is proven first with --list (0 on any well-formed file, empty included; 128 on a
  # malformed one); after that, exit 1 of --get-regexp can only mean "no matching key"
  gmf="$WORK/gm.$BASHPID.$RANDOM$RANDOM"
  git -C "$CUR_REPO" config --blob "$commit:.gitmodules" --list > /dev/null 2>&1; grc=$?
  [ "$grc" -eq 0 ] && { git -C "$CUR_REPO" config -z --blob "$commit:.gitmodules" --get-regexp '^submodule\..*\.(path|url)$' > "$gmf" 2>/dev/null; grc=$?; }
  if [ "$grc" -ne 0 ] && [ "$grc" -ne 1 ]; then
    echo "ERROR .gitmodules of $commit cannot be parsed (git config exit $grc)"; GM_ERR=1; return 0
  fi
  while IFS= read -r -d '' rec; do
    if [[ "$rec" == *$'\n'* ]]; then key="${rec%%$'\n'*}"; val="${rec#*$'\n'}"; else key="$rec"; val=""; fi
    name="${key#submodule.}"
    case "$key" in
      *.path) path_of["${name%.path}"]="$val" ;;
      *.url) url_of["${name%.url}"]="$val" ;;
    esac
  done < "$gmf"
  for name in "${!path_of[@]}"; do
    [ -n "$name" ] || continue
    GM_NAME["$commit"$'\t'"${path_of[$name]}"]="$name"
    GM_URL["$commit"$'\t'"${path_of[$name]}"]="${url_of[$name]:-}"
    class_of "${url_of[$name]:-}"
    if [ "$CLS" = own ]; then OWN_KEY["n:$name"]=1; OWN_KEY["p:${path_of[$name]}"]=1
    else TP_KEY["n:$name"]=1; TP_KEY["p:${path_of[$name]}"]=1; fi
  done
}

# key_class KEY... -> sets CLS: own if any key is own anywhere in the range; else
# third-party if any key is third-party; else own (no .gitmodules entry: fail closed)
key_class() {
  local k
  for k in "$@"; do [ "$k" = n: ] || [ "$k" = p: ] && continue; [ -n "${OWN_KEY[$k]:-}" ] && { CLS=own; return; }; done
  for k in "$@"; do [ "$k" = n: ] || [ "$k" = p: ] && continue; [ -n "${TP_KEY[$k]:-}" ] && { CLS=third-party; return; }; done
  CLS=own
}

# store_for REPO NAME PATH OLDPIN NEWPIN -> a git dir holding BOTH commits (empty if none)
# Every git call on a store passes --work-tree=<store>: a store moved or left behind by a rename
# keeps core.worktree pointing at a checkout that no longer exists, and git would then die with
# "cannot chdir" (exit 128) on every command, read-only ones included.
store_for() {
  local repo="$1" name="$2" path="$3" a="$4" b="$5" common cand
  common="$(git -C "$repo" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"
  for cand in "${common:+${name:+$common/modules/$name}}" "$repo/$path"; do
    [ -n "$cand" ] && [ -e "$cand" ] || continue
    if [ "$cand" = "$repo/$path" ]; then
      [ -e "$cand/.git" ] || continue
      cand="$(git -C "$cand" rev-parse --path-format=absolute --git-dir 2>/dev/null)" || continue
    fi
    git --git-dir="$cand" --work-tree="$cand" cat-file -e "$a^{commit}" 2>/dev/null || continue
    git --git-dir="$cand" --work-tree="$cand" cat-file -e "$b^{commit}" 2>/dev/null || continue
    printf '%s\n' "$cand"; return 0
  done
  # fallback: the submodule was renamed or moved since (its store now lives under another name),
  # so try every other submodule git dir under <common>/modules (nested ones too); only a store
  # that is neither shallow nor grafted is taken, so a fallback can never weaken the ancestry proof
  [ -n "$common" ] && [ -d "$common/modules" ] || return 0
  while IFS= read -r -d '' cand; do
    cand="${cand%/HEAD}"
    [ -d "$cand/objects" ] || continue
    [ "$(git --git-dir="$cand" --work-tree="$cand" rev-parse --is-shallow-repository 2>/dev/null)" = false ] || continue
    [ -e "$cand/info/grafts" ] && continue
    git --git-dir="$cand" --work-tree="$cand" cat-file -e "$a^{commit}" 2>/dev/null || continue
    git --git-dir="$cand" --work-tree="$cand" cat-file -e "$b^{commit}" 2>/dev/null || continue
    printf '%s\n' "$cand"; return 0
  done < <(find "$common/modules" -type f -name HEAD -print0 2>/dev/null | LC_ALL=C sort -z)
}

count() {  # count <own|third-party> <FF|NONFF|BLIND>
  case "$1:$2" in
    own:FF) OWN_FF=$((OWN_FF + 1)) ;;
    own:NONFF) OWN_NONFF=$((OWN_NONFF + 1)) ;;
    own:BLIND) OWN_BLIND=$((OWN_BLIND + 1)) ;;
    third-party:FF) TP_FF=$((TP_FF + 1)) ;;
    third-party:NONFF) TP_NONFF=$((TP_NONFF + 1)) ;;
    *) TP_BLIND=$((TP_BLIND + 1)) ;;
  esac
}

# classify_move CLASS LABEL NAME STOREPATH OLDPIN NEWPIN TAG
classify_move() {
  local cls="$1" label="$2" name="$3" spath="$4" osha="$5" nsha="$6" tag="$7" store anc common
  CHECKED=$((CHECKED + 1))
  store="$(store_for "$CUR_REPO" "$name" "$spath" "$osha" "$nsha")"
  if [ -z "$store" ]; then
    echo "BLIND $cls $label $osha $nsha commit=$tag reason=pin-not-in-local-object-store"; count "$cls" BLIND; return
  fi
  if [ "$(git --git-dir="$store" --work-tree="$store" rev-parse --is-shallow-repository 2>/dev/null)" != false ]; then
    echo "BLIND $cls $label $osha $nsha commit=$tag reason=shallow-store"; count "$cls" BLIND; return
  fi
  common="$(git --git-dir="$store" --work-tree="$store" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"
  if [ -z "$common" ] || [ -e "$common/info/grafts" ]; then
    echo "BLIND $cls $label $osha $nsha commit=$tag reason=grafts"; count "$cls" BLIND; return
  fi
  git --git-dir="$store" --work-tree="$store" merge-base --is-ancestor "$osha" "$nsha" >/dev/null 2>&1; anc=$?
  case "$anc" in
    0) echo "FF $cls $label $osha $nsha commit=$tag"; count "$cls" FF ;;
    1) echo "NONFF $cls $label $osha $nsha commit=$tag"; count "$cls" NONFF ;;
    *) echo "BLIND $cls $label $osha $nsha commit=$tag reason=merge-base-rc-$anc"; count "$cls" BLIND ;;
  esac
}

# Removed pins are remembered per line of descent. Entry g (E_SHA/E_PATH/E_NAME/E_KEY) is put in the
# set of the commit that removed it (GSET[commit] = "g:f ..."), and every commit starts from the union
# of its parents' sets, so an addition is only compared with a removal made by one of its ancestors.
# f is "b" (live for a name or a URL-key match) or "n" (live for a name match only); WL is the working
# set of the commit being checked, "x" marks an entry consumed there.
# readd_lookup NAME KEY PATH CLASS NEWPIN -> GONE_HIT:
#   1. entries with the same non-empty name: this is the same submodule (a URL key that differs proves
#      nothing, a URL form such as ../sub.git or a mirror host is not collapsed), so every distinct
#      remembered pin is listed and compared (parallel lines may have removed it at different pins:
#      any backward move FAILs; an unrelated repository reusing the name has no store holding both
#      pins: BLIND); all are consumed
#   2. else entries live for a URL-key match with the same non-empty key: one index when they all hold
#      one pin, "amb" when the pins differ (which copy is this?); an entry whose name differs from
#      NAME keeps its name match ("n"), so a same-URL sibling cannot hide a later re-add of that name
#   3. else entries at the same path: every distinct pin, all consumed. When the entry and the addition
#      both have names (they differ, else step 1) the entry is taken if one local store holds both pins
#      (shared history: the same repository renamed) or if no store resolves either pin (nothing can
#      be told: BLIND); it is left alone only if a store resolves one pin but none holds both (another
#      repository at that path: ADDED)
#   4. else, for an own addition (CLASS own), "unid" when a live entry has neither a name nor a URL key:
#      it can never be matched, so the caller reports BLIND (the entries are consumed: one BLIND each
#      time); a third-party addition leaves such entries alone
#   "none" if nothing matches.
readd_lookup() {
  local name="$1" key="$2" path="$3" cls="$4" nsha="$5" g sha="" pick=""
  local -a cand=() live=()
  local -A seen=()
  GONE_HIT=none
  [ "${#WL[@]}" -eq 0 ] && return 0
  mapfile -t live < <(printf '%s\n' "${!WL[@]}" | LC_ALL=C sort -n)
  if [ -n "$name" ]; then
    for g in "${live[@]}"; do
      [ "${WL[$g]}" != x ] && [ "${E_NAME[$g]}" = "$name" ] && cand+=("$g")
    done
  fi
  if [ "${#cand[@]}" -eq 0 ] && [ -n "$key" ]; then
    for g in "${live[@]}"; do
      [ "${WL[$g]}" = b ] && [ "${E_KEY[$g]}" = "$key" ] && cand+=("$g")
    done
    if [ "${#cand[@]}" -gt 0 ]; then
      for g in "${cand[@]}"; do
        if [ -z "$pick" ]; then pick="$g"; sha="${E_SHA[$g]}"
        elif [ "${E_SHA[$g]}" != "$sha" ]; then GONE_HIT=amb; return 0; fi
      done
      for g in "${cand[@]}"; do
        if [ -n "${E_NAME[$g]}" ] && [ "${E_NAME[$g]}" != "$name" ]; then WL[$g]=n; else WL[$g]=x; fi
      done
      GONE_HIT="$pick"; return 0
    fi
  fi
  if [ "${#cand[@]}" -eq 0 ]; then
    for g in "${live[@]}"; do
      [ "${WL[$g]}" != x ] && [ "${E_PATH[$g]}" = "$path" ] || continue
      if [ -n "${E_NAME[$g]}" ] && [ -n "$name" ] &&
        [ -z "$(store_for "$CUR_REPO" "$name" "$path" "${E_SHA[$g]}" "$nsha")" ]; then
        [ -n "$(store_for "$CUR_REPO" "${E_NAME[$g]}" "$path" "${E_SHA[$g]}" "${E_SHA[$g]}")" ] && continue
        [ -n "$(store_for "$CUR_REPO" "$name" "$path" "$nsha" "$nsha")" ] && continue
      fi
      cand+=("$g")
    done
  fi
  if [ "${#cand[@]}" -gt 0 ]; then
    GONE_HIT=""
    for g in "${cand[@]}"; do
      WL[$g]=x
      [ -n "${seen[x${E_SHA[$g]}]:-}" ] && continue
      seen[x${E_SHA[$g]}]=1; GONE_HIT+="$g "
    done
    return 0
  fi
  [ "$cls" = own ] || return 0
  for g in "${live[@]}"; do
    [ "${WL[$g]}" != x ] && [ -z "${E_NAME[$g]}" ] && [ -z "${E_KEY[$g]}" ] || continue
    WL[$g]=x; GONE_HIT=unid
  done
}

# check_diff A B TAG : classify every gitlink change of tree A -> tree B
check_diff() {
  local a="$1" b="$2" tag="$3" f meta path omode nmode osha nsha name_a name_b i j k paired own_add nadd nrm g
  local -a rm_path=() rm_sha=() rm_name=() rm_url=() rm_left=() ad_path=() ad_sha=() ad_name=() ad_url=() ad_used=() ad_key=()
  local -a rm_key=() rm_pair=()
  local -A amb=()
  f="$WORK/diff.$BASHPID.$RANDOM$RANDOM"
  # --ignore-submodules=none: diff.ignoreSubmodules and submodule.<name>.ignore (repo config or the
  # worktree .gitmodules) would otherwise hide gitlink changes and read as nothing-checked
  if ! git -C "$CUR_REPO" diff-tree -z -r --ignore-submodules=none --no-commit-id --raw "$a" "$b" > "$f" 2>/dev/null; then
    echo "ERROR git diff-tree $a $b failed"; GIT_ERR=1; return
  fi
  parse_gm "$a"; parse_gm "$b"
  while IFS= read -r -d '' meta && IFS= read -r -d '' path; do
    read -r omode nmode osha nsha _ <<<"${meta#:}"
    [ "$omode" = 160000 ] || [ "$nmode" = 160000 ] || continue
    [ "$tag" = RANGE ] && RANGE_GL=$((RANGE_GL + 1))
    name_a="${GM_NAME["$a"$'\t'"$path"]:-}"; name_b="${GM_NAME["$b"$'\t'"$path"]:-}"
    if [ "$omode" = 160000 ] && [ "$nmode" = 160000 ]; then
      key_class "n:$name_a" "n:$name_b" "p:$path"
      classify_move "$CLS" "$path" "${name_b:-$name_a}" "$path" "$osha" "$nsha" "$tag"
    elif [ "$omode" = 160000 ]; then
      rm_path+=("$path"); rm_sha+=("$osha"); rm_name+=("$name_a"); rm_url+=("${GM_URL["$a"$'\t'"$path"]:-}")
    else
      ad_path+=("$path"); ad_sha+=("$nsha"); ad_name+=("$name_b"); ad_url+=("${GM_URL["$b"$'\t'"$path"]:-}")
    fi
  done < "$f"
  # pair renames: first (pass 1, every removal) by same non-empty .gitmodules name, then (pass 2) by
  # same canonical URL key; a URL key shared by more than one unpaired removal or more than one
  # unused addition cannot be paired honestly (which old pin went where?): BLIND ambiguous-pairing
  for j in "${!ad_path[@]}"; do canon_url "${ad_url[$j]}"; ad_key[j]="$CANON_KEY"; done
  for i in "${!rm_path[@]}"; do
    rm_key[i]=""; [ -n "${rm_url[$i]}" ] && { canon_url "${rm_url[$i]}"; rm_key[i]="$CANON_KEY"; }
    rm_pair[i]=""
    for j in "${!ad_path[@]}"; do
      [ -n "${ad_used[$j]:-}" ] && continue
      if [ -n "${rm_name[$i]}" ] && [ "${rm_name[$i]}" = "${ad_name[$j]}" ]; then rm_pair[i]="$j"; ad_used[j]=1; break; fi
    done
  done
  for i in "${!rm_path[@]}"; do
    [ -z "${rm_pair[$i]}" ] && [ -n "${rm_key[$i]}" ] || continue
    CANON_KEY="${rm_key[$i]}"
    nadd=0; nrm=0
    for j in "${!ad_path[@]}"; do
      [ -z "${ad_used[$j]:-}" ] && [ -n "${ad_key[$j]}" ] && [ "$CANON_KEY" = "${ad_key[$j]}" ] && nadd=$((nadd + 1))
    done
    for k in "${!rm_path[@]}"; do
      [ -z "${rm_pair[$k]}" ] && [ "${rm_key[$k]}" = "$CANON_KEY" ] && nrm=$((nrm + 1))
    done
    if [ -n "${amb[x$CANON_KEY]:-}" ] || [ "$nadd" -gt 1 ] || { [ "$nadd" -eq 1 ] && [ "$nrm" -gt 1 ]; }; then
      amb[x$CANON_KEY]=1; rm_pair[i]=amb; continue
    fi
    for j in "${!ad_path[@]}"; do
      [ -n "${ad_used[$j]:-}" ] && continue
      if [ -n "${ad_key[$j]}" ] && [ "$CANON_KEY" = "${ad_key[$j]}" ]; then rm_pair[i]="$j"; ad_used[j]=1; break; fi
    done
  done
  for i in "${!rm_path[@]}"; do
    paired="${rm_pair[$i]}"
    if [ "$paired" = amb ]; then
      key_class "n:${rm_name[$i]}" "p:${rm_path[$i]}"
      CHECKED=$((CHECKED + 1))
      echo "BLIND $CLS ${rm_path[$i]}=>? ${rm_sha[$i]} - commit=$tag reason=ambiguous-pairing"; count "$CLS" BLIND
      continue
    fi
    if [ -n "$paired" ]; then
      key_class "n:${rm_name[$i]}" "n:${ad_name[$paired]}" "p:${rm_path[$i]}" "p:${ad_path[$paired]}"
      classify_move "$CLS" "${rm_path[$i]}=>${ad_path[$paired]}" \
        "${ad_name[$paired]}" "${ad_path[$paired]}" "${rm_sha[$i]}" "${ad_sha[$paired]}" "$tag"
    else
      rm_left+=("$i")
    fi
  done
  # an addition left unpaired here may re-add a submodule an ANCESTOR commit of the range removed
  # (same .gitmodules name, else same URL key, e.g. a rename split over two commits): compare it
  # with each removed pin found, so remove + re-add older can never hide a backward move.
  # Removals of this commit go to NEWRM, seen only by its descendants (same-diff pairs are above).
  # The OLD -> NEW (RANGE) diff never uses remembered removals.
  [ "$tag" = RANGE ] || for j in "${!ad_path[@]}"; do
    [ -n "${ad_used[$j]:-}" ] && continue
    key_class "n:${ad_name[$j]}" "p:${ad_path[$j]}"
    readd_lookup "${ad_name[$j]}" "${ad_key[$j]}" "${ad_path[$j]}" "$CLS" "${ad_sha[$j]}"
    [ "$GONE_HIT" = none ] && continue
    ad_used[j]=1
    if [ "$GONE_HIT" = unid ]; then
      CHECKED=$((CHECKED + 1))
      echo "BLIND own ?=>${ad_path[$j]} - ${ad_sha[$j]} commit=$tag reason=readd-unidentified"; count own BLIND
      continue
    fi
    if [ "$GONE_HIT" = amb ]; then
      key_class "n:${ad_name[$j]}" "p:${ad_path[$j]}"
      CHECKED=$((CHECKED + 1))
      echo "BLIND $CLS ?=>${ad_path[$j]} - ${ad_sha[$j]} commit=$tag reason=readd-ambiguous"; count "$CLS" BLIND
      continue
    fi
    for g in $GONE_HIT; do
      key_class "n:${E_NAME[$g]}" "n:${ad_name[$j]}" "p:${E_PATH[$g]}" "p:${ad_path[$j]}"
      classify_move "$CLS" "${E_PATH[$g]}=>${ad_path[$j]}" \
        "${ad_name[$j]}" "${ad_path[$j]}" "${E_SHA[$g]}" "${ad_sha[$j]}" "$tag"
    done
  done
  # an own removal and an own addition left unpaired in one diff may be a rename we could not
  # recognise: the pins cannot be compared, so it is BLIND, never a silent REMOVED + ADDED
  own_add=0
  for j in "${!ad_path[@]}"; do
    [ -n "${ad_used[$j]:-}" ] && continue
    key_class "n:${ad_name[$j]}" "p:${ad_path[$j]}"; [ "$CLS" = own ] && own_add=1
  done
  for i in "${rm_left[@]}"; do
    if [ "$tag" != RANGE ]; then
      NEWRM+="${#E_SHA[@]}:b "
      E_SHA+=("${rm_sha[$i]}"); E_PATH+=("${rm_path[$i]}"); E_NAME+=("${rm_name[$i]}"); E_KEY+=("${rm_key[$i]}")
    fi
    key_class "n:${rm_name[$i]}" "p:${rm_path[$i]}"
    if [ "$CLS" = own ] && [ "$own_add" -eq 1 ]; then
      CHECKED=$((CHECKED + 1))
      echo "BLIND own ${rm_path[$i]}=>? ${rm_sha[$i]} - commit=$tag reason=unpaired-rename"; count own BLIND
    else
      echo "REMOVED $CLS ${rm_path[$i]} ${rm_sha[$i]} commit=$tag"
    fi
  done
  for j in "${!ad_path[@]}"; do
    [ -n "${ad_used[$j]:-}" ] && continue
    key_class "n:${ad_name[$j]}" "p:${ad_path[$j]}"
    echo "ADDED $CLS ${ad_path[$j]} ${ad_sha[$j]} commit=$tag"
  done
}

# scan REPO OLD NEW : every commit of OLD..NEW against every parent, then OLD tree -> NEW tree
scan() {
  local list commit parents p e i s
  CUR_REPO="$1"; CHECKED=0; OWN_FF=0; OWN_NONFF=0; OWN_BLIND=0; TP_FF=0; TP_NONFF=0; TP_BLIND=0; GIT_ERR=0
  GM_DONE=(); GM_NAME=(); GM_URL=(); OWN_KEY=(); TP_KEY=(); ORG_MATCH=0; RANGE_GL=0; GM_ERR=0
  E_SHA=(); E_PATH=(); E_NAME=(); E_KEY=(); GSET=(); WL=(); NEWRM=""
  list="$WORK/revs.$BASHPID.$RANDOM$RANDOM"
  if ! git -C "$CUR_REPO" rev-list --topo-order --reverse --parents "$2..$3" > "$list" 2>/dev/null; then
    echo "ERROR git rev-list $2..$3 failed"; GIT_ERR=1; return
  fi
  # organisation over the whole range: read .gitmodules of every tree the scan touches first
  parse_gm "$2"; parse_gm "$3"
  while read -r commit parents; do
    parse_gm "$commit"; for p in $parents; do parse_gm "$p"; done
  done < "$list"
  # parents come before children (--topo-order --reverse), so GSET of every in-range parent is final
  while read -r commit parents; do
    WL=()
    for p in $parents; do
      for e in ${GSET[$p]:-}; do i="${e%%:*}"; [ "${WL[$i]:-}" = b ] || WL[$i]="${e#*:}"; done
    done
    NEWRM=""
    for p in $parents; do check_diff "$p" "$commit" "$commit"; done
    s=""; for i in "${!WL[@]}"; do [ "${WL[$i]}" = x ] || s+="$i:${WL[$i]} "; done
    GSET[$commit]="$s$NEWRM"
  done < "$list"
  check_diff "$2" "$3" RANGE
}

# control needle: a known forward and a known backward move of a submodule whose URL carries the
# first organisation of the own-org list, through the same scan(); both must be classed own and the
# own-org list must have matched (a broken list or org parser makes the needle fail)
needle() {
  local N="$WORK/needle" org fw bw out
  org="$(printf '%s\n' "$OWN_ORGS" | sed -n 1p)"
  (
    set -e
    export GIT_AUTHOR_NAME=n GIT_AUTHOR_EMAIL=n@n GIT_COMMITTER_NAME=n GIT_COMMITTER_EMAIL=n@n
    g() { git -c core.hooksPath=/dev/null -c commit.gpgsign=false "$@"; }
    p="$N/parent"
    g init -q "$p"; g init -q "$p/needle_sub"
    g -C "$p/needle_sub" commit -q --allow-empty -m s1; s1="$(g -C "$p/needle_sub" rev-parse HEAD)"
    g -C "$p/needle_sub" commit -q --allow-empty -m s2; s2="$(g -C "$p/needle_sub" rev-parse HEAD)"
    printf '[submodule "needle_sub"]\n\tpath = needle_sub\n\turl = git@github.com:%s/needle_sub.git\n' "$org" > "$p/.gitmodules"
    g -C "$p" add .gitmodules; g -C "$p" commit -q -m root
    g -C "$p" update-index --add --cacheinfo "160000,$s1,needle_sub"; g -C "$p" commit -q -m pin
    g -C "$p" update-index --cacheinfo "160000,$s2,needle_sub"; g -C "$p" commit -q -m forward
    g -C "$p" update-index --cacheinfo "160000,$s1,needle_sub"; g -C "$p" commit -q -m backward
  ) >/dev/null 2>&1 || { echo "NEEDLE FAILED build"; return 2; }
  out="$(scan "$N/parent" "$(git -C "$N/parent" rev-parse HEAD~2)" "$(git -C "$N/parent" rev-parse HEAD)"; echo "ORGMATCH $ORG_MATCH")"
  fw="$(printf '%s\n' "$out" | sed -n 1p | cut -d' ' -f1-2)"
  bw="$(printf '%s\n' "$out" | sed -n 2p | cut -d' ' -f1-2)"
  echo "NEEDLE forward=${fw%% *} backward=${bw%% *} class=${fw#* }"
  # line 3 must be the ORGMATCH marker, which scan never prints and the needle prints last, so the
  # output is then exactly FF, NONFF, marker (any extra or missing scan line moves the marker)
  if [ "$fw" = "FF own" ] && [ "$bw" = "NONFF own" ] &&
     [ "$(printf '%s\n' "$out" | sed -n 3p)" = "ORGMATCH 1" ]; then echo "NEEDLE ok"; return 0; fi
  echo "NEEDLE FAILED"; return 2
}

[ $# -ge 1 ] || usage
load_orgs
command -v python3 >/dev/null 2>&1 || { echo "ERROR python3 not found (needed by $ORG_OF)" >&2; exit 2; }
[ -r "$ORG_OF" ] || { echo "ERROR org parser unreadable: $ORG_OF" >&2; exit 2; }

WORK="$(mktemp -d "${TMPDIR:-/tmp}/subff.XXXXXX")" || { echo "ERROR mktemp failed" >&2; exit 2; }
trap 'rm -rf "${WORK:?}"' EXIT

if [ "$1" = --selftest ]; then needle; exit $?; fi
if [[ "$*" == *...* ]]; then
  echo "ERROR symmetric range A...B is not supported; use OLD..NEW or OLD NEW" >&2; exit 64
fi
if [ $# -eq 1 ] && [[ "$1" == *..* ]]; then OLD="${1%%..*}"; NEW="${1#*..}"
elif [ $# -eq 2 ]; then OLD="$1"; NEW="$2"
else usage; fi

REPO="$IN_TOP"
[ -n "$REPO" ] || { echo "ERROR not inside a git work tree" >&2; exit 64; }
# the clean environment must reach the same git dir the caller's environment named
if [ "$(git -C "$REPO" rev-parse --absolute-git-dir 2>/dev/null)" != "$IN_GITDIR" ]; then
  echo "ERROR work tree $REPO does not resolve to the inherited git dir $IN_GITDIR" >&2; exit 64
fi
# info/grafts in the parent rewrites ancestry and ref resolution: refuse before resolving refs
PCOMMON="$(git -C "$REPO" rev-parse --path-format=absolute --git-common-dir 2>/dev/null)"
if [ -z "$PCOMMON" ] || [ -e "$PCOMMON/info/grafts" ]; then
  echo "ERROR parent repository has info/grafts (or no git dir): ancestry cannot be trusted"
  echo "VERDICT: BLIND reason=parent-grafts"; exit 2
fi
# a shallow parent cannot list the whole range or prove ancestry across its boundary
if [ "$(git -C "$REPO" rev-parse --is-shallow-repository 2>/dev/null)" != false ]; then
  echo "ERROR parent repository is shallow: the range cannot be proven"
  echo "VERDICT: BLIND reason=parent-shallow"; exit 2
fi
OLD_SHA="$(git -C "$REPO" rev-parse -q --verify "$OLD^{commit}" 2>/dev/null)" || { echo "ERROR unknown ref: $OLD" >&2; exit 64; }
NEW_SHA="$(git -C "$REPO" rev-parse -q --verify "$NEW^{commit}" 2>/dev/null)" || { echo "ERROR unknown ref: $NEW" >&2; exit 64; }

needle; nrc=$?
if [ "$nrc" -ne 0 ]; then echo "VERDICT: BLIND reason=control-needle-failed"; exit 2; fi

scan "$REPO" "$OLD_SHA" "$NEW_SHA"
# cross-check the RANGE diff against the endpoint trees themselves: ls-tree is not subject to any
# submodule "ignore" setting, so a gitlink path whose entry differs between OLD and NEW must have
# been seen by the RANGE diff; a mismatch means the diff was blinded (never read as nothing-checked)
LS_DIFF=""
if git -C "$REPO" ls-tree -r -z --full-tree "$OLD_SHA" > "$WORK/lt.old" 2>/dev/null &&
   git -C "$REPO" ls-tree -r -z --full-tree "$NEW_SHA" > "$WORK/lt.new" 2>/dev/null; then
  { grep -z '^160000 ' "$WORK/lt.old" || true; } | LC_ALL=C sort -z > "$WORK/gl.old"
  { grep -z '^160000 ' "$WORK/lt.new" || true; } | LC_ALL=C sort -z > "$WORK/gl.new"
  # comm -3 prefixes NEW-only records with a tab; then drop "<mode> <type> <sha><TAB>" to keep the path
  LS_DIFF="$(LC_ALL=C comm -z -3 "$WORK/gl.old" "$WORK/gl.new" |
    sed -z 's/^\t//; s/^[^\t]*\t//' | LC_ALL=C sort -zu | tr -cd '\0' | wc -c)"
fi
SUMMARY="checked=$CHECKED own_ff=$OWN_FF own_nonff=$OWN_NONFF own_blind=$OWN_BLIND third_party_ff=$TP_FF third_party_nonff=$TP_NONFF third_party_blind=$TP_BLIND"
[ "$CHECKED" -eq 0 ] && NOTE=" note=nothing-checked" || NOTE=""
if [ "$OWN_NONFF" -gt 0 ]; then echo "VERDICT: FAIL $SUMMARY$NOTE"; exit 1; fi
if [ "$GM_ERR" -ne 0 ]; then echo "VERDICT: BLIND $SUMMARY reason=gitmodules-unparsable"; exit 2; fi
if [ "$GIT_ERR" -ne 0 ] || [ -z "$LS_DIFF" ]; then echo "VERDICT: BLIND $SUMMARY reason=git-error"; exit 2; fi
if [ "$LS_DIFF" -ne "$RANGE_GL" ]; then
  echo "ERROR gitlink count mismatch: ls-tree OLD/NEW differ at $LS_DIFF gitlink path(s), the RANGE diff saw $RANGE_GL"
  echo "VERDICT: BLIND $SUMMARY reason=gitlink-diff-mismatch"; exit 2
fi
if [ "$OWN_BLIND" -gt 0 ]; then echo "VERDICT: BLIND $SUMMARY"; exit 2; fi
# a move was classed third-party but the own-org list matched no submodule URL anywhere in the
# range: a stale or wrong list classes own submodules third-party and passes silently, so refuse.
# Moves that are own anyway (relative or local URLs, no .gitmodules entry) cannot be hidden by a
# wrong list, so a range with only such moves is decided normally.
if [ "$CHECKED" -gt 0 ] && [ "$ORG_MATCH" -eq 0 ] && [ $((TP_FF + TP_NONFF + TP_BLIND)) -gt 0 ]; then echo "VERDICT: BLIND $SUMMARY reason=own-orgs-match-nothing"; exit 2; fi
echo "VERDICT: PASS $SUMMARY$NOTE"
exit 0
