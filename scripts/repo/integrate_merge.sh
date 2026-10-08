#!/bin/bash -p
# T040 helper: integrate_merge.sh - the merge path of CPA stage S1 (plan owner's rule (Y), docs/16 section 12.2): `git merge --no-ff`
# of the live remote tips that fast-forward cannot take. Never a rebase, a reset, a push or a force.
#
# Usage   integrate_merge.sh --root <repo> --branch <b> --run-dir <dir> [--audit-dir <dir>] [--repo-key <key>] [--ev <rel dir>]
#                            [--adoption-commit <sha>] [--anchor <sha>] [--path-gates <tsv>] [--approved <manifest.json>]
#                            [--tables <p1,p2,...>] [--run-pid <pid>] [--resolution <dir>] [--timeout <s>]
#   --run-dir      CPA_RUN of the run (must exist): backup/, conflicts/, merge.json, commits.tsv and the deferral flags live there
#   --audit-dir    holds `<run id>/commits.tsv` of every run (default: the parent of --run-dir); the CPA-commit predicate reads it
#   --repo-key     repository key of the commits.tsv row and of the backup file name (default `.`; `/` becomes `__` in the file name)
#   --ev           evidence directory relative to the root (default specs/001-full-project-audit-remediation/evidence); the merge-review
#                  file named by Awaits-Review is `<ev>/reviews/CPA-merge-<run id>.json` (written by ST-REV, never by this helper)
#   --adoption-commit  CENTRAL C9 (c): a tip of which HEAD is an ancestor but whose first-parent chain lacks this commit is a merge target
#   --anchor       adoption anchor of the repository: only non-CPA incoming commits above it are named `Foreign-Commit:` (none without it,
#                  as the S1 foreign listing of T040 reads the anchor; adoption.json itself is T047's)
#   --path-gates   rows `path<TAB>class`; an incoming change to a G-GATE path whose content sha256 is not its entry in --approved
#                  ({"<path>":"<sha256>"} or {"paths":{...}}) makes the merge held; --tables: admission tables (default the three of T040b)
#   --run-pid      process id recorded in merge.json with its start time from /proc/<pid>/stat (default the parent process)
#   --resolution   the `--resolve-merge` form (see below)
# Targets live tips of the remotes (ls-remote, objects fetched with the T032 form) that are not ancestors of HEAD, plus a tip that
#         descends from HEAD but whose first-parent chain lacks the adoption commit. One target when one descends from every other, else
#         each in turn in the order of the remote names; `git merge-tree --write-tree` over every pair first: two targets that conflict with
#         each other give 12 `remotes_diverged` naming the pair, nothing moved.
# Refuses 12 `ff_blocked_by_local_changes` when the index differs from HEAD or the incoming commits change a path with an uncommitted change;
#         20 `merge_in_progress` when MERGE_HEAD exists; 20 when the backup (`git bundle create ... B ^<tip>` + `git bundle verify`, and a copy of
#         every uncommitted file with its sha256 under backup/worktree/) cannot be written (an empty range has no bundle: backup/<key>.bundle.empty).
# Merge   writes merge.json (repository, tip, local tip, run id, pid, pid start time); `git merge --no-ff --no-commit <tip>`; no conflict: the
#         commit `Merge <remote>/<branch> <tip> (CPA)` with trailers CPA-Run, Deferred-Gates, Foreign-Commit, and `Awaits-Review: <review file>` when
#         the local range holds an Awaits-Review commit missing from some remote, the merge brings a G-GATE change outside the approved manifest or a
#         change to an admission table by a non-CPA commit, or the target lacks the adoption commit; the row (key, sha, run id) is appended to
#         commits.tsv right after the commit. A conflict: each conflicting file with its markers is copied to <run-dir>/conflicts/<path>,
#         `git merge --abort`, HEAD and the uncommitted tracked diff are checked unchanged, exit 12 `merge_conflict` naming the paths.
# --resolution <dir>  dir must be <root>/.audit/merge-resolution/<run id> (else 20 `resolution_dir_invalid`) with resolution.json
#         {"tip","paths":[...],"model","effort","files":{"<path>":"<sha256>"},"method":{"<path>":"re-recorded"}} (field names UNCONFIRMED) and one
#         resolved file per path: 12 `merge_target_moved` when the tip is no longer the newest live tip, 12 `merge_conflict` when the conflict set
#         differs, 20 `merge_resolver_not_pinned` unless model is Opus and effort xhigh (section 11.4.211), 20 `resolution_invalid` for a
#         hash mismatch, 10 when a resolved file holds a conflict marker, 20 `store_not_rerecorded` for a chained or binary store path that the
#         record does not name `re-recorded`, whose resolved file does not extend the remote side byte for byte, or whose verify tool
#         (tools/evidence/verify, scripts/register/gate.sh) is absent (fail closed). The S2 secret fold (T040a) is NOT built: a resolution that
#         passed every check above exits 20 `secret_fold_unavailable` and nothing is written, so this slice never completes a resolved merge.
# Exits   0 merged or nothing to do; 10 conflict marker in a resolved file; 11 `remote_unreachable`: the repository has remotes and NONE could be read (ls-remote or fetch
#         failed for all of them; WF8: it was nothing_to_merge, exit 0); 12 blocked (reasons above); 20 refusal (force_refused, usage, unsafe
#         value, not_a_repository, wrong_branch, merge_in_progress, backup_failed, ...). 13 (S2 secret refusal) is not produced in this slice.
# Never   a rebase, a reset (but `merge --abort`, which restores the pre-merge state), a push, a force; every value that reaches git is
#         validated and follows `--` or is a validated hex object name.
set -u
D="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$D/lib_safe.sh"
die() { printf 'REASON\t%s\n' "$1"; echo "integrate_merge: $1: $2" >&2; exit "${3:-20}"; }
for a in "$@"; do case "$a" in --force*|-f|+*|--rebase|--reset|--hard|--merge) die force_refused "$(printf '%q' "$a")" ;; esac; done
ROOT=""; BR=""; RUND=""; AUD=""; KEY="."; EV="specs/001-full-project-audit-remediation/evidence"; ADOPT=""; ANCHOR=""; GATES=""; APPR=""
TABLES="scripts/repo/check_classes.tsv,scripts/repo/check_exemptions.tsv,scripts/repo/fixture_roots.txt"; RPID="$PPID"; RES=""; TMO=60
while [ $# -gt 0 ]; do
  [ $# -ge 2 ] || die usage "$1 needs a value"
  case "$1" in
    --root) ROOT="$2" ;; --branch) BR="$2" ;; --run-dir) RUND="$2" ;; --audit-dir) AUD="$2" ;; --repo-key) KEY="$2" ;; --ev) EV="${2%/}" ;;
    --adoption-commit) ADOPT="$2" ;; --anchor) ANCHOR="$2" ;; --path-gates) GATES="$2" ;; --approved) APPR="$2" ;; --tables) TABLES="$2" ;;
    --run-pid) RPID="$2" ;; --resolution) RES="$2" ;; --timeout) TMO="$2" ;;
    *) die usage "unknown argument $(printf '%q' "$1")" ;;
  esac; shift 2
done
safe_dir_arg "$ROOT" || die unsafe_root "$(printf '%q' "$ROOT")"
safe_branch "$BR" || die unsafe_branch "$(printf '%q' "$BR")"
safe_dir_arg "$RUND" || die unsafe_run_dir "$(printf '%q' "$RUND")"; [ -d "$RUND" ] || die run_dir_absent "$(printf '%q' "$RUND")"
RUND="$(cd "$RUND" && pwd)"; RID="$(basename "$RUND")"; safe_runid "$RID" || die unsafe_run_id "$(printf '%q' "$RID")"
[ -n "$AUD" ] || AUD="$(dirname "$RUND")"; safe_dir_arg "$AUD" || die unsafe_audit_dir "$(printf '%q' "$AUD")"
safe_line "$KEY" && [ -n "$KEY" ] || die unsafe_repo_key "$(printf '%q' "$KEY")"
safe_relpath "$EV" || die unsafe_ev "$(printf '%q' "$EV")"
[ -z "$ADOPT" ] || safe_sha "$ADOPT" || die unsafe_adoption_commit "$(printf '%q' "$ADOPT")"
[ -z "$ANCHOR" ] || safe_sha "$ANCHOR" || die unsafe_anchor "$(printf '%q' "$ANCHOR")"
[[ "$RPID" =~ ^[0-9]+$ ]] || die unsafe_run_pid "$(printf '%q' "$RPID")"; [[ "$TMO" =~ ^[0-9]+$ ]] && [ "$TMO" -gt 0 ] || die usage "--timeout must be a positive integer"
[ -z "$GATES" ] || [ -r "$GATES" ] || die path_gates_unreadable "$(printf '%q' "$GATES") (an explicitly named path-gates file that is missing is a refusal, never an empty table)"
IFS=, read -r -a TBL <<< "$TABLES"; for t in "${TBL[@]}"; do safe_relpath "$t" || die unsafe_table "$(printf '%q' "$t")"; done
[ -d "$ROOT" ] || die not_a_repository "$(printf '%q' "$ROOT")"
ROOT="$(cd "$ROOT" && git rev-parse --show-toplevel 2>/dev/null)" || die not_a_repository root
g() { git -C "$ROOT" "$@"; }
cur="$(g symbolic-ref -q HEAD)" || die wrong_branch "detached HEAD"; cur="${cur#refs/heads/}"; [ "$cur" = "$BR" ] || die wrong_branch "on $(printf '%q' "$cur"), not $(printf '%q' "$BR")"
[ ! -e "$(g rev-parse --path-format=absolute --git-path MERGE_HEAD)" ] || die merge_in_progress "MERGE_HEAD exists; an interrupted merge belongs to an earlier run"
export GIT_ALLOW_PROTOCOL=file:ssh:git:https GIT_TERMINAL_PROMPT=0
W="$(mktemp -d "${TMPDIR:-/tmp}/integrate_merge.XXXXXX")" || die internal mktemp; trap 'rm -rf "$W"' EXIT; trap 'exit 143' TERM; trap 'exit 130' INT
tm() { timeout "$TMO" "$@"; }
LOCAL="$(g rev-parse HEAD)"
# ---- live tips -------------------------------------------------------------------------------------------------------------------
REMS=(); TIPOF=(); ALLTIPS=()
# the remote list is read with its status (a failing `git remote` inside `g remote | sort` was "no remotes" and the merge path reported nothing_to_merge, exit 0; WF7 M-3)
rl="$(g remote 2>/dev/null)" || die git_listing_failed "remote list"
rl="$(printf '%s\n' "$rl" | sort)"
while IFS= read -r r; do
  [ -n "$r" ] || continue
  safe_remote "$r" || die unsafe_remote_name "$(printf '%q' "$r")"
  while IFS= read -r u; do safe_url "$u" || die unsafe_remote_url "$r $(printf '%q' "$u")"; done < <(g config --get-all "remote.$r.url" 2>/dev/null; g config --get-all "remote.$r.pushurl" 2>/dev/null)
  REMS+=("$r")
done <<<"$rl"
NFAILR=0
for r in "${REMS[@]+"${REMS[@]}"}"; do
  if ! lr="$(tm git -C "$ROOT" ls-remote -- "$r" "refs/heads/$BR" 2>&1)"; then echo "fetch_failed:$r" >&2; NFAILR=$((NFAILR+1)); TIPOF+=(""); continue; fi
  t="$(printf '%s\n' "$lr" | lr_exact "$BR")"
  if [ -z "$t" ] || ! safe_sha "$t"; then TIPOF+=(""); continue; fi
  if ! tm git -C "$ROOT" fetch -q --no-tags --no-recurse-submodules --no-write-fetch-head --refmap= -- "$r" "refs/heads/$BR" >/dev/null 2>&1; then echo "fetch_failed:$r" >&2; NFAILR=$((NFAILR+1)); TIPOF+=(""); continue; fi
  TIPOF+=("$t"); ALLTIPS+=("$t")
done
# every remote could not be read: nothing is known about what any of them holds, so "nothing to merge" would be a guess that reads clean (WF7 INFO-1, WF8): 11, as
# integrate_ff_only.sh does. One reachable remote that answered is enough to proceed (its answer is what is merged); the failed ones stay on stderr.
if [ "${#REMS[@]}" -gt 0 ] && [ "$NFAILR" -ge "${#REMS[@]}" ]; then echo "integrate_merge: remote_unreachable: none of the ${#REMS[@]} remote(s) could be read; nothing was merged and nothing can be said about them" ; exit 11; fi
# ---- targets -----------------------------------------------------------------------------------------------------------------------
adopt_ok() { # adopt_ok <tip>: first-parent chain of the tip holds the adoption commit (true when none is required)
  [ -n "$ADOPT" ] || return 0
  local a; a="$(g rev-parse -q --verify "$ADOPT^{commit}" 2>/dev/null)" || return 1   # an unresolvable adoption commit fails closed (WF2 review minor 3)
  g rev-list --first-parent "$1" 2>/dev/null | grep -qx "$a"
}
CAND=(); CANDR=()
for i in "${!REMS[@]}"; do
  t="${TIPOF[$i]-}"; [ -n "$t" ] || continue
  dup=0; for c in "${CAND[@]+"${CAND[@]}"}"; do [ "$c" = "$t" ] && dup=1; done; [ "$dup" = 0 ] || continue
  if [ "$t" = "$LOCAL" ] || g merge-base --is-ancestor "$t" "$LOCAL" 2>/dev/null; then continue; fi
  if g merge-base --is-ancestor "$LOCAL" "$t" 2>/dev/null; then adopt_ok "$t" && continue; fi
  CAND+=("$t"); CANDR+=("${REMS[$i]}")
done
if [ "${#CAND[@]}" -eq 0 ]; then echo "integrate_merge: nothing_to_merge"; exit 0; fi
TARGETS=(); TARGR=(); newest=""
for i in "${!CAND[@]}"; do
  ok=1; for j in "${!CAND[@]}"; do [ "$i" = "$j" ] || g merge-base --is-ancestor "${CAND[$j]}" "${CAND[$i]}" 2>/dev/null || ok=0; done
  [ "$ok" = 1 ] && { newest="$i"; break; }
done
if [ -n "$newest" ]; then TARGETS=("${CAND[$newest]}"); TARGR=("${CANDR[$newest]}"); else TARGETS=("${CAND[@]}"); TARGR=("${CANDR[@]}"); fi
for i in "${!TARGETS[@]}"; do for j in "${!TARGETS[@]}"; do
  [ "$i" -lt "$j" ] || continue
  if ! g merge-tree --write-tree "${TARGETS[$i]}" "${TARGETS[$j]}" >/dev/null 2>&1; then
    [ "$(g merge-tree --write-tree "${TARGETS[$i]}" "${TARGETS[$j]}" >/dev/null 2>&1; echo $?)" = 1 ] || die merge_tree_failed "pair ${TARGR[$i]} ${TARGR[$j]}"
    echo "remotes_diverged ${TARGR[$i]} ${TARGETS[$i]} ${TARGR[$j]} ${TARGETS[$j]}"; exit 12
  fi
done; done
# ---- the working tree must allow the merge --------------------------------------------------------------------------------------------
g diff --cached --quiet 2>/dev/null || { echo "ff_blocked_by_local_changes: the index differs from HEAD"; exit 12; }
# each git read goes to a file first so its status is read: a pipe would lose it and a failing git would read as "nothing dirty" / "no overlap" (WF6 W6-12)
dirty="$W/dirty"; g status --porcelain=v1 -z --ignore-submodules=all >"$W/status.z" 2>/dev/null || die git_status_failed "the working tree status cannot be read"
tr '\0' '\n' < "$W/status.z" | sed -n 's/^.. //p' | sort -u > "$dirty"
for t in "${TARGETS[@]}"; do
  mb="$(g merge-base "$LOCAL" "$t" 2>/dev/null)" || mb="$LOCAL"
  g diff --name-only "$mb" "$t" >"$W/diff.names" 2>/dev/null || die git_diff_failed "the changed paths of ${t:0:12} cannot be listed"
  hit="$(sort -u "$W/diff.names" | comm -12 - "$dirty")"
  [ -z "$hit" ] || { echo "ff_blocked_by_local_changes: $(printf '%s' "$hit" | tr '\n' ' ')"; exit 12; }
done
# ---- resolution record (validated before anything is moved) -------------------------------------------------------------------------------
if [ -n "$RES" ]; then
  safe_dir_arg "$RES" || die resolution_dir_invalid "$(printf '%q' "$RES")"
  rd="$(cd "$RES" 2>/dev/null && pwd -P)" || die resolution_dir_invalid "$(printf '%q' "$RES")"
  case "$rd" in "$(cd "$ROOT" && pwd -P)/.audit/merge-resolution/"*) rid2="${rd##*/}"; safe_runid "$rid2" && [ "$rd" = "$(cd "$ROOT" && pwd -P)/.audit/merge-resolution/$rid2" ] || die resolution_dir_invalid "$rd" ;; *) die resolution_dir_invalid "$rd" ;; esac
  [ -f "$rd/resolution.json" ] || die resolution_dir_invalid "resolution.json is missing"
  rtip="$(jq -r '.tip // empty' "$rd/resolution.json" 2>/dev/null)" || die resolution_dir_invalid "resolution.json is not JSON"
  safe_sha "${rtip:-x}" || die resolution_dir_invalid "tip"
  newest_tip="${TARGETS[$((${#TARGETS[@]}-1))]}"
  [ "$rtip" = "$newest_tip" ] || { echo "merge_target_moved: recorded $rtip, newest live tip $newest_tip"; exit 12; }
  model="$(jq -r '.model // empty' "$rd/resolution.json")"; effort="$(jq -r '.effort // empty' "$rd/resolution.json")"
  case "$(printf '%s' "$model" | tr '[:upper:]' '[:lower:]')" in opus|claude-opus*) ;; *) die merge_resolver_not_pinned "model $(printf '%q' "$model")" ;; esac
  [ "$effort" = xhigh ] || die merge_resolver_not_pinned "effort $(printf '%q' "$effort")"
  # the conflict set of the merge now
  g merge --no-ff --no-commit "$rtip" >/dev/null 2>&1; mrc=$?
  cset="$(g diff --name-only --diff-filter=U 2>/dev/null | sort -u | tr '\n' ' ')"
  [ ! -e "$(g rev-parse --path-format=absolute --git-path MERGE_HEAD)" ] || g merge --abort >/dev/null 2>&1
  mapfile -t WANTA < <(jq -r '.paths[]?' "$rd/resolution.json" | sort -u)
  want="$(printf '%s ' "${WANTA[@]+"${WANTA[@]}"}")"
  [ "$mrc" != 0 ] && [ "$cset" = "$want" ] || { echo "merge_conflict: conflict set now '${cset% }', recorded '${want% }'"; exit 12; }
  marker=0
  for p in "${WANTA[@]}"; do
    safe_relpath "$p" || die resolution_dir_invalid "path $(printf '%q' "$p")"
    [ -f "$rd/$p" ] || die resolution_dir_invalid "resolved file for $(printf '%q' "$p") is missing"
    h="$(sha256sum -- "$rd/$p" | cut -d' ' -f1)"; [ "$h" = "$(jq -r --arg p "$p" '.files[$p] // empty' "$rd/resolution.json")" ] || die resolution_invalid "sha256 of $(printf '%q' "$p") differs from the record"
    grep -qE '^(<<<<<<< |=======$|>>>>>>> )' "$rd/$p" && { echo "conflict marker in resolved file $p"; marker=1; }
  done
  [ "$marker" = 0 ] || exit 10
  for p in "${WANTA[@]}"; do
    case "$p" in "$EV/ledger.jsonl"|"$EV/anchors.jsonl"|"$EV/deferrals.jsonl"|"$EV/flake_ledger.jsonl"|docs/workable_items.db|docs/register/*) store=1 ;; *) store=0 ;; esac
    [ "$store" = 1 ] || continue
    [ "$(jq -r --arg p "$p" '.method[$p] // empty' "$rd/resolution.json")" = re-recorded ] || die store_not_rerecorded "$p: the record does not name method re-recorded"
    case "$p" in *.jsonl) g cat-file blob "$rtip:$p" > "$W/side.blob" 2>/dev/null; sz="$(stat -c %s "$W/side.blob")"; cmp -s -n "$sz" "$W/side.blob" "$rd/$p" && [ "$(stat -c %s "$rd/$p")" -ge "$sz" ] || die store_not_rerecorded "$p does not extend the remote side byte for byte" ;; esac
    case "$p" in */ledger.jsonl|*/anchors.jsonl) [ -x "$ROOT/tools/evidence/verify" ] || die store_not_rerecorded "$p: tools/evidence/verify is absent (fail closed)" ;;
      docs/workable_items.db|docs/register/*) [ -x "$ROOT/scripts/register/gate.sh" ] || die store_not_rerecorded "$p: scripts/register/gate.sh is absent (fail closed)" ;; esac
  done
  die secret_fold_unavailable "the S2 secret fold (T040a) is not built; no resolved file is written"
fi
# ---- backup (section 9.2) and the merge record ---------------------------------------------------------------------------------------------
kf="${KEY//\//__}"; [ "$kf" = . ] && kf=main
BK="$RUND/backup"
mkdir -p "$BK/worktree" 2>/dev/null || die backup_failed "cannot create $(printf '%q' "$BK")"
first="${TARGETS[0]}"
if [ -n "$(g rev-list -n1 "refs/heads/$BR" "^$first" 2>/dev/null)" ]; then
  g bundle create "$BK/$kf.bundle" "refs/heads/$BR" "^$first" >/dev/null 2>&1 || die backup_failed "git bundle create"
  g bundle verify "$BK/$kf.bundle" >/dev/null 2>&1 || die backup_failed "git bundle verify"
else : > "$BK/$kf.bundle.empty" || die backup_failed "empty marker"; fi
: > "$BK/worktree.sha256" || die backup_failed "worktree.sha256"; : > "$BK/nested.tsv" || die backup_failed "nested.tsv"
# the status listing is written to a file and its status read (a failing listing is no empty backup); a rename or copy record is followed by a bare record holding the OLD path; an
# untracked nested repository is recorded, not copied (cp -p cannot copy a directory); a symlink (a dangling one included) is copied as a link (WF17-cpa LIST-1, -9, -10, -11)
g status --porcelain=v1 -z -uall --ignore-submodules=all > "$W/st.z" 2>/dev/null || die git_listing_failed "git status for the 9.2 backup"
while IFS= read -r -d '' e; do
  xy="${e:0:2}"; f="${e:3}"
  case "$xy" in R*|C*|?R|?C) IFS= read -r -d '' _old ;; esac
  safe_relpath "${f%/}" || die backup_failed "unsafe uncommitted path $(printf '%q' "$f")"
  if [ -d "$ROOT/$f" ] && [ ! -L "$ROOT/$f" ]; then
    [ -e "$ROOT/${f%/}/.git" ] || die backup_failed "a directory entry that is no repository: $(printf '%q' "$f")"
    printf 'nested_repo\t%s\t%s\t%s\n' "${f%/}" "$(git -C "$ROOT/${f%/}" rev-parse HEAD 2>/dev/null || echo none)" "$(git -C "$ROOT/${f%/}" status --porcelain=v1 -z 2>/dev/null | sha256sum | cut -d' ' -f1)" >> "$BK/nested.tsv"; continue
  fi
  if [ -L "$ROOT/$f" ]; then
    mkdir -p "$BK/worktree/$(dirname "$f")" && cp -P -p -- "$ROOT/$f" "$BK/worktree/$f" || die backup_failed "link copy of $(printf '%q' "$f")"
    printf 'symlink\t%s\t%s\n' "$f" "$(readlink -- "$ROOT/$f")" >> "$BK/nested.tsv"; continue
  fi
  [ -f "$ROOT/$f" ] || continue
  mkdir -p "$BK/worktree/$(dirname "$f")" && cp -p -- "$ROOT/$f" "$BK/worktree/$f" || die backup_failed "copy of $(printf '%q' "$f")"
  ( cd "$BK/worktree" && sha256sum -- "$f" ) >> "$BK/worktree.sha256" || die backup_failed "sha256 of $(printf '%q' "$f")"
done < "$W/st.z"
pst=""; [ -r "/proc/$RPID/stat" ] && pst="$(awk '{print $22}' "/proc/$RPID/stat" 2>/dev/null)"
# ---- merge each target in turn ---------------------------------------------------------------------------------------------------------------
is_cpa() { # is_cpa <sha>: the one shared predicate (lib_safe.sh cpa_runid + cpa_row; WF3 review m3, X3)
  local id; id="$(cpa_runid "$ROOT" "$1")" || return 1
  cpa_row "$AUD/$id/commits.tsv" "$1"
}
MAN='{}'; [ -n "$APPR" ] && [ -f "$APPR" ] && MAN="$(jq -c '(.paths // .)' "$APPR" 2>/dev/null || echo '{}')"
[ -z "$GATES" ] && [ -f "$ROOT/scripts/repo/path_gates.tsv" ] && GATES="$ROOT/scripts/repo/path_gates.tsv"
for i in "${!TARGETS[@]}"; do
  t="${TARGETS[$i]}"; r="${TARGR[$i]}"; LOCAL="$(g rev-parse HEAD)"
  if [ "$t" = "$LOCAL" ] || g merge-base --is-ancestor "$t" "$LOCAL" 2>/dev/null; then
    g merge-base --is-ancestor "$LOCAL" "$t" 2>/dev/null || { echo "integrate_merge: $r already merged"; continue; }
  fi
  jq -n --arg repo "$KEY" --arg tip "$t" --arg local "$LOCAL" --arg run "$RID" --argjson pid "$RPID" --arg pst "$pst" \
    '{repository:$repo,tip:$tip,local_tip:$local,run_id:$run,pid:$pid,pid_start:$pst}' > "$RUND/merge.json.tmp" && mv -f "$RUND/merge.json.tmp" "$RUND/merge.json" || die backup_failed "merge.json"
  PREDIFF="$(g diff HEAD --binary 2>/dev/null | sha256sum)"
  g merge --no-ff --no-commit "$t" >"$W/merge.out" 2>&1; mrc=$?
  mh="$(g rev-parse --path-format=absolute --git-path MERGE_HEAD)"
  if [ "$mrc" != 0 ]; then
    unm="$(g diff --name-only --diff-filter=U -z 2>/dev/null | tr '\0' '\n')"
    if [ -n "$unm" ]; then
      while IFS= read -r f; do
        safe_relpath "$f" || { g merge --abort >/dev/null 2>&1; die conflict_path_unsafe "$(printf '%q' "$f")"; }
        mkdir -p "$RUND/conflicts/$(dirname "$f")" && cp -p -- "$ROOT/$f" "$RUND/conflicts/$f" 2>/dev/null || true
      done <<< "$unm"
      g merge --abort >/dev/null 2>&1
      [ "$(g rev-parse HEAD)" = "$LOCAL" ] && [ "$(g diff HEAD --binary 2>/dev/null | sha256sum)" = "$PREDIFF" ] || die internal "HEAD or the tracked files changed by the aborted merge"
      echo "merge_conflict: $(printf '%s' "$unm" | tr '\n' ' ')"; exit 12
    fi
    [ ! -e "$mh" ] || g merge --abort >/dev/null 2>&1
    echo "ff_blocked_by_local_changes: git refused the merge: $(head -c 200 "$W/merge.out" | tr '\n' ' ')"; exit 12
  fi
  # held?
  held=""
  while IFS= read -r c; do
    [ -n "$c" ] || continue; vp="$(g log -1 --format=%B "$c" | awk 'index($0,"Awaits-Review: ")==1 {v=substr($0,16)} END{print v}')"; [ -z "$vp" ] || held="${held:+$held,}local_held_commit"
  done < <(for tt in "${ALLTIPS[@]+"${ALLTIPS[@]}"}"; do g rev-list HEAD "^$tt" 2>/dev/null; done | sort -u)
  adopt_ok "$t" || held="${held:+$held,}adoption"
  if [ -n "$GATES" ] && [ -f "$GATES" ]; then
    while IFS= read -r gp; do
      safe_relpath "$gp" || continue
      for c in $(g rev-list "HEAD..$t" 2>/dev/null); do
        ln="$(g diff-tree -r -m --no-commit-id --name-status "$c" -- "$gp" 2>/dev/null | head -1)"; [ -n "$ln" ] || continue
        want="$(printf '%s' "$MAN" | jq -r --arg p "$gp" '.[$p] // empty')"
        if [ "${ln%%	*}" = D ]; then [ -n "$want" ] && held="${held:+$held,}gate"; else got="$(g show "$c:$gp" 2>/dev/null | sha256sum | cut -d' ' -f1)"; [ "$got" = "$want" ] || held="${held:+$held,}gate"; fi
      done
    done < <(awk -F'\t' '$2=="G-GATE"{print $1}' "$GATES")
  fi
  for tb in "${TBL[@]}"; do
    for c in $(g rev-list "HEAD..$t" -- "$tb" 2>/dev/null); do is_cpa "$c" || held="${held:+$held,}admission_table"; done
  done
  mf="$W/msg"
  { printf 'Merge %s/%s %s (CPA)\n\nCPA-Run: %s\n' "$r" "$BR" "$t" "$RID"
    df="$("$D/record_deferral.sh" --run-dir "$RUND" --list 2>/dev/null)" || df=""; [ -z "$df" ] || printf 'Deferred-Gates: %s\n' "$df"
    if [ -n "$ANCHOR" ] && g cat-file -e "$ANCHOR^{commit}" 2>/dev/null; then
      for c in $(g rev-list --reverse "HEAD..$t" "^$ANCHOR" 2>/dev/null); do
        is_cpa "$c" && continue
        g log --format='%(trailers:key=Foreign-Commit,valueonly,separator=%x0A)' HEAD 2>/dev/null | grep -qxF -- "$c" && continue
        printf 'Foreign-Commit: %s\n' "$c"
      done
    fi
    [ -z "$held" ] || printf 'Awaits-Review: %s/reviews/CPA-merge-%s.json\n' "$EV" "$RID"
  } > "$mf"
  # the merge commit and its commits.tsv row are ONE window: a signal that arrives in it is held (a flag) and acted on after the row is written, never between the two (RES-3)
  GOT=""; trap 'GOT=143' TERM; trap 'GOT=130' INT; trap 'GOT=129' HUP
  g commit -q -F "$mf" >/dev/null 2>"$W/commit.err" || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; g merge --abort >/dev/null 2>&1; die commit_failed "$(head -c 300 "$W/commit.err")"; }
  sha="$(g rev-parse HEAD)"
  printf '%s\t%s\t%s\n' "$KEY" "$sha" "$RID" >> "$RUND/commits.tsv" || { trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP; die commits_tsv_write_failed "commits.tsv"; }
  trap 'exit 143' TERM; trap 'exit 130' INT; trap - HUP
  [ -z "$GOT" ] || exit "$GOT"
  printf 'MERGED\t%s\t%s\t%s\t%s\n' "$r" "$t" "$sha" "${held:-unheld}"
done
exit 0
