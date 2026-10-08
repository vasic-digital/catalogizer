# WP-22 fix round 2: every finding of WF23-REVIEW-wp22 mapped to its fix, its tests and its mutants

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Reading | "Fix" names the function or mechanism in `scripts/register/sync_trackers.sh`; test ids are in `test_sync_driver.sh` (old suite, ids A..Z, K, L, W, X, V, U, E, M, R, D, F, G) or `test_sync_fix_r2.sh` (r2 suite: S secret/config, W write-ahead, P process, B breaker, E enabled, R receipt, G gate, O outbound, V revision, X misc, OW owed); mutant names are in `mutate_sync_driver.sh` |
| Not fixed | marked **NOT FIXED** with the reason; nothing is silently skipped |

| Finding | Layer | Fix | Tests | Mutants (killed at the intended check unless stated) |
|---|---|---|---|---|
| A1 secret in adapter command, 10 of 14 forms accepted, persisted into the register | source-defect High | `command_problem` positive grammar (userinfo URL, authorization text, credential-shaped token, credential-named key or flag with a value, short flag with a bare value); command text never persisted: `cmd_ref` = `<exe>#sha256:<16 hex>` | r2: S01-S19 (19 refused forms, canary absent from output), S51-S56 (6 accepted controls), VD1 (command text and canary in no table, batch, journal row or journal input; control finds the hash form); old: Z6 | X07, N01-N06, N12 |
| A2 `env_passthrough` accepts literals | source-defect Medium | `template_problem`: every template must contain `{db}` or `{id}`, no credential shape; REFUSED `config_secret_in_env` | S60-S63 | X13 |
| A3 duplicate YAML keys override silently | source-defect Medium | `unique_loader` (SafeLoader subclass refusing repeated keys) | S7-dup1, S7-dup2, S7-control | N07 |
| A4 whitespace-only credential counts as present | source-defect Low | `envset()` strips | S8 | N08 |
| B1 pushes lost on crash, non-UTF-8, signals; exit code 1 on crash | source-defect High | write-ahead journal `.audit/sync/wal/<run>.jsonl` (intent before, result after every call, fsync), `reconcile_wal` at the next run, bytes decoded with `errors="replace"`, SIGTERM/SIGINT/SIGHUP handlers, exit 5 / 6 | WA1-WA2, WB1-WB3, WC1-WC4, WF1-WF3, WG1, WH1 | N13-N17, N21-N23, N52, N53 |
| B2 timeout kills only the direct child, stray child holds the pipe | source-defect High | adapter in its own session, stdin/stdout/stderr are files, leader waited for WITHOUT reaping, group killed with the 11.4.263 guard on timeout and after return; a timeout of a create is not retried | PA0-PA3 | N18-N20 |
| B3 failed register write leaves the reference only in an ignored side file | source-defect Medium | the write-ahead journal keeps the results; the next run writes them first and hands the recovered reference to the adapter | WB1-WB4 | N15, N16, X09 |
| B4 wrapper exit 21/22 reported as "NOT recorded" | source-defect Medium | `flush` reads the rows back (`rows_present`) after 21/22 | WD1, WD2 | N44 |
| B5 non-string reference discarded, push recorded FAILED | source-defect Low | integers accepted as text, non-conforming references kept sanitized on the FAILED row | WE1 | X18, N45, N46 |
| C1 breaker records a reachable tracker as unreachable | source-defect Medium | `REJECTS` separates "answered" from "unreachable"; the rest after a rejecting trip is not attempted and not recorded (`breaker_open`) | BA1, BA2, BB1; old U1-U5 | N24, X02 |
| C2 `enabled` always 1, removed tracker never retired | source-defect Medium | `tracker_row_upserts`: enabled follows the config, full runs retire unnamed trackers | EA1-EA4 | N25-N27 |
| C3 any non-empty repository file accepted as receipt | source-defect Medium | receipt under THIS run's evidence directory, written during the call, non-empty, naming the item id and the returned reference | RA-evany, RA-stale, RA-emptyrec, RA-norefrec, RA-otheritem, RA-wrongitem, RA-ok | X03, N28-N32 |
| C4 §11.4.214 only printed | source-defect Medium | **PARTLY FIXED**: reported as `OWED reopen`, exit 7 on every run until done. The reopen with its recurrence link is **NOT performed**: `reg_recurrence_links` needs a reported entry or finding that a driver observation does not have (see `fix-r2-owed-items.md` OWED-WP22-A) | OWa-OWc | X10, N50 |
| C5 `/usr/bin/env` form missed | source-defect Low | `real_argv` skips `env [opts] NAME=v` | S9 | N10, X14 |
| D1 first-sync gate only prose | source-defect High | `first_sync_plan` / `first_sync_gate`: plan hash of the dry run + roster approver + pilot limit; REFUSED before any write | GA1-GA7 | N33-N39 |
| D2 no confidentiality filter, no secret scan | source-defect Medium | `outbound_problem`: confidential categories on public trackers (FAILED 78) and credential patterns in title/body (FAILED 77), no adapter call, matched text never printed | OA1-OA5 | N40-N43 |
| D3 drift half not implemented | source-defect Low | **PARTLY FIXED**: `remote_state` accepted and compared, drift reported (output and JSON). **NOT stored**: no column in the DDL (WP-06) | VA2 | N51 |
| E1 "45 of 45 killed" not proven (7 kills at unreachable checks) | test-instrumentation High | the runner scores the WHOLE transcript for environmental refusals, declares intended checks per mutant, counts a kill only at an intended check (MISATTR otherwise), runs a survivor against the other suite; the figure of revision 1 is withdrawn | the runner itself; negative and golden controls | all |
| E2 17 reviewer mutants survived | test-instrumentation High | every reviewer mutant is a permanent mutant (X01-X18, anchors translated onto the current text) with a test that kills it | see the X rows | X01-X18 (X03: killed through the receipt size check where reachable, see the mutation summary for equivalents) |
| E3 L1 without positive control, partial coverage | test-instrumentation Medium | L0/L0b controls, `drv` appends every driver output to the scan, five fixtures scanned, database dumps of all five | L0, L0b, L1; OA3 | n/a |
| E4 K1 without positive control | test-instrumentation Low | a valid SYNCED row is accepted first; rejections must not be foreign-key errors | K1 | n/a |
| E5 W2 asserts `>= 3` | test-instrumentation Low | at least two rows per batch on average | W2 | n/a |
| E6 X1 holds the lock itself | test-instrumentation Medium | two REAL drivers: the second is started while the first is inside a slow adapter call | X0-X2 | X04 |
| E7 `comm` locale bug in `clib.sh` rt_guard | test-instrumentation Medium | `LC_ALL=C` for sort and comm, `--check-order`, failing comm fails the check, control over planted names | RT (every container-leg suite) | n/a (shared file, minimal change, reported) |
| E8 fixture steps ignore the wrapper status | test-instrumentation Medium | each fixture step is checked and read back (A10pre, F0pre, G4pre; `step` in the r2 suite) | A10pre, F0pre, G4pre | n/a |
| E9 suite not deterministic under host pressure, failures say nothing | test-instrumentation Medium | `envretry` (bounded, never overrides the budget), the driver retries a refused wrapper call, the V-loop prints the driver output, the mint path prints the wrapper's last lines | n/a | n/a |
| F1 T188 TIC clause unmet | process-doc Medium | **NOT FIXED**: environment fact (nested podman); disclosed | n/a | n/a |
| F2 RED before T189 unmet | process-doc Low | **NOT FIXED**: past-order fact; this round's RED is real (new suite against the pre-fix driver) | n/a | n/a |
| F3 T191 evidence location | process-doc Medium | `t191_scratch_run.sh` writes `$EV/register/tracker_sync_0.json` plus a text dump of the scratch rows; the guide states that T191 is OPEN and the real register does not exist | the script exits non-zero unless all six candidates have rows | n/a |
| F4 guide overclaims | process-doc Medium | the four statements corrected (45 of 45 withdrawn; WAL-safe not claimed; "no remote issue lost" replaced by the write-ahead behaviour and its limit; "values never in any output" scoped to environment values and the config guard) | n/a | n/a |
| F5 guide orphan | process-doc Low | **NOT FIXED here**: `docs/scripts/README.md` is outside the scope; the guide states the owed link (OWED-WP22-C) | n/a | n/a |
| F6 list form breaks the shared contract | source-defect Low | list form REFUSED | S63 | n/a |
| F7a `--json` replaces any file | source-defect Low | confined to `.audit/` and the evidence directory | S10 | N11 |
| F7b evidence_dir in a work-package folder | source-defect Low | config `evidence_dir` is now `$EV/register` | config | n/a |
| F7c lower-case variable names | source-defect Low | accepted | S8 | N09 |
| F7d non-JSON `missing_env_names` crashes | source-defect Low | `jlist` | EB1 | N49 |
| F7e `mint_basis='manual'` | source-defect Low | `reporting_directive` | old M1-M2 (key lookup unchanged) | n/a |
| F7f DDL has no CHECK for `missing_env_names` | source-defect Low | **NOT FIXED**: DDL belongs to WP-06 (OWED-WP22-E) | n/a | n/a |
| F7g mint failure hides the reason | source-defect Low | the wrapper's last lines are printed | XD1 | N48 |
| F7h first mint failure blocks every tracker | source-defect Low | a failed mint continues with the others | XD2 | N47 |
