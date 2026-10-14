#!/usr/bin/env bash
# llmctl_vision_ensure.sh - detect llmctl and report the truthful state of the llmctl `vision` profile (port 8082).
#
# Usage: llmctl_vision_ensure.sh [--json] [--setup] [--start]
#   (no flag)  READ-ONLY: runs only `llmctl plan --json` and a GET /health probe. Starts nothing, downloads nothing.
#   --json     machine-readable verdict on stdout (single JSON object).
#   --setup    when llmctl is absent but the vendored checkout exists (or LLMCTL_BIN points at one), run `llmctl setup`
#              (doctor + build engines + plan). Never combined with a model start.
#   --start    operator decision: run `llmctl start vision` ONLY when the plan says the profile is neither GATED nor
#              NO_FIT. A GATED or NO_FIT profile is never started or overridden by this script.
# Detection order: LLMCTL_BIN, `command -v llmctl`, <repo>/submodules/helix_qa/submodules/llmctl/bin/llmctl.
# Env: LLMCTL_VISION_URL (default http://127.0.0.1:8082), LLMCTL_BIN, LLMCTL_VENDORED_DIR, LLMCTL_ENSURE_TIMEOUT (seconds, 5).
# Exit codes: 0 READY | 3 ABSENT | 4 GATED | 5 NO_FIT | 6 NOT_RUNNING | 7 UNHEALTHY | 8 UNKNOWN (plan unreadable) | 2 usage.
# Secrets: this script never reads or prints the decide-gateway key (LLMCTL_API_KEY); it only talks to the vision port.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
JSON=0; SETUP=0; START=0
for a in "$@"; do
  case "$a" in
    --json) JSON=1;; --setup) SETUP=1;; --start) START=1;;
    -h|--help) sed -n 2,16p "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 0;;
    *) echo "llmctl_vision_ensure: unknown argument: $a" >&2; exit 2;;
  esac
done
URL="${LLMCTL_VISION_URL:-http://127.0.0.1:8082}"
TMO="${LLMCTL_ENSURE_TIMEOUT:-5}"
VENDORED="${LLMCTL_VENDORED_DIR:-$ROOT/submodules/helix_qa/submodules/llmctl}"

BIN=""; SRC="none"
if [ -n "${LLMCTL_BIN:-}" ]; then
  if [ -f "$LLMCTL_BIN" ] && [ -x "$LLMCTL_BIN" ]; then BIN="$LLMCTL_BIN"; SRC="env"; else SRC="env-unusable"; fi
elif p="$(command -v llmctl 2>/dev/null)" && [ -n "$p" ]; then BIN="$p"; SRC="path"
elif [ -x "$VENDORED/bin/llmctl" ]; then BIN="$VENDORED/bin/llmctl"; SRC="vendored"
fi

emit() { # state exit detail
  if [ "$JSON" = 1 ]; then
    python3 -I -c 'import json,sys; print(json.dumps({"state":sys.argv[1],"exit":int(sys.argv[2]),"detail":sys.argv[3],"bin":sys.argv[4],"source":sys.argv[5],"url":sys.argv[6]}))' "$1" "$2" "$3" "$BIN" "$SRC" "$URL"
  else
    echo "llmctl_vision_ensure: $1 - $3"
    [ "$1" = READY ] && echo "export LLMCTL_VISION_URL=$URL"
  fi
  exit "$2"
}

health() { # echoes the HTTP status (000 on connection failure)
  curl -s -o /dev/null -m "$TMO" -w '%{http_code}' "$URL/health" 2>/dev/null || true
}

code="$(health)"
if [ "$code" = 200 ]; then emit READY 0 "GET $URL/health -> 200"; fi

# --- not serving: is llmctl even here? ---
if [ -z "$BIN" ]; then
  if [ "$SRC" = env-unusable ]; then d="LLMCTL_BIN is set but is not an executable file"; else
    d="llmctl not found (LLMCTL_BIN, PATH, $VENDORED/bin/llmctl); install: git clone https://github.com/vasic-digital/llmctl && llmctl setup"; fi
  emit ABSENT 3 "$d"
fi
if [ "$SETUP" = 1 ]; then
  "$BIN" setup >&2 || emit UNKNOWN 8 "llmctl setup failed"
fi

plan="$("$BIN" plan --json 2>/dev/null)" || plan=""
verdict="$(printf '%s' "$plan" | python3 -I -c '
import json,sys
try:
    d=json.load(sys.stdin); p=d["profiles"]["vision"]
except Exception as e:
    print("UNKNOWN\tplan unreadable: %s" % type(e).__name__); sys.exit(0)
if not p.get("tier_ok"):
    print("GATED\thost tier %s below min_tier=%s (operator decision to override)" % (d.get("tier"), p.get("min_tier")))
elif not p.get("fits"):
    print("NO_FIT\tneeds %s MiB RAM / %s MiB VRAM, budget RAM %s MiB" % (p.get("ram_mb"), p.get("vram_mb"), d.get("budgets",{}).get("ram_mb")))
else:
    print("RUNNABLE\tfits the current budget")
')"
vstate="${verdict%%$'\t'*}"; vdetail="${verdict#*$'\t'}"
case "$vstate" in
  GATED)   emit GATED 4 "$vdetail; not started" ;;
  NO_FIT)  emit NO_FIT 5 "$vdetail; not started" ;;
  UNKNOWN) emit UNKNOWN 8 "$vdetail" ;;
esac

# RUNNABLE here
if [ "$code" != 000 ] && [ -n "$code" ]; then emit UNHEALTHY 7 "GET $URL/health -> HTTP $code"; fi
if [ "$START" = 1 ]; then
  if "$BIN" start vision >&2; then
    code="$(health)"
    [ "$code" = 200 ] && emit READY 0 "started via llmctl start vision; GET $URL/health -> 200"
    emit UNHEALTHY 7 "llmctl start vision returned 0 but GET $URL/health -> HTTP $code"
  fi
  emit NOT_RUNNING 6 "llmctl start vision failed"
fi
emit NOT_RUNNING 6 "profile fits but nothing listens at $URL; start it with: $BIN start vision (operator decision, or re-run with --start)"
