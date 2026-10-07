# Build System

Catalogizer uses a modular, containerized build system with automatic versioning and change detection. The generic build framework lives in the `Build/` submodule and can be reused across projects.

## Architecture

```
Build/                          # Generic framework (reusable submodule)
├── lib/
│   ├── common.sh               # Logging, runtime detection, git helpers, artifacts
│   ├── version.sh              # Semantic versioning via versions.json
│   ├── hash.sh                 # Source hash computation, change detection
│   └── orchestrator.sh         # CLI parsing, build loop, component dispatching
└── README.md

scripts/                        # Project-specific build configuration
├── lib/
│   ├── project-config.sh       # Component list, patterns, platform targets
│   ├── build-catalog-api.sh    # Go cross-compilation
│   ├── build-catalog-web.sh    # React/Vite production build
│   ├── build-api-client.sh     # TypeScript library build
│   ├── build-desktop.sh        # Tauri desktop build
│   ├── build-installer.sh      # Tauri installer wizard build
│   ├── build-android.sh        # Android APK build
│   ├── build-androidtv.sh      # Android TV APK build
│   └── build-component.sh      # Container entry point dispatcher
└── release-build.sh            # Master orchestrator (entry point)

versions.json                   # Version + build state (git-tracked)
```

## Quick Start

```bash
# Build all components with changes
./scripts/release-build.sh

# Force rebuild everything
./scripts/release-build.sh --force

# Build a single component
./scripts/release-build.sh --component catalog-api

# Preview what would build (no actual build)
./scripts/release-build.sh --dry-run

# Bump version before building
./scripts/release-build.sh --bump patch

# Skip tests
./scripts/release-build.sh --skip-tests

# Show change detection status
./scripts/release-build.sh --status

# Show current version
./scripts/release-build.sh --version
```

## CLI Reference

| Flag | Description |
|------|-------------|
| `--component NAME` | Build a single component |
| `--force` | Rebuild all components (ignore change detection) |
| `--dry-run` | Show what would be built without building |
| `--bump TYPE` | Increment version: `major`, `minor`, or `patch` |
| `--skip-tests` | Skip test phase during build |
| `--container` | Force containerized build |
| `--local` | Force local build (no container) |
| `--status` | Show component change detection status |
| `--version` | Show current version string |
| `--help` | Show help message |

## Components

| Component | Type | Artifacts |
|-----------|------|-----------|
| catalog-api | Go | Binaries for linux/windows/macos amd64+arm64 |
| catalog-web | React/TS | Static dist/ bundle |
| catalogizer-api-client | TypeScript | Compiled dist + package.json |
| catalogizer-desktop | Tauri | Linux AppImage, .deb |
| installer-wizard | Tauri | Linux AppImage, .deb |
| catalogizer-android | Kotlin | Release APK |
| catalogizer-androidtv | Kotlin | Release APK |

## Change Detection

Each component has defined source file patterns. Before building, the system computes a SHA256 hash of all matching source files and compares it with the stored hash in `versions.json`. If unchanged (and `--force` is not set), the component is skipped.

### Source Patterns

| Component | Patterns |
|-----------|----------|
| catalog-api | `*.go`, `go.mod`, `go.sum` |
| catalog-web | `*.ts`, `*.tsx`, `*.js`, `*.json`, `*.html`, `*.css` |
| catalogizer-api-client | `*.ts`, `*.js`, `*.json` |
| catalogizer-desktop | `*.ts`, `*.tsx`, `*.js`, `*.json`, `*.html`, `*.css`, `*.rs`, `*.toml` |
| installer-wizard | `*.ts`, `*.tsx`, `*.js`, `*.json`, `*.html`, `*.css`, `*.rs`, `*.toml` |
| catalogizer-android | `*.kt`, `*.java`, `*.xml`, `*.gradle.kts`, `*.properties` |
| catalogizer-androidtv | `*.kt`, `*.java`, `*.xml`, `*.gradle.kts`, `*.properties` |

### Excluded Directories

`node_modules/`, `dist/`, `build/`, `target/`, `.git/`, `coverage/`, `.gradle/`, `.idea/`, `.vscode/`, `__pycache__/`

### Hash Algorithm

1. Find all matching source files (excluding ignored directories)
2. Sort file list for determinism
3. Compute SHA256 of each file
4. Combine all hashes and compute final SHA256

## Versioning

### versions.json

```json
{
  "schema_version": 1,
  "global": {
    "major": 1, "minor": 0, "patch": 0,
    "build_number": 3
  },
  "components": {
    "catalog-api": {
      "last_build_number": 3,
      "last_build_date": "2026-02-17T14:30:00Z",
      "last_source_hash": "a1b2c3d4...",
      "last_git_commit": "c5a4a4b7"
    }
  }
}
```

### Version String Format

`v{major}.{minor}.{patch}-build.{build_number}` (e.g., `v1.0.0-build.3`)

### Version Injection

**Go (catalog-api)**: Via ldflags:
```bash
go build -ldflags "-X main.Version=1.0.0 -X main.BuildNumber=3 -X main.BuildDate=..."
```

The `/health` endpoint returns version info:
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "build_number": "3",
  "build_date": "2026-02-17T14:30:00Z"
}
```

## Release Directory Structure

```
releases/
├── catalog-api/
│   ├── linux-amd64/
│   │   └── v1.0.0-build.3/
│   │       ├── catalog-api
│   │       ├── SHA256SUM
│   │       └── BUILD_INFO.json
│   ├── windows-amd64/
│   │   └── v1.0.0-build.3/
│   │       ├── catalog-api.exe
│   │       ├── SHA256SUM
│   │       └── BUILD_INFO.json
│   ├── darwin-amd64/
│   └── darwin-arm64/
├── catalog-web/
│   └── web/
│       └── v1.0.0-build.3/
│           ├── dist/
│           ├── SHA256SUM
│           └── BUILD_INFO.json
├── catalogizer-android/
│   └── android/
│       └── v1.0.0-build.3/
│           ├── catalogizer-android.apk
│           ├── SHA256SUM
│           └── BUILD_INFO.json
└── ...
```

### BUILD_INFO.json

```json
{
  "component": "catalog-api",
  "platform": "linux-amd64",
  "version": "1.0.0",
  "build_number": 3,
  "version_string": "v1.0.0-build.3",
  "git_commit": "c5a4a4b7",
  "git_commit_full": "c5a4a4b7abc...",
  "git_branch": "main",
  "build_date": "2026-02-17T14:30:00Z",
  "source_hash": "a1b2c3d4..."
}
```

## Containerized Builds

For reproducible builds inside the builder container:

```bash
# Via container-build.sh (the entry point: it generates the per-run credentials and random ports
# docker-compose.build.yml requires, passes them with --env-file and removes them afterwards)
./scripts/container-build.sh 1.0.0

# Validate the compose file with a fresh per-run env, without building
./scripts/container-build.sh --validate-only

# Running the compose file directly needs that env file (without it podman-compose refuses to render):
bash scripts/test-infra/gen_env.sh --build-id mybuild
BUILD_VERSION=1.0.0 BUILD_NUMBER=3 BUILD_COMPONENTS="catalog-api catalog-web" \
  podman-compose --env-file .audit/test-infra/catalogizer-test-mybuild/env \
  -f docker-compose.build.yml up --build --abort-on-container-exit
```

### Container Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `BUILD_VERSION` | Version string | `1.0.0` |
| `BUILD_NUMBER` | Build number | `0` |
| `BUILD_COMPONENTS` | Space-separated components or `all` | `all` |
| `FORCE_BUILD` | Force rebuild | `false` |
| `SKIP_TESTS` | Skip tests | `false` |

## Build Framework (Submodule)

The generic build framework in `Build/` is designed for reuse across projects. See [Build/README.md](../Build/README.md) for integration guide.

### Adapting for Another Project

1. Add `Build/` as a submodule
2. Create `scripts/lib/project-config.sh` defining your components and patterns
3. Create per-component builder scripts
4. Create `scripts/release-build.sh` that sources the framework and defines `build_single_component()`
5. Create `versions.json` (auto-created on first run)

## Testing

```bash
# Run build system tests
bash tests/test_build_system.sh
```

Tests cover: version management, hash computation, change detection, directory structure, BUILD_INFO.json generation, checksum verification, and CLI parsing.
