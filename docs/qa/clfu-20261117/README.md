# clfu-20261117 evidence (PROVISIONAL, branch clfu, Roadmap-Item CLFU)
Items: CLFU1-R-1/2/3/5(part)/6, WPROG3-R3-F2, FU3-R-M1/M2/M3/N1 (see docs/FOLLOW_UPS_NEXT_PHASE.md, section CLFU). Decisions listed there (R-4, R-5 removal).
Verify: `sha256sum -c SHA256SUMS` in this directory.
- Desktop: green_desktop_base_code.txt (new tests, unchanged source: 15 pass); mutants_desktop.txt (D1, D3 survive the OLD tests = the RED, killed by the new ones); green_desktop_full.txt (29 files, 458 tests pass; the stack traces in the log are pre-existing expected-error logging), green_desktop_tsc.txt (tsc rc=0).
- Go: red_go_n4c.txt (base code: 500 instead of 200 = RED), green_go_watchprogress.txt (`-run TestWatchProgress`, whole set incl. the new test; PG-gated tests need a Postgres and are skipped here), green_go_netguard.txt + mutants_go.txt (mutant E1 survives the old netguard redirect tests, killed by TestCLFU_...).
- TV: red_tv.txt (compile-RED: PlayerShutdown absent), green_tv_targeted.txt (PlayerShutdownTest 7, WatchProgressTick 8, DestroyWiringTest 3, 0 fail), mutants_tv_M1_M3.txt (M1 = release after the network calls, M3 = Main instead of Main.immediate: 4 + 1 tests fail), green_tv_full.txt (full :app:testDebugUnitTest, 1317 tests, 0 fail, 0 error).
- Phone: green_phone_targeted.txt (DatabaseTest 24, UpgradeTest 2, RatingTest 6, 0 fail), mutants_phone_P1_P3.txt (+ .diff.txt: REPLACE->ABORT and delete-as-no-op killed by 3 tests). The phone full suite was NOT re-run (only tests were added on the phone side).
- Gradle: plain rootless podman eclipse-temurin:17-jdk (run_gradle.sh), recorded deviation from the containers-submodule build rule; nice 15, one build at a time, load average ~54 on the host.
## Honest limits
- Not run on a device or emulator; the TV `onDestroy` order is proven by PlayerShutdownTest on the extracted function and by a source-shape pin for the wiring, not by driving the activity (UNCONFIRMED live).
- Mutant M2 (NonCancellable removed) was not run separately; the cancellation test also failed under M1 but its own attribution to M2 is UNCONFIRMED. Phone mutants P1/P3 were applied together (attribution by test name). The "old tests survive" statement for the phone mutants comes from the CLFU1 reviewer's report, not re-run here.
- Desktop fixtures copy the Go handler's body shapes by hand (CLFU-N1).
- Go: only `-run TestWatchProgress` (handlers) and `-run TestCLFU_` (netguard) plus the netguard `Redirect|SECQ1` set were run, not the whole module; no -race run.
- No live server, no Postgres run in this scratch clone.
