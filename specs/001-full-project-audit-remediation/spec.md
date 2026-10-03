# Feature Specification: Full Project Audit and Remediation

**Feature Branch**: `001-full-project-audit-remediation` (no branch created; work continues on `main`)
**Created**: 2026-10-03
**Status**: Draft
**Input**: User description: "Do exhaustive analysis of the whole project. Rely heavily on the indexed structural code space (CodeGraph) to reduce token use as much as possible, always, and on the indexed semantic space (Lumen) as well. Detect any gap, misalignment, shortcoming, weak spot, danger zone, bug, error and issue, plus everything already known and documented as issues, workable items and tickets everywhere (HelixQA especially). Investigate systematically, fix and improve, and cover everything with all supported test types defined in the constitution. Every test, existing and new, must validate and verify fully deterministically through machine-produced results, never prediction. Update all documentation and exported files, add new documents properly linked from the main README, write user manuals, guides, FAQs, diagrams, graphs and schemes, and cover all definitions (SQL schemas, templates and others). The project consists of multiple applications (backends, services, APIs, web, mobile, desktop and other clients). All dependencies must always be up to date with their upstream codebases. Commit and push all work regularly, and confirm with `git status`, fully recursively, that nothing is uncommitted or unpushed to any upstream of any repository."

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
- An index answers a question wrongly or is out of date: the audit stops relying on it for that class of question, falls back to direct reading, and records the gap.
- A fix in one application breaks another application sharing a contract: the contract tests on both sides catch it before acceptance.
- A dependency update requires a breaking change: it is recorded with the decision and the affected applications, not applied silently.
- A tracker or upstream host is unreachable: the item is recorded as skipped with the reason, and the missing sync is itself tracked.
- A third-party or vendored repository cannot be made clean by us: it is reported as an accepted exception with its reason.
- Two sources describe the same problem with different severities: the register keeps both views, links them, and records which severity governs.
- A document is generated from a source: the source and the generated copy are checked together, so editing one cannot leave the other stale.

## Open Questions

| # | Question | Status | Resolution |
|---|----------|--------|------------|
| Q1 | How current must dependencies be? | Resolved | Only submodules: fetch and pull the latest codebases from all of their upstreams (FR-017). Third-party package dependencies are reported but not bulk-updated in this feature. |
| Q2 | Is the latency target of the adopted external constitution an acceptance criterion? | Resolved | No. It does not bind this project. Catalogizer sets its own performance targets and aims for the best achievable performance (SC-011). |
| Q3 | How is the minimum code-coverage floor applied to existing code? | Resolved | Per-application phase-in (FR-011). |

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The project MUST have a single register of all problems (bugs, errors, gaps, misalignments, shortcomings, weak spots, danger zones), in which each item has a status, a type, a stable identifier and a comprehensive description.
- **FR-002**: The register MUST include every problem already recorded in any tracker, document, report, QA bank (HelixQA in particular), ticket or constitution conflict list, mapped from each source entry, with nothing dropped.
- **FR-003**: A problem that recurs MUST reopen its original item and MUST NOT create a new one.
- **FR-004**: The register MUST be kept in sync with every configured external tracker, and an unreachable tracker MUST be reported as skipped with its reason, never as synced.
- **FR-005**: The audit MUST use the project's structural and semantic code indexes as the first route for understanding the code, and MUST first demonstrate that each index is complete for the in-scope files, current, and correct on known test questions.
- **FR-006**: The audit MUST cover every application and shared component: backend, services, APIs, web, desktop, mobile, TV, installer, shared libraries, website and build framework.
- **FR-007**: Every finding MUST state its location, severity, category and machine-produced evidence, and MUST link to its register item.
- **FR-008**: Every finding MUST be investigated to a root cause before a fix is applied, and every fix MUST be accompanied by a test that fails before the fix and passes after it.
- **FR-009**: Every test type the constitution defines MUST exist for every application where it applies, and any absent type MUST be tracked as an item with a plan.
- **FR-010**: Every test, existing and new, MUST produce the same verdict on every run, MUST rely on measured and recorded results rather than prediction, and MUST be shown to fail when the behaviour it protects is deliberately broken.
- **FR-011**: A minimum code-coverage floor MUST apply to executable code, as a necessary and never sufficient measure of test quality. It is adopted per application in phases: each application has a recorded coverage baseline and a dated target, and no application may fall below its recorded baseline at any time. During the phase-in the 85% floor of the project's governance gates new and changed code in full, and existing code is held to its recorded baseline and dated target.
- **FR-012**: Every existing document MUST be reviewed and updated to match the current system, and every exported copy MUST match its source.
- **FR-013**: Every in-scope document MUST be reachable by links starting from the main README, and any orphan MUST be listed and resolved.
- **FR-014**: Each application and service MUST have a user manual, task-oriented guides, a FAQ and the relevant architecture, data-flow, state-machine and sequence diagrams, and each diagram MUST be rendered, non-blank and embedded where it is used.
- **FR-015**: Definitions, including SQL schemas, templates and other formal definitions, MUST be documented and MUST match the definitions the system actually uses.
- **FR-016**: Shared contracts between applications MUST be tested on both sides so that an incompatible change is detected before release.
- **FR-017**: Every submodule (each shared module maintained in a separate repository; "at every depth" is an inference from the owner's instruction) MUST be fetched from all of its upstreams and updated to the latest upstream codebase, and each MUST be reported with its pinned and latest upstream commit and a status. Third-party package dependencies are reported with their current and latest versions but are not bulk-updated by this feature (an inference from "only submodules").
- **FR-018**: An accepted dependency update MUST pass the full tests of every affected application first.
- **FR-019**: All work MUST be committed and pushed regularly, and a recursive verification MUST show, using version-control status and remote comparison, that the main repository and every submodule at every depth has nothing uncommitted and nothing unpushed to any upstream.
- **FR-020**: History MUST never be rewritten and nothing may be force-pushed, and any repository that cannot be made clean or pushed MUST be reported with its reason.
- **FR-021**: Every build of a deliverable MUST run in a rootless container, never on the bare host, and the resulting artifact MUST be verified on a clean target before a fix is called done.
- **FR-022**: Every completion claim MUST cite machine-produced evidence from the current work, and an unverified claim MUST be labelled as unconfirmed.
- **FR-023**: The audit and its evidence MUST be independently reviewed by a reviewer separate from the author before acceptance, iterating until no blocking finding remains.
- **FR-024**: All work MUST be done on the main branch of the main repository and on the main branch of every submodule, with no separate feature, product or flavor branches created for this work. Integration remains fast-forward only, with no history rewrite and no force-push.

### Key Entities

- **Finding**: a detected problem with location, severity, category, evidence and a link to its register item.
- **Register Item**: the tracked representation of a problem, with status, type, stable identifier, description and history of reopenings.
- **Application**: a deliverable unit (backend, service, client, library, website or build framework) with its audit status, coverage matrix row and documents.
- **Test Evidence Record**: a machine-readable record of a test run, with verdict, target identity, repetitions and the result before and after a fix.
- **Contract**: an interface shared between applications, with tests on both sides.
- **Dependency Record**: a dependency with the version in use, the upstream version, a status and a decision where they differ.
- **Document**: a unit of documentation with its source, exported copies, links and reachability from the main README.
- **Repository Verification Record**: for each repository, its working-tree state and, for each upstream, whether its tip matches.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of problems recorded in any existing tracker, report, QA bank or ticket appear in the register, and a machine-produced reconciliation lists each source entry with its register item.
- **SC-002**: 100% of applications and shared components have a recorded audit result, and the audit repeated from the same state yields an identical set of findings.
- **SC-003**: 100% of fixed items have a machine-recorded failing run before the fix and a passing run after it, and each passes identically across 3 repeated runs.
- **SC-004**: For every test type the constitution defines, 100% of applications either have it or have a tracked item with a plan, and the coverage matrix shows zero unexplained gaps.
- **SC-005**: Zero tests are accepted that still pass after the behaviour they protect is deliberately broken, measured on a sample drawn by the reviewer.
- **SC-006**: 100% of in-scope documents are reachable from the main README by following links, and zero exported copies differ from their sources.
- **SC-007**: Every application and service has a user manual, guides, a FAQ and its diagrams, and 100% of diagrams render non-blank.
- **SC-008**: 100% of documented SQL schemas and templates match the definitions the system uses.
- **SC-009**: 100% of dependencies are reported with their version, the upstream version and a status, and every dependency that is behind has a recorded decision.
- **SC-010**: A recursive repository check reports every repository (main and all submodules at all depths) with a clean working tree and matching tips on every upstream, with zero unexplained exceptions.
- **SC-011**: Every critical user-facing operation (browsing, search, playback start, scanning a source, sign-in, and each client's start-up) has a measured performance baseline and a documented target set for this project, no measured operation regresses against its baseline, and every identified bottleneck is either fixed with before-and-after measurements or tracked with a plan, aiming for the best achievable performance.
- **SC-012**: Zero completion claims in the final report lack a cited machine-produced evidence record from the current work.

## Assumptions

- The project owner is the sole approver of scope decisions, and decisions left open here are resolved before planning.
- Existing registers (the workable-items database, tracker documents, HelixQA banks and tickets, and the constitution's conflict list) are the authoritative starting inventory; no source is assumed complete.
- Third-party repositories vendored inside the project are in scope for status reporting but are not modified or pushed.
- The project's existing governance (the constitution and its appendix) defines the supported test types, the evidence standard and the review rules; this feature applies them and does not change them.
- The structural and semantic code indexes exist or can be built in a rootless container, and their health is verified as part of the work.
- The work is delivered in priority order (P1 first), and each story can be accepted independently.
- The owner's instruction that all work happens on main branches is a decision for this feature and overrides the default branch-per-feature convention; every commit still goes through review and is pushed to every upstream.
- Pulling the latest governance submodule includes running its post-pull validation sweep and registration hook, because the project's governance requires them after every pull.
- The latency target from the adopted external constitution is not a criterion here, because the owner decided it applies to a different product.
- Findings that require operator decisions are recorded as blocked items with their choices rather than guessed.

## Brainstorm Log

<!--
  This section records insights from /speckit.superspec.brainstorm sessions.
  Each entry is dated and summarizes what was discovered and decided.
  Do not edit manually — this is maintained by the brainstorm command.
-->
