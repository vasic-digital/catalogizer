//! Backend-enforced user confirmation for server-URL changes.
//!
//! Trust model: the webview frontend may be compromised, so it must not be able
//! to repoint the app (and with it every authenticated request) at another
//! server on its own. Every path that would change the effective server URL
//! (`set_server_url`, `update_config`) goes through [`confirm_url_change`], which
//! blocks on a NATIVE dialog the frontend cannot click. A declined or failed
//! prompt leaves the configuration untouched.

use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;

/// Asks the user (out of the webview's reach) whether to adopt a server URL.
/// Blocking: callers run it on a blocking thread.
pub trait UrlConfirmer: Send + Sync {
    /// `current` is the URL in force (None on first-time setup); `requested`
    /// is the already-validated URL that would be applied.
    fn confirm_server_url_change(&self, current: Option<&str>, requested: &str) -> bool;
}

/// Serialises prompts: at most ONE native dialog is in flight; a second request
/// while one is open is refused (a compromised frontend cannot flood the user).
/// A panicking/failed confirmer fails closed and releases the gate.
pub struct PromptGate {
    inner: Arc<dyn UrlConfirmer>,
    busy: AtomicBool,
}

pub type SharedConfirmer = Arc<PromptGate>;

struct BusyGuard<'a>(&'a AtomicBool);
impl Drop for BusyGuard<'_> {
    fn drop(&mut self) {
        self.0.store(false, Ordering::SeqCst);
    }
}

impl PromptGate {
    pub fn new(inner: Arc<dyn UrlConfirmer>) -> SharedConfirmer {
        Arc::new(Self { inner, busy: AtomicBool::new(false) })
    }

    /// Blocking. Err when another prompt is already open.
    pub fn ask(&self, current: Option<&str>, requested: &str) -> Result<bool, String> {
        if self
            .busy
            .compare_exchange(false, true, Ordering::SeqCst, Ordering::SeqCst)
            .is_err()
        {
            return Err("Another server confirmation is already open".to_string());
        }
        let _guard = BusyGuard(&self.busy);
        Ok(self.inner.confirm_server_url_change(current, requested))
    }
}

const MAX_SHOWN: usize = 120;

fn shorten(s: &str) -> String {
    if s.chars().count() <= MAX_SHOWN {
        s.to_string()
    } else {
        format!("{}... (shortened)", s.chars().take(MAX_SHOWN).collect::<String>())
    }
}

fn host_line(url: &str) -> String {
    match url::Url::parse(url).ok().and_then(|u| u.host_str().map(String::from)) {
        Some(h) => format!("Host: {h}"),
        None => "Host: (unreadable)".to_string(),
    }
}

/// Text shown in the native dialog (kept pure so it is unit-tested). Both URLs
/// are normalised (punycode hosts, no control characters) by `validate_server_url`
/// before they get here; the host is shown on its own line and long URLs shortened.
/// `current` is the URL in force, or the last user-confirmed one, or None.
pub fn prompt_message(current: Option<&str>, requested: &str) -> String {
    let host = host_line(requested);
    let req = shorten(requested);
    match current {
        None => format!(
            "Catalogizer wants to connect to this server:\n\n{host}\n{req}\n\nOnly continue if you recognise this address."
        ),
        Some(cur) => format!(
            "Catalogizer wants to change its server.\n\nPrevious: {}\n\nNew: {req}\n{host}\n\nAll requests, including your sign-in, would go to the new server. Only continue if you started this change.",
            shorten(cur)
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
    fn message_shows_punycode_host_line_and_shortens_long_urls() {
        let m = prompt_message(None, "https://xn--ctalogizer-zqi.example/");
        assert!(m.contains("Host: xn--ctalogizer-zqi.example"));
        let long = format!("https://a.com/{}", "p".repeat(500));
        let m = prompt_message(Some("https://old.com/"), &long);
        assert!(m.contains("(shortened)") && m.len() < 700, "{}", m.len());
        assert!(!m.contains(&"p".repeat(200)));
    }

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
