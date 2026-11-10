#!/usr/bin/env bash
# lan_ip.sh - ONE shared helper that answers "which LAN IPv4 address does this QA host have right now?" (CAT-LANIP-DYN-1).
# SOURCE it (never executed, never calls exit):  . "<repo>/docs/qa/lib/lan_ip.sh"
#   qa_lan_ip             prints the address on stdout, rc 0; on failure nothing on stdout, a message on stderr, rc 1.
#   qa_ip_is_ipv4 IP      rc 0 only when the WHOLE string is a dotted-quad IPv4 (newline/junk safe).
#   qa_ip_is_assigned IP  rc 0 only when IP is configured on a local interface (`ip -4 -o addr show`, exact match).
# Resolution order (no guessing, no hardcoded fallback):
#   1. explicit override: $QA_LAN_IP (a dedicated name; QA_BIND = noVNC publish address and LAN_BIND = monitoring wildcard opt-in are NOT read) (an invalid or unassigned override FAILS; it never falls through)
#   2. the source address the kernel picks for `ip -4 route get 1.1.1.1`
# The result must be a valid IPv4 AND assigned to this host, otherwise qa_lan_ip fails loudly.
# Per-script variables (SONAR_BIND, S3_BIND, WEB_URL, TS_HOST ...) stay explicit overrides layered on top by their scripts.

qa_ip_is_ipv4() {
  local oct='(25[0-5]|2[0-4][0-9]|1[0-9][0-9]|[1-9]?[0-9])'
  [[ "${1-}" =~ ^$oct\.$oct\.$oct\.$oct$ ]]
}

qa_ip_is_assigned() {
  local ip="${1-}" line
  qa_ip_is_ipv4 "$ip" || return 1
  while IFS= read -r line; do
    [[ "$line" =~ [[:space:]]inet[[:space:]]${ip//./\\.}/ ]] && return 0
  done < <(ip -4 -o addr show 2>/dev/null)
  return 1
}

qa_lan_ip() {
  local cand="" src="" out
  if [ -n "${QA_LAN_IP-}" ]; then cand="$QA_LAN_IP"; src="QA_LAN_IP"
  else
    src="ip -4 route get 1.1.1.1"
    out="$(ip -4 route get 1.1.1.1 2>/dev/null)" || out=""
    if [[ "$out" =~ [[:space:]]src[[:space:]]+([0-9.]+) ]]; then cand="${BASH_REMATCH[1]}"; fi
    if [ -z "$cand" ]; then echo "qa_lan_ip: cannot determine the LAN address ($src gave no src); set QA_LAN_IP=<ipv4 assigned to this host>" >&2; return 1; fi
  fi
  if ! qa_ip_is_ipv4 "$cand"; then echo "qa_lan_ip: value from $src is not a plain IPv4 address; refusing" >&2; return 1; fi
  if ! qa_ip_is_assigned "$cand"; then echo "qa_lan_ip: $cand (from $src) is not assigned to any local interface; refusing" >&2; return 1; fi
  printf '%s\n' "$cand"
}
