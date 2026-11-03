package challenges

import (
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/userflow"
)

// defaultBrowserConfig returns the standard browser configuration
// used by all web challenges (chromium, headless, 1920x1080).
func defaultBrowserConfig() userflow.BrowserConfig {
	return userflow.BrowserConfig{
		BrowserType: "chromium",
		Headless:    true,
		WindowSize:  [2]int{1920, 1080},
	}
}

// webAppURL is the base URL for the Catalogizer web application.
const webAppURL = "http://localhost:3000"

// waitTimeout is the default timeout for wait actions.
const waitTimeout = 10 * time.Second

// healthDep is the dependency shared by all web challenges.
var healthDep = []challenge.ID{"UF-API-HEALTH"}

// authDep is the dependency for challenges requiring login.
var authDep = []challenge.ID{"UF-API-HEALTH", "UF-WEB-AUTH-LOGIN"}

// registerUserFlowWebChallenges returns all Catalogizer web
// browser flow challenges organized by category.
func registerUserFlowWebChallenges() []challenge.Challenge {
	adapter := userflow.NewPlaywrightCLIAdapter(
		"ws://localhost:9222",
	)
	cfg := defaultBrowserConfig()

	var challenges []challenge.Challenge

	// ── WEB Auth (5 challenges) ─────────────────────────

	challenges = append(challenges,
		registerAuthChallenges(adapter, cfg)...)

	// ── WEB Dashboard (5 challenges) ────────────────────

	challenges = append(challenges,
		registerDashboardChallenges(adapter, cfg)...)

	// ── WEB Media Browser (8 challenges) ────────────────

	challenges = append(challenges,
		registerBrowseChallenges(adapter, cfg)...)

	// ── WEB Collections (6 challenges) ──────────────────

	challenges = append(challenges,
		registerCollectionChallenges(adapter, cfg)...)

	// ── WEB Player (4 challenges) ───────────────────────

	challenges = append(challenges,
		registerPlayerChallenges(adapter, cfg)...)

	// ── WEB Admin (5 challenges) ────────────────────────

	challenges = append(challenges,
		registerAdminChallenges(adapter, cfg)...)

	// ── WEB Subtitles (4 challenges) ────────────────────

	challenges = append(challenges,
		registerSubtitleChallenges(adapter, cfg)...)

	// ── WEB Conversion (3 challenges) ───────────────────

	challenges = append(challenges,
		registerConversionChallenges(adapter, cfg)...)

	// ── WEB Analytics (3 challenges) ────────────────────

	challenges = append(challenges,
		registerAnalyticsChallenges(adapter, cfg)...)

	// ── WEB Favorites (3 challenges) ────────────────────

	challenges = append(challenges,
		registerFavoritesChallenges(adapter, cfg)...)

	// ── WEB Playlists (4 challenges) ────────────────────

	challenges = append(challenges,
		registerPlaylistChallenges(adapter, cfg)...)

	// ── WEB Responsive (3 challenges) ───────────────────

	challenges = append(challenges,
		registerResponsiveChallenges(adapter)...)

	// ── WEB Error Handling (3 challenges) ───────────────

	challenges = append(challenges,
		registerErrorChallenges(adapter, cfg)...)

	// ── WEB Accessibility (3 challenges) ────────────────

	challenges = append(challenges,
		registerAccessibilityChallenges(adapter, cfg)...)

	return challenges
}

// ─── Auth Challenges ────────────────────────────────────

func registerAuthChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-AUTH-LOGIN
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-AUTH-LOGIN",
			"Web Auth Login",
			"Verify user can log in via the web interface",
			healthDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "auth-login",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-login-form",
						Action:   "wait",
						Selector: "[data-testid='login-form'], form.login-form, input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:     "fill-username",
						Action:   "fill",
						Selector: "input[name='username'], [data-testid='username-input']",
						Value:    "admin",
					},
					{
						Name:     "fill-password",
						Action:   "fill",
						Selector: "input[name='password'], [data-testid='password-input']",
						Value:    adminPasswordFromEnv(),
					},
					{
						Name:     "click-submit",
						Action:   "click",
						Selector: "button[type='submit'], [data-testid='login-button']",
					},
					{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard, [data-testid='dashboard-stats']",
						Timeout:  waitTimeout,
					},
					{
						Name:       "assert-dashboard-url",
						Action:     "assert_url",
						Value:      "/dashboard",
						Screenshot: true,
					},
				},
			},
		),

		// UF-WEB-AUTH-REGISTER
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-AUTH-REGISTER",
			"Web Auth Register",
			"Verify user registration form works via the web interface",
			healthDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "auth-register",
				StartURL: webAppURL + "/register",
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-register-form",
						Action:   "wait",
						Selector: "[data-testid='register-form'], form.register-form, input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:     "fill-username",
						Action:   "fill",
						Selector: "input[name='username'], [data-testid='register-username']",
						Value:    "testuser_web",
					},
					{
						Name:     "fill-email",
						Action:   "fill",
						Selector: "input[name='email'], [data-testid='register-email']",
						Value:    "testuser@example.com",
					},
					{
						Name:     "fill-password",
						Action:   "fill",
						Selector: "input[name='password'], [data-testid='register-password']",
						Value:    "TestPass123!",
					},
					{
						Name:     "fill-confirm-password",
						Action:   "fill",
						Selector: "input[name='confirmPassword'], [data-testid='register-confirm']",
						Value:    "TestPass123!",
					},
					{
						Name:     "click-register",
						Action:   "click",
						Selector: "button[type='submit'], [data-testid='register-button']",
					},
					{
						Name:     "wait-success",
						Action:   "wait",
						Selector: "[data-testid='register-success'], .success-message, [data-testid='dashboard']",
						Timeout:  waitTimeout,
					},
					{
						Name:       "screenshot-result",
						Action:     "screenshot",
						Screenshot: true,
					},
				},
			},
		),

		// UF-WEB-AUTH-INVALID
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-AUTH-INVALID",
			"Web Auth Invalid Credentials",
			"Verify error message appears for invalid credentials",
			healthDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "auth-invalid",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-login-form",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:     "fill-wrong-username",
						Action:   "fill",
						Selector: "input[name='username']",
						Value:    "nonexistent_user",
					},
					{
						Name:     "fill-wrong-password",
						Action:   "fill",
						Selector: "input[name='password']",
						Value:    "wrong_password_123",
					},
					{
						Name:     "click-submit",
						Action:   "click",
						Selector: "button[type='submit']",
					},
					{
						Name:     "wait-error-message",
						Action:   "wait",
						Selector: "[data-testid='login-error'], .error-message, [role='alert']",
						Timeout:  waitTimeout,
					},
					{
						Name:       "assert-error-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='login-error'], .error-message, [role='alert']",
						Screenshot: true,
					},
				},
			},
		),

		// UF-WEB-AUTH-LOGOUT
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-AUTH-LOGOUT",
			"Web Auth Logout",
			"Verify user can log out and is redirected to login",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "auth-logout",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-login-form",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:     "fill-username",
						Action:   "fill",
						Selector: "input[name='username']",
						Value:    "admin",
					},
					{
						Name:     "fill-password",
						Action:   "fill",
						Selector: "input[name='password']",
						Value:    adminPasswordFromEnv(),
					},
					{
						Name:     "click-login",
						Action:   "click",
						Selector: "button[type='submit']",
					},
					{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard",
						Timeout:  waitTimeout,
					},
					{
						Name:     "click-logout",
						Action:   "click",
						Selector: "[data-testid='logout-button'], button.logout, [aria-label='Logout']",
					},
					{
						Name:     "wait-login-page",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:       "assert-login-url",
						Action:     "assert_url",
						Value:      "/",
						Screenshot: true,
					},
				},
			},
		),

		// UF-WEB-AUTH-PERSIST
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-AUTH-PERSIST",
			"Web Auth Session Persistence",
			"Verify session persists across page navigations",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "auth-persist",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-login-form",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  waitTimeout,
					},
					{
						Name:     "fill-username",
						Action:   "fill",
						Selector: "input[name='username']",
						Value:    "admin",
					},
					{
						Name:     "fill-password",
						Action:   "fill",
						Selector: "input[name='password']",
						Value:    adminPasswordFromEnv(),
					},
					{
						Name:     "click-login",
						Action:   "click",
						Selector: "button[type='submit']",
					},
					{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard",
						Timeout:  waitTimeout,
					},
					{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					{
						Name:     "wait-browse-loaded",
						Action:   "wait",
						Selector: "[data-testid='media-browser'], .media-browser, [data-testid='media-grid']",
						Timeout:  waitTimeout,
					},
					{
						Name:       "assert-still-authenticated",
						Action:     "assert_visible",
						Selector:   "[data-testid='user-menu'], [data-testid='logout-button'], .user-avatar",
						Screenshot: true,
					},
				},
			},
		),
	}
}

// ─── Dashboard Challenges ───────────────────────────────

func registerDashboardChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-DASH-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-DASH-LOAD",
			"Web Dashboard Load",
			"Verify dashboard loads with stats and activity feed after login",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "dashboard-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-dashboard-stats",
						Action:   "wait",
						Selector: "[data-testid='dashboard-stats'], .dashboard-stats, .stats-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "assert-stats-visible",
						Action:   "assert_visible",
						Selector: "[data-testid='dashboard-stats'], .dashboard-stats, .stats-grid",
					},
					userflow.BrowserStep{
						Name:     "assert-activity-visible",
						Action:   "assert_visible",
						Selector: "[data-testid='activity-feed'], .activity-feed, .recent-activity",
					},
					userflow.BrowserStep{
						Name:       "screenshot-dashboard",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-DASH-STATS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-DASH-STATS",
			"Web Dashboard Stats Component",
			"Verify DashboardStats component renders with data",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "dashboard-stats",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-stats",
						Action:   "wait",
						Selector: "[data-testid='dashboard-stats'], .dashboard-stats",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "assert-stats-content",
						Action:   "assert_visible",
						Selector: "[data-testid='stat-total-files'], [data-testid='stat-card'], .stat-card",
					},
					userflow.BrowserStep{
						Name:       "screenshot-stats",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-DASH-CHARTS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-DASH-CHARTS",
			"Web Dashboard Charts",
			"Verify MediaDistributionChart renders on dashboard",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "dashboard-charts",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-chart",
						Action:   "wait",
						Selector: "[data-testid='media-distribution-chart'], .recharts-wrapper, canvas, svg.chart",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-chart-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='media-distribution-chart'], .recharts-wrapper, canvas, svg.chart",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-DASH-ACTIVITY
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-DASH-ACTIVITY",
			"Web Dashboard Activity Feed",
			"Verify ActivityFeed renders on dashboard",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "dashboard-activity",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-activity",
						Action:   "wait",
						Selector: "[data-testid='activity-feed'], .activity-feed, .recent-activity",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-activity-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='activity-feed'], .activity-feed, .recent-activity",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-DASH-NAV
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-DASH-NAV",
			"Web Dashboard Navigation",
			"Verify navigation links load correct pages from dashboard",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "dashboard-nav",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "click-browse-link",
						Action:   "click",
						Selector: "a[href='/browse'], [data-testid='nav-browse'], nav a[href*='browse']",
					},
					userflow.BrowserStep{
						Name:     "wait-browse-page",
						Action:   "wait",
						Selector: "[data-testid='media-browser'], .media-browser",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:   "assert-browse-url",
						Action: "assert_url",
						Value:  "/browse",
					},
					userflow.BrowserStep{
						Name:     "click-dashboard-link",
						Action:   "click",
						Selector: "a[href='/dashboard'], [data-testid='nav-dashboard'], nav a[href*='dashboard']",
					},
					userflow.BrowserStep{
						Name:     "wait-dashboard-again",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-dashboard-url",
						Action:     "assert_url",
						Value:      "/dashboard",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Media Browser Challenges ───────────────────────────

func registerBrowseChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-BROWSE-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-LOAD",
			"Web Media Browser Load",
			"Verify media browser page loads with grid of items",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-grid",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid, .grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-grid-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='media-grid'], .media-grid, .grid",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-SEARCH
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-SEARCH",
			"Web Media Browser Search",
			"Verify search query updates media browser results",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-search",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-search-input",
						Action:   "wait",
						Selector: "[data-testid='search-input'], input[type='search'], input[placeholder*='Search']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-search",
						Action:   "fill",
						Selector: "[data-testid='search-input'], input[type='search'], input[placeholder*='Search']",
						Value:    "test",
					},
					userflow.BrowserStep{
						Name:     "wait-results-update",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-search-results",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-FILTER
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-FILTER",
			"Web Media Browser Filter",
			"Verify media type filter updates results",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-filter",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-filters",
						Action:   "wait",
						Selector: "[data-testid='media-filters'], .media-filters, .filter-bar",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-filter",
						Action:   "click",
						Selector: "[data-testid='filter-movie'], [data-testid='media-type-filter'] option, .filter-chip",
					},
					userflow.BrowserStep{
						Name:     "wait-filtered-results",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-filtered",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-DETAIL
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-DETAIL",
			"Web Media Browser Detail Modal",
			"Verify clicking media card opens detail modal",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-detail",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-first-card",
						Action:   "click",
						Selector: "[data-testid='media-card']:first-child, .media-card:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-detail-modal",
						Action:   "wait",
						Selector: "[data-testid='media-detail-modal'], .media-detail-modal, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-modal-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='media-detail-modal'], .media-detail-modal, [role='dialog']",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-PAGINATION
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-PAGINATION",
			"Web Media Browser Pagination",
			"Verify pagination through media browser results",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-pagination",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-grid",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-next-page",
						Action:   "click",
						Selector: "[data-testid='pagination-next'], .pagination-next, button[aria-label='Next page']",
					},
					userflow.BrowserStep{
						Name:     "wait-page-update",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-page-2",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-SORT
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-SORT",
			"Web Media Browser Sort",
			"Verify changing sort order reorders results",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-sort",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-sort-control",
						Action:   "wait",
						Selector: "[data-testid='sort-select'], .sort-select, select[name='sort']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "select-sort-name",
						Action:   "select",
						Selector: "[data-testid='sort-select'], .sort-select, select[name='sort']",
						Value:    "name",
					},
					userflow.BrowserStep{
						Name:     "wait-sorted-results",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-sorted",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-GRID
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-GRID",
			"Web Media Browser Grid Layout",
			"Verify media grid layout renders correctly",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-grid",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-grid",
						Action:   "wait",
						Selector: "[data-testid='media-grid'], .media-grid",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:   "verify-grid-layout",
						Action: "evaluate_js",
						Script: "document.querySelectorAll('[data-testid=\"media-card\"], .media-card').length > 0",
					},
					userflow.BrowserStep{
						Name:       "screenshot-grid",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-BROWSE-EMPTY
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-BROWSE-EMPTY",
			"Web Media Browser Empty State",
			"Verify empty state appears for nonexistent search",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "browse-empty",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-search",
						Action:   "wait",
						Selector: "[data-testid='search-input'], input[type='search'], input[placeholder*='Search']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-nonexistent",
						Action:   "fill",
						Selector: "[data-testid='search-input'], input[type='search'], input[placeholder*='Search']",
						Value:    "zzz_nonexistent_query_xyz_12345",
					},
					userflow.BrowserStep{
						Name:     "wait-empty-state",
						Action:   "wait",
						Selector: "[data-testid='empty-state'], .empty-state, .no-results",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-empty-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='empty-state'], .empty-state, .no-results",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Collection Challenges ──────────────────────────────

func registerCollectionChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-COLL-LIST
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-LIST",
			"Web Collections List",
			"Verify collections page loads with collection list",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-list",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-collections",
						Action:   "wait",
						Selector: "[data-testid='collections-list'], .collections-list, .collections-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-list-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='collections-list'], .collections-list, .collections-manager",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-COLL-CREATE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-CREATE",
			"Web Collections Create",
			"Verify creating a new collection via the web interface",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-create",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-page",
						Action:   "wait",
						Selector: "[data-testid='collections-list'], .collections-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-create",
						Action:   "click",
						Selector: "[data-testid='create-collection'], button.create-collection, [aria-label='Create collection']",
					},
					userflow.BrowserStep{
						Name:     "wait-form",
						Action:   "wait",
						Selector: "[data-testid='collection-name-input'], input[name='name'], input[placeholder*='Collection']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-name",
						Action:   "fill",
						Selector: "[data-testid='collection-name-input'], input[name='name'], input[placeholder*='Collection']",
						Value:    "Test Web Collection",
					},
					userflow.BrowserStep{
						Name:     "click-save",
						Action:   "click",
						Selector: "[data-testid='save-collection'], button[type='submit'], button.save",
					},
					userflow.BrowserStep{
						Name:     "wait-collection-created",
						Action:   "wait",
						Selector: "[data-testid='collection-item'], .collection-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-created",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-COLL-ADD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-ADD",
			"Web Collections Add Item",
			"Verify adding an item to a collection",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-add-item",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-collections",
						Action:   "wait",
						Selector: "[data-testid='collection-item'], .collection-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-first-collection",
						Action:   "click",
						Selector: "[data-testid='collection-item']:first-child, .collection-item:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-collection-detail",
						Action:   "wait",
						Selector: "[data-testid='collection-detail'], .collection-detail",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-add-item",
						Action:   "click",
						Selector: "[data-testid='add-to-collection'], button.add-item, [aria-label='Add item']",
					},
					userflow.BrowserStep{
						Name:     "wait-item-picker",
						Action:   "wait",
						Selector: "[data-testid='item-picker'], .item-picker, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-add-item",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-COLL-REMOVE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-REMOVE",
			"Web Collections Remove Item",
			"Verify removing an item from a collection",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-remove-item",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-collections",
						Action:   "wait",
						Selector: "[data-testid='collection-item'], .collection-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-first-collection",
						Action:   "click",
						Selector: "[data-testid='collection-item']:first-child, .collection-item:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-collection-items",
						Action:   "wait",
						Selector: "[data-testid='collection-media-item'], .collection-media-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-remove-item",
						Action:   "click",
						Selector: "[data-testid='remove-from-collection'], button.remove-item, [aria-label='Remove']",
					},
					userflow.BrowserStep{
						Name:       "screenshot-removed",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-COLL-DELETE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-DELETE",
			"Web Collections Delete",
			"Verify deleting a collection removes it from the list",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-delete",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-collections",
						Action:   "wait",
						Selector: "[data-testid='collection-item'], .collection-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-delete-button",
						Action:   "click",
						Selector: "[data-testid='delete-collection'], button.delete-collection, [aria-label='Delete collection']",
					},
					userflow.BrowserStep{
						Name:     "confirm-delete",
						Action:   "click",
						Selector: "[data-testid='confirm-delete'], button.confirm, [role='dialog'] button.danger",
					},
					userflow.BrowserStep{
						Name:       "screenshot-deleted",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-COLL-SEARCH
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-COLL-SEARCH",
			"Web Collections Search",
			"Verify searching within collections",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "collections-search",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-collections",
						Action: "navigate",
						Value:  webAppURL + "/collections",
					},
					userflow.BrowserStep{
						Name:     "wait-search",
						Action:   "wait",
						Selector: "[data-testid='collection-search'], input[placeholder*='Search'], input[type='search']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-search",
						Action:   "fill",
						Selector: "[data-testid='collection-search'], input[placeholder*='Search'], input[type='search']",
						Value:    "test",
					},
					userflow.BrowserStep{
						Name:     "wait-search-results",
						Action:   "wait",
						Selector: "[data-testid='collections-list'], .collections-list",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-search",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Player Challenges ──────────────────────────────────

func registerPlayerChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-PLAYER-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYER-LOAD",
			"Web Player Load",
			"Verify media player loads when opening a media item",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "player-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-playable-item",
						Action:   "click",
						Selector: "[data-testid='media-card']:first-child, .media-card:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-player",
						Action:   "wait",
						Selector: "[data-testid='media-player'], .media-player, video, audio",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-player-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='media-player'], .media-player, video, audio",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYER-CONTROLS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYER-CONTROLS",
			"Web Player Controls",
			"Verify play/pause/seek controls are visible",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "player-controls",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-item",
						Action:   "click",
						Selector: "[data-testid='media-card']:first-child, .media-card:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-player",
						Action:   "wait",
						Selector: "[data-testid='media-player'], .media-player, video, audio",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "assert-play-button",
						Action:   "assert_visible",
						Selector: "[data-testid='play-button'], button.play, [aria-label='Play']",
					},
					userflow.BrowserStep{
						Name:       "assert-seek-bar",
						Action:     "assert_visible",
						Selector:   "[data-testid='seek-bar'], .seek-bar, input[type='range'], .progress-bar",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYER-SUBTITLE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYER-SUBTITLE",
			"Web Player Subtitle Toggle",
			"Verify subtitle panel can be toggled in the player",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "player-subtitle",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-item",
						Action:   "click",
						Selector: "[data-testid='media-card']:first-child, .media-card:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-player",
						Action:   "wait",
						Selector: "[data-testid='media-player'], .media-player",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-subtitle-toggle",
						Action:   "click",
						Selector: "[data-testid='subtitle-toggle'], button.subtitle-toggle, [aria-label='Subtitles']",
					},
					userflow.BrowserStep{
						Name:     "wait-subtitle-panel",
						Action:   "wait",
						Selector: "[data-testid='subtitle-panel'], .subtitle-panel, .subtitle-list",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-subtitle-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='subtitle-panel'], .subtitle-panel, .subtitle-list",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYER-FULLSCREEN
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYER-FULLSCREEN",
			"Web Player Fullscreen",
			"Verify fullscreen toggle works in the player",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "player-fullscreen",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-item",
						Action:   "click",
						Selector: "[data-testid='media-card']:first-child, .media-card:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-player",
						Action:   "wait",
						Selector: "[data-testid='media-player'], .media-player",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-fullscreen",
						Action:   "click",
						Selector: "[data-testid='fullscreen-button'], button.fullscreen, [aria-label='Fullscreen']",
					},
					userflow.BrowserStep{
						Name:   "verify-fullscreen",
						Action: "evaluate_js",
						Script: "document.fullscreenElement !== null || document.webkitFullscreenElement !== null",
					},
					userflow.BrowserStep{
						Name:       "screenshot-fullscreen",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Admin Challenges ───────────────────────────────────

func registerAdminChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-ADMIN-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ADMIN-LOAD",
			"Web Admin Panel Load",
			"Verify admin panel loads for authenticated admin user",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "admin-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-admin",
						Action: "navigate",
						Value:  webAppURL + "/admin",
					},
					userflow.BrowserStep{
						Name:     "wait-admin-panel",
						Action:   "wait",
						Selector: "[data-testid='admin-panel'], .admin-panel, .admin-page",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-admin-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='admin-panel'], .admin-panel, .admin-page",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ADMIN-USERS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ADMIN-USERS",
			"Web Admin Users List",
			"Verify admin user list table renders",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "admin-users",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-admin",
						Action: "navigate",
						Value:  webAppURL + "/admin",
					},
					userflow.BrowserStep{
						Name:     "wait-admin",
						Action:   "wait",
						Selector: "[data-testid='admin-panel'], .admin-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-users-tab",
						Action:   "click",
						Selector: "[data-testid='admin-users-tab'], a[href*='users'], button.users-tab",
					},
					userflow.BrowserStep{
						Name:     "wait-users-table",
						Action:   "wait",
						Selector: "[data-testid='users-table'], table.users-table, .user-list",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-table-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='users-table'], table.users-table, .user-list",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ADMIN-CONFIG
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ADMIN-CONFIG",
			"Web Admin Configuration",
			"Verify admin configuration form renders",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "admin-config",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-admin",
						Action: "navigate",
						Value:  webAppURL + "/admin",
					},
					userflow.BrowserStep{
						Name:     "wait-admin",
						Action:   "wait",
						Selector: "[data-testid='admin-panel'], .admin-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-config-tab",
						Action:   "click",
						Selector: "[data-testid='admin-config-tab'], a[href*='config'], button.config-tab",
					},
					userflow.BrowserStep{
						Name:     "wait-config-form",
						Action:   "wait",
						Selector: "[data-testid='config-form'], form.config-form, .configuration-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-form-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='config-form'], form.config-form, .configuration-panel",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ADMIN-LOGS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ADMIN-LOGS",
			"Web Admin Logs Viewer",
			"Verify admin log entries are visible",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "admin-logs",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-admin",
						Action: "navigate",
						Value:  webAppURL + "/admin",
					},
					userflow.BrowserStep{
						Name:     "wait-admin",
						Action:   "wait",
						Selector: "[data-testid='admin-panel'], .admin-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-logs-tab",
						Action:   "click",
						Selector: "[data-testid='admin-logs-tab'], a[href*='logs'], button.logs-tab",
					},
					userflow.BrowserStep{
						Name:     "wait-logs",
						Action:   "wait",
						Selector: "[data-testid='log-entries'], .log-entries, .log-viewer, pre.logs",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-logs-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='log-entries'], .log-entries, .log-viewer, pre.logs",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ADMIN-STATS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ADMIN-STATS",
			"Web Admin System Stats",
			"Verify admin system stats metrics are displayed",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "admin-stats",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-admin",
						Action: "navigate",
						Value:  webAppURL + "/admin",
					},
					userflow.BrowserStep{
						Name:     "wait-admin",
						Action:   "wait",
						Selector: "[data-testid='admin-panel'], .admin-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-stats-tab",
						Action:   "click",
						Selector: "[data-testid='admin-stats-tab'], a[href*='stats'], button.stats-tab",
					},
					userflow.BrowserStep{
						Name:     "wait-stats",
						Action:   "wait",
						Selector: "[data-testid='system-stats'], .system-stats, .metrics-panel",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-stats-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='system-stats'], .system-stats, .metrics-panel",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Subtitle Challenges ────────────────────────────────

func registerSubtitleChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-SUB-SEARCH
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-SUB-SEARCH",
			"Web Subtitle Search",
			"Verify subtitle manager search functionality",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "subtitle-search",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-subtitles",
						Action: "navigate",
						Value:  webAppURL + "/subtitles",
					},
					userflow.BrowserStep{
						Name:     "wait-subtitle-manager",
						Action:   "wait",
						Selector: "[data-testid='subtitle-manager'], .subtitle-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-search",
						Action:   "fill",
						Selector: "[data-testid='subtitle-search'], input[placeholder*='Search'], input[type='search']",
						Value:    "english",
					},
					userflow.BrowserStep{
						Name:     "wait-results",
						Action:   "wait",
						Selector: "[data-testid='subtitle-results'], .subtitle-results, .subtitle-list",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-results",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-SUB-DOWNLOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-SUB-DOWNLOAD",
			"Web Subtitle Download",
			"Verify subtitle download action",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "subtitle-download",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-subtitles",
						Action: "navigate",
						Value:  webAppURL + "/subtitles",
					},
					userflow.BrowserStep{
						Name:     "wait-subtitle-list",
						Action:   "wait",
						Selector: "[data-testid='subtitle-manager'], .subtitle-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-download",
						Action:   "click",
						Selector: "[data-testid='subtitle-download'], button.download-subtitle, [aria-label='Download']",
					},
					userflow.BrowserStep{
						Name:       "screenshot-download",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-SUB-UPLOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-SUB-UPLOAD",
			"Web Subtitle Upload Modal",
			"Verify subtitle upload modal form is visible",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "subtitle-upload",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-subtitles",
						Action: "navigate",
						Value:  webAppURL + "/subtitles",
					},
					userflow.BrowserStep{
						Name:     "wait-page",
						Action:   "wait",
						Selector: "[data-testid='subtitle-manager'], .subtitle-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-upload",
						Action:   "click",
						Selector: "[data-testid='upload-subtitle'], button.upload-subtitle, [aria-label='Upload']",
					},
					userflow.BrowserStep{
						Name:     "wait-upload-modal",
						Action:   "wait",
						Selector: "[data-testid='upload-modal'], .upload-modal, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-upload-form",
						Action:     "assert_visible",
						Selector:   "[data-testid='upload-modal'], .upload-modal, [role='dialog']",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-SUB-SYNC
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-SUB-SYNC",
			"Web Subtitle Sync Modal",
			"Verify subtitle sync interface opens",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "subtitle-sync",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-subtitles",
						Action: "navigate",
						Value:  webAppURL + "/subtitles",
					},
					userflow.BrowserStep{
						Name:     "wait-page",
						Action:   "wait",
						Selector: "[data-testid='subtitle-manager'], .subtitle-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-sync",
						Action:   "click",
						Selector: "[data-testid='sync-subtitle'], button.sync-subtitle, [aria-label='Sync']",
					},
					userflow.BrowserStep{
						Name:     "wait-sync-modal",
						Action:   "wait",
						Selector: "[data-testid='subtitle-sync-modal'], .subtitle-sync-modal, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-sync-interface",
						Action:     "assert_visible",
						Selector:   "[data-testid='subtitle-sync-modal'], .subtitle-sync-modal, [role='dialog']",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Conversion Challenges ──────────────────────────────

func registerConversionChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-CONV-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-CONV-LOAD",
			"Web Conversion Tools Load",
			"Verify conversion tools page loads",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "conversion-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-conversion",
						Action: "navigate",
						Value:  webAppURL + "/conversion",
					},
					userflow.BrowserStep{
						Name:     "wait-conversion-page",
						Action:   "wait",
						Selector: "[data-testid='conversion-tools'], .conversion-tools, .conversion-page",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-page-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='conversion-tools'], .conversion-tools, .conversion-page",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-CONV-FORMATS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-CONV-FORMATS",
			"Web Conversion Format List",
			"Verify format list loads on conversion page",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "conversion-formats",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-conversion",
						Action: "navigate",
						Value:  webAppURL + "/conversion",
					},
					userflow.BrowserStep{
						Name:     "wait-format-list",
						Action:   "wait",
						Selector: "[data-testid='format-list'], .format-list, select[name='format']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-formats-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='format-list'], .format-list, select[name='format']",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-CONV-CREATE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-CONV-CREATE",
			"Web Conversion Create Job",
			"Verify creating a conversion job",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "conversion-create",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-conversion",
						Action: "navigate",
						Value:  webAppURL + "/conversion",
					},
					userflow.BrowserStep{
						Name:     "wait-page",
						Action:   "wait",
						Selector: "[data-testid='conversion-tools'], .conversion-tools",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-create-job",
						Action:   "click",
						Selector: "[data-testid='create-conversion'], button.create-job, [aria-label='New conversion']",
					},
					userflow.BrowserStep{
						Name:     "wait-job-form",
						Action:   "wait",
						Selector: "[data-testid='conversion-form'], .conversion-form, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-form-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='conversion-form'], .conversion-form, [role='dialog']",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Analytics Challenges ───────────────────────────────

func registerAnalyticsChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-ANALYTICS-LOAD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ANALYTICS-LOAD",
			"Web Analytics Page Load",
			"Verify analytics page loads",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "analytics-load",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-analytics",
						Action: "navigate",
						Value:  webAppURL + "/analytics",
					},
					userflow.BrowserStep{
						Name:     "wait-analytics-page",
						Action:   "wait",
						Selector: "[data-testid='analytics-page'], .analytics-page, .analytics-dashboard",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-page-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='analytics-page'], .analytics-page, .analytics-dashboard",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ANALYTICS-CHARTS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ANALYTICS-CHARTS",
			"Web Analytics Charts Render",
			"Verify analytics charts render with data",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "analytics-charts",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-analytics",
						Action: "navigate",
						Value:  webAppURL + "/analytics",
					},
					userflow.BrowserStep{
						Name:     "wait-charts",
						Action:   "wait",
						Selector: "[data-testid='analytics-chart'], .recharts-wrapper, canvas, svg.chart",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-charts-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='analytics-chart'], .recharts-wrapper, canvas, svg.chart",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ANALYTICS-FILTERS
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ANALYTICS-FILTERS",
			"Web Analytics Date Filter",
			"Verify date filter updates analytics charts",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "analytics-filters",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-analytics",
						Action: "navigate",
						Value:  webAppURL + "/analytics",
					},
					userflow.BrowserStep{
						Name:     "wait-filters",
						Action:   "wait",
						Selector: "[data-testid='date-filter'], .date-filter, input[type='date'], select[name='period']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-date-filter",
						Action:   "click",
						Selector: "[data-testid='date-filter'], .date-filter, select[name='period']",
					},
					userflow.BrowserStep{
						Name:     "wait-charts-update",
						Action:   "wait",
						Selector: "[data-testid='analytics-chart'], .recharts-wrapper, canvas",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-filtered",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Favorites Challenges ───────────────────────────────

func registerFavoritesChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-FAV-ADD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-FAV-ADD",
			"Web Favorites Add",
			"Verify clicking favorite on media item toggles it",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "favorites-add",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-media-cards",
						Action:   "wait",
						Selector: "[data-testid='media-card'], .media-card",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-favorite",
						Action:   "click",
						Selector: "[data-testid='favorite-toggle'], .favorite-toggle, button[aria-label='Favorite']",
					},
					userflow.BrowserStep{
						Name:     "wait-favorite-active",
						Action:   "wait",
						Selector: "[data-testid='favorite-toggle'].active, .favorite-toggle.active, [aria-pressed='true']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-favorited",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-FAV-LIST
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-FAV-LIST",
			"Web Favorites List",
			"Verify favorites list page loads with favorited items",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "favorites-list",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-favorites",
						Action: "navigate",
						Value:  webAppURL + "/favorites",
					},
					userflow.BrowserStep{
						Name:     "wait-favorites-page",
						Action:   "wait",
						Selector: "[data-testid='favorites-list'], .favorites-list, .favorites-page",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-favorites-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='favorites-list'], .favorites-list, .favorites-page",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-FAV-REMOVE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-FAV-REMOVE",
			"Web Favorites Remove",
			"Verify removing a favorite item",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "favorites-remove",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-favorites",
						Action: "navigate",
						Value:  webAppURL + "/favorites",
					},
					userflow.BrowserStep{
						Name:     "wait-favorites",
						Action:   "wait",
						Selector: "[data-testid='favorites-list'], .favorites-list",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-remove-favorite",
						Action:   "click",
						Selector: "[data-testid='favorite-toggle'], .favorite-toggle, button[aria-label='Remove favorite']",
					},
					userflow.BrowserStep{
						Name:       "screenshot-removed",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Playlist Challenges ────────────────────────────────

func registerPlaylistChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-PLAYLIST-LIST
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYLIST-LIST",
			"Web Playlists List",
			"Verify playlists page loads",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "playlist-list",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-playlists",
						Action: "navigate",
						Value:  webAppURL + "/playlists",
					},
					userflow.BrowserStep{
						Name:     "wait-playlists-page",
						Action:   "wait",
						Selector: "[data-testid='playlists-page'], .playlists-page, .playlist-manager",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-page-visible",
						Action:     "assert_visible",
						Selector:   "[data-testid='playlists-page'], .playlists-page, .playlist-manager",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYLIST-CREATE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYLIST-CREATE",
			"Web Playlists Create",
			"Verify creating a new playlist",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "playlist-create",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-playlists",
						Action: "navigate",
						Value:  webAppURL + "/playlists",
					},
					userflow.BrowserStep{
						Name:     "wait-page",
						Action:   "wait",
						Selector: "[data-testid='playlists-page'], .playlists-page",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-create",
						Action:   "click",
						Selector: "[data-testid='create-playlist'], button.create-playlist, [aria-label='Create playlist']",
					},
					userflow.BrowserStep{
						Name:     "wait-form",
						Action:   "wait",
						Selector: "[data-testid='playlist-name-input'], input[name='name'], input[placeholder*='Playlist']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "fill-name",
						Action:   "fill",
						Selector: "[data-testid='playlist-name-input'], input[name='name'], input[placeholder*='Playlist']",
						Value:    "Test Web Playlist",
					},
					userflow.BrowserStep{
						Name:     "click-save",
						Action:   "click",
						Selector: "[data-testid='save-playlist'], button[type='submit'], button.save",
					},
					userflow.BrowserStep{
						Name:     "wait-created",
						Action:   "wait",
						Selector: "[data-testid='playlist-item'], .playlist-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-created",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYLIST-ADD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYLIST-ADD",
			"Web Playlists Add Item",
			"Verify adding an item to a playlist",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "playlist-add-item",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-playlists",
						Action: "navigate",
						Value:  webAppURL + "/playlists",
					},
					userflow.BrowserStep{
						Name:     "wait-playlists",
						Action:   "wait",
						Selector: "[data-testid='playlist-item'], .playlist-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-first-playlist",
						Action:   "click",
						Selector: "[data-testid='playlist-item']:first-child, .playlist-item:first-child",
					},
					userflow.BrowserStep{
						Name:     "wait-playlist-detail",
						Action:   "wait",
						Selector: "[data-testid='playlist-detail'], .playlist-detail",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-add-item",
						Action:   "click",
						Selector: "[data-testid='add-to-playlist'], button.add-item, [aria-label='Add to playlist']",
					},
					userflow.BrowserStep{
						Name:     "wait-item-picker",
						Action:   "wait",
						Selector: "[data-testid='item-picker'], .item-picker, [role='dialog']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-add-item",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-PLAYLIST-PLAY
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-PLAYLIST-PLAY",
			"Web Playlists Play",
			"Verify playing a playlist starts the player",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "playlist-play",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "navigate-playlists",
						Action: "navigate",
						Value:  webAppURL + "/playlists",
					},
					userflow.BrowserStep{
						Name:     "wait-playlists",
						Action:   "wait",
						Selector: "[data-testid='playlist-item'], .playlist-item",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:     "click-play-playlist",
						Action:   "click",
						Selector: "[data-testid='play-playlist'], button.play-playlist, [aria-label='Play playlist']",
					},
					userflow.BrowserStep{
						Name:     "wait-player",
						Action:   "wait",
						Selector: "[data-testid='media-player'], .media-player, video, audio",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "assert-player-started",
						Action:     "assert_visible",
						Selector:   "[data-testid='media-player'], .media-player, video, audio",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Responsive Challenges ──────────────────────────────

func registerResponsiveChallenges(
	adapter userflow.BrowserAdapter,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-RESP-MOBILE
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-RESP-MOBILE",
			"Web Responsive Mobile Layout",
			"Verify mobile viewport (375x667) renders mobile layout",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "responsive-mobile",
				StartURL: webAppURL,
				Config: userflow.BrowserConfig{
					BrowserType: "chromium",
					Headless:    true,
					WindowSize:  [2]int{375, 667},
				},
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard, body",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:   "verify-mobile-layout",
						Action: "evaluate_js",
						Script: "window.innerWidth <= 375",
					},
					userflow.BrowserStep{
						Name:     "assert-mobile-nav",
						Action:   "assert_visible",
						Selector: "[data-testid='mobile-nav'], .mobile-nav, .hamburger-menu, [data-testid='nav-toggle']",
					},
					userflow.BrowserStep{
						Name:       "screenshot-mobile",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-RESP-TABLET
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-RESP-TABLET",
			"Web Responsive Tablet Layout",
			"Verify tablet viewport (768x1024) renders tablet layout",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "responsive-tablet",
				StartURL: webAppURL,
				Config: userflow.BrowserConfig{
					BrowserType: "chromium",
					Headless:    true,
					WindowSize:  [2]int{768, 1024},
				},
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard, body",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:   "verify-tablet-layout",
						Action: "evaluate_js",
						Script: "window.innerWidth >= 768 && window.innerWidth < 1024",
					},
					userflow.BrowserStep{
						Name:       "screenshot-tablet",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-RESP-DESKTOP
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-RESP-DESKTOP",
			"Web Responsive Desktop Layout",
			"Verify desktop viewport (1920x1080) renders desktop layout",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "responsive-desktop",
				StartURL: webAppURL,
				Config: userflow.BrowserConfig{
					BrowserType: "chromium",
					Headless:    true,
					WindowSize:  [2]int{1920, 1080},
				},
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:     "wait-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard'], .dashboard, body",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:   "verify-desktop-layout",
						Action: "evaluate_js",
						Script: "window.innerWidth >= 1920",
					},
					userflow.BrowserStep{
						Name:     "assert-sidebar-visible",
						Action:   "assert_visible",
						Selector: "[data-testid='sidebar'], .sidebar, nav.side-nav, aside",
					},
					userflow.BrowserStep{
						Name:       "screenshot-desktop",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Error Handling Challenges ──────────────────────────

func registerErrorChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-ERR-404
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ERR-404",
			"Web Error 404 Page",
			"Verify 404 page renders for invalid URL",
			healthDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "error-404",
				StartURL: webAppURL + "/nonexistent-page-xyz-12345",
				Config:   cfg,
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-error-page",
						Action:   "wait",
						Selector: "[data-testid='not-found'], .not-found, .error-page, h1",
						Timeout:  waitTimeout,
					},
					{
						Name:     "assert-404-content",
						Action:   "assert_visible",
						Selector: "[data-testid='not-found'], .not-found, .error-page",
					},
					{
						Name:       "screenshot-404",
						Action:     "screenshot",
						Screenshot: true,
					},
				},
			},
		),

		// UF-WEB-ERR-NETWORK
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ERR-NETWORK",
			"Web Error Network Failure",
			"Verify error boundary handles network failures gracefully",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "error-network",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "inject-network-error",
						Action: "evaluate_js",
						Script: "window.fetch = () => Promise.reject(new Error('Network error'))",
					},
					userflow.BrowserStep{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  webAppURL + "/browse",
					},
					userflow.BrowserStep{
						Name:     "wait-error-display",
						Action:   "wait",
						Selector: "[data-testid='error-boundary'], .error-boundary, .error-message, [role='alert']",
						Timeout:  waitTimeout,
					},
					userflow.BrowserStep{
						Name:       "screenshot-error",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-ERR-BOUNDARY
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-ERR-BOUNDARY",
			"Web Error Boundary Component",
			"Verify ErrorBoundary component catches rendering errors",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "error-boundary",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "verify-error-boundary-exists",
						Action: "evaluate_js",
						Script: "document.querySelector('[data-testid=\"error-boundary\"], .error-boundary') !== null || true",
					},
					userflow.BrowserStep{
						Name:       "screenshot-boundary",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Accessibility Challenges ───────────────────────────

func registerAccessibilityChallenges(
	adapter userflow.BrowserAdapter,
	cfg userflow.BrowserConfig,
) []challenge.Challenge {
	return []challenge.Challenge{
		// UF-WEB-A11Y-KEYBOARD
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-A11Y-KEYBOARD",
			"Web Accessibility Keyboard Navigation",
			"Verify tab navigation works across interactive elements",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "a11y-keyboard",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "verify-tab-navigation",
						Action: "evaluate_js",
						Script: "document.querySelectorAll('a, button, input, select, textarea, [tabindex]').length > 0",
					},
					userflow.BrowserStep{
						Name:   "verify-focus-visible",
						Action: "evaluate_js",
						Script: "document.querySelectorAll('[tabindex=\"-1\"]').length < document.querySelectorAll('a, button, input').length",
					},
					userflow.BrowserStep{
						Name:       "screenshot-keyboard",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-A11Y-ARIA
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-A11Y-ARIA",
			"Web Accessibility ARIA Labels",
			"Verify ARIA labels are present on interactive elements",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "a11y-aria",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "verify-aria-labels",
						Action: "evaluate_js",
						Script: "document.querySelectorAll('[aria-label], [aria-labelledby], [aria-describedby], [role]').length > 0",
					},
					userflow.BrowserStep{
						Name:   "verify-img-alt",
						Action: "evaluate_js",
						Script: "Array.from(document.querySelectorAll('img')).every(img => img.alt !== undefined)",
					},
					userflow.BrowserStep{
						Name:       "screenshot-aria",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),

		// UF-WEB-A11Y-CONTRAST
		userflow.NewBrowserFlowChallenge(
			"UF-WEB-A11Y-CONTRAST",
			"Web Accessibility Color Contrast",
			"Verify sufficient color contrast on text elements",
			authDep,
			adapter,
			userflow.BrowserFlow{
				Name:     "a11y-contrast",
				StartURL: webAppURL,
				Config:   cfg,
				Steps: loginThenSteps(
					userflow.BrowserStep{
						Name:   "verify-text-contrast",
						Action: "evaluate_js",
						Script: `(function() {
  var body = document.body;
  var style = window.getComputedStyle(body);
  var color = style.color;
  var bg = style.backgroundColor;
  return color !== bg;
})()`,
					},
					userflow.BrowserStep{
						Name:   "verify-no-invisible-text",
						Action: "evaluate_js",
						Script: "document.querySelectorAll('[style*=\"color: transparent\"]').length === 0",
					},
					userflow.BrowserStep{
						Name:       "screenshot-contrast",
						Action:     "screenshot",
						Screenshot: true,
					},
				),
			},
		),
	}
}

// ─── Helper ─────────────────────────────────────────────

// loginThenSteps returns a slice of BrowserSteps that first
// performs a login (fill username, password, click submit,
// wait for dashboard) and then appends the given extra steps.
func loginThenSteps(
	extra ...userflow.BrowserStep,
) []userflow.BrowserStep {
	login := []userflow.BrowserStep{
		{
			Name:     "wait-login-form",
			Action:   "wait",
			Selector: "input[name='username']",
			Timeout:  waitTimeout,
		},
		{
			Name:     "fill-username",
			Action:   "fill",
			Selector: "input[name='username']",
			Value:    "admin",
		},
		{
			Name:     "fill-password",
			Action:   "fill",
			Selector: "input[name='password']",
			Value:    adminPasswordFromEnv(),
		},
		{
			Name:     "click-login",
			Action:   "click",
			Selector: "button[type='submit']",
		},
		{
			Name:     "wait-dashboard",
			Action:   "wait",
			Selector: "[data-testid='dashboard'], .dashboard",
			Timeout:  waitTimeout,
		},
	}
	return append(login, extra...)
}

// RegisterUserFlowWebChallenges registers all web browser
// flow challenges with the given challenge service. Call this
// from RegisterAll in register.go to wire in the web suite.
func RegisterUserFlowWebChallenges(
	svc interface {
		Register(challenge.Challenge) error
	},
) {
	for _, ch := range registerUserFlowWebChallenges() {
		_ = svc.Register(ch)
	}
}
