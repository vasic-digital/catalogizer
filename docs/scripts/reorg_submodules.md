# `scripts/reorg_submodules.sh` — Submodule Reorganization (§11.4.18)

Relocates every **owned** Catalogizer submodule from the repository root into a
single `submodules/` directory using lowercase `snake_case` names, and rewrites
all path references across the repo accordingly.

> `constitution` is **already migrated** and is intentionally absent
> from the mapping — the script never touches it. Re-running is a safe no-op.

---

## Safety model

| Variable  | Default | Effect |
|-----------|---------|--------|
| `DRY_RUN` | **`1`** | Prints every action it *would* take (including unified diffs of file rewrites) and **changes nothing**. |
| `DRY_RUN=0` | — | Actually performs `git mv`, `.gitmodules` edits, `git submodule sync`/`absorbgitdirs`, and all file rewrites. |
| `FORCE`   | `0`     | Refuses to run if the working tree has **staged** changes. Set `FORCE=1` to override (only when you understand the staged set). |

- `set -euo pipefail` is active.
- Every destructive operation is guarded by `DRY_RUN`.
- Every rewrite is **idempotent**: anchored patterns mean an already-migrated
  tree reports `[skip] already up-to-date` and the move loop reports
  `[skip] already at target`.
- The script **never commits or pushes**. The operator reviews and commits.

---

## Usage

```bash
# Preview everything (safe default):
scripts/reorg_submodules.sh

# Execute the reorganization:
DRY_RUN=0 scripts/reorg_submodules.sh

# Execute even though unrelated changes are staged:
DRY_RUN=0 FORCE=1 scripts/reorg_submodules.sh
```

---

## What it does, in order

### 1–2. Move + `.gitmodules` patch (per submodule)
- `git mv <Old> submodules/<snake>` (skipped when already at target).
- Renames the `.gitmodules` section header
  `[submodule "<Old>"]` → `[submodule "submodules/<snake>"]` via an exact-match
  `awk` pass, then sets `path = submodules/<snake>` with
  `git config -f .gitmodules`. **URLs are left unchanged.**

### 3. `git submodule sync` + `git submodule absorbgitdirs`
- **`git submodule sync --recursive`** rewrites each submodule's recorded
  URL/path into `.git/config` so worktree pointers match the patched
  `.gitmodules`.
- **`git submodule absorbgitdirs`** relocates any submodule whose real git
  directory still lives inside its (now-moved) worktree into
  `.git/modules/submodules/<snake>`, and fixes the worktree's `.git` gitdir
  pointer plus the module's `core.worktree`. This is what makes the move
  consistent with git's internal bookkeeping.

### 4. Path-reference rewrites — the **BIMODAL** rule

**Consumers NOT under `submodules/`** (depth changes, so add `submodules/`):

| File | Transform | Count |
|------|-----------|-------|
| `catalog-api/go.mod` | `=> ../<Old>` → `=> ../submodules/<snake>` | 24 replace directives |
| `catalog-web/package.json` | `file:../<Old>` → `file:../submodules/<snake>` | 9 deps |
| `catalog-api/Dockerfile` | `COPY <Old>/ /build/<Old>/` → `COPY submodules/<snake>/ /build/submodules/<snake>/` | 23 module COPY lines (catalog-api COPY lines untouched) |

**Sibling consumers ALREADY under `submodules/`** (depth unchanged — keep `../`,
change only the **name**):

| File(s) | Transform |
|---------|-----------|
| React packages (`Auth-Context-React`, `Catalogizer-API-Client-TS`, `Collection-Manager-React`, `Dashboard-Analytics-React`, `Media-Browser-React`, `Media-Player-React`) | `file:../Media-Types-TS` → `file:../media_types_ts`; `file:../Catalogizer-API-Client-TS` → `file:../catalogizer_api_client_ts` |
| **Go siblings** `challenges/go.mod` (`=> ../containers`), `recovery/go.mod` (`=> ../concurrency`) | **LEFT AS-IS** — already lowercase and correct after the move. The script verifies the expected value and refuses to double-prefix; if the expected value is missing it warns instead of editing. |

**Compose files:**

| File | Transform |
|------|-----------|
| `docker-compose.qa-robot.yml` | `./HelixQA` (build) and `./HelixQA/data` (volume) → `./submodules/helix_qa[/data]` |
| `docker-compose.dev.yml` | `../Assets` → `../submodules/assets` (both occurrences on the `../Assets:/app/../Assets:ro` line) |

**Scripts** (anchored so the literal product name "HelixQA" in prose/log strings
is **not** touched — only path segments are):

| File(s) | Transform |
|---------|-----------|
| `deploy-vision-hosts.sh`, `helixqa-orchestrator.sh`, `run-helixqa*.sh` | `$PROJECT_ROOT/HelixQA/...` → `.../submodules/helix_qa/...` |
| `detect-landmines.sh` | scan target `Containers/` → `submodules/containers/`; `-d HelixQA` and `HelixQA/pkg/`, `HelixQA/cmd/` scan targets → `submodules/helix_qa/...` |
| `distributed-boot.sh`, `full-distribute.sh` | `Containers/.env` refs → `submodules/containers/.env` |
| `scripts/audit/anti-bluff-scan.sh` (and `scripts/anti-bluff-scan.sh` if present) | exclude paths `HelixQA/banks/templates`, `HelixQA/tools/opensource` → `submodules/helix_qa/...` |

### Explicitly NOT touched
`.env*` (distribution config owned by other work), `config.json`,
`versions.json` (no submodule path refs).

---

## Mapping (old path → target)

```
WebSocket-Client-TS          → submodules/websocket_client_ts
UI-Components-React          → submodules/ui_components_react
Challenges                   → submodules/challenges
Assets                       → submodules/assets
Concurrency                  → submodules/concurrency
Config                       → submodules/config
Filesystem                   → submodules/filesystem
Database                     → submodules/database
Auth                         → submodules/auth
Middleware                   → submodules/middleware
RateLimiter                  → submodules/rate_limiter
Observability                → submodules/observability
Media                        → submodules/media
Watcher                      → submodules/watcher
EventBus                     → submodules/event_bus
Cache                        → submodules/cache
Security                     → submodules/security
Storage                      → submodules/storage
Streaming                    → submodules/streaming
Discovery                    → submodules/discovery
Entities                     → submodules/entities
Media-Types-TS               → submodules/media_types_ts
Catalogizer-API-Client-TS    → submodules/catalogizer_api_client_ts
Auth-Context-React           → submodules/auth_context_react
Media-Browser-React          → submodules/media_browser_react
Dashboard-Analytics-React    → submodules/dashboard_analytics_react
Media-Player-React           → submodules/media_player_react
Collection-Manager-React     → submodules/collection_manager_react
Containers                   → submodules/containers
Lazy                         → submodules/lazy
Memory                       → submodules/memory
Recovery                     → submodules/recovery
HelixQA                      → submodules/helix_qa
DocProcessor                 → submodules/doc_processor
LLMOrchestrator              → submodules/llm_orchestrator
LLMProvider                  → submodules/llm_provider
VisionEngine                 → submodules/vision_engine
ScreenDiff                   → submodules/screen_diff
ReplayBuffer                 → submodules/replay_buffer
VisualRegression             → submodules/visual_regression
TrainingCollector            → submodules/training_collector
(constitution — already migrated, SKIPPED)
```

---

## Post-run verification checklist

The script prints this at the end; the operator must complete it:

1. `git submodule status` — every path now `submodules/<snake>`.
2. `(cd catalog-api && go build ./...)` — Go replace directives resolve.
3. `(cd catalog-api && go vet ./...)` — zero warnings (Constitution).
4. `(cd catalog-web && npm ci)` — `file:` deps resolve to `submodules/*`.
5. `docker build -f catalog-api/Dockerfile .` — `COPY submodules/<snake>/` valid.
6. Run a HelixQA script, e.g. `bash scripts/run-helixqa-api.sh`.
7. `bash scripts/audit/anti-bluff-scan.sh` — still runs with updated excludes.
8. `git diff .gitmodules` — sections + `path` updated, URLs intact.
9. Review the full working-tree diff, then commit (script does **not** commit/push).
