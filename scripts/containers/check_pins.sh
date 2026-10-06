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
PIPE_RE = re.compile(
    r"\b(?:curl|wget|fetch)\b[^\n]*?(?<!\|)\|(?!\|)\s*(?:sudo\s+(?:-\S+\s+)*)?(?:env\s+\S+=\S+\s+)*(?:/usr/bin/|/bin/)?(?:ba|z|da)?sh\b")
SUBST_RE = re.compile(
    r"(?:\b(?:ba|z|da)?sh\s+(?:-c\s+)?[\"']?\$\(\s*(?:curl|wget|fetch)\b[^\n]*|\b(?:ba|z|da)?sh\s+<\(\s*(?:curl|wget|fetch)\b[^\n]*)")
VALUE_OPTS = {"-v", "--volume", "-e", "--env", "-p", "--publish", "-w", "--workdir", "--name", "-u", "--user", "--network", "--net",
              "--entrypoint", "--memory", "-m", "--cpus", "--pids-limit", "--userns", "--security-opt", "--cap-add", "--cap-drop",
              "--device", "--label", "-l", "--env-file", "--tmpfs", "--mount", "--platform", "--pull", "--restart", "--hostname",
              "-h", "--group-add", "--ulimit", "--memory-swap", "--shm-size", "--log-driver", "--cidfile", "--pid", "--ipc"}
RUNVERB_RE = re.compile(r"(?:^|[\s;&|(])(?:sudo\s+)?(?:docker|podman|nerdctl)\s+(?:container\s+|image\s+)?(run|pull|create)\b(.*)$")
HEREDOC_RE = re.compile(r"(?<!<)<<(?!<)-?\s*(['\"]?)(\w+)\1")

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


def logical_lines(text, skip_heredocs):
    """Yield (first_physical_line_no, text) with continuation lines joined and comments removed."""
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
        cont = line.rstrip().endswith("\\")
        body = strip_comment(line)
        body = body.rstrip()
        if body.endswith("\\"):
            body = body[:-1]
        if acc is None:
            acc, start = body, no
        else:
            acc += " " + body.strip()
        if not cont:
            out.append((start, acc))
            if skip_heredocs:
                m = HEREDOC_RE.search(acc)
                if m:
                    heredoc_end = m.group(2)
            acc = None
    if acc is not None:
        out.append((start, acc))
    return out


def add(path, line, rule, ref):
    viol.append((path, line, rule, " ".join(ref.split())[:200]))


def scan_compose(path, text):
    for no, raw in enumerate(text.split("\n"), 1):
        line = strip_comment(raw.rstrip("\r"))
        m = re.match(r"^\s*-?\s*image:\s*(\S.*?)\s*$", line)
        if not m:
            continue
        ref = m.group(1).strip("\"'")
        v = re.match(r"^\$\{[A-Za-z_][A-Za-z0-9_]*(?::?-(.*))?\}$", ref)
        if v:
            if v.group(1) is None:
                continue
            ref = v.group(1)
        if ref.startswith("localhost/"):
            continue
        if not pinned(ref):  # MUT-ANCHOR compose-check
            add(path, no, "compose_image_unpinned", ref)


def scan_dockerfile(path, text):
    aliases = set()
    args = {}
    for no, ln in logical_lines(text, False):
        am = re.match(r"^\s*ARG\s+([A-Za-z_][A-Za-z0-9_]*)(?:=(.*))?\s*$", ln, re.I)
        if am:
            args[am.group(1)] = am.group(2).strip().strip("\"'") if am.group(2) is not None else None
            continue
        fm = re.match(r"^\s*FROM\s+(?:--\S+\s+)*(\S+)(?:\s+AS\s+(\S+))?", ln, re.I)
        if fm:
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
        cm = re.search(r"\bCOPY\b.*?--from=(\S+)", ln, re.I)
        if cm:
            src = cm.group(1).strip("\"'")
            if src.lower() in aliases or src.isdigit() or src.startswith("$") or src.startswith("localhost/"):
                continue
            if not pinned(src):  # MUT-ANCHOR copyfrom-check
                add(path, no, "copy_from_unpinned", src)


def image_operand(rest):
    toks = rest.split()
    i = 0
    while i < len(toks):
        t = toks[i]
        if t == "--":
            i += 1
            continue
        if t.startswith("-"):
            if "=" not in t and t in VALUE_OPTS:
                i += 2
            else:
                i += 1
            continue
        return t.strip("\"'")
    return None


def scan_pipe(path, no, ln):
    m = PIPE_RE.search(ln)  # MUT-ANCHOR pipe-check
    s = SUBST_RE.search(ln)
    hit = m or s
    if hit:
        add(path, no, "pipe_to_shell", hit.group(0))


def scan_script(path, text, is_test):
    for no, ln in logical_lines(text, is_test):
        seen = set()
        rm = RUNVERB_RE.search(ln)
        if rm:
            op = image_operand(rm.group(2))
            if op and not op.startswith("$") and not op.startswith("-") and not op.startswith("(") and re.match(r"^[a-z0-9][A-Za-z0-9._/-]*(?::[A-Za-z0-9._-]+)?(?:@sha256:[0-9A-Za-z]*)?$", op) and ("/" in op or ":" in op or "@" in op):
                seen.add(op)
                if not pinned(op):  # MUT-ANCHOR script-run-check
                    add(path, no, "script_image_unpinned", op)
        for g in REGISTRY_RE.finditer(ln):
            ref = g.group(1)
            if ref in seen:
                continue
            if "@sha256:" in ref:
                if ref.endswith("@sha256:") and ln[g.end():g.end() + 1] == "$":
                    continue  # the digest is produced by a command substitution at run time: not a literal reference
                if not pinned(ref):
                    add(path, no, "script_image_unpinned", ref)
                continue
            if re.search(r":[A-Za-z0-9._-]+$", ref):  # a registry reference with a tag and no digest
                add(path, no, "script_image_unpinned", ref)
        scan_pipe(path, no, ln)


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
            text = open(full, encoding="utf-8", errors="replace").read()
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
    viol.sort(key=lambda v: (v[0], v[1], v[2], v[3]))
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


main(sys.argv[1:])
PY
