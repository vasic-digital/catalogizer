# Follow-ups for the next phase(s)

Small items left after independent review rounds, documented per the operator rule of
2026-10-28: mid-size or bigger findings get 5 more fix/review rounds; only small ones
land and are listed here. Layers follow constitution §11.4.235(D).
Status values: OPEN until a commit on `main` closes the item.

> **Index (2026-11-05):** the id register is `docs/WORKABLE_ITEMS_REGISTER.md` (Part A points here for every existing follow-up id; Part B registers items that had no entry, e.g. `CAT-QA-SONAR-1`, `CAT-UI-AXE-LIVE-1`, `CAT-REL-V240-1`, `CAT-SYN-LIVE-1`). `docs/CONTINUATION.md` section 5.7 points to the same register. Add every new follow-up to the register as well as to the section below it.

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
- `deployment/amber-up.sh` now uses rootless `podman` (operator decision 2026-11-02; the R6 allowlist row is removed). UNCONFIRMED on amber.local (not contacted): podman installed, cgroup delegation for `--cpus`/`--memory`, and the image store (images built with docker must be rebuilt or loaded into podman).
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
- Tracked follow-ups from the TI-1..PD-1 review: `catalog-api/run_challenges_robust.sh:14` previously hard-coded the published default (fixed by CAT-021: `${ADMIN_PASSWORD:?}`)
  and its `grep -o '"session_token":"[^"]*"'` regex is a scanner false positive (the `[` value prefix); the gate was
  deliberately not widened. Residual scanner limits: a secret shaped like a plain lowercase multi-segment path
  (`/ab/cd/ef`) is treated as a path; a keyword-led `<your-...>` value is treated as a placeholder; the PD-1 alias check
  does not see indirect forms (`eval`, `printf -v`).
- Scanner residuals after the F1-F9 review round (documented, not closed):
  - `*_ENV` keys: an underscore-bearing all-caps token (e.g. `ABCD_1234_EFGH`) is accepted as a variable name; single-word names (`HOME`) are flagged.
  - Unclosed `<secret_…` style placeholders (any keyword) are accepted as placeholders; hex or no-digit slash secrets (`/ab/cd/ef`) are treated as paths.
  - URL-embedded credentials (`postgres://u:pw@h`) are not seen (the scanner is name-keyed); `.env` `export` lines, lowercase `-e db_password=`, `env K=v cmd`, and k8s `name:`/`value:` pairs are not matched.
  - `DB_PASSWORD: |` (YAML block scalar) is a false positive.
  - `catalog-api/start_server.sh:4` JWT_SECRET literal: removed by CAT-021 (now `${JWT_SECRET:?}`).
  - F9: the generator scan covers only `scripts/*.sh`; the other tracked `.sh` files (about 184) are unscanned.
  - The PD-1 alias guard sees only direct `VAR=...$ADMIN_PASSWORD...` assignments, not `eval` or `printf -v`.
  - Further scanner residuals (re-review round): `${A}/secret` (a `/`-led tail after an expansion is treated as a path suffix); a 4-digit-or-less value for a secret-named key in shell (`DB_PASSWORD=1234` in a `.sh`) is exempt as a counter; bare keys `PASS`/`PASSES`/`TOKEN`/`TOKENS` with a digit value in YAML are exempt (`PASS: 1234`); in shell only the first command of a `;` chain is checked (`DB_PASSWORD="$X"; DB_PASSWORD=hunter2`); known pre-existing false positives `PGPASSWORD="$PGPW"`, `${A:-}${B}` and `${DB_PASSWORD:-${OTHER}}`; the alias guard does not see assignments through `eval`, `printf -v`, `declare -n`, or `read`.
  - The `fail` initialisation guard in `test_no_default_secrets.sh` is textual: a late reset written in an obfuscated form (`fail=$((0))`) after the compose gate is not detected; a real fix is to run the gate in a subshell and propagate its exit status.
- Round-4 review leftovers (test-instrumentation, low): FU-1 the fail-init guard is textual, so `fail=$((0))`, `(( fail = 0 ))`, a backgrounded `compose_gate &` and a subshell call `( compose_gate )` get past it; replace it with an end-to-end meta-test that injects a literal into a fixture compose file and requires rc!=0.
  FU-2 the `run-full-qa.sh` alias guard misses `typeset`, `export -n`, `local -r` and env-prefixed `A=1 QPW=$ADMIN_PASSWORD`. FU-3 unbraced `$NAME<literal>` tails are not flagged. FU-4 a symlinked call from outside the repo exits 2 ("cd: null directory", fails closed); derive the repo top from the script's own directory.
- Out of scope but real (CAT-021 removed the literal from all of these except the helixqa bank steps that type it on-device: HelixQA does no env expansion in step text, so those `${ADMIN_PASSWORD}` placeholders are non-functional until HelixQA supports it; banks now declare `requires_env: [ADMIN_PASSWORD]` so they SKIP when unset; tracked: CAT-021-F4): it appeared in `catalog-api/challenges/*`, `start_server.sh`,
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

## FTP live-test follow-ups S1-S4 and T1 (recorded 2026-10-31)
Source: the item list (S1-S4, T1 and one-line descriptions of S1 and T1) was relayed by the operator's
conductor; the author of this entry did not see the FTP review that produced them. Everything else in this
section is the author's reading of the code. The priority "high" and the layer tags on S1 and T1 are the
author's assessment, not relayed, UNCONFIRMED by the operator. No `CAT-NNN` ticket exists for these items
(this file carries none); mint one per item when they are triaged. All OPEN until a commit on `main` closes
them.
- S1 (high, source; author's assessment): the FTP reconnect is not bounded. Read in
  `catalog-api/filesystem/ftp_client.go`: `reconnectIf` (line 240) wraps the caller's context with
  `context.WithTimeout(ctx, 60*time.Second)` (line 251) and passes it to `c.dial` (line 84), which calls
  `ftp.Dial` with `ftp.DialWithTimeout(30 * time.Second)` (line 87), then `Login` (line 93). Read in
  jlaffaye v0.2.4 `ftp.go`: `DialWithTimeout` only sets `dialer.Timeout` (line 189) and `Dial` uses the
  context only inside the TCP dial function (`DialContext`, lines 129-151). After the TCP connect, the 220
  greeting read (`ReadResponse`, line 168), the explicit-TLS handshake (`authTLS`, line 175, which runs only
  if explicit TLS is configured; `ftp_client.go` passes no TLS option, so it does not run for this client),
  `Login` and `ChangeDir` (line 101, the base-directory change in `dial`) have no deadline. So the 60 s and
  30 s literals bound only the TCP connect, and a server that accepts TCP and then stalls makes `reconnectIf`
  hang with no bound; `docs/FTP_CLIENT.md` line 11 says the same of this
  library. Fix: a deadline on the control connection (or a context-aware dial that closes it on
  cancellation), not only shorter literals; make the values configurable. Prove it with a live fixture: a
  real FTP server that accepts TCP and then stalls, or is cut off mid-session, never a stub (§11.4.289).
  `connTracker` (type at lines 493-497, doc comment 491-492) records every connection its `dialFunc`
  creates, i.e. all connections of the copy-destination `ServerConn`; it is used only in `CopyFile` (line
  761) and its `net.Dialer{Timeout: 30 * time.Second}` (line 500) bounds only their TCP connect. A link
  between this and T1 is plausible but UNCONFIRMED. UNCONFIRMED: the repo has two FTP clients.
  `catalog-api/filesystem/ftp_client.go` (the only FTP import in catalog-api production code, line 24)
  imports `github.com/jlaffaye/ftp`. The `pkg/ftp` client described in `docs/FTP_CLIENT.md` lives in the
  own-org submodule `submodules/filesystem` (module `digital.vasic.filesystem`; 18 files under `pkg/ftp` at
  the pinned gitlink, 0 `jlaffaye` lines in its `go.mod`) and is imported by
  `scripts/test-infra/nas_clients/nas_diag_test.go` (line 23) and `nas_clients_test.go` (line 40). Which
  client T1 concerns is UNKNOWN; the S1 analysis above is of the first.
- T1 (high, test-instrumentation; author's assessment): an FTP test hangs (which test was not given:
  UNKNOWN, to be identified). Two steps: (1) add a per-test timeout (`context.WithTimeout` in the test and
  `go test -timeout`) so a hang fails with a stack dump instead of blocking the run; (2) root-cause the hang
  (§11.4.102), reproducing it first (§11.4.115). Possibly related to S1 (UNCONFIRMED: no evidence links them).
- S2: UNKNOWN: details not found, needs triage. Searched `docs/` and the FTP commits `1c8acf3b` to
  `256c376c`. `docs/FTP_CLIENT.md` has no `S` labels; the labels it names include F1-F16, G1-G12, P1-P4 and
  T1-T5, D1-D7, D11, D12, D16, D17 and N1-N17 (earlier review rounds), which are not these items.
- S3: UNKNOWN: details not found, needs triage (same search as S2).
- S4: UNKNOWN: details not found, needs triage (same search as S2).
- Note: the `S1`/`S2` hits in the path-confinement section above are unrelated review items of that section.

## e2e credential-default follow-ups (recorded 2026-10-31)
Context: commit `0cfe6c63` ("test(e2e): require ADMIN_PASSWORD from env, remove literal credential defaults")
landed on `main`. The Playwright specs were not run in review (UNCONFIRMED that they pass with the variable
set). The items below were relayed by the operator's conductor; layers and severities are as relayed, except E4, E5 (re-classified to test-instrumentation) and E7 (author's reading). All
OPEN until a commit on `main` closes them.
- E1 (test-instrumentation, important): the control needle in `catalog-web/src/e2e-env.test.ts` covers only
  the `||` alternative of the scanner's three regex alternatives (`||`, `??`, `password:`); add a positive
  needle for each alternative.
- E2 (test-instrumentation, important): scanner blind spots: a dead `fill(` exclusion; only `.ts` files are
  scanned (not `.js`/`.mjs`); a destructuring default `const { ADMIN_PASSWORD = '...' } = process.env` is not
  matched; multi-line defaults are not matched.
- E3 (test-instrumentation, minor): the test does not assert that the `requireEnv` error message omits env
  values.
- E4 (test-instrumentation, important): `scripts/build-test-release.sh` (line 224 ends in `|| true`; a
  `record_phase "Web Frontend" "PASS"` follows at line 232, control flow between not traced)
  and `scripts/run-all-tests-comprehensive.sh` (lines 120-123, `log_warning` only) appear to swallow the new
  load-time failure when `ADMIN_PASSWORD` is unset (not run to confirm); they should fail or SKIP with reason `credentials_absent`
  (constitution §11.4.289(E)).
- E5 (test-instrumentation, important): `scripts/containers/run_playwright.sh` runs through `run_pinned.sh`,
  where the only `-e` flags found (line 261) are fixed cache/home variables, so no route for `ADMIN_PASSWORD`
  into the container was found other than putting it on the command line (not run to confirm); an env-file or
  an allow-listed passthrough looks needed (§11.4.173).
- E6 (process-doc, minor): `catalog-web/e2e/README.md` (line 15) says to export values in the shell or a
  gitignored `.env`, but no dotenv use was found in `catalog-web/package.json`, `playwright.config.ts` or
  `e2e/`; a search for `e2e/README` in `*.md`, `*.json` and `*.ts` finds only `catalog-web/e2e/fixtures/env.ts` (apart from this file),
  so no document links it; `catalog-web/docs/TESTING.md`, `docs/testing/TESTING.md`, root `AGENTS.md` and
  `catalog-web/AGENTS.md` do not mention `ADMIN_PASSWORD` (grep returned no match).
- E7 (source-defect, severity not assigned; layer is the author's reading): literal default passwords (the value
  `admin123`, which is compromised by disclosure: it appeared in repository history; it must be treated
  as compromised, and whether any deployment still uses it is UNKNOWN) remain outside e2e (catalog-api challenges and tests, `config.go`, `auth/service.go`, shell scripts,
  Android `MainActivity`, HelixQA banks, Website docs, and also `tests/k6` and `docs/issues`); track separately. Measured
  2026-10-31 with `grep -rlF` on the literal: `tests/k6` 16 files (all 16 files in that directory),
  `docs/issues` 22 files (of 1778 entries); control needle: the same command finds known-present
  `tests/k6/monitoring_test.js` and returns 0 for the absent literal `zzq_no_such_literal_9431` in both
  directories. The other locations listed were not re-counted here.

## Appendix digests (279-290) — follow-ups from Opus re-review (2026-10-29)
- m2, m5: minor test-instrumentation/process-doc findings from the re-review; texts to be re-extracted from the reviewer report (a8ee20da1e30f3655) when the next appendix pass runs.
- M6: add an id-to-name join check comparing digest gate ids/names with `constitution/scripts/gates/gate_ledger_deferrals.tsv` (the §11.4.288 OWED-GATE-150..152 name-reorder mutant survives phrase rows).
- 12 new digests not yet compared against groups/*.md (UNCONFIRMED).

## Redis rate limiter (landed 4d5cafcf, 2026-10-30) — open operator decisions and follow-ups
Source: independent Opus review of the fix plus a read-only audit; none of these were changed by 4d5cafcf.
- Production default is the in-memory limiter. Redis limiting needs `REDIS_RATE_LIMIT=true` AND a successful startup PING (`catalog-api/main.go` ~586-602, ~931); the PING is never re-checked at runtime.
- Operator decision: in Redis mode `AuthRedisRateLimiterConfig` (limit 5) and `DefaultRedisRateLimiterConfig` (limit 100) share the key `rate_limit:<IP>` (no username before auth), so ordinary traffic can lock a client out of auth routes, and `/auth/status`, `/me` are effectively 5/min against 600/min in memory.
- Operator decision: Redis presets (5/100/200 per minute) versus in-memory tiers (30/600/2000 per minute).
- Operator decision: `/login` and `/register` always use the in-memory limiter.
- DECIDED 2026-11-12 (CAT-OPDEC-A, PROVISIONAL): after startup, a Redis outage falls back to the in-memory limiter (see the CAT-OPDEC-A section); it no longer returns 503 on the wired routes.
- `TokenBucketRedisRateLimit` still fails open on a Redis error (the other two now fail closed).
- `docs/BACKEND_HARDENING_AUDIT.md` still says the limiter "activates whenever Redis is reachable"; the real condition is the flag plus the startup PING.
- Live test uses `tests.SetupTestRedis`, which runs FLUSHDB on DB 15 of whatever answers on localhost:6379; no live-class tests exist for the sliding-window and token-bucket limiters; the live test has about 100-300 ms timing margins (may be flaky on a loaded host).
- `retry_after` in the 429 JSON body is now millisecond precision (for example "44.8s"); no client code reading it was found.

## check_submodule_ff gate follow-ups (round 9 review)
Source: independent Opus review (round 9) of the gate (fixture suite 276/276, full mutation file baseline PASS plus 99 killed, no blocking finding). Gate files: `scripts/check_submodule_ff.sh`, `scripts/tests/test_check_submodule_ff.sh`, `scripts/tests/test_check_submodule_ff_mutation.sh`, `docs/scripts/check_submodule_ff.md`.
- N9-1: step 3 (the one-pin-resolvable test) cannot distinguish another repository from a shallow or stale object store. Ignore shallow or grafted stores when probing one-pin resolvability so such a case falls to BLIND; name shallow and stale stores in the doc's limits section. The current doc wording about "fetched before the older pin existed" is confusing and should be reworded.
- N9-2: reviewer mutants X1 (old-pin probe uses the new name), X3 (drop `[ -n "$name" ]`) and X4 (drop `[ -n "${E_NAME[$g]}" ]`) survive. Add fixtures: the old pin present only in the old-name store; a named removal with an unnamed same-path re-add, and the reverse.
- m1: no fixture covers the ls-tree branch where fewer gitlink paths differ than in the RANGE diff; needs a git wrapper to build.
- m2: commit-graph trust (the gate reads history through commit-graph data it does not verify).
- m3: documentation wording (see N9-1).
- Wiring: done as CPA stage S3 row `submodule_ff` (`scripts/repo/check_submodule_ff_range.sh`, guide `docs/scripts/check_submodule_ff_range.md`); open: owner approval of the registry row and of the manifest files (wrapper, gate, `scripts/audit/org_of.py`, `own_orgs.txt`) in the cpa-host trust file; a post-S5/pre-S6 range check (S3 sees only already-made commits); the `--repo` pending-pin rows are not covered.
- Wrapper review (716d36fb, Opus): open, not done by operator decision: m1, m3, m4 (minor findings of that review), t2 (test gap), d2 (doc gap), and the question whether BLIND should be split by reason (for example missing object store versus missing gate versus no remote tip) so that only some reasons are deferrals; all need the reviewer's text and an operator decision before work. Done in the follow-up commit: gate exit 1 with no `NONFF own` line is now a refusal 10 (was read as pass); `SUBFF_RANGE_BUDGET` clamped to at most 100.
- C1 (wrapper review round 3, not done): a remote whose name contains a slash is not matched by the `refs/remotes/*/main` glob in `check_submodule_ff_range.sh`, so its main tip is not checked; enumerate remotes with `git remote` and `refs/remotes/<name>/main` instead.

## CAT-001 follow-ups (tracked; not fixed in the CAT-001 remediation)
These are NOT claimed benign. Line numbers are as of the CAT-001 round-1 remediation commits in `catalog-api`.
- `internal/services/translation_service.go` (~223-229): language detection returns a fixed mock confidence of 0.8 instead of a measured value.
- `internal/services/music_recognition_provider.go`: chroma/MFCC features are derived from file bytes (pseudo features), not from decoded audio.
- `internal/services/deep_linking_service.go` `GetAppConfiguration`: returns a static default configuration, not stored/queried config.
- `internal/services/lyrics_service.go`: `getCachedLyrics` / `getCachedTranslation` are always-nil cache stubs; `detectConcertSetlist` (~830) returns an empty list.
- `internal/services/recommendation_service.go` `findSimilarGames`: swallows provider errors (returns fewer results with no error).
- `internal/services/game_software_recognition_provider.go` `recognizeSoftware` GitHub/SourceForge paths: not audited in CAT-001.
- `tests/mocks/smb_mock_server.go`: test-only mock server (legal only for unit-class use per section 11.4.289).
- `handlers/sysmetrics_cpu_linux.go` `hostCPUPercent`: samples /proc/stat over a deliberate 200ms window (a real measurement, not a stub; kept on purpose, adds 200ms to the system-info call).
- `GetCoverURL` goroutine panic seen once during CAT-001 work: UNCONFIRMED, see the investigation note below if present.
- Live OCR.space call (book OCR) was not exercised; only the not-configured and tesseract paths are tested.
- CAT-021 tracked follow-ups (review rounds 1-2, low): M2 the startup hash check costs one bcrypt compare per denylist candidate per admin account (about 11 x cost) - cache or short-circuit if admin counts grow; M6 the template rule (`<...>`, `${`, `$(`, `{{`) also rejects strong passwords that legitimately contain those sequences - document or add an operator override; N2 `internal/auth` `AuthService.Initialize` (plain-bcrypt schema) is unused outside tests, so only `main.go` seedDefaultAdmin/`admin-reset-password` protect the real schema - remove or unify; the JSON bank FQA-ATV-141 and the YAML banks type the literal `${ADMIN_PASSWORD}` until HelixQA expands env in step text (CAT-021-F4).

- CAT-021 round 5 (review round 4 findings): FIXED F1 (every `/api/v1` request now verifies the token's session is active, owned by the subject and unexpired; fails closed on DB error or missing `session_id` claim; one primary-key read per request, not cached), F2 (compose gate mutations M7/M8), F3 (shape regex anchored on both halves), F4, F5, F6, F7. NOT FIXED / honest limits: F8 the `admin-reset-password` path was only run against SQLite; the Postgres reset path and the container form (`docker compose run ... admin-reset-password`) were NEVER run. Strong passphrases containing a marker substring (for example `My-Secure-Passphrase-2026!xQ` contains `securepass`) are still refused by the substring marker list; making the markers exact would weaken the compromised-value list, so it stays a known limitation (operator decision needed). Session lookup per request adds one indexed read; a short TTL cache would trade revocation latency for load and was not added. Tokens issued by `GenerateToken` (no `session_id`) are rejected once a checker is installed; no production path mints them.

## WrapHTTPHandler path-param fix (catalog-api/handlers) — follow-ups (review rounds 1-3)
Source: independent Opus review rounds 1 and 2 of commits c567caca, b328d027, b339ce88 and the round-3 commit. Status: OPEN.
- S3 (minor, design): wizard step routes have no permission check while GetConfiguration requires system.config and TestConfiguration requires system.admin; ValidateWizardStep returns err.Error() to the client.
- S4 (minor, design): UpdateRole PUT replaces permissions, so a body with only `name` empties a custom role's permissions; system roles are protected by `is_system = 0` (user_repository.go).
- T1 (minor, test-instrumentation): the fallback branch of routedID (handlers/path_params.go) is not exercised by any test; no routed path reaches it today.
- T6 (minor, test-instrumentation): ValidateWizardStep and SaveWizardProgress have no authz or behaviour test. Also (round 5): the memStore.GetRole fake invents an empty role for unknown ids, unlike the real repository; RequireAdmin gates /api/v1/admin by role_id == 1 while the UserHandler guards use the system.admin permission (agree today because role 1 is seeded ["*"]); a self-update carrying role_id is silently ignored with 200.
- Config wizard (`ConfigurationHandler.GetWizardStep`, `ValidateWizardStep`, `SaveWizardProgress`, `GetWizardProgress`, `CompleteWizard`) has no permission check: any authenticated user can call it. Decide whether wizard steps should require `system.config` or `system.admin`. `ValidateWizardStep` and `GetWizardStep` were not given a `user_id` check (they do not read `user_id`).
- Admin-over-admin rule (S1 fix) is conservative: a non-admin is refused for any target whose role carries `system.admin` or `*`, and for assigning such a role. It does not model role hierarchies ("at least as privileged as the caller's own role" for non-admin roles); a non-admin can still assign a non-admin role that is more privileged than their own.
- The new gates use fakes for services and repositories (real handlers, real router, real adapter). No live-server run exists for these 22 routes.
- `GetLogShare` is token-gated only (RequireAuth at the `/api/v1` group is the single authentication layer); share tokens are not rate limited separately.
- R3-M1: the admin-over-admin checks are check-then-write (target role read, then update): a concurrent role change can produce a lost update or a stale decision. Needs a transactional or compare-and-set update in the repository.
- R3-M2 (see CAT-050 FIXED and CAT-OPDEC-A below): a non-admin with user.manage / user.create may assign ANY non-admin role, including a role more privileged than their own (only roles that carry `system.admin`, `*` or a matching prefix wildcard are refused).
- Other writers of users.role / role_id found in the R3-S1 sweep: `repository.UserRepository.Create/Update` (reached only via UserHandler, now guarded); `handlers/auth_handler.go` RegisterGin (fixed `RoleID: 2`); `internal/auth` `CreateUser` (fixed role 'user') and `UpdateUser` (sets `role` from request) are reachable only through `internal/handlers.AuthHandler`, which no code constructs (`NewAuthHandler` is never called outside its definition), so they are not exposed under /api/v1 today; if that handler is ever wired, `UpdateUser` must drop `Role` for non-admins. No bulk import, invite or signup flow writing roles was found in handlers/, services/, internal/, repository/.

## Responsive audit series (commits `responsive audit fixes ...` and `header nav keeps priority ...`)
Independent review round 1 returned GO-with-follow-ups. Corrections to the two commit messages (history is not rewritten):
- Commit 1 says requests "were 404ing at /api/v1/api/v1/...". Correction (F4): only the doubled PREFIX is fixed in the
  favorites and playlists client calls. Most of those endpoints still have no server route (item F3 below) and still 404,
  now at the right path.
- Commits 1 and 2 do not mention that the header greeting "Welcome, {name}" is now `hidden 2xl:inline` (not shown below
  1536px) (F9). Nit: the password eye button is `h-11` inside an `h-10` input (2px overhang each side).
- Both commits' evidence is a PARTIAL re-run of the browser harness (`fixedE` runs of 28 and 8 cases), not one full
  matrix per browser on the final tree.

Landing-blocker items closed in the landing-prep commit: F1 (8 Button snapshots updated; diff is only the two added
`[@media(pointer:coarse)]:min-h-[44px] [@media(pointer:coarse)]:min-w-[44px]` classes), F2 (`AuthRace.test.tsx`, RED on
the pre-fix `AuthContext.tsx`, GREEN after), F7 (Tabs overflow class guards, RED on pre-fix `Tabs.tsx`), F8 (oracle
needle cases for the scroll-container exemption, both directions mutation-killed), trailing whitespace in `oracle.mjs`.

Tracked follow-ups (status OPEN):
- F3 (source-defect, important, pre-existing): client calls with no route in `catalog-api/main.go`: `/favorites/stats`,
  `/favorites/toggle`, `/favorites/bulk` (POST and DELETE), `/favorites/check/:mediaId` and `DELETE /favorites/:mediaId`
  (shape differs from the server's `:entity_type/:entity_id`); playlists `GET /:id/items`, `items/reorder`, `share`,
  `shared/:token`, `analytics`, `duplicate`, `export`, `play`, `shuffle`, `reorder`, `import`, `validate-smart-rules`.
  `Favorites.tsx` uses `getFavoriteStats` and `bulkRemoveFromFavorites`, so that page still gets 404s. Needs a
  client/server contract reconciliation plus a contract test (§11.4.244).
- F5 (process-doc, important): residual browser-audit failures after both patches, from the author's verdict files:
  catalog-web chromium 101 of 133 cases FAIL, webkit 123 of 152 FAIL, firefox only a pre-fix run exists (162 of 171
  FAIL); website 35 of 35 FAIL on tap_target_small (VitePress hero buttons 40px). Needs one full-matrix run per browser on
  the final tree and a tracked item per remaining finding class (tap_target_small, occluded_control, h_overflow,
  offscreen_control, no_gutter, text_off_viewport, overlapping_text).
- F6 (source-defect, minor, UNCONFIRMED): header nav `overflow-x-auto` may clip the `focus-visible:ring-2 ring-offset-2`
  focus ring of nav links (top/bottom, and left of the first link). Found by reading CSS only; confirm in a browser,
  then add `py-1 px-1` to the nav.
- F7 design item (source-defect, minor): links overflowing the header nav at 768-1535px have no visible cue and the
  harness no longer reports them; consider a "More" menu or an edge fade.
- Open tap-target items from the audit (see F5) and the "More" menu decision above.
- Pre-existing, not caused by this series: 7 failures in `catalog-web/src/contexts/__tests__/AuthContext.integration.test.tsx`
  on `main` (localStorage `auth_token`/`user` `setItem`/`removeItem` assertions not met; login/logout cycle). Root cause
  not trivial and out of scope here; needs its own investigation (§11.4.102). Also pre-existing: 3 eslint warnings in
  `catalog-web/src/e2e-env.test.ts` fail `--max-warnings 0`.
- `tests/responsive-audit/run.mjs` still hardcodes port 18400 and local paths and is not wired into any gate (F8 rest).

### Responsive follow-ups, closed on branch `fix-responsive-followups` (2026-11-03)
- F5 and the open tap-target items: closed. A central `@media (pointer: coarse)` rule (zero specificity, `:where()`) plus a
  `.touch-target` helper in `catalog-web/src/index.css`; shared Tabs, PageHeader, Header, Collections/Playlists gutters.
  Full-matrix after-run (3 browsers x 6 viewports x 2 themes, 228 cases each): tap_target_small 1084/1084/1076 -> 0/0/2
  (webkit 2 are 132x44 inputs, a sub-pixel rounding artefact), h_overflow 12 -> 0, offscreen_control 28 -> 0,
  occluded_control 36 -> 0, text_off_viewport 38/32/32 -> 0, no_gutter 53/48/48 -> 0.
- F7 design item: closed. Header nav is tiered from one `NAV_ITEMS` list; overflow goes to a "More" menu; the full search box
  shows from `lg`, a compact Search link at `md`.
- Docs HTML without the viewport meta: generator `scripts/docs/render_md_to_html.sh` plus gate
  `scripts/docs/check_html_viewport.py` (§11.4.12); 4 files regenerated.
- Review r1 (NO-GO, F1 important) fixed in the follow-up commit: the More control is now a disclosure (list of plain links,
  `aria-expanded` + `aria-controls`, no menu roles); Escape and item selection return focus to the trigger; tests for
  Escape-in-list, Escape-on-trigger, Tab order, click outside, selecting an item; mutations (remove click-outside, drop
  the focus return) die. Touch-target test now parses the media query exactly; gate parses the real meta tag with a
  carrier fixture; regenerated docs use `pagetitle=` (one h1); search clear buttons no longer cover input text
  (`pr-14`); gate and `--check` are run by `scripts/pre-flight-check.sh`; audit oracle records unrounded sizes and waits
  for layout to settle (`rec.unsettled` is recorded when it never does).
- STILL OPEN, operator decision (11.4.66) D1, "Disconnected" badge layout. It now clears the version chip on phones
  (`bottom-10`), but a fixed overlay can still cover in-flow content at the page bottom while the WebSocket is down
  (admin "0%", AI Search button at 390px). Options: (1) auto-dismiss after ~5s and keep a header status dot;
  (2) dock it in the header on phones (no overlay at all); (3) reserve bottom space in Layout while it is visible.
  Recommendation: (2), since a persistent overlay on a 360px screen always covers something.
- STILL OPEN, operator decision (11.4.66) D2, desktop target size (`tap_target_small_desktop`, about 1280 reports per
  browser, mouse pointer). Today 44px is enforced on coarse pointers only. Options: (a) enforce the WCAG 2.2 AA 2.5.8
  minimum of 24x24 CSS px everywhere and keep 44 on coarse; (b) keep 44 on coarse only (status quo); (c) enforce 44
  everywhere. Recommendation: (a). Nobody has decided this yet.
- CLOSED on branch `fix-ui-d1-d2` (2026-11-03, operator decision: fix to spec; both recommendations applied):
  - D1 (badge): the connection badge is no longer a fixed overlay. `ConnectionStatus` is an in-flow `role="status"` chip docked in
    the header (`Header.tsx`), icon-only below `sm` (text stays for screen readers), icon + text from `sm`. RED (real
    chromium, real catalog-api, `tests/responsive-audit/measure_d1_d2.mjs`, 5 viewports x 2 themes x 6 pages = 60 cases): 28 of
    60 FAIL, the badge intersected "Smart Collection", selects, "Refresh", "Last updated", etc. at every width from 320 to 1440.
    GREEN: 60 of 60 PASS, 0 unobserved, 0 intersections.
  - D2 (target size): `@media (pointer: fine)` now enforces the WCAG 2.2 AA 2.5.8 floor of 24x24 CSS px (central zero-specificity
    `:where()` rule, `.touch-target`, login "Remember me" and "Forgot password?"); 44px on coarse pointers is unchanged (touch
    viewports measured 0 violations before and after). The audit oracle uses 24 on fine pointers and `run.mjs` now fails on
    `tap_target_small_desktop`. RED: 8 oracle findings under 24px at login (20px) plus "Home" breadcrumb 38x20 on `/playlists`.
  - New unit guards (RED on revert, three mutations each killed): `ConnectionStatus.test.tsx` placement, `Header.test.tsx` slot,
    `touch_targets.test.ts` fine floor.
  - Further defects found while measuring (NOT fixed, OPEN): (a) the fixed version chip `v1.1.0` (Layout) overlaps the primary
    "Smart Collection" button on `/collections` at 320x568 (visible in the screenshot, the chip is `fixed` and not in the
    oracle's overlap rule because it has no text overlap with a text box of its own); (b) on `/collections` two chevron icons
    float right of the select boxes (detached from the controls) at 320 and 375; (c) `/collections` shows "Render: 0.0ms" and
    "Page 1/0" developer-style stats to end users; (d) the audit harness `run.mjs` still has fixed default ports and is not
    wired into a gate; `measure_d1_d2.mjs` needs the real API and a built dist.
- D1/D2 review follow-ups (tracked, open; review of fix-ui-d1-d2 @7a539819, GO-with-follow-ups):
  - F2: the connection chip's live region announces "Disconnected" for about 1.3 s on every page load before the socket opens. Show the chip only after a grace period or after the first successful open.
  - F3: use a persistent live-region wrapper (always mounted, content changes) so assistive tech reliably announces state changes; today the `role="status"` node mounts together with its text.
  - F4: chip contrast: red-100 text on red-500 is about 3.1:1 (below 4.5:1 AA for 14px text); check the yellow "Connecting" chip too and pick token pairs that pass in light and dark.
  - F5: `tests/responsive-audit/measure_d1_d2.mjs` should force the socket down deterministically (`page.routeWebSocket`), include `/login`, and cover more than one fine-pointer viewport (it currently measures 1440 only for the fine floor).
  - F7: commit the firefox and webkit evidence JSON, with an artifact fingerprint, to `docs/qa/responsive_d1_d2/` (only chromium is committed).
  - F8: a clean `npm ci` may fail to load 4 test files for lack of the devDependency `@testing-library/dom` (pre-existing); not reproduced or changed here.
  - Version chip overlap at 320 px, the header chevrons, and the dev stats strip still need a responsive pass.
  - Done in this round: F1 (App.test.tsx now asserts the chip is not rendered by App) and F6 (24x24 passes / 24x23 flagged pinned in needle.mjs).
- Oracle tolerance: webkit reported tap_target_small at exactly 44.00 x 44.00 in one of two identical runs (7 cases; r2b), i.e. a float/layout-unit error below 0.01px. The oracle now allows 0.02px (`minPx - 0.02`). Not re-proven by a full matrix run yet.
- OPEN, UNCONFIRMED: the earlier webkit `tap_target_small` pair (register inputs, 132x44 after rounding) and the reviewer's
  3 chromium results (ActivityFeed tabs) were not root-caused. The oracle was changed to record unrounded sizes and to wait
  for layout to settle; the cause of the earlier values is not established.
- entity-movie, entity-album, identities FAIL only on 404s from the audit data / missing `/api/v1/identities` route (not
  layout). The VitePress website was not part of this run.

## Runner scripts (CAT-013/014/016) review round 3 follow-ups (GO-with-follow-ups, all minor)

- N1: `go fmt` failure is silent and `go fmt` rewrites files instead of checking (use `gofmt -l`).
- N2: phase 4 leaves no note when `SKIP_E2E_TESTS=true`.
- N3: phase 9 failed API-client copy is only a warning.
- N4: phases 5 and 6 pass when the build copied zero artifacts.
- T1: report fixture has one pass/skip/fail, so swapped counters in JSON/HTML go unnoticed.
- T2: no test covers `skipped_list` for SEC, E2E, PERF.
- T3: no test covers the phase 7 path where the dumpsys call itself fails.
- Nits: unguarded `cd` in the E2E branch; `redis | grep -q` has the F2 shape (low risk).
- UNCONFIRMED: adb connect exit semantics. ESLint findings stay non-fatal by policy.
## CAT-002 PDF text isolation - follow-ups (review round 5)
- F-E3: the pure-Go child re-runs all package `init`s of the API binary (about 80 MB RSS, 0.1-0.2 s) on every call; a small dedicated child binary or lazy init would cut this.
- The hostile fallback chain (poppler killed by the 20 s CPU limit, then the Go child) measured 20.7 s total; a stricter bound would need a shared per-file budget across the two engines.
- A genuinely large legitimate document (more than roughly 4000 pages of pure-Go extraction) can now exceed the 20 s CPU limit; poppler handles about 8000 pages in the same budget. Make the CPU limit page-count aware if such inputs are expected.
- Measurements were taken on a 16-CPU host; GOMAXPROCS=64 is simulated by environment, not run on 64 CPUs. A full catalog-api image build and a container run of the extractor were not done in this round.

## CAT-002 review round 5 follow-ups (GO-with-follow-ups, all minor)

- F1: the child env allowlist is tested only on the poppler path; a mutation giving the Go child the full environment survives.
- F2: mutations dropping GOMEMLIMIT and raising the child ceiling to 4 GiB survive (ceiling assertion compares against its own constant); marker check HasPrefix vs Contains (nit).
- F3: "851 pages in ~4 s" holds for the generated fixture only; real very large PDFs (e.g. 851 and 570 pages) cannot be extracted by the pure-Go engine (>120 s) and rely on poppler.
- F5: limits are sized from host MemAvailable, not the container memory limit; three children at their ceilings could exceed a container limit. No compose file limits catalog-api memory today.
- UNCONFIRMED: image build and in-container extraction; real 64-CPU host.


---

## Crash-pages fix, round 2 follow-ups

Minor or out-of-scope items found while fixing the four crashing pages
(`/analytics`, `/collections`, `/admin`, `/settings`) and the class-exhaustive
sweep of the api modules. Each is a tracked item for the next iteration; none
blocks the crash fix.

| ID | Item | Where | Why deferred |
|----|------|-------|--------------|
| WEB-F4 | Favorites are always created with `entity_type: 'movie'` (`DEFAULT_ENTITY`) | `catalog-web/src/lib/favoritesApi.ts` | Needs the page to pass the real entity type; UI change |
| WEB-F5 | `addToFavorites` returns a synthesised, not the stored, `Favorite` | `favoritesApi.ts` | Backend returns no row; needs a backend contract decision |
| WEB-F6 | Favorites stats are derived client side from at most 200 rows | `favoritesApi.ts` | Needs a backend stats endpoint |
| WEB-F7 | Favorites tab is unfiltered by type and sort is always ascending | `pages/Favorites*` | Product decision on filter/sort semantics |
| WEB-F12 | `useOptimizedData` has a global cache keyed off its inputs and is unused by `Collections` now | `components/performance/MemoCache.tsx` | Do not delete without a §11.4.124 history investigation |
| SMB | `POST /smb/discover` needs a JSON body with `host` (400 without), so the Settings "Discover Shares" button cannot work; the client is now null-safe but the button needs a host input | `lib/smbApi.ts`, `pages/Settings.tsx`, backend `SMBDiscoveryHandler` | Product decision: scan a subnet server side, or add a host field |
| COL | `collectionsApi` still calls routes the backend does not serve (smart-rule preview, templates, automation, integrations) | `lib/collectionsApi.ts` | Backend features absent; pages should hide or label them |
| ID | identities/discovery endpoints answer 404 | catalog-api routes | Feature not implemented |
| SIM | `getSimilar` shows owned (local) similar items only; `external_items` are not rendered | `lib/recommendationsApi.ts`, `pages/EntityDetail.tsx` | UI for external suggestions not designed |
| MET | `/admin` performance counters are cumulative since process start and `error_rate` is already a percentage; uptime and total requests are not in the report | `reporting_service.go` | Needs a report schema extension |

## Production mock data (§11.4.289)

These ship canned data in production paths and must be removed or clearly
labelled; they are listed so they are tracked, not because this change touches
them:

- `catalog-api/handlers/recommendation_handler.go` lines ~231-275
- `catalog-api/internal/services/recommendation_service.go` lines ~229-234, 713, 751, 1220-1290
- `catalog-web/src/lib/adminApi.ts` lines ~4-75 (mock fallbacks used on 404 around lines 89, 101, 113, 125)

## Docs C2/C5 landing (openapi.yaml re-verification) - source defects found (review round 1)

Measured on the real catalog-api binary on 2026-11-03 and documented in `docs/api/openapi.yaml` and
`docs/api/API_DOCUMENTATION.md` as known defects. The fixes belong to a separate fix work item.

- R2 (important, FIXED in the login/validator fix, branch fix-user-login-validator): `PUT /api/v1/users/{id}` never saved `role_id`. The response is 200 and echoes the requested
  `role_id`, but `UserRepository.Update` (`catalog-api/repository/user_repository.go`) has no `role_id` column, so a
  later GET shows the old role; `is_active` is saved. Changing a user's role through the API does not work. The defect
  is pinned by `catalog-api/openapi_contract_binary_test.go` (assertion "role_id is not persisted"); the fix must flip
  that assertion and update both docs.
- R5 (important, FIXED: passwords are exempt from pattern checks): the request validation middleware rejects passwords that contain SQL keywords (for example a word
  such as `Create`) as potential SQL injection. Reported by review round 1; the exact keyword list is UNCONFIRMED here.
- F1 (important, FIXED): users created with `POST /api/v1/users` cannot log in until an admin runs reset-password (password
  stored as SHA-256, login verifies bcrypt).
- F2 (important, FIXED): the same validator rejects the permission names `user.create`, `user.update`, `user.delete` on
  `POST /api/v1/roles`; only `user.view`, `user.manage` and `user.*` pass.
- F3 (minor): `DELETE /api/v1/logs/share/{id}` with an unknown id answers 500, not 404; several 404/500 bodies echo
  raw service or SQL error text; error reports store the server host name in `system_info`.
- F4 (minor): an empty body on `POST /api/v1/users/{id}/unlock` is rejected (400 `invalid JSON format`).
- F6 (minor): `scripts/check_doc_reachability.py` does not require a direct README to `docs/README.md` link.

## Docs C2/C5 review round 2 follow-ups (GO-with-follow-ups, process-doc minor)

- R6: docs say every cell of the user/role permission table is "asserted by" the binary contract test; the test only uses `user.*` roles, so single-permission cells (`user.view` only, `user.manage` only) are measured live but not asserted. Add those two roles to the test or soften the wording.
- R7 (measured): the input validator pattern list is `catalog-api/middleware/input_validation.go:57-66`. On POST /api/v1/users it rejects whole words CREATE, AND, OR, FROM, SLEEP and `--` in ANY string field (password, display_name "Rock and Roll"); `Selection` and `Order-By` pass. Permission names `user.create` fail for the same reason. FIXED in the login/validator fix (structural SQL-injection detection).

## Login/validator fix (branch fix-user-login-validator) follow-ups

- SQL-injection detector (`utils.ContainsSQLInjection`) is a heuristic defence in depth; values reach parameterised queries. The review measured 12 of 51 payloads passing before round 2 (subselects `1 AND (SELECT COUNT(*) FROM users)>0`, `1 OR TRUE`, `') OR ('1'='1`, `SELECT*FROM users`, `1 AND extractvalue(...)`, `1' GROUP BY 1,2 HAVING 1=1`, URL-encoded forms, `1'||(SELECT 1)||'`); round 2 closed the cheap ones. Remaining misses are tracked, not claimed solved. Prose such as "Select items from list" is still flagged.
- Password-field exemption is keyed on the leaf field name at the TOP level of the JSON body only (nested `password` keys are checked). Credentials nested deeper in future endpoints would be pattern-checked.
- Legacy SHA-256 password rows: sunset plan, gauge and startup log shipped (docs/security/LEGACY_PASSWORD_HASH_SUNSET.md); the verify path is removed only once the gauge reads 0. Dormant accounts still need a forced reset.
- CLOSED 2026-11-03: `HashData` removed (dead since c0c5ac78).
- CLOSED 2026-11-03: `database/schema_v3_multiuser.sql` no longer seeds the published-hash admin.
- The access-token `role_id` claim is trusted by `RequireAdmin` until expiry. Session revocation on role change closes the gap while the session checker is installed; a DB re-check of the role in `RequireAdmin` was considered and not done.

- R7 (measured): the input validator pattern list is `catalog-api/middleware/input_validation.go:57-66`. On POST /api/v1/users it rejects whole words CREATE, AND, OR, FROM, SLEEP and `--` in ANY string field (password, display_name "Rock and Roll"); `Selection` and `Order-By` pass. Permission names `user.create` fail for the same reason. Under investigation in the login/validator fix.

### Crash-pages fix, round 3 follow-ups

| ID | Item | Where | Why deferred |
|----|------|-------|--------------|
| WEB-N3 | Similar items carry a file id built by `generateMediaID` but `EntityDetail` uses it to navigate to `/entity/:id`: it may open the wrong entity. UNCONFIRMED latent | `recommendation_service.go`, `pages/EntityDetail.tsx`, `lib/recommendationsApi.ts` | Needs a reproduction with real media rows; scratch DB has none |
| WEB-N4 | No test asserts the `genre` field of `getMediaMetadata`; a mutant mapping genre from the director column survives | `handlers/web_contract_realdb_test.go` | Add a seeded-genre assertion |
| WEB-N5 | `SubtitleService.SearchSubtitles` returns canned "Sample Movie (2024)" results in production (§11.4.289 production mock data) | `internal/services/subtitle_service.go:~485` | Needs real provider integration or removal |
| WEB-N6 | Subtitle page sends `query`, `limit`, `offset` that the search handler ignores; `/subtitles/media/:id` carries no title/year so the client fills blanks | `handlers/subtitle_handler.go`, `lib/subtitleApi.ts` | Backend contract extension |
| WEB-N7 | Remaining `omitempty` list/map fields were swept: the unguarded ones consumed by catalog-web were the two subtitle lists (fixed); the others (`audio_tracks`, `chapters`, `cast`, `external_ids`, `metadata`, wizard `fields`) are read behind `x && x.length` guards or are not consumed | `catalog-api/**` | Keep the sweep in the review checklist |

## Crash-pages review round 3 follow-ups (GO-with-follow-ups, test-instrumentation minor)

- R3-1: add a subtitles fixture where `media_item_id` differs from the requested id (removing the `media_item_id` -> `media_id` mapping survives today).
- R3-2: the Go wire test calls the response builders directly; add a handler-level test so a handler that stops using them is caught.
- UNCONFIRMED: live subtitle flow on a populated DB; recommendations query on Postgres; Playwright suite not re-run by the reviewer.

## Login/validator fix round 3 follow-ups (GO-with-follow-ups, minor)

- `revokeUserSessions` now logs when the repository lacks `DeactivateAllUserSessions` instead of staying silent, but still does not fail closed (in-memory test doubles keep no sessions); consider a compile-time requirement on the production handler constructors.
- XSS rule nits: `behavior:url(...)` (old Internet Explorer only) is not detected; the open-tag rule's `<\s*` also flags prose such as "x < link y" (browsers do not parse `< tag` as a tag); drop the `\s*` after `<`.
- catalog-web: after change-password the success toast is followed by a redirect to /login on the next 401; log out explicitly or keep the caller's own session.
- Pre-existing on main (not caused by this branch): admin "suspended" status sets IsLocked but `UserRepository.Update` does not persist `is_locked` and no sessions are revoked; `tests/stress` CacheWriteThroughput and middleware TestTokenBucketRedisRateLimit_Unit_Miniredis_RefillAfterEmpty are flaky under load (section 11.4.248 quarantine candidates).

## WebDAV live-test gap (recorded 2026-11-03)

- Status: SKIP(service_unavailable). The Synology NAS answers on 445/80/443/5000/5001 only; WebDAV ports 5005/5006 refuse connections and DSM web ports answer PROPFIND with 405 or the login page (200, no DAV header). No WebDAV stand-in was used (section 11.4.289).
- OPERATOR ACTION: DSM > Package Center > install and enable "WebDAV Server" (enable HTTPS, port 5006), allow the test user on the share, then run `LIVE_WEBDAV_URL=https://<nas>:5006 LIVE_WEBDAV_USER=... LIVE_WEBDAV_PASSWORD=... LIVE_WEBDAV_SANDBOX=/work20/Testing LIVE_WEBDAV_INSECURE=1 go test -tags live -run TestLiveWebDAV -v ./filesystem/` in `catalog-api`.
- UNCONFIRMED: the positive legs (connect, list, write, read-back sha256, delete, 404, wrong-credential 401) of `catalog-api/filesystem/live_webdav_test.go` have never run against a real server; only the SKIP paths are proven. Re-run 3 times once the service is enabled and capture redacted output.
- Add a paired mutation (wrong password via env, wrong sandbox) to the WebDAV live test as the SMB/FTP live tests have (`LIVE_MUTATE`).
- Pin the Synology certificate instead of `LIVE_WEBDAV_INSECURE=1` (as `submodules/filesystem/pkg/ftp/tls.go` pins per host; confirmed).
- Review R1 F12/F13 (deferred): IPv6 literal hosts are mangled by the redaction host extraction; the probe URL escaping should reuse the client's URL builder. Positive-leg assertion that `ReadFile` error text contains `404` depends on the client's message format and is UNCONFIRMED live.
- Review R2 F-R2-5 (note): Go follows 301/302 on PROPFIND by switching to GET, so a redirect to a login page is SKIPped as non-WebDAV; acceptable.

## Documentation follow-ups (docs link audit, 2026-11-03)

Status: OPEN until a commit on `main` closes the item.

- Six runbooks named by `docs/runbooks/ALERT_RUNBOOKS_INDEX.md` do not exist and were un-linked, not invented: SLOW_QUERIES, BRUTE_FORCE, DOS_ATTACK, NODE_DOWN, NETWORK_ISSUES, CERTIFICATE_EXPIRY. Each needs a real runbook written from the alert rules and the code, then re-linked from the index, `DATABASE_CONNECTION.md`, `HIGH_CPU.md` and `SECURITY_BREACH.md`.
- Link-checker weaknesses (T1, test-instrumentation, the checker is not in the repository yet; the audit copy lives in the session scratchpad): (a) its control check ended in `or True`, so a broken control could never fail; (b) it ignores `#anchors`, so a link to a heading that does not exist passes (the spec 001 crawl found 83 broken anchors); (c) it had no VitePress site-root mode, so 37 `Website/` links such as `/download` were reported broken. The scratchpad copy now asserts a positive and a negative control and resolves `Website/` root links against `Website/` (`.md` and `index.md` forms); anchors are still unchecked. Land a checker in `scripts/` with these three properties plus a gate before relying on its counts.
- Link-check false positives to exclude when the checker lands: backticked code such as `NewValue[T](loader)` in `docs/architecture/LAZY_LOADING.md` and spec 001 doc 19, and links into submodules that are not checked out.


## CAT-001 review R1 deferrals (2026-11-03)

Status: OPEN until a commit on `main` closes the item.

- F10 (latent source-defect, round 2 corrected): `ConfigurationWizardService.testNetworkDatabaseConnection` dials a user-supplied host:port. `NewConfigurationWizardService` is never constructed in production code today, so the probe is unreachable now; it becomes an SSRF-shaped probe the moment the service is wired. Before wiring it, add an allow-list or private-range policy and a test through the real router.
- F2 follow-up: `POST /api/v1/copy/storage`, `POST /api/v1/admin/storage/scan` (use `POST /api/v1/scans`), `POST /api/v1/admin/backups/{id}/restore`, `GET /api/v1/sync/devices`, `/sync/conflicts`, `/translations`, `/localization/*` and `/recommendations*` now answer 501 because no engine exists. Building a real copy-to-storage engine, restore, recommendation engine, device registry and localization catalog is feature work, not done here.
- F2 web follow-up: `CollectionAutomation`, `AIUserBehaviorAnalytics`, `AIPredictions`, `AISmartOrganization` show an "unavailable" state; the collection-analytics usage tabs and the dashboard activity feed need a server-side events source before they can show data.
- F3 follow-up: no subtitle provider client is implemented; `GET /subtitles/providers` is an empty list and `POST /subtitles/search` is 501 until at least one client (for example OpenSubtitles) is written and live-tested.

## CAT-001 review R2 residual fake-data inventory (2026-11-03)

Converted to an honest unavailable state with vitest tests: CollectionExport import, SmartCollectionBuilder test-rules, CollectionRealTime, AIMetadata, AIComponents, AIAnalytics, CollectionAutomation, CollectionAnalytics, ActivityFeed, AIDashboard metrics and random counter, Dashboard user statistics, Analytics random growth chart fallback.

Remaining, deliberately not changed (reason in each):

- `pages/AIDashboard.tsx` and `pages/Collections.tsx` pass literal "Sample Content" items to the AI panels. The panels are unavailable placeholders that ignore their props, so nothing invented is displayed; remove the literals when the panels are rebuilt on a real engine.
- `components/collections/AdvancedSearch.tsx:199`, `SmartCollectionBuilder.tsx:100` and `components/upload/UploadManager.tsx:50` use `Math.random` only to mint client-side ids, not displayed data.
- "Sample Items" headings in CollectionExport and SmartCollectionBuilder label real parsed or matched items, not invented ones; SubtitleSyncModal "Sample Duration" is a real input label.
- `lib/adminApi.ts` and `lib/identitiesApi.ts` mention "mock" only in comments saying they never fake a response.
- Test-only helpers (`test-setup.ts`, `src/test/`) are not user reachable.

## CAT-001 review R3 class inventory (2026-11-03)

Class: user-reachable UI or API output that is not derived from real data or a real action (invented numbers, timer-driven progress or status, canned arrays, "queued"/"started"/"success" answers with no side effect).
Instrument: `sweep.sh` (grep over `catalog-web/src` non-test for Math.random, simulate/mock/fake wording, setTimeout/setInterval, trend and delta strings, 'connected'/'Live' literals, count/size/popularity literals; plus a grep for StatusAccepted/"queued"/"accepted"/"started" over `catalog-api` non-test Go). Control needles found by it: UploadManager "Simulate", Analytics "+12% from last", CollectionSharing 'connected', CollectionTemplates size '45.2 GB', CollectionExport "Simulate export", stub_handler.go "Metadata refresh queued"; a fabricated needle returned 0 hits.

Fixed in R3 (with tests): UploadManager (no timers; "not available" without an upload handler, success only after the handler resolves); MediaBrowser (no fake onUpload); DashboardStats (trends removed, quality from real counts); Analytics (change/trend props removed; W4 test added, mutation dies); CollectionTemplates (popularity, counts, sizes, estimated items and the "Most Popular" sort removed); CollectionSharing (fake "Live" status removed); CollectionExport (timed fake progress removed); ExternalIntegrations (test, sync, toggle and delete report the server's real answer; no success toast on 404); Admin (restore/scan show the server's 501 guidance); `POST /api/v1/media/:id/refresh` (501, no row change); `POST /api/v1/entities/:id/metadata/refresh` with no database (501); openapi and API_DOCUMENTATION (refresh, trending summary); CH-145 challenge (501 or 404).

Reviewed, real, not changed: `POST /scans` (202 after a real `QueueScan`); `POST /admin/backups` (real file written; 202 with status "completed"); `POST /media/analyze` (a DB lookup via `mediaQueryHandler.AnalyzeMedia`, stub_handler.go:418; it analyses nothing, it reads stored rows); `EnrichAllEntities` (real background goroutine); `ConnectionStatus` interval (reads the real WebSocket state); `SplashScreen`, `PlaylistPlayer`, `MediaPlayer`, `PerformanceOptimizer`, `Collections.tsx` debounce, `lib/utils.ts`, `useDebounce`, `SubtitleUploadModal` form reset (UI timing, no invented data); Math.random id minting (AdvancedSearch, SmartCollectionBuilder, UploadManager).

Listed, not fixed (reason):
- `pages/AIDashboard.tsx` setTimeout alert handlers and "Sample Content" literals: the panels are unavailable placeholders that ignore their props, so the handlers cannot fire.
- `components/performance/MemoCache.tsx` "for now" measurement stubs: performance utility, shows no user data.
- `ExternalIntegrations` create/save are local-only because the server has no `/integrations` API; the toasts now say so. A backend integrations API is the real fix.
- Admin storage "Scan" cannot start a real scan from a path: `POST /scans` needs a `storage_root_id`. The UI shows the server's guidance.


## Authorization gaps (branch fix-authz-gaps, CAT-049/050/051/052)

- F4 / CAT-051 FIXED: `RequireAdmin` consults `UserRepository.IsAdmin` (current role permissions, active user) when `SetAdminChecker` is installed (main.go does); the role_id claim is only the fallback for stacks without the checker. Fails closed (503) when the lookup fails.
- R3-M1 / CAT-049 FIXED for the target side: `UserRepository.UpdateGuarded` is a compare-and-set on role_id and is_active; `UpdateUser` answers 409 when they changed between the decision and the save. Other columns are still last-write-wins. A caller demoted mid-request is not re-checked (its sessions are revoked on demotion).
- R3-M2 / CAT-050 FIXED: a non-admin may only assign roles whose every permission it holds itself (role 2 was exempt until CAT-OPDEC-A, 2026-11-12; it is now compared like any other role). OPERATOR DECISION still open: whether this subset rule is the wanted role model; acting on (not assigning to) a more privileged non-admin target is still unguarded.
- CAT-052: `memStore.GetRole` now returns an error for unknown roles like the real repository.
- N1 (review r1, minor; DECIDED and FIXED by CAT-OPDEC-A, role 2 is now compared against the caller): `defaultUserRoleID = 2` was hard-coded as always assignable by a user manager. If an admin later widens role 2's permissions, any user manager can hand them out. Decision needed: compare role 2 against the caller too, or keep the baseline.
- N2 (review r1) FIXED: `Permissions.HasPermission` prefix wildcards now need a segment boundary (`.` or `:`), so `sys*`/`s*`/`system*` no longer grant `system.admin`; `*`, `system.*`, `admin:*` unchanged. Existing roles stored with such boundary-less wildcards lose that grant (none are seeded).
- N7 (review r1, minor): the compare-and-set covers only the target's role_id/is_active; a concurrent change of the target role's permissions or of the caller's own role is not re-checked; `admin_handler.UpdateUser` still uses a plain Update.
- B1: `newAuthMiddleware` (main_jwtwire.go) is the only constructor main.go uses; `TestMainGoWiresJWTMiddlewareThroughHelper` fails if main.go builds a bare middleware.

## Login/validator follow-ups closed (branch fix-lv-followups, 2026-11-03)

- SQLi detector: 5 corpus misses closed (`admin' #`, `admin'/*`, `x';SHUTDOWN`, `1' ORDER BY 1#`, `PROCEDURE ANALYSE()`), and two prose false positives removed (`a -- b`, `Loop; sleep well`).
- XSS: `behavior:url(` detected; `< tag` with a space is no longer flagged.
- Legacy SHA-256 hash: metric `catalogizer_auth_legacy_password_hashes`, startup log, plan in docs/security/LEGACY_PASSWORD_HASH_SUNSET.md (verify path not removed).
- `AuthService.HashData` removed (dead since c0c5ac78). `database/schema_v3_multiuser.sql` no longer seeds the published-hash admin.
- Session revocation is interface-required and runs revoke, update, revoke (fails closed). The revoke pair alone does NOT close the login race: `AuthService.Login` reads the user, runs bcrypt (about 57 ms) and inserts the session afterwards, so that session is never revoked. Round 2 closed it in `Login` itself: after `createSession` the user row is re-read and the new session is deactivated (login refused) when role, is_active, is_locked, password hash/salt changed (tests: `services/login_stale_state_test.go`, hook `loginPostSessionHook`). Remaining window: between that re-read and the response (microseconds, a change landing there is revoked by the handler's second revoke). After a failed second revoke the handlers answer 500 and a repeat of the same request revokes again (the revoke is keyed on the request naming role_id/is_active/status inactive, not only on a value diff). catalog-web signs out locally after change-password.
- Round-2 review (R2) records, OPEN: (a) SQLi detector `--`/`#` rule is a single linear pass with a numeric-prefix rule; KNOWN FALSE POSITIVES by operator decision (security over prose): a purely numeric title before `--` (`2001 -- A Space Odyssey`) and any text with an odd number of quotes before `--` after a digit or `)` (`Don't stop (live) -- remaster`); bare `-- ` / `---` are also blocked. Decision candidate for the operator. (b) Still-missed attack shapes: `@@version LIKE`, `sqlite_version()`, `COPY ... TO`, `DBMS_PIPE`, percent-encoded forms, `0x31=0x31`. (c) False positive `Select your favourite movies from the list`. (d) The `&&` rule against quoted prose was not probed (UNCONFIRMED). (e) catalog-web: 7 `AuthContext.integration` and 2 `WebSocketContext` tests fail on main before this branch; they blind token-storage regressions in those files.
- SQL-injection detector, measured 2026-11-03 on the independent review corpus (72 attacks, 66 legitimate strings): the first fix missed 7 attacks and blocked 15 legitimate strings. Fixed since: substr/randomblob/load_extension probes, `' || '1'='1`, bare `1 order by N`, context-aware `--`/`#` comments ("Episode 3 -- The Return", "Ep. 7 #"), `union select committee`, `Sleep (2010 film)`. STILL OPEN (cannot be told from an attack by structure alone): percent-encoded payloads (JSON bodies are not decoded), `Delete from favourites` / `Drop table manners at dinner` (indistinguishable from `DELETE FROM users` as a bare value), `5 > 3 and 2 < 4`, `she said 'hello' and 'bye'`, `-moz-binding:url(` in the XSS rules. Real protection stays parameterised SQL.
- `AdminHandler.UpdateUser`: "suspended" sets IsLocked but is not persisted and revokes nothing; the `role` field is parsed and ignored. Pre-existing.

## Conversion worker fix follow-ups (minor)

- The worker requeues `running` jobs at start, which assumes a single API instance per database; with several instances use a lease column (`claimed_by`, `claimed_until`).
- `convertPDFToImage` reads `dpi`/`page` settings as Go `int` from decoded JSON (always `float64`), so those settings are silently ignored; same family in `start_page`/`end_page`.
- Converter tool stdout/stderr go to the process stdout; capture a bounded tail into `error_message` for diagnosis.
- `retry` is allowed only for `failed`; `cancelled` jobs cannot be re-queued (the openapi text used to promise it).
- Not verified here: ffmpeg/pandoc/ebook/LibreOffice paths through the worker (only ImageMagick end to end); the Postgres dialect of `ClaimJob`/`RequeueRunning` (plain `?` placeholders, as the rest of the repository).

### Review R1 deferrals (conversion worker)

- F4: a foreign job id answers 403 while an unknown id answers 404, so ids are probeable (ids are sequential); consider 404 for non-owners without permission (decision pending).
- F6: `RunWorker` refuses a second start in one process, but two API instances on one database still requeue each other's running jobs; needs the lease column (`claimed_by`, `claimed_until`).
- F10: `TestClaimJob_ExactlyOneWinner` runs on a single-connection in-memory SQLite; it is a logic check, not a concurrency proof against Postgres.
- F11: `ClaimJob`, `RequeueRunning`, `GetDueJobs` and `UpdateJobIf` have no Postgres run in the committed suite (SKIP, tracked: needs a Postgres test DSN in the suite). The R2 reviewer ran them once against a throwaway postgres:16 with the production migrations and they passed; that run is not committed.
- Per-job timeout default is 2h (`CATALOGIZER_CONVERSION_TIMEOUT`, Go duration); no per-type limits (ffmpeg vs image).
- R2-N2 / R3-C3 residual (`scheduled_for` rows written before the UTC normalisation):
  - SQLite (measured): the stored text keeps the original offset, so the rewrite is exact. `ConversionRepository.NormalizeLegacyScheduledFor` runs at every worker start, rewrites pending rows to UTC and is idempotent (tests: `TestNormalizeLegacyScheduledFor_RewritesOffsetRowsToUTC`, `TestRunWorker_NormalizesLegacyScheduledForAtStart`; a legacy +14:00 due row was measured as NOT returned before the rewrite and returned after it). Unparseable text is left alone and logged as a count.
  - Postgres (measured by the R3 reviewer on a throwaway postgres:16: a legacy +14:00 row that is due is NOT returned): the column is `TIMESTAMP` without zone, so the original offset was dropped at write time and cannot be recovered. By reasoning (not measured): every old scheduled pending row written in a non-UTC zone is delayed by the writer's UTC offset (jobs are delayed, never lost). Nothing is rewritten on Postgres; operators who need exact times for old rows must reschedule them. The code assumes nothing about the old zone.
  - Reach is small: no first-party client sends `scheduled_for`.
- R2-N7 (= F4): 403 vs 404 for another user's job id stays an operator decision.
- Pre-existing, not from this branch: `GOOS=windows go vet ./services/` fails on older test files that use `syscall.Mkfifo`/unix helpers (`pathconfine_*`, `converter_private_dir_rv7`, `pdf_text_*`); the conversion worker tests are split so they add no new windows failure (`conversion_worker_unix_test.go`). The non-test package builds for windows.
- The `pgid > 1` guard is pinned by `TestGroupKillTarget_RefusesPidsAtOrBelowOne` (pure function); the surrounding `Cancel` closure is not exercised with a faked pid.

## Documentation follow-ups 3

Found by `scripts/docs/check_md_links.py` (links to documents or assets that do not exist; nothing was invented, the links were turned into plain text):

- `docs/COMPREHENSIVE_IMPLEMENTATION_PLAN.md`: the table of contents lists "Appendices" but the document has no such section; the entry is marked "not written yet".
- `docs/diagrams/SEQUENCE_DIAGRAMS.md`: `images/sequence-diagrams-11.svg`, `-12.svg`, `-13.svg` (Admin System Info, Lazy Provider Search, Non-Blocking Health Check) were never generated; the table cells say "not generated yet".
- `docs/INSTALLATION_GUIDE.md`: 28 screenshots under `docs/screenshots/` (wizard, docker, admin, troubleshooting, dashboard) were never captured; each image is replaced by "screenshot not captured yet" with its caption. Capturing them needs a real wizard run (HelixQA recording), not mock images.
- `docs/deployment/DEPLOYMENT.md`: the table of contents had no dedicated "Environment Configuration" and "SMB Configuration" sections; both entries point to "Production Environment Variables" (the nearest real section). A dedicated SMB configuration section is still unwritten.
- `README.md` table of contents: "Frontend Documentation", "Media Detection Engine" and "External Providers" had no section in the README; they now point to `docs/architecture/REACT_FRONTEND_GUIDE.md`, `docs/architecture/MEDIA_RECOGNITION_PIPELINE.md` and `docs/ENV_VARIABLES.md` (nearest real documents; a dedicated external-providers guide is not written yet).
- `docs/CONTRIBUTING.md` had a stray code fence that hid the "Pull Request Process" heading; removed. The issue-template block near "Logs" nests a fence inside a fence and will still render early on GitHub (use a four-backtick outer fence); not changed here.

## Docs link checker review follow-ups (review GO-with-follow-ups, 2026-11-03)

Source: independent Opus review of `scripts/docs/check_md_links.py` (merge 71ab088e). All minor.

1. test-instrumentation: `scripts/docs/tests/test_check_md_links.py` does not cover duplicate-heading `-N` suffixes, emoji/punctuation slugs, reference-style link definitions, setext headings, `--alt-root`, or the BLIND control path. Reviewer mutations M1, M2, M5, M6, M7 survived the tests (M5 only the real scan catches). Add one fixture per case plus a test that a blinded `controls()` exits 2.
2. test-instrumentation: `CHECK_MD_LINKS_NO_EXISTENCE` is a mutation switch built into the shipped checker; with it set in the environment the pre-flight check passes silently because `controls()` ignores it. Remove the switch (mutate a test copy instead) or make `controls()` refuse when it is set.
3. process-doc: `scripts/pre-flight-check.sh` hides the BROKEN rows (output to /dev/null) and runs without `--alt-root`, so in a checkout without initialised submodules it fails on 11 submodule-path links. Print the BROKEN rows on failure.
4. nit: `check_md_links.py` anchor lookup falls back to case-insensitive match; UNCONFIRMED whether GitHub resolves mixed-case fragments that way.
5. nit: setext heading underline requires `-{2,}`; CommonMark accepts a single `-`.

## CAT-001 review R4 class inventory (2026-11-03)

Class: user-reachable output that is not derived from real data or a real action.
Instruments (independent of `sweep.sh`): the reviewer's `sweep4.py` (web and API pattern sweep), `toasts.py` (every `toast.success(` in catalog-web non-test code, flagged when no `await`/`onSuccess`/`mutateAsync` precedes it in the enclosing function) and `goerr.py` (Go `if err != nil` blocks that answer 200/201/202). Control needles: `sweep4.py` negative control 0 hits, positive controls found Math.random / StatusAccepted / `"success": true` sites; `toasts.py` found the known `useFavorites.tsx:61` and `Collections.tsx:417-453` sites; `goerr.py` found the known `admin_handler.go` storage-list site before the fix and none after (remaining hits reviewed below).

Fixed in R4 (RED test first, mutation killed): R4-1 Dashboard system status (real `/admin/system-info` figures or "Unavailable", no invented Online/Network); R4-2 Dashboard quick actions (real navigation; Scan Library disabled with a reason); R4-3 `GET /configuration/status` (501, no invented health); R4-4 template apply (real create call, success only after the server answers; the server refuses smart collections with 501, so a template apply fails honestly); smart collections: `POST/PUT /collections` answer 501 for `is_smart` / `smart_rules` instead of silently dropping them; R4-5 AdvancedSearch (Search disabled with a reason, "Search saved for this session only"), CollectionPreview (no fake "Playing/Previewing/Downloading" toasts, no Download button); R4-7 media refresh test (snapshot taken before any call; constant-write mutation dies); N4/N5 ExternalIntegrations toggle and delete failure tests; R4-9a `GET /admin/storage` DB error is 500, not 200 `[]`; sweep finds: CollectionSharing "QR Code" (copied an invented `catalogizer.app/shared/<id>/qr-<ts>` URL and said a QR code was copied: now disabled with a stated reason) and `copyToClipboard` (success toast before the write resolved: now awaited, error toast on rejection); `download.go` directory `estimated_archive_size` (was the directory entry's own size, labelled "Placeholder": now omitted).

Remaining members, with disposition (nothing below is silently accepted):
- R4-6 `catalog-web/src/pages/Collections.tsx:411-453`: nine AI handlers toast success ("AI suggestion applied", "AI rule executed", ...) for actions they do not perform; they cannot fire today because the AI components ignore these props. Disposition: remove the handlers or wire real actions when the AI components get real props.
- R4-9b `SmartCollectionBuilder` save: now shows the server's 501 as a generic mutation error ("Request failed with status code 501"); an explicit message is a UI follow-up. UPDATE 2026-11-03 (merged smart collections, migration 23): smart collection storage and evaluation now exist (`internal/smartrules`, `GET /collections/:id/items`), so `POST/PUT /collections` no longer answer 501 for `is_smart` / `smart_rules` and the earlier N2/N4 items for smart collections are closed; see "Smart collections and smart playlists" below for what remains open.
- `catalog-web/src/hooks/useFavorites.tsx:61`: "Added to favorites" is toasted in `onMutate`, before the server answers (the optimistic update is rolled back on error, but the success toast is not retracted). Disposition: move the toast to `onSuccess`.
- `catalog-api/handlers/localization_handlers.go:423`: constant `confidence: 1.0` in a response. Disposition: derive or drop.
- `mediaApi.analyzeDirectory` (`catalog-web/src/services/mediaApi.ts`) sends `directory_path`, the handler reads `path` or `id`; no caller remains after R4-2. Disposition: fix or delete when a scan-from-dashboard feature is built.
- Admin `POST /admin/storage/scan` is 501 by design (use `POST /api/v1/scans`).
- `catalog-web/src/pages/Dashboard.tsx:371` "Files This Week" sums `growthTrends[*].files_added`; UNCONFIRMED that the server returns exactly seven days. Disposition: label from the returned range or request a 7-day window.
- `catalog-web/src/components/collections/CollectionSharing.tsx` embed tab: the iframe points at `/embed/<id>`; UNCONFIRMED that route exists (no server or SPA route was found by name in this review).
- `catalog-web/src/components/collections/ExternalIntegrations.tsx:820,924`: success toasts for local-only add/update state; they say "added locally ... not saved on the server", which is honest. Disposition: remove with the integrations stub UI.
- `catalog-web/src/pages/Playlists.tsx:197`, `AdvancedSearch.tsx:279,285,290`, `SmartCollectionBuilder.tsx:216`: toasts for in-page state changes (form items, presets); the state change is real and visible, so they are accurate.
- `Favorites.tsx:36` export toast follows a real Blob download; `EntityDetail.tsx:60` is `onSuccess` of the download mutation (it reports "started" after the request resolved).
- `catalog-api/handlers/media_handler.go:211` "Watch progress accepted" is honest (`persisted: false` with the reason).
- `catalog-api/handlers/admin_handler.go:365` (backup list) returns `[]` when the backup directory cannot be read; a missing directory means no backups, but a permission error is indistinguishable; left for the next iteration.
- Pre-existing gofmt diffs in `download.go` (indentation near `ResolveSMBIdentity`), `media_entity_handler.go` and its test; not reformatted here.

Authz round-2 additions (landed with fix-authz-gaps):

- R2 (authz): `/admin/image-quality/revalidate` and `/admin/covers/generate-thumbnails` were registered on the plain API group (any authenticated user could trigger the sweeps); they now carry `RequireAdmin` per route. They were NOT moved into `adminGroup`, because that group also installs the CSRF guard and existing bearer-token clients of these two POSTs send no CSRF token; moving them is an operator decision. `TestAdminGate_RealBinary` pins the gate on the real binary; the group's `RequireAdmin` is distinguishable from the handlers' own permission checks only by a `system.configure`-only role (handlers of /admin/config etc. check their own permission).
- R2: `docs/api/openapi.yaml` from main had two unquoted descriptions (conversion cancel/retry) that made the file invalid YAML; quoted them.

## Authz review r3 follow-ups (GO-with-follow-ups, 2026-11-03)

- T1 (test-instrumentation, minor): `catalog-api/admin_gate_binary_test.go` and the existing `binEnv` harness in `seed_admin_binary_test.go` fail under `go test -race` (stdout buffer read while the child's copier writes). `scripts/run-all-tests.sh` runs `-race ./...`, so the root package is red there. Fix: mutex-guarded writer in `binEnv`.
- T2 (nit): comment at `admin_gate_binary_test.go:43-45` wrongly says /admin/health, /logs, /errors have no handler-level check.
- T3 (nit): B1 line above still names the grep test as the guard; it is `TestAdminGate_RealBinary`.
- T4 (latent, minor): `UserHandler.UpdateUserSettings` (user_handler.go ~763-781) full-row `Update` from a stale read; unrouted today; use `UpdateGuarded` or limit to the settings column before routing.
- P2 (source-defect, pre-existing, IMPORTANT, new item): `POST /storage/roots`, `/smb/*`, `POST /scans`, `/smb/probe-and-ingest` (main.go ~1349-1388) have no permission check; registration is open, so any self-registered user can create storage roots, queue scans and probe/ingest SMB hosts. UNCONFIRMED whether a `local` root outside the configured base is accepted. Being fixed with systematic debugging.
- P3 (minor, operator decision): `/metrics` unauthenticated; `/debug/pprof` unauthenticated when `HELIX_PPROF_ENABLED=true`.

## P2 route-permission fix (fix-p2-route-perms, 2026-11-03)

Closed here: P2 (storage/scan/SMB/copy routes with no permission check), T1 (`binEnv` race under `go test -race`), and the local-root confinement hole (a `local` storage root of `/etc` or `/` was accepted: CONFIRMED, now refused with 400). Gate (as first written; superseded on the merged tree, see P2-q): a `storage.manage` permission checked by `JWTMiddleware.RequirePermission`. The merged tree gates these routes with `RequireAdmin` instead, and `RequirePermission` (live database check) now serves only the `system.configure` wizard routes; `POST /challenges/*/run` now needs `RequireAdmin`. Open items found by the class-exhaustive route classification, NOT fixed here (all authenticated-only today; each needs a per-route decision on the intended permission and a check of the catalog-web / Android TV clients first):

- P2-a (CLOSED in review round 1, F5): `GET /analytics/user/:user_id` and `GET /recommendations/personalized/:user_id` took the user id from the path and compared it with nobody (CONFIRMED by the round-1 review: cross-user read, IDOR). Both now sit behind `RequireSelfOrAdmin("user_id")` (403 for another user), pinned by `TestRoutePerms_RealBinary_SelfOrAdmin`. Still open: the recommendations payload itself is fabricated (`getPersonalizedRecommendations` returns up to three generated "Personalized Media N for User M" items, §11.4.289); it should answer an honest 501 until a real implementation exists (P2-k).
- P2-b (partly CLOSED in review round 1, F6): `GET /analytics/system`, `GET /reports/usage` and `GET /reports/performance` are now admin only. Still open: `POST /analytics/reports` (CreateReport) has no permission check; `analytics.view` / `report.view` constants exist but are not enforced.
- P2-c (minor): `POST /entities/enrich`, `POST /entities/:id/metadata/refresh`, `POST /media/:id/refresh`, `POST /media/analyze`, `POST /subtitles/{download,translate,upload}` can start outbound provider calls / CPU work for any authenticated user (resource amplification; the intended permission would be `media.edit` / `media.manage`). `POST /sync/cleanup` was listed here as "possibly global": it IS global (CONFIRMED by the round-1 review: it deletes sessions of every user) and is now admin only (F7); the other `/sync/*` endpoints are per-user, and their mutations are admin only (F3, merged tree) on top of the `pathguard` sync roots.
- P2-d (minor): `GET /scans` and `GET /scans/:job_id` stay auth-only on purpose (the Android TV client polls them); they expose storage root names and current paths of running scans to any user. Decide whether to gate them with `storage.manage` and update the TV client.
- P2-e (minor): `PUT/DELETE /storage/roots/:id` and `POST /storage/roots/:id/test` are called by `catalog-web/src/lib/mediaApi.ts` but are not registered in `main.go` (route drift, confirmed from the route table; the web UI gets 404). Not a security issue, but when they are added they must carry `storageGate`.
- P2-f (CLOSED, merged with main): local roots are re-checked at every use (`filesystem.CheckLocalRoot` in `clientForRoot` and `QueueScan`) and, since the merge, also refuse the database/key files; rows inserted directly into the database are covered by the use-time check.
- P2-g (nit): `GET /smb/discover` and `GET /smb/test` take credentials in the query string (they end up in access logs); the POST forms exist. **PARTLY FIXED (security queue): CONFIRMED the access log wrote `password=` values (the redaction list knew only ticket/token/access_token/api_key); `internal/logredact` now also redacts password, passwd, pass, pwd, secret, client_secret, refresh_token, authorization, apikey, case-insensitive (`TestLogger_CredentialQueryParametersAreRedacted`, three mutants killed). Still open: the GET forms themselves still accept credentials in the URL (browser history, reverse-proxy logs outside our control); removing them needs `scripts/run-full-qa.sh` lines 224-225 changed. Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- P2-h (operator decision, carried from P3): `/metrics`, `/discovery`, `/ws` and (when `HELIX_PPROF_ENABLED=true`) `/debug/pprof/*` are unauthenticated. **UPDATE: `/metrics` and `/debug/pprof/*` now need an administrator (or the scrape token for `/metrics`), see "Authz round 2" below; `/discovery` and `/ws` stay public by design (AZ2-4).**
- P2-i (test-instrumentation): `TestRoutePerms_RealBinary_*` each build and start their own binary (about 13 s each); share one server across them when more routes are added.
- P2-j (process-doc): the route table used for the classification is produced by `python3` over `main.go`; it is not committed. A committed route-classification test (anonymous 401 for every route not on a public allow-list) would catch the next ungated route automatically.

### Review round 1 of fix-p2-route-perms (F1-F11)

Fixed in this round, each with a real-binary test that failed before the change and a paired mutation that the test kills: F1 wizard writes need `system.configure` (`TestRoutePerms_RealBinary_Round1Gates`); F2 local storage roots use `pathguard.ConfineData` (`..._LocalRootRefusesProtectedFiles`); F3 sync endpoint mutations are admin only (first written as `storage.manage`, see P2-q) and the endpoint URL is checked after resolution and dialled pinned (`internal/netguard`, `TestSSRF_RealBinary_SyncEndpointURL`); F4 the image proxy matches the parsed host exactly, https only, redirects only inside the allow-list (`TestSSRF_RealBinary_ImageProxy`, `TestValidateImageProxyURL`, `TestImageRedirectPolicy`); F5 self-or-admin (`TestRoutePerms_RealBinary_SelfOrAdmin`); F6, F7 admin only; F9 the near-miss permissions `storage.managex` and `system.configurex` are refused; F10 docs (`docs/security/PATH_CONFINEMENT.md`, `docs/deployment/ENVIRONMENT_VARIABLES.md`, `docs/api/openapi.yaml` 403/503, `docs/architecture/AUTH_FLOW.md`).

Still open (not fixed here):

- P2-k (CLOSED on main by CAT-001): `GET /recommendations/personalized/:user_id` answers 501 (no recommendation engine, nothing fabricated); this branch only adds the self-or-admin gate (`RequireSelfOrAdmin`).
- P2-l (source-defect): `POST /analytics/reports` has no permission check (see P2-b).
- P2-m (design, carried from F3): `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` is process-wide. A per-endpoint allow (an admin pinning a LAN NAS in the endpoint record) would be narrower. With `proxy.*` configured the image proxy cannot see the resolved address (the proxy resolves it); the exact host allow-list is the only barrier there.
- P2-n (test-instrumentation): the netguard dial test covers loopback by a real listener; link-local and private ranges are covered by address-table tests only, because the CI host has no such listener. A rebinding test (a name that resolves to a public address first and to loopback second) needs an injectable resolver.
- P2-o (F8, CLOSED as solved on main): the scanner follows symlinks, but main re-runs `filesystem.CheckLocalRoot` against `CATALOGIZER_LOCAL_ROOT_BASES` when a root is used (queue scan, client creation), so a directory swapped for an escaping symlink after creation is refused; the resolved path is also stored (`TestRoutePerms_RealBinary_LocalRootConfinement`). `CATALOGIZER_LOCAL_ROOT_BASES` is the single local-root policy; `CATALOGIZER_READ_ROOTS` no longer applies to storage roots.
- P2-q (F-F, note): the storage.manage permission is no longer used as a route gate (storage, scan, smb, copy and sync-endpoint routes are admin-only on the merged tree); the constant remains for future finer-grained gating; the role editor no longer offers it (F-R4), because granting it changes nothing. A holder of storage.manage alone gets 403 on those routes (pinned by the real-binary tests).
- P2-r (F-G, nit): an image-proxy redirect from one allowed host to another allowed host fails the TLS handshake, because the pinned dial keeps the original host's ServerName. It fails safe (the redirect is not followed); making cross-host redirects work would need a per-hop ServerName, and nothing needs it today.
- P2-s (F-H, nit, outside this diff, cause UNCONFIRMED): the built server ignored SIGINT/SIGTERM for more than 10 s with no shutdown log line and needed SIGKILL in the reviewer's run. Probably a blocked shutdown path, possibly pre-existing; not investigated here.
- P2-p (F11, CLOSED here): `UserRepository.livePermissions` (used by `IsAdmin` and `HasPermission`) ignored `is_locked` / `locked_until`, so a locked account kept its gates. It now treats a permanently locked or not-yet-expired lock like a deactivated user, as login does (`TestUserRepository_HasPermission_LockedAccount`).

## Live scaling / stress / chaos suite (2026-11-03, branch `test-scaling-chaos`)

- Added `scripts/test-infra/scaling_live.sh` and `catalog-api/scaling_live_test.go` (tag `live`); guide section "Scaling and chaos (live)" in `docs/TESTING_GUIDE.md`. Measured on a shared host, host-dependent.
- FIXED here (RED first): `FileRepository.SearchFiles` pool deadlock (metadata query while the result cursor was open); beyond 25 concurrent searches the API stalled until clients timed out (32 clients: 19 x 200 in 10 s, p99 about 29.9 s). Test: `repository/file_repository_pool_test.go`.
- S1 (open, UNCONFIRMED): RSS grew from 146 to 257 MiB across 9 load rounds (about 2 minutes of cumulative 8/32/64-client load is not enough to tell warm-up from a leak) and the module monitor logged `MEMORY ALERT: potential leak detected` once at 64 clients. Needs a 30 to 60 minute soak with heap profiles (`HELIX_PPROF_ENABLED=true`, `/debug/pprof/heap`); suspects to check first: the per-IP sliding-window rate limiter (`internal/auth/middleware.go`, `RateLimitByUser` rebuilds a timestamp slice per request and never evicts idle keys) and gin/zap request logging buffers.
- S2 (open): `getFileMetadata` is still an N+1 query per result row (50 queries per `search/files` page). The pool deadlock is gone, but a single batched `WHERE file_id IN (...)` query would cut per-request latency and pool pressure; `GetFileByID` is unaffected.
- S3 (open): `RateLimitByUser` keys on client IP (the user branch is never taken because the JWT middleware stores `user_id` under a different context key than `GetCurrentUser` reads), so all users behind one NAT share 2000 requests/min and 30 logins/min. The live suite works around it with distinct loopback source IPs and documents it.
- S4 (open, by design today): when the DB is starved the session checker answers 401 after 3 s (fail closed). Operators see 401 instead of 503; consider 503 + `Retry-After` for a checker timeout (decision pending).
- S5: `/debug/pprof/*` is registered on the public router when `HELIX_PPROF_ENABLED=true` (the suite relies on it for goroutine counts); confirm it is never enabled in a published configuration.
- Not covered by the live suite: write-heavy load, SMB/WebDAV/NFS backends, Postgres, TLS listener, more than 64 clients, multi-instance on one DB (F6).

## Scaling/chaos review r1 follow-ups (GO-with-follow-ups, 2026-11-03)

Source fix (SearchFiles pool deadlock) confirmed correct (RED reproduced, semantics unchanged). Findings on the suite and class:
- F1 (test-instrumentation, important): `evaluateRound` never flags 401/403/404; a round of 90% 401 passes (the pre-fix session-checker symptom). Bound non-200/non-429 share or fail on any 401/403 after login.
- F2 (test-instrumentation, important): a `go build` failure becomes SKIP (exit 77) in `scaling_live_test.go:93`; `scaling_live.sh:46` exits 0 on partial pass+skip. Build failure must FAIL; partial run needs a distinct exit code.
- F3 (minor): stress/chaos/sigterm evidence written only after assertions pass; `exe_sha256` never asserted equal to `buildInfo.sha256`.
- F4 (minor): 5xx check unit-tested with "500" only (mutation 503 survives).
- F5 (source-defect, minor, same class as the fix): nested pooled query while an outer cursor is open, same deadlock risk under concurrency: `handlers/admin_handler.go:293` GetStorageInfo; `internal/services/catalog.go:443` GetDuplicateGroups; `repository/duplicate_entity_repository.go:60` GetDuplicateGroups; `internal/services/aggregation_service.go:860` getTopLevelDirectories; `internal/services/playlist_service.go:274` GetUserPlaylists (getPlaylistTags/getPlaylistCollaborators at 757/775); `internal/media/realtime/watcher.go:455` checkMediaItemIntegrity. Mutation re-introducing the pattern in `GetDirectoryContents` survived (pool test covers SearchFiles only). Needs a pool test per member plus a class-level guard.
- F6 (minor): SIGKILL integrity oracle has no golden-bad (corrupt DB) control.
- F7 (source-defect, minor): `services/conversion_proc_unix.go:16` converter children have Setpgid but no Pdeathsig; after a server SIGKILL a running `convert` can orphan and race the requeued job on the same output file.
- F8 (nit): about 424 MB left under `.scaling-live-artifacts/` per run, no rotation. F9 (nit): factor 25 and RSS bounds are chosen, not measured (disclosed); `go build` runs on the bare host, not in a container (11.4.173 exception for local containerised builds not applied to test builds).

## CAT-001 review R5 (2026-11-03, extra operator-approved round)

Fixed (RED test first, mutation killed): R5-1 CollectionSharing (no `/collections/:id/share*` routes exist on the server): Create Share Link, Send Invitations and Copy Embed Code are disabled with a stated reason, no URL is invented, the embed iframe for the non-existent `/embed/<id>` route is no longer shown, and the (currently unreachable) create/revoke/delete/email handlers now treat anything but `response.ok` as failure; R5-2 smart playlists: `POST /playlists` answers 501 for `is_smart` / `smart_rules` (nothing stored) and the Smart Builder's Save is disabled with the reason; R5-3 `scripts/run-full-qa.sh`, `scripts/run-helixqa.sh` and `docker-compose.qa.yml` now expect 501 for `/configuration/status` and `/recommendations/trending` (run-helixqa also drops `curl -f`, which appended "000" to the status); R5-4 template "Create Collection" is disabled with a reason (templates are smart collections, always 501); R5-5 "Memory Usage" is relabelled "API Process Memory (heap in use / reserved)" in Dashboard and AdminPanel (the figure is the Go heap ratio, not host memory); R5-6 openapi: 501 on PUT `/collections/{id}`, 500 on GET `/admin/storage`, 501 on POST `/playlists`; `useFavorites`: "Added to favorites" is now toasted in `onSuccess` (and the optimistic updater no longer wipes the cache on add).

Remaining, listed for the next iteration:
- R5-7 `GET /auth/init-status` (`stub_handler.go:631`, `MediaQueryHandler.GetInitStatus`) always answers `{"initialized": true}` without looking at the database. Disposition: derive from the user count or document it as a constant.
- `CollectionSharing` handlers (create/revoke/delete/email) are strict but unreachable behind `SHARE_API_AVAILABLE = false`; no test drives them until the server grows the routes. Same for `TEMPLATE_APPLY_AVAILABLE`.
- `localization_handlers.go:423` constant `confidence: 1.0`, Collections.tsx:411-453 AI toasts, `mediaApi.analyzeDirectory` payload: unchanged from the R4 inventory above.

## CAT-001 verification round (R6, GO-with-follow-ups)
- F1 (minor): other QA consumers still expect 200 from endpoints that now answer an honest 501: scripts/run-full-qa.sh:207 (recommendations/personalized/1), scripts/testing/full_automation/catalog_collections_downloads_stats.sh:384-386, challenges/helixqa-banks/catalogizer-api-comprehensive-executable.yaml:387 and :1023, tests/test_recommendations.sh:29-35, scripts/run-helixqa-web.sh:153-159; close the whole class at once.
- F2 (nit): docker-compose.qa.yml and run-helixqa.sh accept 200-399 for configuration/status and trending, so a regression to an invented 200 would pass.
- F3 (process-doc): the R5 note that useFavorites "no longer wipes the cache on add" is wrong; tanstack query-core 4.41 setQueryData ignores an undefined updater result, so no wipe existed. The change is harmless.
- F4 (nit): gofmt turned SQL '' into a typographic quote in comments at catalog-api/handlers/media_entity_handler.go:913 and media_entity_handler_test.go:470; rephrase.
- F5 (minor): strict response.ok handlers in CollectionSharing and the template apply path are unreachable behind SHARE_API_AVAILABLE / TEMPLATE_APPLY_AVAILABLE=false and are untested.
- F6 (nit): run-helixqa.sh without curl -f reports status 000000 on connection failure (still FAIL).
### S1 resolution (memory soak, 2026-11-03, branch `fix-s1-leak`)

- S1 is NOT an unbounded Go-heap leak. Reproduced with `TestLiveSoak` (new, `catalog-api/scaling_soak_live_test.go`): 30 identical rounds at 32 clients, RSS 183 -> 298 MiB (+115) while Go `HeapSys` stayed 44 -> 52 MiB and post-GC heap objects stayed about 24k. Cause CONFIRMED by smaps (54 glibc malloc arenas, about 190 MiB anonymous) and by the control `MALLOC_ARENA_MAX=2` (+12 MiB over 16 rounds, one arena). The earlier suspect (rate limiter) is REFUTED as the cause of the ramp (the 4-IP `fewips` run ramps the same way) but is a REAL separate defect (below).
- Fixed: arena cap applied in a load-time C constructor (`main_malloc_glibc.go`; mallopt from Go `init()` was measured ineffective, still 54 arenas); operator `MALLOC_ARENA_MAX` wins. Test `TestMallocArenaCapAppliedAtLoad` (RED when the mallopt call is disabled: `applied=0`). Live result after the fix: 70 rounds plateau at about 170 MiB (slope about 0 over rounds 51-70).
- Fixed: `RateLimitByUser` retained one entry per distinct client forever (real, bounded only by client cardinality; about 6 heap objects per client measured). New `internal/auth/ratelimit.go` sweeps idle keys at most once per window. Tests (RED first, 2 of 3 failed with 5001 / 2001 retained keys): `TestSlidingWindowLimiter_EvictsIdleKeys`, `..._EvictionKeepsLiveKeysLimited`, `TestRateLimitByUser_BoundedAcrossManyClients`; mutations run: disabling the sweep fails all three, evicting live keys fails the keeps-live test.
- Fixed (monitor false positive): the `MEMORY ALERT` fired on the goroutine criterion (>1.5x start-up baseline) with heap ratio 1.45 against a 10x threshold. Goroutines: 29 to 30 idle, 207 to 256 peak during 64 clients (measured mid-round), back to 29. The callback now logs `MEMORY NOTICE` at info level with a `trigger` field for goroutine-only flags, and `MEMORY ALERT` only for the heap criterion (`classifyMemoryAlert`, test `TestClassifyMemoryAlert`). Start-up log said threshold 3x; it is 10x (fixed).
- Open: the Go side of the monitor still reads `GetReport()` from the third-party-style `digital.vasic.memory` module (unchanged, submodule untouched); a goroutine criterion relative to a start-up baseline is structurally load-correlated, consider a request-concurrency-aware threshold upstream. Musl/non-cgo builds have no arena cap (no-op); Alpine/musl images were not measured. A 30 to 60 minute soak with write paths and non-search endpoints has not been run (70 x 6 s rounds only).

## S1 memory fix review (round 1, GO-with-follow-ups)
- F1 (minor): no concurrent test for the rate limiter sweep dead-entry protocol (reviewer mutations M4/M5/M9 survive -race); add a parallel allow+sweep test asserting exact in-window counts.
- F2 (minor, docs): arena counts in TESTING_GUIDE/FOLLOW_UPS ("54 before / 6 after") are not reproducible; reviewer counted 44-46 before and 1 after (64 MiB-aligned mappings); restate with the counting command (scratchpad arenas.py approach).
- F3 (nit, docs): 70-round row "162 -> 167" should read about 165.6 -> 171.1 MiB.
- F4 (minor): GLIBC_TUNABLES=glibc.malloc.arena_max is silently overridden by mallopt; skip when set or document.
- F5 (nit): "no-op on musl" holds only with the custom musl tag; the build requires glibc (bionic/musl cgo builds would not compile).
- F6 (minor): goroutine-only trigger demotion cannot tell load-correlated counts from a real goroutine leak at idle; add a sustained-growth-at-idle signal.
- F7 (minor): sweep is O(n) in the triggering request once per window; lastSweep uses wall clock (backward step delays sweeps).
- F8/F9 (nit): arena test proves constructor ran, not the arena count; test-only limiter field on production struct.
- UNCONFIRMED: p99 latency effect of M_ARENA_MAX=2 under 64 clients (not measured).
## Pool-deadlock class fix + scaling-suite findings (2026-11-03, branch `fix-pool-class`)

Closes F1, F2, F4, F5, F6, F7 of the scaling/chaos review (details in `docs/TESTING_GUIDE.md`, "Pool-deadlock class guard" and "Scaling and chaos (live)").
- F5 members (RED first, each fixed by buffering the page and closing the cursor before the nested queries): `AdminHandler.GetStorageInfo`, `CatalogService.GetDuplicateGroups`, `DuplicateEntityRepository.GetDuplicateGroups`, `AggregationService.getTopLevelDirectories`, `PlaylistService.GetUserPlaylists` (pool deadlock RED); `SMBChangeWatcher.checkMediaItemIntegrity` and `ProcessPendingChanges` (no pool deadlock: the media DB has no connection cap; RED is a lost write after the busy timeout in the production rollback-journal mode). `GetDirectoryContents` has no nested query (regression test added, green). `RefreshSmartPlaylist` is single-tx (allowlisted).
- Class guard: `internal/poolguard` (AST scan + control needles + allowlist-staleness test).
- F7: `Pdeathsig=SIGKILL` on Linux for converter children (`services/conversion_proc_pdeathsig_linux.go`), RED/GREEN by SIGKILLing a real re-executed server. Limit: direct child only; helpers forked by the converter are not signalled.
- Still open (new): R1 `internal/services.PlaylistService` needs `is_smart_playlist`, tags and collaborators tables that no production migration creates (only test DDL), so `GetUserPlaylists` fails on a production schema; unclear whether the service is wired in production. R2 the media DB embedded schema has no `change_log` table (only a `schema.sql` beside the DB creates it), so `SMBChangeWatcher.logChange` fails without that file. R3 `ProcessPendingChanges` used the invalid SQLite literals `false`/`true` and so always errored; changed to `0`/`1`; it now succeeds but `processChange` re-logs each change as a new unprocessed `change_log` row (self-feeding queue); it has no production caller. R4 `getTopLevelDirectories` is only reachable from the legacy aggregation path kept as a regression-guard RED leg. R5 `SearchFiles` N+1 `getFileMetadata` (S2) left as is (sqlmock unit tests pin the per-row query). R6 `database.DB.Query`/`QueryRow` use `context.Background()`, so a pool deadlock on those paths never times out. R7 `scalinglive` integrity oracle opens the DB through the production opener, which also runs migrations (it can mutate the file it checks).

## Pool-class fix review (round 1, GO-with-follow-ups)
- F1 (minor): internal/poolguard Scan misses five shapes (cross-package helper, interface call, method value/func var, BeginTx inside loop, compound `rows.Next() && ...` condition); ScanCrossPackage covers the first two. Document limits in TESTING_GUIDE.
- F2 (minor): four guard mutations survive (for{if !rows.Next(){break}} branch, depth 3->1, removing Prepare/PrepareContext/InsertReturningID, file-wide allowlist key).
- F3 (minor): sigkillIntegrityOK open/query/scan/rows.Err error paths turned into return true survive; golden-bad covers only non-ok rows.
- F4 (nit): scripts/test-infra/scaling_live.sh:31 `--classify log abc` fails open (exit 0).
- F5 (minor, pre-existing): rows.Err() unchecked after buffered loops and scan errors `continue` (catalog.go ~457, playlist_service.go ~291, admin_handler.go ~309, watcher.go) so a truncated cursor silently returns partial results.
- F6 (nit): Pdeathsig thread-binding safe only while no dependency exits a goroutine holding LockOSThread; unguarded.
- F7 (UNCONFIRMED, pre-existing): under lib/pq RefreshSmartPlaylist runs tx.ExecContext with a tx cursor open; not run on Postgres.
- F8 (nit): RED signature in TESTING_GUIDE is "12 of 12"; aggregation/admin/playlist show "11 of 12".
- HOST: root filesystem reached 100% during review (go-build cache 63 GB); cleared with `go clean -cache` (63 GB free afterwards). Needs a periodic cache cap.

## Smart collections and smart playlists (feat-smart-collections)
- Implemented: closed rule language `internal/smartrules` (parameterised SQL only), evaluation at read time, `GET /collections/:id/items`, smart playlists, owner/admin authz, migration 23. Guide: docs/guides/SMART_COLLECTIONS_GUIDE.md.
- UNVERIFIED (no `node_modules` in the scratch clone or the live checkout, so vitest and `tsc --noEmit` were not run): the catalog-web edits (Playlists.tsx smart save, SmartPlaylistBuilder.tsx, CollectionTemplates.tsx text, the three updated/added test files).
- Postgres: tested against a real postgres:15-alpine (rootless podman) by `internal/smartrules/smartrules_pg_test.go` (edge cases and a field x operator parity matrix); see docs/SMART_COLLECTION_RULES.md. The migration `ADD COLUMN IF NOT EXISTS` itself is exercised by `RunMigrations` in that run.
- Open: collection templates (CollectionTemplates.tsx) use fields and values the rule language rejects (category, ai_score, trending_score, user_preference_match, `30_days_ago`, media type `audio`, `is_not_empty`); applying stays disabled with a reason until they are rewritten.
- Open: the Collections page does not show smart items yet (the page lists collections; the items endpoint exists and `collectionsApi.getCollectionItems` already targets it).
- Open: `image` media type matches nothing (the catalogue has no image media type); `tags` supports only contains / not_contains (comma-separated text column).
- Per-user caps: 50 smart collections (`MaxSmartCollectionsPerUser`) and 50 smart playlists (`MaxSmartPlaylistsPerUser`); each read evaluates the rules (bounded by the 500-item page cap, list count cap 1000 and rule limits, no timing benchmark taken).
## Live Yandex WebDAV suite (2026-11-03)
- FIXED (live RED first): `WebDAVClient` used `http.Client{Timeout: 30s}`, a whole-request cap that also covers the body. The pre-fix client (commit 05709505) failed the 1 MiB upload of step 04 against the real service with `Client.Timeout exceeded` (committed RED: `docs/qa/webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log`, `RED_pre_fix_client_run1.json`; the service accepts about 16 KiB/s); every large `ReadFile` stream (video through the stream handler) would be cut at 30 s the same way. Now: metadata operations 30 s as a whole; transfers bounded by the caller's context, a 60 s idle timeout and a 10 min wait for the answer after the last upload byte. Unit guards `webdav_transfer_timeout_test.go`; live guard `TestLiveYandex_Suite/04_upload_known_sha256`.
- FIXED (found while wiring the suite): `scripts/test-infra/webdav_live.sh` redacted its output with `sed -e s/<password>/...` (the password and user on a command line, readable in `/proc/<pid>/cmdline`); now `awk` reads them from the environment.
- OPEN W1: `WebDAVClient` has no MOVE (rename); callers must COPY then DELETE (not atomic, doubles traffic on a throttled service). The live suite probes MOVE with a raw guarded request (it works on Yandex).
- OPEN W2: `WebDAVClient` has no ranged read and does not implement `SeekableClient`, so the stream handler cannot seek in a WebDAV video; the service supports `Range` (see the `07_range_partial_read` note in the evidence).
- OPEN W3: `FileExists` returns `(false, nil)` for every non-200 answer, including 401, 403 and 5xx, so an auth or server error reads as "file absent". 404 should be false and any other non-2xx an error. Pinned by existing unit tests, not changed here.
- OPEN W4: `Connect` accepts a plain `200` as success, so a non-WebDAV HTTP server passes the connect check (the older live test treats that as not-a-WebDAV-service).
- FIXED W5 (SECQ-1, 2026-11-05): WebDAV storage roots now use the operator-class outbound policy `netguard.OperatorDialContext` / `OperatorTransport` / `OperatorCheckRedirect` / `CheckOperatorURL` (`catalog-api/internal/netguard/operator.go`, extends the shared netguard, no fork). Refused on the RESOLVED address at every dial (DNS rebinding, redirect hops, pinned IP; a mixed DNS answer is filtered: refused addresses are dropped and only permitted ones dialled on the direct path; with an environment proxy the name is forwarded, so the proxy path stays STRICT and refuses any mixed answer, SECQ1-FU): loopback, link-local incl. 169.254.169.254 and fe80::/10, unspecified, multicast, broadcast, reserved ranges, 0/8, 240/4, `fd00:ec2::254`, `fd20:ce::254`, `100.100.100.200`, and the IPv4-mapped / IPv4-compatible / NAT64 / 6to4 forms of those. Allowed by default: RFC 1918, ULA, CGNAT (Tailscale NAS) and public addresses (Synology LAN root, webdav.yandex.ru). Operator controls: `CATALOGIZER_OPERATOR_DENY_PRIVATE=1` also refuses RFC 1918 / ULA / CGNAT (strict opt-in); `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` re-admits named loopback / private / CGNAT networks in both modes and never the always-refused addresses. Wired in `filesystem/webdav_client.go` (storage-root client), `services/webdav_client.go` (sync client incl. `GetQuota`) and early in `POST /storage/roots` (`validateWebDAVRootURL`, HTTP 400). Tests: `internal/netguard/operator_test.go` (classification table, strict mode, allowlist limits, real loopback listener dial, rebinding via resolver, redirect hops), `filesystem/webdav_ssrf_secq1_test.go`, `services/webdav_ssrf_secq1_test.go`, `handlers/storage_root_ssrf_secq1_test.go`. Review round 1 (Opus NO-GO) fixes: with an environment proxy the destination now fails closed (a host that does not resolve locally, or is not a canonical IP literal / DNS name, is never forwarded; inet_aton spellings such as 0177.0.0.1, 2130706433, 127.1, 0x7f.1 and zone suffixes are refused everywhere), while the proxy's own address (operator configuration, e.g. 127.0.0.1) bypasses the destination policy; SIIT (::ffff:0:a.b.c.d) is judged as its embedded IPv4, Teredo 2001::/32 is refused, `::1` and `::` are no longer unwrapped as IPv4-compatible (so `::1/128` allowlisting works); errors no longer echo resolved addresses; GetQuota shares the client transport. Tests: real proxy listener, pinned-dial, request-level `POST /storage/roots`; each load-bearing check has a surviving-mutation-killed run. Residuals: (1) with an environment proxy the proxy resolves the name again, so a rebinding answer between the local check and the proxy's lookup is not covered (run without a proxy where that matters), and a direct connection to the same address as a proxy used earlier by the same transport is not destination-checked; (2) `gowebdav` (sync client) owns its redirect policy, so redirect hops there rely on the dial check alone; (3) a hostname that does not resolve at validation time is accepted by the API and checked again at dial; (4) default mode deliberately allows private ranges, so a root can still be aimed at other RFC 1918 services (use the strict opt-in on hosts where that matters); (5) no live run against a real NAS or Yandex after this change (UNCONFIRMED; the unit/real-listener tests prove the classification and the dial path only).
- OPEN W6: the client has no retry of its own (one `connection reset by peer` on MKCOL and one 30 s PROPFIND timeout were seen); only the scanner wraps reads with `fabric.Retrying`. Writes (PUT, MKCOL, DELETE) are never retried.
- OPEN W7 (service fact, not a defect): the Yandex WebDAV upload rate was about 16 KiB/s, so cataloguing or copying media into it is slow by design; a collection `DELETE` can answer `202` (asynchronous), the suite polls until the directory is gone.
- OPEN W8: the scan leg replaces only the record seam; the database-backed `insertFileRecord` path and the `UniversalScanner` job path (factory-built client with storage-root credentials) were not run against the real service, because the factory-built client has no outbound guard and its `Connect` would issue a PROPFIND on the service root, outside `/Testing`.

- W9 (open): `WebDAVClient.Connect`/`TestConnection` have no retry; the live Yandex service intermittently answers the first PROPFIND with a connection reset or stalls it to the 30 s op deadline. The live test retries (counted in evidence as `transient_retries`); the production client does not (see W6).
- W10 (UNCONFIRMED): one mid-body 60 s idle cancel was seen on a download while the service was degraded; not reproduced since.
- OPEN WD-F1 (UNCONFIRMED cause, possible DATA LOSS): one throwaway probe (output not committed) saw an upload cut by `connection reset by peer`, a retried PUT answer success, and the file stay absent from the directory for at least 55 s, i.e. an acknowledged write may be lost by the service. No committed run shows it (all committed runs have `lost_acked_writes=0`; the live script now FAILs on any). The production `WebDAVClient.WriteFile` has no read-back and no retry, so if the loss is real Catalogizer could silently lose data written to WebDAV. Next step: repeat the probe under evidence capture (status of every attempt, HEAD/PROPFIND after), then decide on read-back verification in the client.
- WD-F1 update (2026-11-05, evidence `docs/qa/wdf1-repro-20261105/`): result UNREPRODUCED. 61 plain live uploads and 12 forced-reset (chaos class, TCP fault proxy in front of the real service) attempts: no acknowledged write was ever absent (control needle proves the verdict fires). `WriteFile` now has a defensive read-back (HEAD, size compare, one replay), labelled defensive hardening, NOT a proven fix. Mode B (reset after the full body) showed a stored file although the client saw an error: callers that retry must tolerate overwrite.
- OPEN WD-F1a (UNCONFIRMED, tracked anomaly): in the chaos1 run one case (mode A, 2 MB, after the retry was acknowledged) was flagged CORRUPT: the file was present with the right size but the read-back sha256 differed; that run did not record whether it was a read error or different content. Not reproduced in 5 repeats (chaos2) nor in the chaos3 rerun. A same-size content difference is NOT detected by the new size-only read-back; follow-up: compare ETag/MD5 or re-read and hash for critical uploads.
- OPEN WD-F2: the idle guard counts consumer inactivity as stall: a `ReadFile` stream the caller pauses for longer than `webdavIdleTimeout` (60 s) is aborted although the server is fine (a paused video player). Not fixed; needs a producer-side stall notion or a documented caller contract.
- OPEN WD-F3: the live tests replace the client transport with a guard transport, so `newWebDAVHTTPClient` settings (dial/TLS timeouts, HTTP/2, pooling) are NOT exercised against the real service; only the default-client shape is unit-tested.
- NOTE W5 is the missing outbound (SSRF/netguard) guard for the WebDAV storage-root path; `live_webdav_scan_test.go` cites it.

## Registration approval + converter tools in the API image (2026-11-03, branch `feat-registration-approval-ffmpeg`)

- Registration is now OPEN WITH ADMIN APPROVAL (`REGISTRATION_MODE`: `approval` default, `open`, `disabled`). Real-binary test `catalog-api/registration_approval_binary_test.go` (RED first: register answered an active user plus a session on login). Behaviour change documented in `docs/UPGRADE_NOTES.md`. This also narrows P2 above (a self-registered user can no longer act before an admin approves); P2 itself (no permission check on storage roots / SMB / scans) is NOT fixed by this change.
- Pending marker lives in the user's settings JSON (`registration_status: pending`), no migration; a user-settings write that replaces the whole settings string would drop it (the user then shows as `inactive`, still unable to log in). A proper `users.approval_status` column is an option if this proves fragile.
- Web: AdminPanel shows `pending` users with Approve / Reject (calls `PUT /admin/users/:id`); the register toast says approval is needed. NOT RUN: `catalog-web` has no `node_modules` in this clone, so `tsc` and vitest were not executed for the TypeScript/React edits (one test added in `AuthContext.integration.test.tsx`).
- Converter tools: the runtime images now ship `ffmpeg`, `imagemagick` (and already `poppler-utils`); `/health/deep` carries `converters` from real PATH lookups; a missing tool fails the job with `converter tool "x" is not installed on this server`. Reproduced first: with PATH lacking ffmpeg on SQLite the job already ended `failed` (about 2 s) with the bare exec error, it did NOT stay pending. The QA-stack jobs that stayed `pending` (Postgres, image built from main at f28f8165) are therefore UNCONFIRMED as a missing-tool symptom; the cause of that pending state was not reproduced (Postgres not exercised here). Worth a follow-up: run the same job against the Postgres QA stack.
- Image evidence (see the hand-back): `Dockerfile.runtime` built with rootless podman, 761 MB (previous API image about 280-320 MB): `libLLVM` and Mesa come in through ffmpeg dependencies. A smaller image would need a static ffmpeg or a slimmer build; operator decision.
- ImageMagick 7 on trixie ships a `convert` entry point (checked in the image); its default policy may still block PDF input (no ghostscript installed), PDF rasterising uses poppler first.
- Build note: the host disk was at 100% during the image build (builder-stage compile and the ffmpeg layer failed with ENOSPC twice); the verification build used a throwaway rootless podman store on tmpfs. `catalog-api/Dockerfile` itself (multi-stage) built its builder stage successfully with the same apt change in the runtime stage, but the runtime stage of that file was verified through `Dockerfile.runtime` (same apt line), not end to end.


## Registration approval + ffmpeg image review (round 1, GO-with-follow-ups)
- F1 (IMPORTANT, source-defect, outside the diff, to be tackled before the release tag): ffmpeg/convert run on user-controlled files with no hardening (conversion_service.go:702-712, :661-689): no -nostdin, -protocol_whitelist file, demuxer restriction, prlimit caps, or ImageMagick policy.xml; shipping the tools in the image makes this reachable in containers. UNCONFIRMED whether a crafted HLS/concat file can read local files or make outbound requests.
- F2: PDF rasterising via ImageMagick needs ghostscript, absent in the image (`gs: not found`) while /health/deep says convert=true and docs claim PDF fallback; add ghostscript (and check IM policy) or remove the claim.
- F3: no test for the "tool present" side of the converter probe (always-false mutation survives); add a stub ffmpeg on a temp PATH.
- F4: approving via PUT /users/:id {is_active:true} does not clear the pending marker; a later deactivation shows the user as pending again.
- F5: admin PUT /admin/users/:id does not validate `status` (bogus clears the pending marker).
- F6: QA consumers still expect instant-active registration (comprehensive bank api-flow-login-registered-user, tests/test_subtitles.sh, UF-API-AUTH-* challenge, run-full-qa.sh:83); set REGISTRATION_MODE=open on the QA stack or add an approve step.
- F7: no audit record for registration/approval/rejection.
- Nits: pending list filtered inside List(1000,0); registration response echoes the raw settings marker; /health/deep unauthenticated discloses installed tools; auth_handler.go imports unsorted.
- Operator decision: API image grew ~300 MB -> 761 MB (ffmpeg deps libLLVM/Mesa); accept, use a static ffmpeg, or slim. catalog-web changes UNVERIFIED (no node_modules).

## Converter hardening + registration follow-ups (2026-11-03, branch `fix-converter-hardening`)
Fixed (RED first, real binaries; see the commit message for the evidence list):
- F1: ffmpeg runs with `-nostdin`, `-protocol_whitelist file,pipe` and a demuxer allowlist (`-format_whitelist`, so HLS/concat/dash playlists are refused); ImageMagick runs under an embedded `policy.xml` (`MAGICK_CONFIGURE_PATH`, dangerous coders and delegates denied, resource limits), the coder is forced from the file extension, SVG is sanitised and rendered from a private copy; every tool is wrapped in `prlimit` (as, cpu, fsize, nofile, core=0) with a hard timeout and a process-group kill that refuses pgid <= 1.
- F2: ghostscript is deliberately NOT added (the image is 761 MB; poppler pdftoppm rasterises PDF pages since D3, go-fitz was removed). The ImageMagick PDF fallback now returns a specific "needs ghostscript" error and `/health/deep` reports `convert_pdf` only when `convert` and `gs` are both present. Dockerfile comments and the deployment guide say so.
- F3: `ProbeConverters` is tested on both sides with stub executables on a temp PATH.
- F4: `PUT /users/:id` with `is_active` clears the pending marker. F5: admin `PUT /admin/users/:id` rejects a `status` outside active/inactive/suspended with 400 before touching the marker (`suspended` is stored as inactive, as before).
- F7: `registration_created`, `registration_approved`, `registration_rejected`, `admin_user_status_changed` go to `auth_audit_log` through `AuthService.RecordAuditEvent` (no login name, e-mail or password in details).
- N1 gofmt of auth_handler.go imports; N2 the registration response no longer echoes the settings marker; N3 decided: `/health/deep` stays boolean-only and unauthenticated (names which tools exist, nothing else); documented here.
- F6 (partial): `docker-compose.test.yml` sets `REGISTRATION_MODE=open` by default; QA note added to `docs/guides/TESTING_GUIDE.md`.

Still open / UNCONFIRMED:
- F6: the comprehensive bank (`api-flow-login-registered-user`) and the `UF-API-AUTH-*` challenge were NOT inspected or changed; they need `REGISTRATION_MODE=open` or an approve step on whichever stack runs them.
- N4: the pending-users list is still filtered inside `List(1000,0)` (users beyond the first 1000 are invisible to the filter).
- ffmpeg network-protocol reachability (http/tcp) could not be reproduced on this ffmpeg build (the default whitelist already blocks it): the protocol allowlist is defensive, UNCONFIRMED as a fix for a reproduced hole.
- This host's ghostscript is broken (`convert pdf:x.pdf` fails with `/undefinedfilename`), so the live ImageMagick-PDF test SKIPs here with a reason; it did not run on a working gs.
- `prlimit --as` uses a host-adaptive value; its fit for very large legitimate media is UNCONFIRMED (no large-file run).
- Image size decision (761 MB) and catalog-web changes remain as listed in the review round above.

Review round 1 of this branch (NO-GO at 1586248e), addressed in the following commit:
- F1 fixed: `checkSVGBytes` allows only unprefixed attributes and the svg, xlink and xml namespaces (plus xmlns declarations); `sodipodi:absref`, `inkscape:*` and vendor attributes are refused (`TestHardenLive_SVGForeignNamespaceAttributeRefused`). Cost: SVGs saved by Inkscape with `sodipodi:*` metadata are now refused; they must be exported as plain SVG.
- F2 fixed: `imageMagickPolicyDir` verifies file and bytes before EVERY run, rewrites it (fresh directory if needed) and refuses the job when that fails (`TestHardenLive_PolicyRestoredBeforeEveryRun`). The Dockerfiles never installed a policy file (it is embedded and applied through MAGICK_CONFIGURE_PATH); their comments say "built-in policy", and the `ImageMagickPolicy` doc comment was corrected.
- F3 fixed: `CATALOGIZER_CONVERSION_MAX_OUTPUT_BYTES` documented in `docs/ENV_VARIABLES.md` (12b).
- F4 fixed: env-scrub assertion test (`TestHardenLive_ChildEnvironmentScrubbed`) and the `PUT /users/:id` audit-event test (`TestApproval_F4_UserUpdateRecordsAuditEvent`).
- F5 (partial): `/health/deep` now reports `convert_svg` (true only with convert AND inkscape or rsvg-convert on PATH). STILL OPEN: the trixie-slim runtime image installs no SVG renderer, so SVG input fails there while `GetSupportedFormats` still lists `svg` as an input; the list is static. Either install `librsvg2-bin` in `Dockerfile`/`Dockerfile.runtime` (UNCONFIRMED that ImageMagick then uses it under the policy) or filter `svg` out of the list when `convert_svg` is false. The golden-FALSE SVG test passes only because Inkscape is on this host.
- F6 residual: the PDF path through ImageMagick needs ghostscript, which is deliberately not in the image (poppler rasterises since D3); `convert_pdf` is false there and the ImageMagick PDF fallback returns a specific error. Not re-verified on a working gs.

Review round 2 (NO-GO at 43d99475), addressed in the following commit:
- N1/N2/N3 fixed structurally: `sanitizeSVG` re-serialises the SVG from the parsed tree (only SVG-namespace elements, canonical xmlns/xlink declarations, unprefixed/svg/xlink/xml attributes, xlink limited to href/title/role/arcrole/show/actuate/type, no comments/PIs/DOCTYPE/foreign subtrees, attribute values that are absolute paths or URLs refused outside hrefs, duplicate attributes refused, root must be `svg` in the SVG namespace, non-whitespace text before/after the root refused) and the renderer gets only those bytes through the forced `svg:` coder from a private copy.
- N4/N5 adopted as tests (`TestHardenLive_SVGStyleImportOutsideFileRefused`, `TestHardenLive_SVGRenderedFromForcedCoderPrivateCopy`).
- N6 OPEN: the ImageMagick policy directory check compares bytes but a symlink at policy.xml pointing at a file with identical bytes is accepted (the target could be swapped afterwards). The directory is private (0700, created by MkdirTemp), so only the same uid can plant it; Lstat + regular-file check would close it.
- Census limits (UNCONFIRMED): the re-serialised SVG was exercised against Inkscape on this host only; other delegates (rsvg, MSVG) were not run. Entity references other than the five predefined ones are emitted as literal text (escaped), never expanded.

## Docs sync C4 (2026-11-03) - remaining documentation gaps
Found by `scripts/docs/check_doc_coverage.py` (guide `docs/scripts/check_doc_coverage.md`, evidence `docs/qa/docs_sync_c4.md`). Status: OPEN until a commit on `main` closes the item.
- D1: 90 top-level scripts have no `docs/scripts/<name>.md` guide (constitution 11.4.18). They are listed with a reason in `scripts/docs/doc_coverage_exempt.tsv`; write each guide from the script's own behaviour (do not infer) and delete its line. Paths: `scripts/api-down.sh` `scripts/api-up.sh` `scripts/audit-lazy-init.sh` `scripts/audit-semaphores.sh` `scripts/build-all-releases.sh` `scripts/build-test-release.sh` `scripts/ci-local.sh` `scripts/ci-pipeline.sh` `scripts/complete-remaining-tasks.sh` `scripts/deploy-vision-hosts.sh` `scripts/deploy.sh` `scripts/detect-landmines.sh` `scripts/devconnect.sh` `scripts/distributed-boot.sh` `scripts/full-distribute.sh` `scripts/gosec-scan.sh` `scripts/helixqa-orchestrator.sh` `scripts/install-security-tools.sh` `scripts/install-tauri-deps.sh` `scripts/install.sh` `scripts/install_git_hooks.sh` `scripts/local-ci.sh` `scripts/memory-leak-check.sh` `scripts/meta_test_constitution_inheritance.sh` `scripts/mutation_sqlcipher_encryption.sh` `scripts/nancy-scan.sh` `scripts/normalize_submodule_pointers.sh` `scripts/performance-test.sh` `scripts/postgres-down.sh` `scripts/postgres-up.sh` `scripts/pre-flight-check.sh` `scripts/prepare-build-host.sh` `scripts/push_all_submodules.sh` `scripts/python-min-version` `scripts/quick-go-test.sh` `scripts/quick-security-scan.sh` `scripts/quick-start-phase0.sh` `scripts/release-build.sh` `scripts/run-all-platform-tests.sh` `scripts/run-all-security-scans.sh` `scripts/run-all-tests-comprehensive.sh` `scripts/run-all-tests.sh` `scripts/run-android-tests.sh` `scripts/run-api-validation.sh` `scripts/run-desktop-tests.sh` `scripts/run-full-qa.sh` `scripts/run-go-tests.sh` `scripts/run-helixqa-all.sh` `scripts/run-helixqa-android.sh` `scripts/run-helixqa-api.sh` `scripts/run-helixqa-desktop.sh` `scripts/run-helixqa-tests.sh` `scripts/run-helixqa-web.sh` `scripts/run-helixqa.sh` `scripts/run-module-challenges.sh` `scripts/run-race-detector.sh` `scripts/run-security-scan.sh` `scripts/run-sonarqube-scan.sh` `scripts/run-stress-tests.sh` `scripts/safe_submodule_init.sh` `scripts/scan-gosec.sh` `scripts/scan-nancy.sh` `scripts/scan-trivy.sh` `scripts/security-gates.sh` `scripts/security-scan-all.sh` `scripts/security-scan-comprehensive.sh` `scripts/security-scan-full.sh` `scripts/security-scan.sh` `scripts/security-test.sh` `scripts/services-down.sh` `scripts/services-status.sh` `scripts/services-up.sh` `scripts/setup-android-testing.sh` `scripts/setup-android.sh` `scripts/setup-desktop-testing.sh` `scripts/setup-freemium-tokens.sh` `scripts/setup-go-testing.sh` `scripts/setup-implementation.sh` `scripts/setup-security-scanning.sh` `scripts/setup-submodule.sh` `scripts/snyk-scan.sh` `scripts/sonarqube-scan.sh` `scripts/track-progress.sh` `scripts/validate-coverage.sh` `scripts/verify-freemium-setup.sh` `scripts/verify-security-infra.sh` `scripts/verify_constitution_inheritance.sh` `scripts/web-down.sh` `scripts/web-up.sh` `scripts/wire-submodules.sh`.
- D2: FIXED (2026-11-03, live contract sweep): `scripts/lib/gin_routes_ast.go` now follows `pkg.F(group, ...)` / `x.M(group, ...)` registrations into the handler package directories and `docs/api/openapi.yaml` no longer has duplicate `403` keys (it was invalid YAML, checker rc=2). Result: 248 router operations == 248 spec operations.
- D3: 13 env names are exempt, not documented: `DOMAIN`, `PASSWORD`, `TYPE`, `USERNAME` (suffixes of `CATALOGIZER_IDENTITY_<N>_*`, documented as that pattern) and the test-helper names `FTP_TEST_*`, `TI_*`, `WEBDAV_TEST_*` (`catalog-api/tests/infra_helper.go`).
- D4: go-fitz (AGPL) removal: DONE by the D3 change below (PROVISIONAL-DONE).
- D5: the other docs that describe `/health/deep` outside `docs/api/openapi.yaml` (for example `docs/deployment/MONITORING_GUIDE.md`, `docs/api/API_DOCUMENTATION.md`) were not re-checked against the `components` / `converters` response shape; only the OpenAPI schema was corrected.

## Docs sync C4 review (round 1, GO-with-follow-ups)
- F1 fixed in the merge: INFRA_PROVISION_ENABLED row now describes runtime infrastructure provisioning, not a test provisioner.
- F2: openapi components description claims "database, Redis, storage, ..." but only `database` is registered (health.go:76); the endpoint description above it overclaims too.
- F3: the 100 ms timeout branch of /health/deep (main.go:1112-1125) answers 200 with a different shape (`time`, `message`, no uptime/converters); schema does not document it.
- F4: check_doc_coverage.py ENV_CALL misses names read through Go constants (e.g. CATALOGIZER_PDFTEXT_MAX_TEXT, services/pdf_text_isolation.go:29) and nested-paren arguments.
- F5: counts in docs/qa/docs_sync_c4.md, docs/scripts/check_doc_coverage.md and D3: env exemptions are 14 (4 + 10 test names), env fixes 37, total exempt 104; the "104 = 103 + 1 broken link" line is wrong.
- F6: checker guide Limits overstates PHANTOM guarantee for routes registered through an unfollowed path and absent from the spec.
- F7 (UNCONFIRMED): DOCKER_SETUP.md claims PDF rasterising via imagemagick; ghostscript absent (see registration review F2).
- N1-N5: MALLOC_ARENA_MAX condition (cgo && linux && !musl); scripts check covers top-level scripts/<file> only; empty/unparseable openapi exits 1 not 2/3; missing --exempt path treated as empty silently; shebang without exec bit.

## Route permissions review (round 3, F-R1..F-R6)
- F-R1 closed: `POST /analytics/reports` (every report type aggregates system-wide data) and `GET /analytics/media/:media_id` (aggregates every user's access logs) are admin only; real-binary cases in `round1Cases`. Control-needled census of the analytics and report routes: `/analytics/system`, `/reports/usage`, `/reports/performance`, `/analytics/reports`, `/analytics/media/:id` admin only; `/analytics/user/:user_id` self-or-admin; `/analytics/access` and `/analytics/event` are write routes (see P2-t). catalog-web has no caller of `createReport` or `getMediaAnalytics`, so no 403 handling is needed; web code is UNVERIFIED (no node_modules).
- P2-t (important, open): `POST /analytics/access` and `POST /analytics/event` take `user_id` from the request body, so any authenticated user can write access logs and events attributed to another user (forging the data the admin reports aggregate). Decide whether to force `user_id` from the token.
- F-R2/F-R3/F-R5 closed with tests in `services/sync_service_netguard_test.go`; the webdav client cache is guarded by `webdavMu`, and a rejected URL update no longer leaves a client for the rejected URL cached.
- F-R6 (nits, open): (a) a redirect to another allow-listed host fails TLS because the transport uses a fixed `ServerName`; (b) netguard ignores `HTTP_PROXY`/`HTTPS_PROXY` on purpose (`Transport().Proxy` is nil): behind a mandatory egress proxy sync endpoints cannot connect, and any future proxy support would bypass the dial-time address check because the proxy resolves the host; decide the policy before adding it; (c) a trailing-dot host (`example.com.`) is refused by `CheckURL`, which is stricter than DNS requires.

## Smart collections round 1 follow-ups
- SC1 (resolved): the Postgres jsonb branches (genre, quality) now run against a real Postgres in `smartrules_pg_test.go`; tags are comma-separated text and use the text path.
- SC2 (F7): case-insensitive matching is ASCII-only on SQLite; non-ASCII folding is not done.
- SC3 (F10): SmartCollectionBuilder still offers options the server rejects (artist, album, decade, play_count, last_played, file_extension, bitrate, resolution, fps, channels, subtitles, is_empty, last_N_days); aliases were added for is_favorite/file_size/duration/date_modified only. catalog-web edits are UNVERIFIED (no node_modules).
- SC4: control characters (\x01-\x03) are rejected in values; values with control characters never match JSON-encoded columns.
- SC5: the list-count cache is a 30 s TTL per process, not shared between instances; `total_items_status` reports exact/capped/stored.
## QA consumers of the honest-501 and registration-approval behaviours (branch fix-qa-consumers, 2026-11-03)

Status: F1 (endpoints that answer an honest 501), F2 (self-registered users inactive under `REGISTRATION_MODE=approval`) and F6 (the 200..399 acceptance and the `000000` status artefact in the QA runners) are FIXED on the branch for every consumer found by the sweep below. Open items are listed at the end.

Sweep (run inside a checkout, control needles included; the file list is in the branch evidence):
`grep -rIlE 'api/v1/(auth/register|recommendations|configuration/status)|REGISTRATION_MODE' --exclude-dir=.git --exclude-dir=node_modules --exclude-dir=submodules --exclude-dir=constitution --exclude-dir=qa-results scripts tests challenges docker-compose*.yml docs/qa .github deploy tools catalog-api/challenges catalog-api/tests` found 29 files after the fix. Control needle: the known consumer `scripts/run-full-qa.sh` is in the list (1 hit); a fabricated token searched over the same trees gives 0 hits. A second sweep, `grep -rIn -E '\|\| *echo *"?000"?'` over `scripts tests challenges docker-compose*.yml docs/qa`, found 3 real uses of the `|| echo 000` idiom after `curl -w` (`run-api-validation.sh`, `run-full-qa.sh` twice); all three are removed.

Fixed (each demonstrated against a real catalog-api binary on an ephemeral loopback port: RED = old check against the new behaviour, GREEN = fixed check, MUTATION = a reverse proxy in front of the real binary that answers 200 with invented items for trending, the recommendations root, by-type, personalized and configuration/status):
- `scripts/run-full-qa.sh`: `personalized/1` expected 200, now the logged-in admin's own id (owner-or-admin route) is pinned to exactly 501, plus the root alias (501) and a bad id (400); the register step sent no `first_name`/`last_name` (HTTP 400, accepted by the loose `201|409|400|429` list), now it registers with a full body, asserts 201, asserts login 403 while pending, approves through the admin API (CSRF cookie plus header), asserts login 200; mode-agnostic (reads `is_active`). `api()` matched the status as a substring and printed `000000` on a connection failure; now an exact match on the `|`-separated alternatives and a normalised `000`.
- `scripts/run-helixqa.sh`, `docker-compose.qa.yml` (helixqa-api): accepted 200..399 for `configuration/status` and `recommendations/trending` (so a regression to an invented 200 passed). Now exactly 501 is the only passing status for `configuration/status`, `recommendations`, `recommendations/trending` and `recommendations/personalized/*`; every other endpoint must be 2xx/3xx. `run-helixqa.sh` appended a second `000` (`000000`, and `401000` for a 4xx login with `-f`); normalised. `API_URL`/`WEB_URL` are now overridable.
- `scripts/run-helixqa-web.sh`: `-sf ... || echo 000` printed `501000`; normalised.
- `scripts/testing/full_automation/catalog_collections_downloads_stats.sh` R1-R3 (recommendations root, trending, by-type): expected 200 plus items; now exactly 501 plus an `error` body and no `items`/`recommendations` payload. Guide `docs/scripts/catalog_collections_downloads_stats.md` updated.
- `catalog-api/challenges/ch059_recommendation_api.go` (and its mock in `ch051_060_test.go`, which served non-`/api/v1` paths and asserted nothing): root and by-type pinned to 501, trending added; a test that an invented 200 fails all three pins. `userflow_api.go`: `UF-API-AUTH-REGISTER` sent no names (400) and used `ExpectedStatus: 0`, which checks nothing; now a full body and `AcceptedStatuses: [201, 409]`. `UF-API-ADMIN-STATS` `config-status` accepted any status; now exactly 501.
- `tests/test_recommendations.sh`: rewritten. The old script started its own binary, never read a status (`if curl -s ...; then "works"`) and printed "works" for the 401/404/501 answers; it now takes `API_URL`, pins the exact statuses, checks the 501 body and exits non-zero on any failure.
- `tests/test_subtitles.sh`: register-then-login now registers with a valid body, approves the pending account through the admin API (needs `ADMIN_PASSWORD`, fails with an explicit message without it), and uses `BASE_URL`.
- `docker-compose.test.yml`: `REGISTRATION_MODE=${REGISTRATION_MODE:-open}` for the QA stack (documented in the file); suites about something other than the policy get accounts that can log in, `REGISTRATION_MODE=approval` exercises the approval path. The policy itself stays covered by `catalog-api/registration_approval_binary_test.go`. `catalog-api/tests/integration/auth_integration_e2e_test.go` already used `REGISTRATION_MODE=open`.
- `challenges/helixqa-banks/` API comprehensive and negative-paths banks and the web comprehensive bank: expectations for trending, personalized, configuration/status (501), register (201, pending under approval), the api-flow login precondition, mass-assignment and web-register-success updated; the bank converter's `_infer_expectations` now infers 501/201/200 for those steps. These YAML banks still carry `# TODO: Convert to executable` actions, so no runner drives them against a binary (see open items).
- New helper `scripts/lib/qa_registration.sh` (guide `docs/scripts/qa_registration.md`); `scripts/tests/test_test_stack_boots.sh` guard on the QA user password retargeted at the new register call.
- `tests/test_register.js`, `test_auth_debug.js`, `test_subtitle_auth.js`, `test_complete_subtitle.js`, `test_simple.js`, `test_browser_console.js`, `test_subtitles_node.js`: ad-hoc debug scripts, not part of any suite; a header now states they need `REGISTRATION_MODE=open`.

Open / not fixed (reason):
- The YAML banks are prose (`action: '# TODO: Convert to executable'`); there is no bank runner in this tree that executes them, so the changed `expected` text was checked only through the converter's inference, not against a binary. Converting the banks to executable steps is the real fix.
- `GET /api/v1/discovery` answers 404 and `GET /api/v1/recommendations/similar/1` answers 404 on an empty database: `run-full-qa.sh`, `run-helixqa.sh` and the compose QA session still list `discovery` as 2xx and `similar/1` as `200|500`. Not in this class; each is its own finding (route missing or media-not-found on a fresh database).
- The seven JS debug scripts above register and then log in; they are documented, not rewritten.
- `catalog-web/e2e` registration specs mock `/auth/register` in the browser (`page.route`), so they do not meet the server policy at all; a live registration e2e against a `REGISTRATION_MODE=approval` stack is not written.
- Smart playlists and collections QA expectations are being changed on another branch and were left alone.

## QA consumers review follow-ups (Opus review of fix-qa-consumers, GO-with-follow-ups)

- QC1 (minor): `scripts/lib/qa_registration.sh` creates `$TMPDIR/qa_body.*` (mode 600, may hold session_token) and callers never remove it; add an EXIT trap and correct the "no side effects" header.
- QC2 (minor, older): `run-api-validation.sh` phase 5 hard-codes `http://localhost:8085/health`, ignoring `API_URL`.
- QC3 (minor, older): the collections/downloads/stats script exits 0 when every test SKIPs (429 rate limit).
- QC4 (minor, older): `tests/test_subtitles.sh` has no assertions and prints the first 50 chars of the JWT.
- QC5 (minor, older): CH059 test 3 hits a non-existent route (gin 404, vacuous); UF-API-AUTH-REGISTER `register_body` not_empty reports `expected=<nil>`.
- QC6 (nit): `personalized/${ME_ID:-0}` cannot detect a failed id lookup when run as admin.
- QC7 (nit): qa_registration.sh header names docker-compose.qa.yml; the setting is in docker-compose.test.yml.
- QC8 (gap): the docker-compose.qa.yml helixqa-api loop was read, not executed.
- SC6 (review round 3, finding 4): Postgres `::jsonb` casts run on text that may not be valid JSON (a legacy plain-text genre, malformed quality_info) and can raise an error for the whole query; the guards only test the first character.
- SC7 (review round 3, finding 5): the per-user cap check (count then insert) is not atomic; concurrent creates can exceed the cap by a few.
- SC8 (review round 3, finding 6): `fillLiveCounts` shares one per-request deadline, so a slow early collection starves the later ones in a list.
- SC9 (review round 3, finding 7): error mapping of rule/evaluation failures to HTTP statuses is coarse (some evaluation errors surface as 500).


## Route permissions review (round 4 findings N1-N6)
- N1 closed: `POST /analytics/access` and `POST /analytics/event` attribute the write to the token's user id (`bindWriteUserID` in `handlers/service_handlers.go`); a body `user_id` naming another account answers 403, an absent one is replaced by the caller's id (before: user_id 0 rows, or forged rows that raised the admin's `GET /analytics/user/1` counts). Real-binary test `TestRoutePerms_RealBinary_AnalyticsWritesCannotForgeUserID` (RED before) and mutations (see the round report). Control-needled census of handlers reading a user or owner id from body, query or path: mounted routes read the id from the token (`c.Get("user_id")`, `r.Context().Value("user_id")`) except `Param("user_id")` in `GetUserAnalytics` and `recommendations/personalized/:user_id`, both behind `RequireSelfOrAdmin`. No other mounted handler binds a user id from the body. UNMOUNTED, not reachable today: `internal/handlers/media_player_handlers.go` binds `services.UpdatePositionRequest`, `BookmarkRequest`, `ReorderPlaylistRequest` and others that carry `user_id` in the body, and `internal/handlers/localization_handlers.go` `SetupWizardLocalization` accepts a body `UserID`; their constructors (`NewMediaPlayerHandlers`, `NewLocalizationHandlers`) are called only from tests. Whoever mounts them must take the user id from the token first (P2-u).
- N2 (important, OPEN in this branch): `media_collections` has no owner column, so any authenticated user can PUT/DELETE/POST any collection (reviewer live proof: bob `PUT`/`DELETE /collections/1` answered 200). CLOSED BY the pending `feat-smart-collections` branch (migration 23 adds owner and is_public; writes need the owner or an admin; ownerless collections are admin only). It MUST land before or with the release. Not touched here, to avoid conflicts. Regression test: `TestRoutePerms_RealBinary_CollectionOwnership` in `route_perms_round1_binary_test.go` is skipped with that reason until the branch lands (run it today with `CATALOGIZER_RUN_N2=1`: RED, bob's PUT and DELETE answer 200); remove the skip when it merges. N1 mutations: forge check removed, body id not overwritten, LogEvent unbound, LogMediaAccess unbound, 403 turned into 400, zero-id rejected (mut7) are all killed by the real-binary test; the unauthenticated branch of `bindWriteUserID` is unreachable behind the auth middleware and is pinned by the unit test `TestBindWriteUserID_ContextWithoutCallerIsRefused`.
- F-A (round 3, closed): `getWebDAVClient` reused the client cached by endpoint id even after a URL or credential change; it now reuses it only when URL, username and password match. Tests `TestSyncService_CredentialOnlyUpdateUsesNewCredentials` and `TestSyncService_WrongNewPasswordIsRefusedNotTestedWithOldCredentials` (RED before); dropping the comparison is killed by both. F-B: the SelfOrAdmin real-binary test now includes a prefix-sharing id.
- N4 (minor): another user's playlist `PUT`/`DELETE` answers 500 instead of 404. Closed by the same branch (`ownedPlaylist` returns 404). Not touched here.
- N5 (flaky, pre-existing): `tests/stress` `CacheWriteThroughput` has a floor of 100 ops that fails under `-p 4` load. Make the floor relative or run it serially.
- N6 (pre-existing, fails on plain main): `internal/tests/examples` `TestUserRepositoryExample` panics on a nil value.

## UI D1/D2 landing follow-ups (reviewer GO-with-follow-ups, 2026-11-03)
- UI-F5: force socket down via routeWebSocket, cover /login and more than one fine-pointer viewport.
- UI-F7: commit firefox/webkit evidence JSON.
- UI-F8: run clean `npm ci`; missing `@testing-library/dom` devDependency not reproduced.
- UI-F2..F4, version chip overlap at 320, chevrons, dev stats: see UI branch notes.
- Full vitest/eslint not run after merging main bd8e52e4.

## Converter hardening landing follow-ups (reviewers GO / GO-with-follow-ups, 2026-11-03)
- CV-F1: add census entry for text after `</svg>` (mutation M4 survives).
- CV-F2: refusal census should assert which refusal fired, not just assert.Error.
- CV-F3: fix doc comment on imageMagickCoder (returns "svg:", not "").
- CV-F4: renderer-side CSS parser-differential classes (CSS escapes in url(, other fetch functions) not covered by the independent payload review; a second reviewer's hands-on attacks found no leak. Residual: UNCONFIRMED for other renderers.
- CV-F5: Inkscape-metadata and UTF-16 SVGs are refused; consider dropping foreign attrs instead of refusing; document UTF-16; update note on rsvg-convert coverage.

### P2 round-4 review follow-up (GO-with-follow-ups)
- R4-1 (test-instrumentation, minor): reviewer mutation dropping the username comparison in `getWebDAVClient` survives; add a username-rotation test whose fixture server also checks the username. Also untested: end-to-end `UpdateEndpoint` concurrent with `StartSync`. **Username/password rotation FIXED (security queue): `TestSyncService_WebDAVClientIsRebuiltOnUsernameOrPasswordRotation`, both mutants killed; the `UpdateEndpoint` concurrent with `StartSync` end-to-end test is still open.**

## Authz round 2 (fix-authz-round2): decisions applied and what remains
Applied (operator decisions): `/metrics` and `/debug/pprof/*` need an administrator (pprof also still needs `HELIX_PPROF_ENABLED=true`; scrapers must send an admin bearer token); the outbound triggers `POST /entities/enrich`, `POST /entities/:id/metadata/refresh`, `POST /assets/request`, `GET /subtitles/search`, `POST /subtitles/download`, `POST /subtitles/translate` are admin-only; the subtitle-download and external-cover fetches now use `internal/netguard` (dial-time check, pinned address, redirect hops re-checked); `GET /scans*` stays auth-only; `POST /admin/image-quality/revalidate` and `POST /admin/covers/generate-thumbnails` keep their per-route `RequireAdmin`. Subtitle upload is bounded (2 MiB file, 3 MiB request, 413). Route table: `docs/ROUTE_AUTHZ_TABLE.md`.

Open (not fixed here):
- AZ2-1 (defence in depth): fixed-host provider clients (TMDB in `media_entity_handler.go`, fanart.tv, IGDB, Cover Art Archive, `cover_art_service.go`, lyrics, translation, recognition providers) still use `GuardProviderURL` + a plain `http.Client` (no dial-time pinning, redirects followed). Their URLs are built from fixed hosts, so an attacker cannot choose the host today; move them to `netguard.Client` when a provider takes a URL from data. The TMDB client may route through the operator's SOCKS5/HTTP proxy, which `netguard.Transport()` ignores by design; that needs a decision.
- AZ2-2 (policy): every authenticated (including self-registered) user can still read system-wide read-only aggregates: `GET /stats/*`, `/analytics/stats`, `/analytics/duplicates`, `/entities/stats`, `/challenges` and `/challenges/results`, and can attach a subtitle to ANY media item (`POST /subtitles/upload`, no ownership). Operator to decide whether these need a permission.
- AZ2-3: `internal/monitoring.StartPrometheusServer` opens an UNAUTHENTICATED `/metrics` on its own port. It is not called by `main` today (grep over non-test code); anyone wiring it must put the admin gate in front or bind it to loopback. **FIXED (security queue): CONFIRMED it answered on the host LAN address; it now binds 127.0.0.1 and returns the server (`TestStartPrometheusServer_ListensOnLoopbackOnly`, two mutants killed). Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- AZ2-4: public by design and unchanged: `/health`, `/api/v1/health`, `/health/deep`, `/discovery` (host, port, instance id for LAN discovery), `/ws` (token in query), `/api/v1/image-proxy` (https + allow-listed hosts), `/api/v1/assets/:id`, `/api/v1/cover/*`, `/auth/login|register|refresh|logout|status|init-status`. `/auth/logout` is public: confirm it only revokes the presented token.
- AZ2-5: `pprof` `profile`/`trace` hold an admin request for the sampling time (`?seconds=`) and are not rate-limited beyond the global chain.
- AZ2-6: the route table is produced by a control-needled static extractor over `main.go` plus a live probe (`route_enumeration_probe_test.go`, env-gated); a test that fails when a NEW route is registered without a row in a checked-in table does not exist yet. Routes mounted from other files (`RegisterStorageRoutes`, `RegisterScanRoutes`, `RegisterAliasRoutes`) are added by hand to the extractor input; a route added in a fourth place would be missed.
- AZ2-7: `POST /assets/request` still lets an admin pass `source_hint` as a server file path to the asset chain (`LocalScanResolver` confines it to the read roots); not changed here.
- N2 and N4 are closed on main by the smart-collections merge (e1695dfd: ownership migration 23, playlist 404).
### Smart collections round-4 review follow-ups (GO-with-follow-ups)
- SC10 (test-instrumentation, minor): reviewer mutation dropping the viewer uid from `countKey` (collection_handler.go ~193) survives; add a two-viewer test (different favorites, same smart collection within the 30 s TTL, different `total_items`).
- SC11 (process-doc, nit): UPGRADE_NOTES says an admin sets `owner_user_id` for a legacy collection, but `UpdateCollection` does not accept it; say it needs direct SQL on the SQLCipher DB or add an API.
- N2 is closed: `TestRoutePerms_RealBinary_CollectionOwnership` ran GREEN against the real binary and its skip gate is removed in the landing merge.

## Live API contract sweep (2026-11-03) - follow-ups
Evidence: `docs/qa/api-contract-live-20261103/`; runner `scripts/test-infra/api_contract_live.sh` (guide `docs/scripts/api_contract_live.md`).
- AC1 (OPEN, feature gap): `GET /api/v1/download/file/{id}` and `GET /api/v1/download/directory/{path}` are implemented for SMB storage roots only. For every other protocol (local, ftp, nfs, webdav) they now answer 501 (they used to answer 500 after a failed SMB connect). Implementing local-root download needs a path-containment check against the root and is a separate item. The spec still lists no 501 for these operations.
- AC2 (OPEN, UNCONFIRMED): `GET /api/v1/assets/{id}` answers 200 with a pending placeholder for an unknown id, so the documented 404 is never produced; harness exemption `notfoundExempt` records the reason. Decide whether the spec or the handler should change.
- AC3 (OPEN): happy paths not executed by the sweep: `/ws`, challenge run endpoints, admin backup restore (destructive or long-running); their other probes (401/403/404/400) do run.
- AC4 (OPEN): the sweep's schema validator is minimal (type, required, properties, items, enum, nullable, allOf, date-time); no oneOf/anyOf, min/max, pattern or other formats. Non-JSON bodies are not schema-validated.
- AC5 (OPEN): `DELETE /logs/share/{token}` and `GET /logs/share/{token}` share one path template but the delete handler reads an integer share id; OpenAPI cannot name the parameter differently per method, so the spec describes only the token form.

- W10 update (UNCONFIRMED): under the stricter live verdict an idle-guard cancel (`context canceled` after 69 s) failed the download of `two_mb.bin` in step 06 once (log `docs/qa/webdav-yandex-live-20261103/run_stricter_verdict_attempt2_FAIL_idle_cancel_and_EOF.log`); whether the stall was in the response headers or in the body is unknown because the error was not yet diagnosable. The client now reports `webdav idle timeout: no data for <d> (<phase>)`, and the live script fails any run with such an error. The same attempt hit `Put ...: EOF` on a concurrent upload (now classed transient and counted). Not reproduced in the later N=3 run.

### WebDAV live suite round-2 review follow-ups (GO-with-follow-ups)
- R2-1 (test-instrumentation, minor): the transient classifier was widened after observed failures (TLS handshake timeout, `": EOF`) and PASS_WITH_ANOMALIES has no ceiling; cap retries per class/run and label classes unattributed (`": EOF` on PUT may be a stale keep-alive reuse, not the service).
- R2-2 (test-instrumentation, minor): mutant removing `guard.rearm(webdavResponseWait)` on the empty-body path survives; assert the empty PUT is bounded by the response wait.
- R2-3 (nit): mutant dropping the `want>0 && read>=want` re-arm survives, probably equivalent on HTTP/1.1; HTTP/2 not analysed.
- R2-4 (process-doc, minor): docs/TESTING_GUIDE.md ~319-321 lacks PASS_WITH_ANOMALIES and the integrity rules, and understates observed retries.
- R2-5 (nit): `run_N3_PASS.log` (old-verdict run) deleted without note (kept in git history); RED evidence is N=1.
- R2-6 (nit): `transient()`/`retry()` live only in the live-tagged file, no offline mutation check.
- Not run: merged-tree race run of ./internal/services, scan-leg test executed only live by the author.

### Live browser specs for catalog-web (L3-L5, L8, L11; branch test-web-live-l4)
- WL1 (RESOLVED): stack 3 now starts the API with `CATALOGIZER_LOCAL_ROOT_BASES=/media` (colon-separated absolute bases; unset denies every local root, `catalog-api/filesystem/local_policy.go`); `docs/qa/stack3/{up.sh,README.md}` on branch `stack3` of the QA clone (commits 86765365 and 9a15580d; the latter adds `CATALOGIZER_READ_ROOTS=/media:./data`, without which a conversion cannot read a corpus file: the default read roots are `MEDIA_ROOT_PATH:./data`). `catalog.live.spec.ts` is LIVE there (scan of the /media corpus, list/search/browse counts equal to the real API, manifest.json oracle).
- WL2 (image age, historical): stack 1 (main@96b7b5ec) failed the live specs for features that landed later (registration without approval, reload logging out, Collections/Admin chunk crash, "Disconnected" chip on /login, nameless header menu button at 390px). All pass on stack 3; the stack-1 evidence is kept outside the repo.
- WL3 (instrument limit): the websocket outage leg injects the fault with a real TCP forwarder between browser and nginx; it does not exercise API-side socket termination or a server restart.
- WL4 (not covered): Webkit not run; 390px emulation is a viewport only (no touch/mobile UA); playlists UI flows, backups/metrics admin pages, real conversion of video, subtitle download flow.
- WL6 (FIXED here, RED-first): `@tailwindcss/aspect-ratio` replaces `theme.aspectRatio`, so `aspect-video`/`aspect-square` were never emitted; the media player box (and playlist/favourites tiles) collapsed to height 0 and the `<video>` was never visible. Found by `playback.live.spec.ts`; guard `catalog-web/src/__tests__/tailwindAspect.test.ts`.
- WL7 (FIXED here, RED-first): `DELETE /api/v1/users/:id` returned 500 for every user that had ever registered or logged in (`auth_audit_log.user_id` and `user_permissions.granted_by` reference `users(id)` without ON DELETE). `UserRepository.Delete` now detaches both in one transaction (audit rows are kept with NULL user). Guard: `catalog-api/repository/user_repository_delete_fk_test.go` (real SQLite, FKs on). The PostgreSQL leg was RED live on stack 3 (`?` markers sent through a raw `sql.Tx`), fixed with the new `DB.Rewrite`. Still open: users that own `sync_endpoints`, `sync_sessions`, `sync_schedules` or `playlists` rows still cannot be deleted (FK without cascade); `UserRepository.DeleteRole` also uses a raw `tx.Exec("... ?")` and would fail on PostgreSQL.
- WL8 (FIXED here, security review): the player used no authenticated media URL at all (`<source src="<relative path>">`, never reached the stream route). The first fix put the session JWT in `?access_token=` (commit 8e435b5d, superseded). Replaced by 60 s HMAC stream tickets (`POST /api/v1/stream/:id/ticket`, `?ticket=` valid only for that media id and only on `GET /api/v1/stream/:id`; both query params are redacted in the access log). Follow-ups: (a) the legacy `access_token`/`token` query fallback in `middleware/auth.go` is kept only for native clients and should be migrated to tickets and removed; (b) there is no per-user ownership model for catalogue files, so any authenticated user can mint a ticket for any catalogued file (documented in docs/ROUTE_AUTHZ_TABLE.md); (c) `HEAD /stream/:id` is not routed (404), so ticket-bearing HEAD probes are not possible.
- WL9 (UNCONFIRMED, product defects seen while probing, not fixed): the Admin user-list "Edit" button for non-pending users is a no-op; the suspected favourites id-space mismatch (UI posts `entity_type: 'movie'` with a file id) was NOT reproduced: the live favourites spec stores the clicked item and the Favorites page lists it; scanned `.srt` sidecars are catalogued as separate media items and are not linked to their video (`GET /subtitles/media/:id` empty); `GET /download/file/:id` returned 500 "Failed to stream file" in one UI probe.
- WL10 (tooling): `scripts/check_route_vs_spec` (route-vs-spec checker) has 21 pre-existing drifts on main; `docs/api/openapi.yaml` had two duplicate `'403'` keys that made it unparsable (removed here).
- WL11 (FIXED here, RED-first): every state-changing call under `/api/v1/admin/*` from the browser returned 403 `missing csrf header`: the admin group mints `X-CSRF-Token` on GET and requires it back (cookie `__Host-csrf` + header, double submit), but `catalog-web/src/lib/api.ts` never echoed it, so Approve/Deny user, create/restore backup and scan storage could not work in the UI. Found by `management.live.spec.ts`. The client now remembers the minted token and sends it on non-GET requests, and retries once on a csrf rejection (`src/lib/__tests__/apiCsrf.test.ts`). A second, intermittent defect behind it (1 of 3 identical live runs failed with 403): every GET without the header minted a NEW token+cookie, so with the browser's parallel GETs the cookie it kept and the token the page kept could come from different mints. `middleware/csrf.go` now reuses a valid presented cookie's token on GET and reissues a usable pair on every csrf rejection (`middleware/csrf_test.go`). Open: the admin UI "Edit" button for non-pending users is still a no-op (WL9), and only `/admin/*` enforces CSRF; other cookie-less Bearer routes do not need it.
- WL12 (Opus review rounds 1 and 2 of the live branch; FIXED here): `MediaPlayer.tsx` ticket lifecycle. Round 1: the recovery budget was never reset and idle refreshes consumed it, so after 3 refreshes a 401 on a seek ended in "Video playback failed"; a seek more than 60 s after the last mint hit 401. Round 2 found the first fix wrong in three ways (N1 reset on `loadedmetadata` made a corrupt file loop forever; N2 a second error during a reload overwrote the resume position with 0/paused; N3 a 45 s timer `load()`ed every playing element, dropping its buffer). Now (one structural design): no timer ever re-points the element; the ticket is refreshed LAZILY, (a) when the element errors with anything but MEDIA_ERR_DECODE (a 401 on a range request surfaces as a network / not-supported error) and (b) on a `seeking` event when the ticket is older than 45 s AND the seek target is not already buffered; the single resume target is written only while the element is not mid-reload and applied by one `loadedmetadata` listener; the recovery budget (3 consecutive recoveries) is restored ONLY by real progress (`timeupdate` advancing past the resume point), never by `loadedmetadata`. Vitest (25 tests): mutants for reset-on-loadedmetadata, overwrite-mid-reload, retry-decode and seek-ignores-buffer each fail one test; the round-1 component fails 5. LIVE leg "long playback" uses `e2e/live/gen_big_fixture.sh` (a ~200 MB, 140 s, 12 Mb/s file registered in the corpus manifest) so the browser cannot hold it fully buffered: it plays past the 60 s ticket lifetime, asserts the seek target is NOT buffered, seeks to 120 s, and asserts a fresh ticket was minted, the position is >= the target (and >= the pre-refresh position; no `loop`, so a reset to 0 cannot hide), and keeps advancing. Round 3 (Opus, NO-GO, FIXED): D1 Chromium sets `paused=true` BEFORE it fires the error event, so a recovery that captured `!v.paused` resumed a playing video as PAUSED (reproduced 3/3 in headless Chromium after a post-expiry seek and after a cut connection; Firefox resumed). The player now keeps a play INTENT ref driven by user actions and `play`/`pause` events that are not caused by an error, a reload or the end, and calls `play()` after the resume when the intent was playing. D2 a media switch during an in-flight mint played the OLD item (a single inflight flag); every mint is now tagged with a generation, stale results are dropped, a mint for the new id is allowed while another is in flight, the old source is cleared at the switch, and the callers key the player by media id. Also: the progress floor needs 2 s of real advance (the resume itself fires `timeupdate` at the floor), total recoveries per source are capped at 100, the user's LAST seek during an in-flight seek-mint wins, and a position set during a reload beats the stored resume. Tests: 35 jsdom tests (mutants R1, R2, R9, R12, D1, stale-drop, seek-wins, step, pause-filter, clear-on-switch each fail a test) PLUS real-browser tests `e2e/player-real/` (chromium and firefox, real MediaPlayer + real `<video>` against a local short-TTL ticket server; `npx playwright test -c playwright.player.config.ts`): after an expiry recovery (post-expiry seek, cut connection) the element is not paused and advances; reverting the intent fix makes the chromium seek test fail (`paused:true`). The live long leg also asserts `paused===false` after the post-expiry seek. Open (honest): the error path is not a silent stutter - the element is briefly re-pointed and reloaded, which drops its buffer; the local ticket server is a test double, so these real-browser tests are component evidence, not LIVE evidence.
- WL12-F5 (FIXED): the binary test's expired ticket now uses the real user id and live session id of a genuinely minted ticket plus a control (same claims, unexpired = 200), so removing the expiry check in `Verify` makes it fail (mutation run: FAIL).
- WL12-F2 (FIXED, partial): `catalog-web/nginx.conf.template` no longer logs query strings at all (`log_format nocreds` logs `$uri`): redacting named parameters is bypassable by duplicated names, percent-encoded names and new parameter names (checked by `catalog-web/tests/test_nginx_upstream.sh` check 3: ticket, access_token, token, duplicated, `%74icket`, `myticket`, `access%5Ftoken`; a mutant that logs `$request_uri` fails it). NOT covered: the nginx ERROR log prints the full request line and upstream URL when the upstream is unreachable, so a ticket (60 s) or a legacy `access_token` can appear there; nginx cannot redact its error log. Mitigation options: error_log at `crit`, or a log shipper filter. Tracked. (The pre-existing checks 1-2 of that script need container DNS on a podman network and fail in this environment on the untouched template too.)
- WL12-D1 (test baseline): in this environment the same 14 vitest failures in 5 files fail on live main and on this branch: `module-registry.test.ts` (7), `AuthContext.integration` (7), `WebSocketContext`, `WebSocketContext.connectionStatus` and the `websocket` file-level failure; the earlier "7 tests in 2 files" figure came from an incomplete selection (`src/components/media` and `src/lib` only).
- WL12-F3 (open): `UserRepository.Delete` handles `auth_audit_log` and `user_permissions.granted_by` only; a user owning `sync_*` or playlist rows still fails with an FK error (500), and `DeleteRole` runs a raw `*sql.Tx` with `?` placeholders (breaks on PostgreSQL; use `db.Rewrite`). Audit rows now keep `former_user_id=`/`username=` in `details` when the user is deleted (F4, FIXED).
- WL12-F6/F7: the committed evidence records are provisional pending the recorder redesign (the pinned web digest is still supplied by the driver via `LIVE_EXPECTED_WEB_IMAGE_DIGEST`; use the repo digest, not the image ID). The artifact fingerprint is now read from `/health` by every test at run time (`targetFingerprint` in `support.ts`) and `make_evidence.mjs` refuses to write records when the runs disagree with each other or with the build read at evidence time.
- WL5 (test hygiene, RESOLVED by WL7): the registration/admin specs create `live_*` users and delete them in `finally`; until WL7 the delete itself failed (500) and left active users behind. 18 stray users on stack 3 were removed after the fix (all 204).

- AZ2-8 (F7, proxy-ignore): the subtitle service client and the cover/external-metadata resolver client (`netguard.StrictClient`) deliberately ignore `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY`: a proxy would resolve the target itself and bypass the per-connection address check. A deployment that can only reach the internet through a proxy therefore cannot fetch subtitles or covers from provider URLs until a reviewed proxy-aware dialer exists (the proxy address would need to be allow-listed and the destination checked by the proxy policy). Operators should expect "dial" errors, not silent fallbacks.
- AZ2-9 (F3): these two clients are STRICT: `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` does not apply to them (loopback, private, CGNAT, link-local and metadata addresses are always refused, on every redirect hop). The env allow-list still applies to the other `netguard.Client` users. A local metadata mirror on a private address therefore cannot be used for subtitle or cover URLs.
- AZ2-10 (F1): `METRICS_SCRAPE_TOKEN` is a single shared secret with no rotation or per-scraper identity; rotate by changing the env and the mounted file together. Prometheus `credentials_file` is re-read on reload; the OpenTelemetry collector reads the env at start.

## Authz round 2 review follow-ups (Opus round 2, GO-with-follow-ups)

- AZ2-R1 (important, source-defect, pre-existing): fanart_resolver.go, igdb_resolver.go, llm_image_resolver.go check a URL once, then fetch with a plain http.Client that follows redirects and re-resolves DNS (SSRF via redirect/rebinding). Switch to netguard.StrictClient with tests (redirect to loopback/metadata, rebinding). **FIXED (ssrf round 3): fanart, igdb download, llm image download and cover_art_service downloads use netguard.StrictClient; RED/GREEN tests in outbound_strict_resolvers_test.go (redirect to 127.0.0.2, metadata address, rebinding stand-in, allow-list ignored); census in docs/OUTBOUND_HTTP_CLIENTS.md. Review round 1: download() driven with a mapped-loopback literal, guardedRedirect unit + real forward-proxy tests, redirect-cap test, ffmpeg/ffprobe `-protocol_whitelist file,pipe` in cover_art_service.go, bounded download body, SOCKS5 dial honours ctx.**
- AZ2-R2 (minor, test): scrape token on /debug/pprof/* must be 401 in the RealBinary suite (mutant m2b survived). **FIXED: TestRoutePerms_RealBinary_Round2_ScrapeTokenDoesNotOpenPprof (mutant: scrape-skip on the pprof group is caught).**
- AZ2-R3 (minor, test): strict mode with IPv4-mapped IPv6 (::ffff:127.0.0.2, ::ffff:10.0.0.1) and allow-list env set (mutant m7 survived). **FIXED: TestStrictBlocksIPv4MappedIPv6WithAllowListSet (strict check and strict dial; mutant m7 equivalent, strict honouring the allow-list, is caught).**
- AZ2-R4 (minor, test): Refresh-button page wiring test with isAdmin false; consider defaulting canRefresh to false. **FIXED: EntityDetailView canRefresh now defaults to false; page test hides Refresh for a non-admin (mutant canRefresh={true} is caught).**
- AZ2-R5 (minor, deployment): Prometheus token-file mount breaks container start when the file is missing; document creation or make the mount optional. **FIXED: no host token file is mounted; the prometheus entrypoint writes the token from METRICS_SCRAPE_TOKEN inside the container and fails fast with a clear message when unset/short; scripts/monitoring/test_prometheus_token_start.sh (real prom/prometheus under rootless podman) plus --mutations (3/3 killed).**
- AZ2-R6 (nit): deployment/docker-compose.yml monitoring mount path, otel collector not started with METRICS_SCRAPE_TOKEN by any compose file, Gate column for /metrics should mention the token. **FIXED: deployment/docker-compose.yml mounts ../monitoring/... and aliases the server as `api`; root docker-compose.yml gains an `otel-collector` service (profile `otel`, image pinned 0.164.0, config fixed: jaeger/loki exporters removed; Prometheus remote-write receiver is opt-in via PROMETHEUS_EXTRA_FLAGS because it is unauthenticated; Prometheus/Grafana now published on 127.0.0.1 by default via PROMETHEUS_BIND/GRAFANA_BIND) started with METRICS_SCRAPE_TOKEN; VERIFIED with raw evidence in docs/qa/monitoring-deploy-20261104/ (validate OK, collector 0 restarts, scrape with token, `up{job="catalogizer-api"}=1`, Grafana datasource+dashboards provisioned, receiver 404 by default); PROVISIONAL (unverified): `api` alias DNS resolution on the compose network and the full deployment/docker-compose.yml stack; ROUTE_AUTHZ_TABLE Gate column mentions the token.**

## SSRF resolvers review follow-ups (Opus round 2, GO-with-follow-ups)

- SSRF-F1 (minor, test): SOCKS5 ContextDialer fix has no test (revert mutant survives). Add a never-answering SOCKS5 listener + cancelled ctx test. **FIXED (security queue): root cause was different from the finding: net/http detaches a dial from the request context (Go 1.23+), so the ContextDialer never saw the request deadline and a proxy that accepts TCP and never answers held the handshake forever; the dial now has its own 30 s deadline (`socks5DialTimeout`); `TestDownloadImage_Socks5DialHonoursContextCancellation` (real listener, asserts the half-open connection is closed) + default pin; three mutants killed. Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- SSRF-F2 (minor, test): pin the production default of coverArtMaxBytes (32 MiB); tests overwrite the variable. **FIXED (security queue): `TestCoverArtMaxBytes_ProductionDefaultIs32MiB`, mutant 64 MiB killed. Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- SSRF-F3 (minor, test): TestCoverArtFFmpeg_PlaylistDoesNotReachNetwork cannot fail on ffmpeg 8.0.1; add a direct http://127.0.0.1:port/ videoPath case (a direct URL input DOES reach the listener without the whitelist: real behavioural RED, 1 hit each for ffprobe and ffmpeg; 0 with -protocol_whitelist file,pipe). Correct the claim in tests/docs that no behavioural RED exists. **FIXED (security queue): `TestCoverArtFFmpeg_DirectURLVideoPathDoesNotReachNetwork`, claim corrected in the old test comment. Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- SSRF-F4 (nit, test): assert -nostdin in the ffmpeg command. **FIXED (security queue): `TestCoverArtFFCommands_GuardArguments` (the flag was already present; now pinned). Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- SSRF-F5 (minor): cover-art ffmpeg guard lacks the converter's -format_whitelist allow-list; ffprobeDurationCmd has no ctx/timeout. **FIXED (security queue), upgraded from minor to source-defect: CONFIRMED that a library file that is really an HLS playlist (`movie.m3u8` naming a `file:` segment outside the library) gave a duration and a thumbnail from the other file (local file read primitive), and a FIFO named like a video blocked ffprobe forever. `-format_whitelist` (same list as the converter), ffprobe 30 s and ffmpeg 2 min default deadline; four behavioural REDs; six mutants killed. Evidence: `docs/qa/security-queue-fix-20261105/`; PROVISIONAL-DONE until a live rerun on a rebuilt stack.**
- SSRF-F6 (nit, doc): add ftp_client.go:88 (ftp.Dial) and redis/postgres sql.Open operator connections to docs/OUTBOUND_HTTP_CLIENTS.md. **FIXED (security queue): census rows added.**

## Media fixture generator review follow-ups (round 4, GO-with-follow-ups)
- MG-C1 (minor, source): os.walk in the policy check has no onerror; an unreadable subtree is skipped and policy returns ok. Fail closed.
- MG-C2 (minor, process-doc): gen() creates layout dirs before the stub manifest, so "stub plus helpers only" is unreachable; stale in-script comment and unused LAYOUT_TOP.
- MG-C3 (minor, test-instrumentation): mutants CM1-CM4 (.cover.png symlink check, '..' check, --check run1-symlink check, files element-type check) survive the base suite; add fixtures.

## Web player review follow-ups (round 4, GO-with-follow-ups)
- WL13-F1 (minor, source): a Pause click during the reload after recovery/seek-mint is overridden (resumeRef.play copied from wantPlayRef at mint time). Fix: in handleLoadedMetadata use `r.play && wantPlayRef.current`. Reproduced in real Firefox; add RED test first. OS media-key pause during reload is also ignored by the reloadingRef guard. **PROVISIONAL-DONE (2026-11-04):** fixed (`r.play && wantPlayRef.current`); RED/GREEN/mutation on chromium and firefox in `docs/qa/player-lifecycle-wl13-20261104/`. Media-key decision: undetectable (load() already paused the element, no pause event; probe test), documented, not honoured.
- WL13-F2 (minor, source): last seek does not always win when seek 2 lands in a buffered range while a seek-mint is in flight (handleSeeking returns before recording seekDuringMintRef). Record latest target first. **PROVISIONAL-DONE (2026-11-04):** handleSeeking records the target while a mint is in flight; evidence `docs/qa/player-lifecycle-wl13-20261104/`.
- WL13-F3 (minor, source): after media end, seeking back with a near-expiry unbuffered target can auto-play (wantPlayRef stays true). Clear wantPlayRef in handleEnded. **PROVISIONAL-DONE (2026-11-04):** evidence `docs/qa/player-lifecycle-wl13-20261104/`.
- WL13-F4 (nit): key={media.id} remounts player on switch, resetting volume/mute (intended).
- WL13-T (test-instrumentation): add tests for user pause during reload, buffered second seek, ended-then-seek. **PROVISIONAL-DONE (2026-11-04):** `catalog-web/e2e/player-real/player_lifecycle.spec.ts` (real chromium + firefox; ticket_server.mjs gained /ctl ttl, mintDelay, getDelay).

## Stack 3 scripts review follow-ups (round 2, GO-with-follow-ups)
- S3-1 (minor, test-instrumentation): test_scripts.sh mutants survive: `rc=0; [ "$web_up" = 1 ] || rc=6` -> `rc=0`, removing rc=6 from the probe loop, `set -euo` -> `set -uo`. Add a case with web 200 and one probe failing, and failing `podman run`/unhealthy container cases.
- S3-2 (minor, process-doc): README's section 11.4.173 deviation names a "tracked follow-up" with no tracker id; create the CAT item (containers-submodule/remote-host build).
- S3-3 (minor, process-doc): committed stack3 evidence is from build 1e92bc33 with pre-fix scripts; capture a run of the fixed up.sh/build.sh against real podman when stack 3 is next rebuilt from main.
- Process note: docs/qa/README.md is generated; whoever merges second between stack3 and live-map must regenerate with scripts/docs/gen_dir_indexes.py, not hand-merge.

## Live-map review round 3 follow-ups (LM-F*, minor)

- LM-F1 (test-instrumentation, minor): `docs/qa/live-map-20261103/test_restart_stack.sh` stubs cannot detect wrong probe paths (`/nope`, `/wrong`), swapped args 4/5, an optional 5th arg (`${5:-}`), or ignored `pg_isready`. Make stub curl path-exact, log stub calls and assert none in missing-arg cases, add a "5th arg missing" case, decouple pg and redis stubs.
- LM-F2 (process-doc, nit): `restart_stack.sh` header says postgres/redis answered HTTP 200; they are checked via `pg_isready` and `PONG`.
- LM-F3 (process-doc, nit): `restart_stack.sh` has not been run end to end against real podman since the fix (disclosed in MAP section 5); stays provisional.

## Live-evidence verifier / recorders (`scripts/test-infra/verify_live_evidence.sh`, `tests/live-evidence/`, `catalog-api/internal/liveevidence/`) — review round 1 of `a8c72de3`
Threat model unchanged: COOPERATIVE PRODUCER. F1 (loader keys bind every process kind) and F2 (enumerated env key set with prefixes; not claimed exhaustive, residuals listed in `docs/scripts/verify_live_evidence.md`) are fixed in the same wave; the remaining small items are OPEN until a commit on `main` closes them.
- F3 (test-instrumentation, OPEN): the interpreter set is hard-coded in three places (verifier `interp()`, Node `INTERP`, Go `runtimeLibRE`/family regexes) instead of being derived from the option table, the single source. Missing families / documented limit: Tcl, R, Julia, Erlang/Elixir, dotnet, gawk/awk, luajit, versioned names such as `node22`; the runtime library regex lacks `libtcl`, `libR`, `libjulia`, `libcoreclr`, `libluajit`. Fix: generate all three from the table plus a parity test, or document the limit next to `unsupported_exe`.
- F4 (test-instrumentation, OPEN, low: all of these fail closed): Node host resolution differs from Go for `[::1]`-only servers, compressed vs uncompressed IPv6 text, kept brackets and named hosts (Node maps only `localhost` to `127.0.0.1`; Go resolves names). Pinned today by `tests/live-evidence/test_host_node.mjs` (`::1` finds nothing against a 127.0.0.1 listener). Fix: resolve names in Node like Go and compare parsed addresses.
- F8 is closed in the docs (launchers named in `docs/scripts/verify_live_evidence.md` F5). F5 (five reviewer mutants), F6 (`liveevidence_mut` trap) and F7 (schema `env_checked`/`env_injected`, trim wording) are fixed in the same wave.

## API contract sweep follow-ups (test-api-contract-live, review round 1)

- Stream path serves without re-checking confinement: `StreamHandler.StreamFile` -> `SettingsFromRoot` does not re-check
  `CATALOGIZER_LOCAL_ROOT_BASES` or the storage root's `enabled` flag when serving. `GET /api/v1/download/file/:id` now hands non-SMB
  roots to this path (catalog-api/main.go `SetNonSMBFileHandler`) and inherits the gap. Fix at the stream handler (single place).
- Disclosed gaps of the download fix: (1) `GET /download/directory/{path}` happy path ends 404 in the sweep (no cataloged directory path
  is seeded); (2) `POST /download/archive` happy path ends 400 (the body is a `models.DownloadRequest` the generator does not shape);
  (3) a real directory under a non-SMB root still builds its archive through the SMB service (`createZipArchive`/`createTarArchive`
  call `smbService.DownloadFile`), so such an archive would be empty or fail.
- `catalog-api/handlers/download.go` (constructor `NewDownloadHandler(fileRepo, tempDir, ...)`) was not wired in main.go; REMOVED 2026-11-11 (SMB-FU-1, history in docs/FOLLOW_UPS_SMB_FU1_FU2_INVESTIGATION.md).
- Happy-path ratchet: `catalog-api/api_contract_happy_exceptions.tsv` lists the 81 operations whose happy probe is not 2xx, with
  a reason each; many reasons are generic (generator cannot seed a valid body/resource). Seed them and shrink the list; a stale entry is a violation.
- `GET /api/v1/logs/stream` flake (F5): the SSE handler writes nothing until the first log entry, and the sweep used to `ReadAll` the
  never-ending body, so every run spent the whole 60 s client timeout there (about 60 of the 70 s per run); under host load the header
  wait could also hit that timeout and log `transport error / no response`. The sweep now returns after the headers of a
  `text/event-stream` response. A transport error would still be reported; not reproduced on demand (UNCONFIRMED whether headers are
  flushed by middleware before the first entry on every host).


## Contract sweep review round 2 follow-ups (CS-N*)

- CS-N1 (test-instrumentation, important-ish, fix next): `catalog-api/api_contract_happy_exceptions.tsv` entries do not pin the expected status; a listed op can drift 400 to 403 or 404 to 409 silently. Add an expected-status column and treat any other status as a violation; stop tolerating 403 on the admin notfound probe.
- CS-N2 (source-defect class, existing on main): nil slice serialised as JSON `null`. Members found by live probe: GET /entities/duplicates (groups), /files (files), /stats/duplicates/groups (data), /stats/scans (scans), /configuration and /admin/config (sections), /recommendations/similar/{id} (local_items), /logs/collections/{id}/analyze (insights), /roles/{id} GET/PUT (permissions), POST /logs/collections/{id}/export (whole body). About 65 repository list functions use `var out []T`. Class inventory plus fixes needed (§11.4.276(C)); ListHistory already fixed (cd90fc72).
- CS-N3 (test-instrumentation, minor): setup stops polling GET /entities at first non-empty list and takes Items[0] while a scan may still run; 60 s poll timeout is silent; the signature covers statuses only, not data-dependent drift.
- CS-N4 (minor): exception keys are never validated against spec operations.
- CS-N5 (process-doc, minor): gofmt unclean in internal/services/catalog.go, catalog_pathnotfound_test.go, playback_session_repository_test.go; version_ldflag has no dirty-tree marker.
- CS-N6 (minor): 50 happy-2xx responses have no schema; add at least a null-where-array check.

## Nil-slice class review round 1 follow-ups (NS-*, minor)

- NS-1 (VERIFIED live 2026-11-04 on stack 3 build 4c79b7e3 over the LAN URL: empty media_types returns types: [] , evidence docs/qa/live-web-stack3-20261104/REPORT.md; was PROVISIONAL-DONE, CAT-NS-1) (source-defect, latent): GET /entities/types builds `var result []gin.H` (media_entity_handler.go:421); `types` serialises as null when media_types is empty. Fix RED-first.
- NS-2 (test-instrumentation): nullScan judges only a hand-curated `nilListKeys`; derive the list from Go struct nilable json fields or add spec array types.
- NS-3 (DONE for a/b/c + recursion + internal/handlers, CAT-NS-1; residual (d) `x := f()` passthrough documented) (test-instrumentation): AST guard misses named result + bare return, `return nil, nil`, struct-literal field from an unset var, `x := f()` passthrough; it also skips internal/handlers and handlers/backups and is non-recursive.
- NS-4 (spec): fixed members (analyze, configuration, roles/{id}, files) have no response schema; add array schemas and `nullable: true` for pointer members.
- NS-5 (process-doc): build ldflag has no dirty-tree marker; commits lack `Roadmap-Item` trailers (§11.4.293(A), a repo-wide convention gap).
- NS-6 (observation): GET /entities/{id}/files emits capitalised keys (QualityInfo, Language): an untagged struct is served.

## NS-1/NS-3 review round 1 follow-ups (NS3-F*, minor)

Add a fixture with a clean twin for each probe.
- NS3-F1: detector (a) counts `out = append(out, x)` as an assignment; a named result filled by append in a loop then bare-returned is still nil when empty.
- NS3-F2: a named slice/map result returned explicitly (`return out, nil`) is not covered by any detector.
- NS3-F3: the nil-check rule is too broad (any `x == nil` clears x, even if nothing assigns it, or on a shadowed or closure variable). Count it only when the `if` body assigns x.
- NS3-F4 (false positive): detector (b) checks `return nil, nil` inside a closure against the outer function's result types; skip FuncLit bodies or use their own types.
- NS3-F5: named slice/map types (`type IDs []int`) and positional struct literals `&T{l}` are not detected; add to the census residuals.
- NS3-F6: NS-1 stays provisional until a live HTTP run on a deployed stack shows `types: []` (stack 3 now runs b4d86546+; verify after next rebuild).

## WL13 review follow-ups (Opus round 1, GO-with-follow-ups, reviewed head edd44d7b)

- WL13-R1 (test-instrumentation, minor): `waitMetadataAfterMint` can return early (readyState>=1 already true before reload); also wait for the `src` ticket to change or a loadedmetadata count increment.
- WL13-R2 (test-instrumentation, minor): the F2 test checks only `end.t < t2+10`; also assert playback resumed and advances from seek 2.
- WL13-R3 (process-doc, minor): mutation claims in the evidence README lack raw captured logs; `red_f1_revert_chromium_firefox.txt` is a trimmed excerpt.
- WL13-R4 (process-doc, minor): PROVISIONAL-DONE entries name no live verification job (real catalog-api stream, tickets, target, due, command) per §11.4.290; the tests use the local ticket_server fixture (unit-class), so VERIFIED-DONE needs a live run.
- WL13-R6 (nit): `r.play` in `r.play && wantPlayRef.current` is redundant (equivalent mutant); keep as defence in depth or drop.
- Player controls lack accessible names / test ids (aria-labels would help tests and a11y).

## CAT-CSRF-LAN follow-ups

- CSRF-1 (VERIFIED live 2026-11-04 on stack 3 over plain http LAN URL: csrf-dev cookie, admin approve 200, mismatch 403, see docs/qa/live-web-stack3-20261104/REPORT.md; was provisional): fix proven by real-router tests only; confirm in a browser against the stack-3 QA stack on http://<LAN-IP> (PUT /api/v1/admin/users/:id succeeds) after the next rebuild.
- CSRF-2 (process-doc): stack-3 `up.sh` sets the toggle; production compose files deliberately do not. Add a deploy-config lint that CSRF_INSECURE_DEV is absent from production manifests.
- CSRF-3 (enhancement): serve the QA stack over HTTPS and retire the toggle.

## CSRF-LAN review follow-ups (Opus rounds 1-2, GO-with-follow-ups, landed head 99a94f53)

- CSRF-R2-1 (test-instrumentation, minor): the `main()` call `adminCSRFOptions(os.Getenv, logging.Warnf)` is untested (mutant passing a blank getenv survives); cover with the live CSRF-1 browser check on a rebuilt stack 3 or a startup test with a real env. Item stays PROVISIONAL-DONE until CSRF-1 runs live. **Startup test DONE (security queue): `TestAdminCSRFCookie_RealBinary_EnvToggleReachesMiddleware` starts the real binary three times; both main.go mutants (blank getenv, APP_ENV forced) killed. The live browser check CSRF-1 stays as recorded above.**
- CSRF-R2-2 (nit): `catalog-api/csrf_env_test.go` declares a generic package-level helper `env` and a filler `_ = root_middleware.CSRFInsecureDevEnvVar`; rename/remove. **FIXED (security queue).**
- Residual (accepted for dev-only toggle): without the `__Host-` prefix a sibling-subdomain/HTTP MITM could plant `csrf-dev`; attacker still needs an HMAC-signed pair and a matching header.

## Monitoring deploy review follow-ups (Opus rounds 1-3, GO-with-follow-ups, landed head 63cd1ab2)

- MON-G4 (test-instrumentation, minor): the otel collector's own scrape target and `authorization` block are unguarded (reviewer mutants N5 `nosuchhost`, N6 authorization removed survive; author's m17/m18 cover prometheus.yml only). Add static checks that the otel scrape target resolves to the api service/container name and its authorization uses `${env:METRICS_SCRAPE_TOKEN}`.
- MON-G5 (process-doc, nit): evidence files 02-05 under docs/qa/monitoring-deploy-20261104 lack the exact producing commands; show the LAN opt-in case explicitly in the QA-host section; the unquoted `$PROMETHEUS_EXTRA_FLAGS` expansion can glob (operator-controlled input).
- MON-PROV: the `api` network alias DNS resolution and the full `deployment/docker-compose.yml` stack remain PROVISIONAL (evidence used extra_hosts because podman DNS on the scratch network is broken on this host); verify on a healthy network (needs the shared aardvark-dns repaired, operator decision).

## Live-evidence verifier / recorders: round 3 follow-ups (3f7c9e58, GO-with-follow-ups)

- LE3-B (OPEN): no test spawns a real server with `LD_PRELOAD` (or `PYTHONPATH` for python) from a clean-env test process and asserts the key is recorded and the record is UNIT through the real resolver (reviewer mutants R2 and R3 survive that path).
- LE3-D (OPEN, doc nits): TESTING_GUIDE line 570 wording, `GCONV_PATH` in the schema `env_injected` description, and the F3/F4 label clash between the round-1 live-evidence findings and the older F3/F4 entries in this file (rename the live-evidence ones LE1-F3/LE1-F4).
- LE3-C (DONE in this change): env values containing `=` are covered in `test_env_node.mjs` and `TestEnvOfEveryKey`.

## Live QA round 3 + storage r1 findings (2026-11-04)

- LW3-1 (intermittent, root cause UNCONFIRMED): firefox-desktop run 1 `pages.live.spec.ts:6` Favorites page showed the empty login form although login POST returned 200 (1 of 8 browser runs; 18/18 passes on rerun). Needs a trace captured on a failing run (keep traces on failure in CI-less local runs); leads: 401 handler in lib/api.ts or login/header race.
- ST-D1 (product, low-medium): SmbClient.Connect ignores ctx (net.DialTimeout, smb_client.go ~42-47); SMB and FTP stat/read succeed on an already-cancelled context. Fix: DialContext + ctx.Err() checks. **FIXED, PROVISIONAL-DONE (CAT-ST): evidence docs/qa/storage-clients-fix-20261104/README.md; live Synology confirmation owed.**
- ST-D2 (product hazard, SERIOUS): one FTPClient used from 4 goroutines corrupts its control connection (wrong results, a run hung forever in textproto.ReadResponse with no read deadline). Fix: per-client operation mutex + ctx-tied read deadlines. **FIXED, PROVISIONAL-DONE (CAT-ST): evidence docs/qa/storage-clients-fix-20261104/README.md; live Synology confirmation owed.**
- ST-D3 (inconsistency): SmbClient rejects leading-slash paths while FTP/WebDAV accept them. **FIXED, PROVISIONAL-DONE (CAT-ST): evidence docs/qa/storage-clients-fix-20261104/README.md; live Synology confirmation owed.**
- ST-F5 (follow-up, minor): FTP ReadFile stream Close/Read happen outside the operation turn; a caller using the same client concurrently with an open stream still interleaves. No in-repo concurrent caller found. Fix idea: hold the turn until Close, or one client per stream.
- ST-F7 (FIXED, PROVISIONAL-DONE CAT-ST): smbPath now treats a backslash as a separator before looking for `..` (SMB-only; FTP/WebDAV untouched). Evidence docs/qa/storage-clients-fix-20261104/README.md.
- ST-NFS: NFS live test SKIPPED (Synology export list empty, no mount privilege); needs operator enabling an NFS export.
- Evidence: docs/qa/live-storage-r1-20261104.md.

## Conversion defects D1, D2, D7 (conversion part), D10, D11 (fix-conversion-defects, 2026-11-04)

Status: PROVISIONAL-DONE (§11.4.290), not VERIFIED-DONE: the live re-run on a rebuilt catqa stack is owed (verification job: re-run docs/qa/live-conv-docs-subs-r1-20261104.md D1/D2/D7/D10 against the new image; due on the next stack rebuild).
- D1 FIXED (RED first: `TestD1_ConvertDocument_PDFToTextUsesPDFExtractor` failed with `ebook-convert ... not installed`; GREEN after routing PDF-native targets to the CAT-002 extractor before calibre). `/conversion/formats` is now derived from the tools present (`exec.LookPath`); unsupported pairs are refused at submit with 400 + `reason` instead of a job that can never run. pdf->png is not advertised and is refused (no calibre/libreoffice added to the image; add them only by an operator decision).
- D2 FIXED: DELETE /conversion/jobs/:id really deletes pending/completed/failed/cancelled jobs (+ confined output removal for completed), 409 for running, owner/admin only.
- D7 (conversion part) FIXED: job `error_message` and the failure notification replace absolute paths with `<path>/<basename>`. Subtitle `err.Error()` echo and translate-without-provider 500 remain OPEN (not conversion).
- D10 FIXED: target_path extension must match target_format (jpg/jpeg, tif/tiff, htm/html, txt/text aliases) else 400. The "target directory must pre-exist" part is unchanged (documented, OPEN as spec decision).
- D11 (truncated source reports `completed` with a tiny output): OPEN, not bounded here. Needs ffmpeg `-xerror`/stderr "Invalid data" detection plus a `warning` field on the job model and migration. D11 sidecar-subtitle import is a separate (subtitles) item.
- CONV-F1 (OPEN): `convertPDFToImage` (go-fitz) is unreachable through the API now; remove with the go-fitz drop decision.
- CONV-F2 (OPEN): existing rows keep their old unsanitised `error_message`; no backfill.
- Review round 1 (Opus NO-GO) addressed in the follow-up commit: F1 symlink-target delete (acts on the job's own regular file only), F2 foreign job ids answer 404 for GET/cancel/delete/retry/download, F3 routing by `documentRoute` (docx/odt->pdf go to calibre), F4 dispatch on canonical target_format, F5 path sanitiser handles spaces/C:\\/file://, F6 delete conditional on the observed status, F7 mutants killed (logs in docs/qa/conversion-defects-fix-20261104/), F8 docs.
- pdf->html wording correction: pdf->html is advertised only when pandoc or LibreOffice is present, although the plain-text-wrapped HTML fallback in `convertPDFToHTML` works without them. OPEN: advertise html unconditionally (truthful, the output is text wrapped in escaped HTML, no layout) once the operator confirms that is acceptable.
- CONV-F3 (OPEN): `converterPresent` calls exec.LookPath on every format request/validation; cache with a short TTL if profiling shows cost.
- CONV-F4 (OPEN): check the web client handles DELETE /conversion/jobs/:id answering 409 for a running job (previously it cancelled).
- Residual (documented): between the last leaf check and os.Remove a parent directory could be swapped for a symlink; closing it needs unlinkat on a directory fd.
- Review round 2 (bd9d2ada): N1 FIXED (path after `:`, `=`, `(`, `[`, `,` and quoted paths containing `: ` are stripped; RED-first, two mutants killed). Tracked, open:
  - N2 (nit): the sanitiser over-removes text that follows a path with no delimiter; deliberately the safe direction.
  - FU-1: permission inconsistency between viewing a job (view:media) and delete/cancel/retry (manage:users): an administrator may be able to delete a job it cannot view. Decide one rule.
  - CONV-F3 (LookPath caching), CONV-F4 (web client handling of DELETE 409 on a running job) and the parent-directory-swap residual of removeConversionOutput remain open as listed above.
- Review round 3 (8cc5b709): R3-1 FIXED (a path after an unclosed quote, e.g. a tool message cut at 2048 bytes inside a quoted path, is stripped; RED-first, mutant killed). Documented limits of the sanitiser, accepted (the server only produces absolute paths): an unquoted `error:/srv/My: Movies/x.mp4` keeps the part after `: `; relative `../x`, `~/x`, UNC `\\server\share`, percent-encoded `%2F` and `//srv//double` forms are not covered; over-stripping after a delimiter-less path is N2.


## D9 admin role change (2026-11-04) - PROVISIONAL-DONE

- D9 (product bug): `PUT /api/v1/admin/users/:id` parsed `role` but never applied it (200 "User updated successfully", role unchanged); `GET /api/v1/admin/users` showed every user as `user` because `UserRepository.List` does not join the role and the handler defaulted to "user". Fix in `catalog-api/handlers/admin_handler.go`: role names resolved from the roles table (case-insensitive, unknown -> 400 generic), persisted via compare-and-set `UpdateGuarded`, sessions revoked on role change, list resolves real role (fail closed, no silent default), self-demotion and last-active-admin demotion -> 409, audit event `admin_user_role_changed`. Tests: `handlers/admin_user_role_d9_test.go` (real gin router + migrated SQLite). Evidence: `docs/qa/d9-admin-role-fix-20261104/`.
- Status PROVISIONAL-DONE: the live re-run (qa_admin listed as `admin`, PUT role on a rebuilt stack) is owed; not yet verified live.
- Honest limits: the JWT already minted still carries its old `role_id` claim but the admin gate reads the live role from the DB (`IsAdmin`) and the sessions are revoked; the 'viewer' role in the web type has no seeded role (only Admin/User exist). The last-admin count reads up to 100000 users and is racy across processes (the CAS update only guards the target row, not the admin count). The Update path of the repository does not persist `is_locked` (UNCONFIRMED, noticed while reading, not tested: `Update`/`UpdateGuarded` SQL has no `is_locked` column), out of D9 scope.

### D9 review round 1 follow-ups (2026-11-05)

- Last-admin protection is now atomic: `UserRepository.UpdateGuarded` and `Delete` carry the "another active, unlocked administrator exists" condition inside the single write (SQLite; on PostgreSQL the same write runs in a transaction holding `pg_advisory_xact_lock` - UNCONFIRMED, no PostgreSQL run was made here). Covers admin PUT demote/deactivate, `PUT /users/:id` (role_id/is_active) and `DELETE /users/:id`; `repository.ErrLastAdmin` maps to 409. Not covered (noted): `LockAccount` after failed logins, and a permission change of the admin role itself (`UpdateRole`). A refused last-admin write may already have revoked the target's sessions (revoke runs first, fail closed).
- CAT-D9-SUSPEND (important, security, PRE-EXISTING, separate item, status Queued): `PUT /api/v1/admin/users/:id {"status":"suspended"}` returns 200 but is a silent no-op: `UpdateGuarded`/`Update` SQL has no `is_locked` column, `IsActive` stays true and no sessions are revoked, so the "suspended" user keeps working. Reproduce: PUT status=suspended on a normal user, then GET /admin/users shows status active and the user's token still works. Not widened into D9. (Probe output: docs/qa/d9-admin-role-fix-20261104/probe_suspend.txt.)
- Nit: role names differing only by case resolve to the first row of `ListRoles`; role names are not unique-case-insensitive in the schema.
### Storage clients fix: Opus round-4 follow-ups (GO-with-follow-ups, 2026-11-04)
- ST-R4-1 (test-instrumentation, minor): make the GetFileInfo clock-compare race deterministic (ctx whose Err() is nil after its deadline passed) to kill mutant K1.
- ST-R4-2 (test-instrumentation, minor): test where Connect or a stream Close installs a session while redial is in flight; assert exactly one live connection, no leak (kills K3).
- ST-R4-3 (source-defect, nit): Connect does not clear redialNeeded (harmless today because of the client==nil guard); clear it and add a test (kills K5).
- ST-R4-4 (source-defect, nit): reconnectIf discard path calls fresh.Quit() without c.ctl.Delete(fresh), leaking one map entry per lost race (ftp_client.go ~307-309).
- Still open: ST-F5, live Synology FTP/SMB confirmation, SMB real-server test (mid-flight cancel through the context-bound share unproven).

## Subtitle defects D5-D8 + delete route (live conv/docs/subs r1, branch fix-subtitle-defects) - PROVISIONAL-DONE

Status: PROVISIONAL-DONE (§11.4.290). Reproduced first on a real generated media file through the real router and the production sqlite schema (RED: the +7 s subtitle and the in-sync one both returned `is_valid:true, sync_offset 0, confidence 0.8`; unknown media id returned 200; upload for an unknown media id returned 500 `SAVE_ERROR`; binary garbage and a plain-text file were accepted with 200). GREEN is the same tests after the fix. Live re-run on a rebuilt stack is OWED (verification job: re-run docs/qa/live-conv-docs-subs-r1-20261104.md subtitle scenarios against a stack built from this change; it must show `status:measured` with the real offset on the generated media).

- D5 (HIGH) fixed: `verify-sync` now parses SRT, VTT and ASS, takes the media duration from ffprobe, extracts the speech-activity of the real audio with ffmpeg `silencedetect` and estimates the subtitle delay by normalised cross-correlation (+-120 s, first 30 min). Result fields added: `status` (`measured` / `undetermined` / `invalid`), `method`, `reason`, `media_duration`. `sync_offset` (ms) is positive when the subtitle appears late. An ambiguous or weak alignment, a media file that is not locally readable, or audio with no activity variation yields `undetermined` with offset 0 and confidence 0 (never the old constant 0.8). Cues beyond the media end are `invalid`. Unknown media id -> 404 `MEDIA_NOT_FOUND`; a subtitle that belongs to another media item -> 404 `SUBTITLE_NOT_FOUND`.
- D6 fixed: upload for an unknown media id -> 404; content that is not valid UTF-8 text or has no cues in the stated format -> 422 `INVALID_SUBTITLE_CONTENT` (CRLF files accepted); upload now returns the real `subtitle_id` (it was a random string that matched no row).
- D7/D8 fixed (subtitle part): error bodies no longer echo `err.Error()` (list, verify, translate, search, download, bad request); translate with no provider -> 503 `TRANSLATION_UNAVAILABLE`, unknown subtitle -> 404; translated output is written as SRT. NOT in this change: the conversion-job `error_message` path leak and the input-validation middleware rejecting empty JSON bodies (rest of D7/D8).
- Delete route: `DELETE /api/v1/subtitles/:subtitle_id`; uploader or admin; rows without a recorded uploader (legacy, downloaded) admin-only; 401/403/404/204. New migration v24 adds `subtitle_tracks.uploaded_by` (nullable). Route table, OpenAPI and API_ENDPOINTS updated.
- Tests: `handlers/subtitle_defects_test.go` (live-class: real router, production schema + migrations, ffmpeg-generated media with a known irregular audio-burst pattern, subtitle files shifted by known offsets), `internal/services/subtitle_sync_test.go`. Mutations killed: upload status mapping reverted (404 and 422), verify returns constant 0.8, delete authz dropped, unknown-media check removed, translate 503 -> 500.
- Review round 1 (Opus, NO-GO on cce139ad) addressed in the follow-up commit: F1 bounded verify-sync (slot semaphore, 429 `SYNC_BUSY`, shared in-flight runs, per-file result cache keyed on path+mtime+size, `-nostats -threads 1`, `nice`); F2 media resolution only under a LOCAL storage root, confined to it (no `..`, no symlink escape), `file:`-prefixed inputs, `pipe` dropped from the protocol whitelist; F3 reviewer tests adopted, real route table extracted (`RegisterSubtitleRoutes`, called by main.go) and tested, main.go wiring AST check (source-class, labelled as such), end-to-end ambiguity test on strictly periodic audio; F4 delete invalidates cached translations (index key) and translate checks existence before the cache; F5 reason wording, non-owner delete -> 404; F6 case naming; F7 PG migration v24 run result in docs/qa/subtitle-defects-fix-20261104/. Evidence: docs/qa/subtitle-defects-fix-20261104/.
- SUB-F1 (open, minor): the audio-activity estimator needs speech-like on/off structure; music-only or continuously loud audio yields `undetermined` by design. Remote (SMB/FTP/WebDAV) media is not copied for analysis, so those report `undetermined` with `reason`.
- SUB-F2 (open, minor): D11 (scan does not import sidecar subtitles) is untouched.

### Subtitle defects: Opus round-2 follow-ups (GO-with-follow-ups, 2026-11-04)
- SUB-H1 (test-instrumentation, minor): test the path part of the analysis cache key (two different media files, equal size and equal mtime).
- SUB-H2 (source-defect, minor): fairness: one user can hold both global verify-sync slots; add per-user cap/quota.
- SUB-H3 (source-defect, minor): failed analyses are cached 10 minutes like successes; cache failures with short TTL or not at all.
- SUB-H4 (source-defect, nit): translation index read-modify-write without lock can drop a language from the index.
- SUB-H5 (test-instrumentation, nit): invalidation test uses an in-memory cache; exercise the real DB-backed CacheService and assert the index key itself is removed.
- SUB-H6 (process-doc, minor): docs/api/API_ENDPOINTS.md still has old DELETE (403) and verify-sync (no 429) wording.
- SUB-H7 (process-doc, nit): PostgreSQL v24 evidence is the author's transcript only; reviewer did not independently re-run.

## D3 pdf-pages: poppler-only rendering, go-fitz removed (2026-11-04, PROVISIONAL-DONE, live re-run owed)
- Defect: `GET /entities/:id/pdf-pages[/:n]` on a malformed PDF answered 500 with `open pdf document: fitz: cannot open document`, and the PDF was parsed in the API process by go-fitz (MuPDF, cgo, AGPL) outside the CAT-002 isolation.
- Root cause (proven by RED, `docs/qa/d3-pdf-pages-fix-20261104/red_handlers*.log`): the handler opened the buffered file with `fitz.New` in-process and mapped every failure to 500 with `err.Error()` as `details`; no page-size bound existed (the 14400x14400 pt page test hung/OOM-killed the old code).
- Change: page count (`pdfinfo`) and page render (`pdftoppm -png -singlefile`, ZERO-based page index kept) run through `catalog-api/services/pdf_page_render.go` using the CAT-002 `runLimited` (prlimit AS/CPU/FSIZE, no shell, allowlisted env, timeout, bounded output); absolute paths only (no option injection); output goes to a private `catalogizer-pdfpage-*` temp dir that is always removed (pdftoppm cannot write PNG to stdout, so `runLimited` gained an optional `fileSize` limit, default still `--fsize=0`). Handler status map: invalid/malformed/zero/truncated/non-PDF 422, encrypted 422, page bitmap over 64 Mpx or PDF over 1 GiB 413, page out of range 404, bad index 400, reader slots saturated 429 (+Retry-After; `services.PDFReaderSlots`=2, fail-fast), client cancel 499 (log only, not a 504), tool timeout or CPU limit 504 (a child ended by SIGXCPU/SIGKILL; prlimit sets soft=hard so the kernel sends SIGKILL), output over its file/stdout bound 413, poppler/prlimit missing 503, anything else 500; bodies are generic, parser detail is logged only. `go-fitz` (and its indirect `purego`, `jupiterrider/ffi`) removed from `go.mod`/`go.sum`; `convertPDFToImage` kept (still reachable through `convertDocument`; history: introduced in 610c00a5 "Auto-commit") but its body now uses the same poppler renderer, so branch `fix-conversion-defects` may conflict on `services/conversion_service.go` (one import line, one function body, two comments).
- Review round 1 fixes: `-cropbox` added (pdfinfo reports the CropBox, pdftoppm defaulted to the MediaBox: 9000 pt MediaBox + 100 pt CropBox rendered 18750 px, RED in `red2_cropbox.log`); one overall 60 s deadline; poppler children run with `LC_ALL=C LANG=C`; per-tool address-space limit now sized for conversion slots + reader slots.
- Follow-ups (not done): (a) no cache of the buffered PDF, so each page view re-copies the document (bounded by 1 GiB and by the 2 reader slots); a short-TTL cache keyed on entity+mtime+size would remove it. (b) `BookReaderScreen.kt` / `CatalogizerApi.kt` map non-200 responses generically; map 413/422/429/503/504 to user messages (Kotlin not edited beyond comments). (c) CGO/mupdf build packages in `catalog-api/Dockerfile*` are UNCONFIRMED removable; needs a build without them.
- Behaviour notes: poppler output dimensions equal the former render at 150 dpi (200 pt page = 417 px); PNG bytes differ from MuPDF output. `poppler-utils` and `prlimit` are now required at runtime for the reader endpoint (the API image already ships both for CAT-002).
- Limits: unauthenticated 401 is unchanged (route group middleware not touched) and was not re-driven in this change; the Android TV reader and real SMB/FTP/WebDAV/NFS storage were not re-run; the process isolation bounds memory/CPU but the PDF is still parsed by poppler (C++), so a poppler memory-safety bug is contained to a child, not removed. Live re-run owed on the next stack (Android TV book reader over SMB, malformed upload, 401 check).
- Evidence: `docs/qa/d3-pdf-pages-fix-20261104/` (MANIFEST.txt with base sha, timestamp, sha256 of each log).
- Review round 2 follow-ups (D3, not done, no code): (R2-3) per-user fairness / rate limit for the 2 reader slots (one user can hold both and 429 everyone else). (R2-4) when RLIMIT_AS is hit inside poppler the handler reports 422 (invalid PDF) instead of 413. (R2-5) Android TV `BookReaderScreen.kt` `bookErrorForHttp` does not map 429/413/422/503/504 and never retries 429; add Retry-After backoff. (R2-6, nit) `Dockerfile.dev` may still install mupdf-dev/libffi (UNCONFIRMED); `pdfMaxBufferBytes` is a mutable package var only so tests can lower it.
- Review round 2 tests added: `TestD3_SlotHeld_WhileWorkRuns` (blocking source holds both reader slots; third request gets 429 + Retry-After; after release 200; ListPdfPages and GetPdfPage) and `TestD3_PDFRender_AddressSpaceSizing` (stubbed MemAvailable; avail/(2*(conversion+reader slots)), floor, ceiling). Mutants N1 (release right after acquire, both handlers) and N4 (divisor reverted) are killed; N1 kills show as a test-timeout panic (hang), N4 as an assertion failure.

- D3-R3-1 [test-instrumentation, minor, open]: TestD3_SlotHeldWhileWorkRuns kills mutants only through `go test -timeout` (third request blocks on the gate; gate not closed on t.Fatal paths, goroutines leak). Run the third request in a goroutine with a time.After bound, assert 429 within seconds, and close the gate via sync.Once in a defer.

- Review round 4 (8023431a): NEW-1 FIXED (root `TestConversionToolsMissing_RealBinary` now asserts the D1 contract: 400 + `reason` for an unsupported pair, no audio/video/image formats advertised without their tools, /health/deep converter flags kept; evidence `red-root-conversion-tools.log`, `green-root-package.log`). The old "failure names the missing tool" leg is no longer reachable through the API (an unsupported pair is refused at submit); the tool-naming error text is still produced by `runConverterTool` for a stored job whose tool vanished after submit and has no end-to-end test: OPEN, test-instrumentation minor.
- Registered follow-ups (all open): sanitiser limits (unquoted `error:/srv/My: Movies/x.mp4` keeps the part after `: `; relative `../`, `~/`, UNC `\\server\share`, `%2F`, `//srv//double` not covered; N2 over-stripping; a 2048-byte cut inside a private directory name keeps the cut piece); FU-1 (view:media vs manage:users for other users' jobs); CONV-F3 (LookPath caching); CONV-F4 (web client and DELETE 409 on a running job); parent-directory swap window in output delete.
- Pre-existing on main a61d56c6, NOT from this change (test-instrumentation, minor): root `TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions` expects 500 for DELETE /api/v1/logs/share/999999 and gets 404.

- D9-F1 [source-defect, minor, open]: LockAccount (admin /users/:id/lock and auto-lock after failed logins) is not covered by the last-admin guard; a brute-force attacker can temporarily lock the only admin. Add a guarded LockAccount (409 when target is last usable admin).
- D9-F2 [process-doc, minor, open]: PostgreSQL branch of the last-admin guard (tx + pg_advisory_xact_lock) UNCONFIRMED; needs a live PostgreSQL concurrency run.
- D9-F3 [nit, open]: refused last-admin write has already revoked the target's sessions (fail closed); revisit ordering.

## D4 document full-text search (CAT-D4, PROVISIONAL-DONE until a live rerun on stack 3)

Reproduced first (RED, `docs/qa/d4-doc-search-fix-20261105/red_before_fix.txt`): on the real server binary with a real local corpus, `GET /search/files?q=` found a file by name (control needle) but returned 0 for words inside a `.txt`, `.md` and `.pdf` (root cause: `applySearchFilters` matched only `f.name` and `f.path`; no step ever extracted document text into the database). Fix: migration v26 `file_text_index`; `services.DocumentTextIndexer` (txt/md bounded read, pdf through the isolated CAT-002 extractor) run in the background after each completed scan via `UniversalScanner.SetPostScanIndexer`; `q` also matches the extracted text. Verification: `doc_search_binary_test.go` (real binary) + `services/document_text_indexer_test.go`; 8 mutations killed (`mutations.txt`).
Open items:
- D4-1: only `local` storage roots are indexed; smb/ftp/nfs/webdav roots need a client-based fetch to a temp file before extraction.
- D4-2: only pdf, txt, md. docx/odt/epub/html and OCR for scanned PDFs are not indexed (no tool in the image).
- D4-3: matching is a `LIKE '%q%'` substring over the whole text (no ranking, no stemming, ASCII-only case folding in SQLite, no snippets); an FTS5/tsvector index is the scalable replacement. Scan cost grows linearly with stored text (cap 2 MiB per file).
- D4-4: the indexer runs only after a scan; there is no on-demand re-index API and no backfill for files scanned before this change until the next scan of the root. Extraction failures are recorded as `status=failed` (no text) and retried only when size or modified time changes.
- D4-5: Postgres: migration v26 and the dialect upsert are written for both dialects but only SQLite was run live here.
- D4-6: live rerun on stack 3 with the D4 fixtures (`ZEBRA-4242`, `OKAPI-1357`) is owed; until then the item is PROVISIONAL-DONE, not VERIFIED-DONE.
- D4-7 (F7): the content match is an unindexed `LIKE '%q%'` over up to 2 MiB per document, evaluated for every file row; scale threshold to revisit: roughly 10k indexed documents or search p95 above 500 ms, then FTS5/tsvector.
- D4-8 (F8): LIKE wildcards (`%`, `_`) in `q` are not escaped (pre-existing for name/path, now also for content); nit.
- D4-9 (F10): Postgres `LIKE` is case-sensitive while SQLite folds ASCII case, so the same query can match differently per dialect; use ILIKE/lower() when the FTS work lands.
- D4-10 (F6 residual): file reads go through one verified descriptor; the PDF child reopens that same inode via `/proc/<pid>/fd/N`, so a path swap after the open cannot redirect it. Not covered: a Linux-only `/proc` dependency (the indexer fails closed with status failed elsewhere).
- D4-11 (F4/F5): fixed (F4 logs a tool-absent skip of the PDF leg; F5 does not persist missing-tool or cancelled runs as failed). Open: a `failed` row from a corrupt file is retried only when size or mtime changes.
- Migration number: file_text_index is migration v26 (v25 is reserved for conversion_jobs.warning on fix-d11-d7).
- D4-12 (round 2): the indexer holds at most one PDF reader slot globally (docIndexerGate), so interactive readers keep one of the two slots; the pure-Go child divisor in pdfChildLimits counts the indexer's slot. A per-file timeout is recorded failed and retried only on size/mtime change.
- D4-13 (R3-2, minor): a per-file timeout caused by host load is stored as failed and the file is skipped until its size/mtime changes; wanted: scheduled retry of failed rows or a capability-change recheck (under D4-4).
- D4-14 (R3-3, nit): a pass that ends with a DB error while a coalesced request is pending drops that request (`again` needs e==nil); the next scan covers it.

## D11a, D11b, D7/D8 remainder (fix-d11-d7, 2026-11-05) - PROVISIONAL-DONE

Status: PROVISIONAL-DONE (§11.4.290), not VERIFIED-DONE: a live re-run on a rebuilt catqa stack is owed (verification job: re-run docs/qa/live-conv-docs-subs-r1-20261104.md D11 (truncated mp4 -> webm; Movie.Name.2020.1080p.{en.srt,en.vtt,en.ass,out-of-sync.srt} then a library scan) and D8 (anonymous POST /conversion/jobs/:id/cancel with Content-Type: application/json and an empty body) against the new image; due on the next stack rebuild). Evidence: docs/qa/d11-d7-fix-20261105/ (RED before, GREEN after, mutants killed, MANIFEST with sha256). Roadmap-Item CAT-D11 (a, b) and CAT-D7 (c).

- D11a FIXED (commit "flag a truncated source"): a truncated source no longer ends `completed` silently. After an audio/video job the output duration is compared with the duration the source container declares (hardened ffprobe); output < 90 % and >= 1 s shorter sets `warning` on the job (new nullable column conversion_jobs.warning, migration v25, model, repository, OpenAPI `ConversionJob.warning`/`error_message`). Chosen: completed-with-warning, not failed, because a usable (partial) output exists and failing would destroy a recoverable result; `-xerror` (exit 183 on any decode error) was measured and rejected as too strict. Limits: a header that already matches the truncated payload, or a duration only estimated from file size (some MP3/MPEG-TS), is not detected; an unreadable source/output gives no warning; images/documents are not checked.
- D11b FIXED (commit "library scan imports sidecar subtitle files"): after a successful scan of a LOCAL root, srt/vtt/ass/ssa sidecars in the same directory as a video (longest matching base name wins) are stored as `subtitle_tracks` rows, source `sidecar`, path = catalog path, language from a dot tag else `und`, `forced` tag honoured; confined to the resolved root (symlink escape skipped), cue-validated, UTF-8 only, idempotent, changed content refreshed. Closes SUB-F2. Limits: remote roots are not imported (would need a client read path); non-UTF-8 sidecars are skipped; a removed sidecar is not deleted from the table; the confinement helper `confinedRegularFile` duplicates the loop in `resolveMediaFile` (candidate for one shared helper); embedded (in-container) subtitle streams are not extracted.
- D7/D8 remainder FIXED (commit "an empty JSON body is not malformed"): root cause: the global input-validation middleware runs before authentication and `json.Unmarshal` of an empty body failed with "invalid JSON format". An empty or whitespace-only JSON body now passes the middleware (nothing to validate); anonymous callers get 401 from authentication, and a body-requiring handler still answers 400 from its own binding. Non-empty malformed JSON and injection payloads are still refused by the middleware. Limit (open, D8-F1): a NON-empty malformed or injection body from an anonymous caller still gets 400 before 401 because validation precedes authentication in the chain; moving validation after auth for protected groups is a router-order change that was not widened here.
- D11-F1 (open, minor): backfill nothing for jobs completed before v25 (they have no warning). 
- D11-F2 (open, minor): the truncation check runs two ffprobe calls per audio/video job inside the job's semaphore slot; cache or fold into the existing run if profiling shows cost.
- D11-F3 (open, minor): web client does not render `warning` yet (API only).
- D11-F4 (open): PostgreSQL migration v25 (`ADD COLUMN IF NOT EXISTS`) was not run against a real PostgreSQL here; the SQLite path is covered by the production-migration tests (UNCONFIRMED on PostgreSQL).

- PRE-EXISTING (not this branch): root TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions expects 500 for DELETE /api/v1/logs/share/999999, handler returns 404 (fails identically on base 40c07e60, see baseline_openapi_contract_test.txt). Needs separate fix.

### D11/D7 review round 1 follow-ups (fix-d11-d7)
Fixed in round 1: F1 race-safe sidecar read (O_NOFOLLOW|O_NONBLOCK open, fstat on the descriptor, SameFile vs the checked stat, /proc/self/fd realpath re-check); F2/F5 tests (FIFO, swap, BOM, forced); F3 subtitle save logs content length + sha256 only; F4 nil logger safe; F6 whitespace-prefixed XSS body control.
Registered, NOT code:
- F7: an admin-deleted sidecar subtitle track is re-imported on the next scan (no tombstone). Sidecars reached through symlinked directories are never imported (the scanner does not catalog them).
- Product-owner awareness: anyone with write access to a scanned LOCAL library can now inject subtitle text via a sidecar without upload permission (openapi `GET /subtitles/media/{media_id}` documents the `sidecar` source).
- F8 (D11a limit): the warning can false-positive when the source container's declared duration exceeds the converted stream (audio extracted from a video whose audio is shorter, dropped data/subtitle streams).
- Leaf O_NOFOLLOW alone is redundant with the SameFile + fd realpath checks (single-layer mutant survives by design; the combined mutant is killed).

- D11-R2-2 [process-doc, minor, open]: openapi GET /subtitles/media/{media_id} lists source as `upload`, embedded/extracted or `sidecar`; code writes `uploaded`, `downloaded`, `translated`, `sidecar`. Correct the list.
- D11-R2-3 [test-instrumentation, nit, open]: no test covers the readlink-failure branch of descriptorInsideRoot (fails closed today).
- D11-R2-4 [process-doc, nit, open]: on Linux without /proc mounted every sidecar is refused (only trace: Skipped count in the Info summary); add to honest limits.
- D11-R2-5 [source-defect, nit, open]: sidecarAfterCheck test seam is a mutable package global; pass as option/parameter.

## Live verification of stack 3 (2026-11-05, main d9238b47) - evidence docs/qa/stack3-rebuild-live-20261105/

54 PASS / 0 FAIL / 1 SKIP per round, 3 identical rounds; Yandex WebDAV and Synology SMB/FTP read-only suites 3/3. Promoted to VERIFIED-DONE on this evidence: D1, D2, D3, D4, D5 (measured on speech-like media), D6, D7, D8, D9, D10, D11a, D11b, subtitle DELETE route, storage-client live suites (SMB/FTP/WebDAV).
Still open:
- LV3-1: cross-user conversion-job isolation for a non-privileged user is a SKIP (registered plain users get 403 on every conversion route; no API grants the permission). Second admin sees other users' jobs (by design, FU-1).
- LV3-2: storage-client idle-deadline and cut-recovery scenarios not run against real Synology (loopback/unit only); Synology NFS skipped topology_unsupported (no exports).
- LV3-3: D5 on the stock corpus returns `undetermined` (continuous tone, no silences) - add a bursty-audio fixture to gen_media_fixtures.sh.
- LV3-4: stack-3 DB still lists removed fixture rows (rescan does not delete rows for removed files).
- LV3-5: multi-process last-admin race on PostgreSQL not exercised (D9-F2).

## Security queue fix (fix-security-queue, 2026-11-05)

Closed or advanced: SSRF-F1..F6, AZ2-3, P2-g (log redaction half), CSRF-R2-1/R2-2, R4-1 (rotation half); each with a RED test that failed on main (or a surviving mutant), a root cause, and killed mutants; evidence in `docs/qa/security-queue-fix-20261105/`. All PROVISIONAL-DONE until a live rerun on a rebuilt stack. Open, security-relevant, NOT done here (each needs an operator decision or a larger change):
- SECQ-1 (= W5, important, operator decision): the WebDAV STORAGE-ROOT client (`filesystem/webdav_client.go`, built by `filesystem/factory.go`) has no outbound guard, unlike the sync-endpoint client (`netguard.Transport()`, private ranges refused unless `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` allows them). Only an administrator can create a root, so this is admin-to-internal reach, not a user SSRF. Applying `netguard.Transport()` would refuse a LAN NAS root by default: deciding between the sync policy (allow-list env) and a documented exemption for storage roots is an operator decision; implementation is one `SetHTTPTransport` call plus a loopback/private RED test through `POST /storage/roots` + a scan.
- SECQ-2 (= WD-F1, UNCONFIRMED cause, possible data loss): needs the live probe under evidence capture; not reproducible offline, so no fix attempted (the reproduce-first rule forbids guessing).
- SECQ-3 (= AZ2-5): DONE. RED: `middleware/pprof_guard_test.go` (real `net/http/pprof` handlers, real server with the shipped 900 s WriteTimeout) showed `?seconds=600` and malformed `?seconds=abc` held instead of refused, and a second concurrent window accepted. Review round: the same hold exists on the delta profiles (`allocs`, `block`, `goroutine`, `heap`, `mutex`, `threadcreate` with `?seconds=N`); RED per route added. Fix: `middleware.PprofSamplingGuard(60)` on the pprof group, behind RequireAuth and RequireAdmin (wiring asserted by `pprof_wiring_test.go`): on /profile, /trace and ANY pprof path carrying `seconds`, it must be an integer 1..60 else 400 (fractions such as 0.5 are refused, documented), absent seconds on profile/trace bounded to the stdlib default capped, one sampling window at a time across all those paths else 429; plain snapshots untouched; boundary cases cap and cap+1 tested. Residual: the 60 s cap is a constant in `main.go`, not configurable.
- SECQ-4 (= SC7): PLAYLIST PATH DONE, COLLECTION PATH OPEN. RED: `services/playlist_smart_cap_race_test.go` (real PlaylistService on real encrypted SQLite, 16 concurrent creates from cap-1) stored 59 to 65 smart playlists against a cap of 50 in 5 of 5 rounds. Root cause proven: count then insert with no atomicity. Fix: per-user mutex held across count and insert in `PlaylistService.CreatePlaylist` (GREEN: 1 success, 15 refused, 50 stored in 5 of 5 rounds). The lock is in-process only (documented in the code), so a multi-instance or Postgres deployment still needs a DB-level guarantee (conditional insert or unique constraint); the rule evaluation now runs outside the lock. The collection path is tracked as SECQ-4b below.
- SECQ-4b (= SC7, collections, DONE in-process 2026-11-05; PostgreSQL/multi-instance DB-level form still OWED): RED `handlers/collection_smart_cap_race_test.go` stored 58 smart collections vs cap 50 (round 0, 8 of 16 got 201). Fix: per-owner mutex held across count and insert in `CollectionHandler.CreateCollection` (rule evaluation outside the lock); mutation (unlock right after the count) makes the test FAIL (52 > 50). Original text: `CollectionHandler.CreateCollection` has the same count-then-insert cap (`CountSmartByOwner` then `Create`, `MaxSmartCollectionsPerUser` = 50) and is NOT reproduced or fixed. RED plan: a real-router test in `handlers` over a real encrypted SQLite database with `MediaCollectionRepository` (same construction as `realPlaylistService` in `services/playlist_smart_realdb_test.go`), an authenticated actor at 49 smart collections, 16 concurrent POSTs behind a start barrier, assert the stored count never exceeds 50 and exactly one is 201 (expect it to fail like the playlist case did, 59 to 65). Fix options: the same per-owner mutex around count and insert (single process) and/or a conditional INSERT in the repository; the multi-instance guarantee needs the DB-level form.
- SECQ-5 (= P2-g remainder): DONE. RED: GET `/smb/discover` and `/smb/test` with `password=` in the query were processed (400/panic path, parameters read). Fix: both GET handlers now answer 410 Gone without reading any parameter, the response never echoes the secret (tested with a marker password); POST forms unchanged; `scripts/run-full-qa.sh` migrated to POST (+ an assertion that the GET form answers 410); web client `smbApi.discoverGet` and its tests removed; Website API doc updated. Routes stay registered behind the admin gate so old clients get an explicit 410 rather than 404. Spec (`docs/api/openapi.yaml`: deprecated, 410), the contract exception list and the stale docs were updated in the review-fix round. Residual: any generated swagger under `catalog-api/docs` is regenerated by the swagger tooling, not by hand; access-log redaction (`logredact`) is unchanged and still covers `password`.
- SECQ-6 (= AZ2-4): DONE (test-instrumentation). `services/logout_scope_secq6_test.go`: real AuthService on real SQLite; logout of one token leaves another session of the same user and another user's session live, and a token signed with another secret revokes nothing. It passes on the unchanged code (characterization, no defect); paired mutation (Logout calling DeactivateAllUserSessions) makes it FAIL. Residual: `internal/handlers/auth.go` `Logout` slices `authHeader[7:]` without a length check (a header shorter than 7 bytes panics); that handler is not the one routed in main.go (`LogoutGin` is), not changed.
- SECQ-7 (honest limit): the SOCKS5 fix bounds the handshake with a fixed 30 s deadline; a proxy that answers slowly but steadily is not detected.
- SECQ-8 (functional limit, from SSRF-F5): the cover-art demuxer allow-list (same as the converter's, pinned by `TestCoverArtDemuxers_MatchConverterHardeningList`) excludes mxf, dv and nut, so thumbnails of such files are no longer produced: the ffprobe duration step falls back to the file-size estimate, but the ffmpeg frame step returns an error (the scan loop logs a warning and skips the item). Add a demuxer to BOTH lists together if a library needs it.
- Round 1 review of this branch (Opus): F1 deadline test for `generateAndSaveVideoThumbnail`, F2 `apikey` test, F3 non-regular files skipped before ffprobe/ffmpeg (`requireRegularFile`; a missing path now returns an error from `getVideoDuration` instead of the 120 s estimate), F5 `;` treated as a pair separator in `logredact`, F6 caller context threaded into `getVideoDuration`, F4/F7/F8 done. The timeout tests now use PATH shims for ffprobe/ffmpeg (unit class: a regular file cannot block the real tools); the real-tool behaviour stays covered by the playlist/URL tests.

## NEW-1 cover thumbnails (landed 2026-11-05, review round 1 GO-with-follow-ups)

PROVISIONAL-DONE: generate-thumbnails fix (four root causes, RED-first, SQLite + PostgreSQL real-path tests). Live GREEN on a rebuilt stack 3 is OWED (same POST, same signature as `docs/qa/new1-cover-thumbnails/red_run1.txt`); until then it cannot be VERIFIED-DONE.
- NEW1-F2 (test-instrumentation, minor): no test for the remote-protocol branch (`copyRemoteFileToTemp`, its `MkdirAll`) nor for `rows.Err()`.
- NEW1-F3 (source-defect, minor, PRE-EXISTING, path confinement): `cover_art_service.go` ~1273-1290 `resolvedPath := t.videoPath` fallback when protocol is not local and `fsFactory` is nil, or a local root has an empty root path; `files.path` containing `..` escapes the root via `filepath.Join`; a relative path starting with `-` is read as an option by ffprobe. Fix: refuse remote paths without `fsFactory`, add a root-containment check, use `./` or `--` guard.
- NEW1-F4 (source-defect, minor, pre-existing): query does not filter `mf.is_primary` (item with several video files processed per file); a failing `setDefaultCoverArt` only warns yet counts as generated, so the item is regenerated on every run.

## SECQ-1 WebDAV storage-root SSRF guard (landed 2026-11-05, review round 2 GO-with-follow-ups)

PROVISIONAL-DONE (unit + real-listener + handler tests; no live run against the NAS/Yandex yet). Follow-ups:
- SECQ1-F1 (nit, accepted): a direct dial to an address equal to an earlier proxy address skips the destination check (`proxySet.dial`); optional hardening: only bypass for addresses that are not operator-blocked.
- SECQ1-F2 (minor, documented): with a proxy, the proxy re-resolves the name, so DNS rebinding between check and proxy lookup is not covered; the control must be the proxy itself or running without one.
- SECQ1-F3 (nit): GCP IPv6 metadata address `fd20:ce::254` is not in `operatorMetadataNets` (AWS `fd00:ec2::254` is); address UNCONFIRMED, verify against Google docs.
- SECQ1-F4 (minor, availability): `resolveOperator` refuses the whole answer if any address is blocked; an mDNS name answering with both 192.168.x.x and fe80:: would fail a LAN NAS root. Decide strict vs filter (dial is pinned, so filtering is equally safe).
- SECQ1-F5 (minor, test-instrumentation): netguard package tests do not prove `OperatorTransport` applies the dial check (mutant M9 only killed by consumer tests) and nothing kills the redirect-host-check mutant; add a test driving `OperatorTransport` against an unallowlisted loopback listener.
- Live check owed: WebDAV root create + scan on the LAN NAS and Yandex WebDAV through the product path.

## SECQ-3/5/6 (landed 2026-11-05, review round 2 GO-with-follow-ups; flaky-test fix verified by RED -count=10 4 failures, GREEN 0)

PROVISIONAL-DONE until a live rerun on a rebuilt stack 3. Pre-existing, NOT caused by this branch: `DELETE /api/v1/subtitles/{id}` contract violation in the live contract harness (spec answers 404 to a non-owner; the harness expects 403 because the 403 description matches `adminGate403`; the happy path has no seeded subtitle or exceptions row). Fix belongs in the harness or spec wording (tracked SUB-CONTRACT-1). SECQ-4b (CollectionHandler.CreateCollection same count-then-insert race) remains open.

## Stack-3 verification pack follow-ups (landed 2026-11-05, review round 3 GO-with-follow-ups)
- STK3-FU-1: TRUSTED_PROXIES remote-LAN XFF behaviour UNCONFIRMED (QA-MON-FU-4): measure from a second LAN host.
- STK3-FU-2: secrets visible in `podman inspect` accepted risk (QA-MON-FU-3).
- STK3-FU-3: pprof phase B not re-run (needs HELIX_PPROF_ENABLED=true restart) — PROVISIONAL.
- STK3-FU-4: reviewer mutations R7/R8/R13 not wired into a harness; horizontal authz VERIFIED only on 2 routes, others PROVISIONAL.

## NEW1-F3/F4 landed 2026-11-05 (review round 2 GO-with-follow-ups)
- NEW1F3-FU-1 (nit): resolveLocalVideoPath joins before ConfineFile, so pathguard's post-symlink `..` check never fires; safe because the resolved path is what ffmpeg/ffprobe receive. Add a one-line comment.
- NEW1F3-FU-2: check-to-use race between ConfineFile and ffmpeg open (acknowledged in code comment; needs write access inside the media root).
- NEW1F3-FU-3: external cover caching path's setDefaultCoverArt still ignores failures (out of scope of F4).
- Remote-path confinement via fsFactory was not reviewed.

## SECQ-1 follow-ups and SUB-CONTRACT-1 (branch fix/secq1-followups)
- SECQ1-F3 DONE: `fd20:ce::254` added to `operatorMetadataNets`; Google documents it as the Compute Engine IPv6 metadata server (https://docs.cloud.google.com/compute/docs/metadata/querying-metadata). Single address, the /32 is not refused.
- SECQ1-F4 DONE: `resolveOperator` now FILTERS (direct path: drops refused addresses, dials only permitted pinned ones, refuses when none remain; proxy path: STRICT, any refused address refuses the name since the proxy re-resolves). Loopback/metadata-only answers still refuse.
- SECQ1-F5 DONE: tests drive `OperatorTransport` and `OperatorCheckRedirect` directly; mutants (every dial as proxy dial, redirect host check removed, filter reverted, GCP net removed) each killed.
- SECQ1-F1 NOT CHANGED (deliberate): bypassing only for non-blocked proxy addresses would break the supported local proxy on 127.0.0.1 (itself blocked), and a dial cannot tell proxy from direct; residual stays documented.
- SUB-CONTRACT-1 DONE: handler answers 404 to a non-owner by design (ownership masking); the spec's 403 text matched `adminGate403`. Spec 403 wording now describes the fail-closed role-lookup failure only; the happy path (no seeded subtitle, 404) has an `api_contract_happy_exceptions.tsv` row. Live contract sweep RED (2 violations) then GREEN.
- Review round 1 (Opus NO-GO, fixed): R1 proxy path strict (test `reviewer_proxy_mixed_test.go`, RED hits=1 then GREEN).
- OPEN: dead handler branch `ErrSubtitleForbidden` in `DeleteSubtitle` (the service never returns it); kept per section 11.4.124, not removed.
- OPEN: DELETE subtitle happy path has no seeded subtitle (exceptions row only); seed one in the harness and drop the row.
- OPEN: F4 filter has no proxy-path coverage beyond the strict mixed-answer test (no test of a permitted-only answer forwarding through a proxy with a mixed-capable resolver on the direct path).

## SECQ-1 follow-ups landed 2026-11-05 (review round 2 GO-with-follow-ups)
- SECQ1FU-1 (test, minor): reviewer tests TestReviewer2_ProxyMappedMixedRefused / _ProxyAllPermittedForwarded / _DirectStillFilters were overlay-only and are NOT landed; land them (mapped/GCP/NAT64 mixed answers through a proxy; an all-permitted DNS name forwarded through a proxy). The "proxy coverage" line above should read: no landed test sends an all-permitted DNS-name answer through a proxy (TestF1 covers only an IP literal).

## SECQ-4b landed 2026-11-05 (review GO-with-follow-ups)
- SECQ4B-FU-1 (test, minor): race test is timing-based (reviewer mutation survived 1 of 4 runs; fresh-mutex-per-call mutant caught only in round 4 of 5). Make deterministic with a test-only hook between count and insert, or more rounds.
- SECQ4B-FU-2 (doc, minor): the SECQ-4b entry is marked DONE but keeps the old "NOT reproduced or fixed" text in the same bullet; mark as history.
- SECQ4B-FU-3 (nit): lock held while c.JSON writes the 201 (own-user delay only); optional explicit release after repo.Create.
- SECQ4B-FU-4: DB-level cap (PostgreSQL / multi-instance) still owed; mutex is single-process.

## NEW-1 live re-verification (2026-11-05, stack 3 image main-5c480072)
- NEW-1: generated=2 then 0 (idempotent), cover_art rows video_thumbnail/is_default=true x2 on PostgreSQL; evidence docs/qa/new1-cover-thumbnails/green_main5c48.txt. VERIFIED on this stack for the batch endpoint. Still owed: SSRF-F2..F5 through the cover-art product path, SECQ-5 POST flows, pprof delta routes, SECQ-1 via POST /storage/roots; 10 stale video fixtures (files absent) should be reseeded or pruned.

## Stack-3 live re-verification 2 (2026-11-05, image main-5c480072; evidence docs/qa/stack3-live-20261105/)
- VERIFIED live: SECQ-1 (loopback/localhost/link-local/fd20:ce::254/::ffff:127.0.0.1 -> 400, RFC1918 -> 201), SECQ-5 (GET 410, authz 401/403), SECQ-4/4b (51 creates at 16 concurrent -> exactly 50x201 + 1x409, 3 rounds, both resources), subtitle DELETE non-owner 404 with row intact.
- SECQ-3 pprof: UNCONFIRMED (404 everywhere since HELIX_PPROF_ENABLED unset; gate and 60 s guard need a flagged restart).
- No delete API for storage roots (DELETE /storage/roots/{id} 404): QA cleanup needed direct SQL; consider tracking as API gap (STK3L2-FU-1).
- POST /smb/discover with credentials to unreachable host returns 500 on i/o timeout rather than a 4xx/504 (STK3L2-FU-2, minor).

## Stack-3 live browser pass (2026-11-05, image main-5c480072; evidence docs/qa/stack3-ui-20261105/)
VERIFIED: GET /entities/types 200 from UI; 11 app pages render real data; website 47 pages / 31 internal URLs OK; theme toggle; no overflow except D5.
FAIL (open, dispatched as UI-D1..D8): D1 HIGH /conversion crashes (TypeError reading 'name' in ConversionTools); D2 /identities calls missing GET /api/v1/identities (404); D3 /entity/35 (media-item id) 404s and spins forever; D4 /ws closes ~4x/25s idle -> red Disconnected badge; D5 /admin horizontal overflow at 360px; D6 a11y critical (78 unnamed buttons /media, unlabeled selects, 1 unnamed button /ai); D7 colour-contrast serious + svg-img-alt; D8 header search results unfiltered (UNCONFIRMED). Not covered: player subtitles panel. Note: reviewer agent printed QA-stack JWT_SECRET/METRICS token into its own session output (QA-only; rotate with next stack recreate).

## Stack-3 browser pass follow-ups registered (2026-11-05)
- UI-D1-FU-1 (F4): conversion cancel uses DELETE; API route is POST /conversion/jobs/:id/cancel.
- UI-D1-FU-2 (F5): startConversion sends nested shape vs ConversionRequest snake_case; warning/progress/duration not mapped.
- UI-D2-FU-1: identity create/update/delete and /discovery/* routes missing in API; Edit/Delete shown for read-only identities.
- UI-D2-FU-2: non-admin identities empty-state wording misleading.
- UI-D3-FU-1: queries retry on 404 for similar/duplicates/files; no API file-id -> entity-id lookup.
- UI-FAV-1 (IMPORTANT, pre-existing): favourites store file id as entity_id and link /entity/${media.id} (wrong entity).
- UI-D4-FU-1: /ws idle closes UNREPRODUCED; re-check on rebuilt stack. UI-D4-FU-2: WebSocketContext effect keyed on user identity causes login double-connect (key on user?.id/isAuthenticated). UI-D4-FU-3 (nit): server randomString connection ids near-constant.
- UI-D7 (contrast, svg-img-alt), UI-D8 (header search unfiltered, UNCONFIRMED): open.
- SECQ-3 pprof: re-run with HELIX_PPROF_ENABLED=true after rebuild.

## UI-D2/D3 landed (reviewed GO-with-follow-ups, round 2/5; PROVISIONAL until live rerun)
- UI-D3-FU-2 (nit): parseInt('5abc') opens entity 5; use strict ^\d+$ id check.
- UI-D3-FU-3 (nit): after "Try again" the alert stays with no loading indicator during refetch (react-query v4).

## UI-D1 landed (reviewed GO-with-follow-ups, round 2/5; PROVISIONAL until live rerun on rebuilt web image)
- UI-D1-FU-3 (minor): "File is ready for download" shown for every completed job without checking output exists; job `warning` not shown (ties to FU-2).
- UI-D1-FU-4 (nit): completed job with empty id/target_path still shows Download (requests /conversion/jobs//download); require job.id.
- UI-D1-FU-5 (nit): mis-indented closing `})` in ConversionTools.liveshape.test.tsx status-mapping test.

## UI-D7/D8 landed (reviewed GO round 2/5; PROVISIONAL until live rerun of d7_axe.cjs / d8_repro.cjs on rebuilt web image)
- D8 fixed (Header ?search= -> ?q=). D7 fixed for Layout version label, ActivityFeed, Dashboard (light+dark).
- UI-D7-FU-1: remaining contrast: /ai badges (green-600 3.29, yellow-600 2.93) and tabs (dark 3.66, 2.82), /admin dark (23 nodes), /media card text 2.53, /analytics dark labels (Analytics.tsx 381-412) - owned by UI-D5/D6 files; pie svg-img-alt handled by D5/D6.
- UI-D8-FU-1: share one named constant for the search query param between Header.tsx and EntityBrowser.tsx.
- UI-D8-FU-2: e2e/tests/critical-flows.spec.ts:352 asserts search= with `|| true` (cannot fail).
- UI-D8-FU-3 (nit): no test for leading/trailing-space trim. UI-D7-FU-2 (minor): contrast helper may wrongly throw on text-current/inherit/transparent and text-[13px].

## UI-D5/D6 landed (reviewed GO-with-follow-ups round 3/5; PROVISIONAL until live axe rerun docs/qa/ui-d5d6-20261105/axe_live.cjs on rebuilt web image)
- UI-D5D6-FU-1 (minor): test rendering two SmartPlaylistBuilders on one page asserting unique ids (useId-drop mutation survived).
- UI-D5D6-FU-2 (nit): Playlists test name still wrong (checks page-level media-type filter, Playlists.tsx:509); selectLabelsSource id regex misses an id as last attribute before `>` (false alarm only).
- MP-FLAKE-1 (minor, pre-existing): MediaPlayer.test.tsx "seek after ticket near expiry gets a fresh ticket" fails only under full-suite load (35/35 alone x3); needs root cause / quarantine (§11.4.248).
- WEB-DEPS-1: borrowed node_modules lacks @vasic-digital/* packages: 8 test files fail to load (Avatar, EmptyState, LoadingSpinner, AuthContext.integration, module-registry, websocket, WebSocketContext*) - verify on a real npm ci in the web image build.
- UI-D7-FU-1 now includes AIDashboard/AdminPanel/MediaCard/Analytics dark contrast (owners: D5/D6 files).

## Android QA boot landed (reviewed GO-with-follow-ups round 1/5; evidence docs/qa/android-boot-20261105/)
VERIFIED: both debug APKs build in rootless JDK17 container; phone (API34) and TV (API34) AVDs boot headless with KVM; login screens render. UNCONFIRMED: qa_admin login on emulator (needs secret-safe path), LAN access to emulator display from a second machine.
- AND-FU-1: persist debug keystore (adb install -r fails after rebuild). AND-FU-2: documented build-time way to supply/skip Firebase for androidtv (google-services.json placeholder is gitignored, Firebase disabled). AND-FU-3: real login test on emulator via secret-safe path. AND-FU-4: verify LAN display access (SSH tunnel + scrcpy). AND-FU-5: 11.4.173 remote-build-host offload not done.
- AND-FU-6 (minor, pre-existing source-defect): LoginScreen shows "Connected: <url>" even when server unreachable (serverConnected=true on both branches, LoginScreen.kt:273-289); README line + tracked item.
- AND-FU-7 (nit): build_apk.sh set -euo pipefail, id -u/id -g not hardcoded 1000, builder image digest, companion doc; trim 3.1K lines of logs (image-build.log).
- Disk used by Android work ~20 GB (SDK system images 9.8G, gradle 3.3G, AVDs 2.4G, builder image 2.35G): prune pending operator approval.
## Tauri QA boot / S3 follow-ups registered (branches under review)
- TAURI-FU-1: desktop client vs API drift: /api/v1/smb/configs, /smb/status, /auth/logout return 404. 
- S3-FU-1 (F8): read/write have no operation timeout; OperatorTransport lacks ResponseHeaderTimeout. S3-FU-2 (F10): rename tracker/ProtocolHandlerFactory and catalog-web SUPPORTED_PROTOCOLS lack s3. S3-FU-3: live MinIO/Garage evidence; verify copy-source encoding on a real server; SeekableClient/multipart/STS unsupported.

## Tauri QA boot review follow-ups (feat/tauri-qa-boot, GO-with-follow-ups round 2/5, PROVISIONAL)

- TAURI-FU-2 (process-doc, minor): `docs/qa/tauri-boot-20261105/dockerfile_hash_checks.txt` negative control used a badly formatted hash (format error, not mismatch); re-run with a full-length wrong hash and fix the label. A proper control was checked by the reviewer (FAILED, exit 1).
- TAURI-FU-3 (test-instrumentation, minor): no automatic guard that production `dist/` contains no `__QA_IPC_CALLS__` / `qa-tauri-ipc-shim`.
- TAURI-FU-4 (test-instrumentation, nit): vite QA host defaults (127.0.0.1) have no test.
- TAURI-FU-5 (source-defect, pre-existing, TRACKED): Rust `starts_with` URL prefix check in `main.rs` make_http_request accepts `http://a.com.evil` for `http://a.com`; fix with parsed-host comparison (RED-first, security-adjacent).
- TAURI-FU-6 (documented gaps): apt pinning, GPG verification of publisher hashes, committing Cargo.lock.
- TAURI-FU-7 (UNCONFIRMED): no image rebuild, no desktop rebuild with login fix, no native boot, no VNC run, no wizard binary yet.

## Live UI rerun 2026-11-05 on stack 3 (main-1db4760f) — follow-ups

Evidence: docs/qa/ui-live-rerun-20261105/ (N=3 identical sha256 per check). PASS: D5/D6 axe, /admin 360px, D8 search, D1 /conversion, D2 /identities, D3 /entity/:id. D7 whole-check FAIL: remaining violations light /ai 2, light /media 24, dark /analytics 5, dark /admin 23, dark /ai 8 (fix in review as feat/d7-remaining-contrast f03b911d; re-run after rebuild).
- UI-D3-FU-4: on a not-found entity, /entities/:id/duplicates and /recommendations/similar/:id are retried 3x each in background (404); gate retries on 404.
- UI-D5-FU-3: /admin 360px: a tab button right edge 405 inside the scrolling tab bar (document itself does not overflow) — decide if acceptable.
- HELIXQA-FU-1: web Vision bank + CDP wiring for stack 3 (UI leg of §11.4.278); validated vision profile needs host capacity (llmctl capacity: does_not_fit while llmctl-onnx@decide-nli runs uncapped, load ~19) — operator decision.
- HELIXQA-FU-2: upstream helix_qa `--admin-pass-file`/env credential option; stale docker-compose.qa.yml / scripts/run-helixqa-web.sh defaults (8080/3000).

## D7 remaining-contrast follow-ups (review round 2, GO-with-follow-ups)
- D7-R2-F1 (important, pre-existing): class-wide dark-mode text-gray-500/600 without dark variant (77 static strings; e.g. MediaDistributionChart empty text 3.04 dark, CollectionsManager 4 items 3.04, description 1.94). Resolve real ancestor backgrounds, drive with whole-element walker. Not yet fixed.
- D7-R2-F2: contrastD7Shared should use the whole-element walker (land with R2-F1).
- D7-R2-F3: contrast helper should throw on conflicting same-prefix colour classes (fail closed).
- D7-R2-F4: CollectionsManager test uses button index; use accessible label.
- D7-F3: hover/focus contrast and icon-only toggles.
- D7-LIVE: live axe rerun on rebuilt web image owed (PROVISIONAL until then).

## HelixQA stack3 wrapper follow-ups (review round 3, GO-with-follow-ups)
- HQA-R3-1 (nit): record logout HTTP code in run.log/verdict.txt.
- HQA-R3-2 (minor, process-doc): build stamp is hand-written HEAD; use `go version -m` or `git describe --dirty`.
- HQA-R3-3 (nit): record image digest, not only tag.
- HQA-R3-4 (nit): QA_ENV/WEB_URL defaults are host-specific.
- HELIXQA-FU-1/2 still open: web Vision bank + CDP leg (not implemented; PARTIAL verdict), upstream --admin-pass-file.

## Tauri SSRF (TAURI-FU-5) follow-ups (review round 4, GO-with-follow-ups)
- TAURI-F14 (nit, pre-existing): perform_request returns body regardless of status, no size cap (memory exhaustion by malicious server).
- TAURI-F15 (nit): one tauri::test IPC test for make_http_request and set_server_url (mutant O4 survives).
- TAURI-F16 (nit): test server single 2048-byte read; read until headers+Content-Length.
- TAURI-F6: broader trust model (confirm on server URL change) needs operator decision.

## S3 protocol follow-ups (review round 2, GO-with-follow-ups; unit-class evidence only, LIVE PENDING)
- S3-R2-1: SkippedKeys() not wired to scanner/logs (skipped objects uncatalogued silently).
- S3-R2-2 (F8): ReadFile/WriteFile lack per-operation/response-header timeouts.
- S3-R2-3 (F10): rename tracker ProtocolHandlerFactory and web SUPPORTED_PROTOCOLS lack s3; UI/docs for s3 roots.
- S3-R2-4 (nit): fake stripChunked no-op; no test with >1000 delete ids (N2 survivor).
- S3-R2-5: live run needs a real S3 server: verify copy-source encoding and DeleteObjects checksum compatibility (MinIO/Ceph/Garage) - UNCONFIRMED.

## S3 live (Garage) follow-ups (review round 2, GO-with-follow-ups; live evidence on Garage v1.0.1 only, PROVISIONAL)
- S3-LIVE-FU-1: rerun suite against AWS S3 or a stricter server (MinIO/Ceph RGW/second impl) to kill survivors M2 (1001-key delete batch) and MC ('+' unencoded in CopySource); also virtual-host addressing, TLS, >5 GiB objects.
- S3-LIVE-FU-2: MH (ListDirectory keeping names with '/') judged equivalent by reviewer; document definition in README.
- Incident record: author's RED run of old down.sh purged the real disposable state dir (test data only; keys rotated) - see docs/qa/s3-live-20261105 incident record.

## Tauri confirm-on-URL-change follow-ups (review round 3, GO-with-follow-ups; PROVISIONAL until native boot)
- TAURI-F6 (important to verify): the real native DialogConfirmer was never shown on a display; live check on a desktop session with recording + OCR (dialog appears, Cancel/close return false, frontend cannot dismiss, blocking_show from spawn_blocking works on GTK). Mutant R5 (always approve) survives until then.
- TAURI-F7: src-tauri/Cargo.lock is gitignored, dependency versions float (§11.4.246); track the lock.
- TAURI-F9 (minor, functional): server URLs `https://a.com/api/` (trailing slash kept -> `/api//api/v1`), `http://a.com/?x=1` and `#f` produce broken request URLs. Trim all trailing '/', reject query/fragment, assert no '//' anywhere.
- TAURI-F10 (minor): N9 survives - record `in_force` in the Scripted confirmer and assert; adopt reviewer probe P6 (/home/milosvasic/.cache/claude-tmp/rv_tauriconf.Op6VZR/reviewer_probes_r3.patch).
- UX: config is in-memory so every launch prompts the first-time dialog; persist a trusted URL (and last_confirmed_url).

## D7 class-wide contrast (landed PROVISIONAL, GO-with-follow-ups round 2/5)
- D7-R3-FU1: FormatConverter job error/success boxes are light islands in dark mode (add `dark:bg-*-900/20 dark:text-*-300`).
- D7-R3-M1 (test-instrumentation): connecting/disconnected/error badge colours unprotected (mutations N2/N3 survived); render one integration per status in contrastD7Populated.
- D7-R3-M3 (visual): UploadManager queued row `dark:bg-gray-800` probably blends into Card; consider `dark:bg-gray-700/50`.
- Live axe rerun on rebuilt web image (N>=3) still owed; PERFORMANCE/MediaFilters/Playlists fixes not render-tested.

## Live UI rerun 2026-11-05 (image main-17ec3c1f; CAT-UI-AXE-LIVE-1, CAT-STACK3-REBUILD-1, CAT-PPROF-LIVE-1, CAT-PROM-LAN-1)
- VERIFIED (3/3 identical machine evidence, evidence docs/qa/ui-live-rerun-20261105-b): D7 contrast 9 pages x light/dark 0 violations; D5/D6 names + 360px admin; D8 header search; D1 /conversion; D2 /identities; D3 /entity (background 404 retries on missing entity remain, order differs only); pprof gate 401/401/200.
- Not exercised: pprof 60 s profile cap, non-admin 403, cover-art SSRF F2..F5, TRUSTED_PROXIES from a second LAN host. Prometheus /api/v1/targets answers without token on the LAN (only API /metrics needs the scrape token) - decide if Prometheus itself needs auth (new item CAT-PROM-AUTH-1).
- Cleanup owed: qa.env.bak.pre-rotate in catqa3 dir (delete after confirm). Subagent printed unrelated env keys into its own tool output once; operator may rotate (not in repo/evidence).

## D7 follow-ups landed (PROVISIONAL; GO-with-follow-ups round 1/5)
- D7-R4-1 (source-defect minor): UploadManager.tsx:260 failed-row error text `text-red-500` no dark variant: 3.60 light / 3.29 dark. Fix `text-red-700 dark:text-red-400` + error-row test (pre-existing).
- D7-R4-2 (test nit): ExternalIntegrations test assigns global.fetch without restore; use vi.stubGlobal/afterEach.
- D7-R4-3: "pending" badge dark:bg-gray-800 equals card bg (shape invisible, text passes).
- D7-R4-4: contrast helper dark fallback to light bg-*-50 lets islands pass; add class-wide check. Decorative status icons unmeasured (WCAG 1.4.11 3:1).
- Live check on rebuilt web image (includes these fixes) owed (CAT-UI-AXE-LIVE-1 rerun).

## Prometheus auth landed (CAT-PROM-AUTH-1; PROVISIONAL; GO-with-follow-ups round 2/5)
- PROM-R2-1 (test minor): test_prom_auth_grafana.sh `nosecret` mutation kills by crashing Grafana (provisioning error, ~4 min wait), not by auth failure; use wrong-password secret (401) and fail fast if Grafana exits.
- PROM-R2-2: flaky test_scripts.sh root cause UNCONFIRMED (stdin-drain removal did not reproduce in 6 runs); timeout raised 60->180 s may mask it - keep tracked, do not call fixed.
- PROM-R2-3/4/5 (nits): verify.py vs up.sh user precedence wording; validate PROM_AUTH_USER before sed use in up.sh; --memory limit on grafana test containers.
- Live: up.sh never run end to end until conductor applies (monitoring/up.sh, then verify.py + both tests). Basic auth over plain HTTP on LAN is sniffable.

## Prometheus auth applied live (2026-11-05 22:15 CET; CAT-PROM-AUTH-1, CAT-PROM-LAN-1)
- Applied: `GRAFANA_BIND=PGADMIN_BIND=PROM_BIND=192.168.1.115 monitoring/up.sh` (catqa3mon replaced). Live: anonymous /api/v1/targets -> 401; verify.py: prom.anonymous_targets/query/wrong_password 401 PASS, targets up, queries have data, grafana datasource health OK; test_prom_auth.sh PASS (single run).
- verify.py 45 PASS / 3 FAIL: the 3 FAILs are the legacy assertions `lan.{prometheus,grafana,pgadmin}_NOT_reachable_on_LAN_IP` which contradict the operator LAN-exposure decision; make them conditional on the *_BIND settings (new item CAT-VERIFY-LANBIND-1). Prometheus password in mon.env (PROM_AUTH_PASSWORD, user qa_prom).
- N>=3 repeat of the live auth checks owed for VERIFIED.

## D7 R4 follow-ups landed (feat/d7-r4, PROVISIONAL-DONE; live axe re-run on a rebuilt web image owed)
- D7-R4-1 FIXED: UploadManager failed-row error text `text-red-700 dark:text-red-300` (light 3.60 -> >=4.5; dark 3.29 -> >=4.5; red-400 gave 4.478 on the dark row so red-300 is used). Test renders a FAILED upload row in both themes.
- D7-R4-2 FIXED: global.fetch restored in afterEach (contrastD7FollowUps/Class/Populated, ExternalIntegrations.test) with a leak-check test.
- D7-R4-3 FIXED: ExternalIntegrations default/pending chip `dark:bg-gray-700/50`; test asserts chip bg != card bg and ratio > 1.05 in dark.
- D7-R4-4 DONE: `islandsOf()` in src/test/contrast.ts; `failingElements(root, theme, min, { islands: true })` opts in. Goldens in contrastHelper.test.ts. Opt-in only because it finds REAL islands in untouched files (dark theme), listed here as UI-D7-R4-ISLAND-1..: AdvancedSearch tab icon chips `bg-blue-100` (x5); FormatConverter empty-state circle `bg-gray-100`; AI unavailable panels `bg-white rounded-lg shadow p-6` (x3); AIDashboard icon chips `bg-indigo-100`/`bg-green-100`/`bg-yellow-100`. Toggle knobs (small white circles) are exempt by rule.

### D7-R4 review round 1 follow-ups (GO-with-follow-ups)
- D7-R4-FU-1 (minor, test-instrumentation): `islandsOf` knob exemption too broad (no bg-white check, one w/h class suffices); require bg-white + both w/h 1-6 + no text; add goldens `rounded-full w-16 h-16 bg-gray-100` and text badge `rounded-full h-6 px-2 bg-blue-100` as positives. Mutation M4 survives today.
- D7-R4-FU-2 (minor): `LIGHT_BG_NAME` ignores -200 shades, `bg-white dark:bg-white`, `dark:bg-opacity-*`; widen/document.
- D7-R4-FU-3 (nit): no fetch leak check in ExternalIntegrations/Class/Populated tests (M7 survives); reset mock history.
- D7-R4-FU-4 (nit): give each UI-D7-R4-ISLAND row an id (register as CAT-D7-ISLAND-1).

### TRUSTED_PROXIES hardening follow-ups (round 2 GO-with-follow-ups)
- TP-FU-1 (DONE, CAT-FOLLOWUPS-UNIT-1): `warnTrustedProxies(logger, tp)` extracted from main.go (behaviour unchanged) and tested with an observer logger (catch-all, auto, quiet near-misses; 4 mutants killed).
- TP-FU-2: live bring-up of root and deployment compose with new fixed subnets/ip_range (only rendered + throwaway compose proved); `catalogizer/web` image built from catalog-web/Dockerfile is assumed; deployment WS_URL port 8081 vs /ws upstream 8080 mismatch (pre-existing).
- TP-FU-3: F4 capture API log line ClientIP=169.254.1.2; stack script leg B needs second LAN host (= QA-MON-FU-4).

## Tauri native QA review (2026-11-10, Opus: GO-with-follow-ups, 0 source-defects)
- TAURI-R1-FU-1 (test-instrumentation/minor): test_wizard_capabilities.sh checks permissions and window "main" separately across all JSON; parse each capability and require one capability targeting main with both permissions and no excluding `platforms` (reviewer mutations M6, M7 survive).
- TAURI-R1-FU-2 (test-instrumentation/minor): guard import detection matches only single-line single-quote imports; handle double quotes, multi-line, `import *`, dynamic import.
- TAURI-R1-FU-3 (test-instrumentation/minor): wire test_wizard_capabilities.sh into a runner/pre-build sweep (nothing calls it).
- TAURI-R1-FU-4 (process-doc/minor): docs/qa/tauri-boot-20261105/README.md links to docs/qa/tauri-native-20261106/README.md which does not exist; add it from the evidence archive (secret-scan first).
- TAURI-R1-FU-5 (process-doc/minor): out-of-scope edits lack `Out-of-scope:` trailers; Roadmap-Item CAT-TAURI-NATIVE-1 covers them.
- TAURI-R1-FU-6 (process-doc/minor): installer-wizard/src-tauri/gen/schemas/capabilities.json tracked as `{}`; regenerate-and-commit or untrack after §11.4.124 investigation.
- TAURI-R1-FU-7: GREEN live run of rebuilt wizard (same Save Configuration sequence, same target) still owed; item stays PROVISIONAL-DONE until then.
- TAURI-R1-FU-8 (nit): guard header should say exit 2 = BLIND.

## Follow-ups worked on feat/followups-20261110 (PROVISIONAL-DONE; live runs owed)
- D7-R4-FU-1/2/3 DONE: `islandsOf` knob exemption now needs `rounded-full` + `bg-white` + w-1..6 AND h-1..6 + no text; `LIGHT_BG_NAME` includes -200 shades; a `dark:bg-*` that is itself light (`dark:bg-white`) or `dark:bg-opacity-*` no longer hides an island; fetch-leak tests in contrastD7Class/Populated/ExternalIntegrations (+ default mock history reset). Mutations killed (see commit body).
- D7-R4-FU-4 DONE: island rows registered as ids CAT-D7-ISLAND-1..3 in WORKABLE_ITEMS_REGISTER.md: -1 AdvancedSearch tab icon chips `bg-blue-100` (x5), -2 FormatConverter empty-state circle `bg-gray-100` + AI unavailable panels `bg-white rounded-lg shadow p-6` (x3), -3 AIDashboard icon chips `bg-indigo-100`/`bg-green-100`/`bg-yellow-100`. (Grouping by component is the author's choice.)
- CAT-VERIFY-LANBIND-1 DONE in code: `verify.py` derives the exposure expectation (env, else `podman port`, else loopback) via `lanbind.py`; LAN-bound Prometheus/Grafana/pgAdmin are asserted LAN-reachable + anonymous-rejected; PostgreSQL/Redis asserted loopback-only. Unit-tested (`test_lanbind.py`). Live re-run of verify.py owed: stack 3 was DOWN when this was authored (no containers, ports closed), so the live RED could not be re-captured.
- CAT-FRONTDOOR-LOCALMODE-1 DONE: `--interface <ip>` mode + README note; sandbox test `test_frontdoor_interface.sh`. A live run against stack 3 is owed (stack down).
## Tauri follow-ups review (2026-11-10, Opus: GO-with-follow-ups; security-adjacent minors fixed before landing)
- TAURI-R2-FU-1: guard ignores .toml/.json5 capability files and tauri.conf.json enabled-capability list / window label; add parsing + mutation cases.
- TAURI-R2-FU-2: comments/strings containing a dialog import count as usage (conservative); `ask`/`confirm`/`message` not covered.
- TAURI-R2-FU-3: RESOLVED (2026-11-11, operator decision): catalogizer-desktop/src-tauri/gen/schemas/capabilities.json untracked and src-tauri/gen/schemas/ added to catalogizer-desktop/.gitignore. The other three tracked files in that directory (acl-manifests.json, desktop-schema.json, linux-schema.json) stay tracked; .gitignore does not affect tracked files.
- TAURI-R2-FU-4: pre-flight-check.sh must run from repo root (fails closed 127 elsewhere); consider cd to root.
- TAURI-R2-FU-5 (nit): `Out-of-scope:` trailer on c504d197 is a left-alone note, not a closed-list reason.
- FU-5/FU-7 of review 1 (GREEN live wizard run) remain owed; CAT-TAURI-NATIVE-1 stays PROVISIONAL-DONE.

## followups-20261110 review (Opus: GO-with-follow-ups; security-adjacent minors fixed before landing)
- FU20-1: live verify.py run with declared *_BIND on rebuilt stack 3 (CAT-VERIFY-LANBIND-1 PROVISIONAL).
- FU20-2: live test_clientip_frontdoor.sh --interface run; UNCONFIRMED whether a second local IP lands in a separate bucket given pasta aliasing (CAT-FRONTDOOR-LOCALMODE-1 PROVISIONAL).
- FU20-3 (PARTLY DONE, CAT-FOLLOWUPS-UNIT-1): alpha cutoff 50 (own bg and dark:bg, opacity-N) and knob size boundaries pinned in contrastHelper.test.ts; fractional knob sizes (h-3.5) are an explicit known gap `FU20-3b` (it.todo; likely false-positive islands, product decision). STILL OPEN: the leak-check conditional-restore mutant (L1) survives.
- FU20-4 (process-doc): commit 9928be7f carries Roadmap-Item D7-R4-FU-1 only; it implements FU-1..4 (history not rewritten).
- Live axe re-run on rebuilt web image still owed (D7-R4).

## lanip-dyn review (2026-11-10, Opus GO-with-follow-ups; S1-S4,M1-M4 fixed before landing)
- LIP-FU-1: live run of test-servers up.sh (real cert swap) + strict-TLS FTPS client; remove stale TS_HOST line from untracked testservers.env.
- LIP-FU-2 (nit): private-key .bak files accumulate in tls/ (mounted into containers) — add retention/cleanup.
- LIP-FU-3 (nit): if default route moves to ppp/VPN, detection picks VPN address; consider preferring non-ppp interface or warn.
- LIP-FU-4 (DONE, CAT-FOLLOWUPS-UNIT-1): stack3/lib.sh now uses qa_lan_ip from docs/qa/lib/lan_ip.sh (adds QA_LAN_IP override and assigned-address check; any failure -> empty = trust nothing); covered by test_scripts.sh 7/7b with a stub that also answers `ip -4 -o addr show`.
- LIP-FU-5: pre-existing: rclone `--pass $TS_PASS` is on container argv (visible in host ps); use obscured env/config file.
- LIP-FU-6: monitoring/lanbind.py fixtures keep .115 (test data only).

## smb-subdir / tauri round-2 notes (2026-11-11)
- TAURI-R2-FU-3: RESOLVED (2026-11-11, operator decision): catalogizer-desktop/src-tauri/gen/schemas/capabilities.json untracked and src-tauri/gen/schemas/ added to catalogizer-desktop/.gitignore. The other three tracked files in that directory (acl-manifests.json, desktop-schema.json, linux-schema.json) stay tracked; .gitignore does not affect tracked files.
- SMB-FU-1: RESOLVED BY REMOVAL (2026-11-11, operator decision, commit Roadmap-Item CAT-SMB-SUBDIR-1): root-package handlers/download.go and handlers/copy.go, CopyToSMB / ListSMBPath / GetSMBHosts and the orphans they alone kept alive (catalog-api/smb SmbClient + SmbConnectionPool, internal/smb resilience package) were deleted with their tests. Evidence of unreachability is in the commit message; history is in docs/FOLLOW_UPS_SMB_FU1_FU2_INVESTIGATION.md.
- SMB-FU-2: the dead copies of this gap (root handlers, SmbClient) are RESOLVED BY REMOVAL (2026-11-11). The live browse/test paths in internal/services/smb_discovery.go were already confined by openConfined (smb.SplitShare + ResolveSub) in the SMB discovery confine work; no open code path mounts the request-body Share unconfined.
- SMB-FU-3 (optional): routed DownloadHandler test (needs a CatalogService seam).
- SMB-FU-4: catalog-web form hint for entering share/sub/dir.
- SMB-FU-5: live run of docs/qa/smb-subdir-20261110/run.sh against the Anton share (never executed yet).
- SMB-FU-6 (test-instrumentation): routed copy test does not exercise sub-root traversal (pathsafe decides first); service-level tests cover it.
- SMB-FU-7: ResolveSub rejects ':' via sentinel segment; CreateRemoteDir stays public (interface) and unconfined by contract.
- SMB-FU-8: live SMB subdir tests unverified; CAT-SMB-SUBDIR-1 stays PROVISIONAL until docs/qa/smb-subdir-20261110/run.sh passes N>=3.

- SMB-FU-5/8 RESULT (2026-11-11): live run executed on cattsrv-smb (share corpus, sub corpus/nested), 3/3 identical PASS for TestLiveSMBSubdir + TestLiveSMBServiceSubdir, negative control (zeroed sha) FAILs as required. Evidence: docs/qa/smb-subdir-20261110/live-run-20261111T121104Z/. CAT-SMB-SUBDIR-1 is VERIFIED for that target; CAVEAT: the original Anton `Public/Testing` target was not run (tests now env-parametrised, defaults unchanged). Optional: one run on Anton Public/Testing.
- SMB-FU-5 UPDATE (2026-11-11, later): one run on the Anton host Samba share `Public`, sub `Testing` was recorded on main e5817850 (TestLiveSMBSubdir 4 outside-name probes PASS, TestLiveSMBServiceSubdir PASS), evidence `docs/qa/smb-subdir-20261110/live-run-anton-20261111T190325Z/` (log committed in `652928dc`, `sha256sum -c go_test.log.sha256` passes (run from repo root); single run, N=1). Scope: host Samba by host name, not a bare-IP proof, not Synology. PROVISIONAL (N=1 < required N>=3); pending job: two more runs with committed logs. The earlier line "never executed yet" is superseded.

## CAT-FOLLOWUPS-UNIT-1 review follow-ups (2026-11-11, Opus GO-with-follow-ups)
- FU20-3b (test-instrumentation): decide whether fractional knob sizes (w-3.5 h-3.5) are exempt; replace the it.todo in contrastHelper.test.ts.
- PROM-R3-1: PROM_AUTH_USER precedence differs: up.sh lets mon.env win over the environment, verify.py:38 lets the environment win; align and add a both-set test.
- LIP-FU-7 (nit): `QA_LAN_IP_LIB` override in stack3/lib.sh (and sonarqube/lib.sh) is undocumented.
- LIP-FU-8 (nit): lib.sh now hard-depends on docs/qa/lib/lan_ip.sh, so down.sh (which sources lib.sh) fails if that file is missing.
- LIP-FU-9: loopback/unspecified/link-local rejection is in stack3/lib.sh only; IPv6 (::1, fe80::) not applicable (IPv4-only helper).

- SMB-FU-9: DNS-rebinding TOCTOU. services.VetSMBHost resolves once, but the dial (net.JoinHostPort(host,"445")) resolves the name again. Dial the vetted IP instead (keep the name for logging only). Opus review round 1, item 2.
- SMB-FU-10: RESOLVED (branch fix/smb-followups-10-12-14). `pathsafe.SafeRemoteSharePath` (used by the discovery service `BrowseShare` and the handler `validateSMBTarget`) rejects, besides ".." and NUL, any segment containing ":" (ADS / drive form), any segment made only of dots and spaces other than "." (".. ", "..."), and any segment whose base name (text before the first ".", trailing dots/spaces ignored, case-insensitive) is a Windows device name: CON, PRN, AUX, NUL, CONIN$, CONOUT$, CLOCK$, COM0-9, LPT0-9, COM/LPT with superscript 1-3. COM0/LPT0 are treated as reserved conservatively (UNCONFIRMED against the current Microsoft page; not fetched). `SafeRemotePath` is unchanged. Residual: symlinks the server resolves are not handled lexically.
- SMB-FU-11: residual information leaks and timing on /smb/test and /smb/browse (connect failure vs auth failure vs mount failure remain distinguishable by latency and by the boolean/400/500 split). Opus review round 1, item 8.
- SMB-FU-12: RESOLVED (branch fix/smb-followups-10-12-14). `services.NewSMBDiscoveryServiceFromConfig` (used by main.go) registers the (host, port) pairs of `config.SMB.Hosts`; `ValidateSMBPortForHost(host, port)` accepts 445, ports in `CATALOG_SMB_ALLOWED_PORTS` (global, operator-set) and a configured non-445 port ONLY when the request host is that root's host (same name case-insensitively, or a common resolved address). Other hosts, including other RFC1918 hosts, are refused. The binding is re-applied at dial time (`bindVettedIPsToPort`): only vetted addresses that belong to the root are dialed, so DNS rebinding and multi-address names cannot reach another host on the configured port. Malformed entries (empty/invalid host, port 0, <1, >65535, 445) are ignored. The set is read at start-up: a storage root created at runtime needs a restart before discovery accepts its non-default port.
- SMB-FU-9 (PRIORITISED, security): FIXED and merged in 532c2ec8 (Opus GO-with-follow-ups). `VetSMBHostIPs` returns the vetted addresses; `dialVetted` (services/smb_dial.go) dials those IPs in order through the `SMBDialer` seam and hands the net.Conn to go-smb2 (its Dialer.Dial takes a conn, no target name needed for NTLM). DiscoverShares, mountShare (openConfined) and ProbeHostWithIdentities resolve+vet exactly once per request (the probe shares one vetted list across guest and all identities). The service layer now refuses blocked hosts itself (defence in depth to the handler check). Tests: services/smb_dialvetted_test.go (rebinding resolver: allowed IP first, 127.0.0.1 afterwards). dialVetted shares ONE connect deadline across attempts and tries at most 4 vetted IPs. See SMB-FU-9b.
- SMB-FU-9b (follow-up): admin-configured by-name dials can still be rebound: storage roots, `SMBService.getConnection` (services/smb.go), `smb/client.go`, `filesystem/smb_client.go`. They are not request-supplied, but each should resolve once, vet and dial the vetted IP the same way (reuse `VetSMBHostIPs` + `dialVetted`).
- SMB-FU-13: the share listing still shows `print$` (SplitShare allows it; it is not administrative there). Decide whether discovery should hide printer shares.
- SMB-FU-14 (test-instrumentation): RESOLVED (branch fix/smb-followups-10-12-14). Tests replace SMB globals only through `withSMBHostResolver` / `withSMBDialer` / `withSMBWarnf` / `withSMBNow` / `withSMBConfiguredPorts` (services/smb_globals_testhelper_test.go; locked, restored). `TestSMBGlobalsTestsNotParallel` (AST lint, with golden-bad/good self-check) fails if a test using them calls `t.Parallel()` or assigns a global directly. The handlers package has its own locked `withSMBConfiguredPorts`; its other tests still assign `services.SMBHostResolver` by hand (no lint there yet).
- SMB-FU-15: RESOLVED BY REMOVAL (2026-11-11): `internal/handlers/smb.go` SMBHandler was confirmed unwired (no caller outside its own tests, nothing in main.go) and was deleted with its tests.

## Tauri guard follow-ups (branch fix/tauri-guard-followups, PROVISIONAL-DONE)
- TAURI-R1-FU-1/2/3/4/6/8: already resolved on main before this branch (per-capability parsing, import forms, pre-flight wiring, docs/qa/tauri-native-20261106/README.md present, installer-wizard capabilities.json untracked+ignored in c504d197, header states exit 2 = BLIND); verified by re-running guard + mutation test.
- TAURI-R2-FU-1 DONE: .toml and .json5 capability files (json5 = comments/trailing commas only, other syntax fails closed), tauri.conf.json app.security.capabilities (enabled list + inline caps) and app.windows[].label.
- TAURI-R2-FU-2 DONE (review round 3, structural): the source lexer was REMOVED. Usage detection scans the RAW text of every .ts/.tsx/.js/.jsx/.mjs/.cjs/.mts/.cts under installer-wizard/src (all directories, tests included): dialog-plugin imports/export-from/dynamic imports/require and `plugin:dialog|<cmd>` strings count wherever they appear, so a mention in a comment or string is a deliberate false positive (demands the permission); no false negative from masking is possible. ask/confirm/message covered (star/dynamic/unrecognised-position import need all five). Capability files are parsed per capability in all Tauri CapabilityFile shapes (Capability | List | NamedList {"capabilities":[..]}); type errors in tauri.conf.json/capabilities and an unterminated json5 /* comment, or any internal error, exit 2 (BLIND). Window match is fnmatch (pinned by mainwin / ma?n cases). Remaining honest limits: command names built at run time, differently named re-export modules, .vue/.svelte sources, capabilities/ subdirectories, tauri.<platform>.conf.json; a non-string `windows`/`platforms` in a capability is a FAIL (not BLIND). Guard-of-the-guard: scripts/tauri/test_wizard_capabilities_guard_mutants.sh (9 mutants, all killed).
- TAURI-R2-FU-4 DONE: pre-flight-check.sh cds to repo root; test_preflight_cwd.sh wired in.
- TAURI-R2-FU-3 / R1-FU-6 desktop: catalogizer-desktop/src-tauri/gen/schemas/capabilities.json is tracked as `{}` since 2b182b01 (Tauri build output; no capabilities/ dir; installer-wizard ignores the same path in installer-wizard/.gitignore). Recommendation: add the path to catalogizer-desktop's .gitignore and `git rm --cached` it (operator/reviewer decision; NOT done here per §11.4.124).
- TAURI-R2-FU-5: WONTFIX/corrected-in-later-commits (a pushed trailer cannot be changed; later commits carry correct closed-list trailers).
- TAURI-R3-FU-1..4 (register): guard does not recurse into capabilities/ subdirs; ignores tauri.<platform>.conf.json / Tauri.toml; empty enabled list semantics (currently treated as enabling nothing; Tauri may differ, verify); multi-window apps (only main/first label checked).
- Review round 1 (Opus NO-GO) fixed: masking weaker than parent (5 repros + backtick/aliased/re-export cases), fail-open parse errors (now exit 2), PRE_FLIGHT_CHDIR_ONLY exit 0 (now 3), mutants E/J/M killed by cases.
- TAURI-R4 (review round 4, structural, Opus round-3 NO-GO closed): usage detection is now precise-forms + structural fallback (any unexplained "plugin-dialog"/"plugin:dialog" mention incl. after \xNN/\uNNNN decoding, any "__TAURI__" text, app.withGlobalTauri true => all five permissions; only vi/jest module mocks are explained away); "capabilities" key checked before "identifier" (Tauri NamedList first); dialog:default counted as open/save/message only and ask/confirm also demand allow-message; symlinked src dirs followed with realpath loop guard, unreadable dirs BLIND; gofmt-only sweep of 44 catalog-api files reverted from the branch (§11.4.293); denies detection uses a flag instead of message text.
- TAURI-R4-FU-1: split plugin names ('plugin:' + 'dialog'), a package alias re-exported from a differently named module, and non-JS sources (.vue/.svelte) remain undetected (documented guard limits).
- TAURI-R4-FU-2: the guard's dialog:default expansion and the ask/confirm=>message rule follow upstream tauri-plugin-dialog as read in review (v2.6.0 vs v2 head); pin the Rust crate and track Cargo.lock, then replace the conservative assumption with the pinned crate's actual default set.
- TAURI-R4-FU-3: a mention in a comment/string/test file other than vi/jest module mocks demands all five permissions (accepted false positive); no source lexing by design.
- TAURI-R4-FU-4: capability "webviews" field is ignored (conservative: only "windows" is matched); a remote capability with urls [] FAILs (conservative: any "remote" object is treated as granting to remote origins).
- TAURI-R4-FU-5: html scan covers *.html under installer-wizard (excluding node_modules, dist, target, .git); a non-empty src-tauri/permissions/ is BLIND (app-defined permission sets not parsed). Pre-flight runs the guard mutants only when tauri guard/wizard files changed (PRE_FLIGHT_FORCE_TAURI_MUTANTS=1 forces).

## Dead-SMB cleanup review follow-ups (branch fix/dead-smb-and-schema)
- SMB-FU-16 (docs/clients, minor): the Android client and its API docs still call the never-routed `/api/v1/smb/sources/*` and `/api/v1/smb/health` endpoints; align or remove them (not touched here).
- SMB-FU-16b (nit): the `.gitignore` convention differs between catalogizer-desktop (`src-tauri/gen/schemas/` in the app .gitignore) and installer-wizard (single file entry); pick one.
- SMB-FU-16c (nit): `catalog-api/api_contract_happy_exceptions.tsv:61` has an ambiguous `download.go` reference after the root handlers/download.go removal; qualify it as `internal/handlers/download.go`.

## CAT-QA-STACK-LIFECYCLE-1 follow-ups (Opus round 2, minor, 2026-11-11)
- LIFE-FU-1: RESOLVED: ctrctl builds to `<bin>.tmp.<pid>` in the cache dir then `mv -f`; test with 6 concurrent builds (distinct tmp outputs) + mutants. A real cross-FS ETXTBSY reproduction needs an ELF binary (fake go writes scripts): documented skip. (was: ctrctl concurrent first builds race when TMPDIR is on another filesystem (build to `$bin.t...)
- LIFE-FU-2: RESOLVED: tests+mutants for the inspect timeout (`QA_INSPECT_TIMEOUT`), per-checkout cache keying, ctrctl non-zero exit propagation (start and stop). (was: test gaps: `timeout 10` bound on inspect, per-checkout cache keying, ctrctl non-zero exit...)
- LIFE-FU-3: RESOLVED: README and podman_env.sh header corrected (guid= only, per-checkout cache hash dir, mutant count). (was: doc drift (README/podman_env.sh header say "strips every ,key=val"; cache path now per-che...)
- LIFE-FU-4: RESOLVED: `set -f` (restored) in `_qa_strip_guid`, FORCED vs unavailable DEVIATION text, local `rc`, clear missing-`timeout` error (rc 69). Not done by decision: faster unhealthy failure (risky). (was: nits): `_qa_strip_guid` unquoted glob expansion (use `set -f`); DEVIATION text when podman...)
- up.sh/down.sh/build.sh still call podman directly (documented deviation, not yet run against real catqa3-api).
- SMB-FU-17: `internal/handlers/copy.go` (about lines 62 and 162 after the dead-code removal) still uses plain `pathsafe.SafeRemotePath`, so ADS write targets and device names are accepted there. Switch to `SafeRemoteSharePath` once the dead-code removal branch has landed (or delete the code).
- SMB-FU-18 (minor): the SMB-FU-14 lint can be bypassed via a helper function or pointer writes to the globals; handlers tests still assign `services.SMBHostResolver` unlocked and have no lint; `lockSMBGlobals` can deadlock when a subtest of a test that holds the lock calls a helper (different `*testing.T`).

## CAT-QA-STACK-LIFECYCLE-1 follow-ups round 3 (Opus review of LIFE-FU-1..4, minor, 2026-11-11)
- LIFE-FU-5: RESOLVED: test records the tmp inode in the fake go and asserts the final ctrctl has the same inode; mutant `copy_instead_of_rename` (cp+rm) killed. (was: test gap: nothing distinguishes atomic `mv -f` from copy+rm (assert inode equality or a logging fake mv).)
- LIFE-FU-6: RESOLVED: shim mode `SHIM_HEALTH_HANG` hangs only the Health inspect. The test exposed a real defect: a timed-out Health inspect was read as `none` (no healthcheck) and reported HEALTHY at once; `_qa_stack_state` now yields `unknown` when the Health inspect fails or times out, so the wait continues and ends rc 1. Mutant `health_hang_reads_none` killed. (was: test gap: hung-inspect test only hangs the first inspect; add shim mode hanging only the Health inspect.)
- LIFE-FU-7: RESOLVED: test runs `_qa_strip_guid` with `set -f` already on and asserts it stays on; mutant `noglob_restore_unconditional` killed. (was: test gap: no test calls `_qa_strip_guid` with `set -f` already on and checks it stays on.)
- LIFE-FU-8: RESOLVED: `_qa_stack_need_timeout` is pre-flighted in `qa_stack_start` (after the name check, before env normalisation and `_qa_stack_run`); test asserts rc 69, no `podman start`, container state unchanged; mutant `no_start_timeout_preflight` killed. (was: missing-`timeout` check lives in `qa_stack_wait_healthy`, so `qa_stack_start` starts the container and only then returns 69 with it left running; pre-flight the check in `qa_stack_start` before `_qa_stack_run`.)
- LIFE-FU-9: RESOLVED: (a) the tmp cleanup after a failed build IS reachable (fake go now leaves a partial tmp; test asserts none remains; mutant `no_tmp_cleanup` killed); (b) `_qa_stack_run` maps a real ctrctl exit 69 to 1, keeping 69 reserved for backend unavailable (test + mutant `ctrctl_69_unmapped`). (was: unreachable tmp cleanup branch on build failure; a real ctrctl exit 69 would read as "backend unavailable" (latent).)

## CAT-SMB-SUBDIR-1 follow-ups (Opus round 3 of dead-code removal, minor, 2026-11-11)
- SMB-FU-19a (test): pathsafe mutant "ancestor walk stops one level early" survives; only a false rejection when root is a top-level dir (never an escape). Add a test with the deepest existing ancestor a direct child of `/` or record equivalent-for-escapes.
- SMB-FU-19b (docs): README lines ~405-425 (Connection States table, SMB_RETRY_* / SMB_OFFLINE_CACHE_SIZE env block) may still describe the removed per-source SMB state; check whether those env vars are still read and fix the drift.
- SMB-FU-19c (nit): dangling-symlink test passes on any error; assert the reason ("cannot resolve path" vs ErrPathEscapes).

## LIFE-FU-10..15 (Opus round-1 review of LIFE-FU-5..9, 2026-11-11; all minor; PROVISIONAL-DONE on fix/lifecycle-followups-3, each with a test and a killing mutant in test_podman_env_mutations.sh)
- LIFE-FU-10 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): `docs/qa/lib/podman_env.sh` (~:81, start/stop wrappers ~:119/:126): under `set -e` a bare `qa_stack_stop` exits on the raw ctrctl code before the 69→1 remap. Use `"$bin" … || r=$?` pattern or document non-errexit use; add test.
- LIFE-FU-11 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): test gap (mutant R8): no test that a transient `unknown` Health state recovers to healthy; add shim mode failing the first N polls.
- LIFE-FU-12 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): test gap (mutant R3): Health inspect empty output with exit 0 not covered (`${h:-unknown}` default).
- LIFE-FU-13 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): `_qa_strip_guid` (:22/:28) leaves IFS set-but-empty globally after `unset IFS`; use `local IFS`.
- LIFE-FU-14 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): timeout message in qa_stack_wait_healthy should log the last state.
- LIFE-FU-15 (PROVISIONAL-DONE: shim-verified; live lifecycle run on this tip owed): LIFE-FU-6 test bound `el <= 6` equals the 6 s hang; use a longer hang for margin.
## CAT-QA-STACK-LIFECYCLE-1: stack scripts via ctrctl (branch feat/stack-scripts-via-ctrctl, 2026-11-11)
Migrated: container START of `docs/qa/stack3/up.sh` (postgres, redis, api, web, website) and `docs/qa/stack3/sonarqube/up.sh` (db, sonar): `podman run -d` became `podman create` + `qa_stack_start` (helper `ctr_up` in both lib.sh) = ctrctl over submodules/containers. Guard: `docs/qa/stack3/test_ctrctl_routing.sh` + `_mutations.sh`.
ctrctl supports ONLY `start|stop|status <container>` (docs/qa/lib/ctrctl/main.go) and the helper adds a `${STACK}-` name guard and a health wait. Everything below is left on raw podman because no helper/ctrctl verb exists (UNKNOWN whether the Containers runtime offers it; not checked, not guessed):
- STACK-CTR-FU-1: pod create/rm/exists/inspect, `podman port`, `podman ps --filter pod=`, volume create/rm/exists/inspect, image exists/inspect/pull/build, `podman run --rm` (scanner, build.sh, monitoring helper runs), logs, exec, cp, secret create/rm, network create/rm, info (up.sh, down.sh, build.sh, sonarqube/*, monitoring/*, s3/*, helixqa/run.sh) need new ctrctl verbs and helpers; helper changes belong to branch fix/lifecycle-followups-2 (podman_env.sh not edited here).
- STACK-CTR-FU-2: `monitoring/up.sh` and `s3/up.sh` `podman run -d` (names `${MON_POD}-*` / `catqa-s3live-garage`) are NOT migrated: need per-call `QA_STACK_PREFIX` and shim updates; candidates for the same create+qa_stack_start pattern.
- STACK-CTR-FU-3: `down.sh` stops by `pod rm -f` (no ctrctl stop-then-remove path); `qa_stack_stop` could be used before the pod removal.
- STACK-CTR-FU-4: `lifecycle_probe.sh` keeps raw podman on purpose (RED control proving the raw start fails with a guid= DBUS address, and live assertions).
- STACK-CTR-FU-5: `ctr_up` start waits twice (helper wait + the script's own `wait_healthy`); `qa_stack_wait_healthy` accepts "running none", so web/website are not health-gated by the helper. Not run live (host load); the first real `up.sh` run must be evidence-checked (backend= line in output, `podman ps` shows catqa3-api healthy, DBUS guid= address exported).

## STACK-CTR-FU-6..12 (Opus round-1 review of feat/stack-scripts-via-ctrctl, 2026-11-11; all minor; PROVISIONAL-DONE on fix/stack-ctr-followups 2026-11-11 (shim-verified; live up.sh run owed), FU-8 for stack3 up.sh only)
- STACK-CTR-FU-6 (test): static guard in test_ctrctl_routing.sh must match any `podman … run` with `-d`/`--detach` in any position and `podman container start|run` (mutants N1, N2 survive). PROVISIONAL-DONE (shim-verified; live up.sh run owed): raw_start detector (continuation-joined, any flag position, `container run|start`) with golden-bad/good self-test; mutants N1, N2 killed.
- STACK-CTR-FU-7 (test): add create-failure mode using `SHIM_CREATE_RC` (expect non-zero, no `podman start`) (mutants N3, N5). PROVISIONAL-DONE (shim-verified; live up.sh run owed): createfail mode (SHIM_CREATE_RC=1: rc!=0, no start/inspect) for both libs; mutants N3, N5 killed.
- STACK-CTR-FU-8 (test): run up.sh with a recording shim and diff each container's `create` argv against a golden list, proving flags preserved (mutants N4 `--restart`, N6 `--health-cmd`). PROVISIONAL-DONE (shim-verified; live up.sh run owed) for stack3/up.sh: recording-shim run diffed against docs/qa/stack3/golden/up_create_argv.txt (per-argument bracketed argv; regenerate with GOLDEN_UPDATE=1); mutants N4, N6 killed. OPEN: the same golden for sonarqube/up.sh (needs ip/ss/df/load shims, not yet built).
- STACK-CTR-FU-9 (nit): run the `${STACK}-` name guard before `podman create` in `ctr_up` so a refused name leaves no created container. PROVISIONAL-DONE (shim-verified; live up.sh run owed): both ctr_up call _qa_stack_check_name before `podman create`; foreign-name test asserts no create; mutants N7, N8, sonarqube M24 killed.
- STACK-CTR-FU-10 (test): sonarqube/test_guards.sh:52 `created()` pattern should include container `create `. PROVISIONAL-DONE (shim-verified; live up.sh run owed): created() includes `create `/`container create `; T2c/T2d added; dropping the pattern fails T2d, M24 killed.
- STACK-CTR-FU-11 (perf): `_qa_stack_backend` runs `go build` on every `qa_stack_start`; build once per script run. PROVISIONAL-DONE (shim-verified; live up.sh run owed) in the stack3 lib.sh wrappers only (podman_env.sh untouched): _qa_stack_backend is wrapped, resolved once in ctr_up (main shell) and cached in _QA_STACK_BACKEND_CACHE; perf test with counting go shim (builds=1); mutants N9, N10 killed.
- STACK-CTR-FU-12 (nit): test_ctrctl_routing*.sh mode 644 (cosmetic). PROVISIONAL-DONE (shim-verified; live up.sh run owed): test_ctrctl_routing*.sh mode 755.
- Live check owed: first real up.sh run — verify `backend=` line, catqa3-api healthy in `podman ps`, DBUS address without `guid=`.
- STACK-CTR-FU-13 (Opus round-1 F1-F6 review of STACK-CTR-FU-6..12): PROVISIONAL-DONE (shim-verified; live up.sh run owed). F1: the backend cache in stack3/lib.sh and sonarqube/lib.sh is reset at source time, keyed on `$QA_STACK_BACKEND|$QA_REPO_ROOT` and holds only a successful `containers <bin>` (failed build not cached; test failcache); inherited-env tests F1/F1a/F1b; mutants R1, R1b, R4, R4b, R6, R6b killed. F2: GOLDEN_UPDATE refused with `--root`, mutations script unsets it, update exits 3 with "golden regenerated, review the diff" (mutants F2-M1, F2-M2). F3: raw_start also flags `pod start`, `restart`/`container restart`, `unpause`, `kube play`/`play kube` (mutants R2, R3). F4: real root derivation tested (unset QA_REPO_ROOT; mutants R5, R5b). F5: STACK, image, MEDIA_MOUNT and port vars unset in the golden env. F6: ctr_up refuses BEFORE `podman create` when the resolved backend is `none` (mutants R7, R7b). OPEN: the same golden for sonarqube/up.sh (see FU-8).

- STACK-CTR-FU-14 (Opus round-2 follow-ups on STACK-CTR-FU-13, all test-instrumentation, OPEN): (a) mutant X4 survives: no test captures the root the resolver actually receives when QA_REPO_ROOT is unset (stub `_qa_stack_backend_uncached` after sourcing); cache-key behaviour tests for a key built with another root or mode. (b) detector `raw_start` misses global options before start/run (`podman --log-level=debug start`, `--remote container start`), `pod restart`/`pod unpause`, tab separators, `$PODMAN start`, `podman compose up`, `podman-compose`, `auto-update`, `container runlabel` (mutants X5, X6 survive). (c) nit: false positives on trailing comments and echo prose (safe side). (d) nit: `|` in cache key can collide for nonsense QA_STACK_BACKEND values; use a NUL/%q separator. (e) F6 refusal is a deny-list on `none`; prefer allow-list `containers *`|`podman`; cache key lacks QA_CTRCTL_CACHE. (f) golden subshell still inherits QA_PODMAN_ENV_LIB, QA_LAN_IP_LIB, QA_STACK_PREFIX, QA_CTRCTL_CACHE. Coupling note: stack3/lib.sh and sonarqube/lib.sh copy `declare -f _qa_stack_backend` and rely on its name, QA_REPO_ROOT-based root, `containers <bin>`/`podman`/`none` contract and no internal cache; re-run test_ctrctl_routing.sh and its mutations when podman_env.sh changes.
- LIFE-FU-19 (Opus round-2 follow-ups on LIFE-FU-10..15, test gaps, OPEN): F1 the set -e `_qa_stack_run` test covers only the ctrctl branch; add the podman-branch variant (SHIM_PODMAN_FAIL=69; mutant n3 survives). F2 no test that a container vanishing during the health wait fails fast with "not found" (mutant n4). F3 rc passthrough (126/127/130/137) through `_qa_stack_run` untested (mutant n5, optional). F4 LIFE-FU-16/17/18 referenced in test comments: 16-18 are covered by the tests in test_podman_env.sh (PROVISIONAL-DONE, shim-verified; live lifecycle run owed).

- CAT-QA-SONAR-2 (triage of CAT-QA-SONAR-1 run3, main 0020ad3e; OPEN): catalog-api 699 issues, 697 code smells and 2 bugs (go:S3923 MAJOR in services/webdav_client_test.go and services/configuration_wizard_service.go; fix these 2 bugs too): of the smells 338 go:S1192 (duplicated string literals), 310 go:S3776 (cognitive complexity), 10 go:S1186 (empty function) (658 CRITICAL), 28 MAJOR (the 30 API MAJOR include the 2 MAJOR bugs), 11 MINOR; 0 hotspots; no BLOCKER and no vulnerability-class rule. catalog-web 404 issues = 384 code smells / 15 bugs / 5 vulnerabilities (recomputed from `catalog-web.issue_facets.json`): 5 VULNERABILITY `typescript:S2245` MAJOR (insecure PRNG: test-setup.ts:132,135, AdvancedSearch.tsx:199, SmartCollectionBuilder.tsx:100, UploadManager.tsx:50), 15 BUG (S1082 x9 MINOR, S6959 x2, S8985 x2, S3923 x1, S9115 x1 MAJOR); 16 BLOCKER `typescript:S2699` = tests with NO assertion (anti-bluff relevant; files: useCoverQuality.test.tsx:97, HistoryDrawer.test.tsx:210, CollectionExport.test.tsx:109,147, SmartCollectionBuilder.test.tsx:138,233, PlaylistPlayer.test.tsx:111,147,161,177,196, PlaylistItem.test.tsx:70, Settings.test.tsx:253,309, SubtitleManager.test.tsx:154,162), plus 12 CRITICAL (8 S3776, 4 S3735), 219 MAJOR, 157 MINOR; 0 hotspots. Plan, fix order: (1) the 5 web S2245 vulnerabilities, (2) the 15 web bugs (the 2 API go:S3923 bugs: fix landed 79547302 via merge b304304b, PROVISIONAL, Sonar re-run owed, CAT-QA-SONAR-5), (3) the 16 S2699 tests (landed on main: 765d6544 and 6b3d706e (merged in c80e35dd) and 23b842d0 (direct follow-up on main), Opus GO-with-follow-ups; PROVISIONAL, Sonar re-run owed, clearance unconfirmed), then S3735/S3776 in web, then api S1186; S1192/S3776 in api are refactor-scale (low risk, high volume) - baseline-ratchet instead of a mass rewrite. Evidence: docs/qa/sonarqube-20261105/run3/.
- CAT-QA-SONAR-1b (PROVISIONAL-DONE -> evidence): live `up.sh` start through the Containers submodule from the MAIN checkout verified 2026-11-11: `qa_stack: backend=containers ctrctl_sha256=b6f69c8e6507757dfaa49680138c44451b56c81db0fcec73d6dd71363768ec7c`, catsonar-db healthy, SonarQube UP, `down.sh` removed the pod (0 catsonar containers left). Log was not committed (scratch). The stack3 (catqa3) `up.sh` through the same path is still not run live.
- CAT-QA-SONAR-3 (Opus round-2 follow-ups on fix/web-s2699, landed; OPEN, test gaps): FU-1 SubtitleManager.test.tsx:~186 search-params test asserts only `query: 'Matrix'`; assert `language` (and ideally `providers`) so a wrong-language mutant is killed (mutant M10 survived). FU-3 CSV export branch (`exportOptions.format === 'csv'`: mime type and content) in CollectionExport is exercised by no test. FU-4 whole-repo eslint run has 1 pre-existing `no-extra-semi` error (also on main) and `lint` uses --max-warnings 0.

- CAT-QA-SONAR-5 (Opus round-1 GO-with-follow-ups on fix/api-s3923, landed; OPEN, operator decision §11.4.122/§11.4.66): the built-in wizard PostInstall `file_create` step (catalog-api/services/configuration_wizard_service.go:~449-451) declares `"template": "config_template.json"` (Required: true) but no such file exists and template loading was never implemented, so the declared template is silently ignored (pre-existing). Either implement template loading, remove/flag the declared template, or at least log when a template is given but ignored. The S3923 change itself is behaviour-neutral. Pre-existing note: the WebDAV ReadDir path has no explicit timeout (black-holed network could hang webdav_client_test). Not verified: `TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded` fails identically on base and tip (host-dependent; reviewer suspects the 4G scope, UNCONFIRMED).

- CAT-QA-SONAR-6 (Opus round-1 GO-with-follow-ups on fix/sonar-critical-stage2, landed 17aaa10a; PROVISIONAL-DONE, Sonar re-run owed; follow-ups OPEN, minor): FU-2 tests to kill 3 surviving mutants (3-element date range in collectionRules; Collections pagination with >5 pages near the end; FavoriteToggle button-variant `mr-2`); FU-3 characterize CollectionRealTime connected/connecting states; FU-4 make `PRELOAD_BY_TAB` lookup own-property-safe (nit, unreachable today); FU-5 (pre-existing) Postgres container tests in catalog-api/database should SKIP with a reason when podman is unusable instead of FAIL. FU-1 (s3_client.go comment at :339/:343) fixed on main.

- CAT-QA-SONAR-4 (Opus round-2 follow-ups on fix/web-sonar-vulns-bugs, landed f9932aec; PROVISIONAL-DONE, Sonar re-run owed; follow-ups OPEN, minor): FU-A MediaPlayer.test.tsx:~281 "an error that lands during a reload does not lose the resume position" is timing-flaky under load at base and tip (20 ms timer): remove the timing dependency or quarantine (§11.4.248); FU-B cosmetic indentation of the `DiscoveredShares keyboard operability` describe in IdentityManager.test.tsx; FU-C the AdvancedSearch "Load" span copies Button-outline classes by hand (shared non-interactive style); FU-D `randomId()` yields ~36 bits, UI keys only, never a security token.

- CAT-QA-COVERART-1 (cover-art SSRF live test landed, Opus round-3 GO-with-follow-ups; PROVISIONAL-DONE: fixture-server class §11.4.289 C(3), cap/redirect legs run on netguard.Client not the production StrictClient, N=1 host `go test` under bwrap netns; follow-ups OPEN, minor): E-1 mutation.sh must compute `results/final/SHA256SUMS.txt` AFTER the final MUTATION-SUITE line is appended and include `race/` (manifest currently fails on summary.txt); E-2 run.sh `git_dirty_outside_results` emits a stray `UNKNOWN` line in clean repos (`grep -vc` exit 1); add cases/README "Not covered" for 4 surviving mutants: non-http(s) redirect scheme / nil redirectOK (scheme_any, redirect_nilok), redirect to 100.64.0.0/10 (cgnat_drop), redirect to embedded IPv4 NAT64/6to4 (embedded_drop); in the 4-hop case assert the refusal cause (HTTP 302); README table row should mention the 4-hop case and soften "not feasible" to "not feasible with loopback fixtures only". Live proof against the deployed stack and proxy/ffprobe paths still owed.

## CAT-REL-CHECK-1 follow-ups (Opus round-2 GO-with-follow-ups, 2026-11-11)
- N1 (first): checker accepts forged helper output (`PLAN 0` + no rows => PASS version_ok=0; duplicate PLAN lines; INFO-only rows). Fix in bash: hard-code expected 19 rows, refuse second PLAN, require V_OK+V_FAIL == 17 fields + 2 versionCode rows, reject duplicate labels; add a plan/completeness check to the changelog helper.
- N2: BSD-tag manifest lines (`SHA256 (../../x) = h`) bypass the F4 escape check; refuse non-GNU lines or parse BSD form.
- N3: tracked symlinked directory pointing outside the repo is followed; require resolved path under $REPO.
- N4: `versionCode = 7 - 2` read as 7; anchor the literal (`\s*(//.*)?$`).
- N5: duplicate `## [x.y.z]` heading silently ignored; make it BLIND. Also `versionNameSuffix` false ambiguity, README gaps (forged helper output, BSD lines).

## CAT-QA-SONAR-7 (SonarQube run4 on v2.4.1, 13 new web smells; PROVISIONAL static)
- typescript:S6819 x8 (CollectionTemplates.tsx:497, AdvancedSearch.tsx:488/532, CollectionExport.tsx:723, CollectionPreview.tsx:216, DiscoveredShares.tsx:151, HistoryDrawer.tsx:66, SubtitleManager.tsx:278): likely role="button"/"checkbox" divs; prefer native elements.
- S3358 FavoriteToggle.tsx:134 (nested ternary), S4323 FavoriteToggle.tsx:11 (type alias), S7773 collectionRules.ts:358/361/384 (Number.isNaN). Cause UNCONFIRMED.

## CAT-QA-SONAR-7 follow-ups (Opus GO-with-follow-ups, 2026-11-11; fixes PROVISIONAL, no new Sonar scan yet)
- FU-1: no test that checkbox click/Space in CollectionPreview does not fire onPreviewItem (mutant survives on base too).
- FU-2: user-visible change: Enter no longer toggles the CollectionPreview selection checkbox (native, Space only); note in changelog.
- FU-3: HistoryDrawer backdrop stacking depends on `relative` on dialog; needs browser check (jsdom blind).
- FU-4: FavoriteToggle "Loading..." label untested. FU-5: buttons wrap block elements (HTML content model nit). FU-6: CollectionTemplates grid motion.div still role=button. FU-7: CollectionExport says "drag and drop" but has no drag handlers (pre-existing).

## CAT-REL-CHECK-1 round-2 follow-ups (N1-N5 landed; Opus GO-with-follow-ups, 2026-11-12)
- F1 (fix soon): backslash-prefixed BSD manifest lines (`\SHA256 (../x) = h`) bypass the escape check on uutils sha256sum; fail closed on any non-blank line not matching the two unescaped forms; README currently inaccurate for escaped BSD lines.
- F2: manifest entries that are in-dir symlinks to outside the repo are followed (pre-existing); apply the readlink -f under-$REPO check to manifest entries.
- F3: tests for 4 surviving reviewer mutants (prefix check without `/`, repo-root symlink branch, BSD lowercase-only hex, `//`->`/` comment regex).
- F4: under --prev-tag, forged INFO on a versionCode row passes; bash should reject INFO there absent a --prev-tag FAIL row.
- F5 (nit): fenced `## [x]` heading triggers duplicate BLIND; `versionCode = 7 /* c */` and `7;` FAIL (all fail safe).

## CAT-QA-SONAR-7 follow-ups FU-1..FU-7 resolution (branch fix/web-sonar-fu; PROVISIONAL, no new Sonar scan)
- FU-1 DONE: tests that checkbox click/Space do not fire onPreviewItem. FU-2 DONE: Enter-to-toggle restored via keydown on the native checkbox (was a user-visible regression), test added. FU-3 PARTIAL: structural jsdom test (dialog `relative`, after backdrop in DOM); real stacking/paint needs a browser check (OWED). FU-4 DONE: "Loading..." label test. FU-5 OPEN (nit): grid template card buttons still wrap block elements. FU-6 DONE: grid card is now motion.button (no role). FU-7 DONE honestly: text changed to "Click to select a file to upload" (no drag handlers implemented).

## CAT-QA-SONAR-7 FU round-2 (Opus review of fix/web-sonar-fu, GO-with-follow-ups, PROVISIONAL)

- FU2-1 (minor, process-doc): STILL OPEN. HistoryDrawer stacking test is structural only (jsdom has no layout/paint); a live browser paint-order check (e.g. `document.elementFromPoint` at the dialog centre must return a dialog descendant, not the backdrop) is owed before FU-3 closes. UNCONFIRMED: not run in this unit (no browser run on the loaded host).
- FU2-2 (DONE (provisional, unit tests), CAT-QA-SONAR-7-FU2): CollectionPreview checkbox Enter handler now ignores `e.repeat`; test added (RED before, GREEN after).
- FU2-3 (DONE (provisional, unit tests) for CollectionTemplates grid and list template buttons, CAT-QA-SONAR-7-FU2): div/h4/p inside the buttons replaced by spans (block via class); test asserts no block-level descendant. An h4 inside a button was never exposed as a heading to assistive technology (button children are presentational), so no aria compensation is needed. Other buttons repo-wide were not audited.
- FU2-4 (INVESTIGATED, KEPT, CAT-QA-SONAR-7-FU2): `onActivateKey` was added in e823f697 (S1082) with 7 callers (AdvancedSearch x2, upload area, CollectionPreview, CollectionTemplates, an expander, a media select); a0cbf0e9 added the `e.repeat` guard; 60bc821d (S6819) migrated callers to native buttons/inputs, 6c063630 removed the last one. Death cause: Sonar S6819 preferred native elements. It is a tested (keyboard.test.ts), documented utility with no production caller. Not deleted: removal needs an operator keep-or-remove decision (§11.4.122); open question for the operator.

## CAT-REL-CHECK-1 round-3/4 (F1-F5 + review fixes on branch fix/release-checker-f1-f5, 2026-11-12)
- F1 fixed (fail closed on any non-blank manifest line outside the two unescaped forms; README corrected). F2 fixed after review round 4: entries are resolved with `readlink -m` (missing parents are not an escape; a missing tail below an outside symlink still FAILs) and CRLF manifests are normalised (one trailing CR stripped; a CR inside a line is refused). F3 covered by tests plus mutants. F4 fixed (INFO on versionCode under --prev-tag is BLIND unless the --prev-tag row FAILed). F5 fixed (fence-aware headings, comment pattern cannot span a closing `*/`; `7;` and `7 /* c */` parse).
- Open nit: a `~~~` line inside a ``` fence (or the reverse) toggles the fence state, since fences are not tracked by kind/length. Fails safe in most cases (section not found / BLIND).
- Residual: only uutils sha256sum was exercised; GNU coreutils behaviour for CR/escaped lines is untested.

## CAT-REL-CHECK-1 Opus round-2 re-review (GO-with-follow-ups, 137 tests, 76/76 mutants, 7/7 reviewer mutants killed)
- RC2-1 (nit, source): `versionCode = 7 /* x */ /* y */` (two consecutive block comments) FAILs; fails safe; README could say so.
- RC2-2 (nit, source): a ``` fence closed by ~~~ toggles state wrongly; fails safe; already registered above.
- RC2-3 (nit, tests): no test where the only CR is mid-line (e.g. `SHA256 (a\r) = H`); the double-CR test covers the branch.
- RC2-4 (minor, process-doc): GNU coreutils sha256sum not exercised; CR handling UNCONFIRMED there (strip-one/refuse-other is safe either way).

## CAT-PREEXIST pre-existing test failures (retest of main 563617b1 vs v2.4.1, 2026-11-12)
Evidence: `docs/qa/retest-20261112/RETEST_main_563617b1.md` (PROVISIONAL summary). CAT-PREEXIST-1..9 (the 6 Go + 14 web failures) fail identically on `v2.4.1`; none of those is a regression of 2.4.2. CAT-PREEXIST-10 is different: it PASSED on `v2.4.1` and failed 1 of 3 runs on main under load; regression UNCONFIRMED. Status: Queued; root cause NOT investigated (UNCONFIRMED).
- CAT-PREEXIST-1 (Bug): Go `config` TestDocsAdminPasswordExamplesAreAllRejected fails (docs example passwords vs rejection list).
- CAT-PREEXIST-2 (Bug): Go `internal/nilslice_guard` TestNoUnnormalisedNilListReturns flags admin_handler.go:121, collection_handler.go:301, user_repository.go:295, smb_hostvet.go:111.
- CAT-PREEXIST-3 (Bug): Go root `catalogizer` TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions: DELETE /api/v1/logs/share/999999 expected 500 got 404 (test alone takes ~513 s).
- CAT-PREEXIST-4 (Bug): Go `internal/tests/examples` TestUserRepositoryExample: nil-pointer panic.
- CAT-PREEXIST-5 (Bug): Go `services` TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded fails.
- CAT-PREEXIST-6 (Bug): Go `tests` TestConversionIntegration fails.
- CAT-PREEXIST-7 (Bug): Web `module-registry` (7 tests) fail.
- CAT-PREEXIST-8 (Bug): Web `AuthContext.integration` (7 tests) fail.
- CAT-PREEXIST-9 (Bug): Web load failures: Avatar, EmptyState, LoadingSpinner, WebSocketContext, WebSocketContext.connectionStatus, websocket test files.
- CAT-PREEXIST-10 (Bug): Go `tests/stress` TestCacheStress_HighThroughput/CacheWriteThroughput load-flaky (main 2 pass / 1 fail at 74 ops/s; v2.4.1 pass); decide threshold or load gating.

## CAT-REL-CHECK-1 reviewer findings on the v2.4.2 prep (2026-11-12)
- RC-3 (important): `check_release_consistency.sh` silently passes a VERIFIED path that has no SHA256SUMS manifest. Accept `SHA256SUMS.txt` as well as `SHA256SUMS`; treat a missing manifest as BLIND or at least count it; check that the manifest lists every file of the directory.
- RC-3b (A5/A7): a deleted manifest entry plus a tampered file passes. Check manifest completeness against the directory contents.

## CAT-PREEXIST-1..6 landed (2026-11-12) — PROVISIONAL, Opus round-2 GO-with-follow-ups
Items 1-6 are fixed on main (merge 9e299503; each target test GREEN on the branch, RED on the parent, reviewer mutations M1-M6 caught). Root causes: stale test/docs vs code (FACT). Review follow-ups (all open, Queued):
- PX-F1 (Bug, minor): nilslice guard `strings.HasSuffix` match crosses directories (a same-named file in another directory can take over an accepted slot while the original is fixed); match the full scan path or include the parent dir in each key.
- PX-F2 (Task, minor): nilslice detector `offenders` keys declarations by name per function, so a second same-named `var` in a closure is not reported; key by token.Pos/object and add a two-declaration fixture.
- PX-F3 (Bug, minor): DELETE /api/v1/logs/share/{id} 404 body leaks internal error text (`sql: no rows in result set`).
- PX-F4 (Task): examples test uses assert before dereference (cascade panic) - use require; no test covers the protected-last-admin Delete path.
- PX-F5 (Task): pdf limit test restates the formula with the same constants (reviewer mutant docIndexerSlots 1->2 survives) - assert literal 512<<20.
- PX-F6 (Task): TestConversionIntegration hand-written schema - build via RunMigrations.
- PX-F7 (Task, optional): config census should treat `...` as an elision, not an empty password.
CAT-PREEXIST-3 and -4 register notes (513 s runtime, nil-pointer panic) did not reproduce as stated: the test was ~16-20 s; the panic was a test-only cascade from stale sqlmock expectations.

## CAT-PREEXIST-7/8/9 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups (merge of fix/preexist-web)
Fixed at unit level: module-registry (7) + websocket load failures via a test-only `test.alias` fallback to submodule `src/index.ts` when `dist/index.js` is missing (8 of 9 `@vasic-digital/*` packages track no dist); AuthContext.integration (7) via jsdom Storage in `test-setup.ts` (Node 26 global localStorage is unusable). Reviewer: base 5 files/14 tests RED, branch 170 files/2680 tests GREEN. Avatar/EmptyState/LoadingSpinner failures did NOT reproduce (environmental in the retest; cause UNCONFIRMED).
- PW-F1 (Task, minor): `'react'`/`'react-dom'` entries of `resolve.dedupe` untested (reviewer mutant survives); add a test rendering a submodule hook component or reword the comment (role is resolution, not deduplication).
- PW-F2 (Task, minor): add a guard test `localStorage instanceof Storage`, also under `--localstorage-file`.
- PW-F3 (Task, nit): document that a stale untracked `dist/` silently wins over current `src/` in tests.
- PW-F4 (Task): run the production-build effect of `resolve.dedupe` once through `scripts/lib/build-catalog-web.sh` (UNCONFIRMED).

## PW-F1..F3 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups (merge of fix/pw-followups)
PW-F1 (comment), PW-F2 (`jsdomStorage.test.ts`, RED on pre-fix setup in plain and `--localstorage-file` legs) and PW-F3 (comment) are on main; full web 171 files / 2682 tests GREEN in the reviewer's run. Reviewer follow-ups (open, Queued):
- PW-F5 (Task, minor): `jsdomStorage.test.ts` mutation M3 survives (localStorage and sessionStorage sharing one store); add `expect(localStorage).not.toBe(sessionStorage)`, a cross-isolation check and a sessionStorage round-trip; replace the tautological `globalThis.localStorage toBe window.localStorage` (window === globalThis under vitest jsdom).
- PW-F6 (Task, nit): PW-F3 comment says "stale, untracked dist/"; a tracked dist (ui_components_react) also wins over src; reword, and note dist/ is not gitignored in the other 8 submodules.

## PX-F1..F7 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups (merge of fix/px-followups)
Reviewer ran full `./handlers` (436 s, rc=0), the real-binary OpenAPI contract test (PASS 17.5 s, status codes unchanged), and mutants for F1/F4/F5 (killed). Open follow-ups (Queued):
- PX-F8 (Task, minor): F3 test covers only RevokeLogShare; mutants (revert GetLogEntries site, leak err.Error() in 403 and 400 branches) survive. Add a table test over all 5 handlers incl. wrapped-403 and a 400 with marker string.
- PX-F9 (Bug, minor): close the class: 9 remaining `http.Error(w, err.Error(), …)` in handlers/log_management (~lines 107,136,178,258,265,408,488,536,561); align docs/api/openapi.yaml (~5635, ~5776) with the fixed bodies.
- PX-F10 (Task, minor): F2 residual: `checked`/`assigned` sets still keyed by name (outer `out` missed when an inner closure declares `out`); key by id.Obj, add probe test.
- PX-F11 (Task, minor): F7 widening mutants (HasPrefix ".", HasSuffix "...", Contains "...") survive; add negatives ".hunter2", "secret...", "pa...ss".
- PX-F12 (Task, nit): F6 commit/comment wording claims the old schema missed `conversion_jobs.warning`; it already had it.

## PW-F5/F6 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups (merge a1fbdd12)
New jsdom Storage tests kill 6 reviewer mutants. The PW-F6 comment claimed dist/ is not gitignored in the other 8 submodules; the reviewer measured the opposite (all 8 gitignore it, only ui_components_react tracks dist) and the comment was corrected on main. Note: borrowing another clone's node_modules makes the `@vasic-digital/*` links follow that clone's submodules (Avatar/EmptyState/LoadingSpinner then fail to resolve; confirmed by an overlay toggle).
- PW-F7 (Task, minor): isolation probes use only `iso-`-prefixed keys; mutant M7 (shared store for other keys) survives; probe an unprefixed key.
- PW-F8 (Task, nit): `clear`, `key`, `length` of sessionStorage are not asserted (mutant M8 survives).

## PW-F7/F8 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
New jsdom Storage tests kill reviewer mutants M1-M4, M7, M8 (M1 survived the old test). Open (Queued):
- PW-F9 LANDED (CAT-PWF9, PROVISIONAL) (Task, minor): mutant M5 survives (localStorage.removeItem also removing from sessionStorage); set the same key in both, remove from one, assert the other keeps its value, both directions.
- PW-F10 LANDED (CAT-PWF9, PROVISIONAL) (Task, minor): mutant M6 survives (key() order/index); assert {key(0), key(1)} as a set equals the two stored keys.
- PW-F11 accepted, no change (nit) (CAT-PWF9, PROVISIONAL; cleanup redundancy left, harmless and finally-guarded) (nit): cleanup removes 'k'/'iso-*'/'keep' unconditionally; loops remove keys redundantly (harmless).

## PX-F8..F12 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
All 9 remaining `err.Error()` sites in log_management_handler.go now write fixed bodies (status codes unchanged; full ./handlers rc=0, real-binary contract test PASS). Reviewer mutants of its own survive in several areas; open follow-ups (Queued, all minor unless noted):
- PX-F13: log error matrix tests assert status + no leak but not the exact body (M8-a/b survive); assert exact `Not found`/`Access denied`/`Unsupported format`/`Internal server error`.
- PX-F14: PX-F9 test wraps only sql.ErrNoRows for 404 sites; add a plain non-sentinel error case (M8-d survives).
- PX-F15: nilslice guard clean-twin probes for `:=` redeclare of a named result, `fill(&out)`, range key, `!=` nil check, checked-only named result (M10-a..e survive).
- PX-F16: census negatives only ASCII; add `…hunter2`, `pa…ss`, `.`, `....` (F11-a..d survive).
- PX-F17 (observability): writeFixedLogError/writeLogServiceError discard the service error; log it (redacted) server-side before writing the fixed body.
- PX-F18 (class, §11.4.276(C)): 122 `err.Error()` uses remain in non-test catalog-api/handlers (error_reporting 15, auth 8, configuration 7, user 5; openapi ~5322/5464/5522 document three of them); enumerate the class and decide per site.
- PX-F19 (nit): 404 doc lines say any other service error returns 404 (a server failure shows as `Not found`).
Agent note: do not use generic file names in the shared scratchpad (an agent's `W` file was overwritten by another).

## PX-F15/F16 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
Test-only; 12 detector cases + 4 census negatives; all author mutants die. Reviewer mutants that survive (Queued, minor):
- PX-F20: case `checked-only-named-result-clean` pins a known false negative as clean (`if out == nil {}` with an empty body); rename to show current behaviour and track the false negative.
- PX-F21: add clean twins for the reversed `nil == out` (G5) and for a named result assigned in non-first LHS position `x, out = a, b` / `e, out := …` (G7).
- PX-F22: add negative `secret…` (suffix ellipsis, E6). Nits: `pa<ss` (Contains `<` widening, E5); `for _, out = range c` is a detector false positive (G6).
## CAT-OPDEC-B (operator decisions 2026-11-12, branch fix/opdec-b; PROVISIONAL, review owed)
- CAT-QA-SONAR-5 DONE (PROVISIONAL): the wizard PostInstall `file_create` step now really loads its declared `template` (`catalog-api/services/configuration_wizard_service.go` `loadConfigTemplate`): plain file name only (no path separators), looked up first in the operator templates directory (`$CONFIG_PATH/templates`, default `./config/templates`), then in the built-in set embedded from `catalog-api/services/wizard_templates/` (`config_template.json` ships there so a default deployment works with no file on disk). The template is deep-merged under the session configuration (session wins). A missing, unreadable, non-JSON or path-escaping template is an error (a Required step fails finalization; nothing is written). RED: `docs/qa/` not used; RED run saved by the author at the scratch clone (`red_sonar5.txt`: 5 of 6 new tests FAIL on the unfixed code, the sixth was the shipped-file test); GREEN: `go test -run 'CAT_QA_SONAR_5|Wizard' ./services` ok. Two older characterization tests that pinned the ignored-template behaviour were updated (`..._TemplateParamDoesNotChangeOutput` replaced by `..._UnknownTemplateIsError`). Sonar re-run owed.
- FU2-4 DONE (PROVISIONAL, operator chose removal 2026-11-12): `catalog-web/src/lib/keyboard.ts` (`onActivateKey`) and `src/lib/__tests__/keyboard.test.ts` removed. Proof before removal: `grep -rn onActivateKey` over `src` found only the helper and its own test; `git log -S onActivateKey` shows it added in e823f697, with its last caller removed in 6c063630 (never rewired since). After: `tsc --noEmit` clean, `vitest run src/lib`: the same 7 failures (module-registry needs the web submodule sources; websocket.test.tsx suite) before and after the removal, keyboard tests (3 files of 35 → 34) are gone, no new failure.
- SECQ-1 (decision 2026-11-12) status: the premise in the SECQ-1 queue entry above is stale. The storage-root client already refuses loopback/link-local/metadata by default and honours `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` (landed 2026-11-05 via the operator-class policy `netguard.OperatorTransport`, which also guards the sync client). It differs from the strict `netguard.Transport()` only in that RFC 1918 / ULA / CGNAT stay reachable by default so a LAN NAS root works; `CATALOGIZER_OPERATOR_DENY_PRIVATE=1` makes private ranges refused unless allow-listed. Documented in `docs/ENV_VARIABLES.md` / `docs/OUTBOUND_HTTP_CLIENTS.md`. Whether the default should become strict (private refused unless allow-listed) is OPEN for the operator: it would break a LAN NAS root until `CATALOGIZER_OUTBOUND_ALLOW_CIDRS` is set.


## OPDEC-B landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
Wizard template loading + onActivateKey removal; full ./services rc=0. Follow-ups (Queued):
- OPB-I1 [LANDED CAT-OPB-FU, PROVISIONAL] (important, test hygiene): TestConfigurationWizardService_FinalizeConfiguration writes the tracked catalog-api/services/config/config.json (CWD-relative ./config/config.json file_path ignores configPath); point test at temp dir or untrack artifact (investigate first, §11.4.124).
- OPB-M1 [LANDED CAT-OPB-FU, PROVISIONAL: rollback of config.json on a failing Required step] finalizeConfiguration writes <configPath>/config.json before post-install actions, so a failing Required step leaves it written. OPB-M2 [LANDED] symlinked template inside operator dir is followed outside. OPB-M3 [LANDED] tests for surviving mutants: unreadable operator file must error (no fallback); backslash name rejected. OPB-M4 correct red_sonar5 evidence note. OPB-M5 [LANDED] add CATALOGIZER_OPERATOR_DENY_PRIVATE / CATALOGIZER_OUTBOUND_ALLOW_CIDRS to docs/ENV_VARIABLES.md.
- OPB-N1 [LANDED] JSON null template accepted as empty; N2 MkdirAll before load; N3 0644 config may hold secrets.

## PX-F18 enumeration doc review: NO-GO (2026-11-12) — doc NOT landed
Reviewer found an unlisted class: utils.SendErrorResponse(c,status,msg,err) puts err.Error() into `details` — 109 call sites in 11 files (media_entity 35, collection 15, comic_pages 14, stats 10, media 10, search 6, recommendation 6, challenge 5, browse 5, media_browse 2, middleware/auth.go 1; auth.go:167 sends JWT parse text on 401). Also missed: internal/metrics/health.go:157 (unauthenticated /health/deep leaks DB ping error), monitoring.go:251 (unwired); auth rows should be LEAK; middleware rows latent; limits text wrong; tool/main.go errSite misses bare `err` args. Queued: PX-F23 rework doc + fix detector, then fix classes (SendErrorResponse helper first: strip Details in production).
## CAT-OPDEC-A (2026-11-12, operator decisions; PROVISIONAL, branch fix/opdec-a, not pushed)
- Redis limiter fallback: `RedisRateLimiterConfig` gained `Fallback gin.HandlerFunc`. When Redis errors after startup, `RedisRateLimit` and `SlidingWindowRedisRateLimit` hand the request to the fallback instead of answering 503; with no fallback they still fail closed (503). `main.go` wires the in-memory tiers as fallback: auth tier 600/min, default tier 2000/min (the limiters that Redis replaced). `/login` and `/register` always stay on the in-memory 30/min limiter. Fallback is decided per request, so Redis is authoritative again as soon as it answers (counters kept by Redis survive the outage). One log line per 30 s ("Redis rate limiter unavailable; using in-memory limiter fallback") carrying the error text only. `TokenBucketRedisRateLimit` is unchanged (still fails open; not wired in main.go). Open: during an outage each process counts alone (per-process limits, as with the default in-memory mode); `docs/BACKEND_HARDENING_AUDIT.md` wording is still not corrected.
- Role 2 is no longer always assignable: `allowedToAssignRole` compares role 2's permissions against the caller like any other role (the `defaultUserRoleID` baseline and constant are removed). Effect: a user manager lacking any permission of role 2 now gets 403 when assigning role 2 on create or update, so such a manager can no longer create ordinary users until its role covers role 2's permissions. Tests: `TestAZ_OPDECA_Role2IsComparedAgainstCaller` (real stack), `TestRedisRateLimit_OutageFallsBackToInMemory`, `..._OutageWithoutFallbackStillFailsClosed`, `..._RecoversToRedisAfterOutage` (unit-class, miniredis).

## OPDEC-A landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
Reviewer-surviving mutants / findings (Queued, minor): OPA-1 no test for SlidingWindowRedisRateLimit fallback (M2) nor main.go Fallback wiring (M9); OPA-2 log throttle untested (M3/M4: observer-core test, 1 line per 30 s); OPA-3 ~500 ms added latency per request during outage (circuit breaker / short-circuit after recent failure); OPA-4 correct "no address" wording (log carries err verbatim) and SlidingWindow doc comment still says fails closed; OPA-5 throttle state is package-global across tiers; web client/operators should know a manager re-sending a role_id lacking role permissions gets 403. Real Redis not exercised (miniredis only).

## CAT-PXF23 (2026-11-12, branch fix/pxf23; PROVISIONAL, review owed)
- DONE (PROVISIONAL): `utils.SendErrorResponse` no longer sets `details` for ANY status (err is logged server-side with status and request_id; body = success+error only). Decision: all callers were read for 400/409/422: the err values there are `strconv`/`ShouldBindJSON`/time-parse errors (Go internals, not user-facing validation strings), and `catalog-web` does not read `details`, so dropping everywhere is conservative. `health.go` ping failure message is now fixed "Database ping failed" (error logged). Tests: `TestSendErrorResponse_DoesNotLeakInternalErrorText`, `TestCheckDatabase_PingErrorNotLeaked`; six challenge tests that pinned `details` now assert its absence.
- PXF23-F1 (Queued): monitoring.go:251 leak path is unwired (not reached) - wire-or-remove investigation first (§11.4.124).
- PXF23-F2 (Queued): tool/main.go `errSite` detector misses bare `err` arguments to SendErrorResponse; fix detector.
- PXF23-F3 (Queued, UNCONFIRMED scope): other paths still emit `details` NOT via the helper: handlers/sync_handler.go (`"details": err.Error()` literals, documented in docs/api/SYNC_API.md and pinned by sync_handler_test.go:406,709), internal/middleware ErrorHandler (bind errors, middleware_test.go:334), services/error_handling_integration_test.go bodies. Also callers that concatenate err.Error() into `message` (e.g. handlers/collection_handler.go:335 "Invalid smart_rules: "+err.Error()). Enumerate and decide.
- PXF23-F4 (Queued): no end-to-end httptest of the inline /health/deep route in main.go; unit test covers the checker message only.

## PXF23 landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
Reviewer follow-ups (Queued): PXF23-R1 (minor, source) collection_handler.go:353 verifyRules drops client-correctable smartrules.ValidationError text (put it in message like parseSmartRules); same for gin binding messages. PXF23-R2 health test must marshal whole ComponentHealth/response (mutant Latency: err.Error() survives). PXF23-R3 leak test must check response headers (X-Error-Detail mutant survives). PXF23-R4 test that error is actually logged (zap observer; helper and health log-nothing mutants survive). PXF23-R5 extend PXF23-F3 list by file: user_handler.go:172,177 (raw DB error in details on 500), middleware/input_validation.go:363, auth_handler.go:39,59,76,203, main.go:1306,1337, plus err.Error() in message/error in playback(8), scan(6), playlist(5), service_handlers(3), smb_discovery(3), conversion, asset, copy handlers. PXF23-R6 docs: catalog-api/docs/README.md:226, examples.md:700,715 still show general `details`. UNCONFIRMED: submodules/catalogizer_api_client_ts reads of `details`.

## OPB-FU landed (2026-11-12) — PROVISIONAL, Opus GO-with-follow-ups
Follow-ups (Queued, minor): OBF-1 rollback `hadPrevious := readErr == nil` deletes an existing unreadable config.json on Required-step failure (use !os.IsNotExist; do not discard restore/remove errors silently). OBF-2 I1 test only catches on clean tree (use mtime/sentinel); untrack tracked artifact catalog-api/services/config/config.json after §11.4.124 investigation. OBF-3 ENV_VARIABLES.md: ALLOW_CIDRS is honoured by netguard.Transport() (sync client), ignored only by StrictClient users; "never re-admit metadata" too broad (fd00:ec2::254 is ULA); DENY_PRIVATE row "loopback refused in every mode" wrong on operator path. OBF-4 tests for surviving mutants: symlinked CONFIG_PATH, multi-step Required rollback, unreadable pre-existing file, `{}` valid template. OBF-5 basic template file_create overwrites finalize metadata (version, generated_at, user_id): dead write, pre-existing; decide distinct file name (operator). Nits: ".." in file_path unconfined (code-constant templates only); NewConfigurationWizardService(nil) tests create empty services/config dirs; CONFIG_PATH documented as file path in RENAME_DETECTION_DEPLOYMENT_GUIDE.md:113 cannot work.
## PXF23-R landed (2026-11-12, branch fix/pxf23-r; CAT-PXF23-R; PROVISIONAL, review owed)
- PXF23-R1 LANDED: `verifyRules` puts the smartrules `ValidationError` text into the 400 message (like parseSmartRules); non-validation errors stay generic 500. Tests `TestVerifyRules_*` (RED observed first). Gin binding messages NOT changed (UNCONFIRMED safe fixed format; binding text can include Go field/type names and raw values) - stays generic, documented here, still Queued.
- PXF23-R2 LANDED: `TestCheck_PingErrorNotInAnyResponseField` marshals the whole ComponentHealth and HealthCheckResponse (mutant `Latency: err.Error()` dies).
- PXF23-R3 LANDED: `TestSendErrorResponse_HeadersDoNotLeakErrorText` (mutant `X-Error-Detail` dies).
- PXF23-R4 LANDED: zap observer tests for helper (message, status, request_id, error) and health (mutants "logs nothing" die).
- PXF23-R5 remains Queued (not in this task).
- PXF23-R6 LANDED: catalog-api/docs/README.md and docs/examples.md no longer show a general `details` field.

## PXF23-R review follow-ups (2026-11-12) — Opus GO-with-follow-ups
PXR-1 header leak test runs only without request_id (mutant: Warning header leak only when request_id set survives); run with c.Set("request_id") too. PXR-2 catalog-api/docs/README.md:229 "never sent to the client" is false: qualify to SendErrorResponse paths, name exceptions (sync_handler.go:40, PXF23-R5 list) and fix the "own pages" claim. PXR-3 (nit) verifyRules 400 passes err so a client-correctable 400 logs at Error level; parseSmartRules passes nil. PXR-4 (minor, pre-existing) liveCount (collection_handler.go ~131) and items listing (~309) map every Evaluate error to 500; an anonymous viewer of a public smart collection using a user-scoped field likely gets 500 not 4xx (inferred, not reproduced).
## CAT-PXF23-R5 landed (2026-11-12) - PROVISIONAL, review owed
Closed (real-stack tests `handlers/pxf23_r5_leak_test.go`, `middleware/pxf23_r5_input_validation_test.go`; marker `pq: connection refused host=db` injected via SQLite triggers): all `err.Error()` / `details` reaching a response in `handlers/auth_handler.go` (gin + net/http variants), `handlers/user_handler.go` (CreateUser `details` removed on 409 and 500; reset/lock/unlock; password policy), `middleware/input_validation.go:363` (only `validationError` text is returned; invalid server pattern and guardrail internals map to fixed text) and `main.go` admin routes. `services/auth_service.go` gained sentinels (`ErrInvalidCredentials`, `ErrAccountLocked`, `ErrAccountDisabled`, `ErrInvalidRefreshToken`, `ErrSessionExpired`, `ErrSessionNotFound`, `ErrCurrentPasswordIncorrect`) and `PasswordPolicyError`, texts unchanged. Behaviour change: std `ChangePassword` returns 500 (was 400) for an unexpected store error; 400 stays for known client errors. Login/refresh keep 401 for unknown internal errors with fixed text.
Open (Queued):
- PXF23-R5a: `main.go` admin routes (image-quality revalidation sample, video thumbnails) are inline closures with no httptest; the fix is UNCONFIRMED by a test. Extract to handler funcs and add RED tests.
- PXF23-R5b: other `err.Error()` sites listed in PXF23-R5 (playback, scan, playlist, service_handlers, smb_discovery, conversion, asset, copy handlers) and PXF23-F3 remain open; not in this task's file scope.
- PXF23-R5c: `handlers/user_handler.go` still returns decoder/ValidateRequestBody text only via fixed strings; `ValidatePassword` text is intentionally returned (policy messages).
- PXF23-R5d: gin `ShouldBindJSON` text in other handlers (Key: ... Field validation) leaks struct names; only RegisterGin was fixed here.

## PXF23-R5 review follow-ups (2026-11-12) — Opus GO-with-follow-ups; PXF23-R5 itself is now LANDED (PROVISIONAL) for auth_handler/user_handler/input_validation/main.go admin routes
R5-F1 (minor, test) no handler test that locked/disabled/session-expired/pending logins keep their messages (mutants dropping ErrAccountLocked/ErrAccountDisabled survive); add to IntentionalMessagesPreserved for gin and std. R5-F2 (minor, test) no test with a GuardrailEngine: assert result.Results[0].Error marker absent from response. R5-F3 (minor, source) errUserChangedDuringLogin retry hint is now "Authentication failed"; export+map or record decision. R5-F5 (nit) invalid server-side custom pattern answers 400, would fit 500. R5-F6 main.go admin closures untested (R5a). PXR-2 README claim "never sent to the client" still to qualify; remaining sites R5b/R5d (playback, scan, playlist, service_handlers, smb_discovery, conversion, asset, copy handlers, sync_handler.go:40). Std AuthHandler.ChangePassword is not routed in main.go (live route is MediaQueryHandler.ChangePassword), so the 400->500 change reaches no live client. gofmt main.go diff pre-existing (1186 lines).

## CAT-R5-FU landed (2026-11-12, branch fix/r5-fu) - PROVISIONAL, review owed
- R5-F1 [LANDED]: `TestPXF23R5_AccountStateMessagesPreservedOnLogin/OnRefresh` (real stack, gin and std): locked, disabled, pending (correct and wrong password), session-expired and account-disabled-after-login keep their messages. Mutants dropping ErrAccountLocked / ErrAccountDisabled / ErrSessionExpired from `authClientMessage` die.
- R5-F2 [LANDED]: `TestPXF23R5_GuardrailInternalErrorTextNotReturned` uses a real `guardrails.Engine` whose rule error carries the marker; body and headers are marker-free. Mutant (public text includes `Results[0].Error`) dies.
- R5-F3 [LANDED]: `ErrUserChangedDuringLogin` exported (`services/auth_service.go`) and mapped in `authClientMessage`; the retry hint "account changed during login, please retry" is preserved. RED: handler hint test and services sentinel test fail on the old code. Observation (not changed): a WRAPPED client sentinel returns the wrapper text too; services return the sentinels unwrapped.
- R5-F5 [LANDED]: an uncompilable server-side custom pattern now answers 500 `internal_error` with fixed text (was 400 `validation_failed`); new `serverValidationError`. Other validation failures still 400. RED observed (400 vs 500).
- PXR-1 [LANDED]: header leak test runs with and without `request_id`; mutant "Warning header leak only when request_id set" dies (and survives the old test).
- Still Queued: R5-F6 / PXF23-R5a, PXR-2..4, R5b/R5d.

## R5-FU review follow-ups (2026-11-12) — Opus GO-with-follow-ups; fix/r5-fu LANDED (PROVISIONAL)
R5FU-F-A (minor, source-defect, latent) `authClientMessage` returns `err.Error()` for any chain containing a client sentinel; a future `%w` wrap with internal text would reach the client (mutant M5 survives). Fix: return the matched sentinel's own text / `PasswordPolicyError.Msg`; assert `Equal` in the wrapped-case test.
R5FU-F-B (minor, source-defect) `validateSecretValue` (input_validation.go ~319-327) still returns a plain error for an uncompilable pattern (field `password` -> 400 `validation_failed`; other field -> 500). Use `serverValidationError` there too and test with `password`. The R5-F5 wording "now answers 500" overstates until then.
R5FU-F-C (nit) `ErrSessionNotFound` entry in `authClientMessage` is unreachable from current callers (equivalent mutant M3b): remove or document.
R5FU-F-D (nit) message assertions use `Contains`; switch to `Equal` together with F-A.
R5FU-F-E (nit) import grouping in pxf23_r5_leak_test.go; `r5State` builds SQL by concatenation (test-only, fixed literals).

## CAT-D9-SUSPEND (2026-11-12, branch fix/d9-suspend) - PROVISIONAL, review owed
Root cause (fact, reproduced RED): `UserRepository.Update` and `UpdateGuarded` never wrote `is_locked`/`locked_until`, and `AdminHandler.UpdateUser` did not revoke sessions for `status:"suspended"` (the user stays `is_active`), so a suspension returned 200 and changed nothing; "active" could not lift a lock either. Fixed: both writers persist both columns, suspend sets a permanent lock and revokes all sessions (fail closed, before the write), "active" clears lock and expiry, last-admin guard counts a locked admin as removed, `AuthService.GetCurrentUser` refuses a locked account.
Round-1 review (Opus, NO-GO) fixes, same branch:
- D9S-F1 [LANDED]: `LockAccount` (repository level, so every caller) now revokes the user's sessions after writing the lock; JWT-only routes lose access (reviewer probe RV2). The production session checker was left unchanged (smaller, no per-request extra read).
- D9S-F2 [LANDED, reviewer F1]: `Update`/`UpdateGuarded` no longer write `is_locked`/`locked_until` at all; a stale read can neither clear nor set a suspension (RV3/RV3b, `TestD9S_GeneralWritersLeaveLockAlone`). A dedicated `SetSuspended(id, bool)` writes only the lock columns, with an atomic last-admin guard in the same statement (PG advisory lock mirrored from UpdateGuarded, PG path not run live).
- D9S-F3 [LANDED, reviewer F3]: `LockAccount` never replaces a permanent suspension; `UnlockAccount` refuses to lift one (`ErrAccountSuspended`, handler 409); only admin status "active" lifts it (RV4).
- D9S-F4 [LANDED, reviewer F5/F6]: user list and the last-admin clause use the effective lock (an expired timed lock is not suspended and does count as an active admin, RV5, `TestD9S_RepoRefusesLockingLastActiveAdmin`); a permanent suspension answers "account is suspended" (new `ErrAccountSuspended`), timed locks keep "account is temporarily locked".
- D9S-F5 [QUEUED]: the admin update performs revoke, general update and lock write as separate statements (no single transaction); each step is safe alone and a retry converges, but a crash between update and `SetSuspended` leaves sessions revoked and the lock unwritten (retry fixes).
- D9S-F6 [UNCONFIRMED]: PostgreSQL not run live; time comparison of `locked_until` in the admin-count clause relies on same-timezone time.Time parameters (SQLite string compare, PG timestamptz).
- Pre-existing, not in scope: `Login` previously returned ErrAccountLocked for any `is_locked` row on an inactive user; it now uses the effective lock.
## PXF23-R5b (2026-11-12) — remaining client-reaching err.Error() sites in catalog-api handlers: LANDED (PROVISIONAL, awaiting independent review); Roadmap-Item CAT-PXF23-R5B
Closed: playback (8), scan (CreateStorageRoot/GetStorageRoots/QueueScan/loadStorageRoot, local-root and URL-policy messages), playlist and service_handlers (sentinel/validation text only), conversion (job-state sentinel), asset, collection smart-rules text (validation text only, no wrapped chain), configuration (7) and error_reporting (15) std handlers, sync_handler (`details` is now a fixed class text per status, docs/api/SYNC_API.md updated, status codes unchanged), auth_handler.go:58 (pending account), internal/handlers smb_discovery (3), copy.go, localization_handlers.go:301,365. Guard: handlers/pxf23_r5b_leak_test.go (go/ast sweep over handlers and internal/handlers with positive and negative controls, plus real-stack RED/GREEN tests). Helpers: handlers/client_errors.go.
Open, with file:line:
- R5B-1 (minor, source) internal/middleware/middleware.go:80-91 ErrorHandler returns `err.Error()` as `details` (gin.ErrorTypeBind) and as `error` (gin.ErrorTypePublic). No non-test code produces those error types today (grep for Bind/BindJSON/AbortWithError/ErrorTypePublic), so it is a latent sink; fix with a fixed message and keep middleware_test.go:238 truthful. Outside this task's handler scope.
- R5B-2 (minor, test) the sweep is a name/identifier heuristic: an error carried in a variable (`msg := err.Error()` then written) is not seen. Reviewed by hand: collection_handler.go:585 (isNotFoundError, not written), conversion_handler.go:72 (reason text authored by services/conversion_service.go:84, kept), service_handlers.go:420 (compared, not written), user_handler.go:168 (already R5). UNCONFIRMED for any new code; a type-aware (go/types) sweep would close it.
- R5B-3 (minor) scan_handler.go:378,384,401 send `services.ScrubErrorFor(err, root)` (storage probe errors are the feature; scrubber completeness UNCONFIRMED). Allow-listed in the sweep as SCRUBBED.
- R5B-4 (minor, source) filesystem/local_policy.go:60 puts the operating-system error into the wrapped text; the handler cuts it (localRootMessage) but the cut keys on the literal ": cannot resolve path", so a reworded message would pass the OS text through again. Better: make the error typed.
- R5B-5 (minor, test) no handler-level RED test for playlist (3), service_handlers (3), conversion (1) and collection (2) sentinel/validation sites: covered by the helper unit test (validationText, paramRequiredText) and the sweep only. UNCONFIRMED end to end.
- R5B-6 (minor) status codes left as they were: scan_handler.go loadStorageRoot failure answers 404 for any database error (not only no-rows); sync_handler 409 `details` is the single text "endpoint is not active".

## PXF23-R5b review follow-ups (2026-11-12) — Opus GO-with-follow-ups; fix/pxf23-r5b LANDED (PROVISIONAL)
R5B-F1 (minor, source-defect) sync 400 `details` lost authored validation text ("name is required" etc.); map authored validation sentinels/typed error, keep netguard generic. Operator accepted fixed class text 2026-11-12; this is optional refinement.
R5B-F2 (nit) conversion 409 dropped authored hint suffixes ("cancel it first"…); use sentinelText on a typed suffix.
R5B-F3 (nit, process-doc) SCRUBBED comment wrongly says addresses are scrubbed; ScrubErrorFor scrubs secrets only, so ListStorageRoot 502/404 can carry dial/netguard text with addresses (admin-only, pre-existing).
R5B-F4 (minor, test-instrumentation) sweep allow-list keyed by file+substring (mutant R4 at scan_handler.go:448 survived); key by function.
R5B-F5 (minor, test-instrumentation) sweep misses variable-carried errors (mutants R1 conversion_handler.go:344, R2 collection_handler.go:335, R3 localization_handlers.go:301 survived); make it go/types-aware; add HTTP RED tests for conversion 409, collection smart_rules, localization import/edit, QueueScan local root. Unclassified sink: internal/handlers/eventbus_bridge.go:82-84 and scan status `reason` scrubSecrets(err.Error()) (secrets scrubbed only).

## CAT-R5FU-FU (2026-11-12, branch fix/r5fu-fu) - PROVISIONAL, review owed
- R5FU-F-A [LANDED]: `authClientMessage` returns the matched sentinel's own text / `PasswordPolicyError.Msg`, never the wrapper chain text. RED first: `TestR5FUFA_*` failed on the old code (wrapped and errors.Join chains returned wrapper text incl. a password-hash-like marker). Test is at the `authClientMessage` choke point shared by the gin and std handlers (a real-stack wrapped sentinel is not injectable: services return sentinels unwrapped; UNCONFIRMED end-to-end for a future wrap). Wrapped-case test now `Equal`.
- R5FU-F-B [LANDED]: `validateSecretValue` uncompilable pattern -> `serverValidationError` -> 500 `internal_error`, no field/pattern text. RED first: `TestR5FUFB_SecretFieldBadPatternIs500Fixed` got 400 `validation_failed`.
- R5FU-F-C [LANDED, decision: removed]: `ErrSessionNotFound` is returned only by `AuthService.GetCurrentUser`, which no `authClientMessage` caller (Login, Refresh, ChangePassword, ValidatePassword) reaches; removed from the client-visible list (fails closed to the fallback); `TestR5FUFC_SessionNotFoundFallsBack`.
- R5FU-F-D [LANDED]: message assertions in the account-state / intentional-message tests are now `Equal` on the extracted client message (`r5Msg`); NotContains leak checks unchanged.
- R5FU-F-E [LANDED]: import grouping fixed; `r5State` binds the user name as a parameter (set clause is a fixed test literal).
- Correction: the R5-F5 line above ("now answers 500") overstated for the secret-field path (`password` etc.); true only after R5FU-F-B.
- Still Queued: R5-F6 / PXF23-R5a, PXR-2..4, R5b/R5d.

## R5FU-FU review follow-ups (2026-11-12) — Opus GO-with-follow-ups; fix/r5fu-fu LANDED (PROVISIONAL)
R5FUFU-1 (minor, source-defect) LoginGin ErrAccountPending branch (auth_handler.go ~68) still returns err.Error(); route through authClientMessage / services.ErrAccountPending.Error().
R5FUFU-2 (minor, process-doc+test) comment "Order is irrelevant" in authClientSentinels is wrong: precedence = PasswordPolicyError, then slice order (mutant M5 reverse-iteration survived); correct the comment and add a two-sentinel-chain test.
R5FUFU-3 (nit) "R5-F1" doc comment sits above helpers r5UpdateUserSQL/r5Msg instead of the tests it describes.

## REL-CHECK F1-F5 post-hoc review follow-ups (2026-11-12) — Opus GO-with-follow-ups (branch already landed via 9b484c65; PROVISIONAL)
RCF5-1 (minor, source-defect) scripts/release/check_release_consistency.sh: manifest entry `-` makes sha256sum -c read stdin and counts a verified OK (sha_ok inflated; would wait on a TTY); fail closed on ent == `-` or run sha256sum with </dev/null.
RCF5-2 (minor, test-instrumentation) fence-state logic under-tested: mutants RV1 (`~~~` ignored) and RV3 (fence never closes) survive; add a `~~~` test and a heading-after-closed-fence test.
RCF5-3 (minor, source-defect) `\s{0,3}` fence match treats a tab-indented ``` line as a fence (CommonMark: indented code) and hid a real duplicate `## [2.4.0]` heading; use `^ {0,3}`.
RCF5-4 (minor, source-defect, pre-existing) without --section a section inside an HTML comment is picked as first numbered section.
RCF5-5 (nit, process-doc) scripts/release/README.md says readlink -f; code uses readlink -m.
RCF5-6 (nit) SHA256SUMS itself being a symlink to an outside file is accepted (evidence provenance only).
RCF5-7 (nit, test-instrumentation) whitespace-only manifest lines untested (mutant RV4).
RCF5-8 (nit) only uutils sha256sum 0.10.0 exercised; GNU coreutils untested; the R4 mutants were not independently re-run (load 34-38).
## CAT-SECQ1-A follow-ups (2026-11-12) — per-endpoint address-class pinning, part A (W6, W7, W8, W1, W2); PROVISIONAL
Delivered in netguard: class.go (ClassNever/Private/Public, `ClassOf`, own-interface addresses = never, allow-list re-admits loopback/own as private), class-pinned operator transport (first-connection pin, dial-time `Control` hook, `OperatorTransportFor`, `OperatorDialContextClass`, `OperatorCheckRedirectFor`), 168.63.129.16 refused, allow-list hygiene (log invalid, ignore broad unless `CATALOGIZER_OUTBOUND_ALLOW_BROAD=1`), ASCII-only and non-numeric host rule on both paths. Docs corrected (B10).
- CAT-SECQ1-B (W3) schema, migration and automatic classification of `approved_class` for storage roots and sync endpoints. Until then the approved class is the default (private) and the first-connection pin is the only per-root protection: a name that is already rebound to a private address at the FIRST connection is classified private (accepted limit, same as the analysis §4 honest boundary).
- CAT-SECQ1-C (W4) create/update handlers: resolve, classify, confirmation for private, step-up to widen a class; then pass the stored class to `OperatorTransportFor` / `OperatorCheckRedirectFor` in `services/webdav_client.go`, `filesystem/webdav_client.go`, `filesystem/s3_client.go`.
- CAT-SECQ1-D (W5) one policy for SMB, FTP, NFS storage roots (B3) and sync endpoints (B4); retire `smb_hostvet.go` duplicates keeping its port allow-list; `Dialer.Control` for libraries without a DialContext.
- CAT-SECQ1-E (W9, W10, W11) admin UI; remaining docs; the filesystem-level adversarial regression suite.
- A1 (minor, source) the first-connection pin ignores a second distinct host on the same transport (one transport per root is the assumption; S3 presigned hops to other public hosts are fine, a LAN root that redirects to a public host does not tighten the pin by design). Verify no caller shares one OperatorTransport between roots (none found by grep of netguard.OperatorTransport callers on 2026-11-12; UNCONFIRMED for future code).
- A2 (minor) own-address detection calls `net.InterfaceAddrs()` on every dial; cache with a short TTL if profiling shows cost. Container bridge gateway is still private-class (analysis §9, operator decision).
- A3 (minor) `OperatorCheckRedirectFor` resolves the first request's host again to learn its class; the transport pin is authoritative, the redirect check is an early refusal and can be bypassed only where the dial check still holds.
- A4 (nit, test) port policy for operator HTTP clients (B9) and the wizard TCP probe are not changed here.

## CAT-SECQ1-A review round 1 follow-ups (2026-11-12) — fixes F1-F5 in this branch; PROVISIONAL
Fixed: F1 IPv4-mapped broad allow-list entries; F2 concurrent first connections (first connections are serialised by a gate in classPin); F3 tests for the proxy-path pin, the /1../7 boundary and the real Control hook; F4 own-address migration note (ENV_VARIABLES.md, CHANGELOG) and fail-closed interface lookup (private-range addresses refused when the list cannot be read); F5 truthful webdav_client.go comment; S3 client reuses its operator transport across Connect.
- F5-FU (minor, source) wire the operator policy into `services/sync_service.go` / `services/webdav_client.go` (today the sync client replaces the transport with the strict `netguard.Transport()`, and `GetQuota` has no caller), together with CAT-SECQ1-D.
- F2-FU (minor) the pin is per transport object; persistence across restarts needs the stored `approved_class` (CAT-SECQ1-B/C). First connections of one transport are serialised by a capacity-1 gate: a request queued behind it waits for the first decision (net/http dials with a context detached from the request, so the wait is bounded by the dial timeout of 15 s, not by the waiter's deadline); the queue drains and cannot deadlock. Once the first connection is recorded the gate is skipped.
- F4-FU (nit) a failing `net.InterfaceAddrs()` makes every private-range root unreachable until it recovers or the address is allow-listed; no metric/alert is emitted.

## SECQ-1 A round-3 review follow-ups (2026-11-12) — Opus GO-with-follow-ups; fix/secq1-a LANDED (PROVISIONAL)
S1A3-1 (minor, test-instrumentation) mutant RV3-1 survives (allowListedUnwrapped final `return allowListed(ip)` -> false): add positive-control test admitting an allow-listed non-embedded IPv6 private address (ULA/::1) under DENY_PRIVATE.
S1A3-2 (minor, process-doc) undocumented residual: NAT64 with operator network-specific prefix (RFC 6052, e.g. 2001:db8:64::c0a8:105) is ClassPublic; document in OUTBOUND_HTTP_CLIENTS.md/ENV_VARIABLES.md or add configurable NSP list.
S1A3-3 (nit, process-doc) undocumented residual: ISATAP interface ids (::5efe:a.b.c.d) classed public.
S1A3-4 (nit, pre-existing) internal/netguard/netguard_test.go not gofmt-clean.
## CAT-REL-CHECK-1 RC-3 / RC-3b / RC2-1 / RC2-3 resolution (branch fix/release-checker-rc3; PROVISIONAL)
- RC-3 DONE: a VERIFIED path under an evidence prefix (default `docs/qa/`, `--evidence-prefix`) with no `SHA256SUMS` / `SHA256SUMS.txt` at or above it is now `NOMANIFEST` and FAILs. Known gaps are declared in the tracked `docs/qa/EVIDENCE_WAIVERS.txt` (kinds `no-manifest`, `unlisted`, `bad-entry`; reason mandatory; applied waivers printed as WAIVED and counted). The 6 manifest-less paths of CHANGELOG section [2.4.0] are declared there; `--version 2.4.1` (section 2.4.1) is unaffected.
- RC-3b DONE: each manifest in use must list every regular file of its tree (tracked + untracked non-ignored; symlinks, gitignored files and the manifest files themselves excluded); nested manifests are queued and fully checked. Real-tree findings not in any current changelog section (reported, not waived): `docs/qa/coverart/results/final` summary.txt checksum mismatch (E-1) and `race.console.txt` not listed; `docs/qa/ui-d7-class-20261105/README.md` checksum mismatch.
- RC2-1 DONE: any number of consecutive block comments after `versionCode`. RC2-3 DONE: tests for a mid-line-only CR in BSD and GNU lines.
- Open nit: waiver paths are matched as written (no trailing slash); unused waivers are counted (`unused_waivers=`) but not failed because they may belong to another changelog section.

## CAT-REL-CHECK-1 Opus review round 1 fixes (branch fix/release-checker-rc3; PROVISIONAL)
- `MANIFEST*.sha256` are now manifests (found, verified, completeness-checked, excluded from completeness); manifests must be git-tracked. The four dishonest `no-manifest` waivers for them were removed and the real checks run.
- Evidence paths are decided on the resolved location (`//`, `/./`, `..`, symlink aliases, the prefix dir itself). Control characters in VERIFIED paths and nested evidence dir names are BLIND; `UNLISTED` names are escaped; dir names with glob characters are matched literally (`:(literal)` pathspec); a nested dir that vanished or has no tracked manifest is BLIND.
- Waivers follow 11.4.271: item and expiry mandatory, expired is BADWAIVER, the file must equal HEAD, `PASS-WITH-WAIVERS` is a distinct verdict word (exit 0).
- Remaining honest findings in the real tree are NOT waived unless stated in `docs/qa/EVIDENCE_WAIVERS.txt` (see the hand-back): `stack3-rebuild-live-20261105` lists `live_storage_done.txt` that was never committed (waived, truthfully) and does not list `build_end.txt` / `build_start.txt`; `ui-live-rerun-20261105/MANIFEST.sha256` covers only 9 PNGs and leaves about 31 files unlisted.

## CAT-REL-CHECK-1 Opus rounds 2-3 (fix/release-checker-rc3, PROVISIONAL)
- Round 2 (NO-GO, fixed): I-1 local PASS while a clone would FAIL (manifest staged-only; manifest or covered file edited only in the working tree) -> BLIND when a manifest tree differs from HEAD. I-2 a `bad-entry` waiver could launder a checksum mismatch -> mismatch is never waivable. M-1 tests for RV2-1 (untracked nested manifest) and RV2-4 (walk-up >= 3 levels). M-2 a TAB or other control character in a VERIFIED token is BLIND.
- Round 3 (NO-GO, fixed): R3-1 the HEAD check also covers `docs/CHANGELOG.md`, every version file read and every existing cited path (one `head_check`). R3-2 tests added for the survivors "existence check disabled" (tracked file deleted uncommitted -> MISS) and "waiver HEAD check index-only" (staged-only waiver file -> BLIND). R3-4 a waiver naming a mismatched entry is `BADWAIVER`, not `UNUSED-WAIVER`. R3-5 dead CR strip in the Python helper removed. R3-3 README and the waiver-format comment corrected.
- Survivor record: "existence check disabled" survived the first full run because the suite auto-commits fixtures, which turned the old "tracked path removed" test into a committed deletion; the RAW=1 test now kills it.
- Open (evidence, not tooling): `docs/qa/coverart/results/final/summary.txt` checksum mismatch (E-1), NOMANIFEST for `docs/qa/coverart/README.md` and UNLISTED `race.console.txt` on main remain; evidence regeneration is out of scope here and the mismatch is not waivable.
- UNCONFIRMED: `git status` phantom diffs under autocrlf/attributes would make the checker BLIND on a clean tree (fails safe); behaviour on a shallow clone or with submodule paths in cited evidence is untested.


## REL-CHECK RC-3 round-4 review follow-ups (2026-11-12) — Opus GO-with-follow-ups; fix/release-checker-rc3 LANDED (PROVISIONAL)
RC3R4-1 (minor, source-defect) HEAD-consistency checks use git status/diff, which honour skip-worktree/assume-unchanged flags: a CHANGELOG edited under `update-index --skip-worktree` passes locally (fresh clone FAILs). Fix: compare git hash-object of working file with HEAD:path, or refuse when `git ls-files -v` shows S/h. Affects manifest tree and waiver file checks too.
RC3R4-2 (minor, test-instrumentation) author mutants [4] (equivalent: record as such) and [6] (head-check git-status failure not blind: add fault-injection git wrapper test) survive, so the shipped mutation runner is not green on its own list; run the FULL runner once load < 20 (only a filtered run of 14 mutants was done).
RC3R4-3 (nit) mutant NEW3 (head_check returns at first missing path) near-equivalent.
RC3R4-4 (nit) retargeted symlink cited path gives FAIL (MISS) instead of BLIND; fails safe, message only.

## CAT-WP03-MECH follow-ups (branch fix/wp03-release-mechanisms; PROVISIONAL, gate-logic unit class only)
WP03-R1 Opus round-1 NO-GO (I-1..I-5, M-1..M-4, M-6) addressed in the round-1 fix commit: identity/binding fields are non-empty and shape-validated, missing identity is INCOMPARABLE, same file-sha256 set is the same image, the gate binds verdict sha, board/target_id, suite and the target-read build to the compared image, BLIND/INCOMPARABLE paths rewrite --out. Needs a round-2 review.
WP03-1 (consumer work) nothing yet PRODUCES the inputs: the harness must write the TSV verdict files, meta files (board= must equal the hardware target_id), the hardware record (read from the target, kind=physical, build_sha256 lines), the live-run record and the previous-image baseline run; until then both tools have only fixture-input evidence (11.4.289).
WP03-2 `issue-count-higher` is evaluated over the baseline test set; it cannot fire without `baseline-pass-lost` except when baseline-FAIL tests stay FAIL and a lost PASS is offset by an improvement. Decide whether a stricter per-class count is wanted.
WP03-3 not wired into any release/tag script (11.4.293(F), 11.4.298(E)); wiring plus the first-baseline acceptance rule (BLOCKED-NO-BASELINE until the operator records it) are owed. Gates OWED-GATE-176/185 stay deferrals.
WP03-4 UNCONFIRMED: exit codes 10/11/20/21/22 and the `kind=physical` / `build_sha256` / `board==target_id` conventions are this change's choice, not an operator decision.
WP03-5 equivalent mutants (not run, argued in the harness): the row-count control needle, the comparison-sha hex check and the image_new shape check in the gate (each is implied by an earlier validated equality); dropping the explicit unreadable-targets guard (empty-targets guard gives the same BLIND) and unreadable-new-meta guard (missing key gives INCOMPARABLE).
WP03-R3 Opus round-2 NO-GO (I-6, I-7, M-a..M-d, r1..r3) addressed in the round-3 fix commit: exact verdict `case` (no substring membership), unsafe ids (space, comma, glob) fail closed, `--out` realpath clash refused in both tools, duplicate/repeated image lines rejected, gate cross-checks schema, `new_total` == `tests_total` == `tests_reported`, strict `image_new` shape and NO-WORSE record consistency, input snapshot against TOCTOU. Gate-logic unit class; PROVISIONAL.
WP03-6 the TOCTOU snapshot is not unit-testable (a race); it is argued by construction and its mutant is recorded as equivalent. UNCONFIRMED: no test swaps an input between the two reads.

- SECQ-2 / WD-F1 / WD-F1a re-reproduction 2026-11-12 (evidence `docs/qa/secq2-repro-20261112/`, PROVISIONAL): result not-reproducible-with-evidence. 6 further repeats of the WD-F1a case (mode A reset, 2 MiB) all byte-identical (6/6 ok, detail recorded), both control needles fire. Proposal for the operator: close under section 11.4.90 `not-reproducible` (UNCONFIRMED whether the single 2026-11-05 CORRUPT was a read-back glitch). No fix written.
- SECQ2WD-R1-1 (minor, process-doc): fix/wd-repro README must state the re-run used main f709b892, not the 2026-11-05 build where the single CORRUPT appeared (WriteFile gained HEAD size read-back since); operator weighs this before accepting a 11.4.90 not-reproducible closure.
- SECQ2WD-R1-2 (minor, process-doc): record a target identity for the live run (hash of endpoint host, clean-tree note for the test binary) per 11.4.292(C)/11.4.289(D).
- SECQ2WD-R1-3 (nit): "11 further identical attempts" is conservative; chaos3 mode-A 2 MB also ok (12).

## CAT-D9-SUSPEND round-2 review follow-ups (2026-11-12) — PROVISIONAL
- D9S-M1, D9S-M4, D9S-M5: registered per the conductor's round-2 NO-GO message; their text was not carried into this work session, so they are recorded here by id only and must be filled from the review text (UNCONFIRMED content).
- D9S-F5: admin update does revoke, update and SetSuspended as separate statements (not one transaction).
- D9S-F6: the SQLite `locked_until <= ?` comparison relies on same-timezone time strings.
- D9S-F7: `LockAccount`/`UnlockAccount` still compare `is_locked = ?` without COALESCE; only the last-admin clause treats NULL as unlocked (M6).
- PostgreSQL test `TestD9S_PG_SuspendLockUnlock` is gated on CATALOGIZER_PG_TEST_*; it was not run live by this author.
- D9S-R3-1 (minor, source-defect): NULL is_locked handled inconsistently (admin-count clause treats NULL as unlocked; GetByID scan, session checker, LockAccount/UnlockAccount fail or compare without COALESCE). With a NULL-lock peer the last USABLE admin can be suspended. Fix structurally: NOT NULL migration with backfill, or COALESCE in every read; add a behavioural NULL test (merges with D9S-F7).
- D9S-R3-2 (minor, test-instrumentation): no test covers the RefreshToken lock check (reviewer mutant F survived); adopt reviewer probe TestRV3_SuspendedUserAllPaths (scratchpad rv3/).
- D9S-R3-3 (minor): RefreshToken answers "account is disabled" for a suspended account; return lockedError(user).
- D9S-R3-4 (minor, pre-existing class): Login reveals locked/suspended/disabled state BEFORE the password check (account-state enumeration); hide behind the password like pending.
- SEC-WS-1 (IMPORTANT candidate, pre-existing, own item): /ws (handlers/websocket_handler.go HandleConnection) accepts connections with NO authentication (found by D9 round-3 review). Needs reproduce-first (11.4.292), auth + origin check, and tests.
- D9S-M1/M4/M5: round-2 review text was never supplied to the author; ids only. UNCONFIRMED content.
