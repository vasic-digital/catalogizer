#!/usr/bin/env bash
# probe_ftp.sh - T128: FTP login with the generated credentials, a PASSIVE-mode directory listing that shows the seeded corpus, and a passive-mode
# transfer of one seeded file whose sha256 equals the corpus manifest. Active mode is disabled so a pass cannot come from the active path.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of FTP ftp)"
head=("set ftp:passive-mode on" "set ftp:ssl-allow no" "set net:max-retries 2" "set net:reconnect-interval-base 1" "set net:persist-retries 0" "set dns:max-retries 1" "set dns:fatal-timeout 5" "set net:timeout 8" "open -u $TI_FTP_USER,$TI_FTP_PASSWORD ftp://$H")
cf="$(secret_file "${head[@]}" "cls -1 movies" "bye")"
ls="$(lftp -f "$cf" 2>&1)" || fail "ftp login or passive listing failed: ${ls:0:120}"
case "$ls" in *The.Matrix.1999.1080p.BluRay.mkv*) ;; *) fail "ftp passive listing lacks the seeded file (got '${ls:0:120}')";; esac
want="$(sed -n 's#^\([0-9a-f]\{64\}\)  \./*movies/The.Matrix.1999.1080p.BluRay.mkv$#\1#p' /manifest.sha256 2>/dev/null | head -1)"
[ -n "$want" ] || fail "corpus manifest has no entry for the transfer check (/manifest.sha256 missing or malformed)"
cg="$(secret_file "${head[@]}" "get movies/The.Matrix.1999.1080p.BluRay.mkv -o /tmp/ti-ftp.bin" "bye")"
lftp -f "$cg" >/dev/null 2>&1 || fail "ftp passive transfer failed"
got="$(sha256sum /tmp/ti-ftp.bin | cut -d' ' -f1)"
[ "$got" = "$want" ] || fail "ftp passive transfer sha256 $got != manifest $want"
pass "ftp login, passive listing and passive transfer (sha256 equals the corpus manifest)"
