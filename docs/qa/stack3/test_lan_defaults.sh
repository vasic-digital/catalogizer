#!/usr/bin/env bash
# Sandbox test (fake ip/podman/jq/curl): how sonarqube/lib.sh, sonarqube/up.sh|down.sh and s3/up.sh pick their bind address (CAT-LANIP-DYN-1).
# usage: test_lan_defaults.sh [STACK3_DIR]   (STACK3_DIR override lets the mutation runner target a mutated copy of docs/qa/stack3 + lib)   exit 0 = all pass
set -uo pipefail
S3D="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)}"
R="$(mktemp -d)" || exit 1; [ -n "$R" ] && [ -d "$R" ] || exit 1; trap 'rm -rf -- "${R:?}"' EXIT
mkdir -p "$R/bin" "$R/sq" "$R/s3"
cat > "$R/bin/ip" <<'S'
#!/usr/bin/env bash
case "$*" in *"route get"*) [ -n "${FAKE_SRC-}" ] || exit 1; echo "1.1.1.1 via 10.0.0.1 dev eth0 src ${FAKE_SRC} uid 1000";;
  *"addr show"*) for a in ${FAKE_ADDRS-}; do echo "2: eth0    inet $a/24 scope global eth0"; done;; *) exit 1;; esac
S
cat > "$R/bin/podman" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "${PLOG:?}"; exit 1
S
printf '#!/usr/bin/env bash\nexit 0\n' > "$R/bin/jq"; cp "$R/bin/jq" "$R/bin/curl"
cat > "$R/bin/podmanok" <<'S'
#!/usr/bin/env bash
echo "podman $*" >> "${PLOG:?}"; exit 0
S
chmod +x "$R/bin/"*
PLOG="$R/plog"; export PLOG
P=0; F=0; ok() { if [ "$1" = 0 ]; then P=$((P+1)); echo "PASS $2"; else F=$((F+1)); echo "FAIL $2"; fi; }
env_() { env -u SONAR_BIND -u S3_BIND -u QA_LAN_IP -u QA_BIND -u LAN_BIND -u FAKE_SRC -u FAKE_ADDRS PATH="$R/bin:$PATH" SQ_DIR="$R/sq" S3_STATE="$R/s3" "$@"; }
libval() { env_ "$@" bash -c '. "$0/sonarqube/lib.sh" >/dev/null 2>&1; echo "rc=$? bind=${SONAR_BIND-<unset>} unresolved=${SONAR_BIND_UNRESOLVED-0}"' "$S3D"; }
# --- sonarqube/lib.sh
v="$(libval FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5)"; [ "$v" = "rc=0 bind=10.0.0.5 unresolved=0" ]; ok $? "sonar lib: SONAR_BIND unset -> detected address (got '$v')"
v="$(libval FAKE_SRC=10.0.0.5 FAKE_ADDRS="10.0.0.5 10.0.0.6" QA_LAN_IP=10.0.0.6)"; [ "$v" = "rc=0 bind=10.0.0.6 unresolved=0" ]; ok $? "sonar lib: QA_LAN_IP override used (got '$v')"
v="$(libval FAKE_SRC=10.0.0.5 FAKE_ADDRS="10.0.0.5 10.0.0.6" SONAR_BIND=10.0.0.6 LAN_BIND=0.0.0.0 QA_BIND=0.0.0.0)"; [ "$v" = "rc=0 bind=10.0.0.6 unresolved=0" ]; ok $? "sonar lib: explicit SONAR_BIND kept; LAN_BIND/QA_BIND wildcards ignored (got '$v')"
v="$(libval FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 LAN_BIND=0.0.0.0 QA_BIND=0.0.0.0)"; case "$v" in *"bind=0.0.0.0"*) false;; *) true;; esac; ok $? "sonar lib: never 0.0.0.0 from the environment (got '$v')"
# M2: teardown/scan paths must work when the LAN address cannot be detected
v="$(libval FAKE_ADDRS=10.0.0.5)"; [ "$v" = "rc=0 bind=127.0.0.1 unresolved=1" ]; ok $? "sonar lib: undetectable -> sourcing still succeeds, loopback + unresolved flag (got '$v')"
# --- sonarqube/down.sh must not need LAN detection (podman shim fails every call => 'pod not present' path)
( env_ FAKE_ADDRS=10.0.0.5 bash "$S3D/sonarqube/down.sh" >"$R/dout" 2>&1 ); drc=$?
[ "$drc" = 0 ] && grep -q 'not present' "$R/dout"; ok $? "sonar down.sh works with no detectable LAN address (rc=$drc)"
# --- sonarqube/up.sh must refuse (fail closed) before touching podman when unresolved
: > "$PLOG"; ( env_ FAKE_ADDRS=10.0.0.5 bash "$S3D/sonarqube/up.sh" >"$R/uout" 2>&1 ); urc=$?
[ "$urc" != 0 ] && grep -q 'LAN address' "$R/uout" && ! grep -q 'pod create' "$PLOG"; ok $? "sonar up.sh refuses when the LAN address is unresolved, no pod created (rc=$urc)"
# --- s3/up.sh
s3pub() { : > "$PLOG"; rm -f -- "${R:?}/bin/podman.keep"; mv "$R/bin/podman" "$R/bin/podman.keep"; cp "$R/bin/podmanok" "$R/bin/podman"
          ( env_ "$@" timeout 30 bash "$S3D/s3/up.sh" >"$R/s3out" 2>&1 ); echo $? > "$R/s3rc"; mv -f "$R/bin/podman.keep" "$R/bin/podman"
          sed -n 's/.*podman run .* -p \([^ ]*\) .*/\1/p' "$PLOG" | head -n1; }
v="$(s3pub FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5)"; [ "$v" = "127.0.0.1:39320:3900" ]; ok $? "s3 up.sh: default publishes on 127.0.0.1 (got '$v')"
v="$(s3pub FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 LAN_BIND=0.0.0.0 QA_BIND=0.0.0.0)"; [ "$v" = "127.0.0.1:39320:3900" ]; ok $? "s3 up.sh: LAN_BIND/QA_BIND wildcards do not change the default (got '$v')"
v="$(s3pub FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 S3_BIND=lan)"; [ "$v" = "10.0.0.5:39320:3900" ]; ok $? "s3 up.sh: S3_BIND=lan -> detected address (got '$v')"
v="$(s3pub FAKE_SRC=10.0.0.5 FAKE_ADDRS="10.0.0.5 10.0.0.6" S3_BIND=lan QA_LAN_IP=10.0.0.6)"; [ "$v" = "10.0.0.6:39320:3900" ]; ok $? "s3 up.sh: S3_BIND=lan honours QA_LAN_IP (got '$v')"
v="$(s3pub FAKE_SRC=10.0.0.5 FAKE_ADDRS=10.0.0.5 S3_BIND=10.0.0.5 LAN_BIND=0.0.0.0)"; [ "$v" = "10.0.0.5:39320:3900" ]; ok $? "s3 up.sh: explicit S3_BIND kept, LAN_BIND ignored (got '$v')"
v="$(s3pub FAKE_ADDRS=10.0.0.5 S3_BIND=lan)"; [ -z "$v" ] && [ "$(cat "$R/s3rc")" != 0 ]; ok $? "s3 up.sh: S3_BIND=lan with no detectable address fails, nothing published (got '$v', rc=$(cat "$R/s3rc"))"
echo "passed=$P failed=$F"; [ "$F" = 0 ]
