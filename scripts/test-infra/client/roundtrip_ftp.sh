#!/usr/bin/env bash
# roundtrip_ftp.sh - T132: FTP in PASSIVE mode: download corpus files (ASCII and unicode names) and compare their sha256 with the corpus manifest, upload a new
# file, download it back and compare, list it, delete it, confirm it is gone; a wrong password is refused.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of FTP ftp)"; N="$(nonce)"
HEAD=("set ftp:passive-mode on" "set ftp:ssl-allow no" "set net:max-retries 2" "set net:reconnect-interval-base 1" "set net:persist-retries 0" "set dns:max-retries 1" "set dns:fatal-timeout 5" "set net:timeout 10" "set ftp:charset UTF-8" "set file:charset UTF-8" "open -u $TI_FTP_USER,$TI_FTP_PASSWORD ftp://$H")
run() { local f; f="$(secret_file "${HEAD[@]}" "$@")"; lftp -f "$f"; }
head -c 20480 /dev/urandom >/tmp/up.bin; UPSHA="$(sha256sum /tmp/up.bin | cut -d' ' -f1)"
get_check() { local rel=$1 want; want="$(manifest_sha "$rel")"; [ -n "$want" ] || { echo "no manifest entry for $rel"; return 1; }
  rm -f /tmp/dl.bin; run "get '$rel' -o /tmp/dl.bin" >/dev/null && expect_eq "$(sha256sum /tmp/dl.bin | cut -d' ' -f1)" "$want"; }
listed() { run 'cls -1' | grep -qxF "$1"; }
# a name is "gone" only when a listing that WORKS (it shows the corpus directory `movies`) lacks it: a failed listing is no evidence (WF12 F8)
not_listed() { local o; o="$(run 'cls -1' 2>&1)" || { echo "the listing failed: ${o:0:80}"; return 1; }; printf '%s\n' "$o" | grep -qE '^movies/?$' || { echo "control: the listing lacks movies"; return 1; }; ! printf '%s\n' "$o" | grep -qxF "$1"; }
back_equals() { rm -f /tmp/back.bin; run "get '$1' -o /tmp/back.bin" >/dev/null && expect_eq "$(sha256sum /tmp/back.bin | cut -d' ' -f1)" "$UPSHA"; }
refused_pw() { local f; f="$(mktemp)"; chmod 600 "$f"; printf '%s\n' 'set ftp:passive-mode on' 'set net:max-retries 1' 'set net:persist-retries 0' 'set dns:max-retries 1' 'set dns:fatal-timeout 5' 'set net:timeout 8' "open -u $TI_FTP_USER,wrong-credential-1 ftp://$H" 'cls -1' >"$f"; local o; o="$(lftp -f "$f" 2>&1)" && { echo "a wrong password was accepted"; return 1; }; case "$o" in *530*) ;; *) echo "refused, but not with FTP 530: ${o:0:100}"; return 1;; esac; }
step corpus_ascii_file get_check "movies/The.Matrix.1999.1080p.BluRay.mkv"
step corpus_nested_file get_check "music/Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac"
step corpus_unicode_file get_check $'music/Björk - Jóga (Ünicöde).flac'
step corpus_cjk_file get_check $'movies/千と千尋の神隠し.2001.mkv'
step upload run "put /tmp/up.bin -o up-$N.bin"
step list_shows_upload listed "up-$N.bin"
step download_back back_equals "up-$N.bin"
step delete_upload run "rm up-$N.bin"
step upload_gone not_listed "up-$N.bin"
step wrong_password_refused refused_pw
finish ftp
