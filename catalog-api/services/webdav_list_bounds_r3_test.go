package services

// UNIT-CLASS (§11.4.289): the servers below are local httptest fixtures. They prove
// ListFiles' own bounding and error-propagation logic, not that a real WebDAV server works.

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// flatWebDAV answers PROPFIND / with `files` plain files (no sub-collections), so a single
// request reaches the entry budget and neither the depth nor the request budget can stop it.
func flatWebDAV(files int, propfinds *int64) *httptest.Server {
	var body strings.Builder
	body.WriteString(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">`)
	body.WriteString(`<d:response><d:href>/</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`)
	for i := 0; i < files; i++ {
		fmt.Fprintf(&body, `<d:response><d:href>/f%d</d:href><d:propstat><d:prop><d:resourcetype/></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`, i)
	}
	body.WriteString(`</d:multistatus>`)
	payload := []byte(body.String())
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt64(propfinds, 1)
		if r.Method != "PROPFIND" || r.URL.Path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(207)
		_, _ = w.Write(payload)
	}))
}

// T2 (review round 3): the entry cap on its own. Exactly maxListEntries entries list
// completely; one more is an ErrWebDAVListLimit error naming the entry bound, after ONE request.
func TestWebDAVListFiles_EntryCapAlone(t *testing.T) {
	var n int64
	at := flatWebDAV(maxListEntries, &n)
	defer at.Close()
	files, err := NewWebDAVClient(at.URL, "", "").ListFiles("/")
	if err != nil {
		t.Fatalf("control: %d entries (the cap) must list: %v", maxListEntries, err)
	}
	if len(files) != maxListEntries {
		t.Fatalf("control: got %d entries, want %d", len(files), maxListEntries)
	}

	n = 0
	over := flatWebDAV(maxListEntries+1, &n)
	defer over.Close()
	files, err = NewWebDAVClient(over.URL, "", "").ListFiles("/")
	if err == nil {
		t.Fatalf("%d entries listed without error (%d returned)", maxListEntries+1, len(files))
	}
	if !errors.Is(err, ErrWebDAVListLimit) || !strings.Contains(err.Error(), "entries") {
		t.Fatalf("want the entry bound, got %v", err)
	}
	if files != nil {
		t.Fatalf("a partial listing (%d entries) was returned with the error", len(files))
	}
	// far below maxListRequests and only one collection exists: the request and depth bounds
	// cannot be what stopped it (the client may send more than one PROPFIND per ReadDir)
	if got := atomic.LoadInt64(&n); got > 4 {
		t.Fatalf("issued %d PROPFINDs for one flat collection", got)
	}
}

// T3 (review round 3): a sub-collection that cannot be listed fails the WHOLE listing, and the
// error is the listing error, not ErrWebDAVListLimit (so only limit errors are not the sole
// errors propagated).
func TestWebDAVListFiles_SubCollectionFailureFailsListing(t *testing.T) {
	for _, status := range []int{http.StatusNotFound, http.StatusInternalServerError} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				col := func(h string) string {
					return `<d:response><d:href>` + h + `</d:href><d:propstat><d:prop><d:resourcetype><d:collection/></d:resourcetype></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
				}
				file := func(h string) string {
					return `<d:response><d:href>` + h + `</d:href><d:propstat><d:prop><d:resourcetype/></d:prop><d:status>HTTP/1.1 200 OK</d:status></d:propstat></d:response>`
				}
				var body string
				switch r.URL.Path {
				case "/":
					body = col("/") + col("/good/") + col("/bad/") + file("/top.txt")
				case "/good/":
					body = col("/good/") + file("/good/g.txt")
				default: // /bad/
					w.WriteHeader(status)
					return
				}
				w.WriteHeader(207)
				_, _ = w.Write([]byte(`<?xml version="1.0"?><d:multistatus xmlns:d="DAV:">` + body + `</d:multistatus>`))
			}))
			defer srv.Close()
			files, err := NewWebDAVClient(srv.URL, "", "").ListFiles("/")
			if err == nil {
				t.Fatalf("listing with an unreadable sub-collection succeeded: %d entries", len(files))
			}
			if errors.Is(err, ErrWebDAVListLimit) {
				t.Fatalf("a sub-listing failure must not be reported as a limit: %v", err)
			}
			if files != nil {
				t.Fatalf("partial listing returned: %d entries", len(files))
			}
		})
	}
}
