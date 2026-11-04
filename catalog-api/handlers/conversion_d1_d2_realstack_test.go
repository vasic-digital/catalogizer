package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS (§11.4.289): real gin routes, real services, production migrations, real tools.
// Covers D1 (document routing + honest formats), D2 (real delete), D7 (no paths in error
// messages) and D10 (extension must match target format) of
// docs/qa/live-conv-docs-subs-r1-20261104.md.

func pdfRequest(s *convStack, src, dst, tf string) map[string]interface{} {
	return map[string]interface{}{"source_path": src, "target_path": dst, "source_format": "pdf",
		"target_format": tf, "conversion_type": "document", "quality": "high"}
}

func copyPDF(t *testing.T, s *convStack) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "services", "testdata", "pdf", "uni.pdf"))
	require.NoError(t, err)
	p := filepath.Join(s.dataDir, "doc.pdf")
	require.NoError(t, os.WriteFile(p, b, 0o600))
	return p
}

func TestD1_PDFToTextViaAPIReachesCompleted(t *testing.T) {
	if _, err := exec.LookPath("pdfinfo"); err != nil {
		t.Skip("poppler pdfinfo absent: cannot validate the fixture independently (hardware_not_present-class SKIP)")
	}
	s := newConvStack(t)
	src := copyPDF(t, s)
	tgt := filepath.Join(s.dataDir, "doc.txt")
	code, body := s.do(t, "POST", "/jobs", pdfRequest(s, src, tgt, "txt"))
	require.Equal(t, http.StatusOK, code, string(body))
	var job models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &job))
	require.NoError(t, s.svc.StartConversion(job.ID))
	m := pollJob(t, s, job.ID, "completed", "failed")
	require.Equal(t, "completed", m["status"], "error_message: %v", m["error_message"])
	out, err := os.ReadFile(tgt)
	require.NoError(t, err)
	assert.Contains(t, string(out), "Каталогизатор")
}

func TestD1_AdvertisedFormatsMatchRuntime(t *testing.T) {
	s := newConvStack(t)
	f := s.svc.GetSupportedFormats()
	hasCalibre := func() bool { _, err := exec.LookPath("ebook-convert"); return err == nil }()
	if !hasCalibre {
		for _, bad := range []string{"epub", "mobi", "pdf"} {
			assert.NotContains(t, f.Document.Output, bad, "must not advertise %s without ebook-convert", bad)
		}
		code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
			"source_path": filepath.Join(s.dataDir, "a.txt"), "target_path": filepath.Join(s.dataDir, "a.epub"),
			"source_format": "txt", "target_format": "epub", "conversion_type": "document"})
		assert.Equal(t, http.StatusBadRequest, code, string(body))
		assert.Contains(t, string(body), "not supported on this server")
	}
	// pdf -> png is rejected up front with a clear reason, not accepted and failed later
	src := copyPDF(t, s)
	code, body := s.do(t, "POST", "/jobs", pdfRequest(s, src, filepath.Join(s.dataDir, "p.png"), "png"))
	assert.Equal(t, http.StatusBadRequest, code, string(body))
	assert.NotContains(t, f.Document.Output, "png")
}

func TestD10_TargetExtensionMustMatchFormat(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg absent: audio is not advertised then")
	}
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "a.wav")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o600))
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(s.dataDir, "a.png"),
		"source_format": "wav", "target_format": "mp3", "conversion_type": "audio"})
	assert.Equal(t, http.StatusBadRequest, code, string(body))
	assert.Contains(t, string(body), "extension does not match")
	code, body = s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(s.dataDir, "a.mp3"),
		"source_format": "wav", "target_format": "mp3", "conversion_type": "audio"})
	assert.Equal(t, http.StatusOK, code, string(body))
}

func mkJob(t *testing.T, s *convStack, status, src, dst string) int {
	t.Helper()
	id, err := s.repo.CreateJob(&models.ConversionJob{UserID: s.userID, SourcePath: src, TargetPath: dst,
		SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image", Quality: "high",
		Status: status, CreatedAt: time.Now()})
	require.NoError(t, err)
	return id
}

func TestD2_DeleteRemovesTerminalJobsAndOutput(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	out := filepath.Join(s.dataDir, "done.jpg")
	require.NoError(t, os.WriteFile(out, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0o600))
	for _, st := range []string{models.ConversionStatusCompleted, models.ConversionStatusFailed, models.ConversionStatusCancelled, models.ConversionStatusPending} {
		dst := out
		if st != models.ConversionStatusCompleted {
			dst = filepath.Join(s.dataDir, st+".jpg")
		}
		id := mkJob(t, s, st, src, dst)
		code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
		assert.Equal(t, http.StatusOK, code, "%s: %s", st, body)
		code, _ = s.do(t, "GET", "/jobs/"+convItoa(id), nil)
		assert.Equal(t, http.StatusNotFound, code, "%s row must be gone", st)
	}
	_, err := os.Lstat(out)
	assert.True(t, os.IsNotExist(err), "completed job's output must be removed")
	_, err = os.Lstat(src)
	assert.NoError(t, err, "the source is never removed")
}

func TestD2_DeleteRunningIs409AndKeepsRow(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	id := mkJob(t, s, models.ConversionStatusRunning, src, filepath.Join(s.dataDir, "r.jpg"))
	code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
	assert.Equal(t, http.StatusConflict, code, string(body))
	j, err := s.repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusRunning, j.Status)
}

func TestD2_DeleteOwnerOnlyAndConfinedFileRemoval(t *testing.T) {
	s := newConvStack(t)
	_, tokB := s.addUser(t, "mallory", models.Permissions{
		models.PermissionConversionCreate, models.PermissionConversionView, models.PermissionConversionManage})
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	out := filepath.Join(s.dataDir, "mine.jpg")
	require.NoError(t, os.WriteFile(out, []byte("x"), 0o600))
	id := mkJob(t, s, models.ConversionStatusCompleted, src, out)
	assert.Equal(t, http.StatusNotFound, s.doAs(t, tokB, "DELETE", "/jobs/"+convItoa(id)), "foreign job ids answer 404 like unknown ones")
	_, err := os.Lstat(out)
	require.NoError(t, err, "a refused delete must not touch the file")
	_, err = s.repo.GetJob(id)
	require.NoError(t, err, "a refused delete must keep the row")

	// confinement: a completed job pointing OUTSIDE the conversion roots (legacy/forged row)
	// and one whose target is a symlink to an outside file: row goes, outside file stays.
	outside := filepath.Join(t.TempDir(), "precious.txt")
	require.NoError(t, os.WriteFile(outside, []byte("keep"), 0o600))
	id2 := mkJob(t, s, models.ConversionStatusCompleted, src, outside)
	link := filepath.Join(s.dataDir, "lnk.jpg")
	require.NoError(t, os.Symlink(outside, link))
	id3 := mkJob(t, s, models.ConversionStatusCompleted, src, link)
	for _, id := range []int{id2, id3} {
		code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
		assert.Equal(t, http.StatusOK, code, string(body))
	}
	b, err := os.ReadFile(outside)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(b), "files outside the conversion roots must never be removed")
}

func TestD7_FailedJobErrorMessageHasNoServerPaths(t *testing.T) {
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("ImageMagick absent (never stubbed)")
	}
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "garbage.png")
	require.NoError(t, os.WriteFile(src, []byte("this is not a png"), 0o600))
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(s.dataDir, "g.jpg"),
		"source_format": "png", "target_format": "jpg", "conversion_type": "image", "quality": "high"})
	require.Equal(t, http.StatusOK, code, string(body))
	var job models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &job))
	require.NoError(t, s.svc.StartConversion(job.ID))
	m := pollJob(t, s, job.ID, "failed", "completed")
	require.Equal(t, "failed", m["status"])
	msg, _ := m["error_message"].(string)
	require.NotEmpty(t, msg)
	assert.False(t, strings.Contains(msg, s.dataDir), "error_message leaks a server path: %q", msg)
}

// F1: a completed job whose target_path is a symlink to ANOTHER file inside the roots must not
// delete that other file (the link itself is not a regular file: nothing is removed).
func TestRev_F1_InRootSymlinkTargetLeavesVictimIntact(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	victim := filepath.Join(s.dataDir, "victim.jpg")
	require.NoError(t, os.WriteFile(victim, []byte("victim"), 0o600))
	link := filepath.Join(s.dataDir, "link.jpg")
	require.NoError(t, os.Symlink(victim, link))
	id := mkJob(t, s, models.ConversionStatusCompleted, src, link)
	code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
	require.Equal(t, http.StatusOK, code, string(body))
	b, err := os.ReadFile(victim)
	require.NoError(t, err, "the symlink's victim inside the roots must survive")
	assert.Equal(t, "victim", string(b))
}

// F7 M1: target == source (and a hard link to it) is never removed.
func TestRev_F7_TargetEqualsSourceIsKept(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "same.jpg")
	require.NoError(t, os.WriteFile(src, []byte("src"), 0o600))
	hard := filepath.Join(s.dataDir, "hard.jpg")
	require.NoError(t, os.Link(src, hard))
	for _, tgt := range []string{src, hard} {
		id := mkJob(t, s, models.ConversionStatusCompleted, src, tgt)
		code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
		require.Equal(t, http.StatusOK, code, string(body))
		_, err := os.Lstat(src)
		require.NoError(t, err, "the source must survive a delete whose target is %s", tgt)
	}
}

// F7 M2: only regular files are removed; an (empty) directory target stays.
func TestRev_F7_DirectoryTargetIsKept(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	dir := filepath.Join(s.dataDir, "adir.jpg")
	require.NoError(t, os.Mkdir(dir, 0o700))
	id := mkJob(t, s, models.ConversionStatusCompleted, src, dir)
	code, body := s.do(t, "DELETE", "/jobs/"+convItoa(id), nil)
	require.Equal(t, http.StatusOK, code, string(body))
	_, err := os.Lstat(dir)
	assert.NoError(t, err, "a directory must not be removed")
}

// F6/F7 M3: the SQL delete is conditional on the status the caller saw.
func TestRev_F7_DeleteJobIfIsStatusConditional(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "s.png")
	writePNG(t, src)
	id := mkJob(t, s, models.ConversionStatusRunning, src, filepath.Join(s.dataDir, "r.jpg"))
	ok, err := s.repo.DeleteJobIf(id, models.ConversionStatusPending, models.ConversionStatusCompleted,
		models.ConversionStatusFailed, models.ConversionStatusCancelled)
	require.NoError(t, err)
	assert.False(t, ok, "a running row must not be deleted")
	_, err = s.repo.GetJob(id)
	assert.NoError(t, err)
	id2 := mkJob(t, s, models.ConversionStatusCompleted, src, filepath.Join(s.dataDir, "c.jpg"))
	ok, err = s.repo.DeleteJobIf(id2, models.ConversionStatusPending)
	require.NoError(t, err)
	assert.False(t, ok, "snapshot status pending != stored completed: row stays (F6)")
}
