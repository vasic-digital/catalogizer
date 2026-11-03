//go:build live

// LIVE WebDAV integration test for the REAL catalog-api WebDAVClient against a real WebDAV server (e.g. a Synology NAS
// with the WebDAV Server package enabled). Constitution §11.4.289: no stubs, mocks or fixture servers.
//
// Environment (names only; values are never logged):
//
//	LIVE_WEBDAV_URL       base URL of the WebDAV service, e.g. https://nas:5006   (absent => SKIP(credentials_absent))
//	LIVE_WEBDAV_USER, LIVE_WEBDAV_PASSWORD
//	LIVE_WEBDAV_SANDBOX   collection under the URL in which the test may write, e.g. /work20/Testing  (required)
//	LIVE_WEBDAV_INSECURE  "1" accepts a self-signed certificate (the NAS default)
//
// A dial/TLS failure or a non-WebDAV answer (405, or 200 without a DAV header) is an honest SKIP(service_unavailable), never a stand-in; a WebDAV server that
// rejects the credential or lacks the sandbox (401/403/404) FAILs. The test creates ONE uniquely named
// file inside the sandbox, reads it back, compares sha256, deletes only that file, and never touches anything outside the sandbox.
package filesystem

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

type liveDAV struct{ url, user, pass, sandbox string }

func loadLiveDAV(t *testing.T) liveDAV {
	t.Helper()
	d := liveDAV{os.Getenv("LIVE_WEBDAV_URL"), os.Getenv("LIVE_WEBDAV_USER"), os.Getenv("LIVE_WEBDAV_PASSWORD"), os.Getenv("LIVE_WEBDAV_SANDBOX")}
	if d.url == "" || d.user == "" || d.pass == "" || d.sandbox == "" {
		t.Skip("SKIP(credentials_absent): LIVE_WEBDAV_URL / _USER / _PASSWORD / _SANDBOX not set")
	}
	liveSecrets = append(liveSecrets, d.user, d.pass)
	if i := strings.Index(d.url, "://"); i >= 0 {
		h := d.url[i+3:]
		liveSecrets = append(liveSecrets, strings.SplitN(strings.SplitN(h, "/", 2)[0], ":", 2)[0])
	}
	sb, err := validateSandbox(d.sandbox)
	if err != nil {
		t.Fatalf("LIVE_WEBDAV_SANDBOX invalid: %v", err)
	}
	d.sandbox = sb
	return d
}

func (d liveDAV) newClient(t *testing.T, pass string) *WebDAVClient {
	t.Helper()
	c, err := NewWebDAVClient(&WebDAVConfig{URL: d.url, Username: d.user, Password: pass, Path: ""})
	if err != nil {
		t.Fatalf("new client: %s", rd(err))
	}
	if os.Getenv("LIVE_WEBDAV_INSECURE") == "1" {
		c.client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // #nosec G402 -- NAS self-signed cert, opt-in
	}
	return c
}

// requireService SKIPs (honestly) when the server does not speak WebDAV: PROPFIND must answer 207 with the right credential.
func (d liveDAV) requireService(t *testing.T) {
	t.Helper()
	c := d.newClient(t, d.pass)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "PROPFIND", strings.TrimRight(d.url, "/")+(&url.URL{Path: d.sandbox}).EscapedPath(), nil)
	req.SetBasicAuth(d.user, d.pass)
	req.Header.Set("Depth", "0")
	resp, err := c.client.Do(req)
	if err != nil {
		t.Skipf("SKIP(service_unavailable): WebDAV endpoint unreachable (%s); enable the WebDAV Server package on the NAS (DSM > Package Center > WebDAV Server, ports 5005/5006)", rd(err))
	}
	defer resp.Body.Close()
	switch classifyProbe(resp.StatusCode, resp.Header.Get("DAV")) {
	case probeNotDAV:
		t.Skipf("SKIP(service_unavailable): endpoint answered PROPFIND with %d and no DAV header; it is not a WebDAV service (enable the WebDAV Server package)", resp.StatusCode)
	case probeFail:
		t.Fatalf("WebDAV endpoint is present but PROPFIND on the sandbox answered %d (401/403: credential or share rights; 404: sandbox missing)", resp.StatusCode)
	}
}

// rawStatus issues one authenticated request and returns the HTTP status, so absence is asserted as a real 404, not as "any non-200".
func (d liveDAV) rawStatus(t *testing.T, method, p string) int {
	t.Helper()
	c := d.newClient(t, d.pass)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, method, strings.TrimRight(d.url, "/")+(&url.URL{Path: p}).EscapedPath(), nil)
	req.SetBasicAuth(d.user, d.pass)
	resp, err := c.client.Do(req)
	if err != nil {
		t.Fatalf("%s: %s", method, rd(err))
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestLiveWebDAV_RoundTripAndFailurePaths(t *testing.T) {
	d := loadLiveDAV(t)
	d.requireService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	c := d.newClient(t, d.pass)
	if err := c.Connect(ctx); err != nil {
		t.Fatalf("connect: %s", rd(err))
	}
	defer c.Disconnect(ctx)
	if err := c.TestConnection(ctx); err != nil {
		t.Fatalf("test connection: %s", rd(err))
	}

	// list the sandbox
	if _, err := c.ListDirectory(ctx, d.sandbox); err != nil {
		t.Fatalf("list sandbox: %s", rd(err))
	}

	// write a uniquely named file, read it back, compare checksum
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	name := fmt.Sprintf("catalogizer-live-webdav-%d-%s.bin", time.Now().UnixNano(), hex.EncodeToString(rnd[:]))
	p := strings.TrimRight(d.sandbox, "/") + "/" + name
	payload := make([]byte, 64*1024)
	_, _ = rand.Read(payload)
	want := sha256.Sum256(payload)
	created := false
	defer func() { // delete ONLY the file this test created
		if created {
			cctx, cc := context.WithTimeout(context.Background(), 30*time.Second)
			defer cc()
			if err := c.DeleteFile(cctx, p); err != nil {
				t.Errorf("cleanup delete of own file failed: %s", rd(err))
			}
		}
	}()
	created = true // set before the PUT: deleting a never-created unique name is harmless, a half-created file must not leak
	if err := c.WriteFile(ctx, p, bytes.NewReader(payload)); err != nil {
		t.Fatalf("write: %s", rd(err))
	}
	if ok, err := c.FileExists(ctx, p); err != nil || !ok {
		t.Fatalf("exists after write: ok=%v err=%s", ok, rd(err))
	}
	fi, err := c.GetFileInfo(ctx, p)
	if err != nil || fi.Size != int64(len(payload)) {
		t.Fatalf("stat: size mismatch or error (%s)", rd(err))
	}
	entries, err := c.ListDirectory(ctx, d.sandbox)
	if err != nil {
		t.Fatalf("list after write: %s", rd(err))
	}
	found := false
	for _, e := range entries {
		if e.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatalf("written file not present in sandbox listing")
	}
	rc, err := c.ReadFile(ctx, p)
	if err != nil {
		t.Fatalf("read: %s", rd(err))
	}
	got, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatalf("read body: %s", rd(err))
	}
	if sha256.Sum256(got) != want {
		t.Fatalf("checksum mismatch after round trip")
	}

	// failure path: missing path inside the sandbox is a clean not-found
	missing := strings.TrimRight(d.sandbox, "/") + "/" + name + ".does-not-exist"
	if st := d.rawStatus(t, "HEAD", missing); st != 404 {
		t.Fatalf("missing file: want HTTP 404, got %d", st)
	}
	if ok, err := c.FileExists(ctx, missing); err != nil || ok {
		t.Fatalf("missing file: want exists=false err=nil, got ok=%v err=%s", ok, rd(err))
	}
	if _, err := c.ReadFile(ctx, missing); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("reading a missing file must fail with status 404, got %v", err)
	}

	// delete the file and prove it is gone
	if err := c.DeleteFile(ctx, p); err != nil {
		t.Fatalf("delete: %s", rd(err))
	}
	created = false
	if st := d.rawStatus(t, "HEAD", p); st != 404 {
		t.Fatalf("after delete: want HTTP 404 from the server, got %d", st)
	}
	if ok, err := c.FileExists(ctx, p); err != nil || ok {
		t.Fatalf("file still present after delete (ok=%v err=%s)", ok, rd(err))
	}
}

// A deliberately wrong credential must be rejected by the real server (401), never accepted.
func TestLiveWebDAV_BadCredentialRejected(t *testing.T) {
	d := loadLiveDAV(t)
	d.requireService(t)
	bad := d.newClient(t, d.pass+"_deliberately_wrong")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := bad.Connect(ctx)
	if err == nil {
		t.Fatalf("connect with a wrong password must fail")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected a 401 rejection, got: %s", rd(err))
	}
	if bad.IsConnected() {
		t.Fatalf("client must not be connected after a rejected credential")
	}
}
