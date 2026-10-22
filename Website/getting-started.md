---
title: Getting Started
description: Quick start guide for Catalogizer - get up and running in minutes
---

# Getting Started with Catalogizer

This guide walks you through installing Catalogizer and scanning your first media source.

## Prerequisites

Choose one of the following setups:

- **Container setup** (recommended): Podman 5+ with podman-compose, or Docker 20.10+ with Docker Compose v2+. Catalogizer automatically detects the available container runtime.
- **Manual setup**: Go 1.25+, Node.js 18+, Git

## Step 1: Clone the Repository

```bash
git clone <repository-url>
cd Catalogizer
scripts/safe_submodule_init.sh   # never `git submodule update --recursive` through constitution/ (see docs/GOVERNANCE.md)
```

## Step 2: Start the Backend

### Option A: Using Containers (Recommended)

```bash
cp .env.example .env
# Edit .env and set POSTGRES_PASSWORD and JWT_SECRET

podman-compose up -d
# or: docker compose up -d
```

The API server, PostgreSQL, and Redis will start automatically. The API is available at `http://localhost:8080`.

### Option B: Manual Setup

Start the backend with SQLite (no database setup needed):

```bash
cd catalog-api
cp .env.example .env
# Edit .env with your configuration
go run main.go
```

The backend writes its port to `.service-port` for the frontend to discover.

## Step 3: Start the Frontend

In a separate terminal:

```bash
cd catalog-web
npm install
npm run dev
```

The web interface is available at `http://localhost:3000`. It automatically proxies API requests to the backend by reading the `.service-port` file.

## Step 4: Log In

Open `http://localhost:3000` in your browser. Log in with the default credentials:

- **Username**: `admin`
- **Password**: The value of `ADMIN_PASSWORD` from your `.env` file (default: `admin123`)

## Step 5: Add a Storage Source

1. Navigate to **Settings** in the sidebar
2. Click **Add Storage Source**
3. Choose a protocol:
   - **Local Filesystem**: Browse to a directory containing media files
   - **SMB/CIFS**: Enter the server address, share name, and credentials
   - **FTP/FTPS**: Enter the server address, port, and credentials
   - **NFS**: Enter the server and export path
   - **WebDAV**: Enter the URL and credentials
4. Test the connection
5. Save the source

## Step 6: Scan for Media

1. Go to the **Dashboard**
2. Click **Scan Now** or wait for the automatic scan
3. Watch real-time progress via WebSocket updates
4. Once complete, browse your catalog in the **Media Browser**

The scanner will:
- Detect media files across your storage sources
- Identify media types (movies, TV shows, music, games, software, and more)
- Extract quality metadata (resolution, codec, bitrate)
- Fetch external metadata from TMDB, IMDB, MusicBrainz, and other providers
- Build entity hierarchies (TV shows with seasons and episodes, artists with albums)

## What's Next?

Now that you have Catalogizer running with your media:

- **Browse and search** your media library with filters and multiple view modes
- **Create collections** to organize media thematically (Manual, Smart, or Dynamic)
- **Play media** directly in the browser with the built-in player
- **Set up monitoring** with Prometheus and Grafana for production deployments
- **Install native apps** for [desktop](/download#desktop-application), [Android](/download#android), or [Android TV](/download#android-tv-app)
- **Monitor performance** with [Prometheus metrics](/documentation#monitoring) and Grafana dashboards
- **Run challenges** to validate your deployment via the [challenge API](/documentation#testing)

## Running Tests

Verify everything is working correctly:

```bash
# Backend tests
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Frontend tests
cd catalog-web
npm run test

# All tests
./scripts/run-all-tests.sh
```

## Stopping Services

### Containers

```bash
podman-compose down
# or: docker compose down
```

### Manual Setup

Stop the backend and frontend processes with `Ctrl+C` in their respective terminals.

## Need Help?

- Check the [FAQ](/faq) for answers to common questions
- Visit the [Support](/support) page for troubleshooting guides
- Browse the full [Documentation](/documentation) hub
