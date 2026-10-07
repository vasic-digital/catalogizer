#!/usr/bin/env bash
# test_containerfiles.sh - T106 (the build/containers/ tree, docs/16 DR-16-1). Static checks over every image directory:
#   C1 the tree holds every required image directory (go gotools node playwright docs rust android mut sigverify infra-client infra-postgres
#      infra-redis infra-ftp infra-smb infra-webdav; infra-minio is absent: no MinIO server image is obtainable from any official registry, T106 blocked item; plus the two T006 directories kcov and testutil);
#   C2 each directory has a non-empty Containerfile, README.md and digests.lock;
#   C3 the Containerfile passes scripts/containers/check_pins.sh (digest-pinned FROM, no pipe-to-shell; T105);
#   C4 every download line (curl or wget in a RUN) is followed, in the same RUN, by a SHA-256 check (sha256sum -c or shasum -a 256 -c), and a
#      RUN holds at least as many checks as downloads; an `ADD <url>` carries `--checksum=sha256:<64 hex>`;
#   C5 digests.lock names every FROM digest and every SHA-256 literal the Containerfile checks (the lock and the file cannot drift);
#   C7 no RUN swallows a failure (`|| true`, `|| :`): a step that may fail is handled explicitly or not run (WF10 p1 F6, the android licence swallow);
#   C6 the directory has a matching images.lock.yaml entry (rust and android are the two images whose entry T143 and T144 write), the
#      entry carries a `class` from {compile, interpreter, service, runtime, runtime-base} for the entries T106 writes, and when the
#      entry's reference is one of the Containerfile's FROM references the two digests are equal.
# The checker is the python block below, run on the real tree AND on deliberate-violation fixtures written to a temporary directory at run
# time (golden-bad: each rule must fire; golden-good: a compliant directory passes; negative control: a compliant directory next to a bad
# one is not blamed). Paired mutations: copies of THIS file with one rule weakened must FAIL on the fixtures (the real tree stays clean under
# them, so the fixtures are what catches a weakened checker). Mutation record: $CF_MUTATION_RECORD (default: scratch).
# Run: on the host (control plane) or through `scripts/containers/run_pinned.sh IMG-KCOV -- bash scripts/containers/tests/test_containerfiles.sh`.
# Env: CF_TEST_NO_MUTATIONS=1 skips the mutations; CF_TEST_MUTANT=1 marks a mutant run; CF_TEST_ROOT overrides the repository root (tests).
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="${CF_TEST_ROOT:-$(cd "$HERE/../../.." && pwd)}"
SELF="${CF_SELF:-${BASH_SOURCE[0]}}"
FAILS=0; PASSES=0
ok()  { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
command -v python3 >/dev/null 2>&1 || { echo "FAIL: python3 is required"; exit 2; }
T="$(mktemp -d "${TMPDIR:-/tmp}/cf-test.XXXXXX")" && [ -d "$T" ] || { echo "FAIL: cannot create the scratch directory under ${TMPDIR:-/tmp} (mktemp failed); nothing was run"; exit 2; }; trap 'rm -rf "$T"' EXIT

# the checker: prints one line per violation `<rule> <dir>: <message>`; exit 0 whatever it prints (the caller counts lines)
CHECKER="$T/checker.py"
cat >"$CHECKER" <<'PY'
import os, re, subprocess, sys
repo, tree, lockfile, pins = sys.argv[1:5]
required = sys.argv[5].split(",") if len(sys.argv) > 5 and sys.argv[5] else []
ID = {"go": "IMG-GO", "gotools": "IMG-GOTOOLS", "node": "IMG-NODE", "playwright": "IMG-PW", "docs": "IMG-DOCS", "rust": "IMG-RUST",
      "android": "IMG-ANDROID", "mut": "IMG-MUT", "sigverify": "IMG-SIGVERIFY", "infra-client": "IMG-INFRA-CLIENT",
      "infra-postgres": "IMG-INFRA-POSTGRES", "infra-redis": "IMG-INFRA-REDIS", "infra-minio": "IMG-INFRA-MINIO", "infra-ftp": "IMG-INFRA-FTP",
      "infra-smb": "IMG-INFRA-SMB", "infra-webdav": "IMG-INFRA-WEBDAV", "kcov": "IMG-KCOV", "testutil": "IMG-TESTUTIL"}
DEFERRED = {"rust", "android"}          # entries written by T143 and T144
NO_CLASS_CHECK = {"kcov", "testutil", "go"}   # entries written before the class field existed (T006 and T005d; T121b backfills them)
CLASSES = {"compile", "interpreter", "service", "runtime", "runtime-base"}
DIGEST = re.compile(r"@sha256:([0-9a-f]{64})(?![0-9A-Za-z])")
out = []
def v(rule, d, msg): out.append("%s %s: %s" % (rule, d, msg))

import yaml
entries = {}
if os.path.exists(lockfile):
    for e in (yaml.safe_load(open(lockfile)) or {}).get("images", []):
        entries.setdefault(e.get("id"), []).append(e)

for r in required:                                                   # C1
    if not os.path.isdir(os.path.join(tree, r)):
        v("C1-missing-directory", r, "required image directory is absent")

def logical(text):
    acc, cur = [], ""
    for raw in text.split("\n"):
        s = raw.rstrip()
        if not cur and s.lstrip().startswith("#"):
            continue
        if s.endswith("\\"):
            cur += " " + s[:-1].strip()
        else:
            cur += " " + s.strip(); acc.append(cur.strip()); cur = ""
    if cur: acc.append(cur.strip())
    return acc

import shlex
SEPS = {"&&", "||", ";", "|", "&", "(", ")", "{", "}", ";;", "|&", ";&"}
WRAPS = {"sudo", "env", "nohup", "time", "command", "exec", "then", "do", "else", "!", "xargs", "nice", "timeout"}
def run_commands(body):
    """Split the shell text of one RUN into simple commands: [(separator-before, [tokens...]), ...]. Raises ValueError on an unparseable text."""
    lex = shlex.shlex(body, posix=True, punctuation_chars=True)
    lex.whitespace_split = True
    out, cur, sep = [], [], ""
    for tok in lex:
        if tok in SEPS:
            if cur: out.append((sep, cur))
            cur, sep = [], tok
        else:
            cur.append(tok)
    if cur: out.append((sep, cur))
    return out
def head_of(toks):
    """(index of the command word, base name); lookups (`command -v X`) and assignments are not the command."""
    i = 0
    while i < len(toks):
        t = toks[i]
        if re.match(r"^[A-Za-z_]\w*=", t): i += 1; continue
        b = os.path.basename(t)
        if b in WRAPS:
            if b == "command" and i + 1 < len(toks) and toks[i + 1] in ("-v", "-V"): return None, ""
            i += 1
            while i < len(toks) and toks[i].startswith("-") and toks[i] not in ("-",): i += 1
            continue
        return i, b
    return None, ""
def is_download(toks):
    i, b = head_of(toks)
    if i is None or b not in ("curl", "wget"): return False
    return not set(toks[i + 1:]) <= {"--version", "-V", "--help", "-h"}
def is_check(toks):
    i, b = head_of(toks)
    if i is None: return False
    a = toks[i + 1:]
    if b == "sha256sum": return any(x in ("-c", "--check") or (x.startswith("-") and not x.startswith("--") and "c" in x[1:]) for x in a)
    if b == "shasum": return "256" in a and "-a" in a and "-c" in a
    return False
def swallows(cmds_after_oror):
    """True when the right-hand side of an `||` only ever succeeds (true, :, echo, printf, exit 0): the failure of the left side is dropped."""
    if not cmds_after_oror: return False
    for toks in cmds_after_oror:
        i, b = head_of(toks)
        if i is None: return False
        a = toks[i + 1:]
        if b in ("true", ":", "echo", "printf"): continue
        if b == "exit" and a == ["0"]: continue
        return False
    return True
def run_violations(body):
    """(download_without_check, swallowed_failure) of one RUN body. A download must be followed, LATER in the same RUN, by its own SHA-256
    check (each check consumes one earlier download); a failure is swallowed by `|| <command that only succeeds>`, a `{ }`/`( )` group of
    such commands, or `set +e`."""
    pending, swallow = 0, False
    cmds = run_commands(body)
    for n, (sep, toks) in enumerate(cmds):
        if is_download(toks): pending += 1
        elif is_check(toks) and pending > 0: pending -= 1
        i, b = head_of(toks)
        if b == "set" and "+e" in toks[i + 1:]: swallow = True
        if sep == "||":                                   # the right-hand side, extended by the `&&` commands chained to it (`|| echo x && exit 1` fails)
            rhs, k = [toks], n + 1
            while k < len(cmds) and cmds[k][0] == "&&": rhs.append(cmds[k][1]); k += 1
            if swallows(rhs): swallow = True
    return pending > 0, swallow or group_rhs_swallows(body)
def group_rhs_swallows(body):
    """Right-hand sides of `||` that are groups ({ a; b; } or ( a; b )): scanned on the token stream, because the group spans several commands."""
    lex = shlex.shlex(body, posix=True, punctuation_chars=True); lex.whitespace_split = True
    toks = list(lex)
    res = False
    for i, t in enumerate(toks):
        if t == "||" and i + 1 < len(toks) and toks[i + 1] in ("{", "("):
            close = "}" if toks[i + 1] == "{" else ")"
            j, depth, grp = i + 2, 1, []
            while j < len(toks) and depth:
                if toks[j] in ("{", "("): depth += 1
                elif toks[j] in ("}", ")"):
                    depth -= 1
                    if not depth: break
                grp.append(toks[j]); j += 1
            sub, cur = [], []
            for g in grp:
                if g in SEPS:
                    if cur: sub.append(cur)
                    cur = []
                else: cur.append(g)
            if cur: sub.append(cur)
            if swallows(sub): res = True
    return res

for d in sorted(x for x in os.listdir(tree) if os.path.isdir(os.path.join(tree, x))):
    p = os.path.join(tree, d)
    cf = os.path.join(p, "Containerfile")
    if not os.path.isfile(cf) or os.path.getsize(cf) == 0:          # C2
        v("C2-containerfile", d, "Containerfile missing or empty"); continue
    for f in ("README.md", "digests.lock"):
        fp = os.path.join(p, f)
        if not os.path.isfile(fp) or os.path.getsize(fp) == 0:
            v("C2-" + f, d, f + " missing or empty")
    text = open(cf).read()
    r = subprocess.run(["bash", pins, "--root", tree, d], capture_output=True, text=True)   # C3
    vl = [x for x in r.stdout.splitlines() if x.startswith("VIOLATION")]
    if r.returncode == 0 and not vl:      # MUT-ANCHOR c3-clean
        pass
    elif r.returncode == 1 and vl:
        for line in vl: v("C3-check_pins", d, line)
    else:                                 # a crash (3), a usage error (2), a missing script (127) or an inconsistent verdict is never "clean"
        v("C3-check_pins", d, "check_pins exited %d with %d VIOLATION lines (not a clean verdict)" % (r.returncode, len(vl)))   # MUT-ANCHOR c3-else
    shas = []
    for ln in logical(text):                                         # C4, C7
        if re.match(r"^RUN\b", ln, re.I):
            body = re.sub(r"^RUN\s+(?:--\S+\s+)*", "", ln, flags=re.I)
            if re.search(r"\b(?:curl|wget)\b|\|\||set\s+\+e", body):
                try:
                    dl_bad, swallowed = run_violations(body)       # MUT-ANCHOR c4-check
                except ValueError:                                 # unparseable shell text that names curl/wget: refused, not guessed
                    dl_bad, swallowed = bool(re.search(r"\b(?:curl|wget)\b", body)), False
                if dl_bad: v("C4-download-without-sha256", d, ln[:140])     # MUT-ANCHOR c4-count
                if swallowed: v("C7-swallowed-failure", d, ln[:140])        # C7  MUT-ANCHOR c7-swallow
            if re.search(r"\b(?:curl|wget)\b", body): shas += re.findall(r"\b([0-9a-f]{64})\b", ln)
        if re.match(r"^ADD\b", ln, re.I) and re.search(r"\bhttps?://", ln):
            ck = re.search(r"--checksum=sha256:([0-9a-f]{64})\b", ln)    # MUT-ANCHOR c4-add
            if not ck: v("C4-download-without-sha256", d, ln[:140])
            else: shas.append(ck.group(1))
    froms = []
    args = {}
    for ln in logical(text):
        am = re.match(r"^ARG\s+(\w+)=(\S+)", ln, re.I)
        if am: args[am.group(1)] = am.group(2)
        fm = re.match(r"^FROM\s+(?:--\S+\s+)*(\S+)", ln, re.I)
        if fm:
            ref = fm.group(1)
            ref = re.sub(r"^\$\{?(\w+)\}?$", lambda mm: args.get(mm.group(1), ref), ref)
            if DIGEST.search(ref): froms.append(ref)
    if not froms and not any(re.match(r"^FROM\s+\$", ln, re.I) for ln in logical(text)):
        v("C2-from", d, "no FROM line")
    lockp = os.path.join(p, "digests.lock")
    locktxt = open(lockp).read() if os.path.isfile(lockp) else ""
    for ref in froms:                                                # C5
        if DIGEST.search(ref).group(1) not in locktxt:               # MUT-ANCHOR c5-from
            v("C5-digests.lock-from", d, "FROM digest of %s not in digests.lock" % ref.split("@")[0])
    for h in sorted(set(shas)):
        if h not in locktxt: v("C5-digests.lock-sha256", d, "sha256 %s... not in digests.lock" % h[:12])
    if d in ID:                                                      # C6
        es = entries.get(ID[d], [])
        if not es:
            if d not in DEFERRED: v("C6-lock-entry", d, "no images.lock.yaml entry %s" % ID[d])
        else:
            e = es[0]
            if d not in NO_CLASS_CHECK and e.get("class") not in CLASSES:     # MUT-ANCHOR c6-class
                v("C6-class", d, "entry %s class %r not in %s" % (ID[d], e.get("class"), sorted(CLASSES)))
            for ref in froms:
                base, dg = ref.split("@")
                if e.get("reference") == base and e.get("digest") != dg:
                    v("C6-digest-mismatch", d, "Containerfile FROM %s@%s but lock entry digest is %s" % (base, dg[:19], str(e.get("digest"))[:19]))
    else:
        v("C6-unknown-directory", d, "directory has no id mapping in the checker")
print("\n".join(out))
PY

# run_checker <tree> <lock> <required-list> -> OUT (lines)
run_checker() { OUT="$(python3 -I "$CHECKER" "$REPO" "$1" "$2" "$REPO/scripts/containers/check_pins.sh" "$3" 2>&1)"; }

# ---------------------------------------------------------------- the real tree
REQ="go,gotools,node,playwright,docs,rust,android,mut,sigverify,infra-client,infra-postgres,infra-redis,infra-ftp,infra-smb,infra-webdav,kcov,testutil"
if [ -d "$REPO/build/containers" ]; then run_checker "$REPO/build/containers" "$REPO/build/containers/images.lock.yaml" "$REQ"; else OUT="C1-missing-directory build/containers: the tree is absent"; fi
if [ -z "$OUT" ]; then ok "REAL build/containers tree: every required directory, file, pin, checksum and lock entry check passes"
else bad "REAL build/containers tree has $(printf '%s\n' "$OUT" | wc -l) violations: $(printf '%s' "$OUT" | head -6 | tr '\n' '|')"; fi

# ---------------------------------------------------------------- fixtures (golden-bad, golden-good, negative control)
D64="$(printf 'a%.0s' $(seq 64))"; D64B="$(printf 'b%.0s' $(seq 64))"; SH64="$(printf 'c%.0s' $(seq 64))"
mkfix() { # <name> : writes $T/fx-<name>/tree and $T/fx-<name>/lock.yaml; the caller adds directories
  rm -rf "$T/fx-$1"; mkdir -p "$T/fx-$1/tree"; printf 'schema: 1\nimages: []\n' >"$T/fx-$1/lock.yaml"; }
gooddir() { # <fxname> <dir> : a compliant directory
  local p="$T/fx-$1/tree/$2"; mkdir -p "$p"
  printf 'FROM docker.io/library/debian@sha256:%s\nRUN curl -fsSL -o /tmp/f https://example.invalid/f && echo "%s  /tmp/f" | sha256sum -c -\n' "$D64" "$SH64" >"$p/Containerfile"
  printf '# %s\n' "$2" >"$p/README.md"
  printf 'base docker.io/library/debian@sha256:%s\ndownload https://example.invalid/f sha256:%s\n' "$D64" "$SH64" >"$p/digests.lock"; }
addlock() { # <fxname> <id> <reference> <digest> [class]
  python3 -I - "$T/fx-$1/lock.yaml" "$2" "$3" "$4" "${5:-}" <<'PY'
import sys, yaml
f, i, ref, dg, cls = sys.argv[1:6]
d = yaml.safe_load(open(f)); e = {"id": i, "reference": ref, "digest": dg}
if cls: e["class"] = cls
d["images"].append(e); open(f, "w").write(yaml.safe_dump(d))
PY
}
fxcheck() { # <fxname> <required>  -> OUT
  OUT="$(python3 -I "$CHECKER" "$REPO" "$T/fx-$1/tree" "$T/fx-$1/lock.yaml" "$REPO/scripts/containers/check_pins.sh" "${2:-}" 2>&1)"; }
expect_rule() { # <label> <rule-prefix>
  if printf '%s\n' "$OUT" | grep -q "^$2"; then ok "$1 -> $2 fires"; else bad "$1: no '$2' line in: $(printf '%s' "$OUT" | head -4 | tr '\n' '|')"; fi; }

mkfix good; gooddir good go; addlock good IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck good go; check "GOLDEN-GOOD a compliant directory produces no violation" "$OUT" ""

mkfix tagfrom; gooddir tagfrom go; sed -i "1s#.*#FROM docker.io/library/debian:12#" "$T/fx-tagfrom/tree/go/Containerfile"; addlock tagfrom IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck tagfrom; expect_rule "GOLDEN-BAD tag-only FROM" C3-check_pins

mkfix pipe; gooddir pipe go; printf 'RUN curl -fsSL https://example.invalid/i.sh | sh\n' >>"$T/fx-pipe/tree/go/Containerfile"; addlock pipe IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck pipe; expect_rule "GOLDEN-BAD pipe-to-shell" C3-check_pins

mkfix nosha; gooddir nosha go; printf 'RUN curl -fsSL -o /tmp/g https://example.invalid/g && chmod +x /tmp/g\n' >>"$T/fx-nosha/tree/go/Containerfile"; addlock nosha IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck nosha; expect_rule "GOLDEN-BAD download without a SHA-256 check" C4-download-without-sha256

mkfix noreadme; gooddir noreadme go; rm "$T/fx-noreadme/tree/go/README.md"; addlock noreadme IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck noreadme; expect_rule "GOLDEN-BAD README.md missing" C2-README.md

mkfix nolock; gooddir nolock go; rm "$T/fx-nolock/tree/go/digests.lock"; addlock nolock IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck nolock; expect_rule "GOLDEN-BAD digests.lock missing" C2-digests.lock

mkfix drift; gooddir drift go; printf 'base docker.io/library/debian@sha256:%s\n' "$D64B" >"$T/fx-drift/tree/go/digests.lock"; addlock drift IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck drift; expect_rule "GOLDEN-BAD digests.lock does not name the FROM digest" C5-digests.lock-from
expect_rule "GOLDEN-BAD digests.lock does not name the checked SHA-256" C5-digests.lock-sha256

mkfix noentry; gooddir noentry go
fxcheck noentry; expect_rule "GOLDEN-BAD no images.lock.yaml entry" C6-lock-entry

mkfix noclass; gooddir noclass node; addlock noclass IMG-NODE docker.io/library/debian "sha256:$D64"
fxcheck noclass; expect_rule "GOLDEN-BAD lock entry without a class" C6-class

mkfix mismatch; gooddir mismatch go; addlock mismatch IMG-GO docker.io/library/debian "sha256:$D64B" compile
fxcheck mismatch; expect_rule "GOLDEN-BAD lock entry digest differs from the Containerfile FROM digest" C6-digest-mismatch

mkfix aptcurl; gooddir aptcurl go; printf 'RUN apt-get install -y --no-install-recommends curl=7.88.1 ca-certificates\n' >>"$T/fx-aptcurl/tree/go/Containerfile"; addlock aptcurl IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck aptcurl; check "GOLDEN-GOOD apt-get installing the curl package is not a download line" "$OUT" ""

mkfix deferred; gooddir deferred rust
fxcheck deferred; check "GOLDEN-GOOD rust needs no lock entry (T143 writes it)" "$OUT" ""

mkfix missing; gooddir missing go; addlock missing IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck missing "go,node"; expect_rule "GOLDEN-BAD required directory absent" C1-missing-directory

mkfix empty; mkdir -p "$T/fx-empty/tree/go"; : >"$T/fx-empty/tree/go/Containerfile"
fxcheck empty; expect_rule "GOLDEN-BAD empty Containerfile" C2-containerfile

# negative control: a compliant directory next to a bad one is not blamed
mkfix neg; gooddir neg go; gooddir neg node; sed -i "1s#.*#FROM docker.io/library/node:20#" "$T/fx-neg/tree/node/Containerfile"; addlock neg IMG-GO docker.io/library/debian "sha256:$D64" compile; addlock neg IMG-NODE docker.io/library/debian "sha256:$D64" compile
fxcheck neg; check "NEGCTL the compliant directory 'go' is never named" "$(printf '%s\n' "$OUT" | grep -c ' go:')" "0"
check "NEGCTL the bad directory 'node' is named" "$([ "$(printf '%s\n' "$OUT" | grep -c ' node:')" -ge 1 ] && echo yes || echo no)" "yes"

# ---------------------------------------------------------------- WF10 review p1 fixes (F3 C3 on a crash, F4 survivors, F6 swallowed failure, ADD url, download/check counts)
fxcheck_pins() { # <fxname> <check_pins path or stub> [required] -> OUT
  OUT="$(python3 -I "$CHECKER" "$REPO" "$T/fx-$1/tree" "$T/fx-$1/lock.yaml" "$2" "${3:-}" 2>&1)"; }
expect_no_rule() { # <label> <rule-prefix>
  if printf '%s\n' "$OUT" | grep -q "^$2"; then bad "$1: unexpected '$2' line in: $(printf '%s' "$OUT" | head -3 | tr '\n' '|')"; else ok "$1 -> $2 silent"; fi; }

mkfix wget; gooddir wget go; printf 'RUN wget -qO /tmp/g https://example.invalid/g && chmod +x /tmp/g\n' >>"$T/fx-wget/tree/go/Containerfile"; addlock wget IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck wget; expect_rule "GOLDEN-BAD wget download without a SHA-256 check" C4-download-without-sha256

mkfix nosum; gooddir nosum go; printf 'RUN curl -fsSL -o /tmp/g https://example.invalid/g && echo "%s  /tmp/g" | sha256sum\n' "$SH64" >>"$T/fx-nosum/tree/go/Containerfile"; addlock nosum IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck nosum; expect_rule "GOLDEN-BAD sha256sum without -c is not a check" C4-download-without-sha256

mkfix twodl; gooddir twodl go; printf 'RUN curl -fsSL -o /tmp/a https://example.invalid/a && curl -fsSL -o /tmp/b https://example.invalid/b && echo "%s  /tmp/a" | sha256sum -c -\n' "$SH64" >>"$T/fx-twodl/tree/go/Containerfile"; addlock twodl IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck twodl; expect_rule "GOLDEN-BAD two downloads in one RUN checked by one sha256sum -c" C4-download-without-sha256

mkfix twodl2; gooddir twodl2 go; printf 'RUN curl -fsSL -o /tmp/a https://example.invalid/a && echo "%s  /tmp/a" | sha256sum -c - && curl -fsSL -o /tmp/b https://example.invalid/b && echo "%s  /tmp/b" | sha256sum -c -\n' "$SH64" "$SH64" >>"$T/fx-twodl2/tree/go/Containerfile"; addlock twodl2 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck twodl2; expect_no_rule "GOLDEN-GOOD two downloads each followed by its own check" C4-

mkfix order; gooddir order go; printf 'RUN echo "%s  /tmp/g" | sha256sum -c - && curl -fsSL -o /tmp/g https://example.invalid/g && sha256sum /tmp/g\n' "$SH64" >>"$T/fx-order/tree/go/Containerfile"; addlock order IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck order; expect_rule "GOLDEN-BAD the SHA-256 check runs BEFORE the download (a later plain sha256sum is not a check)" C4-download-without-sha256

mkfix twodl3; gooddir twodl3 go; printf 'RUN curl -fsSL -o /tmp/a https://example.invalid/a && curl -fsSL -o /tmp/b https://example.invalid/b && echo "%s  /tmp/a" | sha256sum -c - && sha256sum /tmp/b\n' "$SH64" >>"$T/fx-twodl3/tree/go/Containerfile"; addlock twodl3 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck twodl3; expect_rule "GOLDEN-BAD two downloads, one real check and one plain sha256sum" C4-download-without-sha256

mkfix addurl; gooddir addurl go; printf 'ADD https://example.invalid/h.tgz /opt/h.tgz\n' >>"$T/fx-addurl/tree/go/Containerfile"; addlock addurl IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck addurl; expect_rule "GOLDEN-BAD ADD <url> without --checksum" C4-download-without-sha256

mkfix addurl2; gooddir addurl2 go; printf 'ADD --checksum=sha256:%s https://example.invalid/h.tgz /opt/h.tgz\n' "$SH64" >>"$T/fx-addurl2/tree/go/Containerfile"; addlock addurl2 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck addurl2; expect_no_rule "GOLDEN-GOOD ADD <url> with --checksum=sha256:<64 hex>" C4-

mkfix swallow; gooddir swallow go; printf 'RUN yes | sdkmanager --licenses >/dev/null 2>&1 || true\n' >>"$T/fx-swallow/tree/go/Containerfile"; addlock swallow IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck swallow; expect_rule "GOLDEN-BAD a RUN that swallows a failure with || true" C7-swallowed-failure

mkfix swallow2; gooddir swallow2 go; printf 'RUN rm -f /tmp/x || :\n' >>"$T/fx-swallow2/tree/go/Containerfile"; addlock swallow2 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck swallow2; expect_rule "GOLDEN-BAD a RUN that swallows a failure with || :" C7-swallowed-failure

mkfix swallow3; gooddir swallow3 go; printf '# || true is discussed here only\nRUN yes | sdkmanager --licenses >/dev/null\nRUN test -f /a || test -f /b\n' >>"$T/fx-swallow3/tree/go/Containerfile"; addlock swallow3 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck swallow3; expect_no_rule "GOLDEN-GOOD a comment mentioning || true, a plain pipeline, an || with a real command" C7-

mkfix badclass; gooddir badclass node; addlock badclass IMG-NODE docker.io/library/debian "sha256:$D64" bogus
fxcheck badclass; expect_rule "GOLDEN-BAD lock entry class outside the closed set" C6-class

mkfix emptyreadme; gooddir emptyreadme go; : >"$T/fx-emptyreadme/tree/go/README.md"; addlock emptyreadme IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck emptyreadme; expect_rule "GOLDEN-BAD empty README.md" C2-README.md

mkfix emptylock; gooddir emptylock go; : >"$T/fx-emptylock/tree/go/digests.lock"; addlock emptylock IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck emptylock; expect_rule "GOLDEN-BAD empty digests.lock" C2-digests.lock

mkfix unknowndir; gooddir unknowndir zzz
fxcheck unknowndir; expect_rule "GOLDEN-BAD a directory with no id mapping" C6-unknown-directory

# C3 must refuse anything that is not a clean verdict: a crash (3), a usage error (2), a missing script (127), exit 1 with no VIOLATION line, exit 0 with a VIOLATION line
STUB="$T/stubs"; mkdir -p "$STUB"
printf '#!/bin/sh\nexit 2\n' >"$STUB/rc2.sh"; printf '#!/bin/sh\necho boom >&2\nexit 3\n' >"$STUB/rc3.sh"; printf '#!/bin/sh\nexit 1\n' >"$STUB/rc1_empty.sh"
printf '#!/bin/sh\necho "VIOLATION r p:1: x"\nexit 0\n' >"$STUB/rc0_viol.sh"; printf '#!/bin/sh\nexit 0\n' >"$STUB/rc0.sh"; printf '#!/bin/sh\necho "VIOLATION r p:1: x"\nexit 1\n' >"$STUB/rc1_viol.sh"
mkfix c3; gooddir c3 go; addlock c3 IMG-GO docker.io/library/debian "sha256:$D64" compile
for st in rc2 rc3 rc1_empty rc0_viol; do fxcheck_pins c3 "$STUB/$st.sh"; expect_rule "GOLDEN-BAD C3 refuses check_pins stub $st" C3-check_pins; done
fxcheck_pins c3 "$T/stubs/does-not-exist.sh"; expect_rule "GOLDEN-BAD C3 refuses a missing check_pins (exit 127)" C3-check_pins
fxcheck_pins c3 "$STUB/rc1_viol.sh"; expect_rule "GOLDEN-BAD C3 reports each VIOLATION line of an exit-1 verdict" C3-check_pins
check "GOLDEN-BAD C3 names the check_pins VIOLATION line of an exit-1 verdict" "$OUT" "C3-check_pins go: VIOLATION r p:1: x"
fxcheck_pins c3 "$STUB/rc0.sh"; check "GOLDEN-GOOD C3 accepts exit 0 with no VIOLATION line" "$OUT" ""
# the reviewer's own demonstration: a real check_pins copy that crashes inside scan_dockerfile
python3 -I - "$REPO/scripts/containers/check_pins.sh" "$STUB/crash_pins.sh" <<'PYEND'
import sys
src = open(sys.argv[1]).read(); old = "def scan_dockerfile(path, text):\n"
if src.count(old) != 1: sys.exit(3)
open(sys.argv[2], "w").write(src.replace(old, old + "    raise RuntimeError('injected crash')\n"))
PYEND
fxcheck_pins c3 "$STUB/crash_pins.sh"; expect_rule "GOLDEN-BAD C3 refuses a crashing real check_pins (exit 3, no VIOLATION line)" C3-check_pins

# ---------------------------------------------------------------- WF13 round 3 (11.4.276 structural round): C4 and C7 judge the PARSED commands of a RUN, not a count of regex hits (N4, N9)
CFN=0
cf_case() { # <label> <Containerfile line> <C4-download-without-sha256 | C7-swallowed-failure | clean>
  CFN=$((CFN+1)); local n="r3c$CFN"
  mkfix "$n"; gooddir "$n" go; printf '%s\n' "$2" >>"$T/fx-$n/tree/go/Containerfile"; addlock "$n" IMG-GO docker.io/library/debian "sha256:$D64" compile
  fxcheck "$n"
  if [ "$3" = clean ]; then check "$1 (clean)" "$OUT" ""; else expect_rule "$1" "$3"; fi
}
C4B="C4-download-without-sha256"; C7B="C7-swallowed-failure"
CHK="echo \"$SH64  /tmp/f\" | sha256sum -c -"
cf_case "R3-C4 quoted URL"                                   'RUN curl "https://example.invalid/g" -o /tmp/g && chmod +x /tmp/g' "$C4B"
cf_case "R3-C4 variable URL"                                 'RUN curl "$URL" -o /tmp/g' "$C4B"
cf_case "R3-C4 wget with a quoted URL"                       'RUN wget "https://example.invalid/g" -O /tmp/g' "$C4B"
cf_case "R3-C4 URL before the options (curl URL -o f)"       'RUN curl https://example.invalid/f -o /f' "$C4B"
cf_case "R3-C4 wget URL first"                               'RUN wget https://example.invalid/f -O /f' "$C4B"
cf_case "R3-C4 the second download comes AFTER both checks"  "RUN curl -fsSL -o /tmp/a https://a.invalid/a && $CHK && $CHK && curl -fsSL -o /tmp/b https://b.invalid/b" "$C4B"
cf_case "R3-C4 two downloads (the second quoted), one check" "RUN curl -fsSL -o /tmp/a https://a.invalid/a && curl \"https://b.invalid/b\" -o /tmp/b && $CHK" "$C4B"
cf_case "R3-C4 set -e does not replace a check"              'RUN set -e; curl -fsSL -o /tmp/f https://x.invalid/f && chmod +x /tmp/f' "$C4B"
cf_case "R3-C4 download as the last command after a check of something else" "RUN $CHK && curl -fsSL -o /tmp/g https://x.invalid/g" "$C4B"
cf_case "R3-C4 shasum -a 256 without -c is not a check"      "RUN curl -fsSL -o /tmp/f https://example.invalid/f && echo \"$SH64  /tmp/f\" | shasum -a 256" "$C4B"
cf_case "R3-C4 sha256sum with an unrelated option is not a check" "RUN curl -fsSL -o /tmp/f https://example.invalid/f && echo \"$SH64  /tmp/f\" | sha256sum -b -" "$C4B"
cf_case "R3-C4 control: quoted URL with its check"           "RUN curl \"https://example.invalid/f\" -o /tmp/f && $CHK" clean
cf_case "R3-C4 control: sha256sum --check"                   "RUN curl -fsSL -o /tmp/f https://example.invalid/f && echo \"$SH64  /tmp/f\" | sha256sum --check -" clean
cf_case "R3-C4 control: shasum -a 256 -c"                    "RUN curl -fsSL -o /tmp/f https://example.invalid/f && echo \"$SH64  /tmp/f\" | shasum -a 256 -c -" clean
cf_case "R3-C4 control: command -v curl and curl --version are lookups" 'RUN command -v curl && curl --version' clean
cf_case "R3-C4 control: command -v curl with redirections is a lookup" 'RUN command -v curl >/dev/null 2>&1 && echo ok' clean
cf_case "R3-C4 control: which curl / type curl"              'RUN which curl; type curl' clean
cf_case "R3-C7 || /bin/true"                                 'RUN rm -f /tmp/x || /bin/true' "$C7B"
cf_case "R3-C7 || exit 0"                                    'RUN rm -f /tmp/x || exit 0' "$C7B"
cf_case "R3-C7 || echo ignored"                              'RUN rm -f /tmp/x || echo ignored' "$C7B"
cf_case "R3-C7 || { true; }"                                 'RUN rm -f /tmp/x || { true; }' "$C7B"
cf_case "R3-C7 || ( true )"                                  'RUN rm -f /tmp/x || ( true )' "$C7B"
cf_case "R3-C7 || { echo x; true; }"                         'RUN rm -f /tmp/x || { echo x; true; }' "$C7B"
cf_case "R3-C7 set +e"                                       'RUN set +e; rm -f /tmp/x; echo done' "$C7B"
cf_case "R3-C7 ||true without a space"                       'RUN rm -f /x ||true' "$C7B"
cf_case "R3-C7 || printf"                                    'RUN rm -f /tmp/x || printf done' "$C7B"
cf_case "R3-C7 control: || { echo; exit 1; } is an explicit handler" 'RUN rm -f /tmp/x || { echo fail >&2; exit 1; }' clean
cf_case "R3-C7 control: || exit 1"                           'RUN rm -f /tmp/x || exit 1' clean
cf_case "R3-C7 control: || false"                            'RUN rm -f /tmp/x || false' clean
cf_case "R3-C7 control: || echo x && exit 1"                 'RUN rm -f /tmp/x || echo x && exit 1' clean
cf_case "R3-C7 control: || ( echo x; exit 2 )"               'RUN rm -f /tmp/x || ( echo x; exit 2 )' clean
cf_case "R3-C7 control: || test -f"                          'RUN test -f /a || test -f /b' clean
# a RUN whose shell text cannot be parsed and names a download is refused, not guessed
cf_case "R3-C4 an unparseable RUN that names a download is refused" "RUN curl -fsSL -o /tmp/f 'https://example.invalid/f && echo done" "$C4B"
# the independent reviewer's round-2 survivors (RC1-RC4), each with its distinguishing input
mkfix rc1; gooddir rc1 go; printf 'ADD --checksum=sha256:%s https://x.invalid/f /f\n' "$(printf 'e%.0s' $(seq 64))" >>"$T/fx-rc1/tree/go/Containerfile"; addlock rc1 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck rc1; expect_rule "RC1 the ADD --checksum value must be named in digests.lock" C5-digests.lock-sha256
mkfix rc2; gooddir rc2 go; printf 'RUN echo hi\n' >"$T/fx-rc2/tree/go/Containerfile"; addlock rc2 IMG-GO docker.io/library/debian "sha256:$D64" compile
fxcheck rc2; expect_rule "RC2 a Containerfile with no FROM line" C2-from

# ---------------------------------------------------------------- paired mutations of this file
if [ -z "${CF_TEST_MUTANT:-}" ] && [ -z "${CF_TEST_NO_MUTATIONS:-}" ]; then
  REC="${CF_MUTATION_RECORD:-$T/containerfiles-mutation.txt}"
  { echo "# containerfiles-mutation record: paired mutations of the checker in scripts/containers/tests/test_containerfiles.sh"; echo "# run_at: $(date -u +%Y-%m-%dT%H:%M:%SZ) host: $(hostname)"; echo "# self_sha256: $(sha256sum "$SELF" | cut -d' ' -f1)"; } >"$REC"
  nm=0
  while IFS=$'\t' read -r name old new; do
    case "$name" in ""|"#"*) continue;; esac
    nm=$((nm+1)); cp_="$T/mutant-$name.sh"
    if ! python3 -I - "$SELF" "$cp_" "$old" "$new" <<'PY'
import sys
s = open(sys.argv[1]).read(); old = sys.argv[3].encode().decode("unicode_escape"); new = sys.argv[4].encode().decode("unicode_escape")
if old == new or s.count(old) != 1: print("anchor count %d" % s.count(old)); sys.exit(3)
open(sys.argv[2], "w").write(s.replace(old, new))
PY
    then bad "MUT $name: cannot be applied"; echo "MUTANT $name: NOT-APPLIED" >>"$REC"; continue; fi
    mo="$(CF_SELF="$cp_" CF_TEST_ROOT="$REPO" CF_TEST_MUTANT=1 QUIET=1 bash "$cp_" 2>&1)"; mrc=$?
    if [ "$mrc" -ne 0 ]; then ok "MUT $name: caught"; echo "MUTANT $name: CAUGHT first_fail=$(printf '%s' "$mo" | grep -m1 '^FAIL')" >>"$REC"
    else bad "MUT $name: SURVIVED"; echo "MUTANT $name: SURVIVED" >>"$REC"; fi
  done <"$HERE/containerfiles_mutations.tsv"
  echo "# mutations: $nm" >>"$REC"
fi

echo "test_containerfiles: $PASSES passed, $FAILS failed"
[ "$FAILS" -eq 0 ]
