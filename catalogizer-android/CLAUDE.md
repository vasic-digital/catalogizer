# CLAUDE.md — catalogizer-android

## Overview

Native Android application for Catalogizer media management. Built with Kotlin and Jetpack Compose following MVVM architecture. Provides media browsing, search, playback, and collection management on Android phones and tablets.

- **Package**: `com.catalogizer.android`
- **SDK**: compileSdk 35, minSdk 26, targetSdk 34
- **Version**: 2.4.0 (versionCode 6)

## Commands

```bash
./gradlew assembleDebug          # build debug APK
./gradlew assembleRelease        # build release APK (requires signing config)
./gradlew test                   # run unit tests
./gradlew testDebugUnitTest      # run debug unit tests only
./gradlew connectedAndroidTest   # run instrumentation tests on device/emulator
./gradlew installDebug           # install debug APK on connected device
./gradlew lint                   # run Android lint
./gradlew jacocoTestReport       # generate test coverage report (HTML + XML)
```

## Architecture

**MVVM: Compose UI -> ViewModel (StateFlow) -> Repository -> Room + Retrofit**

### Source Structure

```
app/src/main/java/com/catalogizer/android/
  CatalogizerApplication.kt     # Application class
  DependencyContainer.kt        # Manual dependency injection container
  data/                          # Repository implementations, Room DAOs, Retrofit API
  ui/                            # Compose screens, ViewModels, navigation
```

### Key Libraries

| Library | Purpose |
|---|---|
| Jetpack Compose (BOM 2024.12.01) | Declarative UI |
| Material 3 | Design system |
| Navigation Compose | Screen navigation |
| Room 2.6.1 | Local database (offline cache) |
| Retrofit 2.9.0 + OkHttp 4.12 | REST API client |
| Kotlinx Serialization | JSON serialization |
| Coil | Image loading |
| ExoPlayer (Media3) | Media playback |
| DataStore Preferences | Key-value storage |
| Paging 3 | Paginated data loading |
| WorkManager | Background sync tasks |

### Dependency Injection

Uses a manual `DependencyContainer` (not Hilt) for service wiring. The container is initialized in `CatalogizerApplication` and provides repositories, API clients, and ViewModels.

### Server Configuration

Debug builds use empty API base URL. On first launch:
1. Tries `http://localhost:8080` (works with `adb reverse tcp:8080 tcp:8080`)
2. Falls back to login screen with server URL input and Discover button
3. Server URL is persisted via DataStore across restarts

For emulators: `http://10.0.2.2:8080`. For physical devices: server's LAN IP.

## Testing

- **Unit tests**: JUnit 4 + MockK/Mockito. Coroutines test via `kotlinx-coroutines-test`. Robolectric for Android framework mocking.
- **Instrumentation tests**: Espresso + Compose UI testing (`ui-test-junit4`). Run on device or emulator.
- **Test files**: `app/src/test/` (unit), `app/src/androidTest/` (instrumentation).
- **Coverage**: JaCoCo configured via `jacocoTestReport` task.

## JDK and Build Constraints

- **JDK 21 is the system default**. Compile options target Java 21 (`sourceCompatibility`/`targetCompatibility = VERSION_21`, `jvmTarget = "21"`).
- **`--add-opens` JVM args** are required in `gradle.properties` for kapt (Room compiler) compatibility with JDK 21. These open `jdk.compiler` internal modules to ALL-UNNAMED.
- **JDK image transform disabled**: Multiple properties set `android.useNewJdkImageTransform=false` to avoid jlink issues with AGP 8.1.0 + JDK 21.
- **Signing**: Release builds read signing config from `../docker/signing/signing.properties`.
- **ProGuard**: Release builds enable minification with `proguard-rules.pro`.

## Conventions

- **Kotlin**: Coroutines for async, StateFlow for reactive state, Result sealed classes for error handling.
- **Compose**: Screens as composable functions, ViewModels expose `StateFlow<UiState>`.
- **Networking**: Cleartext traffic only allowed to local networks (10.x, 192.168.x, 127.x) via `network_security_config.xml`.
- **Offline-first**: Room database provides local cache. Sync via WorkManager.

## Constraints

- **Container builds**: Use Podman. Requires the Android SDK (compileSdk 35) in the builder image.
- **Resource limits**: Gradle JVM limited to `-Xmx4096m -XX:MaxMetaspaceSize=1024m`.
- **API keys**: Never commit `local.properties` or `.env` with real secrets.


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


