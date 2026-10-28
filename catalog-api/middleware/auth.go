package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware handles JWT authentication
type JWTMiddleware struct {
	secretKey []byte
}

// Claims represents JWT claims.
// RoleID is populated by the auth service (services/auth_service.go) at
// login time. The convention seeded by main.go::seedDefaultAdmin is
// role_id == 1 == admin, role_id == 2 == default user role. RoleID is
// read here so RequireAdmin() can gate the /api/v1/admin/* group
// without bouncing through the heavier internal/auth middleware.
type Claims struct {
	Username string `json:"username"`
	RoleID   int    `json:"role_id"`
	jwt.RegisteredClaims
}

// NewJWTMiddleware creates a new JWT middleware
func NewJWTMiddleware(secretKey string) *JWTMiddleware {
	return &JWTMiddleware{
		secretKey: []byte(secretKey),
	}
}

// RequireAuth returns a middleware that requires valid JWT authentication.
// Accepts the token either as a standard "Authorization: Bearer <token>"
// header OR as an "access_token" / "token" query parameter. The query
// parameter path exists so native media players (libVLC, ffmpeg, VLC
// iOS, etc.) that cannot easily inject custom headers into their HTTP
// data source can still reach the authenticated /api/v1/stream/:id
// endpoint by appending "?access_token=<jwt>" to the URL.
func (m *JWTMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var tokenString string

		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid authorization header format", nil)
				c.Abort()
				return
			}
			tokenString = parts[1]
		} else {
			// Fallback to query parameter for native clients.
			tokenString = c.Query("access_token")
			if tokenString == "" {
				tokenString = c.Query("token")
			}
			if tokenString == "" {
				utils.SendErrorResponse(c, http.StatusUnauthorized, "Authorization header required", nil)
				c.Abort()
				return
			}
		}

		// Parse and validate token
		claims := &Claims{}
		token, err := m.parseToken(tokenString, claims)

		if err != nil {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid token", err)
			c.Abort()
			return
		}

		if !token.Valid {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid token", nil)
			c.Abort()
			return
		}

		// Set user info in context — convert user_id to int for handler compatibility
		c.Set("username", claims.Username)
		c.Set("role_id", claims.RoleID)
		if uid, err := strconv.Atoi(claims.Subject); err == nil {
			c.Set("user_id", uid)
		} else {
			c.Set("user_id", claims.Subject)
		}

		c.Next()
	}
}

// RoleAdminID is the seeded admin role ID. Mirrors
// services/auth_service.go and main.go::seedDefaultAdmin.
const RoleAdminID = 1

// RequireAdmin returns a middleware that requires the authenticated
// caller to carry role_id == RoleAdminID. Article XI §11.5: prior to
// 2026-04-29 the /api/v1/admin/* group had no role gate at all — any
// authenticated user could call /admin/system-info, /admin/users,
// /admin/storage, /admin/backups, etc. Caught by FQA-API-010 in the
// real-binary bank verification, which expected a 403 and got 200.
//
// MUST be installed AFTER RequireAuth — relies on the role_id context
// key set by RequireAuth.
func (m *JWTMiddleware) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role_id")
		if !exists {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
			c.Abort()
			return
		}
		roleID, ok := roleVal.(int)
		if !ok || roleID != RoleAdminID {
			utils.SendErrorResponse(c, http.StatusForbidden, "Admin role required", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// GenerateToken generates a new JWT token
func (m *JWTMiddleware) GenerateToken(username, userID string, expirationHours int) (string, error) {
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expirationHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "catalog-api",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// parseToken parses and verifies tokenString into claims, pinning the signing
// algorithm to HS256 (the only one GenerateToken issues). Pinning rejects
// alg=none, asymmetric (RS*/ES*) tokens used for key-confusion and the other
// HMAC variants (HS384/HS512) before the key is ever handed out.
func (m *JWTMiddleware) parseToken(tokenString string, claims *Claims) (*jwt.Token, error) {
	return jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Header["alg"])
		}
		return m.secretKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}

// ValidateToken validates a JWT token and returns claims
func (m *JWTMiddleware) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := m.parseToken(tokenString, claims)

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, jwt.ErrTokenNotValidYet
	}

	return claims, nil
}
