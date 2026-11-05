package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/config"
	"catalogizer/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS (§11.4.289): D4 of docs/qa/live-conv-docs-subs-r1-20261104.md. The real server binary on
// a real migrated database scans a real local corpus (a real PDF, a text file, a Markdown file and a
// non-document file) and `GET /api/v1/search/files?q=` must find the words INSIDE the extracted
// document text, not only file names. No mock, no stub, no fixture server.

func searchTotal(t *testing.T, c *apiClient, tok, q string) (int, []string) {
	t.Helper()
	code, raw := c.do("GET", "/api/v1/search/files?q="+url.QueryEscape(q), tok, nil)
	require.Equal(t, 200, code, "%s", raw)
	var m struct {
		Data struct {
			Total int `json:"total_count"`
			Files []struct {
				Name string `json:"name"`
			} `json:"files"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(raw, &m), "%s", raw)
	names := []string{}
	for _, f := range m.Data.Files {
		names = append(names, f.Name)
	}
	return m.Data.Total, names
}

func scanAndWait(t *testing.T, c *apiClient, tok string, rootID int64, dbPath, keyFile string, wantFiles int) {
	t.Helper()
	code, raw := c.do("POST", "/api/v1/scans", tok, map[string]interface{}{"storage_root_id": rootID, "scan_type": "full"})
	require.Contains(t, []int{200, 202}, code, "%s", raw)
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: dbPath, EncryptionKeyFile: keyFile, MaxOpenConnections: 2})
		require.NoError(t, err)
		var n int
		_ = db.QueryRow(`SELECT COUNT(*) FROM files WHERE storage_root_id=? AND is_directory=0 AND deleted=0`, rootID).Scan(&n)
		db.Close()
		if n >= wantFiles {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("scan did not register %d files", wantFiles)
}

func TestD4_FullTextSearchOverExtractedDocuments_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("poppler pdftotext absent: PDF text cannot be validated independently (hardware_not_present-class SKIP)")
	}
	dir := t.TempDir()
	corpus := filepath.Join(dir, "corpus")
	require.NoError(t, os.MkdirAll(corpus, 0o755))
	pdf, err := os.ReadFile(filepath.Join("services", "testdata", "pdf", "multi.pdf"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(corpus, "report.pdf"), pdf, 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(corpus, "notes.txt"), []byte("alpha OKAPI-1357 omega\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(corpus, "readme.md"), []byte("# Title\nMARKDOWN-TOKEN-88\n"), 0o644))
	// a non-document file whose bytes contain a token: must NOT be text-indexed
	require.NoError(t, os.WriteFile(filepath.Join(corpus, "blob.bin"), []byte("BINARY-TOKEN-77"), 0o644))

	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw, "CATALOGIZER_LOCAL_ROOT_BASES="+dir)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	defer func() {
		if t.Failed() {
			t.Logf("server output:\n%s", out.String())
		}
	}()
	c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
	adm := c.login("admin", adminPw)

	dbPath, keyFile := filepath.Join(dir, "catalog.db"), filepath.Join(dir, "keys", "db.key")
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: dbPath, EncryptionKeyFile: keyFile, MaxOpenConnections: 2})
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('d4-root','local',?,1)`, corpus)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRow(`SELECT id FROM storage_roots WHERE name='d4-root'`).Scan(&rootID))
	db.Close()

	scanAndWait(t, c, adm, rootID, dbPath, keyFile, 4)

	// control needle 1: the instrument works, a file NAME is found
	n, names := searchTotal(t, c, adm, "notes")
	require.Equal(t, 1, n, "name search must find notes.txt: %v", names)
	// control needle 2: an absent token finds nothing
	n, _ = searchTotal(t, c, adm, "NOSUCHTOKEN-424242")
	assert.Equal(t, 0, n)

	// the defect: words inside the document text
	deadline := time.Now().Add(30 * time.Second) // indexing runs after the scan, in the background
	var got int
	for time.Now().Before(deadline) {
		if got, _ = searchTotal(t, c, adm, "OKAPI-1357"); got == 1 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	n, names = searchTotal(t, c, adm, "OKAPI-1357")
	assert.Equal(t, 1, n, "text file content must be searchable")
	assert.Equal(t, []string{"notes.txt"}, names)
	n, names = searchTotal(t, c, adm, "MARKDOWN-TOKEN-88")
	assert.Equal(t, 1, n, "markdown content must be searchable")
	assert.Equal(t, []string{"readme.md"}, names)
	n, names = searchTotal(t, c, adm, "lazy dog")
	assert.Equal(t, 1, n, "PDF text must be searchable")
	assert.Equal(t, []string{"report.pdf"}, names)
	// a non-document file is not text-indexed
	n, _ = searchTotal(t, c, adm, "BINARY-TOKEN-77")
	assert.Equal(t, 0, n, "binary files are never text-indexed")

	// a changed file is re-indexed on the next scan; the old words disappear
	require.NoError(t, os.WriteFile(filepath.Join(corpus, "notes.txt"), []byte("alpha CHANGED-TOKEN-5 omega, longer now\n"), 0o644))
	require.NoError(t, os.Chtimes(filepath.Join(corpus, "notes.txt"), time.Now().Add(time.Hour), time.Now().Add(time.Hour)))
	scanAndWait(t, c, adm, rootID, dbPath, keyFile, 4)
	deadline = time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if got, _ = searchTotal(t, c, adm, "CHANGED-TOKEN-5"); got == 1 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	assert.Equal(t, 1, got, "changed content re-indexed")
	n, _ = searchTotal(t, c, adm, "OKAPI-1357")
	assert.Equal(t, 0, n, "stale text must not stay searchable")
}
