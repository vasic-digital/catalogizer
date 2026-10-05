# 05 - Test Strategy and Coverage Matrix

| Field | Value |
|---|---|
| Revision | 18 |
| Created | 2026-10-03 |
| Last modified | 2026-10-05 |
| Status | draft (revision 18: sections 11 and 13.5 follow tasks.md rev 36 (676 tasks, 81 suffix ids, no task added; rev 36 commit `f44e2bb9`, the FINAL round of the round-35 reviews, the design frozen at the central decisions C1 to C12 of T042; rev 34 `4939146a` and rev 35 `9ebde74f` included): the inventory sides with `contract_kind`, a `test_paths` array, `consumer` and the provider `name`, and the per-pair T325 rule; the section 13.5 release-seam bullet amended in place (the released-seam precheck withdrawn, every release-seam script and helper only through `cpa-host --exec-approved` with one refusal code per path class, every release-seam GO owed and listed in `$EV/hc/owner-trust/owed.json`, the harness trust state built by the C3 replay, the `images.lock.yaml` equality dropped from the precondition) and a new bullet on the pre-release production form (`pre_release`, `pre_release_output`), the two post-release `can_i_deploy.sh` runs with one difference policy, the regenerated claim ledger, the one main-tree `mutation_ratchet_challenge.sh`, the catch-up runner, the commit-turn `--holder-pid` (C12), `run_failed` and T581's per-push owed check; revision 17: sections 9.1, 11 and 13.5 follow tasks.md rev 33 (676 tasks, 81 suffix ids, no task added; rev 33 commit `711a8c30`, the round-32 reviews; T288b, T318, T318a, T320, T335a, T383, T394a, T399, T413, T435a, T457, T514, T557, T564a, T565, T567, T569, T580b, T580e, T582, T583, T593): the contract-inventory n/a fields at the side level with fixture `na_on_row`; a guard-polarity flip taking effect in a CPA run only after the owner's approval (central decision C1 of tasks.md T042); the P4-P5 CPA check stages under three rules (the T039 harness, the owner-approval precondition, the pending control) with their post-release real-stage steps named in the P5, P6 and P7 exit records; the P6-P7 fixture harness replacing the scratch clones of the release-seam fixtures; the released-seam precheck `scripts/release/seam_released.sh` (T564a) before every release-seam run; the T514 keyed-union fixture; the cancel of a suspended run through the holder's expiry; the T580b `.git/hooks/` decisions; the T582 trust-manifest closure check; revision 16: sections 8.3, 9.1, 11 and 13.5 follow tasks.md rev 32 (676 tasks, 81 suffix ids; rev 32 commit `87c6c478`, the round-31 reviews; T570, T571, T572, T580, T580a, T580b, T580d, T580e, T580f, T581, T582, T320, T324, T334a, T335a, T336a): every SC-005 draw after a GO writes its own iteration `$EV/reviews/WP-71-sc005-r<n>.json`, the latest iteration read at closure (`sc005_not_latest`); the conductor starts and resumes the turn's run through the host entry point `cpa-host`, a grant's holder is dead only when no live CPA process or suspended-run holder exists for its run id, a new conductor takes over with `--grant --adopt` (`conductor_adopted`, `run_cancelled`, `adopt_no_grant`, `adopt_run_mismatch`, `adopt_holder_alive`), a refused granted id is never retried (fixture (xvi)); the T580d hook-commit run is the last CPA run before T582; the inventory `dependentRequired` rule (`inventory_na_status_missing`), `ancestor_empty`, the T324 two-tree check and the T334a inventory rename rule; revision 15: sections 8.3 and 13.5 follow tasks.md rev 31 (675 tasks, 80 suffix ids; rev 31 commit `14bbb979`, the round-30 reviews; T570, T571, T580, T580a, T580b, T580e, T580f, T581, T582, T514, T518, T567, T042): `scripts/qa/sc005_verdict_check.py` is a release-seam file held on `$EV/reviews/WP-71-sc005.json`, fixture `sc005/unheld`; fixture (vi) split into (vi-a) seeded and (vi-b) lowering; T581 depends on T580f; the commit-turn grant holder is the T580e conductor, which mints the run id (`CPA --run-id`); the `path_gate_unheld` fixtures run through the real CPA S2 path; the T567 host step reaps once before refusing; the T580b hook outputs classified by T580d; revision 14: section 13.5 follows tasks.md rev 30 (675 tasks, 80 suffix ids; rev 30 commit `84145b2e`, the round-29 reviews; T517, T570, T570a, T580e, T580f, T581): the RED/GREEN sample change set (`scripts/qa/redgreen_sample.py`) held and reviewed with the verdict-coverage set on `$EV/reviews/WP-71-verdict-coverage.json` by T570a (`redgreen_sample_not_reproduced`); the T580f qa-cycle scope is the T066 change set; T581 (0) folds in every qa-cycle verdict; every main-tree writer honours the commit-turn grant; T517 soak runs triggered by the P5 exit record and by each T566 candidate build; revision 13: sections 9.1 and 13.5 follow tasks.md rev 29 (675 tasks, 80 suffix ids; rev 28 commit `fb1d1982`, rev 29 commit `7daa7938`; T320, T335a, T569, T570, T570a, T580a, T580e, T580f): the T570 verdict-coverage change set is held on its own verdict `$EV/reviews/WP-71-verdict-coverage.json`, reviewed by the new task T570a, on which T569 depends (the round-28 T570/T565 deadlock); the T580e main-repository commit turn is frozen from grant to commit (`.audit/commit_turn.json`, `evrec` `commit_turn_held`), general windows run before a turn, and the turn check is reviewed in `$EV/reviews/WP-73-commit-turn.json`; the dump-and-commit is reviewed in `$EV/reviews/WP-73-qa-cycle-<fingerprint>.json`; the inventory n/a status and the T335a path check; revision 12: section 8.3 item 6 follows tasks.md rev 28 (673 tasks, 78 suffix ids; rev 28 commit `fb1d1982`; T570, T571, T580, T580a, T582): every T571 verdict passes `sc005_verdict_check.py`, which adds `final_with_open_survivor`, `scoped_without_routed` and the `--require-final` closure mode (`final_go_required`, `sc005_candidate_mismatch`, `final_go_stale`); verdicts record `candidate_fingerprint`; revision 11: section 8.3 follows tasks.md rev 27 (673 tasks, 78 suffix ids; rev 26 commit `616fb74a`, rev 27 commit `13bc5de0`; T570, T571, T572, T580b): the SC-005 verdict is written in two scopes, a `scoped` GO that releases the covered T572 commits with each constitution-test survivor recorded `routed_to_T580b` and a `final` GO only after the T580b (3c) re-check, checked by `scripts/qa/sc005_verdict_check.py` (`survivor_open`, `survivor_misrouted`, `go_scope_missing`), which removes the rev 26 cycle T571 -> T580b -> T572 -> T571; `docs/security/SLSA_LEVEL.md` is an excluded record of the release-seam rule, never a listed data file; revision 10: section 13.5 follows tasks.md rev 26 (673 tasks, 78 suffix ids): the `release_seam` path class and its list holding only tracked policy or threshold inputs as data files; the `--seam pre-qa` ratchet judged only on the candidate's own `qa-<fingerprint>` cycle (`pre_qa_no_candidate_cycle`, the retired `pre_qa_unseeded` refused by the readiness gate); one version increment per QA deploy, a re-cut deploy minting its own; the constitution survivor route through a further T580b (3c) iteration. Revision 9: follows tasks.md rev 24: section 11 adds the expected-RED guard lanes of T334a (one marker form per language that its runner selects on: a `*.expected-red.test.ts` file excluded by the default vitest lane, a JUnit 5 tag or JUnit 4 category in the Gradle trees, a Rust `#[ignore = "expected-red: ATM-<id>"]`, each run by a guard lane and backed by a row of the standing guard registry); the new section 13.5 states the release seam (the standalone release-seam checks at T569 and T582, verdict coverage, the two modes `--seam pre-qa` and `--seam final` of the escape gates, the SLSA gate), the QA-deploy-readiness gate `scripts/release/qa_handoff_gate.sh` (T564a), the final manual-QA hand-off T580e with the review of each manual-finding fix (T580f) and the candidate re-cut set; section 16 gains its acceptance row. Revision 8: section 12.1 and the section 15 risk and open-item rows follow the plan owner's decision C1 of 2026-10-04 (docs/21 ODG-07 revision 15): every build and every compiling test lane runs on the remote build host, dispatched event-driven, never locally (document 16 section 9.6). Revision 7: section 9.3 records when the can-i-deploy gate runs and on which provider version, as the P4-P7 tasks of the round-13 tasks.md wave state it: `--provider catalog-api@<fingerprint>` with the fingerprint of the artifact under test in P4 and P5 (T335, T358) and of the T566 candidate at the release seam (T569, T582), whose provider verification T567 re-runs and publishes under that fingerprint; a provider version without published verification results is refused; the verdict is written only to the file of the required `--out` option (T324, T325). Revision 6: sections 7.3, 11 and 13.1 record how tasks.md rev 12 wires these gates into the commit-push script (document 16 revision 10 §12.2.6): the coverage gate of T503 is the registry row `coverage_gate` with scope `changeset`, outside the path-class table; the protected-spec stage of T564 is the row `protected_spec` with scope `files` and a class row for every class; and the matrix generator of T504 writes `$EV/matrix/coverage-matrix.md` as a class `source` file with the 11.4.44 revision header. Revision 5: the protected-spec rule of section 11 names its local enforcement, the `CPA` stage of tasks.md T564, instead of an unresolved pre-push check, since 11.4.234 allows no blocking hook and document 16 installs none. Revision 4: section 13.4 accounts for all thirteen applications A1 to A13 in the translation table, the accessibility section and the applicability YAML (A12 added; A10 corrected after a file-name search found an English-only i18n seam in the Go modules and translated bundles in two A12 modules); the section 7.2 citation of the `.gitignore` negation task corrected to tasks.md T004. Revision 3: coverage baselines and their run records move to `$EV/coverage_baseline/<app>/` and targets to `$EV/coverage_targets/<app>/targets.yaml`, because `evidence/coverage/` is ignored at any depth by `.gitignore:139` (docs/21 IC-38); new section 13.4 records the translation and i18n applicability per application (n/a with reasons and one open server-side item) and the accessibility (WCAG 2.2 AA) checks per user-facing application, owned by docs/21 WP-61; `\|` escaped in one table cell. Revision 2: catalog-web test-file count and submodule count stated precisely after independent review) |
| Feature | specs/001-full-project-audit-remediation |
| Spec requirements covered | FR-009, FR-010, FR-011, FR-016, FR-025 (and the test side of FR-008, FR-021, FR-022) |
| Success criteria covered | SC-004, SC-005 (and the test side of SC-003, SC-011) |
| Governance anchors | Constitution Principle II; appendix 11.4.27, 11.4.169, 11.4.224, 11.4.85, 11.4.244, 11.4.239, 11.4.248, 11.4.115, 11.4.135, 11.4.201, 11.4.245, 1.1 |
| Companion document | 06-determinism-and-evidence-framework.md (evidence schema, polarity, chaining) |

## Table of contents

1. Purpose, scope and reading guide
2. The test-type catalogue the constitution defines
3. Applications and shared components in scope
4. Measured current state (the real matrix)
5. Findings that change the plan
6. Closing every absent or partial cell (work packages)
7. Coverage phase-in per application (FR-011)
8. Mutation testing and the SC-005 sampling procedure
9. Contract tests on both sides and can-i-deploy (FR-016)
10. Real services, credentials and devices: the blocked-unavailable status (FR-025)
11. Flaky-test quarantine and protected regression specs
12. Execution model: containers, ordering, determinism, host safety
13. The matrix as a living, gated artifact
14. Decision records
15. Risks and open items
16. Traceability and acceptance evidence

---

## 1. Purpose, scope and reading guide

This document is the technical plan for the test side of the feature. It does not restate the
spec. It answers four questions: which test types exist, which of them exist today for each
application (measured, not assumed), how every missing one gets written, and how the coverage
floor, mutation checking, contract testing and real-dependency handling are applied.

Conventions used throughout:

- A cell is **present** when the artifact exists, runs against the real system where the type
  requires it, and I could verify at least one concrete file; **partial** when something exists but
  falls short of the constitution definition (for example an integration test that uses an
  in-memory database); **absent** when I found nothing by the search stated in section 4; **n/a**
  only where the type has no meaning for the application (for example DDoS for a static website
  build). An n/a still needs a recorded reason (11.4.27 B.1, honest SKIP-with-reason in the
  type-breadth ledger).
- Counts in section 4 were produced by read-only `find`/`grep` commands recorded in section 4.1.
  Nothing was executed on the host beyond file counting. `UNCONFIRMED:` marks anything I could not
  verify. Whether a test passes today is `UNKNOWN:` for every cell: this plan inventories
  existence, and the baseline run in section 7.2 supplies pass/fail and coverage.
- All commands that build, compile or run test suites use the containerised form (11.4.173,
  FR-021). Section 12.2 defines the wrapper. Host-side commands in this document are read-only.

## 2. The test-type catalogue the constitution defines

Principle II requires "every supported test type" to cover the whole codebase, with HelixQA and
Challenges fully incorporated. Appendix 11.4.27(B) enumerates fifteen types; 11.4.169 lists the same
closed set; 11.4.27(B.1) and 11.4.224(B.1) add a seven-type minimum floor (unit, integration, E2E,
full automation, security, performance and benchmark, anti-bluff). The fifteen are the unit of
accounting for SC-004.

| # | Type | What it must do here | Real system required | Oracle strategy (11.4.245) | Evidence class (11.4.226) |
|---|---|---|---|---|---|
| 1 | Unit | Isolated logic; mocks permitted only here | no | specified / derived / invariant | source |
| 2 | Integration | Real wired components, real database, real backing services booted by containers | yes | specified (schema, protocol) | runtime |
| 3 | E2E | Whole user flow on the target topology (API process, web in a browser, app on device) | yes | specified (acceptance scenario) | runtime / user-visible |
| 4 | Full automation | Orchestrated, re-runnable, no manual step, deterministic over repeated runs; one suite per feature x platform | yes | specified | runtime |
| 5 | Security | Authn/authz boundaries, injection, secret-leak scans, dependency CVEs, fuzzing | yes | specified / invariant | runtime |
| 6 | DDoS | Request-flood resilience at the advertised tier with rate-limit and back-pressure assertions | yes | specified (SLO / limit) | runtime |
| 7 | Scaling | Behaviour under linear load growth, replica add/remove | yes | specified (SLO) | runtime |
| 8 | Chaos | Process kill, network partition, disk full, clock skew, corrupted input; graceful degradation | yes | invariant | runtime |
| 9 | Stress | Sustained load above tier, bounded resource exhaustion, clean recovery | yes | invariant (no leak, no deadlock) | runtime |
| 10 | Performance | Latency, throughput, tail latency against recorded baselines | yes | statistical | runtime |
| 11 | Benchmarking | Micro and macro benchmarks with historical drift detection | yes | statistical | runtime |
| 12 | UI | Visual regression, DOM state and interaction flow on every target platform | yes | golden-master + vision oracle | user-visible |
| 13 | UX | Flow correctness, accessibility to WCAG 2.2 level AA, i18n where it applies, visual-cue order (section 13.4) | yes | specified (WCAG 2.2 AA success criteria, flow spec) | user-visible |
| 14 | Challenges | Per-feature Challenge scripts from the Challenges submodule with captured runtime evidence | yes | specified | runtime |
| 15 | HelixQA | Every written test bank executed; autonomous QA sessions in release gates | yes | specified + human (manual QA remains the final gate, 11.4.185) | user-visible |

Applicability is data, not opinion. The applicability map in section 13.2 lists, per application,
which of the fifteen types apply and gives the reason for each n/a. 11.4.27(B) states the default:
a CLI-only feature takes unit, integration, E2E, full automation, security, chaos, stress,
performance, Challenges and HelixQA; a UI feature adds UI and UX; a network service adds DDoS,
scaling and benchmarking.

Cross-cutting obligations that apply to all fifteen types and are therefore planned once, not per
type:

- Test-first with a RED observed on the broken artifact and a polarity switch (11.4.224 A,
  11.4.115). Detailed in document 06.
- At least 60% of assertions verify observable behaviour; mutation score at least 85%
  (Principle II).
- Determinism across three repeated runs (FR-010, SC-003, 11.4.50).
- Real systems everywhere except unit tests (11.4.27 A). The only permitted placeholders sit in
  unit-test sources.
- Every fixed defect gains a permanent regression guard (11.4.135) and every critical-invariant
  change carries failure-path scenarios (11.4.239, section 6.9).

## 3. Applications and shared components in scope

| Id | Component | Language / stack | Path | Test runner(s) found |
|---|---|---|---|---|
| A1 | catalog-api (backend) | Go 1.25.7 (`catalog-api/go.mod`), Gin, SQLite/SQL Cipher | `catalog-api/` | `go test`, bash suites, k6 |
| A2 | catalog-web | TypeScript, React, Vite | `catalog-web/` | vitest 4.0.x (`catalog-web/package.json`), Playwright |
| A3 | catalogizer-desktop | Tauri (Rust) + React | `catalogizer-desktop/` | vitest ^0.34, Playwright, `cargo test` |
| A4 | installer-wizard | Tauri (Rust) + React | `installer-wizard/` | vitest ^0.34, `cargo test` |
| A5 | catalogizer-android | Kotlin, Compose, Room, Retrofit | `catalogizer-android/` | Gradle unit, androidTest, JaCoCo |
| A6 | catalogizer-androidtv | Kotlin, Compose for TV | `catalogizer-androidtv/` | Gradle unit, androidTest, JaCoCo |
| A7 | catalogizer-api-client | TypeScript library | `catalogizer-api-client/` | vitest |
| A8 | Website | VitePress documentation site | `Website/` | none (`Website/package.json` has dev/build/preview only) |
| A9 | Build | Bash build framework | `Build/lib/*.sh` | none found |
| A10 | Go shared modules | 14+ Go submodules (Auth, Cache, Database, ...) | `submodules/<name>/` | `go test` in each |
| A11 | TS / React shared modules | api client, media types, WebSocket client, UI components, React feature modules | `submodules/*_ts`, `submodules/*_react`, `submodules/ui_components_react` | vitest (per module) |
| A12 | Governance and QA modules | constitution, challenges, helix_qa, containers, vision engine, others | `submodules/constitution`, `submodules/challenges`, `submodules/helix_qa`, ... | per module |
| A13 | Repository-level harness | bash suites, k6, HelixQA banks, Challenges | `scripts/`, `tests/`, `challenges/` | bash |

The submodule list comes from `.gitmodules` (44 modules: `grep -c '^\[submodule' .gitmodules` returned 44 on 2026-10-03; `grep -c 'path = '` agrees). Per-module test file counts are in
section 4.3. FR-006 requires findings in shared modules to be fixed in those modules and pushed to
their upstreams; the same applies to tests written for them (FR-006, FR-017). A12 modules vendor a
large `tools/opensource` tree inside `submodules/helix_qa`; that tree is third-party and falls under
the accepted-exception rule of FR-008 (status report only, no modification). Its test counts are
therefore excluded from every matrix cell and listed separately.

## 4. Measured current state (the real matrix)

### 4.1 How the numbers were produced

Read-only commands run from the repository root on 2026-10-03. They exclude `node_modules`,
`target`, `dist` and `.git`.

```bash
# Go
find catalog-api -name '*_test.go' | wc -l                       # 372
grep -rh '^func Test' catalog-api --include=*_test.go | wc -l     # 5367
grep -rl 'func Benchmark' catalog-api --include=*_test.go | wc -l # 18
grep -rl 'func Fuzz' catalog-api --include=*_test.go | wc -l      # 8
grep -rn 't\.Skip' catalog-api --include=*_test.go | wc -l        # 113
grep -rh '^//go:build' catalog-api --include=*_test.go | sort | uniq -c
# TypeScript
find catalog-web/src -name '*.test.ts*' -o -name '*.spec.ts*' | wc -l     # 138 = 137 test sources + 1 snapshot file (snapshots.test.tsx.snap)
find catalog-web/src \( -name '*.test.ts' -o -name '*.test.tsx' -o -name '*.spec.ts' -o -name '*.spec.tsx' \) | wc -l   # 137 (sources only)
grep -rnE '^\s*(it|test)\(' catalog-web/src | wc -l                        # 2369
find catalog-web/e2e -name '*.ts' | wc -l                                  # 39
# Kotlin
find catalogizer-android/app/src/test -name '*.kt' | wc -l                 # 69
grep -rn '@Test' catalogizer-android/app/src | wc -l                       # 1132
# Rust
grep -rn '#\[test\]' catalogizer-desktop/src-tauri | wc -l                 # 113
# HelixQA banks
grep -h '^- id:' challenges/helixqa-banks/*.yaml | wc -l                   # 1257 cases
grep -c 'TODO: Convert' challenges/helixqa-banks/*.yaml                    # 1178 markers
```

These are file and declaration counts, not executed-test counts. A `Test` function count says
how many exist, not how many assert something meaningful; section 5 and section 8 deal with that.

### 4.2 Per-application inventory

| Component | Unit | Other evidence found |
|---|---|---|
| A1 catalog-api | 372 `_test.go` files, 5367 `Test*` functions across `handlers` (40), `internal` (160), `repository` (26), `services` (27), `middleware` (18), `database` (16), `filesystem` (11), `challenges` (14), `tests` (42) | `catalog-api/tests/{integration,security,stress,performance,monitoring,benchmarks,automation,mocks}`; 94 non-test Go files in `catalog-api/challenges`; 18 files with benchmarks, 8 with fuzz functions |
| A2 catalog-web | 137 test source files in `src` (138 matches of `*.test.ts*`/`*.spec.ts*`, the 138th being the snapshot file `components/__tests__/__snapshots__/snapshots.test.tsx.snap`), 2369 `it`/`test` declarations | 39 Playwright files under `catalog-web/e2e` (including `visual-regression*.spec.ts`, `accessibility*.spec.ts`, `performance.spec.ts`, `websocket-realtime.spec.ts`); coverage thresholds 80 set in `catalog-web/vitest.config.ts` |
| A3 desktop | 27 test files, 393 declarations; 3 Rust files with 113 `#[test]` | 1 Playwright file under `catalogizer-desktop/e2e`; `vitest --coverage` script, no threshold file checked |
| A4 installer-wizard | 25 test files, 457 declarations; 7 Rust files with 110 `#[test]` | `installer-wizard/test-results.json` and `badges.json` exist; their provenance is `UNCONFIRMED:` |
| A5 android | 69 unit files, 1132 `@Test` (shared count covers both source sets) | 10 `androidTest` files (Room DAO tests, Compose test rule); JaCoCo task registered, no verification rule |
| A6 androidtv | 87 unit files, 1279 `@Test` | 1 `androidTest` file; `catalogizer-androidtv/challenges/helixqa-banks` directory present |
| A7 api-client | 8 test files, 286 declarations | no coverage configuration in `vitest.config.ts` |
| A8 Website | 0 | VitePress build only |
| A9 Build | 0 test files in `Build/` | `scripts/tests/run-all.sh` and `scripts/tests/lib` exist but are not tied to `Build/` by name; `UNCONFIRMED:` whether they test it |
| A13 harness | n/a | 11 bash full-automation suites in `scripts/testing/full_automation`; 16 k6 scripts in `tests/k6`; 15 HelixQA bank files in `challenges/helixqa-banks`; 5 Challenge scripts in `challenges/scripts` |

### 4.3 Shared-module test file counts

Counts of `*_test.go`, `*.test.ts`, `*.test.tsx` per module (same exclusions): assets 9, auth 15,
auth_context_react 2, cache 16, catalogizer_api_client_ts 7, challenges 127, collection_manager_react
5, concurrency 22, config 5, constitution 279, containers 323, dashboard_analytics_react 5, database
24, discovery 15, doc_processor 21, entities 5, event_bus 13, filesystem 11, helix_memory 35, lazy 5,
llm_orchestrator 37, llm_provider 103, llms_verifier 12, media 19, media_browser_react 5,
media_player_react 4, media_types_ts 4, memory 13, middleware 24, observability 18, rate_limiter 18,
recovery 10, replay_buffer 1, screen_diff 1, security 43, storage 31, streaming 23, superspec 0,
training_collector 1, ui_components_react 18, vision_engine 25, visual_regression 1, watcher 11,
websocket_client_ts 4. `helix_qa` reports 2552 but includes the vendored `tools/opensource` tree and
must be recounted without it (work package TS-00). Modules with 0 or 1 test files (superspec,
replay_buffer, screen_diff, training_collector, visual_regression) are candidate absent cells and
need a per-module applicability review, since some may be configuration or asset modules.

### 4.4 The matrix (applications x fifteen types)

Legend: P present, ~ partial, A absent, n/a not applicable (reason in section 13.2), ? could not
classify without reading more than the structural inventory (`UNCONFIRMED:`).

| Type | A1 api | A2 web | A3 desktop | A4 installer | A5 android | A6 tv | A7 client | A8 Website | A9 Build | A10 Go mods | A11 TS mods |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 Unit | P | P | P | P | P | P | P | A | A | P (most) | P (most) |
| 2 Integration | ~ | ~ | A | A | ~ | A | A | A | A | ? | ? |
| 3 E2E | ~ | P | ~ | A | ~ | A | A | A | n/a | A | A |
| 4 Full automation | P | ~ | A | A | A | A | A | A | A | A | A |
| 5 Security | ~ | ~ | A | A | A | A | A | A | A | ? | A |
| 6 DDoS | ~ | n/a | n/a | n/a | n/a | n/a | n/a | A | n/a | ? | n/a |
| 7 Scaling | A | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a | n/a |
| 8 Chaos | ~ | A | A | A | A | A | A | n/a | A | ? | A |
| 9 Stress | P | A | A | A | A | A | A | n/a | n/a | ? | n/a |
| 10 Performance | ~ | ~ | A | A | A | A | A | A | A | ? | A |
| 11 Benchmarking | ~ | A | A | A | A | A | A | n/a | A | ? | A |
| 12 UI | n/a | P | ~ | A | ~ | ~ | n/a | A | n/a | n/a | ~ |
| 13 UX | n/a | ~ | A | A | A | A | n/a | A | n/a | n/a | A |
| 14 Challenges | P | ~ | A | A | A | A | A | A | A | ? | A |
| 15 HelixQA | ~ | ~ | ~ | ~ | ~ | ~ | A | A | A | A | A |

Reading rules for the table: A1 integration is partial because the Go integration suite mixes real
HTTP servers with in-memory SQLite and mock providers (section 5, finding F-3). A1 E2E is partial
because only four files carry the `e2e_binary` build tag, and I did not verify that they run a
built binary against a database in a container (`UNCONFIRMED:`). A1 security is partial because
`catalog-api/tests/security` holds two files (`auth_security_test.go`, `injection_test.go`), one
tagged `catalog_security_real`, alongside gosec, nancy and trivy scripts under `scripts/`; running
them has not been verified. A1 stress is present (eleven files in `catalog-api/tests/stress` plus
k6 `stress_test.js`, `soak_test.js`, `spike_test.js`). A1 scaling is absent: no file whose name or
content addresses replica add/remove was found; `tests/k6/breakpoint_test.js` measures load growth
on one instance, which covers vertical load growth only and is classed performance. HelixQA is
partial for A1-A6 because banks exist but 1178 of their steps are non-executable placeholders
(finding F-1).

A2 full automation is partial: the catalog functional-matrix suites in
`scripts/testing/full_automation` drive the API, not the web UI. A3 E2E is partial: one Playwright
spec for a Tauri application of 27 test files. A5/A6 integration and E2E are partial: ten and one
androidTest files respectively, which cover Room DAOs and Compose rules, not the sign-in to playback
journey.

The `?` cells for A10 are real unknowns. Each shared Go module needs a one-time classification
(work package TS-00) that reads its test directory and assigns P/~/A per type, because 14+ modules
with differing responsibilities cannot be classified from counts.

## 5. Findings that change the plan

Each finding is a candidate register item (FR-001, FR-007). IDs here are working labels; the
register assigns the stable identifiers.

**F-1. HelixQA banks are largely prose, not executable.** In `challenges/helixqa-banks/` there are
1257 cases and 1178 `TODO: Convert to executable` markers (api comprehensive 313, api negative-paths
200, web comprehensive 164, android negative 97, web negative 92, android comprehensive 84, wizard
comprehensive 69, desktop comprehensive 67, cross-platform 51, desktop negative 25, wizard negative
16). The Android TV banks carry zero markers. A bank step whose `action` is a comment cannot execute;
a run of such a bank that reports PASS would be a bluff (11.4.1, 11.4.27). This is the largest
single test-type gap: the HelixQA cell for A1-A6 is "partial" at best, and the SC-004 claim cannot
be made until each marker is converted or the case is closed with evidence. `scripts/audit/
anti-bluff-scan.sh` already defines a finding kind for it (`PROSE_HELIXQA_ACTION`); its output on
the current tree is `UNKNOWN:` until run.

**F-2. Non-unit tests that use fakes (11.4.27 A violations to confirm).** `catalog-api/tests/mocks/`
holds FTP, NFS, SMB and WebDAV mock servers; `catalog-api/tests/integration/provider_pipeline_test.go`
defines `mockProvider`; nine test files under `catalog-api/tests` open `:memory:` SQLite; four
integration files use `httptest`. The repository already ships real containerised equivalents in
`docker-compose.test-infra.yml` (pure-ftpd, samba, bytemark/webdav, an NFS server, MinIO), but all
use floating `:latest` tags, a reproducibility defect against 11.4.246. Whether each in-memory
SQLite use is a violation depends on whether production uses SQLite: the API module lists
`go-sqlcipher`, so an in-memory SQL Cipher database may be a legitimate real engine. That is
`UNCONFIRMED:` and is resolved by DR-3 in section 14.

**F-3. Skips.** 113 `t.Skip` calls in catalog-api tests. Under FR-025 a skip is not an allowed
outcome for an unavailable dependency; those that guard on a missing service, credential or device
become `blocked-unavailable` (section 10). Those that guard on something else (platform, build tag)
need classification. `scripts/audit/anti-bluff-scan.sh` has `SKIP_WITHOUT_TICKET`.

**F-4. Coverage configuration is uneven and mostly absent.** Only `catalog-web/vitest.config.ts`
declares thresholds (80 for lines, functions, branches, statements, below the 85% floor for new
code). Desktop and installer have a coverage script and `@vitest/coverage-v8` at `^0.34.0` but no
threshold; the API client has none; Go coverage is collected by `scripts/track-coverage.sh`
(`go test ./... -coverprofile`, with `2>/dev/null || true`, which swallows failures and would record
0 on a broken run); JaCoCo tasks exist for both Android applications without a verification rule;
no Rust coverage tool is configured. The vitest major versions differ (4.x in web, 0.34 in
desktop, installer), which is a tooling consistency finding and a dependency finding for FR-017's
reporting.

**F-5. Mutation tooling is referenced but not instantiated in the main repository.** The
`mutation_ratchet_challenge.sh` script exists in three submodules and expects a `.go-mutesting.yml`
and `challenges/baselines/bluff-baseline.txt`; neither exists at the repository root. Principle II
claims a mutation score of at least 85% is enforced; in this repository the enforcement is not
instantiated (the claim is `UNCONFIRMED:` until DR-5 is carried out). No mutation tool is
configured for TypeScript, Kotlin or Rust.

**F-6. Contracts exist only in one direction and against an in-memory database.**
`catalog-api/tests/integration/contract_test.go` checks response shapes "defined in the TypeScript
API client" with a Gin engine over in-memory SQLite. There is a spec in `docs/api/openapi.yaml`, a
TypeScript client in `catalogizer-api-client` and Retrofit in the Android application, but no
consumer-driven contract, no broker, and no can-i-deploy gate were found (grep for `pact` finds only
unrelated UI strings in the web and TV sources).

**F-7. Playwright retries.** `catalog-web/playwright.config.ts` sets `retries: process.env.CI ? 2 :
0`. There is no CI (11.4.156), so the branch is inert locally, but a retry setting is the
rerun-until-green pattern 11.4.248 forbids and must be removed rather than left latent.

**F-8. Absent applications.** The Website and `Build/` have no tests; the Build framework is bash
and 11.4.224 requires an executing test for each bash script through its real invocation path
(not `bash -n`).

**F-9. Benchmark baseline is a single package.** `catalog-api/tests/benchmarks/baseline-2026-04-12.md`
records one package (middleware) with a GOMAXPROCS=3 setting; there are no baselines for the other
17 benchmark files, and none for other languages. SC-011 needs baselines for sign-in, browse, search,
playback start, scan and each client start-up (section 6.8).

**F-10. Evidence mechanism is per-script.** `ab_pass_with_evidence` is defined separately inside
several `full_automation` scripts (for example `catalog_aggregation_granularity.sh:139`,
`catalog_books_comics_resume.sh:177`). Duplicated helpers drift; the plan consolidates on one
recorder (document 06).

## 6. Closing every absent or partial cell (work packages)

FR-009 says an absent type is a finding fixed by writing it, not by recording a plan. Each work
package below is a unit of test writing with its oracle, evidence and exit criterion. Packages are
ordered by risk (11.4.132): most-reopened and critical-path first, and independent packages run in
parallel streams (11.4.103, 11.4.230) within the host ceiling.

```mermaid
flowchart TD
  A[TS-00 Inventory and applicability map] --> B[TS-01 Measurement baseline run]
  B --> C[TS-02 Evidence recorder adoption]
  C --> D1[TS-03 HelixQA bank conversion]
  C --> D2[TS-04 Replace fakes with real infra]
  C --> D3[TS-05 Contract tests and can-i-deploy]
  C --> D4[TS-06 Mutation instrumentation]
  D1 --> E[TS-07 Absent-type authoring per application]
  D2 --> E
  D3 --> E
  D4 --> E
  E --> F[TS-08 Performance baselines SC-011]
  E --> G[TS-09 Critical-invariant scenario sets]
  F --> H[TS-10 Matrix gate: zero gaps]
  G --> H
```

### TS-00. Inventory and applicability map

Write `specs/001-full-project-audit-remediation/matrix/applicability.yaml` (format in section
13.2) from a reading of each application's responsibilities. Re-count `helix_qa` without
`tools/opensource`. Classify every shared module for the `?` cells. Output is data; no test code.
Exit: every one of the 15 types x every component has P, ~, A or n/a with a reason, and a script
re-derives the counts (section 13.1) so the matrix is regenerated, not hand-edited.

### TS-01. Measurement baseline

Run every existing suite once, containerised, to learn pass/fail and measure coverage
(section 7.2). Failures found become register items (FR-008). This package is the only way to
move the `?` and the "UNKNOWN:" markers; no later package begins from an unmeasured state.

### TS-02. Evidence recorder adoption

Replace the per-script `ab_pass_with_evidence` copies and add recorders to the Go, vitest, Gradle
and cargo runners per document 06. Without it, no later test can satisfy FR-010's "measured and
recorded" requirement.

### TS-03. Executable HelixQA banks (finding F-1)

For each of the 1178 markers choose one of three dispositions, recorded per case:

1. **Convert** to a real action the HelixQA runner executes (for API banks, an HTTP step with
   method, path, body and an assertion on status and a body field; for UI banks, a driven action
   with a vision or OCR assertion, 11.4.117, 11.4.193 forbids blind typing).
2. **Merge** into an existing real test when the case duplicates one (cite the test path).
3. **Close with evidence** when the case describes behaviour the product does not have
   (false positive) or cannot have (structurally impossible); FR-008 allows this only with evidence.

Bank cases whose target is a physical device or an external service are executed against the real
one or reported `blocked-unavailable` (section 10), never converted into a simulated step.
The conversion order follows risk: API negative-paths and comprehensive (513 markers, authentication
and authorisation edge cases first), then web (256), then the mobile and desktop banks.
Exit: the scan `PROSE_HELIXQA_ACTION` returns zero findings, and the HelixQA runner executes
every case in every bank with a recorded verdict per step (not only a session-level summary).

### TS-04. Replace fakes with real infrastructure (finding F-2)

- Retire `catalog-api/tests/mocks/*_mock_server.go` from non-unit suites. Real FTP, SMB, WebDAV, NFS
  and object-store servers already exist as compose services (`docker-compose.test-infra.yml`,
  `deploy/infra-compose-test.yml`). Pin each image by digest (11.4.246, 11.4.264) and replace
  `:latest`.
- `mockProvider` in `provider_pipeline_test.go` becomes either a recorded real call to the real
  metadata provider (FR-025: real on every run, blocked when the credential is absent) or the test
  moves into a unit test directory where a stub is legal.
- Keep `httptest` only where it hosts the system under test inside the test process and the system
  is the real handler stack; it is not a fake. The anti-bluff scan flag `GO_HTTPTEST_ABUSE` marks
  suspect uses.
- A pre-build scan (the existing `GO_MOCK_IN_INTEGRATION` kind) must fail the local gate on any
  remaining violation, with its paired mutation (plant a fake, the scan must fail).

Mock servers may remain under unit tests of the protocol client code only, where the unit boundary
is the client parser.

### TS-05. Contract tests and can-i-deploy

Section 9.

### TS-06. Mutation instrumentation

Section 8.

### TS-07. Authoring the absent cells per application

For each A in the matrix, write the types marked A. The plan names the shape of each, the oracle, the
real environment and the evidence class. A cell is closed only by a test that has a recorded RED on
a deliberately broken artifact (11.4.224 A, document 06 section 4) and a GREEN that holds across
three runs.

**A1 catalog-api.**
- Scaling: a compose profile that runs N replicas of the API behind the real reverse proxy used in
  `docker-compose.yml`, increases load in steps (k6 ramp, reuse `tests/k6/breakpoint_test.js` shape),
  adds and removes a replica mid-run, and asserts error rate and p95 against the SLO recorded in
  TS-08. Oracle: specified SLO. If SQLite is the only supported store and replicas cannot share it,
  the scaling type is classed n/a with the architecture reason, and the architectural decision is
  recorded as a finding for the owner (DR-6). `UNKNOWN:` which holds until the storage mode is read.
- Security: extend beyond two files: authentication matrix (login, refresh, expired, revoked,
  concurrent), authorisation per route, injection corpora for every input handler, secret-leak scan
  over built images, native fuzz targets for each parser (eight fuzz files exist), dependency CVE scan
  recorded as evidence (gosec, nancy, trivy already scripted).
- Chaos: `tests/integration/chaos_test.go` exists; add process kill during scan, database file
  corruption, disk full on the data volume, clock skew, and network partition to the SMB/FTP
  targets using the real servers; assert categorised errors and recovery (11.4.85).
- DDoS: k6 `ddos_ratelimit_test.js` exists; add the assertion matrix (limit hit returns the
  documented status, back-pressure engages, service recovers within a recorded bound).
- E2E: build the real binary inside the build container, start it with a real database, drive the
  user-visible flows over HTTP; replace "partial" by running all four `e2e_binary` files in the
  containerised runner and extending to scan-a-source and playback-start.

**A2 catalog-web.**
- Integration: component tests with a real API in a container, replacing mock service layers in
  non-unit suites (`catalog-web/src/__tests__/AuthFlow.integration.test.tsx` is the only
  integration-named file found; its API handling is `UNCONFIRMED:`).
- Full automation: a Playwright suite orchestrated by one runner, no manual step, driving every
  feature x platform cell of the feature ledger (the ledger itself is built in TS-00).
- Security: DOM XSS corpus, token storage checks, CSP headers, dependency audit.
- Chaos and stress: API loss mid-session, WebSocket disconnect/reconnect storms
  (`websocket-realtime.spec.ts` exists), large-library rendering.
- Performance and benchmarking: Lighthouse budgets (`catalog-web/lighthouserc.json` exists) plus
  Core Web Vitals recorded as evidence; microbenchmarks for hot components.
- Challenges: add a web Challenge entry per feature, executed by the Challenges runner.
- UX: expand accessibility checks (`accessibility-expanded.spec.ts` exists) and i18n.

**A3 desktop and A4 installer.** Both Tauri. Unit tests exist; Rust tests exist; integration, E2E,
full automation, security, chaos, stress, performance, UI, UX and Challenges cells are absent or
partial. Plan: a Playwright and WebDriver-driven suite against the built desktop binary (built in a
container, run in a container with a virtual display) for UI and E2E; the installer is driven
through its real installation steps on a clean container rootfs (11.4.108, 11.4.139) with captured
filesystem deltas as evidence; Rust integration tests in `src-tauri/tests`; security: command
surface review of the Tauri allow-list and fuzzing of IPC payload parsers; chaos: kill during
install, full disk, interrupted download. Screen automation uses the vision oracle (11.4.117); typing
is always preceded and followed by captured screen state (11.4.193).

**A5 android and A6 androidtv.** Instrumented (androidTest) and Compose UI tests on a real device or
emulator. Emulator tests run against the real API in a container; they are real-device tests under
FR-025 when the claim concerns hardware behaviour (TV remote, HDMI), so the emulator is allowed only
for types where the owner confirms equivalence (DR-4). Add: sign-in to playback journeys
(11.4.143), security (cleartext traffic, token storage, exported components), chaos (network loss,
process death and restore), stress (rapid navigation), performance (cold start, scroll jank via
Macrobenchmark), UI golden-master via host-side render (11.4.170: Roborazzi or Paparazzi class,
`UNCONFIRMED:` which fits this Gradle setup), Challenges and HelixQA banks (Android TV banks are
already executable, which makes TV the model for the Android conversion).

**A7 api-client.** Integration against the real API in a container (every endpoint method), contract
tests as consumer (section 9), security (token handling, error-message hygiene), chaos (timeouts,
partial responses), property-based round-trip tests over the generated types.

**A8 Website.** The site is a static VitePress build. Types that apply: unit (n/a, no logic found),
E2E and UI/UX (build in a container, render every page in three engines, check links, console
errors, responsiveness and SEO per 11.4.190), performance (Lighthouse), and the document tests of
FR-013/FR-014 (links, rendered diagrams). Security: header and dependency scan. DDoS, scaling,
chaos, stress, benchmarking: n/a with reason (static assets served by a CDN or file server; the
platform's concern is the host).

**A9 Build.** Executing tests for each of `orchestrator.sh`, `hash.sh`, `version.sh`, `common.sh`
through their real invocation paths (11.4.224 A), using a temporary sandbox directory, asserting
exit status, output and file effects; stress and chaos on the orchestrator (kill mid-build, disk
full in the build directory, concurrent invocations) because it coordinates builds.

**A10/A11 shared modules.** Per module, after TS-00 classification, write the missing types in the
module and push to its upstream (FR-006). The default minimum is unit, integration, security
(where the module handles input), Challenges and a benchmark for performance-critical ones. Modules
that are pure configuration or asset collections are classed n/a for the types that have no
meaning, with the reason.

### TS-08. Performance baselines (SC-011)

Define the critical operations list from the spec: browsing, search, playback start, scanning a
source, sign-in, and each client start-up. For each: a benchmark or k6 scenario, a recorded baseline
under `specs/001-full-project-audit-remediation/evidence/performance/`, a target set with the
owner (targets are decisions, not derived values; no number is proposed here), and a regression
gate that fails when a run is outside tolerance of the baseline. Statistical method: at least
10 samples per operation (a count to be validated by variance in the first run, `UNKNOWN:`), median
and p95 reported, comparison by a non-parametric test, with the reference-hardware identity
recorded in the evidence so a baseline from another machine is refused. Baselines are
per-hardware; the existing middleware baseline (GOMAXPROCS=3, one package) is the shape to
generalise.

### TS-09. Critical-invariant scenario sets (11.4.239)

Declare, as data, the work classes for this system: INTEGRITY (the database layer and migrations,
file moves/renames in storage operations), AVAILABILITY (sign-in, catalog browse, playback start),
SAFETY (none found; marked n/a with reason unless the desktop or installer touch hardware),
MONEY (none found). For each, write Given/When/Then failure-path scenarios before the fix of any
defect in that class (fresh, retry, same-key-different-payload where idempotency applies,
crash-in-the-middle). The status-write seam refusal (11.4.146 D3) is part of the register design
in the companion plan, not here; this package supplies the scenario sets it cites.

### TS-10. Matrix gate

Section 13.

## 7. Coverage phase-in per application (FR-011)

FR-011 requires: a recorded baseline per application; a dated target per application; no application
below its recorded baseline at any time; during phase-in, the 85% floor applied in full to new and
changed code, with existing code held to baseline and target. Appendix 11.4.224 adds that coverage is
necessary and never sufficient, requires an instrument for every language, and requires
an exclusion list fence. Brownfield adoption of the ratchet is the option the owner already chose
(per-application phase-in, spec Q3), so no further operator question is open on that point.

### 7.1 Instruments per language

| Language | Applications | Instrument | Granularity | Notes |
|---|---|---|---|---|
| Go | A1, A10 | `go test -covermode=atomic -coverprofile` + `go tool cover -func` | statement | `-covermode=atomic` is needed with `-race` and parallel tests; present in the toolchain (go 1.26.0 on the host) |
| TypeScript/React | A2, A3, A4, A7, A11 | vitest with `@vitest/coverage-v8` | line, branch, function, statement | align the major versions first (web 4.x vs 0.34); a threshold failure must fail the run |
| Kotlin | A5, A6 | JaCoCo (tasks already registered) | line, branch | add `jacocoTestCoverageVerification`; coverage of Compose lambdas can mislead; androidTest `.ec` files must be merged to include instrumented tests |
| Rust | A3, A4 (src-tauri) | `cargo llvm-cov` (preferred) or `cargo tarpaulin`; `UNCONFIRMED:` which installs cleanly in the build container | line, region | run in the containerised toolchain |
| Bash | A9, A13, all scripts | PS4 line trace: `PS4='+COV:${BASH_SOURCE##*/}:${LINENO}:' bash -x script` producing executed lines divided by executable lines; kcov/bashcov are absent on the host per 11.4.224 E and may be installed in the build container if a real tool is wanted | line only | no branch coverage; `set +x` regions and traps are unaccounted; state these limits in the report |
| SQL, YAML, Markdown | schemas, configs, docs | none | n/a | behavioural tests of consumers instead (11.4.224 A, behaviour-bearing config) |

A language with no working instrument fails the gate; it is never given an invented percentage
(11.4.224 E). Where a stage such as the Kotlin merged unit+instrumented coverage cannot be measured
on a given runner, the honest result is an `blocked-unavailable` or recorded shortfall, not a skipped
check.

### 7.2 Baseline measurement procedure

1. Run in the build container with the dependencies cached, one application at a time within the
   host ceiling (12.6 and 12.12; section 12.3 computes the parallelism).
2. Run each suite **three times** and compare (FR-010). A suite whose three runs disagree on pass
   or on measured coverage is flaky: it enters quarantine (section 11) and its coverage is measured
   from the stable subset.
3. Write a **baseline record** per application (schema below) to
   `$EV/coverage_baseline/<app>/baseline.json` (`$EV` = `specs/001-full-project-audit-remediation/evidence`) through the
   evidence recorder (document 06). The record is produced by the measuring harness, never edited.
   Revision 3: not `evidence/coverage/<app>/`, which the unanchored `coverage/` rule at `.gitignore:139` ignores at any depth,
   so a baseline there would never be committed (docs/21 IC-38; tasks.md T004 keeps `$EV/coverage/` ignored on purpose).
4. Commit the record. The baseline commit hash is the reference for the ratchet.

Baseline record, with illustrative values only (NOT EXECUTED; the numbers are placeholders):

```json
{
  "schema": "coverage-baseline/1",
  "application": "catalog-api",
  "instrument": "go-cover-atomic",
  "measured_at": "<utc timestamp from the run>",
  "source_commit": "<git sha of the measured tree>",
  "scope": {"include": ["./..."], "exclusion_list": "coverage/exclusions/catalog-api.yaml"},
  "runs": 3,
  "line_percent": "<measured>",
  "branch_percent": null,
  "per_package": {"handlers": "<measured>", "services": "<measured>"},
  "evidence": ["$EV/coverage_baseline/catalog-api/run-1.json", "run-2.json", "run-3.json"]
}
```

### 7.3 The two-part rule

```mermaid
flowchart LR
  A[Change set] --> B{Which lines}
  B -->|new or changed| C[Diff coverage must be 85 percent or more]
  B -->|existing, untouched| D[Application total must not fall below recorded baseline]
  C --> E{Both hold}
  D --> E
  E -->|yes| F[Gate passes]
  E -->|no| G[Gate refuses with named lines or package]
  F --> H[Target check on the dated milestone]
```

- **New and changed code**: diff coverage at or above 85%, computed from the instrument's per-line
  output joined with `git diff` hunks of the change. A change that moves a line counts as changed.
- **Existing code**: the application-wide figure must never be lower than the recorded baseline.
  The comparison uses the same instrument, the same scope and the same exclusion list; a changed
  exclusion list resets nothing and is reviewed (below).
- **Ratchet**: when the application figure rises, the baseline moves up in a committed record with
  the evidence of the run that earned it. It never moves down except by the governed repeal
  described in 11.4.227 (A), which cites a removal reason.
- **Targets**: the dated target is a per-application commitment of the form "total coverage at least
  X by date D", where X and D are chosen by the owner after the baseline run and recorded in
  `$EV/coverage_targets/<app>/targets.yaml` (revision 3: a separate tracked folder, not a sibling of the baseline record). This plan sets the method and the deadline mechanism, not
  the numbers or dates, since a baseline does not exist yet (the figures for the final target are
  `UNKNOWN:` and are not invented here). Method for proposing X: the constitution's aim is close
  to 100% with a floor of 85%; the proposal to the owner is the lowest-effort line that reaches 85%
  by the final milestone using the per-package gap listing from the baseline, ordered by risk
  (critical paths first), with intermediate milestones at equal intervals between baseline and
  final date. Intermediate dates and values are the owner's decision.
- **Exclusion fence** (11.4.224 E): exclusions only for generated code, vendored third-party and
  non-shipping fixtures, each entry justified in a checked-in list; first-party exclusions need a
  tracked item naming the plan to bring them into scope. An unlisted exclusion voids the measured
  figure.
- **Numerator rule** (11.4.224 amendment): only red-capable tests contribute to the measured
  numerator. A test counts toward coverage only if it has a recorded failing-first run or a paired
  mutation observed to fail it; assertion-free tests are rejected by construction. This is how
  coverage is prevented from being gamed (the founding measured fact: a line-granular instrument
  counted an `if/else` line covered while the `else` branch never ran). Implementation: the coverage
  gate joins the coverage output with the mutation or RED evidence index (document 06 section 5);
  a line covered only by tests with no red-capability flag is counted as uncovered for the gate
  (and reported separately so effort can be directed there).
- **Commit-push stage (revision 6; tasks.md T503, document 16 §12.2.6)**: the gate is wired as a named
  stage of the commit-push script with its `scripts/repo/validate_checks.tsv` row `coverage_gate`,
  scope `changeset`: it judges the changed lines and the application totals of the change set through
  the T200 exclusion fence and takes no declared-file list, so it has no row in the path-class table.

### 7.4 Per-application phase-in plan

| Application | Instrument | Existing config | First actions |
|---|---|---|---|
| A1 api | Go cover | `scripts/track-coverage.sh` (swallows errors) | rewrite the collector to fail on error; collect per package; record baseline; wire diff coverage |
| A2 web | vitest v8 | thresholds 80 in `vitest.config.ts` | raise threshold to the floor for new code through the diff gate; keep application baseline |
| A3 desktop, A4 installer | vitest v8 (`^0.34.0`) + Rust | script present, no threshold | upgrade vitest to the repository's major; add thresholds from baseline; add cargo coverage |
| A5, A6 | JaCoCo | task present, no verification | add verification rule from baseline; merge androidTest `.ec`; instrumented runs need a device (FR-025) |
| A7 client | vitest v8 | none | add provider and thresholds |
| A8 Website | n/a executable code | none | document exclusion rationale; measure link and render checks instead |
| A9 Build and bash scripts | PS4 trace | none | build the trace harness as a script with its own tests (11.4.224 A applies to the harness) |
| A10/A11 | per language | per module | measure each module; push the record to the module's repo |

The coverage gate itself is an executable artifact and is under 11.4.224 (A): its test-first
artifact is a paired mutation (lower a figure, add an uncovered changed line, add an
assertion-free test; each must make the gate fail), per section 8.

## 8. Mutation testing and the SC-005 sampling procedure

SC-005: zero accepted tests that still pass when the behaviour they protect is deliberately broken,
measured on a reviewer-drawn sample. FR-010 requires every test to be shown to fail when the
behaviour is broken. Principle II requires a mutation score of at least 85%. Two layers are needed:
automated mutation analysis as a bulk instrument and the human-drawn sample as the acceptance test.

### 8.1 Instruments

| Language | Tool | State in repository | Plan |
|---|---|---|---|
| Go | go-mutesting (referenced by `mutation_ratchet_challenge.sh` in three submodules) | config `.go-mutesting.yml` and baseline `challenges/baselines/bluff-baseline.txt` absent in the main repository | create both; `UNCONFIRMED:` the maintained fork and Go 1.25 compatibility, to be established in the build container |
| TypeScript | Stryker Mutator | none | add for api-client, web (selected packages), shared TS modules; vitest runner plugin |
| Kotlin | PIT (pitest) with the Gradle and Kotlin plugins | none | `UNCONFIRMED:` Compose and coroutine compatibility; scope to logic layers (repositories, view models) |
| Rust | cargo-mutants | none | `UNCONFIRMED:` run time on Tauri crates; scope to non-UI modules |
| Bash | purpose-built mutation harness | none | a script that applies a table of operators (flip `-eq`/`-ne`, delete a command, swap `&&`/`\|\|`, change an exit code) to a copy of the script and runs its test; included in the Build framework tests |

Mutation analysis is expensive; it runs on changed files per change and in full on a schedule
(per package, in parallel streams) in the build container within the memory ceiling. Equivalent
mutants are documented, not hidden (11.4.107 point 10 says behaviourally-equivalent mutants are
expected not to flip).

### 8.2 The paired mutation per test (11.4.224 A, 1.1)

Every new test ships with the specific mutation that must flip it, recorded in the test's evidence
record (`mutation: {operator, location, result: "caught"}`). The mutation is the revert of the
fix commit for a regression test (11.4.115 F), never only a deletion of the strings the test looks
for (a tautological mutation is refused). The recorder (document 06) rejects a test record that
lacks a caught mutation.

### 8.3 Sampling procedure for SC-005

1. The reviewer, not the author, draws a random sample. The draw is reproducible: sample size n and
   a seed recorded; the population is the list of test cases in the evidence ledger; the sampler is a
   script run by the reviewer's own tooling (document 06 section 6 describes the independence
   requirement). Sample size: `UNKNOWN:` to be set by the owner and reviewer; a proposal in the
   absence of data is that n be large enough that a defect rate of 2% would be seen with high
   probability, with the formula and the resulting n recorded, not asserted here.
2. For each sampled test the reviewer chooses and applies a mutation the author did not write (the
   11.4.194 (6)(d) rule: author-written mutations are necessary but not sufficient), runs the test
   three times, and records caught or survived.
3. A survivor is a finding: the test is weakened or rewritten, and the draw is repeated with a fresh
   seed over the corrected population until a sample yields zero survivors (SC-005 requires zero).
4. Evidence: the ledger entries, the mutation diffs and the three-run outputs, chained and anchored
   (document 06 sections 7 and 8).
5. Two verdict scopes (revision 11, tasks.md rev 27 T570, T571, T572, T580b). A survivor in a test
   inside `submodules/constitution` cannot be fixed by the producing stream before the final
   constitution update, so T571 writes first a `scoped` GO (`go_scope: scoped`) that releases the
   covered T572 fix commits and lists each such survivor in `routed_survivors` as
   `routed_to_T580b`, and only after the T580b (3c) re-check records it caught writes the `final`
   GO. Every re-draw verdict passes `scripts/qa/sc005_verdict_check.py <verdict> --ledger
   $EV/ledger.jsonl` (T570, test-first, with a paired mutation; revision 15, tasks.md rev 31 round-30 review I3: a release-seam script, its change set adding it to `scripts/release/release_seam_files.txt`, because HC-6 (T580), the closing T580a (d) iteration and T582 gate on its `--require-final` output, so a later change to it is held at S2; section 13.5) before it is committed:
   `survivor_open` (a `scoped` GO with an open survivor outside `submodules/constitution`, or a
   `final` GO with a routed survivor no cited ledger entry records caught), `survivor_misrouted` (a
   routed survivor whose test lies outside `submodules/constitution`) and `go_scope_missing`.
6. Closure (revision 12, tasks.md rev 28 T570, T571, T580, T580a, T582). Every T571 verdict passes
   the check, the first draw included (a NO-GO carries no `go_scope` and passes on that ground
   only), and the check also refuses `final_with_open_survivor` (a `final` GO with an open, unrouted
   survivor outside `submodules/constitution`) and `scoped_without_routed` (a `scoped` GO with no
   routed survivor, which must be written `final`). Each verdict records `candidate_fingerprint`,
   and a `final` GO cites, per routed survivor, the T580b (3c) commit and the test's git blob sha
   there. The closure mode `--require-final --candidate <fingerprint> --constitution-rev <sha>`,
   run at T580, at the closing T580a iteration and at T582, refuses `final_go_required` (the latest
   GO is `scoped`, or no GO exists), `sc005_candidate_mismatch` and `final_go_stale` (a later (3c)
   iteration changed a routed test after its re-check). A re-cut routes a constitution-test
   survivor through a (3c) iteration and a T580a iteration (`sc005_recheck_owed`). The first GO
   lists the T570 change set's `CPA-Run` id in `covers_runs` (else `verdict_covers_incomplete`).
7. Iteration files (revision 16, tasks.md rev 32 T570, T571, T572, T580, T580a, T580e, T581, T582;
   round-31 review P5-P7 I1). A GO file is final (a declared change to it is refused 20
   `verdict_already_go`, T042 S5), so the first T571 draw writes `$EV/reviews/WP-71-sc005.json`
   (iteration r1, the open iteration), a re-draw writes into the open iteration while it holds no GO
   (a NO-GO re-draw replacing its NO-GO, the replaced verdict kept as an `ev/1` entry), and once a GO
   is written there the next draw (the `final` GO after a `scoped` one, or a re-draw on a re-cut
   candidate) opens `$EV/reviews/WP-71-sc005-r<n>.json` (n = 2, 3, ...). T572 fix commits are held
   on the open iteration. The latest T571 verdict, read by T580, T580a and T582, is the iteration
   with the highest number (the base file counting as r1), and the closure mode also refuses
   `sc005_not_latest` for a verdict file below a higher sibling iteration (fixture `sc005/not_latest`:
   a `final` GO in the base file beside a `scoped` GO in `-r2` is refused `sc005_not_latest`, the
   same check given `-r2` `final_go_required`); fixture `sc005/second_go` (real CPA S5 path in a
   scratch clone) shows a second GO written into the base file, and a fix held on it, refused
   `verdict_already_go`, while the same fix held on an opened `-r2` is accepted with exit 14; each
   with a paired mutation. Every iteration is a record of the general commit window and of T581 (0).

The reviewer-written mutation also serves as the independent check on the automated scores: a
tool-reported 85% with surviving reviewer mutations is a sign that the operators are too weak, and
the discrepancy is itself a finding.

## 9. Contract tests on both sides and can-i-deploy (FR-016)

### 9.1 Contract inventory

| Contract | Provider | Consumers | Definition | Test status |
|---|---|---|---|---|
| REST API v1 | catalog-api (`/api/v1/...`) | catalogizer-api-client (TS), catalog-web, catalogizer-desktop, installer-wizard, Android (Retrofit), Android TV, scripts and HelixQA banks | `docs/api/openapi.yaml` (currency vs routes: `UNCONFIRMED:`) | provider-side shape test only (`catalog-api/tests/integration/contract_test.go`) |
| WebSocket `/ws` | catalog-api `wsHandler` (`catalog-api/main.go:1084`) | catalog-web (`websocket-realtime.spec.ts`), clients using the shared WebSocket client module | message types in code; no schema file found | none found |
| Database schema | migrations in `catalog-api/migrations` (2 files found), plus schema created in code | the API only (internal) | `UNCONFIRMED:` where the full schema lives | covered by integration tests only |
| Shared module APIs | each Go and TS submodule | catalog-api, web and client applications | Go module APIs, TS exports | per-module unit tests; no consumer tests |
| Auth tokens (JWT claims) | catalog-api | all clients | code | partial in security tests |
| Persisted client data (Room schema, local storage keys) | Android apps, web | their own upgrade paths | Room schema JSON | upgrade tests `UNCONFIRMED:` |

Revision 13 (tasks.md rev 29, T320, T324, T325, T335a): the binding inventory is `$AUD/contract/inventory.json` (T320, CT-1..CT-22 from docs/01 §14). A side without a test path carries `n_a_reason` with `n_a_status` `final` (with `n_a_source`, the owner decision record or the docs/01 entry, exempting that consumer from the T325 missing-contract refusal) or `pending` (with the open decision id, for example `ODG-24`, never exempting it). Before the T336a review, `scripts/contract/inventory_paths_check.sh` (T335a) checks at the reviewed commit, never the working tree, that every named test path exists as a blob there (`inventory_row_missing`, `inventory_test_path_absent`, `inventory_na_unsourced`; a pending n/a reported `inventory_na_pending`, which never meets the P4 exit), its verdict written to `$AUD/contract/inventory-paths.json`. Revision 16 (tasks.md rev 32 T320, T335a, T336a, T324, T334a, T383, T399; round-31 reviews P3-P4 I-2, I-3, m-1 to m-3): every row carries `dependentRequired: {n_a_reason: [n_a_status]}`, a row with `n_a_reason` and no `n_a_status` refused `inventory_na_status_missing`, and each n/a `if` holds `required: [n_a_status]`; an empty `--require-ancestor ''` is refused `inventory_provider_commit_missing` (fixture `ancestor_empty`); the T336a stale-run rule also binds a change set that adds, renames or deletes a test path the inventory names; T324 checks both the `--root` tree and the repository tree for writes by the gate under test; and when a fix renames a test file the inventory names (T334a, applied in T383 and T399), the same change set replaces the path, held on a new WP-41-gate iteration of T336a (docs/04 §4, revision 21). Revision 17 (tasks.md rev 33 T320, T324, T325, T335a, T336a; round-32 review P3-P4 M6): the n/a fields live at one level, the side; a row holds a `provider` side object and a `consumers` list of side objects, both typed by one `$defs/side` that carries a `test_path` or `n_a_reason` and holds `n_a_reason`, `n_a_status`, `n_a_source` and `n_a_decision` (a final n/a exempts that consumer side only), a row-level `not` refusing any of the four on the row; T335a's fixture `na_on_row` FAILs `inventory_schema_invalid` and never passes, and the rules of revision 16 apply within the side (docs/04 §4, revision 22). Revision 18 (tasks.md rev 34 to rev 36 T320, T324, T325, T335, T335a, T336a; round-33 reviews P3-P4 I1, I6, M5, round-34 reviews P3-P4 B1, B2, I5, round-35 reviews P3-P4 I4, I5, m2): every side requires `contract_kind` (`pact` or `test`) and a `test_paths` array of at least one concrete file (the single `test_path` of revision 17 withdrawn) or an `n_a_reason`; every consumer side requires `consumer`, the consumer name of its pact files, unique within its row (`inventory_consumer_duplicate`), and the provider side `name`; T325 judges only `pact` sides and per (consumer, provider) pair, never by the consumer name alone (fixture `pair_missing`, providers `prov-alpha` and `prov-beta`, asserted by `jq` pair equality; fixture `test_side`; under `--provider X` only pairs of provider X); T325 reads `compat-window.yaml` and `inventory.json` from the approved export, refused `release_seam_unreleased` with `CPA_APPROVED_DIR` unset; T335 makes its verdict in the pre-release production form (`$AUD/contract/can-i-deploy-pre-release.json`, section 13.5), the real post-release run listed at the WP-41 checkpoint; the T336a `reviewed_files` name `can_i_deploy.sh`, `compat-window.yaml` and `inventory.json` with their sha256 at the tip of `covers_runs` (docs/04 §4, revision 23).

### 9.2 Approach and decision

Decision record DR-1 (section 14): adopt consumer-driven contracts with the Pact family for the
REST API and WebSocket messages, hosted on a self-hosted broker in a rootless container, with
OpenAPI validation as the provider-side schema check; reject (a) spec-only validation without
consumer contracts and (b) a hand-built matrix. Reasons: the constitution describes the
consumer-driven pattern with a broker matrix and can-i-deploy (11.4.244); a spec-only approach cannot
detect a consumer using an undocumented field; building our own broker re-implements a maintained
tool, against 11.4.74 (extend, do not reimplement). Constraint: Pact libraries for each consumer
language (TypeScript, Kotlin/JVM, Rust, Go provider verifier) must exist and fit; this is `UNCONFIRMED:`
until each is tried in the build container. If a consumer language lacks a library, the fallback is a
contract file generated from that consumer's recorded real HTTP traffic against a real provider
and replayed by the provider verifier; the file format is Pact JSON either way, so the broker and
gate are unchanged.

```mermaid
sequenceDiagram
  participant C as Consumer tests (client, web, android, tv)
  participant B as Contract broker (rootless container)
  participant P as Provider verification (catalog-api)
  participant G as can-i-deploy gate
  C->>B: publish contract with consumer version and tag
  P->>B: fetch contracts for all deployed consumer versions
  P->>P: replay each interaction against the real running API
  P->>B: publish verification result with provider version
  G->>B: query compatibility matrix for provider X and consumer set
  alt every cell green
    B-->>G: verdict compatible
    G-->>G: write machine verdict, allow release step
  else a cell red or a contract missing
    B-->>G: verdict incompatible or unknown
    G-->>G: write machine verdict naming the pair, block
  end
```

### 9.3 Requirements derived from 11.4.244

- Tests exist on **both** sides: consumer tests produce the contract; the provider verifies every
  deployed consumer's contract against the real API (the real handler stack on a real database in a
  container, not an in-memory engine and not a mock).
- A red cell blocks; there is no warn-and-continue. A **missing** contract is a first-class refusal
  reason (11.4.201): a consumer in the inventory without a published contract makes the gate
  refuse, so adding a client forces writing its contract.
- A patch-version bump that breaks a contract is a mislabelled release; the gate compares the version
  delta against the contract result.
- The backward-compatibility window is declared data: current plus N previous minor versions,
  the value of N set by the owner; consumers outside the window are listed for an owner decision.
- The broker runs as a rootless container started by the containers module (11.4.76, 11.4.161); its
  image is pinned by digest; its database is on a volume the evidence tree records.
- The gate produces an evidence record (document 06 section 3) naming provider version, consumer
  versions, matrix cells and verdict, and is itself guarded by paired mutations: break a consumer
  expectation (the gate must go red), delete a contract (must refuse), and run a compatible change
  (must not fire, the golden-false case of 11.4.201 (1)).
- Revision 7 (tasks.md T324 to T326, T335, T358, T567, T569 and T582 of the round-13 wave; their P6-P7
  release-seam rule): the gate `scripts/contract/can_i_deploy.sh` judges one provider version with
  `--provider catalog-api@<fingerprint>`, the fingerprint of the artifact whose provider verification results
  were published under it, read at run time from the running artifact (T326), never typed in. In P4 and P5 that
  is the artifact under test (T335, T358); at the release seam it is the T566 candidate: T567 re-runs the
  provider verification against the candidate and publishes it under that fingerprint, T569 judges the
  candidate and T582 re-judges it before the final verification is accepted, a refusal blocking the candidate
  exactly as a FAIL does. A provider version with no published verification results is refused (missing
  verification, never a pass), and the gate writes its verdict only to the file that its required `--out`
  option names, under `.audit/out/<op_id>/` at the release seam, so a release-seam run never writes the tracked
  tree (T324, T325). Those tasks read the pacts and the verification results from committed files
  (`tests/contracts/pacts/`, `tests/contracts/verifications/`, T325), so no broker is on that path; whether the
  broker of DR-1 is still adopted is UNCONFIRMED, to be decided when DR-1 is revisited against T325.

### 9.4 Shared-module contracts

For each Go module consumed by catalog-api, the consumer-driven contract is a compile and behaviour
test in the consuming repository's CI-free local gate that exercises the module's public API; for
the TS and React modules, contract files record the exported component props and client methods.
Changes to a submodule that fail the consumer's test are fixed in the module (FR-006) before the
pointer moves (11.4.26 step 7).

## 10. Real services, credentials and devices: the blocked-unavailable status (FR-025)

### 10.1 The rule

A test of behaviour that depends on an external service, a credential or a physical device runs
against the real one on every run. When the dependency is unavailable the test is reported
`blocked-unavailable` with the exact reason. That status counts as **not passing**, is **never** a
pass or a skip, is **never** replaced by a simulation, and the feature is not complete until the
owner supplies the dependency. The status is not a failure of the product, so it is not a defect
count; it is an open requirement on the owner. This resolves the tension between the owner's rule
and the governance's honest-skip allowance (spec assumption): the governance's `SKIP-with-reason`
becomes a `blocked-unavailable` record that blocks completion.

### 10.2 State machine for a test run

```mermaid
stateDiagram-v2
  [*] --> Preflight
  Preflight --> Blocked: dependency probe fails
  Preflight --> Running: dependency probe passes
  Running --> Passed: all assertions hold on three runs
  Running --> Failed: an assertion fails
  Running --> Flaky: runs disagree
  Flaky --> Quarantined: recorded with deadline
  Blocked --> Preflight: owner supplies dependency, next run
  Failed --> Running: fix applied, new run
  Quarantined --> Running: stabilised
  Passed --> [*]
```

Rules:

- The **preflight probe** is itself a recorded evidence entry: it names the dependency, the probe
  command and result, and its own timestamp (never a cached answer). It uses the real dependency's
  identity (for a device, the stable serial or id, 11.4.111; for a service, a request that proves
  the correct service identity; for a credential, an authenticated call, without printing it).
- `Blocked` carries a **closed reason code**: `service_unreachable`, `credential_absent`,
  `credential_rejected`, `device_absent`, `device_wrong_identity`, `device_unauthorised`,
  `geo_restricted`, `quota_exhausted`, `licence_absent`, `host_resource_unavailable`. Each reason
  has a human-readable detail field, `blocked_detail`, with the observed values (credential
  variable *names* only, never values, 11.4.10).
- `blocked` is one of the four ledger verdicts (`pass`, `fail`, `blocked`, `error`; docs/06 §3.1
  rule 10), not an extension of pass/fail: the ledger entry has `verdict: "blocked"`,
  `blocked_reason`, `blocked_detail` and `counts_as: "not_passing"` (all four are required together
  by `contracts/evidence-record.schema.json` revision 3; `blocked_detail` and `counts_as` are
  refused on any other verdict); the matrix cell is `blocked`, and a non-empty set of blocked cells
  prevents SC-004 and completion (FR-008 treats blocked findings as open). A GREEN entry can never
  be `blocked` (the schema requires `pass` for GREEN); a run whose precondition probe failed before
  a GREEN is recorded as a blocked attempt, not as a GREEN repetition.
- The status never degrades to skip: the reporter has no `skipped` outcome for dependency absence,
  and a test that returns before asserting (an early return) is detected by the anti-bluff scan
  (`GO_NO_ASSERT`).
- No simulation may substitute: if the dependency is real-service X, the test file must not
  contain the stub; the pre-build scan fails on a fake in a non-unit test (TS-04).
- Owner-supplied dependencies are listed in `specs/001-full-project-audit-remediation/matrix/
  dependencies.yaml` (the one place the owner edits to unblock): service endpoint, credential
  variable name, device serial, with the tests that need each and the date supplied. Nothing secret
  is stored there.

### 10.3 Dependency inventory (from the repository)

| Dependency | Needed by | Real form available locally? | Reason code if absent |
|---|---|---|---|
| Real FTP, SMB, WebDAV, NFS, object-store servers | A1 protocol integration, chaos, E2E | yes, `docker-compose.test-infra.yml` containers (digest pin needed) | `service_unreachable` |
| External metadata providers (for example a movie database) | A1 provider pipeline tests, full automation | no, needs the provider's network and API key (credential names `UNCONFIRMED:`) | `credential_absent`, `service_unreachable`, `quota_exhausted` |
| Firebase (analytics, distribution) | `scripts/firebase_*`, Android apps | needs the owner's project; MCP tools exist for it | `credential_absent` |
| Physical Android and Android TV devices (stable serials) | A5, A6 instrumented, HelixQA, recording | no | `device_absent` |
| Desktop display for Tauri UI tests | A3, A4 | a virtual display in a container is real for rendering; hardware-specific claims need hardware | `host_resource_unavailable` |
| Large real media libraries | scan, performance | owner supplied path | `service_unreachable` |

### 10.4 Existing skips

The 113 `t.Skip` calls and any skip in the other suites are classified: (a) dependency absence becomes a
`blocked-unavailable` preflight; (b) platform or build-tag skip becomes a build-tag selection with
the excluded set listed in the matrix as n/a with reason; (c) a skip with no stated cause is a
finding. Skipping without an annotated reason is already caught by `SKIP_WITHOUT_TICKET`.

## 11. Flaky-test quarantine and protected regression specs

11.4.248 treats flakiness as the silent decay vector of the whole programme. FR-010 and SC-003 make
determinism a hard requirement, so this plan does not treat a retry as acceptable.

- **Detection**: the three-run comparison of every measuring run (document 06 section 5). A test
  whose verdict or evidence digest differs between runs is Flaky. Also the aggregator flags the
  rerun-after-red pattern in history (11.4.248 A).
- **Quarantine**: the test moves to `tests/quarantine/<application>/` (path is project data), a
  tracked item with a stabilisation deadline is opened (set by the owner; mechanism here), the
  blocking suite runs without it, and the quarantine count is a visible metric. A deadline past is
  an owner decision or a violation. The ideal quarantine is empty (11.4.50).
- **No retries**: remove `retries` from Playwright configuration, no `-count` loops that pass on any
  success, no `flaky` plugin. A passing rerun of a failed test is not a pass.
- **Protected specs**: regression tests that codify a fixed defect carry `[PROTECTED-SPEC: ATM-NNN]`
  in the language's comment form, referencing the register item. The tag is applied per test, not
  as a blanket. The project's reviewer rules (CODEOWNERS equivalent) require the designated
  reviewer for any change that touches a tagged test or the `regression/` path, so a weakening is
  visible and reviewable, not prevented. CODEOWNERS is a host-platform concept; the local
  equivalent is a stage of the commit-push script, never an automatic hook (revision 5, which
  resolves the revision 4 `UNCONFIRMED:` note on `scripts/hooks/pre-push-gate.sh`: 11.4.234
  forbids a blocking hook, document 16 §12.1 installs none and keeps that script unmodified and
  uninstalled): tasks.md T564 adds a `CPA` stage that refuses an unreviewed change inside the
  protected scope declared in `scripts/qa/protected_spec_scope.yaml` (tagged tests, the T473
  guard registry, any `regression/` directory), and a reviewed change is one whose review
  verdict covers it (document 16 §12.2.4). Revision 6 (tasks.md rev 12 T564; document 16
  §12.2.6): the stage is the registry row `protected_spec` with scope `files`, given the declared
  files whose path class applies it, with its rows in `scripts/repo/check_classes.tsv`: `yes` for
  `source` and `fixtures`, `no` for `patches` (a recorded diff may quote a tagged test and is never
  the test), `evidence`, `evidence-ledger`, `generated`, `legacy-collection` and
  `governance-carrier`, each with its reason, its golden-false fixture and its mutation.
- **Expected-RED guards are not quarantine** (revision 9; tasks.md rev 24 T334a, 11.4.135): a test
  committed RED on purpose before its fix (a consumer contract test that fails until P5, tasks.md
  T327 to T329) leaves its default lane only through the registered marker `expected-red: ATM-<id>`,
  in the form its runner selects on: TypeScript (vitest; `catalog-web`, `catalogizer-desktop`) a test
  file named `*.expected-red.test.ts` with the marker in its header comment, excluded by the default
  lane's vitest `exclude` glob and run by a guard lane; Kotlin and Java (Gradle, both Android trees)
  `@Tag("expected-red")` (JUnit 5) or `@Category(ExpectedRed::class)` (JUnit 4) beside the marker
  comment, excluded from the default `test` task and run by a guard task; Rust
  (`catalogizer-desktop/src-tauri`) `#[ignore = "expected-red: ATM-<id>"]`, skipped by the default
  `cargo test` and run by `cargo test -- --ignored <name>`. Each such test has a row in the standing
  guard registry `scripts/qa/guard_registry.tsv` (polarity `RED_EXPECTED`, the RED evidence on the
  pre-fix artifact fingerprint, the task that owes the GREEN), and the anti-bluff scan accepts a
  marker only when its ATM id has a guard row, reporting `SKIP_WITHOUT_TICKET` otherwise. The lane
  test runs one marked and one unmarked failing fixture per language: the default lane's machine
  report must list the unmarked test and not the marked one, the guard lane's report must list the
  marked one, and the unmarked failing fixture must make the default lane fail (control needle). Revision 16 (tasks.md rev 32 T334a; round-31 review P3-P4 I-2): a fix that renames a marked test file (for example `*.expected-red.test.ts` to its default-lane name) and whose old path `$AUD/contract/inventory.json` names replaces that path in the inventory in the same change set, a `release_seam` edit held on a new WP-41-gate iteration of T336a; whether the inventory names it is a recorded byte compare with a control needle, never assumed. Revision 17 (tasks.md rev 33 T383, T399, T413; central decision C1): the guard-registry row changes these fixes make (the polarity flip from `RED_EXPECTED` to GREEN) take effect in a CPA run only once the owner approves the fixing task's `G-GATE` GO in `cpa-host` (`$EV/hc/owner-trust/<n>.json`): `guard_registry.tsv` is keyed by check name and the approved row governs its key, so until that approval the HEAD row is a report line and the approved expected-RED row keeps deciding the lane.
- Causes of flakiness found by the three-run comparison are investigated to root cause (11.4.102)
  and fixed; they are not hidden by quarantine. Quarantine only isolates while the investigation
  runs.

## 12. Execution model: containers, ordering, determinism, host safety

### 12.1 Where tests run

Per the repository rules, builds and heavy tests run inside rootless containers (11.4.161, 11.4.173,
FR-021). The existing scaffolding is `scripts/build_in_container.sh`, `scripts/container-build.sh`,
`scripts/lib/auto-container.sh`, `scripts/lib/container-runtime.sh`, `docker-compose.test.yml`
and `docker-compose.test-infra.yml`. This plan reuses them and extends them only where a gap is
found (11.4.74). Podman is installed on the host; Docker is not found. The constitution says build
containers are distributed to a remote build host and artifacts brought back (11.4.173 text in
the project CLAUDE.md); whether a remote host is configured in this environment is `UNCONFIRMED:`
(`deploy/MIGRATION_thinker_local.md` suggests a prior migration).

Revision 8 (the plan owner's decision C1 of 2026-10-04, docs/21 ODG-07 revision 15): every
build, and every test lane that compiles or produces an artifact, runs in a rootless container on
the remote build host and is dispatched event-driven: the lane dispatcher submits it and returns at
once, its completion arrives as one authenticated `build-event/1` record consumed exactly once, and
the caller's next step runs as the registered callback, so other lanes and reviews proceed while it
runs (document 16 section 9.6; tasks.md T005a, T005b, T121a). Only interpreter lanes over the
read-only source (bash, python3, jq, sqlite3 gates and the documentation checks) run locally, in
images built remotely or pulled by digest; the site of each lane is the `site` column of
`scripts/containers/lanes.tsv`. With no qualified build host reachable, or a build whose heartbeat
stops past its budget, the lane is `blocked-unavailable` (`host_unreachable`, `no_qualified_host`,
`build_liveness_lost`), never a local run and never a pass (FR-025). The host identity is the
ODG-07 input.

### 12.2 The wrapper

A single wrapper script `scripts/test-in-container.sh <application> <type> [args]` (to be written
test-first; existing build helpers are the base) that: starts the application's toolchain image
(pinned by digest), mounts the source read-only and a results directory read-write, applies memory and
CPU limits computed by the host-safety helper, runs the command through the evidence recorder, and
writes artifacts under `evidence/`. The wrapper is the only entry point used by the plan, so a
direct host-side test run is detectable.

```mermaid
flowchart TD
  S[Request run of type T for application A] --> P[Host-safety preflight and memory budget]
  P --> I[Start pinned toolchain container, rootless]
  I --> D[Preflight: dependencies real and reachable?]
  D -->|no| BL[Record blocked-unavailable with reason, stop]
  D -->|yes| R1[Run 1 through recorder]
  R1 --> R2[Run 2]
  R2 --> R3[Run 3]
  R3 --> C[Compare runs: verdict, evidence digests, coverage]
  C -->|identical| V[Write verdict file, chain, anchor]
  C -->|differ| F[Flaky: quarantine path]
  V --> M[Update matrix cell from ledger]
```

### 12.3 Parallelism and host limits

Parallelism is `min(nproc, floor(budget_gb / per_job_peak_rss_gb))` where the budget is at most 60%
of RAM (12.6) and thread headroom is checked with `ulimit -u` before scaling (12.12). The per-job peak
RSS comes from the first baseline run's resource record (11.4.24), not a guess. Streams that touch
the same physical device are partitioned to a single owner (11.4.119). Mutation runs are the most
memory-hungry and run alone.

### 12.4 Order of execution (risk-first)

11.4.132: highest-risk first, then the rest. For this feature: (1) most-reopened and most recently
changed code; (2) authentication and authorisation, data integrity, playback path; (3) the
suites with real devices (scheduled with the owner); (4) everything else. The risk order is
computed from the register's reopen counts once it exists (companion plan) and from git history of the
changed files; until then it is the order above.

### 12.5 Commands (containerised, NOT EXECUTED)

```bash
# Go unit coverage of one package tree, three runs, evidence recorded
scripts/test-in-container.sh catalog-api unit -- \
  go test -race -covermode=atomic -coverprofile=/out/cover.out ./handlers/...

# Web unit with coverage
scripts/test-in-container.sh catalog-web unit -- npx vitest run --coverage

# Web E2E against the real API stack
scripts/test-in-container.sh catalog-web e2e -- npx playwright test --reporter=json

# Kotlin unit coverage
scripts/test-in-container.sh catalogizer-android unit -- \
  ./gradlew testDebugUnitTest jacocoTestReport

# Rust (src-tauri)
scripts/test-in-container.sh catalogizer-desktop unit -- cargo llvm-cov --lcov --output-path /out/lcov.info

# Bash line coverage (Build framework)
scripts/test-in-container.sh build unit -- scripts/bash-coverage.sh Build/tests/run.sh
```

`scripts/test-in-container.sh` and `scripts/bash-coverage.sh` do not exist; they are deliverables of
TS-02 and TS-07 and are written test-first. The tool names and flags above are the intended
interface, not verified against this repository (`UNCONFIRMED:`).

## 13. The matrix as a living, gated artifact

### 13.1 Source of truth

The matrix is generated, not written. A generator script reads (a) the applicability map, (b) the
evidence ledger (document 06) and (c) the repository, and emits `matrix/coverage-matrix.json` plus
`matrix/coverage-matrix.md` (revision 6, tasks.md T504: under `$EV/matrix/`; an `$EV/**/*.md` file is
class `source` of the commit-push path-class table, so the generator writes it with the 11.4.44
revision header, one final newline and no trailing whitespace). A cell is `present` only when the ledger contains, for that
application and type, at least one test record whose verdict is PASS with three identical runs, a
caught mutation, and an evidence class matching the type's required class (section 2); `partial`
when records exist but fewer than the applicability map requires; `absent` when none; `blocked`
when the preflight record says so; `n/a` when the map says so with a reason.

```mermaid
erDiagram
  APPLICATION ||--o{ CELL : has
  TEST_TYPE ||--o{ CELL : classifies
  CELL ||--o{ TEST_RECORD : backed-by
  TEST_RECORD ||--|{ RUN : consists-of
  TEST_RECORD ||--o| MUTATION : proven-by
  TEST_RECORD ||--o| REGISTER_ITEM : guards
  CELL {
    string status
    string reason
  }
  RUN {
    int iteration
    string verdict
    string evidence_digest
  }
```

### 13.2 Applicability map format and initial content

```yaml
# matrix/applicability.yaml  (NOT EXECUTED; initial proposal for owner review)
schema: applicability/1
types: [unit, integration, e2e, full_automation, security, ddos, scaling, chaos, stress,
        performance, benchmarking, ui, ux, challenges, helixqa]
applications:
  catalog-api:
    applies: all
    na:
      ui: "no user interface"
      ux: "no user interface"
  catalog-web:
    na:
      ddos: "client application; load tests target the API"
      scaling: "client application"
      benchmarking: "no benchmarkable server logic; frontend performance covered by performance type"
  website:
    na:
      ddos: "static site; platform concern"
      scaling: "static site"
      chaos: "static site"
      stress: "static site"
      benchmarking: "static site"
  build:
    na:
      ddos: "not a service"
      scaling: "not a service"
      ui: "no user interface"
      ux: "no user interface"
```

Every `na` needs a reason and is itself reviewed (an n/a that removes an applicable type is a
bluff of the SC-004 kind). Applicability is owner-reviewable data (11.4.35).

### 13.3 The gate

`SC-004` is the gate's acceptance test: the generator exits non-zero when any applicable cell is
not `present`, with a list naming the cell and the missing evidence. A blocked cell makes the gate
fail (FR-025), which keeps completion tied to the owner supplying the dependency. The gate's own
test-first artifact is a paired mutation set: remove a ledger record (cell must flip), forge a
verdict line (the chain check must fail, document 06), mark an applicable type n/a without a reason
(the gate must refuse), and a golden-false case (a complete matrix must pass).

### 13.5 Release seam, final manual QA and candidate re-cut (revisions 9, 10, 13, 14 and 15)

This section restates tasks.md rev 26 (revision 10; first written for rev 24) (P6-P7 release-seam and candidate re-cut rules; T447a, T447b,
T504a, T514, T554, T564a, T569, T570, T580e, T580f, T582); the task list binds.

- **Release seam.** The commit-push script has no release mode. Every release-seam check is a
  standalone script with its own exit code that judges the candidate fingerprint recorded by the
  candidate build (T566), never a commit-push stage: the performance verdict
  (`scripts/perf/release_check.sh`, T514), the escape-ratchet and catchability gates
  (`scripts/qa/escape_gates.sh`, T554), the can-i-deploy matrix for the candidate provider version
  (section 9), the claim-vs-reality ledger (`scripts/docs/claim_ledger.py`, T548a), the quality
  floors `mutation_ratchet_challenge.sh` and `scripts/qa/observable_assertions.sh` (T504a; 85% and
  60% with their ratchet values), the verdict-coverage check `scripts/qa/verdict_coverage.py` (T570,
  11.4.135: a guard of the standing guard registry whose topology is present and that has no verdict
  for the candidate fingerprint refuses the candidate), and the SLSA gate
  `scripts/supply_chain/check_slsa.sh` per deliverable with its `--check-record
  docs/security/SLSA_LEVEL.md` mode (T447a, T447b). They run at two named points, T569 (before the
  candidate's manual QA) and T582 (every check re-run before the final verification is accepted), on
  the committed, checkpointed register; outputs go under `.audit/out/<op_id>/` and are captured
  through the evidence recorder. A refusal blocks the candidate exactly as a FAIL does; an absent
  verdict is a refusal, never a pass. The release-seam scripts are the path class `release_seam`
  over `scripts/release/release_seam_files.txt` (refusal `release_seam_list_stale`); its data files
  are only tracked policy or threshold inputs (for example `$SPEC/perf/targets.yaml`,
  `$SPEC/quality/floor_ratchet.json`, `tests/contracts/compat-window.yaml` for
  `can_i_deploy.sh`), never the register, its dumps, ledgers, evidence, verdicts or reports
  (revision 10).
- **The two modes of the escape gates.** `--seam pre-qa --candidate <fingerprint>` (T569) judges
  its ratchet part only on the candidate's own register cycle `qa-<fingerprint>`, never on the latest
  cycle (which after a T580e session with findings belongs to the previous candidate); since no such
  row exists before this candidate's QA, seeded baseline or not, it reports the named non-refusing
  result `pre_qa_no_candidate_cycle` (never a ratchet PASS; revision 10, replacing
  `pre_qa_unseeded`); a missing `--candidate` is refused `candidate_missing`; the catchability part
  is judged in full.
  `--seam final --candidate <fingerprint>` (T582) needs the register cycle row
  `qa-<fingerprint>` with `manual_qa_ran = 1`, else it refuses with `manual_qa_not_run_for_candidate`;
  an unseeded baseline is refused with `baseline_not_seeded`; a register whose write-ahead log is not
  empty is refused with `register_not_checkpointed`.
- **QA-deploy-readiness gate** (11.4.236; T564a). `scripts/release/qa_handoff_gate.sh
  <fingerprint>` is a standalone script run before any deploy to the owner's QA target. It passes
  only when every output that the T569 evidence entry cites exists, matches its sha256, names this
  fingerprint and is a PASS (`pre_qa_no_candidate_cycle` accepted for the ratchet part only; any other ratchet
  result, the retired `pre_qa_unseeded` included, is refused `release_seam_refused`), and the HC-6
  record names the same fingerprint; otherwise it refuses with a named reason
  (`release_seam_output_absent`, `release_seam_output_sha_mismatch`,
  `release_seam_fingerprint_mismatch`, `release_seam_refused`, `hc6_absent`,
  `hc6_fingerprint_mismatch`). It is test-first with one golden-bad fixture per refusal and one
  golden-good fixture, and its paired mutation reads an absent output as PASS.
- **Final manual QA** (11.4.185, 11.4.238; T580e, plan HC-7). After HC-6 and before the bottom-up
  push of T581, the readiness gate is run on the candidate fingerprint; the candidate digests are
  deployed to the owner's QA target by promotion by digest, the SLSA gate run on each digest first and
  each target's build id read back (11.4.200). This deploy closes the development cycle
  (11.4.235(B)): each QA deploy mints its own version increment as a register `Task` item (revision
  10; a re-cut deployed for QA is a new deploy). When the session records a manual finding, that
  increment is applied as the first held commit of the fix cycle, before any fix commit, so the
  re-cut is built with the new version id; only the increment of the final, finding-free candidate
  is deferred to the first change of the next cycle, after the final state check T595b, never
  committed to this feature's `main` before it (version files are deliverable inputs). The owner's
  session is recorded as the register cycle `qa-<fingerprint>` with `manual_qa_ran = 1`, seeding the
  escape baseline when none exists; every manual finding is a discovery row with channel `manual_qa`
  and a `should_have_been_caught_by` value, linked to or reopening its item (11.4.214), and owes a new
  automated check in the standing guard registry with a RED captured on this candidate: anything
  manual QA finds is a coverage escape of the automated regime. Each such fix change set is reviewed
  on its own (T580f: the RED on the tested candidate, GREEN x3, the new registered check with its
  coverage-escape audit, and one reviewer-authored mutation recorded caught), and the candidate
  re-cut the fix brings is made only after that review's GO.
- **Candidate re-cut.** Whenever a new release candidate is called for, the one set re-run is: the
  candidate build (T566), T567 to T571 on its digests (the survivor loop included), T560, T561, T561a
  and T562 on its state, HC-6 taken again, and the final manual-QA hand-off held again on the re-cut
  candidate; when the manual QA recorded any finding, also the register closure steps T574 to T578
  and T586 on the register state that holds them. Each QA deploy of a re-cut mints its own
  increment (revision 10, replacing "a re-cut mints no second version increment"), and the
  evidence of the earlier candidate is never carried over. A constitution-test survivor or a
  constitution manual finding is fixed by a further T580b (3c) iteration in the re-cut set,
  consuming exactly the T440a `owed_to_T580b` rows (among them the 16 class-(c) third-party gitlinks
  that `submodules/constitution/.gitmodules` records, scoped by that recording parent's
  `.gitmodules`, never the class-(d) grandchild `MVT/js_mse_eme`).
- **The verdict-coverage hold (revision 13, tasks.md rev 29 T570, T570a, T569; round-28 blocking
  finding B1).** The T570 change set that adds `scripts/qa/verdict_coverage.py` to
  `scripts/release/release_seam_files.txt` cannot be held on the T565 verdict: that GO is written
  before T570 can start, so it can never list T570's run in `covers_runs`, and a hold on a GO file is
  refused `verdict_already_go`. It is held on its own verdict file
  `$EV/reviews/WP-71-verdict-coverage.json`, which the new review task T570a writes (G-GATE in its
  gate list, the held run's `CPA-Run` id in `covers_runs`), re-running each refused case
  (`registry_empty`, `join_empty`, a registered guard with no verdict for the T566 fingerprint, a
  verdict only for another fingerprint), the topology-absent listing and the paired mutations, with a
  reviewer-authored mutation recorded caught; T569 depends on T570a. The fixture
  `scripts/qa/tests/fixtures/verdict_coverage_hold/` runs through the real CPA path in a scratch
  clone (revision 15, tasks.md rev 31 round-30 M2: its `path_gate_unheld` cases are refused at S2): the change set held on `$EV/reviews/WP-71-gates.json` after that file is GO is refused
  `verdict_already_go`. The SC-005 verdict-check change set of T570 stays held on
  `$EV/reviews/WP-71-sc005.json` and is reviewed by the first T571 draw, not by T570a.
- **The RED/GREEN sample under the same hold (revision 14, tasks.md rev 30 T570, T570a; round-29
  review I5).** The RED/GREEN sample change set of T570 (`scripts/qa/redgreen_sample.py`, its tests
  and fixtures, its `docs/scripts/README.md` row, and its line in
  `scripts/release/release_seam_files.txt`, because the HC-6 decision of T580 cites its result) is
  held on the same `$EV/reviews/WP-71-verdict-coverage.json`, committed before T570a writes that
  verdict, whose `covers_runs` names the `CPA-Run` id of the verdict-coverage and the RED/GREEN
  sample commits alike (a GO that misses one is refused 20 `verdict_covers_incomplete`). T570a
  reviews the script with its refusals (`population_empty` among them) and paired mutations, records
  a reviewer-authored mutation caught (for example a copy that seeds from the system clock gives a
  different sample on re-run), and re-derives `$EV/retest/redgreen_sample.json` from its recorded
  seed, population list and population sha256: a different sample or different per-item verdicts
  are NO-GO `redgreen_sample_not_reproduced`. A later change to either change set is held on a
  further iteration `$EV/reviews/WP-71-verdict-coverage-r<n>.json`.
- **The commit turn of the manual-QA records (revision 13, tasks.md rev 29 T580e, T580f, T049).**
  Each main-repository run of T580e is a held commit with exactly one `Awaits-Review:` line, so the
  conductor grants the main-repository commit turn to one run at a time, checked by
  `scripts/release/commit_turn_check.sh` (`commit_turn_conflict`, also refused by `--grant`). From
  the grant until that run commits or is refused at S0, nothing writes into the main working tree:
  the grant is recorded in `.audit/commit_turn.json` (run id, `--paths-from` sha256; ignored, never
  committed) and `evrec` refuses appends with `commit_turn_held` while it names another run;
  records that other streams write meanwhile (a T571 re-draw verdict, T567 or T569 records of a
  re-cut) are committed by a general commit window before a turn is granted, and no general window
  starts while a turn is granted. The turn check is committed by T580e's first main-repository run
  and reviewed in `$EV/reviews/WP-73-commit-turn.json` (T580f); the catalog run is held on
  `$EV/reviews/WP-73-manual-qa-catalog.json` and the T066 dump-and-commit on
  `$EV/reviews/WP-73-qa-cycle-<fingerprint>.json`, NO-GO `qa_cycle_scope_mismatch` when its diff
  touches more than that cycle's dump and the WP-73 records (T580f). A T580a constitution-merge
  review re-runs the SC-005 closure check of section 8.3 item 6 on the merge commit.
- **The qa-cycle scope is the T066 change set (revision 14, tasks.md rev 30 T580f, T580e, T066;
  round-29 review B1).** The rev 29 rule admitted only `docs/register/register.sql` and so refused
  the real T066 dump-and-commit, which also writes `docs/workable_items.db` and the regenerated T067
  exports. The `$EV/reviews/WP-73-qa-cycle-<fingerprint>.json` review now admits the T066 commit set
  of this cycle's locked writes: its diff touches only `docs/workable_items.db`,
  `docs/register/register.sql`, the T067 exports regenerated in `docs/register/` (`Issues.md`,
  `Fixed.md`, `Issues_Summary.md`, `Fixed_Summary.md`, the `reconcile.sh` outputs and the HTML, PDF and
  DOCX siblings or their `skipped_tool_absent` rows) plus WP-73 records of the (0) list, and the
  register rows it changes, read by diffing `docs/register/register.sql` against HEAD, are exactly the
  `reg_cycle` row `qa-<fingerprint>` with `manual_qa_ran = 1`, its `reg_discovery` rows, any
  `reg_escape_baseline` seed, the (2) increment `Task` item, the
  `escape-baseline-lowering-<fingerprint>` `Task` item when (3) added it, the items that (4) linked or
  reopened with their link and history rows, and the `reg_export_runs` and `reg_export_files` rows of
  that export; anything else is NO-GO `qa_cycle_scope_mismatch`. The T580e fixture (vi) change set,
  the real T066 output, is the golden-good case, and a scope check admitting only
  `docs/register/register.sql` is its paired mutation. T581 (0) folds in every qa-cycle verdict, the
  final fingerprint's included (round-29 I2).
- **The commit turn honoured by every main-tree writer (revision 14, tasks.md rev 30 T580e, T001,
  T064, T567, T050; round-29 reviews I4, m1, m2).** Besides `evrec`, the T001 disk-headroom writer
  (when `DISK_HEADROOM_OUT_DIR` is unset), `locked.sh` register writes and the T567 host step refuse
  `commit_turn_held` while a grant names another run (fixtures (ix) to (xi)); the grant records the
  holder pid, its cmdline and `started_at`, `scripts/release/commit_turn_check.sh --reap` removes it
  only for a proven-dead holder, never by age (fixture (xii)), a run that exits 16 keeps its grant
  until its resume or a release (fixture (xiv)), and every release and reap is an `ev/1` entry
  (docs/16 section 9.7 item 27).
- **Soak runs triggered by records, never a timer (revision 14, tasks.md rev 30 T517, T457, T518,
  T566, T569; round-29 review I1).** The first T517 soak, stress and breakpoint run is triggered by
  the P5 exit record `$EV/p5-exit.json` (T457), and T517 is done when that run has completed with its
  results captured with `evrec`; that run is the soak input of T518 and T519 and, through T518, of
  `$EV/p6-exit.json`. Each later run is triggered by a T566 candidate build and is awaited and judged
  at the release seam by T569 (an absent result BLOCKED), never re-opening T517; the P6 exit record
  triggers no run. T517 depends on T457 (docs/14 section 8.7).
- **The SC-005 check under the release seam (revision 15, tasks.md rev 31 T570, T571, T580, T580a,
  T582; round-30 review I3).** `scripts/qa/sc005_verdict_check.py` becomes a release-seam file: the
  T570 change set adds it to `scripts/release/release_seam_files.txt`, as for `redgreen_sample.py`,
  so the `release_seam` row of `scripts/repo/path_gates.tsv` holds every later change to it on a
  later T571 draw with G-GATE in its gate list (the first draw carries G-GATE too; revision 16,
  tasks.md rev 32: a later draw writes its own iteration `$EV/reviews/WP-71-sc005-r<n>.json`, section
  8.3 item 7). Fixture `scripts/qa/tests/fixtures/sc005/unheld/` (scratch clone, real CPA S2
  path): an unheld change set to the script is refused 20 `path_gate_unheld` (RED captured before
  the list carries its row, GREEN x3); paired mutation recorded caught: with that path removed from a
  scratch copy of the list the case is accepted.
- **Fixture (vi) in two variants, and T581 on T580f (revision 15, tasks.md rev 31 T580e, T580f, T581;
  round-30 reviews I1, M4).** A seed means no earlier owner cycle and a lowering item needs one, so
  the two never occur in one cycle: (vi-a) seeded, a register with no earlier owner cycle holding the
  `qa-<fingerprint>` cycle row, two `reg_discovery` rows, the baseline seed, the (2) increment `Task`
  item and one reopened item; (vi-b) lowering, a register whose committed HEAD dump already holds an
  earlier owner cycle and its seeded baseline above this cycle's escape count, holding the cycle row,
  two `reg_discovery` rows, the (2) increment item, the `escape-baseline-lowering-<fingerprint>` item
  and one reopened item, each minted item with its own item, history and extension rows. Each
  variant's T066 change set passes the turn check and S0 and is a golden-good case of the T580f
  `qa_cycle_scope_mismatch` check; a scope admitting only `docs/register/register.sql` refuses both,
  and a scope check omitting the mint rows of the minted items refuses (vi-b). T581 depends on T580f,
  whose final qa-cycle and commit-turn verdicts its step (0) commits.
- **The conductor holds the turn (revision 15, tasks.md rev 31 T580e, T042, T050, T567, T514, T518,
  T570; round-30 reviews I2, M2, M3).** The grant's holder is the T580e conductor, never the CPA
  process: the conductor mints the run id, writes it as the grant's `run_id`, starts the run as
  `cpa-host --run-id <id>` (revision 16, tasks.md rev 32: the owner-installed host entry point of
  docs/16 section 12.2.8 item (13)), resumes it with `cpa-host --resume <id>` and exports
  `EVREC_TURN_RUN_ID`; fixture (xiv) gains a case where a waiting writer's `--reap` during the exit-16
  suspension leaves the grant (the conductor is alive) and refuses the append, and a `--grant` that
  records the CPA pid is the paired mutation (docs/16 section 9.7 item 28). The T567 host step reaps
  once before refusing. Every `path_gate_unheld` fixture (T514, T518, T570, T580e) runs through the
  real CPA S2 path. In T580b the outputs of `post_update_hook.sh` are classified by T580d and
  committed held on `$EV/reviews/WP-73-hook.json` when any is of a gated class (docs/11 section 7.4),
  and the T580a (d) fast-forward exception is stated in T580b's parent rule as well.
- **Holder liveness, conductor takeover and the hook-commit run (revision 16, tasks.md rev 32
  T580e, T580f, T580d, T580b, T582, T595a; round-31 reviews P5-P7 I2, I3, m4).** A grant's holder
  counts as dead only when the recorded conductor is proven dead and no live CPA process and no
  suspended-run holder exists for the grant's `run_id`, both read through
  `scripts/longops/holder.sh commit_push`, so a conductor that dies while its run is suspended at
  exit 16 or still running leaves the grant and the freeze in place. A new conductor session takes
  over with `scripts/release/commit_turn_check.sh --grant --adopt <run id>` (only the holder fields
  rewritten, an `ev/1` entry with reason `conductor_adopted`; refusals `adopt_no_grant`,
  `adopt_run_mismatch`, `adopt_holder_alive`) and then resumes the run with `cpa-host --resume <id>`
  or cancels it, moving its fix files back to the producing stream's scratch clone (moved, never
  deleted) and releasing the grant with reason `run_cancelled` (UNCONFIRMED in tasks.md: T042 rev 32
  defines no cancel option for a suspended run, owed to the P0 slice). A granted run that `cpa-host`
  refuses (`lock_held` and `run_id_in_use` included) has its grant released `refused_s0` and the
  conductor takes a new grant with a new id, never retrying the refused one. Fixtures (xv), (xv-b),
  (xv-c) and (xvi) with paired mutations (a `--reap` that tests only the conductor pid, an
  `--adopt` without the liveness check, a conductor that retries the refused id). T580d classifies
  every hook output path into `$EV/wp73/constitution/hook-outputs.json` and derives the gate list;
  in the ungated form the outputs and the T580d GO are committed together by one CPA run, in the
  gated form the outputs are a held commit on `$EV/reviews/WP-73-hook.json` whose GO, listing that
  commit's `CPA-Run` id, the next run commits and pushes with it; in both forms that hook-commit
  run is made by ST-GOV as the last CPA run before T582 starts, resumed to its end on exit 16, so
  the T582 measurement meets no unpushed hook commit.

- **Release-seam harness, the owed approvals and the trust-manifest check (revision 17, tasks.md
  rev 33 T288b, T318, T318a, T394a, T435a, T457, T514, T518, T535, T557, T564a, T565, T567, T569,
  T570, T580b, T580e, T582, T583, T593; round-32 reviews P3-P4 B1, I2, I3, M4 and P5-P7 B1 to B3,
  I1 to I4; amended in place in revision 18 for tasks.md rev 34 to rev 36: the released-seam
  precheck withdrawn, every release-seam GO owed, the harness trust state replayed, the
  image-digest equality dropped).** The CPA fixture harness has one definition for P4 to P7
  (central decision C3 of tasks.md T042, docs/16 section 12.2.8 item (14)): every fixture that runs
  through the real CPA path (S0, S2, S3, S5 or S6) runs in a throwaway clone under the test's
  temporary directory with its own test-state `CPA_HOST_STATE` and `CPA_HOST_ENTRY`, its only
  remote a throwaway bare repository seeded from main HEAD (never a hosted URL or the working path,
  two such repositories when a fixture needs two remotes), its owner approvals scripted through
  `script(1)` against that test state; the phrases "scratch clone", "scratch change set" and
  "scratch trees" in the release-seam fixtures (T514, T518, T535, T570, T580e, T580b) mean this
  harness, while the producing stream's own clone of T580e (xv-b) and (xviii) is named apart. The
  harness trust state is built in one order (revision 18; C3 (a) to (c)): the working-tree copies
  of the files under test committed as one scratch base commit, then the adoption (before T047 a
  fixture adoption listing every CPA-executed path at the clone's HEAD, from T047 on the recorded
  adoption), then a replay of every owner-trust record `$EV/hc/owner-trust/<n>.json` that precedes
  the GO under test in ascending numeric order, each with exactly the `--commit` sha and flags its
  record names; each case runs on a fresh clone and bare remote; an exit 20 the case does not name,
  or an unexpected 10, is a harness or trust-state defect repaired in the harness, never a reopen
  (C3 (f)). A paired mutation that removes a row from a CPA table or path list commits the list
  without the row in the scratch history and approves that version in the scratch state, never
  edits a working-tree copy alone, because the approved rows stay in force; every CPA git call of
  these fixtures carries `-c core.hooksPath=$CPA_RUN/no-hooks`. The P4-P5 CPA check stages (T288b,
  T318a and the post-release real-stage steps of T394a, T503, T504a, T514, T535, T537, T538, T543
  and T564, named in the P5 exit record of T457, the P6 exit record of T557 and the WP-71
  checkpoint T567) run under three rules: (i) on that harness; (ii) precondition, checked before
  anything runs: the owner's real `cpa-host --show` manifest (read only) holds the check's command
  file with the sha256 of its `reviewed_files` entry in the releasing GO, and an approval record
  `$EV/hc/owner-trust/<n>.json` names that GO, else the step is `Operator-blocked`, never run and
  never a reopen; revision 18 (tasks.md rev 36, round-35 review P3-P4 B1): the `images.lock.yaml`
  digest equality that revision 17 listed here is dropped (that file is shared and changes in many
  GOs, so the equality blocked every later step for good), the approved cases using the GO's own
  lock content through the replay, and when a later GO changed the command file the step runs for
  the newest owner-approved GO that lists it instead of refusing; (iii) pending control: the pending
  golden-bad case runs on the replayed state WITHOUT the GO under test and first exits at least 14
  with `CHECK_PENDING_RELEASE` naming the check and never 10 (the check did not run), and after the
  scripted harness approval of that GO the approved golden-bad case exits 10 while the golden-good
  set passes; a run report naming `check_pending_release` means not yet released, never a reopen.
  Changed check rows follow the keyed tables rule (central decision C1): the T394a `eslint` and
  `prettier` switch from `deferred` to `ratchet` and the T514 `perf_cheap` switch to `plain` are
  each `check_pending_release` (exit at least 14) until the owner approves the switch GO, the
  approved `deferred` row being the mode-strictness floor; fixtures on the harness and a
  last-writer-wins mutation each; T394a's post-release step adds the case of a declared row setting
  the approved `ratchet` row back to `deferred`, which runs the approved row and exits 10 by the
  T040 exit precedence (a refusal beside a pending check), nothing committed; the type-aware ESLint
  row runs remotely, so its approved case gives 10, or 20 `remote_check_unavailable` with no
  qualified build host, never a pass. Revision 18 (tasks.md rev 34 to rev 36; central decisions C7
  and C8 as revised in rev 34, round-33 reviews P0 B1 and P5-P7 B1, round-35 review P5-P7 I1): the
  released-seam precheck that revision 17 described here (`scripts/release/seam_released.sh` run as
  `git show <released commit>:... | bash -s`, with its records `seam-precheck-*.txt`) is withdrawn:
  it failed open (no `pipefail`, unfetched objects, a released commit undefined when the remotes
  differ). Every release-seam script, and every helper a task calls outside a CPA run, runs only as
  `cpa-host --exec-approved <path> [args...]` from the owner-approved export (the script, the
  scripts it sources and every listed data file, read from `$CPA_APPROVED_DIR`, never from the
  working tree), with no working-tree sha256 assertion and no check against the released commit;
  a release-seam path that the approved manifest lacks, or a release-seam script started with
  `CPA_APPROVED_DIR` unset, is refused 20 `release_seam_unreleased`, any other helper 20
  `helper_not_approved`, each with reason `path_not_approved`; fixtures (each calling task carries
  its own): a foreign commit pushed to every remote that replaces `scripts/perf/release_check.sh`
  and `scripts/release/qa_handoff_gate.sh` by marker-writing copies leaves both markers absent, a
  held foreign merge of a seam script never executes it before its merge-review GO and the owner's
  approval, and a working-tree-only edit of a listed data file changes no result (T325, T504a,
  T514, T548a, T570); paired mutation: a caller that execs the working-tree path. An owner approval
  is owed for every `G-GATE` GO whose reviewed files intersect the CPA-executed set, which since
  rev 34 includes every release-seam script and data file with no carve-out, so T519, T555, T565,
  T570a and T580f are owed, recorded in the owed list `$EV/hc/owner-trust/owed.json` until approved
  (an owed GO is closed `superseded` when later GOs list each of its paths at newer content); the
  guard-registry rows of T473 are owed too. A direct call of `record_pending_pin.sh` outside a CPA
  run runs through `cpa-host --exec-approved`; the working-tree copy started with
  `CPA_APPROVED_DIR` unset is refused `helper_not_approved` (central decision C8, T435a fixture and
  mutation; no self-check of its own sha256 counts as the guard). The cancel of a suspended run is
  the holder-expiry path of central decision C2: the session waits for `holder.sh commit_push` to
  report the holder `expired`, runs `acquire.sh --expire commit_push --op-id <op_id>` through
  `cpa-host --exec-approved` (an `--expire` outside a run without `--op-id` is refused
  `usage_error`, rev 36), moves the run's files back and releases the grant `run_cancelled`, an
  `expired` holder counting as live until then (T580e fixture (xv-b) and its mutation); this closes
  the UNCONFIRMED of the bullet above. The T580b hook run hashes `.git/hooks/` and
  `.git/modules/*/hooks/` with `.claude/` and records each new or changed hook in
  `hook-outputs.json` with its decision (`kept` or `removed`), with a planted-hook case and a
  mutation that hashes `.claude/` only. T582 adds the trust-manifest closure check
  (`trust-manifest.json` and `trust-diff.json` under `.audit/out/<op_id>/`, each difference an owed
  approval, a `deleted_unretired` path closed only by the owner's `retire`, or a FAIL, with two
  control needles; docs/04 §4), which T583 recomputes and T593 lists and closes from a re-read of
  `cpa-host --show`.

- **The pre-release production form, the post-release runs and the commit-turn holder (revision
  18, tasks.md rev 35 and rev 36 T325, T335, T358, T359, T457, T504a, T548a, T557, T561a, T564a,
  T566, T569, T570, T570a, T571, T580, T580a, T580e, T581, T582; round-34 reviews P3-P4 B1, P5-P7
  B1, I1 to I5, m1, m3, m5, round-35 reviews P3-P4 I1, I2, I6 and P5-P7 B1, I2 to I4, m1, m2).** A
  task that produces an output with a release-seam or helper script whose GO the owner has not yet
  approved cannot wait for that approval (the approval needs the review, the review needs the
  output), so it uses the pre-release production form of central decision C8 (ii): it commits its
  held change set first, its new records included, then runs the script on the T039 harness through
  the test-state `cpa-host --exec-approved`, whose scratch state approves in the TEST state only,
  first every committed GO the run needs that the owner has not approved, in introduction order,
  and last a harness-only fixture `G-GATE` GO listing the held change set, its `covers_runs` naming
  the held commits and a synthetic provenance record (a Workflow run id, Opus, `xhigh`,
  `verdict_sha256`); the output carries a top-level `"pre_release": true` and its `ev/1` entry is
  marked `pre_release` (docs/06 §11; the `ev/1` schema has no such field, a contracts gap recorded in
  contracts/README.md revision 25). T564a, T569, T580, T580a and T582 refuse a `pre_release` output
  with 20 `pre_release_output`; the final run is a real `cpa-host --exec-approved` re-run after the
  owner's approval, compared with the pre-release output with the `pre_release` field excluded.
  The producing tasks: T335 (`$AUD/contract/can-i-deploy-pre-release.json`), T358
  (`$AUD/contract/can-i-deploy-T358-pre-release.json`, its inventory commit held on
  `$EV/reviews/WP-50-T358-gate.json` written by T359), T504a and T561a (the mutation and
  observable-assertion floors, read from the approved `floor_ratchet.json`), T548a, T566 and T580e
  (2) (`check_slsa.sh`), and T571, T580, T580a and T582 for `sc005_verdict_check.py`. Two
  post-release runs follow one difference policy: the WP-41 checkpoint's real `can_i_deploy.sh`
  run after the WP-41-gate approval and T358's real run after the WP-50-T358-gate approval, both
  listed in the P5 exit record T457, must equal their pre-release verdict except the `pre_release`
  field, a difference being a registered finding that does not block; the post-release fingerprint
  is the one the stale-checked result files carry (rev 36). T548a's final WRITE run, once the T549
  approval record exists, regenerates the tracked `docs/reference/claim_ledger.json` without the
  marker before T569 judges it. Release-seam files are main-repository tracked paths only: T504a
  provides ONE main-tree `challenges/mutation_ratchet_challenge.sh` (§11.4.251 fork decision,
  recorded in the T505 verdict), and a list line naming a path inside a submodule is refused at S2
  `release_seam_path_not_main`. T457 and T557 list every owed approval still open and every commit
  whose `Deferred-Gates:` line carries `CHECK_PENDING_RELEASE`, with a catch-up run of the approved
  check over those commits' paths, `cpa-host --exec-approved scripts/repo/validate_cheap.sh --check
  <key> --paths-from <list>` (central decision C8 (i); a key the approved copy lacks refused
  `check_not_approved`). T570's `verdict_coverage.py` reads `scripts/qa/guard_registry.tsv` from
  `$CPA_APPROVED_DIR`, and T570a's `reviewed_files` name `scripts/qa/redgreen_sample.py`, its tests
  and its `release_seam_files.txt` row. The commit-turn grant of T580e names its holder by
  `--holder-pid <pid>`, a live ancestor of the caller recorded with its start time (central
  decision C12; a missing option `usage_error`, a non-ancestor `holder_pid_not_ancestor`, fixture
  (xvii)), because the `cpa-host` that `--exec-approved` places between them is short-lived; which
  process the conductor's pid is under the harness in use is UNCONFIRMED and recorded as a residual.
  A granted run that passed S0 and then exited with a report that records no commit is released
  `run_failed`, its declared files moved back to the producing stream's clone (fixture (xviii), its
  failing check row planted on its own harness instance only). T581 re-checks that no owed approval
  is open right after step (0) and again immediately before EACH push step, and blocks on a
  `deleted_unretired` path until the owner's `retire`.

### 13.4 Cross-cutting applicability: translation and i18n, accessibility (revisions 3 and 4)

Two concerns cut across the fifteen types and had no explicit applicability record. Both are recorded here as data for the applicability map (section 13.2) under a `cross_cutting` key, and both are owned by docs/21 WP-61 (absent test types) for the authoring and by WP-70 for the final matrix gate.

**Translation and i18n.** Measured on 2026-10-03 by reading the manifests and resource trees: no internationalisation library is declared in `catalog-web/package.json`, `catalogizer-desktop/package.json`, `installer-wizard/package.json` or `Website/package.json` (a case-insensitive search for `i18n`, `intl`, `locali`, `lingui`, `formatjs` returns 0 in each); `catalogizer-android` and `catalogizer-androidtv` have only the default `res/values` folder and no locale-qualified `values-*` folder. The server has `catalog-api/internal/handlers/localization_handlers.go` with its test file; document 01 §3.1 records that these handlers register on a `mux.Router`, and whether they are reachable in the running server is `UNCONFIRMED:`. The translation mandates (11.4.255 HelixTranslate pipeline, 11.4.256 independent per-language review, 11.4.237 context-and-spirit review) bind translated content, and none was found in A1 to A9. Revision 4 (file-name search of every submodule's `git ls-files` for `i18n`, `locale`, `translation` and `values-<locale>` paths, 2026-10-03, outside vendored `tools/opensource` trees): several Go shared modules (A10) carry an i18n seam `pkg/i18n` with an English bundle only, and two governance and QA modules (A12) carry translated bundles, `submodules/containers/pkg/i18n/bundles/` (`de`, `fr`, `ja`, `sr`, `zh` besides `en`; 6 files, 103 lines) and `submodules/doc_processor/pkg/i18n/bundles/active.sr.yaml`.

| Application | Translation and i18n | Reason and evidence | What would change it |
|---|---|---|---|
| A1 catalog-api | open | `localization_handlers.go` exists; reachability `UNCONFIRMED:` | if the localization routes are reachable and serve translated strings, i18n tests (locale negotiation, fallback, encoding) and the translation mandates apply to those strings; WP-30 settles reachability |
| A2 catalog-web | n/a | no i18n library declared (manifest read; whether every component string is English was not measured) | adding a second language makes the translation pipeline and per-language review mandatory, and UX gains locale tests |
| A3 desktop, A4 installer | n/a | no i18n library in either manifest | same as A2 |
| A5 android, A6 tv | n/a | only `res/values`, no `values-<locale>` folder | a `values-<locale>` folder makes translation review mandatory and adds locale and right-to-left layout checks |
| A7 api-client | n/a | a library with no user-facing strings | none |
| A8 Website | n/a | no locale configuration found in its manifest | a translated site version |
| A9 Build, A11, A13 | n/a | no user-facing text; A11 React modules render text supplied by A2; A13 is test and harness code | an A11 module that ships its own strings |
| A10 Go shared modules | n/a | revision 4: an i18n seam (`pkg/i18n` translator code) exists in several modules, with an English bundle only (`active.en.yaml` in `middleware`, `observability`, `storage`, `streaming` and `watcher`); no translated bundle found | a non-English bundle in a module, which makes the translation mandates apply in that module's own repository (FR-006) |
| A12 governance and QA modules | open | revision 4: translated bundles exist in `submodules/containers` (`de`, `fr`, `ja`, `sr`, `zh`) and `submodules/doc_processor` (`sr`); `helix_qa`, `challenges`, `vision_engine`, `llm_orchestrator` and `llm_provider` carry English bundles only; whether the translated bundles came through the 11.4.255 pipeline with the 11.4.256 and 11.4.237 reviews is `UNCONFIRMED:` | the docs/21 WP-34 audit row of each of the two modules records the provenance and review of its bundles; any fix lands in that module's own repository (FR-006) |

Every n/a above is reviewable (an n/a that hides an applicable concern is the SC-004 bluff of section 13.2), the A1 row stays open until WP-30 records the reachability result, and the A12 row stays open until the two WP-34 audit rows record the provenance of the translated bundles.

**Accessibility (WCAG 2.2 level AA).** The UX type (section 2, row 13) is closed for a user-facing application only when its accessibility checks pass. Automated tools find part of the WCAG failures, so every application also gets a scripted manual walkthrough whose result is recorded as evidence (human oracle, section 2), and neither part substitutes for the other. Document 18 T9-A (checklist) and T9-B (TV focus restoration) are the sources of the items below.

| Application | Automated checks | Manual or scripted checks | Platform specifics |
|---|---|---|---|
| A2 catalog-web | axe-core rules inside the existing Playwright suites (`accessibility*.spec.ts` exist), Lighthouse accessibility category | keyboard-only walkthrough of sign-in, browse, search, playback start; focus visible and not obscured; target size; dragging alternatives; consistent help | colour contrast in both themes |
| A3 desktop, A4 installer | axe-core against the Tauri webview pages through the planned Playwright and WebDriver suite | keyboard-only walkthrough of every installer step and the desktop main flows | native dialogs (`plugin-dialog`) checked by hand, `UNCONFIRMED:` tool support |
| A5 android | Android accessibility checks in instrumented tests (Accessibility Test Framework class; exact library `UNCONFIRMED:` until the build container resolves it) | TalkBack walkthrough of sign-in to playback on a real device or an owner-approved emulator (DR-4) | content descriptions, touch target size, font scaling |
| A6 tv | the same instrumented checks where the TV components support them | D-pad walkthrough: focus order, focus always visible, and focus restored to the originating item after returning from a detail screen (a test that fails when focus is lost, document 18 T9-B) | 10-foot readability, remote-only navigation |
| A8 Website | axe-core and the Lighthouse accessibility category per page in three engines (11.4.190) | keyboard walkthrough of navigation and search | none |
| A11 React modules | axe-core in component tests with a real DOM | none beyond A2, which renders them | none |
| A12 governance and QA modules | open (revision 4): HelixQA tracks two operator web pages, `submodules/helix_qa/docs/website/challenges-dashboard/index.html` and `docs/website/ticket-viewer/index.html`; whether they are a website the project ships (11.4.190) or QA tooling outside this matrix is recorded on the `submodules/helix_qa` audit row of docs/21 WP-34, and any check lands in that repository (FR-006) | none until that row decides | none |

Each automated check ships with its paired mutation (for example, remove an `aria-label` or the focus-restoration call; the check must fail), and each manual walkthrough is recorded with screen captures and the vision oracle where the UI is not introspectable (11.4.117, 11.4.193). A1, A7, A9, A10 and A13 have no user interface: accessibility is n/a for them with that reason; A12 is open as its row states.

Applicability-map entries added by this section (NOT EXECUTED; proposal for owner review):

```yaml
cross_cutting:
  translation_i18n:
    catalog-api: {status: open, reason: "localization handlers exist; reachability UNCONFIRMED (WP-30)"}
    catalog-web: {status: na, reason: "no i18n library declared"}
    catalogizer-desktop: {status: na, reason: "no i18n library"}
    installer-wizard: {status: na, reason: "no i18n library"}
    catalogizer-android: {status: na, reason: "only res/values"}
    catalogizer-androidtv: {status: na, reason: "only res/values"}
    website: {status: na, reason: "no locale configuration"}
    catalogizer-api-client: {status: na, reason: "library with no user-facing strings"}
    build: {status: na, reason: "no user-facing text"}
    go-modules: {status: na, reason: "i18n seam with English bundles only; no translated bundle (revision 4)"}
    ts-react-modules: {status: na, reason: "render text supplied by catalog-web"}
    governance-qa-modules: {status: open, reason: "translated bundles in containers (de, fr, ja, sr, zh) and doc_processor (sr); pipeline and review provenance UNCONFIRMED (WP-34)"}
    harness: {status: na, reason: "test and harness code, no user-facing text"}
  accessibility_wcag22_aa:
    applies: [catalog-web, catalogizer-desktop, installer-wizard, catalogizer-android, catalogizer-androidtv, website, ts-react-modules]
    na: {catalog-api: "no user interface", catalogizer-api-client: "library", build: "no user interface", go-modules: "no user interface", harness: "no user interface"}
    open: {governance-qa-modules: "HelixQA operator web pages; 11.4.190 scope decided on the WP-34 row"}
```

## 14. Decision records

| Id | Decision | Alternatives rejected | Reason | Open input |
|---|---|---|---|---|
| DR-1 | Consumer-driven contracts (Pact family) with a self-hosted broker and can-i-deploy | Spec-only validation; custom broker | 11.4.244 pattern; avoid re-implementation | confirm library fit per consumer language |
| DR-2 | Coverage per application with diff gate (85%) and baseline ratchet | single global figure | spec Q3 resolved per-application | final targets and dates by owner after baseline |
| DR-3 | In-memory SQLite is real only if it is the production engine; otherwise replace | blanket ban on in-memory DB | the engine is `go-sqlcipher`; semantics, not memory-vs-disk, determine realism | read how production opens the database; owner confirm |
| DR-4 | Emulators allowed for UI logic tests only when the owner agrees equivalence; hardware claims need devices | emulators everywhere | FR-025 and 11.4.136 (real content, real path) | owner confirm which test types may use emulators |
| DR-5 | Instantiate go-mutesting config and baseline at the repository root | rely on submodule script | the script reads root-relative files that do not exist | tool compatibility with Go 1.25 |
| DR-6 | If the API cannot run multiple replicas against its storage, scaling type is n/a with a recorded architectural finding | invent a shared-database shim | an invented test of an unsupported topology is a bluff | read storage mode |
| DR-7 | `blocked-unavailable` as a third verdict, counted not passing, never skip | treat as skip (governance allowance) | owner's stricter rule, spec FR-025 | none |
| DR-8 | One evidence recorder for all languages | per-script helpers | drift (finding F-10) | none |
| DR-9 | No retries anywhere | retries on infrastructure flake | 11.4.248, FR-010 | none |

## 15. Risks and open items

| Risk | Effect | Mitigation |
|---|---|---|
| The measured baseline reveals many failing suites | scope grows; each failure is a finding (FR-008) | triage by risk; run baseline early (TS-01) |
| Mutation tooling does not support Go 1.25, Compose or Tauri | cannot reach the 85% instrument | fall back to the bash mutation harness pattern plus manual reviewer mutation; record the limit honestly (11.4.6) |
| Real-device tests cannot run in the environment | many cells stay `blocked` and completion waits | declare early which devices are needed (`dependencies.yaml`); owner supplies |
| Contract libraries missing for a consumer language | contract tests incomplete | recorded-traffic fallback in Pact JSON (section 9.2) |
| Host memory ceiling slows mutation and Gradle runs | long wall time | every compiling lane runs on the remote build host (11.4.173; owner decision C1, revision 8), dispatched event-driven so the wall time overlaps other work |
| Tests rewritten in bulk by agents become assertion-poor | SC-005 breach | reviewer-written mutations and the numerator rule (sections 7.3, 8.3); 60% observable-assertion check by the anti-bluff scan |
| Test count inflates without value | cost, noise | push-tests-down rule (11.4.169): lower-level test preferred, redundant high-level test removed |

Open items for the plan owner: DR-3, DR-4, DR-6 inputs; final coverage targets and dates;
sample size for SC-005; the library fit of section 9.2; the identity of the remote build host
(12.1; docs/21 ODG-07, the build location itself decided by C1); the origin of `installer-wizard/test-results.json`.

## 16. Traceability and acceptance evidence

| Requirement | Plan sections | Acceptance evidence |
|---|---|---|
| FR-009 | 2, 4, 6, 13 | generated matrix with zero absent applicable cells; each closed cell lists a ledger record |
| FR-010 | 6 (TS-02), 7.2, 8, 11, 12, doc 06 | three-run comparison records; mutation caught per test; no retry configuration |
| FR-011 | 7 | per-application baseline records under `$EV/coverage_baseline/<app>/`, targets files under `$EV/coverage_targets/<app>/`, ratchet gate passing with mutation-proved gate |
| FR-016 | 9 | broker matrix verdict record; both-side tests; missing-contract refusal test |
| FR-025 | 10 | preflight records; `blocked` verdicts with reason codes; no `skipped` outcome in the reporter; dependency file |
| SC-004 | 13 | generator exit code and matrix with zero gaps |
| SC-005 | 8.3 | reviewer sample records: seed, sample, mutation diffs, three-run outputs, zero survivors |
| SC-003 (test side) | doc 06 | RED/GREEN pair per fixed item across three runs |
| SC-011 (test side) | 6 TS-08 | baselines and regression gate records |
| SC-003, SC-005 (release side) | 13.5 | T569 and T582 release-seam outputs for the candidate fingerprint; the `qa_handoff_gate.sh` pass record; the `qa-<fingerprint>` manual-QA cycle and its discovery rows (revision 9) |

Completion claims drawn from this plan cite ledger records, never this document (FR-022, SC-012).
Anything marked `UNCONFIRMED:` or `UNKNOWN:` above is to be resolved by a measured run, not by
argument.
