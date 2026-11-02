---
title: Configuration
description: Environment variables, config.json, and .env setup for Catalogizer
---

# Configuration

Catalogizer is configured through environment variables, `.env` files, and a `config.json` file. This page covers all available settings, the precedence rules, and recommended configurations for development and production.

---

## Configuration Precedence

Settings are resolved in this order (highest priority first):

1. **Environment variables** -- set in the shell or container runtime
2. **`.env` file** -- in the `catalog-api/` directory
3. **`config.json`** -- in the `catalog-api/` directory
4. **Defaults** -- built into the application

A value set as an environment variable always overrides the same key in `.env` or `config.json`.

---

## Environment Variables

### Required Settings

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_SECRET` | Secret key for signing JWT tokens. Use 32+ random characters in production. | `a-random-string-at-least-32-characters` |
| `ADMIN_PASSWORD` | Password for the default admin account. Change immediately after first login. | `<ADMIN_PASSWORD>` |

### Server Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | HTTP server listen port |
| `GIN_MODE` | `debug` | Gin framework mode (`debug`, `release`, `test`) |
| `HOST` | `0.0.0.0` | Bind address |

### Database Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `DB_TYPE` | `sqlite` | Database backend: `sqlite` or `postgres` |
| `DB_HOST` | `localhost` | PostgreSQL hostname |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_NAME` | `catalogizer` | PostgreSQL database name |
| `DB_USER` | `catalogizer` | PostgreSQL username |
| `DB_PASSWORD` | (none) | PostgreSQL password |
| `DB_ENCRYPTION_KEY` | (none) | SQLCipher encryption key (exactly 32 characters) |

### External Metadata Providers

| Variable | Default | Description |
|----------|---------|-------------|
| `TMDB_API_KEY` | (none) | TMDB API key for movie/TV metadata |
| `OMDB_API_KEY` | (none) | OMDB API key for ratings and plot data |

These keys are optional. The media detection pipeline works without them but produces less enriched metadata. When a provider key is missing, that provider is skipped and the pipeline continues with data from available providers.

### CORS Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `CORS_ORIGINS` | `http://localhost:3000` | Comma-separated list of allowed origins |

### Redis Settings

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_URL` | (none) | Redis connection URL (e.g., `redis://localhost:6379`) |

Redis is optional. When not configured, the application uses in-memory caching. Rate limiting also falls back to in-memory counters.

---

## .env File

Create a `.env` file in the `catalog-api/` directory. A template is provided at `.env.example`:

```env
PORT=8080
GIN_MODE=debug
DB_TYPE=sqlite
JWT_SECRET=your-dev-secret-key
ADMIN_PASSWORD=<ADMIN_PASSWORD>
TMDB_API_KEY=your_tmdb_key
OMDB_API_KEY=your_omdb_key
```

The `.env` file is listed in `.gitignore` and must never be committed to version control. If your `.env` contains real API keys, verify that `.gitignore` covers it before every commit.

---

## config.json

The `config.json` file in `catalog-api/` provides non-sensitive defaults. It is version-controlled and shared across developers.

Key settings:

| Key | Default | Description |
|-----|---------|-------------|
| `write_timeout` | `900` | HTTP write timeout in seconds. Must be 900 (not 30) for long-running operations like `RunAll` challenges. |
| `max_open_conns` | `25` | Maximum open database connections |
| `max_idle_conns` | `10` | Maximum idle database connections |
| `conn_max_lifetime` | `5m` | Maximum connection lifetime |
| `conn_max_idle_time` | `3m` | Maximum idle time before connection is closed |

---

## Development Configuration

For local development with SQLite (no external dependencies):

```env
PORT=8080
GIN_MODE=debug
DB_TYPE=sqlite
JWT_SECRET=dev-secret-change-in-production
ADMIN_PASSWORD=<ADMIN_PASSWORD>
```

This is the simplest setup. The backend creates `catalogizer.db` automatically and writes its port to `.service-port` for the frontend to discover.

---

## Production Configuration

For production with PostgreSQL, Redis, and proper security:

```env
PORT=8080
GIN_MODE=release
DB_TYPE=postgres
DB_HOST=db.example.com
DB_PORT=5432
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=strong-random-password
JWT_SECRET=production-secret-64-chars-minimum-randomly-generated
ADMIN_PASSWORD=strong-admin-password
REDIS_URL=redis://redis.example.com:6379
TMDB_API_KEY=your_production_tmdb_key
CORS_ORIGINS=https://catalog.example.com
```

Production recommendations:

- Set `GIN_MODE=release` to disable debug logging and stack traces in error responses
- Use PostgreSQL instead of SQLite for concurrent multi-user access
- Set a strong `JWT_SECRET` (64+ characters, randomly generated)
- Change `ADMIN_PASSWORD` immediately after first login
- Configure `CORS_ORIGINS` to match your frontend domain
- Enable Redis for distributed caching and rate limiting
- Terminate TLS at a reverse proxy (Nginx, Caddy, or Traefik) in front of the API

---

## Container Configuration

When running in containers, pass environment variables through the container runtime or mount a `.env` file:

```bash
# Environment variables
podman run -e JWT_SECRET=... -e DB_TYPE=postgres catalog-api

# Mount .env file
podman run -v ./catalog-api/.env:/app/.env catalog-api
```

Container resource limits are mandatory:

| Container | CPU Limit | Memory Limit |
|-----------|-----------|--------------|
| PostgreSQL | 1 CPU | 2 GB |
| catalog-api | 2 CPUs | 4 GB |
| catalog-web | 1 CPU | 2 GB |
| **Total** | **4 CPUs** | **8 GB** |

For NAS access, add the host mapping to the API container:

```bash
podman run --add-host=synology.local:192.168.0.241 catalog-api
```

---

## Dynamic Port Binding

The backend writes its bound port to `.service-port` at startup. The frontend dev server reads this file to configure the API proxy target, falling back to port 8080 if the file does not exist.

This mechanism allows the backend to bind to a dynamic port when 8080 is occupied, and the frontend automatically discovers the correct port without manual configuration.

---

## Frontend Configuration

The frontend reads its API target from the backend's `.service-port` file. Additional frontend configuration is in `catalog-web/vite.config.ts`:

- **Path aliases**: `@/components`, `@/hooks`, `@/lib`, `@/types`, `@/services`, `@/store`, `@/pages`, `@/assets`
- **API proxy**: Routes `/api` requests to the backend port discovered from `.service-port`
- **Build chunks**: Production builds split into `vendor`, `router`, `ui`, `charts`, and `utils` for optimal loading
