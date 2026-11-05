package filesystem

// TEST DOUBLE NOTICE (11.4.289): this file runs the REAL S3 client (real aws-sdk-go-v2 signing and wire code) against an in-process
// httptest handler that IMITATES a small part of S3. The handler is a test double. These are UNIT-CLASS tests: they prove the client's
// logic (confinement, addressing, SSRF refusal, error hygiene), NOT that the client works against a real S3 server. Live evidence against a
// real S3-compatible server is PENDING (separate unit).

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
	"testing/iotest"
	"time"

	"catalogizer/internal/netguard"

	"github.com/stretchr/testify/require"
)

const (
	s3TestAccess = "AKIATESTACCESSKEY01"
	s3TestSecret = "s3cr3t/TESTSECRETVALUE+0123456789abcdef"
)

type fakeObj struct {
	data []byte
	mod  time.Time
}

// fakeS3 is the test double. Path-style: /<bucket>/<key>. It records every request (host, method, raw path) and checks that the
// request is SigV4-signed with the expected access key.
type fakeS3 struct {
	mu          sync.Mutex
	bucket      string
	objs        map[string]fakeObj
	reqs        []string
	hosts       []string
	vhost       string // when set, the bucket comes from the Host header (virtual-host addressing)
	badSigns    int
	crossBucket int
	delay       time.Duration
	copySources []string
	deletePosts int
}

func newFakeS3(bucket string) *fakeS3 {
	return &fakeS3{bucket: bucket, objs: map[string]fakeObj{}}
}

func (f *fakeS3) put(key, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.objs[key] = fakeObj{data: []byte(body), mod: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)}
}

func (f *fakeS3) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := []string{}
	for k := range f.objs {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func (f *fakeS3) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.reqs...)
}

func (f *fakeS3) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reqs = append(f.reqs, r.Method+" "+r.URL.EscapedPath()+"?"+r.URL.RawQuery)
	f.hosts = append(f.hosts, r.Host)
	if !strings.Contains(r.Header.Get("Authorization"), "Credential="+s3TestAccess+"/") {
		f.badSigns++
		http.Error(w, "<Error><Code>AccessDenied</Code></Error>", http.StatusForbidden)
		return
	}
	var bucket, key string
	if f.vhost != "" {
		host := r.Host
		if i := strings.Index(host, ":"); i >= 0 {
			host = host[:i]
		}
		bucket = strings.TrimSuffix(host, "."+f.vhost)
		key = strings.TrimPrefix(r.URL.Path, "/")
	} else {
		p := strings.TrimPrefix(r.URL.Path, "/")
		bucket, key, _ = strings.Cut(p, "/")
	}
	if bucket != f.bucket {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<Error><Code>NoSuchBucket</Code></Error>"))
		return
	}
	if strings.HasSuffix(key, "forbidden") {
		w.WriteHeader(http.StatusForbidden)
		if r.Method != http.MethodHead {
			_, _ = w.Write([]byte("<Error><Code>AccessDenied</Code></Error>"))
		}
		return
	}
	switch {
	case key == "" && r.Method == http.MethodPost && r.URL.Query().Has("delete"):
		f.deletePosts++
		var req struct {
			Objects []struct{ Key string } `xml:"Object"`
		}
		b, _ := io.ReadAll(r.Body)
		_ = xml.Unmarshal(b, &req)
		var sb strings.Builder
		sb.WriteString("<DeleteResult>")
		for _, o := range req.Objects {
			if strings.HasSuffix(o.Key, "failkey") {
				sb.WriteString("<Error><Key>" + o.Key + "</Key><Code>AccessDenied</Code></Error>")
				continue
			}
			delete(f.objs, o.Key)
			sb.WriteString("<Deleted><Key>" + o.Key + "</Key></Deleted>")
		}
		sb.WriteString("</DeleteResult>")
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sb.String()))
	case key == "" && r.Method == http.MethodHead:
		w.WriteHeader(http.StatusOK)
	case key == "" && r.Method == http.MethodGet:
		f.list(w, r)
	case r.Method == http.MethodPut && r.Header.Get("X-Amz-Copy-Source") != "":
		f.copySources = append(f.copySources, r.Header.Get("X-Amz-Copy-Source"))
		src, _ := url.PathUnescape(r.Header.Get("X-Amz-Copy-Source"))
		src = strings.TrimPrefix(src, "/")
		sbucket, skey, _ := strings.Cut(src, "/")
		if sbucket != f.bucket {
			f.crossBucket++
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("<Error><Code>NoSuchBucket</Code></Error>"))
			return
		}
		o, ok := f.objs[skey]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("<Error><Code>NoSuchKey</Code></Error>"))
			return
		}
		f.objs[key] = fakeObj{data: o.data, mod: time.Now()}
		_, _ = w.Write([]byte(`<CopyObjectResult><ETag>"x"</ETag></CopyObjectResult>`))
	case r.Method == http.MethodPut:
		b, _ := io.ReadAll(r.Body)
		f.objs[key] = fakeObj{data: stripChunked(b, r), mod: time.Now()}
		w.Header().Set("ETag", `"x"`)
	case r.Method == http.MethodHead || r.Method == http.MethodGet:
		o, ok := f.objs[key]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			if r.Method == http.MethodGet {
				_, _ = w.Write([]byte("<Error><Code>NoSuchKey</Code></Error>"))
			}
			return
		}
		w.Header().Set("Content-Length", fmt.Sprint(len(o.data)))
		w.Header().Set("Last-Modified", o.mod.UTC().Format(http.TimeFormat))
		w.Header().Set("ETag", `"x"`)
		if r.Method == http.MethodGet {
			_, _ = w.Write(o.data)
		}
	case r.Method == http.MethodDelete:
		delete(f.objs, key)
		w.WriteHeader(http.StatusNoContent)
	default:
		w.WriteHeader(http.StatusNotImplemented)
	}
}

// stripChunked removes aws-chunked framing if the SDK used it (it should not with WhenRequired; kept so a regression shows as a content mismatch, not a hang).
func stripChunked(b []byte, r *http.Request) []byte {
	if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
		return b
	}
	return b
}

type listResult struct {
	XMLName        xml.Name `xml:"ListBucketResult"`
	Name           string
	Prefix         string
	KeyCount       int
	MaxKeys        int
	Delimiter      string
	IsTruncated    bool
	NextToken      string         `xml:"NextContinuationToken,omitempty"`
	Contents       []listContents `xml:"Contents"`
	CommonPrefixes []listPrefix   `xml:"CommonPrefixes"`
}
type listContents struct {
	Key          string
	LastModified string
	Size         int
	ETag         string
}
type listPrefix struct{ Prefix string }

func (f *fakeS3) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix, delim := q.Get("prefix"), q.Get("delimiter")
	maxKeys := 2 // small page so pagination is exercised
	start := q.Get("continuation-token")
	var keys []string
	for k := range f.objs {
		if strings.HasPrefix(k, prefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	res := listResult{Name: f.bucket, Prefix: prefix, Delimiter: delim, MaxKeys: maxKeys}
	seen := map[string]bool{}
	n := 0
	for _, k := range keys {
		if start != "" && k <= start {
			continue
		}
		if delim != "" {
			rest := strings.TrimPrefix(k, prefix)
			if i := strings.Index(rest, delim); i >= 0 {
				cp := prefix + rest[:i+1]
				if seen[cp] {
					continue
				}
				seen[cp] = true
				if n >= maxKeys {
					res.IsTruncated, res.NextToken = true, lastBefore(keys, cp)
					break
				}
				res.CommonPrefixes = append(res.CommonPrefixes, listPrefix{cp})
				n++
				// skip the rest of this prefix on the next page
				continue
			}
		}
		if n >= maxKeys {
			res.IsTruncated, res.NextToken = true, lastBefore(keys, k)
			break
		}
		o := f.objs[k]
		res.Contents = append(res.Contents, listContents{Key: k, LastModified: o.mod.Format(time.RFC3339), Size: len(o.data), ETag: `"x"`})
		n++
	}
	res.KeyCount = n
	w.Header().Set("Content-Type", "application/xml")
	_, _ = w.Write([]byte(xml.Header))
	_ = xml.NewEncoder(w).Encode(res)
}

// lastBefore returns the key that precedes k so the next page (keys > token) resumes AT k.
func lastBefore(keys []string, k string) string {
	prev := ""
	for _, x := range keys {
		if x >= k {
			return prev
		}
		prev = x
	}
	return prev
}

func s3Client(t *testing.T, srv *httptest.Server, bucket, prefix string) *S3Client {
	t.Helper()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Region: "us-east-1", Bucket: bucket, Prefix: prefix, AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	t.Cleanup(func() { _ = c.Disconnect(context.Background()) })
	return c
}

func TestS3_RoundTripAndStat(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "lib")
	ctx := context.Background()

	require.NoError(t, c.WriteFile(ctx, "/movies/a b+c.txt", strings.NewReader("hello s3")))
	require.Equal(t, []string{"lib/movies/a b+c.txt"}, f.keys(), "object lands under the configured prefix")
	rc, err := c.ReadFile(ctx, "/movies/a b+c.txt")
	require.NoError(t, err)
	b, _ := io.ReadAll(rc)
	_ = rc.Close()
	require.Equal(t, "hello s3", string(b))

	fi, err := c.GetFileInfo(ctx, "/movies/a b+c.txt")
	require.NoError(t, err)
	require.Equal(t, int64(8), fi.Size)
	require.False(t, fi.IsDir)
	require.Equal(t, "a b+c.txt", fi.Name)

	d, err := c.GetFileInfo(ctx, "/movies")
	require.NoError(t, err)
	require.True(t, d.IsDir, "a key prefix with objects below it is a directory")

	ok, err := c.FileExists(ctx, "/nope")
	require.NoError(t, err)
	require.False(t, ok)
	_, err = c.ReadFile(ctx, "/nope")
	require.True(t, errors.Is(err, os.ErrNotExist), "got %v", err)
	require.Zero(t, f.badSigns, "every request was SigV4-signed with the configured access key")
}

func TestS3_ListDirectoryPaginatesAndEmulatesDirs(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	for _, k := range []string{"lib/a.txt", "lib/b.txt", "lib/c.txt", "lib/sub1/x", "lib/sub2/y", "lib/sub2/z", "other/leak.txt"} {
		f.put(k, "d")
	}
	c := s3Client(t, srv, "media", "lib")
	got, err := c.ListDirectory(context.Background(), "/")
	require.NoError(t, err)
	names := map[string]bool{}
	for _, e := range got {
		names[fmt.Sprintf("%s:%v", e.Name, e.IsDir)] = true
	}
	require.Equal(t, map[string]bool{"a.txt:false": true, "b.txt:false": true, "c.txt:false": true, "sub1:true": true, "sub2:true": true}, names)
	require.False(t, names["leak.txt:false"], "an object outside the prefix is never listed")

	lim, err := c.ListDirectory(WithListLimit(context.Background(), 2), "/")
	require.NoError(t, err)
	require.LessOrEqual(t, len(lim), 2, "the list limit is honoured")
}

func TestS3_MkdirDeleteCopy(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	ctx := context.Background()
	require.NoError(t, c.CreateDirectory(ctx, "/d"))
	require.Equal(t, []string{"d/"}, f.keys(), "mkdir is emulated with a zero-byte key ending in a slash")
	di, err := c.GetFileInfo(ctx, "/d")
	require.NoError(t, err)
	require.True(t, di.IsDir)

	require.NoError(t, c.WriteFile(ctx, "/d/one", strings.NewReader("1")))
	require.NoError(t, c.CopyFile(ctx, "/d/one", "/d/two"))
	require.ElementsMatch(t, []string{"d/", "d/one", "d/two"}, f.keys())
	require.NoError(t, c.DeleteFile(ctx, "/d/one"))
	require.ElementsMatch(t, []string{"d/", "d/two"}, f.keys())
	require.Error(t, c.DeleteFile(ctx, "/d/one"), "deleting a missing object is an error like the other clients")

	require.NoError(t, c.DeleteDirectory(ctx, "/d"))
	require.Empty(t, f.keys())
	require.ErrorIs(t, c.DeleteDirectory(ctx, "/"), ErrRefuseDeleteRoot)
}

// Confinement: a ".." segment can never leave the configured bucket prefix.
func TestS3_PathTraversalConfinedToPrefix(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	f.put("secret/other.txt", "TOP")
	f.put("lib/ok.txt", "ok")
	c := s3Client(t, srv, "media", "lib")
	ctx := context.Background()
	for _, p := range []string{"../secret/other.txt", "/../../secret/other.txt", "a/../../secret/other.txt", "/./../secret/other.txt"} {
		_, err := c.ReadFile(ctx, p)
		require.Error(t, err, p)
		require.True(t, errors.Is(err, os.ErrNotExist), "%s: got %v", p, err)
	}
	for _, p := range []string{"../secret/other.txt", "/../../secret/other.txt", "a/../../secret/other.txt", "/./../secret/other.txt"} {
		require.NoError(t, c.WriteFile(ctx, p, strings.NewReader("w")), p)
	}
	for _, k := range f.keys() {
		require.True(t, k == "secret/other.txt" || strings.HasPrefix(k, "lib/"), "key %q escaped the prefix", k)
	}
	b, _ := f.objs["secret/other.txt"]
	require.Equal(t, "TOP", string(b.data), "the object outside the prefix is untouched")
	// a NAME that merely contains two dots stays a name
	require.NoError(t, c.WriteFile(ctx, "/Wait.. Live/x", strings.NewReader("n")))
	require.Contains(t, f.keys(), "lib/Wait.. Live/x")
}

// Confinement to the bucket: the client only ever addresses the configured bucket.
func TestS3_OnlyConfiguredBucketAddressed(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	ctx := context.Background()
	_ = c.WriteFile(ctx, "/otherbucket/../x", strings.NewReader("1"))
	_ = c.CopyFile(ctx, "/x", "/../../otherbucket/y")
	require.NoError(t, c.WriteFile(ctx, "/y", strings.NewReader("1")))
	require.NoError(t, c.CopyFile(ctx, "/y", "/z"))
	require.Zero(t, f.crossBucket, "a server-side copy names the configured bucket as its source")
	for _, r := range f.requests() {
		require.True(t, strings.Contains(r, " /media") || strings.Contains(r, " /media/"), "request left the bucket: %s", r)
	}
}

func TestS3_ConfigRejections(t *testing.T) {
	for name, cfg := range map[string]S3Config{
		"no endpoint":      {Bucket: "b"},
		"no bucket":        {Endpoint: "https://s3.example.com"},
		"bad bucket":       {Endpoint: "https://s3.example.com", Bucket: "a/b"},
		"userinfo":         {Endpoint: "https://u:p@s3.example.com", Bucket: "b"},
		"scheme":           {Endpoint: "ftp://s3.example.com", Bucket: "b"},
		"traversal prefix": {Endpoint: "https://s3.example.com", Bucket: "b", Prefix: "../x"},
	} {
		cfg := cfg
		_, err := NewS3Client(&cfg)
		require.Error(t, err, name)
	}
}

// SSRF: same operator policy as WebDAV. Loopback, link-local and metadata destinations are refused at Connect, before any request reaches them.
func TestS3_SSRFOperatorPolicy(t *testing.T) {
	var hits int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { mu.Lock(); hits++; mu.Unlock() }))
	defer srv.Close()

	t.Setenv(netguard.AllowCIDRsEnv, "")
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Bucket: "b", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	err = c.Connect(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked), "loopback refused: %v", err)
	mu.Lock()
	require.Zero(t, hits, "the refused client must not reach the server")
	mu.Unlock()

	for _, u := range []string{"http://169.254.169.254/", "http://[::ffff:169.254.169.254]/", "http://[fe80::1]/", "http://metadata.invalid-but-literal.0x7f.1/"} {
		c, err := NewS3Client(&S3Config{Endpoint: u, Bucket: "b", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
		if err != nil {
			continue // refused at construction is also a refusal
		}
		err = c.Connect(context.Background())
		require.Error(t, err, u)
	}

	// A redirect to the metadata service is refused.
	redir := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusTemporaryRedirect)
	}))
	defer redir.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c2, err := NewS3Client(&S3Config{Endpoint: redir.URL, Bucket: "b", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	err = c2.Connect(context.Background())
	require.Error(t, err)
	require.True(t, errors.Is(err, netguard.ErrBlocked) || strings.Contains(err.Error(), "blocked"), "redirect to metadata refused: %v", err)
}

// Credentials never appear in an error, in GetConfig, or in a request URL.
func TestS3_NoCredentialsInErrorsOrConfig(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Region: "us-east-1", Bucket: "wrongbucket", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	var all []string
	for _, e := range []error{
		c.Connect(context.Background()),
		c.WriteFile(context.Background(), "/x", strings.NewReader("1")),
		c.CopyFile(context.Background(), "/a", "/b"),
	} {
		if e != nil {
			all = append(all, e.Error())
		}
	}
	_, e := c.ReadFile(context.Background(), "/x")
	if e != nil {
		all = append(all, e.Error())
	}
	require.NotEmpty(t, all)
	for _, s := range append(all, fmt.Sprintf("%+v %v", c.GetConfig(), c.GetConfig())) {
		require.NotContains(t, s, s3TestSecret)
		require.NotContains(t, s, s3TestAccess)
	}
	for _, r := range f.requests() {
		require.NotContains(t, r, s3TestSecret)
	}
}

func TestS3_ContextCancellationAndTimeout(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Bucket: "b", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = c.Connect(ctx)
	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second, "a cancelled context ends the call")

	old := s3OpTimeout
	s3OpTimeout = 200 * time.Millisecond
	defer func() { s3OpTimeout = old }()
	start = time.Now()
	err = c.Connect(context.Background())
	require.Error(t, err)
	require.Less(t, time.Since(start), 5*time.Second, "the per-operation timeout ends a hung server")
}

func TestS3_TLSVerifyOnByDefault(t *testing.T) {
	srv := httptest.NewTLSServer(newFakeS3("b"))
	defer srv.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	c, err := NewS3Client(&S3Config{Endpoint: srv.URL, Bucket: "b", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: true})
	require.NoError(t, err)
	err = c.Connect(context.Background())
	require.Error(t, err, "a self-signed certificate must be refused (no insecure switch exists)")
	require.Contains(t, strings.ToLower(err.Error()), "certificate")
}

// Addressing: path-style puts the bucket in the path; virtual-host puts it in the Host header.
func TestS3_AddressingStyles(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	require.NoError(t, c.WriteFile(context.Background(), "/k", strings.NewReader("1")))
	require.Contains(t, strings.Join(f.requests(), "\n"), "PUT /media/k")

	// virtual-host: dial the in-process server whatever the host name is (unit-test transport override), and assert the Host header.
	fv := newFakeS3("media")
	fv.vhost = "s3.example.test"
	srv2 := httptest.NewServer(fv)
	defer srv2.Close()
	addr := srv2.Listener.Addr().String()
	cv, err := NewS3Client(&S3Config{Endpoint: "http://s3.example.test", Region: "us-east-1", Bucket: "media", AccessKey: s3TestAccess, SecretKey: s3TestSecret, PathStyle: false})
	require.NoError(t, err)
	cv.httpOverride = &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, n, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, n, addr)
	}}}
	require.NoError(t, cv.Connect(context.Background()))
	require.NoError(t, cv.WriteFile(context.Background(), "/k", strings.NewReader("1")))
	require.Equal(t, []string{"k"}, fv.keys())
	require.Contains(t, fv.hosts[len(fv.hosts)-1], "media.s3.example.test", "virtual-host addressing puts the bucket in the Host header")
}

func TestS3_FactoryAndSettings(t *testing.T) {
	require.NoError(t, RegisterS3Protocol())
	require.NoError(t, RegisterS3Protocol(), "idempotent")
	defer UnregisterProtocol("s3")
	require.Contains(t, NewDefaultClientFactory().SupportedProtocols(), "s3")
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.1/32")
	cl, err := NewDefaultClientFactory().CreateClient(&StorageConfig{Protocol: "s3", Settings: map[string]interface{}{
		"url": srv.URL, "path": "media/lib", "username": s3TestAccess, "password": s3TestSecret, "region": "us-east-1", "addressing": "path",
	}})
	require.NoError(t, err)
	require.NoError(t, cl.Connect(context.Background()))
	require.NoError(t, cl.WriteFile(context.Background(), "/f", strings.NewReader("1")))
	require.Equal(t, []string{"lib/f"}, f.keys())
	require.Equal(t, "s3", cl.GetProtocol())

	_, err = NewDefaultClientFactory().CreateClient(&StorageConfig{Protocol: "s3", Settings: map[string]interface{}{"url": srv.URL, "path": "media", "bogus": "x"}})
	require.Error(t, err, "an unknown key is an error (PA-01)")
	_, err = NewDefaultClientFactory().CreateClient(&StorageConfig{Protocol: "s3", Settings: map[string]interface{}{"url": srv.URL, "path": "media", "addressing": "weird"}})
	require.Error(t, err)
}

// ---- review round 1 additions (CAT-S3-1) ----

func TestS3_StatPrefixBoundary(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	f.put("lib/foobar/x", "1")
	c := s3Client(t, srv, "media", "lib")
	ok, err := c.FileExists(context.Background(), "/foo")
	require.NoError(t, err)
	require.False(t, ok, "'foo' must not be a directory because 'foobar/' exists")
	d, err := c.GetFileInfo(context.Background(), "/foobar")
	require.NoError(t, err)
	require.True(t, d.IsDir)
}

func TestS3_ForbiddenIsNotNotFound(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	_, err := c.GetFileInfo(context.Background(), "/forbidden")
	require.Error(t, err)
	require.False(t, errors.Is(err, os.ErrNotExist), "a 403 must not read as 'does not exist': %v", err)
	ok, err := c.FileExists(context.Background(), "/forbidden")
	require.Error(t, err)
	require.False(t, ok)
	require.Error(t, c.DeleteFile(context.Background(), "/forbidden"))
	require.False(t, errors.Is(c.DeleteFile(context.Background(), "/forbidden"), os.ErrNotExist))
}

func TestS3_RmdirMultiPageBatchedPerRequestTimeout(t *testing.T) {
	f := newFakeS3("media")
	f.delay = 120 * time.Millisecond
	srv := httptest.NewServer(f)
	defer srv.Close()
	for i := 0; i < 7; i++ {
		f.put(fmt.Sprintf("d/k%d", i), "x")
	}
	f.put("d2/keep", "x")
	f.put("d/failkey-not-here", "x")
	delete(f.objs, "d/failkey-not-here")
	c := s3Client(t, srv, "media", "")
	old := s3OpTimeout
	s3OpTimeout = 400 * time.Millisecond // 4 list pages + deletes take ~1s in total: one shared deadline would cut it
	defer func() { s3OpTimeout = old }()
	require.NoError(t, c.DeleteDirectory(context.Background(), "/d"))
	require.Equal(t, []string{"d2/keep"}, f.keys(), "the sibling 'd2' survives and every key of 'd' is gone")
	reqs := strings.Join(f.requests(), "\n")
	require.Contains(t, reqs, "continuation-token", "rmdir follows the continuation token across pages")
	require.LessOrEqual(t, f.deletePosts, 4, "deletes are batched, not one request per key")
	require.NotContains(t, reqs, "DELETE /media/d/", "no per-object DELETE requests")
}

func TestS3_RmdirPerKeyErrorReported(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	f.put("d/ok", "x")
	f.put("d/failkey", "x")
	c := s3Client(t, srv, "media", "")
	err := c.DeleteDirectory(context.Background(), "/d")
	require.Error(t, err, "a per-key error in the DeleteObjects answer is an error, not a success")
	require.Contains(t, err.Error(), "AccessDenied")
}

// rewindFails refuses the FIRST rewind (the client's own sizing step) and behaves normally afterwards, so only the client's handling of that
// error decides the outcome (the SDK's own rewind would otherwise mask a swallowed error).
type rewindFails struct {
	*strings.Reader
	failed *bool
}

func (r rewindFails) Seek(off int64, whence int) (int64, error) {
	if whence == io.SeekStart && !*r.failed {
		*r.failed = true
		return 0, errors.New("rewind refused")
	}
	return r.Reader.Seek(off, whence)
}

func TestS3_WriteSeekFailureIsNotSilentEmptyObject(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	err := c.WriteFile(context.Background(), "/x", rewindFails{strings.NewReader("payload"), new(bool)})
	require.Error(t, err)
	require.Empty(t, f.keys(), "no 0-byte object may be stored")
	// a ReadSeeker that is not at offset 0 is stored from offset 0
	rd := strings.NewReader("payload")
	_, _ = rd.Seek(3, io.SeekStart)
	require.NoError(t, c.WriteFile(context.Background(), "/y", rd))
	require.Equal(t, "payload", string(f.objs["y"].data))
}

func TestS3_SpoolTempFileRemoved(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	require.NoError(t, c.WriteFile(context.Background(), "/s", iotest.OneByteReader(strings.NewReader("spooled body"))))
	require.Equal(t, "spooled body", string(f.objs["s"].data))
	ents, err := os.ReadDir(tmp)
	require.NoError(t, err)
	require.Empty(t, ents, "the spool file is removed after the upload")
	// also after a failed upload
	require.Error(t, c.WriteFile(context.Background(), "/forbidden", iotest.OneByteReader(strings.NewReader("zzz"))))
	ents, _ = os.ReadDir(tmp)
	require.Empty(t, ents)
}

// CheckRedirect is a line of defence of its own: called directly, it refuses a hop to a metadata address, a non-http scheme and an 11th hop,
// independent of the dial guard.
func TestS3_CheckRedirectInstalledAndRefuses(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	require.NotNil(t, c.httpC.CheckRedirect, "the guarded redirect policy is installed")
	mk := func(u string) *http.Request { r, _ := http.NewRequest("GET", u, nil); return r }
	require.True(t, errors.Is(c.httpC.CheckRedirect(mk("http://169.254.169.254/x"), []*http.Request{mk("http://a/")}), netguard.ErrBlocked))
	require.True(t, errors.Is(c.httpC.CheckRedirect(mk("ftp://example.com/x"), []*http.Request{mk("http://a/")}), netguard.ErrBlocked))
	require.Error(t, c.httpC.CheckRedirect(mk("http://example.com/"), make([]*http.Request, 10)))
}

func TestS3_CopySourceHeaderExactlyEscaped(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "lib")
	name := "/a b+c&d=e$f:g@h,i;j%k.txt"
	f.put("lib"+name, "1")
	require.NoError(t, c.CopyFile(context.Background(), name, "/dst"))
	require.Equal(t, []string{"media/lib/a%20b%2Bc%26d%3De%24f%3Ag%40h%2Ci%3Bj%25k.txt"}, f.copySources)
	require.Contains(t, f.keys(), "lib/dst")
}

func TestS3_ListSkipsUnaddressableKeys(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	for _, k := range []string{"lib/good", "lib//dbl", "lib/../up", "lib/./dot", "lib/sub/ok"} {
		f.put(k, "1")
	}
	c := s3Client(t, srv, "media", "lib")
	got, err := c.ListDirectory(context.Background(), "/")
	require.NoError(t, err)
	names := []string{}
	for _, e := range got {
		names = append(names, e.Name)
	}
	sort.Strings(names)
	require.Equal(t, []string{"good", "sub"}, names)
	require.GreaterOrEqual(t, c.SkippedKeys(), int64(2), "skipped keys are counted, not silently lost")
}

func TestS3_NotFoundErrorCarriesNoWireText(t *testing.T) {
	f := newFakeS3("media")
	srv := httptest.NewServer(f)
	defer srv.Close()
	c := s3Client(t, srv, "media", "")
	_, err := c.ReadFile(context.Background(), "/missing")
	require.True(t, errors.Is(err, os.ErrNotExist))
	var se *scrubbedError
	require.True(t, errors.As(err, &se))
	require.Equal(t, os.ErrNotExist, se.cause, "the cause is the bare sentinel, not the raw SDK text")
}

func TestS3_ListAndRmdirHungServerBoundedPerRequest(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusOK)
			return
		}
		<-block
	}))
	defer srv.Close()
	defer close(block)
	c := s3Client(t, srv, "b", "")
	old := s3OpTimeout
	s3OpTimeout = 200 * time.Millisecond
	defer func() { s3OpTimeout = old }()
	for name, fn := range map[string]func() error{
		"list":  func() error { _, err := c.ListDirectory(context.Background(), "/"); return err },
		"rmdir": func() error { return c.DeleteDirectory(context.Background(), "/d") },
	} {
		start := time.Now()
		require.Error(t, fn(), name)
		require.Less(t, time.Since(start), 8*time.Second, name)
	}
}
