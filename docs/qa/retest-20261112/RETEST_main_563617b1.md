# Retest main @ 563617b1 (clean git archive; compare v2.4.1 a90896a3)
Method: git archive main + tag v2.4.1 under mktemp root; submodule dirs are empty in git archive, so working trees
of the live checkout's submodules (identical pins v2.4.1..main, 0 gitlink changes) were rsync'd (no .git) into the archive.
Deviations: systemd-run scope (MemoryMax=5G) got SIGTERM'd twice mid-run (cause UNKNOWN), so later Go batches ran unscoped
under nice -n 10, GOMAXPROCS=2, -p 2/-p 1. Host load avg 10-18 throughout (other workloads). Go cache dir private.

## catalog-api (go test ./... , 60 pkgs, 26 split into batches; no-test pkgs excluded)
Run 1 (full): ok x31+, FAIL: catalogizer (root), catalogizer/config, internal/nilslice_guard; remainder re-run in batches.
Final per-package failures on main:
- catalogizer/config TestDocsAdminPasswordExamplesAreAllRejected -> also FAILS v2.4.1 => PRE-EXISTING
- internal/nilslice_guard TestNoUnnormalisedNilListReturns (admin_handler.go:121, collection_handler.go:301, user_repository.go:295, smb_hostvet.go:111) -> FAILS v2.4.1 => PRE-EXISTING
- catalogizer (root) TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions (DELETE /api/v1/logs/share/999999 expected 500 got 404) -> FAILS v2.4.1 => PRE-EXISTING. Re-run alone on main: only this test fails (513s). Run-1 extra "panic: test timed out 10m / TestSSRF_RealBinary_SyncEndpointURL" = load-induced timeout; not reproduced (passes in full re-run with -timeout 25m).
- internal/tests/examples TestUserRepositoryExample (nil-pointer panic) -> FAILS v2.4.1 => PRE-EXISTING
- services TestCAT002_Iso_LimitsAreHostAdaptiveAndBounded -> FAILS v2.4.1 => PRE-EXISTING
- tests TestConversionIntegration -> FAILS v2.4.1 => PRE-EXISTING
- tests/stress TestCacheStress_HighThroughput/CacheWriteThroughput: FLAKY under load (main: 2 pass / 1 fail, 74 ops/s; v2.4.1 pass) => load-dependent, not a regression
Passing: all other packages (incl. handlers, internal/*, repository, middleware, models, pathguard 157s, tests/integration, tests/security, smb, utils, database).
Regressions: NONE found.

## catalog-web (vitest run --pool=threads --maxWorkers=2)
 Test Files  8 failed | 162 passed (170)
      Tests  14 failed | 2585 passed | 1 todo (2600)
Failing = exactly the known set: module-registry (7), AuthContext.integration (7), load failures: Avatar, EmptyState, LoadingSpinner,
WebSocketContext, WebSocketContext.connectionStatus, websocket. Same 8 files / 14 tests fail on v2.4.1 (reproduced). REGRESSIONS: NONE.

## SKIPPED (untestable here)
- catalogizer-desktop, installer-wizard vitest: no node_modules present, no installs allowed.
- cargo/Tauri unit tests: cargo/rustc not installed.
- Live stacks (catqa*/helix*), Synology, credentials: not run per instructions.
Cleanup: node_modules symlinks removed, temp root deleted. No repo modifications.
