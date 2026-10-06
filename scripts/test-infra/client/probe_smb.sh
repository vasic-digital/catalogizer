#!/usr/bin/env bash
# probe_smb.sh - T128: SMB share list shows `testshare` (authenticated), and a directory listing of the share shows the seeded corpus.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of SMB smb)"
af="$(secret_file "username = $TI_SMB_USER" "password = $TI_SMB_PASSWORD")"
shares="$(timeout 20 smbclient -L "//$H" -A "$af" -m SMB3 -g 2>&1)" || fail "smb share list failed: ${shares:0:120}"
case "$shares" in *testshare*) ;; *) fail "smb share list lacks testshare (got '${shares:0:120}')";; esac
ls="$(timeout 20 smbclient "//$H/testshare" -A "$af" -m SMB3 -c 'ls movies/*' 2>&1)" || fail "smb directory listing failed: ${ls:0:120}"
case "$ls" in *The.Matrix.1999.1080p.BluRay.mkv*) ;; *) fail "smb listing lacks the seeded file (got '${ls:0:120}')";; esac
pass "smb share list shows testshare and the share lists the seeded corpus"
