package challenges

import (
	"time"

	"digital.vasic.challenges/pkg/challenge"
	"digital.vasic.challenges/pkg/env"
	"digital.vasic.challenges/pkg/userflow"
)

// androidProjectRoot returns the root directory for the
// catalogizer-android project.
func androidProjectRoot() string {
	return env.GetOrDefault(
		"ANDROID_PROJECT_ROOT",
		"../catalogizer-android",
	)
}

// androidAPKPath returns the path to the debug APK built by
// Gradle.
func androidAPKPath() string {
	return env.GetOrDefault(
		"ANDROID_APK_PATH",
		"../catalogizer-android/app/build/outputs/apk/debug/app-debug.apk",
	)
}

// androidTVProjectRoot returns the root directory for the
// catalogizer-androidtv project.
func androidTVProjectRoot() string {
	return env.GetOrDefault(
		"ANDROIDTV_PROJECT_ROOT",
		"../catalogizer-androidtv",
	)
}

// androidTVAPKPath returns the path to the debug APK for the
// Android TV app.
func androidTVAPKPath() string {
	return env.GetOrDefault(
		"ANDROIDTV_APK_PATH",
		"../catalogizer-androidtv/app/build/outputs/apk/debug/app-debug.apk",
	)
}

// androidMobileConfig returns the MobileConfig for the
// catalogizer-android application.
func androidMobileConfig() userflow.MobileConfig {
	return userflow.MobileConfig{
		PackageName:  "com.vasic.catalogizer",
		ActivityName: ".MainActivity",
		DeviceSerial: env.GetOrDefault(
			"ANDROID_DEVICE_SERIAL", "",
		),
	}
}

// androidTVMobileConfig returns the MobileConfig for the
// catalogizer-androidtv application.
func androidTVMobileConfig() userflow.MobileConfig {
	return userflow.MobileConfig{
		PackageName:  "com.catalogizer.androidtv",
		ActivityName: ".ui.MainActivity",
		DeviceSerial: env.GetOrDefault(
			"ANDROIDTV_DEVICE_SERIAL", "",
		),
	}
}

// registerUserFlowMobileChallenges creates and returns all
// Android and Android TV user flow challenges (38 total).
func registerUserFlowMobileChallenges() []challenge.Challenge {
	androidGradle := userflow.NewGradleCLIAdapter(
		androidProjectRoot(), false,
	)
	androidADB := userflow.NewADBCLIAdapter(
		androidMobileConfig(),
	)
	tvGradle := userflow.NewGradleCLIAdapter(
		androidTVProjectRoot(), false,
	)
	tvADB := userflow.NewADBCLIAdapter(
		androidTVMobileConfig(),
	)

	var challenges []challenge.Challenge

	// -------------------------------------------------------
	// Android Build (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewBuildChallenge(
			"UF-ANDROID-BUILD",
			"Android Build",
			"Build catalogizer-android with Gradle assembleDebug",
			nil,
			androidGradle,
			[]userflow.BuildTarget{
				{
					Name: "debug",
					Task: "assembleDebug",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewUnitTestChallenge(
			"UF-ANDROID-TEST",
			"Android Unit Tests",
			"Run Android unit tests with Gradle",
			[]challenge.ID{"UF-ANDROID-BUILD"},
			androidGradle,
			[]userflow.TestTarget{
				{
					Name: "unit-tests",
					Task: "test",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewLintChallenge(
			"UF-ANDROID-LINT",
			"Android Lint",
			"Run Android lint on catalogizer-android",
			[]challenge.ID{"UF-ANDROID-BUILD"},
			androidGradle,
			[]userflow.LintTarget{
				{
					Name: "android-lint",
					Task: "lint",
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Launch (3 challenges)
	// -------------------------------------------------------

	androidBuildDeps := []challenge.ID{"UF-ANDROID-BUILD"}

	challenges = append(challenges,
		userflow.NewMobileLaunchChallenge(
			"UF-ANDROID-LAUNCH",
			"Android Launch",
			"Install and launch catalogizer-android on emulator",
			androidBuildDeps,
			androidADB,
			androidAPKPath(),
			5*time.Second,
		),
	)

	challenges = append(challenges,
		userflow.NewMobileLaunchChallenge(
			"UF-ANDROID-STABLE",
			"Android Stability",
			"Launch Android app, wait 10s, verify still running",
			androidBuildDeps,
			androidADB,
			androidAPKPath(),
			10*time.Second,
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-SCREENSHOT",
			"Android Screenshot",
			"Launch app and capture a screenshot",
			[]challenge.ID{"UF-ANDROID-LAUNCH"},
			androidADB,
			userflow.MobileFlow{
				Name:        "android-screenshot",
				Description: "Take a screenshot of the running app",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "take-screenshot",
						Action: "screenshot",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "screenshot_exists",
								Target:  "android_screenshot",
								Message: "Android screenshot captured",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Auth (3 challenges)
	// -------------------------------------------------------

	androidLaunchDeps := []challenge.ID{"UF-ANDROID-LAUNCH"}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-AUTH-LOGIN",
			"Android Auth Login",
			"Tap login, enter credentials, verify main screen",
			androidLaunchDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-auth-login",
				Description: "Login on the Android app",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-username-field",
						Action: "tap",
						X:      540,
						Y:      800,
					},
					{
						Name:   "type-username",
						Action: "send_keys",
						Value:  "admin",
					},
					{
						Name:   "tap-password-field",
						Action: "tap",
						X:      540,
						Y:      950,
					},
					{
						Name:   "type-password",
						Action: "send_keys",
						Value:  adminPasswordFromEnv(),
					},
					{
						Name:   "tap-login-button",
						Action: "tap",
						X:      540,
						Y:      1100,
					},
					{
						Name:   "wait-for-main",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "login_success",
								Message: "app is on main screen after login",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_launches",
								Target:  "post_login",
								Message: "app running after login",
							},
						},
					},
				},
			},
		),
	)

	androidAuthDeps := []challenge.ID{
		"UF-ANDROID-AUTH-LOGIN",
	}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-AUTH-INVALID",
			"Android Auth Invalid",
			"Enter wrong credentials, verify error message",
			androidLaunchDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-auth-invalid",
				Description: "Attempt login with bad credentials",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-username-field",
						Action: "tap",
						X:      540,
						Y:      800,
					},
					{
						Name:   "type-bad-username",
						Action: "send_keys",
						Value:  "wronguser",
					},
					{
						Name:   "tap-password-field",
						Action: "tap",
						X:      540,
						Y:      950,
					},
					{
						Name:   "type-bad-password",
						Action: "send_keys",
						Value:  "wrongpass",
					},
					{
						Name:   "tap-login-button",
						Action: "tap",
						X:      540,
						Y:      1100,
					},
					{
						Name:   "wait-for-error",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "error_displayed",
								Message: "app shows error for invalid credentials",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-AUTH-LOGOUT",
			"Android Auth Logout",
			"Login, tap logout, verify login screen returns",
			androidAuthDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-auth-logout",
				Description: "Logout from the Android app",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-username",
						Action: "tap",
						X:      540,
						Y:      800,
					},
					{
						Name:   "type-username",
						Action: "send_keys",
						Value:  "admin",
					},
					{
						Name:   "tap-password",
						Action: "tap",
						X:      540,
						Y:      950,
					},
					{
						Name:   "type-password",
						Action: "send_keys",
						Value:  adminPasswordFromEnv(),
					},
					{
						Name:   "tap-login",
						Action: "tap",
						X:      540,
						Y:      1100,
					},
					{
						Name:   "wait-main-screen",
						Action: "wait",
					},
					{
						Name:   "press-menu",
						Action: "press_key",
						Value:  "KEYCODE_MENU",
					},
					{
						Name:   "tap-logout",
						Action: "tap",
						X:      540,
						Y:      1400,
					},
					{
						Name:   "wait-login-screen",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "logout_success",
								Message: "returned to login screen after logout",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Browse (4 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-BROWSE-LOAD",
			"Android Browse Load",
			"Navigate to browse tab, assert items visible",
			androidAuthDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-browse-load",
				Description: "Open browse screen and verify items",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-browse-tab",
						Action: "tap",
						X:      270,
						Y:      1800,
					},
					{
						Name:   "wait-browse-loaded",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "browse_loaded",
								Message: "browse screen loaded with items",
							},
						},
					},
					{
						Name:   "verify-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	androidBrowseDeps := []challenge.ID{
		"UF-ANDROID-BROWSE-LOAD",
	}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-BROWSE-SEARCH",
			"Android Browse Search",
			"Tap search, enter query, assert results appear",
			androidBrowseDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-browse-search",
				Description: "Search for media on Android",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-search-icon",
						Action: "tap",
						X:      900,
						Y:      150,
					},
					{
						Name:   "type-search-query",
						Action: "send_keys",
						Value:  "movie",
					},
					{
						Name:   "press-enter",
						Action: "press_key",
						Value:  "KEYCODE_ENTER",
					},
					{
						Name:   "wait-search-results",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "search_results",
								Message: "search results displayed",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-BROWSE-DETAIL",
			"Android Browse Detail",
			"Tap a media item, assert detail view opens",
			androidBrowseDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-browse-detail",
				Description: "Tap item to open detail view",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-first-item",
						Action: "tap",
						X:      270,
						Y:      600,
					},
					{
						Name:   "wait-detail-view",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "detail_view",
								Message: "detail view opened for tapped item",
							},
						},
					},
					{
						Name:   "take-screenshot",
						Action: "screenshot",
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-BROWSE-SCROLL",
			"Android Browse Scroll",
			"Scroll the list, assert more items load",
			androidBrowseDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-browse-scroll",
				Description: "Scroll browse list to load more",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "swipe-up",
						Action: "press_key",
						Value:  "KEYCODE_PAGE_DOWN",
					},
					{
						Name:   "wait-after-scroll",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "scroll_loaded",
								Message: "more items loaded after scroll",
							},
						},
					},
					{
						Name:   "assert-still-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Playback (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-PLAY-START",
			"Android Playback Start",
			"Select media item, assert playback starts",
			androidAuthDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-play-start",
				Description: "Start media playback on Android",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-media-item",
						Action: "tap",
						X:      270,
						Y:      600,
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "tap-play-button",
						Action: "tap",
						X:      540,
						Y:      400,
					},
					{
						Name:   "wait-playback",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "playback_started",
								Message: "media playback started",
							},
						},
					},
				},
			},
		),
	)

	androidPlayDeps := []challenge.ID{
		"UF-ANDROID-PLAY-START",
	}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-PLAY-CONTROLS",
			"Android Playback Controls",
			"Assert play/pause controls are visible",
			androidPlayDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-play-controls",
				Description: "Verify playback controls visible",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-media-item",
						Action: "tap",
						X:      270,
						Y:      600,
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "tap-play",
						Action: "tap",
						X:      540,
						Y:      400,
					},
					{
						Name:   "wait-controls",
						Action: "wait",
					},
					{
						Name:   "tap-screen-for-controls",
						Action: "tap",
						X:      540,
						Y:      960,
					},
					{
						Name:   "screenshot-controls",
						Action: "screenshot",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "screenshot_exists",
								Target:  "controls_screenshot",
								Message: "playback controls visible in screenshot",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-PLAY-SEEK",
			"Android Playback Seek",
			"Seek forward during playback, assert position changes",
			androidPlayDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-play-seek",
				Description: "Seek forward in media playback",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-media-item",
						Action: "tap",
						X:      270,
						Y:      600,
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "tap-play",
						Action: "tap",
						X:      540,
						Y:      400,
					},
					{
						Name:   "wait-playback",
						Action: "wait",
					},
					{
						Name:   "seek-forward",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_FAST_FORWARD",
					},
					{
						Name:   "wait-after-seek",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "seek_completed",
								Message: "seek forward completed without crash",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Settings (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-SETTINGS-LOAD",
			"Android Settings Load",
			"Navigate to settings screen",
			androidAuthDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-settings-load",
				Description: "Open settings on Android",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-settings-tab",
						Action: "tap",
						X:      810,
						Y:      1800,
					},
					{
						Name:   "wait-settings",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "settings_loaded",
								Message: "settings screen loaded",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-SETTINGS-SERVER",
			"Android Settings Server URL",
			"Change server URL in settings, save",
			[]challenge.ID{"UF-ANDROID-SETTINGS-LOAD"},
			androidADB,
			userflow.MobileFlow{
				Name:        "android-settings-server",
				Description: "Change server URL on Android",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-settings-tab",
						Action: "tap",
						X:      810,
						Y:      1800,
					},
					{
						Name:   "wait-settings",
						Action: "wait",
					},
					{
						Name:   "tap-server-url-field",
						Action: "tap",
						X:      540,
						Y:      400,
					},
					{
						Name:   "type-server-url",
						Action: "send_keys",
						Value:  "http://192.168.0.100:8080",
					},
					{
						Name:   "tap-save",
						Action: "tap",
						X:      540,
						Y:      1600,
					},
					{
						Name:   "wait-after-save",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "settings_saved",
								Message: "server URL setting saved",
							},
						},
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Offline (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-OFFLINE-BANNER",
			"Android Offline Banner",
			"Disable network, assert offline banner appears",
			androidAuthDeps,
			androidADB,
			userflow.MobileFlow{
				Name:        "android-offline-banner",
				Description: "Verify offline banner when disconnected",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "assert-running-before",
						Action: "assert_running",
					},
					{
						Name:   "enable-airplane-mode",
						Action: "press_key",
						Value:  "KEYCODE_SETTINGS",
					},
					{
						Name:   "wait-for-offline",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "offline_banner",
								Message: "offline banner displayed",
							},
						},
					},
					{
						Name:   "take-screenshot",
						Action: "screenshot",
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROID-OFFLINE-CACHE",
			"Android Offline Cache",
			"Assert cached content is accessible while offline",
			[]challenge.ID{"UF-ANDROID-OFFLINE-BANNER"},
			androidADB,
			userflow.MobileFlow{
				Name:        "android-offline-cache",
				Description: "Verify cached content accessible offline",
				Config:      androidMobileConfig(),
				AppPath:     androidAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "tap-browse-tab",
						Action: "tap",
						X:      270,
						Y:      1800,
					},
					{
						Name:   "wait-cached-content",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "cached_content",
								Message: "cached content accessible offline",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android Instrumented (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewInstrumentedTestChallenge(
			"UF-ANDROID-INSTR-UI",
			"Android Instrumented UI Tests",
			"Run instrumented UI tests on Android device",
			androidBuildDeps,
			androidADB,
			[]string{
				"com.vasic.catalogizer.ui.MainActivityTest",
				"com.vasic.catalogizer.ui.BrowseScreenTest",
			},
		),
	)

	challenges = append(challenges,
		userflow.NewInstrumentedTestChallenge(
			"UF-ANDROID-INSTR-NAV",
			"Android Instrumented Navigation Tests",
			"Run navigation instrumented tests on device",
			androidBuildDeps,
			androidADB,
			[]string{
				"com.vasic.catalogizer.navigation.NavigationTest",
				"com.vasic.catalogizer.navigation.DeepLinkTest",
			},
		),
	)

	// -------------------------------------------------------
	// Android TV Build (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewBuildChallenge(
			"UF-ANDROIDTV-BUILD",
			"Android TV Build",
			"Build catalogizer-androidtv with Gradle assembleDebug",
			nil,
			tvGradle,
			[]userflow.BuildTarget{
				{
					Name: "debug",
					Task: "assembleDebug",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewUnitTestChallenge(
			"UF-ANDROIDTV-TEST",
			"Android TV Unit Tests",
			"Run Android TV unit tests with Gradle",
			[]challenge.ID{"UF-ANDROIDTV-BUILD"},
			tvGradle,
			[]userflow.TestTarget{
				{
					Name: "unit-tests",
					Task: "test",
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewLintChallenge(
			"UF-ANDROIDTV-LINT",
			"Android TV Lint",
			"Run Android lint on catalogizer-androidtv",
			[]challenge.ID{"UF-ANDROIDTV-BUILD"},
			tvGradle,
			[]userflow.LintTarget{
				{
					Name: "android-lint",
					Task: "lint",
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android TV Launch (2 challenges)
	// -------------------------------------------------------

	tvBuildDeps := []challenge.ID{"UF-ANDROIDTV-BUILD"}

	challenges = append(challenges,
		userflow.NewMobileLaunchChallenge(
			"UF-ANDROIDTV-LAUNCH",
			"Android TV Launch",
			"Install and launch catalogizer-androidtv on TV emulator",
			tvBuildDeps,
			tvADB,
			androidTVAPKPath(),
			5*time.Second,
		),
	)

	challenges = append(challenges,
		userflow.NewMobileLaunchChallenge(
			"UF-ANDROIDTV-STABLE",
			"Android TV Stability",
			"Verify Android TV app stability for 10s",
			tvBuildDeps,
			tvADB,
			androidTVAPKPath(),
			10*time.Second,
		),
	)

	// -------------------------------------------------------
	// Android TV Nav (3 challenges)
	// -------------------------------------------------------

	tvLaunchDeps := []challenge.ID{"UF-ANDROIDTV-LAUNCH"}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-NAV-DPAD",
			"Android TV D-pad Navigation",
			"Navigate with D-pad up/down/left/right",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-nav-dpad",
				Description: "D-pad navigation on Android TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "press-down",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "press-right",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_RIGHT",
					},
					{
						Name:   "press-up",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_UP",
					},
					{
						Name:   "press-left",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_LEFT",
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "dpad_navigation",
								Message: "app stable after D-pad navigation",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-NAV-SELECT",
			"Android TV D-pad Select",
			"Navigate to item with D-pad, select with Enter",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-nav-select",
				Description: "Select item with Enter key on TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-to-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-after-select",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "item_selected",
								Message: "item selected with Enter key",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-NAV-BACK",
			"Android TV Back Navigation",
			"Press Back, assert navigation back works",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-nav-back",
				Description: "Navigate back on Android TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-forward",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "press-back",
						Action: "press_key",
						Value:  "KEYCODE_BACK",
					},
					{
						Name:   "wait-after-back",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "back_navigation",
								Message: "navigated back successfully",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android TV Browse (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-BROWSE-LOAD",
			"Android TV Browse Load",
			"Assert browse screen loads on TV",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-browse-load",
				Description: "Verify TV browse screen loads",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_launches",
								Target:  "tv_browse_loaded",
								Message: "TV browse screen loaded",
							},
						},
					},
					{
						Name:   "take-screenshot",
						Action: "screenshot",
					},
				},
			},
		),
	)

	tvBrowseDeps := []challenge.ID{
		"UF-ANDROIDTV-BROWSE-LOAD",
	}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-BROWSE-ROW",
			"Android TV Browse Row Scroll",
			"Navigate row with D-pad, assert items scroll",
			tvBrowseDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-browse-row",
				Description: "Scroll through rows on TV browse",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "scroll-right",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_RIGHT",
					},
					{
						Name:   "scroll-right-again",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_RIGHT",
					},
					{
						Name:   "scroll-right-more",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_RIGHT",
					},
					{
						Name:   "wait-after-scroll",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "row_scrolled",
								Message: "TV row scrolled successfully",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-BROWSE-DETAIL",
			"Android TV Browse Detail",
			"Select item on TV, assert detail screen opens",
			tvBrowseDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-browse-detail",
				Description: "Open detail view on TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-to-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-detail-screen",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "tv_detail_screen",
								Message: "TV detail screen opened",
							},
						},
					},
					{
						Name:   "take-screenshot",
						Action: "screenshot",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android TV Playback (3 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-PLAY-START",
			"Android TV Playback Start",
			"Select media item on TV, assert playback starts",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-play-start",
				Description: "Start playback on Android TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-to-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "press-play",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_PLAY",
					},
					{
						Name:   "wait-playback",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "tv_playback_started",
								Message: "TV media playback started",
							},
						},
					},
				},
			},
		),
	)

	tvPlayDeps := []challenge.ID{
		"UF-ANDROIDTV-PLAY-START",
	}

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-PLAY-CONTROLS",
			"Android TV Playback Controls",
			"Assert media controls overlay on TV",
			tvPlayDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-play-controls",
				Description: "Verify playback controls on TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-to-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "press-play",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_PLAY",
					},
					{
						Name:   "wait-playback",
						Action: "wait",
					},
					{
						Name:   "press-center-for-controls",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "screenshot-controls",
						Action: "screenshot",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "screenshot_exists",
								Target:  "tv_controls_screenshot",
								Message: "TV playback controls visible",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-PLAY-DPAD",
			"Android TV Playback D-pad",
			"Control TV playback via D-pad keys",
			tvPlayDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-play-dpad",
				Description: "Control playback with D-pad on TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "navigate-to-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-item",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-detail",
						Action: "wait",
					},
					{
						Name:   "press-play",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_PLAY",
					},
					{
						Name:   "wait-playback",
						Action: "wait",
					},
					{
						Name:   "fast-forward",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_FAST_FORWARD",
					},
					{
						Name:   "pause-playback",
						Action: "press_key",
						Value:  "KEYCODE_MEDIA_PAUSE",
					},
					{
						Name:   "wait-after-controls",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "tv_dpad_control",
								Message: "TV playback controlled via D-pad",
							},
						},
					},
					{
						Name:   "assert-running",
						Action: "assert_running",
					},
				},
			},
		),
	)

	// -------------------------------------------------------
	// Android TV Settings (2 challenges)
	// -------------------------------------------------------

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-SETTINGS-LOAD",
			"Android TV Settings Load",
			"Navigate to TV settings screen",
			tvLaunchDeps,
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-settings-load",
				Description: "Open settings on Android TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "press-menu",
						Action: "press_key",
						Value:  "KEYCODE_MENU",
					},
					{
						Name:   "wait-settings",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "tv_settings_loaded",
								Message: "TV settings screen loaded",
							},
						},
					},
				},
			},
		),
	)

	challenges = append(challenges,
		userflow.NewMobileFlowChallenge(
			"UF-ANDROIDTV-SETTINGS-SERVER",
			"Android TV Settings Server URL",
			"Configure server URL on Android TV",
			[]challenge.ID{"UF-ANDROIDTV-SETTINGS-LOAD"},
			tvADB,
			userflow.MobileFlow{
				Name:        "tv-settings-server",
				Description: "Change server URL on Android TV",
				Config:      androidTVMobileConfig(),
				AppPath:     androidTVAPKPath(),
				Steps: []userflow.MobileStep{
					{
						Name:   "wait-for-app",
						Action: "wait",
					},
					{
						Name:   "press-menu",
						Action: "press_key",
						Value:  "KEYCODE_MENU",
					},
					{
						Name:   "wait-settings",
						Action: "wait",
					},
					{
						Name:   "navigate-to-server",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_DOWN",
					},
					{
						Name:   "select-server-setting",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "type-server-url",
						Action: "send_keys",
						Value:  "http://192.168.0.100:8080",
					},
					{
						Name:   "confirm-save",
						Action: "press_key",
						Value:  "KEYCODE_DPAD_CENTER",
					},
					{
						Name:   "wait-after-save",
						Action: "wait",
						Assertions: []userflow.StepAssertion{
							{
								Type:    "app_stable",
								Target:  "tv_settings_saved",
								Message: "TV server URL setting saved",
							},
						},
					},
				},
			},
		),
	)

	return challenges
}

// RegisterUserFlowMobileChallenges registers all Android and
// Android TV user flow challenges with the given service.
func RegisterUserFlowMobileChallenges(
	svc interface {
		Register(challenge.Challenge) error
	},
) {
	for _, ch := range registerUserFlowMobileChallenges() {
		_ = svc.Register(ch)
	}
}
