#!/usr/bin/env bash
# verify_repos.sh - the single recursive, read-only repository verifier (T032; promoted from the spec 001 POC verify_repo.sh).
#
# Purpose  For the main repository and EVERY submodule at EVERY depth report: working-tree dirtiness (R1, including an
#          unexplained stash, R3), pin state (R4/R5), and for OWNED repositories each remote's tip compared with the local
#          commit (SAME, REMOTE-BEHIND = unpushed, LOCAL-BEHIND, DIVERGED (also R6: a rewritten remote tip), UNREACHABLE,
#          NO-REMOTE-BRANCH, UNKNOWN-DIFFERENT). Remote tips are read live with `git ls-remote`, never from tracking refs.
#          A DETACHED owned submodule is compared on its pinned commit (the gitlink recorded in its parent) AND on its HEAD
#          (so unpushed work past the pin is seen), against the tip of its .gitmodules branch, else the remote's default
#          branch, on every remote (R4); the worse of the two classes is reported. An ATTACHED owned submodule is compared on
#          its HEAD against its branch tip AND its recorded pin is checked for reachability (a pin the remote does not hold,
#          i.e. a pin ahead of, diverged from or unknown to the remote tip, is the `not our ref` condition of a fresh clone):
#          the worse class wins; a pin that is merely BEHIND the pushed tip is reachable and is not reported as a class, and a pin that ANY remote
#          branch or tag tip equals or descends from is HELD (a fresh clone fetches every branch and tag): only a pin no remote ref holds is reported. A repository with no remote at all
#          cannot be proven pushed: its row lists NO-REMOTE-BRANCH in `unproven` (the schema forbids remotes on a not-owned
#          row, so no pseudo remote is invented), exit 14.
#          Third-party repositories (no organisation of the own list on any remote) are never compared with their remotes.
#          Observes only: never fixes, commits, stashes, resets, or writes a ref, a working tree or FETCH_HEAD; it never
#          writes .git/index either (GIT_OPTIONAL_LOCKS=0: git status does not refresh the index).
#          Every git command whose answer the verdict depends on has its exit status checked: a repository that cannot be
#          examined (corrupt index, broken HEAD, a failing enumeration, a missing worker result, a report count that
#          cannot be read back: jq failed or answered a non-number) makes the run exit 20 with
#          a message naming it and writes NO report; it is never read as clean.
# Usage    verify_repos.sh [--root DIR] [--json FILE | --json-out FILE] [--fetch] [--no-remote] [--strict] [--jobs N]
#                          [--timeout SEC] [--owned-orgs a,b,c] [--exceptions FILE] [--quiet] [--self-test]
#            --root DIR   the repository root itself (a subdirectory of a repository is refused: exit 20)
#            --fetch      decide ancestry questions by fetching OBJECTS only:
#                         git fetch --no-tags --no-write-fetch-head --refmap= --no-recurse-submodules --no-auto-maintenance
#                         <remote> <branch>, with fetch.recurseSubmodules=false, maintenance.auto=false and gc.auto=0 forced on the
#                         command line (no submodule child fetch, so no submodule ref is written; no gc / `git maintenance` and so
#                         no detached job)
#            --no-remote  contact no remote: only dirty, pointer drift and uninitialised are decided
#            --strict     adds `behind` and `pin` to problems (and so to summary.failing); counts are the same in both modes
#            --jobs N     parallel workers, a positive integer (default 6); --timeout SEC per-remote seconds, a positive integer
#            --self-test  runs the control needle (it can be given anywhere on the line) and exits
# Inputs   scripts/audit/own_orgs.txt (one organisation per line, matched case-insensitively; default list, BLOCKED-ON ODG-15
#          for helixdevelopment1 and milos85vasic, see scripts/audit/own_orgs_pending.txt: rows of a pending account are noted
#          on stderr; the contract has no field for both classifications, see audit/owed-contract-items.md),
#          the organisation of a remote URL comes from scripts/audit/org_of.py (shared with derive_scope.sh),
#          exceptions scripts/repo/exceptions.tsv: path, kind, file, wt_sha256, blob_sha256, reason (see below).
#          Env VERIFY_GIT = git binary or shim.
# Exceptions  One row excepts ONE modified tracked file of ONE repository: kind "dirty", the repository path, the file path
#          inside it, the sha256 of the working-tree file, the sha256 of its committed blob (HEAD), a non-empty reason. A
#          repository is excepted only when EVERY change it shows is a ` M` (unstaged modification) of a file named by a row
#          whose two hashes both match NOW, and it has no stash and no untracked file. A stale hash, an extra change, a
#          stash, a staged change, another kind, an empty reason or a legacy 3-column row excepts nothing: the repository is
#          then an unexcepted dirty one (exit 13). Excepted rows stay counted in summary.dirty.
# Output   JSON in the repo-verification-report/1 shape (tool constant "verify_repo.sh"; mode booleans fetch, strict,
#          no_remote) to --json/--json-out FILE; a table and the summary on stdout unless --quiet; with --quiet and no
#          file the JSON goes to stdout.
# Exit     0 clean | 11 unpushed (ahead) | 12 diverged, or a `behind` row under --strict | 13 dirty (stash included) |
#          14 unverified remote (UNREACHABLE, UNKNOWN-DIFFERENT, NO-REMOTE-BRANCH incl. no remote at all; both modes) |
#          15 pointer drift or uninitialised submodule (--strict only) | 20 blind or unable verifier (control needle failed,
#          a git error, a missing repository result), usage or internal error.
#          Several failing classes at once: the first of 13, 12, 11, strict 15, strict behind (12) wins; any failing class
#          precedes 14 (decisions recorded in evidence/wp03/exit-code-decisions.md).
# Repository-configured code  A repository is data under examination and its configuration can name programs. What the verifier switches off, on EVERY git
#          command (`$GIT`): core.hooksPath=/dev/null (no hook of .git/hooks and none of a repository-set core.hooksPath: git runs the
#          reference-transaction hook even for a fetch that writes no ref), core.fsmonitor=false (a configured fsmonitor program), credential.helper
#          and core.askPass empty, core.gitProxy empty, protocol.ext.allow=never (an `ext::` URL, also one reached through url.<base>.insteadOf),
#          core.alternateRefsCommand=true (git fetch runs that program for a repository with an alternate object store; `true` lists no refs; WF7 M-5);
#          per repository, every configured filter.<name>.clean|smudge|process is overridden with an EMPTY command AND filter.<name>.required=false
#          (the status / ls-files / hash-object comparison then runs UNFILTERED: a file a filter normalises can read modified, never clean by mistake;
#          a REQUIRED driver with no command would make git die "clean filter failed" on every touched file, a false exit 20: WF7 I-1); per remote,
#          the explicit option --upload-pack=git-upload-pack on every ls-remote and fetch (a `-c remote.<n>.uploadpack=` would NOT do: git keeps the
#          FIRST of several values and the repository's own is first). NOT switched off, and stated rather than claimed: a remote
#          helper chosen by remote.<name>.vcs or by a `<helper>::` URL (such a URL is unclassifiable and never sent to git here), and
#          an ssh program named by the environment (GIT_SSH, not core.sshCommand: GIT_SSH_COMMAND set by the verifier wins over it).
#          The claim "no git command runs a hook" of earlier revisions was true for hooks only; the other surfaces were open until round 6.
# Side effects  --fetch writes objects into the object store only (no ref, no FETCH_HEAD, no submodule write, no gc/maintenance, no repository hook);
#          a temp dir (removed on exit). Needs git, jq, python3, sha256sum and a POSIX awk (mawk is enough: no `{n}` interval
#          expression and no NUL inside an awk printf format is used); it does not use the `column` command.
set -u
export GIT_OPTIONAL_LOCKS=0
# The path is part of the instrument (11.4.201(7)(c)): a repository-selecting variable inherited from the caller (a git hook exports
# GIT_DIR, GIT_INDEX_FILE, GIT_WORK_TREE ... into everything it runs) would make every `git -C <dir>` act on ANOTHER repository. Scrubbed
# FIRST, before any git call, argument parsing, worker dispatch or the control needle; workers inherit the scrubbed environment.
for _v in $(command git rev-parse --local-env-vars 2>/dev/null) GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_OBJECT_DIRECTORY \
          GIT_ALTERNATE_OBJECT_DIRECTORIES GIT_COMMON_DIR GIT_NAMESPACE GIT_PREFIX GIT_CONFIG GIT_CONFIG_PARAMETERS GIT_CONFIG_COUNT \
          GIT_CEILING_DIRECTORIES $(compgen -e | grep -E '^GIT_CONFIG_(KEY|VALUE)_[0-9]+$'); do unset "$_v"; done
unset _v
SELF="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/$(basename "${BASH_SOURCE[0]}")"
HERE="$(dirname "$SELF")"
ORGPY="$HERE/../audit/org_of.py"
GIT="${VERIFY_GIT:-git}"
# No repository hook ever runs in the verifier (R1, round 5): git runs the reference-transaction hook even for the empty transaction of a
# fetch that writes no ref, and a hook is code of the repository under examination. Every git command below goes through $GIT. Round 6
# (N6-4) widens this to the other repository-configured programs, see "Repository-configured code" in the header.
GIT="$GIT -c core.hooksPath=/dev/null -c core.fsmonitor=false -c credential.helper= -c core.askPass= -c core.gitProxy= -c protocol.ext.allow=never -c core.alternateRefsCommand=true"
ROOT="$(pwd)"; JSON_OUT=""; DO_FETCH=0; NO_REMOTE=0; STRICT=0; JOBS=6; TMO=25; QUIET=0; SELFTEST=0
OWNED_ORGS=""; EXC_FILE="$HERE/exceptions.tsv"
die20() { echo "verify_repos: $*" >&2; exit 20; }
# once the arguments are parsed (see below) a run that exits 20 must not leave the --json FILE of an EARLIER run behind: that report would contradict this exit status
# (WF7 M-6, WF8). A regular file or a symlink is removed (a symlink itself, never its target); a device such as /dev/null is never touched; a bad ARGUMENT exits
# through the first definition, before any run exists, and removes nothing.
die20_run() { echo "verify_repos: $*" >&2; if [ -n "$JSON_OUT" ] && { [ -f "$JSON_OUT" ] || [ -L "$JSON_OUT" ]; }; then rm -f -- "$JSON_OUT"; fi; exit 20; }
usage() { awk 'NR>=2 { if ($0 !~ /^#/) exit; print }' "$SELF" | sed 's/^# \{0,1\}//'; }
SSH_CMD='ssh -o BatchMode=yes -o ConnectTimeout=10'
# ONE status invocation, used by the workers AND by the control needle. --untracked-files=all is explicit so a repository-level
# status.showUntrackedFiles=no cannot hide untracked work; --ignore-submodules=all because submodules are rows of their own.
# -c core.fsmonitor=false / core.untrackedCache=false: a repository-level fsmonitor hook (or a stale monitor daemon) that reports "nothing
# changed" must not make a modified file read clean (11.4.201(7)(c): the path is part of the instrument); the same pair is passed to the
# other index-reading commands below (diff-index, ls-files -v) through GITQ.
GITQ=(-c core.fsmonitor=false -c core.untrackedCache=false)
STATUS_ARGS=("${GITQ[@]}" status --porcelain -z --ignore-submodules=all --untracked-files=all)

# ---------------------------------------------------------------- worker: one repository -> one JSON file (or one .err file)
worker() {
  local idx="$1" rel="$2" pin="$3" outdir="$4" abs head="" branch="" tracked=0 untracked=0 stash=0 dirty=false owned=false
  local r url org rj="[]" nrem=0 rc ent xy excepted=false exc_reason="" top unclass=false i
  fail() { printf '%s\n' "cannot verify $rel: $*" > "${outdir:?}/${idx:?}.err"; return 1; }
  [ "$rel" = "." ] && abs="$ROOT" || abs="$ROOT/$rel"
  # a value that reaches a git argument (a branch from .gitmodules or from a remote's HEAD symref, a remote name) must be a
  # plain ref name: not empty, no leading '-', no whitespace, accepted by git check-ref-format. Refusal = unproven, never clean.
  valid_branch() { [ -n "$1" ] && case "$1" in -*|*[[:space:]]*) return 1 ;; esac && $GIT check-ref-format --branch "$1" >/dev/null 2>&1; }
  valid_remote() { [ -n "$1" ] && case "$1" in -*|*[[:space:]]*) return 1 ;; esac && $GIT check-ref-format "refs/remotes/$1/x" >/dev/null 2>&1; }
  if [ "$pin" = "-" ]; then   # uninitialised submodule: the directory is empty, git would resolve to the PARENT repository
    jq -nc --arg path "$rel" --arg pin "$pin" '{path:$path,pin:$pin,head:"",branch:"",owned:false,dirty_tracked:0,dirty_untracked:0,dirty:false,
        excepted:false,exception_reason:null,remotes:[],noremote_unproven:false}' > "${outdir:?}/${idx:?}.json"; return 0
  fi
  if [ "$rel" != "." ]; then [ -e "$abs/.git" ] || { fail "no .git entry (pin '$pin'); not a checked-out repository"; return 1; }; fi
  # every configured filter driver is switched off for this worker's git commands (an empty command is no filter): git config injected through the
  # environment applies to every git child of the worker, including those in pipelines (N6-4)
  local fk fnm nfk=0; declare -A fseen=(); GIT_CONFIG_COUNT=0
  while IFS= read -r fk; do
    [ -n "$fk" ] || continue
    export "GIT_CONFIG_KEY_$nfk=$fk" "GIT_CONFIG_VALUE_$nfk="; nfk=$((nfk+1))
    # a REQUIRED driver without a command is a hard git error, not "no filter": every driver name also gets required=false (WF7 I-1; the name may contain dots)
    fnm="${fk#filter.}"; fnm="${fnm%.*}"
    if [ -z "${fseen[$fnm]+x}" ]; then fseen["$fnm"]=1; export "GIT_CONFIG_KEY_$nfk=filter.$fnm.required" "GIT_CONFIG_VALUE_$nfk=false"; nfk=$((nfk+1)); fi
  done < <($GIT -C "$abs" config --name-only --get-regexp '^filter\..*\.(clean|smudge|process)$' 2>/dev/null)
  export GIT_CONFIG_COUNT=$nfk
  top="$($GIT -C "$abs" rev-parse --show-toplevel 2>/dev/null)" || { fail "git rev-parse --show-toplevel failed"; return 1; }
  [ "$(cd "$top" 2>/dev/null && pwd -P)" = "$(cd "$abs" 2>/dev/null && pwd -P)" ] || { fail "git resolves this path to another repository ($top)"; return 1; }
  head="$($GIT -C "$abs" rev-parse HEAD 2>/dev/null)" || { fail "git rev-parse HEAD failed"; return 1; }
  [ -n "$head" ] || { fail "empty HEAD"; return 1; }
  branch="$($GIT -C "$abs" symbolic-ref --short -q HEAD 2>/dev/null)"; rc=$?
  case "$rc" in 0) ;; 1) branch="" ;; *) fail "git symbolic-ref failed (rc=$rc)"; return 1 ;; esac
  $GIT -C "$abs" "${STATUS_ARGS[@]}" > "${outdir:?}/${idx:?}.status" 2>"${outdir:?}/${idx:?}.status.err" \
    || { fail "git status failed: $(head -c 160 "${outdir:?}/${idx:?}.status.err" | tr '\n' ' ')"; return 1; }
  local -a cands=()
  while IFS= read -r -d '' ent; do
    xy="${ent:0:2}"
    case "$xy" in "??") untracked=$((untracked+1)) ;; *) tracked=$((tracked+1)); cands+=("$xy ${ent:3}") ;; esac
    case "$xy" in R*|C*|?R|?C) IFS= read -r -d '' ent || true ;; esac   # a rename or copy carries the old path as one more field
  done < "${outdir:?}/${idx:?}.status"
  # a repository-level status.showUntrackedFiles=no is overridden by the explicit --untracked-files=all above; it is only noted
  local suf; suf="$($GIT -C "$abs" config --get status.showUntrackedFiles 2>/dev/null)" || suf=""
  case "$suf" in no|NO|No|false) echo "verify_repos: note: $rel: status.showUntrackedFiles=$suf is configured; overridden (--untracked-files=all is passed explicitly)" >&2 ;; esac
  # a STAGED submodule pointer change: `status --ignore-submodules=all` hides it and `git submodule status` compares the submodule
  # HEAD with the INDEX, so after `git add <sub>` neither shows it. Compare the index with HEAD for gitlink entries (mode 160000).
  local meta pth st m1 m2
  $GIT "${GITQ[@]}" -C "$abs" diff-index --cached --raw -z --ignore-submodules=none HEAD > "${outdir:?}/${idx:?}.cached" 2>"${outdir:?}/${idx:?}.cached.err" \
    || { fail "git diff-index --cached failed: $(head -c 160 "${outdir:?}/${idx:?}.cached.err" | tr '\n' ' ')"; return 1; }
  while IFS= read -r -d '' meta; do
    IFS= read -r -d '' pth || break
    st="${meta##* }"; case "$st" in R*|C*) IFS= read -r -d '' pth || true ;; esac
    m1="${meta:1:6}"; m2="${meta:8:6}"
    if [ "$m1" = 160000 ] || [ "$m2" = 160000 ]; then tracked=$((tracked+1)); cands+=("G  $pth"); fi
  done < "${outdir:?}/${idx:?}.cached"
  # skip-worktree / assume-unchanged entries are invisible to `git status`: compare the ones present on disk with their index blob
  # (an ABSENT skip-worktree file is a sparse checkout, not dirt; an absent assume-unchanged file is a deletion git is told to ignore)
  local tag lf ish wh
  $GIT "${GITQ[@]}" -C "$abs" ls-files -v -z > "${outdir:?}/${idx:?}.lsv" 2>"${outdir:?}/${idx:?}.lsv.err" \
    || { fail "git ls-files -v failed: $(head -c 160 "${outdir:?}/${idx:?}.lsv.err" | tr '\n' ' ')"; return 1; }
  while IFS= read -r -d '' ent; do
    tag="${ent:0:1}"; lf="${ent:2}"
    case "$tag" in S|[a-z]) ;; *) continue ;; esac
    # GIT_LITERAL_PATHSPECS: a name with glob metacharacters (`x[1]`) is a file name here, never a pattern that selects a sibling
    ish="$(GIT_LITERAL_PATHSPECS=1 $GIT -C "$abs" ls-files -s -z -- "$lf" | tr '\0' '\n'; exit "${PIPESTATUS[0]}")" || { fail "git ls-files -s failed for $lf"; return 1; }
    ish="${ish%%$'\n'*}"
    case "$ish" in 160000*) continue ;; esac
    ish="$(printf '%s' "$ish" | awk '{print $2}')"
    if [ -L "$abs/$lf" ]; then wh="$(printf '%s' "$(readlink -- "$abs/$lf")" | $GIT -C "$abs" hash-object --stdin)" || { fail "git hash-object failed for $lf"; return 1; }
    elif [ -f "$abs/$lf" ]; then wh="$($GIT -C "$abs" hash-object -- "$lf")" || { fail "git hash-object failed for $lf"; return 1; }
    else wh=""; [ "$tag" = S ] && continue; fi
    if [ "$wh" != "$ish" ]; then tracked=$((tracked+1)); cands+=("!M $lf"); fi
  done < <(awk 'BEGIN{RS="\0";ORS="\0"} substr($0,1,2)!="H "' "${outdir:?}/${idx:?}.lsv")
  stash="$($GIT -C "$abs" stash list 2>/dev/null)" || { fail "git stash list failed"; return 1; }
  stash="$(printf '%s' "$stash" | grep -c . )"
  dirty=false; [ $((tracked + untracked + stash)) -gt 0 ] && dirty=true
  # --- exceptions (only for a repository whose ONLY dirt is unstaged modifications of files named with matching hashes)
  if [ "$dirty" = true ] && [ "$stash" = 0 ] && [ "$untracked" = 0 ] && [ -f "$EXC_FILE" ]; then
    local c f wth bth ok_all=true reasons="" r_wt r_bl r_re matched hit
    for c in "${cands[@]}"; do
      xy="${c:0:2}"; f="${c:3}"; matched=""; hit=0
      if [ "$xy" = " M" ] && [ -f "$abs/$f" ] && [ ! -L "$abs/$f" ] && $GIT -C "$abs" cat-file -e "HEAD:$f" 2>/dev/null; then
        wth="$(sha256sum < "$abs/$f" | cut -c1-64)"
        bth="$($GIT -C "$abs" cat-file blob "HEAD:$f" 2>/dev/null | sha256sum | cut -c1-64)"
        while IFS=$'\t' read -r r_wt r_bl r_re; do
          [ "$r_wt" = "$wth" ] && [ "$r_bl" = "$bth" ] && { matched="$r_re"; hit=1; break; }
        done < <(awk -F'\t' -v p="$rel" -v f="$f" '$0!~/^#/ && NF>=6 && $1==p && $2=="dirty" && $3==f && length($4)==64 && $4!~/[^0-9a-f]/ && length($5)==64 && $5!~/[^0-9a-f]/ && length($6)>0 {print $4 "\t" $5 "\t" $6}' "$EXC_FILE")
      fi
      if [ "$hit" = 0 ]; then ok_all=false; break; fi
      case "; $reasons;" in *"; $matched;"*) ;; *) reasons="${reasons:+$reasons; }$matched" ;; esac
    done
    [ "$ok_all" = true ] && [ "${#cands[@]}" -gt 0 ] && { excepted=true; exc_reason="$reasons"; }
  fi
  # --- owned: any remote whose organisation (shared parser, case-insensitive, trailing-slash safe) is in the own list
  $GIT -C "$abs" remote > "${outdir:?}/${idx:?}.remotes" 2>/dev/null || { fail "git remote failed"; return 1; }
  local -a urls=() rnames=() orgs=() rlist=()
  mapfile -t rlist < "${outdir:?}/${idx:?}.remotes"   # one remote per LINE: a name with whitespace stays one name (valid_remote refuses it)
  for r in "${rlist[@]}"; do
    [ -n "$r" ] || continue
    url="$($GIT -C "$abs" remote get-url -- "$r" 2>/dev/null)" || { fail "git remote get-url $r failed"; return 1; }
    urls+=("$url"); rnames+=("$r")
  done
  nrem="${#urls[@]}"
  if [ "$nrem" -gt 0 ]; then
    mapfile -t orgs < <(python3 "$ORGPY" "${urls[@]}" 2>/dev/null)
    [ "${#orgs[@]}" = "$nrem" ] || { fail "organisation parser returned ${#orgs[@]} answers for $nrem remotes"; return 1; }
    for i in "${!orgs[@]}"; do
      org="${orgs[$i]}"
      if [ -z "$org" ]; then unclass=true; echo "verify_repos: note: $rel: cannot classify the organisation of remote ${rnames[$i]} URL '${urls[$i]}'; reported unproven" >&2
      else case ",$OWNED_ORGS," in *",$org,"*) owned=true ;; esac; fi
    done
  fi
  if [ "$owned" = true ] && [ "$NO_REMOTE" = 0 ]; then
    local sup="" subname="" want_branch="$branch" cmp="$head" pinned="" refused="" attpin=""
    # R4 for EVERY owned submodule, attached or detached: the parent's recorded gitlink must be reachable from a remote tip
    sup="$($GIT -C "$abs" rev-parse --show-superproject-working-tree 2>/dev/null)" || { fail "git rev-parse --show-superproject-working-tree failed"; return 1; }
    if [ -n "$sup" ] && [ -z "$branch" ]; then   # detached owned submodule: R4 against the .gitmodules branch, else the remote default branch
      if [ -f "$sup/.gitmodules" ]; then
        local abs_p sup_p relsub gm
        abs_p="$(cd "$abs" && pwd -P)"; sup_p="$(cd "$sup" && pwd -P)"; relsub="${abs_p#"$sup_p"/}"
        gm="$($GIT config -f "$sup/.gitmodules" --get-regexp '^submodule\..*\.path$' 2>/dev/null)"; rc=$?
        [ "$rc" -le 1 ] || { fail "git config -f .gitmodules failed (rc=$rc)"; return 1; }
        subname="$(printf '%s\n' "$gm" | awk -v p="$relsub" '{k=$1; v=substr($0, length($1)+2)} v==p{n=k; sub(/^submodule\./,"",n); sub(/\.path$/,"",n); print n; exit}')"
        if [ -n "$subname" ]; then
          want_branch="$($GIT config -f "$sup/.gitmodules" "submodule.$subname.branch" 2>/dev/null)"; rc=$?
          [ "$rc" -le 1 ] || { fail "git config .gitmodules branch failed (rc=$rc)"; return 1; }
          [ "$rc" = 0 ] && ! valid_branch "$want_branch" && { refused="the .gitmodules branch value '$want_branch'"; want_branch=""; }
        fi
        pinned="$($GIT -C "$sup" ls-tree HEAD -- "$relsub" 2>/dev/null)" || { fail "git ls-tree HEAD -- $relsub failed in the superproject"; return 1; }
        pinned="$(printf '%s\n' "$pinned" | awk '{print $3}')"
        [ -n "$pinned" ] && cmp="$pinned"
      fi
    elif [ -n "$sup" ]; then   # ATTACHED owned submodule: HEAD is compared on its branch; the recorded pin is checked for reachability too (I-4)
      local abs_q sup_q relsub_q
      abs_q="$(cd "$abs" && pwd -P)"; sup_q="$(cd "$sup" && pwd -P)"; relsub_q="${abs_q#"$sup_q"/}"
      attpin="$($GIT -C "$sup" ls-tree HEAD -- "$relsub_q" 2>/dev/null)" || { fail "git ls-tree HEAD -- $relsub_q failed in the superproject"; return 1; }
      attpin="$(printf '%s\n' "$attpin" | awk '$2=="commit"{print $3}')"
      [ "$attpin" = "$head" ] && attpin=""   # the pin is the HEAD already compared
    fi
    rank() { case "$1" in DIVERGED) echo 5;; REMOTE-BEHIND) echo 4;; UNKNOWN-DIFFERENT) echo 3;; LOCAL-BEHIND) echo 2;; *) echo 1;; esac; }
    classify() {  # classify <commit> <tip> -> class (a failing ancestry command is a worker failure, never a class)
      local c="$1" t="$2" a
      if [ "$t" = "$c" ]; then echo SAME; return 0; fi
      $GIT -C "$abs" cat-file -e "${t}^{commit}" 2>/dev/null || { echo UNKNOWN-DIFFERENT; return 0; }
      $GIT -C "$abs" cat-file -e "${c}^{commit}" 2>/dev/null || { echo UNKNOWN-DIFFERENT; return 0; }
      $GIT -C "$abs" merge-base --is-ancestor "$t" "$c" 2>/dev/null; a=$?
      case "$a" in 0) echo REMOTE-BEHIND; return 0 ;; 1) ;; *) return 1 ;; esac
      $GIT -C "$abs" merge-base --is-ancestor "$c" "$t" 2>/dev/null; a=$?
      case "$a" in 0) echo LOCAL-BEHIND; return 0 ;; 1) echo DIVERGED; return 0 ;; *) return 1 ;; esac
    }
    for i in "${!rnames[@]}"; do
      r="${rnames[$i]}"; url="${urls[$i]}"
      local out rc2=0 tip="" cls cls_h cls_p held alltips tl tt a rbranch="$want_branch" why="$refused"
      if [ -z "$why" ] && ! valid_remote "$r"; then why="the remote name '$r'"; fi
      if [ -n "$why" ]; then :   # refused below: nothing is sent to git
      elif [ -n "$rbranch" ]; then
        out="$(GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="$SSH_CMD" timeout "$TMO" $GIT -C "$abs" ls-remote --upload-pack=git-upload-pack -- "$r" "refs/heads/$rbranch" 2>/dev/null)"; rc2=$?
        # git ls-remote matches the TAIL of every advertised ref (refs/archive/refs/heads/main answers a request for refs/heads/main): only the
        # line whose ref field is EXACTLY refs/heads/<branch> is the branch tip (11.4.201(9) field identity)
        tip="$(printf '%s\n' "$out" | awk -v want="refs/heads/$rbranch" 'NF==2 && $2==want && length($1)==40 && $1!~/[^0-9a-f]/ {print $1; exit}')"
      else   # no branch known: the remote's default branch (HEAD)
        out="$(GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="$SSH_CMD" timeout "$TMO" $GIT -C "$abs" ls-remote --symref --upload-pack=git-upload-pack -- "$r" HEAD 2>/dev/null)"; rc2=$?
        rbranch="$(printf '%s\n' "$out" | sed -nE 's#^ref: refs/heads/([^[:space:]]+)[[:space:]]+HEAD$#\1#p' | head -1)"
        if [ -n "$rbranch" ] && ! valid_branch "$rbranch"; then why="the default branch name '$rbranch' announced by remote $r"; fi
        tip="$(printf '%s\n' "$out" | awk 'NF==2 && $2=="HEAD" && length($1)==40 && $1!~/[^0-9a-f]/ {print $1; exit}')"
      fi
      if [ -n "$why" ]; then cls="NO-REMOTE-BRANCH"; tip=""; echo "verify_repos: note: $rel: refused $why (not a plain branch/remote name); not sent to git, reported unproven" >&2
      elif [ "$rc2" -ne 0 ]; then cls="UNREACHABLE"
      elif [ -z "$tip" ]; then cls="NO-REMOTE-BRANCH"
      else
        if [ "$tip" != "$cmp" ] || [ "$tip" != "$head" ]; then
          if [ "$DO_FETCH" = 1 ] && [ -n "$rbranch" ]; then
            GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="$SSH_CMD" timeout "$TMO" \
              $GIT -c fetch.recurseSubmodules=false -c maintenance.auto=false -c gc.auto=0 -C "$abs" fetch --no-recurse-submodules --no-auto-maintenance \
                   --upload-pack=git-upload-pack --no-tags --no-write-fetch-head --refmap= --quiet -- "$r" "refs/heads/$rbranch" >/dev/null 2>&1
          fi
        fi
        cls="$(classify "$cmp" "$tip")" || { fail "git merge-base failed against remote $r"; return 1; }
        if [ "$cmp" != "$head" ]; then   # detached: HEAD past (or away from) the pin is compared too; the worse class wins
          cls_h="$(classify "$head" "$tip")" || { fail "git merge-base (HEAD) failed against remote $r"; return 1; }
          [ "$(rank "$cls_h")" -gt "$(rank "$cls")" ] && cls="$cls_h"
        fi
        if [ -n "$attpin" ]; then   # attached: only the classes that mean "the remote does not hold the pin" count; BEHIND the tip is reachable
          cls_p="$(classify "$attpin" "$tip")" || { fail "git merge-base (pin) failed against remote $r"; return 1; }
          case "$cls_p" in REMOTE-BEHIND|DIVERGED|UNKNOWN-DIFFERENT)
            if [ "$(rank "$cls_p")" -gt "$(rank "$cls")" ]; then
              # R3 (round 5): not being on the compared branch is not "the remote does not hold the pin": the pin is held when ANY remote branch or
              # tag tip equals it or descends from it (a fresh clone fetches every branch and tag, and submodule update then finds the commit).
              held=0; undecided=0; alltips="$(GIT_TERMINAL_PROMPT=0 GIT_SSH_COMMAND="$SSH_CMD" timeout "$TMO" $GIT -C "$abs" ls-remote --upload-pack=git-upload-pack -- "$r" 'refs/heads/*' 'refs/tags/*' 2>/dev/null)" || alltips=""
              # an annotated tag is advertised twice: the tag OBJECT (refs/tags/v1) and its peeled commit (refs/tags/v1^{}). The object line is decided by
              # its peeled twin, never "a tip this clone does not hold" (WF7 I-2)
              alltips="$(printf '%s\n' "$alltips" | awk '{ r[$2]=1; l[NR]=$0; n[NR]=$2 } END { for (i=1;i<=NR;i++) { if (n[i] ~ /^refs\/tags\// && n[i] !~ /\^\{\}$/ && ((n[i] "^{}") in r)) continue; print l[i] } }')"
              while IFS= read -r tl; do
                tt="${tl%%[[:space:]]*}"; [ "${#tt}" = 40 ] && [ -z "${tt//[0-9a-f]/}" ] || continue
                # the pattern match of ls-remote is a TAIL match (refs/archive/refs/heads/old answers `refs/heads/*`): only a ref whose name STARTS with
                # refs/heads/ or refs/tags/ is a branch or tag tip (N6-1, 11.4.201(9) field identity)
                tref="${tl#*[[:space:]]}"; case "$tref" in refs/heads/*|refs/tags/*) ;; *) continue ;; esac
                if [ "$tt" = "$attpin" ]; then held=1; break; fi
                # a tip or the pin whose object this clone does not hold cannot be compared: that leaves the question UNDECIDED, which is
                # UNKNOWN-DIFFERENT below, never a definite "the remote does not hold the pin" (N6-2)
                if ! $GIT -C "$abs" cat-file -e "${tt}^{commit}" 2>/dev/null; then
                  # a HELD blob or tree (a tag on a non-commit) can never descend from the pin: decided, not undecided (WF7 I-2); anything else is not held
                  case "$($GIT -C "$abs" cat-file -t "$tt" 2>/dev/null)" in blob|tree) continue ;; esac
                  undecided=1; continue
                fi
                $GIT -C "$abs" cat-file -e "${attpin}^{commit}" 2>/dev/null || { undecided=1; continue; }
                $GIT -C "$abs" merge-base --is-ancestor "$attpin" "$tt" 2>/dev/null; a=$?
                case "$a" in 0) held=1; break ;; 1) ;; *) fail "git merge-base (pin vs a remote ref) failed against remote $r"; return 1 ;; esac
              done <<<"$alltips"
              if [ "$held" != 1 ]; then
                [ "$undecided" = 1 ] && cls_p=UNKNOWN-DIFFERENT
                [ "$(rank "$cls_p")" -gt "$(rank "$cls")" ] && cls="$cls_p"
              fi
            fi ;;
          esac
        fi
      fi
      rj="$(jq -c --arg r "$r" --arg u "$url" --arg t "$tip" --arg c "$cls" \
            '. + [{remote:$r,url:$u,remote_tip:$t,class:$c}]' <<<"$rj")"
    done
  fi
  local noremote_unproven=false
  [ "$nrem" -eq 0 ] && [ "$NO_REMOTE" = 0 ] && noremote_unproven=true   # no remote at all: nothing can be shown pushed, unproven, never clean
  [ "$unclass" = true ] && [ "$NO_REMOTE" = 0 ] && noremote_unproven=true   # a remote whose organisation cannot be read is never silently third-party
  jq -nc --arg path "$rel" --arg pin "$pin" --arg head "$head" --arg branch "$branch" --arg reason "$exc_reason" \
     --argjson owned "$owned" --argjson tracked "$tracked" --argjson untracked "$untracked" --argjson dirty "$dirty" \
     --argjson excepted "$excepted" --argjson remotes "$rj" --argjson nru "$noremote_unproven" \
     '{noremote_unproven:$nru,path:$path,pin:$pin,head:$head,branch:$branch,owned:$owned,dirty_tracked:$tracked,
       dirty_untracked:$untracked,dirty:$dirty,excepted:$excepted,exception_reason:(if $excepted then $reason else null end),remotes:$remotes}' \
     > "${outdir:?}/${idx:?}.json" || { fail "report row assembly failed"; return 1; }
  return 0
}

if [ "${1:-}" = "--worker" ]; then
  shift; ROOT="$1"; shift; DO_FETCH="$1"; shift; NO_REMOTE="$1"; shift; TMO="$1"; shift; OWNED_ORGS="$1"; shift; EXC_FILE="$1"; shift
  worker "$@"; exit 0
fi

# ---------------------------------------------------------------- control needle (blind-verifier guard, 11.4.201)
# The needle runs the SAME status invocation (STATUS_ARGS) as the worker, in a throwaway repository created inside a scrubbed
# environment with hooks disabled and NO commit (it never writes outside its own temp dir): a clean one MUST read clean, one with a
# known-present untracked file MUST read dirty, and one whose index is corrupt MUST make the command FAIL (so a git error
# can never be mistaken for a clean tree on the real path: the worker checks the same exit status on every real repository).
needle() {
  local N G2 o1 o2 r3; N="$(mktemp -d)" || return 1
  G2="$GIT -c core.hooksPath=/dev/null"   # hooks never run here; the needle needs no commit, so it writes nothing outside $N
  $G2 -C "$N" init -q --template= . 2>/dev/null || { rm -rf "${N:?}"; return 1; }
  o1="$($G2 -C "$N" "${STATUS_ARGS[@]}" 2>/dev/null | tr '\0' '\n' | grep -c .)"
  echo needle > "$N/needle.txt"
  o2="$($G2 -C "$N" "${STATUS_ARGS[@]}" 2>/dev/null | tr '\0' '\n' | grep -c '^??')"
  printf 'garbage' > "$N/.git/index"
  $G2 -C "$N" "${STATUS_ARGS[@]}" >/dev/null 2>&1; r3=$?
  rm -rf "${N:?}"
  [ "$o1" = 0 ] && [ "$o2" = 1 ] && [ "$r3" != 0 ]
}

# ---------------------------------------------------------------- arg parsing
posint() { case "$2" in ''|*[!0-9]*|0|0[0-9]*) die20 "$1 needs a positive integer, got '$2'" ;; esac; }
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || die20 "--root needs a value"; ROOT="$(cd "$2" 2>/dev/null && pwd)" || die20 "no such directory: $2"; shift 2 ;;
    --json|--json-out) [ $# -ge 2 ] || die20 "$1 needs a value"; JSON_OUT="$2"; shift 2 ;;
    --fetch) DO_FETCH=1; shift ;;
    --no-remote) NO_REMOTE=1; shift ;;
    --strict) STRICT=1; shift ;;
    --jobs) [ $# -ge 2 ] || die20 "--jobs needs a value"; posint --jobs "$2"; JOBS="$2"; shift 2 ;;
    --timeout) [ $# -ge 2 ] || die20 "--timeout needs a value"; posint --timeout "$2"; TMO="$2"; shift 2 ;;
    --owned-orgs) [ $# -ge 2 ] || die20 "--owned-orgs needs a value"; OWNED_ORGS="$2"; shift 2 ;;
    --exceptions) [ $# -ge 2 ] || die20 "--exceptions needs a value"; EXC_FILE="$2"; shift 2 ;;
    --quiet) QUIET=1; shift ;;
    --self-test) SELFTEST=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown argument: $1" >&2; usage >&2; exit 20 ;;
  esac
done
# ---------------------------------------------------------------- self-test (the scripts/repo/tests matrix is the full test)
if [ "$SELFTEST" = 1 ]; then
  needle && { echo "SELF-TEST: pass=1 fail=0 (control needle: sees a dirty file, reads a clean tree clean, a corrupt index fails); full matrix: scripts/repo/tests/test_verify_repos.sh"; exit 0; }
  echo "SELF-TEST: the control needle FAILED" >&2; exit 20
fi
die20() { die20_run "$@"; }
command -v jq >/dev/null || die20 "jq required"
command -v python3 >/dev/null || die20 "python3 required (the shared organisation parser)"
command -v sha256sum >/dev/null || die20 "sha256sum required"
[ -r "$ORGPY" ] || die20 "organisation parser missing: $ORGPY"
$GIT -C "$ROOT" rev-parse --git-dir >/dev/null 2>&1 || die20 "not a git repository: $ROOT"
if [ -z "$OWNED_ORGS" ]; then
  OWN_F="$HERE/../audit/own_orgs.txt"
  [ -r "$OWN_F" ] || die20 "no --owned-orgs and no $OWN_F"
  OWNED_ORGS="$(grep -v '^[[:space:]]*#' "$OWN_F" | grep . | paste -sd, -)"
  PEND_F="$HERE/../audit/own_orgs_pending.txt"
  [ -r "$PEND_F" ] && echo "note: own-organisation list BLOCKED-ON ODG-15 for: $(grep -v '^[[:space:]]*#' "$PEND_F" | grep . | paste -sd, -) (counted third-party here; IC-30 'emit both classifications' is an owed contract item, see specs/001-full-project-audit-remediation/audit/owed-contract-items.md)" >&2
fi
OWNED_ORGS="$(printf '%s' "$OWNED_ORGS" | tr 'A-Z' 'a-z' | tr -d ' ')"
needle || die20 "control needle failed: the verifier cannot see a dirty file or a git error (blind), no verdict is possible"

# ---------------------------------------------------------------- enumerate + run
WORK="$(mktemp -d)"; trap 'rm -rf "${WORK:?}"' EXIT
LIST="$WORK/list.tsv"
printf '.\t.\n' > "$LIST"
SUBST="$WORK/submodule_status.txt"
$GIT -C "$ROOT" submodule status --recursive > "$SUBST" 2>"$WORK/submodule_status.err" \
  || die20 "cannot enumerate the submodules: git submodule status --recursive failed: $(head -c 300 "$WORK/submodule_status.err" | tr '\n' ' ')"
while IFS= read -r line; do
  [ -n "$line" ] || continue
  pin="${line:0:1}"; rest="${line:1}"
  if [[ "$rest" =~ ^[0-9a-f]+\ (.*)\ \([^\(\)]*\)$ ]]; then spath="${BASH_REMATCH[1]}"
  elif [[ "$rest" =~ ^[0-9a-f]+\ (.*)$ ]]; then spath="${BASH_REMATCH[1]}"
  else die20 "cannot parse a git submodule status line: $line"; fi
  [ -n "$spath" ] || die20 "empty submodule path in: $line"
  printf '%s\t%s\n' "$spath" "${pin:- }"
done < "$SUBST" >> "$LIST"
export ROOT DO_FETCH NO_REMOTE TMO OWNED_ORGS EXC_FILE GIT SSH_CMD
mkdir -p "$WORK/out"
NLIST="$(wc -l < "$LIST")"
# bash printf, not awk: mawk ends a printf format at the first \0 and so truncates every record (container root cause RC1)
{ n=0; while IFS=$'\t' read -r lp lpin; do n=$((n+1)); [ "$lpin" = " " ] && lpin="="; printf '%06d\0%s\0%s\0' "$n" "$lp" "$lpin"; done < "$LIST"; } | \
  xargs -0 -n 3 -P "$JOBS" bash -c '[ "$3" = "=" ] && p=" " || p="$3"; "'"$SELF"'" --worker "$ROOT" "$DO_FETCH" "$NO_REMOTE" "$TMO" "$OWNED_ORGS" "$EXC_FILE" "$1" "$2" "$p" "'"$WORK/out"'"' _
# rows must equal listed repositories: every listed one has a result and none has an error (a lost repository is never "clean")
nbad=0; nres=0
for i in $(seq 1 "$NLIST"); do
  id="$(printf '%06d' "$i")"
  if [ -f "$WORK/out/$id.err" ]; then nbad=$((nbad+1)); while IFS= read -r m; do echo "verify_repos: $m" >&2; done < "$WORK/out/$id.err"
  elif [ -s "$WORK/out/$id.json" ]; then nres=$((nres+1))
  else nbad=$((nbad+1)); echo "verify_repos: cannot verify $(sed -n "${i}p" "$LIST" | cut -f1): the worker produced no result" >&2; fi
done
[ "$nbad" -eq 0 ] || die20 "$nbad of $NLIST repositories could not be verified; no report written (a repository that cannot be examined is never read as clean)"
[ "$nres" -eq "$NLIST" ] || die20 "internal error: $nres results for $NLIST listed repositories"
jq -s -e --argjson n "$NLIST" 'length==$n' "$WORK"/out/*.json >/dev/null || die20 "internal error: result files do not parse or do not number $NLIST"
STRICT_J=$([ "$STRICT" = 1 ] && echo true || echo false)
jq -s --argjson strict "$STRICT_J" --arg root "$ROOT" \
   --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" --argjson fetch "$([ "$DO_FETCH" = 1 ] && echo true || echo false)" \
   --argjson noremote "$([ "$NO_REMOTE" = 1 ] && echo true || echo false)" '
  (sort_by(.path)) as $repos |
  ($repos | map(
     . as $r |
     ($r.remotes|map(.class)) as $cl |
     ($r.pin == "+" or $r.pin == "U" or $r.pin == "-") as $pindrift |
     {path,pin,head,branch,owned,dirty,dirty_tracked,dirty_untracked,remotes,
      pin_state:(if .pin=="=" or .pin==" " or .pin=="." then "ok" elif .pin=="+" then "drifted" elif .pin=="-" then "uninitialised" elif .pin=="U" then "conflict" else "?" end),
      excepted,exception_reason,
      problems:([
        (if ($r.dirty and ($r.excepted|not)) then "dirty" else empty end),
        (if ($cl|index("REMOTE-BEHIND")) then "ahead" else empty end),
        (if ($cl|index("DIVERGED")) then "diverged" else empty end),
        (if $strict and ($cl|index("LOCAL-BEHIND")) then "behind" else empty end),
        (if $strict and $pindrift then "pin" else empty end)
      ]),
      unproven:(([ $cl[] | select(.=="UNREACHABLE" or .=="UNKNOWN-DIFFERENT" or .=="NO-REMOTE-BRANCH") ] + (if $r.noremote_unproven then ["NO-REMOTE-BRANCH"] else [] end))|unique)
     })) as $rows |
  { tool:"verify_repo.sh", root:$root, generated_utc:$ts, fetch:$fetch, strict:$strict, no_remote:$noremote,
    summary:{ repos:($rows|length), owned:($rows|map(select(.owned))|length),
              dirty:($rows|map(select(.dirty))|length),
              dirty_excepted:($rows|map(select(.excepted))|length),
              ahead:($rows|map(select(.remotes|map(.class)|index("REMOTE-BEHIND")))|length),
              diverged:($rows|map(select(.remotes|map(.class)|index("DIVERGED")))|length),
              pin_drift:($rows|map(select(.pin_state!="ok"))|length),
              unproven:($rows|map(select(.unproven|length>0))|length),
              classes:([$rows[].remotes[].class]|group_by(.)|map({key:.[0],value:length})|from_entries),
              failing:($rows|map(select(.problems|length>0))|length) },
    repos:$rows }' "$WORK"/out/*.json > "$WORK/report.json" || die20 "report assembly failed"
# ---------------------------------------------------------------- exit map counts (docs/16 11.4, data-model section 9)
# read BEFORE anything leaves the process (N6-3): an unreadable count is exit 20 and "writes NO report" (the header), so neither --json FILE, nor the
# --quiet stdout copy, nor the table may exist when it happens
# a count that cannot be read (jq failed, or answered something that is not a non-negative integer) is exit 20 (R2, round 5): an empty
# count would make `[ "" -gt 0 ]` false and the run would fall through to a LOWER code or to 0.
cnt() { local v; v="$(jq "$1" "$WORK/report.json")" || return 1; case "$v" in ''|*[!0-9]*) return 1 ;; esac; printf '%s' "$v"; }
n_dirty="$(cnt '[.repos[]|select(.problems|index("dirty"))]|length')" || die20 "exit map: the dirty count could not be read from the report"
n_div="$(cnt '[.repos[]|select(.problems|index("diverged"))]|length')" || die20 "exit map: the diverged count could not be read from the report"
n_ahead="$(cnt '[.repos[]|select(.problems|index("ahead"))]|length')" || die20 "exit map: the ahead count could not be read from the report"
n_pin="$(cnt '[.repos[]|select(.problems|index("pin"))]|length')" || die20 "exit map: the pin count could not be read from the report"
n_beh="$(cnt '[.repos[]|select(.problems|index("behind"))]|length')" || die20 "exit map: the behind count could not be read from the report"
n_unp="$(cnt '.summary.unproven')" || die20 "exit map: the unproven count could not be read from the report"
[ -n "$JSON_OUT" ] && { cp "$WORK/report.json" "$JSON_OUT" || die20 "cannot write $JSON_OUT"; }
if [ "$QUIET" = 0 ]; then
  jq -r '.repos[] | [ .path, (.branch // "(detached)"), (if .owned then "owned" else "third" end), .pin_state,
        (if .dirty then ("dirty t=" + (.dirty_tracked|tostring) + " u=" + (.dirty_untracked|tostring) + (if .excepted then " [EXC]" else "" end)) else "clean" end),
        ((.remotes|map(.remote + ":" + .class)|join(" "))|if .=="" then "-" else . end) ] | @tsv' \
    "$WORK/report.json" | { printf 'PATH\tBRANCH\tOWN\tPIN\tTREE\tREMOTES\n'; cat; } > "$WORK/table.tsv"
  # aligned by awk (the `column` command is not in every image; container root cause RC5)
  awk -F'\t' 'NR==FNR{for(i=1;i<=NF;i++) if(length($i)>w[i]) w[i]=length($i); next}
               {l=""; for(i=1;i<=NF;i++){c=$i; if(i<NF){for(k=length($i);k<w[i]+2;k++) c=c" "} l=l c} print l}' "$WORK/table.tsv" "$WORK/table.tsv"
  echo; jq -c '.summary' "$WORK/report.json"
fi
if [ -z "$JSON_OUT" ] && [ "$QUIET" = 1 ]; then cat "$WORK/report.json"; fi

# ---------------------------------------------------------------- exit map
[ "$n_dirty" -gt 0 ] && exit 13
[ "$n_div" -gt 0 ] && exit 12
[ "$n_ahead" -gt 0 ] && exit 11
[ "$n_pin" -gt 0 ] && exit 15
[ "$n_beh" -gt 0 ] && exit 12
[ "$n_unp" -gt 0 ] && exit 14
exit 0
