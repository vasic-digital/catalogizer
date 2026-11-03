# Catalogizer Authentication Flow Documentation

This document describes the authentication and authorization system used across the Catalogizer platform, including JWT token handling, session management, role-based access control, and the dual auth service architecture.

## Overview

Catalogizer implements JWT-based authentication with two complementary auth service layers:

1. **Root Auth Service** (`catalog-api/services/auth_service.go`) -- Primary authentication for the root-level middleware. Uses salted bcrypt password hashing and the `repository.UserRepository` for data access.
2. **Internal Auth Service** (`catalog-api/internal/auth/service.go`) -- Internal authentication service with rate limiting middleware. Uses bcrypt directly and manages its own table schema.

Both services share the same JWT secret configured via `Auth.JWTSecret` in config.json or the `JWT_SECRET` environment variable.

## Login Flow

```mermaid
sequenceDiagram
    participant Client
    participant API as catalog-api
    participant MW as Auth Rate Limiter<br/>(5 req/min)
    participant AuthSvc as AuthService
    participant UserRepo as UserRepository
    participant DB as SQLite

    Client->>API: POST /api/v1/auth/login<br/>{ username, password, device_info, remember_me }
    API->>MW: Check rate limit
    alt Rate Limit Exceeded
        MW-->>Client: 429 Too Many Requests<br/>{ error, retry_after }
    end
    MW->>AuthSvc: Login(req, ipAddress, userAgent)
    AuthSvc->>UserRepo: GetByUsernameOrEmail(username)
    UserRepo->>DB: SELECT FROM users WHERE username=? OR email=?
    DB-->>UserRepo: User record
    UserRepo-->>AuthSvc: User object (with role)

    AuthSvc->>AuthSvc: Check user.CanLogin()<br/>(is_active && !is_locked)
    alt Account Locked
        AuthSvc-->>Client: 401 "account is temporarily locked"
    end
    alt Account Disabled
        AuthSvc-->>Client: 401 "account is disabled"
    end

    AuthSvc->>AuthSvc: Verify password<br/>(bcrypt with salt)
    alt Invalid Password
        AuthSvc->>UserRepo: IncrementFailedLoginAttempts(userID)
        AuthSvc-->>Client: 401 "invalid credentials"
    end

    AuthSvc->>UserRepo: ResetFailedLoginAttempts(userID)
    AuthSvc->>UserRepo: CreateSession(user, deviceInfo, ip, ua)
    UserRepo->>DB: INSERT INTO user_sessions
    DB-->>UserRepo: Session ID

    AuthSvc->>AuthSvc: GenerateJWT(user, sessionID)<br/>Claims: user_id, username, role_id
    AuthSvc->>AuthSvc: GenerateRefreshToken()<br/>(32-byte random hex)
    AuthSvc->>UserRepo: UpdateSessionTokens(sessionID, jwt, refreshToken)
    AuthSvc->>UserRepo: UpdateLastLogin(userID, ipAddress)

    AuthSvc-->>Client: 200 OK<br/>{ user, session_token, refresh_token, expires_at }
```

## JWT Refresh Flow

```mermaid
sequenceDiagram
    participant Client
    participant API as catalog-api
    participant MW as Auth Rate Limiter
    participant AuthSvc as AuthService
    participant UserRepo as UserRepository
    participant DB as SQLite

    Client->>API: POST /api/v1/auth/refresh<br/>{ refresh_token }
    API->>MW: Check rate limit
    MW->>AuthSvc: RefreshToken(refreshToken)

    AuthSvc->>UserRepo: GetSessionByRefreshToken(refreshToken)
    UserRepo->>DB: SELECT FROM user_sessions<br/>WHERE refresh_token=? AND is_active=1
    DB-->>UserRepo: Session record

    alt Session Not Found or Expired
        AuthSvc-->>Client: 401 "invalid or expired refresh token"
    end

    AuthSvc->>UserRepo: GetByID(session.user_id)
    UserRepo->>DB: SELECT FROM users WHERE id=?
    DB-->>UserRepo: User record

    alt User Inactive or Locked
        AuthSvc-->>Client: 401 "account disabled"
    end

    AuthSvc->>AuthSvc: GenerateJWT(user, sessionID)<br/>New access token
    AuthSvc->>AuthSvc: GenerateRefreshToken()<br/>New refresh token (rotation)
    AuthSvc->>UserRepo: UpdateSessionTokens(sessionID, newJWT, newRefresh)
    UserRepo->>DB: UPDATE user_sessions SET<br/>session_token=?, refresh_token=?,<br/>last_activity_at=CURRENT_TIMESTAMP

    AuthSvc-->>Client: 200 OK<br/>{ session_token, refresh_token, expires_at }
```

## Request Authentication Flow

```mermaid
sequenceDiagram
    participant Client
    participant MW as JWTMiddleware
    participant RateLimiter as Rate Limiter<br/>(100 req/min)
    participant Handler as Route Handler
    participant DB as SQLite

    Client->>MW: GET /api/v1/catalog<br/>Authorization: Bearer <JWT>
    MW->>MW: Extract token from<br/>"Bearer <token>" header

    alt No Authorization Header
        MW-->>Client: 401 "Authorization header required"
    end
    alt Invalid Header Format
        MW-->>Client: 401 "Invalid authorization header format"
    end

    MW->>MW: Parse JWT with Claims<br/>Validate signature (HS256)
    MW->>MW: Check token expiration

    alt Token Expired or Invalid
        MW-->>Client: 401 "Invalid token"
    end

    MW->>MW: Set context:<br/>username, user_id
    MW->>RateLimiter: Check per-user rate limit<br/>(sliding window)

    alt Rate Limit Exceeded
        RateLimiter-->>Client: 429 Too Many Requests
    end

    RateLimiter->>Handler: Process request
    Handler-->>Client: 200 Response
```

## Logout Flow

```mermaid
sequenceDiagram
    participant Client
    participant API as catalog-api
    participant AuthSvc as AuthService
    participant DB as SQLite

    Client->>API: POST /api/v1/auth/logout<br/>Authorization: Bearer <JWT>
    API->>AuthSvc: Logout(token)
    AuthSvc->>DB: DELETE FROM sessions<br/>WHERE token=?
    AuthSvc->>AuthSvc: Parse token for user_id
    AuthSvc->>DB: INSERT INTO auth_audit_log<br/>(user_id, "logout", ...)
    AuthSvc-->>Client: 200 OK
```

## Token Structure

### Access Token (JWT)

The access token is a standard JWT signed with HMAC-SHA256 (`HS256`).

**Header:**
```json
{
  "alg": "HS256",
  "typ": "JWT"
}
```

**Payload (Claims) -- Root Auth Service:**
```json
{
  "user_id": 1,
  "username": "admin",
  "role_id": 1,
  "session_id": "sess_abc123",
  "sub": "1",
  "exp": 1705420800,
  "iat": 1705334400,
  "nbf": 1705334400,
  "iss": "catalog-api"
}
```

Defined in: `catalog-api/services/auth_service.go` -- `JWTClaims`

**Payload (Claims) -- Internal Auth Service:**
```json
{
  "user_id": 1,
  "username": "admin",
  "role": "admin",
  "permissions": [
    "read:media",
    "write:media",
    "delete:media",
    "read:catalog",
    "write:catalog",
    "delete:catalog",
    "trigger:analysis",
    "view:analysis",
    "manage:users",
    "manage:roles",
    "view:logs",
    "admin:system",
    "access:api",
    "write:api"
  ],
  "type": "access",
  "iat": 1705334400,
  "exp": 1705420800
}
```

Defined in: `catalog-api/internal/auth/models.go` -- `Claims`

### Token Lifetimes

| Token Type | Lifetime | Description |
|------------|----------|-------------|
| Access Token (JWT) | 24 hours | Short-lived token for API authentication |
| Refresh Token | 7 days | Long-lived token for obtaining new access tokens |
| Session | 24 hours | Server-side session tied to access token |

### Token Generation

- **JWT Secret**: Configured via `Auth.JWTSecret` in `config.json` or `JWT_SECRET` environment variable. If not set or set to `"change-this-secret-in-production"`, a cryptographically secure 32-byte random secret is generated at startup (ephemeral, sessions will not persist across restarts).
- **Signing Algorithm**: HMAC-SHA256 (`jwt.SigningMethodHS256`)
- **Refresh Token**: 32-byte cryptographically random hex string (via `crypto/rand`)

### Refresh Token

The refresh token is an opaque random string, not a JWT. It is stored in the `user_sessions` table alongside the access token and is used to obtain a new access/refresh token pair without re-entering credentials.

Token rotation is implemented: each refresh operation generates both a new access token and a new refresh token, invalidating the previous refresh token.

## Permission Model

### Role-Based Access Control (RBAC)

Users are assigned a role, and each role has a set of permissions. Permissions follow the `resource:action` naming convention (internal auth) or `resource.action` dot notation (root models).

### Default Roles

#### Internal Auth Service Roles

| Role | Permissions |
|------|------------|
| `admin` | `read:media`, `write:media`, `delete:media`, `read:catalog`, `write:catalog`, `delete:catalog`, `trigger:analysis`, `view:analysis`, `manage:users`, `manage:roles`, `view:logs`, `admin:system`, `access:api`, `write:api` |
| `moderator` | `read:media`, `write:media`, `read:catalog`, `write:catalog`, `trigger:analysis`, `view:analysis`, `access:api`, `write:api` |
| `user` | `read:media`, `write:media`, `read:catalog`, `write:catalog`, `view:analysis`, `access:api` |
| `viewer` | `read:media`, `read:catalog`, `view:analysis`, `access:api` |

Source: `catalog-api/internal/auth/models.go` -- `GetRolePermissions()`

#### Root Models Roles (Migration-seeded)

| Role ID | Name | Permissions |
|---------|------|------------|
| 1 | Admin | `["*"]` (wildcard -- all permissions) |
| 2 | User | `["media.view", "media.download"]` |

Source: `catalog-api/database/migrations.go` -- `createAuthTables()`

### Wildcards and the admin gate

- `*` grants every permission. A prefix wildcard must end at a segment boundary: `system.*` (dot form) and `admin:*` (colon form) are valid; `sys*`, `s*` and `system*` are NOT treated as wildcards and grant nothing beyond a literal match (`Permissions.HasPermission`).
- `/api/v1/admin/*` (and the two admin routes registered on the API group, `image-quality/revalidate` and `covers/generate-thumbnails`) are gated by `RequireAdmin`, which reads the caller's CURRENT role from the database on every request (`UserRepository.IsAdmin`: active user whose role holds `system.admin`, `*` or a covering wildcard). The `role_id` claim in the access token is not trusted for this decision, so demotions, role permission edits and deactivations apply on the next request. A lookup failure answers 503 (fail closed).
- The storage-operator routes (`/storage/roots`, `/storage-roots*`, `/storage/list/*`, `POST /scans`, all `/smb/*`, `/copy/*`) are gated by `RequirePermission("storage.manage")`, also decided live from the database (`UserRepository.HasPermission`: an active user whose role holds the permission, `system.admin`, `*` or a covering wildcard); a stale `role_id` claim never grants and a lookup failure answers 503. The standard user role (id 2) does not hold it, so open self-registration cannot reach these routes. `POST /challenges/*/run` needs the admin role (`RequireAdmin`). Review round 1 added: the sync endpoint mutations (`POST/PUT/DELETE /sync/endpoints*`, `POST /sync/endpoints/:id/sync`, `POST /sync/schedules`) need `storage.manage`; the configuration wizard writes (`/configuration/wizard/step/:id/{validate,save}`, `/configuration/wizard/complete`) need `system.configure` (admin also passes); `GET /analytics/system`, `GET /reports/{usage,performance}` and `POST /sync/cleanup` are admin only; `GET /analytics/user/:user_id` and `GET /recommendations/personalized/:user_id` need the caller to be that user or an admin (`RequireSelfOrAdmin`, 403 otherwise). Near-miss permission strings such as `storage.managex` never admit (exact match or a segment-boundary wildcard only). Deny by default: a middleware built without the checker admits only the token's admin role.
- A non-admin with `user.manage`/`user.create` may assign only roles whose every permission it holds itself (the standard user role, id 2, is always assignable).

### Permission Categories

#### Internal Auth Permissions (resource:action format)

| Permission | Resource | Action | Description |
|------------|----------|--------|-------------|
| `read:media` | media | read | View media items and metadata |
| `write:media` | media | write | Create and update media items |
| `delete:media` | media | delete | Delete media items |
| `read:catalog` | catalog | read | Browse file catalog |
| `write:catalog` | catalog | write | Modify file catalog |
| `delete:catalog` | catalog | delete | Delete from catalog |
| `trigger:analysis` | analysis | trigger | Start media analysis |
| `view:analysis` | analysis | view | View analysis results |
| `manage:users` | users | manage | Create, update, delete users |
| `manage:roles` | roles | manage | Create, update, delete roles |
| `view:logs` | logs | view | View system logs |
| `admin:system` | system | admin | Full system administration |
| `access:api` | api | access | Access API endpoints |
| `write:api` | api | write | Modify data via API |

#### Root Model Permissions (resource.action format)

| Permission | Description |
|------------|-------------|
| `system.admin` | Full system administration |
| `system.configure` | System configuration access |
| `user.view/create/update/delete/manage` | User management operations |
| `media.view/upload/edit/delete/share/download/convert/manage` | Media operations |
| `share.view/create/edit/delete/manage` | Share management |
| `analytics.view/export/manage` | Analytics access |
| `favorite.view/create/delete` | Favorites management |
| `sync.view/create/execute/manage` | Sync operations |
| `conversion.view/create/manage` | Conversion job management |
| `report.view/create/export` | Report generation |
| `*` | Wildcard -- grants all permissions |

Source: `catalog-api/models/user.go` -- Permission constants

### Permission Checking

Permissions support wildcard matching:

1. **Exact match**: `"media.view"` matches `"media.view"`
2. **Wildcard**: `"*"` matches any permission
3. **Prefix wildcard**: `"media.*"` matches `"media.view"`, `"media.edit"`, etc.
4. **Admin override**: Admin role users (`admin:system` or `*`) bypass all permission checks

Source: `catalog-api/models/user.go` -- `Permissions.HasPermission()`

### Account Security

#### Failed Login Protection

- Failed login attempts are tracked per user (`failed_login_attempts` column)
- On successful login, the counter is reset to zero
- Account locking is supported via `is_locked` and `locked_until` fields
- `CanLogin()` checks both `is_active` and lock status

#### Audit Trail

All authentication events are logged to the `auth_audit_log` table:

| Event Type | Description |
|------------|-------------|
| `login_success` | Successful authentication |
| `failed_login` | Invalid credentials |
| `failed_login_inactive` | Login attempt on disabled account |
| `logout` | Session termination |
| `password_changed` | Password update |

Each audit entry captures: user_id, IP address, user agent, and additional details.

## Middleware Stack

The authentication and authorization middleware is applied in the following order:

```mermaid
graph TD
    REQ["Incoming Request"] --> CORS["CORS Middleware"]
    CORS --> METRICS["Prometheus Metrics"]
    METRICS --> LOGGER["Request Logger (Zap)"]
    LOGGER --> ERR["Error Handler"]
    ERR --> REQID["Request ID"]
    REQID --> INPUT["Input Validation"]

    INPUT --> AUTH_CHECK{"/api/v1/auth/*?"}
    AUTH_CHECK -->|Yes| AUTH_RL["Auth Rate Limiter<br/>(5 req/min)"]
    AUTH_RL --> AUTH_HANDLER["Auth Handlers<br/>(login, register, refresh, logout)"]

    AUTH_CHECK -->|No| JWT["JWT RequireAuth()"]
    JWT --> API_RL["API Rate Limiter<br/>(100 req/min)"]
    API_RL --> HANDLER["Route Handler"]

    style AUTH_RL fill:#ffcdd2
    style JWT fill:#c8e6c9
    style API_RL fill:#fff9c4
```

### Middleware Details

| Middleware | Source | Description |
|-----------|--------|-------------|
| CORS | `catalog-api/middleware/request.go` | Cross-origin resource sharing headers |
| Prometheus Metrics | `catalog-api/internal/metrics/` | Request timing and counting |
| Logger | `catalog-api/internal/middleware/` | Structured request logging (Zap) |
| Error Handler | `catalog-api/internal/middleware/` | Global error recovery |
| Request ID | `catalog-api/middleware/request.go` | Unique request ID generation |
| Input Validation | `catalog-api/middleware/input_validation.go` | Request body sanitization |
| JWT RequireAuth | `catalog-api/middleware/auth.go` | Token extraction and validation |
| Rate Limiter | `catalog-api/internal/auth/middleware.go` | Per-user sliding window rate limiting |

### Rate Limiting

Two rate limiting tiers are applied:

| Tier | Endpoints | Limit | Window | Implementation |
|------|-----------|-------|--------|---------------|
| Auth | `/api/v1/auth/*` | 5 requests | 1 minute | In-memory sliding window per user |
| API | `/api/v1/*` (authenticated) | 100 requests | 1 minute | In-memory sliding window per user |

The rate limiter uses an in-memory sliding window algorithm. Redis-based distributed rate limiting is available when Redis is configured (`REDIS_ADDR` environment variable), but falls back to in-memory when Redis is unavailable.

Source: `catalog-api/internal/auth/middleware.go` -- `RateLimitByUser()`

## API Endpoints

| Method | Path | Auth Required | Description |
|--------|------|---------------|-------------|
| POST | `/api/v1/auth/login` | No | Authenticate and get tokens |
| POST | `/api/v1/auth/register` | No | Create new user account |
| POST | `/api/v1/auth/refresh` | No | Refresh access token |
| POST | `/api/v1/auth/logout` | No | Invalidate session |
| GET | `/api/v1/auth/me` | Yes | Get current user info |

All other `/api/v1/*` routes require a valid JWT in the `Authorization: Bearer <token>` header.
