# WP-22 fix round 2: items owed (to be tracked; the register does not exist yet, so they are listed here for the tracker owner)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Rule | 11.4.197 (a started effort is completed or closed with a reason), 11.4.276(E) (nothing in this list is a silent deferral) |

| Id (proposed) | Owed item | Why it is not done in this pass | Owner / precondition |
|---|---|---|---|
| OWED-WP22-A | **11.4.214 reopen and recurrence link for a terminal minted tracker item.** The driver now reports `OWED reopen of <id>` and exits 7 on every run until it is done; the reopen itself (engine `reopen`, `reg_status_log`) and the `reg_recurrence_links` row (verdict SAME_DEFECT) must go through the register custody path. | `reg_recurrence_links` requires `reported_entry_id` or `reported_finding_id`; a driver observation has neither, and inventing a source entry would fabricate a record (11.4.6). | WP-06 custody owner: define the intake path for tracker-sync recurrences (a `reg_source_entries` row of kind `tracker_sync`, or a finding), then let the driver call it. |
| OWED-WP22-B | **T191 on the real register.** Evidence `$EV/register/tracker_sync_0.json` exists only from a scratch register (stated inside the file). | `docs/workable_items.db` does not exist (go-live T069). | After T069: `scripts/register/sync_trackers.sh --json specs/001-full-project-audit-remediation/evidence/register/tracker_sync_0.json`; T191 stays OPEN until then. |
| OWED-WP22-C | **Index link**: add `sync_trackers.md` to `docs/scripts/README.md` (11.4.212). | The index is outside this fix round's scope and was not edited. | Owner of `docs/scripts/README.md`. |
| OWED-WP22-D | **Shared scorer**: `scripts/register/tests/mutscore.sh` scores a run as environmental only when the FIRST FAIL line names a refusal; the other register mutation runners share it. `mutate_sync_driver.sh` now scores the whole transcript locally; the shared file is unchanged. | Shared with other runners outside this scope. | Owner of `mutscore.sh` / `mutate_register_ops.sh`. |
| OWED-WP22-E | **DDL (WP-06)**: a `remote_state` column (drift is reported, not stored) and a CHECK that `credentials_absent` rows carry `missing_env_names`. | DDL belongs to WP-06. | WP-06. |
| OWED-WP22-F | **Owner decisions (ODG-10)**: the `first_sync_approvers` roster, the pilot size, the register categories that hold security findings (`confidential_categories`; the closed category set has no `security`), which trackers are public. | Owner decisions. | Owner. |
| OWED-WP22-G | **T188 clauses "run through TIC tooling unit" and "RED observed before T189"** stay unmet (disclosed by round 1): an environment fact (nested podman) and a past-order fact. | Not changeable by a code fix. | T194 review to rule on the wording. |
| OWED-WP22-H | **Second independent review** of this fix pass (constitution 11.4.142 / 11.4.209 / 11.4.134, round 2 of at most 7 under 11.4.276). The first review's NO-GO is not lifted by the fixer. | Independence: the fixer cannot review its own fixes. | Reviewer. |
