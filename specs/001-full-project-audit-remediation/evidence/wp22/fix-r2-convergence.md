# WP-22 fix round 2: convergence assessment (11.4.276, written BEFORE the fixes)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Subject | `scripts/register/sync_trackers.sh` (T189) and its tests, after the independent review `WF23-REVIEW-wp22` (verdict NO-GO, 19 `source-defect`, 9 `test-instrumentation`, 5 `process-doc` findings) |
| Round | 2 of the review loop of this item (round 1 = the author's build; this is the first fix pass after the first independent verdict) |
| Fixer | one Sonnet instance (11.4.231 default tier); the review itself ran on Opus (11.4.209) |
| Budget | 11.4.276: at most 7 rounds (5 declared); this pass is a ONE-PASS fix of every finding, so the structural-round trigger of clause (E)(2) is applied in advance |

## 1. Why round 1 produced 33 findings (root causes, not instances)

| # | Root cause | Findings it explains |
|---|---|---|
| RC1 | **Wrong model of the adapter process.** The driver modelled the adapter as "a child that writes a JSON line to a pipe and exits"; the real behaviour space includes a child that leaves background children, prints undecodable bytes, hangs, or dies together with the driver. | B1 (crash and non-UTF-8 lose pushes), B2 (timeout kills only the direct child, a stray child holding the pipe makes a success FAILED), B3 (unwritten rows never read back), B5 |
| RC2 | **Remote effects were not made durable before the next effect.** SYNCED rows were buffered in memory; the register write was the only record. A write-ahead record did not exist. | B1, B3, B4 (exit 21/22 reported as "not recorded" although the write landed) |
| RC3 | **Deny-list instead of grammar for credential-bearing text** (one regex, one `key=value` shape). | A1, A2, A3 (the same "silently disable the check" class one level up), F6 |
| RC4 | **Prose where a gate belongs** (docs/04 section 10.3 "Gate: first sync ... dry run reviewed by the owner, then a limited pilot batch"). | D1 |
| RC5 | **Conflated states**: one counter for "failed" and "unreachable"; `enabled` always 1; a receipt was "a non-empty file of the repository". | C1, C2, C3 |
| RC6 | **Tests that could not fail for the property they name**: the mutation scorer looked at the first FAIL line only and counted environmental write failures as kills (7 of 45); fixture steps ran without checking their exit status; leak scans had no positive control; the revision fields, the latest reference, the backoff and the shared lock were never asserted. | E1-E9, 17 of 18 reviewer mutants survived |
| RC7 | **Guide statements written from intent, not from evidence** ("45 of 45", "WAL-safe", "no remote issue is lost", "values never read into any output"). | F4 |

## 2. Class-complete inventories (control-needled), built BEFORE the fixes

* **Credential-bearing command forms (A1).** The 14-form probe of the review (`probe_config_secret_guard.txt`: c01-c18, positive controls c01/c08/c13/c14 refused by the old guard, 10 accepted) is the founding inventory. The class is closed by a POSITIVE grammar, not by listing: any token carrying `user:password@`, any Authorization/Bearer/header text, any credential-shaped token (known prefixes), any key or flag that NAMES a credential (token, secret, password, api-key, auth, pat, ...) with a value (`=value` or the next token) unless the value is a `$NAME` reference or a `{db}`/`{id}` placeholder, and any single-letter flag followed by a bare word. The test inventory is 19 refused forms (S01-S19) and 6 accepted controls (S51-S56); the pre-fix driver is the control that proves the instrument can see: in the RED run (`fix-r2-red-prefix-r2.txt`) it REFUSES the 7 forms its old regex covered (S01, S05, S06, S07, S16, S17, S18) and ACCEPTS 12 of the 19 (S02, S03, S04, S08-S15, S19), and it accepts all 6 plain-command controls (S51-S56).
* **Adapter-process behaviours (B1/B2/B5).** exit 0/non-zero/signal; launch failure; timeout (direct child, background child); undecodable stdout; driver killed with SIGKILL after the adapter pushed; driver terminated by SIGTERM; register write refused (1), journal row missing after a landed write (21), exit status unknown (22); reference as string, integer, with a space. One test per member (WA1, WB1-WB4, WC1-WC4, WD1-WD2, WE1, WF1, WG1, PA1-PA3).
* **State and registry semantics (C1/C2/D2).** breaker trip causes {unreachable only, any rejecting answer}, streak reset; enabled {true, false, removed from config, subset run}; receipt {right, other path, stale, empty, no reference, other item, other reference}; outbound {public/private tracker x confidential/secret/plain item}.
* **Config evolution (A3/F6/F7).** duplicate key at tracker level and at top level, list-form env_passthrough, literal env_passthrough, whitespace-only credential, lower-case variable name, `/usr/bin/env` and `env NAME=v` wrappers, `bash missing.sh`, `--json` outside `.audit/` and the evidence directory.

## 3. Ground truth re-established

The reviewer's probes (`probe_adapter.sh`, `probe_run_probes.sh`, RUN1-RUN6) were re-read and each behaviour is now a permanent test (`test_sync_fix_r2.sh`, run against the PRE-FIX driver for the RED evidence). The authoritative sources consulted for the model: `locked.sh` header (exit 21 = the write ran, its journal row could not be written; 22 = container exit status unknown), `register_ext.sql` (`v_stale_tracker_sync` lists only `reg_trackers.enabled=1`; the closed `mint_basis` set contains `reporting_directive`; `reg_recurrence_links` needs a reported entry or finding), `report_item.sh` (`env_passthrough` is read with `.items()`: mapping only), constitution 11.4.263 (never signal pgid <= 1).

## 4. Structural decisions taken now (clause (E)(3) applied in advance)

1. The adapter runs in its OWN session; its stdin, stdout and stderr are FILES, never pipes; the leader is waited for WITHOUT reaping, the group is signalled while the pid is still reserved, the guard `pgid > 1 and pgid != own group` precedes every `killpg`.
2. A write-ahead journal (`.audit/sync/wal/<run>.jsonl`, fsync): intent before, result after every adapter call; the next run reconciles first.
3. The first-sync gate is code: plan hash from a dry run + roster approver + pilot limit.
4. The command text is never persisted (hash only). Scope decision recorded: the review's alternative "store the adapter path only" was not chosen because the register needs to notice a changed command; a 16-hex digest does that without exposing text.

## 5. What is deliberately NOT fixed in this pass (with reason)

* **11.4.214 reopen with its recurrence link is not performed by the driver.** `reg_recurrence_links` requires `reported_entry_id` or `reported_finding_id` (a source entry or finding); a driver observation has neither, and inventing one would be a fabricated record (11.4.6). The driver now exits 7 until the reopen is done through the register custody path and reports the owed item (was: print and exit 0). A tracked item is owed: see `fix-r2-owed-items.md`.
* **Drift storage (D3).** `reg_tracker_sync_log` has no `remote_state` column (DDL belongs to WP-06); drift is reported in the output and the JSON, not stored.
* **T188 clause "run through TIC tooling unit" and "RED observed before T189"** remain as disclosed (F1/F2): an environment fact and a past-order fact; not changeable by a code fix.
* **T191 on the real register** needs `docs/workable_items.db` (T069 go-live).
