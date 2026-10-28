package services

// UNIT-CLASS (§11.4.289): the server below is a local httptest fixture that imitates a
// hostile WebDAV endpoint. It proves ListFiles' own bounding logic, not that any real WebDAV
// server works with Catalogizer.

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// hostileWebDAV answers every PROPFIND with the requested collection plus `fanout`
// sub-collections while the path has fewer than `maxDepth` segments (maxDepth <= 0 means
// unbounded), so the tree is exponential in breadth and, optionally, infinite in depth.
func hostileWebDAV(t *testing.T, fanout, maxDepth int, propfinds *int64) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PROPFIND" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		atomic.AddInt64(propfinds, 1)
		self := r.URL.Path
		if !strings.HasSuffix(self, "/") {
			self += "/"
		}
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">`)
		entry := func(href string) {
			fmt.Fprintf(&b, `<d:response><d:href>%s</d:href><d:propstat><d:prop><d:displayname>x</d:displayname><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, href)
		}
		entry(self)
		n := fanout // per request: never mutate the captured parameter
		if maxDepth > 0 && strings.Count(strings.Trim(self, "/"), "/")+1 >= maxDepth {
			n = 0
		}
		for i := 0; i < n; i++ {
			entry(fmt.Sprintf("%sd%d/", self, i))
		}
		b.WriteString(`</d:multistatus>`)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = w.Write([]byte(b.String()))
	}))
}

// S2 (review round 2): breadth explosion must stop at the request/entry cap with an ERROR.
func TestWebDAVListFiles_BreadthExplosionReturnsError(t *testing.T) {
	var n int64
	// 20 levels of fan-out 2: about 2^21 collections, shallower than maxListDepth, so only
	// the request/entry budget can stop it (a depth-only bound would run for days).
	srv := hostileWebDAV(t, 2, 20, &n)
	defer srv.Close()
	c := NewWebDAVClient(srv.URL, "", "")
	start := time.Now()
	files, err := c.ListFiles("/")
	el := time.Since(start)
	if err == nil {
		t.Fatalf("hostile tree listed without error: %d entries, %d PROPFINDs", len(files), n)
	}
	if !errors.Is(err, ErrWebDAVListLimit) {
		t.Fatalf("want ErrWebDAVListLimit, got %v", err)
	}
	if got := atomic.LoadInt64(&n); got > int64(maxListRequests)+1 {
		t.Fatalf("issued %d PROPFINDs, cap is %d", got, maxListRequests)
	}
	if !strings.Contains(err.Error(), "PROPFIND requests") && !strings.Contains(err.Error(), "entries") {
		t.Fatalf("stopped by the wrong bound: %v", err)
	}
	if el > 30*time.Second {
		t.Fatalf("took %s", el)
	}
}

// Depth truncation must also be an error, never a silent partial listing.
func TestWebDAVListFiles_DepthLimitReturnsError(t *testing.T) {
	var n int64
	srv := hostileWebDAV(t, 1, 0, &n) // a single infinite chain: only depth can stop it
	defer srv.Close()
	_, err := NewWebDAVClient(srv.URL, "", "").ListFiles("/")
	if !errors.Is(err, ErrWebDAVListLimit) || !strings.Contains(err.Error(), "depth") {
		t.Fatalf("want ErrWebDAVListLimit at depth %d, got %v (PROPFINDs=%d)", maxListDepth, err, n)
	}
}

// Control: a small finite tree lists completely with no error.
func TestWebDAVListFiles_FiniteTreeLists(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := `<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">`
		col := func(h string) string {
			return `<d:response><d:href>` + h + `</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
		}
		file := func(h string) string {
			return `<d:response><d:href>` + h + `</d:href><d:propstat><d:prop><d:resourcetype/><d:getcontentlength>3</d:getcontentlength></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
		}
		switch r.URL.Path {
		case "/":
			body += col("/") + col("/a/") + file("/f.txt")
		case "/a/":
			body += col("/a/") + file("/a/g.txt")
		default:
			w.WriteHeader(404)
			return
		}
		body += `</d:multistatus>`
		w.WriteHeader(207)
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	files, err := NewWebDAVClient(srv.URL, "", "").ListFiles("/")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 {
		t.Fatalf("want 3 entries, got %d", len(files))
	}
}
