#!/usr/bin/env bash
# roundtrip_smb.sh - T132: SMB (SMB3, authenticated): download corpus files (ASCII and unicode) and compare sha256 with the manifest, upload a file, download
# it back and compare, list it, delete it, confirm it is gone; a wrong password is refused.
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of SMB smb)"; N="$(nonce)"
AF="$(secret_file "username = $TI_SMB_USER" "password = $TI_SMB_PASSWORD")"
sm() { timeout 60 smbclient "//$H/testshare" -A "$AF" -m SMB3 -c "$1"; }
head -c 20480 /dev/urandom >/tmp/up.bin; UPSHA="$(sha256sum /tmp/up.bin | cut -d' ' -f1)"
get_check() { local rel=$1 want dir base; want="$(manifest_sha "$rel")"; [ -n "$want" ] || { echo "no manifest entry for $rel"; return 1; }
  dir="$(dirname "$rel")"; base="$(basename "$rel")"; rm -f /tmp/dl.bin; sm "cd \"$dir\"; get \"$base\" /tmp/dl.bin" >/dev/null && expect_eq "$(sha256sum /tmp/dl.bin | cut -d' ' -f1)" "$want"; }
listed() { sm "ls $1" | grep -qF "$1"; }
not_listed() { ! sm "ls $1" 2>&1 | grep -qE "$1 +[A-Z]* +[0-9]+"; }
back_equals() { rm -f /tmp/back.bin; sm "get $1 /tmp/back.bin" >/dev/null && expect_eq "$(sha256sum /tmp/back.bin | cut -d' ' -f1)" "$UPSHA"; }
refused_pw() { local f; f="$(mktemp)"; chmod 600 "$f"; printf '%s\n' "username = $TI_SMB_USER" 'password = wrong-credential-1' >"$f"; ! timeout 30 smbclient "//$H/testshare" -A "$f" -m SMB3 -c 'ls' >/dev/null 2>&1; }
step corpus_ascii_file get_check "movies/The.Matrix.1999.1080p.BluRay.mkv"
step corpus_nested_file get_check "music/Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac"
step corpus_unicode_file get_check $'music/Björk - Jóga (Ünicöde).flac'
step corpus_cjk_file get_check $'movies/千と千尋の神隠し.2001.mkv'
step upload sm "put /tmp/up.bin up-$N.bin"
step list_shows_upload listed "up-$N.bin"
step download_back back_equals "up-$N.bin"
step delete_upload sm "del up-$N.bin"
step upload_gone not_listed "up-$N.bin"
step wrong_password_refused refused_pw
finish smb
