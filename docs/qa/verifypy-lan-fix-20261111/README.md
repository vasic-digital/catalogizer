# verify.py LAN/pgAdmin fix evidence (CAT-QA-VERIFYPY-1)
- verify_v2_run3.txt (pass=45 fail=6 total=51): NOT a verifier result. 429 login rate limiting from back-to-back runs (api.metrics http=0, two nginx login 429s) plus the 3 by-design undeclared-exposure FAILs. Superseded by verify_v3_run3.txt, re-run after a >=2 min pause: pass=50 fail=3.
- verify_v3_run{1,2,3}.txt: no *_BIND declared -> 50/3, the 3 being lan.*_LAN_exposure_undeclared (by design).
- verify_v3_declared_run{1,2,3}.txt: binds declared -> 53/0.
- test_lanbind_red_v3.txt / test_lanbind_green_v3.txt: RED (signature) then GREEN 23/23; 5 reviewer-style mutants killed.
