# WP-06 T068 fix round 2: items owed (to be tracked; the register does not exist yet, so they are listed here for the tracker owner)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08T18:31:49Z |
| Last modified | 2026-10-08T18:31:49Z |
| Rule | 11.4.197 (a started effort is completed or closed with a reason), 11.4.276(E) (nothing in this list is a silent deferral); same form as `../wp22/fix-r2-owed-items.md` |

| Id (proposed) | Owed item | Why it is not done in this pass | Owner / precondition |
|---|---|---|---|
| OWED-WP06-15 | **Track the T068 review records** `evidence/reviews/WP-06-T068.json` and `WP-06-T068-r2.json` (untracked, not ignored) and the earlier WF2/WF6/WF10/WF13/WF15 review documents that the README and commits cite but the repository does not hold (11.4.215: a binding review record lives tracked). | This round must not edit files under `evidence/reviews/` and does not commit. | Orchestrator, in the next commit turn; then register this id as a `Task` item at T069. |
| OWED-WP06-16 | **`reg_test_runs` MUTATION rows** for the DDL, gate, replay and locked mutation results (`fix-r2-t068-*mutations*`, `fix-r3-t068-*mutations*`). | `reg_test_runs.atm_id` is a NOT NULL foreign key to `reg_ids`; no register item exists before the T069 import and minting ids for them now would be a mint outside the import. | After T069: attach to the WP-06 item. |
| OWED-WP06-17 | **docs/04 v6 revision** (`fix-r3-t068-owed-docs04-section5-v6.diff`, `patch --dry-run` exit 0 against the current docs/04) plus the section 6 constraint-map rows for `reg_ids_mint_monotone`, `uq_reg_ids_import_marker`, `v_ids_malformed`, `v_import_mint_orphans`. | docs/04 is the binding document and has its own owner. | docs/04 owner. |
| OWED-WP06-18 | **Container-image digest** in the T060-T063 transcripts and the vendored sqlite 3.53.3 stat-pin check (host sqlite is 3.50.6); `export-header.json` revision header for Issues.md / Issues_Summary.md; the OWED-WP06-14 SIGKILL residual and container KILL escalation. | Needs RUNP IMG-TESTUTIL (T007/T008) or an engine / export.sh change; no new container work in this round. | T007/T008 owner. |
| OWED-WP06-19 | **16 MiB margin of the real register**: the 2,010-item fixture (`fix-r2-t068-size-2010-items.txt`) is 10.3 MiB; source entries, findings, evidence, status log and test-run rows are not in it. | The real rows do not exist before T069. | Re-measure at T069. |
| OWED-WP06-20 | **Independent confirmation** of the author-reviewed equivalent mutant `prefix_yaml_must_exist_removed` (`equivalent_gate_mutants.tsv`). | Independence: the author cannot confirm own equivalence. | Next re-review. |
