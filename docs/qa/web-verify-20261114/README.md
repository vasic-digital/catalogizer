# PW-F4 and FU2-1 verification (2026-11-14, PROVISIONAL; corrected after Opus round 1)

Scratch clone of main 6b67141ae, 9 catalog-web submodules initialised from local paths, submodule dist built with `npm ci && npm run build` in dependency order (as scripts/container-local.sh does), then `build_catalog_web` from scripts/lib/build-catalog-web.sh was run with BUILD_SKIP_TESTS=true via `build_catalog_web_driver.sh.txt` (exit 0; `pwf4_build_tail.txt`; file hashes `pwf4_dist_sha256_script_build.txt`).

## PW-F4 (resolve.dedupe, production build)
Reproducer: `build_variant.mjs` (run with catalog-web as cwd), three variants, each checked by `assert_single_copy.py` (reads the source maps only, so it sees only modules present in them; control needles: known-present react found, known-absent fake not found, no maps = exit 2 BLIND). Effective dedupe printed by Vite per variant: `pwf4_effective_dedupe_printout.txt`.

| variant | EFFECTIVE dedupe | sources | react copies | other 3 pkgs | assert |
|---|---|---|---|---|---|
| project (as committed) | react, react-dom, @tanstack/react-query, axios, react, react-dom | 1058 | 1 | 1 each | PASS |
| project dedupe emptied | react, react-dom (added by @vitejs/plugin-react 4.7.0, vite:react-refresh) | 1058 | 1 | 1 each | PASS |
| effective none (plugin's stripped too) | [] | 1080 | 7 (catalog-web + 6 submodule node_modules) | 1 each | FAIL (exit 1) |

Non-map output hashes per variant: `pwf4_nonmap_sha256_*.txt`; project vs project-dedupe-empty are identical, project vs effective-none differ in 44 file pairs.

Finding (this corrects the first commit, which wrongly said dedupe has no effect): dedupe as a whole is load-bearing; with none, react is bundled 7 times. The project's explicit `react`/`react-dom` entries are redundant with the plugin's own dedupe. For `@tanstack/react-query` and `axios` the single copy holds even with all dedupe removed; why is UNCONFIRMED (auth_context_react/dist/AuthContext.js imports react-query and submodules/auth_context_react/node_modules/@tanstack/react-query exists). No recommendation to remove dedupe.

Deviations: build-catalog-web.sh runs npm on the host (no container step); the container path is scripts/container-local.sh web-build, not run (no node image locally). A clean clone's script build fails until submodule dists exist (vite: Failed to resolve entry for @vasic-digital/auth-context); the script does not build them.

## FU2-1 (HistoryDrawer paint order)
`fu2_1_paint_order.mjs` mounts the real HistoryDrawer ALONE (not via the MediaGrid/ProgressBadge click-through) with the real Tailwind CSS under a vite dev server in headless chromium 1228 (PW_CHROMIUM; bundled playwright wants 1200). It has no backend, so the drawer's API calls fail (caught) and paint order was checked with empty-state content only. It asserts elementFromPoint at the dialog centre and the title centre is inside `[role=dialog]` and not the backdrop; two controls also gate the verdict (a probe under the overlay must NOT be hit at its centre; a fake id must be absent). 3 runs: identical JSON (`fu2_1_run*.json`, screenshot sha256 644e92...). `--mutate` (dialog z-index -1): the hit is the backdrop button, exit 1 (`fu2_1_mutated.json`). The mutated screenshot is pixel-identical (transparent backdrop), so only hit testing discriminates. Run with catalog-web as cwd. No defect found.
