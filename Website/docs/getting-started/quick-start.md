---
title: Quick Start
description: Get Catalogizer running in 5 minutes with the backend and frontend
---

# Quick Start

This guide gets Catalogizer running on your local machine in about five minutes. You will start the backend, start the frontend, log in, and browse the interface.

---

## Prerequisites

You need one of the following setups:

- **Minimal**: Go 1.25+ and Node.js 18+
- **Container-based**: Podman 5+ with podman-compose (or Docker 20.10+ with Docker Compose v2+)

Git is required for both options.

---

## Step 1: Clone the Repository

```bash
git clone git@github.com:vasic-digital/Catalogizer.git
cd Catalogizer
scripts/safe_submodule_init.sh   # never `--recursive` through constitution/ (see docs/GOVERNANCE.md)
```

---

## Step 2: Start the Backend

### Option A: Manual (SQLite, no dependencies)

```bash
cd catalog-api
cp .env.example .env
```

Edit `.env` and set at minimum:

```env
JWT_SECRET=your-dev-secret-key-at-least-32-chars
ADMIN_PASSWORD=admin123
```

Start the server:

```bash
go run main.go
```

The backend creates an SQLite database automatically and writes its port to `.service-port`. You will see output indicating the server is running.

### Option B: Containers (PostgreSQL + Redis)

```bash
cp .env.example .env
# Edit .env: set POSTGRES_PASSWORD and JWT_SECRET

podman-compose -f docker-compose.dev.yml up -d
```

The API, PostgreSQL, and Redis start automatically. The API is available at `http://localhost:8080`.

---

## Step 3: Start the Frontend

Open a second terminal:

```bash
cd catalog-web
npm install
npm run dev
```

The frontend starts on port 3000. It reads `../catalog-api/.service-port` to automatically route API requests to the backend. If port 3000 is already in use by another process, stop that process first:

```bash
ss -tlnp | grep :3000
```

---

## Step 4: Log In

Open `http://localhost:3000` in your browser. Log in with:

- **Username**: `admin`
- **Password**: The `ADMIN_PASSWORD` value from your `.env` file (default: `admin123`)

You will see the dashboard with an empty catalog.

---

## Step 5: Explore the Interface

With the system running, you can explore these sections:

| Section | What You Will See |
|---------|-------------------|
| **Dashboard** | Catalog statistics, source health, recent additions |
| **Browse** | Media browser (empty until you add a storage source and scan) |
| **Search** | Full-text search across all media metadata |
| **Collections** | Create Manual, Smart, or Dynamic collections |
| **Settings** | Add storage sources, manage users, configure preferences |

---

## What's Next

Now that Catalogizer is running:

- **[Add a storage source and run your first scan](/docs/getting-started/first-scan)** to populate your catalog
- **[Configure environment variables](/docs/getting-started/configuration)** for API keys, database settings, and other options
- **[Read the Web App Guide](/guides/web-app)** for a tour of browsing, collections, and playback
- **[Install native apps](/download)** for desktop, Android, or Android TV

---

## Stopping Services

### Manual Setup

Press `Ctrl+C` in each terminal to stop the backend and frontend.

### Containers

```bash
podman-compose -f docker-compose.dev.yml down
```

---

## Troubleshooting

### Port 3000 Already in Use

Another process (such as Bear Messenger) may be using port 3000. Find and stop it:

```bash
ss -tlnp | grep :3000
# Kill the process using that port, then restart npm run dev
```

### Backend Won't Start

Check that Go 1.25+ is installed:

```bash
go version
```

Verify that `.env` exists in `catalog-api/` and contains valid `JWT_SECRET` and `ADMIN_PASSWORD` values.

### Frontend Can't Reach Backend

The frontend reads `../catalog-api/.service-port` to find the backend port. Verify this file exists and contains a port number:

```bash
cat catalog-api/.service-port
```

If the file is missing, the backend did not start successfully. Check the backend terminal for errors.
