#!/usr/bin/env bash
# marker.sh - T133 (inside IMG-INFRA-CLIENT): put / check a marker file on the WebDAV server and a marker row in PostgreSQL of THIS project, to prove two
# projects running at once do not share data. Usage: marker.sh put|has|hasnot <name>; prints `PASS ...` or `FAIL ...` (exit 0 / 1).
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
act=${1:-}; name=${2:-}; [ -n "$act" ] && [ -n "$name" ] || fail "usage: marker.sh put|has|hasnot <name>"
cc="$(secret_file "user = \"$TI_WEBDAV_USER:$TI_WEBDAV_PASSWORD\"")"
q() { PGPASSWORD="$TI_POSTGRES_PASSWORD" PGCONNECT_TIMEOUT=8 psql -h "$(host_of POSTGRES postgres)" -U "$TI_POSTGRES_USER" -d "$TI_POSTGRES_DB" -v ON_ERROR_STOP=1 -tA "$@"; }
dav_has() { [ "$(curl -sS -m 15 -K "$cc" -o /dev/null -w '%{http_code}' "http://$(host_of WEBDAV webdav)/marker-$name.txt")" = 200 ]; }
pg_has() { [ "$(q -c "SELECT count(*) FROM ti_marker WHERE name = '$name'" 2>/dev/null)" = 1 ]; }
case "$act" in
  put) printf 'marker %s\n' "$name" >/tmp/m.txt
       c="$(curl -sS -m 15 -K "$cc" -T /tmp/m.txt -o /dev/null -w '%{http_code}' "http://$(host_of WEBDAV webdav)/marker-$name.txt")"; { [ "$c" = 201 ] || [ "$c" = 204 ]; } || fail "webdav put answered $c"
       q -c 'CREATE TABLE IF NOT EXISTS ti_marker (name text PRIMARY KEY)' >/dev/null && q -c "INSERT INTO ti_marker VALUES ('$name')" >/dev/null || fail "postgres marker insert failed"
       pass "marker $name put (webdav file and postgres row)";;
  has) dav_has && pg_has && pass "marker $name present in webdav and postgres" || fail "marker $name missing";;
  hasnot) if dav_has; then fail "marker $name found in webdav of this project"; elif pg_has; then fail "marker $name found in postgres of this project"; else pass "marker $name absent from webdav and postgres"; fi;;
  *) fail "unknown action $act";;
esac
