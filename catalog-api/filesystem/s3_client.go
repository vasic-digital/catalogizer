// s3_client.go - CAT-S3-1: an S3-compatible object-store client behind FileSystemClient (AWS S3, MinIO, Ceph RGW, Garage, Backblaze B2, ...).
//
// Library: github.com/aws/aws-sdk-go-v2/service/s3 (already a direct dependency of catalog-api, used by the sync service). No new module.
//
// Model. A storage root is "bucket[/prefix]" on an endpoint. The client only ever addresses that one bucket and only keys under the prefix:
//   - every caller path is cleaned AS IF ROOTED (ConfinePath), so a ".." segment cannot climb out of the prefix; a NAME that merely
//     contains two dots stays a name;
//   - the bucket is fixed at construction and never taken from a path.
//
// Directories do not exist in S3. They are emulated: a "directory" is a key prefix that has at least one key below it, or a zero-byte key
// whose name ends in "/" (what CreateDirectory writes and what most S3 consoles write). ListDirectory uses the "/" delimiter.
//
// Network policy (SECQ-1/W5, same as WebDAV): the endpoint is an operator-chosen destination. The transport is netguard.OperatorTransport
// (the RESOLVED address of every connection is checked: loopback, link-local and cloud-metadata addresses are refused unless the operator
// allow-list names loopback/private ranges) and every redirect hop is re-checked. TLS certificate verification is on and there is NO switch to
// turn it off; a private CA is added to the system trust store. A plain-http endpoint is accepted because LAN MinIO/Ceph installs use it.
//
// Credentials never appear in an error, in GetConfig or in a log line (scrubbed in wrapErr).
//
// Not implemented, by design (gaps recorded in the CAT-S3-1 report): SeekableClient (range reads), multipart upload for objects above 5 GiB
// (a single PutObject/CopyObject is used), server-side encryption options, and STS / instance-role credentials (static keys or anonymous only).
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"catalogizer/internal/netguard"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// s3OpTimeout bounds one metadata operation as a whole (HEAD, LIST, DELETE, COPY of small objects). Transfers (ReadFile body, WriteFile) are
// bounded by the caller's context. A variable so tests can lower it.
var s3OpTimeout = 30 * time.Second

// S3Config is the configuration of an S3Client.
type S3Config struct {
	// Endpoint is the service URL (https://s3.eu-central-1.amazonaws.com, http://minio.lan:9000). Required. No userinfo.
	Endpoint string `json:"endpoint"`
	// Region is the signing region. Empty defaults to us-east-1 (what most self-hosted servers accept).
	Region string `json:"region"`
	// Bucket is the only bucket the client addresses. Required.
	Bucket string `json:"bucket"`
	// Prefix confines the client to the keys below it ("" = the whole bucket).
	Prefix string `json:"prefix"`
	// AccessKey / SecretKey are static credentials; both empty means anonymous access.
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	// PathStyle selects path-style addressing (endpoint/bucket/key). False selects virtual-host style (bucket.endpoint/key).
	PathStyle bool `json:"path_style"`
}

// S3Client implements FileSystemClient for S3-compatible object stores.
type S3Client struct {
	cfg       S3Config
	prefix    string // normalised: "" or "a/b/"
	endpoint  *url.URL
	api       *awss3.Client
	httpC     *http.Client
	connected atomic.Bool
	skipped   atomic.Int64
	mu        sync.Mutex

	// opTransport is the operator transport of this client, created on the first Connect and REUSED by later Connects so the
	// address-class pin of its first connection (netguard.classPin, CAT-SECQ1-A) survives a reconnect. A new S3Client (process
	// restart, root re-created) starts a new pin. Guarded by mu.
	opTransport *http.Transport

	// httpOverride replaces the guarded HTTP client. Unit tests only (virtual-host Host-header assertion); never set by production code.
	httpOverride *http.Client
}

// NewS3Client validates cfg and returns an unconnected client. Network policy is enforced in Connect and on every dial.
func NewS3Client(cfg *S3Config) (*S3Client, error) {
	if cfg == nil {
		return nil, errors.New("s3: nil config")
	}
	c := *cfg
	if strings.TrimSpace(c.Endpoint) == "" {
		return nil, errors.New("s3: endpoint is required")
	}
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Host == "" {
		return nil, errors.New("s3: endpoint is not an absolute URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("s3: endpoint scheme %q not permitted (http or https)", u.Scheme)
	}
	if u.User != nil {
		return nil, errors.New("s3: credentials in the endpoint URL are not permitted")
	}
	if err := validateS3Bucket(c.Bucket); err != nil {
		return nil, err
	}
	if hasDotDotSegment(c.Prefix) {
		return nil, errors.New("s3: prefix must not contain a '..' segment")
	}
	if (c.AccessKey == "") != (c.SecretKey == "") {
		return nil, errors.New("s3: access key and secret key must be set together (or both empty for anonymous access)")
	}
	if c.Region == "" {
		c.Region = "us-east-1"
	}
	p := strings.Trim(ConfinePath(c.Prefix), "/")
	if p != "" {
		p += "/"
	}
	return &S3Client{cfg: c, prefix: p, endpoint: u}, nil
}

func validateS3Bucket(b string) error {
	if b == "" {
		return errors.New("s3: bucket is required")
	}
	if len(b) > 63 || strings.ContainsAny(b, "/\\ \t\r\n\x00?#%") || b == "." || b == ".." {
		return errors.New("s3: invalid bucket name")
	}
	return nil
}

func (c *S3Client) opCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s3OpTimeout)
}

// Connect validates the endpoint against the operator network policy, builds the signing client and checks the bucket with HeadBucket.
func (c *S3Client) Connect(ctx context.Context) error {
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	if _, err := netguard.CheckOperatorURL(cctx, c.cfg.Endpoint, "http", "https"); err != nil {
		return c.wrapErr("connect", err)
	}
	httpC := c.httpOverride
	if httpC == nil {
		c.mu.Lock()
		if c.opTransport == nil {
			c.opTransport = netguard.OperatorTransport()
		}
		tr := c.opTransport
		c.mu.Unlock()
		httpC = &http.Client{Transport: tr, CheckRedirect: netguard.OperatorCheckRedirect}
	}
	opts := awss3.Options{
		Region:       c.cfg.Region,
		BaseEndpoint: aws.String(c.cfg.Endpoint),
		UsePathStyle: c.cfg.PathStyle,
		HTTPClient:   httpC,
		// Many S3-compatible servers reject the default CRC trailing checksums; compute them only where the API requires one.
		RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
		ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
		RetryMaxAttempts:           2,
	}
	if c.cfg.AccessKey != "" {
		opts.Credentials = credentials.NewStaticCredentialsProvider(c.cfg.AccessKey, c.cfg.SecretKey, "")
	} else {
		opts.Credentials = aws.AnonymousCredentials{}
	}
	api := awss3.New(opts)
	if _, err := api.HeadBucket(cctx, &awss3.HeadBucketInput{Bucket: aws.String(c.cfg.Bucket)}); err != nil {
		return c.wrapErr("connect (HeadBucket)", err)
	}
	c.mu.Lock()
	c.api, c.httpC = api, httpC
	c.mu.Unlock()
	c.connected.Store(true)
	return nil
}

// Disconnect drops the connection state and idle sockets.
func (c *S3Client) Disconnect(ctx context.Context) error {
	c.connected.Store(false)
	c.mu.Lock()
	h := c.httpC
	c.api, c.httpC = nil, nil
	c.mu.Unlock()
	if h != nil {
		h.CloseIdleConnections()
	}
	return nil
}

// IsConnected reports whether Connect succeeded and Disconnect was not called since.
func (c *S3Client) IsConnected() bool { return c.connected.Load() }

// TestConnection re-checks the bucket.
func (c *S3Client) TestConnection(ctx context.Context) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	if _, err := api.HeadBucket(cctx, &awss3.HeadBucketInput{Bucket: aws.String(c.cfg.Bucket)}); err != nil {
		return c.wrapErr("test connection", err)
	}
	return nil
}

func (c *S3Client) client() (*awss3.Client, error) {
	if !c.IsConnected() {
		return nil, errors.New("not connected")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.api == nil {
		return nil, errors.New("not connected")
	}
	return c.api, nil
}

// key maps a caller path to an object key below the prefix. The path is cleaned as if rooted, so it cannot leave the prefix.
func (c *S3Client) key(p string) string {
	return c.prefix + strings.TrimPrefix(ConfinePath(p), "/")
}

// dirPrefix is the listing prefix of the directory p ("" -> the root prefix).
func (c *S3Client) dirPrefix(p string) string {
	k := c.key(p)
	if k == "" || strings.HasSuffix(k, "/") {
		return k
	}
	return k + "/"
}

// rel maps an object key back to the rooted caller path.
func (c *S3Client) rel(key string) string {
	return "/" + strings.TrimSuffix(strings.TrimPrefix(key, c.prefix), "/")
}

// scrubbedError carries a message with the credentials removed and the original cause for errors.Is / errors.As.
type scrubbedError struct {
	msg   string
	cause error
}

func (e *scrubbedError) Error() string { return e.msg }
func (e *scrubbedError) Unwrap() error { return e.cause }

func (c *S3Client) wrapErr(op string, err error) error {
	if err == nil {
		return nil
	}
	if isS3NotFound(err) {
		return &scrubbedError{msg: fmt.Sprintf("s3 %s: %v", op, os.ErrNotExist), cause: os.ErrNotExist}
	}
	msg := err.Error()
	for _, secret := range []string{c.cfg.SecretKey, c.cfg.AccessKey} {
		if secret != "" {
			msg = strings.ReplaceAll(msg, secret, "***")
		}
	}
	return &scrubbedError{msg: "s3 " + op + ": " + msg, cause: err}
}

func isS3NotFound(err error) bool {
	var nf *types.NotFound
	var nsk *types.NoSuchKey
	var nsb *types.NoSuchBucket
	if errors.As(err, &nf) || errors.As(err, &nsk) || errors.As(err, &nsb) {
		return true
	}
	var re *smithyhttp.ResponseError
	return errors.As(err, &re) && re.HTTPStatusCode() == http.StatusNotFound
}

// s3CtxReader makes a copy loop observe cancellation.
type s3CtxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r s3CtxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// ReadFile opens the object for reading. The caller must Close the body.
func (c *S3Client) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	api, err := c.client()
	if err != nil {
		return nil, err
	}
	out, err := api.GetObject(ctx, &awss3.GetObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(c.key(path))})
	if err != nil {
		return nil, c.wrapErr("read "+c.rel(c.key(path)), err)
	}
	return out.Body, nil
}

// WriteFile stores data as one object. A non-seekable reader is spooled to a temporary file first (the signer needs the length and, on
// plain http, the payload hash).
func (c *S3Client) WriteFile(ctx context.Context, path string, data io.Reader) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	k := c.key(path)
	if k == "" || strings.HasSuffix(k, "/") {
		return fmt.Errorf("s3 write: %q is not an object path", path)
	}
	body, size, cleanup, err := c.seekable(ctx, data)
	if err != nil {
		return c.wrapErr("write spool", err)
	}
	defer cleanup()
	if _, err := api.PutObject(ctx, &awss3.PutObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(k), Body: body, ContentLength: aws.Int64(size)}); err != nil {
		return c.wrapErr("write "+c.rel(k), err)
	}
	return nil
}

// seekable returns a body the signer can read twice and its length. A ReadSeeker is used as it is and is ALWAYS read from offset 0 (its
// current offset is ignored); if it cannot be sized or rewound the error is returned, never a silent fall-through to the spool (which would
// read from the END and store a 0-byte object as a success).
func (c *S3Client) seekable(ctx context.Context, data io.Reader) (io.ReadSeeker, int64, func(), error) {
	if rs, ok := data.(io.ReadSeeker); ok {
		end, err := rs.Seek(0, io.SeekEnd)
		if err != nil {
			return nil, 0, func() { /* no-op cleanup: nothing was acquired for this source, so there is nothing to release */ }, fmt.Errorf("size the source: %w", err)
		}
		if _, err = rs.Seek(0, io.SeekStart); err != nil {
			return nil, 0, func() { /* no-op cleanup: nothing was acquired for this source, so there is nothing to release */ }, fmt.Errorf("rewind the source: %w", err)
		}
		return rs, end, func() { /* no-op cleanup: nothing was acquired for this source, so there is nothing to release */ }, nil
	}
	f, err := os.CreateTemp("", "catalogizer-s3-spool-*")
	if err != nil {
		return nil, 0, func() { /* no-op cleanup: nothing was acquired for this source, so there is nothing to release */ }, err
	}
	cleanup := func() { _ = f.Close(); _ = os.Remove(f.Name()) }
	n, err := io.Copy(f, s3CtxReader{ctx: ctx, r: data})
	if err != nil {
		cleanup()
		return nil, 0, func() { /* no-op cleanup: the temp file was already released by cleanup() above */ }, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		cleanup()
		return nil, 0, func() { /* no-op cleanup: the temp file was already released by cleanup() above */ }, err
	}
	return f, n, cleanup, nil
}

// GetFileInfo stats an object; a key prefix with objects below it (or its "dir/" marker) is a directory. The root is always a directory.
func (c *S3Client) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	api, err := c.client()
	if err != nil {
		return nil, err
	}
	k := c.key(path)
	name := pathpkg.Base(ConfinePath(path))
	if k == "" || k == c.prefix {
		return &FileInfo{Name: "/", Path: "/", IsDir: true, Mode: os.ModeDir | 0o755}, nil
	}
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	h, err := api.HeadObject(cctx, &awss3.HeadObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(k)})
	if err == nil {
		fi := &FileInfo{Name: name, Path: c.rel(k), Size: aws.ToInt64(h.ContentLength), Mode: 0o644}
		if h.LastModified != nil {
			fi.ModTime = *h.LastModified
		}
		return fi, nil
	}
	if !isS3NotFound(err) {
		return nil, c.wrapErr("stat "+c.rel(k), err)
	}
	l, lerr := api.ListObjectsV2(cctx, &awss3.ListObjectsV2Input{Bucket: aws.String(c.cfg.Bucket), Prefix: aws.String(k + "/"), MaxKeys: aws.Int32(1)})
	if lerr != nil {
		return nil, c.wrapErr("stat "+c.rel(k), lerr)
	}
	if len(l.Contents) > 0 || len(l.CommonPrefixes) > 0 {
		return &FileInfo{Name: name, Path: c.rel(k), IsDir: true, Mode: os.ModeDir | 0o755}, nil
	}
	return nil, c.wrapErr("stat "+c.rel(k), os.ErrNotExist)
}

// FileExists reports whether path is an object or a directory.
func (c *S3Client) FileExists(ctx context.Context, path string) (bool, error) {
	_, err := c.GetFileInfo(ctx, path)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

// DeleteFile removes one object. A missing object is an error (os.ErrNotExist), like the other clients; S3's own delete is idempotent.
func (c *S3Client) DeleteFile(ctx context.Context, path string) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	k := c.key(path)
	if k == "" || strings.HasSuffix(k, "/") {
		return fmt.Errorf("s3 delete: %q is not an object path", path)
	}
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	if _, err := api.HeadObject(cctx, &awss3.HeadObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(k)}); err != nil {
		return c.wrapErr("delete "+c.rel(k), err)
	}
	if _, err := api.DeleteObject(cctx, &awss3.DeleteObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(k)}); err != nil {
		return c.wrapErr("delete "+c.rel(k), err)
	}
	return nil
}

// CopyFile copies one object inside the bucket with a server-side copy (objects up to 5 GiB).
func (c *S3Client) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	src, dst := c.key(srcPath), c.key(dstPath)
	if src == "" || dst == "" || strings.HasSuffix(src, "/") || strings.HasSuffix(dst, "/") {
		return fmt.Errorf("s3 copy: not object paths")
	}
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	if _, err := api.CopyObject(cctx, &awss3.CopyObjectInput{
		Bucket:     aws.String(c.cfg.Bucket),
		Key:        aws.String(dst),
		CopySource: aws.String(escapeCopySource(c.cfg.Bucket, src)),
	}); err != nil {
		return c.wrapErr("copy "+c.rel(src)+" -> "+c.rel(dst), err)
	}
	return nil
}

// escapeCopySource percent-encodes "bucket/key" for the X-Amz-Copy-Source header: every byte except the unreserved set and "/" is encoded
// (url.PathEscape would leave + & = $ : @ , as they are).
func escapeCopySource(bucket, key string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for _, ch := range []byte(bucket + "/" + key) {
		switch {
		case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9', ch == '-', ch == '_', ch == '.', ch == '~', ch == '/':
			b.WriteByte(ch)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[ch>>4])
			b.WriteByte(hex[ch&15])
		}
	}
	return b.String()
}

// SkippedKeys reports how many listed keys were skipped because they do not map to a clean name below the directory (empty, "." or ".."
// segments such as "lib//x" or "lib/../up"). Such a key can exist in S3 and cannot be addressed safely through a slash path.
func (c *S3Client) SkippedKeys() int64 { return c.skipped.Load() }

func cleanS3Name(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.Contains(name, "/")
}

// ListDirectory lists the entries directly below path ("/" delimiter). It honours WithListLimit. Every page has its own timeout, so a
// large directory is not cut by one deadline that covers all pages.
func (c *S3Client) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	api, err := c.client()
	if err != nil {
		return nil, err
	}
	pfx := c.dirPrefix(path)
	limit, limited := ListLimit(ctx)
	var out []*FileInfo
	var token *string
	for {
		res, err := c.listPage(ctx, api, &awss3.ListObjectsV2Input{Bucket: aws.String(c.cfg.Bucket), Prefix: aws.String(pfx), Delimiter: aws.String("/"), ContinuationToken: token})
		if err != nil {
			return nil, c.wrapErr("list "+c.rel(pfx), err)
		}
		for _, cp := range res.CommonPrefixes {
			p := aws.ToString(cp.Prefix)
			name := strings.TrimSuffix(strings.TrimPrefix(p, pfx), "/")
			if !cleanS3Name(name) {
				c.skipped.Add(1)
				continue
			}
			out = append(out, &FileInfo{Name: name, Path: c.rel(p), IsDir: true, Mode: os.ModeDir | 0o755})
		}
		for _, o := range res.Contents {
			k := aws.ToString(o.Key)
			if k == pfx {
				continue // the directory's own marker
			}
			name := strings.TrimPrefix(k, pfx)
			if strings.Contains(name, "/") {
				continue
			}
			if !cleanS3Name(name) {
				c.skipped.Add(1)
				continue
			}
			fi := &FileInfo{Name: name, Path: c.rel(k), Size: aws.ToInt64(o.Size), Mode: 0o644}
			if o.LastModified != nil {
				fi.ModTime = *o.LastModified
			}
			out = append(out, fi)
		}
		if limited && len(out) >= limit {
			return out[:limit], nil
		}
		if !aws.ToBool(res.IsTruncated) || res.NextContinuationToken == nil {
			return out, nil
		}
		token = res.NextContinuationToken
	}
}

// listPage runs one ListObjectsV2 request under its own operation timeout.
func (c *S3Client) listPage(ctx context.Context, api *awss3.Client, in *awss3.ListObjectsV2Input) (*awss3.ListObjectsV2Output, error) {
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	return api.ListObjectsV2(cctx, in)
}

// CreateDirectory emulates mkdir with a zero-byte "dir/" key. The root needs nothing.
func (c *S3Client) CreateDirectory(ctx context.Context, path string) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	k := c.dirPrefix(path)
	if k == "" || k == c.prefix {
		return nil
	}
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	if _, err := api.PutObject(cctx, &awss3.PutObjectInput{Bucket: aws.String(c.cfg.Bucket), Key: aws.String(k), Body: strings.NewReader(""), ContentLength: aws.Int64(0)}); err != nil {
		return c.wrapErr("mkdir "+c.rel(k), err)
	}
	return nil
}

// DeleteDirectory removes every key below the directory, marker included, in DeleteObjects batches of at most 1000 keys. Every request has
// its own timeout (a large directory is not cut half-deleted by one deadline). A per-key failure reported by the server is an error. The
// root is refused (ErrRefuseDeleteRoot).
func (c *S3Client) DeleteDirectory(ctx context.Context, path string) error {
	api, err := c.client()
	if err != nil {
		return err
	}
	pfx := c.dirPrefix(path)
	if pfx == "" || pfx == c.prefix {
		return ErrRefuseDeleteRoot
	}
	var token *string
	for {
		res, err := c.listPage(ctx, api, &awss3.ListObjectsV2Input{Bucket: aws.String(c.cfg.Bucket), Prefix: aws.String(pfx), ContinuationToken: token})
		if err != nil {
			return c.wrapErr("rmdir list "+c.rel(pfx), err)
		}
		var ids []types.ObjectIdentifier
		for _, o := range res.Contents {
			k := aws.ToString(o.Key)
			if !strings.HasPrefix(k, pfx) { // belt and braces: never delete outside the listed prefix
				continue
			}
			ids = append(ids, types.ObjectIdentifier{Key: aws.String(k)})
		}
		for len(ids) > 0 {
			n := len(ids)
			if n > s3DeleteBatch {
				n = s3DeleteBatch
			}
			if err := c.deleteBatch(ctx, api, ids[:n]); err != nil {
				return err
			}
			ids = ids[n:]
		}
		if !aws.ToBool(res.IsTruncated) || res.NextContinuationToken == nil {
			return nil
		}
		token = res.NextContinuationToken
	}
}

// s3DeleteBatch is the DeleteObjects limit (S3 allows 1000 keys per call).
const s3DeleteBatch = 1000

func (c *S3Client) deleteBatch(ctx context.Context, api *awss3.Client, ids []types.ObjectIdentifier) error {
	cctx, cancel := c.opCtx(ctx)
	defer cancel()
	out, err := api.DeleteObjects(cctx, &awss3.DeleteObjectsInput{Bucket: aws.String(c.cfg.Bucket), Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)}})
	if err != nil {
		return c.wrapErr("rmdir", err)
	}
	if len(out.Errors) > 0 {
		e := out.Errors[0]
		return c.wrapErr("rmdir", fmt.Errorf("%d of %d keys not deleted, first: %s (%s)", len(out.Errors), len(ids), c.rel(aws.ToString(e.Key)), aws.ToString(e.Code)))
	}
	return nil
}

// GetProtocol returns "s3".
func (c *S3Client) GetProtocol() string { return "s3" }

// GetConfig returns a copy of the configuration with the credentials masked.
func (c *S3Client) GetConfig() interface{} {
	cp := c.cfg
	if cp.AccessKey != "" {
		cp.AccessKey = "***"
	}
	if cp.SecretKey != "" {
		cp.SecretKey = "***"
	}
	return &cp
}
