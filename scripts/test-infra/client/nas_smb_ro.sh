#!/usr/bin/env bash
# nas_smb_ro.sh - T132 real-NAS leg (inside IMG-INFRA-CLIENT): READ-ONLY SMB access to ONE real Synology host. Allowed requests, built only from the fixed templates below:
#   (1) the share list (`smbclient -L`), (2) one depth-1 listing `ls` of the root of one disk share, (3) one bounded read: `get` of the SMALLEST regular file of that listing
#   (at most 65536 bytes, a plain safe name), to /tmp of the container. No put, del, mkdir, rmdir, rename, set* or any other write-class command exists in this script
#   (tests/infra/test_nas_readonly_leg.sh scans for them). At most 2 requests per second (a sleep of 1 s separates the requests).
# Usage: nas_smb_ro.sh <ip> <index>     the credentials are read from the auth file /out/auth (mode 0600, created by the host script, deleted by it); never argv or env.
# Output: /out/nas-<index>.tsv with `key<TAB>value` lines: share, entries, dirs, files, names_sha256 (the sha256 of the sorted entry NAMES: the names themselves are never recorded),
#         read_size, read_sha256 (of the bytes of the one small file; its name and content are never recorded), max_protocol_requested (the SMB3 ceiling asked for, not the negotiated dialect), requests, writes_performed=0 (by construction).
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
IP=${1:-}; IDX=${2:-}; AF=/out/auth; OUT="/out/nas-$IDX.tsv"
[[ "$IP" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] && [[ "$IDX" =~ ^[1-7]$ ]] || fail "usage: nas_smb_ro.sh <ip> <1-7>"
[ -r "$AF" ] || fail "no auth file"
REQ=0; : >"$OUT"
kv() { printf '%s\t%s\n' "$1" "$2" >>"$OUT"; }
req() { REQ=$((REQ+1)); sleep 1; }   # <= 1 request per second after the first
req; shares="$(timeout 30 smbclient -L "//$IP" -A "$AF" -m SMB3 -g 2>&1)" || { kv error share_list_failed; fail "share list failed"; }
kv shares_seen "$(printf '%s\n' "$shares" | grep -c '^Disk|')"
SHARE="$(printf '%s\n' "$shares" | sed -n 's/^Disk|\(DATA[0-9-]*\|Data\)|.*/\1/p' | head -1)"
[ -n "$SHARE" ] || { kv error no_data_share; fail "no DATA share in the share list"; }
case "$SHARE" in *[!A-Za-z0-9-]*) kv error unsafe_share_name; fail "unsafe share name";; esac
kv share "$SHARE"
req; ls_out="$(timeout 60 smbclient "//$IP/$SHARE" -A "$AF" -m SMB3 -c 'ls' 2>&1)" || { kv error listing_failed; fail "listing failed"; }
entries="$(printf '%s\n' "$ls_out" | sed -nE 's/^  (.*[^ ])  +([A-Z]+) +([0-9]+)  +[A-Z][a-z]{2} [A-Z][a-z]{2} .*$/\2\t\3\t\1/p' | grep -vP '^[A-Z]*\t[0-9]+\t\.\.?$')"
n="$(printf '%s\n' "$entries" | grep -c .)"; dirs="$(printf '%s\n' "$entries" | awk -F'\t' '$1 ~ /D/ { c++ } END { print c+0 }')"
kv entries "$n"; kv dirs "$dirs"; kv files "$((n-dirs))"
kv names_sha256 "$(printf '%s\n' "$entries" | cut -f3 | LC_ALL=C sort | sha256sum | cut -d' ' -f1)"
# the smallest regular file of at most 65536 bytes with a plain name
pick="$(printf '%s\n' "$entries" | awk -F'\t' '$1 !~ /D/ && $2 > 0 && $2 <= 65536 && $3 ~ /^[A-Za-z0-9._ -]+$/ { print $2 "\t" $3 }' | sort -n | head -1)"
if [ -n "$pick" ]; then
  size="${pick%%$'\t'*}"; name="${pick#*$'\t'}"
  req; if timeout 60 smbclient "//$IP/$SHARE" -A "$AF" -m SMB3 -c "get \"$name\" /tmp/nas-small.bin" >/dev/null 2>&1; then
    kv read_size "$(stat -c %s /tmp/nas-small.bin)"; kv read_sha256 "$(sha256sum /tmp/nas-small.bin | cut -d' ' -f1)"; rm -f /tmp/nas-small.bin
  else kv read_size failed; fi
else kv read_size none_small_file; fi
# writes_performed is 0 BY CONSTRUCTION (no write-class template exists in this script; tests/infra/test_nas_readonly_leg.sh section A scans for them); max_protocol_requested is the `-m SMB3` ceiling we ask for, NOT the negotiated dialect (never read)
kv requests "$REQ"; kv writes_performed 0; kv max_protocol_requested SMB3
pass "read-only listing of one disk share and at most one bounded read (requests=$REQ, writes=0)"
