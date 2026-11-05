# Stack 3 rebuild + live re-verification of the security queue and monitoring add-on (2026-11-05)

Base: main 3293e280. Authoring commits on branch `qa/stack3-verify-20261105` of a scratch clone (head recorded in the hand-back, never pushed).
Stack 3 was rebuilt from the clone head 015896fd89180a42cc6c1e794719bd04e39e9c40 (the first commit of this branch; later commits only touch docs/qa/stack3 scripts, not the image):
`/health` on the running API returns `build_number`/`version` = that sha (health_final.json), api image `sha256:54d8cc6b...` (image_ids.txt). The target fingerprint was read from the target, not asserted.
Pods: `catqa3` (rebuilt, volumes kept) and `catqa3mon` (rebuilt). Stacks `catqa`/`catqa2` and the shared aardvark-dns were not touched. Root disk stayed >= 25 GiB free; host load was 20-45 from other workloads during the run (not from this work: builds ran under nice and a 5 GiB cgroup cap).

## URL / port table (host 192.168.1.115)
| Service | Bind | Status (url_status_final.txt) |
|---|---|---|
| catalog-api | 0.0.0.0:38082 | /health 200 |
| catalog-web | 0.0.0.0:33020 | 200 |
| Website | 0.0.0.0:33021 | 200 |
| PostgreSQL / Redis | 127.0.0.1:35434 / 127.0.0.1:36381 | loopback |
| nginx front door (web + /api + /ws) | 0.0.0.0:33080 | /edge-health 200 |
| nginx front door (website) | 0.0.0.0:33081 | 200 |
| Grafana / pgAdmin / Prometheus | 127.0.0.1:33030 / 33050 / 39090 | 200 / 200 / 200 |
Credentials: `~/.cache/claude-tmp/catqa3/{qa,mon}.env` (0600); a value scan of this directory found no leak.

## Changes made (docs/qa/stack3 only; no product code)
1. QA-MON-FU-1: stack-3 `up.sh` generates `METRICS_SCRAPE_TOKEN` (64 hex, 0600 `qa.env`; appended to a legacy `qa.env` without touching other lines) and passes it to the API by name. `monitoring/up.sh` now seeds it as Prometheus' credentials file (`seed_scrape_token.sh`, token on stdin) and starts NO token-refresher / admin session (refresher kept as fallback only when the token is absent). Live: no `tokenrefresh` container; Prometheus target `up` (prom_targets_final.txt).
2. TRUSTED_PROXIES: `lib.sh` default = the host's primary address as /32 (measured below). Loopback is deliberately NOT trusted (first attempt with 127.0.0.1/32 added measured a forgery path; see clientip_probe_after.txt vs clientip_probe_final.txt).
3. Optional `HELIX_PPROF_ENABLED` pass-through (by name) so pprof auth could be tested live; default stack has it unset.
RED first: red_stack3_test_scripts.txt (4 FAIL) and red_monitoring_test_scripts.txt (4 FAIL) before the change; green afterwards (stack3 test_scripts.sh 15/15, monitoring test_scripts.sh all PASS). The pprof pass-through test (last two stack3 tests) was added together with the change, no separate RED was captured for it.

## Results per item
Legend: VERIFIED = live machine evidence on the rebuilt stack at the artifact fingerprint above; PROVISIONAL = not (fully) provable live here; REOPENED = contrary evidence. Promotion itself is the conductor's / independent reviewer's call (11.4.142); this table is the author's evidence statement.

| Item | Status | Evidence (N = identical repeats) |
|---|---|---|
| QA-MON-FU-1 static scrape token (no admin session in Prometheus) | VERIFIED | Prometheus target up with static token, no refresher container; `/metrics` anon 401, bogus 401, scrape token 200 (27 KB, go_goroutines present), off-by-one token 401, scrape token rejected on admin/users, media/stats, auth/profile; run_A1..A3 (N=3, 27 checks each, 0 FAIL); monitoring verify.py N=2 45/45 PASS (monitoring_verify_run1/2.txt); real-binary Round2 MetricsScrapeToken / ShortScrapeTokenIgnored / ScrapeTokenDoesNotOpenPprof N=3 PASS |
| /metrics requires auth (route perms) | VERIFIED | same as above |
| pprof requires admin (SECQ gate) | VERIFIED | stack restarted twice with HELIX_PPROF_ENABLED=true (up_pprof*_output.txt), then restored (api env has no HELIX_PPROF, api_env_names.txt): anon 401 on 4 paths, scrape token rejected, plain user 403, admin 200; run_B1..B3 (N=3, 9 checks, 0 FAIL); with the flag unset it is 404 (A runs) |
| Cross-user authz | VERIFIED (tested routes) | second user created by register + admin approval: /metrics not 200 (403), admin/users 403, generate-thumbnails 403, admin conversion jobs 1,2,150 not readable, pprof 403; admin controls 200; user logout leaves admin session valid (SECQ-6 cross-token, live); N=3. Real-binary SelfOrAdmin, CollectionOwnership, AdminGate, ObservabilityAdminOnly N=3 PASS. Not covered: every route (route enumeration is a separate suite) |
| CSRF-R2-1 / R2-2 cookie | VERIFIED | stack runs APP_ENV=development + CSRF_INSECURE_DEV=true: `csrf-dev` cookie, HttpOnly, no Secure, no `__Host-` prefix; mutating request without cookie 403, without header 403, wrong token 403, valid pair passes the guard (404 for the nonexistent user); N=3; real-binary EnvToggleReachesMiddleware N=3 PASS. The browser-level CSRF-1 check (real browser UI) was NOT run |
| P2-g access-log redaction | VERIFIED | `GET /api/v1/smb/test?...password=<sentinel>&pass=&secret=&token=&keep=VISIBLEVAL`: API log shows `password=REDACTED&pass=REDACTED&secret=REDACTED&token=REDACTED&keep=VISIBLEVAL`; sentinel absent from the whole log window; control needle (keep=) present; N=3. The GET forms still accept credentials (SECQ-5 stays open). A first attempt of this check was vacuous (log window parse returned 0 lines, the control needle caught it) and was fixed before the recorded runs |
| QA-MON add-on (Prometheus/Grafana/pgAdmin/nginx) | VERIFIED | verify.py 45 PASS / 0 FAIL, twice, >= 1 min apart |
| Existing feature suite D1-D11 (regression) | VERIFIED (no regression) | live3.py N=3: 54 PASS / 0 FAIL / 1 SKIP each (live3_run1..3.txt). Note: my first attempt had 3 FAIL (D5) because I had deleted the generated Burst.Movie fixtures; fixtures regenerated (ffmpeg, same expression file) and the harness lib.py rebuilt (it was not tracked) |
| SSRF-F1 SOCKS5 handshake bound | PROVISIONAL | real-listener test passes N=3 inside the api image (cover_services_run1..3.txt: Socks5HandshakeIsBoundedByDialTimeout, ProductionDefaultIsBounded) but there is no live stack route (needs a SOCKS5 proxy configured); not exercised on the stack |
| SSRF-F2..F5 cover-art ffmpeg/ffprobe hardening | PROVISIONAL | real ffprobe/ffmpeg tests PASS N=3 in the api image (PlaylistToLocalFileIsRefused, PlaylistToLocalFileMakesNoThumbnail, DirectURLInputDoesNotReachNetwork, GuardArguments, MaxBytes default 32 MiB, hung tool bounded); control needle in the stack image: unguarded `ffprobe` follows an HLS playlist (10.000000), the production flags refuse it ("Format not on whitelist") and still read a plain mp4 (5.000000) (ffprobe_*.txt). The live API path (admin batch thumbnail endpoint) could not be used: see NEW-1. 21 other CoverArt tests fail only because the test binary was built CGO_ENABLED=0 (sqlite3 driver missing), none of the security tests |
| SSRF-F6 doc census | n/a | documentation only |
| AZ2-3 standalone Prometheus listener loopback | PROVISIONAL | real listener test PASS N=3 (unit_monitoring_run*.txt); `main` does not call it, so there is nothing to observe on the stack |
| R4-1 WebDAV client rebuilt on credential rotation | PROVISIONAL | not run here (needs a WebDAV endpoint); unit-level evidence is the earlier security-queue pack |
| TRUSTED_PROXIES at the nginx front door | PROVISIONAL (partial) | wiring VERIFIED: API env has TRUSTED_PROXIES=192.168.1.115/32. Measured with pasta: a request through the nginx front door AND a direct host-local request both reach the API with the host's LAN address as peer. Consequence (contrary evidence, honest): a client on the SAME host can still forge X-Forwarded-For (9.9.9.9 was logged, clientip_probe_final.txt) because its address is indistinguishable from the proxy's; with 127.0.0.1/32 added the same forgery also worked. UNCONFIRMED: that a remote LAN client keeps its own address and cannot forge (no second LAN machine was available). |

REOPENED: none of the listed items. NEW defect found, not fixed (outside this task's scope; needs a RED test and a decision):

NEW-1 (cover-art batch thumbnail generation never generates anything): `POST /api/v1/admin/covers/generate-thumbnails` returns `{"generated":0}` in 0.0 s with no log line on this PostgreSQL stack although 11 video items exist (cover_run1.txt, cover_rows.txt: cover_art has 0 rows). Cause (read from code and SQL, not instrumented at runtime, label UNCONFIRMED until a debug run): `GenerateMissingVideoThumbnails` (internal/services/cover_art_service.go:1255-1285, since commit 58cb30b8 2026-10-08) selects `models.StorageRootConnColumnsFor("sr")` = protocol, host, port, path, username, password, domain, url, mount_point, options, but scans into `&t.protocol, &t.rootPath, &t.host, &t.port, ...` (path/host swapped, "/media" cannot scan into the int64 port), and the scan error is swallowed (`err == nil` guard); in addition the query builds `f.path || '/' || f.name` while `files.path` already holds the file name (new1_query_columns.txt: `movies/X.mp4/X.mp4`). Effect: the endpoint is a silent no-op, so SSRF-F2..F5 cannot be exercised through the product path and the feature itself is broken. Recommend: file as a Bug, reproduce-first (this run is the live RED), fix, then re-run the cover-art leg of this pack.

Residue left on the stack (not removed, as before): media rows for the generated fixtures (media items 18-20: live3 Burst movie + notes, sec3 Plain.Control / Hls.Evil / Control.Plain), the `secq_*`/`secp_*` users were deleted, conversion jobs were deleted by the harness; fixture files on disk (media-corpus/sec3, live3) were removed. Test binaries ran with `CGO_ENABLED=0` for the unit packages and in a golang:1.26 container (host-local, not the remote build host: 11.4.173 deviation as already documented in stack3 README).
