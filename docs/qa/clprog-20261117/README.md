# clprog-20261117 evidence (PROVISIONAL)
Clients vs the WPROG3 contract (docs/FOLLOW_UPS_NEXT_PHASE.md, section clprog).
- red_desktop_service.txt: desktop new tests on base apiService: 4 failed / 2 passed.
- red_apiclient.txt: api-client tests on base: 6 failed.
- red_tv.txt: Android TV new tests on base: compile errors (symbols absent) = compile-RED. No RED run was recorded for the Android phone module (its tests were changed together with the code); stated honestly.
- green_desktop_full.txt (446 pass), green_apiclient_full.txt (316 pass), green_tv_full.txt (BUILD SUCCESSFUL, 1299 tests 0 fail), green_android_full.txt (BUILD SUCCESSFUL).
- Gradle: plain rootless podman eclipse-temurin:17-jdk (recorded deviation), run_tv.sh, nice 15, one build at a time, host load ~50.
- Untracked scratch not committed: google-services.json, local.properties, node_modules.

## Round 2 (review NO-GO F1/F2)
- mutants_desktop.txt: M5 and M6 each fail 1 of 3 new call-site tests (killed). mutant_tv_advance_on_notstored.txt: advance-on-NotStored fails WatchProgressTickTest (killed).
- green_desktop_r2_full.txt (449 pass, tsc clean), green_tv_r2_full.txt (BUILD SUCCESSFUL), green_android_r2.txt (CatalogizerApiTest + MediaRepositoryTest, BUILD SUCCESSFUL; the full phone suite was not re-run, only those classes). RED for the new call-site/tick tests: the tick test is compile-RED on base (class absent); desktop call-site tests were written after the fix and validated by the mutants.
