# SEC-WS-1 Reproduction record (§11.4.292(C)) - PROVISIONAL

- Result: reproduced-host-fixture (host-only code path; real handler stack, real HTTP server, real websocket client; no device).
- Surface: catalog-api `GET /ws` route as wired by `registerWebSocketRoute`, with production `newAuthMiddleware` over a migrated in-memory SQLite DB.
- Pre-fix state: commit 541d4765 (route `router.GET("/ws", h.HandleConnection)`, `CheckOrigin` returns true). Built by introducing the helper in its pre-fix form (no `RequireAuth`) and running the new tests unchanged, plus `AllowedOrigins` field present but ignored.
- Command: `cd catalog-api && go test -p 1 -parallel 2 -count=1 -run 'TestWSRoute|TestMainGoMountsWebSocket' .` (throttled: systemd-run MemoryMax=3G, nice -n 10, GOMAXPROCS=2).
- Symptom signature: `An error is expected but got nil` for unauthenticated, bad-token, revoked-session, suspended-user and foreign-Origin dials (upgrade 101 completed).
- Round-2 defect (reviewer F1): same-host check compared Origin host:port with a Host header that nginx `proxy_set_header Host $host` strips of the port; reproduced in `TestWSRoute_BehindReverseProxyHostForwarding/Host_$host` (403 for Origin http://127.0.0.1:<port>).
- Evidence sha256:
  - 7e22363dd85dc0d5e88bed9868f824389984ec17d6a186d94713a58cfb9f41cd  red.txt
  - 0488e7833f1d55308a336c0a23ed9404b781a8ce00ae57ba5d05df06a7e47a75  green.txt
  - 30afa4f82d9620cb5e3e1d4b298f672177a6812db6f334d312d43f8ba9bff965  green_round2.txt
  - 2f9894f02f3392d0e40bb4e04dcff7c6aabfccb5185c1183494503e8c66dd01b  mutants.txt
  - 26d9af38195a148fabc5ba4d07e9dec104c4e1b7569da290200aca8ce579cee0  mutants_round2.txt
- Not live: no real browser/Android/nginx run (owed, SEC-WS-1-F1). Root cause established by code reading plus the failing tests above, not by a device experiment.
