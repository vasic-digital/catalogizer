#!/usr/bin/env python3
"""bashcov.py - T199. The analysis half of scripts/bash-coverage.sh: classifies the executable lines of a bash script, reads the PS4 trace and writes the
`bash-coverage/1` record. Subcommands:
  report --src-root DIR --trace FILE --out FILE --pre-sha FILE [--exclusions FILE] [--cwd DIR] --command STR --command-rc N --bash-version V -- TARGET...
  refuse --out FILE --status S --reason R --command STR [--command-rc N] [--bash-version V]     writes a record that carries NO figure (interrupted, trace_writers_outlived_command)
  classify FILE            prints the executable lines, the functions and the multi-line groups (a diagnostic of the classifier)

THE TRACE (WP-23 fix round 5, K3.1 K3.3 K4.4): every record is `+COV US <len> US <BASH_SOURCE> US <LINENO> US <$$> US <PWD> US <sha256 of the file> US <command>`
(US = the byte 0x1f). The path is LENGTH-PREFIXED, so a path that contains `:12:` or the delimiter itself cannot be split wrongly; a RELATIVE path is resolved against the
$PWD the traced shell had when it first reported that path, never against the directory the harness was started in; the sha256 is taken IN the run (a copy a test deletes
before the report keeps its identity).

THE CLASSIFIER (K4.1 K4.2 K4.3): a tokenizer that follows quote, `$( )`, `$(( ))`, backtick, `${ }`, `[[ ]]`, array and here-document state across physical lines. Bash reports
ONE line for a command that spans lines, and WHICH line depends on the construct (measured, bash 5.3.9: an assignment with a multi-line word at its LAST line, a simple command
with a multi-line argument at its FIRST line, `[[ ]]` at its first line, a continued command at its first line in 5.3 and its last in 5.2, and commands INSIDE a multi-line
command substitution at line numbers that are wrong). The classifier therefore makes every multi-line command ONE GROUP: the first line is the counted line, every other line of
the group is an alias of it, and a traced line anywhere in the group credits the group once. The lines of a multi-line quoted word, `$( )`, array and `[[ ]]` are never counted
on their own (they were, in round 1: 39 phantom lines in the committed build-scripts figure).

EXECUTABLE-LINE RULE (a documented HEURISTIC, 11.4.6, stated in every record): a line is executable when `bash -x` can print it, that is, it is not
  - blank, a comment, or a here-document body or its delimiter line;
  - a line inside a multi-line group (a multi-line word, command substitution, process substitution, array or `[[ ]]`) other than its first;
  - a continuation line (the line before ends with a backslash): the trace reports the FIRST line of a continued command - EXCEPT the continuation of a pipeline or an and/or
    list (the line before ends with `| \\`, `|| \\` or `&& \\`): each element is its own command and the trace reports its own line (measured: Build/lib/common.sh 178-179);
  - structural only: `fi`, `done`, `esac`, `else`, `then`, `do`, `{`, `}`, `)`, `;;` (optionally followed by a redirection or `;`); `done < <(cmd)` is NOT structural (the process
    substitution runs `cmd` there);
  - a function definition line (`name() {`, `name()`, `function name {`): the trace shows the calls, not the definition;
  - a case label alone (`pat)`); a label with a command after it (`1) echo one ;;`) is executable.
The classifier is checked against the tracer: a line the trace shows that the rule calls non-executable is a `traced_non_executable` line, and ANY such line makes the record
`classifier_disagreement` with NO figure and exit 3 (a classifier error is a refusal, never a number)."""
import hashlib, json, os, posixpath, re, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib

US = b"\x1f"
PS4 = '+COV\\037${#BASH_SOURCE}\\037${BASH_SOURCE}\\037${LINENO}\\037$$\\037${PWD}\\037${COV_H[${BASH_SOURCE:-/dev/null}]:=$(sha256sum <"${BASH_SOURCE:-/dev/null}" 2>/dev/null)}\\037'
TRACE_HEAD = re.compile(rb"^\++COV\x1f(\d+)\x1f")
FUNC_DEF_RE = re.compile(r"^(?:function\s+([A-Za-z_][A-Za-z0-9_:.-]*)\s*(?:\(\))?|([A-Za-z_][A-Za-z0-9_:.-]*)\s*\(\))\s*(\{)?\s*$")
STRUCT_RE = re.compile(r"^(fi|done|esac|else|then|do|\{|\}|\)|\(|;;&?|;&)(?![A-Za-z0-9_])\s*(.*)$")
ARM_RE = re.compile(r"^\(?[^\s()=;&|]+(?:\|[^\s()=;&|]+)*\)\s*(.*)$")
REDIR_HEAD_RE = re.compile(r"^\d*(?:>&|<&|&>>?|>>|<<<|<<-?|>\||>|<)\s*[^\s;&]*")
CMDSTART_KW = {"then", "do", "else", "elif", "if", "while", "until", "!", "time", "{"}
LIMITS = [
    "line, not branch: an if/else line counts covered when either side ran (the measured founding case of 11.4.224 C)",
    "regions under `set +x` and commands run by `sh` (dash) or by a process that unsets BASH_ENV are not traced and count as uncovered; the actions of a trap are traced, but bash reports them at LINENO 1 of whatever file is executing, so a LINENO 1 record is credited only when its command text appears in line 1 of the target",
    "the executable-line rule is a heuristic: structural lines, function definitions, here-document bodies, continuation lines and every line of a multi-line command after its first are not counted (bashcov.py docstring); a multi-line command counts once",
    "bash 5.3 reports commands INSIDE a multi-line command substitution or process substitution at line numbers that are wrong (measured: shifted past the construct): the whole construct counts as one line, and a wrong number that falls on a later line of the target can credit that line (an overcount of at most the lines that follow such a construct); a number past the end of the file is listed in `trace_out_of_range`, not credited",
    "a coverage percentage is a minimum on a proxy, necessary and never sufficient (11.4.224 C): it does not show that a test would catch a defect",
]


def is_excluded(rel, excl):
    for e in excl:
        p = e["path"]
        if fence_lib.matches(p, rel):
            return e
    return None


# --------------------------------------------------------------------------- the tokenizer
def _word_start(raw, j):
    return j == 0 or raw[j - 1] in " \t;&|(){}<>"


def scan(lines):
    """Returns (kinds, alias, clean, gend, balanced). kinds[i] in skip|hdoc|start|cont (1-based); alias[i] = counted line of a cont line (or of a here-document body line that
    holds a command substitution); clean[i] = the first physical line with quoted text, substitutions, arrays and braces of `${ }` replaced by placeholders; gend[i] = last line of
    the group that starts at i."""
    n = len(lines)
    kinds = [None] * (n + 2); alias = {}; clean = [""] * (n + 2); gend = {}
    stack = []
    hd_queue = []; hd_active = None; hd_owner = 0
    group = None; bs_cont = False; pipe_next = False
    for i in range(1, n + 1):
        raw = lines[i - 1]
        if hd_active is not None:
            delim, dash, quoted = hd_active
            body = raw.lstrip("\t") if dash else raw
            kinds[i] = "hdoc"
            if body == delim:
                hd_active = hd_queue.pop(0) if hd_queue else None
            elif not quoted and ("$(" in raw or "`" in raw):
                alias[i] = hd_owner
            continue
        starts_in_ctx = bool(stack) or (bs_cont and not pipe_next)
        if not starts_in_ctx:
            group = i
            if not raw.strip() or raw.lstrip().startswith("#"):
                kinds[i] = "skip"; bs_cont = False; pipe_next = False
                continue
            kinds[i] = "start"
        else:
            kinds[i] = "cont"; alias[i] = group
        bs_cont = False; pipe_next = False
        buf = []; j = 0; L = len(raw); trailing_bs = False; commented = False
        cmd_start = (not stack) or stack[-1]["k"] in ("cmdsub", "bt")
        word = ""
        new_hd = []

        def flush_word():
            nonlocal word, cmd_start
            if not word:
                return
            top = stack[-1] if stack else None
            if top is not None and top["k"] == "cmdsub":
                if word == "case" and cmd_start:
                    top["cases"] += 1; top["pat"] = False
                elif word == "in" and top["cases"] and top.get("await_in"):
                    top["pat"] = True; top["await_in"] = False
                elif word == "esac" and top["cases"]:
                    top["cases"] -= 1
                if word == "case" and cmd_start:
                    top["await_in"] = True
            cmd_start = word in CMDSTART_KW
            word = ""
        while j < L:
            c = raw[j]
            top = stack[-1] if stack else None
            k = top["k"] if top else None
            if k == "sq":
                if c == "'":
                    stack.pop()
                j += 1; continue
            if k == "ansic":
                if c == "\\":
                    j += 2; continue
                if c == "'":
                    stack.pop()
                j += 1; continue
            if k == "dq":
                if c == "\\":
                    j += 2; continue
                if c == '"':
                    stack.pop(); j += 1; continue
                if c == "$" and raw[j + 1:j + 2] == "(":
                    if raw[j + 2:j + 3] == "(":
                        stack.append({"k": "arith", "d": 0}); j += 3
                    else:
                        stack.append({"k": "cmdsub", "pd": 0, "cases": 0, "pat": False}); j += 2
                    continue
                if c == "$" and raw[j + 1:j + 2] == "{":
                    stack.append({"k": "brace"}); j += 2; continue
                if c == "`":
                    stack.append({"k": "bt"}); j += 1; continue
                j += 1; continue
            # ---- code-like states: normal, cmdsub, bt, array, brace, arith, dbracket
            at_depth0 = not stack
            if k == "arith":
                if c == "(":
                    top["d"] += 1
                elif c == ")":
                    if top["d"] > 0:
                        top["d"] -= 1
                    elif raw[j + 1:j + 2] == ")":
                        stack.pop(); j += 2; continue
                elif c == '"':
                    stack.append({"k": "dq"})
                elif c == "'":
                    stack.append({"k": "sq"})
                elif c == "$" and raw[j + 1:j + 2] == "(":
                    stack.append({"k": "cmdsub", "pd": 0, "cases": 0, "pat": False}); j += 2; continue
                j += 1; continue
            if c == "\\":
                if j + 1 >= L:
                    trailing_bs = True
                j += 2; continue
            if c == "'":
                flush_word()
                if at_depth0:
                    buf.append("\x01")
                stack.append({"k": "sq"}); j += 1; continue
            if c == '"':
                flush_word()
                if at_depth0:
                    buf.append("\x01")
                stack.append({"k": "dq"}); j += 1; continue
            if c == "$":
                nx = raw[j + 1:j + 2]
                if nx == "'":
                    if at_depth0:
                        buf.append("\x01")
                    stack.append({"k": "ansic"}); j += 2; continue
                if nx == '"':
                    if at_depth0:
                        buf.append("\x01")
                    stack.append({"k": "dq"}); j += 2; continue
                if nx == "(":
                    flush_word()
                    if at_depth0:
                        buf.append("\x02")
                    if raw[j + 2:j + 3] == "(":
                        stack.append({"k": "arith", "d": 0}); j += 3
                    else:
                        stack.append({"k": "cmdsub", "pd": 0, "cases": 0, "pat": False}); cmd_start = True; j += 2
                    continue
                if nx == "{":
                    flush_word()
                    if at_depth0:
                        buf.append("\x03")
                    stack.append({"k": "brace"}); j += 2; continue
                if at_depth0:
                    buf.append(c)
                j += 1; continue
            if c == "`":
                flush_word()
                if k == "bt":
                    stack.pop()
                else:
                    if at_depth0:
                        buf.append("\x02")
                    stack.append({"k": "bt"}); cmd_start = True
                j += 1; continue
            if k == "brace":
                if c == "}":
                    stack.pop()
                j += 1; continue
            if c == "#" and k != "dbracket" and _word_start(raw, j) and (j == 0 or raw[j - 1] != "$"):
                commented = True; break
            if (c in "<>") and raw[j + 1:j + 2] == "(" and k != "dbracket":
                flush_word()
                if at_depth0:
                    buf.append("\x02")
                stack.append({"k": "cmdsub", "pd": 0, "cases": 0, "pat": False}); cmd_start = True; j += 2; continue
            if c == "<" and raw[j:j + 3] == "<<<":
                if at_depth0:
                    buf.append("<<<")
                j += 3; continue
            if c == "<" and raw[j:j + 2] == "<<" and k not in ("dbracket",):
                m = j + 2; dash = False
                if raw[m:m + 1] == "-":
                    dash = True; m += 1
                while m < L and raw[m] in " \t":
                    m += 1
                dl = []; quoted = False
                while m < L and raw[m] not in " \t;&|<>()":
                    ch = raw[m]
                    if ch in "'\"":
                        quoted = True; q = ch; m += 1
                        while m < L and raw[m] != q:
                            dl.append(raw[m]); m += 1
                        m += 1; continue
                    if ch == "\\":
                        quoted = True; m += 1
                        if m < L:
                            dl.append(raw[m]); m += 1
                        continue
                    dl.append(ch); m += 1
                if dl:
                    new_hd.append(("".join(dl), dash, quoted))
                if at_depth0:
                    buf.append("<<")
                j = m; continue
            if c == "(" and raw[j + 1:j + 2] == "(" and k in (None, "cmdsub", "bt") and cmd_start:
                flush_word()
                if at_depth0:
                    buf.append("\x03")
                stack.append({"k": "arith", "d": 0}); j += 2; continue
            if c == "[" and raw[j + 1:j + 2] == "[" and k in (None, "cmdsub", "bt") and cmd_start and (j + 2 >= L or raw[j + 2] in " \t"):
                flush_word()
                if at_depth0:
                    buf.append("\x03")
                stack.append({"k": "dbracket"}); j += 2; continue
            if c == "]" and raw[j + 1:j + 2] == "]" and k == "dbracket" and (j == 0 or raw[j - 1] in " \t"):
                stack.pop(); cmd_start = False; j += 2; continue
            if c == "=" and raw[j + 1:j + 2] == "(" and j > 0 and (raw[j - 1].isalnum() or raw[j - 1] in "_]+") and k in (None, "cmdsub", "bt"):
                flush_word()
                if at_depth0:
                    buf.append("=\x03")
                stack.append({"k": "array"}); j += 2; continue
            if c == "(":
                flush_word()
                if k == "cmdsub":
                    top["pd"] += 1
                if at_depth0:
                    buf.append(c)
                cmd_start = True; j += 1; continue
            if c == ")":
                flush_word()
                if k == "cmdsub":
                    if top["pd"] > 0:
                        top["pd"] -= 1
                        if top["pat"]:
                            top["pat"] = False
                    elif top["cases"] and top["pat"]:
                        top["pat"] = False
                    else:
                        stack.pop()
                elif k == "array":
                    stack.pop()
                if at_depth0:
                    buf.append(c)
                cmd_start = False; j += 1; continue
            if c == ";" and k == "cmdsub" and top["cases"] and raw[j:j + 2] == ";;":
                top["pat"] = True
            if c in ";&|":
                flush_word(); cmd_start = True
                if at_depth0:
                    buf.append(c)
                j += 1; continue
            if c in "{}" and at_depth0:
                flush_word(); buf.append(c)
                cmd_start = (c == "{")
                j += 1; continue
            if c.isalnum() or c in "_!":
                word += c
                if at_depth0:
                    buf.append(c)
                j += 1; continue
            flush_word()
            if c in " \t":
                if at_depth0:
                    buf.append(c)
            elif at_depth0:
                buf.append(c); cmd_start = False
            j += 1
        flush_word()
        clean[i] = "".join(buf)
        # a backslash that ends the line in normal state continues the command (not after a comment, not inside a quote: the stack is non-empty then)
        if trailing_bs and not commented and not stack:
            bs_cont = True
            before = clean[i].rstrip()
            pipe_next = bool(re.search(r"(\|\||&&|\|)$", before))
        for item in new_hd:
            # a here-document is one only when its delimiter line exists later (an `<<` anywhere else is not)
            if any(((l.lstrip("\t") if item[1] else l) == item[0]) for l in lines[i:]):
                hd_queue.append(item)
        if hd_queue:
            hd_owner = group if group is not None else i
            hd_active = hd_queue.pop(0)
        if not stack and not (bs_cont and not pipe_next):
            if group is not None and group != i:
                gend[group] = i
    balanced = not stack and hd_active is None
    return kinds, alias, clean, gend, balanced


def _struct_only(rest):
    """True when what follows a leading structural word is only redirections, separators and further structural words (`esac; done`, `) 5>&-; }`, `done < "$f"`);
    a command substitution or process substitution (`done < <(cmd)`) or any other word makes the line executable."""
    while True:
        if "\x02" in rest:
            return False
        r = rest.lstrip(" \t;&")
        while True:
            m = REDIR_HEAD_RE.match(r)
            if not m:
                break
            r = r[m.end():].lstrip(" \t;&")
        if not r:
            return True
        m = STRUCT_RE.match(r)
        if not m:
            return False
        rest = m.group(2)


def classify(path):
    """Returns (executable set, functions [{name, start, end, lines}], alias map, groups [(start, end)], balanced)."""
    with open(path, encoding="utf-8", errors="replace") as fh:
        lines = fh.read().split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    kinds, alias, clean, gend, balanced = scan(lines)
    exe = set(); funcs = []
    case_depth = 0; brace = 0
    for i in range(1, len(lines) + 1):
        if kinds[i] != "start":
            continue
        s = clean[i].strip()
        if not s:
            # a line that is only quoted text or a substitution placeholder is still a command line when something was there
            s = lines[i - 1].strip()
        executable = True
        fm = FUNC_DEF_RE.match(s)
        sm = STRUCT_RE.match(s)
        if fm:
            executable = False
            funcs.append({"name": fm.group(1) or fm.group(2), "start": i, "end": None, "lines": [], "open": True, "depth": brace})
            if fm.group(3):
                brace += 1
        elif sm:
            executable = not _struct_only(sm.group(2))
        elif case_depth > 0 and ARM_RE.match(s) and not s.startswith("case "):
            rest = ARM_RE.match(s).group(1).strip()
            rest = re.sub(r";;&?$|;&$", "", rest).strip()
            executable = bool(rest)
        if re.match(r"^case\b", s):
            case_depth += 1
        if re.match(r"^esac\b", s):
            case_depth = max(0, case_depth - 1)
        if executable:
            exe.add(i)
            for f in funcs:
                if f["open"]:
                    f["lines"].append(i)
        t = clean[i]
        opens = t.count("{") - (1 if fm and fm.group(3) else 0)
        closes = t.count("}")
        brace += opens - closes
        for f in funcs:
            if f["open"] and i > f["start"] and brace <= f["depth"] and closes:
                f["open"] = False; f["end"] = i
    # every non-first line of a group folds onto the group's counted line; a here-document body line that holds a substitution folds onto its owner
    al = {}
    for ln, tgt in alias.items():
        if tgt is not None:
            al[ln] = tgt
    groups = sorted((s, e) for s, e in gend.items())
    return exe, [{"name": f["name"], "start": f["start"], "end": f["end"], "lines": f["lines"]} for f in funcs if f["lines"]], al, groups, balanced, len(lines)


# --------------------------------------------------------------------------- the trace
def parse_trace(path):
    """Yields dicts {path, line, pid, pwd, sha, cmd} for every parsable record, and counts the records that do not parse."""
    recs = []; bad = 0; total = 0
    with open(path, "rb") as fh:
        data = fh.read()
    for raw in data.split(b"\n"):
        m = TRACE_HEAD.match(raw)
        if not m:
            continue
        total += 1
        n = int(m.group(1)); p0 = m.end()
        rest = raw[p0:]
        path_b = None
        if len(rest) > n and rest[n:n + 1] == US:
            path_b = rest[:n]; after = rest[n + 1:]
        else:
            txt = rest.decode("utf-8", "replace")
            if len(txt) > n and txt[n] == "\x1f":
                path_b = txt[:n].encode("utf-8"); after = txt[n + 1:].encode("utf-8")
        if path_b is None:
            bad += 1; continue
        parts = after.split(US, 4)
        if len(parts) < 5 or not parts[0].isdigit():
            bad += 1; continue
        hexs = re.match(rb"[0-9a-f]{64}", parts[3])
        recs.append({"path": path_b.decode("utf-8", "replace"), "line": int(parts[0]), "pid": parts[1].decode("ascii", "replace"),
                     "pwd": parts[2].decode("utf-8", "replace"), "sha": hexs.group(0).decode() if hexs else "", "cmd": parts[4].decode("utf-8", "replace")})
    return recs, total, bad


def sha_file(p):
    try:
        return hashlib.sha256(open(p, "rb").read()).hexdigest()
    except OSError:
        return None


def credited_lines(src, rel, traced, presha):
    """The traced lines credited to ONE target, by PATH and never by base name alone. A traced path counts when it is
      direct - the same file: its path, resolved against the $PWD the traced shell had when it first reported that path, has the target's realpath;
      copy   - an absolute path elsewhere that ENDS WITH `/<target relative path>` of a target whose path has a directory part, AND whose sha256, taken in the run, equals
               the target's sha256 from before the run (a copy that is not the same content earns nothing and is listed `copy_unverified`).
    Anything else earns the target nothing and is listed as `foreign_same_basename` when it shares the base name."""
    absT = os.path.realpath(os.path.join(src, rel)); relT = posixpath.normpath(rel)
    got = set(); modes = {"direct": set(), "copy": set(), "copy_unverified": set(), "paths": set()}
    for (p, base), (lines, shas) in traced.items():
        mode = None
        full = posixpath.normpath(p if os.path.isabs(p) else posixpath.join(base, p))
        if os.path.realpath(full) == absT:   # MUT:direct_realpath
            mode = "direct"
        elif os.path.isabs(p) and "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix
            if presha and shas and presha in shas and len(shas) == 1:   # MUT:copy_hash
                mode = "copy"
            else:
                modes["copy_unverified"].add(p)
        if mode:
            got |= lines; modes[mode].add(p); modes["paths"].add(p)
    return got, modes


def pct(a, b):
    return "%.2f" % (100.0 * a / b) if b else "0.00"


def _args(argv):
    a = {}; targets = []
    i = 0
    while i < len(argv):
        if argv[i] == "--":
            targets = argv[i + 1:]; break
        if i + 1 >= len(argv):
            sys.stderr.write("bashcov: usage: option %s needs a value\n" % argv[i]); sys.exit(2)
        a[argv[i][2:].replace("-", "_")] = argv[i + 1]; i += 2
    return a, targets


def _write(out, rec):
    os.makedirs(os.path.dirname(os.path.abspath(out)), exist_ok=True)
    tmp = out + ".tmp.%d" % os.getpid()
    try:
        with open(tmp, "w", encoding="utf-8") as fh:
            fh.write(json.dumps(rec, indent=2, sort_keys=True) + "\n")
        os.replace(tmp, out)
    finally:
        if os.path.exists(tmp):
            os.unlink(tmp)


def cmd_refuse(argv):
    a, _t = _args(argv)
    for k in ("out", "status", "reason", "command"):
        if k not in a:
            sys.stderr.write("bashcov: usage: refuse needs --%s\n" % k); sys.exit(2)
    rec = {"schema": "bash-coverage/1", "instrument": "ps4-line-trace", "status": a["status"], "reason": a["reason"], "command": a["command"],
           "command_rc": int(a.get("command_rc", "0")), "bash_version": a.get("bash_version"), "totals": {"executable": None, "executed": None, "percent": None}, "limits": LIMITS}
    _write(a["out"], rec)
    sys.stderr.write("bash-coverage: REFUSED reason=%s %s\n" % (a["status"], a["reason"]))
    sys.exit(3)


def cmd_report(argv):
    a, targets = _args(argv)
    for k in ("src_root", "trace", "out", "pre_sha", "command", "command_rc"):
        if k not in a:
            sys.stderr.write("bashcov: usage: report needs --%s\n" % k.replace("_", "-")); sys.exit(2)
    src = os.path.abspath(a["src_root"])
    excl = []; excl_sha = None
    if a.get("exclusions"):
        import yaml
        raw = open(a["exclusions"], "rb").read(); excl_sha = hashlib.sha256(raw).hexdigest()
        excl = (yaml.safe_load(raw.decode("utf-8")) or {}).get("exclusions") or []
    norm_targets = []
    for t in targets:
        rel_ = posixpath.normpath(t)   # MUT:target_norm  (K3.4: `./vendor/v.sh` and `lib/../vendor/v.sh` name the file the fence names)
        if os.path.isabs(rel_) or rel_ == ".." or rel_.startswith("../"):
            sys.stderr.write("bash-coverage: REFUSED reason=target_outside_src %s is not under %s\n" % (t, src)); sys.exit(3)
        if not os.path.isfile(os.path.join(src, rel_)):
            sys.stderr.write("bash-coverage: REFUSED reason=target_missing %s (not a file under %s)\n" % (t, src)); sys.exit(3)
        norm_targets.append(rel_)
    targets = norm_targets
    # two targets whose paths are one file: refused. Two targets that merely share a base name are legal now that the trace carries the path.
    if len(set(os.path.realpath(os.path.join(src, t)) for t in targets)) != len(targets):   # MUT:collision
        sys.stderr.write("bash-coverage: REFUSED reason=duplicate_target two --target arguments name the same file\n"); sys.exit(3)
    pre = {}
    try:
        for line in open(a["pre_sha"], encoding="utf-8"):
            if line.strip():
                h, _s, rel = line.rstrip("\n").partition("  ")
                pre[posixpath.normpath(rel)] = h
    except OSError as e:
        sys.stderr.write("bash-coverage: REFUSED reason=pre_sha_unreadable %s\n" % e); sys.exit(3)
    recs, trace_lines, trace_bad = parse_trace(a["trace"])
    trace_digest = sha_file(a["trace"])
    if trace_lines == 0:   # MUT:empty
        sys.stderr.write("bash-coverage: REFUSED reason=trace_empty the trace holds no line: the command ran no traced bash (the instrument was blind), so no percentage is reported (11.4.201)\n"); sys.exit(3)
    if trace_bad:
        sys.stderr.write("bash-coverage: REFUSED reason=trace_unparsed %d trace record(s) could not be parsed; the instrument would undercount silently\n" % trace_bad); sys.exit(3)
    # the first $PWD each (process, path) pair reported is the directory a relative path was resolved against
    first_pwd = {}
    traced = {}
    for r in recs:
        if not r["path"]:
            continue
        key = (r["pid"], r["path"])
        base = first_pwd.setdefault(key, r["pwd"])
        ent = traced.setdefault((r["path"], base), (set(), set()))
        ent[0].add((r["line"], r["cmd"]));
        if r["sha"]:
            ent[1].add(r["sha"])
    rows = []; excluded = []
    tot_e = tot_x = 0
    changed = []
    for t in targets:
        rel = t
        e = is_excluded(rel, excl)
        if e:   # MUT:excl
            excluded.append({"file": rel, "class": e.get("class"), "justification": e.get("justification"), "tracked_item": e.get("tracked_item")}); continue
        absT = os.path.join(src, t)
        post = sha_file(absT)
        if pre.get(rel) != post:   # MUT:target_hash
            changed.append(rel)
            continue
        exe, funcs, alias, groups, balanced, nlines = classify(absT)
        with open(absT, encoding="utf-8", errors="replace") as fh:
            src_lines = fh.read().split("\n")
        first = src_lines[0] if src_lines else ""
        # strip the command text from the line records
        tr2 = {}
        ignored_line1 = []
        for key, (lc, shas) in traced.items():
            keep = set()
            for (ln, cmd) in lc:
                if ln == 1:
                    # bash reports the action of a trap (and of eval'd text) at LINENO 1 of whatever file is executing: credit it only when the command text is in line 1
                    w = cmd.split(None, 1)[0] if cmd.strip() else ""
                    w = w.lstrip("'\"")
                    if not w or w not in first:   # MUT:line1_trap
                        ignored_line1.append(key[0]); continue
                keep.add(ln)
            tr2[key] = (keep, shas)
        seen, modes = credited_lines(src, t, tr2, pre.get(rel))
        out_of_range = sorted(l for l in seen if l > nlines)
        seen = {alias.get(l, l) for l in seen if l <= nlines}
        executed = exe & seen
        uncovered_functions = [f["name"] for f in funcs if not (set(f["lines"]) & executed)]
        base = os.path.basename(t)
        foreign = sorted(set(p for (p, _b) in tr2 if os.path.basename(p) == base and p not in modes["paths"]))
        rows.append({"file": rel, "sha256": post, "executable": len(exe), "executed": len(executed), "percent": pct(len(executed), len(exe)),
                     "executed_lines": sorted(executed), "uncovered_lines": sorted(exe - executed), "uncovered_functions": uncovered_functions,
                     "traced_non_executable": sorted(seen - exe), "trace_out_of_range": out_of_range, "trace_line1_ignored": sorted(set(ignored_line1)),
                     "groups": [[s, e_] for s, e_ in groups], "classifier_balanced": balanced,
                     "attribution": {"direct_paths": sorted(modes["direct"]), "copy_paths": sorted(modes["copy"]), "copy_unverified": sorted(modes["copy_unverified"]),
                                     "foreign_same_basename": foreign}})
        tot_e += len(exe); tot_x += len(executed)
    if changed:
        sys.stderr.write("bash-coverage: REFUSED reason=target_changed_during_run %s: the file's sha256 after the run differs from the one before it\n" % ", ".join(changed)); sys.exit(3)
    rc = int(a["command_rc"])
    disagree = any(r["traced_non_executable"] for r in rows) or any(not r["classifier_balanced"] for r in rows)
    status = "command_failed" if rc != 0 and not disagree else "classifier_disagreement" if disagree else "ok"
    rec = {"schema": "bash-coverage/1", "instrument": "ps4-line-trace", "ps4": PS4, "bash_version": a.get("bash_version"), "src_root": src,
           "command": a.get("command"), "command_rc": rc, "status": status, "trace_lines": trace_lines, "trace_sha256": trace_digest,
           "targets": rows, "excluded": excluded, "exclusions_file": a.get("exclusions"), "exclusions_sha256": excl_sha,
           "totals": {"executable": tot_e, "executed": tot_x, "percent": None if disagree else pct(tot_x, tot_e)}, "limits": LIMITS}
    _write(a["out"], rec)
    if disagree:
        bad = [(r["file"], r["traced_non_executable"]) for r in rows if r["traced_non_executable"]]
        sys.stderr.write("bash-coverage: REFUSED reason=classifier_disagreement the trace shows lines the classifier calls non-executable (%s), or a target did not tokenize to a balanced state: no figure is reported (11.4.201)\n" % json.dumps(bad)[:300])
        sys.exit(3)
    print("bash-coverage: %d target(s), %d of %d executable lines executed = %s%% (command rc %d)" % (len(rows), tot_x, tot_e, rec["totals"]["percent"], rc))
    sys.exit(0)


def cmd_classify(argv):
    exe, funcs, alias, groups, balanced, _n = classify(argv[0])
    print("executable:", " ".join(map(str, sorted(exe))))
    print("groups:", " ".join("%d-%d" % g for g in groups))
    print("balanced:", balanced)
    for f in funcs:
        print("function %s lines %s" % (f["name"], f["lines"]))


if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in ("report", "classify", "refuse"):
        sys.stderr.write("usage: bashcov.py report|classify|refuse ...\n"); sys.exit(2)
    {"report": cmd_report, "classify": cmd_classify, "refuse": cmd_refuse}[sys.argv[1]](sys.argv[2:])
