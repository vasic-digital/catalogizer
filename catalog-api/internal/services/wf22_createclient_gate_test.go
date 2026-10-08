package services

// WF22 R1 gate: every production call of ClientFactory.CreateClient builds its settings through the single mapping (filesystem.SettingsFromRoot).
// The review found a FIFTH caller (cover-art thumbnails) that the settings contract and its doc did not know about and that the strict factory
// refused for ftp, nfs and webdav. This test enumerates the call sites from the SOURCE, so a sixth one cannot appear unnoticed.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// expectedCreateClientSites are the production files that call CreateClient and the test that pushes a representative root of every protocol
// through their settings builder.
var expectedCreateClientSites = map[string]string{
	"internal/services/universal_scanner.go": "TestSettingsContract_ScannerSettingsReachTheClient (internal/services)",
	"internal/services/cover_art_service.go": "TestWF22_R1_CoverArtThumbnailCopyBuildsEveryProtocolThroughTheSingleMapping (internal/services)",
	"internal/handlers/stream_handler.go":    "TestStreamHandler_StorageRootToSettings_AllProtocols (internal/handlers)",
	"handlers/comic_pages_handler.go":        "TestWF22_ComicAndPDFSettingsGoThroughTheSingleMapping (handlers)",
	"handlers/pdf_pages_handler.go":          "TestWF22_ComicAndPDFSettingsGoThroughTheSingleMapping (handlers)",
}

func TestWF22_R1_Gate_EveryCreateClientCallerUsesTheSingleSettingsMapping(t *testing.T) {
	root := filepath.Join("..", "..") // catalog-api
	fset := token.NewFileSet()

	type fileInfo struct {
		callsCreateClient bool
		callsMapping      bool
		calls             map[string]bool // bare function names called
	}
	files := map[string]*fileInfo{}
	// function name -> true if its body calls SettingsFromRoot (package-level helpers such as comicStorageRootSettings)
	mappers := map[string]bool{}

	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", "node_modules", ".git", "tests", "mocks", "docs", "testdata":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return perr
		}
		rel, _ := filepath.Rel(root, p)
		fi := &fileInfo{calls: map[string]bool{}}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				ce, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fn := ce.Fun.(type) {
				case *ast.SelectorExpr:
					if fn.Sel.Name == "CreateClient" {
						fi.callsCreateClient = true
					}
					if fn.Sel.Name == "SettingsFromRoot" {
						fi.callsMapping = true
						mappers[fd.Name.Name] = true
					}
					fi.calls[fn.Sel.Name] = true
				case *ast.Ident:
					fi.calls[fn.Name] = true
					if fn.Name == "SettingsFromRoot" {
						fi.callsMapping = true
						mappers[fd.Name.Name] = true
					}
				}
				return true
			})
		}
		files[filepath.ToSlash(rel)] = fi
		return nil
	})
	require.NoError(t, err)

	// control needle: the instrument sees the factory's own definition and the known mapping
	require.Contains(t, files, "filesystem/settings.go", "control: the walk reaches the filesystem package")
	require.Contains(t, files, "internal/services/universal_scanner.go")

	var sites []string
	for rel, fi := range files {
		if !fi.callsCreateClient {
			continue
		}
		sites = append(sites, rel)
		usesMapping := fi.callsMapping
		for name := range fi.calls {
			if mappers[name] {
				usesMapping = true
			}
		}
		assert.True(t, usesMapping, "%s calls CreateClient but does not build its settings through filesystem.SettingsFromRoot (directly or through a helper that does): "+
			"a hand-built map is how cover-art thumbnails came to send an SMB-shaped map to every protocol (WF22 R1)", rel)
	}
	sort.Strings(sites)
	var expected []string
	for k := range expectedCreateClientSites {
		expected = append(expected, k)
	}
	sort.Strings(expected)
	assert.Equal(t, expected, sites, "a new CreateClient caller must be added to expectedCreateClientSites WITH a test that pushes a root of every protocol through its settings")
}

// TestWF22_H1_Gate_EveryStorageRootLoaderSelectsTheConnectionColumns closes the class "a query builds a StorageRoot for a client but leaves a
// column out": the scan handler's loader selected neither url nor mount_point, so WebDAV and NFS scans queued through the API reached the factory
// without their address (WF22 H1). Every production query that selects a root's password (= builds a root to connect with) must select the
// url, the mount point and the options too - or use models.StorageRootConnColumns, which does.
func TestWF22_H1_Gate_EveryStorageRootLoaderSelectsTheConnectionColumns(t *testing.T) {
	root := filepath.Join("..", "..")
	// documented exceptions: reads the password but builds no client from the row
	exceptions := map[string]string{
		"main.go": "SMB identity ingestion at start-up (name, host, port, path, username, password, domain of smb roots): it feeds the identity table, no client is built from the row",
	}
	checked, loaders := 0, 0
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "vendor", "node_modules", ".git", "tests", "mocks", "docs", "testdata":
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
		text := string(b)
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		checked++
		for from := 0; ; {
			i := strings.Index(text[from:], "storage_roots")
			if i < 0 {
				break
			}
			i += from
			from = i + len("storage_roots")
			start := strings.LastIndex(text[:i], "SELECT")
			if start < 0 || i-start > 3000 {
				continue
			}
			seg := text[start:i]
			if !strings.Contains(seg, "password") {
				continue
			}
			loaders++
			if why, ok := exceptions[rel]; ok {
				t.Logf("%s: exception: %s", rel, why)
				continue
			}
			if strings.Contains(seg, "StorageRootConnColumns") {
				continue
			}
			for _, col := range []string{"url", "mount_point", "options"} {
				assert.Contains(t, seg, col, "%s: a query that selects the storage root's password but not its %s builds a root the settings contract cannot map completely (WF22 H1)", rel, col)
			}
		}
		return nil
	})
	require.NoError(t, err)
	require.Greater(t, checked, 100, "control: the walk covers the source tree")
	assert.GreaterOrEqual(t, loaders, 5, "control: the instrument finds the known loaders (scan handler, cover art, stream, comic, pdf) and the start-up identity query")
}
