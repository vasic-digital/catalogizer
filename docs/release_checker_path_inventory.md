# Release checker: read sources and the ways the working tree can differ from a clean clone

Item CAT-REL-CHECK-3 (successor of CAT-REL-CHECK-2, whose round 5 review was NO-GO; the round-5 snapshot design is kept).
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
   walk, raw hash against the HEAD blob, `git status`, skip-worktree / assume-unchanged flags) and the
   status/flag checks for every manifest tree. Those guards say BLIND when the working tree differs from HEAD; they
   do not decide FAIL/MISS. CAT-REL-CHECK-3 (M1): whether a cited path exists is decided by `SNAP` (`[ -e "$SNAP/$q" ]`),
   not by the working tree. Honest limits of the guards (they are not a proof of equality): `chain_one` returns without
   a guard when the cited path does not exist in the working tree (`[ -e "$REPO/$spec" ] || return 0`), so a HEAD file
   that is missing locally (deleted, sparse-checkout, skip-worktree) gives the verdict of a clean clone (PASS) and not
   BLIND; and a path that exists in HEAD only as a dangling symlink is MISS here as it is in a clone. A difference
   that the guards do see makes the run BLIND (exit 2); it never changes a verdict that came from `SNAP`.
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
| cited path existence, symlink resolution, outside-repo test | `SNAP` | `head_check "cited path"` only (BLIND guard; no verdict from the working tree) |
| manifest discovery (`SHA256SUMS`, `.txt`, `MANIFEST*.sha256`) | `SNAP` glob (C1) | tree status + flags + `raw_tree_check` |
| manifest entries and `sha256sum --strict -c` | `SNAP` (C2) | `head_check` of entries reached through a symlink |
| completeness (UNLISTED) listing | `ls-tree -r HEAD -- dir/`, content in `SNAP` | tree status + flags |
| `.gitignore` decisions | `check-ignore` in `SNAP`, committed files only (C4) | none |
| waiver file | `SNAP` (C3) | `head_check "waiver file"`; a waiver file present only in the working tree is BLIND |
| nested manifests | `ls-tree` + `SNAP` | tree status |
| replace refs (C8) | ignored globally | n/a |

Remaining reads of `$REPO/...` are exactly the guards of column 3 (the needle self-check now tests the fabricated name
in `SNAP`). Remaining limits (UNCONFIRMED where not measured): the snapshot writes every file of HEAD per
run (23k files / 186 MB were counted on the real repository; the run time was not measured); a gitlink is represented by
an empty directory, so evidence below a submodule is MISS/FAIL, which matches a clone without submodules; an empty
symlink target in HEAD cannot be written to the snapshot and gives BLIND; the snapshot writer uses a fixed interpreter
path, but git itself is taken from PATH. Residual (N1, not detected): a file that HEAD stores as a Git LFS pointer is
read as the pointer text, while a clone with git-lfs installed would fetch the real content; a manifest naming such a
file therefore compares the pointer, and the verdict can differ from that clone's. Repositories that use LFS for
evidence must not rely on this checker for those files.

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

## CAT-REL-CHECK-3: snapshot extraction safety (round 5 finding I1) and the class inventory

Finding I1: the round-5 writer created entries in `ls-tree` order and followed earlier symlinks, so a HEAD tree holding
`l` as a symlink to an absolute directory AND `l/` as a subtree with `pwn` wrote `<outside>/pwn` (reproduced on the
base tip: file created with content `PWNED`, verdict BLIND only afterwards). Such a tree is accepted by `git mktree`
(and by `git clone`, fsck is off), so repository content is untrusted input to a file writer. Root cause: the writer
trusted the listing and used path-string operations (`os.makedirs`, `open`, `os.symlink`) that follow links.

Fix at the class level (`scripts/release/check_release_consistency.sh`, snapshot writer):

1. The whole listing is validated BEFORE anything is written. Refused (BLIND, reason printed): a duplicate path;
   a path that is both an entry and a directory prefix of another entry (file/symlink/gitlink versus subtree, in any
   stored order); an empty, `.`, `..` or `.git` (any case) component (a newline inside a name is legal and written as is) (an absolute path has an empty
   first component); an unsupported mode; more than `RELCHK_MAX_FILES` entries (default 1,000,000) or more than
   `RELCHK_MAX_BYTES` (default 4 GiB) of content.
2. Extraction walks directories through file descriptors (`openat` with `O_DIRECTORY|O_NOFOLLOW`, `mkdirat`); regular
   files are created with `O_EXCL|O_NOFOLLOW`; so no write can pass a symlink even if one existed.
3. Symlinks are created last, in a second pass, after every file and directory. By rule 1 no symlink path is a prefix
   of another entry, so no later write could go through one.
4. After extraction the snapshot is walked without following links: exactly the expected entries with the expected
   kinds must exist, no unexpected entry, and every directory must resolve inside the snapshot (`realpath`).
5. A time limit (`RELCHK_SNAP_TIMEOUT`, default 900 s) bounds the writer; every git call is bounded by
   `RELCHK_GIT_TIMEOUT` (default 120 s): a hung git is a git failure, which ends BLIND. A blob is streamed to its file
   in 1 MiB pieces (a single large blob is no longer held whole in memory); a symlink target is read whole but is
   limited to 64 KiB and may not hold a NUL byte; an unexpected exception in the writer is reported as
   `snapshot refused: internal error in the snapshot writer (<type>)`, never a traceback.
6. Every numeric knob (`RELCHK_GIT_TIMEOUT`, `RELCHK_SNAP_TIMEOUT`, `RELCHK_MAX_FILES`, `RELCHK_MAX_BYTES`,
   `RELCHK_MAX_READ`) is validated in the shell: a non-numeric, zero, leading-zero or more-than-15-digit value is a clear
   BLIND (F6a-F6c, R2-5, R2-6); unset or empty selects the default.

Class inventory: every place the checker writes, or follows a path derived from repository content.

| Place | Content-derived? | Covered by |
|-------|------------------|------------|
| snapshot writer: directories, files, symlinks | yes (tree entries) | rules 1-4 above; tests A1-A9, B1-B7 |
| `mktemp -d "$SNAPBASE/snap.XXXXXX"`, `RELCHK_SNAP_DIR` | no (operator/test input) | fixed prefix check; the hook is refused when it exists |
| `git init` inside the snapshot (writes `SNAP/.git`) | no | a HEAD `.git` component is refused (B1-B3), so `.git/info/exclude` cannot be planted (RM1) |
| `trap rm -rf "$SNAP"` | no (rm does not follow links) | n/a |
| `check-ignore` in the snapshot | reads committed `.gitignore` only | `-c core.excludesFile=/dev/null` (RM17) |
| `sha256sum -c` in `SNAP/<dir>` | reads files named by manifest entries | entries are resolved with `readlink -m` and refused when outside the snapshot (F4/F2 tests). The snapshot DOES keep absolute and `..` HEAD symlinks (they are data); every reader below refuses a target outside `SNAP`, so no link is followed out |
| `readlink -f` / `-m` / `cd` on cited paths | reads | resolved result must stay inside `SNAP` (N3, F3 tests) |
| working-tree reads in `head_check` / `chain_one` | reads (BLIND guard only) | never decide FAIL/MISS (M1) |
| python helpers (versions, changelog, completeness): every content read | read `SNAP` | one reader `snap_bytes`/`snap_text` (`RELCHK_PYLIB`): resolves the target, refuses one outside `SNAP`, a non-regular file (opened `O_NONBLOCK`, `fstat`) or more than `RELCHK_MAX_READ` bytes (default 64 MiB); tests F2a-F2n run under `prlimit --as=1GiB` |
| python helpers: git calls | read git | no writes; bounded by `RELCHK_GIT_TIMEOUT` |
| shell `done < "$SNAP/..."` (waiver file, manifests) | read `SNAP` | `snap_readable`: inside `SNAP`, regular, at most `RELCHK_MAX_READ` bytes (F2f-F2j) |
| `sha256sum -c` file contents | reads files named by manifest entries | an entry resolving outside `SNAP` fails first; inside `SNAP` only HEAD blobs exist, total bounded by `RELCHK_MAX_BYTES` |
| `readlink`, `stat`, `[ -e ]`, `[ -f ]` on snapshot paths | metadata only | never read content |
| `raw_tree_check` listing | git, captured to a file inside `SNAP` (removed with it on every return path, R2-3) | exit status checked: a failed or hung `ls-tree -r` is BLIND, not an empty tree (F5a, F5b) |
| working-tree reads in `raw_tree_check` (`hash-object`, `readlink`) | reads the working tree | BLIND guard only, never HEAD content. A type change (regular file to symlink or FIFO) is caught earlier by `git status` (measured: a tracked file replaced by a symlink to `/dev/zero` ends BLIND in 3 s under a 1 GiB cap); the remaining case, a stat-cache trick that hides it from `git status`, is bounded by `RELCHK_GIT_TIMEOUT` and was not exercised (UNCONFIRMED) |

The checker writes nothing else: no file in the repository, no index, no refs. Remaining limits (UNCONFIRMED where not
measured): the TOCTOU between the git object store and a concurrent `gc` is not addressed; `RELCHK_*` environment
variables are trusted operator input; a case-insensitive clone could merge case-variant siblings (not an escape;
cited-path collisions are still reported by `name_collides`).

Tests: `scripts/release/test_check_release_consistency_rc4.sh` (also run by the main suite). Unit-class evidence only.

Mutation evidence (CAT-REL-CHECK-3, unit-class). `mutation_check_release_consistency.sh` carries 30 mutants of the new
code: the original 15, nine added in round 2 (reader target outside the snapshot, reader size cap, waiver-file and manifest
readability checks, `raw_tree_check` ls-tree status, writer excepthook, numeric knob validation, N4 post-extraction check,
N8 child kill) and six added in round 3 (REV-A python reader sibling prefix, REV-B bash `snap_readable` sibling prefix,
REV-C bash `snap_readable` size bound, R2-3 scratch file location, R2-5 `RELCHK_GIT_TIMEOUT` validation, R2-6 knob length
bound). Four older mutants were repaired because their target text moved (I4-1 head check, CC2r5 C6, CC2r5 C1, RM1 text);
`MUT_APPLY_ONLY=1` must print no ERROR line. The runner runs the whole main suite per mutant
(`MUT_FILTER='CAT-REL-CHECK-3|I4-1 head check only the resolved|CC2r5 C6 token path set|CC2r5 C1 version file read|existence check disabled'`);
the result table of the last run belongs to the item's report, not to this document.

Layers that are equivalent mutants, stated honestly: (a) `O_NOFOLLOW` on directory descent and (b) `O_EXCL` versus
`O_TRUNC` on file creation are reachable only by a race that replaces a directory or file during extraction, because the
up-front validation already refuses every duplicate or conflicting path; (c) the writer's `realpath` check of every
extracted directory (reviewer mutant N6) is the same class; (d) the reader's `S_ISREG` / `O_NONBLOCK` check is reachable
only if a FIFO or device appears INSIDE the snapshot, which the writer never creates, because every target outside the
snapshot is refused first (tests F2a-F2c, F2n, F2o, S1 prove the outside check, S2/S3 the sibling-prefix rule). None of these four is in the runner and none is claimed as killed; they are defence in depth.
The post-extraction expected-entries verification (reviewer mutant N4) IS reachable: a tree deeper than `PATH_MAX`
(test D1: 20 levels of 249-byte names) makes it end with `snapshot refused: extracted entry ...`, so it has a test and a mutant.
