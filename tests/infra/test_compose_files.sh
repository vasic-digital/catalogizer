#!/usr/bin/env bash
# test_compose_files.sh - T129 (RED first). Oracle for docker-compose.test-infra.yml and the postgres/redis services of docker-compose.build.yml:
# digest-pinned images, `${...}` variables only (no literal credential, no literal host port), per-run credentials from gen_env.sh in a mode 0600 file
# outside version control, one compose project per run, project labels.
# Oracle strategy (11.4.245): SPECIFIED (the lock entries IMG-INFRA-* hold the digests; docs/16 D-06/D-07 define the literal rules) and a control needle:
# a literal credential planted in a scratch copy MUST still be found by the same scanner (a scanner that finds nothing proves nothing).
# Paired mutations: scratch copies with ONE defect (a planted literal password, a tag instead of a digest, a literal host port, `privileged: true`, a
# gen_env.sh that writes mode 0644); the same checks must FAIL for each.
# Usage:  test_compose_files.sh                    the files in the working tree
#         COMPOSE_TEST_INFRA=<file> COMPOSE_TEST_BUILD=<file> test_compose_files.sh     other versions of the two files (RED: the files of git HEAD)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
INFRA="${COMPOSE_TEST_INFRA:-$TI_REPO/docker-compose.test-infra.yml}"
BUILD="${COMPOSE_TEST_BUILD:-$TI_REPO/docker-compose.build.yml}"
GEN="${COMPOSE_TEST_GEN:-$TI_REPO/scripts/test-infra/gen_env.sh}"
LOCK="$TI_REPO/build/containers/images.lock.yaml"

# scan <file> <kind: infra|build>: prints one finding per line; empty output = clean
scan() {
python3 -I - "$1" "$2" "$LOCK" <<'PY'
import re, sys, yaml
path, kind, lockp = sys.argv[1:4]
text = open(path).read()
try: d = yaml.safe_load(text)
except Exception as e: print("yaml_unparsable %s" % e); sys.exit(0)
lock = {i["id"]: i for i in yaml.safe_load(open(lockp))["images"]}
want = {"postgres": "IMG-INFRA-POSTGRES", "redis": "IMG-INFRA-REDIS", "ftp": "IMG-INFRA-FTP", "smb": "IMG-INFRA-SMB", "webdav": "IMG-INFRA-WEBDAV"}
owned = list(want) if kind == "infra" else ["postgres", "redis"]
sv = d.get("services", {}) or {}
VAR = re.compile(r"^\$\{[A-Za-z_][A-Za-z0-9_]*(:?[-?][^}]*)?\}")
SECRETISH = re.compile(r"(^|_)(PASSWORD|PASS|SECRET|TOKEN|KEY|USER(_?NAME)?)$", re.I)   # a credential-bearing variable name (PASSIVE_PORTS and USER_HOME are not)
def parts(v): return str(v).split(";")
SKIPKEYS = {"image", "ports", "volumes", "tmpfs", "networks", "pull_policy", "mem_limit", "cpus", "cap_add", "profiles", "deploy", "restart", "depends_on", "network_mode", "privileged"}
URLCRED = re.compile(r"[A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:([^@\s/]+)@")
SECRETFLAG = re.compile(r"(?:^|[\s'\"])(--requirepass|--password|--passwd|--pass|--auth|--secret|--token|-a)(?:=|\s+)([^\s'\"]+|\"[^\"]*\")")
SECRETASSIGN = re.compile(r"\b([A-Z0-9_]*(?:PASSWORD|PASSWD|SECRET|TOKEN))=([^\s'\";]+)")
def isref(v): v = str(v).strip("\\\"'"); return v.startswith("$") or v == ""
def leaves(o, path=()):
    if isinstance(o, dict):
        for k, v in o.items(): yield from leaves(v, path + (k,))
    elif isinstance(o, (list, tuple)):
        for i, v in enumerate(o): yield from leaves(v, path + (i,))
    elif isinstance(o, str): yield path, o
for name, s in sv.items():
    env = s.get("environment") or {}
    if isinstance(env, list): env = dict(e.split("=", 1) if "=" in e else (e, "") for e in env)
    for k, v in env.items():
        if SECRETISH.search(k) and v not in (None, "") and not str(v).startswith("${") and not all(p.startswith("${") for p in parts(v) if p):
            print("literal_credential %s.%s" % (name, k))
    for hist in ("testpass", "testuser", "minioadmin", "catalogizer_test_pass", "catalogizer_test"):
        if hist in yaml.dump(s): print("historical_literal %s contains %s" % (name, hist))
    # F4 (WF12): a literal credential is found by STRUCTURE in every string of the service (command, entrypoint, healthcheck, URL/DSN values, labels, build args), not only in
    # environment keys with a credential-like name. A value is a reference when it starts with `$` (compose `${...}` or a container variable `$$X`), also after a quote.
    for path, leaf in leaves(s):
        if path and path[0] in SKIPKEYS: continue
        where = "%s.%s" % (name, ".".join(str(x) for x in path))
        for m in URLCRED.finditer(leaf):
            if not isref(m.group(1)): print("literal_url_credential %s" % where)
        for m in SECRETFLAG.finditer(leaf):
            if not isref(m.group(2)): print("literal_credential_argument %s" % where)
        for m in SECRETASSIGN.finditer(leaf):
            if not isref(m.group(2)): print("literal_credential_assignment %s" % where)
    if str(env.get("POSTGRES_HOST_AUTH_METHOD", "")).lower() == "trust": print("auth_disabled %s POSTGRES_HOST_AUTH_METHOD=trust (the server would accept any password)" % name)
    if name not in owned and not (kind == "infra" and name == "minio"): continue   # the build compose's builder and emulator are not T129 services
    for p in (s.get("ports") or []):
        # forms: "[127.0.0.1:]${PORT}:80" or "8080:80"; the host-side port must be a variable, and the bind address loopback
        m = re.match(r"^(?:([0-9.]+):)?(\$\{[^}]+\}|[0-9]+):([0-9]+)(/[a-z]+)?$", str(p))
        if not m: print("port_unparsable %s %s" % (name, p)); continue
        if not m.group(2).startswith("${"): print("literal_host_port %s %s" % (name, p))
        if m.group(1) != "127.0.0.1": print("port_not_loopback %s %s" % (name, p))
    if s.get("privileged"): print("privileged %s" % name)
    if s.get("network_mode") == "host": print("host_network %s" % name)
    if True:
        if name in want:
            img = s.get("image", "")
            m = re.search(r"@(sha256:[0-9a-f]{64})$", img)
            if not m: print("image_not_digest_pinned %s %s" % (name, img))
            elif m.group(1) != lock[want[name]]["digest"]: print("image_digest_not_in_lock %s" % name)
        labels = s.get("labels") or {}
        if isinstance(labels, list): labels = dict(l.split("=", 1) for l in labels)
        if labels.get("project") != "catalogizer": print("label_project_missing %s" % name)
        if "catalogizer.test_project" not in labels: print("label_test_project_missing %s" % name)
        if kind == "infra" and "op_id" not in labels: print("label_op_id_missing %s" % name)
    for c in (s.get("cap_add") or []):
        if not (kind == "infra" and name == "ftp" and c == "AUDIT_WRITE"): print("cap_add %s %s" % (name, c))
    if kind == "infra" and name == "ftp" and "AUDIT_WRITE" not in (s.get("cap_add") or []): print("ftp_audit_write_missing ftp (pure-ftpd never becomes ready without it, evidence/wp12/ftp-capability.txt)")
if kind == "infra":
    for need in want:
        if need not in sv: print("service_missing %s" % need)
    if "nfs" in sv: print("kernel_nfs_service_present nfs")
    if "test-data-seeder" in sv: print("unpinned_seeder_present")
PY
}

# 1. both files exist and parse
for f in "$INFRA" "$BUILD"; do if [ -f "$f" ] && python3 -I -c "import yaml,sys;yaml.safe_load(open(sys.argv[1]))" "$f" 2>/dev/null; then ok "parses: $(basename "$f")"; else bad "missing or unparsable: $f"; fi; done
# 2. no literals, pinned, labelled, rootless
for pair in "$INFRA:infra" "$BUILD:build"; do
  f="${pair%:*}"; k="${pair##*:}"; res="$(scan "$f" "$k")"
  if [ -z "$res" ]; then ok "$(basename "$f"): no literal credential or port, digest-pinned and labelled, rootless"; else bad "$(basename "$f"): $(printf '%s\n' "$res" | grep -c .) finding(s): $(printf '%s' "$res" | head -4 | tr '\n' ';')"; fi
done
# the repository's own pin checker agrees about the test-infra file (it is the T105 gate)
if [ -f "$INFRA" ] && [ "$INFRA" = "$TI_REPO/docker-compose.test-infra.yml" ]; then
  if bash "$TI_REPO/scripts/containers/check_pins.sh" docker-compose.test-infra.yml >"$TI_SCRATCH/pins.txt" 2>&1; then ok "check_pins.sh: docker-compose.test-infra.yml has no unpinned reference"; else bad "check_pins.sh reports: $(head -3 "$TI_SCRATCH/pins.txt" | tr '\n' ';')"; fi
fi
# 3. gen_env.sh: mode 0600, outside version control, defines every variable the files need, differs between runs
if [ -f "$GEN" ]; then
  G1="$TI_SCRATCH/g1"; G2="$TI_SCRATCH/g2"; mkdir -p "$G1" "$G2"
  TI_STATE_DIR="$G1" bash "$GEN" --build-id gen1 >"$TI_SCRATCH/gen1.out" 2>&1; check "gen_env.sh exits 0 (run 1)" "$?" 0
  TI_STATE_DIR="$G2" bash "$GEN" --build-id gen1 >"$TI_SCRATCH/gen2.out" 2>&1; check "gen_env.sh exits 0 (run 2)" "$?" 0
  E1="$G1/catalogizer-test-gen1/env"; E2="$G2/catalogizer-test-gen1/env"
  check "env file mode is 0600" "$(stat -c %a "$E1" 2>/dev/null)" 600
  check "ports file holds no credential variable" "$(grep -cE '^TI_[A-Z_]*(PASSWORD|USER|DB)=' "$G1/catalogizer-test-gen1/ports.env" 2>/dev/null)" 0
  used="$(grep -ho '\${TI_[A-Z][A-Z_]*' "$INFRA" "$BUILD" 2>/dev/null | sed 's/^\${//' | sort -u | grep -v '^TI_MINIO_IMAGE$')"
  miss=""; for v in $used; do grep -q "^$v=" "$E1" 2>/dev/null || miss="$miss $v"; done
  if [ -z "$used" ]; then bad "the compose files reference no TI_* variable at all (everything is literal)"
  elif [ -z "$miss" ]; then ok "gen_env.sh defines every TI_* variable the compose files reference ($(echo $used | wc -w))"; else bad "variables referenced but not generated:$miss"; fi
  if [ "$(grep '^TI_POSTGRES_PASSWORD=' "$E1")" != "$(grep '^TI_POSTGRES_PASSWORD=' "$E2")" ] && [ "$(grep '^TI_PORT_POSTGRES=' "$E1")" != "$(grep '^TI_PORT_POSTGRES=' "$E2")" ]; then ok "credentials and ports differ between two runs (random per run)"; else bad "two runs produced the same credential or port"; fi
  pw="$(sed -n 's/^TI_POSTGRES_PASSWORD=//p' "$E1")"; if [ "${#pw}" -ge 24 ]; then ok "generated password has >= 24 characters"; else bad "generated password too short (${#pw})"; fi
  # outside version control: the real state directory is git-ignored
  if git -C "$TI_REPO" check-ignore -q ".audit/test-infra/catalogizer-test-x/env"; then ok "the default env file location is git-ignored"; else bad "the default env file location is NOT git-ignored"; fi
  # the files render with the generated env (every ${...} substituted), and WITHOUT it podman-compose refuses (required variables)
  rendered="$(cd "$TI_REPO" && podman-compose -f "$INFRA" --env-file "$E1" config 2>"$TI_SCRATCH/cfg.err")"; rc=$?
  if [ "$rc" = 0 ] && ! printf '%s' "$rendered" | grep -q '\${'; then ok "podman-compose config renders docker-compose.test-infra.yml with the generated env, nothing unsubstituted"; else bad "config did not render cleanly rc=$rc: $(tail -2 "$TI_SCRATCH/cfg.err" | tr '\n' ' ')"; fi
  (cd "$TI_REPO" && env -i PATH="$PATH" HOME="$HOME" podman-compose -f "$INFRA" config >/dev/null 2>&1); if [ $? -ne 0 ]; then ok "without the env file the compose file is refused (required variables)"; else bad "the compose file rendered without the per-run env (a variable has a literal default)"; fi
else bad "gen_env.sh absent: $GEN"; fi
# 4. control needle + paired mutations (scan and gen_env mutants)
if [ "${COMPOSE_TEST_NO_MUTATIONS:-0}" != 1 ]; then
  MUTLOG="${COMPOSE_MUTATION_RECORD:-$TI_SCRATCH/mutations.txt}"; : >"$MUTLOG"
  mutant() { # mutant <name> <kind> <python-replace old> <new>   expects the scanner to find at least one problem in the mutated copy of INFRA/BUILD
    local name=$1 kind=$2 old=$3 new=$4 src dst="$TI_SCRATCH/mut-$1.yml" res
    [ "$kind" = infra ] && src="$INFRA" || src="$BUILD"
    python3 -I - "$src" "$dst" "$old" "$new" <<'PY' || { bad "mutation $name: anchor missing in the file"; return; }
import sys
s = open(sys.argv[1]).read()
if s.count(sys.argv[3]) < 1: print("anchor not found: %r" % sys.argv[3]); sys.exit(1)
open(sys.argv[2], "w").write(s.replace(sys.argv[3], sys.argv[4], 1))
PY
    res="$(scan "$dst" "$kind")"
    if [ -n "$res" ]; then ok "mutation $name CAUGHT ($(printf '%s' "$res" | head -1))"; echo "$name CAUGHT" >>"$MUTLOG"; else bad "mutation $name SURVIVED"; echo "$name SURVIVED" >>"$MUTLOG"; fi
  }
  mutant needle_literal_password infra 'POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"' 'POSTGRES_PASSWORD: "hunter2literal"'
  mutant needle_literal_password_build build 'POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"' 'POSTGRES_PASSWORD: "catalogizer_test_pass"'
  # reviewer-authored mutants RM1-RM3, RM5, RM7 of the WF12 review (adopted verbatim): a literal in `command`, in a URL-valued env with a neutral key, in a healthcheck,
  # a server that accepts any password, and the removal of the one capability pure-ftpd needs
  mutant rm1_literal_in_command infra '\"$$REDIS_PASSWORD\"' 'hunter2literal'
  mutant rm2_literal_in_url_env infra '      PASSWORD: "${TI_WEBDAV_PASSWORD:?}"' '      PASSWORD: "${TI_WEBDAV_PASSWORD:?}"
      DAV_DSN: "http://admin:hunter2literal@webdav/"'
  mutant rm3_literal_in_healthcheck infra 'redis-cli ping | grep -qx PONG' 'redis-cli -a hunter2literal ping | grep -qx PONG'
  mutant rm5_trust_auth infra '      POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"' '      POSTGRES_PASSWORD: "${TI_POSTGRES_PASSWORD:?}"
      POSTGRES_HOST_AUTH_METHOD: trust'
  mutant rm7_audit_write_removed infra '    cap_add:
      - AUDIT_WRITE
' ''
  mutant literal_in_label_and_args infra '  project: catalogizer
  op_id: "${TI_OP_ID:?TI_OP_ID is generated by scripts/test-infra/gen_env.sh}"
  catalogizer.op_id' '  project: catalogizer
  note: "login=admin PASSWORD=hunter2literal"
  op_id: "${TI_OP_ID:?TI_OP_ID is generated by scripts/test-infra/gen_env.sh}"
  catalogizer.op_id'
  mutant tag_not_digest infra 'docker.io/library/redis@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499' 'docker.io/library/redis:7-alpine'
  mutant digest_not_in_lock infra 'docker.io/library/redis@sha256:858f009f9709ce576febc734aa78b8f6d624b82571f9ddb6bda4377c833b3499' 'docker.io/library/redis@sha256:0000000000000000000000000000000000000000000000000000000000000000'
  mutant literal_host_port infra '"127.0.0.1:${TI_PORT_REDIS:?}:6379"' '"6379:6379"'
  mutant privileged_service infra '    cap_add:' '    privileged: true
    cap_add:'
  mutant label_project_removed infra '  project: catalogizer
  op_id' '  op_id'
  mutant kernel_nfs_back infra '  # MinIO: BLOCKED.' '  nfs:
    image: docker.io/erichough/nfs-server@sha256:0000000000000000000000000000000000000000000000000000000000000000
    labels: *labels
  # MinIO: BLOCKED.'
  if [ -f "$GEN" ]; then # a gen_env.sh copy that writes mode 0644 must fail the mode check
    cp "$GEN" "$TI_SCRATCH/gen_mode.sh"; cp "$(dirname "$GEN")/lib.sh" "$TI_SCRATCH/lib.sh"
    python3 -I - "$TI_SCRATCH/gen_mode.sh" <<'PY' && {
import sys
s = open(sys.argv[1]).read(); assert s.count("chmod 600 \"$TMP\"") == 1; open(sys.argv[1], "w").write(s.replace("chmod 600 \"$TMP\"", "chmod 644 \"$TMP\""))
PY
      TI_ROOT="$TI_REPO" TI_STATE_DIR="$TI_SCRATCH/g3" bash "$TI_SCRATCH/gen_mode.sh" --build-id gen1 >/dev/null 2>&1
      m="$(stat -c %a "$TI_SCRATCH/g3/catalogizer-test-gen1/env" 2>/dev/null)"
      if [ "$m" != 600 ]; then ok "mutation gen_env_mode644 CAUGHT (mode $m != 600)"; echo "gen_env_mode644 CAUGHT" >>"$MUTLOG"; else bad "mutation gen_env_mode644 SURVIVED"; fi; }
  fi
  [ -z "${COMPOSE_EV:-}" ] || cp "$MUTLOG" "$COMPOSE_EV/compose-mutations.txt"
fi
ti_summary
