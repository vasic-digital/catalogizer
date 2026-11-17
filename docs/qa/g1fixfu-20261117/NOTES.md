# g1fixfu (Sonnet author, PROVISIONAL)
- G1FIX-R1-F1: new `scanSanctionedHookBodies` in main_chpw_authsurface_test.go: refuses any registration/Use/NoRoute/NoMethod/reflective Call/Method call (any receiver) inside routeDumpHook and routeChainFuncNames bodies; pins routeDumpHook(...) to exactly one call site, in main() of main.go.
- RED (red_1.log): mutants M2b, M7, FU1-FU4 survived before the scanner. GREEN (green_1.log): `-run AuthSurface` ok, all existing mutants killed, golden-FALSE passes.
- Own mutants FU1 (Use in hook), FU2 (POST in routeChainFuncNames), FU3 (hook call from other function), FU4 (Group in hook).
- N1: TestUpdateFavoriteStatus_AnonymousIs401BeforeParse also asserts "Authentication required" (green_2.log). No separate RED run for N1 (assertion-only strengthening; UNCONFIRMED that it kills a message-changing mutant).
- UNCONFIRMED: the banned-name set is not exhaustive (e.g. a registration through an aliased function value is not seen); G1FIX-R1-F2 (doc rewrite) not in scope here.
