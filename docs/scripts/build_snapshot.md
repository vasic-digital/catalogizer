# snapshot.py, treecache.py, keyring.sh - User Guide (T005b)

**Revision:** 1
**Last modified:** 2026-10-06T20:30:00Z
**Status:** new, uncommitted; independent review (constitution 11.4.142) owed. NOT yet listed in `docs/scripts/README.md` (owed row below).

Companion guide (Helix Constitution section 11.4.18) for `scripts/build/lib/snapshot.py` (driver side), `scripts/build/lib/treecache.py` (build-host side, python3 only, no git) and `scripts/build/lib/keyring.sh`.
Part of T005b (constitution 11.4.173: builds run on a remote host); read `dispatch.md` first.

## The snapshot (`snapshot.py manifest|digest`)
`manifest --root R [--class compile|full] [--exclude P]... [--mode tic|cpa] [--declare P]... [--component C] [--tmp DIR]` prints JSON: one entry per repository (the root `.` and every INITIALISED submodule at every depth, in
path order) with its tree id, HEAD, gitlinks, and the digest = sha256 of the `path TAB tree` lines. `digest` prints only the digest.
- Each tree is written from that repository's OWN temporary index (`GIT_INDEX_FILE` under `--tmp`; the real index and the working tree are only read, never written; the one write into the checkout is that `git hash-object -w` and `git write-tree` add loose objects to each repository's own object database, which `git gc` reclaims). `tic` (default): seeded from a copy of the real index (stat
  cache reused), every changed, deleted or untracked non-ignored path then loaded (`ls-files -m -d -o --exclude-standard`; `git hash-object --no-filters -w` and `git update-index --index-info`, never `git add`; symlinks
  hashed by their target), so no edit is silently left out. `cpa`: seeded from HEAD (`git read-tree HEAD`), exactly the `--declare` paths loaded (undeclared staged content never enters); with `--component` an undeclared
  dirty file inside that component's input closure is refused (exit 20 `undeclared_dirty_input`; interpretation: the closure's INPUT paths, the component and what it references, not whole repositories).
- Every git call runs with the LFS driver disabled (`-c filter.lfs.process= -c filter.lfs.clean=cat -c filter.lfs.smudge=cat -c filter.lfs.required=false`) and `core.attributesFile=/dev/null`, and the objects are hashed
  `--no-filters`: `export-ignore`, `export-subst` and `filter=lfs` are never applied (test p4: a clean-filter shim writes a marker, never written by the snapshot; control needle: the same shim writes it when `git add`
  runs it). `class compile` removes every `--exclude` path (the evidence and audit trees: no build input) from the root tree, so a changed evidence file changes no compile-class digest.
- A submodule whose directory has no `.git` is not initialised: its gitlink stays in the parent tree as the commit id, it has no entry of its own.

## The input closure (`snapshot.py closure --root R --component C [--containerfile F --context D]`)
JSON `{repos, inputs}`: the root and the submodules the component's inputs reach, found transitively by a scan of the component (and of every referenced directory): Go `replace` and `go.work` (`use`, `replace`), npm `file:`/
`link:` dependencies and `workspaces`, Cargo `path` dependencies, Gradle `includeBuild` and `projectDir`, the `COPY` and `ADD` sources of every Containerfile (`--containerfile`, resolved against `--context`, or found in the
component against its own directory) and the `build.context` of every compose service. A referenced directory brings the initialised submodules below it. A reference into a submodule that is not initialised: exit 20
`submodule_uninitialised`. `plan`/`pack`/`tar` with `--component` refuse (20 `closure_incomplete`) a path the component references inside a gitlink the shipped set lacks. Limits (UNCONFIRMED against the real 97-submodule
root): the scan is syntactic (no dynamic references), a Containerfile found without `--context` resolves against its own directory, and a `COPY . .` context ships everything below it by design.

## Shipping (`plan`, `pack`, `tar`) and the build-host cache (`treecache.py`)
`plan` lists `b <id>` / `t <id>` for every object of the shipped trees; `treecache.py missing <cache>` filters the ones the cache lacks; `pack` writes a tar of the raw git objects named on stdin (members `b/<id>`, `t/<id>`);
`treecache.py store <cache>` verifies each object against its id (sha1 of `<type> <len>\0` + bytes) and writes it atomically; `materialize <cache> <manifest> --repos A,B --dest D` writes the trees (submodule trees at their
path), then RECOMPUTES every tree from the files on disk (the gitlink entries taken from the manifest: a gitlink cannot be recomputed from shipped files); `verify <dir> <manifest>` recomputes an existing tree (a cached tree is
re-verified, never trusted). Any mismatch: exit 20 `snapshot_mismatch`, and a materialisation leaves nothing behind. `tar` writes the same trees as a tar by plumbing (`ls-tree` + `cat-file --batch`, never `git archive`).
Deviation, stated: the spec recomputes with `git hash-object --no-filters` and `git mktree` on the host; this does the same computation in python (sha1 of the git object format), so the host needs no git and the check is
independent of the driver's tool. Transfer budget: the object and byte counts are measured and asserted (second submit of an unchanged checkout: 0 objects; one changed source file: its blob and the trees above it);
the TIME budget is not measured.

## The keyring cache (`keyring.sh refill|status`)
The durable source of truth is the mode-0600 secret file outside the checkout; the session keyring entry `catalogizer:build_hmac:<sha256 of the checkout path>` is only a cache, refilled from the file (after a reboot
or logout the keyring is empty). `refill <statedir> <checkout>` verifies the file exactly as a pump does (`event_core.sh derive-key`), loads the entry when absent or different (the secret goes to `keyctl padd` on stdin,
never a command line) and prints `keyring refilled|keyring current`; with the file lost or altered it exits 20 `driver_secret_lost` and REMOVES the entry (a cache never outlives its file). The hub calls it at its start.
Honest limit: no current consumer reads the cache (`derive-key` reads the file); the refill after a reboot is what is exercised and proven (test c39, with a private session keyring per case). Needs `keyctl`
(exit 21 `keyctl_absent` otherwise).

## Tests
`scripts/build/tests/test_snapshot.sh` (p1 to p9, real git fixtures), `test_dispatch_c.sh` c1 to c4 and c39, `test_dispatch_c_e2e.sh` r1 (real container); paired mutations `m_snap_*`, `m_tc_*`, `m_ship_*`, `m_keyring_*`.

## Index row owed (docs/scripts/README.md)
`| [build_snapshot.md](build_snapshot.md) | scripts/build/lib/snapshot.py, treecache.py, keyring.sh | T005b |`
