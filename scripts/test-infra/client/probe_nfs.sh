#!/usr/bin/env bash
# probe_nfs.sh - T134 (inside IMG-INFRA-CLIENT): an NFSv3 MOUNT and READDIR of the export through the libnfs user-space client (nfs-ls); no kernel mount, no mount(2).
# libnfs 4.0.0 takes an IP address, not a host name, so the service name is resolved first. A refused or unreachable server FAILs.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of NFS nfs)"; IP="$(getent hosts "$H" | awk '{ print $1; exit }')"
[ -n "$IP" ] || fail "nfs: host $H does not resolve"
out="$(timeout 20 nfs-ls "nfs://$IP/export" 2>&1)"; rc=$?
[ "$rc" = 0 ] || fail "nfs-ls of the export failed rc=$rc: ${out:0:120}"
case "$out" in *"Failed"*|*"failed"*) fail "nfs-ls reported a failure: ${out:0:120}";; esac
bad="$(timeout 20 nfs-ls "nfs://$IP/no-such-export" 2>&1)"; brc=$?
case "$bad" in *Failed*|*failed*|*MNT3ERR*) ;; *) fail "nfs: a mount of a non-existent export was not refused ('${bad:0:80}' rc=$brc)";; esac
pass "nfs MOUNT and READDIR of /export answered; a mount of an unexported path was refused (libnfs user-space client)"
