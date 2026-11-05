#!/usr/bin/env bash
# Sandbox tests (no real container, port or credential): fake curl/podman on PATH inside a mktemp dir.
#  N1: the admin bearer must never appear in curl argv (it arrives on stdin), logout retries are bounded, previous token is persisted/removed.
#  N2: the digest check refuses an unlisted image unless ALLOW_UNPINNED_IMAGES=1 (loud warning), refuses a mismatch, accepts a match.
# usage: test_scripts.sh [--script <path-to-token_refresher.sh>]   (the override lets the RED run target a pre-fix copy)  exit 0 = all pass
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; SCRIPT="$HERE/token_refresher.sh"; [ "${1:-}" = --script ] && SCRIPT="$2"
S="$(mktemp -d)"; : "${S:?}"; trap 'rm -rf -- "${S:?}"' EXIT
fail=0; ok() { echo "PASS $1"; }; bad() { echo "FAIL $1 $2"; fail=1; }
mkdir -p "$S/bin" "$S/tok"
cat > "$S/bin/curl" <<'FC'
#!/bin/sh
# fake curl: records argv; stdin saved when the call reads it (@-); /login returns a rotating token, /logout returns $FAKE_LOGOUT_CODE
echo "$*" >> "$FAKE_DIR/argv.log"
case "$*" in *@-*) cat > "$FAKE_DIR/stdin.last"; cat "$FAKE_DIR/stdin.last" >> "$FAKE_DIR/stdin.log"; echo >> "$FAKE_DIR/stdin.log";; esac
case "$*" in
  */login*) n=$(($(cat "$FAKE_DIR/n" 2>/dev/null || echo 0)+1)); echo $n > "$FAKE_DIR/n"; printf '{"session_token":"SECRETTOKEN%s"}' "$n";;
  */logout*) echo x >> "$FAKE_DIR/logout.calls"; printf '%s' "${FAKE_LOGOUT_CODE:-200}";;
esac
FC
chmod +x "$S/bin/curl"; printf 'ADMIN_USERNAME=u"s\\er\nADMIN_PASSWORD=p"w\\d\n' > "$S/cred"
run() { # <logout-code>
  : > "$S/argv.log"; : > "$S/stdin.log"; : > "$S/logout.calls"; rm -f "$S/n" "$S/tok/"* 2>/dev/null
  FAKE_DIR="$S" FAKE_LOGOUT_CODE="$1" PATH="$S/bin:$PATH" CRED_FILE="$S/cred" TOK_DIR="$S/tok" TOK_CHOWN= LOGOUT_DELAY=0 CYCLE_SLEEP=0 MAX_CYCLES=3 \
    timeout "${TEST_RUN_TIMEOUT:-60}" sh "$SCRIPT" > "$S/out.log" 2> "$S/err.log"; }
run 200
grep -q SECRETTOKEN "$S/argv.log" && bad N1.bearer_not_in_argv "token found in curl argv" || ok N1.bearer_not_in_argv
grep -q 'Authorization: Bearer SECRETTOKEN1' "$S/stdin.log" && ok N1.bearer_sent_on_stdin || bad N1.bearer_sent_on_stdin "no stdin header"
grep -q 'u"s\\er' "$S/argv.log" && bad N1.credentials_not_in_argv "username in argv" || ok N1.credentials_not_in_argv
[ "$(wc -l < "$S/logout.calls")" = 2 ] && ok N1.two_logouts_for_three_cycles || bad N1.two_logouts_for_three_cycles "calls=$(wc -l < "$S/logout.calls")"
[ "$(cat "$S/tok/previous.tok" 2>/dev/null)" = SECRETTOKEN3 ] && ok N4.previous_token_persisted || bad N4.previous_token_persisted ""
m="$(stat -c %a "$S/tok/previous.tok" 2>/dev/null)"; { [ "$m" = 600 ] || [ "$m" = 400 ]; } && ok N4.previous_token_file_mode_owner_only || bad N4.previous_token_file_mode_owner_only "mode=$m"
[ "$(stat -c %a "$S/tok/metrics.tok" 2>/dev/null)" = 400 ] && ok F2.token_file_mode_0400 || bad F2.token_file_mode_0400 "$(stat -c %a "$S/tok/metrics.tok" 2>&1)"
# JSON escaping of the login body (stdin of the login call): must parse and round-trip
python3 -c 'import json,sys; d=json.loads(open(sys.argv[1]).read().splitlines()[0]); assert d=={"username":"u\"s\\er","password":"p\"w\\d"},d' "$S/stdin.log" 2>/dev/null && ok F3.login_json_escaped_roundtrip || bad F3.login_json_escaped_roundtrip ""
# restart persistence: a pre-existing previous.tok is revoked on the first refresh
: > "$S/argv.log"; : > "$S/stdin.log"; : > "$S/logout.calls"; rm -f "$S/n"; printf 'STALEOLD' > "$S/tok/previous.tok"
FAKE_DIR="$S" FAKE_LOGOUT_CODE=200 PATH="$S/bin:$PATH" CRED_FILE="$S/cred" TOK_DIR="$S/tok" TOK_CHOWN= LOGOUT_DELAY=0 CYCLE_SLEEP=0 MAX_CYCLES=1 timeout "${TEST_RUN_TIMEOUT:-30}" sh "$SCRIPT" >/dev/null 2>&1
grep -q 'Bearer STALEOLD' "$S/stdin.log" && ok N4.restart_revokes_persisted_previous || bad N4.restart_revokes_persisted_previous ""
run 500   # logout always fails: bounded retries (3 per cycle), then dropped
[ "$(wc -l < "$S/logout.calls")" = 6 ] && ok N1.failed_logout_retried_3x_per_cycle_then_dropped || bad N1.failed_logout_retried_3x_per_cycle_then_dropped "calls=$(wc -l < "$S/logout.calls")"
grep -q 'NOT revoked' "$S/err.log" && ok N1.failed_logout_reported || bad N1.failed_logout_reported ""

# N2: digest check
mkdir -p "$S/p"; cat > "$S/p/podman" <<'FP'
#!/bin/sh
# fake podman: `image inspect <img> --format ...` -> digest table
case "$3" in docker.io/a:1) echo sha256:AAA;; docker.io/b:1) echo sha256:BBB;; *) exit 1;; esac
FP
chmod +x "$S/p/podman"; printf 'docker.io/a:1 sha256:AAA\ndocker.io/b:1 sha256:WRONG\n' > "$S/pins"
. "$HERE/digest_check.sh"; export PATH="$S/p:$PATH"
check_image_digests "$S/pins" docker.io/a:1 >/dev/null 2>&1 && ok N2.matching_pin_accepted || bad N2.matching_pin_accepted ""
check_image_digests "$S/pins" docker.io/b:1 >/dev/null 2>&1 && bad N2.mismatch_refused "accepted" || ok N2.mismatch_refused
check_image_digests "$S/pins" docker.io/a:1 docker.io/unlisted:9 >"$S/o" 2>&1 && bad N2.unlisted_image_refused "accepted" || ok N2.unlisted_image_refused
ALLOW_UNPINNED_IMAGES=1 check_image_digests "$S/pins" docker.io/a:1 docker.io/unlisted:9 >"$S/o" 2>&1 && grep -q 'WARNING.*unlisted:9.*unpinned' "$S/o" && ok N2.optout_allows_with_loud_warning || bad N2.optout_allows_with_loud_warning "$(cat "$S/o")"
check_image_digests "$S/nonexistent-pins" docker.io/a:1 >/dev/null 2>&1 && bad N2.missing_pins_file_refused "accepted" || ok N2.missing_pins_file_refused
# QA-MON-FU-1: seed_scrape_token.sh writes a static METRICS_SCRAPE_TOKEN (stdin only) as the Prometheus credentials file; refuses short tokens
SEED="$HERE/seed_scrape_token.sh"; TOK32="$(printf 'A%.0s' $(seq 1 40))"
mkdir -p "$S/seed"; rm -f "$S/seed/"*
if [ -f "$SEED" ]; then
  printf '%s' "$TOK32" | TOK_DIR="$S/seed" TOK_CHOWN= sh "$SEED"; rc=$?
else rc=127; fi
[ "$rc" = 0 ] && [ "$(cat "$S/seed/metrics.tok" 2>/dev/null)" = "$TOK32" ] && ok FU1.token_seeded_from_stdin || bad FU1.token_seeded_from_stdin "rc=$rc"
[ "$(stat -c %a "$S/seed/metrics.tok" 2>/dev/null)" = 400 ] && ok FU1.seeded_token_mode_0400 || bad FU1.seeded_token_mode_0400 ""
rm -f "$S/seed/"*; if [ -f "$SEED" ]; then printf 'short' | TOK_DIR="$S/seed" TOK_CHOWN= sh "$SEED" 2>/dev/null; rc=$?; else rc=127; fi
[ "$rc" != 0 ] && [ "$rc" != 127 ] && [ ! -e "$S/seed/metrics.tok" ] && ok FU1.short_token_refused_nothing_written || bad FU1.short_token_refused_nothing_written "rc=$rc"
grep -q 'METRICS_SCRAPE_TOKEN' "$HERE/up.sh" && grep -q 'seed_scrape_token.sh' "$HERE/up.sh" && ok FU1.up_sh_uses_static_token_path || bad FU1.up_sh_uses_static_token_path "up.sh does not wire the seed"

# F4/F6 (opus review): seed boundary 31/32, chown failure is fatal, stale refresher session files are removed
rm -f "$S/seed/"*; T31="$(printf 'B%.0s' $(seq 1 31))"; T32="$(printf 'C%.0s' $(seq 1 32))"
printf '%s' "$T31" | TOK_DIR="$S/seed" TOK_CHOWN= sh "$SEED" >/dev/null 2>&1; rc31=$?
[ "$rc31" != 0 ] && [ ! -e "$S/seed/metrics.tok" ] && ok FU1.boundary_31_chars_refused || bad FU1.boundary_31_chars_refused "rc=$rc31"
printf '%s' "$T32" | TOK_DIR="$S/seed" TOK_CHOWN= sh "$SEED" >/dev/null 2>&1; rc32=$?
[ "$rc32" = 0 ] && [ "$(cat "$S/seed/metrics.tok" 2>/dev/null)" = "$T32" ] && ok FU1.boundary_32_chars_accepted || bad FU1.boundary_32_chars_accepted "rc=$rc32"
rm -f "$S/seed/"*; printf '%s' "$T32" | TOK_DIR="$S/seed" TOK_CHOWN=no_such_user_xx:no_such_grp_xx sh "$SEED" >/dev/null 2>&1; rcc=$?
[ "$rcc" != 0 ] && [ ! -e "$S/seed/metrics.tok" ] && ok FU1.chown_failure_is_fatal_nothing_published || bad FU1.chown_failure_is_fatal_nothing_published "rc=$rcc"
rm -f "$S/seed/"*; printf 'STALE_REFRESHER_SESSION' > "$S/seed/previous.tok"; printf '%s' "$T32" | TOK_DIR="$S/seed" TOK_CHOWN= sh "$SEED" >/dev/null 2>&1
[ ! -e "$S/seed/previous.tok" ] && ok FU1.stale_previous_tok_removed || bad FU1.stale_previous_tok_removed "previous.tok still present"
# up.sh behaviour with a stub podman/curl/ss: the token reaches the seeding container on STDIN only (argv and env names checked, not a name grep)
U="$S/up"; mkdir -p "$U/bin" "$U/qa"
cat > "$U/bin/podman" <<'FP'
#!/bin/bash
echo "podman $*" >> "$U_LOG"
case "$*" in *seed_scrape_token.sh*) cat > "$U_DIR/seed.stdin";; esac
case "$*" in *"cat /tokens/previous.tok"*) printf 'OLDADMINSESSIONTOKEN';; esac
case "$1 $2" in "pod exists") exit 1;; "image inspect") awk -v i="$3" '$1==i{print $2}' "$PINS"; exit 0;; esac
exit 0
FP
printf '#!/bin/sh\necho "curl $*" >> "$U_LOG"\ncase "$*" in *@-*) cat >> "$U_DIR/curl.stdin";; esac\nprintf 200\n' > "$U/bin/curl"; printf '#!/bin/sh\nexit 0\n' > "$U/bin/sleep"; printf '#!/bin/sh\nexit 0\n' > "$U/bin/ss"; chmod +x "$U/bin/"*
( umask 077; { echo ADMIN_USERNAME=a; echo ADMIN_PASSWORD=pw; echo POSTGRES_USER=c; echo POSTGRES_PASSWORD=pp; echo "METRICS_SCRAPE_TOKEN=$T32"; } > "$U/qa/qa.env" )
PINS="$HERE/evidence/image_digests.txt" U_LOG="$U/log" U_DIR="$U" QA_DIR="$U/qa" PATH="$U/bin:$PATH" timeout 60 bash "$HERE/up.sh" >/dev/null 2>&1 </dev/null   # </dev/null + timeout: a mutated up.sh that stops piping must FAIL the asserts below, never hang the stub
[ "$(cat "$U/seed.stdin" 2>/dev/null)" = "$T32" ] && ok FU1.up_feeds_token_on_stdin || bad FU1.up_feeds_token_on_stdin "stdin=$(wc -c < "$U/seed.stdin" 2>/dev/null)"
grep -qF -- "$T32" "$U/log" && bad FU1.token_value_never_in_argv "token value in podman argv" || ok FU1.token_value_never_in_argv
grep -qE -- '(-e|--env)[= ]METRICS_SCRAPE_TOKEN' "$U/log" && bad FU1.token_never_passed_as_env_to_monitoring "env pass-through" || ok FU1.token_never_passed_as_env_to_monitoring
grep -q 'auth/logout' "$U/log" && grep -q 'Bearer OLDADMINSESSIONTOKEN' "$U/curl.stdin" 2>/dev/null && ! grep -q OLDADMINSESSIONTOKEN "$U/log" && ok FU1.stale_refresher_session_revoked_token_on_stdin_only || bad FU1.stale_refresher_session_revoked_token_on_stdin_only "no logout / token in argv"
grep -q -- 'tokenrefresh' "$U/log" && bad FU1.no_refresher_started_in_static_mode "refresher started" || ok FU1.no_refresher_started_in_static_mode

[ "$fail" = 0 ] && echo "ALL PASS" || echo "SOME FAILED"; exit "$fail"
