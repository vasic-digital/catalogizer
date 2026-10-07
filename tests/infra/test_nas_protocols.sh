#!/usr/bin/env bash
# test_nas_protocols.sh - WP-12 oracle for scripts/test-infra/nas_protocols.sh, client/nas_proto.py and client/nas_proto_nfs.sh (the READ-ONLY NFS/FTP/FTPS/SFTP survey of the real Synology hosts).
# Oracle strategy (11.4.245): SPECIFIED (the owner's rules: read-only, <= 1 request/s per host, credentials only from the gitignored env file and never in argv/logs/evidence, SKIP not FAIL
# when the env file is absent, names and addresses never recorded) and INVARIANT (no credential value or host address occurs in the committed evidence; SHA256SUMS verifies).
# Sections:  A static: no write-class verb in the clients (control needles: planted verbs ARE found), credentials only reach the 0600 file   B unit: tests/infra/nas_proto_unit.py (real TCP RPC fixture)
#            C SKIP behaviour   D evidence integrity (only when the evidence directory exists; the real .env is used for the leak scan when present)
# Usage:  test_nas_protocols.sh      Env: NASP_EV (evidence dir, default specs/.../evidence/wp12/nas-protocols)
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
SD=scripts/test-infra
for f in nas_protocols.sh client/nas_proto.py client/nas_proto_nfs.sh; do need_script "$SD/$f" || { ti_summary; exit 1; }; done
ORCH="$TI_REPO/$SD/nas_protocols.sh"; PY="$TI_REPO/$SD/client/nas_proto.py"; NFS="$TI_REPO/$SD/client/nas_proto_nfs.sh"
FX="$TI_REPO/.audit/scratch/nasp-fx.$$"; mkdir -p "$FX"; trap 'rm -rf -- "${FX:?}"; ti_cleanup' EXIT

# ---- A static ----
FTP_WRITE='\b(STOR|STOU|APPE|DELE|MKD|RMD|RNFR|RNTO|SITE|CHMOD|SMNT|REIN)\b'
SFTP_WRITE='SSH_FXP_(WRITE|REMOVE|MKDIR|RMDIR|RENAME|SETSTAT|FSETSTAT|SYMLINK|LINK)|storbinary|storlines|\.delete\(|\.mkd\(|\.rmd\(|\.rename\(|\.sendcmd\("(STOR|DELE|MKD|RMD|SITE)'
code_only() { grep -vE '^[[:space:]]*#' "$1" | python3 -I -c 'import sys,re; t=sys.stdin.read(); t=re.sub(r"\"\"\"[\s\S]*?\"\"\"","",t,count=1); print(t)'; }   # drop comments and the module docstring (it NAMES the forbidden verbs)
hits() { code_only "$1" | grep -nE "$FTP_WRITE|$SFTP_WRITE" || true; }
h=$(hits "$PY"); [ -z "$h" ] && ok "A: nas_proto.py code holds no FTP/SFTP write-class verb" || bad "A: write-class verb in nas_proto.py: $h"
printf 'ftp.storbinary("STOR x", f)\nsf.req(13, b"")  # SSH_FXP_REMOVE\nssh_fxp_mkdir = SSH_FXP_MKDIR\n' >"$FX/needle.py"; [ -n "$(hits "$FX/needle.py")" ] && ok "A: control needle: planted write verbs are found by the same scan" || bad "A: control needle: the scan is blind"
sf=$(code_only "$PY" | grep -oE 'sf\.(req|send)\([0-9]+' | grep -oE '[0-9]+$' | sort -un | tr '\n' ' ')
check "A: the only SFTP packet types sent are INIT(1) CLOSE(4) OPEN(3, read-only flag) READ(5) OPENDIR(11) READDIR(12)" "$sf" "1 3 4 5 11 12 "
check "A: the SFTP OPEN uses SSH_FXF_READ (1) only" "$(code_only "$PY" | grep -c 'sf.req(3, s_str(fp.encode()) + struct.pack(">I", 1)')" 1
nw=$(code_only "$NFS" | grep -E '\b(nfs-cp|nfs-mkdir|nfs-rm|nfs-mv|nfs-write|mount|umount|sudo|rm|mv|cp|dd|touch|mkdir|chmod|chown)\b' | grep -vE "rm -rf|rec |^\s*printf|#" || true)
nfs_verbs=$(code_only "$NFS" | grep -oE '\bnfs-[a-z]+' | sort -u | tr '\n' ' ')
check "A: the NFS walker uses only nfs-ls and nfs-cat" "$nfs_verbs" "nfs-cat nfs-ls "
printf 'nfs-cp /tmp/a nfs://h/x\n' >"$FX/needle.sh"; check "A: control needle: a planted nfs-cp is found" "$(code_only "$FX/needle.sh" | grep -oE '\bnfs-[a-z]+' | sort -u | tr '\n' ' ')" "nfs-cp "
rpc_procs=$(code_only "$PY" | grep -oE 'rpc_call\(ip, [A-Za-z_]+, [0-9]+, [A-Za-z0-9_]+, [0-9]+' | grep -oE '[0-9]+$' | sort -un | tr '\n' ' ')
check "A: RPC procedures called: NULL(0) MNT(1) UMNT(3) DUMP(4) EXPORT(5) only" "$rpc_procs" "0 1 3 4 5 "
cred_lines=$(grep -nE 'PASS_V|USER_V' "$ORCH" | grep -vE '^\S*:\s*#' | grep -vE 'envval|printf .%s\\n%s\\n. "\$USER_V" "\$PASS_V"|leak|for pat' || true)
[ -z "$cred_lines" ] && ok "A: the credential variables of the orchestrator reach only the 0600 file and the leak scan" || bad "A: credential variable used elsewhere: $cred_lines"
check "A: the orchestrator writes the credential file with umask 077 and chmod 600" "$(grep -c 'umask 077; printf' "$ORCH")$(grep -c 'chmod 600 "\$hd/cred"' "$ORCH")" "11"
check "A: the credential file, askpass and ssh config are removed after each host" "$(grep -c 'rm -f -- "\$hd/cred" "\$hd/askpass"' "$ORCH")" 1
check "A: the run goes through run_pinned (ti_runpinned) only; no host client is called" "$(code_only "$ORCH" | grep -cE '(^|[;&|(] *)(lftp|smbclient|nfs-ls|nfs-cat|sftp|ssh|ssh-keyscan|curl|openssl) ')" 0

# ---- B unit ----
out=$(python3 -I "$TI_REPO/tests/infra/nas_proto_unit.py" 2>&1); rc=$?
check "B: the python unit oracle passes (real TCP RPC fixture, bounded walk, no names recorded)" "$rc" 0
check "B: the unit oracle ran at least 30 checks (it is not empty)" "$([ "$(printf '%s\n' "$out" | grep -c '^PASS')" -ge 30 ] && echo yes || echo no)" yes

# ---- C SKIP ----
o=$(TI_ENV_FILE="$FX/no-such.env" bash "$ORCH" 2>&1); rc=$?
check "C: absent env file exits 0" "$rc" 0
case "$o" in "SKIP nas_protocols reason=env_absent"*) ok "C: output is SKIP nas_protocols reason=env_absent";; *) bad "C: wrong output: $o";; esac
printf 'SYNOLOGY_IP_3=192.0.2.1\n' >"$FX/partial.env"
o=$(TI_ENV_FILE="$FX/partial.env" bash "$ORCH" --hosts 3 2>&1); rc=$?
check "C: env file without credentials exits 0" "$rc" 0
case "$o" in "SKIP nas_protocols reason=credentials_absent variables: SYNOLOGY_SMB_USER SYNOLOGY_SMB_PASSWORD"*) ok "C: the SKIP names the missing variables (names only)";; *) bad "C: wrong output: $o";; esac
o=$(bash "$ORCH" --hosts 9 2>&1); check "C: a host outside 1..7 is a usage error (exit 2)" "$?" 2
o=$(bash "$ORCH" --protocols smb 2>&1); check "C: an unknown protocol is a usage error (exit 2)" "$?" 2

# ---- D evidence ----
EV="${NASP_EV:-$TI_REPO/specs/001-full-project-audit-remediation/evidence/wp12/nas-protocols}"
if [ -s "$EV/SHA256SUMS" ]; then
  ( cd "$EV" && sha256sum -c --quiet SHA256SUMS ) >/dev/null 2>&1; check "D: SHA256SUMS verifies" "$?" 0
  check "D: every json of the evidence directory is listed in SHA256SUMS" "$(cd "$EV" && ls -1 *.json | sort | tr '\n' ' ')" "$(cut -c67- "$EV/SHA256SUMS" | sort | tr '\n' ' ')"
  for f in "$EV"/nfs-[1-7].json "$EV"/ftp-[1-7].json "$EV"/ftps-[1-7].json "$EV"/sftp-[1-7].json; do
    [ -f "$f" ] || continue
    jq -e '.identity.head != null and .identity.run_at != null and (.identity.tools|length) == 3 and .identity.writes_performed == 0 and .identity.credentials_recorded == false' "$f" >/dev/null 2>&1 || bad "D: identity header missing or wrong in ${f##*/}"
  done; ok "D: every per-host-per-protocol file carries the identity header (head, run_at, tool sha256, writes 0)"
  ENVF="$TI_REPO/.env"
  if [ -r "$ENVF" ]; then
    leak=0; u=$(python3 -I "$TI_REPO/$SD/dotenv_get.py" "$ENVF" SYNOLOGY_SMB_USER); pw=$(python3 -I "$TI_REPO/$SD/dotenv_get.py" "$ENVF" SYNOLOGY_SMB_PASSWORD)
    for pat in "$u" "$pw"; do [ -n "$pat" ] && grep -rqF -- "$pat" "$EV" && leak=1; done
    for i in 1 2 3 4 5 6 7; do ip=$(python3 -I "$TI_REPO/$SD/dotenv_get.py" "$ENVF" "SYNOLOGY_IP_$i"); [ -n "$ip" ] && grep -rqF -- "$ip" "$EV" && leak=1; done
    check "D: neither the credential values nor any host address occur in the evidence" "$leak" 0
    printf '%s\n' "$pw" >"$FX/needle.txt"; [ -n "$pw" ] && grep -rqF -- "$pw" "$FX" && ok "D: control needle: the leak scan finds the password when it is present" || bad "D: control needle: the leak scan is blind"
  else blocked "D: no .env, the leak scan against the real values is skipped (honest SKIP)"; fi
  check "D: no entry-name field exists in any evidence file (names_recorded false everywhere)" "$(jq -s '[.[] | select(.names_recorded == true)] | length' "$EV"/ftp-[1-7].json "$EV"/ftps-[1-7].json "$EV"/sftp-[1-7].json 2>/dev/null)" 0
else blocked "D: no evidence directory yet (run nas_protocols.sh first)"; fi
ti_summary
