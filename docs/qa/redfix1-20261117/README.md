# REDFIX1 (PROVISIONAL)
RED (main 2f14871ce): R7 pins-exactly and S1G6F3 fail ("route-level handler runs before the gate (main.altSvcMiddleware in main_router.go)"); R5 RealRouteTable failed in the original run (red_original_failing_run.txt), did not fail in the scratch rerun (red_rerun_scratch_clone.txt: R7 + S1G6F3 FAIL).
Fix: preGateAllowedSymbols["main_router.go"]={"altSvcMiddleware"} + content pin (sha256 of printed decl, comments stripped) in api_contract_gate_altsvc_pin_test.go with mutants (body, header value) and golden-FALSE (comment/whitespace edit, other name not allow-listed).
GREEN: green_guard_set.txt, full guard set `go test -p 1 -timeout 90m -v -run '...'` ok 960s, 116 PASS, 0 FAIL (an earlier non-verbose run hit go's default 10m timeout, not a test failure).
Trial merge with altsvc3 1fe0e80dd: merge-tree clean, altSvcMiddleware body byte-identical.
