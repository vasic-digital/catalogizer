# AGENTS.md — catalogizer-androidtv Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalogizer-androidtv` Android TV application. It defines responsibilities, package boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Package**: `com.catalogizer.androidtv`
- **Language**: Kotlin
- **Framework**: Jetpack Compose for TV (`tv-foundation`, `tv-material`), Leanback 1.0.0, Material 3
- **Architecture**: MVVM (Compose UI -> ViewModel -> Repository -> Room + Retrofit)
- **DI**: Manual `DependencyContainer` (not Hilt)
- **SDK**: compileSdk 34, minSdk 26, targetSdk 34
- **JDK**: 17 target (sourceCompatibility, targetCompatibility, jvmTarget all VERSION_17)
- **Media**: ExoPlayer (Media3) with media session, TV Provider for home screen channels

## Package Ownership Boundaries

| Package | Scope | Boundary |
|---|---|---|
| `data/` | Repository implementations, Room DAOs, Retrofit API, DTOs | Data layer. Do not import from `ui/`. |
| `data/local/` | Room database, DAOs, entity definitions | Local persistence. Migration required for schema changes. |
| `data/remote/` | Retrofit API interface, OkHttp configuration | Network layer. Single source of truth for HTTP calls. |
| `data/repository/` | Repository implementations bridging remote + local | Owns data flow. ViewModels access data only through repositories. |
| `data/tv/` | TV Provider channel/program CRUD, Watch Next, background sync | Home screen integration. Interacts with Android TV system APIs. |
| `ui/` | TV-optimized Compose screens, ViewModels, navigation | Presentation layer. Uses `tv-foundation`/`tv-material` composables. |
| `ui/screens/` | Screen-level composables optimized for 10-foot UI | One composable per screen. Focus management is critical. |
| `ui/components/` | Reusable TV composables (cards, rows, carousels) | Shared across screens. Must handle D-pad focus correctly. |
| `ui/viewmodels/` | ViewModels exposing `StateFlow<UiState>` | Own screen state. |
| `ui/navigation/` | Navigation graph and route definitions | Central routing including deep link handling. |
| `utils/` | Utility functions | Pure helpers. |

## Dependency Graph

```
CatalogizerTVApplication
 └── DependencyContainer
      ├── data/remote/        (Retrofit API, OkHttp)
      ├── data/local/         (Room database, DAOs)
      ├── data/repository/    (Repository implementations)
      ├── data/tv/            (TV channels, Watch Next, sync)
      │    ├── TvChannelRepository
      │    ├── ChannelProgramMapper
      │    ├── WatchNextManager
      │    └── TvChannelSyncWorker
      └── ui/
           ├── viewmodels/
           ├── screens/
           ├── components/
           ├── navigation/
           └── ChannelDeepLinkActivity
```

## Agent Coordination Rules

### 1. TV-specific UI requirements

- **All interactive elements must be D-pad focusable** with visible focus indicators. Use `tv-foundation` focus APIs.
- **No touch events**: The TV UI is navigated entirely via D-pad (directional, center/select, back). Never add click-only handlers.
- **10-foot UI**: Text must be large enough to read from a couch distance. Use TV Material typography scales.
- **ADB input sequence**: For text fields, press `dpad_center` BEFORE `type`, use `KEYCODE_TAB` between fields. This is critical for HelixQA testing.

### 2. Home screen channels (`data/tv/`)

The TV app integrates with Android TV's home screen via `androidx.tvprovider`:

| File | Responsibility |
|------|---------------|
| `TvChannelRepository.kt` | Channel and program CRUD against the TV Provider content provider |
| `ChannelProgramMapper.kt` | Converts `MediaItem` entities to `PreviewProgram` objects |
| `WatchNextManager.kt` | Manages the system Watch Next row (partially watched + next episode) |
| `TvChannelSyncWorker.kt` | WorkManager periodic sync (every 6 hours) |

Coordination rules for channel code:
- `TYPE_APP` does not exist in `tvprovider:1.0.0` — "software" media type maps to `TYPE_CLIP`.
- Watch Next entries must be cleaned up on logout (channels, Watch Next, sync worker, DataStore keys).
- Deep links use `catalogizer://media/{id}?type={type}`, handled by `ChannelDeepLinkActivity`.

### 3. Adding a new screen

1. Create the screen composable in `ui/screens/` using TV composables (`TvLazyRow`, `TvCard`, etc.).
2. Ensure all focusable elements have visible focus indicators.
3. Create the ViewModel in `ui/viewmodels/`.
4. Add the route to the navigation graph.
5. Wire the ViewModel in `DependencyContainer`.
6. Add unit tests and verify D-pad navigation manually or via HelixQA.

### 4. Media playback

- Uses Media3 ExoPlayer with `media3-session` for TV media session integration.
- Play/pause from remote control must work. Background audio control via media session.
- Playback progress is tracked for Watch Next integration.

### 5. Network and server configuration

- Cleartext traffic allowed only to local networks (10.x, 192.168.x, 127.x) via `network_security_config.xml`.
- Server URL persisted via DataStore. For physical TV devices, enter the server's LAN IP.
- ADB reverse proxy: `adb reverse tcp:8080 tcp:8080` required for each device before testing.

### 6. Testing standards

- **Unit tests**: JUnit 4 + MockK. Coroutines via `kotlinx-coroutines-test`. `unitTests.isReturnDefaultValues = true` is set.
- **Instrumentation tests**: Espresso + Compose UI testing on TV device or emulator.
- **Coverage**: JaCoCo via `./gradlew jacocoTestReport`.
- **HelixQA**: Autonomous LLM-driven QA targets this app on physical devices (Mi Box 4). All sessions must include video recording.

## File Ownership

| File | Primary Concern | Cross-Module Impact |
|------|----------------|---------------------|
| `CatalogizerTVApplication.kt` | Application class, container init | All services and repositories |
| `DependencyContainer.kt` | Manual DI wiring | All ViewModels and repositories |
| `data/tv/TvChannelRepository.kt` | Channel/program CRUD | Home screen integration |
| `data/tv/WatchNextManager.kt` | Watch Next row | Playback progress tracking |
| `data/tv/TvChannelSyncWorker.kt` | Background sync | WorkManager scheduling |
| `ui/ChannelDeepLinkActivity.kt` | Deep link intent router | Home screen channel taps |
| `ui/screens/settings/ChannelSettingsSection.kt` | Per-category tap behavior | Deep link launch mode |

## Build & Validation Commands

```bash
# Build
./gradlew assembleDebug                                     # debug APK
./gradlew assembleRelease                                   # release APK (requires signing)

# Test
./gradlew test                                              # all unit tests
./gradlew testDebugUnitTest                                 # debug unit tests only
./gradlew :app:testDebugUnitTest --tests ClassName          # single class
./gradlew connectedAndroidTest                              # instrumentation tests

# Quality
./gradlew lint                                              # Android lint
./gradlew jacocoTestReport                                  # coverage report

# Install to TV device
./gradlew installDebug
adb reverse tcp:8080 tcp:8080                               # proxy for localhost API access
```

## JDK Constraints

- **JDK 17 target** (unlike the phone app which targets JDK 21). All `sourceCompatibility`, `targetCompatibility`, and `jvmTarget` are `VERSION_17` / `"17"`.
- **`--add-opens` JVM args** required in `gradle.properties` (Gradle JVM + Kotlin daemon) AND in the `kapt` block of `build.gradle.kts` for Room annotation processing with JDK 21.
- **Kotlin daemon**: Separate `kotlin.daemon.jvmargs` with `--add-opens` flags in `gradle.properties`.
- **JDK image transform disabled**: `android.useNewJdkImageTransform=false`.
- **Gradle JVM memory**: `-Xmx4096m -XX:MaxMetaspaceSize=1024m`. Kotlin daemon: `-Xmx1024m`.

## Commit Conventions

Conventional Commits:
- `feat(androidtv): add category channel integration`
- `fix(androidtv): correct focus handling on media row`
- `test(androidtv): add Watch Next manager coverage`

Every commit must:
- Pass `./gradlew lint`.
- Pass `./gradlew testDebugUnitTest`.
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **Container builds**: Use Podman. Requires Android SDK 34 in the builder image.
- **Resource limits**: Gradle JVM limited to `-Xmx4096m -XX:MaxMetaspaceSize=1024m`. Kotlin daemon to `-Xmx1024m`.
- **API keys**: Never commit `local.properties` or `.env` with real secrets.
- **ADB reverse proxy**: Must configure `adb reverse tcp:8080 tcp:8080` per device before testing.
- **Signing**: Release builds read signing config from `../docker/signing/signing.properties`.
- **`.devignore`**: Check device model against `.devignore` before any ADB operation. Abort if matched.

## ⚠️ MANDATORY: NO SUDO OR ROOT EXECUTION

**ALL operations MUST run at local user level ONLY.**

This is a PERMANENT and NON-NEGOTIABLE security constraint:

- **NEVER** use `sudo` in ANY command
- **NEVER** use `su` in ANY command
- **NEVER** execute operations as `root` user
- **NEVER** elevate privileges for file operations
- **ALL** infrastructure commands MUST use user-level container runtimes (rootless podman/docker)
- **ALL** file operations MUST be within user-accessible directories
- **ALL** service management MUST be done via user systemd or local process management
- **ALL** builds, tests, and deployments MUST run as the current user

### Container-Based Solutions
When a build or runtime environment requires system-level dependencies, use containers instead of elevation:

- **Use the `Containers` submodule** (`https://github.com/vasic-digital/Containers`) for containerized build and runtime environments
- **Add the `Containers` submodule as a Git dependency** and configure it for local use within the project
- **Build and run inside containers** to avoid any need for privilege escalation
- **Rootless Podman/Docker** is the preferred container runtime

### Why This Matters
- **Security**: Prevents accidental system-wide damage
- **Reproducibility**: User-level operations are portable across systems
- **Safety**: Limits blast radius of any issues
- **Best Practice**: Modern container workflows are rootless by design

### When You See SUDO
If any script or command suggests using `sudo` or `su`:
1. STOP immediately
2. Find a user-level alternative
3. Use rootless container runtimes
4. Use the `Containers` submodule for containerized builds
5. Modify commands to work within user permissions

**VIOLATION OF THIS CONSTRAINT IS STRICTLY PROHIBITED.**

## MANDATORY: Zero Unfinished Work

No TODOs, FIXMEs, empty implementations, silent error swallows, fake data, or `unwrap()`-equivalent patterns may be committed. The local anti-bluff scan and the dedicated commit/push validation script report them; no CI/CD pipeline is used (§11.4.156, §11.4.234). When an issue is found, fix all instances — not just the reported one.
