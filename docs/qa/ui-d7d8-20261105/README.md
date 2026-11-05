# UI-D7 / UI-D8 (CAT-UI-D7D8), 2026-11-05

Live RED on QA stack 3 (image main-5c480072+) before the fix. Credentials never recorded (read from the 0600 env file in a subshell).

- D7: `d7_axe.cjs` (axe color-contrast + svg-img-alt, light and dark, 9 app pages) -> `d7_live_red.txt` (readable) / `d7_live_red.json`. Fixed here: Layout version label (2.65:1 light / 3.29:1 dark), Dashboard stat labels and scan rows (3.03:1 dark, 3.3:1 light), ActivityFeed hint text (2.53:1 light). Unit ratios computed from the real Tailwind theme match the live axe ratios (2.66 vs 2.65, 2.54 vs 2.53, 3.30, 3.04): `d7_unit_red.txt`.
- NOT fixed here (owned by UI-D5/D6 or inside forbidden files): svg-img-alt on dashboard/analytics pie charts (MediaDistributionChart, Analytics cells); /ai green/yellow badges and tabs (AIDashboard); /admin dark cards and tabs (AdminPanel); /media card text (MediaCard); `.text-center:nth-child(2)` dark on dashboard was ActivityFeed (fixed).
- D8: `d8_repro.cjs` -> `d8_live_red.json`: header search for a nonsense term navigates to `/browse?search=...`; Browse shows the unfiltered type grid, no entity query carrying the term is issued. Root cause: Header sends `search`, EntityBrowser reads `q`. Unit RED: `d8_unit_red.txt`.
