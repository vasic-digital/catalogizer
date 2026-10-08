# WF22 fix round 2: convergence assessment (11.4.276)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Fixer | single fixer, Sonnet (this pass); the dispatch path cannot set or report effort (11.4.231(F.2)), so no effort is claimed |
| Input | independent one-pass review `WF22-REVIEW-scanner.md`: NO-GO, 0 BLOCKING, 17 IMPORTANT (14 source-defect), 19 MINOR; 37 reviewer mutants of which 21 survived the author's tests |
| Round position | the review is round 1 of this work item; this is the fixer pass before round 2. Round budget `R_max`: none declared by the consumer, so the default 5 applies (11.4.276(A)); 1 of 5 consumed |
| Scope declared before the pass (11.4.276(E)) | `catalog-api/**` (settings contract, generic scanner, handlers, services, `tests/realfs`), the user docs of those keys, evidence `wp12/scanner/`. Not touched: `submodules/filesystem`, `scripts/**`, longops, anti-mess, coverage, register, existing test-infra files, `docs/scripts/README.md` |

## 1. Why the first pass needed a second one (the root causes, 11.4.276 forensic list applied to this item)

| Root cause (11.4.276) | Where it showed in this item |
|---|---|
| (1) wrong-model ground truth | the scanner and its tests modelled the catalog's path convention as slash-rooted, while `ensureDirectoryPathExists` (the writer of the parent chain) and the SMB scanner are slashless; the tests used `db=nil`, so nothing ever compared the two models. Also: the SQLite engine bundled with go-sqlcipher is older than 3.24 (see 3) |
| (2) instance-at-a-time closure of a known class | `filepath`->`path` was applied to the SMB join only (1 of at least 5 members); the settings mapping was unified in 3 of 5 `CreateClient` callers |
| (3) fixer-authored tests that never drive the real code | every job / realfs "end-to-end" test passed `db=nil` |
| (4) non-binding signals | the author's own UNCONFIRMED note about a relative FTP path was a stated risk that stayed a note |
| (5) scope drift | none observed |

## 2. Defect classes closed in this pass (each with its member inventory; instruments control-needled, `fix-r2-class-inventory.txt`)

| Class | Members found (review) | Inventory method | Closure |
|---|---|---|---|
| A. hand-built settings at a `CreateClient` caller | `cover_art_service.go` (5th caller) | `grep` of all production callers (5 found; positive control: the definition, 1 hit; negative control `ZzNoSuchMethod(`: 0) | all 5 go through `SettingsFromRoot`; AST gate test `TestWF22_R1_Gate_...` enumerates callers from the source and fails on a 6th without a mapping; per-caller matrix tests |
| B. catalog path convention / row identity | R17a (leading slash), R17b (`INSERT OR REPLACE`), T1 (no DB test) | the writers of `files.path`: `insertFileRecord` callers (SMB, local, generic) | one convention (slashless) converted at the DB seam; UPDATE-then-INSERT; SMB-vs-generic agreement test; DB tests on the production migrations; real-stack DB tests |
| C. loader omits columns | H1 (scan handler), H2 (create) | `FROM/JOIN storage_roots` queries that select `password` (6 found: scan handler, cover art, stream, comic, pdf, main.go identity ingestion) | shared column set `models.StorageRootConnColumns` + scan targets; source-scan gate `TestWF22_H1_Gate_...` (positive control: it finds >= 5 loaders) |
| D. root `path` forwarded into clients | WebDAV (replace), FTP (twice), NFS (correct), SMB/local (n/a) | read of each client's use of `config.Path` | WebDAV joins under the URL path; FTP records the server's `PWD` after the CWD |
| E. credential in a reason | WebDAV `fmt.Errorf` sites | `grep` of Errorf lines naming `fullURL/srcURL/dstURL` (18; 18 now wrapped, 0 not) | `redactURL` at the source + `scrubSecrets` at the `Reason` seam (fail / cancel / note / event payload) |
| F. WebDAV listing identity | R10, R11, F2, F3, W1 | the parser and `resolveURL` | name = href segment; `..` only as a segment; self-skip case-insensitive; direct children only; bounded streaming decode |
| G. written guarantees not kept | R2, R4, R3, R16, R12, R15, R6, R5, R14, S1 | every sentence of the guarantees table checked against a test | depth/entry bounds fail, list limit + wave processing, `allow_empty` (migration v21), thresholds, honest incremental claim, injective token, name rules, `#snapshot` |
| H. cancellation / status semantics | R7, R13, P0 | the status transitions of `processScanJob` | deadline = failed/timeout; cancelled event + reason payload; prompt return on cancel; panic path fails the visible status |
| I. `filepath` on remote paths | `insertFileRecord`, `resolveURL`, FTP (3 sites), NFS `FileInfo.Path` | import census (positive and negative control) | all remote-path sites use `path`; `TestWF22_X1_...` forbids `path/filepath` outside local / mount code; audited as safe: `filepath.Ext/Base` on file or archive entry names |
| J. settings validation gaps | F1, F4, C09 | `typeOK` / mapping read | range check, port 1-65535, stored port 0 = unset, empty domain column, hint gating test |
| K. reproducibility | P1 | `git ls-files` of the imported submodule packages | reported exactly in `fix-r2-reproducibility.txt`; not hidden, not fixable here (the submodule is another worker's) |
| L. evidence / harness integrity | T2-T4 | the harness read | new fail-closed harness on a private copy; logs carry the tree fingerprint |
| M. docs | P2-P4 | grep of the keys in user docs | docs and guides corrected |

## 3. Ground truth measured in this pass (not assumed)

- The SQLite bundled with `go-sqlcipher` rejects `INSERT ... ON CONFLICT DO UPDATE` (`near "ON": syntax error`, measured when the first upsert was tried: 9 existing tests failed) and has no `ALTER TABLE ... DROP COLUMN`. So the SQLite branch is UPDATE-then-INSERT in one transaction, and the v21 test builds the "v20" table directly.
- An exact-bound probe cannot be met by any scanner that lists a directory at all if the client cannot stop early (the reviewer's R4 hook counted the server-side size of `/a`): the fix therefore passes the remaining budget to the client in the context and the fake honours it; the WebDAV client stops decoding at it.
- The real stack (pure-ftpd, Apache mod_dav, corpus 24 files + 23 directories) CONFIRMED the reviewer's R16, which the review had only shown with an in-process MLSD server: on the pre-fix code a clean scan of the real FTP corpus ends with `error_count 48` (= 2 x 24 directories: the `.` and `..` entries); after the fix it is 0 (`fix-r2-realfs_red_prefix.log`, `fix-r2-realfs_green*.log`). It also confirmed, on the real server, that every row was stored with a leading slash and that the relative FTP root path (`movies`) scans a different tree than `/movies`.

## 4. Structural decisions recorded (11.4.276(E)(3) assessment)

No finding repeated a class across rounds in a way that forces a structural round: round 1 of the review produced the classes above; this pass closed each class with its inventory rather than its first instance. The decisions that need the owner (11.4.66) and were taken with the default stated:

1. Slashless catalog paths (SMB convention) rather than slash-rooted: existing rows and `ensureDirectoryPathExists` are slashless; the alternative would migrate existing data.
2. `allow_empty` as a column (migration v21, default false) rather than a JSON key in `options` (that column is a mount-options string for NFS).
3. A tree deeper than `max_depth` FAILS (`limit_exceeded`). With the API default `max_depth` 10 a deeper library now needs `max_depth` raised (cap 64); the alternative (truncate silently) is what the review rejected.
4. More than half of the sub-directories unreadable (or records unstorable) fails the scan; thresholds are options of the scanner (`MaxSkippedRatio`, `MaxRecordFailRatio`, default 0.5).

## 5. Not fixed in this pass (honest list)

| Finding | Why |
|---|---|
| R7 (part): a cancel endpoint, the `reason` field in `catalog-web/src/lib/scansApi.ts`, a UI notification for `scan.cancelled` | outside `catalog-api`; a cancel endpoint is new API surface (needs a job-context registry) that would itself need review; tracked in the README |
| P1 | the ordering dependency is another worker's: the submodule (with `pkg/fabric`, `pkg/decorators`) must be committed, pushed and its pointer bumped before `catalog-api` builds from a clean checkout; reported exactly |
| P4 (NFS real evidence) | structurally unavailable rootless (`syscall.Mount` needs CAP_SYS_ADMIN): recorded `blocked-unavailable`, not claimed |

## 6. Mutation results (final) and equivalent / unreachable mutants

Unit harness (`fix-r2-mutate.py`, private copy): 116 mutants, 116 killed, 0 survived after three passes (45 + 69 + 2); the first pass could not decide 71 because the host was short of memory (`run_pinned` REFUSED / linker killed), the second pass decided 69 of them, the last two had mutant text that did not compile and were corrected in the harness (E09, H06) and re-run. Negative control (no change) passed before every pass.

Real-stack harness: 11 mutants, 6 killed, 5 survived. The survivors are not coverage gaps of the unit regime:
- N13: masked on the real server by the FTP client's own `.`/`..` filter; killed at unit level where the scanner sees raw entries.
- N18: the real corpus is shallower than `max_depth`, so the depth probe is never reached; killed at unit level.
- N23, W03: on the real stack the failure path is the status-error branch, whose password is already redacted at the client; the scrub seam is reached only by an error that carries the raw URL; killed at unit level.
- T01: at a chroot login directory `/` the fallback branch gives the same result as the `PWD` base; killed at unit level with a non-root login directory.
Removed as dead or redundant while closing the classes: N06 (a fetch-bound check that could never fire after the wave change) and E08 (a self-skip made redundant by the direct-children filter). Tests added for survivors found in the unit harness: M18 (message test), A14, C07b, C07c (wholeNumber range), W03, W08, T02.
