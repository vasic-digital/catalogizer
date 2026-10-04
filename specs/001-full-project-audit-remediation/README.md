# Spec 001: Full Project Audit and Remediation (document index)

| Field | Value |
|---|---|
| Revision | 18 |
| Created | 2026-10-03 |
| Last modified | 2026-10-04 |
| Status | draft (revision 18: the task count is that of tasks.md revision 30, commit `84145b2e` (675 tasks: the 595 frozen ids T001 to T595 plus 80 suffix ids; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script, the same as revision 29, which revision 30 rewrites without adding an id; 1,422 edges from the `depends on` clauses, 1,437 with the `after` phrases); the counts are those of docs/21 revision 21, amended in place for tasks.md revision 30, which keeps 54 work packages and 65 inconsistencies. Revision 17: the task count was that of tasks.md revision 29, commit `7daa7938` (675 tasks: the 595 frozen ids T001 to T595 plus 80 suffix ids; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; revision 29 adds T335a and T570a); the counts are those of docs/21 revision 21, amended in place for tasks.md revision 29, which keeps 54 work packages and 65 inconsistencies. Revision 16: the task count was that of tasks.md revision 28, commit `fb1d1982` (673 tasks: the 595 frozen ids T001 to T595 plus 78 suffix ids; 317 `[TDD]`, 84 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script, the same as revision 27, commit `13bc5de0`, which revision 28 rewrites without adding an id); the counts are those of docs/21 revision 21, which keeps 54 work packages and 65 inconsistencies; the row of [checklists/round25-residue.md](checklists/round25-residue.md) now names the round-27 review it records and the revision 26, 27 and 28 dispositions. Revision 15: the task count was that of tasks.md revision 27, commit `13bc5de0` (673 tasks: the 595 frozen ids T001 to T595 plus 78 suffix ids; 317 `[TDD]`, 84 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script, the same as revision 26, commit `616fb74a`, which revision 27 rewrites without adding an id); the counts are those of docs/21 revision 20, which keeps 54 work packages and 65 inconsistencies; section 2 links the round-25 review residue [checklists/round25-residue.md](checklists/round25-residue.md), which no page linked before (§11.4.212). Revision 14: the task count was that of tasks.md revision 26, commit `616fb74a` (673 tasks: the 595 frozen ids T001 to T595 plus 78 suffix ids; 317 `[TDD]`, 84 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script); revision 24 is committed as `c2b01d73` (670 tasks) and revision 25 as `2e1fd314` (671 tasks); revision 25 added T134a and revision 26 adds T225d and T225e; the counts are those of docs/21 revision 19, which keeps 54 work packages and 65 inconsistencies; the `$AUD` folder note names the reviewed scan exclusion list `worktree-scan-exclusions.txt`. Revision 13: the task count was that of the tasks.md revision 24 candidate, then not yet committed and since committed as `c2b01d73` (670 tasks: the 595 frozen ids T001 to T595 plus 75 suffix ids; the committed revision 23 has 666; revisions 18 to 24 added T084a, T139a, T175a, T200a, T225a, T225b, T225c, T240a, T241a, T254a, T254b, T272a, T277a, T277b, T288a, T288b, T318a, T322a, T334a, T336a, T420a, T456a, T456b, T456c, T564a, T566a, T580d, T580e and T580f; 315 `[TDD]`, 84 `[REVIEW]`); the counts are those of docs/21 revision 18: 54 work packages (WP-58, the fix wave of the unowned-component findings, in P5; sizes M 24) and 65 inconsistencies (IC-62 the review gate of each declared path, IC-63 the P2 exit and the build-host input, IC-64 the audit unit partition and WP-58, IC-65 the final manual QA and the next version); the folder notes name the unit partition under `$AUD` and the released copy of the commit-push code in each run folder. Revision 12: the task count is that of tasks.md revision 17 (641 tasks: the 595 frozen ids T001 to T595 plus 46 suffix ids; revision 15 added T137a and T440c, revision 16 none, revision 17 T442a, T442b and T580c; 302 `[TDD]`, 74 `[REVIEW]`); the counts are those of docs/21 revision 17; the `.audit/` paragraph follows docs/16 revision 14 §9.6: the driver secret of the build dispatcher is no longer under the checkout (it is a mode-0600 file under `${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<checkout-id>/`, outside every container mount, the kernel keyring only a cache), so the `.audit/secrets/build_hmac.key` entry is withdrawn, and the per-build folder holds a terminal directory instead of a consumed set. Revision 11: the task count is that of tasks.md revision 14 (636 tasks: the 595 frozen ids T001 to T595 plus 41 suffix ids; revision 14 added T005d, T094d, T121b, T435c, T440b and T447b); the counts are those of docs/21 revision 16, which reconciles the plan with the committed tasks.md rev 14 without changing the phase, work-package, risk, decision or inconsistency counts; the `.audit/` note names the callback arguments and state files of a dispatched build and the driver secret file `.audit/secrets/build_hmac.key`, and drops the per-build callback table, which tasks.md rev 14 replaces by the tracked registry `scripts/build/callbacks.tsv`. Revision 10: the task count is that of tasks.md revision 13 (630 tasks: the 595 frozen ids T001 to T595 plus 35 suffix ids; revision 13 added T005a, T005b, T005c, T006a, T012a, T089a, T093a, T094a, T094b, T094c, T121a, T440a, T447a, T504a, T548a and T561a, and T067a, an earlier suffix id, is now counted); the counts are those of docs/21 revision 15, which applies the plan owner's decisions of 2026-10-04 (every build on the remote build host, dispatched event-driven; SLSA Build L2 at minimum with no L1 interim; every pin and package dependency moved to latest whenever possible after its tests pass) without changing the phase, work-package, risk, decision or inconsistency counts; the `.audit/` note names the per-build folders `.audit/builds/<build_id>/` of the event-driven build dispatcher. Revision 9: the counts are those of docs/21 revision 14, whose inconsistency list runs from IC-01 to IC-61 (IC-59 the nested gitlinks of a moved module and the constitution's own work, IC-60 FR-008 and SC-003 against the legacy headerless class, IC-61 the can-i-deploy check at the release seam) and whose grouped owner decisions are 42 (ODG-42 added); the task count is unchanged at 613, because the P4 to P7 half of tasks.md rev 13 adds no id; the `.audit/` note names the retired nested checkouts under `.audit/removed/<op_id>/<path>/` and the release-seam outputs under `.audit/out/<op_id>/`. Revision 8: the task count is that of tasks.md revision 12 (613 tasks: the 595 frozen ids T001 to T595 plus the 18 suffix ids T040a, T040b, T042a, T064a, T248a, T283a, T308a, T394a, T434a, T435a, T435b, T448a, T462a, T579a, T580a, T580b, T595a and T595b; revision 12 added T448a); the counts are those of docs/21 revision 13, whose inconsistency list runs from IC-01 to IC-58 (IC-54 the size bound of the evidence ledgers, IC-55 the held table changes and the class resolution of the commit-push checks, IC-56 the `--commit-before-integrate` remediation and the stores with a single writer, IC-57 the revision headers of the legacy Markdown files, IC-58 the main repository's ODG-41 kind) and whose risk register has 35 risks (R-35 added); the `.audit/` note names the retired nested checkouts of a constitution update. Revision 7: the task count is that of tasks.md revision 11 (612 tasks: the 595 frozen ids T001 to T595 plus the 17 suffix ids T040a, T040b, T042a, T064a, T248a, T283a, T308a, T394a, T434a, T435a, T435b, T462a, T579a, T580a, T580b, T595a and T595b; revision 10 added none, revision 11 added T040b, T308a and T394a); the counts are those of docs/21 revision 12, whose inconsistency list runs from IC-01 to IC-53 (IC-52 the path classes of the commit-push checks with the hex filter of the secret fold, IC-53 the release of a merge commit on its own merge-review file and the resolution of a merge conflict); the `.audit/` note names the merge record, backup and conflict files of a commit-push run, the resolution folder of a refused merge and the copies of the WP-73 push records. Revision 6: the task count is that of tasks.md revision 9 (609 tasks: the 595 frozen ids T001 to T595 plus the 14 suffix ids T040a, T042a, T064a, T248a, T283a, T434a, T435a, T435b, T462a, T579a, T580a, T580b, T595a and T595b); the inconsistency count is docs/21 revision 11's (IC-01 to IC-51: IC-49 the script's own merge integration of a diverged repository, IC-50 the filters and exemptions of the pre-commit checks, IC-51 the scope and bootstrap of the ratchet baselines); the `.audit/` note names the writers of the pending pin moves; the checklist and the proof-of-concept READMEs now carry their revision headers. Revision 5: the task count is that of tasks.md revision 8 (604 tasks: the 595 frozen ids T001 to T595 plus the 9 suffix ids T040a, T042a, T248a, T283a, T434a, T462a, T580a, T595a and T595b); the inconsistency count is docs/21 revision 10's (IC-01 to IC-48) and the grouped owner decisions are 41 (ODG-41 added); the new contract `review-verdict.schema.json` is listed; the `.audit/` note names the final verification reports and the container output folders. Revision 4: the task count is that of tasks.md revision 7 (601 tasks: the 595 frozen ids T001 to T595 plus the 6 suffix ids T040a, T042a, T248a, T434a, T462a and T580a); the inconsistency count is docs/21 revision 9's (IC-01 to IC-45); the ignored `.audit/` folder of the commit-push runs is named next to the folders the work creates. Revision 3: counts re-taken from docs/21 revision 8 (254 plan-document seeds); the task count stated (595 numbered tasks, ids T001 to T595 frozen, later tasks with suffix ids); the task-id citation rule added to "How to read"; reachability note updated to the committed root link. Revision 2: "How to read" note and the phase and work-package table from docs/21 revision 7; counts re-taken from docs/21 revision 7; reachability note updated) |
| Feature | `specs/001-full-project-audit-remediation` |
| Purpose | Entry point of this specification folder: every document in it is linked from this page, grouped by role, so each one is reachable by following links (spec FR-013, SC-006; constitution §11.4.212) |
| Reachability note | This page is reachable from [spec.md](spec.md) and from [plan.md](plan.md), and the repository's main README links it (README.md line 40, committed in `5807ca9f`, read with `git show HEAD:README.md` on 2026-10-03) |

## Table of contents

1. [Start here](#1-start-here)
   - [How to read this plan](#how-to-read-this-plan)
   - [Phases and work packages](#phases-and-work-packages)
2. [Specification, plan and tasks](#2-specification-plan-and-tasks)
3. [Design artifacts](#3-design-artifacts)
4. [Planning documents 01 to 21](#4-planning-documents-01-to-21)
5. [Proof-of-concept tools](#5-proof-of-concept-tools)
6. [Contract schemas](#6-contract-schemas)
7. [Folders the work will create](#7-folders-the-work-will-create)
8. [Counts at a glance](#8-counts-at-a-glance)

## 1. Start here

Read in this order:

1. [spec.md](spec.md): what the feature must achieve (FR-001 to FR-025, SC-001 to SC-012) and the owner's clarifications.
2. [plan.md](plan.md): technical context, constitution check and project structure.
3. [docs/21-master-plan-phases-risks-and-traceability.md](docs/21-master-plan-phases-risks-and-traceability.md): the master plan that turns documents 01 to 20 into 8 phases and 54 work packages, with risks, owner decisions and the traceability matrix.
4. [tasks.md](tasks.md): the task list by phase and work package.
5. [quickstart.md](quickstart.md): read-only commands that show the state the plan starts from.

### How to read this plan

- **Layers.** [spec.md](spec.md) says what must be true (FR-001 to FR-025, SC-001 to SC-012). [docs/21](docs/21-master-plan-phases-risks-and-traceability.md) turns documents 01 to 20 into phases, work packages, owner decisions and risks, and maps every requirement to evidence (its section 6). [tasks.md](tasks.md) breaks each work package into tasks under a heading with the same `WP-nn` id. Documents 01 to 20 hold the detailed method that a work package cites as `docNN §x`; read them for a package, not front to back.
- **Identifiers.** `WP-nn`: work package (docs/21 section 5). `ODG-nn`: grouped owner decision (docs/21 section 8). `OD-nn`: one of the 79 finer owner decisions in [research.md](research.md); a different id space from `ODG-nn` (docs/21 IC-34). `IC-nn`: an inconsistency between documents and its resolution (docs/21 section 10). `Tnnn`: a task in tasks.md; the ids T001 to T595 are frozen from tasks.md revision 6, a task added later takes a suffix id placed where it belongs (for example T039a), and a citation in another document always names the current id (checked by the task-id citation check of docs/21 section 12.7, which a planted stale id fails). `R-nn` is a risk in docs/21 section 7 but a research decision in research.md, so a citation across documents is written `docNN:ID` (docs/21 IC-19).
- **Disagreements.** Where two documents disagree, docs/21 section 10 records both values and the resolution. A value the plan could not settle is marked `UNCONFIRMED:`, and the work package that measures it owns the answer.
- **Paths.** `$AUD` is `specs/001-full-project-audit-remediation/audit` and `$EV` is `specs/001-full-project-audit-remediation/evidence` (docs/06 section 11). Neither exists yet.
- **Claims.** Nothing in this folder reports work as done. Every acceptance needs machine-produced evidence from the work itself (docs/06, spec FR-022 and SC-012). Owner decisions block the work they name until answered (docs/21 planning rule 6).
- **Counts.** Every count is a point-in-time value of 2026-10-03 and is re-measured by the work package that relies on it. tasks.md revision 30, commit `84145b2e`, has 675 tasks, the same ids as revision 29, commit `7daa7938` (revisions 28, commit `fb1d1982`, 27, commit `13bc5de0`, and 26, commit `616fb74a`, have 673; revision 25, commit `2e1fd314`, has 671): the 595 ids T001 to T595, frozen since revision 6, and 80 suffix ids (T005a, T005b, T005c, T005d, T006a, T012a, T040a, T040b, T042a, T064a, T067a, T084a, T089a, T093a, T094a, T094b, T094c, T094d, T121a, T121b, T134a, T137a, T139a, T175a, T200a, T225a, T225b, T225c, T225d, T225e, T240a, T241a, T248a, T254a, T254b, T272a, T277a, T277b, T283a, T288a, T288b, T308a, T318a, T322a, T334a, T335a, T336a, T394a, T420a, T434a, T435a, T435b, T435c, T440a, T440b, T440c, T442a, T442b, T447a, T447b, T448a, T456a, T456b, T456c, T462a, T504a, T548a, T561a, T564a, T566a, T570a, T579a, T580a, T580b, T580c, T580d, T580e, T580f, T595a, T595b). A later revision adds more suffix ids, so count the task lines of tasks.md for the current total.

### Phases and work packages

From docs/21 revision 21, sections 3.1 and 11.2. Tasks: 675 in tasks.md revision 30 (the 595 frozen ids T001 to T595 plus 80 suffix ids); a later revision adds suffix ids only.

| Phase | Name | Work packages | Count |
|---|---|---|---:|
| P0 | Foundation | WP-01 to WP-09 | 9 |
| P1 | Containerized infrastructure | WP-10 to WP-15 | 6 |
| P2 | Inventory and baselines | WP-20 to WP-24 | 5 |
| P3 | Audit pass | WP-30 to WP-39, WP-39R | 11 |
| P4 | Contract layer | WP-40, WP-41 | 2 |
| P5 | Remediation | WP-50 to WP-58 | 9 |
| P6 | Test closure, QA, performance, documentation | WP-60 to WP-65 | 6 |
| P7 | Verification and closure | WP-70 to WP-74, WP-74R | 6 |
| **Total** | 8 phases | 13 streams; sizes S 3, M 24, L 20, XL 7 (docs/21 section 11) | **54** |

## 2. Specification, plan and tasks

| Document | Role |
|---|---|
| [spec.md](spec.md) | Feature specification (clarified) |
| [checklists/requirements.md](checklists/requirements.md) | Specification quality checklist |
| [checklists/round25-residue.md](checklists/round25-residue.md) | Round-25 review residue of tasks.md revision 25 (3 blocking, 25 important, 17 minor findings), the round-27 review of revision 27 (tasks.md 2 blocking, 14 important, 18 minor findings; docs set 3 minor) and the revision 26, 27 and 28 dispositions, tracked as open items |
| [plan.md](plan.md) | Implementation plan |
| [tasks.md](tasks.md) | Task list (phases P0 to P7, every work package of docs/21 section 5; 675 tasks in revision 30, commit `84145b2e`: the frozen ids T001 to T595 plus 80 suffix ids, later tasks with suffix ids) |

## 3. Design artifacts

| Document | Role |
|---|---|
| [research.md](research.md) | Phase 0 research: decisions R-01 to R-30, conflicts between documents, the 79 owner decisions OD-01 to OD-79 |
| [data-model.md](data-model.md) | Entities, fields, state machines and completion gates |
| [quickstart.md](quickstart.md) | Runnable validation guide for the planning baseline |
| [contracts/README.md](contracts/README.md) | Machine-readable interfaces: producer, consumers and validation result of each schema |

## 4. Planning documents 01 to 21

### 4.1 Baseline and method

| Document | Subject |
|---|---|
| [01 - System Architecture Map](docs/01-system-architecture-map.md) | Verified baseline of the applications, services, submodules and contracts |
| [02 - Audit Methodology and Index Strategy](docs/02-audit-methodology-and-index-strategy.md) | How the audit runs, the CodeGraph and Lumen health gate, finding records and determinism |
| [03 - Existing Issue Inventory](docs/03-existing-issue-inventory.md) | Every existing problem source and the reconciliation plan |
| [04 - Findings Register Design](docs/04-findings-register-design.md) | The single problem register: DDL, custody triggers, import and exports |
| [05 - Test Strategy and Coverage Matrix](docs/05-test-strategy-and-coverage-matrix.md) | Test types per application, coverage baselines and targets |
| [06 - Determinism and Evidence Framework](docs/06-determinism-and-evidence-framework.md) | Evidence records, RED and GREEN polarity, chaining, anchors and the evidence layout |

### 4.2 Per-application audit plans

| Document | Subject |
|---|---|
| [07 - Backend (catalog-api) Audit Plan](docs/07-backend-catalog-api-audit-plan.md) | Go API |
| [08 - Web Client Audit Plan](docs/08-web-client-audit-plan.md) | catalog-web and its React and TypeScript submodules |
| [09 - Desktop and Installer Audit Plan](docs/09-desktop-and-installer-audit-plan.md) | catalogizer-desktop and installer-wizard |
| [10 - Android and Android TV Audit Plan](docs/10-android-and-android-tv-audit-plan.md) | catalogizer-android and catalogizer-androidtv |

### 4.3 Cross-cutting programmes

| Document | Subject |
|---|---|
| [11 - Submodules Audit and Update Runbook](docs/11-submodules-audit-and-update-runbook.md) | Every submodule, the update layers and recursive verification |
| [12 - HelixQA, Challenges and Governance Plan](docs/12-helixqa-challenges-and-governance-plan.md) | QA banks, Challenges, governance gates and the commit-push sequence |
| [13 - Documentation Program Plan](docs/13-documentation-program-plan.md) | Reachability, exports, manuals, guides, FAQs and definitions |
| [14 - Performance Engineering Plan](docs/14-performance-engineering-plan.md) | Baselines, targets, regression gate and bottlenecks |
| [15 - Security, Secrets and Danger-Zone Plan](docs/15-security-and-danger-zone-plan.md) | Threat model, scanners, secrets, DAST, licences and danger zones |
| [16 - Containerized Infrastructure and Local Enforcement Plan](docs/16-containerized-infrastructure-and-local-enforcement-plan.md) | Pinned images, runners, remote builds, verifier and commit-push script |

### 4.4 Research

| Document | Subject |
|---|---|
| [17 - Research: Engineering Practices](docs/17-research-engineering-practices.md) | Tools and practices for the audit and remediation work |
| [18 - Research: Product Innovation and Game Changers](docs/18-research-product-innovation-and-game-changers.md) | 47 candidate improvements; how each is routed is in docs/21 sections 9.1 and 9.5 |
| [20 - Research, Second Pass](docs/20-research-second-pass-gaps.md) | Primary-source checks of the gaps admitted in document 17 |

### 4.5 Proof-of-concept results and master plan

| Document | Subject |
|---|---|
| [19 - Proof-of-concept tools and measured results](docs/19-poc-tools-and-results.md) | The three self-tested tools of section 5 and what they measured |
| [21 - Master Plan: Phases, Risks and Traceability](docs/21-master-plan-phases-risks-and-traceability.md) | Synthesis of documents 01 to 20 |

## 5. Proof-of-concept tools

Read-only tools, each self-tested with golden-good, golden-bad and control cases; their stored results are next to them under `poc/`.

| Tool | README |
|---|---|
| Recursive repository verifier | [poc/repo_verify/README.md](poc/repo_verify/README.md) |
| Documentation link crawler | [poc/doc_links/README.md](poc/doc_links/README.md) |
| API route and contract drift detector | [poc/route_drift/README.md](poc/route_drift/README.md) |

## 6. Contract schemas

JSON Schema draft 2020-12; [contracts/README.md](contracts/README.md) states the producer, consumers and validation result of each.

| Schema | `$id` |
|---|---|
| [repo-verification-report.schema.json](contracts/repo-verification-report.schema.json) | `repo-verification-report/1` |
| [link-crawl-report.schema.json](contracts/link-crawl-report.schema.json) | `link-crawl-report/1` |
| [route-drift-report.schema.json](contracts/route-drift-report.schema.json) | `route-drift-report/1` |
| [evidence-record.schema.json](contracts/evidence-record.schema.json) | `ev/1` |
| [finding.schema.json](contracts/finding.schema.json) | `finding/1` |
| [bank-case.schema.json](contracts/bank-case.schema.json) | `bank-case/3` |
| [review-verdict.schema.json](contracts/review-verdict.schema.json) | `review-verdict/1` |

## 7. Folders the work will create

These do not exist yet; tasks.md creates them and this index will link their entry documents once they exist.

| Path | Content | Owner document |
|---|---|---|
| `audit/` (`$AUD`) | audit runs, one `finding/1` file per finding, index health, golden questions, the unit partition `unit-partition.tsv` that maps every tracked path to one audit unit or a reasoned exclusion (tasks.md T225; docs/21 IC-64), the unit list `units.json` that seeds the register's components (T225d, T225e) and the reviewed scan exclusion list `worktree-scan-exclusions.txt` for plan-written state (T225, T303) | docs/02 |
| `evidence/` (`$EV`) | ledger, anchors, blobs, verdicts, reviews, human-checkpoint and phase-exit records | docs/06 section 11 |
| `decisions/` | owner-decision intake records | docs/21 section 8, tasks.md WP-01 |
| `matrix/` | test-type applicability and coverage matrix | docs/05 section 13 |
| `perf/` | performance targets and baselines | docs/14 |

Not tracked and outside this folder: the ignored `.audit/` directory at the repository root holds one folder per commit-push run (`.audit/commit-push/<run_id>/`: its report, verifier JSON and transcripts, the released copy of the commit-push code that the run executes (`released/`, materialised from the released commit, the merge base of the reachable remote tips of `main`; tasks.md T042), and for a merge integration its merge record `merge.json`, the backup under `backup/` and the conflicting files of a refused merge under `conflicts/`), the resolution of a refused merge (`.audit/merge-resolution/<run_id>/`), the copies of the WP-73 push records (`.audit/wp73/t581/<run_id>/`), the leftover checkouts of nested gitlinks that a move deletes, moved there with their sha256 lists, never deleted, one folder per operation (`.audit/removed/<op_id>/<path>/`; docs/21 IC-59), the pending pin moves (`.audit/pending_pins.tsv`, written by `--repo` commits and by the recorded fast-forwards of the submodule update layers), the long-op records, one folder per dispatched remote build with its event journal, its terminal directory (the terminal kind, the consumed event and the callback state, created once by a rename), callback arguments and brought-back artifacts (`.audit/builds/<build_id>/`; docs/16 §9.6), the read-only verified test binaries that split lanes run locally (`.audit/builds/bin/<sha256>`), the default output folders of container runs and the outputs of the release-seam checks, the can-i-deploy verdict included (`.audit/out/<op_id>/`; docs/21 IC-61) and the final strict verification reports (`.audit/verify/<run_id>/`). The driver secret of the build dispatcher is not under the checkout: it is a mode-0600 file under `${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<checkout-id>/`, outside every container mount, the kernel keyring only a cache (docs/16 revision 14 §9.6). The commit-push script writes only there, never into the tracked tree; the record of a push is the commit and its `CPA-Run:` trailer (docs/06 section 11).

## 8. Counts at a glance

Taken from docs/21 revision 21 (section 1.3, unchanged since revision 19; revision 18 added WP-58, and revision 19 re-projects the evidence-ledger row on the 317 `[TDD]` tasks); each is re-measured by the work package that relies on it.

| Quantity | Value |
|---|---|
| Phases / work packages / streams | 8 / 54 / 13 |
| Tasks | 675 in tasks.md revision 30, commit `84145b2e` (the 595 frozen ids T001 to T595 plus 80 suffix ids; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, recounted by script for docs/21 revision 21 as amended for revision 30, the same as revision 29, commit `7daa7938`; revisions 28, 27 and 26 have 673 tasks, 317 `[TDD]` and 84 `[REVIEW]`, revision 25 671 and revision 24 670); suffix-id tasks added later are counted from tasks.md itself |
| Cross-document inconsistencies resolved or tracked | 65 (IC-01 to IC-65, docs/21 section 10) |
| Risks | 35 |
| Grouped owner decisions | 42 (ODG-01 to ODG-42), plus 16 ungrouped `research.md` decisions (docs/21 section 8.6) |
| Owner decisions in `research.md` | 79 (OD-01 to OD-79) |
| Plan-document seed findings | 254 itemised, at most 198 register items after duplicate families |
| doc18 innovation entries tracked as Feature items | 34 |
| Repositories | 98 (the main repository and 97 submodules: 44 direct, 53 nested) |
