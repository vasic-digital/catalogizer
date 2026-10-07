#!/usr/bin/env bash
# check_pins.sh - T105 (docs/16 section 7.1 step 3, finding D-07, D-05; constitution 11.4.246/11.4.264 digest identity, 11.4.173).
# Fails when a compose file, a Dockerfile/Containerfile or a shell script references an EXTERNAL image without a full
# `@sha256:<64 lowercase hex>` digest, or installs software by piping a download into a shell.
# Revision 4 (WF16 round 4, 2026-10-07, constitution 11.4.276, the round after the structural round): ONE shell grammar (compound commands tracked on
# the raw words, wrapper tables, shell and interpreter reader grammar, loaded by the Containerfile checker from this very file), BOTH halves of the
# engine help text (value and boolean options, pflag short clusters, `--`, array expansions), every expansion operator nested, every unreadable
# input shape is exit 3, here-document delimiter words, compose files read structurally; the rule text and the class tables are
# docs/scripts/check_pins.md (Round 3 and Round 4 classes).
#
# Usage: check_pins.sh [--root DIR] [--list] [--recurse] [--exclude-dir NAME]... [PATH...]
#        check_pins.sh --dump-engine-options     print `table<TAB>option<TAB>value|bool` of the option tables (the test compares them with engine_options.tsv)
#        check_pins.sh --dump-grammar            print the shell-grammar tables (wrapper, wrapopt, shell, interp, opener, closer, dlword, stdinpath)
#   --root DIR          directory the reported paths are relative to (default: the git top level of the cwd, else the cwd)
#   PATH...             files or directories (relative to the root) to scan; default: every tracked file of the root
#                       (`git ls-files`; with --recurse also the files of every submodule checkout, `--recurse-submodules`),
#                       or every file under the root when it is not a git work tree. Directories .git, node_modules, vendor and
#                       .audit are never entered.
#   --list              print one TSV row per violation `rule<TAB>path<TAB>line<TAB>reference` and nothing else (the baseline form)
#   --exclude-dir NAME  skip directories with this exact name (repeatable)
# Exit: 0 no violation; 1 at least one violation; 2 usage error (unknown option, root missing or not a directory);
#       3 internal error (an uncaught exception, or an input file that cannot be read: nothing on stdout, a traceback and
#       `check_pins: internal error` / `check_pins: cannot read <path>` on stderr). A caller comparing rows with a baseline must treat 3 as a
#       failed check, never as zero rows.
#
# Rules (the rule id is the second word of every `VIOLATION <rule> <path>:<line>: <reference or excerpt>` line):
#   compose_image_unpinned  `image:` of a compose file (a name containing `compose`, or any .yml/.yaml with a top-level `services:` key)
#                           without a full digest, read structurally when PyYAML parses the file (quoted keys, flow mappings), else with the
#                           text grammar (CHECK_PINS_NO_YAML=1 forces it); the value may be on the next line. `${VAR:-default}` (every
#                           operator, nested) is judged by its innermost default; a bare `${VAR}` / `$VAR`
#                           is resolved by the caller and not judged; `localhost/...` images are locally built, not external, and are
#                           skipped; with a `build:` key in the same service the image is local only with `pull_policy: never|build`, or
#                           when it is a single-component name and no pull_policy is given (compose pulls first otherwise).
#   from_unpinned           `FROM` of a Dockerfile/Containerfile naming an external image without a full digest (tag-only and
#                           no-tag references alike; build-stage aliases, `scratch` and `localhost/` bases are not external).
#   from_arg_unpinned       `FROM ${ARG}` / `${ARG:-default}` whose default is present and carries no full digest (an `ARG` without a
#                           default is supplied by the builder and is not judged here). Only ARG lines before the first FROM are global
#                           (a stage-local `ARG X` redeclaration never hides the global default); several NAME=value per ARG line count.
#   copy_from_unpinned      `COPY --from=` / `ADD --from=` / `RUN --mount=...,from=` naming an external image without a full digest (a
#                           stage alias or stage number is not an image).
#   script_image_unpinned   a shell script naming an external image without a digest: the image operand (a bare name counts: implicit
#                           :latest) of `docker|podman|nerdctl run|pull|create` and `buildah from|pull`, the engine word found at ANY
#                           position of every simple command of a line (prefix variables, helpers, wrappers with options, global options,
#                           `image`/`container`, `sh -c`/`ssh`/`eval` strings, `$(...)`, engines kept in a variable such as $DOCKER),
#                           or any registry-qualified reference carrying a tag (docker.io, ghcr.io, quay.io, mcr.microsoft.com, gcr.io,
#                           lscr.io, registry.*, *.pkg.dev, public.ecr.aws; a `docker://` prefix is removed first). `localhost/` and
#                           `$VAR` operands are not judged; option values are skipped (value AND boolean option tables, short-flag clusters,
#                           `--`, array expansions; see the guide).
#   pipe_to_shell           a download (`curl`, `wget`, `fetch`, or a variable that holds one) in a pipeline that ends in a program reader: a
#                           shell (`sh bash zsh dash ash ksh mksh csh tcsh fish rbash`, behind sudo/doas/env/nice/busybox with their options,
#                           by path too) with `-s`, `-`, or neither `-c` nor a script operand, an interpreter with no script and no inline
#                           code (`python3`, `python3 -W ignore`, `perl`, `node -r x`, `ruby`, `php`, ...), `source`/`.` of /dev/stdin, the
#                           `bash <(curl ...)`, `source <(curl ...)`, `. <(curl ...)`, `bash <<< "$(curl ...)"`, `sh -c "$(curl ...)"` and
#                           `eval "$(curl ...)"` forms, in a Dockerfile/Containerfile or a script; the download may be inside a compound
#                           command piped as a whole ((..), { ..; }, for/select/while/until, if, case); a pipe continued by a trailing `|`
#                           or a backslash, and a pipe after a `for ... done` loop spread over continuation lines, is one logical line and
#                           is caught; the quoted
#                           install hint a script prints for the operator is flagged too (it instructs an unverified install).
#                           Reported at the first physical line of the logical line.
# Carriers do NOT fire (11.4.201: a mention is not the thing): full-line and trailing `#` comments are removed before judging, and
# Markdown, text and every other file type is not scanned at all. In a script under a `tests` directory (only there), here-document bodies
# and quoted strings handed to helpers are fixture DATA and are not judged, nor are the simple commands that only write or filter data
# (printf, echo, sed, tee, grep, awk, a bare VAR=value) or a registry literal outside a real engine command (a violation fixture written by
# a test is not a violation of the test); in any other script a here-document body is judged like code (a script that generates a
# Dockerfile is scanned).
# Determinism: output is sorted by path, line, rule; two runs over the same tree print the same bytes.
# Honest boundary: the scan reads text; it does not resolve a registry, does not expand variables other than a `${VAR:-default}`
# default, and does not prove a digest exists or is signed (T149). Documented in docs/scripts/check_pins.md.
set -u
exec python3 -I - "$@" <<'PY'
import os, re, stat, subprocess, sys
try:
    import yaml                          # compose files are read structurally when PyYAML is there and the file parses (CHECK_PINS_NO_YAML=1 forces the text grammar)
except Exception:
    yaml = None

DIGEST_RE = re.compile(r"@sha256:[0-9a-f]{64}(?![0-9A-Za-z])")
SKIP_DIRS = {".git", "node_modules", "vendor", ".audit"}
REGISTRY_RE = re.compile(
    r"(?<![\w./-])((?:index\.docker\.io|registry-1\.docker\.io|docker\.io|ghcr\.io|quay\.io|mcr\.microsoft\.com|(?:[a-z0-9-]+\.)?gcr\.io|lscr\.io"
    r"|nvcr\.io|registry\.[\w.-]+|[\w-]+\.pkg\.dev|public\.ecr\.aws)"
    r"/[A-Za-z0-9._/-]+(?::[A-Za-z0-9._-]+)?(?:@sha256:[0-9A-Za-z]*)?)")
# options that take a SEPARATE value, per table; generated from the engine help text (podman 5.7.0 parsed, docker from its CLI reference) into
# scripts/containers/tests/engine_options.tsv by scripts/containers/tests/gen_engine_options.sh, and checked equal to it by test_check_pins.sh
# (run = run|create|buildah from, pull = pull, global = the options before the sub-command). nerdctl/buildah are UNCONFIRMED (not installed).
RUN_VALUE_OPTS = {
    "--add-host", "--annotation", "--arch", "--attach", "--authfile", "--blkio-weight", "--blkio-weight-device", "--cap-add", "--cap-drop",
    "--cert-dir", "--cgroup-conf", "--cgroup-parent", "--cgroupns", "--cgroups", "--chrootdirs", "--cidfile", "--conmon-pidfile",
    "--cpu-count", "--cpu-percent", "--cpu-period", "--cpu-quota", "--cpu-rt-period", "--cpu-rt-runtime", "--cpu-shares", "--cpus",
    "--cpuset-cpus", "--cpuset-mems", "--creds", "--decryption-key", "--detach-keys", "--device", "--device-cgroup-rule",
    "--device-read-bps", "--device-read-iops", "--device-write-bps", "--device-write-iops", "--dns", "--dns-option", "--dns-search",
    "--domainname", "--entrypoint", "--env", "--env-file", "--env-merge", "--expose", "--gidmap", "--gpus", "--group-add", "--group-entry",
    "--health-cmd", "--health-interval", "--health-log-destination", "--health-max-log-count", "--health-max-log-size",
    "--health-on-failure", "--health-retries", "--health-start-interval", "--health-start-period", "--health-startup-cmd",
    "--health-startup-interval", "--health-startup-retries", "--health-startup-success", "--health-startup-timeout", "--health-timeout",
    "--hostname", "--hosts-file", "--hostuser", "--image-volume", "--init-ctr", "--init-path", "--io-maxbandwidth", "--io-maxiops", "--ip",
    "--ip6", "--ipc", "--isolation", "--label", "--label-file", "--link", "--link-local-ip", "--log-driver", "--log-opt", "--mac-address",
    "--memory", "--memory-reservation", "--memory-swap", "--memory-swappiness", "--mount", "--name", "--network", "--network-alias",
    "--oom-score-adj", "--os", "--passwd-entry", "--personality", "--pid", "--pidfile", "--pids-limit", "--platform", "--pod",
    "--pod-id-file", "--preserve-fd", "--preserve-fds", "--publish", "--pull", "--rdt-class", "--requires", "--restart", "--retry",
    "--retry-delay", "--runtime", "--sdnotify", "--seccomp-policy", "--secret", "--security-opt", "--shm-size", "--shm-size-systemd",
    "--stop-signal", "--stop-timeout", "--storage-opt", "--subgidname", "--subuidname", "--sysctl", "--systemd", "--timeout", "--tmpfs",
    "--tz", "--uidmap", "--ulimit", "--umask", "--unsetenv", "--user", "--userns", "--uts", "--variant", "--volume", "--volume-driver",
    "--volumes-from", "--workdir", "-a", "-c", "-e", "-h", "-l", "-m", "-p", "-u", "-v", "-w"}
PULL_VALUE_OPTS = {
    "--arch", "--authfile", "--cert-dir", "--creds", "--decryption-key", "--os", "--platform", "--policy", "--retry", "--retry-delay",
    "--variant"}
GLOBAL_VALUE_OPTS = {
    "--cdi-spec-dir", "--cgroup-manager", "--config", "--conmon", "--connection", "--context", "--events-backend", "--hooks-dir", "--host",
    "--identity", "--imagestore", "--log-level", "--module", "--network-cmd-path", "--network-config-dir", "--out", "--root", "--runroot",
    "--runtime", "--runtime-flag", "--ssh", "--storage-driver", "--storage-opt", "--tls-ca", "--tls-cert", "--tls-key", "--tlscacert",
    "--tlscert", "--tlskey", "--tmpdir", "--url", "--volumepath", "-H", "-c", "-l"}
RUN_BOOL_OPTS = {
    "--detach", "--disable-content-trust", "--env-host", "--help", "--http-proxy", "--init", "--interactive", "--no-healthcheck",
    "--no-hostname", "--no-hosts", "--oom-kill-disable", "--passwd", "--privileged", "--publish-all", "--quiet", "--read-only",
    "--read-only-tmpfs", "--replace", "--rm", "--rmi", "--rootfs", "--sig-proxy", "--tls-verify", "--tty", "--unsetenv-all",
    "--use-api-socket", "-P", "-d", "-i", "-q", "-t"}
PULL_BOOL_OPTS = {
    "--all-tags", "--disable-content-trust", "--quiet", "--tls-verify", "-a", "-q"}
GLOBAL_BOOL_OPTS = {
    "--debug", "--help", "--remote", "--syslog", "--tls", "--tlsverify", "--transient-store", "--version", "-D", "-r", "-v"}
ENGINES = {"docker", "podman", "nerdctl"}
ENGINE_VERBS = {"run", "pull", "create"}
# wrapper words that run the next word as a command; the value options of the ones that take them are skipped (a value is not the command)
WRAPPERS = {"sudo", "env", "time", "exec", "nohup", "command", "timeout", "nice", "ionice", "xargs", "stdbuf", "then", "do", "else", "elif",
            "if", "while", "until", "!", "{", "watch", "setsid", "builtin", "doas", "busybox"}
WRAP_VALUE_OPTS = {
    "sudo": {"-u", "-g", "-C", "-h", "-p", "-r", "-t", "-U", "-D", "-R", "-T", "--user", "--group", "--host", "--prompt", "--role", "--type",
             "--chdir", "--other-user", "--chroot", "--close-from", "--command-timeout"},
    "doas": {"-u", "-C"}, "env": {"-u", "--unset", "-C", "--chdir", "-S", "--split-string", "-a", "--argv0", "-f", "--file"},
    "nice": {"-n", "--adjustment"},
    "ionice": {"-c", "-n", "-p", "-P", "-u", "--class", "--classdata"}, "stdbuf": {"-i", "-o", "-e", "--input", "--output", "--error"},
    "timeout": {"-s", "--signal", "-k", "--kill-after"}}
SHELLISH = {"sh", "bash", "zsh", "dash", "ash", "ksh", "eval", "ssh"}
# in a test script these commands only WRITE or FILTER data: a fixture line they carry is not a command of the test
DATA_CMDS = {"printf", "echo", "sed", "tee", "grep", "egrep", "fgrep", "awk"}
# commands that only talk ABOUT a command (their words are prose or a lookup key), never run it
# (also the logging helpers every shell script defines: their unquoted words are a message, `warn podman pull failed for redis` names no image)
LOG_HELPERS = {"die", "warn", "warning", "err", "error", "fail", "fatal", "log", "info", "say", "msg", "note", "notice", "debug", "usage", "print",
               "logger", "log_info", "log_warn", "log_error", "log_err", "log_debug", "log_fatal"}
NON_EXEC_HEADS = DATA_CMDS | LOG_HELPERS | {"man", "which", "type", "whereis", "whatis", "info", "help", "apropos", "test", "[", "[[", "hash"}
# package managers: their words are package NAMES (`apt-get install curl` installs a package, it does not download with curl)
PKG_HEADS = {"apt", "apt-get", "aptitude", "apk", "yum", "dnf", "microdnf", "zypper", "pacman", "pip", "pip3", "npm", "yarn", "gem", "brew",
             "dpkg", "rpm", "update-alternatives", "dnf5", "tdnf", "emerge", "opkg"}
# a container engine kept in a variable: $DOCKER, ${PODMAN}, ${CONTAINER_ENGINE:-podman}, ...
ENGINE_VAR_RE = re.compile(r"^\$\{?(?:DOCKER|PODMAN|NERDCTL|CONTAINER_?ENGINE|CONTAINER_?RUNTIME|CTR_?ENGINE|OCI_?ENGINE|ENGINE)"
                           r"(?:_?BIN|_?CMD|_?PATH)?(?::?-[^}]*)?\}?$", re.I)
# a reader of a program from its standard input: shells (value options -o/-O/--rcfile/--init-file; -c = inline program, -s = read stdin) and
# interpreters (per interpreter: the letters of its inline-program flags, its options that take a separate value, its inline long options;
# ground truth: the live --help of python3, perl, ruby, node on this host; php, lua, deno, bun, Rscript are UNCONFIRMED, not installed)
SHELLS = {"sh", "bash", "zsh", "dash", "ash", "ksh", "mksh", "csh", "tcsh", "fish", "rbash"}
SHELL_VALUE_OPTS = {"-o", "+o", "-O", "+O", "--rcfile", "--init-file"}
INTERPS = {
    "python": ("cm", {"-W", "-X", "--check-hash-based-pycs"}, set()),
    "perl": ("eE", {"-I"}, set()),
    "ruby": ("e", {"-r", "-I", "-C", "-E", "-F"}, set()),
    "node": ("ep", {"-r", "--require", "--import", "--loader", "--experimental-loader", "-C", "--conditions"}, {"--eval", "--print"}),
    "php": ("rRBEf", {"-c", "-d", "-z"}, set()),
    "lua": ("e", {"-l"}, set()),
    "deno": ("e", set(), {"--eval"}),
    "bun": ("e", set(), {"--eval"}),
    "Rscript": ("e", set(), set()),
}
STDIN_PATHS = {"/dev/stdin", "/dev/fd/0", "/proc/self/fd/0"}
# compound commands of the shell grammar: the words that open one (`{` too) and the words that close one
OPENERS = {"for", "select", "case", "if", "while", "until", "{"}
CLOSERS = {"done", "esac", "fi", "}"}
DL_WORDS = {"curl", "wget", "fetch"}
# a downloader kept in a variable: $CURL, "${WGET}", ${DOWNLOADER:-curl}
DL_VAR_RE = re.compile(r"^\$\{?(?:CURL|WGET|FETCH|DOWNLOADER?)(?:_?BIN|_?CMD|_?PATH)?(?::?-[^}]*)?\}?$", re.I)
ARRAY_EXP_RE = re.compile(r"^\$(?:\{[A-Za-z_][A-Za-z0-9_]*\[[@*]\]\}|[@*])$")
EXP_OPEN_RE = re.compile(r"\$\{[A-Za-z_][A-Za-z0-9_]*:?[-=+?]")
EXP_DEFAULT_RE = re.compile(r"^\$\{[A-Za-z_][A-Za-z0-9_]*:?[-=+](.*)\}$", re.S)
# the programs that run a program read from text: shells, eval, interpreters; built from the sets above (one source of truth)
READER_NAMES = "(?:" + "|".join(sorted(SHELLS, key=lambda w: -len(w))) + r"|eval|python[0-9.]*|perl|ruby|node(?:js)?|php|lua|deno|bun|Rscript)"
DLX = r"(?:curl|wget|fetch|\$\{?(?:CURL|WGET|FETCH|DOWNLOADER?)\b)[^\n]*"
SUBST_RE = re.compile(
    # a command substitution as the program (also after -c / -e): sh -c "$(curl ...)", eval "$(curl ...)", python3 -c "`wget ...`"
    r"(?<![\w.-])" + READER_NAMES + r"(?![\w-])\s+(?:-\S+\s+)*[\"']?(?:\$\(|`)\s*" + DLX
    # a process substitution as the program file: bash <(curl ...), python3 <(curl ...), source <(curl ...), . <(curl ...), bash < <(curl ...)
    + r"|(?<![\w.-])(?:" + READER_NAMES + r"|source)(?![\w-])\s+(?:-\S+\s+)*(?:<\s*)?<\(\s*" + DLX
    + r"|(?<![\w.])\.\s+(?:<\s*)?<\(\s*" + DLX
    # a here-string: bash <<< "$(curl ...)", sh -s <<<$(wget ...)
    + r"|(?<![\w.-])" + READER_NAMES + r"(?![\w-])[^|;&\n]*?<<<\s*[\"']?(?:\$\(|`)\s*" + DLX)
ASSIGN_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*=")
DURATION_RE = re.compile(r"^\d+(?:\.\d+)?[smhd]?$")
IMG_SHAPE = re.compile(r"^[a-z0-9][A-Za-z0-9._-]*(?::[0-9]+)?(?:/[A-Za-z0-9._-]+)*(?::[A-Za-z0-9._-]+)?(?:@sha256:[0-9A-Za-z]*)?$")
# a here-document opener at the top level of a logical line: quoted strings and arithmetic `$((..))` / `((..))` are consumed first
# (the delimiter is ONE shell word: 'quoted', "quoted" (spaces allowed), \backslash-quoted, or bare up to a blank or an operator character)
HDOC_OPEN_RE = re.compile(r"""<<-?\s*(?:'([^']*)'|"([^"]*)"|\\?([^\s;&|()<>'"\\]+))""")
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


def skip_arith(s, i):
    """s[i] is the first `(` of a `((`; return the index after the matching `))` (parentheses nest), or len(s)."""
    depth = 0
    n = len(s)
    while i < n:
        if s[i] == "(":
            depth += 1
        elif s[i] == ")":
            depth -= 1
            if depth == 0:
                return i + 1
        i += 1
    return n


def top_level_heredoc(acc):
    """Delimiter of a here-document opened at the top level of one logical line (quoted strings, here-strings `<<<` and arithmetic
    `$(( ))` / `(( ))` with any nesting are consumed first); None when there is none."""
    i, n = 0, len(acc)
    while i < n:
        c = acc[i]
        if c == "'":
            j = acc.find("'", i + 1)
            i = n if j < 0 else j + 1
        elif c == '"':
            j = i + 1
            while j < n and acc[j] != '"':
                j += 2 if acc[j] == "\\" else 1
            i = j + 1
        elif c == "\\":
            i += 2
        elif c == "$" and acc.startswith("((", i + 1):
            i = skip_arith(acc, i + 1)
        elif c == "(" and acc.startswith("((", i) and (i == 0 or acc[i - 1] in " \t;&|`"):
            i = skip_arith(acc, i)
        elif acc.startswith("<<<", i):
            i += 3
        elif acc.startswith("<<", i):
            m = HDOC_OPEN_RE.match(acc, i)
            if m:
                return next(g for g in m.groups() if g is not None)
            i += 2
        else:
            i += 1
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
    """Index of the command word after wrappers (sudo, env, timeout 30, VAR=x, ...) and the value options of the wrappers that take one;
    None when there is none."""
    i = 0
    n = len(tokens)
    while i < n:
        t, q = tokens[i]
        if ASSIGN_RE.match(t):              # VAR=value (the value may have been quoted)
            i += 1
            continue
        if q:
            break
        base = os.path.basename(t)
        if base in WRAPPERS:
            i += 1
            vals = WRAP_VALUE_OPTS.get(base, ())
            while i < n and tokens[i][0].startswith("-") and tokens[i][0] != "-" and not tokens[i][1]:
                opt = tokens[i][0]
                i += 1
                if "=" not in opt and opt in vals and i < n:
                    i += 1
            continue
        if DURATION_RE.match(t):
            i += 1
            continue
        break
    return i if i < n else None


def looks_like_value(t):
    """A token that is the value of an unknown option, not an image (key=value, a port or address, host:ip, host:host-gateway)."""
    return ("=" in t or re.match(r"^\d+(?:\.\d+){0,3}(?::\d+)?$", t) is not None
            or re.match(r"^[A-Za-z0-9.-]+:(?:\d+\.){3}\d+$", t) is not None or re.match(r"^[A-Za-z0-9.-]+:host-gateway$", t) is not None)


def skip_option(tokens, i, vtab, btab):
    """The option token at tokens[i] -> (index of the next token, unknown?). A long option takes a separate value when it is in the value
    table (`--name=v` carries its own); a short-flag cluster is read left to right like pflag/getopt: boolean letters continue, the first
    value letter takes the REST of the cluster as its value (`-dp8080:80`) or the next token (`-dp 8080:80`, `-itw /src`)."""
    t = tokens[i][0]
    if "=" in t:
        return i + 1, False
    if t.startswith("--"):
        if t in vtab:
            return i + 2, False
        return i + 1, t not in btab
    for j in range(1, len(t)):
        o = "-" + t[j]
        if o in vtab:
            return (i + 1 if j < len(t) - 1 else i + 2), False
        if o not in btab:
            return i + 1, True
    return i + 1, False


def image_operand(tokens, vtab, btab):
    """The image operand of the tokens that follow `<engine> [globals] <verb>`: `vtab` = options that take a separate value, `btab` = boolean
    options (both halves of the engine help text); an UNKNOWN option (nerdctl, buildah, a new flag) is a boolean unless the next token cannot
    be an image operand; `--` ends the options (the next word is the operand); an array/positional expansion is an option list."""
    i = 0
    after_unknown = False
    n = len(tokens)
    while i < n:
        t, q = tokens[i]
        if t == "--" and not q:
            return tokens[i + 1][0] if i + 1 < n else None
        if not q and t.startswith("-") and len(t) > 1:
            i, after_unknown = skip_option(tokens, i, vtab, btab)
            continue
        if re.match(r"^\d*[<>]", t) and not q:
            i += 1
            continue
        if ARRAY_EXP_RE.match(t):
            i += 1
            after_unknown = False
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


def engine_word(t, q):
    """'engine' for docker|podman|nerdctl (by path too) or a variable that holds one, 'buildah' for buildah, else None."""
    base = os.path.basename(t)
    if base in ENGINES or ENGINE_VAR_RE.match(t):
        return "engine"
    if base == "buildah":
        return "buildah"
    return None


def engine_operand(kind, rest):
    """Image operand of `<engine> [global options] [image|container] run|pull|create|from ...` (rest = the tokens after the engine word), or None."""
    verbs = {"from", "pull"} if kind == "buildah" else ENGINE_VERBS
    j = 0
    while j < len(rest) and not rest[j][1] and rest[j][0].startswith("-") and rest[j][0] != "-":
        j = skip_option(rest, j, GLOBAL_VALUE_OPTS, GLOBAL_BOOL_OPTS)[0]
    if j < len(rest) and rest[j][0] in ("container", "image"):
        j += 1
    if j < len(rest) and rest[j][0] in verbs:
        if rest[j][0] == "pull":
            return image_operand(rest[j + 1:], PULL_VALUE_OPTS, PULL_BOOL_OPTS)
        return image_operand(rest[j + 1:], RUN_VALUE_OPTS, RUN_BOOL_OPTS)
    return None


def cmd_images(tokens, depth, strict=False):
    """Image operands of every `<engine> run|pull|create` (and `buildah from|pull`) in one simple command. The engine word is looked for
    at ANY position of the command (a prefix such as $SUDO, a helper function, ssh, eval, a wrapper with options all put words before it),
    except in a command that only talks about commands (echo, grep, man, ...); engines kept in a variable are recognised by name; $(...)
    substitutions and the quoted command line of a shell -c / ssh / eval are searched too. `strict` (a quoted string handed to an unknown
    helper) only trusts a string that STARTS with the engine and keeps only operands shaped like an image reference."""
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
    if base in NON_EXEC_HEADS:
        return res
    for k in range(h if strict else 0, len(tokens)):
        if strict and k != h:
            break
        kind = engine_word(*tokens[k])
        if kind is None:
            continue
        op = engine_operand(kind, tokens[k + 1:])
        if op is not None:
            if not strict or any(c in op for c in "/:."):
                res.append(op)
            break
    if depth < 4:
        for t, q in tokens[h + 1:]:
            if q and re.search(r"\s", t):
                res += text_images(t, depth + 1, strict=base not in SHELLISH)
    return res


def text_images(text, depth=0, strict=False):
    res = []
    for s, e, tokens in split_commands(text):
        res += cmd_images(tokens, depth, strict)
    return res


def mask_data(ln):
    """Blank out the simple commands that only write or filter data (printf, echo, sed, tee, grep, awk, a bare assignment)."""
    chars = list(ln)
    for s, e, tokens in split_commands(ln):
        h = head_index(tokens)
        if (h is None or os.path.basename(tokens[h][0]) in DATA_CMDS) and not any(q and ("$(" in t or "`" in t) for t, q in tokens):
            for k in range(s, min(e, len(chars))):
                chars[k] = " "
    return "".join(chars)


def sibling_value(lines, idx, col, key):
    """Value of the `key:` line of the same service (same indentation as the image: line, within the same mapping); None when absent."""
    def ind(l):
        return len(l) - len(l.lstrip(" "))
    rx = re.compile(r"^\s*%s\s*:\s*(.*?)\s*$" % key)
    for rng in (range(idx - 1, -1, -1), range(idx + 1, len(lines))):
        for k in rng:
            l = strip_comment(lines[k].rstrip("\r"))
            if not l.strip():
                continue
            if ind(l) < col:
                break
            if ind(l) == col:
                m = rx.match(l)
                if m:
                    return m.group(1).strip("\"'")
    return None


def compose_ref(val):
    """The reference an `image:` value stands for: None when it is a variable the caller resolves (`$VAR`, `${VAR}`, `${VAR:?msg}`) or a local image."""
    ref = operand_ref(val.strip("\"'")).strip("\"'")
    if ref.startswith("$") or ref.startswith("localhost/") or ref == "":
        return None
    return ref


def judge_compose_image(path, line, val, has_build, pol):
    ref = compose_ref(val)
    if ref is None:
        return
    if has_build:
        # a service with build: AND image: names the tag the build produces, but compose pulls first unless pull_policy says otherwise
        # (compose spec, build.md). Local only: pull_policy never|build (nothing is pulled), or a single-component name (no namespace, no
        # registry: not creatable by a user on a registry; the tag the build produces) with no explicit pull policy. Everything else
        # (a namespaced or registry-qualified name, or any other policy: always, missing, if_not_present, daily, weekly, every_*, a
        # variable) can be pulled from a registry and is judged. Residual (documented): a single-component name with no policy that
        # is also a real official image cannot be told from a local tag by its text.
        if pol in ("never", "build") or ("/" not in ref and pol is None):
            return
    if not pinned(ref):  # MUT-ANCHOR compose-check
        add(path, line, "compose_image_unpinned", ref)


def compose_walk(path, node, seen):
    """Structural walk of a composed YAML node graph: every mapping with an `image` key (any depth; quoted keys and flow mappings included)."""
    if id(node) in seen:
        return
    seen.add(id(node))
    if isinstance(node, yaml.MappingNode):
        items = [(k, v) for k, v in node.value if isinstance(k, yaml.ScalarNode)]
        sib = {k.value: v for k, v in items}
        for k, v in items:
            if k.value == "image" and isinstance(v, yaml.ScalarNode):
                pv = sib.get("pull_policy")
                judge_compose_image(path, k.start_mark.line + 1, v.value, "build" in sib, pv.value if isinstance(pv, yaml.ScalarNode) else None)
        for k, v in node.value:
            compose_walk(path, v, seen)
    elif isinstance(node, yaml.SequenceNode):
        for c in node.value:
            compose_walk(path, c, seen)


def scan_compose(path, text):
    if yaml is not None and not os.environ.get("CHECK_PINS_NO_YAML"):
        try:
            docs = list(yaml.compose_all(text))
        except Exception:
            docs = None                     # not YAML (a tab, a template): the text grammar below still judges it, never silently skips it
        if docs is not None:
            seen = set()
            for d in docs:
                if d is not None:
                    compose_walk(path, d, seen)
            return
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
        col = len(m.group(1))
        judge_compose_image(path, idx + 1, val, sibling_value(lines, idx, col, "build") is not None, sibling_value(lines, idx, col, "pull_policy"))


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


def interp_kind(base):
    """Canonical interpreter name of a command word (python3.12 -> python, nodejs -> node), else None."""
    if re.match(r"^python[0-9.]*$", base):
        return "python"
    if base == "nodejs":
        return "node"
    return base if base in INTERPS else None


def shell_args(args):
    """Parse the arguments of a shell -> (inline program given with -c?, read stdin with -s or `-`?, first operand index or None)."""
    c = sflag = False
    i, n = 0, len(args)
    while i < n:
        t, q = args[i]
        if q:
            break
        if t == "-":
            return c, True, None
        if t == "--":
            i += 1
            break
        if t[:1] in "-+" and len(t) > 1:
            if t.startswith("--"):
                i += 2 if (t in SHELL_VALUE_OPTS and "=" not in t) else 1
                continue
            letters = t[1:]
            if t[0] == "-" and "c" in letters:
                c = True
            if t[0] == "-" and "s" in letters:
                sflag = True
            i += 2 if any(ch in "oO" for ch in letters) else 1
            continue
        break
    return c, sflag, (i if i < n else None)


def shell_reads_stdin(args):
    """A shell reads its PROGRAM from stdin when -s is given, or when it has neither an inline program (-c) nor a script operand."""
    c, sflag, operand = shell_args(args)
    if sflag:
        return True
    return not c and operand is None


def interp_reads_stdin(kind, args):
    """An interpreter reads its program from stdin with `-` or when it has no inline-program flag and no script operand."""
    inline, vopts, ilong = INTERPS[kind]
    i, n = 0, len(args)
    while i < n:
        t, q = args[i]
        if q:
            return False
        if t == "-":
            return True
        if t == "--":
            return i + 1 >= n
        if t.startswith("--"):
            name = t.split("=", 1)[0]
            if name in ilong:
                return False
            i += 2 if (name in vopts and "=" not in t) else 1
            continue
        if t.startswith("-") and len(t) > 1:
            for j in range(1, len(t)):
                o = "-" + t[j]
                if o in vopts:
                    i += 1 if j < len(t) - 1 else 2
                    break
                if t[j] in inline:
                    return False
            else:
                i += 1
            continue
        return False                            # a script file operand: the program is the file, not stdin
    return True


def reader_hit(tokens):
    """True when this pipeline stage reads a PROGRAM from its stdin: a shell with no inline program and no script operand (or with -s / `-`),
    an interpreter with no inline program and no script, `source`/`.` of the standard input."""
    h = head_index(tokens)
    if h is None:
        return False
    base = os.path.basename(tokens[h][0])
    rest = tokens[h + 1:]
    if base in SHELLS:
        return shell_reads_stdin(rest)
    if base in ("source", "."):
        ops = [t for t, q in rest if not t.startswith("-")]
        return bool(ops) and ops[0] in STDIN_PATHS
    kind = interp_kind(base)
    return kind is not None and interp_reads_stdin(kind, rest)


def dl_token(t, q):
    """A downloader word (curl, wget, fetch) or a variable that holds one ($CURL, ${WGET:-wget})."""
    return (not q and os.path.basename(t) in DL_WORDS) or DL_VAR_RE.match(t) is not None


def pipe_hit(ln, depth=0):
    """Text of the first `<download> | <program reader>` pipeline in the logical line `ln`, else None. A pipeline is a run of commands
    joined by `|`; the download may be any earlier stage, or inside a compound command that is piped as a whole: (subshell), { group; },
    for/select/while/until loops, if/case. Depth is tracked on the RAW words: parentheses from the separators, keywords from the words that open
    (OPENERS, also behind then/do/else/{) and close (CLOSERS) a compound command, never from the command word after wrapper skipping."""
    ln = re.sub(r"^\s*(?:ONBUILD\s+)?RUN\s+(?:--\S+\s+)*", "", ln, flags=re.I)   # a Dockerfile RUN (also quoted into an echo): the shell command is what follows
    cmds = split_commands(ln)
    pd = kd = 0                                  # parenthesis depth, keyword depth
    prev_e = 0
    dl_start = None
    for s, e, toks in cmds:
        pre = ln[prev_e:s]                       # the separator run in front of this command: ; && || | |& ( ) newline
        prev_e = e
        pn = re.sub(r"\s", "", pre)
        piped = re.search(r"(?<!\|)\|&?\(*$", pn) is not None
        pd = max(0, pd - pn.count(")"))
        depth_before = pd + kd
        pd += pn.count("(")
        h = head_index(toks)
        prefix = toks if h is None else toks[:h + 1]
        kd += sum(1 for t, q in prefix if not q and t in OPENERS)
        if not piped and depth_before == 0:
            dl_start = None                      # a new pipeline outside any compound command forgets an earlier download
        if piped and dl_start is not None and reader_hit(toks):
            return ln[dl_start:e]
        if dl_start is None and any(dl_token(t, q) for t, q in toks):   # a download word anywhere (also an install hint)
            dl_start = s
        if depth < 3:
            for t, q in toks:
                if q and re.search(r"\s", t) and re.search(r"\b(?:curl|wget|fetch)\b", t):
                    inner = pipe_hit(t, depth + 1)
                    if inner:
                        return inner
        if toks and not toks[0][1] and toks[0][0] in CLOSERS:
            kd = max(0, kd - 1)
    return None


def scan_pipe(path, no, ln):
    hit = pipe_hit(ln)
    if hit:
        add(path, no, "pipe_to_shell", hit)
        return
    m = SUBST_RE.search(ln)
    if m:
        add(path, no, "pipe_to_shell", m.group(0))


def scan_script(path, text, is_test):
    for no, ln in logical_lines(text, is_test):
        work = blank_quoted(mask_data(ln)) if is_test else ln
        seen = set()
        for op in text_images(work):
            op = operand_ref(op)
            if op.startswith("$") or op.startswith("localhost/") or not IMG_SHAPE.match(op):
                continue
            seen.add(op)
            if not pinned(op):  # MUT-ANCHOR script-run-check
                add(path, no, "script_image_unpinned", op)
        # `${VAR:-ref}` / `${VAR=ref}`: the reference after the expansion operator is a reference (a space after the operator ends the `-` lookbehind)
        wreg = EXP_OPEN_RE.sub(lambda mo: mo.group(0) + " ", work.replace("docker://", " "))
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


def blank_quoted(ln):
    """In a test script a quoted string handed to a helper (or a here-string) is fixture data: blank single-quoted strings and double-quoted
    strings without a command substitution (their quotes stay, their content becomes spaces)."""
    out = list(ln)
    i, n = 0, len(ln)
    while i < n:
        c = ln[i]
        if c == "\\":
            i += 2
            continue
        if c in "'\"":
            j = i + 1
            while j < n and ln[j] != c:
                j += 2 if (c == '"' and ln[j] == "\\") else 1
            body = ln[i + 1:j]
            if c == "'" or ("$(" not in body and "`" not in body):
                for k in range(i + 1, min(j, n)):
                    out[k] = " "
            i = j + 1
            continue
        i += 1
    return "".join(out)


def operand_ref(op):
    """`${VAR:-default}`, `-`, `:=`, `=`, `:+alt`, `+alt` (nested too: `${A:-${B:-ref}}`) is judged by its innermost default / alternative (a bare
    `${VAR}` or `${VAR:?msg}` stays `$...` and is not judged)."""
    while True:
        m = EXP_DEFAULT_RE.match(op)
        if not m:
            return op
        op = m.group(1).strip("\"'")


def classify(rel):
    b = os.path.basename(rel)
    low = b.lower()
    if low.endswith((".md", ".txt", ".html", ".json", ".tsv", ".lock")):
        return None
    if re.match(r"^(dockerfile|containerfile)([._-].*)?$", low) or re.search(r"\.(dockerfile|containerfile)$", low):
        return "dockerfile"
    if low.endswith((".yml", ".yaml")):
        return "compose?" if "compose" not in low else "compose"
    if low.endswith((".sh", ".bash")):
        return "script"
    return None


def die_unreadable(rel, exc):
    sys.stderr.write("check_pins: cannot read %s: %s\n" % (rel, getattr(exc, "strerror", None) or exc))
    sys.exit(3)


def stat_of(full, rel, follow):
    """os.stat / os.lstat of a scan input: None when it is absent; ANY other failure (a permission, an I/O error) is exit 3, never a skip."""
    try:
        return os.stat(full) if follow else os.lstat(full)
    except (FileNotFoundError, NotADirectoryError):
        return None
    except OSError as exc:
        die_unreadable(rel, exc)


def walk_files(top, root, excl):
    """Every file below `top` (sorted, `excl` directories skipped); a directory that cannot be listed is exit 3."""
    def onerr(exc):
        die_unreadable(os.path.relpath(exc.filename, root) if exc.filename else top, exc)
    out = []
    for dp, dns, fns in os.walk(top, onerror=onerr):
        dns[:] = sorted(d for d in dns if d not in excl)
        for f in sorted(fns):
            out.append(os.path.relpath(os.path.join(dp, f), root))
    return out


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
        elif a == "--dump-engine-options":
            for tname, vt, bt in (("run", RUN_VALUE_OPTS, RUN_BOOL_OPTS), ("pull", PULL_VALUE_OPTS, PULL_BOOL_OPTS),
                                  ("global", GLOBAL_VALUE_OPTS, GLOBAL_BOOL_OPTS)):
                for o in sorted(vt):
                    sys.stdout.write("%s\t%s\tvalue\n" % (tname, o))
                for o in sorted(bt):
                    sys.stdout.write("%s\t%s\tbool\n" % (tname, o))
            sys.exit(0)
        elif a == "--dump-grammar":
            rows = [("wrapper", w) for w in WRAPPERS] + [("wrapopt", w, o) for w, os_ in WRAP_VALUE_OPTS.items() for o in os_]
            rows += [("shell", w) for w in SHELLS] + [("interp", k) for k in INTERPS] + [("opener", w) for w in OPENERS]
            rows += [("closer", w) for w in CLOSERS] + [("dlword", w) for w in DL_WORDS] + [("stdinpath", w) for w in STDIN_PATHS]
            for r in sorted(rows):
                sys.stdout.write("\t".join(r) + "\n")
            sys.exit(0)
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
            st = stat_of(full, p, True)
            if st is not None and stat.S_ISDIR(st.st_mode):
                cand += walk_files(full, root, excl)
            elif st is not None and stat.S_ISREG(st.st_mode):
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
        cand += walk_files(root, root, excl)
    for rel in sorted(set(cand)):
        parts = rel.split("/")
        if any(p in excl for p in parts[:-1]):
            continue
        kind = classify(rel)
        if kind is None:
            continue
        full = os.path.join(root, rel)
        st = stat_of(full, rel, False)
        if st is None or not stat.S_ISREG(st.st_mode):
            continue                        # absent (a deleted tracked file), a symlink, a submodule directory
        try:
            text = open(full, encoding="utf-8-sig", errors="replace").read()
        except OSError as exc:              # an input that cannot be read cannot be judged: never "0 violations"
            die_unreadable(rel, exc)
        if kind == "compose?":              # a YAML file with another name is a compose file when it has a top-level `services:` key
            if not re.search(r"^services\s*:", text, re.M):
                continue
            kind = "compose"
        files_scanned += 1
        if kind == "compose":  # MUT-ANCHOR compose-dispatch
            scan_compose(rel, text)
        elif kind == "dockerfile":
            scan_dockerfile(rel, text)
            for no, ln in logical_lines(text, False):
                scan_pipe(rel, no, ln)
        else:
            base = os.path.basename(rel)
            is_test = "tests" in parts[:-1]      # fixture data lives in a tests directory; a test_*-named script elsewhere is operational code
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
