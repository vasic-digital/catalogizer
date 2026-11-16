# ADMD1F1-S1 round 4 (structural) - PROVISIONAL
One structural change: the scanner models the full gin method set and walks every function, with types resolved from the real type expression.
- red.txt: new R4 tests against the round-3 scanner (+shim vars) -> 19 FAIL lines. U1/U2 only fail under mutant M9.
- green_{1,2,3}.txt: ADMD1|Admin403|ContractGate|OpenAPI ok x3; vet_live.txt empty (clean); gofmt clean on the 4 scanner files.
- mutants.txt: M1..M19 + compiling M7'/M8'; M13/M18 survive as documented equivalents (both outcomes fail closed). M9 killed by R1 probes and by U1/U2.
- real_counts.txt: routes=265 gated=77 unresolved=0 on this clone; the round-3 scanner gives the identical route set (the 264 of the brief does not reproduce in this tree; UNCONFIRMED why).
- Probe names P11,P13-15,P26,P27,Q1-Q4 were reconstructed from the brief (originals not supplied).
- Server exemption is by resolved import path: net/http AND github.com/quic-go/quic-go/http3 (main.go really serves http3.Server{Handler: router}).
