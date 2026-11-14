package auth

import (
	"catalogizer/internal/logredact"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AuthMiddleware handles JWT authentication
type AuthMiddleware struct {
	authService *AuthService
	logger      *zap.Logger
	limiter     *slidingWindowLimiter // last limiter built by RateLimitByUser (test introspection only)
}

// NewAuthMiddleware creates a new auth middleware
func NewAuthMiddleware(authService *AuthService, logger *zap.Logger) *AuthMiddleware {
	return &AuthMiddleware{
		authService: authService,
		logger:      logger,
	}
}

// RequireAuth middleware that requires authentication
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
			m.logger.Debug("Token validation failed", zap.Error(err))
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
			c.Abort()
			return
		}

		// Set user in context
		c.Set("user", user)
		c.Set("user_id", user.ID)
		c.Set("username", user.Username)
		c.Set("role", user.Role)
		c.Set("permissions", user.Permissions)

		c.Next()
	}
}

// RequirePermission middleware that requires specific permission
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
			m.logger.Debug("Permission denied",
				zap.String("username", userObj.Username),
				zap.String("required_permission", permission))
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireRole middleware that requires specific role
func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
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

		// Check if user has any of the required roles
		hasRole := false
		for _, role := range roles {
			if userObj.Role == role {
				hasRole = true
				break
			}
		}

		if !hasRole && !userObj.IsAdmin() {
			m.logger.Debug("Role access denied",
				zap.String("username", userObj.Username),
				zap.String("user_role", userObj.Role),
				zap.Strings("required_roles", roles))
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient role privileges"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireResourceAccess middleware for resource-action based permissions
func (m *AuthMiddleware) RequireResourceAccess(resource, action string) gin.HandlerFunc {
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

		if !userObj.CanAccess(resource, action) {
			m.logger.Debug("Resource access denied",
				zap.String("username", userObj.Username),
				zap.String("resource", resource),
				zap.String("action", action))
			c.JSON(http.StatusForbidden, gin.H{
				"error":    "Access denied to resource",
				"resource": resource,
				"action":   action,
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// OptionalAuth middleware that extracts user if token is present but doesn't require it
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		token := m.extractToken(c)
		if token != "" {
			user, err := m.authService.ValidateToken(token)
			if err == nil {
				// Set user in context if token is valid
				c.Set("user", user)
				c.Set("user_id", user.ID)
				c.Set("username", user.Username)
				c.Set("role", user.Role)
				c.Set("permissions", user.Permissions)
			}
		}
		c.Next()
	}
}

// AdminOnly middleware that requires admin role
func (m *AuthMiddleware) AdminOnly() gin.HandlerFunc {
	return m.RequireRole(RoleAdmin)
}

// ModeratorOrAdmin middleware that requires moderator or admin role
func (m *AuthMiddleware) ModeratorOrAdmin() gin.HandlerFunc {
	return m.RequireRole(RoleModerator, RoleAdmin)
}

// extractToken extracts JWT token from Authorization header
func (m *AuthMiddleware) extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		return ""
	}

	// Check for Bearer token
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

// GetCurrentUser helper function to get current user from context
func GetCurrentUser(c *gin.Context) (*User, bool) {
	user, exists := c.Get("user")
	if !exists {
		return nil, false
	}

	userObj, ok := user.(*User)
	return userObj, ok
}

// GetCurrentUserID helper function to get current user ID from context
func GetCurrentUserID(c *gin.Context) (int64, bool) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, false
	}

	id, ok := userID.(int64)
	return id, ok
}

// HasPermission helper function to check if current user has permission
func HasPermission(c *gin.Context, permission string) bool {
	user, exists := GetCurrentUser(c)
	if !exists {
		return false
	}

	return user.HasPermission(permission) || user.IsAdmin()
}

// IsAdmin helper function to check if current user is admin
func IsAdmin(c *gin.Context) bool {
	user, exists := GetCurrentUser(c)
	if !exists {
		return false
	}

	return user.IsAdmin()
}

// CanAccessResource helper function to check resource access
func CanAccessResource(c *gin.Context, resource, action string) bool {
	user, exists := GetCurrentUser(c)
	if !exists {
		return false
	}

	return user.CanAccess(resource, action)
}

// LogUserActivity logs user activity for audit purposes
func (m *AuthMiddleware) LogUserActivity() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		// Log after request completion
		user, exists := GetCurrentUser(c)
		if exists {
			m.logger.Info("User activity",
				zap.String("username", user.Username),
				zap.String("method", c.Request.Method),
				zap.String("path", logredact.SecretPath(c.Request.URL.Path)),
				zap.Int("status", c.Writer.Status()),
				zap.String("ip", c.ClientIP()),
				zap.String("user_agent", c.Request.UserAgent()),
			)
		}
	}
}

// RateLimitByUser implements per-user rate limiting with sliding window algorithm
func (m *AuthMiddleware) RateLimitByUser(requests int, window string) gin.HandlerFunc {
	// Parse window duration
	windowDuration, err := time.ParseDuration(window)
	if err != nil {
		m.logger.Error("Invalid rate limit window", zap.String("window", window), zap.Error(err))
		windowDuration = time.Minute // Default to 1 minute
	}

	limiter := newSlidingWindowLimiter(requests, windowDuration)
	m.limiter = limiter // exposed for tests (introspection of retained keys)

	return func(c *gin.Context) {
		// Get client identifier (IP address for unauthenticated, user ID for authenticated)
		var key string
		user, exists := GetCurrentUser(c)
		if exists {
			key = fmt.Sprintf("ratelimit:user:%d", user.ID)
		} else {
			// Use IP address for unauthenticated requests
			clientIP := c.ClientIP()
			if clientIP == "" {
				clientIP = "unknown"
			}
			key = fmt.Sprintf("ratelimit:ip:%s", clientIP)
		}

		allowed, current := limiter.allow(key, time.Now())
		if !allowed {
			logFields := []zap.Field{
				zap.String("key", key),
				zap.Int("requests", current),
				zap.Int("limit", requests),
				zap.String("window", window),
			}

			if exists {
				logFields = append(logFields,
					zap.String("username", user.Username),
					zap.Int64("user_id", user.ID),
				)
			}

			m.logger.Warn("Rate limit exceeded", logFields...)
			c.Header("Retry-After", strconv.FormatInt(int64(math.Max(1, math.Ceil(windowDuration.Seconds()))), 10))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded. Please try again later.",
				"retry_after": windowDuration.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
