package firebase

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// disabled-mode tests (no real Firebase credentials)
// ---------------------------------------------------------------------------

func TestNewDisabledNoEnv(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")

	app := New(Config{})
	if !app.Initialized() {
		t.Fatal("Initialized() should be true even in disabled mode")
	}
	if app.Enabled() {
		t.Fatal("Enabled() should be false when credentials are absent")
	}
	if app.FirebaseSDK() != nil {
		t.Fatal("FirebaseSDK() should be nil in disabled mode")
	}
	if app.ProjectID() != "" {
		t.Fatalf("ProjectID() should be empty, got %q", app.ProjectID())
	}

	// All client methods must be safe no-ops.
	if err := app.LogEvent(context.Background(), "test", nil); err != nil {
		t.Fatalf("LogEvent should no-op, got error: %v", err)
	}
	if err := app.ReportNonFatal(context.Background(), "test", "stack"); err != nil {
		t.Fatalf("ReportNonFatal should no-op, got error: %v", err)
	}

	// Summary must not contain credentials.
	sum := app.Summary()
	if sum == "" {
		t.Fatal("Summary() should not be empty")
	}
	if !contains(sum, "disabled") {
		t.Fatalf("Summary() should say 'disabled', got %q", sum)
	}
}

func TestNewDisabledNonExistentFile(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/tmp/nonexistent-firebase-cred.json")

	app := New(Config{})
	if !app.Initialized() {
		t.Fatal("Initialized() should be true")
	}
	if app.Enabled() {
		t.Fatal("Enabled() should be false for non-existent file")
	}
}

// ---------------------------------------------------------------------------
// Analytics client — disabled no-op
// ---------------------------------------------------------------------------

func TestDisabledAnalytics(t *testing.T) {
	ac := disabledAnalytics{}
	if err := ac.LogEvent(context.Background(), Event{Name: "test"}); err != nil {
		t.Fatalf("disabledAnalytics.LogEvent should no-op: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Crashlytics client — disabled no-op
// ---------------------------------------------------------------------------

func TestDisabledCrashlytics(t *testing.T) {
	cc := disabledCrashlytics{}
	if err := cc.ReportNonFatal(context.Background(), NonFatal{Message: "test"}); err != nil {
		t.Fatalf("disabledCrashlytics.ReportNonFatal should no-op: %v", err)
	}
}

// ---------------------------------------------------------------------------
// extractProjectID function
// ---------------------------------------------------------------------------

func TestExtractProjectID(t *testing.T) {
	data := []byte(`{"project_id": "my-project-123"}`)
	if id := extractProjectID(data); id != "my-project-123" {
		t.Fatalf("expected my-project-123, got %q", id)
	}

	if id := extractProjectID([]byte(`{}`)); id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
	if id := extractProjectID([]byte(`not json`)); id != "" {
		t.Fatalf("expected empty for invalid JSON, got %q", id)
	}
}

// ---------------------------------------------------------------------------
// Config with nil logger — must not panic
// ---------------------------------------------------------------------------

func TestNewNilLogger(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	app := New(Config{Logger: nil})
	if app == nil {
		t.Fatal("New with nil logger should not return nil")
	}
}

// ---------------------------------------------------------------------------
// Predefined event names are non-empty.
// ---------------------------------------------------------------------------

func TestPredefinedEvents(t *testing.T) {
	events := []string{
		EventScanCompleted,
		EventMediaFound,
		EventAuthLogin,
		EventAPIError,
		EventStartup,
		EventShutdown,
	}
	for _, e := range events {
		if e == "" {
			t.Fatal("predefined event name must not be empty")
		}
	}
}

// ---------------------------------------------------------------------------
// Summary output — disabled mode
// ---------------------------------------------------------------------------

func TestSummaryDisabled(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	app := New(Config{})
	sum := app.Summary()
	if !contains(sum, "disabled") {
		t.Fatalf("disabled summary should mention 'disabled', got: %s", sum)
	}
}

// ---------------------------------------------------------------------------
// Interface compliance — compile-time checks
// ---------------------------------------------------------------------------

var _ AnalyticsClient = (*disabledAnalytics)(nil)
var _ CrashlyticsClient = (*disabledCrashlytics)(nil)

// ---------------------------------------------------------------------------
// measurementAnalytics with test logger — ensures no panic
// ---------------------------------------------------------------------------

func TestMeasurementAnalyticsWithNoopLogger(t *testing.T) {
	ma := &measurementAnalytics{
		apiSecret:     "test-secret",
		measurementID: "G-TEST",
		client:        nil, // will be replaced if needed
		logger:        zap.NewNop(),
	}
	if ma == nil {
		t.Fatal("measurementAnalytics should not be nil")
	}
	// The LogEvent will fail because client is nil; that's OK, it
	// must not panic and must return nil (best-effort semantics).
	_ = ma.LogEvent(context.Background(), Event{Name: "test.event"})
}

// ---------------------------------------------------------------------------
// restCrashlytics with empty project — must no-op
// ---------------------------------------------------------------------------

func TestRestCrashlyticsEmptyProject(t *testing.T) {
	rc := &restCrashlytics{
		project: "",
		logger:  zap.NewNop(),
	}
	if err := rc.ReportNonFatal(context.Background(), NonFatal{
		Message: "test",
	}); err != nil {
		t.Fatalf("restCrashlytics with empty project should no-op, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func contains(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Verify ginMiddleware builds correctly (used from main.go).
func TestGinMiddlewareNotNil(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	app := New(Config{})
	mw := GinMiddleware(app)
	if mw == nil {
		t.Fatal("GinMiddleware should not return nil")
	}
}

type captureCrashlytics struct{ got []NonFatal }

func (c *captureCrashlytics) ReportNonFatal(_ context.Context, nf NonFatal) error {
	c.got = append(c.got, nf)
	return nil
}

// R2-4: a 5xx report to Crashlytics carries neither the share token of the path nor a ?token= of the Referer.
func TestGinMiddleware_CrashlyticsReportRedactsPathAndReferer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cc := &captureCrashlytics{}
	app := &App{crashlytics: cc}
	router := gin.New()
	router.Use(GinMiddleware(app))
	router.GET("/api/v1/logs/share/:token", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/logs/share/SHARESECRETTOK", nil)
	req.Header.Set("Referer", "https://app/x?token=REFERERSECRET&a=1")
	router.ServeHTTP(httptest.NewRecorder(), req)
	if len(cc.got) != 1 {
		t.Fatalf("expected 1 report (control needle), got %d", len(cc.got))
	}
	all := cc.got[0].Message + "\n" + cc.got[0].StackTrace
	for _, s := range []string{"SHARESECRETTOK", "REFERERSECRET"} {
		if strings.Contains(all, s) {
			t.Errorf("crashlytics report leaks %q:\n%s", s, all)
		}
	}
	if !strings.Contains(all, ":redacted") || !strings.Contains(all, "a=1") {
		t.Errorf("report lost its non-secret content:\n%s", all)
	}
}
