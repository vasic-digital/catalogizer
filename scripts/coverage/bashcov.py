#!/usr/bin/env python3
"""bashcov.py - T199. The analysis half of scripts/bash-coverage.sh: classifies executable lines of a bash script, reads the PS4 trace and writes the
`bash-coverage/1` record. Subcommands:
  report --src-root DIR --trace FILE --out FILE [--exclusions FILE] --command STR --command-rc N --bash-version V -- TARGET...
  classify FILE            prints the executable lines, one per line, and the functions (a diagnostic of the classifier)
The tracer's line for a command that spans lines depends on the bash version (measured: a `cmd \\` + `arg` continuation is reported at the FIRST line by bash 5.3.9 and at the
LAST line by 5.2.15, the version of IMG-KCOV), so the classifier also returns an ALIAS map (continuation and array lines -> the command's counted line) and the report folds
a traced alias line onto it: one command, one counted line, whichever line the running bash reports.

EXECUTABLE-LINE RULE (a documented HEURISTIC, 11.4.6, stated in every record): a line is executable when `bash -x` can print it, that is, it is not
  - blank, a comment, or a here-document body or its delimiter line;
  - a continuation line (the line before ends with a backslash): the trace reports the FIRST line of a continued command - EXCEPT the continuation
    of a pipeline or an and/or list (the line before ends with `| \\`, `|| \\` or `&& \\`): each element is its own command and the trace reports
    its own line (measured: Build/lib/common.sh 178-179), so those lines are executable;
  - the lines of a multi-line array assignment (`NAME=(` ... `)`) except the closing one: bash traces the assignment once, at the closing line;
  - structural only: `fi`, `done`, `esac`, `else`, `then`, `do`, `{`, `}`, `)`, `;;` (optionally followed by a redirection or `;`);
  - a function definition line (`name() {`, `name()`, `function name {`): the trace shows the calls, not the definition;
  - a case label alone (`pat)`); a label with a command after it (`1) echo one ;;`) is executable.
The classifier is checked against the tracer: a line the trace shows that the rule calls non-executable is listed in `traced_non_executable` (a classifier error is visible)."""
import hashlib, json, os, posixpath, re, sys
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import fence_lib

PS4 = '+COV:${BASH_SOURCE}:${LINENO}:'
TRACE_RE = re.compile(r"^\++COV:([^\n]+?):(\d+):")
STRUCT_RE = re.compile(r"^(fi|done|esac|else|then|do|\{|\}|\)|\(|;;&?|;&)(?![A-Za-z0-9_])\s*([;<>|&0-9].*)?$")
FUNC_RE = re.compile(r"^(?:function\s+)?([A-Za-z_][A-Za-z0-9_:.-]*)\s*(?:\(\)\s*)?\{?\s*$")
FUNC_DEF_RE = re.compile(r"^(?:function\s+([A-Za-z_][A-Za-z0-9_:.-]*)\s*(?:\(\))?|([A-Za-z_][A-Za-z0-9_:.-]*)\s*\(\))\s*(\{)?\s*$")
ARRAY_OPEN_RE = re.compile(r"^[A-Za-z_][A-Za-z0-9_]*(?:\[[^\]]*\])?\+?=\([^)]*$")   # `NAME=(` with the `)` on a later line
ARM_RE = re.compile(r"^\(?[^\s()=;&|]+(?:\|[^\s()=;&|]+)*\)\s*(.*)$")
HEREDOC_RE = re.compile(r"<<-?\s*(?:\\?(['\"]?)([A-Za-z_][A-Za-z0-9_]*)\1)")
LIMITS = [
    "line, not branch: an if/else line counts covered when either side ran (the measured founding case of 11.4.224 C)",
    "regions under `set +x`, commands run by `sh` (dash) or a process that unsets BASH_ENV, and traps are not traced and count as uncovered",
    "the executable-line rule is a heuristic: structural lines, function definitions, here-document bodies and continuation lines are not counted (bashcov.py docstring)",
    "a coverage percentage is a minimum on a proxy, necessary and never sufficient (11.4.224 C): it does not show that a test would catch a defect",
]


def is_excluded(rel, excl):
    for e in excl:
        p = e["path"]
        if fence_lib.matches(p, rel):
            return e
    return None


def classify(path):
    """Returns (executable set, functions [{name, start, end, lines}])."""
    with open(path, encoding="utf-8", errors="replace") as fh:
        lines = fh.read().split("\n")
    if lines and lines[-1] == "":
        lines.pop()
    exe = set(); funcs = []
    alias = {}   # a line the tracer may report for a command that starts on another line -> that first line (the report folds them: trace semantics differ by bash version)
    cont_start = 0; array_lines = []
    heredoc = None; cont = False; case_depth = 0; prev_pipe = False; in_array = False
    stack = []  # open function frames: [name, start, depth_of_braces]
    brace = 0
    for i, raw in enumerate(lines, 1):
        s = raw.strip()
        if heredoc is not None:
            tok, dash = heredoc
            if (raw.lstrip("\t") if dash else raw) == tok:
                heredoc = None
            continue
        if cont:
            cont = s.endswith("\\") and not s.startswith("#")
            if prev_pipe and s and not s.startswith("#"):   # MUT:pipe_cont
                exe.add(i); cont_start = i
                for f in funcs:
                    if f["open"]:
                        f["lines"].append(i)
            elif cont_start:
                alias[i] = cont_start   # MUT:alias_cont  (bash 5.2 reports the LAST line of a continued simple command, 5.3 the first)
            body = s[:-1].rstrip() if s.endswith("\\") else s
            prev_pipe = bool(re.search(r"(\|\||&&|\|)$", body))
            continue
        if not s or s.startswith("#"):
            continue
        if in_array:   # MUT:array
            # a multi-line array assignment: bash traces it ONCE, at the line holding the closing parenthesis (measured: Build/lib/hash.sh 31-42 traces line 42 only)
            array_lines.append(i)
            if ")" in re.sub(r"'[^']*'|\"[^\"]*\"", "", s):
                in_array = False
                exe.add(i)
                for al in array_lines[:-1]:
                    alias[al] = i
                for f in funcs:
                    if f["open"]:
                        f["lines"].append(i)
            continue
        if ARRAY_OPEN_RE.match(re.sub(r"'[^']*'|\"[^\"]*\"", "", s)):
            in_array = True; array_lines = [i]
            continue
        executable = True
        fm = FUNC_DEF_RE.match(s)
        if fm:
            executable = False
            name = fm.group(1) or fm.group(2)
            funcs.append({"name": name, "start": i, "end": None, "lines": [], "open": True, "depth": brace})
            if fm.group(3):
                brace += 1
        elif STRUCT_RE.match(s):   # MUT:struct
            executable = False
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
        # brace tracking for function ranges (crude: braces outside quotes on the line)
        t = re.sub(r"'[^']*'|\"[^\"]*\"|\$\{[^}]*\}", "", s)
        opens = t.count("{") - (1 if fm and fm.group(3) else 0)
        closes = t.count("}")
        brace += opens - closes
        for f in funcs:
            if f["open"] and i > f["start"] and brace <= f["depth"] and closes:
                f["open"] = False; f["end"] = i
        if cont is False and raw.rstrip().endswith("\\") and not s.startswith("#"):
            cont = True
            prev_pipe = bool(re.search(r"(\|\||&&|\|)$", s[:-1].rstrip())); cont_start = i
        m = HEREDOC_RE.search(s)
        if m and "<<<" not in s[: m.start() + 3]:
            heredoc = (m.group(2), "<<-" in s[m.start(): m.start() + 3])
    return exe, [{"name": f["name"], "start": f["start"], "end": f["end"], "lines": f["lines"]} for f in funcs if f["lines"]], alias


def credited_lines(src, rel, cwd, traced):
    """The traced lines credited to ONE target, by PATH and never by base name alone (review I3). A traced path counts when it is
      direct - the same file: an absolute path whose realpath equals the target's, or a relative path that resolves from the directory the harness was
               started in (`--cwd`) to the target's realpath;
      copy   - an absolute path elsewhere that ENDS WITH `/<target relative path>` of a target whose path has a directory part (a test that copies the
               tree to a temp dir keeps the layout; a root-level target has no layout to match and is credited only when direct).
    Anything else - including a relative path used after a `cd`, whose directory the trace cannot know - earns the target nothing and is listed as
    `foreign_same_basename` when it shares the base name: an undercount that is visible, never an overcount."""
    absT = os.path.realpath(os.path.join(src, rel)); relT = posixpath.normpath(rel)
    got = set(); modes = {"direct": set(), "copy": set(), "paths": set()}
    for p, lines in traced.items():
        mode = None
        if os.path.isabs(p):
            if os.path.realpath(p) == absT:   # MUT:direct_realpath
                mode = "direct"
            elif "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix
                mode = "copy"
        elif os.path.realpath(os.path.join(cwd, p)) == absT:
            mode = "direct"
        if mode:
            got |= lines; modes[mode].add(p); modes["paths"].add(p)
    return got, modes


def pct(a, b):
    return "%.2f" % (100.0 * a / b) if b else "0.00"


def cmd_report(argv):
    a = {}; targets = []
    i = 0
    while i < len(argv):
        if argv[i] == "--":
            targets = argv[i + 1:]; break
        a[argv[i][2:].replace("-", "_")] = argv[i + 1]; i += 2
    src = os.path.abspath(a["src_root"])
    excl = []; excl_sha = None
    if a.get("exclusions"):
        import yaml
        raw = open(a["exclusions"], "rb").read(); excl_sha = hashlib.sha256(raw).hexdigest()
        excl = (yaml.safe_load(raw.decode("utf-8")) or {}).get("exclusions") or []
    for t in targets:
        if not os.path.isfile(os.path.join(src, t)):
            sys.stderr.write("bash-coverage: REFUSED reason=target_missing %s (not a file under %s)\n" % (t, src)); sys.exit(3)
    # two targets whose paths are one file: refused. Two targets that merely share a base name are legal now that the trace carries the path.
    if len(set(os.path.realpath(os.path.join(src, t)) for t in targets)) != len(targets):   # MUT:collision
        sys.stderr.write("bash-coverage: REFUSED reason=duplicate_target two --target arguments name the same file\n"); sys.exit(3)
    traced = {}      # normalised traced path -> set of lines
    trace_lines = 0
    with open(a["trace"], encoding="utf-8", errors="replace") as fh:
        for line in fh:
            m = TRACE_RE.match(line)
            if m:
                trace_lines += 1
                traced.setdefault(posixpath.normpath(m.group(1)), set()).add(int(m.group(2)))
    if trace_lines == 0:   # MUT:empty
        sys.stderr.write("bash-coverage: REFUSED reason=trace_empty the trace holds no line: the command ran no traced bash (the instrument was blind), so no percentage is reported (11.4.201)\n"); sys.exit(3)
    rows = []; excluded = []
    tot_e = tot_x = 0
    cwd = os.path.abspath(a.get("cwd") or os.getcwd())
    for t in targets:
        rel = t
        e = is_excluded(rel, excl)
        if e:   # MUT:excl
            excluded.append({"file": rel, "class": e.get("class"), "justification": e.get("justification"), "tracked_item": e.get("tracked_item")}); continue
        exe, funcs, alias = classify(os.path.join(src, t))
        seen, modes = credited_lines(src, t, cwd, traced)
        seen = {alias.get(l, l) for l in seen}
        executed = exe & seen
        uncovered_functions = [f["name"] for f in funcs if not (set(f["lines"]) & executed)]
        base = os.path.basename(t)
        foreign = sorted(p for p in traced if os.path.basename(p) == base and p not in modes["paths"])
        rows.append({"file": rel, "executable": len(exe), "executed": len(executed), "percent": pct(len(executed), len(exe)),
                     "executed_lines": sorted(executed), "uncovered_lines": sorted(exe - executed), "uncovered_functions": uncovered_functions,
                     "traced_non_executable": sorted(seen - exe), "attribution": {"direct_paths": sorted(modes["direct"]), "copy_paths": sorted(modes["copy"]),
                     "foreign_same_basename": foreign}})
        tot_e += len(exe); tot_x += len(executed)
    rc = int(a["command_rc"])
    rec = {"schema": "bash-coverage/1", "instrument": "ps4-line-trace", "ps4": PS4, "bash_version": a.get("bash_version"), "src_root": src,
           "command": a.get("command"), "command_rc": rc, "status": ("command_failed" if rc != 0 else "classifier_disagreement" if any(r["traced_non_executable"] for r in rows) else "ok"), "trace_lines": trace_lines,
           "targets": rows, "excluded": excluded, "exclusions_file": a.get("exclusions"), "exclusions_sha256": excl_sha,
           "totals": {"executable": tot_e, "executed": tot_x, "percent": pct(tot_x, tot_e)}, "limits": LIMITS}
    os.makedirs(os.path.dirname(os.path.abspath(a["out"])), exist_ok=True)
    with open(a["out"], "w", encoding="utf-8") as fh:
        fh.write(json.dumps(rec, indent=2, sort_keys=True) + "\n")
    print("bash-coverage: %d target(s), %d of %d executable lines executed = %s%% (command rc %d)" % (len(rows), tot_x, tot_e, rec["totals"]["percent"], rc))
    sys.exit(0)


def cmd_classify(argv):
    exe, funcs, _alias = classify(argv[0])
    print("executable:", " ".join(map(str, sorted(exe))))
    for f in funcs:
        print("function %s lines %s" % (f["name"], f["lines"]))


if __name__ == "__main__":
    if len(sys.argv) < 2 or sys.argv[1] not in ("report", "classify"):
        sys.stderr.write("usage: bashcov.py report|classify ...\n"); sys.exit(2)
    (cmd_report if sys.argv[1] == "report" else cmd_classify)(sys.argv[2:])
