# Module 3: Authentication and Authorization - Script

**Duration**: 45 minutes
**Module**: 3 - Authentication and Authorization

---

## Scene 1: JWT Authentication (0:00 - 20:00)

**[Visual: Architecture diagram showing auth flow: Client -> Middleware -> AuthService -> UserRepository -> Database]**

**Narrator**: Welcome to Module 3. Authentication is the gatekeeper of every API. Catalogizer implements a full JWT-based authentication system with access tokens, refresh tokens, session tracking, and role-based access control. Let us trace the entire flow.

**[Visual: Open `catalog-api/internal/auth/service.go`]**

**Narrator**: The `AuthService` in the `internal/auth` package is the central authentication engine. It delegates token generation to the `digital.vasic.auth` submodule's JWT manager, while keeping validation logic local with typed claims parsing.

```go
// catalog-api/internal/auth/service.go
type AuthService struct {
    db            *database.DB
    jwtSecret     []byte
    jwtMgr        *jwtmod.Manager // access-token manager (24h TTL)
    jwtRefreshMgr *jwtmod.Manager // refresh-token manager (7d TTL)
    logger        *zap.Logger
    tokenTTL      time.Duration
}

func NewAuthService(db *database.DB, jwtSecret string, logger *zap.Logger) *AuthService {
    accessCfg := jwtmod.DefaultConfig(jwtSecret)
    accessCfg.Expiration = 24 * time.Hour

    refreshCfg := jwtmod.DefaultConfig(jwtSecret)
    refreshCfg.Expiration = 7 * 24 * time.Hour

    return &AuthService{
        db:            db,
        jwtSecret:     []byte(jwtSecret),
        jwtMgr:        jwtmod.NewManager(accessCfg),
        jwtRefreshMgr: jwtmod.NewManager(refreshCfg),
        logger:        logger,
        tokenTTL:      24 * time.Hour,
    }
}
```

**[Visual: Show token generation flow]**

**Narrator**: Two separate JWT managers handle access and refresh tokens. Access tokens expire in 24 hours and contain the user's ID, username, role, and session ID. Refresh tokens last 7 days and are used to obtain new access tokens without re-entering credentials.

**[Visual: Open `catalog-api/services/auth_service.go` showing JWTClaims]**

**Narrator**: The domain-level auth service defines JWT claims that carry authorization data. Notice the `SessionID` field -- this ties every token to a specific session, enabling per-device logout.

```go
// catalog-api/services/auth_service.go
type JWTClaims struct {
    UserID    int    `json:"user_id"`
    Username  string `json:"username"`
    RoleID    int    `json:"role_id"`
    SessionID string `json:"session_id"`
    jwt.RegisteredClaims
}
```

**[Visual: Show login flow in auth_service.go]**

**Narrator**: The login flow is methodical. First, find the user by username or email. Second, check account status -- is it locked or disabled? Third, verify the password using bcrypt with a per-user salt. Fourth, on success, generate a session ID, create both tokens, and record the session. On failure, increment the failed login counter for brute-force protection.

```go
// catalog-api/services/auth_service.go
func (s *AuthService) Login(req models.LoginRequest, ipAddress string, userAgent string) (*AuthResult, error) {
    user, err := s.userRepo.GetByUsernameOrEmail(req.Username)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, errors.New("invalid credentials")
        }
        return nil, fmt.Errorf("failed to get user: %w", err)
    }

    if !user.CanLogin() {
        if user.IsLocked {
            return nil, errors.New("account is temporarily locked")
        }
        return nil, errors.New("account is disabled")
    }

    if !s.verifyPassword(req.Password, user.Salt, user.PasswordHash) {
        s.userRepo.IncrementFailedLoginAttempts(user.ID)
        return nil, errors.New("invalid credentials")
    }
    // ... generate tokens and session
}
```

**[Visual: Show password hashing with bcrypt]**

**Narrator**: Passwords are hashed with bcrypt plus a random salt. The salt is generated per-user and stored alongside the hash. Bcrypt's built-in work factor makes brute-force attacks computationally expensive. Notice the error message is deliberately vague -- "invalid credentials" -- to prevent username enumeration.

**[Visual: Show refresh token flow]**

**Narrator**: Token refresh is handled by `RefreshToken()`. The client sends the refresh token, which is validated and exchanged for a new access-refresh pair. The old refresh token is invalidated to prevent replay attacks. This is called refresh token rotation.

**[Visual: Show the initialization sequence]**

**Narrator**: On first startup, `Initialize()` creates the auth tables (on SQLite; PostgreSQL uses the migration system) and seeds a default admin user. The admin password comes from the `ADMIN_PASSWORD` environment variable, which always overrides `config.json`.

```go
// catalog-api/internal/auth/service.go
func (s *AuthService) Initialize() error {
    if err := s.createTables(); err != nil {
        return fmt.Errorf("failed to create auth tables: %w", err)
    }
    if err := s.createDefaultAdmin(); err != nil {
        return fmt.Errorf("failed to create default admin: %w", err)
    }
    s.logger.Info("Authentication system initialized")
    return nil
}
```

---

## Scene 2: Role-Based Access Control (20:00 - 35:00)

**[Visual: Open `catalog-api/internal/auth/middleware.go`]**

**Narrator**: Authentication tells us who the user is. Authorization tells us what they can do. Catalogizer's middleware chain handles both.

```go
// catalog-api/internal/auth/middleware.go
type AuthMiddleware struct {
    authService *AuthService
    logger      *zap.Logger
}

func NewAuthMiddleware(authService *AuthService, logger *zap.Logger) *AuthMiddleware {
    return &AuthMiddleware{
        authService: authService,
        logger:      logger,
    }
}
```

**[Visual: Show `RequireAuth` middleware]**

**Narrator**: `RequireAuth` is a Gin middleware that extracts the Bearer token from the Authorization header, validates it against the auth service, and populates the Gin context with user information. If validation fails, the request is aborted with a 401.

```go
// catalog-api/internal/auth/middleware.go
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
    return func(c *gin.Context) {
        token := m.extractToken(c)
        if token == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing or invalid authorization header"})
            c.Abort()
            return
        }

        user, err := m.authService.ValidateToken(token)
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
            c.Abort()
            return
        }

        c.Set("user", user)
        c.Set("user_id", user.ID)
        c.Set("username", user.Username)
        c.Set("role", user.Role)
        c.Set("permissions", user.Permissions)
        c.Next()
    }
}
```

**[Visual: Show `RequirePermission` middleware]**

**Narrator**: `RequirePermission` builds on `RequireAuth`. It checks whether the authenticated user has a specific permission. Admin users bypass all permission checks. Non-admin users must have the exact permission string in their role's permission set.

```go
// catalog-api/internal/auth/middleware.go
func (m *AuthMiddleware) RequirePermission(permission string) gin.HandlerFunc {
    return func(c *gin.Context) {
        user, exists := c.Get("user")
        if !exists {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
            c.Abort()
            return
        }

        userObj, ok := user.(*User)
        if !ok {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid user context"})
            c.Abort()
            return
        }

        if !userObj.HasPermission(permission) && !userObj.IsAdmin() {
            c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

**[Visual: Show role definitions in `internal/auth/models.go`]**

**Narrator**: Roles are defined in the `internal/auth/models.go` file. Each role has a name and a set of permission strings. The User model includes an `IsAdmin()` helper and a `HasPermission()` method that checks the user's permission set.

**[Visual: Show middleware chaining in route registration]**

**Narrator**: In `main.go`, middleware is chained per route group. Public endpoints like `/api/v1/auth/login` have no middleware. Protected endpoints use `RequireAuth()`. Admin endpoints chain both `RequireAuth()` and `RequirePermission("admin")`.

**[Visual: Show the handler layer using `c.Get("user")`]**

**Narrator**: Downstream handlers access the authenticated user through Gin's context. The `c.Get("user")` call retrieves the full user object, including their role and permissions, set by the middleware.

**[Visual: Open `catalog-api/handlers/role_handler.go`]**

**Narrator**: The role handler exposes CRUD endpoints for managing roles and assigning them to users. Only administrators can modify roles, enforced by the middleware chain.

---

## Scene 3: Multi-Device Sessions (35:00 - 45:00)

**[Visual: Diagram showing multiple devices with session tokens]**

**Narrator**: Modern applications need to handle users logged in from multiple devices simultaneously. Catalogizer tracks active sessions with device information, IP addresses, and last-activity timestamps.

**[Visual: Show session tracking in auth_service.go]**

**Narrator**: Each login creates a unique session ID using cryptographically secure random bytes. The session is stored in the database alongside the user agent and IP address. This enables the user to see all their active sessions and selectively revoke them.

**[Visual: Show logout flow]**

**Narrator**: Logout comes in two flavors. Single logout invalidates only the current session token. "Logout all devices" queries all active sessions for the user and invalidates them in a single transaction.

```go
// catalog-api/handlers/auth_handler.go
func (h *AuthHandler) LogoutGin(c *gin.Context) {
    token := extractTokenFromGin(c)
    if token == "" {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization token required"})
        return
    }

    err := h.authService.Logout(token)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
```

**[Visual: Show session deactivation]**

**Narrator**: Session deactivation works by marking the session as inactive in the database. When a token is validated, the auth service checks both the JWT signature and the session's active status. A valid JWT from a deactivated session is rejected.

**[Visual: Show the user handler listing active sessions]**

**Narrator**: The user handler exposes an endpoint to list all active sessions for the current user. Each session includes the device type, IP address, and last activity time, so users can identify and terminate suspicious sessions.

**[Visual: Course title card]**

**Narrator**: That completes our authentication module. You have seen JWT generation with dual managers, refresh token rotation, role-based access control with permission middleware, and multi-device session management. In Module 4, we tackle the media detection pipeline -- the core intelligence of Catalogizer.

---

## Key Code Examples

### JWT Middleware Chain
```go
// main.go - Route registration
authMiddleware := auth.NewAuthMiddleware(authService, logger)

// Public routes
router.POST("/api/v1/auth/login", authHandler.LoginGin)
router.POST("/api/v1/auth/refresh", authHandler.RefreshTokenGin)

// Protected routes
api := router.Group("/api/v1")
api.Use(authMiddleware.RequireAuth())
api.GET("/files", fileHandler.ListFiles)
api.GET("/me", authHandler.GetCurrentUserGin)

// Admin-only routes
admin := api.Group("/admin")
admin.Use(authMiddleware.RequirePermission("admin"))
admin.GET("/users", userHandler.ListUsers)
```

### Password Hashing
```go
// services/auth_service.go
func (s *AuthService) hashPassword(password, salt string) string {
    combined := password + salt
    hash, _ := bcrypt.GenerateFromPassword([]byte(combined), bcrypt.DefaultCost)
    return string(hash)
}

func (s *AuthService) verifyPassword(password, salt, hash string) bool {
    combined := password + salt
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(combined)) == nil
}
```

### Config Precedence for Secrets
```bash
# Environment variables always override config.json
export JWT_SECRET=your-production-secret
export ADMIN_PASSWORD=<your-admin-password>
```

---

## Quiz Questions

1. Why does Catalogizer use two separate JWT managers (access and refresh)?
   **Answer**: Access tokens (24h TTL) are used for API authentication and are sent with every request. Refresh tokens (7d TTL) are used only to obtain new access tokens. Separating them allows short-lived access tokens (limiting the damage window if stolen) while providing a smooth user experience through silent refresh.

2. What happens when a user's password verification fails?
   **Answer**: The failed login attempt counter is incremented in the database. The response returns a generic "invalid credentials" error (not specifying whether the username or password was wrong) to prevent username enumeration attacks. After enough failures, the account is temporarily locked.

3. How does the permission middleware determine if a user is authorized?
   **Answer**: The `RequirePermission` middleware first checks if the user is an admin (admins bypass all permission checks). For non-admin users, it checks if the required permission string exists in the user's role permission set. If not, it returns 403 Forbidden.

4. How does "logout all devices" work without invalidating JWTs at the cryptographic level?
   **Answer**: Each JWT contains a `session_id` claim. When "logout all devices" is triggered, all the user's sessions are marked inactive in the database. During token validation, the auth service checks both the JWT signature and the session's active status. Valid JWTs tied to deactivated sessions are rejected.
