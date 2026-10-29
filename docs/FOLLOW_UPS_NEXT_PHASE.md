# Follow-ups for the next phase(s)

Small items left after independent review rounds, documented per the operator rule of
2026-10-28: mid-size or bigger findings get 5 more fix/review rounds; only small ones
land and are listed here. Layers follow constitution §11.4.235(D).
Status values: OPEN until a commit on `main` closes the item.

## Container hygiene guard (`scripts/tests/test_container_hygiene.sh`) — test-instrumentation
Found by the round-3 reviewer (landed series `a9e5248b`). Commit `8c73f797` claimed O1-O4 CLOSED; the
independent review of that commit returned NO-GO (a regression in the R3 exemption, `|| exit 256` exempted, and
compose shapes skipped silently), so those claims were wrong. Status after the round-2 fix commit:
- O1 (R7): long-form, flow-form, quoted-key, unknown-first-key and quoted short-form bind volumes are parsed;
  flow `volumes: [...]`, `volumes: *alias`, a volumes sequence at the key indentation and multi-line flow items
  fail closed as `R10-COMPOSE-UNMODELED`. OPEN only for the gaps listed in `docs/scripts/container_local.md`.
- O2 (R9): flag clusters, `-XHEAD`, `--request HEAD`, `wget --method=HEAD`, compose `test:` block scalars. OPEN only
  for the listed gaps.
- O3 (R3): the build-command class incl. env prefixes, `npx`/`yarn exec`, package-manager options
  (`--cwd`, `--prefix`, `workspace`), `sh -c "..."`, lowercase `run`, interpreter heredocs, `--mount` flags; masks
  `||`, single pipe, any `;`-trailing command, background `&`; `|| exit N` (1..255) and `|| false` exempt with the
  following delimiter kept; errexit/pipefail read per line and per stage `SHELL`. OPEN only for the listed gaps.
- O4 (R5/R10): quoted/space-before-colon keys modelled; tags, block scalars, aliases, flow values of
  `privileged`/`userns_mode`/`build`/`volumes`/`healthcheck`, multi-line flow bodies, services or bodies at an
  unmodelled indentation fail closed as R10. OPEN only for the listed gaps.
- Round-3 review of `ca23173e` (GO with follow-ups, all test-instrumentation, OPEN until a commit on `main` closes them):
  - F1-K (top priority): a modelled key with an empty value whose plain scalar sits on the NEXT line passes silently
    (`privileged`, `userns_mode`, `image` at indent 4; `test`, `context`, `dockerfile` at indent 6) so R5/R7/R9 are missed
    and `userns_mode: keep-id` on the next line is a false positive. Fix: take the value or report R10.
  - F2-N: errexit/pipefail are read by position, not scope: quoted `set -e`, `(set -e); build; true`, `set -e & build`,
    `true || set -e`, `! npm run build`, wrapped `timeout 600 npm run build || true`, a comment inside a `\` continuation,
    `sh -ce '...'`; `set -o errexit -o pipefail; build | tee` is a false positive. Also the already-listed `set +e`, `if build; then`, `ONBUILD RUN`.
  - F3-S: compose name scope misses `deploy/infra-compose*.yml` and `podman-compose.yml`; widening to `*compose*.y*ml` needs flow `volumes: [..]` modelled.
  - F4-M: add selftest fixtures for `-o errexit`, a `/bin/sh -e` path prefix and a `key: |` block-scalar golden-FALSE (code mutants CM3, CM6, CM9 survive; CM1 shows the subshell choice is untested).
  - F5-D: the gap list in `docs/scripts/container_local.md` omits the items above.
- RM6 (`destination` key): equivalent mutant for every verdict (no rule reads the mount target, only the source and
  `ro`), documented, not killable by a fixture.
- Round 3 (review of `507120b6` returned NO-GO): F1 odd/over-deep indentation inside a service, F2 non-plain
  top-level lines and env-substituted `privileged`/`userns_mode` now fail closed as R10; F3 exec-form RUN, a
  build word glued to `"`/`)`/`>`, quoted script names, `run-script`, and positional errexit/pipefail are
  modelled; F4 `curl -4I`, relative long-form bind sources and `target: /a:b` with `read_only` are modelled.
  The R3 items that stay OPEN are listed in the gaps (nested `sh -c` under `set -e`, `set +e`, escaped quotes
  in exec form). OPEN only for those.
Remaining gaps: `docs/scripts/container_local.md` (section "Hygiene guard parser coverage").

## Operator decisions pending (process-doc)
- `deployment/amber-up.sh` runs plain `docker` (allowlisted R6): keep or migrate to podman.
- `scripts/deploy-vision-hosts.sh` default host `thinker.local` (allowlisted R4): the operator
  said it is obsolete, but constitution §11.4.173 still names it as the build host; needs a
  recorded decision.
- Image `catalogizer/server` (`deployment/docker-compose.yml:109`, curl healthcheck): no
  Dockerfile in the repo builds it; whether curl exists in it is UNCONFIRMED.
- Android `/dev/kvm` as uid 1000 (F10) and the real-image F1 run: UNCONFIRMED.

## Secrets scanner (`scripts/` secret scan) — test-instrumentation (security series round 5)
- TI-1 (DONE, see scripts/tests/scan_literal_secrets.py): value-prefix exclusions (`/ [ { <`) hide real literals such as
  `JWT_SECRET: /k7Qz...`, `ADMIN_PASSWORD=[hunter2]`, `export DB_PASSWORD=/s3cr3t`. Treat `/` as a
  path only in the short-volume form; limit `<...>` to exact placeholders; add the six literals
  as must-flag fixtures and `VOL=api_keys:/app/keys` as must-not-flag.
- TI-2 (DONE, scripts/tests/generator_scan_test_roots.txt): generator scan skips every path containing `/tests/`; use an explicit list of test roots.
- TI-3 (DONE: scanner now also runs over the QA compose/scripts): the stack-boot test greps only the literal `admin123`; broaden or rely on the scanner.
- PD-1 (DONE: separate required `QA_USER_PASSWORD`, documented in `.env.example`): `run-full-qa.sh` registers `testuser_qa` with `ADMIN_PASSWORD`; use a separate value.
- Tracked follow-ups from the TI-1..PD-1 review: `catalog-api/run_challenges_robust.sh:14` still hard-codes `admin123`
  and its `grep -o '"session_token":"[^"]*"'` regex is a scanner false positive (the `[` value prefix); the gate was
  deliberately not widened. Residual scanner limits: a secret shaped like a plain lowercase multi-segment path
  (`/ab/cd/ef`) is treated as a path; a keyword-led `<your-...>` value is treated as a placeholder; the PD-1 alias check
  does not see indirect forms (`eval`, `printf -v`).
- Scanner residuals after the F1-F9 review round (documented, not closed):
  - `*_ENV` keys: an underscore-bearing all-caps token (e.g. `ABCD_1234_EFGH`) is accepted as a variable name; single-word names (`HOME`) are flagged.
  - Unclosed `<secret_…` style placeholders (any keyword) are accepted as placeholders; hex or no-digit slash secrets (`/ab/cd/ef`) are treated as paths.
  - URL-embedded credentials (`postgres://u:pw@h`) are not seen (the scanner is name-keyed); `.env` `export` lines, lowercase `-e db_password=`, `env K=v cmd`, and k8s `name:`/`value:` pairs are not matched.
  - `DB_PASSWORD: |` (YAML block scalar) is a false positive.
  - `catalog-api/start_server.sh:4` still carries a `JWT_SECRET` literal.
  - F9: the generator scan covers only `scripts/*.sh`; the other tracked `.sh` files (about 184) are unscanned.
  - The PD-1 alias guard sees only direct `VAR=...$ADMIN_PASSWORD...` assignments, not `eval` or `printf -v`.
  - Further scanner residuals (re-review round): `${A}/secret` (a `/`-led tail after an expansion is treated as a path suffix); a 4-digit-or-less value for a secret-named key in shell (`DB_PASSWORD=1234` in a `.sh`) is exempt as a counter; bare keys `PASS`/`PASSES`/`TOKEN`/`TOKENS` with a digit value in YAML are exempt (`PASS: 1234`); in shell only the first command of a `;` chain is checked (`DB_PASSWORD="$X"; DB_PASSWORD=hunter2`); known pre-existing false positives `PGPASSWORD="$PGPW"`, `${A:-}${B}` and `${DB_PASSWORD:-${OTHER}}`; the alias guard does not see assignments through `eval`, `printf -v`, `declare -n`, or `read`.
  - The `fail` initialisation guard in `test_no_default_secrets.sh` is textual: a late reset written in an obfuscated form (`fail=$((0))`) after the compose gate is not detected; a real fix is to run the gate in a subshell and propagate its exit status.
- Round-4 review leftovers (test-instrumentation, low): FU-1 the fail-init guard is textual, so `fail=$((0))`, `(( fail = 0 ))`, a backgrounded `compose_gate &` and a subshell call `( compose_gate )` get past it; replace it with an end-to-end meta-test that injects a literal into a fixture compose file and requires rc!=0.
  FU-2 the `run-full-qa.sh` alias guard misses `typeset`, `export -n`, `local -r` and env-prefixed `A=1 QPW=$ADMIN_PASSWORD`. FU-3 unbraced `$NAME<literal>` tails are not flagged. FU-4 a symlinked call from outside the repo exits 2 ("cd: null directory", fails closed); derive the repo top from the script's own directory.
- Out of scope but real: `admin123` still appears in `catalog-api/challenges/*`, `start_server.sh`,
  `run_challenges*.sh`, `check_scans.sh`, `catalog-web/e2e/fixtures/auth.ts`, the Android and
  Android TV `MainActivity.kt` (production app source: own item), and the helixqa banks.

## SQLCipher encryption at rest
- Review of `60ee1acb` (GO, three LOW test-instrumentation findings, mutants that survive):
  RV5 `scrubDSN` bypass in the new `probeInUse` open-error branch (the driver error never carries the DSN, so W1 cannot fail);
  RV6 dropped `db.Close()` in that branch (needs a goroutine-count check);
  RV4 NOTADB split in the `cipher_version` branch (probably an equivalent mutant).
- `classifyProbeError` busy branch (`encryption.go:522`) formats `err` without `scrubDSN`; no live leak, add it for consistency.
- `docker-compose.dev.yml` keeps a literal `MINIO_ROOT_USER: minioadmin` (a username, not a password).
- Windows behaviour is validated only through emulated `hostOS` and `go vet`; no run on real Windows.
- Descriptor-count tests are Linux-only (`/proc/self/fd`).

## Path confinement
- F4: TOCTOU between check and use remains open; plan `os.Root`-based sinks
  (see `docs/security/PATH_CONFINEMENT.md` once landed). Named windows (review round 10, F-D):
  WebDAV upload checks each file with `uploadableFile` at scan time, but `WebDAVClient.UploadFile`
  (`catalog-api/services/webdav_client.go`, function at line 113, `os.Open` at line 121 at commit
  `8e77e247`) opens it later with plain `os.Open`, without `O_NOFOLLOW`; a file or ancestor swapped
  in between is read. The same check-then-open gap applies to the WebDAV download write, `copyFile`
  and the `cleanupDestination` removes; all four are NARROWED by the round-9 per-file re-checks,
  not closed.
- Cost (review round 10, F-E): since round 9 each per-file sink confines twice (against the walk
  root and against the configured roots), so confinement work per file is about 2x. Not measured;
  measure on a large NAS tree before calling it acceptable.
- Consolidate `internal/pathguard` into `internal/pathsafe` after the security series lands.
- Review round 3, items T4 and P1: the review asked for them to be recorded here as follow-ups. Their
  content was never given to the fixer (the hand-off named only the labels), and no copy of the
  round-3 review was found in the fixer's working files. What T4 and P1 say is therefore not known
  to the author of this entry; the reviewer or the coordinator who holds the round-3 review has to
  fill it in. Until then this line only records that two round-3 items are open and unnamed.
- The sync walkers skip non-regular files and re-check each path, but a symlink or file swapped in
  between that check and the `open` is not caught (same race as F4). Round 6 checked the LibreOffice
  HTML output path before LibreOffice ran and again before the rename; since round 7 LibreOffice writes
  into the private converter directory instead (see below). `copyFile` opens its confined destination with `os.OpenFile` (no `O_NOFOLLOW`):
  same race, same `os.Root` fix. Since round 7 the external converters write into a private 0700
  directory and only regular files are renamed into place, so a link planted at the target or at a
  derived name (ImageMagick `out-N.ext`, `%d`) is no longer written through; `Dir(target)` swapped for
  a link between the check and `MkdirTemp`/`rename` remains the F4 race.
- Round 7 correction: the round-6 commit and docs called the conversion derived writes "closed"; only
  the Go writers and the HTML pre-check were. ffmpeg, ebook-convert, ImageMagick (`convertImage`,
  `convertPDFWithImageMagick`, including `name-N.ext` and `%d`) still wrote through links until round 7
  (S2). The round-6 report also gave 130 write sinks; the count at commit `e5b65d1d` is 129.
- Round 7, hard links (P4): a write through a hard link inside a write root to an arbitrary file outside
  the roots is not refused (only links to the protected database/key files are). `fs.protected_hardlinks=1`
  limits it to files the service user may write anyway. Fix needs a link-count (`st_nlink > 1`) policy on
  every write sink, which would also refuse legitimate hard-linked media; operator decision.
- Round 7, sync roots (S1): a symlinked sync root is resolved and the RESOLVED path is walked and used
  as the per-file confinement root. Correction (review round 8, item 1): the round-7 text said "nothing
  new is admitted". That was wrong. Round 7 resolved the root with `EvalSymlinks` at walk time,
  AFTER `confineSyncPaths` had checked it, and did not check the result again: a root swapped for a
  link to an outside directory in between was followed (a swapped source copied outside files into the
  destination, a swapped destination let mirror cleanup delete outside files; reviewer probes
  `TestRV8_Probe_RootSwapAfterConfine` / `..._DestSwapAfterConfine`). Since round 8 `performLocalSync`
  and the upload source (WebDAV, S3, GCS) resolve and confine each root in ONE step
  (`pathguard.ConfineData` returns the resolved path it checked) and walk that path; the walkers never
  resolve a root again. Behaviour change: a sync now also fails at run time when its root is outside
  the CURRENT roots (for example after `CATALOGIZER_SYNC_ROOTS` was narrowed), not only when the
  endpoint is saved. Error texts: `source directory does not exist`, `source path is not a directory`
  (unchanged), `source directory: ... outside the allowed roots`, `destination directory: ...`,
  `upload source "<path>": ...`. OPEN residual: a component swapped while the walk is running is the
  F4 race above.
- Round 9, ancestor swap (F1): round 8 only stopped a swap of the root's LAST component. An ancestor of
  the resolved root (inside the configured roots) swapped for a link to an outside directory after the
  run-time check was followed: mirror cleanup removed `outside/a/dst/precious.txt`, `copyFile` wrote
  into `outside/a/dst`, the upload walk read outside files, and the WebDAV download wrote outside (reviewer
  probes in `rv9.jXwU`, adopted as `services/ancestor_swap_rv9_test.go`). The per-file checks compared
  against the walk-root string, which `ConfineDataFile` re-resolves through the same swapped ancestor,
  so they passed. Since round 9 every per-file sink ALSO confines against the configured roots
  (`uploadableFile` and the `copyFile` source: read roots plus sync write roots; `copyFile` destination
  and parent, mirror directory guard, WebDAV download path: sync write roots; `cleanupDestination`:
  `confineRemoval` checks each entry's parent before each `os.Remove`, and fails closed).
  What these per-file re-checks DO protect against: an ancestor swapped to a target outside the
  configured roots before the per-file check runs. Correction (review round 10, F-C): they also
  refuse a swap that jumps below a DIFFERENT configured root (`prefixesStayWithin` requires the path
  to enter a root at the root itself and never leave it), and for writes a swap to the read root
  itself. What they do NOT protect against: (a) a swap whose target stays within the SAME checked
  root, or points at another checked root's own top directory; (b) a swap between a per-file check
  and the following `open` / `remove` (the F4 race). The race is narrowed, not closed; `os.Root`
  remains the plan.
  Behaviour changes: `cleanupDestination` never removes the destination root itself (`relPath == "."`),
  and a refused remove stops the cleanup with an error; a refused WebDAV download entry counts as a
  failed file and the download continues.
- Round 9, converter publish (F4 doc): checks and kept-mode `chmod`s are all applied before the first
  rename, but the renames are not atomic as a group; a failing rename leaves the earlier outputs
  published and the error reports how many. The chmod-before-rename ordering is not covered by a test
  (it needs fault injection into `os.Rename`); its mutant M9-CH (chmod moved into the rename loop)
  survives and is recorded as not covered (equivalent without a failing rename).
- Round 9 own mutants (script `mut9.py`): M9-1, M9-5, M9-6, M9-7, M9-8 and M9-N6 are killed by the
  adopted probes. M9-2 (copyFile source check widened to `/`) first survived because both callers run
  `uploadableFile` first; it is killed by the sink-level test `TestRV9_CopyFileRefusesSwappedSourceAncestorDirectly`.
  M9-9 (root-removal guard dropped) first survived because `confineRemoval` still refused the root;
  it is killed by `TestRV9_CleanupKeepsDestinationRoot` (asserts nil and that the root is kept). M9-3
  (copyFile destination widened) and M9-4 (destination parent widened) each survive alone because the
  other check refuses the same swap; widening both (M9-34b) is killed. The two new tests were written
  after the fix, so their RED is shown by the mutants.
- Round 10 (F-A): the reviewer's probes P1, P2 and P5 are adopted as
  `catalog-api/services/read_root_distinct_test.go` (read root and sync write root are DIFFERENT
  directories; P1/P2 are golden-FALSE). Mutants (script `mut10.py`, each restore byte-identical):
  X1 (`syncReadRoots` returns the write roots only) killed by `TestReadRootSourceDistinctFromSyncRoot`;
  X4 (`copyFile` write check widened to read roots), X5 (WebDAV download confine widened to read
  roots) and X6 (`confineRemoval` widened to read roots) killed by `TestSwapToReadRootItselfIsRefused`.
  These tests were written after the round-9 fix, so their RED is shown by the mutants.
- Round 9 equivalent mutants named by the reviewer: N8, N14 and N16 (documented equivalents in the
  round-9 review). N6 (keep `fi.Mode()` instead of `Perm()`) is now killed by
  `TestRV9_KeptTargetModeDropsSpecialBits` (a setuid/setgid and a sticky target; that test was written
  after the fix, so its RED is shown by the mutant, not by a pre-fix run).
- Round 8 mutants that survive, recorded as equivalent: R8-2 (destination pre-`MkdirAll` confine
  widened to `/`) and R8-3 (post-`MkdirAll` confine widened to `/`) each survive alone because the
  other check still refuses the swapped root; widening both (R8-23) is killed. R8-10 (`MkdirAll` on
  the unresolved destination instead of the confined path) survives: without a concurrent swap both
  name the same directory, and the post-`MkdirAll` confine refuses a swapped one (F4 race otherwise).
- Round 8, sync mode: an unknown `sync_mode` is refused before anything touches the file system (it
  used to create the destination directory first).
- Round 7 evidence gap: the reviewer's evidence file `EV/probe_symlinked_source.txt` named in the
  round-7 review does not exist at the given path; the probe test itself
  (`zz_reviewer_probe_test.go`) was found and adopted as `services/symlinked_root_rv7_test.go`.
- Round 7, process sinks: `exec.Command` call sites outside the conversion service (cover-art
  ffprobe/ffmpeg, darwin NFS `cp`/`mv`/`mount`) were classified, not changed (see the round-7 inventory
  in `docs/security/PATH_CONFINEMENT.md`).
- Round 7 mutant R7-10 survives and is recorded as equivalent, not tested: removing the
  `ConfineDataFile` check on each converter output destination changes nothing observable, because the
  destination is `Dir(target)` (already confined) joined with a `ReadDir` name (no `/`), and the only
  remaining escape (a leaf link pointing outside the roots) is refused by `refuseNonRegularLeaf`. The
  check stays as defence in depth against the `Dir(target)` swap race named above.
- Round 8 (item 6): a crash (or kill) of the service while an external converter runs leaves its
  private directory `.catalogizer-conv-*` next to the target; the deferred `RemoveAll` does not run.
  Not swept automatically: a sweep at start-up or per conversion cannot tell a stale directory from one
  a concurrent conversion is using without a lock or a process registry. OPEN: remove such
  directories by hand, or add a start-up sweep once conversions record their owner.
- Round 8 (items 4, 5) are documented, accepted behaviour of the private converter directory, see
  `docs/security/PATH_CONFINEMENT.md` ("What the private converter directory changes").
- `uploadableFile` is called by the S3, GCS and (since round 5) WebDAV upload walkers. The WebDAV one
  is driven end to end against a real in-process WebDAV server; S3 and GCS are not (they need a bucket
  client, and a double would only be a gate-logic test, §11.4.289), so only `uploadableFile` itself is
  tested for them (round-5 item V15, kept open).
- Round 4 missed the WebDAV upload walker (`scanLocalFiles`): it still uploaded symlinked and
  hard-linked source files until round 5 (RV5-A). A class-wide sink inventory is now in
  `docs/security/PATH_CONFINEMENT.md` ("Sink inventory").
- Filesystem `local_client` / `nfs_client` `resolvePath` is lexical (`ConfinePath`): a symlink inside
  the admin-configured storage base is followed outside it. Changing that changes behaviour for
  existing storage roots; needs an operator decision.
- `asset_resolvers` cover lookup checks the directory with `Confine` and each candidate with `Lstat`
  (regular file) but not for a hard link of a protected file (read only, fixed file names).
- `tests/integration` `TestDatabaseConcurrentAccess` fails when run on its own (3/3 runs on this
  branch, and also on `main` 0916e992 before the round-3 fix); it passed once inside the whole
  `./tests/...` run, so it is also order-dependent. Independent of path confinement; investigate
  separately.
- Round 6, cover art: `GET` cover download serves `cover_art.local_path` with `c.File` without
  re-checking it. Rows are written only by the cover-art service (cache dir + generated id), so this is
  a database-trust residual, not a request-supplied path; a confine against the cover cache dir (and
  the storage read roots for scanned local covers) needs the operator to say which roots are valid.
- Round 6 discovery (not path confinement): PDF to text, and therefore the PDF to HTML fallback, fail
  at run time in the test container with `failed to extract text from page 1: unipdf license code
  required`. The conversion feature is broken for PDFs without a unipdf licence key; nothing in the
  suite flags it because the existing tests stop before text extraction. Needs a licence decision or a
  different PDF text engine (§11.4.238 coverage escape: add a live PDF-to-text test).
- Path confinement refuses about 5% of random cases the kernel would have written inside the root
  (mostly `..` after a not-yet-existing element; see `confine_differential_test.go` false-refusal
  count). Safe direction; revisit only if a real client needs such paths.

## Live NAS tests
- FTP read-closer test leftovers (from the FTP review, recorded here as relayed; the author of this
  entry did not see that review): M1b, the fixed 250 ms quiescence bound (`quiescenceWindow`,
  `catalog-api/filesystem/ftp_readcloser_unit_test.go` line 113) is a timing assumption, not a
  proven bound; O1 is open and its content was not given to the author of this entry (UNKNOWN; the
  holder of the FTP review has to fill it in). The earlier "two blind sleeps" item is resolved and
  removed.

## Appendix digests (279-290) — follow-ups from Opus re-review (2026-10-29)
- m2, m5: minor test-instrumentation/process-doc findings from the re-review; texts to be re-extracted from the reviewer report (a8ee20da1e30f3655) when the next appendix pass runs.
- M6: add an id-to-name join check comparing digest gate ids/names with `constitution/scripts/gates/gate_ledger_deferrals.tsv` (the §11.4.288 OWED-GATE-150..152 name-reorder mutant survives phrase rows).
- 12 new digests not yet compared against groups/*.md (UNCONFIRMED).
