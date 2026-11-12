package main

import (
	"github.com/gin-gonic/gin"

	root_handlers "catalogizer/handlers"
	root_middleware "catalogizer/middleware"
)

// registerWebSocketRoute mounts the real-time WebSocket endpoint. main.go calls
// it (TestMainGoMountsWebSocketThroughHelper pins that). The route sits behind the
// same RequireAuth as /api/v1 (Bearer header or the ?token= fallback the web and
// Android clients use, session check included), and the handler enforces an Origin policy.
func registerWebSocketRoute(router gin.IRoutes, mw *root_middleware.JWTMiddleware, h *root_handlers.WebSocketHandler) {
	router.GET("/ws", mw.RequireAuth(), h.HandleConnection)
}
