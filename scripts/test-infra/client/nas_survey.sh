#!/usr/bin/env bash
# nas_survey.sh - READ-ONLY bounded survey of the disk shares of ONE Synology host (inside IMG-INFRA-CLIENT). Only `smbclient -L` and `ls` of directories (breadth-first,
# depth <= 3, <= 5000 entries and <= 80 listing requests per share, 1 request/second). No write-class command exists here. Credentials: auth file /out/auth (0600), never argv/env.
# Output /out/survey-<idx>.json: per share status, top-level dir/file counts, sha256 of the sorted top-level names, extension histogram. Entry names are never recorded.
# Usage: nas_survey.sh <ip> <1-7>
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
IP=${1:-}; IDX=${2:-}; AF=/out/auth; OUT="/out/survey-$IDX.json"
[[ "$IP" =~ ^[0-9]{1,3}(\.[0-9]{1,3}){3}$ ]] && [[ "$IDX" =~ ^[1-7]$ ]] || fail "usage"
[ -r "$AF" ] || fail "no auth file"
REQ=0; req() { REQ=$((REQ+1)); sleep 1; }
req; shares="$(timeout 30 smbclient -L "//$IP" -A "$AF" -m SMB3 -g 2>&1)" || fail "share list failed"
mapfile -t SH < <(printf '%s\n' "$shares" | sed -n 's/^Disk|\([^|]*\)|.*/\1/p')
RES=''; addres() { RES="${RES:+$RES,}$1"; }
for s in "${SH[@]}"; do
  case "$s" in *[!A-Za-z0-9._-]*) continue;; esac
  req; top="$(timeout 60 smbclient "//$IP/$s" -A "$AF" -m SMB3 -c 'ls' 2>&1)"; trc=$?
  if [ $trc -ne 0 ] || printf '%s' "$top" | grep -q 'NT_STATUS'; then
    st="$(printf '%s' "$top" | grep -o 'NT_STATUS_[A-Z_]*' | head -1)"
    addres "$(printf '{"share":"%s","status":"denied_or_error","nt_status":"%s"}' "$s" "${st:-error}")"; continue
  fi
  parse() { sed -nE 's/^  (.*[^ ])  +([A-Z]+) +([0-9]+)  +[A-Z][a-z]{2} [A-Z][a-z]{2} .*$/\2\t\3\t\1/p' | grep -vP '^[A-Z]*\t[0-9]+\t\.\.?$'; }
  ent="$(printf '%s\n' "$top" | parse)"
  tn="$(printf '%s\n' "$ent" | grep -c .)"; td="$(printf '%s\n' "$ent" | awk -F'\t' '$1 ~ /D/{c++} END{print c+0}')"
  tsha="$(printf '%s\n' "$ent" | cut -f3 | LC_ALL=C sort | sha256sum | cut -d' ' -f1)"
  files=0; dirs=0; reqs=1; trunc=false; : >/tmp/ext.txt
  # queue of "depth<TAB>path"; the top level is already listed
  : >/tmp/q.txt
  while IFS=$'\t' read -r f sz nm; do
    [ -n "$nm" ] || continue
    if [[ "$f" == *D* ]]; then dirs=$((dirs+1)); case "$nm" in @*|\#*) ;; *) printf '1\t%s\n' "$nm" >>/tmp/q.txt;; esac
    else files=$((files+1)); case "$nm" in *.*) echo "${nm##*.}" | tr 'A-Z' 'a-z' >>/tmp/ext.txt;; *) echo "(none)" >>/tmp/ext.txt;; esac; fi
  done <<<"$ent"
  total=$tn
  while IFS=$'\t' read -r d p && [ -n "$p" ]; do
    if [ "$reqs" -ge 80 ] || [ "$total" -ge 5000 ]; then trunc=true; break; fi
    case "$p" in *\"*|*\\*) continue;; esac
    req; reqs=$((reqs+1)); out="$(timeout 60 smbclient "//$IP/$s" -A "$AF" -m SMB3 -c "cd \"$p\"; ls" 2>&1)" || continue
    e2="$(printf '%s\n' "$out" | parse)"; [ -n "$e2" ] || continue
    while IFS=$'\t' read -r f sz nm; do
      [ -n "$nm" ] || continue; total=$((total+1))
      if [[ "$f" == *D* ]]; then dirs=$((dirs+1)); case "$nm" in @*|\#*) ;; *) [ "$d" -lt 3 ] && printf '%s\t%s/%s\n' "$((d+1))" "$p" "$nm" >>/tmp/q.txt;; esac
      else files=$((files+1)); case "$nm" in *.*) echo "${nm##*.}" | tr 'A-Z' 'a-z' >>/tmp/ext.txt;; *) echo "(none)" >>/tmp/ext.txt;; esac; fi
    done <<<"$e2"
  done </tmp/q.txt   # reads the growing queue file: breadth-first
  hist="[$(sort /tmp/ext.txt | uniq -c | sort -rn | head -25 | awk '{printf "%s{\"ext\":\"%s\",\"count\":%s}",(NR>1?",":""),$2,$1}')]"
  addres "$(printf '{"share":"%s","status":"listed","top_level_entries":%s,"top_level_dirs":%s,"top_level_files":%s,"top_level_names_sha256":"%s","sampled_entries":%s,"sampled_files":%s,"sampled_dirs":%s,"listing_requests":%s,"truncated_by_bound":%s,"max_depth":3,"ext_histogram_top25":%s}' "$s" "$tn" "$td" "$((tn-td))" "$tsha" "$total" "$files" "$dirs" "$reqs" "$trunc" "$hist")"
done
printf '{"alias":"Synology%s","requests":%s,"writes_performed":0,"names_recorded":false,"shares":[%s]}
' "$IDX" "$REQ" "$RES" >"$OUT"
pass "survey host $IDX shares=${#SH[@]} requests=$REQ writes=0"
