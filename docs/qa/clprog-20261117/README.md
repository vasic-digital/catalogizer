# clprog-20261117 evidence (PROVISIONAL)
Clients vs the WPROG3 contract (docs/FOLLOW_UPS_NEXT_PHASE.md, section clprog).
- red_desktop_service.txt: desktop new tests on base apiService: 4 failed / 2 passed.
- red_apiclient.txt: api-client tests on base: 6 failed.
- red_tv.txt: Android TV new tests on base: compile errors (symbols absent) = compile-RED. No RED run was recorded for the Android phone module (its tests were changed together with the code); stated honestly.
- green_desktop_full.txt (446 pass), green_apiclient_full.txt (316 pass), green_tv_full.txt (BUILD SUCCESSFUL, 1299 tests 0 fail), green_android_full.txt (BUILD SUCCESSFUL).
- Gradle: plain rootless podman eclipse-temurin:17-jdk (recorded deviation), run_tv.sh, nice 15, one build at a time, host load ~50.
- Untracked scratch not committed: google-services.json, local.properties, node_modules.
