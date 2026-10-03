# Implementation Plan: Full Project Audit and Remediation

**Branch**: `001-full-project-audit-remediation` (no branch; all work on `main`, per spec FR-024) | **Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/001-full-project-audit-remediation/spec.md`

## Summary

The spec asks for an exhaustive, evidence-backed audit of every Catalogizer application and shared component, a single register of every known and discovered problem, every finding fixed and proven by deterministic machine-produced results, complete and linked documentation, submodules brought to their latest upstream codebases, and a recursive proof that nothing is uncommitted or unpushed.

The technical approach, fixed by the 21 planning documents in [docs/](docs/) (about 239,000 words), is:

1. **Prove the instruments first.** Both code indexes fail their own health checks today (CodeGraph covers 7,150 files; Lumen reports 10,397 files and stale; no scope data file, no `.lumenignore`, no `.mcp.json`). A health gate must pass before an index is trusted ([docs/02](docs/02-audit-methodology-and-index-strategy.md)).
2. **Build the register and the evidence machinery before the first finding.** The register is the constitution's workable-items engine database plus an extension layer, not a second database ([docs/04](docs/04-findings-register-design.md)); evidence records, polarity runs and tamper-evident chaining follow [docs/06](docs/06-determinism-and-evidence-framework.md).
3. **Containerize before any build or test.** Several existing container assets cannot build from a clean checkout, and some scripts build on the bare host ([docs/16](docs/16-containerized-infrastructure-and-local-enforcement-plan.md)).
4. **Import everything already known** (about 4,000 candidate entries, led by 1,778 HelixQA tickets whose ids collide) and **audit every application twice from the same state** with independent review ([docs/03](docs/03-existing-issue-inventory.md), [07](docs/07-backend-catalog-api-audit-plan.md) to [10](docs/10-android-and-android-tv-audit-plan.md), [12](docs/12-helixqa-challenges-and-governance-plan.md)).
5. **Fix contracts before clients**, remediate by risk with RED-then-GREEN proof repeated three times, close the test-type matrix, convert the prose QA steps, run the documentation and performance programmes, update submodules per the runbook, then verify recursively and close ([docs/05](docs/05-test-strategy-and-coverage-matrix.md), [11](docs/11-submodules-audit-and-update-runbook.md), [13](docs/13-documentation-program-plan.md), [14](docs/14-performance-engineering-plan.md), [15](docs/15-security-and-danger-zone-plan.md)).

The master plan ([docs/21](docs/21-master-plan-phases-risks-and-traceability.md)) defines 8 phases, 53 work packages, 34 risks and 38 grouped owner decisions (ODG-01 to ODG-38; the finer-grained collection in [research.md](research.md) lists 79, OD-01 to OD-79), and maps every FR and SC to at least one work package and one evidence type (0 uncovered). Research with cited sources is in [docs/17](docs/17-research-engineering-practices.md), [18](docs/18-research-product-innovation-and-game-changers.md) and [20](docs/20-research-second-pass-gaps.md); working, self-tested proof-of-concept tools are in [poc/](poc/) and [docs/19](docs/19-poc-tools-and-results.md).

## Technical Context

**Language/Version**: Go 1.25.7 (`catalog-api`, module `catalogizer`); TypeScript (`catalog-web` uses TypeScript 4.9, `catalogizer-api-client` uses TypeScript 5: a version spread to reconcile); React 18; Rust edition 2021 (Tauri 2: `catalogizer-desktop`, `installer-wizard`); Kotlin (`catalogizer-android` compileSdk 35, `catalogizer-androidtv` compileSdk 34; JDK target conflict recorded in the constitution Known Conflicts item 13); Bash and Python 3 for tooling.
**Primary Dependencies**: Gin, JWT, `quic-go/http3`, Brotli, Prometheus client, dual-dialect SQL layer; React Query, Zustand (declared, reported unused by docs/08), Tailwind, React Hook Form, Zod, Vite 6; Tauri 2, tokio, reqwest; Jetpack Compose, Room 2.6.1, Retrofit 2.9.0, OkHttp 4.12, Media3, WorkManager; 44 direct submodules (97 repositories recursively).
**Storage**: SQLite (development; SQLCipher use is UNCONFIRMED per docs/20) and PostgreSQL (production) behind a dialect-rewriting wrapper; Room on Android; the register in a tracked SQLite database (`docs/workable_items.db`, to be created; `.gitignore:85` must be amended per docs/04).
**Testing**: `go test` with the race detector (containerized), Vitest and Playwright, Kotlin unit and instrumentation tests, `cargo test`, HelixQA banks and Challenges; mutation testing, contract testing and visual proof tools are to be instantiated (docs/05, docs/17, docs/20). Pass/fail baselines of every existing suite are UNKNOWN until the first containerized run.
**Target Platform**: Linux server (containers), browsers, Linux/macOS/Windows desktop (only Linux buildable on this host; others `blocked-unavailable` until the owner supplies hosts), Android phone and tablet, Android TV. Host: 16 CPU, about 30 GiB RAM, rootless Podman; most scanners absent, so every detector runs in a container.
**Project Type**: multi-application system (web service, web client, desktop client, installer, two mobile clients, shared libraries, website, build framework) with a governance submodule.
**Performance Goals**: no Catalogizer-specific numeric target is asserted by this plan. Per the owner (spec SC-011, Q2), the HelixPlay 30/50 ms SLA does not bind; targets are derived from measured baselines per docs/14, aiming for the best achievable performance with no regressions.
**Constraints**: all builds and tests in rootless containers (FR-021); host memory at most 60% of RAM (about 18 GiB), thread headroom checked before parallel fan-out; no force-push, no history rewrite; no CI/CD (enforcement is a local, staged commit/push validation script); all work on `main`; every change independently reviewed (Opus, `xhigh`); every completion claim backed by machine-produced evidence from the current work.

No `NEEDS CLARIFICATION` marker remains. What is not yet known is carried as `UNCONFIRMED`/`UNKNOWN` items in each document and as owner inputs in [research.md](research.md) (OD-01 to OD-79; 12 have a reversible safe default and do not block work).

## Constitution Check

*GATE: Must pass before proceeding. Re-check after design phase.*

Evaluated against `.specify/memory/constitution.md` v2.1.2 and its binding appendix.

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Anti-Bluff End-User Usability | PASS | Every claim carries machine-produced evidence (docs/06); guards are validated with golden-good, golden-bad and control needles; the POC tools were self-tested and run for real (docs/19). Unrun examples in the documents are labelled `NOT EXECUTED`. |
| II. Test-First on Real Systems | NEEDS ATTENTION | The plan mandates RED-first for all work and real dependencies in non-unit tests, but the current suites use mocks in non-unit tests (docs/05: mock servers, `page.route`, in-memory SQLite), mutation tooling is not instantiated, and Rust coverage tooling is absent. Closing these is plan work (docs/05, WP in P6), not a violation of the plan. |
| III. Evidence-Gated Independent Review | PASS | Every document and every phase has an independent Opus review gate (docs/21 §4.4). Effort cannot be set through the Agent tool; reviews record that limit. |
| IV. Containerized, Rootless, Distributed Builds | NEEDS ATTENTION | The plan puts all builds in rootless containers, but today four container assets cannot build from a clean checkout and `auto-container.sh` builds on the bare host when the tool exists (docs/16 D-01 to D-10). The remote build hosts were not contacted; their availability is an owner input. |
| V. Reuse-First, Decoupled Modules | PASS | Reuses the constitution's workable-items engine, `repo_verify`-style tooling, gitleaks/Trivy/ZAP wrappers and the Containers submodule rather than reimplementing (docs/04, 11, 16). Fixes in shared modules go to their own upstreams (spec FR-006). |
| VI. Absolute Data, History and Host Safety | PASS | No history rewrite or force-push anywhere in the plan; credential rotation is requested, never done by rewriting (docs/15). Hardlinked backups precede destructive steps. The constitution post-pull hook is gated on the owner's explicit go-ahead (docs/11). |
| VII. Continuity and Zero-Loss Traceability | PASS | Register, request ledger and continuation documents are in the plan; legacy tickets are keyed by file path because their ids collide (docs/03, docs/04). |
| VIII. Manual-QA-Final and an Honest Definition of Done | PASS | The plan keeps live manual QA as the final human gate even though work lands on `main` (constitution item 15), and treats unavailable devices and services as `blocked-unavailable`, never as a pass. |

Gate result: no unjustified violation. Two NEEDS ATTENTION items (II and IV) describe the starting state the plan is designed to change; they carry no exemption. Re-evaluation after design is in the section below.

### Post-design re-evaluation

The design artifacts ([research.md](research.md), [data-model.md](data-model.md), [contracts/](contracts/), [quickstart.md](quickstart.md)) introduce no new principle conflict. The decisions that needed a conflict resolution are listed under Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/001-full-project-audit-remediation/
├── spec.md                 # Feature specification (clarified)
├── plan.md                 # This file
├── research.md             # Phase 0: decisions, rationale, alternatives, owner inputs
├── data-model.md           # Phase 1: entities, relationships, state transitions
├── quickstart.md           # Phase 1: runnable validation guide
├── contracts/              # Phase 1: JSON schemas of machine-produced reports
├── checklists/requirements.md
├── docs/                   # 21 planning documents
│   ├── 01-system-architecture-map.md
│   ├── 02-audit-methodology-and-index-strategy.md
│   ├── 03-existing-issue-inventory.md
│   ├── 04-findings-register-design.md
│   ├── 05-test-strategy-and-coverage-matrix.md
│   ├── 06-determinism-and-evidence-framework.md
│   ├── 07..10  per-application audit plans (backend, web, desktop+installer, Android+TV)
│   ├── 11-submodules-audit-and-update-runbook.md
│   ├── 12-helixqa-challenges-and-governance-plan.md
│   ├── 13-documentation-program-plan.md
│   ├── 14-performance-engineering-plan.md
│   ├── 15-security-and-danger-zone-plan.md
│   ├── 16-containerized-infrastructure-and-local-enforcement-plan.md
│   ├── 17, 18, 20  web research (engineering practice, product innovation, second pass)
│   ├── 19-poc-tools-and-results.md
│   └── 21-master-plan-phases-risks-and-traceability.md
├── poc/                    # Working, self-tested read-only tools
│   ├── repo_verify/        # recursive repository verifier
│   ├── doc_links/          # link crawler
│   └── route_drift/        # route and contract drift extractor
└── tasks.md                # Task breakdown (next command, /speckit-superspec-tasks)
```

### Source Code (repository root)

This feature changes existing applications rather than adding a new one. Its work lands in the real layout:

```text
catalog-api/                  # Go API (handlers, services, repository, database, internal/*, challenges)
catalog-web/                  # React web client and e2e
catalogizer-desktop/          # Tauri desktop (src-tauri Rust + React)
installer-wizard/             # Tauri installer
catalogizer-android/          # Android phone/tablet
catalogizer-androidtv/        # Android TV
catalogizer-api-client/       # TypeScript API client
Website/                      # VitePress site
Build/ build-scripts/ scripts/ docker/ deployment/ config/ monitoring/   # build, ops and tooling
challenges/ docs/ database/ templates/ tests/                            # QA banks, documentation, schemas, tests
submodules/                   # 44 direct, 97 recursive repositories
.specify/memory/              # governance layer (constitution + appendix)
docs/workable_items.db        # the register (to be created and tracked)
```

**Structure Decision**: no new top-level application. New tooling is promoted from [poc/](poc/) into a tracked, tested location under `scripts/` during the work packages that need it; new shared contracts live next to their owners. The register is the constitution's engine database plus an extension layer (docs/04), and the planning documents stay under this specification directory.

## Execution Strategy

### TDD Requirements

- [ ] Register and evidence recorder (WP-05, WP-06): invariants (status custody, recurrence links, evidence class) are checked by RED-first tests with golden-bad fixtures.
- [ ] Every fixed finding (all applications): a test that fails before and passes after, repeated three times, with machine-recorded verdicts.
- [ ] Auth, SSRF and input-validation paths (backend WebSocket and image proxy; desktop `make_http_request`; Android exported components): negative-path tests with concrete hostile inputs.
- [ ] Verifier, link-crawler, route-drift and commit/push script: promoted from the POCs with their golden-good, golden-bad and control tests retained.
- [ ] Migrations and dialect rewriting (both SQLite and PostgreSQL): tests on a real database of each dialect.

### Parallel Execution Opportunities

- [ ] Streams ST-API, ST-WEB, ST-DESK, ST-AND, ST-SUB and ST-SEC audit disjoint components and can run as parallel subagents after P2, bounded by six concurrent agents, 60% memory and thread headroom (docs/21 §4.2).
- [ ] ST-DOC and ST-PERF can start as soon as their inputs exist (docs/21 §3.1), overlapping P5 remediation.
- [ ] Per-application client fixes (P5) wait for the live route dump and contract tests (P4), but may run while another application is still in P3.
- [ ] Single-owner resources serialise: one stream per physical device, build host and the register database.

### Human Checkpoints

1. After the owner decision intake (WP-01): confirm credentials, devices, build hosts, legacy-ticket policy and the constitution hook variant.
2. After foundation (P0) and infrastructure (P1): confirm the verifier, the commit/push script and the container runners work on a clean checkout.
3. After the audit pass (P3): review the first register totals and the independent review's verdict before any fix.
4. After each remediation wave: confirm behaviour against the acceptance scenarios and run the affected suites.
5. Before final closure (P7): full retest on the candidate artifact, recursive repository verification, and the owner's live manual QA.

### Review Gates

- [ ] Constitution `Known Conflicts` decisions and any governance amendment: independent Opus review before acceptance.
- [ ] Security-sensitive code (auth, SSRF, credentials, exported Android components, IPC): review before integration.
- [ ] Data-model and migration changes: review before any migration runs.
- [ ] API contract and schema changes: review before any consumer is changed.
- [ ] Every documentation and export batch: independent review plus the link, export-sync and schema-diff reports.

## Complexity Tracking

> No principle is violated. The items below are conflicts between sources that the plan resolves, each with its reason.

| Conflict | Why Needed | Simpler Alternative Rejected Because |
|----------|------------|-------------------------------------|
| Work on `main` only (spec FR-024) versus the branch-per-feature convention (§11.4.195) | Owner instruction (2026-10-03), recorded as constitution Known Conflicts item 15 | Feature branches contradict the owner's decision; live manual QA still gates releases, which replaces the skipped merge gate |
| `blocked-unavailable` status (FR-025) versus the governance's honest SKIP-with-reason | Owner decision that an unavailable real service or device means the test cannot pass; a distinct status avoids mislabelling infrastructure gaps as defects (§11.4.1) | A plain FAIL mislabels the cause; a SKIP lets completion proceed, which the owner forbade |
| Claim SLSA L1 now, work toward a self-assessed L2 (docs/20 DR-20-01) versus the constitution's L2 minimum with no CI | SLSA's "hosted build platform" text does not mandate CI, but an owner-operated dedicated build host qualifying as hosted is an interpretation | Claiming L2 without a hosted platform would be an unverifiable claim; reading it as impossible overstates the standard. Escalated to the owner as an interpretation decision |
| Legacy closed tickets imported as terminal-but-unverified with a re-verification queue (docs/04 D-1) versus counting them as done | FR-008 forbids closing by low severity and `wontfix`; 1,111 closures have no evidence | Importing them as done would repeat the original failure; re-proving all 1,495 first would block the audit. A queue ranks them by risk |
| Third-party package dependencies reported, not updated (spec Q1) versus keeping all dependencies current | Owner decision: only submodules are updated | Bulk updates carry breaking-change risk the owner declined for this feature |
