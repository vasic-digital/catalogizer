# Release checker: ways a cited path can differ between the working tree and a clean clone

Item CAT-REL-CHECK-2 (structural replacement of the point checks of CAT-REL-CHECK-1, review rounds 1-5).
Status: PROVISIONAL (see docs/FOLLOW_UPS_NEXT_PHASE.md). Tool: `scripts/release/check_release_consistency.sh`.

## Class

The checker reads the working tree, a release clone sees only HEAD. A cited repo path (and the changelog,
the version files and the manifest trees) can therefore pass locally and fail in a clone, or the reverse. The
class is "the path as the kernel resolves it here is not the path the clone resolves": any component of the path,
any symlink hop, any directory on the way, or the final entry may be missing from HEAD, have another type, another
link target or other content, or be hidden from `git status`.

## One mechanism (not point checks)

`head_check` / `chain_one` resolve each path **component by component the way the kernel does**, following the
`readlink` chain hop by hop (cap 40 hops) and honouring `.` and `..` after a symlinked directory. For **every
component and every hop** the tree entry of HEAD must exist (`git ls-tree HEAD -- <path>`), must have the same
type as the working tree (file / directory / symlink / submodule), a symlink hop must carry the **same target
text as HEAD's blob**, and no name may collide with a sibling ignoring case. The final file must hash to its HEAD
blob (`git hash-object --no-filters`: raw bytes, no clean filter, no stat cache), the final file or directory must show no `git status`
difference, and no file under it may carry the skip-worktree or assume-unchanged flag (`git ls-files -v`). Anything
that differs or cannot be proven makes the run BLIND (exit 2). The same status and flag checks cover every manifest
tree, plus a raw per-file compare of every tracked file and symlink under it (N1), and a HEAD walk of every manifest entry reached through a symlink. All of it is "HEAD versus the working tree": the index is not what a clone sees.

## Inventory (each member has a RED-first test; mutants are in `mutation_check_release_consistency.sh`)

| # | Member | Behaviour now | Test |
|---|--------|---------------|------|
| 1 | symlink chain of n hops, all committed | PASS | Z1, Z12, Z39, existing I4-1 control |
| 2 | middle hop retargeted in the working tree only | BLIND | Z2 (R5-1a), Z5 |
| 3 | middle hop only staged | BLIND | Z3 (R5-1b) |
| 4 | middle hop untracked | BLIND | Z4 |
| 5 | hop with skip-worktree / assume-unchanged and retargeted | BLIND | Z6, Z7 |
| 6 | symlink loop | MISS (the kernel cannot open it), never PASS; the 40-hop cap is defence in depth (equivalent mutant, below) | Z19 |
| 7 | dangling hop | MISS | Z18 |
| 8 | absolute hop (even into the repo) | BLIND (a clone elsewhere cannot prove it) | Z16 |
| 9 | hop leaving the repository | MISS (existing outside-repo rule); `..` above the root inside the walk is BLIND (unreachable defence, equivalent mutant) | Z17 |
| 10 | hop into a submodule | BLIND | Z22 |
| 11 | path below a gitlink | MISS (git does not track below a gitlink), never PASS | Z23 |
| 12 | clean gitlink cited directly (m-3, fixed: `cat-file -e HEAD:gitlink` was the false BLIND) | PASS | Z20 |
| 13 | submodule checked out at another commit | BLIND | Z21 |
| 14 | final file modified / staged | BLIND | existing R3-1, Z8 |
| 15 | final file modified and assume-unchanged or skip-worktree (m-1, fixed) | BLIND (content hash) | Z8, Z9 |
| 16 | cited directory holding such a file | BLIND (flag check) | Z10 |
| 17 | manifest tree: HEAD mismatch hidden by assume-unchanged (m-1) | BLIND | Z11 |
| 18 | directory symlink mid-path, committed | PASS (previously a false MISS) | Z12, Z40 |
| 19 | directory symlink retargeted / only staged | BLIND | Z13, Z14 |
| 20 | real directory replaced by a symlink | BLIND (type differs) | Z15 |
| 21 | case-insensitive collision of the cited name or of a parent directory | BLIND | Z24, Z25 |
| 22 | gitignored untracked hop | BLIND (not in HEAD) | Z26 |
| 23 | gitignored but force-added file, committed | PASS | Z27 |
| 24 | spaces in file, directory and symlink names (m-2, fixed) | checked like any path when the token is exactly a tracked file or directory | Z28-Z30 |
| 25 | untracked name with a space | stays a skipped token (indistinguishable from a command line) | Z31 |
| 26 | non-ASCII names | PASS committed, BLIND edited | Z32, Z33 |
| 27 | tab, newline, other control characters | BLIND (existing); a newline cannot occur inside a backtick token; paths are never NUL-delimited by the checker | existing M-2, R3-3 |
| 28 | CRLF working copy of an LF blob | BLIND: raw bytes are compared to the blob (`hash-object --no-filters`); also with `core.autocrlf=true` | Z34, Z35 |
| 29 | committed `..`, mid-path `./` and `..` in link targets | PASS, resolved like the kernel | Z36, Z37, Z39, Z40 |
| 30 | uncommitted edit seen through those spellings | BLIND | Z38 |
| 31 | HEAD membership with several specs (RV5-4) | BLIND | Z41 |
| 32 | `--version X` without a `## [X]` section (silently used the topmost) | BLIND (section not found) | Z42-Z44 |
| 33 | cited path resolving to the repository root (F1; was exempt from the walk) | the cited spelling is always walked; a retargeted alias is BLIND | Z50-Z52 |
| 34 | hop target containing/ending in a newline (F2; `$(...)`/`read` lost it) | raw compare with a sentinel; newline or control character in a target is BLIND | Z53-Z55 |
| 35 | status / flag check on a symlink hop entry (F2; only the final entry had it) | every hop gets `entry_clean` | Z56 |
| 36 | local attribute / filter source (clean filter, info/attributes, autocrlf after `git add`) (F3) | raw bytes differ from the blob: BLIND | Z57-Z59 |
| 37 | inherited `GIT_*` environment (F6) | unset at start | Z60 |
| 38 | no `--version`, topmost section differs from versions.json (F6) | section derived from versions.json, else BLIND | Z61, Z62 |
| 39 | manifest tree: a tracked file tampered in HEAD, worktree restored behind a local clean filter (N1a) or a restored mtime with core.trustctime=false (N1b) | raw per-file compare of every tracked file and symlink under the tree: BLIND | Z69, Z70 |
| 40 | committed `eol=crlf` attribute (N4) | accepted limit: BLIND, message says how to resolve; stricter than necessary, never a false pass | Z71 |

Equivalent mutants (not run): the unsupported-HEAD-mode branch in `chain_one` (`git ls-tree` canonicalises a regular-file mode such as 100664 to 100644; measured with a literal tree), removing the 40-hop cap (a real loop is MISS before the walk starts, and a HEAD loop
cannot match a working tree that is not a loop because the link target is compared with HEAD's blob), and the
`..`-above-the-root guard inside the walk (the outside-repo rule rejects such paths first).

## Residual, not handled (UNCONFIRMED where marked)

* Unicode case folding and normalisation (NFC vs NFD) of names; the collision check folds ASCII only.
* A clone made with `core.symlinks=false` (Windows) turns symlinks into text files; sparse or shallow clones; LFS
  pointers (hash of the pointer vs content) - all UNCONFIRMED, untested.
* The executable bit is not compared directly; `git status` reports a mode change on the final entry.
* (N1 closed: every tracked file under a manifest tree is compared raw.) Remaining: a committed `eol=` attribute makes the affected file BLIND (accepted limit, N4).
* Untracked files inside a cited directory outside the evidence prefix; the executable bit when `core.fileMode=false`.
* A name containing a double quote or a backslash is never extracted as a path, so it is never checked.
