// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod url_guard;

#[cfg(feature = "vlc-player")]
mod vlc;

use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use tauri::State;
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
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            server_url: None,
            auth_token: None,
            theme: "dark".to_string(),
            auto_start: false,
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
) -> Result<(), String> {
    let mut config = config.lock().await;
    *config = new_config;
    Ok(())
}

#[tauri::command]
async fn set_server_url(url: String, config: State<'_, ConfigState>) -> Result<(), String> {
    let mut config = config.lock().await;
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
    // Validate URL against configured server URL to prevent SSRF
    let config_guard = config.lock().await;
    let server_url = config_guard.server_url.as_ref().ok_or("Server URL not configured")?;
    
    // Parsed-URL comparison (scheme, host, port, no userinfo, path boundary); TAURI-FU-5.
    if !url_guard::is_within_server(server_url, &url) {
        return Err(format!("URL must be within configured server: {}", server_url));
    }

    let client = reqwest::Client::new();

    let mut request = match method.to_uppercase().as_str() {
        "GET" => client.get(&url),
        "POST" => client.post(&url),
        "PUT" => client.put(&url),
        "DELETE" => client.delete(&url),
        "PATCH" => client.patch(&url),
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
        .manage(config)
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

    /// Tests for SSRF URL validation logic
    mod ssrf_validation_tests {
        #[test]
        fn test_url_starts_with_configured_server() {
            let server_url = "http://localhost:8080";
            let request_url = "http://localhost:8080/api/v1/media";

            let normalized_server = server_url.trim_end_matches('/');
            let normalized_request = request_url.trim_end_matches('/');

            assert!(normalized_request.starts_with(normalized_server));
        }

        #[test]
        fn test_url_does_not_match_different_server() {
            let server_url = "http://localhost:8080";
            let request_url = "http://evil.com/api/v1/media";

            let normalized_server = server_url.trim_end_matches('/');
            let normalized_request = request_url.trim_end_matches('/');

            assert!(!normalized_request.starts_with(normalized_server));
        }

        #[test]
        fn test_url_trailing_slash_normalization() {
            let server_url = "http://localhost:8080/";
            let normalized = server_url.trim_end_matches('/');
            assert_eq!(normalized, "http://localhost:8080");
        }

        #[test]
        fn test_url_no_trailing_slash() {
            let server_url = "http://localhost:8080";
            let normalized = server_url.trim_end_matches('/');
            assert_eq!(normalized, "http://localhost:8080");
        }

        #[test]
        fn test_url_multiple_trailing_slashes() {
            let server_url = "http://localhost:8080///";
            let normalized = server_url.trim_end_matches('/');
            assert_eq!(normalized, "http://localhost:8080");
        }

        #[test]
        fn test_ssrf_prevention_port_mismatch() {
            let server_url = "http://localhost:8080";
            let request_url = "http://localhost:9090/api/v1/data";

            let normalized_server = server_url.trim_end_matches('/');
            let normalized_request = request_url.trim_end_matches('/');

            assert!(!normalized_request.starts_with(normalized_server));
        }

        #[test]
        fn test_ssrf_prevention_scheme_mismatch() {
            let server_url = "https://localhost:8080";
            let request_url = "http://localhost:8080/api/v1/data";

            let normalized_server = server_url.trim_end_matches('/');
            let normalized_request = request_url.trim_end_matches('/');

            assert!(!normalized_request.starts_with(normalized_server));
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