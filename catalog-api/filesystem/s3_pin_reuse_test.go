package filesystem

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// CAT-SECQ1-A: the operator transport (and with it the address-class pin of its first connection) is reused by later
// Connects of the same client; the endpoint is a real local HTTP server, allow-listed loopback.
func TestS3ConnectReusesOperatorTransport(t *testing.T) {
	t.Setenv("CATALOGIZER_OUTBOUND_ALLOW_CIDRS", "127.0.0.0/8")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer srv.Close()
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Bucket: "b", AccessKey: "a", SecretKey: "s", PathStyle: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = c.Connect(ctx) // HeadBucket answers 403: the connect fails after the transport was built
	first := c.opTransport
	if first == nil {
		t.Fatal("no operator transport after Connect")
	}
	_ = c.Connect(ctx)
	if c.opTransport != first {
		t.Fatal("second Connect built a new transport (the first-connection pin would be reset)")
	}
}
