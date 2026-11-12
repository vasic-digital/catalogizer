package handlers

// CAT-PXF23-R5B: helpers that keep the text of internal errors (driver text, wrapped %w chains, binding internals) off
// the wire. Every helper logs the full error server-side and sends a fixed or authored message.

import (
	"errors"
	"log"
	"net/http"
	"regexp"

	"catalogizer/internal/smartrules"

	"github.com/gin-gonic/gin"
)

// respondInternal logs err server-side and answers status with the fixed message msg.
func respondInternal(c *gin.Context, status int, msg string, err error) {
	log.Printf("handlers: %s %s: %s (not returned to client): %v", c.Request.Method, c.FullPath(), msg, err)
	c.JSON(status, gin.H{"error": msg})
}

// respondBadBody answers 400 for a request body that could not be bound; the binder text names Go structs and
// fields, so only a fixed message goes out.
func respondBadBody(c *gin.Context, err error) {
	respondInternal(c, http.StatusBadRequest, "Invalid request body", err)
}

// stdError is respondInternal for net/http handlers: the status text (or msg) goes out, err stays in the log.
func stdError(w http.ResponseWriter, status int, err error) {
	log.Printf("handlers: http %d (error text not returned to client): %v", status, err)
	http.Error(w, http.StatusText(status), status)
}

// validationText returns the authored text of the smartrules.ValidationError inside err's chain (never the text of
// the wrapping chain), or a fixed phrase when there is none.
func validationText(err error) string {
	var v *smartrules.ValidationError
	if errors.As(err, &v) {
		return v.Msg
	}
	return "invalid request"
}

// sentinelText returns the text of the first sentinel that err wraps (the sentinel's own authored text, without the
// wrapping chain), or fallback when none matches.
func sentinelText(err error, fallback string, sentinels ...error) string {
	for _, s := range sentinels {
		if errors.Is(err, s) {
			return s.Error()
		}
	}
	return fallback
}

var paramRequiredRE = regexp.MustCompile(`\b[a-z][a-z0-9_]* parameter required\b`)

// paramRequiredText extracts the authored "<key> parameter required" phrase from a service error; the rest of the
// chain is dropped.
func paramRequiredText(err error) string {
	if m := paramRequiredRE.FindString(err.Error()); m != "" {
		return m
	}
	return "required parameter missing"
}

// syncDetails is the fixed class text of the "details" field of the sync API (it used to carry err.Error()).
func syncDetails(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "invalid input"
	case http.StatusUnauthorized, http.StatusForbidden:
		return "not permitted"
	case http.StatusNotFound:
		return "not found"
	case http.StatusConflict:
		return "endpoint is not active"
	case http.StatusBadGateway:
		return "connection test failed"
	}
	return "internal error"
}

// syncFail logs err and answers status with the fixed message and class text.
func syncFail(c *gin.Context, status int, msg string, err error) {
	log.Printf("handlers: sync %s %s: %s (not returned to client): %v", c.Request.Method, c.FullPath(), msg, err)
	c.JSON(status, gin.H{"success": false, "error": msg, "details": syncDetails(status)})
}
