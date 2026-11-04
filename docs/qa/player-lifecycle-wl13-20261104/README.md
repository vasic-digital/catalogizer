# WL13 player lifecycle (F1, F2, F3): real-browser evidence, 2026-11-04

Spec: `catalog-web/e2e/player-real/player_lifecycle.spec.ts` (real `MediaPlayer`, real `<video>`, chromium and firefox, local ticket
server; NOT live-API evidence). "Near expiry" (45 s client-side ticket age) is produced with `page.clock.setSystemTime`.

| File | What it shows |
|---|---|
| `red_main_chromium.txt`, `red_main_firefox.txt` | Spec run against unmodified main (d4a3dbd6 source). F2 and F3 FAIL on both browsers with the right reason (F2: ended at t=45 instead of near seek 2; F3: auto-played after the end). The F1 FAIL in these two logs is NOT valid evidence: that first draft of the F1 test clicked Pause only after the controls had auto-hidden (click at ~27 s, after the media ended), so it failed for a test reason. |
| `red_f1_revert_chromium_firefox.txt` | Corrected F1 test with the one-line F1 fix reverted (`if (r.play) ...`, the main behaviour): FAIL on both browsers, `t=45, paused=false` (the Pause was overridden). |
| `green_final_both_browsers.txt` | Fixed source: 12 passed (4 new + 2 existing recovery tests, x 2 browsers). |

Reviewer-style mutations (each run on both browsers, each FAILS): F1 `r.play && wantPlayRef.current` -> `r.play`; F2 `seekDuringMintRef.current = t`
-> `??= t` (first target kept); F3 delete `wantPlayRef.current = false` in `handleEnded`.

Decision (F1, OS media key): during a reload `load()` has already paused the element, so `pause()` is a no-op and no `pause` event fires
(probe test, both browsers). The player cannot tell a media-key pause from "still reloading", so it is not honoured; the in-page Pause
button is. Honouring it would need a MediaSession `pause` handler (not in scope).
