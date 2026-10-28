# Filesystem path confinement (catalog-api)

Request-fed filesystem paths (sync endpoints, conversion jobs, cover-art lookup, asset cache,
backup restore) are confined to configured root directories by `catalog-api/internal/pathguard`
(symlink-resolving, fail-closed: an empty root list, relative path, NUL byte, symlink loop or any
resolution error is a rejection; a dangling symlink is followed to where a write would land).
Resolution walks the path exactly as the request gives it (it is not `filepath.Clean`ed first), one
component at a time the way the kernel does: every symlink is followed (dangling ones included), a
relative link target is resolved against the link's real directory, a `..` steps up from the directory
reached so far (so `root/lnk/../x` with `lnk -> /outside/sub` resolves to `/outside/x`, not `root/x`),
and a `..` that follows a not-yet-existing element is refused.

Before review round 3 the path was `filepath.Clean`ed before resolution, which folds `lnk/..` without
looking at `lnk`: `root/lnk/../pwn.txt` was accepted as `root/pwn.txt` while the kernel wrote
`/outside/pwn.txt` (N1). A differential test (`internal/pathguard/confine_differential_test.go`) now
creates random symlink trees and compares every verdict with what the kernel actually writes through the
raw path; seeds default/1/2/3 (4,000 cases each) give 0 escapes. The price is more refusals on the safe
side: about 200 of 4,000 random cases the kernel could have written inside the root are refused, nearly
all of them a `..` after a missing element.

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

`./data/catalogizer.db` is protected even if the server never registers it; at boot the configured
database path and key file are added, and the boot log line lists them as `protected=`. Use a
sub-directory such as `./data/sync` or `./data/converted` as a sync or conversion target. Cover-art
lookup (`LocalScanResolver`) uses the plain read-root check and is not affected.

Upgrade impact: a sync endpoint whose `local_path` (or a `sync_settings` directory) is `./data` itself, or
any directory containing the database, is now refused when it runs; point it at a sub-directory.

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

## Follow-up (tracked, NOT implemented): dynamic storage-root provider

Storage roots registered at run time (through the storage-roots API) are not read roots unless listed in
`CATALOGIZER_READ_ROOTS`. Whether `pathguard` should consult them dynamically is an open operator decision
(UNCONFIRMED); until then, list them in the variable.

## Follow-up (tracked, NOT implemented): F4 TOCTOU

`Confine` validates then the caller opens by path; a symlink swapped in between validation and use is not closed.
Proper fix: open through `os.Root` (Go 1.24+) at the sinks - `performLocalSync` / mirror / incremental / bidirectional
copy+delete, `downloadFromWebDAV`, `uploadToWebDAV` (`scanLocalFiles`), S3/GCS `filepath.Walk`, conversion readers/writers
(ffmpeg/ebook/text), `LocalScanResolver` open, admin restore. Consolidation of `pathguard` into `internal/pathsafe`
is a separate later step.
