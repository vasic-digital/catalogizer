# Release checker: read sources and the ways the working tree can differ from a clean clone

Item CAT-REL-CHECK-2 (structural replacement of the point checks of CAT-REL-CHECK-1), review round 5 version.
Status: PROVISIONAL (see docs/FOLLOW_UPS_NEXT_PHASE.md). Tool: `scripts/release/check_release_consistency.sh`.

## Class

A release clone sees HEAD only. Whatever the checker reads from the working tree, the index or local git
configuration (`.git/info/*`, `core.excludesFile`, filters, replace refs, assume-unchanged / skip-worktree flags)
can make a local run PASS where a plain clone fails. Round 4 showed that closing the members one by one leaves new
ones open (C1-C8 below). The structure is therefore: **content is read from HEAD only; the working tree is
consulted only to prove that it equals HEAD (BLIND guards).**

## Mechanism (round 5)

1. `GIT_NO_REPLACE_OBJECTS=1` is exported for the whole run (also inherited by the python helpers and
   `git show <prevtag>:path`), so replace refs cannot change what HEAD contains (C8).
2. `HEAD^{commit}` is resolved once. A snapshot of exactly that tree (files, symlinks, executable bit; gitlinks as
   empty directories) is written to a private temp directory `SNAP` (`mktemp`, removed on exit) with one
   `ls-tree -r -z` and one `cat-file --batch`. Nothing in the snapshot comes from the working tree or the index.
3. Every content read (version files, changelog, `versions.json`, gradle files, evidence files, manifests, waiver
   file, sha256sum runs, symlink resolution of cited paths and manifest entries) happens inside `SNAP`.
4. Every set of paths (tracked-token set, top-level names, manifest discovery, completeness listing) comes from
   `git ls-tree HEAD`, never from `ls-files` (index) or from a directory listing of the working tree.
5. `.gitignore` decisions use `git check-ignore` inside `SNAP` with `GIT_CONFIG_GLOBAL=/dev/null`,
   `GIT_CONFIG_NOSYSTEM=1` and `core.excludesFile=/dev/null`: only the committed `.gitignore` files count (C4).
6. The working tree is read only by the BLIND guards: `head_check` / `chain_one` (component-by-component, hop-by-hop
   walk, raw hash against the HEAD blob, `git status`, skip-worktree / assume-unchanged flags), `raw_tree_check` and the
   status/flag checks for every manifest tree, and the `[ -e "$REPO/$q" ]` presence check of a cited path.
   A difference makes the run BLIND (exit 2); it never changes a verdict that came from `SNAP`.
7. Semantic consequence: a file or entry that exists only in the working tree or only in the index is invisible. A
   manifest entry or cited path that names such a file is FAIL/MISS (as in a clone), not BLIND.

## Inventory of every read in the script

Census: `grep -n 'git -C\|\$REPO/\|\$SNAP/\|repo,\|snap'` over the script; control needles: the known token `git -C`
was found 19 times, a fabricated token 0 times.

| Read | Source now | Guard against the working tree |
|------|-----------|--------------------------------|
| repo is a git repository, HEAD commit | `rev-parse` | none needed |
| version files, openapi, tauri, cargo, lock files | `SNAP` | `head_check` of every version file that was read |
| gradle file choice (`build.gradle` vs `.kts`, both-exist check) | `SNAP` | `head_check` (C5) |
| `versions.json` (also under `--check changelog`) | `SNAP` | `head_check "versions.json"` when the section is derived (C7) |
| previous tag, `git show <prevtag>:path` | repo object store, no-replace | n/a (tags are not the working tree) |
| `docs/CHANGELOG.md` and the section parse | `SNAP` | `head_check` |
| tracked-token needle, token path set, top-level names, tdirs (C6) | `ls-tree -r HEAD` | none needed |
| `tracked()` | `head_entry` (`ls-tree HEAD -- path`) | none needed |
| cited path existence, symlink resolution, outside-repo test | `SNAP` | `[ -e "$REPO/$q" ]` + `head_check "cited path"` |
| manifest discovery (`SHA256SUMS`, `.txt`, `MANIFEST*.sha256`) | `SNAP` glob (C1) | tree status + flags + `raw_tree_check` |
| manifest entries and `sha256sum --strict -c` | `SNAP` (C2) | `head_check` of entries reached through a symlink |
| completeness (UNLISTED) listing | `ls-tree -r HEAD -- dir/`, content in `SNAP` | tree status + flags |
| `.gitignore` decisions | `check-ignore` in `SNAP`, committed files only (C4) | none |
| waiver file | `SNAP` (C3) | `head_check "waiver file"`; a waiver file present only in the working tree is BLIND |
| nested manifests | `ls-tree` + `SNAP` | tree status |
| replace refs (C8) | ignored globally | n/a |

Remaining reads of `$REPO/...` are exactly the guards of column 3 plus `[ -e "$REPO/$neg" ]` for a fabricated name in
the needle self-check. Remaining limits (UNCONFIRMED where not measured): the snapshot writes every file of HEAD per
run (23k files / 186 MB were counted on the real repository; the run time was not measured); a gitlink is represented by
an empty directory, so evidence below a submodule is MISS/FAIL, which matches a clone without submodules; an empty
symlink target in HEAD cannot be written to the snapshot and gives BLIND; the snapshot writer uses a fixed interpreter
path, but git itself is taken from PATH.

## Class C1-C8 (round 4 NO-GO) and their tests

All are driven through the real script on a fixture and compared with a plain `git clone` of the same repository
(`cc` helper in the test file: a local PASS where the clone is not a PASS fails the test). RED-run against the round 4
checker (`4ced5ef1`): Z87-Z96 fail there (Z97 guards the round 2 OWN6 member, which still holds).

| # | Member | Now | Test |
|---|--------|-----|------|
| C1 | manifest deleted from index/worktree, HEAD has a wrong checksum | BLIND (tree differs from HEAD), never PASS | Z86 |
| C2 | manifest entry names an untracked working-tree file | SHAFAIL (as a clone) | Z87 |
| C3 | waiver file edited behind assume-unchanged / skip-worktree / local clean filter | HEAD's waiver file rules (FAIL as a clone) | Z88, Z89, Z90 |
| C4 | `.git/info/exclude` / `core.excludesFile` names the missing entry | SHAFAIL, not UNVERIFIABLE | Z91, Z92 |
| C5 | build.gradle and build.gradle.kts both in HEAD, one deleted locally | FAIL (ambiguous) as a clone | Z93 |
| C6 | index-only top-level directory turns a token into a path | same verdict as the clone | Z94, Z94b |
| C7 | `versions.json` edited locally under `--check changelog` | HEAD's section, BLIND for the edit | Z95, Z95b |
| C8 | replace ref hides a tampered HEAD | never PASS (BLIND) | Z96 |
| OWN6 | `git hash-object` fails in the per-path chain walk | BLIND, never a pass | Z97 |

## Earlier members (rounds 1-4) after round 5

The earlier members (symlink chains, retargeted or staged hops, flags, loops, dangling and absolute hops, submodules,
case collisions, CRLF, spaces and non-ASCII names, `..` spellings, several version specs) keep their tests Z1-Z85.
Where a member used to be BLIND only because the file was staged or untracked, it is now MISS or FAIL (the snapshot
has no such file, as a clone has none); the tests were updated accordingly (Z14, Z16, Z17, Z26, Z41, Z65, Z73, Z81
and others). Cases that stay BLIND are those where HEAD holds the entry but the working tree differs from it.

Equivalent mutants (documented): (2) the completeness listing (RC-3b) testing the files in the working tree instead of the
snapshot: it runs after `raw_tree_check`, which already returned BLIND for any file under the manifest tree that differs
from HEAD (the mutant was run once and survived, as predicted by the code order, line 944 before line 1012). (1) Reading a manifest's lines from the working tree instead of the snapshot. A
working-tree manifest that differs from HEAD makes `raw_tree_check` return BLIND before any line is parsed, so the
switch cannot change a verdict (measured: both variants gave `VERDICT: BLIND` for an appended escape line).

## Honest boundary

This list is the set of read sources found by the census above, each closed by the single mechanism; it is not a proof
that no other divergence exists. The mutation runner covers the mechanism (reads switched back to the working tree,
the index, `info/exclude`, replace refs; dropped guards).
