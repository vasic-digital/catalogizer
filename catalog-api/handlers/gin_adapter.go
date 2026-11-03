package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
)

// WrapHTTPHandler adapts a standard net/http handler to a gin.HandlerFunc.
// It propagates Gin context values (user_id, role, etc.) into the request's
// context so that net/http handlers can access them via r.Context().Value().
// The user_id is converted from string (JWT subject) to int for handler compatibility.
func WrapHTTPHandler(handler func(http.ResponseWriter, *http.Request)) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := c.Request
		// Propagate user_id: convert string (JWT subject) to int for handler compatibility
		if userID, exists := c.Get("user_id"); exists {
			switch v := userID.(type) {
			case string:
				if id, err := strconv.Atoi(v); err == nil {
					req = req.WithContext(context.WithValue(req.Context(), "user_id", id))
				}
			case int:
				req = req.WithContext(context.WithValue(req.Context(), "user_id", v))
			}
		}
		if role, exists := c.Get("role"); exists {
			req = req.WithContext(context.WithValue(req.Context(), "role", role))
		}
		if username, exists := c.Get("username"); exists {
			req = req.WithContext(context.WithValue(req.Context(), "username", username))
		}
		// Copy gin path params (:id, :token, :step_id, ...) into gorilla/mux
		// vars so handlers written against mux.Vars(r) see them.
		if len(c.Params) > 0 {
			vars := make(map[string]string, len(c.Params))
			for _, p := range c.Params {
				vars[p.Key] = p.Value
			}
			req = mux.SetURLVars(req, vars)
		}
		handler(c.Writer, req)
	}
}
