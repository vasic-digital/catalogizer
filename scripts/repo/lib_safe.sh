#!/bin/bash -p
# lib_safe.sh - argument validators shared by the WP-04 CPA helpers (check_no_ci, check_revision_headers, scope_check,
# validate_cheap, commit_recursive, push_recursive). Sourced, never run.
#
# Why     a value that comes from .gitmodules, a remote name or URL, a declared path, a branch or a run id can reach a git or
#         shell argument. Each helper validates it with one of these functions BEFORE use and still places `--` before every
#         path or ref operand, so a value that slips past one layer is still a plain operand (an automated review flagged
#         argument injection in a sibling script).
# Rule    every function returns 0 (safe) or 1 (unsafe) and prints nothing; the caller refuses with exit 20 and names the value
#         with `%q`-style quoting. Control characters (including tab and newline, which would break the TSV stores and the
#         trailers) are unsafe everywhere.
# Never   none of these runs git or touches the file system, except safe_branch (git check-ref-format, no repository needed).
# Paths   a path a caller DECLARES (a change set, a commit path, a held row, a --repo operand) is a LITERAL path: safe_declpath refuses every
#         spelling that a path-matching layer could read differently from a literal (`.`, a `./` prefix, a `/./` or `/.` component, a trailing
#         `/`, the glob characters `* ? [`, a leading `:` pathspec magic). Callers also run git with GIT_LITERAL_PATHSPECS=1, so a value that
#         slips past this layer is still never a pattern. safe_relpath alone is NOT enough for a declared path (WF2 review B1).
LC_ALL=C
_LS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

_no_ctl() { case "$1" in *[[:cntrl:]]*) return 1 ;; esac; return 0; }

# safe_relpath <p>: relative path, no leading dash, no `..` component, no control character, no backslash.
safe_relpath() {
  local p="$1"
  [ -n "$p" ] || return 1
  _no_ctl "$p" || return 1
  case "$p" in /*|-*|*\\*|..|../*|*/..|*/../*|./..|*//*) return 1 ;; esac
  return 0
}
# safe_declpath <p>: a declared path: safe_relpath plus a canonical literal spelling (see the Paths note above).
safe_declpath() {
  safe_relpath "$1" || return 1
  case "$1" in .|./*|*/./*|*/.|*/|:*|*'*'*|*'?'*|*'['*) return 1 ;; esac
  return 0
}
# lr_exact <branch>: stdin is `git ls-remote` output; prints the object name of the ref refs/heads/<branch> EXACTLY (a tail-matching decoy
# such as refs/decoy/refs/heads/<branch> is never taken for it; WF2 review I3), nothing when the remote has no such ref.
lr_exact() { awk -v ref="refs/heads/$1" '$2==ref {print $1; exit}'; }
# safe_runid <id>: the run id of a CPA run (also a path component).
safe_runid() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; }
# ---- the ONE commit-message trailer parser (WF-R6 R-01) -----------------------------------------------------------------------------------------------------------------
# Every reader of a CPA commit message (the CPA-commit predicate of S1/S6/the merge path, the hold check of push_recursive, the held scan and the Foreign-Commit dedupe of
# integrate_merge, commit_recursive's own dedupe) goes through _CPA_TRL_AWK. There is NO second model: git's `interpret-trailers --parse` and `%(trailers)` stop at a message line
# that starts with `---` (a patch divider) and so describe a DIFFERENT block than a whole-message awk scan did; the review's `---` probe pushed a held commit whose Awaits-Review
# trailer one reader found and another did not. The grammar, as the writer (commit_recursive.sh, integrate_merge.sh) produces it:
#   <free text>  BLANK LINE  CPA-Run: <id>  [Deferred-Gates: ..] [Awaits-Review: ..] [Foreign-Commit: <sha>]...  [attribution lines]
# The TRAILER BLOCK is the FINAL paragraph (the lines after the last blank line; a blank line is empty or only spaces, tabs and CRs; trailing blank lines are dropped) and it MUST open
# with `CPA-Run: `. A message that has no such block has no trailers (status 1). Inside the block a line `Key: value` (Key = letter, then letters, digits, `-`) is a trailer, any other
# line is attribution text and is ignored, and the following make the block MALFORMED (status 3, every caller fails closed): a control character (a CR included), a folded line (leading
# space or tab), a line that starts with `---`, a repeated CPA-Run, Awaits-Review or Deferred-Gates key. A reserved key (CPA-Run, Awaits-Review, Deferred-Gates, Foreign-Commit; compared
# case-insensitively, with leading blanks allowed) on a line BEFORE the block, or in any spelling but the exact one INSIDE it (`awaits-review: v`, `Awaits-Review : v`, `Awaits-Review:v`), is also MALFORMED: the writer never puts one there, so it is somebody's text posing as a trailer.
# _cpa_git: git for every READ of a commit message. Configuration that alters what a read prints is neutralised here, not trusted: replace refs (a refs/replace entry can show another message for
# the commit), log.showSignature (adds gpg text to %B) and i18n.logOutputEncoding (re-encodes %B, a UTF-16 output is no text for awk).
_cpa_git() { git --no-replace-objects -c log.showSignature=false -c i18n.logOutputEncoding=UTF-8 "$@"; }
# cpa_commit_verify <repo dir> <message file> <run id> <expected Awaits-Review value or empty>: read back the commit that HEAD names right after `git commit` and refuse unless it holds EXACTLY the
# bytes the writer composed. Everything between the writer and the object can alter the message (commit.cleanup, core.commentChar/commentString, a commit-msg or prepare-commit-msg hook, i18n.commitEncoding
# with a non-ASCII message, an amend); a reader that parses the stored bytes then sees another message than the one that was intended (WP-04-T046-r2 F-01: a held commit lost its Awaits-Review line and was pushed).
# Status 0 verified; 1 with the reason on stdout.
cpa_commit_verify() {
  local d="$1" mf="$2" rid="$3" aw="$4" raw hdr got rc
  raw="$(mktemp "${TMPDIR:-/tmp}/cpa_cv.XXXXXX")" || { printf 'scratch_unavailable'; return 1; }
  if ! _cpa_git -C "$d" cat-file commit HEAD > "$raw" 2>/dev/null; then rm -f "$raw"; printf 'commit_unreadable'; return 1; fi
  hdr="$(sed -n '1,/^$/p' "$raw")"
  sed '1,/^$/d' "$raw" > "$raw.msg"
  if printf '%s\n' "$hdr" | grep -q '^encoding ' && LC_ALL=C grep -q '[^ -~[:space:]]' "$mf"; then rm -f "$raw" "$raw.msg"; printf 'commit_encoding_header_with_non_ascii_message'; return 1; fi
  if ! cmp -s "$mf" "$raw.msg"; then rm -f "$raw" "$raw.msg"; printf 'stored_message_differs_from_composed_message'; return 1; fi
  rm -f "$raw" "$raw.msg"
  got="$(cpa_trailer "$d" HEAD CPA-Run)" || { printf 'stored_message_has_no_readable_trailer_block'; return 1; }
  [ "$got" = "$rid" ] || { printf 'stored_cpa_run_differs'; return 1; }
  got="$(cpa_trailer "$d" HEAD Awaits-Review)"; rc=$?
  [ "$rc" = 0 ] && [ "$got" = "$aw" ] || { printf 'stored_awaits_review_differs'; return 1; }
  return 0
}
_CPA_TRL_AWK='
function blank(s) { return s ~ /^[ \t\r]*$/ }
function reserved(s,   k) { k = tolower(s); return k ~ /^[ \t]*(cpa-run|awaits-review|deferred-gates|foreign-commit)[ \t]*:/ }
# parse(n): the lines L[1..n] -> PST (0 block read, 1 no block, 3 malformed), NT trailers TK[i]/TV[i]
function parse(n,   s, i, k, v, seen) {
  NT = 0; PST = 0; delete seen
  while (n > 0 && blank(L[n])) n--
  s = n; while (s > 0 && !blank(L[s])) s--; s++
  if (n < 1 || index(L[s], "CPA-Run: ") != 1) { PST = 1; return }
  for (i = 1; i < s; i++) if (reserved(L[i])) { PST = 3; return }
  for (i = s; i <= n; i++) {
    if (L[i] ~ /[[:cntrl:]]/ || L[i] ~ /^[ \t]/ || L[i] ~ /^---/) { PST = 3; return }
    if (match(L[i], /^[A-Za-z][A-Za-z0-9-]*: /)) {
      k = substr(L[i], 1, RLENGTH - 2); v = substr(L[i], RLENGTH + 1)
      # a reserved key in any other spelling than the exact one (case, blanks before the colon, none after it) is somebody posing as a trailer: git reads some of those as holds
      if (reserved(L[i]) && k != "CPA-Run" && k != "Awaits-Review" && k != "Deferred-Gates" && k != "Foreign-Commit") { PST = 3; return }
      if (k == "CPA-Run" || k == "Awaits-Review" || k == "Deferred-Gates") { if (seen[k]++) { PST = 3; return } }
      NT++; TK[NT] = k; TV[NT] = v
    } else if (reserved(L[i])) { PST = 3; return }
  }
}
'
# cpa_trailer <repo dir> <sha> <Key>: prints the value of the LAST `Key:` trailer of the commit's trailer block (nothing when the block has none). Status 0 block read, 1 the message has no
# CPA trailer block, 2 git could not read the commit, 3 malformed block (see above). A caller treats 1/2/3 as "not a CPA commit" or refuses; none reads a value out of a status that is not 0.
cpa_trailer() {
  local msg
  msg="$(_cpa_git -C "$1" log -1 --format=%B "$2" 2>/dev/null; exit "${PIPESTATUS[0]}")" || return 2
  printf '%s\n' "$msg" | LC_ALL=C awk -v want="$3" "$_CPA_TRL_AWK"'
    { L[NR] = $0 }
    END { parse(NR); if (PST) exit PST; for (i = 1; i <= NT; i++) if (TK[i] == want) v = TV[i]; print v }'
}
# cpa_runid <repo dir> <sha>: the run id of the commit's `CPA-Run:` trailer (ONE parser for S1, S6 and the merge path), nothing and status 1 when there is no block, the block is malformed
# or the id is no safe run id (a value such as `../elsewhere` can never name a path outside the audit directory; WF3 review m3, X2, X3).
cpa_runid() { local id; id="$(cpa_trailer "$1" "$2" CPA-Run)" || return 1; safe_runid "$id" || return 1; printf '%s' "$id"; }
# cpa_hold_state <repo dir> <sha>: prints the Awaits-Review value of the commit's trailer block (may be empty). Status 0 read, 3 the commit cannot be judged (git failure, malformed block).
# A message WITHOUT a CPA block (status 1: a hand commit; it is refused elsewhere as unrecorded) counts as held when any of its lines starts with `Awaits-Review: `: the merge path holds more
# on such a commit, never less, and prints the marker `?` as the value.
cpa_hold_state() {
  local v rc
  v="$(cpa_trailer "$1" "$2" Awaits-Review)"; rc=$?
  case "$rc" in
    0) printf '%s' "$v"; return 0 ;;
    1) if _cpa_git -C "$1" log -1 --format=%B "$2" 2>/dev/null | grep -qi '^[[:space:]]*awaits-review[[:space:]]*:'; then printf '?'; fi; return 0 ;;
    *) return 3 ;;
  esac
}
# cpa_foreign_listed <repo dir> <rev> <sha>: status 0 when some commit reachable from <rev> names <sha> in a `Foreign-Commit:` trailer of its trailer block, 1 when none does, 2 when git fails.
# One git call: the messages come NUL-separated (git refuses a NUL inside a message), are re-separated with RS=0x1e (an 0x1e inside a message is turned into the control character 0x01 first, so a block that holds one is MALFORMED here exactly as it is for cpa_trailer; F-04) and parsed by the same awk.
cpa_foreign_listed() {
  local raw
  raw="$(_cpa_git -C "$1" log -z --format=%B "$2" -- 2>/dev/null | tr '\036\000' '\001\036'; exit "${PIPESTATUS[0]}")" || return 2
  printf '%s' "$raw" | LC_ALL=C awk -v want="$3" "$_CPA_TRL_AWK"'
    BEGIN { RS = "\036" }
    { n = split($0, L, "\n"); parse(n); if (PST == 0) for (i = 1; i <= NT; i++) if (TK[i] == "Foreign-Commit" && TV[i] == want) f = 1 }
    END { exit f ? 0 : 1 }'
}
# cpa_msg_check <text>: a message (or attribution text) the writers may put in front of the trailer block. Status 0 safe; status 1 with the reason on stdout otherwise: a control character
# other than newline and tab (a CR included), a line that starts with `---`, or a line that reads like one of the run's reserved trailers (case-insensitive, leading blanks allowed).
cpa_msg_check() {
  local r
  r="$(printf '%s\n' "$1" | LC_ALL=C awk '
    /[\001-\010\013-\037\177]/ { print "control_character"; bad = 1; exit }
    /^---/ { print "message_has_patch_divider"; bad = 1; exit }
    { k = tolower($0) }
    k ~ /^[ \t]*(cpa-run|awaits-review|deferred-gates|foreign-commit)[ \t]*:/ { print "message_impersonates_trailer"; bad = 1; exit }
    END { exit bad ? 1 : 0 }')" && return 0
  printf '%s' "$r"; return 1
}
# cpa_row <commits.tsv> <sha>: the run's commits.tsv holds a row whose second column is the sha (clause (a) of the CPA-commit predicate).
cpa_row() { [ -f "$1" ] && awk -F'\t' -v s="$2" '$2==s {f=1} END{exit !f}' "$1"; }
# gitlinks <repo dir>: the gitlink paths of the index, each NUL-terminated. Pure bash on purpose: an awk that reads or prints NUL (RS/ORS "\0", printf "\0")
# is not portable to the mawk of the pinned container images (WF3 review, container portability).
gitlinks() { local e; while IFS= read -r -d '' e; do case "$e" in 160000\ *) printf '%s\0' "${e#*$'\t'}" ;; esac; done < <(git -C "$1" ls-files -s -z 2>/dev/null); }
# gitlinks_ok <repo dir>: the index listing that gitlinks reads succeeds. gitlinks runs in a process substitution, so its own status is lost;
# every caller checks this FIRST and refuses 20 git_listing_failed, so a failing listing is never read as "no submodules" (WF5 review F3).
gitlinks_ok() { git -C "$1" ls-files -s -z >/dev/null 2>&1; }
# known_tip <repo dir> <remote> <branch>: the last fetched tip of refs/remotes/<remote>/<branch> when it is a commit held locally: what an UNREACHABLE
# remote last held as far as this clone knows. Prints nothing when there is none (the remote is then unknown, never "empty"; WF3 review I-1).
known_tip() { local t; t="$(git -C "$1" rev-parse -q --verify "refs/remotes/$2/$3^{commit}" 2>/dev/null)" || return 0; safe_sha "$t" && printf '%s' "$t"; return 0; }
# safe_remote <name>: a git remote name used as an operand of ls-remote / push.
safe_remote() { [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]*$ ]] && [ "${1: -5}" != ".lock" ]; }
# safe_url <u>: a remote URL; never an option-like value, never an `ext::` transport (it runs a command).
safe_url() { [ -n "$1" ] && _no_ctl "$1" && case "$1" in -*|ext::*|*"::"*) return 1 ;; esac; return 0; }
# safe_sha <s>: a full object name.
safe_sha() { [[ "$1" =~ ^[0-9a-f]{40}$ || "$1" =~ ^[0-9a-f]{64}$ ]]; }
# safe_branch <b>: a branch name that git itself accepts and that is no option.
safe_branch() { [ -n "$1" ] && case "$1" in -*) return 1 ;; esac; _no_ctl "$1" && git check-ref-format --branch "$1" >/dev/null 2>&1; }
# safe_line <s>: a single-line value for a trailer or a TSV field.
safe_line() { _no_ctl "$1"; }
# safe_dir_arg <d>: a directory operand given on a command line (repository, root, run dir): not option-like, no control character.
safe_dir_arg() { [ -n "$1" ] && _no_ctl "$1" && case "$1" in -*) return 1 ;; esac; return 0; }

# cpa_rundir_state <run dir>: the ONE predicate for the state of a CPA run directory (WF17-cpa MODEL-1: the launcher, its S0 close and the report listings use this and no other model of
# "is that run alive"). Prints `finished` (report.json exists), `closed` (interrupted.json exists: a successor proved the run dead and recorded it), `live` (run.pid names a pid whose
# /proc start ticks equal the recorded value; never a pgrep or cmdline match, never the pid embedded in the run id) or `interrupted`.
cpa_rundir_state() {
  local d="$1" pid st cur
  [ -f "$d/report.json" ] && { echo finished; return 0; }
  [ -f "$d/interrupted.json" ] && { echo closed; return 0; }
  { read -r pid st < "$d/run.pid"; } 2>/dev/null
  if [[ "${pid:-}" =~ ^[0-9]+$ ]] && [ "$pid" -gt 1 ] && [ -n "${st:-}" ]; then
    cur="$(sed -e 's/^.*) //' "/proc/$pid/stat" 2>/dev/null | awk '{print $20}')"
    if [ -n "$cur" ] && [ "$cur" = "$st" ]; then echo live; return 0; fi
  fi
  echo interrupted
}
# repo_owned <repo dir> <comma-separated own organisations>: the repository has at least one remote AND EVERY URL of every remote (`git remote get-url --all` and
# `--push --all`) names an organisation of the own list (WF17-cpa REPO-3a). One extra remote or one extra push URL that names a third party makes the repository NOT owned: owned
# means "everything this repository can push to is ours", never "some remote is ours". The organisation comes from scripts/audit/org_of.py, the one shared parser.
repo_owned() {
  local d="$1" own r u urlv org n=0 rl
  own=",$(printf '%s' "$2" | tr '[:upper:]' '[:lower:]'),"
  [ -n "$2" ] || return 1
  rl="$(git -C "$d" remote 2>/dev/null)" || return 1
  [ -n "$rl" ] || return 1
  for r in $rl; do
    safe_remote "$r" || return 1
    for u in "" "--push"; do
      while IFS= read -r urlv; do
        [ -n "$urlv" ] || continue; n=$((n+1))
        org="$(python3 -I "$_LS_DIR/../audit/org_of.py" "$urlv" 2>/dev/null | head -1 | tr '[:upper:]' '[:lower:]')"
        [ -n "$org" ] || return 1
        case "$own" in *",$org,"*) ;; *) return 1 ;; esac
      done < <(git -C "$d" remote get-url ${u:+$u} --all "$r" 2>/dev/null)
    done
  done
  [ "$n" -gt 0 ]
}
# repo_config_gate <repo dir> <allow table>: an S0 gate on the repository's OWN configuration (WF17-cpa REPO-2): every key of `git config --local --list` must match a row of the
# approved table (column 1, a glob), `include.path`/`includeIf.*` are never in it, `.git/info/attributes` and `config.worktree` must be absent or empty, and no .gitattributes of the
# work tree may name filter=, diff=, merge= or working-tree-encoding= (a clean filter, a diff driver or a re-encoding decides which BYTES a commit holds: judged bytes would not be committed
# bytes). Prints the first offence and returns 1.
repo_config_gate() {
  local d="$1" tbl="$2" k pat ok gd f cf
  [ -r "$tbl" ] || { echo "allow table unreadable"; return 1; }
  # the listing is written to a file first so its status is read: inside a process substitution a failing `git config --list` was "no keys" and the gate passed (WF-R6 R-06)
  cf="$(mktemp "${TMPDIR:-/tmp}/rcg.XXXXXX")" || { echo "$d: scratch file unavailable"; return 1; }
  if ! git -C "$d" config --local --list --name-only -z >"$cf" 2>/dev/null; then rm -f "$cf"; echo "$d: git config listing failed"; return 1; fi
  while IFS= read -r -d '' k; do
    ok=0
    while IFS=$'\t' read -r pat _; do
      case "$pat" in ''|'#'*) continue ;; esac
      # shellcheck disable=SC2254  # the row IS a glob
      case "$k" in $pat) ok=1; break ;; esac
    done < "$tbl"
    [ "$ok" = 1 ] || { rm -f "$cf"; echo "$d: config key $k is not in the approved table"; return 1; }
  done < "$cf"
  rm -f "$cf"
  gd="$(git -C "$d" rev-parse --absolute-git-dir 2>/dev/null)" || { echo "$d: git dir unreadable"; return 1; }
  for f in "$gd/info/attributes" "$gd/config.worktree"; do
    if [ -e "$f" ] || [ -L "$f" ]; then
      [ -f "$f" ] && [ ! -s "$f" ] || { echo "$d: $(basename "$f") is not empty"; return 1; }
    fi
  done
  while IFS= read -r -d '' f; do
    case "${f##*/}" in .gitattributes) ;; *) continue ;; esac
    if grep -qE '(^|[[:space:]])(filter|diff|merge|working-tree-encoding)=' "$d/$f" 2>/dev/null; then echo "$d: $f names filter=, diff=, merge= or working-tree-encoding="; return 1; fi
  done < <(git -C "$d" ls-files -z --cached --others --exclude-standard -- '*.gitattributes' 2>/dev/null)
  return 0
}
