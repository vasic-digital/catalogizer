# UI-D7-FU (remaining D7 contrast), PROVISIONAL until live axe

- Test: catalog-web/src/styles/__tests__/contrastD7Remaining.test.tsx (real Tailwind theme + real component classes).
- Correction of the first commit's claims (review F6): the first RED run was 6 failed / 4 passed (not 8/10), and the test checks every ELEMENT THAT DIRECTLY HOLDS TEXT (not every text node); elements inside svg and over translucent black overlays (bg-black/N, image-dependent) are skipped; large text (3:1) is text-2xl+ or text-xl font-bold only.
- unit_red.txt / unit_green.txt were regenerated with the corrected test (all five /ai sections awaited after lazy load, performance-report slowest-endpoints row rendered): RED against pre-fix sources 6 failed / 4 passed, GREEN 10 passed.
- Not covered: hover/focus states and non-text (icon-only toggle) contrast (follow-up); live axe re-run on a rebuilt stack.
