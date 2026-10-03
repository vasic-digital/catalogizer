# Phase 0 Research: Full Project Audit and Remediation

| Field | Value |
|---|---|
| Feature | `specs/001-full-project-audit-remediation` |
| Created | 2026-10-03 |
| Revision | 3 |
| Last modified | 2026-10-03 |
| Status | draft, consolidation of planning documents 01 to 20 (document 21 excluded, still being written) |
| Inputs | `spec.md`, `docs/01`..`docs/20`, `poc/` tools and results, `.specify/memory/constitution.md` |
| Rule | Every decision cites its source document and section. Nothing here was re-derived from memory. Where the documents disagree, the conflict is stated and a resolution is recommended; the owner may overturn any recommendation. |

## Table of contents

1. How this consolidation was made
2. Technical context resolved (replaces every `NEEDS CLARIFICATION`)
3. Decisions (R-01 to R-30)
4. Conflicts between planning documents and how they are resolved
5. Owner decisions and inputs (consolidated table)
6. Items that remain UNCONFIRMED or UNKNOWN (not owner decisions, resolved by work)

## 1. How this consolidation was made

The decision records, open-question lists and risk sections of each planning document were read section by section (`docs/02` §5, §16; `docs/03` §8, §14; `docs/04` §3, §7, §15; `docs/05` §5, §8, §9, §10, §14, §15; `docs/06` §3, §18; `docs/07` §14; `docs/08` §15; `docs/09` §17; `docs/10` §22; `docs/11` §1.3, §6, §7, §12; `docs/12` §3.5, §6, §19, §20; `docs/13` §3, §8, §10, §14; `docs/14` §7, §8, §16, §18; `docs/15` §14; `docs/16` §10, §14, §18, §20; `docs/17` §14 and recommendations; `docs/18` §1; `docs/19`; `docs/20` §3, §4, §7, §9, §13). The three POC tools were re-run on 2026-10-03 (see `quickstart.md`) so that the numbers below are measured in this session, not copied.

## 2. Technical context resolved

The `plan.md` template is still unfilled; none of the planning documents contains a literal `NEEDS CLARIFICATION`. The template's unknowns are resolved as follows.

| Template field | Resolution | Source |
|---|---|---|
| Language/Version | Go 1.25.7 (`catalog-api`, Go submodules); TypeScript (React web, api-client, TS submodules, Tauri front ends); Rust (Tauri 2 back ends); Kotlin with AGP 8.2.2, Gradle wrapper 8.11.1 (Android, Android TV); Bash (`Build/`, `scripts/`); Python 3 (audit tooling, HelixQA support) | docs/01 §3; docs/20 §7.1, §9.1 |
| Primary Dependencies | Gin (+ gorilla/mux handlers, partly unwired), go-sqlcipher/SQLite and PostgreSQL, React + React Query + Vitest + Playwright, Tauri 2, Jetpack Compose + Room + Retrofit, HelixQA, Challenges, constitution submodule | docs/01 §3, §6; docs/05 §4 |
| Storage | Application: SQLite (sqlcipher) or PostgreSQL via Go migrations v1 to 20; register: `docs/workable_items.db` (engine schema plus `reg_*` extension) | docs/01 §6.3; docs/04 DR-1 |
| Testing | 16 test types seeded in `reg_test_types` (unit, integration, e2e, full_automation, security, ddos, scaling, chaos, stress, performance, benchmark, ui, ux, challenge, helixqa, contract) | docs/04 §5; docs/05 §2 |
| Target Platform | Linux server and containers (rootless Podman), browsers, Windows/macOS/Linux desktop, Android phone/tablet, Android TV | docs/01 §11 |
| Project Type | Multi-application monorepo with 44 direct and 53 nested submodules (98 repositories in total) | docs/19 §6.1 (re-measured) |
| Performance Goals | Project-set targets per operation from measured baselines (formula in R-17); the external 30/50 ms numbers do not bind (spec Q2) | docs/14 §7; spec Q2 |
| Constraints | Main branch only, fast-forward only, no force-push; rootless containers for every build; 60% host memory ceiling (about 18 GiB of 30 GiB); no CI/CD; real services and devices or `blocked` | spec FR-020, FR-021, FR-024, FR-025; docs/02 §2.3 |

## 3. Decisions

Each entry: **Decision**, **Rationale**, **Alternatives considered**, **Source**. "Owner input" points to section 5 when the decision is provisional.

### R-01 SLSA level claim

- **Decision**: Record the honest current level as Build L1 once provenance is generated for every release artifact (until then the level is `UNKNOWN`/L0 and is written as such in `docs/security/SLSA_LEVEL.md`). Build the self-assessed L2 path (owner-operated dedicated build host that generates and signs provenance; verifier checks `builder.id`) as the target. Never write "L2" until the owner chooses option B or C; never claim L3 on a single-uid host.
- **Rationale**: SLSA v1.1/v1.2 Build L2 requires a hosted platform that generates and signs provenance; CI is not mandated, but a workstation is excluded. Constitution §11.4.246 sets L2 as minimum and §11.4.156 forbids CI/CD, so the only honest path to L2 is designating the dedicated build host as the platform.
- **Alternatives considered**: claim L2 from local builds (over-claim, a bluff under §11.4.201/§11.4.226); adopt a hosted CI service (contradicts §11.4.156); `slsa-github-generator` (GitHub Actions only); BuildKit `--attest` alone (no level statement, Podman support unverified).
- **Source**: docs/20 §3.1 to §3.6 (DR-20-01); docs/17 §7.1 (superseded framing corrected by docs/20 §3.1 point 2); docs/15 §9 and OQ-S3; docs/16 §14. Owner input: OD-01, OD-02.

### R-02 Real-service and real-device unavailability: the `blocked` verdict

- **Decision**: A test that depends on an external service, credential or physical device runs against the real one every time. When the dependency is absent the run records verdict `blocked` with one reason from the closed set (`service_unreachable`, `credential_absent`, `credential_rejected`, `device_absent`, `device_wrong_identity`, `device_unauthorised`, `geo_restricted`, `quota_exhausted`, `licence_absent`, `host_resource_unavailable`). `blocked` counts as not passing, is never a skip and is never replaced by a simulation. The register item stays `Operator-blocked` and the feature cannot complete until the owner supplies the dependency. Probes run first and decide `blocked` before the case runs (`blocked_when` in bank-case v3).
- **Rationale**: spec clarification 2 and FR-025 are stricter than the governance's honest SKIP; a distinct verdict keeps an infrastructure gap from being counted as a product defect while still not counting as a pass.
- **Alternatives considered**: treat as SKIP-with-reason (governance allowance, rejected by owner); emulators and mocks (forbidden for non-unit tests, §11.4.27; emulators allowed only for hardware-independent logic if the owner agrees, OD-27); fail as a defect (mislabels an infrastructure gap).
- **Source**: docs/05 §10 and DR-7; docs/06 §3.1 (`blocked_reason`); docs/12 §10 and DR-4; docs/04 §5 (`reg_test_runs.verdict` with `BLOCKED` requiring a reason).

### R-03 HelixQA ticket id collisions

- **Decision**: Legacy `HELIX-NNN` ids are labels, never keys. Import is keyed on `(source, locator)` = file path plus content hash; matching for recurrence uses `normalised(subject, scope)`. Register ids are freshly minted `ATM-NNN` from the append-only `reg_ids` table; the legacy id is stored in `reg_source_entries.legacy_id` so searching `HELIX-022` returns every item that label touched (`v_legacy_id_collisions`). The tool defect itself (HelixQA `NextFindingID` restarts at `HELIX-001` on an empty store and dedupes by exact title against open rows only, so recurrences mint new tickets) is a finding fixed upstream in `submodules/helix_qa`, and ticket emission from now on goes through the register (links-not-mints).
- **Rationale**: measured 1,778 ticket files carry only 676 distinct ids, 560 ids shared by more than one file (`HELIX-001` to `HELIX-005` each appear in 8 files); identity by id would be wrong by construction.
- **Alternatives considered**: keep `HELIX-NNN` as identity; renumber legacy files in place (rewrites history of a generated collection, loses citations).
- **Source**: docs/03 §2 F-2, §8.4, R-1; docs/04 §8, §15.2; docs/12 §3.5, §11; docs/13 D-13-02.

### R-04 Legacy-closed tickets: re-verification policy

- **Decision (mechanical default, provisional)**: Of 1,778 legacy files, 1,495 closed-class (`resolved` 704, `fixed` 492, `closed` 299) are imported as terminal with `custody_basis='legacy_import'` and `reverify_required=1`; `wontfix` (282) imports as `Queued` with `legacy_status='wontfix'` because "won't fix" is not a permitted closure under FR-008; the single open ticket imports as `Queued`. The re-verification queue `v_reverify_queue` (ordered by severity) feeds the audit; confirmation adds machine evidence and flips `custody_basis` to `machine_evidence`. Until the owner decides OD-10, the completion gate counts every `reverify_required=1` row as not done (mechanism: `v_reverify_queue` is registered in `reg_gate_checks` with kind `view_not_done`, docs/04 §12.3, §13.3). A legacy item that is reopened leaves the queue: it becomes an ordinary `machine_evidence` item and its next closure needs a full chain recorded after the reopen (docs/04 §5 limitation 5, §14.9 B2). Only a closed-class legacy entry can be imported terminal, and the exemption also ends when the item is moved into work (docs/04 §14.10 m-a).
- **Rationale**: SC-003 needs RED/GREEN runs that cannot be created retroactively; importing all 1,495 as open would bury the register, dropping them is forbidden (FR-002). 1,111 tickets have no Resolution section and 460 were bulk-closed by one agent on 2026-04-17, so none of the closures is evidence.
- **Alternatives considered**: import all as `Ready for testing` and re-prove every one (correct, very large, many screens no longer exist); import as `Fixed` (forbidden, unproven closure); drop (forbidden).
- **Conflict noted**: docs/03 DR-2 proposes "Queued or Ready for testing, never closed"; docs/04 DR-5 proposes terminal-with-reverify flag. Both keep the item open in effect (docs/04's gate counts the flag as not done). Recommendation: docs/04 DR-5 mechanics, because it preserves the historical status as data while the gate treats it as open; the owner decides the end state (OD-10).
- **Source**: docs/04 DR-5, §13.3, D-1; docs/03 §2 F-4, F-5, DR-2, R-2.

### R-05 The register is the workable-items engine database plus an extension layer

- **Decision**: One SQLite file `docs/workable_items.db`, tracked in git (§11.4.95), holding the constitution engine's tables unchanged plus an idempotent `reg_`-prefixed extension (v4 after four independent review rounds: 25 tables, 28 views, 41 triggers, measured in docs/04 §14.10; `register_ext.sql`, executed against a fresh engine DB, docs/04 §14.7 to §14.10). `reg_ids` is the identity anchor (generated `ATM-NNN`, append-only); because the engine's `items` key `(atm_id, current_location, representation)` cannot be foreign-keyed and admits the same id twice, the extension adds triggers that refuse an unminted id and a second row per id, plus gate views that report either if the triggers are bypassed (the engine's own `validate` checks neither, and the engine schema is not ours to change). Closure custody: an ACCEPTED decision must cite `custody_decision` evidence and a complete RED/GREEN/mutation/independent-review chain (`v_closure_ready`), re-checked at the status write; only evidence recorded after the item's last reopen counts, copies of earlier-cycle evidence files (same sha256) are refused and an earlier-cycle GREEN fingerprint does not count (docs/04 §7.1, §14.9, §14.10); the producer-equals-verifier residual (self-declared identities, bytes and fingerprints in SQL) is stated and closed by outside-SQL controls (docs/04 §7.1, §5 limitations 3 and 9). Engine status strings are kept verbatim, including `(→ Fixed.md)`. Tracker Markdown and exports are derived from the DB only. Id prefix `ATM` (§11.4.54). The constitution's own schema is not patched (extension is project data).
- **Rationale**: avoids a second source of truth (§11.4.93), reuses the engine and its tests (§11.4.74), keeps the audit trail in git.
- **Alternatives considered**: separate findings DB; patching `schema_embed.sql` in the constitution; PostgreSQL service; Markdown trackers as source of truth; full relational redesign; findings as rows of `items` only.
- **Source**: docs/04 DR-1 to DR-4, §4, §5, §15.2; docs/03 DR-1, DR-5, §2 F-1 (no register exists today). Owner input: OD-11 (prefix and path confirmation).

### R-06 Index health gate before any reliance on CodeGraph or Lumen

- **Decision**: Proofs P1 to P8 (docs/02 §4.1) are a gate per audit pass, written to `specs/001-full-project-audit-remediation/audit/index-health.json` (`$AUD/index-health.json`, `index-health/1`). P1 to P5 (CodeGraph state, completeness against tracked files, scope correctness, golden answers, freshness) block CodeGraph; P6 to P8 (Lumen embedder dimensionality, complete-and-fresh, scope parity) block Lumen. A failing index falls back to grep and direct reads for that class of question, with the gap recorded. Lumen semantic queries run only after a recorded freshness step, because a search writes the Lumen DB (`EnsureFresh`) and would make two audit runs read different states. CodeGraph is used through the CLI until the MCP server is registered (D-01). Thresholds: Lumen recall >= 0.85 (harness default), CodeGraph wrong-answer count recorded, both as consumer data.
- **Rationale**: the constitution records an index that reported "complete" while answering 5 of 15 fixture queries wrongly; scope files are absent here, so coverage must be proven empirically.
- **Measured today (2026-10-03, this session)**: CodeGraph 1.6.0, `state: complete`, `pendingRefs: 0` but `pendingChanges.added: 2` and `lastIndexed 2026-10-02T19:24:35Z` older than HEAD `e4852ce7` (committed 2026-10-03T10:56:02Z) so **P1 and P5 FAIL**; no `.codegraph/config.json`, no `.mcp.json`, no `.lumenignore`. Lumen: health OK, 10,397 files = indexed, 167,256 chunks, **`Stale: yes` so P7 FAILs**; 10,397 vs 7,150 files so **P8 FAILs**.
- **Alternatives considered**: trust `state: complete` alone; let every audit worker sync the index (database-locked incident; single writer D-03 instead).
- **Source**: docs/02 §2, §3, §4, D-01 to D-03, D-07; constitution §11.4.78, §11.4.275 (via docs/02).

### R-07 Dependency update policy: submodules only

- **Decision**: Own-organisation submodules (51 owned repositories of 98) are fast-forwarded to the unique maximum tip across all their remotes on their default branch, `--ff-only`, layer by layer (L0 nested constitution engines, L1 constitution plus post-pull sweep and hook, L2 Go leaf libraries, L3 Go dependents, L4 QA modules, L5 TS libraries), each move gated by a containerised module gate plus the affected applications' full tests (FR-018). Third-party pins (23 behind) are reported with pinned and latest commit and a status, not moved (`UPDATE_THIRD_PARTY=1` only on owner decision). Third-party package dependencies (npm, Go modules, Gradle, Cargo) are reported with current and latest versions and are not bulk-updated (spec Q1). Divergence is reported, never auto-resolved. Backup first (hardlinked `.git`).
- **Rationale**: spec Q1 and FR-017/FR-018; moving 22 vendored tool snapshots without tests widens blast radius for no product benefit.
- **Alternatives considered**: move all third-party pins; bulk-update package dependencies.
- **Source**: spec Q1, FR-017, FR-018; docs/11 §6.1 P-1 to P-6, §6.3, D-1. Owner input: OD-30, OD-33.

### R-08 Main-branch-only policy

- **Decision**: All work on the default branch of the main repository and of every submodule; no feature/product/flavor branches. "Main branch" is interpreted as the repository's default branch, which is `master` in 7 repositories (measured: 68 `main`, 7 `master`, 23 detached/no branch among 98); branches are not renamed. Integration is fast-forward only; nothing is force-pushed; `--no-verify` is never used.
- **Rationale**: owner decision (spec FR-024, constitution Known Conflict 15); renaming branches would be a history-affecting change on every remote.
- **Alternatives considered**: rename `master` to `main` (rejected, FR-020).
- **Source**: spec FR-024, FR-020; docs/11 D-3 (docs/11 counted 4 repositories on `master` among direct submodules; the recursive POC run counts 7 including nested); docs/03 §5.17 item 15. Owner input: OD-32.

### R-09 Android toolchain conflict (AGP, compileSdk, JDK)

- **Decision**: Treat as a finding with three options and settle it mechanically first: in the Android build container run `./gradlew :app:help --warning-mode all`, `./gradlew -q --version`, `./gradlew javaToolchains` and capture the compileSdk warning; then the owner chooses (a) `compileSdk = 34`, (b) AGP 8.6 to 8.13 with the existing Gradle 8.11.1, or (c) AGP 9.x with Gradle 9.x and the AGP 9 migration. Remove the undocumented `org.gradle.java.version` property. JDK 17 is the documented minimum; 21 is usable.
- **Rationale**: AGP 8.2 supports API 34 at most; the minimum AGP for API 35 is 8.6.0 (primary Android documentation retrieved 2026-10-03). `org.gradle.java.version` is not a documented Gradle property.
- **Alternatives considered**: status quo (outside documented support); immediate jump to AGP 9 without measurement.
- **Source**: docs/20 §7.1; docs/10 DR-10-01, DR-10-02; docs/03 §5.17 item 13. Owner input: OD-40, OD-41.

### R-10 OpenAPI drift approach

- **Decision**: Keep `docs/api/openapi.yaml` (OpenAPI 3.0.3, hand-maintained) as the contract and add three gates: (1) route table from the running router (`gin.Engine.Routes()` via a router constructor extracted from `main.go`) versus spec, (2) `oasdiff breaking` between base and head spec, (3) Schemathesis against a live seeded API. The regex POC `route_drift.py` stays as a cheap cross-check and regression oracle. Spec-first generation (oapi-codegen) is decided for new endpoints only after the drift test sizes the problem.
- **Measured today**: 247 served routes (gin plus wired mux), 181 spec operations, 68 undocumented, 2 stale (`GET /api/v1/discovery`, `GET /api/v1/recommendations/test`), 62 unwired mux routes (media player 46, localization 16, no `RegisterRoutes` call site), client calls without a route: api-client 31, web 69, android 22, androidtv 1; 30 double-prefix calls (web 28, android 2).
- **Alternatives considered**: swaggo v1 (produces Swagger 2.0, a step back from OpenAPI 3.0.3); swaggo v2 (release candidates only); full migration to spec-first inside this feature (too large); regex extraction as the permanent gate (cannot see helper-built routes).
- **Source**: docs/20 §9 (DR-20-05); docs/19 §5, §6.3, §8; docs/07 §12; docs/13 §10.4. Owner input: OD-63.

### R-11 Mutation-testing tools

- **Decision**: TypeScript: StrykerJS with the Vitest runner (web, api-client, TS submodules). Go: start with go-mutesting because the existing `mutation_ratchet_challenge.sh` expects `.go-mutesting.yml` and `challenges/baselines/bluff-baseline.txt` at the repository root (to be created), and pilot Gremlins per changed package; whichever proves Go 1.25.7 compatible in the build container is kept, the other dropped with the reason recorded. Rust: cargo-mutants per diff on `src-tauri`. Kotlin: open-source PIT on pure logic layers (repositories, view models). Bash: a purpose-built operator-table harness. All runs per changed file per change, full runs scheduled within the memory ceiling. SC-005 acceptance stays the reviewer-drawn sample with reviewer-authored mutations (R-16).
- **Rationale**: Principle II claims an 85% mutation score but no tool is instantiated at the root (docs/05 F-5); tools must be verified in containers before relied upon.
- **Conflict noted**: docs/05 §8.1 and DR-5 choose go-mutesting; docs/17 recommends Gremlins. Resolution above: try both in the container, keep the one that runs (both are UNCONFIRMED for Go 1.25.7).
- **Alternatives considered**: no automated mutation (cannot reach the instrument); per-script mutation helpers.
- **Source**: docs/05 §8.1, §8.2, DR-5, F-5; docs/17 recommendation (line 112), §14 items 1 and 2.

### R-12 Finding and register vocabularies reconciled

- **Decision**: Finding file `finding/1` (docs/02 §9) uses severity `S1..S5` and the seven finding types; the register row (`reg_findings`, `reg_item_ext`) uses `critical/high/medium/low/cosmetic` and an eleven-value category set. Mapping: S1->critical, S2->high, S3->medium, S4->low, S5->cosmetic; types map one-to-one onto categories, while `documentation`, `dependency`, `test_gap` and `governance` are register categories assigned to imported items or derived from the unit. Finding identity (revised after independent review, which found the file schema requiring `F-<unit>-NNN` while the DB required `FND-NNNN` and `reg_findings` had no column for the file id): ONE canonical id `FND-NNNN`, minted by the register (`reg_findings.finding_id` generated from an AUTOINCREMENT `finding_seq`, monotone, never reused, immutable), used as the file name `$AUD/findings/<finding_id>.json` (`$AUD` = `specs/001-full-project-audit-remediation/audit`; evidence blobs are stored by docs/06 under `$EV/blobs/<sha256>`) and in every cross-reference; the unit-local `F-<unit>-NNN` is kept as a stored alias `unit_alias` (column in `reg_findings`, UNIQUE, immutable, CHECK that `<unit>` equals `component_id`; required field in `finding/1`). The `ev/1` `item` field accepts only canonical ids. `should_have_been_caught_by: none` requires `should_have_been_caught_by_justification` (§11.4.238 extension). Encoded in docs/04 §5 v2, `data-model.md` §2 and `contracts/finding.schema.json`; executed in docs/04 §14.7 and contracts/README.md. Follow-up (resolved): docs/02 §9 now shows the canonical `"finding_id": "FND-0001"` with `unit_alias`; the earlier `F-<unit>-<seq>` example is gone.
- **Rationale**: the two documents were written independently; one mapping avoids two incompatible vocabularies.
- **Alternatives considered**: rewrite one document's vocabulary (both are already used by executed POCs: docs/04 DDL executed).
- **Source**: docs/02 §5, §6, §9; docs/04 §5 (`reg_findings`, `reg_item_ext`).

### R-13 Contract testing approach

- **Decision**: Both-sided contracts in Pact JSON format: consumer tests (api-client TS, web, desktop, Android, Android TV) publish contracts; the provider replays them against the real running API on a real database in a container; a `can-i-deploy` gate refuses on any red cell and on a missing contract. Start in file-based mode (contracts in the repository); add a self-hosted broker in a rootless container only if the matrix becomes unmanageable. Where a consumer language lacks a usable Pact library, generate Pact JSON from that consumer's recorded real traffic against the real provider. OpenAPI drift gates (R-10) are the provider-side schema check. WebSocket message contracts are included.
- **Conflict noted**: docs/05 DR-1 wants Pact plus a broker now; docs/17 recommends Pact file-based and defers the broker; docs/08 DR-W8-05 suggests Zod response validation first and Pact after a dependency-existence verdict. Resolution: file-based Pact first (satisfies §11.4.244 both-sides and can-i-deploy without a new service), broker deferred; Zod validation is allowed as an additional client-side runtime guard, not as the contract test.
- **Alternatives considered**: spec-only validation (cannot detect a consumer using an undocumented field); hand-built matrix or broker (re-implementation, §11.4.74); Dredd (rejected, docs/17).
- **Source**: docs/05 §9, DR-1; docs/17 recommendation (line 151); docs/08 DR-W8-05; docs/20 §9. Owner input: OD-24, OD-25.

### R-14 Performance gating method

- **Decision**: Open-model arrival-rate load (no coordinated omission); per-repetition percentiles as the sampling unit; R = 10 repetitions per side (proposed); one-sided Mann-Whitney U plus an effect-size guard (median degradation above `max(regress_threshold, MDE)`) plus a bootstrap 95% CI of the median ratio; Holm-Bonferroni across the metrics of one gate run; A/A runs establish noise floor and MDE; a `noisy` result may be re-run once, counted and surfaced, never re-run until green. Targets come from the formula in docs/14 §7.2 (baseline `B`, user-perceived threshold `U` with citation, physical floor `P`), written to `perf/targets.yaml` and approved by the owner. Disk-backed SQLite is the baseline of record. Legacy thresholds in k6 scripts, Lighthouse config and Prometheus alerts are classified, never silently kept.
- **Rationale**: SC-011 asks for project-set targets and no regression; fixed thresholds measure the host, not the code; latency distributions are skewed.
- **Alternatives considered**: closed-model VU baselines; fixed absolute thresholds; single-run comparison; Student t-test as primary; Lighthouse score as the only web gate; copying the external 30/50 ms targets (excluded by the owner).
- **Source**: docs/14 §4.1, §7, §8, §16 (D-14-01 to D-14-04); spec Q2, SC-011. Owner input: OD-50 to OD-55.

### R-15 Determinism, repetition and flaky tests

- **Decision**: Determinism of the audit is defined on finding fingerprints (SC-002), not report text. A fix is accepted with RED on the pre-fix artifact and GREEN three times on the fixed artifact with identical verdicts and stdout digests and different artifact fingerprints. New or changed tests additionally run 30 shuffled times with the race detector at authoring time (proposed tuning; the 3-run gate stays). No retry facility in any harness; a rerun may classify a flake, never pass it. Mixed verdict -> quarantine per §11.4.248 with owner and deadline, seed recorded.
- **Rationale**: three runs only detect highly flaky tests (probability of a mixed verdict at p = 0.01 is 0.030 for N = 3, 0.260 for N = 30, derived arithmetic in docs/20 §4.2).
- **Alternatives considered**: byte-identical reports (timestamps legitimately differ); retries on infrastructure flakes (forbidden); Bayesian flake scoring (needs history volume).
- **Source**: docs/02 D-06; docs/06 §5, DR-E6; docs/05 DR-9, F-7 (Playwright `retries: 2` to be removed); docs/20 §4. Owner input: OD-20.

### R-16 Mutation sampling for SC-005

- **Decision**: The reviewer, not the author, draws a reproducible random sample (seed and n recorded) from the ledger's test population, applies a mutation the author did not write, runs each sampled test three times; any survivor is a finding and the draw repeats with a fresh seed until a sample yields zero survivors. Sample size n is computed from a formula for detecting a 2% weak-test rate with high probability and recorded, then approved by the owner.
- **Source**: docs/05 §8.3; docs/06 §18. Owner input: OD-21.

### R-17 Coverage floor phase-in

- **Decision**: Per application: measure a baseline in the container, record it with a dated target; no application may fall below its baseline; new and changed code must meet 85% (diff gate) with only RED-capable tests counted. Absent instruments are installed in images or recorded as a blocked item, never an invented percentage. Existing thresholds (web 80%) and silent-failure scripts (`track-coverage.sh` with `|| true`) are findings.
- **Source**: spec FR-011, Q3; docs/05 §7, DR-2, F-4; docs/09 D-ADR-11. Owner input: OD-22.

### R-18 Documentation reachability and export design

- **Decision**: Scope is classified once in a tracked `docs/DOC_SCOPE.yaml` (classes A product, B project management, C generated record collections, D governance, E submodule, F vendor); an unclassified path fails the gate. README links to hubs under `docs/hubs/`; every document carries a `Part of` footer; class C members (1,778 tickets) are reachable through one generated, fingerprinted index page keyed on file path. The crawler rules of `poc/doc_links/crawl_links.py` (fence-aware, anchors checked, `--site-root Website` for VitePress) become the reachability gate. Exports: pandoc (html5, docx) and weasyprint (pdf) in a pinned rootless container with `SOURCE_DATE_EPOCH`, each twin embedding the source sha256; `docs/EXPORT_MANIFEST.json` maps sources to outputs; the commit script refuses a staged source whose twins are stale; class A gets md+html+pdf+docx, class B md+html+pdf; stop and ask if twins exceed 150 MB. Definitions are generated from what the system uses: schema dumped from a database after the real Go migration path (v1 to 20), route table from the router, environment keys from code, with diff gates.
- **Measured today**: 2,557 Markdown files in scope (2,551 in the stored POC run; the 6 added files are all under `specs/001-.../`), 42 reachable from README, 2,515 orphans, 123 broken links (87 with `--site-root Website`), 83 broken anchors (84), maximum depth 3; 1,778 orphans under `docs/issues`.
- **Alternatives considered**: one huge README table; link-checking SaaS; timestamps instead of fingerprints for export sync; twins generated in CI (forbidden); twins only as md+html (violates §11.4.65 without a waiver); diagrams only as PNG.
- **Source**: docs/13 §3 (D-13-01, D-13-02), §4, §7, §8, §10, §14 (D-13-03 to D-13-07); docs/19 §4, §6.2, §7. Owner input: OD-56 to OD-61.

### R-19 Constitution `post_update_hook.sh` variant

- **Decision**: Variant B (project-only) by default: run the hook with a `PATH` that excludes the `claude` binary so only project-level changes happen (`skills/` symlinks, `.mcp.json`, `.git/hooks`, `.claude/settings.json`, `.claude/skills`, `.gemini`, `.qwen`, `prompts`); every resulting change goes through independent review before commit. Variant A (also writes user-level Claude plugin state through `claude plugin marketplace add/install`) only with the owner's explicit go-ahead naming the target Claude alias. The §11.4.32 sweep runs with the substitute script set because `verify-all-constitution-rules.sh` and `verify-governance-cascade.sh` do not exist; the gap is recorded, not claimed as compliance.
- **Source**: docs/11 §7, D-2, R-3. Owner input: OD-31.

### R-20 Tool supply chain and pinning

- **Decision**: One `tools.lock` with digest-pinned, signature-verified images for every scanner and build image; the harness refuses mutable tags. Replace `docker.io/aquasec/trivy:latest` and the `curl|sh` installer; treat Trivy as optional after the March 2026 compromise; run the host exposure check and rotate atomically if positive. Detectors run in rootless containers; nothing is installed on the host. Vulnerability databases are pinned offline snapshots per audit baseline.
- **Source**: docs/20 §2, W20-01 to W20-03 (DR-20-02); docs/02 D-04, D-07; docs/17 recommendation (line 198); docs/16 §18. Owner input: OD-03.

### R-21 Containerised builds and build hosts

- **Decision**: Catalogizer Containerfiles under `build/containers/` with `digests.lock`, registered for the containers submodule's distributed build; OCI archives plus digest verification for transfer between hosts; when no qualified host is available the build is `BLOCKED` and tracked, never run on the bare host; `git` and `ssh` are not builds and run on the host.
- **Source**: docs/16 DR-16-1, DR-16-3, §18. Owner input: OD-04.

### R-22 NFS real-service tests

- **Decision**: First establish whether Catalogizer's NFS path uses a kernel mount or a userspace client. Userspace: run an unprivileged userspace NFS server container. Kernel mount: run on a host where the owner permits privilege and record `blocked` locally with the exact reason; never `privileged: true`, never rootful, never a simulation. The scanners for FTP, NFS and WebDAV are stubs (docs/07 C5); implementing or removing advertised protocol support is an owner decision (§11.4.122).
- **Source**: docs/16 §10.3 (DR-16-2), V-08; docs/07 §14.3 items 1 and 5; docs/09 D-ADR-08. Owner input: OD-05, OD-06.

### R-23 HelixQA banks: conversion to executable cases

- **Decision**: Keep the banks and convert in place: mechanical conversion for the 504 HTTP-pattern lines, human-authored assertions for the rest, review by a separate actor; schema `bank-case/3` (`contracts/bank-case.schema.json`) with a validator gate before any bank executes; v3-only fields go through a Catalogizer wrapper and sidecar first, upstream typed fields later; two lanes (deterministic gates, exploratory never gates); `skipped` is not a final status; `MANIFEST.yaml` lists every bank; a bank-id floor guards against silent loss.
- **Measured today**: 15 bank files, 1,269 cases, **0 cases valid against `bank-case/3`** (none carries `metadata.v3`; the tracked placeholder `api-auth-login` fails on its `TODO` action and missing assertion).
- **Alternatives considered**: regenerate cases with a model (oracle problem, not repeatable, loses ids); encode assertions as JSON in `expected` (untyped blob ignored by the executor).
- **Source**: docs/12 §6, §7, DR-1 to DR-4; docs/05 F-1; docs/03 F-6. Owner input: OD-45, OD-46.

### R-24 External trackers and sync

- **Decision**: No tracker is configured today. The register syncs one way (DB to tracker) through adapters; each attempt is logged in `reg_tracker_sync_log` as `SYNCED` (exit 0, remote ref, evidence), `SKIPPED` (closed reason: `credentials_absent`, `tracker_client_absent`, `unreachable`, `not_configured`, `disabled_by_operator`) or `FAILED`; a mass push needs a dry run, a pilot batch and owner approval.
- **Source**: docs/04 §10, D-2, K-7; docs/03 §5.20, open question 3. Owner input: OD-12.

### R-25 Location of "tickets" and the "constitution conflict list" (FR-002)

- **Decision**: Tickets = the 1,778 HelixQA files in `docs/issues/` plus the ANR file `issues/ANR-2026-04-08-...` and FIX-QA/DEFER-QA/FINDING ids in QA archives; conflict list = "Known Conflicts and Open Decisions" in `.specify/memory/constitution.md` (16 items, about 14 open or unconfirmed sub-items). This resolves docs/04 D-3, which was written before docs/03's inventory; the owner confirms there is no other ticket store (OD-13).
- **Source**: docs/03 §2, §5.1, §5.2, §5.12, §5.17; docs/04 D-3.

### R-26 Shared modules and the governance submodule are fixed in place

- **Decision**: Findings in owned shared modules (including `submodules/constitution`) are fixed in those modules and pushed to all of their upstreams with the same review and evidence; vendored third-party code is reported upstream and closed as an accepted exception excluded from the zero-open count.
- **Source**: spec clarification 3, FR-006, FR-008; docs/11 §6.1.

### R-27 Own-organisation list for classification

- **Decision**: Own orgs = `vasic-digital`, `HelixDevelopment`, `helixdevelopment1` (GitLab namespace) and `milos85vasic` (personal mirrors of the main repository). Both tools must use the same list, read from one data file.
- **Conflict noted**: docs/11 `submodule_verify.sh` defaults to `vasic-digital|HelixDevelopment|helixdevelopment1`; `poc/repo_verify/verify_repo.sh` defaults to `vasic-digital,HelixDevelopment,milos85vasic`. Today both yield 51 owned repositories in the recursive run only because no submodule's remotes rely solely on the missing entry; this is UNCONFIRMED until a run with the union is compared.
- **Source**: docs/11 §2.1, D-8; docs/19 §3.1. Owner input: OD-34.

### R-28 Gates without CI: the commit-push script

- **Decision**: A dedicated `scripts/commit-push-all.sh` with the docs/16 §12 stages S0 to S8 (S0 preflight, including the anti-mess sweep; S1 fetch, `--prune` allowed per docs/21 IC-36; S2 scope check; S3 cheap validation; S4 long-gate verdicts or a recorded deferral; S5 commit; S6 fast-forward push to every remote, skipped under `--local-only`; S7 verify, which runs the recursive repository verifier `scripts/repo/verify_repos.sh` (promoted from `poc/repo_verify/verify_repo.sh`) in strict mode, validates its JSON against `contracts/repo-verification-report.schema.json` and re-runs the anti-mess sweep; S8 report, written to `$EV/commit-push/<run_id>.json`) and the docs/16 exit codes. The existing blocking pre-push hook is not installed and not deleted; long gates may be deferred only with the recorded `SKIP_LONG` flag, each deferral appended to `$EV/deferrals.jsonl`. Revision 3: corrected from an earlier S0-S6 numbering taken from the superseded docs/12 §16.3 skeleton (docs/21 IC-16 binds docs/16's stages and exit codes).
- **Source**: docs/12 §16, DR-7; docs/16 §12; docs/19 §9 (promotion path). Owner input: OD-60 (binding path), resolved by docs/21 IC-16.

### R-29 Product innovation is out of scope

- **Decision**: Everything tagged `PROPOSAL` in docs/18 goes to a future feature; only `R-ADJ` research feeds fixes owned by documents 07, 08, 10 and 14.
- **Source**: docs/18 §1.

### R-30 Corrections to planning documents found during consolidation

| Document | Statement | Correction | Evidence |
|---|---|---|---|
| docs/19 §10 | `repo_verify` supports "FR-001..FR-003" | it supports FR-019, FR-020 and SC-010 (FR-001..003 are register requirements) | spec.md FR list |
| docs/13 §2.1, docs/19 §6.2 | 2,540 / 2,551 Markdown files in scope | 2,557 today; the delta is new files under `specs/001-.../` (docs 19, 20, 21 and 3 POC READMEs) | fresh crawl in this session |
| docs/06 §3.1 | `additionalProperties: false` while rule 8 writes `redacted: true` | `redacted` added to `ev/1` | `contracts/evidence-record.schema.json` |
| docs/19 §5.2 rule N1 | methods OPTIONS and HEAD ignored | ignored on the server side only; one web client call is `HEAD /api/v1/cover/{}` and appears in `client_calls_without_route` | schema validation of `route_drift` output failed until HEAD was allowed |
| docs/02 §2.1 | `pendingChanges` all zero | `pendingChanges.added = 2` today and the index predates HEAD | `codegraph status --json` in this session |

## 4. Conflicts between planning documents and how they are resolved

| # | Topic | Positions | Resolution | Reference |
|---|---|---|---|---|
| C-1 | Legacy closed tickets | docs/03 DR-2 non-terminal; docs/04 DR-5 terminal with `reverify_required` | docs/04 mechanics, counted as not done; owner decides end state | R-04, OD-10 |
| C-2 | Go mutation tool | docs/05 go-mutesting; docs/17 Gremlins | try both in container, keep the one that runs | R-11 |
| C-3 | Contract tooling | docs/05 Pact + broker; docs/17 file-based; docs/08 Zod first | file-based Pact, broker deferred, Zod as extra guard | R-13 |
| C-4 | Conflict list / tickets location | docs/04 D-3 "not found"; docs/03 found them | docs/03 inventory | R-25 |
| C-5 | Own-org list | docs/11 vs docs/19 defaults | union in one data file | R-27 |
| C-6 | SLSA framing | docs/17 "cannot both be met"; docs/20 "CI not required by the text" | docs/20 reading; L1 now, L2 via owner decision | R-01 |
| C-7 | Severity/type vocabularies | docs/02 S1..S5; docs/04 critical..cosmetic | mapping table | R-12 |
| C-8 | Count of `master` branches | docs/11: 4 (direct); POC: 7 (recursive) | both true at their scope | R-08 |

## 5. Owner decisions and inputs (consolidated)

Duplicates across documents are merged into one row; every source is listed. "Blocks" names what cannot complete without the answer. Rows marked `default` have a reversible working default the plan may use until the owner answers; rows without a default block the named work outright. Spec rule: a finding waiting for one of these stays open (FR-008, FR-025).

| Id | Question | Options | Recommendation | Blocks | Sources |
|---|---|---|---|---|---|
| OD-01 | How to state the SLSA Build level given §11.4.246 (L2) and §11.4.156 (no CI)? | A claim L1; B designate dedicated build host as hosted platform, self-assessed L2; C ask constitution owners; D hosted CI (rejected) | A now, B as target, C as the question | `docs/security/SLSA_LEVEL.md`, WS6, P8 provenance | docs/20 DR-20-01; docs/15 OQ-S3; docs/17 §7.1 |
| OD-02 | Which host holds the provenance-signing key and who may use it? | build host key; hardware token; other | key on the dedicated build host, not used by interactive sessions | provenance signing, L2 claim | docs/15 OQ-S3; docs/16 §14.2, V-15 |
| OD-03 | Treat a possible Trivy-image pull as an incident? | run mechanical check, rotate atomically if positive; ignore | run the check | W20-03, credential state | docs/20 DR-20-02 |
| OD-04 | Which remote build hosts exist and qualify (`thinker.local`, `amber.local`), and a dedicated measurement host? | list hosts and roles | owner states | every containerised build offload, perf baselines (R-14-01) | docs/16 V-02; docs/05 §15; docs/14 D-14-08, item 2 |
| OD-05 | NFS: is a privileged host available if the NFS path needs a kernel mount, or an NFS server to test against? | userspace server; privileged remote host; supply NFS host | decide after V-08 code read | NFS real-service tests | docs/16 DR-16-2; docs/07 §14.3 Q1; docs/09 D-ADR-08 |
| OD-06 | FTP/NFS/WebDAV scanners are stubs: implement or remove advertised protocol support (§11.4.122)? | implement; remove after history check | implement (advertised feature) | backend scanner findings C5 | docs/07 §14.3 Q5 |
| OD-07 | Has the credential rotation in `SECURITY_KEY_ROTATION_REQUIRED.md` been performed, per provider, and when? | yes with dates; no | owner states; treat as not rotated until proven | WS1, S-25 items (38 rows) | docs/15 OQ-S1; docs/03 open question 4 |
| OD-08 | Firebase Android key: restrict or rotate? | A restrict; B rotate | owner | WS1 | docs/15 OQ-S2 |
| OD-09 | Default admin credential (documented in banks and docs): rotate and remove from docs? | rotate; keep | rotate, treat as compromised | security findings, bank R-5 | docs/10 DR-10-09; docs/12 §6.5 |
| OD-10 | Legacy closed tickets (1,495): accept as terminal with re-verification flag, or must every one be re-proven? What happens to ones that cannot be re-proven (screens gone)? | re-prove all; accept with flag; close as obsolete with evidence | keep flag, count as not done; obsolete-with-evidence for removed screens | register import Stage 2, completion gate | docs/04 D-1, DR-5; docs/03 DR-2 |
| OD-11 | Register id prefix `ATM` and register path (`docs/workable_items.db`, generated trackers under `docs/tracking/`?) | ATM / other; path | ATM, `docs/workable_items.db` | register bootstrap (default) | docs/03 DR-1; docs/04 DR-1 |
| OD-12 | Which external trackers are "configured" (GitHub, GitLab, GitFlic, GitVerse issues, Crashlytics, Sonar)? Are there problem entries there? | list | owner states; none configured today | FR-004 sync driver, SC-001 completeness | docs/04 D-2; docs/03 open question 3 |
| OD-13 | Is there any ticket store besides `docs/issues/`, QA archives and the constitution conflict list? | yes (where) / no | confirm no | SC-001 claim | docs/04 D-3; docs/03 §5.20 |
| OD-14 | Evidence-class ranking and false-positive custody are now in SQL (`v_closure_chain`, docs/04 v2); the `obsolete_details` exemption was removed in the second review round (Obsolete now only through the non-fix decision chain); remaining: accept the producer-equals-verifier residual with the docs/04 §7.1 outside-SQL controls, whether to add an `In testing -> Obsolete` edge, and whether to propose upstream that the engine's `obsolete-details` accept a not-yet-Obsolete item | accept with controls; require more | accept with controls | register design review | docs/04 D-4, §5 limitations 3-4; docs/06 §18 |
| OD-15 | Route `report_item.sh` through a wrapper that mints ids from `reg_ids` | yes / no | yes | reporting directives on this repository | docs/04 D-5 |
| OD-16 | Category mapping from legacy labels to register categories (UX -> shortcoming, etc.) | mapping table | owner approves the table | import Stage 2 | docs/04 D-6 |
| OD-17 | Replace naive front-matter parser with a YAML parser verified on all 1,778 files | yes / no | yes | import Stage 2 (default) | docs/04 D-7 |
| OD-18 | Ticket Markdown: stay in `docs/issues/` or move to a generated `docs/register/` view? | keep path regenerated; move | keep path, regenerate from register | register exports | docs/12 OD-2 |
| OD-19 | Explain the 187-ticket difference (report counts 1,965, directory holds 1,778) | owner knowledge | owner states if known; else recorded UNCONFIRMED | SC-001 population statement | docs/03 open question 2 |
| OD-20 | Adopt 30 shuffled race runs per new or changed test at authoring time (tuning of SC-003)? | yes / no | yes, 3-run gate kept | W20-05 | docs/20 §4.3 |
| OD-21 | SC-005 sample size n and its formula | n from 2% detection target | compute and record, owner approves | SC-005 acceptance | docs/05 §8.3; docs/06 §18 |
| OD-22 | Final coverage targets and dates per application after baselines | per-app values | owner sets after TS-01 baseline | FR-011 targets | docs/05 DR-2, §15; docs/09 D-ADR-11 |
| OD-23 | Is in-memory SQLite in non-unit tests a real engine (production is go-sqlcipher)? | real if same engine; replace otherwise | read production open path, then owner confirms | F-2 test-realism findings | docs/05 DR-3 |
| OD-24 | Pact library fit per consumer language (TS, Kotlin/JVM, Rust, Go verifier) | per language | try in container, fall back to recorded traffic | FR-016 contract tests | docs/05 DR-1, §15; docs/17 §14 item 3 |
| OD-25 | Backward-compatibility window N (current plus N previous minor versions) | N value | owner sets | can-i-deploy gate | docs/05 §9.3 |
| OD-26 | Scaling tests if the API cannot run multiple replicas on its storage: n/a with an architectural finding? | n/a + finding; build support | n/a + finding | SC-004 scaling cells | docs/05 DR-6 |
| OD-27 | Which test types may use emulators (hardware-independent logic only)? | narrow; broader | hardware-independent only; device claims need devices | Android/TV cells | docs/05 DR-4; docs/10 DR-10-06 |
| OD-28 | Which devices, share servers and credentials are available, and where? (phone, tablet, Mi Box/TV, SMB share, NAS, provider credentials, Firebase console) | list by model and protocol | owner supplies list | every `blocked` test (FR-025), perf end-to-end figures | docs/12 OD-3; docs/15 OQ-S8; docs/14 item 4; docs/10 R-10-2; docs/09 §17 |
| OD-29 | Windows and macOS hosts, signing certificates and notarisation credentials | supply / not | owner supplies | desktop packaging, installer signatures | docs/15 OQ-S9; docs/09 D-ADR-06, §17 |
| OD-30 | Move third-party vendored pins (23 behind)? | report only; move all; move selected with tests | report only now | FR-017 scope (default) | docs/11 D-1 |
| OD-31 | `post_update_hook.sh` variant A (user-level Claude config) or B (project-only); which Claude alias? | A / B | B by default | constitution pull completion | docs/11 D-2 |
| OD-32 | Interpret "main branch" as default branch (`master` in 4 repositories among the direct submodules, docs/11 D-3; 7 counted recursively including nested, C-8)? | yes / rename | yes | FR-024 (default) | docs/11 D-3 |
| OD-33 | `submodules/llms_verifier`: real submodule, in-tree code, or drop the `helix_qa` replace? | three options | decide after its upstream is established | `helix_qa` health | docs/11 D-4 |
| OD-34 | Own-org list: add `helixdevelopment1` (and `milos85vasic`)? | yes / no | yes, one shared data file | classification accuracy | docs/11 D-8; docs/19 §3.1 |
| OD-35 | Docling CRLF quirk: leave and report, or exception? | leave and report | leave and report (already in `exceptions.tsv`) | SC-010 exception list (default) | docs/11 D-5 |
| OD-36 | Add the missing GitLab remote for `websocket_client_ts` | yes / no | yes | FR-019 for that module | docs/11 D-6 |
| OD-37 | Gate image: `localhost/catalogizer-builder:latest` or pinned per-language images | two options | per-language, digest-pinned | submodule gates (default) | docs/11 D-7 |
| OD-38 | Explicit go-ahead to run the constitution update path (fast-forward plus hook) | go / wait | go with variant B | constitution layer L1 | docs/11 §6.2 item 6 |
| OD-39 | Independent reviewer substrate for QA case review | as pinned (Opus xhigh) | as pinned | FR-023 for banks (default) | docs/12 OD-1 |
| OD-40 | Gradle/JDK toolchain: 17 everywhere, 21 with AGP upgrade, or status quo | a / b / c | measure first, then owner | Android builds | docs/10 DR-10-01; Known Conflict 13 |
| OD-41 | `compileSdk 35` vs AGP 8.2.2: lower to 34, AGP 8.6 to 8.13, or AGP 9 | a / b / c | decide after container warning capture | Android builds | docs/10 DR-10-02; docs/20 §7.1 |
| OD-42 | Phone offline layer and `SyncService`: wire or retire | wire; retire after §11.4.122 | wire | phone sync findings | docs/10 DR-10-03 |
| OD-43 | Crash reporting on the phone: add Crashlytics or none | add; none | none until decided | phone observability | docs/10 DR-10-04 |
| OD-44 | Cleartext and certificate model for LAN deployments (self-signed with pinning, private CA, public) and Android/TV cleartext policy | options in docs/10 §17.2 | owner | WS2/WS4, DR-10-05 | docs/15 OQ-S5; docs/10 DR-10-05 |
| OD-45 | `google-services.json` for builds without Firebase values: owner supplies or plugin made conditional | supply; conditional | owner | TV build (`blocked` until decided) | docs/10 DR-10-07 |
| OD-46 | Phone playback: implement with Media3 or retire the placeholder | implement; remove | implement | phone playback findings | docs/10 DR-10-08 |
| OD-47 | Baseline rule for the escape ratchet (seeded by the first manual-QA cycle) | as stated in docs/12 §12.4 | as stated | §11.4.238 ratchet (default) | docs/12 OD-4, DR-9 |
| OD-48 | Web auth token storage: localStorage, httpOnly cookie + CSRF, or in-memory + refresh cookie (also used in the WebSocket URL) | a / b / c | c or b, cross-owned with API | DR-W8-01, security fix | docs/08 DR-W8-01; docs/15 DR-S2 |
| OD-49 | Web state management: remove Zustand and dead aliases or adopt stores | remove; implement | decide from census | DR-W8-02 | docs/08 DR-W8-02 |
| OD-50 | Performance parameters `improve_fraction`, `ratchet_limit`, `overhead_allowance`, `stop_fraction`, `regress_threshold`, `alpha`, `R`, `max_cv`, `max_throttle` | proposed alpha 0.05, R 10, Holm | owner sets after A/A | perf targets and gate | docs/14 §8.3, §18 item 1 |
| OD-51 | Real-world library size and shape distributions for datasets DS-S/M/L | owner data | owner supplies | perf datasets | docs/14 §18 item 3 |
| OD-52 | Is a local HTTP upstream an acceptable stand-in for the image-proxy overhead test (tension with FR-025)? | yes / no | owner | D-14-07 | docs/14 D-14-07, item 5 |
| OD-53 | Approve adding a bundle-analyzer dev dependency | yes / no | yes | web bundle budget | docs/14 D-14-05, item 6 |
| OD-54 | Two variants of one operation conflict (speed vs memory): default order correctness > latency > memory > throughput | accept / change | accept | perf decision rule (default) | docs/14 §7.3 |
| OD-55 | User-perceived thresholds where no credible source exists (for example OP-10) | owner value | owner sets | that operation's target | docs/14 §7.1 |
| OD-56 | Which `.sql` path each deployment mode truly uses (Go migrations vs `initdb` SQL files) | evidence by containerised boot | prove, then owner confirms | FR-015, SC-008 schema docs | docs/13 §14 Q1; docs/01 §6.3 |
| OD-57 | Are `qa-ai-system/` and `catalog-api/challenges/` documentation live? | live / legacy | owner | doc disposition | docs/13 §14 Q2 |
| OD-58 | OpenDesign token file location for export styling; canonical token format (CSS or TS) | CSS canonical generating TS | CSS canonical | exports, DR-W8-03 (default) | docs/13 §14 Q3; docs/08 DR-W8-03 |
| OD-59 | Approve repointing `HelixQA/...` README links to targets under `submodules/helix_qa/` | approve per target | approve after existence check | 7 broken README links | docs/13 §14 Q5 |
| OD-60 | Catalogizer binding path of the §11.4.234 commit-push script | `scripts/commit-push-all.sh` / other | `scripts/commit-push-all.sh` | resolved: docs/21 IC-16 fixes `scripts/commit-push-all.sh` with docs/16's stages S0-S8 (S7 verify, S8 report); no longer blocks | docs/13 §14 Q4; docs/12 §16; docs/21 IC-16 |
| OD-61 | Export twin-size threshold (stop and ask above 150 MB) | value | 150 MB | export programme (default) | docs/13 §14 Q6, §8.1 |
| OD-62 | Commit `src-tauri/Cargo.lock` for desktop and installer | yes / no | yes | Rust audit and reproducibility | docs/20 DR-20-03; docs/09 D-ADR-07 |
| OD-63 | Spec-first (oapi-codegen) for new endpoints | yes / no / later | decide after drift test | API workflow | docs/20 DR-20-05 |
| OD-64 | Accept an Atlas account and licence for migration linting | yes / no | no; SQLite procedure checks plus squawk | schema gates | docs/20 DR-20-04 |
| OD-65 | Is public registration (`/auth/register`) intended? Are `/assets` and `/cover` intended public? | per route | owner | WS2 auth fixes | docs/15 OQ-S4 |
| OD-66 | Key management for stored share credentials (env KEK, OS keystore, vault) | three options | owner | WS3 credential encryption | docs/15 OQ-S6 |
| OD-67 | StackHawk account for HawkScan, or keep it a documented blocked item | create; blocked | owner | WS5 | docs/15 OQ-S7 |
| OD-68 | Is `go-sqlcipher` encryption actually keyed in production configuration? | yes / no | owner states, then verify | severity of C18 | docs/07 §14.3 Q3 |
| OD-69 | Is the shipped API client meant to track this server (31 client operations have no route)? | yes / no | owner | client drift findings | docs/07 §14.3 Q4; docs/19 §6.3 |
| OD-70 | Desktop: new dependency for FTP/WebDAV (crates such as `reqwest`) acceptable? | yes / no | yes after existence verdict | D-ADR-03 | docs/09 D-ADR-03 |
| OD-71 | Desktop token storage: remember-me wanted? | yes / no | owner | D-ADR-04 | docs/09 D-ADR-04 |
| OD-72 | Tauri plugins `shell`, `fs`: keep with minimal capability or remove after history check | keep / remove | owner confirms | D-ADR-09 | docs/09 D-ADR-09 |
| OD-73 | Desktop updater: in or out of scope (needs signing keys) | in / out | out until decided | D-ADR-10 | docs/09 D-ADR-10 |
| OD-74 | Is a `catalog-api` auth change acceptable for the installer's SMB routes (I-10)? | yes / no | owner | installer SMB fix | docs/09 §17 |
| OD-75 | Origin of `installer-wizard/test-results.json` | owner knowledge | owner states | test inventory | docs/05 §15 |
| OD-76 | Where large evidence blobs live (retention) and an anchor location with enforced fast-forward-only branches | remote / local store | owner provides | §11.4.268 anchor strength; out-of-tree placement of large blobs (in-tree `$EV/blobs/<sha256>` below the docs/06 §11 size split proceeds meanwhile) | docs/06 §18; docs/21 §8.6 |
| OD-77 | Removal of fake-data web features (§11.4.122), per feature | replace with real data; remove | replace | R5 web | docs/08 R5 |
| OD-78 | React Query alignment: upgrade app to v5 or lower shared peer range | upgrade; lower | decide after codemod dry run | DR-W8-06 | docs/08 DR-W8-06 |
| OD-79 | Index thresholds (Lumen min recall 0.85, max CodeGraph wrong answers) as consumer data | values | 0.85 recall; wrong answers recorded | index gate (default) | docs/02 D-02 |

**Count: 79 owner decisions and inputs** (OD-01 to OD-79). Of these, 12 carry a reversible working default that does not block work (OD-11, OD-17, OD-30, OD-32, OD-35, OD-37, OD-39, OD-47, OD-54, OD-58, OD-61, OD-79); 1 is resolved by a plan decision (OD-60, by docs/21 IC-16); the remaining 66 block the named work until answered. Every OD id is mapped to an ODG group or to the ungrouped table in docs/21 §8.6.

Decisions recorded by the documents as settled and needing no owner input (for completeness): docs/09 D-ADR-01, -02, -05; docs/12 DR-1 to DR-8; docs/13 D-13-01 to D-13-07; docs/14 D-14-01 to D-14-04, D-14-06; docs/16 DR-16-1, DR-16-3; docs/06 DR-E1 to DR-E6; docs/05 DR-7 to DR-9; docs/02 D-01, D-03 to D-07; docs/08 DR-W8-04, DR-W8-07.

## 6. Items that remain UNCONFIRMED or UNKNOWN (resolved by work, not by the owner)

| Item | How it is resolved | Source |
|---|---|---|
| Exact `workable-items` initialisation command | read engine source; `validate --db` bootstraps (executed in docs/04) | docs/03 Q1; docs/04 §5 |
| `codegraph_safe.sh` grammar; `codegraph affected` existence; Lumen embedding dimension | read headers and model card at audit start | docs/02 §16.3 |
| Real nested submodule count | measured: 97 entries (44 direct, 53 nested), 98 repositories with main | `quickstart.md` step 3 |
| Tool versions in images (staticcheck, govulncheck, gosec, load tools) | W0 pins and verifies in container | docs/07 §14.3 Q2 |
| `UNCONFIRMED` POC leads POC-F-01 (62 unwired routes) and POC-F-02 (28 double-prefix web calls) | one runtime router dump and one runtime request each | docs/19 §8 |
| Testing-diary store existence | read engine schema (`test_diary` exists in the engine) | docs/03 Q5; docs/04 §4 |
| Builder image contents, `install_upstreams` on PATH, Podman `:O` mounts | read and probe in WP-0 | docs/11 §12 |
| Gin `Engine.Routes()` signature, benchstat defaults, k6 WebSocket support | read pinned versions' documentation | docs/20 §9.4; docs/14 §18 item 7 |
