//! Backend-enforced user confirmation for server-URL changes.
//!
//! Trust model: the webview frontend may be compromised, so it must not be able
//! to repoint the app (and with it every authenticated request) at another
//! server on its own. Every path that would change the effective server URL
//! (`set_server_url`, `update_config`) goes through [`confirm_url_change`], which
//! blocks on a NATIVE dialog the frontend cannot click. A declined or failed
//! prompt leaves the configuration untouched.

use std::sync::Arc;

/// Asks the user (out of the webview's reach) whether to adopt a server URL.
/// Blocking: callers run it on a blocking thread.
pub trait UrlConfirmer: Send + Sync {
    /// `current` is the URL in force (None on first-time setup); `requested`
    /// is the already-validated URL that would be applied.
    fn confirm_server_url_change(&self, current: Option<&str>, requested: &str) -> bool;
}

pub type SharedConfirmer = Arc<dyn UrlConfirmer>;

/// Text shown in the native dialog (kept pure so it is unit-tested).
pub fn prompt_message(current: Option<&str>, requested: &str) -> String {
    match current {
        None => format!(
            "Catalogizer wants to connect to this server:\n\n{requested}\n\nOnly continue if you recognise this address."
        ),
        Some(cur) => format!(
            "Catalogizer wants to change its server.\n\nCurrent: {cur}\nNew: {requested}\n\nAll requests, including your sign-in, would go to the new server. Only continue if you started this change."
        ),
    }
}

/// Production confirmer: a native message dialog (tauri-plugin-dialog).
pub struct DialogConfirmer(pub tauri::AppHandle);

impl UrlConfirmer for DialogConfirmer {
    fn confirm_server_url_change(&self, current: Option<&str>, requested: &str) -> bool {
        use tauri_plugin_dialog::{DialogExt, MessageDialogButtons, MessageDialogKind};
        self.0
            .dialog()
            .message(prompt_message(current, requested))
            .title("Confirm server change")
            .kind(MessageDialogKind::Warning)
            .buttons(MessageDialogButtons::OkCancelCustom(
                "Change server".to_string(),
                "Cancel".to_string(),
            ))
            .blocking_show()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn message_names_both_urls_for_a_change() {
        let m = prompt_message(Some("https://old.example"), "https://new.example");
        assert!(m.contains("https://old.example") && m.contains("https://new.example"));
    }

    #[test]
    fn message_for_first_time_names_only_requested() {
        let m = prompt_message(None, "https://new.example");
        assert!(m.contains("https://new.example") && !m.contains("Current:"));
    }
}
