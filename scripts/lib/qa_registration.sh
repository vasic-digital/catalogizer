#!/bin/bash
# QA helper for the registration policy (REGISTRATION_MODE, default "approval").
#
# Under the default policy a self-registered account is created INACTIVE ("pending") and cannot
# log in (HTTP 403) until an administrator approves it. QA scripts whose subject is something
# OTHER than the registration policy should either run the stack with REGISTRATION_MODE=open
# (see docker-compose.qa.yml) or call qa_register_and_activate below, which works in both modes
# and so does not depend on how the stack was started.
#
# Source this file; it defines functions only and has no side effects.
#   qa_curl_code <curl args...>       prints the 3-digit HTTP code ("000" on a transport failure;
#                                      never "000000", never a -f artefact); body in $QA_BODY_FILE
#   qa_register API USER EMAIL PASS   POST /auth/register, prints the HTTP code
#   qa_admin_approve API ADMIN_TOKEN USER_ID   PUT /admin/users/:id {"status":"active"} with the
#                                      CSRF cookie+header the admin group requires; prints code
#   qa_register_and_activate API ADMIN_TOKEN USER EMAIL PASS
#                                      registers; if the account is pending, approves it via the
#                                      admin API; returns 0 and prints "<user_id>" when the
#                                      account can then log in, non-zero otherwise.

QA_BODY_FILE="${QA_BODY_FILE:-$(mktemp "${TMPDIR:-/tmp}/qa_body.XXXXXX")}"

qa_curl_code() {
    local code
    code=$(curl -s -o "$QA_BODY_FILE" -w '%{http_code}' "$@" 2>/dev/null) || true
    [[ "$code" =~ ^[0-9]{3}$ ]] || code="000"
    printf '%s' "$code"
}

qa_register() {
    local api="$1" user="$2" email="$3" pass="$4"
    qa_curl_code -X POST -H 'Content-Type: application/json' \
        -d "{\"username\":\"${user}\",\"email\":\"${email}\",\"password\":\"${pass}\",\"first_name\":\"QA\",\"last_name\":\"User\"}" \
        "${api}/api/v1/auth/register"
}

qa_admin_approve() {
    local api="$1" token="$2" uid="$3" hdrs csrf_hdr csrf_cookie
    hdrs=$(curl -s -D - -o /dev/null -H "Authorization: Bearer ${token}" "${api}/api/v1/admin/users" 2>/dev/null) || true
    csrf_hdr=$(printf '%s\n' "$hdrs" | tr -d '\r' | sed -n 's/^[Xx]-[Cc]srf-[Tt]oken: *//p' | head -1)
    csrf_cookie=$(printf '%s\n' "$hdrs" | tr -d '\r' | sed -n 's/^[Ss]et-[Cc]ookie: *\(__Host-csrf=[^;]*\).*/\1/p' | head -1)
    if [ -z "$csrf_hdr" ] || [ -z "$csrf_cookie" ]; then printf '000'; return 0; fi
    qa_curl_code -X PUT -H 'Content-Type: application/json' -H "Authorization: Bearer ${token}" \
        -H "X-CSRF-Token: ${csrf_hdr}" -H "Cookie: ${csrf_cookie}" \
        -d '{"status":"active"}' "${api}/api/v1/admin/users/${uid}"
}

qa_register_and_activate() {
    local api="$1" token="$2" user="$3" email="$4" pass="$5" code uid active
    code=$(qa_register "$api" "$user" "$email" "$pass")
    [ "$code" = "201" ] || { echo "register ${user}: HTTP ${code}" >&2; return 1; }
    uid=$(python3 -c 'import sys,json; d=json.load(open(sys.argv[1])); print(d.get("id",""))' "$QA_BODY_FILE" 2>/dev/null)
    active=$(python3 -c 'import sys,json; d=json.load(open(sys.argv[1])); print(str(d.get("is_active")).lower())' "$QA_BODY_FILE" 2>/dev/null)
    [ -n "$uid" ] || { echo "register ${user}: no id in response" >&2; return 1; }
    if [ "$active" != "true" ]; then
        code=$(qa_admin_approve "$api" "$token" "$uid")
        [ "$code" = "200" ] || { echo "approve ${user} (id ${uid}): HTTP ${code}" >&2; return 1; }
    fi
    code=$(qa_curl_code -X POST -H 'Content-Type: application/json' \
        -d "{\"username\":\"${user}\",\"password\":\"${pass}\"}" "${api}/api/v1/auth/login")
    [ "$code" = "200" ] || { echo "login ${user} after activation: HTTP ${code}" >&2; return 1; }
    printf '%s' "$uid"
}
