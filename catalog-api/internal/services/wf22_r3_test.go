package services

// WF22 round 3 (structural fix round, 11.4.276(E)). Class inventories (A containment at the scan entry, B credential exposure in logs / reasons,
// C the single settings mapping) and the independent reviewer's round-2 probes + survivors adopted: P4, P8, P9, P10 and mutants K03, K10, K15,
// K16, K17.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"catalogizer/filesystem"
	"catalogizer/models"

	"digital.vasic.filesystem/pkg/fabric"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// ---------------------------------------------------------------- class A: containment at the scan entry

// P4 (review): a job path "/../../etc" must reach the client confined.
func TestWF22R3_P4_JobPathConfinedToRoot(t *testing.T) {
	m := newMemFS()
	m.addFile("/", "inside.mkv", 1, t0)
	m.addDir("/etc")
	m.addFile("/etc", "x.conf", 1, t0)
	var mu sync.Mutex
	var raw []string
	m.listHook = func(dir string) { mu.Lock(); raw = append(raw, dir); mu.Unlock() }
	rec := newRecorder()
	_, _ = scanOnce(t, newTestScanner(rec, nil), m, jobOf("/../../etc", 2, "full"))
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, raw, "control: the scan listed something")
	for _, d := range raw {
		assert.NotContains(t, d, "..", "the scanner hands an unconfined job path to the client: ListDirectory(%q)", d)
	}
	assert.Equal(t, "/etc", raw[0])
}

// QueueScan is the one entry of every scan request: the path is confined there, in the form the caller used.
func TestWF22R3_N2_QueueScanConfinesTheRequestedPath(t *testing.T) {
	for in, want := range map[string]string{
		"":                 "",
		"/":                "/",
		"/../../etc":       "/etc",
		"../../etc":        "etc",
		"a/../../b":        "b",
		"/movies/Wait.. L": "/movies/Wait.. L",
		".":                ".",
	} {
		s := NewUniversalScanner(nil, zap.NewNop(), nil, nil)
		h := "x"
		require.NoError(t, s.QueueScan(ScanJob{ID: "q", Path: in, StorageRoot: &models.StorageRoot{Name: "r", Protocol: "ftp", Host: &h}}))
		select {
		case j := <-s.scanQueue:
			assert.Equal(t, want, j.Path, "QueueScan path %q", in)
		default:
			t.Fatal("job not queued")
		}
	}
}

// ---------------------------------------------------------------- N8 / P8

func TestWF22R3_N8_IPv6HostWithPortSharesTheHostBudget(t *testing.T) {
	h := "fe80::1"
	p21 := 21
	k1 := hostKey(ScanJob{StorageRoot: &models.StorageRoot{Host: &h, Port: &p21}})
	a, errA := fabric.HostKey(k1)
	require.NoError(t, errA, "key %q must be a valid host key", k1)
	assert.Equal(t, "[fe80::1]:21", k1)
	// the URL fallback of the same host agrees
	u := "http://[fe80::1]:21/dav"
	k2 := hostKey(ScanJob{StorageRoot: &models.StorageRoot{URL: &u}})
	b, errB := fabric.HostKey(k2)
	require.NoError(t, errB)
	assert.Equal(t, a, b, "a host root and a url root of the same IPv6 endpoint share one budget")
	// control needle: IPv4
	h4 := "192.168.1.5"
	c, _ := fabric.HostKey(hostKey(ScanJob{StorageRoot: &models.StorageRoot{Host: &h4, Port: &p21}}))
	u4 := "http://192.168.1.5:21/dav"
	d, _ := fabric.HostKey(hostKey(ScanJob{StorageRoot: &models.StorageRoot{URL: &u4}}))
	require.Equal(t, c, d, "control needle")
}

// ---------------------------------------------------------------- N6 / P9

type r3CaptureFactory struct{ cfg *filesystem.StorageConfig }

func (f *r3CaptureFactory) CreateClient(c *filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	f.cfg = c
	return nil, errors.New("captured")
}
func (f *r3CaptureFactory) SupportedProtocols() []string { return []string{"smb"} }

func TestWF22R3_N6_CoverArtResolvesTheSMBIdentityLikeEveryOtherCaller(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "svc-media")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "r3-probe-secret")
	opts := `{"identity_index": 1}`
	task := thumbnailTask{protocol: "smb", host: sql.NullString{String: "nas", Valid: true}, rootPath: sql.NullString{String: "media", Valid: true},
		options: sql.NullString{String: opts, Valid: true}, videoPath: "/x.mkv"}
	f := &r3CaptureFactory{}
	s := &CoverArtService{fsFactory: f}
	_, _ = s.copyRemoteFileToTemp(context.Background(), task)
	require.NotNil(t, f.cfg, "factory not reached")
	scannerView, err := (&UniversalScanner{}).storageRootToSettings(task.storageRoot())
	require.NoError(t, err)
	assert.Equal(t, "svc-media", scannerView["username"], "control: the scanner resolves the identity")
	assert.Equal(t, scannerView, f.cfg.Settings, "cover art maps the same SMB root exactly like the scanner")
}

// ---------------------------------------------------------------- N9 / P10, K16, K17

func TestWF22R3_N9_ReasonScrubsIdentityAndAtSignPasswords(t *testing.T) {
	t.Setenv("CATALOGIZER_IDENTITY_COUNT", "1")
	t.Setenv("CATALOGIZER_IDENTITY_1_TYPE", "credentials")
	t.Setenv("CATALOGIZER_IDENTITY_1_USERNAME", "svc-media")
	t.Setenv("CATALOGIZER_IDENTITY_1_PASSWORD", "envSecret123")
	opts := `{"identity_index": 1}`
	h := "nas"
	smbRoot := &models.StorageRoot{Protocol: "smb", Host: &h, Options: &opts}
	got := scrubSecrets("smb login failed for svc-media with envSecret123", secretsOf(smbRoot))
	assert.NotContains(t, got, "envSecret123", "the env-resolved SMB identity password survives the reason scrub")

	u := "https://alice:p@ssw0rd@nas/dav"
	wRoot := &models.StorageRoot{Protocol: "webdav", URL: &u}
	got = scrubSecrets("dial failed for https://alice:p@ssw0rd@nas/dav/x", secretsOf(wRoot))
	assert.NotContains(t, got, "ssw0rd", "a URL password containing '@' leaks a fragment through the reason scrub: %q", got)
	assert.Contains(t, got, "nas/dav/x", "the host and path stay readable")
	// the same URL with NO known secret: the userinfo regex alone must remove the whole userinfo
	got = scrubSecrets("dial failed for https://alice:p@ssw0rd@nas/dav/x", nil)
	assert.NotContains(t, got, "ssw0rd")
	// control needle: an ordinary root password IS scrubbed
	pw := "plainSecret9"
	assert.NotContains(t, scrubSecrets("x plainSecret9 y", secretsOf(&models.StorageRoot{Password: &pw})), pw)
}

// K16: a secret is also scrubbed in its URL-escaped spellings (the form that appears in an error echoing a URL).
func TestWF22R3_K16_EscapedSecretVariantsAreScrubbed(t *testing.T) {
	pw := "p@ss w/rd&x"
	root := &models.StorageRoot{Protocol: "ftp", Password: &pw}
	for _, spelled := range []string{
		"raw p@ss w/rd&x in text",
		"query p%40ss+w%2Frd%26x in url",
		"userinfo p%40ss%20w%2Frd&x in url",
		"path p@ss%20w%2Frd&x in url",
	} {
		got := scrubSecrets(spelled, secretsOf(root))
		assert.NotContains(t, got, "p%40ss", spelled)
		assert.NotContains(t, got, "p@ss", spelled)
		assert.NotContains(t, got, "w%2Frd", spelled)
	}
}

// K17: the scheme of a URL is matched case-insensitively.
func TestWF22R3_K17_UserinfoScrubIsCaseInsensitive(t *testing.T) {
	got := scrubSecrets("dial HTTPS://alice:topsecret@nas/dav and Ftp://bob:hunter2@h/x", nil)
	assert.NotContains(t, got, "topsecret")
	assert.NotContains(t, got, "hunter2")
}

// ---------------------------------------------------------------- N10: logs carry what the Reason carries

type r3ErrFactory struct{ err error }

func (f *r3ErrFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return nil, f.err
}
func (f *r3ErrFactory) SupportedProtocols() []string { return []string{"webdav"} }

func TestWF22R3_N10_LogsCarryTheScrubbedErrorOnly(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)
	pw := "logSecret-4242"
	url := "https://alice:" + pw + "@nas/dav"
	root := &models.StorageRoot{ID: 1, Name: "dav", Protocol: "webdav", URL: &url, Password: &pw}
	f := &r3ErrFactory{err: fmt.Errorf("failed to dial https://alice:%s@nas/dav: refused (password %s)", pw, pw)}
	s := NewUniversalScanner(nil, zap.New(core), nil, f)
	require.NoError(t, s.Start())
	defer s.Stop()
	require.NoError(t, s.QueueScan(ScanJob{ID: "n10", StorageRoot: root, Path: "/", Context: context.Background()}))
	var st *ScanStatus
	require.Eventually(t, func() bool {
		var ok bool
		st, ok = s.GetActiveScanStatus("n10")
		return ok && st != nil && st.GetSnapshot().Status == "failed"
	}, 10*time.Second, 20*time.Millisecond)
	assert.NotContains(t, st.GetSnapshot().Reason, pw, "control: the Reason is scrubbed")
	require.NotEmpty(t, logs.All(), "control: the scanner logged")
	for _, e := range logs.All() {
		line := e.Message + fmt.Sprint(e.ContextMap())
		assert.NotContains(t, line, pw, "a log line carries the credential the Reason does not: %s", line)
	}
}

// ---------------------------------------------------------------- K03: the budget boundary

// A directory whose first fetch ends exactly at the remaining budget with the extra entries uncounted must still be re-listed: the entry after
// the fetch limit is countable, so the total goes over MaxEntries and the scan FAILS rather than silently omitting it.
func TestWF22R3_K03_ExactBudgetBoundaryIsRelistedAndFailsInsteadOfTruncating(t *testing.T) {
	m := newMemFS()
	m.addDir("/")
	for _, n := range []string{"a", "b", "c"} {
		m.addFile("/", n, 1, t0)
	}
	m.addSubdir("/", "skipme")
	m.addFile("/", "d", 1, t0)
	_, err := scanOnce(t, newTestScanner(newRecorder(), func(o *GenericScannerOptions) {
		o.MaxEntries = 3
		o.SkipDirNames = []string{"skipme"}
	}), m, jobOf("/", 10, "full"))
	require.Error(t, err, "4 countable entries over a budget of 3 must not be accepted as a complete scan")
	assert.Equal(t, FailLimit, scanKind(t, err))
}

// ---------------------------------------------------------------- K10: a relative PWD answer

func TestWF22R3_K10_FTPRelativeWorkingDirectoryAnswerIsNotUsedAsTheBase(t *testing.T) {
	cwdServerPWD = func(string) string { return "movies" } // relative, whatever the real cwd is
	defer func() { cwdServerPWD = nil }()
	m := "modify=20260102030405"
	tree := map[string][]string{
		"/":       {"type=dir;" + m + "; movies"},
		"/movies": {"type=file;size=3;" + m + "; a.mkv"},
	}
	var seen []string
	host, port := startCwdServer(t, tree, &seen)
	c, err := filesystem.NewDefaultClientFactory().CreateClient(&filesystem.StorageConfig{Protocol: "ftp",
		Settings: map[string]interface{}{"host": host, "port": port, "username": "u", "password": "p", "path": "movies"}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	defer c.Disconnect(context.Background())
	rec := newRecorder()
	err = NewFTPScanner(nil).WithRecorder(rec.record).ScanPath(context.Background(), c, jobOf("/", 10, "full"), &ScanStatus{})
	require.NoError(t, err, "listings: %v", seen)
	assert.Equal(t, []string{"/a.mkv"}, rec.sorted())
}

// ---------------------------------------------------------------- classifiers

// walkSources calls fn for every production Go source file of catalog-api.
func walkSources(t *testing.T, fn func(rel, src string)) int {
	t.Helper()
	root := filepath.Join("..", "..")
	n := 0
	require.NoError(t, filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", "node_modules", ".git", "mocks", "docs", "testdata", "tests":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		rel, _ := filepath.Rel(root, p)
		n++
		fn(filepath.ToSlash(rel), string(b))
		return nil
	}))
	return n
}

// callsOf lists, per enclosing function, the selector/ident call names in src.
func callsOf(t *testing.T, src string) map[string][]string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	require.NoError(t, err)
	out := map[string][]string{}
	for _, d := range f.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if !ok || fd.Body == nil {
			continue
		}
		name := fd.Name.Name
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if ce, ok := n.(*ast.CallExpr); ok {
				switch fn := ce.Fun.(type) {
				case *ast.SelectorExpr:
					if id, ok := fn.X.(*ast.Ident); ok {
						out[name] = append(out[name], id.Name+"."+fn.Sel.Name)
					}
					out[name] = append(out[name], fn.Sel.Name)
				case *ast.Ident:
					out[name] = append(out[name], fn.Name)
				}
			}
			return true
		})
	}
	return out
}

func has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// Class C: SettingsFromRoot (the single mapping, with its error) is called from a closed set of production functions, and MustSettingsFromRoot
// (panics) from none. A new caller must be added here WITH its error handling.
func TestWF22R3_C_Classifier_SettingsFromRootCallersAreAClosedSet(t *testing.T) {
	// control needle: the instrument finds a call and attributes it to its function
	c := callsOf(t, "package x\nfunc F() { filesystem.SettingsFromRoot(r, nil) }\nfunc G() {}\n")
	require.True(t, has(c["F"], "SettingsFromRoot"))
	require.False(t, has(c["G"], "SettingsFromRoot"))

	allowed := map[string]bool{
		"filesystem/settings.go:MustSettingsFromRoot":                  true, // the test helper itself
		"filesystem/settings.go:ValidateRoot":                          true,
		"handlers/comic_pages_handler.go:comicStorageRootSettings":     true,
		"internal/handlers/stream_handler.go:storageRootToSettings":    true,
		"internal/services/universal_scanner.go:storageRootToSettings": true,
		"internal/services/cover_art_service.go:copyRemoteFileToTemp":  true,
	}
	var found []string
	n := walkSources(t, func(rel, src string) {
		for fn, calls := range callsOf(t, src) {
			if has(calls, "MustSettingsFromRoot") && rel != "filesystem/settings.go" {
				t.Errorf("%s:%s calls MustSettingsFromRoot (panics): production code handles the error", rel, fn)
			}
			if has(calls, "SettingsFromRoot") {
				found = append(found, rel+":"+fn)
			}
		}
	})
	require.Greater(t, n, 100, "control: the walk covers the source tree")
	sort.Strings(found)
	var want []string
	for k := range allowed {
		want = append(want, k)
	}
	sort.Strings(want)
	assert.Equal(t, want, found)
}

// Class A: scanQueue is written only by QueueScan (so the confinement there covers every job), and QueueScan confines.
func TestWF22R3_A_Classifier_OnlyQueueScanFeedsTheQueueAndItConfines(t *testing.T) {
	b, err := os.ReadFile("universal_scanner.go")
	require.NoError(t, err)
	src := string(b)
	require.Equal(t, 1, strings.Count(src, "s.scanQueue <-"), "exactly one producer of the scan queue")
	i := strings.Index(src, "func (s *UniversalScanner) QueueScan")
	j := strings.Index(src, "s.scanQueue <-")
	require.True(t, i >= 0 && j > i)
	assert.Contains(t, src[i:j], "ConfineScanPath(job.Path)")
	g, err := os.ReadFile("generic_scanner.go")
	require.NoError(t, err)
	k := strings.Index(string(g), "func (g *GenericScanner) ScanPath")
	require.True(t, k >= 0)
	assert.Contains(t, string(g)[k:k+900], "filesystem.ConfinePath(job.Path)")
}

// Class B (logs/errors): in the scanner pipeline an error is logged through errField (scrubbed), never zap.Error.
func TestWF22R3_B_Classifier_ScannerLogsNeverUseRawZapError(t *testing.T) {
	needle := "package x\nfunc F() { l.Error(\"x\", zap.Error(err)) }\n"
	require.Contains(t, needle, "zap.Error(")
	for _, f := range []string{"universal_scanner.go", "generic_scanner.go"} {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.NotContains(t, string(b), "zap.Error(", "%s logs a raw error: a client/server answer may carry a credential (WF22 N10); use errField", f)
		assert.Contains(t, string(b), "errField(", "control: %s does use the scrubbed field", f)
	}
}

// N10 (handlers): the one call handlers use to log a client error removes the root's credentials.
func TestWF22R3_N10_ScrubErrorForRemovesTheRootsCredentials(t *testing.T) {
	pw := "hdlSecret-31337"
	u := "https://alice:" + pw + "@nas/dav"
	root := &models.StorageRoot{Protocol: "webdav", URL: &u, Password: &pw}
	got := ScrubErrorFor(fmt.Errorf("PROPFIND https://alice:%s@nas/dav failed: bad password %s", pw, pw), root)
	assert.NotContains(t, got, pw)
	assert.Contains(t, got, "PROPFIND", "control: the message itself survives")
	assert.Equal(t, "", ScrubErrorFor(nil, root))
}
