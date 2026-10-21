# Catalogizer Submodule Dependency Graph

> **Purpose.** The repository uses **41 git submodules** from
> `github.com/vasic-digital` (each multi-remote-pushed to GitHub, GitLab,
> GitFlic, GitVerse, and HelixDevelopment fork org). They are wired into
> the monorepo via Go `replace` directives and TypeScript `file:` links.
> This document lists every submodule, what it does, who consumes it, and
> how it is wired.
>
> **Last refresh:** 2026-04-21.

## 1. At a Glance

```
catalog-api (Go 1.25)              catalog-web (React 18/Vite)
     │                                    │
 go.mod replace ─────┐        ┌─────── package.json file:
                    ▼        ▼
       ┌───────── 21 Go submodules ─────── 9 TS submodules ────┐
       │  Auth, Cache, Config, Database,   UI-Components,       │
       │  Discovery, Entities, EventBus,   Auth-Context,        │
       │  Filesystem, Lazy, Media,         Media-Browser,       │
       │  Memory, Middleware,              Media-Player,        │
       │  Observability, RateLimiter,      Dashboard-Analytics, │
       │  Recovery, Security, Storage,     Collection-Manager,  │
       │  Streaming, Watcher, Concurrency  Media-Types,         │
       │  + Challenges (QA framework)      Catalogizer-API-     │
       │                                     Client-TS,         │
       │                                   WebSocket-Client-TS  │
       └────────────────────────────────────────────────────────┘

        HelixQA / AI stack (independent of the media app)
        ─────────────────────────────────────────────────
        HelixQA ──► Challenges, Containers, DocProcessor,
                     LLMOrchestrator, LLMProvider,
                     VisionEngine, LLMsVerifier,
                     ScreenDiff, ReplayBuffer, VisualRegression,
                     TrainingCollector

        Build framework + assets
        ─────────────────────────
        Build ── generic shell build framework
        Assets ── shared branding / icons
        Containers ── Podman helpers

```

## 2. Authoritative Source of Truth

- **`/.gitmodules`** is the canonical list of 41 submodule paths + SSH URLs.
- **`catalog-api/go.mod`** `replace` directives resolve
  `digital.vasic.*` module paths to local submodule paths. See
  `Appendix A`.
- **`catalog-web/package.json`** `file:` links resolve `@vasic-digital/*`
  to sibling directories. See `Appendix B`.
- **`catalogizer-android/app/build.gradle.kts`** uses module references
  for shared Kotlin libraries (currently no submodules — the Android app
  consumes the backend via HTTP, not via shared Kotlin code).

## 3. Go Submodules Consumed by catalog-api

21 `digital.vasic.*` Go modules. Each is a standalone Go module with
its own `go.mod`, test suite, `Upstreams/` multi-remote config, and
docs.

| Submodule path | Go module | Purpose | Consumers |
|---|---|---|---|
| `Assets` | `digital.vasic.assets` | Embedded static assets, default config | catalog-api |
| `Auth` | `digital.vasic.auth` | JWT issue / verify, password hashing | catalog-api, Middleware |
| `Cache` | `digital.vasic.cache` | In-memory + Redis-backed cache | catalog-api, Middleware |
| `Challenges` | `digital.vasic.challenges` | Challenge runner + bank loader | catalog-api, HelixQA |
| `Concurrency` | `digital.vasic.concurrency` | Semaphores, worker pools | catalog-api |
| `Config` | `digital.vasic.config` | Layered config (env > .env > json > defaults) | all Go consumers |
| `Containers` | `digital.vasic.containers` | Rootless podman helpers | catalog-api, HelixQA |
| `Database` | `digital.vasic.database` | Dual-dialect SQLite + PostgreSQL wrapper | catalog-api |
| `Discovery` | `digital.vasic.discovery` | mDNS + `.service-port` | catalog-api |
| `Entities` | `digital.vasic.entities` | Media entity schema + resolvers | catalog-api |
| `EventBus` | `digital.vasic.eventbus` | In-process pub/sub → WebSocket | catalog-api |
| `Filesystem` | `digital.vasic.filesystem` | `UnifiedClient` over SMB / FTP / NFS / WebDAV / local | catalog-api |
| `Lazy` | `digital.vasic.lazy` | LazyServiceRegistry (deferred init + DI) | catalog-api |
| `Media` | `digital.vasic.media` | Metadata providers, thumbnails, FFmpeg | catalog-api |
| `Memory` | `digital.vasic.memory` | Persistent Claude memory store | HelixQA |
| `Middleware` | `digital.vasic.middleware` | HTTP middleware (auth, rate limit, metrics, CORS) | catalog-api |
| `Observability` | `digital.vasic.observability` | Prometheus metrics, structured logs | catalog-api |
| `RateLimiter` | `digital.vasic.ratelimiter` | Redis sliding-window limiter | Middleware |
| `Recovery` | `digital.vasic.recovery` | Graceful shutdown + signal handling | catalog-api |
| `Security` | `digital.vasic.security` | SSRF guard, path traversal, goxmldsig wrapper | catalog-api |
| `Storage` | `digital.vasic.storage` | Blob / thumbnail persistence | catalog-api, Media |
| `Streaming` | `digital.vasic.streaming` | HTTP range + Smooth streaming | catalog-api |
| `Watcher` | `digital.vasic.watcher` | SMB / inotify change watching | catalog-api |

## 4. TypeScript Submodules Consumed by catalog-web

9 `@vasic-digital/*` NPM packages, resolved via `file:` sibling paths.
Each is a standalone TS package with its own `package.json`, test
suite, and build output.

| Submodule path | NPM package | Purpose |
|---|---|---|
| `Auth-Context-React` | `@vasic-digital/auth-context` | React auth provider + hook |
| `Catalogizer-API-Client-TS` | `@vasic-digital/catalogizer-api-client` | Typed REST client (OpenAPI-generated) |
| `Collection-Manager-React` | `@vasic-digital/collection-manager` | Collections UI |
| `Dashboard-Analytics-React` | `@vasic-digital/dashboard-analytics` | Dashboard charts |
| `Media-Browser-React` | `@vasic-digital/media-browser` | Media grid, filters, sort |
| `Media-Player-React` | `@vasic-digital/media-player` | Video / audio player |
| `Media-Types-TS` | `@vasic-digital/media-types` | Shared type definitions |
| `UI-Components-React` | `@vasic-digital/ui-components` | Design-system components |
| `WebSocket-Client-TS` | `@vasic-digital/websocket-client` | Auto-reconnecting WS client |

## 5. HelixQA / AI Stack (Independent)

Operate independently of catalog-api — they can be consumed by any
project. HelixQA imports them transitively.

| Submodule path | Go module / NPM | Purpose |
|---|---|---|
| `HelixQA` | `digital.vasic.helixqa` | Autonomous LLM-driven QA orchestrator |
| `DocProcessor` | `digital.vasic.docprocessor` | Document chunking + embedding |
| `LLMOrchestrator` | `digital.vasic.llmorchestrator` | Multi-provider routing |
| `LLMProvider` | `digital.vasic.llmprovider` | Provider abstractions (OpenAI, Anthropic, Gemini, …) |
| `LLMsVerifier` | `digital.vasic.llmsverifier` | Phase-specific strategy selection (Nav / Analysis / Planning) |
| `VisionEngine` | `digital.vasic.visionengine` | Image + screenshot analysis |
| `ScreenDiff` | `digital.vasic.screendiff` | Pixel-diff + DOM diff |
| `ReplayBuffer` | `digital.vasic.replaybuffer` | Session replay recorder |
| `VisualRegression` | `digital.vasic.visualregression` | Screenshot-based regression detection |
| `TrainingCollector` | `digital.vasic.trainingcollector` | Labeled-data capture |

## 6. Build + Infrastructure Submodules

| Submodule path | Purpose |
|---|---|
| `Build` | Generic shell build framework (`Build/lib/common.sh`, `version.sh`, `hash.sh`, `orchestrator.sh`) |
| `Website` | Public marketing site |

## 7. Dependency Edges (which depends on which)

```
catalog-api
  ├── Auth
  │     └── (stdlib only)
  ├── Cache
  │     └── (go-redis)
  ├── Challenges
  │     └── Containers
  ├── Concurrency
  ├── Config
  ├── Database
  │     ├── (go-sqlite3 + sqlcipher + pgx)
  │     └── Dialect rewriter
  ├── Discovery
  ├── Entities
  │     └── Database
  ├── EventBus
  ├── Filesystem
  │     ├── (smb2, ftp, webdav drivers)
  │     └── Watcher
  ├── Lazy
  ├── Media
  │     ├── (FFmpeg exec)
  │     ├── Storage
  │     └── (TMDB / OMDB / MusicBrainz / OpenLibrary clients)
  ├── Memory            ── (embeddings + sqlite store)
  ├── Middleware
  │     ├── Auth
  │     ├── RateLimiter
  │     └── Observability
  ├── Observability
  │     └── (prometheus client)
  ├── RateLimiter
  │     └── (go-redis sliding window)
  ├── Recovery
  ├── Security
  │     └── (goxmldsig, ssrf guard)
  ├── Storage
  ├── Streaming
  │     └── Storage
  └── Watcher
        └── Filesystem

catalog-web
  ├── @vasic-digital/auth-context
  │     └── @vasic-digital/catalogizer-api-client
  ├── @vasic-digital/catalogizer-api-client
  │     └── @vasic-digital/media-types
  ├── @vasic-digital/collection-manager
  │     ├── @vasic-digital/ui-components
  │     ├── @vasic-digital/media-types
  │     └── @vasic-digital/catalogizer-api-client
  ├── @vasic-digital/dashboard-analytics
  │     └── @vasic-digital/ui-components
  ├── @vasic-digital/media-browser
  │     ├── @vasic-digital/ui-components
  │     ├── @vasic-digital/media-types
  │     └── @vasic-digital/catalogizer-api-client
  ├── @vasic-digital/media-player
  │     ├── @vasic-digital/ui-components
  │     └── (hls.js, video.js)
  ├── @vasic-digital/media-types
  ├── @vasic-digital/ui-components
  │     └── (tailwind, framer-motion)
  └── @vasic-digital/websocket-client
        └── @vasic-digital/media-types

HelixQA
  ├── Challenges
  │     └── Containers
  ├── Containers
  ├── DocProcessor
  │     └── LLMProvider
  ├── LLMOrchestrator
  │     └── LLMProvider
  ├── LLMProvider
  ├── LLMsVerifier
  │     ├── LLMProvider
  │     └── VisionEngine
  ├── Memory
  ├── ReplayBuffer
  ├── ScreenDiff
  │     └── VisionEngine
  ├── TrainingCollector
  ├── VisionEngine
  └── VisualRegression
        └── VisionEngine
```

## 8. Version Pinning Strategy

- **Each submodule tracks `main`.** `.gitmodules` does not pin specific
  SHAs; the working tree SHA is what the monorepo commits reference.
- `go.mod` `replace` directives point to `../Submodule` — the local
  working copy is always used.
- `package.json` `file:../Submodule` has the same effect for TS packages.
- **Release cuts** (`releases/<component>/<version>/`) freeze the
  submodule SHAs in the release bundle — after that the monorepo can
  advance without affecting the frozen release.

## 9. Update Cadence

| Tier | Examples | Cadence |
|---|---|---|
| **Core runtime** (Database, Auth, Middleware, Filesystem, Media) | Updated as needed | On every fix or feature |
| **Infrastructure** (Config, Concurrency, Lazy, Recovery, Observability) | Rarely updated | Weekly or less |
| **QA / AI** (HelixQA, LLMOrchestrator, VisionEngine) | Iterated heavily during QA cycles | Multiple commits per day during Article VII cycles |
| **UI libraries** (UI-Components, Auth-Context, Media-Browser) | Coupled to catalog-web features | On every UI feature |
| **Shared types** (Media-Types) | Changes propagate everywhere | Only when the API contract changes |

## 10. Multi-Remote Push

Each submodule has its own 4-upstream push config (via `Upstreams/`):

- GitHub `vasic-digital`
- GitLab `vasic-digital`
- GitHub `HelixDevelopment` (fork org for HelixQA components)
- GitLab `helixdevelopment1`

The main repo has 6 upstreams: adds `GitFlic` (ru) and `GitVerse` (ru,
port 2222).

Run `install_upstreams` inside any submodule to configure its remotes;
`commit "msg"` pushes to all of them in one go.

## 11. Working with Submodules

```bash
# After clone: do NOT run `git submodule update --init --recursive` from the root. It descends through `constitution/`
# into engines that vendor a constitution of their own (see section 15). Use this instead:
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --init --recursive --
git submodule update --init constitution            # one level only
git -C constitution submodule update --init submodules/llmctl submodules/claude_toolkit submodules/open_design   # only those you need

# Sync to latest tracked branches (same rule: name the paths, skip `constitution`)
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --remote --recursive --

# Add a new submodule
./scripts/setup-submodule.sh NewModule --create-repos --go

# Commit + push one submodule to all its upstreams
cd SubmoduleName
commit "feat(x): ..."           # shell function from Upstreams/
```

## 12. Appendix A — Full `go.mod` replace block

Extracted from `catalog-api/go.mod` (authoritative):

```
replace (
  digital.vasic.assets         => ../Assets
  digital.vasic.auth           => ../Auth
  digital.vasic.cache          => ../Cache
  digital.vasic.challenges     => ../Challenges
  digital.vasic.concurrency    => ../Concurrency
  digital.vasic.config         => ../Config
  digital.vasic.containers     => ../Containers
  digital.vasic.database       => ../Database
  digital.vasic.discovery      => ../Discovery
  digital.vasic.entities       => ../Entities
  digital.vasic.eventbus       => ../EventBus
  digital.vasic.filesystem     => ../Filesystem
  digital.vasic.lazy           => ../Lazy
  digital.vasic.media          => ../Media
  digital.vasic.memory         => ../Memory
  digital.vasic.middleware     => ../Middleware
  digital.vasic.observability  => ../Observability
  digital.vasic.ratelimiter    => ../RateLimiter
  digital.vasic.recovery       => ../Recovery
  digital.vasic.security       => ../Security
  digital.vasic.storage        => ../Storage
  digital.vasic.streaming      => ../Streaming
  digital.vasic.watcher        => ../Watcher
)
```

The actual block in `go.mod` is the normative one — if this appendix
drifts, run `grep -A 30 "^replace" catalog-api/go.mod`.

## 13. Appendix B — Full `package.json` `file:` block

Extracted from `catalog-web/package.json`:

```json
{
  "dependencies": {
    "@vasic-digital/auth-context":           "file:../Auth-Context-React",
    "@vasic-digital/catalogizer-api-client": "file:../Catalogizer-API-Client-TS",
    "@vasic-digital/collection-manager":     "file:../Collection-Manager-React",
    "@vasic-digital/dashboard-analytics":    "file:../Dashboard-Analytics-React",
    "@vasic-digital/media-browser":          "file:../Media-Browser-React",
    "@vasic-digital/media-player":           "file:../Media-Player-React",
    "@vasic-digital/media-types":            "file:../Media-Types-TS",
    "@vasic-digital/ui-components":          "file:../UI-Components-React",
    "@vasic-digital/websocket-client":       "file:../WebSocket-Client-TS"
  }
}
```

Run `grep '@vasic-digital/' catalog-web/package.json` if this drifts.

## 14. Change-Impact Quick Reference

Use [the master plan's §6.2 impact matrix](research/Catalogizer_Ultimate_Master_Plan.md#62-change-impact-matrix)
when deciding what CI must run after changing a submodule. Summary:

- Any Go submodule → `catalog-api` build + all integration tests
- Any React / TS submodule → all TS packages build + their tests, then
  `catalog-web` build + lint + unit + E2E
- `catalog-api` endpoint → contract tests + all client E2E tests
- `Database` → migration test + all backend tests
- `HelixQA` → full HelixQA run against all platforms
- `Filesystem` → protocol matrix (Local / SMB / FTP / NFS / WebDAV)

## 15. Constitution Hub Engines

> Added 2026-10-21. Sections 1 to 14 predate it and describe the 41 submodules of the 2026-04-21 refresh. A read-only count on
> 2026-10-21 (`git submodule status`) lists 44 top-level submodules, `constitution` among them.

The `constitution` submodule is a hub: it carries reusable engines under `constitution/submodules/`. They are not consumed through
Go `replace` directives or `package.json` `file:` links; they are developer and QA tooling, never part of a product artifact. The
Spec Kit constitution binds three of them (section "Constitution Hub Engines"; summary and open items in
[GOVERNANCE.md](GOVERNANCE.md)):

| Engine | Origin | Gitlink at constitution `4bcb512` | In canon `helix-deps.yaml` |
|---|---|---|---|
| `llmctl` | `vasic-digital/llmctl` | `5354354b` (56 commits past `v3.2.2`) | yes, `v3.2.2`, `recursive_init: false` |
| `claude_toolkit` | `vasic-digital/claude_toolkit` | `223c7025` (`v1.30.5`) | no |
| `open_design` | `nexu-io/open-design` (third-party) | `3b0df90f30` (0.25.1) | no |

`llmctl` and `claude_toolkit` each vendor a constitution of their own, so a recursive update through `constitution/` is unsafe and
canon's own gate `CM-LLMCTL-SUBMODULE-NONRECURSIVE` currently fails at this pin. Initialise one level only (section 11 above shows the
safe form).
