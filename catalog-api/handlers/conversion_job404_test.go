package handlers

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
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

// CAT-JOB-404-1 (operator decision 2026-11-13, FOLLOW_UPS R2-N7 / F4): a non-owner asking for
// another user's conversion job id gets 404 indistinguishable from a nonexistent id; the owner
// and administrators get the real answer. Real gin routes, real services, real SQLite built by
// the production migrations, real JWTs through the real Login. No mock in the path.

type job404Stack struct {
	router     *gin.Engine
	repo       *repository.ConversionRepository
	dataDir    string
	ownerTok   string
	ownerID    int
	otherTok   string
	adminTok   string
	viewerOnly string // token of a user whose role has NO conversion/manage permission but view:media
}

func newJob404Stack(t *testing.T) *job404Stack {
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
	// role 3: may use conversions and also holds view:media (the permission the old GetJob
	// accepted as a licence to read ANOTHER user's job); role 4: view:media only
	_, err = sqlDB.Exec(`INSERT INTO roles (id, name, description, permissions, is_system) VALUES
		(3, 'ConvUser', 'conversion user', '["conversion.view","conversion.create","conversion.manage","view:media"]', 0),
		(4, 'MediaViewer', 'media viewer', '["view:media","conversion.view"]', 0)`)
	require.NoError(t, err)

	userRepo := repository.NewUserRepository(db)
	repo := repository.NewConversionRepository(db)
	auth := services.NewAuthService(userRepo, "job404-secret")
	svc := services.NewConversionService(repo, userRepo, auth)

	mk := func(name string, role int) (string, int) {
		hash, salt, err := auth.HashPasswordForUser("correct-horse-battery-1A")
		require.NoError(t, err)
		id, err := userRepo.Create(&models.User{Username: name, Email: name + "@example.com", PasswordHash: hash,
			Salt: salt, RoleID: role, IsActive: true, CreatedAt: time.Now(), UpdatedAt: time.Now()})
		require.NoError(t, err)
		res, err := auth.Login(models.LoginRequest{Username: name, Password: "correct-horse-battery-1A"}, "127.0.0.1", "test")
		require.NoError(t, err)
		return res.SessionToken, id
	}
	s := &job404Stack{repo: repo, dataDir: dir}
	s.ownerTok, s.ownerID = mk("owner", 3)
	s.otherTok, _ = mk("other", 3)
	s.adminTok, _ = mk("admin", 1)
	s.viewerOnly, _ = mk("viewer", 4)

	h := NewConversionHandler(svc, auth)
	r := gin.New()
	g := r.Group("/api/v1/conversion")
	g.GET("/jobs/:id", h.GetJob)
	g.POST("/jobs/:id/cancel", h.CancelJob)
	g.DELETE("/jobs/:id", h.DeleteJob)
	g.POST("/jobs/:id/retry", h.RetryJob)
	g.GET("/jobs/:id/download", h.DownloadJobFile)
	s.router = r
	return s
}

func (s *job404Stack) call(method, path, tok string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1/conversion"+path, nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	s.router.ServeHTTP(w, req)
	return w
}

func (s *job404Stack) newJob(t *testing.T, status string, withOutput bool) int {
	t.Helper()
	dst := filepath.Join(s.dataDir, "out-"+status+".jpg")
	if withOutput {
		require.NoError(t, os.WriteFile(dst, []byte("\xff\xd8\xff\xd9"), 0o600))
	}
	id, err := s.repo.CreateJob(&models.ConversionJob{
		UserID: s.ownerID, SourcePath: filepath.Join(s.dataDir, "in.png"), TargetPath: dst,
		SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image", Quality: "high",
		Status: status, CreatedAt: time.Now(),
	})
	require.NoError(t, err)
	return id
}

type job404Endpoint struct {
	name       string
	method     string
	suffix     string
	jobStatus  string
	withOutput bool
}

// the complete enumeration of job-by-id routes (main.go conversionGroup)
var job404Endpoints = []job404Endpoint{
	{"GetJob", "GET", "", models.ConversionStatusPending, false},
	{"CancelJob", "POST", "/cancel", models.ConversionStatusPending, false},
	{"DeleteJob", "DELETE", "", models.ConversionStatusFailed, false},
	{"RetryJob", "POST", "/retry", models.ConversionStatusFailed, false},
	{"DownloadJobFile", "GET", "/download", models.ConversionStatusCompleted, true},
}

func TestJob404_OwnerAndAdminGetRealAnswer(t *testing.T) {
	for _, ep := range job404Endpoints {
		for who, tokf := range map[string]func(*job404Stack) string{
			"owner": func(s *job404Stack) string { return s.ownerTok },
			"admin": func(s *job404Stack) string { return s.adminTok },
		} {
			t.Run(ep.name+"/"+who, func(t *testing.T) {
				s := newJob404Stack(t)
				id := s.newJob(t, ep.jobStatus, ep.withOutput)
				w := s.call(ep.method, "/jobs/"+convItoa(id)+ep.suffix, tokf(s))
				assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
			})
		}
	}
}

func TestJob404_NonOwnerSeesExactlyWhatANonexistentIdShows(t *testing.T) {
	for _, ep := range job404Endpoints {
		for who, tokf := range map[string]func(*job404Stack) string{
			"other-conversion-user": func(s *job404Stack) string { return s.otherTok },
			"view-media-only":       func(s *job404Stack) string { return s.viewerOnly },
		} {
			t.Run(ep.name+"/"+who, func(t *testing.T) {
				s := newJob404Stack(t)
				id := s.newJob(t, ep.jobStatus, ep.withOutput)
				tok := tokf(s)
				missing := s.call(ep.method, "/jobs/999999"+ep.suffix, tok)
				foreign := s.call(ep.method, "/jobs/"+convItoa(id)+ep.suffix, tok)
				// the baseline itself must be a plain 404 for the same caller (permission gates
				// that run before the job lookup are job-independent and identical for both)
				if missing.Code == http.StatusNotFound {
					assert.Equal(t, http.StatusNotFound, foreign.Code, foreign.Body.String())
				}
				assert.Equal(t, missing.Code, foreign.Code, "status differs from a nonexistent id: %s", foreign.Body.String())
				assert.Equal(t, missing.Body.String(), foreign.Body.String(), "body differs from a nonexistent id")
				assert.Equal(t, missing.Header(), foreign.Header(), "headers differ from a nonexistent id")
				// and the foreign job is untouched
				got, err := s.repo.GetJob(id)
				require.NoError(t, err, "a non-owner must not delete the job")
				assert.Equal(t, ep.jobStatus, got.Status, "a non-owner must not change the job")
			})
		}
	}
}

func TestJob404_NonexistentIdIs404(t *testing.T) {
	for _, ep := range job404Endpoints {
		t.Run(ep.name, func(t *testing.T) {
			s := newJob404Stack(t)
			w := s.call(ep.method, "/jobs/999999"+ep.suffix, s.ownerTok)
			assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
			w = s.call(ep.method, "/jobs/999999"+ep.suffix, s.adminTok)
			assert.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
		})
	}
}

// the handler-level mapping: even if a service ever returned the old forbidden error it must not
// reach the client as a 403
func TestJob404_WriteJobErrorMapsForbiddenTo404SameBodyAsNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	run := func(err error) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		writeJobError(c, err, "x")
		return w
	}
	nf := run(models.ErrConversionJobNotFound)
	fb := run(models.ErrConversionForbidden)
	assert.Equal(t, http.StatusNotFound, nf.Code)
	assert.Equal(t, http.StatusNotFound, fb.Code)
	assert.Equal(t, nf.Body.String(), fb.Body.String())
}
