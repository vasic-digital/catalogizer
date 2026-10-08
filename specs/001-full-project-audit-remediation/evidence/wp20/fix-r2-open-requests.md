# WP-20 / WP-21 round 2 - decisions and requests that are NOT this fixer's to make

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Author | the single fixer (Sonnet) of the WF23 fix pass; nothing here is an owner answer |
| Status | OPEN: every item waits for the person named; none is closed by this pass |

## D1 - OD-17 (T164): the owner question, verbatim for relay

> **OD-17 (T164).** The plan states that a YAML parser loads every `docs/issues/*.md` front-matter block and replaces the doc04 naive scanner (research.md OD-17 default; doc04 14.4 D-7). Measured 2026-10-08 on the 1,778-ticket HEAD corpus (instrument: `yaml.safe_load` on the block between the first two `---` lines, control: the same call loads 1,379 blocks and the fixture forms): it REFUSES 399 blocks, all with `ScannerError: mapping values are not allowed here` (an unquoted colon inside a plain value, for example `resolution: a: b`); 1,379 parse. Which of these do you want?
> **(A)** keep the tolerant line reader as the reader of record (it reads the 1,379 as YAML does and a colon inside a value as data) and report the strict count as a standing fact;
> **(B)** require strict YAML and pre-normalise the 399 blocks by a recorded, reviewed, deterministic rewrite that quotes the value after the first colon (the source tickets are not edited; the rewrite is a tool step whose output is evidence);
> **(C)** edit the 399 tickets so that they are valid YAML (399 tracked source files change, every changed ticket's sha256 changes);
> **(D)** both (A) and the strict count gated at 399, so that a new invalid block is a failure.
> This pass ships (A) plus the strict count reported (a hybrid of A and the fact of D without the gate): the tolerant reader, the strict checker, a test that asserts both, and `--expect-strict-invalid N`. T164 stays open until you answer.

## Requests to other owners (exact text)

- **R1 - owner of `scripts/register/locked.sh` (WP-06, another stream).** T165 specifies `locked.sh import-sql <sql> <sha256>` (two argv elements, a `<file>.sql.sha256` file, refusal `import_args_invalid` for one or three arguments) and a test `scripts/register/tests/test_locked_import_sql.sh` with fixtures; the committed `locked.sh` takes ONE argument (`[ $# -eq 1 ] || usage`) and reads the sibling `<stem>.sha256` (`stem="${IMPORT%.sql}"; sib="$stem.sha256"`, line 247) i.e. `source_entries.sha256`; the test file is absent. This pass did NOT edit `locked.sh`: the enumerator and the lead scan now write BOTH `<name>.sql.sha256` and `<name>.sha256` (same one bare-hash line), so the committed `locked.sh` finds its sibling. Please either change `import-sql` to the T165 shape or record in tasks.md T165 that the sibling convention is the contract. The end-to-end `locked.sh import-sql` run with its journal fixture is UNCONFIRMED (not made: it starts a container and writes `.audit/` journal files in a tree other streams use).
- **R2 - owner of T161.** `check_freeze_listing.sh` (compares the listing's sha256 with `freeze.json`) does not exist; `lead_scan.py` falls back to a path-SET comparison, which accepts an appended path already present or a reordering. The stand-in `freeze.json` shape (snapshot, manifest, listing, gitlinks, frozen_at, head) is invented by the WP-20 worker; the manifest's own sha256 recorded by T161 is verified by no consumer.
- **R3 - owner of T161.** T165 asks for "one row per remote and named service" and doc03 10 step 2 for "one row per remote and per named service", but the T161 text records no remote list in `freeze.json` and the enumerator may read only the frozen snapshot (no `.git`). This pass requires a `remotes` list in `freeze.json` (`[{"name","url"}]`, url without credentials, from `git remote -v`; `[]` is a stated fact; absent is refused `freeze_remotes_missing`) and takes the four named services (firebase-crashlytics, sonarqube, snyk, trivy) from doc03 5.20 with marker-file detection in the snapshot. Please add the field to T161 or tell the enumerator where else to read the 8 remotes (doc03 5.20: origin, upstream, github, githubvasicdigital, gitlab, gitlabvasicdigital, gitflicvasicdigital, gitversevasicdigital).
- **R4 - owner of `docs/scripts/README.md` (not touched in this pass, finding E6, constitution 11.4.212).** None of `enumerate_sources.md`, `lead_scan.md`, `snapshot_manifest.md`, `register_wp20_tests.md`, `register_gate.md` is linked from any Markdown. Add to the scripts index: `docs/scripts/enumerate_sources.md`, `docs/scripts/lead_scan.md`, `docs/scripts/snapshot_manifest.md`, `docs/scripts/register_wp20_tests.md`, `docs/scripts/register_gate.md`.
- **R5 - owner of `docs/scripts/register_gate.md`.** It cites `mut1/mut.tsv`; the evidence holds `evidence/wp21/mut2/` (the first run had two survivors and was replaced). Edit the path.
- **R6 - owner of the evidence layout.** T165/T164/T167 name `$EV/register/source-class-kinds.json`, `$EV/register/frontmatter-snapshot-mutation.txt`, `$EV/register/lead-scan-scope.md`, `$EV/register/non-problem-mapping.md`, `$EV/register/lead-scan-run.json`; this pass was scoped to `$EV/wp20/` and wrote them there (`source-class-kinds.json`, `fix-r2-frontmatter-snapshot-mutation.txt`, `lead-scan-scope.md`, `non-problem-mapping.md`, `fix-r2-lead-scan-run-HEAD-archive.json`). Move or re-record when the layout is decided.
- **R7 - owner of `scripts/register/tests/test_reverify_gate.sh` (WP-21, outside this pass's edit scope; finding D6).** Mutant GM01 (`[[ $n =~ ^[0-9]+$ ]]` weakened to `[[ $n =~ [0-9] ]]`) survives because the F5 fixture's error text holds no digit. Add a fixture whose failure text contains a digit (for example a fake `sqlite3` printing `error 5: database is locked`) and assert the gate prints `not counted`, never `rows=<text>`. GM05 is equivalent by exit code (the seed diff fails the same rows).
- **R8 - owner of T161 / T176.** The REAL freeze run of `test_real_snapshot.py` (`REAL_FREEZE_JSON`) and `test_enumerate_planted.sh` with `REAL_SNAPSHOT` is owed against the real frozen snapshot; this pass ran both on a `git archive HEAD` snapshot (8,163 files, helix_qa banks only, 24 `export-ignore` paths missing), see `fix-r2-README.md`.

## Findings of the review NOT fixed (and why)

| Finding | Why |
|---|---|
| D1 | needs OD-17 (above) |
| D6 | the test is a WP-21 file outside this pass's edit scope (R7) |
| E4 (the `locked.sh` side) | `locked.sh` is owned by another stream (R1); the enumerator side is fixed |
| E6 | `docs/scripts/README.md` is outside this pass's scope (R4) |
| F1, F2, F6 | T161 stand-in design belongs to the T161 owner (R2, R3); F3, F4 are fixed, F5 (TOCTOU) is documented |
| C5 "written through the recorder" (the file itself) | the recorder (`evrec`) records commands and output digests, it does not author a tool's JSON file; the exact command and the outputs are in the ledger, the counted rows are in the run record after `--finalize` |
