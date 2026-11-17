# admd1 s1g round 6b (Opus round-1 review of s1g5fu: R7, F-msg, F-allow, nits) — PROVISIONAL
Branch s1g5fu2 = main + s1g5fu (merge; docs/FOLLOW_UPS_NEXT_PHASE.md conflict resolved by keeping both sections) + this change. No push.
Commands: GOMAXPROCS=2 nice -n 10 go test -p 1 ... in catalog-api; submodules symlinked from the main checkout for the builds, restored before commit.
Files: red.txt (new tests vs pre-fix behaviour: 3 of 5 FAIL), green_{1,2,3}.txt (-run 'ADMD1|Admin403|ContractGate|OpenAPI|R6', exit 0 x3), vet_fmt.txt, mutants.txt (M1..M9 incl. M7b; mutants_script.sh.txt is the script), SHA256SUMS.
Not run / UNCONFIRMED: full `go test ./...` of catalog-api, TestLiveAPIContract live sweep, Android/web. Scanner does not recognise wrap(namedHandler) before the gate (real-chain allow-list is the backstop). Whether the Opus reviewer accepts the symbol-level allow-list design is UNCONFIRMED (review owed).
