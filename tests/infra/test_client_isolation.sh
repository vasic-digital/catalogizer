#!/usr/bin/env bash
# test_client_isolation.sh - WF12 F3, reworked by WF17 round 5 (TI-F1, TI-G4). Oracle for scripts/test-infra/run_client.sh and nas_readonly_leg.sh: a client container must NOT see the repository's `.env`
# (NAS credentials), any project's per-run credential file, a symlinked alias of one, a `.env*` / `*.env` file inside a directory it IS allowed to see, nor anything beyond the view. Real stack (redis), real
# IMG-INFRA-CLIENT container. Oracle strategy (11.4.245): SPECIFIED (what the container sees is what is mounted) and INVARIANT (control needles: the view still holds the client's own lib.sh, an allowed file placed
# next to the decoys IS visible, the project's own credential arrives by --env-file). No file content is ever read or printed: existence and readability only.
# The oracle is EXPOSURE-based, never refusal-based: (1) the mount set of the client container is read back from podman (`inspect .Mounts`): the view (read-only), the out directory and the manifest, nothing else —
# in particular neither the repository root, nor the state directory, nor the env file; (2) the top of /src is compared with the EXPECTED view of the SUT directory (`.audit scripts`: the client directory and the allowed scratch directory the command names), not with a deny-list;
# (3) adversarial words: `..` components, a symlinked scratch directory, `.env` and `creds.env` inside an allowed directory.
# Decoys: a file under .audit/test-infra/<decoy project>/env (0600), one under .audit/scratch/<decoy>/ (the sanctioned scratch root), a symlink .audit/scratch/<alias> -> ../test-infra (an alias of the secret
# store), `.env` and `creds.env` next to an allowed file; the real `.env` is checked when it exists on this host (a decoy is planted when it does not, and removed after).
# Mutants live in a directory UNDER scripts/test-infra/ (never under .audit, so the expected top of /src is the same for the SUT and for a mutant: WF17 N1 identity must SURVIVE).
# Paired mutations: the client sees the repository again; the view copies the repository root's files; the path allow-list removed; both traversal guards removed (IS1); an extra mount of the state directory (IS5);
# the view dir refusals dropped (IS2); `-maxdepth 1` dropped; `.env*` copied; identity (must SURVIVE).
# Usage: test_client_isolation.sh  (ISO_NO_MUTATIONS=1: tests only)   Env: TI_SUT_DIR (repo-relative; a mutant)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
. "$(dirname "${BASH_SOURCE[0]}")/mutlib.sh"; MUT_ENV=ISO; MUT_SELF="${BASH_SOURCE[0]}"
SD="${TI_SUT_DIR:-scripts/test-infra}"
export TI_ROOT="$TI_REPO"; TI_DOWN="$TI_REPO/$SD/down.sh"
for f in up.sh down.sh run_client.sh client/isolation_probe.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
ID=$(ti_new_id); TI_IDS+=("$ID"); P=$(ti_project "$ID")
out=$(bash "$TI_REPO/$SD/up.sh" --build-id "$ID" --services redis 2>&1); rc=$?
check "up exits 0" "$rc" 0
[ "$rc" = 0 ] || { echo "  up said: $(printf '%s' "$out" | tail -3 | tr '\n' ' ' | cut -c1-300)"; ti_summary; exit 1; }
R="zziso$RANDOM"
DEC1="$TI_REPO/.audit/test-infra/catalogizer-test-$R/env"; mkdir -p "$(dirname "$DEC1")"; ( umask 077; echo "TI_REDIS_PASSWORD=decoy-never-a-secret" >"$DEC1" ); TI_FOREIGN_DIRS+=("$TI_REPO/.audit/test-infra/catalogizer-test-$R")
DEC2="$TI_REPO/.audit/scratch/decoy-$R/secret.env"; mkdir -p "$(dirname "$DEC2")"; echo "x=y" >"$DEC2"; TI_FOREIGN_DIRS+=("$TI_REPO/.audit/scratch/decoy-$R")
ln -s ../test-infra "$TI_REPO/.audit/scratch/ti-lnk-$R"; TI_FOREIGN_DIRS+=("$TI_REPO/.audit/scratch/ti-lnk-$R")   # an alias of the secret store inside the sanctioned scratch root
ALLOWED=".audit/scratch/iso-allowed-$R"; mkdir -p "$TI_REPO/$ALLOWED"; TI_FOREIGN_DIRS+=("$TI_REPO/$ALLOWED")
echo ok >"$TI_REPO/$ALLOWED/ok.txt"; echo "ALLOWED_DECOY_NOT_A_SECRET=1" >"$TI_REPO/$ALLOWED/.env"; echo "ALLOWED_DECOY_NOT_A_SECRET=2" >"$TI_REPO/$ALLOWED/creds.env"
if [ ! -f "$TI_REPO/.env" ]; then echo "CONSC_DECOY_NOT_A_SECRET=1" >"$TI_REPO/.env"; DECOY_ENV=1; TI_FOREIGN_DIRS+=("$TI_REPO/.env"); else DECOY_ENV=0; fi
OWNENV=".audit/test-infra/$P/env"
paths=(".audit/test-infra/catalogizer-test-$R/env" ".audit/scratch/decoy-$R/secret.env" "$OWNENV" "docker-compose.test-infra.yml" ".git/config" ".env"
       ".audit/scratch/ti-lnk-$R/catalogizer-test-$R/env" "$ALLOWED/.env" "$ALLOWED/creds.env" "scripts/test-infra/client/.env")
V0=$(ls -d "$TI_REPO"/.audit/scratch/ti-view.* 2>/dev/null | wc -l)
# battery <sut dir> [expected top]: prints one FAIL line per violated expectation; exit status = count
battery() {
  local d=$1 n=0 o rc cid p want_top
  want_top=".audit scripts"   # the view of the SUT: the client script directory, plus the one allowed scratch directory the command names (and nothing else)
  # a long-running probe so the container's mounts can be read back while it runs
  o=$(bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- bash "/src/$d/client/isolation_probe.sh" "/src/$ALLOWED/ok.txt" "${paths[@]}" "$ALLOWED/ok.txt" 2>&1); rc=$?   # the absolute word puts the allowed directory INTO the view; the probe checks the relative paths
  [ "$rc" = 0 ] || { echo "FAIL the probe exited $rc ($(printf '%s' "$o" | tail -1 | cut -c1-100))"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -qx 'NEEDLE present' || { echo "FAIL control needle: the client does not see its own lib.sh (the probe is blind)"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -qx 'ENV TI_REDIS_PASSWORD set' || { echo "FAIL the project's own credential did not reach the client by --env-file"; n=$((n+1)); }
  printf '%s\n' "$o" | grep -qx "PATH $ALLOWED/ok.txt readable" || { echo "FAIL control: an allowed, non-secret file next to the decoys is NOT visible ($(printf '%s\n' "$o" | grep -F "PATH $ALLOWED/ok.txt" | head -1)): the view is empty or the probe blind"; n=$((n+1)); }
  for p in "${paths[@]}"; do printf '%s\n' "$o" | grep -qx "PATH $p absent" || { echo "FAIL /src/$p is visible to the client ($(printf '%s\n' "$o" | grep -F "PATH $p " | head -1))"; n=$((n+1)); }; done
  # the top of /src is the EXPECTED view, exactly: no deny-list that a new leak can slip past
  [ "$(printf '%s\n' "$o" | sed -n 's/^TOP //p' | sed 's/ *$//')" = "$want_top" ] || { echo "FAIL the top of /src is '$(printf '%s\n' "$o" | sed -n 's/^TOP //p')', expected exactly '$want_top'"; n=$((n+1)); }
  # adversarial words are refused by name, and no container runs for them
  for w in "/src/.env" "/src/scripts/test-infra/../../.env" "/src/scripts/test-infra/../../docker-compose.test-infra.yml" "/src/.audit/scratch/ti-lnk-$R/catalogizer-test-$R/env" "/src/$ALLOWED/.env" "/src/$ALLOWED/creds.env"; do
    o=$(bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- cat "$w" 2>&1); rc=$?
    { [ "$rc" -ne 0 ] && printf '%s' "$o" | grep -q 'reason=client_path_not_in_view'; } || { echo "FAIL the word $w was not refused as client_path_not_in_view (rc=$rc)"; n=$((n+1)); }
  done
  # false-refusal control: a component that merely CONTAINS two dots is legal (a..b is not a traversal)
  mkdir -p "$TI_REPO/.audit/scratch/iso-a..b-$R"; echo ok >"$TI_REPO/.audit/scratch/iso-a..b-$R/f.txt"; TI_FOREIGN_DIRS+=("$TI_REPO/.audit/scratch/iso-a..b-$R")
  o=$(bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- test -r "/src/.audit/scratch/iso-a..b-$R/f.txt" 2>&1); rc=$?
  [ "$rc" = 0 ] || { echo "FAIL control: a path component 'a..b' was falsely refused (rc=$rc: $(printf '%s' "$o" | tail -1 | cut -c1-100))"; n=$((n+1)); }
  # the mount set of a live client container, read back from podman: the view (ro), /out, /manifest.sha256 and nothing else
  bash "$TI_REPO/$d/run_client.sh" --build-id "$ID" -- sleep 8 >/dev/null 2>&1 & local cp=$!
  local i; for i in $(seq 1 100); do cid=""; for p in $(podman ps -q --filter "label=catalogizer.test_project=$P"); do case "$(podman inspect --format '{{.Config.Image}}' "$p" 2>/dev/null)" in *infra-client*) cid=$p;; esac; done; [ -n "$cid" ] && break; sleep 0.2; done
  if [ -n "$cid" ]; then
    local mounts; mounts=$(podman inspect --format '{{range .Mounts}}{{.Destination}}={{.Source}};{{end}}' "$cid" 2>/dev/null)
    local dests; dests=$(printf '%s' "$mounts" | tr ';' '\n' | sed 's/=.*//' | sort | tr '\n' ' ')
    case "$dests" in "/manifest.sha256 /out /src "|"/manifest.sha256 /out /src /tmp "|"/out /src "|"/out /src /tmp ") ;; *) echo "FAIL the client's mount destinations are '$dests', expected only /src, /out, /manifest.sha256 (and the /tmp tmpfs)"; n=$((n+1));; esac
    printf '%s' "$mounts" | tr ';' '\n' | grep -E '^/src=' | grep -q "$TI_REPO/.audit/scratch/ti-view\." || { echo "FAIL /src is not a ti-view scratch directory: $(printf '%s' "$mounts" | tr ';' '\n' | grep '^/src=')"; n=$((n+1)); }
    # the corpus manifest is mounted as ONE regular file from the state directory (non-secret hashes; the env file next to it is never mounted): that single file is the only legal source under the state directory
    local msrc; msrc=$(printf '%s' "$mounts" | tr ';' '\n' | sed -n 's|^/manifest.sha256=||p')
    [ -z "$msrc" ] || { [ -f "$msrc" ] && [ "$(basename "$msrc")" = manifest.sha256 ] || { echo "FAIL the manifest mount source is not the regular file manifest.sha256: $msrc"; n=$((n+1)); }; }
    printf '%s' "$mounts" | tr ';' '\n' | grep -v '^/manifest.sha256=' | grep -qE "(^|=)$TI_REPO(/\.audit/test-infra[^;]*)?$|ti-state" && { echo "FAIL a mount source is the repository root or the state directory: $mounts"; n=$((n+1)); }
    [ "$(podman inspect --format '{{range .Mounts}}{{if eq .Destination "/src"}}{{.RW}}{{end}}{{end}}' "$cid")" = false ] || { echo "FAIL /src is mounted read-write"; n=$((n+1)); }
  else echo "FAIL control: no client container was seen while it ran (the mount oracle is blind)"; n=$((n+1)); fi
  wait "$cp" 2>/dev/null
  return "$n"
}
res=$(battery "$SD"); n=$?
if [ "$n" -eq 0 ]; then ok "the client sees only its scripts and an allowed non-secret file: no .env (real or planted), no .env*/*.env inside an allowed dir, no per-run credential file of any project, no symlinked alias, no .git; its own credential arrives by --env-file; the mount set is exactly {view ro, out, manifest}"; else bad "$n violation(s): $(printf '%s' "$res" | tr '\n' ';' | cut -c1-600)"; fi
check "no view directory is left behind by the battery (the count did not grow)" "$(ls -d "$TI_REPO"/.audit/scratch/ti-view.* 2>/dev/null | wc -l)" "$V0"
[ "$DECOY_ENV" = 1 ] && rm -f "$TI_REPO/.env"

if [ "${ISO_NO_MUTATIONS:-0}" != 1 ] && [ "${ISO_TEST_MUTANT:-0}" != 1 ]; then
  MUTLOG="${ISO_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  # a mutant is a COPY of the scripts in a directory UNDER scripts/test-infra/ (so the expected view is the same as the SUT's), battery run against it, directory removed
  mutiso() { # mutiso <name> <caught|survive> <file> <old> <new>...
    local name=$1 want=$2 file=$3 d="scripts/test-infra/zz-mut-iso-$1-$$" r n; shift 3
    rm -rf -- "${TI_REPO:?}/${d:?}"; mkdir -p "$TI_REPO/$d"; TI_FOREIGN_DIRS+=("$TI_REPO/$d"); cp "$TI_REPO/$SD"/*.sh "$TI_REPO/$SD"/*.py "$TI_REPO/$d/"; cp -r "$TI_REPO/$SD/client" "$TI_REPO/$d/client"
    python3 -I - "$TI_REPO/$d" "$file" "$@" <<'PY' || { bad "mutation $name: anchor missing"; return; }
import sys
d, cur = sys.argv[1], sys.argv[2]; a = sys.argv[3:]; texts = {}; i = 0
while i < len(a):
    if a[i] == "@@FILE": cur = a[i + 1]; i += 2; continue   # the pairs that follow edit another file of the mutant copy
    if cur not in texts: texts[cur] = open(d + "/" + cur).read()
    if texts[cur].count(a[i]) != 1: print("anchor count %d for %r in %s" % (texts[cur].count(a[i]), a[i][:80], cur)); sys.exit(1)
    texts[cur] = texts[cur].replace(a[i], a[i + 1]); i += 2
for f, t in texts.items(): open(d + "/" + f, "w").write(t)
PY
    r=$(battery "$d"); n=$?
    if [ "$want" = survive ]; then
      if [ "$n" -eq 0 ]; then ok "identity mutant $name SURVIVED (as required)"; echo "$name SURVIVED-AS-REQUIRED" >>"$MUTLOG"; else bad "identity mutant $name FAILED the battery ($(printf '%s' "$r" | head -1 | cut -c1-120))"; echo "$name FAILED-BUT-IDENTITY" >>"$MUTLOG"; fi
    elif [ "$n" -gt 0 ]; then ok "mutation $name CAUGHT ($(printf '%s' "$r" | head -1 | cut -c1-110))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
    rm -rf -- "${TI_REPO:?}/${d:?}"; }
  [ -f "$TI_REPO/.env" ] || { echo "CONSC_DECOY_NOT_A_SECRET=1" >"$TI_REPO/.env"; DECOY_ENV=1; }
  mutiso client_sees_repository caught run_client.sh '(cd "$VIEW" && TI_RUNP_PRINT=1' '(cd "$TI_ROOT" && TI_RUNP_PRINT=1'
  mutiso view_copies_repo_root_files caught run_client.sh 'ti_view_dir "$VIEW" "${VDIRS[@]}" ||' 'ti_view_dir "$VIEW" "${VDIRS[@]}" . ||'
  mutiso allow_list_removed caught run_client.sh '      ti_path_in_view "$rel" || ti_refuse client_path_not_in_view' '      true || ti_refuse client_path_not_in_view'
  # IS1 (WF17 reviewer): the `..` component guard AND the canonical-path equality (the two layers that reject a traversal) removed: `/src/scripts/test-infra/../../docker-compose.test-infra.yml` is then served
  mutiso is1_traversal_layers_removed caught lib.sh '  case "/$d/" in */./*|*//*|*/../*) return 1;; esac' '  :' '  [ "$rp" = "$(realpath -m -- "$TI_ROOT")/$d" ]' '  true'
  # IS2 (WF17 reviewer): the canonical-path equality (the only layer that rejects a SYMLINKED component) removed: the alias of the secret store is served
  mutiso is2_symlink_layer_removed caught lib.sh '  [ "$rp" = "$(realpath -m -- "$TI_ROOT")/$d" ]' '  true'
  # IS5 (WF17 reviewer): run_client also mounts the state directory read-only at /ti-state
  mutiso is5_extra_state_mount caught run_client.sh '    NEW+=(--network "$NET" --env-file "$ENVF"' '    NEW+=(-v "$TI_STATE_DIR:/ti-state:ro" --network "$NET" --env-file "$ENVF"'
  # the `.env*` / `*.env` exclusion removed at BOTH places (the word check and the copy filter): an env file inside an allowed directory is then served
  mutiso env_files_served caught run_client.sh '      case "$rel" in */.env*|*.env|.env*) ti_refuse client_path_not_in_view "$a names an env file: a client never sees one";; esac' '      :' @@FILE lib.sh "find \"\$rp\" -maxdepth 1 -type f ! -name '.env*' ! -name '*.env' -exec cp -p -t \"\$dest/\$d\" {} +" "find \"\$rp\" -maxdepth 1 -type f -exec cp -p -t \"\$dest/\$d\" {} +" "[ -z \"\$(find \"\$rp\" -maxdepth 1 -type f ! -name '.env*' ! -name '*.env' -print -quit)\" ]" "false"
  # the copy recurses (no -maxdepth 1): hidden subtrees come along
  mutiso view_recurses caught lib.sh "find \"\$rp\" -maxdepth 1 -type f ! -name '.env*' ! -name '*.env' -exec cp -p -t \"\$dest/\$d\" {} +" "cp -a \"\$rp/.\" \"\$dest/\$d/\""
  # N1 / IS0b identity: a harmless change must keep the whole battery green
  mutiso n1_identity survive run_client.sh 'ARGV=()' 'ARGV=(); :'
  [ "$DECOY_ENV" = 1 ] && rm -f "$TI_REPO/.env"
  [ -z "${ISO_EV:-}" ] || cp "$MUTLOG" "$ISO_EV/client-isolation-mutations.txt"
fi
ti_summary
