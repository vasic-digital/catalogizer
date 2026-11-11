#!/bin/bash
# CAT-FRONTDOOR-RATELIMIT-1 stack-level check, run against a LIVE stack 3 (does not restart anything).
# Usage: test_clientip_frontdoor.sh URL [SECOND_HOST_SSH] [--interface LOCAL_IP]
#   --interface LOCAL_IP  (CAT-FRONTDOOR-LOCALMODE-1) run property B WITHOUT ssh, from a SECOND IP ASSIGNED TO THIS HOST: the probe is sent with
#                         `curl --interface LOCAL_IP`, so the front door sees a different source address than the flood. Needs LOCAL_IP to be
#                         configured on a local interface (ip addr add ... ) and not to be the address the kernel picks by default for URL's host
#                         (otherwise both calls share one bucket and B would prove nothing: honest SKIP). Can be combined with SECOND_HOST_SSH.
# Property A (single host, always run): a client-forged X-Forwarded-For must NOT yield a fresh rate-limit bucket (spoof-resistance).
# Property B (needs a second LAN host reachable by ssh, else SKIP honestly): a client on ANOTHER host keeps its own bucket, i.e. is not
#   locked out by a flood from this host. Host-local clients all share pasta's alias (169.254.1.2) by design; that is not property B.
# WARNING: the flood (35 bad logins) locks every client that shares this host's rate-limit bucket (host-local clients,
#   pasta alias 169.254.1.2) out of login for about a minute. Do not run it while someone is using the stack from this host.
# Waits 62 s first so the 30/min login window is empty. Exit 0 PASS / 1 FAIL.
set -u
. "${QA_LAN_IP_LIB:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../lib" && pwd)/lan_ip.sh}" || { echo "cannot source docs/qa/lib/lan_ip.sh" >&2; exit 2; }
T=""; OTHER=""; IFACE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --interface) [ $# -ge 2 ] || { echo "--interface needs a local IPv4 address" >&2; exit 2; }; IFACE="$2"; shift 2;;
    --interface=*) IFACE="${1#--interface=}"; shift;;
    *) if [ -z "$T" ]; then T="$1"; elif [ -z "$OTHER" ]; then OTHER="$1"; else echo "unexpected argument: $1" >&2; exit 2; fi; shift;;
  esac
done
[ -n "$T" ] || { echo "usage: $0 URL [SECOND_HOST_SSH] [--interface LOCAL_IP]" >&2; exit 2; }
if [ -n "$IFACE" ]; then
  # whole-string IPv4 check in bash itself (a grep would test line by line and pass "10.0.0.5<newline>junk")
  if ! qa_ip_is_ipv4 "$IFACE"; then echo "FAIL --interface must be an IPv4 address (four octets 0-255, nothing else), got '$IFACE'"; exit 1; fi
  # the address must be configured on a local interface: exact match "inet <ip>/" (so 10.0.0.5 never matches 10.0.0.50)
  if ! qa_ip_is_assigned "$IFACE"; then echo "FAIL --interface $IFACE is not assigned to this host (ip -4 addr); refusing to run B against an address the kernel cannot bind"; exit 1; fi
fi
post(){ curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' "$@" -d '{"username":"qa_nonexistent_fd","password":"x"}' "$T/api/v1/auth/login"; }
sleep "${FD_WARMUP:-62}"   # FD_WARMUP only for the sandbox test (test_frontdoor_interface.sh)
# Pre-flood control: the instrument must see an un-throttled 401 (a wrong URL / dead API would also look "429-free"),
# otherwise the later 429s prove nothing.
ctl=$(post)
if [ "$ctl" != 401 ]; then echo "CONTROL FAIL: pre-flood login returned $ctl (expected 401); instrument blind or stack unhealthy"; echo FAIL; exit 1; fi
for _ in $(seq 1 35); do post >/dev/null; done
no_xff=$(post); spoof=$(post -H 'X-Forwarded-For: 9.9.9.9'); chain=$(post -H 'X-Forwarded-For: 8.8.8.8, 9.9.9.9')
echo "A: after flood no-XFF=$no_xff spoof=$spoof chain-spoof=$chain (all must be 429)"
rc=0; [ "$no_xff$spoof$chain" = 429429429 ] || rc=1
RAN_B=0; RAN_LOCAL=0; RAN_SSH=0
if [ -n "$IFACE" ]; then
  host="$(printf '%s' "$T" | sed -E 's#^[a-z]+://##; s#[/:].*$##')"
  defsrc="$(ip -4 route get "$host" 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p' | head -n 1)"
  if [ "$defsrc" = "$IFACE" ]; then echo "B(local): SKIP interface_same_as_default_source ($IFACE is the address the kernel already uses for $host; same bucket, proves nothing)"
  else
    loc=$(post --interface "$IFACE")
    echo "B(local): probe from second local IP $IFACE (default source ${defsrc:-unknown}) got $loc (must be 401, not 429)"; RAN_B=1; RAN_LOCAL=1; [ "$loc" = 401 ] || rc=1
  fi
fi
if [ -n "$OTHER" ]; then
  other=$(ssh -o BatchMode=yes -o ConnectTimeout=5 "$OTHER" "curl -s -o /dev/null -m 10 -w '%{http_code}' -X POST -H 'Content-Type: application/json' -d '{\"username\":\"qa_nonexistent_fd\",\"password\":\"x\"}' $(printf '%q' "$T/api/v1/auth/login")")
  echo "B: second host $OTHER got $other (must be 401, not 429)"; RAN_B=1; RAN_SSH=1; [ "$other" = 401 ] || rc=1
fi
[ "$RAN_B" = 0 ] && [ -z "$IFACE" ] && echo "B: SKIP second_host_not_supplied (UNCONFIRMED: remote-LAN bucket isolation)"
if [ $rc != 0 ]; then echo FAIL; exit 1; fi
if [ "$RAN_B" = 1 ]; then
  legs="B"; [ "$RAN_SSH$RAN_LOCAL" = 11 ] && legs="B and B(local)"; [ "$RAN_SSH$RAN_LOCAL" = 01 ] && legs="B(local)"
  echo "PASS (A and $legs)"
else echo "PASS (A only; B SKIP - remote-LAN isolation NOT verified)"; fi
exit 0
