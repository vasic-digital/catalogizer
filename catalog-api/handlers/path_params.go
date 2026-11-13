package handlers

import (
	"net/http"
	"strings"

	"catalogizer/internal/logredact"

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

// requestLogPath returns the request path for a log field with secret segments (a share token, invite code ...)
// redacted. Together with routedID this is the only place in package handlers that reads the raw URL path (the
// TestNoHardcodedAPIPrefixPathParsing sweep keeps every other handler file free of it); it parses no ids.
func requestLogPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return logredact.SecretPath(r.URL.Path)
}

// requestKnownSecrets returns the credential values the request carries (path secrets, sensitive route params and
// query values, credential headers) so they can be removed from any text logged for it.
func requestKnownSecrets(r *http.Request) []string {
	if r == nil || r.URL == nil {
		return nil
	}
	return logredact.KnownSecrets(r.URL.Path, r.URL.RawQuery, r.Header, mux.Vars(r))
}
