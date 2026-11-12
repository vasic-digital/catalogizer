# Anton Public/Testing SMB live runs N=3 (CAT-SMB-SUBDIR-1) - PROVISIONAL/VERIFIED see below
Command (per docs/qa/smb-subdir-20261110/run.sh, minus systemd-run scope): GOMAXPROCS=2 nice -n 10 go test -count=1 -tags live_smb -run 'TestLiveSMBSubdir|TestLiveSMBServiceSubdir' -v ./filesystem ./internal/services (cwd catalog-api), creds from 0600 env in subshell.
Source: git archive of main 1538ef19 (own-org Go replace submodules extracted at recorded gitlinks, see submodule_pins.txt). Real host Samba, share Public, subdir Testing, by env-supplied host. Host load ~9-10.
Results: run1/2/3 exit 0, both tests PASS each run (4 outside-name probes), normalized logs identical (hash-set size 1). Credential literals grep-checked absent.
Scope: not bare-IP, not Synology. No negative control run here (existing run uses test-server negctrl). Host-side go test (deviation from 11.4.173 as in run.sh).
