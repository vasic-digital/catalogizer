splashfix 2026-11-17 PROVISIONAL
Root cause: Node 26.8.1 native Web Storage global; its localStorage getter returns undefined without --localstorage-file ("ExperimentalWarning: localStorage is not available"). Vitest jsdom env does not override the existing global, so localStorage/sessionStorage are undefined in tests. Component SplashScreen.tsx is correct (real webview has Storage); the test environment is wrong.
Fix: setup files of catalogizer-desktop (src/test-utils/setup.ts) and installer-wizard (src/test/setup.ts) install an in-memory Storage only when the global is unusable. No assertion changed, component unchanged.
RED: red-*.txt (13/13 fail each). GREEN: green-full-*.txt (desktop 440/440, wizard 457/457), tsc-*.txt rc=0.
Mutant: mutant-desktop.txt (store 'false' + invert first-launch test) -> 3 of 13 fail.
catalog-web RegisterForm flake: not reproduced (regform-runs.txt); needs initialised submodules to run at all. Unchanged.
