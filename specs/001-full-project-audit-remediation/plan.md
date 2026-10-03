# Implementation Plan: Full Project Audit and Remediation

| Field | Value |
|---|---|
| Revision | 8 |
| Created | 2026-10-03 |
| Last modified | 2026-10-04 |
| Status | draft (revision 8: Constitution Check VI and the Execution Strategy follow tasks.md rev 11 (612 tasks) and the plan owner's rules taken after the round-10 reviews of commit `6dca8771` (docs/21 IC-52, IC-53): the merge integration writes a `git bundle` backup with a copy of the uncommitted files, a merge commit is held on its own merge-review file `$EV/reviews/CPA-merge-<run_id>.json`, and a conflict is resolved only by a `--resolve-merge` run; which pre-commit check applies to a file is decided by the reviewed path-class table of rule (V), replacing the exemption list, and the S2 secret fold carries the hex filter. Revision 7: the register backup rule names its single helper `scripts/register/backup_db.sh` (tasks.md T064a; docs/04 §12.2 revision 10); Constitution Check VI and the Execution Strategy state the commit-push script's own merge integration of a diverged repository, never a rebase, reset or force (docs/21 IC-49), and name the filters and exemption list of its pre-commit checks and the main-root keying of its ratchet baselines (docs/21 IC-50, IC-51), the plan owner's rules after the review of commit `b9412d06`. Revision 6: the pre-operation backup rule names SQLite's own backup (`sqlite3 .backup` or `VACUUM INTO` through `locked.sh`, never a hardlink, which shares the database file) for the register and keeps the hardlinked mirror for `.git` (Constitution Check VI; docs/04 §12.2 revision 9, docs/21 IC-46 (g)); the owner-decision count is docs/21 revision 10's 41 (ODG-41 added); a held commit is pushed only after a GO verdict that names its run is committed in the main repository (Execution Strategy; docs/21 IC-46). Revision 5: the revision header is now this table, the §11.4.44 form that the revision-header check of tasks.md T040 reads (a `Revision` row and a `Last modified` row in the first 40 lines; the bold line used before counted as missing); the commit-push script is stated to write only its ignored run folder `.audit/commit-push/<run_id>/` and to leave the tracked tree clean (Project Structure, Execution Strategy; docs/06 §11 revision 10, docs/21 IC-42). Revision 4: the `.gitignore` list cited as tasks.md T004, the id in tasks.md rev 6, whose ids T001 to T595 are frozen; the plan-document seed count is docs/21 revision 8's 254. Revision 3: links to directories replaced by links to files, the spec-folder index [README.md](README.md) added, candidate-entry estimate and owner-decision count aligned with docs/21 revision 6) |

**Branch**: `001-full-project-audit-remediation` (no branch; all work on `main`, per spec FR-024) | **Date**: 2026-10-03 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `specs/001-full-project-audit-remediation/spec.md`

## Summary

The spec asks for an exhaustive, evidence-backed audit of every Catalogizer application and shared component, a single register of every known and discovered problem, every finding fixed and proven by deterministic machine-produced results, complete and linked documentation, submodules brought to their latest upstream codebases, and a recursive proof that nothing is uncommitted or unpushed.

The technical approach, fixed by the 21 planning documents under `docs/` (index: [README.md](README.md); about 280,000 words by `wc -w docs/*.md`, measured on 2026-10-03 while the documents were in their revision-6 round; plan revision 2 stated about 239,000), is:

1. **Prove the instruments first.** Both code indexes fail their own health checks today (CodeGraph covers 7,150 files; Lumen reports 10,397 files and stale; no scope data file, no `.lumenignore`, no `.mcp.json`). A health gate must pass before an index is trusted ([docs/02](docs/02-audit-methodology-and-index-strategy.md)).
2. **Build the register and the evidence machinery before the first finding.** The register is the constitution's workable-items engine database plus an extension layer, not a second database ([docs/04](docs/04-findings-register-design.md)); evidence records, polarity runs and tamper-evident chaining follow [docs/06](docs/06-determinism-and-evidence-framework.md).
3. **Containerize before any build or test.** Several existing container assets cannot build from a clean checkout, and some scripts build on the bare host ([docs/16](docs/16-containerized-infrastructure-and-local-enforcement-plan.md)).
4. **Import everything already known** (about 4,700 candidate entries, led by 1,778 HelixQA tickets whose ids collide, plus 254 seeds from the plan documents and 34 doc18 innovation entries tracked as Feature items) and **audit every application twice from the same state** with independent review ([docs/03](docs/03-existing-issue-inventory.md), [07](docs/07-backend-catalog-api-audit-plan.md) to [10](docs/10-android-and-android-tv-audit-plan.md), [12](docs/12-helixqa-challenges-and-governance-plan.md)).
5. **Fix contracts before clients**, remediate by risk with RED-then-GREEN proof repeated three times, close the test-type matrix, convert the prose QA steps, run the documentation and performance programmes, update submodules per the runbook, then verify recursively and close ([docs/05](docs/05-test-strategy-and-coverage-matrix.md), [11](docs/11-submodules-audit-and-update-runbook.md), [13](docs/13-documentation-program-plan.md), [14](docs/14-performance-engineering-plan.md), [15](docs/15-security-and-danger-zone-plan.md)).

The master plan ([docs/21](docs/21-master-plan-phases-risks-and-traceability.md)) defines 8 phases, 53 work packages, 34 risks and 41 grouped owner decisions (ODG-01 to ODG-41; the finer-grained collection in [research.md](research.md) lists 79, OD-01 to OD-79), and maps every FR and SC to at least one work package and one evidence type (0 uncovered). Research with cited sources is in [docs/17](docs/17-research-engineering-practices.md), [18](docs/18-research-product-innovation-and-game-changers.md) and [20](docs/20-research-second-pass-gaps.md); working, self-tested proof-of-concept tools are under `poc/` ([repo_verify](poc/repo_verify/README.md), [doc_links](poc/doc_links/README.md), [route_drift](poc/route_drift/README.md)) and described in [docs/19](docs/19-poc-tools-and-results.md).

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
| VI. Absolute Data, History and Host Safety | PASS | No history rewrite or force-push anywhere in the plan; credential rotation is requested, never done by rewriting (docs/15). Pre-operation backups precede destructive steps: a hardlinked mirror for `.git` (docs/11 §6.6), and for the SQLite register a copy made by SQLite itself (`sqlite3 .backup` or `VACUUM INTO` through `locked.sh`), checked by sha256, `PRAGMA integrity_check` and a restore probe, never a hardlink, which shares the database file; one reviewed helper, `scripts/register/backup_db.sh` (tasks.md T064a), takes every register backup (docs/04 §12.2, docs/21 IC-46 (g)). A repository that diverged from its remote is integrated by a merge commit that the commit-push script makes itself under its lock and after a §9.2 backup (a `git bundle` of the local range checked by `git bundle verify`, plus a copy of every uncommitted file with its sha256), never by a rebase, a reset or a force; a merge with a conflict is refused with nothing moved and is made only from a reviewed resolution on Opus at xhigh, and a merge left in progress is refused (docs/21 IC-49, IC-53; tasks.md T040, T042). The constitution post-pull hook is gated on the owner's explicit go-ahead (docs/11). |
| VII. Continuity and Zero-Loss Traceability | PASS | Register, request ledger and continuation documents are in the plan; legacy tickets are keyed by file path because their ids collide (docs/03, docs/04). |
| VIII. Manual-QA-Final and an Honest Definition of Done | PASS | The plan keeps live manual QA as the final human gate even though work lands on `main` (constitution item 15), and treats unavailable devices and services as `blocked-unavailable`, never as a pass. |

Gate result: no unjustified violation. Two NEEDS ATTENTION items (II and IV) describe the starting state the plan is designed to change; they carry no exemption. Re-evaluation after design is in the section below.

### Post-design re-evaluation

The design artifacts ([research.md](research.md), [data-model.md](data-model.md), [contracts/README.md](contracts/README.md), [quickstart.md](quickstart.md)) introduce no new principle conflict. The decisions that needed a conflict resolution are listed under Complexity Tracking.

## Project Structure

### Documentation (this feature)

```text
specs/001-full-project-audit-remediation/
├── README.md               # Index of every document of this specification folder
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
docs/register/                # register extension SQL and reconciliation.csv (docs/04 §12)
```

New tracked locations this feature creates (from docs/04, 05, 06, 14 and 16 as decided in tasks.md):

```text
scripts/                      # promoted tools: repo/verify_repos.sh, commit-push-all.sh, register/gate.sh,
                              #   anti-mess/sweep.sh, containers/*, longops/*, coverage/* (each with tests/)
tools/evidence/               # evidence recorder (evrec), matrix and analyzers (docs/06)
tools/perf/                   # performance harness (docs/14)
tools/audit/rust_ast/         # Rust AST detector (target/ stays ignored)
build/containers/             # Containerfiles and images.lock.yaml (docs/16 §7.1), incl. testutil/Containerfile
build/components.json         # component inventory used by the container runners
build/hosts.env.example       # build-host template (build/hosts.env stays ignored)
config/index/scope.yaml       # CodeGraph/Lumen scope data file (docs/02)
coverage/exclusions/          # per-application coverage exclusion fences (docs/05 §7.3)
specs/001-full-project-audit-remediation/
├── audit/                    # $AUD: runs, findings/<FND-NNNN>.json, index health
├── evidence/                 # $EV: ledger, anchors, blobs/<sha256>, verdicts, reviews, hc (docs/06 §11)
├── decisions/                # owner-decision records
├── matrix/                   # test-type coverage matrix
└── perf/                     # perf targets and baselines
```

`.gitignore` hides `build/`, `tools/`, `coverage/` and `*.db` today (`.gitignore:40`, `:85`, `:101`, `:139`, `:153`, `:263`). Task T004 of tasks.md is the authoritative list; it appends, as the last lines of `.gitignore`: `!/build/`, `/build/*`, `!/build/containers/`, `!/build/components.json`, `!/build/hosts.env.example`, `/build/containers/**/*.bin`, `!/scripts/build/`, `!/tools/`, `/tools/*`, `!/tools/evidence/`, `/tools/evidence/.cache`, `!/tools/perf/`, `!/tools/audit/`, `/tools/audit/*`, `!/tools/audit/rust_ast/`, `/tools/audit/rust_ast/target/`, `!/tools/audit/rust_ast/Cargo.lock`, `!/coverage/`, `/coverage/*`, `!/coverage/exclusions/`, `!/scripts/coverage/`, `!/docs/workable_items.db`, `docs/*.bak-*`, `docs/.register.lock` and `/.audit/`, proved by `git check-ignore` on its planned and control paths before and after. `/.audit/` is the ignored, never tracked home of the commit-push run folders (`.audit/commit-push/<run_id>/`) and the long-op records (docs/06 §11). Coverage evidence under the specification folder uses `coverage_baseline/` and `coverage_targets/` because the unanchored `coverage/` rule applies at any depth (docs/06 §11).

**Structure Decision**: no new top-level application. New tooling is promoted from the `poc/` tools ([docs/19](docs/19-poc-tools-and-results.md)) into tracked, tested locations under `scripts/`, `tools/`, `build/`, `config/index/` and `coverage/exclusions/` (listed above) during the work packages that need it; new shared contracts live next to their owners. The register is the constitution's engine database plus an extension layer (docs/04), and the planning documents, audit outputs (`$AUD`) and evidence (`$EV`) stay under this specification directory.

## Execution Strategy

### TDD Requirements

- [ ] Register and evidence recorder (WP-05, WP-06): invariants (status custody, recurrence links, evidence class) are checked by RED-first tests with golden-bad fixtures.
- [ ] Every fixed finding (all applications): a test that fails before and passes after, repeated three times, with machine-recorded verdicts.
- [ ] Auth, SSRF and input-validation paths (backend WebSocket and image proxy; desktop `make_http_request`; Android exported components): negative-path tests with concrete hostile inputs.
- [ ] Verifier, link-crawler and route-drift extractor: promoted from the POCs (`poc/repo_verify`, `poc/doc_links`, `poc/route_drift`) with their golden-good, golden-bad and control tests retained. The commit/push script (`scripts/commit-push-all.sh`, stages S0-S8, docs/16 §12, docs/21 IC-16) has no POC and is written test-first; it writes every output of a run only into its ignored run folder `.audit/commit-push/<run_id>/`, never into the tracked tree, and marks each commit it makes with the trailer `CPA-Run: <run_id>`, so the tracked tree is clean after every run (docs/06 §11, docs/21 IC-42). A change made before its review GO is committed as a held commit (`Awaits-Review: <verdict path>`) and pushed only once a GO verdict committed in the main repository names its run in `covers_runs` (`contracts/review-verdict.schema.json`; docs/21 IC-46). A diverged repository is integrated by the script's own `git merge --no-ff` commit, which carries its run trailer and, when it resolves conflicts (only through a `--resolve-merge` run) or merges into a held range, is held on its own merge-review file `$EV/reviews/CPA-merge-<run_id>.json`; nothing below an unreleased merge is pushed (docs/21 IC-49, IC-53); the checks it takes from `.pre-commit-config.yaml` keep their hook filters, and which check applies to a file is decided by the reviewed path-class table of rule (V) (classes `source`, `generated`, `evidence`, `patches`, `governance-carrier`, `fixtures`; tasks.md T040b), with one register bound of 16 MiB per file, while the S2 secret fold applies to every class and passes bare 40- and 64-hex values only through its baseline's hex filter (docs/21 IC-50, IC-52); its ratchet baselines are keyed from the main root over every own-organisation repository (docs/21 IC-51).
- [ ] Migrations and dialect rewriting (both SQLite and PostgreSQL): tests on a real database of each dialect.

### Parallel Execution Opportunities

- [ ] Streams ST-API, ST-WEB, ST-DESK, ST-AND, ST-SUB and ST-SEC audit disjoint components and can run as parallel subagents after P2, bounded by six concurrent agents, 60% memory and thread headroom (docs/21 §4.2).
- [ ] ST-DOC and ST-PERF can start as soon as their inputs exist (docs/21 §3.1), overlapping P5 remediation.
- [ ] Per-application client fixes (P5) wait for the live route dump and contract tests (P4), but may run while another application is still in P3.
- [ ] Single-owner resources serialise: one stream per physical device, build host and the register database.

### Human Checkpoints

The checkpoint ids are those of docs/21 §3.3; tasks.md records each one under `$EV/hc/<HC-id>.json`.

1. **HC-0** (P0 entry): plan accepted, owner request list sent (credentials, devices, build hosts, legacy-ticket policy, constitution hook variant).
2. **HC-1** (P0 exit): tooling verified on a clean checkout (verifier, commit/push script, pinned container runners); register empty and operational.
3. **HC-1b** (P1): remote build-host roles and capacity confirmed by the owner (ODG-07).
4. **HC-2** (P2): SC-001 reconciliation reviewed; legacy-closure policy (ODG-09) and category mapping (docs/04 D-6) confirmed.
5. **HC-3** (P3): audit report accepted, including first register totals, "none found" units with evidence, and the independent review verdict, before any fix.
6. **HC-4** (P4): contract changes that need a compatibility window approved (ODG-25).
7. **HC-5** (P5, per user story): behaviour confirmed against the acceptance scenarios; §11.4.122 removal decisions (ODG-20 to ODG-24) before any component is removed.
8. **HC-6** (P7): full retest on the candidate artifact and recursive repository verification reviewed before the final push.
9. **HC-7** (P7): final acceptance against the spec, including the owner's live manual QA.

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
