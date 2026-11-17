# clfu1-20261117 evidence (PROVISIONAL)
Follow-ups CLPROG-R2-m1/m2/n1/n2 and RATING1-R-F3 (see docs/FOLLOW_UPS_NEXT_PHASE.md, section CLFU1).
- red_desktop_n2.txt: 3 of 4 new desktop tests fail on the base apiService. red_tv.txt: compile-RED (runWith/finalSave absent). red_phone.txt: compile-RED on base main sources (Int rating, getUserRating, CREATE_USER_RATINGS_SQL absent). The K6 test passes on base by design (the base code is already right); it exists to kill mutant K6.
- green_desktop_n2.txt (64 pass in services+hook tests, tsc clean), green_tv_targeted.txt (17 tests, 0 fail), green_phone_targeted.txt, green_phone_full.txt, green_tv_full.txt (full suites).
- Mutants (each run with several mutants applied together; attribution is by failing test name): mutant_desktop_n2.txt (3 tests fail); mutants_tv_K5_K7_K8.txt (K5, K7, K8 each fail their own test); mutants_phone.txt (K6, R1 writes community column, R2 no 400 drop, R3 no fractional drop, R4 wrong migration SQL: all killed). Survivors: none.
- Gradle: plain rootless podman eclipse-temurin:17-jdk (recorded deviation), run_tv.sh, nice 15, one build at a time. Scratch only, not committed: google-services.json, local.properties, node_modules, build dirs.
- Totals: phone full 1091 tests 0 fail/0 error; TV full 1307 tests 0 fail/0 error; desktop full 453 pass (29 files), tsc 0 (green_desktop_full.txt).
