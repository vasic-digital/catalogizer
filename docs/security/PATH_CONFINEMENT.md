# Filesystem path confinement (catalog-api)

Request-fed filesystem paths (sync endpoints, conversion jobs, cover-art lookup, asset cache,
backup restore) are confined to configured root directories by `catalog-api/internal/pathguard`
(symlink-resolving; a rejection is returned for an empty root list, a relative path, a NUL byte, a symlink
loop or any resolution error; a dangling symlink is followed to where a write would land). Since review
round 6 every root ENTRY is validated too: an empty, relative (including `.`) or NUL-carrying entry fails
the whole call with `ErrInvalidRoot`. Before round 6 an empty entry resolved to `/`, so a root list such
as `["/srv/data", ""]` silently allowed every path; a relative entry was resolved against nothing
meaningful. An unparsable root list is therefore now a configuration error, not a wider allowed set.
Resolution walks the path exactly as the request gives it (it is not `filepath.Clean`ed first), one
component at a time the way the kernel does: every symlink is followed (dangling ones included), a
relative link target is resolved against the link's real directory, a `..` steps up from the directory
reached so far (so `root/lnk/../x` with `lnk -> /outside/sub` resolves to `/outside/x`, not `root/x`),
and a `..` that follows a not-yet-existing element is refused.

Before review round 3 the path was `filepath.Clean`ed before resolution, which folds `lnk/..` without
looking at `lnk`: `root/lnk/../pwn.txt` was accepted as `root/pwn.txt` while the kernel wrote
`/outside/pwn.txt` (N1).

Callers keep the raw string after the check and derive more paths from it with `filepath.Join`,
`filepath.Dir` and `filepath.Walk`, which clean lexically. Review round 4 (H1) showed that a path the
kernel resolves inside the root could still lead those functions outside it (with `lnk -> d1/d2`,
`root/lnk/../../out` is `root/out` to the kernel but `<parent>/out` to `Join`; a local mirror wrote there,
a mirror clean-up deleted through it, and the upload walkers read through it). Since round 4 `Confine`
also refuses:

* a path whose lexical reading (`filepath.Clean`) and kernel reading name different files, i.e. any `..`
  that follows a symlink (`path has a ".." after a symlink`);
* a path whose lexical prefixes do not enter the root through the root itself, or leave it again once
  inside. This refuses, for example, `/elsewhere/l` with `l -> root/sub/f`, and `root/a/b` with
  `root/a -> /outside/x` and `/outside/x/b -> root/b`. Use the canonical path below the root instead.

What that rule guarantees is narrower than the round-4 text of this file said ("`Dir(path)` and every
directory derived by trimming stay inside the root"; review round 5, RV5-B, showed that was false). Every
prefix from the root down to the path stays inside, but `Confine` also accepts the root itself, and for a
path equal to the root `Dir(path)` is the root's PARENT. Since round 5 there are two entry points:

* `Confine` / `ConfineData` for DIRECTORY arguments (a sync root, a walk start, `Confine(Dir(hint))`):
  the root itself is valid.
* `ConfineFile` / `ConfineDataFile` for a path written or read as a FILE, or next to which a file is
  written (download target, copy source and destination, conversion source and target, every file an
  upload walker reads): the path must resolve STRICTLY below the root it matched, otherwise
  `ErrRootItself` (`path is the allowed root itself, not a file below it`). Only for such a path does
  `Dir(path)` resolve inside the root (to the root at the shallowest). Directories derived by trimming
  further than `Dir` are outside the guarantee, as they were before.

A path equal to a nested root but strictly below an outer root is accepted through the outer root. Forms
refused as "the root itself": `root`, `root/`, `root/.`, `root/sub/..`, a link inside the root pointing at
`.`, and a WebDAV listing entry whose name is the configured remote path itself (it mapped the download
target to the local root). Before round 5 a conversion with `target_path` equal to a conversion root was
accepted, and LibreOffice and the multi-page image writer then wrote into the root's parent; the routed
`POST /copy/local` with `destination_path` `.` did the same with the temp directory.

What the differential test (`internal/pathguard/confine_differential_test.go`) covers: random trees of
directories, files and symlinks (relative, absolute, dangling, loops, chains, `..`-bearing targets) inside
and around one root, random raw paths starting at the root (`..`, `.`, `//`, trailing `/`). For every
accepted path it performs one of five operations through the raw string and counts an escape when the
kernel created something outside the real root, or the walk visited a directory outside it: create the
file, `MkdirAll` it, create `Join(raw, "child")`, create a file in `Dir(raw)`, `Walk(raw)`. Measured on
this commit: seeds default/1/2/3 at 4,000 cases each and seeds 4/5 at 20,000 each gave 0 escapes; an
earlier draft of the round-4 fix had 1 escape on the default seed (the `/elsewhere/l` case above), which
is how that rule was found. Since round 5 a second test
(`internal/pathguard/confine_extended_differential_test.go`) runs seven operations through the raw
string: create the file, `MkdirAll` it, create `Join(raw, "child")`, `MkdirAll(Join(raw, "d", "e"))`,
`MkdirAll(Dir(raw))` plus a sibling file in it, `Walk(raw)`, and rename a file from `Dir(raw)` onto raw.
The file-class operations (create, sibling in `Dir`, rename onto) are judged against `ConfineFile`, the
others against `Confine`, with no exemption for the root itself. Measured on the round-5 commit, 1,500
cases per seed: default 0 escapes (743 accepted as file, 36 refused as the root itself), seed 1 0 (717,
50), seed 2 0 (708, 56), seed 3 0 (705, 44); the round-4 test at 4,000 cases per seed also gave 0 escapes
for the same four seeds. A random test is not a proof: it says nothing about trees it did not
generate, about more than one root, or about a symlink swapped between the check and the use (F4,
below). The price is refusals on the safe side: about 5% of random cases the kernel could have handled
inside the root are refused, nearly all a `..` after a missing element or after a symlink.

Not covered by path checks at all: a hard link to a protected file (handled separately, see below) and
any race between check and use.

## Environment variables

Each is a colon-separated list of absolute directories. When set and non-empty it REPLACES the default of that class.
The effective roots are logged once at boot (`Filesystem path confinement roots: ...`).

| Variable | Class | Default | Used for |
|---|---|---|---|
| `CATALOGIZER_READ_ROOTS` | read | `MEDIA_ROOT_PATH` (if set) + `./data` | sync upload source, local-mirror source, conversion input, cover-art lookup (`LocalScanResolver`) |
| `CATALOGIZER_SYNC_ROOTS` | write | `./data` | download target, `destination_directory`, both sides of a bidirectional sync |
| `CATALOGIZER_CONVERSION_ROOTS` | write | `./data` | conversion output (`target_path`) |

Everything under a write root is also readable. `local_path` of a sync endpoint is READ for
`sync_direction: upload`, WRITTEN for `download` / `bidirectional`. `source_directory` / `destination_directory`
in `sync_settings` are checked for every endpoint type; `sync_mode: bidirectional` treats the source side as written.
Storage roots registered in the database are NOT added automatically; list their directories in `CATALOGIZER_READ_ROOTS`
(UNCONFIRMED whether a dynamic provider is wanted - see "Follow-up: dynamic storage-root provider" below).

A local endpoint (`type: local`) with no `source_directory` uses `local_path` as its source, but `local_path` is
classed by `sync_direction` (written for `download` / `bidirectional`). So a local mirror from a read root such as
`/media/movies` with `sync_direction: download` is REFUSED. This is deliberate: the class of `local_path` stays a
function of one field for every endpoint type, and the error is on the safe side (refusing a read, never allowing a
write). To mirror from `/media`, set `source_directory: /media/movies` (checked as a read path unless
`sync_mode: bidirectional`) or use `sync_direction: upload`.

In `docker-compose.yml` the host directory `MEDIA_ROOT_PATH` from `.env` (`.env.example`: `./media`) is mounted
`:ro` at `/media`, and the api container gets `MEDIA_ROOT_PATH=/media` as a fixed value (the container-side path,
not the host one). So `/media/...` is a read root out of the box: push-syncing `/media/movies` and converting
`/media/video.mp4` to a path under `/app/data` work; converting *to* `/media/...` or downloading *into* it is
refused. Before review round 2 the host value (`./media`) was passed through, so the read root became
`/app/media` and the library at `/media` was NOT readable; `scripts/tests/test_compose_media_root.sh` renders the
compose file with `.env.example` through `podman compose config` and fails if this regresses.

The default write root `./data` is `/app/data` in the container (WORKDIR `/app`). It is the named volume `api_data`,
so synced and converted files survive `podman compose down` / up. Without that volume (compose files before review
round 2, or a hand-written `podman run`) they are lost when the container is recreated; set
`CATALOGIZER_SYNC_ROOTS` / `CATALOGIZER_CONVERSION_ROOTS` to a mounted directory in that case.

## Protected application files

The default write root `./data` also holds the default SQLite database `./data/catalogizer.db`
(`database.path`), and `./data` is a read root as well. So a request-named path (sync `local_path`,
`source_directory` / `destination_directory`, every file a WebDAV download writes, conversion source and
target) is additionally refused (`path overlaps a protected application file`) when, after symlink
resolution, it

* is the database or one of its SQLite companion files (`-wal`, `-shm`, `-journal`) or the database key
  file (`database.encryption_key_file` / `DATABASE_ENCRYPTION_KEY_FILE` / the per-user default), or
* lies below one of them, or
* is a directory that contains one of them (a sync of `./data` would upload the database, overwrite it, or
  delete it in mirror mode).

A hard link to a protected file is the same file under another name; no path comparison sees it, so
`ConfineData` also compares file identity (`os.SameFile`) with every protected file that exists. When the
configured database path is itself a symlink, the companions next to the link AND next to its target are
protected, because which name SQLite gives them is not assumed. Both are computed when the path is
registered (at boot): a link changed later, or a hard link made to a companion file that did not exist
at check time, is not seen.

Per file, the sync walkers (local mirror / incremental / bidirectional, S3, GCS and WebDAV upload) read
only regular files: a symlink, device, socket or pipe met while walking a source is skipped and logged,
and every file read or written is re-checked with `ConfineDataFile` against its own root, so a link
planted in a destination does not redirect a copy, and a hard link of the database planted in a source is
not uploaded. The WebDAV upload walker (`scanLocalFiles`) was missed in round 4 and still uploaded
symlinked and hard-linked files until round 5 (RV5-A). A LibreOffice HTML conversion now renames the file LibreOffice wrote
in its output directory; before round 4 it renamed `<source>.html` next to the input, i.e. it could move a
file out of the read root.

A sync ROOT that is itself a symlink to a directory (local mirror / incremental / bidirectional source and
destination, WebDAV / S3 / GCS upload source) is resolved before it is walked (review round 7, S1).
`filepath.Walk` `Lstat`s its root, reports a symlinked root as the link and never descends, so until round 7
such a sync "succeeded" with 0 files. Entries met INSIDE the walk are still handled as above (links skipped).
Since review round 8 the resolution IS the confinement check: `performLocalSync` and the upload source call
`pathguard.ConfineData` on each root at run time and walk the resolved path it returns (source: read roots,
or the sync write roots for a bidirectional sync; destination: sync write roots, checked before and again
after `MkdirAll`; upload source: read roots). Round 7 resolved the root a second time with `EvalSymlinks`
after `confineSyncPaths` had checked it and did not check the result, so a root swapped for a link to an
outside directory in between was followed (outside files copied in, outside files deleted by mirror
cleanup). What the walk does NOT follow after the run-time check is a swap of the root's LAST component:
`filepath.Walk` does not descend into a symlinked root. An ANCESTOR of the resolved root (a directory
inside the configured roots) swapped for a link after the check IS followed, because the walk re-resolves
every path through the swapped ancestor (review round 9, F1: mirror cleanup deleted `outside/a/dst/precious.txt`,
`copyFile` wrote into `outside/a/dst`). The per-file checks compared against the walk-root string, which
`ConfineDataFile` re-resolves the same way, so they passed. Since review round 9 every per-file sink also
confines against the CONFIGURED roots, which the swap does not move:
- `uploadableFile` and the `copyFile` source: `ConfineDataFile(path, read roots + sync write roots)`;
- `copyFile` destination and its parent directory, the mirror `MkdirAll(destPath)` guard, and the WebDAV
  download path: the sync write roots;
- `cleanupDestination`: the parent of each entry is confined against the walk root AND the sync write roots
  before EACH `os.Remove` (`confineRemoval`); a refusal stops the cleanup with an error. The destination
  root itself (`relPath == "."`) is never removed (behaviour change: before round 9 an empty destination
  root whose source was gone was removed).

What the per-file re-checks protect against: an ancestor swap whose target is OUTSIDE the configured roots,
done before the per-file check runs. What they do NOT protect against: a swap whose target stays INSIDE
the configured roots (the configured-roots check passes, so files can still be copied into or removed from
another directory inside the roots), and a swap made after a per-file check and before the matching
`open` / `remove` (the F4 TOCTOU below). The race is narrowed, not closed; the planned fix is `os.Root`
(F4). A symlinked NAS mount used as the configured root itself keeps working in every mode (regression
tests `TestRV8_SymlinkedRootLayoutsKeepWorking`, `TestRV9_ConfiguredRootsCheckAdmitsLegitimateLayouts`).

External converters (ffmpeg, ebook-convert, ImageMagick `convert`, pandoc, LibreOffice) open their output by
path and follow a symlink there; ImageMagick also writes derived names (`out-0.bmp`, `out-1.bmp` for
multi-frame input) and ffmpeg / ImageMagick expand `%d` patterns. Since review round 7 (S2) every such tool
writes into a fresh private 0700 directory created next to the target (`.catalogizer-conv-*`, inside the
conversion write roots); afterwards every entry it produced must be a regular file, every destination
`Dir(target)/<name>` must pass `ConfineDataFile` and must not already be a link, FIFO or directory, and only
then are the files renamed into place (`rename(2)` never follows a link at the destination). All entries are
checked before the first is moved, a tool failure publishes nothing, and the private directory is removed.
What the private converter directory changes (review round 8, accepted behaviour):
- The CHECKS are all or nothing: every output is checked, and every kept-permission `chmod` is applied
  inside the private directory, before the first rename (for example `out-1.bmp` refused means `out-0.bmp`
  is not published either). The RENAMES are not atomic as a group: if the rename of output k fails,
  outputs 1..k-1 are already published and stay; the error names the output and how many were published
  ("... failed after N of M outputs were published"). Not rolled back (a rollback rename can fail the same
  way). The private directory is created 0700.
- An existing target is REPLACED, not written in place: `rename(2)` puts a new inode at the name. Its
  permission bits are copied from the replaced regular file (a 0664 target stays 0664 even when the tool
  writes under `umask 077`); owner, group, ACLs and extended attributes are NOT copied, and other hard
  links to the old target keep the old content (before round 7 a tool writing in place updated every hard
  link). A new target gets the mode the tool created it with.
- The output directory itself must be writable: `MkdirTemp` creates the private directory there. A
  read-only output directory holding a writable, pre-created target used to work (`ffmpeg -y` wrote into
  the existing file) and now fails with a permission error. Accepted: writing in place is exactly the
  write-through-a-planted-link the private directory exists to stop.
- A crash while a tool runs leaves `.catalogizer-conv-*` behind (see `docs/FOLLOW_UPS_NEXT_PHASE.md`).

Proven with fake tools (unit class) and with the real ImageMagick 7.1.2 on the host: real `convert two.gif
out.bmp` with `out-0.bmp` planted as a link to an outside file overwrites that file on the round-6 code and is
refused on the round-7 code. What this does NOT close: a directory component of the target (`Dir(target)`)
swapped for a symlink between the check and `MkdirTemp` / `rename` (the F4 TOCTOU below), and files a tool
writes OUTSIDE its output directory by its own logic (none of the five is known to; not verified for every
option).

Not refused: a write through a HARD LINK to an arbitrary file outside the roots. A hard link inside a write
root is a regular file there, so every check above passes and the write lands in the shared inode. Only
links to the protected database / key files are caught (`os.SameFile`, above). Exposure is limited by the
kernel setting `fs.protected_hardlinks=1` (default on most Linux distributions), which stops an unprivileged
user from hard-linking a file it does not own or cannot read and write; a file the service user itself owns
can still be linked. Not fixed; tracked in `docs/FOLLOW_UPS_NEXT_PHASE.md`.

`./data/catalogizer.db` is protected even if the server never registers it; at boot the configured
database path and key file are added (if the key file location cannot be determined, a warning is
logged and only the database files are protected), and the boot log line lists them as `protected=`. Use a
sub-directory such as `./data/sync` or `./data/converted` as a sync or conversion target. Cover-art
lookup (`LocalScanResolver`) uses the plain read-root check and is not affected.

Upgrade impact: a sync endpoint whose `local_path` (or a `sync_settings` directory) is `./data` itself, or
any directory containing the database, is now refused when it runs; point it at a sub-directory.

## Sink inventory (review round 5)

Every place in `catalog-api` (non-test code) that walks, reads or writes a local path a request or a
stored endpoint can name, found with a grep over `filepath.Walk`, `os.Open*`, `os.Create`,
`os.WriteFile`, `os.Remove*`, `os.Rename` and `MkdirAll`. A "no production caller" verdict was checked
with the same grep finding the 3 real callers of `NewSyncService` (positive control).

| Sink | Path source | Status |
|---|---|---|
| `uploadToWebDAV` -> `scanLocalFiles` walk + read | sync `local_path` | CLOSED round 5: `uploadableFile` per file (regular, `ConfineDataFile`); round 9 also confines against the configured read roots (ancestor swap, F4 residual) |
| `downloadFromWebDAV` write | sync `local_path` + remote entry name | CLOSED: `ConfineDataFile`; entry equal to the remote path refused (round 5); round 9 also confines against the sync write roots (ancestor swap, F4 residual) |
| S3 / GCS upload walkers | sync `local_path` | CLOSED round 4 (`uploadableFile`); not driven end to end without a storage double (V15, see follow-ups) |
| local mirror / incremental / bidirectional `copyFile` | `local_path`, `sync_settings` dirs | CLOSED: both sides `ConfineDataFile` (round 5; round 4 used `ConfineData`); round 9 also confines against the configured roots (ancestor swap, F4 residual) |
| `cleanupDestination` remove | destination root | NARROWED (round 9): lexical `Walk`, `os.Remove` removes a link itself, and since round 9 each entry's parent is confined against the walk root and the sync write roots before each remove. An ancestor swapped to OUTSIDE the configured roots is refused; a swap that stays inside them, or one made between the check and the remove, is not (F4) |
| conversion readers / writers | `source_path`, `target_path` | CLOSED: both `ConfineDataFile` (round 5) |
| routed `POST /copy/local` (`internal/handlers/copy.go`) | `destination_path` | CLOSED round 5: `ContainedJoin` + `ConfineDataFile` under the temp dir |
| `asset_resolvers` cover lookup | media-item hint | SCOPED OUT: `Confine(Dir(hint))` directory argument, fixed cover file names, `Lstat` regular-file check; no hard-link check (read only) |
| filesystem `local_client` / `nfs_client` | admin-configured storage base | FOLLOW-UP: `resolvePath` is lexical (`ConfinePath`); symlinks inside the base are followed (operator decision needed) |
| SMB `DownloadFile` | temp file, routed copy handler | Callers pass a temp path or the confined copy destination |
| `WebDAVClient.SyncDirectory` / `UploadBatch` / `DownloadBatch`, `calculateChecksum`, `ConfigurationWizardService`, `NewEnhancedChangeWatcher`, `MediaPlayerHandlers`, `handlers/copy.go` (unrouted `CopyHandler`) | various | UNREACHABLE: no production caller (kept per §11.4.124, not deleted) |
| config, firebase, encryption, provisioner, admin backup dir, `configPath`, `LogDirectory`, `statsPath`, cover-art `cacheDir`, temp files | server configuration | NOT request-supplied |

## Sink inventory (review round 6, class-exhaustive)

Grep over non-test `.go` files under `catalog-api` (`grep -rnE --include=*.go`, `_test.go` excluded):

```
os\.(Create|CreateTemp|OpenFile|MkdirAll|Mkdir|MkdirTemp|Rename|WriteFile|Remove|RemoveAll|Symlink|Link|Chmod|Chtimes|Truncate)\(|\.File\(|\.FileAttachment\(|SaveUploadedFile\(
```

* Result: 129 call sites at commit `e5b65d1d` (the round-6 commit; measured with `git grep` on that
  commit, 2026-10-29). The round-6 report said 130: that count was wrong (corrected in round 7). The
  round-7 working tree has 132 (the private converter directory adds `MkdirTemp`, `RemoveAll` and `Rename`).
  The round-9 working tree (on `b031ef41`) has 133 (round 8 added the private-directory `Chmod`); the line
  numbers below were re-measured on that tree (evidence `sinks_rv9_counts.txt`).
* Positive control: `services/sync_service.go` returns 7 hits (round 9: lines 396, 700, 758, 954, 966,
  1010, 1016), including `os.MkdirAll(destPath` (1 hit, the mirror directory sink); an instrument that
  misses it is blind.
* Negative control: `os\.CreateZZQ\(` returns 0 hits.
* Gin read sinks (`c.File`) are part of the pattern; `FileAttachment` and `SaveUploadedFile` returned 0.

Rows are grouped by class; every hit belongs to exactly one row.

| Sink (file:line at grep time) | Path source | Status |
|---|---|---|
| `handlers/conversion_handler.go:317` `c.File(job.TargetPath)` | stored conversion job | FIXED round 6: `ConfineDataFile` against the conversion roots (403) + `Lstat` regular-file check (404); a symlink or a root itself is never served |
| `handlers/cover_handler.go:121` `c.File(*coverArt.LocalPath)` | `cover_art.local_path` row | RESIDUAL (DB trust): written only by the cover-art service (`cacheDir` + generated id); `ScanLocalCoverArt` paths come only from the unrouted `MediaPlayerHandlers`. A tampered row would be served; follow-up below |
| `services/sync_service.go:758` mirror `MkdirAll(destPath)` | `local_path` + walked entry | FIXED round 6: `ConfineData(destPath, destDir)` before `MkdirAll`; round 9 adds the sync write roots |
| `services/sync_service.go:954` `copyFile` `MkdirAll(Dir(dst))` | destination root | FIXED round 6: `ConfineData(Dir(dst), dstRoot)` before `MkdirAll` (round 5 already refused it through `prefixesStayWithin`; the test is kept as a regression guard) |
| `services/sync_service.go:966` `copyFile` `OpenFile(dst)` | destination root | CONFINED (`ConfineDataFile` against the walk root and, since round 9, the sync write roots); leaf-symlink and ancestor-swap TOCTOU residual, see F4 |
| `services/sync_service.go:396` WebDAV download `MkdirAll` | `local_path` | CONFINED round 5; round 9 adds the sync write roots |
| `services/sync_service.go:700` `MkdirAll(dst)` | `performLocalSync` destination root | CONFINED (`ConfineData` against the sync write roots, checked before and after `MkdirAll`, round 8) |
| `services/sync_service.go:1010,1016` `Remove` | `cleanupDestination` | lexical walk, removes a link itself; since round 9 `confineRemoval` checks the parent against the walk root and the sync write roots before each remove, and the root itself is never removed; residual F4 |
| `services/webdav_client.go:137,149` `DownloadFile` | confined `localPath` from `downloadFromWebDAV`; `DownloadBatch`/`SyncDirectory` unreachable | FIXED round 6: `pathsafe.CreateNoFollow` (a symlink planted at the leaf after confinement is refused) |
| `services/conversion_service.go` per-page writes, `.tmp.txt`, final HTML write | conversion target | FIXED round 6: `createConversionOutput` = `ConfineDataFile` against `ConversionWriteRoots()` + `pathsafe.CreateNoFollow`; HTML fallback checks the target leaf before any tool runs |
| `services/conversion_service.go:786` LibreOffice `Rename` | tool output next to target | FIXED round 6: `moveLibreOfficeOutput` (`Lstat` regular file, both sides `ConfineDataFile`); since round 7 both names are inside the private converter directory |
| external tools (ffmpeg, ebook-convert, ImageMagick, pandoc, LibreOffice) writing target / derived names | conversion target | FIXED round 7 (S2): private directory + publish of regular files only. Round 6 checked only the HTML path, so the derived writes were NOT closed in round 6, contrary to the round-6 commit wording |
| `services/conversion_service.go:830,837,879,885` private converter directory (`MkdirTemp`, `RemoveAll`, kept-mode `Chmod`, publish `Rename`) | conversion target directory | rounds 7-9: created 0700 inside the conversion write roots; see the private-directory notes above |
| `services/conversion_service.go:432` `Remove(tempTextFile)` | derived temp file | removes the link itself, never its target |
| `internal/handlers/copy.go:165` | routed `POST /copy/local` | CONFINED round 5 |
| `internal/handlers/copy.go:253,259`, `internal/handlers/download.go` (8), `handlers/comic_pages_handler.go`, `handlers/pdf_pages_handler.go`, `internal/services/smb.go:317,321`, `internal/services/cover_art_service.go:1357,1364,1413,1275` | `os.CreateTemp` names | NOT request-supplied (temp files) |
| `internal/services/smb.go:213` `DownloadFile` `MkdirAll` | callers pass a temp file or the confined copy destination | CONFINED by caller |
| `internal/services/cover_art_service.go:758,763,855,1151,1155` | `cacheDir` + generated ids | NOT request-supplied |
| `internal/services/cover_art_service.go:441` `ProcessImage` | caller-supplied | UNREACHABLE: no production caller |
| `filesystem/local_client.go` (7), `filesystem/nfs_client.go` (8), `filesystem/nfs_client_darwin.go` (5) | admin-configured storage base | FOLLOW-UP (unchanged): lexical `ConfinePath` |
| `handlers/copy.go` (4) | unrouted `CopyHandler` | UNREACHABLE |
| `services/configuration_wizard_service.go` (11) | wizard | UNREACHABLE |
| `database/encryption.go` (33) | backup/restore/rekey paths | admin/operator-supplied (CLI and admin backup dir), NOT request-supplied |
| `main.go` (5), `config/config.go` (2), `services/configuration_service.go` (2), `handlers/admin_handler.go:409`, `internal/media/manager.go:583`, `services/log_management_service.go:695,710` | server configuration / log directory walk | NOT request-supplied |
| `internal/pathsafe/pathsafe.go:132` | the no-follow helper itself | helper |
| `tests/automation/*.go`, `tests/test_utils.go`, `tests/test_main.go` | test harness (non-`_test` files) | NOT shipped behaviour |

## Sink inventory (review round 7: process and read sinks)

The round-6 pattern covered writes only. Two more classes, measured on commit `e5b65d1d` with `git grep -nE`
over non-test `.go` files under `catalog-api` (counts and needles: scratchpad evidence `sinks_rv7_counts.txt`):

* `exec\.Command(Context)?\(`: 16 call sites (10 in the round-7 working tree; the 7 conversion calls became
  one `runConverterTool`). Positive control: `services/conversion_service.go` gives 7, including
  `exec.Command("pandoc"`; negative control `exec\.CommandZZQ\(` gives 0.
* Read sinks `os\.(Open|ReadFile|ReadDir|Lstat|Stat)\(|filepath\.(Walk|WalkDir)\(|ioutil\.(ReadFile|ReadDir)\(`:
  146 call sites (147 on the round-9 tree; `services/sync_service.go` has 16 of them). Positive control:
  `services/sync_service.go` gives 6 `filepath.Walk(`; negative control `os\.OpenZZQ\(` gives 0.

| Sink | Path source | Status |
|---|---|---|
| `services/conversion_service.go` 7 `exec.Command` (ffmpeg x2, ebook-convert, pandoc, libreoffice, convert x2) | conversion source / target | FIXED round 7 (S2), see above; the source is confined by `confineConversionPaths` |
| `internal/services/cover_art_service.go:965,996,1389` (ffprobe, ffmpeg) | media item path / `cacheDir` output | read of a catalogued media path; output in `cacheDir` + generated id, NOT request-supplied |
| `filesystem/nfs_client_darwin.go` (5: mount, umount x2, cp, mv) | admin-configured storage base | FOLLOW-UP (unchanged, same class as the write rows) |
| `internal/infra/provisioner.go:168` `getent hosts` | configured host name | NOT a path |
| sync walkers and readers (`services/sync_service.go`, 16 on the round-9 tree) | `local_path`, `sync_settings` dirs | CONFINED per file (rounds 4-6); symlinked ROOTS resolved since round 7 (S1) |
| `services/conversion_service.go` reads (`Open(SourcePath)`, `ReadFile(tempTextFile)`, `Lstat` x2) | conversion source / derived temp | CONFINED (`confineConversionPaths`, `createConversionOutput`) |
| `handlers/conversion_handler.go:327` `Lstat(TargetPath)` | stored job | part of the round-6 download fix |
| `internal/services/asset_resolvers.go` (6) | media-item hint | SCOPED OUT (unchanged, see round 5) |
| `internal/handlers/copy.go:158`, `internal/services/smb.go:285` | confined copy destination / temp file | CONFINED by caller |
| `services/webdav_client.go` (3), `handlers/copy.go:475`, `internal/media/realtime/enhanced_watcher.go` (3) | `DownloadFile` confined local path; `SyncDirectory`, unrouted `CopyHandler`, `NewEnhancedChangeWatcher` | CONFINED by caller / UNREACHABLE (no production caller) |
| `handlers/admin_handler.go` (4) | admin backup dir, database path | admin-supplied, NOT request-supplied |
| remaining (database/encryption, config, main, challenges, cover-art cache, media recognition, log management, filesystem clients, pathguard / pathsafe, tests harness) | server configuration, operator CLI, admin-configured storage base, test harness | NOT request-supplied, or the filesystem-client FOLLOW-UP |

## WebDAV listing limits

`WebDAVClient.ListFiles` (used by WebDAV sync) bounds a listing by depth (32), PROPFIND requests (10000) and
entries (200000). Exceeding any of them, or failing to list any sub-collection, fails the whole listing with an
error (`webdav listing exceeded its limit: ...`); a partial listing is never returned as complete. Review round 1
bounded only the depth and swallowed sub-listing errors: a server reporting two sub-collections per collection
could still force about 2^33 requests, so the round-1 commit message's "bounded" was not true for breadth.

## Upgrade impact

* Sync endpoints and pending conversion jobs stored before this change are re-checked when they RUN
  (`performSync`, `StartConversion`). Any whose paths fall outside the roots above is refused: the sync session
  fails with `sync refused: ... rejected: path is outside the allowed roots`, the conversion job is marked `failed`.
* Fix: add the directory to the matching variable (`CATALOGIZER_SYNC_ROOTS` for written paths,
  `CATALOGIZER_READ_ROOTS` for read-only paths, `CATALOGIZER_CONVERSION_ROOTS` for outputs) and restart; or edit the endpoint.
* The old default `./data/sync` is now `./data`.
* A sync whose `local_path` (or `source_directory`) is a single regular FILE rather than a directory is
  refused LOUDLY since round 6, in every mode: a local sync fails with `source path is not a directory`;
  WebDAV, S3 and GCS uploads fail with `upload source "<path>": is not a directory` (round-7 wording; round 6
  printed it without the colon; a missing source fails with `upload source "<path>": ...`) before any remote
  client is created; mirror and `copyFile` refuse a
  single-file root as "the root itself". Round 5 made the uploads copy nothing while reporting success
  (0 files); that silent outcome is gone. Point the endpoint at the directory that holds the file.
* A conversion whose `target_path` (or `source_path`) is a configured root itself, and a
  `POST /copy/local` with `destination_path` `.` or an equivalent, are refused since round 5.

## Follow-up (tracked, NOT implemented): dynamic storage-root provider

Storage roots registered at run time (through the storage-roots API) are not read roots unless listed in
`CATALOGIZER_READ_ROOTS`. Whether `pathguard` should consult them dynamically is an open operator decision
(UNCONFIRMED); until then, list them in the variable.

## Follow-up (tracked, NOT implemented): F4 TOCTOU

`Confine` validates then the caller opens by path; a symlink swapped in between validation and use is not closed.
Proper fix: open through `os.Root` (Go 1.24+) at the sinks - `performLocalSync` / mirror / incremental / bidirectional
copy+delete, `downloadFromWebDAV`, `uploadToWebDAV` (`scanLocalFiles`), S3/GCS `filepath.Walk`, conversion readers/writers
(the text writer; the external-tool outputs go through a private directory since round 7, but `Dir(target)` itself
can still be swapped), `LocalScanResolver` open, admin restore. Consolidation of `pathguard` into `internal/pathsafe`
is a separate later step.
