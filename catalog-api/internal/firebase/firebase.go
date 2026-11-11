// Package firebase provides Firebase Admin SDK integration for Analytics
// event logging and Crashlytics non-fatal reporting.
//
// The Firebase app is initialized from the GOOGLE_APPLICATION_CREDENTIALS
// environment variable (path to a service-account JSON key). When
// unconfigured, the package runs in disabled mode — no crash, no panic,
// just a warning log. All methods become safe no-ops.
//
// §11.4.10: credential values are NEVER logged, only the resolved
// project ID (if available) is logged at init time.
package firebase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	firebase "firebase.google.com/go/v4"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

// ---------------------------------------------------------------------------
// Event / payload types
// ---------------------------------------------------------------------------

// Event represents a custom Analytics event to log via the Measurement
// Protocol.
type Event struct {
	Name   string
	Params map[string]string
}

// NonFatal represents a non-fatal error to report to Firebase Crashlytics.
type NonFatal struct {
	Message    string
	StackTrace string
	Severity   string
	Context    map[string]string
}

// ---------------------------------------------------------------------------
// Interfaces (disabled-safe)
// ---------------------------------------------------------------------------

// AnalyticsClient logs custom events.
type AnalyticsClient interface {
	LogEvent(ctx context.Context, event Event) error
}

// CrashlyticsClient reports non-fatal errors.
type CrashlyticsClient interface {
	ReportNonFatal(ctx context.Context, nf NonFatal) error
}

// ---------------------------------------------------------------------------
// App
// ---------------------------------------------------------------------------

// App wraps a firebase.App alongside Analytics + Crashlytics clients.
// When credentials are not configured, clients are safe no-ops.
type App struct {
	mu          sync.RWMutex
	firebase    *firebase.App
	project     string
	analytics   AnalyticsClient
	crashlytics CrashlyticsClient
	logger      *zap.Logger
	initialized bool
}

// Config carries optional overrides for the Firebase init.
type Config struct {
	// CredentialsFile overrides GOOGLE_APPLICATION_CREDENTIALS.
	CredentialsFile string
	// ProjectID overrides the project ID.
	ProjectID string
	// Logger receives startup messages. Nil = zap.NewNop().
	Logger *zap.Logger
	// HTTPClient for outbound calls. Nil = http.DefaultClient.
	HTTPClient *http.Client
}

// ---------------------------------------------------------------------------
// disabled no‑op implementations
// ---------------------------------------------------------------------------

type disabledAnalytics struct{}

func (disabledAnalytics) LogEvent(_ context.Context, _ Event) error { return nil }

type disabledCrashlytics struct{}

func (disabledCrashlytics) ReportNonFatal(_ context.Context, _ NonFatal) error { return nil }

// ---------------------------------------------------------------------------
// Analytics: GA4 Measurement Protocol
// ---------------------------------------------------------------------------

type measurementAnalytics struct {
	apiSecret     string
	measurementID string
	client        *http.Client
	logger        *zap.Logger
}

type mpPayload struct {
	ClientID string       `json:"client_id"`
	Events   []mpEvent    `json:"events"`
}

type mpEvent struct {
	Name   string            `json:"name"`
	Params map[string]string `json:"params,omitempty"`
}

func (m *measurementAnalytics) LogEvent(ctx context.Context, ev Event) error {
	if m == nil || m.client == nil {
		return nil
	}

	u := fmt.Sprintf("https://www.google-analytics.com/mp/collect?measurement_id=%s&api_secret=%s",
		m.measurementID, m.apiSecret)

	body, err := json.Marshal(mpPayload{
		ClientID: "catalogizer-server",
		Events:   []mpEvent{{Name: ev.Name, Params: ev.Params}},
	})
	if err != nil {
		m.logger.Warn("firebase: analytics marshal failed", zap.Error(err))
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		m.logger.Warn("firebase: analytics request failed", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		m.logger.Warn("firebase: analytics non-2xx", zap.Int("status", resp.StatusCode))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Crashlytics: Firestore‑backed REST API with OAuth2
// ---------------------------------------------------------------------------

type restCrashlytics struct {
	project  string
	client   *http.Client
	tokenSrc oauth2.TokenSource
	logger   *zap.Logger
}

func (r *restCrashlytics) ReportNonFatal(ctx context.Context, nf NonFatal) error {
	if r == nil || r.project == "" {
		return nil
	}

	if r.client == nil || r.tokenSrc == nil {
		return nil
	}

	tok, err := r.tokenSrc.Token()
	if err != nil {
		r.logger.Warn("firebase: crashlytics token acquisition failed", zap.Error(err))
		return nil
	}

	u := fmt.Sprintf("https://firebasecrashlytics.googleapis.com/v1/projects/%s/reports:reportNonFatal",
		r.project)

	payload := map[string]interface{}{
		"message":    nf.Message,
		"stackTrace": nf.StackTrace,
		"severity":   nf.Severity,
		"context":    nf.Context,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		r.logger.Warn("firebase: crashlytics marshal failed", zap.Error(err))
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return nil
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

	resp, err := r.client.Do(req)
	if err != nil {
		r.logger.Warn("firebase: crashlytics request failed", zap.Error(err))
		return nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		r.logger.Warn("firebase: crashlytics non-2xx", zap.Int("status", resp.StatusCode))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

const defaultTimeout = 10 * time.Second

// New initialises the Firebase Admin SDK App from
// GOOGLE_APPLICATION_CREDENTIALS (env var or config override).
//
// When credentials are absent the package runs in disabled mode — safe
// no‑ops, no panic, no crash.
func New(cfg Config) *App {
	logger := cfg.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	app := &App{logger: logger}

	credFile := cfg.CredentialsFile
	if credFile == "" {
		credFile = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if credFile == "" {
		logger.Warn("firebase: GOOGLE_APPLICATION_CREDENTIALS not set — disabled")
		app.analytics = disabledAnalytics{}
		app.crashlytics = disabledCrashlytics{}
		app.initialized = true
		return app
	}

	credJSON, err := os.ReadFile(credFile)
	if err != nil {
		logger.Warn("firebase: cannot read credentials file, disabled", zap.Error(err))
		app.analytics = disabledAnalytics{}
		app.crashlytics = disabledCrashlytics{}
		app.initialized = true
		return app
	}

	projectID := cfg.ProjectID
	if projectID == "" {
		projectID = extractProjectID(credJSON)
	}
	app.project = projectID

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultTimeout}
	}

	// Firebase Admin SDK App
	fbCtx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	opts := []option.ClientOption{option.WithCredentialsJSON(credJSON)}
	fbApp, fbErr := firebase.NewApp(fbCtx, nil, opts...)
	if fbErr != nil {
		logger.Warn("firebase: Admin SDK init failed, disabled", zap.Error(fbErr))
		app.analytics = disabledAnalytics{}
		app.crashlytics = disabledCrashlytics{}
		app.initialized = true
		return app
	}
	app.firebase = fbApp

	// OAuth2 token source for Crashlytics REST calls
	tsCtx, tsCancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer tsCancel()
	var tokenSrc oauth2.TokenSource
	jwtConf, jwtErr := google.JWTConfigFromJSON(credJSON,
		"https://www.googleapis.com/auth/firebase",
	)
	if jwtErr == nil {
		tokenSrc = jwtConf.TokenSource(tsCtx)
		logger.Info("firebase: OAuth2 token source created")
	} else {
		logger.Warn("firebase: JWT config failed, Crashlytics disabled", zap.Error(jwtErr))
	}

	// Analytics (Measurement Protocol)
	apiSecret := os.Getenv("FIREBASE_MEASUREMENT_API_SECRET")
	measurementID := os.Getenv("FIREBASE_MEASUREMENT_ID")
	if apiSecret != "" && measurementID != "" {
		app.analytics = &measurementAnalytics{
			apiSecret:     apiSecret,
			measurementID: measurementID,
			client:        httpClient,
			logger:        logger,
		}
		logger.Info("firebase: Analytics client ready")
	} else {
		logger.Warn("firebase: FIREBASE_MEASUREMENT_API_SECRET or FIREBASE_MEASUREMENT_ID not set — Analytics disabled")
		app.analytics = disabledAnalytics{}
	}

	// Crashlytics
	if projectID != "" && tokenSrc != nil {
		app.crashlytics = &restCrashlytics{
			project:  projectID,
			client:   httpClient,
			tokenSrc: tokenSrc,
			logger:   logger,
		}
		logger.Info("firebase: Crashlytics client ready", zap.String("project", projectID))
	} else {
		logger.Warn("firebase: project ID or token source unavailable — Crashlytics disabled")
		app.crashlytics = disabledCrashlytics{}
	}

	app.initialized = true
	return app
}

func extractProjectID(data []byte) string {
	var k struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(data, &k); err != nil {
		return ""
	}
	return k.ProjectID
}

// ---------------------------------------------------------------------------
// Accessors
// ---------------------------------------------------------------------------

func (a *App) Initialized() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.initialized
}

func (a *App) FirebaseSDK() *firebase.App {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.firebase
}

func (a *App) ProjectID() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.project
}

func (a *App) Enabled() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.firebase != nil
}

func (a *App) Analytics() AnalyticsClient {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.analytics == nil {
		return disabledAnalytics{}
	}
	return a.analytics
}

func (a *App) Crashlytics() CrashlyticsClient {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.crashlytics == nil {
		return disabledCrashlytics{}
	}
	return a.crashlytics
}

// ---------------------------------------------------------------------------
// Convenience helpers
// ---------------------------------------------------------------------------

func (a *App) LogEvent(ctx context.Context, name string, params map[string]string) error {
	return a.Analytics().LogEvent(ctx, Event{Name: name, Params: params})
}

func (a *App) ReportNonFatal(ctx context.Context, msg, stackTrace string) error {
	return a.Crashlytics().ReportNonFatal(ctx, NonFatal{
		Message:    msg,
		StackTrace: stackTrace,
		Severity:   "ERROR",
	})
}

// Predefined event names.
const (
	EventScanCompleted = "scan.completed"
	EventMediaFound    = "media.found"
	EventAuthLogin     = "auth.login"
	EventAPIError      = "api.error"
	EventStartup       = "startup"
	EventShutdown      = "shutdown"
)

// ---------------------------------------------------------------------------
// Gin middleware: capture HTTP 5xx → Crashlytics non‑fatal
// ---------------------------------------------------------------------------

// GinMiddleware returns a Gin handler that captures 5xx-level HTTP
// status codes AFTER the handler chain runs and reports them as
// non-fatal errors to Firebase Crashlytics.
//
// Usage:
//
//	fbApp := firebase.New(firebase.Config{Logger: logger})
//	router.Use(firebase.GinMiddleware(fbApp))
func GinMiddleware(fbApp *App) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		status := c.Writer.Status()
		if status >= 500 {
			fbApp.ReportNonFatal(c.Request.Context(),
				fmt.Sprintf("HTTP %d: %s %s", status, c.Request.Method, c.Request.URL.Path),
				fmt.Sprintf("Referer: %s\nUser-Agent: %s\nRemoteAddr: %s",
					c.Request.Referer(), c.Request.UserAgent(), c.Request.RemoteAddr),
			)
		}
	}
}

// ---------------------------------------------------------------------------
// Summary (safe for logging — NEVER credential values)
// ---------------------------------------------------------------------------

func (a *App) Summary() string {
	if !a.Enabled() {
		return "firebase: disabled (no credentials)"
	}
	b := &strings.Builder{}
	b.WriteString("firebase: enabled")
	if a.project != "" {
		b.WriteString(" project=")
		b.WriteString(a.project)
	}
	_, anOK := a.analytics.(*measurementAnalytics)
	_, crOK := a.crashlytics.(*restCrashlytics)
	b.WriteString(fmt.Sprintf(" analytics=%v crashlytics=%v", anOK, crOK))
	return b.String()
}
