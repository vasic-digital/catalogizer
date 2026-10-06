#!/bin/sh
# entrypoint.sh - T134: rpcbind (the portmapper the libnfs client asks first), then unfsd in the foreground (TCP only). A failure of either is printed and ends the
# container (never a silent half-start).
set -u
mkdir -p /run/rpcbind /export
rpcbind -f -w &
i=0; while [ "$i" -lt 20 ]; do rpcinfo -p 127.0.0.1 >/dev/null 2>&1 && break; i=$((i+1)); sleep 0.5; done
rpcinfo -p 127.0.0.1 >/dev/null 2>&1 || { echo "ti-nfs: rpcbind did not answer on 127.0.0.1:111" >&2; exit 71; }
exec /usr/local/sbin/unfsd -d -t -e /etc/exports
