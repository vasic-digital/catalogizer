# Development Setup Guide

Complete guide for setting up the Catalogizer development environment across all platforms.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Database Setup](#database-setup)
- [Backend Setup (catalog-api)](#backend-setup-catalog-api)
- [Frontend Setup (catalog-web)](#frontend-setup-catalog-web)
- [Desktop Applications](#desktop-applications)
- [Android Applications](#android-applications)
- [Environment Variables](#environment-variables)
- [IDE Configuration](#ide-configuration)
- [Debugging](#debugging)
- [Hot Reload](#hot-reload)
- [Container Development](#container-development)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

### Required Tools

**Go 1.21+** (Backend)
```bash
# Linux
wget https://go.dev/dl/go1.21.6.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.6.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin

# Verify installation
go version
```

**Node.js 18+ and npm** (Frontend)
```bash
# Using nvm (recommended)
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.5/install.sh | bash
nvm install 18
nvm use 18

# Verify installation
node --version && npm --version
```

**Rust and Cargo** (Tauri Desktop Apps)
```bash
# Install rustup
curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh

# Verify installation
rustc --version && cargo --version
```

**Android Studio** (Android Apps)
- Download from: https://developer.android.com/studio
- Install Android SDK (API 28+)
- Install Kotlin plugin
- Set `ANDROID_HOME` environment variable:
  ```bash
  export ANDROID_HOME=$HOME/Android/Sdk
  export PATH=$PATH:$ANDROID_HOME/tools:$ANDROID_HOME/platform-tools
  ```

**Container Runtime** (Optional but recommended)
```bash
# Podman (preferred)
sudo dnf install podman podman-compose  # Fedora/RHEL
sudo apt install podman podman-compose  # Ubuntu/Debian

# Or Docker
sudo dnf install docker docker-compose
sudo systemctl enable --now docker
sudo usermod -aG docker $USER  # Logout and login
```

### Optional Tools

- **SQLite3**: For database inspection
- **Redis**: For rate limiting and caching (development)
- **Git**: Version control
- **jq**: JSON processing in scripts

---

## Database Setup

### SQLite (Development - Default)

**No setup required.** The `catalog-api` backend automatically creates `catalogizer.db` on first run.

```bash
cd catalog-api
go run main.go  # Creates catalogizer.db automatically
```

**Inspect database:**
```bash
sqlite3 catalogizer.db
.tables
.schema files
SELECT COUNT(*) FROM files;
.quit
```

**Reset database:**
```bash
rm catalogizer.db
go run main.go  # Recreates with migrations
```

### PostgreSQL (Production)

**Install PostgreSQL:**
```bash
# Linux
sudo dnf install postgresql postgresql-server  # Fedora/RHEL
sudo apt install postgresql postgresql-contrib # Ubuntu/Debian

# Initialize and start
sudo postgresql-setup --initdb
sudo systemctl enable --now postgresql
```

**Create database and user:**
```bash
sudo -u postgres psql

CREATE DATABASE catalogizer;
CREATE USER catalogizer WITH PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer;
\q
```

**Configure environment:**
```bash
export DB_TYPE=postgres
export DB_HOST=localhost
export DB_PORT=5432
export DB_NAME=catalogizer
export DB_USER=catalogizer
export DB_PASSWORD=your_secure_password
```

---

## Backend Setup (catalog-api)

### 1. Clone and Install Dependencies

```bash
cd catalog-api

# Download Go dependencies
go mod download

# Verify dependencies
go mod verify
```

### 2. Configure Environment

Create `.env` file in `catalog-api/`:

```env
# Server Configuration
PORT=8080
GIN_MODE=debug
CORS_ORIGINS=http://localhost:5173,http://localhost:3000

# Database (SQLite default)
DB_TYPE=sqlite
DB_PATH=catalogizer.db

# PostgreSQL (uncomment if using)
# DB_TYPE=postgres
# DB_HOST=localhost
# DB_PORT=5432
# DB_NAME=catalogizer
# DB_USER=catalogizer
# DB_PASSWORD=your_password

# Authentication
JWT_SECRET=your-development-secret-key-change-in-production
JWT_EXPIRATION=24h
ADMIN_USERNAME=admin
ADMIN_PASSWORD=<generate-your-own, e.g. openssl rand -base64 24>
SESSION_TIMEOUT=30m

# External API Keys (optional)
TMDB_API_KEY=your_tmdb_api_key
OMDB_API_KEY=your_omdb_api_key
IMDB_API_KEY=your_imdb_api_key

# Redis (optional - for rate limiting)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0

# File Watcher Settings
WATCHER_DEBOUNCE_MS=500
WATCHER_BUFFER_SIZE=1000

# Media Analysis
MEDIA_WORKERS=4
ANALYSIS_TIMEOUT=30s

# Logging
LOG_LEVEL=debug
LOG_FORMAT=json
```

### 3. Run Database Migrations

Migrations run automatically on startup, but you can verify:

```bash
go run main.go
# Check logs for: "Running database migrations..."
```

### 4. Run Development Server

```bash
# Standard run
go run main.go

# With live reload (using air)
go install github.com/cosmtrek/air@latest
air  # Watches for changes and reloads
```

**Verify backend is running:**
```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

### 5. Run Tests

```bash
# All tests
go test ./...

# Verbose with coverage
go test -v -cover ./...

# Race condition detection
go test -race ./...

# Specific package
go test -v ./handlers/

# Specific test
go test -v -run TestCreateUser ./handlers/
```

---

## Frontend Setup (catalog-web)

### 1. Install Dependencies

```bash
cd catalog-web
npm install
```

### 2. Configure Environment

Create `.env` file in `catalog-web/`:

```env
# API Configuration
VITE_API_BASE_URL=http://localhost:8080/api/v1
VITE_WS_URL=ws://localhost:8080/ws

# Feature Flags
VITE_ENABLE_ANALYTICS=false
VITE_ENABLE_DEBUG=true

# External Services (optional)
VITE_SENTRY_DSN=
```

### 3. Run Development Server

```bash
npm run dev
# Runs at http://localhost:5173
```

**Verify frontend:**
Open http://localhost:5173 in browser. You should see the login page.

### 4. Run Tests

```bash
# Unit tests (Vitest)
npm run test          # Watch mode
npm run test -- --run # Single run

# E2E tests (Playwright)
npm run test:e2e      # Headless
npm run test:e2e:ui   # With UI

# Type checking
npm run type-check

# Linting
npm run lint
```

### 5. Build for Production

```bash
npm run build
# Output in dist/

# Preview production build
npm run preview
```

---

## Desktop Applications

### catalogizer-desktop

**Prerequisites:**
- Tauri CLI: `cargo install tauri-cli`
- System dependencies (Linux):
  ```bash
  sudo dnf install webkit2gtk4.0-devel openssl-devel  # Fedora
  sudo apt install libwebkit2gtk-4.0-dev libssl-dev   # Ubuntu
  ```

**Setup:**
```bash
cd catalogizer-desktop
npm install

# Development
npm run tauri:dev

# Build
npm run tauri:build
# Output in src-tauri/target/release/bundle/
```

### installer-wizard

Same setup as catalogizer-desktop:

```bash
cd installer-wizard
npm install
npm run tauri:dev
```

---

## Android Applications

### catalogizer-android

**Setup:**
```bash
cd catalogizer-android

# Build debug APK
./gradlew assembleDebug

# Install on connected device
./gradlew installDebug

# Run tests
./gradlew test
./gradlew connectedAndroidTest  # Requires device/emulator
```

**Open in Android Studio:**
1. File → Open → Select `catalogizer-android/`
2. Wait for Gradle sync
3. Click "Run" or press Shift+F10

### catalogizer-androidtv

Same setup as catalogizer-android:

```bash
cd catalogizer-androidtv
./gradlew assembleDebug
```

---

## Environment Variables

### catalog-api Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | 8080 | Server port |
| `GIN_MODE` | debug | Gin mode: debug, release |
| `DB_TYPE` | sqlite | Database type: sqlite, postgres |
| `DB_PATH` | catalogizer.db | SQLite database file path |
| `DB_HOST` | localhost | PostgreSQL host |
| `DB_PORT` | 5432 | PostgreSQL port |
| `DB_NAME` | catalogizer | Database name |
| `DB_USER` | catalogizer | Database user |
| `DB_PASSWORD` | | Database password |
| `JWT_SECRET` | (required) | JWT signing secret |
| `JWT_EXPIRATION` | 24h | JWT token expiration |
| `ADMIN_USERNAME` | admin | Default admin username |
| `ADMIN_PASSWORD` | (required) | Default admin password |
| `TMDB_API_KEY` | | TheMovieDB API key |
| `OMDB_API_KEY` | | OMDB API key |
| `REDIS_HOST` | localhost | Redis host |
| `REDIS_PORT` | 6379 | Redis port |
| `LOG_LEVEL` | info | Log level: debug, info, warn, error |

### catalog-web Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `VITE_API_BASE_URL` | http://localhost:8080/api/v1 | Backend API URL |
| `VITE_WS_URL` | ws://localhost:8080/ws | WebSocket URL |
| `VITE_ENABLE_ANALYTICS` | false | Enable analytics |
| `VITE_ENABLE_DEBUG` | false | Enable debug mode |

---

## IDE Configuration

### Visual Studio Code

**Recommended Extensions:**
- Go (golang.go)
- ESLint (dbaeumer.vscode-eslint)
- Prettier (esbenp.prettier-vscode)
- Tailwind CSS IntelliSense (bradlc.vscode-tailwindcss)
- rust-analyzer (rust-lang.rust-analyzer)
- Kotlin (fwcd.kotlin)

**Workspace Settings (`.vscode/settings.json`):**
```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace",
  "editor.formatOnSave": true,
  "[go]": {
    "editor.defaultFormatter": "golang.go"
  },
  "[typescript]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  },
  "[typescriptreact]": {
    "editor.defaultFormatter": "esbenp.prettier-vscode"
  }
}
```

### GoLand / IntelliJ IDEA

1. Open `catalog-api/` as Go project
2. Enable Go modules support
3. Configure run configuration:
   - Program: `main.go`
   - Working directory: `catalog-api/`
   - Environment: Load from `.env`

---

## Debugging

### Backend (Go)

**Using Delve:**
```bash
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug main.go
dlv debug main.go

# Debug test
dlv test ./handlers -- -test.run TestCreateUser
```

**VSCode Debug Configuration (`.vscode/launch.json`):**
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch catalog-api",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/catalog-api/main.go",
      "env": {
        "GIN_MODE": "debug"
      },
      "args": []
    }
  ]
}
```

### Frontend (React)

**Browser DevTools:**
- Chrome DevTools (F12)
- React Developer Tools extension
- Redux DevTools (if using Redux)

**VSCode Debugging:**
Install "Debugger for Chrome" extension and add configuration.

---

## Hot Reload

### Backend (Air)

Install Air for automatic reloading:

```bash
go install github.com/cosmtrek/air@latest
```

Create `.air.toml` in `catalog-api/`:
```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./tmp/main ."
  bin = "tmp/main"
  include_ext = ["go", "tpl", "tmpl", "html"]
  exclude_dir = ["tmp", "vendor", "testdata"]
  delay = 1000
```

Run:
```bash
cd catalog-api && air
```

### Frontend (Vite)

Vite has built-in hot reload:
```bash
cd catalog-web && npm run dev
# Changes auto-reload in browser
```

---

## Container Development

### Full Stack with Podman/Docker

```bash
# Start all services
podman-compose -f docker-compose.dev.yml up

# Or with Docker
docker-compose -f docker-compose.dev.yml up

# Services:
# - catalog-api: http://localhost:8080
# - catalog-web: http://localhost:5173
# - postgres: localhost:5432
# - redis: localhost:6379
```

### Individual Containers

```bash
# PostgreSQL
podman run -d --name catalogizer-postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=catalogizer \
  -p 5432:5432 \
  postgres:15

# Redis
podman run -d --name catalogizer-redis \
  -p 6379:6379 \
  redis:7-alpine
```

---

## Troubleshooting

### Backend Issues

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080
# Or
netstat -tulpn | grep 8080

# Kill process
kill -9 <PID>
```

**Database migration errors:**
```bash
# Reset database
rm catalogizer.db
go run main.go  # Recreates with fresh migrations
```

**Go module issues:**
```bash
go clean -modcache
go mod download
go mod verify
```

### Frontend Issues

**npm install fails:**
```bash
# Clear npm cache
npm cache clean --force
rm -rf node_modules package-lock.json
npm install
```

**Port 5173 in use:**
```bash
# Change port in package.json:
"dev": "vite --port 3000"
```

**TypeScript errors:**
```bash
# Rebuild TypeScript
npm run type-check
# Clear cache
rm -rf node_modules/.vite
```

### Tauri Build Issues

**Linux missing dependencies:**
```bash
# Fedora
sudo dnf install webkit2gtk4.0-devel openssl-devel librsvg2-devel

# Ubuntu
sudo apt install libwebkit2gtk-4.0-dev libssl-dev librsvg2-dev
```

**macOS code signing:**
```bash
# Disable code signing for development
export TAURI_SKIP_CODESIGN=1
npm run tauri:build
```

### Android Build Issues

**Gradle sync failed:**
1. Invalidate caches: File → Invalidate Caches / Restart
2. Re-download dependencies: `./gradlew clean build --refresh-dependencies`

**SDK not found:**
```bash
# Set ANDROID_HOME
export ANDROID_HOME=$HOME/Android/Sdk
export PATH=$PATH:$ANDROID_HOME/tools:$ANDROID_HOME/platform-tools
```

---

## Quick Start Summary

**Full stack in 5 minutes:**

```bash
# 1. Start backend
cd catalog-api
echo "JWT_SECRET=dev-secret-key" > .env
go run main.go &

# 2. Start frontend
cd ../catalog-web
npm install
npm run dev &

# 3. Open browser
xdg-open http://localhost:5173
# Login: admin / <your-admin-password>
```

**Verify everything works:**
```bash
# Backend health
curl http://localhost:8080/health

# Run tests
cd catalog-api && go test ./...
cd ../catalog-web && npm run test -- --run
```

---

## Next Steps

- Read [API Documentation](../api/API_DOCUMENTATION.md)
- Review [Protocol Implementation Guides](PROTOCOL_IMPLEMENTATION_GUIDE.md)
- Check [Configuration Reference](CONFIGURATION_REFERENCE.md)
- Join the community and contribute!
