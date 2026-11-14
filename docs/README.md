# Catalogizer Documentation

Comprehensive documentation for the Catalogizer multi-platform media collection manager.

## Quick Navigation

| Document | Description |
|----------|-------------|
| [Changelog](CHANGELOG.md) | Release notes and version history |
| [Release notes v2.4.3](release/RELEASE_NOTES_v2.4.3.md) | v2.4.3 pre-release notes (not production-ready) |
| [Upgrade Notes](UPGRADE_NOTES.md) | Behaviour changes an operator must know about when upgrading (registration approval, known limitations) |
| [Legacy Password Hash Sunset](security/LEGACY_PASSWORD_HASH_SUNSET.md) | Sunset plan for the legacy SHA-256 password hash shape (plan; verify path not removed) |
| [Installation Guide](INSTALLATION_GUIDE.md) | Getting started with Catalogizer |
| [User Guide](USER_GUIDE.md) | End-user documentation |
| [Configuration Guide](CONFIGURATION_GUIDE.md) | Application configuration options |
| [Contributing Guide](CONTRIBUTING.md) | How to contribute to the project |

---

## Architecture

| Document | Description |
|----------|-------------|
| [Architecture Overview](architecture/ARCHITECTURE.md) | System design, component interactions, and data flow |
| [Database Schema](architecture/DATABASE_SCHEMA.md) | Complete table, index, and trigger reference |
| [SQL Migrations](architecture/SQL_MIGRATIONS.md) | Migration versions, schema changes, and how to add new migrations |
| [Auth Flow](architecture/AUTH_FLOW.md) | Authentication and authorization architecture |
| [Go Backend Guide](architecture/GO_BACKEND_GUIDE.md) | Backend development patterns, Handler/Service/Repository layers |
| [React Frontend Guide](architecture/REACT_FRONTEND_GUIDE.md) | Frontend architecture, React Query, and component patterns |
| [Android Architecture](architecture/ANDROID_ARCHITECTURE.md) | MVVM architecture for Android and Android TV |
| [Tauri IPC Guide](architecture/TAURI_IPC_GUIDE.md) | Desktop application Rust/React IPC commands and events |

## API Reference

| Document | Description |
|----------|-------------|
| [API Documentation](api/API_DOCUMENTATION.md) | REST API endpoint reference for `/api/v1` |
| [WebSocket Events](api/WEBSOCKET_EVENTS.md) | Real-time event bus and WebSocket message formats |
| [Browse API](api/BROWSE_API.md) | Browse API Reference (base path `/api/v1`) |
| [Search API](api/SEARCH_API.md) | Search API Reference (base path `/api/v1`) |
| [Sync API](api/SYNC_API.md) | Sync API Reference (base path `/api/v1/sync`) |

## User and Platform Guides

| Document | Description |
|----------|-------------|
| [Web App Guide](guides/WEB_APP_GUIDE.md) | Using the React web frontend |
| [Desktop Guide](guides/DESKTOP_GUIDE.md) | Using the Tauri desktop application |
| [Android Guide](guides/ANDROID_GUIDE.md) | Using the Android mobile app |
| [Android TV Guide](guides/ANDROID_TV_GUIDE.md) | Using the Android TV lean-back app |
| [Installer Wizard Guide](guides/INSTALLER_WIZARD_GUIDE.md) | Step-by-step installation wizard |
| [Rename Detection Guide](guides/RENAME_DETECTION_GUIDE.md) | File rename detection system |
| [Rename Detection Deployment](guides/RENAME_DETECTION_DEPLOYMENT_GUIDE.md) | Deploying rename detection |
| [Troubleshooting (Guides)](guides/TROUBLESHOOTING.md) | Platform-specific troubleshooting |
| [Quick Start](guides/QUICK_START_TOMORROW.md) | Quick start reference |

## Deployment and Operations

| Document | Description |
|----------|-------------|
| [Synology hosts](infrastructure/synology-hosts.md) | Seven read-only NAS hosts: shares, access method, safety policy, survey results |
| [Deployment Guide](DEPLOYMENT_GUIDE.md) | Full deployment instructions (dev to production) |
| [Deployment (Detailed)](deployment/DEPLOYMENT.md) | Extended deployment documentation |
| [Docker Setup](deployment/DOCKER_SETUP.md) | Docker and Docker Compose configuration |
| [Minimal ffmpeg in the API image](deployment/FFMPEG_MINIMAL.md) | What the slim ffmpeg includes/excludes, licence, rebuild |
| [Environment Variables](deployment/ENVIRONMENT_VARIABLES.md) | Complete environment variable reference |
| [Monitoring Guide](deployment/MONITORING_GUIDE.md) | Metrics, alerting, and observability |
| [Production Runbook](deployment/PRODUCTION_RUNBOOK.md) | Operational procedures and incident response |
| [Backup and Recovery](deployment/BACKUP_AND_RECOVERY.md) | Database and file backup strategies |
| [Scaling Guide](deployment/SCALING_GUIDE.md) | Horizontal and vertical scaling strategies |

## Testing

| Document | Description |
|----------|-------------|
| [Testing Guide](TESTING_GUIDE.md) | How to run tests across all modules |
| [Test Strategy](testing/TEST_STRATEGY.md) | Overall testing philosophy and approach |
| [Security Testing Guide](SECURITY_TESTING_GUIDE.md) | Security scanning and penetration testing |
| [Test Results Summary](testing/TEST_RESULTS_SUMMARY.md) | Latest test execution results |
| [Test Results](testing/TEST_RESULTS.md) | Detailed test output |
| [Final Test Report](testing/FINAL_TEST_REPORT.md) | Comprehensive test verification report |
| [Real-Service Test Stack](testing/real-service-stack.md) | Rootless per-run PostgreSQL, Redis, FTP, SMB, WebDAV and NFS test servers: lifecycle scripts, round trips, leases, leak sweep |
| [Test Expansion Progress](testing/TEST_EXPANSION_PROGRESS.md) | Test coverage expansion tracking |
| [Test Implementation Summary](testing/TEST_IMPLEMENTATION_SUMMARY.md) | Test implementation details |
| [Testing Report](testing/TESTING_REPORT.md) | Testing phase report |
| [Comprehensive Test Verification](testing/COMPREHENSIVE_TEST_VERIFICATION.md) | Cross-module test verification |
| [Protocol Testing](testing/PROTOCOL_TESTING_COMPLETE.md) | Network protocol test results |

## QA

| Document | Description |
|----------|-------------|
| [QA Testing Guide](qa/QA_TESTING_GUIDE.md) | QA process and test case management |
| [Catalogizer QA](qa/CATALOGIZER_QA.md) | QA test plan and execution summary |

## Security

| Document | Description |
|----------|-------------|
| [Freemium Security Setup](FREEMIUM_SECURITY_SETUP.md) | Security configuration for freemium deployments |

## Governance

| Document | Description |
|----------|-------------|
| [Governance and the Constitution](GOVERNANCE.md) | The governance layers, the pinned canon commit, how to move the pin, the binding of the vendored engines `llmctl`, `claude_toolkit` and `open_design`, and safe submodule initialisation |
| [Spec Kit constitution](../.specify/memory/constitution.md) | Catalogizer's governance layer (version 2.4.0) |
| [Constitution appendix](../.specify/memory/constitution-appendix.md) | The operative digest of every canon anchor, the project rules and the hub-engine register |

## Project Status and Tracking

| Document | Description |
|----------|-------------|
| [Master Audit Plan](MASTER_AUDIT_AND_IMPLEMENTATION_PLAN.md) | Comprehensive audit and implementation roadmap |
| [Completion Summary](status/COMPLETION_SUMMARY.md) | Project completion status overview |
| [Comprehensive Status Report](status/COMPREHENSIVE_STATUS_REPORT.md) | Detailed status across all modules |
| [Project Dashboard](status/PROJECT_DASHBOARD.md) | High-level project metrics |
| [Implementation Task Tracker](status/IMPLEMENTATION_TASK_TRACKER.md) | Task-level tracking |
| [Implementation Report](status/IMPLEMENTATION_REPORT.md) | Implementation phase report |
| [Implementation Summary](status/IMPLEMENTATION_SUMMARY.md) | Implementation highlights |
| [Implementation Tracking](status/IMPLEMENTATION_TRACKING.md) | Ongoing implementation tracking |
| [Improvements Applied](status/IMPROVEMENTS_APPLIED.md) | List of applied improvements |
| [Improvements Summary](status/IMPROVEMENTS_SUMMARY.md) | Summary of improvement areas |
| [Installer Wizard Summary](status/INSTALLER_WIZARD_SUMMARY.md) | Installer wizard implementation status |
| [Project Completion Summary](status/PROJECT_COMPLETION_SUMMARY.md) | Final completion summary |
| [Security Integration](status/SECURITY_INTEGRATION_COMPLETE.md) | Security integration status |
| [Final Expansion Summary](status/FINAL_EXPANSION_SUMMARY.md) | Feature expansion summary |
| [Final Polish Report](status/FINAL_POLISH_REPORT.md) | Final polish pass report |
| [Status and Implementation Plan](status/COMPREHENSIVE_STATUS_AND_IMPLEMENTATION_PLAN.md) | Combined status and plan |

## Development Phases

| Document | Description |
|----------|-------------|
| [Phase 1 Complete](phases/PHASE1_COMPLETE.md) | Phase 1 completion report |
| [Phase 1 Progress](phases/PHASE1_PROGRESS_UPDATE.md) | Phase 1 progress update |
| [Phase 1 Report](phases/PHASE_1_PROGRESS_REPORT.md) | Phase 1 detailed report |
| [Phase 2 Feature Docs](phases/PHASE_2_FEATURE_DOCUMENTATION.md) | Phase 2 feature documentation |
| [Phase 2 Implementation](phases/PHASE_2_IMPLEMENTATION_DOCUMENTATION.md) | Phase 2 implementation details |
| [Phase 2 Testing Plan](phases/PHASE_2_TESTING_PLAN.md) | Phase 2 testing strategy |
| [Phase 2-4 Completion](phases/PHASE2_4_COMPLETION_REPORT.md) | Phases 2-4 completion report |
| [Phase 2 Final](phases/PHASE2_FINAL_COMPLETION.md) | Phase 2 final completion |
| [Phase 3 Plan](phases/PHASE3_IMPLEMENTATION_PLAN.md) | Phase 3 implementation plan |

## Roadmap

| Document | Description |
|----------|-------------|
| [Roadmap v3](roadmap/CATALOGIZER_ROADMAP_V3.md) | Product roadmap and future plans |

---

## Other top-level documents

Every Markdown file directly under `docs/` that is not linked in a section above. Descriptions are copied mechanically from each file.

| File | Description |
|---|---|
| [ADMIN_GUIDE.md](ADMIN_GUIDE.md) | Catalogizer -- Administrator Guide - This guide is intended for system administrators responsible for deploying, configuring, and maintaining a Catalogizer instance. It covers the backend server... |
| [AI_STACK_AUDIT.md](AI_STACK_AUDIT.md) | AI Stack Audit — Master Plan Phase 5 - **Purpose.** Master Plan v2 Phase 5 "HelixQA & AI Stack Completion" |
| [ANDROID_TV_AUDIT.md](ANDROID_TV_AUDIT.md) | Android TV Audit — Master Plan Phase 9 - **Purpose.** Master Plan v2 Phase 9 "Android TV Hardening" (7 days) |
| [API_CONTRACTS.md](API_CONTRACTS.md) | Catalogizer API Contracts - **Purpose.** Single source of truth for the operational contract between |
| [ARCHITECTURE_DIAGRAMS.md](ARCHITECTURE_DIAGRAMS.md) | HelixQA Autonomous QA Session - Architecture Diagrams |
| [BACKEND_HARDENING_AUDIT.md](BACKEND_HARDENING_AUDIT.md) | Backend Hardening Audit — Master Plan Phase 6 - **Purpose.** Master Plan v2 Phase 6 "Backend Integration Hardening" |
| [BUILD_CONTAINER_AUTO_DISPATCH.md](BUILD_CONTAINER_AUTO_DISPATCH.md) | Build Container Auto-Dispatch - The host machine often does not have every toolchain the build needs: |
| [BUILD_SYSTEM.md](BUILD_SYSTEM.md) | Build System - Catalogizer uses a modular, containerized build system with automatic versioning and change detection. The generic build framework lives in the `Build/` submodule and... |
| [COMPREHENSIVE_IMPLEMENTATION_PLAN.md](COMPREHENSIVE_IMPLEMENTATION_PLAN.md) | CATALOGIZER PROJECT - COMPREHENSIVE IMPLEMENTATION PLAN - Transform Catalogizer from 65% complete to 100% production-ready with: |
| [COMPREHENSIVE_PACKAGE_SUMMARY.md](COMPREHENSIVE_PACKAGE_SUMMARY.md) | CATALOGIZER PROJECT - COMPREHENSIVE IMPLEMENTATION PACKAGE - This comprehensive implementation package contains everything needed to transform Catalogizer from 65% to 100% complete: |
| [CONTAINERS_SUBMODULE_PROPOSAL.md](CONTAINERS_SUBMODULE_PROPOSAL.md) | Proposal for the Containers submodule (upstream `digital.vasic.containers`) - Raised by CAT-QA-STACK-LIFECYCLE-1, 2026-11-11. Nothing here is implemented. |
| [CONTINUATION.md](CONTINUATION.md) | CONTINUATION — Catalogizer - Live work-state document per Constitution **§12.10 (CONTINUATION)** and |
| [CROSS_PLATFORM_AUDIT.md](CROSS_PLATFORM_AUDIT.md) | Cross-Platform Contract Audit — Master Plan Phase 11 - **Purpose.** Master Plan v2 Phase 11 "Cross-Platform Contract |
| [CYCLE_CLOSURE_2026-04-22.md](CYCLE_CLOSURE_2026-04-22.md) | Cycle Closure — 2026-04-22 - **Session:** Article VII Master Cycle "Z-cycle v2" (2026-04-21) → |
| [DATABASE_ENCRYPTION.md](DATABASE_ENCRYPTION.md) | Database Encryption at Rest (CAT-001) - Catalogizer's main SQLite database is encrypted with **SQLCipher** (via |
| [DATA_DICTIONARY.md](DATA_DICTIONARY.md) | Catalogizer Data Dictionary - This document describes every table, column, constraint, relationship, and index in the Catalogizer database schema. The schema supports two dialects -- SQLite... |
| [DESKTOP_PERF_AUDIT.md](DESKTOP_PERF_AUDIT.md) | Desktop + Performance Audit — Master Plan Phase 10 + Phase 13 - **Purpose.** Master Plan v2 Phase 10 "Desktop Hardening" (5 days) |
| [DEVELOPER_GUIDE.md](DEVELOPER_GUIDE.md) | Catalogizer -- Developer Guide - Create a `.env` file in `catalog-api/`: |
| [DISABLED_FEATURES_AUDIT.md](DISABLED_FEATURES_AUDIT.md) | Disabled-Features Audit — Master Plan Phase 3 - **Purpose.** Master Plan Phase 3 requires "zero `.disabled` files (or |
| [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) | Disaster Recovery Plan - This Disaster Recovery Plan (DRP) provides a comprehensive framework for responding to and recovering from disruptive events that impact the Catalogizer media management... |
| [DOCUMENTATION_AUDIT.md](DOCUMENTATION_AUDIT.md) | Documentation Audit — Master Plan Phase 14 - **Purpose.** Master Plan v2 Phase 14 "Documentation Completion & Video |
| [ENV_VARIABLES.md](ENV_VARIABLES.md) | `.env` Variables — Canonical Reference - If you do **nothing else**, you already have everything you need: |
| [EXECUTIVE_SUMMARY.md](EXECUTIVE_SUMMARY.md) | Catalogizer Project Transformation - Executive Summary - This document provides a comprehensive overview of the transformation work completed on the Catalogizer project, moving it from 65% to 85% completion with a focus on... |
| [FINAL_AUDIT_ZERO_UNFINISHED_WORK.md](FINAL_AUDIT_ZERO_UNFINISHED_WORK.md) | Final Audit - Zero Unfinished Work Achieved - Status: ALL CRITICAL ISSUES RESOLVED |
| [FOLLOW_UPS_NEXT_PHASE.md](FOLLOW_UPS_NEXT_PHASE.md) | Follow-ups for the next phase(s) - Small items left after independent review rounds, documented per the operator rule of |
| [FOLLOW_UPS_SMB_FU1_FU2_INVESTIGATION.md](FOLLOW_UPS_SMB_FU1_FU2_INVESTIGATION.md) | SMB-FU-1 / SMB-FU-2 investigation (read-only) - No code was changed; items not proven by running code are marked UNCONFIRMED. |
| [FRONTEND_AUDIT.md](FRONTEND_AUDIT.md) | Frontend Audit — Master Plan Phase 7 - **Purpose.** Master Plan v2 Phase 7 "Frontend Integration Hardening" |
| [FTP_CLIENT.md](FTP_CLIENT.md) | FTP / explicit FTPS client (`pkg/ftp`) - The class of a connect-phase error comes from the PHASE, never from the server's wording: `fabric.Classify` reads credential-failure phrases ("access denied", "not... |
| [HELIXQA_AUTONOMOUS_QUICKSTART.md](HELIXQA_AUTONOMOUS_QUICKSTART.md) | HelixQA Autonomous QA Session - User Guide - Located in `qa-results/`: |
| [HOST_POWER_MANAGEMENT.md](HOST_POWER_MANAGEMENT.md) | Host Power Management — Hard Ban (CONST-033) - On 2026-04-26 18:23:43 the host running mission-critical parallel CLI |
| [LANDMINES.md](LANDMINES.md) | Catalogizer Production Landmines & Semantic Rules - **Purpose.** This document captures every unwritten rule, hidden invariant, |
| [MASTER_EXECUTION_CHECKLIST.md](MASTER_EXECUTION_CHECKLIST.md) | MASTER EXECUTION CHECKLIST - This checklist tracks execution of the comprehensive implementation plan across all 10 phases. |
| [MIGRATION_GUIDE.md](MIGRATION_GUIDE.md) | Catalogizer Migration Guide - This guide covers database migration procedures, environment variable changes, and version upgrade paths for Catalogizer deployments. |
| [NFS3_CLIENT.md](NFS3_CLIENT.md) | NFSv3 user-space client (`pkg/nfs3`) - A minimal, pure-Go, **read-only** NFSv3 client. No kernel mount, no root, no cgo, no third-party NFS code. It replaces the |
| [OPEN_POINTS_CLOSURE.md](OPEN_POINTS_CLOSURE.md) | Open Points — Closure Brief - autonomously — they need credentials, hardware, external accounts, or |
| [OUTBOUND_HTTP_CLIENTS.md](OUTBOUND_HTTP_CLIENTS.md) | Outbound HTTP clients in catalog-api (SSRF census) - Date: 2026-11-04. Scope: non-test Go code under `catalog-api/`. |
| [PHASE_15_STATUS.md](PHASE_15_STATUS.md) | Phase 15 — Final Integration & Deployment Status - **Purpose.** Master Plan v2 Phase 15 "Final Integration, Deployment |
| [PRIORITIZED_EXECUTION_ORDER.md](PRIORITIZED_EXECUTION_ORDER.md) | PRIORITIZED EXECUTION ORDER |
| [README_IMPLEMENTATION_PACKAGE.md](README_IMPLEMENTATION_PACKAGE.md) | CATALOGIZER COMPREHENSIVE IMPLEMENTATION PACKAGE - This package contains everything needed to transform Catalogizer from 65% to 100% complete, covering all unfinished work, comprehensive testing, security hardening,... |
| [ROUTE_AUTHZ_TABLE.md](ROUTE_AUTHZ_TABLE.md) | Route authorization table (authz round 2) - Generated 2026-11-03 on branch `fix-authz-round2` from a static extraction of route registrations. |
| [SECURITY_IMPROVEMENTS_SUMMARY.md](SECURITY_IMPROVEMENTS_SUMMARY.md) | Security Improvements Summary - This document summarizes all security improvements implemented in Phase 1-3 of the Catalogizer project transformation. |
| [SECURITY_INCIDENT_RESPONSE.md](SECURITY_INCIDENT_RESPONSE.md) | Security Incident Response Plan - This Security Incident Response Plan (SIRP) establishes procedures for detecting, responding to, and recovering from security incidents affecting the Catalogizer... |
| [SECURITY_SCAN_RESULTS.md](SECURITY_SCAN_RESULTS.md) | Security Scan Results - All critical security issues have been addressed. Remaining findings are either false positives or acceptable for the use case. |
| [SESSION_HANDOFF_2026-04-18.md](SESSION_HANDOFF_2026-04-18.md) | Session Handoff — 2026-04-18 - This file is a **one-time session summary + known-issues log**. After reading it, work from the canonical sources. |
| [SESSION_RESUMPTION.md](SESSION_RESUMPTION.md) | SESSION RESUMPTION — Catalogizer - Standing, out-of-the-box resumption file per Constitution **§11.4.131**. |
| [SUBMODULE_DEPENDENCIES.md](SUBMODULE_DEPENDENCIES.md) | Catalogizer Submodule Dependency Graph - **Purpose.** The repository uses **41 git submodules** from |
| [TEST_INFRASTRUCTURE_AUDIT.md](TEST_INFRASTRUCTURE_AUDIT.md) | Test Infrastructure Audit — Master Plan Phase 2 - **Purpose.** Master Plan Phase 2 "Test Infrastructure Resurrection" |
| [TEST_SUITE_DOCUMENTATION.md](TEST_SUITE_DOCUMENTATION.md) | HelixQA Autonomous QA Session - Test Suite Documentation - This document describes the comprehensive testing strategy for the HelixQA Autonomous QA Session implementation. |
| [TROUBLESHOOTING_GUIDE.md](TROUBLESHOOTING_GUIDE.md) | Catalogizer v3.0 - Troubleshooting and Maintenance Guide - This guide provides comprehensive troubleshooting procedures and maintenance tasks for Catalogizer v3.0. It covers common issues, diagnostic procedures, and preventive... |
| [UNFINISHED_WORK_COMPREHENSIVE_REPORT.md](UNFINISHED_WORK_COMPREHENSIVE_REPORT.md) | CATALOGIZER PROJECT - COMPREHENSIVE UNFINISHED WORK REPORT - The Catalogizer project has **significant unfinished work** across all dimensions: |
| [VIDEO_COURSE_SCRIPTS.md](VIDEO_COURSE_SCRIPTS.md) | Catalogizer Video Course Scripts - This document contains video course scripts for Catalogizer users, developers, and integrators. |
| [VLC_INTEGRATION.md](VLC_INTEGRATION.md) | VLC Media Player Integration - Catalogizer now includes full VLC Media Player integration for both Android TV and Desktop platforms, providing robust video/audio playback with advanced features like... |
| [android-testing-guide.md](android-testing-guide.md) | Android Testing Guide for Catalogizer - This guide covers the test infrastructure setup for Catalogizer Android applications. The goal is to achieve **95%+ test coverage** across all components. |
| [build-system.md](build-system.md) | Containerized Build System - The Catalogizer containerized build system runs all tests and builds inside Docker/Podman containers, providing a reproducible, isolated environment with integration... |
| [dependency_existence_register.md](dependency_existence_register.md) | Dependency-Existence Verdict Register - Canon section 11.4.270: every proposed or adopted dependency carries one register row with a closed-set existence verdict (VERIFIED, AMBIGUOUS, UNVERIFIED) and citable... |
| [desktop-testing-guide.md](desktop-testing-guide.md) | Desktop App Testing Guide for Catalogizer - This guide covers the test infrastructure setup for Catalogizer Desktop application (Tauri + React). The goal is to achieve **80%+ test coverage** across all components. |
| [service-discovery.md](service-discovery.md) | Service Discovery Implementation - Catalogizer now includes a service discovery mechanism that allows containers to bind to dynamic ports and clients to automatically discover service endpoints. This... |

## Complete subtree index

Every directory under `docs/`, with its entry point. A directory with its own `README.md` lists every file in it; small directories are listed here.

| Subtree | Entry point | Notes |
|---|---|---|
| `api/` | [API Documentation](api/API_DOCUMENTATION.md) | Hand-maintained API reference; linked in the API Reference section above. |
| `architecture/` | [architecture/README.md](architecture/README.md) |  |
| `audits/` | [audits/README.md](audits/README.md) |  |
| `courses/` | [courses/README.md](courses/README.md) |  |
| `database/` | [SQL_MIGRATION_REFERENCE.md](database/SQL_MIGRATION_REFERENCE.md) | small directory, files listed below |
| `decisions/` | [llms_verifier_reconciliation.md](decisions/llms_verifier_reconciliation.md), [decision_log_2026-11-02.md](decisions/decision_log_2026-11-02.md) | small directory, files listed below |
| `deployment/` | [deployment/README.md](deployment/README.md) |  |
| `design/` | [design/README.md](design/README.md) |  |
| `diagrams/` | [diagrams/README.md](diagrams/README.md) |  |
| `features/` | [Status.md](features/Status.md) | small directory, files listed below |
| `filesystem/` | [pkg_decorators.md](filesystem/pkg_decorators.md), [pkg_fabric.md](filesystem/pkg_fabric.md) | small directory, files listed below |
| `guides/` | [guides/README.md](guides/README.md) |  |
| `incidents/` | [2026-04-28-host-reboot-investigation.md](incidents/2026-04-28-host-reboot-investigation.md) | small directory, files listed below |
| `infrastructure/` | [synology-hosts.md](infrastructure/synology-hosts.md) | small directory, files listed below |
| `issues/` | [issues/](issues/) | The single tracker root for workable-item tickets (one file per item, ID-prefixed names). Not indexed file by file here; the files are generated tracker output and the directory is the canonical entry. |
| `manuals/` | [manuals/README.md](manuals/README.md) |  |
| `nexus/` | [nexus/README.md](nexus/README.md) |  |
| `phases/` | [phases/README.md](phases/README.md) |  |
| `plans/` | [plans/README.md](plans/README.md) |  |
| `qa/` | [qa/README.md](qa/README.md) |  |
| `reports/` | [reports/README.md](reports/README.md) |  |
| `requests/` | [background_queue.md](requests/background_queue.md), [history.md](requests/history.md) | small directory, files listed below |
| `research/` | [research/README.md](research/README.md) |  |
| `roadmap/` | [CATALOGIZER_ROADMAP_V3.md](roadmap/CATALOGIZER_ROADMAP_V3.md), [DECOUPLING_PLAN.md](roadmap/DECOUPLING_PLAN.md) | small directory, files listed below |
| `runbooks/` | [runbooks/README.md](runbooks/README.md) |  |
| `scripts/` | [scripts/README.md](scripts/README.md) | One companion guide per script (constitution 11.4.18); its README is the index. |
| `security/` | [security/README.md](security/README.md) |  |
| `status/` | [status/README.md](status/README.md) |  |
| `superpowers/` | [superpowers/README.md](superpowers/README.md) |  |
| `testing/` | [testing/README.md](testing/README.md) |  |
| `tutorials/` | [tutorials/README.md](tutorials/README.md) |  |
| `video-course/` | [video-course/README.md](video-course/README.md) |  |
| `vlc-integration/` | [vlc-integration/README.md](vlc-integration/README.md) | VLC integration notes; its README is the index. |
| `website/` | [website/README.md](website/README.md) |  |

### Other directories

| Path | Index |
|---|---|
| `../catalogizer-androidtv/docs/` | [Android TV module documentation index](../catalogizer-androidtv/docs/README.md) |
| `docs/CONTINUATION.md` | [Session continuation document](CONTINUATION.md) (also exported as `CONTINUATION.html` / `CONTINUATION.pdf`) |

## Documentation reachability gate

Every in-scope Markdown file must be reachable by links from the root `README.md`. Run `scripts/docs/check_doc_reachability.py` (guide: [check_doc_reachability.md](scripts/check_doc_reachability.md)); it reports orphan counts and fails when any orphan is not in the ratchet baseline or the baseline has grown. Honest headline: the gate currently reports 2456 of 2456 in-scope files reachable, but 1778 of them (72%) are reachable only because `docs/issues` is declared a tracker root (one link covers every ticket); the other 678 are reachable through explicit links in `scripts/docs/doc_reachability_baseline.txt` (the list of tolerated orphans).
