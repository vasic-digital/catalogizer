package handlers

// WF22 fix round 2: the user path of the generic scanner. A root created through the API (url / mount point / allow_empty included), queued
// through the API, scanned by the real UniversalScanner with the real client factory against a real HTTP server and a real SQLite database:
// the journey the reviewer found unreachable (H1, H2) and untested (T1).

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/services"
	"catalogizer/internal/tests"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type apiHarness struct {
	t      *testing.T
	db     *database.DB
	r      *gin.Engine
	scan   *services.UniversalScanner
	handle *ScanHandler
}

func newAPIHarness(t *testing.T, withScanner bool) *apiHarness {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := database.WrapDB(tests.SetupTestDB(t), database.DialectSQLite)
	t.Cleanup(func() { db.Close() })
	h := &apiHarness{t: t, db: db, r: gin.New()}
	var sc scannerInterface
	if withScanner {
		h.scan = services.NewUniversalScanner(db, zap.NewNop(), nil, filesystem.NewDefaultClientFactory())
		require.NoError(t, h.scan.Start())
		t.Cleanup(h.scan.Stop)
		sc = h.scan
	} else {
		sc = newMockUniversalScanner()
	}
	h.handle = NewScanHandler(sc, db)
	h.r.POST("/storage/roots", h.handle.CreateStorageRoot)
	h.r.GET("/storage/roots", h.handle.GetStorageRoots)
	h.r.POST("/scans", h.handle.QueueScan)
	h.r.GET("/scans", h.handle.ListScans)
	h.r.GET("/scans/:job_id", h.handle.GetScanStatus)
	return h
}

func (h *apiHarness) do(method, path string, body interface{}) (int, map[string]interface{}) {
	h.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(h.t, err)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.r.ServeHTTP(w, req)
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

// waitScan polls the status endpoint until the job leaves "running"/"queued".
func (h *apiHarness) waitScan(jobID string) map[string]interface{} {
	h.t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		code, st := h.do("GET", "/scans/"+jobID, nil)
		if code == http.StatusOK {
			if s, _ := st["status"].(string); s != "running" && s != "" {
				return st
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.t.Fatal("scan did not finish within 30 s")
	return nil
}

// davServer is a tiny WebDAV server (PROPFIND Depth:1 only, basic auth) over a fixed tree: dir -> [name or name/]
func davServer(t *testing.T, user, pass string, tree map[string][]string) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u, p, ok := r.BasicAuth(); !ok || u != user || p != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="x"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method != "PROPFIND" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		dir := strings.TrimRight(r.URL.Path, "/")
		if dir == "" {
			dir = "/"
		}
		ents, ok := tree[dir]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		self := strings.TrimRight(dir, "/") + "/"
		var b strings.Builder
		b.WriteString(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>` + self + `</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		for _, e := range ents {
			isDir := strings.HasSuffix(e, "/")
			name := strings.TrimSuffix(e, "/")
			rt := `<D:resourcetype/><D:getcontentlength>7</D:getcontentlength>`
			href := self + name
			if isDir {
				rt = `<D:resourcetype><D:collection/></D:resourcetype>`
				href += "/"
			}
			b.WriteString(`<D:response><D:href>` + href + `</D:href><D:propstat><D:prop>` + rt + `</D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>`)
		}
		b.WriteString(`</D:multistatus>`)
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = io.WriteString(w, b.String())
	}))
	t.Cleanup(srv.Close)
	return srv
}

func filesIn(t *testing.T, db *database.DB, rootName string) map[string]bool {
	t.Helper()
	rows, err := db.Query(`SELECT f.path FROM files f JOIN storage_roots s ON s.id = f.storage_root_id WHERE s.name = ?`, rootName)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var p string
		require.NoError(t, rows.Scan(&p))
		out[p] = true
	}
	return out
}

// The whole journey for WebDAV: POST /storage/roots (url, credentials) -> POST /scans -> the real scanner -> the real WebDAV client -> SQLite.
func TestWF22_UserPath_WebDAVRootCreatedAndScannedThroughTheAPI(t *testing.T) {
	srv := davServer(t, "alice", "S3cr3t-Pw", map[string][]string{
		"/":                 {"movie.mkv", "docs/"},
		"/docs":             {"readme.txt", "Wait.. Live/"},
		"/docs/Wait.. Live": {"track.flac"},
	})
	h := newAPIHarness(t, true)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "dav1", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "S3cr3t-Pw", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	rootID := int64(body["id"].(float64))

	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": rootID})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "completed", st["status"], "reason: %v", st["reason"])
	assert.Equal(t, float64(5), st["files_found"])

	got := filesIn(t, h.db, "dav1")
	for _, p := range []string{"movie.mkv", "docs", "docs/readme.txt", "docs/Wait.. Live", "docs/Wait.. Live/track.flac"} {
		assert.True(t, got[p], "row %q missing; have %v", p, got)
	}
	assert.Len(t, got, 5, "exactly one row per entry, no leading-slash duplicates")
}

func TestWF22_UserPath_WebDAVListingFailureReasonHasNoCredential(t *testing.T) {
	// a server that accepts the connection test (Depth 0) and then fails every listing: the client error names the URL of the listing
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = io.WriteString(w, `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	h := newAPIHarness(t, true)
	// the password also sits in the URL userinfo of this root: it must not come back through GET /scans/:id
	u := strings.Replace(srv.URL, "http://", "http://alice:wrong-password@", 1)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "dav2", "protocol": "webdav", "url": u, "username": "alice", "password": "wrong-password"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": int64(body["id"].(float64))})
	require.Equal(t, http.StatusAccepted, code)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "failed", st["status"])
	reason, _ := st["reason"].(string)
	assert.Contains(t, reason, "status 500", "control: the failure happened at the listing, whose error names the URL")
	assert.NotContains(t, reason, "wrong-password", "G01+R9: reason is in the API and never carries a credential")
	_, list := h.do("GET", "/scans", nil)
	assert.NotContains(t, fmt.Sprint(list), "wrong-password")
	// and the credential-bearing url is never returned by the root listing
	_, roots := h.do("GET", "/storage/roots", nil)
	assert.NotContains(t, fmt.Sprint(roots), "wrong-password")
	assert.Contains(t, fmt.Sprint(roots), "xxxxx", "the url is returned redacted")
}

func TestWF22_UserPath_WebDAVWithAWrongPasswordFailsAtTheConnection(t *testing.T) {
	srv := davServer(t, "alice", "right-password", map[string][]string{"/": {"a"}})
	h := newAPIHarness(t, true)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "dav3", "protocol": "webdav", "url": srv.URL, "username": "alice", "password": "wrong-password"})
	require.Equal(t, http.StatusCreated, code)
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": int64(body["id"].(float64))})
	require.Equal(t, http.StatusAccepted, code)
	st := h.waitScan(body["job_id"].(string))
	assert.Equal(t, "failed", st["status"])
	assert.Contains(t, st["reason"], "401")
	assert.NotContains(t, fmt.Sprint(st), "wrong-password")
}

func TestWF22_R3_AllowEmptyIsAPerRootSettingThatDefaultsToFalse(t *testing.T) {
	srv := davServer(t, "u", "p", map[string][]string{"/": {}})
	h := newAPIHarness(t, true)
	mk := func(name string, allow interface{}) string {
		req := map[string]interface{}{"name": name, "protocol": "webdav", "url": srv.URL, "username": "u", "password": "p"}
		if allow != nil {
			req["allow_empty"] = allow
		}
		code, body := h.do("POST", "/storage/roots", req)
		require.Equal(t, http.StatusCreated, code, "%v", body)
		code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": int64(body["id"].(float64))})
		require.Equal(t, http.StatusAccepted, code)
		return body["job_id"].(string)
	}
	st := h.waitScan(mk("empty-default", nil))
	assert.Equal(t, "failed", st["status"], "default: an empty answer is a failure")
	assert.Contains(t, st["reason"], "empty_root")
	assert.Contains(t, st["reason"], "allow_empty", "and the reason says how to declare the share empty")

	st = h.waitScan(mk("empty-allowed", true))
	assert.Equal(t, "completed", st["status"], "allow_empty=true: the share is expected to be empty")

	_, roots := h.do("GET", "/storage/roots", nil)
	var seen = map[string]interface{}{}
	for _, r := range roots["roots"].([]interface{}) {
		m := r.(map[string]interface{})
		seen[m["name"].(string)] = m["allow_empty"]
	}
	assert.Equal(t, false, seen["empty-default"])
	assert.Equal(t, true, seen["empty-allowed"])
}

// sftp / ftps / nfs3 style protocols: registered once, then creatable and scannable through the API with no further code.
type staticClient struct {
	tree map[string][]*filesystem.FileInfo
}

func (c *staticClient) Connect(context.Context) error        { return nil }
func (c *staticClient) Disconnect(context.Context) error     { return nil }
func (c *staticClient) IsConnected() bool                    { return true }
func (c *staticClient) TestConnection(context.Context) error { return nil }
func (c *staticClient) GetProtocol() string                  { return "sftp" }
func (c *staticClient) GetConfig() interface{}               { return nil }
func (c *staticClient) ReadFile(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}
func (c *staticClient) WriteFile(context.Context, string, io.Reader) error {
	return fmt.Errorf("read only")
}
func (c *staticClient) GetFileInfo(context.Context, string) (*filesystem.FileInfo, error) {
	return &filesystem.FileInfo{}, nil
}
func (c *staticClient) FileExists(context.Context, string) (bool, error) { return true, nil }
func (c *staticClient) DeleteFile(context.Context, string) error         { return fmt.Errorf("read only") }
func (c *staticClient) CopyFile(context.Context, string, string) error {
	return fmt.Errorf("read only")
}
func (c *staticClient) CreateDirectory(context.Context, string) error { return fmt.Errorf("read only") }
func (c *staticClient) DeleteDirectory(context.Context, string) error { return fmt.Errorf("read only") }
func (c *staticClient) ListDirectory(_ context.Context, p string) ([]*filesystem.FileInfo, error) {
	return c.tree[strings.TrimRight(p, "/")+"/"], nil
}

func TestWF22_H2_RegisteredProtocolIsCreatableAndScannableThroughTheAPI(t *testing.T) {
	var gotSettings map[string]interface{}
	require.NoError(t, filesystem.RegisterProtocol(filesystem.ProtocolSpec{
		Name: "sftp",
		Keys: map[string]filesystem.KeyType{filesystem.KeyHost: filesystem.KeyString, filesystem.KeyPort: filesystem.KeyInt, filesystem.KeyPath: filesystem.KeyString,
			filesystem.KeyUsername: filesystem.KeyString, filesystem.KeyPassword: filesystem.KeyString},
		New: func(s map[string]interface{}) (filesystem.FileSystemClient, error) {
			gotSettings = s
			return &staticClient{tree: map[string][]*filesystem.FileInfo{
				"/":     {{Name: "a.bin", Size: 3}, {Name: "sub", IsDir: true}},
				"/sub/": {{Name: "b.bin", Size: 4}},
			}}, nil
		},
	}))
	t.Cleanup(func() { filesystem.UnregisterProtocol("sftp") })
	h := newAPIHarness(t, true)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "sftp1", "protocol": "sftp", "host": "nas", "port": 22, "path": "/data", "username": "u", "password": "p"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, body = h.do("POST", "/scans", map[string]interface{}{"storage_root_id": int64(body["id"].(float64))})
	require.Equal(t, http.StatusAccepted, code, "%v", body)
	st := h.waitScan(body["job_id"].(string))
	require.Equal(t, "completed", st["status"], "reason: %v", st["reason"])
	assert.Equal(t, map[string]bool{"a.bin": true, "sub": true, "sub/b.bin": true}, filesIn(t, h.db, "sftp1"))
	assert.Equal(t, "nas", gotSettings["host"])
	assert.Equal(t, 22, gotSettings["port"])
	assert.Equal(t, "/data", gotSettings["path"])
}

func TestWF22_H2_NFSRootKeepsItsMountPointAndOptionsAndBuildsAClient(t *testing.T) {
	h := newAPIHarness(t, false)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "nfs1", "protocol": "nfs", "host": "nas", "path": "/export/media", "mount_point": "/mnt/nas", "options": "vers=3,ro", "allow_empty": true})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	root, err := h.handle.loadStorageRoot(context.Background(), int64(body["id"].(float64)))
	require.NoError(t, err)
	require.NotNil(t, root.MountPoint)
	assert.Equal(t, "/mnt/nas", *root.MountPoint)
	require.NotNil(t, root.Options)
	assert.Equal(t, "vers=3,ro", *root.Options)
	assert.True(t, root.AllowEmpty)
	assert.True(t, root.Enabled)
	settings := filesystem.SettingsFromRoot(root, nil)
	require.NoError(t, filesystem.ValidateSettings("nfs", settings))
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "nfs", Settings: settings})
	require.NoError(t, err)
	assert.Equal(t, "nfs", c.GetProtocol())
}

func TestWF22_H2_CreateStorageRootRefusesWhatCouldNeverBeScanned(t *testing.T) {
	h := newAPIHarness(t, false)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "w0", "protocol": "webdav"})
	assert.Equal(t, http.StatusBadRequest, code, "a webdav root without a url: %v", body)
	for _, port := range []int{0, -1, 65536} {
		code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": fmt.Sprintf("p%d", port), "protocol": "ftp", "host": "h", "port": port})
		assert.Equal(t, http.StatusBadRequest, code, "port %d: %v", port, body)
	}
	code, body = h.do("POST", "/storage/roots", map[string]interface{}{"name": "g0", "protocol": "gopher"})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Contains(t, fmt.Sprint(body["accepted"]), "webdav", "the accepted list is the factory's")
	assert.Contains(t, fmt.Sprint(body["accepted"]), "local")
	code, _ = h.do("POST", "/storage/roots", map[string]interface{}{"name": "ok1", "protocol": "ftp", "host": "h", "port": 21})
	assert.Equal(t, http.StatusCreated, code)
}

func TestWF22_H1_LoaderSelectsEveryColumnTheSettingsContractConsumes(t *testing.T) {
	h := newAPIHarness(t, false)
	_, err := h.db.Exec(`INSERT INTO storage_roots (name, protocol, host, port, path, username, password, domain, url, mount_point, options, allow_empty, enabled, max_depth)
		VALUES ('all','webdav','nas',8443,'/p','u','pw','DOM','https://nas/dav','/mnt/x','vers=4',1,0,7)`)
	require.NoError(t, err)
	var id int64
	require.NoError(t, h.db.QueryRow(`SELECT id FROM storage_roots WHERE name='all'`).Scan(&id))
	r, err := h.handle.loadStorageRoot(context.Background(), id)
	require.NoError(t, err)
	want := models.StorageRoot{ID: id, Name: "all", Protocol: "webdav"}
	assert.Equal(t, want.Name, r.Name)
	assert.Equal(t, "nas", *r.Host)
	assert.Equal(t, 8443, *r.Port)
	assert.Equal(t, "/p", *r.Path)
	assert.Equal(t, "u", *r.Username)
	assert.Equal(t, "pw", *r.Password)
	assert.Equal(t, "DOM", *r.Domain)
	assert.Equal(t, "https://nas/dav", *r.URL)
	assert.Equal(t, "/mnt/x", *r.MountPoint)
	assert.Equal(t, "vers=4", *r.Options)
	assert.True(t, r.AllowEmpty)
	assert.False(t, r.Enabled)
	assert.Equal(t, 7, r.MaxDepth)
	_, err = h.handle.loadStorageRoot(context.Background(), id+1000)
	assert.Error(t, err)
}

func TestWF22_G01_ScanStatusJSONCarriesTheReason(t *testing.T) {
	h := newAPIHarness(t, false)
	mock := h.handle.scanner.(*mockUniversalScanner)
	st := &services.ScanStatus{JobID: "j1", StorageRootName: "r", Protocol: "ftp", StartTime: time.Now(), Status: "failed"}
	mock.activeStatuses["j1"] = st
	st2 := &services.ScanStatus{JobID: "j2", StorageRootName: "r", Protocol: "ftp", StartTime: time.Now(), Status: "completed"}
	mock.activeStatuses["j2"] = st2
	// set the reason through the exported snapshot path: the status fails through the scanner; here we use the package-visible constructor of a failed scan
	us := services.NewUniversalScanner(nil, zap.NewNop(), nil, &failingFactory{})
	defer us.Stop()
	job := services.ScanJob{ID: "jf", Path: "/", MaxDepth: 3, Context: context.Background(), StorageRoot: &models.StorageRoot{Name: "jf", Protocol: "ftp"}}
	require.NoError(t, us.Start())
	require.NoError(t, us.QueueScan(job))
	var real *services.ScanStatus
	for i := 0; i < 200; i++ {
		if s, ok := us.GetActiveScanStatus("jf"); ok && s.GetSnapshot().Status == "failed" {
			real = s
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NotNil(t, real)
	mock.activeStatuses["jf"] = real
	code, body := h.do("GET", "/scans/jf", nil)
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "failed", body["status"])
	assert.Contains(t, body["reason"], "unsupported protocol", "the API field reason: why the scan failed")
	code, body = h.do("GET", "/scans/j2", nil)
	require.Equal(t, http.StatusOK, code)
	reason, present := body["reason"]
	assert.True(t, present, "the field is always present")
	assert.Equal(t, "", reason)
	_, list := h.do("GET", "/scans", nil)
	assert.Contains(t, fmt.Sprint(list["scans"]), "unsupported protocol")
}

type failingFactory struct{}

func (failingFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return nil, fmt.Errorf("unsupported protocol: ftp (test)")
}
func (failingFactory) SupportedProtocols() []string { return nil }

// R1 gate, handlers half: the comic-page and PDF-page handlers build their settings with comicStorageRootSettings; every protocol's root must
// produce settings the strict factory accepts and delivers to the client.
func TestWF22_ComicAndPDFSettingsGoThroughTheSingleMapping(t *testing.T) {
	str := func(s string) *string { return &s }
	port := 2121
	roots := map[string]*models.StorageRoot{
		"local":  {Protocol: "local", Path: str("/data/x")},
		"smb":    {Protocol: "smb", Host: str("smb.example"), Port: &port, Path: str("share"), Username: str("u"), Password: str("p"), Domain: str("DOM")},
		"ftp":    {Protocol: "ftp", Host: str("ftp.example"), Port: &port, Path: str("/pub"), Username: str("u"), Password: str("p")},
		"nfs":    {Protocol: "nfs", Host: str("nfs.example"), Path: str("/export"), MountPoint: str("/mnt/x"), Options: str("vers=3")},
		"webdav": {Protocol: "webdav", URL: str("https://dav.example/dav"), Path: str("/media"), Username: str("u"), Password: str("p")},
	}
	f := filesystem.NewDefaultClientFactory()
	for proto, root := range roots {
		s := comicStorageRootSettings(root)
		require.NoError(t, filesystem.ValidateSettings(proto, s), proto)
		c, err := f.CreateClient(&filesystem.StorageConfig{Protocol: proto, Settings: s})
		require.NoError(t, err, proto)
		assert.Equal(t, proto, c.GetProtocol())
	}
	assert.Equal(t, "/pub", f2(t, f, "ftp", comicStorageRootSettings(roots["ftp"])).(*filesystem.FTPConfig).Path)
	assert.Equal(t, "https://dav.example/dav", f2(t, f, "webdav", comicStorageRootSettings(roots["webdav"])).(*filesystem.WebDAVConfig).URL)
}

func f2(t *testing.T, f *filesystem.DefaultClientFactory, proto string, s map[string]interface{}) interface{} {
	t.Helper()
	c, err := f.CreateClient(&filesystem.StorageConfig{Protocol: proto, Settings: s})
	require.NoError(t, err)
	return c.GetConfig()
}
