# `.env` Variables — Canonical Reference

**Purpose:** single source of truth for every environment variable the Catalogizer + HelixQA + OCU + Containers stack reads, with direct registration links for every service that issues a credential.

**File location:** project root `/run/media/milosvasic/DATA4TB/Projects/Catalogizer/.env` (gitignored; `chmod 600`).

**Legend:**
- 🔴 **MANDATORY** — stack fails or degrades severely without it
- 🟠 **HIGHLY RECOMMENDED** — needed for Constitution §7.4 real-catalogue coverage
- 🟡 **RECOMMENDED** — enables full feature set; sensible default or fallback exists
- 🟢 **OPTIONAL** — fine-tuning / opt-in features
- ⚫ **CI-ONLY** — leave UNSET in production (kill-switches, stubs)
- ✅ already in your `.env`
- ❌ missing from your `.env`

---

## 🔝 Absolute minimum to run autonomous QA today

If you do **nothing else**, you already have everything you need:

- 🔴 `JWT_SECRET`, `ADMIN_*` ✅
- 🔴 Database + Redis creds ✅
- 🔴 At least one LLM vision provider key ✅ (you have Gemini, ASTICA, OpenRouter, Kimi + 30 more)

A Phase 6 autonomous QA session can run right now with what's in `.env`. The categories below unlock **incremental coverage**, not correctness.

---

## 🚀 Minimum to unlock Phase 4+ live cycle (full Constitution §7.4 coverage)

```bash
# §7 — real catalogue content drives browse/play/search (MANDATORY per §7.4)
TMDB_API_KEY=<v3-key>                # 🟠 https://www.themoviedb.org/settings/api
TMDB_ACCESS_TOKEN=<v4-read-token>    # 🟠 same dashboard
OMDB_API_KEY=<omdb-key>              # 🟠 https://www.omdbapi.com/apikey.aspx
FANART_TV_API_KEY=<fanart-key>       # 🟠 https://fanart.tv/get-an-api-key/
IGDB_CLIENT_ID=<twitch-app-id>       # 🟠 https://dev.twitch.tv/console/apps
IGDB_CLIENT_SECRET=<twitch-secret>   # 🟠 same console

# §8 — OCU CUDA/NVENC routing to thinker.local (MANDATORY for OCU final wiring)
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_DEFAULT_SSH_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_NAME=thinker
CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
CONTAINERS_REMOTE_HOST_1_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_LABELS=gpu=true,gpu_vendor=nvidia,cuda=12.2,nvenc=true
CONTAINERS_REMOTE_HOST_1_GPU_AUTOPROBE=true
HELIX_OCU_CUDA_ADDR=thinker.local:50060
```

Everything else is incremental enrichment.

---

## 1. 🔴 Authentication & session secrets — MANDATORY

Generate locally — no service sign-up needed.

```bash
JWT_SECRET=<32+ byte random>              # ✅ openssl rand -hex 32
ADMIN_USERNAME=admin                       # ✅
ADMIN_PASSWORD=<secure>                    # ✅ openssl rand -base64 24
HELIXAGENT_API_KEY=<secret>                # ✅ openssl rand -hex 32
```

## 2. 🔴 Database — MANDATORY

Local PostgreSQL (or SQLite fallback) — no external sign-up.

```bash
# PostgreSQL (production)
DATABASE_TYPE=postgres                     # ✅ postgres | sqlite
DATABASE_HOST=localhost                    # ✅
DATABASE_PORT=5433                         # ✅
DATABASE_NAME=catalogizer                  # ✅
DATABASE_USER=catalogizer                  # ✅
DATABASE_PASSWORD=<secure>                 # ✅
DATABASE_SSL_MODE=disable                  # ✅ (or require for remote)
DATABASE_ENCRYPTION_KEY_FILE=              # SQLite only: path of the 0600 SQLCipher key file (default ~/.config/catalogizer/db.key, auto-generated; see docs/DATABASE_ENCRYPTION.md)

# Shorthand mirrors (legacy clients)
DB_HOST / DB_PORT / DB_NAME / DB_USER / DB_PASSWORD / POSTGRES_URL  # ✅
```

## 3. 🔴 Cache / vector / eventing — MANDATORY

Local services — no sign-up.

```bash
REDIS_HOST=localhost                       # ✅
REDIS_PORT=6379                            # ✅
REDIS_PASSWORD=                            # ✅
REDIS_URL=redis://localhost:6379           # ✅
CHROMA_URL=http://localhost:8000           # ✅
QDRANT_URL=http://localhost:6333           # ✅
COGNEE_AUTH_EMAIL=<email>                  # ✅
COGNEE_AUTH_PASSWORD=<pass>                # ✅
```

## 4. 🟡 Service-level feature flags

Already set in your `.env`. Adjust only for multi-host deployments.

```bash
SVC_POSTGRESQL_ENABLED / REMOTE / HOST / PORT / REQUIRED   # ✅
SVC_REDIS_ENABLED / REMOTE / HOST / PORT                   # ✅
SVC_CHROMADB_ENABLED / REMOTE / HOST / REQUIRED            # ✅
SVC_COGNEE_ENABLED / REQUIRED                              # ✅
SVC_QDRANT_REMOTE / HOST                                   # ✅
BOOT_STRICT_MODE=false                                     # ✅
```

## 5. 🔴 Server binding — MANDATORY

```bash
SERVER_HOST=0.0.0.0                        # ✅
SERVER_PORT=8080                           # ✅
```

---

## 6. LLM providers — vision + chat

### 6.1 🔴 At least ONE vision-capable provider — MANDATORY for HelixQA autonomous QA

You already have Gemini (✅) which satisfies this requirement alone. Every other provider below strengthens fallback, reduces cost, or enables provider-specific features.

### 6.2 Cloud LLM providers — registration links

| Provider | 🔴/🟡/🟢 | Env var(s) | Registration | Pricing |
|---|---|---|---|---|
| **Google Gemini** | 🔴 (✅ present) | `GEMINI_API_KEY` | https://aistudio.google.com/apikey | Pay-as-you-go; generous free tier |
| **Anthropic (direct)** | 🟢 (OAuth covers it) | `ANTHROPIC_API_KEY` | https://console.anthropic.com/settings/keys | Pay-as-you-go |
| **Anthropic (OAuth via Claude Code)** | 🟡 (✅ present) | `CLAUDE_CODE_USE_OAUTH_CREDENTIALS=true` | Already active | — |
| **OpenAI** | 🟢 (OpenRouter routes) | `OPENAI_API_KEY` | https://platform.openai.com/api-keys | Pay-as-you-go |
| **OpenRouter** (gateway → 100+ models) | 🟡 (✅ present) | `OPENROUTER_API_KEY` | https://openrouter.ai/keys | Pay-as-you-go |
| **Astica.AI** (vision analysis) | 🟢 (✅ present) | `ASTICA_API_KEY` | https://astica.ai/vision/ | Credit-based; free tier |
| **Groq** (fast inference) | 🟢 (✅ present) | `GROQ_API_KEY` | https://console.groq.com/keys | Free tier + paid |
| **Mistral** | 🟢 (✅ present) | `MISTRAL_API_KEY` | https://console.mistral.ai/api-keys/ | Pay-as-you-go |
| **Codestral** (Mistral) | 🟢 (✅ present) | `CODESTRAL_API_KEY` | https://console.mistral.ai/codestral/ | Pay-as-you-go |
| **DeepSeek** | 🟢 (✅ present) | `DEEPSEEK_API_KEY` | https://platform.deepseek.com/api_keys | Pay-as-you-go |
| **Cerebras** (fast inference) | 🟢 (✅ present) | `CEREBRAS_API_KEY` | https://cloud.cerebras.ai/ | Waitlist / pay |
| **Cohere** | 🟢 (✅ present) | `COHERE_API_KEY` | https://dashboard.cohere.com/api-keys | Free tier + paid |
| **Hugging Face** | 🟢 (✅ present) | `HUGGINGFACE_API_KEY` | https://huggingface.co/settings/tokens | Free tier |
| **NVIDIA NIM** | 🟢 (✅ present) | `NVIDIA_API_KEY` | https://build.nvidia.com/ | Credit-based |
| **Replicate** | 🟢 (✅ present) | `REPLICATE_API_KEY` | https://replicate.com/account/api-tokens | Pay-per-prediction |
| **SambaNova** | 🟢 (✅ present) | `SAMBANOVA_API_KEY` | https://cloud.sambanova.ai/apis | Waitlist / pay |
| **Fireworks.ai** | 🟢 (✅ present) | `FIREWORKS_API_KEY` | https://fireworks.ai/account/api-keys | Pay-as-you-go |
| **Hyperbolic** | 🟢 (✅ present) | `HYPERBOLIC_API_KEY` | https://app.hyperbolic.xyz/settings | Credit-based |
| **Zhipu AI (BigModel)** — CN | 🟢 (✅ present) | `ZHIPU_API_KEY` | https://bigmodel.cn/ | Free tier (CN) |
| **Z.AI** (Zhipu international) | 🟢 (✅ present) | `ZAI_API_KEY` | https://z.ai/ | Free tier |
| **Kimi / Moonshot** | 🟢 (✅ present) | `KIMI_API_KEY` | https://platform.moonshot.cn/console/api-keys | Free tier (CN) |
| **Google Vertex AI** | 🟢 (✅ present) | `VERTEX_API_KEY` | https://console.cloud.google.com/vertex-ai | Pay-as-you-go |
| **Upstage** (Korean models) | 🟢 (✅ present) | `UPSTAGE_API_KEY` | https://console.upstage.ai/ | Free tier + paid |
| **Novita.ai** | 🟢 (✅ present) | `NOVITA_API_KEY` | https://novita.ai/settings/key-management | Pay-as-you-go |
| **Public AI** | 🟢 (✅ present) | `PUBLICAI_API_KEY` | https://publicai.co/ | Varies |
| **Sarvam** (Indic LLMs) | 🟢 (✅ present) | `SARVAM_API_KEY` | https://dashboard.sarvam.ai/ | Free tier |
| **Vulavula** (African LLMs) | 🟢 (✅ present) | `VULAVULA_API_KEY` | https://vulavula.com/ | Varies |
| **Cloudflare Workers AI** | 🟢 (✅ present) | `CLOUDFLARE_API_KEY` | https://dash.cloudflare.com/profile/api-tokens | Free tier + paid |
| **SiliconFlow** — CN | 🟢 (✅ present) | `SILICONFLOW_API_KEY` | https://cloud.siliconflow.cn/account/ak | Free tier (CN) |
| **Venice AI** | 🟢 (✅ present) | `VENICE_API_KEY` | https://venice.ai/settings/api | Pay-as-you-go |
| **Modal** (serverless compute) | 🟢 (✅ present) | `MODAL_API_KEY` + `MODAL_API_KEY_ID` | https://modal.com/settings/tokens | Free tier + paid |
| **Chutes** | 🟢 (✅ present) | `CHUTES_API_KEY` | https://chutes.ai/ | Varies |
| **Junie** (JetBrains) | 🟢 (✅ present) | `JUNIE_API_KEY` | JetBrains account — https://www.jetbrains.com/junie/ | Subscription |
| **Kilo Code** | 🟢 (✅ present) | `KILO_API_KEY` | https://kilocode.ai/ | Subscription |
| **Letta** | 🟢 (✅ present) | `LETTA_API_KEY` | https://www.letta.com/ | Free tier + paid |
| **Memo** | 🟢 (✅ present) | `MEMO_API_KEY` | Check project docs | Varies |
| **NIA** | 🟢 (✅ present) | `NIA_API_KEY` | https://trynia.ai/ | Varies |
| **NLP Cloud** | 🟢 (✅ present) | `NLP_API_KEY` | https://nlpcloud.com/home/token | Pay-as-you-go |
| **Qwen Code OAuth** | 🟡 (✅ present) | `QWEN_CODE_USE_OAUTH_CREDENTIALS=true` | Already active | — |

### 6.3 🟢 Vision pipeline tuning — OPTIONAL (sensible defaults work)

```bash
HELIX_VISION_PROVIDER=                     # 🟢 force provider (auto-ranked if unset)
HELIX_VISION_GEMINI_MODEL=gemini-2.5-flash # 🟢
HELIX_VISION_OPENAI_MODEL=gpt-4o-mini      # 🟢
HELIX_VISION_ANTHROPIC_MODEL=claude-sonnet-4-6  # 🟢
HELIX_VISION_KIMI_MODEL=moonshot-v1-32k-vision  # 🟢
HELIX_VISION_QWEN_MODEL=qwen-vl-max        # 🟢
HELIX_VISION_ASTICA_MODEL=2.5_full         # 🟢
HELIX_VISION_STEPGUI_MODEL=step-1v-32k     # 🟢
HELIX_VISION_FALLBACK_CHAIN=gemini,openrouter,kimi  # 🟢
HELIX_VISION_OPENCV_ENABLED=false          # 🟢
HELIX_VISION_CHEAPER_ENABLED=true          # 🟢
HELIX_VISION_LEARNING_ENABLED=false        # 🟢
HELIX_VISION_MAX_IMAGE_SIZE=2097152        # 🟢
HELIX_VISION_MAX_MEMORIES=1000             # 🟢
HELIX_VISION_MULTI_USER=false              # 🟢
HELIX_VISION_PERSIST_PATH=                 # 🟢
HELIX_VISION_TIMEOUT=60                    # 🟢
HELIX_VISION_CHANGE_THRESHOLD=0.1          # 🟢
HELIX_VISION_SSIM_THRESHOLD=0.95           # 🟢
HELIX_VISION_MODEL=                        # 🟢
```

### 6.4 🟢 llama.cpp RPC distributed inference — OPTIONAL (on-prem multi-host)

```bash
HELIX_LLAMACPP=true                                                # 🟢
HELIX_LLAMACPP_MODEL=qwen2.5-vl-7b-instruct.gguf                   # 🟢
HELIX_LLAMACPP_MMPROJ=mmproj-qwen2.5-vl-7b-instruct.gguf           # 🟢
HELIX_LLAMACPP_RPC_WORKERS=thinker.local:50052,amber.local:50052   # 🟢
HELIX_LLAMACPP_FREE_GPU=true                                       # 🟢
HELIX_VISION_HOST=thinker.local                                    # 🟢
HELIX_VISION_HOSTS=thinker.local,amber.local                       # 🟢
HELIX_VISION_USER=milosvasic                                       # 🟢
HELIX_OLLAMA_URL=http://thinker.local:11434                        # 🟢
HELIX_OLLAMA_MODEL=minicpm-v:8b                                    # 🟢
```

GGUF models — download from Hugging Face:
- https://huggingface.co/Qwen/Qwen2.5-VL-7B-Instruct-GGUF
- https://huggingface.co/openbmb/MiniCPM-V-2_6-gguf

---

## 7. 🟠 Metadata providers — HIGHLY RECOMMENDED

Drive real catalogue content per Constitution §7.4. Without these, HelixQA can still exercise the UI but not against real titles.

| Service | 🔴/🟠/🟢 | Env vars | Registration | Cost |
|---|---|---|---|---|
| **TMDB** (movies + TV) | 🟠 | `TMDB_API_KEY` (v3) + `TMDB_ACCESS_TOKEN` (v4 Bearer) | https://www.themoviedb.org/settings/api → Request API Key | **Free** for personal use |
| **OMDB** (fallback movie info) | 🟠 | `OMDB_API_KEY` | https://www.omdbapi.com/apikey.aspx | Free: 1000 req/day — paid tiers: $1-$15/mo |
| **Fanart.tv** (posters / fanart) | 🟠 | `FANART_TV_API_KEY` | https://fanart.tv/get-an-api-key/ | **Free** for registered users |
| **IGDB** (games, via Twitch) | 🟠 | `IGDB_CLIENT_ID` + `IGDB_CLIENT_SECRET` | https://dev.twitch.tv/console/apps/create → "Manage" → Client ID + secret | **Free** (Twitch dev account) |
| **IGDB** (alternative: pre-exchanged) | 🟢 | `IGDB_BEARER_TOKEN` | Generate via Twitch OAuth token endpoint | — |
| **Open Library** (books) | — | _no key required_ | https://openlibrary.org/developers/api | Public |
| **MusicBrainz** (music) | — | _no key required_ | https://musicbrainz.org/doc/MusicBrainz_API | Public |
| **GitHub** (software search) | 🟢 | `GITHUB_TOKEN` (already present for git) | https://github.com/settings/personal-access-tokens | Free |

**TMDB key creation walk-through:**
1. Create account: https://www.themoviedb.org/signup
2. Go to https://www.themoviedb.org/settings/api
3. Click "Request an API Key" → "Developer"
4. Fill the form (any reasonable use-case description)
5. Copy the **v3 API Key** and the **v4 API Read Access Token** — set both env vars

**Twitch/IGDB walk-through:**
1. Log in to https://dev.twitch.tv/console
2. Register a new application (category: "Website Integration"; OAuth redirect: `http://localhost` is fine)
3. Copy the Client ID
4. Click "New Secret" → copy the Client Secret
5. Our code exchanges these for a bearer token automatically

---

## 8. 🔴 OCU GPU sidecar + Containers remote distribution — MANDATORY for CUDA/NVENC path

Unlocks §2.1.1 from the session handoff — the final unwired piece of the P*.5 production wiring.

```bash
CONTAINERS_REMOTE_ENABLED=true             # ❌
CONTAINERS_REMOTE_DEFAULT_SSH_USER=milosvasic  # ❌
CONTAINERS_REMOTE_HOST_1_NAME=thinker       # ❌
CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local  # ❌
CONTAINERS_REMOTE_HOST_1_USER=milosvasic    # ❌
CONTAINERS_REMOTE_HOST_1_LABELS=gpu=true,gpu_vendor=nvidia,gpu_model=rtx3060,cuda=12.2,nvenc=true,vulkan=true  # ❌
CONTAINERS_REMOTE_HOST_1_GPU_AUTOPROBE=true # ❌
HELIX_OCU_CUDA_ADDR=thinker.local:50060     # ❌
```

No registration — operator-side config. Requires thinker.local reachable via SSH (key-auth, which already works).

---

## 9. HelixQA runtime

### 9.1 🟢 Infra health probes — OPTIONAL

```bash
HELIX_INFRA_HOST=localhost                 # 🟢
HELIX_INFRA_API_PORT=8080                  # 🟢
HELIX_INFRA_API_SERVICE=catalog-api        # 🟢
HELIX_INFRA_API_HEALTH_PATH=/api/v1/health # 🟢
HELIX_WEB_URL=http://localhost:3000        # 🟢
HELIX_QA_CLEAN_ALL_ON_NEW_RUN=true         # ✅
```

### 9.2 🟢 ADB / device control — OPTIONAL (auto-detected)

```bash
ANDROID_HOME=/home/milosvasic/Android/Sdk  # 🟢 auto-detected
ANDROID_SDK_ROOT=/home/milosvasic/Android/Sdk
HELIX_ANDROID_DEVICE=192.168.0.193:5555    # 🟢 force a serial
HELIX_ANDROID_PACKAGE=com.catalogizer.androidtv
HELIXQA_ADB_SERIAL=                        # 🟢
HELIX_ADB_EXCLUDE=ATMOSphere               # 🟢 (already enforced via .devignore)
HELIX_DESKTOP_DISPLAY=:0                   # 🟢 defaults to $DISPLAY
HELIX_FFMPEG_PATH=/home/milosvasic/bin/ffmpeg  # 🟢
HELIXQA_PLAYWRIGHT_NODE_PATH=              # 🟢
HELIX_ENV_FILE=                            # 🟢
```

### 9.3 ⚫ CI kill-switches — LEAVE UNSET in production

```bash
HELIXQA_CAPTURE_WEB_STUB=1                 # forces chromedp → ErrNotWired
HELIXQA_CAPTURE_ANDROID_STUB=1
HELIXQA_CAPTURE_LINUX_STUB=1
HELIXQA_INTERACT_WEB_STUB=1
HELIXQA_INTERACT_ANDROID_STUB=1
HELIXQA_INTERACT_LINUX_STUB=1
HELIXQA_VISION_CPU_STUB=1
HELIXQA_OBSERVE_DBUS_STUB=1
HELIXQA_OBSERVE_CDP_STUB=1
HELIXQA_OBSERVE_AX_STUB=1
HELIXQA_OBSERVE_LDPRELOAD_STUB=1
HELIXQA_RECORD_X264_STUB=1
HELIXQA_RECORD_VAAPI_STUB=1
HELIXQA_RECORD_NVENC_STUB=1
HELIXQA_VAAPI_DEVICE=/dev/dri/renderD128   # 🟢 override (not a stub)
```

### 9.4 🟢 LD_PRELOAD shim runtime — OPTIONAL

```bash
HELIXQA_LD_SHIM=/tmp/helix-shim.so         # 🟢 only when shim compiled
HELIXQA_LD_SHIM_FIFO=/tmp/helix-shim.fifo  # 🟢 operator-owned path
```

Shim compilation walk-through in `docs/hooks/README.md`.

---

## 10. 🟡 Git / release remotes — RECOMMENDED

Already present in your `.env`. Create PATs via:

| Service | Env var | Registration / PAT dashboard | Scope |
|---|---|---|---|
| **GitHub** | `GITHUB_TOKEN` ✅ | https://github.com/settings/personal-access-tokens/new | `repo`, `workflow` |
| **GitLab** | `GITLAB_TOKEN` ✅ | https://gitlab.com/-/user_settings/personal_access_tokens | `api`, `read_repository`, `write_repository` |
| **GitFlic** | `GITFLIC_TOKEN` ✅ | https://gitflic.ru/user/settings/tokens | repo read/write |
| **GitVerse** | `GITVERSE_TOKEN` ✅ | https://gitverse.ru/user/settings/tokens | repo read/write |

Models-API access (GitHub Copilot / GitHub Models):

| Env var | Registration |
|---|---|
| `GITHUB_MODELS_API_KEY` ✅ | https://github.com/marketplace/models — enable Models access on your account |

---

## 11. 🟢 Observability — OPTIONAL

| Service | Env var | Registration | Purpose |
|---|---|---|---|
| **Sentry** | `SENTRY_DSN` ❌ | https://sentry.io/settings/projects/ → Create project → copy DSN | Production error reporting |
| **Semgrep** | `SEMGREP_APP_TOKEN` ❌ | https://semgrep.dev/orgs/-/settings/tokens | Silences local Semgrep hook auth-missing warnings (cosmetic) |

---

## 12. 🟢 Container build overrides — OPTIONAL

```bash
CATALOGIZER_BUILDER_IMAGE=localhost/catalogizer-builder:latest  # 🟢
CATALOGIZER_BUILDER_AUTO=1                                      # 🟢
```

---

## 13. Database / retention of existing services (reference only)

These are used by service-layer integrations — typically no key needed unless you run your own cloud instance:

| Service | Env var | Registration | Purpose |
|---|---|---|---|
| Chroma Cloud | `CHROMA_API_KEY` ❌ (optional) | https://trychroma.com/ | Managed vector store |
| Pinecone | not used | https://www.pinecone.io/ | Alternative vector store |
| Supabase | not used | https://supabase.com/ | Alternative Postgres |

---

## ✅ Summary matrix

| # | Section | Mandatory count | Recommended count | Optional count | Status |
|---|---|---|---|---|---|
| 1 | Auth secrets | 4 | 0 | 0 | ✅ All present |
| 2 | Database | 7 | 0 | 0 | ✅ All present |
| 3 | Cache / vector | 7 | 0 | 0 | ✅ All present |
| 4 | Service flags | 0 | 13 | 0 | ✅ All present |
| 5 | Server binding | 2 | 0 | 0 | ✅ All present |
| 6.1 | 1 LLM vision key | **1** (any provider) | 0 | 0 | ✅ Gemini + 33 others present |
| 6.2 | More LLM providers | 0 | 0 | 38 | ✅ 34 present |
| 6.3 | Vision tuning | 0 | 0 | 20 | — (defaults OK) |
| 6.4 | llama.cpp RPC | 0 | 0 | 10 | — (on-prem opt-in) |
| 7 | Metadata providers | 0 | **6** (TMDB×2, OMDB, Fanart, IGDB×2) | 2 | ❌ **All 6 missing** |
| 8 | OCU CUDA/NVENC | **8** | 0 | 0 | ❌ **All 8 missing** |
| 9 | HelixQA runtime | 0 | 0 | ~25 | ✅ defaults OK |
| 10 | Git remotes | 4 | 1 | 0 | ✅ All present |
| 11 | Observability | 0 | 0 | 2 | ❌ Both missing (optional) |
| 12 | Container overrides | 0 | 0 | 2 | ✅ defaults OK |

**Total mandatory still missing: 14 variables** (6 metadata + 8 OCU).

These are the values to prepare.

---

## How to hand the values back

Option A — paste a plain block into the next chat turn:
```
TMDB_API_KEY=abc123...
TMDB_ACCESS_TOKEN=xyz789...
OMDB_API_KEY=12345678
FANART_TV_API_KEY=...
IGDB_CLIENT_ID=...
IGDB_CLIENT_SECRET=...
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_DEFAULT_SSH_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_NAME=thinker
CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
CONTAINERS_REMOTE_HOST_1_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_LABELS=gpu=true,gpu_vendor=nvidia,cuda=12.2,nvenc=true
CONTAINERS_REMOTE_HOST_1_GPU_AUTOPROBE=true
HELIX_OCU_CUDA_ADDR=thinker.local:50060
```

Option B — append directly to `.env` yourself; I'll re-audit in the next session.

---

## Per-submodule references (authoritative `.env.example` files)

If you want the exact variable list for one specific component, read its own template:

- `catalog-api/.env.example` — backend
- `catalog-web/.env.example` — frontend
- `HelixQA/.env.example` — QA orchestrator
- `LLMsVerifier/.env.example` — provider scoring
- `LLMProvider/.env.example` — provider-specific runtime
- `LLMOrchestrator/.env.example` — agent runtime
- `VisionEngine/.env.example` — vision providers
- `DocProcessor/.env.example` — document processing
- `Containers/.env.example` — container distribution
- `Challenges/Containers/.env.example` — challenge-test stack
- `OCU-CUDA-Sidecar/.env.example` — GPU sidecar
- `.env.example` (project root) — consolidated template

This document supersedes the per-submodule templates for operator setup.

---

*Last updated: 2026-04-19. Regenerated by auditing `os.Getenv` / `os.LookupEnv` call sites across HelixQA + Containers + LLMsVerifier + LLMProvider + LLMOrchestrator + VisionEngine. Registration links verified live.*
