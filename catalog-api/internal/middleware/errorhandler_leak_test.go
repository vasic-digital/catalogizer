package middleware

// CAT-WP02-ERRLEAK: ErrorHandler must not put the text of an error (binder internals, driver text) on the wire.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

const errLeakMarker = "pq: connection refused host=db.internal:5432 /var/lib/secret/path"

func TestErrorHandler_NeverEchoesErrorText(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  gin.ErrorType
		want int
	}{
		{"bind", gin.ErrorTypeBind, http.StatusBadRequest},
		{"public", gin.ErrorTypePublic, http.StatusInternalServerError},
		{"private", gin.ErrorTypePrivate, http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			r := gin.New()
			r.Use(ErrorHandler())
			r.GET("/t", func(c *gin.Context) {
				_ = c.Error(&gin.Error{Err: errors.New(errLeakMarker), Type: tc.typ})
			})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/t", nil))
			assert.Equal(t, tc.want, w.Code)
			assert.NotContains(t, w.Body.String(), "connection refused")
			assert.NotContains(t, w.Body.String(), "/var/lib/secret")
			assert.NotContains(t, w.Body.String(), "db.internal")
		})
	}
}
