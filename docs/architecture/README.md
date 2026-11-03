# Architecture index

Documents in this directory.

Back to the documentation index: [docs/README.md](../README.md). Descriptions are copied mechanically from each file (its first heading and first prose line) unless overridden in scripts/docs/index_description_overrides.tsv; non-Markdown artefacts are listed by name and type without being read. 40 entries.

| File | Description |
|---|---|
| [ANDROID_ARCHITECTURE.md](ANDROID_ARCHITECTURE.md) | Android Architecture Guide (catalogizer-android) - This guide documents the architecture, patterns, and conventions used in the catalogizer-android Kotlin/Compose application. |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Catalogizer Architecture Documentation - Catalogizer is built using a modern microservices-inspired architecture with clear separation of concerns, robust error handling, and high availability in mind. The... |
| [AUTH_FLOW.md](AUTH_FLOW.md) | Catalogizer Authentication Flow Documentation - This document describes the authentication and authorization system used across the Catalogizer platform, including JWT token handling, session management, role-based... |
| [CONCURRENCY_CONTROL.md](CONCURRENCY_CONTROL.md) | Concurrency Control Architecture - Catalogizer uses multiple concurrency patterns across its Go backend to safely handle parallel operations: mutex-based synchronization, semaphores, circuit breakers, and... |
| [CONCURRENCY_PATTERNS.md](CONCURRENCY_PATTERNS.md) | Concurrency Patterns in Catalogizer - This document describes the concurrency patterns, synchronization primitives, and best practices used throughout the Catalogizer project to ensure thread-safe,... |
| [CONCURRENCY_SAFETY_GUIDE.md](CONCURRENCY_SAFETY_GUIDE.md) | Concurrency Safety Guide - Patterns for safe concurrent access in the Catalogizer Go backend. This document covers the mutex, semaphore, channel, and circuit breaker patterns used throughout... |
| [DATABASE_SCHEMA.md](DATABASE_SCHEMA.md) | Catalogizer Database Schema - This document describes the complete database schema for the Catalogizer system. The database uses SQLite (with optional SQLCipher encryption) for development and... |
| [FILESYSTEM_INTERFACE.md](FILESYSTEM_INTERFACE.md) | Filesystem Interface Architecture - The Catalogizer filesystem interface provides a unified abstraction layer for accessing files across multiple protocols: Local, SMB, FTP, NFS, and WebDAV. This design... |
| [GO_BACKEND_GUIDE.md](GO_BACKEND_GUIDE.md) | Go Backend Architecture Guide (catalog-api) - This guide covers the architecture, patterns, and conventions used in the catalog-api Go backend. It is written for developers joining the project who need to understand... |
| [LAZY_INIT_PATTERNS.md](LAZY_INIT_PATTERNS.md) | Lazy Initialization Patterns - This document is the canonical reference for when a catalog-api service should defer expensive construction work to first use, and when it should eagerly fail at boot.... |
| [LAZY_LOADING.md](LAZY_LOADING.md) | Lazy Loading Architecture - The digital.vasic.lazy module (Lazy/ submodule) provides generic lazy-loading primitives used across the Catalogizer backend for deferred initialization of expensive... |
| [MEDIA_RECOGNITION_PIPELINE.md](MEDIA_RECOGNITION_PIPELINE.md) | Media Recognition Pipeline Architecture - The media recognition pipeline is responsible for automatically identifying media files (movies, TV shows, music, podcasts) and enriching them with metadata from... |
| [OPTIMIZATION_GUIDE.md](OPTIMIZATION_GUIDE.md) | Catalogizer Optimization Guide - This document comprehensively describes all performance optimizations implemented in Catalogizer, covering frontend React optimizations, backend Go optimizations,... |
| [REACT_FRONTEND_GUIDE.md](REACT_FRONTEND_GUIDE.md) | React Frontend Architecture Guide (catalog-web) - This guide documents the architecture, component hierarchy, state management, and conventions used in the catalog-web React/TypeScript frontend. |
| [SQL_COMPLETE_SCHEMA.md](SQL_COMPLETE_SCHEMA.md) | Complete SQL Schema Reference - This document contains the complete CREATE TABLE statements for all Catalogizer database tables, organized by database engine (SQLite and PostgreSQL) with all migrations... |
| [SQL_MIGRATIONS.md](SQL_MIGRATIONS.md) | SQL Migration Documentation - This document provides a comprehensive reference for all database migrations in the Catalogizer project, including their purpose, the tables they create or modify,... |
| [TAURI_IPC_GUIDE.md](TAURI_IPC_GUIDE.md) | Tauri IPC Commands Guide (catalogizer-desktop) - This guide documents all IPC commands and the state management architecture used in the catalogizer-desktop Tauri application. |
| [android-architecture.mmd](android-architecture.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [api-request-flow.mmd](api-request-flow.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [auth-flow.mmd](auth-flow.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [build-pipeline.mmd](build-pipeline.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [challenge-system.mmd](challenge-system.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [database-erd.mmd](database-erd.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [decisions/ADR-001-dual-database-dialect.md](decisions/ADR-001-dual-database-dialect.md) | ADR-001: Dual Database Dialect (SQLite + PostgreSQL) - Catalogizer needs to support two distinct deployment scenarios with fundamentally different database requirements: |
| [decisions/ADR-002-submodule-architecture.md](decisions/ADR-002-submodule-architecture.md) | ADR-002: Submodule Architecture & Integration Decisions - Catalogizer uses 29 independent git submodules under the vasic-digital organization. Each has its own repo, tests, and documentation. Of the 20 Go submodules, 10 are... |
| [decisions/ADR-003-http3-quic-requirement.md](decisions/ADR-003-http3-quic-requirement.md) | ADR-003: HTTP/3 (QUIC) with Brotli Compression Requirement - Catalogizer is a media collection manager that serves large file catalogs across multiple protocols (SMB, FTP, NFS, WebDAV, local). The API handles: |
| [decisions/ADR-004-challenge-based-testing.md](decisions/ADR-004-challenge-based-testing.md) | ADR-004: Challenge-Based Testing Framework - Catalogizer is a complex system that integrates multiple protocols (SMB, FTP, NFS, WebDAV, local), a dual-dialect database, real-time scanning, media entity aggregation,... |
| [decisions/ADR-005-container-only-builds.md](decisions/ADR-005-container-only-builds.md) | ADR-005: Container-Only Build and Runtime Policy - Catalogizer targets 7 platforms (Go backend, React web, Tauri desktop, Tauri installer wizard, Android, Android TV, TypeScript API client) with complex build... |
| [decisions/ADR-006-zero-warning-policy.md](decisions/ADR-006-zero-warning-policy.md) | ADR-006: Zero Warning / Zero Error Policy - Media management applications are used by non-technical users who expect a polished, reliable experience. Browser console errors, failed network requests, and... |
| [decisions/ADR-007-lazy-initialization.md](decisions/ADR-007-lazy-initialization.md) | ADR-007: Lazy Initialization via LazyServiceRegistry - Catalogizer's backend (catalog-api) registers a large number of services at startup: media detection pipelines, metadata providers (TMDB, OMDB, OpenLibrary,... |
| [decisions/ADR-008-semaphore-concurrency-control.md](decisions/ADR-008-semaphore-concurrency-control.md) | ADR-008: Semaphore-Based Concurrency Control - Catalogizer performs many operations that are naturally concurrent: scanning files across multiple storage roots, querying metadata providers (TMDB, OMDB, OpenLibrary,... |
| [decisions/ADR-009-non-blocking-ui-patterns.md](decisions/ADR-009-non-blocking-ui-patterns.md) | ADR-009: Non-Blocking UI Patterns - Catalogizer's web frontend (catalog-web) communicates with a backend API that performs inherently slow operations: scanning large media libraries across network storage... |
| [deployment-topology.mmd](deployment-topology.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [lazy-initialization-flow.mmd](lazy-initialization-flow.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [media-pipeline.mmd](media-pipeline.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [security-scanning-pipeline.mmd](security-scanning-pipeline.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [semaphore-control-flow.mmd](semaphore-control-flow.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [system-overview.mmd](system-overview.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [test-coverage-matrix.mmd](test-coverage-matrix.mmd) | (Mermaid diagram source; not read, listed by name only) |
| [websocket-events.mmd](websocket-events.mmd) | (Mermaid diagram source; not read, listed by name only) |
