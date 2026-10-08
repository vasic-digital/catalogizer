package filesystem

// WF22 fix round 2: tests added for the reviewer's findings and for every reviewer mutant that survived the author's tests. Each test names the
// finding it pins (F1-F5, R9-R11, R16, R18, W1, X1, C02, C09, D02, E03, E07) and fails on the code the review was written against.

import (
	"context"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- settings contract ------------------------------------------------------------------------------------------------------------------

func TestWF22_F1_PortAndIntegerRanges(t *testing.T) {
	ok := []interface{}{1, 21, 65535, int32(2121), int64(445), float64(990)}
	for _, p := range ok {
		assert.NoError(t, ValidateSettings("ftp", map[string]interface{}{"port": p}), "port %v", p)
	}
	for _, p := range []interface{}{0, -1, 65536, int64(1) << 40, 1e300, -1e300, 21.5, float64(70000)} {
		err := ValidateSettings("ftp", map[string]interface{}{"port": p})
		require.Error(t, err, "port %v", p)
		var se *SettingsError
		require.ErrorAs(t, err, &se)
		assert.Len(t, se.WrongType, 1)
		assert.NotContains(t, err.Error(), fmt.Sprint(p), "a value is never echoed")
	}
	// the read side never returns garbage either
	assert.Equal(t, 21, IntSetting(map[string]interface{}{"port": 1e300}, "port", 21))
	assert.Equal(t, 21, IntSetting(map[string]interface{}{"port": 21.5}, "port", 21))
}

func TestWF22_D02_FactoryReadsEveryIntegerSpelling(t *testing.T) {
	cases := map[string]interface{}{"int": 2121, "int32": int32(2121), "int64": int64(2121), "float64": float64(2121)}
	for name, v := range cases {
		assert.Equal(t, 2121, getIntSetting(map[string]interface{}{"port": v}, "port", 21), name)
		c, err := NewDefaultClientFactory().CreateClient(&StorageConfig{Protocol: "ftp", Settings: map[string]interface{}{"host": "h", "port": v}})
		require.NoError(t, err, name)
		assert.Equal(t, 2121, c.GetConfig().(*FTPConfig).Port, "%s: the port the client dials", name)
	}
	assert.Equal(t, 21, getIntSetting(map[string]interface{}{}, "port", 21))
}

func TestWF22_C09_LegacyHintOnlyWhenTheHintedKeyBelongsToTheProtocol(t *testing.T) {
	err := ValidateSettings("ftp", map[string]interface{}{"export_path": "/x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export_path (use path)", "ftp has a path key: the hint is useful")
	err = ValidateSettings("smb", map[string]interface{}{"export_path": "/x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "export_path")
	assert.NotContains(t, err.Error(), "use path", "smb has no path key (its root is share): the hint would send the user to another wrong key")
}

func TestWF22_C02_F4_SMBDomainColumn(t *testing.T) {
	h, dom, empty := "nas", "CORP", ""
	ident := func(*models.StorageRoot) (string, string, string) { return "u", "p", "IDDOM" }
	assert.Equal(t, "CORP", SettingsFromRoot(&models.StorageRoot{Protocol: "smb", Host: &h, Domain: &dom}, ident)[KeyDomain], "a non-empty domain column wins")
	assert.Equal(t, "IDDOM", SettingsFromRoot(&models.StorageRoot{Protocol: "smb", Host: &h, Domain: &empty}, ident)[KeyDomain], "an empty column does not erase the identity's domain")
	_, has := SettingsFromRoot(&models.StorageRoot{Protocol: "smb", Host: &h, Domain: &empty}, nil)[KeyDomain]
	assert.False(t, has, "and without any domain the key is absent, so the factory's WORKGROUP default applies")
}

func TestWF22_StoredPortZeroMeansUnset(t *testing.T) {
	h, z := "nas", 0
	s := SettingsFromRoot(&models.StorageRoot{Protocol: "ftp", Host: &h, Port: &z}, nil)
	_, has := s[KeyPort]
	assert.False(t, has)
	require.NoError(t, ValidateSettings("ftp", s))
}

// ---- WebDAV client ----------------------------------------------------------------------------------------------------------------------

func TestWF22_F5_RootPathIsJoinedUnderTheURLPath(t *testing.T) {
	for _, tc := range []struct{ url, path, in, want string }{
		{"https://nas.example/remote.php/dav/files/alice", "/Movies", "/x.mkv", "https://nas.example/remote.php/dav/files/alice/Movies/x.mkv"},
		{"https://nas.example/remote.php/dav/files/alice/", "Movies", "/x.mkv", "https://nas.example/remote.php/dav/files/alice/Movies/x.mkv"},
		{"https://nas.example/dav/My%20Files", "/Movies", "/a b.mkv", "https://nas.example/dav/My%20Files/Movies/a%20b.mkv"},
		{"https://nas.example", "/Movies", "/x.mkv", "https://nas.example/Movies/x.mkv"},
		{"https://nas.example/dav", "/", "/x.mkv", "https://nas.example/dav/x.mkv"},
		{"https://nas.example/dav", "", "/x.mkv", "https://nas.example/dav/x.mkv"},
	} {
		c, err := NewWebDAVClient(&WebDAVConfig{URL: tc.url, Path: tc.path})
		require.NoError(t, err)
		assert.Equal(t, tc.want, c.resolveURL(tc.in), "url=%s path=%s", tc.url, tc.path)
	}
}

func TestWF22_R11_DotDotInANameIsKeptDotDotSegmentIsClamped(t *testing.T) {
	c, err := NewWebDAVClient(&WebDAVConfig{URL: "https://nas.example/dav"})
	require.NoError(t, err)
	assert.Equal(t, "https://nas.example/dav/Wait..%20Live/track.flac", c.resolveURL("/Wait.. Live/track.flac"))
	assert.Equal(t, "https://nas.example/dav/...And%20Justice%20for%20All/01.flac", c.resolveURL("/...And Justice for All/01.flac"))
	assert.Equal(t, "https://nas.example/dav/a..b", c.resolveURL("a..b"))
	assert.Equal(t, "https://nas.example/dav/etc/passwd", c.resolveURL("/../../etc/passwd"), "a real traversal segment cannot climb out of the base")
	assert.Equal(t, "https://nas.example/dav/b", c.resolveURL("/a/../b"))
	assert.Equal(t, "https://nas.example/dav", c.resolveURL(""))
}

func TestWF22_R9_NoErrorOfTheWebDAVClientCarriesACredential(t *testing.T) {
	const secret = "S3cr3t-Pw-Probe"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	withUser := strings.Replace(srv.URL, "http://", "http://alice:"+secret+"@", 1)
	c, err := NewWebDAVClient(&WebDAVConfig{URL: withUser})
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))
	errs := map[string]error{}
	_, errs["ReadFile"] = c.ReadFile(ctx, "/f")
	errs["WriteFile"] = c.WriteFile(ctx, "/f", strings.NewReader("x"))
	_, errs["GetFileInfo"] = c.GetFileInfo(ctx, "/f")
	_, errs["ListDirectory"] = c.ListDirectory(ctx, "/d")
	errs["CreateDirectory"] = c.CreateDirectory(ctx, "/d")
	errs["DeleteDirectory"] = c.DeleteDirectory(ctx, "/d")
	errs["DeleteFile"] = c.DeleteFile(ctx, "/f")
	errs["CopyFile"] = c.CopyFile(ctx, "/f", "/g")
	for name, e := range errs {
		require.Error(t, e, name)
		assert.NotContains(t, e.Error(), secret, "%s: the URL userinfo password must not reach an error (it ends up in the scan reason)", name)
	}
	// transport failures of every method: the server that answered above is gone (the error then names the URL of the request)
	srv.Close()
	terrs := map[string]error{}
	_, terrs["ReadFile"] = c.ReadFile(ctx, "/f")
	terrs["WriteFile"] = c.WriteFile(ctx, "/f", strings.NewReader("x"))
	_, terrs["GetFileInfo"] = c.GetFileInfo(ctx, "/f")
	_, terrs["ListDirectory"] = c.ListDirectory(ctx, "/d")
	_, terrs["FileExists"] = c.FileExists(ctx, "/f")
	terrs["CreateDirectory"] = c.CreateDirectory(ctx, "/d")
	terrs["DeleteDirectory"] = c.DeleteDirectory(ctx, "/d")
	terrs["DeleteFile"] = c.DeleteFile(ctx, "/f")
	terrs["CopyFile"] = c.CopyFile(ctx, "/f", "/g")
	for name, e := range terrs {
		require.Error(t, e, "transport "+name)
		assert.Contains(t, e.Error(), "127.0.0.1", "control: the transport error does name the host of the request (%s)", name)
		assert.NotContains(t, e.Error(), secret, "transport %s: the URL userinfo password must not reach an error", name)
	}
	// transport failure: the host does not answer
	dead := strings.Replace(withUser, srv.URL[len("http://"):], "127.0.0.1:1", 1)
	c2, err := NewWebDAVClient(&WebDAVConfig{URL: dead})
	require.NoError(t, err)
	cerr := c2.Connect(ctx)
	require.Error(t, cerr)
	assert.NotContains(t, cerr.Error(), secret)
	// an unparsable URL: the error does not echo it
	_, err = NewWebDAVClient(&WebDAVConfig{URL: "http://alice:" + secret + "@host:badport/"})
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, redactURL("http://alice:"+secret+"@host:badport/"), secret)
	assert.Equal(t, "http://alice:xxxxx@nas/dav", redactURL("http://alice:"+secret+"@nas/dav"))
}

// propfindServer answers every PROPFIND Depth:1 with n children of /d/, streaming them, and counts the bytes it managed to write.
func propfindServer(t *testing.T, n int, written *int64) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" && r.Header.Get("Depth") == "0" {
			w.WriteHeader(http.StatusMultiStatus)
			_, _ = w.Write([]byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"></D:multistatus>`))
			return
		}
		w.WriteHeader(http.StatusMultiStatus)
		put := func(s string) {
			k, _ := io.WriteString(w, s)
			atomic.AddInt64(written, int64(k))
		}
		put(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/d/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>`)
		for i := 0; i < n; i++ {
			put(fmt.Sprintf(`<D:response><D:href>/d/f%07d</D:href><D:propstat><D:prop><D:getcontentlength>1</D:getcontentlength><D:resourcetype/></D:prop></D:propstat></D:response>`, i))
			if f, ok := w.(http.Flusher); ok && i%500 == 0 {
				f.Flush()
			}
		}
		put(`</D:multistatus>`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWF22_W1_AnswerLargerThanTheByteBoundIsAnErrorNotATruncation(t *testing.T) {
	var written int64
	srv := propfindServer(t, 5000, &written)
	c, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	old := MaxPropfindBytes
	MaxPropfindBytes = 20_000
	defer func() { MaxPropfindBytes = old }()
	out, err := c.ListDirectory(context.Background(), "/d")
	require.Error(t, err, "5000 entries are far more than 20 kB: the listing would be incomplete")
	assert.Nil(t, out)
	assert.ErrorIs(t, err, errPropfindTooLarge)
	assert.Contains(t, err.Error(), "bound 20000 bytes")

	MaxPropfindBytes = 64 << 20
	out, err = c.ListDirectory(context.Background(), "/d")
	require.NoError(t, err)
	assert.Len(t, out, 5000, "control: within the bound the full listing comes back")
}

func TestWF22_R4_ListLimitStopsDecodingEarly(t *testing.T) {
	var written int64
	srv := propfindServer(t, 20000, &written)
	c, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	old := MaxPropfindBytes
	MaxPropfindBytes = 50_000 // the whole answer is ~4 MB: reading it all would trip the bound
	defer func() { MaxPropfindBytes = old }()
	out, err := c.ListDirectory(WithListLimit(context.Background(), 7), "/d")
	require.NoError(t, err, "the limit is satisfied long before the byte bound")
	assert.Len(t, out, 7)
	assert.Equal(t, "f0000000", out[0].Name)
	l, ok := ListLimit(WithListLimit(context.Background(), 7))
	assert.True(t, ok)
	assert.Equal(t, 7, l)
	_, ok = ListLimit(context.Background())
	assert.False(t, ok)
	_, ok = ListLimit(WithListLimit(context.Background(), 0))
	assert.False(t, ok, "a limit below 1 is no limit")
}

// ---- PROPFIND parser --------------------------------------------------------------------------------------------------------------------

func TestWF22_R10_NameIsTheHrefSegmentNotTheDisplayName(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/dav/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/Docs/</D:href><D:propstat><D:prop><D:displayname>My Documents</D:displayname><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`<D:response><D:href>/dav/a%20b.txt</D:href><D:propstat><D:prop><D:displayname>presentation</D:displayname><D:getcontentlength>3</D:getcontentlength><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`</D:multistatus>`)
	out, err := parsePropfind(body, "http://nas/dav/")
	require.NoError(t, err)
	require.Len(t, out, 2)
	assert.Equal(t, "Docs", out[0].Name)
	assert.True(t, out[0].IsDir)
	assert.Equal(t, "a b.txt", out[1].Name, "decoded href segment")
	assert.Equal(t, "a b.txt", out[1].Path)
}

func TestWF22_E03_UnparsableOrMissingTimeIsTheZeroTime(t *testing.T) {
	assert.True(t, parseDAVTime("").IsZero())
	assert.True(t, parseDAVTime("not a date").IsZero(), "never time.Now(): the value feeds change detection")
	assert.False(t, parseDAVTime("Tue, 14 Nov 2023 22:13:20 GMT").IsZero())
	body := []byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:"><D:response><D:href>/x/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/x/f</D:href><D:propstat><D:prop><D:getlastmodified>garbage</D:getlastmodified><D:resourcetype/></D:prop></D:propstat></D:response></D:multistatus>`)
	out, err := parsePropfind(body, "http://nas/x/")
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.True(t, out[0].ModTime.IsZero())
}

func TestWF22_ParserRejectsNonXMLAndBodiesWithoutAnyElement(t *testing.T) {
	_, err := parsePropfind([]byte(``), "http://nas/x/")
	assert.Error(t, err)
	_, err = parsePropfind([]byte(`<<<not xml`), "http://nas/x/")
	assert.Error(t, err)
	out, err := parsePropfind([]byte(`<D:multistatus xmlns:D="DAV:"></D:multistatus>`), "http://nas/x/")
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestWF22_F3_OnlyDirectChildrenAreListed(t *testing.T) {
	body := []byte(`<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/dav/movies/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/movies/a.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/movies/sub/deep.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>http://other.example/dav/movies/b.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`<D:response><D:href>/dav/other/c.mkv</D:href><D:propstat><D:prop><D:resourcetype/></D:prop></D:propstat></D:response>` +
		`</D:multistatus>`)
	out, err := parsePropfind(body, "http://nas/dav/movies/")
	require.NoError(t, err)
	var names []string
	for _, f := range out {
		names = append(names, f.Name)
	}
	assert.Equal(t, []string{"a.mkv", "b.mkv"}, names, "a deeper entry and an entry of another collection are not children (an absolute href is judged by its path)")
}

// ---- FTP client -------------------------------------------------------------------------------------------------------------------------

func TestWF22_R18_FTPResolvePath(t *testing.T) {
	c := NewFTPClient(&FTPConfig{Path: "movies"})
	assert.Equal(t, "/movies/a.mkv", c.resolvePath("/a.mkv"), "not connected: the relative base is anchored at /")
	assert.Equal(t, "/movies", c.resolvePath("/"))
	c.base = "/home/ftp/movies" // what Connect records from PWD after the CWD
	assert.Equal(t, "/home/ftp/movies/a.mkv", c.resolvePath("/a.mkv"), "the CWD result, not the configured text, is the base: it is not applied twice")
	assert.Equal(t, "/home/ftp/movies", c.resolvePath("/"))
	assert.Equal(t, "/home/ftp/movies/x/y", c.resolvePath("x/y"))
	assert.Equal(t, "/a.mkv", NewFTPClient(&FTPConfig{}).resolvePath("/a.mkv"), "no base path: untouched")
	assert.Equal(t, "/movies/a.mkv", NewFTPClient(&FTPConfig{Path: "/movies"}).resolvePath("/a.mkv"))
}

// ---- X1: remote paths are slash paths ----------------------------------------------------------------------------------------------------

// TestWF22_X1_RemotePathCodeDoesNotUseOSPathFunctions closes the class "path/filepath on a remote path" (Windows builds exist): the clients of
// REMOTE protocols and the scanners use package path. Local file systems (local client, the NFS client's local mount) may use filepath.
func TestWF22_X1_RemotePathCodeDoesNotUseOSPathFunctions(t *testing.T) {
	importsFilepath := func(file string) bool {
		f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
		require.NoError(t, err, file)
		for _, imp := range f.Imports {
			if imp.Path.Value == `"path/filepath"` {
				return true
			}
		}
		return false
	}
	// control needle: the instrument sees an import that is there
	require.True(t, importsFilepath("local_client.go"), "control: the local client does import path/filepath")
	allowed := map[string]bool{"local_client.go": true, "nfs_client.go": true, "nfs_client_darwin.go": true}
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		checked++
		if importsFilepath(f) && !allowed[f] {
			t.Errorf("%s imports path/filepath, but handles remote slash paths: use package path (WF22 X1)", f)
		}
	}
	require.Greater(t, checked, 8)
	_, err = os.Stat("ftp_client.go")
	require.NoError(t, err)
}

func TestWF22_F1_WholeNumberBranches(t *testing.T) {
	ok := map[string]interface{}{"int": 21, "int32": int32(21), "int64": int64(21), "float64": float64(21), "min32": int64(math.MinInt32), "max32": int64(math.MaxInt32), "fmax32": float64(math.MaxInt32)}
	for name, v := range ok {
		n, good := wholeNumber(v)
		assert.True(t, good, name)
		assert.NotZero(t, n, name)
	}
	bad := map[string]interface{}{
		"int64 above int32": int64(math.MaxInt32) + 1, "int64 below int32": int64(math.MinInt32) - 1, "int huge": int(1) << 40,
		"float above": float64(math.MaxInt32) + 1, "float below": float64(math.MinInt32) - 1, "float huge": 1e300, "float -huge": -1e300,
		"float fraction": 21.5, "NaN": math.NaN(), "+Inf": math.Inf(1), "-Inf": math.Inf(-1), "string": "21", "nil": nil, "bool": true,
	}
	for name, v := range bad {
		_, good := wholeNumber(v)
		assert.False(t, good, name)
	}
}
