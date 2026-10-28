package services

// Review round 6 (F1, F3, F4). Adapted from the reviewer probes
// TestRV6_MirrorDirectoryMkdirFollowsDestinationSymlink, TestRV6_WebDAVUploadSingleFileRootIsSilentNoOp,
// TestRV6_LegitLocalModesInsideDataRoot and TestRV6_ConversionDerivedSiblingsFollowPlantedSymlink.

import (
	"bytes"
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// F1: the mirror created a destination directory for every source directory with
// os.MkdirAll(Join(destDir, rel)) and no confinement, so a directory symlink planted in the
// destination let it create directories OUTSIDE the destination root.
func TestRV6_MirrorDirectoryMkdirRefusesDestinationSymlink(t *testing.T) {
	base, src, dst, _, _ := perFileFixture(t)
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(dst, "sub")))
	require.NoError(t, os.MkdirAll(filepath.Join(src, "sub", "deeper", "deepest"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(src, "sub", "deeper", "f.txt"), []byte("f"), 0o644))

	err := (&SyncService{}).performMirrorSync(src, dst, &models.SyncSession{})
	_, serr := os.Stat(filepath.Join(outside, "deeper"))
	assert.True(t, os.IsNotExist(serr), "mirror created a directory outside the destination root through a planted symlink")
	assert.Error(t, err, "the refused directory is reported, never silently skipped")
}

// F1 (copyFile): Dir(dst) under a planted link directory is refused before MkdirAll runs, so
// the outside directory is never created.
func TestRV6_CopyFileDoesNotCreateParentThroughPlantedLink(t *testing.T) {
	base, src, dst, _, _ := perFileFixture(t)
	outside := filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(dst, "lnk")))
	err := (&SyncService{}).copyFile(filepath.Join(src, "a.txt"), filepath.Join(dst, "lnk", "new", "a.txt"), 0o644, src, dst)
	assert.Error(t, err)
	_, serr := os.Stat(filepath.Join(outside, "new"))
	assert.True(t, os.IsNotExist(serr), "copyFile created a directory outside the destination root")
}

// F3: an upload whose local_path / source_directory is a single regular file used to succeed
// with zero files. It must be refused loudly.
func TestRV6_WebDAVUploadSingleFileRootIsRefused(t *testing.T) {
	_, src, _, _, _ := perFileFixture(t)
	single := filepath.Join(src, "a.txt")
	davRoot := t.TempDir()
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionUpload, LocalPath: single, RemotePath: "/r"}
	sess := newRV5Session(t, svc, 1)
	err := svc.uploadToWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", ""))
	require.Error(t, err, "a single-file upload root must be refused, not reported as a 0-file success")
	assert.Contains(t, err.Error(), "not a directory")
	ents, _ := os.ReadDir(davRoot)
	assert.Empty(t, ents)
}

func TestRV6_CloudUploadSingleFileRootIsRefused(t *testing.T) {
	_, src, _, _, _ := perFileFixture(t)
	single := filepath.Join(src, "a.txt")
	missing := filepath.Join(src, "missing")
	svc := &SyncService{}
	for _, tc := range []struct{ name, dir string }{{"single-file", single}, {"missing", missing}} {
		s3 := fmt.Sprintf(`{"bucket":"b","access_key":"k","secret_key":"s","source_directory":%q}`, tc.dir)
		err := svc.performS3Sync(context.Background(), &models.SyncSession{}, &models.SyncEndpoint{SyncSettings: &s3})
		require.Error(t, err, "S3 %s", tc.name)
		assert.Contains(t, err.Error(), "upload source", "S3 %s: %v", tc.name, err)

		gcs := fmt.Sprintf(`{"bucket":"b","source_directory":%q}`, tc.dir)
		err = svc.performGoogleCloudStorageSync(context.Background(), &models.SyncSession{}, &models.SyncEndpoint{SyncSettings: &gcs})
		require.Error(t, err, "GCS %s", tc.name)
		assert.Contains(t, err.Error(), "upload source", "GCS %s: %v", tc.name, err)

		// local_path fallback (no source_directory)
		empty := `{"bucket":"b","access_key":"k","secret_key":"s"}`
		err = svc.performS3Sync(context.Background(), &models.SyncSession{}, &models.SyncEndpoint{SyncSettings: &empty, LocalPath: tc.dir})
		require.Error(t, err, "S3 local_path %s", tc.name)
		assert.Contains(t, err.Error(), "upload source", "S3 local_path %s: %v", tc.name, err)
	}
}

// Regression control: mirror / incremental / bidirectional inside one data root keep working.
func TestRV6_LegitLocalModesInsideDataRoot(t *testing.T) {
	for _, mode := range []string{"mirror", "incremental", "bidirectional"} {
		t.Run(mode, func(t *testing.T) {
			data := t.TempDir()
			src, dst := filepath.Join(data, "s"), filepath.Join(data, "d")
			require.NoError(t, os.MkdirAll(filepath.Join(src, "x", "y"), 0o755))
			require.NoError(t, os.MkdirAll(dst, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(src, "x", "y", "f"), []byte("F"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(dst, "back"), []byte("B"), 0o644))
			fut := time.Now().Add(time.Hour)
			require.NoError(t, os.Chtimes(filepath.Join(dst, "back"), fut, fut))
			s := &SyncService{}
			var err error
			switch mode {
			case "mirror":
				err = s.performMirrorSync(src, dst, &models.SyncSession{})
			case "incremental":
				err = s.performIncrementalSync(src, dst, &models.SyncSession{})
			default:
				err = s.performBidirectionalSync(src, dst, &models.SyncSession{})
			}
			require.NoError(t, err)
			b, rerr := os.ReadFile(filepath.Join(dst, "x", "y", "f"))
			require.NoError(t, rerr)
			assert.Equal(t, "F", string(b))
			if mode == "bidirectional" {
				b, rerr = os.ReadFile(filepath.Join(src, "back"))
				require.NoError(t, rerr)
				assert.Equal(t, "B", string(b))
			}
			if mode == "mirror" {
				_, serr := os.Stat(filepath.Join(dst, "back"))
				assert.True(t, os.IsNotExist(serr), "mirror cleanup removes the extra file")
			}
		})
	}
}

func rv6PDF(pages int) []byte {
	var b bytes.Buffer
	var offs []int
	obj := func(s string) { offs = append(offs, b.Len()); b.WriteString(s) }
	b.WriteString("%PDF-1.4\n")
	kids := ""
	for i := 0; i < pages; i++ {
		kids += fmt.Sprintf("%d 0 R ", 4+2*i)
	}
	obj("1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n")
	obj(fmt.Sprintf("2 0 obj<</Type/Pages/Kids[%s]/Count %d>>endobj\n", kids, pages))
	obj("3 0 obj<</Type/Font/Subtype/Type1/BaseFont/Helvetica>>endobj\n")
	for i := 0; i < pages; i++ {
		st := fmt.Sprintf("BT /F1 12 Tf 10 10 Td (p%d) Tj ET", i)
		obj(fmt.Sprintf("%d 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 100 100]/Resources<</Font<</F1 3 0 R>>>>/Contents %d 0 R>>endobj\n", 4+2*i, 5+2*i))
		obj(fmt.Sprintf("%d 0 obj<</Length %d>>stream\n%s\nendstream endobj\n", 5+2*i, len(st), st))
	}
	x := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer<</Size %d/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, x)
	return b.Bytes()
}

// rv6ConvFixture: one conversion root used as both read and write root, an outside directory
// and a 2-page PDF in the root.
func rv6ConvFixture(t *testing.T) (root, outside, src string) {
	t.Helper()
	base := t.TempDir()
	root = filepath.Join(base, "conv")
	outside = filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	src = filepath.Join(root, "in.pdf")
	require.NoError(t, os.WriteFile(src, rv6PDF(2), 0o644))
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)
	return
}

// F4: every derived write of a conversion goes through the write roots and refuses a symlink leaf.
func TestRV6_ConversionDerivedWritesRefusePlantedSymlinks(t *testing.T) {
	t.Run("multi-page sibling", func(t *testing.T) {
		root, outside, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "img.png")
		require.NoError(t, confineConversionPaths(src, tgt))
		require.NoError(t, os.Symlink(filepath.Join(outside, "pwn_page"), filepath.Join(root, "img_page_1.png")))
		err := (&ConversionService{}).convertPDFToImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt}, "png")
		assert.Error(t, err)
		_, serr := os.Lstat(filepath.Join(outside, "pwn_page"))
		assert.True(t, os.IsNotExist(serr), "multi-page writer followed a planted symlink outside the root")
	})
	// A link that resolves INSIDE the root passes ConfineDataFile; only the no-follow open stops
	// the writer from overwriting the file the link names (reviewer mutant R6-9 survived without it).
	t.Run("multi-page sibling linked to an in-root file", func(t *testing.T) {
		root, _, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "img.png")
		victim := filepath.Join(root, "victim.bin")
		require.NoError(t, os.WriteFile(victim, []byte("KEEP"), 0o644))
		require.NoError(t, os.Symlink(victim, filepath.Join(root, "img_page_1.png")))
		err := (&ConversionService{}).convertPDFToImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt}, "png")
		assert.Error(t, err)
		b, rerr := os.ReadFile(victim)
		require.NoError(t, rerr)
		assert.Equal(t, "KEEP", string(b), "multi-page writer overwrote an in-root file through a planted link")
	})
	t.Run("multi-page legit control", func(t *testing.T) {
		root, _, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "ok.png")
		require.NoError(t, (&ConversionService{}).convertPDFToImage(&models.ConversionJob{SourcePath: src, TargetPath: tgt}, "png"))
		for _, n := range []string{"ok_page_1.png", "ok_page_2.png"} {
			fi, err := os.Lstat(filepath.Join(root, n))
			require.NoError(t, err)
			assert.True(t, fi.Mode().IsRegular() && fi.Size() > 0, n)
		}
	})
	t.Run("text target symlink leaf", func(t *testing.T) {
		root, outside, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "out.txt")
		require.NoError(t, os.Symlink(filepath.Join(outside, "pwn_txt"), tgt))
		err := (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
		assert.Error(t, err)
		_, serr := os.Lstat(filepath.Join(outside, "pwn_txt"))
		assert.True(t, os.IsNotExist(serr))
	})
	t.Run("text target outside roots", func(t *testing.T) {
		_, outside, src := rv6ConvFixture(t)
		tgt := filepath.Join(outside, "direct.txt")
		err := (&ConversionService{}).convertPDFToText(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
		assert.Error(t, err)
		_, serr := os.Lstat(tgt)
		assert.True(t, os.IsNotExist(serr), "the text writer must confine its own output, not trust its caller")
	})
	t.Run("html fallback tmp sibling", func(t *testing.T) {
		root, outside, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "out.html")
		require.NoError(t, confineConversionPaths(src, tgt))
		require.NoError(t, os.Symlink(filepath.Join(outside, "pwn_tmp"), tgt+".tmp.txt"))
		err := (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
		t.Logf("convertPDFToHTML err=%v", err)
		_, serr := os.Lstat(filepath.Join(outside, "pwn_tmp"))
		assert.True(t, os.IsNotExist(serr), "HTML fallback wrote its temp file outside the root")
	})
	t.Run("html fallback final symlink leaf", func(t *testing.T) {
		root, outside, src := rv6ConvFixture(t)
		tgt := filepath.Join(root, "fin.html")
		require.NoError(t, os.Symlink(filepath.Join(outside, "pwn_html"), tgt))
		err := (&ConversionService{}).convertPDFToHTML(&models.ConversionJob{SourcePath: src, TargetPath: tgt})
		t.Logf("convertPDFToHTML err=%v", err)
		// The text fallback cannot reach its final write in this environment (unipdf text
		// extraction needs a licence), and pandoc / LibreOffice would write through the link
		// themselves, so the refusal must come from the pre-check before any tool runs.
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not a regular file")
		_, serr := os.Lstat(filepath.Join(outside, "pwn_html"))
		assert.True(t, os.IsNotExist(serr), "HTML fallback wrote the final file through a symlink leaf")
	})
}

// F4 (LibreOffice rename): the file LibreOffice leaves beside the target is moved into place
// only when it is a regular file inside the write roots; a planted symlink is never renamed
// over the target (c.File(job.TargetPath) would then serve the link's target).
func TestRV6_LibreOfficeOutputMoveRefusesLinks(t *testing.T) {
	root, outside, _ := rv6ConvFixture(t)
	secret := filepath.Join(outside, "secret")
	require.NoError(t, os.WriteFile(secret, []byte("S"), 0o600))
	tgt := filepath.Join(root, "doc.out.html")

	lo := filepath.Join(root, "doc.html")
	require.NoError(t, os.Symlink(secret, lo))
	err := moveLibreOfficeOutput(lo, tgt)
	assert.Error(t, err)
	_, serr := os.Lstat(tgt)
	assert.True(t, os.IsNotExist(serr), "a symlink was renamed over the conversion target")

	// golden-FALSE: a regular output file is moved into place
	require.NoError(t, os.Remove(lo))
	require.NoError(t, os.WriteFile(lo, []byte("<html/>"), 0o644))
	require.NoError(t, moveLibreOfficeOutput(lo, tgt))
	b, rerr := os.ReadFile(tgt)
	require.NoError(t, rerr)
	assert.Equal(t, "<html/>", string(b))

	// target outside the write roots
	require.NoError(t, os.WriteFile(lo, []byte("x"), 0o644))
	err = moveLibreOfficeOutput(lo, filepath.Join(outside, "moved.html"))
	assert.Error(t, err)
	_, serr = os.Lstat(filepath.Join(outside, "moved.html"))
	assert.True(t, os.IsNotExist(serr))
	assert.False(t, strings.Contains(fmt.Sprint(err), "%!"))

	// a link that resolves INSIDE the root passes the confine; only the regular-file check stops
	// it from being renamed over the target (reviewer mutant R6-11 survived without this case)
	inRoot := filepath.Join(root, "other.html")
	require.NoError(t, os.WriteFile(inRoot, []byte("OTHER"), 0o644))
	lo2 := filepath.Join(root, "doc2.html")
	require.NoError(t, os.Symlink(inRoot, lo2))
	tgt2 := filepath.Join(root, "doc2.out.html")
	assert.Error(t, moveLibreOfficeOutput(lo2, tgt2))
	_, serr = os.Lstat(tgt2)
	assert.True(t, os.IsNotExist(serr), "an in-root link was renamed over the conversion target")
}

// Class-exhaustive pass: WebDAVClient.DownloadFile opened the local file with os.Create, which
// follows a symlink planted at the download target after downloadFromWebDAV confined it.
func TestRV6_WebDAVDownloadFileRefusesSymlinkLeaf(t *testing.T) {
	base := t.TempDir()
	davRoot := filepath.Join(base, "dav")
	local := filepath.Join(base, "local")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{davRoot, local, outside} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "f.txt"), []byte("REMOTE"), 0o644))
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	cl := NewWebDAVClient(srv.URL, "", "")

	require.NoError(t, os.Symlink(filepath.Join(outside, "pwn"), filepath.Join(local, "f.txt")))
	assert.Error(t, cl.DownloadFile("/f.txt", filepath.Join(local, "f.txt")))
	_, serr := os.Lstat(filepath.Join(outside, "pwn"))
	assert.True(t, os.IsNotExist(serr), "DownloadFile wrote through a planted symlink")

	// golden-FALSE: a plain new file downloads
	require.NoError(t, cl.DownloadFile("/f.txt", filepath.Join(local, "g.txt")))
	b, err := os.ReadFile(filepath.Join(local, "g.txt"))
	require.NoError(t, err)
	assert.Equal(t, "REMOTE", string(b))
}
