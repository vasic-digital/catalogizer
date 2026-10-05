# Feature Specification: Full Project Audit and Remediation

| Field | Value |
|---|---|
| Revision | 9 |
| Created | 2026-10-03 |
| Last modified | 2026-10-05 |
| Status | Draft (revision 9, analyze-remediation round approved by the owner on 2026-10-05: a Clarifications session 2026-10-05 records the design freeze, the remote build host rule (FR-021), the owner-installed host entry point checkpoint (FR-019, FR-024) and the three plan-level owner answers of 2026-10-04 still pending confirmation; FR-017 and SC-009 carry a pending-amendment note (third-party package dependencies are report-only until the owner confirms the governance amendment of Known Conflicts item 16) and define "latest" as the latest stable release; SC-005 states one sample-size formula and one population; SC-011 counts a target of `none` as unmet, names its approver (ODG-32) and the target derivation rule (docs/14 §7) and drops the aspirational clause; FR-005 points to the docs/02 question set and pass rules; FR-012 and SC-006 gain a measurable review criterion and FR-012 flags the scope of document classes C and D as pending owner decision; FR-019 states that only the SC-010 end state is measured; FR-017/FR-018 and FR-020/FR-024 are de-duplicated by reference; Key Entities gain Review, Source entry and Owner decision; a Glossary and naming section defines "plan owner", the label namespaces and the layering rule, and the Assumptions state that this feature cuts no release tag unless the owner decides so at HC-7. No FR or SC id changed or was renumbered. Revision 8: owner decision recorded 2026-10-04, pending confirmation: "it all" includes package dependencies, so FR-017, SC-009, Open Questions Q1 and the Assumptions now require the package dependencies of every application to move to their latest versions whenever possible, after the tests pass, re-evaluated whenever new things enter the system, with impossible updates recorded as blockers; a fourth question is added to the Clarifications session 2026-10-04. Revision 7: three owner decisions recorded 2026-10-04, pending confirmation, in the Clarifications session 2026-10-04: FR-008 and SC-003 admit a named, tracked and ratcheted legacy class (headerless legacy documents) outside the zero-open count for that class only; FR-017 covers every submodule at every depth, own-organisation and vendored third-party, moving each pin to the latest upstream whenever possible after the tests pass and recording every move that is not possible as a blocker with its reason (third-party repositories themselves are never modified or pushed); an unavailable service, credential or device gives the status `blocked-unavailable`, distinct from a defect failure (the 2026-10-03 answer to the second question is reworded to match). Analysis fixes: the governance assumption aligned with FR-006; owner decisions resolved before the dependent work package rather than before planning; SC-011 and SC-005 made testable; FR-009 and FR-014 cross-referenced to the WP-23 applicability map and docs/13; the duplicated Created and Status lines below the table removed. Revision 6: the revision and last-modified lines are moved into this table, the §11.4.44 form that the revision-header check of tasks.md T040 reads; no requirement changed. Revision 5: adds this revision line and the last-modified date, constitution §11.4.44; no requirement changed. Revision 4 is commit `e811cbd0`, which added the "Plan documents" section; revisions 1 to 3 are commits `4e633fec`, `8aee842f` and `e4852ce7`, read with `git log -- spec.md`) |

**Feature Branch**: `001-full-project-audit-remediation` (no branch created; work continues on `main`)
**Input**: User description: "Do exhaustive analysis of the whole project. Rely heavily on the indexed structural code space (CodeGraph) to reduce token use as much as possible, always, and on the indexed semantic space (Lumen) as well. Detect any gap, misalignment, shortcoming, weak spot, danger zone, bug, error and issue, plus everything already known and documented as issues, workable items and tickets everywhere (HelixQA especially). Investigate systematically, fix and improve, and cover everything with all supported test types defined in the constitution. Every test, existing and new, must validate and verify fully deterministically through machine-produced results, never prediction. Update all documentation and exported files, add new documents properly linked from the main README, write user manuals, guides, FAQs, diagrams, graphs and schemes, and cover all definitions (SQL schemas, templates and others). The project consists of multiple applications (backends, services, APIs, web, mobile, desktop and other clients). All dependencies must always be up to date with their upstream codebases. Commit and push all work regularly, and confirm with `git status`, fully recursively, that nothing is uncommitted or unpushed to any upstream of any repository."

## Clarifications

### Session 2026-10-03

- Q: Which of the problems the audit finds must actually be fixed inside this feature, as opposed to only being recorded and tracked? → A: Every finding of every severity is fixed and proven before the feature is done.
- Q: When a test needs something outside our control (a live external service, an API key, or a physical device), what counts as acceptable evidence if it is unavailable during a run? → A: The test must run against the real service or device every time; if it is unavailable, the test does not pass and the feature is not done. (Wording amended 2026-10-04: the run is reported with the status `blocked-unavailable`, not as a defect failure; see the 2026-10-04 session and FR-025.)
- Q: Should this feature change the shared modules that other projects also use (the governance constitution submodule and the reusable libraries), or only audit them and report findings? → A: Fix everything everywhere, including the governance submodule and all shared modules, and push their fixes to their own upstreams.

### Session 2026-10-04

Each answer below is an owner decision recorded 2026-10-04, pending confirmation.

- Q: Legacy documents without a revision header cannot all be brought to the header form inside this feature; how do they count against "zero findings open"? → A: They form one named, tracked and ratcheted legacy class, "headerless legacy documents". Its count is recorded as a baseline and may never grow; documents leave it as they are fixed. Only that class is outside the zero-open count, and only for that class (FR-008, SC-003).
- Q: Which submodules does FR-017 bring to their latest upstream? → A: All of them, at every depth, own-organisation and vendored third-party alike (owner, verbatim: "We shall aim for the latest versions of it all if and when it is possible and changes applied when new things are brought in into the System!"). Each pin moves to the latest upstream whenever possible, applied only after the affected tests pass, and the move is repeated whenever new upstream content or new dependencies enter the system. A move that is not possible (incompatible, blocked, or upstream unavailable) is a recorded blocker with its reason, never silently skipped. Third-party repositories themselves are never modified or pushed by us; only our pointer to them moves (FR-017).
- Q: Does "it all" also cover package dependencies? → A: Yes. The owner's original requirement is that all dependencies are always up to date with the latest upstream, so the package dependencies of every application (Go, npm, Gradle and Maven, Cargo and others) follow the same rule as submodules: latest version whenever possible, applied after the tests pass, re-evaluated whenever new things enter the system, and every impossible update recorded as a blocker with its reason (FR-017, SC-009).
- Q: When a test's external service, credential or device is unavailable, what status does the run get? → A: `blocked-unavailable`, a status distinct from a defect failure. It counts as not passing, is never a pass or a skip, and is never replaced by a simulation (FR-025).

### Session 2026-10-05

Recorded in the analyze-remediation round the owner approved on 2026-10-05. Items marked pending owner confirmation keep the stricter reading until the owner confirms or changes them.

- Status of earlier answers: the four answers of the 2026-10-04 session above remain pending confirmation, and so do the three plan-level owner answers of 2026-10-04 recorded in the master plan ([docs/21](docs/21-master-plan-phases-risks-and-traceability.md) section 8): OA-C1 (every build runs on a remote build host, ODG-07), OA-C2 (supply-chain build level L2 at minimum, ODG-16) and the FR-017 answer. None of them is treated as confirmed by this session.
- Design freeze: the trust design of the commit-and-push launcher is frozen at its twelve central decisions (docs/21 section 12.22, 2026-10-05); no specification round reworks it, and a defect found in it later is handled in implementation, test-first, like any other finding. This changes no requirement of this specification.
- Remote builds (FR-021): following OA-C1, every build of a deliverable runs in a rootless container on a designated remote build host, never on the bare host and never only in a local container, and its artifacts are brought back and verified by checksum before use. The identity of the remote build host is an owner input that is still open (ODG-07).
- Host entry point (FR-019, FR-024): following the owner decision of 2026-10-05, from its adoption onward the commit-and-push launcher is started through a host entry point that the owner installs and approves personally at an owner checkpoint; no agent installs, alters or approves it.
- Third-party package dependencies (FR-017, SC-009): the project governance (Known Conflicts and Open Decisions item 16 of `.specify/memory/constitution.md`, operator decision of 2026-10-03) says third-party package dependencies are reported, not bulk updated. The 2026-10-04 answer that widens FR-017 to them takes effect only after the owner confirms it and the governance is amended through its amendment procedure; until then they are report-only (pending owner confirmation).
- Other items pending owner confirmation in this revision: the SC-005 sample formula and population, the SC-011 rule that a target of `none` is unmet, the scope of document classes C and D under FR-012, the definition of "latest" in FR-017, the end-state-only reading of FR-019, the meaning of "plan owner" (Glossary and naming) and whether a release tag is cut at HC-7 (Assumptions).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - One trustworthy register of every known and discovered problem (Priority: P1)

A project owner opens a single register and sees every problem anyone has ever recorded or that the audit finds: bugs, errors, gaps, misalignments, shortcomings, weak spots and danger zones. Items already documented in trackers, reports, QA banks and tickets (HelixQA in particular) are all present exactly once, each with a status, a type, a stable identifier and a clear description.

**Why this priority**: Nothing can be fixed, proven or reported complete until the full set of problems is known and nothing is lost between trackers. This is the foundation of every other story.

**Independent Test**: Reconcile the register against every existing source of problems (tracker documents, the workable-items database, HelixQA banks and tickets, status reports, open conflicts in the constitution). Every source entry maps to exactly one register entry, and the reconciliation report is machine-produced.

**Acceptance Scenarios**:

1. **Given** problems recorded in several places (documents, database, QA banks, tickets), **When** the register is built, **Then** each problem appears once, with status, type, identifier and description, and the mapping from every source entry is listed.
2. **Given** a problem that returns after being closed, **When** it is recorded again, **Then** the original item is reopened rather than a new item created.
3. **Given** the register and the external trackers, **When** they are compared, **Then** they agree, and any tracker that cannot be reached is reported as skipped with its reason, never silently omitted.

---

### User Story 2 - An exhaustive, evidence-backed audit of the whole project (Priority: P1)

A reviewer receives an audit covering every application and shared component in the project. Each finding states what is wrong, where, how severe it is, and the machine-produced evidence that it is real. The audit relies on the project's structural and semantic code indexes first, and proves that those indexes are complete and current before relying on them.

**Why this priority**: The owner's goal is that nothing important is missed. A partial or unevidenced audit gives false confidence.

**Independent Test**: Run the audit twice from the same state and compare. The set of findings and their evidence are identical. Then inject a known defect into a scratch copy and confirm the audit reports it.

**Acceptance Scenarios**:

1. **Given** the project's code indexes, **When** the audit begins, **Then** it first reports whether each index covers every in-scope file, is current, and answers known test questions correctly, and it refuses to continue on an index that fails these checks.
2. **Given** every application (backends, services, APIs, web, mobile, desktop, TV, installer and shared libraries), **When** the audit completes, **Then** each has a recorded result for gaps, misalignments, shortcomings, weak spots, danger zones, bugs and errors, including "none found" with the evidence for that claim.
3. **Given** a finding, **When** a reviewer reads it, **Then** it includes the location, severity, reproduction or proof, and a link to the register item.

---

### User Story 3 - Every fix proven by deterministic machine-produced results (Priority: P1)

For each finding, the project owner sees the problem reproduced before the fix, the fix applied, and the same check passing afterwards, with machine-produced evidence at both points. Every kind of test the constitution requires covers the changed behaviour, and each test gives the same result on every run, relying on measured results and never on prediction.

**Why this priority**: The project's prime directive is that passing tests must mean the feature works for the end user. Unproven fixes repeat the failure the project was founded to prevent.

**Independent Test**: For a sample of fixed items, check out the pre-fix state, run the item's test and see it fail, check out the fixed state, run it and see it pass, and repeat three times with identical results.

**Acceptance Scenarios**:

1. **Given** a finding, **When** its fix is accepted, **Then** a test that failed before the fix passes after it, with both runs recorded as machine-readable evidence.
2. **Given** a test, **When** it is run repeatedly, **Then** it returns the same verdict every time, and a flaky test is quarantined with an owner and a deadline instead of being retried until green.
3. **Given** a test that claims to prove behaviour, **When** the behaviour is deliberately broken, **Then** the test fails, demonstrating that it can detect the defect.
4. **Given** the full set of test types the constitution defines, **When** coverage is reported, **Then** every application shows which types exist, which are missing, and a tracked item for each missing one.

---

### User Story 4 - Complete, current and reachable documentation (Priority: P2)

A user, administrator or developer starts at the main README and can reach every document in the project through links. Existing documents are brought up to date, exported copies match their sources, and new user manuals, guides, FAQs, diagrams, graphs and schemes exist for every application and service. Definitions such as SQL schemas and templates are documented and match what the system actually uses.

**Why this priority**: Documentation that is stale, missing or unreachable undermines users and creates new defects, but it can follow once the problems are known.

**Independent Test**: Crawl the links from the main README and list every document in the project. Every in-scope document is reachable, every exported copy matches its source, and every documented schema matches the live definition.

**Acceptance Scenarios**:

1. **Given** the main README, **When** its links are followed transitively, **Then** every in-scope document is reachable, and any orphan is listed.
2. **Given** a source document and its exported copies, **When** they are compared, **Then** they agree, and a stale copy is reported.
3. **Given** each application and service, **When** its documentation is checked, **Then** it has a user manual, task-oriented guides, a FAQ and the relevant diagrams, and every diagram is rendered, non-blank and embedded where it is used.
4. **Given** the documented SQL schemas, templates and other definitions, **When** compared with the definitions the system uses, **Then** they match.

---

### User Story 5 - Every application is covered and kept consistent (Priority: P2)

The owner can see, for each application (backend API, services, web client, desktop client, installer, phone and tablet client, TV client, shared libraries, website and build framework), what was audited, what was fixed, which tests protect it and which documents describe it, and can confirm that the applications agree with each other where they share contracts.

**Why this priority**: The project is many applications, and a defect often lives in the seam between them.

**Independent Test**: Produce a per-application coverage matrix and confirm that each shared contract (for example, between the backend and each client) has tests on both sides.

**Acceptance Scenarios**:

1. **Given** the list of applications, **When** the coverage matrix is produced, **Then** each row shows audit status, open and closed findings, test types present, and documentation present.
2. **Given** a contract shared by a backend and its clients, **When** it changes, **Then** tests on both sides detect an incompatible change before release.

---

### User Story 6 - Dependencies always current with their upstreams (Priority: P2)

The owner can see, for every dependency (including shared modules maintained in separate repositories), the version in use, the latest upstream version, and whether they match. Differences are either resolved or explained by a recorded decision.

**Why this priority**: Out-of-date dependencies carry security and compatibility risk, but updating blindly can break the system, so it follows the audit and test foundation.

**Independent Test**: Run the dependency comparison and confirm that every dependency is listed with its version, its upstream version and a status.

**Acceptance Scenarios**:

1. **Given** all dependencies, **When** compared with their upstreams, **Then** the report lists each with current and latest versions and a status of current, behind with a recorded reason, or updated.
2. **Given** an update, **When** applied, **Then** the full set of tests for the affected applications passes before the update is accepted.

---

### User Story 7 - Nothing left uncommitted or unpushed anywhere (Priority: P3)

At any point, the owner can run one deterministic check and see that the main repository and every submodule, at every depth, has nothing uncommitted and nothing unpushed to any of its upstream hosts, using the version-control status as the proof. Work is committed and pushed regularly throughout, not only at the end.

**Why this priority**: It protects all other work from loss, but it is mechanical once the work exists.

**Independent Test**: Run the recursive verification. It reports every repository, its working-tree state, and, for each upstream host, whether the remote branch tip equals the local one.

**Acceptance Scenarios**:

1. **Given** the main repository and all submodules, **When** the verification runs, **Then** each is listed with a clean working tree and matching tips on every upstream, or with the exact reason it is not.
2. **Given** a repository that cannot be made clean or pushed (for example, a third-party repository we do not own), **When** the verification runs, **Then** it is reported with the reason and is not silently ignored.

---

### Edge Cases

- A finding turns out to be a false positive: it is closed with the evidence that disproves it, never deleted.
- A finding cannot be fixed without a decision only the owner can make: the feature is not complete until the owner decides, so the finding is recorded as blocked with the owner's choices and what it blocks, and it is raised to the owner rather than guessed.
- An index answers a question wrongly or is out of date: the audit stops relying on it for that class of question, falls back to direct reading, and records the gap.
- An external service, credential or device needed by a test is unavailable: the run is reported as `blocked-unavailable` with what is missing (distinct from a defect failure), it counts as not passing, and it is never skipped as a pass or replaced by a simulation.
- A fix in one application breaks another application sharing a contract: the contract tests on both sides catch it before acceptance.
- A submodule update breaks an application: the break is a finding, it is fixed before the update is accepted, and it is never applied silently.
- A tracker or upstream host is unreachable: the item is recorded as skipped with the reason, and the missing sync is itself tracked.
- A third-party or vendored repository cannot be made clean by us: it is reported as an accepted exception with its reason.
- Two sources describe the same problem with different severities: the register keeps both views, links them, and records which severity governs.
- A document is generated from a source: the source and the generated copy are checked together, so editing one cannot leave the other stale.

## Open Questions

| # | Question | Status | Resolution |
|---|----------|--------|------------|
| Q1 | How current must dependencies be? | Resolved | Only submodules: fetch and pull the latest codebases from all of their upstreams (FR-017; widened 2026-10-04 to every submodule at every depth, own-organisation and vendored third-party, pending confirmation). Superseded 2026-10-04 for package dependencies (pending confirmation): they too are moved to their latest versions whenever possible after the tests pass, with impossible updates recorded as blockers (FR-017). Until the owner confirms that answer and the governance is amended (Clarifications 2026-10-05), third-party package dependencies are report-only. |
| Q2 | Is the latency target of the adopted external constitution an acceptance criterion? | Resolved | No. It does not bind this project. Catalogizer sets its own per-operation targets from measured baselines (SC-011). |
| Q3 | How is the minimum code-coverage floor applied to existing code? | Resolved | Per-application phase-in (FR-011). |

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST have a single register of all problems (bugs, errors, gaps, misalignments, shortcomings, weak spots, danger zones), in which each item has a status, a type, a stable identifier and a comprehensive description.
- **FR-002**: The register MUST include every problem already recorded in any tracker, document, report, QA bank (HelixQA in particular), ticket or constitution conflict list, mapped from each source entry, with nothing dropped.
- **FR-003**: A problem that recurs MUST reopen its original item and MUST NOT create a new one.
- **FR-004**: The register MUST be kept in sync with every configured external tracker, and an unreachable tracker MUST be reported as skipped with its reason, never as synced.
- **FR-005**: The audit MUST use the project's structural and semantic code indexes as the first route for understanding the code, and MUST first demonstrate that each index is complete for the in-scope files, current, and correct on known test questions. The known test questions are the pre-declared fixed question set of [docs/02](docs/02-audit-methodology-and-index-strategy.md) §4.4, written before the first audit run and never edited to fit results; an index is correct when it meets the pass rules of the docs/02 §4.1 proof table (every question returns its recorded answer, and the deliberately unanswerable question returns nothing) and the thresholds of docs/02 decision D-02.
- **FR-006**: The audit MUST cover every application and shared component: backend, services, APIs, web, desktop, mobile, TV, installer, shared libraries, website, build framework and the shared governance module. Findings in shared modules MUST be fixed in those modules and pushed to their own upstreams, with the same independent review and evidence as any other fix.
- **FR-007**: Every finding MUST state its location, severity, category and machine-produced evidence, and MUST link to its register item.
- **FR-008**: Every finding, of every severity, MUST be investigated to a root cause before a fix is applied, and every fix MUST be accompanied by a test that fails before the fix and passes after it. The feature is not done while any finding is open. A finding may be closed without a fix only with evidence that it is a false positive or is structurally impossible to fix, never because it is low severity. A finding that is blocked waiting for an owner decision, a credential, a device or a missing service counts as open, so completion waits for the owner. A finding located only in vendored third-party code that the owner does not maintain is out of scope for changes: it is closed as an accepted exception with its reason and reported to that code's upstream, and it is excluded from the zero-open count. One named legacy class, "headerless legacy documents" (existing documents that lack the required revision header), is tracked as a single register item with a recorded baseline count that is ratcheted: the count may never grow, and it falls as documents are fixed. Findings of that class only are outside the zero-open count while the class item stays open; no other finding may be placed in it, and a new document without a header is an ordinary open finding (owner decision recorded 2026-10-04, pending confirmation).
- **FR-009**: Every test type the constitution defines MUST exist for every application where it applies. An absent type is itself a finding and MUST be fixed by writing it; it is not closed by recording a plan. Where a type applies is decided by the per-application applicability map of work package WP-23 ([docs/05](docs/05-test-strategy-and-coverage-matrix.md), [docs/21](docs/21-master-plan-phases-risks-and-traceability.md)), in which every cell is either applicable or not applicable with a recorded reason, and no cell is left undecided.
- **FR-010**: Every test, existing and new, MUST produce the same verdict on every run, MUST rely on measured and recorded results rather than prediction, and MUST be shown to fail when the behaviour it protects is deliberately broken.
- **FR-011**: A minimum code-coverage floor MUST apply to executable code, as a necessary and never sufficient measure of test quality. It is adopted per application in phases: each application has a recorded coverage baseline and a dated target, and no application may fall below its recorded baseline at any time. During the phase-in the 85% floor of the project's governance gates new and changed code in full, and existing code is held to its recorded baseline and dated target.
- **FR-012**: Every existing document MUST be reviewed and updated to match the current system, and every exported copy MUST match its source. A document counts as reviewed when it has a recorded review verdict against a named source-of-truth check (the code, configuration or definition the document describes), and it counts as matching when that check records zero open differences between the document's claims and the system. Whether the document classes C and D of the documentation programme ([docs/13](docs/13-documentation-program-plan.md) §3: class C, the generated record collections such as the QA ticket files; class D, the governance and agent files) need this full claim review or only a basis record is an owner decision, pending owner confirmation; until the owner decides, this requirement is read in full for every class, and a lighter treatment of classes C and D is a recorded deviation.
- **FR-013**: Every in-scope document MUST be reachable by links starting from the main README, and any orphan MUST be listed and resolved.
- **FR-014**: Each application and service MUST have a user manual, task-oriented guides, a FAQ and the relevant architecture, data-flow, state-machine and sequence diagrams, and each diagram MUST be rendered, non-blank and embedded where it is used. Which diagram classes are relevant to each application and service is decided by the documentation programme ([docs/13](docs/13-documentation-program-plan.md)), aligned with the WP-23 applicability map, with every exclusion recorded with its reason.
- **FR-015**: Definitions, including SQL schemas, templates and other formal definitions, MUST be documented and MUST match the definitions the system actually uses.
- **FR-016**: Shared contracts between applications MUST be tested on both sides so that an incompatible change is detected before release.
- **FR-017**: Every submodule at every depth, both own-organisation modules and vendored third-party repositories, MUST be fetched from all of its upstreams, and its pin MUST be moved to the latest upstream commit whenever that is possible, the move being accepted only under FR-018. The move MUST be repeated whenever new upstream content or a new dependency enters the system, not only once. A move that is not possible (incompatible, blocked, or the upstream unavailable) MUST be recorded as a blocker with its reason and is never silently skipped. Each submodule MUST be reported with its pinned commit, its latest upstream commit and a status. Third-party repositories themselves are never modified or pushed by this feature; only the project's pointer to them moves. The same rule applies to the package dependencies of every application (for example Go modules, npm packages, Gradle and Maven artifacts, Cargo crates): each is moved to its latest upstream version whenever that is possible, accepted only under FR-018, re-evaluated whenever new upstream releases or new dependencies enter the system, and an update that is not possible (incompatible, blocked, or the upstream unavailable) is recorded as a blocker with its reason, never silently skipped. Each package dependency is reported with its current version, its latest upstream version and a status (owner decisions recorded 2026-10-04, pending confirmation). "Latest upstream version" means the latest stable release; pre-releases are excluded unless the owner says otherwise (pending owner confirmation). Pending amendment: the project governance records the operator decision of 2026-10-03 that third-party package dependencies are reported, not bulk updated (Known Conflicts and Open Decisions item 16 of `.specify/memory/constitution.md`). Until the owner confirms the 2026-10-04 answer and that item is amended through the governance amendment procedure, third-party package dependencies are report-only: each is reported with its current version, its latest stable version and a status, and none is moved by this feature (Clarifications 2026-10-05).
- **FR-018**: An accepted dependency update (a submodule pin move or a package update under FR-017) MUST pass the full tests of every affected application first.
- **FR-019**: All work MUST be committed and pushed regularly, and a recursive verification MUST show, using version-control status and remote comparison, that the main repository and every submodule at every depth has nothing uncommitted and nothing unpushed to any upstream. "Regularly" is not a separately measured cadence: the measured requirement is the end state of SC-010 (pending owner confirmation; a cadence the owner sets would be added here). From the launcher's adoption onward, commits and pushes go through the project's commit-and-push launcher, started through a host entry point that the owner installs and approves personally at an owner checkpoint; no agent installs, alters or approves that entry point (owner decision of 2026-10-05, Clarifications 2026-10-05); how commits made before that adoption are routed is stated in the plan.
- **FR-020**: History MUST never be rewritten and nothing may be force-pushed, and any repository that cannot be made clean or pushed MUST be reported with its reason. This is the single statement of the no-rewrite and no-force-push rule; other requirements refer to it.
- **FR-021**: Every build of a deliverable MUST run in a rootless container on a designated remote build host, never on the bare host and never only in a local container; its artifacts MUST be brought back and verified by checksum before use, and the resulting artifact MUST be verified on a clean target before a fix is called done. The identity of the remote build host is an owner input that is still open (owner answer OA-C1 of 2026-10-04, pending confirmation; ODG-07; Clarifications 2026-10-05).
- **FR-022**: Every completion claim MUST cite machine-produced evidence from the current work, and an unverified claim MUST be labelled as unconfirmed.
- **FR-023**: The audit and its evidence MUST be independently reviewed by a reviewer separate from the author before acceptance, iterating until no blocking finding remains.
- **FR-024**: All work MUST be done on the main branch of the main repository and on the main branch of every submodule, with no separate feature, product or flavor branches created for this work. Integration remains fast-forward only, under FR-020, and commits follow FR-019, including its owner-installed host entry point once the launcher is adopted.
- **FR-025**: Any test of behaviour that depends on an external service, credential or physical device MUST run against the real one on every run. If it is unavailable, the run is reported with the status `blocked-unavailable` and the exact reason, a status distinct from a defect failure, which counts as not passing, is never treated as a pass or a skip, and is never replaced by a simulation; the feature is not complete until the owner supplies the missing service, credential or device.

### Key Entities

- **Finding**: a detected problem with location, severity, category, evidence and a link to its register item.
- **Register Item**: the tracked representation of a problem, with status, type, stable identifier, description and history of reopenings.
- **Application**: a deliverable unit (backend, service, client, library, website or build framework) with its audit status, coverage matrix row and documents.
- **Test Evidence Record**: a machine-readable record of a test run, with verdict, target identity, repetitions and the result before and after a fix.
- **Contract**: an interface shared between applications, with tests on both sides.
- **Dependency Record**: a dependency with the version in use, the upstream version, a status and a decision where they differ.
- **Document**: a unit of documentation with its source, exported copies, links and reachability from the main README.
- **Repository Verification Record**: for each repository, its working-tree state and, for each upstream, whether its tip matches.
- **Review**: an independent reviewer's recorded verdict on a change, a finding's evidence or a document (FR-012, FR-023), with the reviewer separate from the author, the reviewed item, the verdict and its blocking findings.
- **Source Entry**: one problem as recorded in an existing tracker, document, report, QA bank, ticket or conflict list, with its origin and the register item it maps to (FR-002, SC-001).
- **Owner Decision**: a question only the project owner can answer, with its options, the recorded answer and its date, whether it is confirmed or pending confirmation, and the work it blocks until answered.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of problems recorded in any existing tracker, report, QA bank or ticket appear in the register, and a machine-produced reconciliation lists each source entry with its register item.
- **SC-002**: 100% of applications and shared components have a recorded audit result, and the audit repeated from the same state yields an identical set of findings.
- **SC-003**: 100% of findings, of every severity, are either fixed or closed with evidence (false positive, structurally impossible, or an accepted exception in vendored third-party code), zero findings remain open at completion apart from the findings of the ratcheted "headerless legacy documents" class (FR-008), whose count at completion is at or below its recorded baseline and has never risen above it (owner decision recorded 2026-10-04, pending confirmation), and every fixed item has a machine-recorded failing run before the fix and a passing run after it that passes identically across 3 repeated runs.
- **SC-004**: For every test type the constitution defines, 100% of applications have it where it applies, and the coverage matrix shows zero gaps.
- **SC-005**: Zero tests are accepted that still pass after the behaviour they protect is deliberately broken, measured on a sample drawn by the reviewer, not the author. The sample size is the one the owner sets under decision ODG-17 (work package WP-71 of [docs/21](docs/21-master-plan-phases-risks-and-traceability.md)); until it is set, this criterion is the single statement of the formula and the population, and the research decision OD-21 and the reviewer sampling task (T571) cite it rather than restate it (pending owner confirmation). The population P is every test recorded in the evidence ledger, which includes every test that guards a fixed finding. The sample size is n = min(P, max(30, ceil(0.10 × P), 149)), where 149 = ceil(ln(0.05) / ln(0.98)) is the smallest sample that sees at least one weak test with probability 0.95 when 2% of the tests are weak; when P is at most n, every test is sampled. The sample is a random draw with a seed the reviewer chooses and records with the draw, so that the same seed reproduces the same sample, and each sampled test is broken by a mutation the reviewer writes.
- **SC-006**: 100% of in-scope documents are reachable from the main README by following links, zero exported copies differ from their sources, and 100% of existing documents in the FR-012 scope have a recorded review verdict against a named source-of-truth check with zero open differences between their claims and the system.
- **SC-007**: Every application and service has a user manual, guides, a FAQ and its diagrams, and 100% of diagrams render non-blank.
- **SC-008**: 100% of documented SQL schemas and templates match the definitions the system uses.
- **SC-009**: 100% of dependencies are reported with their version, the upstream version and a status; 100% of submodules at every depth (own-organisation and vendored third-party) are at their latest upstream commit at completion or carry a recorded blocker with its reason; and 100% of package dependencies of every application are at their latest upstream version (latest stable release, FR-017) at completion or carry a recorded blocker with its reason. Pending amendment (FR-017, Clarifications 2026-10-05): until the owner confirms the 2026-10-04 answer and the governance item is amended, the package clause for third-party package dependencies is measured on the report only (each listed with its current version, latest stable version and status), and no third-party package is moved.
- **SC-010**: A recursive repository check reports every repository (main and all submodules at all depths) with a clean working tree and matching tips on every upstream, with zero unexplained exceptions.
- **SC-011**: Every critical user-facing operation (browsing, search, playback start, scanning a source, sign-in, and each client's start-up) has a measured performance baseline and a documented per-operation target set for this project from that baseline (the baseline and target work of WP-38 and WP-62 in [docs/21](docs/21-master-plan-phases-risks-and-traceability.md), [docs/14](docs/14-performance-engineering-plan.md)). Each target is derived from the measured baseline by the target-setting method of docs/14 §7 (its formula in §7.2 and decision rules in §7.3), never invented, and is approved by the owner under decision ODG-32. The criterion is met when, on the final candidate, every operation's measured result meets its recorded target and none is worse than its baseline beyond the measurement-noise bound recorded with that baseline, and every identified bottleneck is fixed with before-and-after measurements. An operation whose recorded target is `none` leaves this criterion unmet, and an operation without a measured baseline also leaves it unmet (pending owner confirmation).
- **SC-012**: Zero completion claims in the final report lack a cited machine-produced evidence record from the current work.

## Assumptions

- The project owner is the sole approver of scope decisions, and each decision left open here is resolved before the work package that depends on it starts (the decision-to-work-package map is in [docs/21](docs/21-master-plan-phases-risks-and-traceability.md)), not necessarily before planning.
- Existing registers (the workable-items database, tracker documents, HelixQA banks and tickets, and the constitution's conflict list) are the authoritative starting inventory; no source is assumed complete.
- Third-party repositories vendored inside the project are in scope for status reporting and for moving the project's pin to their latest upstream (FR-017), but the repositories themselves are not modified or pushed.
- The project's existing governance (the constitution and its appendix) defines the supported test types, the evidence standard and the review rules, and this feature applies them. Consistent with FR-006, a defect found in the shared governance module is fixed in that module and pushed to its own upstream like any other finding, through the module's own amendment and review process; this feature does not weaken or bypass any governance rule to make its own work pass.
- The structural and semantic code indexes exist or can be built in a rootless container, and their health is verified as part of the work.
- The work is delivered in priority order (P1 first), and each story can be accepted independently.
- The owner's instruction that all work happens on main branches is a decision for this feature and overrides the default branch-per-feature convention; every commit still goes through review and is pushed to every upstream.
- Pulling the latest governance submodule includes running its post-pull validation sweep and registration hook, because the project's governance requires them after every pull.
- Vendored third-party repositories are never changed or pushed by this feature; the project's pointer to each of them moves to its latest upstream whenever possible after the tests pass, and a move that is not possible is a recorded blocker with its reason (FR-017; owner decision recorded 2026-10-04, pending confirmation). Modules the owner maintains, including shared ones other projects use, are in scope for fixes.
- The latency target from the adopted external constitution is not a criterion here, because the owner decided it applies to a different product.
- Findings that require operator decisions are recorded as blocked items with their choices rather than guessed.
- The owner supplies the external credentials and physical devices that real-service and real-device tests need, and tells us where they are (see FR-025). The owner's rule that an unavailable dependency means the test cannot pass is stricter than the governance's allowance of an honest skip with a reason; the `blocked-unavailable` status in FR-025 reports the exact reason, so it does not mislabel an infrastructure gap as a defect failure.
- Package dependencies of every application are moved to their latest upstream versions whenever possible, after the tests pass, and re-evaluated whenever new things enter the system (FR-017). An outdated package without a recorded blocker is an open finding; one whose update is not possible is a recorded blocker with its reason, which counts as open in the zero-open count until it is resolved or closed with evidence that the update is structurally impossible (FR-008). For third-party package dependencies this applies only after the pending governance amendment of FR-017; until then they are report-only.
- This feature cuts no release tag. Whether a release tag is cut at the final acceptance (HC-7 of [docs/21](docs/21-master-plan-phases-risks-and-traceability.md)) is an owner decision taken there (pending owner confirmation).

## Glossary and naming

- **Project owner** (also "owner"): the single person who approves scope and answers owner decisions (Assumptions).
- **Plan owner**: used in the plan documents for the project owner acting in the planning role, deciding how the plan is written and checked. It names the same person as "project owner"; no text of this feature is read as naming a different person, and a plan-owner decision is an owner decision recorded like any other (UNCONFIRMED: no document states who the plan owner is; this reading is pending owner confirmation).
- **Owner decision**: see Key Entities. Grouped decisions are numbered ODG-nn and the ungrouped research decisions OD-nn (docs/21 section 8).
- **Label namespaces**: several unrelated label families use the letter C with a number. The owner answers of 2026-10-04 are written OA-C1 and OA-C2 (OA-C1 the remote build host, OA-C2 the supply-chain build level); the twelve frozen central decisions of the commit-and-push launcher are written CENTRAL C1 to CENTRAL C12 (namespace CENTRAL-Cn, docs/21 section 12.22); the conversion classes of the QA bank conversion are written CONV-C1 to CONV-C3 (UNCONFIRMED: the defining location of that third family was not located while writing this revision; the docs/12 escape-ratchet fixture cycles also named C1 to C3 are a fourth, local use and keep their own wording). Where an older text says only "C1" or "C2", the context decides which family is meant, and in this specification "C1" and "C2" without a prefix are not used.

## Plan documents

This section only points to the documents derived from this specification; it adds and changes no requirement.

Layering rule: this specification states requirements and acceptance criteria; the plan documents map them to phases, work packages, tasks and owner-decision numbers. Where a requirement here cites a work package (for example WP-23 or WP-71) or a decision number, the citation is a pointer for the reader; the rule stays in this specification, and a change of the mapping is made in the plan, not here.

- [README.md](README.md): index of every document of this specification folder, grouped.
- [plan.md](plan.md): implementation plan and constitution check.
- [docs/21-master-plan-phases-risks-and-traceability.md](docs/21-master-plan-phases-risks-and-traceability.md): master plan with phases, work packages, risks, owner decisions and the requirement traceability matrix.
- [tasks.md](tasks.md): task list by phase and work package.
- [research.md](research.md), [data-model.md](data-model.md), [quickstart.md](quickstart.md), [contracts/README.md](contracts/README.md): design artifacts.
- [checklists/requirements.md](checklists/requirements.md): specification quality checklist.

## Brainstorm Log

<!--
  This section records insights from /speckit.superspec.brainstorm sessions.
  Each entry is dated and summarizes what was discovered and decided.
  Do not edit manually — this is maintained by the brainstorm command.
-->
