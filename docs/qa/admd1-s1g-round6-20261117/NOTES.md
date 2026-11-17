# ADMD1F1-S1 round 6 (Opus round-5 follow-ups) - PROVISIONAL
- F3 first: route-set equality (real engine vs ROUTE_AUTHZ_TABLE.md), both directions, inside groundTruthViolations. Real run: 265 real = 265 table (checked before the change: no diff).
- F1: scanner reports a non-call handler before the gate; real-chain check reports non-pinned handlers before the gate (preGateAllowedFiles). Old golden-FALSE `h, RequireAdmin(), h` replaced by `mkMw(), RequireAdmin(), h`.
- F2: full File path compared. F4: shadow probe. F5: this evidence. F6: wrapper closure by symbol + file.
- red.txt: 3 FAIL against the round-5 code (the F4 probe was already green on round-5 code; it is a new kill for old survivor #8, see M8).
- green_1..3: `-run 'ADMD1|Admin403|ContractGate|OpenAPI'` ok x3 (commands in the files). vet_live.txt, vet_fmt.txt.
- mutants.txt: 12 mutants, all KILLED (M1 ungated route, M2 handler before gate, M3 decoy gate in another middleware/auth.go: reviewer's three, real binary). Process note: the first M5-M12 pass was INVALID (the M4 table row leaked because the restore ran only inside catalog-api); it was re-run after restoring the table, only the second pass is kept.
- Scope/UNCONFIRMED: routes only under other env/config (R5-FU-1); live sweep backstop for F1/decoy not run; pre-gate allow-list is a pinned file list; M1-M3 evidence lines are truncated to the first assertion text.
