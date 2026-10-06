#!/usr/bin/env bash
# check_pins.sh - T105 (docs/16 section 7.1 step 3, finding D-07, D-05; constitution 11.4.246/11.4.264 digest identity, 11.4.173).
# Fails when a compose file, a Dockerfile/Containerfile or a shell script references an EXTERNAL image without a full
# `@sha256:<64 lowercase hex>` digest, or installs software by piping a download into a shell.
#
# Usage: check_pins.sh [--root DIR] [--list] [--recurse] [--exclude-dir NAME]... [PATH...]
#   --root DIR          directory the reported paths are relative to (default: the git top level of the cwd, else the cwd)
#   PATH...             files or directories (relative to the root) to scan; default: every tracked file of the root
#                       (`git ls-files`; with --recurse also the files of every submodule checkout, `--recurse-submodules`),
#                       or every file under the root when it is not a git work tree. Directories .git, node_modules, vendor and
#                       .audit are never entered.
#   --list              print one TSV row per violation `rule<TAB>path<TAB>line<TAB>reference` and nothing else (the baseline form)
#   --exclude-dir NAME  skip directories with this exact name (repeatable)
# Exit: 0 no violation; 1 at least one violation; 2 usage error (unknown option, root missing or not a directory).
#
# Rules (the rule id is the second word of every `VIOLATION <rule> <path>:<line>: <reference or excerpt>` line):
#   compose_image_unpinned  `image:` of a compose file (a name containing `compose`, extension .yml/.yaml) without a full digest.
#                           `${VAR:-default}` is judged by its default; a bare `${VAR}` is resolved by the caller and not judged;
#                           `localhost/...` images are locally built (their FROM lines are judged), not external, and are skipped.
#   from_unpinned           `FROM` of a Dockerfile/Containerfile naming an external image without a full digest (tag-only and
#                           no-tag references alike; build-stage aliases, `scratch` and `localhost/` bases are not external).
#   from_arg_unpinned       `FROM ${ARG}` whose `ARG` default is present and carries no full digest (an ARG without a default is
#                           supplied by the builder and is not judged here).
#   copy_from_unpinned      `COPY --from=<external image>` without a full digest (a stage alias or stage number is not an image).
#   script_image_unpinned   a shell script naming an external image without a digest: the image operand of `docker|podman|nerdctl
#                           run|pull|create`, or any registry-qualified reference carrying a tag (docker.io, ghcr.io, quay.io,
#                           mcr.microsoft.com, gcr.io, lscr.io, registry.*, *.pkg.dev, public.ecr.aws).
#   pipe_to_shell           a download (`curl`, `wget`, `fetch`) piped into a shell (`| sh`, `| bash -`, `| sudo bash`), the
#                           `bash <(curl ...)` and `sh -c "$(curl ...)"` forms, in a Dockerfile/Containerfile or a script; a
#                           pipe after a `for ... done` loop spread over continuation lines is one logical line and is caught;
#                           the quoted install hint a script prints for the operator is flagged too (it instructs an unverified
#                           install). Reported at the first physical line of the logical line.
# Carriers do NOT fire (11.4.201: a mention is not the thing): full-line and trailing `#` comments are removed before judging, and
# Markdown, text and every other file type is not scanned at all. In a script under a `tests` directory, or named test_* /
# mutate_*, here-document bodies are fixture DATA and are not judged (a violation fixture written by a test is not a violation of the
# test); in any other script a here-document body is judged like code (a script that generates a Dockerfile is scanned).
# Determinism: output is sorted by path, line, rule; two runs over the same tree print the same bytes.
# Honest boundary: the scan reads text; it does not resolve a registry, does not expand variables other than a `${VAR:-default}`
# default, and does not prove a digest exists or is signed (T149). Documented in docs/scripts/check_pins.md.
set -u
exec python3 -I - "$@" <<'PY'
import os, re, subprocess, sys

DIGEST_RE = re.compile(r"@sha256:[0-9a-f]{64}(?![0-9A-Za-z])")
SKIP_DIRS = {".git", "node_modules", "vendor", ".audit"}
REGISTRY_RE = re.compile(
    r"(?<![\w./-])((?:docker\.io|ghcr\.io|quay\.io|mcr\.microsoft\.com|gcr\.io|lscr\.io|registry\.[\w.-]+|[\w-]+\.pkg\.dev|public\.ecr\.aws)"
    r"/[A-Za-z0-9._/-]+(?::[A-Za-z0-9._-]+)?(?:@sha256:[0-9A-Za-z]*)?)")
SHELL_ALT = r"(?:(?:ba|z|da|a|k)?sh\b|(?:python[0-9.]*|perl|ruby)\s+-(?![\w-]))"
PIPE_RE = re.compile(
    r"\b(?:curl|wget|fetch)\b[^\n]*?(?<!\|)\|(?!\|)\s*(?:sudo\s+(?:-\S+\s+)*)?(?:env\s+\S+=\S+\s+)*(?:/usr/(?:local/)?bin/|/bin/)?" + SHELL_ALT)
SUBST_RE = re.compile(
    r"(?:\b(?:(?:ba|z|da|a|k)?sh|eval)\s+(?:-c\s+)?[\"']?\$\(\s*(?:curl|wget|fetch)\b[^\n]*"
    r"|(?:\b(?:(?:ba|z|da|a|k)?sh|source)|(?<![\w.])\.)\s+<\(\s*(?:curl|wget|fetch)\b[^\n]*)")
# options of `docker|podman|nerdctl run|create|pull` that take a separate value (so the value is not the image operand)
VALUE_OPTS = {
    "-a", "--attach", "--add-host", "--annotation", "--arch", "--authfile", "--blkio-weight", "--blkio-weight-device", "--cap-add",
    "--cap-drop", "--cert-dir", "--cgroup-conf", "--cgroup-parent", "--cgroupns", "--cgroups", "--cidfile", "--conmon-pidfile", "--cpu-period",
    "--cpu-quota", "--cpu-rt-period", "--cpu-rt-runtime", "--cpu-shares", "--cpus", "--cpuset-cpus", "--cpuset-mems", "--creds",
    "--decryption-key", "--detach-keys", "--device", "--device-cgroup-rule", "--device-read-bps", "--device-read-iops", "--device-write-bps",
    "--device-write-iops", "--dns", "--dns-option", "--dns-search", "--domainname", "-e", "--env", "--entrypoint", "--env-file", "--env-host",
    "--env-merge", "--expose", "--gidmap", "--gpus", "--group-add", "--group-entry", "--health-cmd", "--health-interval", "--health-on-failure",
    "--health-retries", "--health-start-period", "--health-startup-cmd", "--health-startup-interval", "--health-startup-retries",
    "--health-startup-success", "--health-startup-timeout", "--health-timeout", "-h", "--hostname", "--hooks-dir", "--hostuser", "--image-volume",
    "--init-path", "--ip", "--ip6", "--ipc", "--isolation", "-l", "--label", "--label-file", "--link", "--link-local-ip", "--log-driver",
    "--log-opt", "--mac-address", "-m", "--memory", "--memory-reservation", "--memory-swap", "--memory-swappiness", "--mount", "--name", "--net",
    "--network", "--network-alias", "--no-healthcheck-x", "--oom-score-adj", "--os", "--passwd-entry", "--personality", "--pid", "--pidfile",
    "--pids-limit", "--platform", "--pod", "--pod-id-file", "-p", "--publish", "--pull", "--rdt-class", "--restart", "--retry", "--retry-delay",
    "--runtime", "--seccomp-policy", "--secret", "--security-opt", "--shm-size", "--shm-size-systemd", "--stop-signal", "--stop-timeout",
    "--storage-opt", "--subgidname", "--subuidname", "--sysctl", "--timeout", "--tmpfs", "--tz", "--uidmap", "-u", "--user", "--userns",
    "--uts", "--variant", "-v", "--volume", "--volumes-from", "-w", "--workdir"}
# global options (before the sub-command) that take a separate value
GLOBAL_VALUE_OPTS = {"--log-level", "--root", "--runroot", "--storage-driver", "--storage-opt", "--url", "--connection", "-c", "--host", "-H",
                     "--config", "--context", "--namespace", "--cgroup-manager", "--conmon", "--events-backend", "--hooks-dir", "--identity",
                     "--imagestore", "--network-cmd-path", "--network-config-dir", "--runtime", "--runtime-flag", "--ssh", "--tmpdir",
                     "--volumepath", "--cdi-spec-dir", "--userns-uid-map", "--userns-gid-map", "--module", "-l"}
ENGINES = {"docker", "podman", "nerdctl"}
ENGINE_VERBS = {"run", "pull", "create"}
WRAPPERS = {"sudo", "env", "time", "exec", "nohup", "command", "timeout", "nice", "ionice", "xargs", "stdbuf", "then", "do", "else", "elif",
            "if", "while", "until", "!", "{", "watch", "setsid", "builtin", "doas"}
SHELLISH = {"sh", "bash", "zsh", "dash", "ash", "ksh", "eval", "ssh"}
# in a test script these commands only WRITE or FILTER data: a fixture line they carry is not a command of the test
DATA_CMDS = {"printf", "echo", "sed", "tee", "grep", "egrep", "fgrep", "awk"}
ASSIGN_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
DURATION_RE = re.compile(r"^\d+(?:\.\d+)?[smhd]?$")
IMG_SHAPE = re.compile(r"^[a-z0-9][A-Za-z0-9._-]*(?::[0-9]+)?(?:/[A-Za-z0-9._-]+)*(?::[A-Za-z0-9._-]+)?(?:@sha256:[0-9A-Za-z]*)?$")
# a here-document opener at the top level of a logical line: quoted strings and arithmetic `$((..))` / `((..))` are consumed first
HDOC_SCAN_RE = re.compile(r"""'[^']*'|"(?:[^"\\]|\\.)*"|\$?\(\([^()]*\)\)|(?<!<)<<(?!<)-?\s*(['"]?)(\w+)\1""")
CONT_TRAIL_RE = re.compile(r"(?:(?<!\|)\||\|\||&&)\s*$")

viol = []      # (path, line, rule, reference)
files_scanned = 0


def usage(msg):
    sys.stderr.write("check_pins: %s\n" % msg)
    sys.stderr.write("usage: check_pins.sh [--root DIR] [--list] [--recurse] [--exclude-dir NAME]... [PATH...]\n")
    sys.exit(2)


def pinned(ref):
    return DIGEST_RE.search(ref) is not None


def strip_comment(s):
    """Remove a trailing `#` comment (a `#` outside quotes that starts the line or follows whitespace)."""
    q = ""
    for i, ch in enumerate(s):
        if q:
            if ch == q:
                q = ""
        elif ch in "'\"":
            q = ch
        elif ch == "#" and (i == 0 or s[i - 1].isspace()):
            return s[:i]
    return s


def top_level_heredoc(acc):
    for m in HDOC_SCAN_RE.finditer(acc):
        if m.group(2):
            return m.group(2)
    return None


def logical_lines(text, skip_heredocs):
    """Yield (first_physical_line_no, text) with continuation lines (backslash, or a trailing | || &&) joined and comments removed."""
    out = []
    acc = None
    start = 0
    heredoc_end = None
    for no, raw in enumerate(text.split("\n"), 1):
        line = raw.rstrip("\r")
        if heredoc_end is not None:
            if line.strip() == heredoc_end:
                heredoc_end = None
            continue
        stripped = line.strip()
        if stripped.startswith("#"):
            continue
        body = strip_comment(line)
        body = body.rstrip()
        cont = line.rstrip().endswith("\\")
        if body.endswith("\\"):
            body = body[:-1]
        elif CONT_TRAIL_RE.search(body):
            cont = True
        if acc is None:
            acc, start = body, no
        else:
            acc += " " + body.strip()
        if not cont:
            out.append((start, acc))
            if skip_heredocs:
                delim = top_level_heredoc(acc)
                if delim:
                    heredoc_end = delim
            acc = None
    if acc is not None:
        out.append((start, acc))
    return out


def add(path, line, rule, ref):
    viol.append((path, line, rule, " ".join(ref.split())[:200]))


def split_commands(s):
    """Split one logical line into simple commands at unquoted ; & | ( ) ` and newlines.
    Returns [(start, end, [(token_text_without_quotes, was_quoted), ...]), ...]; start/end are offsets into s."""
    cmds, toks, cur = [], [], []
    state = {"quoted": False, "has": False, "seg": 0}
    n = len(s)

    def endtok():
        if state["has"]:
            toks.append(("".join(cur), state["quoted"]))
        del cur[:]
        state["quoted"] = False
        state["has"] = False

    def endcmd(pos):
        endtok()
        if toks:
            cmds.append((state["seg"], pos, list(toks)))
        del toks[:]
        state["seg"] = pos + 1

    i = 0
    while i < n:
        ch = s[i]
        if ch == "\\" and i + 1 < n:
            cur.append(s[i + 1]); state["has"] = True; i += 2
        elif ch == "'":
            j = s.find("'", i + 1)
            if j < 0:
                j = n
            cur.append(s[i + 1:j]); state["has"] = True; state["quoted"] = True; i = j + 1
        elif ch == '"':
            j = i + 1
            buf = []
            while j < n and s[j] != '"':
                if s[j] == "\\" and j + 1 < n:
                    buf.append(s[j + 1] if s[j + 1] in '"\\$`' else s[j:j + 2]); j += 2
                else:
                    buf.append(s[j]); j += 1
            cur.append("".join(buf)); state["has"] = True; state["quoted"] = True; i = j + 1
        elif ch in " \t":
            endtok(); i += 1
        elif ch in ";\n|()`":
            endcmd(i); i += 1
        elif ch == "&":
            if cur and cur[-1][-1:] in ("<", ">"):
                cur.append("&"); state["has"] = True
            else:
                endcmd(i)
            i += 1
        else:
            cur.append(ch); state["has"] = True; i += 1
    endcmd(n)
    return cmds


def head_index(tokens):
    """Index of the command word after wrappers (sudo, env, timeout 30, VAR=x, ...); None when there is none."""
    i = 0
    while i < len(tokens):
        t, q = tokens[i]
        if ASSIGN_RE.match(t):              # VAR=value (the value may have been quoted)
            i += 1
            continue
        if q:
            break
        base = os.path.basename(t)
        if base == "sudo":
            i += 1
            while i < len(tokens) and tokens[i][0].startswith("-") and not tokens[i][1]:
                opt = tokens[i][0]
                i += 1
                if opt in ("-u", "-g", "-C", "-h", "-p", "-r", "-t", "-U", "-D") and i < len(tokens):
                    i += 1
            continue
        if base in WRAPPERS or DURATION_RE.match(t) or (t.startswith("-") and i > 0):
            i += 1
            continue
        break
    return i if i < len(tokens) else None


def looks_like_value(t):
    """A token that is the value of an unknown option, not an image (key=value, a port or address, host:ip, host:host-gateway)."""
    return ("=" in t or re.match(r"^\d+(?:\.\d+){0,3}(?::\d+)?$", t) is not None
            or re.match(r"^[A-Za-z0-9.-]+:(?:\d+\.){3}\d+$", t) is not None or re.match(r"^[A-Za-z0-9.-]+:host-gateway$", t) is not None)


def image_operand(tokens):
    i = 0
    after_unknown = False
    while i < len(tokens):
        t, q = tokens[i]
        if t == "--" and not q:
            i += 1
            after_unknown = False
            continue
        if not q and t.startswith("-") and len(t) > 1:
            if "=" in t:
                i += 1
                after_unknown = False
            elif t in VALUE_OPTS:
                i += 2
                after_unknown = False
            else:
                i += 1
                after_unknown = True      # an unknown option: a boolean unless the next token cannot be an image operand
            continue
        if re.match(r"^\d*[<>]", t) and not q:
            i += 1
            continue
        if after_unknown and looks_like_value(t):
            i += 1
            after_unknown = False
            continue
        return t
    return None


def subst_texts(t):
    """Texts of $(...) and `...` substitutions inside a quoted token."""
    out = []
    k = t.find("$(")
    while k >= 0:
        out.append(t[k + 2:])
        k = t.find("$(", k + 2)
    parts = t.split("`")
    for idx in range(1, len(parts), 2):
        out.append(parts[idx])
    return out


def cmd_images(tokens, depth):
    """Image operands of every docker|podman|nerdctl run|pull|create (and `buildah from`) in one simple command, looking through
    wrapper words, global options, `image`/`container`, shell -c / ssh / eval strings and $(...) substitutions."""
    res = []
    if depth < 4:
        for t, q in tokens:
            if q and ("$(" in t or "`" in t):
                for inner in subst_texts(t):
                    res += text_images(inner, depth + 1)
    h = head_index(tokens)
    if h is None:
        return res
    base = os.path.basename(tokens[h][0])
    rest = tokens[h + 1:]
    if base in ENGINES or base == "buildah":
        verbs = {"from", "pull"} if base == "buildah" else ENGINE_VERBS
        j = 0
        while j < len(rest) and not rest[j][1] and rest[j][0].startswith("-"):
            j += 2 if ("=" not in rest[j][0] and rest[j][0] in GLOBAL_VALUE_OPTS) else 1
        if j < len(rest) and rest[j][0] in ("container", "image"):
            j += 1
        if j < len(rest) and rest[j][0] in verbs:
            op = image_operand(rest[j + 1:])
            if op:
                res.append(op)
    elif base in SHELLISH and depth < 4:
        for t, q in rest:
            if q and re.search(r"\s", t):
                res += text_images(t, depth + 1)
    return res


def text_images(text, depth=0):
    res = []
    for s, e, tokens in split_commands(text):
        res += cmd_images(tokens, depth)
    return res


def mask_data(ln):
    """Blank out the simple commands that only write or filter data (printf, echo, sed, tee, grep, awk, a bare assignment)."""
    chars = list(ln)
    for s, e, tokens in split_commands(ln):
        h = head_index(tokens)
        if h is None or os.path.basename(tokens[h][0]) in DATA_CMDS:
            for k in range(s, min(e, len(chars))):
                chars[k] = " "
    return "".join(chars)


def has_build_sibling(lines, idx, col):
    def ind(l):
        return len(l) - len(l.lstrip(" "))
    for rng in (range(idx - 1, -1, -1), range(idx + 1, len(lines))):
        for k in rng:
            l = strip_comment(lines[k].rstrip("\r"))
            if not l.strip():
                continue
            if ind(l) < col:
                break
            if ind(l) == col and re.match(r"^\s*build\s*:", l):
                return True
    return False


def scan_compose(path, text):
    lines = text.split("\n")
    for idx, raw in enumerate(lines):
        line = strip_comment(raw.rstrip("\r"))
        m = re.match(r"^(\s*(?:-\s*)?)image:\s*(.*?)\s*$", line)
        if not m:
            continue
        val = m.group(2)
        if val == "":                       # the value is on a following line
            for nxt in lines[idx + 1:]:
                c = strip_comment(nxt.rstrip("\r")).strip()
                if c:
                    val = c
                    break
            if val == "":
                continue
        ref = val.strip("\"'")
        if re.match(r"^\$[A-Za-z_][A-Za-z0-9_]*$", ref):
            continue                        # a bare $VAR is resolved by the caller
        v = re.match(r"^\$\{[A-Za-z_][A-Za-z0-9_]*(?::?-(.*))?\}$", ref)
        if v:
            if v.group(1) is None:
                continue
            ref = v.group(1)
        if ref.startswith("localhost/"):
            continue
        first = ref.split("/")[0]
        qualified = "/" in ref and ("." in first or ":" in first)
        if not qualified and has_build_sibling(lines, idx, len(m.group(1))):   # the tag a build: service produces is a local image
            continue
        if not pinned(ref):  # MUT-ANCHOR compose-check
            add(path, idx + 1, "compose_image_unpinned", ref)


def judge_source(path, no, src, aliases):
    src = src.strip("\"'")
    if src.lower() in aliases or src.isdigit() or src.startswith("$") or src.startswith("localhost/"):
        return
    if not pinned(src):  # MUT-ANCHOR copyfrom-check
        add(path, no, "copy_from_unpinned", src)


def scan_dockerfile(path, text):
    aliases = set()
    args = {}
    seen_from = False
    for no, ln in logical_lines(text, False):
        am = re.match(r"^\s*ARG\s+(.*?)\s*$", ln, re.I)
        if am:
            if not seen_from:               # an ARG after the first FROM is stage-local and never feeds a later FROM
                for tok in am.group(1).split():
                    name, eq, val = tok.partition("=")
                    if re.match(r"^[A-Za-z_][A-Za-z0-9_]*$", name):
                        args[name] = val.strip("\"'") if eq else None
            continue
        fm = re.match(r"^\s*FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?", ln, re.I)
        if fm:
            seen_from = True
            ref, alias = fm.group(1), fm.group(2)
            is_alias = ref.lower() in aliases
            if alias:
                aliases.add(alias.lower())
            if is_alias:
                continue  # MUT-ANCHOR alias-skip
            if ref.lower() == "scratch" or ref.startswith("localhost/"):
                continue
            vm = re.match(r"^\$\{?([A-Za-z_][A-Za-z0-9_]*)(?::?-([^}]*))?\}?$", ref)
            if vm:
                default = vm.group(2) if vm.group(2) is not None else args.get(vm.group(1))
                if default is None or default == "":
                    continue
                if not pinned(default):  # MUT-ANCHOR arg-check
                    add(path, no, "from_arg_unpinned", default)
                continue
            if not pinned(ref):  # MUT-ANCHOR from-check
                add(path, no, "from_unpinned", ref)
            continue
        cm = re.match(r"^\s*(?:ONBUILD\s+)?(?:COPY|ADD)\b.*?--from=(\S+)", ln, re.I)
        if cm:
            judge_source(path, no, cm.group(1), aliases)
            continue
        if re.match(r"^\s*(?:ONBUILD\s+)?RUN\b", ln, re.I):
            for mt in re.finditer(r"--mount=(\S+)", ln):
                mf = re.search(r"(?:^|,)from=([^,\s]+)", mt.group(1))
                if mf:
                    judge_source(path, no, mf.group(1), aliases)


def scan_pipe(path, no, ln):
    m = PIPE_RE.search(ln)  # MUT-ANCHOR pipe-check
    s = SUBST_RE.search(ln)
    hit = m or s
    if hit:
        add(path, no, "pipe_to_shell", hit.group(0))


def scan_script(path, text, is_test):
    for no, ln in logical_lines(text, is_test):
        work = mask_data(ln) if is_test else ln
        seen = set()
        for op in text_images(work):
            if op.startswith("$") or op.startswith("localhost/") or not IMG_SHAPE.match(op):
                continue
            seen.add(op)
            if not pinned(op):  # MUT-ANCHOR script-run-check
                add(path, no, "script_image_unpinned", op)
        wreg = work.replace("docker://", " ")
        # in a test script a registry-qualified literal outside a real engine command is fixture data (an argument of a helper, a table row)
        for g in ([] if is_test else REGISTRY_RE.finditer(wreg)):  # MUT-ANCHOR registry-literal-rule
            ref = g.group(1)
            if ref in seen:
                continue
            if "@sha256:" in ref:
                if ref.endswith("@sha256:") and wreg[g.end():g.end() + 1] == "$":
                    continue  # the digest is produced by a command substitution at run time: not a literal reference
                if not pinned(ref):
                    add(path, no, "script_image_unpinned", ref)
                continue
            if re.search(r":[A-Za-z0-9._-]+$", ref):  # a registry reference with a tag and no digest
                add(path, no, "script_image_unpinned", ref)
        scan_pipe(path, no, work)


def classify(rel):
    b = os.path.basename(rel)
    low = b.lower()
    if low.endswith((".md", ".txt", ".html", ".json", ".tsv", ".lock")):
        return None
    if re.match(r"^(dockerfile|containerfile)([._-].*)?$", low) or re.search(r"\.(dockerfile|containerfile)$", low):
        return "dockerfile"
    if "compose" in low and low.endswith((".yml", ".yaml")):
        return "compose"
    if low.endswith((".sh", ".bash")):
        return "script"
    return None


def main(argv):
    global files_scanned
    root = None
    lst = False
    recurse = False
    excl = set(SKIP_DIRS)
    paths = []
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--root":
            if i + 1 >= len(argv):
                usage("--root requires a value")
            root = argv[i + 1]
            i += 2
        elif a == "--list":
            lst = True
            i += 1
        elif a == "--recurse":
            recurse = True
            i += 1
        elif a == "--exclude-dir":
            if i + 1 >= len(argv):
                usage("--exclude-dir requires a value")
            excl.add(argv[i + 1])
            i += 2
        elif a in ("-h", "--help"):
            sys.stdout.write("see the header of check_pins.sh and docs/scripts/check_pins.md\n")
            sys.exit(0)
        elif a.startswith("-"):
            usage("unknown option '%s'" % a)
        else:
            paths.append(a)
            i += 1
    if root is None:
        try:
            root = subprocess.run(["git", "rev-parse", "--show-toplevel"], capture_output=True, text=True, check=True).stdout.strip()
        except Exception:
            root = os.getcwd()
    if not os.path.isdir(root):
        usage("root '%s' is not a directory" % root)
    root = os.path.realpath(root)
    cand = []
    if paths:
        for p in paths:
            full = os.path.join(root, p)
            if os.path.isdir(full):
                for dp, dns, fns in os.walk(full):
                    dns[:] = sorted(d for d in dns if d not in excl)
                    for f in sorted(fns):
                        cand.append(os.path.relpath(os.path.join(dp, f), root))
            elif os.path.isfile(full):
                cand.append(os.path.relpath(full, root))
            else:
                usage("path '%s' does not exist under the root" % p)
    elif os.path.exists(os.path.join(root, ".git")):
        cmd = ["git", "-C", root, "ls-files", "-z"] + (["--recurse-submodules"] if recurse else [])
        r = subprocess.run(cmd, capture_output=True)
        if r.returncode != 0:
            usage("git ls-files failed in %s" % root)
        cand = [x for x in r.stdout.decode("utf-8", "replace").split("\0") if x]
    else:
        for dp, dns, fns in os.walk(root):
            dns[:] = sorted(d for d in dns if d not in excl)
            for f in sorted(fns):
                cand.append(os.path.relpath(os.path.join(dp, f), root))
    for rel in sorted(set(cand)):
        parts = rel.split("/")
        if any(p in excl for p in parts[:-1]):
            continue
        kind = classify(rel)
        if kind is None:
            continue
        full = os.path.join(root, rel)
        if not os.path.isfile(full) or os.path.islink(full):
            continue
        try:
            text = open(full, encoding="utf-8-sig", errors="replace").read()
        except OSError:
            continue
        files_scanned += 1
        if kind == "compose":  # MUT-ANCHOR compose-dispatch
            scan_compose(rel, text)
        elif kind == "dockerfile":
            scan_dockerfile(rel, text)
            for no, ln in logical_lines(text, False):
                scan_pipe(rel, no, ln)
        else:
            base = os.path.basename(rel)
            is_test = "tests" in parts[:-1] or base.startswith(("test_", "mutate_"))
            scan_script(rel, text, is_test)
    viol.sort(key=lambda v: (v[0], v[1], v[2], v[3]))  # MUT-ANCHOR output-sort
    uniq = []
    for v in viol:
        if not uniq or uniq[-1] != v:
            uniq.append(v)
    if lst:
        for p, ln, rule, ref in uniq:
            sys.stdout.write("%s\t%s\t%d\t%s\n" % (rule, p, ln, ref))
    else:
        for p, ln, rule, ref in uniq:
            sys.stdout.write("VIOLATION %s %s:%d: %s\n" % (rule, p, ln, ref))
        sys.stdout.write("check_pins: %d violations in %d files scanned\n" % (len(uniq), files_scanned))
    sys.exit(1 if uniq else 0)


try:
    main(sys.argv[1:])
except Exception as exc:  # exit 3: an internal error is neither "clean" (0) nor "violations" (1); nothing was printed to stdout
    import traceback
    traceback.print_exc()
    sys.stderr.write("check_pins: internal error: %s: %s\n" % (type(exc).__name__, exc))
    sys.exit(3)  # MUT-ANCHOR crash-exit
PY
