#!/usr/bin/env bash
# Guard (review round 2, S3): with the shipped .env.example, the rendered compose config
# must give the api container MEDIA_ROOT_PATH equal to the container path where the media
# library is mounted (/media), so the default read root of docs/security/PATH_CONFINEMENT.md
# is the library and not /app/media. Also asserts the default write root /app/data is a
# named volume. Renders through the real compose provider (`podman compose config`), not a
# grep of the YAML text, so ${VAR:-default} expansion is the provider's own.
# Control: a fixture carrying the pre-fix form `${MEDIA_ROOT_PATH:-/media}` MUST fail.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
command -v podman >/dev/null 2>&1 || { echo "SKIP - podman not installed (not a PASS)"; exit 0; }
podman compose version >/dev/null 2>&1 || { echo "SKIP - no podman compose provider (not a PASS)"; exit 0; }

check() { # $1 = compose file; prints the reason and returns 1 on failure
    local out
    out=$(podman compose --env-file "$PWD/.env.example" -f "$1" config 2>/dev/null) || { echo "render failed: $1"; return 2; }
    printf '%s\n' "$out" | sed -n '/^networks:\|^services:\|^volumes:/,$p' | python3 -c '
import sys, yaml
doc = yaml.safe_load(sys.stdin)
api = doc["services"]["api"]
env = api.get("environment") or {}
if isinstance(env, list):
    env = dict(e.split("=", 1) for e in env)
media = str(env.get("MEDIA_ROOT_PATH", ""))
targets = []
for v in api.get("volumes") or []:
    if isinstance(v, str):
        parts = v.split(":")
        targets.append((parts[0], parts[1] if len(parts) > 1 else parts[0]))
    else:
        targets.append((str(v.get("source")), str(v.get("target"))))
bad = []
if media not in [t for _, t in targets]:
    bad.append("MEDIA_ROOT_PATH=%r is not a container mount target %r" % (media, [t for _, t in targets]))
read = str(env.get("CATALOGIZER_READ_ROOTS", ""))
if read and media not in read.split(":"):
    bad.append("CATALOGIZER_READ_ROOTS=%r omits %r" % (read, media))
if not any(t == "/app/data" and not s.startswith(("/", ".")) for s, t in targets):
    bad.append("/app/data (default write root) is not a named volume")
if bad:
    print("; ".join(bad)); sys.exit(1)
print("ok MEDIA_ROOT_PATH=%s" % media)
'
}

tmp=$(mktemp -d); trap 'rm -rf "${tmp:?}"' EXIT
# Control needle: the pre-fix form must be detected.
sed 's|^      MEDIA_ROOT_PATH: /media$|      MEDIA_ROOT_PATH: ${MEDIA_ROOT_PATH:-/media}|' docker-compose.yml > "$PWD/.compose_media_selftest.yml"
trap 'rm -rf "${tmp:?}"; rm -f "$PWD/.compose_media_selftest.yml"' EXIT
if ! grep -q 'MEDIA_ROOT_PATH: ${MEDIA_ROOT_PATH:-/media}' "$PWD/.compose_media_selftest.yml"; then
    echo "SELFTEST FAIL: could not build the pre-fix fixture"; exit 2
fi
if check "$PWD/.compose_media_selftest.yml" >"$tmp/ctl" 2>&1; then
    echo "SELFTEST FAIL: pre-fix compose passed: $(cat "$tmp/ctl")"; exit 2
fi
echo "selftest: pre-fix fixture rejected ($(cat "$tmp/ctl"))"

if check docker-compose.yml; then echo PASS; exit 0; fi
echo FAIL; exit 1
