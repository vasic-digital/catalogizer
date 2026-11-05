# CAT-UI-D5D6: /admin 360px overflow + unnamed controls (axe)

RED (live, QA stack 3 image main-5c480072, qa_admin, headless Chromium + axe-core): `red-live-axe.json` produced by `axe_live.cjs`
(BASE/U/P env; password read from the 0600 env file in a subshell, never stored).
RED counts: /media button-name 78; /collections button-name 2 + select-name 2; /playlists button-name 2 + select-name 4;
/favorites select-name 2; /ai button-name 1; /dashboard svg-img-alt 6; /analytics svg-img-alt 6; /admin @360px scrollWidth 618 (client 360), cause: 40-char commit hash in the Version stat card.

GREEN live re-run plan (after conductor rebuilds the web image): `NODE_MODULES_DIR=<dir with playwright-core+axe-core> [CHROMIUM_PATH=<chrome-headless-shell>] BASE=... U=... P=... node axe_live.cjs`; expect every list empty
and admin360 sw == cw == 360. Not run here (stack 3 must not be rebuilt by the author).
