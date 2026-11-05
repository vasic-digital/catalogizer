# CAT-TAURI-1: Tauri desktop + installer-wizard boot for manual QA (2026-11-05)

## VERIFIED (this session, captured output)
- Rootless podman image `catalogizer-tauri:qa` built from `docker/Dockerfile.tauri` (base `docker.io/library/debian:bookworm-slim` sha256:f3034a6e..ad782b, Rust 1.90.0 via rustup, Node 20.19.5, libwebkit2gtk-4.1-dev; image 2.88 GB). Docker Hub hit its unauthenticated pull rate limit; the already-present local debian:bookworm-slim was used (no remote pull needed).
- `scripts/tauri/build.sh` built **catalogizer-desktop** in the container (`tauri build --no-bundle`, rustc 1.90.0, 6m08s, jobs=3):
  `catalogizer-desktop` ELF x86-64 16379216 B, sha256 `0e7cee0ec3d9f92edd1c1dd44c992d39d295182a790d722d55dc28ac1e261eca`.
  CAVEAT: that binary embeds the frontend from BEFORE the login fix below; it must be rebuilt.
- Browser path (vite QA servers, real React UI + QA-only IPC shim, real API): desktop on :1420, wizard on :1421 (0.0.0.0).
  Headless Chromium (`drive_ui.cjs`, result.json): login screen rendered; qa_admin login -> `POST /qa-api/api/v1/auth/login 200`, home page with real data
  ("Total Items 47", user qa_admin@catalogizer.local), `GET media/stats 200`, `media/search 200`. Wizard first screen renders (title "Configure your media storage", no page errors).
- Root-cause defects found and fixed (catalogizer-desktop): login could never work in the shipped app: (1) `authStore.serverUrl` is never populated, so the URL became `null/api/auth/login`
  and the Rust SSRF check rejects it; (2) client used `/api/...` but catalog-api serves `/api/v1/...` (404); (3) login response field is `session_token`, client read `token`.
  RED: 3 vitest failures before fix; GREEN: authStore 26/26, services+stores 138/138. Mutations (drop session_token fallback; use store URL again; double v1 on entity path) each killed by a test.
  Full desktop suite: 13 failures (localStorage in one file) are pre-existing, identical with and without the change (13/434 baseline, 13/436 with).

## NOT done / UNCONFIRMED
- Wizard Rust build: started, killed at conductor request (load 41, / at 98%) before finishing; no wizard binary.
- Desktop binary has not been booted: `scripts/tauri/run-qa.sh` (Xvfb + x11vnc + noVNC in the container) is written but NEVER RUN; VNC route UNCONFIRMED.
- Desktop binary needs a rebuild to include the login fix.
- Shim limits: the wizard's native commands (network/SMB scan, file dialogs) are answered with empty values; the browser run does not prove the Rust commands.
- Entity progress calls still 404 in the earlier run only because of a doubled `/v1` that was then fixed; not re-driven live after that fix (UNCONFIRMED live).

## How to run
- Build (container, ~6-12 min, needs ~2.5 GB disk): `scripts/tauri/build.sh [OUT]`
- Browser QA (LAN): `cd catalogizer-desktop && npm ci && QA_PORT=1420 QA_API_URL=http://127.0.0.1:38082 npx vite --config vite.qa.config.ts` -> http://192.168.1.115:1420 ; `cd installer-wizard && QA_PORT=1421 npx vite --config vite.qa.config.ts` -> :1421. Login qa_admin (password in the QA env file).
- Driver: `( set -a; . qa.env; set +a; node docs/qa/tauri-boot-20261105/drive_ui.cjs <dir containing @playwright/test> <outdir> http://192.168.1.115:1420 http://192.168.1.115:1421 )`
- Native under VNC (unverified): `QA_BIND=192.168.1.115 scripts/tauri/run-qa.sh desktop` -> http://192.168.1.115:6080/vnc.html, password in `<OUT>/vnc.pass.txt` (0600). The app starts on Settings; enter server URL http://192.168.1.115:38082.

Screenshots are gitignored (`docs/qa/**/*.png`); hashes in MANIFEST.screenshots.sha256, files in /home/milosvasic/.cache/claude-tmp/evidence-archive/tauri-boot-20261105/.
