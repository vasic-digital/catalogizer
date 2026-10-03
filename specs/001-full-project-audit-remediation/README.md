# Spec 001: Full Project Audit and Remediation (document index)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-03 |
| Last modified | 2026-10-03 |
| Status | draft |
| Feature | `specs/001-full-project-audit-remediation` |
| Purpose | Entry point of this specification folder: every document in it is linked from this page, grouped by role, so each one is reachable by following links (spec FR-013, SC-006; constitution §11.4.212) |
| Reachability note | This page is reachable from [spec.md](spec.md) and from [plan.md](plan.md). For the folder to be reachable from the repository's main README, that README must link this page; that edit belongs to the README's owner and is not made here |

## Table of contents

1. [Start here](#1-start-here)
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
3. [docs/21-master-plan-phases-risks-and-traceability.md](docs/21-master-plan-phases-risks-and-traceability.md): the master plan that turns documents 01 to 20 into 8 phases and 53 work packages, with risks, owner decisions and the traceability matrix.
4. [tasks.md](tasks.md): the task list by phase and work package.
5. [quickstart.md](quickstart.md): read-only commands that show the state the plan starts from.

## 2. Specification, plan and tasks

| Document | Role |
|---|---|
| [spec.md](spec.md) | Feature specification (clarified) |
| [checklists/requirements.md](checklists/requirements.md) | Specification quality checklist |
| [plan.md](plan.md) | Implementation plan |
| [tasks.md](tasks.md) | Task list (phases P0 to P7, every work package of docs/21 section 5) |

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

## 7. Folders the work will create

These do not exist yet; tasks.md creates them and this index will link their entry documents once they exist.

| Path | Content | Owner document |
|---|---|---|
| `audit/` (`$AUD`) | audit runs, one `finding/1` file per finding, index health, golden questions | docs/02 |
| `evidence/` (`$EV`) | ledger, anchors, blobs, verdicts, reviews, human-checkpoint and phase-exit records | docs/06 section 11 |
| `decisions/` | owner-decision intake records | docs/21 section 8, tasks.md WP-01 |
| `matrix/` | test-type applicability and coverage matrix | docs/05 section 13 |
| `perf/` | performance targets and baselines | docs/14 |

## 8. Counts at a glance

Taken from docs/21 revision 6 (section 1.3); each is re-measured by the work package that relies on it.

| Quantity | Value |
|---|---|
| Phases / work packages / streams | 8 / 53 / 13 |
| Risks | 34 |
| Grouped owner decisions | 40 (ODG-01 to ODG-40), plus 16 ungrouped `research.md` decisions (docs/21 section 8.6) |
| Owner decisions in `research.md` | 79 (OD-01 to OD-79) |
| Plan-document seed findings | 253 itemised, at most 198 register items after duplicate families |
| doc18 innovation entries tracked as Feature items | 34 |
| Repositories | 98 (the main repository and 97 submodules: 44 direct, 53 nested) |
