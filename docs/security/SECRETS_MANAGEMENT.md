# Secrets Management

## Configuration Precedence

Catalogizer follows a strict precedence chain for all configuration values:

```
Environment variables  >  .env file  >  config.json  >  Defaults
```

Environment variables always win. This is critical for secrets -- never commit secrets to `config.json`.

## JWT Authentication

### JWT_SECRET

The JWT secret key is used by `JWTMiddleware` (`catalog-api/middleware/auth.go`) for signing and validating tokens.

```env
JWT_SECRET=your-production-secret-key-at-least-32-chars
```

- **Algorithm:** HMAC-SHA256 (`jwt.SigningMethodHS256`)
- **Issuer:** `catalog-api`
- **Claims:** `username`, `sub` (user ID), `exp`, `iat`, `nbf`
- **Token format:** `Authorization: Bearer <token>`

Tokens are generated via `GenerateToken(username, userID, expirationHours)` and validated on every authenticated request by the `RequireAuth()` middleware.

If `JWT_SECRET` is weak or predictable, an attacker can forge tokens. Use a cryptographically random string of at least 32 characters.

### ADMIN_PASSWORD

The admin user password, set at startup. Used for initial login.

```env
ADMIN_PASSWORD=<your-admin-password>
```

### ADMIN_USERNAME

Optional. Defaults to `admin` if not set.

```env
ADMIN_USERNAME=admin
```

## API Keys

### TMDB_API_KEY

The Movie Database API key for metadata enrichment. Optional -- metadata lookups are skipped if not set.

```env
TMDB_API_KEY=your_tmdb_api_key
```

### OMDB_API_KEY

Open Movie Database API key. Optional, same behavior as TMDB.

```env
OMDB_API_KEY=your_omdb_api_key
```

## Database Credentials

For PostgreSQL production deployments:

```env
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5433
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=strong-database-password
```

SQLite development mode requires no credentials (`DB_TYPE=sqlite`).

## .env File Setup

Create `.env` in `catalog-api/`:

```env
# Required
JWT_SECRET=change-this-to-a-random-64-char-string
ADMIN_PASSWORD=change-this

# Server
PORT=8080
GIN_MODE=release

# Database (production)
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5433
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=change-this

# Optional metadata providers
TMDB_API_KEY=
OMDB_API_KEY=

# CORS (production)
CORS_ALLOWED_ORIGINS=https://your-domain.com
```

The `.env` file is gitignored and must never be committed to version control.

## Container Secrets

When running with Podman, pass secrets as environment variables:

```bash
podman run --network host \
  -e JWT_SECRET="$JWT_SECRET" \
  -e ADMIN_PASSWORD="$ADMIN_PASSWORD" \
  -e DB_PASSWORD="$DB_PASSWORD" \
  catalog-api:latest
```

Or use `--env-file`:

```bash
podman run --network host --env-file ./catalog-api/.env catalog-api:latest
```

## Security Practices

1. **Never commit secrets** -- `.env`, credentials, and API keys must stay out of git
2. **Rotate JWT_SECRET** -- changing it invalidates all existing tokens (forced re-login)
3. **Use strong passwords** -- ADMIN_PASSWORD is the gateway to the entire system
4. **Separate environments** -- use different secrets for dev, staging, and production
5. **Minimize API key scope** -- TMDB/OMDB keys should be read-only
6. **PostgreSQL over SQLite** -- production deployments should use PostgreSQL with proper user permissions

## Source

- `catalog-api/middleware/auth.go` (JWT implementation)
- `catalog-api/main.go` (environment variable loading)
- `catalog-api/.env.example` or CLAUDE.md (reference configuration)
