# 09 - Desktop and Installer Audit Plan (catalogizer-desktop, installer-wizard)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-03 |
| Last modified | 2026-10-03 |
| Status | draft |
| Feature | specs/001-full-project-audit-remediation |
| Spec requirements covered | FR-006, FR-007, FR-008, FR-009, FR-010, FR-011, FR-014, FR-015, FR-016, FR-021, FR-022, FR-025 |
| Success criteria covered | SC-002, SC-003, SC-004, SC-005, SC-007, SC-011, SC-012 |
| Governance anchors | 11.4.108, 11.4.200, 11.4.173, 11.4.246, 11.4.252, 11.4.224, 11.4.244, 11.4.245, 11.4.266, 11.4.201, 11.4.27, 11.4.261, 11.4.3, 11.4.124, 11.4.122, 11.4.263 |
| Companion documents | 05-test-strategy-and-coverage-matrix.md, 06-determinism-and-evidence-framework.md, 07-backend-catalog-api-audit-plan.md |

## Table of contents

1. Scope, inputs and evidence rules
2. Measured inventory of the two applications
3. IPC surface and trust boundaries (with diagrams)
4. Risk-rated audit scope
5. Candidate findings already visible in the code (to be reproduced, not yet confirmed)
6. Rust-specific detector suite (containerized)
7. Security review plan
8. Test plan by type
9. Installer wizard: state machine, protocol testers and real targets
10. Packaging, signing and release verification
11. Shared-contract drift check
12. Frontend plan (React, stores, services, e2e)
13. Performance baselines
14. Documentation required
15. Work packages, sequencing and acceptance evidence
16. Traceability to the spec
17. Decision records, rejected alternatives, owner decisions
18. Risks
Appendix A. POC: Rust SSRF validation table test (NOT EXECUTED)
Appendix B. POC: vitest bridge and route-contract test (NOT EXECUTED)
Appendix C. POC: refused-port and protocol-tester RED tests (NOT EXECUTED)
Appendix D. Container command forms (NOT EXECUTED)

---

## 1. Scope, inputs and evidence rules

### 1.1 What this document plans

This is the technical plan for auditing, fixing, testing and documenting two Tauri 2 applications (Rust backend, React 18 and TypeScript front end):

| Application | Directory | Role |
|---|---|---|
| Catalogizer Desktop | `catalogizer-desktop/` | End-user client of `catalog-api`: login, library, search, media detail, settings; optional embedded VLC player (`vlc-player` Cargo feature) |
| Installation Wizard | `installer-wizard/` | First-run configurator: network scan, protocol selection (SMB, FTP, NFS, WebDAV, local), per-protocol connection tests, configuration file load and save |

Both are in the in-scope list of FR-006 ("desktop", "installer"). This plan does not restate the spec; it states how each requirement is met for these two applications.

### 1.2 How the facts below were obtained

Every statement about code cites a repository-relative path (and line where useful). Facts were gathered on 2026-10-03 by direct reads of the sources and a few read-only host commands (file listing, `grep`, a 3-line `awk` probe). Nothing was built and no test was executed on the host. Items that need execution are written as hypotheses and tagged:

- `CONFIRMED-BY-READ` - visible in the source as quoted; behaviour at runtime still to be demonstrated by a failing test (FR-008).
- `UNCONFIRMED:` - plausible from the code or from framework documentation but not verified here.
- `UNKNOWN:` - information not available to this plan.
- `MEASURED-NOW` - a command was run during this planning session and its output is quoted.

Per constitution 11.4.194(6)(b), counts below are leads; the lines they point to are the findings.

### 1.3 Evidence rules that apply to every work package in this document

1. Builds run only in rootless containers provisioned through the `Containers` submodule, never on the bare host (11.4.173, FR-021). Container-internal uid 0 is a user-namespace mapping of the invoking user; no `sudo`, no host root.
2. A finding is closed only by a test that fails before the fix on the broken artifact and passes after (FR-008, 11.4.115), with a machine-written verdict (06 document).
3. A test of behaviour that depends on a real service, credential or device runs against the real one; if absent it is reported `blocked-unavailable` with the exact reason, counts as not passing, and is never simulated (FR-025). Section 9.5 defines what "real" means for protocol testers.
4. Every number in a final report cites an evidence record from the current work (FR-022, SC-012).

---

## 2. Measured inventory of the two applications

### 2.1 Source size and structure (read from the tree on 2026-10-03)

| Item | catalogizer-desktop | installer-wizard |
|---|---|---|
| Version (package.json, Cargo.toml, tauri.conf.json) | 2.4.0 | 2.4.0 |
| Rust sources | `src-tauri/src/main.rs` (890 lines), `vlc/mod.rs` (1000), `vlc/commands.rs` (826) | `main.rs` (801), `network.rs` (504), `smb.rs` (573), `webdav.rs` (362), `ftp.rs` (227), `nfs.rs` (165), `local.rs` (109) |
| Tauri plugins registered | `tauri-plugin-shell` | `tauri-plugin-shell`, `tauri-plugin-dialog`, `tauri-plugin-fs` |
| Tauri plugins imported by the front end (non-test) | none (`@tauri-apps/plugin-shell` appears only in test mocks: `src/test/setup.ts:12`, `src/test-utils/setup.ts:14`) | `plugin-dialog` (`src/services/tauri.ts:2`); `plugin-fs` and `plugin-shell` only in test mocks (`src/test/setup.ts:14,19`) |
| Capability files | none: `src-tauri/capabilities/` does not exist; `src-tauri/gen/schemas/capabilities.json` is `{}` | none: same |
| CSP | `default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; img-src 'self' data:; connect-src 'self'; font-src 'self' data: https://fonts.gstatic.com` | same without the two Google font origins |
| Front-end tests | 21 files (pages, stores, services, hooks, components, utils, types) | contexts, components, services, utils, types, App |
| E2E | `e2e/theme.spec.ts` (Playwright, Chromium against Vite dev server) | none |
| Cargo.lock | gitignored (`.gitignore:136,150`) | same |
| `rust-version` in Cargo.toml | 1.60 | 1.70 |
| Update mechanism | none (no updater plugin in `Cargo.toml`) | none |
| Signing configuration | none | macOS `signingIdentity: null`; Windows `certificateThumbprint: null`, `timestampUrl: ""` (`installer-wizard/src-tauri/tauri.conf.json`) |
| Bundle targets | `"targets": "all"` | `"targets": "all"` |

### 2.2 Raw token counts in non-test Rust code (MEASURED-NOW, method stated)

Method: for each file, `awk` counting lines matching `unwrap(|expect(|panic!|unsafe` before the first `#[cfg(test)]` line.

| File | Count | Reading |
|---|---|---|
| `catalogizer-desktop/src-tauri/src/main.rs` | 1 | the `expect("Failed to initialize VLC player")` at line 151, compiled only with `vlc-player` |
| `catalogizer-desktop/src-tauri/src/vlc/mod.rs` | 35 | 34 lines contain `unsafe` (FFI to libvlc, `unsafe impl Send/Sync` at lines 88-89), 1 `.unwrap()` at line 445 |
| `catalogizer-desktop/src-tauri/src/vlc/commands.rs` | 0 | |
| `installer-wizard/src-tauri/src/*.rs` | 0 | |

These are leads. The `unwrap()` at `vlc/mod.rs:445` is on a constant string and probably cannot panic, but it contradicts the rule that the existing landmine detector claims to enforce (see 5.4, finding D-04).

### 2.3 Existing claims that the audit must reconcile (claim-vs-reality ledger, 11.4.266)

| Claim | Where | Status |
|---|---|---|
| "RULE-DESK-001: catalogizer-desktop Rust unwrap() clean (non-test code)" | `docs/DESKTOP_PERF_AUDIT.md`, produced by `scripts/detect-landmines.sh:139-165` | False-null instrument, see D-04 |
| Test coverage 93% overall, "Tauri Backend 87%, Integration" | `installer-wizard/STATUS.md`; badges from `scripts/generate-badges.js` / `badges.json` | UNCONFIRMED; no Rust coverage instrument is configured in the repo; the `STATUS.md` "Last Updated" badge contains the literal text `$(date +"%Y-%m-%d")`, i.e. an unexpanded shell substitution |
| "80%+ test coverage" target, "Tauri Command Tests ... `src/__tests__/tauri/`" | `docs/desktop-testing-guide.md` | The directory `src/__tests__/tauri/` does not exist; existing files are `tauriCommands.test.ts` and `tauriCommands.extended.test.ts` |
| "Configuration persistence" | `catalogizer-desktop/CLAUDE.md` (module overview) | No persistence exists: `main.rs` holds `AppConfig` in a `tokio::sync::Mutex` only; no file I/O; the `directories` dependency is unused in Rust sources (MEASURED-NOW: no `directories`/`fs` use in `main.rs`) |
| "Proxied HTTP with SSRF validation" | `catalogizer-desktop/CLAUDE.md`, `main.rs:83-93` | Partially true; see section 7.2 |

Each row becomes a register item (spec FR-001/FR-002) and a ledger row.

---

## 3. IPC surface and trust boundaries

### 3.1 Desktop IPC commands (CONFIRMED-BY-READ, `catalogizer-desktop/src-tauri/src/main.rs:38-141, 158-207`)

| Command | Arguments | Effect | Trust note |
|---|---|---|---|
| `get_config` | - | returns whole `AppConfig` including `auth_token` to the web view | token readable by any script in the web view |
| `update_config` | `AppConfig` | replaces whole config (including `server_url`, `auth_token`) | no validation |
| `set_server_url` | `url: String` | sets the SSRF allow-base | no scheme, host or emptiness validation |
| `set_auth_token` / `clear_auth_token` | `token` | in-memory only | lost on restart |
| `make_http_request` | `url, method, headers, body` | HTTP via `reqwest::Client::new()`; returns body text | see section 7.2 |
| `get_app_version`, `get_platform`, `get_arch` | - | constants | low risk |
| 19 `vlc_*` commands | see `vlc/commands.rs` | libvlc control; compiled only with `--features vlc-player` | `vlc_play(url)` and `vlc_take_snapshot(filepath)` take unvalidated strings |

The handler list is written twice in `main.rs` (lines 158-168 and 176-206); the second `.invoke_handler(...)` replaces the first when the feature is on. A drift between the two lists would silently drop commands in one configuration. A test must assert the two sets agree (section 8.2).

### 3.2 Installer IPC commands (CONFIRMED-BY-READ, `installer-wizard/src-tauri/src/main.rs:54-193`)

| Command | Arguments | Effect | Trust note |
|---|---|---|---|
| `scan_network` | - | per interface assumes a /24, TCP probes, `ping`, `arp`, reverse DNS | active scanning of every attached /24 |
| `scan_smb_shares` / `browse_smb_share` / `test_smb_connection` | host, share, credentials | POSTs to `catalog-api` at `CATALOG_API_URL` (default `http://localhost:8080`) endpoints `/api/v1/smb/discover|browse|test` (`smb.rs:225-227`) | plain HTTP, password in JSON body, no auth header |
| `test_ftp_connection` | host, port, user, password, path | raw `std::net::TcpStream` FTP dialogue | CRLF injection, blocking I/O in async fn |
| `test_nfs_connection` | host, path, mount_point, options | TCP connect to port 2049, then `create_dir_all(mount_point)` | arbitrary directory creation; `path` and `options` ignored |
| `test_webdav_connection` | url, user, password, path | raw HTTP `PROPFIND` over TCP; HTTPS only checks reachability | false positives, see D-12 |
| `test_local_connection` | `base_path` | exists, is_dir, `read_dir` | path-existence oracle |
| `load_configuration` / `save_configuration` | `file_path`, `Configuration` | reads/writes any path the process may access; the file holds `accesses[].secret` in plain text (`main.rs:34-44`) | arbitrary file read/write from the web view |
| `get_default_config_path` | - | `$HOME/.catalogizer/config.json` | |

### 3.3 Diagram: IPC architecture and trust boundaries

```mermaid
flowchart LR
  subgraph WebView["Web view (React 18, CSP: connect-src self)"]
    UI["Pages and wizard steps"]
    ST["Zustand stores / contexts"]
    SV["apiService.ts / services/tauri.ts"]
    UI --> ST --> SV
  end
  subgraph Core["Tauri core (Rust)"]
    IPC["invoke_handler (app commands, no capability file)"]
    CFG["AppConfig in Mutex (memory only)"]
    PROXY["make_http_request (reqwest)"]
    VLC["vlc_* (libvlc FFI, optional feature)"]
    PROBE["protocol testers: smb, ftp, nfs, webdav, local, network scan"]
    FSX["load/save configuration (plain-text secrets)"]
  end
  subgraph Plugins["Plugins"]
    SH["plugin-shell (registered, unused by UI)"]
    DLG["plugin-dialog / plugin-fs (installer)"]
  end
  subgraph Ext["Outside the app"]
    API["catalog-api (/api/v1/...)"]
    LAN["LAN hosts: SMB, FTP, NFS, WebDAV"]
    DISK["Local disk"]
  end
  SV -- "invoke(name,args) TRUST BOUNDARY 1" --> IPC
  IPC --> CFG
  IPC --> PROXY --> API
  IPC --> VLC --> DISK
  IPC --> PROBE --> LAN
  PROBE -- "POST /api/v1/smb/*" --> API
  IPC --> FSX --> DISK
  SV -. "plugin ACL: no capability files" .-> Plugins
```

Trust boundary 1 (web view to Rust) is the one the audit treats as hostile: any script executing in the web view (XSS in a rendered media title or overview, a compromised npm dependency in the bundle) can call every command above with arbitrary arguments. The CSP (`script-src 'self'`) reduces but does not remove this; the Rust side MUST NOT rely on the web view being honest.

### 3.4 Diagram: proxied request with SSRF validation (current behaviour and target behaviour)

```mermaid
sequenceDiagram
  participant W as Web view (apiService.ts)
  participant R as Rust make_http_request
  participant C as AppConfig (Mutex)
  participant S as Configured server
  participant X as Any other host
  W->>R: invoke make_http_request(url, method, headers, body)
  R->>C: lock, read server_url
  alt current check (main.rs:88-93)
    R->>R: url.trim_end_matches("/").starts_with(server_url)
    Note over R: string prefix only; scheme/host/port/userinfo not parsed
  else target check (Appendix A)
    R->>R: parse both URLs, compare scheme, host, port, path segment prefix; reject userinfo
  end
  R->>S: reqwest::Client::new() send (follows up to 10 redirects, no timeout)
  S-->>R: 302 Location: http://169.254.169.254/ (hypothesis H-REDIR)
  R->>X: follows redirect (current behaviour)
  X-->>R: body
  R-->>W: body text, status code discarded
  Note over R,W: target: redirect policy none or revalidating, timeout, size cap, non-2xx returned as error with status
```

---

## 4. Risk-rated audit scope

Rating scale (consistent with the register design in 04): Critical, High, Medium, Low, by blast radius and likelihood. "Depth" is the audit effort class: D3 full line-by-line plus dynamic tests, D2 targeted review plus tests, D1 detector sweep only.

| ID | Area | Files | Rating | Depth | Rationale |
|---|---|---|---|---|---|
| R-01 | `make_http_request` and SSRF check | `catalogizer-desktop/src-tauri/src/main.rs:75-126` | Critical | D3 | Only network egress of the desktop app; a prefix-string check on user-influenced input; redirect, timeout and size not handled |
| R-02 | Capability and permission model | both `tauri.conf.json`, absence of `capabilities/` | High | D3 | Plugins registered with no ACL; dialog/fs likely denied (functional) and, if later broadened carelessly, over-permissive (security) |
| R-03 | Installer file commands | `installer-wizard/src-tauri/src/main.rs:88-111` | High | D3 | Arbitrary-path read/write by IPC; plain-text secrets at rest |
| R-04 | FTP and WebDAV hand-written clients | `ftp.rs`, `webdav.rs` | High | D3 | Hand-rolled protocol code, CRLF injection, substring status parsing, cleartext credentials |
| R-05 | Network scan | `network.rs` | High | D3 | Active scanning, defects in liveness and port logic, hard-coded stub for share names |
| R-06 | Auth token handling | `authStore.ts`, `main.rs` config | High | D2 | Token exposed to web view via `get_config`, no persistence, no expiry handling |
| R-07 | Desktop to API contract | `src/services/apiService.ts`, `src/stores/authStore.ts` vs `catalog-api/main.go:1143-1207` | High | D3 | Paths appear to be `/api/...` while the server registers `/api/v1/...` (see D-01) |
| R-08 | VLC FFI | `vlc/mod.rs`, `vlc/commands.rs` | High | D3 (feature on) | 34 `unsafe` lines, `unsafe impl Send/Sync`, unvalidated URL and snapshot path |
| R-09 | NFS tester | `nfs.rs` | Medium | D2 | Reports success without testing NFS; creates directories |
| R-10 | SMB tester via catalog-api | `smb.rs` | Medium | D2 | Failure modes collapse into `Ok(false)`; unauthenticated call (UNCONFIRMED whether the route needs a token) |
| R-11 | Packaging and release scripts | `catalogizer-desktop/build-scripts/build-release.sh`, `installer-wizard/install_sys_dependencies.sh`, `tauri.conf.json` bundle | High | D3 | Host builds, `sudo`, silent skips, no signing, no reproducibility (Cargo.lock ignored) |
| R-12 | Dependency posture | `Cargo.toml`, `package.json` | Medium | D2 | `reqwest 0.11`, `vite ^4`, `vitest ^0.34`, `tauri-cli` pinned `2.8.0` in installer but `>=2.0.0` in desktop; real upstream versions UNKNOWN until the detector run |
| R-13 | React pages, stores | `src/pages/*`, `src/stores/*` | Medium | D2 | Type `any` on config, error handling, XSS surface of rendered metadata |
| R-14 | Installer wizard state | `WizardContext.tsx`, `WizardLayout.tsx`, `ConfigurationContext.tsx` | Medium | D2 | Two sources of truth for position (route and `currentStep`) |
| R-15 | Playwright e2e | `e2e/theme.spec.ts`, `playwright.config.ts` | Medium | D2 | Visual regression only; runs against Vite in a plain browser, so no IPC is exercised |
| R-16 | Documentation and badges | `STATUS.md`, `README.md`, `ARCHITECTURE.md`, `TESTING.md`, `docs/desktop-testing-guide.md` | Medium | D2 | Unverified coverage claims |
| R-17 | Presentation-only components | `Layout.tsx`, `ProgressBadge.tsx`, `SplashScreen.tsx`, `ui/*` | Low | D1 | Detector sweep, theme tests |

---

## 5. Candidate findings already visible in the code

These are the first register items from reading alone. Each carries id prefix `DSK-` (desktop) or `INS-` (installer), per the register design in 04, until the register assigns stable ids. None is confirmed until a RED test on the broken artifact exists (FR-008, 11.4.115(F)). Where a recurrence of a previously recorded item is found, FR-003 applies (reopen, do not mint).

### 5.1 Desktop

| Cand. | Where | Description | Initial rating | Reproduction approach |
|---|---|---|---|---|
| D-01 | `src/services/apiService.ts:35`, `src/stores/authStore.ts:32,69,109` vs `catalog-api/main.go:1143,1163,1196-1207` | Client builds `${server_url}/api/auth/login`, `/api/media/search`, `/api/auth/status`; the server registers groups `/api/v1/auth` and `/api/v1`. No `/api` (unversioned) group appears in `main.go` (grep of `router.Group(` shows only `/debug/pprof`, `/api/v1/auth`, `/api/v1`). If no alias exists, login and every data call fail against the real backend. Also `getMediaUrl` uses `/media/:id/stream` while the server registers `/stream/:id` and `/media/:id` verbs `GET/PUT/POST` (lines 1188, 1204-1207). | Critical (if confirmed) | Section 11 contract test against the real `catalog-api` in a container. UNCONFIRMED: an alias or reverse-proxy rewrite elsewhere |
| D-02 | `main.rs:88-93` | SSRF check is `starts_with` on trimmed strings. Allows `http://api.example.com.evil.com/`, `http://localhost:8080@evil.example/`, and, when `server_url` is `Some("")`, every URL. The unit test `test_ssrf_prevention_subdomain_attack` (`main.rs:797-808`) asserts the bypass as "acceptable". | Critical | Appendix A table test calling the extracted validator; plus `mockito`/real-server redirect test for H-REDIR |
| D-03 | `main.rs:55-58` | `set_server_url` and `update_config` accept any string; the allow-base is attacker-settable from the web view, so the check does not protect against a compromised renderer | High | Rust test that sets `server_url` to a link-local address and then proxies |
| D-04 | `scripts/detect-landmines.sh:139-165` | RULE-DESK-001 uses `/\bunwrap\(\)/` in `awk`. MEASURED-NOW on this host (GNU Awk 5.3.2): `printf 'a.unwrap();' \| awk '/\bunwrap\(\)/{print "MATCH"}'` prints nothing, while `awk '/unwrap\(\)/'` matches; in gawk `\b` is backspace, word boundary is `\y`. A non-test `unwrap()` exists at `vlc/mod.rs:445`, yet the audit doc reports "clean". This is a false-null instrument (11.4.201(6)-(7)). | High (governance) | Control-needle test of the detector (section 6.4) |
| D-05 | `main.rs:117-125` | Response status is discarded: non-2xx bodies are returned as `Ok(text)`; `authStore.login` then does `JSON.parse` on an error body and `set_auth_token({token: undefined})` | High | Test with a real server returning 401 JSON |
| D-06 | `main.rs:95` | `reqwest::Client::new()` per call, no timeout, default redirect policy (up to 10), no response size cap | High | Hang test with a server that never answers; redirect test |
| D-07 | `main.rs` config | No persistence: server URL and token vanish on restart, contradicting the module overview; `directories` crate unused | Medium | Restart test: set config, relaunch, read config |
| D-08 | `main.rs` `get_config` | Returns `auth_token` to the web view; token lives in JS memory and Rust memory in clear | Medium | Decision D-ADR-04 (credential storage) |
| D-09 | `vlc/commands.rs:72-80, 301-311` | `vlc_play` accepts any URL string (libvlc locations include `file://`, device and screen capture schemes); `vlc_take_snapshot` writes to an arbitrary path | High (feature on) | Rust tests with scheme allow-list and path confinement |
| D-10 | `main.rs:151` | `VLCPlayer::new().expect(...)` panics at start when libvlc is missing (feature on); `vlc_initialize` is a no-op returning `Ok(())` (`commands.rs:65-68`), so the UI hook `useVLCPlayer` cannot report init failure | Medium | Start-up test in a container without libvlc |
| D-11 | `src/hooks/useCoverQuality.ts:38` | Uses browser `fetch` to the server origin while CSP is `connect-src 'self'`; in a release build this request is blocked by CSP (UNCONFIRMED; depends on the `tauri://` origin handling). Also bypasses the SSRF-validated proxy | Medium | Real-window test with tauri-driver; or CSP evaluation in the e2e |
| D-12 | `tauri.conf.json` `plugins.shell.open: true`; `tauri-plugin-shell` registered | v1-style plugin key; plugin has no capability and no UI usage. UNCONFIRMED whether the key is accepted by the v2 schema. Removal is an end-user-component decision only if it is user-visible; here it is not, but 11.4.124 still demands history investigation before removal | Low | `git log -S` on introduction; keep or remove decision recorded |
| D-13 | `Cargo.toml` `rust-version = "1.60"` | Almost certainly below Tauri 2's minimum Rust version. UNCONFIRMED exact value | Low | `cargo +1.60 check` in container, expect failure |
| D-14 | `src-tauri/Cargo.lock` gitignored | Application builds are not reproducible; `cargo audit` has nothing to audit (11.4.246) | High (supply chain) | Decision D-ADR-07 |

### 5.2 Installer wizard

| Cand. | Where | Description | Initial rating | Reproduction approach |
|---|---|---|---|---|
| I-01 | `network.rs:85-92, 185-198` | `timeout(d, TcpStream::connect(..)).await.is_ok()` is true whenever the connect future finishes in time, including `ECONNREFUSED`. A host that actively refuses (RST) is judged alive and every refused port is reported open | High | RED test against a local closed port on 127.0.0.1 (Appendix C) |
| I-02 | `network.rs:94-100` | `ping -W 1000` with the comment "1 second timeout"; on Linux iputils `-W` is seconds, so an unreachable host may block for up to 1000 s. Also a blocking `std::process::Command::output()` inside an async task. UNCONFIRMED; depends on platform | High | Timing test with a blackholed address in a container network |
| I-03 | `network.rs:204-228` | `scan_smb_shares_for_host` ignores `_ip` and returns the hard-coded list `C$, ADMIN$, IPC$, shared, public, media, downloads` (comments: "placeholder ... For now"). The UI therefore shows invented shares for any SMB-looking host: a stubbed-core claim (11.4.266 type `stubbed-core`) | High | Test against a real Samba container exposing different shares |
| I-04 | `ftp.rs:34-47` | `write!(stream, "USER {}\r\n", username)` and `PASS`, `CWD` with unvalidated user input: CRLF injection allows injecting extra FTP commands | High | Test with `user\r\nDELE x` against a real FTP server container with an audit log |
| I-05 | `ftp.rs`, `nfs.rs`, `webdav.rs` | `format!("{}:{}", host, port).parse::<SocketAddr>()` accepts only IP literals; any hostname fails with "Invalid address". The unit test `test_nfs_address_parsing_invalid` (`nfs.rs`) demonstrates this as expected behaviour. IPv6 literals also fail without brackets | Medium | Test with `localhost` and a DNS name |
| I-06 | `ftp.rs`, `webdav.rs`, `nfs.rs` | Blocking `std::net::TcpStream` I/O inside `async fn` commands blocks tokio worker threads for up to the 10 s timeouts | Medium | tokio test measuring a concurrent timer's lag |
| I-07 | `webdav.rs` response check | `response.contains("207") \|\| response.contains("200")` over the entire raw response; a `401` response with `Content-Length: 1200` or any body containing "200" is reported success. The `https://` branch only connects to the port and returns `Ok(true)`, never validating credentials | High | Real WebDAV container returning 401 with such a body; HTTPS server rejecting credentials |
| I-08 | `nfs.rs:7-33` | Returns `Ok(true)` after a TCP connect to 2049 and `create_dir_all(mount_point)`; `path` and `options` explicitly ignored (`let _ = (path, options)`). A "test" with a side effect that does not test NFS exports | High | Real NFS export container: wrong export path must give failure |
| I-09 | `main.rs:88-111` | `load_configuration`/`save_configuration` take any path; secrets stored in plain text (`ConfigurationAccess.secret`) | High | Path-confinement test; file-mode test (0600) |
| I-10 | `smb.rs:173-215` | `test_connection` returns `Ok(false)` on network error, on HTTP failure, and on 401; the UI cannot tell wrong password from API down. The request carries no `Authorization` header | Medium | Real `catalog-api` with auth required, and API stopped |
| I-11 | No capability files | `open`/`save` from `plugin-dialog` are called in `services/tauri.ts:202,228`. In Tauri 2, plugin commands are denied unless a capability grants them (per the Tauri 2 permissions documentation; UNCONFIRMED for this build). "Open configuration file" and "Save configuration file" may therefore not work in the shipped app | High (functional) | Real-window test (tauri-driver) invoking the dialog path |
| I-12 | `package.json` | `@catalogizer/api-client` declared as `file:../catalogizer-api-client` but no import of it under `installer-wizard/src` (MEASURED-NOW: no match for `api-client` in `src`, `vite.config.ts`, `tsconfig.json`): config-present-but-unwired (11.4.266) | Low | Investigate git history first (11.4.124) |
| I-13 | `install_sys_dependencies.sh:1` | Uses `sudo apt-get install` (constitution forbids sudo on the host) | High (governance) | Replace with the container build image; keep as documentation of package names only |
| I-14 | `STATUS.md` | Coverage and performance table with no measurement source; literal `$(date ...)` in a badge URL | Medium | Reconcile or delete claims (FR-012) |
| I-15 | `ConfigurationAccess.secret`, wizard summary | Credentials entered for SMB tests are POSTed over plain HTTP to `CATALOG_API_URL` (default `http://localhost:8080`); if the variable points to a remote host the password crosses the network in clear | Medium | Network capture test; decision on TLS requirement |

### 5.3 Shared

| Cand. | Description |
|---|---|
| S-01 | `build-scripts/build-release.sh` runs `rm -rf dist/ src-tauri/target/`, `npm install` and `tauri build` on the bare host (11.4.173 violation); `npm run lint \|\| echo ... continuing` turns a lint failure into success; each artifact copy is guarded by `if [ -f ... ]`, so a missing AppImage, deb, rpm, dmg or msi is skipped silently and the script still reports completion |
| S-02 | `.github/workflows/` contains only `README.md` (MEASURED-NOW), consistent with 11.4.156 (no active CI); enforcement is local, so every gate below is a local script |

### 5.4 Why D-04 matters beyond itself

D-04 is a measurement failure, not only a coding defect: a gate that cannot see the thing it names returns the same quiet zero as a clean tree. The audit therefore requires a control needle for every detector (section 6.4), and the detector sweep result for `unwrap` MUST be produced by the new Rust detector, not by the old script. The old script's RULE-DESK-001 stays registered as a gated defect until fixed with its own RED test (a file containing a known `.unwrap()` must make the script exit non-zero).

---

## 6. Rust-specific detector suite (containerized)

All commands run in the build container (Appendix D shows the form). The image is defined once through the `Containers` submodule; its digest is recorded in each evidence record. Tool versions are pinned in the image and recorded; none is assumed here (`UNKNOWN:` which versions the image will carry).

### 6.1 Detector table

| Id | Detector | Command (inside container, from `src-tauri/`) | Output artifact | Gate rule |
|---|---|---|---|---|
| DET-R01 | Compile with all feature sets | `cargo check --all-targets` and `cargo check --all-targets --features vlc-player` | `check-default.json`, `check-vlc.json` (`--message-format=json`) | zero errors; zero warnings for the first-party crates |
| DET-R02 | Clippy, all targets, deny warnings | `cargo clippy --all-targets --features vlc-player --message-format=json -- -D warnings -W clippy::unwrap_used -W clippy::expect_used -W clippy::panic -W clippy::indexing_slicing -W clippy::await_holding_lock -W clippy::large_futures` | `clippy.json` | no suppression attributes without a justification comment; findings become register items |
| DET-R03 | Format | `cargo fmt --check` | exit code | clean |
| DET-R04 | Advisories | `cargo audit --json` (requires a lock file; D-14) | `audit.json` | any RUSTSEC advisory is a finding with severity from the advisory; unmaintained crates recorded |
| DET-R05 | Policy | `cargo deny check` with a committed `deny.toml` (licences, bans, sources, advisories) | `deny.json` | `deny.toml` is itself a deliverable; the repo has none (`git ls-files` shows no `deny.toml`) |
| DET-R06 | Outdated | `cargo outdated --format json` (or `cargo update --dry-run`) | `outdated.json` | every behind dependency gets a decision record (SC-009) |
| DET-R07 | Unsafe inventory | `cargo geiger --output-format Json` for the crate, plus an AST count of `unsafe` blocks and `unsafe impl` | `unsafe.json` | every `unsafe` block carries a `// SAFETY:` comment and a test or a recorded reason none is possible |
| DET-R08 | Panic-on-fallible-path inventory | the new AST detector (6.3) listing `unwrap`, `expect`, `panic!`, `unreachable!`, `todo!`, `unimplemented!`, indexing `[..]` in non-test code, with the enclosing function and whether it is reachable from an IPC command | `panic-inventory.json` | each entry is closed (fixed or `// SAFE:` with reason) |
| DET-R09 | Blocking in async | custom lint over the AST (6.2) plus `clippy::await_holding_lock` | `blocking-async.json` | see 6.2 |
| DET-R10 | Dead code | `cargo build` warnings `dead_code`, `unused` and `cargo udeps` (nightly; UNCONFIRMED availability) | `dead.json` | investigate history before removal (11.4.124); never delete on sight |
| DET-R11 | MSRV | `cargo +<declared rust-version> check` | exit code | declared `rust-version` equals the real minimum (D-13) |
| DET-R12 | Rust coverage | `cargo llvm-cov --json` (or `cargo tarpaulin`; UNCONFIRMED which is installable in the image) | `coverage-rust.json` | feeds FR-011 phase-in; a percentage is only reported from this instrument |
| DET-T01 | TypeScript | `npx tsc --noEmit` | exit code | clean |
| DET-T02 | ESLint | `npx eslint . -f json` (desktop `package.json` has eslint dependencies but no `lint` script; installer `lint` is `echo ... skipped`) | `eslint.json` | the missing `lint` scripts are findings (S-03) |
| DET-T03 | npm advisories | `npm audit --json` | `npm-audit.json` | as DET-R04 |
| DET-T04 | Outdated | `npm outdated --json` | `npm-outdated.json` | as DET-R06 |
| DET-T05 | Coverage | `vitest run --coverage` (`@vitest/coverage-v8`) | `coverage-final.json` | FR-011 |

### 6.2 Blocking-in-async patterns to detect (tokio)

The AST detector (6.3) flags, inside `async fn` bodies and `async` blocks:

| Pattern | Known instances (CONFIRMED-BY-READ) |
|---|---|
| `std::net::TcpStream::connect_timeout` / `read` / `write` | `ftp.rs`, `nfs.rs`, `webdav.rs` |
| `std::process::Command::output()` | `network.rs` (`ping`, `arp`) |
| `std::fs::*` | `main.rs` `load_configuration`/`save_configuration` (installer), `nfs.rs` |
| `std::thread::sleep`, `reqwest::blocking` | none found by read; detector confirms |
| `std::sync::Mutex` guard held across `.await` | `vlc/commands.rs` uses `std::sync::Mutex` in sync commands (acceptable); the desktop async commands use `tokio::sync::Mutex` and hold the guard across `request.send().await` in `make_http_request` (lock held for the whole HTTP round trip because `config_guard` lives until function end: `main.rs:84-125`). This serialises all config reads behind a slow request |

The last row is a candidate finding in its own right (D-15, Medium): the config lock is held across network I/O.

### 6.3 Building the AST detector

Use `syn` in a small Rust binary under the build container (the repo carries no such tool today), rather than `grep` or `awk`, to avoid the D-04 class of failure. Inputs: the `.rs` files, the `#[cfg(test)]` module boundaries from the syntax tree (not line heuristics), and the list of `#[tauri::command]` functions to compute IPC reachability by call-graph walk. Output: JSON with `file`, `line`, `kind`, `function`, `ipc_reachable`, `in_test`. The tool is test-first (11.4.224): its fixtures include a golden-good file, a golden-bad file with each pattern, and a negative control (a pattern inside a string literal or a comment that must NOT be flagged).

### 6.4 Control needles for every detector (11.4.201(7)(b))

For each detector, a needle file with a known violation is placed in a scratch crate, run through the same command line, and the detector MUST report it before a zero on the real tree is accepted. For D-04, the minimal needle already exists (the 3-line awk probe above). The result JSON records `needle_found: true`.

### 6.5 Determinism and evidence

Each detector run writes its output plus `{command, container_image_digest, cargo_lock_sha256, git_commit, start/end}` to the evidence store defined in 06. Two consecutive runs from the same state MUST produce identical finding sets (SC-002); sort order and path normalisation are part of the detector wrapper.

---

## 7. Security review plan

### 7.1 Method

For every IPC command: enumerate arguments, derive the hostile input classes, write a test per class, and map to the fail-closed rule for dangerous combinations (11.4.252: mutation + untrusted input + side effect). Review order follows section 4 ratings.

### 7.2 SSRF and the proxied request

The current validator and its concrete bypass inputs (all with `server_url` as listed; expected = what a correct validator MUST return):

| # | `server_url` | Requested URL | Current result (by reading `starts_with`) | Expected |
|---|---|---|---|---|
| 1 | `http://localhost:8080` | `http://localhost:8080/api/v1/media/search` | allow | allow |
| 2 | `http://api.example.com` | `http://api.example.com.evil.com/steal` | allow (bypass; asserted as acceptable by an existing test) | reject: different host |
| 3 | `http://localhost:8080` | `http://localhost:8080@evil.example/` | allow (host is `evil.example`, userinfo is `localhost:8080`) | reject: userinfo present |
| 4 | `https://h` | `http://h/` | reject | reject: scheme |
| 5 | `http://h:8080` | `http://h:9090/` | reject | reject: port |
| 6 | `http://h:80` | `http://h/` | reject (string differs) | allow (same origin by default port) |
| 7 | `http://H.Example` | `http://h.example/x` | reject (case) | allow (host case-insensitive) |
| 8 | `http://h/api` | `http://h/api/../admin` | allow (prefix) | reject: normalised path leaves `/api` |
| 9 | `http://h/api` | `http://h/apix` | allow (prefix, no segment boundary) | reject: segment boundary |
| 10 | `` (empty, `Some("")`) | any | allow every URL | reject: invalid configured server |
| 11 | `http://h` | `http://[::1]/` , `http://127.0.0.1/` , `http://169.254.169.254/` | reject by prefix | reject (still), but only because host differs; also test when `server_url` is set to those values by `set_server_url` (D-03) |
| 12 | `http://h` | `http://h/` with redirect `302 -> http://169.254.169.254/latest/` from the server | follows redirect (hypothesis H-REDIR) | redirect policy none, or revalidate each hop against the same rule |
| 13 | `http://h` | `http://h/%2e%2e/` , `http://h/..%2f` | allow (prefix) | normalise before comparison; document what the server does with encoded dots |
| 14 | `http://h` | `http://h\@evil.example/` (backslash) | allow (prefix) | parse with the WHATWG URL parser the HTTP client uses and compare parsed components; test for parser differentials |

Target design (Appendix A): extract `validate_request_url(server_url: &str, request_url: &str) -> Result<reqwest::Url, String>`:

1. parse both with `reqwest::Url` (the same parser the client uses, avoiding differential parsing);
2. require scheme `http` or `https` and a non-empty host on the configured URL; validate when it is set (`set_server_url`, `update_config`), not only when used;
3. require equal scheme, equal host (case-insensitive by the parser), equal `port_or_known_default()`;
4. reject any userinfo on the request URL;
5. require the normalised request path to equal the configured path or extend it at a `/` boundary;
6. build the client once, store it in managed state, with `redirect::Policy::none()` (or a custom policy that re-runs the validator per hop), a request timeout, and a response size cap;
7. return an error carrying the HTTP status for non-2xx instead of the body as success; keep `Content-Type` and status available to the caller.

Header handling: forbid or ignore caller-supplied `Host`, `Content-Length`, `Transfer-Encoding`, `Connection`; the token is attached by Rust from `AppConfig`, not by the web view (decision D-ADR-04, links to D-08).

Rejected alternative: an IP deny list (loopback, link-local, RFC 1918). The configured server may legitimately be on a LAN or loopback (the installer defaults to `localhost:8080`), so a deny list produces false refusals (11.4.201(1)); an allow-origin comparison is the right primitive, with a deny list only as defence in depth for redirects.

### 7.3 Command injection and protocol injection

| Surface | Hostile input | Test |
|---|---|---|
| `ftp.rs` `USER`/`PASS`/`CWD` | `"a\r\nDELE secret"` in username, password or path | against a real FTP server container with command logging, assert the injected verb is never received; fix: reject `\r`, `\n`, NUL in all FTP arguments |
| `webdav.rs` request line and `Host` header | path containing `\r\n`; URL with spaces; credentials with `:` | assert rejection; prefer replacing the hand-written HTTP with `reqwest` (decision D-ADR-05) |
| `network.rs` `Command::new("ping")`/`arp` | `ip.to_string()` of a typed `Ipv4Addr`; not injectable | keep typed argument; regression test that the argument type remains `Ipv4Addr` |
| `vlc_play(url)` | `file:///etc/shadow`, `screen://`, `v4l2:///dev/video0`, `http://` with credentials in URL | scheme allow-list `http`, `https` and the configured server only; NUL bytes already rejected by `CString::new` |
| `vlc_set_aspect_ratio(ratio)` | arbitrary string passed to libvlc | allow-list of ratio forms `^\d{1,3}:\d{1,3}$` |

### 7.4 Path traversal and filesystem

| Command | Issue | Fix and test |
|---|---|---|
| `load_configuration(file_path)` / `save_configuration(file_path, ...)` | any path | restrict to a path returned by the dialog plugin within the current session, or to the default config directory; canonicalise and reject symlink escapes; test with `../`, absolute `/etc/...`, a symlink inside the allowed directory pointing out |
| `save_configuration` | writes secrets with default umask | create with mode 0600 on Unix, and document Windows ACL behaviour; test with `stat` |
| `test_nfs_connection(mount_point)` | `create_dir_all` of any path | do not create directories in a test command; validate only |
| `test_local_connection(base_path)` | existence oracle for any path | acceptable for a local installer, but document it; return only a boolean, no error text distinguishing existence from permission |
| `vlc_take_snapshot(filepath)` | arbitrary write | confine to a snapshots directory |

### 7.5 Credential storage and handling

| Item | Current | Plan |
|---|---|---|
| Desktop token | in-memory, returned to the web view by `get_config` | decision D-ADR-04: keep the token in Rust only (attach in `make_http_request`), persist in the OS keychain through a reviewed crate (candidate: platform keyring crates; UNKNOWN which is maintained; web research is part of WP-D4), never in `localStorage` |
| Desktop persistence | none | persist non-secret config (theme, server URL, auto start) to the app config directory (`directories` is already a dependency); test restart behaviour |
| Installer config file | plain-text `secret` | mode 0600 at minimum; offer an option to store only a reference; document the risk in the manual |
| Wizard to `catalog-api` | HTTP, password in body | require HTTPS for non-loopback `CATALOG_API_URL`; refuse otherwise (fail closed, 11.4.252) |
| Logging | `env_logger`, `log::warn!` on failures | assert logs never contain passwords or tokens (test with a sentinel secret and a log capture) |

### 7.6 Capabilities, CSP, plugins

1. Tauri 2 expects capability files under `src-tauri/capabilities/` (per the Tauri 2 security documentation; UNCONFIRMED against the exact framework version in `Cargo.lock`, which is not committed). With none present, plugin commands are not granted to the web view and the app's own commands are callable by every window. Plan: add explicit capability files per application listing exactly the permissions the UI uses (installer: dialog open and save, and nothing from `fs`; desktop: none beyond core), and a test that invokes a permission the capability does not grant and expects denial.
2. Decide whether to keep `tauri-plugin-shell` (desktop, both) and `tauri-plugin-fs` (installer). They are registered but unused by non-test front-end code; the Rust backend does its own file I/O. Removal affects no end-user feature, but 11.4.124 requires investigating history first and recording the evidence; a keep decision requires a capability that is minimal.
3. CSP: desktop allows `style-src 'unsafe-inline'` and Google font origins; the app is otherwise offline-capable. Plan: self-host fonts, drop the two external origins, evaluate removing `'unsafe-inline'` (Tailwind in a Vite build normally emits CSS files; measure with a real window and the console CSP report), and add `connect-src` only where a front-end `fetch` is intentionally used (D-11: prefer routing through `make_http_request` instead).
4. Navigation and window: confirm the web view cannot navigate to remote origins (`app.security` has no `dangerousRemoteDomainIpcAccess`; verify nothing sets it) and that links open through an allow-listed opener.

### 7.7 Updater and signing

There is no updater and no signing configuration. Plan outputs: (a) a decision record on whether an auto-updater is in scope (owner decision, section 17); if yes, the Tauri updater requires a signing key pair for update artifacts, whose private key never enters the repository (11.4.10); (b) release verification of signatures where signing exists (section 10); (c) an explicit finding "artifacts are unsigned" with the owner decision on certificates (macOS signing identity and notarisation, Windows certificate): these need credentials the audit cannot invent, so tests that depend on them are `blocked-unavailable` until supplied (FR-025).

### 7.8 Protocol testers' network behaviour

| Behaviour | Risk | Control |
|---|---|---|
| `scan_network` scans every attached /24 including container and VPN interfaces | scanning networks the user did not intend | confirm in UI; limit to private ranges; cap total hosts; record in the manual |
| `scan_network` reverse DNS and ARP | information disclosure only locally | document |
| Testers send credentials to user-entered hosts | credentials to wrong host after a typo | show host in confirmation; never persist on failure |
| Cleartext FTP and WebDAV over HTTP | credentials on the wire | warn in UI, support FTPS and HTTPS; document |

---

## 8. Test plan by type

Applicable types come from the constitution's catalogue (11.4.27, 11.4.169; see 05 for the full matrix). The table states, for these two applications, what exists (measured) and what must be written.

| Test type | Desktop: existing | Desktop: to write | Installer: existing | Installer: to write |
|---|---|---|---|---|
| Unit (Rust) | `main.rs` tests (many re-implement logic inline rather than call the command: e.g. HTTP method tests assert on string literals, SSRF tests assert `starts_with` on literals) | tests that call the real validator, the real config commands via Tauri's test runtime, VLC command argument validation | in-module tests per protocol file (many assert constants, e.g. `assert_eq!(port, 2049)`; `format!` results) | real behaviour tests per section 9 |
| Unit (front end, vitest) | pages, stores, hooks, services, utils | missing: `HistoryDrawer`, `ProgressBadge`, `VLCPlayer` component, `useVLCPlayer` behaviour (UNCONFIRMED which are untested: only `Layout`, `SplashScreen` and `useCoverQuality` have tests under `components/`, `hooks/`) | contexts, components, services | wizard steps are untested individually (UNCONFIRMED; only `App.test.tsx`, layout and ui tests visible) |
| Integration | none against the real backend | IPC contract tests (8.3), real `catalog-api` in container | none | protocol testers against real servers (section 9) |
| Contract (consumer and provider) | none | section 11 | none | `catalog-api` smb routes contract |
| Property / fuzz | none | URL validator property tests (`proptest`); parser differential test against `reqwest::Url` | none | FTP/WebDAV argument sanitiser properties |
| E2E (real window) | none | tauri-driver journeys (8.5) | none | wizard happy path and failure paths in a real window |
| E2E (browser) | `e2e/theme.spec.ts` | not a substitute for the real-window E2E | none | n/a |
| Security | none | section 7 tests as automated checks | none | injection, path confinement, secret handling |
| Performance / benchmark | none | section 13 | none | scan duration, tester latency |
| Stress / chaos | none | rapid config mutation under proxy load (lock behaviour D-15), server drop mid-response | none | scan with unreachable subnets, server killed mid-handshake |
| Visual / UI | Playwright theme snapshots | extend per page in both themes (11.4.170 host-rendered pixels); WCAG contrast tests exist (`src/styles/wcag_contrast.test.ts`) | none | wizard steps in both themes |
| Packaging smoke | none | section 10 | none | section 10 |
| Mutation | none | `cargo mutants` on `main.rs`, `vlc/commands.rs`; Stryker or equivalent for TS (UNCONFIRMED tool availability in image) | none | same for protocol modules |
| HelixQA / Challenges | UNKNOWN whether any bank covers desktop or installer; to inventory in 03 | add banks that drive the real window | UNKNOWN | add |
| Documentation tests | none | rendered-diagram and link checks | none | same |

### 8.1 Rules every new test follows

1. Test-first with a recorded RED run on the pre-fix artifact (11.4.224(A), 11.4.115(F)).
2. Named oracle strategy per test (11.4.245): for the SSRF validator the oracle is the WHATWG URL parser plus a hand-built expected table (specified oracle, independent of the code under test); for protocol testers the oracle is the real server's own log (derived from the server, not from the code).
3. A paired mutation per test: weakening the validator to `starts_with` MUST turn the SSRF table test red (this is the §1.1 pair for D-02).
4. Tests that merely assert constants or re-implement the logic in the test body (`test_http_method_parsing`, `test_ssrf_*` in `main.rs`, `test_nfs_standard_port`) are classified `coverage-theater` in the ledger and are replaced, not kept (11.4.261, 11.4.266); they do not count toward the FR-011 numerator because they are not RED-capable.

### 8.2 Rust unit and tokio tests (desktop)

| Test group | Content |
|---|---|
| `validate_request_url` table | rows from 7.2; Appendix A |
| config commands via the Tauri test runtime | build an app with `tauri::test::mock_builder()` (API surface UNCONFIRMED for the pinned version) and drive `get_config`, `update_config`, `set_server_url` through the IPC layer, not by direct function call, so argument deserialisation (camelCase keys such as `newConfig`) is covered |
| handler-list parity | with and without `vlc-player`, the set of registered command names equals the intended set (guards the duplicated `generate_handler!` lists) |
| proxy behaviour | against a real HTTP server started by the test (a local `hyper` or `axum` server is a real server; not a mock of reqwest): status propagation, redirect refusal, timeout, size cap, header filtering |
| lock behaviour | a slow server and concurrent `get_config` calls; the config call must not wait for the slow request (D-15) |
| VLC argument validation | scheme allow-list, snapshot path confinement; the libvlc-dependent tests run only where libvlc exists in the image |

### 8.3 IPC contract tests against the real backend

Two layers, because one layer cannot cover both argument serialisation and window behaviour:

1. Rust integration tests with the Tauri test runtime call each command by name with JSON arguments exactly as the front end sends them and assert the JSON shape of the reply. The front end's expected shapes are generated from `src/types/index.ts` into a JSON schema file checked in (WP-D6); the Rust test validates replies against it.
2. Front-end tests with a recorded-then-real bridge: `@tauri-apps/api/core` `invoke` is mocked in unit tests (this is acceptable only as a unit test, 11.4.27(A)). The real-bridge check is layer 3.
3. Real-window tests (8.5) drive the packaged app against a real `catalog-api` container.

### 8.4 Vitest plan

Existing tests mock `invoke`. Add: (a) the route-contract test of Appendix B, which fails today if D-01 is real; (b) a test that `makeRequest` never sends an `Authorization` header built from `get_config` once the token handling moves into Rust (D-ADR-04); (c) store tests for non-2xx handling; (d) accessibility and theme tests per page in both themes; (e) a test that no component uses `fetch` directly (D-11) via an ESLint rule `no-restricted-globals` for `fetch` in `src/`.

### 8.5 Real-window end-to-end (Playwright is not enough)

`e2e/theme.spec.ts` drives the Vite dev server in Chromium; outside Tauri, `invoke` has no host, so IPC paths are never exercised there (the test itself relies on "no server configured redirects to Settings"). Real-window tests use `tauri-driver` with the platform WebDriver (the Tauri 2 documentation lists Linux and Windows support and no macOS support; UNCONFIRMED for the pinned version; macOS journeys would be `blocked-unavailable` or covered by a different harness decision, D-ADR-06). Journeys: first run to settings; set server; login against the real API; library browse; search; detail; (feature on) playback start with real content from a fixture library (11.4.136, 11.4.143: drive the real UI, not a deep link); logout; restart persistence. Each journey records a screenshot and an OCR/vision verdict (11.4.170, 11.4.159).

### 8.6 Coverage phase-in (FR-011)

1. Record baselines in the first measurement run: `cargo llvm-cov` for each crate, `vitest --coverage` for each front end, with the exclusion list checked in and justified (generated code under `gen/`, vendored assets only; first-party exclusions need a tracked item, 11.4.224(E)).
2. The numerator counts RED-capable tests only; assertion-free or constant-asserting tests do not count.
3. Adoption option (owner decision, section 17): ratchet from baseline to 85% in named steps, with the phase list recorded as data.
4. The VLC module counts only when the feature is on in the measured build; the report states the feature set measured. Branch coverage is not measurable with the line-based method in 11.4.224(E); use `llvm-cov` region coverage and state its limits.

---

## 9. Installer wizard: state machine, protocol testers and real targets

### 9.1 State machine as implemented (CONFIRMED-BY-READ)

Routing is defined in `App.tsx`; the step list is computed in `WizardLayout.tsx:15-45` from `configState.selectedProtocol`; `WizardContext` keeps a numeric `currentStep`, `totalSteps`, `canGoNext`, `canGoPrevious`; each step calls `setCanNext(...)` (e.g. `NFSConfigurationStep.tsx:60`: `canNext` iff at least one config is saved).

```mermaid
stateDiagram-v2
  [*] --> Splash
  Splash --> Welcome: splash complete
  Welcome --> ProtocolSelection: next (canNext true)
  ProtocolSelection --> NetworkScan: protocol chosen and scan step included
  ProtocolSelection --> Configure: protocol chosen, no scan
  NetworkScan --> Configure: at least one host selected or scan found none
  state Configure {
    [*] --> SMB
    [*] --> FTP
    [*] --> NFS
    [*] --> WebDAV
    [*] --> Local
  }
  Configure --> Manage: at least one configuration saved
  Manage --> Summary: next
  Summary --> [*]: finish
  Configure --> ProtocolSelection: previous
  Manage --> Configure: previous
  Summary --> Manage: previous
```

Observations to turn into tests:

1. Two sources of truth for position: the router pathname (`currentStepIndex = steps.findIndex(path === location.pathname)`) and `WizardContext.currentStep` advanced by `nextStep()`. A direct URL change, or a protocol change after visiting a configure step, can desynchronise them (UNCONFIRMED). Test: select SMB, go forward, go back, switch to FTP, go forward; assert pathname, `currentStep` and the progress indicator agree.
2. `NetworkScanStep.setCanNext(selectedHosts.length > 0 || hosts?.length === 0)`: `hosts` undefined (scan failed) yields `canNext` false with no recovery path other than rescanning. Test the failure state of `scan_network`.
3. `WizardLayout` "Finish" label appears at the last step but the finishing action (writing the configuration) is performed where? UNCONFIRMED; trace in WP-I1.
4. The step list is state-derived; `isComplete` is computed from `totalSteps`, which is updated in an effect after render, so one render may see the stale total. Test the transition frame.

### 9.2 Protocol tester inventory and required behaviours

| Protocol | Implementation | What "success" must mean | Today |
|---|---|---|---|
| SMB | delegates to `catalog-api` `/api/v1/smb/test` | authenticated session to the share established and share listed | depends on the API; failures collapse to `Ok(false)` (I-10); share discovery in the scan is a hard-coded list (I-03) |
| FTP | raw TCP dialogue | TCP connect, banner 220, `USER`, `PASS` (accept 230; handle 331 then 230), optional `CWD`, `QUIT`; support passive listing for a directory check | substring-free but fragile: single `read` per reply (partial reads), strict `230` only, CRLF injection (I-04), IP-literal hosts only (I-05) |
| NFS | TCP connect to 2049 | export reachable and the specific export path mountable or listable (portmapper/mountd `EXPORT` query, or an NFSv4 lookup of the path) | TCP only, creates a directory (I-08) |
| WebDAV | raw HTTP over TCP | `PROPFIND Depth:0` returns 207, with credentials accepted (401 distinguished from 403 and from network failure) | substring status test, HTTPS unauthenticated (I-07) |
| Local | `read_dir` | directory readable by the current user | adequate |

### 9.3 Real targets (FR-025) and what they are

FR-025 requires the real service for behaviour that depends on one. Two classes of target exist and must be distinguished in every report:

1. Reference servers started by the test in rootless containers through the `Containers` submodule: a real Samba, a real FTP daemon, a real WebDAV server (for example Apache or nginx WebDAV modules), and a real NFS server. These are real protocol implementations; tests against them verify the testers' protocol behaviour deterministically and are required for every run. UNCONFIRMED: a kernel NFS server needs privileges a rootless container does not have; a user-space NFS server (for example nfs-ganesha) is the candidate; if no rootless NFS server works, NFS tests are `blocked-unavailable` with that reason and the owner decides (section 17), never simulated.
2. The owner's real NAS or share (the device the wizard is actually meant for): tests that claim compatibility with it run only when its address and credentials are supplied through the secrets mechanism (11.4.10); otherwise they are reported `blocked-unavailable`. This audit does not know what shares exist (`UNKNOWN:`).

Mocks of the protocol (a fake FTP server written for the test, a stub that returns 207) are forbidden outside unit tests of pure parsing functions (11.4.27(A)).

### 9.4 Per-protocol test matrix (to write)

| Id | Target | Scenario | Expected | Defect it guards |
|---|---|---|---|---|
| P-FTP-01 | real FTP, user `u` | valid login | true | baseline |
| P-FTP-02 | same | wrong password | error "authentication failed", never true | |
| P-FTP-03 | same | username `u\r\nDELE x` | rejected before send; server log shows no `DELE` | I-04 |
| P-FTP-04 | same | host `localhost` | works | I-05 |
| P-FTP-05 | same | server sends 220 banner in two TCP segments | works | partial read |
| P-FTP-06 | same | USER reply 331 then PASS 230 | works | |
| P-DAV-01 | real WebDAV, auth required | valid credentials | true | |
| P-DAV-02 | same | wrong credentials, response body contains "200" | error | I-07 |
| P-DAV-03 | same, HTTPS with a self-signed test CA | wrong credentials | error, not `true` | I-07 (HTTPS branch) |
| P-DAV-04 | same | hostname URL | works | I-05 |
| P-NFS-01 | real NFS export `/export` | path `/export` | true | |
| P-NFS-02 | same | path `/nope` | false | I-08 |
| P-NFS-03 | any | `mount_point` not existing | no directory created | I-08 |
| P-SMB-01 | real Samba share with 2 named shares | scan_shares | exactly the real shares, not the hard-coded list | I-03 |
| P-SMB-02 | same, through real `catalog-api` | wrong password | distinct error class from API down | I-10 |
| P-SMB-03 | `catalog-api` stopped | any | error "service unreachable", not `false` | I-10 |
| P-NET-01 | 127.0.0.1 closed port | `scan_ports` | closed port not reported | I-01 |
| P-NET-02 | blackholed address | `is_host_alive` | returns within the stated bound | I-02 |
| P-NET-03 | container network with 2 live hosts and 1 refusing host | scan | exactly the live hosts | I-01 |
| P-LOC-01 | existing dir, file, missing path, unreadable dir | all four | true, error, error, error | |
| P-CFG-01 | save then load | roundtrip with secrets; file mode 0600 | I-09 |
| P-CFG-02 | save to `/etc/x`, `../../x`, symlink escape | refuse | I-09 |

### 9.5 Blocked-unavailable handling

The test harness (06) provides a `requires:` declaration per test (service, credential name, device). Pre-flight resolves each requirement by a real probe (connect and identify), never by configuration presence (11.4.201(11)). Unresolved requirements produce verdict `blocked-unavailable` with `{requirement, probe_command, probe_output}`; the run exit code is non-zero; reports list blocked tests separately from failures and skips; no `#[ignore]`, `skip` or conditional compilation is used to hide them. A completion claim for the installer is impossible while any required test is blocked (spec: "the feature is not complete until the owner supplies the missing service").

---

## 10. Packaging, signing and release verification

### 10.1 Current state

| Item | Finding |
|---|---|
| Build entry point | `catalogizer-desktop/build-scripts/build-release.sh`: native `npm install`, `rm -rf dist/ src-tauri/target/`, `tauri build` on the bare host, branching on `$OSTYPE` (S-01) |
| Bundle targets | `"all"`; AppImage, deb, rpm on Linux, dmg and app on macOS, msi and nsis on Windows are requested; the script copies only artifacts it finds and does not fail on missing ones |
| Linux system packages | `installer-wizard/install_sys_dependencies.sh` uses `sudo apt-get install libgtk-3-dev libwebkit2gtk-4.1-dev libayatana-appindicator3-dev librsvg2-dev` (I-13) |
| Reproducibility | no committed lock file for the Rust crates (D-14); Node lock files exist (`package-lock.json` in both) |
| Signing | none configured |
| Release evidence | none; artifacts are named by version only |

### 10.2 Target pipeline

```mermaid
flowchart TD
  A["Source at git commit + Cargo.lock + package-lock.json"] --> B["Rootless build container (Containers submodule), image digest recorded"]
  B --> C["Frontend build: npm ci, tsc, vite build"]
  C --> D["tauri build: AppImage, deb, rpm (Linux container)"]
  D --> E["Artifacts + SHA-256 + SBOM + build manifest (commit, lock hashes, image digest, build id)"]
  E --> F["Copy back to originating host (rsync/scp)"]
  F --> G["Clean-target smoke: fresh container, install artifact, launch under Xvfb, read build id through IPC"]
  G --> H{"build id == manifest build id?"}
  H -- yes --> I["Real-window journeys (8.5) against real catalog-api"]
  H -- no --> X["FAIL: wrong artifact (11.4.200)"]
  J["macOS and Windows hosts (owner supplied)"] -. "dmg / msi / nsis" .-> E
```

### 10.3 Steps and acceptance evidence

1. Replace `build-release.sh` host logic with a script that calls the containerized build and fails when any requested artifact is missing; the `|| echo ... continuing` on lint is removed and lint becomes a real gate (S-03).
2. Commit `Cargo.lock` for both applications (decision D-ADR-07) so `cargo audit`, `cargo deny` and reproducible builds are possible (11.4.246). Record the build as hermetic: no network at build time after dependency fetch, pinned base image digest.
3. Build identity: add a `build.rs`-injected `CATALOGIZER_BUILD_ID` (git commit and manifest hash) and expose it through the existing `get_app_version` command extension or a new `get_build_info` command; a `--build-info` command-line flag is also needed so the smoke test can read identity without driving a UI (11.4.200 requires reading identity back from the intended target). The smoke test compares it with the manifest produced at build time (11.4.108 layers 2 and 3).
4. Linux artifacts: AppImage (run with extract-and-run inside the container where FUSE is unavailable; UNCONFIRMED), deb (install into a clean container), rpm (install into a clean RPM-based container). Each smoke records: install exit code, installed file list hash, launch under a virtual display, window title via the WebDriver, build id.
5. macOS dmg and Windows msi: these formats need their native toolchains; a Linux container cannot produce them (per Tauri's platform documentation; UNCONFIRMED for the exact tooling in use). Without an owner-supplied macOS and Windows build host (reachable build hosts such as the one named in the project's containerized-build mandate are `UNKNOWN:` for this task) these artifacts are `blocked-unavailable`; the report lists them as such and the feature is not complete for those platforms until the owner supplies the hosts.
6. Signing: with credentials supplied, verify signatures (`codesign --verify`, `signtool verify`, `gpg --verify` for Linux repository signing if adopted); without credentials the finding "unsigned artifacts" stays open as a blocked item with the owner's options.
7. SBOM and provenance: generate an SBOM (CycloneDX) for the Rust and npm dependency sets and a build manifest; attach checksums; record SLSA level honestly (11.4.246) in the manifest.
8. Version consistency: a test asserts `package.json`, `Cargo.toml`, `tauri.conf.json` versions and the `get_app_version` result agree (all are 2.4.0 today; MEASURED-NOW).

### 10.4 Cross-OS smoke matrix

| Platform | Artifact | Build location | Smoke location | Status until owner action |
|---|---|---|---|---|
| Linux x86_64 | AppImage, deb, rpm | rootless container | clean container | executable now |
| Linux aarch64 | same | UNKNOWN: needs an arm host or emulation | UNKNOWN | blocked-unavailable |
| macOS | dmg, app | macOS host required | macOS host | blocked-unavailable |
| Windows | msi, nsis | Windows host required | Windows host | blocked-unavailable |

---

## 11. Shared-contract drift check

### 11.1 Contract surfaces

| Consumer | Provider | Surface |
|---|---|---|
| desktop front end | `catalog-api` | HTTP endpoints used by `apiService.ts` and `authStore.ts` |
| installer Rust | `catalog-api` | `/api/v1/smb/discover`, `/browse`, `/test` (registered in `catalog-api/internal/handlers/smb_discovery.go` annotations at lines 79, 110, 156) |
| desktop front end | desktop Rust | the IPC command set and its argument/reply shapes |
| installer front end | installer Rust | same |
| both front ends | `catalogizer-api-client` | UNKNOWN whether any type is shared: desktop `package.json` does not depend on it; installer declares it but does not import it (I-12); `src/types/index.ts` in each app is a hand copy (UNCONFIRMED vs the client library types) |

### 11.2 Method

1. Provider side: extract the registered route table from `catalog-api/main.go` (via the structural index query and a parser over `router.Group` / `api.GET|POST|PUT|DELETE` calls), checked in as a generated JSON file with its generator (11.4.77) and a fingerprint (11.4.86).
2. Consumer side: extract every endpoint literal and method from `apiService.ts`, `authStore.ts` and `smb.rs`, by AST (TypeScript compiler API; `syn`), not regex alone.
3. Compare: every consumer call MUST resolve to a registered provider route with a compatible method; mismatch is a finding. Appendix B is the static layer.
4. Dynamic layer: boot the real `catalog-api` with its test database in a container (the repo has `docker-compose.test-infra.yml`; use through the Containers submodule), run each consumer call with real credentials created for the test, validate response bodies against the provider's OpenAPI document where one exists (UNKNOWN: whether `catalog-api` publishes one; the handlers carry swag annotations such as `@Router`), else against generated JSON schemas checked in on both sides.
5. A `can-i-deploy` check (11.4.244): before a desktop or installer release, the provider version in the release manifest must be compatible with the consumer contract file; missing contract is a refusal.

### 11.3 Expected first results (hypotheses)

H-C1 login and logout paths differ (`/api/auth/*` vs `/api/v1/auth/*`); H-C2 `/api/auth/status` has no counterpart in the provider routes read here (UNCONFIRMED; line 1143-1160 not fully read for an `/status` route); H-C3 `/media/:id/stream` and `/media/:id/download` vs `/stream/:id`; H-C4 `/entities/:id/progress` exists provider-side (line 1523) and is the one the front end uses with `/api/v1` prefix literal at `apiService.ts:112` comments, i.e. the progress endpoints may be correct while the older ones drifted. Each is tested before being called a finding.

---

## 12. Frontend plan (React, stores, services, e2e)

1. Typing: `invoke<any>('get_config')` appears in `App.tsx:28`, `authStore.ts:102` and `apiService.ts`; replace with generated types from the contract schema (8.3). `any` counts are measured by DET-T02 and tracked.
2. Error handling: `authStore.login` assumes `data.token` and `data.user` exist; add runtime validation (zod is already used in the installer; add or reuse in the desktop) and test the malformed-body case (D-05).
3. XSS surface: pages render server-provided strings (titles, overviews, filenames). Review for `dangerouslySetInnerHTML` and unsanitised URLs (detector: ESLint `react/no-danger`, and a grep-free AST check); tests render hostile strings (`<img src=x onerror=...>`, `javascript:` URLs) and assert inertness.
4. `SettingsPage` (401 lines) and `VLCPlayer.tsx` (515 lines) are the largest components; review for state-machine clarity: model player state `{idle, opening, playing, paused, ended, error}` mirrored from `PlaybackState` in Rust and test the hook's polling teardown (no `invoke` after unmount; `useVLCPlayer.ts:95` calls `vlc_stop` on cleanup).
5. Theme: `src/styles/tokens.css`, `tokens.ts`, `tokens.test.ts` and `wcag_contrast.test.ts` exist; extend visual regression to all pages and both themes and bind them to the design-token contract of the design anchors (11.4.216, 11.4.162); the e2e threshold `maxDiffPixelRatio: 0.02` is a calibration decision to record, not a given.
6. Wizard: tests per step with the real `TauriService` wired to a Tauri test runtime (layer 1 of 8.3), plus 9.1's state-machine tests.
7. Remove no component on sight: `HistoryDrawer`, `ProgressBadge`, `debug/` components may be unused; follow 11.4.124 (history first).

---

## 13. Performance baselines

SC-011 requires measured baselines and documented targets for critical operations. For these applications:

| Operation | Metric | Instrument | Target source |
|---|---|---|---|
| Desktop cold start to first interactive screen | ms, p50/p95 over N runs | tauri-driver timestamps, `performance.mark` in the page | proposed target to be set by owner after first measurement; `installer-wizard/STATUS.md` claims "App Startup < 2s" without a source (I-14) |
| Login round trip | ms | real backend, loopback and LAN | same |
| Search request through the proxy | ms added by the Rust hop | A/B: direct HTTP vs `make_http_request` | overhead budget proposed after measurement |
| Library page render with N items | ms and memory | profiler trace | same |
| Playback start (feature on) | ms from click to first frame | real-window, real media | same |
| Installer network scan | seconds per /24, CPU, concurrent tasks | container network with N live hosts; hostile case with blackholed addresses (I-02) | same |
| Protocol tester latency | ms per tester vs the 10 s timeouts | real servers | same |
| Memory | RSS after 1 hour idle, after 100 navigations | container `podman stats` (host ceilings per constitution) | same |

Method: N >= 20 runs after a documented warm-up, medians and 95th percentiles, recorded with environment fingerprint; the host memory ceiling and thread limits (constitution 12.6, 12.12) apply, so no scan targets more than the test network.

---

## 14. Documentation required

| Document | Content | Existing | Action |
|---|---|---|---|
| Desktop user manual | install, first run, server setup, login, browse, search, playback, settings, uninstall | `README.md` (86 lines) only | write |
| Desktop guides | connect to a server, play a title, troubleshoot (server unreachable, certificate, VLC missing) | none | write |
| Desktop FAQ | from real support questions (to be gathered from the register and QA banks) | none | write |
| Installer manual and guides | per protocol, config file format, credentials handling, scan privacy | `README.md`, `TESTING.md` (252 lines) | rewrite against measured behaviour |
| Installer FAQ | refused connections, hostnames, IPv6, HTTPS WebDAV, NFS caveats | none | write |
| Architecture docs | `ARCHITECTURE.md` in each app (desktop 48 lines) | reconcile with section 3 diagrams | update |
| IPC reference | each command: arguments, replies, errors, capability | none | generate from source (a generator, 11.4.77) and test it against the schema |
| Diagrams (must render non-blank, FR-014) | architecture (3.3), data flow (token and config), state machine (9.1), sequences (3.4 and wizard test sequence), packaging flow (10.2) | partial | embed in the manuals; render checks in the docs pipeline |
| Definitions (FR-015) | `Configuration`, `AppConfig` schemas, config file JSON schema | none | JSON schema checked in, tested against Rust structs via `schemars` or a golden roundtrip |
| Security notes | threat model summary from section 7 | none | write |
| Release notes and build manifest | per section 10 | none | generate |
| Reconciliation | `STATUS.md` (claims and literal `$(date ...)` badge), `docs/DESKTOP_PERF_AUDIT.md` (RULE-DESK-001 claim), `docs/desktop-testing-guide.md` (non-existent directory, 80% claim), `catalogizer-desktop/CLAUDE.md` (persistence claim) | | correct each claim or delete it; each correction cites measured evidence |

All documents carry the revision header, are exported per the export rule, and are linked from the main README transitively (FR-013). Docs for `docs/` files under `submodules` or vendored trees are out of this plan.

---

## 15. Work packages, sequencing and acceptance evidence

Ordering follows risk (11.4.132): R-01, R-07, R-11, R-02 first.

| WP | Title | Depends on | Output | Acceptance evidence |
|---|---|---|---|---|
| WP-D0 | Build container and evidence harness for Rust and Node | Containers submodule, 06 | image digest, detector wrappers | detector control needles pass (6.4) |
| WP-D1 | Detector sweep, both apps | WP-D0 | JSON outputs, register items | identical finding set on two runs (SC-002) |
| WP-D2 | Contract drift check (D-01) | WP-D0, backend container from 07 | route tables, contract test | RED against the pre-fix client, GREEN after fix |
| WP-D3 | SSRF and proxy hardening (D-02, D-03, D-05, D-06, D-15) | WP-D0 | extracted validator, managed client, tests, mutation pair | Appendix A table all green; mutation to `starts_with` goes red |
| WP-D4 | Credential and config handling (D-07, D-08, I-09, I-15) | WP-D3 | persistence, keychain decision implemented, file modes, HTTPS requirement | restart test, mode test, log-sentinel test |
| WP-D5 | Capabilities, CSP, plugins (D-11, D-12, I-11) | WP-D0 | capability files, minimal CSP, decisions | denial tests, real-window dialog test |
| WP-D6 | IPC schema and typed bridge | WP-D3 | generated schema, typed front-end calls | schema drift test |
| WP-D7 | VLC hardening (D-09, D-10) | WP-D3 | scheme allow-list, path confinement, init error reporting, `// SAFETY:` comments | feature-on tests in an image with libvlc |
| WP-I1 | Wizard state machine tests (9.1) | WP-D0 | tests | transition tests pass; desync cases RED then fixed |
| WP-I2 | Network scan correctness (I-01, I-02, I-03, I-06) | WP-D0, real-target containers | fixes, tests P-NET-*, P-SMB-01 | tests against the container network |
| WP-I3 | Protocol testers rewrite (I-04..I-08, I-10) | WP-I2 | tests P-FTP, P-DAV, P-NFS, P-SMB | per-row evidence; blocked rows listed |
| WP-I4 | Config file commands (I-09) | WP-D4 | confinement, mode | P-CFG tests |
| WP-P1 | Packaging pipeline and smoke (section 10) | WP-D0, D14 decision | scripts, manifests, smoke tests | build id match; blocked platforms listed |
| WP-T1 | Coverage baselines and phase-in (8.6) | WP-D1 | baseline JSON, exclusion list, phase table | measured numbers only |
| WP-T2 | Real-window E2E | WP-P1 | tauri-driver journeys | screenshots plus vision verdicts |
| WP-T3 | Mutation sampling | WP-D3, WP-I3 | mutation reports | SC-005 sample drawn by the reviewer |
| WP-X1 | Documentation set (section 14) | all | documents | links reachable, diagrams non-blank |
| WP-X2 | Dependency decisions | WP-D1 | decision per behind dependency | SC-009 table |

Each WP closes with: register items updated, evidence records stored, the independent review of 11.4.209 run on the change (iterating to no blocking finding, FR-023), then fast-forward push to all upstreams (no force, FR-020).

---

## 16. Traceability to the spec

| Requirement | Where addressed here |
|---|---|
| FR-006 | sections 2, 4 (both applications in scope with ratings) |
| FR-007 | section 5 (location, severity, evidence path per candidate) and 6 (machine evidence) |
| FR-008 | 5 (RED reproduction approach), 8.1, 15 |
| FR-009 | section 8 matrix; absent types are written, not planned |
| FR-010 | 6.5, 8.1 (determinism, oracle, mutation pair) |
| FR-011 | 8.6, WP-T1 |
| FR-012, FR-013, FR-014, FR-015 | section 14 |
| FR-016 | section 11 |
| FR-021 | 1.3, 6, 10 |
| FR-022 | 1.2, 1.3 |
| FR-025 | 9.3, 9.5, 10.4 |
| SC-002 | 6.5 |
| SC-003 | 5 and 15 (every candidate fixed or closed with evidence) |
| SC-004 | 8 |
| SC-005 | WP-T3 |
| SC-007 | 14 |
| SC-011 | 13 |
| SC-012 | 1.3 |

---

## 17. Decision records, rejected alternatives, owner decisions

| ID | Decision | Choice proposed | Rejected alternative and why | Owner input needed |
|---|---|---|---|---|
| D-ADR-01 | SSRF primitive | parsed same-origin comparison plus no redirects/revalidation | IP deny list: false refusals for LAN and loopback servers (11.4.201(1)) | no |
| D-ADR-02 | Where validation runs | in Rust, at set time and use time | front-end validation: bypassable by any script in the web view | no |
| D-ADR-03 | Hand-written FTP/WebDAV vs crates | evaluate maintained crates through web research before writing code (11.4.8, 11.4.270 existence register); prefer `reqwest` for WebDAV | keep hand-written sockets: continuing CRLF and parsing defects | yes, if a new dependency is judged unacceptable |
| D-ADR-04 | Token storage | Rust-only, OS keychain, attached in Rust | keep token in web view memory: exposure to any script; persisting in `localStorage`: worse | yes: whether remember-me is wanted |
| D-ADR-05 | WebDAV client | `reqwest` with explicit methods | extending the raw socket code | no |
| D-ADR-06 | Real-window harness | tauri-driver where supported | Playwright against Vite: does not exercise IPC (section 8.5); macOS harness `UNKNOWN:` | yes: macOS hosts |
| D-ADR-07 | Commit `Cargo.lock` for applications | yes (remove from `.gitignore` for these crates) | keep ignored: no audit, no reproducibility | confirm |
| D-ADR-08 | NFS real target | user-space NFS server container if feasible | simulating NFS: forbidden (FR-025) | yes, if infeasible: supply an NFS host |
| D-ADR-09 | Plugins `shell`, `fs` | keep only with minimal capability, else remove after history check | silent removal: forbidden (11.4.122/11.4.124) | confirm keep or remove |
| D-ADR-10 | Updater | out of scope until owner decides | add one now: needs signing keys the audit cannot create | yes |
| D-ADR-11 | Coverage adoption | phase-in ratchet from measured baseline | immediate 85%: not meetable on the first run; no floor: forbidden by FR-011 | confirm phase list |

Blocked items needing the owner (to be recorded as blocked with choices, not guessed): macOS and Windows build and smoke hosts; signing certificates; an owner NAS for compatibility claims; NFS server feasibility; whether a `catalog-api` auth change is acceptable for the installer's SMB routes (I-10).

---

## 18. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Rust build containers need GUI libraries (webkit2gtk) and are large; host memory ceiling | slow or failing builds | build image once, cache layers per 11.4.82, limit parallel jobs per host rules, run heavy builds on the designated build host |
| Tauri test runtime API differs from assumption | rework of 8.2/8.3 | spike in WP-D0 reading the pinned version's source |
| D-01 refuted (an alias exists) | contract test shows no drift | the test stays as a permanent guard; finding closed as false positive with evidence |
| Fixing capabilities breaks the installer's dialogs in unexpected ways | regression | real-window test before and after |
| FTP behaviour differs across servers | flaky tests | pin server image digests; cover two daemons; record versions |
| `ping`/ARP behaviour differs per OS | scan results differ | OS-specific implementation behind one trait; per-OS tests where hosts exist, otherwise blocked-unavailable |
| Large front-end dependency lag (`vite ^4`, `vitest ^0.34`) makes updates break tests | delayed FR-018 | update one major at a time with the full suite per step, decisions recorded |
| Coverage tools for Rust unavailable in the image | no baseline | install in image or record SKIP-with-reason plus tracked item; never invent a percentage |

---

## Appendix A. POC: Rust SSRF validation table test (NOT EXECUTED)

Production code (new function in `catalogizer-desktop/src-tauri/src/main.rs` or a `proxy.rs` module):

```rust
use reqwest::Url;

/// Returns the parsed request URL when it is within the configured server origin and path.
pub fn validate_request_url(server_url: &str, request_url: &str) -> Result<Url, String> {
    let server = Url::parse(server_url).map_err(|e| format!("configured server URL invalid: {e}"))?;
    if !matches!(server.scheme(), "http" | "https") || server.host_str().is_none() {
        return Err("configured server URL must be http(s) with a host".into());
    }
    let req = Url::parse(request_url).map_err(|e| format!("request URL invalid: {e}"))?;
    if !req.username().is_empty() || req.password().is_some() {
        return Err("userinfo not allowed in request URL".into());
    }
    if req.scheme() != server.scheme() { return Err("scheme differs from configured server".into()); }
    if req.host_str() != server.host_str() { return Err("host differs from configured server".into()); }
    if req.port_or_known_default() != server.port_or_known_default() {
        return Err("port differs from configured server".into());
    }
    let base = server.path().trim_end_matches('/');
    let path = req.path();
    let within = base.is_empty() || path == base || path.starts_with(&format!("{base}/"));
    if !within { return Err("path outside configured server base".into()); }
    Ok(req)
}
```

Test table (the oracle is the expected column written from the URL standard and the threat model, not derived from the function):

```rust
#[cfg(test)]
mod ssrf_table {
    use super::validate_request_url;

    #[test]
    fn ssrf_validation_table() {
        // (server, request, expect_ok, row id from section 7.2)
        let rows: &[(&str, &str, bool, &str)] = &[
            ("http://localhost:8080", "http://localhost:8080/api/v1/media/search", true,  "1"),
            ("http://api.example.com", "http://api.example.com.evil.com/steal",     false, "2"),
            ("http://localhost:8080", "http://localhost:8080@evil.example/",        false, "3"),
            ("https://h.example",     "http://h.example/",                          false, "4"),
            ("http://h.example:8080", "http://h.example:9090/",                     false, "5"),
            ("http://h.example:80",   "http://h.example/",                          true,  "6"),
            ("http://H.Example",      "http://h.example/x",                         true,  "7"),
            ("http://h.example/api",  "http://h.example/api/../admin",              false, "8"),
            ("http://h.example/api",  "http://h.example/apix",                      false, "9"),
            ("",                      "http://anything.example/",                   false, "10"),
            ("http://h.example",      "http://169.254.169.254/latest/",             false, "11"),
            ("http://h.example",      "http://h.example/%2e%2e/",                   true,  "13-note"),
        ];
        let mut failures = Vec::new();
        for (server, request, expect_ok, id) in rows {
            let got = validate_request_url(server, request).is_ok();
            if got != *expect_ok {
                failures.push(format!("row {id}: server={server:?} request={request:?} expected_ok={expect_ok} got_ok={got}"));
            }
        }
        assert!(failures.is_empty(), "SSRF table mismatches:\n{}", failures.join("\n"));
    }
}
```

Expected machine-readable output when run with `cargo test ssrf_validation_table -- --format json -Z unstable-options` (the JSON test format needs nightly; UNCONFIRMED) or the plain summary:

```
test ssrf_table::ssrf_validation_table ... ok
test result: ok. 1 passed; 0 failed
```

RED run on the pre-fix artifact: replace the body of `validate_request_url` by the current logic `request.trim_end_matches('/').starts_with(server.trim_end_matches('/'))` returning `Ok(Url::parse(..))`; expected failure lines include rows 2, 3, 6, 7, 8, 9, 10. This is the paired mutation required by 8.1(3). Row "13-note" documents that encoded dot segments are not decoded by the URL parser; what the server does with them is a server-side concern recorded in the register, not asserted here.

Redirect and timeout test (real local server; sketch, NOT EXECUTED):

```rust
// Start a real hyper/axum server on 127.0.0.1:0 returning 302 to http://127.0.0.1:<other>/secret,
// configure server_url to the first server, call the proxy function, and assert:
//   - with the target policy (redirect none) the result is Err("redirect refused" or status 302), and
//   - the second server received zero requests (it records every request it gets).
```

## Appendix B. POC: vitest bridge and route-contract test (NOT EXECUTED)

`catalogizer-desktop/src/__tests__/apiContract.test.ts`:

```ts
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const invoke = vi.fn();
vi.mock('@tauri-apps/api/core', () => ({ invoke }));

import { apiService } from '../services/apiService';

// Provider routes, parsed from the real source of truth (static layer).
function providerRoutes(): Set<string> {
  const src = readFileSync(resolve(__dirname, '../../../catalog-api/main.go'), 'utf8');
  const prefixes: Record<string, string> = { api: '/api/v1', authGroup: '/api/v1/auth' };
  const re = /\b(api|authGroup)\.(GET|POST|PUT|DELETE|PATCH)\("([^"]*)"/g;
  const out = new Set<string>();
  for (const m of src.matchAll(re)) out.add(`${m[2]} ${prefixes[m[1]]}${m[3]}`);
  return out;
}

function matches(routes: Set<string>, method: string, path: string): boolean {
  const clean = path.split('?')[0];
  for (const r of routes) {
    const [m, p] = r.split(' ');
    if (m !== method) continue;
    const rx = new RegExp('^' + p.replace(/:[^/]+/g, '[^/]+') + '$');
    if (rx.test(clean)) return true;
  }
  return false;
}

describe('desktop client calls only registered provider routes', () => {
  beforeEach(() => {
    invoke.mockReset();
    invoke.mockImplementation(async (cmd: string) =>
      cmd === 'get_config' ? { server_url: 'http://h.example', auth_token: 't', theme: 'dark', auto_start: false } : '{}');
  });

  it('every ApiService request URL resolves to a provider route', async () => {
    const routes = providerRoutes();
    expect(routes.size).toBeGreaterThan(20); // control needle: the parser can see the provider file
    const calls: Array<() => Promise<unknown>> = [
      () => apiService.login({ username: 'u', password: 'p' } as never),
      () => apiService.searchMedia({}),
      () => apiService.getMediaById(1),
      () => apiService.getMediaStats(),
      () => apiService.getMediaUrl(1),
    ];
    const bad: string[] = [];
    for (const call of calls) {
      invoke.mockClear();
      await call().catch(() => undefined);
      const req = invoke.mock.calls.find(c => c[0] === 'make_http_request');
      const { url, method } = req![1] as { url: string; method: string };
      const path = new URL(url).pathname;
      if (!matches(routes, method, path)) bad.push(`${method} ${path}`);
    }
    expect(bad).toEqual([]); // expected RED today if D-01 is real
  });
});
```

Expected output while D-01 stands (illustrative, not observed): `AssertionError: expected [ 'POST /api/auth/login', 'GET /api/media/search', ... ] to deeply equal []`. The `routes.size > 20` assertion is the control needle (11.4.201(7)(b)): a zero from an unreadable provider file cannot pass silently. This static test is the cheapest contract layer; the dynamic layer against a real `catalog-api` (section 11.2 step 4) is still required.

## Appendix C. POC: refused-port and protocol-tester RED tests (NOT EXECUTED)

Refused connection judged alive (I-01), in `installer-wizard/src-tauri/src/network.rs` tests:

```rust
#[tokio::test]
async fn closed_local_port_is_not_reported_open() {
    // Reserve a free port, then close the listener so connects are refused.
    let port = { let l = std::net::TcpListener::bind("127.0.0.1:0").unwrap(); l.local_addr().unwrap().port() };
    let ip = std::net::Ipv4Addr::new(127, 0, 0, 1);
    let res = tokio::time::timeout(
        std::time::Duration::from_millis(200),
        tokio::net::TcpStream::connect((ip, port)),
    ).await;
    // Current production logic: `res.is_ok()` -> true even though the inner connect failed.
    assert!(res.is_ok(), "precondition: timeout did not elapse (connection was refused fast)");
    assert!(res.unwrap().is_err(), "the inner result is the refusal that production code ignores");
    // The real test after extraction of `fn port_open(ip, port) -> bool`:
    // assert!(!port_open(ip, port).await);   // RED before the fix, GREEN after
}
```

(`unwrap()` is acceptable inside tests under the module's rules; the production fix uses `matches!(res, Ok(Ok(_)))`.)

FTP CRLF injection test shape (I-04), against a real FTP daemon container whose command log is the oracle:

```
given:  vsftpd (or equivalent) container, log of received commands exposed as a volume
when:   test_ftp_connection(host, 21, "u\r\nDELE x", "p", None)
then:   Err(...) returned before any byte is sent; the daemon log contains no "DELE"
RED:    on the current code the daemon log contains "DELE x"
```

WebDAV false positive (I-07):

```
given:  WebDAV container that answers 401 with header "Content-Length: 1200" for bad credentials
when:   test_webdav_connection("http://host/", "bad", "bad", None)
then:   Err("authentication failed")
RED:    current code returns Ok(true) because the raw response contains "200"
```

## Appendix D. Container command forms (NOT EXECUTED)

The image reference is configuration data supplied by the Containers submodule (11.4.28, 11.4.173); digests are recorded per run. `${RUST_TAURI_IMAGE}` is built once from a Containerfile that installs the Tauri 2 Linux prerequisites (GTK and WebKitGTK development packages, the same package names that `installer-wizard/install_sys_dependencies.sh` lists; verify against the current Tauri 2 prerequisites page at build time) plus the detector tools.

```bash
# rootless podman; host user namespace; no sudo; read-only source mount, writable target dir
podman run --rm \
  --userns=keep-id \
  --memory=8g --cpus=4 \
  -v "$PWD":/work:ro,Z \
  -v "$PWD/.audit/09/target-desktop":/work/catalogizer-desktop/src-tauri/target:rw,Z \
  -w /work/catalogizer-desktop/src-tauri \
  "${RUST_TAURI_IMAGE}" \
  sh -c 'cargo clippy --all-targets --message-format=json -- -D warnings \
         > /work/.audit/09/clippy-desktop.json'
```

Resource caps follow the host rules (memory ceiling, process limits); the numbers above are placeholders to be derived from measured host capacity per the build-resource policy, not fixed values (`UNKNOWN:` host capacity at execution time). The `:ro` source mount with a separate writable evidence directory keeps detector runs from modifying the tree. The same form runs `cargo test`, `cargo audit`, `cargo deny`, `cargo llvm-cov`, `npm ci && npx vitest run --coverage`, and `npx tsc --noEmit`.

Real-target containers for section 9 are started through the Containers submodule's compose/boot mechanism with pinned image digests and torn down after each run; their container logs are collected as oracle evidence.
