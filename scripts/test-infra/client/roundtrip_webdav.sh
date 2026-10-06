#!/usr/bin/env bash
# roundtrip_webdav.sh - T132: WebDAV (basic authentication): GET corpus files (ASCII and unicode, percent-encoded) and compare sha256 with the manifest, MKCOL a
# collection, PUT a file, GET it back and compare, PROPFIND lists it, DELETE it (204) and a GET afterwards is 404; an unauthenticated PUT is refused (401).
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of WEBDAV webdav)"; N="$(nonce)"
CC="$(secret_file "user = \"$TI_WEBDAV_USER:$TI_WEBDAV_PASSWORD\"")"
cv() { curl -sS -m 30 -K "$CC" "$@"; }
code_is() { local want=$1; shift; expect_eq "$(cv -o /dev/null -w '%{http_code}' "$@")" "$want"; }
enc() { printf '%s' "$1" | perl -pe 's/([^A-Za-z0-9._~\/-])/sprintf("%%%02X", ord($1))/ge'; }
head -c 20480 /dev/urandom >/tmp/up.bin; UPSHA="$(sha256sum /tmp/up.bin | cut -d' ' -f1)"
get_check() { local rel=$1 want; want="$(manifest_sha "$rel")"; [ -n "$want" ] || { echo "no manifest entry for $rel"; return 1; }
  rm -f /tmp/dl.bin; [ "$(cv -o /tmp/dl.bin -w '%{http_code}' "http://$H/$(enc "$rel")")" = 200 ] && expect_eq "$(sha256sum /tmp/dl.bin | cut -d' ' -f1)" "$want"; }
put_ok() { local c; c="$(cv -T /tmp/up.bin -o /dev/null -w '%{http_code}' "http://$H/ti-$N/up.bin")"; [ "$c" = 201 ] || [ "$c" = 204 ]; }
back_equals() { rm -f /tmp/back.bin; cv -o /tmp/back.bin "http://$H/ti-$N/up.bin" && expect_eq "$(sha256sum /tmp/back.bin | cut -d' ' -f1)" "$UPSHA"; }
lists() { cv -X PROPFIND -H 'Depth: 1' "http://$H/ti-$N/" | grep -q 'up.bin'; }
unauth_put() { expect_eq "$(curl -sS -m 15 -T /tmp/up.bin -o /dev/null -w '%{http_code}' "http://$H/ti-denied-$N.bin")" 401; }
step corpus_ascii_file get_check "movies/The.Matrix.1999.1080p.BluRay.mkv"
step corpus_nested_file get_check "music/Pink Floyd/The Dark Side of the Moon/01 - Speak to Me.flac"
step corpus_unicode_file get_check $'music/Björk - Jóga (Ünicöde).flac'
step corpus_cjk_file get_check $'movies/千と千尋の神隠し.2001.mkv'
step mkcol code_is 201 -X MKCOL "http://$H/ti-$N/"
step put_file put_ok
step get_back back_equals
step propfind_lists lists
step delete_file code_is 204 -X DELETE "http://$H/ti-$N/up.bin"
step get_after_delete_404 code_is 404 "http://$H/ti-$N/up.bin"
step delete_collection code_is 204 -X DELETE "http://$H/ti-$N/"
step unauthenticated_put_refused unauth_put
finish webdav
