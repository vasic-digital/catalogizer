#!/usr/bin/env bash
# probe_webdav.sh - T128: an authenticated WebDAV PROPFIND (Depth 1) answers 207 Multi-Status listing the seeded corpus, and the same request
# without credentials is refused (401).
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
H="$(host_of WEBDAV webdav)"
cc="$(secret_file "user = \"$TI_WEBDAV_USER:$TI_WEBDAV_PASSWORD\"")"
body="$(curl -sS -m 15 -K "$cc" -X PROPFIND -H 'Depth: 1' -o /tmp/ti-dav.xml -w '%{http_code}' "http://$H/movies/" 2>&1)" || fail "webdav PROPFIND failed: ${body:0:120}"
[ "$body" = 207 ] || fail "webdav PROPFIND answered HTTP $body, want 207"
grep -q 'The.Matrix.1999.1080p.BluRay.mkv' /tmp/ti-dav.xml || fail "webdav PROPFIND body lacks the seeded file"
code="$(curl -sS -m 15 -X PROPFIND -H 'Depth: 1' -o /dev/null -w '%{http_code}' "http://$H/movies/" 2>&1)"
[ "$code" = 401 ] || fail "webdav answered HTTP $code to an unauthenticated PROPFIND, want 401"
pass "webdav PROPFIND answered 207 with the seeded corpus; the unauthenticated request got 401"
