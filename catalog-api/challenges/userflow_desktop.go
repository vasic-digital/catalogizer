package challenges

import (
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/env"
	"digital.vasic.challenges/pkg/userflow"
)

// desktopProjectRoot returns the root directory for the
// catalogizer-desktop Tauri application.
func desktopProjectRoot() string {
	return env.GetOrDefault(
		"DESKTOP_PROJECT_ROOT",
		"../catalogizer-desktop",
	)
}

// desktopBinaryPath returns the path to the built desktop
// binary.
func desktopBinaryPath() string {
	return env.GetOrDefault(
		"DESKTOP_BINARY_PATH",
		"../catalogizer-desktop/src-tauri/target/debug/catalogizer-desktop",
	)
}

// wizardProjectRoot returns the root directory for the
// installer-wizard Tauri application.
func wizardProjectRoot() string {
	return env.GetOrDefault(
		"WIZARD_PROJECT_ROOT",
		"../installer-wizard",
	)
}

// wizardBinaryPath returns the path to the built wizard
// binary.
func wizardBinaryPath() string {
	return env.GetOrDefault(
		"WIZARD_BINARY_PATH",
		"../installer-wizard/src-tauri/target/debug/installer-wizard",
	)
}

// desktopCargoAdapter returns a CargoCLIAdapter rooted at
// the desktop project's src-tauri directory.
func desktopCargoAdapter() *userflow.CargoCLIAdapter {
	return userflow.NewCargoCLIAdapter(
		desktopProjectRoot() + "/src-tauri",
	)
}

// wizardCargoAdapter returns a CargoCLIAdapter rooted at
// the wizard project's src-tauri directory.
func wizardCargoAdapter() *userflow.CargoCLIAdapter {
	return userflow.NewCargoCLIAdapter(
		wizardProjectRoot() + "/src-tauri",
	)
}

// desktopTauriAdapter returns a TauriCLIAdapter configured
// for the desktop application binary.
func desktopTauriAdapter() *userflow.TauriCLIAdapter {
	return userflow.NewTauriCLIAdapter(
		desktopBinaryPath(),
	)
}

// wizardTauriAdapter returns a TauriCLIAdapter configured
// for the installer wizard binary.
func wizardTauriAdapter() *userflow.TauriCLIAdapter {
	return userflow.NewTauriCLIAdapter(
		wizardBinaryPath(),
	)
}

// registerUserFlowDesktopChallenges creates and returns all
// desktop and wizard user flow challenges (28 total).
func registerUserFlowDesktopChallenges() []challenge.Challenge {
	cargo := desktopCargoAdapter()
	tauriDesktop := desktopTauriAdapter()
	wizardCargo := wizardCargoAdapter()
	tauriWizard := wizardTauriAdapter()

	var challenges []challenge.Challenge

	// -------------------------------------------------------
	// Desktop Build (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewBuildChallenge(
			"UF-DESKTOP-BUILD",
			"Desktop Build",
			"Build catalogizer-desktop with Cargo",
			nil,
			cargo,
			[]userflow.BuildTarget{
				{
					Name: "catalogizer-desktop",
					Task: "build",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewUnitTestChallenge(
			"UF-DESKTOP-TEST",
			"Desktop Unit Tests",
			"Run Rust unit tests for catalogizer-desktop",
			[]challenge.ID{"UF-DESKTOP-BUILD"},
			cargo,
			[]userflow.TestTarget{
				{
					Name: "all-tests",
					Task: "test",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewLintChallenge(
			"UF-DESKTOP-LINT",
			"Desktop Lint",
			"Run cargo clippy on catalogizer-desktop",
			[]challenge.ID{"UF-DESKTOP-BUILD"},
			cargo,
			[]userflow.LintTarget{
				{
					Name: "clippy",
					Task: "clippy",
				},
			},
		),
	)

	// -------------------------------------------------------
	// Desktop Launch (3 challenges)
	// -------------------------------------------------------

	desktopBuildDeps := []challenge.ID{"UF-DESKTOP-BUILD"}

	challenges = append(challenges,
		userflow.NewDesktopLaunchChallenge(
			"UF-DESKTOP-LAUNCH",
			"Desktop Launch",
			"Launch catalogizer-desktop and verify window appears",
			desktopBuildDeps,
			tauriDesktop,
			userflow.DesktopAppConfig{
				BinaryPath: desktopBinaryPath(),
			},
			5*time.Second,
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopLaunchChallenge(
			"UF-DESKTOP-STABLE",
			"Desktop Stability",
			"Launch catalogizer-desktop, wait 10s, verify still running",
			desktopBuildDeps,
			tauriDesktop,
			userflow.DesktopAppConfig{
				BinaryPath: desktopBinaryPath(),
			},
			10*time.Second,
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-SCREENSHOT",
			"Desktop Screenshot",
			"Launch desktop app, take screenshot, verify non-empty",
			[]challenge.ID{"UF-DESKTOP-LAUNCH"},
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-screenshot",
				Description: "Take a screenshot of the desktop app",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:       "wait-for-content",
						Action:     "wait",
						Selector:   "body",
						Timeout:    5 * time.Second,
						Screenshot: true,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "screenshot_exists",
								Target:  "desktop_screenshot",
								Message: "desktop screenshot captured",
							},
						},
					},
					{
						Name:   "capture-screenshot",
						Action: "screenshot",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "screenshot_exists",
								Target:  "screenshot_data",
								Message: "screenshot data is non-empty",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Desktop Auth (3 challenges)
	// -------------------------------------------------------

	desktopLaunchDeps := []challenge.ID{"UF-DESKTOP-LAUNCH"}

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-AUTH-LOGIN",
			"Desktop Auth Login",
			"Navigate to login, fill form, assert dashboard",
			desktopLaunchDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-auth-login",
				Description: "Login to the desktop application",
				StartURL:    "tauri://localhost/login",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-login-form",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  5 * time.Second,
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
						Name:     "assert-dashboard",
						Action:   "wait",
						Selector: "[data-testid='dashboard']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "login_flow",
								Message: "login navigated to dashboard",
							},
						},
					},
				},
			},
		),
	)

	desktopAuthDeps := []challenge.ID{
		"UF-DESKTOP-AUTH-LOGIN",
	}

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-AUTH-PERSIST",
			"Desktop Auth Persist",
			"Login, navigate away, assert still authenticated",
			desktopAuthDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-auth-persist",
				Description: "Verify auth persists across navigation",
				StartURL:    "tauri://localhost/login",
				Steps: []userflow.BrowserStep{
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
						Selector: "[data-testid='dashboard']",
						Timeout:  10 * time.Second,
					},
					{
						Name:   "navigate-browse",
						Action: "navigate",
						Value:  "tauri://localhost/browse",
					},
					{
						Name:     "assert-still-authed",
						Action:   "assert_visible",
						Selector: "[data-testid='browse-grid']",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "auth_persist",
								Message: "user remains authenticated after navigation",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-AUTH-LOGOUT",
			"Desktop Auth Logout",
			"Login, logout, assert login screen visible",
			desktopAuthDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-auth-logout",
				Description: "Login then logout and verify redirect",
				StartURL:    "tauri://localhost/login",
				Steps: []userflow.BrowserStep{
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
						Selector: "[data-testid='dashboard']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "click-logout",
						Action:   "click",
						Selector: "[data-testid='logout-button']",
					},
					{
						Name:     "assert-login-screen",
						Action:   "wait",
						Selector: "input[name='username']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "logout_flow",
								Message: "logout returned to login screen",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Desktop Browse (4 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-BROWSE-LOAD",
			"Desktop Browse Load",
			"Navigate to browse page, assert media grid loads",
			desktopAuthDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-browse-load",
				Description: "Load the browse page and verify grid",
				StartURL:    "tauri://localhost/browse",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-grid",
						Action:   "wait",
						Selector: "[data-testid='browse-grid']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "browse_grid",
								Message: "browse grid loaded successfully",
							},
						},
					},
				},
			},
		),
	)

	browseDeps := []challenge.ID{
		"UF-DESKTOP-BROWSE-LOAD",
	}

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-BROWSE-SEARCH",
			"Desktop Browse Search",
			"Search for media items, assert results appear",
			browseDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-browse-search",
				Description: "Search media in browse view",
				StartURL:    "tauri://localhost/browse",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-search-input",
						Action:   "wait",
						Selector: "[data-testid='search-input']",
						Timeout:  5 * time.Second,
					},
					{
						Name:     "enter-search",
						Action:   "fill",
						Selector: "[data-testid='search-input']",
						Value:    "movie",
					},
					{
						Name:     "assert-results",
						Action:   "wait",
						Selector: "[data-testid='search-results']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "search_results",
								Message: "search returned visible results",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-BROWSE-DETAIL",
			"Desktop Browse Detail",
			"Click a media item, assert detail view opens",
			browseDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-browse-detail",
				Description: "Click media item to open detail view",
				StartURL:    "tauri://localhost/browse",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-grid",
						Action:   "wait",
						Selector: "[data-testid='browse-grid']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "click-first-item",
						Action:   "click",
						Selector: "[data-testid='media-item']:first-child",
					},
					{
						Name:     "assert-detail-view",
						Action:   "wait",
						Selector: "[data-testid='entity-detail']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "detail_view",
								Message: "detail view opened for selected item",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-BROWSE-FILTER",
			"Desktop Browse Filter",
			"Apply a media type filter, assert filtered results",
			browseDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-browse-filter",
				Description: "Apply filter to browse results",
				StartURL:    "tauri://localhost/browse",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-grid",
						Action:   "wait",
						Selector: "[data-testid='browse-grid']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "click-filter",
						Action:   "click",
						Selector: "[data-testid='filter-button']",
					},
					{
						Name:     "select-movie-type",
						Action:   "click",
						Selector: "[data-testid='filter-type-movie']",
					},
					{
						Name:     "assert-filtered",
						Action:   "wait",
						Selector: "[data-testid='browse-grid']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "filtered_results",
								Message: "browse grid shows filtered results",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Desktop IPC (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewDesktopIPCChallenge(
			"UF-DESKTOP-IPC-VERSION",
			"Desktop IPC Version",
			"Invoke get_version IPC command, verify response",
			desktopLaunchDeps,
			tauriDesktop,
			[]userflow.IPCCommand{
				{
					Name:    "get_version",
					Command: "get_version",
					Assertions: []userflow.StepAssertion{
						{
							Type:    "not_empty",
							Target:  "version_response",
							Message: "version response is non-empty",
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopIPCChallenge(
			"UF-DESKTOP-IPC-CONFIG",
			"Desktop IPC Config",
			"Invoke get_config IPC command, verify config returned",
			desktopLaunchDeps,
			tauriDesktop,
			[]userflow.IPCCommand{
				{
					Name:    "get_config",
					Command: "get_config",
					Assertions: []userflow.StepAssertion{
						{
							Type:    "not_empty",
							Target:  "config_response",
							Message: "config response is non-empty",
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopIPCChallenge(
			"UF-DESKTOP-IPC-SETTINGS",
			"Desktop IPC Settings",
			"Invoke get_settings IPC command, verify settings",
			desktopLaunchDeps,
			tauriDesktop,
			[]userflow.IPCCommand{
				{
					Name:    "get_settings",
					Command: "get_settings",
					Assertions: []userflow.StepAssertion{
						{
							Type:    "not_empty",
							Target:  "settings_response",
							Message: "settings response is non-empty",
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Desktop Settings (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-SETTINGS-LOAD",
			"Desktop Settings Load",
			"Navigate to settings page and verify it loads",
			desktopAuthDeps,
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-settings-load",
				Description: "Load the settings page",
				StartURL:    "tauri://localhost/settings",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-settings-page",
						Action:   "wait",
						Selector: "[data-testid='settings-page']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "settings_page",
								Message: "settings page loaded",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-DESKTOP-SETTINGS-SAVE",
			"Desktop Settings Save",
			"Change a setting, save, verify persisted",
			[]challenge.ID{"UF-DESKTOP-SETTINGS-LOAD"},
			tauriDesktop,
			userflow.BrowserFlow{
				Name:        "desktop-settings-save",
				Description: "Modify and save a setting",
				StartURL:    "tauri://localhost/settings",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-settings",
						Action:   "wait",
						Selector: "[data-testid='settings-page']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "change-theme-setting",
						Action:   "click",
						Selector: "[data-testid='theme-toggle']",
					},
					{
						Name:     "click-save",
						Action:   "click",
						Selector: "[data-testid='settings-save']",
					},
					{
						Name:     "assert-saved",
						Action:   "wait",
						Selector: "[data-testid='save-confirmation']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "settings_save",
								Message: "setting saved and confirmed",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Wizard Build (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewBuildChallenge(
			"UF-WIZARD-BUILD",
			"Wizard Build",
			"Build installer-wizard with Cargo",
			nil,
			wizardCargo,
			[]userflow.BuildTarget{
				{
					Name: "installer-wizard",
					Task: "build",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewUnitTestChallenge(
			"UF-WIZARD-TEST",
			"Wizard Unit Tests",
			"Run Rust unit tests for installer-wizard",
			[]challenge.ID{"UF-WIZARD-BUILD"},
			wizardCargo,
			[]userflow.TestTarget{
				{
					Name: "all-tests",
					Task: "test",
				},
			},
		),
	)

	// -------------------------------------------------------
	// Wizard Flow (5 challenges)
	// -------------------------------------------------------

	wizardBuildDeps := []challenge.ID{"UF-WIZARD-BUILD"}

	challenges = append(challenges,
		userflow.NewDesktopLaunchChallenge(
			"UF-WIZARD-LAUNCH",
			"Wizard Launch",
			"Launch installer wizard and verify window appears",
			wizardBuildDeps,
			tauriWizard,
			userflow.DesktopAppConfig{
				BinaryPath: wizardBinaryPath(),
			},
			5*time.Second,
		),
	)

	wizardLaunchDeps := []challenge.ID{
		"UF-WIZARD-LAUNCH",
	}

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-WELCOME",
			"Wizard Welcome Screen",
			"Assert the welcome screen is visible after launch",
			wizardLaunchDeps,
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-welcome",
				Description: "Verify welcome screen appears",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "assert-welcome",
						Action:   "wait",
						Selector: "[data-testid='wizard-welcome']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "welcome_screen",
								Message: "wizard welcome screen is visible",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-PROTOCOL",
			"Wizard Protocol Selection",
			"Select a storage protocol and assert next step",
			[]challenge.ID{"UF-WIZARD-WELCOME"},
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-protocol",
				Description: "Select SMB protocol in wizard",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-protocol-step",
						Action:   "wait",
						Selector: "[data-testid='wizard-welcome']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "click-next",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "wait-protocol-page",
						Action:   "wait",
						Selector: "[data-testid='protocol-select']",
						Timeout:  5 * time.Second,
					},
					{
						Name:     "select-smb",
						Action:   "click",
						Selector: "[data-testid='protocol-smb']",
					},
					{
						Name:     "assert-next-enabled",
						Action:   "assert_visible",
						Selector: "[data-testid='wizard-next']:not([disabled])",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "protocol_selection",
								Message: "protocol selected, next step available",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-SERVER",
			"Wizard Server Details",
			"Fill server details, validate, assert next step",
			[]challenge.ID{"UF-WIZARD-PROTOCOL"},
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-server",
				Description: "Enter server connection details",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-server-form",
						Action:   "wait",
						Selector: "[data-testid='server-form']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "fill-server-host",
						Action:   "fill",
						Selector: "[data-testid='server-host']",
						Value:    "192.168.0.241",
					},
					{
						Name:     "fill-server-share",
						Action:   "fill",
						Selector: "[data-testid='server-share']",
						Value:    "media",
					},
					{
						Name:     "fill-server-user",
						Action:   "fill",
						Selector: "[data-testid='server-username']",
						Value:    "guest",
					},
					{
						Name:     "click-next",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "assert-next-step",
						Action:   "wait",
						Selector: "[data-testid='wizard-step-confirm']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "server_details",
								Message: "server details accepted, moved to confirmation",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-COMPLETE",
			"Wizard Complete",
			"Complete the wizard, assert success screen visible",
			[]challenge.ID{"UF-WIZARD-SERVER"},
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-complete",
				Description: "Complete the setup wizard",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-confirm-step",
						Action:   "wait",
						Selector: "[data-testid='wizard-step-confirm']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "click-finish",
						Action:   "click",
						Selector: "[data-testid='wizard-finish']",
					},
					{
						Name:     "assert-success",
						Action:   "wait",
						Selector: "[data-testid='wizard-success']",
						Timeout:  10 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "wizard_complete",
								Message: "wizard completed with success screen",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Wizard Validation (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-VALIDATE-EMPTY",
			"Wizard Validate Empty Form",
			"Submit empty form, assert validation errors shown",
			wizardLaunchDeps,
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-validate-empty",
				Description: "Submit wizard with empty fields",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-welcome",
						Action:   "wait",
						Selector: "[data-testid='wizard-welcome']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "skip-to-server",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "select-protocol",
						Action:   "click",
						Selector: "[data-testid='protocol-smb']",
					},
					{
						Name:     "click-next-without-data",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "assert-errors",
						Action:   "wait",
						Selector: "[data-testid='validation-error']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "empty_form_errors",
								Message: "validation errors displayed for empty form",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-VALIDATE-IP",
			"Wizard Validate Invalid IP",
			"Enter invalid IP address, assert validation error",
			wizardLaunchDeps,
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-validate-ip",
				Description: "Enter invalid IP in server form",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-welcome",
						Action:   "wait",
						Selector: "[data-testid='wizard-welcome']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "next-to-protocol",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "select-smb",
						Action:   "click",
						Selector: "[data-testid='protocol-smb']",
					},
					{
						Name:     "next-to-server",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "wait-server-form",
						Action:   "wait",
						Selector: "[data-testid='server-form']",
						Timeout:  5 * time.Second,
					},
					{
						Name:     "enter-invalid-ip",
						Action:   "fill",
						Selector: "[data-testid='server-host']",
						Value:    "999.999.999.999",
					},
					{
						Name:     "click-next",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "assert-ip-error",
						Action:   "wait",
						Selector: "[data-testid='validation-error']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "invalid_ip_error",
								Message: "validation error shown for invalid IP",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewDesktopFlowChallenge(
			"UF-WIZARD-VALIDATE-PATH",
			"Wizard Validate Invalid Path",
			"Enter invalid share path, assert validation error",
			wizardLaunchDeps,
			tauriWizard,
			userflow.BrowserFlow{
				Name:        "wizard-validate-path",
				Description: "Enter invalid path in server form",
				StartURL:    "tauri://localhost",
				Steps: []userflow.BrowserStep{
					{
						Name:     "wait-welcome",
						Action:   "wait",
						Selector: "[data-testid='wizard-welcome']",
						Timeout:  10 * time.Second,
					},
					{
						Name:     "next-to-protocol",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "select-smb",
						Action:   "click",
						Selector: "[data-testid='protocol-smb']",
					},
					{
						Name:     "next-to-server",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "wait-server-form",
						Action:   "wait",
						Selector: "[data-testid='server-form']",
						Timeout:  5 * time.Second,
					},
					{
						Name:     "enter-valid-host",
						Action:   "fill",
						Selector: "[data-testid='server-host']",
						Value:    "192.168.0.1",
					},
					{
						Name:     "enter-invalid-path",
						Action:   "fill",
						Selector: "[data-testid='server-share']",
						Value:    "../../../etc/passwd",
					},
					{
						Name:     "click-next",
						Action:   "click",
						Selector: "[data-testid='wizard-next']",
					},
					{
						Name:     "assert-path-error",
						Action:   "wait",
						Selector: "[data-testid='validation-error']",
						Timeout:  5 * time.Second,
						Assertions: []userflow.StepAssertion{
							{
								Type:    "flow_completes",
								Target:  "invalid_path_error",
								Message: "validation error shown for invalid path",
							},
						},
					},
				},
			},
		),
	)

	return challenges
}

// RegisterUserFlowDesktopChallenges registers all desktop and
// wizard user flow challenges with the given challenge service.
func RegisterUserFlowDesktopChallenges(
	svc interface {
		Register(challenge.Challenge) error
	},
) {
	for _, ch := range registerUserFlowDesktopChallenges() {
		_ = svc.Register(ch)
	}
}
