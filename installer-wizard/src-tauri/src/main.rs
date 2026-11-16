// Prevents additional console window on Windows in release, DO NOT REMOVE!!
#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

use serde::{Deserialize, Serialize};

mod ftp;
mod local;
mod network;
mod nfs;
mod smb;
mod webdav;

#[derive(Debug, Serialize, Deserialize)]
pub struct NetworkHost {
    pub ip: String,
    pub hostname: Option<String>,
    pub mac_address: Option<String>,
    pub vendor: Option<String>,
    pub open_ports: Vec<u16>,
    pub smb_shares: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct SMBShare {
    pub host: String,
    pub share_name: String,
    pub path: String,
    pub writable: bool,
    pub description: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConfigurationSource {
    pub r#type: String,
    pub url: String,
    pub access: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConfigurationAccess {
    pub name: String,
    pub r#type: String,
    pub account: String,
    pub secret: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct Configuration {
    pub accesses: Vec<ConfigurationAccess>,
    pub sources: Vec<ConfigurationSource>,
}

// Learn more about Tauri commands at https://tauri.app/v1/guides/features/command
#[tauri::command]
async fn scan_network() -> Result<Vec<NetworkHost>, String> {
    network::scan_network().await.map_err(|e| e.to_string())
}

#[tauri::command]
async fn scan_smb_shares(host: String) -> Result<Vec<SMBShare>, String> {
    smb::scan_shares(&host).await.map_err(|e| e.to_string())
}

#[tauri::command]
async fn browse_smb_share(
    host: String,
    share: String,
    path: Option<String>,
) -> Result<Vec<smb::FileEntry>, String> {
    smb::browse_share(&host, &share, path.as_deref())
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn test_smb_connection(
    host: String,
    share: String,
    username: String,
    password: String,
    domain: Option<String>,
) -> Result<bool, String> {
    smb::test_connection(&host, &share, &username, &password, domain.as_deref())
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn load_configuration(file_path: String) -> Result<Configuration, String> {
    use std::fs;

    let content =
        fs::read_to_string(&file_path).map_err(|e| format!("Failed to read file: {}", e))?;

    let config: Configuration =
        serde_json::from_str(&content).map_err(|e| format!("Failed to parse JSON: {}", e))?;

    Ok(config)
}

#[tauri::command]
async fn save_configuration(file_path: String, config: Configuration) -> Result<(), String> {
    use std::fs;

    let content = serde_json::to_string_pretty(&config)
        .map_err(|e| format!("Failed to serialize configuration: {}", e))?;

    fs::write(&file_path, content).map_err(|e| format!("Failed to write file: {}", e))?;

    Ok(())
}

#[tauri::command]
async fn test_ftp_connection(
    host: String,
    port: u16,
    username: String,
    password: String,
    path: Option<String>,
) -> Result<bool, String> {
    ftp::test_connection(&host, port, &username, &password, path.as_deref())
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn test_nfs_connection(
    host: String,
    path: String,
    mount_point: String,
    options: Option<String>,
) -> Result<bool, String> {
    nfs::test_connection(&host, &path, &mount_point, options.as_deref())
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn test_webdav_connection(
    url: String,
    username: String,
    password: String,
    path: Option<String>,
) -> Result<bool, String> {
    webdav::test_connection(&url, &username, &password, path.as_deref())
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn test_local_connection(base_path: String) -> Result<bool, String> {
    local::test_connection(&base_path)
        .await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn get_default_config_path() -> Result<String, String> {
    use std::env;
    use std::path::PathBuf;

    let home_dir = env::var("HOME")
        .or_else(|_| env::var("USERPROFILE"))
        .map_err(|_| "Unable to determine home directory")?;

    let mut path = PathBuf::from(home_dir);
    path.push(".catalogizer");
    path.push("config.json");

    Ok(path.to_string_lossy().to_string())
}

fn main() {
    // Initialize basic logging to stderr
    env_logger::init();

    let app = tauri::Builder::default()
        .plugin(tauri_plugin_shell::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_fs::init())
        .invoke_handler(tauri::generate_handler![
            scan_network,
            scan_smb_shares,
            browse_smb_share,
            test_smb_connection,
            test_ftp_connection,
            test_nfs_connection,
            test_webdav_connection,
            test_local_connection,
            load_configuration,
            save_configuration,
            get_default_config_path
        ])
        .run(tauri::generate_context!());

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

    /// Tests for NetworkHost struct
    mod network_host_tests {
        use super::*;

        #[test]
        fn test_network_host_serialization() {
            let host = NetworkHost {
                ip: "192.168.1.100".to_string(),
                hostname: Some("fileserver".to_string()),
                mac_address: Some("AA:BB:CC:DD:EE:FF".to_string()),
                vendor: None,
                open_ports: vec![22, 80, 445],
                smb_shares: vec!["shared".to_string(), "media".to_string()],
            };

            let json = serde_json::to_string(&host).unwrap();
            let deserialized: NetworkHost = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.ip, "192.168.1.100");
            assert_eq!(deserialized.hostname, Some("fileserver".to_string()));
            assert_eq!(
                deserialized.mac_address,
                Some("AA:BB:CC:DD:EE:FF".to_string())
            );
            assert!(deserialized.vendor.is_none());
            assert_eq!(deserialized.open_ports, vec![22, 80, 445]);
            assert_eq!(deserialized.smb_shares.len(), 2);
        }

        #[test]
        fn test_network_host_with_no_optional_fields() {
            let host = NetworkHost {
                ip: "10.0.0.1".to_string(),
                hostname: None,
                mac_address: None,
                vendor: None,
                open_ports: vec![],
                smb_shares: vec![],
            };

            let json = serde_json::to_string(&host).unwrap();
            assert!(json.contains("\"ip\":\"10.0.0.1\""));
            assert!(json.contains("\"hostname\":null"));
        }

        #[test]
        fn test_network_host_deserialization_from_json() {
            let json = r#"{
                "ip": "172.16.0.1",
                "hostname": "nas",
                "mac_address": null,
                "vendor": "ExampleNAS",
                "open_ports": [80, 443, 5000],
                "smb_shares": ["homes", "video"]
            }"#;

            let host: NetworkHost = serde_json::from_str(json).unwrap();
            assert_eq!(host.ip, "172.16.0.1");
            assert_eq!(host.hostname, Some("nas".to_string()));
            assert!(host.mac_address.is_none());
            assert_eq!(host.vendor, Some("ExampleNAS".to_string()));
            assert_eq!(host.open_ports.len(), 3);
        }

        #[test]
        fn test_network_host_debug_trait() {
            let host = NetworkHost {
                ip: "1.2.3.4".to_string(),
                hostname: None,
                mac_address: None,
                vendor: None,
                open_ports: vec![],
                smb_shares: vec![],
            };
            let debug = format!("{:?}", host);
            assert!(debug.contains("NetworkHost"));
            assert!(debug.contains("1.2.3.4"));
        }
    }

    /// Tests for SMBShare struct
    mod smb_share_tests {
        use super::*;

        #[test]
        fn test_smb_share_serialization() {
            let share = SMBShare {
                host: "192.168.1.50".to_string(),
                share_name: "media".to_string(),
                path: "\\\\192.168.1.50\\media".to_string(),
                writable: true,
                description: Some("Media files".to_string()),
            };

            let json = serde_json::to_string(&share).unwrap();
            let deserialized: SMBShare = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.host, "192.168.1.50");
            assert_eq!(deserialized.share_name, "media");
            assert!(deserialized.writable);
            assert_eq!(deserialized.description, Some("Media files".to_string()));
        }

        #[test]
        fn test_smb_share_without_description() {
            let share = SMBShare {
                host: "10.0.0.5".to_string(),
                share_name: "data".to_string(),
                path: "\\\\10.0.0.5\\data".to_string(),
                writable: false,
                description: None,
            };

            let json = serde_json::to_string(&share).unwrap();
            assert!(json.contains("\"description\":null"));
        }
    }

    /// Tests for ConfigurationSource struct
    mod configuration_source_tests {
        use super::*;

        #[test]
        fn test_configuration_source_serialization() {
            let source = ConfigurationSource {
                r#type: "smb".to_string(),
                url: "smb://192.168.1.100/media".to_string(),
                access: "my-access".to_string(),
            };

            let json = serde_json::to_string(&source).unwrap();
            let deserialized: ConfigurationSource = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.r#type, "smb");
            assert_eq!(deserialized.url, "smb://192.168.1.100/media");
            assert_eq!(deserialized.access, "my-access");
        }

        #[test]
        fn test_configuration_source_type_field() {
            // Test that the `type` reserved keyword is handled correctly via r#type
            let json = r#"{"type": "ftp", "url": "ftp://example.com", "access": "ftp-creds"}"#;
            let source: ConfigurationSource = serde_json::from_str(json).unwrap();
            assert_eq!(source.r#type, "ftp");
        }
    }

    /// Tests for ConfigurationAccess struct
    mod configuration_access_tests {
        use super::*;

        #[test]
        fn test_configuration_access_serialization() {
            let access = ConfigurationAccess {
                name: "NAS Credentials".to_string(),
                r#type: "smb".to_string(),
                account: "admin".to_string(),
                secret: "password123".to_string(),
            };

            let json = serde_json::to_string(&access).unwrap();
            let deserialized: ConfigurationAccess = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.name, "NAS Credentials");
            assert_eq!(deserialized.r#type, "smb");
            assert_eq!(deserialized.account, "admin");
            assert_eq!(deserialized.secret, "password123");
        }

        #[test]
        fn test_configuration_access_type_field() {
            let json = r#"{"name": "test", "type": "ftp", "account": "user", "secret": "pass"}"#;
            let access: ConfigurationAccess = serde_json::from_str(json).unwrap();
            assert_eq!(access.r#type, "ftp");
        }
    }

    /// Tests for Configuration struct
    mod configuration_tests {
        use super::*;

        #[test]
        fn test_configuration_serialization_roundtrip() {
            let config = Configuration {
                accesses: vec![ConfigurationAccess {
                    name: "NAS".to_string(),
                    r#type: "smb".to_string(),
                    account: "admin".to_string(),
                    secret: "pass".to_string(),
                }],
                sources: vec![ConfigurationSource {
                    r#type: "smb".to_string(),
                    url: "smb://nas/media".to_string(),
                    access: "NAS".to_string(),
                }],
            };

            let json = serde_json::to_string_pretty(&config).unwrap();
            let deserialized: Configuration = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.accesses.len(), 1);
            assert_eq!(deserialized.sources.len(), 1);
            assert_eq!(deserialized.accesses[0].name, "NAS");
            assert_eq!(deserialized.sources[0].url, "smb://nas/media");
        }

        #[test]
        fn test_empty_configuration() {
            let config = Configuration {
                accesses: vec![],
                sources: vec![],
            };

            let json = serde_json::to_string(&config).unwrap();
            let deserialized: Configuration = serde_json::from_str(&json).unwrap();

            assert!(deserialized.accesses.is_empty());
            assert!(deserialized.sources.is_empty());
        }

        #[test]
        fn test_configuration_with_multiple_entries() {
            let config = Configuration {
                accesses: vec![
                    ConfigurationAccess {
                        name: "NAS1".to_string(),
                        r#type: "smb".to_string(),
                        account: "admin".to_string(),
                        secret: "pass1".to_string(),
                    },
                    ConfigurationAccess {
                        name: "FTP Server".to_string(),
                        r#type: "ftp".to_string(),
                        account: "ftpuser".to_string(),
                        secret: "ftppass".to_string(),
                    },
                ],
                sources: vec![
                    ConfigurationSource {
                        r#type: "smb".to_string(),
                        url: "smb://nas1/media".to_string(),
                        access: "NAS1".to_string(),
                    },
                    ConfigurationSource {
                        r#type: "ftp".to_string(),
                        url: "ftp://ftp.example.com/files".to_string(),
                        access: "FTP Server".to_string(),
                    },
                ],
            };

            let json = serde_json::to_string(&config).unwrap();
            let deserialized: Configuration = serde_json::from_str(&json).unwrap();

            assert_eq!(deserialized.accesses.len(), 2);
            assert_eq!(deserialized.sources.len(), 2);
        }

        #[test]
        fn test_configuration_json_deserialization() {
            let json = r#"{
                "accesses": [
                    {"name": "test", "type": "smb", "account": "user", "secret": "pass"}
                ],
                "sources": [
                    {"type": "smb", "url": "smb://host/share", "access": "test"}
                ]
            }"#;

            let config: Configuration = serde_json::from_str(json).unwrap();
            assert_eq!(config.accesses.len(), 1);
            assert_eq!(config.sources.len(), 1);
        }
    }

    /// Tests for local connection validation
    mod local_connection_tests {
        #[tokio::test]
        async fn test_local_connection_with_nonexistent_path() {
            let result = super::test_local_connection("/nonexistent/path/12345".to_string()).await;
            assert!(result.is_err());
            assert!(result.unwrap_err().contains("does not exist"));
        }

        #[tokio::test]
        async fn test_local_connection_with_valid_directory() {
            let result = super::test_local_connection("/tmp".to_string()).await;
            assert!(result.is_ok());
            assert!(result.unwrap());
        }
    }

    /// Tests for default config path
    mod config_path_tests {
        #[tokio::test]
        async fn test_get_default_config_path_returns_path() {
            let result = super::get_default_config_path().await;
            assert!(result.is_ok());
            let path = result.unwrap();
            assert!(path.contains(".catalogizer"));
            assert!(path.contains("config.json"));
        }

        #[tokio::test]
        async fn test_get_default_config_path_format() {
            let result = super::get_default_config_path().await.unwrap();
            // Should end with the expected filename
            assert!(result.ends_with("config.json"));
        }
    }

    /// Tests for load_configuration and save_configuration
    mod configuration_io_tests {
        use super::*;

        #[tokio::test]
        async fn test_load_configuration_nonexistent_file() {
            let result =
                super::load_configuration("/tmp/nonexistent_catalogizer_test_12345.json".to_string())
                    .await;
            assert!(result.is_err());
            let err_msg = result.unwrap_err();
            assert!(
                err_msg.contains("Failed to read file"),
                "Expected read error, got: {}",
                err_msg
            );
        }

        #[tokio::test]
        async fn test_load_configuration_invalid_json() {
            let test_path = "/tmp/catalogizer_test_invalid_json.json";
            std::fs::write(test_path, "not valid json {{{").expect("Should write test file");

            let result = super::load_configuration(test_path.to_string()).await;
            assert!(result.is_err());
            let err_msg = result.unwrap_err();
            assert!(
                err_msg.contains("Failed to parse JSON"),
                "Expected parse error, got: {}",
                err_msg
            );

            let _ = std::fs::remove_file(test_path);
        }

        #[tokio::test]
        async fn test_save_and_load_configuration_roundtrip() {
            let test_path = "/tmp/catalogizer_test_save_load.json";

            let config = Configuration {
                accesses: vec![ConfigurationAccess {
                    name: "Test NAS".to_string(),
                    r#type: "smb".to_string(),
                    account: "admin".to_string(),
                    secret: "password".to_string(),
                }],
                sources: vec![ConfigurationSource {
                    r#type: "smb".to_string(),
                    url: "smb://nas/media".to_string(),
                    access: "Test NAS".to_string(),
                }],
            };

            // Save
            let save_result =
                super::save_configuration(test_path.to_string(), config).await;
            assert!(save_result.is_ok());

            // Load
            let load_result = super::load_configuration(test_path.to_string()).await;
            assert!(load_result.is_ok());
            let loaded = load_result.unwrap();
            assert_eq!(loaded.accesses.len(), 1);
            assert_eq!(loaded.sources.len(), 1);
            assert_eq!(loaded.accesses[0].name, "Test NAS");
            assert_eq!(loaded.sources[0].url, "smb://nas/media");

            let _ = std::fs::remove_file(test_path);
        }

        #[tokio::test]
        async fn test_save_configuration_empty() {
            let test_path = "/tmp/catalogizer_test_save_empty.json";

            let config = Configuration {
                accesses: vec![],
                sources: vec![],
            };

            let save_result =
                super::save_configuration(test_path.to_string(), config).await;
            assert!(save_result.is_ok());

            // Verify the file content
            let content = std::fs::read_to_string(test_path).expect("Should read file");
            assert!(content.contains("\"accesses\""));
            assert!(content.contains("\"sources\""));

            let _ = std::fs::remove_file(test_path);
        }

        #[tokio::test]
        async fn test_save_configuration_invalid_path() {
            let config = Configuration {
                accesses: vec![],
                sources: vec![],
            };

            let result = super::save_configuration(
                "/nonexistent_dir_12345/config.json".to_string(),
                config,
            )
            .await;
            assert!(result.is_err());
            let err_msg = result.unwrap_err();
            assert!(
                err_msg.contains("Failed to write file"),
                "Expected write error, got: {}",
                err_msg
            );
        }
    }

    /// Tests for configuration with multiple protocols
    mod multi_protocol_tests {
        use super::*;

        #[test]
        fn test_configuration_all_protocol_types() {
            let config = Configuration {
                accesses: vec![
                    ConfigurationAccess {
                        name: "SMB Access".to_string(),
                        r#type: "smb".to_string(),
                        account: "admin".to_string(),
                        secret: "pass".to_string(),
                    },
                    ConfigurationAccess {
                        name: "FTP Access".to_string(),
                        r#type: "ftp".to_string(),
                        account: "ftpuser".to_string(),
                        secret: "ftppass".to_string(),
                    },
                    ConfigurationAccess {
                        name: "WebDAV Access".to_string(),
                        r#type: "webdav".to_string(),
                        account: "davuser".to_string(),
                        secret: "davpass".to_string(),
                    },
                    ConfigurationAccess {
                        name: "NFS Access".to_string(),
                        r#type: "nfs".to_string(),
                        account: "".to_string(),
                        secret: "".to_string(),
                    },
                ],
                sources: vec![
                    ConfigurationSource {
                        r#type: "smb".to_string(),
                        url: "smb://nas/media".to_string(),
                        access: "SMB Access".to_string(),
                    },
                    ConfigurationSource {
                        r#type: "ftp".to_string(),
                        url: "ftp://ftp.example.com/files".to_string(),
                        access: "FTP Access".to_string(),
                    },
                    ConfigurationSource {
                        r#type: "webdav".to_string(),
                        url: "https://dav.example.com/media".to_string(),
                        access: "WebDAV Access".to_string(),
                    },
                    ConfigurationSource {
                        r#type: "local".to_string(),
                        url: "/mnt/media".to_string(),
                        access: "".to_string(),
                    },
                ],
            };

            assert_eq!(config.accesses.len(), 4);
            assert_eq!(config.sources.len(), 4);

            let json = serde_json::to_string_pretty(&config).expect("Should serialize");
            let deserialized: Configuration =
                serde_json::from_str(&json).expect("Should deserialize");
            assert_eq!(deserialized.accesses.len(), 4);
            assert_eq!(deserialized.sources.len(), 4);
        }

        #[test]
        fn test_configuration_source_access_linkage() {
            let config = Configuration {
                accesses: vec![ConfigurationAccess {
                    name: "MyNAS".to_string(),
                    r#type: "smb".to_string(),
                    account: "admin".to_string(),
                    secret: "pass".to_string(),
                }],
                sources: vec![ConfigurationSource {
                    r#type: "smb".to_string(),
                    url: "smb://nas/videos".to_string(),
                    access: "MyNAS".to_string(),
                }],
            };

            // Verify the source references the access by name
            let source = &config.sources[0];
            let matching_access = config
                .accesses
                .iter()
                .find(|a| a.name == source.access);
            assert!(matching_access.is_some());
            assert_eq!(matching_access.expect("Should find access").r#type, "smb");
        }
    }

    /// Tests for NetworkHost edge cases
    mod network_host_edge_case_tests {
        use super::*;

        #[test]
        fn test_network_host_ipv6_address() {
            let host = NetworkHost {
                ip: "::1".to_string(),
                hostname: Some("localhost".to_string()),
                mac_address: None,
                vendor: None,
                open_ports: vec![],
                smb_shares: vec![],
            };

            let json = serde_json::to_string(&host).expect("Should serialize");
            let deserialized: NetworkHost =
                serde_json::from_str(&json).expect("Should deserialize");
            assert_eq!(deserialized.ip, "::1");
        }

        #[test]
        fn test_network_host_many_ports() {
            let host = NetworkHost {
                ip: "10.0.0.1".to_string(),
                hostname: None,
                mac_address: None,
                vendor: None,
                open_ports: (1..=100).collect(),
                smb_shares: vec![],
            };

            assert_eq!(host.open_ports.len(), 100);
            assert_eq!(host.open_ports[0], 1);
            assert_eq!(host.open_ports[99], 100);
        }

        #[test]
        fn test_network_host_many_shares() {
            let shares: Vec<String> = (0..20).map(|i| format!("share_{}", i)).collect();
            let host = NetworkHost {
                ip: "10.0.0.1".to_string(),
                hostname: None,
                mac_address: None,
                vendor: None,
                open_ports: vec![445],
                smb_shares: shares,
            };

            assert_eq!(host.smb_shares.len(), 20);
            assert_eq!(host.smb_shares[0], "share_0");
            assert_eq!(host.smb_shares[19], "share_19");
        }

        #[test]
        fn test_network_host_special_chars_in_hostname() {
            let host = NetworkHost {
                ip: "192.168.1.1".to_string(),
                hostname: Some("my-nas.local".to_string()),
                mac_address: None,
                vendor: None,
                open_ports: vec![],
                smb_shares: vec![],
            };

            let json = serde_json::to_string(&host).expect("Should serialize");
            let deserialized: NetworkHost =
                serde_json::from_str(&json).expect("Should deserialize");
            assert_eq!(
                deserialized.hostname,
                Some("my-nas.local".to_string())
            );
        }
    }
}
