package handlers

import (
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// routedID returns the resource id of the current request. It prefers the
// routed path parameter "id" (set by WrapHTTPHandler / gorilla mux). When the
// handler is invoked without a router (direct calls), it falls back to the path
// segment that follows the resource collection name (e.g. "users"), which works
// for any prefix such as /api/users/5 or /api/v1/users/5/lock.
func routedID(r *http.Request, resource string) string {
	if v := mux.Vars(r)["id"]; v != "" {
		return v
	}
	segs := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	for i, s := range segs {
		if s == resource && i+1 < len(segs) {
			return segs[i+1]
		}
	}
	return ""
}
