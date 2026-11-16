# Catalogizer Production Landmines & Semantic Rules

> **Purpose.** This document captures every unwritten rule, hidden invariant,
> or surprising gotcha discovered across the Catalogizer multi-platform
> system. These are constraints you cannot derive from reading the code —
> they exist because of past incidents, hardware quirks, or multi-component
> interactions. Violating one produces false-positive tests, customer-facing
> bugs, or infrastructure damage.
>
> **How to use.** Before any code change, grep this file for the component
> you are touching. When a rule's **Detection** command is non-empty, run
> it; when a rule discovers a new incident, append a rule with the same
> `RULE-<scope>-NNN` pattern and increment `Last refresh`.
>
> **Last refresh:** 2026-04-21 (Article VII Master Cycle 2026-04-21-T-v2 —
> Z-cycle; added RULE-TV-004 foreground-drift guard after the RuTube
> false-positive incident).

---

## Project-Wide (CONSTITUTION.md Articles)

### RULE-CONST-001: No sudo, no root, ever

- **Context:** every operation must run as the local user. Rootful
  containers, `sudo` commands, system services are categorically
  forbidden. See CONSTITUTION.md §1.
- **Detection:**
  ```bash
  grep -rnE "\bsudo\b|su -c" scripts/ deployment/ Containers/ docker/ \
    --include="*.sh" --include="*.yml" --include="*.yaml" \
    --include="Dockerfile*" --include="Containerfile*"
  ```
- **Fix:** use rootless podman (`--userns=keep-id`), user-local systemd
  units, user-writable directories. If a step requires elevation, find a
  user-level alternative.

### RULE-CONST-002: 100% coverage across 10 categories (Article V)

- **Context:** unit, integration, E2E, full automation, stress, security,
  DDoS / rate-limit, benchmarking, challenges, HelixQA. No category may be
  incomplete before shipping.
- **Detection:** `scripts/run-all-tests.sh` must exit 0; HelixQA pipeline
  report must show 100% coverage.
- **Fix:** add the missing test; do not skip with `t.Skip()` or
  `it.skip()`.

### RULE-CONST-003: Open-Points Closure brief is authoritative

- **Context:** `docs/OPEN_POINTS_CLOSURE.md` is the single source of truth
  for operator-action items. Deleting an unclosed item is a violation.
- **Detection:** `git log --all -p -- docs/OPEN_POINTS_CLOSURE.md` — any
  commit that removes a `- [ ]` item without simultaneously ticking it is
  suspect.
- **Fix:** tick the checkbox *in the same commit* that closes the item,
  and refresh the `Last refresh:` date.

### RULE-CONST-004: Devices in `.devignore` are categorically excluded

- **Context:** `.devignore` lists devices that MUST NOT be targeted.
  `ATMOSphere rk3588_t` is listed; Mi Box 4 is not. Checked by device
  model, not IP.
- **Detection (pre-flight in every script that talks to ADB):**
  ```bash
  DEVICE_MODEL=$(adb -s "$DEVICE" shell getprop ro.product.model)
  grep -qi "$DEVICE_MODEL" .devignore && { echo "in .devignore"; exit 1; }
  ```
- **Fix:** abort the operation; never fall back to an excluded device.

### RULE-CONST-005: HTTP/3 (QUIC) + Brotli, fallback HTTP/2 + gzip

- **Context:** CLAUDE.md §HTTP. HTTP/1.1 is never acceptable in production.
  catalog-api uses `quic-go/http3` and `andybalholm/brotli`; Android uses
  OkHttp + Cronet + Brotli.
- **Detection:**
  ```bash
  grep -rn "http.DefaultClient\|&http.Client{}" catalog-api/ \
    --include="*.go" | grep -v "_test.go"
  ```
- **Fix:** construct clients via `internal/httpclient/` which wires QUIC +
  Brotli + connection pooling.

### RULE-CONST-006: Device state preservation (Article VIII)

- **Context:** a QA session must return every test device to its starting
  state. `font_scale`, `screen_off_timeout`, `screen_brightness`,
  `accelerometer_rotation` captured at Phase 0b, restored on every exit
  path.
- **Detection:** compare `adb shell settings list system` before vs after
  a HelixQA session.
- **Fix:** `pkg/autonomous/device_preserve.go` owns the capture/restore.
  Never write app code that modifies device settings.

### RULE-CONST-007: HelixQA tool hygiene (Article IX)

- **Context:** no manual `adb shell screenrecord` loops from bash, no
  `tee`-based exit-code laundering, no "✓ PASSED" log reachable without
  its assertion passing.
- **Detection:**
  ```bash
  grep -n "PIPESTATUS\|set -o pipefail" scripts/helixqa-orchestrator.sh
  # must find both; absence indicates the shell is hiding exit codes
  ```
- **Fix:** fix the Go source in `HelixQA/pkg/`, not the orchestrator bash.

---

## Go Backend (catalog-api)

### RULE-GO-001: Request context must propagate end-to-end

- **Context:** HTTP handlers must pass `c.Request.Context()` through the
  service layer to DB / external HTTP calls. `context.Background()` in a
  handler path silently leaks goroutines on client disconnect.
- **Detection:**
  ```bash
  grep -rn "context.Background()\|context.TODO()" catalog-api/handlers \
    catalog-api/internal/handlers catalog-api/services \
    catalog-api/internal/services --include="*.go" | grep -v "_test.go"
  ```
- **Fix:** accept `ctx context.Context` as the first parameter; pass
  `c.Request.Context()` from the handler. The one documented exception is
  long-running work that must outlive the HTTP request (e.g. RunAll) —
  use `context.WithoutCancel(c.Request.Context())` there, not
  `context.Background()`.

### RULE-GO-002: SQLite WAL mode must be explicit

- **Context:** go-sqlcipher ignores `_journal_mode=WAL` in the connection
  string. You must issue `PRAGMA journal_mode=WAL` after opening.
- **Detection:**
  ```bash
  grep -n "PRAGMA journal_mode" catalog-api/database/connection.go
  ```
- **Fix:** see `catalog-api/database/connection.go`.

### RULE-GO-003: PostgreSQL placeholder rewriting is mandatory

- **Context:** catalog-api supports SQLite + PostgreSQL via
  `database/dialect.go`. Raw `?` works for SQLite; PostgreSQL needs
  `$1, $2, …`. The `database.DB` wrapper rewrites automatically, but only
  if you use `db.Exec/Query/QueryRow` — not `sqlDB.Raw()`.
- **Detection:**
  ```bash
  grep -rn "\.Raw\(\|\.DB\(\)\.Exec\|\.DB\(\)\.Query" catalog-api/ \
    --include="*.go" | grep -v "_test.go"
  ```
- **Fix:** use the wrapper.

### RULE-GO-004: `LastInsertId()` is forbidden — use `InsertReturningID`

- **Context:** PostgreSQL does not implement `LastInsertId()`. The
  abstraction uses `INSERT … RETURNING id` everywhere.
- **Detection:**
  ```bash
  grep -rn "LastInsertId()" catalog-api/ --include="*.go" | grep -v _test
  ```
- **Fix:** `database.InsertReturningID()` or `TxInsertReturningID()`.

### RULE-GO-005: Rate limiting must be Redis-backed

- **Context:** in-memory rate limiting breaks on multi-instance
  deployments and can be bypassed via `X-Forwarded-For` spoofing.
- **Detection:** open `auth/middleware.go:285` — if `RateLimit` uses a
  process-local `sync.Map`, it is broken.
- **Fix:** use `digital.vasic.ratelimiter` with a Redis sliding window.
  Tests: Normal, Edge, Over-limit, IP-spoof-rejected.

### RULE-GO-006: Tests must never disable for "convenience"

- **Context:** disabled tests hide real bugs. `.go.disabled` files and
  unexplained `t.Skip()` calls both qualify.
- **Detection:**
  ```bash
  find . -name "*.go.disabled" -o -name "*.disabled.go"
  grep -rn "t.Skip(" catalog-api/ --include="*.go" | grep -v "TODO\|issue"
  ```
- **Fix:** re-enable and fix, or delete with explicit commit-message
  justification.

### RULE-GO-007: Service cleanup goroutines must be Close()-able

- **Context:** `CacheService`, `WebSocketHandler`, `WorkerPool`,
  `Throttler`, `Debouncer`, `SMBChangeWatcher` all spawn goroutines. Tests
  must `defer svc.Close()` / `defer h.Stop()` or the test binary leaks.
- **Detection:** run `go test ./... -count=1 -race` — leaks surface as
  "test binary did not finish" timeouts.
- **Fix:** constructors use `sync.Once` for Stop; WaitGroup-track every
  goroutine; Stop drains pending work.

### RULE-GO-008: Analytics calls must not block handler critical path

- **Context:** analytics adds 200-400ms latency. Don't call
  `analytics.Track()` inline inside an HTTP handler.
- **Detection:** grep handler bodies for `analytics.Track` that isn't
  wrapped in `go func()` + timeout.
- **Fix:** wrap in goroutine with a short context timeout.

### RULE-GO-009: FFmpeg wrappers must check exit code AND stderr

- **Context:** FFmpeg can report exit 0 while emitting error lines to
  stderr (codec quirks). Silent success produces corrupt output.
- **Detection:** read every FFmpeg call site; verify `cmd.Run()` error
  handling inspects `stderr.String()` when exit is 0.
- **Fix:** capture stderr into a buffer and inspect for "error", "invalid",
  "corrupt" even on exit 0.

### RULE-GO-010: SQLCipher keys must never be logged

- **Context:** database encryption key must not appear in logs, crash
  dumps, error strings, or metrics.
- **Detection:**
  ```bash
  grep -rn "DB_ENCRYPTION_KEY\|sqlcipher.*key" catalog-api/ \
    --include="*.go" | grep -E "fmt\.|log\."
  ```
- **Fix:** key in env only; redact in all logs.

### RULE-GO-011: Challenges run exclusively via system deliverables

- **Context:** all challenge operations execute via the running
  catalog-api binary or native apps — never via curl, shell scripts, or
  third-party tools. This is an Article VII invariant.
- **Detection:** grep `challenges/` test harness for direct curl calls.
- **Fix:** route through the challenge framework.

### RULE-GO-012: Migrations declare SQLite *and* PostgreSQL variants

- **Context:** `database/migrations/` has dual-dialect migrations. A
  `CREATE TABLE … AUTOINCREMENT` migration applied to PostgreSQL produces
  `syntax error near AUTOINCREMENT`.
- **Detection:** every migration must have a Postgres-compatible body (or
  be wrapped by the dialect rewriter).
- **Fix:** see migrations_v18_media_favorite.go for the dual pattern.

---

## React / Web (catalog-web)

### RULE-WEB-001: API response caching

- **Context:** `/api/v1/user` and auth endpoints must not be browser-cached.
- **Detection:** check catalog-web fetch call sites for `Cache-Control`
  usage on auth routes.
- **Fix:** set `Cache-Control: no-cache` on auth endpoints.

### RULE-WEB-002: WebSocket reconnection is mandatory

- **Context:** WebSocket disconnects are normal; the client must auto-
  reconnect with exponential backoff and a max-retry limit.
- **Detection:** inspect `WebSocketProvider` in `catalog-web/src/` —
  absence of reconnect logic = RULE-WEB-002 failure.
- **Fix:** exponential backoff 500ms → 16s with jitter; max 20 retries.

### RULE-WEB-003: Zero console warnings, zero console errors

- **Context:** warnings indicate potential runtime issues. CLAUDE.md §26.
- **Detection:** open DevTools on every route; `npm run dev` →
  `chrome --enable-logging --v=1`.
- **Fix:** resolve every warning. No deprecation warnings, no PropTypes
  warnings, no React DevTools warnings, no network 4xx/5xx in Network tab.

### RULE-WEB-004: `postcss.config.js` must use CommonJS

- **Context:** ESM `export default` in postcss.config.js breaks the
  bundler on Node 18 LTS.
- **Detection:**
  ```bash
  grep -n "export default" catalog-web/postcss.config.js
  ```
- **Fix:** `module.exports = { plugins: { … } }`.

### RULE-WEB-005: API proxy reads dynamic port

- **Context:** catalog-api writes `.service-port` at startup. The web
  dev server's vite.config.ts reads it; hardcoding 8080 breaks CI ports.
- **Detection:** read `catalog-web/vite.config.ts` proxy target resolver.
- **Fix:** keep the resolver that reads `../catalog-api/.service-port`
  with an 8080 fallback.

### RULE-WEB-006: ESLint with `--max-warnings 0`

- **Context:** see RULE-WEB-003 — warnings are errors.
- **Detection:** `cat catalog-web/package.json | jq .scripts.lint` must
  contain `--max-warnings 0`.
- **Fix:** keep the flag; never remove it under deadline pressure.

---

## Android Mobile (catalogizer-android)

### RULE-AND-001: Scoped storage on Android 13+

- **Context:** Android 13 requires scoped storage. `READ_EXTERNAL_STORAGE`
  is no longer granted via dialog.
- **Detection:** check `AndroidManifest.xml` for legacy storage
  permissions; check `FileProvider` usage.
- **Fix:** use `MediaStore` API for media access.

### RULE-AND-002: Foreground service for sync

- **Context:** background sync is killed by Doze mode. Must run as a
  foreground service with a persistent notification.
- **Detection:** `SyncService` must extend `Service` with
  `startForeground()` in `onCreate`.
- **Fix:** persistent notification channel + `FOREGROUND_SERVICE_DATA_SYNC`
  permission on API 34+.

### RULE-AND-003: ProGuard rules for every library

- **Context:** R8/ProGuard strips code that looks unused. Retrofit, Gson,
  Hilt annotations and Room entities must be `-keep`-ed.
- **Detection:** release-build crash with
  `java.lang.ClassNotFoundException` usually = missing ProGuard rule.
- **Fix:** add `-keep class com.catalogizer.android.data.model.** { *; }`
  per library. Run `./gradlew assembleRelease` + smoke-test.

### RULE-AND-004: JDK 21 needs `--add-opens` for kapt

- **Context:** kapt on JDK 21 fails with `InaccessibleObjectException`
  unless the JVM exposes `java.base/sun.reflect.annotation`.
- **Detection:** `./gradlew assembleDebug` fails with "cannot access class
  sun.reflect…" = missing --add-opens.
- **Fix:** `org.gradle.jvmargs=--add-opens=java.base/sun.reflect.annotation=ALL-UNNAMED`
  in `gradle.properties`.

---

## Android TV (catalogizer-androidtv)

### RULE-TV-001: HTTP/1.1 only on Mi Box 4 (API 28)

- **Context:** the Android TV module's OkHttpClient must force HTTP/1.1
  on Android 9 — some chipsets (including Mi Box 4) fail the HTTP/2
  handshake.
- **Detection:** grep the OkHttpClient.Builder usage in the TV module.
- **Fix:** `.protocols(listOf(Protocol.HTTP_1_1))`.

### RULE-TV-002: Every element must be D-pad focusable

- **Context:** TV users navigate with D-pad only. Every interactive element
  implements `Focusable` with a visible focus indicator.
- **Detection:** HelixQA D-pad walk session — any unreachable element is
  a rule-break.
- **Fix:** `android:focusable="true"` + `android:focusableInTouchMode="true"`
  + `nextFocus*` when traversal needs guidance.

### RULE-TV-003: Use Leanback fragments for TV UI

- **Context:** `BrowseSupportFragment`, `DetailsSupportFragment`,
  `PlaybackSupportFragment`, `SearchSupportFragment`, Watch Next row on
  home screen — not stock Android Views.
- **Detection:** grep TV module for `RecyclerView`, `ViewPager2` where
  Leanback would be idiomatic.
- **Fix:** migrate to Leanback; it carries D-pad focus + row/card
  rendering for free.

### RULE-TV-004: Structured bank tests must verify app foreground per step

- **Context (2026-04-21 incident):** Android TV's launcher aggregates
  TvContractCompat channels from every app that publishes them (RuTube,
  IPTV Pro, mitv-videoplayer, YouTube TV). A stray DPAD_ENTER on a
  foreign channel tile launches that app; subsequent keypresses land in
  the wrong UI and LLM vision verifies generic "home screen" prompts as
  TRUE, producing 80+ consecutive false-positive PASSes in a session
  where Catalogizer was never the active app.
- **Detection:**
  ```bash
  adb -s $DEVICE shell dumpsys window windows | grep mCurrentFocus
  # must contain com.catalogizer.androidtv
  ```
- **Fix:** `HelixQA/pkg/autonomous/structured_executor.go` —
  `ensureAppForeground()` runs before every step; emits CRITICAL finding
  on drift and force-launches the target activity. `.env`
  `HELIX_COMPETING_APP_PACKAGES` force-stops known channel publishers at
  phase start.

### RULE-TV-005: Android 9 `cmd input keyevent` is a no-op

- **Context (2026-04-21 incident):** on Android 9 / SDK 28,
  `cmd input keyevent KEYCODE_DPAD_CENTER` returns exit 0 with stdout
  "No shell command implementation." — the keypress is silently dropped.
- **Detection:** `HelixQA/pkg/navigator/executor.go#adbOutputIndicatesNoOp`.
- **Fix:** fall back to legacy `input keyevent` (without the `cmd`
  prefix) — mandatory for API ≤ 28.

### RULE-TV-006: `screenrecord` caps at 180s per invocation

- **Context:** Android's built-in `screenrecord` binary terminates at 180
  seconds. Long sessions need segment + concat.
- **Detection:** inspect `HelixQA/pkg/video/scrcpy.go` — segment loop +
  ffmpeg concat.
- **Fix:** `scrcpy.go` spawns segment goroutine tied to
  `context.Background()` (not caller ctx, or it dies on phase end);
  `ffmpeg -f concat -c copy` merges on Stop.

---

## Desktop (catalogizer-desktop, installer-wizard)

### RULE-DESK-001: No Rust `unwrap()` in release code

- **Context:** `unwrap()` panics abort the entire Tauri app on any
  unexpected state.
- **Detection:**
  ```bash
  grep -rn "unwrap()" catalogizer-desktop/src-tauri/src \
    installer-wizard/src-tauri/src --include="*.rs" | grep -v "// SAFE"
  ```
- **Fix:** proper `Result` handling; use `?` operator; document every
  remaining `unwrap()` with `// SAFE: <reason>` on the preceding line.

### RULE-DESK-002: Tauri file-protocol allowlist

- **Context:** Tauri file access is explicit. Opening up `**` is a
  security hole.
- **Detection:** read `catalogizer-desktop/src-tauri/tauri.conf.json`
  allowlist.
- **Fix:** minimal permissions, scoped to media directories only.

### RULE-DESK-003: AppImage needs `APPIMAGE_EXTRACT_AND_RUN=1` in containers

- **Context:** Tauri's AppImage bundler uses FUSE, which is not available
  in rootless containers. Without the env var, bundling fails.
- **Detection:** CI log "fuse: device not found" during `tauri build`.
- **Fix:** set `APPIMAGE_EXTRACT_AND_RUN=1` in the bundler container.

---

## HelixQA

### RULE-HELIX-001: Library is project-agnostic

- **Context:** HelixQA and its submodule dependencies (Challenges,
  Containers, DocProcessor, LLMOrchestrator, LLMProvider, VisionEngine)
  must work with ANY project. No hardcoded `com.catalogizer.*`,
  `com.atmosphere.*`, IP addresses, regions, or device serials.
- **Detection:**
  ```bash
  grep -rn "com\.catalogizer\|com\.atmosphere\|ru\.iptvremote" \
    HelixQA/pkg/ HelixQA/cmd/ --include="*.go" | grep -v "_test.go"
  ```
- **Fix:** register consumer-specific data via `PipelineConfig` fields
  (AndroidPackage, CompetingAppPackages, …) populated by the caller from
  `HELIX_*` env variables.

### RULE-HELIX-002: LLM vision drives every navigation decision

- **Context:** no hardcoded tap coords, sleep timers, keystroke sequences,
  or fallback hardcoded actions. Every action is
  `screenshot → vision → action`.
- **Detection:**
  ```bash
  grep -rn "input tap [0-9]" HelixQA/pkg/ --include="*.go"
  grep -rn "time\.Sleep\([0-9]" HelixQA/pkg/navigator --include="*.go"
  ```
- **Fix:** if vision provider unavailable → phase skips. Never fake.

### RULE-HELIX-003: Bank actions are executable, not prose

- **Context:** YAML banks specify `action: "adb_shell: input text admin"`
  or `action: "keypress: KEYCODE_ENTER"` — not `"Enter credentials"`.
- **Detection:** load every bank via `pkg/testbank`; any step whose
  action is parsed as `ACTION_PLACEHOLDER` is a rule-break.
- **Fix:** bank authors use the executable action vocabulary documented
  in `HelixQA/banks/README.md`.

### RULE-HELIX-004: Bank IDs must be globally unique across formats

- **Context (2026-04-21 incident):** when the same test exists as both
  `.json` and `.yaml` with the same ID, `testbank.LoadDir` raises
  `duplicate test case id`, which aborts the structured phase before the
  first step.
- **Detection:**
  ```bash
  # cross-format ID duplicates
  jq -r '.test_cases[].id' challenges/helixqa-banks/*.json | sort > /tmp/j
  yq -r '.test_cases[].id' challenges/helixqa-banks/*.yaml | sort > /tmp/y
  comm -12 /tmp/j /tmp/y
  ```
- **Fix:** remove the superseded format; rename collisions (e.g.
  `tv-cold-start` → `tv-cold-start-full` when both comprehensive and full
  executable variants exist).

### RULE-HELIX-005: Evidence validation is mandatory

- **Context:** every "PASS" must be backed by a real screenshot that
  matches expected outcome. Post-login UI dump must not contain
  "Sign In". Empty screens when backend has data = bug.
- **Detection:** session analysis tool compares pipeline-report.json
  against screenshots directory size + blank-screenshot count.
- **Fix:** `HelixQA/pkg/autonomous/screenshot.go#IsBlankScreenshot`
  filters blank captures before verification.

### RULE-HELIX-006: Orchestrator exit code reflects real result

- **Context (2026-04-21 incident):** `if cmd | tee` returns tee's exit
  code, so the orchestrator logged "✓ completed successfully" even when
  HelixQA crashed with a corrupt memory.db.
- **Detection:**
  ```bash
  grep -n "set -o pipefail\|PIPESTATUS" scripts/helixqa-orchestrator.sh
  ```
- **Fix:** `set -o pipefail` + `${PIPESTATUS[0]}` + scan
  `pipeline-report.json` for `Session failed`.

### RULE-HELIX-007: `.devconnect` lines must not carry inline comments

- **Context (2026-04-21 incident):** the orchestrator's
  `grep -v '^#' | head -1` picked up trailing `# MIBOX4 …` and HelixQA
  tried to ping the whole line.
- **Detection:**
  ```bash
  grep -nE "^\s*[^#].*#" .devconnect
  ```
- **Fix:** comments on their own lines; IP lines clean.

### RULE-HELIX-008: Vision verification is tri-state

- **Context:** astica returns natural-language ("the screen shows the
  home page") not literal "VERIFIED: yes". Strict literal-only verification
  false-fails every astica-backed step.
- **Detection:** `HelixQA/pkg/autonomous/structured_executor.go#verifyOutcome`.
- **Fix:** tri-state — exact "VERIFIED: yes" honored; non-empty prose
  treated as ambiguous and defers to action success; empty response errors.

### RULE-HELIX-009: Recorder accepts AndroidDevice OR AndroidDevices[0]

- **Context (2026-04-21 incident):** orchestrator sets only
  `AndroidDevices[]` (from `.devconnect` enumeration) and leaves
  `AndroidDevice` empty. Recorder passed empty string to
  `adb -s "" shell screenrecord` → "more than one device/emulator" →
  segment-loop safety fuse fired.
- **Detection:** read `HelixQA/pkg/autonomous/pipeline.go` recorder init.
- **Fix:** fall back to `AndroidDevices[0]` when `AndroidDevice` is empty.

### RULE-HELIX-010: Segment loop ctx bound to `context.Background()`

- **Context (2026-04-21 incident):** loopCtx chained to caller's ctx →
  phase end cancels it → every new `exec.CommandContext` returns exit 1
  instantly → 20 000-iteration spin.
- **Detection:** read `HelixQA/pkg/video/scrcpy.go` segment loop.
- **Fix:** bind to `context.Background()`; lifetime tied to explicit
  `Stop()`. Add 2-second minimum-iteration floor + 5-consecutive-failure
  ceiling as defence in depth.

---

## Challenges Framework

### RULE-CH-001: `RunAll` is synchronous / blocking

- **Context:** no other challenge can run until `RunAll` finishes.
  Progress-based liveness: 5-minute stale threshold kills stuck
  challenges.
- **Detection:** inspect `challenges/pkg/runner` liveness tracker.
- **Fix:** keep the single-writer pattern; do not add parallel runs.

### RULE-CH-002: `challenge.NewConfig()` default timeout is 5 minutes

- **Context:** zero the Timeout field explicitly to let the runner's
  overall timeout govern; otherwise a 5-minute cap will pre-empt a long
  scan.
- **Detection:** grep challenge constructors for `Timeout: 5 * time.Minute`
  left at default.
- **Fix:** `cfg.Timeout = 0` in `RegisterAll`.

### RULE-CH-003: `config.json` `write_timeout` must be 900

- **Context:** long-running RunAll (e.g. 508 challenges ~ 13 minutes) will
  hit the default 30-second write timeout and drop the client before
  the response is flushed.
- **Detection:**
  ```bash
  grep -n "write_timeout" catalog-api/config/config.json
  ```
- **Fix:** 900 seconds.

---

## Containers / Build

### RULE-CONT-001: Resource limits per container

- **Context:** host runs other mission-critical processes. Exceeding 30–40%
  CPU / RAM can freeze the machine.
- **Detection:** `podman stats --no-stream` during any build.
- **Fix:** explicit `--cpus` and `--memory` on every `podman run`:
  PostgreSQL `--cpus=1 --memory=2g`, catalog-api `--cpus=2 --memory=4g`,
  catalog-web `--cpus=1 --memory=2g`, builder `--cpus=3 --memory=8g`.
  Total budget 4 CPUs / 8 GB across all containers.

### RULE-CONT-002: `--network host` for SSL-heavy builds

- **Context:** default container networking has SSL issues with
  `dl.google.com`, `crates.io`, `registry-1.docker.io`. Go module download
  and cargo fetch fail intermittently.
- **Detection:** build logs with `x509: certificate signed by unknown
  authority`.
- **Fix:** `podman build --network host` and `podman run --network host`.

### RULE-CONT-003: `GOTOOLCHAIN=local` prevents auto-downloads

- **Context:** without this, Go downloads a newer toolchain when `go.mod`
  requests it. The download can race with container start and fail.
- **Detection:** `env | grep GOTOOLCHAIN`.
- **Fix:** export `GOTOOLCHAIN=local` in every build script.

### RULE-CONT-004: Fully-qualified image names

- **Context:** short names (`redis:7-alpine`) fail in rootless podman
  without a TTY because the "which registry?" prompt can't be answered.
- **Detection:** `podman build` failures with "short-name alias not
  found".
- **Fix:** `docker.io/library/redis:7-alpine`, etc.

### RULE-CONT-005: catalog-api container needs `--add-host` for NAS

- **Context:** `nas.local` is an mDNS name unavailable to the
  container network.
- **Detection:** catalog-api logs `lookup nas.local: no such host`.
- **Fix:** `--add-host=nas.local:192.168.0.241`.

---

## Security & Secrets

### RULE-SEC-001: No real `.env` files committed anywhere

- **Context:** `.env` contains live LLM / TMDB / IGDB keys. Commit =
  immediate rotation + incident report.
- **Detection:**
  ```bash
  git ls-files --cached -- '*.env' ':!*.env.example' \
    ':!*.env.example.*' ':!.env.example'
  # must output nothing
  ```
- **Fix:** `.gitignore` covers `.env` in project root AND every submodule.
  Rotate any leaked key; file a post-mortem.

### RULE-SEC-002: `.env.example` placeholders only

- **Context:** example files use `YOUR_API_KEY_HERE`, never a real key.
- **Detection:**
  ```bash
  grep -rE "^\w+=[A-Za-z0-9_]{20,}" -- '*.env.example'
  # must output nothing — a 20+ char value is a real key
  ```
- **Fix:** replace with `YOUR_*_HERE`.

### RULE-SEC-003: Pre-commit hook enforces "no TODO / FIXME"

- **Context:** the program has a zero-unfinished-work constraint. One
  TODO triggers the hook which refuses the commit.
- **Detection:** `pre-commit run --all-files` must pass cleanly.
- **Fix:** finish the work or rewrite the comment to explain the existing
  behavior.

---

## Git & Submodules

### RULE-GIT-001: SSH URLs only

- **Context:** `.gitmodules` uses SSH URLs. HTTPS breaks automation.
- **Detection:** `grep -n "https://" .gitmodules`.
- **Fix:** `git@github.com:…`, `git@gitlab.com:…`, GitVerse uses
  `ssh://gitverse.ru:2222/…`.

### RULE-GIT-002: No GitHub Actions workflows

- **Context:** `.github/workflows/` is forbidden. CI runs locally.
- **Detection:** `find . -path '*/.github/workflows/*' -type f`.
- **Fix:** delete; move logic into `scripts/`.

### RULE-GIT-003: Pre-commit signing required (unless user opts out)

- **Context:** commits go to 6 upstreams; consistent signing matters.
  `--no-verify` / `--no-gpg-sign` forbidden unless user explicitly
  requests.
- **Detection:** git log — unsigned commits from the Catalogizer Dev
  author after 2026-04-01.
- **Fix:** signing policy in `git config` — do not set
  `commit.gpgsign=false`.

### RULE-GIT-004: Releases + reports are gitignored artefacts

- **Context:** `releases/` and `reports/` contain build outputs — not
  version-controlled.
- **Detection:** `git ls-files releases/ reports/` must be empty.
- **Fix:** `.gitignore` already handles this; do not `git add -f`.

---

## Appendix: Quick Detection Script

```bash
#!/usr/bin/env bash
# scripts/detect-landmines.sh — run before every PR
set -euo pipefail
fail() { echo "❌ $1"; exit 1; }

# RULE-SEC-001
git ls-files --cached -- '*.env' ':!*.env.example' | grep -q . && \
  fail "RULE-SEC-001: real .env tracked"

# RULE-GO-004
grep -rn "LastInsertId()" catalog-api/ --include="*.go" | grep -v _test && \
  fail "RULE-GO-004: LastInsertId() present"

# RULE-GIT-002
find . -path '*/.github/workflows/*' -type f 2>/dev/null | grep -q . && \
  fail "RULE-GIT-002: workflows present"

# RULE-CONST-001
grep -rnE "\bsudo\b|su -c" scripts/ deployment/ --include="*.sh" \
  2>/dev/null | grep -vE "^\s*#|sudoers" && \
  fail "RULE-CONST-001: sudo reference in scripts/"

# RULE-HELIX-001
grep -rn "com\.catalogizer\|ru\.iptvremote" HelixQA/pkg/ HelixQA/cmd/ \
  --include="*.go" 2>/dev/null | grep -v _test.go && \
  fail "RULE-HELIX-001: project-specific string baked into HelixQA library"

echo "✓ landmine pre-flight clean"
```

## Index

Total rules: **47** across 11 categories.

| Category | Count | Rules |
|---|---|---|
| Project-Wide (Constitution) | 7 | CONST-001 … 007 |
| Go Backend (catalog-api) | 12 | GO-001 … 012 |
| React / Web (catalog-web) | 6 | WEB-001 … 006 |
| Android Mobile | 4 | AND-001 … 004 |
| Android TV | 6 | TV-001 … 006 |
| Desktop (Tauri) | 3 | DESK-001 … 003 |
| HelixQA | 10 | HELIX-001 … 010 |
| Challenges | 3 | CH-001 … 003 |
| Containers / Build | 5 | CONT-001 … 005 |
| Security / Secrets | 3 | SEC-001 … 003 |
| Git & Submodules | 4 | GIT-001 … 004 |
