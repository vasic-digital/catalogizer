#!/usr/bin/env bash
# Sandbox test for docs/qa/lib/lan_ip.sh (fake `ip`, nothing real is touched).  usage: test_lan_ip.sh [LIB]   exit 0 = all pass.
# LIB defaults to the sibling lan_ip.sh; the mutation runner passes a mutated copy.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
LIB="${1:-$HERE/lan_ip.sh}"
[ -f "$LIB" ] || { echo "FAIL library missing: $LIB"; exit 1; }
R="$(mktemp -d)" || exit 1
[ -n "$R" ] && [ -d "$R" ] || exit 1
trap 'rm -rf -- "${R:?}"' EXIT
mkdir -p "$R/bin"
cat > "$R/bin/ip" <<'S'
#!/usr/bin/env bash
# fake: route get -> src $FAKE_SRC (nothing + rc 1 if unset); addr show -> inet lines from $FAKE_ADDRS (space separated)
case "$*" in
  *"route get"*) [ -n "${FAKE_SRC-}" ] || exit 1; echo "1.1.1.1 via 10.0.0.1 dev eth0 src ${FAKE_SRC} uid 1000";;
  *"addr show"*) for a in ${FAKE_ADDRS-}; do echo "2: eth0    inet $a/24 brd 10.0.0.255 scope global eth0"; done;;
  *) exit 1;;
esac
S
chmod +x "$R/bin/ip"
P=0; F=0
ok() { if [ "$1" = 0 ]; then P=$((P+1)); echo "PASS $2"; else F=$((F+1)); echo "FAIL $2"; fi; }
# q <env assignments...>: sources the lib in a clean subshell, prints "<stdout>|<rc>"
q() { ( unset QA_LAN_IP QA_BIND LAN_BIND FAKE_SRC FAKE_ADDRS; export PATH="$R/bin:$PATH"; for kv in "$@"; do export "${kv?}"; done
        . "$LIB"; o="$(qa_lan_ip 2>"$R/err")"; rc=$?; echo "$o|$rc" ); }
v="$(q FAKE_SRC=10.0.0.5 'FAKE_ADDRS=10.0.0.5 10.0.0.6')"; [ "$v" = "10.0.0.5|0" ]; ok $? "route-derived address returned (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 'FAKE_ADDRS=10.0.0.5 10.0.0.6' QA_LAN_IP=10.0.0.6)"; [ "$v" = "10.0.0.6|0" ]; ok $? "QA_LAN_IP override wins over route (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 'FAKE_ADDRS=10.0.0.5 10.0.0.6' LAN_BIND=0.0.0.0)"; [ "$v" = "10.0.0.5|0" ]; ok $? "LAN_BIND=0.0.0.0 (monitoring wildcard opt-in) does NOT affect qa_lan_ip (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 'FAKE_ADDRS=10.0.0.5 127.0.0.1' QA_BIND=127.0.0.1)"; [ "$v" = "10.0.0.5|0" ]; ok $? "QA_BIND=127.0.0.1 (noVNC publish address) does NOT affect qa_lan_ip (got '$v')"
v="$(q FAKE_ADDRS=10.0.0.5 QA_BIND=10.0.0.5 LAN_BIND=10.0.0.5)"; [ "$v" = "|1" ]; ok $? "QA_BIND/LAN_BIND are never a source of the address (no route -> failure)"
v="$(q FAKE_SRC=10.0.0.5 'FAKE_ADDRS=10.0.0.5 10.0.0.6' QA_LAN_IP=10.0.0.5 QA_BIND=10.0.0.6 LAN_BIND=10.0.0.6)"; [ "$v" = "10.0.0.5|0" ]; ok $? "QA_LAN_IP beats QA_BIND/LAN_BIND (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 QA_LAN_IP=10.0.0.9)"; [ "$v" = "|1" ]; ok $? "override not assigned -> failure, no fall-through to route (got '$v')"
v="$(q FAKE_SRC=10.0.0.7 FAKE_ADDRS=10.0.0.5)"; [ "$v" = "|1" ]; ok $? "route src not assigned locally -> failure (got '$v')"
v="$(q FAKE_ADDRS=10.0.0.5)"; [ "$v" = "|1" ]; ok $? "no route -> failure (got '$v')"
grep -q 'qa_lan_ip' "$R/err"; ok $? "failure prints a message on stderr"
v="$(q FAKE_SRC=999.1.1.1 FAKE_ADDRS=999.1.1.1)"; [ "$v" = "|1" ]; ok $? "invalid octet rejected (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 QA_LAN_IP=not-an-ip)"; [ "$v" = "|1" ]; ok $? "junk override rejected (got '$v')"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.50)"; [ "$v" = "|1" ]; ok $? "10.0.0.5 does not match assigned 10.0.0.50 (prefix)"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 "QA_LAN_IP=10.0.0.5
junk")"; [ "$v" = "|1" ]; ok $? "newline injection (valid line + junk) rejected (got '$(printf %s "$v" | tr '\n' '~')')"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 "QA_LAN_IP=10.0.0.5
10.0.0.5")"; [ "$v" = "|1" ]; ok $? "two-line override (both valid) rejected"
( . "$LIB"; export PATH="$R/bin:$PATH" FAKE_ADDRS=10.0.0.5; qa_ip_is_assigned "10.0.0.5
10.0.0.5" ); [ $? -ne 0 ]; ok $? "qa_ip_is_assigned rejects newline-joined value"
v="$(q FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 QA_LAN_IP=not-an-ip)"; grep -q 'not a plain IPv4' "$R/err"; ok $? "junk override is reported as not-IPv4 (validator itself fires, not only the assigned check)"
( . "$LIB"; qa_ip_is_ipv4 10.0.0.5 ); ok $? "qa_ip_is_ipv4 accepts a dotted quad"
for bad in 010.0.0.5 10.00.0.5 10.0.0.05 1.2.3.4.5 10.0.0.5x 10.0.0.5. 1.2.3 256.1.1.1 "10.0.0.5
junk" "x10.0.0.5" ""; do
  ( . "$LIB"; qa_ip_is_ipv4 "$bad" ); [ $? -ne 0 ]; ok $? "qa_ip_is_ipv4 rejects '$(printf %s "$bad" | tr '\n' '~')'"
done
echo "passed=$P failed=$F"; [ "$F" = 0 ]
