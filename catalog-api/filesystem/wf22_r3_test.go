//go:build linux

package filesystem

// WF22 round 3 (structural fix round, 11.4.276(E)): class inventories + the reviewer's round-2 probes adopted verbatim in their assertions.
//
//   class A  path containment  - every protocol client's path-taking method resolves its path through the ONE confinement rule (ConfinePath),
//            proven by a source classifier (with a control needle) and by behaviour on every client;
//   class C  the single settings mapping - create-time ROW validation (ValidateRoot), strict options (nothing dropped silently);
//   N1       NFS Connect really detects an NFS mount at the mount point (the stub that always said "mounted" is gone).

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func s3(v string) *string { return &v }

// ---------------------------------------------------------------- class A: path containment

func TestWF22R3_A_ConfinePath_Table(t *testing.T) {
	for in, want := range map[string]string{
		"":                    "/",
		".":                   "/",
		"/":                   "/",
		"a/b":                 "/a/b",
		"/a/b/":               "/a/b",
		"/../../etc":          "/etc",
		"../../etc/passwd":    "/etc/passwd",
		"/data/../../../root": "/root",
		"/Wait.. Live":        "/Wait.. Live",
		"/...And Justice":     "/...And Justice",
		"a/./b//c":            "/a/b/c",
	} {
		assert.Equal(t, want, ConfinePath(in), "ConfinePath(%q)", in)
	}
	assert.Equal(t, "/srv/movies/etc", JoinConfined("/srv/movies", "/../../etc"))
	assert.Equal(t, "/srv/movies", JoinConfined("/srv/movies", ""))
}

// N2 (review P3): an FTP path is confined to the root's base like the WebDAV one.
func TestWF22R3_N2_FTPPathConfinedToBase(t *testing.T) {
	c := &FTPClient{config: &FTPConfig{Path: "/srv/movies"}, base: "/srv/movies"}
	assert.Equal(t, "/srv/movies/etc", c.resolvePath("/../../etc"))
	assert.Equal(t, "/srv/movies/Wait.. Live", c.resolvePath("/Wait.. Live"))
	assert.Equal(t, "/srv/movies", c.resolvePath("/"))
	// not connected through Connect: the configured path, made absolute, is the base
	u := &FTPClient{config: &FTPConfig{Path: "movies"}}
	assert.Equal(t, "/movies/etc", u.resolvePath("../../etc"))
}

// N7 (review P2/P2b): a name containing ".." is a name; a ".." segment cannot leave the base - on NFS and on local.
func TestWF22R3_N7_NFSAndLocalKeepDotDotNamesAndConfineSegments(t *testing.T) {
	mp := t.TempDir()
	d := filepath.Join(mp, "Wait.. Live")
	require.NoError(t, os.MkdirAll(d, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(d, "01.flac"), []byte("x"), 0o644))
	plain := filepath.Join(mp, "Wait Live")
	require.NoError(t, os.MkdirAll(plain, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(plain, "01.flac"), []byte("x"), 0o644))
	ctx := context.Background()

	n := &NFSClient{mountPoint: mp, connected: true, mounted: true, config: NFSConfig{MountPoint: mp}}
	// IsConnected on the pre-fix shape asserted the stub; the listing is what matters here, so serve it through a mounts file naming mp as NFS
	withProcMounts(t, "nas:/export "+mp+" nfs4 rw 0 0\n")
	ents, err := n.ListDirectory(ctx, "/Wait.. Live")
	require.NoError(t, err, "nfs resolvePath=%q", n.resolvePath("/Wait.. Live"))
	require.Len(t, ents, 1)
	pe, err := n.ListDirectory(ctx, "/Wait Live")
	require.NoError(t, err, "control: the plain name lists")
	require.Len(t, pe, 1)
	assert.Equal(t, filepath.Join(mp, "etc"), n.resolvePath("/../../etc"))

	l := NewLocalClient(&LocalConfig{BasePath: mp})
	require.NoError(t, l.Connect(ctx))
	le, err := l.ListDirectory(ctx, "/Wait.. Live")
	require.NoError(t, err, "local resolvePath=%q", l.resolvePath("/Wait.. Live"))
	require.Len(t, le, 1)
	assert.Equal(t, filepath.Join(mp, "etc", "passwd"), l.resolvePath("../../etc/passwd"))
}

func TestWF22R3_A_WebDAVAndSMBConfine(t *testing.T) {
	w, err := NewWebDAVClient(&WebDAVConfig{URL: "https://nas/dav", Path: "/movies"})
	require.NoError(t, err)
	assert.Equal(t, "https://nas/dav/movies/etc", w.resolveURL("/../../etc"))
	assert.Equal(t, "etc/passwd", smbPath("../../etc/passwd"))
	assert.Equal(t, ".", smbPath("/.."))
	assert.Equal(t, "movies/Wait.. Live", smbPath("movies/Wait.. Live"), "a name containing .. is handed over untouched")
	assert.Equal(t, "movies/a", smbPath("movies/a"))
	assert.Equal(t, "", smbPath(""), "an empty path is passed through exactly as before")
}

// pathParamNames are the parameter names a client method uses for a remote path.
var pathParamNames = map[string]bool{"path": true, "srcPath": true, "dstPath": true, "src": true, "dst": true, "p": true}

// resolverNames are the confining resolvers: a path parameter must be an argument of one of these calls.
var resolverNames = map[string]bool{"resolvePath": true, "resolveURL": true, "smbPath": true, "ConfinePath": true, "JoinConfined": true}

// unconfinedPathMethods is the class-A classifier: for every method of the client types in src, every string parameter that looks like a path
// and is used in the body must be passed to a confining resolver. It returns "Type.Method(param)" for each violation.
func unconfinedPathMethods(t *testing.T, filename, src string, types map[string]bool) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), filename, src, 0)
	require.NoError(t, err)
	var out []string
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Recv.List) == 0 {
			continue
		}
		var recv string
		switch rt := fd.Recv.List[0].Type.(type) {
		case *ast.StarExpr:
			if id, ok := rt.X.(*ast.Ident); ok {
				recv = id.Name
			}
		case *ast.Ident:
			recv = rt.Name
		}
		if !types[recv] || resolverNames[fd.Name.Name] {
			continue
		}
		for _, fld := range fd.Type.Params.List {
			if id, ok := fld.Type.(*ast.Ident); !ok || id.Name != "string" {
				continue
			}
			for _, nm := range fld.Names {
				if !pathParamNames[nm.Name] {
					continue
				}
				used, confined := false, false
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					switch x := n.(type) {
					case *ast.CallExpr:
						name := ""
						switch fn := x.Fun.(type) {
						case *ast.Ident:
							name = fn.Name
						case *ast.SelectorExpr:
							name = fn.Sel.Name
						}
						if resolverNames[name] {
							for _, a := range x.Args {
								if id, ok := a.(*ast.Ident); ok && id.Name == nm.Name {
									confined = true
								}
							}
						}
					case *ast.Ident:
						if x.Name == nm.Name && x.Obj != nil {
							used = true
						}
					}
					return true
				})
				if used && !confined {
					out = append(out, recv+"."+fd.Name.Name+"("+nm.Name+")")
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func TestWF22R3_A_Classifier_EveryClientPathParameterGoesThroughAConfiningResolver(t *testing.T) {
	// control needles: a known violator is flagged, a known compliant method is not, and a method that never uses its parameter is not
	bad := "package x\ntype C struct{}\nfunc (c *C) Open(path string) error { _, err := os.Open(path); return err }\n" +
		"func (c *C) Good(path string) error { _, err := os.Open(c.resolvePath(path)); return err }\n" +
		"func (c *C) Stub(path string) error { return nil }\n" +
		"func (c *C) Two(srcPath, dstPath string) error { a := c.resolvePath(srcPath); _ = a; os.Open(dstPath); return nil }\n"
	got := unconfinedPathMethods(t, "needle.go", bad, map[string]bool{"C": true})
	require.Equal(t, []string{"C.Open(path)", "C.Two(dstPath)"}, got, "control needle: the classifier sees a violator and only the violators")

	types := map[string]bool{"LocalClient": true, "NFSClient": true, "FTPClient": true, "WebDAVClient": true, "SmbClient": true}
	files, err := filepath.Glob("*_client*.go")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(files), 5, "control: the client sources are found")
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		checked++
		assert.Empty(t, unconfinedPathMethods(t, f, string(b), types), "%s: a client method hands a path to the filesystem / server without ConfinePath (WF22 N2/N7)", f)
	}
	assert.GreaterOrEqual(t, checked, 7, "control: linux/darwin/windows NFS + local + ftp + smb + webdav")
}

func TestWF22R3_A_EveryResolverUsesTheSharedRule(t *testing.T) {
	needles := map[string][]string{
		"local_client.go":      {"func (c *LocalClient) resolvePath"},
		"nfs_client.go":        {"func (c *NFSClient) resolvePath"},
		"nfs_client_darwin.go": {"func (c *NFSClient) resolvePath"},
		"ftp_client.go":        {"func (c *FTPClient) resolvePath"},
		"webdav_client.go":     {"func (c *WebDAVClient) resolveURL"},
		"smb_client.go":        {"func smbPath"},
	}
	for file, heads := range needles {
		b, err := os.ReadFile(file)
		require.NoError(t, err)
		src := string(b)
		for _, h := range heads {
			i := strings.Index(src, h)
			require.GreaterOrEqual(t, i, 0, "control: %s has %s", file, h)
			end := strings.Index(src[i:], "\n}\n")
			require.Greater(t, end, 0)
			body := src[i : i+end]
			assert.True(t, strings.Contains(body, "ConfinePath") || strings.Contains(body, "JoinConfined"), "%s %s does not use the shared confinement rule", file, h)
			assert.NotContains(t, body, `ReplaceAll(`, "%s %s still strips '..' out of names", file, h)
		}
	}
}

// ---------------------------------------------------------------- N1: NFS connect

func withProcMounts(t *testing.T, content string) {
	t.Helper()
	old := procMountsPath
	p := filepath.Join(t.TempDir(), "mounts")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	procMountsPath = p
	t.Cleanup(func() { procMountsPath = old })
}

func hasCapSysAdmin() bool {
	b, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "CapEff:") {
			var v uint64
			for _, c := range strings.TrimSpace(strings.TrimPrefix(l, "CapEff:")) {
				v = v*16 + uint64(strings.IndexRune("0123456789abcdef", c))
			}
			return v&(1<<21) != 0
		}
	}
	return false
}

// N1 (review P1, the behaviour that was wrong): a Connect that returns nil must leave a client that can list; Connect on a directory that is
// not an NFS mount must not claim success.
func TestWF22R3_N1_ConnectNeverSucceedsWithoutAMount(t *testing.T) {
	if hasCapSysAdmin() {
		t.Skip("CAP_SYS_ADMIN present: Connect would really try to mount")
	}
	mp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(mp, "a.mkv"), []byte("x"), 0o644))
	c, err := NewNFSClient(NFSConfig{Host: "nas.invalid", Path: "/export", MountPoint: mp})
	require.NoError(t, err)
	withProcMounts(t, "proc /proc proc rw 0 0\n/dev/sda1 / ext4 rw 0 0\n")
	err = c.Connect(context.Background())
	if err == nil {
		_, lerr := c.ListDirectory(context.Background(), "/")
		require.NoError(t, lerr, "Connect returned nil but ListDirectory fails (N1)")
		return
	}
	assert.False(t, c.IsConnected())
}

// N1: a mount point that already IS an NFS mount (mounted by the host, bind-mounted into a rootless container) connects with no privilege and
// lists; Disconnect leaves a mount this client did not make alone.
func TestWF22R3_N1_PreMountedNFSConnectsListsAndIsNotUnmounted(t *testing.T) {
	mp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(mp, "a.mkv"), []byte("x"), 0o644))
	withProcMounts(t, "proc /proc proc rw 0 0\nnas:/export "+mp+" nfs4 rw,vers=4.1 0 0\n")
	c, err := NewNFSClient(NFSConfig{Host: "nas", Path: "/export", MountPoint: mp})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	assert.True(t, c.IsConnected())
	assert.False(t, c.mounted, "this client did not mount it, so it must not unmount it")
	ents, err := c.ListDirectory(context.Background(), "/")
	require.NoError(t, err)
	require.Len(t, ents, 1)
	assert.Equal(t, "/a.mkv", ents[0].Path)
	require.NoError(t, c.Disconnect(context.Background()))
	assert.False(t, c.IsConnected())
}

func TestWF22R3_N1_MountsParser(t *testing.T) {
	mounts := "nas:/a /mnt/with\\040space nfs rw 0 0\nnas:/b /mnt/v4 nfs4 rw 0 0\n/dev/x /mnt/ext ext4 rw 0 0\ntmpfs /mnt/nfs-lookalike tmpfs rw 0 0\n"
	assert.True(t, mountsContainNFS(mounts, "/mnt/with space"))
	assert.True(t, mountsContainNFS(mounts, "/mnt/v4/"))
	assert.False(t, mountsContainNFS(mounts, "/mnt/ext"), "an ext4 mount is not NFS")
	assert.False(t, mountsContainNFS(mounts, "/mnt/nfs-lookalike"), "the path text must not decide, the type does")
	assert.False(t, mountsContainNFS(mounts, "/mnt/missing"))
	assert.False(t, mountsContainNFS("", "/x"))
	// control needle: the instrument sees a present NFS mount
	assert.True(t, mountsContainNFS(mounts, "/mnt/v4"))
}

// ---------------------------------------------------------------- class C: one settings mapping

func TestWF22R3_N3_ValidateRootRefusesWhatTheProtocolDoesNotConsumeOrRequire(t *testing.T) {
	host, path, user, pw := s3("nas"), s3("/media"), s3("u"), s3("p")
	port := 21
	cases := []struct {
		name string
		r    models.StorageRoot
		want string // substring of the error; "" = valid
	}{
		{"ftp ok", models.StorageRoot{Protocol: "ftp", Host: host, Port: &port, Path: path, Username: user, Password: pw}, ""},
		{"ftp with url+mount_point (review P5)", models.StorageRoot{Protocol: "ftp", Host: host, Path: path, URL: s3("https://u:p@x/dav"), MountPoint: s3("/mnt/x")}, "mount_point, url not used"},
		{"ftp without host", models.StorageRoot{Protocol: "ftp", Path: path}, "missing host"},
		{"nfs empty (review P5b)", models.StorageRoot{Protocol: "nfs"}, "missing host, path, mount_point"},
		{"nfs ok", models.StorageRoot{Protocol: "nfs", Host: host, Path: s3("/export"), MountPoint: s3("/mnt/x"), Options: s3("vers=4")}, ""},
		{"nfs with a username", models.StorageRoot{Protocol: "nfs", Host: host, Path: s3("/e"), MountPoint: s3("/m"), Username: user}, "username not used"},
		{"smb without share", models.StorageRoot{Protocol: "smb", Host: host}, "missing share"},
		{"smb ok with identity_index", models.StorageRoot{Protocol: "smb", Host: host, Path: s3("media"), Options: s3(`{"identity_index": 2}`)}, ""},
		{"smb with a url", models.StorageRoot{Protocol: "smb", Host: host, Path: s3("media"), URL: s3("http://x")}, "url not used"},
		{"webdav without url", models.StorageRoot{Protocol: "webdav"}, "missing url"},
		{"webdav ok", models.StorageRoot{Protocol: "webdav", URL: s3("https://nas/dav"), Username: user, Password: pw}, ""},
		{"webdav with a host", models.StorageRoot{Protocol: "webdav", URL: s3("https://nas/dav"), Host: host}, "host not used"},
		{"local ok", models.StorageRoot{Protocol: "local", Path: s3("/data")}, ""},
		{"local without path", models.StorageRoot{Protocol: "local"}, "missing base_path"},
		{"local with a host", models.StorageRoot{Protocol: "local", Path: s3("/d"), Host: host}, "host not used"},
		{"ftp with a pin it cannot honour", models.StorageRoot{Protocol: "ftp", Host: host, Options: s3(`{"host_key_fingerprint":"SHA256:x"}`)}, "options.host_key_fingerprint"},
		{"ftp options not JSON", models.StorageRoot{Protocol: "ftp", Host: host, Options: s3("vers=3")}, "options is not a JSON object"},
		{"unknown protocol", models.StorageRoot{Protocol: "gopher", Host: host}, "unsupported protocol"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			err := ValidateRoot(&c.r)
			if c.want == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.want)
		})
	}
}

// N4 (review P6): nothing in a registered protocol's options is dropped silently.
func TestWF22R3_N4_RegisteredProtocolOptionsAreStrict(t *testing.T) {
	spec := ProtocolSpec{Name: "r3sftp", Keys: map[string]KeyType{KeyHost: KeyString, KeyHostKeyFingerprint: KeyString, KeyTLSMode: KeyString, KeyCertSHA256: KeyString},
		New: func(map[string]interface{}) (FileSystemClient, error) { return nil, nil }, Required: []string{KeyHost}}
	require.NoError(t, RegisterProtocol(spec))
	defer UnregisterProtocol("r3sftp")
	root := &models.StorageRoot{Protocol: "r3sftp", Host: s3("nas")}

	good := `{"host_key_fingerprint":"SHA256:abc","tls_mode":"strict","cert_sha256":"ab12"}`
	root.Options = &good
	s, err := SettingsFromRoot(root, nil)
	require.NoError(t, err)
	assert.Equal(t, "SHA256:abc", s[KeyHostKeyFingerprint])
	assert.Equal(t, "strict", s[KeyTLSMode])
	assert.Equal(t, "ab12", s[KeyCertSHA256], "the certificate pin is forwarded")
	require.NoError(t, ValidateRoot(root))

	for name, bad := range map[string]string{
		"malformed json with a pin":                    `{"host_key_fingerprint": "SHA256:abc", }`,
		"non-string tls_mode":                          `{"tls_mode": true}`,
		"misspelt key":                                 `{"host_key_fingerprnt": "SHA256:abc"}`,
		"not an object":                                `["host_key_fingerprint"]`,
		"credential_ref the protocol does not consume": `{"credential_ref":"vault:x"}`,
	} {
		b := bad
		root.Options = &b
		_, err := SettingsFromRoot(root, nil)
		assert.Error(t, err, name)
		assert.Error(t, ValidateRoot(root), name)
	}
	// the error names keys, never values
	secret := `{"host_key_fingerprint": 12345}`
	root.Options = &secret
	_, err = SettingsFromRoot(root, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "12345")
}

// the one mapping: whatever SettingsFromRoot emits for a valid row is accepted by the factory's validator and by the factory (create ->
// validate -> serialise -> read agree).
func TestWF22R3_C_MappingOutputAlwaysPassesTheFactoryValidator(t *testing.T) {
	port := 445
	for _, r := range []models.StorageRoot{
		{Protocol: "local", Path: s3("/d")},
		{Protocol: "smb", Host: s3("h"), Port: &port, Path: s3("s"), Username: s3("u"), Password: s3("p"), Domain: s3("D")},
		{Protocol: "ftp", Host: s3("h"), Username: s3("u"), Password: s3("p"), Path: s3("/p")},
		{Protocol: "nfs", Host: s3("h"), Path: s3("/e"), MountPoint: s3("/m"), Options: s3("vers=4")},
		{Protocol: "webdav", URL: s3("https://h/d"), Username: s3("u"), Password: s3("p"), Path: s3("/m")},
	} {
		r := r
		require.NoError(t, ValidateRoot(&r), r.Protocol)
		s, err := SettingsFromRoot(&r, nil)
		require.NoError(t, err)
		assert.NoError(t, ValidateSettings(r.Protocol, s), r.Protocol)
	}
	_ = bytes.MinRead
}
