package main

import (
	"github.com/gin-gonic/gin"

	root_handlers "catalogizer/handlers"
	root_middleware "catalogizer/middleware"
)

// registerWebSocketRoute mounts the real-time WebSocket endpoint. main.go calls
// it (TestMainGoMountsWebSocketThroughHelper pins that). The route sits behind the
// same RequireAuth as /api/v1 (Bearer header or a single-use ?ticket=, session check included; the session JWT in the query is no longer accepted), and the handler enforces an Origin policy.
func registerWebSocketRoute(router gin.IRoutes, mw *root_middleware.JWTMiddleware, h *root_handlers.WebSocketHandler) {
	// Post-upgrade authorisation (SEC-WS-1 F3): the connection is re-validated with
	// the very checks RequireAuth / RequireAdmin use, so revocation, suspension,
	// lock and token expiry close the socket and admin-only events stay admin-only.
	h.SetAuthChecks(root_handlers.WSSessionCheck(mw.SessionCheck()), root_handlers.WSAdminCheck(mw.AdminCheck()))
	// SEC-WS-1 F2: /ws takes "Authorization: Bearer" or a single-use ?ticket= only; a session JWT in the query is refused.
	// The ticket is minted by POST /api/v1/ws/ticket (Authorization header only), rate limited per user and capped per user (429 + Retry-After).
	router.POST("/api/v1/ws/ticket", mw.RequireBearerHeader(), mw.WSTicketMintLimit(), mw.MintWSTicket)
	router.GET("/ws", mw.RequireWSAuth(), h.HandleConnection)
}
