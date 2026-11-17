// Package netguardtest builds the real loopback topology used to prove that an outbound HTTP client refuses a
// redirect into a blocked address class (AZ2-1). It is imported by tests only (a guard test enforces that).
//
// "front" listens on 127.0.0.1 and plays the third-party provider host. "sink" is a second real listener whose
// hits are counted. The redirect targets the sink through 0.0.0.0, an address class netguard refuses whatever the
// operator allow-list says, and which Linux connects to the local machine, so a client that follows the redirect
// provably reaches the sink.
package netguardtest

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Topology is the pair of real listeners.
type Topology struct {
	FrontURL string // http://127.0.0.1:port : answers /ok with 200 and the /to-* paths with 302
	SinkURL  string // http://127.0.0.1:port : the sink reached directly (control needle for the counter)
	hits     atomic.Int32
}

// SinkHits is the number of requests the sink has served.
func (t *Topology) SinkHits() int { return int(t.hits.Load()) }

// New starts both listeners and registers their cleanup.
func New(t testing.TB) *Topology {
	t.Helper()
	top := &Topology{}
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		top.hits.Add(1)
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("SINK-SECRET"))
	}))
	t.Cleanup(sink.Close)
	top.SinkURL = sink.URL
	_, port, err := net.SplitHostPort(sink.Listener.Addr().String())
	if err != nil {
		t.Fatalf("sink addr: %v", err)
	}
	front := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			_, _ = w.Write([]byte("ok"))
		case "/to-unspecified":
			http.Redirect(w, r, fmt.Sprintf("http://0.0.0.0:%s/secret", port), http.StatusFound)
		case "/to-userinfo":
			http.Redirect(w, r, fmt.Sprintf("http://user:pw@127.0.0.1:%s/secret", port), http.StatusFound)
		case "/to-gopher":
			http.Redirect(w, r, "gopher://127.0.0.1:1/x", http.StatusFound)
		case "/loop":
			http.Redirect(w, r, "/loop", http.StatusFound)
		case "/big":
			w.Header().Set("Content-Type", "application/octet-stream")
			buf := make([]byte, 1<<20)
			for i := 0; i < 200; i++ {
				if _, err := w.Write(buf); err != nil {
					return
				}
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(front.Close)
	top.FrontURL = front.URL
	return top
}
