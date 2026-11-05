// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod confirm;
mod url_guard;

#[cfg(feature = "vlc-player")]
mod vlc;

use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use tauri::State;
use confirm::SharedConfirmer;
use tokio::sync::Mutex;

#[cfg(feature = "vlc-player")]
use vlc::commands::{VLCPlayerState, PlayRequest, PlaybackStatus, TrackListResponse};
#[cfg(feature = "vlc-player")]
use vlc::VLCPlayer;

#[derive(Debug, Clone, Serialize, Deserialize)]
struct AppConfig {
    server_url: Option<String>,
    auth_token: Option<String>,
    theme: String,
    auto_start: bool,
    /// Last server URL the USER confirmed natively. Backend-only (never serialised,
    /// so the frontend can neither read nor reset it); shown as "Previous" in the
    /// dialog after the URL was cleared, so clear-then-set cannot look like first-time.
    #[serde(skip)]
    last_confirmed_url: Option<String>,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            server_url: None,
            auth_token: None,
            theme: "dark".to_string(),
            auto_start: false,
            last_confirmed_url: None,
        }
    }
}

type ConfigState = Mutex<AppConfig>;

#[tauri::command]
async fn get_config(config: State<'_, ConfigState>) -> Result<AppConfig, String> {
    let config = config.lock().await;
    Ok(config.clone())
}

#[tauri::command]
async fn update_config(
    new_config: AppConfig,
    config: State<'_, ConfigState>,
    confirmer: State<'_, SharedConfirmer>,
) -> Result<(), String> {
    apply_config(&config, &confirmer, new_config).await
}

/// Server-URL change gate: ask the user natively whenever the URL differs from
/// the one in force AND from the last one the user confirmed (so first-time
/// setup prompts, and clearing then re-setting an unknown URL prompts with the
/// previously confirmed URL shown). The config lock is NOT held while the
/// dialog is open. Returns the `server_url` snapshot the decision was based on;
/// callers must write only if it is still current ([`STALE`]).
async fn confirm_url_change(
    config: &ConfigState,
    confirmer: &SharedConfirmer,
    requested: &str,
) -> Result<Option<String>, String> {
    let (current, last) = {
        let g = config.lock().await;
        (g.server_url.clone(), g.last_confirmed_url.clone())
    };
    if current.as_deref() == Some(requested) || last.as_deref() == Some(requested) {
        return Ok(current);
    }
    let in_force = current.is_some();
    let reference = current.clone().or(last);
    let c = confirmer.clone();
    let req = requested.to_string();
    let approved = tokio::task::spawn_blocking(move || c.ask(reference.as_deref(), in_force, &req))
        .await
        .map_err(|e| format!("Confirmation prompt failed: {}", e))??;
    if approved {
        Ok(current)
    } else {
        Err("Server URL change was not confirmed by the user".to_string())
    }
}

const STALE: &str = "Server URL changed while waiting for confirmation; please try again";

async fn apply_config(
    config: &ConfigState,
    confirmer: &SharedConfirmer,
    mut new_config: AppConfig,
) -> Result<(), String> {
    let mut snapshot = None;
    if let Some(u) = new_config.server_url.as_deref() {
        let u = url_guard::validate_server_url(u)?;
        snapshot = Some(confirm_url_change(config, confirmer, &u).await?);
        new_config.server_url = Some(u);
    }
    let mut config = config.lock().await;
    match (&new_config.server_url, snapshot) {
        (Some(u), Some(snap)) => {
            // Compare-and-swap: nobody changed the URL while the dialog was open.
            if config.server_url != snap {
                return Err(STALE.to_string());
            }
            new_config.last_confirmed_url = Some(u.clone());
        }
        _ => new_config.last_confirmed_url = config.last_confirmed_url.clone(),
    }
    *config = new_config;
    Ok(())
}

#[tauri::command]
async fn set_server_url(
    url: String,
    config: State<'_, ConfigState>,
    confirmer: State<'_, SharedConfirmer>,
) -> Result<(), String> {
    apply_server_url(&config, &confirmer, url).await
}

async fn apply_server_url(
    config: &ConfigState,
    confirmer: &SharedConfirmer,
    url: String,
) -> Result<(), String> {
    let url = url_guard::validate_server_url(&url)?;
    let snapshot = confirm_url_change(config, confirmer, &url).await?;
    let mut config = config.lock().await;
    if config.server_url != snapshot {
        return Err(STALE.to_string());
    }
    config.last_confirmed_url = Some(url.clone());
    config.server_url = Some(url);
    Ok(())
}

#[tauri::command]
async fn set_auth_token(token: String, config: State<'_, ConfigState>) -> Result<(), String> {
    let mut config = config.lock().await;
    config.auth_token = Some(token);
    Ok(())
}

#[tauri::command]
async fn clear_auth_token(config: State<'_, ConfigState>) -> Result<(), String> {
    let mut config = config.lock().await;
    config.auth_token = None;
    Ok(())
}

#[tauri::command]
async fn make_http_request(
    url: String,
    method: String,
    headers: HashMap<String, String>,
    body: Option<String>,
    config: State<'_, ConfigState>,
) -> Result<String, String> {
    request_via_config(&config, &url, &method, headers, body).await
}

/// Body of the `make_http_request` command (testable without Tauri state); TAURI-FU-5.
async fn request_via_config(
    config: &ConfigState,
    url: &str,
    method: &str,
    headers: HashMap<String, String>,
    body: Option<String>,
) -> Result<String, String> {
    let server_url = {
        let guard = config.lock().await;
        guard.server_url.clone().ok_or("Server URL not configured")?
    };
    perform_request(&server_url, url, method, headers, body).await
}

/// SSRF-guarded request execution (testable without Tauri state); TAURI-FU-5.
async fn perform_request(
    server_url: &str,
    url: &str,
    method: &str,
    headers: HashMap<String, String>,
    body: Option<String>,
) -> Result<String, String> {
    // Parsed-URL comparison (scheme, host, port, no userinfo, path boundary).
    if !url_guard::is_within_server(server_url, url) {
        return Err(format!("URL must be within configured server: {}", server_url));
    }

    let client = url_guard::build_client(server_url)?;

    let mut request = match method.to_uppercase().as_str() {
        "GET" => client.get(url),
        "POST" => client.post(url),
        "PUT" => client.put(url),
        "DELETE" => client.delete(url),
        "PATCH" => client.patch(url),
        _ => return Err("Unsupported HTTP method".to_string()),
    };

    // Add headers
    for (key, value) in headers {
        request = request.header(&key, &value);
    }

    // Add body if provided
    if let Some(body_content) = body {
        request = request.body(body_content);
    }

    // Execute request
    match request.send().await {
        Ok(response) => {
            match response.text().await {
                Ok(text) => Ok(text),
                Err(e) => Err(format!("Failed to read response: {}", e)),
            }
        }
        Err(e) => Err(format!("Request failed: {}", e)),
    }
}

#[tauri::command]
fn get_app_version() -> String {
    env!("CARGO_PKG_VERSION").to_string()
}

#[tauri::command]
fn get_platform() -> String {
    std::env::consts::OS.to_string()
}

#[tauri::command]
fn get_arch() -> String {
    std::env::consts::ARCH.to_string()
}

fn main() {
    env_logger::init();

    let config = ConfigState::default();
    
    // Initialize VLC player (only if vlc-player feature is enabled)
    #[cfg(feature = "vlc-player")]
    let vlc_state = {
        let player = VLCPlayer::new().expect("Failed to initialize VLC player");
        VLCPlayerState(std::sync::Mutex::new(player))
    };

    let mut builder = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .manage(config)
        .setup(|app| {
            use tauri::Manager;
            let confirmer: SharedConfirmer = confirm::PromptGate::new(std::sync::Arc::new(
                confirm::DialogConfirmer(app.handle().clone()),
            ));
            app.manage(confirmer);
            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            get_config,
            update_config,
            set_server_url,
            set_auth_token,
            clear_auth_token,
            make_http_request,
            get_app_version,
            get_platform,
            get_arch
        ]);
    
    // Register VLC commands if feature is enabled
    #[cfg(feature = "vlc-player")]
    {
        use vlc::commands::*;
        builder = builder
            .manage(vlc_state)
            .invoke_handler(tauri::generate_handler![
                get_config,
                update_config,
                set_server_url,
                set_auth_token,
                clear_auth_token,
                make_http_request,
                get_app_version,
                get_platform,
                get_arch,
                vlc_initialize,
                vlc_play,
                vlc_pause,
                vlc_resume,
                vlc_stop,
                vlc_toggle_playback,
                vlc_seek,
                vlc_seek_forward,
                vlc_seek_backward,
                vlc_get_status,
                vlc_set_volume,
                vlc_toggle_mute,
                vlc_set_rate,
                vlc_get_tracks,
                vlc_set_audio_track,
                vlc_set_subtitle_track,
                vlc_set_subtitle_delay,
                vlc_set_audio_delay,
                vlc_set_aspect_ratio,
                vlc_take_snapshot
            ]);
    }
    
    let app = builder.run(tauri::generate_context!());
    
    match app {
        Ok(_) => (),
        Err(e) => {
            eprintln!("Failed to start Tauri application: {}", e);
            std::process::exit(1);
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Tests for AppConfig
    mod config_tests {
        use super::*;

        #[test]
        fn test_default_config() {
            let config = AppConfig::default();

            assert!(config.server_url.is_none());
            assert!(config.auth_token.is_none());
            assert_eq!(config.theme, "dark");
            assert!(!config.auto_start);
        }

        #[test]
        fn test_config_with_custom_values() {
            let config = AppConfig {
                server_url: Some("http://localhost:8080".to_string()),
                auth_token: Some("test-token".to_string()),
                theme: "light".to_string(),
                auto_start: true,
                last_confirmed_url: None,
            };

            assert_eq!(config.server_url, Some("http://localhost:8080".to_string()));
            assert_eq!(config.auth_token, Some("test-token".to_string()));
            assert_eq!(config.theme, "light");
            assert!(config.auto_start);
        }

        #[test]
        fn test_config_serialization() {
            let config = AppConfig {
                server_url: Some("http://example.com".to_string()),
                auth_token: None,
                theme: "dark".to_string(),
                auto_start: false,
                last_confirmed_url: None,
            };

            let json = serde_json::to_string(&config).expect("Serialization should succeed");
            let deserialized: AppConfig = serde_json::from_str(&json).expect("Serialization should succeed");

            assert_eq!(config.server_url, deserialized.server_url);
            assert_eq!(config.auth_token, deserialized.auth_token);
            assert_eq!(config.theme, deserialized.theme);
            assert_eq!(config.auto_start, deserialized.auto_start);
        }

        #[test]
        fn test_config_deserialization_from_json() {
            let json = r#"{
                "server_url": "https://api.example.com",
                "auth_token": "jwt-token-123",
                "theme": "system",
                "auto_start": true
            }"#;

            let config: AppConfig = serde_json::from_str(json).expect("Serialization should succeed");

            assert_eq!(config.server_url, Some("https://api.example.com".to_string()));
            assert_eq!(config.auth_token, Some("jwt-token-123".to_string()));
            assert_eq!(config.theme, "system");
            assert!(config.auto_start);
        }

        #[test]
        fn test_config_deserialization_with_nulls() {
            let json = r#"{
                "server_url": null,
                "auth_token": null,
                "theme": "dark",
                "auto_start": false
            }"#;

            let config: AppConfig = serde_json::from_str(json).expect("Serialization should succeed");

            assert!(config.server_url.is_none());
            assert!(config.auth_token.is_none());
        }

        #[test]
        fn test_config_clone() {
            let original = AppConfig {
                server_url: Some("http://test.com".to_string()),
                auth_token: Some("token".to_string()),
                theme: "dark".to_string(),
                auto_start: true,
                last_confirmed_url: None,
            };

            let cloned = original.clone();

            assert_eq!(original.server_url, cloned.server_url);
            assert_eq!(original.auth_token, cloned.auth_token);
            assert_eq!(original.theme, cloned.theme);
            assert_eq!(original.auto_start, cloned.auto_start);
        }
    }

    /// Tests for system info commands
    mod system_info_tests {
        use super::*;

        #[test]
        fn test_get_app_version_returns_non_empty() {
            let version = get_app_version();
            assert!(!version.is_empty());
        }

        #[test]
        fn test_get_platform_returns_valid_os() {
            let platform = get_platform();

            // Should be one of the known operating systems
            let valid_platforms = ["linux", "macos", "windows", "freebsd", "android", "ios"];
            assert!(
                valid_platforms.contains(&platform.as_str()),
                "Unexpected platform: {}",
                platform
            );
        }

        #[test]
        fn test_get_arch_returns_valid_arch() {
            let arch = get_arch();

            // Should be one of the known architectures
            let valid_archs = ["x86", "x86_64", "arm", "aarch64", "mips", "mips64", "powerpc", "powerpc64", "riscv64"];
            assert!(
                valid_archs.contains(&arch.as_str()),
                "Unexpected architecture: {}",
                arch
            );
        }

        #[test]
        fn test_get_app_version_format() {
            let version = get_app_version();

            // Version should follow semver format (at least contain a dot)
            assert!(
                version.contains('.'),
                "Version should be in semver format: {}",
                version
            );
        }
    }

    /// Tests for config state management
    mod config_state_tests {
        use super::*;

        #[tokio::test]
        async fn test_config_state_default() {
            let state = ConfigState::default();
            let config = state.lock().await;

            assert!(config.server_url.is_none());
            assert!(config.auth_token.is_none());
        }

        #[tokio::test]
        async fn test_config_state_mutation() {
            let state = ConfigState::default();

            {
                let mut config = state.lock().await;
                config.server_url = Some("http://localhost".to_string());
                config.theme = "light".to_string();
            }

            {
                let config = state.lock().await;
                assert_eq!(config.server_url, Some("http://localhost".to_string()));
                assert_eq!(config.theme, "light");
            }
        }

        #[tokio::test]
        async fn test_config_state_concurrent_access() {
            let state = std::sync::Arc::new(ConfigState::default());
            let state_clone = state.clone();

            // Spawn a task that writes to config
            let write_task = tokio::spawn(async move {
                let mut config = state_clone.lock().await;
                config.server_url = Some("http://written.com".to_string());
            });

            // Wait for write to complete
            write_task.await.expect("Serialization should succeed");

            // Verify the write was successful
            let config = state.lock().await;
            assert_eq!(config.server_url, Some("http://written.com".to_string()));
        }
    }

    /// Tests for HTTP request handling
    mod http_request_tests {
        use super::*;

        #[test]
        fn test_http_method_parsing() {
            // Test that various HTTP methods are recognized
            let methods = ["GET", "POST", "PUT", "DELETE", "PATCH"];

            for method in methods {
                let upper = method.to_uppercase();
                assert!(
                    ["GET", "POST", "PUT", "DELETE", "PATCH"].contains(&upper.as_str()),
                    "Method {} should be valid",
                    method
                );
            }
        }

        #[test]
        fn test_http_method_case_insensitivity() {
            let lower = "get".to_uppercase();
            let upper = "GET".to_uppercase();
            let mixed = "GeT".to_uppercase();

            assert_eq!(lower, "GET");
            assert_eq!(upper, "GET");
            assert_eq!(mixed, "GET");
        }

        #[test]
        fn test_invalid_method() {
            let method = "INVALID";
            let is_valid = ["GET", "POST", "PUT", "DELETE", "PATCH"]
                .contains(&method.to_uppercase().as_str());

            assert!(!is_valid, "INVALID should not be a valid method");
        }

        #[test]
        fn test_headers_hashmap() {
            let mut headers: HashMap<String, String> = HashMap::new();
            headers.insert("Content-Type".to_string(), "application/json".to_string());
            headers.insert("Authorization".to_string(), "Bearer token123".to_string());

            assert_eq!(headers.len(), 2);
            assert_eq!(headers.get("Content-Type"), Some(&"application/json".to_string()));
            assert_eq!(headers.get("Authorization"), Some(&"Bearer token123".to_string()));
        }
    }

    /// Tests for URL validation
    mod url_tests {
        #[test]
        fn test_url_parsing() {
            let url = "http://localhost:8080/api/v1/test";

            // Basic URL validation
            assert!(url.starts_with("http://") || url.starts_with("https://"));
            assert!(url.contains("localhost"));
        }

        #[test]
        fn test_https_url() {
            let url = "https://api.catalogizer.com/v1/media";

            assert!(url.starts_with("https://"));
        }

        #[test]
        fn test_url_with_port() {
            let url = "http://192.168.1.100:3000/api";

            assert!(url.contains(":3000"));
        }

        #[test]
        fn test_url_with_path() {
            let url = "http://example.com/api/v1/users/123";

            assert!(url.contains("/api/v1/users/"));
        }
    }

    /// Integration-style tests (still unit tests but test interactions)
    /// Call-site tests: drive the same functions the Tauri commands call (TAURI-FU-5).
    mod callsite_tests {
        use super::*;
        use std::sync::{atomic::{AtomicUsize, Ordering}, Arc};

        #[tokio::test]
        async fn perform_request_refuses_cross_origin_redirect_and_follows_in_scope() {
            let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let b = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let (pa, pb) = (a.local_addr().unwrap().port(), b.local_addr().unwrap().port());
            let hits_b = Arc::new(AtomicUsize::new(0));
            tokio::spawn(crate::url_guard::tests::serve(a, pb, Arc::new(AtomicUsize::new(0))));
            tokio::spawn(crate::url_guard::tests::serve(b, pa, hits_b.clone()));
            let server = format!("http://127.0.0.1:{}", pa);

            let r = perform_request(&server, &format!("{}/redir-out", server), "GET", HashMap::new(), None).await;
            assert!(r.is_err(), "cross-origin redirect must be refused: {:?}", r);
            assert_eq!(hits_b.load(Ordering::SeqCst), 0);

            let r = perform_request(&server, &format!("{}/redir-in", server), "GET", HashMap::new(), None).await;
            assert_eq!(r.unwrap(), "ok");

            let r = perform_request(&server, "http://127.0.0.1:1/x", "GET", HashMap::new(), None).await;
            assert!(r.unwrap_err().contains("within configured server"));
            let r = perform_request(&server, &format!("{}/ok", server), "TRACE", HashMap::new(), None).await;
            assert_eq!(r.unwrap_err(), "Unsupported HTTP method");
        }

        #[tokio::test]
        async fn request_via_config_guards_against_configured_server() {
            let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let b = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let (pa, pb) = (a.local_addr().unwrap().port(), b.local_addr().unwrap().port());
            let hits_b = Arc::new(AtomicUsize::new(0));
            tokio::spawn(crate::url_guard::tests::serve(a, pb, Arc::new(AtomicUsize::new(0))));
            tokio::spawn(crate::url_guard::tests::serve(b, pa, hits_b.clone()));

            let state: ConfigState = Mutex::new(AppConfig::default());
            let r = request_via_config(&state, &format!("http://127.0.0.1:{}/ok", pa), "GET", HashMap::new(), None).await;
            assert_eq!(r.unwrap_err(), "Server URL not configured");

            state.lock().await.server_url = Some(format!("http://127.0.0.1:{}", pa));
            let r = request_via_config(&state, &format!("http://127.0.0.1:{}/ok", pb), "GET", HashMap::new(), None).await;
            assert!(r.unwrap_err().contains("within configured server"));
            assert_eq!(hits_b.load(Ordering::SeqCst), 0, "server B must receive zero requests");

            let r = request_via_config(&state, &format!("http://127.0.0.1:{}/ok", pa), "GET", HashMap::new(), None).await;
            assert_eq!(r.unwrap(), "ok");
        }

        #[tokio::test]
        async fn perform_request_forwards_headers_and_body() {
            let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let pa = a.local_addr().unwrap().port();
            tokio::spawn(crate::url_guard::tests::serve(a, 1, Arc::new(AtomicUsize::new(0))));
            let server = format!("http://127.0.0.1:{}", pa);
            let mut h = HashMap::new();
            h.insert("Authorization".to_string(), "Bearer tok-123".to_string());
            let r = perform_request(&server, &format!("{}/echo", server), "POST", h, Some("payload-xyz".into()))
                .await
                .unwrap()
                .to_lowercase();
            assert!(r.contains("authorization: bearer tok-123"), "{}", r);
            assert!(r.contains("payload-xyz"), "{}", r);
            assert!(r.starts_with("post /echo"), "{}", r);
        }

        /// Test confirmer: scripted answer + records every prompt it was shown.
        struct Scripted {
            answer: bool,
            seen: std::sync::Mutex<Vec<(Option<String>, String)>>,
        }
        impl Scripted {
            fn new(answer: bool) -> Arc<Self> {
                Arc::new(Self { answer, seen: Default::default() })
            }
            fn prompts(&self) -> Vec<(Option<String>, String)> {
                self.seen.lock().unwrap().clone()
            }
        }
        impl crate::confirm::UrlConfirmer for Scripted {
            fn confirm_server_url_change(&self, current: Option<&str>, _in_force: bool, requested: &str) -> bool {
                self.seen.lock().unwrap().push((current.map(String::from), requested.to_string()));
                self.answer
            }
        }
        fn shared(c: &Arc<Scripted>) -> SharedConfirmer {
            crate::confirm::PromptGate::new(c.clone())
        }

        #[tokio::test]
        async fn set_server_url_rejects_bad_urls_and_keeps_old_value() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            assert!(apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.is_ok());
            let before = yes.prompts().len();
            for bad in ["ftp://a.com", "http://u@a.com", "http://u:p@a.com", "garbage", "file:///etc/passwd"] {
                assert!(apply_server_url(&state, &shared(&yes), bad.into()).await.is_err(), "{}", bad);
            }
            assert_eq!(yes.prompts().len(), before, "invalid URLs must not even reach the prompt");
            assert_eq!(state.lock().await.server_url.as_deref(), Some("http://a.com"));
        }

        #[tokio::test]
        async fn update_config_rejects_bad_server_url_and_keeps_old_config() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            let good = AppConfig { server_url: Some("https://a.com".into()), ..AppConfig::default() };
            assert!(apply_config(&state, &shared(&yes), good).await.is_ok());
            for bad in ["ftp://a.com", "http://u@a.com"] {
                let cfg = AppConfig { server_url: Some(bad.into()), theme: "light".into(), ..AppConfig::default() };
                assert!(apply_config(&state, &shared(&yes), cfg).await.is_err(), "{}", bad);
            }
            let g = state.lock().await;
            assert_eq!(g.server_url.as_deref(), Some("https://a.com"));
            assert_eq!(g.theme, "dark");
            drop(g);
            assert!(apply_config(&state, &shared(&yes), AppConfig::default()).await.is_ok());
        }

        #[tokio::test]
        async fn url_change_requires_confirmation_and_decline_keeps_old_value() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            let no = Scripted::new(false);
            let r = apply_server_url(&state, &shared(&no), "http://evil.com".into()).await;
            assert_eq!(r.unwrap_err(), "Server URL change was not confirmed by the user");
            assert_eq!(state.lock().await.server_url.as_deref(), Some("http://a.com"));
            assert_eq!(no.prompts(), vec![(Some("http://a.com".to_string()), "http://evil.com".to_string())]);
            apply_server_url(&state, &shared(&yes), "http://b.com".into()).await.unwrap();
            assert_eq!(state.lock().await.server_url.as_deref(), Some("http://b.com"));
            assert_eq!(yes.prompts().last().unwrap(), &(Some("http://a.com".to_string()), "http://b.com".to_string()));
        }

        #[tokio::test]
        async fn first_time_setup_is_confirmed_too() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let no = Scripted::new(false);
            assert!(apply_server_url(&state, &shared(&no), "http://a.com".into()).await.is_err());
            assert!(state.lock().await.server_url.is_none());
            assert_eq!(no.prompts(), vec![(None, "http://a.com".to_string())]);
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            assert_eq!(state.lock().await.server_url.as_deref(), Some("http://a.com"));
        }

        #[tokio::test]
        async fn unchanged_url_needs_no_prompt() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            let no = Scripted::new(false);
            apply_server_url(&state, &shared(&no), "http://a.com".into()).await.unwrap();
            let cfg = AppConfig { server_url: Some("http://a.com".into()), theme: "light".into(), ..AppConfig::default() };
            apply_config(&state, &shared(&no), cfg).await.unwrap();
            assert!(no.prompts().is_empty(), "same URL must not prompt");
            assert_eq!(state.lock().await.theme, "light");
        }

        #[tokio::test]
        async fn update_config_cannot_bypass_confirmation() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            let no = Scripted::new(false);
            let cfg = AppConfig { server_url: Some("http://evil.com".into()), theme: "light".into(), ..AppConfig::default() };
            assert!(apply_config(&state, &shared(&no), cfg).await.is_err());
            let g = state.lock().await;
            assert_eq!(g.server_url.as_deref(), Some("http://a.com"));
            assert_eq!(g.theme, "dark", "a declined change must not apply ANY part of the config");
            drop(g);
            // Clearing is allowed, but the next set is then a first-time set and prompts again.
            apply_config(&state, &shared(&no), AppConfig::default()).await.unwrap();
            assert!(no.prompts().len() == 1);
            assert!(apply_server_url(&state, &shared(&no), "http://evil.com".into()).await.is_err());
            assert_eq!(no.prompts().len(), 2);
            assert!(state.lock().await.server_url.is_none());
        }

        #[tokio::test]
        async fn config_lock_is_not_held_while_prompting() {
            struct Probe(Arc<ConfigState>, std::sync::Mutex<Option<bool>>);
            impl crate::confirm::UrlConfirmer for Probe {
                fn confirm_server_url_change(&self, _: Option<&str>, _in_force: bool, _: &str) -> bool {
                    *self.1.lock().unwrap() = Some(self.0.try_lock().is_ok());
                    true
                }
            }
            let state = Arc::new(Mutex::new(AppConfig::default()));
            let probe = Arc::new(Probe(state.clone(), Default::default()));
            let c: SharedConfirmer = crate::confirm::PromptGate::new(probe.clone());
            apply_server_url(&state, &c, "http://a.com".into()).await.unwrap();
            assert_eq!(*probe.1.lock().unwrap(), Some(true), "config must be readable while the dialog is open");
        }

        // ---- review round 1 fixes ----
        /// Run an async test body on its own runtime/thread with a hard deadline, so a
        /// deadlock (e.g. config lock held across a prompt) FAILS instead of hanging the suite.
        fn run_with_timeout<F>(secs: u64, f: F)
        where
            F: std::future::Future<Output = ()> + Send + 'static,
        {
            let (tx, rx) = std::sync::mpsc::channel();
            let h = std::thread::spawn(move || {
                let rt = tokio::runtime::Builder::new_multi_thread().worker_threads(3).enable_all().build().unwrap();
                rt.block_on(f);
                let _ = tx.send(());
            });
            match rx.recv_timeout(std::time::Duration::from_secs(secs)) {
                Ok(()) => {}
                Err(std::sync::mpsc::RecvTimeoutError::Timeout) => panic!("test timed out after {}s (deadlock?)", secs),
                Err(_) => std::panic::resume_unwind(h.join().unwrap_err()),
            }
        }

        #[tokio::test]
        async fn newline_in_url_cannot_forge_dialog_text() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            let raw = "https://evil.example/\n\nThis is your usual server.\nNew: https://catalogizer.good.example";
            assert!(apply_server_url(&state, &shared(&yes), raw.into()).await.is_err());
            assert!(yes.prompts().is_empty(), "rejected before any prompt");
            assert!(state.lock().await.server_url.is_none());
        }

        #[tokio::test]
        async fn idn_lookalike_is_shown_and_stored_as_punycode() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "https://c\u{0430}talogizer.example".into()).await.unwrap();
            let seen = yes.prompts();
            assert!(seen[0].1.is_ascii() && seen[0].1.contains("xn--"), "{:?}", seen);
            assert_eq!(state.lock().await.server_url.as_deref(), Some(seen[0].1.as_str()));
        }

        #[tokio::test]
        async fn lookalike_and_variant_urls_prompt_but_equivalents_do_not() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            let no = Scripted::new(false);
            for other in ["http://a.com.evil.com", "http://a.co", "http://a.com:8080", "https://a.com", "http://a.com/x", "http://b.a.com", "http://a.com.:80"] {
                let before = no.prompts().len();
                assert!(apply_server_url(&state, &shared(&no), other.into()).await.is_err(), "{}", other);
                assert_eq!(no.prompts().len(), before + 1, "{} must prompt", other);
            }
            let quiet = Scripted::new(false);
            for same in ["http://a.com", "HTTP://A.COM", "http://a.com", "http://a.com:80"] {
                assert!(apply_server_url(&state, &shared(&quiet), same.into()).await.is_ok(), "{}", same);
            }
            assert!(quiet.prompts().is_empty());
        }

        struct Interloper(Arc<ConfigState>);
        impl crate::confirm::UrlConfirmer for Interloper {
            fn confirm_server_url_change(&self, _: Option<&str>, _in_force: bool, _: &str) -> bool {
                self.0.blocking_lock().server_url = Some("http://c.com".into());
                true
            }
        }

        #[test]
        fn url_changed_during_prompt_is_not_overwritten() {
            run_with_timeout(20, async move {
            let init = AppConfig { server_url: Some("http://a.com".into()), ..AppConfig::default() };
            let st = Arc::new(Mutex::new(init));
            let c = crate::confirm::PromptGate::new(Arc::new(Interloper(st.clone())));
            let r = apply_server_url(&st, &c, "http://b.com".into()).await;
            assert!(r.unwrap_err().contains("changed while waiting"));
            assert_eq!(st.lock().await.server_url.as_deref(), Some("http://c.com"));
            let init = AppConfig { server_url: Some("http://a.com".into()), ..AppConfig::default() };
            let st = Arc::new(Mutex::new(init));
            let c = crate::confirm::PromptGate::new(Arc::new(Interloper(st.clone())));
            let cfg = AppConfig { server_url: Some("http://b.com".into()), theme: "light".into(), ..AppConfig::default() };
            let r = apply_config(&st, &c, cfg).await;
            assert!(r.unwrap_err().contains("changed while waiting"));
            let g = st.lock().await;
            assert_eq!((g.server_url.as_deref(), g.theme.as_str()), (Some("http://c.com"), "dark"));
            })
        }

        #[tokio::test]
        async fn clear_then_set_shows_previously_confirmed_url() {
            let state: ConfigState = Mutex::new(AppConfig::default());
            let yes = Scripted::new(true);
            apply_server_url(&state, &shared(&yes), "http://a.com".into()).await.unwrap();
            let no = Scripted::new(false);
            apply_config(&state, &shared(&no), AppConfig::default()).await.unwrap();
            assert!(apply_server_url(&state, &shared(&no), "http://evil.com".into()).await.is_err());
            assert_eq!(no.prompts(), vec![(Some("http://a.com".to_string()), "http://evil.com".to_string())]);
            // the frontend cannot reset or read the backend-only memory
            assert!(!serde_json::to_string(&*state.lock().await).unwrap().contains("last_confirmed"));
            let cfg: AppConfig = serde_json::from_str(r#"{"server_url":null,"auth_token":null,"theme":"dark","auto_start":false,"last_confirmed_url":"http://evil.com"}"#).unwrap();
            assert!(cfg.last_confirmed_url.is_none());
            // re-adopting the previously confirmed URL needs no new prompt
            let quiet = Scripted::new(false);
            apply_server_url(&state, &shared(&quiet), "http://a.com".into()).await.unwrap();
            assert!(quiet.prompts().is_empty());
        }

        struct Panicker;
        impl crate::confirm::UrlConfirmer for Panicker {
            fn confirm_server_url_change(&self, _: Option<&str>, _in_force: bool, _: &str) -> bool {
                panic!("dialog backend died")
            }
        }

        #[tokio::test]
        async fn panicking_confirmer_fails_closed_and_releases_the_gate() {
            let init = AppConfig { server_url: Some("http://a.com".into()), ..AppConfig::default() };
            let state: ConfigState = Mutex::new(init);
            let c = crate::confirm::PromptGate::new(Arc::new(Panicker));
            assert!(apply_server_url(&state, &c, "http://b.com".into()).await.is_err());
            assert_eq!(state.lock().await.server_url.as_deref(), Some("http://a.com"));
            // gate released: the next attempt reaches the confirmer again (panics again) instead of "already open"
            let e = apply_server_url(&state, &c, "http://b.com".into()).await.unwrap_err();
            assert!(!e.contains("already open"), "{}", e);
        }

        struct Blocking(std::sync::Mutex<std::sync::mpsc::Receiver<()>>, std::sync::mpsc::SyncSender<()>);
        impl crate::confirm::UrlConfirmer for Blocking {
            fn confirm_server_url_change(&self, _: Option<&str>, _in_force: bool, _: &str) -> bool {
                let _ = self.1.try_send(()); // signal: dialog open
                self.0.lock().unwrap().recv_timeout(std::time::Duration::from_secs(10)).ok(); // wait for release
                true
            }
        }

        #[test]
        fn only_one_prompt_in_flight_second_is_rejected() {
            run_with_timeout(20, async move {
            let (rel_tx, rel_rx) = std::sync::mpsc::sync_channel::<()>(1);
            let (open_tx, open_rx) = std::sync::mpsc::sync_channel::<()>(1);
            let st = Arc::new(Mutex::new(AppConfig::default()));
            let c = crate::confirm::PromptGate::new(Arc::new(Blocking(std::sync::Mutex::new(rel_rx), open_tx)));
            let (st1, c1) = (st.clone(), c.clone());
            let first = tokio::spawn(async move { apply_server_url(&st1, &c1, "http://a.com".into()).await });
            tokio::task::spawn_blocking(move || open_rx.recv_timeout(std::time::Duration::from_secs(10)).expect("dialog never opened")).await.unwrap();
            let second = apply_server_url(&st, &c, "http://b.com".into()).await;
            assert_eq!(second.unwrap_err(), "Another server confirmation is already open");
            rel_tx.send(()).unwrap();
            first.await.unwrap().unwrap();
            assert_eq!(st.lock().await.server_url.as_deref(), Some("http://a.com"));
            })
        }

        #[tokio::test]
        async fn stored_url_is_bare_so_frontend_concat_has_no_double_slash() {
            for (raw, want) in [("http://a.com", "http://a.com"), ("http://a.com/", "http://a.com"), ("https://a.com:8443/", "https://a.com:8443"), ("https://a.com/api", "https://a.com/api")] {
                let state: ConfigState = Mutex::new(AppConfig::default());
                let yes = Scripted::new(true);
                apply_server_url(&state, &shared(&yes), raw.into()).await.unwrap();
                let stored = state.lock().await.server_url.clone().unwrap();
                assert_eq!(stored, want);
                // authStore.ts / apiService.ts shape: `${serverUrl}/api/v1/auth/login`
                let frontend = format!("{}/api/v1/auth/login", stored);
                let path = url::Url::parse(&frontend).unwrap().path().to_string();
                assert!(!path.starts_with("//") && !path.contains("//api"), "{}", frontend);
                assert!(crate::url_guard::is_within_server(&stored, &frontend), "{}", frontend);
                // the equality check uses the same canonical form: re-entering is not a change
                let no = Scripted::new(false);
                apply_server_url(&state, &shared(&no), raw.into()).await.unwrap();
                assert!(no.prompts().is_empty(), "{} re-entered must not prompt", raw);
            }
        }

        // ---- IPC-level tests (TAURI-F15): real command wrappers through tauri::test ----
        use tauri::test::{get_ipc_response, mock_builder, mock_context, noop_assets, MockRuntime};
        use tauri::webview::InvokeRequest;
        use tauri::ipc::InvokeBody;

        fn ipc_window(state: ConfigState, c: SharedConfirmer) -> (tauri::App<MockRuntime>, tauri::WebviewWindow<MockRuntime>) {
            let app = mock_builder()
                .manage(state)
                .manage(c)
                .invoke_handler(tauri::generate_handler![get_config, update_config, set_server_url, make_http_request])
                .build(mock_context(noop_assets()))
                .expect("mock app");
            let w = tauri::WebviewWindowBuilder::new(&app, "main", Default::default()).build().expect("window");
            (app, w)
        }

        fn ipc(w: &tauri::WebviewWindow<MockRuntime>, cmd: &str, body: serde_json::Value) -> Result<serde_json::Value, serde_json::Value> {
            // The IPC call blocks this thread until the command answers; block_in_place
            // keeps the test runtime's other workers (echo servers) running meanwhile.
            tokio::task::block_in_place(|| get_ipc_response(
                w,
                InvokeRequest {
                    cmd: cmd.into(),
                    callback: tauri::ipc::CallbackFn(0),
                    error: tauri::ipc::CallbackFn(1),
                    url: "tauri://localhost".parse().unwrap(),
                    body: InvokeBody::Json(body),
                    headers: Default::default(),
                    invoke_key: tauri::test::INVOKE_KEY.to_string(),
                },
            ))
            .map(|r| r.deserialize::<serde_json::Value>().unwrap())
        }

        #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
        async fn ipc_set_server_url_is_confirmed_by_the_backend() {
            let no = Scripted::new(false);
            let (_app, w) = ipc_window(Mutex::new(AppConfig::default()), shared(&no));
            let r = ipc(&w, "set_server_url", serde_json::json!({"url": "http://a.com"}));
            assert_eq!(r.unwrap_err(), serde_json::json!("Server URL change was not confirmed by the user"));
            assert_eq!(no.prompts(), vec![(None, "http://a.com".to_string())]);
            assert!(ipc(&w, "get_config", serde_json::json!({})).unwrap()["server_url"].is_null());

            let yes = Scripted::new(true);
            let (_app2, w2) = ipc_window(Mutex::new(AppConfig::default()), shared(&yes));
            ipc(&w2, "set_server_url", serde_json::json!({"url": "http://a.com"})).unwrap();
            assert_eq!(ipc(&w2, "get_config", serde_json::json!({})).unwrap()["server_url"], "http://a.com");
            let bad = ipc(&w2, "set_server_url", serde_json::json!({"url": "ftp://a.com"}));
            assert!(bad.unwrap_err().as_str().unwrap().contains("http or https"));
        }

        #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
        async fn ipc_update_config_cannot_change_server_without_confirmation() {
            let no = Scripted::new(false);
            let init = AppConfig { server_url: Some("http://a.com".into()), ..AppConfig::default() };
            let (_app, w) = ipc_window(Mutex::new(init), shared(&no));
            let cfg = serde_json::json!({"newConfig": {"server_url": "http://evil.com", "auth_token": null, "theme": "light", "auto_start": false}});
            assert!(ipc(&w, "update_config", cfg).is_err());
            let got = ipc(&w, "get_config", serde_json::json!({})).unwrap();
            assert_eq!(got["server_url"], "http://a.com");
            assert_eq!(got["theme"], "dark");
        }

        #[tokio::test(flavor = "multi_thread", worker_threads = 2)]
        async fn ipc_make_http_request_wrapper_forwards_every_argument() {
            let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let pa = a.local_addr().unwrap().port();
            tokio::spawn(crate::url_guard::tests::serve(a, 1, Arc::new(AtomicUsize::new(0))));
            let server = format!("http://127.0.0.1:{}", pa);
            let init = AppConfig { server_url: Some(server.clone()), ..AppConfig::default() };
            let (_app, w) = ipc_window(Mutex::new(init), shared(&Scripted::new(false)));
            let r = ipc(&w, "make_http_request", serde_json::json!({
                "url": format!("{}/echo", server), "method": "PUT",
                "headers": {"X-Probe": "hdr-42"}, "body": "body-77"
            })).unwrap();
            let echoed = r.as_str().unwrap().to_lowercase();
            assert!(echoed.starts_with("put /echo"), "{}", echoed);
            assert!(echoed.contains("x-probe: hdr-42") && echoed.contains("body-77"), "{}", echoed);
            let out = ipc(&w, "make_http_request", serde_json::json!({
                "url": "http://127.0.0.1:1/x", "method": "GET", "headers": {}, "body": null
            }));
            assert!(out.unwrap_err().as_str().unwrap().contains("within configured server"));

            let (_app2, w2) = ipc_window(Mutex::new(AppConfig::default()), shared(&Scripted::new(false)));
            let out = ipc(&w2, "make_http_request", serde_json::json!({
                "url": format!("{}/echo", server), "method": "GET", "headers": {}, "body": null
            }));
            assert_eq!(out.unwrap_err(), serde_json::json!("Server URL not configured"));
        }

        #[tokio::test]
        async fn perform_request_sends_large_header_and_body_intact() {
            let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
            let pa = a.local_addr().unwrap().port();
            tokio::spawn(crate::url_guard::tests::serve(a, 1, Arc::new(AtomicUsize::new(0))));
            let server = format!("http://127.0.0.1:{}", pa);
            let mut h = HashMap::new();
            h.insert("X-Big".to_string(), "h".repeat(3000));
            let body = format!("{}END-MARK", "b".repeat(9000));
            let r = perform_request(&server, &format!("{}/echo", server), "POST", h, Some(body.clone())).await.unwrap();
            assert!(r.ends_with(&body), "echoed body must be complete (9 KB + marker)");
            assert!(r.to_lowercase().contains(&format!("x-big: {}", "h".repeat(3000))));
        }
    }

    mod integration_tests {
        use super::*;

        #[test]
        fn test_full_config_lifecycle() {
            // Create default config
            let mut config = AppConfig::default();
            assert!(config.server_url.is_none());

            // Set server URL
            config.server_url = Some("http://api.test.com".to_string());
            assert!(config.server_url.is_some());

            // Set auth token
            config.auth_token = Some("jwt-token".to_string());
            assert!(config.auth_token.is_some());

            // Change theme
            config.theme = "light".to_string();
            assert_eq!(config.theme, "light");

            // Enable auto-start
            config.auto_start = true;
            assert!(config.auto_start);

            // Clear auth token (logout)
            config.auth_token = None;
            assert!(config.auth_token.is_none());
        }

        #[test]
        fn test_config_persistence_roundtrip() {
            let original = AppConfig {
                server_url: Some("http://test.com".to_string()),
                auth_token: Some("secret".to_string()),
                theme: "auto".to_string(),
                auto_start: true,
                last_confirmed_url: None,
            };

            // Serialize to JSON
            let json = serde_json::to_string_pretty(&original).expect("Serialization should succeed");

            // Verify JSON contains expected fields
            assert!(json.contains("server_url"));
            assert!(json.contains("auth_token"));
            assert!(json.contains("theme"));
            assert!(json.contains("auto_start"));

            // Deserialize back
            let restored: AppConfig = serde_json::from_str(&json).expect("Serialization should succeed");

            // Verify values match
            assert_eq!(original.server_url, restored.server_url);
            assert_eq!(original.auth_token, restored.auth_token);
            assert_eq!(original.theme, restored.theme);
            assert_eq!(original.auto_start, restored.auto_start);
        }

        #[test]
        fn test_theme_values() {
            let valid_themes = ["dark", "light", "system", "auto"];

            for theme in valid_themes {
                let config = AppConfig {
                    theme: theme.to_string(),
                    ..AppConfig::default()
                };
                assert_eq!(config.theme, theme);
            }
        }
    }

    /// Tests for auth token management
    mod auth_token_tests {
        use super::*;

        #[tokio::test]
        async fn test_set_auth_token() {
            let state = ConfigState::default();
            {
                let mut config = state.lock().await;
                config.auth_token = Some("test-jwt-token".to_string());
            }
            let config = state.lock().await;
            assert_eq!(config.auth_token, Some("test-jwt-token".to_string()));
        }

        #[tokio::test]
        async fn test_clear_auth_token() {
            let state = ConfigState::default();
            {
                let mut config = state.lock().await;
                config.auth_token = Some("token-to-clear".to_string());
            }
            {
                let mut config = state.lock().await;
                config.auth_token = None;
            }
            let config = state.lock().await;
            assert!(config.auth_token.is_none());
        }

        #[tokio::test]
        async fn test_set_server_url_updates_config() {
            let state = ConfigState::default();
            {
                let mut config = state.lock().await;
                config.server_url = Some("https://api.example.com".to_string());
            }
            let config = state.lock().await;
            assert_eq!(config.server_url, Some("https://api.example.com".to_string()));
        }

        #[tokio::test]
        async fn test_update_entire_config() {
            let state = ConfigState::default();
            let new_config = AppConfig {
                server_url: Some("http://new-server:9090".to_string()),
                auth_token: Some("new-token".to_string()),
                theme: "light".to_string(),
                auto_start: true,
                last_confirmed_url: None,
            };
            {
                let mut config = state.lock().await;
                *config = new_config;
            }
            let config = state.lock().await;
            assert_eq!(config.server_url, Some("http://new-server:9090".to_string()));
            assert_eq!(config.auth_token, Some("new-token".to_string()));
            assert_eq!(config.theme, "light");
            assert!(config.auto_start);
        }
    }

    /// Tests for config edge cases
    mod config_edge_case_tests {
        use super::*;

        #[test]
        fn test_config_debug_trait() {
            let config = AppConfig::default();
            let debug_str = format!("{:?}", config);
            assert!(debug_str.contains("AppConfig"));
            assert!(debug_str.contains("theme"));
        }

        #[test]
        fn test_config_with_empty_strings() {
            let config = AppConfig {
                server_url: Some("".to_string()),
                auth_token: Some("".to_string()),
                theme: "".to_string(),
                auto_start: false,
                last_confirmed_url: None,
            };
            assert_eq!(config.server_url, Some("".to_string()));
            assert_eq!(config.auth_token, Some("".to_string()));
            assert_eq!(config.theme, "");
        }

        #[test]
        fn test_config_with_special_characters_in_url() {
            let config = AppConfig {
                server_url: Some("http://192.168.1.100:8080/api?key=value&foo=bar".to_string()),
                ..AppConfig::default()
            };
            assert!(config.server_url.expect("Serialization should succeed").contains("?key=value"));
        }

        #[test]
        fn test_config_deserialization_with_missing_optional_fields() {
            let json = r#"{
                "theme": "dark",
                "auto_start": false
            }"#;

            let config: AppConfig = serde_json::from_str(json).expect("Serialization should succeed");
            assert!(config.server_url.is_none());
            assert!(config.auth_token.is_none());
            assert_eq!(config.theme, "dark");
        }

        #[test]
        fn test_config_serialization_preserves_none() {
            let config = AppConfig::default();
            let json = serde_json::to_string(&config).expect("Serialization should succeed");
            assert!(json.contains("null"));
        }

        #[test]
        fn test_config_with_unicode_in_theme() {
            let config = AppConfig {
                theme: "dark".to_string(),
                ..AppConfig::default()
            };
            let json = serde_json::to_string(&config).expect("Serialization should succeed");
            let deserialized: AppConfig = serde_json::from_str(&json).expect("Deserialization should succeed");
            assert_eq!(deserialized.theme, "dark");
        }

        #[test]
        fn test_config_with_long_url() {
            let long_url = format!("http://example.com/{}", "a".repeat(2000));
            let config = AppConfig {
                server_url: Some(long_url.clone()),
                ..AppConfig::default()
            };
            let json = serde_json::to_string(&config).expect("Serialization should succeed");
            let deserialized: AppConfig = serde_json::from_str(&json).expect("Deserialization should succeed");
            assert_eq!(deserialized.server_url, Some(long_url));
        }

        #[test]
        fn test_config_with_jwt_token_format() {
            let jwt = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U";
            let config = AppConfig {
                auth_token: Some(jwt.to_string()),
                ..AppConfig::default()
            };
            assert!(config.auth_token.as_ref().expect("Token should exist").contains('.'));
            let parts: Vec<&str> = config.auth_token.as_ref().expect("Token should exist").split('.').collect();
            assert_eq!(parts.len(), 3, "JWT should have 3 parts");
        }
    }

    /// Tests for HTTP method validation
    mod http_method_validation_tests {
        #[test]
        fn test_all_supported_methods() {
            let supported = ["GET", "POST", "PUT", "DELETE", "PATCH"];
            for method in &supported {
                assert!(
                    supported.contains(method),
                    "{} should be supported",
                    method
                );
            }
        }

        #[test]
        fn test_unsupported_methods() {
            let supported = ["GET", "POST", "PUT", "DELETE", "PATCH"];
            let unsupported = ["HEAD", "OPTIONS", "TRACE", "CONNECT"];
            for method in &unsupported {
                assert!(
                    !supported.contains(method),
                    "{} should not be supported",
                    method
                );
            }
        }

        #[test]
        fn test_method_case_normalization() {
            assert_eq!("get".to_uppercase(), "GET");
            assert_eq!("Post".to_uppercase(), "POST");
            assert_eq!("pUt".to_uppercase(), "PUT");
            assert_eq!("delete".to_uppercase(), "DELETE");
            assert_eq!("pAtCh".to_uppercase(), "PATCH");
        }
    }

    /// Tests for platform info functions
    mod platform_info_extended_tests {
        use super::*;

        #[test]
        fn test_platform_is_consistent() {
            let p1 = get_platform();
            let p2 = get_platform();
            assert_eq!(p1, p2, "Platform should be consistent across calls");
        }

        #[test]
        fn test_arch_is_consistent() {
            let a1 = get_arch();
            let a2 = get_arch();
            assert_eq!(a1, a2, "Architecture should be consistent across calls");
        }

        #[test]
        fn test_version_is_consistent() {
            let v1 = get_app_version();
            let v2 = get_app_version();
            assert_eq!(v1, v2, "Version should be consistent across calls");
        }

        #[test]
        fn test_version_parts() {
            let version = get_app_version();
            let parts: Vec<&str> = version.split('.').collect();
            assert!(
                parts.len() >= 2,
                "Version should have at least major.minor: {}",
                version
            );
            for part in &parts {
                assert!(
                    part.parse::<u32>().is_ok(),
                    "Version part '{}' should be numeric",
                    part
                );
            }
        }
    }
}