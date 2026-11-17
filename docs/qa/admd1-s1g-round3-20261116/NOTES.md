# ADMD1F1-S1 round 3 (structural) - PROVISIONAL
- red.txt: 26 subtests/tests FAIL before the rule (new R3 tests on the round-2 scanner).
- green_{1,2,3}.txt: `go test -count=1 -v . -run 'ADMD1|Admin403|ContractGate|OpenAPI'` ok x3; vet_live.txt: `go vet -tags live .` clean (empty).
- mutants.txt: 17 own mutants of the new rules, all killed (run in a trimmed copy of the scanner + the same tests; M14 is killed by a build break only, i.e. weak; M15/M13 also by the real-source test).
- Real source: 0 unresolved; routes=264 gated=77 (measured with the round-2 scanner and the new one alike; the "265" in the brief did not reproduce: UNCONFIRMED what counted 265).
