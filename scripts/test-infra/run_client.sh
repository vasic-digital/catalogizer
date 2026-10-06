#!/usr/bin/env bash
# run_client.sh - T128. Runs ONE protocol-client command inside the interpreter-class image IMG-INFRA-CLIENT, attached to the compose network of a
# test-infrastructure project, through scripts/containers/run_pinned.sh (rootless, digest-pinned, resource-limited, disk gate first).
# Never a host client and never an `exec` into a service container: the image must be the lock entry IMG-INFRA-CLIENT and its class must be
# `interpreter` (T121b); a service-class image or any other id is refused.
# Usage:  run_client.sh --build-id <id> [--out DIR] [--image IMG-ID] -- <command word>...
#   --build-id ID   the project catalogizer-test-<id> whose network the client joins and whose per-run env file (credentials, 0600) is handed over
#   --out DIR       output directory mounted at /out (default <repo>/.audit/out/<project>-client); must satisfy run_pinned's sanctioned roots
#   --image ID      the image id (default IMG-INFRA-CLIENT; the option exists so the refusal of another image is testable)
# The container gets the env file through `--env-file` (never argv), the label op_id of the project's registered long operation (so the anti-mess
# sweep matches the container to its operation), the repository read-only at /src (client scripts live in scripts/test-infra/client/) and the seeded
# corpus manifest at /manifest.sha256 (read-only) when it exists.
# Exit: the client's exit code; 1 REFUSED (`test-infra: REFUSED reason=<code>`); 2 usage.
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$HERE/lib.sh"
BID=""; OUT=""; IMG="IMG-INFRA-CLIENT"
while [ $# -gt 0 ]; do
  case "$1" in
    --build-id) BID=${2:-}; shift 2;; --out) OUT=${2:-}; shift 2;; --image) IMG=${2:-}; shift 2;;
    --) shift; break;; *) ti_die "unknown argument '$1'" 2;;
  esac
done
ti_valid_id "$BID" || ti_die "--build-id must match ^[a-z0-9][a-z0-9-]{0,30}\$" 2
[ $# -ge 1 ] || ti_die "command missing after --" 2
ti_need python3 podman
CLASS="$(python3 -I - "$TI_LOCK" "$IMG" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1])); m = [i for i in d.get("images", []) if i.get("id") == sys.argv[2]]
print(("ERR:unknown" if not m else "ERR:duplicate" if len(m) > 1 else m[0].get("class", "")))
PY
)" || ti_refuse lock_unreadable "$TI_LOCK"
case "$CLASS" in ERR:unknown) ti_refuse image_id_unknown "$IMG";; ERR:duplicate) ti_refuse lock_duplicate_id "$IMG";; esac
[ "$CLASS" = interpreter ] || ti_refuse client_image_not_interpreter_class "$IMG has class '$CLASS'; a probe never runs in a service-class image"
[ "$IMG" = IMG-INFRA-CLIENT ] || ti_refuse not_the_infra_client "every probe client runs in IMG-INFRA-CLIENT, got $IMG (class '$CLASS')"
P="$(ti_project "$BID")"; S="$(ti_state "$BID")"; ENVF="$S/env"
[ -r "$ENVF" ] || ti_refuse project_not_up "no env file for $P ($ENVF); start it with up.sh"
NET="$(ti_network "$BID")"
podman network exists "$NET" 2>/dev/null || ti_refuse network_absent "$NET"
OPID="$(ti_env_get "$ENVF" TI_OP_ID)"
[ -n "$OUT" ] || OUT="$TI_ROOT/.audit/out/$P-client"
mkdir -p "$OUT" || ti_die "cannot create $OUT"
ARGV=()
while IFS= read -r line; do ARGV+=("$line"); done < <(cd "$TI_ROOT" && RUNP_PRINT_ARGV=1 bash scripts/containers/run_pinned.sh --out "$OUT" --op-id "$P-client-$$-$RANDOM" "$IMG" -- "$@")
[ "${#ARGV[@]}" -gt 5 ] && [ "${ARGV[0]}" = podman ] || ti_refuse run_pinned_failed "run_pinned.sh printed no podman argv (see its message above)"
# insert the project's network, env file, op label and the manifest mount before the `--` that precedes the image
NEW=(); done_ins=0
for a in "${ARGV[@]}"; do
  if [ "$done_ins" = 0 ] && [ "$a" = "--" ]; then
    NEW+=(--network "$NET" --env-file "$ENVF" --label "op_id=$OPID" --label "catalogizer.test_project=$P")
    [ ! -r "$S/manifest.sha256" ] || NEW+=(-v "$S/manifest.sha256:/manifest.sha256:ro")
    done_ins=1
  fi
  NEW+=("$a")
done
[ "$done_ins" = 1 ] || ti_refuse argv_malformed "no -- separator before the image in the composed argv"
cd "$TI_ROOT" && exec "${NEW[@]}"
