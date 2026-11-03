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
- Legacy SHA-256 password rows have no sunset: they are crackable offline until each user logs in. Add a forced reset or expiry for rows still in that shape, and log counts of remaining legacy rows.
- `UserAuthServiceInterface.HashData` and `AuthServiceAdapter.HashData` are no longer used by production code (dead since the shared hasher); investigate history before removing (section 11.4.124).
- `database/schema_v3_multiuser.sql` seeds a published default admin hash (password `password`). Checked 2026-11-03: no Go code, script, Makefile, compose or Dockerfile references it and `scripts/register/root_dispositions.yaml` records it as not loaded; consider deleting or neutralising the seed row (investigate history first, section 11.4.124).
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
