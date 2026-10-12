# tasks.md rev 40 - accepted with residual items (owner decision 2026-10-12)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Status | active |
| Basis | Owner answered the §11.4.276 round-budget question: one final xhigh review, then accept with residual LOW items tracked, no further rounds. |
| Review | Opus xhigh, Workflow path: NO-GO under the strict 0-source-defect bar (0 Blocking, 0 Important, 7 Minor). Report: scratchpad WF31-REVIEW-tasks-rev40.md (graph re-measured: 685 tasks, 0 cycles/dangling/dup, 0 ticked). |

Residual items to close inside the implementing tasks (each is a spec-text requirement on future code, not shipped behaviour; all are UNCONFIRMED until T050/T049/T056/T580e are implemented and reviewed):

| ID | Layer | Owner task | Item |
|---|---|---|---|
| WF31-1 | source-defect (spec) | T050/T580e | bounded retry of `release_record_unwritten`: place the attempt counter in exactly one spot (callee, persistent), fix the N vs N+1 off-by-one, define key + reset, define the operator-release path, drop the reference to undeclared "other commit-turn budgets" |
| WF31-2 | source-defect (spec) | T050 | `--grant` must be under the commit-turn flock and define behaviour when a grant already exists (§11.4.232(B)) |
| WF31-3 | test-instrumentation | T049/T056 | RED tests for the evrec idempotent key and "evrec must not take the commit-turn lock" |
| WF31-4 | test-instrumentation | T580e | fixture (xxiii) must be in the list of fixtures run under the bounded wait |
| WF31-5 | process-doc | UNTICK | T005a/T005b rows cite WP-09-T010.json, whose tasks field is T010/T005 only |
| WF31-6 | process-doc | T283a | citation-check findings rose 259 -> 485 (docs/16 Status cell prepend orphaned 110 rows; docs/21 +5, docs/16 l.873 +1): restructure the Status cell edit |
| WF31-7 | process-doc | docs/16, docs/05 | state diagram lacks release_record_unwritten/blocked_escape; docs/05 lacks fixtures (xix)-(xxiii) |
