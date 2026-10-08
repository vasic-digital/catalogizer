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
# a name is LISTED only when a listing that WORKS shows a directory-entry row for exactly that name: smbclient prints `NT_STATUS_NO_SUCH_FILE listing \<name>` (the name inside) on STDOUT with rc 1 for an absent file, so
# a bare substring match on the name always "finds" it (WF17 TI-G2: the upload could go to another name and the step still read ok). The status is the producer's (no pipe hides it).
listed() { local o rx; o="$(sm "ls $1")" || return 1; rx="$(printf '%s' "$1" | sed 's/[.[\*^$]/\\&/g')"; printf '%s\n' "$o" | grep -qE "^  $rx +[A-Z]* +[0-9]+ "; }
not_listed() { local o; o="$(sm "ls $1" 2>&1)" && { echo "the listing still shows $1"; return 1; }; case "$o" in *NT_STATUS_NO_SUCH_FILE*) ;; *) echo "not listed, but not NT_STATUS_NO_SUCH_FILE: ${o:0:100}"; return 1;; esac; }
back_equals() { rm -f /tmp/back.bin; sm "get $1 /tmp/back.bin" >/dev/null && expect_eq "$(sha256sum /tmp/back.bin | cut -d' ' -f1)" "$UPSHA"; }
refused_pw() { local f; f="$(mktemp)"; chmod 600 "$f"; printf '%s\n' "username = $TI_SMB_USER" 'password = wrong-credential-1' >"$f"; local o; o="$(timeout 30 smbclient "//$H/testshare" -A "$f" -m SMB3 -c 'ls' 2>&1)" && { echo "a wrong password was accepted"; return 1; }; case "$o" in *NT_STATUS_LOGON_FAILURE*) ;; *) echo "refused, but not NT_STATUS_LOGON_FAILURE: ${o:0:100}"; return 1;; esac; }
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
