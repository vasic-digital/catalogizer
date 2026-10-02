# AGENTS.md — catalogizer-android Multi-Agent Coordination Guide

This document provides guidance for AI agents (Claude Code, Copilot, Cursor, etc.) working on the `catalogizer-android` native Android application. It defines responsibilities, package boundaries, and coordination protocols to prevent conflicts when multiple agents operate concurrently on this module.

## Module Identity

- **Package**: `com.catalogizer.android`
- **Language**: Kotlin
- **Framework**: Jetpack Compose (BOM 2024.12.01), Material 3, Navigation Compose
- **Architecture**: MVVM (Compose UI -> ViewModel -> Repository -> Room + Retrofit)
- **DI**: Manual `DependencyContainer` (not Hilt)
- **SDK**: compileSdk 35, minSdk 26, targetSdk 34
- **JDK**: 21 (sourceCompatibility, targetCompatibility, jvmTarget all VERSION_21)

## Package Ownership Boundaries

| Package | Scope | Boundary |
|---|---|---|
| `data/` | Repository implementations, Room DAOs, Retrofit API definitions, DTOs | Data layer only. Do not import from `ui/`. |
| `data/local/` | Room database, DAOs, entity definitions | Local persistence. Changes here require migration consideration. |
| `data/remote/` | Retrofit API interface, OkHttp configuration | Network layer. Do not duplicate HTTP logic outside this package. |
| `data/repository/` | Repository implementations bridging remote + local | Owns data flow. ViewModels access data exclusively through repositories. |
| `ui/` | Compose screens, ViewModels, navigation graph | Presentation layer. Do not access Room or Retrofit directly — go through repositories. |
| `ui/screens/` | Screen-level composables | One composable per screen. Compose from `ui/components/`. |
| `ui/components/` | Reusable UI composables | Shared across screens. Do not import from `ui/screens/`. |
| `ui/viewmodels/` | ViewModels exposing `StateFlow<UiState>` | Own screen state. Do not hold references to Compose state. |
| `ui/navigation/` | Navigation graph and route definitions | Central routing. All screen routes defined here. |
| `utils/` | Utility functions | Pure helpers with no Android framework dependencies where possible. |

## Dependency Graph

```
CatalogizerApplication
 └── DependencyContainer
      ├── data/remote/       (Retrofit API, OkHttp client)
      ├── data/local/        (Room database, DAOs)
      ├── data/repository/   (Repository implementations)
      └── ui/
           ├── viewmodels/   (ViewModels consuming repositories)
           ├── screens/      (Compose screens consuming ViewModels)
           ├── components/   (Shared composables)
           └── navigation/   (Nav graph)
```

No package inside `data/` may import from `ui/`. ViewModels do not import from `data/local/` or `data/remote/` directly — they go through `data/repository/`.

## Agent Coordination Rules

### 1. Dependency injection

The app uses a manual `DependencyContainer` initialized in `CatalogizerApplication`. When adding a new repository or service:

1. Add the constructor call in `DependencyContainer`.
2. Expose it as a property for ViewModel consumption.
3. Do not create singletons outside the container.

### 2. Adding a new screen

1. Create the screen composable in `ui/screens/`.
2. Create the ViewModel in `ui/viewmodels/` exposing `StateFlow<UiState>`.
3. Add the route to the navigation graph in `ui/navigation/`.
4. Wire the ViewModel in `DependencyContainer` if it needs repository dependencies.
5. Add unit tests for the ViewModel and UI tests for the screen.

### 3. Room database changes

- Add a new migration in the Room database builder. Never modify an existing migration after it has shipped.
- Update the database version number.
- Add both the migration and a destructive fallback test.
- DAOs must return `Flow<T>` or `suspend` functions — never blocking queries on the main thread.

### 4. Network layer

- All REST calls go through the Retrofit interface in `data/remote/`.
- OkHttp interceptors handle auth token injection.
- Cleartext traffic is only allowed to local networks (10.x, 192.168.x, 127.x) via `network_security_config.xml`.
- Server URL is persisted via DataStore. Default empty base URL works with `adb reverse tcp:8080 tcp:8080`.

### 5. Coroutines and async

- Use `suspend` functions in repositories and data sources.
- ViewModels collect from `Flow` and expose `StateFlow<UiState>`.
- Use `viewModelScope` for ViewModel coroutines.
- Use `kotlinx-coroutines-test` and `UnconfinedTestDispatcher` in tests.

### 6. Testing standards

- **Unit tests**: JUnit 4 + MockK. Coroutines via `kotlinx-coroutines-test`. Test files in `app/src/test/`.
- **Instrumentation tests**: Espresso + Compose UI testing (`ui-test-junit4`). Test files in `app/src/androidTest/`.
- **Coverage**: JaCoCo via `./gradlew jacocoTestReport`.
- **ViewModel tests**: Test state transitions by collecting from `StateFlow` with `Turbine` or manual collection.

### 7. Error handling

- Use sealed `Result` classes for repository operation outcomes.
- Never swallow exceptions silently — log and propagate or convert to a user-visible error state.
- Network errors must be classified (timeout, auth expired, server error) for proper UI feedback.

## File Ownership

| File | Primary Concern | Cross-Module Impact |
|------|----------------|---------------------|
| `CatalogizerApplication.kt` | Application class, container init | All services and repositories |
| `DependencyContainer.kt` | Manual DI wiring | All ViewModels and repositories |
| `data/remote/CatalogizerApi.kt` | Retrofit interface | All repositories that fetch remote data |
| `data/local/AppDatabase.kt` | Room database definition | All DAOs and migrations |
| `ui/navigation/` | Navigation graph | All screens and deep links |
| `network_security_config.xml` | Cleartext traffic policy | All network calls |

## Build & Validation Commands

```bash
# Build
./gradlew assembleDebug                                     # debug APK
./gradlew assembleRelease                                   # release APK (requires signing)

# Test
./gradlew test                                              # all unit tests
./gradlew testDebugUnitTest                                 # debug unit tests only
./gradlew :app:testDebugUnitTest --tests ClassName          # single class
./gradlew :app:testDebugUnitTest --tests ClassName.method   # single method
./gradlew connectedAndroidTest                              # instrumentation tests

# Quality
./gradlew lint                                              # Android lint
./gradlew jacocoTestReport                                  # coverage report
```

## JDK Constraints

- **JDK 21** is the compile target. All `sourceCompatibility`, `targetCompatibility`, and `jvmTarget` are set to `VERSION_21` / `"21"`.
- **`--add-opens` JVM args** are required in `gradle.properties` for kapt (Room annotation processor) compatibility with JDK 21. These open `jdk.compiler` internal modules.
- **JDK image transform disabled**: `android.useNewJdkImageTransform=false` to avoid jlink issues with AGP 8.1.0 + JDK 21.
- **Gradle JVM memory**: Limited to `-Xmx4096m -XX:MaxMetaspaceSize=1024m`.

## Commit Conventions

Conventional Commits:
- `feat(android): add media detail screen`
- `fix(android): handle network timeout in repository`
- `test(android): add ViewModel state transition coverage`

Every commit must:
- Pass `./gradlew lint`.
- Pass `./gradlew testDebugUnitTest`.
- End with the Co-Authored-By trailer when authored with an AI assistant.

## Constraints

- **Container builds**: Use Podman. Requires the Android SDK (compileSdk 35) in the builder image.
- **Resource limits**: Gradle JVM limited to `-Xmx4096m -XX:MaxMetaspaceSize=1024m`.
- **API keys**: Never commit `local.properties` or `.env` with real secrets.
- **Offline-first**: Room database provides local cache. Network failures must degrade gracefully.
- **Signing**: Release builds read signing config from `../docker/signing/signing.properties`.

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
