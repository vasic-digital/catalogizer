#!/usr/bin/env bash
# nas_proto_nfs.sh - WP-12 READ-ONLY NFS walk of ONE host through the libnfs user-space client (inside IMG-INFRA-CLIENT; no kernel mount, no mount(2), no sudo).
# Verbs used: `nfs-ls` (MOUNT + READDIR) and `nfs-cat` (one bounded read, bytes counted and discarded). No nfs-cp, no create/remove/rename/setattr exists here
# (tests/infra/test_nas_protocols.sh scans for them). At most one request per second (sleep 1 before each listing/read); bounds per export and version: 30 listing requests,
# depth 3, 2000 entries; the read sample is the first 1 MiB (head -c) of ONE file. Entry names are used in memory only and never written.
# Usage:  nas_proto_nfs.sh <ip> <index> <exports file> <versions, e.g. 3 or 3,4>
#   <exports file>  one export path per line (the exports MOUNT EXPORT listed plus any path a MNT attempt proved mountable; written by nas_proto.py rpc)
# Output: /out/nfs-<index>.tsv  -> records `R<TAB>export<TAB>version<TAB>key<TAB>value`, `LAT<TAB>export<TAB>version<TAB>ms`, `EXT<TAB>export<TAB>version<TAB>ext<TAB>count`, `MODE<TAB>...`.
#         The host script nas_protocols.sh assembles the JSON. Timing of a listing includes the MOUNT round trip of nfs-ls (one nfs-ls = MOUNT + READDIR); stated, not hidden.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
IP=${1:-}; IDX=${2:-}; EXF=${3:-}; VERS=${4:-3}; OUT="/out/nfs-$IDX.tsv"
[[ "$IP" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] && [[ "$IDX" =~ ^[1-7]$ ]] && [[ "$VERS" =~ ^[34](,[34])*$ ]] || fail "usage: nas_proto_nfs.sh <ip> <1-7> <exports file> <3|4|3,4>"
[ -r "$EXF" ] || fail "no exports file"
: >"$OUT"
MAXREQ=30; MAXDEPTH=3; MAXENT=2000; READ=1048576
now_ms() { echo $(( $(date +%s%N) / 1000000 )); }
rec() { printf 'R\t%s\t%s\t%s\t%s\n' "$1" "$2" "$3" "$4" >>"$OUT"; }
parse() { sed -nE 's/^([-dlbcps][-rwxsStT]{9})[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+[0-9]+[[:space:]]+([0-9]+)[[:space:]]+(.+)$/\1\t\2\t\3/p'; }   # mode, size, name
SAFE='^[A-Za-z0-9._/-]{1,200}$'
n_ex=0
while IFS= read -r EXP; do
  [[ "$EXP" =~ $SAFE ]] || continue
  n_ex=$((n_ex+1))
  IFS=, read -r -a VV <<<"$VERS"
  for V in "${VV[@]}"; do
    url() { printf 'nfs://%s%s?version=%s' "$IP" "$1" "$V"; }     # nfs-ls/nfs-cat: the path is part of the URL, the query selects the NFS version
    sleep 1; t0=$(now_ms)
    top="$(timeout 60 nfs-ls "$(url "$EXP")" 2>&1)"; trc=$?; t1=$(now_ms)
    if [ $trc -ne 0 ] || printf '%s' "$top" | grep -qiE 'failed|error|MNT3ERR|NFS3ERR|NFS4ERR'; then
      st="$(printf '%s' "$top" | grep -oE 'MNT3ERR_[A-Z]+|NFS3ERR_[A-Z]+|NFS4ERR_[A-Z_]+' | head -1)"
      rec "$EXP" "$V" status "mount_or_list_refused"; rec "$EXP" "$V" reason "${st:-error}"; continue
    fi
    printf 'LAT\t%s\t%s\t%s\n' "$EXP" "$V" "$((t1-t0))" >>"$OUT"
    ent="$(printf '%s\n' "$top" | parse)"
    reqs=1; total=0; files=0; dirs=0; trunc=false; candge=""; candgesz=0; candlt=""; candltsz=0
    declare -A EXT=() MODEF=() MODED=(); : >/tmp/q.txt
    rec "$EXP" "$V" top_level_names_sha256 "$(printf '%s\n' "$ent" | cut -f3 | LC_ALL=C sort | sha256sum | cut -d' ' -f1)"
    rec "$EXP" "$V" top_level_entries "$(printf '%s\n' "$ent" | grep -c .)"
    consume() {  # consume <dir path> <depth> <entries>
      local p=$1 d=$2 e=$3 m s nm
      while IFS=$'\t' read -r m s nm; do
        [ -n "$nm" ] || continue; total=$((total+1))
        case "$nm" in *\"*|*\\*|*$'\t'*) continue;; esac
        if [ "${m:0:1}" = d ]; then dirs=$((dirs+1)); MODED[${m:1:9}]=$(( ${MODED[${m:1:9}]:-0} + 1 ))
          case "$nm" in @*|\#*|.|..) ;; *) [ "$d" -lt "$MAXDEPTH" ] && printf '%s\t%s\n' "$((d+1))" "$p/$nm" >>/tmp/q.txt;; esac
        elif [ "${m:0:1}" = - ]; then files=$((files+1)); MODEF[${m:1:9}]=$(( ${MODEF[${m:1:9}]:-0} + 1 ))
          x="(none)"; case "$nm" in *.*) x="$(printf '%s' "${nm##*.}" | tr 'A-Z' 'a-z' | cut -c1-12)";; esac; EXT[$x]=$(( ${EXT[$x]:-0} + 1 ))
          case "$p" in */@*|*/\#*) ;; *)
            if [ "$s" -ge "$READ" ]; then if [ -z "$candge" ] || [ "$s" -lt "$candgesz" ]; then candge="$p/$nm"; candgesz=$s; fi
            elif [ "$s" -gt 0 ]; then if [ -z "$candlt" ] || [ "$s" -gt "$candltsz" ]; then candlt="$p/$nm"; candltsz=$s; fi; fi;; esac
        fi
      done <<<"$e"
    }
    consume "$EXP" 0 "$ent"
    while IFS=$'\t' read -r d p && [ -n "$p" ]; do
      if [ "$reqs" -ge "$MAXREQ" ] || [ "$total" -ge "$MAXENT" ]; then trunc=true; break; fi
      sleep 1; t0=$(now_ms); o="$(timeout 60 nfs-ls "$(url "$p")" 2>&1)"; rc=$?; t1=$(now_ms); reqs=$((reqs+1))
      [ $rc -eq 0 ] || continue
      printf 'LAT\t%s\t%s\t%s\n' "$EXP" "$V" "$((t1-t0))" >>"$OUT"
      consume "$p" "$d" "$(printf '%s\n' "$o" | parse)"
    done </tmp/q.txt
    rec "$EXP" "$V" status listed; rec "$EXP" "$V" sampled_entries "$total"; rec "$EXP" "$V" sampled_files "$files"; rec "$EXP" "$V" sampled_dirs "$dirs"
    rec "$EXP" "$V" listing_requests "$reqs"; rec "$EXP" "$V" truncated_by_bound "$trunc"
    for k in "${!EXT[@]}"; do printf 'EXT\t%s\t%s\t%s\t%s\n' "$EXP" "$V" "$k" "${EXT[$k]}" >>"$OUT"; done
    for k in "${!MODEF[@]}"; do printf 'MODEF\t%s\t%s\t%s\t%s\n' "$EXP" "$V" "$k" "${MODEF[$k]}" >>"$OUT"; done
    for k in "${!MODED[@]}"; do printf 'MODED\t%s\t%s\t%s\t%s\n' "$EXP" "$V" "$k" "${MODED[$k]}" >>"$OUT"; done
    pick="${candge:-$candlt}"; psz="${candgesz}"; [ -n "$candge" ] || psz="$candltsz"
    if [ -n "$pick" ]; then
      sleep 1; t0=$(now_ms); n="$(timeout 60 nfs-cat "$(url "$pick")" 2>/dev/null | head -c "$READ" | wc -c)"; t1=$(now_ms)
      rec "$EXP" "$V" read_bytes "$n"; rec "$EXP" "$V" read_ms_including_mount "$((t1-t0))"; rec "$EXP" "$V" read_file_size_class "$([ -n "$candge" ] && echo ge_1MiB || echo lt_1MiB)"
    else rec "$EXP" "$V" read_bytes no_file_in_sample; fi
    unset EXT MODEF MODED
  done
done <"$EXF"
rec "-" "-" exports_walked "$n_ex"; rec "-" "-" writes_performed 0
pass "nfs host $IDX exports=$n_ex"
