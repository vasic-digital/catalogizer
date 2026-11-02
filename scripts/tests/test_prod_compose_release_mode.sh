#!/usr/bin/env bash
# Gate (CAT-021 I2): every shipped compose service built from the production
# catalog-api/Dockerfile must set GIN_MODE with a release default, otherwise
# CATALOGIZER_ALLOW_INSECURE_DEFAULTS would not be refused there.
# Self-test: a bad fixture must FAIL and a good fixture must PASS.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
check() { python3 - "$@" <<'PY'
import sys, yaml, re
bad = 0
for f in sys.argv[1:]:
    d = yaml.safe_load(open(f)) or {}
    for name, svc in (d.get("services") or {}).items():
        b = svc.get("build")
        df = b.get("dockerfile") if isinstance(b, dict) else None
        if df != "catalog-api/Dockerfile":
            continue
        env = svc.get("environment") or {}
        if isinstance(env, list):
            env = dict(e.split("=", 1) if "=" in e else (e, "") for e in env)
        v = str(env.get("GIN_MODE", ""))
        if not re.fullmatch(r"release|\$\{GIN_MODE:-release\}", v):
            print(f"FAIL {f}: service {name} lacks GIN_MODE release default"); bad = 1
sys.exit(bad)
PY
}
tmp=$(mktemp -d); trap 'rm -rf "${tmp:?}"' EXIT
printf 'services:\n  a:\n    build: {dockerfile: catalog-api/Dockerfile}\n    environment: {X: "1"}\n' > "$tmp/bad.yml"
printf 'services:\n  a:\n    build: {dockerfile: catalog-api/Dockerfile}\n    environment: {GIN_MODE: "${GIN_MODE:-release}"}\n' > "$tmp/good.yml"
check "$tmp/bad.yml" >/dev/null && { echo "SELFTEST FAIL: bad fixture passed"; exit 2; }
check "$tmp/good.yml" || { echo "SELFTEST FAIL: good fixture failed"; exit 2; }
check docker-compose*.yml deploy/*.yml || exit 1
echo "PASS: production compose files carry GIN_MODE release"
