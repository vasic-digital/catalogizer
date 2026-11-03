# Responsive audit harness

Real-browser (Playwright chromium/firefox/webkit) layout oracle over every catalog-web, Website, installer and desktop page x viewport x theme x dpr x reduced-motion, against the real built app and the real catalog-api (no mocks).

- `oracle.mjs`: in-page layout oracle (horizontal overflow, off-screen / occluded controls, tap targets < 44px, text < 12px, clipped and overlapping text, off-viewport text).
- `needle.mjs`: control needle. Feeds the oracle a known-bad and a known-good page; the audit is only valid when it prints `ORACLE_VALID`.
- `run.mjs --app=catalog-web|website|installer|desktop --browser=chromium|firefox|webkit [--only=page,page] [--append]`: writes `out/verdicts-<app>-<browser>.jsonl` (one machine-readable verdict per case, with screenshot path and sha256).
- `serve.mjs <dir> <port> [apiPort] [spa]`: static server with SPA fallback and /api + /ws proxy to a real catalog-api.
- `summarize.py`: aggregates the verdicts.

Needs `playwright-core` matching the installed browsers (1.61.0 for chromium-1228 / firefox-1532 / webkit-2311), `out/` and `run/secrets.env` (ADMIN_USERNAME / ADMIN_PASSWORD, never committed) in the parent of the working directory.

Additional flags: `--tag=<name>` (suffix for verdict file / screenshot dir), `--vps=320,375,...` (restrict viewport widths), `--base=<url>` (audit another build), `--authed` (desktop only: emulates ONLY the Tauri IPC bridge, `get_config` -> real API URL + real session token; all HTTP still goes to the real catalog-api).

Oracle rules worth knowing: visually-hidden / sr-only / aria-hidden / inert elements and controls inside a closed off-canvas fixed drawer are ignored; under mobile emulation the intended viewport width is passed in (`window.innerWidth` grows to fit overflowing content and would hide the overflow); a run is only valid after the splash screen is gone (`splash_stuck` otherwise).

Scope notes: the desktop app declares `minWidth: 800` in `tauri.conf.json`, so desktop findings below 800px are outside the supported window range; the installer wizard declares a 1000x700 default window.

## Running against a local build (no fixed ports or paths)

`run.mjs` reads its endpoints and work directory from the environment, so it can audit any build against any real catalog-api:

| Variable | Meaning | Default |
|---|---|---|
| `AUDIT_API_URL` | real catalog-api base URL (login + `/api` proxy target) | `http://127.0.0.1:18400` |
| `AUDIT_WEB_URL` | base URL of the catalog-web build under test | `http://127.0.0.1:18401` |
| `AUDIT_WORK` | work directory holding `out/` and `run/secrets.env` | parent of the working directory |

`--matrix=followup` selects the 2026-11 follow-up matrix: 360x640, 390x844, 768x1024, 1024x768, 1440x900, 1920x1080, each in BOTH light and dark in every browser (the default matrix runs dark only in chromium plus a few key widths elsewhere).
Example (build served with `serve.mjs <dist> <port> <apiPort> 1`):

```
AUDIT_WORK=/path/work AUDIT_API_URL=http://127.0.0.1:18500 AUDIT_WEB_URL=http://127.0.0.1:18502 \
  node run.mjs --app=catalog-web --browser=firefox --matrix=followup --tag=after
```

The `website-pages.json` file is only read for `--app=website`.

## Targeted measurement: `measure_d1_d2.mjs`

Real-browser check of the connection badge (must not intersect any other visible interactive element or text box) and of the target-size floor (44px coarse, 24px fine) on dashboard, collections, media, playlists, admin and settings at 320/375/768/1024/1440, light and dark. Needs the same env as `run.mjs` (`AUDIT_WORK`, `AUDIT_API_URL`, `AUDIT_WEB_URL`); `--browser=`, `--pages=`, `--shots=<dir>`, `MEASURE_OUT=<file.json>`. Exit 0 only when every case passes and the badge was observed in every case (an unobserved badge is invalid, never clean). Without a real API the badge never appears, so the run is `UNOBSERVED`.
