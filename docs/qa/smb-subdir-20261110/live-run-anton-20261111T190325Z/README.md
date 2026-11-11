# Anton Public/Testing SMB live run (CAT-SMB-SUBDIR-1)
Command: docs/qa/smb-subdir-20261110/run.sh <0600 env> (go test -tags live_smb -run 'TestLiveSMBSubdir|TestLiveSMBServiceSubdir' ./filesystem ./internal/services) on main e5817850, 2026-11-11T19:03:25Z, host load ~17.
Result: TestLiveSMBSubdir PASS (4 outside-name probes), TestLiveSMBServiceSubdir PASS. Log: go_test.log (sha256 in go_test.log.sha256). Host, user and password values were checked absent from the log.
Scope: real host Samba share Public, subdir Testing, via hostname/env-supplied host (not a bare-IP proof). Not a Synology run.
