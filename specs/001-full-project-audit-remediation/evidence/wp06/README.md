# WP-06 host-side evidence (T060, T061, T062, T063): register extension DDL, apply script, mutation runner, register gate

Identity: catalogizer / 001 / WP-06 | rev 3 (WF3 fix round 4 appended at the end; rev 2 text above is kept as history and is superseded where the round 4 section says so) | 2026-10-06 | draft, UNREVIEWED (the independent re-review of fix round 3 is owed)

Rev 1 was reviewed by WF2 (`WF2-REVIEW-wp06-wp08.md`: NO-GO, 1 blocking B-1, 9 important I-1..I-9, 11 minor). Rev 2 is fix round 3. Everything below was run on the host (host sqlite3, the committed engine binary); the container image digest is UNCONFIRMED (RUNP/IMG-TESTUTIL, T007/T008, do not exist).

## Task acceptance: what is and is not met

| Task | State | Remaining |
|---|---|---|
| T060 test | the three register tests pass 3 of 3 on the host (`GREEN-*-run{1,2,3}.txt`) | image digest in every WP-06 transcript (UNCONFIRMED, host run) |
| T061 DDL + apply script | DDL is now v5 (see "DDL v5"), the trigger-name sets are equal (`triggers.txt`, 43 each) | docs/04 section 5 still shows v4: revision owed, see "docs/04 drift (I-9)" |
| T062 paired mutations | machine-written, consistent (`mutations-trigger-clause.tsv`, `mutations-check.tsv` and their `.summary` files); the reviewer's survivors are killed | "results inserted as `reg_test_runs` rows with `polarity='MUTATION'`" is NOT DONE (needs the live register, T069; OWED). Equivalent mutants are a reviewed list, not hand-edited rows. |
| T063 gate | gate + test + gate mutations (`gate-mutations.tsv`) | gate run at the container path; `RUNP` (T007) |

## Findings of the WF2 review and what was done

| Finding | Resolution | Proof |
|---|---|---|
| **B-1** trigger mutants killed only by the count; custody clauses without behavioural coverage | New `tests/test_register_behaviour.sh`: NO count assertion; every one of the trigger names is aimed by a behavioural assertion registered through `tg()` and a final T-COVER check fails when a trigger has none; a scenario per custody clause (sections C, R, L: 40 scenarios incl. the reviewer's R1 R5 R7 R8 R9 R10). `mutate_ddl.py` gained 36 clause-level mutants (custody views and trigger bodies; the reviewer's R1..R10 are the first ten); the runner refuses a clause mutant whose text is not found exactly once. | `mutations-trigger-clause.tsv` (+ `.summary`); `mutations-count-only-informational.tsv` shows the OLD test (`test_register_ext.sh`) killing trigger mutants by the count alone: `COUNT_ONLY` rows. |
| **I-1** gate fail-open on objects outside the globs | `gate.sh` compares EVERY schema object of the database with the reference (name, type, table; an unexpected or missing object FAILs whatever its name), every extension object's and every engine trigger's and view's definition, and the seed rows incl. `reg_meta`. New view `v_reopen_unlogged` (engine `item_history` Reopened events the register log lacks) is a `view_empty` check. | gate fixtures B20-B26d, B29, X9.0-X9.3 (the reviewer's X9 attack end to end on a legacy-import item) |
| **I-2** raw DELETE of an item | new view `v_deleted_items` (ids with status-log rows but no `items` row), a `view_empty` check | B30 |
| **I-3** evidence re-hash absent | IMPLEMENTED in `gate.sh` (`step_evidence_rehash`): every `reg_evidence` file must exist and match its recorded sha256 and size; relative paths resolve under the tree root; URL-shaped paths are counted as skipped and printed, never claimed verified; a tab/newline path fails closed. | E1-E7 |
| **I-4** `reg_findings` mutable | triggers `reg_findings_no_delete` and `reg_findings_observation_no_update` (atm_id, run_id, component, location, category, severity, detector, fingerprint, evidence_id). A correction is a new finding plus a recurrence link. | T4e-T4l |
| **I-5** ledger ratchet gameable | see `../wp08/README.md` | |
| **I-6** T087 presented as done | see `../wp08/README.md` (status table with the criteria that remain) | |
| **I-7** mutation evidence hand-edited / inconsistent / runner red | the runner writes `<out>` and `<out>.summary` itself (`mutants=N killed=K equivalent_reviewed=E survived_unreviewed=U`, K+E+U=N asserted); equivalent mutants are a reviewed list `scripts/register/equivalent_mutants.tsv` with reasons (a stale row fails the runner); the identity header carries the sha256 of the runner, mutator, DDL, every test and the list. Exit 0 only when U=0. | the `.summary` files |
| **I-8** gate: table and index weakening not fixtured | B24 (table rebuilt without the author != reviewer CHECK), B25/B26b (dropped indexes), B26c (redefined engine view), B26d (index redefined without its WHERE); the reviewer's G1 and G2 are mutants of `mutate_gate.sh` (`defs_only_triggers_and_views`, `defs_ignore_indexes`) and are killed | `gate-mutations.tsv` |
| **I-9** docs/04 drift | NOT edited here. Recorded below and as `owed-docs04-section5-v5.diff` | |
| m-1 | an option without a value exits 2 (gate, apply, runner, ratchet); measured RED: the old `apply_ext.sh --db` hung (rc 143 after the manual kill, `RED-old-ext.txt`) | G7, F6 |
| m-2 | the online backup is taken from a read-only connection; a WAL database with an uncheckpointed `-wal` is left byte-identical | G8, G8b; mutant `backup_not_readonly` |
| m-3 | every message is flattened (no database text can start a line) and database-sourced names pass `sane()`; the pair is killed together (`output_both_defences_removed`), each alone is a reviewed equivalent (`equivalent_gate_mutants.tsv`) | S1, S2 |
| m-4 | `meta.schema_version` must equal `reg_meta.engine_schema_required` (checked BEFORE the engine validate, which migrates the copy); `reg_meta` rows are part of the seed comparison | B27, B28 |
| m-5 | F3 runs with `env -u REGISTER_GO_LIVE`; F4 compares existence and checksum before and after, so it stays valid after T069 | F3, F4 |
| m-6 | real-register guard also refuses a `file:` URI, a symlink and a hardlink (`-ef`); `PRAGMA foreign_keys=ON` is set before `BEGIN`; error file from `mktemp`; a DDL path with a quote, backslash or newline is refused, one with spaces works | F5-F11 |
| m-7 | NOT DONE: companions for `apply_ext.sh`, `gate.sh`, `mutate_ddl.py`, `run_mutations.sh`, the ratchet in `docs/scripts/` are outside this fix round's scope. OWED. | |
| m-8 | `lib.sh` identity header adds the sha256 of the DDL, apply script, gate and the test; the runners print their own | headers |
| m-9 | see wp08 | |
| m-10 | G4 asserts exit 2 and a usage message, G5 exit 1 and the absence message | G4, G5 |
| m-11 | the runner passes mutant paths NUL-delimited as an argument (`bash -c 'one "$1"' _`), not interpolated into code; its identity line carries the test sha256 | |

## DDL v5 (changes relative to the v4 text of docs/04 section 5, which is the pre-fix file byte for byte, sha256 e798b2f4...)

* `ext_schema_version` 4 to 5 (re-apply bumps it: `ON CONFLICT DO UPDATE`).
* triggers 41 to 43: `reg_findings_no_delete`, `reg_findings_observation_no_update`.
* views 28 to 30: `v_deleted_items`, `v_reopen_unlogged`; `reg_gate_checks` gains both views and both triggers.
* tables 25, unchanged.
* The patch is `owed-docs04-section5-v5.diff`.

## docs/04 drift (I-9): recorded, NOT edited here; the revision entry that is OWED

docs/04 is the binding document and another agent is editing it, so this round did not touch it. The owed revision entry (to be added by its owner, with the section 5 block replaced by the v5 DDL, `owed-docs04-section5-v5.diff`):

1. section 5 DDL v4 to v5 (above); section 4 counts "Views (28)" and "Triggers (41)" to 30 and 43; the `ext_schema_version=4` statements, T061 text ("`ext_schema_version=4`", "41 expected").
2. the header sentence "the DDL of section 5 unchanged ... `c179f65c...`": that hash is not reproduced by any extraction of the HEAD or working-tree document (five variants tried by the reviewer; UNCONFIRMED what it hashed). The `CAT-` rename of the `reg_ids.atm_id` expression (ODG-11) is a DDL change that the revision 25 header records for the design text; the DDL sha256 is now `701171a4...` (v5) and the pre-fix `e798b2f4...`.
3. line 969 (constraint map, gate row): "compares `sqlite_master` text of every `reg_*`/`v_*`/`trg_*`/`idx_*`/`uq_*` object" becomes: every schema object (names, types, tables), the definitions of the extension objects and of the engine's triggers and views, and the seed rows including `reg_meta`; plus the new steps (engine schema version, evidence re-hash).
4. section 12.3 step 2 still shows the extracted-script gate (view list read from the database under test, `$v` interpolated unquoted, `$WI validate` on the real DB); `scripts/register/gate.sh` replaced it: it reads the view list from a trusted reference, regex-checks and quotes names, and runs everything on a read-only copy. The text should point to the script.
5. section 14 line 1846 "NOT EXECUTED: apply-script engine-version check, the proposed `scripts/register/gate.sh` at its repository path ...": both now executed (`GREEN-ext-run*.txt` C1-C4, `GREEN-gate-run*.txt`, `gate.txt`); the item "evidence-file re-hash" is now implemented and fixtured (E1-E7); "limitation 6" (a deleted row is reported by `v_ids_without_item`) must say `v_deleted_items`.
6. section 5 limitation 1 (identity update) and 3(a) (the re-hash) need the v5 wording.

## Owed items (tracked here until the register is live, T069; names are stable references for this fix round)

| Id | What | Why not done now |
|---|---|---|
| OWED-WP06-1 | docs/04 revision entry and section 5 replacement (above) | binding document, another agent edits it |
| OWED-WP06-2 | tasks.md: T061 text (`ext_schema_version=4`, 41 triggers), T062 (the `reg_test_runs` MUTATION rows), T063 (gate steps added) | instruction: no tasks.md edit |
| OWED-WP06-3 | insert the mutation results as `reg_test_runs` rows with `polarity='MUTATION'` | register not live |
| OWED-WP06-4 | `docs/scripts/*.md` companions (WF2 m-7) | outside this round's file scope |
| OWED-WP06-5 | container-image leg: rerun every WP-06 transcript through `RUNP IMG-TESTUTIL` and record the digest | T007/T008 absent |
| OWED-WP06-6 | independent confirmation of the authored equivalents M16, M36 (`equivalent_mutants.tsv`) and of `output_not_flattened`, `missing_objects_not_sanitised` (`equivalent_gate_mutants.tsv`) | needs the re-review |
| OWED-WP06-7 | a decision for `reg_findings`: whether a controlled re-point of `atm_id` (merge of duplicate items) is wanted; today it is refused, the way is a new finding plus a recurrence link | owner decision |

## UNCONFIRMED (stated, not claimed)

1. The gate compares the SQL text of the engine's triggers and views with a fresh engine database built by the same binary. Against a real register whose engine schema was MIGRATED from an older engine version the text could differ and the gate would FAIL on a clean database. Not measurable now (`docs/workable_items.db` does not exist). If it happens at T069 the comparison is narrowed to names. WF6-4 widening (owner decision, NOT decided here): since round 5 `sqlite_sequence` and every `sqlite_autoindex_*` object are also compared by name and type against the fresh reference (they are no longer hidden by prefix). A register whose engine schema was migrated by table rebuilds (different autoindex numbering, a leftover `sqlite_sequence` row set or a `sqlite_sequence` that the reference does not hold) could therefore differ from a fresh reference and the gate would FAIL on a clean database. Not measurable now for the same reason. If it happens at T069 the owner decides between narrowing those objects to a name-prefix class again (with the exact-definition pin the stat tables have) or regenerating the register on a fresh engine schema.
2. `v_reopen_unlogged` counts engine Reopened events created at or after the id's first status-log row (second resolution, `created_at` against `changed_at`). Reopen history that pre-dates the extension is not counted. A reopen in the same second as the first log row is a theoretical false positive.
3. Evidence rows of the real register may cite files outside the tree root (absolute paths are read as given) or non-file receipts that are not URL-shaped; the re-hash would FAIL them. Unmeasurable before T069.
4. The WAL result (G8) is measured on a fixture with an abandoned writer; a long-lived WAL register under concurrent writers is not measured.
5. Equivalent mutants M16, M36 and the gate pair are the author's arguments, not tests.
6. The host-side runs are not the container runs; sqlite 3.50.6 only.
7. The scripts, tests and evidence are UNTRACKED; the git head in every header does not identify them, the sha256 values do.

## Mutation results (machine-written `.summary` files)

* trigger+clause, tests behaviour+gate: mutants=79 killed=75 equivalent_reviewed=4 survived_unreviewed=0
* CHECK, test ext (counts excluded): mutants=73 killed=72 equivalent_reviewed=1 survived_unreviewed=0
* gate.sh, test_gate: mutants=32 killed=30 equivalent_reviewed=2 survived_unreviewed=0
* informational, OLD ext test, triggers, counts excluded: mutants=43 killed=22, 21 COUNT_ONLY (the B-1 finding reproduced: 19 of the reviewer plus the 2 new triggers).
* Not mutated: the runner itself (only fixture-tested, `test_run_mutations.sh`), `apply_ext.sh`; UNCONFIRMED adequacy.

## Files

* GREEN: `GREEN-behaviour-runs1-3.txt` (144 checks x3), `GREEN-ext-runs1-3.txt` (176 x3), `GREEN-gate-runs1-3.txt` (62 x3), `GREEN-run-mutations-runs1-3.txt` (24 x3, the runner's own test)
* RED (the rev 2 tests run against the rev 1 scripts and DDL, reconstructed byte for byte, DDL sha256 e798b2f4...): `RED-old-behaviour.txt` (8 failing), `RED-old-ext.txt` (14), `RED-old-gate.txt` (28), plus `RED-runner-stale-check.txt` (the runner's own RED). The rev 1 DDL was not the RED subject for the 41 pre-existing triggers: their RED is the mutation kill (`mutations-trigger-clause.tsv`).
* mutation: `mutations-trigger-clause.tsv`, `mutations-check.tsv`, `gate-mutations.tsv`, `mutations-count-only-informational.tsv`, each with a `.summary`
* `triggers.txt` (T061 set comparison), `gate.txt` (T063 `GATE OK` on the T061 scratch database), `owed-docs04-section5-v5.diff`
* SUPERSEDED rev 1 files (kept as history, do not cite): `mutations.tsv` (hand-edited row), `mutations-summary.txt` (112/2, inconsistent with its table), `ext-green.txt`, `gate-green.txt`, `gate-mutation*.txt`, `ext-red.txt`, `gate-red.txt`, `gate-untrusted-red.txt`, `gate.txt` before the rewrite.


## Round 4: WF3 fix round (review `WF3-REVIEW-wp06-wp08.md`, NO-GO 0 blocking / 5 important / 6 minor; owner scope: every real defect)

Everything here is UNTRACKED and UNREVIEWED; the independent re-review of round 4 is owed. Every evidence file below starts with a header naming the sha256 of the scripts and tests it ran (`gate.sh f2b94af5...`, `apply_ext.sh 73c975fb...`, `register_ext.sql 701171a4...` UNCHANGED, `mutate_ddl.py 41972d11...`, `test_gate.sh 66f979f2...`, `test_register_behaviour.sh 77c06dc0...`, `test_register_ext.sh 65b55e9c...`, `mutate_gate.sh 6d500dd8...`). Host run: host sqlite3 3.50.6, committed engine binary, uid 1000.

| Finding | Change | Test-first evidence (RED), then GREEN, then mutants |
|---|---|---|
| **WF3-I1** object filter `LIKE 'sqlite_%'` hid `sqliteX...` | `gate.sh`: `name NOT GLOB 'sqlite_*'` (case-sensitive, `_` literal) | RED `RED-rev3-gate.txt` (B35 sqliteX trigger, B36 sqlite1 view, B37 sqlitezz table all printed GATE OK on rev 2). GREEN `GREEN-gate-rev3-runs1-3.txt` (75 checks x3). Mutants `object_filter_like_wildcard`, `object_filter_prefix_widened` (the reviewer's W4) killed |
| **WF3-I2** URL-shaped evidence path skipped the re-hash and printed GATE OK | A URL-shaped path is legal ONLY for kind `tracker_receipt`; on any other kind `FAIL evidence N: kind K must cite a file`. A run with skipped paths ends `GATE PASS-PARTIAL url_skipped=N (refused...)` exit 3, or exit 0 with the new flag `--allow-url-evidence` (still `PASS-PARTIAL`, never a bare `GATE OK`). Design basis: docs/04 section 7.1(a) and K-11 (the re-hash is the compensating control for producer = verifier, so a run that skipped paths is partial); the rule keeps the one legitimate non-file kind and closes the opt-out. The rev 2 fixture E6 ("a URL path is skipped and the gate passes") is REPLACED, a deliberate rule change, by E6 / E6a / E6b / E6c / E8 | RED as above (E6, E6a, E6b, E6c). Mutants `url_skip_any_colon` (W5, killed by E8), `url_any_kind_skipped`, `partial_exits_zero_no_flag`, `partial_ignored_gate_ok`, `url_skipped_not_counted`, `rehash_url_counted_verified` killed |
| **WF3-I3** ledger F2/F3 gaming (3 routes) | `project_gate_ledger_ratchet.sh` rev 3: F2b (every non-family name in the documents must be registered in `--prev-names`), an absent `--prev-names` file FAILs, F1d (a ledger row whose name is in no document FAILs, so an orphan DEFERRED row cannot keep freed slack and its reference is validated) | RED `../wp08/T086-RED-rev3.txt` (4 failing checks on the reconstructed rev 2 ratchet). GREEN `../wp08/T086-GREENx3-rev3.txt` (55 x3). Mutants `I3_F2b_unregistered_name_ok`, `I3_prev_names_absent_ok`, `I3_F1d_orphan_row_ok`, `I3_F2b_applied_to_family_only_names` killed |
| **WF3-I4** no mutation/false-positive coverage for the round-3 views and the I-4 column list | Golden-good gate fixtures GF1 (legitimately reopened item), GF2 (minted id without an item), GF3 (brownfield reopen older than the first log row); behavioural checks T4m (category), T4n (evidence_id), T4o (location_line); clause mutants W1, W2, W3, W8, W9, W9b, W10 in `mutate_ddl.py` | RED = the mutants themselves (the DDL was already correct, these tests pin it): `mutations-trigger-clause-rev3.tsv`: W1 killed by GF1, W2 by GF2, W3 by GF3, W8 by T4m, W9 by T4n, W9b by T4o. W10 (component_id) is reviewed-equivalent (the `unit_alias GLOB 'F-'||component_id||'-*'` CHECK refuses the edit anyway; measured by the reviewer, row in `equivalent_mutants.tsv`) |
| **WF3-I5** docs/04 not synced | NOT edited (the owner edits docs/04). The exact patch is `owed-docs04-sync.diff` (see "Owed patch" below) | `patch --dry-run` and a real apply to a copy reproduce the patched text byte for byte |
| m1 `item_history` DELETE defeats `v_reopen_unlogged` | NOT changed in the DDL (the attack needs a transient DDL writer, docs/04 limitation 9(a)). Claim scoped: X9.3 detects the reopen that a SILENCED log writer hid, it does NOT detect a later DELETE of the engine's `item_history` row. Owed: column-scoped guards on `item_history` `Reopened` rows, and the outside verifier (limitation 9(d)) comparing ledger `REOPEN` entries with register `Reopened` rows | |
| m2 `output_not_flattened` equivalence wrong | Row removed from `equivalent_gate_mutants.tsv`; fixture S3 (an index named `ix<LF>GATE OK` made inconsistent through `writable_schema`, built with python because the sqlite3 shell is defensive) asserts zero bare `GATE OK` lines | mutant `output_not_flattened` killed |
| m3 percent-encoded URI bypassed the apply guard | `apply_ext.sh` refuses EVERY `file:` URI (case-insensitive), exit 4; F10 loop kept, F12 / F12b / F12c / F13 added | RED `RED-rev3-ext-apply.txt` (4 failing: `file:.../re%61lish.db` applied to the protected file, sha256 changed). GREEN `GREEN-ext-rev3-runs1-3.txt` (180 x3) |
| m4 ledger W6, W7 | fixtures `W6`, `W7`, `W7b`; mutants `W6_item_id_regex_unanchored_at_end`, `W7_slash_comment_counts_as_code` | killed, `../wp08/mutations-rev3.tsv` |
| m5 docs/scripts companions, m6 ledger carrier gaps | NOT DONE (outside the file scope; m6 is stated in wp08 Owed 5) | OWED |

### Results (machine-written `.summary` files; every number re-derivable from the tsv beside it)

* trigger+clause, tests behaviour+gate: `mutants=86 killed=81 equivalent_reviewed=5 survived_unreviewed=0` (`mutations-trigger-clause-rev3.tsv`; rev 2 had 79)
* CHECK, test ext, counts excluded: `mutants=73 killed=72 equivalent_reviewed=1 survived_unreviewed=0` (`mutations-check-rev3.tsv`)
* gate.sh, test_gate: `gate mutants=39 killed=38 equivalent_reviewed=1 survived_unreviewed=0` (`gate-mutations-rev3.tsv`; the remaining equivalent is `missing_objects_not_sanitised`, confirmed by the reviewer for newlines)
* GREEN x3 (host): test_gate 75, test_register_behaviour 147, test_register_ext 180, test_run_mutations 24: `GREEN-*-rev3-runs1-3.txt`
* Container leg: see `container-rev3-*.txt` (IMG-TESTUTIL via `scripts/containers/run_pinned.sh`, uid 1000 mapped; works as of this run)

### Owed patch (WF3 I5) and exact description

`owed-docs04-sync.diff` is a unified diff against `docs/04-findings-register-design.md` as it was in the working tree at the time (base sha256 in the file header; the file is modified and uncommitted by its owner, so patch may reject a hunk if the touched lines moved: the hunk text is then the replacement). It does exactly these six things:
1. replaces the section 5 SQL block (v4, sha256 `e798b2f4...`) by the v5 DDL (`701171a4...`, `ext_schema_version=5`);
2. section 4: `ext_schema_version=4` becomes `5`; "Views (28)" becomes 30 and gains `v_deleted_items`, `v_reopen_unlogged`; "Triggers (41)" becomes 43 and gains `reg_findings_no_delete`, `reg_findings_observation_no_update` (with the ten guarded columns); "All 41 are registered" becomes "All 43";
3. limitation 6: the raw `DELETE FROM items` is now reported by `v_deleted_items` (not `v_ids_without_item`, which stays a `view_report` for the mint-but-not-added gap);
4. limitation 9(a) and the section 6 constraint-map row: the gate compares EVERY schema object (all but `sqlite_*`, selected with `NOT GLOB`), not the five name globs;
5. section 12.3 step 2: the injectable inline gate script is replaced by `scripts/register/gate.sh --db $DB` with its step list, the evidence rule (only `tracker_receipt` may be URL-shaped), and the exit codes 0 / 1 / 2 / 3 (`GATE PASS-PARTIAL`);
6. section 12.3 checklist item: `ext_schema_version=5`.
Still owed to the docs/04 owner, not in the diff: a revision entry in the Status cell (the revision number is the owner's), the header sentence "DDL of section 5 unchanged ... `c179f65c...`" (it is a history entry about an older revision, true for its time; the new revision entry must give the v5 hash `701171a4...`), and a v5 executed-evidence record in section 14 (lines recording "views 28 | triggers 41 | ext_schema_version=4" are historic and untouched). tasks.md (WF3 I5): T061 text `ext_schema_version=4` and "41 expected" become 5 / 43; T062 `reg_test_runs` MUTATION rows stay owed (register not live); T086 and T087 paths under `scripts/gates/` become `scripts/ledger/` (or the files are relocated). No tasks.md edit was made.

### Owed / UNCONFIRMED after round 4

* OWED-WP06-1 replaced by `owed-docs04-sync.diff` (apply by the docs/04 owner); OWED-WP06-2 tasks.md amendments; OWED-WP06-3 `reg_test_runs` MUTATION rows (register not live); OWED-WP06-4 `docs/scripts/*.md` companions (out of scope); OWED-WP06-6 only `missing_objects_not_sanitised`, W10, M16, M36 stay as reviewed equivalents; OWED-WP06-7 owner decision on re-pointing a finding's `atm_id`.
* UNCONFIRMED: the container run covers the tests listed in `container-rev3-*.txt` only; the three mutation sweeps are host-side; control bytes in `sane()`/`flat()` rely on the sqlite3 printing ESC as `^[` (measured on host sqlite3 3.50.6; the container sqlite3 3.40.1 passes S1, S2 and S3 in `container-rev3-test_gate.txt`, ESC bytes themselves not separately counted).
* UNCONFIRMED: `--allow-url-evidence` is an owner-facing switch whose default is refusal; whether the real register will hold any `tracker_receipt` row with a URL is not measurable before T069.
* The runner (`run_mutations.sh`) and `apply_ext.sh` are still not themselves mutated (rev 2 statement unchanged).

## WF5 fix round 5 (2026-10-06): WF5-1 reserved-prefix objects hidden from the gate

Reproduced first with host sqlite3 3.50.6 and python's sqlite3 module (non-defensive by default). SQLite refuses a `sqlite_` name case-insensitively ("object name reserved for internal use": lowercase, UPPERCASE and Mixed-case all refused), but ONLY while writable_schema is off. With `PRAGMA writable_schema=ON` a trigger, view, index and table with any case of the prefix is created, persists, and a later plain connection sees it. The old filter `name NOT GLOB 'sqlite_*'` hid every lowercase one of them, whatever its type (UPPERCASE and Mixed names were already visible because GLOB is case-sensitive). What SQLite itself creates: `sqlite_sequence` (AUTOINCREMENT, also in the reference), `sqlite_autoindex_*` (deterministic, also in the reference) and `sqlite_stat1..4` (ANALYZE only, never in the reference).

Fix (`scripts/register/gate.sh`): the comparison filter is now `NOT (type='table' AND name GLOB 'sqlite_stat[1-4]')`; sqlite_sequence and autoindexes are compared like any object. The stat tables are not simply hidden: each present one must carry exactly SQLite's own definition (`step_reference_diff` part 0), else FAIL `unexpected schema object`. **WF6 correction (round 6, 2026-10-06): this claim was FALSE for stat3 and stat4 as shipped in round 5.** The round 5 pins spelled the column lists `nEq,nLt,nDLt`; SQLite writes them in lowercase (`neq,nlt,ndlt`), so a register after ANALYZE on a STAT4 build was refused (WF6-1, a false refusal). See the round 6 section at the end for the corrected pins and the evidence. Evidence: `RED-wf5-1-gate.txt`, `GREEN-wf5-1-gate-runs1-3.txt` (111/0 x3), `gate-mutations-wf5-1.tsv` (43 mutants, 42 killed, 1 reviewed equivalent, 0 survivors; new mutants `object_filter_name_only`, `object_filter_any_type`, `object_filter_prefix_widened`, `object_filter_like_wildcard`, `stat_definition_not_pinned`, `stat_check_removed` all killed). New fixtures: B40.1-B40.16 (lowercase/UPPER/Mixed trigger, view, index, table, plus sqlite_stat1..3 as trigger/view/index, sqlite_stat9, sqlite_sequence_x, sqlite_autoindex_*), B41 (the swallow trigger persists), B42 (forged sqlite_stat1), G9 (ANALYZE, no false refusal). No DDL change, so the DDL mutation run (register_ext.sql, about 8 minutes) was NOT re-run: UNCONFIRMED only in the sense that it was not needed; test_register_behaviour 147/0, test_register_ext 180/0, test_run_mutations 24/0 and the ledger test 55/0 were re-run after the fix.

Owed after round 5 (not fixed here, by instruction):
* OWED-WP06-8 (WF5-2, mutation adequacy): two ledger ratchet guards survive exact-to-substring weakening (`grep -qxF` to `grep -qF` in F1d and F2b). Add two prefix-collision fixtures (orphan row `CM-ALPHA DEFERRED CAT-5` with documents naming only `CM-ALPHA-ONE`; document name `CM-ALPHA` with prev-names holding only `CM-ALPHA-ONE`) and mutants R1, R2 to `scripts/ledger/tests/test_project_gate_ledger_mutations.sh`. The shipped ratchet code is correct.
* OWED-WP06-9 (WF5-3, docs): `owed-docs04-sync.diff` no longer applies to docs/04 (sha256 `bd128a40`, hunk 8 fails) and its limitation 9(a) / constraint-map text still says "all but sqlite_*, selected with NOT GLOB"; regenerate against the current docs/04 with the round 5 filter wording (compare everything except sqlite_stat1..4, which are pinned by exact definition).
* OWED-WP06-10: container leg evidence files do not record the image digest (OWED-WP06-5); the container leg was not re-run after round 5.
* The real ledger run is RED on five names introduced by another stream: an owner decision, not a ratchet defect.

## WF6 fix round 6 (2026-10-06): WF6-1 stat pins case, WF6-2 mutation adequacy, WF6-3 docs, WF6-4 diagnosability, stray directories

Review: `WF6-REVIEW-register-gate.md` (GO-with-fixes; not tracked in this directory, it lives in the session scratchpad). Test-first: the new fixtures were added to `scripts/register/tests/test_gate.sh` and run against the round 5 `gate.sh` first: `RED-wf6-gate.txt` (3 FAIL of 124: G10.2 exact stat3 refused, G10.3 exact stat4 refused, B43.5 refusal tail cut at 60 characters; G10.1 exact stat2 and the forged-stat2/3/4 fixtures already passed against the old gate, they are guards for the mutants below). Then `gate.sh` was fixed and the suite run three times: `GREEN-wf6-gate-runs1-3.txt`.

* **WF6-1 (important).** SQLite's own text, read from the vendored 3.53.3 source (`mattn/go-sqlite3 v1.14.48`, `sqlite3-binding.c:123799/123801`): `{ "sqlite_stat1", "tbl,idx,stat" }`, `{ "sqlite_stat4", "tbl,idx,neq,nlt,ndlt,sample" }`. The gate pins now read `sqlite_stat3(tbl,idx,neq,nlt,ndlt,sample)` and `sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample)` (lowercase). `sqlite_stat2` stays the documented text `(tbl,idx,sampleno,sample)`. UNCONFIRMED: the stat2/stat3 texts that pre-3.30 builds wrote cannot be read from the vendored source (3.53.3 writes neither: its table has `{ "sqlite_stat3", 0 }`); stat3 follows stat4's lowercase list; no pre-3.30 SQLite is available offline. The reviewer's capture of a real ANALYZE on a STAT4 build (`probe/stat4_run.out`: `CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample)`) is the independent confirmation for stat4; the golden-good fixtures G10.1-G10.3 are author-written (writable_schema with SQLite's exact text).
* **WF6-2.** Mutants added to `mutate_gate.sh`: `stat_all_narrowed_to_stat1` (reviewer R1), `stat_pins_234_removed` (reviewer R2), `stat3_pin_mixed_case`, `stat4_pin_mixed_case` (the round 5 defect restored). Fixtures: G10.1-G10.3 (exact definitions accepted), B43.1-B43.4 (forged stat2/stat3/stat4, and a stat4 with a trailing extra column refused). Sweep: `gate-mutations-wf6.tsv` (47 mutants, 46 killed, 1 reviewed equivalent, 0 survivors; the four new mutants killed). Suites after the fix: test_gate 124/0 x3, test_register_ext 184/0 x3, test_register_behaviour 147/0, test_run_mutations 24/0 (`GREEN-wf6-*.txt`).
* **WF6-3.** (a) the `gate.sh` comment named a non-existent `step_stat_tables`; it now names part 0 of `step_reference_diff`. (b) B41 now proves the swallow trigger is effective: control B41.0 (the illegal status-log insert is refused by the register guard, rc 19) and B41.1 (with the trigger the same insert is swallowed: rc 0, row count unchanged). (c) the RED transcript `RED-wf5-1-gate.txt` is a transcript and is NOT edited; its sentence "B40.9-11 in the final numbering" is wrong: the stat-named trigger, view and index fixtures are B40.10-B40.12 (B40.9 is `Sqlite_Hidden_T`), and no RED was captured for those three fixtures (as that file already says). Correction note: `RED-wf5-1-gate.CORRECTION-wf6.txt`. (d) the round 5 sentence about exact definitions is corrected in place above.
* **WF6-4.** (1) the stat-mismatch FAIL now prints the differing definition untruncated up to 600 characters (`sane_full`), pinned by B43.5; (2) the widened UNCONFIRMED 1 above (owner decision, not decided).
* **OWED-WP06-8 closed (ledger).** Fixtures WF6-R1/R2/R3 in `scripts/ledger/tests/test_project_gate_ledger.sh` and mutants `R1_F1d_substring_match`, `R2_F2b_substring_match` in `test_project_gate_ledger_mutations.sh`: see `../wp08/ledger-mutations-wf6.tsv` (34 mutants, 34 caught).
* **Stray directories `file:` and `FILE:` at the repository root.** Cause: a `file:` argument is a relative path for any program that does not treat it as a URI; the `test_register_ext.sh` F5/F10/F12/F13 calls ran with cwd = the repository root, so a run in which the `file:` guard of `apply_ext.sh` was absent (UNCONFIRMED which run: the directories held `regtest.8hniEE` and a WF3 scratch path, created 07:23 and 01:55) bootstrapped databases under `./file:/...` and `./FILE:/...` (`RED-wf6-stray-dirs.txt` reproduces the mechanism with a guard-less copy). Fix: those calls now run through `inCW` (cwd = a scratch directory); new checks F14 (control needle: a guard-less copy does create the stray, under the scratch cwd), F15 (the real guard refuses and creates nothing), F16 (no `file:`/`FILE:` in the repository root). The two existing ignored directories were verified untracked, held only test-created databases under `/tmp/regtest.*` and scratch paths, and were removed.

