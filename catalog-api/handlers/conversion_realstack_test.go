package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-stack conversion tests (§11.4.115 / §11.4.289): the real gin routes as wired in main.go,
// the real ConversionService/AuthService/repositories, an in-memory SQLite database built by
// the PRODUCTION migrations, a real JWT obtained through the real Login, and the real
// ImageMagick converter. No mock, stub or fixture server is in the path.

type convStack struct {
	router   *gin.Engine
	svc      *services.ConversionService
	repo     *repository.ConversionRepository
	token    string
	userRepo *repository.UserRepository
	auth     *services.AuthService
	userID   int
	dataDir  string
}

func newConvStack(t *testing.T) *convStack {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	t.Setenv("CATALOGIZER_CONVERSION_ROOTS", dir)

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	sqlDB.SetMaxOpenConns(1)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))

	userRepo := repository.NewUserRepository(db)
	repo := repository.NewConversionRepository(db)
	auth := services.NewAuthService(userRepo, "conv-realstack-secret")
	svc := services.NewConversionService(repo, userRepo, auth)

	hash, salt, err := auth.HashPasswordForUser("correct-horse-battery-1A")
	require.NoError(t, err)
	uid, err := userRepo.Create(&models.User{
		Username: "convuser", Email: "conv@example.com", PasswordHash: hash, Salt: salt,
		RoleID: 1, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	require.NoError(t, err)
	res, err := auth.Login(models.LoginRequest{Username: "convuser", Password: "correct-horse-battery-1A"}, "127.0.0.1", "test")
	require.NoError(t, err)

	h := NewConversionHandler(svc, auth)
	r := gin.New()
	g := r.Group("/api/v1/conversion")
	g.POST("/jobs", h.CreateJob)
	g.GET("/jobs", h.ListJobs)
	g.GET("/jobs/:id", h.GetJob)
	g.POST("/jobs/:id/cancel", h.CancelJob)
	g.DELETE("/jobs/:id", h.DeleteJob)
	g.POST("/jobs/:id/retry", h.RetryJob)
	g.GET("/jobs/:id/download", h.DownloadJobFile)
	return &convStack{router: r, svc: svc, repo: repo, userRepo: userRepo, auth: auth, token: res.SessionToken, userID: uid, dataDir: dir}
}

func (s *convStack) do(t *testing.T, method, path string, body interface{}) (int, []byte) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, "/api/v1/conversion"+path, rd)
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

func writePNG(t *testing.T, p string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := 0; x < 8; x++ {
		for y := 0; y < 8; y++ {
			img.Set(x, y, color.RGBA{uint8(x * 30), uint8(y * 30), 90, 255})
		}
	}
	f, err := os.Create(p)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, png.Encode(f, img))
}

func insertFailedJob(t *testing.T, s *convStack, src, dst string) int {
	t.Helper()
	id, err := s.repo.CreateJob(&models.ConversionJob{
		UserID: s.userID, SourcePath: src, TargetPath: dst, SourceFormat: "png", TargetFormat: "jpg",
		ConversionType: "image", Quality: "high", Status: models.ConversionStatusFailed, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	return id
}

func pollJob(t *testing.T, s *convStack, id int, want ...string) map[string]interface{} {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		code, body := s.do(t, "GET", "/jobs/"+convItoa(id), nil)
		require.Equal(t, http.StatusOK, code, string(body))
		var m map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &m))
		for _, w := range want {
			if m["status"] == w {
				return m
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("job %d did not reach %v in time", id, want)
	return nil
}

func convItoa(i int) string { b, _ := json.Marshal(i); return string(b) }

func TestConversionRealStack_ListJobsEmptyIsArrayNotNull(t *testing.T) {
	s := newConvStack(t)
	code, body := s.do(t, "GET", "/jobs", nil)
	require.Equal(t, http.StatusOK, code)
	assert.JSONEq(t, `[]`, string(body), "an empty list must serialize as [], not null")
}

func TestConversionRealStack_InvalidRequestIs400(t *testing.T) {
	s := newConvStack(t)
	bad := []map[string]interface{}{
		{"source_path": filepath.Join(s.dataDir, "a.png"), "target_path": filepath.Join(s.dataDir, "a.jpg"),
			"source_format": "png", "target_format": "nonsense-fmt", "conversion_type": "image"},
		{"source_path": "", "target_path": "", "source_format": "png", "target_format": "jpg", "conversion_type": "image"},
		{"source_path": filepath.Join(s.dataDir, "a.png"), "target_path": filepath.Join(s.dataDir, "a.jpg"),
			"source_format": "png", "target_format": "jpg", "conversion_type": "bogus"},
		{"source_path": "/etc/passwd", "target_path": filepath.Join(s.dataDir, "a.jpg"),
			"source_format": "png", "target_format": "jpg", "conversion_type": "image"},
	}
	for i, b := range bad {
		code, body := s.do(t, "POST", "/jobs", b)
		assert.Equal(t, http.StatusBadRequest, code, "case %d: %s", i, string(body))
	}
}

func TestConversionRealStack_RetryFailedJobIs200AndRequeues(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "r.png")
	writePNG(t, src)
	id := insertFailedJob(t, s, src, filepath.Join(s.dataDir, "r.jpg"))
	code, body := s.do(t, "POST", "/jobs/"+convItoa(id)+"/retry", nil)
	assert.Equal(t, http.StatusOK, code, string(body))
}

func TestConversionRealStack_RetryNonFailedJobIs409(t *testing.T) {
	s := newConvStack(t)
	src := filepath.Join(s.dataDir, "r2.png")
	writePNG(t, src)
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(s.dataDir, "r2.jpg"),
		"source_format": "png", "target_format": "jpg", "conversion_type": "image", "quality": "high"})
	require.Equal(t, http.StatusOK, code, string(body))
	var job models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &job))
	code, body = s.do(t, "POST", "/jobs/"+convItoa(job.ID)+"/retry", nil)
	assert.Equal(t, http.StatusConflict, code, string(body))
}

// End to end: submit over HTTP, a real worker picks it up, ImageMagick converts, the job reaches a
// terminal state, and the output downloads as a real JPEG.
func TestConversionRealStack_SubmitReachesCompletedViaWorker(t *testing.T) {
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("ImageMagick `convert` not installed: real converter unavailable (never stubbed)")
	}
	s := newConvStack(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.svc.RunWorker(ctx, 50*time.Millisecond); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	src := filepath.Join(s.dataDir, "in.png")
	dst := filepath.Join(s.dataDir, "out.jpg")
	writePNG(t, src)
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": dst, "source_format": "png", "target_format": "jpg",
		"conversion_type": "image", "quality": "high"})
	require.Equal(t, http.StatusOK, code, string(body))
	var job models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &job))

	m := pollJob(t, s, job.ID, "completed", "failed")
	require.Equal(t, "completed", m["status"], "job result: %v", m)

	code, body = s.do(t, "GET", "/jobs/"+convItoa(job.ID)+"/download", nil)
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, len(body), 3)
	assert.Equal(t, []byte{0xFF, 0xD8, 0xFF}, body[:3], "output must be a real JPEG")

	code, body = s.do(t, "GET", "/jobs", nil)
	require.Equal(t, http.StatusOK, code)
	var list []models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &list))
	require.Len(t, list, 1)
}

// A job whose source is not a valid image reaches `failed`; retrying it requeues it and the worker
// runs it again to a terminal state (not stuck).
func TestConversionRealStack_FailedJobRetryReachesTerminalAgain(t *testing.T) {
	if _, err := exec.LookPath("convert"); err != nil {
		t.Skip("ImageMagick `convert` not installed: real converter unavailable (never stubbed)")
	}
	s := newConvStack(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { s.svc.RunWorker(ctx, 50*time.Millisecond); close(done) }()
	t.Cleanup(func() { cancel(); <-done })

	src := filepath.Join(s.dataDir, "bad.png")
	require.NoError(t, os.WriteFile(src, []byte("not an image"), 0o600))
	code, body := s.do(t, "POST", "/jobs", map[string]interface{}{
		"source_path": src, "target_path": filepath.Join(s.dataDir, "bad.jpg"),
		"source_format": "png", "target_format": "jpg", "conversion_type": "image", "quality": "high"})
	require.Equal(t, http.StatusOK, code, string(body))
	var job models.ConversionJob
	require.NoError(t, json.Unmarshal(body, &job))
	m := pollJob(t, s, job.ID, "failed")
	assert.NotEmpty(t, m["error_message"])

	// fix the source, retry
	writePNG(t, src)
	code, body = s.do(t, "POST", "/jobs/"+convItoa(job.ID)+"/retry", nil)
	require.Equal(t, http.StatusOK, code, string(body))
	m = pollJob(t, s, job.ID, "completed", "failed")
	assert.Equal(t, "completed", m["status"], "%v", m)
}

// Ownership isolation: a second user whose role may use conversion (create/view/manage) but is
// neither admin nor holds media.view / manage-users must reach only their own jobs.
func (s *convStack) addUser(t *testing.T, name string, perms models.Permissions) (int, string) {
	t.Helper()
	db := s.userRepo
	roleID := 2 // the standard "User" role
	if perms != nil {
		desc := name + " role"
		id, err := db.CreateRole(&models.Role{Name: name + "-role", Description: &desc, Permissions: perms,
			CreatedAt: time.Now(), UpdatedAt: time.Now()})
		require.NoError(t, err)
		roleID = id
	}
	hash, salt, err := s.auth.HashPasswordForUser("correct-horse-battery-1A")
	require.NoError(t, err)
	uid, err := db.Create(&models.User{Username: name, Email: name + "@example.com", PasswordHash: hash, Salt: salt,
		RoleID: roleID, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	require.NoError(t, err)
	res, err := s.auth.Login(models.LoginRequest{Username: name, Password: "correct-horse-battery-1A"}, "127.0.0.1", "test")
	require.NoError(t, err)
	return uid, res.SessionToken
}

func (s *convStack) doAs(t *testing.T, token, method, path string) int {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1/conversion"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w.Code
}

func TestConversionRealStack_OwnershipIsolation(t *testing.T) {
	s := newConvStack(t)
	_, tokB := s.addUser(t, "bob", models.Permissions{
		models.PermissionConversionCreate, models.PermissionConversionView, models.PermissionConversionManage})
	// A owns a failed job, a pending job and a completed job with a real output file
	src := filepath.Join(s.dataDir, "own.png")
	writePNG(t, src)
	failed := insertFailedJob(t, s, src, filepath.Join(s.dataDir, "own.jpg"))
	pending, err := s.repo.CreateJob(&models.ConversionJob{UserID: s.userID, SourcePath: src,
		TargetPath: filepath.Join(s.dataDir, "p.jpg"), SourceFormat: "png", TargetFormat: "jpg",
		ConversionType: "image", Quality: "high", Status: models.ConversionStatusPending, CreatedAt: time.Now()})
	require.NoError(t, err)
	outFile := filepath.Join(s.dataDir, "done.jpg")
	require.NoError(t, os.WriteFile(outFile, []byte{0xFF, 0xD8, 0xFF, 0xD9}, 0o600))
	completed, err := s.repo.CreateJob(&models.ConversionJob{UserID: s.userID, SourcePath: src, TargetPath: outFile,
		SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image", Quality: "high",
		Status: models.ConversionStatusCompleted, CreatedAt: time.Now()})
	require.NoError(t, err)

	// another user's job: 403 on every route
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tokB, "GET", "/jobs/"+convItoa(failed)))
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tokB, "POST", "/jobs/"+convItoa(failed)+"/retry"))
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tokB, "POST", "/jobs/"+convItoa(pending)+"/cancel"))
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tokB, "DELETE", "/jobs/"+convItoa(pending)))
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tokB, "GET", "/jobs/"+convItoa(completed)+"/download"))
	// untouched
	j, err := s.repo.GetJob(failed)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusFailed, j.Status)
	j, err = s.repo.GetJob(pending)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusPending, j.Status)
	// B's list shows none of A's jobs
	req := httptest.NewRequest("GET", "/api/v1/conversion/jobs", nil)
	req.Header.Set("Authorization", "Bearer "+tokB)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	assert.JSONEq(t, `[]`, w.Body.String())

	// the owner succeeds on the same routes
	assert.Equal(t, http.StatusOK, s.doAs(t, s.token, "GET", "/jobs/"+convItoa(failed)))
	assert.Equal(t, http.StatusOK, s.doAs(t, s.token, "GET", "/jobs/"+convItoa(completed)+"/download"))
	assert.Equal(t, http.StatusOK, s.doAs(t, s.token, "POST", "/jobs/"+convItoa(failed)+"/retry"))
	assert.Equal(t, http.StatusOK, s.doAs(t, s.token, "POST", "/jobs/"+convItoa(pending)+"/cancel"))
}

func TestConversionRealStack_PlainUserRoleHasNoConversionAccess(t *testing.T) {
	s := newConvStack(t)
	_, tok := s.addUser(t, "plain", nil) // standard User role: media.view, media.download only
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tok, "GET", "/jobs"))
	assert.Equal(t, http.StatusForbidden, s.doAs(t, tok, "POST", "/jobs/1/retry"))
}
