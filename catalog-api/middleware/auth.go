package middleware

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogizer/internal/streamticket"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTMiddleware handles JWT authentication
type JWTMiddleware struct {
	secretKey []byte
	// sessionCheck, when set, is consulted on every authenticated request to
	// confirm the session the token was minted for is still active. It makes
	// server-side revocation (admin-reset-password, logout, deactivation)
	// effective for stateless access tokens. See SetSessionChecker.
	sessionCheck SessionChecker
	// adminCheck, when set, decides RequireAdmin from the database (the
	// user's CURRENT role permissions) instead of the role_id claim baked into
	// the access token. See SetAdminChecker.
	adminCheck AdminChecker
	// permCheck, when set, decides RequirePermission from the database (the
	// user's CURRENT role permissions). See SetPermissionChecker.
	permCheck PermissionChecker
	// tickets, when set, lets the stream route authenticate a short-lived stream ticket (?ticket=) instead of a session token.
	tickets *streamticket.Signer
}

// SetStreamTicketSigner enables ?ticket= authentication on GET/HEAD /api/v1/stream/:id (and nowhere else).
func (m *JWTMiddleware) SetStreamTicketSigner(s *streamticket.Signer) { m.tickets = s }

// streamRoute reports whether the request is a GET/HEAD of the stream route and returns the media id of its path.
func streamRoute(c *gin.Context) (int64, bool) {
	if (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) || c.FullPath() != "/api/v1/stream/:id" {
		return 0, false
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	return id, err == nil
}

// PermissionChecker reports whether the user identified by userID (the token
// subject) is, right now, an active user entitled to permission (a role that
// carries it, or system administration). A non-nil error means "could not be
// determined"; RequirePermission fails closed on it.
type PermissionChecker func(ctx context.Context, userID int, permission string) (bool, error)

// SetPermissionChecker installs the live permission check. Without it
// RequirePermission falls back to the token's role_id claim equal to
// RoleAdminID (administrators only: it never grants a non-admin).
func (m *JWTMiddleware) SetPermissionChecker(c PermissionChecker) { m.permCheck = c }

// AdminChecker reports whether the user identified by userID (the token
// subject) is, right now, an active user whose role carries system
// administration rights. A non-nil error means "could not be determined";
// RequireAdmin fails closed on it.
type AdminChecker func(ctx context.Context, userID int) (bool, error)

// SetAdminChecker installs the live admin check. Without it RequireAdmin falls
// back to the token's role_id claim equal to RoleAdminID (legacy behaviour kept
// for stacks that do not wire the database); production wires it in main.go.
func (m *JWTMiddleware) SetAdminChecker(c AdminChecker) { m.adminCheck = c }

// SessionChecker reports whether the session identified by sessionID (the
// "session_id" claim) is still active and belongs to userID (the token
// subject). A non-nil error means "not active OR could not be determined";
// RequireAuth fails closed on any error.
type SessionChecker func(ctx context.Context, sessionID, userID string) error

// SetSessionChecker installs the revocation check. Tokens issued by the auth
// service always carry a session_id; once a checker is installed a token
// without one is rejected (fail closed).
func (m *JWTMiddleware) SetSessionChecker(c SessionChecker) { m.sessionCheck = c }

// Claims represents JWT claims.
// RoleID is populated by the auth service (services/auth_service.go) at
// login time. The convention seeded by main.go::seedDefaultAdmin is
// role_id == 1 == admin, role_id == 2 == default user role. RoleID is
// read here so RequireAdmin() can gate the /api/v1/admin/* group
// without bouncing through the heavier internal/auth middleware.
type Claims struct {
	Username  string `json:"username"`
	RoleID    int    `json:"role_id"`
	SessionID string `json:"session_id,omitempty"`
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
		// Stream ticket: valid ONLY for GET/HEAD of /api/v1/stream/:id and ONLY for the media id it was minted for. On every other
		// route the "ticket" parameter is ignored (the request then fails like any unauthenticated one).
		if tk := c.Query("ticket"); tk != "" && authHeader == "" {
			if mediaID, ok := streamRoute(c); ok && m.tickets != nil {
				cl, err := m.tickets.Verify(tk, mediaID)
				if err == nil && m.sessionCheck != nil && (cl.SessionID == "" || m.sessionCheck(c.Request.Context(), cl.SessionID, cl.UserID) != nil) {
					err = streamticket.ErrInvalid
				}
				if err != nil {
					utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid or expired stream ticket", nil)
					c.Abort()
					return
				}
				c.Set("username", cl.Username)
				c.Set("session_id", cl.SessionID)
				if uid, err := strconv.Atoi(cl.UserID); err == nil {
					c.Set("user_id", uid)
				} else {
					c.Set("user_id", cl.UserID)
				}
				c.Set("auth_via_stream_ticket", true)
				c.Next()
				return
			}
		}
		if authHeader != "" {
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid authorization header format", nil)
				c.Abort()
				return
			}
			tokenString = parts[1]
		} else {
			// Native-client LEGACY path (libVLC, ffmpeg, VLC iOS cannot be taught the ticket flow): the session token in the query
			// string. Web clients MUST use /api/v1/stream/:id/ticket instead; the value is redacted from the API's own gin/zap access logs (a reverse proxy in front must redact too: catalog-web/nginx.conf.template does for its access log; proxy error logs are not covered). Follow-up:
			// migrate native clients to tickets and delete this fallback (docs/FOLLOW_UPS_NEXT_PHASE.md).
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

		if m.sessionCheck != nil {
			if claims.SessionID == "" {
				utils.SendErrorResponse(c, http.StatusUnauthorized, "Invalid token", nil)
				c.Abort()
				return
			}
			if err := m.sessionCheck(c.Request.Context(), claims.SessionID, claims.Subject); err != nil {
				utils.SendErrorResponse(c, http.StatusUnauthorized, "Session expired or revoked", nil)
				c.Abort()
				return
			}
		}

		// Set user info in context — convert user_id to int for handler compatibility
		c.Set("username", claims.Username)
		c.Set("role_id", claims.RoleID)
		c.Set("session_id", claims.SessionID)
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
// caller to be an administrator. With an AdminChecker installed (production)
// the decision is made from the database on every request using the caller's
// current role permissions; without one it requires the token's role_id claim
// to equal RoleAdminID. Article XI §11.5: prior to
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
		if m.adminCheck != nil {
			uid, ok := c.Get("user_id")
			userID, isInt := uid.(int)
			if !ok || !isInt {
				utils.SendErrorResponse(c, http.StatusForbidden, "Admin role required", nil)
				c.Abort()
				return
			}
			admin, err := m.adminCheck(c.Request.Context(), userID)
			if err != nil {
				utils.SendErrorResponse(c, http.StatusServiceUnavailable, "Could not verify admin role", nil)
				c.Abort()
				return
			}
			if !admin {
				utils.SendErrorResponse(c, http.StatusForbidden, "Admin role required", nil)
				c.Abort()
				return
			}
			c.Next()
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

// RequirePermission returns a middleware that requires the authenticated
// caller to hold permission (or be an administrator). Like RequireAdmin the
// decision is made from the database on every request when a PermissionChecker
// is installed, so a stale role_id claim never grants; without one it admits
// only the token's role_id == RoleAdminID (deny by default for everyone else).
// Responds 401 when unauthenticated, 403 when refused, 503 when the check
// could not be made (fail closed).
//
// MUST be installed AFTER RequireAuth.
func (m *JWTMiddleware) RequirePermission(permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		roleVal, exists := c.Get("role_id")
		if !exists {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
			c.Abort()
			return
		}
		if m.permCheck != nil {
			uid, ok := c.Get("user_id")
			userID, isInt := uid.(int)
			if !ok || !isInt {
				utils.SendErrorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
				c.Abort()
				return
			}
			allowed, err := m.permCheck(c.Request.Context(), userID, permission)
			if err != nil {
				utils.SendErrorResponse(c, http.StatusServiceUnavailable, "Could not verify permissions", nil)
				c.Abort()
				return
			}
			if !allowed {
				utils.SendErrorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
				c.Abort()
				return
			}
			c.Next()
			return
		}
		roleID, ok := roleVal.(int)
		if !ok || roleID != RoleAdminID {
			utils.SendErrorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireSelfOrAdmin admits the caller when the URL parameter param names their
// own user id, or when they are an administrator (decided like RequireAdmin). Any
// other caller gets 403 so that /analytics/user/:id style routes cannot be used to
// read another account's data (review round 1, F5). MUST be installed AFTER RequireAuth.
func (m *JWTMiddleware) RequireSelfOrAdmin(param string) gin.HandlerFunc {
	admin := m.RequireAdmin()
	return func(c *gin.Context) {
		if _, exists := c.Get("role_id"); !exists {
			utils.SendErrorResponse(c, http.StatusUnauthorized, "Authentication required", nil)
			c.Abort()
			return
		}
		if uid, ok := c.Get("user_id"); ok {
			if userID, isInt := uid.(int); isInt && c.Param(param) == strconv.Itoa(userID) {
				c.Next()
				return
			}
		}
		admin(c)
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
