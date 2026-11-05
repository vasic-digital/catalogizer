//! SSRF guard for `make_http_request` (TAURI-FU-5): compares PARSED URLs
//! instead of a string prefix, so `http://a.com.evil` and `http://a.com@evil`
//! cannot pass for `http://a.com`.

use url::Url;

/// Validate a server URL at set time: parseable, http/https, host, no userinfo.
pub fn validate_server_url(raw: &str) -> Result<String, String> {
    let u = Url::parse(raw).map_err(|e| format!("Invalid server URL: {}", e))?;
    if !matches!(u.scheme(), "http" | "https") {
        return Err("Server URL must use http or https".to_string());
    }
    if u.host().is_none() {
        return Err("Server URL must have a host".to_string());
    }
    if !u.username().is_empty() || u.password().is_some() {
        return Err("Server URL must not contain credentials".to_string());
    }
    let lp = u.path().to_ascii_lowercase();
    if lp.contains("%2f") || lp.contains("%5c") || lp.contains("%2e") {
        return Err("Server URL path must not contain encoded separators".to_string());
    }
    Ok(raw.to_string())
}

/// HTTP client that re-checks every redirect hop against the configured server.
pub fn build_client(server: &str) -> Result<reqwest::Client, String> {
    let server = server.to_string();
    let policy = reqwest::redirect::Policy::custom(move |attempt| {
        if attempt.previous().len() > 10 {
            attempt.error("too many redirects")
        } else if is_within_server(&server, attempt.url().as_str()) {
            attempt.follow()
        } else {
            attempt.error("redirect target is outside the configured server")
        }
    });
    reqwest::Client::builder()
        .redirect(policy)
        .build()
        .map_err(|e| format!("Failed to build client: {}", e))
}

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
    // Encoded separators / dots can smuggle traversal past the path check.
    let lp = request.path().to_ascii_lowercase();
    if lp.contains("%2f") || lp.contains("%5c") || lp.contains("%2e") {
        return false;
    }
    let base = server.path().trim_end_matches('/');
    let path = request.path();
    base.is_empty() || path == base || path.strip_prefix(base).is_some_and(|rest| rest.starts_with('/'))
}

#[cfg(test)]
pub(crate) mod tests {
    use super::{build_client, is_within_server, validate_server_url};

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

    #[test]
    fn userinfo_variants_and_non_http_server_rejected() {
        assert!(!is_within_server("http://a.com", "http://user@a.com/x"));
        assert!(!is_within_server("http://a.com", "http://:pw@a.com/x"));
        assert!(!is_within_server("ftp://a.com", "ftp://a.com/x"));
    }

    #[test]
    fn encoded_separators_rejected() {
        let s = "http://a.com/api";
        assert!(!is_within_server(s, "http://a.com/api/..%2fadmin"));
        assert!(!is_within_server(s, "http://a.com/api/..%2Fadmin"));
        assert!(!is_within_server(s, "http://a.com/api/..%5cadmin"));
        assert!(!is_within_server(s, "http://a.com/api/%2E%2e/admin"));
        // The url crate already resolves %2e dot-segments; any %2e that survives
        // parsing is rejected conservatively (defence in depth).
        assert!(!is_within_server(s, "http://a.com/api/v1/file%2Etxt"));
        assert!(is_within_server(s, "http://a.com/api/v1/a%20b"));
    }

    #[test]
    fn server_url_validation() {
        assert!(validate_server_url("http://localhost:8080").is_ok());
        assert!(validate_server_url("https://a.com/api").is_ok());
        assert!(validate_server_url("ftp://a.com").is_err());
        assert!(validate_server_url("file:///etc/passwd").is_err());
        assert!(validate_server_url("http://u:p@a.com").is_err());
        assert!(validate_server_url("http://u@a.com").is_err());
        assert!(validate_server_url("http://a.com/api%2f..").is_err());
        assert!(validate_server_url("http://a.com/a%5Cb").is_err());
        assert!(validate_server_url("garbage").is_err());
        assert!(validate_server_url("").is_err());
    }

    // LIVE: real sockets, real reqwest client.
    pub(crate) async fn serve(listener: tokio::net::TcpListener, other_port: u16, hits: std::sync::Arc<std::sync::atomic::AtomicUsize>) {
        use tokio::io::{AsyncReadExt, AsyncWriteExt};
        loop {
            let Ok((mut sock, _)) = listener.accept().await else { return };
            let hits = hits.clone();
            tokio::spawn(async move {
                let mut buf = [0u8; 2048];
                let n = sock.read(&mut buf).await.unwrap_or(0);
                let req = String::from_utf8_lossy(&buf[..n]).to_string();
                let path = req.split_whitespace().nth(1).unwrap_or("/").to_string();
                hits.fetch_add(1, std::sync::atomic::Ordering::SeqCst);
                let resp = match path.as_str() {
                    p if p.starts_with("/chain/") => {
                        let n: u32 = p[7..].parse().unwrap_or(0);
                        if n == 0 {
                            "HTTP/1.1 200 OK\r\nContent-Length: 3\r\nConnection: close\r\n\r\nend".to_string()
                        } else {
                            format!("HTTP/1.1 302 Found\r\nLocation: /chain/{}\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", n - 1)
                        }
                    }
                    "/redir-out" => format!("HTTP/1.1 302 Found\r\nLocation: http://127.0.0.1:{}/x\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", other_port),
                    "/redir-in" => "HTTP/1.1 302 Found\r\nLocation: /ok\r\nContent-Length: 0\r\nConnection: close\r\n\r\n".to_string(),
                    _ => "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nok".to_string(),
                };
                let _ = sock.write_all(resp.as_bytes()).await;
            });
        }
    }

    #[tokio::test]
    async fn live_hop_cap_is_exactly_ten() {
        use std::sync::{atomic::AtomicUsize, Arc};
        let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let pa = a.local_addr().unwrap().port();
        tokio::spawn(serve(a, 1, Arc::new(AtomicUsize::new(0))));
        let server = format!("http://127.0.0.1:{}", pa);
        let client = build_client(&server).unwrap();
        let ok = client.get(format!("{}/chain/10", server)).send().await.unwrap();
        assert_eq!(ok.text().await.unwrap(), "end", "10 redirects must succeed");
        assert!(client.get(format!("{}/chain/11", server)).send().await.is_err(), "11 redirects must fail");
    }

    #[tokio::test]
    async fn live_redirect_policy_blocks_out_of_scope_and_allows_in_scope() {
        use std::sync::{atomic::{AtomicUsize, Ordering}, Arc};
        let a = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let b = tokio::net::TcpListener::bind("127.0.0.1:0").await.unwrap();
        let (pa, pb) = (a.local_addr().unwrap().port(), b.local_addr().unwrap().port());
        let hits_a = Arc::new(AtomicUsize::new(0));
        let hits_b = Arc::new(AtomicUsize::new(0));
        tokio::spawn(serve(a, pb, hits_a.clone()));
        tokio::spawn(serve(b, pa, hits_b.clone()));
        let server = format!("http://127.0.0.1:{}", pa);
        let client = build_client(&server).unwrap();

        let r = client.get(format!("{}/redir-out", server)).send().await;
        assert!(r.is_err(), "cross-origin redirect must be refused");
        assert_eq!(hits_b.load(Ordering::SeqCst), 0, "other server must never be contacted");

        let r = client.get(format!("{}/redir-in", server)).send().await.unwrap();
        assert_eq!(r.text().await.unwrap(), "ok");
    }
}
