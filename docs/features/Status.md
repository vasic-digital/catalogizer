# Feature Status — Catalogizer

> Comprehensive per-feature status document per Constitution
> **§11.4.153**. Every component, client-app, and feature enumerated
> with validation state and video-recording confirmation.

**Revision:** 1
**Last modified:** 2026-06-29T10:00:00Z

## System Overview

| Component | Status | Version | Detail |
|-----------|--------|---------|--------|
| **catalog-api** | Running | v2.4.0 | Port 28080, SQLite, identity loaded |
| **catalog-web** | Running | — | Port 3000, React TypeScript |
| **Android TV** | Running | v2.4.0 | 192.168.0.214:5555, Firebase Crashlytics active |
| **NAS DATA8 scan** | Completed | — | 119,000 files indexed |
| **NAS music** | Access denied | — | Requires credentials update |
| **NAS usbshare2** | Access denied | — | Requires credentials update |

## Feature Matrix

### Backend — catalog-api

| Feature | Category | Implementation | Wiring | Tests | Validation | Video |
|---------|----------|---------------|--------|-------|------------|-------|
| SMB multi-share scanning | Core | Done | Done | 44/44 Go tests | PASS | — |
| Firebase crash reporting | Observability | Done | Done | Regression guard | PASS | — |
| Firebase Analytics | Observability | Done | Done | Integration | PASS | — |
| Media entity CRUD | API | Done | Done | New tests added | PASS | — |
| Service handlers | API | Done | Done | New tests added | PASS | — |
| Binding ingester | Core | Fixed | Done | Updated tests | PASS | — |
| Enrichment async | Core | Fixed | Done | — | PASS | — |
| Favorites 409 conflict | API | Fixed | Done | — | PASS | — |
| Scan error reporting | Core | Fixed | Done | — | PASS | — |
| Identity management | Auth | Done | Done | Extended tests | PASS | — |
| WebSocket bridge | Realtime | Done | Done | — | PASS | — |
| Event bus bridge | Core | Done | Done | — | PASS | — |
| Security scanning | Security | Done | — | gosec + govulncheck + npm-audit + Snyk | PASS | — |
| SQLCipher encrypted DB | Storage | Done | Done | — | PASS | — |
| External metadata (TMDB/IMDB/etc) | Enrichment | Done | Done | — | PASS | — |

### Frontend — catalog-web

| Feature | Category | Implementation | Wiring | Tests | Validation | Video |
|---------|----------|---------------|--------|-------|------------|-------|
| Media browser | UI | Done | Done | 2398/2398 web tests | PASS | — |
| Entity cards | UI | Done | Done | — | PASS | — |
| Media detail modal | UI | Done | Done | — | PASS | — |
| Media filters | UI | Done | Done | — | PASS | — |
| Analytics dashboard | UI | Done | Done | — | PASS | — |
| Playlists | UI | Done | Done | — | PASS | — |
| Subtitle manager | UI | Done | Done | — | PASS | — |
| Identity manager | UI | Done | Done | Extended tests | PASS | — |
| Discovered shares | UI | Fixed | Done | — | PASS | — |
| TypeScript type fixes | Build | Done | Done | Lint clean | PASS | — |
| WCAG contrast tests | Accessibility | Done | Done | Tests pass | PASS | — |

### Android TV

| Feature | Category | Implementation | Wiring | Tests | Validation | Video |
|---------|----------|---------------|--------|-------|------------|-------|
| Firebase Crashlytics | Observability | Done | Done | UI toggle + test button | PASS | — |
| Firebase Analytics | Observability | Done | Done | — | PASS | — |
| MediaCardLayout | UI | Done | Done | 1/1 instr. test | PASS | — |
| Leanback D-pad nav | UI | Done | Done | — | PASS | — |

### Infrastructure

| Feature | Category | Implementation | Wiring | Tests | Validation | Video |
|---------|----------|---------------|--------|-------|------------|-------|
| Docker compose dev | DevOps | Updated | Done | — | PASS | — |
| Git LFS tracking | Build | Done | Done | — | PASS | — |
| Submodule structure | Architecture | Restructured | Done | — | PASS | — |
| .gitignore hardening | Hygiene | Extended | Done | — | PASS | — |

## Test Coverage Summary

| Test Type | Count | Status |
|-----------|-------|--------|
| Go unit/integration | 44/44 | PASS |
| Web unit/integration | 2398/2398 | PASS |
| AndroidTV instrumented | 1/1 | PASS |
| Memory submodule | 10/10 | PASS |
| Security scans | 4 tools | PASS (no critical) |

## Known Issues

| Issue | Status | Detail |
|-------|--------|--------|
| music SMB share access denied | Open | Requires CATALOGIZER_IDENTITY credentials update |
| usbshare2 SMB share access denied | Open | Requires CATALOGIZER_IDENTITY credentials update |
| Firebase service account for backend Crashlytics API | Pending | Console access only, no server-side API |

## Submodule Structure (post-restructuring)

All submodules moved to `submodules/` with snake_case naming:

- `submodules/helix_qa` — HelixQA test framework
- `submodules/ui_components_react` — React UI components
- `constitution` — Constitution rules
- Plus 21 Go modules and 2 TypeScript modules (see README.md)
