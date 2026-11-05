//! SSRF guard for `make_http_request` (TAURI-FU-5): compares PARSED URLs
//! instead of a string prefix, so `http://a.com.evil` and `http://a.com@evil`
//! cannot pass for `http://a.com`.

use url::Url;

/// True only when `request` targets the configured `server` origin
/// (scheme, host case-insensitively, effective port), carries no userinfo,
/// and its path is the server path or a path-boundary-aware descendant of it.
pub fn is_within_server(server: &str, request: &str) -> bool {
    let (Ok(server), Ok(request)) = (Url::parse(server), Url::parse(request)) else {
        return false;
    };
    if !matches!(server.scheme(), "http" | "https") || server.scheme() != request.scheme() {
        return false;
    }
    // Reject userinfo in the request: `http://a.com@evil` is a host trick.
    if !request.username().is_empty() || request.password().is_some() {
        return false;
    }
    // `Url::host()` is lowercased/IDNA-normalised, so this is case-insensitive.
    match (server.host(), request.host()) {
        (Some(s), Some(r)) if s == r => {}
        _ => return false,
    }
    if server.port_or_known_default() != request.port_or_known_default() {
        return false;
    }
    let base = server.path().trim_end_matches('/');
    let path = request.path();
    base.is_empty() || path == base || path.strip_prefix(base).is_some_and(|rest| rest.starts_with('/'))
}

#[cfg(test)]
mod tests {
    use super::is_within_server;

    #[test]
    fn legit_cases_allowed() {
        let s = "http://localhost:8080";
        assert!(is_within_server(s, "http://localhost:8080"));
        assert!(is_within_server(s, "http://localhost:8080/"));
        assert!(is_within_server(s, "http://localhost:8080/api/v1/media?x=1#f"));
        assert!(is_within_server("http://localhost:8080/", "http://localhost:8080/api"));
        assert!(is_within_server("HTTP://LocalHost:8080", "http://localhost:8080/a"));
        assert!(is_within_server("http://a.com", "http://A.COM/x"));
        assert!(is_within_server("http://a.com", "http://a.com:80/x"));
        assert!(is_within_server("https://a.com", "https://a.com:443/x"));
        assert!(is_within_server("http://a.com/api", "http://a.com/api"));
        assert!(is_within_server("http://a.com/api/", "http://a.com/api/v1"));
        assert!(is_within_server("http://[::1]:8080", "http://[::1]:8080/x"));
    }

    #[test]
    fn host_suffix_bypass_rejected() {
        assert!(!is_within_server("http://a.com", "http://a.com.evil"));
        assert!(!is_within_server("http://a.com", "http://a.com.evil/x"));
        assert!(!is_within_server("http://a.com", "http://a.com.evil.com/steal"));
        assert!(!is_within_server("http://localhost:8080", "http://localhost:80801/x"));
    }

    #[test]
    fn userinfo_bypass_rejected() {
        assert!(!is_within_server("http://a.com", "http://a.com@evil"));
        assert!(!is_within_server("http://a.com", "http://a.com@evil.com/x"));
        assert!(!is_within_server("http://a.com", "http://user:pw@a.com/x"));
        assert!(!is_within_server("http://a.com", "http://a.com:80@evil/x"));
        // WHATWG parsing (also what reqwest sends) turns `\` into `/`: the real
        // target is host a.com, path /@evil/x, so this is in-scope, not a bypass.
        assert!(is_within_server("http://a.com", "http://a.com\\@evil/x"));
    }

    #[test]
    fn origin_mismatch_rejected() {
        assert!(!is_within_server("http://localhost:8080", "http://localhost:9090/x"));
        assert!(!is_within_server("https://localhost:8080", "http://localhost:8080/x"));
        assert!(!is_within_server("http://a.com", "https://a.com/x"));
        assert!(!is_within_server("http://a.com", "http://evil.com/x"));
        assert!(!is_within_server("http://a.com", "ftp://a.com/x"));
        assert!(!is_within_server("http://a.com", "http://a.com:81/x"));
    }

    #[test]
    fn path_boundary_enforced() {
        assert!(!is_within_server("http://a.com/api", "http://a.com/apix"));
        assert!(!is_within_server("http://a.com/api", "http://a.com/api2/x"));
        assert!(!is_within_server("http://a.com/api", "http://a.com/other"));
        assert!(!is_within_server("http://a.com/api", "http://a.com/api/../admin"));
        assert!(!is_within_server("http://a.com/api", "http://a.com/api/%2e%2e/admin"));
    }

    #[test]
    fn unparseable_rejected() {
        assert!(!is_within_server("not a url", "not a url"));
        assert!(!is_within_server("http://a.com", ""));
        assert!(!is_within_server("http://a.com", "//a.com/x"));
        assert!(!is_within_server("", "http://a.com"));
    }
}
