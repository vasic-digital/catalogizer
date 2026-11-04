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
- Operator decision: after startup, a Redis outage returns 503 on Redis-limited routes; alternative is fallback to the in-memory limiter.
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
- R3-M2: a non-admin with user.manage / user.create may assign ANY non-admin role, including a role more privileged than their own (only roles that carry `system.admin`, `*` or a matching prefix wildcard are refused).
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
- R3-M2 / CAT-050 FIXED: a non-admin may only assign roles whose every permission it holds itself (role 2, the standard user role, always assignable). OPERATOR DECISION still open: whether this subset rule is the wanted role model; acting on (not assigning to) a more privileged non-admin target is still unguarded.
- CAT-052: `memStore.GetRole` now returns an error for unknown roles like the real repository.
- N1 (review r1, minor): `defaultUserRoleID = 2` is hard-coded as always assignable by a user manager. If an admin later widens role 2's permissions, any user manager can hand them out. Decision needed: compare role 2 against the caller too, or keep the baseline.
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
- P2-g (nit): `GET /smb/discover` and `GET /smb/test` take credentials in the query string (they end up in access logs); the POST forms exist.
- P2-h (operator decision, carried from P3): `/metrics`, `/discovery`, `/ws` and (when `HELIX_PPROF_ENABLED=true`) `/debug/pprof/*` are unauthenticated.
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
- OPEN W5: no outbound SSRF/netguard policy applies to the WebDAV storage-root path on main (`services.GuardProviderURL` guards metadata providers; `isHostAllowed` guards only the SMB probe). The suite shows the canonical guard lets `webdav.yandex.ru` through and refuses loopback, but nothing calls it for a WebDAV root. Branch P2 adds netguard; re-run the live suite there to prove a public https WebDAV host still works.
- OPEN W6: the client has no retry of its own (one `connection reset by peer` on MKCOL and one 30 s PROPFIND timeout were seen); only the scanner wraps reads with `fabric.Retrying`. Writes (PUT, MKCOL, DELETE) are never retried.
- OPEN W7 (service fact, not a defect): the Yandex WebDAV upload rate was about 16 KiB/s, so cataloguing or copying media into it is slow by design; a collection `DELETE` can answer `202` (asynchronous), the suite polls until the directory is gone.
- OPEN W8: the scan leg replaces only the record seam; the database-backed `insertFileRecord` path and the `UniversalScanner` job path (factory-built client with storage-root credentials) were not run against the real service, because the factory-built client has no outbound guard and its `Connect` would issue a PROPFIND on the service root, outside `/Testing`.

- W9 (open): `WebDAVClient.Connect`/`TestConnection` have no retry; the live Yandex service intermittently answers the first PROPFIND with a connection reset or stalls it to the 30 s op deadline. The live test retries (counted in evidence as `transient_retries`); the production client does not (see W6).
- W10 (UNCONFIRMED): one mid-body 60 s idle cancel was seen on a download while the service was degraded; not reproduced since.
- OPEN WD-F1 (UNCONFIRMED cause, possible DATA LOSS): one throwaway probe (output not committed) saw an upload cut by `connection reset by peer`, a retried PUT answer success, and the file stay absent from the directory for at least 55 s, i.e. an acknowledged write may be lost by the service. No committed run shows it (all committed runs have `lost_acked_writes=0`; the live script now FAILs on any). The production `WebDAVClient.WriteFile` has no read-back and no retry, so if the loss is real Catalogizer could silently lose data written to WebDAV. Next step: repeat the probe under evidence capture (status of every attempt, HEAD/PROPFIND after), then decide on read-back verification in the client.
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
- F2: ghostscript is deliberately NOT added (the image is 761 MB; go-fitz already rasterises PDF pages). The ImageMagick PDF fallback now returns a specific "needs ghostscript" error and `/health/deep` reports `convert_pdf` only when `convert` and `gs` are both present. Dockerfile comments and the deployment guide say so.
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
- F6 residual: the PDF path through ImageMagick needs ghostscript, which is deliberately not in the image (go-fitz rasterises); `convert_pdf` is false there and the ImageMagick PDF fallback returns a specific error. Not re-verified on a working gs.

Review round 2 (NO-GO at 43d99475), addressed in the following commit:
- N1/N2/N3 fixed structurally: `sanitizeSVG` re-serialises the SVG from the parsed tree (only SVG-namespace elements, canonical xmlns/xlink declarations, unprefixed/svg/xlink/xml attributes, xlink limited to href/title/role/arcrole/show/actuate/type, no comments/PIs/DOCTYPE/foreign subtrees, attribute values that are absolute paths or URLs refused outside hrefs, duplicate attributes refused, root must be `svg` in the SVG namespace, non-whitespace text before/after the root refused) and the renderer gets only those bytes through the forced `svg:` coder from a private copy.
- N4/N5 adopted as tests (`TestHardenLive_SVGStyleImportOutsideFileRefused`, `TestHardenLive_SVGRenderedFromForcedCoderPrivateCopy`).
- N6 OPEN: the ImageMagick policy directory check compares bytes but a symlink at policy.xml pointing at a file with identical bytes is accepted (the target could be swapped afterwards). The directory is private (0700, created by MkdirTemp), so only the same uid can plant it; Lstat + regular-file check would close it.
- Census limits (UNCONFIRMED): the re-serialised SVG was exercised against Inkscape on this host only; other delegates (rsvg, MSVG) were not run. Entity references other than the five predefined ones are emitted as literal text (escaped), never expanded.

## Docs sync C4 (2026-11-03) - remaining documentation gaps
Found by `scripts/docs/check_doc_coverage.py` (guide `docs/scripts/check_doc_coverage.md`, evidence `docs/qa/docs_sync_c4.md`). Status: OPEN until a commit on `main` closes the item.
- D1: 90 top-level scripts have no `docs/scripts/<name>.md` guide (constitution 11.4.18). They are listed with a reason in `scripts/docs/doc_coverage_exempt.tsv`; write each guide from the script's own behaviour (do not infer) and delete its line. Paths: `scripts/api-down.sh` `scripts/api-up.sh` `scripts/audit-lazy-init.sh` `scripts/audit-semaphores.sh` `scripts/build-all-releases.sh` `scripts/build-test-release.sh` `scripts/ci-local.sh` `scripts/ci-pipeline.sh` `scripts/complete-remaining-tasks.sh` `scripts/deploy-vision-hosts.sh` `scripts/deploy.sh` `scripts/detect-landmines.sh` `scripts/devconnect.sh` `scripts/distributed-boot.sh` `scripts/full-distribute.sh` `scripts/gosec-scan.sh` `scripts/helixqa-orchestrator.sh` `scripts/install-security-tools.sh` `scripts/install-tauri-deps.sh` `scripts/install.sh` `scripts/install_git_hooks.sh` `scripts/local-ci.sh` `scripts/memory-leak-check.sh` `scripts/meta_test_constitution_inheritance.sh` `scripts/mutation_sqlcipher_encryption.sh` `scripts/nancy-scan.sh` `scripts/normalize_submodule_pointers.sh` `scripts/performance-test.sh` `scripts/postgres-down.sh` `scripts/postgres-up.sh` `scripts/pre-flight-check.sh` `scripts/prepare-build-host.sh` `scripts/push_all_submodules.sh` `scripts/python-min-version` `scripts/quick-go-test.sh` `scripts/quick-security-scan.sh` `scripts/quick-start-phase0.sh` `scripts/release-build.sh` `scripts/run-all-platform-tests.sh` `scripts/run-all-security-scans.sh` `scripts/run-all-tests-comprehensive.sh` `scripts/run-all-tests.sh` `scripts/run-android-tests.sh` `scripts/run-api-validation.sh` `scripts/run-desktop-tests.sh` `scripts/run-full-qa.sh` `scripts/run-go-tests.sh` `scripts/run-helixqa-all.sh` `scripts/run-helixqa-android.sh` `scripts/run-helixqa-api.sh` `scripts/run-helixqa-desktop.sh` `scripts/run-helixqa-tests.sh` `scripts/run-helixqa-web.sh` `scripts/run-helixqa.sh` `scripts/run-module-challenges.sh` `scripts/run-race-detector.sh` `scripts/run-security-scan.sh` `scripts/run-sonarqube-scan.sh` `scripts/run-stress-tests.sh` `scripts/safe_submodule_init.sh` `scripts/scan-gosec.sh` `scripts/scan-nancy.sh` `scripts/scan-trivy.sh` `scripts/security-gates.sh` `scripts/security-scan-all.sh` `scripts/security-scan-comprehensive.sh` `scripts/security-scan-full.sh` `scripts/security-scan.sh` `scripts/security-test.sh` `scripts/services-down.sh` `scripts/services-status.sh` `scripts/services-up.sh` `scripts/setup-android-testing.sh` `scripts/setup-android.sh` `scripts/setup-desktop-testing.sh` `scripts/setup-freemium-tokens.sh` `scripts/setup-go-testing.sh` `scripts/setup-implementation.sh` `scripts/setup-security-scanning.sh` `scripts/setup-submodule.sh` `scripts/snyk-scan.sh` `scripts/sonarqube-scan.sh` `scripts/track-progress.sh` `scripts/validate-coverage.sh` `scripts/verify-freemium-setup.sh` `scripts/verify-security-infra.sh` `scripts/verify_constitution_inheritance.sh` `scripts/web-down.sh` `scripts/web-up.sh` `scripts/wire-submodules.sh`.
- D2: `scripts/check_api_route_vs_spec.py` (and its `scripts/lib/gin_routes_ast.go`) does not follow `Register*Routes` helpers (`catalog-api/handlers/storage_routes.go`, `catalog-api/handlers/alias_routes.go`), so it reports 21 spec operations as "IN SPEC, NOT IN CODE" although they are registered (`catalog-api/main.go:1347,1384,1702`). Teach the extractor to expand helper calls, then the gate can run clean; `check_doc_coverage.py routes` already does.
- D3: 13 env names are exempt, not documented: `DOMAIN`, `PASSWORD`, `TYPE`, `USERNAME` (suffixes of `CATALOGIZER_IDENTITY_<N>_*`, documented as that pattern) and the test-helper names `FTP_TEST_*`, `TI_*`, `WEBDAV_TEST_*` (`catalog-api/tests/infra_helper.go`).
- D4: go-fitz (AGPL) is still imported (`catalog-api/handlers/pdf_pages_handler.go:19`, `catalog-api/go.mod:84`); the code removal is not part of this docs change. The docs now state it as pending (`docs/UPGRADE_NOTES.md`).
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
- R4-1 (test-instrumentation, minor): reviewer mutation dropping the username comparison in `getWebDAVClient` survives; add a username-rotation test whose fixture server also checks the username. Also untested: end-to-end `UpdateEndpoint` concurrent with `StartSync`.

## Authz round 2 (fix-authz-round2): decisions applied and what remains
Applied (operator decisions): `/metrics` and `/debug/pprof/*` need an administrator (pprof also still needs `HELIX_PPROF_ENABLED=true`; scrapers must send an admin bearer token); the outbound triggers `POST /entities/enrich`, `POST /entities/:id/metadata/refresh`, `POST /assets/request`, `GET /subtitles/search`, `POST /subtitles/download`, `POST /subtitles/translate` are admin-only; the subtitle-download and external-cover fetches now use `internal/netguard` (dial-time check, pinned address, redirect hops re-checked); `GET /scans*` stays auth-only; `POST /admin/image-quality/revalidate` and `POST /admin/covers/generate-thumbnails` keep their per-route `RequireAdmin`. Subtitle upload is bounded (2 MiB file, 3 MiB request, 413). Route table: `docs/ROUTE_AUTHZ_TABLE.md`.

Open (not fixed here):
- AZ2-1 (defence in depth): fixed-host provider clients (TMDB in `media_entity_handler.go`, fanart.tv, IGDB, Cover Art Archive, `cover_art_service.go`, lyrics, translation, recognition providers) still use `GuardProviderURL` + a plain `http.Client` (no dial-time pinning, redirects followed). Their URLs are built from fixed hosts, so an attacker cannot choose the host today; move them to `netguard.Client` when a provider takes a URL from data. The TMDB client may route through the operator's SOCKS5/HTTP proxy, which `netguard.Transport()` ignores by design; that needs a decision.
- AZ2-2 (policy): every authenticated (including self-registered) user can still read system-wide read-only aggregates: `GET /stats/*`, `/analytics/stats`, `/analytics/duplicates`, `/entities/stats`, `/challenges` and `/challenges/results`, and can attach a subtitle to ANY media item (`POST /subtitles/upload`, no ownership). Operator to decide whether these need a permission.
- AZ2-3: `internal/monitoring.StartPrometheusServer` opens an UNAUTHENTICATED `/metrics` on its own port. It is not called by `main` today (grep over non-test code); anyone wiring it must put the admin gate in front or bind it to loopback.
- AZ2-4: public by design and unchanged: `/health`, `/api/v1/health`, `/health/deep`, `/discovery` (host, port, instance id for LAN discovery), `/ws` (token in query), `/api/v1/image-proxy` (https + allow-listed hosts), `/api/v1/assets/:id`, `/api/v1/cover/*`, `/auth/login|register|refresh|logout|status|init-status`. `/auth/logout` is public: confirm it only revokes the presented token.
- AZ2-5: `pprof` `profile`/`trace` hold an admin request for the sampling time (`?seconds=`) and are not rate-limited beyond the global chain.
- AZ2-6: the route table is produced by a control-needled static extractor over `main.go` plus a live probe (`route_enumeration_probe_test.go`, env-gated); a test that fails when a NEW route is registered without a row in a checked-in table does not exist yet. Routes mounted from other files (`RegisterStorageRoutes`, `RegisterScanRoutes`, `RegisterAliasRoutes`) are added by hand to the extractor input; a route added in a fourth place would be missed.
- AZ2-7: `POST /assets/request` still lets an admin pass `source_hint` as a server file path to the asset chain (`LocalScanResolver` confines it to the read roots); not changed here.
- N2 and N4 are closed on main by the smart-collections merge (e1695dfd: ownership migration 23, playlist 404).
### Smart collections round-4 review follow-ups (GO-with-follow-ups)
- SC10 (test-instrumentation, minor): reviewer mutation dropping the viewer uid from `countKey` (collection_handler.go ~193) survives; add a two-viewer test (different favorites, same smart collection within the 30 s TTL, different `total_items`).
- SC11 (process-doc, nit): UPGRADE_NOTES says an admin sets `owner_user_id` for a legacy collection, but `UpdateCollection` does not accept it; say it needs direct SQL on the SQLCipher DB or add an API.
- N2 is closed: `TestRoutePerms_RealBinary_CollectionOwnership` ran GREEN against the real binary and its skip gate is removed in the landing merge.
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

- AZ2-R1 (important, source-defect, pre-existing): fanart_resolver.go, igdb_resolver.go, llm_image_resolver.go check a URL once, then fetch with a plain http.Client that follows redirects and re-resolves DNS (SSRF via redirect/rebinding). Switch to netguard.StrictClient with tests (redirect to loopback/metadata, rebinding).
- AZ2-R2 (minor, test): scrape token on /debug/pprof/* must be 401 in the RealBinary suite (mutant m2b survived).
- AZ2-R3 (minor, test): strict mode with IPv4-mapped IPv6 (::ffff:127.0.0.2, ::ffff:10.0.0.1) and allow-list env set (mutant m7 survived).
- AZ2-R4 (minor, test): Refresh-button page wiring test with isAdmin false; consider defaulting canRefresh to false.
- AZ2-R5 (minor, deployment): Prometheus token-file mount breaks container start when the file is missing; document creation or make the mount optional.
- AZ2-R6 (nit): deployment/docker-compose.yml monitoring mount path, otel collector not started with METRICS_SCRAPE_TOKEN by any compose file, Gate column for /metrics should mention the token.
